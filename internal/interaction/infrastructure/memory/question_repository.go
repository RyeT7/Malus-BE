package memory

import (
	"context"
	"sync"

	"malus-be/internal/interaction/domain"
	"malus-be/internal/kernel"
)

type QuestionRepository struct {
	mu    sync.RWMutex
	items map[kernel.ID]domain.Snapshot
}

func NewQuestionRepository() *QuestionRepository {
	return &QuestionRepository{items: make(map[kernel.ID]domain.Snapshot)}
}

func (r *QuestionRepository) Get(_ context.Context, id kernel.ID) (*domain.Question, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.items[id]
	if !ok {
		return nil, kernel.NotFound("question %s", id)
	}
	return domain.Restore(snap), nil
}

func (r *QuestionRepository) List(_ context.Context) ([]*domain.Question, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	questions := make([]*domain.Question, 0, len(r.items))
	for _, snap := range r.items {
		questions = append(questions, domain.Restore(snap))
	}
	return questions, nil
}

func (r *QuestionRepository) Save(_ context.Context, q *domain.Question) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := q.Snapshot()
	current, exists := r.items[snap.ID]
	if exists != (snap.ETag != "") || (exists && current.ETag != snap.ETag) {
		return kernel.ErrConcurrentUpdate
	}
	snap.ETag = kernel.NewID().String()
	r.items[snap.ID] = snap
	return nil
}
