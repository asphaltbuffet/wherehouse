// Package status implements the status command for changing entity status.
package status

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

type statusApp interface {
	ChangeStatus(ctx context.Context, req app.ChangeStatusRequest) error
}
