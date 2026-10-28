package realtime

import (
	"testing"
	"time"
)

func TestHubPublishesAndCleansUp(t *testing.T) {
	hub := NewHub(1)
	messages, cancel := hub.Subscribe("rfq-1")
	if hub.SubscriberCount("rfq-1") != 1 {
		t.Fatal("subscriber was not registered")
	}
	hub.Publish("rfq-1", "BID_ACCEPTED")
	message := <-messages
	if message.RFQID != "rfq-1" || message.Reason != "BID_ACCEPTED" {
		t.Fatalf("message = %+v", message)
	}
	cancel()
	cancel()
	if hub.SubscriberCount("rfq-1") != 0 {
		t.Fatal("subscriber was not removed")
	}
}

func TestHubDisconnectsSlowSubscriberWithoutBlocking(t *testing.T) {
	hub := NewHub(1)
	messages, _ := hub.Subscribe("rfq-1")
	hub.Publish("rfq-1", "FIRST")
	done := make(chan struct{})
	go func() {
		hub.Publish("rfq-1", "SECOND")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on slow subscriber")
	}
	if hub.SubscriberCount("rfq-1") != 0 {
		t.Fatal("slow subscriber was not removed")
	}
	if _, open := <-messages; !open {
		t.Fatal("buffered first message should remain readable")
	}
	if _, open := <-messages; open {
		t.Fatal("slow subscriber channel should be closed")
	}
}
