# Charm Ecosystem for Go TUI Development

Reference for Go developers building a Bubbletea TUI inventory application. Covers all Charm libraries, component APIs, and selection guidance.

---

## The Charm Stack

All Charm libraries form an integrated ecosystem. v2 libraries use the vanity domain `charm.land/`; v1 libraries use `github.com/charmbracelet/`.

| Library | Purpose | Import (v1 stable) |
|---------|---------|-------------------|
| **Bubble Tea** | Core TUI framework (TEA architecture) | `github.com/charmbracelet/bubbletea` |
| **Lip Gloss** | Styling and layout | `github.com/charmbracelet/lipgloss` |
| **Bubbles** | Reusable TUI components | `github.com/charmbracelet/bubbles` |
| **Huh** | Forms and structured user input | `github.com/charmbracelet/huh` |
| **Glamour** | Stylesheet-driven Markdown renderer | `github.com/charmbracelet/glamour` |
| **Wish** | Build SSH apps with Bubble Tea | `github.com/charmbracelet/wish` |
| **Harmonica** | Physics-based animation toolkit | `github.com/charmbracelet/harmonica` |
| **Log** | Structured terminal logger | `github.com/charmbracelet/log` |

### Dependency Versions (as of 2025)

- **Bubbletea v1**: `github.com/charmbracelet/bubbletea` — stable, production-ready
- **Bubbletea v2**: `charm.land/bubbletea/v2` — RC stage in early 2025; API finalized but requires package path migration
- **Bubbles**: `github.com/charmbracelet/bubbles` — v1 stable
- **Lipgloss**: `github.com/charmbracelet/lipgloss` — v1 stable

For new projects: v1 is safe. v2 API is finalized but adopting it requires migrating all import paths to `charm.land/`.

---

## Lip Gloss — Styling and Layout

```
github.com/charmbracelet/lipgloss    (v1)
charm.land/lipgloss/v2               (v2)
```

### Styles

```go
style := lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("63")).       // 256-color ANSI
    Background(lipgloss.Color("#FAFAFA")).  // hex RGB (truecolor)
    Padding(1, 2).                           // vertical, horizontal
    Margin(0, 1).
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("63")).
    Width(30).
    Height(10).
    Align(lipgloss.Center).
    MaxWidth(80)

rendered := style.Render("content")
```

### Color Types

| Syntax | Type |
|--------|------|
| `lipgloss.Color("63")` | 256-color ANSI |
| `lipgloss.Color("#FAFAFA")` | Hex RGB (truecolor) |
| `lipgloss.Color("9")` | Standard 16 ANSI colors (0–15) |

v2 behavior: colors are auto-downsampled to the terminal's capability.

### Border Styles

```go
lipgloss.NormalBorder()
lipgloss.RoundedBorder()
lipgloss.BlockBorder()
lipgloss.ThickBorder()
lipgloss.DoubleBorder()
lipgloss.HiddenBorder()
```

Selective borders:

```go
// Positional: top, right, bottom, left
style.Border(lipgloss.NormalBorder(), true, false, true, false)

// Individual sides
style.BorderTop(true).BorderLeft(true)
```

### Measurement

Required for correct layout — handles ANSI escape codes.

```go
w := lipgloss.Width(renderedStr)    // width in terminal columns
h := lipgloss.Height(renderedStr)   // height in lines
w, h := lipgloss.Size(renderedStr)  // both at once
```

### Layout Composition

```go
// Horizontal split
combined := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, rightBlock)

// Vertical stack
stacked := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
```

Alignment constants:

| Constant | Value |
|----------|-------|
| `lipgloss.Top` | 0.0 |
| `lipgloss.Center` | 0.5 |
| `lipgloss.Bottom` | 1.0 |
| `lipgloss.Left` | 0.0 |
| `lipgloss.Right` | 1.0 |

### Placement

```go
// Place content within a fixed-size block
centered := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
leftBottom := lipgloss.Place(w, h, lipgloss.Left, lipgloss.Bottom, content)

// Single-axis placement
hCentered := lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
vBottom := lipgloss.PlaceVertical(height, lipgloss.Bottom, content)

// With styled whitespace
block := lipgloss.PlaceHorizontal(80, lipgloss.Center, text,
    lipgloss.WithWhitespaceBackground(lipgloss.Color("240")))
```

### Common Patterns

Active/inactive panel borders:

```go
activeStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("63"))

inactiveStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("240"))
```

Status bar with left/right sections:

```go
leftSection := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("[NORMAL]")
rightSection := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("? help")
spacer := lipgloss.NewStyle().
    Width(termWidth - lipgloss.Width(leftSection) - lipgloss.Width(rightSection)).
    Render("")
statusBar := lipgloss.JoinHorizontal(lipgloss.Top, leftSection, spacer, rightSection)
```

---

## Bubbles — Component Library

```
github.com/charmbracelet/bubbles    (v1)
charm.land/bubbles/v2               (v2)
```

All components implement the `tea.Model` interface (`Init`, `Update`, `View`). Embed them in your model struct and delegate messages via `component.Update(msg)`.

### List (`bubbles/list`)

The most feature-rich component. Ideal for browsing inventory items. Built-in: fuzzy filtering, pagination, keyboard navigation (`j`/`k`/`g`/`G`), spinner for loading, delegate pattern for custom row rendering.

```go
import "github.com/charmbracelet/bubbles/list"

// Items must implement list.Item
type Item interface {
    FilterValue() string  // used for fuzzy filter
}

// Construction
l := list.New(items, list.NewDefaultDelegate(), width, height)
l.Title = "Items"

// Configuration
l.SetShowHelp(true)
l.SetFilteringEnabled(true)   // enables fuzzy filter with /
l.SetShowStatusBar(true)
l.SetShowPagination(true)
l.Styles.Title = titleStyle

// In Update()
m.list, cmd = m.list.Update(msg)

// Get selected item
selected := m.list.SelectedItem().(MyItem)
```

### Viewport (`bubbles/viewport`)

Scrollable read-only content pane. Use for item detail views, event logs, help text.

Built-in keys: `j`/`k` scroll line, `Ctrl+d`/`Ctrl+u` half-page, `Ctrl+f`/`Ctrl+b` full-page, `g`/`G` top/bottom. Supports mouse wheel scrolling.

```go
import "github.com/charmbracelet/bubbles/viewport"

vp := viewport.New(width, height)
vp.SetContent(longText)

// In Update()
m.viewport, cmd = m.viewport.Update(msg)

// In View()
content := m.viewport.View()
```

### Text Input (`bubbles/textinput`)

Single-line input field. Use for search bars, inline rename, quick entry.

```go
import "github.com/charmbracelet/bubbles/textinput"

ti := textinput.New()
ti.Placeholder = "Search items..."
ti.CharLimit = 80
ti.Width = 40
ti.Focus()  // enable input

// Styles
ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

// Get current value
value := ti.Value()
```

### Text Area (`bubbles/textarea`)

Multi-line input field. Use for notes, descriptions, longer free-text fields.

```go
import "github.com/charmbracelet/bubbles/textarea"

ta := textarea.New()
ta.Placeholder = "Add a note..."
ta.SetWidth(40)
ta.SetHeight(6)
ta.Focus()
```

### Spinner (`bubbles/spinner`)

Animated loading indicator. Use for async operations such as DB queries and projection rebuilds.

```go
import "github.com/charmbracelet/bubbles/spinner"

s := spinner.New()
s.Spinner = spinner.Dot  // Dot, Line, MiniDot, Jump, Pulse, Points, Globe, Moon, Monkey
s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

// In Init() — must send initial tick
return s.Tick

// In Update()
m.spinner, cmd = m.spinner.Update(msg)

// In View()
fmt.Sprintf("%s Loading...", m.spinner.View())
```

### Progress (`bubbles/progress`)

Animated progress bar. Use for projection rebuild, batch import operations.

```go
import "github.com/charmbracelet/bubbles/progress"

// Gradient fill
p := progress.New(progress.WithDefaultGradient())

// Solid fill
p := progress.New(progress.WithSolidFill("63"))
p.Width = 40

// Set percentage (returns animation command)
cmd = p.SetPercent(0.75)

// In Update()
m.progress, cmd = m.progress.Update(msg)

// In View()
progressBar := m.progress.View()
```

### File Picker (`bubbles/filepicker`)

Filesystem navigation component. Useful for import/export file selection.

```go
import "github.com/charmbracelet/bubbles/filepicker"

fp := filepicker.New()
fp.AllowedTypes = []string{".json", ".csv"}
fp.CurrentDirectory, _ = os.UserHomeDir()
```

### Table (`bubbles/table`)

Tabular data display with scrolling and row selection.

```go
import "github.com/charmbracelet/bubbles/table"

columns := []table.Column{
    {Title: "Name",     Width: 30},
    {Title: "Location", Width: 20},
    {Title: "Project",  Width: 15},
}
rows := []table.Row{
    {"10mm Socket", "Garage:Toolbox", "Car Repair"},
}

t := table.New(
    table.WithColumns(columns),
    table.WithRows(rows),
    table.WithFocused(true),
    table.WithHeight(10),
)

t.SetStyles(table.Styles{
    Header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
    Selected: lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("63")),
})
```

### Help (`bubbles/help`)

Renders a key binding help bar. Supports short (one-line) and full (multi-line) views.

```go
import "github.com/charmbracelet/bubbles/help"

h := help.New()
h.ShowAll = false  // start with condensed view

// Toggle in Update()
case key.Matches(msg, m.keys.Help):
    m.help.ShowAll = !m.help.ShowAll

// In View() — keys must implement help.KeyMap (ShortHelp, FullHelp methods)
helpView := m.help.View(m.keys)
```

### Key (`bubbles/key`)

Type-safe key binding definitions. Used with `bubbles/help` for documented bindings.

```go
import "github.com/charmbracelet/bubbles/key"

binding := key.NewBinding(
    key.WithKeys("k", "up"),
    key.WithHelp("↑/k", "move up"),
)

// In Update()
if key.Matches(msg, binding) {
    // handle key
}
```

### Paginator (`bubbles/paginator`)

Page state tracker. Use when implementing manual pagination outside of `bubbles/list`.

```go
import "github.com/charmbracelet/bubbles/paginator"

p := paginator.New()
p.Type = paginator.Dots    // or paginator.Arabic
p.PerPage = 10
p.SetTotalPages(len(items))

// Navigation
p.PrevPage()
p.NextPage()

// Get current page slice bounds
start, end := p.GetSliceBounds(len(items))
pageItems := items[start:end]
```

---

## Huh — Terminal Forms

For structured input flows: add item wizard, edit form, confirmation dialogs.

```go
import "github.com/charmbracelet/huh"

var name, location string
var confirmed bool

form := huh.NewForm(
    huh.NewGroup(
        huh.NewInput().
            Title("Item Name").
            Placeholder("e.g. 10mm Socket").
            Validate(func(s string) error {
                if s == "" {
                    return errors.New("name required")
                }
                return nil
            }).
            Value(&name),

        huh.NewSelect[string]().
            Title("Location").
            Options(huh.NewOptions(locationNames...)...).
            Value(&location),

        huh.NewConfirm().
            Title("Save?").
            Value(&confirmed),
    ),
)
```

### Running a Form

**Blocking (outside Bubbletea):**

```go
err := form.Run()
```

**Non-blocking (embedded in Bubbletea):**

Huh forms implement `tea.Model`. Embed the form in your model and delegate messages:

```go
// In model struct
type Model struct {
    form *huh.Form
}

// In Update()
m.form, cmd = m.form.Update(msg)

// In View()
return m.form.View()
```

---

## Glamour — Markdown Rendering

Renders Markdown to styled terminal output. Use for item notes or descriptions stored in Markdown format. Combine with `bubbles/viewport` for scrollable rendered output.

```go
import "github.com/charmbracelet/glamour"

renderer, err := glamour.NewTermRenderer(
    glamour.WithAutoStyle(),   // detects dark/light terminal
    glamour.WithWordWrap(80),
)

rendered, err := renderer.Render(markdownContent)
// Pass rendered string to viewport.SetContent()
```

---

## Testing with teatest

```
github.com/charmbracelet/x/exp/teatest
```

Note: `teatest` is experimental (under `charmbracelet/x/exp`). No backwards-compatibility guarantees.

```go
import "github.com/charmbracelet/x/exp/teatest"

func TestMyApp(t *testing.T) {
    m := initialModel()
    tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

    // Send key messages
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // Wait for expected output
    teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
        return bytes.Contains(out, []byte("expected text"))
    }, teatest.WithDuration(time.Second))

    // Assert final output against golden file
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
    finalModel := tm.FinalModel(t)
    out := tm.FinalOutput(t)
    golden.RequireEqual(t, out)
}
```

---

## VHS — Demo Recording

Generates animated GIFs and screenshots from declarative tape scripts. Not a runtime dependency.

```
# demo.tape
Output demo.gif
Set FontSize 14
Set Width 1200
Set Height 600

Type "wherehouse list"
Enter
Sleep 1s
Type "j"
Sleep 500ms
```

Run: `vhs demo.tape`

---

## Third-Party Libraries

### bubblelayout

```
github.com/winder/bubblelayout
```

Declarative layout manager for Bubbletea. Alternative to manual `JoinHorizontal`/`JoinVertical` for complex multi-pane layouts.

- Dual-pane layouts
- Accordion mode
- Configurable split ratios

### vimtea

```
github.com/kujtimiihoxha/vimtea
```

Full vim editor component for Bubbletea. Use for text-editing panels requiring modal editing.

- Modes: Normal, Insert, Visual, Command
- Navigation: `h`/`j`/`k`/`l`
- Operations: `d`/`y`/`p` (delete, yank, paste)
- Undo/redo support

---

## Wish — SSH Apps

```
github.com/charmbracelet/wish
```

Runs Bubbletea applications over SSH. The Cursed Renderer provides significant bandwidth savings for remote TUI sessions. Not required for local-only applications.

---

## Component Selection Guide

| Use Case | Component | Notes |
|----------|-----------|-------|
| Browse list of items | `bubbles/list` | Built-in fuzzy filter, pagination, keyboard nav |
| Show item detail | `bubbles/viewport` | Scrollable read-only pane |
| Search / filter bar | `bubbles/textinput` | Single-line, focus-managed |
| Add / edit item form | `huh` form | Wizard-style with validation |
| Quick single-field input | `bubbles/textinput` | Inline rename, quick add |
| Multi-line notes input | `bubbles/textarea` | Free text, configurable dimensions |
| Loading indicator | `bubbles/spinner` | Multiple animation styles |
| Progress (rebuild, import) | `bubbles/progress` | Gradient or solid fill, animated |
| Key binding help bar | `bubbles/help` + `bubbles/key` | Short/full toggle |
| Tabular item view | `bubbles/table` | Fixed columns, scrolling, row selection |
| Item notes (Markdown) | `glamour` in `viewport` | Render Markdown, scroll with viewport |
| File import/export picker | `bubbles/filepicker` | Filtered by extension |
| Manual pagination | `bubbles/paginator` | Dots or Arabic page indicator |
| Complex multi-pane layout | `bubblelayout` (third-party) | Declarative alternative to JoinHorizontal |
| Modal text editing panel | `vimtea` (third-party) | Vim bindings, undo/redo |
