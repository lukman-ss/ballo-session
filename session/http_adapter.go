package session

import (
	"errors"
	"net/http"
)

// HTTPAdapter provides net/http compatible middleware.
type HTTPAdapter struct {
	manager *Manager
}

// NewHTTPAdapter creates adapter.
func NewHTTPAdapter(manager *Manager) *HTTPAdapter {
	return &HTTPAdapter{manager: manager}
}

// Middleware returns http.Handler that manages session.
func (a *HTTPAdapter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := a.manager.GetSession(r)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				// internal error, abort
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// no session found, proceed with nil session
		}
		if sess == nil {
			sess, err = a.manager.StartSession(w, r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		ctx := ContextWithSession(r.Context(), sess)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)

		if sess != nil {
			if err := a.manager.CommitSession(w, r, sess); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	})
}
