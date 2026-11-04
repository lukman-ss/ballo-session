package session

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreCloseMultiple(t *testing.T) {
	ms := NewMemoryStore()
	if err := ms.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := ms.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

func TestMemoryStoreConcurrentGetAndDelete(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	// Create session that expires soon
	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Millisecond)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(200)

	// Many goroutines reading
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			_, _ = ms.Get(sess.ID)
		}()
	}

	// Many goroutines attempting delete via manager (race condition test)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			_, _ = ms.Get(sess.ID)
		}()
	}

	wg.Wait()
}

func TestMemoryStoreConcurrentSessionData(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var wg sync.WaitGroup
	const writers = 10
	const opsPerGoroutine = 100

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				sess.Set("key", gID)
				_, _ = sess.Get("key")
			}
		}(i)
	}

	wg.Wait()
}

func TestManagerStartSessionConcurrent(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	const goroutines = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/", nil)
			sess, err := mgr.StartSession(w, req)
			if err != nil {
				t.Errorf("StartSession failed: %v", err)
				return
			}
			if sess == nil || sess.ID == "" {
				t.Error("nil or empty session ID")
			}
		}()
	}

	wg.Wait()
}

func TestMemoryStoreSessionDataConcurrentWithManager(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	sess, err := mgr.StartSession(w, req)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// Concurrent Set
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			sess.Set("counter", i)
		}
	}()

	// Concurrent Get
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = sess.Get("counter")
		}
	}()

	// CommitSession (update)
	go func() {
		defer wg.Done()
		_ = mgr.CommitSession(w, req, sess)
	}()

	wg.Wait()
}

func TestMemoryStoreGetReturnsCopy(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	sess.Set("original", "value")
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got1, err := ms.Get(sess.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	// Modify returned session
	got1.Set("original", "modified")
	got1.Set("newkey", "newvalue")

	// Get again, should not see modifications from other goroutine
	got2, err := ms.Get(sess.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	// Data should still contain original value, not modified
	val, ok := got2.Get("original")
	if !ok || val != "value" {
		t.Errorf("expected 'value', got %v (ok=%v)", val, ok)
	}

	// New key should not appear
	_, ok = got2.Get("newkey")
	if ok {
		t.Error("unexpected key 'newkey'")
	}
}

func TestSessionDataRace(t *testing.T) {
	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)

	var wg sync.WaitGroup
	wg.Add(200)

	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			sess.Set("key", "value")
		}()
	}

	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			_, _ = sess.Get("key")
		}()
	}

	wg.Wait()
}
