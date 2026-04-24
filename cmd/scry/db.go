package scry

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type scryDB interface {
	Close() error
	GetEntitiesByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Entity, error)
	ListEntities(ctx context.Context, underID, entityType, status string) ([]*database.Entity, error)
}

var _ scryDB = (*database.Database)(nil)
