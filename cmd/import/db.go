// Package importcmd implements the import command.
package importcmd

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type importApp interface {
	ImportEvents(ctx context.Context, events []app.ExportResult, opts app.ImportOptions) (app.ImportResult, error)
	HasEvents(ctx context.Context) (bool, error)
}
