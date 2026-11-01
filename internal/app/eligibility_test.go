package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func facts(status inventory.EntityStatus) app.EntityFacts {
	return app.EntityFacts{Path: "Garage:Drill", Status: status}
}

func TestCheck(t *testing.T) {
	ok := facts(inventory.EntityStatusOk)
	missing := facts(inventory.EntityStatusMissing)
	loaned := facts(inventory.EntityStatusLoaned)
	borrowed := facts(inventory.EntityStatusBorrowed)

	locked := ok
	locked.Locked = true
	lockedBorrowed := borrowed
	lockedBorrowed.Locked = true
	displaced := ok
	displaced.Displaced = true
	discrete := ok
	discrete.Discrete = true
	withChild := ok
	withChild.LiveChildren = 1
	borrowedWithChild := borrowed
	borrowedWithChild.LiveChildren = 1
	isHome := ok
	isHome.DisplacedHomedHere = 2

	tests := []struct {
		name   string
		f      app.EntityFacts
		a      app.Action
		kind   error // nil ⇒ allowed
		remedy app.Action
	}{
		// lost: ok only, not locked (ADR 0024)
		{"lost ok", ok, app.ActionLost, nil, 0},
		{"lost missing", missing, app.ActionLost, app.ErrWrongStatus, 0},
		{"lost loaned (asymmetric: return first)", loaned, app.ActionLost, app.ErrWrongStatus, app.ActionReturn},
		{"lost locked", locked, app.ActionLost, app.ErrLocked, 0},
		// found: missing only, no locked guard
		{"found missing", missing, app.ActionFound, nil, 0},
		{"found ok", ok, app.ActionFound, app.ErrWrongStatus, 0},
		{"found loaned", loaned, app.ActionFound, app.ErrWrongStatus, app.ActionReturn},
		// loan: ok or missing (asymmetric), not locked
		{"loan ok", ok, app.ActionLoan, nil, 0},
		{"loan missing", missing, app.ActionLoan, nil, 0},
		{"loan borrowed", borrowed, app.ActionLoan, app.ErrWrongStatus, 0},
		{"loan locked", locked, app.ActionLoan, app.ErrLocked, 0},
		// return (ADR 0039 table)
		{"return displaced ok", displaced, app.ActionReturn, nil, 0},
		{"return ok at home", ok, app.ActionReturn, app.ErrNoChange, 0},
		{"return loaned", loaned, app.ActionReturn, nil, 0},
		{"return borrowed", borrowed, app.ActionReturn, nil, 0},
		{"return borrowed with child", borrowedWithChild, app.ActionReturn, app.ErrHasChildren, 0},
		{"return missing", missing, app.ActionReturn, app.ErrWrongStatus, app.ActionFound},
		// move/use: ok or borrowed, not locked (ADR 0034)
		{"move ok", ok, app.ActionMove, nil, 0},
		{"move borrowed", borrowed, app.ActionMove, nil, 0},
		{"move missing", missing, app.ActionMove, app.ErrWrongStatus, app.ActionFound},
		{"move loaned", loaned, app.ActionMove, app.ErrWrongStatus, app.ActionReturn},
		{"move locked borrowed", lockedBorrowed, app.ActionMove, app.ErrLocked, 0},
		{"use loaned", loaned, app.ActionUse, app.ErrWrongStatus, app.ActionReturn},
		// remove: not borrowed, no live children, not a home (ADR 0022, 0032, 0039)
		{"remove ok", ok, app.ActionRemove, nil, 0},
		{"remove borrowed", borrowed, app.ActionRemove, app.ErrWrongStatus, app.ActionReturn},
		{"remove with child", withChild, app.ActionRemove, app.ErrHasChildren, 0},
		{"remove home", isHome, app.ActionRemove, app.ErrIsHome, 0},
		// lock/unlock
		{"lock ok", ok, app.ActionLock, nil, 0},
		{"lock borrowed", borrowed, app.ActionLock, nil, 0},
		{"lock already", locked, app.ActionLock, app.ErrNoChange, 0},
		{"lock missing", missing, app.ActionLock, app.ErrWrongStatus, app.ActionFound},
		{"lock displaced", displaced, app.ActionLock, app.ErrDisplaced, app.ActionReturn},
		{"unlock locked", locked, app.ActionUnlock, nil, 0},
		{"unlock not locked", ok, app.ActionUnlock, app.ErrNoChange, 0},
		// discrete
		{"set discrete ok", ok, app.ActionSetDiscrete, nil, 0},
		{"set discrete already", discrete, app.ActionSetDiscrete, app.ErrNoChange, 0},
		{"set discrete with child", withChild, app.ActionSetDiscrete, app.ErrHasChildren, 0},
		{"set discrete home", isHome, app.ActionSetDiscrete, app.ErrIsHome, 0},
		{"clear discrete", discrete, app.ActionClearDiscrete, nil, 0},
		{"clear discrete not set", ok, app.ActionClearDiscrete, app.ErrNoChange, 0},
		// add-child / tag
		{"add child", ok, app.ActionAddChild, nil, 0},
		{"add child discrete", discrete, app.ActionAddChild, app.ErrDiscrete, 0},
		{"tag loaned", loaned, app.ActionTag, nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rej := app.Check(tc.f, tc.a)
			if tc.kind == nil {
				assert.Nil(t, rej)
				return
			}
			require.NotNil(t, rej)
			require.ErrorIs(t, rej, tc.kind)
			assert.Equal(t, tc.a, rej.Action)
			assert.Equal(t, tc.f.Status, rej.Status)
			assert.Equal(t, tc.remedy, rej.Remedy)
		})
	}
}

func TestRejection_Error(t *testing.T) {
	ok := facts(inventory.EntityStatusOk)
	locked := ok
	locked.Locked = true
	displaced := ok
	displaced.Displaced = true
	oneChild := ok
	oneChild.LiveChildren = 1
	twoChildren := ok
	twoChildren.LiveChildren = 2
	homeOne := ok
	homeOne.DisplacedHomedHere = 1
	homeTwo := ok
	homeTwo.DisplacedHomedHere = 2

	tests := []struct {
		f    app.EntityFacts
		a    app.Action
		want string
	}{
		{locked, app.ActionLost, `cannot mark "Garage:Drill" as missing: entity is locked`},
		{
			facts(inventory.EntityStatusLoaned),
			app.ActionMove,
			`cannot move "Garage:Drill": entity is loaned — return it first`,
		},
		{
			facts(inventory.EntityStatusMissing),
			app.ActionUse,
			`cannot use "Garage:Drill": entity is missing — mark it found first`,
		},
		{facts(inventory.EntityStatusBorrowed), app.ActionLoan, `cannot loan "Garage:Drill": entity is borrowed`},
		{facts(inventory.EntityStatusOk), app.ActionFound, `cannot mark "Garage:Drill" as found: entity is ok`},
		{ok, app.ActionReturn, `cannot return "Garage:Drill": already at home`},
		{locked, app.ActionLock, `cannot lock "Garage:Drill": already locked`},
		{ok, app.ActionUnlock, `cannot unlock "Garage:Drill": not locked`},
		{displaced, app.ActionLock, `cannot lock "Garage:Drill": entity is displaced — return it first`},
		{
			oneChild,
			app.ActionRemove,
			`cannot remove "Garage:Drill": it contains 1 other entity — move or remove it first`,
		},
		{
			twoChildren,
			app.ActionSetDiscrete,
			`cannot mark "Garage:Drill" discrete: it contains 2 other entities — move or remove them first`,
		},
		{homeOne, app.ActionRemove, `cannot remove "Garage:Drill": it is home to 1 displaced entity — return it first`},
		{
			homeTwo,
			app.ActionSetDiscrete,
			`cannot mark "Garage:Drill" discrete: it is home to 2 displaced entities — return them first`,
		},
		{
			facts(inventory.EntityStatusBorrowed),
			app.ActionRemove,
			`cannot remove "Garage:Drill": entity is borrowed — return it first`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			rej := app.Check(tc.f, tc.a)
			require.NotNil(t, rej)
			assert.EqualError(t, rej, tc.want)
		})
	}
}

func TestRequire_AllowedIsUntypedNil(t *testing.T) {
	err := facts(inventory.EntityStatusOk).Require(app.ActionLost)
	require.NoError(t, err, "allowed action must return an untyped nil error")
}

func TestRequire_RejectedUnwraps(t *testing.T) {
	err := facts(inventory.EntityStatusLoaned).Require(app.ActionMove)
	var rej *app.Rejection
	require.ErrorAs(t, err, &rej)
	assert.Equal(t, app.ActionReturn, rej.Remedy)
}

func TestActionsFor_CoversEveryAction(t *testing.T) {
	set := app.ActionsFor(facts(inventory.EntityStatusLoaned))
	for _, a := range app.Actions() {
		assert.Contains(t, set, a, a.String())
		assert.Equal(t, app.Check(facts(inventory.EntityStatusLoaned), a), set.Rejection(a), a.String())
	}
	assert.Nil(t, app.ActionSet(nil).Rejection(app.ActionMove), "missing key means allowed")
}
