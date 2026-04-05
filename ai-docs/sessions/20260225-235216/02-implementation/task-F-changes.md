# Task F Changes: Refactor cmd/config/get.go

## File Modified

`/home/grue/dev/wherehouse/cmd/config/get.go`

## Change

**Line 70**: Replace local helper call with `internal/config` package function.

### Before
```go
value, err := getConfigValue(globalConfig, key)
```

### After
```go
value, err := config.GetValue(globalConfig, key)
```

## Import Changes

None. The `internal/config` package was already imported on line 11:
```go
"github.com/asphaltbuffet/wherehouse/internal/config"
```

## Unchanged

- The "show all" path using `toml.Marshal(globalConfig)` (lines 95-103) is unchanged.
- All flag parsing, output formatting, and error handling logic is unchanged.
- The `go-toml/v2` import is retained for the `toml.Marshal` call in the "show all" path.

## Verification

`go build ./cmd/config/...` passes with no errors.
