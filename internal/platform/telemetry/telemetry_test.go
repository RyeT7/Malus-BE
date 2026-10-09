package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func record(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	return recorder
}

func TestStartWithoutEndpointIsANoOp(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := Start(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerTracesRequestsButNotHealthChecks(t *testing.T) {
	recorder := record(t)
	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), "content")

	for _, path := range []string{"/healthz", "/readyz", "/v1/presentation"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "GET /v1/presentation" {
		names := make([]string, len(spans))
		for i, s := range spans {
			names[i] = s.Name()
		}
		t.Fatalf("want one span named GET /v1/presentation, got %v", names)
	}
}

func TestTransportCarriesTheTraceToTheNextService(t *testing.T) {
	recorder := record(t)
	var upstreamTrace string
	upstream := httptest.NewServer(Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamTrace = r.Header.Get("traceparent")
	}), "upstream"))
	defer upstream.Close()

	gateway := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream.URL+"/v1/questions", nil)
		resp, err := (&http.Client{Transport: Transport(http.DefaultTransport)}).Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
	}), "gateway")
	gateway.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/questions", nil))

	if upstreamTrace == "" {
		t.Fatal("the upstream request carried no traceparent header")
	}
	spans := recorder.Ended()
	traceID := spans[0].SpanContext().TraceID()
	for _, s := range spans {
		if s.SpanContext().TraceID() != traceID {
			t.Fatalf("spans belong to different traces: %v and %v", traceID, s.SpanContext().TraceID())
		}
	}
	if len(spans) != 3 {
		t.Fatalf("want gateway server, gateway client and upstream server spans, got %d", len(spans))
	}
}
