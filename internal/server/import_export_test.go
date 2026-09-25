package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPImportExportAPI(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	content := "{\"mcpServers\":{\"remote\":{\"url\":\"https://example.com/mcp\"},\"local\":{\"command\":\"node\",\"args\":[\"server.js\"]}}}"
	imported := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{
		"format":   "auto",
		"conflict": "error",
		"content":  content,
	}, session)
	if imported.Code != http.StatusOK {
		t.Fatalf("import = %d %s", imported.Code, imported.Body.String())
	}
	if !strings.Contains(imported.Body.String(), "\"added\":2") {
		t.Fatalf("unexpected import result: %s", imported.Body.String())
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/mcps", nil, session)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "\"name\":\"remote\"") || !strings.Contains(list.Body.String(), "\"name\":\"local\"") {
		t.Fatalf("list after import = %d %s", list.Code, list.Body.String())
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/mcps/export?format=mcp-center", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "mcp-center-export.json") {
		t.Fatalf("unexpected content disposition: %q", got)
	}
	if strings.Contains(rec.Body.String(), "\"id\"") || !strings.Contains(rec.Body.String(), "\"mcps\"") {
		t.Fatalf("unexpected portable export: %s", rec.Body.String())
	}

	req, _ = http.NewRequest(http.MethodGet, "/api/mcps/export?format=mcpServers", nil)
	req.AddCookie(session)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "\"mcpServers\"") {
		t.Fatalf("generic export = %d %s", rec.Code, rec.Body.String())
	}
}

func TestMCPImportRejectsUnsafeLiteralSecret(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	content := "{\"mcpServers\":{\"unsafe\":{\"url\":\"https://example.com/mcp\",\"headers\":{\"Authorization\":\"Bearer plaintext\"}}}}"
	rec := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{
		"format":   "auto",
		"conflict": "error",
		"content":  content,
	}, session)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "environment reference") {
		t.Fatalf("unsafe import = %d %s", rec.Code, rec.Body.String())
	}
	list := requestJSON(t, handler, http.MethodGet, "/api/mcps", nil, session)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "\"mcps\":[]") {
		t.Fatalf("failed import mutated store: %d %s", list.Code, list.Body.String())
	}
}
