# Suggested Commands

## Build & test
```
mise run build      # go build → dist/wherehouse
mise run test       # gotestsum, race, coverage → bin/coverage.out
mise run lint       # golangci-lint --fix → bin/golangci-lint.html
mise run generate   # go generate ./... (stringer)
mise run mock       # regenerate mocks via mockery
mise run dev        # generate + lint + test + snapshot + mock
mise run snapshot   # goreleaser single-target snapshot
mise run cover      # coverage HTML → bin/coverage.html
mise run mod-tidy   # go mod tidy + gomod2nix generate
```

## Single-package / single-test
```
gotestsum -- -race ./internal/store/...
gotestsum -- -run TestFoo ./cmd/scry/...
```

## VCS (jujutsu, not git)
```
jj log --no-graph -r 'trunk()..@'   # review branch history
jj describe <change-id> -m "msg"    # rename a commit
```

## Preferred shell tools
- `fd` over `find`
- `rg` over `grep`
- `sd` over `sed`
- `jq` for JSON
