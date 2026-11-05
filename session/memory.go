package session

import (
	"sync"
	"time"
)

// MemoryStore is an in-memory session store for development/testing.
type MemoryStore struct {
	mu        sync.RWMutex
	sess      map[string]*Session
	close     chan struct{}
	closeOnce sync.Once
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
		s.mu.RLock()
		expired := now.After(s.ExpiredAt)
		s.mu.RUnlock()
		if expired {
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
	sess, ok := ms.sess[id]
	ms.mu.RUnlock()

	if !ok {
		return nil, ErrNotFound
	}
	// Acquire session read lock for ExpiredAt and Data copy.
	sess.mu.RLock()
	expired := time.Now().After(sess.ExpiredAt)
	sessCopy := &Session{
		ID:        sess.ID,
		ExpiredAt: sess.ExpiredAt,
		data:      make(map[string]interface{}),
	}
	for k, v := range sess.data {
		sessCopy.data[k] = v
	}
	sess.mu.RUnlock()

	if expired {
		// Delete under store write lock if still expired. Re-check
		// session expiry under its own lock to avoid racing with concurrent
		// mutations of ExpiredAt.
		ms.mu.Lock()
		if s, exists := ms.sess[id]; exists {
			s.mu.RLock()
			expiredAgain := time.Now().After(s.ExpiredAt)
			s.mu.RUnlock()
			if expiredAgain {
				delete(ms.sess, id)
			}
		}
		ms.mu.Unlock()
		return nil, ErrNotFound
	}
	return sessCopy, nil
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

// Close stops the GC loop. Safe for multiple calls.
func (ms *MemoryStore) Close() error {
	ms.closeOnce.Do(func() {
		close(ms.close)
	})
	return nil
}
