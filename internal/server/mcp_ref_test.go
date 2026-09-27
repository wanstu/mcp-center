package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcenter "github.com/wanstu/mcp-center-core"
	accesscore "github.com/wanstu/mcp-center-core/access"
)

type authRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

type proxyEchoArgs struct {
	Message string `json:"message"`
}

func TestMCPProxySupportsIDAndNameReferences(t *testing.T) {
	upstreamMCP := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "1.0.0"}, nil)
	mcp.AddTool(upstreamMCP, &mcp.Tool{Name: "echo", Description: "echo"}, func(ctx context.Context, req *mcp.CallToolRequest, args proxyEchoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "echo:" + args.Message}}}, nil, nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return upstreamMCP
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer upstream.Close()

	app := newTestApp(t)
	def, err := app.core.Add(mcpcenter.Config{
		Name:      "wm 测试 db",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  upstream.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := bootstrapSession(t, app)
	create := requestJSON(t, app.Handler(), http.MethodPost, "/api/access/credentials", map[string]any{
		"name":         "proxy-test",
		"enabled":      true,
		"allow_bearer": true,
		"scopes":       []string{"mcp:*"},
	}, session)
	if create.Code != http.StatusCreated {
		t.Fatalf("credential = %d %s", create.Code, create.Body.String())
	}
	var created accesscore.CreatedCredential
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	center := httptest.NewServer(app.Handler())
	defer center.Close()

	callViaRef := func(ref string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client := mcp.NewClient(&mcp.Implementation{Name: "proxy-test-client", Version: "1.0.0"}, nil)
		httpClient := &http.Client{Transport: authRoundTripper{token: created.Secret}}
		cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:             center.URL + "/mcp/" + url.PathEscape(ref),
			HTTPClient:           httpClient,
			DisableStandaloneSSE: true,
		}, nil)
		if err != nil {
			t.Fatalf("connect via %q: %v", ref, err)
		}
		defer cs.Close()

		tools, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("list tools via %q: %v", ref, err)
		}
		if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
			t.Fatalf("unexpected tools via %q: %#v", ref, tools.Tools)
		}
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "echo",
			Arguments: map[string]any{"message": "ok"},
		})
		if err != nil {
			t.Fatalf("call via %q: %v", ref, err)
		}
		if len(result.Content) != 1 {
			t.Fatalf("unexpected result via %q: %#v", ref, result)
		}
		text, ok := result.Content[0].(*mcp.TextContent)
		if !ok || text.Text != "echo:ok" {
			t.Fatalf("unexpected content via %q: %#v", ref, result.Content)
		}
	}

	callViaRef(def.ID)
	callViaRef(def.Name)
	callViaRef(strings.ToUpper(def.Name))

	challengeReq := httptest.NewRequest(http.MethodPost, "/mcp/"+url.PathEscape(def.Name), nil)
	challengeRec := httptest.NewRecorder()
	app.Handler().ServeHTTP(challengeRec, challengeReq)
	if challengeRec.Code != http.StatusUnauthorized {
		t.Fatalf("name challenge status = %d %s", challengeRec.Code, challengeRec.Body.String())
	}
	expectedMetadata := app.mcpProtectedResourceMetadataURL(def.Name)
	if !strings.Contains(challengeRec.Header().Get("WWW-Authenticate"), expectedMetadata) {
		t.Fatalf("name challenge metadata = %q, want %q", challengeRec.Header().Get("WWW-Authenticate"), expectedMetadata)
	}

	missing := httptest.NewRequest(http.MethodPost, "/mcp/missing-name", nil)
	missingRec := httptest.NewRecorder()
	app.Handler().ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing name status = %d %s", missingRec.Code, missingRec.Body.String())
	}
}

func TestOAuthResourcesAcceptIDAndNameReferences(t *testing.T) {
	app := newTestApp(t)
	def, err := app.core.Add(mcpcenter.Config{
		Name:      "wm test db",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  "https://example.test/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range []string{def.ID, def.Name, strings.ToUpper(def.Name)} {
		resource := app.mcpResourceURL(ref)
		if !app.isLocalMCPResource(resource) {
			t.Fatalf("resource should be valid for %q: %s", ref, resource)
		}
	}
	if app.isLocalMCPResource(app.mcpResourceURL("missing")) {
		t.Fatal("missing MCP name should not be accepted as OAuth resource")
	}
	if app.isLocalMCPResource(app.mcpResourceURL(def.Name) + "?token=nope") {
		t.Fatal("OAuth resource with query string should be rejected")
	}

	metadataPath := "/.well-known/oauth-protected-resource/mcp/" + url.PathEscape(def.Name)
	rec := requestJSON(t, app.Handler(), http.MethodGet, metadataPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("protected resource metadata = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), app.mcpResourceURL(def.Name)) {
		t.Fatalf("metadata did not advertise name resource: %s", rec.Body.String())
	}
}
