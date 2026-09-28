package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestMCPDisableStateGuardsAllAccessPaths(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	def, err := app.core.Add(mcpcenter.Config{
		Name:      "disable-me",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}

	disable := requestJSON(t, handler, http.MethodPut, "/api/mcps/"+def.ID+"/state", map[string]any{
		"disabled": true,
	}, session)
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), "\"disabled\":true") {
		t.Fatalf("disable = %d %s", disable.Code, disable.Body.String())
	}

	got, err := app.core.Get(def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Disabled {
		t.Fatalf("disabled state not persisted: %#v", got)
	}

	tools := requestJSON(t, handler, http.MethodGet, "/api/mcps/"+def.ID+"/tools", nil, session)
	if tools.Code != http.StatusConflict {
		t.Fatalf("disabled tools = %d %s", tools.Code, tools.Body.String())
	}

	proxy := requestJSON(t, handler, http.MethodPost, "/mcp/"+url.PathEscape(def.Name), nil)
	if proxy.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled proxy = %d %s", proxy.Code, proxy.Body.String())
	}

	resource := requestJSON(t, handler, http.MethodGet, "/.well-known/oauth-protected-resource/mcp/"+url.PathEscape(def.Name), nil)
	if resource.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled resource metadata = %d %s", resource.Code, resource.Body.String())
	}

	oauthTest := requestJSON(t, handler, http.MethodPost, "/api/oauth/tests", map[string]any{
		"mcp_ref": def.Name,
	}, session)
	if oauthTest.Code != http.StatusConflict {
		t.Fatalf("disabled OAuth self-test = %d %s", oauthTest.Code, oauthTest.Body.String())
	}

	enable := requestJSON(t, handler, http.MethodPut, "/api/mcps/"+def.ID+"/state", map[string]any{
		"disabled": false,
	}, session)
	if enable.Code != http.StatusOK || strings.Contains(enable.Body.String(), "\"disabled\":true") {
		t.Fatalf("enable = %d %s", enable.Code, enable.Body.String())
	}
	got, err = app.core.Get(def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disabled {
		t.Fatalf("MCP should be enabled: %#v", got)
	}
}
