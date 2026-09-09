package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/fortune-tech/time-tracker/apps/api/internal/organizations"
	platformemail "github.com/fortune-tech/time-tracker/apps/api/internal/platform/email"
	"github.com/fortune-tech/time-tracker/apps/api/internal/timetracking"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(database *pgxpool.Pool, logger *slog.Logger, sender platformemail.Sender, webBaseURL string, cookieSecure bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", live)
	mux.HandleFunc("GET /health/ready", ready(database))
	mux.HandleFunc("GET /api/v1", apiRoot)
	mux.Handle("POST /api/v1/auth/register", auth.RegisterHandler{Database: database, Email: sender, WebBaseURL: webBaseURL})
	mux.Handle("POST /api/v1/auth/verify-email", auth.VerifyEmailHandler{Database: database})
	mux.Handle("POST /api/v1/auth/login", auth.LoginHandler{Database: database, CookieSecure: cookieSecure})
	mux.Handle("POST /api/v1/auth/logout", auth.LogoutHandler{Database: database, CookieSecure: cookieSecure})
	mux.Handle("GET /api/v1/auth/me", auth.MeHandler{Database: database})
	mux.Handle("PATCH /api/v1/profile", auth.ProfileHandler{Database: database})
	mux.Handle("POST /api/v1/auth/password-reset/request", auth.PasswordResetRequestHandler{Database: database, Email: sender, WebBaseURL: webBaseURL})
	mux.Handle("POST /api/v1/auth/password-reset/confirm", auth.PasswordResetConfirmHandler{Database: database})
	mux.Handle("POST /api/v1/auth/resend-verification", auth.ResendVerificationHandler{Database: database, Email: sender, WebBaseURL: webBaseURL})
	mux.Handle("POST /api/v1/organizations", organizations.CreateHandler{Database: database})
	mux.Handle("GET /api/v1/organizations", organizations.ListHandler{Database: database})
	mux.Handle("PATCH /api/v1/organizations/{organizationID}", organizations.UpdateHandler{Database: database})
	mux.Handle("GET /api/v1/organizations/{organizationID}/members", organizations.MembersHandler{Database: database})
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/members/{userID}", organizations.RemoveMemberHandler{Database: database})
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects", organizations.ProjectsHandler{Database: database})
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects", organizations.ProjectsHandler{Database: database})
	mux.Handle("PATCH /api/v1/organizations/{organizationID}/projects/{projectID}", organizations.ProjectHandler{Database: database})
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/projects/{projectID}", organizations.ProjectHandler{Database: database})
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects/{projectID}/assignments/{userID}", organizations.AssignmentHandler{Database: database})
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/projects/{projectID}/assignments/{userID}", organizations.AssignmentHandler{Database: database})
	mux.Handle("GET /api/v1/organizations/{organizationID}/tags", organizations.TagsHandler{Database: database})
	mux.Handle("POST /api/v1/organizations/{organizationID}/tags", organizations.TagsHandler{Database: database})
	mux.Handle("PATCH /api/v1/organizations/{organizationID}/tags/{tagID}", organizations.TagHandler{Database: database})
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/tags/{tagID}", organizations.TagHandler{Database: database})
	mux.Handle("POST /api/v1/organizations/{organizationID}/invitations", organizations.InvitationCreateHandler{Database: database, Email: sender, WebBaseURL: webBaseURL})
	mux.Handle("GET /api/v1/organizations/{organizationID}/invitations", organizations.OrganizationInvitationListHandler{Database: database})
	mux.Handle("POST /api/v1/organizations/{organizationID}/invitations/{invitationID}/resend", organizations.InvitationResendHandler{Database: database, Email: sender, WebBaseURL: webBaseURL})
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/invitations/{invitationID}", organizations.InvitationCancelHandler{Database: database})
	mux.Handle("GET /api/v1/invitations", organizations.InvitationListHandler{Database: database})
	mux.Handle("POST /api/v1/invitations/{invitationID}/{decision}", organizations.InvitationDecisionHandler{Database: database})
	mux.HandleFunc("POST /api/v1/timer/start", timetracking.Handler{Database: database}.Start)
	mux.HandleFunc("POST /api/v1/timer/pause", timetracking.Handler{Database: database}.Pause)
	mux.HandleFunc("POST /api/v1/timer/resume", timetracking.Handler{Database: database}.Resume)
	mux.HandleFunc("POST /api/v1/timer/stop", timetracking.Handler{Database: database}.Stop)
	mux.HandleFunc("PATCH /api/v1/timer/active", timetracking.Handler{Database: database}.UpdateActive)
	mux.HandleFunc("GET /api/v1/timer/active", timetracking.Handler{Database: database}.Active)
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
