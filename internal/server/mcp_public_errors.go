package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
)

const (
	publicMCPUpstreamUnavailable = "upstream MCP unavailable"
	publicMCPRequestFailed       = "upstream MCP request failed"
)

func withPublicMCPErrorSanitization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mcpJSONRPCMethod(r) != "tools/call" {
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
		if sanitized, ok := sanitizePublicMCPJSONRPCError(body); ok {
			body = sanitized
		}
		copyHTTPResponse(w, result, body)
	})
}

func sanitizePublicMCPJSONRPCError(body []byte) ([]byte, bool) {
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false
	}
	rawError, ok := envelope["error"]
	if !ok {
		return body, false
	}
	errorObject, ok := rawError.(map[string]any)
	if !ok {
		return body, false
	}
	errorObject["message"] = publicMCPRequestFailed
	delete(errorObject, "data")

	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}
	return out, true
}

func writePublicMCPUpstreamUnavailable(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadGateway, map[string]any{
		"error":      publicMCPUpstreamUnavailable,
		"error_kind": "upstream_unavailable",
	})
}
