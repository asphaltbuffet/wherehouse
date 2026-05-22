package inventory

import "encoding/json"

// Event represents a single entry in the inventory event log.
type Event struct {
	EventID      int64
	EventType    EventType
	TimestampUTC string
	ActorUserID  string
	Payload      json.RawMessage
	Note         *string
	EntityID     *string
}
