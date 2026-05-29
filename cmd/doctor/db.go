package doctor

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type doctorApp interface {
	ValidateEventLog(ctx context.Context) ([]app.DoctorIssue, error)
	CheckProjectionConsistency(ctx context.Context) ([]app.DoctorIssue, error)
	TruncateAndReplay(ctx context.Context) (int, error)
}
