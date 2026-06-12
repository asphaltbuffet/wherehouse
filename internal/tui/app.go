package tui

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// App is the subset of app.App the TUI requires.
type App interface {
	GetRootEntities(ctx context.Context) ([]app.EntityResult, error)
	GetChildren(ctx context.Context, parentID string) ([]app.EntityResult, error)
}
