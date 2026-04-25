package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// MigrateDatabase is a stub retained for interface compatibility.
// The UUID-to-nanoid ID migration is no longer applicable in the entity model.
// This function will be removed or replaced in a future task.
func MigrateDatabase(_ *cobra.Command, _ *database.Database, _ bool) error {
	return errors.New(
		"migrate database: ID migration is not applicable in the entity model (entities_current uses nanoid IDs natively)",
	)
}
