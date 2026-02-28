# Bubbletea TUI Framework Architecture

## Overview

Bubbletea is Charm's Go TUI framework built around The Elm Architecture (TEA). It provides a structured, unidirectional data flow model: messages drive state updates, state drives rendering. This document covers the core architecture, v2 changes, command system, component composition, and practical patterns for production use.

---

## Import Paths

| Version | Module |
|---------|--------|
| v1 | `github.com/charmbracelet/bubbletea` |
| v2 | `charm.land/bubbletea/v2` |

Companion libraries also moved in v2:

- `charm.land/bubbles/v2`
- `charm.land/lipgloss/v2`

---

## The Elm Architecture (TEA)

Every Bubbletea program is built around three functions on a `Model`:

```go
type Model interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (tea.Model, tea.Cmd)
    View() tea.View   // v2: returns tea.View struct, not string
}
```

### Init

`Init()` is called once when the program starts. It returns an initial `tea.Cmd` (or `nil`). Use it to kick off side effects that should run at startup (fetching data, starting timers, etc.).

```go
func (m Model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestWindowSize(),
        m.spinner.Init(),
    )
}
```

### Update

`Update(msg)` is the heart of the program. It receives a message, returns the new model state and an optional command to execute. It must never block.

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        if msg.Code == tea.KeyEscape {
            return m, tea.Quit()
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    }
    return m, nil
}
```

### View

`View()` renders the current model state to the terminal. In v1 it returns a `string`. In v2 it returns a `tea.View` struct, which is a declarative description of the desired terminal state.

```go
// v1
func (m Model) View() string {
    return fmt.Sprintf("Items: %d\n", len(m.items))
}

// v2
func (m Model) View() tea.View {
    return tea.View{
        Content:     fmt.Sprintf("Items: %d\n", len(m.items)),
        AltScreen:   true,
        WindowTitle: "Wherehouse",
    }
}
```

`View()` runs on the event loop — keep it fast. No I/O, no expensive computation.

---

## v2 Key Changes (2025)

### Module Path

The module path changed from `github.com/charmbracelet/bubbletea` to `charm.land/bubbletea/v2`. This is a hard import break — v1 and v2 cannot be mixed in the same program.

### View Struct (Declarative Terminal Control)

`View()` now returns a `tea.View` struct instead of a string. This struct controls the full terminal session declaratively:

```go
type View struct {
    Content             string
    AltScreen           bool
    MouseMode           MouseMode
    ReportFocus         bool
    KeyboardEnhancements KeyboardEnhancements
    WindowTitle         string
    Cursor              CursorConfig   // Position, Color, Shape, Blink
    ForegroundColor     color.Color
    BackgroundColor     color.Color
}
```

Settings like `AltScreen`, `MouseMode`, and `KeyboardEnhancements` no longer require separate option calls or runtime commands — they are expressed per-frame in the view.

### Key Messages

`tea.KeyMsg` is replaced by two distinct message types:

- `tea.KeyPressMsg` — key pressed
- `tea.KeyReleaseMsg` — key released (requires keyboard enhancement support)

Both carry the same fields:

```go
type KeyPressMsg struct {
    Code        Key
    Text        string
    Mod         KeyMod
    BaseCode    Key
    IsRepeat    bool
    ShiftedCode Key
}
```

The `Keystroke()` helper method returns a human-readable string:

```go
case tea.KeyPressMsg:
    switch msg.Keystroke() {
    case "ctrl+c":
        return m, tea.Quit()
    case "ctrl+shift+a":
        return m, doSomething()
    case "space":
        // Note: space returns "space", not " "
        return m, toggleSelection()
    }
```

### Mouse Messages

Mouse input is split into four distinct message types:

- `tea.MouseClickMsg`
- `tea.MouseReleaseMsg`
- `tea.MouseWheelMsg`
- `tea.MouseMotionMsg`

### Paste Messages

Bracketed paste produces three messages:

- `tea.PasteStartMsg` — paste bracket opened
- `tea.PasteMsg` — the pasted content
- `tea.PasteEndMsg` — paste bracket closed

### Terminal Messages

New message types surface terminal state changes:

| Message | Description |
|---------|-------------|
| `WindowSizeMsg` | Terminal resized |
| `FocusMsg` | Terminal window gained focus |
| `BlurMsg` | Terminal window lost focus |
| `ColorProfileMsg` | Color profile detected |
| `ForegroundColorMsg` | Terminal foreground color |
| `BackgroundColorMsg` | Terminal background color |
| `EnvMsg` | Environment variables |

### Progressive Keyboard Enhancements

v2 supports extended key encoding (Kitty keyboard protocol) on compatible terminals (Kitty, Ghostty, Alacritty, iTerm2). This enables:

- `shift+enter` as a distinct key from `enter`
- `ctrl+m` as distinct from `enter`
- Key release events

Applications degrade gracefully on terminals without support.

### Renderer

The renderer was rebuilt from scratch using an ncurses-inspired diff algorithm. Key improvements:

- Significantly reduced bandwidth — important for SSH sessions via Wish
- **Synchronized updates** (Mode 2026): batches writes to reduce screen tearing
- **Wide character support** (Mode 2027): correct handling of emoji and CJK double-width characters
- **Color auto-downsampling**: detects terminal color profile and automatically downgrades colors for compatibility

### Native Clipboard

OSC52 clipboard support works over SSH without requiring host-side helpers:

```go
return m, tea.SetClipboard(selectedText)
```

---

## Commands

Commands are the mechanism for side effects. A `tea.Cmd` is a function that executes asynchronously (in a goroutine) and returns a message:

```go
type Cmd func() Msg
```

Commands must not mutate model state directly — they communicate results back through messages.

### Command Constructors

**Concurrency control:**

```go
tea.Batch(cmds ...Cmd)     // run all commands concurrently
tea.Sequence(cmds ...Cmd)  // run commands serially, in order
```

Use `Sequence` when a command's message must be processed before the next command starts.

**Timers:**

```go
tea.Tick(d time.Duration, fn func(time.Time) Msg) Cmd
tea.Every(d time.Duration, fn func(time.Time) Msg) Cmd
```

**Process execution:**

```go
tea.ExecProcess(c *exec.Cmd, fn func(error) Msg) Cmd
```

Suspends the TUI, runs the process with full terminal control (useful for `$EDITOR`), then resumes.

**Output:**

```go
tea.Println(args ...interface{}) Cmd
tea.Printf(format string, args ...interface{}) Cmd
```

Prints above the TUI output. Safe to call from Update.

**Clipboard:**

```go
tea.SetClipboard(s string) Cmd
tea.ReadClipboard() Cmd
```

**Program control:**

```go
tea.Quit() Cmd
tea.Interrupt() Cmd  // sends SIGINT
tea.Suspend() Cmd    // SIGTSTP / Ctrl+Z
tea.ClearScreen() Cmd
```

**Terminal queries:**

```go
tea.RequestWindowSize() Cmd
tea.RequestCursorPosition() Cmd
tea.RequestForegroundColor() Cmd
tea.RequestBackgroundColor() Cmd
tea.RequestTerminalVersion() Cmd
```

### Command Execution Model

Commands run in parallel goroutines. Messages from concurrent commands arrive in non-deterministic order. Design `Update()` to handle messages in any sequence. Use `tea.Sequence` only when ordering is critical.

---

## Program Setup

```go
p := tea.NewProgram(initialModel())
if _, err := p.Run(); err != nil {
    log.Fatal(err)
}
```

### Options

| Option | Purpose |
|--------|---------|
| `WithInput(r io.Reader)` | Override stdin |
| `WithOutput(w io.Writer)` | Override stdout |
| `WithContext(ctx)` | Attach cancellation context |
| `WithFPS(fps int)` | Renderer frame rate cap |
| `WithWindowSize(w, h int)` | Force fixed terminal size |
| `WithFilter(fn)` | Intercept/transform messages before Update |
| `WithColorProfile(p)` | Override color profile detection |
| `WithoutRenderer()` | Headless mode (no terminal output) |
| `WithoutSignalHandler()` | Disable default SIGINT/SIGTERM handling |

### Program Methods

```go
p.Run()                 // start and block until exit
p.Send(msg tea.Msg)     // inject a message from outside the event loop
p.Quit()                // request graceful shutdown
p.Kill()                // immediate shutdown
p.Wait()                // wait for program to exit
p.ReleaseTerminal()     // temporarily yield terminal control
p.RestoreTerminal()     // reclaim terminal
p.Println(args...)      // print above TUI output
p.Printf(fmt, args...)  // print above TUI output
```

`p.Send()` is safe to call from other goroutines. Use it to push external events (e.g., from a background watcher) into the program.

### Error Types

```go
tea.ErrInterrupted    // Ctrl+C
tea.ErrProgramKilled  // Kill() was called
tea.ErrProgramPanic   // panic in Update or View
```

---

## Nested Component Architecture

Any non-trivial application outgrows a single model. Bubbletea has no built-in component system — composition is done by embedding sub-models into parent models.

### Embedding Sub-Models

```go
type RootModel struct {
    activeScreen Screen
    list         list.Model
    detail       DetailModel
    help         help.Model
    width        int
    height       int
}
```

Each sub-model implements `Init()`, `Update()`, and `View()` in isolation.

### Init: Batching Children

```go
func (m RootModel) Init() tea.Cmd {
    return tea.Batch(
        m.list.Init(),
        m.detail.Init(),
        m.help.Init(),
    )
}
```

### Update: Message Routing

The root `Update()` acts as a message router and screen compositor. There are three routing paths:

```go
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {

    // 1. Global keys — handled directly, never forwarded
    case tea.KeyPressMsg:
        switch msg.Keystroke() {
        case "ctrl+c", "q":
            return m, tea.Quit()
        case "?":
            m.showHelp = !m.showHelp
            return m, nil
        }

    // 3. Broadcast messages — sent to all children
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        var cmd tea.Cmd
        m.list, cmd = m.list.Update(msg)
        cmds = append(cmds, cmd)
        m.detail, cmd = m.detail.Update(msg)
        cmds = append(cmds, cmd)
        return m, tea.Batch(cmds...)
    }

    // 2. Route to active child
    var cmd tea.Cmd
    switch m.activeScreen {
    case ScreenList:
        m.list, cmd = m.list.Update(msg)
        cmds = append(cmds, cmd)
    case ScreenDetail:
        m.detail, cmd = m.detail.Update(msg)
        cmds = append(cmds, cmd)
    }

    return m, tea.Batch(cmds...)
}
```

Messages flow down the tree; commands bubble up via `tea.Batch`.

### View: Compositing

```go
func (m RootModel) View() tea.View {
    var content string
    switch m.activeScreen {
    case ScreenList:
        content = m.list.View()
    case ScreenDetail:
        content = m.detail.View()
    }
    if m.showHelp {
        content = lipgloss.JoinVertical(lipgloss.Left, content, m.help.View())
    }
    return tea.View{
        Content:     content,
        AltScreen:   true,
        WindowTitle: "Wherehouse",
    }
}
```

### Model Stack Alternative

Rather than embedding all models statically in the root, use a dynamic stack:

```go
type StackModel struct {
    stack []tea.Model
}

func (m StackModel) current() tea.Model {
    return m.stack[len(m.stack)-1]
}
```

Define semantic commands to push/pop screens:

```go
type OpenDetailCmd struct{ ItemID string }
type CloseCmd struct{}
```

The stack's `Update()` handles these commands to push a new model or pop the current one. Closed models are garbage collected. Models can open other models without knowing about their siblings. This pattern scales well for deep navigation hierarchies.

---

## Performance Considerations

| Concern | Guidance |
|---------|---------|
| `Update()` must not block | All I/O must be in `tea.Cmd` goroutines |
| `View()` must not block | Pure render from model state only |
| Expensive computation | Return a `tea.Cmd`; deliver result as a message |
| Ordered side effects | Use `tea.Sequence` instead of `tea.Batch` |
| Parallel commands | Messages from `tea.Batch` arrive in unpredictable order; design Update accordingly |
| SQLite with `MaxOpenConns(1)` | Never call the DB inside a loop that also iterates `*sql.Rows` — deadlock; use JOINs/subqueries instead |

---

## Debugging

### Log Messages to File

Bubbletea takes over the terminal, so `fmt.Println` is not usable during development. Redirect structured output to a file:

```go
import "github.com/davecgh/go-spew/spew"

f, _ := os.OpenFile("debug.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
log.SetOutput(f)

// In Update:
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    log.Printf("%s", spew.Sdump(msg))
    // ...
}
```

Then in a second terminal:

```sh
tail -f debug.log
```

### Terminal Recovery

If the program crashes and leaves the terminal in raw mode:

```sh
reset
```

### Testing with `teatest`

The `teatest` library provides end-to-end test helpers:

```go
func TestApp(t *testing.T) {
    p := teatest.NewTestProgram(t, initialModel(),
        teatest.WithInitialTermSize(80, 24),
    )

    // Send keystrokes
    p.Send(tea.KeyPressMsg{Code: tea.KeyDown})
    p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

    // Poll for expected output within timeout
    teatest.WaitFor(t, p.Output(), func(bts []byte) bool {
        return bytes.Contains(bts, []byte("Item selected"))
    }, teatest.WithDuration(3*time.Second))

    // Assert against golden file
    p.Send(tea.KeyPressMsg{Code: tea.KeyRune, Rune: 'q'})
    p.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
    teatest.RequireEqualOutput(t, p.FinalOutput(t))
}
```

Use `WithWindowSize(w, h)` on the program or `WithInitialTermSize` on `teatest` to produce deterministic output for golden file comparison.

---

## Practical Notes

### Choose v1 or v2 Before Starting

v1 and v2 have incompatible module paths and cannot coexist. Decide upfront. v2 is the forward-looking choice for new projects in 2025 and provides the declarative `View` struct, improved key handling, and the rebuilt renderer.

### Keep the Event Loop Clean

`Update()` and `View()` are synchronous and block the event loop. Any operation that could take more than a microsecond belongs in a `tea.Cmd`. This includes all database queries, file I/O, network calls, and anything that acquires a mutex.

### Design for Message Ordering

`tea.Batch` runs commands concurrently. Do not assume message delivery order when batching independent commands. If order matters, use `tea.Sequence`.

### SQLite Single Connection

If the application uses SQLite with `sql.DB.SetMaxOpenConns(1)`, any call pattern where a `*sql.Rows` is open while another query executes will deadlock. Resolve all data needs in a single query using JOINs or subqueries. Never issue a secondary query inside a `for rows.Next()` loop.

### Sub-Model or Stack

Use embedded sub-models when the set of screens is small and fixed. Use a model stack when screens open other screens dynamically or when the navigation graph is not known at compile time. The stack approach avoids a single bloated root model but requires discipline in command semantics.

### p.Send() for External Events

Use `p.Send(msg)` to inject messages from goroutines running outside the Bubbletea event loop — background file watchers, timers not managed by `tea.Tick`, or results from long-running operations initiated before the program started.

### Keystroke Normalization

In v2, space bar produces `"space"` from `Keystroke()`, not `" "`. Check with `msg.Keystroke() == "space"` rather than `msg.Text == " "` or checking the rune value.
