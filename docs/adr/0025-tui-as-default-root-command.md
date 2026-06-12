# TUI as default root command using bubbletea and bubbles/list

When `wherehouse` is invoked without a subcommand, it launches a read-only interactive TUI for navigating the inventory hierarchy. We set `RunE` on the root cobra command (previously `nil`, falling through to help text) rather than adding a named subcommand, because the TUI is the natural default experience — typing the bare binary name should show you your inventory, not a help screen.

## Package structure

All TUI logic lives in `internal/tui/`, mirroring the `internal/web/` pattern. The cobra wiring is a thin `cmd/tui/` launcher following the `NewTUICmd(a *app.App)` / `NewDefaultTUICmd()` constructor pattern (ADR 0013). `cmd/serve/` is the precedent: it is a shell only; all logic lives in `internal/`.

## Framework choice

We use `github.com/charmbracelet/bubbletea` with `github.com/charmbracelet/bubbles/list` rather than a custom list implementation. The `bubbles/list` component provides scrolling and fuzzy filtering (via `sahilm/fuzzy`) with no extra wiring beyond implementing `FilterValue() string` on items. Filtering is an anticipated near-term requirement, so adopting `bubbles/list` now avoids a later rewrite. A custom `ItemDelegate` is used for single-line rendering (`▶ DisplayName [status]`) instead of `DefaultDelegate`'s two-line layout.

## Store addition

`GetRootEntities` is added to `internal/store` and exposed via `app.GetRootEntities(ctx)` to fetch depth-0 entities (those with `parent_id IS NULL`). The existing `GetChildren` covers all deeper levels, so navigation is uniform: every level uses one of these two calls.

## Considered options

- **Named subcommand (`wherehouse tui`)**: rejected — it buries the primary experience behind an extra word and is inconsistent with the intent that bare invocation is useful.
- **Custom list widget**: rejected in favour of `bubbles/list` because filtering would require reimplementing what bubbles provides for free.
