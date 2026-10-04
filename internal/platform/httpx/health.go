package httpx

import (
	"context"
	"net/http"
)

type Check func(ctx context.Context) error

func RegisterHealth(mux *http.ServeMux, checks ...Check) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		for _, check := range checks {
			if err := check(r.Context()); err != nil {
				Problem(w, r, http.StatusServiceUnavailable, "Service Unavailable", err.Error())
				return
			}
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}
