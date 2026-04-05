# Wherehouse - Project Overview

## Purpose
Event-sourced CLI inventory tracker: "Where did I put my 10mm socket?"
Answers location questions with full audit trail. Alpha stage (v0.1.x).

## Tech Stack
- **Language**: Go 1.25
- **Database**: SQLite via modernc.org/sqlite (pure Go, no CGo), WAL mode
- **CLI**: spf13/cobra + spf13/viper (TOML config)
- **Terminal styling**: charm.land/lipgloss/v2
- **Migrations**: golang-migrate/migrate v4
- **Mocks**: vektra/mockery v3
- **Tests**: stretchr/testify (assert + require)
- **Build/task automation**: mise
- **VCS**: jujutsu (jj) — NOT git

## Architecture: Event Sourcing
- Events are the immutable source of truth (append-only)
- Projections are derived/rebuildable state
- Ordering by `event_id` only (timestamps are informational, not unique)
- No undo — corrections create compensating events
- Replay by `event_id` order ensures determinism

## Repository Layout
```
wherehouse/
├── cmd/                 # CLI commands (cobra); one subdir per command
│   └── root.go          # Root command; registers via NewDefaultXxxCmd()
├── internal/
│   ├── cli/             # Shared CLI helpers (selectors, output, flags)
│   ├── config/          # Configuration management
│   ├── database/        # SQLite: events, projections, migrations, replay
│   │   ├── eventTypes.go        # EventType iota + ParseEventType + stringer
│   │   ├── eventHandler.go      # processEventInTx routing switch
│   │   ├── itemEventHandler.go
│   │   ├── locationEventHandler.go
│   │   ├── projectEventHandler.go
│   │   ├── replay.go            # Event replay engine
│   │   ├── validation.go        # Integrity checks
│   │   └── migrations/          # SQL schema (golang-migrate)
│   ├── logging/         # Logging + log rotation
│   ├── nanoid/          # NanoID generation
│   ├── styles/          # lipgloss appStyles singleton
│   └── version/         # Build version info
├── docs/DESIGN.md
├── ai-docs/
│   ├── knowledge/       # Authoritative AI agent reference docs
│   ├── research/        # Design proposals (may not be implemented)
│   └── sessions/        # Session plans/status
└── main.go
```

## What Does NOT Exist Yet
- `internal/tui/` (TUI is planned, not implemented)
- Tags/tagging (no ItemTaggedEvent, no tags column)
- Project CLI commands (database layer exists but not wired to cmd/)
- `internal/events/` package (event types live in `internal/database/eventTypes.go`)
