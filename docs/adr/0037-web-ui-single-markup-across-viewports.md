# Web UI uses a single markup structure across viewports

The web UI serves the same templates to every screen size. Responsiveness comes from CSS media queries (one `48rem` width breakpoint, plus `pointer: coarse` for touch-target sizing) and a single `tree.js` toggle that slides the tree panel in as a drawer on narrow screens. There are no mobile-specific templates, no server-side viewport detection, and no markup that is rendered only to be hidden at some widths. Features needed for phone navigation therefore ship to desktop too: the children list on entity detail, the home detail at `/` (roots as a children list), and the root-level actions (Add root, Borrow) that moved out of the header into the detail actions. The server cannot see the viewport, and keeping one structure means each handler test covers every viewport and each feature is built once.

## Considered Options

- **Separate mobile templates / viewport detection** — rejected: doubles template and test surface, and user-agent sniffing is unreliable.
- **Render both variants, hide one with CSS** — rejected: fetches and renders data the viewer never sees, and handler tests can no longer assert what the user actually gets.

## Consequences

- Desktop gains the children list and a home landing page, which duplicate the tree on purpose.
- Header shortcuts for Add root and Borrow are gone on desktop as well; if that proves painful, restoring them on wide viewports only would be a deliberate exception to this ADR.
