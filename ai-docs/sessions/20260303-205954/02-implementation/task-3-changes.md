# Task 3: OutputWriter Adoption Changes

## Summary
Adopted `cli.OutputWriter` in four command packages that previously bypassed it.

## Files Modified

### cmd/find/find.go
- Removed `"github.com/goccy/go-json"` import (no longer needed after removing custom encoder)
- Added `out := cli.NewOutputWriterFromConfig(...)` in `runFindCore`
- Changed `outputJSON(cmd.OutOrStdout(), ...)` to `outputJSON(out, ...)`
- Changed `outputHuman(cmd.OutOrStdout(), ...)` to `outputHuman(out.Writer(), ...)`
- Changed `outputJSON` signature from `(w io.Writer, ...)` to `(out *cli.OutputWriter, ...)`
- Replaced `json.NewEncoder(w).Encode(output)` with `out.JSON(output)`

### cmd/scry/scry.go
- Removed `"github.com/goccy/go-json"` import (no longer needed after removing custom encoder)
- Added `out := cli.NewOutputWriterFromConfig(...)` in `runScryCore`
- Changed `outputJSON(cmd.OutOrStdout(), result)` to `outputJSON(out, result)`
- Changed `outputHuman(cmd.OutOrStdout(), ...)` to `outputHuman(out.Writer(), ...)`
- Changed `fmt.Fprintf(cmd.OutOrStdout(), "No suggestions ...")` to `out.Println(...)`
- Changed `outputJSON` signature from `(w io.Writer, ...)` to `(out *cli.OutputWriter, ...)`
- Replaced `json.NewEncoder(w).Encode(output)` with `out.JSON(output)`

### cmd/history/output.go
- Removed `"github.com/spf13/cobra"` import (no longer needed)
- Added `"github.com/asphaltbuffet/wherehouse/internal/cli"` import
- Changed `formatOutput` signature: replaced `cmd *cobra.Command, jsonMode bool` with `out *cli.OutputWriter, jsonMode bool`
- Changed `formatJSON` signature from `(w io.Writer, ...)` to `(out *cli.OutputWriter, ...)`
- Replaced `json.NewEncoder(w).Encode(output)` in `formatJSON` with `out.JSON(output)`
- `formatHuman` and `formatEvent` retain `io.Writer` for lipgloss-styled writes; callers pass `out.Writer()`

### cmd/history/history.go
- Added `out := cli.NewOutputWriterFromConfig(...)` in `runHistoryCore`
- Changed `formatOutput(ctx, cmd, db, filtered, cfg.IsJSON())` to `formatOutput(ctx, out, db, filtered, cfg.IsJSON())`

### cmd/initialize/database.go
- Removed `"encoding/json"` import
- Added `"github.com/asphaltbuffet/wherehouse/internal/cli"` import
- Added `out := cli.NewOutputWriterFromConfig(...)` in `runInitializeDatabase`
- Changed `printInitResult(cmd, cfg, dbPath, backupPath)` to `printInitResult(out, cfg, dbPath, backupPath)`
- Changed `printInitResult` signature from `(cmd *cobra.Command, cfg *config.Config, ...)` to `(out *cli.OutputWriter, cfg *config.Config, ...)`
- Replaced `json.NewEncoder(cmd.OutOrStdout()).Encode(result)` with `out.JSON(result)`
- Replaced `fmt.Fprintf(cmd.OutOrStdout(), ...)` calls with `out.Info(...)` and `out.Success(...)`

## Verification
- `go build ./cmd/...`: clean
- `golangci-lint run ./cmd/find/... ./cmd/scry/... ./cmd/history/... ./cmd/initialize/...`: 0 issues
- `go test ./cmd/find/... ./cmd/scry/... ./cmd/history/... ./cmd/initialize/...`: all pass
