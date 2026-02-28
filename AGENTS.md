# Wherehouse Agent Guide

## Project Overview

Wherehouse is an **event-sourced** CLI/TUI inventory tracker that answers "Where did I put my 10mm socket?". Built with Go + SQLite, it uses events as source of truth with disposable projections for fast queries. Multi-user attribution only (no permissions).

The project implements event sourcing architecture where:
- **Events** are the source of truth (append-only log)
- **Projections** are derived state (rebuildable)
- **Replay** by `event_id` order ensures determinism
- **No undo** - corrections create new compensating events

## Project Structure

```
wherehouse/
├── cmd/                    # CLI commands (cobra)
├── internal/
│   ├── config/            # Configuration management
│   ├── database/          # SQLite operations (COMPLETE)
│   │   ├── database.go    # Connection, initialization
│   │   ├── events.go      # Event storage
│   │   ├── projections.go # Projection CRUD
│   │   ├── replay.go      # Event replay engine
│   │   ├── validation.go  # Integrity checks
│   │   └── migrations/    # SQL schema migrations
│   ├── events/            # Event type definitions
│   ├── models/            # Domain entities
│   ├── projections/       # Projection builders
│   ├── validation/        # Business rule enforcement
│   └── cli/               # CLI command implementations
├── docs/
│   └── DESIGN.md          # Full design specification
└── .claude/
    └── knowledge/         # AI agent context
```

## Key Technologies

- **Language**: Go 1.25+
- **Database**: SQLite 3.x (modernc.org/sqlite driver)
- **CLI Framework**: spf13/cobra
- **Configuration**: spf13/viper (TOML format)
- **Terminal Styling**: charmbracelet/lipgloss
- **UUID Generation**: google/uuid (v7 preferred)
- **Migrations**: golang-migrate/migrate

## Essential Commands

### Building
```bash
# Build the project
go build -o dist/wherehouse

# Using mise (recommended for development)
mise run build
mise run snapshot    # builds with version injection
```

### Testing
```bash
# Run all tests
go test ./...

# Using mise (recommended for development)
mise run test
mise run ci        # full CI pipeline (test, lint, build)
mise run dev       # development workflow (generate, mock, lint, test)
```

### Linting
```bash
# Using golangci-lint
golangci-lint run

# Using mise (recommended for development)
mise run lint
```

### Development Tasks
```bash
# Generate artifacts
mise run generate

# Update dependencies
mise run update-deps

# Clean build artifacts
mise run clean
```

## Code Organization & Patterns

### File Layout
- Commands are in `cmd/` directory, organized by feature 
- Core logic is in `internal/` directory
- Database operations in `internal/database/`
- CLI-specific utilities in `internal/cli/`
- Configuration management in `internal/config/`
- Domain models in `internal/models/`
- Event types and handlers in `internal/events/`
- Business rule validations in `internal/validation/`

### Event Sourcing Design
- Events are immutable (never modified or deleted)
- Ordering by `event_id` only (timestamps informational)
- Projections rebuildable from events (`doctor --rebuild`)
- Validation failures stop replay (no silent repair)
- System locations (`Missing`, `Borrowed`, `Loaned`) are special - immutable and predefined

### Error Handling Pattern
- Functions return `error` types for all errors  
- Uses `fmt.Errorf("...: %w", err)` for wrapping errors  
- All database operations use transactions  
- Retry logic implemented for SQL BUSY/LOCKED errors

### Database Operations
- SQLite connection managed via `internal/database/database.go` 
- Uses WAL mode for concurrent access
- Foreign key constraints enabled via PRAGMA
- Migrations handled with `golang-migrate`  
- Database access patterns with `ExecInTransaction` and `WithRetry` helpers
- Projections are derived tables that can be rebuilt from events

## Naming Conventions and Style

- All identifiers use Go naming conventions (PascalCase for public, camelCase for private)
- Canonical names are lowercased, with whitespace and special characters converted to underscores  
- UUIDs are v7 format preferred  
- Function names are descriptive without being verbose  
- Structs with fields are named descriptively (e.g., `Database`, `Config`)  
- Test files end with `_test.go`

## Testing Approach

### Test Types
- Unit tests for functions and methods
- Integration tests for database operations and full command flows
- End-to-end tests of command-line behavior  
- Test coverage targets 80%+ code coverage

### Testing Tools
- Standard Go testing package  
- testify for assertions
- gotestsum for improved test output
- golangci-lint for static analysis and linting  

### Testing Patterns
- Tests organized by package with `package_test.go` and individual function tests  
- Test fixtures for database state when needed  
- Use of `mocks/` directory for interfaces when needed  
- Integration tests use in-memory databases for speed and isolation

## Important Gotchas

1. **Event Sourcing**: Events are always immutable, and database state is rebuildable from the event log. Never modify events or projections directly.

2. **System Location Restrictions**:
   - Items cannot be moved from/to system locations (`Missing`, `Borrowed`, `Loaned`) 
   - Dedicated commands are required for these operations
   - System locations have predefined UUIDs for deterministic behavior

3. **Database Design**:
   - Connection pool maxed to 1 (SQLite limitation)  
   - WAL mode used for concurrent access
   - Foreign key constraints enabled
   - Database operations may retry on BUSY/LOCKED errors

4. **Selector Syntax**:
   - Support for multiple selector types: UUID, LOCATION:ITEM, and canonical name matching
   - Ambiguous selectors fail with ID list display
   - Cannot use colons in item/project names (reserved for selectors)

5. **Configuration Management**:
   - Viper is used for configuration, with native write support instead of manual TOML editing
   - Config file locations follow XDG-compliant patterns
   - Commands should be able to override config at runtime with flags

6. **Error Handling**:
   - All errors must be returned and handled by callers
   - Use `fmt.Errorf("...: %w", err)` for proper error wrapping
   - All database operations use transactions via `ExecInTransaction`

## Architecture Decision Notes

1. **Event Sourcing Framework**: Custom implementation chosen over libraries for simplicity, transparency, and control
2. **CLI Design**: Uses Cobra for strong CLI patterns with consistent flag handling and help output
3. **Database Design**: SQLite chosen for portability, single-file storage, and efficient local storage
4. **Configuration**: Viper chosen for TOML support and robust config handling
5. **Terminal UI**: Planned for future use, with current focus on CLI implementation