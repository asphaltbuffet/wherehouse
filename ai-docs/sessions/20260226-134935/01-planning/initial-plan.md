# Architecture Plan: `wherehouse list` Command

## Overview

The `list` command is a **read-only** command that renders a tree view of locations and their
items. It follows the same structural pattern as `move` (a `cmd/list/` package with its own
`list.go`, `helpers.go`, and `output.go`), delegating shared utilities to `internal/cli/`.

No new events are emitted. No projections are modified. This is a pure read path.

---

## Command Specification

```
wherehouse list [<location>...] [--recurse|-r] [--json] [-q]
```

**Arguments:**
- Zero or more location selectors (display name, canonical name, or UUID)
- If no arguments: show all root-level locations (parent_id IS NULL), including system locations

**Flags:**
- `-r` / `--recurse` - recursively include sub-locations and their items
- `--json` - structured JSON output (global flag, via config)
- `-q` - quiet mode (global flag, via config)

**Exit codes:**
- 0 - success
- 1 - named location not found or other error

---

## Package Structure

```
cmd/list/
  list.go        - cobra.Command definition, GetListCmd()
  output.go      - tree rendering + JSON marshaling for list results
  helpers.go     - openDatabase(), resolveLocation() wrappers
  doc.go         - package godoc
```

No new `internal/` packages are needed. All DB queries use existing functions or two new
DB methods (see Database Layer section below).

---

## Data Model for Tree Rendering

The tree is a simple in-memory recursive structure:

```go
// LocationNode is one node in the rendered tree.
// Items are leaf children; sub-locations are branch children.
type LocationNode struct {
    Location  *database.Location  // the location row
    Items     []*database.Item    // items directly in this location (alphabetical)
    Children  []*LocationNode     // sub-locations (alphabetical by display name)
}
```

Building the tree has two strategies depending on the `--recurse` flag:

### Non-recursive (default)
- Fetch only the requested location(s) (or root locations if none given)
- Fetch items directly in each requested location (`GetItemsByLocation`)
- Do NOT fetch children
- Display as a flat list of locations, each with their direct items

### Recursive (`--recurse`)
- Fetch the requested location(s) (or root locations if none given)
- For each location, recursively build the full subtree:
  - `GetLocationChildren(locationID)` → children
  - `GetItemsByLocation(locationID)` → items at this level
  - Recurse into each child

The recursion is done in Go (not SQL WITH RECURSIVE) to keep it simple and leverage
existing DB functions. For typical inventories (depth <= 5, < 1000 locations), this is fast
enough. N+1 queries are acceptable given SQLite's local nature.

---

## Database Layer

### Existing functions (no changes needed)
- `db.GetLocation(ctx, locationID)` - get location by UUID
- `db.GetLocationByCanonicalName(ctx, canonicalName)` - get location by canonical name
- `db.GetLocationChildren(ctx, parentID)` - get direct children of a location
- `db.GetItemsByLocation(ctx, locationID)` - get all items in a location

### New functions needed

**`GetRootLocations(ctx context.Context) ([]*Location, error)`**

```go
// GetRootLocations retrieves all locations with no parent (top-level), ordered by display_name.
// Includes system locations (Missing, Borrowed).
func (d *Database) GetRootLocations(ctx context.Context) ([]*Location, error) {
    const query = `
        SELECT location_id, display_name, canonical_name, parent_id,
               full_path_display, full_path_canonical, depth, is_system, updated_at
        FROM locations_current
        WHERE parent_id IS NULL
        ORDER BY display_name
    `
    rows, err := d.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to query root locations: %w", err)
    }
    defer rows.Close()
    return scanLocations(rows)
}
```

This is the only new DB function required. The rest of the query work uses existing functions.

---

## Location Selector Resolution

Location arguments resolve identically to the `move` command destination:

```go
// In cmd/list/helpers.go
func resolveLocation(ctx context.Context, db *database.Database, input string) (string, error) {
    return cli.ResolveLocation(ctx, db, input)
}
```

`cli.ResolveLocation` already handles UUID and canonical name. No changes needed.

**Error behavior:**
- Named location not found → fatal error, exit 1, message to stderr
- All-or-nothing: if any named location fails to resolve, the whole command fails before
  any output is printed

---

## Human-Readable Tree Output

Tree rendering uses standard ASCII box-drawing characters (no external library needed).
The Go standard library `fmt` package is sufficient.

### Tree characters
```
├── (intermediate sibling)
└── (last sibling)
│   (continuation line prefix)
    (last-child prefix padding)
```

### Output format (non-recursive default)

When no `--recurse`:

```
Garage (3 items)
  ├── drill
  ├── hammer
  └── wrench

Office (0 items)
```

For multiple locations, each root is separated by a blank line.

### Output format (--recurse)

```
Garage (2 items, 2 sub-locations)
  ├── drill
  ├── hammer
  ├── [Shelf A] (1 item)
  │   └── sandpaper
  └── [Workbench] (2 items, 1 sub-location)
      ├── chisel
      ├── plane
      └── [Drawer 1] (1 item)
          └── 10mm socket

Missing (1 item)
  └── screwdriver
```

Sub-locations are displayed with brackets `[Name]` to visually distinguish them from items.

Items that are `in_temporary_use = true` show a marker (e.g., `*` suffix).

### Item annotation markers

```
drill           (normal item)
hammer *        (in temporary use)
```

The `*` suffix indicates the item is currently in temporary use (away from home).

### System location handling

System locations (Missing, Borrowed) appear in the listing like any other location. They
are not hidden. The `--recurse` flag applies to them (though they cannot have sub-locations
in practice since the system prevents that).

---

## JSON Output Mode

When `--json` is set, output a single JSON object. Human tree output is completely suppressed.

### JSON schema

```json
{
  "locations": [
    {
      "location_id": "uuid",
      "display_name": "Garage",
      "canonical_name": "garage",
      "full_path_display": "Garage",
      "is_system": false,
      "item_count": 2,
      "items": [
        {
          "item_id": "uuid",
          "display_name": "drill",
          "canonical_name": "drill",
          "in_temporary_use": false,
          "project_id": null
        }
      ],
      "children": [
        {
          "location_id": "uuid",
          "display_name": "Shelf A",
          "canonical_name": "shelf_a",
          "full_path_display": "Garage >> Shelf A",
          "is_system": false,
          "item_count": 1,
          "items": [...],
          "children": []
        }
      ]
    }
  ]
}
```

**Notes:**
- `children` is always present in JSON (empty array when not recursive or no children)
- `items` is always present (empty array when none)
- `project_id` is null when not set (not omitted)
- JSON output always includes full path fields for machine consumption

### Go structs for JSON

```go
// ListLocationJSON is the JSON representation of one location node.
type ListLocationJSON struct {
    LocationID      string             `json:"location_id"`
    DisplayName     string             `json:"display_name"`
    CanonicalName   string             `json:"canonical_name"`
    FullPathDisplay string             `json:"full_path_display"`
    IsSystem        bool               `json:"is_system"`
    ItemCount       int                `json:"item_count"`
    Items           []ListItemJSON     `json:"items"`
    Children        []ListLocationJSON `json:"children"`
}

// ListItemJSON is the JSON representation of one item within a location.
type ListItemJSON struct {
    ItemID         string  `json:"item_id"`
    DisplayName    string  `json:"display_name"`
    CanonicalName  string  `json:"canonical_name"`
    InTemporaryUse bool    `json:"in_temporary_use"`
    ProjectID      *string `json:"project_id"`
}

// ListOutputJSON is the root JSON output envelope.
type ListOutputJSON struct {
    Locations []ListLocationJSON `json:"locations"`
}
```

---

## Quiet Mode

`-q` suppresses the human-readable tree output. Since `list` is a read command (not a write
confirmation), quiet mode results in no output and exit 0. This is consistent with the
contract that `-q` suppresses informational output.

`-qq` same behavior.

---

## Command Implementation (`cmd/list/list.go`)

```go
func GetListCmd() *cobra.Command {
    listCmd := &cobra.Command{
        Use:   "list [<location>...]",
        Short: "List items in locations",
        Long: `List items in one or more locations.

Without arguments, shows all top-level locations and their direct items.

With location arguments, shows items in those specific locations.

Use --recurse to include sub-locations and all their contents.

Examples:
  wherehouse list
  wherehouse list Garage
  wherehouse list "Garage" "Office"
  wherehouse list --recurse
  wherehouse list -r Garage`,
        Args: cobra.ArbitraryArgs,
        RunE: runList,
    }

    listCmd.Flags().BoolP("recurse", "r", false, "recursively list sub-locations and their items")

    return listCmd
}
```

### `runList` flow

```
1. Parse flags: recurse bool
2. Open database
3. Set up OutputWriter from config (json mode, quiet mode)
4. Resolve location arguments:
   a. If len(args) == 0: call db.GetRootLocations() → root []*Location
   b. If len(args) > 0: for each arg, call resolveLocation() → []*Location
      - Collect ALL resolution errors before returning (fail-fast: fail on first error)
5. Build LocationNode tree for each resolved location:
   a. If recurse: buildLocationNodeRecursive(ctx, db, loc)
   b. If not recurse: buildLocationNodeFlat(ctx, db, loc) [items only, no children]
6. Output:
   a. If json mode: marshal to ListOutputJSON, call out.JSON()
   b. Else if not quiet: render tree to stdout via renderTree()
```

---

## Tree Rendering Implementation

Tree rendering is a recursive function in `cmd/list/output.go`:

```go
// renderTree renders a LocationNode as a tree to the writer.
// prefix is the accumulated indentation string for current level.
// isLast indicates whether this node is the last sibling at its level.
func renderNode(w io.Writer, node *LocationNode, prefix string, isLast bool)
```

Algorithm:

```
For each LocationNode:
  1. Print location header line (connector + display name + item/child count summary)
  2. Build child prefix (prefix + "│   " or "    " depending on isLast)
  3. Collect all children: items (as leaf nodes) + sub-locations (as branch nodes)
     - Items appear before sub-locations at each level (items first, then children)
  4. For each child at this level:
     a. Determine connector: "├── " for non-last, "└── " for last
     b. If item: print prefix + connector + display_name [+ marker if temp use]
     c. If sub-location: recurse renderNode with updated prefix

Root-level nodes are separated by blank lines.
```

The root-level loop (non-recursive for multiple locations) iterates over resolved locations
and calls `renderNode` for each.

---

## File Layout Detail

### `cmd/list/list.go`
- Package declaration, import list
- `GetListCmd() *cobra.Command` - cobra setup, flags
- `runList(cmd *cobra.Command, args []string) error` - main handler
- `buildLocationNodeFlat(ctx, db, loc) (*LocationNode, error)` - non-recursive tree build
- `buildLocationNodeRecursive(ctx, db, loc) (*LocationNode, error)` - recursive tree build

### `cmd/list/output.go`
- `LocationNode` struct
- `ListOutputJSON`, `ListLocationJSON`, `ListItemJSON` structs
- `renderTree(w io.Writer, nodes []*LocationNode)` - root renderer
- `renderNode(w io.Writer, node *LocationNode, prefix string, isLast bool)` - recursive node
- `toJSON(nodes []*LocationNode) ListOutputJSON` - convert tree to JSON structs

### `cmd/list/helpers.go`
- `openDatabase(ctx) (*database.Database, error)` - wraps `cli.OpenDatabase`
- `resolveLocation(ctx, db, input) (string, error)` - wraps `cli.ResolveLocation`

### `cmd/list/doc.go`
- Package godoc comment

### `internal/database/location.go` (addition)
- `GetRootLocations(ctx) ([]*Location, error)` - new function

---

## Registration

In `cmd/root.go`, add the import and register:

```go
import "github.com/asphaltbuffet/wherehouse/cmd/list"

// In GetRootCmd():
rootCmd.AddCommand(list.GetListCmd())
```

---

## Testing Strategy

### Unit tests: `cmd/list/output_test.go`
- `renderTree` with empty node set → empty output
- Single location, no items, no children
- Single location with items (tree connector characters)
- Multiple root locations with blank line separation
- Recursive tree with nested children
- JSON output marshaling (struct → JSON roundtrip)
- Item with `in_temporary_use = true` shows `*` marker

### Unit tests: `cmd/list/list_test.go`
- `buildLocationNodeFlat` - verifies items populated, children empty
- `buildLocationNodeRecursive` - verifies full subtree built

### Integration tests
- `runList` with real in-memory DB (pattern from `cmd/move/item_test.go`)
- `wherehouse list` (no args) → root locations output
- `wherehouse list Garage` → specific location
- `wherehouse list --recurse` → full tree
- `wherehouse list --json` → valid JSON schema
- `wherehouse list UnknownLoc` → exit 1, error to stderr
- `wherehouse list -q` → no output, exit 0

### New DB function test
- `GetRootLocations` in `internal/database/location_test.go`

---

## Alternatives Considered

### Alternative 1: SQL WITH RECURSIVE for subtree queries
- Pro: Single query for full subtree, avoids N+1
- Con: SQLite WITH RECURSIVE is supported but adds complexity; overkill for typical
  inventory sizes; breaks existing `scanLocations` helper reuse
- Decision: Go-level recursion using existing DB functions is simpler and maintainable

### Alternative 2: External tree rendering library (e.g., github.com/xlab/treeprint)
- Pro: Less code for rendering
- Con: Additional dependency for a straightforward ASCII rendering task; the project
  already uses lipgloss for styling; ASCII box-drawing is ~50 lines of Go
- Decision: Implement directly using `fmt.Fprintf`, consistent with project philosophy
  of minimal dependencies

### Alternative 3: Single flat list with indentation (no connectors)
- Pro: Simpler rendering
- Con: Harder to read at a glance; user request explicitly says "tree-style output"
- Decision: Use proper tree connectors

### Alternative 4: Put all logic in `internal/cli/list.go`
- Pro: Shares code across potential TUI usage
- Con: Other commands (`move`, `add`) keep their logic in `cmd/<name>/`; premature
  abstraction before TUI requirements are known
- Decision: Follow existing pattern, keep in `cmd/list/`

---

## Summary of Changes

| File | Change |
|------|--------|
| `cmd/list/list.go` | NEW - cobra command, runList, node builders |
| `cmd/list/output.go` | NEW - tree rendering, JSON structs |
| `cmd/list/helpers.go` | NEW - thin wrappers around cli.* |
| `cmd/list/doc.go` | NEW - package godoc |
| `cmd/list/list_test.go` | NEW - unit + integration tests |
| `cmd/list/output_test.go` | NEW - rendering tests |
| `cmd/root.go` | MODIFY - add import + AddCommand |
| `internal/database/location.go` | MODIFY - add GetRootLocations |
| `internal/database/location_test.go` | MODIFY - add GetRootLocations test |

---

## Design Decisions

1. **Go-level recursion over SQL WITH RECURSIVE**: Simpler, reuses existing `GetLocationChildren`
   and `GetItemsByLocation`, acceptable performance for local SQLite inventories.

2. **Items before sub-locations within a node**: At each tree level, items are listed first,
   then sub-locations. This matches mental model (contents before containers).

3. **Brackets around sub-location names** `[Shelf A]`: Visual distinction from items in the
   tree without requiring color (works in non-color terminals).

4. **`*` marker for temporary use**: Minimal annotation that does not break the tree layout.
   Could be expanded to `[temp]` suffix or color styling via lipgloss in a future pass.

5. **Fail-all-or-nothing for selector resolution**: If any named location argument fails to
   resolve, the command exits with error before printing any output. Consistent with `move`.

6. **Single new DB function**: Only `GetRootLocations` is needed. All other DB access uses
   existing functions. This minimizes the DB layer change surface.
