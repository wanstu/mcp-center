package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestConnectionInfoAPI(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	rec := requestJSON(t, handler, http.MethodGet, "/api/connection-info", nil, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("connection info = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"\"base_url\":\"http://127.0.0.1:8610\"",
		"\"mcp_endpoint_template\":\"http://127.0.0.1:8610/mcp/{id}\"",
		"\"is_loopback\":true",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in %s", expected, body)
		}
	}
}

func TestConnectionInfoRequiresAdmin(t *testing.T) {
	app := newTestApp(t)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/api/connection-info", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
