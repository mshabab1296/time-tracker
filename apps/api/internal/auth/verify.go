package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerifyEmailHandler struct{ Database *pgxpool.Pool }
type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (handler VerifyEmailHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var input verifyEmailRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Token == "" {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A verification token is required.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to verify the email address.")
		return
	}
	defer tx.Rollback(request.Context())
	var userID string
	err = tx.QueryRow(request.Context(), `SELECT user_id FROM email_action_tokens WHERE token_hash = $1 AND action_type = 'EMAIL_VERIFICATION' AND used_at IS NULL AND expires_at > $2 FOR UPDATE`, tokenHash(input.Token), time.Now().UTC()).Scan(&userID)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_OR_EXPIRED_TOKEN", "The verification token is invalid or expired.")
		return
	}
	if _, err = tx.Exec(request.Context(), `UPDATE email_action_tokens SET used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE token_hash = $1`, tokenHash(input.Token)); err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to verify the email address.")
		return
	}
	if _, err = tx.Exec(request.Context(), `UPDATE users SET email_verified_at = COALESCE(email_verified_at, CURRENT_TIMESTAMP), updated_at = CURRENT_TIMESTAMP WHERE id = $1`, userID); err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to verify the email address.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to verify the email address.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"data": map[string]bool{"emailVerified": true}, "error": nil, "meta": map[string]any{}})
}
