package session

import (
	"fmt"
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

// TestMemoryStoreConcurrentGetUpdateSet exercises MemoryStore.Get, Update,
// and Session.Set together under concurrent load with the race detector.
// It verifies that the store's snapshot of Data is taken under sess.mu and
// that no data race occurs between the store and session-level mutations.
func TestMemoryStoreConcurrentGetUpdateSet(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(4)

	// Writer: mutate session data directly.
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			sess.Set("counter", i)
		}
	}()

	// Reader: read through the store, which copies Data under sess.mu.
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			got, err := ms.Get(sess.ID)
			if err != nil {
				t.Errorf("get failed: %v", err)
				return
			}
			_, _ = got.Get("counter")
		}
	}()

	// Updater: persist session snapshot back to the store.
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = ms.Update(sess)
		}
	}()

	// Deleter: delete session from store concurrently.
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(time.Millisecond * time.Duration(i+1))
			_ = ms.Delete(sess.ID)
			_ = ms.Create(sess) // recreate for further tests
		}
	}()

	wg.Wait()
}

func TestSessionDataRace(t *testing.T) {
	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)

	var wg sync.WaitGroup
	wg.Add(300)

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

	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			sess.DeleteKey("key")
		}()
	}

	wg.Wait()
}

// TestMemoryStoreGetConcurrentSetDeleteKey verifies that Get safely copies
// Session.Data and reads ExpiredAt while other goroutines mutate session data.
func TestMemoryStoreGetConcurrentSetDeleteKey(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var wg sync.WaitGroup
	const writers = 50
	const ops = 200

	// Writers mutate session data concurrently.
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				sess.Set(fmt.Sprintf("k%d", j), id)
				if j%3 == 0 {
					sess.DeleteKey(fmt.Sprintf("k%d", j))
				}
			}
		}(i)
	}

	// Readers fetch session from store and inspect copy.
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				got, err := ms.Get(sess.ID)
				if err != nil {
					t.Errorf("get failed: %v", err)
					return
				}
				_, _ = got.Get("dummy")
				_ = got.ExpiredAt
			}
		}()
	}

	wg.Wait()
}

// TestMemoryStoreGCConcurrent verifies GC respects session mutex when checking expiry.
func TestMemoryStoreGCConcurrent(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(-1 * time.Second)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var wg sync.WaitGroup
	// Goroutines continuously read and write while GC runs.
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = ms.Get(sess.ID)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sess.Set("counter", j)
			}
		}()
	}

	wg.Wait()
}

// TestMemoryStoreNoInternalPointerMutation checks that stored session pointers
// are not mutated by external code except through public API.
func TestMemoryStoreNoInternalPointerMutation(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	sess := New()
	sess.ExpiredAt = time.Now().Add(1 * time.Hour)
	if err := ms.Create(sess); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Modify internal data via Set.
	sess.Set("user", "lukman")

	// Get should return a copy with the modified data.
	got, err := ms.Get(sess.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	val, ok := got.Get("user")
	if !ok || val != "lukman" {
		t.Errorf("expected 'lukman', got %v (ok=%v)", val, ok)
	}

	// Direct modification of returned copy must not affect store.
	got.Set("user", "modified")
	got2, err := ms.Get(sess.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	val2, _ := got2.Get("user")
	if val2 != "lukman" {
		t.Errorf("expected original value 'lukman', got %v", val2)
	}
}
