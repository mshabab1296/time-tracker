package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Opt-in: 1,000 authenticated users, each making one request every five seconds.
// Measures real HTTP + session validation + PostgreSQL queries, not mocked handlers.
func TestReleaseCapacity(t *testing.T) {
	if os.Getenv("RUN_CAPACITY_TEST") != "1" {
		t.Skip("set RUN_CAPACITY_TEST=1 to run the capacity scenario")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("capacity test requires DATABASE_URL for a disposable database")
	}
	seconds := 60
	mixed := os.Getenv("CAPACITY_MIXED") == "1"
	if raw := os.Getenv("CAPACITY_DURATION_SECONDS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 15 || value > 600 {
			t.Fatal("CAPACITY_DURATION_SECONDS must be 15..600")
		}
		seconds = value
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org, project := uuid.New(), uuid.New()
	users := make([]uuid.UUID, 1000)
	tokens, hashes := make([]string, 1000), make([]string, 1000)
	for index := range users {
		users[index] = uuid.New()
		tokens[index] = "capacity-" + users[index].String()
		hashes[index] = auth.HashActionToken(tokens[index])
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, query := range []string{`DELETE FROM time_entries WHERE organization_id=$1`, `DELETE FROM tickets WHERE organization_id=$1`, `DELETE FROM audit_logs WHERE organization_id=$1`, `DELETE FROM project_assignments WHERE project_id IN (SELECT id FROM projects WHERE organization_id=$1)`, `DELETE FROM projects WHERE organization_id=$1`, `DELETE FROM memberships WHERE organization_id=$1`, `DELETE FROM organizations WHERE id=$1`} {
			if _, err := pool.Exec(ctx, query, org); err != nil {
				t.Errorf("capacity cleanup: %v", err)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=ANY($1)`, users); err != nil {
			t.Errorf("cleanup sessions: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id=ANY($1)`, users); err != nil {
			t.Errorf("cleanup users: %v", err)
		}
		pool.Close()
	})
	exec(`INSERT INTO users(id,email,password_hash,name,timezone,email_verified_at) SELECT id,id::text||'@example.test','not-used','Load tester','UTC',CURRENT_TIMESTAMP FROM unnest($1::uuid[]) AS id`, users)
	exec(`INSERT INTO organizations(id,name,timezone) VALUES($1,'Capacity fixture','UTC')`, org)
	exec(`INSERT INTO memberships(id,organization_id,user_id,role) SELECT gen_random_uuid(),$1,id,'MEMBER' FROM unnest($2::uuid[]) AS id`, org, users)
	exec(`INSERT INTO sessions(id,user_id,token_hash,csrf_token_hash,expires_at) SELECT gen_random_uuid(),id,hash,hash,CURRENT_TIMESTAMP+INTERVAL '1 hour' FROM unnest($1::uuid[],$2::text[]) AS source(id,hash)`, users, hashes)
	exec(`INSERT INTO projects(id,organization_id,name) VALUES($1,$2,'Load project')`, project, org)
	exec(`INSERT INTO project_assignments(id,project_id,user_id) SELECT gen_random_uuid(),$1,id FROM unnest($2::uuid[]) AS id`, project, users)
	started := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour).Add(9 * time.Hour)
	exec(`INSERT INTO time_entries(id,organization_id,user_id,project_id,description,source_type,status,started_at,ended_at,duration_seconds) SELECT gen_random_uuid(),$1,id,$2,'Load work','MANUAL','STOPPED',$3,$4,3600 FROM unnest($5::uuid[]) AS id`, org, project, started, started.Add(time.Hour), users)
	if mixed {
		// 99 historical days per user: 100,000 total entries including yesterday.
		exec(`INSERT INTO time_entries(id,organization_id,user_id,project_id,description,source_type,status,started_at,ended_at,duration_seconds) SELECT gen_random_uuid(),$1,id,$2,'Historical load work','MANUAL','STOPPED',$3::timestamptz-n*INTERVAL '1 day',$4::timestamptz-n*INTERVAL '1 day',3600 FROM unnest($5::uuid[]) AS id CROSS JOIN generate_series(1,99) AS n`, org, project, started, started.Add(time.Hour), users)
	}
	exec(`INSERT INTO tickets(id,organization_id,created_by_user_id,reference,title) SELECT gen_random_uuid(),$1,$2,'LOAD-'||n,'Load ticket' FROM generate_series(1,1000) AS n`, org, users[0])
	exec(`INSERT INTO time_entry_tickets(organization_id,time_entry_id,ticket_id) SELECT $1,te.id,t.id FROM time_entries te CROSS JOIN (SELECT id FROM tickets WHERE organization_id=$1 ORDER BY reference LIMIT 1) t WHERE te.organization_id=$1`, org)
	server := httptest.NewServer(New(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "http://localhost", false))
	defer server.Close()
	transport := &http.Transport{MaxIdleConns: 1000, MaxIdleConnsPerHost: 1000}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	date := started.Format("2006-01-02")
	paths := []string{"/api/v1/auth/me", "/api/v1/timer/active?organizationId=" + org.String(), "/api/v1/time-entries/today?organizationId=" + org.String() + "&date=" + date, "/api/v1/organizations/" + org.String() + "/tickets?limit=25", "/api/v1/reports/entries?organizationId=" + org.String() + "&startDate=" + date + "&endDate=" + date}
	readEndpoints := len(paths)
	if mixed {
		paths = append(paths, "/api/v1/timer/start", "/api/v1/timer/pause", "/api/v1/timer/resume", "/api/v1/timer/stop", "/api/v1/time-entries/manual")
	}
	var mutex sync.Mutex
	latencies := make([][]time.Duration, len(paths))
	failures := 0
	var workers sync.WaitGroup
	start := time.Now()
	end := start.Add(time.Duration(seconds) * time.Second)
	for user := range users {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			var timerID uuid.UUID
			next := start.Add(time.Duration(index) * 5 * time.Millisecond)
			for round := 0; next.Before(end); round++ {
				if delay := time.Until(next); delay > 0 {
					time.Sleep(delay)
				}
				pathIndex := (index + round) % readEndpoints
				method, expectedStatus := "GET", 200
				var body io.Reader
				if mixed && index%5 < 2 {
					method = "POST"
					var payload map[string]any
					if index%5 == 0 {
						pathIndex = readEndpoints + round%4
						payload = map[string]any{"entryId": timerID}
						if round%4 == 0 {
							expectedStatus = 201
							payload = map[string]any{"organizationId": org, "projectId": project, "description": "Capacity timer", "tagIds": []string{}, "ticketIds": []string{}}
						}
					} else {
						pathIndex, expectedStatus = readEndpoints+4, 201
						entryStart := started.Add(3 * time.Hour).Add(time.Duration(round) * 30 * time.Minute)
						payload = map[string]any{"organizationId": org, "projectId": project, "description": "Capacity manual entry", "tagIds": []string{}, "ticketIds": []string{}, "startedAt": entryStart, "endedAt": entryStart.Add(20 * time.Minute)}
					}
					encoded, _ := json.Marshal(payload)
					body = bytes.NewReader(encoded)
				}
				request, _ := http.NewRequest(method, server.URL+paths[pathIndex], body)
				request.AddCookie(&http.Cookie{Name: "timetracker_session", Value: tokens[index]})
				if method == "POST" {
					request.Header.Set("X-CSRF-Token", tokens[index])
					request.Header.Set("Content-Type", "application/json")
				}
				requestStart := time.Now()
				response, err := client.Do(request)
				failed := err != nil
				if response != nil {
					responseBody, readErr := io.ReadAll(response.Body)
					response.Body.Close()
					failed = failed || response.StatusCode != expectedStatus || readErr != nil
					if mixed && index%5 == 0 && round%4 == 0 && !failed {
						var envelope struct {
							Data struct {
								ID uuid.UUID `json:"id"`
							} `json:"data"`
						}
						failed = json.Unmarshal(responseBody, &envelope) != nil || envelope.Data.ID == uuid.Nil
						timerID = envelope.Data.ID
					}
				}
				elapsed := time.Since(requestStart)
				mutex.Lock()
				latencies[pathIndex] = append(latencies[pathIndex], elapsed)
				if failed {
					failures++
				}
				mutex.Unlock()
				next = next.Add(5 * time.Second)
			}
		}(user)
	}
	workers.Wait()
	total := 0
	for index, samples := range latencies {
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		total += len(samples)
		if len(samples) < 20 {
			t.Errorf("insufficient samples for endpoint %s: %d", paths[index], len(samples))
			continue
		}
		p95 := samples[(len(samples)*95+99)/100-1]
		t.Logf("endpoint=%s samples=%d p95=%s", paths[index], len(samples), p95)
		if p95 > 300*time.Millisecond {
			t.Errorf("endpoint %s p95 %s exceeds 300ms", paths[index], p95)
		}
	}
	t.Logf("users=1000 requests=%d errors=%d duration=%s rate=%.1f/s poolMaxConns=10", total, failures, time.Since(start), float64(total)/time.Since(start).Seconds())
	if failures != 0 {
		t.Errorf("capacity workload had %d failed requests", failures)
	}
}
