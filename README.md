# wherehouse

> Event-sourced CLI inventory tracker: "Where did I put my 10mm socket?"

[![GitHub release (with filter)](https://img.shields.io/github/v/release/asphaltbuffet/wherehouse)](https://github.com/asphaltbuffet/wherehouse/releases)
[![go.mod](https://img.shields.io/github/go-mod/go-version/asphaltbuffet/wherehouse)](go.mod)
[![GitHub License](https://img.shields.io/github/license/asphaltbuffet/wherehouse)](LICENSE)

---

## Quick Example

```bash
# Add entities. --locked pins a place so it can't be reparented;
# --discrete marks a terminal item that holds nothing else.
wherehouse add "Garage" --locked
wherehouse add "Garage:Toolbox"
wherehouse add "Garage:Toolbox:10mm socket" --discrete

# Search by name
wherehouse scry "socket"
# → Garage:Toolbox:10mm socket

# Move something
wherehouse move "Garage:Toolbox" --to "Basement"

# View full event history
wherehouse history "Basement:Toolbox:10mm socket"

# Browse everything in a web UI
wherehouse serve
```

---

## Why Wherehouse?

You know you own a 10mm socket wrench. You used it last week. Where is it now?

Wherehouse tracks every entity's location with a complete audit trail. Event-sourced architecture means you can see where things were, when they moved, and rebuild the entire state from history.

### Key Features

- **Event-Sourced** — append-only event log; projections are derived and rebuildable
- **Unified Entity Model** — locations and items are both entities; hierarchy via colon-separated paths (`Garage:Toolbox:Wrench`)
- **Hierarchical Paths** — place > container > leaf nesting with colon-path addressing
- **Status Tracking** — mark entities as `ok`, `missing`, `borrowed`, `loaned`, or `removed`
- **Full History** — every move, rename, and status change is recorded with actor and timestamp
- **Web UI** — local HTTP server for browsing, searching, adding, and editing inventory
- **Network Storage Ready** — SQLite WAL mode works with NFS/SMB mounts
- **Multi-User Attribution** — trust-based; tracks who made changes, no permissions enforcement
- **Single File Database** — entire inventory in one portable SQLite file

---

## Installation

### Build from Source

**Requirements**: Go 1.25+, SQLite 3.x (embedded via `modernc.org/sqlite`)

```bash
git clone https://github.com/asphaltbuffet/wherehouse.git
cd wherehouse

# Build
go build -o dist/wherehouse .

# Or with mise
mise run build

# Install to user bin
mkdir -p ~/.local/bin
cp dist/wherehouse ~/.local/bin/
```

### Nix

**Standalone install**:

```bash
nix profile install github:asphaltbuffet/wherehouse/v0.6.1
```

**Home Manager** — add as an input and load the bundled module:

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url      = "github:NixOS/nixpkgs/nixpkgs-unstable";
    home-manager.url = "github:nix-community/home-manager";
    wherehouse.url   = "github:asphaltbuffet/wherehouse/v0.6.1";
    wherehouse.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { nixpkgs, home-manager, wherehouse, ... }: {
    homeConfigurations."alice" = home-manager.lib.homeManagerConfiguration {
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
      modules = [
        wherehouse.homeManagerModules.default
        { programs.wherehouse.enable = true; }
      ];
    };
  };
}
```

---

## Quick Start

### 1. Initialize Config

```bash
wherehouse config init
# → Created config at ~/.config/wherehouse/wherehouse.toml
```

The database is created automatically on first use at `~/.local/share/wherehouse/wherehouse.db`.

### 2. Add Entities

Everything is an `Entity` — there is no separate "place", "container", or "item"
type. Entities are distinguished by their position in the hierarchy and two optional
attributes:

| Attribute | Effect |
|---|---|
| `--locked` | Pins the entity so it cannot be directly reparented (use for fixed places like a room or shelf) |
| `--discrete` | Marks a terminal item that holds nothing — adding children to it is blocked (use for a single tracked item) |

A plain `add` with neither flag creates a movable holder (e.g. a box or toolbox).

```bash
# Build a hierarchy
wherehouse add "Garage" --locked
wherehouse add "Garage:Toolbox"                    # nested path creates under Garage

# Add individual items
wherehouse add "Garage:Toolbox:10mm socket" --discrete
wherehouse add "Garage:Toolbox:Ratchet" --discrete
```

Paths are colon-separated from the root. Providing a nested path like `Garage:Toolbox:Wrench` will add `Wrench` under the existing `Garage:Toolbox` entity.

### 3. Search

```bash
# Search by name (substring matching)
wherehouse scry "socket"
# → Garage:Toolbox:10mm socket

# List all entities
wherehouse scry

# JSON output for scripting
wherehouse scry "socket" --json
```

### 4. Move Entities

```bash
# Move a container (and everything in it) to a new parent
wherehouse move "Garage:Toolbox" --to "Basement"

# Move a single item
wherehouse move "Garage:Toolbox:Ratchet" --to "Garage:Pegboard"
```

Locked entities (`--locked`) cannot be directly reparented; everything else is movable.

### 5. View History

```bash
# Full event timeline for an entity (newest first)
wherehouse history "Basement:Toolbox:10mm socket"

# JSON output
wherehouse history "Basement:Toolbox" --json
```

### 6. Track Status

Status is read-only; each transition has its own intent-driven command.

```bash
# Show the current status of an entity (read-only)
wherehouse status "Basement:Toolbox:10mm socket"

# Mark something as missing (only ok entities can be marked missing)
wherehouse lost "Basement:Toolbox:10mm socket"

# Recover a missing entity
wherehouse found "Basement:Toolbox:10mm socket"

# Lend something out (recipient required)
wherehouse loan "Garage:Ladder" --to "Bob" --note "lent for the weekend"

# Bring a loaned (or borrowed) entity back
wherehouse return "Garage:Ladder"

# Track an externally-owned item brought into the inventory
wherehouse borrow "Garage:Alice's Drill" --from "Alice"
```

### 7. List Entities

```bash
# List everything
wherehouse list

# List under a specific path
wherehouse list --under "Garage:Toolbox"

# Filter by status
wherehouse list --status missing
```

### 8. Rename Entities

```bash
wherehouse rename "Garage:Toolbox" --to "Tool Chest"
```

### 9. Remove Entities

```bash
wherehouse remove "Garage:Tool Chest:Broken Wrench"
wherehouse remove "Garage:Old Box" --note "disposed"
```

### 10. Export Event Log

```bash
# Export all events as NDJSON (one JSON object per line)
wherehouse export

# Suppress the "no events" warning when the database is empty
wherehouse export --quiet
```

The `--json` flag is accepted silently (the command always emits NDJSON).

### 11. Check Inventory Health

```bash
# Run all checks (config, event log, projection consistency)
wherehouse doctor

# Rebuild the projection from the event log when checks pass
wherehouse doctor --rebuild

# Rebuild even if issues are found
wherehouse doctor --rebuild --force

# JSON output (healthy flag + issue list)
wherehouse doctor --json
```

Exit code is non-zero when any issue is found, making it safe to use in scripts.

### 12. Web UI

```bash
# Start local web server (default: http://127.0.0.1:8080)
wherehouse serve

# Custom port or bind address
wherehouse serve --port 9090
wherehouse serve --bind 0.0.0.0   # share on LAN
```

Open `http://localhost:8080` in your browser. From the UI you can browse the full entity tree, search, add entities, edit names, and toggle item status.

---

## Commands

```
wherehouse <command> [flags]

Entity Management:
  add <path>           Add an entity (--locked, --discrete, --create-parents)
  move <path>          Move an entity to a new parent (--to <dest>)
  rename <path>        Rename an entity (--to <new-name>)
  remove <path>        Remove an entity from the inventory
  list                 List entities (--under, --status filters)
  scry [<name>]        Search entities by name, or list all
  history <path>       Show full event timeline for an entity

Status:
  status <path>        Show the current status of an entity (read-only)
  lost <path>...       Mark entities as missing (from ok)
  found <path>...      Recover missing entities (to ok)
  loan <path>...       Lend entities out (--to <recipient>)
  return <path>...     Bring loaned/borrowed entities back
  borrow <path>...     Track externally-owned items (--from <lender>)
  export               Export all events as NDJSON to stdout
  doctor               Check inventory health (--rebuild, --rebuild --force)

Web UI:
  serve                Start local web server (--port, --bind)

Configuration:
  config init          Create config file with defaults (--local, --force)
  config check         Validate config file(s)
  config path          Show config file path(s)

Global Flags:
  -h, --help           Show help
  --version            Show version
  --config <path>      Custom config file path
  --no-config          Skip all config files (use defaults only)
  --db <path>          Override database path
  --as <identity>      Override user identity
  --json               Output as JSON
  -q, --quiet          Quiet mode (-q minimal, -qq silent)
```

---

## Configuration

### File Locations (XDG-Compliant)

**Config file** (in priority order):
1. `--config <path>` flag
2. `$WHEREHOUSE_CONFIG` environment variable
3. `./wherehouse.toml` (current directory)
4. `~/.config/wherehouse/wherehouse.toml` (default)

**Data**: `~/.local/share/wherehouse/wherehouse.db`

### Config File

```bash
wherehouse config init
```

Generated `~/.config/wherehouse/wherehouse.toml`:

```toml
[database]
path = "~/.local/share/wherehouse/wherehouse.db"

[logging]
level = "warn"
# file_path = "~/.local/state/wherehouse/wherehouse.log"
# max_size_mb = 10
# max_backups = 3

[user]
default_identity = ""
os_username_map = {}

[output]
default_format = "human"
quiet = false
```

### Environment Variables

```bash
export WHEREHOUSE_DATABASE_PATH="/mnt/nas/wherehouse.db"
export WHEREHOUSE_CONFIG="$HOME/projects/workshop/wherehouse.toml"
export WHEREHOUSE_LOG_PATH="/var/log/wherehouse/wherehouse.log"
export WHEREHOUSE_OUTPUT_DEFAULT_FORMAT="json"
```

### Home Manager

```nix
programs.wherehouse = {
  enable = true;

  settings = {
    database.path = "~/.local/share/wherehouse/wherehouse.db";

    user = {
      defaultIdentity = "";
      osUsernameMap = { jdoe = "John Doe"; };
    };

    logging = {
      level = "warn";
    };

    output = {
      defaultFormat = "human";
      quiet = false;
    };
  };
};
```

| Option | Type | Default | Description |
|---|---|---|---|
| `settings.database.path` | string | XDG data dir | Path to SQLite database file |
| `settings.logging.filePath` | string | XDG state dir | Path to log file |
| `settings.logging.level` | `"debug"`…`"error"` | `"warn"` | Minimum log level |
| `settings.logging.maxSizeMB` | int | `0` (disabled) | Max log size before rotation |
| `settings.logging.maxBackups` | int | `3` | Old rotated files to keep |
| `settings.user.defaultIdentity` | string | OS username | Display name for attribution |
| `settings.user.osUsernameMap` | attrset | `{}` | Map OS usernames to display names |
| `settings.output.defaultFormat` | `"human"` \| `"json"` | `"human"` | Default output format |
| `settings.output.quiet` | bool | `false` | Suppress non-essential output |

---

## Architecture

### Event Sourcing

- **Events** are the source of truth (append-only log, never modified)
- **Projections** are derived state (rebuildable from events)
- **Replay** ordered strictly by `event_id` — timestamps are informational only
- **No undo** — corrections create new compensating events

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

**Projection tables** (derived, rebuildable):
- `locations_current` — current entity hierarchy
- `items_current` — current entity state
- `projects_current` — active and completed projects

---

## Development

### Prerequisites

- Go 1.25+
- [mise](https://mise.jdx.dev/) (recommended for task automation)

### Build & Test

```bash
# Build
mise run build        # → dist/wherehouse

# Test (race detector + coverage)
mise run test

# Lint
mise run lint

# Full pipeline
mise run dev        # binary lands in dist/<os>_<arch>_<variant>/wherehouse
```

---

### Releasing

The changelog (`.changes/`) is the single source of truth for versions. Releases are triggered by pushing a version tag to GitHub — there is no `VERSION` file.

**To cut a release:**

1. As you work, use `changie new` to record user-facing changes in `.changes/unreleased/`. This is optional for non-user-facing changes (tooling, CI, docs) — but at least one fragment must exist before a release.
2. Run `mise run pre-release <major|minor|patch>` — this:
   - Hard-fails if no unreleased fragments exist
   - Runs `changie batch <bump>` to create `.changes/<version>.md`
   - Runs `changie merge` to update `CHANGELOG.md`
   - Updates nix flake pins in this file
3. Commit `.changes/`, `CHANGELOG.md`, and `README.md` together and open a PR
4. After the PR merges, run `mise run release` — this:
   - Asserts the working copy is clean and `@` is on trunk
   - Asserts the tag doesn't already exist
   - Asserts `CHANGELOG.md` has an entry for the version
   - Creates and pushes the `v<version>` tag

When the tag push lands, CI:
- Extracts the version from the tag name
- Asserts `CHANGELOG.md` has a matching entry
- Runs goreleaser with `.changes/<version>.md` as the release notes

**Non-user-facing changes** (dependency bumps, CI fixes, tooling) should not trigger a release. Merge them to main without running `pre-release` — they will be bundled into the next release that does have user-facing changes.

---

## Troubleshooting

### `SQLITE_BUSY` / lock errors

Multiple processes accessing the same database, or a network mount with locking issues.

```bash
ps aux | grep wherehouse
```

For network storage, verify NFSv4/SMB file locking support.

### Performance degradation

```bash
# Check database size
ls -lh ~/.local/share/wherehouse/wherehouse.db
```

