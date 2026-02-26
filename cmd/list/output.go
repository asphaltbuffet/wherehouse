package list

import (
	"fmt"
	"io"

	"github.com/dustin/go-humanize/english"
	"github.com/xlab/treeprint"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// LocationNode is one node in the rendered tree.
//
// In non-recursive mode, Children are populated with hint-only nodes
// (Items and Children are nil; ChildItemCount and ChildLocationCount are set).
// In recursive mode, Items and Children are fully populated;
// ChildItemCount and ChildLocationCount are unused (derive from len).
type LocationNode struct {
	Location           *database.Location
	Items              []*database.Item // direct items (alphabetical)
	Children           []*LocationNode  // sub-locations (alphabetical by display_name)
	ChildItemCount     int              // hint nodes only: item count for this location
	ChildLocationCount int              // hint nodes only: direct child location count
	NotFound           bool             // true if this node represents an unresolved arg
	InputArg           string           // original input argument, used when NotFound=true
}

// ItemJSON is the JSON representation of a single item.
type ItemJSON struct {
	ItemID         string  `json:"item_id"`
	DisplayName    string  `json:"display_name"`
	CanonicalName  string  `json:"canonical_name"`
	InTemporaryUse bool    `json:"in_temporary_use"`
	ProjectID      *string `json:"project_id"`
}

// LocationJSON is the JSON representation of a location with its contents.
type LocationJSON struct {
	LocationID      string         `json:"location_id"`
	DisplayName     string         `json:"display_name"`
	CanonicalName   string         `json:"canonical_name"`
	FullPathDisplay string         `json:"full_path_display"`
	IsSystem        bool           `json:"is_system"`
	ItemCount       int            `json:"item_count"`
	LocationCount   int            `json:"location_count"`
	Items           []ItemJSON     `json:"items"`
	Children        []LocationJSON `json:"children"`
	NotFound        bool           `json:"not_found,omitempty"`
}

// OutputJSON is the top-level JSON output structure.
type OutputJSON struct {
	Locations []LocationJSON `json:"locations"`
}

// locationHeader returns the formatted display string for a location node header.
// e.g. "Garage (3 items, 2 locations)" or "Office (0 items, 0 locations)".
func locationHeader(name string, itemCount, locationCount int) string {
	return fmt.Sprintf("%s (%d %s, %d %s)",
		name,
		itemCount, english.PluralWord(itemCount, "item", ""),
		locationCount, english.PluralWord(locationCount, "location", ""),
	)
}

// populateTree adds the items and child sub-locations from node into branch.
// Items appear before sub-locations; both groups are already in alphabetical
// order from the database queries.
func populateTree(branch treeprint.Tree, node *LocationNode) {
	// Items first (already alphabetical from DB)
	for _, item := range node.Items {
		label := item.DisplayName
		if item.InTemporaryUse {
			label += " *"
		}
		branch.AddNode(label)
	}

	// Sub-locations
	for _, child := range node.Children {
		if child.NotFound {
			// Guard: not-found nodes should not appear as children, but handle defensively.
			branch.AddNode(child.InputArg + " [not found]")
			continue
		}

		// Determine counts for this child.
		var childItems, childLocs int
		// Flat hint nodes have both Items and Children as nil (only ChildItemCount/ChildLocationCount set).
		// Recursive nodes always have non-nil Children (make(...) in buildLocationNodeRecursive ensures this
		// even for empty slices), so this nil check reliably distinguishes the two modes.
		if child.Items != nil || child.Children != nil {
			// Recursive mode: derive from populated slices.
			childItems = len(child.Items)
			childLocs = len(child.Children)
		} else {
			// Flat mode: use pre-fetched counts.
			childItems = child.ChildItemCount
			childLocs = child.ChildLocationCount
		}

		header := "[" + child.Location.DisplayName + "] " +
			fmt.Sprintf("(%d %s, %d %s)",
				childItems, english.PluralWord(childItems, "item", ""),
				childLocs, english.PluralWord(childLocs, "location", ""),
			)
		childBranch := branch.AddBranch(header)

		// Only recurse if fully built (recursive mode).
		if child.Items != nil || child.Children != nil {
			populateTree(childBranch, child)
		}
	}
}

// renderTree renders a slice of root LocationNodes to w, one tree per root.
// Roots are separated by blank lines.
func renderTree(w io.Writer, nodes []*LocationNode) {
	for i, node := range nodes {
		if i > 0 {
			fmt.Fprintln(w)
		}

		if node.NotFound {
			fmt.Fprintln(w, node.InputArg+" [not found]")
			continue
		}

		// Determine item and location counts for the root node header.
		itemCount := len(node.Items)
		locCount := len(node.Children)

		// In flat mode, Items/Children may be nil but counts are in the hint fields.
		if node.Items == nil && node.Children == nil {
			itemCount = node.ChildItemCount
			locCount = node.ChildLocationCount
		}

		root := treeprint.New()
		root.SetValue(locationHeader(node.Location.DisplayName, itemCount, locCount))
		populateTree(root, node)
		fmt.Fprint(w, root.String())
	}
}

// toJSON converts a slice of LocationNodes to the OutputJSON structure.
func toJSON(nodes []*LocationNode) OutputJSON {
	locs := make([]LocationJSON, 0, len(nodes))
	for _, node := range nodes {
		locs = append(locs, nodeToJSON(node))
	}
	return OutputJSON{Locations: locs}
}

// nodeToJSON converts a single LocationNode to its JSON representation.
func nodeToJSON(node *LocationNode) LocationJSON {
	if node.NotFound {
		return LocationJSON{
			DisplayName: node.InputArg,
			NotFound:    true,
		}
	}

	itemCount := len(node.Items)
	locCount := len(node.Children)
	if node.Items == nil && node.Children == nil {
		itemCount = node.ChildItemCount
		locCount = node.ChildLocationCount
	}

	items := make([]ItemJSON, 0, len(node.Items))
	for _, item := range node.Items {
		j := ItemJSON{
			ItemID:         item.ItemID,
			DisplayName:    item.DisplayName,
			CanonicalName:  item.CanonicalName,
			InTemporaryUse: item.InTemporaryUse,
			ProjectID:      item.ProjectID,
		}
		items = append(items, j)
	}

	children := make([]LocationJSON, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, nodeToJSON(child))
	}

	return LocationJSON{
		LocationID:      node.Location.LocationID,
		DisplayName:     node.Location.DisplayName,
		CanonicalName:   node.Location.CanonicalName,
		FullPathDisplay: node.Location.FullPathDisplay,
		IsSystem:        node.Location.IsSystem,
		ItemCount:       itemCount,
		LocationCount:   locCount,
		Items:           items,
		Children:        children,
	}
}
