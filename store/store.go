package store

import (
	"fmt"
	"slices"
	"sync"
	"time"
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
	HashMap   map[string]float32
	ZSkipList *SkipList
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
)

func New() *Store {
	return &Store{
		data:        map[string]*Entry{},
		waiterStore: WaiterStore{},
	}
}

func (s *Store) addWaiter(key string) *BLPopWaiter {
	waiter := BLPopWaiter{}
	waiter.channel = make(chan BLPopdata, 1)
	waiter.key = key
	waiters, ok := s.waiterStore[key]
	if !ok {
		s.waiterStore[key] = []BLPopWaiter{
			waiter,
		}
	}
	waiters = append(waiters, waiter)
	s.waiterStore[key] = waiters
	return &waiter
}

func (s *Store) LPopList(key string) {

}

func (s *Store) removeWaiter(waiter *BLPopWaiter) {
	waiters := s.waiterStore[waiter.key]
	remainingWaiters := waiters[1:]
	if len(remainingWaiters) == 0 {
		delete(s.waiterStore, waiter.key)
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
		return nil, fmt.Errorf(ErrorMessageStringTypeCaste)
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
