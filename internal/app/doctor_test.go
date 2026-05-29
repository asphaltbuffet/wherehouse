package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestAppWithStore(t *testing.T) (*app.App, *store.Store) {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return app.New(s), s
}

func TestValidateEventLog_CleanLog(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	issues, err := a.ValidateEventLog(ctx)
	require.NoError(t, err)
	assert.NotNil(t, issues)
	assert.Empty(t, issues)
}

func TestValidateEventLog_UnknownEventType(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx := context.Background()

	entityID := "e1"
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"entity.unknown_future", "2026-01-01T00:00:00Z", "alice", `{}`, &entityID,
	)
	require.NoError(t, err)

	issues, err := a.ValidateEventLog(ctx)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, app.DoctorKindEventLog, issues[0].Kind)
	assert.NotNil(t, issues[0].EventID)
	assert.Contains(t, issues[0].Description, "unknown event type")
}

func TestValidateEventLog_MalformedPayload(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx := context.Background()

	entityID := "e1"
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"entity.created", "2026-01-01T00:00:00Z", "alice", `not-json`, &entityID,
	)
	require.NoError(t, err)

	issues, err := a.ValidateEventLog(ctx)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, app.DoctorKindEventLog, issues[0].Kind)
	assert.NotNil(t, issues[0].EventID)
	assert.Contains(t, issues[0].Description, "payload")
}

func TestValidateEventLog_MissingEntityID(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx := context.Background()

	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, NULL)`,
		"entity.renamed", "2026-01-01T00:00:00Z", "alice",
		`{"entity_id":"e1","display_name":"Garage"}`,
	)
	require.NoError(t, err)

	issues, err := a.ValidateEventLog(ctx)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, app.DoctorKindEventLog, issues[0].Kind)
	assert.NotNil(t, issues[0].EventID)
	assert.Contains(t, issues[0].Description, "entity_id")
}

func TestValidateEventLog_EntityCreated_MissingDisplayName(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx := context.Background()

	entityID := "e1"
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"entity.created", "2026-01-01T00:00:00Z", "alice",
		`{"entity_id":"e1","display_name":"","entity_type":"place"}`, &entityID,
	)
	require.NoError(t, err)

	issues, err := a.ValidateEventLog(ctx)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, app.DoctorKindEventLog, issues[0].Kind)
	assert.Contains(t, issues[0].Description, "display_name")
}

func TestValidateEventLog_EntityCreated_InvalidEntityType(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx := context.Background()

	entityID := "e1"
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"entity.created", "2026-01-01T00:00:00Z", "alice",
		`{"entity_id":"e1","display_name":"Garage","entity_type":"not_a_type"}`, &entityID,
	)
	require.NoError(t, err)

	issues, err := a.ValidateEventLog(ctx)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, app.DoctorKindEventLog, issues[0].Kind)
	assert.Contains(t, issues[0].Description, "entity_type")
}
