package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/cavenine/queryops/internal/testdb"
	"github.com/google/uuid"
)

func TestPubSub_PublishAndSubscribe(t *testing.T) {
	tdb := testdb.SetupTestDB(t)

	ps, err := New(context.Background(), tdb.Pool, &Config{AutoInitializeSchema: true})
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() {
		_ = ps.Close()
	}()

	sub, err := ps.NewSubscriber(context.Background())
	if err != nil {
		t.Fatalf("creating subscriber: %v", err)
	}
	defer func() {
		_ = sub.Close()
	}()

	topic := "test-topic-" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages, err := sub.Subscribe(ctx, topic)
	if err != nil {
		t.Fatalf("subscribing: %v", err)
	}

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
	tdb := testdb.SetupTestDB(t)

	ps, err := New(context.Background(), tdb.Pool, &Config{AutoInitializeSchema: true})
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() {
		_ = ps.Close()
	}()

	topic := "test-topic-" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	makeSubscriber := func() (<-chan *message.Message, func(), error) {
		sub, err := ps.NewSubscriber(context.Background())
		if err != nil {
			return nil, nil, err
		}
		msgs, err := sub.Subscribe(ctx, topic)
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
	tdb := testdb.SetupTestDB(t)

	ps, err := New(context.Background(), tdb.Pool, &Config{AutoInitializeSchema: true})
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() {
		_ = ps.Close()
	}()

	sub, err := ps.NewSubscriber(context.Background())
	if err != nil {
		t.Fatalf("creating subscriber: %v", err)
	}
	defer func() {
		_ = sub.Close()
	}()

	topicA := "test-topic-a-" + uuid.NewString()
	topicB := "test-topic-b-" + uuid.NewString()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messagesA, err := sub.Subscribe(ctx, topicA)
	if err != nil {
		t.Fatalf("subscribing topicA: %v", err)
	}

	event := QueryResultEvent{
		HostID:     uuid.New(),
		QueryID:    uuid.New(),
		Status:     QueryResultStatusCompleted,
		OccurredAt: time.Now().UTC().Truncate(time.Second),
	}

	if err := ps.Publisher().Publish(topicB, event.ToMessage()); err != nil {
		t.Fatalf("publishing topicB: %v", err)
	}

	select {
	case msg := <-messagesA:
		if msg != nil {
			msg.Ack()
		}
		t.Fatalf("unexpected message received on topicA")
	case <-time.After(500 * time.Millisecond):
		// ok
	}
}
