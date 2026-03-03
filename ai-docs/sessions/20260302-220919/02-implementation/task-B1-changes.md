# Task B1 Changes

## Files Modified

### internal/cli/add.go
- Line 43: `"item.created"` → `database.ItemCreatedEvent`

### internal/cli/selectors_test.go
- All `"location.created"` string literals (6 occurrences) → `database.LocationCreatedEvent`
- All `"item.created"` string literals (7 occurrences) → `database.ItemCreatedEvent`
- Total: 13 replacements

## Summary

Replaced all string literal event type arguments in AppendEvent calls within internal/cli/ with typed EventType constants from the database package. No string-based APIs (e.g. EventStyle) were affected in these files.
