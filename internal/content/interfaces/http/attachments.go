package http

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"malus-be/internal/content/application"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/httpx"
)

type uploadRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

type uploadResponse struct {
	Attachment attachmentResponse `json:"attachment"`
	UploadURL  string             `json:"uploadUrl"`
	Headers    map[string]string  `json:"headers"`
	ExpiresAt  time.Time          `json:"expiresAt"`
}

func (h *Handler) listAttachments(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.ListAttachments(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toList(views, toAttachmentResponse))
}

func (h *Handler) requestUpload(w http.ResponseWriter, r *http.Request) {
	var req uploadRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	ticket, err := h.svc.RequestUpload(r.Context(), application.RequestUpload{FileName: req.FileName, ContentType: req.ContentType, Size: req.Size})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusCreated, uploadResponse{
		Attachment: toAttachmentResponse(ticket.Attachment),
		UploadURL:  ticket.UploadURL,
		Headers:    ticket.Headers,
		ExpiresAt:  ticket.ExpiresAt,
	})
}

func (h *Handler) completeUpload(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.CompleteUpload(r.Context(), kernel.ID(r.PathValue("id")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toAttachmentResponse(view))
}

func (h *Handler) attachmentContent(w http.ResponseWriter, r *http.Request) {
	link, err := h.svc.AttachmentLink(r.Context(), kernel.ID(r.PathValue("id")), h.isAdmin(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		httpx.JSON(w, http.StatusOK, map[string]string{"url": link})
		return
	}
	http.Redirect(w, r, link, http.StatusFound)
}

func (h *Handler) isAdmin(r *http.Request) bool {
	roles := strings.Split(r.Header.Get(httpx.UserRolesHeader), ",")
	for i := range roles {
		roles[i] = strings.TrimSpace(roles[i])
	}
	return h.adminRole != "" && slices.Contains(roles, h.adminRole)
}
