# Final Implementation Plan: `wherehouse list` Command

**Session**: 20260226-134935
**Status**: Ready for implementation
**Incorporates clarifications**: 2026-02-26 (initial), user feedback 2026-02-26

---

## Summary of All Changes Applied

### Initial Clarifications
1. **Non-recursive mode shows direct child location names** as hints — calls `GetLocationChildren` but does NOT recurse items.
2. **Item count annotation on every location header**: `Garage (3 items)` appears in both recursive and non-recursive output.
3. **No depth limit on `--recurse`**: Full tree always, confirmed.
4. **Sort alphabetical by display_name throughout**: Already satisfied by `ORDER BY display_name` in DB queries.

### User Feedback (6 items)
1. **Remove `--quiet` flag entirely**: The list command is for display; quiet mode is nonsensical.
2. **Child location hints show BOTH item count AND location count**: `[Shelf A] (1 item, 3 locations)` — both counts, no contents fetched.
3. **Use `github.com/dustin/go-humanize` `english.PluralWord` for pluralization**: Already in `go.mod`. No `go get` needed.
4. **Use `github.com/xlab/treeprint` for tree rendering**: NOT in `go.mod`. `go get github.com/xlab/treeprint` required. See selection rationale below.
5. **No `//nolint` directive on `cobra.ArbitraryArgs`**: `ArbitraryArgs` does not trigger `mnd`; no nolint comment needed.
6. **Not-found locations render inline as `[location-arg] [not found]`**: No error returned; rendering continues for other valid locations.

---

## Tree Rendering Package Selection

**Chosen**: `github.com/xlab/treeprint`

**Rationale**:
- `xlab/treeprint` builds a tree data structure in memory then renders it, rather than requiring manual prefix tracking. This eliminates the custom `renderNode`/`renderTree` recursion that was in the prior plan.
- Its `AddBranch(value)` and `AddNode(value)` methods accept any string as the display value, so pre-formatted strings like `"Garage (3 items, 2 locations)"` and `"drill *"` can be passed directly.
- `VisitAll` / `String()` produce correct `├──`, `└──`, `│   ` connectors without any manual prefix arithmetic.
- Actively maintained, used in production CLIs (e.g., common-fate/cli), predictable platform-independent output.

**Alternatives considered**:
- `github.com/disiqueira/gotree`: Simpler API, but last commit 2018, less flexible for mixed item/location nodes.
- `github.com/a8m/tree`: Mirrors the `tree` Unix command, hardcoded for filesystem entries — does not suit our domain model.
- Custom implementation (prior plan): Eliminated per user feedback; external package preferred.

**`go get` required**: `go get github.com/xlab/treeprint`

---

## Command Specification (Final)

```
wherehouse list [<location>...] [--recurse|-r] [--json]
```

**Arguments:**
- Zero or more location selectors (display name, canonical name, or UUID).
- If no arguments: show all root-level locations (`parent_id IS NULL`), including system locations.

**Flags:**
- `-r` / `--recurse` — recursively include sub-locations and all their items.
- `--json` — structured JSON output (global flag, via config).

**Note**: No `--quiet` / `-q` flag. The list command always produces output.

**Exit codes:**
- 0 — success (including when some location arguments are not found; those render inline).
- 1 — database error only.

---

## Output Formats

### Non-Recursive Output Format

Non-recursive mode shows direct items first, then direct child location names as hints (no items fetched for children). Child hints show both item count AND direct child-location count.

```
Garage (3 items, 2 locations)
  ├── drill
  ├── hammer
  ├── wrench
  ├── [Shelf A] (1 item, 0 locations)
  └── [Workbench] (2 items, 1 location)

Office (0 items, 0 locations)

Missing (1 item, 0 locations)
  └── screwdriver

bad-shelf [not found]
```

Key behaviors:
- Location header shows `(N items, M locations)` always — counts of direct items and direct child locations.
- Items listed first (alphabetical), then sub-location hints (alphabetical, in brackets).
- Sub-location hints show `(N items, M locations)` — fetched during flat build, no contents shown.
- Multiple roots (or multiple resolved locations) separated by blank lines.
- Not-found arguments rendered as `arg-value [not found]`, no error returned, exit 0.

### Recursive Output Format

```
Garage (3 items, 2 locations)
  ├── drill
  ├── hammer
  ├── wrench
  ├── [Shelf A] (1 item, 0 locations)
  │   └── sandpaper
  └── [Workbench] (2 items, 1 location)
      ├── chisel
      ├── plane
      └── [Drawer 1] (1 item, 0 locations)
          └── 10mm socket

Missing (1 item, 0 locations)
  └── screwdriver
```

Key behaviors:
- Header shows `(N items, M locations)` at every level — always both counts.
- Items first, then sub-locations (alphabetical within each group at each level).
- Full tree, no depth limit.
- `*` suffix on items where `in_temporary_use = true`.

---

## Pluralization

Use `github.com/dustin/go-humanize/english` throughout. Already in `go.mod`.

```go
import "github.com/dustin/go-humanize/english"

// "1 item" or "3 items"
english.PluralWord(itemCount, "item", "")
// "1 location" or "3 locations"
english.PluralWord(childCount, "location", "")
// "0 items" etc.
fmt.Sprintf("%d %s", n, english.PluralWord(n, "item", ""))
```

`english.PluralWord(n, singular, plural)` — pass `""` as plural to use automatic `s`-suffixing. Remove the custom `pluralize` helper from the prior plan entirely.

---

## LocationNode Data Model

```go
// LocationNode is one node in the rendered tree.
// In non-recursive mode, Children are populated with hint-only nodes
// (Items and Children are nil; ChildItemCount and ChildLocationCount are set).
// In recursive mode, Items and Children are fully populated;
// ChildItemCount and ChildLocationCount are unused (derive from len).
type LocationNode struct {
    Location           *database.Location
    Items              []*database.Item   // direct items (alphabetical)
    Children           []*LocationNode    // sub-locations (alphabetical by display_name)
    ChildItemCount     int                // hint nodes only: item count for this location
    ChildLocationCount int                // hint nodes only: direct child location count
    NotFound           bool               // true if this node represents an unresolved arg
    InputArg           string             // original input argument, used when NotFound=true
}
```

### `buildLocationNodeFlat` (non-recursive)

```
1. Fetch items for location (GetItemsByLocation) → node.Items
2. Fetch direct children (GetLocationChildren) → for each child:
   a. Fetch child's direct items (GetItemsByLocation) → childItemCount = len(...)
   b. Fetch child's direct children (GetLocationChildren) → childLocationCount = len(...)
   c. Create LocationNode{
          Location:           child,
          ChildItemCount:     childItemCount,
          ChildLocationCount: childLocationCount,
      }
3. node.Children = hint nodes from step 2
4. Return node
```

Note: N+1 queries are acceptable for local SQLite inventories of this scale.

### `buildLocationNodeRecursive`

```
1. Fetch items for location (GetItemsByLocation) → node.Items
2. Fetch direct children (GetLocationChildren) → for each child:
   a. Recurse buildLocationNodeRecursive(ctx, db, child) → child node
3. node.Children = child nodes from step 2
4. Return node
```

---

## Tree Rendering with `xlab/treeprint`

The `treeprint` package builds a tree in-memory then renders to string. We populate it from our `LocationNode` tree and call `String()`.

### Header string construction

```go
// locationHeader returns the formatted display string for a location node header.
// e.g. "Garage (3 items, 2 locations)" or "Office (0 items, 0 locations)"
func locationHeader(name string, itemCount, locationCount int) string {
    return fmt.Sprintf("%s (%d %s, %d %s)",
        name,
        itemCount,   english.PluralWord(itemCount,   "item",     ""),
        locationCount, english.PluralWord(locationCount, "location", ""),
    )
}
```

### populateTree — add one LocationNode into a treeprint.Tree branch

```go
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
            // Should not occur in child hints; guard only
            branch.AddNode(child.InputArg + " [not found]")
            continue
        }
        // Determine counts for this child
        var childItems, childLocs int
        if child.Items != nil {
            // recursive mode: derive from populated slices
            childItems = len(child.Items)
            childLocs = len(child.Children)
        } else {
            // flat mode: use pre-fetched counts
            childItems = child.ChildItemCount
            childLocs = child.ChildLocationCount
        }
        header := "[" + child.Location.DisplayName + "] " +
            fmt.Sprintf("(%d %s, %d %s)",
                childItems, english.PluralWord(childItems, "item", ""),
                childLocs,  english.PluralWord(childLocs,  "location", ""),
            )
        childBranch := branch.AddBranch(header)
        // Only recurse if fully built (recursive mode)
        if child.Items != nil || child.Children != nil {
            populateTree(childBranch, child)
        }
    }
}
```

### renderTree

```go
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
        itemCount := len(node.Items)
        if node.Items == nil {
            itemCount = node.ChildItemCount
        }
        locCount := len(node.Children)
        if node.Children == nil {
            locCount = node.ChildLocationCount
        }
        root := treeprint.New()
        root.SetValue(locationHeader(node.Location.DisplayName, itemCount, locCount))
        populateTree(root, node)
        fmt.Fprint(w, root.String())
    }
}
```

Note: `treeprint.New()` creates an unnamed root; `SetValue` sets the root display string. The `String()` method returns the full rendered tree including the root line.

---

## Package Structure

```
cmd/list/
  doc.go       - package godoc
  list.go      - cobra.Command definition, GetListCmd(), runList, node builders
  output.go    - locationHeader, populateTree, renderTree, JSON structs/marshaling
  helpers.go   - openDatabase(), resolveLocation() wrappers
```

---

## Subtask Breakdown with Agent Routing

### Subtask 1: Database — `GetRootLocations` function
**Agent**: db-developer
**Files**: `internal/database/location.go`, `internal/database/location_test.go`

Add one new exported function:

```go
// GetRootLocations retrieves all locations with no parent (top-level),
// ordered by display_name. Includes system locations (Missing, Borrowed).
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

**Tests** (add to `internal/database/location_test.go`):
- Empty database → empty slice, no error
- Single root location → returned
- Multiple root locations → returned alphabetically by display_name
- Root location with children → only root returned (children excluded)
- System locations (Missing, Borrowed) → included in results

**Acceptance**: `mise run test` passes, `mise run lint` clean.

---

### Subtask 2: CLI — `list` command implementation
**Agent**: golang-ui-developer
**Files**: `cmd/list/` (all new), `cmd/root.go` (AddCommand)
**Depends on**: Subtask 1 merged (or stub `GetRootLocations` locally for parallel work)
**Prerequisite**: `go get github.com/xlab/treeprint` must be run before compilation.

#### `cmd/list/doc.go`

```go
// Package list implements the wherehouse list command for displaying
// locations and their items in a tree view.
package list
```

#### `cmd/list/helpers.go`

```go
package list

import (
    "context"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
    "github.com/asphaltbuffet/wherehouse/internal/database"
)

func openDatabase(ctx context.Context) (*database.Database, error) {
    return cli.OpenDatabase(ctx)
}

func resolveLocation(ctx context.Context, db *database.Database, input string) (*database.Location, error) {
    return cli.ResolveLocation(ctx, db, input)
}
```

Note: verify `cli.ResolveLocation` signature — it may return a UUID string rather than `*database.Location`. If so, follow with `db.GetLocation(ctx, id)`.

#### `cmd/list/output.go`

Data structures and rendering (see "Tree Rendering with xlab/treeprint" section above for full function bodies).

JSON output structs:

```go
type ListItemJSON struct {
    ItemID         string  `json:"item_id"`
    DisplayName    string  `json:"display_name"`
    CanonicalName  string  `json:"canonical_name"`
    InTemporaryUse bool    `json:"in_temporary_use"`
    ProjectID      *string `json:"project_id"`
}

type ListLocationJSON struct {
    LocationID      string             `json:"location_id"`
    DisplayName     string             `json:"display_name"`
    CanonicalName   string             `json:"canonical_name"`
    FullPathDisplay string             `json:"full_path_display"`
    IsSystem        bool               `json:"is_system"`
    ItemCount       int                `json:"item_count"`
    LocationCount   int                `json:"location_count"`
    Items           []ListItemJSON     `json:"items"`
    Children        []ListLocationJSON `json:"children"`
    NotFound        bool               `json:"not_found,omitempty"`
}

type ListOutputJSON struct {
    Locations []ListLocationJSON `json:"locations"`
}
```

#### `cmd/list/list.go`

```go
func GetListCmd() *cobra.Command {
    listCmd := &cobra.Command{
        Use:   "list [<location>...]",
        Short: "List items in locations",
        Long: `List items in one or more locations.

Without arguments, shows all top-level locations and their direct items.
Direct child locations are shown as hints with item and location counts.

With location arguments, shows items in those specific locations.
If a location argument cannot be resolved, it is shown inline as "[arg] [not found]".

Use --recurse (-r) to include sub-locations and all their contents.

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

`runList` flow:

```
1. Parse --recurse flag
2. openDatabase(ctx)
3. Load config (json mode) via config from context — no quiet mode
4. Resolve location arguments:
   a. If len(args) == 0: db.GetRootLocations(ctx) → []*Location, build nodes normally
   b. If len(args) > 0: for each arg:
      - Call resolveLocation(ctx, db, arg)
      - If error (not found): create LocationNode{NotFound: true, InputArg: arg}
      - If ok: build LocationNode normally (flat or recursive)
5. Output:
   a. json mode: toJSON(nodes) → marshal → os.Stdout
   b. default: renderTree(os.Stdout, nodes)
6. Return nil (not-found args do NOT cause non-zero exit)
```

#### `cmd/root.go` change

```go
import listcmd "github.com/asphaltbuffet/wherehouse/cmd/list"

// In GetRootCmd():
rootCmd.AddCommand(listcmd.GetListCmd())
```

---

## Testing Strategy

### `cmd/list/output_test.go`

- `locationHeader` — 0/0, N/0, 0/M, N/M combinations; correct pluralization via go-humanize
- `renderTree` — empty node slice → empty output
- `renderTree` — single not-found node → `"arg [not found]\n"` output
- `renderTree` — single node, no items, no children → header only with counts
- `renderTree` — single node with items: tree connectors correct (`├──`, `└──`)
- `renderTree` — node with items and child hints (flat style): hints show both item and location counts
- `renderTree` — last-item connector (`└──`) vs intermediate (`├──`)
- `renderTree` — recursive nested tree: prefix indentation correct (treeprint handles this)
- `renderTree` — item with `in_temporary_use=true` shows `*` suffix
- `renderTree` — multiple roots separated by blank lines
- `toJSON` — produces correct JSON struct from LocationNode tree, not-found node has `not_found: true`
- JSON roundtrip: marshal + unmarshal produces identical struct

### `cmd/list/list_test.go`

Using in-memory SQLite test DB (pattern from `cmd/move/item_test.go`):

- `buildLocationNodeFlat` — items populated; children are hint-only (Items==nil); ChildItemCount and ChildLocationCount correct
- `buildLocationNodeRecursive` — full subtree built, all items populated
- `runList` with no args → root locations rendered (integration)
- `runList Garage` → Garage only, with direct children as hints showing both counts
- `runList --recurse` → full tree
- `runList --json` → valid JSON matching schema
- `runList UnknownLoc` → exit 0, "[UnknownLoc] [not found]" in output, no error returned
- `runList Garage UnknownLoc` → Garage rendered normally, "[UnknownLoc] [not found]" appended, exit 0
- No `--quiet` / `-q` test (flag does not exist)

### `internal/database/location_test.go` additions

- `GetRootLocations` empty DB → empty slice
- `GetRootLocations` with root locations → returned alphabetically
- `GetRootLocations` with nested locations → only roots returned
- `GetRootLocations` system locations included

---

## File Change Summary

| File | Action | Agent |
|------|--------|-------|
| `internal/database/location.go` | MODIFY — add `GetRootLocations` | db-developer |
| `internal/database/location_test.go` | MODIFY — add `GetRootLocations` tests | db-developer |
| `cmd/list/doc.go` | CREATE | golang-ui-developer |
| `cmd/list/list.go` | CREATE | golang-ui-developer |
| `cmd/list/output.go` | CREATE | golang-ui-developer |
| `cmd/list/helpers.go` | CREATE | golang-ui-developer |
| `cmd/list/list_test.go` | CREATE | golang-ui-developer |
| `cmd/list/output_test.go` | CREATE | golang-ui-developer |
| `cmd/root.go` | MODIFY — AddCommand | golang-ui-developer |
| `go.mod` / `go.sum` | MODIFY — `go get github.com/xlab/treeprint` | golang-ui-developer |

---

## Key Design Decisions (Final)

1. **`xlab/treeprint` for rendering**: Eliminates custom prefix/connector arithmetic. Pre-formatted strings are passed as node values, so lipgloss styling (if desired later) can be applied before insertion. No `//nolint` needed anywhere in this command.

2. **Child hints show both item and location counts**: `[Shelf A] (1 item, 3 locations)` — consistent with the recursive header format. Fetching child location count requires one extra `GetLocationChildren` call per child hint; acceptable for local SQLite.

3. **`english.PluralWord` throughout**: No custom `pluralize` helper. `go-humanize` is already in `go.mod`.

4. **Not-found locations are inline, non-fatal**: `runList` never returns a non-zero exit for resolution failures. This allows `wherehouse list Garage OldShelf` to show Garage normally and mark OldShelf inline, which is more useful than aborting.

5. **No `--quiet` flag**: Removed entirely. The command's sole purpose is output.

6. **No `//nolint` on `cobra.ArbitraryArgs`**: It is not a magic number and does not trigger `mnd`.

7. **Items before sub-locations** at each level in both modes, alphabetical within each group.

8. **Brackets `[Name]` for sub-locations**: Visual distinction from items in non-color terminals.

9. **`*` suffix for temporary-use items**: Minimal annotation, no color dependency.

10. **Go-level recursion, no SQL `WITH RECURSIVE`**: Reuses existing `GetLocationChildren` and `GetItemsByLocation`. N+1 is acceptable for local SQLite inventory scale.

11. **`go get github.com/xlab/treeprint` required**: Not yet in `go.mod`. The implementing agent must run this before building.
