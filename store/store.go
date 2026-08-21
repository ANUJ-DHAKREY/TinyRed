package store

import (
	"sync"
	"tinyred/resp"
)

type Store struct {
	Data map[string]*resp.Entry
	mu   sync.RWMutex
}

func (s *Store) Get(key string) (*resp.Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.Data[key]
	return value, ok
}

func (s *Store) Set(key string, entry *resp.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = entry
}

func (s *Store) Update(key string, entry *resp.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = entry
}
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Data, key)
}

func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.Data))
	for k := range s.Data {
		keys = append(keys, k)
	}
	return keys
}
