package find

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// findDB is the database interface required by the find command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery
type findDB interface {
	Close() error
	SearchByName(ctx context.Context, name string, limit int) ([]*database.SearchResult, error)
	GetItemLoanedInfo(ctx context.Context, itemID string) (*database.LoanedInfo, error)
}
