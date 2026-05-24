package eventbus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// validatePlaceParentTx enforces: a place entity may only be nested inside another place.
func validatePlaceParentTx(ctx context.Context, tx store.Tx, parentID string) error {
	var entityTypeStr string
	err := tx.QueryRowContext(ctx,
		`SELECT entity_type FROM entities_current WHERE entity_id = ?`,
		parentID,
	).Scan(&entityTypeStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("parent entity %q not found", parentID)
		}
		return fmt.Errorf("query parent %s: %w", parentID, err)
	}

	parentType, err := inventory.ParseEntityType(entityTypeStr)
	if err != nil {
		return fmt.Errorf("parse parent entity type: %w", err)
	}

	if parentType != inventory.EntityTypePlace {
		return fmt.Errorf("a place entity can only be nested inside another place, not %q", entityTypeStr)
	}
	return nil
}
