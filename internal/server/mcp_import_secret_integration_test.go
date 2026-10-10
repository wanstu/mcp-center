package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportAutomaticallyEncryptsSelectedMCPVariablesAndRevealsToAdmin(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)
	const privateValue = "import-autosave-probe-private-token-917"
	const differentValue = "another-import-autosave-probe-private-token-917"
	content := `{"mcpServers":{"One Tool":{"command":"node","env":{"MY_TOKEN":"` + privateValue + `"}},"Other Tool":{"command":"node","env":{"MY_TOKEN":"` + differentValue + `"}}}}`
	payload := map[string]any{"format": "auto", "content": content, "convert_literals": true}
	preview := requestJSON(t, handler, http.MethodPost, "/api/mcps/import/preview", payload, session)
	if preview.Code != http.StatusOK || strings.Contains(preview.Body.String(), privateValue) || strings.Contains(preview.Body.String(), differentValue) {
		t.Fatalf("preview failed/leaked secret: %d", preview.Code)
	}
	var parsed mcpImportPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Candidates) != 2 {
		t.Fatalf("unexpected preview items: %+v", parsed.Candidates)
	}
	ref := ""
	for _, c := range parsed.Candidates {
		if c.Name == "One Tool" {
			if len(c.ReferenceRequirements) != 1 {
				t.Fatalf("missing ref: %+v", c)
			}
			ref = c.ReferenceRequirements[0].ReferenceName
		}
	}
	if !strings.HasPrefix(ref, "ONE_TOOL_ENV_MY_TOKEN_") {
		t.Fatalf("MCP prefix missing: %q", ref)
	}
	payload["selected_names"] = []string{"One Tool"}
	apply := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", payload, session)
	if apply.Code != http.StatusOK || strings.Contains(apply.Body.String(), privateValue) {
		t.Fatalf("import failed/leaked: %d %s", apply.Code, apply.Body.String())
	}
	m, err := app.core.List()
	if err != nil || len(m) != 1 || m[0].EnvRefs["MY_TOKEN"] != "$"+"{"+ref+"}" {
		t.Fatalf("MCP definition invalid: %+v err=%v", m, err)
	}
	resolved, err := app.core.Resolve(m[0].ID)
	if err != nil || resolved.Env["MY_TOKEN"] != privateValue {
		t.Fatalf("runtime didn't resolve imported secret: %v", err)
	}
	secrets := app.upstreamSecrets.List()
	if len(secrets) != 1 || secrets[0].Name != ref {
		t.Fatalf("unselected variable stored: %+v", secrets)
	}
	raw, err := os.ReadFile(filepath.Join(app.dataDir, "upstream-secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), privateValue) || strings.Contains(string(raw), differentValue) {
		t.Fatal("encrypted file leaks plaintext")
	}
	unauth := requestJSON(t, handler, http.MethodGet, "/api/upstream-secrets/"+ref+"/reveal", nil)
	if unauth.Code != http.StatusUnauthorized || strings.Contains(unauth.Body.String(), privateValue) {
		t.Fatal("unauthorized reveal succeeded")
	}
	list := requestJSON(t, handler, http.MethodGet, "/api/upstream-secrets", nil, session)
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), privateValue) {
		t.Fatal("secret list leaks plaintext")
	}
	reveal := requestJSON(t, handler, http.MethodGet, "/api/upstream-secrets/"+ref+"/reveal", nil, session)
	if reveal.Code != http.StatusOK || !strings.Contains(reveal.Body.String(), privateValue) {
		t.Fatalf("admin cannot reveal variable: %d", reveal.Code)
	}
	if !strings.Contains(reveal.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("reveal response cacheable")
	}
	missing := requestJSON(t, handler, http.MethodGet, "/api/upstream-secrets/MISSING/reveal", nil, session)
	if missing.Code != http.StatusNotFound {
		t.Fatal("missing variable reveal should be 404")
	}
	repeated := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": content, "convert_literals": true, "selected_names": []string{"One Tool"}, "conflict": "update_by_name"}, session)
	if repeated.Code != http.StatusOK || len(app.upstreamSecrets.List()) != 1 {
		t.Fatalf("idempotent import failed: %d %s", repeated.Code, repeated.Body.String())
	}
}

func TestImportAtomicOnConflictAndSkipsUnselectedSecrets(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)
	first := `{"mcpServers":{"already":{"command":"node"}}}`
	create := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": first}, session)
	if create.Code != http.StatusOK {
		t.Fatalf("seed failed: %d", create.Code)
	}
	content := `{"mcpServers":{"already":{"command":"node","env":{"PASSWORD":"dont-save-on-conflict"}},"fresh":{"command":"node","env":{"PASSWORD":"persist-on-skip"}}}}`
	failed := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": content, "convert_literals": true, "conflict": "error"}, session)
	if failed.Code != http.StatusBadRequest || len(app.upstreamSecrets.List()) != 0 {
		t.Fatalf("failed import left variables: %d %+v", failed.Code, app.upstreamSecrets.List())
	}
	skipped := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": content, "convert_literals": true, "conflict": "skip"}, session)
	if skipped.Code != http.StatusOK || len(app.upstreamSecrets.List()) != 1 {
		t.Fatalf("skip created wrong variables: %d %+v", skipped.Code, app.upstreamSecrets.List())
	}
	if !strings.HasPrefix(app.upstreamSecrets.List()[0].Name, "FRESH_ENV_PASSWORD_") {
		t.Fatal("skipped MCP variable was created")
	}
}
