# Tech Stack

- Language: Go 1.25, no CGo
- CLI framework: cobra
- DB: SQLite (via `internal/store`)
- Config: viper-backed TOML (XDG)
- Styling: lipgloss (Wong palette, `AdaptiveColor{Light, Dark}`)
- Test: `testify/assert` (non-fatal), `testify/require` (preconditions); mocks via mockery
- Build: mise tasks (see `mem:suggested_commands`)
- Release: goreleaser (snapshot only via `mise run snapshot`)
- Module: `github.com/asphaltbuffet/wherehouse`
