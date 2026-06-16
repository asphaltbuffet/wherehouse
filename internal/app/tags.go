package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/logging"
)

// TagEntity adds and/or removes tags on the entity identified by req.EntityID.
// Tags in both Add and Remove cancel each other out (a warning is logged).
// Adding an existing tag or removing a missing tag are no-ops.
func (a *App) TagEntity(ctx context.Context, req TagEntityRequest) error {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return wrapEntityError(req.EntityID, err)
	}

	addSet := make(map[string]bool, len(req.Add))
	for _, t := range req.Add {
		addSet[inventory.CanonicalizeString(t)] = true
	}
	removeSet := make(map[string]bool, len(req.Remove))
	for _, t := range req.Remove {
		removeSet[inventory.CanonicalizeString(t)] = true
	}

	// Cancel tags that appear in both sets.
	for tag := range addSet {
		if removeSet[tag] {
			logging.Warn("tag appears in both --add and --remove, skipping", "tag", tag)
			delete(addSet, tag)
			delete(removeSet, tag)
		}
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	// Each tag mutation is dispatched as a separate event in its own transaction.
	// This is intentional: each event is self-describing and independently replayable.
	// A crash mid-loop leaves the projection in a partial state, but TruncateAndReplay
	// will reconstruct the correct state from the event stream.
	// Removals first to avoid spurious duplicate-tag warnings.
	for tag := range removeSet {
		payload := eventbus.EntityTagRemovedPayload{EntityID: entity.EntityID, Tag: tag}
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("marshal tag_removed payload: %w", marshalErr)
		}
		if _, dispErr := a.bus.Dispatch(ctx, inventory.EntityTagRemovedEvent, req.ActorID, raw, note); dispErr != nil {
			return fmt.Errorf("remove tag %q: %w", tag, dispErr)
		}
	}

	for tag := range addSet {
		payload := eventbus.EntityTagAddedPayload{EntityID: entity.EntityID, Tag: tag}
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("marshal tag_added payload: %w", marshalErr)
		}
		if _, dispErr := a.bus.Dispatch(ctx, inventory.EntityTagAddedEvent, req.ActorID, raw, note); dispErr != nil {
			return fmt.Errorf("add tag %q: %w", tag, dispErr)
		}
	}

	return nil
}

// ListTags returns the canonical tags for the entity identified by req.EntityID, sorted alphabetically.
func (a *App) ListTags(ctx context.Context, req ListTagsRequest) ([]string, error) {
	entity, err := a.store.GetEntity(ctx, req.EntityID)
	if err != nil {
		return nil, wrapEntityError(req.EntityID, err)
	}
	tags, err := a.store.GetTagsByEntity(ctx, entity.EntityID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}
