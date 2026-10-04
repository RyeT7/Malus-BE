package application

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"malus-be/internal/worker/domain"
)

type Handler func(ctx context.Context, job *domain.Job) error

type Processor struct {
	handlers map[string]Handler
	log      *slog.Logger
	backoff  time.Duration
}

func NewProcessor(log *slog.Logger, backoff time.Duration) *Processor {
	return &Processor{handlers: make(map[string]Handler), log: log, backoff: backoff}
}

func (p *Processor) Handle(kind string, h Handler) {
	p.handlers[kind] = h
}

func (p *Processor) Run(ctx context.Context, queue domain.Queue) error {
	for {
		job, err := queue.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		p.Process(ctx, job)
	}
}

func (p *Processor) Process(ctx context.Context, job *domain.Job) {
	log := p.log.With("job_id", job.ID().String(), "job_kind", job.Kind())

	handler, ok := p.handlers[job.Kind()]
	if !ok {
		job.DeadLetter("no handler registered")
		log.WarnContext(ctx, "job dead-lettered", "reason", job.LastError())
		return
	}

	for {
		if err := job.Start(); err != nil {
			log.ErrorContext(ctx, "job cannot start", "error", err)
			return
		}
		err := handler(ctx, job)
		if err == nil {
			job.Succeed()
			log.InfoContext(ctx, "job succeeded", "attempts", job.Attempts())
			return
		}
		job.Fail(err)
		if !job.CanRetry() {
			log.ErrorContext(ctx, "job dead-lettered", "attempts", job.Attempts(), "error", err)
			return
		}
		log.WarnContext(ctx, "job failed, retrying", "attempts", job.Attempts(), "error", err)
		if !sleep(ctx, p.delay(job.Attempts())) {
			return
		}
	}
}

func (p *Processor) delay(attempt int) time.Duration {
	base := p.backoff << (attempt - 1)
	return base/2 + rand.N(base/2+1)
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
