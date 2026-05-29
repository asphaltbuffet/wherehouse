package list

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type listApp interface {
	ListEntities(ctx context.Context) ([]app.EntityResult, error)
}
