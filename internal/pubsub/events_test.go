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
