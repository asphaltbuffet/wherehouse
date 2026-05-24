package eventbus_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDispatch_UnknownEventType(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	payload := json.RawMessage(`{}`)
	_, err := b.Dispatch(context.Background(), inventory.EventType(999), "alice", payload, nil)
	assert.Error(t, err)
}
