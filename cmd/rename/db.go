package rename

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type renameApp interface {
	RenameEntity(ctx context.Context, req app.RenameEntityRequest) (app.EntityResult, error)
}
