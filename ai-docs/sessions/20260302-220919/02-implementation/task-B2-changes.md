# Task B2 Changes: cmd/ EventType Call Site Updates

## Files Modified

### cmd/add/location.go
- Line 112: `"location.created"` → `database.LocationCreatedEvent`

### cmd/lost/item.go
- Line 108: `"item.missing"` → `database.ItemMissingEvent`

### cmd/found/found.go
- Line 188: `"item.found"` → `database.ItemFoundEvent`
- Line 231: `"item.moved"` → `database.ItemMovedEvent`

### cmd/loan/item.go
- Line 175: `"item.loaned"` → `database.ItemLoanedEvent`

### cmd/move/item.go
- Added import: `"github.com/asphaltbuffet/wherehouse/internal/database"`
- Line 169: `"item.moved"` → `database.ItemMovedEvent`

### cmd/move/mover.go
- Added `//go:generate mockery --name=moveDB` directive
- Interface method `AppendEvent`: `eventType string` → `eventType database.EventType`

### cmd/move/mocks/mock_movedb.go
- `AppendEvent` method signature: `eventType string` → `eventType database.EventType`
- All function literal type signatures updated to use `database.EventType`
- Type assertion in `Run` callback: `args[1].(string)` → `args[1].(database.EventType)`
- Comment updated: `eventType string` → `eventType database.EventType`

### cmd/history/output.go
- Removed `eventTypeMissing = "item.missing"` string constant
- `convertToJSONEvent`: `event.EventType` → `event.EventType.String()` in struct literal
- `formatEvent`: Connector comparison updated to `database.ItemFoundEvent` / `database.ItemMissingEvent`
- `formatEvent`: Marker comparisons updated to `database.ItemDeletedEvent` / `database.ItemMissingEvent`
- `formatEvent`: All `EventStyle(event.EventType)` calls → `EventStyle(eventTypeStr)` where `eventTypeStr := event.EventType.String()`
- `formatEvent`: `Render(event.EventType)` → `Render(eventTypeStr)` for the type display
- `formatEventDetails`: All `case "item.*":` string literals replaced with typed constants

## Build Result
`go build ./cmd/...` — clean, no errors

## Test Result
`go test ./cmd/...` — all packages pass
