package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
	versioncmd "github.com/asphaltbuffet/wherehouse/internal/version"
)

const keyEsc = "esc"

// tuiMode controls which sub-model is active.
type tuiMode int

const (
	modeBrowse  tuiMode = iota // two-pane navigator (default)
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

// borderWidth is the total horizontal space consumed by a single rounded border (left + right).
const borderWidth = 2

// borderHeight is the total vertical space consumed by a single rounded border (top + bottom).
const borderHeight = 2

// headerHeight is the number of terminal rows used by the application header bar.
const headerHeight = 1

// helpHeightShort is the number of rows the collapsed help bar occupies.
const helpHeightShort = 1

// crumbHeight is the number of rows used by the breadcrumb line inside the nav pane border.
const crumbHeight = 1

// listViewOverhead is the number of extra lines bubbles/list View() renders beyond SetSize height.
// The list pads items to fill PerPage slots then wraps in lipgloss.Height(N), which in
// lipgloss v2 counts trailing newlines as extra rows, producing height+3 total lines.
const listViewOverhead = 3

// navWidthRatio is the fraction of terminal width given to the navigation pane.
const navWidthRatio = 0.60

// navPaneMinWidth is the minimum number of columns for the navigation pane.
const navPaneMinWidth = 20

// keyMap defines all keybindings exposed to the help bubble.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	DrillIn  key.Binding
	DrillOut key.Binding
	Filter   key.Binding
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
		DrillIn: key.NewBinding(
			key.WithKeys("l", "right", "enter"),
			key.WithHelp("l/→", "open"),
		),
		DrillOut: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "back"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
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
	return []key.Binding{k.Up, k.Down, k.DrillIn, k.DrillOut, k.Scry, k.Detail, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.DrillIn, k.DrillOut, k.Filter},
		{k.Add, k.Loan, k.Borrow},
		{k.Lost, k.Return, k.Found},
		{k.History, k.Scry, k.Detail, k.Help, k.Quit},
	}
}

// item wraps app.EntityResult to satisfy bubbles/list.Item.
type item struct {
	result app.EntityResult
}

func (i item) FilterValue() string { return i.result.DisplayName }
func (i item) Title() string       { return i.result.DisplayName }
func (i item) Description() string { return "" }

// rootsLoadedMsg carries the result of the initial root entity fetch.
type rootsLoadedMsg struct {
	items []app.EntityResult
	err   error
}

// childrenLoadedMsg carries the result of a drill-down fetch.
type childrenLoadedMsg struct {
	parentID   string
	parentPath string
	items      []app.EntityResult
	err        error
}

// levelRestoredMsg carries the result of a drill-up reload.
type levelRestoredMsg struct {
	pathStack   []string
	parentStack []string
	items       []app.EntityResult
	err         error
}

// Model is the bubbletea model for the TUI.
type Model struct {
	app         App
	list        list.Model
	help        help.Model
	keys        keyMap
	st          *styles.Styles
	pathStack   []string // FullPathDisplay values, empty = at root
	parentStack []string // entity IDs of ancestors, for navigating back
	termWidth   int
	termHeight  int
	err         error
	mode        tuiMode
	errMsg      string        // transient detail-pane error, cleared on next action
	rightPane   rightPaneKind // what the right pane shows (hidden by default)
	form        formModel
	confirm     confirmModel
	history     historyModel
	scry        scryModel
}

// New creates a new TUI model.
func New(a App) Model {
	st := styles.DefaultStyles()
	d := newDelegate(st)
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowHelp(false)

	// Strip keys we've claimed at the Model level so the list never intercepts them.
	km := l.KeyMap
	km.NextPage = key.NewBinding(key.WithKeys("right", "l"))
	km.PrevPage = key.NewBinding(key.WithKeys("left", "h"))
	l.KeyMap = km

	h := help.New()
	h.ShowAll = false

	return Model{
		app:  a,
		list: l,
		help: h,
		keys: defaultKeyMap(),
		st:   st,
	}
}

// Init loads root entities on startup.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		entities, err := m.app.GetRootEntities(context.Background())
		return rootsLoadedMsg{items: entities, err: err}
	}
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Route messages to active sub-model; fall through for modeBrowse.
	switch m.mode {
	case modeBrowse:
		// handled below
	case modeForm:
		return m.updateForm(msg)
	case modeConfirm:
		return m.updateConfirm(msg)
	case modeScry:
		return m.updateScry(msg)
	}

	return m.updateBrowse(msg)
}

func (m Model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	if result, cmd, handled := m.handleLevelMsg(msg); handled {
		return result, cmd
	}

	switch msg := msg.(type) {
	case historyLoadedMsg:
		var cmd tea.Cmd
		m.history, cmd = m.history.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg), nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleLevelMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case rootsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit, true
		}
		m.pathStack = nil
		m.parentStack = nil
		m = m.loadLevel(msg.items, "")
		return m, nil, true

	case childrenLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit, true
		}
		m.pathStack = append(m.pathStack, msg.parentPath)
		m.parentStack = append(m.parentStack, msg.parentID)
		m = m.loadLevel(msg.items, "")
		return m, nil, true

	case levelRestoredMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit, true
		}
		m.pathStack = msg.pathStack
		m.parentStack = msg.parentStack
		m = m.loadLevel(msg.items, "")
		return m, nil, true

	case childRefreshMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("refresh failed: %v", msg.err)
			return m, nil, true
		}
		m = m.loadLevel(msg.items, msg.targetEntityID)
		return m, nil, true

	case scryNavigatedMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("navigate failed: %v", msg.err)
			return m, nil, true
		}
		m.mode = modeBrowse
		m.pathStack = msg.pathStack
		m.parentStack = msg.parentStack
		m = m.loadLevel(msg.items, msg.targetEntityID)
		return m, nil, true
	}

	return m, nil, false
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.termWidth = msg.Width
	m.termHeight = msg.Height
	m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
	m.help.SetWidth(msg.Width)
	return m
}

// loadLevel replaces the list contents and positions cursor on targetEntityID (or top if empty).
func (m Model) loadLevel(entities []app.EntityResult, targetEntityID string) Model {
	m.list.SetItems(toListItems(entities))
	m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
	m.list.ResetFilter()
	selectByID(&m.list, targetEntityID)
	return m
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == keyEsc {
			m.mode = modeBrowse
			return m, nil
		}
	case actionDoneMsg:
		if msg.err != nil {
			m.form.err = msg.err
			return m, nil
		}
		m.mode = modeBrowse
		return m, m.refreshCmd(msg.result.EntityID)
	}
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case confirmCancelledMsg:
		m.mode = modeBrowse
		return m, nil
	case actionDoneMsg:
		m.mode = modeBrowse
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		return m, m.refreshCmd(msg.result.EntityID)
	case tea.KeyPressMsg:
		if msg.String() == keyEsc {
			m.mode = modeBrowse
			return m, nil
		}
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

// refreshCmd reloads the current level after a mutation.
func (m Model) refreshCmd(targetEntityID string) tea.Cmd {
	var parentID string
	if len(m.parentStack) > 0 {
		parentID = m.parentStack[len(m.parentStack)-1]
	}
	a := m.app
	id := targetEntityID
	if parentID == "" {
		return func() tea.Msg {
			items, err := a.GetRootEntities(context.Background())
			return childRefreshMsg{items: items, targetEntityID: id, err: err}
		}
	}
	return func() tea.Msg {
		items, err := a.GetChildren(context.Background(), parentID)
		return childRefreshMsg{items: items, targetEntityID: id, err: err}
	}
}

// selectByID scans the current list for the entity with the given ID and selects it.
// Falls back to ResetSelected if not found (e.g. entity was removed).
func selectByID(l *list.Model, entityID string) {
	if entityID == "" {
		l.ResetSelected()
		return
	}
	for i, it := range l.Items() {
		if it, ok := it.(item); ok && it.result.EntityID == entityID {
			l.Select(i)
			return
		}
	}
	l.ResetSelected()
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Delegate all input to the list while filtering.
	if m.list.SettingFilter() {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	// Navigation keys clear any transient error.
	if key.Matches(msg, m.keys.Up, m.keys.Down, m.keys.DrillIn, m.keys.DrillOut) {
		m.errMsg = ""
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		return m, nil

	case key.Matches(msg, m.keys.PgUp), key.Matches(msg, m.keys.PgDown):
		if m.rightPane != rightPaneHistory {
			return m, nil
		}
		var cmd tea.Cmd
		m.history, cmd = m.history.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.DrillIn):
		return m.drillDown()

	case key.Matches(msg, m.keys.DrillOut):
		return m.drillUp()

	case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
		return m.handleNavKey(msg)

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

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// selectedItem returns the currently highlighted item, if any.
func (m Model) selectedItem() (item, bool) {
	sel, ok := m.list.SelectedItem().(item)
	return sel, ok
}

func (m Model) toggleDetail() (tea.Model, tea.Cmd) {
	if _, ok := m.selectedItem(); !ok {
		return m, nil
	}
	if m.rightPane == rightPaneDetail {
		m.rightPane = rightPaneHidden
	} else {
		m.rightPane = rightPaneDetail
	}
	m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
	return m, nil
}

func (m Model) handleNavKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	if m.rightPane == rightPaneHistory {
		if sel, ok := m.selectedItem(); ok {
			m.history = newHistoryModel(sel.result, m.app, m.st, m.detailPaneWidth(), m.paneHeight())
			return m, tea.Batch(listCmd, m.history.loadCmd())
		}
	}
	return m, listCmd
}

// gateError sets errMsg and returns the model unchanged (no mode switch).
func (m Model) gateError(msg string) (tea.Model, tea.Cmd) {
	m.errMsg = msg
	return m, nil
}

func (m Model) openAdd() (tea.Model, tea.Cmd) {
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	if sel.result.Discrete {
		return m.gateError("cannot add: entity is discrete (no children allowed)")
	}
	m.errMsg = ""
	m.form = newFormModel(formAdd, sel.result, m.app, m.st)
	m.mode = modeForm
	return m, nil
}

func (m Model) openLoan() (tea.Model, tea.Cmd) {
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	r := sel.result
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
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	m.errMsg = ""
	m.form = newFormModel(formBorrow, sel.result, m.app, m.st)
	m.mode = modeForm
	return m, nil
}

func (m Model) openLost() (tea.Model, tea.Cmd) {
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	r := sel.result
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
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	r := sel.result
	if r.Status != inventory.EntityStatusLoaned && r.Status != inventory.EntityStatusBorrowed {
		return m.gateError("cannot return: entity must be loaned or borrowed (is " + r.Status.String() + ")")
	}
	m.errMsg = ""
	m.confirm = newConfirmModel(confirmReturn, r, m.app, m.st)
	m.mode = modeConfirm
	return m, nil
}

func (m Model) openFound() (tea.Model, tea.Cmd) {
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	r := sel.result
	if r.Status != inventory.EntityStatusMissing {
		return m.gateError("cannot mark found: entity must be missing (is " + r.Status.String() + ")")
	}
	m.errMsg = ""
	m.confirm = newConfirmModel(confirmFound, r, m.app, m.st)
	m.mode = modeConfirm
	return m, nil
}

func (m Model) openHistory() (tea.Model, tea.Cmd) {
	sel, ok := m.selectedItem()
	if !ok {
		return m.gateError("no entity selected")
	}
	if m.rightPane == rightPaneHistory {
		m.rightPane = rightPaneHidden
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		return m, nil
	}
	m.errMsg = ""
	m.rightPane = rightPaneHistory
	m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
	m.history = newHistoryModel(sel.result, m.app, m.st, m.detailPaneWidth(), m.paneHeight())
	return m, m.history.loadCmd()
}

func (m Model) openScry() (tea.Model, tea.Cmd) {
	m.errMsg = ""
	m.scry = newScryModel(m.app, m.st)
	m.mode = modeScry
	return m, nil
}

// View renders the full TUI.
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

// CurrentPath returns the breadcrumb string for the current navigation level.
func (m Model) CurrentPath() string {
	if len(m.pathStack) == 0 {
		return "wherehouse"
	}
	return m.pathStack[len(m.pathStack)-1]
}

// ItemCount returns the number of items currently displayed.
func (m Model) ItemCount() int { return len(m.list.Items()) }

// CursorIndex returns the index of the currently highlighted item.
func (m Model) CursorIndex() int { return m.list.Index() }

// RightPane returns the current right pane state: "hidden", "detail", or "history".
func (m Model) RightPane() string {
	switch m.rightPane {
	case rightPaneDetail:
		return "detail"
	case rightPaneHistory:
		return "history"
	case rightPaneHidden:
		return "hidden"
	}
	return "hidden"
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
	w := max(int(float64(m.termWidth)*navWidthRatio), navPaneMinWidth)
	return w
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
		// FullHelp renders len(groups) rows + 1 for the short-help fallback line.
		return len(m.keys.FullHelp()) + 1
	}
	return helpHeightShort
}

func (m Model) paneHeight() int {
	return max(0, m.termHeight-headerHeight-m.helpHeight())
}

func (m Model) listHeight() int {
	return max(0, m.paneHeight()-borderHeight-crumbHeight-listViewOverhead)
}

func (m Model) renderHeader() string {
	title := fmt.Sprintf("wherehouse %s", versioncmd.Version)
	return m.st.TUIHeader().Width(m.termWidth).Render(title)
}

func (m Model) renderNavPane() string {
	crumbText := "wherehouse"
	if len(m.pathStack) > 0 {
		crumbs := make([]string, len(m.pathStack))
		for i, p := range m.pathStack {
			if parsed, err := entitypath.Parse(p); err == nil {
				crumbs[i] = parsed.Base()
			} else {
				crumbs[i] = p
			}
		}
		crumbText = strings.Join(crumbs, " › ")
	}
	crumb := m.st.TUICrumb().Render(crumbText)
	inner := lipgloss.JoinVertical(lipgloss.Left, crumb, m.list.View())

	return m.st.TUINavBorder().
		Width(m.navPaneWidth()).
		Height(m.paneHeight()).
		Render(inner)
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

	sel, ok := m.list.SelectedItem().(item)
	if !ok || m.list.IsFiltered() && m.list.SelectedItem() == nil {
		return prefix + m.st.TUIDetailValue().Render("—")
	}
	if !ok {
		return prefix + m.st.TUIDetailValue().Render("no selection")
	}

	r := sel.result
	w := m.detailPaneInnerWidth()

	label := func(s string) string {
		return m.st.TUIDetailLabel().Render(s + ":")
	}
	val := func(s string) string {
		return m.st.TUIDetailValue().Render(s)
	}
	row := func(l, v string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, l, " ", v)
	}

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

// statusDisplay renders a colored status string for the detail pane.
func statusDisplay(r app.EntityResult, st *styles.Styles) string {
	s := r.Status.String()
	switch r.Status.String() {
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

func (m Model) drillDown() (tea.Model, tea.Cmd) {
	selected, ok := m.list.SelectedItem().(item)
	if !ok || !selected.result.HasChildren {
		return m, nil
	}
	id := selected.result.EntityID
	path := selected.result.FullPathDisplay
	return m, func() tea.Msg {
		children, err := m.app.GetChildren(context.Background(), id)
		return childrenLoadedMsg{parentID: id, parentPath: path, items: children, err: err}
	}
}

func (m Model) drillUp() (tea.Model, tea.Cmd) {
	if len(m.pathStack) == 0 {
		return m, nil
	}
	newLen := len(m.pathStack) - 1
	targetPath := append([]string(nil), m.pathStack[:newLen]...)
	targetParent := append([]string(nil), m.parentStack[:newLen]...)

	if len(targetParent) == 0 {
		return m, func() tea.Msg {
			entities, err := m.app.GetRootEntities(context.Background())
			return rootsLoadedMsg{items: entities, err: err}
		}
	}
	parentID := targetParent[len(targetParent)-1]
	return m, func() tea.Msg {
		children, err := m.app.GetChildren(context.Background(), parentID)
		return levelRestoredMsg{pathStack: targetPath, parentStack: targetParent, items: children, err: err}
	}
}

func toListItems(entities []app.EntityResult) []list.Item {
	items := make([]list.Item, len(entities))
	for i, e := range entities {
		items[i] = item{result: e}
	}
	return items
}
