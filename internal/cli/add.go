package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// addItemsDB is the database interface required by addItems.
// *database.Database satisfies this interface.
type addItemsDB interface {
	LocationItemQuerier
	ValidateLocationExists(ctx context.Context, locationID string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

// AddItems adds items to the database.
func AddItems(ctx context.Context, items []string, location string) error {
	db, err := OpenDatabase(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	return addItems(ctx, db, items, location)
}

// addItems is the injectable implementation used by AddItems and loadCSV.
func addItems(ctx context.Context, db addItemsDB, items []string, location string) error {
	locationID, err := ResolveLocation(ctx, db, location)
	if err != nil {
		return fmt.Errorf("invalid location: %w", err)
	}

	err = db.ValidateLocationExists(ctx, locationID)
	if err != nil {
		return fmt.Errorf("location not found: %w", err)
	}

	for _, item := range items {
		var itemID string

		itemID, err = nanoid.New()
		if err != nil {
			return fmt.Errorf("failed to generate ID: %w", err)
		}

		payload := map[string]any{
			"item_id":        itemID,
			"display_name":   item,
			"canonical_name": database.CanonicalizeString(item),
			"location_id":    locationID,
		}

		_, err = db.AppendEvent(ctx, database.ItemCreatedEvent, GetActorUserID(ctx), payload, "")
		if err != nil {
			return fmt.Errorf("failed to add %q: %w", item, err)
		}
	}

	return nil
}
