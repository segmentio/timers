package timers

import (
	"context"
	"testing"
	"time"
)

func TestSleep(t *testing.T) {
	t.Run("timeout", testSleepTimeout)
	t.Run("cancel", testSleepCancel)
}

func testSleepTimeout(t *testing.T) {
	t.Parallel()

	const sleepDuration = 100 * time.Microsecond

	then := time.Now()
	err := Sleep(context.Background(), sleepDuration)
	now := time.Now()

	if err != nil {
		t.Errorf("unexpected error returned from Sleep, expected <nil> but got %q", err)
	}

	if elapsed := now.Sub(then); elapsed < sleepDuration {
		t.Errorf("not enough time has passed since sleep was called, expected more than %s but got %s", sleepDuration, elapsed)
	}
}

func testSleepCancel(t *testing.T) {
	t.Parallel()

	const sleepDuration = 100 * time.Millisecond
	const abortDuration = sleepDuration / 2

	ctx, cancel := context.WithTimeout(context.Background(), abortDuration)
	defer cancel()

	then := time.Now()
	err := Sleep(ctx, sleepDuration)
	now := time.Now()

	if ctxErr := ctx.Err(); err != ctxErr {
		t.Errorf("unexpected error returned from Sleep, expected %q but got %q", ctxErr, err)
	}

	if elapsed := now.Sub(then); elapsed >= sleepDuration {
		t.Errorf("too much time has passed since sleep was called, expected less then %s but got %s", sleepDuration, elapsed)
	}
}
