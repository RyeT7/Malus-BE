package broadcast

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"malus-be/internal/realtime/application"
)

const (
	webPubSubAPIVersion = "2024-01-01"
	webPubSubScope      = "https://webpubsub.azure.com/.default"
	clientTokenMinutes  = 60
)

var hubName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,127}$`)

type WebPubSubConfig struct {
	Endpoint  string
	Hub       string
	AccessKey string
}

type WebPubSub struct {
	endpoint *url.URL
	hub      string
	key      string
	cred     azcore.TokenCredential
	client   *http.Client
	now      func() time.Time
}

func NewWebPubSub(cfg WebPubSubConfig, allowKey bool) (*WebPubSub, error) {
	endpoint, err := url.Parse(strings.TrimRight(cfg.Endpoint, "/"))
	if err != nil || (endpoint.Scheme != "https" && endpoint.Scheme != "http") || endpoint.Host == "" {
		return nil, errors.New("WEBPUBSUB_ENDPOINT must be an absolute http(s) URL")
	}
	if !hubName.MatchString(cfg.Hub) {
		return nil, fmt.Errorf("invalid Web PubSub hub name %q", cfg.Hub)
	}
	w := &WebPubSub{endpoint: endpoint, hub: cfg.Hub, client: &http.Client{Timeout: 10 * time.Second}, now: time.Now}
	if cfg.AccessKey != "" {
		if !allowKey {
			return nil, errors.New("WEBPUBSUB_ACCESS_KEY is only allowed when APP_ENV=local; use a managed identity")
		}
		w.key = cfg.AccessKey
		return w, nil
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	w.cred = cred
	return w, nil
}

func (w *WebPubSub) Publish(ctx context.Context, state application.LiveState) error {
	body, err := encode(state)
	if err != nil {
		return err
	}
	resp, err := w.call(ctx, ":send", nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("web pubsub send: status %d", resp.StatusCode)
	}
	return nil
}

func (w *WebPubSub) Connection(ctx context.Context) (application.Connection, error) {
	resp, err := w.call(ctx, ":generateToken", url.Values{"minutesToExpire": {fmt.Sprint(clientTokenMinutes)}}, nil)
	if err != nil {
		return application.Connection{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return application.Connection{}, fmt.Errorf("web pubsub token: status %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil || out.Token == "" {
		return application.Connection{}, errors.New("web pubsub token: empty response")
	}
	scheme := "wss"
	if w.endpoint.Scheme == "http" {
		scheme = "ws"
	}
	client := url.URL{Scheme: scheme, Host: w.endpoint.Host, Path: "/client/hubs/" + w.hub, RawQuery: url.Values{"access_token": {out.Token}}.Encode()}
	return application.Connection{Kind: "webpubsub", URL: client.String()}, nil
}

func (w *WebPubSub) call(ctx context.Context, action string, extra url.Values, body []byte) (*http.Response, error) {
	query := url.Values{"api-version": {webPubSubAPIVersion}}
	for k, v := range extra {
		query[k] = v
	}
	target := *w.endpoint
	target.Path = "/api/hubs/" + w.hub + "/" + action
	target.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	token, err := w.authToken(ctx, target.String())
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web pubsub %s: %w", action, err)
	}
	return resp, nil
}

func (w *WebPubSub) authToken(ctx context.Context, audience string) (string, error) {
	if w.key == "" {
		tok, err := w.cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{webPubSubScope}})
		if err != nil {
			return "", fmt.Errorf("web pubsub credential: %w", err)
		}
		return tok.Token, nil
	}
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := w.now()
	claims, err := json.Marshal(map[string]any{"aud": audience, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()})
	if err != nil {
		return "", err
	}
	payload := header + "." + enc.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(w.key))
	mac.Write([]byte(payload))
	return payload + "." + enc.EncodeToString(mac.Sum(nil)), nil
}
