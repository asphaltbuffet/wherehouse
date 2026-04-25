package status

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// NewDefaultStatusCmd returns a status command that opens the database from context configuration at runtime.
func NewDefaultStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <entity-id>",
		Short: "Change the status of an entity",
		Long: `Change the status of an entity.

Valid statuses: ok, borrowed, missing, loaned, removed

Examples:
  wherehouse status <id> --set loaned --note "loaned to Alice"
  wherehouse status <id> --set ok`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runStatus(cmd, args, db)
		},
	}
	cmd.Flags().StringP("set", "s", "", "New status: ok, borrowed, missing, loaned, removed (REQUIRED)")
	_ = cmd.MarkFlagRequired("set")
	cmd.Flags().StringP("note", "n", "", "Optional context note")
	return cmd
}

// NewStatusCmd returns a status command using the provided database. Intended for testing.
func NewStatusCmd(db statusDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <entity-id>",
		Short: "Change the status of an entity",
		Long: `Change the status of an entity.

Valid statuses: ok, borrowed, missing, loaned, removed

Examples:
  wherehouse status <id> --set loaned --note "loaned to Alice"
  wherehouse status <id> --set ok`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args, db)
		},
	}
	cmd.Flags().StringP("set", "s", "", "New status: ok, borrowed, missing, loaned, removed (REQUIRED)")
	_ = cmd.MarkFlagRequired("set")
	cmd.Flags().StringP("note", "n", "", "Optional context note")
	return cmd
}

type statusResult struct {
	EntityID      string  `json:"entity_id"`
	DisplayName   string  `json:"display_name"`
	Status        string  `json:"status"`
	StatusContext *string `json:"status_context,omitempty"`
}

func runStatus(cmd *cobra.Command, args []string, db statusDB) error {
	ctx := cmd.Context()
	entityID := args[0]
	setFlag, _ := cmd.Flags().GetString("set")
	noteFlag, _ := cmd.Flags().GetString("note")

	newStatus, err := database.ParseEntityStatus(setFlag)
	if err != nil {
		return err
	}

	entity, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("entity %q not found: %w", entityID, err)
	}

	var statusContext *string
	if noteFlag != "" {
		statusContext = &noteFlag
	}

	payload := map[string]any{
		"entity_id":      entityID,
		"status":         newStatus.String(),
		"status_context": statusContext,
	}

	actorUserID := cli.GetActorUserID(ctx)
	if _, err = db.AppendEvent(ctx, database.EntityStatusChangedEvent, actorUserID, payload, ""); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(statusResult{
			EntityID:      entityID,
			DisplayName:   entity.DisplayName,
			Status:        newStatus.String(),
			StatusContext: statusContext,
		})
	}

	msg := fmt.Sprintf("Status of %q set to %s", entity.DisplayName, newStatus)
	if noteFlag != "" {
		msg += fmt.Sprintf(" (%s)", noteFlag)
	}
	out.Success(msg)
	return nil
}
