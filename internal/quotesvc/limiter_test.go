package quotesvc

import (
	"testing"
	"time"
)

func TestLimiterUnlimited(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := NewLimiter(0, 1, clk.Now, nil)
	for i := 0; i < 100; i++ {
		if d := l.reserve(clk.Now()); d != 0 {
			t.Fatalf("unlimited limiter returned wait %v", d)
		}
	}
}

func TestLimiterBurstThenThrottle(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := NewLimiter(2, 2, clk.Now, nil) // 2 tokens/sec, burst 2

	if d := l.reserve(clk.Now()); d != 0 {
		t.Fatalf("first reserve should be immediate, got %v", d)
	}
	if d := l.reserve(clk.Now()); d != 0 {
		t.Fatalf("second reserve (burst) should be immediate, got %v", d)
	}
	d := l.reserve(clk.Now())
	if d != 500*time.Millisecond {
		t.Fatalf("third reserve should wait 500ms at 2/s, got %v", d)
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := NewLimiter(2, 2, clk.Now, nil)
	l.reserve(clk.Now())
	l.reserve(clk.Now()) // burst drained
	clk.advance(time.Second)
	if d := l.reserve(clk.Now()); d != 0 {
		t.Fatalf("after 1s refill, reserve should be immediate, got %v", d)
	}
}
