package timetracking

import (
	"errors"
	"net/http"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type createManualEntryRequest struct {
	OrganizationID  uuid.UUID   `json:"organizationId"`
	ProjectID       uuid.UUID   `json:"projectId"`
	TagIDs          []uuid.UUID `json:"tagIds"`
	StartedAt       time.Time   `json:"startedAt"`
	EndedAt         *time.Time  `json:"endedAt"`
	DurationMinutes *int64      `json:"durationMinutes"`
}

// CreateManual records a completed manual entry. It accepts exactly one of an
// end time or duration in minutes and normalizes both forms to start/end times.
func (handler Handler) CreateManual(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
	if !ok {
		return
	}
	var input createManualEntryRequest
	if !decodeJSON(writer, request, &input) || !validManualInput(writer, input) {
		return
	}
	var endedAt time.Time
	if input.DurationMinutes != nil {
		endedAt = input.StartedAt.Add(time.Duration(*input.DurationMinutes) * time.Minute)
	} else {
		endedAt = *input.EndedAt
	}
	now := time.Now().UTC()
	if !input.StartedAt.Before(endedAt) || input.StartedAt.After(now) || endedAt.After(now) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Start and end must be ordered, positive, and not in the future.")
		return
	}

	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create the manual entry.")
		return
	}
	defer tx.Rollback(request.Context())
	if !lockUser(request, tx, userID) {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create the manual entry.")
		return
	}
	if !validateSelection(request, tx, userID, input.OrganizationID, input.ProjectID, input.TagIDs) {
		respondError(writer, http.StatusForbidden, "INVALID_ENTRY_SELECTION", "Select an assigned project and tags from the same organization.")
		return
	}
	overlaps, err := entryOverlaps(request, tx, userID, input.StartedAt, endedAt)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to validate the manual entry.")
		return
	}
	if overlaps {
		respondError(writer, http.StatusConflict, "TIME_ENTRY_OVERLAP", "This entry overlaps an existing time entry.")
		return
	}

	entryID := uuid.Must(uuid.NewV7())
	durationSeconds := int64(endedAt.Sub(input.StartedAt).Seconds())
	_, err = tx.Exec(request.Context(), `
		INSERT INTO time_entries (id, organization_id, user_id, project_id, source_type, status, started_at, ended_at, duration_seconds)
		VALUES ($1, $2, $3, $4, 'MANUAL', 'STOPPED', $5, $6, $7)`, entryID, input.OrganizationID, userID, input.ProjectID, input.StartedAt, endedAt, durationSeconds)
	if isOverlapConstraintViolation(err) {
		respondError(writer, http.StatusConflict, "TIME_ENTRY_OVERLAP", "This entry overlaps an existing time entry.")
		return
	}
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create the manual entry.")
		return
	}
	if err := replaceTags(request, tx, entryID, input.TagIDs); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create the manual entry.")
		return
	}
	if err := auditTimeEntry(request, tx, input.OrganizationID, userID, "TIME_ENTRY_CREATED", entryID); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create the manual entry.")
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create the manual entry.")
		return
	}
	respond(writer, http.StatusCreated, map[string]any{"id": entryID, "organizationId": input.OrganizationID, "projectId": input.ProjectID, "tagIds": input.TagIDs, "sourceType": "MANUAL", "status": "STOPPED", "startedAt": input.StartedAt, "endedAt": endedAt, "durationSeconds": durationSeconds})
}

func validManualInput(writer http.ResponseWriter, input createManualEntryRequest) bool {
	if input.OrganizationID == uuid.Nil || input.ProjectID == uuid.Nil || input.StartedAt.IsZero() || !validTagIDs(input.TagIDs) || (input.EndedAt == nil && input.DurationMinutes == nil) || (input.EndedAt != nil && input.DurationMinutes != nil) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide an organization, assigned project, tags, start time, and either end time or duration.")
		return false
	}
	if input.StartedAt.Second() != 0 || input.StartedAt.Nanosecond() != 0 || (input.EndedAt != nil && (input.EndedAt.Second() != 0 || input.EndedAt.Nanosecond() != 0)) || (input.DurationMinutes != nil && *input.DurationMinutes <= 0) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Manual entries must use minute precision and a positive duration.")
		return false
	}
	return true
}

func entryOverlaps(request *http.Request, tx pgx.Tx, userID uuid.UUID, startedAt, endedAt time.Time) (bool, error) {
	return entryOverlapsExcept(request, tx, userID, uuid.Nil, startedAt, endedAt)
}

func entryOverlapsExcept(request *http.Request, tx pgx.Tx, userID, excludedEntryID uuid.UUID, startedAt, endedAt time.Time) (bool, error) {
	var entryID uuid.UUID
	err := tx.QueryRow(request.Context(), `
		SELECT id FROM time_entries
		WHERE user_id = $1 AND id <> $2 AND started_at < $4 AND (ended_at IS NULL OR ended_at > $3)
		LIMIT 1 FOR UPDATE`, userID, excludedEntryID, startedAt, endedAt).Scan(&entryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func auditTimeEntry(request *http.Request, tx pgx.Tx, organizationID, actorID uuid.UUID, action string, entryID uuid.UUID) error {
	_, err := tx.Exec(request.Context(), `INSERT INTO audit_logs (id, organization_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, $4, 'TIME_ENTRY', $5)`, uuid.Must(uuid.NewV7()), organizationID, actorID, action, entryID)
	return err
}

func isOverlapConstraintViolation(err error) bool {
	var databaseError *pgconn.PgError
	return err != nil && errors.As(err, &databaseError) && databaseError.Code == "23P01"
}
