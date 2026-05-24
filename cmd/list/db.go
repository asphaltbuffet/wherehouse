package list

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type listApp interface {
	ListEntities(ctx context.Context) ([]app.EntityResult, error)
}
