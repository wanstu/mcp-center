package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestChangePasswordRotatesSessions(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	firstSession := bootstrapSession(t, app)

	secondLogin := requestJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "very-secure-password",
	})
	if secondLogin.Code != http.StatusOK {
		t.Fatalf("second login = %d %s", secondLogin.Code, secondLogin.Body.String())
	}
	secondSession := sessionFrom(t, secondLogin)

	change := requestJSON(t, handler, http.MethodPost, "/api/account/password", map[string]any{
		"current_password": "very-secure-password",
		"new_password":     "new-secure-password-2",
	}, firstSession)
	if change.Code != http.StatusOK {
		t.Fatalf("change password = %d %s", change.Code, change.Body.String())
	}
	freshSession := sessionFrom(t, change)

	for _, old := range []*http.Cookie{firstSession, secondSession} {
		me := requestJSON(t, handler, http.MethodGet, "/api/me", nil, old)
		if me.Code != http.StatusUnauthorized {
			t.Fatalf("old session still valid = %d %s", me.Code, me.Body.String())
		}
	}
	me := requestJSON(t, handler, http.MethodGet, "/api/me", nil, freshSession)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), "\"username\":\"admin\"") {
		t.Fatalf("fresh session invalid = %d %s", me.Code, me.Body.String())
	}

	oldLogin := requestJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "very-secure-password",
	})
	if oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password still works = %d %s", oldLogin.Code, oldLogin.Body.String())
	}
	newLogin := requestJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "new-secure-password-2",
	})
	if newLogin.Code != http.StatusOK {
		t.Fatalf("new password login = %d %s", newLogin.Code, newLogin.Body.String())
	}
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	change := requestJSON(t, handler, http.MethodPost, "/api/account/password", map[string]any{
		"current_password": "wrong-password",
		"new_password":     "new-secure-password-2",
	}, session)
	if change.Code != http.StatusBadRequest || !strings.Contains(change.Body.String(), "current password") {
		t.Fatalf("wrong current password = %d %s", change.Code, change.Body.String())
	}

	me := requestJSON(t, handler, http.MethodGet, "/api/me", nil, session)
	if me.Code != http.StatusOK {
		t.Fatalf("failed password change should not invalidate session = %d %s", me.Code, me.Body.String())
	}
}
