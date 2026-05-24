package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets
var assetsFS embed.FS

// registerRoutes wires all URL patterns onto mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	staticSub, _ := fs.Sub(assetsFS, "assets/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /tree/{entityID}/children", s.handleTreeChildren)
	mux.HandleFunc("GET /entities/{entityID}", s.handleEntityDetail)
}
