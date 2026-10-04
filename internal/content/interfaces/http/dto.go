package http

import (
	"time"

	"malus-be/internal/content/application"
)

type createRequest struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type draftRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type rollbackRequest struct {
	Version int `json:"version"`
}

type contentResponse struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type versionResponse struct {
	Number      int             `json:"number"`
	Content     contentResponse `json:"content"`
	PublishedAt time.Time       `json:"publishedAt"`
}

type sectionResponse struct {
	ID                    string           `json:"id"`
	Kind                  string           `json:"kind"`
	Draft                 contentResponse  `json:"draft"`
	Published             *versionResponse `json:"published,omitempty"`
	HasUnpublishedChanges bool             `json:"hasUnpublishedChanges"`
	CreatedAt             time.Time        `json:"createdAt"`
	UpdatedAt             time.Time        `json:"updatedAt"`
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

func toContentResponse(v application.ContentView) contentResponse {
	return contentResponse{Title: v.Title, Body: v.Body}
}

func toVersionResponse(v application.VersionView) versionResponse {
	return versionResponse{Number: v.Number, Content: toContentResponse(v.Content), PublishedAt: v.PublishedAt}
}

func toSectionResponse(v application.SectionView) sectionResponse {
	resp := sectionResponse{
		ID:                    v.ID,
		Kind:                  v.Kind,
		Draft:                 toContentResponse(v.Draft),
		HasUnpublishedChanges: v.HasUnpublishedChanges,
		CreatedAt:             v.CreatedAt,
		UpdatedAt:             v.UpdatedAt,
	}
	if v.Published != nil {
		p := toVersionResponse(*v.Published)
		resp.Published = &p
	}
	return resp
}

func toList[V, R any](views []V, convert func(V) R) listResponse[R] {
	items := make([]R, 0, len(views))
	for _, v := range views {
		items = append(items, convert(v))
	}
	return listResponse[R]{Items: items}
}
