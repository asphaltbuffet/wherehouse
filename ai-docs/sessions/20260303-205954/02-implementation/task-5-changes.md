# Task 5: Help Text Normalization

## Summary

Normalized help text across all `cmd/` subdirectory cobra command files for consistency.

## Changes Made

### 1. Capitalize Short descriptions (4.2 from review)

**`cmd/root.go`**
- `Short`: `"a personal inventory tracker"` → `"A personal inventory tracker"`

**`cmd/migrate/migrate.go`**
- `Short`: `"run data migration operations"` → `"Run data migration operations"`

**`cmd/migrate/database.go`**
- `Short`: `"migrate database IDs from UUID to nanoid format"` → `"Migrate database IDs from UUID to nanoid format"`

### 2. Normalize example comment style to `# comment` format (4.1 from review)

All examples now use `# description` inline comment style, consistent with `find`, `scry`, and `add/location` (the existing good examples).

**`cmd/migrate/migrate.go`**
- `"  wherehouse migrate database        Migrate IDs from UUID to nanoid format"` → `"  wherehouse migrate database        # Migrate IDs from UUID to nanoid format"`

**`cmd/migrate/database.go`**
- Two example lines updated to use `#` comment style

**`cmd/config/config.go`**
- Six example lines converted from tab-aligned description format to `# comment` format

**`cmd/config/init.go`**
- Four example lines converted to `# comment` format

**`cmd/config/get.go`**
- Four example lines converted to `# comment` format

**`cmd/config/set.go`**
- Three example lines had no inline description; added `# comment` annotations

**`cmd/config/check.go`**
- Single example line converted to `# comment` format

**`cmd/config/edit.go`**
- Two example lines converted to `# comment` format

**`cmd/config/path.go`**
- Two example lines converted to `# comment` format

**`cmd/initialize/initialize.go`**
- Two example lines converted to `# comment` format

**`cmd/history/history.go`**
- Four example lines converted to `# comment` format with descriptive comments added

**`cmd/list/list.go`**
- Five example lines converted to `# comment` format with descriptive comments added

### 3. Fix column alignment in add parent command (4.3 from review)

**`cmd/add/add.go`**
- Fixed misaligned description columns in the two example lines:
  - Before: `location` line had no gap, `item` line had excess padding
  - After: Both lines use consistent 2-space gap before `# comment`

### 4. Add examples to commands missing them

**`cmd/initialize/database.go`**
- Added an `Examples:` section (the subcommand had no examples despite meaningful usage)

## Files Modified

| File | Change |
|------|--------|
| `cmd/root.go` | Capitalize Short |
| `cmd/migrate/migrate.go` | Capitalize Short, add `#` to example |
| `cmd/migrate/database.go` | Capitalize Short, add `#` to examples |
| `cmd/add/add.go` | Fix column alignment, add `#` to examples |
| `cmd/config/config.go` | Convert examples to `#` style |
| `cmd/config/init.go` | Convert examples to `#` style |
| `cmd/config/get.go` | Convert examples to `#` style |
| `cmd/config/set.go` | Add `#` comments to examples |
| `cmd/config/check.go` | Add `#` comment to example |
| `cmd/config/edit.go` | Convert examples to `#` style |
| `cmd/config/path.go` | Convert examples to `#` style |
| `cmd/initialize/initialize.go` | Convert examples to `#` style |
| `cmd/initialize/database.go` | Add Examples section |
| `cmd/history/history.go` | Convert examples to `#` style |
| `cmd/list/list.go` | Convert examples to `#` style |

## Unchanged (already consistent)

- `cmd/find/find.go` — already uses `#` comment style
- `cmd/scry/scry.go` — already uses `#` comment style
- `cmd/add/item.go` — already uses `#` comment style
- `cmd/add/location.go` — already uses `#` comment style
- `cmd/found/found.go` — no comment style needed (bare examples are minimal)
- `cmd/loan/loan.go` — already uses bare format consistently
- `cmd/lost/lost.go` — already uses bare format consistently
- `cmd/move/move.go` — already uses bare format consistently
