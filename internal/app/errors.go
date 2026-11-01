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

// ErrEmptyTag is returned when a tag to add has an empty canonical form:
// blank, or only spaces, dashes, and underscores (e.g. "--").
var ErrEmptyTag = errors.New("tag is empty")

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

// ErrHomeCycle is returned when a move or use would put a displaced entity's
// home inside the entity itself (ADR 0039). It is always wrapped together with
// ErrInvalidDestination.
var ErrHomeCycle = errors.New("home cycle")

// ErrDisplaced is returned when an action is refused because the entity is
// displaced, e.g. locking it: it must be returned first (ADR 0039).
var ErrDisplaced = errors.New("entity is displaced")

// ErrLocked is returned when an action is blocked by the entity's locked flag.
var ErrLocked = errors.New("entity is locked")

// ErrWrongStatus is returned when the entity's status is not a legal source
// for the action (ADR 0024, ADR 0039).
var ErrWrongStatus = errors.New("entity status does not allow this action")

// ErrDiscrete is returned when a child would be added to a discrete entity.
var ErrDiscrete = errors.New("entity is discrete")

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

// sentinelError is an error whose text is msg alone but which matches every
// sentinel in errs under [errors.Is]. It lets one rejection satisfy several
// sentinels without each sentinel's text being appended to the message.
type sentinelError struct {
	msg  string
	errs []error
}

func (e *sentinelError) Error() string   { return e.msg }
func (e *sentinelError) Unwrap() []error { return e.errs }

// rejection builds a sentinelError with the formatted message.
func rejection(errs []error, format string, args ...any) error {
	return &sentinelError{msg: fmt.Sprintf(format, args...), errs: errs}
}
