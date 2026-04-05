# Action Items: UUID-to-nanoid Migration Review

**Source:** /home/grue/dev/wherehouse/ai-docs/sessions/20260302-092754/03-reviews/iteration-01/internal-review.md
**Date:** 2026-03-02

---

## IMPORTANT Fixes (4)

### 1. DRY: Remove duplicate `looksLikeNanoid` in migrate.go

**Files:** `/home/grue/dev/wherehouse/internal/cli/migrate.go` lines 127-144
**Action:** Delete `looksLikeNanoid` and `isNanoidChar` functions. Replace all call sites with `LooksLikeID` (already in same package, `selectors.go`). Both functions are functionally identical -- 10-char alphanumeric check.

### 2. Use `nanoid.IDLength` instead of hardcoded `10` in `LooksLikeID`

**File:** `/home/grue/dev/wherehouse/internal/cli/selectors.go` line 53
**Action:** Replace `const idLength = 10` with a reference to `nanoid.IDLength`. This maintains a single source of truth for ID length. Must be done together with item 1 since item 1 depends on `LooksLikeID` being canonical.

### 3. Fix stale "UUID" references in `selectors.go` doc comments

**File:** `/home/grue/dev/wherehouse/internal/cli/selectors.go` lines 65-74
**Action:** Replace "UUID" with "ID" in the `ResolveItemSelector` function doc comments:
- Line 65: `"resolves an item selector to an item UUID"` -> `"...to an item ID"`
- Line 67: `"UUID (exact ID, verified against database)"` -> `"ID (exact, verified against database)"`
- Line 74: `"Returns the item UUID string"` -> `"Returns the item ID string"`

### 4. (Awareness) Event payload string replacement ordering

**File:** `/home/grue/dev/wherehouse/internal/cli/migrate.go` lines 285-295
**Action:** No code change required for this migration. The risk is theoretical only because UUIDs (36 chars) cannot be substrings of nanoids (10 chars). Document this assumption with a code comment in `rewriteEventPayloads` explaining why the naive `strings.ReplaceAll` approach is safe for UUID->nanoid specifically.
