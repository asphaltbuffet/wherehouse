package apptesting

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// MustLookupID resolves path to its entity ID, failing the test if it does not
// resolve. An empty path is the root and returns "". Fixtures that seed by
// path use it to fill CreateEntityRequest.ParentID and BorrowEntityRequest.ParentID.
func MustLookupID(t *testing.T, a *app.App, path string) string {
	t.Helper()
	if path == "" {
		return ""
	}
	e, err := a.LookupEntityByPath(t.Context(), path)
	require.NoError(t, err)
	return e.EntityID
}
