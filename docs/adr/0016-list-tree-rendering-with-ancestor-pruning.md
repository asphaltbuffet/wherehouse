# list command renders as a tree with ancestor pruning

The `list` command is a hierarchy browser, not a search tool — its primary use case is "what's in X?" The default text output renders all entities as a tree using `charm.land/lipgloss/v2/tree` (already in the module graph). When `--status` or `--type` filters are active, nodes that neither match the filter nor are ancestors of a matching node are omitted entirely. Ancestor nodes are shown in a dimmed style to provide structural context without claiming to be results.

## Considered Options

**Ghost parents (dim all non-matching nodes):** Rejected. With filters like `--status missing` over a full inventory, ghost nodes would outnumber matches by a large margin, making the output harder to read than a flat list.

**Flat list when filtered:** Rejected. The primary use case (`--under X --status missing`) is inherently a subtree browse — a tree with pruned branches is more useful than a flat list of full paths when the scope is bounded.

**Ancestor pruning (chosen):** A node appears if it matched, or if it is on the path to a node that matched. This gives structural context proportional to the result set, with no noise.

## Consequences

- `filterEntities` continues to return only matching entities. The tree builder in `runList` receives both the full entity list and the filtered list, constructs a matched-ID set, and uses `entitypath.IsAncestor` to decide whether each unmatched node is structural scaffolding.
- Tags and verbose formatting apply only to matched nodes, not ancestor-only nodes.
- `--json` output is unaffected — it continues to return only matched entities as a flat array.
