package pubsub

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

const (
	QueryResultStatusCompleted = "completed"
	QueryResultStatusFailed    = "failed"
)

// TopicQueryResults returns the topic name for a host's query results.
func TopicQueryResults(hostID uuid.UUID) string {
	return fmt.Sprintf("query_results:%s", hostID.String())
}

// QueryResultEvent is published when distributed query results are saved.
type QueryResultEvent struct {
	// HostID is the host that executed the query.
	HostID uuid.UUID `json:"host_id"`

	// QueryID is the distributed query ID.
	QueryID uuid.UUID `json:"query_id"`

	// Status is the result status.
	Status string `json:"status"`

	// OccurredAt is when the result was saved.
	OccurredAt time.Time `json:"occurred_at"`

	// Error is set if status is failed.
	Error *string `json:"error,omitempty"`
}

// ToMessage converts the event to a Watermill message.
func (e QueryResultEvent) ToMessage() *message.Message {
	payload, err := json.Marshal(e)
	if err != nil {
		payload = []byte("{}")
	}

	msg := message.NewMessage(uuid.NewString(), payload)
	msg.Metadata.Set("event_type", "query_result")
	msg.Metadata.Set("host_id", e.HostID.String())
	msg.Metadata.Set("query_id", e.QueryID.String())
	return msg
}

// ParseQueryResultEvent parses a Watermill message into a QueryResultEvent.
func ParseQueryResultEvent(msg *message.Message) (QueryResultEvent, error) {
	var event QueryResultEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return event, fmt.Errorf("parsing query result event: %w", err)
	}
	return event, nil
}
