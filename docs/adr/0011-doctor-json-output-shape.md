# Doctor --json output shape

`wherehouse doctor --json` emits a single JSON object covering all check results:

```json
{
  "healthy": true,
  "issue_count": 0,
  "issues": [],
  "rebuilt": 5
}
```

## Field decisions

### `healthy` (bool)

`true` when `issues` is empty. Provides a fast pass/fail signal without array traversal. Derived from `issue_count == 0` but included explicitly for machine consumers that branch on a single key.

### `issue_count` (int)

Redundant with `len(issues)` but included deliberately: it is trivial to produce and saves every consumer the array-length computation. The cost of the extra field is negligible; the benefit compounds across all consumers.

### `issues` (array, never null)

Always an array (empty, not null, when no issues). Null arrays require defensive nil-checks in every consumer; an empty array does not.

Each element:
- `kind` — `"config"`, `"event_log"`, or `"projection"`. Lets consumers filter by check layer without parsing `description`.
- `event_id` — integer when the issue is tied to a specific event row, `null` otherwise. Never omitted.
- `description` — human-readable detail string.

### `rebuilt` (int, omitted when not applicable)

Present only when `TruncateAndReplay` actually ran and completed. **Omitted** (not `null`, not `0`) when `--rebuild` was not passed or when the rebuild was skipped due to issues without `--force`.

The presence/absence of the key is meaningful: present means "a rebuild happened and processed N events"; absent means "no rebuild occurred this run." Using `null` or `0` would conflate "rebuild ran and processed nothing" with "rebuild did not run," which are distinct states.

## Exit code

Non-zero when `healthy == false`, regardless of `--json`. The JSON object and the exit code are independent signals for different consumers.

## Single document

All check results — config, event-log, projection, and rebuild — appear in one JSON object. Streaming individual findings as separate JSON lines was rejected: there is one exit code (pass/fail) and the object's `healthy` field represents the same single verdict. A single document maps naturally to that model.
