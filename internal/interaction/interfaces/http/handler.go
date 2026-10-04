package http

import (
	"net/http"

	"malus-be/internal/interaction/application"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/httpx"
)

const ViewerIDHeader = "X-Viewer-ID"

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/questions", h.list)
	mux.HandleFunc("POST /v1/questions", h.ask)
	mux.HandleFunc("POST /v1/questions/{id}/upvotes", h.upvote)
	mux.HandleFunc("POST /v1/questions/{id}/answer", h.answer)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListQuestions(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toList(views, toQuestionResponse))
}

func (h *Handler) ask(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.AskQuestion(r.Context(), req.Text, req.Author)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/questions/"+view.ID)
	httpx.JSON(w, http.StatusCreated, toQuestionResponse(view))
}

func (h *Handler) upvote(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Upvote(r.Context(), kernel.ID(r.PathValue("id")), r.Header.Get(ViewerIDHeader))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toQuestionResponse(view))
}

func (h *Handler) answer(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.MarkAnswered(r.Context(), kernel.ID(r.PathValue("id")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toQuestionResponse(view))
}
