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
	version    int64
	startedAt  time.Time
	updatedAt  time.Time
	endedAt    *time.Time
	etag       string
}

func Start(slideCount int, now time.Time) (*Session, error) {
	if slideCount < 1 {
		return nil, kernel.Invalid("slide count must be at least 1")
	}
	s := &Session{id: kernel.NewID(), slideCount: slideCount, version: 1, startedAt: now, updatedAt: now}
	s.Record(SessionStarted{EventBase: kernel.NewEventBase(s.id, now), SlideCount: slideCount})
	return s, nil
}

func (s *Session) ID() kernel.ID        { return s.id }
func (s *Session) SlideCount() int      { return s.slideCount }
func (s *Session) Slide() int           { return s.slide }
func (s *Session) Version() int64       { return s.version }
func (s *Session) StartedAt() time.Time { return s.startedAt }
func (s *Session) UpdatedAt() time.Time { return s.updatedAt }
func (s *Session) EndedAt() *time.Time  { return s.endedAt }
func (s *Session) Active() bool         { return s.endedAt == nil }

func (s *Session) GoTo(slide int, now time.Time) error {
	if !s.Active() {
		return kernel.Conflict("session %s has ended", s.id)
	}
	if slide < 0 || slide >= s.slideCount {
		return kernel.Invalid("slide %d is outside 0..%d", slide, s.slideCount-1)
	}
	if slide == s.slide {
		return nil
	}
	s.slide = slide
	s.version++
	s.updatedAt = now
	s.Record(SlideChanged{EventBase: kernel.NewEventBase(s.id, now), Slide: slide, Version: s.version})
	return nil
}

func (s *Session) End(now time.Time) {
	if !s.Active() {
		return
	}
	s.endedAt = &now
	s.version++
	s.updatedAt = now
	s.Record(SessionEnded{EventBase: kernel.NewEventBase(s.id, now), Version: s.version})
}

type Snapshot struct {
	ID         kernel.ID
	SlideCount int
	Slide      int
	Version    int64
	StartedAt  time.Time
	UpdatedAt  time.Time
	EndedAt    *time.Time
	ETag       string
}

func (s *Session) Snapshot() Snapshot {
	return Snapshot{
		ID:         s.id,
		SlideCount: s.slideCount,
		Slide:      s.slide,
		Version:    s.version,
		StartedAt:  s.startedAt,
		UpdatedAt:  s.updatedAt,
		EndedAt:    s.endedAt,
		ETag:       s.etag,
	}
}

func Restore(snap Snapshot) *Session {
	return &Session{
		id:         snap.ID,
		slideCount: snap.SlideCount,
		slide:      snap.Slide,
		version:    snap.Version,
		startedAt:  snap.StartedAt,
		updatedAt:  snap.UpdatedAt,
		endedAt:    snap.EndedAt,
		etag:       snap.ETag,
	}
}

type SessionRepository interface {
	Get(ctx context.Context, id kernel.ID) (*Session, error)
	FindActive(ctx context.Context) (*Session, error)
	Save(ctx context.Context, s *Session) error
}
