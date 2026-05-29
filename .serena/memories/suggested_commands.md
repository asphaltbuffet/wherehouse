# Suggested Commands

## Build & test
```
mise run build       # go build → dist/wherehouse
mise run test        # gotestsum, race detector, coverage → bin/coverage.out
mise run lint        # golangci-lint --fix → bin/golangci-lint.html
mise run generate    # go generate ./... (stringer for iota enums)
mise run mock        # regenerate mocks via mockery
mise run dev         # generate + lint + test + snapshot + mock
mise run snapshot    # goreleaser build (single target, no release)
mise run cover       # coverage HTML → bin/coverage.html
mise run mod-tidy    # go mod tidy + gomod2nix generate
```

## Single package / test
```
gotestsum -- -race ./internal/app/...
gotestsum -- -run TestFoo ./internal/app/...
```

## VCS (jujutsu — never use git directly)
```
jj log --no-graph -r 'trunk()..@'   # review branch history
jj describe <change-id> -m "msg"    # rename a commit
```

## Preferred shell tools
- `fd` over `find`, `rg` over `grep`, `sd` over `sed`, `jq` for JSON
