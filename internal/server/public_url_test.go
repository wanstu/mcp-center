package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestPublicHTTPSBaseURLSettingRepairsProxyOAuthMetadata(t *testing.T) {
	app := newTestApp(t)
	app.baseURLExplicit = false // Debian service without --base-url
	handler := app.Handler()
	session := bootstrapSession(t, app)
	const public = "https://cmcps.wanstu.cn"
	if _, err := app.core.Add(mcpcenter.Config{Name: "chenyang-windows", Transport: mcpcenter.TransportStreamableHTTP, AuthMode: mcpcenter.AuthNone, Endpoint: "https://upstream.example.test/mcp"}); err != nil {
		t.Fatal(err)
	}
	// Reproduce the production TLS-terminating proxy: externally HTTPS,
	// backend request over HTTP, no trustworthy TLS field on the request.
	before := requestJSON(t, handler, http.MethodGet, "http://cmcps.wanstu.cn/.well-known/oauth-authorization-server", nil)
	if before.Code != http.StatusOK || !strings.Contains(before.Body.String(), "\"issuer\":\"http://cmcps.wanstu.cn\"") {
		t.Fatalf("expected reproducible HTTP metadata before settings: %d %s", before.Code, before.Body.String())
	}
	put := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{"public_base_url": public}, session)
	if put.Code != http.StatusOK {
		t.Fatalf("save canonical HTTPS address: %d %s", put.Code, put.Body.String())
	}
	request := func(url string) map[string]any {
		t.Helper()
		res := requestJSON(t, handler, http.MethodGet, url, nil)
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", url, res.Code, res.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	auth := request("http://cmcps.wanstu.cn/.well-known/oauth-authorization-server")
	expected := map[string]string{
		"issuer":                 public,
		"authorization_endpoint": public + "/oauth/authorize",
		"token_endpoint":         public + "/oauth/token",
		"registration_endpoint":  public + "/oauth/register",
		"revocation_endpoint":    public + "/oauth/revoke",
	}
	for field, value := range expected {
		if auth[field] != value {
			t.Errorf("%s got %v wanted %q", field, auth[field], value)
		}
	}
	resource := request("http://cmcps.wanstu.cn/.well-known/oauth-protected-resource/mcp/chenyang-windows")
	if resource["resource"] != public+"/mcp/chenyang-windows" {
		t.Errorf("resource metadata wrong: %#v", resource)
	}
	servers, ok := resource["authorization_servers"].([]any)
	if !ok || len(servers) != 1 || servers[0] != public {
		t.Fatalf("resource metadata issuer wrong: %+v", resource)
	}
	info := requestJSON(t, handler, http.MethodGet, "/api/connection-info", nil, session)
	if info.Code != http.StatusOK || !strings.Contains(info.Body.String(), "\"base_url_source\":\"settings\"") || !strings.Contains(info.Body.String(), public) {
		t.Fatalf("connection info not updated: %d %s", info.Code, info.Body.String())
	}
	// Persisted settings survive process restart; command-line canonical
	// address has higher priority than administrator settings.
	values, err := app.settings.Get()
	if err != nil || values.PublicBaseURL != public {
		t.Fatal("HTTPS URL was not persisted")
	}
	explicit := newTestApp(t)
	explicit.baseURLExplicit = true
	explicit.baseURL = "https://explicit.test"
	if got := explicit.connectionBaseURL(nil); got != "https://explicit.test" {
		t.Fatalf("unexpected explicit URL %s", got)
	}
}

func TestPublicBaseURLValidationAndAdminOnly(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)
	invalid := []string{"http://cmcps.wanstu.cn", "https://cmcps.wanstu.cn/mcp/test", "https://cmcps.wanstu.cn/", "https://cmcps.wanstu.cn?foo=1", "https://u:p@cmcps.wanstu.cn", "javascript:alert(1)"}
	for _, value := range invalid {
		rec := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{"public_base_url": value}, session)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("accepted invalid URL %q: %d %s", value, rec.Code, rec.Body.String())
		}
	}
	forbidden := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{"public_base_url": "https://attacker.test"})
	if forbidden.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized update succeeded: %d", forbidden.Code)
	}
	good := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{"public_base_url": "https://cmcps.wanstu.cn"}, session)
	if good.Code != http.StatusOK {
		t.Fatalf("saving URL: %d", good.Code)
	}
	cleared := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{"public_base_url": ""}, session)
	if cleared.Code != http.StatusOK {
		t.Fatalf("clearing URL: %d", cleared.Code)
	}
	current, err := app.settings.Get()
	if err != nil || current.PublicBaseURL != "" {
		t.Fatalf("clearing URL failed %+v %v", current, err)
	}
}
