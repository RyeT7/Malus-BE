package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"malus-be/internal/platform/config"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/logging"
)

type Runtime struct {
	Config config.Config
	Log    *slog.Logger
	Mux    *http.ServeMux
}

type Setup func(ctx context.Context, rt Runtime) ([]httpx.Check, error)

type Command func(ctx context.Context, cfg config.Config, log *slog.Logger) error

type Option func(*options)

type options struct {
	commands map[string]Command
}

func WithCommand(name string, cmd Command) Option {
	return func(o *options) { o.commands[name] = cmd }
}

func Run(name string, setup Setup, opts ...Option) {
	o := options{commands: make(map[string]Command)}
	for _, opt := range opts {
		opt(&o)
	}

	if err := config.LoadDotEnv(".env"); err != nil {
		fmt.Fprintln(os.Stderr, "load .env:", err)
		os.Exit(1)
	}
	cfg := config.Load(name)
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(probe(cfg.Addr))
	}

	log := logging.New(cfg)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 {
		cmd, ok := o.commands[os.Args[1]]
		if !ok {
			log.Error("unknown command", "command", os.Args[1])
			os.Exit(2)
		}
		if err := cmd(ctx, cfg, log); err != nil {
			log.Error("command failed", "command", os.Args[1], "error", err)
			os.Exit(1)
		}
		return
	}

	mux := http.NewServeMux()
	checks, err := setup(ctx, Runtime{Config: cfg, Log: log, Mux: mux})
	if err != nil {
		log.Error("setup failed", "error", err)
		os.Exit(1)
	}
	httpx.RegisterHealth(mux, checks...)

	handler := httpx.Chain(mux, httpx.RequestID, httpx.Recover(log), httpx.AccessLog(log))
	if err := httpx.Serve(ctx, cfg.Addr, handler, log); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func probe(addr string) int {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1" + addr + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
