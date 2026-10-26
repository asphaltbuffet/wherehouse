package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestMarkLoaned_RejectsBlankRecipient(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
	}{
		{name: "empty recipient", recipient: ""},
		{name: "whitespace recipient", recipient: "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := apptesting.OpenApp(t)
			e, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: "Drill", ActorID: "test"})
			require.NoError(t, err)

			_, err = a.MarkLoaned(t.Context(), []app.ChangeStatusRequest{
				{EntityID: e.EntityID, StatusContext: tt.recipient, ActorID: "test"},
			})
			require.ErrorIs(t, err, app.ErrRecipientRequired)

			got, err := a.GetEntityByID(t.Context(), e.EntityID)
			require.NoError(t, err)
			assert.Equal(t, inventory.EntityStatusOk, got.Status, "nothing may change on validation failure")
		})
	}
}

func TestMarkLoaned_TrimsRecipient(t *testing.T) {
	a := apptesting.OpenApp(t)
	e, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: "Drill", ActorID: "test"})
	require.NoError(t, err)

	_, err = a.MarkLoaned(t.Context(), []app.ChangeStatusRequest{
		{EntityID: e.EntityID, StatusContext: "  Alice  ", ActorID: "test"},
	})
	require.NoError(t, err)

	got, err := a.GetEntityByID(t.Context(), e.EntityID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusLoaned, got.Status)
	assert.Equal(t, "Alice", got.StatusContext)
}
