package migrate

import "github.com/spf13/cobra"

// NewMigrateCmd returns the parent migrate command.
func NewMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run data migration operations",
		Long: `The migrate command provides subcommands for migrating wherehouse data.

Examples:
  wherehouse migrate database        # Migrate IDs from UUID to nanoid format`,
	}

	cmd.AddCommand(NewDatabaseCmd())

	return cmd
}
