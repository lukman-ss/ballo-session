package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	bh "github.com/lukman-ss/ballo-http"
)

func TestHTTPAdapterMiddleware(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)
	adapter := NewHTTPAdapter(mgr)

	var sessionID string

	next := bh.Adapt(func(r *bh.Request) (*bh.Response, error) {
		sess, ok := SessionFromContext(r.Context())
		if !ok {
			t.Error("expected session in context")
		}
		if sess != nil {
			sessionID = sess.ID
			sess.Set("test", "value")
		}
		return bh.JSON(200, map[string]string{"status": "ok"})
	})

	handler := adapter.Middleware(next)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	handler.ServeHTTP(w, req)

	if sessionID == "" {
		t.Error("expected session ID to be set")
	}
}

func TestHTTPAdapterSessionNotFound(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	mgr := NewManager(ms, config)
	adapter := NewHTTPAdapter(mgr)

	nextCalled := false
	next := bh.Adapt(func(r *bh.Request) (*bh.Response, error) {
		nextCalled = true
		sess, _ := SessionFromContext(r.Context())
		if sess == nil {
			t.Error("expected session in context")
		}
		return bh.JSON(200, map[string]string{"status": "ok"})
	})

	handler := adapter.Middleware(next)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
}
