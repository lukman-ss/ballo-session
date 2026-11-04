package session

import (
	"sync"
	"time"
)

// MemoryStore is an in-memory session store for development/testing.
type MemoryStore struct {
	mu    sync.RWMutex
	sess  map[string]*Session
	close chan struct{}
}

// NewMemoryStore creates a new in-memory session store with GC.
func NewMemoryStore() *MemoryStore {
	ms := &MemoryStore{
		sess:  make(map[string]*Session),
		close: make(chan struct{}),
	}
	go ms.gcLoop()
	return ms
}

func (ms *MemoryStore) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.gc()
		case <-ms.close:
			return
		}
	}
}

func (ms *MemoryStore) gc() {
	now := time.Now()
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for id, s := range ms.sess {
		if now.After(s.ExpiredAt) {
			delete(ms.sess, id)
		}
	}
}

// Create stores a new session.
func (ms *MemoryStore) Create(sess *Session) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.sess[sess.ID] = sess
	return nil
}

// Get retrieves a session by ID.
func (ms *MemoryStore) Get(id string) (*Session, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	sess, ok := ms.sess[id]
	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(sess.ExpiredAt) {
		delete(ms.sess, id)
		return nil, ErrNotFound
	}
	return sess, nil
}

// Update updates a session.
func (ms *MemoryStore) Update(sess *Session) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.sess[sess.ID] = sess
	return nil
}

// Delete removes a session.
func (ms *MemoryStore) Delete(id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.sess, id)
	return nil
}

// Close stops the GC loop.
func (ms *MemoryStore) Close() error {
	close(ms.close)
	return nil
}
