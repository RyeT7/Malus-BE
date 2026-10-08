package domain

import "malus-be/internal/kernel"

type SessionStarted struct {
	kernel.EventBase
	SlideCount int `json:"slideCount"`
}

func (SessionStarted) EventType() string { return "realtime.session.started" }

type SlideChanged struct {
	kernel.EventBase
	Slide   int   `json:"slide"`
	Version int64 `json:"version"`
}

func (SlideChanged) EventType() string { return "realtime.session.slide_changed" }

type SessionEnded struct {
	kernel.EventBase
	Version int64 `json:"version"`
}

func (SessionEnded) EventType() string { return "realtime.session.ended" }
