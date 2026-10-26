package attr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultAttrCmd returns an attr command that opens the database from context configuration at runtime.
func NewDefaultAttrCmd() *cobra.Command {
	cmd := buildAttrCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runAttr(cmd, args, a)
	}
	return cmd
}

// NewAttrCmd returns an attr command backed by the provided App (for tests).
func NewAttrCmd(a *app.App) *cobra.Command {
	cmd := buildAttrCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runAttr(cmd, args, a) }
	return cmd
}

func buildAttrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attr <path>...",
		Short: "Show or change the locked/discrete attributes of entities",
		Long: `Show or change the locked and discrete attributes of one or more entities.

With no attribute flag, prints each entity's attributes. With --locked and/or
--discrete, sets them; use the = form to clear (--locked=false). Setting an
attribute to the value it already has is an error.

  locked    can only be set on ok or borrowed entities
  discrete  can only be set on entities with no children

All entities are updated atomically — if any path fails, none are changed.

Examples:
  wherehouse attr "Garage:Shelf"
  wherehouse attr "Garage:Shelf" "Garage:Workbench" --locked
  wherehouse attr "Garage:Shelf" --locked=false
  wherehouse attr "Garage:Box of Nails" --discrete --locked=false --note "single unit"`,
		Args: cobra.MinimumNArgs(1),
	}
	cmd.Flags().Bool("locked", false, "Set the locked attribute (--locked=false to clear)")
	cmd.Flags().Bool("discrete", false, "Set the discrete attribute (--discrete=false to clear)")
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runAttr(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	locked := optionalBool(cmd, "locked")
	discrete := optionalBool(cmd, "discrete")
	noteFlag, _ := cmd.Flags().GetString("note")

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	entities := make([]app.EntityResult, len(args))
	for i, path := range args {
		entity, err := a.LookupEntityByPath(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to find %q: %w", path, err)
		}
		entities[i] = entity
	}

	if locked == nil && discrete == nil {
		if noteFlag != "" {
			return errors.New("--note requires --locked or --discrete")
		}
		return out.Render(app.ToAttrOutputs(entities), func() string { return formatAttrs(entities) })
	}

	reqs := make([]app.SetAttributesRequest, len(entities))
	for i, e := range entities {
		reqs[i] = app.SetAttributesRequest{
			EntityID: e.EntityID,
			Locked:   locked,
			Discrete: discrete,
			Note:     noteFlag,
			ActorID:  cli.GetActorUserID(ctx),
		}
	}

	results, err := a.SetAttributes(ctx, reqs)
	if err != nil {
		return fmt.Errorf("failed to set attributes: %w", err)
	}

	if out.IsJSON() {
		return out.JSON(app.ToAttrOutputs(results))
	}

	change := describeChange(locked, discrete)
	for _, result := range results {
		out.Success(fmt.Sprintf("%q %s", result.FullPathDisplay, change))
	}
	return nil
}

// optionalBool returns nil when the flag was not given, so "not given" and
// "given as false" stay distinct.
func optionalBool(cmd *cobra.Command, name string) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetBool(name)
	return &v
}

// describeChange names what a write did, discrete phrase first.
func describeChange(locked, discrete *bool) string {
	var parts []string
	if discrete != nil {
		if *discrete {
			parts = append(parts, "marked discrete")
		} else {
			parts = append(parts, "cleared discrete")
		}
	}
	if locked != nil {
		if *locked {
			parts = append(parts, "locked")
		} else {
			parts = append(parts, "unlocked")
		}
	}
	return strings.Join(parts, ", ")
}

func formatAttrs(entities []app.EntityResult) string {
	width := 0
	for _, e := range entities {
		width = max(width, len(e.FullPathDisplay))
	}
	var b strings.Builder
	for _, e := range entities {
		fmt.Fprintf(&b, "%-*s  locked=%t  discrete=%t\n", width, e.FullPathDisplay, e.Locked, e.Discrete)
	}
	return b.String()
}
