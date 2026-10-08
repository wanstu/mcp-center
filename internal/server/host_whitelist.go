package server

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// hostWhitelist restricts only the address in the request Host header; it does
// not filter the client's source IP. It is opt-in and applies to every route.
func (a *App) hostWhitelist(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, err := a.settings.Get()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !values.HostWhitelistEnabled {
			next.ServeHTTP(w, r)
			return
		}
		host := requestHostname(r.Host)
		for _, allowed := range values.AllowedHosts {
			if strings.EqualFold(strings.TrimSuffix(allowed, "."), host) {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, errors.New("当前访问域名/IP 不在白名单中"))
	})
}

func requestHostname(raw string) string {
	if strings.ContainsAny(raw, "/@\\") || strings.TrimSpace(raw) != raw {
		return ""
	}
	// Hostname handles both hostname:port and [IPv6]:port, without looking at
	// spoofable X-Forwarded-* headers.
	u := &url.URL{Host: raw}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || strings.ContainsAny(host, ": %") && net.ParseIP(host) == nil {
		return ""
	}
	return host
}
