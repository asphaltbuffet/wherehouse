package app

import (
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// App is the top-level application handle.
// Create one per process via New and share it across all callers.
type App struct {
	store *store.Store
	bus   *eventbus.Bus
}

// New creates an App from an open store.
func New(s *store.Store) *App {
	return &App{
		store: s,
		bus:   eventbus.New(s),
	}
}
