package memory

import (
	"context"

	"malus-be/internal/worker/domain"
)

type Queue struct {
	jobs chan *domain.Job
}

func NewQueue(size int) *Queue {
	return &Queue{jobs: make(chan *domain.Job, size)}
}

func (q *Queue) Enqueue(ctx context.Context, job *domain.Job) error {
	select {
	case q.jobs <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) Receive(ctx context.Context) (*domain.Job, error) {
	select {
	case job := <-q.jobs:
		return job, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
