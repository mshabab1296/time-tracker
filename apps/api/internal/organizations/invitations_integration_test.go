package organizations

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type capturedEmailSender struct{ calls int }

func (sender *capturedEmailSender) Send(_, _, _ string) error { sender.calls++; return nil }

func TestInvitationCanBeCreatedResentAndCancelled(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	organizationID, adminID, membershipID, sessionID := testUUID(t), testUUID(t), testUUID(t), testUUID(t)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM invitations WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE user_id = $1`, adminID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM memberships WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, adminID)
	}
	defer cleanup()
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at) VALUES ($1, $2, 'not-used', 'Admin', 'UTC', CURRENT_TIMESTAMP)`, adminID, "invitation-admin-"+adminID.String()+"@example.com")
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO organizations (id, name, timezone) VALUES ($1, 'Invitation test', 'UTC')`, organizationID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'ADMIN')`, membershipID, organizationID, adminID)
	}
	if err != nil {
		t.Fatalf("create fixtures: %v", err)
	}
	rawSession, csrfToken := "invitation-admin-session", "invitation-admin-csrf"
	_, err = pool.Exec(context.Background(), `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, sessionID, adminID, auth.HashActionToken(rawSession), auth.HashActionToken(csrfToken), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	sender := &capturedEmailSender{}
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/placeholder/invitations", bytes.NewBufferString(`{"email":"invitee@example.com"}`))
	createRequest.SetPathValue("organizationID", organizationID.String())
	createRequest.Header.Set("X-CSRF-Token", csrfToken)
	createRequest.AddCookie(&http.Cookie{Name: "timetracker_session", Value: rawSession})
	createRecorder := httptest.NewRecorder()
	InvitationCreateHandler{Database: pool, Email: sender, WebBaseURL: "http://localhost:5173"}.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated || sender.calls != 1 {
		t.Fatalf("create invitation status=%d emails=%d body=%s", createRecorder.Code, sender.calls, createRecorder.Body.String())
	}
	var invitationID uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM invitations WHERE organization_id = $1 AND email = 'invitee@example.com'`, organizationID).Scan(&invitationID); err != nil {
		t.Fatalf("load invitation: %v", err)
	}

	resendRequest := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/placeholder/invitations/placeholder/resend", nil)
	resendRequest.SetPathValue("organizationID", organizationID.String())
	resendRequest.SetPathValue("invitationID", invitationID.String())
	resendRequest.Header.Set("X-CSRF-Token", csrfToken)
	resendRequest.AddCookie(&http.Cookie{Name: "timetracker_session", Value: rawSession})
	resendRecorder := httptest.NewRecorder()
	InvitationResendHandler{Database: pool, Email: sender, WebBaseURL: "http://localhost:5173"}.ServeHTTP(resendRecorder, resendRequest)
	if resendRecorder.Code != http.StatusOK || sender.calls != 2 {
		t.Fatalf("resend invitation status=%d emails=%d body=%s", resendRecorder.Code, sender.calls, resendRecorder.Body.String())
	}

	cancelRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/placeholder/invitations/placeholder", nil)
	cancelRequest.SetPathValue("organizationID", organizationID.String())
	cancelRequest.SetPathValue("invitationID", invitationID.String())
	cancelRequest.Header.Set("X-CSRF-Token", csrfToken)
	cancelRequest.AddCookie(&http.Cookie{Name: "timetracker_session", Value: rawSession})
	cancelRecorder := httptest.NewRecorder()
	InvitationCancelHandler{Database: pool}.ServeHTTP(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel invitation status=%d body=%s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM invitations WHERE id = $1`, invitationID).Scan(&status); err != nil || status != "CANCELLED" {
		t.Fatalf("invitation status=%q err=%v", status, err)
	}
}
