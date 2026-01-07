package eventbus

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryBus_SubscribePublishUnsubscribe(t *testing.T) {
	bus := NewMemoryBus()

	gotCh := make(chan Event, 1)
	handlerID := bus.Subscribe(EventTaskUpdated, func(e Event) {
		gotCh <- e
	})
	if handlerID == "" {
		t.Fatalf("expected non-empty handlerID")
	}

	event := Event{
		ID:        "e1",
		Type:      EventTaskUpdated,
		Source:    "test",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"k": "v"},
	}
	bus.Publish(event)

	select {
	case got := <-gotCh:
		if got.ID != event.ID || got.Type != event.Type || got.Source != event.Source {
			t.Fatalf("unexpected event: %#v", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for event")
	}

	bus.Unsubscribe(EventTaskUpdated, handlerID)

	bus.Publish(event)
	select {
	case got := <-gotCh:
		t.Fatalf("did not expect event after unsubscribe, got %#v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMemoryBus_PublishOnlyToMatchingType(t *testing.T) {
	bus := NewMemoryBus()

	called := make(chan struct{}, 1)
	handlerID := bus.Subscribe(EventTerminalOutput, func(Event) {
		called <- struct{}{}
	})
	if handlerID == "" {
		t.Fatalf("expected non-empty handlerID")
	}
	defer bus.Unsubscribe(EventTerminalOutput, handlerID)

	bus.Publish(Event{ID: "e1", Type: EventTaskUpdated})

	select {
	case <-called:
		t.Fatalf("handler should not be called for non-matching type")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMemoryBus_PublishToAllSubscribers(t *testing.T) {
	bus := NewMemoryBus()

	var calls int32
	done := make(chan struct{}, 1)
	handler := func(Event) {
		if atomic.AddInt32(&calls, 1) == 2 {
			done <- struct{}{}
		}
	}

	id1 := bus.Subscribe(EventTerminalOutput, handler)
	id2 := bus.Subscribe(EventTerminalOutput, handler)
	if id1 == "" || id2 == "" {
		t.Fatalf("expected non-empty handlerIDs")
	}
	defer bus.Unsubscribe(EventTerminalOutput, id1)
	defer bus.Unsubscribe(EventTerminalOutput, id2)

	bus.Publish(Event{Type: EventTerminalOutput})

	select {
	case <-done:
		if atomic.LoadInt32(&calls) != 2 {
			t.Fatalf("expected 2 calls, got %d", calls)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for handlers")
	}
}

func TestMemoryBus_HandlerPanicRecovered(t *testing.T) {
	bus := NewMemoryBus()

	var calls int32
	done := make(chan struct{}, 1)
	handlerID := bus.Subscribe(EventTerminalStatus, func(Event) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			panic("boom")
		}
		if n == 2 {
			done <- struct{}{}
		}
	})
	if handlerID == "" {
		t.Fatalf("expected non-empty handlerID")
	}
	defer bus.Unsubscribe(EventTerminalStatus, handlerID)

	bus.Publish(Event{Type: EventTerminalStatus})
	bus.Publish(Event{Type: EventTerminalStatus})

	select {
	case <-done:
		if atomic.LoadInt32(&calls) != 2 {
			t.Fatalf("expected 2 calls, got %d", calls)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for second handler call")
	}
}

func TestMemoryBus_InvalidArgsNoop(t *testing.T) {
	bus := NewMemoryBus()

	if id := bus.Subscribe("", func(Event) {}); id != "" {
		t.Fatalf("expected empty handlerID for empty eventType, got %q", id)
	}
	if id := bus.Subscribe(EventTaskUpdated, nil); id != "" {
		t.Fatalf("expected empty handlerID for nil handler, got %q", id)
	}

	bus.Unsubscribe("", "x")
	bus.Unsubscribe(EventTaskUpdated, "")
	bus.Unsubscribe(EventTaskUpdated, "not-exist")

	bus.Publish(Event{})
}
