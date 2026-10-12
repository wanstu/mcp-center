package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestMCPImportADMSourcesPreviewSelectionAndSanitization(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	cookie := bootstrapSession(t, app)
	const secret = "sensitive-test-secret-not-for-preview"
	content := `{
 // OpenCode local & remote, with trailing commas
 "mcp":{"servers":{
   "local-tools":{"type":"local","command":["node","server.js"],"environment":{"DEMO_KEY":"` + secret + `"},},
   "remote-tools":{"type":"remote","url":"https://example.test/mcp","headers":{"Authorization":"Bearer ` + secret + `"},},
 }}}`
	payload := map[string]any{"format": "auto", "content": content, "convert_literals": true}
	previewResponse := requestJSON(t, handler, http.MethodPost, "/api/mcps/import/preview", payload, cookie)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	if strings.Contains(previewResponse.Body.String(), secret) {
		t.Fatal("preview leaked raw secret")
	}
	var preview mcpImportPreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Format != importOpenCode || len(preview.Candidates) != 2 {
		t.Fatalf("unexpected preview %+v", preview)
	}
	for _, c := range preview.Candidates {
		if c.ExistingMCPID != "" || len(c.Errors) > 0 || len(c.ReferenceRequirements) != 1 {
			t.Fatalf("unexpected candidate preview: %+v", c)
		}
		if len(c.Warnings) == 0 {
			t.Fatalf("missing secret conversion warning: %+v", c)
		}
	}
	list := requestJSON(t, handler, http.MethodGet, "/api/mcps", nil, cookie)
	if strings.Contains(list.Body.String(), "local-tools") || strings.Contains(list.Body.String(), "remote-tools") {
		t.Fatal("preview mutated store")
	}
	payload["selected_names"] = []string{"remote-tools"}
	apply := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", payload, cookie)
	if apply.Code != http.StatusOK || !strings.Contains(apply.Body.String(), `"added":1`) {
		t.Fatalf("apply %d %s", apply.Code, apply.Body.String())
	}
	if strings.Contains(apply.Body.String(), secret) {
		t.Fatal("import result leaked secret")
	}
	saved, err := app.core.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Name != "remote-tools" {
		t.Fatalf("selected import saved %+v", saved)
	}
	if !strings.HasPrefix(saved[0].HeaderRefs["Authorization"], "Bearer ${REMOTE_TOOLS_HEADERS_AUTHORIZATION_") {
		t.Fatalf("unexpected generated reference %+v", saved[0].HeaderRefs)
	}
	if strings.Contains(saved[0].HeaderRefs["Authorization"], secret) {
		t.Fatal("literal credential persisted")
	}
	invalid := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": content, "convert_literals": true, "selected_names": []string{}}, cookie)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("empty selection accepted: %d", invalid.Code)
	}
	second := requestJSON(t, handler, http.MethodPost, "/api/mcps/import/preview", payload, cookie)
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"existing_mcp_id":"mcp_`) {
		t.Fatalf("existing detection %d %s", second.Code, second.Body.String())
	}
}
func TestMCPImportADMFormatsAndScope(t *testing.T) {
	app := newTestApp(t)
	cases := []struct{ name, format, content, scope, wantFormat string }{
		{"workbuddy", "auto", `{"mcpServers":{"wb":{"type":"streamableHttp","url":"https://example.org/mcp","runtime":"node"}}}`, "", importWorkBuddy},
		{"codex direct", "auto", `{"direct":{"command":"node","args":["run.js"]}}`, "", importCodex},
		{"mcphub", "auto", `{"servers":{"tools":{"url":"https://example.org/mcp"}}}`, "", importHub},
		{"claude", "auto", `{"projects":{"/repo/a":{"mcpServers":{"claude":{"command":"node"}}},"/repo/b":{"mcpServers":{"other":{"command":"node"}}}}}`, "/repo/b", importClaude},
		{"generic", "auto", `{"mcpServers":{"generic":{"url":"https://example.org/mcp"}}}`, "", mcpcenter.ImportFormatMCPServers},
		{"center", "auto", `{"version":1,"mcps":[{"name":"center","transport":"stdio","auth_mode":"none","executable":"node"}]}`, "", mcpcenter.ImportFormatCore},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			p, err := app.parseMCPImport(mcpImportRequest{Format: testCase.format, Content: testCase.content, SourceScope: testCase.scope})
			if err != nil {
				t.Fatal(err)
			}
			if p.Format != testCase.wantFormat || len(p.Candidates) != 1 || len(p.Candidates[0].Errors) != 0 {
				t.Fatalf("unexpected format candidates %+v", p)
			}
			if testCase.name == "workbuddy" && !strings.Contains(p.Candidates[0].Warnings[0].Kind, "ignored") {
				t.Fatal("workbuddy extension not warned")
			}
			if testCase.name == "claude" && (p.SourceScope != "/repo/b" || p.Candidates[0].Name != "other") {
				t.Fatalf("incorrect Claude project %+v", p)
			}
		})
	}
	_, err := app.parseMCPImport(mcpImportRequest{Format: importAuto, Content: cases[3].content})
	if err == nil || !strings.Contains(err.Error(), "项目路径") {
		t.Fatalf("multiple scope accepted: %v", err)
	}
}
func TestMCPImportADMBatchAtomicAndSafety(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	cookie := bootstrapSession(t, app)
	content := `{"mcpServers":{"first":{"command":"node"},"bad":{"type":"websocket","url":"https://example.org/ws"},"second":{"command":"python"}}}`
	p, err := app.parseMCPImport(mcpImportRequest{Format: "auto", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Candidates) != 3 {
		t.Fatalf("bad preview %+v", p)
	}
	rec := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": content}, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid batch accepted %d %s", rec.Code, rec.Body.String())
	}
	all, err := app.core.List()
	if err != nil || len(all) != 0 {
		t.Fatalf("invalid batch partially wrote items %+v %v", all, err)
	}
	rec = requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": content, "selected_names": []string{"first", "second"}}, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"added":2`) {
		t.Fatalf("selected valid items %d %s", rec.Code, rec.Body.String())
	}
	all, err = app.core.List()
	if err != nil || len(all) != 2 {
		t.Fatalf("valid batch %+v %v", all, err)
	}
	invalid := []string{
		`{"mcpServers":{"a":{"command":"node"},"a":{"command":"python"}}}`,
		`{"mcpServers":{"A":{"command":"node"},"a":{"command":"python"}}}`,
		`{"mcpServers":{"a":{"command":"node"}}} {"extra":1}`,
	}
	for _, source := range invalid {
		rec = requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": source}, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid json accepted %d: %s", rec.Code, source)
		}
	}
	for _, arg := range []map[string]any{
		{"format": "auto", "content": `{"mcpServers":{"safe":{"command":"node"}}}`, "selected_names": []string{"absent"}},
		{"format": "auto", "content": `{"mcpServers":{"safe":{"command":"node"}}}`, "selected_names": []string{"safe", "safe"}},
	} {
		rec = requestJSON(t, handler, http.MethodPost, "/api/mcps/import", arg, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad selection accepted %d: %s", rec.Code, rec.Body.String())
		}
	}
}
func TestMCPImportPreviewRequiresAdmin(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mcps/import/preview", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated preview %d", rec.Code)
	}
}

func TestMCPImportConflictPoliciesAndNoPartialWrite(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)
	first := `{"mcpServers":{"shared":{"url":"https://old.example.org/mcp"}}}`
	newContent := `{"mcpServers":{"another":{"command":"node"},"shared":{"url":"https://new.example.org/mcp"}}}`
	create := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": first}, session)
	if create.Code != http.StatusOK {
		t.Fatalf("bootstrap import: %d %s", create.Code, create.Body.String())
	}
	old, err := app.core.List()
	if err != nil || len(old) != 1 {
		t.Fatalf("old %+v %v", old, err)
	}
	id := old[0].ID
	reject := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": newContent, "conflict": "error"}, session)
	if reject.Code != http.StatusBadRequest {
		t.Fatalf("expected conflict error %d", reject.Code)
	}
	unchanged, err := app.core.List()
	if err != nil || len(unchanged) != 1 || unchanged[0].ID != id || unchanged[0].Endpoint != old[0].Endpoint {
		t.Fatalf("conflict mutated database %+v", unchanged)
	}
	skip := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": newContent, "conflict": "skip"}, session)
	if skip.Code != http.StatusOK || !strings.Contains(skip.Body.String(), `"skipped":1`) || !strings.Contains(skip.Body.String(), `"added":1`) {
		t.Fatalf("skip result %d %s", skip.Code, skip.Body.String())
	}
	update := requestJSON(t, handler, http.MethodPost, "/api/mcps/import", map[string]any{"format": "auto", "content": newContent, "conflict": "update_by_name", "selected_names": []string{"shared"}}, session)
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"updated":1`) {
		t.Fatalf("update result %d %s", update.Code, update.Body.String())
	}
	final, err := app.core.List()
	if err != nil || len(final) != 2 {
		t.Fatalf("final list %+v %v", final, err)
	}
	found := false
	for _, def := range final {
		if def.Name == "shared" {
			found = true
			if def.ID != id || def.Endpoint != "https://new.example.org/mcp" {
				t.Fatalf("update changed identity or URL %+v", def)
			}
		}
	}
	if !found {
		t.Fatal("shared definition missing after update")
	}
}
