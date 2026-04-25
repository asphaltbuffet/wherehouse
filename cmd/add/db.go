package add

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type addDB interface {
	Close() error
	GetEntity(ctx context.Context, entityID string) (*database.Entity, error)
	GetEntitiesByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Entity, error)
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

var _ addDB = (*database.Database)(nil)
