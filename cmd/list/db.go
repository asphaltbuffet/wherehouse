package list

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery

type listDB interface {
	Close() error
	ListEntities(ctx context.Context, underID, entityType, status string) ([]*database.Entity, error)
}

var _ listDB = (*database.Database)(nil)
