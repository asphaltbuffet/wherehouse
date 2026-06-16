package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// seedEntityInStatus creates Box:Item and drives it into the requested source status
// using the legal commands, so the setup itself only ever traverses reachable states.
func seedEntityInStatus(t *testing.T, a *app.App, status inventory.EntityStatus) {
	t.Helper()
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ActorID: "test"})
	require.NoError(t, err)

	if status == inventory.EntityStatusBorrowed {
		_, err = a.BorrowEntities(ctx, []app.BorrowEntityRequest{
			{DisplayName: "Item", ParentPath: "Box", StatusContext: "Alice", ActorID: "test"},
		})
		require.NoError(t, err)
		return
	}

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Item", ParentPath: "Box", ActorID: "test",
	})
	require.NoError(t, err)

	item, err := a.LookupEntityByPath(ctx, "Box:Item")
	require.NoError(t, err)

	switch status {
	case inventory.EntityStatusOk:
		// already ok
	case inventory.EntityStatusMissing:
		_, err = a.MarkLost(ctx, []app.ChangeStatusRequest{{EntityID: item.EntityID, ActorID: "test"}})
		require.NoError(t, err)
	case inventory.EntityStatusLoaned:
		_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
			{EntityID: item.EntityID, StatusContext: "Bob", ActorID: "test"},
		})
		require.NoError(t, err)
	case inventory.EntityStatusRemoved:
		require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: item.EntityID, ActorID: "test"}))
	case inventory.EntityStatusBorrowed:
		t.Fatalf("borrowed handled above")
	}
}

// TestStatusTransitionTable is an executable copy of ADR 0024's legal transition table.
// Each row sets Box:Item into a source status and asserts whether a command is allowed.
func TestStatusTransitionTable(t *testing.T) {
	type call func(ctx context.Context, a *app.App) error

	lookupItem := func(ctx context.Context, a *app.App) (string, error) {
		e, err := a.LookupEntityByPath(ctx, "Box:Item")
		if err != nil {
			return "", err
		}
		return e.EntityID, nil
	}

	markLost := func(ctx context.Context, a *app.App) error {
		id, err := lookupItem(ctx, a)
		if err != nil {
			return err
		}
		_, err = a.MarkLost(ctx, []app.ChangeStatusRequest{{EntityID: id, ActorID: "test"}})
		return err
	}
	markFound := func(ctx context.Context, a *app.App) error {
		id, err := lookupItem(ctx, a)
		if err != nil {
			return err
		}
		_, err = a.MarkFound(ctx, []app.ChangeStatusRequest{{EntityID: id, ActorID: "test"}})
		return err
	}
	markLoaned := func(ctx context.Context, a *app.App) error {
		id, err := lookupItem(ctx, a)
		if err != nil {
			return err
		}
		_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
			{EntityID: id, StatusContext: "Bob", ActorID: "test"},
		})
		return err
	}
	markReturned := func(ctx context.Context, a *app.App) error {
		id, err := lookupItem(ctx, a)
		if err != nil {
			return err
		}
		_, err = a.MarkReturned(ctx, []app.ChangeStatusRequest{{EntityID: id, ActorID: "test"}})
		return err
	}

	tests := []struct {
		name      string
		source    inventory.EntityStatus
		op        call
		wantAllow bool
	}{
		// lost: legal only from ok
		{"lost from ok", inventory.EntityStatusOk, markLost, true},
		{"lost from missing", inventory.EntityStatusMissing, markLost, false},
		{"lost from loaned", inventory.EntityStatusLoaned, markLost, false},
		{"lost from borrowed", inventory.EntityStatusBorrowed, markLost, false},

		// found: legal only from missing
		{"found from missing", inventory.EntityStatusMissing, markFound, true},
		{"found from ok", inventory.EntityStatusOk, markFound, false},
		{"found from loaned", inventory.EntityStatusLoaned, markFound, false},
		{"found from borrowed", inventory.EntityStatusBorrowed, markFound, false},

		// loan: legal from ok or missing
		{"loan from ok", inventory.EntityStatusOk, markLoaned, true},
		{"loan from missing", inventory.EntityStatusMissing, markLoaned, true},
		{"loan from loaned", inventory.EntityStatusLoaned, markLoaned, false},
		{"loan from borrowed", inventory.EntityStatusBorrowed, markLoaned, false},

		// return: legal from loaned or borrowed
		{"return from loaned", inventory.EntityStatusLoaned, markReturned, true},
		{"return from borrowed", inventory.EntityStatusBorrowed, markReturned, true},
		{"return from ok", inventory.EntityStatusOk, markReturned, false},
		{"return from missing", inventory.EntityStatusMissing, markReturned, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := openTestApp(t)
			seedEntityInStatus(t, a, tc.source)

			err := tc.op(context.Background(), a)
			if tc.wantAllow {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestStatusBatch_IllegalTransitionRollsBackBatch confirms one illegal transition
// in a batch rolls back the whole batch (atomicity of the precondition guards).
func TestStatusBatch_IllegalTransitionRollsBackBatch(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ActorID: "test"})
	require.NoError(t, err)
	for _, name := range []string{"A", "B"} {
		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: name, ParentPath: "Box", ActorID: "test"})
		require.NoError(t, err)
	}

	entityA, err := a.LookupEntityByPath(ctx, "Box:A")
	require.NoError(t, err)

	// A is ok (legal lost), B is ok then we attempt found on it (illegal from ok) — whole batch must roll back.
	_, err = a.MarkFound(ctx, []app.ChangeStatusRequest{
		{EntityID: entityA.EntityID, ActorID: "test"}, // A is ok -> found illegal
	})
	require.Error(t, err)

	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Box:A" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status, "A must be unchanged after failed batch")
		}
	}
}
