package main

import (
	"context"
	"net/http"
	"os"

	"malus-be/internal/kernel"
	"malus-be/internal/platform/config"
	"malus-be/internal/platform/cosmosdb"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/messaging"
	"malus-be/internal/platform/service"
	"malus-be/internal/realtime/application"
	"malus-be/internal/realtime/infrastructure/broadcast"
	"malus-be/internal/realtime/infrastructure/cosmos"
	realtimehttp "malus-be/internal/realtime/interfaces/http"
)

func main() {
	service.Run("realtime", serve)
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
	sessions := config.String("COSMOS_SESSIONS_CONTAINER", "sessions")
	if rt.Config.IsLocal() {
		if err := cosmosdb.WaitForContainer(ctx, client, database, sessions, cosmos.PartitionKeyPath, rt.Log); err != nil {
			return nil, err
		}
	}
	container, err := client.NewContainer(database, sessions)
	if err != nil {
		return nil, err
	}
	repo := cosmos.NewSessionRepository(container)

	broadcaster, err := newBroadcaster(rt)
	if err != nil {
		return nil, err
	}

	bus := messaging.NewMemoryBus("/malus/realtime")
	bus.Subscribe(messaging.LogHandler(rt.Log))

	svc := application.NewService(repo, broadcaster, bus, rt.Log, kernel.SystemClock)
	realtimehttp.NewHandler(svc).Register(rt.Mux)
	return []httpx.Check{repo.Ping}, nil
}

func newBroadcaster(rt service.Runtime) (application.Broadcaster, error) {
	if endpoint := os.Getenv("WEBPUBSUB_ENDPOINT"); endpoint != "" {
		rt.Log.Info("live updates via Azure Web PubSub", "endpoint", endpoint)
		return broadcast.NewWebPubSub(broadcast.WebPubSubConfig{
			Endpoint:  endpoint,
			Hub:       config.String("WEBPUBSUB_HUB", "malus"),
			AccessKey: os.Getenv("WEBPUBSUB_ACCESS_KEY"),
		}, rt.Config.IsLocal())
	}
	hub := broadcast.NewHub(config.String("LIVE_STREAM_URL", "/v1/live/stream"))
	rt.Mux.Handle("GET /v1/live/stream", http.Handler(hub))
	rt.Log.Info("live updates via built-in server-sent events (single replica only)")
	return hub, nil
}
