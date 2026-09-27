package server

import (
	"net/http"
	"net/url"
	"strings"
)

func (a *App) handleConnectionInfo(w http.ResponseWriter, _ *http.Request) {
	parsed, _ := url.Parse(a.baseURL)
	host := strings.ToLower(parsed.Hostname())
	isLoopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	writeJSON(w, http.StatusOK, map[string]any{
		"base_url":                      a.baseURL,
		"mcp_endpoint_template":         a.baseURL + "/mcp/{id}",
		"authorization_server_metadata": a.baseURL + "/.well-known/oauth-authorization-server",
		"is_loopback":                   isLoopback,
	})
}
