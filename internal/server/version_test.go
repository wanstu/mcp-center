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
	if !strings.Contains(rec.Body.String(), "\"version\"") {
		t.Fatalf("version response missing version: %s", rec.Body.String())
	}
}
