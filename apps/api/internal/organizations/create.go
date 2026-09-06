package organizations

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateHandler struct{ Database *pgxpool.Pool }
type ListHandler struct{ Database *pgxpool.Pool }
type createRequest struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

func (handler CreateHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
	if !ok {
		return
	}
	var input createRequest
	if !decodeJSON(writer, request, &input) {
		return
	}
	input.Name, input.Timezone = strings.TrimSpace(input.Name), strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if input.Name == "" || len(input.Name) > 100 {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide an organization name up to 100 characters.")
		return
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a valid IANA timezone.")
		return
	}
	organizationID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create organization.")
		return
	}
	membershipID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create organization.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create organization.")
		return
	}
	defer tx.Rollback(request.Context())
	if _, err = tx.Exec(request.Context(), `INSERT INTO organizations (id, name, timezone) VALUES ($1, $2, $3)`, organizationID, input.Name, input.Timezone); err == nil {
		_, err = tx.Exec(request.Context(), `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'ADMIN')`, membershipID, organizationID, userID)
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create organization.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create organization.")
		return
	}
	respond(writer, http.StatusCreated, map[string]any{"id": organizationID, "name": input.Name, "timezone": input.Timezone, "role": "ADMIN"})
}

func (handler ListHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	rows, err := handler.Database.Query(request.Context(), `SELECT o.id, o.name, o.timezone, m.role FROM memberships m JOIN organizations o ON o.id = m.organization_id WHERE m.user_id = $1 ORDER BY o.name`, userID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load organizations.")
		return
	}
	defer rows.Close()
	organizations := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var name, timezone, role string
		if err := rows.Scan(&id, &name, &timezone, &role); err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to load organizations.")
			return
		}
		organizations = append(organizations, map[string]any{"id": id, "name": name, "timezone": timezone, "role": role})
	}
	if err := rows.Err(); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load organizations.")
		return
	}
	respond(writer, http.StatusOK, organizations)
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
