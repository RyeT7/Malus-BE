package domain

import "malus-be/internal/kernel"

type QuestionAsked struct {
	kernel.EventBase
}

func (QuestionAsked) EventType() string { return "interaction.question.asked" }

type QuestionUpvoted struct {
	kernel.EventBase
	Votes int `json:"votes"`
}

func (QuestionUpvoted) EventType() string { return "interaction.question.upvoted" }

type QuestionAnswered struct {
	kernel.EventBase
}

func (QuestionAnswered) EventType() string { return "interaction.question.answered" }
