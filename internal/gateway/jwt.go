package gateway

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var (
	ErrNoCredentials = errors.New("no credentials")
	ErrInvalidToken  = errors.New("invalid token")
)

type KeySource interface {
	Key(ctx context.Context, kid string) (*rsa.PublicKey, error)
}

type Verifier struct {
	keys     KeySource
	issuer   string
	audience string
	leeway   time.Duration
	now      func() time.Time
}

func NewVerifier(keys KeySource, issuer, audience string) *Verifier {
	return &Verifier{keys: keys, issuer: issuer, audience: audience, leeway: time.Minute, now: time.Now}
}

type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

type Claims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	ObjectID  string   `json:"oid"`
	Audience  audience `json:"aud"`
	ExpiresAt float64  `json:"exp"`
	NotBefore float64  `json:"nbf"`
	Roles     []string `json:"roles"`
	Name      string   `json:"name"`
	Username  string   `json:"preferred_username"`
}

func (v *Verifier) Verify(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: malformed", ErrInvalidToken)
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Claims{}, fmt.Errorf("%w: header: %v", ErrInvalidToken, err)
	}
	if header.Alg != "RS256" {
		return Claims{}, fmt.Errorf("%w: unsupported alg %q", ErrInvalidToken, header.Alg)
	}

	key, err := v.keys.Key(ctx, header.Kid)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: signature encoding", ErrInvalidToken)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return Claims{}, fmt.Errorf("%w: bad signature", ErrInvalidToken)
	}

	var claims Claims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: claims: %v", ErrInvalidToken, err)
	}
	if err := v.validate(claims); err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return claims, nil
}

func (v *Verifier) validate(c Claims) error {
	now := v.now()
	if c.Issuer != v.issuer {
		return errors.New("issuer mismatch")
	}
	if !slices.Contains(c.Audience, v.audience) {
		return errors.New("audience mismatch")
	}
	if c.ExpiresAt == 0 || now.After(unix(c.ExpiresAt).Add(v.leeway)) {
		return errors.New("token expired")
	}
	if c.NotBefore != 0 && now.Add(v.leeway).Before(unix(c.NotBefore)) {
		return errors.New("token not yet valid")
	}
	if c.ObjectID == "" && c.Subject == "" {
		return errors.New("token has no subject")
	}
	return nil
}

func decodeSegment(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func unix(f float64) time.Time {
	return time.Unix(int64(f), 0)
}
