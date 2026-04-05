package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// AddItems adds a items to the database.
func AddItems(ctx context.Context, items []string, location string) error {
	db, err := OpenDatabase(ctx)
	if err != nil {
		return err
	}

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
