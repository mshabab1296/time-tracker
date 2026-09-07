package organizations

import (
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	platformemail "github.com/fortune-tech/time-tracker/apps/api/internal/platform/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InvitationCreateHandler struct {
	Database   *pgxpool.Pool
	Email      platformemail.Sender
	WebBaseURL string
}
type InvitationListHandler struct{ Database *pgxpool.Pool }
type OrganizationInvitationListHandler struct{ Database *pgxpool.Pool }
type InvitationDecisionHandler struct{ Database *pgxpool.Pool }
type InvitationCancelHandler struct{ Database *pgxpool.Pool }
type InvitationResendHandler struct {
	Database   *pgxpool.Pool
	Email      platformemail.Sender
	WebBaseURL string
}
type invitationRequest struct {
	Email string `json:"email"`
}

func (handler InvitationCreateHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	var input invitationRequest
	if !decodeJSON(writer, request, &input) {
		return
	}
	email, ok := normalizedEmail(writer, input.Email)
	if !ok {
		return
	}
	invitation, token, ok := issueInvitation(writer, request, handler.Database, organizationID, actorID, email)
	if !ok {
		return
	}
	if err := sendInvitation(handler.Email, handler.WebBaseURL, email, invitation.organizationName, token); err != nil {
		respondError(writer, http.StatusServiceUnavailable, "EMAIL_DELIVERY_FAILED", "The invitation was created, but the email could not be delivered. Resend it later.")
		return
	}
	respond(writer, http.StatusCreated, invitation.response())
}

func (handler InvitationResendHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	invitationID, ok := pathUUID(writer, request, "invitationID")
	if !ok {
		return
	}
	var email, organizationName string
	err := handler.Database.QueryRow(request.Context(), `SELECT i.email, o.name FROM invitations i JOIN organizations o ON o.id = i.organization_id WHERE i.id = $1 AND i.organization_id = $2 AND i.status IN ('PENDING', 'EXPIRED')`, invitationID, organizationID).Scan(&email, &organizationName)
	if err != nil {
		respondError(writer, http.StatusNotFound, "INVITATION_NOT_FOUND", "A pending or expired invitation was not found.")
		return
	}
	invitation, token, ok := refreshInvitation(writer, request, handler.Database, invitationID, organizationID, actorID, email, organizationName)
	if !ok {
		return
	}
	if err := sendInvitation(handler.Email, handler.WebBaseURL, email, invitation.organizationName, token); err != nil {
		respondError(writer, http.StatusServiceUnavailable, "EMAIL_DELIVERY_FAILED", "The invitation was refreshed, but the email could not be delivered.")
		return
	}
	respond(writer, http.StatusOK, invitation.response())
}

func (handler InvitationListHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return
	}
	var email string
	if err := handler.Database.QueryRow(request.Context(), `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
		return
	}
	_, _ = handler.Database.Exec(request.Context(), `UPDATE invitations SET status = 'EXPIRED', updated_at = CURRENT_TIMESTAMP WHERE email = $1 AND status = 'PENDING' AND expires_at <= CURRENT_TIMESTAMP`, email)
	rows, err := handler.Database.Query(request.Context(), `SELECT i.id, i.organization_id, o.name, i.expires_at FROM invitations i JOIN organizations o ON o.id = i.organization_id WHERE i.email = $1 AND i.status = 'PENDING' ORDER BY i.created_at DESC`, email)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
		return
	}
	defer rows.Close()
	invitations := make([]map[string]any, 0)
	for rows.Next() {
		var id, organizationID uuid.UUID
		var organizationName string
		var expiresAt time.Time
		if err := rows.Scan(&id, &organizationID, &organizationName, &expiresAt); err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
			return
		}
		invitations = append(invitations, map[string]any{"id": id, "organizationId": organizationID, "organizationName": organizationName, "expiresAt": expiresAt})
	}
	if err := rows.Err(); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
		return
	}
	respond(writer, http.StatusOK, invitations)
}

func (handler OrganizationInvitationListHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := requireAdminRead(writer, request, handler.Database)
	if !ok {
		return
	}
	_, _ = handler.Database.Exec(request.Context(), `UPDATE invitations SET status = 'EXPIRED', updated_at = CURRENT_TIMESTAMP WHERE organization_id = $1 AND status = 'PENDING' AND expires_at <= CURRENT_TIMESTAMP`, organizationID)
	rows, err := handler.Database.Query(request.Context(), `SELECT id, email, status, expires_at, created_at FROM invitations WHERE organization_id = $1 ORDER BY created_at DESC`, organizationID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
		return
	}
	defer rows.Close()
	invitations := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var email, status string
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&id, &email, &status, &expiresAt, &createdAt); err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
			return
		}
		invitations = append(invitations, map[string]any{"id": id, "email": email, "status": status, "expiresAt": expiresAt, "createdAt": createdAt})
	}
	if err := rows.Err(); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to load invitations.")
		return
	}
	respond(writer, http.StatusOK, invitations)
}

func (handler InvitationDecisionHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.AuthenticatedUserID(writer, request, handler.Database)
	if !ok {
		return
	}
	invitationID, ok := pathUUID(writer, request, "invitationID")
	if !ok {
		return
	}
	decision := request.PathValue("decision")
	if decision != "accept" && decision != "decline" {
		respondError(writer, http.StatusNotFound, "NOT_FOUND", "Invitation action not found.")
		return
	}
	tx, err := handler.Database.BeginTx(request.Context(), pgx.TxOptions{})
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to update invitation.")
		return
	}
	defer tx.Rollback(request.Context())
	var organizationID uuid.UUID
	var email string
	err = tx.QueryRow(request.Context(), `SELECT i.organization_id, i.email FROM invitations i JOIN users u ON u.email = i.email WHERE i.id = $1 AND u.id = $2 AND i.status = 'PENDING' AND i.expires_at > CURRENT_TIMESTAMP FOR UPDATE`, invitationID, userID).Scan(&organizationID, &email)
	if err != nil {
		respondError(writer, http.StatusBadRequest, "INVITATION_NOT_AVAILABLE", "The invitation is unavailable, expired, or belongs to another account.")
		return
	}
	if decision == "decline" {
		_, err = tx.Exec(request.Context(), `UPDATE invitations SET status = 'DECLINED', responded_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, invitationID)
	} else {
		membershipID, idErr := uuid.NewV7()
		if idErr != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to accept invitation.")
			return
		}
		_, err = tx.Exec(request.Context(), `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'MEMBER') ON CONFLICT (organization_id, user_id) DO NOTHING`, membershipID, organizationID, userID)
		if err == nil {
			_, err = tx.Exec(request.Context(), `UPDATE invitations SET status = 'ACCEPTED', responded_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, invitationID)
		}
	}
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to update invitation.")
		return
	}
	if err = tx.Commit(request.Context()); err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to update invitation.")
		return
	}
	audit(request, handler.Database, organizationID, userID, "INVITATION_"+strings.ToUpper(decision)+"ED", "INVITATION", invitationID)
	respond(writer, http.StatusOK, map[string]any{"status": strings.ToUpper(decision) + "ED", "organizationId": organizationID})
}

func (handler InvitationCancelHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	organizationID, actorID, ok := requireAdmin(writer, request, handler.Database)
	if !ok {
		return
	}
	invitationID, ok := pathUUID(writer, request, "invitationID")
	if !ok {
		return
	}
	command, err := handler.Database.Exec(request.Context(), `UPDATE invitations SET status = 'CANCELLED', updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND organization_id = $2 AND status = 'PENDING'`, invitationID, organizationID)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to cancel invitation.")
		return
	}
	if command.RowsAffected() == 0 {
		respondError(writer, http.StatusNotFound, "INVITATION_NOT_FOUND", "A pending invitation was not found.")
		return
	}
	audit(request, handler.Database, organizationID, actorID, "INVITATION_CANCELLED", "INVITATION", invitationID)
	respond(writer, http.StatusOK, map[string]bool{"cancelled": true})
}

type invitation struct {
	id, organizationID      uuid.UUID
	email, organizationName string
	expiresAt               time.Time
}

func (value invitation) response() map[string]any {
	return map[string]any{"id": value.id, "organizationId": value.organizationID, "email": value.email, "expiresAt": value.expiresAt, "status": "PENDING"}
}
func normalizedEmail(writer http.ResponseWriter, raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if _, err := mail.ParseAddress(email); err != nil || len(email) > 320 {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a valid email address.")
		return "", false
	}
	return email, true
}
func issueInvitation(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool, organizationID, actorID uuid.UUID, email string) (invitation, string, bool) {
	var existing bool
	if err := database.QueryRow(request.Context(), `SELECT EXISTS(SELECT 1 FROM memberships m JOIN users u ON u.id = m.user_id WHERE m.organization_id = $1 AND u.email = $2)`, organizationID, email).Scan(&existing); err != nil || existing {
		respondError(writer, http.StatusConflict, "ALREADY_A_MEMBER", "That email already belongs to this organization.")
		return invitation{}, "", false
	}
	var organizationName string
	if err := database.QueryRow(request.Context(), `SELECT name FROM organizations WHERE id = $1`, organizationID).Scan(&organizationName); err != nil {
		respondError(writer, http.StatusNotFound, "ORGANIZATION_NOT_FOUND", "Organization not found.")
		return invitation{}, "", false
	}
	var invitationID uuid.UUID
	err := database.QueryRow(request.Context(), `SELECT id FROM invitations WHERE organization_id = $1 AND email = $2 AND status IN ('PENDING', 'EXPIRED') ORDER BY updated_at DESC LIMIT 1`, organizationID, email).Scan(&invitationID)
	if err != nil {
		invitationID, err = uuid.NewV7()
		if err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to create invitation.")
			return invitation{}, "", false
		}
	}
	return refreshInvitation(writer, request, database, invitationID, organizationID, actorID, email, organizationName)
}
func refreshInvitation(writer http.ResponseWriter, request *http.Request, database *pgxpool.Pool, invitationID, organizationID, actorID uuid.UUID, email, organizationName string) (invitation, string, bool) {
	token, err := auth.NewActionToken()
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to create invitation.")
		return invitation{}, "", false
	}
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	command, err := database.Exec(request.Context(), `UPDATE invitations SET token_hash = $1, invited_by_user_id = $2, status = 'PENDING', expires_at = $3, responded_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $4 AND organization_id = $5 AND email = $6 AND status = 'EXPIRED'`, auth.HashActionToken(token), actorID, expiresAt, invitationID, organizationID, email)
	if err != nil {
		respondError(writer, 500, "INTERNAL_ERROR", "Unable to refresh invitation.")
		return invitation{}, "", false
	}
	if command.RowsAffected() == 0 {
		_, err = database.Exec(request.Context(), `INSERT INTO invitations (id, organization_id, email, token_hash, invited_by_user_id, status, expires_at) VALUES ($1, $2, $3, $4, $5, 'PENDING', $6) ON CONFLICT (organization_id, email) WHERE status = 'PENDING' DO UPDATE SET token_hash = EXCLUDED.token_hash, invited_by_user_id = EXCLUDED.invited_by_user_id, expires_at = EXCLUDED.expires_at, updated_at = CURRENT_TIMESTAMP`, invitationID, organizationID, email, auth.HashActionToken(token), actorID, expiresAt)
		if err != nil {
			respondError(writer, 500, "INTERNAL_ERROR", "Unable to create invitation.")
			return invitation{}, "", false
		}
	}
	audit(request, database, organizationID, actorID, "INVITATION_SENT", "INVITATION", invitationID)
	return invitation{id: invitationID, organizationID: organizationID, email: email, organizationName: organizationName, expiresAt: expiresAt}, token, true
}
func sendInvitation(sender platformemail.Sender, webBaseURL, email, organizationName, token string) error {
	url := strings.TrimRight(webBaseURL, "/") + "/?invitationToken=" + token
	return sender.Send(email, "Invitation to "+organizationName+" on TimeTracker", "You have been invited to "+organizationName+". Sign up or log in with this email address, then accept the invitation in TimeTracker.\n\n"+url+"\n\nThis link expires in seven days.")
}
