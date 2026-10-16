package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcenter "github.com/wanstu/mcp-center-core"
	accesscore "github.com/wanstu/mcp-center-core/access"
)

func TestPublicMCPJSONRPCErrorSanitization(t *testing.T) {
	const sensitive = `Post "http://127.0.0.1:8002/mcp": EOF`
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"error": map[string]any{
			"code":    -32603,
			"message": sensitive,
			"data":    map[string]any{"path": `C:\\internal\\secret`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/eadm", strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"x","arguments":{}}}`))
	rec := httptest.NewRecorder()

	withPublicMCPErrorSanitization(next).ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "127.0.0.1:8002") || strings.Contains(body, "internal") || strings.Contains(body, "EOF") {
		t.Fatalf("sensitive upstream detail leaked: %s", body)
	}
	if !strings.Contains(body, publicMCPRequestFailed) {
		t.Fatalf("public error missing: %s", body)
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	errObj := envelope["error"].(map[string]any)
	if _, exists := errObj["data"]; exists {
		t.Fatalf("error data must be removed: %#v", errObj)
	}
	if got := int(errObj["code"].(float64)); got != -32603 {
		t.Fatalf("error code changed: %d", got)
	}
}

func TestPublicMCPErrorSanitizationPreservesSuccessfulToolResult(t *testing.T) {
	const original = `{"jsonrpc":"2.0","id":8,"result":{"content":[{"type":"text","text":"http://127.0.0.1:8002/mcp"}]}}`
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(original))
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/eadm", strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"x","arguments":{}}}`))
	rec := httptest.NewRecorder()

	withPublicMCPErrorSanitization(next).ServeHTTP(rec, req)

	if rec.Body.String() != original {
		t.Fatalf("successful tool result changed: %s", rec.Body.String())
	}
}

func TestMCPProxyBuildFailureDoesNotExposeEndpoint(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := dead.URL + "/mcp"
	dead.Close()

	app := newTestApp(t)
	def, err := app.core.Add(mcpcenter.Config{
		Name:      "dead-upstream",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  endpoint,
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := createProxyTestCredential(t, app)

	req := httptest.NewRequest(http.MethodPost, "/mcp/"+def.Name, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, endpoint) || strings.Contains(body, "127.0.0.1") || strings.Contains(body, "connection refused") {
		t.Fatalf("public response leaked upstream detail: %s", body)
	}
	if !strings.Contains(body, publicMCPUpstreamUnavailable) {
		t.Fatalf("public response = %s", body)
	}

	status, err := app.core.RuntimeStatus(def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status.LastError, endpoint) {
		t.Fatalf("admin runtime detail should retain real upstream error, got: %q", status.LastError)
	}
}

func TestMCPProxyToolErrorDoesNotExposeUpstreamDetail(t *testing.T) {
	upstreamMCP := mcp.NewServer(&mcp.Implementation{Name: "failing-upstream", Version: "1.0.0"}, nil)
	mcp.AddTool(upstreamMCP, &mcp.Tool{Name: "fail", Description: "fail"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return upstreamMCP
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))

	app := newTestApp(t)
	def, err := app.core.Add(mcpcenter.Config{
		Name:      "failing-upstream",
		Transport: mcpcenter.TransportStreamableHTTP,
		AuthMode:  mcpcenter.AuthNone,
		Endpoint:  upstream.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := createProxyTestCredential(t, app)
	center := httptest.NewServer(app.Handler())
	defer center.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "sanitize-test", Version: "1.0.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             center.URL + "/mcp/" + def.Name,
		HTTPClient:           &http.Client{Transport: authRoundTripper{token: secret}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	upstream.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "fail", Arguments: map[string]any{}})
	visible := ""
	if err != nil {
		visible = err.Error()
	} else if result != nil {
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		visible = string(data)
	}
	if strings.Contains(visible, upstream.URL) || strings.Contains(visible, "127.0.0.1") || strings.Contains(visible, "connection refused") {
		t.Fatalf("tool call leaked upstream detail: %s", visible)
	}
	if !strings.Contains(visible, publicMCPRequestFailed) {
		t.Fatalf("unexpected public error: %s", visible)
	}
}

func createProxyTestCredential(t *testing.T, app *App) string {
	t.Helper()
	session := bootstrapSession(t, app)
	create := requestJSON(t, app.Handler(), http.MethodPost, "/api/access/credentials", map[string]any{
		"name":         "sanitize-test",
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
	return created.Secret
}
