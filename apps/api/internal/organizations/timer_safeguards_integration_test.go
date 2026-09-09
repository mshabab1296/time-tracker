package organizations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestActiveTimerBlocksMemberAndAssignmentRemovalAndReferencedResourceDeletion(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	organizationID, adminID, memberID := testUUID(t), testUUID(t), testUUID(t)
	projectID, tagID, entryID, sessionID := testUUID(t), testUUID(t), testUUID(t), testUUID(t)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE user_id = $1`, adminID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM time_entries WHERE id = $1`, entryID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_assignments WHERE project_id = $1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tags WHERE id = $1`, tagID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM memberships WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, adminID, memberID)
	}
	defer cleanup()

	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at) VALUES ($1, $2, 'not-used', 'Admin', 'UTC', CURRENT_TIMESTAMP), ($3, $4, 'not-used', 'Member', 'UTC', CURRENT_TIMESTAMP)`, adminID, "safeguard-admin-"+adminID.String()+"@example.com", memberID, "safeguard-member-"+memberID.String()+"@example.com")
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO organizations (id, name, timezone) VALUES ($1, 'Safeguard test', 'UTC')`, organizationID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'ADMIN'), ($4, $2, $5, 'MEMBER')`, testUUID(t), organizationID, adminID, testUUID(t), memberID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO projects (id, organization_id, name) VALUES ($1, $2, 'Tracked project')`, projectID, organizationID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO project_assignments (id, project_id, user_id) VALUES ($1, $2, $3)`, testUUID(t), projectID, memberID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO tags (id, organization_id, name) VALUES ($1, $2, 'Tracked tag')`, tagID, organizationID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO time_entries (id, organization_id, user_id, project_id, source_type, status, started_at) VALUES ($1, $2, $3, $4, 'TIMER', 'RUNNING', CURRENT_TIMESTAMP)`, entryID, organizationID, memberID, projectID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO time_entry_tags (time_entry_id, tag_id) VALUES ($1, $2)`, entryID, tagID)
	}
	rawSession, csrfToken := "safeguard-session-"+adminID.String(), "safeguard-csrf-"+adminID.String()
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, sessionID, adminID, auth.HashActionToken(rawSession), auth.HashActionToken(csrfToken), time.Now().Add(time.Hour))
	}
	if err != nil {
		t.Fatalf("create safeguards fixtures: %v", err)
	}

	removeRequest := adminRequest(http.MethodDelete, rawSession, csrfToken)
	removeRequest.SetPathValue("organizationID", organizationID.String())
	removeRequest.SetPathValue("userID", memberID.String())
	removeRecorder := httptest.NewRecorder()
	RemoveMemberHandler{Database: pool}.ServeHTTP(removeRecorder, removeRequest)
	if removeRecorder.Code != http.StatusConflict {
		t.Fatalf("remove member status=%d body=%s", removeRecorder.Code, removeRecorder.Body.String())
	}

	unassignRequest := adminRequest(http.MethodDelete, rawSession, csrfToken)
	unassignRequest.SetPathValue("organizationID", organizationID.String())
	unassignRequest.SetPathValue("projectID", projectID.String())
	unassignRequest.SetPathValue("userID", memberID.String())
	unassignRecorder := httptest.NewRecorder()
	AssignmentHandler{Database: pool}.ServeHTTP(unassignRecorder, unassignRequest)
	if unassignRecorder.Code != http.StatusConflict {
		t.Fatalf("unassign project status=%d body=%s", unassignRecorder.Code, unassignRecorder.Body.String())
	}

	projectRequest := adminRequest(http.MethodDelete, rawSession, csrfToken)
	projectRequest.SetPathValue("organizationID", organizationID.String())
	projectRequest.SetPathValue("projectID", projectID.String())
	projectRecorder := httptest.NewRecorder()
	ProjectHandler{Database: pool}.ServeHTTP(projectRecorder, projectRequest)
	if projectRecorder.Code != http.StatusConflict {
		t.Fatalf("delete project status=%d body=%s", projectRecorder.Code, projectRecorder.Body.String())
	}

	tagRequest := adminRequest(http.MethodDelete, rawSession, csrfToken)
	tagRequest.SetPathValue("organizationID", organizationID.String())
	tagRequest.SetPathValue("tagID", tagID.String())
	tagRecorder := httptest.NewRecorder()
	TagHandler{Database: pool}.ServeHTTP(tagRecorder, tagRequest)
	if tagRecorder.Code != http.StatusConflict {
		t.Fatalf("delete tag status=%d body=%s", tagRecorder.Code, tagRecorder.Body.String())
	}
}

func adminRequest(method, session, csrf string) *http.Request {
	request := httptest.NewRequest(method, "/api/v1/organizations/placeholder", nil)
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(&http.Cookie{Name: "timetracker_session", Value: session})
	return request
}
