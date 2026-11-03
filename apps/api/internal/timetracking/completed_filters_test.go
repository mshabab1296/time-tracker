package timetracking

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestParseCompletedEntryFilters(t *testing.T) {
	projectID := uuid.New()
	request := httptest.NewRequest(http.MethodGet, "/time-entries?projectId="+projectID.String()+"&date=2026-09-29&search=%20FLD-78%20", nil)
	recorder := httptest.NewRecorder()
	filters, ok := parseCompletedEntryFilters(recorder, request)
	if !ok || recorder.Code != http.StatusOK {
		t.Fatalf("filters rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if filters.ProjectID == nil || *filters.ProjectID != projectID || filters.Date == nil || *filters.Date != "2026-09-29" || filters.Search != "FLD-78" {
		t.Fatalf("unexpected filters: %#v", filters)
	}
}

func TestParseCompletedEntryFiltersRejectsInvalidValues(t *testing.T) {
	for _, test := range []struct {
		name  string
		query string
	}{
		{"project", "projectId=not-a-uuid"},
		{"date", "date=2026-02-30"},
		{"search", "search=" + strings.Repeat("a", 201)},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/time-entries?"+test.query, nil)
			recorder := httptest.NewRecorder()
			if _, ok := parseCompletedEntryFilters(recorder, request); ok || recorder.Code != http.StatusBadRequest {
				t.Fatalf("invalid %s accepted: status=%d body=%s", test.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRequestedEntryUserAllMembersRequiresAdmin(t *testing.T) {
	actorID := uuid.New()
	for _, test := range []struct {
		role       string
		wantStatus int
		wantOK     bool
	}{
		{"ADMIN", http.StatusOK, true},
		{"MEMBER", http.StatusForbidden, false},
	} {
		t.Run(test.role, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/time-entries?userId=all", nil)
			recorder := httptest.NewRecorder()
			targetID, ok := requestedEntryUser(recorder, request, actorID, test.role)
			if ok != test.wantOK || recorder.Code != test.wantStatus || targetID != uuid.Nil {
				t.Fatalf("target=%s ok=%v status=%d", targetID, ok, recorder.Code)
			}
		})
	}
}

func TestCompletedEntryDateFilterUsesMemberLocalDay(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	query := `WITH te AS (
		SELECT $1::timestamptz AS started_at, $2::timestamptz AS ended_at
	), u AS (SELECT $3::text AS timezone)
	SELECT ` + completedEntryDayOverlapSQL + ` FROM te, u`
	for _, test := range []struct {
		name, start, end string
		want             bool
	}{
		{"October 2 shortly after midnight", "2026-10-01T18:41:40Z", "2026-10-01T18:54:32Z", false},
		{"October 1 shortly after midnight", "2026-09-30T19:05:39Z", "2026-09-30T19:48:52Z", true},
		{"crosses midnight into October 2", "2026-10-01T18:20:00Z", "2026-10-01T18:40:00Z", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got bool
			if err := pool.QueryRow(context.Background(), query, test.start, test.end, "Asia/Calcutta", "2026-10-01").Scan(&got); err != nil {
				t.Fatalf("query member-local date filter: %v", err)
			}
			if got != test.want {
				t.Fatalf("date filter matched=%v, want %v", got, test.want)
			}
		})
	}
}

func TestCompletedEntriesAllMembersQuery(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin fixture transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	organizationID, projectID := uuid.New(), uuid.New()
	firstUserID, secondUserID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO users (id, email, password_hash, name, timezone, email_verified_at)
		VALUES ($1, $2, 'not-used', 'First member', 'Asia/Calcutta', CURRENT_TIMESTAMP),
		       ($3, $4, 'not-used', 'Second member', 'UTC', CURRENT_TIMESTAMP)`, firstUserID, firstUserID.String()+"@example.com", secondUserID, secondUserID.String()+"@example.com"); err != nil {
		t.Fatalf("insert test users: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO organizations (id, name, timezone) VALUES ($1, 'All entries test', 'UTC')`, organizationID); err != nil {
		t.Fatalf("insert test organization: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects (id, organization_id, name) VALUES ($1, $2, 'Test project')`, projectID, organizationID); err != nil {
		t.Fatalf("insert test project: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO time_entries (id, organization_id, user_id, project_id, source_type, status, started_at, ended_at, duration_seconds, description)
		VALUES ($1, $2, $3, $4, 'MANUAL', 'STOPPED', '2026-10-01T10:00:00Z', '2026-10-01T10:30:00Z', 1800, 'First work'),
		       ($5, $2, $6, $4, 'MANUAL', 'STOPPED', '2026-10-01T11:00:00Z', '2026-10-01T11:30:00Z', 1800, 'Second work')`, uuid.New(), organizationID, firstUserID, projectID, uuid.New(), secondUserID); err != nil {
		t.Fatalf("insert test entries: %v", err)
	}

	for _, test := range []struct {
		name       string
		userFilter any
		want       int
	}{
		{"all", nil, 2},
		{"one member", firstUserID, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var total int
			if err := tx.QueryRow(ctx, completedEntryCountQuery, organizationID, test.userFilter, nil, "2026-10-01", "").Scan(&total); err != nil {
				t.Fatalf("count completed entries: %v", err)
			}
			if total != test.want {
				t.Fatalf("total=%d, want %d", total, test.want)
			}
		})
	}
	rows, err := tx.Query(ctx, completedEntryQuery+` ORDER BY te.started_at DESC, te.id DESC LIMIT $6 OFFSET $7`, organizationID, nil, nil, "2026-10-01", "", 25, 0)
	if err != nil {
		t.Fatalf("list all completed entries: %v", err)
	}
	defer rows.Close()
	timezones := map[string]bool{}
	for rows.Next() {
		entry, err := scanCompletedEntry(rows)
		if err != nil {
			t.Fatalf("scan completed entry: %v", err)
		}
		timezones[entry.Timezone] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read completed entries: %v", err)
	}
	if !timezones["Asia/Calcutta"] || !timezones["UTC"] || len(timezones) != 2 {
		t.Fatalf("unexpected entry timezones: %v", timezones)
	}
}
