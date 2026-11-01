package app

//go:generate stringer -type=Action -linecomment

// Action is a user intent applied to one existing entity (CONTEXT.md "Action").
// The zero value is not an action; Rejection.Remedy uses it for "no remedy".
type Action int

// Actions, in the order the detail view lists them.
const (
	ActionLost          Action = iota + 1 // lost
	ActionFound                           // found
	ActionLoan                            // loan
	ActionReturn                          // return
	ActionMove                            // move
	ActionUse                             // use
	ActionRemove                          // remove
	ActionLock                            // lock
	ActionUnlock                          // unlock
	ActionSetDiscrete                     // set-discrete
	ActionClearDiscrete                   // clear-discrete
	ActionAddChild                        // add-child
	ActionTag                             // tag
)

// Actions returns every Action in declaration order.
func Actions() []Action {
	return []Action{
		ActionLost, ActionFound, ActionLoan, ActionReturn, ActionMove, ActionUse, ActionRemove,
		ActionLock, ActionUnlock, ActionSetDiscrete, ActionClearDiscrete, ActionAddChild, ActionTag,
	}
}
