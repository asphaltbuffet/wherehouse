# Import requires strictly ascending `event_id` order and errors on violation

The `import` command validates that `event_id` values in the NDJSON input are strictly ascending before replaying any events. If the order is non-monotonic (e.g. two export files concatenated together, or a manually shuffled file), import exits with an error and performs no DB writes.

We rejected silently sorting the input for two reasons: (1) out-of-order input almost certainly indicates user error (concatenated exports, truncated file, manual editing) — surfacing this is more honest than hiding it; (2) the path-changed validation logic relies on positional ordering — `EntityPathChangedEvent` records are expected to appear immediately after their triggering `EntityReparentedEvent` in the stream, and this assumption breaks if the stream is reordered.

The error message should direct the user to re-export from the source database rather than attempting to manually sort the NDJSON, since sorting by `event_id` alone does not restore the positional relationship between reparent and path-changed records.
