// Package move implements the move command for relocating entities.
package move

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type moveDB interface {
	Close() error
	GetEntity(ctx context.Context, entityID string) (*database.Entity, error)
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

var _ moveDB = (*database.Database)(nil)
