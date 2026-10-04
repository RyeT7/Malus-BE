package domain

import "malus-be/internal/kernel"

type SessionStarted struct {
	kernel.EventBase
	SlideCount int `json:"slideCount"`
}

func (SessionStarted) EventType() string { return "realtime.session.started" }

type SlideChanged struct {
	kernel.EventBase
	Slide int `json:"slide"`
}

func (SlideChanged) EventType() string { return "realtime.session.slide_changed" }
