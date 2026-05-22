package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestMetadataRoundtrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetMetadata(ctx, "schema_version", "1"))

	val, err := s.GetMetadata(ctx, "schema_version")
	require.NoError(t, err)
	assert.Equal(t, "1", val)
}

func TestGetMetadata_Missing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetMetadata(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}
