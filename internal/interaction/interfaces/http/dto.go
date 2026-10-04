package http

import (
	"time"

	"malus-be/internal/interaction/application"
)

type askRequest struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

type questionResponse struct {
	ID         string     `json:"id"`
	Text       string     `json:"text"`
	Author     string     `json:"author,omitempty"`
	Anonymous  bool       `json:"anonymous"`
	Votes      int        `json:"votes"`
	Answered   bool       `json:"answered"`
	AskedAt    time.Time  `json:"askedAt"`
	AnsweredAt *time.Time `json:"answeredAt,omitempty"`
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

func toQuestionResponse(v application.QuestionView) questionResponse {
	return questionResponse{
		ID:         v.ID,
		Text:       v.Text,
		Author:     v.Author,
		Anonymous:  v.Anonymous,
		Votes:      v.Votes,
		Answered:   v.Answered,
		AskedAt:    v.AskedAt,
		AnsweredAt: v.AnsweredAt,
	}
}

func toList[V, R any](views []V, convert func(V) R) listResponse[R] {
	items := make([]R, 0, len(views))
	for _, v := range views {
		items = append(items, convert(v))
	}
	return listResponse[R]{Items: items}
}
