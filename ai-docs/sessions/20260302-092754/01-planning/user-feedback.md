# User Feedback on Plan

## Feedback 1: Thin cmd/migrate Pattern (already incorporated)
- `cmd/migrate/database.go` is a thin wrapper only
- All business logic lives in `internal/cli/migrate.go`
- `RunE` calls `cli.MigrateDatabase(cmd, db, dryRun)` or similar
- Matches how cmd/initialize/database.go delegates to internal/cli/database.go

## Feedback 2: TDD for the Entire Plan (already incorporated)
The entire implementation must follow strict TDD — tests written first, watched to fail, then minimal implementation.

## Feedback 3: Testify Only — t.Fatal and t.Error Are Forbidden
ALL test code MUST use testify. `t.Fatal`, `t.Error`, `t.Errorf`, `t.Fatalf` are FORBIDDEN.

**Required:**
- Use `github.com/stretchr/testify/assert` for non-fatal assertions
- Use `github.com/stretchr/testify/require` for fatal assertions (stops test immediately on failure)
- `require` is the testify equivalent of `t.Fatal` — use it when subsequent test steps depend on this passing
- `assert` is the testify equivalent of `t.Error` — use it when the test should continue after failure

**Examples:**
```go
// FORBIDDEN:
if err != nil {
    t.Fatalf("expected no error, got %v", err)
}
if got != want {
    t.Errorf("got %v, want %v", got, want)
}

// REQUIRED:
require.NoError(t, err)
assert.Equal(t, want, got)
```

All example test code in the plan must be updated to use testify.
All test-writing instructions to golang-tester must explicitly require testify.
