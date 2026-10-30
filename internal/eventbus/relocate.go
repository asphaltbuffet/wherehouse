package eventbus

import (
	"context"
	"encoding/json"
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

// decodeRelocation reads the payload of any event for which Relocates() is true.
func decodeRelocation(ev *inventory.Event) (relocation, error) {
	switch ev.EventType { //nolint:exhaustive // only relocating events are decoded here
	case inventory.EntityReparentedEvent:
		var p EntityReparentedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return relocation{}, fmt.Errorf("decode %s: %w", ev.EventType, err)
		}
		return relocation{entityID: p.EntityID, newParentID: p.NewParentID, homeRule: homeFollowsParent}, nil
	case inventory.EntityUsedEvent:
		var p EntityUsedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return relocation{}, fmt.Errorf("decode %s: %w", ev.EventType, err)
		}
		return relocation{entityID: p.EntityID, newParentID: p.NewParentID, homeRule: homeUnchanged}, nil
	case inventory.EntityReturnedHomeEvent:
		var p EntityReturnedHomePayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return relocation{}, fmt.Errorf("decode %s: %w", ev.EventType, err)
		}
		return relocation{entityID: p.EntityID, newParentID: p.ParentID, homeRule: homeUnchanged}, nil
	default:
		return relocation{}, fmt.Errorf("decode relocation: %s does not relocate", ev.EventType)
	}
}

// relocateEntityTx is the pathChangeFn for every relocating event. It updates
// the moved entity's parent, home, and path, and returns it together with its
// descendants as they were before the move. Descendants are read first so the
// path-prefix query still matches the old path.
func (b *Bus) relocateEntityTx(
	ctx context.Context, tx store.Tx, ev *inventory.Event,
) (*inventory.Entity, []*inventory.Entity, error) {
	rel, err := decodeRelocation(ev)
	if err != nil {
		return nil, nil, err
	}
	descendants, err := b.store.GetDescendantsTx(ctx, tx, rel.entityID)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: get descendants: %w", ev.EventType, err)
	}
	entity, err := b.store.GetEntityTx(ctx, tx, rel.entityID)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: get entity: %w", ev.EventType, err)
	}

	entity.ParentID = rel.newParentID
	rel.homeRule(entity)
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: recompute path: %w", ev.EventType, err)
	}
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	if err = b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ev.EventType, err)
	}
	return entity, descendants, nil
}

// handleEntityRehomed sets home to the payload's home_id. The parent does not
// change, so there is nothing to propagate.
func (b *Bus) handleEntityRehomed(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityRehomedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityRehomed: unmarshal: %w", err)
	}
	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRehomed: get entity: %w", err)
	}
	entity.HomeID = p.HomeID
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	if err = b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return fmt.Errorf("handleEntityRehomed: %w", err)
	}
	return nil
}
