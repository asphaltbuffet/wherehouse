# User Request: `wherehouse found` Command

## Summary
Add `wherehouse found <item>... --in <location>` command.

## Behavior
- Sets `current_location` to `location` but home is NOT changed
- `--return` flag does an additional move back to item's home

## Examples
```bash
# Found the item somewhere - just update its current location
wherehouse found "10mm socket" --in "garage"

# Found the item AND want to return it to its home location
wherehouse found "10mm socket" --in "garage" --return
```

## Notes
- Multiple items should be supported (`<item>...` variadic)
- This differs from `move` in that `home_location` is preserved
- `--return` is a convenience flag that combines found + return-to-home semantics
