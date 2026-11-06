package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// relocation is a decoded parent change. homeRule decides the entity's home
// afterwards: a move re-homes, a use or return-home leaves home alone.
type relocation struct {
	entityID    string
	newParentID *string
	homeRule    func(*inventory.Entity)
}

func homeFollowsParent(e *inventory.Entity) { e.HomeID = e.ParentID }

func homeUnchanged(*inventory.Entity) {}

// relocator is a payload for an event where Relocates() is true.
type relocator interface {
	Payload
	relocation() relocation
}

func (p EntityReparentedPayload) relocation() relocation {
	return relocation{entityID: p.EntityID, newParentID: p.NewParentID, homeRule: homeFollowsParent}
}

func (p EntityUsedPayload) relocation() relocation {
	return relocation{entityID: p.EntityID, newParentID: p.NewParentID, homeRule: homeUnchanged}
}

func (p EntityReturnedHomePayload) relocation() relocation {
	return relocation{entityID: p.EntityID, newParentID: p.ParentID, homeRule: homeUnchanged}
}

// registerRelocation routes a relocating event through relocateEntityTx.
func registerRelocation[P relocator](b *Bus) {
	registerPathChange(b, func(
		ctx context.Context, tx store.Tx, ev *inventory.Event, p P,
	) (*inventory.Entity, []*inventory.Entity, error) {
		return b.relocateEntityTx(ctx, tx, ev, p.relocation())
	})
}

// relocateEntityTx is the pathChangeFn for every relocating event. It updates
// the moved entity's parent, home, and path, and returns it together with its
// descendants as they were before the move. Descendants are read first so the
// path-prefix query still matches the old path.
func (b *Bus) relocateEntityTx(
	ctx context.Context, tx store.Tx, ev *inventory.Event, rel relocation,
) (*inventory.Entity, []*inventory.Entity, error) {
	descendants, err := tx.GetDescendantsAnyStatus(ctx, rel.entityID)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: get descendants: %w", ev.EventType, err)
	}
	entity, err := tx.GetEntityAnyStatus(ctx, rel.entityID)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: get entity: %w", ev.EventType, err)
	}

	entity.ParentID = rel.newParentID
	rel.homeRule(entity)
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = tx.ComputeEntityPath(
		ctx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: recompute path: %w", ev.EventType, err)
	}
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	if err = tx.UpdateEntity(ctx, entity); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ev.EventType, err)
	}
	return entity, descendants, nil
}

// handleEntityRehomed sets home to the payload's home_id. The parent does not
// change, so there is nothing to propagate.
func (b *Bus) handleEntityRehomed(ctx context.Context, tx store.Tx, ev *inventory.Event, p EntityRehomedPayload) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRehomed: get entity: %w", err)
	}
	entity.HomeID = p.HomeID
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	if err = tx.UpdateEntity(ctx, entity); err != nil {
		return fmt.Errorf("handleEntityRehomed: %w", err)
	}
	return nil
}
