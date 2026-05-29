// Package export implements the export command for dumping all events as JSON.
package export

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type exportApp interface {
	GetAllEvents(ctx context.Context) ([]app.ExportResult, error)
}
