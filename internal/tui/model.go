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
	"github.com/asphaltbuffet/wherehouse/internal/styles"
	versioncmd "github.com/asphaltbuffet/wherehouse/internal/version"
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
	}
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.DrillIn, k.DrillOut, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.DrillIn, k.DrillOut},
		{k.Filter, k.Help, k.Quit},
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
	switch msg := msg.(type) {
	case rootsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.pathStack = nil
		m.parentStack = nil
		m.list.SetItems(toListItems(msg.items))
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		m.list.ResetFilter()
		m.list.ResetSelected()
		return m, nil

	case childrenLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.pathStack = append(m.pathStack, msg.parentPath)
		m.parentStack = append(m.parentStack, msg.parentID)
		m.list.SetItems(toListItems(msg.items))
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		m.list.ResetFilter()
		m.list.ResetSelected()
		return m, nil

	case levelRestoredMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.pathStack = msg.pathStack
		m.parentStack = msg.parentStack
		m.list.SetItems(toListItems(msg.items))
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		m.list.ResetFilter()
		m.list.ResetSelected()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		m.help.SetWidth(msg.Width)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Delegate all input to the list while filtering.
	if m.list.SettingFilter() {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.list.SetSize(m.navPaneInnerWidth(), m.listHeight())
		return m, nil

	case key.Matches(msg, m.keys.DrillIn):
		return m.drillDown()

	case key.Matches(msg, m.keys.DrillOut):
		return m.drillUp()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the full TUI.
func (m Model) View() tea.View {
	if m.err != nil {
		v := tea.NewView(fmt.Sprintf("error: %v\n", m.err))
		v.AltScreen = true
		return v
	}

	header := m.renderHeader()
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.renderNavPane(), m.renderDetailPane())
	helpBar := m.renderHelp()

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body, helpBar))
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

// --- layout helpers ---

func (m Model) navPaneWidth() int {
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

func (m Model) buildDetailContent() string {
	sel, ok := m.list.SelectedItem().(item)
	if !ok || m.list.IsFiltered() && m.list.SelectedItem() == nil {
		return m.st.TUIDetailValue().Render("—")
	}
	if !ok {
		return m.st.TUIDetailValue().Render("no selection")
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

	return strings.Join(lines, "\n")
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
