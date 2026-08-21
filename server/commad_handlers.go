package server

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"tinyred/resp"
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
	LLEN   Command = "llen "
	BLPOP  Command = "blpop"
	LRANGE Command = "lrange"
)

type CommandEntry struct {
	Handler func(Request) ([]byte, error)
	MinArgs int
	MaxArgs int
}

func (s *Server) HandleBLPop(req Request) ([]byte, error) {
	ss := &resp.SimpleString{
		Value: "PONG",
	}

	return ss.Marshal(), nil
}
func (s *Server) HandleLLen(req Request) ([]byte, error) {
	ss := &resp.SimpleString{
		Value: "PONG",
	}

	return ss.Marshal(), nil
}
func (s *Server) HandleLPop(req Request) ([]byte, error) {
	ss := &resp.SimpleString{
		Value: "PONG",
	}

	return ss.Marshal(), nil
}
func (s *Server) HandleLPush(req Request) ([]byte, error) {
	listKey := req.Arguments[0]
	values := make([]string, len(req.Arguments[1:]))
	copy(values, req.Arguments[1:])
	slices.Reverse(values)
	list, ok := s.Store.Get(listKey)
	if !ok {
		entry := &resp.Entry{
			Type:  resp.EntryTypeList,
			Value: values,
		}
		s.Store.Set(listKey, entry)
		result := resp.Integer{Value: int64(len(values))}
		return result.Marshal(), nil
	}

	if list.Type != resp.EntryTypeList {
		return resp.ErrWrongType(), nil
	}
	existing, ok := list.Value.([]string)
	if !ok {
		return nil, fmt.Errorf("internal error: list value is not []string")
	}

	existing = append(values, existing...)
	list.Value = existing
	s.Store.Set(listKey, list)
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
	if !ok {
		arr := resp.Array{}
		return arr.Marshal(), nil
	}

	if list.Type != resp.EntryTypeList {
		return resp.ErrWrongType(), nil
	}

	existing, ok := list.Value.([]string)
	if !ok {
		return nil, fmt.Errorf("internal error: list value is not []string")
	}

	listLen := len(existing)
	startIdx = normalizeIndex(startIdx, listLen)
	endIdx = normalizeIndex(endIdx, listLen)

	arr := resp.Array{}
	for i := startIdx; i <= endIdx; i++ {
		bs := resp.BulkString{Value: existing[i]}
		arr.Value = append(arr.Value, string(bs.Marshal()))
	}
	return arr.Marshal(), nil
}
func (s *Server) HandleRPush(req Request) ([]byte, error) {
	listKey := req.Arguments[0]
	values := req.Arguments[1:]

	list, ok := s.Store.Get(listKey)
	if !ok {
		entry := &resp.Entry{
			Type:  resp.EntryTypeList,
			Value: values,
		}
		s.Store.Set(listKey, entry)
		result := resp.Integer{Value: int64(len(values))}
		return result.Marshal(), nil
	}

	if list.Type != resp.EntryTypeList {
		return resp.ErrWrongType(), nil
	}
	existing, ok := list.Value.([]string)
	if !ok {
		return nil, fmt.Errorf("internal error: list value is not []string")
	}
	existing = append(existing, values...)
	list.Value = existing
	s.Store.Set(listKey, list)
	result := resp.Integer{Value: int64(len(existing))}
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

func (s *Server) HandleConfig(req Request) ([]byte, error) {

	if strings.ToLower(req.Arguments[0]) != "get" {
		return nil, fmt.Errorf("invalid parameter for command config")
	}

	if strings.ToLower(req.Arguments[1]) != ConfigDir && strings.ToLower(req.Arguments[1]) != ConfigDbfilename {
		return nil, fmt.Errorf("invalid value parameter for config get")
	}

	arr := resp.Array{}
	switch strings.ToLower(req.Arguments[1]) {
	case ConfigDir:
		bs := resp.BulkString{
			Value: ConfigDir,
		}
		arr.Value = append(arr.Value, string(bs.Marshal()))
		if s.Config.Dir == "" {
			nullStr := resp.NullBulkString{}
			arr.Value = append(arr.Value, string(nullStr.Marshal()))
		} else {
			bs2 := resp.BulkString{
				Value: s.Config.Dir,
			}
			arr.Value = append(arr.Value, string(bs2.Marshal()))
		}
	case ConfigDbfilename:
		bs := resp.BulkString{
			Value: ConfigDbfilename,
		}
		arr.Value = append(arr.Value, string(bs.Marshal()))
		if s.Config.Dbfilename == "" {
			nullStr := resp.NullBulkString{}
			arr.Value = append(arr.Value, string(nullStr.Marshal()))
		} else {
			bs2 := resp.BulkString{
				Value: s.Config.Dbfilename,
			}
			arr.Value = append(arr.Value, string(bs2.Marshal()))
		}
	}
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
