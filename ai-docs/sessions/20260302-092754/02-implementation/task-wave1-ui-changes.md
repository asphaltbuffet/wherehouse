# Wave 1 UI Changes — Files Modified

## Subtask A: Remove uuidPrefixLength from history output

**File: `cmd/history/output.go`**
- Removed `uuidPrefixLength = 8` constant from the `const` block
- Replaced the conditional truncation logic in `resolveLocationPath`:
  - Before: if `len(locationID) >= uuidPrefixLength`, return `location:<first-8-chars>`; else return full ID
  - After: always return `location:<full-id>` on lookup failure (no truncation)

## Subtask B: Update doc strings across cmd/**

Comments and doc strings updated (no logic changed):

| File | Change |
|------|--------|
| `cmd/history/resolver.go` | "item UUID" → "item ID" in function comment |
| `cmd/history/history.go` | "UUID: --id \<uuid\>" → "ID: --id \<id\>" in Long; flag description "Item UUID" → "Item ID"; inline comment "Resolve item selector to UUID" → "to ID" |
| `cmd/lost/helpers.go` | "item UUID" → "item ID"; "UUID (exact ID)" → "ID (exact match)" |
| `cmd/lost/doc.go` | "UUID: Exact ID match" → "ID: Exact ID match"; example UUID → `aB3xK9mPqR` |
| `cmd/lost/lost.go` | "UUID: 550e8400..." → "ID: aB3xK9mPqR"; example UUID in Examples → `aB3xK9mPqR` |
| `cmd/move/helpers.go` | "resolve a name or UUID to a location UUID" → "resolve a name or ID to a location ID"; "Full UUID" → "Full ID"; "item UUID" → "item ID"; "UUID (exact ID)" → "ID (exact match)" |
| `cmd/move/doc.go` | "UUID: 550e8400..." → "ID: aB3xK9mPqR"; example UUID → `aB3xK9mPqR` |
| `cmd/move/move.go` | "UUID: 550e8400..." → "ID: aB3xK9mPqR"; two example UUID strings → `aB3xK9mPqR` |
| `cmd/loan/helpers.go` | "item UUID" → "item ID"; "UUID (exact ID)" → "ID (exact match)" |
| `cmd/loan/loan.go` | "UUID: 550e8400..." → "ID: aB3xK9mPqR"; example UUID → `aB3xK9mPqR` |
| `cmd/found/doc.go` | "UUID: 550e8400..." → "ID: aB3xK9mPqR" |
| `cmd/found/found.go` | "UUID: 550e8400..." → "ID: aB3xK9mPqR" |
| `cmd/list/helpers.go` | "location name or UUID to the location UUID string" → "location name or ID to the location ID string"; "full UUID" → "full ID"; "location UUID string" → "location ID string" |
| `cmd/add/helpers.go` | "resolve a name or UUID to a location UUID" → "resolve a name or ID to a location ID"; "Full UUID" → "Full ID" |
| `cmd/add/location.go` | "unique UUID" → "unique ID" in Long; "Generate UUID v7" → "Generate ID"; error message "generate UUID for location" → "generate ID for location" |
| `cmd/add/item.go` | "unique UUID" → "unique ID" in Long; "canonical name or UUID" → "canonical name or ID"; "Resolve location to UUID" → "Resolve location to ID"; "Generate UUID v7" → "Generate ID"; error "generate UUID" → "generate ID" |

## Verification

- `go build ./cmd/...` — PASS
- `go test ./cmd/history/...` — PASS (0.003s)
