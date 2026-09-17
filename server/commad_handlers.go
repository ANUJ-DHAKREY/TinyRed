package server

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"tinyred/resp"
	"tinyred/store"
)

const (
	ECHO         string = "echo"
	SET          string = "set"
	GET          string = "get"
	PING         string = "ping"
	CONFIG       string = "config"
	KEYS         string = "keys"
	RPUSH        string = "rpush"
	LPUSH        string = "lpush"
	LPOP         string = "lpop"
	LLEN         string = "llen"
	BLPOP        string = "blpop"
	LRANGE       string = "lrange"
	INCR         string = "incr"
	MULTI        string = "multi"
	DISCARD      string = "discard"
	EXEC         string = "exec"
	WATCH        string = "watch"
	UNWATCH      string = "unwatch"
	SUBSCRIBE    string = "subscribe"
	UNSUBSCRIBE  string = "unsubscribe"
	PSUBSCRIBE   string = "psubscribe"
	PUNSUBSCRIBE string = "punsubscribe"
	QUIT         string = "quit"
	PUBLISH      string = "publish"
	ZADD         string = "zadd"
	ZCARD        string = "zcard"
	ZRANGE       string = "zrange"
	ZRANK        string = "zrank"
	ZREM         string = "zrem"
	ZSCORE       string = "zscore"
)

var SubcribedModeCommands = map[string]bool{
	SUBSCRIBE:    true,
	UNSUBSCRIBE:  true,
	PSUBSCRIBE:   true,
	PUNSUBSCRIBE: true,
	PING:         true,
	QUIT:         true,
}

type CommandEntry struct {
	Handler func(Request, *Client) ([]byte, error)
	MinArgs int
	MaxArgs int
	IsWrite bool
}

type ZSetEntry struct {
	Member string
	Score  float32
}

func (s *Server) HandleZAdd(req Request, c *Client) ([]byte, error) {
	//validate the parameters
	//check even arguments are passed
	// ZADD racer_scores 8.0 "Sam"
	setKey := req.Arguments[0]
	req.Arguments = req.Arguments[1:]
	if len(req.Arguments)%2 != 0 {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: string(resp.ErrWrongArgCount(req.Command)),
		}
	}
	list := make([]ZSetEntry, 0)
	for i := 0; i < len(req.Arguments); i += 2 {
		score, err := strconv.ParseFloat(req.Arguments[i], 32)
		if err != nil {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageNotFloat,
			}
		}
		list = append(list, ZSetEntry{
			Member: req.Arguments[i+1],
			Score:  float32(score),
		})
	}

	var membersAdded int64
	_, err := s.Store.Update(setKey, func(e *store.Entry) (*store.Entry, error) {
		if e == nil {
			zset := store.ZSet{
				HashMap:   make(map[string]float32),
				ZSkipList: store.NewSkipList(),
			}
			for _, pair := range list {
				zset.HashMap[pair.Member] = pair.Score
				zset.ZSkipList.Insert(pair.Member, pair.Score)
			}
			membersAdded = int64(len(list))
			return &store.Entry{
				Type:  store.EntryTypeSortedSet,
				Value: zset,
			}, nil
		}

		if e.Type != store.EntryTypeSortedSet {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageWrongType,
			}
		}
		zset, ok := e.Value.(store.ZSet)
		if !ok {
			return nil, fmt.Errorf(ErrorMessageZsetTypeCaste)
		}
		for _, pair := range list {
			oldScore, exists := zset.HashMap[pair.Member]
			if exists {
				zset.ZSkipList.Delete(pair.Member, oldScore)
			} else {
				membersAdded++
			}
			zset.ZSkipList.Insert(pair.Member, pair.Score)
			zset.HashMap[pair.Member] = pair.Score
		}
		return e, nil
	})
	if err != nil {
		return nil, err
	}
	return (&resp.Integer{
		Value: membersAdded,
	}).Marshal(), nil
}

func (s *Server) HandleZRank(req Request, c *Client) ([]byte, error) {
	setKey := req.Arguments[0]
	member := req.Arguments[1]
	entry, ok := s.Store.Get(setKey)
	if !ok {
		return (&resp.NullBulkString{}).Marshal(), nil
	}

	if entry.Type != store.EntryTypeSortedSet {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: resp.ErrorMessageWrongType,
		}
	}

	zset, ok := entry.Value.(store.ZSet)
	if !ok {
		return nil, fmt.Errorf(ErrorMessageZsetTypeCaste)
	}
	score, ok := zset.HashMap[member]
	if !ok {
		return (&resp.NullBulkString{}).Marshal(), nil
	}
	rank := zset.ZSkipList.GetRank(member, score)
	return (&resp.Integer{
		Value: rank - 1,
	}).Marshal(), nil
}

func (s *Server) HandleZRange(req Request, c *Client) ([]byte, error) {

	//ZRANGE racer_scores 0 2
	//get key , parse start and end index //throw error if not integer
	setKey := req.Arguments[0]
	startIdx, err := strconv.Atoi(req.Arguments[1])
	if err != nil {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: resp.ErrorMessageNotInteger,
		}
	}
	endIdx, err := strconv.Atoi(req.Arguments[2])
	if err != nil {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: resp.ErrorMessageNotInteger,
		}
	}
	arr := &resp.Array{}
	entry, ok := s.Store.Get(setKey)
	// If the sorted set does not exist, an empty array (*0\r\n) is returned
	if !ok {
		return arr.Marshal(), nil
	}
	if entry.Type != store.EntryTypeSortedSet {
		return nil, &resp.SimpleError{
			Type:    resp.WRONGTYPE,
			Message: resp.ErrorMessageWrongType,
		}
	}

	zset, ok := entry.Value.(store.ZSet)
	if !ok {
		return nil, fmt.Errorf(ErrorMessageZsetTypeCaste)
	}

	len := len(zset.HashMap)
	startIdx = normalizeIndex(startIdx, len)
	endIdx = normalizeIndex(endIdx, len)
	if startIdx >= endIdx {
		return arr.Marshal(), nil
	}

	memberList := zset.ZSkipList.Range(int64(startIdx+1), int64(endIdx+1))
	for _, member := range memberList {
		bs := string((&resp.BulkString{
			Value: member.Member,
		}).Marshal())
		arr.Value = append(arr.Value, bs)
	}
	return arr.Marshal(), nil
	// If the start index is greater than or equal to the cardinality of the sorted set, an empty array is returned.
	// If the stop index is greater than the cardinality of the sorted set, the stop index is treated as the last element.
	// If the start index is greater than the stop index, the result is an empty array.
	// An index of -1 refers to the last element, -2 to the second last, and so on.
	// If a absolute value of the negative index is out of range (i.e. >= the cardinality of the sorted set),
	// it is treated as 0 (start of the sorted set).
}

func (s *Server) HandleZRem(req Request, c *Client) ([]byte, error) {
	setKey := req.Arguments[0]
	member := req.Arguments[1]
	var removed int64
	_, err := s.Store.Update(setKey, func(e *store.Entry) (*store.Entry, error) {
		if e == nil {
			return nil, nil
		}
		if e.Type != store.EntryTypeSortedSet {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageWrongType,
			}
		}
		zset, ok := e.Value.(store.ZSet)
		if !ok {
			return nil, fmt.Errorf(ErrorMessageZsetTypeCaste)
		}
		score, ok := zset.HashMap[member]
		if !ok {
			return e, nil
		}
		delete(zset.HashMap, member)
		zset.ZSkipList.Delete(member, score)
		removed = 1
		return e, nil
	})
	if err != nil {
		return nil, err
	}
	return (&resp.Integer{Value: removed}).Marshal(), nil
}

func (s *Server) HandleZScore(req Request, c *Client) ([]byte, error) {
	setKey := req.Arguments[0]
	member := req.Arguments[1]
	entry, ok := s.Store.Get(setKey)
	if !ok {
		return (&resp.NullBulkString{}).Marshal(), nil
	}

	if entry.Type != store.EntryTypeSortedSet {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: resp.ErrorMessageWrongType,
		}
	}

	zset, ok := entry.Value.(store.ZSet)
	if !ok {
		return nil, fmt.Errorf(ErrorMessageZsetTypeCaste)
	}
	score, ok := zset.HashMap[member]
	if !ok {
		return (&resp.NullBulkString{}).Marshal(), nil
	}

	return (&resp.BulkString{
		Value: strconv.FormatFloat(float64(score), 'f', -1, 32),
	}).Marshal(), nil
}

func (s *Server) HandleZCard(req Request, c *Client) ([]byte, error) {
	setKey := req.Arguments[0]
	entry, ok := s.Store.Get(setKey)
	if !ok {
		return (&resp.Integer{}).Marshal(), nil
	}

	if entry.Type != store.EntryTypeSortedSet {
		return nil, &resp.SimpleError{
			Type:    resp.WRONGTYPE,
			Message: resp.ErrorMessageWrongType,
		}
	}

	zset, ok := entry.Value.(store.ZSet)
	if !ok {
		return nil, fmt.Errorf(ErrorMessageZsetTypeCaste)
	}
	return (&resp.Integer{
		Value: int64(len(zset.HashMap)),
	}).Marshal(), nil
}

func (s *Server) HandleUnsubscribe(req Request, c *Client) ([]byte, error) {
	//check if the this client if subscribe to that channel
	//if yes then remove that channel from subcrption list from client
	//and server subcriber list
	//return the reamining channel count
	channel := req.Arguments[0]
	exist, ok := c.SubscribedChannels[channel]

	// "unsubscribe" (as a RESP bulk string)
	// The channel name (as a RESP bulk string)
	// Count of remaining channels the client has subscribed to (as a RESP integer)
	unsubscribeString := (&resp.BulkString{Value: "unsubscribe"}).Marshal()
	channelName := (&resp.BulkString{Value: channel}).Marshal()
	response := &resp.Array{Value: []string{string(unsubscribeString), string(channelName)}}
	if !ok || !exist {
		response.Value = append(response.Value, string((&resp.Integer{Value: 0}).Marshal()))
		return response.Marshal(), nil
	}
	delete(c.SubscribedChannels, channel)
	subscribers, ok := s.SubscribersMetaData[channel]
	if !ok {
		response.Value = append(response.Value, string((&resp.Integer{Value: int64(len(c.SubscribedChannels))}).Marshal()))
		return response.Marshal(), nil
	}
	idx := slices.Index(subscribers, c.Id)
	if idx == -1 {
		response.Value = append(response.Value, string((&resp.Integer{Value: int64(len(c.SubscribedChannels))}).Marshal()))
		return response.Marshal(), nil
	}
	subscribers = slices.Delete(subscribers, idx, idx+1)
	if len(subscribers) == 0 {
		delete(s.SubscribersMetaData, channel)
	} else {
		s.SubscribersMetaData[channel] = subscribers
	}
	response.Value = append(response.Value, string((&resp.Integer{Value: int64(len(c.SubscribedChannels))}).Marshal()))
	return response.Marshal(), nil
}

func (s *Server) HandlePublish(req Request, c *Client) ([]byte, error) {
	//get the message and the channel
	channel := req.Arguments[0]
	message := req.Arguments[1]
	subscribers, ok := s.SubscribersMetaData[channel]
	if !ok || len(subscribers) == 0 {
		return (&resp.Integer{Value: 0}).Marshal(), nil
	}
	for _, subId := range subscribers {
		//check how many subcribers exist for that key
		//for each subcribers spwan a go routine for now just write in a for loop
		//and write response to the subcriber
		subscriber, ok := s.Clients[subId]
		if !ok {
			return nil, fmt.Errorf("some internal error occured client/subscriber npt found")
		}
		messageName := (&resp.BulkString{Value: "message"}).Marshal()
		channel := (&resp.BulkString{Value: channel}).Marshal()
		messageContent := (&resp.BulkString{Value: string(message)}).Marshal()
		data := (&resp.Array{Value: []string{string(messageName), string(channel), string(messageContent)}}).Marshal()
		_, err := subscriber.Conn.Write(data)
		if err != nil {
			return nil, err
		}
	}
	return (&resp.Integer{Value: int64(len(subscribers))}).Marshal(), nil
	//return the count of subcribers in the result

}
func (s *Server) HandleSubscribe(req Request, c *Client) ([]byte, error) {
	//make a entry of they channel in the client subscribed channel list(if not already exist)
	//add it to the central server subscriber object
	//change the mode of the client to subscribe
	channel := req.Arguments[0]
	exist, ok := c.SubscribedChannels[channel]
	if !ok || !exist {
		c.SubscribedChannels[channel] = true
	}
	subscribers, ok := s.SubscribersMetaData[channel]
	if !ok {
		s.SubscribersMetaData[channel] = []int64{c.Id}
	}
	subscribers = append(subscribers, c.Id)
	s.SubscribersMetaData[channel] = subscribers
	c.Mode = ClientModeSubscribed
	// "subscribe" (as a RESP bulk string)
	// The channel name (as a RESP bulk string)
	// The number of channels this client has subscribed to so far (as a RESP integer)
	subscribe := (&resp.BulkString{
		Value: "subscribe",
	}).Marshal()
	channelName := (&resp.BulkString{
		Value: channel,
	}).Marshal()
	channelCount := (&resp.Integer{
		Value: int64(len(c.SubscribedChannels)),
	}).Marshal()
	arr := resp.Array{
		Value: []string{string(subscribe), string(channelName), string(channelCount)},
	}
	return arr.Marshal(), nil
}

func (s *Server) HandleUnwatch(req Request, c *Client) ([]byte, error) {
	s.UnwatchAll(c)
	ss := resp.SimpleString{
		Value: "OK",
	}
	return ss.Marshal(), nil
}

func (s *Server) HandleWatch(req Request, c *Client) ([]byte, error) {
	//check if we are already in transaction
	//if yes then throw error
	if c.Mode == ClientModeTransaction {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: "WATCH inside MULTI is not allowed",
		}
	}
	keys := make([]string, len(req.Arguments))
	copy(keys, req.Arguments)
	//if not register the keys to client watcher and
	// add client to global key watcher list
	for _, key := range keys {
		exist, ok := c.WatchedKeys[key]
		if !ok || !exist {
			c.WatchedKeys[key] = true
		}
		watcherList, ok := s.WatcherClients[key]
		if !ok {
			s.WatcherClients[key] = []int64{c.Id}
		}

		if !slices.Contains(watcherList, c.Id) {
			watcherList = append(watcherList, c.Id)
		}
	}
	//responde back with ok
	return (&resp.SimpleString{
		Value: "OK",
	}).Marshal(), nil
}
func (s *Server) HandleDiscard(req Request, c *Client) ([]byte, error) {
	if c.Mode != ClientModeTransaction {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: "DISCARD without MULTI",
		}
	}
	s.ResetTransactionState(c)
	return (&resp.SimpleString{
		Value: "OK",
	}).Marshal(), nil
}

func (s *Server) HandleMulti(req Request, c *Client) ([]byte, error) {
	//check if Mode of the client of transaction return err
	if c.Mode == ClientModeTransaction {
		return nil, &resp.SimpleError{
			Type:    resp.ERR,
			Message: "MULTI calls can not be nested",
		}
	}

	//if normal change mode to transaction and return ok response
	c.Mode = ClientModeTransaction
	return (&resp.SimpleString{
		Value: "OK",
	}).Marshal(), nil
}
func (s *Server) HandleINCR(req Request, c *Client) ([]byte, error) {
	// Key exists and has a numerical value (This stage)
	// Key doesn't exist (later stages)
	// Key exists but doesn't have a numerical value (later stages)
	key := req.Arguments[0]

	e, err := s.Store.Update(key, func(e *store.Entry) (*store.Entry, error) {
		if e == nil {
			entry := store.Entry{
				Type:  store.EntryTypeString,
				Value: "1",
			}
			return &entry, nil
		}
		if e.Type != store.EntryTypeString {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageNotInteger,
			}
		}

		value, ok := e.Value.(string)
		if !ok {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageNotInteger,
			}
		}
		val, err := strconv.Atoi(value)
		if err != nil {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageNotInteger,
			}
		}
		e.Value = strconv.Itoa(val + 1)
		return e, nil
	})
	if err != nil {
		return nil, err
	}
	value, _ := e.Value.(string)
	val, _ := strconv.Atoi(value)
	i := resp.Integer{
		Value: int64(val),
	}
	return i.Marshal(), nil
}

func (s *Server) HandleBLPop(req Request, c *Client) ([]byte, error) {
	key := req.Arguments[0]
	timeoutArg := req.Arguments[1]

	timeoutInSec, err := strconv.ParseFloat(timeoutArg, 32)
	if err != nil {
		return nil, &resp.SimpleError{
			Type:    resp.WRONGTYPE,
			Message: resp.ErrorMessageNotNumber,
		}
	}
	poppedElement, err := s.Store.BLPOP(key, timeoutInSec)
	if err != nil {
		return nil, err
	}
	if poppedElement == (store.BLPopdata{}) {
		na := resp.NullArray{}
		return na.Marshal(), nil
	}
	res := resp.Array{}
	keyBs := resp.BulkString{
		Value: poppedElement.Key,
	}
	valueBs := resp.BulkString{
		Value: poppedElement.Value,
	}

	res.Value = append(res.Value, string(keyBs.Marshal()))
	res.Value = append(res.Value, string(valueBs.Marshal()))
	return res.Marshal(), nil
}

func (s *Server) HandleLLen(req Request, c *Client) ([]byte, error) {
	//find a list by its key
	//if not exist return 0 length list
	//check if its list
	//if its a list return its length
	key := req.Arguments[0]
	list, ok := s.Store.Get(key)
	if !ok {
		result := &resp.Integer{Value: 0}
		return result.Marshal(), nil
	}

	if list.Type != store.EntryTypeList {
		return nil, &resp.SimpleError{
			Type:    resp.WRONGTYPE,
			Message: resp.ErrorMessageWrongType,
		}
	}
	values, ok := list.Value.([]string)
	if !ok {
		return nil, fmt.Errorf(ErrorMessageStringTypeCaste)
	}
	result := &resp.Integer{
		Value: int64(len(values)),
	}
	return result.Marshal(), nil
}

func (s *Server) HandleLPop(req Request, c *Client) ([]byte, error) {
	//check if length is given
	//if not length is 1
	//check if list exist or list have atleast 1 element
	//if not then return null bulk string
	//if length is greater then list length
	//then normalise the parameter length to list length
	//then pop the elments from the front
	//set the remaining list as new list
	//return the popped list

	key := req.Arguments[0]
	elementToPopCount := 1
	if len(req.Arguments) > 1 {
		parsedCount, err := strconv.Atoi(req.Arguments[1])
		if err != nil {
			return nil, &resp.SimpleError{}
		}
		elementToPopCount = parsedCount
	}
	result := make([]string, 0)
	_, err := s.Store.Update(key, func(e *store.Entry) (*store.Entry, error) {
		if e == nil {
			return nil, nil
		}
		values, ok := e.Value.([]string)
		if !ok {
			return nil, fmt.Errorf(ErrorMessageStringTypeCaste)
		}
		size := len(values)
		if size == 0 {
			return e, nil
		}
		if elementToPopCount > size {
			elementToPopCount = size
		}
		result = values[:elementToPopCount]
		values = values[elementToPopCount:]
		e.Value = values
		return e, nil
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return (&resp.NullBulkString{}).Marshal(), nil
	}
	if len(req.Arguments) == 1 {
		bs := &resp.BulkString{
			Value: result[0],
		}
		return bs.Marshal(), nil
	}
	arr := &resp.Array{}
	for _, val := range result {
		bs := &resp.BulkString{
			Value: val,
		}
		arr.Value = append(arr.Value, string(bs.Marshal()))
	}
	return arr.Marshal(), nil
}

func (s *Server) HandleLPush(req Request, c *Client) ([]byte, error) {
	listKey := req.Arguments[0]
	values := make([]string, len(req.Arguments[1:]))
	copy(values, req.Arguments[1:])
	list, err := s.Store.ListPush(listKey, values, store.Left)
	if err != nil {
		return nil, err
	}
	existing := list.Value.([]string)
	result := resp.Integer{Value: int64(len(existing))}
	return result.Marshal(), nil
}

func normalizeIndex(idx int, listLen int) int {
	if idx < 0 {
		idx = listLen + idx
		if idx < 0 {
			idx = 0
		}
	}
	if idx > listLen-1 {
		idx = listLen - 1
	}
	return idx
}
func (s *Server) HandleLRange(req Request, c *Client) ([]byte, error) {
	listKey := req.Arguments[0]
	startIdx, err := strconv.Atoi(req.Arguments[1])
	if err != nil {
		return resp.ErrNotInteger(), nil
	}
	endIdx, err := strconv.Atoi(req.Arguments[2])
	if err != nil {
		return resp.ErrNotInteger(), nil
	}

	list, ok := s.Store.Get(listKey)
	arr := resp.Array{}
	if !ok {
		return arr.Marshal(), nil
	}

	if list.Type != store.EntryTypeList {
		return resp.ErrWrongType(), nil
	}

	existing, ok := list.Value.([]string)
	if !ok {
		return nil, fmt.Errorf(ErrorMessageStringTypeCaste)
	}

	listLen := len(existing)
	if startIdx >= listLen {
		return arr.Marshal(), nil
	}

	startIdx = normalizeIndex(startIdx, listLen)
	endIdx = normalizeIndex(endIdx, listLen)
	if startIdx > endIdx {
		return arr.Marshal(), nil
	}
	for i := startIdx; i <= endIdx; i++ {
		bs := resp.BulkString{Value: existing[i]}
		arr.Value = append(arr.Value, string(bs.Marshal()))
	}
	return arr.Marshal(), nil
}
func (s *Server) HandleRPush(req Request, c *Client) ([]byte, error) {
	listKey := req.Arguments[0]
	values := req.Arguments[1:]
	list, err := s.Store.ListPush(listKey, values, store.Right)
	if err != nil {
		return nil, err
	}

	newValues := list.Value.([]string)
	result := resp.Integer{Value: int64(len(newValues))}
	return result.Marshal(), nil
}

func (s *Server) HandleKeys(req Request, c *Client) ([]byte, error) {
	if req.Arguments[0] != "*" {
		return nil, fmt.Errorf("we do not support regex/pattern other then *")
	}
	arr := resp.Array{}
	for _, key := range s.Store.Keys() {
		bs := resp.BulkString{
			Value: key,
		}
		arr.Value = append(arr.Value, string(bs.Marshal()))
	}
	return arr.Marshal(), nil
}

func (s *Server) getConfig(key string) (string, bool) {
	key = strings.ToLower(key)
	configValue := reflect.ValueOf(s.Config).Elem()
	configType := configValue.Type()
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		if field.Tag.Get("config") != key {
			continue
		}
		return fmt.Sprint(configValue.Field(i).Interface()), true
	}
	return "", false
}
func (s *Server) HandleConfig(req Request, c *Client) ([]byte, error) {

	if strings.ToLower(req.Arguments[0]) != "get" {
		return nil, fmt.Errorf("invalid parameter for command config")
	}

	arr := resp.Array{}
	configKey := strings.ToLower(req.Arguments[1])
	configValue, ok := s.getConfig(configKey)
	if !ok {
		return arr.Marshal(), nil
	}
	keyResponse := resp.BulkString{Value: configKey}
	arr.Value = append(arr.Value, string(keyResponse.Marshal()))
	valueResponse := resp.BulkString{Value: configValue}
	arr.Value = append(arr.Value, string(valueResponse.Marshal()))
	data := arr.Marshal()
	return data, nil
}

func (s *Server) HandlePing(req Request, c *Client) ([]byte, error) {
	if c.Mode == ClientModeSubscribed {
		//"pong" (encoded as a bulk string)
		// "" (empty bulk string)
		pong := (&resp.BulkString{
			Value: "pong",
		}).Marshal()
		emptyString := (&resp.BulkString{}).Marshal()
		arr := resp.Array{Value: []string{string(pong), string(emptyString)}}
		return arr.Marshal(), nil
	}

	ss := &resp.SimpleString{
		Value: "PONG",
	}

	return ss.Marshal(), nil
}

func (s *Server) HandleEcho(req Request, c *Client) ([]byte, error) {
	bs := &resp.BulkString{
		Value: req.Arguments[0],
	}
	data := bs.Marshal()
	return data, nil
}
func (s *Server) HandleSet(req Request, c *Client) ([]byte, error) {
	key := req.Arguments[0]
	value := req.Arguments[1]
	var expireAt time.Time
	for i := 2; i < len(req.Arguments); i++ {
		argument := strings.ToLower(req.Arguments[i])
		switch argument {
		case "px":
			{
				i++
				timeInMilli, err := strconv.Atoi(req.Arguments[i])
				if err != nil {
					return nil, &resp.SimpleError{
						Type:    resp.ERR,
						Message: fmt.Sprintf("Expecting a integer for expire got %s", req.Arguments[i]),
					}
				}
				expireAt = time.Now().Add(time.Millisecond * time.Duration(timeInMilli))
			}
		case "ex":
			{
				i++
				timeInSec, err := strconv.Atoi(req.Arguments[i])
				if err != nil {
					return nil, &resp.SimpleError{
						Type:    resp.ERR,
						Message: fmt.Sprintf("Expecting a integer for expire got %s", req.Arguments[i]),
					}
				}
				expireAt = time.Now().Add(time.Second * time.Duration(timeInSec))
			}
		default:
			{
				return nil, &resp.SimpleError{
					Type:    resp.ERR,
					Message: fmt.Sprintf("unknown Optional parameter  '%s'", req.Arguments[i]),
				}
			}
		}
	}

	entry := &store.Entry{
		Type:     store.EntryTypeString,
		Value:    value,
		ExpireAt: expireAt,
	}
	s.Store.Set(key, entry)
	ss := &resp.SimpleString{
		Value: "OK",
	}

	return ss.Marshal(), nil
}

func (s *Server) HandleGet(req Request, c *Client) ([]byte, error) {
	key := req.Arguments[0]
	entry, ok := s.Store.Get(key)
	if !ok {
		ns := &resp.NullBulkString{}
		return ns.Marshal(), nil
	}

	// Check if key has expired
	if !entry.ExpireAt.IsZero() && time.Now().After(entry.ExpireAt) {
		s.Store.Delete(key)
		ns := &resp.NullBulkString{}
		return ns.Marshal(), nil
	}

	//it should always work cause we are only storing strings for now
	strVal, _ := entry.Value.(string)

	bs := &resp.BulkString{
		Value: strVal,
	}

	data := bs.Marshal()
	return data, nil
}
