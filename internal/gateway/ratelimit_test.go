package gateway

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterRefillsOverTime(t *testing.T) {
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(2, 3)
	l.now = func() time.Time { return now }

	for i := range 3 {
		if !l.Allow("ip") {
			t.Fatalf("request %d within burst was rejected", i+1)
		}
	}
	if l.Allow("ip") {
		t.Fatal("request beyond burst was allowed")
	}
	if !l.Allow("other-ip") {
		t.Fatal("keys must have independent buckets")
	}

	now = now.Add(500 * time.Millisecond)
	if !l.Allow("ip") {
		t.Fatal("one token should refill after 500ms at 2 rps")
	}
	if l.Allow("ip") {
		t.Fatal("only one token should have refilled")
	}
}

func TestClientIPResolver(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.9")

	if got := clientIPResolver(false)(r); got != "10.0.0.5" {
		t.Fatalf("untrusted: want remote addr, got %q", got)
	}
	if got := clientIPResolver(true)(r); got != "203.0.113.9" {
		t.Fatalf("trusted: want rightmost forwarded entry, got %q", got)
	}
}
