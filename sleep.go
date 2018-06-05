package timers

import (
	"context"
	"time"
)

// Sleep puts the calling goroutine to sleep until the given duration has
// passed, or until the context is canceled, whichever comes first, in which
// case it will return the context's error.
func Sleep(ctx context.Context, duration time.Duration) (err error) {
	timer := time.NewTimer(duration)
	select {
	case <-timer.C:
	case <-ctx.Done():
		err = ctx.Err()
	}
	timer.Stop()
	return
}
