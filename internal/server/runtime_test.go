package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRuntimeManagementAPI(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	add := requestJSON(t, handler, http.MethodPost, "/api/mcps", map[string]any{
		"name":      "runtime",
		"transport": "streamable-http",
		"auth_mode": "none",
		"endpoint":  "https://example.com/mcp",
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

	list := requestJSON(t, handler, http.MethodGet, "/api/runtime", nil, session)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), id) || !strings.Contains(list.Body.String(), "\"state\":\"disconnected\"") || !strings.Contains(list.Body.String(), "\"scheduled_probes\"") {
		t.Fatalf("runtime list = %d %s", list.Code, list.Body.String())
	}

	disconnect := requestJSON(t, handler, http.MethodPost, "/api/runtime/"+id+"/disconnect", nil, session)
	if disconnect.Code != http.StatusOK || !strings.Contains(disconnect.Body.String(), "\"state\":\"disconnected\"") {
		t.Fatalf("disconnect = %d %s", disconnect.Code, disconnect.Body.String())
	}

	unauthorized := requestJSON(t, handler, http.MethodGet, "/api/runtime", nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized runtime list = %d", unauthorized.Code)
	}
}
