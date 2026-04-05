package lost

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// openDatabase opens the database connection using config settings.
func openDatabase(ctx context.Context) (*database.Database, error) {
	return cli.OpenDatabase(ctx)
}

// resolveItemSelector resolves an item selector to an item ID.
// Supports three selector types:
//  1. ID (exact match)
//  2. LOCATION:ITEM (both canonical names, filters by location)
//  3. Canonical name (must match exactly 1 item)
func resolveItemSelector(ctx context.Context, db *database.Database, selector string) (string, error) {
	return cli.ResolveItemSelector(ctx, db, selector, "wherehouse lost")
}
