package migrate

import "github.com/spf13/cobra"

var migrateCmd *cobra.Command

// GetMigrateCmd returns the parent migrate command.
func GetMigrateCmd() *cobra.Command {
	if migrateCmd != nil {
		return migrateCmd
	}

	migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "run data migration operations",
		Long: `The migrate command provides subcommands for migrating wherehouse data.

Examples:
  wherehouse migrate database        Migrate IDs from UUID to nanoid format`,
	}

	migrateCmd.AddCommand(GetDatabaseCmd())

	return migrateCmd
}
