package memory

import (
	"context"
	"sync"

	"malus-be/internal/kernel"
	"malus-be/internal/realtime/domain"
)

type SessionRepository struct {
	mu    sync.RWMutex
	items map[kernel.ID]domain.Snapshot
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{items: make(map[kernel.ID]domain.Snapshot)}
}

func (r *SessionRepository) Get(_ context.Context, id kernel.ID) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.items[id]
	if !ok {
		return nil, kernel.NotFound("session %s", id)
	}
	return domain.Restore(snap), nil
}

func (r *SessionRepository) FindActive(_ context.Context) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *domain.Snapshot
	for _, snap := range r.items {
		if snap.EndedAt != nil {
			continue
		}
		if latest == nil || snap.StartedAt.After(latest.StartedAt) {
			s := snap
			latest = &s
		}
	}
	if latest == nil {
		return nil, kernel.NotFound("no live session")
	}
	return domain.Restore(*latest), nil
}

func (r *SessionRepository) Save(_ context.Context, s *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := s.Snapshot()
	current, exists := r.items[snap.ID]
	if exists != (snap.ETag != "") || (exists && current.ETag != snap.ETag) {
		return kernel.ErrConcurrentUpdate
	}
	snap.ETag = kernel.NewID().String()
	r.items[snap.ID] = snap
	return nil
}
