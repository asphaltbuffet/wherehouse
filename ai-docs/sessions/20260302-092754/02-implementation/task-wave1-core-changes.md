# Wave 1 Core Changes

## Files Created

### `/home/grue/dev/wherehouse/internal/nanoid/nanoid.go`
- New package `nanoid`
- Imports `github.com/matoous/go-nanoid/v2`
- Exports: `Alphabet` constant (62-char A-Za-z0-9), `IDLength = 10`, `New() (string, error)`, `MustNew() string`

### `/home/grue/dev/wherehouse/go.mod` (modified)
- Added `github.com/matoous/go-nanoid/v2 v2.1.0`

### `/home/grue/dev/wherehouse/go.sum` (modified)
- Added checksums for `github.com/matoous/go-nanoid/v2 v2.1.0`

## Files Modified

### `/home/grue/dev/wherehouse/internal/cli/selectors.go`
- Removed `github.com/google/uuid` import (no longer needed)
- Renamed `LooksLikeUUID` to `LooksLikeID`
- Updated detection logic: 10-char alphanumeric (A-Za-z0-9) instead of 36-char UUID format
- Updated `ResolveLocation`: always tries direct DB ID lookup before canonical name fallback
- Updated `ResolveItemSelector`: `LooksLikeID` gates fast-path with error-on-not-found; non-colon strings also try DB lookup before name resolution (preserves backward compat with UUID-format IDs in tests)
- Updated call sites inside `ResolveLocation` and `ResolveItemSelector` from `LooksLikeUUID` to `LooksLikeID`
