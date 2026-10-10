package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

// OAuth Dynamic Client Registration (RFC 7591) is deliberately limited to
// public PKCE clients. Registration never grants access to any MCP resource.
// Users still authenticate and explicitly approve authorization requests.
const (
	dynamicClientLimit           = 1500
	dynamicClientRequestsPerHour = 40
	dynamicRegistrationMaxBytes  = 16 << 10
)

type dynamicClientRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type registrationBucket struct {
	Window   time.Time
	Requests int
}

func (a *App) allowDynamicClientRegistration(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	// Limit direct network peers. Do not trust user-controlled X-Forwarded-For.
	now := time.Now()
	a.dcrMu.Lock()
	defer a.dcrMu.Unlock()
	if a.dcrBuckets == nil {
		a.dcrBuckets = make(map[string]registrationBucket)
	}
	if len(a.dcrBuckets) > 1024 {
		for k, v := range a.dcrBuckets {
			if now.Sub(v.Window) >= time.Hour {
				delete(a.dcrBuckets, k)
			}
		}
	}
	bucket := a.dcrBuckets[host]
	if now.Sub(bucket.Window) >= time.Hour {
		bucket = registrationBucket{Window: now}
	}
	if bucket.Requests >= dynamicClientRequestsPerHour {
		return false
	}
	bucket.Requests++
	a.dcrBuckets[host] = bucket
	return true
}

func (a *App) handleOAuthDynamicRegistration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	// Registering clients is not anonymous authorization; it only creates a
	// public identity for a later user-approved, resource-bound PKCE flow.
	if !a.allowDynamicClientRegistration(r.RemoteAddr) {
		w.Header().Set("Retry-After", "3600")
		writeOAuthError(w, http.StatusTooManyRequests, "rate_limited", "client registration rate limit reached")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeOAuthError(w, http.StatusUnsupportedMediaType, "invalid_client_metadata", "registration requires application/json")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, dynamicRegistrationMaxBytes+1))
	if err != nil || len(body) > dynamicRegistrationMaxBytes {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "registration JSON is too large or unreadable")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var input dynamicClientRequest
	if err := decoder.Decode(&input); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "invalid JSON registration")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "registration request contains extra JSON")
		return
	}
	if input.TokenEndpointAuthMethod != "" && input.TokenEndpointAuthMethod != "none" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "only public clients with token_endpoint_auth_method=none are supported")
		return
	}
	if len(input.RedirectURIs) < 1 || len(input.RedirectURIs) > 8 || len(input.ClientName) > 160 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "invalid redirect_uris count or client_name length")
		return
	}
	for _, uri := range input.RedirectURIs {
		if len(uri) > 2048 {
			writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri is too long")
			return
		}
	}
	if input.ClientName == "" {
		input.ClientName = "MCP OAuth Client"
	}
	clients, err := a.oauth.ListClients()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "unable to read OAuth client registry")
		return
	}
	if len(clients) >= dynamicClientLimit {
		writeOAuthError(w, http.StatusTooManyRequests, "registration_limit", "OAuth client registry capacity reached")
		return
	}
	client, err := a.oauth.RegisterClient(oauthcore.ClientMetadata{
		ClientName: input.ClientName, RedirectURIs: input.RedirectURIs,
		GrantTypes: input.GrantTypes, ResponseTypes: input.ResponseTypes,
	})
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", err.Error())
		return
	}
	// RFC 7591 requires a 201 response including the registered metadata.
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  client.ID,
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"client_name":                client.ClientName,
		"redirect_uris":              client.RedirectURIs,
		"grant_types":                client.GrantTypes,
		"response_types":             client.ResponseTypes,
		"token_endpoint_auth_method": "none",
	})
}
