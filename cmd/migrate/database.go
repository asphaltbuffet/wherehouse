package migrate

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

const longDesc = `Rewrites all entity IDs in the wherehouse database from UUID format
to 10-character alphanumeric nanoid format.

This command is opt-in and must be run explicitly. It operates as a single
atomic transaction: either all IDs are migrated successfully or no changes
are made.

System locations receive deterministic IDs:
  Missing  -> sys0000001
  Borrowed -> sys0000002
  Loaned   -> sys0000003

All other locations and items receive new randomly generated IDs.
Both projection tables and event payload JSON are updated together.

WARNING: Back up your database before running this migration.
After migration, any external references to old UUID-format IDs will be invalid.

Examples:
  wherehouse migrate database --dry-run   # Preview migration without making changes
  wherehouse migrate database             # Run migration`

// NewDatabaseCmd returns the `migrate database` subcommand.
func NewDatabaseCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "database",
		Short: "Migrate database IDs from UUID to nanoid format",
		Long:  longDesc,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()

			return cli.MigrateDatabase(cmd, db, dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview migration without making changes")

	return cmd
}
