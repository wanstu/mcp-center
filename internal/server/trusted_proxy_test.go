package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoopbackReverseProxyCanSupplyHTTPSProto(t *testing.T) {
	app := newTestApp(t)
	app.baseURLExplicit = false
	req := httptest.NewRequest(http.MethodGet, "http://cmcps.wanstu.cn/.well-known/oauth-authorization-server", nil)
	req.Host = "cmcps.wanstu.cn"
	req.RemoteAddr = "127.0.0.1:45123"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "attacker.example")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata = %d %s", rec.Code, rec.Body.String())
	}
	for _, expected := range []string{
		"\"issuer\":\"https://cmcps.wanstu.cn\"",
		"\"authorization_endpoint\":\"https://cmcps.wanstu.cn/oauth/authorize\"",
		"\"registration_endpoint\":\"https://cmcps.wanstu.cn/oauth/register\"",
	} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Fatalf("missing %s in %s", expected, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "attacker.example") {
		t.Fatalf("forwarded host must not affect issuer: %s", rec.Body.String())
	}
}

func TestExternalRequestCannotSpoofForwardedHTTPS(t *testing.T) {
	app := newTestApp(t)
	app.baseURLExplicit = false
	req := httptest.NewRequest(http.MethodGet, "http://cmcps.wanstu.cn/.well-known/oauth-authorization-server", nil)
	req.Host = "cmcps.wanstu.cn"
	req.RemoteAddr = "203.0.113.25:45123"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"issuer\":\"http://cmcps.wanstu.cn\"") {
		t.Fatalf("external forwarded proto was trusted: %s", rec.Body.String())
	}
}

func TestTrustedProxyHTTPSMarksSessionCookieSecure(t *testing.T) {
	app := newTestApp(t)
	app.baseURLExplicit = false
	payload, err := json.Marshal(map[string]any{"username": "admin", "password": "very-secure-password"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://cmcps.wanstu.cn/api/setup/register", bytes.NewReader(payload))
	req.Host = "cmcps.wanstu.cn"
	req.RemoteAddr = "[::1]:45123"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("bootstrap = %d %s", rec.Code, rec.Body.String())
	}
	found := false
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookie {
			found = true
			if !cookie.Secure {
				t.Fatal("session cookie is not Secure behind trusted HTTPS proxy")
			}
		}
	}
	if !found {
		t.Fatal("session cookie missing")
	}
}

func TestForwardedProtoFallbacksOnlyFromLoopback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{name: "forwarded", header: "Forwarded", value: "for=192.0.2.4;proto=https;host=cmcps.wanstu.cn"},
		{name: "scheme", header: "X-Forwarded-Scheme", value: "https"},
		{name: "port", header: "X-Forwarded-Port", value: "443"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://cmcps.wanstu.cn/", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			req.Header.Set(tc.header, tc.value)
			if proto, ok := trustedForwardedProto(req); !ok || proto != "https" {
				t.Fatalf("proto = %q, ok = %v", proto, ok)
			}
		})
	}
}
