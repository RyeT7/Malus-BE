package http

import (
	"net/http"
	"strconv"
	"strings"

	"malus-be/internal/content/application"
	"malus-be/internal/platform/httpx"
)

func sectionETag(revision int64) string {
	return `"` + strconv.FormatInt(revision, 10) + `"`
}

func writeSection(w http.ResponseWriter, status int, view application.SectionView) {
	w.Header().Set("ETag", sectionETag(view.Revision))
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, status, toSectionResponse(view))
}

func requireRevision(w http.ResponseWriter, r *http.Request) (int64, bool) {
	header := strings.TrimSpace(r.Header.Get("If-Match"))
	if header == "" {
		httpx.Problem(w, r, http.StatusPreconditionRequired, "Precondition Required", "send If-Match with the section's current ETag")
		return 0, false
	}
	value := strings.Trim(strings.TrimPrefix(header, "W/"), `"`)
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil || revision < 1 {
		httpx.Problem(w, r, http.StatusPreconditionFailed, "Precondition Failed", "If-Match does not match the section's current ETag")
		return 0, false
	}
	return revision, true
}
