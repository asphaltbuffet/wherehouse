// Package history implements the history command for displaying entity event history.
package history

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type historyDB interface {
	Close() error
	GetEventsByEntity(ctx context.Context, entityID string) ([]*database.Event, error)
}

// Compile-time check that *database.Database satisfies historyDB.
var _ historyDB = (*database.Database)(nil)
