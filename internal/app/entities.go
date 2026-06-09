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

// CreateEntity creates a new entity, resolving ParentPath to a parent entity ID if provided.
func (a *App) CreateEntity(ctx context.Context, req CreateEntityRequest) (EntityResult, error) {
	results, err := a.CreateEntities(ctx, []CreateEntityRequest{req})
	if err != nil {
		return EntityResult{}, err
	}
	return results[0], nil
}

// RenameEntity renames an entity resolved by its current path.
func (a *App) RenameEntity(ctx context.Context, req RenameEntityRequest) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return EntityResult{}, fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	payload := eventbus.EntityRenamedPayload{
		EntityID:    entity.EntityID,
		DisplayName: req.NewName,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.Dispatch(ctx, inventory.EntityRenamedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("rename entity: %w", err)
	}

	updated, err := a.store.GetEntity(ctx, entity.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get renamed entity: %w", err)
	}

	return a.entityWithTags(ctx, updated)
}

// ReparentEntity moves an entity to a new parent, resolved by paths.
func (a *App) ReparentEntity(ctx context.Context, req ReparentEntityRequest) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return EntityResult{}, fmt.Errorf("resolve entity path %q: %w", req.EntityPath, err)
	}

	if entity.Locked {
		return EntityResult{}, fmt.Errorf("cannot move %q: entity is locked", entity.FullPathDisplay)
	}

	var newParentID *string
	if req.NewParentPath != "" {
		var parentEntity *inventory.Entity
		parentEntity, err = a.resolveEntityPath(ctx, req.NewParentPath)
		if err != nil {
			return EntityResult{}, fmt.Errorf("resolve new parent path %q: %w", req.NewParentPath, err)
		}
		if parentEntity.Discrete {
			return EntityResult{}, fmt.Errorf("cannot move into %q: entity is discrete", parentEntity.FullPathDisplay)
		}
		newParentID = &parentEntity.EntityID
	}

	payload := eventbus.EntityReparentedPayload{
		EntityID:    entity.EntityID,
		NewParentID: newParentID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.Dispatch(ctx, inventory.EntityReparentedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("reparent entity: %w", err)
	}

	updated, err := a.store.GetEntity(ctx, entity.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get reparented entity: %w", err)
	}

	return a.entityWithTags(ctx, updated)
}

// RemoveEntity permanently marks an entity as removed.
func (a *App) RemoveEntity(ctx context.Context, req RemoveEntityRequest) error {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	payload := eventbus.EntityRemovedPayload{EntityID: entity.EntityID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err = a.bus.Dispatch(ctx, inventory.EntityRemovedEvent, req.ActorID, raw, note); err != nil {
		return fmt.Errorf("remove entity: %w", err)
	}

	return nil
}

// GetEntityByPath retrieves an entity by its colon-separated display path.
func (a *App) GetEntityByPath(ctx context.Context, path string) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, path)
	if err != nil {
		return EntityResult{}, err
	}
	return a.entityWithTags(ctx, entity)
}

// GetEntityByID retrieves an entity by its stable ID.
// Returns store.ErrNotFound if the entity does not exist; the returned
// EntityResult has HasChildren=false (callers needing it should use ListEntities).
func (a *App) GetEntityByID(ctx context.Context, entityID string) (EntityResult, error) {
	entity, err := a.store.GetEntity(ctx, entityID)
	if err != nil {
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
	}
	return results, nil
}

// resolveEntityPath looks up an entity by its colon-separated display path.
// Returns store.ErrNotFound if no match exists.
func (a *App) resolveEntityPath(ctx context.Context, path string) (*inventory.Entity, error) {
	return a.resolveEntityPathWith(path, func(canonical string) ([]*inventory.Entity, error) {
		return a.store.GetEntitiesByCanonicalName(ctx, canonical)
	})
}

func (a *App) resolveEntityPathTx(ctx context.Context, tx store.Tx, path string) (*inventory.Entity, error) {
	return a.resolveEntityPathWith(path, func(canonical string) ([]*inventory.Entity, error) {
		return a.store.GetEntitiesByCanonicalNameTx(ctx, tx, canonical)
	})
}

func (a *App) resolveEntityPathWith(
	path string,
	fetch func(canonical string) ([]*inventory.Entity, error),
) (*inventory.Entity, error) {
	p, err := entitypath.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}

	segments := p.Segments()
	if len(segments) == 0 {
		return nil, fmt.Errorf("parse path %q: %w", path, store.ErrNotFound)
	}

	canonicalSegments := make([]string, len(segments))
	for i, seg := range segments {
		canonicalSegments[i] = inventory.CanonicalizeString(seg)
	}
	p2, err := entitypath.New(canonicalSegments...)
	if err != nil {
		return nil, fmt.Errorf("build canonical path: %w", err)
	}

	canonicalPath := p2.String()
	canonicalLeaf := canonicalSegments[len(canonicalSegments)-1]

	candidates, err := fetch(canonicalLeaf)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w", canonicalLeaf, err)
	}

	for _, e := range candidates {
		if e.FullPathCanonical == canonicalPath {
			return e, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", store.ErrNotFound, path)
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

	if req.ParentPath != "" {
		parent, err := a.resolveEntityPathTx(ctx, tx, req.ParentPath)
		if err != nil {
			return EntityResult{}, fmt.Errorf("resolve parent path %q: %w", req.ParentPath, err)
		}
		if parent.Discrete {
			return EntityResult{}, fmt.Errorf("cannot add child to %q: entity is discrete", parent.FullPathDisplay)
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
