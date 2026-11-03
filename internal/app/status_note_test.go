package app_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// A return of a loaned, displaced entity writes a status change then a
// returned_home; the note rides on the first event only.
func TestMarkReturned_NoteOnFirstEventOnly(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx := t.Context()

	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "t"})
	require.NoError(t, err)
	shed, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Shed", ActorID: "t"})
	require.NoError(t, err)
	drill, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Drill", ParentID: garage.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = a.UseEntity(ctx, app.UseEntityRequest{EntityID: drill.EntityID, NewParentID: shed.EntityID, ActorID: "t"})
	require.NoError(t, err)
	_, err = a.MarkLoaned(
		ctx,
		[]app.ChangeStatusRequest{{EntityID: drill.EntityID, StatusContext: "Sam", ActorID: "t"}},
	)
	require.NoError(t, err)

	_, err = a.MarkReturned(ctx, []app.ChangeStatusRequest{
		{EntityID: drill.EntityID, ActorID: "t", Note: "back from Sam"},
	})
	require.NoError(t, err)

	rows, err := s.DB().QueryContext(ctx, `SELECT event_type, note FROM events ORDER BY event_id DESC LIMIT 2`)
	require.NoError(t, err)
	defer rows.Close()
	type row struct {
		et   string
		note sql.NullString
	}
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.et, &r.note))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2)
	assert.Equal(t, row{et: "entity.returned_home"}, got[0])
	assert.Equal(
		t,
		row{et: "entity.status_changed", note: sql.NullString{String: "back from Sam", Valid: true}},
		got[1],
	)
}
