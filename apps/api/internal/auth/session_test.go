package auth

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("a long, unique test password")
	if err != nil {
		t.Fatalf("hashPassword returned error: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("hash uses an unexpected format: %q", hash)
	}
	if !verifyPassword("a long, unique test password", hash) {
		t.Fatal("correct password was rejected")
	}
	if verifyPassword("incorrect password", hash) {
		t.Fatal("incorrect password was accepted")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	for _, value := range []string{"", "plaintext", "$argon2id$v=19$not-parameters$salt$hash", "$bcrypt$2a$whatever"} {
		if verifyPassword("password", value) {
			t.Fatalf("malformed hash %q was accepted", value)
		}
	}
}

func TestSessionCookiesHaveExpectedSecurityProperties(t *testing.T) {
	recorder := httptest.NewRecorder()
	setSessionCookies(recorder, "session-token", "csrf-token", time.Now().Add(time.Hour), true)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != 2 {
		t.Fatalf("session cookie is not HttpOnly, Secure, SameSite=Lax: %#v", cookies[0])
	}
	if cookies[1].HttpOnly || !cookies[1].Secure || cookies[1].Value != "csrf-token" {
		t.Fatalf("csrf cookie has unexpected properties: %#v", cookies[1])
	}
}
