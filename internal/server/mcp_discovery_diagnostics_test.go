package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPDiscoveryDiagnosticDoesNotCaptureCredentialsOrArguments(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://example.test/mcp/demo", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{"password":"SUPER_SECRET"}}}`,
	))
	req.Header.Set("Authorization", "Bearer ACCESS_TOKEN_SECRET")
	req.Header.Set("Mcp-Session-Id", "SESSION_SECRET")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	req.Header.Set("User-Agent", "ChatGPT-Test")

	diagnostic := newMCPDiscoveryDiagnostic(req, "demo")
	data, err := json.Marshal(diagnostic.inputMeta)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, secret := range []string{"ACCESS_TOKEN_SECRET", "SUPER_SECRET", "SESSION_SECRET"} {
		if strings.Contains(got, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, got)
		}
	}
	if diagnostic.rpcMethod != "tools/call" {
		t.Fatalf("rpc method = %q", diagnostic.rpcMethod)
	}
	if present, _ := diagnostic.inputMeta["request_session_present"].(bool); !present {
		t.Fatal("request session presence was not recorded")
	}
}

func TestSummarizeMCPToolsListResponse(t *testing.T) {
	diagnostic := &mcpDiscoveryDiagnostic{
		status:     "success",
		outputMeta: map[string]any{},
	}
	summarizeMCPToolsListResponse(diagnostic, []byte(
		`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"good","inputSchema":{"type":"object"}},{"name":"bad","inputSchema":{"type":"string"}}]}}`,
	))
	if got := diagnostic.outputMeta["tool_count"]; got != 2 {
		t.Fatalf("tool_count = %#v", got)
	}
	if got := diagnostic.outputMeta["invalid_input_schema_count"]; got != 1 {
		t.Fatalf("invalid_input_schema_count = %#v", got)
	}
	names, ok := diagnostic.outputMeta["tool_names_sample"].([]string)
	if !ok || len(names) != 2 || names[0] != "good" || names[1] != "bad" {
		t.Fatalf("tool_names_sample = %#v", diagnostic.outputMeta["tool_names_sample"])
	}
}

func TestSummarizeMCPToolsListJSONRPCError(t *testing.T) {
	diagnostic := &mcpDiscoveryDiagnostic{
		status:     "success",
		outputMeta: map[string]any{},
	}
	summarizeMCPToolsListResponse(diagnostic, []byte(
		`{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"method not found"}}`,
	))
	if diagnostic.status != "error" || diagnostic.errorKind != "jsonrpc_error" {
		t.Fatalf("status=%q error_kind=%q", diagnostic.status, diagnostic.errorKind)
	}
	if got := diagnostic.outputMeta["jsonrpc_error_code"]; got != float64(-32601) {
		t.Fatalf("jsonrpc_error_code = %#v", got)
	}
}
