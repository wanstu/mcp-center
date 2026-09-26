package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accesscore "github.com/wanstu/mcp-center-core/access"
)

func bootstrapSession(t *testing.T, app *App) *http.Cookie {
	t.Helper()
	rec := requestJSON(t, app.Handler(), http.MethodPost, "/api/setup/register", map[string]any{
		"username": "admin",
		"password": "very-secure-password",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("bootstrap = %d %s", rec.Code, rec.Body.String())
	}
	return sessionFrom(t, rec)
}

func TestAccessCredentialAPIAndChannels(t *testing.T) {
	app := newTestApp(t)
	session := bootstrapSession(t, app)
	handler := app.Handler()

	settings := requestJSON(t, handler, http.MethodGet, "/api/access/settings", nil, session)
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), "\"query_enabled\":false") {
		t.Fatalf("settings = %d %s", settings.Code, settings.Body.String())
	}

	create := requestJSON(t, handler, http.MethodPost, "/api/access/credentials", map[string]any{
		"name":         "automation",
		"enabled":      true,
		"allow_bearer": true,
		"allow_header": true,
		"allow_query":  true,
		"scopes":       []string{"mcp:*"},
	}, session)
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	var created accesscore.CreatedCredential
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Secret == "" || created.Credential.ID == "" {
		t.Fatalf("unexpected credential: %#v", created)
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/access/credentials", nil, session)
	if list.Code != http.StatusOK {
		t.Fatalf("list = %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), created.Secret) {
		t.Fatal("credential list leaked the raw secret")
	}

	bearer := httptest.NewRequest(http.MethodPost, "/mcp/example", nil)
	bearer.Header.Set("Authorization", "Bearer "+created.Secret)
	principal, err := app.authenticateMCPRequest(bearer, "example")
	if err != nil {
		t.Fatal(err)
	}
	if principal.Kind != "access_credential" || principal.AuthChannel != accesscore.ChannelBearer {
		t.Fatalf("unexpected bearer principal: %#v", principal)
	}

	header := httptest.NewRequest(http.MethodPost, "/mcp/example", nil)
	header.Header.Set("X-MCP-Token", created.Secret)
	principal, err = app.authenticateMCPRequest(header, "example")
	if err != nil {
		t.Fatal(err)
	}
	if principal.AuthChannel != accesscore.ChannelHeader {
		t.Fatalf("unexpected header principal: %#v", principal)
	}

	query := httptest.NewRequest(http.MethodPost, "/mcp/example?mcp_token="+created.Secret, nil)
	if _, err := app.authenticateMCPRequest(query, "example"); err == nil {
		t.Fatal("query auth should be globally disabled by default")
	}

	updateSettings := requestJSON(t, handler, http.MethodPut, "/api/access/settings", map[string]any{
		"bearer_enabled": true,
		"header_enabled": true,
		"header_name":    "X-MCP-Token",
		"query_enabled":  true,
		"query_name":     "mcp_token",
	}, session)
	if updateSettings.Code != http.StatusOK {
		t.Fatalf("update settings = %d %s", updateSettings.Code, updateSettings.Body.String())
	}
	principal, err = app.authenticateMCPRequest(query, "example")
	if err != nil {
		t.Fatal(err)
	}
	if principal.AuthChannel != accesscore.ChannelQuery {
		t.Fatalf("unexpected query principal: %#v", principal)
	}

	ambiguous := httptest.NewRequest(http.MethodPost, "/mcp/example?mcp_token="+created.Secret, nil)
	ambiguous.Header.Set("X-MCP-Token", created.Secret)
	if _, err := app.authenticateMCPRequest(ambiguous, "example"); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("expected ambiguous credential error, got %v", err)
	}
}

func TestAccessCredentialAPIRejectsUnsupportedScopes(t *testing.T) {
	app := newTestApp(t)
	session := bootstrapSession(t, app)
	handler := app.Handler()

	rec := requestJSON(t, handler, http.MethodPost, "/api/access/credentials", map[string]any{
		"name":         "readonly",
		"enabled":      true,
		"allow_bearer": true,
		"scopes":       []string{"mcp:read"},
	}, session)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "only mcp:*") {
		t.Fatalf("unsupported scope = %d %s", rec.Code, rec.Body.String())
	}

	ok := requestJSON(t, handler, http.MethodPost, "/api/access/credentials", map[string]any{
		"name":         "full",
		"enabled":      true,
		"allow_bearer": true,
		"scopes":       []string{"mcp:*"},
	}, session)
	if ok.Code != http.StatusCreated {
		t.Fatalf("mcp:* scope = %d %s", ok.Code, ok.Body.String())
	}
}
