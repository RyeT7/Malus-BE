package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const EnvLocal = "local"

type Config struct {
	Service  string
	Env      string
	Addr     string
	LogLevel slog.Level
}

func (c Config) IsLocal() bool { return c.Env == EnvLocal }

func Load(service string) Config {
	return Config{
		Service:  service,
		Env:      String("APP_ENV", EnvLocal),
		Addr:     ":" + String("PORT", "8080"),
		LogLevel: parseLevel(String("LOG_LEVEL", "info")),
	}
}

func String(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func Required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func parseLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return slog.LevelInfo
	}
	return level
}
