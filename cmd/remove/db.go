package remove

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type removeApp interface {
	RemoveEntity(ctx context.Context, req app.RemoveEntityRequest) error
}
