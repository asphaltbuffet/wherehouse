# Build & Test Commands (mise)

```
mise run build       # go build → dist/wherehouse
mise run test        # gotestsum --race --coverage
mise run lint        # golangci-lint --fix
mise run generate    # go generate ./... (stringer for iota enums)
mise run mock        # regenerate mocks via mockery
mise run dev         # generate + lint + test + snapshot + mock
mise run snapshot    # goreleaser single-target build
mise run cover       # coverage HTML → bin/coverage.html
mise run mod-tidy    # go mod tidy + gomod2nix

# Single package
gotestsum -- -race ./internal/eventbus/...

# Single test
gotestsum -- -run TestFoo ./cmd/scry/...
```

VCS is jujutsu (`jj`), not git. Never run `git` commands directly.
Mise tasks live in `.mise/tasks/` as scripts (not only in `mise.toml`).
