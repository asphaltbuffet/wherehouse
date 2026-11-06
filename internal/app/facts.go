package app

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// loadFacts reads a live entity and the relational facts Check needs, all
// through q. Guards pass their write transaction (ADR 0035); EntityActions
// passes the store's Reader.
func (a *App) loadFacts(
	ctx context.Context, q store.Reader, entityID string,
) (*inventory.Entity, EntityFacts, error) {
	e, err := q.GetLiveEntity(ctx, entityID)
	if err != nil {
		return nil, EntityFacts{}, wrapEntityError(entityID, err)
	}
	children, err := q.CountLiveChildren(ctx, e.EntityID)
	if err != nil {
		return nil, EntityFacts{}, fmt.Errorf("load facts %q: %w", e.FullPathDisplay, err)
	}
	homed, err := q.CountLiveDisplacedWithHome(ctx, e.EntityID)
	if err != nil {
		return nil, EntityFacts{}, fmt.Errorf("load facts %q: %w", e.FullPathDisplay, err)
	}
	return e, EntityFacts{
		Path:               e.FullPathDisplay,
		Status:             e.Status,
		Locked:             e.Locked,
		Discrete:           e.Discrete,
		Displaced:          e.IsDisplaced(),
		LiveChildren:       children,
		DisplacedHomedHere: homed,
	}, nil
}

// EntityActions reports which actions the live entity entityID currently
// allows. It is for render decisions only: the in-transaction guard still
// decides every write (ADR 0035, ADR 0040).
func (a *App) EntityActions(ctx context.Context, entityID string) (ActionSet, error) {
	_, facts, err := a.loadFacts(ctx, a.store.Reader(), entityID)
	if err != nil {
		return nil, err
	}
	return ActionsFor(facts), nil
}
