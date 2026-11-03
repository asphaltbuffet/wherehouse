package eventbus

import "errors"

// ErrNoSubject is returned when a dispatched payload names no entity.
var ErrNoSubject = errors.New("payload has no entity_id")

// ErrDerivedEvent is returned when Dispatch is given a derived event, which
// only the bus writes.
var ErrDerivedEvent = errors.New("derived events are written only by the bus")

// ErrSubjectMismatch is returned when an imported event's entity_id column
// disagrees with its payload's entity_id (ADR 0042).
var ErrSubjectMismatch = errors.New("entity_id does not match payload")
