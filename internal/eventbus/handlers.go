package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func (b *Bus) handleEntityCreated(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityCreated: unmarshal: %w", err)
	}

	entityType, err := inventory.ParseEntityType(p.EntityType)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: %w", err)
	}

	if entityType == inventory.EntityTypePlace && p.ParentID != nil {
		if err = validatePlaceParentTx(ctx, tx, *p.ParentID); err != nil {
			return fmt.Errorf("handleEntityCreated: %w", err)
		}
	}

	canonicalName := inventory.CanonicalizeString(p.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := b.store.ComputeEntityPathTx(
		ctx,
		tx,
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
		EntityType:        entityType,
		ParentID:          p.ParentID,
		FullPathDisplay:   fullPathDisplay,
		FullPathCanonical: fullPathCanonical,
		Depth:             depth,
		Status:            inventory.EntityStatusOk,
		LastEventID:       ev.EventID,
		UpdatedAt:         time.Now().UTC(),
	}

	return b.store.InsertEntityTx(ctx, tx, entity)
}

func (b *Bus) handleEntityRenamed(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityRenamedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityRenamed: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: get entity: %w", err)
	}

	oldCanonical := entity.CanonicalName

	// Fetch descendants before updating the parent so the path-prefix query still
	// matches the old canonical path stored in the DB.
	descendants, err := b.store.GetDescendantsTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: get descendants: %w", err)
	}

	entity.DisplayName = p.DisplayName
	entity.CanonicalName = inventory.CanonicalizeString(p.DisplayName)
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err = b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return fmt.Errorf("handleEntityRenamed: %w", err)
	}

	if entity.CanonicalName != oldCanonical {
		if err = b.propagatePathChangesTx(ctx, tx, ev, entity, descendants); err != nil {
			return fmt.Errorf("handleEntityRenamed: propagate: %w", err)
		}
	}

	return nil
}

func (b *Bus) handleEntityReparented(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityReparentedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityReparented: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: get entity: %w", err)
	}

	// Fetch descendants before updating the parent so the path-prefix query still
	// matches the old canonical path stored in the DB.
	descendants, err := b.store.GetDescendantsTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: get descendants: %w", err)
	}

	entity.ParentID = p.NewParentID
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err = b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return fmt.Errorf("handleEntityReparented: %w", err)
	}

	if err = b.propagatePathChangesTx(ctx, tx, ev, entity, descendants); err != nil {
		return fmt.Errorf("handleEntityReparented: propagate: %w", err)
	}
	return nil
}

// handleEntityReparentedProjectionOnlyTx updates the reparented entity's path in
// the projection without calling propagatePathChangesTx. Used during projection
// rebuilds where EntityPathChangedEvents already in the event log will be applied
// directly, so regenerating them would corrupt the projection and grow the event log.
func (b *Bus) handleEntityReparentedProjectionOnlyTx(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityReparentedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityReparentedProjectionOnlyTx: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityReparentedProjectionOnlyTx: get entity: %w", err)
	}

	entity.ParentID = p.NewParentID
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityReparentedProjectionOnlyTx: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err = b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return fmt.Errorf("handleEntityReparentedProjectionOnlyTx: %w", err)
	}
	return nil
}

// handleEntityReparentedComputePayloadsTx updates the projection for the
// reparented entity and all its descendants (no event writes) and returns the
// expected EntityPathChangedPayload for each descendant. Used by ReplayEvent
// so import can validate export integrity without side-effect event insertion.
func (b *Bus) handleEntityReparentedComputePayloadsTx(ctx context.Context, tx store.Tx, ev *inventory.Event) ([]EntityPathChangedPayload, error) {
	var p EntityReparentedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return nil, fmt.Errorf("handleEntityReparentedComputePayloadsTx: unmarshal: %w", err)
	}

	// Fetch descendants before updating the parent so the path-prefix query
	// still matches the old canonical path stored in the DB.
	descendants, err := b.store.GetDescendantsTx(ctx, tx, p.EntityID)
	if err != nil {
		return nil, fmt.Errorf("handleEntityReparentedComputePayloadsTx: get descendants: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return nil, fmt.Errorf("handleEntityReparentedComputePayloadsTx: get entity: %w", err)
	}

	entity.ParentID = p.NewParentID
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return nil, fmt.Errorf("handleEntityReparentedComputePayloadsTx: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err = b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return nil, fmt.Errorf("handleEntityReparentedComputePayloadsTx: update entity: %w", err)
	}

	payloads := ComputeDescendantPathPayloads(entity, descendants)

	for i, d := range descendants {
		dp := payloads[i]
		d.FullPathDisplay = dp.FullPathDisplay
		d.FullPathCanonical = dp.FullPathCanonical
		d.Depth = dp.Depth
		d.LastEventID = ev.EventID
		d.UpdatedAt = time.Now().UTC()
		if err = b.store.UpdateEntityTx(ctx, tx, d); err != nil {
			return nil, fmt.Errorf("handleEntityReparentedComputePayloadsTx: update descendant %s: %w", d.EntityID, err)
		}
	}

	return payloads, nil
}

func (b *Bus) handleEntityPathChanged(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityPathChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityPathChanged: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityPathChanged: get entity: %w", err)
	}

	entity.FullPathDisplay = p.FullPathDisplay
	entity.FullPathCanonical = p.FullPathCanonical
	entity.Depth = p.Depth
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return b.store.UpdateEntityTx(ctx, tx, entity)
}

func (b *Bus) handleEntityStatusChanged(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityStatusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityStatusChanged: unmarshal: %w", err)
	}

	status, err := inventory.ParseEntityStatus(p.Status)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: get entity: %w", err)
	}

	entity.Status = status
	entity.StatusContext = p.StatusContext
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return b.store.UpdateEntityTx(ctx, tx, entity)
}

func (b *Bus) handleEntityRemoved(ctx context.Context, tx store.Tx, ev *inventory.Event) error {
	var p EntityRemovedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityRemoved: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRemoved: get entity: %w", err)
	}

	entity.Status = inventory.EntityStatusRemoved
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return b.store.UpdateEntityTx(ctx, tx, entity)
}

// propagatePathChangesTx emits entity.path_changed derived events for all descendants
// and updates each projection within the same transaction.
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
) error {
	payloads := ComputeDescendantPathPayloads(parent, descendants)

	for i, d := range descendants {
		p := payloads[i]
		d.FullPathDisplay = p.FullPathDisplay
		d.FullPathCanonical = p.FullPathCanonical
		d.Depth = p.Depth
		d.LastEventID = triggeringEv.EventID
		d.UpdatedAt = time.Now().UTC()

		payloadJSON, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal path_changed payload for %s: %w", d.EntityID, err)
		}

		const q = `
            INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
            VALUES (?, ?, ?, ?, NULL, ?)`
		_, err = tx.ExecContext(ctx, q,
			inventory.EntityPathChangedEvent,
			time.Now().UTC().Format(time.RFC3339),
			triggeringEv.ActorUserID,
			string(payloadJSON),
			d.EntityID,
		)
		if err != nil {
			return fmt.Errorf("insert path_changed event for %s: %w", d.EntityID, err)
		}

		if err = b.store.UpdateEntityTx(ctx, tx, d); err != nil {
			return fmt.Errorf("update descendant %s: %w", d.EntityID, err)
		}
	}

	return nil
}

// ComputeDescendantPathPayloads computes the new EntityPathChangedPayload for
// each descendant given the already-updated parent. Descendants must be ordered
// parent-before-child (depth ASC). No DB access; safe to call outside a
// transaction.
func ComputeDescendantPathPayloads(parent *inventory.Entity, descendants []*inventory.Entity) []EntityPathChangedPayload {
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
		computed.FullPathDisplay = parentDisplay + ":" + d.DisplayName
		computed.FullPathCanonical = parentCanonical + ":" + d.CanonicalName
		computed.Depth = parentDepth + 1

		payloads[i] = EntityPathChangedPayload{
			EntityID:          d.EntityID,
			FullPathDisplay:   computed.FullPathDisplay,
			FullPathCanonical: computed.FullPathCanonical,
			Depth:             computed.Depth,
		}
		updated[d.EntityID] = &computed
	}

	return payloads
}
