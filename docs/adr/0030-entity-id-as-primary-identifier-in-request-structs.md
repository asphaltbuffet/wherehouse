# Mutations accept `entity_id`; path resolution is a CLI/UI boundary concern

Request structs in `internal/app` were originally built around `EntityPath` (a colon-separated display path) as the sole way to identify an existing entity for mutation. This made sense when the CLI was the only caller: the user types a path, the app resolves it.

As the web UI and TUI evolved, both layers acquired `entity_id` from their own context (URL path values, tree node state) but were forced to round-trip through path resolution anyway — fetching the entity by ID, discarding the ID, passing `FullPathDisplay` into the request, and re-resolving it back to the ID inside the app layer.

## Decisions

### Mutations take `entity_id`; path is a CLI input concern

The following request structs replace `EntityPath string` with `EntityID string`:

- `RenameEntityRequest`
- `ReparentEntityRequest` — additionally `NewParentPath` → `NewParentID`
- `RemoveEntityRequest`
- `ChangeStatusRequest`
- `TagEntityRequest`
- `ListTagsRequest`
- `GetHistoryRequest` — `EntityPath` removed; `EntityID` was already present

The app layer never performs path resolution for these operations. If `EntityID` is empty the method returns an error immediately.

### `LookupEntityByPath` is the single path→ID bridge

`resolveEntityPath` (previously private) is promoted to a public `App` method:

```go
func (a *App) LookupEntityByPath(ctx context.Context, path string) (EntityResult, error)
```

CLI commands that accept a path argument call this method before constructing a mutating request.

### Creation requests retain `ParentPath`

`CreateEntityRequest.ParentPath` and `BorrowEntityRequest.ParentPath` are unchanged. These fields carry genuine user input (the user types or selects a destination path), not a reference to an entity the caller already holds. PathResolution for creation is the CLI's responsibility as today.

### TUI `App` interface shape is unchanged

The TUI's narrow `App` interface (`internal/tui/app.go`) retains its status-specific method names (`MarkLoaned`, `MarkLost`, etc.). Callers populate `EntityID` from the selected tree node's `EntityResult`. The interface shape is a separate concern.

### Web handlers pass `entity_id` directly

HTTP handlers extract `entityID` from the URL path value and pass it into request structs without fetching the entity first to obtain its `FullPathDisplay`. The redundant `buildDetailData` → `FullPathDisplay` detour is removed from `handleEditName` and `handleToggleMissing`.

## Consequences

- The private `resolveEntityPath`, `resolveEntityPathTx`, `resolveEntityPathWith` methods are collapsed into `LookupEntityByPath` (public) and a transaction-scoped internal helper if still needed.
- Every CLI command that previously passed a raw path string into a request struct gains a `LookupEntityByPath` call before the request is built.
- Web and TUI callers that already hold `entity_id` make no additional store round-trips.
- The app layer's mutation methods become testable without constructing a valid entity path — tests pass an `entity_id` directly.

## Considered Options

**Keep dual-mode (EntityPath + EntityID) on each request struct.** Rejected — leaves a permanent ambiguity in every method about which field takes precedence and which callers use which. `GetHistoryRequest` already demonstrated the dual-mode pattern; its inconsistency with the rest of the structs is what motivated this ADR.

**Push path→ID resolution into the store layer.** Rejected — the filtering step (matching `FullPathCanonical` across candidates with the same canonical leaf name) is application-level disambiguation, not a SQL predicate. The store already provides `GetEntitiesByCanonicalName`; the selection logic belongs in `LookupEntityByPath`.
