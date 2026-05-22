// Package eventbus owns all event processing for wherehouse.
// It is the single place where events are persisted and projections are updated.
// Business rules (validation, path propagation, derived events) live here.
package eventbus
