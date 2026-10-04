package main

import (
	"context"

	"malus-be/internal/gateway"
	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/service"
)

func main() {
	service.Run("gateway", func(_ context.Context, rt service.Runtime) ([]httpx.Check, error) {
		cfg, err := gateway.LoadConfig(rt.Config.Env)
		if err != nil {
			return nil, err
		}
		handler, err := gateway.New(cfg, gateway.NewAuthenticator(cfg.Auth, rt.Log), rt.Log)
		if err != nil {
			return nil, err
		}
		rt.Mux.Handle("/", handler)
		return nil, nil
	})
}
