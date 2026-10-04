package gateway

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Upstreams         map[string]*url.URL
	AllowedOrigins    []string
	TrustProxyHeaders bool
	Auth              AuthConfig
	RateLimit         RateLimitConfig
}

type AuthConfig struct {
	Disabled  bool
	Issuer    string
	Audience  string
	JWKSURL   string
	AdminRole string
}

type RateLimitConfig struct {
	RPS        float64
	Burst      int
	WriteRPS   float64
	WriteBurst int
}

func LoadConfig(env string) (Config, error) {
	var errs []error

	upstreams := make(map[string]*url.URL)
	for name, def := range map[string]string{
		"content":     "http://localhost:8081",
		"interaction": "http://localhost:8082",
		"realtime":    "http://localhost:8083",
	} {
		key := strings.ToUpper(name) + "_URL"
		u, err := url.Parse(getenv(key, def))
		if err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, fmt.Errorf("%s must be an absolute URL", key))
			continue
		}
		upstreams[name] = u
	}

	cfg := Config{
		Upstreams:         upstreams,
		AllowedOrigins:    splitList(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		TrustProxyHeaders: getbool("TRUST_PROXY_HEADERS", false, &errs),
		Auth: AuthConfig{
			Disabled:  getbool("AUTH_DISABLED", false, &errs),
			Issuer:    os.Getenv("AUTH_ISSUER"),
			Audience:  os.Getenv("AUTH_AUDIENCE"),
			JWKSURL:   os.Getenv("AUTH_JWKS_URL"),
			AdminRole: getenv("AUTH_ADMIN_ROLE", "admin"),
		},
		RateLimit: RateLimitConfig{
			RPS:        getfloat("RATE_LIMIT_RPS", 20, &errs),
			Burst:      getint("RATE_LIMIT_BURST", 40, &errs),
			WriteRPS:   getfloat("RATE_LIMIT_WRITE_RPS", 5, &errs),
			WriteBurst: getint("RATE_LIMIT_WRITE_BURST", 10, &errs),
		},
	}

	if cfg.Auth.Disabled && env != "local" {
		errs = append(errs, fmt.Errorf("AUTH_DISABLED is only allowed when APP_ENV=local, got %q", env))
	}
	if !cfg.Auth.Disabled && (cfg.Auth.Issuer == "" || cfg.Auth.Audience == "" || cfg.Auth.JWKSURL == "") {
		errs = append(errs, errors.New("AUTH_ISSUER, AUTH_AUDIENCE and AUTH_JWKS_URL are required unless AUTH_DISABLED=true"))
	}

	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getbool(key string, fallback bool, errs *[]error) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be a boolean", key))
	}
	return b
}

func getint(key string, fallback int, errs *[]error) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive integer", key))
	}
	return n
}

func getfloat(key string, fallback float64, errs *[]error) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive number", key))
	}
	return f
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
