package realtime

import (
	"context"
	"errors"
	"testing"
)

func TestHubPublishToUserBroadcastsToAllConnections(t *testing.T) {
	hub := NewHub()
	var first []Event
	var second []Event

	hub.register(&client{
		userID: "user_1",
		send: func(event Event) error {
			first = append(first, event)
			return nil
		},
	})
	hub.register(&client{
		userID: "user_1",
		send: func(event Event) error {
			second = append(second, event)
			return nil
		},
	})

	if err := hub.PublishToUser(context.Background(), "user_1", "ride_offer.pending", map[string]string{"id": "offer_1"}); err != nil {
		t.Fatalf("PublishToUser() error = %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("first=%d second=%d", len(first), len(second))
	}
	if first[0].Type != "ride_offer.pending" || second[0].Type != "ride_offer.pending" {
		t.Fatalf("events = %#v %#v", first, second)
	}
}

func TestHubPublishToUserUnregistersBrokenConnections(t *testing.T) {
	hub := NewHub()
	broken := &client{
		userID: "user_1",
		send: func(Event) error {
			return errors.New("broken connection")
		},
	}
	hub.register(broken)

	if err := hub.PublishToUser(context.Background(), "user_1", "ride_offer.pending", nil); err != nil {
		t.Fatalf("PublishToUser() error = %v", err)
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if _, ok := hub.clients["user_1"][broken]; ok {
		t.Fatalf("broken client was not removed")
	}
}
