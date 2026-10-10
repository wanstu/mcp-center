package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// withOpenAIToolOAuthMetadata enriches tools/list descriptors with the
// OAuth metadata ChatGPT expects while leaving all other MCP traffic untouched.
// The upstream/core tool model does not retain this OpenAI extension, so the
// public OAuth boundary restores it at the final HTTP hop.
func withOpenAIToolOAuthMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMCPToolsListRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		result := rec.Result()
		defer result.Body.Close()

		body, err := io.ReadAll(result.Body)
		if err != nil {
			copyHTTPResponse(w, result, nil)
			return
		}
		enriched, ok := enrichOpenAIToolOAuthMetadata(body)
		if !ok {
			enriched = body
		}
		summarizeMCPToolsListResponse(mcpProtocolDiagnosticFrom(r.Context()), enriched)
		copyHTTPResponse(w, result, enriched)
	})
}

func isMCPToolsListRequest(r *http.Request) bool {
	if r == nil || r.Method != http.MethodPost || r.Body == nil {
		return false
	}
	body, err := readAndRestoreBody(r)
	if err != nil {
		return false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return false
	}
	var envelope struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return envelope.Method == "tools/list"
}

func enrichOpenAIToolOAuthMetadata(body []byte) ([]byte, bool) {
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false
	}
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		return nil, false
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		return nil, false
	}

	changed := false
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := tool["securitySchemes"]; !exists {
			tool["securitySchemes"] = cloneOpenAIMCPToolOAuthSchemes()
			changed = true
		}
		meta, _ := tool["_meta"].(map[string]any)
		if meta == nil {
			meta = map[string]any{}
			tool["_meta"] = meta
			changed = true
		}
		if _, exists := meta["securitySchemes"]; !exists {
			meta["securitySchemes"] = cloneOpenAIMCPToolOAuthSchemes()
			changed = true
		}
	}
	if !changed {
		return body, true
	}
	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}
	return out, true
}

func cloneOpenAIMCPToolOAuthSchemes() []any {
	return []any{
		map[string]any{
			"type":   "oauth2",
			"scopes": []string{"mcp:*"},
		},
	}
}

func copyHTTPResponse(w http.ResponseWriter, result *http.Response, body []byte) {
	for key, values := range result.Header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := result.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if body != nil {
		_, _ = w.Write(body)
	}
}
