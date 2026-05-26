package web

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/logging"
)

const (
	shutdownTimeout   = 5 * time.Second
	readHeaderTimeout = 30 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 60 * time.Second
)

// Config holds all configuration for the web server.
type Config struct {
	App    App
	Bind   string
	Port   int
	Output io.Writer      // destination for the startup URL line; defaults to os.Stdout
	Logger logging.Logger // structured logger; defaults to logging.GetLogger()
}

// Server is the wherehouse web server.
type Server struct {
	cfg       Config
	httpSrv   *http.Server
	templates *template.Template
}

// New constructs a Server from cfg. Returns an error if template parsing fails.
func New(cfg Config) (*Server, error) {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if cfg.Bind == "" {
		cfg.Bind = "127.0.0.1"
	}
	if cfg.Logger == nil {
		cfg.Logger = logging.GetLogger()
	}

	tmpl, err := parseTemplates(assetsFS)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	mux := http.NewServeMux()
	srv := &Server{
		cfg:       cfg,
		templates: tmpl,
		httpSrv: &http.Server{
			Addr:              net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port)),
			Handler:           securityHeaders(limitBody(csrfGuard(mux))),
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
		},
	}
	srv.registerRoutes(mux)
	return srv, nil
}

// Run starts the HTTP server, prints the URL, and blocks until ctx is cancelled
// or an OS interrupt (SIGINT/SIGTERM) is received, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	addr := "http://" + net.JoinHostPort(s.cfg.Bind, strconv.Itoa(s.cfg.Port))
	fmt.Fprintln(s.cfg.Output, "Listening on", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// Handler returns the underlying [http.Handler] (for use in httptest).
func (s *Server) Handler() http.Handler {
	return s.httpSrv.Handler
}
