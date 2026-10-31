package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/logging"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// TagEntity adds and/or removes tags on the entity identified by req.EntityID.
// Tags in both Add and Remove cancel each other out (a warning is logged).
// Adding an existing tag or removing a missing tag are no-ops. An Add tag
// whose canonical form is empty fails the whole call with ErrEmptyTag before
// anything is written; an empty Remove tag is ignored.
func (a *App) TagEntity(ctx context.Context, req TagEntityRequest) error {
	addSet, removeSet, setsErr := tagSets(req)
	if setsErr != nil {
		return setsErr
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	// One transaction for the whole call, checked against the live entity
	// inside it (ADR 0035): either every tag change commits or none does.
	// Each change is still its own event, so replay is unaffected.
	// Removals first to avoid spurious duplicate-tag warnings.
	return a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		entity, err := a.store.GetLiveEntityTx(ctx, tx, req.EntityID)
		if err != nil {
			return wrapEntityError(req.EntityID, err)
		}

		for tag := range removeSet {
			raw, marshalErr := json.Marshal(eventbus.EntityTagRemovedPayload{EntityID: entity.EntityID, Tag: tag})
			if marshalErr != nil {
				return fmt.Errorf("marshal tag_removed payload: %w", marshalErr)
			}
			if _, dispErr := a.bus.DispatchInTx(
				ctx, tx, inventory.EntityTagRemovedEvent, req.ActorID, raw, note,
			); dispErr != nil {
				return fmt.Errorf("remove tag %q: %w", tag, dispErr)
			}
		}

		for tag := range addSet {
			raw, marshalErr := json.Marshal(eventbus.EntityTagAddedPayload{EntityID: entity.EntityID, Tag: tag})
			if marshalErr != nil {
				return fmt.Errorf("marshal tag_added payload: %w", marshalErr)
			}
			if _, dispErr := a.bus.DispatchInTx(
				ctx, tx, inventory.EntityTagAddedEvent, req.ActorID, raw, note,
			); dispErr != nil {
				return fmt.Errorf("add tag %q: %w", tag, dispErr)
			}
		}
		return nil
	})
}

// tagSets canonicalizes req's Add and Remove tags. A tag in both sets cancels
// out of both (a warning is logged). An Add tag with an empty canonical form
// is ErrEmptyTag; an empty Remove tag is dropped.
func tagSets(req TagEntityRequest) (map[string]bool, map[string]bool, error) {
	addSet := make(map[string]bool, len(req.Add))
	for _, t := range req.Add {
		c := inventory.CanonicalizeString(t)
		if c == "" {
			return nil, nil, fmt.Errorf("add tag %q: %w", t, ErrEmptyTag)
		}
		addSet[c] = true
	}
	removeSet := make(map[string]bool, len(req.Remove))
	for _, t := range req.Remove {
		if c := inventory.CanonicalizeString(t); c != "" {
			removeSet[c] = true
		}
	}

	for tag := range addSet {
		if removeSet[tag] {
			logging.Warn("tag appears in both --add and --remove, skipping", "tag", tag)
			delete(addSet, tag)
			delete(removeSet, tag)
		}
	}
	return addSet, removeSet, nil
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

// AllTags returns the tag vocabulary: every distinct tag in the inventory,
// including tags on removed entities, sorted alphabetically.
func (a *App) AllTags(ctx context.Context) ([]string, error) {
	tags, err := a.store.GetDistinctTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("all tags: %w", err)
	}
	return tags, nil
}
