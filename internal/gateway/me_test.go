package gateway

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func decodeMe(t *testing.T, w *httptest.ResponseRecorder) meResponse {
	t.Helper()
	var body meResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode /v1/me: %v", err)
	}
	return body
}

func TestMeRequiresToken(t *testing.T) {
	h, _, _ := newTestGateway(t)
	w := do(h, http.MethodGet, "/v1/me", "", nil)
	if w.Code != http.StatusUnauthorized || w.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("want 401 with WWW-Authenticate, got %d %v", w.Code, w.Header())
	}
	if w := do(h, http.MethodGet, "/v1/me", "not.a.jwt", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for invalid token, got %d", w.Code)
	}
}

func TestMeReportsAdminRole(t *testing.T) {
	h, key, _ := newTestGateway(t)
	future := time.Now().Add(time.Hour)

	adminClaims := claims([]string{"admin"}, future)
	adminClaims["name"] = "Ryuu"
	w := do(h, http.MethodGet, "/v1/me", sign(t, key, adminClaims), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("identity responses must not be cached")
	}
	if body := decodeMe(t, w); !body.Admin || body.Name != "Ryuu" || body.Subject != "user-123" {
		t.Fatalf("unexpected body: %+v", body)
	}

	viewerClaims := claims(nil, future)
	viewerClaims["preferred_username"] = "someone@school.edu"
	w = do(h, http.MethodGet, "/v1/me", sign(t, key, viewerClaims), nil)
	body := decodeMe(t, w)
	if w.Code != http.StatusOK || body.Admin || body.Name != "someone@school.edu" || body.Roles == nil {
		t.Fatalf("want non-admin with username fallback and empty roles, got %d %+v", w.Code, body)
	}
}

func TestMeWithAuthDisabled(t *testing.T) {
	upstream, _ := url.Parse("http://127.0.0.1:1")
	cfg := Config{
		Upstreams: map[string]*url.URL{"content": upstream, "interaction": upstream, "realtime": upstream},
		Auth:      AuthConfig{AdminRole: "admin"},
		RateLimit: RateLimitConfig{RPS: 100, Burst: 100, WriteRPS: 100, WriteBurst: 100},
	}
	h, err := New(cfg, NewDevAuthenticator("admin"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	w := do(h, http.MethodGet, "/v1/me", "", nil)
	if body := decodeMe(t, w); w.Code != http.StatusOK || !body.Admin || body.Subject != "local-dev" {
		t.Fatalf("want local dev admin, got %d %+v", w.Code, body)
	}
}
