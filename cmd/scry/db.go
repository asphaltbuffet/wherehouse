package scry

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// scryDB is the database interface required by the scry command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery
type scryDB interface {
	Close() error
	GetItem(ctx context.Context, itemID string) (*database.Item, error)
	GetLocation(ctx context.Context, locationID string) (*database.Location, error)
	GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
	GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
	GetSystemLocationIDs(ctx context.Context) (missingID, borrowedID, loanedID string, err error)
	ScryItem(ctx context.Context, item *database.Item) (*database.ScryResult, error)
}
