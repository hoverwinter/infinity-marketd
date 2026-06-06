package quotesvc

import (
	"context"
	"sync"
	"time"
)

// Limiter is a simple token-bucket rate limiter. A non-positive rate means
// unlimited. The reserve core is pure (no sleeping) so timing is testable.
type Limiter struct {
	mu     sync.Mutex
	rate   float64 // tokens per second; <= 0 means unlimited
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) error
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// NewLimiter creates a limiter. now/sleep may be nil (real clock / real sleep).
func NewLimiter(ratePerSec float64, burst int, now func() time.Time, sleep func(context.Context, time.Duration) error) *Limiter {
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = defaultSleep
	}
	b := float64(burst)
	if b < 1 {
		b = 1
	}
	return &Limiter{rate: ratePerSec, burst: b, tokens: b, last: now(), now: now, sleep: sleep}
}

// reserve consumes one token and returns how long the caller must wait before
// the request is allowed. It is pure aside from updating bucket state.
func (l *Limiter) reserve(now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rate <= 0 {
		return 0
	}
	if now.After(l.last) {
		l.tokens += now.Sub(l.last).Seconds() * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
	}
	l.last = now
	l.tokens--
	if l.tokens >= 0 {
		return 0
	}
	return time.Duration((-l.tokens) / l.rate * float64(time.Second))
}

// Wait blocks until a token is available or ctx is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	d := l.reserve(l.now())
	if d <= 0 {
		return nil
	}
	return l.sleep(ctx, d)
}

// Tokens returns the currently available token count (for health reporting).
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tokens
}
