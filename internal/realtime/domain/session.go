package domain

import (
	"context"
	"time"

	"malus-be/internal/kernel"
)

type Session struct {
	kernel.AggregateRoot
	id         kernel.ID
	slideCount int
	slide      int
	startedAt  time.Time
	updatedAt  time.Time
}

func Start(slideCount int, now time.Time) (*Session, error) {
	if slideCount < 1 {
		return nil, kernel.Invalid("slide count must be at least 1")
	}
	s := &Session{id: kernel.NewID(), slideCount: slideCount, startedAt: now, updatedAt: now}
	s.Record(SessionStarted{EventBase: kernel.NewEventBase(s.id, now), SlideCount: slideCount})
	return s, nil
}

func (s *Session) ID() kernel.ID        { return s.id }
func (s *Session) SlideCount() int      { return s.slideCount }
func (s *Session) Slide() int           { return s.slide }
func (s *Session) StartedAt() time.Time { return s.startedAt }
func (s *Session) UpdatedAt() time.Time { return s.updatedAt }

func (s *Session) GoTo(slide int, now time.Time) error {
	if slide < 0 || slide >= s.slideCount {
		return kernel.Invalid("slide %d is outside 0..%d", slide, s.slideCount-1)
	}
	if slide == s.slide {
		return nil
	}
	s.slide = slide
	s.updatedAt = now
	s.Record(SlideChanged{EventBase: kernel.NewEventBase(s.id, now), Slide: slide})
	return nil
}

type Snapshot struct {
	ID         kernel.ID
	SlideCount int
	Slide      int
	StartedAt  time.Time
	UpdatedAt  time.Time
}

func (s *Session) Snapshot() Snapshot {
	return Snapshot{ID: s.id, SlideCount: s.slideCount, Slide: s.slide, StartedAt: s.startedAt, UpdatedAt: s.updatedAt}
}

func Restore(snap Snapshot) *Session {
	return &Session{id: snap.ID, slideCount: snap.SlideCount, slide: snap.Slide, startedAt: snap.StartedAt, updatedAt: snap.UpdatedAt}
}

type SessionRepository interface {
	Get(ctx context.Context, id kernel.ID) (*Session, error)
	Save(ctx context.Context, s *Session) error
}
