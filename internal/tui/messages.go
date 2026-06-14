package tui

import "github.com/asphaltbuffet/wherehouse/internal/app"

// actionDoneMsg is returned after any mutating app call completes.
type actionDoneMsg struct {
	result app.EntityResult
	err    error
}

// childRefreshMsg triggers a reload of the current level after a mutation,
// optionally repositioning the cursor on targetEntityID.
type childRefreshMsg struct {
	items          []app.EntityResult
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

// scryNavigatedMsg triggers browse-tree navigation to a scry result's position.
type scryNavigatedMsg struct {
	items          []app.EntityResult
	targetEntityID string
	pathStack      []string
	parentStack    []string
	err            error
}
