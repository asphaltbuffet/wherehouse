# TUI uses a mode enum for view switching, not separate tea.Program instances

When adding interactive actions (forms, confirm prompts, history view, scry search) to the TUI, we needed a way to swap what is rendered and what `Update` routes. We use a `tuiMode` integer enum on the single `Model` struct rather than launching separate `tea.Program` instances or composing separate top-level bubbles.

The modes are: `modeBrowse` (the two-pane navigator), `modeForm` (text-input form for `add`/`loan`/`borrow`), `modeConfirm` (y/n prompt with optional inline note for `lost`/`found`/`return`), and `modeScry` (inventory-wide Levenshtein search). History is not a separate mode — see below.

`View()` dispatches on `m.mode`; `Update()` routes `tea.KeyPressMsg` to the active sub-model. All sub-models (`formModel`, `confirmModel`, `historyModel`, `scryModel`) are value fields on `Model` and are initialized when their mode is entered.

## Consequences

- **Keybinding gates are evaluated in `modeBrowse` only.** Sub-models handle their own input exclusively while active; the browse keymap is unreachable from other modes.
- **`modeConfirm` interaction:** `Enter` submits; `Esc` cancels; any other printable character (including `y` and `n`) feeds an inline note field directly — no Tab-cycling between widgets. `confirmModel` owns its own cancellation: it emits `confirmCancelledMsg` on `Esc`; `updateConfirm` handles only outcomes (`confirmCancelledMsg`, `actionDoneMsg`) and does not intercept key presses directly.
- **Post-mutation refresh** reloads the current level and attempts to reposition the cursor on the affected entity by ID. If the entity is absent from the refreshed list (e.g. `return` on a `borrowed` entity sets it to `removed`, which `GetChildren` excludes), the cursor falls back to `ResetSelected()`.
- **Scry navigation** exits `modeScry`, loads the matched entity's parent level, and positions the cursor on that entity by ID — same mechanism as post-mutation refresh.
- **History lives in the right pane, not a mode.** `H` toggles a history panel inside `modeBrowse`'s right pane — the same pane used by the detail view (`d`). The two are mutually exclusive: pressing `H` while detail is showing replaces it with history, and vice versa; pressing the active key again hides the pane. `pgup`/`pgdn` scroll the history viewport while it is visible and are no-ops otherwise. `modeHistory` was removed; `historyModel` remains as a pane renderer only.
- **History auto-refreshes on any level load.** After any level-load message (`rootsLoadedMsg`, `childrenLoadedMsg`, `levelRestoredMsg`, `childRefreshMsg`, `scryNavigatedMsg`) settles and `loadLevel` positions the cursor, if `rightPane == rightPaneHistory` the model reinitialises `historyModel` for the newly selected entity and fires `loadCmd()`. This keeps the history pane coherent during drill-down, drill-up, post-mutation refresh, and scry navigation.
- **History loads use a generation counter.** Each `historyModel` carries a `gen int` stamp incremented on every reinitialisation. `historyLoadedMsg` carries the `gen` it was fired for; `updateBrowse` discards responses where `msg.gen != m.history.gen`. This prevents out-of-order responses from rapid Up/Down navigation from briefly displaying history for the wrong entity.
- **`q` always quits.** The history pane is not a mode; `q` is not intercepted by pane state. Dismiss history with `H`.

## Considered Options

**Separate `tea.Program` per action.** Rejected — switching programs requires tearing down and restarting the terminal, losing the spatial context of where the user was browsing.

**Composable top-level bubbles (the `composable-views` pattern).** Rejected — adds indirection without benefit for a single-window TUI. The mode enum is simpler and the sub-models are small enough that the `Model` struct remains coherent.
