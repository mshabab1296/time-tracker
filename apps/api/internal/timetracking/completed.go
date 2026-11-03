package timetracking

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const defaultCompletedEntryPageSize = 25
const maximumCompletedEntryPageSize = 100

type completedEntry struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	UserID           uuid.UUID
	UserName         string
	Timezone         string
	ProjectID        uuid.UUID
	ProjectName      string
	TicketIDs        []uuid.UUID
	TicketReferences []string
	TicketTitles     []string
	Description      string
	SourceType       string
	StartedAt        time.Time
	EndedAt          time.Time
	DurationSeconds  int64
	TagIDs           []uuid.UUID
	TagNames         []string
}

// ListCompleted returns an authorized, paginated list of completed entries in
// an organization. Members can list only their own entries; Admins can select
// any member or all members through the optional userId query parameter.
func (handler Handler) ListCompleted(writer http.ResponseWriter, request *http.Request) {
	actorID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	organizationID, ok := requestedOrganization(writer, request, handler, actorID)
	if !ok {
		return
	}
	role, err := organizationRole(request, handler, organizationID, actorID)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	targetUserID, ok := requestedEntryUser(writer, request, actorID, role)
	if !ok {
		return
	}
	allMembers := targetUserID == uuid.Nil
	var userFilter any = targetUserID
	if allMembers {
		userFilter = nil
	}
	limit, offset, ok := completedEntryPagination(writer, request)
	if !ok {
		return
	}
	filters, ok := parseCompletedEntryFilters(writer, request)
	if !ok {
		return
	}
	var timezone string
	if !allMembers {
		if err := handler.Database.QueryRow(request.Context(), `SELECT COALESCE((SELECT timezone FROM users WHERE id = $1), 'UTC')`, targetUserID).Scan(&timezone); err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
			return
		}
	}
	var total int
	if err := handler.Database.QueryRow(request.Context(), completedEntryCountQuery, organizationID, userFilter, filters.ProjectID, filters.Date, filters.Search).Scan(&total); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	rows, err := handler.Database.Query(request.Context(), completedEntryQuery+` ORDER BY te.started_at DESC, te.id DESC LIMIT $6 OFFSET $7`, organizationID, userFilter, filters.ProjectID, filters.Date, filters.Search, limit, offset)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		entry, err := scanCompletedEntry(rows)
		if err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
			return
		}
		items = append(items, completedEntryResponse(entry))
	}
	if err := rows.Err(); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	respond(writer, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset, "timezone": timezone})
}

// CompletedDetail returns one completed entry, including its immutable timer
// event history when it originated from the timer.
func (handler Handler) CompletedDetail(writer http.ResponseWriter, request *http.Request) {
	actorID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	organizationID, ok := requestedOrganization(writer, request, handler, actorID)
	if !ok {
		return
	}
	entryID, err := uuid.Parse(request.PathValue("entryID"))
	if err != nil || entryID == uuid.Nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A completed entry identifier is required.")
		return
	}
	entry, found, err := handler.loadCompletedEntry(request, organizationID, entryID)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the completed entry.")
		return
	}
	if !found {
		respondError(writer, http.StatusNotFound, "TIME_ENTRY_NOT_FOUND", "Completed entry not found.")
		return
	}
	if entry.UserID != actorID {
		role, err := organizationRole(request, handler, organizationID, actorID)
		if err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the completed entry.")
			return
		}
		if role != "ADMIN" {
			respondError(writer, http.StatusForbidden, "FORBIDDEN", "You cannot view this completed entry.")
			return
		}
	}
	response := completedEntryResponse(entry)
	events := make([]map[string]any, 0)
	if entry.SourceType == "TIMER" {
		rows, err := handler.Database.Query(request.Context(), `SELECT event_type, occurred_at FROM timer_events WHERE time_entry_id = $1 ORDER BY sequence_number`, entry.ID)
		if err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the completed entry.")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var eventType string
			var occurredAt time.Time
			if err := rows.Scan(&eventType, &occurredAt); err != nil {
				respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the completed entry.")
				return
			}
			events = append(events, map[string]any{"type": eventType, "occurredAt": occurredAt})
		}
		if err := rows.Err(); err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the completed entry.")
			return
		}
	}
	response["events"] = events
	respond(writer, http.StatusOK, response)
}

const completedEntryFilterSQL = `
	AND ($3::uuid IS NULL OR te.project_id = $3::uuid)
	AND ($4::date IS NULL OR ` + completedEntryDayOverlapSQL + `)
	AND ($5::text = ''
		OR position(lower($5::text) in lower(te.description)) > 0
		OR position(lower($5::text) in lower(p.name)) > 0
		OR EXISTS (
			SELECT 1 FROM time_entry_tickets search_link
			JOIN tickets search_ticket ON search_ticket.id = search_link.ticket_id
			WHERE search_link.time_entry_id = te.id
			AND (position(lower($5::text) in lower(search_ticket.reference)) > 0
				OR position(lower($5::text) in lower(search_ticket.title)) > 0)
		)
		OR EXISTS (
			SELECT 1 FROM time_entry_tags search_link
			JOIN tags search_tag ON search_tag.id = search_link.tag_id
			WHERE search_link.time_entry_id = te.id
			AND position(lower($5::text) in lower(search_tag.name)) > 0
		))`

// Cast to timestamp without time zone before AT TIME ZONE. Applying it
// directly to a date first casts the date to timestamptz using the DB session
// timezone, shifting the member's local day boundaries.
const completedEntryDayOverlapSQL = `(
		te.started_at < ((($4::date + 1)::timestamp) AT TIME ZONE u.timezone)
		AND te.ended_at > (($4::date)::timestamp AT TIME ZONE u.timezone)
	)`

const completedEntryCountQuery = `
	SELECT COUNT(*) FROM time_entries te
	JOIN users u ON u.id = te.user_id
	JOIN projects p ON p.id = te.project_id
	WHERE te.organization_id = $1 AND ($2::uuid IS NULL OR te.user_id = $2::uuid) AND te.status = 'STOPPED'` + completedEntryFilterSQL

const completedEntryQuery = `
	SELECT te.id, te.organization_id, te.user_id, u.name, u.timezone, te.project_id, p.name, te.description, te.source_type,
	       te.started_at, te.ended_at, te.duration_seconds,
	       ARRAY(SELECT ticket.id FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
	       ARRAY(SELECT ticket.reference FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
	       ARRAY(SELECT ticket.title FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
	       COALESCE(array_agg(t.id) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::uuid[]),
	       COALESCE(array_agg(t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
	FROM time_entries te
	JOIN users u ON u.id = te.user_id
	JOIN projects p ON p.id = te.project_id
	LEFT JOIN time_entry_tags tet ON tet.time_entry_id = te.id
	LEFT JOIN tags t ON t.id = tet.tag_id
	WHERE te.organization_id = $1 AND ($2::uuid IS NULL OR te.user_id = $2::uuid) AND te.status = 'STOPPED'` + completedEntryFilterSQL + `
	GROUP BY te.id, u.name, u.timezone, p.name`

type completedEntryFilters struct {
	ProjectID *uuid.UUID
	Date      *string
	Search    string
}

func parseCompletedEntryFilters(writer http.ResponseWriter, request *http.Request) (completedEntryFilters, bool) {
	var filters completedEntryFilters
	if rawProjectID := request.URL.Query().Get("projectId"); rawProjectID != "" {
		projectID, err := uuid.Parse(rawProjectID)
		if err != nil || projectID == uuid.Nil {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A valid project identifier is required.")
			return completedEntryFilters{}, false
		}
		filters.ProjectID = &projectID
	}
	if rawDate := request.URL.Query().Get("date"); rawDate != "" {
		if _, err := time.Parse("2006-01-02", rawDate); err != nil {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Date must use YYYY-MM-DD.")
			return completedEntryFilters{}, false
		}
		filters.Date = &rawDate
	}
	filters.Search = strings.TrimSpace(request.URL.Query().Get("search"))
	if utf8.RuneCountInString(filters.Search) > 200 {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Search must be at most 200 characters.")
		return completedEntryFilters{}, false
	}
	return filters, true
}

type completedEntryScanner interface{ Scan(...any) error }

func scanCompletedEntry(scanner completedEntryScanner) (completedEntry, error) {
	entry := completedEntry{TagIDs: make([]uuid.UUID, 0), TagNames: make([]string, 0)}
	err := scanner.Scan(&entry.ID, &entry.OrganizationID, &entry.UserID, &entry.UserName, &entry.Timezone, &entry.ProjectID, &entry.ProjectName, &entry.Description, &entry.SourceType, &entry.StartedAt, &entry.EndedAt, &entry.DurationSeconds, &entry.TicketIDs, &entry.TicketReferences, &entry.TicketTitles, &entry.TagIDs, &entry.TagNames)
	return entry, err
}

func (handler Handler) loadCompletedEntry(request *http.Request, organizationID, entryID uuid.UUID) (completedEntry, bool, error) {
	entry := completedEntry{TagIDs: make([]uuid.UUID, 0), TagNames: make([]string, 0)}
	err := handler.Database.QueryRow(request.Context(), `
		SELECT te.id, te.organization_id, te.user_id, u.name, u.timezone, te.project_id, p.name, te.description, te.source_type,
		       te.started_at, te.ended_at, te.duration_seconds,
		       ARRAY(SELECT ticket.id FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
		       ARRAY(SELECT ticket.reference FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
		       ARRAY(SELECT ticket.title FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
		       COALESCE(array_agg(t.id) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::uuid[]),
		       COALESCE(array_agg(t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
		FROM time_entries te
		JOIN users u ON u.id = te.user_id
		JOIN projects p ON p.id = te.project_id
		LEFT JOIN time_entry_tags tet ON tet.time_entry_id = te.id
		LEFT JOIN tags t ON t.id = tet.tag_id
		WHERE te.organization_id = $1 AND te.id = $2 AND te.status = 'STOPPED'
		GROUP BY te.id, u.name, u.timezone, p.name`, organizationID, entryID).Scan(&entry.ID, &entry.OrganizationID, &entry.UserID, &entry.UserName, &entry.Timezone, &entry.ProjectID, &entry.ProjectName, &entry.Description, &entry.SourceType, &entry.StartedAt, &entry.EndedAt, &entry.DurationSeconds, &entry.TicketIDs, &entry.TicketReferences, &entry.TicketTitles, &entry.TagIDs, &entry.TagNames)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return completedEntry{}, false, nil
		}
		return completedEntry{}, false, err
	}
	return entry, true, nil
}

func completedEntryResponse(entry completedEntry) map[string]any {
	tags := make([]map[string]any, 0, len(entry.TagIDs))
	for index, tagID := range entry.TagIDs {
		tags = append(tags, map[string]any{"id": tagID, "name": entry.TagNames[index]})
	}
	tickets := make([]map[string]any, 0, len(entry.TicketIDs))
	for index, id := range entry.TicketIDs {
		tickets = append(tickets, map[string]any{"id": id, "reference": entry.TicketReferences[index], "title": entry.TicketTitles[index]})
	}
	return map[string]any{"id": entry.ID, "organizationId": entry.OrganizationID, "userId": entry.UserID, "userName": entry.UserName, "timezone": entry.Timezone, "projectId": entry.ProjectID, "projectName": entry.ProjectName, "description": entry.Description, "sourceType": entry.SourceType, "startedAt": entry.StartedAt, "endedAt": entry.EndedAt, "durationSeconds": entry.DurationSeconds, "tickets": tickets, "tags": tags}
}

func organizationRole(request *http.Request, handler Handler, organizationID, userID uuid.UUID) (string, error) {
	var role string
	err := handler.Database.QueryRow(request.Context(), `SELECT role FROM memberships WHERE organization_id = $1 AND user_id = $2`, organizationID, userID).Scan(&role)
	return role, err
}

func requestedEntryUser(writer http.ResponseWriter, request *http.Request, actorID uuid.UUID, role string) (uuid.UUID, bool) {
	rawUserID := request.URL.Query().Get("userId")
	if rawUserID == "" {
		return actorID, true
	}
	if rawUserID == "all" {
		if role != "ADMIN" {
			respondError(writer, http.StatusForbidden, "FORBIDDEN", "You cannot view another member's completed entries.")
			return uuid.Nil, false
		}
		return uuid.Nil, true
	}
	targetUserID, err := uuid.Parse(rawUserID)
	if err != nil || targetUserID == uuid.Nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A valid user identifier is required.")
		return uuid.Nil, false
	}
	if targetUserID != actorID && role != "ADMIN" {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "You cannot view another member's completed entries.")
		return uuid.Nil, false
	}
	return targetUserID, true
}

func completedEntryPagination(writer http.ResponseWriter, request *http.Request) (int, int, bool) {
	limit, offset := defaultCompletedEntryPageSize, 0
	if rawLimit := request.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > maximumCompletedEntryPageSize {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Limit must be between 1 and 100.")
			return 0, 0, false
		}
		limit = parsed
	}
	if rawOffset := request.URL.Query().Get("offset"); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Offset must be zero or greater.")
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}
