package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

const (
	sessionCookieName = "timetracker_session"
	csrfCookieName    = "timetracker_csrf"
)

type LoginHandler struct {
	Database     *pgxpool.Pool
	CookieSecure bool
}
type LogoutHandler struct {
	Database     *pgxpool.Pool
	CookieSecure bool
}
type MeHandler struct{ Database *pgxpool.Pool }
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type authenticatedUser struct {
	ID       uuid.UUID
	Email    string
	Name     string
	Timezone string
	Verified bool
}

func (handler LoginHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var input loginRequest
	if !decodeBody(writer, request, &input) {
		return
	}
	var user authenticatedUser
	var passwordHash string
	var verifiedAt *time.Time
	err := handler.Database.QueryRow(request.Context(), `SELECT id, password_hash, email, name, timezone, email_verified_at FROM users WHERE email = $1`, strings.ToLower(strings.TrimSpace(input.Email))).Scan(&user.ID, &passwordHash, &user.Email, &user.Name, &user.Timezone, &verifiedAt)
	if err != nil || !verifyPassword(input.Password, passwordHash) {
		writeError(writer, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is incorrect.")
		return
	}
	if verifiedAt == nil {
		writeError(writer, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Verify your email address before signing in.")
		return
	}
	rawToken, err := randomToken()
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to sign in.")
		return
	}
	csrfToken, err := randomToken()
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to sign in.")
		return
	}
	sessionID, err := uuid.NewV7()
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to sign in.")
		return
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = handler.Database.Exec(request.Context(), `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, sessionID, user.ID, tokenHash(rawToken), tokenHash(csrfToken), expiresAt)
	if err != nil {
		writeError(writer, 500, "INTERNAL_ERROR", "Unable to sign in.")
		return
	}
	setSessionCookies(writer, rawToken, csrfToken, expiresAt, handler.CookieSecure)
	user.Verified = true
	writeJSON(writer, http.StatusOK, success(userResponse(user)))
}

func (handler LogoutHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if _, ok := requireSession(writer, request, handler.Database, true); !ok {
		return
	}
	if token, err := request.Cookie(sessionCookieName); err == nil {
		_, _ = handler.Database.Exec(request.Context(), `DELETE FROM sessions WHERE token_hash = $1`, tokenHash(token.Value))
	}
	clearSessionCookies(writer, handler.CookieSecure)
	writeJSON(writer, http.StatusOK, success(map[string]bool{"loggedOut": true}))
}

func (handler MeHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	user, ok := requireSession(writer, request, handler.Database, false)
	if !ok {
		return
	}
	writeJSON(writer, http.StatusOK, success(userResponse(user)))
}

func requireSession(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool, requireCSRF bool) (authenticatedUser, bool) {
	token, err := request.Cookie(sessionCookieName)
	if err != nil || token.Value == "" {
		writeError(writer, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.")
		return authenticatedUser{}, false
	}
	var user authenticatedUser
	var csrfHash string
	var verifiedAt *time.Time
	err = database.QueryRow(request.Context(), `SELECT u.id, u.email, u.name, u.timezone, u.email_verified_at, s.csrf_token_hash FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = $1 AND s.expires_at > CURRENT_TIMESTAMP`, tokenHash(token.Value)).Scan(&user.ID, &user.Email, &user.Name, &user.Timezone, &verifiedAt, &csrfHash)
	if err != nil {
		clearSessionCookies(writer, false)
		writeError(writer, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.")
		return authenticatedUser{}, false
	}
	user.Verified = verifiedAt != nil
	if requireCSRF {
		provided := request.Header.Get("X-CSRF-Token")
		if provided == "" || subtle.ConstantTimeCompare([]byte(tokenHash(provided)), []byte(csrfHash)) != 1 {
			writeError(writer, http.StatusForbidden, "CSRF_VALIDATION_FAILED", "A valid CSRF token is required.")
			return authenticatedUser{}, false
		}
	}
	_, _ = database.Exec(request.Context(), `UPDATE sessions SET last_seen_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE token_hash = $1 AND last_seen_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes'`, tokenHash(token.Value))
	return user, true
}

// AuthenticatedUserID verifies the database-backed session and CSRF token for a
// state-changing request. Domain packages use it instead of trusting client data.
func AuthenticatedUserID(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool) (uuid.UUID, bool) {
	user, ok := requireSession(writer, request, database, true)
	return user.ID, ok
}

// AuthenticatedUserIDForRead verifies a session for a read-only endpoint.
func AuthenticatedUserIDForRead(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool) (uuid.UUID, bool) {
	user, ok := requireSession(writer, request, database, false)
	return user.ID, ok
}

func userResponse(user authenticatedUser) map[string]any {
	return map[string]any{"id": user.ID, "email": user.Email, "name": user.Name, "timezone": user.Timezone, "emailVerified": user.Verified}
}
func success(data any) map[string]any {
	return map[string]any{"data": data, "error": nil, "meta": map[string]any{}}
}

func decodeBody(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is invalid.")
		return false
	}
	return true
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || memory == 0 || iterations == 0 || parallelism == 0 {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[4])
	expected, hashErr := base64.RawStdEncoding.DecodeString(parts[5])
	if saltErr != nil || hashErr != nil || len(salt) == 0 || len(expected) == 0 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func setSessionCookies(writer http.ResponseWriter, token, csrfToken string, expiry time.Time, secure bool) {
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expiry})
	http.SetCookie(writer, &http.Cookie{Name: csrfCookieName, Value: csrfToken, Path: "/", Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expiry})
}
func clearSessionCookies(writer http.ResponseWriter, secure bool) {
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		http.SetCookie(writer, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: name == sessionCookieName, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}
}
