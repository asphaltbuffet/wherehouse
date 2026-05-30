# Tech Stack

- Language: Go 1.25, no CGo
- CLI framework: cobra
- DB: SQLite (via internal/store, migrations auto-apply)
- Config: viper-backed TOML, XDG paths
- UI: lipgloss (styles singleton), bubbletea where needed
- Build: mise (`mise run build/test/lint/generate/mock/dev/snapshot`)
- Test runner: gotestsum (race detector on)
- Lint: golangci-lint --fix
- Code gen: stringer (iota enums via `go generate`)
- Mocks: mockery (external deps only — see `mem:conventions`)
- Release: goreleaser
- Module pins: gomod2nix for Nix reproducibility
