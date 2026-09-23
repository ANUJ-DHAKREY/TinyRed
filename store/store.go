package store

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
	"tinyred/resp"
)

type Store struct {
	data        map[string]*Entry
	mu          sync.RWMutex
	waiterStore WaiterStore
}

type Entry struct {
	Type     string
	Value    any // holds string, []string, map[string]string, etc.
	ExpireAt time.Time
}

type ZSet struct {
	HashMap   map[string]float64
	ZSkipList *SkipList
}

type ZSetEntry struct {
	Member string
	Score  float64
}

const (
	EntryTypeString    string = "string"
	EntryTypeList      string = "list"
	EntryTypeSortedSet string = "sortedset"
)

type WaiterStore map[string][]BLPopWaiter

type BLPopdata struct {
	Key   string
	Value string
}

type BLPopWaiter struct {
	channel chan BLPopdata
	key     string
}

type TempPushWaiterQueue struct {
	channel chan BLPopdata
	data    BLPopdata
}

const (
	Right bool = true
	Left  bool = false
)

const (
	ErrorMessageStringTypeCaste string = "internal error: list value is not []string"
	ErrorMessageZsetTypeCaste   string = "internal error: Zset value is not *Zset"
)

func New() *Store {
	return &Store{
		data:        map[string]*Entry{},
		waiterStore: WaiterStore{},
	}
}

func (s *Store) addWaiter(key string) *BLPopWaiter {
	waiter := BLPopWaiter{
		channel: make(chan BLPopdata, 1),
		key:     key,
	}
	s.waiterStore[key] = append(s.waiterStore[key], waiter)
	return &waiter
}

func (s *Store) LPopList(key string) {

}

func (s *Store) removeWaiter(waiter *BLPopWaiter) {
	waiters := s.waiterStore[waiter.key]
	idx := -1
	for i := range waiters {
		if waiters[i].channel == waiter.channel {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	remainingWaiters := slices.Delete(waiters, idx, idx+1)
	if len(remainingWaiters) == 0 {
		delete(s.waiterStore, waiter.key)
		return
	}
	s.waiterStore[waiter.key] = remainingWaiters
}
func (s *Store) waitForResolution(waiter *BLPopWaiter, timeout float64) BLPopdata {

	if timeout == 0 {
		return <-waiter.channel
	} else {
		timeDuration := time.Duration(timeout * float64(time.Second))
		select {
		case data := <-waiter.channel:
			{
				return data
			}

		case <-time.After(timeDuration):
			{
				s.mu.Lock()
				defer s.mu.Unlock()
				s.removeWaiter(waiter)
				return BLPopdata{}
			}
		}
	}

}

func (s *Store) BLPOP(key string, timeout float64) (BLPopdata, error) {
	result := BLPopdata{}
	s.mu.Lock()
	list, ok := s.data[key]
	if !ok {
		waiter := s.addWaiter(key)
		s.mu.Unlock()
		return s.waitForResolution(waiter, timeout), nil
	}
	elements, ok := list.Value.([]string)
	if !ok {
		s.mu.Unlock()
		return result, fmt.Errorf("internal error: list value is not []string")
	}

	if len(elements) == 0 {
		waiter := s.addWaiter(key)
		s.mu.Unlock()
		return s.waitForResolution(waiter, timeout), nil
	}

	result.Key = key
	result.Value = elements[0]
	elements = elements[1:]
	list.Value = elements
	s.data[key] = list
	s.mu.Unlock()
	return result, nil
}
func (s *Store) Get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	return value, ok
}

func (s *Store) Set(key string, entry *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry
}

func (s *Store) Update(key string, fn func(*Entry) (*Entry, error)) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.data[key]
	newEntry, err := fn(entry)

	if err != nil {
		return &Entry{}, err
	}
	if newEntry != nil {
		s.data[key] = newEntry
	}

	return newEntry, nil
}

func (s *Store) SendToWaiters(responseQueue []TempPushWaiterQueue) {
	for _, res := range responseQueue {
		res.channel <- res.data
	}
}
func (s *Store) ListPush(key string, values []string, direction bool) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	waiters, ok := s.waiterStore[key]
	if direction == Left {
		slices.Reverse(values)
	}
	if ok {
		for len(waiters) > 0 {
			if len(values) == 0 {
				break
			}
			waiter := waiters[0]
			waiters = waiters[1:]
			value := values[0]
			values = values[1:]
			waiter.channel <- BLPopdata{Key: key, Value: value}
		}

		if len(waiters) > 0 {
			s.waiterStore[key] = waiters
		} else {
			delete(s.waiterStore, key)
		}

		if len(values) == 0 {
			return &Entry{Type: EntryTypeList, Value: []string{}}, nil
		}
	}

	list, ok := s.data[key]
	if !ok {
		entry := &Entry{
			Type:  EntryTypeList,
			Value: values,
		}
		s.data[key] = entry
		return entry, nil
	}

	existing, ok := list.Value.([]string)
	if !ok {
		return nil, errors.New(ErrorMessageStringTypeCaste)
	}
	if direction == Right {
		values = append(existing, values...)

	} else {
		values = append(values, existing...)
	}
	list.Value = values
	return list, nil
}
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

func (s *Store) ZAdd(key string, entries []ZSetEntry) (int64, error) {
	var membersAdded int64
	_, err := s.Update(key, func(e *Entry) (*Entry, error) {
		if e == nil {
			zset := ZSet{
				HashMap:   make(map[string]float64),
				ZSkipList: NewSkipList(),
			}
			for _, pair := range entries {
				zset.HashMap[pair.Member] = pair.Score
				zset.ZSkipList.Insert(pair.Member, pair.Score)
			}
			membersAdded = int64(len(entries))
			return &Entry{
				Type:  EntryTypeSortedSet,
				Value: zset,
			}, nil
		}

		if e.Type != EntryTypeSortedSet {
			return nil, &resp.SimpleError{
				Type:    resp.ERR,
				Message: resp.ErrorMessageWrongType,
			}
		}
		zset, ok := e.Value.(ZSet)
		if !ok {
			return nil, errors.New(ErrorMessageZsetTypeCaste)
		}
		for _, pair := range entries {
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
		return 0, err
	}
	return membersAdded, nil
}
