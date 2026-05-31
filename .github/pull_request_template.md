## Summary

<!-- What does this change do and why? -->

## Checklist

- [ ] New or changed behavior has test coverage

### If this PR triggers a release

- [ ] Ran `mise run release <major|minor|patch>`
- [ ] `CHANGELOG.md` has an entry for the new version

### If this PR adds a new event type

- [ ] Ran `go generate ./...` to regenerate `eventtype_string.go`
- [ ] Added entry to `payloadPrototypes` in `internal/app/doctor.go`

### If this PR adds a new CLI command

- [ ] `doc.go` with package comment in `cmd/xxx/`
- [ ] Registered in `cmd/root.go` via `NewDefaultXxxCmd()`
- [ ] Persistent flags (`--json`, `--quiet`) inherited via root command wiring
- [ ] Tests use `apptesting.OpenApp(t)` and assert on DB state
