package pubsub

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestQueryResultEvent_SerializationRoundTrip(t *testing.T) {
	occurredAt := time.Now().UTC().Truncate(time.Second)

	original := QueryResultEvent{
		HostID:     uuid.New(),
		QueryID:    uuid.New(),
		Status:     QueryResultStatusCompleted,
		OccurredAt: occurredAt,
	}

	msg := original.ToMessage()
	if got := msg.Metadata.Get("event_type"); got != "query_result" {
		t.Fatalf("event_type = %q, want query_result", got)
	}
	if got := msg.Metadata.Get("host_id"); got != original.HostID.String() {
		t.Fatalf("host_id = %q, want %q", got, original.HostID.String())
	}
	if got := msg.Metadata.Get("query_id"); got != original.QueryID.String() {
		t.Fatalf("query_id = %q, want %q", got, original.QueryID.String())
	}

	parsed, err := ParseQueryResultEvent(msg)
	if err != nil {
		t.Fatalf("ParseQueryResultEvent error = %v", err)
	}

	if parsed.HostID != original.HostID {
		t.Fatalf("HostID = %v, want %v", parsed.HostID, original.HostID)
	}
	if parsed.QueryID != original.QueryID {
		t.Fatalf("QueryID = %v, want %v", parsed.QueryID, original.QueryID)
	}
	if parsed.Status != original.Status {
		t.Fatalf("Status = %q, want %q", parsed.Status, original.Status)
	}
	if !parsed.OccurredAt.Equal(original.OccurredAt) {
		t.Fatalf("OccurredAt = %v, want %v", parsed.OccurredAt, original.OccurredAt)
	}
	if parsed.Error != nil {
		t.Fatalf("Error = %v, want nil", *parsed.Error)
	}
}

func TestCampaignResultEvent_SerializationRoundTrip(t *testing.T) {
	occurredAt := time.Now().UTC().Truncate(time.Second)

	errText := "boom"
	original := CampaignResultEvent{
		CampaignID:     uuid.New(),
		HostID:         uuid.New(),
		HostIdentifier: "host-123",
		Status:         QueryResultStatusFailed,
		OccurredAt:     occurredAt,
		RowCount:       42,
		Error:          &errText,
	}

	msg := original.ToMessage()
	if got := msg.Metadata.Get("event_type"); got != "campaign_result" {
		t.Fatalf("event_type = %q, want campaign_result", got)
	}
	if got := msg.Metadata.Get("campaign_id"); got != original.CampaignID.String() {
		t.Fatalf("campaign_id = %q, want %q", got, original.CampaignID.String())
	}
	if got := msg.Metadata.Get("host_id"); got != original.HostID.String() {
		t.Fatalf("host_id = %q, want %q", got, original.HostID.String())
	}

	parsed, err := ParseCampaignResultEvent(msg)
	if err != nil {
		t.Fatalf("ParseCampaignResultEvent error = %v", err)
	}

	if parsed.CampaignID != original.CampaignID {
		t.Fatalf("CampaignID = %v, want %v", parsed.CampaignID, original.CampaignID)
	}
	if parsed.HostID != original.HostID {
		t.Fatalf("HostID = %v, want %v", parsed.HostID, original.HostID)
	}
	if parsed.HostIdentifier != original.HostIdentifier {
		t.Fatalf("HostIdentifier = %q, want %q", parsed.HostIdentifier, original.HostIdentifier)
	}
	if parsed.Status != original.Status {
		t.Fatalf("Status = %q, want %q", parsed.Status, original.Status)
	}
	if !parsed.OccurredAt.Equal(original.OccurredAt) {
		t.Fatalf("OccurredAt = %v, want %v", parsed.OccurredAt, original.OccurredAt)
	}
	if parsed.RowCount != original.RowCount {
		t.Fatalf("RowCount = %d, want %d", parsed.RowCount, original.RowCount)
	}
	if parsed.Error == nil || *parsed.Error != *original.Error {
		if parsed.Error == nil {
			t.Fatalf("Error = nil, want %q", *original.Error)
		}
		t.Fatalf("Error = %q, want %q", *parsed.Error, *original.Error)
	}
}
