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
	mux.HandleFunc("GET /entities/parent-options", s.handleParentOptions)
	mux.HandleFunc("GET /entities/borrow", s.handleBorrowForm)
	mux.HandleFunc("POST /entities/borrow", s.handleBorrow)
	mux.HandleFunc("GET /entities/{entityID}/tree-node", s.handleTreeNode)
	mux.HandleFunc("POST /entities/add", s.handleRootAddItem)
	mux.HandleFunc("GET /entities/{entityID}/edit/name", s.handleEditNameForm)
	mux.HandleFunc("POST /entities/{entityID}/edit/name", s.handleEditName)
	mux.HandleFunc("GET /entities/{entityID}/actions/tag", s.handleTagForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/tag", s.handleTag)
	mux.HandleFunc("POST /entities/{entityID}/actions/untag", s.handleUntag)
	mux.HandleFunc("GET /entities/{entityID}/actions/lost", s.handleLostForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/lost", s.handleLost)
	mux.HandleFunc("GET /entities/{entityID}/actions/found", s.handleFoundForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/found", s.handleFound)
	mux.HandleFunc("POST /entities/{entityID}/actions/attr", s.handleAttr)
	mux.HandleFunc("GET /entities/{entityID}/actions/return", s.handleReturnForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/return", s.handleReturn)
	mux.HandleFunc("GET /entities/{entityID}/actions/loan", s.handleLoanForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/loan", s.handleLoan)
	mux.HandleFunc("GET /entities/{entityID}/actions/move", s.handleMoveForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/move", s.handleMove)
	mux.HandleFunc("GET /entities/{entityID}/actions/use", s.handleUseForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/use", s.handleUse)
	mux.HandleFunc("GET /entities/{entityID}/actions/remove", s.handleRemoveForm)
	mux.HandleFunc("POST /entities/{entityID}/actions/remove", s.handleRemove)
	mux.HandleFunc("GET /search", s.handleSearch)
	mux.HandleFunc("GET /reports", s.handleReports)
}
