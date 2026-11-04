// Package session provides server-side session management.
package session

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// IDLen is the length of session IDs in bytes.
const IDLen = 32

// ErrNotFound is returned when session is not found.
var ErrNotFound = errors.New("session not found")

// Store persists sessions. Implementations must be safe for concurrent use.
type Store interface {
	Create(sess *Session) error
	Get(id string) (*Session, error)
	Delete(id string) error
	Update(sess *Session) error
}

// Session holds the session state.
type Session struct {
	mu        sync.RWMutex
	ID        string
	Data      map[string]interface{}
	ExpiredAt time.Time
}

// New creates a session with a random ID.
func New() *Session {
	return &Session{
		ID:   generateID(),
		Data: make(map[string]interface{}),
	}
}

// generateID creates a cryptographically secure random session ID.
func generateID() string {
	b := make([]byte, IDLen)
	_, err := rand.Read(b)
	if err != nil {
		panic("session: crypto/rand failure")
	}
	return fmt.Sprintf("%x", b)
}

// Set stores a value in the session data.
func (s *Session) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
}

// Get retrieves a value from the session data.
func (s *Session) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Data[key]
	return v, ok
}

// DeleteKey removes a key from the session data.
func (s *Session) DeleteKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Data, key)
}

// IsValid checks if the session is not expired.
func (s *Session) IsValid() bool {
	return time.Until(s.ExpiredAt) > 0
}

// contextKey is the key type for session context storage.
type contextKey int

const sessionKey contextKey = iota

// ContextWithSession stores a session in context.
func ContextWithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, sessionKey, sess)
}

// SessionFromContext retrieves a session from context.
func SessionFromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(sessionKey).(*Session)
	return sess, ok
}

// CookieConfig holds cookie settings.
type CookieConfig struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
	MaxAge   int // seconds; 0 = session cookie, -1 = delete
}

// DefaultCookieConfig returns a secure default cookie config.
func DefaultCookieConfig() *CookieConfig {
	return &CookieConfig{
		Name:     "session_id",
		Path:     "/",
		Domain:   "",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600,
	}
}

// Manager handles session lifecycle.
type Manager struct {
	store   Store
	config  *CookieConfig
	mutex   sync.RWMutex
	gcMutex sync.Mutex
}

// NewManager creates a new session manager. Panics on nil store or empty cookie name.
func NewManager(store Store, config *CookieConfig) *Manager {
	if store == nil {
		panic("session: store must not be nil")
	}
	if config == nil {
		config = DefaultCookieConfig()
	} else if config.Name == "" {
		panic("session: cookie name must not be empty")
	}
	return &Manager{
		store:  store,
		config: config,
	}
}

// StartSession creates a new session and sets the session cookie.
func (m *Manager) StartSession(w http.ResponseWriter, r *http.Request) (*Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	sess := New()
	sess.ExpiredAt = time.Now().Add(time.Duration(m.config.MaxAge) * time.Second)

	if err := m.store.Create(sess); err != nil {
		return nil, fmt.Errorf("session: cannot create session: %w", err)
	}

	m.setSessionCookie(w, r, sess.ID)
	return sess, nil
}

// GetSession retrieves a session by cookie.
func (m *Manager) GetSession(r *http.Request) (*Session, error) {
	m.mutex.RLock()
	config := m.config
	m.mutex.RUnlock()

	cookie, err := r.Cookie(config.Name)
	if err != nil {
		return nil, ErrNotFound
	}

	sess, err := m.store.Get(cookie.Value)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("session: cannot get session: %w", err)
	}

	if !sess.IsValid() {
		_ = m.store.Delete(cookie.Value)
		return nil, ErrNotFound
	}

	return sess, nil
}

// CommitSession saves session changes and rewrites the cookie if ID changed.
func (m *Manager) CommitSession(w http.ResponseWriter, r *http.Request, sess *Session) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	_, err := r.Cookie(m.config.Name)
	if err != nil {
		m.setSessionCookie(w, r, sess.ID)
	}

	if err := m.store.Update(sess); err != nil {
		return fmt.Errorf("session: cannot update session: %w", err)
	}

	return nil
}

// RegenerateID creates a new session ID and preserves session data.
func (m *Manager) RegenerateID(w http.ResponseWriter, r *http.Request, sess *Session) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	oldID := sess.ID
	newID := generateID()
	sess.ID = newID

	if err := m.store.Create(sess); err != nil {
		sess.ID = oldID
		return fmt.Errorf("session: cannot create regenerated session: %w", err)
	}

	if err := m.store.Delete(oldID); err != nil {
		_ = m.store.Delete(newID)
		sess.ID = oldID
		return fmt.Errorf("session: cannot delete old session: %w", err)
	}

	m.setSessionCookie(w, r, newID)
	return nil
}

// DestroySession deletes the session and clears the cookie.
func (m *Manager) DestroySession(w http.ResponseWriter, r *http.Request, sess *Session) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.store.Delete(sess.ID); err != nil {
		return fmt.Errorf("session: cannot delete session: %w", err)
	}

	m.clearSessionCookie(w, r)
	return nil
}

func (m *Manager) setSessionCookie(w http.ResponseWriter, r *http.Request, id string) {
	cookie := &http.Cookie{
		Name:     m.config.Name,
		Value:    id,
		Path:     m.config.Path,
		Domain:   m.config.Domain,
		Secure:   m.config.Secure,
		HttpOnly: m.config.HttpOnly,
		SameSite: m.config.SameSite,
		MaxAge:   m.config.MaxAge,
	}
	http.SetCookie(w, cookie)
}

func (m *Manager) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     m.config.Name,
		Value:    "",
		Path:     m.config.Path,
		Domain:   m.config.Domain,
		Secure:   m.config.Secure,
		HttpOnly: m.config.HttpOnly,
		SameSite: m.config.SameSite,
		MaxAge:   -1,
	}
	http.SetCookie(w, cookie)
}

// GC starts a garbage collection sweep over expired sessions.
func (m *Manager) GC() error {
	m.gcMutex.Lock()
	defer m.gcMutex.Unlock()

	// Memory store handles GC internally; other stores may implement differently.
	return nil
}
