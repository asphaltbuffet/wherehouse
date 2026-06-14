package app_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestAppWithPath(t *testing.T) (*app.App, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return app.New(s), path
}

func TestGetInfo_Unnamed(t *testing.T) {
	a, dbPath := openTestAppWithPath(t)
	ctx := context.Background()

	info, err := a.GetInfo(ctx)
	require.NoError(t, err)

	assert.Equal(t, "(unnamed)", info.Name)
	assert.Equal(t, dbPath, info.DatabasePath)
	assert.Equal(t, 0, info.EntityCounts["ok"])
	assert.Equal(t, 0, info.EntityCounts["missing"])
	assert.Equal(t, 0, info.EntityCounts["borrowed"])
	assert.Equal(t, 0, info.EntityCounts["loaned"])
	assert.Equal(t, 0, info.EntityCounts["removed"])
}

func TestGetInfo_WithName(t *testing.T) {
	a, _ := openTestAppWithPath(t)
	ctx := context.Background()

	require.NoError(t, a.SetWherehouseName(ctx, "123 Fake Street"))

	info, err := a.GetInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "123 Fake Street", info.Name)
}

func TestGetInfo_AfterClear(t *testing.T) {
	a, _ := openTestAppWithPath(t)
	ctx := context.Background()

	require.NoError(t, a.SetWherehouseName(ctx, "My House"))
	require.NoError(t, a.ClearWherehouseName(ctx))

	info, err := a.GetInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "(unnamed)", info.Name)
}

func TestGetInfo_EntityCounts(t *testing.T) {
	a, _ := openTestAppWithPath(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)

	info, err := a.GetInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, info.EntityCounts["ok"])
}

func TestSetWherehouseName_Validation(t *testing.T) {
	a, _ := openTestAppWithPath(t)
	ctx := context.Background()

	require.ErrorContains(t, a.SetWherehouseName(ctx, ""), "name cannot be empty")
	require.ErrorContains(t, a.SetWherehouseName(ctx, "bad\nname"), "name cannot contain newlines")
	require.ErrorContains(t, a.SetWherehouseName(ctx, "bad\rname"), "name cannot contain newlines")
	require.ErrorContains(t, a.SetWherehouseName(ctx, strings.Repeat("x", 256)), "name cannot exceed 255 characters")
}

func TestSetWherehouseName_MaxLength(t *testing.T) {
	a, _ := openTestAppWithPath(t)
	ctx := context.Background()

	assert.NoError(t, a.SetWherehouseName(ctx, strings.Repeat("x", 255)))
}
