// Package found implements the wherehouse found command,
// which records that a previously missing or lost item has been found.
//
// The found command fires an item.found event, setting the item's current
// location and establishing (or preserving) its home location for future
// return tracking.
//
// With --return, a second item.moved event is fired to return the item to
// its home location (TempOriginLocationID) if known.
//
// Supported selector types:
//   - UUID: 550e8400-e29b-41d4-a716-446655440001 (exact ID)
//   - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
//   - Canonical name: "10mm socket" (must match exactly 1 item)
//
// Examples:
//
//	wherehouse found "10mm socket" --in garage
//	wherehouse found "10mm socket" --in garage --return
//	wherehouse found garage:screwdriver --in shed --note "behind workbench"
package found
