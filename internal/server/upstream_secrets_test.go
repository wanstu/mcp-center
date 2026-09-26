package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestUpstreamSecretAPIResolvesMCPReferencesWithoutLeakingValue(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	add := requestJSON(t, handler, http.MethodPost, "/api/mcps", map[string]any{
		"name":      "secured-upstream",
		"transport": "streamable-http",
		"auth_mode": "headers",
		"endpoint":  "https://example.com/mcp",
		"header_refs": map[string]string{
			"Authorization": "Bearer ${UPSTREAM_TOKEN}",
		},
		"health_policy": map[string]any{
			"health_check_enabled": false,
			"auto_reconnect":       false,
		},
	}, session)
	if add.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", add.Code, add.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(add.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	mcpValue, ok := payload["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("invalid add response: %s", add.Body.String())
	}
	id, _ := mcpValue["id"].(string)
	if id == "" {
		t.Fatalf("missing MCP id: %s", add.Body.String())
	}
	if _, err := app.core.Resolve(id); err == nil {
		t.Fatal("MCP should have unresolved secret before variable is stored")
	}

	set := requestJSON(t, handler, http.MethodPut, "/api/upstream-secrets/UPSTREAM_TOKEN", map[string]any{
		"value": "super-secret-value",
	}, session)
	if set.Code != http.StatusOK {
		t.Fatalf("set secret = %d %s", set.Code, set.Body.String())
	}
	if strings.Contains(set.Body.String(), "super-secret-value") {
		t.Fatalf("set response leaked secret: %s", set.Body.String())
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/upstream-secrets", nil, session)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "UPSTREAM_TOKEN") {
		t.Fatalf("list = %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), "super-secret-value") {
		t.Fatalf("list leaked secret: %s", list.Body.String())
	}

	activation, err := app.core.Resolve(id)
	if err != nil {
		t.Fatal(err)
	}
	if got := activation.Headers["Authorization"]; got != "Bearer super-secret-value" {
		t.Fatalf("resolved Authorization = %q", got)
	}

	replace := requestJSON(t, handler, http.MethodPut, "/api/upstream-secrets/UPSTREAM_TOKEN", map[string]any{
		"value": "rotated-secret",
	}, session)
	if replace.Code != http.StatusOK {
		t.Fatalf("replace = %d %s", replace.Code, replace.Body.String())
	}
	activation, err = app.core.Resolve(id)
	if err != nil {
		t.Fatal(err)
	}
	if got := activation.Headers["Authorization"]; got != "Bearer rotated-secret" {
		t.Fatalf("rotated Authorization = %q", got)
	}

	del := requestJSON(t, handler, http.MethodDelete, "/api/upstream-secrets/UPSTREAM_TOKEN", nil, session)
	if del.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", del.Code, del.Body.String())
	}
	if _, err := app.core.Resolve(id); err == nil {
		t.Fatal("MCP should be unresolved after variable deletion")
	}
}

func TestUpstreamSecretAPIRequiresAdmin(t *testing.T) {
	app := newTestApp(t)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/api/upstream-secrets", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
