package tui

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// App is the subset of app.App the TUI requires.
type App interface {
	GetRootEntities(ctx context.Context) ([]app.EntityResult, error)
	GetChildren(ctx context.Context, parentID string) ([]app.EntityResult, error)
	CreateEntities(ctx context.Context, reqs []app.CreateEntityRequest) ([]app.EntityResult, error)
	MarkLoaned(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	BorrowEntities(ctx context.Context, reqs []app.BorrowEntityRequest) ([]app.EntityResult, error)
	MarkLost(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	MarkReturned(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	MarkFound(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	GetHistory(ctx context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error)
	FindEntities(ctx context.Context, req app.FindEntitiesRequest) ([]app.FindResult, error)
}
