// Package status implements the status command for changing entity status.
package status

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type statusApp interface {
	ChangeStatus(ctx context.Context, req app.ChangeStatusRequest) error
}
