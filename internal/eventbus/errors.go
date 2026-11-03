package eventbus

import "errors"

// ErrNoSubject is returned when a dispatched payload names no entity.
var ErrNoSubject = errors.New("payload has no entity_id")
