package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestEntity is a test helper that appends an EntityCreatedEvent for the
// given entity. parentID may be nil for top-level entities.
func createTestEntity(t *testing.T, db *Database, id, name, entityType string, parentID *string) {
	t.Helper()
	payload := map[string]any{
		"entity_id":    id,
		"display_name": name,
		"entity_type":  entityType,
	}
	if parentID != nil {
		payload["parent_id"] = *parentID
	}
	_, err := db.AppendEvent(context.Background(), EntityCreatedEvent, TestActorUser, payload, "")
	require.NoError(t, err)
}

func TestHandleEntityCreated_TopLevel(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "garage-1", "Garage", "place", nil)

	entity, err := db.GetEntity(ctx, "garage-1")
	require.NoError(t, err)

	assert.Equal(t, "garage-1", entity.EntityID)
	assert.Equal(t, "Garage", entity.DisplayName)
	assert.Equal(t, "garage", entity.CanonicalName)
	assert.Equal(t, EntityTypePlace, entity.EntityType)
	assert.Nil(t, entity.ParentID)
	assert.Equal(t, "Garage", entity.FullPathDisplay)
	assert.Equal(t, "garage", entity.FullPathCanonical)
	assert.Equal(t, 0, entity.Depth)
	assert.Equal(t, EntityStatusOk, entity.Status)
	assert.Nil(t, entity.StatusContext)
}

func TestHandleEntityCreated_Nested(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "parent-1", "Parent", "place", nil)
	parentID := "parent-1"
	createTestEntity(t, db, "child-1", "Child", "container", &parentID)

	child, err := db.GetEntity(ctx, "child-1")
	require.NoError(t, err)

	assert.Equal(t, "Parent::Child", child.FullPathDisplay)
	assert.Equal(t, "parent::child", child.FullPathCanonical)
	assert.Equal(t, 1, child.Depth)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, "parent-1", *child.ParentID)
}

func TestHandleEntityCreated_PlaceUnderContainer_Rejected(t *testing.T) {
	db := NewTestDB(t)

	createTestEntity(t, db, "container-1", "Toolbox", "container", nil)
	containerID := "container-1"

	payload := map[string]any{
		"entity_id":    "place-under-container",
		"display_name": "BadPlace",
		"entity_type":  "place",
		"parent_id":    containerID,
	}
	_, err := db.AppendEvent(context.Background(), EntityCreatedEvent, TestActorUser, payload, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "place entity can only be nested inside another place")
}

func TestHandleEntityReparented_UpdatesPath(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "garage-1", "Garage", "place", nil)
	createTestEntity(t, db, "workshop-1", "Workshop", "place", nil)
	garageID := "garage-1"
	createTestEntity(t, db, "toolbox-1", "Toolbox", "container", &garageID)

	// Reparent Toolbox from Garage to Workshop.
	workshopID := "workshop-1"
	payload := map[string]any{
		"entity_id": "toolbox-1",
		"parent_id": workshopID,
	}
	_, err := db.AppendEvent(ctx, EntityReparentedEvent, TestActorUser, payload, "")
	require.NoError(t, err)

	toolbox, err := db.GetEntity(ctx, "toolbox-1")
	require.NoError(t, err)

	assert.Equal(t, "Workshop::Toolbox", toolbox.FullPathDisplay)
	assert.Equal(t, "workshop::toolbox", toolbox.FullPathCanonical)
	assert.Equal(t, 1, toolbox.Depth)
	require.NotNil(t, toolbox.ParentID)
	assert.Equal(t, "workshop-1", *toolbox.ParentID)
}

func TestHandleEntityReparented_PropagatesPath(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "garage-1", "Garage", "place", nil)
	createTestEntity(t, db, "workshop-1", "Workshop", "place", nil)
	garageID := "garage-1"
	createTestEntity(t, db, "toolbox-1", "Toolbox", "container", &garageID)
	toolboxID := "toolbox-1"
	createTestEntity(t, db, "screwdriver-1", "screwdriver", "leaf", &toolboxID)

	// Reparent Toolbox to Workshop — screwdriver should follow.
	workshopID := "workshop-1"
	payload := map[string]any{
		"entity_id": "toolbox-1",
		"parent_id": workshopID,
	}
	_, err := db.AppendEvent(ctx, EntityReparentedEvent, TestActorUser, payload, "")
	require.NoError(t, err)

	screwdriver, err := db.GetEntity(ctx, "screwdriver-1")
	require.NoError(t, err)

	assert.Equal(t, "Workshop::Toolbox::screwdriver", screwdriver.FullPathDisplay)
	assert.Equal(t, "workshop::toolbox::screwdriver", screwdriver.FullPathCanonical)
	assert.Equal(t, testExpectedDepthLevel2, screwdriver.Depth)
}

func TestHandleEntityReparented_Place_Rejected(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "garage-1", "Garage", "place", nil)
	createTestEntity(t, db, "workshop-1", "Workshop", "place", nil)

	// Attempting to reparent a place entity must fail.
	workshopID := "workshop-1"
	payload := map[string]any{
		"entity_id": "garage-1",
		"parent_id": workshopID,
	}
	_, err := db.AppendEvent(ctx, EntityReparentedEvent, TestActorUser, payload, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "place entities cannot be reparented")
}

func TestHandleEntityRenamed_UpdatesDescendants(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "garage-1", "Garage", "place", nil)
	garageID := "garage-1"
	createTestEntity(t, db, "toolbox-1", "Toolbox", "container", &garageID)
	toolboxID := "toolbox-1"
	createTestEntity(t, db, "screwdriver-1", "screwdriver", "leaf", &toolboxID)

	// Rename Toolbox.
	payload := map[string]any{
		"entity_id":    "toolbox-1",
		"display_name": "Big Toolbox",
	}
	_, err := db.AppendEvent(ctx, EntityRenamedEvent, TestActorUser, payload, "")
	require.NoError(t, err)

	toolbox, err := db.GetEntity(ctx, "toolbox-1")
	require.NoError(t, err)
	assert.Equal(t, "Big Toolbox", toolbox.DisplayName)
	assert.Equal(t, "big_toolbox", toolbox.CanonicalName)
	assert.Equal(t, "Garage::Big Toolbox", toolbox.FullPathDisplay)

	screwdriver, err := db.GetEntity(ctx, "screwdriver-1")
	require.NoError(t, err)
	assert.Equal(t, "Garage::Big Toolbox::screwdriver", screwdriver.FullPathDisplay)
	assert.Equal(t, "garage::big_toolbox::screwdriver", screwdriver.FullPathCanonical)
}

func TestHandleEntityStatusChanged(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "wrench-1", "Wrench", "leaf", nil)

	ctx2 := "lent to Bob"
	payload := map[string]any{
		"entity_id":      "wrench-1",
		"status":         "loaned",
		"status_context": ctx2,
	}
	_, err := db.AppendEvent(ctx, EntityStatusChangedEvent, TestActorUser, payload, "")
	require.NoError(t, err)

	entity, err := db.GetEntity(ctx, "wrench-1")
	require.NoError(t, err)
	assert.Equal(t, EntityStatusLoaned, entity.Status)
	require.NotNil(t, entity.StatusContext)
	assert.Equal(t, "lent to Bob", *entity.StatusContext)
}

func TestHandleEntityRemoved_BlockedByChildren(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "parent-1", "Parent", "place", nil)
	parentID := "parent-1"
	createTestEntity(t, db, "child-1", "Child", "container", &parentID)

	payload := map[string]any{"entity_id": "parent-1"}
	_, err := db.AppendEvent(ctx, EntityRemovedEvent, TestActorUser, payload, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-removed children")
}

func TestHandleEntityRemoved_Success(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "orphan-1", "Orphan", "leaf", nil)

	payload := map[string]any{"entity_id": "orphan-1"}
	_, err := db.AppendEvent(ctx, EntityRemovedEvent, TestActorUser, payload, "")
	require.NoError(t, err)

	entity, err := db.GetEntity(ctx, "orphan-1")
	require.NoError(t, err)
	assert.Equal(t, EntityStatusRemoved, entity.Status)
	assert.Nil(t, entity.StatusContext)
}

func TestHandleEntityPathChanged_UpdatesProjection(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "ent-001", "Garage", "place", nil)

	// Fire a path_changed event directly (as if triggered by a parent rename).
	payload := map[string]any{
		"entity_id":           "ent-001",
		"full_path_display":   "NewGarage",
		"full_path_canonical": "newgarage",
		"depth":               0,
	}
	_, err := db.AppendEvent(ctx, EntityPathChangedEvent, TestActorUser, payload, "")
	require.NoError(t, err)

	e, err := db.GetEntity(ctx, "ent-001")
	require.NoError(t, err)
	assert.Equal(t, "NewGarage", e.FullPathDisplay)
	assert.Equal(t, "newgarage", e.FullPathCanonical)
}

func TestHandleEntityStatusChanged_InvalidStatus_Rejected(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	createTestEntity(t, db, "ent-001", "hammer", "container", nil)

	// Attempt to set an invalid status.
	payload := map[string]any{
		"entity_id": "ent-001",
		"status":    "broken", // invalid
	}
	_, err := db.AppendEvent(ctx, EntityStatusChangedEvent, TestActorUser, payload, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
}
