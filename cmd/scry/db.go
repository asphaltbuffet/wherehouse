package scry

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type scryApp interface {
	ListEntities(ctx context.Context) ([]app.EntityResult, error)
	FindEntities(ctx context.Context, req app.FindEntitiesRequest) ([]app.FindResult, error)
}
