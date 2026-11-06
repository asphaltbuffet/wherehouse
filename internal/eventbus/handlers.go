package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func (b *Bus) handleEntityCreated(ctx context.Context, tx store.Tx, ev *inventory.Event, p EntityCreatedPayload) error {
	locked := p.Locked
	discrete := p.Discrete
	// Legacy migration: old events carry entity_type instead of locked/discrete flags.
	// "place" → locked=true; "leaf" → discrete=true; "container" maps to zero values — no explicit case needed.
	if p.LegacyEntityType == "place" {
		locked = true
	}
	if p.LegacyEntityType == "leaf" {
		discrete = true
	}

	canonicalName := inventory.CanonicalizeString(p.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := tx.ComputeEntityPath(
		ctx,
		p.DisplayName,
		canonicalName,
		p.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: %w", err)
	}

	entity := &inventory.Entity{
		EntityID:          p.EntityID,
		DisplayName:       p.DisplayName,
		CanonicalName:     canonicalName,
		Locked:            locked,
		Discrete:          discrete,
		ParentID:          p.ParentID,
		HomeID:            p.ParentID,
		FullPathDisplay:   fullPathDisplay,
		FullPathCanonical: fullPathCanonical,
		Depth:             depth,
		Status:            inventory.EntityStatusOk,
		LastEventID:       ev.EventID,
		UpdatedAt:         time.Now().UTC(),
	}

	return tx.InsertEntity(ctx, entity)
}

// renameEntityTx is the pathChangeFn for EntityRenamedEvent. Descendants are
// returned whenever the display name changed: descendants' display paths follow
// it even when the canonical name does not (e.g. "Garage" → "garage").
func (b *Bus) renameEntityTx(
	ctx context.Context, tx store.Tx, ev *inventory.Event, p EntityRenamedPayload,
) (*inventory.Entity, []*inventory.Entity, error) {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return nil, nil, fmt.Errorf("handleEntityRenamed: get entity: %w", err)
	}

	oldDisplay := entity.DisplayName

	// Fetch descendants before updating the parent so the path-prefix query still
	// matches the old canonical path stored in the DB.
	descendants, err := tx.GetDescendantsAnyStatus(ctx, p.EntityID)
	if err != nil {
		return nil, nil, fmt.Errorf("handleEntityRenamed: get descendants: %w", err)
	}

	entity.DisplayName = p.DisplayName
	entity.CanonicalName = inventory.CanonicalizeString(p.DisplayName)
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = tx.ComputeEntityPath(
		ctx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("handleEntityRenamed: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err = tx.UpdateEntity(ctx, entity); err != nil {
		return nil, nil, fmt.Errorf("handleEntityRenamed: %w", err)
	}

	if entity.DisplayName == oldDisplay {
		return entity, nil, nil
	}
	return entity, descendants, nil
}

func (b *Bus) handleEntityPathChanged(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	p EntityPathChangedPayload,
) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityPathChanged: get entity: %w", err)
	}

	entity.FullPathDisplay = p.FullPathDisplay
	entity.FullPathCanonical = p.FullPathCanonical
	entity.Depth = p.Depth
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return tx.UpdateEntity(ctx, entity)
}

func (b *Bus) handleEntityStatusChanged(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	p EntityStatusChangedPayload,
) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: get entity: %w", err)
	}

	entity.Status = p.Status
	entity.StatusContext = p.StatusContext
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return tx.UpdateEntity(ctx, entity)
}

func (b *Bus) handleEntityRemoved(ctx context.Context, tx store.Tx, ev *inventory.Event, p EntityRemovedPayload) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRemoved: get entity: %w", err)
	}

	entity.Status = inventory.EntityStatusRemoved
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return tx.UpdateEntity(ctx, entity)
}

func (b *Bus) handleEntityBorrowed(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	p EntityBorrowedPayload,
) error {
	canonicalName := inventory.CanonicalizeString(p.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := tx.ComputeEntityPath(
		ctx, p.DisplayName, canonicalName, p.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityBorrowed: %w", err)
	}

	entity := &inventory.Entity{
		EntityID:          p.EntityID,
		DisplayName:       p.DisplayName,
		CanonicalName:     canonicalName,
		ParentID:          p.ParentID,
		HomeID:            p.ParentID,
		FullPathDisplay:   fullPathDisplay,
		FullPathCanonical: fullPathCanonical,
		Depth:             depth,
		Status:            inventory.EntityStatusBorrowed,
		StatusContext:     p.StatusContext,
		LastEventID:       ev.EventID,
		UpdatedAt:         time.Now().UTC(),
	}

	return tx.InsertEntity(ctx, entity)
}

func (b *Bus) handleEntityTagAdded(
	ctx context.Context,
	tx store.Tx,
	_ *inventory.Event,
	p EntityTagAddedPayload,
) error {
	return tx.InsertTag(ctx, p.EntityID, p.Tag)
}

func (b *Bus) handleEntityTagRemoved(
	ctx context.Context,
	tx store.Tx,
	_ *inventory.Event,
	p EntityTagRemovedPayload,
) error {
	return tx.DeleteTag(ctx, p.EntityID, p.Tag)
}

func (b *Bus) handleEntityLocked(ctx context.Context, tx store.Tx, ev *inventory.Event, p EntityLockedPayload) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityLocked: get entity: %w", err)
	}
	entity.Locked = true
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	return tx.UpdateEntity(ctx, entity)
}

func (b *Bus) handleEntityUnlocked(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	p EntityUnlockedPayload,
) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityUnlocked: get entity: %w", err)
	}
	entity.Locked = false
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	return tx.UpdateEntity(ctx, entity)
}

func (b *Bus) handleEntityDiscreteSet(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	p EntityDiscreteSetPayload,
) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityDiscreteSet: get entity: %w", err)
	}
	entity.Discrete = true
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	return tx.UpdateEntity(ctx, entity)
}

func (b *Bus) handleEntityDiscreteCleared(
	ctx context.Context,
	tx store.Tx,
	ev *inventory.Event,
	p EntityDiscreteClearedPayload,
) error {
	entity, err := tx.GetEntityAnyStatus(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityDiscreteCleared: get entity: %w", err)
	}
	entity.Discrete = false
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()
	return tx.UpdateEntity(ctx, entity)
}

// propagatePathChangesTx emits entity.path_changed derived events for all descendants
// and updates each projection within the same transaction. The derived events carry
// the triggering event's timestamp, so an import reproduces the source rows. It
// returns the payloads it wrote, in descendant order.
//
// descendants must be fetched BEFORE the parent entity is updated in the DB, so that
// path-prefix queries still match the old canonical path. They must be ordered depth ASC
// so grandchildren compute their paths from already-updated parents. An `updated` map
// tracks each entity's new path so grandchildren compute from the already-updated
// parent — not the stale root path.
func (b *Bus) propagatePathChangesTx(
	ctx context.Context,
	tx store.Tx,
	triggeringEv *inventory.Event,
	parent *inventory.Entity,
	descendants []*inventory.Entity,
) ([]EntityPathChangedPayload, error) {
	payloads, err := ComputeDescendantPathPayloads(parent, descendants)
	if err != nil {
		return nil, fmt.Errorf("compute descendant paths: %w", err)
	}

	for i, d := range descendants {
		p := payloads[i]
		d.FullPathDisplay = p.FullPathDisplay
		d.FullPathCanonical = p.FullPathCanonical
		d.Depth = p.Depth
		d.UpdatedAt = time.Now().UTC()

		var payloadJSON []byte
		payloadJSON, err = json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("marshal path_changed payload for %s: %w", d.EntityID, err)
		}

		// Stamp the descendant's own path_changed event, as replay does.
		d.LastEventID, err = tx.AppendEvent(ctx, &inventory.Event{
			EventType:    inventory.EntityPathChangedEvent,
			TimestampUTC: triggeringEv.TimestampUTC,
			ActorUserID:  triggeringEv.ActorUserID,
			Payload:      payloadJSON,
			EntityID:     &d.EntityID,
		})
		if err != nil {
			return nil, fmt.Errorf("insert path_changed event for %s: %w", d.EntityID, err)
		}

		if err = tx.UpdateEntity(ctx, d); err != nil {
			return nil, fmt.Errorf("update descendant %s: %w", d.EntityID, err)
		}
	}

	return payloads, nil
}

// ComputeDescendantPathPayloads computes the new EntityPathChangedPayload for
// each descendant given the already-updated parent. Descendants must be ordered
// parent-before-child (depth ASC). No DB access; safe to call outside a
// transaction.
func ComputeDescendantPathPayloads(
	parent *inventory.Entity,
	descendants []*inventory.Entity,
) ([]EntityPathChangedPayload, error) {
	updated := map[string]*inventory.Entity{parent.EntityID: parent}
	payloads := make([]EntityPathChangedPayload, len(descendants))

	for i, d := range descendants {
		var parentDisplay, parentCanonical string
		var parentDepth int
		if d.ParentID != nil {
			if p, ok := updated[*d.ParentID]; ok {
				parentDisplay = p.FullPathDisplay
				parentCanonical = p.FullPathCanonical
				parentDepth = p.Depth
			}
		}

		computed := *d
		displayPath, err := entitypath.AppendTo(parentDisplay, d.DisplayName)
		if err != nil {
			return nil, fmt.Errorf("build display path for %s: %w", d.EntityID, err)
		}

		canonicalPath, err := entitypath.AppendTo(parentCanonical, d.CanonicalName)
		if err != nil {
			return nil, fmt.Errorf("build canonical path for %s: %w", d.EntityID, err)
		}

		computed.FullPathDisplay = displayPath.String()
		computed.FullPathCanonical = canonicalPath.String()
		computed.Depth = parentDepth + 1

		payloads[i] = EntityPathChangedPayload{
			EntityID:          d.EntityID,
			FullPathDisplay:   computed.FullPathDisplay,
			FullPathCanonical: computed.FullPathCanonical,
			Depth:             computed.Depth,
		}
		updated[d.EntityID] = &computed
	}

	return payloads, nil
}
