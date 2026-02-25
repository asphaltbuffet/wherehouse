/*
Package loan implements the "wherehouse loan" command for marking items as loaned to someone.

The loan command moves items to the "Loaned" system location and records the recipient's
name in the event log. Items can be loaned from any location, including system locations
like Missing and Borrowed. Re-loaning is supported - items already in Loaned location can
be loaned again to a different person.

Key features:
  - Batch operations: Loan multiple items in one command
  - Re-loaning: Items can be loaned multiple times (creates new events)
  - Free text recipient: --to flag accepts any string (no validation beyond non-empty)
  - Fail-fast: Validation errors abort entire batch before creating events

Event type: item.loaned
System location: Loaned (canonical_name: "loaned", is_system: true)

CLI usage:

	wherehouse loan <item-selector>... --to <name> [--note <text>]

Examples:

	wherehouse loan garage:socket --to "Bob Smith"
	wherehouse loan "10mm socket" --to alice@example.com --note "for weekend"
	wherehouse loan wrench screwdriver --to Bob
*/
package loan
