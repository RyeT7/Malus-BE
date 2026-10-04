package application

import (
	"context"
	"time"

	"malus-be/internal/kernel"
	"malus-be/internal/realtime/domain"
)

type Service struct {
	sessions domain.SessionRepository
	events   kernel.EventPublisher
	now      kernel.Clock
}

func NewService(sessions domain.SessionRepository, events kernel.EventPublisher, now kernel.Clock) *Service {
	return &Service{sessions: sessions, events: events, now: now}
}

type SessionView struct {
	ID         string
	SlideCount int
	Slide      int
	StartedAt  time.Time
	UpdatedAt  time.Time
}

func toSessionView(s *domain.Session) SessionView {
	return SessionView{
		ID:         s.ID().String(),
		SlideCount: s.SlideCount(),
		Slide:      s.Slide(),
		StartedAt:  s.StartedAt(),
		UpdatedAt:  s.UpdatedAt(),
	}
}

func (s *Service) StartSession(ctx context.Context, slideCount int) (SessionView, error) {
	session, err := domain.Start(slideCount, s.now())
	if err != nil {
		return SessionView{}, err
	}
	if err := s.commit(ctx, session); err != nil {
		return SessionView{}, err
	}
	return toSessionView(session), nil
}

func (s *Service) GetSession(ctx context.Context, id kernel.ID) (SessionView, error) {
	session, err := s.sessions.Get(ctx, id)
	if err != nil {
		return SessionView{}, err
	}
	return toSessionView(session), nil
}

func (s *Service) GoToSlide(ctx context.Context, id kernel.ID, slide int) (SessionView, error) {
	session, err := s.sessions.Get(ctx, id)
	if err != nil {
		return SessionView{}, err
	}
	if err := session.GoTo(slide, s.now()); err != nil {
		return SessionView{}, err
	}
	if err := s.commit(ctx, session); err != nil {
		return SessionView{}, err
	}
	return toSessionView(session), nil
}

func (s *Service) commit(ctx context.Context, session *domain.Session) error {
	if err := s.sessions.Save(ctx, session); err != nil {
		return err
	}
	return s.events.Publish(ctx, session.PullEvents()...)
}
