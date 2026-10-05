package gateway

import (
	"errors"
	"net/http"

	"malus-be/internal/platform/httpx"
)

type meResponse struct {
	Subject string   `json:"subject"`
	Name    string   `json:"name,omitempty"`
	Roles   []string `json:"roles"`
	Admin   bool     `json:"admin"`
}

func me(authn Authenticator, adminRole string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := authn.Authenticate(r)
		switch {
		case errors.Is(err, ErrNoCredentials):
			w.Header().Set("WWW-Authenticate", `Bearer`)
			httpx.Problem(w, r, http.StatusUnauthorized, "Unauthorized", "a bearer token is required")
			return
		case err != nil:
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			httpx.Problem(w, r, http.StatusUnauthorized, "Unauthorized", "the bearer token is invalid or expired")
			return
		}
		roles := id.Roles
		if roles == nil {
			roles = []string{}
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, meResponse{Subject: id.Subject, Name: id.Name, Roles: roles, Admin: id.HasRole(adminRole)})
	})
}
