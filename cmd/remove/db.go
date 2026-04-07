package remove

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// removeDB is the database interface required by the remove command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery
type removeDB interface {
	Close() error
	GetItem(ctx context.Context, itemID string) (*database.Item, error)
	GetLocation(ctx context.Context, locationID string) (*database.Location, error)
	GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
	GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
	GetItemsByLocation(ctx context.Context, locationID string) ([]*database.Item, error)
	GetLocationChildren(ctx context.Context, parentID string) ([]*database.Location, error)
	ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}
