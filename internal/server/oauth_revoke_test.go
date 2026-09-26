package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

func TestOAuthRevocationEndpoint(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	client, err := app.oauth.RegisterClient(oauthcore.ClientMetadata{
		ClientName:   "Revocation Client",
		RedirectURIs: []string{"https://client.example/callback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	code, err := app.oauth.Approve(oauthcore.AuthorizationRequest{
		UserID:              "usr_test",
		ClientID:            client.ID,
		RedirectURI:         client.RedirectURIs[0],
		Resource:            "http://127.0.0.1:8610/mcp/test",
		Scope:               "mcp:*",
		CodeChallenge:       oauthcore.PKCEChallengeS256(verifier),
		CodeChallengeMethod: oauthcore.CodeChallengeMethodS256,
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := app.oauth.ExchangeCode(client.ID, code, client.RedirectURIs[0], "http://127.0.0.1:8610/mcp/test", verifier)
	if err != nil {
		t.Fatal(err)
	}

	meta := requestJSON(t, handler, http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	if meta.Code != http.StatusOK || !strings.Contains(meta.Body.String(), "\"revocation_endpoint\":\"http://127.0.0.1:8610/oauth/revoke\"") {
		t.Fatalf("metadata missing revocation endpoint: %d %s", meta.Code, meta.Body.String())
	}

	form := url.Values{}
	form.Set("client_id", client.ID)
	form.Set("token", tokens.AccessToken)
	req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke = %d %s", rec.Code, rec.Body.String())
	}
	if _, err := app.oauth.ValidateAccessToken(tokens.AccessToken); err == nil {
		t.Fatal("revoked access token is still valid")
	}
	if _, err := app.oauth.Refresh(client.ID, tokens.RefreshToken); err == nil {
		t.Fatal("revocation should invalidate refresh token in the same grant")
	}

	form.Set("token", "already-gone-token")
	req = httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("idempotent revoke = %d %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthRevocationRejectsDifferentClient(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	client, err := app.oauth.RegisterClient(oauthcore.ClientMetadata{
		ClientName:   "Owner",
		RedirectURIs: []string{"https://owner.example/callback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := app.oauth.RegisterClient(oauthcore.ClientMetadata{
		ClientName:   "Other",
		RedirectURIs: []string{"https://other.example/callback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	code, err := app.oauth.Approve(oauthcore.AuthorizationRequest{
		UserID:              "usr_test",
		ClientID:            client.ID,
		RedirectURI:         client.RedirectURIs[0],
		Resource:            "http://127.0.0.1:8610/mcp/test",
		Scope:               "mcp:*",
		CodeChallenge:       oauthcore.PKCEChallengeS256(verifier),
		CodeChallengeMethod: oauthcore.CodeChallengeMethodS256,
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := app.oauth.ExchangeCode(client.ID, code, client.RedirectURIs[0], "http://127.0.0.1:8610/mcp/test", verifier)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("client_id", other.ID)
	form.Set("token", tokens.AccessToken)
	req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-client revoke = %d %s", rec.Code, rec.Body.String())
	}
	if _, err := app.oauth.ValidateAccessToken(tokens.AccessToken); err != nil {
		t.Fatalf("token should remain valid after rejected revoke: %v", err)
	}
}
