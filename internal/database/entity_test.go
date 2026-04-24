package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertTestEntity inserts a row directly into entities_current for testing.
func insertTestEntity(
	t *testing.T,
	db *Database,
	id, displayName, canonName, entityType, status string,
	parentID *string,
	depth int,
	pathDisplay, pathCanon string,
) {
	t.Helper()

	_, err := db.DB().ExecContext(context.Background(), `
		INSERT INTO entities_current
		(entity_id, display_name, canonical_name, entity_type, parent_id,
		 full_path_display, full_path_canonical, depth, status, status_context,
		 last_event_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, 1, ?)
	`, id, displayName, canonName, entityType, parentID, pathDisplay, pathCanon, depth, status, time.Now().UTC().Format(time.RFC3339))
	require.NoError(t, err)
}

func TestGetEntity_NotFound(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	_, err := db.GetEntity(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEntityNotFound, "expected ErrEntityNotFound, got %v", err)
}

func TestGetEntity_Found(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertTestEntity(t, db, "ent-001", "Workshop", "workshop", "place", "ok", nil, 0, "Workshop", "workshop")

	entity, err := db.GetEntity(ctx, "ent-001")
	require.NoError(t, err)
	require.NotNil(t, entity)

	assert.Equal(t, "ent-001", entity.EntityID)
	assert.Equal(t, "Workshop", entity.DisplayName)
	assert.Equal(t, "workshop", entity.CanonicalName)
	assert.Equal(t, EntityTypePlace, entity.EntityType)
	assert.Nil(t, entity.ParentID)
	assert.Equal(t, "Workshop", entity.FullPathDisplay)
	assert.Equal(t, "workshop", entity.FullPathCanonical)
	assert.Equal(t, 0, entity.Depth)
	assert.Equal(t, EntityStatusOk, entity.Status)
	assert.Nil(t, entity.StatusContext)
	assert.Equal(t, int64(1), entity.LastEventID)
	assert.False(t, entity.UpdatedAt.IsZero())
}

func TestGetEntitiesByCanonicalName_Empty(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	entities, err := db.GetEntitiesByCanonicalName(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, entities)
}

func TestGetEntitiesByCanonicalName_MultipleMatches(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// Two entities with same canonical_name in different parents
	insertTestEntity(t, db, "ent-001", "Shelf", "shelf", "container", "ok", nil, 0, "Shelf", "shelf")
	insertTestEntity(t, db, "ent-002", "Shelf", "shelf", "container", "ok", nil, 0, "Shelf2", "shelf2")

	entities, err := db.GetEntitiesByCanonicalName(ctx, "shelf")
	require.NoError(t, err)
	assert.Len(t, entities, 2)
}

func TestGetDescendants_ReturnsChildren(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	parent := "ent-parent"
	child1 := "ent-child1"
	child2 := "ent-child2"

	insertTestEntity(t, db, parent, "Garage", "garage", "place", "ok", nil, 0, "Garage", "garage")
	insertTestEntity(
		t,
		db,
		child1,
		"Top Shelf",
		"top-shelf",
		"container",
		"ok",
		&parent,
		1,
		"Garage::Top Shelf",
		"garage::top-shelf",
	)
	insertTestEntity(
		t,
		db,
		child2,
		"Bottom Shelf",
		"bottom-shelf",
		"container",
		"ok",
		&parent,
		1,
		"Garage::Bottom Shelf",
		"garage::bottom-shelf",
	)

	descendants, err := db.GetDescendants(ctx, parent)
	require.NoError(t, err)
	require.Len(t, descendants, 2)

	ids := []string{descendants[0].EntityID, descendants[1].EntityID}
	assert.Contains(t, ids, child1)
	assert.Contains(t, ids, child2)
}

func TestListEntities_FilterByType(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertTestEntity(t, db, "ent-001", "Workshop", "workshop", "place", "ok", nil, 0, "Workshop", "workshop")
	insertTestEntity(t, db, "ent-002", "Toolbox", "toolbox", "container", "ok", nil, 0, "Toolbox", "toolbox")
	insertTestEntity(t, db, "ent-003", "Hammer", "hammer", "leaf", "ok", nil, 0, "Hammer", "hammer")

	places, err := db.ListEntities(ctx, "", "place", "")
	require.NoError(t, err)
	require.Len(t, places, 1)
	assert.Equal(t, "ent-001", places[0].EntityID)

	containers, err := db.ListEntities(ctx, "", "container", "")
	require.NoError(t, err)
	require.Len(t, containers, 1)
	assert.Equal(t, "ent-002", containers[0].EntityID)
}

func TestListEntities_FilterByStatus(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertTestEntity(t, db, "ent-001", "Hammer", "hammer", "leaf", "ok", nil, 0, "Hammer", "hammer")
	insertTestEntity(t, db, "ent-002", "Wrench", "wrench", "leaf", "missing", nil, 0, "Wrench", "wrench")
	insertTestEntity(t, db, "ent-003", "Drill", "drill", "leaf", "borrowed", nil, 0, "Drill", "drill")

	missing, err := db.ListEntities(ctx, "", "", "missing")
	require.NoError(t, err)
	require.Len(t, missing, 1)
	assert.Equal(t, "ent-002", missing[0].EntityID)

	ok, err := db.ListEntities(ctx, "", "", "ok")
	require.NoError(t, err)
	require.Len(t, ok, 1)
	assert.Equal(t, "ent-001", ok[0].EntityID)
}

func TestComputeEntityPathTx_NilParent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	var gotDisplay, gotCanonical string
	var gotDepth int

	err := db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		var txErr error
		gotDisplay, gotCanonical, gotDepth, txErr = db.ComputeEntityPathTx(ctx, tx, "Garage", "garage", nil)
		return txErr
	})
	require.NoError(t, err)

	assert.Equal(t, "Garage", gotDisplay)
	assert.Equal(t, "garage", gotCanonical)
	assert.Equal(t, 0, gotDepth)
}

func TestComputeEntityPathTx_WithParent(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	parentID := "ent-parent"
	insertTestEntity(t, db, parentID, "Garage", "garage", "place", "ok", nil, 0, "Garage", "garage")

	var gotDisplay, gotCanonical string
	var gotDepth int

	err := db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		var txErr error
		gotDisplay, gotCanonical, gotDepth, txErr = db.ComputeEntityPathTx(ctx, tx, "Top Shelf", "top-shelf", &parentID)
		return txErr
	})
	require.NoError(t, err)

	assert.Equal(t, "Garage::Top Shelf", gotDisplay)
	assert.Equal(t, "garage::top-shelf", gotCanonical)
	assert.Equal(t, 1, gotDepth)
}
