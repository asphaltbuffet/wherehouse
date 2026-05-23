// Package history implements the history command for displaying entity event history.
package history

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type historyApp interface {
	GetHistory(ctx context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error)
}
