# wherehouse

> Event-sourced CLI inventory tracker: "Where did I put my 10mm socket?"

[![GitHub release (with filter)](https://img.shields.io/github/v/release/asphaltbuffet/wherehouse)](https://github.com/asphaltbuffet/wherehouse/releases)
[![go.mod](https://img.shields.io/github/go-mod/go-version/asphaltbuffet/wherehouse)](go.mod)
[![GitHub License](https://img.shields.io/github/license/asphaltbuffet/wherehouse)](LICENSE)

---

## Quick Example

```bash
# Initialize your inventory database
wherehouse initialize database

# Create location hierarchy
wherehouse add location Garage
wherehouse add location Toolbox --in Garage
wherehouse add location "Socket Set" --in Toolbox

# Add items to locations
wherehouse add item "10mm socket wrench" --in "Socket Set"
wherehouse add item "step ladder" --in Garage

# Find anything instantly
wherehouse find "socket"
# → 10mm socket wrench
#   Location: Garage >> Toolbox >> Socket Set

# Move items with context (coming soon)
wherehouse move "ladder" Kitchen --project "change-lightbulb" --temporary

# Track project items (coming soon)
wherehouse find --project "change-lightbulb"
# → step ladder (temporary use, origin: Garage)

# Track project items (coming soon)
wherehouse find --project "change-lightbulb"
# → step ladder (temporary use, origin: Garage)

# Full history and audit trail
wherehouse history "ladder"
# → ○  2 hours ago (alice)  item.moved
#   │  Moved: Garage → Kitchen
#   │  Type: temporary_use
#   │  Project: change-lightbulb
#   │
#   ○  2026-02-15 10:30 (alice)  item.created
#      Created at: Garage

# Mark items as missing or borrowed (coming soon)
wherehouse missing "socket"  # lost it
wherehouse found "socket" Basement --home Garage  # found it!
```

---

## Why Wherehouse?

**The Problem**: You know you own a 10mm socket wrench. You used it last week. Where did you put it?

**The Solution**: Wherehouse tracks every item's location with a complete audit trail. Event-sourced architecture means you can see where items were, when they moved, and rebuild the entire state from history.

### Key Features

- ✅ **Event-Sourced Architecture** - Complete audit trail, rebuild state from history
- 🚀 **Fast Lookups** - SQLite-backed projections for instant queries
- 🌳 **Hierarchical Locations** - Nested organization (Garage > Toolbox > Drawer 3)
- 🏷️ **Project Tracking** - Associate items with temporary projects
- 🔍 **Flexible Search** - By name, location, project, or status
- 📊 **Full History** - See every movement, note, and change
- 🔧 **Self-Healing** *(planned)* - `doctor` command validates and repairs database
- 🌐 **Network Storage Ready** - Works with NFS, SMB mounts (SQLite WAL mode)
- 📱 **Multi-User Attribution** - Track who moved what (trust-based, no permissions)
- 💾 **Single File Database** - Entire inventory in one portable SQLite file
- 🎨 **Terminal UI** *(planned)* - Interactive TUI for visual browsing

### Design Philosophy

- **Explicit over implicit** - No silent magic, you control everything
- **Deterministic over convenient** - Event ordering by ID, not timestamps
- **Transparent over abstracted** - Direct SQL, no ORM hiding behavior
- **Audit trail over performance** - Every change recorded forever

---

## Installation

### Build from Source (Current Method)

**Requirements**:
- Go 1.25 or higher
- SQLite 3.x (embedded via modernc.org/sqlite)
- mise (optional, for dev task automation)
- goreleaser (optional, for full build automation)

```bash
# Clone repository
git clone https://github.com/asphaltbuffet/wherehouse.git
cd wherehouse

# Build
mise run snapshot
# or
mise run build

# Install to user bin
mkdir -p ~/.local/bin
cp dist/wherehouse ~/.local/bin/
export PATH="$PATH:$HOME/.local/bin"  # add to ~/.bashrc or ~/.zshrc

# Verify installation
wherehouse --version
```

### Nix

**Standalone install** (no flake required):

```bash
nix profile install github:asphaltbuffet/wherehouse
```

**In a home-manager flake** — add as an input and load the bundled module:

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url      = "github:NixOS/nixpkgs/nixpkgs-unstable";
    home-manager.url = "github:nix-community/home-manager";
    wherehouse.url   = "github:asphaltbuffet/wherehouse";
    wherehouse.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { nixpkgs, home-manager, wherehouse, ... }: {
    homeConfigurations."alice" = home-manager.lib.homeManagerConfiguration {
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
      modules = [
        wherehouse.homeManagerModules.default
        {
          programs.wherehouse.enable = true;
          # see Configuration → Home Manager for all settings
        }
      ];
    };
  };
}
```

**As a NixOS overlay** (adds `pkgs.wherehouse`):

```nix
nixpkgs.overlays = [ wherehouse.overlays.default ];
```

### Using mise (Development)

```bash
# Install dependencies and build
mise install
mise run build

# Run tests
mise run test

# Run linter
mise run lint

# Full dev pipeline
mise run dev
```

## Other Binaries
- **Pre-compiled binaries**: GitHub releases page

---

## Quick Start

### 1. Initialize Database

```bash
wherehouse initialize database
# → Database initialized: ~/.local/share/wherehouse/wherehouse.db

# If the database already exists, use --force to reinitialize (backs up first)
wherehouse initialize database --force
# → Backup created: ~/.local/share/wherehouse/wherehouse.db.backup.20260226
# → Database initialized: ~/.local/share/wherehouse/wherehouse.db
```

### 2. Initialize Config

```bash
wherehouse config init
# → Created config at ~/.config/wherehouse/wherehouse.toml
```

### 3. Create Location Hierarchy

```bash
# Create top-level locations
wherehouse add location Garage
wherehouse add location Basement
wherehouse add location Kitchen

# Create nested locations
wherehouse add location Toolbox --in Garage
wherehouse add location "Socket Set" --in Toolbox
```

### 4. Add Items

```bash
# Add item to specific location
wherehouse add item "10mm socket wrench" --in "Socket Set"

# Add multiple items at once
wherehouse add item "step ladder" "work bench" "tool cart" --in Garage

# Items with special characters work fine
wherehouse add item "3/8\" drive ratchet" --in Toolbox
```

### 5. Find Items

```bash
# Search by name (substring matching)
wherehouse find "socket"
# → 10mm socket wrench
#   Location: Garage >> Toolbox >> Socket Set
#
# → Socket set organizer
#   Location: Garage >> Toolbox

# Limit results
wherehouse find "screw" -n 5
# Shows only the 5 closest matches (by Levenshtein distance)

# Verbose output with match details
wherehouse find "ladder" -v
# → step ladder
#   Location: Garage
#   ID: 01HXXX...
#   Match distance: 0 (exact match)

# JSON output for scripting
wherehouse find "socket" --json
# → {"search_term":"socket","results":[...],"total_count":2,...}

# Missing items show last known location
wherehouse find "wrench"
# → 10mm wrench (MISSING)
#   Last location: Garage >> Toolbox
#   Currently: Missing
```

### 6. View Item History

```bash
# Show complete event timeline (newest first)
wherehouse history "socket"
# → ○  2 hours ago (alice)  item.moved
#   │  Moved: Garage >> Toolbox → Kitchen >> Counter
#   │  Type: temporary_use
#   │
#   ○  2026-02-20 14:30 (bob)  item.created
#      Created at: Garage >> Toolbox

# Limit to recent events
wherehouse history "ladder" -n 5

# Show events since a date
wherehouse history "socket" --since "2026-02-01"

# Natural language dates
wherehouse history "wrench" --since "2 weeks ago"
wherehouse history "socket" --since yesterday

# Chronological order (oldest first)
wherehouse history "ladder" --oldest-first

# JSON output for scripting
wherehouse history "socket" --json
# → {"events":[...],"count":12}

# Search by UUID instead of name
wherehouse history --id "01HXXX-XXXX-..."
```

### 7. Move Items

```bash
# Permanent move (rehome)
wherehouse move "socket" Basement:Toolbox

# Temporary use (tracks origin)
wherehouse move "ladder" Kitchen --temporary

# Move with project association
wherehouse move "paint roller" "Bedroom" --project "bedroom-repaint"

# Keep project when moving
wherehouse move "roller" Basement --keep-project
```

### 8. Track Missing Items

```bash
# Mark as missing
wherehouse missing "socket"
# → Moved to Missing (last known: Garage >> Toolbox)

# Search missing items
wherehouse find --location Missing

# Mark as found
wherehouse found "socket" Basement --home "Garage:Toolbox"
# → Found in Basement, set temporary use (origin: Garage >> Toolbox)

# Return to origin
wherehouse move "socket" "Garage:Toolbox" --rehome
```

---

## Usage

### Commands Overview

```bash
wherehouse [command] [flags]

Item Management:
  add item ✅           Create new item in inventory
  move ✅              Move item to different location
  where ✅             Find item(s) or locations by name
  history ✅           Show complete event timeline for item
  missing              Mark item as lost (coming soon)
  found                Mark missing item as found (coming soon)
  delete               Permanently remove item (coming soon)

Location Management:
  add location ✅      Create new location
  location list        Show all locations (coming soon)
  location tree        Display hierarchy as tree (coming soon)
  location move        Reparent location in tree (coming soon)
  location delete      Remove empty location (coming soon)

Project Management:
  project create       Start new project (coming soon)
  project complete     Mark project as finished (coming soon)
  project list         Show all projects (coming soon)
  project delete       Remove project (if no items) (coming soon)

Database Operations:
  initialize database ✅  Initialize new database (--force to overwrite with backup)
  doctor               Validate database consistency (partial)
  export               Export events and projections (coming soon)
  import               Import from export file (coming soon)

Configuration:
  config init ✅       Create config file with defaults
  config get ✅        Show configuration values
  config set ✅        Set a configuration value
  config check ✅      Validate config file
  config edit ✅       Open config file in $EDITOR
  config path ✅       Show config file path(s)

Global Flags:
  -h, --help           Show help ✅
  --version            Show version ✅
  --config <path>      Custom config file ✅
  --db <path>          Override database path ✅
  --as <identity>      Override user identity ✅
  --json               Output as JSON ✅
  -q, --quiet          Suppress non-error output ✅
  -i, --in             Specify location (for add commands) ✅
```

### Common Workflows

#### Starting a Project

```bash
# Create project
wherehouse project create deck-rebuild

# Gather tools
wherehouse move "circular saw" Backyard --project deck-rebuild --temporary
wherehouse move "drill" Backyard --project deck-rebuild --temporary
wherehouse move "level" Backyard --project deck-rebuild --temporary

# Check project inventory
wherehouse find --project deck-rebuild

# Complete project (shows items to return)
wherehouse project complete deck-rebuild
# → Project completed. Items to return:
#   - circular saw → Garage >> Toolbox
#   - drill → Garage >> Toolbox
#   - level → Garage >> ToolWall
```

#### Borrowing Items

```bash
# Mark as borrowed
wherehouse borrow "ladder" --to "Bob" --note "for his garage project"

# See all borrowed items
wherehouse find --location Borrowed

# Return borrowed item
wherehouse move "ladder" Garage --rehome
```

#### Debugging Inconsistencies

```bash
# Validate database
wherehouse doctor
# → Checking event log integrity... ✓
# → Checking location tree... ✓
# → Checking projection consistency... ✓
# → Database is healthy

# Rebuild projections from events
wherehouse doctor --rebuild
# → Rebuilding projections from 1,247 events...
# → locations_current: 47 rows
# → items_current: 289 rows
# → projects_current: 12 rows
# → Rebuild complete

# Export for backup
wherehouse export > backup-$(date +%Y%m%d).json
```

---

## Configuration

### File Locations (XDG-Compliant)

**Config file** (in priority order):
1. `--config <path>` flag
2. `$WHEREHOUSE_CONFIG` environment variable
3. `./wherehouse.toml` (current directory)
4. `~/.config/wherehouse/wherehouse.toml` (default)

**Data locations**:
- Database: `~/.local/share/wherehouse/wherehouse.db`

### Configuration File

Create the default config with:

```bash
wherehouse config init
```

The generated file contains all keys with their default values (no comments). Use
`config edit` to open the file in `$EDITOR` if you want to annotate it.

Individual values can be set via `config set <key> <value>`. All keys are supported:

```bash
wherehouse config set database.path /mnt/nas/wherehouse.db
wherehouse config set logging.level debug
wherehouse config set logging.max_size_mb 10
wherehouse config set logging.max_backups 3
wherehouse config set output.default_format json
wherehouse config set output.quiet true
wherehouse config set user.default_identity alice
```

> **Note**: `user.os_username_map` is a map type — edit the config file directly to set it.

**Full annotated example** (`~/.config/wherehouse/wherehouse.toml`):

```toml
[database]
# Path to SQLite database file. Supports ~ and $ENV_VARS.
# Default: $XDG_DATA_HOME/wherehouse/wherehouse.db
path = "~/.local/share/wherehouse/wherehouse.db"
# Or network storage:
# path = "/mnt/nas/shared/wherehouse.db"

[logging]
# Path to log file. Supports ~ and $ENV_VARS.
# Default: $XDG_STATE_HOME/wherehouse/wherehouse.log
# file_path = "~/.local/state/wherehouse/wherehouse.log"

# Minimum log level: "debug", "info", "warn", "error". Default: "warn".
level = "warn"

# Max log file size (MB) before rotation. 0 = no rotation (default).
# max_size_mb = 10

# Number of old log files to keep when rotation is enabled. Default: 3.
# max_backups = 3

[user]
# Display name for event attribution. Empty = OS username.
default_identity = ""

# Map OS usernames to display names.
# os_username_map = { "jdoe" = "John Doe" }
os_username_map = {}

[output]
# Default output format: "human" or "json"
default_format = "human"

# Enable quiet mode by default
quiet = false
```

### Home Manager

The flake ships a home-manager module at `homeManagerModules.default`. Enable it and
configure `programs.wherehouse.settings` to generate `~/.config/wherehouse/wherehouse.toml`
automatically — equivalent to running `wherehouse config init` and editing the result.

```nix
programs.wherehouse = {
  enable = true;   # installs the package and enables the module

  settings = {
    database.path = "~/.local/share/wherehouse/wherehouse.db";

    user = {
      # Empty string means use the OS username.
      defaultIdentity = "";

      # Map OS usernames to display names.
      osUsernameMap = {
        jdoe   = "John Doe";
        asmith = "Alice Smith";
      };
    };

    logging = {
      # filePath defaults to $XDG_STATE_HOME/wherehouse/wherehouse.log
      # filePath = "~/.local/state/wherehouse/wherehouse.log";
      level      = "warn";  # "debug", "info", "warn", "error"
      # maxSizeMB  = 10;    # enable log rotation at 10 MB
      # maxBackups = 3;     # keep 3 rotated files
    };

    output = {
      defaultFormat = "human";  # "human" or "json"
      quiet         = false;
    };
  };
};
```

All `settings` fields are optional — omitting a field leaves the application default in
effect. The config file is only written when at least one field is set.

| Option | Type | Default | Description |
|---|---|---|---|
| `settings.database.path` | string | XDG data dir | Path to SQLite database file |
| `settings.logging.filePath` | string | XDG state dir | Path to log file (file-only, never screen) |
| `settings.logging.level` | `"debug"`…`"error"` | `"warn"` | Minimum log level |
| `settings.logging.maxSizeMB` | int | `0` (disabled) | Max log size before rotation |
| `settings.logging.maxBackups` | int | `3` (when rotating) | Old rotated files to keep |
| `settings.user.defaultIdentity` | string | OS username | Display name for attribution |
| `settings.user.osUsernameMap` | attrset | `{}` | Map OS usernames to display names |
| `settings.output.defaultFormat` | `"human"` \| `"json"` | `"human"` | Default output format |
| `settings.output.quiet` | bool | `false` | Suppress non-essential output |

### Environment Variables

```bash
# Override database path
export WHEREHOUSE_DATABASE_PATH="/mnt/nas/wherehouse.db"

# Override config location
export WHEREHOUSE_CONFIG="$HOME/projects/workshop/wherehouse.toml"

# Override log file path
export WHEREHOUSE_LOG_PATH="/var/log/wherehouse/wherehouse.log"

# Override output format
export WHEREHOUSE_OUTPUT_DEFAULT_FORMAT="json"
```

---

## Integration & Scripting

### Shell Completion

**Bash** (`~/.bashrc`):
```bash
eval "$(wherehouse completion bash)"
# Or install system-wide:
# wherehouse completion bash | sudo tee /etc/bash_completion.d/wherehouse
```

**Zsh** (`~/.zshrc`):
```bash
eval "$(wherehouse completion zsh)"
# Or for fpath completion:
# wherehouse completion zsh > ~/.zsh/completions/_wherehouse
```

**Fish** (`~/.config/fish/config.fish`):
```fish
wherehouse completion fish | source
# Or install permanently:
# wherehouse completion fish > ~/.config/fish/completions/wherehouse.fish
```

### JSON Output for Scripting

All commands support `--json` for machine-readable output:

```bash
# Find all items in a project
wherehouse find --project deck-rebuild --json | jq -r '.[] | .display_name'

# Export locations as JSON
wherehouse location list --json | jq '.[] | select(.parent_id == null)'

# Check for missing items
MISSING_COUNT=$(wherehouse find --location Missing --json | jq 'length')
if [ "$MISSING_COUNT" -gt 0 ]; then
  echo "Warning: $MISSING_COUNT items are missing!"
fi
```

### Exit Codes

```bash
0   Success
1   General error
2   Command-line usage error
3   Database error
4   Item/location/project not found
5   Validation error (constraint violation)
6   Integrity error (event replay failure)
```

### Example Scripts

**Find all tools in garage:**
```bash
#!/bin/bash
wherehouse find --location Garage --json \
  | jq -r '.[] | "\(.display_name) → \(.location)"'
```

**Backup database with rotation:**
```bash
#!/bin/bash
BACKUP_DIR="$HOME/backups/wherehouse"
mkdir -p "$BACKUP_DIR"

# Export as JSON
wherehouse export > "$BACKUP_DIR/wherehouse-$(date +%Y%m%d-%H%M%S).json"

# Keep only last 10 backups
ls -t "$BACKUP_DIR"/*.json | tail -n +11 | xargs rm -f
```

**Daily missing items report:**
```bash
#!/bin/bash
# Add to crontab: 0 9 * * * /usr/local/bin/check-missing.sh

MISSING=$(wherehouse find --location Missing --json)
COUNT=$(echo "$MISSING" | jq 'length')

if [ "$COUNT" -gt 0 ]; then
  echo "Missing Items Report - $(date)"
  echo "$MISSING" | jq -r '.[] | "- \(.display_name) (last seen: \(.location))"'
fi
```

---

## Architecture

### Event Sourcing

Wherehouse uses **event sourcing** as its core architecture:

- **Events** are the source of truth (append-only log)
- **Projections** are derived state (rebuildable from events)
- **Replay** by `event_id` order ensures determinism
- **No undo** - corrections create new compensating events

**Example event log:**
```
event_id | event_type        | item_id  | payload
---------|-------------------|----------|----------------------------------
1        | item.created      | abc-123  | {"name":"socket","location":"..."}
2        | item.moved        | abc-123  | {"from":"garage","to":"kitchen"}
3        | item.marked_missing| abc-123 | {"prev_location":"kitchen"}
4        | item.marked_found | abc-123  | {"found":"basement","home":"garage"}
```

**Benefits:**
- Complete audit trail (who, what, when, why)
- Rebuild database state from scratch
- Time-travel queries (future feature)
- Debugging via event replay

### Database Schema

**Events table** (source of truth):
```sql
CREATE TABLE events (
  event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type       TEXT NOT NULL,
  timestamp_utc    TEXT NOT NULL,
  actor_user_id    TEXT NOT NULL,
  payload          TEXT NOT NULL,  -- JSON
  note             TEXT
);
```

**Projection tables** (derived state):
- `locations_current` - Current location hierarchy
- `items_current` - Current item state and associations
- `projects_current` - Active and completed projects

**Invariants:**
- Events are immutable (never modified or deleted)
- Ordering by `event_id` only (timestamps informational)
- Projections rebuildable from events (`doctor --rebuild`)
- Validation failures stop replay (no silent repair)

### Technology Stack

- **Language**: Go 1.21+
- **Database**: SQLite 3.x (modernc.org/sqlite driver)
- **CLI Framework**: spf13/cobra
- **Configuration**: spf13/viper (TOML format)
- **Terminal Styling**: charmbracelet/lipgloss
- **UUID Generation**: google/uuid (v7 preferred)
- **Migrations**: golang-migrate/migrate

---

## Development

### Prerequisites

- **Go**: 1.21 or higher
- **SQLite**: 3.x (embedded, no separate install)
- **make**: GNU Make (optional, for convenience)
- **mise**: Task automation (optional, recommended)

### Building from Source

```bash
# Clone repository
git clone https://github.com/asphaltbuffet/wherehouse.git
cd wherehouse

# Install dependencies
go mod download

# Build
go build -o dist/wherehouse

# Run
./dist/wherehouse --help
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/database/...

# Integration tests (with mise)
mise run test

# Generate coverage report
mise run test
# → Coverage: tmp/coverage.html
```

### Linting

```bash
# Using golangci-lint
golangci-lint run

# Via mise
mise run lint
# → Report: tmp/lint-report.html
```

### Project Structure

```
wherehouse/
├── cmd/                    # CLI commands (cobra)
│   ├── root.go            # Root command
│   ├── add.go             # Add item command
│   ├── where.go           # Find item command
│   └── ...
├── internal/
│   ├── config/            # Configuration management
│   ├── database/          # ✅ SQLite operations (COMPLETE)
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
│   └── tui/               # Terminal UI (planned)
├── docs/
│   ├── DESIGN.md          # Full design specification
│   └── ...
├── .claude/
│   └── knowledge/         # AI agent context
├── ai-docs/
│   └── sessions/          # Development session logs
├── dist/                  # Build artifacts (gitignored)
├── go.mod
├── main.go
├── Makefile
└── README.md
```

### Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Quick start:**
1. Fork the repository
2. Create feature branch: `git checkout -b feat/my-feature`
3. Make changes and add tests
4. Run `mise run ci` (or `make test lint`)
5. Commit using conventional commits: `feat: add export command`
6. Submit pull request

**Development workflow:**
```bash
# Install pre-commit hooks
mise run install-hooks

# Make changes, run tests continuously
mise watch test

# Full CI check before commit
mise run ci
```

---

## Roadmap

### ✅ Completed (v0.1.0 - Alpha)

- [x] Event-sourced database architecture
- [x] SQLite backend with WAL mode
- [x] Migration framework (golang-migrate)
- [x] Event storage and replay
- [x] Projection CRUD operations
- [x] Validation and integrity checking
- [x] XDG-compliant configuration
- [x] 100+ integration tests
- [x] CLI command implementations (partial)
  - [x] `add item` - Add items to locations
  - [x] `add location` - Create location hierarchy
  - [x] `where` (aliased as `find`) - Find items/locations with intelligent ranking
  - [x] `history` - Show complete event timeline for items
  - [x] `move` - Move items between locations (selectors, project tracking, temporary moves)
  - [x] `config` subcommands (init, get, set, check, edit, path) - viper-backed configuration
  - [x] `initialize database` - create SQLite database with `--force` backup/overwrite
  - [x] Basic output formatting (human-readable, JSON, quiet modes)
  - [x] Flag overrides: `--db`, `--as` override config file values at runtime
  - [x] Clear error when database file is missing (guides user to `initialize database`)

### 🚧 In Progress (v0.2.0 - Alpha)

- [ ] CLI command implementations (continued)
  - [ ] `project` subcommands
  - [ ] `doctor`, `export`, `import`
  - [ ] `missing`, `found`, `borrow`
- [ ] Shell completions (bash, zsh, fish)
- [ ] Man page generation

### 📋 Planned (v0.3.0 - Beta)

- [ ] Terminal UI (TUI) for interactive browsing
- [ ] Full-text search across items and notes
- [ ] Export/import with multiple formats (JSON, CSV)
- [ ] Performance optimizations
- [ ] Pre-compiled binaries for releases

### 🔮 Future (v1.0.0+)

- [ ] Plugin system for custom event types
- [ ] Read-only web dashboard
- [ ] Multi-database sync (experimental)
- [ ] Advanced queries (time-travel, analytics)
- [ ] Package manager distribution (AUR, PPA, Homebrew)

### ❌ Not Planned

- Cloud hosting service
- Mobile applications
- Real-time collaboration
- Enterprise features (SSO, RBAC, permissions)
- Distributed/multi-site deployment

---

## Troubleshooting

### `wherehouse: command not found`

**Cause**: Binary not in PATH

**Solution**:
```bash
# Find wherehouse location
which wherehouse

# If missing, add ~/.local/bin to PATH (in ~/.bashrc or ~/.zshrc)
export PATH="$PATH:$HOME/.local/bin"

# Or install system-wide
sudo cp dist/wherehouse /usr/local/bin/
```

### Database initialization fails

**Cause**: Permissions or existing corrupted database

**Solution**:
```bash
# Check directory permissions
ls -ld ~/.local/share/wherehouse/

# Create directory if missing
mkdir -p ~/.local/share/wherehouse

# Reinitialize database (backs up existing file automatically)
wherehouse initialize database --force
```

### `SQLITE_BUSY` or lock errors

**Cause**: Multiple processes accessing database or network mount locking issues

**Solution**:
```bash
# Check for other wherehouse processes
ps aux | grep wherehouse

# Increase busy_timeout in config.toml
[sqlite]
busy_timeout = 60000  # 60 seconds

# For network storage, verify locking support
# NFS: Use NFSv4 with proper lock daemon
# SMB: Ensure file locking is enabled
```

### `wherehouse doctor` reports corruption

**Cause**: Projection state doesn't match event log (concurrent writes, crash, bug)

**Solution**:
```bash
# Backup first
wherehouse export > backup-$(date +%Y%m%d).json

# Rebuild projections from events
wherehouse doctor --rebuild
# → This will delete and recreate projection tables from event log

# Verify
wherehouse doctor
```

### Performance degradation over time

**Cause**: Database fragmentation or missing indexes

**Solution**:
```bash
# Optimize database (future command)
wherehouse maintenance --vacuum --analyze

# Check database size
ls -lh ~/.local/share/wherehouse/inventory.db

# Export and reimport for defragmentation
wherehouse export > backup.json
wherehouse initialize database --force
wherehouse import < backup.json
```

---

## FAQ

**Q: Can wherehouse work on network storage (NAS, SMB, NFS)?**
A: Yes! SQLite's WAL mode supports network filesystems. Configure in `config.toml`:
```toml
db_path = "/mnt/nas/inventory.db"
[sqlite]
journal_mode = "WAL"
busy_timeout = 30000  # 30s for network latency
```

**Q: How do I back up my inventory?**
A: Three options:
1. Export (recommended): `wherehouse export > backup.json` (events + projections)
2. Copy database: `cp ~/.local/share/wherehouse/inventory.db backup.db`
3. Version control: `git add inventory.db && git commit` (if small enough)

**Q: Can multiple people use the same database?**
A: Yes for attribution, no for permissions. Wherehouse tracks *who* made changes but doesn't enforce *who can* make changes. It's trust-based, designed for households or small teams.

**Q: What happens if I delete an item by mistake?**
A: Deletion is permanent. The event log preserves history, but the item won't appear in queries. Best practice: use "Missing" location instead of deleting.

**Q: How big can my inventory get?**
A: Tested with 100,000 items and 500,000 events. Practical limit for good UX: 10,000-50,000 items. SQLite can handle millions, but query performance degrades.

**Q: Why event sourcing for a simple inventory tracker?**
A: Audit trail and debugging. "Where did I last see my socket?" becomes a query with timeline output. Projections can be rebuilt if corrupted. History survives accidental changes.

---

## License

MIT License - see [LICENSE](LICENSE) for details.

**TL;DR**: Free to use, modify, and distribute. No warranty provided.

---

## Credits

**Created by**: [Ben Lechlitner](https://github.com/asphaltbuffet)

**Built with**:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [SQLite](https://sqlite.org/) - Embedded database
- [golang-migrate](https://github.com/golang-migrate/migrate) - Schema migrations

**Special thanks**:
- Event sourcing patterns inspired by [Martin Fowler's work](https://martinfowler.com/eaaDev/EventSourcing.html)
- CLI design guidance from [clig.dev](https://clig.dev/)

---

## Support

- 📖 **Documentation**: [docs/](docs/)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/asphaltbuffet/wherehouse/discussions)
- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/asphaltbuffet/wherehouse/issues)
- 📧 **Email**: wherehouse@example.com
- 💡 **Feature Requests**: [GitHub Discussions - Ideas](https://github.com/asphaltbuffet/wherehouse/discussions/categories/ideas)

---

**Status**: Alpha development - database foundation complete, CLI in progress

**Next milestone**: v0.2.0 with full CLI implementation (ETA: Q2 2026)

**Star the project** if you find it useful! ⭐
