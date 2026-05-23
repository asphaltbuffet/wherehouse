package web

import (
	"fmt"
	"html/template"
	"io/fs"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// parseTemplates parses all templates from the templates/ subdirectory of fsys.
func parseTemplates(fsys fs.FS) (*template.Template, error) {
	sub, err := fs.Sub(fsys, "assets/templates")
	if err != nil {
		return nil, fmt.Errorf("sub templates: %w", err)
	}

	funcMap := template.FuncMap{
		"entityTypeIcon": entityTypeIcon,
		"statusClass":    statusClass,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(sub, "*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return tmpl, nil
}

func entityTypeIcon(t inventory.EntityType) string {
	switch t {
	case inventory.EntityTypePlace:
		return "🏠"
	case inventory.EntityTypeContainer:
		return "📦"
	case inventory.EntityTypeLeaf:
		return "🔧"
	default:
		return "•"
	}
}

func statusClass(s inventory.EntityStatus) string {
	switch s {
	case inventory.EntityStatusOk:
		return "ok"
	case inventory.EntityStatusBorrowed:
		return "borrowed"
	case inventory.EntityStatusLoaned:
		return "loaned"
	case inventory.EntityStatusMissing:
		return "missing"
	case inventory.EntityStatusRemoved:
		return ""
	default:
		return ""
	}
}
