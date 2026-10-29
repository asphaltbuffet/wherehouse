package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// CreateEntity creates a new entity under req.ParentID (root when empty).
func (a *App) CreateEntity(ctx context.Context, req CreateEntityRequest) (EntityResult, error) {
	results, err := a.CreateEntities(ctx, []CreateEntityRequest{req})
	if err != nil {
		return EntityResult{}, err
	}
	return results[0], nil
}

// RenameEntity renames an entity. The liveness check reads through the same
// transaction that writes the event (ADR 0035).
func (a *App) RenameEntity(ctx context.Context, req RenameEntityRequest) (EntityResult, error) {
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		entity, err := a.store.GetLiveEntityTx(ctx, tx, req.EntityID)
		if err != nil {
			return wrapEntityError(req.EntityID, err)
		}

		raw, err := json.Marshal(eventbus.EntityRenamedPayload{
			EntityID:    entity.EntityID,
			DisplayName: req.NewName,
		})
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityRenamedEvent, req.ActorID, raw, note); err != nil {
			return fmt.Errorf("rename entity: %w", err)
		}
		return nil
	}); err != nil {
		return EntityResult{}, err
	}

	// AnyStatus: a concurrent remove may commit between our commit and this read.
	return a.GetEntityByIDAnyStatus(ctx, req.EntityID)
}

// ReparentEntity moves an entity to a new parent. A displaced entity moved to
// where it already is is re-homed in place; moved to its home it is rejected
// (use 'return'). Every guard reads through
// the same transaction that writes the event, so a concurrent remove or a
// crossing move cannot slip between check and write (ADR 0035).
func (a *App) ReparentEntity(ctx context.Context, req ReparentEntityRequest) (EntityResult, error) {
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		entity, err := a.store.GetLiveEntityTx(ctx, tx, req.EntityID)
		if err != nil {
			return wrapEntityError(req.EntityID, err)
		}
		if err = checkMovable(entity, verbMove); err != nil {
			return err
		}
		if entity.IsDisplaced() {
			switch req.NewParentID {
			case idOf(entity.ParentID):
				return a.rehomeTx(ctx, tx, entity, req.ActorID, note)
			case idOf(entity.HomeID):
				return fmt.Errorf("cannot move %q to its home — use 'return' instead: %w: %w",
					entity.FullPathDisplay, ErrDestinationIsHome, ErrInvalidDestination)
			}
		}
		if err = checkNotAlreadyThere(entity, req.NewParentID, verbMove); err != nil {
			return err
		}

		var newParentID *string
		if req.NewParentID != "" {
			if err = a.checkMoveDestinationTx(ctx, tx, entity, req.NewParentID, verbMove); err != nil {
				return err
			}
			newParentID = &req.NewParentID
		}
		return a.reparentTx(ctx, tx, entity.EntityID, newParentID, req.ActorID, note)
	}); err != nil {
		return EntityResult{}, err
	}

	// AnyStatus: a concurrent remove may commit between our commit and this read.
	return a.GetEntityByIDAnyStatus(ctx, req.EntityID)
}

// reparentTx dispatches the reparent event for an already-validated move.
func (a *App) reparentTx(
	ctx context.Context,
	tx store.Tx,
	entityID string,
	newParentID *string,
	actorID string,
	note *string,
) error {
	raw, err := json.Marshal(eventbus.EntityReparentedPayload{EntityID: entityID, NewParentID: newParentID})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityReparentedEvent, actorID, raw, note); err != nil {
		return fmt.Errorf("reparent entity: %w", err)
	}
	return nil
}

// Verbs that name the user's action in rejection copy: move, use and return
// share the destination checks but each says its own word.
const (
	verbMove   = "move"
	verbUse    = "use"
	verbReturn = "return"
)

// checkMovable rejects an entity that may not change location at all
// (ADR 0034): locked, missing, or loaned. verb is the user's action.
func checkMovable(entity *inventory.Entity, verb string) error {
	if entity.Locked {
		return fmt.Errorf("cannot %s %q: entity is locked", verb, entity.FullPathDisplay)
	}
	if entity.Status == inventory.EntityStatusMissing {
		return fmt.Errorf("cannot %s %q: entity is missing — mark it found first", verb, entity.FullPathDisplay)
	}
	if entity.Status == inventory.EntityStatusLoaned {
		return fmt.Errorf("cannot %s %q: entity is loaned — return it first", verb, entity.FullPathDisplay)
	}
	return nil
}

// checkNotAlreadyThere rejects newParentID when it is the current parent.
func checkNotAlreadyThere(entity *inventory.Entity, newParentID, verb string) error {
	if newParentID == idOf(entity.ParentID) {
		return fmt.Errorf("cannot %s %q: already there: %w", verb, entity.FullPathDisplay, ErrInvalidDestination)
	}
	return nil
}

// idOf dereferences an optional entity ID; nil (the root) is "".
func idOf(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

// getParentTx loads parentID as the parent of a new or moved entity. Create,
// borrow and reparent share this rule: the parent must be live (a removed one
// reads as ErrNotFound) and not discrete.
func (a *App) getParentTx(ctx context.Context, tx store.Tx, parentID string) (*inventory.Entity, error) {
	parent, err := a.store.GetLiveEntityTx(ctx, tx, parentID)
	if err != nil {
		return nil, wrapEntityError(parentID, err)
	}
	if parent.Discrete {
		return nil, fmt.Errorf("%q is discrete and cannot hold entities: %w",
			parent.FullPathDisplay, ErrInvalidDestination)
	}
	return parent, nil
}

// checkMoveDestinationTx rejects a non-root destination that fails
// getParentTx, or is the entity itself or one of its descendants.
func (a *App) checkMoveDestinationTx(
	ctx context.Context,
	tx store.Tx,
	entity *inventory.Entity,
	parentID, verb string,
) error {
	parent, err := a.getParentTx(ctx, tx, parentID)
	if err != nil {
		return fmt.Errorf("cannot %s %q: %w", verb, entity.FullPathDisplay, err)
	}
	inside, err := a.isSelfOrDescendant(ctx, tx, entity.EntityID, parent.EntityID)
	if err != nil {
		return err
	}
	if inside {
		return fmt.Errorf("cannot %s %q into itself or its own contents: %w",
			verb, entity.FullPathDisplay, ErrInvalidDestination)
	}
	return a.checkHomeCycleTx(ctx, tx, entity, parent.EntityID, verb)
}

// isSelfOrDescendant reports whether candidateID is entityID or lies beneath it.
// It walks the candidate's ancestors by ID — names are not unique, so path
// prefixes cannot be trusted — and reads removed ancestors too, because a live
// entity may sit under a removed one (#287). A projection already damaged by
// the pre-ADR-0034 cycle bug is reported as an error instead of looping forever.
// It reads through tx so the walk sees the same tree the move will commit to (#301).
func (a *App) isSelfOrDescendant(ctx context.Context, tx store.Tx, entityID, candidateID string) (bool, error) {
	seen := make(map[string]bool)
	for id := candidateID; id != ""; {
		if id == entityID {
			return true, nil
		}
		if seen[id] {
			return false, fmt.Errorf("ancestor chain of %q loops at %q", candidateID, id)
		}
		seen[id] = true
		e, err := a.store.GetEntityTx(ctx, tx, id)
		if err != nil {
			return false, wrapEntityError(id, err)
		}
		id = ""
		if e.ParentID != nil {
			id = *e.ParentID
		}
	}
	return false, nil
}

// RemoveEntity permanently marks an entity as removed. It refuses an entity
// that still contains non-removed entities (#287): they would be left live
// under a removed parent, unreachable from the tree. It also refuses an entity
// that is home to displaced entities (ErrIsHome): they must be returned first.
func (a *App) RemoveEntity(ctx context.Context, req RemoveEntityRequest) error {
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	return a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		entity, err := a.store.GetLiveEntityTx(ctx, tx, req.EntityID)
		if err != nil {
			return wrapEntityError(req.EntityID, err)
		}

		if entity.Status == inventory.EntityStatusBorrowed {
			return fmt.Errorf("cannot remove %q: entity is borrowed — use 'return' instead", entity.FullPathDisplay)
		}

		n, err := a.store.CountLiveChildrenTx(ctx, tx, entity.EntityID)
		if err != nil {
			return fmt.Errorf("remove %q: %w", entity.FullPathDisplay, err)
		}
		if n > 0 {
			return fmt.Errorf("cannot remove %q (%d inside; remove or move them first): %w",
				entity.FullPathDisplay, n, ErrHasChildren)
		}
		if err = a.checkNotHomeTx(ctx, tx, entity, "remove"); err != nil {
			return err
		}

		raw, err := json.Marshal(eventbus.EntityRemovedPayload{EntityID: entity.EntityID})
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityRemovedEvent, req.ActorID, raw, note); err != nil {
			return fmt.Errorf("remove entity: %w", err)
		}
		return nil
	})
}

// LookupEntityStatus returns all entities ever at the given path (any status, including removed),
// ranked by last_event_id DESC (most recent first). Returns ErrNotFound when no entity matches.
func (a *App) LookupEntityStatus(ctx context.Context, path string) ([]EntityResult, error) {
	canonicalPath, canonicalLeaf, err := canonicalizePath(path)
	if err != nil {
		return nil, err
	}

	candidates, err := a.store.GetEntitiesByCanonicalNameAnyStatus(ctx, canonicalLeaf)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w", canonicalLeaf, err)
	}

	var matches []EntityResult
	for _, e := range candidates {
		if e.FullPathCanonical == canonicalPath {
			var r EntityResult
			r, err = a.entityWithTags(ctx, e)
			if err != nil {
				return nil, err
			}
			matches = append(matches, r)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, path)
	}
	return matches, nil
}

// GetEntityByID retrieves an entity by its stable ID.
// Returns ErrNotFound if the entity does not exist; the returned
// EntityResult has HasChildren=false (callers needing it should use ListEntities).
func (a *App) GetEntityByID(ctx context.Context, entityID string) (EntityResult, error) {
	entity, err := a.store.GetEntity(ctx, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return EntityResult{}, fmt.Errorf("get entity %q: %w", entityID, ErrNotFound)
		}
		return EntityResult{}, fmt.Errorf("get entity %q: %w", entityID, err)
	}
	return a.entityWithTags(ctx, entity)
}

// GetEntityByIDAnyStatus retrieves an entity by ID regardless of status, including removed entities.
func (a *App) GetEntityByIDAnyStatus(ctx context.Context, entityID string) (EntityResult, error) {
	entity, err := a.store.GetEntityAnyStatus(ctx, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return EntityResult{}, fmt.Errorf("get entity %q: %w", entityID, ErrNotFound)
		}
		return EntityResult{}, fmt.Errorf("get entity %q: %w", entityID, err)
	}
	return a.entityWithTags(ctx, entity)
}

// ListEntities returns all non-removed entities.
func (a *App) ListEntities(ctx context.Context) ([]EntityResult, error) {
	entities, err := a.store.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	parentIDs := make(map[string]bool, len(entities))
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.EntityID
		if e.ParentID != nil {
			parentIDs[*e.ParentID] = true
		}
	}

	tagsByID, err := a.store.GetTagsByEntities(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list entities tags: %w", err)
	}

	results := make([]EntityResult, len(entities))
	for i, e := range entities {
		results[i] = entityToResult(e, tagsByID[e.EntityID])
		results[i].HasChildren = parentIDs[e.EntityID]
	}
	fillHomePaths(results, pathsByID(entities))
	return results, nil
}

// GetChildren returns direct children of parentID, excluding removed entities.
func (a *App) GetChildren(ctx context.Context, parentID string) ([]EntityResult, error) {
	rows, err := a.store.GetChildren(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("get children of %s: %w", parentID, err)
	}

	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.Entity.EntityID
	}

	tagsByID, err := a.store.GetTagsByEntities(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get children tags: %w", err)
	}

	results := make([]EntityResult, len(rows))
	for i, row := range rows {
		results[i] = entityToResult(row.Entity, tagsByID[row.Entity.EntityID])
		results[i].HasChildren = row.HasChildren
		if results[i], err = a.withHomePath(ctx, results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// HasLiveChildren reports whether parentID has any non-removed direct children.
func (a *App) HasLiveChildren(ctx context.Context, parentID string) (bool, error) {
	n, err := a.store.CountLiveChildren(ctx, parentID)
	if err != nil {
		return false, fmt.Errorf("has live children of %s: %w", parentID, err)
	}
	return n > 0, nil
}

// resolveEntityPath looks up an entity by its colon-separated display path.
// Returns ErrNotFound if no match exists.
func (a *App) resolveEntityPath(ctx context.Context, path string) (*inventory.Entity, error) {
	return a.resolveEntityPathWith(path, func(canonical string) ([]*inventory.Entity, error) {
		return a.store.GetEntitiesByCanonicalName(ctx, canonical)
	})
}

// LookupEntityByPath resolves a colon-separated path to an EntityResult.
// This is the single PathResolution entry point for CLI callers. Web callers
// that already hold an entity_id should pass it directly to request structs
// and skip this call entirely.
func (a *App) LookupEntityByPath(ctx context.Context, path string) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, path)
	if err != nil {
		return EntityResult{}, err
	}
	return a.entityWithTags(ctx, entity)
}

func (a *App) resolveEntityPathTx(ctx context.Context, tx store.Tx, path string) (*inventory.Entity, error) {
	return a.resolveEntityPathWith(path, func(canonical string) ([]*inventory.Entity, error) {
		return a.store.GetEntitiesByCanonicalNameTx(ctx, tx, canonical)
	})
}

// canonicalizePath parses a display path and returns its full canonical path and
// canonical leaf segment. An empty path resolves to ErrNotFound. Both the resolver
// and LookupEntityStatus use this so the two cannot disagree on what a path means.
func canonicalizePath(path string) (string, string, error) {
	p, err := entitypath.Parse(path)
	if err != nil {
		return "", "", fmt.Errorf("parse path %q: %w", path, err)
	}

	segments := p.Segments()
	if len(segments) == 0 {
		return "", "", fmt.Errorf("parse path %q: %w", path, ErrNotFound)
	}

	canonicalSegments := make([]string, len(segments))
	for i, seg := range segments {
		canonicalSegments[i] = inventory.CanonicalizeString(seg)
	}
	cp, err := entitypath.New(canonicalSegments...)
	if err != nil {
		return "", "", fmt.Errorf("build canonical path: %w", err)
	}

	return cp.String(), canonicalSegments[len(canonicalSegments)-1], nil
}

func (a *App) resolveEntityPathWith(
	path string,
	fetch func(canonical string) ([]*inventory.Entity, error),
) (*inventory.Entity, error) {
	canonicalPath, canonicalLeaf, err := canonicalizePath(path)
	if err != nil {
		return nil, err
	}

	candidates, err := fetch(canonicalLeaf)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w", canonicalLeaf, err)
	}

	for _, e := range candidates {
		if e.FullPathCanonical == canonicalPath {
			return e, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", ErrNotFound, path)
}

// CreateEntities creates all requested entities in a single transaction; all succeed or all fail.
func (a *App) CreateEntities(ctx context.Context, reqs []CreateEntityRequest) ([]EntityResult, error) {
	if len(reqs) == 0 {
		return nil, errors.New("CreateEntities: at least one request required")
	}

	results := make([]EntityResult, len(reqs))

	err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		for i, req := range reqs {
			result, err := a.createEntityInTx(ctx, tx, req)
			if err != nil {
				return err
			}
			results[i] = result
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (a *App) createEntityInTx(ctx context.Context, tx store.Tx, req CreateEntityRequest) (EntityResult, error) {
	var parentID *string
	if req.ParentID != "" {
		parent, err := a.getParentTx(ctx, tx, req.ParentID)
		if err != nil {
			return EntityResult{}, err
		}
		parentID = &parent.EntityID
	}

	entityID, err := nanoid.New()
	if err != nil {
		return EntityResult{}, fmt.Errorf("generate entity ID: %w", err)
	}

	payload := eventbus.EntityCreatedPayload{
		EntityID:    entityID,
		DisplayName: req.DisplayName,
		Locked:      req.Locked,
		Discrete:    req.Discrete,
		ParentID:    parentID,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.DispatchInTx(ctx, tx, inventory.EntityCreatedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("create entity %q: %w", req.DisplayName, err)
	}

	entity, err := a.store.GetEntityTx(ctx, tx, entityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get created entity %q: %w", entityID, err)
	}

	return entityToResult(entity, nil), nil
}
