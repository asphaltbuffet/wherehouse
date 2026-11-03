package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// decodeAs is the one place a stored payload is unmarshalled.
func decodeAs[P Payload](raw json.RawMessage) (P, error) {
	var p P
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("decode %s: %w", p.EventType(), err)
	}
	return p, nil
}

func decodePayload[P Payload](raw json.RawMessage) (Payload, error) {
	p, err := decodeAs[P](raw)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// register adds an event whose handler touches only its own entity. The
// registry key is P's EventType, so key, payload and handler cannot disagree.
func register[P Payload](b *Bus, apply func(context.Context, store.Tx, *inventory.Event, P) error) {
	var zero P
	b.registry[zero.EventType()] = eventRegistration{
		decode: decodePayload[P],
		applyFn: func(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
			p, err := decodeAs[P](ev.Payload)
			if err != nil {
				return err
			}
			return apply(ctx, tx, ev, p)
		},
	}
}

// registerPathChange adds an event whose handler may move descendants' paths.
func registerPathChange[P Payload](
	b *Bus,
	fn func(context.Context, store.Tx, *inventory.Event, P) (*inventory.Entity, []*inventory.Entity, error),
) {
	var zero P
	b.registry[zero.EventType()] = eventRegistration{
		decode: decodePayload[P],
		pathChangeFn: func(
			ctx context.Context, tx store.Tx, ev *inventory.Event,
		) (*inventory.Entity, []*inventory.Entity, error) {
			p, err := decodeAs[P](ev.Payload)
			if err != nil {
				return nil, nil, err
			}
			return fn(ctx, tx, ev, p)
		},
	}
}
