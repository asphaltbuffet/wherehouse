package add

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type addApp interface {
	CreateEntity(ctx context.Context, req app.CreateEntityRequest) (app.EntityResult, error)
}
