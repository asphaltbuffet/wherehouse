// Package move implements the wherehouse move command.
//
// The move command relocates items to a different location with strict validation:
//   - System locations (Missing, Borrowed) are forbidden as source or destination
//   - Canonical name matches must be EXACT and UNIQUE (ambiguous names fail with ID list)
//   - Items only (location moves deferred to v2)
//   - Fail-fast batch processing (stops on first error)
//
// Supported selector types:
//   - UUID: 550e8400-e29b-41d4-a716-446655440001 (exact ID)
//   - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
//   - Canonical name: "10mm socket" (must match exactly 1 item)
//
// Examples:
//
//	wherehouse move garage:socket --to toolbox
//	wherehouse move 550e8400-e29b-41d4-a716-446655440001 --to desk
//	wherehouse move "10mm socket" --to garage --temp
//	wherehouse move wrench screwdriver --to toolbox --keep-project
package move
