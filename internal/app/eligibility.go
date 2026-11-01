package app

import (
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// EntityFacts is everything Check needs to know about one live entity
// (ADR 0040). Path is used only to render messages.
type EntityFacts struct {
	Path               string
	Status             inventory.EntityStatus
	Locked             bool
	Discrete           bool
	Displaced          bool
	LiveChildren       int // non-removed direct children
	DisplacedHomedHere int // non-removed entities whose home is this entity but which sit elsewhere
}

// Rejection is why an action may not apply to an entity right now
// (CONTEXT.md "Rejection"). Kind is one of ErrLocked, ErrWrongStatus,
// ErrDisplaced, ErrDiscrete, ErrHasChildren, ErrIsHome or ErrNoChange.
// Remedy is zero when no single action clears it.
type Rejection struct { //nolint:errname // named by CONTEXT.md and ADR 0040
	Action Action
	Kind   error
	Status inventory.EntityStatus
	Count  int // LiveChildren for ErrHasChildren, DisplacedHomedHere for ErrIsHome
	Remedy Action
	Path   string
}

// Unwrap returns the sentinel Kind so [errors.Is] matches it.
func (r *Rejection) Unwrap() error {
	if r == nil {
		return nil
	}
	return r.Kind
}

const nilRejectionText = "<nil rejection>"

// Error renders the CLI sentence: cannot <action> "<path>": <reason>[ — <remedy>].
func (r *Rejection) Error() string {
	if r == nil {
		return nilRejectionText
	}
	msg := fmt.Sprintf("cannot %s: %s", actionPhrase(r.Action, r.Path), r.reason())
	switch r.Remedy { //nolint:exhaustive // only found and return are ever remedies
	case ActionFound:
		msg += " — mark it found first"
	case ActionReturn:
		msg += " — return it first"
	}
	return msg
}

func actionPhrase(a Action, path string) string {
	switch a {
	case ActionLost:
		return fmt.Sprintf("mark %q as missing", path)
	case ActionFound:
		return fmt.Sprintf("mark %q as found", path)
	case ActionSetDiscrete:
		return fmt.Sprintf("mark %q discrete", path)
	case ActionClearDiscrete:
		return fmt.Sprintf("clear discrete on %q", path)
	case ActionAddChild:
		return fmt.Sprintf("add to %q", path)
	case ActionLoan, ActionReturn, ActionMove, ActionUse, ActionRemove, ActionLock, ActionUnlock, ActionTag:
		return fmt.Sprintf("%s %q", a, path)
	}
	return fmt.Sprintf("%s %q", a, path)
}

func (r *Rejection) reason() string {
	switch {
	case errors.Is(r.Kind, ErrLocked):
		return "entity is locked"
	case errors.Is(r.Kind, ErrWrongStatus):
		return "entity is " + r.Status.String()
	case errors.Is(r.Kind, ErrDisplaced):
		return "entity is displaced"
	case errors.Is(r.Kind, ErrDiscrete):
		return "entity is discrete"
	case errors.Is(r.Kind, ErrHasChildren):
		return "it contains " + countPhrase(r.Count, "other entity", "other entities") +
			" — move or remove " + pronoun(r.Count) + " first"
	case errors.Is(r.Kind, ErrIsHome):
		return "it is home to " + countPhrase(r.Count, "displaced entity", "displaced entities") +
			" — return " + pronoun(r.Count) + " first"
	case errors.Is(r.Kind, ErrNoChange):
		return noChangeReason(r.Action)
	}
	return r.Kind.Error()
}

func noChangeReason(a Action) string {
	switch a { //nolint:exhaustive // ErrNoChange is only produced for these actions
	case ActionLock:
		return "already locked"
	case ActionUnlock:
		return "not locked"
	case ActionSetDiscrete:
		return "already discrete"
	case ActionClearDiscrete:
		return "not discrete"
	case ActionReturn:
		return "already at home"
	}
	return ErrNoChange.Error()
}

func countPhrase(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func pronoun(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

// canAddChild is the add-child rule (CONTEXT.md "Eligible parent"): any entity
// that is neither removed nor discrete. Check, the legal-destination rules and
// entityToResult all read it, so the rule is written once (ADR 0041).
func canAddChild(status inventory.EntityStatus, discrete bool) bool {
	return status != inventory.EntityStatusRemoved && !discrete
}

// Check reports whether action a may apply to the entity described by f.
// It is the only place the transition table (ADR 0024, ADR 0039) and the
// locked, discrete, live-children and home rules are encoded. Status is
// checked before locked, so an impossible locked+missing entity reports
// the status.
//
//nolint:gocognit,gocyclo,cyclop,funlen // one flat table is the point (ADR 0040)
func Check(
	f EntityFacts,
	a Action,
) *Rejection {
	reject := func(kind error, count int) *Rejection {
		return &Rejection{
			Action: a, Kind: kind, Status: f.Status, Count: count,
			Remedy: remedyFor(a, kind, f.Status), Path: f.Path,
		}
	}
	isOk := f.Status == inventory.EntityStatusOk
	isMissing := f.Status == inventory.EntityStatusMissing
	isBorrowed := f.Status == inventory.EntityStatusBorrowed

	switch a {
	case ActionLost:
		if !isOk {
			return reject(ErrWrongStatus, 0)
		}
		if f.Locked {
			return reject(ErrLocked, 0)
		}
	case ActionFound:
		if !isMissing {
			return reject(ErrWrongStatus, 0)
		}
	case ActionLoan:
		if !isOk && !isMissing {
			return reject(ErrWrongStatus, 0)
		}
		if f.Locked {
			return reject(ErrLocked, 0)
		}
	case ActionReturn:
		switch f.Status { //nolint:exhaustive // removed never reaches Check
		case inventory.EntityStatusBorrowed:
			return f.leaveRules(reject)
		case inventory.EntityStatusLoaned:
		case inventory.EntityStatusOk:
			if !f.Displaced {
				return reject(ErrNoChange, 0)
			}
		default:
			return reject(ErrWrongStatus, 0)
		}
	case ActionMove, ActionUse:
		if !isOk && !isBorrowed {
			return reject(ErrWrongStatus, 0)
		}
		if f.Locked {
			return reject(ErrLocked, 0)
		}
	case ActionRemove:
		if isBorrowed {
			return reject(ErrWrongStatus, 0)
		}
		return f.leaveRules(reject)
	case ActionLock:
		if f.Locked {
			return reject(ErrNoChange, 0)
		}
		if !isOk && !isBorrowed {
			return reject(ErrWrongStatus, 0)
		}
		if f.Displaced {
			return reject(ErrDisplaced, 0)
		}
	case ActionUnlock:
		if !f.Locked {
			return reject(ErrNoChange, 0)
		}
	case ActionSetDiscrete:
		if f.Discrete {
			return reject(ErrNoChange, 0)
		}
		return f.leaveRules(reject)
	case ActionClearDiscrete:
		if !f.Discrete {
			return reject(ErrNoChange, 0)
		}
	case ActionAddChild:
		if !canAddChild(f.Status, f.Discrete) {
			return reject(ErrDiscrete, 0)
		}
	case ActionTag:
	}
	return nil
}

// leaveRules are the rules shared by every action after which the entity can
// no longer hold children or serve as a home: remove, return of a borrowed
// entity, and set-discrete (ADR 0032, ADR 0039).
func (f EntityFacts) leaveRules(reject func(error, int) *Rejection) *Rejection {
	if f.LiveChildren > 0 {
		return reject(ErrHasChildren, f.LiveChildren)
	}
	if f.DisplacedHomedHere > 0 {
		return reject(ErrIsHome, f.DisplacedHomedHere)
	}
	return nil
}

// remedyFor is the one action that would clear a rejection (CONTEXT.md
// "Remedy"), derived from the transition table.
func remedyFor(a Action, kind error, status inventory.EntityStatus) Action {
	if errors.Is(kind, ErrDisplaced) {
		return ActionReturn
	}
	if !errors.Is(kind, ErrWrongStatus) {
		return 0
	}
	switch {
	case status == inventory.EntityStatusMissing && a != ActionLost:
		return ActionFound
	case status == inventory.EntityStatusLoaned && a != ActionLoan:
		return ActionReturn
	case status == inventory.EntityStatusBorrowed && a == ActionRemove:
		return ActionReturn
	}
	return 0
}

// Require is Check as an error: nil when allowed. Guards use this, never a
// *Rejection assigned to an error variable (a typed nil is a non-nil error).
func (f EntityFacts) Require(a Action) error {
	if rej := Check(f, a); rej != nil {
		return rej
	}
	return nil
}

// ActionSet is the verdict for every Action on one entity. A nil entry, or a
// missing key, means allowed.
type ActionSet map[Action]*Rejection

// ActionsFor runs Check for every Action.
func ActionsFor(f EntityFacts) ActionSet {
	set := make(ActionSet, len(Actions()))
	for _, a := range Actions() {
		set[a] = Check(f, a)
	}
	return set
}

// Rejection returns the verdict for a; nil means allowed.
func (s ActionSet) Rejection(a Action) *Rejection { return s[a] }
