package add

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type addApp interface {
	CreateEntity(ctx context.Context, req app.CreateEntityRequest) (app.EntityResult, error)
}
