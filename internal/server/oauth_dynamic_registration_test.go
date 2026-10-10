package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcpcenter "github.com/wanstu/mcp-center-core"
	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

// Reproduces ChatGPT's client-ID discovery path: resource metadata,
// authorization-server discovery, unauthenticated DCR, then PKCE authorization.
func TestOAuthDynamicClientRegistrationChatGPTFlow(t *testing.T) {
	app := newTestApp(t)
	app.baseURLExplicit = false
	handler := app.Handler()
	_, err := app.core.Add(mcpcenter.Config{Name: "example", Transport: mcpcenter.TransportStreamableHTTP, AuthMode: mcpcenter.AuthNone, Endpoint: "https://upstream.test/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	host := "mcp.example.test"
	origin := "https://" + host
	send := func(method, path string, body any) *httptest.ResponseRecorder {
		rec := requestJSON(t, handler, method, origin+path, body)
		return rec
	}
	resource := send(http.MethodGet, "/.well-known/oauth-protected-resource/mcp/example", nil)
	if resource.Code != http.StatusOK || !strings.Contains(resource.Body.String(), origin+"/mcp/example") {
		t.Fatalf("resource metadata: %d %s", resource.Code, resource.Body.String())
	}
	meta := send(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	if meta.Code != http.StatusOK {
		t.Fatalf("authorization server metadata: %d %s", meta.Code, meta.Body.String())
	}
	var metadata struct {
		Issuer               string `json:"issuer"`
		RegistrationEndpoint string `json:"registration_endpoint"`
	}
	if err := json.Unmarshal(meta.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Issuer != origin || metadata.RegistrationEndpoint != origin+"/oauth/register" {
		t.Fatalf("metadata %+v", metadata)
	}
	body := map[string]any{
		"client_name":                "ChatGPT Test Client",
		"redirect_uris":              []string{"https://chatgpt.com/connector_platform_oauth_redirect"},
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"client_uri":                 "https://chatgpt.com", // Unknown metadata should be ignored per RFC7591.
	}
	reg := send(http.MethodPost, "/oauth/register", body)
	if reg.Code != http.StatusCreated {
		t.Fatalf("dynamic registration: %d %s", reg.Code, reg.Body.String())
	}
	if reg.Header().Get("Cache-Control") != "no-store, private" {
		t.Fatalf("registration response is cacheable")
	}
	var client struct {
		ClientID                string   `json:"client_id"`
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	}
	if err := json.Unmarshal(reg.Body.Bytes(), &client); err != nil {
		t.Fatal(err)
	}
	if client.ClientID == "" || client.TokenEndpointAuthMethod != "none" || client.ClientIDIssuedAt == 0 || len(client.RedirectURIs) != 1 {
		t.Fatalf("invalid RFC7591 registration %#v", client)
	}
	if strings.Contains(reg.Body.String(), "client_secret") {
		t.Fatal("public client must not receive client_secret")
	}
	verifier := "test-verifier-that-is-long-enough-for-pkce-abcdefghijklmnopqrstuvwxyz"
	q := url.Values{
		"client_id": {client.ClientID}, "response_type": {"code"}, "redirect_uri": {client.RedirectURIs[0]},
		"resource": {origin + "/mcp/example"}, "scope": {"mcp:*"}, "state": {"state"},
		"code_challenge": {oauthcore.PKCEChallengeS256(verifier)}, "code_challenge_method": {"S256"},
	}
	authorize := send(http.MethodGet, "/oauth/authorize?"+q.Encode(), nil)
	if authorize.Code != http.StatusOK || !strings.Contains(authorize.Body.String(), "登录 MCP Center") {
		t.Fatalf("consent must require login: %d %s", authorize.Code, authorize.Body.String())
	}
	loggedIn := bootstrapSession(t, app)
	authReq := httptest.NewRequest(http.MethodGet, origin+"/oauth/authorize?"+q.Encode(), nil)
	authReq.AddCookie(loggedIn)
	authResp := httptest.NewRecorder()
	handler.ServeHTTP(authResp, authReq)
	if authResp.Code != http.StatusOK || !strings.Contains(authResp.Body.String(), "授权 MCP 客户端") || !strings.Contains(authResp.Body.String(), "https://chatgpt.com/connector_platform_oauth_redirect") {
		t.Fatalf("logged in consent prompt failed: %d %s", authResp.Code, authResp.Body.String())
	}
	code, err := app.oauth.Approve(oauthcore.AuthorizationRequest{
		UserID: "usr_test", ClientID: client.ClientID, RedirectURI: client.RedirectURIs[0], Resource: origin + "/mcp/example", Scope: "mcp:*",
		CodeChallenge: oauthcore.PKCEChallengeS256(verifier), CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := app.oauth.ExchangeCode(client.ClientID, code, client.RedirectURIs[0], origin+"/mcp/example", verifier)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.oauth.ValidateAccessTokenForResource(tokens.AccessToken, origin+"/mcp/example"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.oauth.ValidateAccessTokenForResource(tokens.AccessToken, origin+"/mcp/another"); err == nil {
		t.Fatal("token should be resource-bound")
	}
}

func TestOAuthDynamicClientRegistrationValidationAndAbuseControl(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	valid := map[string]any{"client_name": "new client", "redirect_uris": []string{"https://client.example.test/callback"}}
	cases := []struct {
		name   string
		data   map[string]any
		status int
	}{
		{"missing_redirect", map[string]any{"client_name": "none"}, http.StatusBadRequest},
		{"wrong_auth", map[string]any{"redirect_uris": []string{"https://example.test/callback"}, "token_endpoint_auth_method": "client_secret_post"}, http.StatusBadRequest},
		{"unsafe_http", map[string]any{"redirect_uris": []string{"http://example.test/callback"}}, http.StatusBadRequest},
		{"javascript_uri", map[string]any{"redirect_uris": []string{"javascript:alert(1)"}}, http.StatusBadRequest},
		{"unsupported_grant", map[string]any{"redirect_uris": []string{"https://example.test/callback"}, "grant_types": []string{"client_credentials"}}, http.StatusBadRequest},
		{"valid", valid, http.StatusCreated},
	}
	for _, tc := range cases {
		rec := requestJSON(t, handler, http.MethodPost, "/oauth/register", tc.data)
		if rec.Code != tc.status {
			t.Fatalf("%s got %d expected %d: %s", tc.name, rec.Code, tc.status, rec.Body.String())
		}
	}
	count := 0
	for i := 0; i < dynamicClientRequestsPerHour+2; i++ {
		rec := requestJSON(t, handler, http.MethodPost, "/oauth/register", valid)
		if rec.Code == http.StatusTooManyRequests {
			count++
		}
	}
	if count == 0 {
		t.Fatal("registration rate limiter did not activate")
	}
}
