package broadcast

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"malus-be/internal/realtime/application"
)

func TestHubStreamsPublishedStateToClients(t *testing.T) {
	hub := NewHub("/v1/live/stream")
	srv := httptest.NewServer(hub)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
	for i := 0; i < 50 && hub.Clients() == 0; i++ {
		time.Sleep(20 * time.Millisecond)
	}

	if err := hub.Publish(context.Background(), application.LiveState{SessionID: "s1", Slide: 4, SlideCount: 9, Version: 3, Active: true}); err != nil {
		t.Fatal(err)
	}

	lines := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case line := <-lines:
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var msg stateMessage
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &msg); err != nil {
				t.Fatal(err)
			}
			if msg.Type != "state" || msg.SessionID != "s1" || msg.Slide != 4 || msg.Version != 3 || !msg.Active {
				t.Fatalf("unexpected message: %+v", msg)
			}
			return
		case <-timeout:
			t.Fatal("no state message received")
		}
	}
}

func TestHubConnectionPointsAtStream(t *testing.T) {
	conn, _ := NewHub("/v1/live/stream").Connection(context.Background())
	if conn.Kind != "sse" || conn.URL != "/v1/live/stream" {
		t.Fatalf("unexpected connection: %+v", conn)
	}
}

func TestWebPubSubSendsBroadcastAndIssuesClientURL(t *testing.T) {
	const key = "test-access-key"
	var sendBody []byte
	var sendAuth, tokenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/hubs/malus/:send":
			sendAuth = r.Header.Get("Authorization")
			sendBody, _ = io.ReadAll(r.Body)
			if r.URL.Query().Get("api-version") != webPubSubAPIVersion || r.Header.Get("Content-Type") != "application/json" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPost && r.URL.Path == "/api/hubs/malus/:generateToken":
			tokenQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "client-token"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	wps, err := NewWebPubSub(WebPubSubConfig{Endpoint: srv.URL, Hub: "malus", AccessKey: key}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := wps.Publish(context.Background(), application.LiveState{SessionID: "s1", Slide: 2, SlideCount: 5, Version: 7, Active: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	var msg stateMessage
	if err := json.Unmarshal(sendBody, &msg); err != nil || msg.Slide != 2 || msg.Version != 7 {
		t.Fatalf("unexpected broadcast body %s err=%v", sendBody, err)
	}

	token := strings.TrimPrefix(sendAuth, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("want a JWT in Authorization, got %q", sendAuth)
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != parts[2] {
		t.Fatal("JWT signature does not verify with the access key")
	}
	claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if !strings.Contains(string(claims), `/api/hubs/malus/:send?api-version=`+webPubSubAPIVersion) {
		t.Fatalf("JWT audience must be the request URL, got %s", claims)
	}

	conn, err := wps.Connection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(conn.URL)
	if conn.Kind != "webpubsub" || u.Scheme != "ws" || u.Path != "/client/hubs/malus" || u.Query().Get("access_token") != "client-token" {
		t.Fatalf("unexpected client connection: %+v", conn)
	}
	if !strings.Contains(tokenQuery, "minutesToExpire=60") {
		t.Fatalf("want minutesToExpire in token request, got %q", tokenQuery)
	}
}

func TestWebPubSubRefusesKeyOutsideLocal(t *testing.T) {
	if _, err := NewWebPubSub(WebPubSubConfig{Endpoint: "https://x.webpubsub.azure.com", Hub: "malus", AccessKey: "k"}, false); err == nil {
		t.Fatal("want error for an access key outside local")
	}
	if _, err := NewWebPubSub(WebPubSubConfig{Endpoint: "https://x.webpubsub.azure.com", Hub: "bad-name!"}, true); err == nil {
		t.Fatal("want error for an invalid hub name")
	}
}
