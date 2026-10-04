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

func (r *SessionRepository) Save(_ context.Context, s *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[s.ID()] = s.Snapshot()
	return nil
}
