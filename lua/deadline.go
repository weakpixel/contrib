package ecslua

import (
	"context"
	"time"
)

// deadlineCheckEvery is how many Done calls share one clock read. GopherLua
// calls Done once per VM instruction, so reading the clock every time would
// dominate tight loops.
const deadlineCheckEvery = 1024

// closedChan is returned by Done once the deadline has passed.
var closedChan = func() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}()

// deadlineCtx is a context that expires at a wall-clock deadline without a
// timer goroutine. context.WithTimeout closes Done from a timer goroutine,
// which never runs on js/wasm while a Lua loop spins (one thread, no async
// preemption), so a TickBudget would never fire there. Polling the clock from
// Done works on one thread and behaves the same natively.
//
// It is used by one goroutine (the World's) only.
type deadlineCtx struct {
	context.Context // Background: no values, no parent cancellation
	deadline        time.Time
	calls           int
	expired         bool
}

func newDeadlineCtx(budget time.Duration) *deadlineCtx {
	return &deadlineCtx{Context: context.Background(), deadline: time.Now().Add(budget)}
}

func (c *deadlineCtx) Deadline() (time.Time, bool) { return c.deadline, true }

// Done returns nil (never done) until the deadline has passed, then a closed
// channel.
func (c *deadlineCtx) Done() <-chan struct{} {
	if !c.expired {
		c.calls++
		if c.calls%deadlineCheckEvery != 0 || time.Now().Before(c.deadline) {
			return nil
		}
		c.expired = true
	}
	return closedChan
}

// Err is context.DeadlineExceeded once Done has reported the deadline.
func (c *deadlineCtx) Err() error {
	if c.expired {
		return context.DeadlineExceeded
	}
	return nil
}
