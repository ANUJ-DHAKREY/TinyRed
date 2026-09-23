package server

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

const (
	NodeRoleMaster  string = "master"
	NodeRoleReplica string = "replica"
)

const (
	ReSyncTypeFull string = "FULLRESYNC"
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

type Response = resp.Value

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
	Replicaof        string `config:"-"`
}

type Client struct {
	ClientType         string
	Id                 int64
	Mode               string
	Conn               net.Conn
	CommandQueue       []Request
	WatchedKeys        map[string]bool
	DirtyCompareAndSet atomic.Bool
	SubscribedChannels map[string]bool
}

type MasterConfig struct {
	Host string
	Port uint16
	Conn net.Conn
}

type ReplicaConfig struct {
	ReplicaId     string
	Conn          net.Conn
	ReplicaOffSet int64
}

type NodeConfig struct {
	Role              string
	Master            MasterConfig
	replicaNodes      []ReplicaConfig
	ReplicationId     string
	ReplicationOffset int64
	ReplicaReady      bool
}

type Server struct {
	NodeConfig      *NodeConfig
	Logger          *slog.Logger
	Config          *Config
	Store           *store.Store
	CommandHandlers map[string]CommandEntry
	RDB             rdb.RDBProcessor
	AOF             aof.AOF

	executionMutex sync.RWMutex

	sessionMu           sync.RWMutex
	clients             map[int64]*Client
	watcherClients      map[string][]int64
	subscribersMetaData map[string][]int64

	clientIDCounter atomic.Int64
}

func newReplicationID() string {
	buf := make([]byte, 20)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (s *Server) StartMaster() error {
	if s.Config.Dir != "" && s.Config.Dbfilename != "" {
		filePath := filepath.Join(s.Config.Dir, s.Config.Dbfilename)

		err := s.RDB.Load(filePath, s.Store)

		if err != nil {
			return err
		}
	}

	if s.Config.Appendonly == TypeAppendOnlyYes {
		aofMgr, err := aof.New(aof.Config{
			Dir:            s.Config.Dir,
			Appenddirname:  s.Config.Appenddirname,
			Appendfilename: s.Config.Appendfilename,
			Appendfsync:    s.Config.Appendfsync,
		})
		if err != nil {
			return err
		}
		s.AOF = aofMgr

		err = s.AOF.Load(func(args []string) error {
			req := Request{
				Command:   strings.ToLower(args[0]),
				Arguments: args[1:],
			}
			client := Client{
				ClientType:         TypeClientInternal,
				Id:                 s.nextClientID(),
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
			return err
		}
	}
	s.NodeConfig.ReplicationId = newReplicationID()
	s.NodeConfig.ReplicationOffset = 0
	return nil
}

func (s *Server) ReplicaHandShake() (net.Conn, *bufio.Reader, error) {
	//connect to master on tcp connection
	//write the commands to server and wait for responses
	//if succesfull add conn object to master conf
	//and return from this
	address := net.JoinHostPort(s.NodeConfig.Master.Host, strconv.Itoa(int(s.NodeConfig.Master.Port)))

	conn, err := net.DialTimeout("tcp", address, time.Duration(s.Config.Timeout)*time.Second)
	if err != nil {
		return nil, nil, err
	}
	s.NodeConfig.Master.Conn = conn
	reader := bufio.NewReader(conn)
	//send ping
	_, err = conn.Write([]byte(resp.StaticRequestPing))
	if err != nil {
		return nil, nil, err
	}

	value, err := resp.ReadResponse(reader)
	if err != nil {
		return nil, nil, err
	}
	pong, ok := value.(*resp.SimpleString)
	if !ok || pong.Value != "PONG" {
		return nil, nil, fmt.Errorf("expected PONG response, got %T", value)
	}
	//send repl conf
	port := strconv.Itoa(s.Config.Port)
	_, err = conn.Write([]byte(fmt.Sprintf(resp.StaticRequestReplConfPort, len(port), port)))
	if err != nil {
		return nil, nil, err
	}
	value, err = resp.ReadResponse(reader)
	if err != nil {
		return nil, nil, err
	}

	OKReply, ok := value.(*resp.SimpleString)
	if !ok || !strings.EqualFold(OKReply.Value, "OK") {
		return nil, nil, fmt.Errorf("expected OK response, got %T", value)
	}

	//send repl conf
	_, err = conn.Write([]byte(resp.StaticRequestReplConfCapa))
	if err != nil {
		return nil, nil, err
	}
	value, err = resp.ReadResponse(reader)
	if err != nil {
		return nil, nil, err
	}

	OKReply, ok = value.(*resp.SimpleString)
	if !ok || !strings.EqualFold(OKReply.Value, "OK") {
		return nil, nil, fmt.Errorf("expected OK response, got %T", value)
	}

	//send psync2
	_, err = conn.Write([]byte(resp.StaticRequestPsync2))
	if err != nil {
		return nil, nil, err
	}
	value, err = resp.ReadResponse(reader)
	if err != nil {
		return nil, nil, err
	}

	ReSyncResp, ok := value.(*resp.SimpleString)
	if !ok {
		return nil, nil, fmt.Errorf("expected a simple string response, got %T", value)
	}
	fields := strings.Fields(ReSyncResp.Value)
	if len(fields) != 3 {
		return nil, nil, fmt.Errorf("invalid psync2 response")
	}
	if fields[0] != ReSyncTypeFull || fields[1] == "" || fields[2] == "" {
		return nil, nil, fmt.Errorf("invalid resync response")
	}

	return conn, reader, nil
}
func (s *Server) StartReplica() error {
	conn, reader, err := s.ReplicaHandShake()
	if err != nil {
		return err
	}
	//$<length_of_file>\r\n<binary_contents_of_file>
	data, err := resp.ReadHeader(reader)
	if err != nil {
		return err
	}
	symbol := data[0]
	if symbol != resp.TypeSymbols[resp.TypeBulkString] {
		return fmt.Errorf("invalid RDB data recieved")
	}
	size := data[1 : len(data)-2]
	rdbFileSize, err := strconv.Atoi(string(size))
	if err != nil {
		return err
	}
	buf := make([]byte, rdbFileSize)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return err
	}
	s.executionMutex.Lock()
	s.NodeConfig.ReplicaReady = true
	s.executionMutex.Unlock()
	return s.processMasterCommands(conn, reader)
}

func (s *Server) processMasterCommands(conn net.Conn, reader *bufio.Reader) error {
	client := &Client{
		ClientType:         TypeClientInternal,
		Id:                 s.nextClientID(),
		Mode:               ClientModeNormal,
		Conn:               conn,
		CommandQueue:       make([]Request, 0),
		WatchedKeys:        make(map[string]bool),
		SubscribedChannels: make(map[string]bool),
	}

	for {
		req, err := s.ParseRequest(reader)
		if err != nil {
			return err
		}
		requestOffset := int64(len(getRequestinResp(req)))

		if req.Command == REPLCONF && len(req.Arguments) == 2 &&
			strings.EqualFold(req.Arguments[0], "GETACK") {
			ack := getRequestinResp(Request{
				Command:   REPLCONF,
				Arguments: []string{"ACK", strconv.FormatInt(s.NodeConfig.ReplicationOffset, 10)},
			})
			if _, err := conn.Write(ack); err != nil {
				return err
			}
			s.NodeConfig.ReplicationOffset += requestOffset
			continue
		}

		if _, err := s.Execute(req, client); err != nil {
			return err
		}
		s.NodeConfig.ReplicationOffset += requestOffset
	}
}
func IsValidMasterConf(endpoint string) bool {
	endpoint = strings.TrimSpace(endpoint)
	conf := strings.Fields(endpoint)
	if len(conf) != 2 {
		return false
	}
	host := conf[0]
	portStr := conf[1]
	if host == "" {
		return false
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}

	if port < 1 || port > 65535 {
		return false
	}

	target := net.JoinHostPort(host, portStr)

	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
func NewServer(config *Config, logger *slog.Logger, st *store.Store, rdbProc rdb.RDBProcessor) (*Server, error) {
	s := &Server{
		Config:              config,
		Logger:              logger,
		Store:               st,
		RDB:                 rdbProc,
		clients:             make(map[int64]*Client),
		watcherClients:      map[string][]int64{},
		subscribersMetaData: map[string][]int64{},
		NodeConfig: &NodeConfig{
			Role:         NodeRoleMaster,
			replicaNodes: make([]ReplicaConfig, 0),
			Master:       MasterConfig{},
			ReplicaReady: true,
		},
	}

	replicaOf := config.Replicaof
	if replicaOf != "" {
		fields := strings.Fields(replicaOf)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid master configuration")
		}
		s.NodeConfig.Master.Host = fields[0]
		port, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid master port")
		}
		s.NodeConfig.Master.Port = uint16(port)
		s.NodeConfig.Role = NodeRoleReplica
		s.NodeConfig.ReplicaReady = false
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
		INFO:        {Handler: s.HandleInfo, MinArgs: 0, MaxArgs: -1, IsWrite: false},
		REPLCONF:    {Handler: s.HandleReplConf, MinArgs: 2, MaxArgs: 2, IsWrite: false},
		PSYNC:       {Handler: s.HandlePSync, MinArgs: 2, MaxArgs: 2, IsWrite: false},
		WAIT:        {Handler: s.HandleWait, MinArgs: 2, MaxArgs: 2, IsWrite: false},
	}

	switch s.NodeConfig.Role {
	case NodeRoleMaster:
		{
			err := s.StartMaster()
			if err != nil {
				return nil, err
			}
		}
	case NodeRoleReplica:
		{
			go func() {
				if err := s.StartReplica(); err != nil {
					s.Logger.Error("replica handshake failed", "error", err)
				}
			}()
		}
	default:
		{
			return nil, fmt.Errorf("Invalid value provided for replicaof flag")
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
	if s.NodeConfig.Role == NodeRoleReplica && !s.NodeConfig.ReplicaReady {
		switch req.Command {
		case INFO, CONFIG, ECHO, PING:
		default:
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: "LOADING Replica is loading the dataset in memory",
			}
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
	if cmd.IsWrite {
		key := req.Arguments[0]
		s.sessionMu.RLock()
		watchers := make([]*Client, 0, len(s.watcherClients[key]))
		for _, watcherId := range s.watcherClients[key] {
			if watcher, ok := s.clients[watcherId]; ok {
				watchers = append(watchers, watcher)
			}
		}
		s.sessionMu.RUnlock()
		for _, watcher := range watchers {
			watcher.DirtyCompareAndSet.Store(true)
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

func (s *Server) nextClientID() int64 {
	return s.clientIDCounter.Add(1)
}

func (s *Server) AddClient(conn net.Conn) *Client {
	clientId := s.nextClientID()
	client := &Client{
		Id:                 clientId,
		ClientType:         TypeClientExternal,
		Mode:               ClientModeNormal,
		Conn:               conn,
		CommandQueue:       make([]Request, 0),
		WatchedKeys:        make(map[string]bool, 0),
		SubscribedChannels: make(map[string]bool, 0),
	}

	s.sessionMu.Lock()
	s.clients[clientId] = client
	s.sessionMu.Unlock()
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
		Timeout:          5,
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
	flag.StringVar(&dConfig.Replicaof, "replicaof", dConfig.Replicaof, "Decides the role of master in redis cluster(master/replica)")
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

func (s *Server) ParseResponse(reader *bufio.Reader) (Response, error) {
	chars, err := reader.Peek(1)
	if err != nil {

	}
	_ = chars

	return Response{}, nil
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
	s.sessionMu.Lock()
	delete(s.clients, c.Id)
	s.sessionMu.Unlock()
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
	s.sessionMu.Lock()
	for key := range keys {
		watchers, ok := s.watcherClients[key]
		if !ok {
			continue
		}
		i := slices.Index(watchers, c.Id)
		if i == -1 {
			continue
		}
		watchers = slices.Delete(watchers, i, i+1)
		if len(watchers) == 0 {
			delete(s.watcherClients, key)
		} else {
			s.watcherClients[key] = watchers
		}
	}
	s.sessionMu.Unlock()
	c.WatchedKeys = map[string]bool{}
	c.DirtyCompareAndSet.Store(false)
}

func (s *Server) ResetTransactionState(c *Client) {
	s.UnwatchAll(c)
	c.ResetTransactionState()
}

func newReplicaId() string {
	buf := make([]byte, 10)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}
func (s *Server) AddReplica(c *Client) {
	s.sessionMu.Lock()
	s.NodeConfig.replicaNodes = append(s.NodeConfig.replicaNodes, ReplicaConfig{
		ReplicaId:     newReplicaId(),
		ReplicaOffSet: 0,
		Conn:          c.Conn,
	})
	s.sessionMu.Unlock()
}

func getRequestinResp(req Request) []byte {
	args := append([]string{strings.ToUpper(req.Command)}, req.Arguments...)
	arr := resp.Array{}
	for _, arg := range args {
		bs := resp.BulkString{Value: arg}
		arr.Value = append(arr.Value, string(bs.Marshal()))
	}
	return arr.Marshal()
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

			if c.DirtyCompareAndSet.Load() {
				s.ResetTransactionState(c)
				data := (&resp.NullArray{}).Marshal()
				_, err = c.Conn.Write(data)
				if err != nil {
					s.Logger.Error("error writing response", "err", err.Error())
					break
				}
				continue
			}
			s.executionMutex.Lock()
			AOFData := make([]byte, 0)
			response := resp.Array{}
			queue := c.CommandQueue
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

			cmdEntry, ok := s.CommandHandlers[req.Command]
			if ok && cmdEntry.IsWrite {
				if s.Config.Appendonly == TypeAppendOnlyYes {
					err := s.AOF.Append(AOFData)
					_ = err
				}

				requestInResp := getRequestinResp(req)
				s.sessionMu.RLock()
				replicas := append([]ReplicaConfig(nil), s.NodeConfig.replicaNodes...)
				s.sessionMu.RUnlock()
				for _, replica := range replicas {
					_, err := replica.Conn.Write(requestInResp)
					_ = err
					//ignoring error for now
				}
			}

			//handle AOF error
			//for now ignore the error in future we need to
			//stop servng write commands in case of error while writing
			s.executionMutex.Unlock()
			c.Conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
			_, err := c.Conn.Write(response.Marshal())
			if err != nil {
				s.Logger.Error("error writing response", "err", err.Error())
				break
			}
			continue
		}

		s.executionMutex.RLock()
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
		s.executionMutex.RUnlock()
		cmdEntry, ok := s.CommandHandlers[req.Command]
		if ok && cmdEntry.IsWrite {
			if s.Config.Appendonly == TypeAppendOnlyYes {
				err := s.AOF.AppendCmd(strings.ToUpper(req.Command), req.Arguments)
				// for now ignore the error handle AOF write failures in future
				_ = err
			}

			requestInResp := getRequestinResp(req)
			s.sessionMu.RLock()
			replicas := append([]ReplicaConfig(nil), s.NodeConfig.replicaNodes...)
			s.sessionMu.RUnlock()
			for _, replica := range replicas {
				_, err := replica.Conn.Write(requestInResp)
				_ = err
				//ignoring error for now
			}
		}

		if req.Command == PSYNC {
			s.AddReplica(c)
			c.Conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
			_, err = c.Conn.Write(data)

			//err := s.RDB.Load(filepath.Join(s.Config.Dir, s.Config.Dbfilename), s.Store)
			//for now we are ignoring rdn falure error and sendinf a
			// static empty rdb file content in response
			buf := []byte("$0\r\n")
			_, err = c.Conn.Write(buf)
			if err != nil {
				s.DeleteReplica(c)
			}
			continue
		}
		c.Conn.SetWriteDeadline(time.Now().Add(120 * time.Second))
		_, err = c.Conn.Write(data)
		if err != nil {
			s.Logger.Error("error writing response", "err", err.Error())
			break
		}
	}
}

func (s *Server) DeleteReplica(c *Client) {
	s.sessionMu.Lock()
	for idx, replica := range s.NodeConfig.replicaNodes {
		if replica.Conn == c.Conn {
			s.NodeConfig.replicaNodes = slices.Delete(s.NodeConfig.replicaNodes, idx, idx+1)
			break
		}
	}
	s.sessionMu.Unlock()
}
