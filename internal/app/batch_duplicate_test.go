package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type statusBatchFn func(context.Context, []app.ChangeStatusRequest) ([]app.EntityResult, error)

// #299: a batch naming the same entity twice is rejected up front with
// ErrDuplicateEntity, naming the entity, and appends nothing.
func TestStatusBatch_DuplicateIDRejected(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, a *app.App) string
		op    func(a *app.App) statusBatchFn
		ctx   string
	}{
		{
			name:  "return loaned",
			setup: loanedEntity,
			op:    func(a *app.App) statusBatchFn { return a.MarkReturned },
		},
		{
			name:  "return borrowed",
			setup: borrowedEntity,
			op:    func(a *app.App) statusBatchFn { return a.MarkReturned },
		},
		{
			name:  "loan",
			setup: okEntity,
			op:    func(a *app.App) statusBatchFn { return a.MarkLoaned },
			ctx:   "Bob",
		},
		{
			name:  "lost",
			setup: okEntity,
			op:    func(a *app.App) statusBatchFn { return a.MarkLost },
		},
		{
			name: "found",
			setup: func(t *testing.T, a *app.App) string {
				t.Helper()
				id := okEntity(t, a)
				_, err := a.MarkLost(t.Context(), []app.ChangeStatusRequest{{EntityID: id, ActorID: "test"}})
				require.NoError(t, err)
				return id
			},
			op: func(a *app.App) statusBatchFn { return a.MarkFound },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := apptesting.OpenApp(t)
			id := tt.setup(t, a)
			before, err := a.GetEntityByID(t.Context(), id)
			require.NoError(t, err)
			beforeLen := historyLen(t, a, id)

			req := app.ChangeStatusRequest{EntityID: id, StatusContext: tt.ctx, ActorID: "test"}
			_, err = tt.op(a)(t.Context(), []app.ChangeStatusRequest{req, req})
			require.ErrorIs(t, err, app.ErrDuplicateEntity)
			assert.Contains(t, err.Error(), before.FullPathDisplay, "error names the entity by path, not ID")

			assert.Equal(t, beforeLen, historyLen(t, a, id), "batch must roll back: no new events")
			after, err := a.GetEntityByID(t.Context(), id)
			require.NoError(t, err)
			assert.Equal(t, before.Status, after.Status)
		})
	}
}

func okEntity(t *testing.T, a *app.App) string {
	t.Helper()
	e, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: "Hammer", ActorID: "test"})
	require.NoError(t, err)
	require.Equal(t, inventory.EntityStatusOk, e.Status)
	return e.EntityID
}

func loanedEntity(t *testing.T, a *app.App) string {
	t.Helper()
	id := okEntity(t, a)
	_, err := a.MarkLoaned(t.Context(), []app.ChangeStatusRequest{
		{EntityID: id, StatusContext: "Bob", ActorID: "test"},
	})
	require.NoError(t, err)
	return id
}

func borrowedEntity(t *testing.T, a *app.App) string {
	t.Helper()
	b, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)
	return b[0].EntityID
}
