package main

import (
	"context"
	"database/sql"
	"log/slog"

	"malus-be/internal/content/application"
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

func serve(_ context.Context, rt service.Runtime) ([]httpx.Check, error) {
	db, err := openDB(rt.Config)
	if err != nil {
		return nil, err
	}

	bus := messaging.NewMemoryBus("/malus/content")
	bus.Subscribe(messaging.LogHandler(rt.Log))

	svc := application.NewService(sqlserver.NewSectionRepository(db), bus, kernel.SystemClock)
	contenthttp.NewHandler(svc).Register(rt.Mux)
	return []httpx.Check{db.PingContext}, nil
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
