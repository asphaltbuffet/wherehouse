# Vim-Style UX Design and Navigation Patterns for Bubbletea TUIs

This document covers Vim-style UX conventions, modal state machines, key binding definition, help systems, layout design, and panel focus management for Bubbletea TUIs. Target audience: Go developers building an inventory tracking TUI that should feel natural to Vim users.

---

## Vim Philosophy Applied to TUIs

Vim's UX philosophy is built around minimizing hand movement and maximizing expressiveness through composable primitives. When applied to a TUI, these principles translate into a predictable, keyboard-driven interface that experienced users can operate without a mouse.

### Home Row Navigation

`hjkl` map to directional movement without leaving the home row:

| Key | Direction |
|-----|-----------|
| `h` | Left (or "go up a level" in trees) |
| `j` | Down |
| `k` | Up |
| `l` | Right (or "enter / expand") |

Arrow keys are accepted as aliases but `hjkl` should always work.

### Modal Design

Modes allow the same key to mean different things in different contexts. This eliminates the need for modifier keys like `Ctrl` or `Alt` for most operations, and keeps bindings mnemonic.

Core modes:

- **Normal**: Navigation, selection, commands. The default mode.
- **Insert**: Text entry (for rename, create, etc.)
- **Search**: Incremental filter or find
- **Command**: Extended commands (`:move`, `:rebuild`, etc.)

### User Expectations

Vim users arrive with strong muscle memory. Violating these conventions will cause friction:

- `ESC` always goes "back" or cancels the current mode — never does something destructive
- `/` activates search; `:` activates command mode
- `g` jumps to top; `G` jumps to bottom
- `q` closes or quits the current pane (not the entire app unless at root)
- `?` toggles help
- `d` deletes, `y` yanks (copies), `p` pastes, `u` undoes
- `Tab` / `Shift+Tab` cycle focus between panels
- `Ctrl+d` / `Ctrl+u` scroll half a page; `Ctrl+f` / `Ctrl+b` scroll a full page

---

## Modal State Machine Pattern in Bubbletea

### Mode Enum

Define modes as an integer enum in your model:

```go
type Mode int

const (
    ModeNormal Mode = iota
    ModeInsert
    ModeSearch
    ModeCommand
)
```

Include the mode in your top-level model along with mode-specific key maps:

```go
type Model struct {
    mode       Mode
    normalKeys NormalKeyMap
    insertKeys InsertKeyMap
    // ... other fields
}
```

### Key Routing in Update()

Dispatch key messages to mode-specific handlers:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch m.mode {
        case ModeNormal:
            return m.handleNormalKey(msg)
        case ModeInsert:
            return m.handleInsertKey(msg)
        case ModeSearch:
            return m.handleSearchKey(msg)
        }
    }
    return m, nil
}
```

Each handler function returns `(tea.Model, tea.Cmd)` and is responsible only for its mode's keys. This keeps `Update()` readable and each handler focused.

### Mode Transitions

| Key | From | To | Notes |
|-----|------|----|-------|
| `i` | Normal | Insert | Enter text input |
| `ESC` | Any | Normal | Always escape to normal |
| `/` | Normal | Search | Activate search bar |
| `:` | Normal | Command | Activate command line |
| `Enter` | Any | — | Confirm current action |
| `q` | Normal | — | Quit/close current pane, return to parent |

`ESC` must reliably return to Normal from any mode. If Normal mode is the root context, `ESC` should be a no-op rather than quitting the application.

---

## Key Binding Definition with bubbles/key

The `bubbles/key` package provides structured key binding definitions with built-in help text and enable/disable support.

### Defining a KeyMap

```go
import "github.com/charmbracelet/bubbles/key"

type NormalKeyMap struct {
    Up     key.Binding
    Down   key.Binding
    Left   key.Binding
    Right  key.Binding
    Top    key.Binding
    Bottom key.Binding
    Select key.Binding
    Search key.Binding
    Delete key.Binding
    Help   key.Binding
    Quit   key.Binding
}

var DefaultNormalKeys = NormalKeyMap{
    Up: key.NewBinding(
        key.WithKeys("k", "up"),
        key.WithHelp("↑/k", "move up"),
    ),
    Down: key.NewBinding(
        key.WithKeys("j", "down"),
        key.WithHelp("↓/j", "move down"),
    ),
    Top: key.NewBinding(
        key.WithKeys("g"),
        key.WithHelp("g", "go to top"),
    ),
    Bottom: key.NewBinding(
        key.WithKeys("G"),
        key.WithHelp("G", "go to bottom"),
    ),
    Search: key.NewBinding(
        key.WithKeys("/"),
        key.WithHelp("/", "search"),
    ),
    Help: key.NewBinding(
        key.WithKeys("?"),
        key.WithHelp("?", "toggle help"),
    ),
    Quit: key.NewBinding(
        key.WithKeys("q", "ctrl+c"),
        key.WithHelp("q", "quit"),
    ),
}
```

### Matching Keys in Update()

```go
case tea.KeyPressMsg:
    switch {
    case key.Matches(msg, m.keys.Up):
        // move up
    case key.Matches(msg, m.keys.Down):
        // move down
    case key.Matches(msg, m.keys.Search):
        m.mode = ModeSearch
    case key.Matches(msg, m.keys.Quit):
        return m, tea.Quit
    }
```

Use `key.Matches` rather than comparing `msg.String()` directly — it handles multi-key bindings and respects the enabled state.

### Dynamic Enable/Disable

Disable bindings that are irrelevant in the current mode. This also removes them from the help display automatically:

```go
// Disable bindings not relevant in current mode
m.keys.Delete.SetEnabled(m.mode == ModeNormal)
m.keys.Quit.SetEnabled(true)
```

Call this after every mode transition to keep the help display accurate.

---

## Help System with bubbles/help

### Model Setup

```go
import "github.com/charmbracelet/bubbles/help"

type Model struct {
    help     help.Model
    showHelp bool
    keys     KeyMap
}
```

### KeyMap Interface

Your key map must implement `help.KeyMap`:

```go
type KeyMap interface {
    ShortHelp() []key.Binding
    FullHelp() [][]key.Binding
}
```

`ShortHelp` returns a flat slice shown as a single line at the bottom of the screen. `FullHelp` returns a slice of columns, displayed as a multi-column overlay when the user presses `?`.

```go
func (k NormalKeyMap) ShortHelp() []key.Binding {
    return []key.Binding{k.Up, k.Down, k.Search, k.Help, k.Quit}
}

func (k NormalKeyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.Up, k.Down, k.Left, k.Right},
        {k.Top, k.Bottom, k.Select},
        {k.Search, k.Delete},
        {k.Help, k.Quit},
    }
}
```

### Rendering

```go
// In View():
helpView := m.help.View(m.keys)
```

Toggle between short and full help using the `?` key:

```go
case key.Matches(msg, m.keys.Help):
    m.help.ShowAll = !m.help.ShowAll
```

### Help Component Behavior

- Auto-generates text from the `WithHelp` labels in each binding
- Truncates gracefully if the terminal is too narrow (ellipsis at end)
- Disabled bindings are excluded from the display automatically
- Short mode: single line, right-aligned or bottom of screen
- Full mode: multi-column overlay toggled with `?`

Available style fields for customization: `ShortKey`, `ShortDesc`, `ShortSeparator`, `Ellipsis`, `FullKey`, `FullDesc`, `FullSeparator`.

---

## Layout Design

### Common Layouts

**Full-screen list** (simplest): Single panel occupying the full terminal. Use for simple commands or drill-down navigation where only one list is visible at a time.

**Split pane** (most common for inventory apps):
- Left sidebar: tree or flat list of locations/categories
- Right main panel: items in the selected location
- Status bar fixed at bottom

**Master-detail**: Same as split pane, but the right panel shows the detail view of the selected item rather than a secondary list.

### Lipgloss Layout Functions

```go
import "github.com/charmbracelet/lipgloss"

// Horizontal split (sidebar | content)
view := lipgloss.JoinHorizontal(lipgloss.Top,
    sidebarView,
    contentView,
)

// Vertical stack (header / content / statusbar)
view := lipgloss.JoinVertical(lipgloss.Left,
    headerView,
    mainContent,
    statusBarView,
)

// Measure rendered height/width (handles ANSI escape codes correctly)
w := lipgloss.Width(renderedString)
h := lipgloss.Height(renderedString)
_, _ = lipgloss.Size(renderedString)

// Place content with whitespace padding (centering dialogs, overlays)
centered := lipgloss.Place(termWidth, termHeight,
    lipgloss.Center, lipgloss.Center,
    content)
```

### Handling Terminal Resize

Bubbletea sends `tea.WindowSizeMsg` whenever the terminal is resized. Update all dimension-dependent components:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.viewport.Width = msg.Width - sidebarWidth
    m.viewport.Height = msg.Height - headerHeight - statusBarHeight
    m.sidebar.Height = msg.Height - headerHeight - statusBarHeight
```

This must be handled on startup (the first `WindowSizeMsg`) as well as on resize. Do not assume a default terminal size.

### Dimension Calculation Rules

- Always subtract border thickness (2 per bordered panel: 1 top + 1 bottom or 1 left + 1 right) from content dimensions
- Use `lipgloss.Height()` to measure rendered content height — do not hard-code line counts
- Sidebar: fixed width (e.g. 30 columns) or percentage (e.g. 25% of terminal width)
- Never auto-wrap text inside bordered panels — truncate explicitly to avoid layout corruption
- Account for padding set via `lipgloss.Style` `.Padding()` or `.PaddingLeft()` etc. when calculating available content width

### Focus Indicators

Active and inactive panels must be visually distinct:

```go
var (
    activeBorderColor   = lipgloss.Color("63")  // bright
    inactiveBorderColor = lipgloss.Color("240") // dimmed
)

activeStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(activeBorderColor)

inactiveStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(inactiveBorderColor)
```

Apply the appropriate style in `View()` based on `m.focused`.

---

## Status Bar Design

The status bar sits at the bottom of the screen, fixed at 1-2 lines. It communicates the current mode, navigation context, and available key hints.

### Layout

- **Left**: Current mode indicator — `[NORMAL]`, `[INSERT]`, `[SEARCH]`, `[COMMAND]`
- **Center**: Context information — current location path, item count, selection count
- **Right**: Short key hints — `? for help` or active binding reminders

Use `lipgloss.JoinHorizontal` with spacers to achieve left/center/right alignment:

```go
func (m Model) statusBar() string {
    mode := fmt.Sprintf("[%s]", m.modeName())

    context := fmt.Sprintf("%s  %d items", m.currentPath(), len(m.items))

    hint := "? for help"

    // Fill remaining width with spaces
    spacer := strings.Repeat(" ",
        max(0, m.width-lipgloss.Width(mode)-lipgloss.Width(context)-lipgloss.Width(hint)))

    return lipgloss.JoinHorizontal(lipgloss.Top,
        modeStyle.Render(mode),
        contextStyle.Render(context),
        spacer,
        hintStyle.Render(hint),
    )
}
```

---

## Search and Filter UX

### Activating Search

`/` in Normal mode transitions to Search mode. The search input renders at the bottom of the screen, between the content and the status bar.

As the user types, the list filters in real-time (prefix or fuzzy match depending on requirements). The cursor position in the list resets to the first match.

### Using bubbles/textinput

```go
import "github.com/charmbracelet/bubbles/textinput"

type Model struct {
    searchInput textinput.Model
    // ...
}

func NewModel() Model {
    ti := textinput.New()
    ti.Placeholder = "search..."
    ti.CharLimit = 128
    return Model{searchInput: ti}
}
```

Activate and deactivate the input on mode transitions:

```go
case ModeSearch:
    m.searchInput.Focus()
case ModeNormal:
    m.searchInput.Blur()
    m.searchInput.Reset()
```

### Search Mode Key Handling

```go
func (m Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "esc":
        m.mode = ModeNormal
        m.searchInput.Reset()
        m.filterQuery = ""
        return m, nil
    case "enter":
        m.mode = ModeNormal
        // confirm selection at current filtered position
        return m, nil
    default:
        var cmd tea.Cmd
        m.searchInput, cmd = m.searchInput.Update(msg)
        m.filterQuery = m.searchInput.Value()
        return m, cmd
    }
}
```

`ESC` clears the search and returns to Normal mode. `Enter` confirms the current selection without clearing the filter (let the Normal mode handler reset it if needed).

---

## Visual Selection

`v` in Normal mode enters a visual selection state. Track selection with a start index and a cursor index:

```go
type Model struct {
    cursor        int
    selectStart   int
    inVisual      bool
    selectedItems []int // indices of selected rows
}
```

In Visual mode, `j`/`k` extend the selection. Highlight selected rows with a distinct background color using lipgloss. On `y` (yank) or `d` (delete), operate on the full selection range and return to Normal mode.

In an inventory context, visual selection enables bulk move operations: select multiple items, then invoke a move command that targets all of them.

---

## Panel Focus Management

### Panel Enum

```go
type Panel int

const (
    PanelSidebar Panel = iota
    PanelContent
    PanelDetail
)

const numPanels = 3
```

### Cycling Focus

```go
type Model struct {
    focused Panel
}

// In Update(), Tab to cycle panels:
case key.Matches(msg, m.keys.NextPanel):
    m.focused = (m.focused + 1) % numPanels

case key.Matches(msg, m.keys.PrevPanel):
    m.focused = (m.focused + numPanels - 1) % numPanels
```

### Routing Key Events to the Focused Panel

Only the focused panel receives key input. Unfocused panels update only on data changes or window resize:

```go
switch m.focused {
case PanelSidebar:
    m.sidebar, cmd = m.sidebar.Update(msg)
case PanelContent:
    m.content, cmd = m.content.Update(msg)
case PanelDetail:
    m.detail, cmd = m.detail.Update(msg)
}
```

Pass `WindowSizeMsg` to all panels regardless of focus, since every panel needs to know the terminal dimensions.

---

## Vim Key Convention Reference

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `h` / `←` | Move left / go up a level |
| `l` / `→` | Move right / enter / expand |
| `g` | Go to top |
| `G` | Go to bottom |
| `Ctrl+d` | Half page down |
| `Ctrl+u` | Half page up |
| `/` | Enter search |
| `n` / `N` | Next / previous search result |
| `ESC` | Cancel / back to Normal |
| `Enter` | Confirm / select |
| `q` | Close / quit pane |
| `?` | Toggle help |
| `:` | Command mode |
| `i` | Enter insert / edit mode |
| `r` | Rename / quick-edit |
| `d` | Delete |
| `y` | Yank (copy) |
| `p` | Paste |
| `Tab` | Next panel focus |
| `Shift+Tab` | Previous panel focus |
| `Ctrl+c` | Hard quit |

---

## Implementation Checklist

When building a Vim-style Bubbletea TUI, verify:

- [ ] `ESC` transitions from any mode to Normal without side effects
- [ ] `hjkl` work alongside arrow keys for all directional movement
- [ ] Mode indicator visible in status bar at all times
- [ ] `?` toggles full help overlay; short help always visible
- [ ] Key bindings use `bubbles/key` with `WithHelp` labels on every binding
- [ ] Disabled bindings excluded from help display via `SetEnabled(false)`
- [ ] `tea.WindowSizeMsg` handled and propagated to all sub-components
- [ ] Border thickness accounted for in all width/height calculations
- [ ] Active panel has distinct border color from inactive panels
- [ ] Tab/Shift+Tab cycle panel focus
- [ ] Search input uses `bubbles/textinput` and activates on `/`
- [ ] `q` closes current pane/returns to parent, not hard quit from nested views
