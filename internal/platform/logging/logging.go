package logging

import (
	"log/slog"
	"os"

	"malus-be/internal/platform/config"
)

func New(cfg config.Config) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
	return slog.New(handler).With("service", cfg.Service, "env", cfg.Env)
}
