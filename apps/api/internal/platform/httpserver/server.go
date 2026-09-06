package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(database *pgxpool.Pool, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", live)
	mux.HandleFunc("GET /health/ready", ready(database))
	mux.HandleFunc("GET /api/v1", apiRoot)
	return withRequestLogging(logger, mux)
}

func live(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"data": map[string]string{"status": "ok"}, "error": nil, "meta": map[string]any{}})
}

func ready(database *pgxpool.Pool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ping(ctx); err != nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]any{"data": nil, "error": map[string]string{"code": "SERVICE_UNAVAILABLE", "message": "Service is not ready."}, "meta": map[string]any{}})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"data": map[string]string{"status": "ok"}, "error": nil, "meta": map[string]any{}})
	}
}

func apiRoot(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"data": map[string]string{"service": "timetracker-api"}, "error": nil, "meta": map[string]any{}})
}

func withRequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		writer.Header().Set("X-Request-ID", requestID)
		startedAt := time.Now()
		next.ServeHTTP(writer, request)
		logger.Info("request completed", "requestId", requestID, "method", request.Method, "path", request.URL.Path, "durationMs", time.Since(startedAt).Milliseconds())
	})
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
