package timetracking

import (
	"net/http"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type updateCompletedEntryRequest struct {
	ProjectID uuid.UUID   `json:"projectId"`
	TagIDs    []uuid.UUID `json:"tagIds"`
	StartedAt *time.Time  `json:"startedAt"`
	EndedAt   *time.Time  `json:"endedAt"`
}

type editableCompletedEntry struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	ProjectID      uuid.UUID
	SourceType     string
	StartedAt      time.Time
	EndedAt        time.Time
}

// UpdateCompleted changes a completed entry's project and tags. Manual entries
// may additionally have their start/end timestamps changed. Timer timestamps
// are edited through the dedicated timer-event workflow.
func (handler Handler) UpdateCompleted(writer http.ResponseWriter, request *http.Request) {
	actorID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
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
	var input updateCompletedEntryRequest
	if !decodeJSON(writer, request, &input) || input.ProjectID == uuid.Nil || !validTagIDs(input.TagIDs) || (input.StartedAt == nil) != (input.EndedAt == nil) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a project, valid tags, and both start/end values when changing time.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update the completed entry.")
		return
	}
	defer tx.Rollback(request.Context())
	entry, found, err := loadEditableCompletedEntry(request, tx, organizationID, entryID)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update the completed entry.")
		return
	}
	if !found {
		respondError(writer, http.StatusNotFound, "TIME_ENTRY_NOT_FOUND", "Completed entry not found.")
		return
	}
	if !canManageCompletedEntry(request, tx, organizationID, actorID, entry.UserID) {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "You cannot update this completed entry.")
		return
	}
	if !lockUser(request, tx, entry.UserID) || !validateSelection(request, tx, entry.UserID, organizationID, input.ProjectID, input.TagIDs) {
		respondError(writer, http.StatusForbidden, "INVALID_ENTRY_SELECTION", "Select an assigned project and tags from the same organization.")
		return
	}
	startedAt, endedAt := entry.StartedAt, entry.EndedAt
	if input.StartedAt != nil {
		if entry.SourceType != "MANUAL" {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Edit timer timestamps through timer events.")
			return
		}
		startedAt, endedAt = *input.StartedAt, *input.EndedAt
		if !validManualTimestamp(startedAt) || !validManualTimestamp(endedAt) || !startedAt.Before(endedAt) || startedAt.After(time.Now().UTC()) || endedAt.After(time.Now().UTC()) {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Manual entry times must be minute-precise, ordered, and not in the future.")
			return
		}
		overlaps, err := entryOverlapsExcept(request, tx, entry.UserID, entry.ID, startedAt, endedAt)
		if err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to validate the completed entry.")
			return
		}
		if overlaps {
			respondError(writer, http.StatusConflict, "TIME_ENTRY_OVERLAP", "This entry overlaps an existing time entry.")
			return
		}
	}
	durationSeconds := int64(endedAt.Sub(startedAt).Seconds())
	_, err = tx.Exec(request.Context(), `UPDATE time_entries SET project_id = $1, started_at = $2, ended_at = $3, duration_seconds = $4, updated_at = CURRENT_TIMESTAMP WHERE id = $5`, input.ProjectID, startedAt, endedAt, durationSeconds, entry.ID)
	if isOverlapConstraintViolation(err) {
		respondError(writer, http.StatusConflict, "TIME_ENTRY_OVERLAP", "This entry overlaps an existing time entry.")
		return
	}
	if err != nil || replaceTags(request, tx, entry.ID, input.TagIDs) != nil || auditTimeEntry(request, tx, organizationID, actorID, "TIME_ENTRY_UPDATED", entry.ID) != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update the completed entry.")
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update the completed entry.")
		return
	}
	respond(writer, http.StatusOK, map[string]any{"id": entry.ID, "projectId": input.ProjectID, "tagIds": input.TagIDs, "startedAt": startedAt, "endedAt": endedAt, "durationSeconds": durationSeconds})
}

// DeleteCompleted permanently deletes a completed entry. Database cascades
// remove timer events and tag links, while the audit record is retained.
func (handler Handler) DeleteCompleted(writer http.ResponseWriter, request *http.Request) {
	actorID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
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
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete the completed entry.")
		return
	}
	defer tx.Rollback(request.Context())
	entry, found, err := loadEditableCompletedEntry(request, tx, organizationID, entryID)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete the completed entry.")
		return
	}
	if !found {
		respondError(writer, http.StatusNotFound, "TIME_ENTRY_NOT_FOUND", "Completed entry not found.")
		return
	}
	if !canManageCompletedEntry(request, tx, organizationID, actorID, entry.UserID) || !lockUser(request, tx, entry.UserID) {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "You cannot delete this completed entry.")
		return
	}
	if err := auditTimeEntry(request, tx, organizationID, actorID, "TIME_ENTRY_DELETED", entry.ID); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete the completed entry.")
		return
	}
	if _, err := tx.Exec(request.Context(), `DELETE FROM time_entries WHERE id = $1`, entry.ID); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete the completed entry.")
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete the completed entry.")
		return
	}
	respond(writer, http.StatusOK, map[string]any{"deleted": true})
}

func loadEditableCompletedEntry(request *http.Request, tx pgx.Tx, organizationID, entryID uuid.UUID) (editableCompletedEntry, bool, error) {
	entry := editableCompletedEntry{}
	err := tx.QueryRow(request.Context(), `SELECT id, organization_id, user_id, project_id, source_type, started_at, ended_at FROM time_entries WHERE id = $1 AND organization_id = $2 AND status = 'STOPPED' FOR UPDATE`, entryID, organizationID).Scan(&entry.ID, &entry.OrganizationID, &entry.UserID, &entry.ProjectID, &entry.SourceType, &entry.StartedAt, &entry.EndedAt)
	if err == pgx.ErrNoRows {
		return editableCompletedEntry{}, false, nil
	}
	if err != nil {
		return editableCompletedEntry{}, false, err
	}
	return entry, true, nil
}

func canManageCompletedEntry(request *http.Request, tx pgx.Tx, organizationID, actorID, ownerID uuid.UUID) bool {
	if actorID == ownerID {
		return true
	}
	var role string
	return tx.QueryRow(request.Context(), `SELECT role FROM memberships WHERE organization_id = $1 AND user_id = $2`, organizationID, actorID).Scan(&role) == nil && role == "ADMIN"
}

func validManualTimestamp(value time.Time) bool {
	return value.Second() == 0 && value.Nanosecond() == 0
}
