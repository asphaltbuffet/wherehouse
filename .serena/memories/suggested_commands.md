# Suggested Commands for wherehouse Development

## Essential Build/Test Commands
```bash
mise run build       # build to dist/wherehouse
mise run test        # run all tests (gotestsum, race detector, coverage)
mise run lint        # golangci-lint --fix, outputs bin/golangci-lint.html
mise run generate    # go generate ./... (stringer, etc.)
mise run mock        # generate mocks with mockery
mise run dev         # full pipeline: generate + lint + test + snapshot + mock
mise run snapshot    # goreleaser build snapshot (single target)
mise run cover       # coverage HTML report to bin/coverage.html
mise run mod-tidy    # go mod tidy + gomod2nix generate
mise run update-deps # go get -u + mod-tidy
mise run clean       # remove dist/, bin/, build artifacts
```

## VCS (jj, not git)
```bash
jj log                                        # show commit history
jj show <change_id>                           # show what a commit changed
jj log --no-graph -r 'description(glob:"*keyword*")' -T 'change_id ++ " " ++ description.first_line() ++ "\n"'
                                              # find historical commit by keyword
```

## Code Generation
```bash
go generate ./...                             # regenerate eventtype_string.go, mocks
```

## Running the Binary
```bash
dist/wherehouse --help
dist/wherehouse initialize database
dist/wherehouse add item "name" --in "Location"
dist/wherehouse find "query"
dist/wherehouse history "item"
dist/wherehouse move "item" "NewLocation"
```

## Notes
- Always run `mise run lint` and `mise run test` before committing
- Use `/pre-commit` skill before every commit
- Use `/commit` skill for commit message conventions
- Use `/audit-docs` skill after features or fixes
- No `&&` between shell commands — run as separate tool calls
- No `git` commands — use `jj` equivalents
