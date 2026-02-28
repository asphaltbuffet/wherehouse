# User Request

Refactor unit tests in `cmd/move/` using the cobra-cli-testing research document as a design reference.

## Reference Document
`docs/research/testing/cobra-cli-testing.md`

## Goals
- Apply the patterns and anti-patterns from the cobra-cli-testing research document
- Improve test quality, coverage, and maintainability in `cmd/move/`
- Ensure tests follow the "thin entrypoint" principle
- Eliminate tests for behaviors Cobra already guarantees
- Add tests for things that MUST be tested (flag-to-parameter wiring, semantic validation, error propagation, output routing)
- All tests must pass, zero linter errors

## Target Files
- `cmd/move/item_test.go`
- `cmd/move/helpers_test.go`
- `cmd/move/item.go` (may need minor refactoring to support testability)
- `cmd/move/helpers.go` (may need minor refactoring to support testability)
