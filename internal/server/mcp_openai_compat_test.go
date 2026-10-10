package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIToolOAuthMetadataAddedToToolsList(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		if err != nil || !strings.Contains(string(got), `"method":"tools/list"`) {
			t.Fatalf("request body was not preserved: %q, %v", got, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"read_env","description":"Read environment","inputSchema":{"type":"object"}}]}}`))
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/example", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	rec := httptest.NewRecorder()

	withOpenAIToolOAuthMetadata(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	tool := envelope["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)
	assertOAuthSecuritySchemes(t, tool["securitySchemes"])
	meta, ok := tool["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("_meta = %#v", tool["_meta"])
	}
	assertOAuthSecuritySchemes(t, meta["securitySchemes"])
}

func TestOpenAIToolOAuthMetadataDoesNotRewriteNonToolsList(t *testing.T) {
	const original = `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(original))
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/example", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	rec := httptest.NewRecorder()

	withOpenAIToolOAuthMetadata(next).ServeHTTP(rec, req)

	if rec.Body.String() != original {
		t.Fatalf("non-tools response changed: %s", rec.Body.String())
	}
}

func TestOpenAIToolOAuthMetadataPreservesExistingSchemes(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"x","inputSchema":{"type":"object"},"securitySchemes":[{"type":"noauth"}],"_meta":{"securitySchemes":[{"type":"noauth"}]}}]}}`)
	out, ok := enrichOpenAIToolOAuthMetadata(body)
	if !ok {
		t.Fatal("enrichment failed")
	}
	var envelope map[string]any
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	tool := envelope["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)
	if got := tool["securitySchemes"].([]any)[0].(map[string]any)["type"]; got != "noauth" {
		t.Fatalf("top-level scheme overwritten: %v", got)
	}
	meta := tool["_meta"].(map[string]any)
	if got := meta["securitySchemes"].([]any)[0].(map[string]any)["type"]; got != "noauth" {
		t.Fatalf("_meta scheme overwritten: %v", got)
	}
}

func assertOAuthSecuritySchemes(t *testing.T, raw any) {
	t.Helper()
	schemes, ok := raw.([]any)
	if !ok || len(schemes) != 1 {
		t.Fatalf("securitySchemes = %#v", raw)
	}
	scheme, ok := schemes[0].(map[string]any)
	if !ok || scheme["type"] != "oauth2" {
		t.Fatalf("security scheme = %#v", schemes[0])
	}
	scopes, ok := scheme["scopes"].([]any)
	if !ok || len(scopes) != 1 || scopes[0] != "mcp:*" {
		t.Fatalf("security scopes = %#v", scheme["scopes"])
	}
}
