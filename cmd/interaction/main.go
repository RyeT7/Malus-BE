package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

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

func serve(ctx context.Context, rt service.Runtime) ([]httpx.Check, error) {
	endpoint, err := config.Required("COSMOS_ENDPOINT")
	if err != nil {
		return nil, err
	}
	client, err := cosmosdb.NewClient(endpoint, os.Getenv("COSMOS_KEY"), rt.Config.IsLocal())
	if err != nil {
		return nil, err
	}
	database := config.String("COSMOS_DATABASE", "malus")
	questions := config.String("COSMOS_QUESTIONS_CONTAINER", "questions")
	if rt.Config.IsLocal() {
		if err := bootstrapLocal(ctx, client, database, questions, rt.Log); err != nil {
			return nil, err
		}
	}
	container, err := client.NewContainer(database, questions)
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

func bootstrapLocal(ctx context.Context, client *azcosmos.Client, database, container string, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		err := cosmosdb.EnsureContainer(ctx, client, database, container, cosmos.PartitionKeyPath)
		if err == nil {
			log.Info("cosmos container ready", "database", database, "container", container)
			return nil
		}
		log.Warn("waiting for cosmos emulator", "error", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("bootstrap cosmos: %w", err)
		case <-time.After(3 * time.Second):
		}
	}
}
