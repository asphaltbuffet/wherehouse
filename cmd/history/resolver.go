package history

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// resolveItemSelector converts a selector (name or location:name) to item UUID.
// Returns error if selector is ambiguous or not found.
func resolveItemSelector(ctx context.Context, db *database.Database, selector string) (string, error) {
	return cli.ResolveItemSelector(ctx, db, selector, "wherehouse history")
}
