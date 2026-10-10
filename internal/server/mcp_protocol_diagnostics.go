package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

type mcpDiagnosticResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *mcpDiagnosticResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *mcpDiagnosticResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

func (w *mcpDiagnosticResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *mcpDiagnosticResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *mcpDiagnosticResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

type mcpProtocolDiagnostic struct {
	requestID  string
	mcpID      string
	rpcMethod  string
	startedAt  time.Time
	principal  mcpPrincipal
	inputMeta  map[string]any
	outputMeta map[string]any
	status     string
	errorKind  string
}

func newMCPProtocolDiagnostic(r *http.Request) mcpProtocolDiagnostic {
	method := mcpRPCMethod(r)
	if method == "" {
		method = strings.ToLower(strings.TrimSpace(r.Method))
	}
	return mcpProtocolDiagnostic{
		requestID: requestID(r),
		rpcMethod: method,
		startedAt: time.Now().UTC(),
		status:    "success",
		inputMeta: map[string]any{
			"http_method":             r.Method,
			"rpc_method":              method,
			"protocol_version":        strings.TrimSpace(r.Header.Get("MCP-Protocol-Version")),
			"accept":                  strings.TrimSpace(r.Header.Get("Accept")),
			"content_type":            strings.TrimSpace(r.Header.Get("Content-Type")),
			"user_agent":              truncateDiagnostic(strings.TrimSpace(r.UserAgent()), 180),
			"request_session_present": strings.TrimSpace(r.Header.Get("Mcp-Session-Id")) != "",
		},
		outputMeta: map[string]any{},
	}
}

func (d *mcpProtocolDiagnostic) fail(kind string) {
	d.status = "error"
	d.errorKind = kind
}

func (d *mcpProtocolDiagnostic) record(ctx context.Context, a *App) {
	if a == nil || a.audit == nil {
		return
	}
	principalID := d.principal.ID
	principalName := d.principal.DisplayName
	authChannel := d.principal.AuthChannel
	_ = a.audit.Record(ctx, mcpcenter.AuditEvent{
		RequestID:     d.requestID,
		Source:        "mcp-protocol",
		PrincipalID:   principalID,
		PrincipalName: principalName,
		AuthChannel:   authChannel,
		MCPID:         d.mcpID,
		ToolName:      "mcp/" + d.rpcMethod,
		StartedAt:     d.startedAt,
		DurationMS:    time.Since(d.startedAt).Milliseconds(),
		Status:        d.status,
		ErrorKind:     d.errorKind,
		InputMeta:     d.inputMeta,
		OutputMeta:    d.outputMeta,
	})
}

func truncateDiagnostic(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func mcpRPCMethod(r *http.Request) string {
	if r == nil || r.Body == nil || r.Method != http.MethodPost {
		return ""
	}
	body, err := readAndRestoreBody(r)
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		return ""
	}
	var envelope struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}
	return strings.TrimSpace(envelope.Method)
}

type mcpProtocolDiagnosticContextKey struct{}

func withMCPProtocolDiagnostic(ctx context.Context, diagnostic *mcpProtocolDiagnostic) context.Context {
	return context.WithValue(ctx, mcpProtocolDiagnosticContextKey{}, diagnostic)
}

func mcpProtocolDiagnosticFrom(ctx context.Context) *mcpProtocolDiagnostic {
	if ctx == nil {
		return nil
	}
	diagnostic, _ := ctx.Value(mcpProtocolDiagnosticContextKey{}).(*mcpProtocolDiagnostic)
	return diagnostic
}

func summarizeMCPToolsListResponse(diagnostic *mcpProtocolDiagnostic, body []byte) {
	if diagnostic == nil {
		return
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		diagnostic.outputMeta["json_parse_ok"] = false
		return
	}
	diagnostic.outputMeta["json_parse_ok"] = true
	if rpcError, ok := envelope["error"].(map[string]any); ok {
		diagnostic.fail("jsonrpc_error")
		if code, exists := rpcError["code"]; exists {
			diagnostic.outputMeta["jsonrpc_error_code"] = code
		}
		if message, ok := rpcError["message"].(string); ok {
			diagnostic.outputMeta["jsonrpc_error_message"] = truncateDiagnostic(message, 240)
		}
		return
	}
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		diagnostic.outputMeta["tools_result_present"] = false
		return
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		diagnostic.outputMeta["tools_result_present"] = false
		return
	}
	diagnostic.outputMeta["tools_result_present"] = true
	diagnostic.outputMeta["tool_count"] = len(tools)

	names := make([]string, 0, minInt(len(tools), 20))
	invalidSchemas := 0
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			invalidSchemas++
			continue
		}
		if name, ok := tool["name"].(string); ok && len(names) < 20 {
			names = append(names, name)
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			invalidSchemas++
			continue
		}
		if schemaType, _ := schema["type"].(string); schemaType != "" && schemaType != "object" {
			invalidSchemas++
		}
	}
	diagnostic.outputMeta["tool_names_sample"] = names
	diagnostic.outputMeta["invalid_input_schema_count"] = invalidSchemas
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
