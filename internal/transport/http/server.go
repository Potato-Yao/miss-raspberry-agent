package http

import (
	"context"
	"errors"
	stdhttp "net/http"
)

// Server wraps net/http.Server with a small lifecycle that plays nicely with context
// cancellation.
type Server struct {
	server *stdhttp.Server
}

// NewServer creates a server that listens on addr and serves handler.
func NewServer(addr string, handler stdhttp.Handler) *Server {
	return &Server{server: &stdhttp.Server{Addr: addr, Handler: handler}}
}

// Start blocks until the server stops. A graceful Shutdown makes it return nil.
func (s *Server) Start() error {
	err := s.server.ListenAndServe()
	if errors.Is(err, stdhttp.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server, waiting for in-flight requests until ctx is done.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
