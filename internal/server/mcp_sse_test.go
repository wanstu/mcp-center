package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	core "github.com/wanstu/mcp-center-core"
	accesscore "github.com/wanstu/mcp-center-core/access"
)

func TestSSEUpstreamAccessibleViaCenter(t *testing.T) {
	upstream := mcp.NewServer(&mcp.Implementation{Name: "legacy-sse-upstream", Version: "1"}, nil)
	mcp.AddTool(upstream, &mcp.Tool{Name: "echo_sse"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "connected-over-sse"}}}, nil, nil
	})
	sseHandler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return upstream }, nil)
	sseServer := httptest.NewServer(sseHandler)
	defer sseServer.Close()
	app := newTestApp(t)
	def, err := app.core.Add(core.Config{Name: "sse-legacy", Transport: core.TransportSSE, Endpoint: sseServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer app.core.Disconnect(def.ID)
	cookie := bootstrapSession(t, app)
	created := requestJSON(t, app.Handler(), http.MethodPost, "/api/access/credentials", map[string]any{
		"name": "sse-test", "enabled": true, "allow_bearer": true, "scopes": []string{"mcp:*"},
	}, cookie)
	if created.Code != 201 {
		t.Fatalf("credential error: %s", created.Body.String())
	}
	var cred accesscore.CreatedCredential
	if err := json.Unmarshal(created.Body.Bytes(), &cred); err != nil {
		t.Fatal(err)
	}
	center := httptest.NewServer(app.Handler())
	defer center.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             center.URL + "/mcp/" + url.PathEscape(def.Name),
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: authRoundTripper{token: cred.Secret}},
	}, nil)
	if err != nil {
		t.Fatalf("connect Center: %v", err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "echo_sse" {
		t.Fatalf("tools %#v error=%v", tools, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo_sse", Arguments: map[string]any{}})
	status, _ := app.core.RuntimeStatus(def.ID)
	t.Logf("upstream runtime status=%+v", status)
	if err != nil || result.IsError || len(result.Content) != 1 || result.Content[0].(*mcp.TextContent).Text != "connected-over-sse" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestSSEImportFromGenericConfig(t *testing.T) {
	app := newTestApp(t)
	preview, err := app.parseMCPImport(mcpImportRequest{Format: "auto", Content: `{"mcpServers":{"remote-legacy":{"type":"sse","url":"https://example.test/events"}}}`})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 1 || len(preview.Candidates[0].Errors) > 0 || preview.Candidates[0].Transport != core.TransportSSE {
		t.Fatalf("SSE preview: %#v", preview.Candidates)
	}
}
