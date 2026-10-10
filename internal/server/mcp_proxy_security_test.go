package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuthenticatedProxyHandlerAllowsPublicHostBehindLoopbackProxy(t *testing.T) {
	newServer := func() *mcp.Server {
		return mcp.NewServer(&mcp.Implementation{Name: "proxy-test", Version: "1.0.0"}, nil)
	}
	newRequest := func() *http.Request {
		body := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":"chatgpt-test","version":"1"},"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
		req := httptest.NewRequest(http.MethodPost, "http://cmcps.example/mcp/demo", strings.NewReader(body))
		req.Host = "cmcps.example"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2026-07-28")
		req.Header.Set("Mcp-Method", "server/discover")
		return req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8610}))
	}

	protected := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return newServer()
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	blocked := httptest.NewRecorder()
	protected.ServeHTTP(blocked, newRequest())
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("default SDK handler = %d, want 403; body=%s", blocked.Code, blocked.Body.String())
	}

	allowed := httptest.NewRecorder()
	newAuthenticatedProxyHTTPHandler(newServer()).ServeHTTP(allowed, newRequest())
	if allowed.Code == http.StatusForbidden {
		t.Fatalf("authenticated proxy handler still forbidden: %s", allowed.Body.String())
	}
	if allowed.Code < 200 || allowed.Code >= 300 {
		t.Fatalf("server/discover = %d, body=%s", allowed.Code, allowed.Body.String())
	}
	if !strings.Contains(allowed.Body.String(), `"result"`) {
		t.Fatalf("server/discover response missing result: %s", allowed.Body.String())
	}
}
