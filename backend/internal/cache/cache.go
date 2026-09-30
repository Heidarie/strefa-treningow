package cache

import (
	"sync"
	"time"
)

type entry struct {
	data    []byte
	expires time.Time
}

// Store is a bounded, revision-scoped cache. A database revision change evicts all entries.
type Store struct {
	mu      sync.Mutex
	version int64
	bytes   int
	items   map[string]entry
}

func (s *Store) Get(version int64, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if version < s.version {
		return nil, false
	}
	if version != s.version {
		s.items = make(map[string]entry)
		s.bytes = 0
		s.version = version
	}
	e, ok := s.items[key]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.data, true
}
func (s *Store) Set(version int64, key string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if version != s.version || len(data) > 1<<20 {
		return
	}
	if old, ok := s.items[key]; ok {
		s.bytes -= len(old.data)
	}
	if s.bytes+len(data) > 32<<20 || len(s.items) >= 512 {
		s.items = make(map[string]entry)
		s.bytes = 0
	}
	s.items[key] = entry{append([]byte(nil), data...), time.Now().Add(2 * time.Minute)}
	s.bytes += len(data)
}
