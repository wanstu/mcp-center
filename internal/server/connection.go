package server

import (
	"net"
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

// The canonical command-line or administrator-configured URL takes precedence.
// Otherwise, when the immediate peer is loopback, MCP Center trusts only the
// forwarded protocol (not forwarded host) from a same-machine reverse proxy.
// Direct external requests can never spoof these headers to change the issuer.
func (a *App) connectionBaseURL(r *http.Request) string {
	if a.baseURLExplicit {
		return a.baseURL
	}
	if values, err := a.settings.Get(); err == nil && values.PublicBaseURL != "" {
		return values.PublicBaseURL
	}
	if r == nil || r.Host == "" {
		return a.baseURL
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if proto, ok := trustedForwardedProto(r); ok {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

func trustedForwardedProto(r *http.Request) (string, bool) {
	if r == nil || !requestFromLoopback(r) {
		return "", false
	}
	for _, key := range []string{"X-Forwarded-Proto", "X-Forwarded-Scheme"} {
		if proto := normalizedForwardedProto(firstForwardedValue(r.Header.Get(key))); proto != "" {
			return proto, true
		}
	}
	for _, entry := range strings.Split(r.Header.Get("Forwarded"), ",") {
		for _, item := range strings.Split(entry, ";") {
			parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "proto") {
				continue
			}
			if proto := normalizedForwardedProto(strings.Trim(parts[1], string(rune(34)))); proto != "" {
				return proto, true
			}
		}
	}
	switch firstForwardedValue(r.Header.Get("X-Forwarded-Port")) {
	case "443":
		return "https", true
	case "80":
		return "http", true
	}
	return "", false
}

func requestFromLoopback(r *http.Request) bool {
	remote := strings.TrimSpace(r.RemoteAddr)
	if remote == "" {
		return false
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = strings.Trim(remote, "[]")
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func firstForwardedValue(value string) string {
	if i := strings.IndexByte(value, ','); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value)
}

func normalizedForwardedProto(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http":
		return "http"
	case "https":
		return "https"
	default:
		return ""
	}
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
		"secure":                        strings.HasPrefix(baseURL, "https://"),
		"restart_required_for":          []string{"listen_address", "data_dir"},
		"base_url_source":               a.publicBaseURLSource(r),
		"mcp_endpoint_template":         baseURL + "/mcp/{id}",
		"mcp_id_endpoint_template":      baseURL + "/mcp/{id}",
		"mcp_name_endpoint_template":    baseURL + "/mcp/{name}",
		"authorization_server_metadata": baseURL + "/.well-known/oauth-authorization-server",
		"is_loopback":                   isLoopback,
	})
}

func (a *App) publicBaseURLSource(r *http.Request) string {
	if a.baseURLExplicit {
		return "command-line"
	}
	if values, err := a.settings.Get(); err == nil && values.PublicBaseURL != "" {
		return "settings"
	}
	if _, ok := trustedForwardedProto(r); ok {
		return "trusted-proxy"
	}
	return "automatic"
}

func (a *App) publicBaseURLIsHTTPS(r *http.Request) bool {
	return strings.HasPrefix(a.connectionBaseURL(r), "https://")
}
