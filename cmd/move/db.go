// Package move implements the move command for relocating entities.
package move

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type moveApp interface {
	ReparentEntity(ctx context.Context, req app.ReparentEntityRequest) (app.EntityResult, error)
}
