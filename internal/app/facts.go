package app

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// loadFactsTx reads a live entity and the relational facts Check needs, all
// through q. Guards pass their write transaction (ADR 0035); EntityActions
// passes the store's Reader.
func (a *App) loadFactsTx(
	ctx context.Context, q store.Tx, entityID string,
) (*inventory.Entity, EntityFacts, error) {
	e, err := a.store.GetLiveEntityTx(ctx, q, entityID)
	if err != nil {
		return nil, EntityFacts{}, wrapEntityError(entityID, err)
	}
	children, err := a.store.CountLiveChildrenTx(ctx, q, e.EntityID)
	if err != nil {
		return nil, EntityFacts{}, fmt.Errorf("load facts %q: %w", e.FullPathDisplay, err)
	}
	homed, err := a.store.CountDisplacedWithHomeTx(ctx, q, e.EntityID)
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
	_, facts, err := a.loadFactsTx(ctx, a.store.Reader(), entityID)
	if err != nil {
		return nil, err
	}
	return ActionsFor(facts), nil
}
