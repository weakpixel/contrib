package ecslua

import (
	"context"
	"testing"
	"time"
)

func TestDeadlineCtx(t *testing.T) {
	c := newDeadlineCtx(time.Millisecond)
	if d, ok := c.Deadline(); !ok || d.IsZero() {
		t.Fatalf("Deadline() = %v, %v; want a deadline", d, ok)
	}
	if c.Done() != nil || c.Err() != nil {
		t.Fatal("expired at once")
	}
	time.Sleep(2 * time.Millisecond)
	// The clock is read every deadlineCheckEvery calls, so it takes at most
	// that many calls to notice.
	var done <-chan struct{}
	for range deadlineCheckEvery {
		if done = c.Done(); done != nil {
			break
		}
	}
	if done == nil {
		t.Fatalf("Done still open after %d calls past the deadline", deadlineCheckEvery)
	}
	select {
	case <-done:
	default:
		t.Fatal("Done channel is not closed")
	}
	if c.Err() != context.DeadlineExceeded {
		t.Fatalf("Err() = %v, want context.DeadlineExceeded", c.Err())
	}
	if c.Done() != done {
		t.Fatal("Done changed after expiry")
	}
}
