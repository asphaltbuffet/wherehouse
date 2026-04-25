// Package status implements the status command for changing entity status.
package status

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type statusDB interface {
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

var _ statusDB = (*database.Database)(nil)
