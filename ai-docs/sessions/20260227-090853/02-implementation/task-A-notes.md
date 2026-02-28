# Task A - Decisions and Deviations

## Followed Plan Exactly

No deviations from the final plan. The implementation matches the specification in `01-planning/final-plan.md` in all material respects.

## Implementation Notes

### Linter auto-removed nolint comment on MinimumNArgs(1)
The `golangci-lint --fix` pass removed the `//nolint:mnd` comment on `cobra.MinimumNArgs(1)`.
This suggests the linter's `mnd` rule does not trigger on `MinimumNArgs(1)` (only on
`ExactArgs(N)` per the CLAUDE.md gotcha note). No manual suppression was required.

### Warning logic uses switch statement
The plan showed an if/else chain for the missing-vs-system-vs-normal location warning.
A `switch` statement was used instead for cleaner Go style (aligns with linter preferences).

### Warnings in quiet mode
The `OutputWriter.Warning()` method already suppresses output in quiet mode, so quiet-mode
suppression of warnings is handled automatically by the output layer - no special casing needed
in `runFoundItem`.

### JSON output for warnings
In the `Result` struct, `Warnings []string` uses `json:",omitempty"` so it is omitted from
JSON output when empty (no empty array). This matches the plan's JSON example which shows
warnings only when present.

### No from_location validation for item.found event
As noted in the plan (section 5.3), `item.found` does not require `from_location_id` validation.
The existing `handleItemFound` handler only needs `item_id`, `found_location_id`, and
`home_location_id`. This is correct and intentional.
