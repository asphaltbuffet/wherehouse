package inventory

import "encoding/json"

type Event struct {
	EventID      int64
	EventType    EventType
	TimestampUTC string
	ActorUserID  string
	Payload      json.RawMessage
	Note         *string
	EntityID     *string
}
