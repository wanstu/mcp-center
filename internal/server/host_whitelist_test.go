package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func hostRequest(t *testing.T, handler http.Handler, host, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
	req.Host = host
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func TestHostWhitelistDisabledAllowsAnyHostname(t *testing.T) {
	app := newTestApp(t)
	for _, host := range []string{"127.0.0.1:8610", "192.168.1.20:8610", "example.local:8610", "my.example.com:8610", "[::1]:8610"} {
		result := hostRequest(t, app.Handler(), host, "/api/setup", nil)
		if result.Code != http.StatusOK {
			t.Fatalf("default disabled host %s status %d: %s", host, result.Code, result.Body.String())
		}
	}
}

func TestHostWhitelistLiveSettingsAndHostMatching(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	cookie := bootstrapSession(t, app)
	config := map[string]any{"host_whitelist_enabled": true, "allowed_hosts": []string{"localhost", "192.168.1.20", "Example.COM", "::1"}}
	put := requestJSON(t, handler, http.MethodPut, "/api/settings", config, cookie)
	if put.Code != http.StatusOK {
		t.Fatalf("configure: %d %s", put.Code, put.Body.String())
	}
	for _, host := range []string{"localhost:8610", "192.168.1.20:8610", "EXAMPLE.com:8610", "[::1]:8610"} {
		if r := hostRequest(t, handler, host, "/api/setup", nil); r.Code != http.StatusOK {
			t.Errorf("allow %s: %d %s", host, r.Code, r.Body.String())
		}
	}
	for _, host := range []string{"evil.example.com:8610", "another.local:8610", "127.0.0.1:8610"} {
		if r := hostRequest(t, handler, host, "/api/setup", nil); r.Code != http.StatusForbidden {
			t.Errorf("deny %s: %d %s", host, r.Code, r.Body.String())
		}
	}
	// Host restrictions apply to all paths, even before the session/access checks.
	r := hostRequest(t, handler, "bad.example:8610", "/mcp/anything", nil)
	if r.Code != http.StatusForbidden {
		t.Fatalf("MCP denied by host: %d", r.Code)
	}
	req := httptest.NewRequest(http.MethodPut, "http://example.com:8610/api/settings", strings.NewReader(`{"host_whitelist_enabled":false}`))
	req.Host = "example.com:8610"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable from allowed host: %d %s", rec.Code, rec.Body.String())
	}
	if result := hostRequest(t, handler, "bad.example:8610", "/api/setup", nil); result.Code != http.StatusOK {
		t.Fatalf("disable did not apply immediately: %d", result.Code)
	}
	stored, e := app.settings.Get()
	if e != nil {
		t.Fatal(e)
	}
	if len(stored.AllowedHosts) != 4 || stored.HostWhitelistEnabled {
		t.Fatalf("partial update lost allowed list: %+v", stored)
	}
}

func TestHostWhitelistInvalidConfigPreservesExistingSettings(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	cookie := bootstrapSession(t, app)
	for _, invalid := range []any{
		map[string]any{"host_whitelist_enabled": true},
		map[string]any{"host_whitelist_enabled": true, "allowed_hosts": []string{}},
		map[string]any{"allowed_hosts": []string{"https://example.com"}},
		map[string]any{"allowed_hosts": []string{"example.com:8610"}},
		map[string]any{"allowed_hosts": []string{"*.example.com"}},
	} {
		rec := requestJSON(t, handler, http.MethodPut, "/api/settings", invalid, cookie)
		if rec.Code != http.StatusBadRequest {
			body, _ := json.Marshal(invalid)
			t.Errorf("expected 400 for %s, got %d", body, rec.Code)
		}
	}
	values, e := app.settings.Get()
	if e != nil {
		t.Fatal(e)
	}
	if values.HostWhitelistEnabled || len(values.AllowedHosts) != 0 {
		t.Fatalf("unexpected settings after failed updates: %+v", values)
	}
}

func TestConnectionInfoUsesIncomingHostUnlessExplicitBaseURL(t *testing.T) {
	app := newTestApp(t)
	// A direct App constructed with BaseURL set intentionally has a fixed URL.
	cookie := bootstrapSession(t, app)
	got := hostRequest(t, app.Handler(), "10.0.0.42:8610", "/api/connection-info", cookie)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "http://127.0.0.1:8610") {
		t.Fatalf("explicit base URL changed: %s", got.Body.String())
	}
	app.baseURLExplicit = false
	got = hostRequest(t, app.Handler(), "10.0.0.42:8610", "/api/connection-info", cookie)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "http://10.0.0.42:8610/mcp/{id}") {
		t.Fatalf("dynamic base URL missing: %s", got.Body.String())
	}
}
