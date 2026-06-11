# EntityBorrowedEvent for atomic borrow creation

When borrowing external items into the inventory, we use a dedicated `EntityBorrowedEvent` rather than two events (`EntityCreatedEvent` + `EntityStatusChangedEvent`). The `borrow` command is an intent-driven operation distinct from `add` — the entity enters inventory already in `borrowed` status, with a lender recorded in `StatusContext`. Using a single event makes this atomic and keeps the event log readable: one event per user action.

## Considered Options

Two events (`EntityCreatedEvent` + `EntityStatusChangedEvent`) were considered. This would have avoided adding a new event type to the pipeline, but would have made the history ambiguous — a newly created entity at `ok` status briefly appears before the status change event, and the intent (borrowing vs. adding then marking borrowed) is lost.
