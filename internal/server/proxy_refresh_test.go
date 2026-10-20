package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcenter "github.com/wanstu/mcp-center-core"
)

func newMutableToolUpstream(t *testing.T) (*mcp.Server, *httptest.Server) {
	t.Helper()

	upstream := mcp.NewServer(&mcp.Implementation{Name: "proxy-refresh-test", Version: "1.0.0"}, nil)
	upstream.AddTool(&mcp.Tool{
		Name:        "first",
		Description: "first tool",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return upstream
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return upstream, server
}

func addSecondTool(upstream *mcp.Server) {
	upstream.AddTool(&mcp.Tool{
		Name:        "second",
		Description: "second tool",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
}

func addProxyRefreshTestMCP(t *testing.T, app *App, endpoint string) mcpcenter.Definition {
	t.Helper()
	def, err := app.core.Add(mcpcenter.Config{
		Name:      "proxy-refresh",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  endpoint,
		HealthPolicy: mcpcenter.HealthPolicy{
			HealthCheckEnabled:   true,
			CheckIntervalSeconds: 60,
			ProbeTimeoutSeconds:  5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return def
}

func seedProxySnapshot(t *testing.T, app *App, id string) {
	t.Helper()
	if _, err := app.proxyHandler(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if !app.proxyCached(id) {
		t.Fatal("expected public proxy snapshot to be cached")
	}
}

func assertRefreshedToolsAndInvalidatedProxy(t *testing.T, app *App, id string) {
	t.Helper()
	if app.proxyCached(id) {
		t.Fatal("expected refreshed tool registry to invalidate public proxy snapshot")
	}
	tools, err := app.core.ListTools(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected refreshed upstream tools, got %d: %#v", len(tools), tools)
	}
}

func TestPublicToolsListRequestRefreshesUpstreamBeforeServing(t *testing.T) {
	app := newTestApp(t)
	upstream, upstreamHTTP := newMutableToolUpstream(t)
	def := addProxyRefreshTestMCP(t, app, upstreamHTTP.URL)
	seedProxySnapshot(t, app, def.ID)

	addSecondTool(upstream)
	req := httptest.NewRequest(http.MethodPost, "/mcp/"+def.Name, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err := app.refreshProxyToolsForRequest(context.Background(), def.ID, req); err != nil {
		t.Fatal(err)
	}

	assertRefreshedToolsAndInvalidatedProxy(t, app, def.ID)
}

func TestRuntimeRefreshInvalidatesPublicProxyToolSnapshot(t *testing.T) {
	app := newTestApp(t)
	upstream, upstreamHTTP := newMutableToolUpstream(t)
	def := addProxyRefreshTestMCP(t, app, upstreamHTTP.URL)
	seedProxySnapshot(t, app, def.ID)

	addSecondTool(upstream)

	req := httptest.NewRequest(http.MethodPost, "/api/runtime/"+def.ID+"/refresh", nil)
	req.SetPathValue("id", def.ID)
	rec := httptest.NewRecorder()
	app.handleRuntimeRefresh(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime refresh = %d %s", rec.Code, rec.Body.String())
	}

	assertRefreshedToolsAndInvalidatedProxy(t, app, def.ID)
}

func TestScheduledProbeInvalidatesPublicProxyToolSnapshot(t *testing.T) {
	app := newTestApp(t)
	upstream, upstreamHTTP := newMutableToolUpstream(t)
	def := addProxyRefreshTestMCP(t, app, upstreamHTTP.URL)
	seedProxySnapshot(t, app, def.ID)

	addSecondTool(upstream)
	app.runScheduledProbe(context.Background(), def.ID)

	assertRefreshedToolsAndInvalidatedProxy(t, app, def.ID)
}
