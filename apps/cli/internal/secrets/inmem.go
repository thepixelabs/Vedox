package secrets

import "sync"

// InMemoryStore is a SecretStore backed entirely by an in-process map. It is
// intended for tests and ephemeral use cases (e.g., vedox doctor running
// against a stub backend) where hitting the real OS keychain, age-encrypted
// file, or environment variable is undesirable.
//
// Tests MUST use InMemoryStore (or another test double) instead of calling
// NewKeyringStore directly. NewKeyringStore goes through the real
// github.com/zalando/go-keyring path, which on macOS mutates the user's login
// keychain and may prompt for permission in sandboxed contexts such as
// altergo, corporate MDM, or CI.
//
// Thread safety: every exported method holds an internal mutex. Concurrent
// Get/Put/Delete/List calls across goroutines are safe.
type InMemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewInMemoryStore returns an empty InMemoryStore ready for use.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{data: make(map[string][]byte)}
}

func (s *InMemoryStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return nil, &ErrNotFound{Key: key}
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (s *InMemoryStore) Put(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(value))
	copy(cp, value)
	s.data[key] = cp
	return nil
}

func (s *InMemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		return &ErrNotFound{Key: key}
	}
	delete(s.data, key)
	return nil
}

func (s *InMemoryStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	return out, nil
}
