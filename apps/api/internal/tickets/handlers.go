package tickets

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct{ Database *pgxpool.Pool }

type ticketInput struct {
	Reference string `json:"reference"`
	Title     string `json:"title"`
}

type ticket struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organizationId"`
	CreatedByUserID uuid.UUID `json:"createdByUserId"`
	Reference       string    `json:"reference"`
	Title           string    `json:"title"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (handler Handler) List(writer http.ResponseWriter, request *http.Request) {
	organizationID, _, _, ok := handler.authorize(writer, request, false)
	if !ok {
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("query"))
	if len(query) > 100 {
		errorResponse(writer, 400, "VALIDATION_ERROR", "Search text is too long.")
		return
	}
	limit, offset := 25, 0
	if raw := request.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			errorResponse(writer, 400, "VALIDATION_ERROR", "Limit must be between 1 and 100.")
			return
		}
		limit = value
	}
	if raw := request.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			errorResponse(writer, 400, "VALIDATION_ERROR", "Offset must be non-negative.")
			return
		}
		offset = value
	}
	var total int
	if err := handler.Database.QueryRow(request.Context(), `SELECT count(*) FROM tickets WHERE organization_id = $1 AND ($2 = '' OR reference ILIKE '%' || $2 || '%' OR title ILIKE '%' || $2 || '%')`, organizationID, query).Scan(&total); err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to load tickets.")
		return
	}
	rows, err := handler.Database.Query(request.Context(), `SELECT id, organization_id, created_by_user_id, reference, title, created_at FROM tickets WHERE organization_id = $1 AND ($2 = '' OR reference ILIKE '%' || $2 || '%' OR title ILIKE '%' || $2 || '%') ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`, organizationID, query, limit, offset)
	if err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to load tickets.")
		return
	}
	defer rows.Close()
	items := make([]ticket, 0)
	for rows.Next() {
		var item ticket
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.CreatedByUserID, &item.Reference, &item.Title, &item.CreatedAt); err != nil {
			errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to load tickets.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to load tickets.")
		return
	}
	respond(writer, 200, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (handler Handler) Get(writer http.ResponseWriter, request *http.Request) {
	organizationID, _, _, ok := handler.authorize(writer, request, false)
	if !ok {
		return
	}
	id, ok := ticketID(writer, request)
	if !ok {
		return
	}
	var item ticket
	err := handler.Database.QueryRow(request.Context(), `SELECT id, organization_id, created_by_user_id, reference, title, created_at FROM tickets WHERE organization_id = $1 AND id = $2`, organizationID, id).Scan(&item.ID, &item.OrganizationID, &item.CreatedByUserID, &item.Reference, &item.Title, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		errorResponse(writer, 404, "TICKET_NOT_FOUND", "Ticket not found.")
		return
	}
	if err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to load ticket.")
		return
	}
	respond(writer, 200, item)
}

func (handler Handler) Create(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, _, ok := handler.authorize(writer, request, true)
	if !ok {
		return
	}
	input, ok := readInput(writer, request)
	if !ok {
		return
	}
	id, err := uuid.NewV7()
	if err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to create ticket.")
		return
	}
	var item ticket
	err = handler.Database.QueryRow(request.Context(), `INSERT INTO tickets (id, organization_id, created_by_user_id, reference, title) VALUES ($1, $2, $3, $4, $5) RETURNING id, organization_id, created_by_user_id, reference, title, created_at`, id, organizationID, actorID, input.Reference, input.Title).Scan(&item.ID, &item.OrganizationID, &item.CreatedByUserID, &item.Reference, &item.Title, &item.CreatedAt)
	if duplicate(err) {
		errorResponse(writer, 409, "TICKET_REFERENCE_EXISTS", "A ticket with this reference already exists.")
		return
	}
	if err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to create ticket.")
		return
	}
	handler.audit(request, organizationID, actorID, "TICKET_CREATED", id)
	respond(writer, 201, item)
}

func (handler Handler) Update(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, role, ok := handler.authorize(writer, request, true)
	if !ok {
		return
	}
	id, ok := ticketID(writer, request)
	if !ok {
		return
	}
	input, ok := readInput(writer, request)
	if !ok {
		return
	}
	var item ticket
	err := handler.Database.QueryRow(request.Context(), `UPDATE tickets SET reference = $1, title = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3 AND organization_id = $4 AND (created_by_user_id = $5 OR $6 = 'ADMIN') RETURNING id, organization_id, created_by_user_id, reference, title, created_at`, input.Reference, input.Title, id, organizationID, actorID, role).Scan(&item.ID, &item.OrganizationID, &item.CreatedByUserID, &item.Reference, &item.Title, &item.CreatedAt)
	if duplicate(err) {
		errorResponse(writer, 409, "TICKET_REFERENCE_EXISTS", "A ticket with this reference already exists.")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		errorResponse(writer, 404, "TICKET_NOT_FOUND", "Ticket not found or not editable.")
		return
	}
	if err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to update ticket.")
		return
	}
	handler.audit(request, organizationID, actorID, "TICKET_UPDATED", id)
	respond(writer, 200, item)
}

func (handler Handler) Delete(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, role, ok := handler.authorize(writer, request, true)
	if !ok {
		return
	}
	id, ok := ticketID(writer, request)
	if !ok {
		return
	}
	command, err := handler.Database.Exec(request.Context(), `DELETE FROM tickets WHERE id = $1 AND organization_id = $2 AND (created_by_user_id = $3 OR $4 = 'ADMIN')`, id, organizationID, actorID, role)
	if referenced(err) {
		errorResponse(writer, 409, "TICKET_IN_USE", "A ticket with time entries cannot be deleted.")
		return
	}
	if err != nil {
		errorResponse(writer, 500, "INTERNAL_ERROR", "Unable to delete ticket.")
		return
	}
	if command.RowsAffected() == 0 {
		errorResponse(writer, 404, "TICKET_NOT_FOUND", "Ticket not found or not deletable.")
		return
	}
	handler.audit(request, organizationID, actorID, "TICKET_DELETED", id)
	respond(writer, 200, map[string]bool{"deleted": true})
}

func (handler Handler) authorize(writer http.ResponseWriter, request *http.Request, write bool) (uuid.UUID, uuid.UUID, string, bool) {
	organizationID, err := uuid.Parse(request.PathValue("organizationID"))
	if err != nil || organizationID == uuid.Nil {
		errorResponse(writer, 400, "VALIDATION_ERROR", "Organization identifier is invalid.")
		return uuid.Nil, uuid.Nil, "", false
	}
	var userID uuid.UUID
	var ok bool
	if write {
		userID, ok = auth.AuthenticatedUserID(writer, request, handler.Database)
	} else {
		userID, ok = auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	}
	if !ok {
		return uuid.Nil, uuid.Nil, "", false
	}
	var role string
	if err := handler.Database.QueryRow(request.Context(), `SELECT role FROM memberships WHERE organization_id = $1 AND user_id = $2`, organizationID, userID).Scan(&role); err != nil {
		errorResponse(writer, 403, "FORBIDDEN", "Organization membership is required.")
		return uuid.Nil, uuid.Nil, "", false
	}
	return organizationID, userID, role, true
}

func ticketID(writer http.ResponseWriter, request *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(request.PathValue("ticketID"))
	if err != nil || id == uuid.Nil {
		errorResponse(writer, 400, "VALIDATION_ERROR", "Ticket identifier is invalid.")
		return uuid.Nil, false
	}
	return id, true
}

func readInput(writer http.ResponseWriter, request *http.Request) (ticketInput, bool) {
	var input ticketInput
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20)).Decode(&input); err != nil {
		errorResponse(writer, 400, "VALIDATION_ERROR", "Provide a valid ticket.")
		return input, false
	}
	input.Reference, input.Title = strings.TrimSpace(input.Reference), strings.TrimSpace(input.Title)
	if len(input.Reference) < 1 || len(input.Reference) > 100 || len(input.Title) < 1 || len(input.Title) > 200 {
		errorResponse(writer, 400, "VALIDATION_ERROR", "Ticket reference and title are required (maximum 100 and 200 characters).")
		return input, false
	}
	return input, true
}

func (handler Handler) audit(request *http.Request, organizationID, actorID uuid.UUID, action string, id uuid.UUID) {
	auditID, err := uuid.NewV7()
	if err == nil {
		_, _ = handler.Database.Exec(request.Context(), `INSERT INTO audit_logs (id, organization_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, $4, 'TICKET', $5)`, auditID, organizationID, actorID, action, id)
	}
}

func duplicate(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
func referenced(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
func respond(writer http.ResponseWriter, status int, data any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"data": data, "error": nil, "meta": map[string]any{}})
}
func errorResponse(writer http.ResponseWriter, status int, code, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"data": nil, "error": map[string]string{"code": code, "message": message}, "meta": map[string]any{}})
}
