# User Request: Replace UUIDs with nanoid

## Summary
Refactor the project to replace all use of UUIDs with nanoid.

## Requirements
- nanoid length is 10 characters
- Completely remove any use of UUID package
- Migrate existing database use of UUID to nanoids (generate new IDs for all items/locations in active DB)
- Document the change to nanoid and any mitigation steps the user needs to take

## Key Constraints
- All existing data must be migrated (no data loss)
- nanoid IDs should be 10 characters long
- The UUID package must be completely removed from go.mod/go.sum
- User-facing documentation explaining migration steps is required
