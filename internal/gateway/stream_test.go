package gateway

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"malus-be/internal/platform/httpx"
)

func TestEventStreamOutlivesWriteTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		for i := 1; i <= 5; i++ {
			fmt.Fprintf(w, "data: %d\n\n", i)
			_ = rc.Flush()
			time.Sleep(500 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	u, _ := url.Parse(upstream.URL)
	cfg := Config{
		Upstreams: map[string]*url.URL{"content": u, "interaction": u, "realtime": u},
		Auth:      AuthConfig{AdminRole: "admin"},
		RateLimit: RateLimitConfig{RPS: 100, Burst: 100, WriteRPS: 100, WriteBurst: 100},
	}
	h, err := New(cfg, NewDevAuthenticator("admin"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	gw := httptest.NewUnstartedServer(httpx.Chain(h, httpx.RequestID))
	gw.Config.WriteTimeout = time.Second
	gw.Start()
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodGet, gw.URL+"/v1/live/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var events []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "data: ") {
			events = append(events, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(events) != 5 || time.Since(start) < 2*time.Second {
		t.Fatalf("want all 5 events over ~2.5s despite a 1s write timeout, got %v after %v", events, time.Since(start))
	}
}
