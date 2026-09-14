package timetracking

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const defaultCompletedEntryPageSize = 25
const maximumCompletedEntryPageSize = 100

type completedEntry struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	UserID          uuid.UUID
	UserName        string
	ProjectID       uuid.UUID
	ProjectName     string
	SourceType      string
	StartedAt       time.Time
	EndedAt         time.Time
	DurationSeconds int64
	TagIDs          []uuid.UUID
	TagNames        []string
}

// ListCompleted returns an authorized, paginated list of completed entries in
// an organization. Members can list only their own entries; Admins can select
// any member through the optional userId query parameter.
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
	limit, offset, ok := completedEntryPagination(writer, request)
	if !ok {
		return
	}
	var total int
	if err := handler.Database.QueryRow(request.Context(), `SELECT COUNT(*) FROM time_entries WHERE organization_id = $1 AND user_id = $2 AND status = 'STOPPED'`, organizationID, targetUserID).Scan(&total); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load completed entries.")
		return
	}
	rows, err := handler.Database.Query(request.Context(), completedEntryQuery+` ORDER BY te.started_at DESC LIMIT $3 OFFSET $4`, organizationID, targetUserID, limit, offset)
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
	respond(writer, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
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

const completedEntryQuery = `
	SELECT te.id, te.organization_id, te.user_id, u.name, te.project_id, p.name, te.source_type,
	       te.started_at, te.ended_at, te.duration_seconds,
	       COALESCE(array_agg(t.id) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::uuid[]),
	       COALESCE(array_agg(t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
	FROM time_entries te
	JOIN users u ON u.id = te.user_id
	JOIN projects p ON p.id = te.project_id
	LEFT JOIN time_entry_tags tet ON tet.time_entry_id = te.id
	LEFT JOIN tags t ON t.id = tet.tag_id
	WHERE te.organization_id = $1 AND te.user_id = $2 AND te.status = 'STOPPED'
	GROUP BY te.id, u.name, p.name`

type completedEntryScanner interface{ Scan(...any) error }

func scanCompletedEntry(scanner completedEntryScanner) (completedEntry, error) {
	entry := completedEntry{TagIDs: make([]uuid.UUID, 0), TagNames: make([]string, 0)}
	err := scanner.Scan(&entry.ID, &entry.OrganizationID, &entry.UserID, &entry.UserName, &entry.ProjectID, &entry.ProjectName, &entry.SourceType, &entry.StartedAt, &entry.EndedAt, &entry.DurationSeconds, &entry.TagIDs, &entry.TagNames)
	return entry, err
}

func (handler Handler) loadCompletedEntry(request *http.Request, organizationID, entryID uuid.UUID) (completedEntry, bool, error) {
	entry := completedEntry{TagIDs: make([]uuid.UUID, 0), TagNames: make([]string, 0)}
	err := handler.Database.QueryRow(request.Context(), `
		SELECT te.id, te.organization_id, te.user_id, u.name, te.project_id, p.name, te.source_type,
		       te.started_at, te.ended_at, te.duration_seconds,
		       COALESCE(array_agg(t.id) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::uuid[]),
		       COALESCE(array_agg(t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
		FROM time_entries te
		JOIN users u ON u.id = te.user_id
		JOIN projects p ON p.id = te.project_id
		LEFT JOIN time_entry_tags tet ON tet.time_entry_id = te.id
		LEFT JOIN tags t ON t.id = tet.tag_id
		WHERE te.organization_id = $1 AND te.id = $2 AND te.status = 'STOPPED'
		GROUP BY te.id, u.name, p.name`, organizationID, entryID).Scan(&entry.ID, &entry.OrganizationID, &entry.UserID, &entry.UserName, &entry.ProjectID, &entry.ProjectName, &entry.SourceType, &entry.StartedAt, &entry.EndedAt, &entry.DurationSeconds, &entry.TagIDs, &entry.TagNames)
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
	return map[string]any{"id": entry.ID, "organizationId": entry.OrganizationID, "userId": entry.UserID, "userName": entry.UserName, "projectId": entry.ProjectID, "projectName": entry.ProjectName, "sourceType": entry.SourceType, "startedAt": entry.StartedAt, "endedAt": entry.EndedAt, "durationSeconds": entry.DurationSeconds, "tags": tags}
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
