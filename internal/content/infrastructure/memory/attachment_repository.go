package memory

import (
	"context"
	"sort"
	"sync"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

type AttachmentRepository struct {
	mu    sync.RWMutex
	items map[kernel.ID]domain.AttachmentSnapshot
}

func NewAttachmentRepository() *AttachmentRepository {
	return &AttachmentRepository{items: make(map[kernel.ID]domain.AttachmentSnapshot)}
}

func (r *AttachmentRepository) Get(_ context.Context, id kernel.ID) (*domain.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.items[id]
	if !ok {
		return nil, kernel.NotFound("attachment %s", id)
	}
	return domain.RestoreAttachment(snap), nil
}

func (r *AttachmentRepository) GetMany(_ context.Context, ids []kernel.ID) (map[kernel.ID]*domain.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	found := make(map[kernel.ID]*domain.Attachment, len(ids))
	for _, id := range ids {
		if snap, ok := r.items[id]; ok {
			found[id] = domain.RestoreAttachment(snap)
		}
	}
	return found, nil
}

func (r *AttachmentRepository) List(_ context.Context) ([]*domain.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*domain.Attachment, 0, len(r.items))
	for _, snap := range r.items {
		list = append(list, domain.RestoreAttachment(snap))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt().After(list[j].CreatedAt()) })
	return list, nil
}

func (r *AttachmentRepository) Save(_ context.Context, a *domain.Attachment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[a.ID()] = a.Snapshot()
	return nil
}
