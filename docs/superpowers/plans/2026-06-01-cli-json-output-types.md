# Plan: Centralise CLI JSON output shapes as app-owned projection types

**Issue:** #209
**ADR:** 0014
**Approach:** TDD (red → green → refactor), one command per slice.

## Goal

Move every command's JSON output shape out of `cmd/` and into `internal/app` as a
dedicated, JSON-tagged projection type with a pure projector function. Commands stop
declaring local result structs, stop calling `.String()` on enums, and stop branching
on `--json`. A new `cli.OutputWriter.Render(v, textFn)` owns the json-vs-text branch.

## Decisions (from grilling session — see ADR 0014)

- Enums (`EntityType`, `EntityStatus`, `EventType`) get `MarshalJSON`/`UnmarshalJSON`
  in `internal/inventory`, emitting string names, **erroring** on out-of-range values.
- `app.EntityResult`/`FindResult`/`HistoryResult` stay rich and **untagged** (the web
  layer consumes them field-by-field). The CLI shapes are *separate* projection types.
- New types + projectors live in **one file**: `internal/app/output.go`.
- `Render` stays dumb: `--json`-vs-text only. No generics, no projector arg.
- Text formatters (lipgloss-using) stay in `cmd/`; `internal/app` keeps zero styling deps.
- **`move --json` drops `old_path`** (breaking change; history owns the past).
- `status` is projected from command *inputs* (path, status, note), not a result —
  `ChangeStatus` returns only `error` and is **not** changed.

## Slice ordering (each slice is independently shippable & test-first)

### Slice 0 — Enum JSON marshaling (foundation)

`internal/inventory/entity_type.go`, `entity_status.go`, `event_type.go`.

1. **RED:** table test `TestEntityType_MarshalJSON` — each valid value marshals to its
   quoted string name; the zero value returns an error. `TestEntityType_UnmarshalJSON` —
   round-trips every valid name; unknown string errors; numeric input errors.
   Repeat for `EntityStatus`, `EventType`.
2. **GREEN:** add `MarshalJSON` (lookup name; error if not found — reuse/derive a
   forward map or guard against the existing `*ByName` keyspace) and `UnmarshalJSON`
   (delegate to the existing `Parse*`).
3. **REFACTOR:** ensure `MarshalJSON` and `String()` cannot drift (marshal via `String()`
   where the valid-range check allows).

*Guard:* this changes how these enums serialize **everywhere**, including any
`internal/web` JSON and `out.JSON(map[...])` sites. Grep for marshaling of these types
before merging; confirm `internal/web` templates don't JSON-encode raw enums in a way
that now changes. Existing CLI tests still pass because commands currently pre-stringify.

### Slice 1 — `cli.OutputWriter.Render` (new seam, no command uses it yet)

`internal/cli/output.go`, `output_test.go`.

1. **RED:** `TestOutputWriter_Render` — in JSON mode, `Render(v, textFn)` emits
   `v` as indented JSON and does **not** call `textFn`; in text mode it writes
   `textFn()` and does **not** marshal `v`. Quiet-mode behavior matches the existing
   `Success`/`Println` precedent (decide & assert: text suppressed? JSON still emitted?).
2. **GREEN:** `func (w *OutputWriter) Render(v any, textFn func() string) error`.

### Slices 2–7 — one command each (template below)

Order: **list → scry → history → add → move → status**
(`list`/`scry` first: simplest pure projections; `move`/`status` last: behavior change /
input-projection).

Per-command template:

1. **RED (app):** in `internal/app/output_test.go`, test the projector against a
   hand-built result fixture — asserts field mapping AND the JSON bytes
   (`json.Marshal(ToXxx(...))`) equal the *current* wire format. This is where the
   contract is pinned. (For `move`: assert `old_path` is **absent**.)
2. **GREEN (app):** add the output type + projector to `internal/app/output.go`.
3. **RED (cmd):** the command's existing `--json` golden/integration test should now
   assert against the projector-produced output. For most this is unchanged bytes
   (proving behavior preservation); for `move` update the expectation to the new shape.
4. **GREEN (cmd):** replace the command's local struct + `if cfg.IsJSON()` block with
   `out.Render(app.ToXxx(result), func() string { <existing text formatting> })`.
   Delete the local `xxxResult`/`xxxEntry` type.
5. **REFACTOR:** confirm no `inventory` enum `.String()` or `out.JSON(` remains in the
   command; confirm the text closure preserves conditional formatting (esp. `status`'s
   `(%s)` note suffix and `add`'s interpolation).

Per-command specifics:

| Cmd | Source | Output type | Notes |
|---|---|---|---|
| list | `[]EntityResult` | `ListItem{entity_id,path,type,status}` | drops 4 EntityResult fields |
| scry | `[]FindResult` | `ScryItem{entity_id,path,type,status}` | still drops `Distance` (preserve current behavior; do NOT add it) |
| history | `[]HistoryResult` | `HistoryItem{event_id,event_type,timestamp,actor_user}` | drops `Payload`,`Note` |
| add | `EntityResult` | `AddOutput{entity_id,path}` | |
| move | `EntityResult` | `MoveOutput{entity_id,display_name,path}` | **BREAKING: no `old_path`/`new_path`→`path`**; update text to drop "from" |
| status | inputs (path,newStatus,note) | `StatusOutput{path,status,status_context?}` | projector takes inputs, not a result; `status_context` stays `*string`+`omitempty` |

### Slice 8 — Docs & issue

- `mise run lint` + `mise run test` green.
- `/audit-docs`: update CLAUDE.md command-constructor section with the rule
  "JSON output type + projector live in `internal/app/output.go`; commands call the
  projector + `out.Render`, never marshal directly." Also fix the `items_current`/
  `locations_current` vs actual `entities_current` table-name drift noted during grilling.
- **Update issue #209** (see below).

## Issue #209 update

Post a comment recording the triage outcome and correcting the issue's stale claims,
then the issue tracks this plan/ADR. Key points for the comment:
- Accepted with a **changed approach**: NOT tags on shared `app.*Result` (rejected —
  `EntityResult` is the web layer's rich shared currency; `omitempty` can't do
  caller-conditional field selection). Instead: per-command projection types in
  `internal/app/output.go` + `Render`. See ADR 0014.
- Corrections to the issue body: scry's struct is `{EntityID,Path,Type,Status}` and
  **drops `Distance`** (issue said `{ID,Name,Distance}`); the real transform stringifies
  enums and selects fields, it is not a rename.
- Scope note: includes the deliberate `move --json` breaking change (drops `old_path`).

## Verification

- `rg 'out\.JSON\(' cmd/` → only `remove`/`doctor`/`config`/`rename` remain (out of
  scope this pass — they were not in the grilled set; note as possible follow-ups).
- `rg 'internal/styles|lipgloss' internal/app/` → empty (layering preserved).
- `rg '\.String\(\)' cmd/*/*.go` → no `inventory`-enum stringification in scoped commands.
- Full `mise run test` (race) green; `move` JSON test reflects new shape.

## Out of scope (candidate follow-ups)

- `rename`, `remove`, `doctor`, `config path` JSON shapes (not in the grilled set;
  `doctor` already has its own ADR 0011 contract).
- Enriching `ChangeStatus` to return a result (not needed; status projects from inputs).
