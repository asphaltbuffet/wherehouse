package app

import (
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// wrapEntityError translates a store.GetEntity error into an app-layer error.
// store.ErrNotFound becomes app.ErrNotFound; all other errors (DB failures,
// context cancellations, etc.) are propagated directly so callers can
// distinguish "entity missing" from "storage unavailable".
func wrapEntityError(id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("get entity %q: %w", id, ErrNotFound)
	}
	return fmt.Errorf("get entity %q: %w", id, err)
}
