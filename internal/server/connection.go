package server

import (
	"net/http"
	"net/url"
	"strings"
)

func (a *App) mcpResourceURL(ref string) string {
	return strings.TrimRight(a.baseURL, "/") + "/mcp/" + url.PathEscape(ref)
}

func (a *App) mcpResourceURLForRequest(r *http.Request, ref string) string {
	return a.connectionBaseURL(r) + "/mcp/" + url.PathEscape(ref)
}

func (a *App) mcpProtectedResourceMetadataURL(ref string) string {
	return strings.TrimRight(a.baseURL, "/") + "/.well-known/oauth-protected-resource/mcp/" + url.PathEscape(ref)
}

func (a *App) mcpProtectedResourceMetadataURLForRequest(r *http.Request, ref string) string {
	return a.connectionBaseURL(r) + "/.well-known/oauth-protected-resource/mcp/" + url.PathEscape(ref)
}

// connectionBaseURL uses the address actually opened by the user unless an
// explicit canonical public address was configured. Never trust proxy headers.
func (a *App) connectionBaseURL(r *http.Request) string {
	if a.baseURLExplicit || r.Host == "" {
		return a.baseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (a *App) handleConnectionInfo(w http.ResponseWriter, r *http.Request) {
	baseURL := a.connectionBaseURL(r)
	parsed, _ := url.Parse(baseURL)
	host := strings.ToLower(parsed.Hostname())
	isLoopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	a.runtimeInfoMu.RLock()
	listenAddr := a.listenAddr
	a.runtimeInfoMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"base_url":                      baseURL,
		"listen_address":                listenAddr,
		"data_dir":                      a.dataDir,
		"secure":                        a.secure,
		"restart_required_for":          []string{"listen_address", "base_url", "data_dir"},
		"mcp_endpoint_template":         baseURL + "/mcp/{id}",
		"mcp_id_endpoint_template":      baseURL + "/mcp/{id}",
		"mcp_name_endpoint_template":    baseURL + "/mcp/{name}",
		"authorization_server_metadata": baseURL + "/.well-known/oauth-authorization-server",
		"is_loopback":                   isLoopback,
	})
}
