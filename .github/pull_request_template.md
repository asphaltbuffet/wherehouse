## Summary

<!-- What does this change do and why? -->

## Checklist

### Every PR

- [ ] `mise run lint` passes
- [ ] `mise run test` passes
- [ ] New or changed behavior has test coverage

### If this PR should trigger a release

- [ ] Ran `mise run release <major|minor|patch>` (updates `VERSION` and README nix pins)
- [ ] `CHANGELOG.md` has an entry for the new version (`## [x.y.z] - YYYY-MM-DD`)
- [ ] `VERSION`, `CHANGELOG.md`, and `README.md` are all updated in this PR

### If this PR adds a new event type

- [ ] Added to `EventType` iota + `eventTypeByName` map in `internal/inventory/event_type.go`
- [ ] Ran `go generate ./...` to regenerate `eventtype_string.go`
- [ ] Added case to `applyEventTx` in `internal/eventbus/bus.go`
- [ ] Added payload struct to `internal/eventbus/payloads.go`
- [ ] Added entry to `payloadPrototypes` in `internal/app/doctor.go`

### If this PR adds a new CLI command

- [ ] `NewXxxCmd(db xxxDB)` and `NewDefaultXxxCmd()` constructors present
- [ ] Per-command `xxxDB` interface defined in `cmd/xxx/db.go`
- [ ] Registered in `cmd/root.go` via `NewDefaultXxxCmd()`
- [ ] `--json` flag present
