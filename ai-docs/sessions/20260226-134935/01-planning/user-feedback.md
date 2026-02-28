# User Feedback on Plan

1. **No quiet mode**: Remove `--quiet` flag — the entire purpose of this command is to display data, so quiet mode is nonsensical here.

2. **Sub-location hints show both counts**: Child location hints should show item count AND location count (like recursive), but without listing contents. Example: `[Shelf A] (1 item, 3 locations)` — not just `[Shelf A] (1 item)`.

3. **Use go-humanize for pluralization**: Use the `english.PluralWord` function from the `github.com/dustin/go-humanize` package instead of writing custom pluralization. Example: `english.PluralWord(childCount, "sub-location", "")`.

4. **Use external tree rendering package**: Do not write a custom `renderTree` function — use an existing external package for tree rendering.

5. **No nolint directive on cobra.ArbitraryArgs**: `Args: cobra.ArbitraryArgs,` does not trigger any linter warning and does not need a `//nolint` directive.

6. **Not-found locations render gracefully**: If a location argument cannot be resolved, do NOT return an error. Instead, display it inline with the others as `Bad Shelf [not found]` and continue rendering any other valid locations.
