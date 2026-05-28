# Tech Stack

- **Language**: Go 1.25, no CGo
- **CLI framework**: Cobra (github.com/spf13/cobra)
- **Config**: Viper-backed TOML, XDG-compliant (`internal/config`)
- **DB**: SQLite (modernc.org/sqlite, no cgo); migrations in `internal/store/migrations/`
- **ID generation**: 10-char alphanumeric NanoID (`internal/nanoid`)
- **Build**: mise tasks (`mise run build/test/lint/generate/mock/dev/snapshot`)
- **Test runner**: gotestsum + race detector
- **Lint**: golangci-lint --fix
- **Code gen**: stringer for iota enums (`go generate ./...`), mockery for mocks
- **Release**: goreleaser (snapshot only in dev)
- **Web UI**: HTMX templates, embedded assets in `internal/web/assets`
- **Styling**: lipgloss + Wong palette with AdaptiveColor (colorblind safe)
- **Fuzzy search**: Levenshtein distance (`internal/app/search.go`)
- **Module management**: gomod2nix (`mise run mod-tidy`)
- **VCS**: jujutsu (`jj`), not git
