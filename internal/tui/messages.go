package tui

import "github.com/asphaltbuffet/wherehouse/internal/app"

// actionDoneMsg is returned after any mutating app call completes.
type actionDoneMsg struct {
	result app.EntityResult
	err    error
}

// rootsLoadedMsg carries the result of the initial root entity fetch.
type rootsLoadedMsg struct {
	items []app.EntityResult
	err   error
}

// treeExpandedMsg carries children loaded when a node is expanded.
type treeExpandedMsg struct {
	parentID string
	depth    int
	items    []app.EntityResult
	err      error
}

// treeRefreshMsg carries reloaded children for a parent after a mutation.
type treeRefreshMsg struct {
	parentID       string
	items          []app.EntityResult
	targetEntityID string
	err            error
}

// treeRevealMsg carries one level of children needed to reveal a scry result.
// When remainingPath is empty the target has been reached.
type treeRevealMsg struct {
	parentID       string
	depth          int
	items          []app.EntityResult
	remainingPath  []string // entity IDs still to expand toward the target
	targetEntityID string
	err            error
}

// historyLoadedMsg carries GetHistory results for the history view.
type historyLoadedMsg struct {
	entity app.EntityResult
	items  []app.HistoryResult
	err    error
	gen    int
}

// scryResultsMsg carries FindEntities results for the scry view.
type scryResultsMsg struct {
	items []app.FindResult
	err   error
}

// scryNavigatedMsg triggers tree reveal navigation to a scry result.
type scryNavigatedMsg struct {
	targetEntityID string
	ancestorIDs    []string // ordered root→parent, excluding the target itself
	err            error
}
