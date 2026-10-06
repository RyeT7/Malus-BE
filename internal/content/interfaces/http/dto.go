package http

import (
	"time"

	"malus-be/internal/content/application"
)

type itemRequest struct {
	Heading  string `json:"heading"`
	Detail   string `json:"detail"`
	Semester int    `json:"semester"`
}

type createRequest struct {
	Kind  string        `json:"kind"`
	Title string        `json:"title"`
	Body  string        `json:"body"`
	Items []itemRequest `json:"items"`
}

type draftRequest struct {
	Title string        `json:"title"`
	Body  string        `json:"body"`
	Items []itemRequest `json:"items"`
}

func toItemInputs(items []itemRequest) []application.ItemInput {
	inputs := make([]application.ItemInput, len(items))
	for i, item := range items {
		inputs[i] = application.ItemInput{Heading: item.Heading, Detail: item.Detail, Semester: item.Semester}
	}
	return inputs
}

type rollbackRequest struct {
	Version int `json:"version"`
}

type itemResponse struct {
	Heading  string `json:"heading"`
	Detail   string `json:"detail"`
	Semester int    `json:"semester,omitempty"`
}

type contentResponse struct {
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Items []itemResponse `json:"items"`
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

type presentationSectionResponse struct {
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Items       []itemResponse `json:"items"`
	Version     int            `json:"version"`
	PublishedAt time.Time      `json:"publishedAt"`
}

type presentationResponse struct {
	Sections []presentationSectionResponse `json:"sections"`
}

func toPresentationResponse(v application.PresentationView) presentationResponse {
	sections := make([]presentationSectionResponse, 0, len(v.Sections))
	for _, s := range v.Sections {
		sections = append(sections, presentationSectionResponse{
			Kind:        s.Kind,
			Title:       s.Title,
			Body:        s.Body,
			Items:       toItemResponses(s.Items),
			Version:     s.Version,
			PublishedAt: s.PublishedAt,
		})
	}
	return presentationResponse{Sections: sections}
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

func toItemResponses(items []application.ItemView) []itemResponse {
	responses := make([]itemResponse, len(items))
	for i, item := range items {
		responses[i] = itemResponse{Heading: item.Heading, Detail: item.Detail, Semester: item.Semester}
	}
	return responses
}

func toContentResponse(v application.ContentView) contentResponse {
	return contentResponse{Title: v.Title, Body: v.Body, Items: toItemResponses(v.Items)}
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
