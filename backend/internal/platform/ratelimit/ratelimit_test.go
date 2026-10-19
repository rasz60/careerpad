package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	l := New(2, time.Hour)
	l.now = func() time.Time { return now }

	for i := range 2 {
		if !l.Allow("a") {
			t.Fatalf("request %d should pass", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("third request in the window should be blocked")
	}
	if !l.Allow("b") {
		t.Fatal("other keys are counted separately")
	}

	now = now.Add(time.Hour + time.Second)
	if !l.Allow("a") {
		t.Fatal("after the window the key should pass again")
	}
}
