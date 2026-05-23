package web

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"
)

const shutdownTimeout = 5 * time.Second

// Config holds all configuration for the web server.
type Config struct {
	App    App
	Bind   string
	Port   int
	Output io.Writer // destination for the startup URL line; defaults to os.Stdout
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
	if cfg.Port == 0 {
		cfg.Port = 8080
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
			Addr:              fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port),
			Handler:           mux,
			ReadHeaderTimeout: 30 * time.Second,
			ReadTimeout:       60 * time.Second,
			WriteTimeout:      60 * time.Second,
		},
	}
	srv.registerRoutes(mux)
	return srv, nil
}
