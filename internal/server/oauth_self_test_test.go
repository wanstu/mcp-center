package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcenter "github.com/wanstu/mcp-center-core"
)

type oauthSelfTestToolArgs struct {
	Message string `json:"message"`
}

func TestOAuthSelfTestEndToEnd(t *testing.T) {
	upstreamMCP := mcp.NewServer(&mcp.Implementation{Name: "oauth-self-test-upstream", Version: "1.0.0"}, nil)
	mcp.AddTool(upstreamMCP, &mcp.Tool{Name: "echo", Description: "echo"}, func(ctx context.Context, req *mcp.CallToolRequest, args oauthSelfTestToolArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "echo:" + args.Message}}}, nil, nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return upstreamMCP
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer upstream.Close()

	app := newTestApp(t)
	center := httptest.NewServer(app.Handler())
	defer center.Close()
	app.baseURL = center.URL
	app.secure = false

	def, err := app.core.Add(mcpcenter.Config{
		Name:      "oauth-self-test-mcp",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  upstream.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	session := bootstrapSession(t, app)
	start := requestJSON(t, app.Handler(), http.MethodPost, "/api/oauth/tests", map[string]any{
		"mcp_ref": def.Name,
	}, session)
	if start.Code != http.StatusCreated {
		t.Fatalf("start = %d %s", start.Code, start.Body.String())
	}
	var started struct {
		Test struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"test"`
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Test.ID == "" || started.AuthorizeURL == "" || started.Test.Status != "waiting_authorization" {
		t.Fatalf("unexpected start response: %s", start.Body.String())
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	centerURL, _ := url.Parse(center.URL)
	jar.SetCookies(centerURL, []*http.Cookie{session})
	client := &http.Client{Jar: jar}

	getResp, err := client.Get(started.AuthorizeURL)
	if err != nil {
		t.Fatal(err)
	}
	getBody, _ := io.ReadAll(getResp.Body)
	_ = getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK || !strings.Contains(string(getBody), "授权 MCP 客户端") {
		t.Fatalf("authorize GET = %d %s", getResp.StatusCode, getBody)
	}

	authorizeURL, err := url.Parse(started.AuthorizeURL)
	if err != nil {
		t.Fatal(err)
	}
	form := authorizeURL.Query()
	form.Set("decision", "approve")
	postResp, err := client.PostForm(center.URL+"/oauth/authorize", form)
	if err != nil {
		t.Fatal(err)
	}
	callbackBody, _ := io.ReadAll(postResp.Body)
	_ = postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK || !strings.Contains(string(callbackBody), "OAuth 自测通过") {
		t.Fatalf("authorize/callback = %d %s", postResp.StatusCode, callbackBody)
	}

	status := requestJSON(t, app.Handler(), http.MethodGet, "/api/oauth/tests/"+started.Test.ID, nil, session)
	if status.Code != http.StatusOK {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}
	var result struct {
		Test oauthSelfTest `json:"test"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Test.Status != "passed" {
		t.Fatalf("self-test did not pass: %#v", result.Test)
	}
	if result.Test.ToolCount != 1 {
		t.Fatalf("tool count = %d", result.Test.ToolCount)
	}
	for _, step := range result.Test.Steps {
		if step.Status != "passed" {
			t.Fatalf("step not passed: %#v", step)
		}
	}
	if result.Test.ClientID != "" || result.Test.Verifier != "" || result.Test.State != "" {
		t.Fatalf("self-test response leaked private data: %#v", result.Test)
	}

	clients, err := app.oauth.ListClients()
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 0 {
		t.Fatalf("temporary OAuth client was not cleaned up: %#v", clients)
	}
}

func TestOAuthSelfTestRequiresAdminAndValidMCP(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	unauthorized := requestJSON(t, handler, http.MethodPost, "/api/oauth/tests", map[string]any{
		"mcp_ref": "missing",
	})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized start = %d %s", unauthorized.Code, unauthorized.Body.String())
	}

	session := bootstrapSession(t, app)
	missing := requestJSON(t, handler, http.MethodPost, "/api/oauth/tests", map[string]any{
		"mcp_ref": "missing",
	}, session)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing MCP = %d %s", missing.Code, missing.Body.String())
	}
}
