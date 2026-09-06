package auth

import (
	"net/http"
	"strings"
	"time"

	platformemail "github.com/fortune-tech/time-tracker/apps/api/internal/platform/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResendVerificationHandler struct {
	Database   *pgxpool.Pool
	Email      platformemail.Sender
	WebBaseURL string
}
type resendVerificationRequest struct {
	Email string `json:"email"`
}

func (handler ResendVerificationHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var input resendVerificationRequest
	if !decodeBody(writer, request, &input) {
		return
	}
	var userID uuid.UUID
	var verifiedAt *time.Time
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if err := handler.Database.QueryRow(request.Context(), `SELECT id, email_verified_at FROM users WHERE email = $1`, email).Scan(&userID, &verifiedAt); err == nil && verifiedAt == nil {
		token, tokenErr := randomToken()
		tokenID, idErr := uuid.NewV7()
		if tokenErr == nil && idErr == nil {
			_, _ = handler.Database.Exec(request.Context(), `UPDATE email_action_tokens SET used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND action_type = 'EMAIL_VERIFICATION' AND used_at IS NULL`, userID)
			if _, err := handler.Database.Exec(request.Context(), `INSERT INTO email_action_tokens (id, user_id, token_hash, action_type, expires_at) VALUES ($1, $2, $3, 'EMAIL_VERIFICATION', $4)`, tokenID, userID, tokenHash(token), time.Now().UTC().Add(24*time.Hour)); err == nil {
				verificationURL := strings.TrimRight(handler.WebBaseURL, "/") + "/verify-email?token=" + token
				_ = handler.Email.Send(email, "Verify your TimeTracker email", "Open this link to verify your email address:\n\n"+verificationURL+"\n\nThis link expires in 24 hours.")
			}
		}
	}
	writeJSON(writer, http.StatusOK, success(map[string]bool{"accepted": true}))
}
