package add

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
func resolveLocation(ctx context.Context, db *database.Database, input string) (string, error) {
	return cli.ResolveLocation(ctx, db, input)
}
