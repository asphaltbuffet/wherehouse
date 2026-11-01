package app

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// destFixture is a snapshot covering every legal-destination rule:
//
//	Garage > Shelf            (Shelf is Drill's home)
//	Deck > Drill > Bit        (Drill used out to Deck: displaced)
//	Box                       (discrete)
//	Loft (removed) > Trunk    (live under a removed ancestor, #287)
//	Loft (removed) > Saw      (displaced, home Garage: its current parent cannot be a home)
func destFixture() snapshotHierarchy {
	ok := inventory.EntityStatusOk
	return newSnapshotHierarchy([]*inventory.Entity{
		{EntityID: "garage", FullPathDisplay: "Garage", Status: ok},
		{
			EntityID: "shelf", FullPathDisplay: "Garage:Shelf",
			ParentID: new("garage"), HomeID: new("garage"), Status: ok,
		},
		{EntityID: "deck", FullPathDisplay: "Deck", Status: ok},
		{EntityID: "drill", FullPathDisplay: "Deck:Drill", ParentID: new("deck"), HomeID: new("shelf"), Status: ok},
		{EntityID: "bit", FullPathDisplay: "Deck:Drill:Bit", ParentID: new("drill"), HomeID: new("drill"), Status: ok},
		{EntityID: "box", FullPathDisplay: "Box", Discrete: true, Status: ok},
		{EntityID: "loft", FullPathDisplay: "Loft", Status: inventory.EntityStatusRemoved},
		{EntityID: "trunk", FullPathDisplay: "Loft:Trunk", ParentID: new("loft"), HomeID: new("loft"), Status: ok},
		{EntityID: "saw", FullPathDisplay: "Loft:Saw", ParentID: new("loft"), HomeID: new("garage"), Status: ok},
		{EntityID: "kite", FullPathDisplay: "Kite", HomeID: new("garage"), Status: ok},
		{EntityID: "nail", FullPathDisplay: "Box:Nail", ParentID: new("box"), HomeID: new("garage"), Status: ok},
	})
}

func TestCheckRelocation(t *testing.T) {
	h := destFixture()
	tests := []struct {
		name, moving, dest, verb string
		kind                     error // nil = legal
		msg                      string
	}{
		{"to-root", "bit", "", verbMove, nil, ""},
		{"direct-home-cycle", "shelf", "drill", verbMove, ErrHomeCycle,
			`cannot move "Garage:Shelf" there: "Deck:Drill"'s home would end up inside it ` +
				`— return "Deck:Drill" first: invalid destination`},
		{"indirect-home-cycle", "garage", "bit", verbUse, ErrHomeCycle,
			`cannot use "Garage" there: "Deck:Drill"'s home would end up inside it ` +
				`— return "Deck:Drill" first: invalid destination`},
		{"into-descendant", "garage", "shelf", verbMove, ErrInsideItself,
			`cannot move "Garage" into itself or its own contents: invalid destination`},
		{"discrete", "bit", "box", verbMove, ErrDiscrete,
			`cannot move "Deck:Drill:Bit": "Box" is discrete and cannot hold entities: invalid destination`},
		{"removed-dest", "bit", "loft", verbMove, ErrNotFound, ""},
		{"missing-dest", "bit", "nope", verbMove, ErrNotFound, ""},
		{"move-to-home", "drill", "shelf", verbMove, ErrDestinationIsHome,
			`cannot move "Deck:Drill" to its home — use 'return' instead: ` +
				`destination is the entity's home: invalid destination`},
		{"use-at-home", "drill", "shelf", verbUse, ErrDestinationIsHome,
			`cannot use "Deck:Drill" at its home — use 'return' instead: ` +
				`destination is the entity's home: invalid destination`},
		{"use-already-there", "drill", "deck", verbUse, ErrNoChange,
			`cannot use "Deck:Drill": already there: invalid destination`},
		{"rehome", "drill", "deck", verbMove, nil, ""},
		{"move-not-displaced-already-there", "bit", "drill", verbMove, ErrNoChange,
			`cannot move "Deck:Drill:Bit": already there: invalid destination`},
		{"use-not-displaced-at-home", "bit", "drill", verbUse, ErrDestinationIsHome, ""},
		{"under-removed-ancestor", "trunk", "garage", verbMove, nil, ""},
		{"onto-under-removed-ancestor", "bit", "trunk", verbMove, nil, ""},
		{"rehome-under-removed", "saw", "loft", verbMove, ErrNotFound,
			`cannot move "Loft:Saw" here: its current location cannot be a home`},
		{"return-home", "drill", "shelf", verbReturn, nil, ""},
		{"into-self", "bit", "bit", verbMove, ErrInsideItself,
			`cannot move "Deck:Drill:Bit" into itself or its own contents: invalid destination`},
		{"rehome-at-root", "kite", "", verbMove, nil, ""},
		{"rehome-under-discrete", "nail", "box", verbMove, ErrDiscrete, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRelocation(t.Context(), h, h[tt.moving], tt.dest, tt.verb)
			if tt.kind == nil {
				require.NoError(t, err)
				return
			}
			var dr *DestinationRejection
			require.ErrorAs(t, err, &dr)
			require.ErrorIs(t, err, tt.kind)
			require.ErrorIs(t, err, ErrInvalidDestination, "every destination rejection matches the generic sentinel")
			if tt.msg != "" {
				assert.EqualError(t, err, tt.msg)
			}
		})
	}
}

func TestCheckRelocation_HomeCycleNamesBlocker(t *testing.T) {
	h := destFixture()
	err := checkRelocation(t.Context(), h, h["garage"], "bit", verbMove)
	var dr *DestinationRejection
	require.ErrorAs(t, err, &dr)
	assert.Equal(t, "Deck:Drill", dr.Blocker)
}

func TestCheckRelocation_LoopingAncestorsIsAnError(t *testing.T) {
	ok := inventory.EntityStatusOk
	h := newSnapshotHierarchy([]*inventory.Entity{
		{EntityID: "a", FullPathDisplay: "A", ParentID: new("b"), HomeID: new("b"), Status: ok},
		{EntityID: "b", FullPathDisplay: "B", ParentID: new("a"), HomeID: new("a"), Status: ok},
		{EntityID: "c", FullPathDisplay: "C", Status: ok},
	})
	err := checkRelocation(t.Context(), h, h["c"], "a", verbMove)
	require.Error(t, err)
	var dr *DestinationRejection
	assert.NotErrorAs(t, err, &dr, "a damaged projection is an error, not a rejection")
}

func TestCheckNewParent(t *testing.T) {
	h := destFixture()
	ctx := t.Context()

	got, err := checkNewParent(ctx, h, verbAdd, "Nails", "deck")
	require.NoError(t, err)
	assert.Equal(t, "deck", got.EntityID)

	_, err = checkNewParent(ctx, h, verbAdd, "Nails", "box")
	require.ErrorIs(t, err, ErrDiscrete)
	require.ErrorIs(t, err, ErrInvalidDestination)
	require.EqualError(t, err, `cannot add "Nails": "Box" is discrete and cannot hold entities: invalid destination`)

	_, err = checkNewParent(ctx, h, verbBorrow, "Nails", "loft")
	require.ErrorIs(t, err, ErrNotFound)

	_, err = checkNewParent(ctx, h, verbAdd, "Nails", "nope")
	require.ErrorIs(t, err, ErrNotFound)
}

// describeDestErr reduces a checkRelocation result to something comparable
// across adapters: "" when legal, the Kind's text for a rejection, otherwise
// the error text.
func describeDestErr(err error) string {
	var dr *DestinationRejection
	switch {
	case err == nil:
		return ""
	case errors.As(err, &dr) && dr.Rehome:
		return "rehome rejection: " + dr.Kind.Error()
	case errors.As(err, &dr):
		return "rejection: " + dr.Kind.Error()
	default:
		return "error: " + err.Error()
	}
}

// TestDestinationAdapters_Agree builds a real database holding every rule's
// trigger and asserts the snapshot adapter (picker) and the transaction
// adapter (write guards) decide every (moving, destination, verb) triple the
// same way (ADR 0041).
func TestDestinationAdapters_Agree(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(store.Config{Path: filepath.Join(t.TempDir(), "test.db"), AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	a := New(s)

	ids := map[string]string{}
	mk := func(name, parent string, discrete bool) {
		e, cerr := a.CreateEntity(ctx, CreateEntityRequest{
			DisplayName: name, ParentID: ids[parent], Discrete: discrete, ActorID: "t",
		})
		require.NoError(t, cerr)
		ids[name] = e.EntityID
	}
	mk("Garage", "", false)
	mk("Shelf", "Garage", false)
	mk("Drill", "Shelf", false)
	mk("Bit", "Drill", false)
	mk("Deck", "", false)
	mk("Box", "", true)
	mk("Loft", "", false)
	mk("Trunk", "Loft", false)
	mk("Saw", "Garage", false)
	_, err = a.UseEntity(ctx, UseEntityRequest{EntityID: ids["Drill"], NewParentID: ids["Deck"], ActorID: "t"})
	require.NoError(t, err)
	_, err = a.UseEntity(ctx, UseEntityRequest{EntityID: ids["Saw"], NewParentID: ids["Loft"], ActorID: "t"})
	require.NoError(t, err)
	// Remove Loft leaving Trunk and the displaced Saw live beneath it (#287),
	// bypassing app guards; re-homing Saw in place is then rejected.
	raw, err := json.Marshal(eventbus.EntityRemovedPayload{EntityID: ids["Loft"]})
	require.NoError(t, err)
	_, err = eventbus.New(s).Dispatch(ctx, inventory.EntityRemovedEvent, "t", raw, nil)
	require.NoError(t, err)

	all, err := s.ListAllEntities(ctx)
	require.NoError(t, err)
	snap := newSnapshotHierarchy(all)
	dests := []string{"", "missing-id"}
	for _, e := range all {
		dests = append(dests, e.EntityID)
	}

	seen := map[string]bool{}
	for _, moving := range all {
		if moving.Status == inventory.EntityStatusRemoved {
			continue
		}
		for _, dest := range dests {
			for _, verb := range []string{verbMove, verbUse, verbReturn} {
				fromSnap := describeDestErr(checkRelocation(ctx, snap, moving, dest, verb))
				var fromTx string
				require.NoError(t, s.ExecInTransaction(ctx, func(tx store.Tx) error {
					fromTx = describeDestErr(checkRelocation(ctx, txHierarchy{a: a, tx: tx}, moving, dest, verb))
					return nil
				}))
				assert.Equal(t, fromSnap, fromTx, "%s %s → %q", verb, moving.FullPathDisplay, dest)
				fromStore := describeDestErr(checkRelocation(ctx, storeHierarchy{a: a}, moving, dest, verb))
				assert.Equal(t, fromSnap, fromStore, "%s %s → %q", verb, moving.FullPathDisplay, dest)
				seen[fromSnap] = true
			}
		}
	}
	for _, kind := range []error{ErrHomeCycle, ErrInsideItself, ErrDiscrete, ErrNotFound, ErrDestinationIsHome, ErrNoChange} {
		assert.True(t, seen["rejection: "+kind.Error()], "fixture never exercised %v", kind)
	}
	assert.True(t, seen["rehome rejection: "+ErrNotFound.Error()], "fixture never exercised a rehome rejection")
	assert.True(t, seen[""], "fixture never produced a legal destination")
}

var (
	_ hierarchy = txHierarchy{}
	_ hierarchy = snapshotHierarchy{}
)
