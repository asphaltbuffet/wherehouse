package web

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

// App is the dependency contract the web package requires from the app layer.
type App interface {
	ListEntities(ctx context.Context) ([]app.EntityResult, error)
	GetEntityByID(ctx context.Context, entityID string) (app.EntityResult, error)
	GetChildren(ctx context.Context, parentID string) ([]app.EntityResult, error)
	HasLiveChildren(ctx context.Context, parentID string) (bool, error)
	GetHistory(ctx context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error)
	CreateEntity(ctx context.Context, req app.CreateEntityRequest) (app.EntityResult, error)
	RenameEntity(ctx context.Context, req app.RenameEntityRequest) (app.EntityResult, error)
	MarkLost(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	MarkFound(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	FindEntities(ctx context.Context, req app.FindEntitiesRequest) ([]app.FindResult, error)
	MarkReturned(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	MarkLoaned(ctx context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error)
	FindParentCandidates(ctx context.Context, req app.FindEntitiesRequest) ([]app.FindResult, error)
	GetEntityByIDAnyStatus(ctx context.Context, entityID string) (app.EntityResult, error)
	BorrowEntities(ctx context.Context, reqs []app.BorrowEntityRequest) ([]app.EntityResult, error)
	SetAttributes(ctx context.Context, reqs []app.SetAttributesRequest) ([]app.EntityResult, error)
}
