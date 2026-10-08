package http

import (
	"time"

	"malus-be/internal/realtime/application"
)

type startRequest struct {
	SlideCount int `json:"slideCount"`
}

type slideRequest struct {
	Slide int `json:"slide"`
}

type sessionResponse struct {
	ID         string     `json:"id"`
	SlideCount int        `json:"slideCount"`
	Slide      int        `json:"slide"`
	Version    int64      `json:"version"`
	Active     bool       `json:"active"`
	StartedAt  time.Time  `json:"startedAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
}

type connectionResponse struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

func toSessionResponse(v application.SessionView) sessionResponse {
	return sessionResponse{
		ID:         v.ID,
		SlideCount: v.SlideCount,
		Slide:      v.Slide,
		Version:    v.Version,
		Active:     v.Active,
		StartedAt:  v.StartedAt,
		UpdatedAt:  v.UpdatedAt,
		EndedAt:    v.EndedAt,
	}
}
