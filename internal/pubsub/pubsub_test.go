package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

func TestPubSub_PublishAndSubscribe(t *testing.T) {
	ctx := context.Background()

	ps, err := New(ctx, nil) // nil config = use embedded NATS
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() {
		_ = ps.Close()
	}()

	sub, err := ps.NewSubscriber(ctx)
	if err != nil {
		t.Fatalf("creating subscriber: %v", err)
	}
	defer func() {
		_ = sub.Close()
	}()

	topic := "test-topic"
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	messages, err := sub.Subscribe(subCtx, topic)
	if err != nil {
		t.Fatalf("subscribing: %v", err)
	}

	// Give subscriber time to be ready
	time.Sleep(50 * time.Millisecond)

	event := QueryResultEvent{
		HostID:     uuid.New(),
		QueryID:    uuid.New(),
		Status:     QueryResultStatusCompleted,
		OccurredAt: time.Now().UTC().Truncate(time.Second),
	}

	if err := ps.Publisher().Publish(topic, event.ToMessage()); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	select {
	case msg := <-messages:
		if msg == nil {
			t.Fatalf("received nil message")
		}
		received, err := ParseQueryResultEvent(msg)
		if err != nil {
			t.Fatalf("parsing event: %v", err)
		}
		if received.QueryID != event.QueryID {
			t.Fatalf("QueryID = %v, want %v", received.QueryID, event.QueryID)
		}
		if received.Status != event.Status {
			t.Fatalf("Status = %q, want %q", received.Status, event.Status)
		}
		msg.Ack()
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestPubSub_MultipleSubscribers(t *testing.T) {
	ctx := context.Background()

	ps, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() {
		_ = ps.Close()
	}()

	topic := "test-topic"
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	makeSubscriber := func() (<-chan *message.Message, func(), error) {
		sub, err := ps.NewSubscriber(ctx)
		if err != nil {
			return nil, nil, err
		}
		msgs, err := sub.Subscribe(subCtx, topic)
		if err != nil {
			_ = sub.Close()
			return nil, nil, err
		}
		cleanup := func() { _ = sub.Close() }
		return msgs, cleanup, nil
	}

	msgs1, cleanup1, err := makeSubscriber()
	if err != nil {
		t.Fatalf("subscriber1: %v", err)
	}
	defer cleanup1()

	msgs2, cleanup2, err := makeSubscriber()
	if err != nil {
		t.Fatalf("subscriber2: %v", err)
	}
	defer cleanup2()

	msgs3, cleanup3, err := makeSubscriber()
	if err != nil {
		t.Fatalf("subscriber3: %v", err)
	}
	defer cleanup3()

	// Give subscribers time to be ready
	time.Sleep(50 * time.Millisecond)

	event := QueryResultEvent{
		HostID:     uuid.New(),
		QueryID:    uuid.New(),
		Status:     QueryResultStatusCompleted,
		OccurredAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := ps.Publisher().Publish(topic, event.ToMessage()); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	recv := func(name string, ch <-chan *message.Message) {
		t.Helper()
		select {
		case msg := <-ch:
			if msg == nil {
				t.Fatalf("%s received nil message", name)
			}
			msg.Ack()
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for %s", name)
		}
	}

	recv("subscriber1", msgs1)
	recv("subscriber2", msgs2)
	recv("subscriber3", msgs3)
}

func TestPubSub_TopicIsolation(t *testing.T) {
	ctx := context.Background()

	ps, err := New(ctx, nil)
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() {
		_ = ps.Close()
	}()

	sub, err := ps.NewSubscriber(ctx)
	if err != nil {
		t.Fatalf("creating subscriber: %v", err)
	}
	defer func() {
		_ = sub.Close()
	}()

	topicA := "test-topic-a"
	topicB := "test-topic-b"

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	messagesA, err := sub.Subscribe(subCtx, topicA)
	if err != nil {
		t.Fatalf("subscribing topicA: %v", err)
	}

	// Give subscriber time to be ready
	time.Sleep(50 * time.Millisecond)

	event := QueryResultEvent{
		HostID:     uuid.New(),
		QueryID:    uuid.New(),
		Status:     QueryResultStatusCompleted,
		OccurredAt: time.Now().UTC().Truncate(time.Second),
	}

	// Publish to topicB, should NOT be received on topicA
	if err := ps.Publisher().Publish(topicB, event.ToMessage()); err != nil {
		t.Fatalf("publishing topicB: %v", err)
	}

	select {
	case msg := <-messagesA:
		if msg != nil {
			msg.Ack()
		}
		t.Fatalf("unexpected message received on topicA")
	case <-time.After(200 * time.Millisecond):
		// ok - no message received, which is expected
	}
}
