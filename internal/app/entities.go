package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// CreateEntity creates a new entity, resolving ParentPath to a parent entity ID if provided.
func (a *App) CreateEntity(ctx context.Context, req CreateEntityRequest) (EntityResult, error) {
	var parentID *string

	if req.ParentPath != "" {
		parent, err := a.resolveEntityPath(ctx, req.ParentPath)
		if err != nil {
			return EntityResult{}, fmt.Errorf("resolve parent path %q: %w", req.ParentPath, err)
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
		EntityType:  req.EntityType.String(),
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

	if _, err = a.bus.Dispatch(ctx, inventory.EntityCreatedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("create entity: %w", err)
	}

	entity, err := a.store.GetEntity(ctx, entityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get created entity: %w", err)
	}

	return a.entityToResult(ctx, entity)
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

	return a.entityToResult(ctx, updated)
}

// ReparentEntity moves an entity to a new parent, resolved by paths.
func (a *App) ReparentEntity(ctx context.Context, req ReparentEntityRequest) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return EntityResult{}, fmt.Errorf("resolve entity path %q: %w", req.EntityPath, err)
	}

	var newParentID *string
	if req.NewParentPath != "" {
		var parentEntity *inventory.Entity
		parentEntity, err = a.resolveEntityPath(ctx, req.NewParentPath)
		if err != nil {
			return EntityResult{}, fmt.Errorf("resolve new parent path %q: %w", req.NewParentPath, err)
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

	return a.entityToResult(ctx, updated)
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
	return a.entityToResult(ctx, entity)
}

// GetEntityByID retrieves an entity by its stable ID.
// Returns store.ErrNotFound if the entity does not exist; the returned
// EntityResult has HasChildren=false (callers needing it should use ListEntities).
func (a *App) GetEntityByID(ctx context.Context, entityID string) (EntityResult, error) {
	entity, err := a.store.GetEntity(ctx, entityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get entity %q: %w", entityID, err)
	}
	return a.entityToResult(ctx, entity)
}

// ListEntities returns all non-removed entities.
func (a *App) ListEntities(ctx context.Context) ([]EntityResult, error) {
	entities, err := a.store.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	parentIDs := make(map[string]bool, len(entities))
	for _, e := range entities {
		if e.ParentID != nil {
			parentIDs[*e.ParentID] = true
		}
	}

	results := make([]EntityResult, len(entities))
	for i, e := range entities {
		var toResultErr error
		results[i], toResultErr = a.entityToResult(ctx, e)
		if toResultErr != nil {
			return nil, toResultErr
		}
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

	results := make([]EntityResult, len(rows))
	for i, row := range rows {
		var rowErr error
		results[i], rowErr = a.entityToResult(ctx, row.Entity)
		if rowErr != nil {
			return nil, rowErr
		}
		results[i].HasChildren = row.HasChildren
	}
	return results, nil
}

// resolveEntityPath looks up an entity by its colon-separated display path.
// Returns store.ErrNotFound if no match exists.
func (a *App) resolveEntityPath(ctx context.Context, path string) (*inventory.Entity, error) {
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

	candidates, err := a.store.GetEntitiesByCanonicalName(ctx, canonicalLeaf)
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
