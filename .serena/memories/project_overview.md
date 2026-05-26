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
│   ├── add/             # add item / add location
│   ├── config/          # config init/get/set/check/edit/path
│   ├── history/         # event timeline for an item
│   ├── list/            # list items/locations
│   ├── move/            # move items between locations
│   ├── remove/          # remove items
│   ├── rename/          # rename items/locations
│   ├── scry/            # find/search items
│   ├── serve/           # thin shell only — no net/http, html/template, or //go:embed
│   ├── status/          # item status commands
│   └── root.go          # Root command; registers via NewDefaultXxxCmd()
├── internal/
│   ├── app/             # Business logic layer (App struct, EntityResult, HistoryResult, FindResult)
│   ├── cli/             # Shared CLI helpers (selectors, output, flags, user identity)
│   ├── config/          # XDG-compliant TOML config (viper-backed)
│   ├── entitypath/      # Colon-separated path parsing (e.g. "Garage:Toolbox:Drawer")
│   ├── eventbus/        # Event dispatch + projection handlers (Bus, Dispatch, handlers.go)
│   ├── inventory/       # Pure domain types: EntityType, EntityStatus, Entity, Event, EventType iota
│   ├── logging/         # Structured logging + rotation
│   ├── nanoid/          # 10-char alphanumeric NanoID generation
│   ├── store/           # SQLite persistence layer (Store, ExecInTransaction, WithRetry)
│   │   └── migrations/  # SQL schema (golang-migrate)
│   ├── styles/          # lipgloss appStyles singleton
│   ├── version/         # Build version info
│   └── web/             # HTTP server, handlers, templates, embedded assets
│       └── assets/      # //go:embed target (static/, templates/)
├── docs/
│   ├── DESIGN.md
│   └── PROJECT-v0.md
└── main.go
```

## Package Responsibilities (former internal/database/ is now split)
- `internal/inventory/` — pure domain types + EventType iota + `eventTypeByName` map + stringer
- `internal/store/` — raw SQLite I/O: Store struct, migrations, ExecInTransaction, WithRetry
- `internal/eventbus/` — Bus struct, Dispatch, per-event handlers (handlers.go), path propagation
- `internal/app/` — App struct, business logic above the store layer

## EntityResult Field Pitfall
`CanonicalName` = normalized leaf name only (no colons).
`FullPathDisplay` = full colon-separated path (e.g. `"Garage:Toolbox"`).
Use `FullPathDisplay` when checking entity depth or path structure.

## What Does NOT Exist
- `internal/tui/` (TUI is planned, not implemented)
- Tags/tagging (no ItemTaggedEvent, no tags column)
- `internal/database/` package (decomposed — see Package Responsibilities above)
- `ai-docs/` directory
