# Suggested Commands

All via `mise run <task>` (tasks live in `.mise/tasks/`):

| Task | What it does |
|---|---|
| `mise run build` | `go build → dist/wherehouse` |
| `mise run test` | gotestsum, race detector, coverage → `bin/coverage.out` |
| `mise run lint` | golangci-lint --fix → `bin/golangci-lint.html` |
| `mise run generate` | `go generate ./...` (stringer for iota enums) |
| `mise run mock` | regenerate mocks via mockery |
| `mise run dev` | generate + lint + test + snapshot + mock |
| `mise run snapshot` | goreleaser build (single target, no release) |
| `mise run cover` | coverage HTML → `bin/coverage.html` |
| `mise run mod-tidy` | `go mod tidy` + `gomod2nix generate` |

Single package test: `gotestsum -- -race ./internal/store/...`
Single test: `gotestsum -- -run TestFoo ./cmd/scry/...`

Preferred shell tools: `fd` (not find), `rg` (not grep), `sd` (not sed), `jq` for JSON.
VCS: `jj` only — never `git`.
