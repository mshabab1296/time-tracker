package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	platformemail "github.com/fortune-tech/time-tracker/apps/api/internal/platform/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

type RegisterHandler struct {
	Database   *pgxpool.Pool
	Email      platformemail.Sender
	WebBaseURL string
}
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

func (handler RegisterHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var input registerRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is invalid.")
		return
	}
	email, name, timezone := strings.ToLower(strings.TrimSpace(input.Email)), strings.TrimSpace(input.Name), strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := mail.ParseAddress(email); err != nil || len(email) > 320 || name == "" || len(name) > 100 || len(input.Password) < 12 {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a valid email, name, and password of at least 12 characters.")
		return
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a valid IANA timezone.")
		return
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	token, err := randomToken()
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	userID, err := uuid.NewV7()
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	tokenID, err := uuid.NewV7()
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	defer tx.Rollback(request.Context())
	_, err = tx.Exec(request.Context(), `INSERT INTO users (id, email, password_hash, name, timezone) VALUES ($1, $2, $3, $4, $5)`, userID, email, passwordHash, name, timezone)
	if err != nil {
		var dbError *pgconn.PgError
		if errors.As(err, &dbError) && dbError.Code == "23505" {
			writeError(writer, 409, "EMAIL_ALREADY_REGISTERED", "An account already uses this email address.")
			return
		}
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	_, err = tx.Exec(request.Context(), `INSERT INTO email_action_tokens (id, user_id, token_hash, action_type, expires_at) VALUES ($1, $2, $3, 'EMAIL_VERIFICATION', $4)`, tokenID, userID, tokenHash(token), time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to register the account.")
		return
	}
	verificationURL := strings.TrimRight(handler.WebBaseURL, "/") + "/verify-email?token=" + token
	if err := handler.Email.Send(email, "Verify your TimeTracker email", "Open this link to verify your email address:\n\n"+verificationURL+"\n\nThis link expires in 24 hours."); err != nil {
		writeError(writer, http.StatusServiceUnavailable, "EMAIL_DELIVERY_FAILED", "Your account was created, but we could not send the verification email. Use the resend option later.")
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"data": map[string]any{"id": userID, "email": email, "emailVerified": false}, "error": nil, "meta": map[string]any{}})
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}
func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// NewActionToken returns an opaque token suitable for one-time email actions.
func NewActionToken() (string, error) { return randomToken() }

// HashActionToken returns the database-safe representation of an action token.
func HashActionToken(token string) string { return tokenHash(token) }
func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"data": nil, "error": map[string]string{"code": code, "message": message}, "meta": map[string]any{}})
}
func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
