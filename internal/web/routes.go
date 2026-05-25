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
	mux.HandleFunc("GET /entities/{parentID}/add", s.handleAddItemForm)
	mux.HandleFunc("POST /entities/{parentID}/add", s.handleAddItem)
	mux.HandleFunc("GET /entities/add", s.handleRootAddItemForm)
	mux.HandleFunc("POST /entities/add", s.handleRootAddItem)
	mux.HandleFunc("GET /entities/{entityID}/edit/name", s.handleEditNameForm)
	mux.HandleFunc("GET /entities/{entityID}/edit/status", s.handleEditStatusForm)
	mux.HandleFunc("POST /entities/{entityID}/edit/name", s.handleEditName)
	mux.HandleFunc("POST /entities/{entityID}/edit/status", s.handleEditStatus)
	mux.HandleFunc("GET /search", s.handleSearch)
}
