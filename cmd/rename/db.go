// Package rename implements the rename command for updating entity display names.
package rename

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type renameDB interface {
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

var _ renameDB = (*database.Database)(nil)
