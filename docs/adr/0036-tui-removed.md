# TUI removed in favour of the web UI

The interactive TUI (ADRs 0025–0029) is removed. The web UI (`wherehouse serve`) covers the same browsing and editing use cases and is already planned to match the CLI, so a second interactive front end duplicates that work. Every feature would have had to be built and tested twice, and the TUI needed its own dependencies (`bubbletea`, `bubbles`), styles, and store queries. Removing it simplifies the codebase for future maintenance.

Bare `wherehouse` with no subcommand reverts to the Cobra default of printing help.

## Consequences

- `internal/tui/`, `cmd/tui/`, the `TUI*` styles, and the `charm.land/bubbletea` / `charm.land/bubbles` dependencies are deleted.
- ADRs 0025–0029 are marked superseded by this ADR and kept as history. TUI mentions in other ADRs (e.g. 0030) are left as written.
