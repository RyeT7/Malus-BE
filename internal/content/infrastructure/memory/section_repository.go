package memory

import (
	"context"
	"sort"
	"sync"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type SectionRepository struct {
	mu    sync.RWMutex
	items map[kernel.ID]domain.Snapshot
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

func (r *SectionRepository) FindByKind(_ context.Context, kind domain.Kind) (*domain.Section, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, snap := range r.items {
		if snap.Kind == kind {
			return domain.Restore(snap), nil
		}
	}
	return nil, kernel.NotFound("section of kind %q", kind)
}

func (r *SectionRepository) List(_ context.Context) ([]*domain.Section, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sections := make([]*domain.Section, 0, len(r.items))
	for _, snap := range r.items {
		sections = append(sections, domain.Restore(snap))
	}
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].CreatedAt().Before(sections[j].CreatedAt())
	})
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
	for _, other := range r.items {
		if other.ID != snap.ID && other.Kind == snap.Kind {
			return kernel.Conflict("section of kind %q already exists", snap.Kind)
		}
	}
	snap.Revision++
	r.items[snap.ID] = snap
	return nil
}
