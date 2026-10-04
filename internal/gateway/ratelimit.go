package gateway

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"malus-be/internal/platform/httpx"
)

type Limiter struct {
	rate  float64
	burst float64
	idle  time.Duration
	now   func() time.Time

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewLimiter(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:    rate,
		burst:   float64(burst),
		idle:    10 * time.Minute,
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) > l.idle {
		for k, b := range l.buckets {
			if now.Sub(b.last) > l.idle {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func rateLimit(all, writes *Limiter, clientIP func(*http.Request) string) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			if !all.Allow(key) || (isWrite(r.Method) && !writes.Allow(key)) {
				w.Header().Set("Retry-After", "1")
				httpx.Problem(w, r, http.StatusTooManyRequests, "Too Many Requests", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isWrite(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

func clientIPResolver(trustProxyHeaders bool) func(*http.Request) string {
	return func(r *http.Request) string {
		if trustProxyHeaders {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.Split(xff, ",")
				if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
					return ip
				}
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}
