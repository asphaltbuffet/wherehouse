# Task B: Files Created/Modified

## New Files

### `cmd/list/doc.go`
Package documentation for the `list` package.

### `cmd/list/helpers.go`
Thin wrappers over `internal/cli`:
- `openDatabase(ctx)` → `cli.OpenDatabase(ctx)`
- `resolveLocation(ctx, db, input)` → `cli.ResolveLocation(ctx, db, input)` (returns UUID string)

### `cmd/list/output.go`
Tree rendering and JSON output:
- `LocationNode` struct — data model for one node in the rendered tree
- `ItemJSON`, `LocationJSON`, `OutputJSON` — JSON output types (renamed from List* to avoid revive stutter lint)
- `locationHeader(name, itemCount, locationCount)` — formats `"Garage (3 items, 2 locations)"`
- `populateTree(branch, node)` — adds items and children to a treeprint.Tree branch
- `renderTree(w, nodes)` — renders slice of root nodes to io.Writer, blank lines between roots
- `toJSON(nodes)` / `nodeToJSON(node)` — converts LocationNode tree to JSON structs

### `cmd/list/list.go`
Cobra command + builders:
- `GetListCmd()` — returns the `list` cobra.Command with `--recurse`/`-r` flag
- `runList(cmd, args)` — main entry point: open DB, build nodes, render or JSON encode
- `buildNodes(ctx, db, args, recurse)` — dispatches to root or arg-based resolution
- `buildRootNodes(ctx, db, recurse)` — uses `db.GetRootLocations`
- `buildNode(ctx, db, loc, recurse)` — dispatches to flat or recursive builder
- `buildLocationNodeFlat(ctx, db, loc)` — items + hint-only children with item/location counts
- `buildLocationNodeRecursive(ctx, db, loc)` — full subtree (recursive)

### `cmd/list/list_test.go`
23 tests covering:
- `buildLocationNodeFlat`: items populated, children hint-only, counts correct, empty location
- `buildLocationNodeRecursive`: full tree, empty location
- `buildNodes`: no args uses roots, unknown arg produces NotFound node, mixed args, recurse
- `toJSON`: not-found node, single location, JSON roundtrip
- `renderTree`: empty, not-found, single location, connectors, temp-use star, flat hints, multiple roots, pluralization, recursive

## Modified Files

### `cmd/root.go`
- Added `listcmd "github.com/asphaltbuffet/wherehouse/cmd/list"` import
- Added `rootCmd.AddCommand(listcmd.GetListCmd())` in `GetRootCmd()`

### `go.mod` / `go.sum`
- Added `github.com/xlab/treeprint v1.2.0` (new dependency for tree rendering)
