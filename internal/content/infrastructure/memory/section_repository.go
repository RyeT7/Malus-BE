package memory

import (
	"context"
	"slices"
	"sync"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type SectionRepository struct {
	mu    sync.RWMutex
	items map[kernel.ID]domain.Snapshot
	order []kernel.ID
}

func NewSectionRepository() *SectionRepository {
	return &SectionRepository{items: make(map[kernel.ID]domain.Snapshot)}
}

func (r *SectionRepository) Get(_ context.Context, id kernel.ID) (*domain.Section, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.items[id]
	if !ok {
		return nil, kernel.NotFound("section %s", id)
	}
	return domain.Restore(snap), nil
}

func (r *SectionRepository) List(_ context.Context) ([]*domain.Section, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sections := make([]*domain.Section, 0, len(r.order))
	for _, id := range r.order {
		sections = append(sections, domain.Restore(r.items[id]))
	}
	return sections, nil
}

func (r *SectionRepository) Save(_ context.Context, s *domain.Section) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := s.Snapshot()
	current, exists := r.items[snap.ID]
	if exists != (snap.Revision != 0) || (exists && current.Revision != snap.Revision) {
		return kernel.ErrConcurrentUpdate
	}
	if !exists {
		r.order = append(r.order, snap.ID)
	}
	snap.Revision++
	r.items[snap.ID] = snap
	return nil
}

func (r *SectionRepository) Delete(_ context.Context, s *domain.Section) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.items[s.ID()]
	if !exists || current.Revision != s.Revision() {
		return kernel.ErrConcurrentUpdate
	}
	delete(r.items, s.ID())
	r.order = slices.DeleteFunc(r.order, func(id kernel.ID) bool { return id == s.ID() })
	return nil
}

func (r *SectionRepository) Reorder(_ context.Context, ids []kernel.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(ids) != len(r.items) {
		return kernel.Conflict("the sections changed while reordering; reload and try again")
	}
	for _, id := range ids {
		if _, ok := r.items[id]; !ok {
			return kernel.Conflict("the sections changed while reordering; reload and try again")
		}
	}
	r.order = slices.Clone(ids)
	return nil
}
