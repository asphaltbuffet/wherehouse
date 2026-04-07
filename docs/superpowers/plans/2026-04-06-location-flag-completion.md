# Location Flag Shell Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add shell tab-completion for location-accepting flags (`--in` and `--to`) across `add item`, `add location`, `found`, and `move` commands, returning full canonical paths and excluding system locations.

**Architecture:** A single shared helper `LocationCompletions` in `internal/cli/completions.go` opens the database, queries all locations, filters out system locations (`IsSystem == true`), and returns `FullPathCanonical` values. Each of the four commands registers this helper via `cmd.RegisterFlagCompletionFunc` immediately after their flag definition.

**Tech Stack:** Go, cobra v1.10.2, `internal/database.GetAllLocations`, `internal/cli.OpenDatabase`

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Create | `internal/cli/completions.go` | `LocationCompletions` helper |
| Create | `internal/cli/completions_test.go` | Tests for `LocationCompletions` |
| Modify | `cmd/found/found.go` | Register completion on `"in"` flag |
| Modify | `cmd/add/item.go` | Register completion on `"in"` flag |
| Modify | `cmd/add/location.go` | Register completion on `"in"` flag |
| Modify | `cmd/move/move.go` | Register completion on `"to"` flag |

---

## Task 1: Write the failing test for `LocationCompletions`

**Files:**
- Create: `internal/cli/completions_test.go`

- [ ] **Step 1.1: Write the failing test**

Create `internal/cli/completions_test.go`:

```go
package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// makeCompletionCtx returns a context with config pointing at a freshly
// initialised (auto-migrated) SQLite database in t.TempDir().
func makeCompletionCtx(t *testing.T) (context.Context, *database.Database) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "completion_test.db")

	db, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: dbPath},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)
	return ctx, db
}

func TestLocationCompletions_ReturnsNonSystemLocations(t *testing.T) {
	ctx, db := makeCompletionCtx(t)

	// Create two regular locations and confirm system locations already exist
	err := db.CreateLocation(ctx, "loc001", "Garage", nil, false, 0, "2026-01-01T00:00:00Z")
	require.NoError(t, err)
	err = db.CreateLocation(ctx, "loc002", "Toolbox", &[]string{"loc001"}[0], false, 0, "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	completions, directive := LocationCompletions(ctx)

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, completions, "garage")
	assert.Contains(t, completions, "garage/toolbox")
	// System locations must not appear
	assert.NotContains(t, completions, "missing")
	assert.NotContains(t, completions, "borrowed")
	assert.NotContains(t, completions, "loaned")
	assert.NotContains(t, completions, "removed")
}

func TestLocationCompletions_EmptyDatabase(t *testing.T) {
	ctx, _ := makeCompletionCtx(t)
	// Fresh DB has only system locations — all should be filtered out
	completions, directive := LocationCompletions(ctx)

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, completions)
}

func TestLocationCompletions_ErrorOnMissingConfig(t *testing.T) {
	// Context with no config causes OpenDatabase to fail
	completions, directive := LocationCompletions(context.Background())

	assert.Equal(t, cobra.ShellCompDirectiveError, directive)
	assert.Nil(t, completions)
}
```

- [ ] **Step 1.2: Run the tests to confirm they fail**

```
mise run test -- ./internal/cli/... -run TestLocationCompletions
```

Expected: FAIL — `LocationCompletions` is not defined.

---

## Task 2: Implement `LocationCompletions`

**Files:**
- Create: `internal/cli/completions.go`

- [ ] **Step 2.1: Create the implementation**

Create `internal/cli/completions.go`:

```go
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// LocationCompletions returns the full canonical paths of all non-system
// locations for use as shell completions. It opens its own database connection
// via OpenDatabase(ctx) so that it can be called from cobra RegisterFlagCompletionFunc
// handlers, which run before RunE and outside the command's normal DB lifecycle.
//
// On success it returns (paths, ShellCompDirectiveNoFileComp).
// On any error it returns (nil, ShellCompDirectiveError) so that the shell
// silently offers no completions rather than printing an error.
func LocationCompletions(ctx context.Context) ([]string, cobra.ShellCompDirective) {
	db, err := OpenDatabase(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer db.Close()

	locs, err := db.GetAllLocations(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, loc := range locs {
		if loc.IsSystem {
			continue
		}
		completions = append(completions, loc.FullPathCanonical)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
```

- [ ] **Step 2.2: Run the tests to confirm they pass**

```
mise run test -- ./internal/cli/... -run TestLocationCompletions
```

Expected: PASS (3 tests).

- [ ] **Step 2.3: Run linting**

```
mise run lint
```

Expected: no errors.

- [ ] **Step 2.4: Commit**

```
jj describe -m "feat(cli): add LocationCompletions helper for shell tab-completion"
jj new
```

---

## Task 3: Wire completion into `found --in`

**Files:**
- Modify: `cmd/found/found.go` — `registerFoundFlags` function

- [ ] **Step 3.1: Add RegisterFlagCompletionFunc to `registerFoundFlags`**

In `cmd/found/found.go`, update `registerFoundFlags` to add the completion registration after the `"in"` flag definition:

```go
func registerFoundFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("in", "i", "", "location where item was found (required)")
	_ = cmd.MarkFlagRequired("in")
	_ = cmd.RegisterFlagCompletionFunc("in", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return cli.LocationCompletions(cmd.Context())
	})

	cmd.Flags().BoolP("return", "r", false, "also return item to its home location")
	cmd.Flags().StringP("note", "n", "", "optional note for event")
}
```

- [ ] **Step 3.2: Run existing found tests**

```
mise run test -- ./cmd/found/...
```

Expected: PASS — existing tests unaffected.

- [ ] **Step 3.3: Run linting**

```
mise run lint
```

Expected: no errors.

- [ ] **Step 3.4: Commit**

```
jj describe -m "feat(found): register shell completion for --in flag"
jj new
```

---

## Task 4: Wire completion into `add item --in`

**Files:**
- Modify: `cmd/add/item.go` — `GetItemCmd` function

- [ ] **Step 4.1: Add RegisterFlagCompletionFunc to `GetItemCmd`**

In `cmd/add/item.go`, after the `MarkFlagRequired` call for `"in"`, add the completion registration:

```go
itemCmd.Flags().StringP("in", "i", "", "Location where items are stored (REQUIRED)")
if err := itemCmd.MarkFlagRequired("in"); err != nil {
    panic(fmt.Sprintf("failed to mark 'in' flag as required: %v", err))
}
if err := itemCmd.RegisterFlagCompletionFunc("in", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    return cli.LocationCompletions(cmd.Context())
}); err != nil {
    panic(fmt.Sprintf("failed to register 'in' flag completion: %v", err))
}
```

You will also need to add the `cli` import to `cmd/add/item.go`. The import block should become:

```go
import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)
```

- [ ] **Step 4.2: Run existing add tests**

```
mise run test -- ./cmd/add/...
```

Expected: PASS.

- [ ] **Step 4.3: Run linting**

```
mise run lint
```

Expected: no errors.

- [ ] **Step 4.4: Commit**

```
jj describe -m "feat(add): register shell completion for item --in flag"
jj new
```

---

## Task 5: Wire completion into `add location --in`

**Files:**
- Modify: `cmd/add/location.go` — `GetLocationCmd` function

- [ ] **Step 5.1: Add RegisterFlagCompletionFunc to `GetLocationCmd`**

In `cmd/add/location.go`, after the `Flags().StringP("in", ...)` call, add the completion registration:

```go
locationCmd.Flags().StringP("in", "i", "", "Parent location name or ID (optional, omit for root)")
if err := locationCmd.RegisterFlagCompletionFunc("in", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    return cli.LocationCompletions(cmd.Context())
}); err != nil {
    panic(fmt.Sprintf("failed to register 'in' flag completion: %v", err))
}
```

You will also need to add `"fmt"` and the `cli` import to `cmd/add/location.go`. The import block should become:

```go
import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)
```

- [ ] **Step 5.2: Run existing add tests**

```
mise run test -- ./cmd/add/...
```

Expected: PASS.

- [ ] **Step 5.3: Run linting**

```
mise run lint
```

Expected: no errors.

- [ ] **Step 5.4: Commit**

```
jj describe -m "feat(add): register shell completion for location --in flag"
jj new
```

---

## Task 6: Wire completion into `move --to`

**Files:**
- Modify: `cmd/move/move.go` — `registerMoveFlags` function

- [ ] **Step 6.1: Add RegisterFlagCompletionFunc to `registerMoveFlags`**

In `cmd/move/move.go`, update `registerMoveFlags` to add the completion registration after the `"to"` flag definition:

```go
func registerMoveFlags(cmd *cobra.Command) {
	// Required flags
	cmd.Flags().StringP("to", "t", "", "destination location (required)")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.RegisterFlagCompletionFunc("to", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return cli.LocationCompletions(cmd.Context())
	})

	// Move type flags
	cmd.Flags().Bool("temp", false, "temporary move (preserve origin for return)")

	// Event metadata
	cmd.Flags().StringP("note", "n", "", "optional note for event")
}
```

You will also need to add the `cli` import to `cmd/move/move.go`. Check the current imports and add:

```go
"github.com/asphaltbuffet/wherehouse/internal/cli"
```

- [ ] **Step 6.2: Run existing move tests**

```
mise run test -- ./cmd/move/...
```

Expected: PASS.

- [ ] **Step 6.3: Run linting**

```
mise run lint
```

Expected: no errors.

- [ ] **Step 6.4: Commit**

```
jj describe -m "feat(move): register shell completion for --to flag"
jj new
```

---

## Task 7: Full verification

- [ ] **Step 7.1: Run the full test suite**

```
mise run test
```

Expected: all tests PASS.

- [ ] **Step 7.2: Run linting**

```
mise run lint
```

Expected: zero errors.

- [ ] **Step 7.3: Build and smoke-test completions**

```
mise run build
./wherehouse completion bash > /tmp/wh_completion.bash
source /tmp/wh_completion.bash
```

Then test in your shell (requires a real DB):
```
wherehouse found "socket" --in <Tab>
wherehouse add item "nail" --in <Tab>
wherehouse move socket --to <Tab>
```

Expected: location full canonical paths appear as suggestions; system locations (`missing`, `borrowed`, `loaned`, `removed`) do not appear.

- [ ] **Step 7.4: Final commit**

```
jj describe -m "chore: verify full test suite passes with completion wiring"
jj new
```
