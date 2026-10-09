package gateway

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"malus-be/internal/platform/httpx"
	"malus-be/internal/platform/telemetry"
)

const maxBodyBytes = 1 << 20

func newProxy(name string, target *url.URL, log *slog.Logger) http.Handler {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	proxy := &httputil.ReverseProxy{
		Transport: telemetry.Transport(transport),
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			pr.Out.Host = target.Host
			pr.Out.Header.Del(httpx.UserIDHeader)
			pr.Out.Header.Del(httpx.UserRolesHeader)
			pr.Out.Header.Set(httpx.RequestIDHeader, httpx.RequestIDFrom(pr.In.Context()))
			if id, ok := identityFrom(pr.In.Context()); ok {
				pr.Out.Header.Set(httpx.UserIDHeader, id.Subject)
				pr.Out.Header.Set(httpx.UserRolesHeader, strings.Join(id.Roles, ","))
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del(httpx.RequestIDHeader)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.ErrorContext(r.Context(), "upstream request failed",
				"upstream", name, "error", err, "request_id", httpx.RequestIDFrom(r.Context()))
			httpx.Problem(w, r, http.StatusBadGateway, "Bad Gateway", "")
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		}
		proxy.ServeHTTP(w, r)
	})
}
