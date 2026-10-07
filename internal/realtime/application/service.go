package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"malus-be/internal/kernel"
	"malus-be/internal/realtime/domain"
)

type LiveState struct {
	SessionID  string
	Slide      int
	SlideCount int
	Version    int64
	Active     bool
}

type Connection struct {
	Kind string
	URL  string
}

type Broadcaster interface {
	Publish(ctx context.Context, state LiveState) error
	Connection(ctx context.Context) (Connection, error)
}

type Service struct {
	sessions    domain.SessionRepository
	broadcaster Broadcaster
	events      kernel.EventPublisher
	log         *slog.Logger
	now         kernel.Clock
}

func NewService(sessions domain.SessionRepository, broadcaster Broadcaster, events kernel.EventPublisher, log *slog.Logger, now kernel.Clock) *Service {
	return &Service{sessions: sessions, broadcaster: broadcaster, events: events, log: log, now: now}
}

type SessionView struct {
	ID         string
	SlideCount int
	Slide      int
	Version    int64
	Active     bool
	StartedAt  time.Time
	UpdatedAt  time.Time
	EndedAt    *time.Time
}

func toSessionView(s *domain.Session) SessionView {
	return SessionView{
		ID:         s.ID().String(),
		SlideCount: s.SlideCount(),
		Slide:      s.Slide(),
		Version:    s.Version(),
		Active:     s.Active(),
		StartedAt:  s.StartedAt(),
		UpdatedAt:  s.UpdatedAt(),
		EndedAt:    s.EndedAt(),
	}
}

func toLiveState(s *domain.Session) LiveState {
	return LiveState{SessionID: s.ID().String(), Slide: s.Slide(), SlideCount: s.SlideCount(), Version: s.Version(), Active: s.Active()}
}

func (s *Service) StartSession(ctx context.Context, slideCount int) (SessionView, error) {
	session, err := domain.Start(slideCount, s.now())
	if err != nil {
		return SessionView{}, err
	}
	if previous, err := s.sessions.FindActive(ctx); err == nil {
		previous.End(s.now())
		if err := s.commit(ctx, previous); err != nil {
			return SessionView{}, err
		}
	} else if !errors.Is(err, kernel.ErrNotFound) {
		return SessionView{}, err
	}
	if err := s.commit(ctx, session); err != nil {
		return SessionView{}, err
	}
	return toSessionView(session), nil
}

func (s *Service) GoToSlide(ctx context.Context, id kernel.ID, slide int) (SessionView, error) {
	return s.change(ctx, id, func(session *domain.Session) error {
		return session.GoTo(slide, s.now())
	})
}

func (s *Service) EndSession(ctx context.Context, id kernel.ID) (SessionView, error) {
	return s.change(ctx, id, func(session *domain.Session) error {
		session.End(s.now())
		return nil
	})
}

func (s *Service) GetSession(ctx context.Context, id kernel.ID) (SessionView, error) {
	session, err := s.sessions.Get(ctx, id)
	if err != nil {
		return SessionView{}, err
	}
	return toSessionView(session), nil
}

func (s *Service) LiveSession(ctx context.Context) (SessionView, error) {
	session, err := s.sessions.FindActive(ctx)
	if err != nil {
		return SessionView{}, err
	}
	return toSessionView(session), nil
}

func (s *Service) Connection(ctx context.Context) (Connection, error) {
	return s.broadcaster.Connection(ctx)
}

const maxConcurrentRetries = 3

func (s *Service) change(ctx context.Context, id kernel.ID, apply func(*domain.Session) error) (SessionView, error) {
	for attempt := 1; ; attempt++ {
		session, err := s.sessions.Get(ctx, id)
		if err != nil {
			return SessionView{}, err
		}
		if err := apply(session); err != nil {
			return SessionView{}, err
		}
		err = s.commit(ctx, session)
		if errors.Is(err, kernel.ErrConcurrentUpdate) && attempt < maxConcurrentRetries {
			continue
		}
		if err != nil {
			return SessionView{}, err
		}
		return toSessionView(session), nil
	}
}

func (s *Service) commit(ctx context.Context, session *domain.Session) error {
	events := session.PullEvents()
	if len(events) > 0 {
		if err := s.sessions.Save(ctx, session); err != nil {
			return err
		}
		if err := s.events.Publish(ctx, events...); err != nil {
			s.log.WarnContext(ctx, "publishing session events failed", "error", err)
		}
	}
	if err := s.broadcaster.Publish(ctx, toLiveState(session)); err != nil {
		s.log.WarnContext(ctx, "broadcasting live state failed", "session_id", session.ID().String(), "error", err)
	}
	return nil
}
