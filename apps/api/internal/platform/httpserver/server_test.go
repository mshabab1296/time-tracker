package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingCorrelatesRequestsWithoutSecrets(t *testing.T) {
	for _, requestID := range []string{"client-request-1", "", strings.Repeat("a", 129), "bad\nidentifier"} {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		handler := withRequestLogging(logger, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, 403, map[string]any{"data": nil, "error": map[string]string{"code": "FORBIDDEN"}, "meta": map[string]any{}})
		}))
		request := httptest.NewRequest("GET", "/secret?token=DO-NOT-LOG", nil)
		request.Header.Set("X-Request-ID", requestID)
		request.AddCookie(&http.Cookie{Name: "timetracker_session", Value: "DO-NOT-LOG"})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		id := recorder.Header().Get("X-Request-ID")
		if !validRequestID(id) || !strings.Contains(logs.String(), id) || !strings.Contains(logs.String(), `"status":403`) || strings.Contains(logs.String(), "DO-NOT-LOG") {
			t.Fatalf("unsafe or incomplete log: %s", logs.String())
		}
		if requestID == "client-request-1" && id != requestID {
			t.Fatal("valid correlation ID was not preserved")
		}
	}
}

func TestPanicReturnsSafeCorrelatedError(t *testing.T) {
	var logs bytes.Buffer
	handler := withRequestLogging(slog.New(slog.NewJSONHandler(&logs, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("database password DO-NOT-LOG") }))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/panic", nil))
	if recorder.Code != 500 || !strings.Contains(recorder.Body.String(), "INTERNAL_ERROR") || strings.Contains(recorder.Body.String()+logs.String(), "DO-NOT-LOG") {
		t.Fatalf("unsafe panic response: %s %s", recorder.Body.String(), logs.String())
	}
}

func TestMetricsTrackStatusAndLatency(t *testing.T) {
	metrics := newRequestMetrics()
	handler := metrics.observe(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(403) }))
	for range 2 {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/example", nil))
	}
	recorder := httptest.NewRecorder()
	metrics.serveHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	for _, line := range []string{`timetracker_http_requests_total{status_class="4xx"} 2`, `timetracker_http_request_duration_seconds_bucket{le="+Inf"} 2`, `timetracker_http_request_duration_seconds_count 2`} {
		if !strings.Contains(recorder.Body.String(), line) {
			t.Fatalf("missing metric: %s", line)
		}
	}
}

func TestHealthAndAPIRootContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	handler := New(nil, logger, nil, "http://localhost", false)
	for _, path := range []string{"/health/live", "/api/v1"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
		if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"error":null`) || recorder.Header().Get("X-Request-ID") == "" {
			t.Fatalf("contract: %s", recorder.Body.String())
		}
	}
}
