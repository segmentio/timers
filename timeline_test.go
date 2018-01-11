package timers

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTimeline(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T)
	}{
		{
			scenario: "scheduling a deadline every 10ms triggers as expected",
			function: testTimeline10ms,
		},
		{
			scenario: "multiple goroutines waiting on the same timeline are all notified when the deadline expires",
			function: testTimelineMulti,
		},
		{
			scenario: "canceling a timeline cancels all contexts of this timeline",
			function: testTimelineCancel,
		},
		{
			scenario: "canceling the background context of a timeline also cancels contexts that it created",
			function: testTimelineBackground,
		},
	}

	for _, test := range tests {
		testFunc := test.function
		t.Run(test.scenario, func(t *testing.T) {
			t.Parallel()
			testFunc(t)
		})
	}
}

func testTimeline10ms(t *testing.T) {
	timeline := Timeline{Resolution: 1 * time.Millisecond}
	defer timeline.Cancel()

	for i := 0; i != 100; i++ {
		t0 := time.Now()

		ctx := timeline.Timeout(10 * time.Millisecond)
		<-ctx.Done()

		t1 := time.Now()
		d, _ := ctx.Deadline()

		for j, delay := range []time.Duration{d.Sub(t0), t1.Sub(t0)} {
			if delay < (10 * time.Millisecond) {
				t.Error("the delay is too short, expected > 10ms, got", delay, "at", j, "/", i)
			}
			if delay > (15 * time.Millisecond) {
				t.Error("the delay is too large, expected < 13ms, got", delay, "at", j, "/", i)
			}
		}

		if err := ctx.Err(); err != context.DeadlineExceeded {
			t.Error("bad context error:", err)
		}
	}
}

func testTimelineMulti(t *testing.T) {
	timeline := Timeline{Resolution: 10 * time.Millisecond}
	defer timeline.Cancel()

	wg := sync.WaitGroup{}
	deadline := time.Now().Add(100 * time.Millisecond)

	for i := 0; i != 10; i++ {
		wg.Add(1)
		go func(ctx context.Context) {
			<-ctx.Done()
			wg.Done()
		}(timeline.Deadline(deadline))
	}

	wg.Wait()
}

func testTimelineCancel(t *testing.T) {
	timeline := Timeline{}

	ctx1 := timeline.Timeout(1 * time.Second)
	ctx2 := timeline.Timeout(2 * time.Second)
	ctx3 := timeline.Timeout(3 * time.Second)

	timeline.Cancel()

	for _, ctx := range []context.Context{ctx1, ctx2, ctx3} {
		if err := ctx.Err(); err != context.Canceled {
			t.Error("bad context error:", err)
		}
	}
}

func testTimelineBackground(t *testing.T) {
	background, cancel := context.WithCancel(context.Background())
	timeline := Timeline{Background: background}

	ctx1 := timeline.Timeout(1 * time.Second)
	ctx2 := timeline.Timeout(2 * time.Second)
	ctx3 := timeline.Timeout(3 * time.Second)

	cancel()

	for _, ctx := range []context.Context{ctx1, ctx2, ctx3} {
		if err := ctx.Err(); err != context.Canceled {
			t.Error("bad context error:", err)
		}
	}
}

func BenchmarkTimeline(b *testing.B) {
	timeouts := []time.Duration{
		100 * time.Millisecond,
		250 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		10 * time.Second,
	}

	timeline := Timeline{}
	defer timeline.Cancel()

	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			timeline.Timeout(timeouts[i%len(timeouts)])
		}
	})
}
