package meshtasticapi

import (
	"context"
	"testing"
	"time"
)

func TestWaitForConfigErrorsWhenErrorChannelClosed(t *testing.T) {
	t.Parallel()

	c := &Client{
		events: make(chan Event),
		errors: make(chan error),
		closed: make(chan struct{}),
	}
	close(c.errors)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := c.waitForConfig(ctx); err == nil {
		t.Fatal("expected error when error channel closes before config complete")
	}
}

func TestWaitForConfigCompletesOnConfigEvent(t *testing.T) {
	t.Parallel()

	c := &Client{
		events: make(chan Event, 1),
		errors: make(chan error, 1),
		closed: make(chan struct{}),
	}
	c.events <- Event{Type: EventConfigComplete}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.waitForConfig(ctx); err != nil {
		t.Fatalf("expected success on config complete event, got %v", err)
	}
}
