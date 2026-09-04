package server

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"tinyred/aof"
	"tinyred/rdb"
	"tinyred/resp"
	"tinyred/store"
)

const (
	ConfigPort           = "port"
	ConfigBind           = "bind"
	ConfigTimeout        = "timeout"
	ConfigTCPKeepAlive   = "tcp-keepalive"
	ConfigDir            = "dir"
	ConfigDbfilename     = "dbfilename"
	ConfigAppendonly     = "appendonly"
	ConfigAppenddirname  = "appenddirname"
	ConfigAppendfilename = "appendfilename"
	ConfigAppendfsync    = "appendfsync"
)

const (
	TypeAppendOnlyYes string = "yes"
	TypeAppendOnlyNo  string = "no"
)

const (
	ErrorMessageStringTypeCaste string = "internal error: list value is not []string"
)

type Request struct {
	Command   string
	Arguments []string
}

type Config struct {
	Port             int    `config:"port"`
	Bind             string `config:"bind"`
	Timeout          int    `config:"timeout"`
	KeepAlive        bool   `config:"-"`
	KeepAliveTimeout int    `config:"tcp-keepalive"`
	Dir              string `config:"dir"`
	Dbfilename       string `config:"dbfilename"`
	Appendonly       string `config:"appendonly"`
	Appenddirname    string `config:"appenddirname"`
	Appendfilename   string `config:"appendfilename"`
	Appendfsync      string `config:"appendfsync"`
}

type Server struct {
	Logger          *slog.Logger
	Config          *Config
	Store           *store.Store
	CommandHandlers map[Command]CommandEntry
	RDB             rdb.RDBProcessor
	AOF             aof.AOF
}

func NewServer(config *Config, logger *slog.Logger, st *store.Store, rdbProc rdb.RDBProcessor) (*Server, error) {
	s := &Server{
		Config: config,
		Logger: logger,
		Store:  st,
		RDB:    rdbProc,
		AOF:    aof,
	}

	s.CommandHandlers = map[Command]CommandEntry{
		ECHO:   {Handler: s.HandleEcho, MinArgs: 1, MaxArgs: 1},
		SET:    {Handler: s.HandleSet, MinArgs: 2, MaxArgs: -1},
		GET:    {Handler: s.HandleGet, MinArgs: 1, MaxArgs: -1},
		PING:   {Handler: s.HandlePing, MinArgs: 0, MaxArgs: -1},
		CONFIG: {Handler: s.HandleConfig, MinArgs: 2, MaxArgs: -1},
		KEYS:   {Handler: s.HandleKeys, MinArgs: 1, MaxArgs: -1},
		RPUSH:  {Handler: s.HandleRPush, MinArgs: 2, MaxArgs: -1},
		LPUSH:  {Handler: s.HandleLPush, MinArgs: 2, MaxArgs: -1},
		LRANGE: {Handler: s.HandleLRange, MinArgs: 3, MaxArgs: -1},
		LPOP:   {Handler: s.HandleLPop, MinArgs: 1, MaxArgs: 2},
		LLEN:   {Handler: s.HandleLLen, MinArgs: 1, MaxArgs: -1},
		BLPOP:  {Handler: s.HandleBLPop, MinArgs: 2, MaxArgs: -1},
	}

	if config.Dir != "" && config.Dbfilename != "" {
		filePath := filepath.Join(config.Dir, config.Dbfilename)

		err := s.RDB.Load(filePath, s.Store)

		if err != nil {
			return nil, err
		}
	}

	//check i manifest exist
	//if not then create one with content
	//and create a aof from same content
	//if Appendonly is enabled
	if config.Appendonly == TypeAppendOnlyYes {
		appendOnlyDir := filepath.Join(config.Dir, config.Appenddirname)
		manifestFilePath := filepath.Join(appendOnlyDir, config.Appendfilename+".manifest")
		aofPath := filepath.Join(appendOnlyDir, config.Appendfilename+".1.incr.aof")

		if _, err := os.Stat(manifestFilePath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return s, err
			}
			manifestContent := []byte("file " + filepath.Base(aofPath) + " seq 1 type i\n")
			if err := WriteFile(manifestFilePath, manifestContent, 0644); err != nil {
				return s, err
			}
		}
		content, err := os.ReadFile(manifestFilePath)
		if err != nil {
			return s, err
		}

		metadata, err := parseManifestFile(string(content))
		if err != nil {
			return s, err
		}
		if err := ensureAOFFile(filepath.Join(appendOnlyDir, metadata.AofFilename)); err != nil {
			return s, err
		}
		err = s.AOF.Load(filepath.Join(appendOnlyDir, metadata.AofFilename), func(args []string) error {
			req := Request{
				Command:   strings.ToLower(args[0]),
				Arguments: args[1:],
			}
			_, err := s.Execute(req)
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			return s, err
		}
	}

	return s, nil
}

func (s *Server) Execute(req Request) ([]byte, error) {
	cmd, ok := s.CommandHandlers[Command(req.Command)]
	if !ok {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: fmt.Sprintf("unknown command '%s'", req.Command),
		}
	}
	if len(req.Arguments) < cmd.MinArgs || (cmd.MaxArgs != -1 && len(req.Arguments) > cmd.MaxArgs) {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: fmt.Sprintf("wrong number of arguments for '%s' command", req.Command),
		}
	}
	return cmd.Handler(req)
}
func ensureAOFFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error making directory %s: %w", dir, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error creating AOF file: %w", err)
	}
	return file.Close()
}

func parseManifestFile(content string) (aof.ManifestMetaData, error) {
	// "file appendonly.aof.1.incr.aof seq 1 type i\n
	metadata := strings.Fields(content)
	if len(metadata) < 6 {
		return aof.ManifestMetaData{}, fmt.Errorf("corrupted manifest file")
	}
	var data aof.ManifestMetaData
	data.NodeType = metadata[0]
	data.AofFilename = metadata[1]
	seq, err := strconv.Atoi(metadata[3])
	if err != nil {
		return aof.ManifestMetaData{}, err
	}
	data.Seq = seq
	data.FileType = metadata[5][0]
	return data, nil
}

func WriteFile(path string, content []byte, umask os.FileMode) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("error making directory %s", err)
		}
	}

	writeErr := os.WriteFile(path, content, umask)
	if writeErr != nil {
		return fmt.Errorf("error writing pdf: %s", writeErr)
	}

	return nil
}

func (s *Server) ListenAndServe() error {
	port := fmt.Sprintf(":%d", s.Config.Port)
	listner, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}

	fmt.Printf("Server started on port: %d", s.Config.Port)
	return s.Serve(listner)
}
func (s *Server) Serve(listener net.Listener) error {
	//will add gracefull shutDown in future
	for {
		conn, err := listener.Accept()
		if err != nil {
			s.Logger.Error("error accepting connection", "error", err)
			continue
		}
		go s.HandleConection(conn)
	}
}

func GetDefaultConfig() (*Config, error) {
	config := &Config{
		Port:             6379,
		Bind:             "127.0.0.1",
		Timeout:          60,
		KeepAlive:        true,
		KeepAliveTimeout: 300,
		Dir:              "",
		Dbfilename:       "dump.rdb",
		Appendonly:       TypeAppendOnlyNo,
		Appenddirname:    "appendonlydir",
		Appendfilename:   "appendonly.aof",
		Appendfsync:      "everysec",
	}

	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	config.Dir = filepath.Dir(exePath)
	return config, nil
}

//write the validation logic for the flags as well
//and abort the startup if some wrong value is given
//the process should match with redis behaviour

func GetConfig(dConfig *Config) *Config {
	flag.IntVar(&dConfig.Port, "port", dConfig.Port, "Port on which server id being run")
	flag.StringVar(&dConfig.Bind, "host", dConfig.Bind, "Host to run the server, currently")
	flag.IntVar(&dConfig.Timeout, "timeout", dConfig.Timeout, "Client Timeout duration")
	flag.StringVar(&dConfig.Dbfilename, "dbfilename", dConfig.Dbfilename, "name of the RDB file")
	flag.StringVar(&dConfig.Dir, "dir", dConfig.Dir, "Base directory where RDB file")
	flag.StringVar(&dConfig.Appendonly, "appendonly", dConfig.Appendonly, "Controls whether AOF persistence is enabled or disabled")
	flag.StringVar(&dConfig.Appenddirname, "appenddirname", dConfig.Appenddirname, "The subdirectory under dir where AOF and manifest files are stored")
	flag.StringVar(&dConfig.Appendfilename, "appendfilename", dConfig.Appendfilename, "The name of the append-only file that records write operations")
	flag.StringVar(&dConfig.Appendfsync, "appendfsync", dConfig.Appendfsync, "How often buffered writes are flushed to the AOF file on disk")

	flag.Parse()

	return dConfig
}

type ProtocolError struct {
	Err error
}

func (e *ProtocolError) Error() string {
	return "Protocol error: " + e.Err.Error()
}

func (e *ProtocolError) Unwrap() error {
	return e.Err
}

func protocolError(err error) error {
	return &ProtocolError{Err: err}
}

func (s *Server) ParseRequest(reader *bufio.Reader) (Request, error) {
	chars, err := reader.Peek(1)
	if err != nil {
		return Request{}, err
	}
	//add more request specific logic, validation and error
	//may be duplicate Array Unmarshal to here for concrete request handling
	if chars[0] != resp.TypeSymbols[resp.TypeArray] {
		return Request{}, protocolError(fmt.Errorf("expected %q, got %q", resp.TypeSymbols[resp.TypeArray], chars[0]))
	}

	arr := resp.Array{}
	err = arr.Unmarshal(reader)
	if err != nil {
		return Request{}, protocolError(err)
	}
	if len(arr.Value) == 0 {
		return Request{}, protocolError(fmt.Errorf("empty command array"))
	}
	req := Request{
		Command:   strings.ToLower(arr.Value[0]),
		Arguments: arr.Value[1:],
	}

	return req, nil
}

func (s *Server) HandleConection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	defer conn.Close()
	for {
		conn.SetReadDeadline(time.Now().Add(time.Duration(s.Config.Timeout) * time.Second))
		req, err := s.ParseRequest(reader)
		if err != nil {
			var protocolErr *ProtocolError
			if errors.As(err, &protocolErr) {
				clientErr := &resp.SimpleError{
					Type:    resp.ERR,
					Message: protocolErr.Error(),
				}
				_, _ = conn.Write(clientErr.Marshal())
				return
			}
			var clientErr *resp.SimpleError
			if errors.As(err, &clientErr) {
				_, _ = conn.Write(clientErr.Marshal())
				return
			}
			// EOF, timeout, or other I/O error — close connection
			return
		}
		data, err := s.Execute(req)
		if err != nil {
			var clientErr *resp.SimpleError
			if errors.As(err, &clientErr) {
				conn.Write(clientErr.Marshal())
			} else {
				s.Logger.Error("internal error occured", "cmd", req.Command)
				conn.Write(resp.ErrGeneric())
			}
			conn.Close()
			return
		}

		if s.Config.Appendonly == TypeAppendOnlyYes {
			err := s.AOF.Append(data)
			if err != nil {
				s.Logger.Error("internal error occured", "cmd", req.Command)
				_, err = conn.Write(resp.ErrGeneric())
				if err != nil {
					conn.Close()
					return
				}
			}
		}
		conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
		_, err = conn.Write(data)
		if err != nil {
			conn.Close()
			return
		}
		//write to AOF
		//write to replication connection for master
		//for replica check if  it replication connection if yes do not reply
		//if not responde to read command and send error for write commands
		// err = s.handleResponse(req.Command, conn, data, err)
	}
}

func (s *Server) handleResponse(command string, conn net.Conn, data []byte, err error) error {
	if len(data) == 0 {
		return fmt.Errorf("Response: no data recieved")
	}
	return err
}
