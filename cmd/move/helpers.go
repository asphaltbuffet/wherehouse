package move

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// openDatabase opens the database connection using config settings.
func openDatabase(ctx context.Context) (*database.Database, error) {
	return cli.OpenDatabase(ctx)
}

// resolveLocation attempts to resolve a name or UUID to a location UUID.
// Accepts either:
// - Full UUID (verified against database).
// - Display name or canonical name (looked up in projection).
func resolveLocation(ctx context.Context, db moveDB, input string) (string, error) {
	return cli.ResolveLocation(ctx, db, input)
}

// resolveItemSelector resolves an item selector to an item UUID.
// Supports three selector types:
//  1. UUID (exact ID)
//  2. LOCATION:ITEM (both canonical names, filters by location)
//  3. Canonical name (must match exactly 1 item)
func resolveItemSelector(ctx context.Context, db moveDB, selector string) (string, error) {
	return cli.ResolveItemSelector(ctx, db, selector, "wherehouse move")
}
