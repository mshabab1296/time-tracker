package auth

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileHandler struct{ Database *pgxpool.Pool }
type updateProfileRequest struct {
	Name string `json:"name"`
}

func (handler ProfileHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	user, ok := requireSession(writer, request, handler.Database, true)
	if !ok {
		return
	}
	var input updateProfileRequest
	if !decodeBody(writer, request, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 100 {
		writeError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Provide a name up to 100 characters.")
		return
	}
	if _, err := handler.Database.Exec(request.Context(), `UPDATE users SET name = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, input.Name, user.ID); err != nil {
		writeError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update profile.")
		return
	}
	user.Name = input.Name
	writeJSON(writer, http.StatusOK, success(userResponse(user)))
}
