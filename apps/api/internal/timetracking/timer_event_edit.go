package timetracking

import (
	"net/http"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maximumCompletedTimerEvents = 102

type updateTimerEventsRequest struct {
	Events []struct {
		Type       string    `json:"type"`
		OccurredAt time.Time `json:"occurredAt"`
	} `json:"events"`
}

// UpdateTimerEvents replaces the complete event history for a stopped timer.
// Accepting the full sequence makes pair insertion/removal atomic and allows a
// single validation pass before any persisted history is changed.
func (handler Handler) UpdateTimerEvents(writer http.ResponseWriter, request *http.Request) {
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
	var input updateTimerEventsRequest
	if !decodeJSON(writer, request, &input) {
		return
	}
	events := make([]timerEvent, 0, len(input.Events))
	for _, event := range input.Events {
		events = append(events, timerEvent{Type: event.Type, At: event.OccurredAt})
	}
	durationSeconds, valid := validateCompletedTimerEvents(events, time.Now().UTC())
	if !valid {
		respondError(writer, http.StatusBadRequest, "INVALID_TIMER_EVENTS", "Timer events must be chronological and follow START, optional PAUSE/RESUME pairs, then STOP.")
		return
	}

	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	defer tx.Rollback(request.Context())
	entry, found, err := loadEditableCompletedEntry(request, tx, organizationID, entryID)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	if !found {
		respondError(writer, http.StatusNotFound, "TIME_ENTRY_NOT_FOUND", "Completed entry not found.")
		return
	}
	if entry.SourceType != "TIMER" {
		respondError(writer, http.StatusBadRequest, "INVALID_ENTRY_TYPE", "Only timer-generated entries have timer events.")
		return
	}
	if !canManageCompletedEntry(request, tx, organizationID, actorID, entry.UserID) {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "You cannot update this timer history.")
		return
	}
	if !lockUser(request, tx, entry.UserID) {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	startedAt, endedAt := events[0].At, events[len(events)-1].At
	overlaps, err := entryOverlapsExcept(request, tx, entry.UserID, entry.ID, startedAt, endedAt)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to validate timer events.")
		return
	}
	if overlaps {
		respondError(writer, http.StatusConflict, "TIME_ENTRY_OVERLAP", "The edited timer overlaps an existing time entry.")
		return
	}
	_, err = tx.Exec(request.Context(), `UPDATE time_entries SET started_at = $1, ended_at = $2, duration_seconds = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $4`, startedAt, endedAt, durationSeconds, entry.ID)
	if isOverlapConstraintViolation(err) {
		respondError(writer, http.StatusConflict, "TIME_ENTRY_OVERLAP", "The edited timer overlaps an existing time entry.")
		return
	}
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	if _, err := tx.Exec(request.Context(), `DELETE FROM timer_events WHERE time_entry_id = $1`, entry.ID); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	for index, event := range events {
		if _, err := tx.Exec(request.Context(), `INSERT INTO timer_events (id, time_entry_id, event_type, occurred_at, sequence_number) VALUES ($1, $2, $3, $4, $5)`, uuid.Must(uuid.NewV7()), entry.ID, event.Type, event.At, index+1); err != nil {
			respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
			return
		}
	}
	if err := auditTimeEntry(request, tx, organizationID, actorID, "TIME_ENTRY_EVENTS_UPDATED", entry.ID); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	if err := tx.Commit(request.Context()); err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update timer events.")
		return
	}
	respond(writer, http.StatusOK, map[string]any{"id": entry.ID, "startedAt": startedAt, "endedAt": endedAt, "durationSeconds": durationSeconds, "events": input.Events})
}

func validateCompletedTimerEvents(events []timerEvent, now time.Time) (int64, bool) {
	if len(events) < 2 || len(events) > maximumCompletedTimerEvents || len(events)%2 != 0 || events[0].Type != "START" || events[len(events)-1].Type != "STOP" {
		return 0, false
	}
	for index, event := range events {
		if event.At.IsZero() || event.At.After(now) || (index > 0 && !event.At.After(events[index-1].At)) {
			return 0, false
		}
		if index == 0 || index == len(events)-1 {
			continue
		}
		expected := "PAUSE"
		if index%2 == 0 {
			expected = "RESUME"
		}
		if event.Type != expected {
			return 0, false
		}
	}
	durationSeconds, valid := duration(events, events[len(events)-1].At)
	return durationSeconds, valid && durationSeconds > 0
}
