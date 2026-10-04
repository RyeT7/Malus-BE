package http

import (
	"net/http"

	"malus-be/internal/content/application"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/presentation", h.presentation)
	mux.HandleFunc("GET /v1/sections", h.list)
	mux.HandleFunc("POST /v1/sections", h.create)
	mux.HandleFunc("GET /v1/sections/{id}", h.get)
	mux.HandleFunc("PUT /v1/sections/{id}/draft", h.editDraft)
	mux.HandleFunc("POST /v1/sections/{id}/publish", h.publish)
	mux.HandleFunc("GET /v1/sections/{id}/versions", h.versions)
	mux.HandleFunc("POST /v1/sections/{id}/rollback", h.rollback)
}

func (h *Handler) presentation(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.GetPresentation(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	etag := `"` + view.Revision + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if httpx.ETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	httpx.JSON(w, http.StatusOK, toPresentationResponse(view))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListSections(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toList(views, toSectionResponse))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.CreateSection(r.Context(), application.CreateSection{Kind: req.Kind, Title: req.Title, Body: req.Body})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/sections/"+view.ID)
	httpx.JSON(w, http.StatusCreated, toSectionResponse(view))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.GetSection(r.Context(), kernel.ID(r.PathValue("id")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSectionResponse(view))
}

func (h *Handler) editDraft(w http.ResponseWriter, r *http.Request) {
	var req draftRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.EditDraft(r.Context(), application.EditDraft{ID: kernel.ID(r.PathValue("id")), Title: req.Title, Body: req.Body})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSectionResponse(view))
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Publish(r.Context(), kernel.ID(r.PathValue("id")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSectionResponse(view))
}

func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListVersions(r.Context(), kernel.ID(r.PathValue("id")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toList(views, toVersionResponse))
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	var req rollbackRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.Rollback(r.Context(), kernel.ID(r.PathValue("id")), req.Version)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSectionResponse(view))
}
