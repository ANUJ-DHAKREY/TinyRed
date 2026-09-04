package server

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"tinyred/resp"
	"tinyred/store"
)

const (
	ECHO   Command = "echo"
	SET    Command = "set"
	GET    Command = "get"
	PING   Command = "ping"
	CONFIG Command = "config"
	KEYS   Command = "keys"
	RPUSH  Command = "rpush"
	LPUSH  Command = "lpush"
	LPOP   Command = "lpop"
	LLEN   Command = "llen"
	BLPOP  Command = "blpop"
	LRANGE Command = "lrange"
)

type CommandEntry struct {
	Handler func(Request) ([]byte, error)
	MinArgs int
	MaxArgs int
}

func (s *Server) HandleBLPop(req Request) ([]byte, error) {
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

func (s *Server) HandleLLen(req Request) ([]byte, error) {
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

	if list.Type != resp.EntryTypeList {
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

func (s *Server) HandleLPop(req Request) ([]byte, error) {
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
	_, err := s.Store.Update(key, func(e *resp.Entry) (*resp.Entry, error) {
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

func (s *Server) HandleLPush(req Request) ([]byte, error) {
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
func (s *Server) HandleLRange(req Request) ([]byte, error) {
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

	if list.Type != resp.EntryTypeList {
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
func (s *Server) HandleRPush(req Request) ([]byte, error) {
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

func (s *Server) HandleKeys(req Request) ([]byte, error) {
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
func (s *Server) HandleConfig(req Request) ([]byte, error) {

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

func (s *Server) HandlePing(req Request) ([]byte, error) {
	ss := &resp.SimpleString{
		Value: "PONG",
	}

	return ss.Marshal(), nil
}

func (s *Server) HandleEcho(req Request) ([]byte, error) {
	bs := &resp.BulkString{
		Value: req.Arguments[0],
	}
	data := bs.Marshal()
	return data, nil
}
func (s *Server) HandleSet(req Request) ([]byte, error) {
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

	entry := &resp.Entry{
		Type:     resp.EntryTypeString,
		Value:    value,
		ExpireAt: expireAt,
	}
	s.Store.Set(key, entry)
	ss := &resp.SimpleString{
		Value: "OK",
	}

	return ss.Marshal(), nil
}

func (s *Server) HandleGet(req Request) ([]byte, error) {
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

type Command string
