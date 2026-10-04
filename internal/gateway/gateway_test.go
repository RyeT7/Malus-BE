package gateway

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"malus-be/internal/platform/httpx"
)

const (
	testIssuer   = "https://issuer.test/v2.0"
	testAudience = "api://malus"
	testKid      = "key-1"
)

type upstreamSeen struct {
	path    string
	userID  string
	roles   string
	request string
}

func newTestGateway(t *testing.T) (http.Handler, *rsa.PrivateKey, *upstreamSeen) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"kid": testKid,
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwks.Close)

	seen := &upstreamSeen{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.path = r.URL.Path
		seen.userID = r.Header.Get(httpx.UserIDHeader)
		seen.roles = r.Header.Get(httpx.UserRolesHeader)
		seen.request = r.Header.Get(httpx.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	u, _ := url.Parse(upstream.URL)
	cfg := Config{
		Upstreams:      map[string]*url.URL{"content": u, "interaction": u, "realtime": u},
		AllowedOrigins: []string{"http://localhost:5173"},
		Auth:           AuthConfig{AdminRole: "admin"},
		RateLimit:      RateLimitConfig{RPS: 1000, Burst: 1000, WriteRPS: 1000, WriteBurst: 1000},
	}
	authn := NewJWTAuthenticator(NewVerifier(NewJWKS(jwks.URL), testIssuer, testAudience))
	h, err := New(cfg, authn, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return httpx.Chain(h, httpx.RequestID), key, seen
}

func sign(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	input := enc(map[string]string{"alg": "RS256", "kid": testKid, "typ": "JWT"}) + "." + enc(claims)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func claims(roles []string, exp time.Time) map[string]any {
	return map[string]any{
		"iss":   testIssuer,
		"aud":   testAudience,
		"oid":   "user-123",
		"exp":   exp.Unix(),
		"roles": roles,
	}
}

func do(h http.Handler, method, path, token string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPublicRouteStripsSpoofedIdentity(t *testing.T) {
	h, _, seen := newTestGateway(t)
	w := do(h, http.MethodGet, "/v1/questions", "", map[string]string{httpx.UserIDHeader: "attacker", httpx.UserRolesHeader: "admin"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if seen.userID != "" || seen.roles != "" {
		t.Fatalf("spoofed identity reached upstream: id=%q roles=%q", seen.userID, seen.roles)
	}
	if seen.request == "" || seen.request != w.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("request id not propagated: upstream=%q response=%q", seen.request, w.Header().Values(httpx.RequestIDHeader))
	}
	if n := len(w.Header().Values(httpx.RequestIDHeader)); n != 1 {
		t.Fatalf("want one X-Request-ID header, got %d", n)
	}
}

func TestAdminRouteAuthorization(t *testing.T) {
	h, key, seen := newTestGateway(t)
	future := time.Now().Add(time.Hour)

	cases := []struct {
		name  string
		token string
		want  int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"garbage token", "not.a.jwt", http.StatusUnauthorized},
		{"expired", sign(t, key, claims([]string{"admin"}, time.Now().Add(-time.Hour))), http.StatusUnauthorized},
		{"wrong audience", sign(t, key, map[string]any{"iss": testIssuer, "aud": "other", "oid": "u", "exp": future.Unix()}), http.StatusUnauthorized},
		{"missing role", sign(t, key, claims(nil, future)), http.StatusForbidden},
		{"admin", sign(t, key, claims([]string{"admin"}, future)), http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := do(h, http.MethodPut, "/v1/sessions/abc/slide", tc.token, nil); w.Code != tc.want {
				t.Fatalf("want %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
	if seen.userID != "user-123" || seen.roles != "admin" || seen.path != "/v1/sessions/abc/slide" {
		t.Fatalf("identity not forwarded: %+v", seen)
	}
}

func TestTamperedTokenRejected(t *testing.T) {
	h, _, _ := newTestGateway(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	forged := sign(t, other, claims([]string{"admin"}, time.Now().Add(time.Hour)))
	if w := do(h, http.MethodPost, "/v1/sessions", forged, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for token signed by another key, got %d", w.Code)
	}
}

func TestUnknownRouteIsNotFound(t *testing.T) {
	h, _, _ := newTestGateway(t)
	if w := do(h, http.MethodDelete, "/v1/sections", "", nil); w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	if w := do(h, http.MethodGet, "/internal/debug", "", nil); w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	h, _, _ := newTestGateway(t)
	w := do(h, http.MethodOptions, "/v1/questions", "", map[string]string{
		"Origin":                        "http://localhost:5173",
		"Access-Control-Request-Method": "POST",
	})
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("preflight failed: %d %v", w.Code, w.Header())
	}

	w = do(h, http.MethodGet, "/v1/questions", "", map[string]string{"Origin": "https://evil.test"})
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin must not get CORS headers")
	}
}
