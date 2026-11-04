package session

import (
	"net/http"
	"testing"

	"github.com/lukman-ss/ballo"
)

func TestBalloAdapterMiddleware(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)
	adapter := NewBalloAdapter(mgr)

	var sessionID string

	handler := func(req *ballo.Request) (*ballo.Response, error) {
		sess, ok := SessionFromContext(req.Context)
		if !ok {
			t.Error("expected session in context")
		}
		if sess != nil {
			sessionID = sess.ID
			sess.Set("test", "value")
		}
		return &ballo.Response{StatusCode: 200, Body: []byte("ok")}, nil
	}

	mw := adapter.Middleware()
	wrapped := mw(handler)

	req, _ := http.NewRequest("GET", "/", nil)

	// First request - no cookie, should create session
	balloReq := &ballo.Request{Request: req, Context: req.Context()}
	resp, err := wrapped(balloReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Check session created
	if sessionID == "" {
		t.Error("expected session ID to be set")
	}
}

func TestBalloAdapterSessionFromContext(t *testing.T) {
	ms := NewMemoryStore()
	defer ms.Close()

	config := DefaultCookieConfig()
	config.MaxAge = 3600
	mgr := NewManager(ms, config)
	adapter := NewBalloAdapter(mgr)

	handler := func(req *ballo.Request) (*ballo.Response, error) {
		sess, ok := SessionFromContext(req.Context)
		if !ok {
			t.Fatal("expected session in context")
		}
		if sess == nil {
			t.Fatal("expected non-nil session")
		}
		return &ballo.Response{StatusCode: 200}, nil
	}

	mw := adapter.Middleware()
	wrapped := mw(handler)

	req, _ := http.NewRequest("GET", "/", nil)

	balloReq := &ballo.Request{Request: req, Context: req.Context()}
	resp, _ := wrapped(balloReq)

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}
