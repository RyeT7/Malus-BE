package main

import (
	"context"

	"malus-be/internal/kernel"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/messaging"
	"malus-be/internal/platform/service"
	"malus-be/internal/realtime/application"
	"malus-be/internal/realtime/infrastructure/memory"
	realtimehttp "malus-be/internal/realtime/interfaces/http"
)

func main() {
	service.Run("realtime", func(_ context.Context, rt service.Runtime) ([]httpx.Check, error) {
		bus := messaging.NewMemoryBus("/malus/realtime")
		bus.Subscribe(messaging.LogHandler(rt.Log))

		svc := application.NewService(memory.NewSessionRepository(), bus, kernel.SystemClock)
		realtimehttp.NewHandler(svc).Register(rt.Mux)
		return nil, nil
	})
}
