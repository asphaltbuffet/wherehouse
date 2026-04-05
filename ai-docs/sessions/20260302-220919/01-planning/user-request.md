# User Request

Refactor `EventType` in the `Event` struct to use the typed `EventType` defined in `internal/database/eventTypes.go`.

## Requirements

1. **Make `EventType` Stringer-compatible**: Add a `//go:generate` directive to automatically create string representations of the enum using the `stringer` tool.

2. **Replace all uses of `EventType string`** with `EventType EventType` (typed int enum instead of raw string).

3. **Database storage must remain string-based**: The database must still store the string representation of the event type so that changing the integer backing of the enum cannot break existing database data.

4. **Strict TDD process required**: Write failing tests before implementing changes.

## Key File
- `internal/database/eventTypes.go` — defines the `EventType` int enum with stringer comments
