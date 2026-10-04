package gateway

import (
	"fmt"
	"log/slog"
	"net/http"

	"malus-be/internal/platform/httpx"
)

func New(cfg Config, authn Authenticator, log *slog.Logger) (http.Handler, error) {
	proxies := make(map[string]http.Handler, len(cfg.Upstreams))
	for name, target := range cfg.Upstreams {
		proxies[name] = newProxy(name, target, log)
	}

	mux := http.NewServeMux()
	for _, route := range Routes {
		proxy, ok := proxies[route.Upstream]
		if !ok {
			return nil, fmt.Errorf("route %q references unknown upstream %q", route.Pattern, route.Upstream)
		}
		mux.Handle(route.Pattern, authorize(route.Access, authn, cfg.Auth.AdminRole, proxy))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.Problem(w, r, http.StatusNotFound, "Not Found", "")
	})

	return httpx.Chain(mux,
		cors(cfg.AllowedOrigins),
		rateLimit(
			NewLimiter(cfg.RateLimit.RPS, cfg.RateLimit.Burst),
			NewLimiter(cfg.RateLimit.WriteRPS, cfg.RateLimit.WriteBurst),
			clientIPResolver(cfg.TrustProxyHeaders),
		),
	), nil
}

func NewAuthenticator(cfg AuthConfig, log *slog.Logger) Authenticator {
	if cfg.Disabled {
		log.Warn("authentication disabled: every request is treated as admin", "subject", "local-dev")
		return NewDevAuthenticator(cfg.AdminRole)
	}
	return NewJWTAuthenticator(NewVerifier(NewJWKS(cfg.JWKSURL), cfg.Issuer, cfg.Audience))
}
