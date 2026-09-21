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
	"slices"
	"strings"
	"sync"
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
	ErrorMessageZsetTypeCaste   string = "internal error: Zset value is not *Zset"
)

const (
	TypeClientExternal string = "external"
	TypeClientInternal string = "internal"
)

const (
	ClientModeNormal      string = "normal"
	ClientModeTransaction string = "transaction"
	ClientModeSubscribed  string = "subscribe"
)

var TransactionalCommands map[string]bool = map[string]bool{
	MULTI:   true,
	DISCARD: true,
	EXEC:    true,
	WATCH:   true,
}

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

type Client struct {
	ClientType         string
	Id                 int64
	Mode               string
	Conn               net.Conn
	CommandQueue       []Request
	WatchedKeys        map[string]bool
	DirtyCompareAndSet bool
	SubscribedChannels map[string]bool
}
type Server struct {
	Logger              *slog.Logger
	Config              *Config
	Store               *store.Store
	CommandHandlers     map[string]CommandEntry
	RDB                 rdb.RDBProcessor
	AOF                 aof.AOF
	Clients             map[int64]*Client
	ExecutionMutex      sync.RWMutex
	WatcherClients      map[string][]int64
	SubscribersMetaData map[string][]int64
}

func NewServer(config *Config, logger *slog.Logger, st *store.Store, rdbProc rdb.RDBProcessor) (*Server, error) {
	s := &Server{
		Config:              config,
		Logger:              logger,
		Store:               st,
		RDB:                 rdbProc,
		Clients:             make(map[int64]*Client),
		ExecutionMutex:      sync.RWMutex{},
		WatcherClients:      map[string][]int64{},
		SubscribersMetaData: map[string][]int64{},
	}

	s.CommandHandlers = map[string]CommandEntry{
		ECHO:        {Handler: s.HandleEcho, MinArgs: 1, MaxArgs: 1, IsWrite: false},
		SET:         {Handler: s.HandleSet, MinArgs: 2, MaxArgs: -1, IsWrite: true},
		GET:         {Handler: s.HandleGet, MinArgs: 1, MaxArgs: -1, IsWrite: false},
		PING:        {Handler: s.HandlePing, MinArgs: 0, MaxArgs: -1, IsWrite: false},
		CONFIG:      {Handler: s.HandleConfig, MinArgs: 2, MaxArgs: -1, IsWrite: false},
		KEYS:        {Handler: s.HandleKeys, MinArgs: 1, MaxArgs: -1, IsWrite: false},
		RPUSH:       {Handler: s.HandleRPush, MinArgs: 2, MaxArgs: -1, IsWrite: true},
		LPUSH:       {Handler: s.HandleLPush, MinArgs: 2, MaxArgs: -1, IsWrite: true},
		LRANGE:      {Handler: s.HandleLRange, MinArgs: 3, MaxArgs: -1, IsWrite: false},
		LPOP:        {Handler: s.HandleLPop, MinArgs: 1, MaxArgs: 2, IsWrite: true},
		LLEN:        {Handler: s.HandleLLen, MinArgs: 1, MaxArgs: -1, IsWrite: false},
		BLPOP:       {Handler: s.HandleBLPop, MinArgs: 2, MaxArgs: -1, IsWrite: true},
		INCR:        {Handler: s.HandleINCR, MinArgs: 1, MaxArgs: 1, IsWrite: true},
		MULTI:       {Handler: s.HandleMulti, MinArgs: 0, MaxArgs: 0, IsWrite: false},
		DISCARD:     {Handler: s.HandleDiscard, MinArgs: 0, MaxArgs: 0, IsWrite: false},
		WATCH:       {Handler: s.HandleWatch, MinArgs: 1, MaxArgs: -1, IsWrite: false},
		UNWATCH:     {Handler: s.HandleUnwatch, MinArgs: 0, MaxArgs: 0, IsWrite: false},
		SUBSCRIBE:   {Handler: s.HandleSubscribe, MinArgs: 1, MaxArgs: 1, IsWrite: false},
		PUBLISH:     {Handler: s.HandlePublish, MinArgs: 1, MaxArgs: 2, IsWrite: false},
		UNSUBSCRIBE: {Handler: s.HandleUnsubscribe, MinArgs: 1, MaxArgs: 1, IsWrite: false},
		ZADD:        {Handler: s.HandleZAdd, MinArgs: 3, MaxArgs: -1, IsWrite: true},
		ZCARD:       {Handler: s.HandleZCard, MinArgs: 1, MaxArgs: 1, IsWrite: false},
		ZRANGE:      {Handler: s.HandleZRange, MinArgs: 3, MaxArgs: 3, IsWrite: false},
		ZRANK:       {Handler: s.HandleZRank, MinArgs: 2, MaxArgs: 2, IsWrite: false},
		ZSCORE:      {Handler: s.HandleZScore, MinArgs: 2, MaxArgs: 2, IsWrite: false},
		ZREM:        {Handler: s.HandleZRem, MinArgs: 2, MaxArgs: 2, IsWrite: true},
		GEOADD:      {Handler: s.HandleGeoAdd, MinArgs: 4, MaxArgs: 4, IsWrite: true},
		GEOPOS:      {Handler: s.HandleGeoPos, MinArgs: 2, MaxArgs: -1, IsWrite: false},
		GEODIST:     {Handler: s.HandleGeoDist, MinArgs: 3, MaxArgs: 3, IsWrite: false},
		GEOSEARCH:   {Handler: s.HandleGeoSearch, MinArgs: 7, MaxArgs: 7, IsWrite: false},
	}

	if config.Dir != "" && config.Dbfilename != "" {
		filePath := filepath.Join(config.Dir, config.Dbfilename)

		err := s.RDB.Load(filePath, s.Store)

		if err != nil {
			return nil, err
		}
	}

	if config.Appendonly == TypeAppendOnlyYes {
		aofMgr, err := aof.New(aof.Config{
			Dir:            config.Dir,
			Appenddirname:  config.Appenddirname,
			Appendfilename: config.Appendfilename,
			Appendfsync:    config.Appendfsync,
		})
		if err != nil {
			return nil, err
		}
		s.AOF = aofMgr

		err = s.AOF.Load(func(args []string) error {
			req := Request{
				Command:   strings.ToLower(args[0]),
				Arguments: args[1:],
			}
			client := Client{
				ClientType:         TypeClientInternal,
				Id:                 getNewClientId(),
				Mode:               ClientModeNormal,
				Conn:               &net.TCPConn{},
				CommandQueue:       make([]Request, 0),
				WatchedKeys:        make(map[string]bool, 0),
				SubscribedChannels: make(map[string]bool, 0),
			}
			_, err := s.Execute(req, &client)
			return err
		})
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Server) Execute(req Request, c *Client) ([]byte, error) {
	cmd, ok := s.CommandHandlers[req.Command]
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

	transactionalCommand, ok := TransactionalCommands[req.Command]
	if !ok || !transactionalCommand {
		if c.Mode == ClientModeTransaction {
			c.CommandQueue = append(c.CommandQueue, req)
			return (&resp.SimpleString{
				Value: "QUEUED",
			}).Marshal(), nil
		}
	}
	data, err := cmd.Handler(req, c)
	if err != nil {
		return data, err
	}
	//this is not thread safe, will make this thread safe when we restructure the codebase
	if cmd.IsWrite {
		key := req.Arguments[0]
		watcherList, ok := s.WatcherClients[key]
		if !ok {
			return data, nil
		}
		for _, watcherId := range watcherList {
			watcher := s.Clients[watcherId]
			watcher.DirtyCompareAndSet = true
		}
	}
	return data, err
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

var ClientOffset int64
var ClientIDMutex sync.Mutex

func getNewClientId() int64 {
	ClientIDMutex.Lock()
	defer ClientIDMutex.Unlock()
	ClientOffset += 1
	return ClientOffset
}

func (s *Server) AddClient(conn net.Conn) *Client {
	clientId := getNewClientId()
	client := &Client{
		Id:                 clientId,
		ClientType:         TypeClientExternal,
		Mode:               ClientModeNormal,
		Conn:               conn,
		CommandQueue:       make([]Request, 0),
		WatchedKeys:        make(map[string]bool, 0),
		SubscribedChannels: make(map[string]bool, 0),
	}

	s.Clients[clientId] = client
	return client
}
func (s *Server) Serve(listener net.Listener) error {
	//will add gracefull shutDown in future
	for {
		conn, err := listener.Accept()
		if err != nil {
			s.Logger.Error("error accepting connection", "error", err)
			continue
		}
		client := s.AddClient(conn)
		go s.HandleConnection(client)
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

func (s *Server) CloseConnection(c *Client) {
	err := c.Conn.Close()
	if err != nil {
		s.Logger.Error("error closing Connection", "err", err)
	}
	delete(s.Clients, c.Id)
}

func (s *Server) ExecuteInTransaction(req Request, c *Client) []byte {
	data, err := s.Execute(req, c)
	if err != nil {
		var clientErr *resp.SimpleError
		if errors.As(err, &clientErr) {
			return clientErr.Marshal()
		} else {
			s.Logger.Error("internal error occured", "cmd", req.Command)
			return resp.ErrGeneric()
		}
	}
	return data
}

func (c *Client) ResetTransactionState() {
	c.CommandQueue = []Request{}
	c.Mode = ClientModeNormal
}
func (s *Server) UnwatchAll(c *Client) {
	// Remove c from each key's global watcher list.
	keys := c.WatchedKeys
	for key := range keys {
		watchers, ok := s.WatcherClients[key]
		if !ok {
			break
		}
		i := slices.Index(watchers, c.Id)
		if i == -1 {
			continue
		}
		watchers = slices.Delete(watchers, i, i+1)
		if len(watchers) == 0 {
			delete(s.WatcherClients, key)
		}
		s.WatcherClients[key] = watchers
	}
	c.WatchedKeys = map[string]bool{}
	c.DirtyCompareAndSet = false
}

func (s *Server) ResetTransactionState(c *Client) {
	s.UnwatchAll(c)
	c.ResetTransactionState()
}
func (s *Server) HandleConnection(c *Client) {
	reader := bufio.NewReader(c.Conn)
	defer s.CloseConnection(c)
	for {
		c.Conn.SetReadDeadline(time.Now().Add(time.Duration(s.Config.Timeout) * time.Second))
		req, err := s.ParseRequest(reader)
		if err != nil {
			var protocolErr *ProtocolError
			if errors.As(err, &protocolErr) {
				clientErr := &resp.SimpleError{
					Type:    resp.ERR,
					Message: protocolErr.Error(),
				}
				_, err := c.Conn.Write(clientErr.Marshal())
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
				}
				break
			}
			var clientErr *resp.SimpleError
			if errors.As(err, &clientErr) {
				_, err := c.Conn.Write(clientErr.Marshal())
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
				continue
			}
			// EOF, timeout, or other I/O error — close connection
			break
		}

		if c.Mode == ClientModeSubscribed {
			allowed, ok := SubcribedModeCommands[req.Command]
			if !ok || !allowed {
				data := (&resp.SimpleError{
					Type:    resp.ERR,
					Message: fmt.Sprintf("Can't execute %s: only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context", req.Command),
				}).Marshal()
				_, err = c.Conn.Write(data)
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
				continue
			}
		}
		//SubcribedModeCommands
		if req.Command == EXEC {
			if c.Mode != ClientModeTransaction {
				se := resp.SimpleError{
					Type:    resp.ERR,
					Message: "EXEC without MULTI",
				}

				_, err := c.Conn.Write(se.Marshal())
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
				continue
			}

			if c.DirtyCompareAndSet {
				s.ResetTransactionState(c)
				data := (&resp.NullArray{}).Marshal()
				_, err = c.Conn.Write(data)
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
				continue
			}
			s.ExecutionMutex.Lock()
			AOFData := make([]byte, 0)
			response := resp.Array{}
			queue := s.Clients[c.Id].CommandQueue
			s.ResetTransactionState(c)
			for _, req := range queue {
				data := s.ExecuteInTransaction(req, c)
				response.Value = append(response.Value, string(data))
				if s.Config.Appendonly == TypeAppendOnlyYes {
					a := resp.Array{}
					a.Value = append(a.Value, string((&resp.BulkString{Value: req.Command}).Marshal()))
					for i := range req.Arguments {
						a.Value = append(a.Value, string((&resp.BulkString{Value: req.Arguments[i]}).Marshal()))
					}
					AOFData = append(AOFData, a.Marshal()...)
				}
			}

			if s.Config.Appendonly == TypeAppendOnlyYes {
				err := s.AOF.Append(AOFData)
				_ = err
			}

			//handle AOF error
			//for now ignore the error in future we need to
			//stop servng write commands in case of error while writing
			s.ExecutionMutex.Unlock()
			c.Conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
			_, err := c.Conn.Write(response.Marshal())
			if err != nil {
				s.Logger.Error("error writing response", "err", err.Error())
				break
			}
			continue
		}

		s.ExecutionMutex.RLock()
		data, err := s.Execute(req, c)
		if err != nil {
			var clientErr *resp.SimpleError
			if errors.As(err, &clientErr) {
				_, err = c.Conn.Write(clientErr.Marshal())
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
			} else {
				s.Logger.Error("internal error occured", "cmd", req.Command)
				_, err = c.Conn.Write(resp.ErrGeneric())
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
			}
		}

		cmdEntry, ok := s.CommandHandlers[req.Command]
		if s.Config.Appendonly == TypeAppendOnlyYes && ok && cmdEntry.IsWrite {
			err := s.AOF.AppendCmd(strings.ToUpper(req.Command), req.Arguments)
			// for now ignore the error handle AOF write failures in future
			_ = err
		}
		s.ExecutionMutex.RUnlock()
		c.Conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
		_, err = c.Conn.Write(data)
		if err != nil {
			s.Logger.Error("error writing response", "err", err.Error())
			break
		}
	}
}
