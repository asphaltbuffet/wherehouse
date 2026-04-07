# TODO

Issues deferred from PR #126 code review.

## [M1] ORDER BY clauses missing tiebreakers

Files: `internal/database/item.go` (approx. lines 113, 140, 234), `internal/database/location.go` (approx. lines 289, 316, 408)

Queries using `ORDER BY display_name` or `ORDER BY i.display_name` lack a tiebreaker. Per project rules, every query that could tie must include `event_id ASC/DESC` as a secondary sort. Items and locations can share display names.

## [M2] removeItem only rejects the "removed" system location

File: `cmd/remove/item.go` (approx. line 46)

The check `location.IsSystem && location.CanonicalName == "removed"` intentionally allows removal of items in Missing/Borrowed/Loaned. This decision should be documented in a comment so future maintainers understand it is deliberate rather than a bug.

## [M3] extractLocationFromEvent does not handle item.removed

File: `internal/database/search.go` (approx. lines 317–342)

`extractLocationFromEvent` does not map the `item.removed` event type, so it falls through to the default (empty return). The impact is low — the enrichment path already handles `IsRemoved` — but adding the case would make the function complete.
