package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maximumTagsPerEntry = 50

type Handler struct{ Database *pgxpool.Pool }

type startRequest struct {
	OrganizationID uuid.UUID   `json:"organizationId"`
	ProjectID      uuid.UUID   `json:"projectId"`
	TagIDs         []uuid.UUID `json:"tagIds"`
}

type actionRequest struct {
	EntryID uuid.UUID `json:"entryId"`
}

type updateActiveRequest struct {
	EntryID   uuid.UUID   `json:"entryId"`
	ProjectID uuid.UUID   `json:"projectId"`
	TagIDs    []uuid.UUID `json:"tagIds"`
}

type timerEvent struct {
	Type string
	At   time.Time
}

type entry struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	UserID          uuid.UUID
	ProjectID       uuid.UUID
	Status          string
	StartedAt       time.Time
	EndedAt         *time.Time
	DurationSeconds int64
	TagIDs          []uuid.UUID
	Events          []timerEvent
}

func (handler Handler) Start(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
	if !ok {
		return
	}
	var input startRequest
	if !decodeJSON(writer, request, &input) || !validStartInput(writer, input) {
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	defer tx.Rollback(request.Context())
	if !lockUser(request, tx, userID) {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	active, found, err := loadActive(request, tx, userID, true)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	if found {
		if active.OrganizationID == input.OrganizationID && active.ProjectID == input.ProjectID && sameTags(active.TagIDs, input.TagIDs) {
			respondEntry(writer, http.StatusOK, active, time.Now().UTC())
			return
		}
		respondError(writer, http.StatusConflict, "ACTIVE_TIMER_EXISTS", "Stop the active timer before starting another one.")
		return
	}
	if !validateSelection(request, tx, userID, input.OrganizationID, input.ProjectID, input.TagIDs) {
		respondError(writer, http.StatusForbidden, "INVALID_TIMER_SELECTION", "Select an accessible project and tags from the same organization.")
		return
	}
	entryID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	now := time.Now().UTC()
	_, err = tx.Exec(request.Context(), `INSERT INTO time_entries (id, organization_id, user_id, project_id, source_type, status, started_at) VALUES ($1, $2, $3, $4, 'TIMER', 'RUNNING', $5)`, entryID, input.OrganizationID, userID, input.ProjectID, now)
	if uniqueViolation(err) {
		respondError(writer, http.StatusConflict, "ACTIVE_TIMER_EXISTS", "Stop the active timer before starting another one.")
		return
	}
	if err == nil {
		_, err = tx.Exec(request.Context(), `INSERT INTO timer_events (id, time_entry_id, event_type, occurred_at, sequence_number) VALUES ($1, $2, 'START', $3, 1)`, eventID, entryID, now)
	}
	if err == nil {
		err = replaceTags(request, tx, entryID, input.TagIDs)
	}
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to start timer.")
		return
	}
	respondEntry(writer, http.StatusCreated, entry{ID: entryID, OrganizationID: input.OrganizationID, UserID: userID, ProjectID: input.ProjectID, Status: "RUNNING", StartedAt: now, TagIDs: input.TagIDs, Events: []timerEvent{{Type: "START", At: now}}}, now)
}

func (handler Handler) Active(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	organizationID, ok := requestedOrganization(writer, request, handler, userID)
	if !ok {
		return
	}
	active, found, err := loadActiveInOrganization(request, handler.Database, userID, organizationID)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load active timer.")
		return
	}
	if !found {
		respond(writer, http.StatusOK, nil)
		return
	}
	respondEntry(writer, http.StatusOK, active, time.Now().UTC())
}

func (handler Handler) Pause(writer http.ResponseWriter, request *http.Request) {
	handler.transition(writer, request, "PAUSE")
}
func (handler Handler) Resume(writer http.ResponseWriter, request *http.Request) {
	handler.transition(writer, request, "RESUME")
}
func (handler Handler) Stop(writer http.ResponseWriter, request *http.Request) {
	handler.transition(writer, request, "STOP")
}

func (handler Handler) transition(writer http.ResponseWriter, request *http.Request, action string) {
	userID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
	if !ok {
		return
	}
	var input actionRequest
	if !decodeJSON(writer, request, &input) || input.EntryID == uuid.Nil {
		if input.EntryID == uuid.Nil {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A timer entry identifier is required.")
		}
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	defer tx.Rollback(request.Context())
	if !lockUser(request, tx, userID) {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	current, found, err := loadEntry(request, tx, input.EntryID, userID, true)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	if !found {
		respondError(writer, http.StatusNotFound, "TIMER_NOT_FOUND", "Timer entry not found.")
		return
	}
	if action == "PAUSE" && current.Status == "PAUSED" || action == "RESUME" && current.Status == "RUNNING" || action == "STOP" && current.Status == "STOPPED" {
		respondEntry(writer, http.StatusOK, current, time.Now().UTC())
		return
	}
	if (action == "PAUSE" && current.Status != "RUNNING") || (action == "RESUME" && current.Status != "PAUSED") || (action == "STOP" && current.Status != "RUNNING" && current.Status != "PAUSED") {
		respondError(writer, http.StatusConflict, "INVALID_TIMER_STATE", "This timer action is not valid for the current state.")
		return
	}
	now := time.Now().UTC()
	eventID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	_, err = tx.Exec(request.Context(), `INSERT INTO timer_events (id, time_entry_id, event_type, occurred_at, sequence_number) VALUES ($1, $2, $3, $4, $5)`, eventID, current.ID, action, now, len(current.Events)+1)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	current.Events = append(current.Events, timerEvent{Type: action, At: now})
	if action == "STOP" {
		duration, valid := duration(current.Events, now)
		if !valid {
			respondError(writer, http.StatusConflict, "INVALID_TIMER_STATE", "The timer event history is invalid.")
			return
		}
		_, err = tx.Exec(request.Context(), `UPDATE time_entries SET status = 'STOPPED', ended_at = $1, duration_seconds = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`, now, duration, current.ID)
		current.Status, current.EndedAt, current.DurationSeconds = "STOPPED", &now, duration
	} else {
		status := map[string]string{"PAUSE": "PAUSED", "RESUME": "RUNNING"}[action]
		_, err = tx.Exec(request.Context(), `UPDATE time_entries SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, status, current.ID)
		current.Status = status
	}
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	respondEntry(writer, http.StatusOK, current, now)
}

func (handler Handler) UpdateActive(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
	if !ok {
		return
	}
	var input updateActiveRequest
	if !decodeJSON(writer, request, &input) || input.EntryID == uuid.Nil || input.ProjectID == uuid.Nil || !validTagIDs(input.TagIDs) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a timer entry, project, and up to 50 unique tags.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	defer tx.Rollback(request.Context())
	if !lockUser(request, tx, userID) {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	current, found, err := loadEntry(request, tx, input.EntryID, userID, true)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	if !found {
		respondError(writer, http.StatusNotFound, "TIMER_NOT_FOUND", "Timer entry not found.")
		return
	}
	if current.Status == "STOPPED" {
		respondError(writer, http.StatusConflict, "INVALID_TIMER_STATE", "Only an active timer can be updated here.")
		return
	}
	if !validateSelection(request, tx, userID, current.OrganizationID, input.ProjectID, input.TagIDs) {
		respondError(writer, http.StatusForbidden, "INVALID_TIMER_SELECTION", "Select an accessible project and tags from the same organization.")
		return
	}
	_, err = tx.Exec(request.Context(), `UPDATE time_entries SET project_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, input.ProjectID, current.ID)
	if err == nil {
		err = replaceTags(request, tx, current.ID, input.TagIDs)
	}
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer.")
		return
	}
	current.ProjectID, current.TagIDs = input.ProjectID, input.TagIDs
	respondEntry(writer, http.StatusOK, current, time.Now().UTC())
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadActive(request *http.Request, database querier, userID uuid.UUID, forUpdate bool) (entry, bool, error) {
	query := `SELECT id FROM time_entries WHERE user_id = $1 AND source_type = 'TIMER' AND status IN ('RUNNING', 'PAUSED')`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var id uuid.UUID
	err := database.QueryRow(request.Context(), query, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return entry{}, false, nil
	}
	if err != nil {
		return entry{}, false, err
	}
	return loadEntry(request, database, id, userID, forUpdate)
}

func loadActiveInOrganization(request *http.Request, database querier, userID, organizationID uuid.UUID) (entry, bool, error) {
	var id uuid.UUID
	err := database.QueryRow(request.Context(), `SELECT id FROM time_entries WHERE user_id = $1 AND organization_id = $2 AND source_type = 'TIMER' AND status IN ('RUNNING', 'PAUSED')`, userID, organizationID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return entry{}, false, nil
	}
	if err != nil {
		return entry{}, false, err
	}
	return loadEntry(request, database, id, userID, false)
}

func loadEntry(request *http.Request, database querier, entryID, userID uuid.UUID, forUpdate bool) (entry, bool, error) {
	query := `SELECT id, organization_id, user_id, project_id, status, started_at, ended_at, duration_seconds FROM time_entries WHERE id = $1 AND user_id = $2 AND source_type = 'TIMER'`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var result entry
	err := database.QueryRow(request.Context(), query, entryID, userID).Scan(&result.ID, &result.OrganizationID, &result.UserID, &result.ProjectID, &result.Status, &result.StartedAt, &result.EndedAt, &result.DurationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return entry{}, false, nil
	}
	if err != nil {
		return entry{}, false, err
	}
	rows, err := database.Query(request.Context(), `SELECT event_type, occurred_at FROM timer_events WHERE time_entry_id = $1 ORDER BY sequence_number`, result.ID)
	if err != nil {
		return entry{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var event timerEvent
		if err = rows.Scan(&event.Type, &event.At); err != nil {
			return entry{}, false, err
		}
		result.Events = append(result.Events, event)
	}
	if err = rows.Err(); err != nil {
		return entry{}, false, err
	}
	tagRows, err := database.Query(request.Context(), `SELECT tag_id FROM time_entry_tags WHERE time_entry_id = $1 ORDER BY tag_id`, result.ID)
	if err != nil {
		return entry{}, false, err
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var id uuid.UUID
		if err = tagRows.Scan(&id); err != nil {
			return entry{}, false, err
		}
		result.TagIDs = append(result.TagIDs, id)
	}
	return result, true, tagRows.Err()
}

func validateSelection(request *http.Request, tx pgx.Tx, userID, organizationID, projectID uuid.UUID, tagIDs []uuid.UUID) bool {
	var allowed bool
	err := tx.QueryRow(request.Context(), `SELECT EXISTS (
		SELECT 1 FROM memberships m JOIN projects p ON p.organization_id = m.organization_id
		WHERE m.organization_id = $1 AND m.user_id = $2 AND p.id = $3
		AND (m.role = 'ADMIN' OR EXISTS (SELECT 1 FROM project_assignments pa WHERE pa.project_id = p.id AND pa.user_id = m.user_id))
	)`, organizationID, userID, projectID).Scan(&allowed)
	if err != nil || !allowed {
		return false
	}
	for _, tagID := range tagIDs {
		var found bool
		if tx.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM tags WHERE id = $1 AND organization_id = $2)`, tagID, organizationID).Scan(&found) != nil || !found {
			return false
		}
	}
	return true
}

func replaceTags(request *http.Request, tx pgx.Tx, entryID uuid.UUID, tagIDs []uuid.UUID) error {
	if _, err := tx.Exec(request.Context(), `DELETE FROM time_entry_tags WHERE time_entry_id = $1`, entryID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(request.Context(), `INSERT INTO time_entry_tags (time_entry_id, tag_id) VALUES ($1, $2)`, entryID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func lockUser(request *http.Request, tx pgx.Tx, userID uuid.UUID) bool {
	_, err := tx.Exec(request.Context(), `SELECT pg_advisory_xact_lock(hashtext($1))`, userID.String())
	return err == nil
}

func duration(events []timerEvent, now time.Time) (int64, bool) {
	var runningAt *time.Time
	var total time.Duration
	for index, event := range events {
		if event.At.After(now) || (index > 0 && event.At.Before(events[index-1].At)) {
			return 0, false
		}
		switch event.Type {
		case "START", "RESUME":
			if runningAt != nil {
				return 0, false
			}
			at := event.At
			runningAt = &at
		case "PAUSE", "STOP":
			if runningAt == nil {
				return 0, false
			}
			total += event.At.Sub(*runningAt)
			runningAt = nil
		default:
			return 0, false
		}
	}
	if runningAt != nil {
		total += now.Sub(*runningAt)
	}
	return int64(total.Seconds()), true
}

func respondEntry(writer http.ResponseWriter, status int, value entry, now time.Time) {
	durationSeconds := value.DurationSeconds
	if value.Status != "STOPPED" {
		durationSeconds, _ = duration(value.Events, now)
	}
	events := make([]map[string]any, 0, len(value.Events))
	for _, event := range value.Events {
		events = append(events, map[string]any{"type": event.Type, "occurredAt": event.At})
	}
	tags := make([]uuid.UUID, 0, len(value.TagIDs))
	tags = append(tags, value.TagIDs...)
	respond(writer, status, map[string]any{"id": value.ID, "organizationId": value.OrganizationID, "projectId": value.ProjectID, "tagIds": tags, "status": value.Status, "startedAt": value.StartedAt, "endedAt": value.EndedAt, "durationSeconds": durationSeconds, "events": events})
}

func validStartInput(writer http.ResponseWriter, input startRequest) bool {
	if input.OrganizationID == uuid.Nil || input.ProjectID == uuid.Nil || !validTagIDs(input.TagIDs) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide an organization, project, and up to 50 unique tags.")
		return false
	}
	return true
}
func validTagIDs(ids []uuid.UUID) bool {
	if len(ids) > maximumTagsPerEntry {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
func sameTags(left, right []uuid.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	found := make(map[uuid.UUID]struct{}, len(left))
	for _, id := range left {
		found[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := found[id]; !ok {
			return false
		}
	}
	return true
}
func uniqueViolation(err error) bool {
	var databaseError *pgconn.PgError
	return err != nil && errors.As(err, &databaseError) && databaseError.Code == "23505"
}
func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is invalid.")
		return false
	}
	return true
}
func respond(writer http.ResponseWriter, status int, data any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"data": data, "error": nil, "meta": map[string]any{}})
}
func respondError(writer http.ResponseWriter, status int, code, message string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"data": nil, "error": map[string]string{"code": code, "message": message}, "meta": map[string]any{}})
}
