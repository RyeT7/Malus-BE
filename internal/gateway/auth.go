package gateway

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"malus-be/internal/platform/httpx"
)

type Identity struct {
	Subject string
	Roles   []string
}

func (i Identity) HasRole(role string) bool { return slices.Contains(i.Roles, role) }

type Authenticator interface {
	Authenticate(r *http.Request) (Identity, error)
}

type JWTAuthenticator struct {
	verifier *Verifier
}

func NewJWTAuthenticator(v *Verifier) *JWTAuthenticator {
	return &JWTAuthenticator{verifier: v}
}

func (a *JWTAuthenticator) Authenticate(r *http.Request) (Identity, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return Identity{}, ErrNoCredentials
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return Identity{}, ErrInvalidToken
	}
	claims, err := a.verifier.Verify(r.Context(), token)
	if err != nil {
		return Identity{}, err
	}
	subject := claims.ObjectID
	if subject == "" {
		subject = claims.Subject
	}
	return Identity{Subject: subject, Roles: claims.Roles}, nil
}

type DevAuthenticator struct {
	identity Identity
}

func NewDevAuthenticator(adminRole string) *DevAuthenticator {
	return &DevAuthenticator{identity: Identity{Subject: "local-dev", Roles: []string{adminRole}}}
}

func (a *DevAuthenticator) Authenticate(*http.Request) (Identity, error) {
	return a.identity, nil
}

type identityKey struct{}

func identityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

func authorize(access Access, authn Authenticator, adminRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := authn.Authenticate(r)
		switch {
		case errors.Is(err, ErrNoCredentials):
			if access == Admin {
				w.Header().Set("WWW-Authenticate", `Bearer`)
				httpx.Problem(w, r, http.StatusUnauthorized, "Unauthorized", "a bearer token is required")
				return
			}
			next.ServeHTTP(w, r)
			return
		case err != nil:
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			httpx.Problem(w, r, http.StatusUnauthorized, "Unauthorized", "the bearer token is invalid or expired")
			return
		}
		if access == Admin && !id.HasRole(adminRole) {
			httpx.Problem(w, r, http.StatusForbidden, "Forbidden", "the admin role is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, id)))
	})
}
