package tag

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultTagCmd wires the tag command with a production App opened from config.
func NewDefaultTagCmd() *cobra.Command {
	cmd := buildTagCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runTag(cmd, args, a)
	}
	return cmd
}

// NewTagCmd returns the tag command injected with the given App.
func NewTagCmd(a *app.App) *cobra.Command {
	cmd := buildTagCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runTag(cmd, args, a) }
	return cmd
}

func buildTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <path>",
		Short: "Add, remove, or list tags on an entity",
		Long: `Add, remove, or list classification tags on an entity.

Tags are short labels that group entities across the hierarchy regardless of location.
Tags are normalized: "Torx" and "torx" are the same tag.

Examples:
  wherehouse tag "Garage:Toolbox:T0 screwdriver"
  wherehouse tag "Garage:Toolbox:T0 screwdriver" --add tool --add torx
  wherehouse tag "Garage:Toolbox:T0 screwdriver" --remove torx
  wherehouse tag "Garage:Toolbox:T0 screwdriver" --add hand_tool --remove tool`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringArrayP("add", "a", nil, "Tag to add (repeatable)")
	cmd.Flags().StringArrayP("remove", "r", nil, "Tag to remove (repeatable)")
	return cmd
}

func runTag(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	path := args[0]

	addFlags, _ := cmd.Flags().GetStringArray("add")
	removeFlags, _ := cmd.Flags().GetStringArray("remove")

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.IsJSON(), cfg.IsQuiet())

	entity, err := a.LookupEntityByPath(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to find %q: %w", path, err)
	}

	isMutation := len(addFlags) > 0 || len(removeFlags) > 0

	if isMutation {
		err = a.TagEntity(ctx, app.TagEntityRequest{
			EntityID: entity.EntityID,
			ActorID:  cli.GetActorUserID(ctx),
			Add:      addFlags,
			Remove:   removeFlags,
		})
		if err != nil {
			return fmt.Errorf("tag %q: %w", path, err)
		}
	}

	tags, err := a.ListTags(ctx, app.ListTagsRequest{EntityID: entity.EntityID})
	if err != nil {
		return fmt.Errorf("list tags for %q: %w", path, err)
	}

	if out.IsJSON() {
		return out.JSON(app.ToTagOutput(path, tags))
	}

	if len(tags) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(tags, "\n"))
	}
	return nil
}
