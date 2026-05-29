package remove

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type removeApp interface {
	RemoveEntity(ctx context.Context, req app.RemoveEntityRequest) error
}
