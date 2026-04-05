package list

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// listDB is the database interface required by the list command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery
type listDB interface {
	Close() error
	GetItem(ctx context.Context, itemID string) (*database.Item, error)
	GetLocation(ctx context.Context, locationID string) (*database.Location, error)
	GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
	GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
	GetRootLocations(ctx context.Context) ([]*database.Location, error)
	GetItemsByLocation(ctx context.Context, locationID string) ([]*database.Item, error)
	GetLocationChildren(ctx context.Context, locationID string) ([]*database.Location, error)
}
