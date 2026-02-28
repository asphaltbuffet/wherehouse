# User Clarifications

## Singleton Pattern
Yes — include refactoring the move command away from the singleton (`GetMoveCmd()`) pattern.
Use the constructor approach: `NewMoveCmd()` (and `NewMoveItemCmd(fn moveItemFunc)` for the injected spy variant).
This eliminates global state, makes test isolation trivial, and removes all `//nolint:reassign` noise.

## Implicit answers from singleton decision:
- **Testability seam**: Use `NewMoveCmd(fn moveItemFunc)` constructor (the singleton removal makes this natural)
- **Wrapper tests**: Keep 1-2 smoke cases for wiring verification (default preference)
- **Output routing tests**: Use spy function approach via the constructor injection (avoids config threading complexity in wiring tests)

## Scope of singleton removal
- Remove `GetMoveCmd()` global var singleton in `cmd/move/move.go`
- Replace with `NewMoveCmd()` / `NewMoveItemCmd(fn)` constructor functions
- Update `cmd/root.go` (or wherever `GetMoveCmd()` is called) to use the new constructor
- Ensure no other callers break

## Revised: Testability Seam — Use mockery v3 instead of spy function

**Reject** the `NewMoveCmd(fn moveItemFunc)` spy injection approach.

**Use mockery v3** to mock the database package interface instead.
- Flag-wiring tests should inject a mock database (via context or constructor), not a spy function
- This avoids seeding entirely for CLI-layer tests (which is excessive for this layer)
- Keep `moveItem()` tested with real in-memory SQLite (where database behavior is relevant)
- The mock approach is idiomatic Go and avoids introducing an ad-hoc `moveItemFunc` type

**Questions to resolve for the architect:**
1. Does `internal/database` already expose an interface, or does it use a concrete struct `*database.Database`?
2. Does the project already use mockery anywhere (check for `.mockery.yaml` or `mocks/` directories)?
3. How does `openDatabase(cmd.Context())` work — does it pull from context, or open a fresh connection? If context-based, mocking via context injection is straightforward.
4. What interface methods does the CLI-layer move command actually need from the database? (resolveLocation, resolveItemSelector, moveItem etc.)

The plan should add a minimal interface in `internal/database` (or `cmd/move`) covering only the methods the move command needs, generate a mock with mockery v3, and use that mock in flag-wiring/output-routing tests.
