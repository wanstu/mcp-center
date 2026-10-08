package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpcenter "github.com/wanstu/mcp-center-core"
	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

func TestOAuthUsesRequestHostAndRespectsWhitelist(t *testing.T) {
	app := newTestApp(t)
	app.baseURLExplicit = false
	handler := app.Handler()
	def, err := app.core.Add(mcpcenter.Config{
		Name: "oauth-host-test", Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode: mcpcenter.AuthNone, Endpoint: "https://upstream.example/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	host := "mcp.example.test:8610"
	origin := "https://" + host
	resource := origin + "/mcp/" + def.Name
	get := func(h, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "https://"+h+path, nil)
		req.Host = h
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	rec := get(host, "/.well-known/oauth-authorization-server")
	if rec.Code != http.StatusOK {
		t.Fatalf("OAuth metadata: %d %s", rec.Code, rec.Body.String())
	}
	var metadata struct {
		Issuer                string `json:"issuer"`
		AuthorizationEndpoint string `json:"authorization_endpoint"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Issuer != origin || metadata.AuthorizationEndpoint != origin+"/oauth/authorize" {
		t.Fatalf("OAuth metadata = %#v", metadata)
	}
	rec = get(host, "/.well-known/oauth-protected-resource/mcp/"+def.Name)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), resource) {
		t.Fatalf("OAuth resource metadata: %d %s", rec.Code, rec.Body.String())
	}

	client, err := app.oauth.RegisterClient(oauthcore.ClientMetadata{ClientName: "Dynamic host client", RedirectURIs: []string{"http://127.0.0.1:9900/callback"}})
	if err != nil {
		t.Fatal(err)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"resource":              {resource},
		"scope":                 {"mcp:*"},
		"state":                 {"state-value"},
		"code_challenge":        {oauthcore.PKCEChallengeS256(verifier)},
		"code_challenge_method": {"S256"},
	}
	rec = get(host, "/oauth/authorize?"+q.Encode())
	if rec.Code != http.StatusOK {
		t.Fatalf("valid authorization URL: %d %s", rec.Code, rec.Body.String())
	}
	rec = get("alternate.example.test:8610", "/oauth/authorize?"+q.Encode())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("resource must match request origin, got %d", rec.Code)
	}

	authReq := httptest.NewRequest(http.MethodGet, "https://"+host+"/mcp/"+def.Name, nil)
	authReq.Host = host
	authRec := httptest.NewRecorder()
	handler.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusUnauthorized || !strings.Contains(authRec.Header().Get("WWW-Authenticate"), origin+"/.well-known/oauth-protected-resource/") {
		t.Fatalf("OAuth challenge wrong origin: %d %s", authRec.Code, authRec.Header().Get("WWW-Authenticate"))
	}

	// PKCE authorization code and bearer token are still required on an allowed Host.
	code, err := app.oauth.Approve(oauthcore.AuthorizationRequest{
		UserID: "usr_test", ClientID: client.ID, RedirectURI: client.RedirectURIs[0],
		Resource: resource, Scope: "mcp:*", CodeChallenge: oauthcore.PKCEChallengeS256(verifier),
		CodeChallengeMethod: oauthcore.CodeChallengeMethodS256,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := app.oauth.ExchangeCode(client.ID, code, client.RedirectURIs[0], resource, verifier)
	if err != nil {
		t.Fatal(err)
	}
	authReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	principal, err := app.authenticateMCPRequest(authReq, def.Name)
	if err != nil || principal.Kind != "oauth" {
		t.Fatalf("OAuth token should pass on resource host: %#v, %v", principal, err)
	}
	crossHost := authReq.Clone(authReq.Context())
	crossHost.Host = "alternate.example.test:8610"
	if p, err := app.authenticateMCPRequest(crossHost, def.Name); err == nil {
		t.Fatalf("token must not cross resource hosts: %#v", p)
	}

	values, err := app.settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	values.HostWhitelistEnabled = true
	values.AllowedHosts = []string{"mcp.example.test"}
	if _, err = app.settings.Update(values); err != nil {
		t.Fatal(err)
	}
	if got := get("alternate.example.test:8610", "/.well-known/oauth-authorization-server"); got.Code != http.StatusForbidden {
		t.Fatalf("OAuth discovery bypassed host whitelist: %d %s", got.Code, got.Body.String())
	}
	if got := get(host, "/.well-known/oauth-authorization-server"); got.Code != http.StatusOK {
		t.Fatalf("allowed OAuth discovery failed: %d %s", got.Code, got.Body.String())
	}
	if got := get("alternate.example.test:8610", "/oauth/authorize?"+q.Encode()); got.Code != http.StatusForbidden {
		t.Fatalf("OAuth authorize bypassed whitelist: %d", got.Code)
	}
	if got := get("alternate.example.test:8610", "/api/setup"); got.Code != http.StatusForbidden {
		t.Fatalf("admin route bypassed whitelist: %d", got.Code)
	}
}

func TestLegacyOAuthDisabledInSettingsDoesNotDisableOAuth(t *testing.T) {
	app := newTestApp(t)
	path := filepath.Join(app.dataDir, "settings.json")
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err = json.Unmarshal(old, &stored); err != nil {
		t.Fatal(err)
	}
	stored["oauth_disabled"] = true
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.settings.Get(); err != nil {
		t.Fatalf("legacy settings failed to load: %v", err)
	}
	result := requestJSON(t, app.Handler(), http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	if result.Code != http.StatusOK {
		t.Fatalf("legacy disabled flag must not block OAuth: %d %s", result.Code, result.Body.String())
	}
	values, err := app.settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	values.AuditRetentionDays = 12
	if _, err = app.settings.Update(values); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "oauth_disabled") {
		t.Fatalf("legacy OAuth toggle is still persisted: %s", updated)
	}
}
