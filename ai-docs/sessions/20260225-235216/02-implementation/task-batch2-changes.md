# Batch 2 Changes

## Files Modified

### /home/grue/dev/wherehouse/internal/cli/flags.go
- Added import: `"context"`
- Added import: `"github.com/asphaltbuffet/wherehouse/internal/config"`
- Added function: `GetConfig(ctx context.Context) (*config.Config, bool)`
- Added function: `MustGetConfig(ctx context.Context) *config.Config`

### /home/grue/dev/wherehouse/internal/cli/output.go
- Added import: `"github.com/asphaltbuffet/wherehouse/internal/config"`
- Added function: `NewOutputWriterFromConfig(out, err io.Writer, cfg *config.Config) *OutputWriter`

## Verification
- `go build ./...` passed with no errors
