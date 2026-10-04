package main

import (
	"context"
	"errors"
	"time"

	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/service"
	"malus-be/internal/worker/application"
	"malus-be/internal/worker/domain"
	"malus-be/internal/worker/infrastructure/memory"
)

func main() {
	service.Run("worker", func(ctx context.Context, rt service.Runtime) ([]httpx.Check, error) {
		queue := memory.NewQueue(64)
		processor := application.NewProcessor(rt.Log, 500*time.Millisecond)
		processor.Handle("pdf.export", func(ctx context.Context, job *domain.Job) error {
			rt.Log.InfoContext(ctx, "pdf export requested", "job_id", job.ID().String())
			return nil
		})
		processor.Handle("notification.send", func(ctx context.Context, job *domain.Job) error {
			rt.Log.InfoContext(ctx, "notification requested", "job_id", job.ID().String())
			return nil
		})

		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			if err := processor.Run(ctx, queue); err != nil {
				rt.Log.Error("processor stopped", "error", err)
			}
		}()

		ready := func(context.Context) error {
			select {
			case <-stopped:
				return errors.New("processor is not running")
			default:
				return nil
			}
		}
		return []httpx.Check{ready}, nil
	})
}
