package server

import (
	"net/http"
	"net/url"
	"strings"
)

func (a *App) mcpResourceURL(ref string) string {
	return strings.TrimRight(a.baseURL, "/") + "/mcp/" + url.PathEscape(ref)
}

func (a *App) mcpProtectedResourceMetadataURL(ref string) string {
	return strings.TrimRight(a.baseURL, "/") + "/.well-known/oauth-protected-resource/mcp/" + url.PathEscape(ref)
}

func (a *App) handleConnectionInfo(w http.ResponseWriter, _ *http.Request) {
	parsed, _ := url.Parse(a.baseURL)
	host := strings.ToLower(parsed.Hostname())
	isLoopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	a.runtimeInfoMu.RLock()
	listenAddr := a.listenAddr
	a.runtimeInfoMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"base_url":                      a.baseURL,
		"listen_address":                listenAddr,
		"data_dir":                      a.dataDir,
		"secure":                        a.secure,
		"restart_required_for":          []string{"listen_address", "base_url", "data_dir"},
		"mcp_endpoint_template":         a.baseURL + "/mcp/{id}",
		"mcp_id_endpoint_template":      a.baseURL + "/mcp/{id}",
		"mcp_name_endpoint_template":    a.baseURL + "/mcp/{name}",
		"authorization_server_metadata": a.baseURL + "/.well-known/oauth-authorization-server",
		"is_loopback":                   isLoopback,
	})
}
