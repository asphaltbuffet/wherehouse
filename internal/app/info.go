package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

const displayNameKey = "display_name"
const maxNameLength = 255

// GetInfo returns the database display name, path, and entity counts by status.
func (a *App) GetInfo(ctx context.Context) (InfoResult, error) {
	name, err := a.store.GetMetadata(ctx, displayNameKey)
	if errors.Is(err, store.ErrNotFound) {
		name = ""
	} else if err != nil {
		return InfoResult{}, fmt.Errorf("get info: %w", err)
	}

	if name == "" {
		name = "(unnamed)"
	}

	counts, err := a.store.CountEntitiesByStatus(ctx)
	if err != nil {
		return InfoResult{}, fmt.Errorf("get info counts: %w", err)
	}

	return InfoResult{
		Name:         name,
		DatabasePath: a.store.Path(),
		EntityCounts: counts,
	}, nil
}

// SetWherehouseName stores a display name for this database.
func (a *App) SetWherehouseName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if strings.ContainsAny(name, "\n\r") {
		return errors.New("name cannot contain newlines")
	}
	if len(name) > maxNameLength {
		return errors.New("name cannot exceed 255 characters")
	}
	if err := a.store.SetMetadata(ctx, displayNameKey, name); err != nil {
		return fmt.Errorf("set wherehouse name: %w", err)
	}
	return nil
}

// ClearWherehouseName removes the display name, reverting to "(unnamed)".
func (a *App) ClearWherehouseName(ctx context.Context) error {
	if err := a.store.DeleteMetadata(ctx, displayNameKey); err != nil {
		return fmt.Errorf("clear wherehouse name: %w", err)
	}
	return nil
}
