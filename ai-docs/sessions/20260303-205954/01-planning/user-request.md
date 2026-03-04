# User Request

Take the result of the code review at:
  `/home/grue/dev/wherehouse/ai-docs/research/reviews/cmd-cli-consistency-review.md`

and create a plan to address all of the issues in the recommended refactoring order.

## Refactoring Order (from review)

1. Extract `cli.AddLocations` (mirrors existing `cli.AddItems`)
2. Standardize dependency injection across all commands using the `move` pattern
3. Adopt `OutputWriter` in the 4 bypassing commands (`find`, `scry`, `history`, `initialize`)
4. Replace the inline relative-time logic in `history` with `cli.FormatRelativeTime`
5. Normalize help text structure across commands

## Source Review

The full code review with detailed findings (file paths, line numbers, categorized issues) is at:
  `ai-docs/research/reviews/cmd-cli-consistency-review.md`

The review covers four concern areas:
- Output formatting and use of a common OutputWriter
- Exclusion of business logic from CLI layer
- Mocking of dependencies for isolated, efficient testing
- Consistent help text
