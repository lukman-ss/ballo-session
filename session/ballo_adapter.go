package session

import (
	"errors"
	"net/http"

	"github.com/lukman-ss/ballo"
)

// BalloAdapter wraps session Manager for Ballo framework.
type BalloAdapter struct {
	manager *Manager
}

// NewBalloAdapter creates a Ballo adapter.
func NewBalloAdapter(manager *Manager) *BalloAdapter {
	return &BalloAdapter{manager: manager}
}

// Middleware wraps a Ballo Handler to manage session lifecycle.
func (a *BalloAdapter) Middleware() func(ballo.Handler) ballo.Handler {
	return func(next ballo.Handler) ballo.Handler {
		return func(req *ballo.Request) (*ballo.Response, error) {
			sess, err := a.manager.GetSession(req.Request)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return nil, err
			}

			if sess != nil {
				ctx := ContextWithSession(req.Context, sess)
				req.Context = ctx
			}

			// Ensure session exists; create if missing
			if sess == nil {
				sess, err = a.manager.StartSession(&responseWriterAdapter{Response: &ballo.Response{}}, req.Request)
				if err != nil {
					return nil, err
				}
				ctx := ContextWithSession(req.Context, sess)
				req.Context = ctx
			}

			resp, err := next(req)
			if err != nil {
				return resp, err
			}

			if sess != nil {
				if err := a.manager.CommitSession(&responseWriterAdapter{Response: resp}, req.Request, sess); err != nil {
					return nil, err
				}
			}

			return resp, nil
		}
	}
}

// responseWriterAdapter implements http.ResponseWriter from *ballo.Response.
type responseWriterAdapter struct {
	*ballo.Response
}

func (a *responseWriterAdapter) Header() http.Header {
	if a.Response.Header == nil {
		a.Response.Header = make(http.Header)
	}
	return a.Response.Header
}

func (a *responseWriterAdapter) Write(b []byte) (int, error) {
	if a.Response.Body == nil {
		a.Response.Body = b
	} else {
		a.Response.Body = append(a.Response.Body, b...)
	}
	return len(b), nil
}

func (a *responseWriterAdapter) WriteHeader(statusCode int) {
	a.Response.StatusCode = statusCode
}
