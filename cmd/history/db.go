package history

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// historyDB is the database interface required by the history command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery
type historyDB interface {
	Close() error
	GetItem(ctx context.Context, itemID string) (*database.Item, error)
	GetLocation(ctx context.Context, locationID string) (*database.Location, error)
	GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
	GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
	GetEventsByEntity(ctx context.Context, itemID, locationID *string) ([]*database.Event, error)
}
