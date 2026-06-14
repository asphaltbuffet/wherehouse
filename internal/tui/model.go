package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
	versioncmd "github.com/asphaltbuffet/wherehouse/internal/version"
)

const keyEsc = "esc"

// tuiMode controls which sub-model is active.
type tuiMode int

const (
	modeBrowse  tuiMode = iota // tree navigator (default)
	modeForm                   // text-input form for add/loan/borrow
	modeConfirm                // y/n prompt for lost/found/return
	modeScry                   // inventory-wide search
)

// rightPaneKind controls what the right pane displays in modeBrowse.
type rightPaneKind int

const (
	rightPaneHidden  rightPaneKind = iota // right pane not shown; nav expands to full width
	rightPaneDetail                       // entity detail view
	rightPaneHistory                      // entity event history
)

const (
	borderWidth     = 2
	borderHeight    = 2
	headerHeight    = 1
	helpHeightShort = 1
	navWidthRatio   = 0.60
	navPaneMinWidth = 20
)

// keyMap defines all keybindings exposed to the help bubble.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Expand   key.Binding
	Collapse key.Binding
	Help     key.Binding
	Quit     key.Binding
	Add      key.Binding
	Loan     key.Binding
	Borrow   key.Binding
	Lost     key.Binding
	Return   key.Binding
	Found    key.Binding
	History  key.Binding
	Scry     key.Binding
	Detail   key.Binding
	PgUp     key.Binding
	PgDown   key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Expand: key.NewBinding(
			key.WithKeys("l", "right", "enter"),
			key.WithHelp("l/→", "expand"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "collapse"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "Q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Loan: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "loan"),
		),
		Borrow: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "borrow"),
		),
		Lost: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "lost"),
		),
		Return: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "return"),
		),
		Found: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "found"),
		),
		History: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "history"),
		),
		Scry: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "scry"),
		),
		Detail: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "detail"),
		),
		PgUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "scroll up"),
		),
		PgDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "scroll down"),
		),
	}
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Expand, k.Collapse, k.Scry, k.Detail, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Expand, k.Collapse},
		{k.Add, k.Loan, k.Borrow},
		{k.Lost, k.Return, k.Found},
		{k.History, k.Scry, k.Detail, k.Help, k.Quit},
	}
}

// Model is the bubbletea model for the TUI.
type Model struct {
	app        App
	tree       treeModel
	nodes      []treeNode // all loaded nodes; superset of visible
	visible    []int      // indices into nodes currently shown
	cursor     int        // index into visible
	help       help.Model
	keys       keyMap
	st         *styles.Styles
	termWidth  int
	termHeight int
	err        error
	mode       tuiMode
	errMsg     string
	rightPane  rightPaneKind
	form       formModel
	confirm    confirmModel
	history    historyModel
	scry       scryModel
}

// New constructs a TUI Model backed by the given App.
func New(a App) Model {
	st := styles.DefaultStyles()
	h := help.New()
	h.ShowAll = false
	return Model{
		app:  a,
		tree: newTreeModel(st),
		help: h,
		keys: defaultKeyMap(),
		st:   st,
	}
}

// Init implements tea.Model; fires the initial root entity load.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		entities, err := m.app.GetRootEntities(context.Background())
		return rootsLoadedMsg{items: entities, err: err}
	}
}

// --- Update ---

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeBrowse:
		return m.updateBrowse(msg)
	case modeForm:
		return m.updateForm(msg)
	case modeConfirm:
		return m.updateConfirm(msg)
	case modeScry:
		return m.updateScry(msg)
	default:
		return m.updateBrowse(msg)
	}
}

//nolint:gocognit,gocyclo,cyclop,funlen // message dispatch hub; length is inherent to handling all browse-mode messages
func (m Model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case rootsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.nodes = make([]treeNode, len(msg.items))
		for i, r := range msg.items {
			m.nodes[i] = treeNodeFromResult(r, 0, "")
		}
		m.visible = rebuildVisible(m.nodes)
		m.cursor = 0
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd

	case treeExpandedMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("load failed: %v", msg.err)
			return m, nil
		}
		children := make([]treeNode, len(msg.items))
		for i, r := range msg.items {
			children[i] = treeNodeFromResult(r, msg.depth+1, msg.parentID)
		}
		if pi := findNodeIndex(m.nodes, msg.parentID); pi >= 0 {
			m.nodes[pi].loaded = true
			m.nodes[pi].expanded = true
		}
		m.nodes = spliceChildren(m.nodes, msg.parentID, children)
		m.visible = rebuildVisible(m.nodes)
		m.cursor = setCursorToEntity(m.visible, m.nodes, msg.parentID, m.cursor)
		m.cursor = clampCursor(m.cursor, len(m.visible))
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd

	case treeRefreshMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("refresh failed: %v", msg.err)
			return m, nil
		}
		if msg.parentID == "" {
			m = m.handleRootRefresh(msg.items, msg.targetEntityID)
			m = m.syncTree()
			var hCmd tea.Cmd
			m, hCmd = m.reloadHistoryCmd()
			return m, hCmd
		}
		children := make([]treeNode, len(msg.items))
		depth := 0
		if pi := findNodeIndex(m.nodes, msg.parentID); pi >= 0 {
			depth = m.nodes[pi].depth
			m.nodes[pi].loaded = true
		}
		for i, r := range msg.items {
			children[i] = treeNodeFromResult(r, depth+1, msg.parentID)
		}
		m.nodes = spliceChildren(m.nodes, msg.parentID, children)
		m.visible = rebuildVisible(m.nodes)
		if msg.targetEntityID != "" {
			m.cursor = setCursorToEntity(m.visible, m.nodes, msg.targetEntityID, m.cursor)
		}
		m.cursor = clampCursor(m.cursor, len(m.visible))
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd

	case treeRevealMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("navigate failed: %v", msg.err)
			return m, nil
		}
		children := make([]treeNode, len(msg.items))
		for i, r := range msg.items {
			children[i] = treeNodeFromResult(r, msg.depth+1, msg.parentID)
		}
		if pi := findNodeIndex(m.nodes, msg.parentID); pi >= 0 {
			m.nodes[pi].loaded = true
			m.nodes[pi].expanded = true
		}
		m.nodes = spliceChildren(m.nodes, msg.parentID, children)
		if len(msg.remainingPath) > 0 {
			nextID := msg.remainingPath[0]
			rest := msg.remainingPath[1:]
			nextDepth := msg.depth + 1
			if ni := findNodeIndex(m.nodes, nextID); ni >= 0 && !m.nodes[ni].loaded {
				a := m.app
				return m, func() tea.Msg {
					items, err := a.GetChildren(context.Background(), nextID)
					return treeRevealMsg{
						parentID:       nextID,
						depth:          nextDepth,
						items:          items,
						remainingPath:  rest,
						targetEntityID: msg.targetEntityID,
						err:            err,
					}
				}
			}
			if ni := findNodeIndex(m.nodes, nextID); ni >= 0 {
				m.nodes[ni].expanded = true
			}
			m.visible = rebuildVisible(m.nodes)
			nextChildren := childrenOf(m.nodes, nextID)
			nextMsg := treeRevealMsg{
				parentID:       nextID,
				depth:          nextDepth,
				items:          nodeResultSlice(nextChildren),
				remainingPath:  rest,
				targetEntityID: msg.targetEntityID,
			}
			return m.updateBrowse(nextMsg)
		}
		m.mode = modeBrowse
		m.visible = rebuildVisible(m.nodes)
		m.cursor = setCursorToEntity(m.visible, m.nodes, msg.targetEntityID, m.cursor)
		m.cursor = clampCursor(m.cursor, len(m.visible))
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd

	case scryNavigatedMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("navigate failed: %v", msg.err)
			return m, nil
		}
		m.mode = modeBrowse
		if newCursor := setCursorToEntity(m.visible, m.nodes, msg.targetEntityID, -1); newCursor >= 0 {
			m.cursor = newCursor
			m = m.syncTree()
			var hCmd tea.Cmd
			m, hCmd = m.reloadHistoryCmd()
			return m, hCmd
		}
		if findNodeIndex(m.nodes, msg.targetEntityID) >= 0 {
			return m.revealNode(msg.targetEntityID)
		}
		if len(msg.ancestorIDs) > 0 {
			return m.revealViaAncestors(msg.targetEntityID, msg.ancestorIDs)
		}
		m = m.syncTree()
		return m, nil

	case historyLoadedMsg:
		if msg.gen != m.history.gen {
			return m, nil
		}
		var cmd tea.Cmd
		m.history, cmd = m.history.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg), nil
	}

	return m, nil
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kMsg, ok := msg.(tea.KeyPressMsg); ok && kMsg.String() == keyEsc {
		m.mode = modeBrowse
		return m, nil
	}
	if done, ok := msg.(actionDoneMsg); ok {
		if done.err != nil {
			m.errMsg = done.err.Error()
			m.mode = modeBrowse
			return m, nil
		}
		m.mode = modeBrowse
		return m, m.refreshCmd(done.result.EntityID, done.result.FullPathDisplay)
	}
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case confirmCancelledMsg:
		_ = msg
		m.mode = modeBrowse
		return m, nil
	case actionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.mode = modeBrowse
			return m, nil
		}
		m.mode = modeBrowse
		return m, m.refreshCmd(msg.result.EntityID, msg.result.FullPathDisplay)
	}
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	return m, cmd
}

func (m Model) updateScry(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scryCancelledMsg:
		m.mode = modeBrowse
		return m, nil
	case scryNavigatedMsg:
		return m.updateBrowse(msg)
	case tea.KeyPressMsg:
		if msg.String() == keyEsc {
			m.mode = modeBrowse
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.scry, cmd = m.scry.Update(msg)
	return m, cmd
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.termWidth = msg.Width
	m.termHeight = msg.Height
	m.tree = m.tree.SetSize(m.navPaneInnerWidth(), m.treeHeight())
	m.help.SetWidth(msg.Width)
	if m.rightPane == rightPaneHistory {
		m.history = m.history.Resize(m.detailPaneWidth(), m.paneHeight())
	}
	m = m.syncTree()
	return m
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Up, m.keys.Down, m.keys.Expand, m.keys.Collapse) {
		m.errMsg = ""
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.tree = m.tree.SetSize(m.navPaneInnerWidth(), m.treeHeight())
		return m, nil

	case key.Matches(msg, m.keys.PgUp), key.Matches(msg, m.keys.PgDown):
		if m.rightPane != rightPaneHistory {
			return m, nil
		}
		var cmd tea.Cmd
		m.history, cmd = m.history.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.Expand):
		return m.handleExpand()

	case key.Matches(msg, m.keys.Collapse):
		return m.handleCollapse()

	case key.Matches(msg, m.keys.Up):
		m.cursor = clampCursor(m.cursor-1, len(m.visible))
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd

	case key.Matches(msg, m.keys.Down):
		m.cursor = clampCursor(m.cursor+1, len(m.visible))
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd

	case key.Matches(msg, m.keys.Add):
		return m.openAdd()
	case key.Matches(msg, m.keys.Loan):
		return m.openLoan()
	case key.Matches(msg, m.keys.Borrow):
		return m.openBorrow()
	case key.Matches(msg, m.keys.Lost):
		return m.openLost()
	case key.Matches(msg, m.keys.Return):
		return m.openReturn()
	case key.Matches(msg, m.keys.Found):
		return m.openFound()
	case key.Matches(msg, m.keys.History):
		return m.openHistory()
	case key.Matches(msg, m.keys.Scry):
		return m.openScry()
	case key.Matches(msg, m.keys.Detail):
		return m.toggleDetail()
	}

	return m, nil
}

// handleExpand: l/→/enter — expand if collapsed, collapse if expanded, no-op on leaf.
func (m Model) handleExpand() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	ni := m.visible[m.cursor]
	n := m.nodes[ni]

	if !n.hasChildren {
		return m, nil
	}
	if n.expanded {
		// Already expanded → collapse.
		m.nodes[ni].expanded = false
		m.visible = rebuildVisible(m.nodes)
		m.cursor = clampCursor(m.cursor, len(m.visible))
		m = m.syncTree()
		return m, nil
	}
	// Collapsed → expand. Lazy-load if needed.
	if !n.loaded {
		a := m.app
		id := n.result.EntityID
		depth := n.depth
		return m, func() tea.Msg {
			items, err := a.GetChildren(context.Background(), id)
			return treeExpandedMsg{parentID: id, depth: depth, items: items, err: err}
		}
	}
	// Already loaded — just expand.
	m.nodes[ni].expanded = true
	m.visible = rebuildVisible(m.nodes)
	m = m.syncTree()
	return m, nil
}

// handleCollapse: h/← — collapse if expanded, move to parent if collapsed/leaf.
func (m Model) handleCollapse() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	ni := m.visible[m.cursor]
	n := m.nodes[ni]

	if n.expanded {
		m.nodes[ni].expanded = false
		m.visible = rebuildVisible(m.nodes)
		m.cursor = clampCursor(m.cursor, len(m.visible))
		m = m.syncTree()
		var hCmd tea.Cmd
		m, hCmd = m.reloadHistoryCmd()
		return m, hCmd
	}
	// Move cursor to parent.
	if n.parentID == "" {
		return m, nil
	}
	m.cursor = setCursorToEntity(m.visible, m.nodes, n.parentID, m.cursor)
	m = m.syncTree()
	var hCmd tea.Cmd
	m, hCmd = m.reloadHistoryCmd()
	return m, hCmd
}

// refreshCmd reloads the parent of the given entity and returns a treeRefreshMsg.
// parentPath is used to find the parent node by FullPathDisplay when parentID is unknown.
func (m Model) refreshCmd(entityID, entityFullPath string) tea.Cmd {
	// Determine parentID from the target node's parentID field.
	if ni := findNodeIndex(m.nodes, entityID); ni >= 0 {
		parentID := m.nodes[ni].parentID
		a := m.app
		id := entityID
		if parentID == "" {
			return func() tea.Msg {
				items, err := a.GetRootEntities(context.Background())
				// Roots don't have a single parentID; use empty string sentinel.
				return treeRefreshMsg{parentID: "", items: items, targetEntityID: id, err: err}
			}
		}
		return func() tea.Msg {
			items, err := a.GetChildren(context.Background(), parentID)
			return treeRefreshMsg{parentID: parentID, items: items, targetEntityID: id, err: err}
		}
	}
	// Entity not yet in nodes (just created) — find parent by path.
	// Parent path is entityFullPath without the last segment.
	parts := strings.Split(entityFullPath, ":")
	a := m.app
	id := entityID
	if len(parts) <= 1 {
		return func() tea.Msg {
			items, err := a.GetRootEntities(context.Background())
			return treeRefreshMsg{parentID: "", items: items, targetEntityID: id, err: err}
		}
	}
	parentPath := strings.Join(parts[:len(parts)-1], ":")
	if pi := findNodeIndexByPath(m.nodes, parentPath); pi >= 0 {
		parentID := m.nodes[pi].result.EntityID
		return func() tea.Msg {
			items, err := a.GetChildren(context.Background(), parentID)
			return treeRefreshMsg{parentID: parentID, items: items, targetEntityID: id, err: err}
		}
	}
	// Parent unknown — reload roots as best effort.
	return func() tea.Msg {
		items, err := a.GetRootEntities(context.Background())
		return treeRefreshMsg{parentID: "", items: items, targetEntityID: id, err: err}
	}
}

// treeRefreshMsg with empty parentID means roots; handle that in updateBrowse.
// Override: when parentID is "" treat as root splice.
func (m Model) handleRootRefresh(items []app.EntityResult, targetEntityID string) Model {
	newRoots := make([]treeNode, len(items))
	for i, r := range items {
		if existing := findNodeIndex(m.nodes, r.EntityID); existing >= 0 {
			n := m.nodes[existing]
			n.result = r
			n.hasChildren = r.HasChildren
			newRoots[i] = n
		} else {
			newRoots[i] = treeNodeFromResult(r, 0, "")
		}
	}
	var nonRoots []treeNode
	for _, n := range m.nodes {
		if n.depth > 0 {
			nonRoots = append(nonRoots, n)
		}
	}
	combined := make([]treeNode, 0, len(newRoots)+len(nonRoots))
	combined = append(combined, newRoots...)
	combined = append(combined, nonRoots...)
	m.nodes = combined
	m.visible = rebuildVisible(m.nodes)
	if targetEntityID != "" {
		m.cursor = setCursorToEntity(m.visible, m.nodes, targetEntityID, m.cursor)
	}
	m.cursor = clampCursor(m.cursor, len(m.visible))
	return m
}

// revealNode expands all loaded ancestor nodes to make entityID visible.
func (m Model) revealNode(entityID string) (tea.Model, tea.Cmd) {
	ni := findNodeIndex(m.nodes, entityID)
	if ni < 0 {
		return m, nil
	}
	// Walk ancestors upward and expand each.
	pid := m.nodes[ni].parentID
	for pid != "" {
		if pi := findNodeIndex(m.nodes, pid); pi >= 0 {
			m.nodes[pi].expanded = true
			pid = m.nodes[pi].parentID
		} else {
			break
		}
	}
	m.visible = rebuildVisible(m.nodes)
	m.cursor = setCursorToEntity(m.visible, m.nodes, entityID, m.cursor)
	m.cursor = clampCursor(m.cursor, len(m.visible))
	m = m.syncTree()
	m2, cmd := m.reloadHistoryCmd()
	return m2, cmd
}

// revealViaAncestors fires GetChildren for each unloaded ancestor in order.
func (m Model) revealViaAncestors(targetEntityID string, ancestorIDs []string) (tea.Model, tea.Cmd) {
	if len(ancestorIDs) == 0 {
		return m.revealNode(targetEntityID)
	}
	firstID := ancestorIDs[0]
	rest := ancestorIDs[1:]
	firstDepth := 0
	// Determine depth of first ancestor (it must be a root or already loaded).
	if ni := findNodeIndex(m.nodes, firstID); ni >= 0 {
		firstDepth = m.nodes[ni].depth
		if !m.nodes[ni].loaded {
			a := m.app
			return m, func() tea.Msg {
				items, err := a.GetChildren(context.Background(), firstID)
				return treeRevealMsg{
					parentID:       firstID,
					depth:          firstDepth,
					items:          items,
					remainingPath:  rest,
					targetEntityID: targetEntityID,
					err:            err,
				}
			}
		}
		m.nodes[ni].expanded = true
	}
	m.visible = rebuildVisible(m.nodes)
	m = m.syncTree()
	return m.revealViaAncestors(targetEntityID, rest)
}

// childrenOf returns all direct children of parentID from nodes.
func childrenOf(nodes []treeNode, parentID string) []treeNode {
	var out []treeNode
	for _, n := range nodes {
		if n.parentID == parentID {
			out = append(out, n)
		}
	}
	return out
}

// nodeResultSlice extracts EntityResult from a slice of treeNodes.
func nodeResultSlice(nodes []treeNode) []app.EntityResult {
	out := make([]app.EntityResult, len(nodes))
	for i, n := range nodes {
		out[i] = n.result
	}
	return out
}

// syncTree updates the treeModel's cursor, visible slice, and re-renders.
func (m Model) syncTree() Model {
	m.tree.cursor = m.cursor
	m.tree.visible = m.visible
	m.tree = m.tree.render(m.nodes)
	m.tree = m.tree.scrollToCursor()
	return m
}

func (m Model) reloadHistoryCmd() (Model, tea.Cmd) {
	if m.rightPane != rightPaneHistory {
		return m, nil
	}
	r, ok := m.selectedResult()
	if !ok {
		return m, nil
	}
	m.history = newHistoryModel(r, m.app, m.st, m.detailPaneWidth(), m.paneHeight(), m.history.gen+1)
	return m, m.history.loadCmd()
}

func (m Model) gateError(msg string) (tea.Model, tea.Cmd) {
	m.errMsg = msg
	return m, nil
}

// selectedResult returns the app.EntityResult for the currently focused node.
func (m Model) selectedResult() (app.EntityResult, bool) {
	if len(m.visible) == 0 || m.cursor < 0 || m.cursor >= len(m.visible) {
		return app.EntityResult{}, false
	}
	return m.nodes[m.visible[m.cursor]].result, true
}

func (m Model) openAdd() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	if r.Discrete {
		return m.gateError("cannot add: entity is discrete (no children allowed)")
	}
	m.errMsg = ""
	m.form = newFormModel(formAdd, r, m.app, m.st)
	m.mode = modeForm
	return m, nil
}

func (m Model) openLoan() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	if r.Locked {
		return m.gateError("cannot loan: entity is locked")
	}
	if r.Status != inventory.EntityStatusOk && r.Status != inventory.EntityStatusMissing {
		return m.gateError("cannot loan: entity must be ok or missing (is " + r.Status.String() + ")")
	}
	m.errMsg = ""
	m.form = newFormModel(formLoan, r, m.app, m.st)
	m.mode = modeForm
	return m, nil
}

func (m Model) openBorrow() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	m.errMsg = ""
	m.form = newFormModel(formBorrow, r, m.app, m.st)
	m.mode = modeForm
	return m, nil
}

func (m Model) openLost() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	if r.Locked {
		return m.gateError("cannot mark lost: entity is locked")
	}
	if r.Status != inventory.EntityStatusOk {
		return m.gateError("cannot mark lost: entity must be ok (is " + r.Status.String() + ")")
	}
	m.errMsg = ""
	m.confirm = newConfirmModel(confirmLost, r, m.app, m.st)
	m.mode = modeConfirm
	return m, nil
}

func (m Model) openReturn() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	if r.Status != inventory.EntityStatusLoaned && r.Status != inventory.EntityStatusBorrowed {
		return m.gateError("cannot return: entity must be loaned or borrowed (is " + r.Status.String() + ")")
	}
	m.errMsg = ""
	m.confirm = newConfirmModel(confirmReturn, r, m.app, m.st)
	m.mode = modeConfirm
	return m, nil
}

func (m Model) openFound() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	if r.Status != inventory.EntityStatusMissing {
		return m.gateError("cannot mark found: entity must be missing (is " + r.Status.String() + ")")
	}
	m.errMsg = ""
	m.confirm = newConfirmModel(confirmFound, r, m.app, m.st)
	m.mode = modeConfirm
	return m, nil
}

func (m Model) openHistory() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m.gateError("no entity selected")
	}
	if m.rightPane == rightPaneHistory {
		m.rightPane = rightPaneHidden
		m.tree = m.tree.SetSize(m.navPaneInnerWidth(), m.treeHeight())
		return m, nil
	}
	m.errMsg = ""
	m.rightPane = rightPaneHistory
	m.tree = m.tree.SetSize(m.navPaneInnerWidth(), m.treeHeight())
	m.history = newHistoryModel(r, m.app, m.st, m.detailPaneWidth(), m.paneHeight(), m.history.gen+1)
	return m, m.history.loadCmd()
}

func (m Model) openScry() (tea.Model, tea.Cmd) {
	m.errMsg = ""
	m.scry = newScryModel(m.app, m.st)
	m.scry.results.SetSize(m.termWidth, m.termHeight-scryUIOverhead)
	m.mode = modeScry
	return m, nil
}

func (m Model) toggleDetail() (tea.Model, tea.Cmd) {
	if _, ok := m.selectedResult(); !ok {
		return m, nil
	}
	if m.rightPane == rightPaneDetail {
		m.rightPane = rightPaneHidden
	} else {
		m.rightPane = rightPaneDetail
	}
	m.tree = m.tree.SetSize(m.navPaneInnerWidth(), m.treeHeight())
	return m, nil
}

// --- View ---

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.err != nil {
		v := tea.NewView(fmt.Sprintf("error: %v\n", m.err))
		v.AltScreen = true
		return v
	}

	var content string
	switch m.mode {
	case modeBrowse:
		header := m.renderHeader()
		var body string
		switch m.rightPane {
		case rightPaneDetail:
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderNavPane(), m.renderDetailPane())
		case rightPaneHistory:
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderNavPane(), m.renderHistoryPane())
		case rightPaneHidden:
			body = m.renderNavPane()
		}
		helpBar := m.renderHelp()
		content = lipgloss.JoinVertical(lipgloss.Left, header, body, helpBar)
	case modeForm:
		content = m.form.View(m.termWidth)
	case modeConfirm:
		content = m.confirm.View(m.termWidth)
	case modeScry:
		content = m.scry.View(m.termWidth, m.termHeight)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// --- exported accessors for tests ---

// CurrentPath returns the FullPathDisplay of the currently focused entity, or "wherehouse" at root with no selection.
func (m Model) CurrentPath() string {
	r, ok := m.selectedResult()
	if !ok {
		return "wherehouse"
	}
	return r.FullPathDisplay
}

// ItemCount returns the number of currently visible nodes.
func (m Model) ItemCount() int { return len(m.visible) }

// CursorIndex returns the index of the currently highlighted item.
func (m Model) CursorIndex() int { return m.cursor }

// RightPane returns the current right pane state: "hidden", "detail", or "history".
func (m Model) RightPane() string {
	switch m.rightPane {
	case rightPaneDetail:
		return "detail"
	case rightPaneHistory:
		return "history"
	case rightPaneHidden:
		return "hidden"
	default:
		return "hidden"
	}
}

// Mode returns the current mode name for test assertions.
func (m Model) Mode() string {
	switch m.mode {
	case modeBrowse:
		return "browse"
	case modeForm:
		return "form"
	case modeConfirm:
		return "confirm"
	case modeScry:
		return "scry"
	default:
		return "browse"
	}
}

// ErrMsg returns the current transient error message.
func (m Model) ErrMsg() string { return m.errMsg }

// FormKind returns the active form kind name for test assertions.
func (m Model) FormKind() string { return m.form.kindName() }

// ConfirmNote returns the current note field value in modeConfirm for test assertions.
func (m Model) ConfirmNote() string { return m.confirm.note.Value() }

// --- layout helpers ---

func (m Model) navPaneWidth() int {
	if m.rightPane == rightPaneHidden {
		return m.termWidth
	}
	return max(int(float64(m.termWidth)*navWidthRatio), navPaneMinWidth)
}

func (m Model) navPaneInnerWidth() int {
	return max(0, m.navPaneWidth()-borderWidth)
}

func (m Model) detailPaneWidth() int {
	return max(0, m.termWidth-m.navPaneWidth())
}

func (m Model) detailPaneInnerWidth() int {
	return max(0, m.detailPaneWidth()-borderWidth)
}

func (m Model) helpHeight() int {
	if m.help.ShowAll {
		return len(m.keys.FullHelp()) + 1
	}
	return helpHeightShort
}

func (m Model) paneHeight() int {
	return max(0, m.termHeight-headerHeight-m.helpHeight())
}

func (m Model) treeHeight() int {
	return max(0, m.paneHeight()-borderHeight)
}

func (m Model) renderHeader() string {
	title := fmt.Sprintf("wherehouse %s", versioncmd.Version)
	return m.st.TUIHeader().Width(m.termWidth).Render(title)
}

func (m Model) renderNavPane() string {
	return m.st.TUINavBorder().
		Width(m.navPaneWidth()).
		Height(m.paneHeight()).
		Render(m.tree.View())
}

func (m Model) renderDetailPane() string {
	inner := m.buildDetailContent()
	return m.st.TUIDetailBorder().
		Width(m.detailPaneWidth()).
		Height(m.paneHeight()).
		Render(inner)
}

func (m Model) renderHistoryPane() string {
	title := m.st.TUIDetailLabel().Render("history: " + m.history.entity.FullPathDisplay)
	body := strings.Join([]string{title, m.history.viewportView()}, "\n")
	return m.st.TUIDetailBorder().
		Width(m.detailPaneWidth()).
		Height(m.paneHeight()).
		Render(body)
}

func (m Model) buildDetailContent() string {
	var prefix string
	if m.errMsg != "" {
		prefix = m.st.DangerText().Render(m.errMsg) + "\n\n"
	}

	r, ok := m.selectedResult()
	if !ok {
		return prefix + m.st.TUIDetailValue().Render("no selection")
	}

	w := m.detailPaneInnerWidth()

	label := func(s string) string { return m.st.TUIDetailLabel().Render(s + ":") }
	val := func(s string) string { return m.st.TUIDetailValue().Render(s) }
	row := func(l, v string) string { return lipgloss.JoinHorizontal(lipgloss.Top, l, " ", v) }

	var lines []string
	lines = append(lines, m.st.TUICrumb().Width(w).Render(r.DisplayName))
	lines = append(lines, "")
	lines = append(lines, row(label("status"), statusDisplay(r, m.st)))

	if !r.UpdatedAt.IsZero() {
		lines = append(lines, row(label("updated"), val(r.UpdatedAt.UTC().Format("2006-01-02 15:04"))))
	}

	discreteVal := "no"
	if r.Discrete {
		discreteVal = "yes"
	}
	lines = append(lines, row(label("discrete"), val(discreteVal)))

	lockedVal := "no"
	if r.Locked {
		lockedVal = "yes"
	}
	lines = append(lines, row(label("locked"), val(lockedVal)))

	if len(r.Tags) > 0 {
		lines = append(lines, "")
		lines = append(lines, label("tags"))
		for _, t := range r.Tags {
			lines = append(lines, "  "+m.st.AccentText().Render("#"+t))
		}
	}

	if r.StatusContext != "" {
		lines = append(lines, "")
		lines = append(lines, row(label("note"), val(r.StatusContext)))
	}

	return prefix + strings.Join(lines, "\n")
}

func (m Model) renderHelp() string {
	return m.help.View(m.keys)
}

func statusDisplay(r app.EntityResult, st *styles.Styles) string {
	s := r.Status.String()
	switch s {
	case "ok":
		return st.SuccessText().Render(s)
	case "missing":
		return st.DangerText().Render(s)
	case "borrowed", "loaned":
		return st.WarningText().Render(s)
	default:
		return st.Muted().Render(s)
	}
}
