package app

import (
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrLenderRequired is returned when a borrow request has no lender (StatusContext).
var ErrLenderRequired = errors.New("lender is required")

// ErrRecipientRequired is returned when a loan request has no recipient (StatusContext).
var ErrRecipientRequired = errors.New("recipient is required")

// ErrHasChildren is returned when an operation that removes an entity finds it
// still has non-removed children.
var ErrHasChildren = errors.New("entity still contains other entities")

// ErrDuplicateEntity is returned when a batch names the same entity more than once.
var ErrDuplicateEntity = errors.New("entity named more than once")

// ErrNoChange is returned when a command would set an attribute to the value it
// already has (ADR 0024: no-op changes are errors, not events).
var ErrNoChange = errors.New("no change")

// ErrNoAttributes is returned when an attribute request names no attribute.
var ErrNoAttributes = errors.New("no attribute to change")

// ErrInvalidDestination is returned when a request names a parent the entity
// may not be placed under: a discrete entity (create, borrow, move), or for a
// move also itself, one of its descendants, its current parent, or its home
// (use/move while displaced) (ADR 0034, ADR 0039).
var ErrInvalidDestination = errors.New("invalid destination")

// ErrDestinationIsHome is returned when a use or move names the entity's home
// as the destination; going home is a return (ADR 0039). It is always wrapped
// together with ErrInvalidDestination.
var ErrDestinationIsHome = errors.New("destination is the entity's home")

// ErrIsHome is returned when an entity cannot be removed or made discrete
// because displaced entities belong in it (ADR 0039).
var ErrIsHome = errors.New("entity is home to displaced entities")

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
