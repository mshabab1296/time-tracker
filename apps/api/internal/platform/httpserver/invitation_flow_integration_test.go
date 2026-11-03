package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type invitationFlowSender struct{ calls int }

func (sender *invitationFlowSender) Send(_, _, _ string) error {
	sender.calls++
	return nil
}

func TestInvitationSignupVerifiesEmailAndLinkJoinsOrganization(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	adminID, organizationID, invitationID := uuid.New(), uuid.New(), uuid.New()
	email := "invitation-flow-" + invitationID.String() + "@example.com"
	token := "invitation-flow-token-" + invitationID.String()
	var inviteeID uuid.UUID
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM memberships WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM invitations WHERE id = $1`, invitationID)
		if inviteeID != uuid.Nil {
			_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE user_id = $1`, inviteeID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM email_action_tokens WHERE user_id = $1`, inviteeID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, inviteeID)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, adminID)
	}()
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at) VALUES ($1, $2, 'not-used', 'Admin', 'UTC', CURRENT_TIMESTAMP)`, adminID, "invitation-admin-"+adminID.String()+"@example.com")
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO organizations (id, name, timezone) VALUES ($1, 'Invitation test', 'UTC')`, organizationID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO invitations (id, organization_id, email, token_hash, invited_by_user_id, status, expires_at) VALUES ($1, $2, $3, $4, $5, 'PENDING', $6)`, invitationID, organizationID, email, auth.HashActionToken(token), adminID, time.Now().Add(time.Hour))
	}
	if err != nil {
		t.Fatalf("prepare invitation: %v", err)
	}

	sender := &invitationFlowSender{}
	server := New(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), sender, "http://localhost:5174", false)
	registerBody, _ := json.Marshal(map[string]string{"email": email, "name": "Invitee", "password": "Abcdefg1", "timezone": "UTC", "invitationToken": token})
	register := httptest.NewRecorder()
	server.ServeHTTP(register, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(registerBody)))
	if register.Code != http.StatusCreated || sender.calls != 0 {
		t.Fatalf("register status=%d emails=%d body=%s", register.Code, sender.calls, register.Body.String())
	}
	var verified bool
	if err := pool.QueryRow(context.Background(), `SELECT id, email_verified_at IS NOT NULL FROM users WHERE email = $1`, email).Scan(&inviteeID, &verified); err != nil || !verified {
		t.Fatalf("invited account verified=%v err=%v", verified, err)
	}
	var verificationEmails int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM email_action_tokens WHERE user_id = $1`, inviteeID).Scan(&verificationEmails); err != nil || verificationEmails != 0 {
		t.Fatalf("verification tokens=%d err=%v", verificationEmails, err)
	}

	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": "Abcdefg1"})
	login := httptest.NewRecorder()
	server.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody)))
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range login.Result().Cookies() {
		switch cookie.Name {
		case "timetracker_session":
			sessionCookie = cookie
		case "timetracker_csrf":
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("login did not set session and CSRF cookies")
	}
	for attempt := 0; attempt < 2; attempt++ {
		accept := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/invitations/accept-token", bytes.NewBufferString(fmt.Sprintf(`{"token":%q}`, token)))
		request.AddCookie(sessionCookie)
		request.Header.Set("X-CSRF-Token", csrfCookie.Value)
		server.ServeHTTP(accept, request)
		if accept.Code != http.StatusOK {
			t.Fatalf("accept attempt %d status=%d body=%s", attempt, accept.Code, accept.Body.String())
		}
	}
	var role, status string
	if err := pool.QueryRow(context.Background(), `SELECT role FROM memberships WHERE organization_id = $1 AND user_id = $2`, organizationID, inviteeID).Scan(&role); err != nil || role != "MEMBER" {
		t.Fatalf("membership role=%q err=%v", role, err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT status FROM invitations WHERE id = $1`, invitationID).Scan(&status); err != nil || status != "ACCEPTED" {
		t.Fatalf("invitation status=%q err=%v", status, err)
	}
}
