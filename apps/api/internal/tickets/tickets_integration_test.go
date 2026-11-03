package tickets_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/fortune-tech/time-tracker/apps/api/internal/reporting"
	"github.com/fortune-tech/time-tracker/apps/api/internal/tickets"
	"github.com/fortune-tech/time-tracker/apps/api/internal/timetracking"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixture struct {
	pool                   *pgxpool.Pool
	org, otherOrg, project uuid.UUID
	users                  [3]uuid.UUID
	sessions, csrf         [3]string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{pool: pool, org: uuid.New(), otherOrg: uuid.New(), project: uuid.New()}
	for i := range f.users {
		f.users[i] = uuid.New()
		f.sessions[i] = "ticket-session-" + f.users[i].String()
		f.csrf[i] = "ticket-csrf-" + f.users[i].String()
	}
	t.Cleanup(func() {
		ctx := context.Background()
		for _, query := range []string{
			`DELETE FROM time_entries WHERE organization_id = ANY($1)`,
			`DELETE FROM tickets WHERE organization_id = ANY($1)`,
			`DELETE FROM audit_logs WHERE organization_id = ANY($1)`,
			`DELETE FROM project_assignments WHERE project_id IN (SELECT id FROM projects WHERE organization_id = ANY($1))`,
			`DELETE FROM projects WHERE organization_id = ANY($1)`,
			`DELETE FROM memberships WHERE organization_id = ANY($1)`,
			`DELETE FROM organizations WHERE id = ANY($1)`,
		} {
			if _, err := pool.Exec(ctx, query, []uuid.UUID{f.org, f.otherOrg}); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = ANY($1)`, f.users[:]); err != nil {
			t.Errorf("cleanup sessions: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, f.users[:]); err != nil {
			t.Errorf("cleanup users: %v", err)
		}
		pool.Close()
	})
	f.exec(t, `INSERT INTO organizations (id,name,timezone) VALUES ($1,'Ticket integration','UTC'),($2,'Other organization','UTC')`, f.org, f.otherOrg)
	for i, user := range f.users {
		f.exec(t, `INSERT INTO users (id,email,password_hash,name,timezone,email_verified_at) VALUES ($1,$2,'not-used','Ticket tester','UTC',CURRENT_TIMESTAMP)`, user, user.String()+"@example.com")
		role := "MEMBER"
		if i == 0 {
			role = "ADMIN"
		}
		f.exec(t, `INSERT INTO memberships (id,organization_id,user_id,role) VALUES ($1,$2,$3,$4)`, uuid.New(), f.org, user, role)
		f.exec(t, `INSERT INTO sessions (id,user_id,token_hash,csrf_token_hash,expires_at) VALUES ($1,$2,$3,$4,$5)`, uuid.New(), user, auth.HashActionToken(f.sessions[i]), auth.HashActionToken(f.csrf[i]), time.Now().Add(time.Hour))
	}
	f.exec(t, `INSERT INTO memberships (id,organization_id,user_id,role) VALUES ($1,$2,$3,'ADMIN')`, uuid.New(), f.otherOrg, f.users[0])
	f.exec(t, `INSERT INTO projects (id,organization_id,name) VALUES ($1,$2,'Assigned project')`, f.project, f.org)
	for _, user := range f.users {
		f.exec(t, `INSERT INTO project_assignments (id,project_id,user_id) VALUES ($1,$2,$3)`, uuid.New(), f.project, user)
	}
	return f
}

func (f *fixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) call(t *testing.T, action http.HandlerFunc, actor int, method, url string, body any, paths map[string]string, status int) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, url, bytes.NewReader(payload))
	request.AddCookie(&http.Cookie{Name: "timetracker_session", Value: f.sessions[actor]})
	if method != http.MethodGet {
		request.Header.Set("X-CSRF-Token", f.csrf[actor])
	}
	for key, value := range paths {
		request.SetPathValue(key, value)
	}
	recorder := httptest.NewRecorder()
	action(recorder, request)
	if recorder.Code != status {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, url, recorder.Code, status, recorder.Body.String())
	}
	assertIntegrationContract(t, action, method, recorder)
	return recorder
}
func decodeData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Data  T              `json:"data"`
		Error any            `json:"error"`
		Meta  map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error != nil || envelope.Meta == nil {
		t.Fatalf("invalid success envelope: %s", recorder.Body.String())
	}
	return envelope.Data
}
func (f *fixture) paths(id uuid.UUID) map[string]string {
	return map[string]string{"organizationID": f.org.String(), "ticketID": id.String()}
}
func (f *fixture) create(t *testing.T, actor int, reference string) uuid.UUID {
	result := f.call(t, tickets.Handler{Database: f.pool}.Create, actor, "POST", "/tickets", map[string]string{"reference": reference, "title": "Investigate bug"}, f.paths(uuid.Nil), 201)
	return decodeData[struct {
		ID uuid.UUID `json:"id"`
	}](t, result).ID
}

func TestTicketPermissionsIsolationAndPagination(t *testing.T) {
	f := setup(t)
	handler := tickets.Handler{Database: f.pool}
	own := f.create(t, 0, "APP-1")
	second := f.create(t, 0, "APP-2")
	third := f.create(t, 0, "APP-3")
	input := map[string]string{"reference": "APP-1", "title": "Updated title"}
	f.call(t, handler.Update, 2, "PATCH", "/tickets", input, f.paths(own), 403)
	f.call(t, handler.Delete, 2, "DELETE", "/tickets", nil, f.paths(own), 403)
	f.call(t, handler.Update, 1, "PATCH", "/tickets", input, f.paths(own), 403)
	f.call(t, handler.Update, 0, "PATCH", "/tickets", input, f.paths(own), 200)
	f.call(t, handler.Create, 1, "POST", "/tickets", map[string]string{"reference": "app-1", "title": "Duplicate"}, f.paths(uuid.Nil), 403)
	f.call(t, handler.Create, 0, "POST", "/tickets", map[string]string{"reference": "app-1", "title": "Duplicate"}, f.paths(uuid.Nil), 409)
	foreignPaths := map[string]string{"organizationID": f.otherOrg.String(), "ticketID": own.String()}
	f.call(t, handler.Get, 0, "GET", "/tickets", nil, foreignPaths, 404)
	f.call(t, handler.Get, 1, "GET", "/tickets", nil, foreignPaths, 403)
	for _, query := range []string{"limit=0", "limit=101", "offset=-1", "limit=no", "query=" + strings.Repeat("x", 101)} {
		f.call(t, handler.List, 1, "GET", "/tickets?"+query, nil, f.paths(uuid.Nil), 400)
	}
	page := decodeData[struct {
		Items                []struct{ ID uuid.UUID }
		Total, Limit, Offset int
	}](t, f.call(t, handler.List, 1, "GET", "/tickets?limit=1&offset=1&query=app-", nil, f.paths(uuid.Nil), 200))
	if len(page.Items) != 1 || page.Total != 3 || page.Limit != 1 || page.Offset != 1 || page.Items[0].ID != second {
		t.Fatalf("pagination=%+v", page)
	}
	searched := decodeData[struct {
		Items []struct{ ID uuid.UUID }
		Total int
	}](t, f.call(t, handler.List, 1, "GET", "/tickets?query=updated", nil, f.paths(uuid.Nil), 200))
	if searched.Total != 1 || searched.Items[0].ID != own {
		t.Fatalf("search=%+v", searched)
	}
	f.call(t, handler.Delete, 1, "DELETE", "/tickets", nil, f.paths(second), 403)
	f.call(t, handler.Delete, 0, "DELETE", "/tickets", nil, f.paths(second), 200)
	f.call(t, handler.Delete, 0, "DELETE", "/tickets", nil, f.paths(third), 200)
	f.exec(t, `DELETE FROM memberships WHERE organization_id=$1 AND user_id=$2`, f.org, f.users[1])
	f.call(t, handler.Get, 1, "GET", "/tickets", nil, f.paths(own), 403)
	f.call(t, handler.Update, 1, "PATCH", "/tickets", input, f.paths(own), 403)
	f.call(t, handler.Delete, 1, "DELETE", "/tickets", nil, f.paths(own), 403)
	f.call(t, handler.Create, 1, "POST", "/tickets", input, f.paths(uuid.Nil), 403)
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE organization_id=$1 AND target_type='TICKET'`, f.org).Scan(&count); err != nil || count != 6 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

func TestAdminTrackerViewIsReadOnly(t *testing.T) {
	f := setup(t)
	handler := timetracking.Handler{Database: f.pool}
	start := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Minute)
	entry := decodeData[struct{ ID uuid.UUID }](t, f.call(t, handler.CreateManual, 1, "POST", "/manual", map[string]any{
		"organizationId": f.org, "projectId": f.project, "description": "Member work", "tagIds": []string{},
		"startedAt": start, "endedAt": start.Add(time.Hour),
	}, nil, 201))
	base := "/tracker?organizationId=" + f.org.String() + "&userId=" + f.users[1].String()
	entries := decodeData[[]struct{ ID uuid.UUID }](t, f.call(t, handler.TodayEntries, 0, "GET", base+"&date="+start.Format("2006-01-02"), nil, nil, 200))
	if len(entries) != 1 || entries[0].ID != entry.ID {
		t.Fatalf("admin tracker entries=%+v", entries)
	}
	allEntries := decodeData[struct {
		Items []struct{ ID uuid.UUID }
		Total int
	}](t, f.call(t, handler.ListCompleted, 0, "GET", "/entries?organizationId="+f.org.String()+"&userId=all", nil, nil, 200))
	if allEntries.Total != 1 || len(allEntries.Items) != 1 || allEntries.Items[0].ID != entry.ID {
		t.Fatalf("admin all-member entries=%+v", allEntries)
	}
	f.call(t, handler.ListCompleted, 1, "GET", "/entries?organizationId="+f.org.String()+"&userId=all", nil, nil, 403)
	f.call(t, handler.WeekSummary, 0, "GET", base+"&date="+start.Format("2006-01-02"), nil, nil, 200)
	active := decodeData[struct{ ID uuid.UUID }](t, f.call(t, handler.Start, 1, "POST", "/timer/start", map[string]any{
		"organizationId": f.org, "projectId": f.project, "description": "Live member work", "tagIds": []string{},
	}, nil, 201))
	visible := decodeData[struct{ ID uuid.UUID }](t, f.call(t, handler.Active, 0, "GET", base, nil, nil, 200))
	if visible.ID != active.ID {
		t.Fatalf("admin saw active timer %s, want %s", visible.ID, active.ID)
	}
	f.call(t, handler.Stop, 0, "POST", "/timer/stop", map[string]any{"entryId": active.ID}, nil, 404)
	f.call(t, handler.Stop, 1, "POST", "/timer/stop", map[string]any{"entryId": active.ID}, nil, 200)
	f.call(t, handler.UpdateTimerEvents, 0, "PUT", "/entries?organizationId="+f.org.String(), map[string]any{
		"events": []map[string]any{{"type": "START", "occurredAt": start}, {"type": "STOP", "occurredAt": start.Add(time.Minute)}},
	}, map[string]string{"entryID": active.ID.String()}, 403)
	f.call(t, handler.TodayEntries, 2, "GET", base, nil, nil, 403)
	f.call(t, handler.WeekSummary, 2, "GET", base, nil, nil, 403)
	f.call(t, handler.Active, 2, "GET", base, nil, nil, 403)
	entryURL := "/entries?organizationId=" + f.org.String()
	entryPaths := map[string]string{"entryID": entry.ID.String()}
	change := map[string]any{"projectId": f.project, "description": "Admin edit", "tagIds": []string{}, "ticketIds": []string{}}
	f.call(t, handler.UpdateCompleted, 0, "PATCH", entryURL, change, entryPaths, 403)
	f.call(t, handler.DeleteCompleted, 0, "DELETE", entryURL, nil, entryPaths, 403)
	f.call(t, handler.UpdateCompleted, 1, "PATCH", entryURL, change, entryPaths, 200)
}

func TestMultiTicketEntriesReportsAndDeletion(t *testing.T) {
	f := setup(t)
	ticketHandler := tickets.Handler{Database: f.pool}
	entries := timetracking.Handler{Database: f.pool}
	reports := reporting.Handler{Database: f.pool}
	first, second := f.create(t, 0, "APP-1"), f.create(t, 0, "APP-2")
	foreign := uuid.New()
	f.exec(t, `INSERT INTO tickets(id,organization_id,created_by_user_id,reference,title) VALUES($1,$2,$3,'OTHER-1','Other')`, foreign, f.otherOrg, f.users[0])
	start := time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour).Add(8 * time.Hour)
	body := map[string]any{"organizationId": f.org, "projectId": f.project, "description": "Investigation", "tagIds": []string{}, "ticketIds": []uuid.UUID{first, second}, "startedAt": start, "endedAt": start.Add(time.Hour)}
	created := decodeData[struct{ ID uuid.UUID }](t, f.call(t, entries.CreateManual, 1, "POST", "/manual", body, nil, 201))
	_, invalidLinkErr := f.pool.Exec(context.Background(), `INSERT INTO time_entry_tickets(organization_id,time_entry_id,ticket_id) VALUES($1,$2,$3)`, f.org, created.ID, foreign)
	var constraintError *pgconn.PgError
	if !errors.As(invalidLinkErr, &constraintError) || constraintError.Code != "23503" {
		t.Fatalf("cross-organization link was not rejected by FK: %v", invalidLinkErr)
	}
	body["startedAt"], body["endedAt"] = start.Add(2*time.Hour), start.Add(3*time.Hour)
	body["ticketIds"] = []uuid.UUID{first, first}
	f.call(t, entries.CreateManual, 1, "POST", "/manual", body, nil, 400)
	body["ticketIds"] = []uuid.UUID{foreign}
	f.call(t, entries.CreateManual, 1, "POST", "/manual", body, nil, 403)
	body["ticketIds"] = []uuid.UUID{}
	f.call(t, entries.CreateManual, 1, "POST", "/manual", body, nil, 201)
	entryURL := "/entries?organizationId=" + f.org.String()
	entryPaths := map[string]string{"entryID": created.ID.String()}
	detail := decodeData[struct {
		Tickets []struct {
			ID        uuid.UUID
			Reference string
		}
	}](t, f.call(t, entries.CompletedDetail, 1, "GET", entryURL, nil, entryPaths, 200))
	if len(detail.Tickets) != 2 {
		t.Fatalf("entry tickets=%+v", detail)
	}
	f.call(t, ticketHandler.Delete, 0, "DELETE", "/tickets", nil, f.paths(first), 409)
	f.call(t, ticketHandler.Update, 0, "PATCH", "/tickets", map[string]string{"reference": "APP-RENAMED", "title": "Renamed ticket"}, f.paths(first), 200)
	detail = decodeData[struct {
		Tickets []struct {
			ID        uuid.UUID
			Reference string
		}
	}](t, f.call(t, entries.CompletedDetail, 1, "GET", entryURL, nil, entryPaths, 200))
	if !strings.Contains(fmt.Sprint(detail.Tickets), "APP-RENAMED") {
		t.Fatalf("historical rename missing: %+v", detail)
	}
	reportURL := fmt.Sprintf("/reports?organizationId=%s&startDate=%s&endDate=%s", f.org, start.Format("2006-01-02"), start.Format("2006-01-02"))
	for _, suffix := range []string{"", "&ticketId=" + first.String() + "," + second.String()} {
		result := decodeData[struct {
			Items                []json.RawMessage
			Total                int
			TotalDurationSeconds int64
		}](t, f.call(t, reports.Entries, 1, "GET", reportURL+suffix, nil, nil, 200))
		wantTotal, wantSeconds := 2, int64(7200)
		if suffix != "" {
			wantTotal, wantSeconds = 1, 3600
		}
		if result.Total != wantTotal || result.TotalDurationSeconds != wantSeconds {
			t.Fatalf("report totals=%+v", result)
		}
	}
	summary := decodeData[struct {
		Items                []struct{ DurationSeconds int64 }
		Total                int
		TotalDurationSeconds int64
	}](t, f.call(t, reports.Summary, 1, "GET", reportURL+"&groupBy=ticket", nil, nil, 200))
	if summary.Total != 3 || summary.TotalDurationSeconds != 7200 {
		t.Fatalf("summary=%+v", summary)
	}
	for _, item := range summary.Items {
		if item.DurationSeconds != 3600 {
			t.Fatalf("summary row=%+v", item)
		}
	}
	f.call(t, reports.Entries, 1, "GET", reportURL+"&scope=organization", nil, nil, 403)
	f.call(t, reports.Entries, 0, "GET", reportURL+"&scope=organization", nil, nil, 200)
	for _, format := range []string{"detailed", "summary"} {
		recorder := f.call(t, reports.Export, 1, "GET", reportURL+"&format="+format+"&groupBy=ticket", nil, nil, 200)
		records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		want := 3
		if format == "summary" {
			want = 4
		}
		if len(records) != want || !strings.Contains(recorder.Body.String(), "APP-RENAMED") || recorder.Header().Get("Content-Disposition") == "" {
			t.Fatalf("%s CSV: %s", format, recorder.Body.String())
		}
		if format == "detailed" {
			for _, row := range records[1:] {
				if row[9] != "3600" {
					t.Fatalf("CSV duration=%s", row[9])
				}
			}
		}
	}
	update := map[string]any{"projectId": f.project, "description": "Reclassified", "tagIds": []string{}, "ticketIds": []uuid.UUID{second}}
	f.call(t, entries.UpdateCompleted, 1, "PATCH", entryURL, update, entryPaths, 200)
	f.call(t, ticketHandler.Delete, 0, "DELETE", "/tickets", nil, f.paths(first), 200)
	f.call(t, entries.DeleteCompleted, 1, "DELETE", entryURL, nil, entryPaths, 200)
	f.call(t, ticketHandler.Delete, 0, "DELETE", "/tickets", nil, f.paths(second), 200)
	var links int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM time_entry_tickets WHERE time_entry_id=$1`, created.ID).Scan(&links); err != nil || links != 0 {
		t.Fatalf("link cleanup=%d %v", links, err)
	}
}

func TestActiveTimerMultipleTicketsAndRefresh(t *testing.T) {
	f := setup(t)
	handler := timetracking.Handler{Database: f.pool}
	first, second := f.create(t, 0, "APP-1"), f.create(t, 0, "APP-2")
	body := map[string]any{"organizationId": f.org, "projectId": f.project, "description": "Investigation", "tagIds": []string{}, "ticketIds": []uuid.UUID{first, second}}
	result := decodeData[struct {
		ID               uuid.UUID
		TicketReferences []string
	}](t, f.call(t, handler.Start, 1, "POST", "/timer/start", body, nil, 201))
	if strings.Join(result.TicketReferences, ", ") != "APP-1, APP-2" {
		t.Fatalf("start references=%+v", result)
	}
	activeURL := "/timer/active?organizationId=" + f.org.String()
	refreshed := decodeData[struct{ TicketReferences []string }](t, f.call(t, handler.Active, 1, "GET", activeURL, nil, nil, 200))
	if len(refreshed.TicketReferences) != 2 {
		t.Fatalf("refresh=%+v", refreshed)
	}
	update := map[string]any{"entryId": result.ID, "projectId": f.project, "description": "Investigation", "tagIds": []string{}, "ticketIds": []uuid.UUID{second}}
	changed := decodeData[struct{ TicketReferences []string }](t, f.call(t, handler.UpdateActive, 1, "PATCH", activeURL, update, nil, 200))
	if strings.Join(changed.TicketReferences, ",") != "APP-2" {
		t.Fatalf("update=%+v", changed)
	}
	update["ticketIds"] = []uuid.UUID{}
	changed = decodeData[struct{ TicketReferences []string }](t, f.call(t, handler.UpdateActive, 1, "PATCH", activeURL, update, nil, 200))
	if changed.TicketReferences == nil || len(changed.TicketReferences) != 0 {
		t.Fatalf("no-ticket response=%+v", changed)
	}
	f.call(t, handler.Stop, 1, "POST", "/timer/stop", map[string]any{"entryId": result.ID}, nil, 200)
}

func TestTimerReclassificationPreservesPausedDuration(t *testing.T) {
	f := setup(t)
	handler := timetracking.Handler{Database: f.pool}
	ticketID := f.create(t, 0, "APP-1")
	entryID := uuid.New()
	start := time.Now().UTC().AddDate(0, 0, -3).Truncate(time.Minute)
	f.exec(t, `INSERT INTO time_entries(id,organization_id,user_id,project_id,description,source_type,status,started_at,ended_at,duration_seconds) VALUES($1,$2,$3,$4,'Paused work','TIMER','STOPPED',$5,$6,2400)`, entryID, f.org, f.users[1], f.project, start, start.Add(time.Hour))
	for index, event := range []struct {
		kind    string
		minutes int
	}{{"START", 0}, {"PAUSE", 20}, {"RESUME", 40}, {"STOP", 60}} {
		f.exec(t, `INSERT INTO timer_events(id,time_entry_id,event_type,occurred_at,sequence_number) VALUES($1,$2,$3,$4,$5)`, uuid.New(), entryID, event.kind, start.Add(time.Duration(event.minutes)*time.Minute), index+1)
	}
	result := decodeData[struct{ DurationSeconds int64 }](t, f.call(t, handler.UpdateCompleted, 1, "PATCH", "/entries?organizationId="+f.org.String(), map[string]any{"projectId": f.project, "description": "Updated description", "tagIds": []string{}, "ticketIds": []uuid.UUID{ticketID}}, map[string]string{"entryID": entryID.String()}, 200))
	if result.DurationSeconds != 2400 {
		t.Fatalf("reclassification counted paused time: %d", result.DurationSeconds)
	}
	var seconds int64
	if err := f.pool.QueryRow(context.Background(), `SELECT duration_seconds FROM time_entries WHERE id=$1`, entryID).Scan(&seconds); err != nil || seconds != 2400 {
		t.Fatalf("stored duration=%d err=%v", seconds, err)
	}
}

func TestTicketAuditFailureRollsBackMutation(t *testing.T) {
	f := setup(t)
	handler := tickets.Handler{Database: f.pool}
	id := f.create(t, 0, "ATOMIC-1")
	functionName := "ticket_audit_test_" + strings.ReplaceAll(f.org.String(), "-", "")
	triggerName := functionName + "_trigger"
	f.exec(t, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.organization_id = '%s'::uuid THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$`, functionName, f.org))
	t.Cleanup(func() { f.exec(t, fmt.Sprintf(`DROP FUNCTION %s()`, functionName)) })
	f.exec(t, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName))
	t.Cleanup(func() { f.exec(t, fmt.Sprintf(`DROP TRIGGER %s ON audit_logs`, triggerName)) })
	f.call(t, handler.Create, 0, "POST", "/tickets", map[string]string{"reference": "ATOMIC-2", "title": "Must roll back"}, f.paths(uuid.Nil), 500)
	f.call(t, handler.Update, 0, "PATCH", "/tickets", map[string]string{"reference": "ATOMIC-CHANGED", "title": "Must roll back"}, f.paths(id), 500)
	f.call(t, handler.Delete, 0, "DELETE", "/tickets", nil, f.paths(id), 500)
	var count int
	var reference string
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM tickets WHERE organization_id=$1`, f.org).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback count=%d err=%v", count, err)
	}
	if err := f.pool.QueryRow(context.Background(), `SELECT reference FROM tickets WHERE id=$1`, id).Scan(&reference); err != nil || reference != "ATOMIC-1" {
		t.Fatalf("rollback reference=%s err=%v", reference, err)
	}
}
