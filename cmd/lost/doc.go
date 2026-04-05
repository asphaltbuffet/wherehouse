// Package lost implements the `wherehouse lost` command that marks items as lost/missing.
//
// This command moves items to the system "Missing" location while preserving their
// home location information. When an item is marked as missing, its original home
// location is preserved so it can be returned when found.
//
// The command supports multiple selector types for item identification:
//   - ID: Exact ID match (verified against database)
//   - LOCATION:ITEM: Scoped selector using canonical names
//   - Canonical name: Must match exactly one item
//
// # Event-Sourcing Design
//
// This command creates an "item.missing" event, which triggers the event
// handler to move the item to the Missing system location. The home location
// (temp_origin_location_id) is preserved automatically by the event handler.
//
// # Business Rules
//
// Validation before event creation:
//   - Item must exist in database
//   - Item must NOT already be in Missing location (prevents duplicate events)
//   - Borrowed items CAN be marked as missing (borrowed → missing is valid)
//   - previous_location_id must match current projection state
//
// # Examples
//
//	wherehouse lost "10mm socket"
//	wherehouse lost garage:socket --note "checked toolbox"
//	wherehouse lost aB3xK9mPqR
package lost
