# TUI uses a mode enum for view switching, not separate tea.Program instances

When adding interactive actions (forms, confirm prompts, history view, scry search) to the TUI, we needed a way to swap what is rendered and what `Update` routes. We use a `tuiMode` integer enum on the single `Model` struct rather than launching separate `tea.Program` instances or composing separate top-level bubbles.

The modes are: `modeBrowse` (the existing two-pane navigator), `modeForm` (text-input form for `add`/`loan`/`borrow`), `modeConfirm` (y/n prompt with optional inline note for `lost`/`found`/`return`), `modeHistory` (scrollable event timeline), and `modeScry` (inventory-wide Levenshtein search).

`View()` dispatches on `m.mode`; `Update()` routes `tea.KeyPressMsg` to the active sub-model. All sub-models (`formModel`, `confirmModel`, `historyModel`, `scryModel`) are value fields on `Model` and are initialized when their mode is entered.

## Consequences

- **Keybinding gates are evaluated in `modeBrowse` only.** Sub-models handle their own input exclusively while active; the browse keymap is unreachable from other modes.
- **`modeConfirm` interaction:** `[y]` is pre-selected; `Enter` or `y` submits; `n`/`Esc` cancels; any other printable character feeds an inline note field directly — no Tab-cycling between widgets.
- **Post-mutation refresh** reloads the current level and attempts to reposition the cursor on the affected entity by ID. If the entity is absent from the refreshed list (e.g. `return` on a `borrowed` entity sets it to `removed`, which `GetChildren` excludes), the cursor falls back to `ResetSelected()`.
- **Scry navigation** exits `modeScry`, loads the matched entity's parent level, and positions the cursor on that entity by ID — same mechanism as post-mutation refresh.

## Considered Options

**Separate `tea.Program` per action.** Rejected — switching programs requires tearing down and restarting the terminal, losing the spatial context of where the user was browsing.

**Composable top-level bubbles (the `composable-views` pattern).** Rejected — adds indirection without benefit for a single-window TUI. The mode enum is simpler and the sub-models are small enough that the `Model` struct remains coherent.
