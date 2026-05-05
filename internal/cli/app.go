package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// App contains details on the application runtime.
type App struct {
	actor    string
	ctx      context.Context
	database *database.Database
}

// NewApp creates a new application manager.
func NewApp(ctx context.Context) (*App, error) {
	db, err := OpenDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	aID := GetActorUserID(ctx)

	return &App{
		aID,
		ctx,
		db,
	}, nil
}
