package list

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// openDatabase opens the database connection using config settings.
func openDatabase(ctx context.Context) (*database.Database, error) {
	if testOpenDatabase != nil {
		return testOpenDatabase(ctx)
	}
	return cli.OpenDatabase(ctx)
}

// resolveLocation resolves a location name or ID to the location ID string.
// Accepts either a full ID (verified against database) or a display/canonical name.
// Returns the location ID string or error if not found.
func resolveLocation(ctx context.Context, db *database.Database, input string) (string, error) {
	return cli.ResolveLocation(ctx, db, input)
}
