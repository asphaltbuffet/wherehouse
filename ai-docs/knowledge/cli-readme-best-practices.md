# CLI Application README Best Practices

**Purpose**: Best practices for writing effective README documentation for command-line tools
**Primary OS**: Linux (with cross-platform considerations)
**Date**: 2026-02-21

---

## Executive Summary

A CLI application README must serve both **human users** (who need to understand and use the tool quickly) and **machine readers** (package managers, GitHub UI). For Linux-focused CLI tools, the README should prioritize:

1. **Quick Start** - Get users running commands within 30 seconds
2. **Installation Clarity** - Multiple methods for diverse Linux distributions
3. **Practical Examples** - Show don't tell (GIFs, Asciinema recordings)
4. **Progressive Disclosure** - Brief overview → common tasks → full reference
5. **Integration Paths** - How the tool fits into existing workflows

**Golden Rule**: Lead with examples, support with reference documentation.

---

## Essential Sections (Priority Order)

### 1. Project Header

**Purpose**: Instant recognition and value proposition

**Contents**:
```markdown
# wherehouse

> Event-sourced CLI inventory tracker: "Where did I put my 10mm socket?"

[Build Status] [Version] [License] [Downloads]
```

**Best Practices**:
- **Logo/Icon** (optional) - Visual identity, especially for popular tools
- **One-line description** - What it does, who it's for (< 80 chars)
- **Badges** - Build status, coverage, version, license (top-right or below title)
- **Screenshot/Demo first** - For TUI apps, show the interface immediately

**Examples from the Wild**:
- **ripgrep**: "recursively searches the current directory for a regex pattern"
- **fd**: "A simple, fast and user-friendly alternative to 'find'"
- **gh**: "GitHub on the command line"

**Anti-Pattern**: Vague descriptions like "A productivity tool for developers"

---

### 2. Demo/Quick Example

**Purpose**: Prove value in 10 seconds

**Contents**:
```markdown
## Quick Example

```bash
# Find where you stored that socket wrench
wherehouse where "10mm socket"
# → Garage >> Toolbox >> Socket Set

# Move it to your current project
wherehouse move "10mm socket" Kitchen --project fixing-sink
```

Or use Asciinema/GIF:
```markdown
![Demo](docs/demo.gif)
```

**Best Practices**:
- **Real-world use case** - Not "hello world", but actual problem-solving
- **Output included** - Show expected results, not just input
- **Progressive complexity** - Simple example → slightly more advanced
- **Visual demos** (highly recommended):
  - **Asciinema** - Terminal recordings (text-based, searchable)
  - **GIFs** - Animated screenshots (universally supported)
  - **Screenshot** - Static output for TUI apps

**Tools for Demos**:
- [Asciinema](https://asciinema.org/) - Terminal session recorder
- [VHS](https://github.com/charmbracelet/vhs) - Generate terminal GIFs from scripts
- [termtosvg](https://github.com/nbedos/termtosvg) - Terminal to SVG recorder

**Anti-Pattern**: Long text descriptions without showing actual usage

---

### 3. Installation

**Purpose**: Get the tool onto user's system with minimal friction

**Contents** (Linux-focused):

```markdown
## Installation

### Package Managers (Recommended)

#### Arch Linux
```bash
pacman -S wherehouse
```

#### Debian/Ubuntu
```bash
apt install wherehouse
```

#### Fedora/RHEL
```bash
dnf install wherehouse
```

#### Homebrew (Linux & macOS)
```bash
brew install wherehouse
```

### Pre-compiled Binaries

Download from [releases page](https://github.com/user/wherehouse/releases):
```bash
curl -LO https://github.com/user/wherehouse/releases/latest/download/wherehouse-linux-amd64
chmod +x wherehouse-linux-amd64
sudo mv wherehouse-linux-amd64 /usr/local/bin/wherehouse
```

### Build from Source

**Requirements**: Go 1.21+

```bash
git clone https://github.com/user/wherehouse.git
cd wherehouse
make install  # or: go install
```

### Verification

Verify installation:
```bash
wherehouse --version
wherehouse doctor  # run self-test
```
```

**Best Practices**:
- **Order by popularity**: Package managers first, then binaries, then source
- **Multiple Linux distros**: Cover major families (Debian, Arch, RPM-based)
- **Architecture clarity**: Specify amd64, arm64, etc.
- **Verification steps**: Include post-install validation
- **Uninstall instructions** (often forgotten):
  ```markdown
  ### Uninstall
  ```bash
  # Package manager
  apt remove wherehouse

  # Manual install
  sudo rm /usr/local/bin/wherehouse
  rm -rf ~/.config/wherehouse
  ```
  ```

**Linux-Specific Considerations**:
- **XDG Base Directory** compliance - mention `~/.config/`, `~/.local/share/`
- **Shell completions** - bash, zsh, fish installation
- **systemd integration** (if applicable) - service files, timers
- **Man pages** - `man wherehouse` availability

**Anti-Pattern**: Only providing "build from source" or "npm install -g"

---

### 4. Usage / Getting Started

**Purpose**: Enable users to accomplish their first task

**Contents**:

```markdown
## Usage

### Initialize Database

```bash
wherehouse init
# → Created database at ~/.local/share/wherehouse/inventory.db
```

### Basic Operations

**Add an item:**
```bash
wherehouse add "10mm socket wrench" --location "Garage:Toolbox"
```

**Find an item:**
```bash
wherehouse where "socket"
# → 10mm socket wrench: Garage >> Toolbox >> Socket Set
# → 13mm socket wrench: Garage >> Toolbox >> Socket Set
```

**Move an item:**
```bash
wherehouse move "10mm socket" Kitchen --temporary
```

### Common Workflows

**Starting a project:**
```bash
wherehouse project create fixing-sink
wherehouse move "wrench" Kitchen --project fixing-sink
wherehouse move "plunger" Kitchen --project fixing-sink
```

**Finding lost items:**
```bash
wherehouse missing "screwdriver"
wherehouse history "screwdriver"  # see last known location
```

### Configuration

Config file: `~/.config/wherehouse/config.toml`

```toml
db_path = "/mnt/nas/inventory.db"  # network storage
default_verbosity = "quiet"
```

**Environment variables:**
- `WHEREHOUSE_DB_PATH` - Override database location
- `WHEREHOUSE_CONFIG` - Custom config file path
```

**Best Practices**:
- **Task-oriented sections** - "How do I...?" not "The --flag option..."
- **Progression**: Basic → Common workflows → Advanced usage
- **Configuration hierarchy** - Flags > env vars > config file > defaults
- **Output examples** - Show what success looks like
- **Common errors** - Address predictable mistakes

**Linux-Specific**:
- Mention **PATH** setup if not using package manager
- **Shell integration** - completion, aliases, functions
- **Permissions** - If tool needs sudo or specific user/group
- **File locations** - Follow FHS (Filesystem Hierarchy Standard)

**Anti-Pattern**: Dumping full `--help` output into README

---

### 5. Features

**Purpose**: Sell the tool to evaluators and document capabilities

**Contents**:

```markdown
## Features

- ✅ **Event-sourced architecture** - Complete audit trail, rebuild state from history
- 🚀 **Fast lookups** - SQLite-backed projections, optimized queries
- 🌳 **Hierarchical locations** - Nested organization (Garage > Toolbox > Drawer)
- 🏷️ **Project tracking** - Associate items with temporary projects
- 🔍 **Rich search** - By name, location, project, status
- 📊 **History tracking** - See every move, note, and change
- 🔧 **Self-healing** - `doctor` command validates and repairs database
- 🌐 **Network storage** - Works with NFS, SMB, network mounts (WAL mode)
- 📱 **Multi-user** - Attribution tracking (no permissions, trust-based)
- 💾 **Single file** - Entire database in one portable SQLite file
- 🎨 **Terminal UI** (planned) - Interactive TUI for visual browsing

**Design Philosophy:**
- Explicit over implicit (no silent magic)
- Deterministic over convenient (event_id ordering)
- Transparent over abstracted (direct SQL, no ORM)
```

**Best Practices**:
- **Bullets or checkboxes** - Scannable list format
- **Emoji/Icons** (optional) - Visual categorization
- **Differentiation** - What makes this tool unique vs alternatives
- **Trade-offs** - Honest about limitations ("Single SQLite file = not for 10M items")
- **Grouping** - Performance, UX, Architecture, Integration, etc.

**Anti-Pattern**: Marketing fluff without technical substance

---

### 6. Command Reference

**Purpose**: Comprehensive command documentation

**Contents**:

```markdown
## Commands

### `wherehouse add <item> --location <location>`

Create a new item in the inventory.

**Options:**
- `--location <path>` - Required. Hierarchical path (e.g., `Garage:Toolbox`)
- `--project <slug>` - Optional. Associate with project
- `--note <text>` - Optional. Add creation note

**Examples:**
```bash
wherehouse add "screwdriver" --location Workshop:ToolWall
wherehouse add "ladder" --location Garage --note "borrowed from neighbor"
```

### `wherehouse where <item>`

Find location of item(s).

**Options:**
- `-v, --verbose` - Show full paths and metadata
- `--json` - Output as JSON for scripting

**Examples:**
```bash
wherehouse where socket         # shows all matching items
wherehouse where "10mm socket"  # exact name match
wherehouse where --location Garage  # all items in location
```

### Global Flags

- `-h, --help` - Show help
- `--version` - Show version
- `--config <path>` - Custom config file
- `-q, --quiet` - Suppress non-error output
- `-v, --verbose` - Increase verbosity (repeatable: -vv, -vvv)
```

**Best Practices**:
- **Group by function** - Data management, querying, maintenance
- **Syntax first** - `command <required> [optional]` format
- **Flag conventions**:
  - Short (`-h`) and long (`--help`) forms
  - Common flags consistent: `-v` = verbose, `-q` = quiet, `-f` = force
- **Return codes** - Document non-zero exit codes for scripting
- **Dangerous operations** - Mark destructive commands clearly

**Alternative**: Link to full reference
```markdown
## Commands

See [COMMANDS.md](docs/COMMANDS.md) for full reference.

Quick reference:
- `add` - Create item
- `move` - Relocate item
- `where` - Find item
- `history` - Show item timeline
```

**Anti-Pattern**: Mixing command reference with tutorial content

---

### 7. Configuration

**Purpose**: Document all configuration options

**Contents**:

```markdown
## Configuration

**File locations** (in priority order):
1. `--config <path>` flag
2. `WHEREHOUSE_CONFIG` environment variable
3. `./wherehouse.toml` (current directory)
4. `~/.config/wherehouse/config.toml` (XDG_CONFIG_HOME)
5. Built-in defaults

**Example config** (`config.toml`):

```toml
config_version = 1  # Required

# Database
db_path = "~/.local/share/wherehouse/inventory.db"
# db_path = "/mnt/nas/shared/inventory.db"  # network storage

# Behavior
default_grouping = "location"  # or "project"
logging_level = "info"  # debug, info, warn, error

# User identity
[users.alice]
display_name = "Alice Smith"
email = "alice@example.com"

[user_identity]
os_username_map = { "asmith" = "alice" }
```

**Environment variables:**
- `WHEREHOUSE_DB_PATH` - Override database path
- `WHEREHOUSE_LOG_LEVEL` - Override logging level
- `NO_COLOR` - Disable colored output

**XDG directories:**
- Config: `~/.config/wherehouse/`
- Data: `~/.local/share/wherehouse/`
- Cache: `~/.cache/wherehouse/`
```

**Best Practices**:
- **Resolution order** - Flags > env vars > config file > defaults
- **Example file** - Complete, commented example
- **Validation** - How to check config validity
- **Migration** - Version bumps and breaking changes

**Linux-Specific**:
- **XDG compliance** - Follow XDG Base Directory spec
- **Env var conventions** - `TOOLNAME_OPTION` format
- **Config file format** - TOML/YAML/JSON (TOML recommended for clarity)

**Anti-Pattern**: Requiring config file for basic usage

---

### 8. Integration & Scripting

**Purpose**: Show how tool fits into workflows

**Contents**:

```markdown
## Integration

### Shell Completion

**Bash:**
```bash
wherehouse completion bash | sudo tee /etc/bash_completion.d/wherehouse
```

**Zsh:**
```bash
wherehouse completion zsh > ~/.zsh/completions/_wherehouse
```

**Fish:**
```bash
wherehouse completion fish > ~/.config/fish/completions/wherehouse.fish
```

### JSON Output for Scripting

All commands support `--json` for machine-readable output:

```bash
wherehouse where socket --json | jq '.[] | .location'
```

Example output:
```json
[
  {
    "item_id": "8f3a2c1d-...",
    "display_name": "10mm socket wrench",
    "location": "Garage >> Toolbox",
    "project": null,
    "last_updated": "2026-02-20T15:30:00Z"
  }
]
```

### Exit Codes

- `0` - Success
- `1` - General error
- `2` - Command-line usage error
- `3` - Database error
- `4` - Item not found
- `5` - Validation error

### Example Scripts

**Find all tools in a project:**
```bash
#!/bin/bash
PROJECT="deck-rebuild"
wherehouse where --project "$PROJECT" --json | jq -r '.[] | .display_name'
```

**Backup database:**
```bash
#!/bin/bash
DB_PATH=$(wherehouse config db-path)
cp "$DB_PATH" "backup-$(date +%Y%m%d).db"
```
```

**Best Practices**:
- **Shell completions** - Critical for CLI UX
- **JSON output** - Enable piping to jq, csvkit, etc.
- **Exit codes** - Document for script error handling
- **Example scripts** - Show real-world integration patterns
- **Pipeline examples** - How tool works with grep, awk, sed, jq

**Anti-Pattern**: Only supporting human-readable output

---

### 9. Development / Contributing

**Purpose**: Lower barrier for contributors

**Contents**:

```markdown
## Development

### Prerequisites

- Go 1.21+
- SQLite 3.x
- make (optional)

### Build from Source

```bash
git clone https://github.com/user/wherehouse.git
cd wherehouse
go build -o wherehouse
./wherehouse --version
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Integration tests
go test -tags=integration ./...
```

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Quick start:**
1. Fork the repository
2. Create feature branch: `git checkout -b feat/my-feature`
3. Make changes and add tests
4. Run `make lint test`
5. Submit pull request

**Code style:**
- Follow `gofmt` formatting
- Write tests for new features
- Update documentation

### Project Structure

```
wherehouse/
├── cmd/              # CLI commands
├── internal/
│   ├── database/     # SQLite operations
│   ├── events/       # Event handling
│   └── validation/   # Business rules
├── docs/             # Documentation
└── tests/            # Integration tests
```
```

**Best Practices**:
- **Prerequisites** - Exact versions required
- **Quick test** - `make test` or `go test ./...`
- **Contribution guide link** - Keep README concise, detail in CONTRIBUTING.md
- **Code standards** - Linting, formatting, test requirements
- **Project structure** - High-level directory explanation

**Anti-Pattern**: Dumping entire architecture documentation in README

---

### 10. Troubleshooting / FAQ

**Purpose**: Address common issues proactively

**Contents**:

```markdown
## Troubleshooting

### `wherehouse: command not found`

**Cause**: Binary not in PATH

**Solution:**
```bash
# Find wherehouse location
which wherehouse

# If missing, add to PATH (in ~/.bashrc or ~/.zshrc)
export PATH="$PATH:$HOME/.local/bin"

# Or install system-wide
sudo cp wherehouse /usr/local/bin/
```

### Database locked errors

**Cause**: Multiple processes accessing database simultaneously

**Solution:**
- Check for other wherehouse processes: `ps aux | grep wherehouse`
- Verify network mount supports file locking
- Increase busy_timeout in config

### `wherehouse doctor` reports corruption

**Cause**: Inconsistent projections vs event log

**Solution:**
```bash
# Rebuild projections from events
wherehouse doctor --rebuild

# Backup first
cp ~/.local/share/wherehouse/inventory.db backup.db
```

### Performance degradation

**Cause**: Database needs optimization

**Solution:**
```bash
# Vacuum and analyze
wherehouse maintenance --vacuum --analyze
```

## FAQ

**Q: Can I use wherehouse on network storage?**
A: Yes, SQLite WAL mode supports NFS/SMB. Configure in config.toml.

**Q: How do I migrate from v1 to v2?**
A: See [MIGRATION.md](docs/MIGRATION.md) for upgrade guide.

**Q: Does wherehouse support multi-tenant?**
A: No, it's designed for personal/small team use with attribution only.
```

**Best Practices**:
- **Problem → Solution** format
- **Common issues first** - Installation, permissions, PATH
- **Actionable commands** - Copy-paste solutions
- **Link to full docs** - For complex topics

**Anti-Pattern**: "Works on my machine" without troubleshooting help

---

### 11. Performance / Benchmarks

**Purpose**: Set expectations, prove claims

**Contents**:

```markdown
## Performance

**Test environment**:
- CPU: AMD Ryzen 9 5900X
- Storage: NVMe SSD
- Database: 10,000 items, 50,000 events

**Benchmarks:**

| Operation | Time | Notes |
|-----------|------|-------|
| `wherehouse where socket` | 12ms | Cold cache |
| `wherehouse add` | 8ms | Including validation |
| `wherehouse history` | 15ms | 100 event history |
| `wherehouse doctor --rebuild` | 450ms | Full projection rebuild |

**Comparison:**

Wherehouse is ~3x faster than traditional CRUD inventory systems due to:
- SQLite indexes on canonical names
- Projection-based queries (no JOIN overhead)
- Event log append-only writes

**Scaling limits:**
- ✅ Tested: 100,000 items, 500,000 events
- ⚠️  Practical: 10,000-50,000 items for responsive UX
- ❌ Not designed: Millions of items (use PostgreSQL backend)
```

**Best Practices**:
- **Test environment** - Hardware, dataset size
- **Real-world operations** - Not synthetic benchmarks
- **Honest limits** - When tool isn't appropriate
- **Comparison context** - vs alternatives, not absolute numbers

**Anti-Pattern**: Benchmarks without methodology or hardware specs

---

### 12. Project Status / Roadmap

**Purpose**: Set expectations for maturity and direction

**Contents**:

```markdown
## Project Status

**Current version**: 1.0.0 (stable)

**Stability**: Production-ready for personal/small team use

**Roadmap:**

### v1.1 (Q2 2026)
- [ ] Terminal UI (TUI) for visual browsing
- [ ] Export/import functionality
- [ ] Full-text search

### v2.0 (Q4 2026)
- [ ] Plugin system for custom event types
- [ ] Web UI (read-only dashboard)
- [ ] Multi-database sync (experimental)

**Not planned:**
- Cloud hosting service
- Mobile apps
- Real-time collaboration
- Enterprise features (SSO, RBAC)

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history.

**Recent releases:**
- **1.0.0** (2026-02-21) - Initial stable release
  - Event-sourced architecture
  - SQLite backend
  - Full CLI implementation
```

**Best Practices**:
- **Version badge** - Current stable version
- **Maturity indication** - Alpha, beta, stable, deprecated
- **Roadmap transparency** - What's coming, what's not
- **Changelog link** - Keep README concise

**Anti-Pattern**: Promising features that never ship

---

### 13. License & Credits

**Purpose**: Legal clarity and attribution

**Contents**:

```markdown
## License

MIT License - see [LICENSE](LICENSE) for details.

**TL;DR**: Free to use, modify, distribute. No warranty.

## Credits

Created by [Your Name](https://github.com/yourusername)

**Contributors:**
- [@contributor1](https://github.com/contributor1) - TUI implementation
- [@contributor2](https://github.com/contributor2) - Network storage support

**Acknowledgments:**
- Inspired by [Grocy](https://grocy.info/) and [PartKeepr](https://partkeepr.org/)
- Uses [Charm](https://charm.sh/) libraries for TUI
- Built with [Cobra](https://github.com/spf13/cobra) CLI framework

## Support

- 📖 [Documentation](https://wherehouse.example.com/docs)
- 💬 [Discussions](https://github.com/user/wherehouse/discussions)
- 🐛 [Issue Tracker](https://github.com/user/wherehouse/issues)
- 📧 Email: support@wherehouse.example.com
```

**Best Practices**:
- **License type** - Clear, linked to LICENSE file
- **Contributors** - Acknowledge major contributors
- **Support channels** - Where to get help
- **Sponsorship** (optional) - GitHub Sponsors, Open Collective

**Anti-Pattern**: Unclear licensing or missing attribution

---

## Linux-Specific Best Practices

### 1. Filesystem Hierarchy Standard (FHS)

Follow Linux conventions:

```
/usr/local/bin/wherehouse        # User-installed binaries
/etc/wherehouse/                 # System-wide config
~/.config/wherehouse/            # User config (XDG_CONFIG_HOME)
~/.local/share/wherehouse/       # User data (XDG_DATA_HOME)
~/.cache/wherehouse/             # Cache files (XDG_CACHE_HOME)
/usr/share/man/man1/wherehouse.1 # Man page
```

Document this in README:

```markdown
### File Locations

**XDG-compliant paths:**
- Config: `$XDG_CONFIG_HOME/wherehouse/` (default: `~/.config/wherehouse/`)
- Data: `$XDG_DATA_HOME/wherehouse/` (default: `~/.local/share/wherehouse/`)
- Cache: `$XDG_CACHE_HOME/wherehouse/` (default: `~/.cache/wherehouse/`)

**Environment variable overrides:**
- `WHEREHOUSE_CONFIG_DIR` - Config directory
- `WHEREHOUSE_DATA_DIR` - Data directory
```

### 2. Man Pages

Mention man page availability:

```markdown
### Documentation

**Man page:**
```bash
man wherehouse
man wherehouse-add     # subcommand help
```

**Online docs:** https://wherehouse.example.com/docs
```

### 3. Shell Completion

Critical for UX, document prominently:

```markdown
### Shell Completion

**Install completions during setup:**

```bash
# Bash
wherehouse completion bash | sudo tee /etc/bash_completion.d/wherehouse

# Zsh (add to ~/.zshrc)
autoload -U compinit; compinit
wherehouse completion zsh > "${fpath[1]}/_wherehouse"

# Fish
wherehouse completion fish > ~/.config/fish/completions/wherehouse.fish
```

**Verify:**
```bash
wherehouse wh<TAB>  # should complete to "wherehouse where"
```

### 4. systemd Integration

If tool can run as service/timer:

```markdown
### systemd Service (Optional)

**Run as background sync service:**

```bash
# Install service file
sudo cp contrib/wherehouse-sync.service /etc/systemd/system/
sudo systemctl enable wherehouse-sync
sudo systemctl start wherehouse-sync

# Check status
systemctl status wherehouse-sync
```

**Timer for periodic operations:**
```bash
sudo cp contrib/wherehouse-backup.timer /etc/systemd/system/
sudo systemctl enable wherehouse-backup.timer
```

### 5. Distribution Packaging

If providing distribution packages:

```markdown
### Distribution Packages

**Official packages:**
- Arch Linux: [AUR/wherehouse](https://aur.archlinux.org/packages/wherehouse)
- Ubuntu PPA: [ppa:user/wherehouse](https://launchpad.net/~user/+archive/ubuntu/wherehouse)
- Fedora COPR: [copr/wherehouse](https://copr.fedorainfracloud.org/coprs/user/wherehouse/)

**Community packages:**
- NixOS: `nixpkgs.wherehouse`
- Gentoo: `app-misc/wherehouse`
```

---

## README Structure Template

**Recommended order for Linux CLI tools:**

```markdown
# Tool Name

> One-line description

[Badges: Build | Coverage | Version | License]

## Demo / Quick Example
(Screenshot, GIF, or Asciinema)

## Features
(Bulleted list of key capabilities)

## Installation
### Package Managers (Linux focus)
- Arch, Debian, Fedora, etc.
### Pre-compiled Binaries
### Build from Source

## Quick Start
(5-minute getting started guide)

## Usage
### Basic Commands
### Common Workflows
### Configuration

## Commands Reference
(Brief reference or link to full docs)

## Integration
### Shell Completion
### JSON Output
### Exit Codes
### Pipeline Examples

## Troubleshooting
(Common issues and solutions)

## Development
(Building, testing, contributing link)

## Performance / Benchmarks
(Optional, if performance is a selling point)

## Roadmap
(Future plans, what's not planned)

## License & Credits

## Support
(Where to get help)
```

**Estimated length**: 300-600 lines for comprehensive README

---

## Anti-Patterns to Avoid

### 1. **Help Output Dumping**
❌ Don't paste entire `--help` output into README
✅ Do show 2-3 key commands with output examples

### 2. **No Quick Start**
❌ Don't start with architecture diagrams
✅ Do lead with "install → run first command → see output"

### 3. **Installation Vagueness**
❌ "Install it however you want"
✅ "Use apt install, or download binary, or build from source"

### 4. **Missing Linux Distro Coverage**
❌ Only showing Ubuntu instructions
✅ Cover Arch, Debian, Fedora, and "other" with binary

### 5. **No Visual Examples**
❌ Walls of text explaining features
✅ GIF showing the tool in action

### 6. **Broken Links**
❌ "See docs" without link
✅ Link to actual documentation: [Commands](docs/commands.md)

### 7. **Version Ambiguity**
❌ No version mentioned, unclear stability
✅ "Current version: 1.2.0 (stable)" with badge

### 8. **No Uninstall Instructions**
❌ Only showing how to install
✅ Document how to cleanly remove tool and config

### 9. **Technical Jargon Without Context**
❌ "Event-sourced CQRS architecture with projections"
✅ "Complete audit trail - rebuild state from history"

### 10. **No Support Path**
❌ No indication where to get help
✅ "Questions? Open a discussion or file an issue"

---

## Tools for README Creation

### Generators
- **readme-md-generator** - Interactive README generator
- **make-readme-markdown** - Scaffold based on package.json

### Demo Creation
- **Asciinema** - Terminal session recorder (text-based, searchable)
- **VHS** - Generate GIFs from scripts (by Charm)
- **termtosvg** - SVG terminal recordings

### Badges
- **shields.io** - Badge generation for build status, version, etc.
- **badgen.net** - Fast badge service

### Linting
- **awesome-lint** - Lint awesome lists and READMEs
- **remark-lint** - Markdown linting

### Testing
- **markdown-link-check** - Verify all links work
- **README-rating** - Score README quality

---

## Examples from Popular Linux CLI Tools

### Minimal but Effective
- **ripgrep**: Features → Installation → Usage → FAQ (short, practical)
- **fd**: Demo GIF → Features → Installation → How to Use

### Comprehensive
- **GitHub CLI (gh)**: Installation by OS → Usage → Manual → Contributing
- **bat**: Features → Installation → Usage → Customization → Integration

### Balanced
- **fzf**: Demo → Installation → Usage → Examples → Tips → Advanced

**Common Pattern**: Demo/Example → Installation → Usage → Reference

---

## Checklist for CLI README

Before publishing, verify:

- [ ] **One-line description** - What tool does, who it's for
- [ ] **Demo or screenshot** - Visual proof of value
- [ ] **Installation for 3+ Linux distros** - Arch, Debian, Fedora
- [ ] **Quick start** - 5-minute "hello world"
- [ ] **Usage examples** - Real-world tasks with output
- [ ] **Configuration** - Where config lives, example file
- [ ] **Shell completion** - Installation instructions
- [ ] **JSON/machine output** - Scripting support
- [ ] **Exit codes** - Documented for scripts
- [ ] **Troubleshooting** - 3+ common issues
- [ ] **Uninstall instructions** - Clean removal
- [ ] **License** - Clear and linked
- [ ] **Support path** - Where to get help
- [ ] **Build status badge** - CI passing
- [ ] **Version badge** - Current release
- [ ] **Links verified** - No 404s
- [ ] **XDG compliance mentioned** - Config/data paths

---

## References

### Best Practice Guides
- [Command Line Interface Guidelines](https://clig.dev/) - Comprehensive CLI design guide
- [Make a README](https://www.makeareadme.com/) - README structure guide
- [GitHub README Best Practices](https://github.com/jehna/readme-best-practices)
- [The Good Docs Project - README Template](https://www.thegooddocsproject.dev/template/readme)

### Example READMEs
- [ripgrep](https://github.com/BurntSushi/ripgrep) - Search tool README
- [fd](https://github.com/sharkdp/fd) - Find alternative with excellent demo
- [GitHub CLI](https://github.com/cli/cli) - Multi-platform installation
- [fzf](https://github.com/junegunn/fzf) - Interactive fuzzy finder

### Documentation Guides
- [Google Developer Documentation Style Guide - CLI Syntax](https://developers.google.com/style/code-syntax)
- [Telerik Style Guide - CLI Documentation](https://docs.telerik.com/style-guide/document-command-line-tools)

### Tools
- [Asciinema](https://asciinema.org/) - Terminal recordings
- [VHS](https://github.com/charmbracelet/vhs) - Terminal GIF generator
- [shields.io](https://shields.io/) - Badges

---

## Conclusion

**The perfect CLI README:**
1. **Shows value in 10 seconds** (demo/example)
2. **Gets users running in 30 seconds** (quick install)
3. **Solves first task in 2 minutes** (quick start)
4. **Supports advanced use in 10 minutes** (command reference)
5. **Enables integration in 15 minutes** (JSON output, scripting)

**For Linux CLI tools specifically:**
- Cover major distros (Arch, Debian, Fedora)
- Document XDG-compliant paths
- Provide shell completion instructions
- Include man page availability
- Show integration with standard Unix tools (grep, jq, etc.)

**Remember**: Lead with examples, support with reference. Users want to see the tool work before reading how it works.

---

**Version**: 1.0
**Status**: Research complete
**Next Steps**: Apply to Wherehouse README.md
