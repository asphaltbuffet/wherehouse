# Suggested Commands

## Build & test (mise tasks in `.mise/tasks/`)

```
mise run build     # go build → dist/wherehouse
mise run test      # gotestsum, race detector, coverage
mise run lint      # golangci-lint --fix
mise run generate  # go generate ./... (stringer for iota enums)
mise run mock      # regenerate mocks via mockery
mise run dev       # generate + lint + test + snapshot + mock
mise run snapshot  # goreleaser build (single target)
mise run cover     # coverage HTML → bin/coverage.html
mise run mod-tidy  # go mod tidy + gomod2nix generate
```

## Single package / test

```
gotestsum -- -race ./internal/database/...
gotestsum -- -run TestFoo ./cmd/scry/...
```

## VCS (jujutsu — NOT git)

```
jj log --no-graph -r 'trunk()..@'   # branch history
jj describe <change-id> -m "msg"    # rename commit
```
