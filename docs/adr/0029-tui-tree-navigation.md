# TUI navigation replaces drill-in/out list with a persistent expand/collapse tree

The current TUI navigation shows exactly one level at a time: drilling into a node replaces the entire list with that node's children; drilling out reloads the parent level. This is disorienting because the user loses spatial context — they cannot see where they are in the hierarchy relative to siblings or ancestors.

The web UI shows a persistent left-panel tree where nodes expand and collapse in place. This ADR aligns the TUI with that model.

## Decisions

### Widget: hand-rolled tree over `bubbles/viewport`

`charm.land/bubbles/v2` (v2.1.0) does not include a tree component — the official PR (#639) was closed without merging in February 2026. The third-party `teatree` library exists but carries a provisional (pre-v1) API with no stability guarantee.

The tree widget is hand-rolled. It maintains a `[]treeNode` flat slice of currently visible (non-collapsed) nodes. Each `treeNode` carries:

```
treeNode {
    entityID     string
    displayName  string
    status       inventory.EntityStatus
    hasChildren  bool
    loaded       bool   // true once GetChildren has been called for this node
    expanded     bool
    depth        int
    parentID     string // "" for root nodes
}
```

`bubbles/viewport` handles scrolling. `bubbles/list` is removed entirely — its fuzzy filtering was per-level only and is superseded by the existing `scry` command (`s`), which performs inventory-wide Levenshtein search.

### Loading: lazy on expand

Root entities are loaded at startup via `GetRootEntities` (as today). Children are loaded on first expand of a node via `GetChildren`. A node's `loaded` flag is set to `true` once its children have been spliced in. Re-expanding a node that is already loaded does not re-fetch — the children are already in the slice, just hidden.

### Expand/collapse keybindings (extends ADR 0026)

| Key | Behaviour |
|---|---|
| `l` / `→` / `enter` | If collapsed and has children: expand (load if needed). If expanded: collapse. If leaf: no-op. |
| `h` / `←` | If expanded: collapse. If collapsed or leaf: move cursor to parent node. |
| `j` / `↓` | Move cursor down one visible node. |
| `k` / `↑` | Move cursor up one visible node. |

All action keybindings (`a`, `L`, `b`, `x`, `r`, `f`, `H`, `s`, `d`) are unchanged. The `Filter` key (`/`) is removed — `bubbles/list` and its filter are gone.

### Mutation refresh: targeted

After any mutation, only the affected node's parent's children are reloaded via `GetChildren`. The updated children are spliced back into the `[]treeNode` at that parent's position, preserving the expanded/collapsed state of all other nodes. The cursor repositions on the mutated entity by ID. If the entity is absent from the refreshed children (e.g. a `return` on a `borrowed` entity sets it to `removed`, which `GetChildren` excludes), the cursor falls to the next visible node.

When a mutation adds a child to a node that is currently collapsed, the node's `hasChildren` flag is updated but the node is not auto-expanded.

### Scry navigation: reveal in tree

When the user selects a result in `modeScry`, all ancestor nodes from root to the target entity are expanded, lazy-loading any that have not yet been fetched. Previously expanded siblings remain expanded. The cursor is placed on the target entity. `scryNavigatedMsg` carries `pathStack` (which already contains the ancestor IDs needed to drive the reveal chain).

## Consequences

- `bubbles/list`, `delegate.go`, `toListItems`, `selectByID`, `pathStack`, `parentStack`, `drillDown`, `drillUp`, `loadLevel`, `handleLevelMsg`, `childrenLoadedMsg`, `levelRestoredMsg`, and `rootsLoadedMsg` are removed or replaced.
- `Model.list list.Model` is replaced by `Model.tree treeModel` (the new viewport-backed widget) and `Model.nodes []treeNode` (the authoritative node slice, including collapsed nodes).
- The crumb bar (`renderNavPane` breadcrumb) is removed — ancestry is visible directly in the tree.
- `childRefreshMsg` is replaced by a `treeRefreshMsg` that carries the parent ID and its reloaded children.
- The `Filter` keybinding is removed from `keyMap`. The help line gains `expand`/`collapse` labels.
- ADR 0027 (mode-based view switching) is unaffected — `modeBrowse`, `modeForm`, `modeConfirm`, `modeScry` remain. The tree widget lives entirely within `modeBrowse`.
- ADR 0026 keybinding table gains the expand/collapse semantics for `l`/`h` described above; all action keys are unchanged.

## Considered Options

**Keep `bubbles/list` with a flattened indented view (A3).** Rejected — `bubbles/list` provides fuzzy filtering as its primary value-add; without it the component is just a cursor + scroll wrapper with extra coupling. A `bubbles/viewport` with a `[]treeNode` slice is simpler and fully owned.

**Use `teatree` (third-party).** Rejected — provisional API (pre-v1), single-author library. The hand-rolled implementation is comparable in size (~200 lines) and has no external dependency risk.

**Reset tree on scry navigation.** Rejected — destroying the user's expanded state on every scry result contradicts the spatial-context benefit that motivated the tree in the first place.
