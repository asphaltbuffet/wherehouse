package eventbus_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestPayload_NamesEventTypeAndSubject(t *testing.T) {
	tests := []struct {
		p    eventbus.Payload
		want inventory.EventType
	}{
		{eventbus.EntityCreatedPayload{EntityID: "x"}, inventory.EntityCreatedEvent},
		{eventbus.EntityRenamedPayload{EntityID: "x"}, inventory.EntityRenamedEvent},
		{eventbus.EntityReparentedPayload{EntityID: "x"}, inventory.EntityReparentedEvent},
		{eventbus.EntityPathChangedPayload{EntityID: "x"}, inventory.EntityPathChangedEvent},
		{eventbus.EntityStatusChangedPayload{EntityID: "x"}, inventory.EntityStatusChangedEvent},
		{eventbus.EntityRemovedPayload{EntityID: "x"}, inventory.EntityRemovedEvent},
		{eventbus.EntityTagAddedPayload{EntityID: "x"}, inventory.EntityTagAddedEvent},
		{eventbus.EntityTagRemovedPayload{EntityID: "x"}, inventory.EntityTagRemovedEvent},
		{eventbus.EntityLockedPayload{EntityID: "x"}, inventory.EntityLockedEvent},
		{eventbus.EntityUnlockedPayload{EntityID: "x"}, inventory.EntityUnlockedEvent},
		{eventbus.EntityDiscreteSetPayload{EntityID: "x"}, inventory.EntityDiscreteSetEvent},
		{eventbus.EntityDiscreteClearedPayload{EntityID: "x"}, inventory.EntityDiscreteClearedEvent},
		{eventbus.EntityBorrowedPayload{EntityID: "x"}, inventory.EntityBorrowedEvent},
		{eventbus.EntityUsedPayload{EntityID: "x"}, inventory.EntityUsedEvent},
		{eventbus.EntityReturnedHomePayload{EntityID: "x"}, inventory.EntityReturnedHomeEvent},
		{eventbus.EntityRehomedPayload{EntityID: "x"}, inventory.EntityRehomedEvent},
	}
	for _, tc := range tests {
		t.Run(tc.want.String(), func(t *testing.T) {
			assert.Equal(t, tc.want, tc.p.EventType())
			assert.Equal(t, "x", tc.p.Subject())
		})
	}
}

// The Status field changed from string to inventory.EntityStatus; the wire
// format must not (ADR 0042).
func TestEntityStatusChangedPayload_JSONUnchanged(t *testing.T) {
	recipient := "Sam"
	got, err := json.Marshal(eventbus.EntityStatusChangedPayload{
		EntityID: "e1", Status: inventory.EntityStatusLoaned, StatusContext: &recipient,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"entity_id":"e1","status":"loaned","status_context":"Sam"}`, string(got))

	var back eventbus.EntityStatusChangedPayload
	require.NoError(t, json.Unmarshal([]byte(`{"entity_id":"e1","status":"missing"}`), &back))
	assert.Equal(t, inventory.EntityStatusMissing, back.Status)
}
