package session

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if len(id1) != IDLen*2 { // hex encoded
		t.Errorf("expected id length %d, got %d", IDLen*2, len(id1))
	}
	if id1 == id2 {
		t.Error("expected different IDs")
	}
}

func TestSessionSetAndGet(t *testing.T) {
	sess := New()
	sess.Set("key", "value")

	v, ok := sess.Get("key")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if v != "value" {
		t.Errorf("expected 'value', got %v", v)
	}
}

func TestSessionDeleteKey(t *testing.T) {
	sess := New()
	sess.Set("key", "value")
	sess.DeleteKey("key")

	_, ok := sess.Get("key")
	if ok {
		t.Error("expected key to be deleted")
	}
}

func TestSessionIsValid(t *testing.T) {
	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	if !sess.IsValid() {
		t.Error("expected valid session")
	}

	sess.ExpiredAt = time.Now().Add(-1 * time.Second)
	if sess.IsValid() {
		t.Error("expected expired session")
	}
}

func TestDefaultCookieConfig(t *testing.T) {
	config := DefaultCookieConfig()

	if config.Name != "session_id" {
		t.Errorf("expected name 'session_id', got %s", config.Name)
	}
	if !config.Secure {
		t.Error("expected Secure true")
	}
	if !config.HttpOnly {
		t.Error("expected HttpOnly true")
	}
	if config.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSiteStrictMode, got %v", config.SameSite)
	}
	if config.MaxAge != 3600 {
		t.Errorf("expected MaxAge 3600, got %d", config.MaxAge)
	}
}

func TestMemoryStoreCreateGet(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)

	if err := ms.Create(sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ms.Get(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("expected ID %s, got %s", sess.ID, got.ID)
	}
}

func TestMemoryStoreNotFound(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	_, err := ms.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)

	if err := ms.Create(sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := ms.Delete(sess.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := ms.Get(sess.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete")
	}
}

func TestManagerStartSession(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	sess, err := mgr.StartSession(w, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session")
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	cookie := cookies[0]
	if cookie.Name != "session_id" {
		t.Errorf("expected cookie name 'session_id', got %s", cookie.Name)
	}
}

func TestManagerGetSession(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	sess, _ := mgr.StartSession(w, req)

	// Simulate next request with cookie
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.AddCookie(cookieFromRecorder(w))

	got, err := mgr.GetSession(req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("expected ID %s, got %s", sess.ID, got.ID)
	}
}

func TestManagerGetSessionNotFound(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	mgr := NewManager(ms, nil)

	req, _ := http.NewRequest("GET", "/", nil)

	_, err := mgr.GetSession(req)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestManagerDestroySession(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	sess, _ := mgr.StartSession(w, req)

	if err := mgr.DestroySession(w, req, sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := mgr.GetSession(req)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after destroy")
	}
}

func TestManagerRegenerateID(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	sess, _ := mgr.StartSession(w, req)
	sess.Set("key", "value")

	oldID := sess.ID
	if err := mgr.RegenerateID(w, req, sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.ID == oldID {
		t.Error("expected different session ID after regeneration")
	}

	// Data should be preserved
	_, ok := sess.Get("key")
	if !ok {
		t.Error("expected session data to be preserved")
	}
}

func TestSessionCookieFlags(t *testing.T) {
	config := DefaultCookieConfig()
	config.Secure = true
	config.HttpOnly = true
	config.SameSite = http.SameSiteStrictMode
	config.Path = "/test"

	ms := NewMemoryStore()
	defer ms.Close()

	mgr := NewManager(ms, config)

	req, _ := http.NewRequest("GET", "/", nil)

	// Force StartSession
	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	w := httptest.NewRecorder()
	if err := mgr.store.Create(sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mgr.setSessionCookie(w, req, sess.ID)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	cookie := cookies[0]

	if !cookie.Secure {
		t.Error("expected Secure cookie flag")
	}
	if !cookie.HttpOnly {
		t.Error("expected HttpOnly cookie flag")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSiteStrict, got %v", cookie.SameSite)
	}

	if cookie.Path != "/test" {
		t.Errorf("expected path '/test', got %s", cookie.Path)
	}
}

func TestSessionExpiry(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = -1 // expired immediately
	mgr := NewManager(ms, config)

	// Create expired session manually
	sess := New()
	sess.ExpiredAt = time.Now().Add(-1 * time.Second)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate next request with expired cookie
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.AddCookie(&http.Cookie{Name: "session_id", Value: sess.ID})

	_, err := mgr.GetSession(req2)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/", nil)
			mgr.StartSession(w, req)
		}()
	}

	wg.Wait()
}

func cookieFromRecorder(r *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range r.Result().Cookies() {
		if c.Name == "session_id" {
			return c
		}
	}
	return nil
}

type failStore struct{}

func (f *failStore) Create(sess *Session) error      { return errors.New("db error") }
func (f *failStore) Get(id string) (*Session, error) { return nil, errors.New("db error") }
func (f *failStore) Update(sess *Session) error      { return errors.New("db error") }
func (f *failStore) Delete(id string) error          { return errors.New("db error") }
func (f *failStore) Close() error                    { return nil }

func TestStoreError(t *testing.T) {
	fs := &failStore{}
	mgr := NewManager(fs, DefaultCookieConfig())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	_, err := mgr.StartSession(w, req)
	if err == nil {
		t.Fatal("expected error on failed store create")
	}

	req.AddCookie(&http.Cookie{Name: "session_id", Value: "some-id"})
	_, err = mgr.GetSession(req)
	if err == nil {
		t.Fatal("expected error on failed store get")
	}

	sess := New()
	sess.ID = "test-id"
	if err := mgr.CommitSession(w, req, sess); err == nil {
		t.Fatal("expected error on failed store update")
	}

	if err := mgr.DestroySession(w, req, sess); err == nil {
		t.Fatal("expected error on failed store delete")
	}

	if err := mgr.RegenerateID(w, req, sess); err == nil {
		t.Fatal("expected error on failed store regenerate")
	}
}
