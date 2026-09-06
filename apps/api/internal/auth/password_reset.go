package auth

import (
	"net/http"
	"strings"
	"time"

	platformemail "github.com/fortune-tech/time-tracker/apps/api/internal/platform/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PasswordResetRequestHandler struct {
	Database   *pgxpool.Pool
	Email      platformemail.Sender
	WebBaseURL string
}
type PasswordResetConfirmHandler struct{ Database *pgxpool.Pool }
type passwordResetRequest struct {
	Email string `json:"email"`
}
type passwordResetConfirm struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (handler PasswordResetRequestHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var input passwordResetRequest
	if !decodeBody(writer, request, &input) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	var userID uuid.UUID
	if err := handler.Database.QueryRow(request.Context(), `SELECT id FROM users WHERE email = $1`, email).Scan(&userID); err == nil {
		token, tokenErr := randomToken()
		tokenID, idErr := uuid.NewV7()
		if tokenErr == nil && idErr == nil {
			_, _ = handler.Database.Exec(request.Context(), `UPDATE email_action_tokens SET used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND action_type = 'PASSWORD_RESET' AND used_at IS NULL`, userID)
			if _, err := handler.Database.Exec(request.Context(), `INSERT INTO email_action_tokens (id, user_id, token_hash, action_type, expires_at) VALUES ($1, $2, $3, 'PASSWORD_RESET', $4)`, tokenID, userID, tokenHash(token), time.Now().UTC().Add(time.Hour)); err == nil {
				resetURL := strings.TrimRight(handler.WebBaseURL, "/") + "/reset-password?token=" + token
				_ = handler.Email.Send(email, "Reset your TimeTracker password", "Open this link to choose a new password:\n\n"+resetURL+"\n\nThis link expires in one hour.")
			}
		}
	}
	// This response stays identical whether an account exists, preventing email enumeration.
	writeJSON(writer, http.StatusOK, success(map[string]bool{"accepted": true}))
}

func (handler PasswordResetConfirmHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var input passwordResetConfirm
	if !decodeBody(writer, request, &input) {
		return
	}
	if input.Token == "" || len(input.Password) < 12 {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a valid reset token and a password of at least 12 characters.")
		return
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to reset password.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to reset password.")
		return
	}
	defer tx.Rollback(request.Context())
	var userID uuid.UUID
	err = tx.QueryRow(request.Context(), `SELECT user_id FROM email_action_tokens WHERE token_hash = $1 AND action_type = 'PASSWORD_RESET' AND used_at IS NULL AND expires_at > CURRENT_TIMESTAMP FOR UPDATE`, tokenHash(input.Token)).Scan(&userID)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_OR_EXPIRED_TOKEN", "The reset token is invalid or expired.")
		return
	}
	if _, err = tx.Exec(request.Context(), `UPDATE users SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, passwordHash, userID); err == nil {
		_, err = tx.Exec(request.Context(), `DELETE FROM sessions WHERE user_id = $1`, userID)
	}
	if err == nil {
		_, err = tx.Exec(request.Context(), `UPDATE email_action_tokens SET used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE token_hash = $1`, tokenHash(input.Token))
	}
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to reset password.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to reset password.")
		return
	}
	writeJSON(writer, http.StatusOK, success(map[string]bool{"passwordReset": true}))
}
