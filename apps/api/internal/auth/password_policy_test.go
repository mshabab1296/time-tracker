package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewPasswordRequirements(t *testing.T) {
	for _, test := range []struct {
		password string
		valid    bool
	}{
		{"Abcdefg1", true},
		{"Ébcdefg١", true},
		{"Abcdef1", false},
		{"abcdefgh1", false},
		{"ABCDEFGH", false},
		{"", false},
	} {
		if got := validNewPassword(test.password); got != test.valid {
			t.Errorf("password %#v: valid=%v, want %v", test.password, got, test.valid)
		}
	}
}

func TestRegistrationAndResetRejectWeakPasswordsBeforeDatabase(t *testing.T) {
	for _, test := range []struct {
		name    string
		handler http.Handler
		body    string
	}{
		{"registration", RegisterHandler{}, `{"email":"user@example.test","name":"Test User","password":"weakpass1"}`},
		{"reset", PasswordResetConfirmHandler{}, `{"token":"synthetic-token","password":"weakpass1"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()
			test.handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), passwordRequirements) {
				t.Fatalf("status=%d response=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
