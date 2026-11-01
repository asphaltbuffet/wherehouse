package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
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
