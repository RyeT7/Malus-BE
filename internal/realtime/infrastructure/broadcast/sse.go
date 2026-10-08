package broadcast

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"malus-be/internal/realtime/application"
)

const clientBuffer = 16

type Hub struct {
	streamURL string
	heartbeat time.Duration

	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub(streamURL string) *Hub {
	return &Hub{streamURL: streamURL, heartbeat: 20 * time.Second, clients: make(map[chan []byte]struct{})}
}

func (h *Hub) Publish(_ context.Context, state application.LiveState) error {
	data, err := encode(state)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
		}
	}
	return nil
}

func (h *Hub) Connection(context.Context) (application.Connection, error) {
	return application.Connection{Kind: "sse", URL: h.streamURL}, nil
}

func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": connected\n\n")
	if err := rc.Flush(); err != nil {
		return
	}

	ch := make(chan []byte, clientBuffer)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()

	ticker := time.NewTicker(h.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}
