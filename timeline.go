package timers

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Timeline is a data structure that maintains a cache of deadlines represented
// by background contexts. A Timeline has a resolution attribute representing
// the accuracy of the deadlines it maintains. All deadlines that fall within
// the same resolution window share the same context, making it very efficient
// to create thousands, or even millions of them since the runtime only needs to
// maintain a single timer per resolution window.
//
// Timelines are safe to use concurrently from multiple goroutines, however they
// should not be copied after being first used.
//
// The zero-value is a valid timeline with a resolution of 100ms.
type Timeline struct {
	// Resolution represents the accuracy of timers managed by this timeline.
	// The lower the resolution the more accurate the timers are, but it also
	// means the timeline will put more pressure on the runtime and use more
	// memory.
	Resolution time.Duration

	// Background configures the background context used by contexts created by
	// the timeline. If nil, the default background context is used instead.
	Background context.Context

	mutex     sync.RWMutex
	deadlines map[int64]deadline

	cleanupLock int64
	cleanupTime int64
}

var (
	// HighRes is a timeline configured for high resolution timers, with 10
	// millisecond accuracy.
	HighRes = Timeline{
		Resolution: 10 * time.Millisecond,
	}

	// LowRes is a timeline configured for low resolution timers, with 1 second
	// accuracy. This timeline is typically useful for network timeouts.
	//
	// Here is an example of how the timeline may be used to set a timeout on an
	// http request:
	//
	//	req = req.WithContext(timers.LowRes.Timeout(10 * time.Second))
	//	res, err := httpClient.Do(req)
	//
	LowRes = Timeline{
		Resolution: 1 * time.Second,
	}
)

// Cancel cancels all contexts and releases all internal resources managed by
// the timeline.
func (t *Timeline) Cancel() {
	t.mutex.Lock()

	for _, d := range t.deadlines {
		d.cancel()
	}

	t.deadlines = nil
	t.mutex.Unlock()
}

// Timeout returns a context which expires after the given amount of time has
// passed, plus up to the timeline's resolution.
func (t *Timeline) Timeout(timeout time.Duration) context.Context {
	now := time.Now()
	return t.Context(now.Add(timeout), now)
}

// Deadline returns a context which expires when the given deadline is reached,
// plus up to the timeline's resolution.
func (t *Timeline) Deadline(deadline time.Time) context.Context {
	return t.Context(deadline, time.Now())
}

// Context returns a context which expires when the given deadline is reached,
// using `now` as the current time.
func (t *Timeline) Context(at time.Time, now time.Time) context.Context {
	r := int64(t.resolution())
	k := at.UnixNano()

	// Round up to the nearest resoltion, unless the time already is a multiple
	// of the timeline resolution.
	if (k % r) != 0 {
		k = ((k / r) + 1) * r
	}

	t.mutex.RLock()
	d, ok := t.deadlines[k]
	t.mutex.RUnlock()

	if ok { // fast path
		return d.context
	}

	background := t.background()
	expiration := time.Unix(0, k)

	t.mutex.Lock()
	d, ok = t.deadlines[k]
	if !ok {
		if t.deadlines == nil {
			t.deadlines = make(map[int64]deadline)
		}
		d = makeDeadline(background, jitterTime(expiration, time.Duration(r)))
		t.deadlines[k] = d
	}
	t.mutex.Unlock()

	if cleanupTime := t.loadCleanupTime(); cleanupTime.IsZero() || cleanupTime.Before(now) {
		if t.tryLockCleanup() {
			t.storeCleanupTime(t.nextCleanupTime(cleanupTime))
			t.cleanup(now)
			t.unlockCleanup()
		}
	}

	return d.context
}

func (t *Timeline) nextCleanupTime(lastCleanupTime time.Time) time.Time {
	return lastCleanupTime.Add(100 * t.resolution())
}

func (t *Timeline) loadCleanupTime() time.Time {
	return time.Unix(0, atomic.LoadInt64(&t.cleanupTime))
}

func (t *Timeline) storeCleanupTime(cleanupTime time.Time) {
	atomic.StoreInt64(&t.cleanupTime, cleanupTime.UnixNano())
}

func (t *Timeline) tryLockCleanup() bool {
	return atomic.CompareAndSwapInt64(&t.cleanupLock, 0, 1)
}

func (t *Timeline) unlockCleanup() {
	atomic.StoreInt64(&t.cleanupLock, 0)
}

func (t *Timeline) cleanup(now time.Time) {
	r := t.resolution()
	t.mutex.RLock()

	for k, d := range t.deadlines {
		t.mutex.RUnlock()

		if deadline, _ := d.context.Deadline(); now.After(deadline.Add(r)) {
			d.cancel()
			t.mutex.Lock()
			delete(t.deadlines, k)
			t.mutex.Unlock()
		}

		t.mutex.RLock()
	}

	t.mutex.RUnlock()
}

func (t *Timeline) resolution() time.Duration {
	if r := t.Resolution; r != 0 {
		return r
	}
	return 100 * time.Millisecond
}

func (t *Timeline) background() context.Context {
	if b := t.Background; b != nil {
		return b
	}
	return context.Background()
}

type deadline struct {
	context context.Context
	cancel  context.CancelFunc
}

func makeDeadline(parent context.Context, expiration time.Time) deadline {
	context, cancel := context.WithDeadline(parent, expiration)
	return deadline{
		context: context,
		cancel:  cancel,
	}
}

var (
	jitterMutex sync.Mutex
	jitterRand  = rand.New(
		rand.NewSource(time.Now().UnixNano()),
	)
)

func jitter(d time.Duration) time.Duration {
	jitterMutex.Lock()
	x := time.Duration(jitterRand.Int63n(int64(d)))
	jitterMutex.Unlock()
	return x
}

func jitterTime(t time.Time, d time.Duration) time.Time {
	return t.Add(jitter(d))
}
