package list

import (
	"context"
	"errors"
	"fmt"

	"github.com/goccy/go-json"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

const listLongDescription = `List items in one or more locations.

Without arguments, shows all top-level locations and their direct items.
Direct child locations are shown as hints with item and location counts.

With location arguments, shows items in those specific locations.
If a location argument cannot be resolved, it is shown inline as
"[arg] [not found]" and does not cause a non-zero exit.

Use --recurse (-r) to include sub-locations and all their contents.

Examples:
  wherehouse list                      # Show all top-level locations
  wherehouse list Garage               # Show items in Garage
  wherehouse list "Garage" "Office"    # Show items in multiple locations
  wherehouse list --recurse            # Show all locations recursively
  wherehouse list -r Garage            # Recurse into Garage`

// NewListCmd returns a list command that opens the database from context
// configuration at runtime.
func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [<location>...]",
		Short: "List items in locations",
		Long:  listLongDescription,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runListCore(cmd, args, db)
		},
	}

	cmd.Flags().BoolP("recurse", "r", false, "recursively list sub-locations and their items")

	return cmd
}

// ensure *database.Database satisfies listDB at compile time.
var _ listDB = (*database.Database)(nil)

// runListCore is the main entry point for the list command.
func runListCore(cmd *cobra.Command, args []string, db listDB) error {
	ctx := cmd.Context()

	recurse, _ := cmd.Flags().GetBool("recurse")

	cfg := cli.MustGetConfig(ctx)

	if cfg == nil {
		return errors.New("config not found in context")
	}

	nodes, err := buildNodes(ctx, db, args, recurse)
	if err != nil {
		return err
	}

	if cfg.IsJSON() {
		output := toJSON(nodes)
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(output); encErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", encErr)
		}
		return nil
	}

	renderTree(cmd.OutOrStdout(), nodes)
	return nil
}

// buildNodes constructs the LocationNode slice for the given arguments.
// If args is empty, root locations are used. Unresolvable args become
// NotFound nodes (no error returned).
func buildNodes(ctx context.Context, db listDB, args []string, recurse bool) ([]*LocationNode, error) {
	if len(args) == 0 {
		return buildRootNodes(ctx, db, recurse)
	}

	nodes := make([]*LocationNode, 0, len(args))
	for _, arg := range args {
		locationID, resolveErr := resolveLocation(ctx, db, arg)
		if resolveErr != nil {
			// Render inline as not-found; do not propagate error.
			nodes = append(nodes, &LocationNode{NotFound: true, InputArg: arg})
			continue
		}

		loc, locErr := db.GetLocation(ctx, locationID)
		if locErr != nil {
			// Should be rare (ID resolved but not fetchable); treat as not-found.
			nodes = append(nodes, &LocationNode{NotFound: true, InputArg: arg})
			continue
		}

		node, buildErr := buildNode(ctx, db, loc, recurse)
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build node for %q: %w", arg, buildErr)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// buildRootNodes fetches all root locations and builds a node for each.
func buildRootNodes(ctx context.Context, db listDB, recurse bool) ([]*LocationNode, error) {
	roots, err := db.GetRootLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get root locations: %w", err)
	}

	nodes := make([]*LocationNode, 0, len(roots))
	for _, loc := range roots {
		node, buildErr := buildNode(ctx, db, loc, recurse)
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build node for %q: %w", loc.DisplayName, buildErr)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// buildNode dispatches to the flat or recursive builder based on the recurse flag.
func buildNode(
	ctx context.Context,
	db listDB,
	loc *database.Location,
	recurse bool,
) (*LocationNode, error) {
	if recurse {
		return buildLocationNodeRecursive(ctx, db, loc)
	}
	return buildLocationNodeFlat(ctx, db, loc)
}

// buildLocationNodeFlat builds a LocationNode for non-recursive display.
// Items are populated; children are hint-only nodes (Items/Children are nil,
// ChildItemCount and ChildLocationCount are set from lightweight DB queries).
func buildLocationNodeFlat(ctx context.Context, db listDB, loc *database.Location) (*LocationNode, error) {
	node := &LocationNode{Location: loc}

	// Fetch direct items.
	items, err := db.GetItemsByLocation(ctx, loc.LocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items for %q: %w", loc.DisplayName, err)
	}
	node.Items = items

	// Fetch direct children and build hint nodes.
	children, err := db.GetLocationChildren(ctx, loc.LocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get children for %q: %w", loc.DisplayName, err)
	}

	hints := make([]*LocationNode, 0, len(children))
	for _, child := range children {
		childItems, childItemsErr := db.GetItemsByLocation(ctx, child.LocationID)
		if childItemsErr != nil {
			return nil, fmt.Errorf("failed to get items for child %q: %w", child.DisplayName, childItemsErr)
		}

		grandchildren, grandchildErr := db.GetLocationChildren(ctx, child.LocationID)
		if grandchildErr != nil {
			return nil, fmt.Errorf("failed to get children of child %q: %w", child.DisplayName, grandchildErr)
		}

		hints = append(hints, &LocationNode{
			Location:           child,
			ChildItemCount:     len(childItems),
			ChildLocationCount: len(grandchildren),
		})
	}
	node.Children = hints

	return node, nil
}

// buildLocationNodeRecursive builds a fully-populated LocationNode tree.
// Both Items and Children are populated at every level; ChildItemCount and
// ChildLocationCount are unused in recursive mode.
func buildLocationNodeRecursive(
	ctx context.Context,
	db listDB,
	loc *database.Location,
) (*LocationNode, error) {
	node := &LocationNode{Location: loc}

	// Fetch direct items.
	items, err := db.GetItemsByLocation(ctx, loc.LocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items for %q: %w", loc.DisplayName, err)
	}
	node.Items = items

	// Fetch and recurse into children.
	children, err := db.GetLocationChildren(ctx, loc.LocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get children for %q: %w", loc.DisplayName, err)
	}

	childNodes := make([]*LocationNode, 0, len(children))
	for _, child := range children {
		childNode, childErr := buildLocationNodeRecursive(ctx, db, child)
		if childErr != nil {
			return nil, childErr
		}
		childNodes = append(childNodes, childNode)
	}
	node.Children = childNodes

	return node, nil
}

// resolveLocation resolves a location name or ID to the location ID string.
func resolveLocation(ctx context.Context, db listDB, input string) (string, error) {
	return cli.ResolveLocation(ctx, db, input)
}
