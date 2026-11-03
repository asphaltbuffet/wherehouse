package eventbus

import "errors"

// ErrNoSubject is returned when a dispatched payload names no entity.
var ErrNoSubject = errors.New("payload has no entity_id")

// ErrDerivedEvent is returned when Dispatch is given a derived event, which
// only the bus writes.
var ErrDerivedEvent = errors.New("derived events are written only by the bus")
