package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"malus-be/internal/content/application"
	"malus-be/internal/content/infrastructure/blobstore"
	"malus-be/internal/content/infrastructure/sqlserver"
	contenthttp "malus-be/internal/content/interfaces/http"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/config"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/messaging"
	"malus-be/internal/platform/service"
	"malus-be/internal/platform/sqldb"
)

func main() {
	service.Run("content", serve, service.WithCommand("migrate", migrate))
}

func serve(ctx context.Context, rt service.Runtime) ([]httpx.Check, error) {
	db, err := openDB(rt.Config)
	if err != nil {
		return nil, err
	}
	blobs, err := openBlobs(ctx, rt)
	if err != nil {
		return nil, err
	}

	bus := messaging.NewMemoryBus("/malus/content")
	bus.Subscribe(messaging.LogHandler(rt.Log))

	svc := application.NewService(sqlserver.NewSectionRepository(db), sqlserver.NewAttachmentRepository(db), blobs, bus, kernel.SystemClock)
	contenthttp.NewHandler(svc, config.String("AUTH_ADMIN_ROLE", "admin")).Register(rt.Mux)
	return []httpx.Check{db.PingContext, blobs.Ping}, nil
}

func openBlobs(ctx context.Context, rt service.Runtime) (*blobstore.Store, error) {
	store, err := blobstore.New(blobstore.Config{
		ConnectionString: os.Getenv("BLOB_CONNECTION_STRING"),
		AccountURL:       os.Getenv("BLOB_ACCOUNT_URL"),
		Container:        config.String("BLOB_CONTAINER", "attachments"),
		PublicEndpoint:   os.Getenv("BLOB_PUBLIC_ENDPOINT"),
	}, rt.Config.IsLocal())
	if err != nil || !rt.Config.IsLocal() {
		return store, err
	}

	origins := strings.Split(config.String("BLOB_CORS_ORIGINS", "http://localhost:5173"), ",")
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		err := store.EnsureContainer(ctx)
		if err == nil {
			err = store.AllowBrowserUploads(ctx, origins)
		}
		if err == nil {
			rt.Log.Info("blob storage ready", "cors_origins", origins)
			return store, nil
		}
		rt.Log.Warn("waiting for blob storage", "error", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("bootstrap blob storage: %w", err)
		case <-time.After(3 * time.Second):
		}
	}
}

func migrate(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	db, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	return sqldb.Migrate(ctx, db, sqlserver.Migrations(), log)
}

func openDB(cfg config.Config) (*sql.DB, error) {
	dsn, err := config.Required("SQL_DSN")
	if err != nil {
		return nil, err
	}
	return sqldb.Open(dsn, cfg.IsLocal())
}
