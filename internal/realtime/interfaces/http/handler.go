package http

import (
	"net/http"

	"malus-be/internal/kernel"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/realtime/application"
)

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sessions", h.start)
	mux.HandleFunc("GET /v1/sessions/{id}", h.get)
	mux.HandleFunc("PUT /v1/sessions/{id}/slide", h.goTo)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.StartSession(r.Context(), req.SlideCount)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/sessions/"+view.ID)
	httpx.JSON(w, http.StatusCreated, toSessionResponse(view))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.GetSession(r.Context(), kernel.ID(r.PathValue("id")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSessionResponse(view))
}

func (h *Handler) goTo(w http.ResponseWriter, r *http.Request) {
	var req slideRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.GoToSlide(r.Context(), kernel.ID(r.PathValue("id")), req.Slide)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSessionResponse(view))
}
