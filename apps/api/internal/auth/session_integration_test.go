package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This test exercises the real PostgreSQL expiry predicate. It is intentionally
// opt-in so the normal unit suite does not need a database service.
func TestExpiredSessionIsRejected(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	userID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	passwordHash, err := hashPassword("a long integration test password")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at) VALUES ($1, $2, $3, 'Session test', 'UTC', CURRENT_TIMESTAMP)`, userID, "session-test-"+userID.String()+"@example.com", passwordHash)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	}()

	rawToken := "expired-session-token"
	_, err = pool.Exec(context.Background(), `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, created_at, last_seen_at, expires_at) VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '1 hour')`, sessionID, userID, tokenHash(rawToken), tokenHash("csrf-token"))
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: rawToken})
	MeHandler{Database: pool}.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}
