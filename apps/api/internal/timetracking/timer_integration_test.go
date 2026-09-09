package timetracking

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTimerStateFlowAndRetries(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	organizationID, userID, projectID, otherProjectID, tagID, sessionID := newID(t), newID(t), newID(t), newID(t), newID(t), newID(t)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM time_entries WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tags WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM memberships WHERE organization_id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	}
	defer cleanup()

	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at) VALUES ($1, $2, 'not-used', 'Timer user', 'UTC', CURRENT_TIMESTAMP)`, userID, "timer-"+userID.String()+"@example.com")
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO organizations (id, name, timezone) VALUES ($1, 'Timer test', 'UTC')`, organizationID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'ADMIN')`, newID(t), organizationID, userID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO projects (id, organization_id, name) VALUES ($1, $2, 'Primary'), ($3, $2, 'Other')`, projectID, organizationID, otherProjectID)
	}
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO tags (id, organization_id, name) VALUES ($1, $2, 'Billable')`, tagID, organizationID)
	}
	rawSession, csrfToken := "timer-session-"+userID.String(), "timer-csrf-"+userID.String()
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, sessionID, userID, auth.HashActionToken(rawSession), auth.HashActionToken(csrfToken), time.Now().Add(time.Hour))
	}
	if err != nil {
		t.Fatalf("create timer fixtures: %v", err)
	}

	handler := Handler{Database: pool}
	startBody := `{"organizationId":"` + organizationID.String() + `","projectId":"` + projectID.String() + `","tagIds":["` + tagID.String() + `"]}`
	first := invoke(t, handler.Start, http.MethodPost, startBody, rawSession, csrfToken)
	if first.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", first.Code, first.Body.String())
	}
	active := decodeEntry(t, first)
	if active.Data.Status != "RUNNING" || active.Data.ID == uuid.Nil {
		t.Fatalf("start response was invalid: %#v", active)
	}
	if retry := invoke(t, handler.Start, http.MethodPost, startBody, rawSession, csrfToken); retry.Code != http.StatusOK {
		t.Fatalf("same start retry status=%d body=%s", retry.Code, retry.Body.String())
	}
	otherStart := `{"organizationId":"` + organizationID.String() + `","projectId":"` + otherProjectID.String() + `","tagIds":[]}`
	if conflict := invoke(t, handler.Start, http.MethodPost, otherStart, rawSession, csrfToken); conflict.Code != http.StatusConflict {
		t.Fatalf("different active start status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	updateBody := `{"entryId":"` + active.Data.ID.String() + `","projectId":"` + otherProjectID.String() + `","tagIds":[]}`
	if updated := invoke(t, handler.UpdateActive, http.MethodPatch, updateBody, rawSession, csrfToken); updated.Code != http.StatusOK || decodeEntry(t, updated).Data.Status != "RUNNING" {
		t.Fatalf("update active timer status=%d body=%s", updated.Code, updated.Body.String())
	}

	entryBody := `{"entryId":"` + active.Data.ID.String() + `"}`
	for _, step := range []struct {
		name   string
		action func(http.ResponseWriter, *http.Request)
		status string
	}{
		{"pause", handler.Pause, "PAUSED"},
		{"pause retry", handler.Pause, "PAUSED"},
		{"resume", handler.Resume, "RUNNING"},
		{"stop", handler.Stop, "STOPPED"},
		{"stop retry", handler.Stop, "STOPPED"},
	} {
		recorder := invoke(t, step.action, http.MethodPost, entryBody, rawSession, csrfToken)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", step.name, recorder.Code, recorder.Body.String())
		}
		if got := decodeEntry(t, recorder).Data.Status; got != step.status {
			t.Fatalf("%s status=%q want %q", step.name, got, step.status)
		}
	}
	activeRecorder := invoke(t, handler.Active, http.MethodGet, "", rawSession, "")
	if activeRecorder.Code != http.StatusOK || !bytes.Contains(activeRecorder.Body.Bytes(), []byte(`"data":null`)) {
		t.Fatalf("active after stop status=%d body=%s", activeRecorder.Code, activeRecorder.Body.String())
	}
	var eventCount int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM timer_events WHERE time_entry_id = $1`, active.Data.ID).Scan(&eventCount); err != nil || eventCount != 4 {
		t.Fatalf("event count=%d err=%v, want 4", eventCount, err)
	}

	statuses := make(chan int, 2)
	for range 2 {
		go func() { statuses <- invoke(t, handler.Start, http.MethodPost, startBody, rawSession, csrfToken).Code }()
	}
	created, retried := 0, 0
	for range 2 {
		switch <-statuses {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			retried++
		}
	}
	if created != 1 || retried != 1 {
		t.Fatalf("concurrent starts created=%d retried=%d, want one of each", created, retried)
	}
	var activeCount int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM time_entries WHERE user_id = $1 AND status IN ('RUNNING', 'PAUSED')`, userID).Scan(&activeCount); err != nil || activeCount != 1 {
		t.Fatalf("active timer count=%d err=%v, want 1", activeCount, err)
	}
}

type timerResponse struct {
	Data struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	} `json:"data"`
}

func decodeEntry(t *testing.T, recorder *httptest.ResponseRecorder) timerResponse {
	t.Helper()
	var response timerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func invoke(t *testing.T, action func(http.ResponseWriter, *http.Request), method, body, session, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "/api/v1/timer", bytes.NewBufferString(body))
	request.AddCookie(&http.Cookie{Name: "timetracker_session", Value: session})
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	recorder := httptest.NewRecorder()
	action(recorder, request)
	return recorder
}

func newID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
