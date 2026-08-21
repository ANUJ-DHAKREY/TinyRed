package server

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"time"
	"tinyred/rdb"
	"tinyred/resp"
	"tinyred/store"
)

const (
	ConfigPort         = "port"
	ConfigBind         = "bind"
	ConfigTimeout      = "timeout"
	ConfigTCPKeepAlive = "tcp-keepalive"
	ConfigDir          = "dir"
	ConfigDbfilename   = "dbfilename"
)

type Config struct {
	Port             int
	Bind             string
	Timeout          int
	KeepAlive        bool
	KeepAliveTimeout int
	Dir              string
	Dbfilename       string
}

type Server struct {
	Logger          *slog.Logger
	Config          *Config
	Store           *store.Store
	CommandHandlers map[Command]CommandEntry
	RDB             rdb.RDBProcessor
}

func NewServer(config *Config, logger *slog.Logger, st *store.Store, rdbProc rdb.RDBProcessor) (*Server, error) {
	s := &Server{
		Config: config,
		Logger: logger,
		Store:  st,
		RDB:    rdbProc,
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
		LPOP:   {Handler: s.HandleLPop, MinArgs: 1, MaxArgs: -1},
		LLEN:   {Handler: s.HandleLLen, MinArgs: 1, MaxArgs: -1},
		BLPOP:  {Handler: s.HandleBLPop, MinArgs: 1, MaxArgs: -1},
	}

	if config.Dir != "" && config.Dbfilename != "" {
		filePath := filepath.Join(config.Dir, config.Dbfilename)

		err := s.RDB.Load(filePath, s.Store)

		if err != nil {
			return nil, err
		}
	}
	return s, nil
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

func GetDefaultConfig() *Config {
	return &Config{
		Port:             6379,
		Bind:             "127.0.0.1",
		Timeout:          60,
		KeepAlive:        true,
		KeepAliveTimeout: 300,
		Dir:              "",
		Dbfilename:       "",
	}
}

func GetConfig(dConfig *Config) *Config {
	flag.IntVar(&dConfig.Port, "port", dConfig.Port, "Port on which server id being run")
	flag.StringVar(&dConfig.Bind, "host", dConfig.Bind, "Host to run the server, currently")
	flag.IntVar(&dConfig.Timeout, "timeout", dConfig.Timeout, "Client Timeout duration")
	flag.StringVar(&dConfig.Dbfilename, "dbfilename", dConfig.Dbfilename, "name of the RDB file")
	flag.StringVar(&dConfig.Dir, "dir", dConfig.Dir, "Base directory where RDB file")
	flag.Parse()

	return dConfig
}

type Request struct {
	Command   string
	Arguments []string
}

func (s *Server) ParseRequest(conn net.Conn, reader *bufio.Reader) (Request, error) {
	conn.SetReadDeadline(time.Now().Add(time.Duration(s.Config.Timeout) * time.Second))
	chars, err := reader.Peek(1)
	if err != nil {
		return Request{}, err
	}
	//add more request specific logic, validation and error
	//may be duplicate Array Unmarshal to here for concrete request handling
	if chars[0] != resp.TypeSymbols[resp.TypeArray] {
		return Request{}, &resp.SimpleError{
			Type:    resp.ERR,
			Message: fmt.Sprintf("protocol error: expected %q and recieved %q", resp.TypeSymbols[resp.TypeArray], chars[0]),
		}
	}

	arr := resp.Array{}
	err = arr.Unmarshal(reader)
	if err != nil {
		return Request{}, &resp.SimpleError{
			Type:    resp.ERR,
			Message: err.Error(),
		}
	}
	req := Request{
		Command: strings.ToLower(arr.Value[0]),
	}

	for i := 1; i < len(arr.Value); i++ {
		req.Arguments = append(req.Arguments, arr.Value[i])
	}

	return req, nil
}

func (s *Server) HandleConection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	defer conn.Close()
	for {
		req, err := s.ParseRequest(conn, reader)
		if err != nil {
			var clientErr *resp.SimpleError
			if errors.As(err, &clientErr) {
				conn.Write(clientErr.Marshal())
				continue
			}
			// EOF, timeout, or other I/O error — close connection
			return
		}
		cmd, ok := s.CommandHandlers[Command(req.Command)]
		if !ok {
			conn.Write(resp.ErrUnknownCommand(req.Command))
			continue
		}
		if len(req.Arguments) < cmd.MinArgs || (cmd.MaxArgs != -1 && len(req.Arguments) > cmd.MaxArgs) {
			conn.Write(resp.ErrWrongArgCount(req.Command))
			continue
		}

		data, err := cmd.Handler(req)
		conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
		if err != nil {
			var clientErr *resp.SimpleError
			if errors.As(err, &clientErr) {
				conn.Write(clientErr.Marshal())
			} else {
				s.Logger.Error("internal error occured", "cmd", req.Command)
				conn.Write(resp.ErrGeneric())
			}
			continue
		}

		conn.Write(data)
	}
}
