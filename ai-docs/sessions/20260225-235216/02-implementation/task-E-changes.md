# Task E Changes: Refactor cmd/config/check.go

## File Modified

`/home/grue/dev/wherehouse/cmd/config/check.go`

## Changes Made

Replaced two calls to `loadConfigFile(cmdFS, ...)` with `config.Check(cmdFS, ...)`.

### Before

```go
if err := loadConfigFile(cmdFS, expandedGlobal); err != nil {
```

```go
if err := loadConfigFile(cmdFS, expandedLocal); err != nil {
```

### After

```go
if err := config.Check(cmdFS, expandedGlobal); err != nil {
```

```go
if err := config.Check(cmdFS, expandedLocal); err != nil {
```

## No Other Changes

- No imports added or removed (`internal/config` was already imported).
- No logic changes; only the function called changed.
- `helpers.go` was not modified.
