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
//
// Deprecated for new functionality; kept for backward compatibility with the
// host details page stream.
func TopicQueryResults(hostID uuid.UUID) string {
	return fmt.Sprintf("query_results:%s", hostID.String())
}

// TopicCampaign returns the topic name for a campaign's results.
func TopicCampaign(campaignID uuid.UUID) string {
	return fmt.Sprintf("campaign:%s", campaignID.String())
}

// QueryResultEvent is published when distributed query results are saved.
//
// Deprecated for new functionality; kept for backward compatibility with the
// host details page stream.
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

// CampaignResultEvent is published when a host returns results for a campaign.
type CampaignResultEvent struct {
	CampaignID uuid.UUID `json:"campaign_id"`
	HostID     uuid.UUID `json:"host_id"`

	// HostIdentifier is optional convenience data for clients.
	HostIdentifier string `json:"host_identifier,omitempty"`

	Status string `json:"status"`

	// OccurredAt is when the result was saved.
	OccurredAt time.Time `json:"occurred_at"`

	RowCount int     `json:"row_count,omitempty"`
	Error    *string `json:"error,omitempty"`
}

// ToMessage converts the event to a Watermill message.
func (e CampaignResultEvent) ToMessage() *message.Message {
	payload, err := json.Marshal(e)
	if err != nil {
		payload = []byte("{}")
	}

	msg := message.NewMessage(uuid.NewString(), payload)
	msg.Metadata.Set("event_type", "campaign_result")
	msg.Metadata.Set("campaign_id", e.CampaignID.String())
	msg.Metadata.Set("host_id", e.HostID.String())
	return msg
}

// ParseCampaignResultEvent parses a Watermill message into a CampaignResultEvent.
func ParseCampaignResultEvent(msg *message.Message) (CampaignResultEvent, error) {
	var event CampaignResultEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return event, fmt.Errorf("parsing campaign result event: %w", err)
	}
	return event, nil
}
