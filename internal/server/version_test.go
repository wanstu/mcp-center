package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestVersionEndpointIsPublic(t *testing.T) {
	app := newTestApp(t)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/api/version", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("version = %d %s", rec.Code, rec.Body.String())
	}
	for _, field := range []string{"\"version\"", "\"platform\"", "\"architecture\"", "\"go_version\""} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Fatalf("version response missing %s: %s", field, rec.Body.String())
		}
	}
}
