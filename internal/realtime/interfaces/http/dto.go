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
	ID         string    `json:"id"`
	SlideCount int       `json:"slideCount"`
	Slide      int       `json:"slide"`
	StartedAt  time.Time `json:"startedAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func toSessionResponse(v application.SessionView) sessionResponse {
	return sessionResponse{
		ID:         v.ID,
		SlideCount: v.SlideCount,
		Slide:      v.Slide,
		StartedAt:  v.StartedAt,
		UpdatedAt:  v.UpdatedAt,
	}
}
