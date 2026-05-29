# Tech Stack

- Language: Go 1.25, no CGo
- DB: SQLite via `database/sql` + `go-migrate` for migrations (`internal/store/migrations/`)
- CLI: Cobra (`cmd/`)
- Config: Viper, XDG-compliant TOML (`internal/config/`)
- Web: stdlib `net/http` + `html/template` + `//go:embed` (`internal/web/`)
- Logging: structured + rotation (`internal/logging/`)
- Styles: lipgloss + Wong palette (`internal/styles/`)
- ID generation: 10-char alphanumeric NanoID (`internal/nanoid/`)
- Build/task runner: `mise` (tasks in `.mise/tasks/`)
- Linter: golangci-lint
- Test runner: gotestsum
- Snapshot/release: goreleaser
- Nix: `gomod2nix` for reproducible builds
- Mocks: mockery (external deps only; internal interfaces use hand-rolled fakes)
- Assertions: testify (`assert` non-fatal, `require` preconditions)
