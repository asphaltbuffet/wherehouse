package web

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

// App is the dependency contract the web package requires from the app layer.
type App interface {
	ListEntities(ctx context.Context) ([]app.EntityResult, error)
	GetChildren(ctx context.Context, parentID string) ([]app.EntityResult, error)
	GetHistory(ctx context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error)
}
