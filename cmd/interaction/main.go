package main

import (
	"context"
	"os"

	"malus-be/internal/interaction/application"
	"malus-be/internal/interaction/infrastructure/cosmos"
	interactionhttp "malus-be/internal/interaction/interfaces/http"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/config"
	"malus-be/internal/platform/cosmosdb"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/messaging"
	"malus-be/internal/platform/service"
)

func main() {
	service.Run("interaction", serve)
}

func serve(_ context.Context, rt service.Runtime) ([]httpx.Check, error) {
	endpoint, err := config.Required("COSMOS_ENDPOINT")
	if err != nil {
		return nil, err
	}
	client, err := cosmosdb.NewClient(endpoint, os.Getenv("COSMOS_KEY"), rt.Config.IsLocal())
	if err != nil {
		return nil, err
	}
	container, err := client.NewContainer(
		config.String("COSMOS_DATABASE", "malus"),
		config.String("COSMOS_QUESTIONS_CONTAINER", "questions"),
	)
	if err != nil {
		return nil, err
	}
	repo := cosmos.NewQuestionRepository(container)

	bus := messaging.NewMemoryBus("/malus/interaction")
	bus.Subscribe(messaging.LogHandler(rt.Log))

	svc := application.NewService(repo, bus, kernel.SystemClock)
	interactionhttp.NewHandler(svc).Register(rt.Mux)
	return []httpx.Check{repo.Ping}, nil
}
