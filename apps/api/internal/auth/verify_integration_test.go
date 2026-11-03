package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestVerificationLinkCanBeOpenedTwice(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	userID, tokenID := uuid.New(), uuid.New()
	token := "verification-repeat-" + tokenID.String()
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM email_action_tokens WHERE id = $1`, tokenID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	}()
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone) VALUES ($1, $2, 'not-used', 'Test', 'UTC')`, userID, "verification-repeat-"+userID.String()+"@example.com")
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO email_action_tokens (id, user_id, token_hash, action_type, expires_at) VALUES ($1, $2, $3, 'EMAIL_VERIFICATION', $4)`, tokenID, userID, tokenHash(token), time.Now().Add(time.Hour))
	}
	if err != nil {
		t.Fatalf("prepare verification: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`{"token":"`+token+`"}`))
		recorder := httptest.NewRecorder()
		VerifyEmailHandler{Database: pool}.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("verification attempt %d status=%d body=%s", attempt, recorder.Code, recorder.Body.String())
		}
	}
}
