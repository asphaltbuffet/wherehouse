# Code Review: UUID-to-nanoid Migration

**Date:** 2026-03-02
**Reviewer:** code-reviewer agent
**Assessment:** CHANGES NEEDED
**Linting:** PASS (0 issues)
**Tests:** PASS (655 tests, 5 skipped)

---

## Strengths

- **Clean nanoid package** (`internal/nanoid/nanoid.go`): Minimal, correct wrapper. Alphabet is 62 chars (A-Za-z0-9), length 10, proper error propagation. `MustNew` panics with context. Exported `Alphabet` constant enables test validation. No issues.

- **Atomic migration transaction**: `MigrateDatabase` correctly wraps all writes in a single `ExecInTransaction` call. `ExecInTransaction` uses `defer tx.Rollback()` pattern correctly. If any step fails, all changes are rolled back.

- **Idempotency implementation**: The `looksLikeNanoid` check in `buildMigrateMapping` correctly skips entities whose IDs are already in nanoid format, mapping them to themselves. The `oldID == newID` guard in each `migrate*Rows` function avoids unnecessary UPDATE statements. Running the migration twice produces identical results.

- **System location IDs are deterministic and consistent**: Both `schema_metadata.go` (seed) and `migrate.go` (migration) use the same `sys0000001/2/3` values. No mismatch risk.

- **Thin-cmd pattern correctly followed**: `cmd/migrate/database.go` delegates entirely to `cli.MigrateDatabase`. Matches `cmd/initialize/` pattern exactly (parent + subcommand, `Get*Cmd` lazy init, no business logic).

- **No remaining `google/uuid` imports in production code**: Confirmed zero `.go` files import `google/uuid`. It remains in `go.mod` only as `// indirect` (transitive dependency).

- **Test quality**: All test files use `testify/assert` and `testify/require`. No `t.Fatal`/`t.Error` found. Range-over-int used in nanoid tests. Good coverage of dry-run, idempotency, system ID determinism, and payload rewriting.

- **Prepared statements throughout**: All SQL in migration uses parameterized queries (`?` placeholders). No string concatenation with user data. The `rewriteEventPayloads` function correctly uses `tx.PrepareContext` for batch updates.

- **Documentation** (`docs/migration-nanoid.md`): Complete, accurate, actionable. Covers backup, dry-run, apply, verify, idempotency, rollback, and external references.

---

## Concerns

### IMPORTANT (should fix before merge)

**I-1. Duplicate logic: `looksLikeNanoid` / `isNanoidChar` in migrate.go vs `LooksLikeID` in selectors.go**

File: `/home/grue/dev/wherehouse/internal/cli/migrate.go` lines 127-144
File: `/home/grue/dev/wherehouse/internal/cli/selectors.go` lines 52-63

`looksLikeNanoid` in migrate.go is functionally identical to `LooksLikeID` in selectors.go. Both check for exactly 10 alphanumeric characters. The only difference is that `LooksLikeID` hardcodes `const idLength = 10` instead of referencing `nanoid.IDLength`, and uses inline range checks instead of a helper function.

Both are in the same package (`cli`), so `looksLikeNanoid` can simply call `LooksLikeID` (or be replaced by it entirely). Additionally, `LooksLikeID` should reference `nanoid.IDLength` instead of hardcoding `10` to maintain a single source of truth for the ID length.

This violates the DRY principle stated in AGENTS.md: "DRY/search first: before adding new helpers or logic, search for prior art and reuse or extract a shared helper instead of duplicating."

**I-2. Stale doc comments referencing "UUID" in selectors.go**

File: `/home/grue/dev/wherehouse/internal/cli/selectors.go` lines 65-74

The `ResolveItemSelector` function doc still says:
- Line 65: `"resolves an item selector to an item UUID"`
- Line 67: `"UUID (exact ID, verified against database)"`
- Line 74: `"Returns the item UUID string"`

These should say "ID" not "UUID" to match the migration. The doc string updates in Task 12 missed this file's function comments.

**I-3. Event payload string replacement could cause cascading corruption on repeated IDs**

File: `/home/grue/dev/wherehouse/internal/cli/migrate.go` lines 285-295

The `rewriteEventPayloads` function iterates over `mapping` (a `map[string]string`) and applies `strings.ReplaceAll` for each old->new pair. Because Go map iteration order is non-deterministic, if a newly generated nanoid happens to be a substring of (or identical to) another old UUID, the replacement could corrupt data.

In practice, this is extremely unlikely because UUIDs are 36 characters and nanoids are 10 characters -- a 10-char nanoid will not match a 36-char UUID pattern in JSON. However, the code does not validate this invariant. A defensive check would be to verify no new ID appears as a substring of any old ID before applying replacements.

**Risk assessment:** LOW in practice for UUID->nanoid migration (length difference prevents substring matches), but the algorithm is theoretically unsafe for arbitrary ID mappings. Since this is a one-time migration from 36-char to 10-char IDs, the practical risk is negligible. Noting for awareness rather than as a blocking issue.

**I-4. `LooksLikeID` hardcodes length instead of using `nanoid.IDLength`**

File: `/home/grue/dev/wherehouse/internal/cli/selectors.go` line 53

```go
const idLength = 10
```

This should reference `nanoid.IDLength` to maintain a single source of truth. If `IDLength` ever changes, this local constant would silently diverge.

### MINOR (optional improvements)

**M-1. `migrate_test.go` unused strings import workaround**

File: `/home/grue/dev/wherehouse/internal/cli/migrate_test.go` line 287

```go
var _ = strings.Contains
```

This is a workaround to avoid an unused import error. The `strings` package is imported but never used in the test file. Remove the import and this line.

**M-2. Migration test uses empty database (no seed data)**

File: `/home/grue/dev/wherehouse/internal/cli/migrate_test.go` lines 262-284

`setupTestDBForMigration` creates a database with migrations applied but no seed data beyond system locations. The `TestMigrateDatabase_EventPayloads_UpdatedCorrectly` test checks that payloads no longer contain UUID patterns, but with only system location events (which are inserted via `INSERT OR IGNORE`, not through the event system), there may be no event payloads to verify at all.

The test would be stronger if it seeded the database with `SeedTestData` or manually inserted UUID-formatted test events, then verified those specific payloads were rewritten. As-is, the test may pass vacuously (no events to check).

**M-3. `printPostMigrationInstructions` removed from implementation**

The plan specified a `printPostMigrationInstructions` function and a corresponding test (`TestMigrateDatabase_PrintsPostMigrationInstructions`). The implementation in `migrate.go` does not include `printPostMigrationInstructions` -- the function was dropped. The test was also dropped from `migrate_test.go`. This is consistent (no orphan test), but the plan's intent to provide post-migration guidance to the user was not fulfilled. The documentation file covers this, so the practical impact is low.

---

## Questions

**Q-1.** The system locations in a fresh database are seeded with `sys0000001/2/3` IDs via `schema_metadata.go`. But older databases have UUID-format system location IDs. When the `MigrateDatabase` function runs, it looks up system locations by `canonical_name` (`missing`, `borrowed`, `loaned`). Is there a scenario where a system location's canonical name could have been changed, causing the migration to assign it a random nanoid instead of the deterministic `sys*` ID?

**Q-2.** The migration rewrites `events.payload` JSON via naive string replacement. Are there any event types whose payload contains free-text fields (notes, descriptions) where a UUID-like string might appear as user content rather than as an entity ID?

---

## Specific Risk Assessment (from review request)

**Risk: Could event payload string replacement corrupt non-ID data?**
Practical risk is negligible for UUID->nanoid migration. UUIDs are 36 chars with specific hex+dash format. The `strings.ReplaceAll` searches for exact 36-char UUID matches. A false positive would require a user's free-text note to contain the exact same 36-char UUID string as an entity ID -- effectively impossible in a personal inventory tool. If this migration pattern were ever reused for shorter IDs, the risk would increase.

**Risk: Is the migration truly atomic?**
Yes. All writes happen inside a single `ExecInTransaction` call. SQLite transactions are atomic. If any UPDATE fails, the deferred `tx.Rollback()` reverts everything.

**Risk: Does `looksLikeNanoid` duplicate `LooksLikeID`?**
Yes. They are functionally identical. See I-1 above.

**Risk: Any remaining uuid imports in production code?**
No. Zero matches for `google/uuid` in any `.go` file. It remains only as `// indirect` in `go.mod`.

**Risk: Does `cmd/migrate/` match `cmd/initialize/` pattern?**
Yes, exactly. Both use lazy-init `Get*Cmd()` pattern, parent+subcommand structure, and delegate to `internal/cli/` for business logic.

---

## Summary

| Category | Count |
|----------|-------|
| CRITICAL | 0 |
| IMPORTANT | 4 |
| MINOR | 3 |

**Assessment:** CHANGES NEEDED

The migration implementation is solid, atomic, idempotent, and well-tested. The four IMPORTANT issues are all straightforward to fix:

1. Replace `looksLikeNanoid`/`isNanoidChar` with calls to `LooksLikeID` (DRY)
2. Fix stale "UUID" doc comments in `selectors.go`
3. (Awareness) Payload string replacement ordering -- no code change needed for this migration
4. Use `nanoid.IDLength` in `LooksLikeID` instead of hardcoded `10`

**Estimated effort:** 15-30 minutes for items 1, 2, and 4.
**Risk level:** Low -- the migration logic is correct and well-guarded.
**Testability:** Good -- all critical paths have test coverage.
