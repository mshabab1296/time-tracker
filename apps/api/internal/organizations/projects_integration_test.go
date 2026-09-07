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

func TestMemberSeesOnlyAssignedProjectsAndCannotCreateTags(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	organizationID := testUUID(t)
	adminID := testUUID(t)
	memberID := testUUID(t)
	assignedProjectID := testUUID(t)
	hiddenProjectID := testUUID(t)
	assignmentID := testUUID(t)
	sessionID := testUUID(t)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_assignments WHERE project_id IN ($1, $2)`, assignedProjectID, hiddenProjectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE user_id IN ($1, $2)`, adminID, memberID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM memberships WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, adminID, memberID)
	}
	defer cleanup()
	passwordHash := "not-used-by-this-test"
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at) VALUES ($1, $2, $3, 'Admin', 'UTC', CURRENT_TIMESTAMP), ($4, $5, $3, 'Member', 'UTC', CURRENT_TIMESTAMP)`, adminID, "admin-"+adminID.String()+"@example.com", passwordHash, memberID, "member-"+memberID.String()+"@example.com")
	if err != nil {
		t.Fatalf("create users: %v", err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO organizations (id, name, timezone) VALUES ($1, 'Test organization', 'UTC')`, organizationID)
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'ADMIN'), ($4, $2, $5, 'MEMBER')`, testUUID(t), organizationID, adminID, testUUID(t), memberID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO projects (id, organization_id, name) VALUES ($1, $2, 'Assigned'), ($3, $2, 'Hidden')`, assignedProjectID, organizationID, hiddenProjectID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO project_assignments (id, project_id, user_id) VALUES ($1, $2, $3)`, assignmentID, assignedProjectID, memberID)
	}
	if err != nil {
		t.Fatalf("create organization fixtures: %v", err)
	}
	rawSession, csrfToken := "member-project-session", "member-project-csrf"
	_, err = pool.Exec(context.Background(), `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, sessionID, memberID, auth.HashActionToken(rawSession), auth.HashActionToken(csrfToken), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/placeholder/projects", nil)
	listRequest.SetPathValue("organizationID", organizationID.String())
	listRequest.AddCookie(&http.Cookie{Name: "timetracker_session", Value: rawSession})
	ProjectsHandler{Database: pool}.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("member project list status = %d; body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	if !bytes.Contains(listRecorder.Body.Bytes(), []byte("Assigned")) || bytes.Contains(listRecorder.Body.Bytes(), []byte("Hidden")) {
		t.Fatalf("member project list did not enforce assignment visibility: %s", listRecorder.Body.String())
	}

	tagRecorder := httptest.NewRecorder()
	tagRequest := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/placeholder/tags", bytes.NewBufferString(`{"name":"Forbidden"}`))
	tagRequest.Header.Set("X-CSRF-Token", csrfToken)
	tagRequest.SetPathValue("organizationID", organizationID.String())
	tagRequest.AddCookie(&http.Cookie{Name: "timetracker_session", Value: rawSession})
	TagsHandler{Database: pool}.ServeHTTP(tagRecorder, tagRequest)
	if tagRecorder.Code != http.StatusForbidden {
		t.Fatalf("member tag creation status = %d, want %d; body=%s", tagRecorder.Code, http.StatusForbidden, tagRecorder.Body.String())
	}
}

func testUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
