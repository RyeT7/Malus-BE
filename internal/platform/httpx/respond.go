package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"malus-be/internal/kernel"
)

const maxBodyBytes = 1 << 20

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

type ProblemDetails struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func Problem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:      "about:blank",
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  r.URL.Path,
		RequestID: RequestIDFrom(r.Context()),
	})
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, kernel.ErrInvalid):
		Problem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
	case errors.Is(err, kernel.ErrNotFound):
		Problem(w, r, http.StatusNotFound, "Not Found", err.Error())
	case errors.Is(err, kernel.ErrConflict):
		Problem(w, r, http.StatusConflict, "Conflict", err.Error())
	case errors.Is(err, kernel.ErrPrecondition):
		Problem(w, r, http.StatusPreconditionFailed, "Precondition Failed", "the resource was changed since you loaded it; reload to get the latest version")
	case errors.Is(err, kernel.ErrConcurrentUpdate):
		Problem(w, r, http.StatusConflict, "Conflict", "the resource was modified by another request; reload and retry")
	default:
		slog.ErrorContext(r.Context(), "unhandled error", "error", err, "request_id", RequestIDFrom(r.Context()))
		Problem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
	}
}

func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return kernel.Invalid("malformed request body: %v", err)
	}
	return nil
}
