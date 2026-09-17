package timetracking

import (
	"net/http"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
)

// TodayEntries returns the signed-in user's completed entries overlapping the
// requested local calendar day. When date is omitted, it defaults to today.
func (handler Handler) TodayEntries(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	organizationID, ok := requestedOrganization(writer, request, handler, userID)
	if !ok {
		return
	}
	var timezone string
	if err := handler.Database.QueryRow(request.Context(), `SELECT timezone FROM users WHERE id = $1`, userID).Scan(&timezone); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if requestedDate := request.URL.Query().Get("date"); requestedDate != "" {
		dayStart, err = time.ParseInLocation("2006-01-02", requestedDate, location)
		if err != nil {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Date must use YYYY-MM-DD.")
			return
		}
	}
	dayEnd := dayStart.AddDate(0, 0, 1)
	rows, err := handler.Database.Query(request.Context(), `
		SELECT te.id, p.name, te.started_at, te.ended_at, te.duration_seconds, te.source_type,
		       COALESCE(array_agg(t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
		FROM time_entries te
		JOIN projects p ON p.id = te.project_id
		LEFT JOIN time_entry_tags tet ON tet.time_entry_id = te.id
		LEFT JOIN tags t ON t.id = tet.tag_id
		WHERE te.user_id = $1 AND te.organization_id = $2 AND te.status = 'STOPPED' AND te.started_at < $4 AND te.ended_at > $3
		GROUP BY te.id, p.name
		ORDER BY te.started_at DESC`, userID, organizationID, dayStart, dayEnd)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	type dayEntry struct {
		id                      uuid.UUID
		projectName, sourceType string
		startedAt, endedAt      time.Time
		durationSeconds         int64
		tags                    []string
	}
	loaded := make([]dayEntry, 0)
	for rows.Next() {
		item := dayEntry{tags: make([]string, 0)}
		if err := rows.Scan(&item.id, &item.projectName, &item.startedAt, &item.endedAt, &item.durationSeconds, &item.sourceType, &item.tags); err != nil {
			rows.Close()
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
			return
		}
		loaded = append(loaded, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	rows.Close()
	entries := make([]map[string]any, 0, len(loaded))
	for _, item := range loaded {
		durationSeconds := clippedDuration(item.startedAt, item.endedAt, dayStart, dayEnd)
		if item.sourceType == "TIMER" {
			durationSeconds, err = handler.timerDurationInRange(request, item.id, dayStart, dayEnd)
			if err != nil {
				respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
				return
			}
		}
		entries = append(entries, map[string]any{"id": item.id, "projectName": item.projectName, "startedAt": item.startedAt, "endedAt": item.endedAt, "durationSeconds": durationSeconds, "sourceType": item.sourceType, "tags": item.tags})
	}
	respond(writer, http.StatusOK, entries)
}

func (handler Handler) timerDurationInRange(request *http.Request, entryID uuid.UUID, rangeStart, rangeEnd time.Time) (int64, error) {
	rows, err := handler.Database.Query(request.Context(), `SELECT event_type, occurred_at FROM timer_events WHERE time_entry_id = $1 ORDER BY sequence_number`, entryID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var total int64
	var activeStartedAt *time.Time
	for rows.Next() {
		var eventType string
		var occurredAt time.Time
		if err := rows.Scan(&eventType, &occurredAt); err != nil {
			return 0, err
		}
		if eventType == "START" || eventType == "RESUME" {
			at := occurredAt
			activeStartedAt = &at
		} else if (eventType == "PAUSE" || eventType == "STOP") && activeStartedAt != nil {
			total += clippedDuration(*activeStartedAt, occurredAt, rangeStart, rangeEnd)
			activeStartedAt = nil
		}
	}
	return total, rows.Err()
}

func clippedDuration(startedAt, endedAt, rangeStart, rangeEnd time.Time) int64 {
	if startedAt.Before(rangeStart) {
		startedAt = rangeStart
	}
	if endedAt.After(rangeEnd) {
		endedAt = rangeEnd
	}
	if !endedAt.After(startedAt) {
		return 0
	}
	return int64(endedAt.Sub(startedAt).Seconds())
}

// WeekSummary returns the signed-in user's completed time for the current
// Monday-Sunday week, grouped by the user's local calendar day.
func (handler Handler) WeekSummary(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	organizationID, ok := requestedOrganization(writer, request, handler, userID)
	if !ok {
		return
	}
	location, ok := userLocation(writer, request, handler, userID)
	if !ok {
		return
	}
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	daysSinceMonday := (int(today.Weekday()) + 6) % 7
	weekStart := today.AddDate(0, 0, -daysSinceMonday)
	weekEnd := weekStart.AddDate(0, 0, 7)

	rows, err := handler.Database.Query(request.Context(), `
		SELECT id, source_type, started_at, ended_at
		FROM time_entries
		WHERE user_id = $1 AND organization_id = $2 AND status = 'STOPPED' AND started_at < $4 AND ended_at > $3`, userID, organizationID, weekStart, weekEnd)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the weekly summary.")
		return
	}
	defer rows.Close()
	type summaryEntry struct {
		id         uuid.UUID
		sourceType string
		startedAt  time.Time
		endedAt    time.Time
	}
	entries := make([]summaryEntry, 0)
	totals := make(map[string]int64, 7)
	for rows.Next() {
		var item summaryEntry
		if err := rows.Scan(&item.id, &item.sourceType, &item.startedAt, &item.endedAt); err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the weekly summary.")
			return
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the weekly summary.")
		return
	}
	for _, item := range entries {
		if item.sourceType != "TIMER" {
			allocateSummaryInterval(totals, item.startedAt, item.endedAt, weekStart, weekEnd, location)
			continue
		}
		eventRows, err := handler.Database.Query(request.Context(), `SELECT event_type, occurred_at FROM timer_events WHERE time_entry_id = $1 ORDER BY sequence_number`, item.id)
		if err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the weekly summary.")
			return
		}
		var activeStartedAt *time.Time
		for eventRows.Next() {
			var eventType string
			var occurredAt time.Time
			if err := eventRows.Scan(&eventType, &occurredAt); err != nil {
				eventRows.Close()
				respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the weekly summary.")
				return
			}
			if eventType == "START" || eventType == "RESUME" {
				activeStartedAt = &occurredAt
			} else if (eventType == "PAUSE" || eventType == "STOP") && activeStartedAt != nil {
				allocateSummaryInterval(totals, *activeStartedAt, occurredAt, weekStart, weekEnd, location)
				activeStartedAt = nil
			}
		}
		if err := eventRows.Err(); err != nil {
			eventRows.Close()
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the weekly summary.")
			return
		}
		eventRows.Close()
	}
	days := make([]map[string]any, 0, 7)
	for day := weekStart; day.Before(weekEnd); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		days = append(days, map[string]any{"date": date, "durationSeconds": totals[date]})
	}
	respond(writer, http.StatusOK, map[string]any{"weekStart": weekStart.Format("2006-01-02"), "days": days})
}

func allocateSummaryInterval(totals map[string]int64, startedAt, endedAt, weekStart, weekEnd time.Time, location *time.Location) {
	if !endedAt.After(startedAt) {
		return
	}
	if startedAt.Before(weekStart) {
		startedAt = weekStart
	}
	if endedAt.After(weekEnd) {
		endedAt = weekEnd
	}
	for startedAt.Before(endedAt) {
		local := startedAt.In(location)
		nextDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).AddDate(0, 0, 1)
		segmentEnd := endedAt
		if nextDay.Before(segmentEnd) {
			segmentEnd = nextDay
		}
		totals[local.Format("2006-01-02")] += int64(segmentEnd.Sub(startedAt).Seconds())
		startedAt = segmentEnd
	}
}

func userLocation(writer http.ResponseWriter, request *http.Request, handler Handler, userID uuid.UUID) (*time.Location, bool) {
	var timezone string
	if err := handler.Database.QueryRow(request.Context(), `SELECT timezone FROM users WHERE id = $1`, userID).Scan(&timezone); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return nil, false
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	return location, true
}

func requestedOrganization(writer http.ResponseWriter, request *http.Request, handler Handler, userID uuid.UUID) (uuid.UUID, bool) {
	organizationID, err := uuid.Parse(request.URL.Query().Get("organizationId"))
	if err != nil || organizationID == uuid.Nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "An organization is required.")
		return uuid.Nil, false
	}
	var isMember bool
	if err := handler.Database.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM memberships WHERE organization_id = $1 AND user_id = $2)`, organizationID, userID).Scan(&isMember); err != nil || !isMember {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "You do not have access to this organization.")
		return uuid.Nil, false
	}
	return organizationID, true
}
