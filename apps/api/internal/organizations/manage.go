package organizations

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UpdateHandler struct{ Database *pgxpool.Pool }
type MembersHandler struct{ Database *pgxpool.Pool }
type RemoveMemberHandler struct{ Database *pgxpool.Pool }
type ProjectsHandler struct{ Database *pgxpool.Pool }
type ProjectHandler struct{ Database *pgxpool.Pool }
type AssignmentHandler struct{ Database *pgxpool.Pool }
type TagsHandler struct{ Database *pgxpool.Pool }
type TagHandler struct{ Database *pgxpool.Pool }

type updateOrganizationRequest struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}
type namedResourceRequest struct {
	Name string `json:"name"`
}

func (handler UpdateHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	var input updateOrganizationRequest
	if !decodeJSON(writer, request, &input) {
		return
	}
	input.Name, input.Timezone = strings.TrimSpace(input.Name), strings.TrimSpace(input.Timezone)
	if input.Name == "" || len(input.Name) > 100 {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide an organization name up to 100 characters.")
		return
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a valid IANA timezone.")
		return
	}
	_, err := handler.Database.Exec(request.Context(), `UPDATE organizations SET name = $1, timezone = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`, input.Name, input.Timezone, organizationID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to update organization.")
		return
	}
	respond(writer, http.StatusOK, map[string]any{"id": organizationID, "name": input.Name, "timezone": input.Timezone})
}

func (handler MembersHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := requireAdminRead(writer, request, handler.Database)
	if !ok {
		return
	}
	rows, err := handler.Database.Query(request.Context(), `SELECT u.id, u.email, u.name, m.role, m.created_at FROM memberships m JOIN users u ON u.id = m.user_id WHERE m.organization_id = $1 ORDER BY u.name`, organizationID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load members.")
		return
	}
	defer rows.Close()
	members := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var email, name, role string
		var createdAt time.Time
		if err := rows.Scan(&id, &email, &name, &role, &createdAt); err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to load members.")
			return
		}
		members = append(members, map[string]any{"id": id, "email": email, "name": name, "role": role, "createdAt": createdAt})
	}
	if err := rows.Err(); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load members.")
		return
	}
	respond(writer, http.StatusOK, members)
}

func (handler RemoveMemberHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	memberID, ok := pathUUID(writer, request, "userID")
	if !ok {
		return
	}
	if userHasActiveTimer(request, handler.Database, organizationID, memberID) {
		respondError(writer, http.StatusConflict, "MEMBER_HAS_ACTIVE_TIMER", "Stop the member's active timer before removing them.")
		return
	}
	command, err := handler.Database.Exec(request.Context(), `DELETE FROM memberships WHERE organization_id = $1 AND user_id = $2 AND role = 'MEMBER'`, organizationID, memberID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to remove member.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "MEMBER_NOT_FOUND", "A removable member was not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "MEMBER_REMOVED", "USER", memberID)
	respond(writer, http.StatusOK, map[string]bool{"removed": true})
}

func (handler ProjectsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		handler.create(writer, request)
		return
	}
	handler.list(writer, request)
}

func (handler ProjectsHandler) list(writer http.ResponseWriter, request *http.Request) {
	organizationID, userID, role, ok := requireOrganizationMemberRead(writer, request, handler.Database)
	if !ok {
		return
	}
	query := `SELECT p.id, p.name, p.created_at FROM projects p WHERE p.organization_id = $1 ORDER BY p.name`
	arguments := []any{organizationID}
	if role == "MEMBER" {
		query = `SELECT p.id, p.name, p.created_at FROM projects p JOIN project_assignments pa ON pa.project_id = p.id WHERE p.organization_id = $1 AND pa.user_id = $2 ORDER BY p.name`
		arguments = append(arguments, userID)
	}
	rows, err := handler.Database.Query(request.Context(), query, arguments...)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load projects.")
		return
	}
	defer rows.Close()
	projects := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var name string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to load projects.")
			return
		}
		projects = append(projects, map[string]any{"id": id, "name": name, "createdAt": createdAt})
	}
	if err := rows.Err(); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load projects.")
		return
	}
	respond(writer, http.StatusOK, projects)
}

func (handler ProjectsHandler) create(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	name, ok := namedInput(writer, request, "project")
	if !ok {
		return
	}
	projectID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create project.")
		return
	}
	_, err = handler.Database.Exec(request.Context(), `INSERT INTO projects (id, organization_id, name) VALUES ($1, $2, $3)`, projectID, organizationID, name)
	if uniqueViolation(err) {
		respondError(writer, http.StatusConflict, "PROJECT_NAME_EXISTS", "A project with that name already exists.")
		return
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create project.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "PROJECT_CREATED", "PROJECT", projectID)
	respond(writer, http.StatusCreated, map[string]any{"id": projectID, "name": name})
}

func (handler ProjectHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPatch {
		handler.rename(writer, request)
		return
	}
	handler.delete(writer, request)
}

func (handler ProjectHandler) rename(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	projectID, ok := pathUUID(writer, request, "projectID")
	if !ok {
		return
	}
	name, ok := namedInput(writer, request, "project")
	if !ok {
		return
	}
	command, err := handler.Database.Exec(request.Context(), `UPDATE projects SET name = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND organization_id = $3`, name, projectID, organizationID)
	if uniqueViolation(err) {
		respondError(writer, http.StatusConflict, "PROJECT_NAME_EXISTS", "A project with that name already exists.")
		return
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to rename project.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "PROJECT_RENAMED", "PROJECT", projectID)
	respond(writer, http.StatusOK, map[string]any{"id": projectID, "name": name})
}

func (handler ProjectHandler) delete(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	projectID, ok := pathUUID(writer, request, "projectID")
	if !ok {
		return
	}
	command, err := handler.Database.Exec(request.Context(), `DELETE FROM projects WHERE id = $1 AND organization_id = $2`, projectID, organizationID)
	if foreignKeyViolation(err) {
		respondError(writer, http.StatusConflict, "PROJECT_IN_USE", "A project with time entries cannot be deleted.")
		return
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to delete project.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "PROJECT_DELETED", "PROJECT", projectID)
	respond(writer, http.StatusOK, map[string]bool{"deleted": true})
}

func (handler AssignmentHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	projectID, ok := pathUUID(writer, request, "projectID")
	if !ok {
		return
	}
	userID, ok := pathUUID(writer, request, "userID")
	if !ok {
		return
	}
	if request.Method == http.MethodPost {
		assignmentID, err := uuid.NewV7()
		if err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to assign project.")
			return
		}
		command, err := handler.Database.Exec(request.Context(), `INSERT INTO project_assignments (id, project_id, user_id) SELECT $1, p.id, $2 FROM projects p JOIN memberships m ON m.organization_id = p.organization_id WHERE p.id = $3 AND p.organization_id = $4 AND m.user_id = $2 AND m.role = 'MEMBER' ON CONFLICT (project_id, user_id) DO NOTHING`, assignmentID, userID, projectID, organizationID)
		if err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to assign project.")
			return
		}
		if command.RowsAffected() == 0 {
			respondError(writer, http.StatusConflict, "ASSIGNMENT_NOT_CREATED", "The member or project is invalid, or the assignment already exists.")
			return
		}
		audit(request, handler.Database, organizationID, actorID, "PROJECT_ASSIGNED", "PROJECT", projectID)
		respond(writer, http.StatusCreated, map[string]bool{"assigned": true})
		return
	}
	if userHasActiveTimerForProject(request, handler.Database, organizationID, userID, projectID) {
		respondError(writer, http.StatusConflict, "ASSIGNMENT_HAS_ACTIVE_TIMER", "Stop the member's active timer for this project before removing the assignment.")
		return
	}
	command, err := handler.Database.Exec(request.Context(), `DELETE FROM project_assignments pa USING projects p WHERE pa.project_id = p.id AND pa.project_id = $1 AND pa.user_id = $2 AND p.organization_id = $3`, projectID, userID, organizationID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to remove assignment.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "Project assignment not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "PROJECT_UNASSIGNED", "PROJECT", projectID)
	respond(writer, http.StatusOK, map[string]bool{"removed": true})
}

func (handler TagsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		handler.create(writer, request)
		return
	}
	handler.list(writer, request)
}
func (handler TagsHandler) list(writer http.ResponseWriter, request *http.Request) {
	organizationID, _, _, ok := requireOrganizationMemberRead(writer, request, handler.Database)
	if !ok {
		return
	}
	rows, err := handler.Database.Query(request.Context(), `SELECT id, name, created_at FROM tags WHERE organization_id = $1 ORDER BY name`, organizationID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load tags.")
		return
	}
	defer rows.Close()
	tags := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var name string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to load tags.")
			return
		}
		tags = append(tags, map[string]any{"id": id, "name": name, "createdAt": createdAt})
	}
	if err := rows.Err(); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load tags.")
		return
	}
	respond(writer, http.StatusOK, tags)
}
func (handler TagsHandler) create(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	name, ok := namedInput(writer, request, "tag")
	if !ok {
		return
	}
	tagID, err := uuid.NewV7()
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create tag.")
		return
	}
	_, err = handler.Database.Exec(request.Context(), `INSERT INTO tags (id, organization_id, name) VALUES ($1, $2, $3)`, tagID, organizationID, name)
	if uniqueViolation(err) {
		respondError(writer, http.StatusConflict, "TAG_NAME_EXISTS", "A tag with that name already exists.")
		return
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create tag.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "TAG_CREATED", "TAG", tagID)
	respond(writer, http.StatusCreated, map[string]any{"id": tagID, "name": name})
}
func (handler TagHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPatch {
		handler.rename(writer, request)
		return
	}
	handler.delete(writer, request)
}
func (handler TagHandler) rename(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	tagID, ok := pathUUID(writer, request, "tagID")
	if !ok {
		return
	}
	name, ok := namedInput(writer, request, "tag")
	if !ok {
		return
	}
	command, err := handler.Database.Exec(request.Context(), `UPDATE tags SET name = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND organization_id = $3`, name, tagID, organizationID)
	if uniqueViolation(err) {
		respondError(writer, http.StatusConflict, "TAG_NAME_EXISTS", "A tag with that name already exists.")
		return
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to rename tag.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "TAG_RENAMED", "TAG", tagID)
	respond(writer, http.StatusOK, map[string]any{"id": tagID, "name": name})
}
func (handler TagHandler) delete(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	tagID, ok := pathUUID(writer, request, "tagID")
	if !ok {
		return
	}
	command, err := handler.Database.Exec(request.Context(), `DELETE FROM tags WHERE id = $1 AND organization_id = $2`, tagID, organizationID)
	if foreignKeyViolation(err) {
		respondError(writer, http.StatusConflict, "TAG_IN_USE", "A tag with time entries cannot be deleted.")
		return
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to delete tag.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "TAG_DELETED", "TAG", tagID)
	respond(writer, http.StatusOK, map[string]bool{"deleted": true})
}

func requireAdmin(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool) (uuid.UUID, uuid.UUID, bool) {
	organizationID, ok := pathUUID(writer, request, "organizationID")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := auth.AuthenticatedUserID(writer, request, database)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	if !hasRole(request, database, organizationID, userID, "ADMIN") {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "Administrator access is required.")
		return uuid.Nil, uuid.Nil, false
	}
	return organizationID, userID, true
}
func requireAdminRead(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool) (uuid.UUID, uuid.UUID, bool) {
	organizationID, ok := pathUUID(writer, request, "organizationID")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, database)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	if !hasRole(request, database, organizationID, userID, "ADMIN") {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "Administrator access is required.")
		return uuid.Nil, uuid.Nil, false
	}
	return organizationID, userID, true
}
func requireOrganizationMemberRead(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool) (uuid.UUID, uuid.UUID, string, bool) {
	organizationID, ok := pathUUID(writer, request, "organizationID")
	if !ok {
		return uuid.Nil, uuid.Nil, "", false
	}
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, database)
	if !ok {
		return uuid.Nil, uuid.Nil, "", false
	}
	var role string
	if err := database.QueryRow(request.Context(), `SELECT role FROM memberships WHERE organization_id = $1 AND user_id = $2`, organizationID, userID).Scan(&role); err != nil {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "Organization membership is required.")
		return uuid.Nil, uuid.Nil, "", false
	}
	return organizationID, userID, role, true
}
func hasRole(request *http.Request, database *pgxpool.Pool, organizationID, userID uuid.UUID, role string) bool {
	var found bool
	return database.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM memberships WHERE organization_id = $1 AND user_id = $2 AND role = $3)`, organizationID, userID, role).Scan(&found) == nil && found
}
func pathUUID(writer http.ResponseWriter, request *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(request.PathValue(name))
	if err != nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Path identifier is invalid.")
		return uuid.Nil, false
	}
	return id, true
}
func namedInput(writer http.ResponseWriter, request *http.Request, resource string) (string, bool) {
	var input namedResourceRequest
	if !decodeJSON(writer, request, &input) {
		return "", false
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 100 {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a "+resource+" name up to 100 characters.")
		return "", false
	}
	return input.Name, true
}
func uniqueViolation(err error) bool {
	var databaseError *pgconn.PgError
	return err != nil && errors.As(err, &databaseError) && databaseError.Code == "23505"
}
func foreignKeyViolation(err error) bool {
	var databaseError *pgconn.PgError
	return err != nil && errors.As(err, &databaseError) && databaseError.Code == "23503"
}
func userHasActiveTimer(request *http.Request, database *pgxpool.Pool, organizationID, userID uuid.UUID) bool {
	var found bool
	return database.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM time_entries WHERE organization_id = $1 AND user_id = $2 AND status IN ('RUNNING', 'PAUSED'))`, organizationID, userID).Scan(&found) == nil && found
}
func userHasActiveTimerForProject(request *http.Request, database *pgxpool.Pool, organizationID, userID, projectID uuid.UUID) bool {
	var found bool
	return database.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM time_entries WHERE organization_id = $1 AND user_id = $2 AND project_id = $3 AND status IN ('RUNNING', 'PAUSED'))`, organizationID, userID, projectID).Scan(&found) == nil && found
}
func audit(request *http.Request, database *pgxpool.Pool, organizationID, actorID uuid.UUID, action, targetType string, targetID uuid.UUID) {
	id, err := uuid.NewV7()
	if err == nil {
		_, _ = database.Exec(request.Context(), `INSERT INTO audit_logs (id, organization_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, $4, $5, $6)`, id, organizationID, actorID, action, targetType, targetID)
	}
}
