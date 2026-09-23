package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestAuditAPIRequiresAdminAndDoesNotExposeArgumentValues(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	unauthorized := requestJSON(t, handler, http.MethodGet, "/api/audit", nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized audit status = %d", unauthorized.Code)
	}

	session := bootstrapSession(t, app)
	event := mcpcenter.AuditEvent{
		RequestID:     "req_audit_test",
		Source:        "console",
		PrincipalID:   "user_1",
		PrincipalName: "admin",
		AuthChannel:   "session",
		MCPID:         "mcp_1",
		ToolName:      "query",
		StartedAt:     time.Now().UTC(),
		DurationMS:    15,
		Status:        "success",
		InputMeta: map[string]any{
			"key_count":   1,
			"keys":        []string{"password"},
			"value_types": map[string]string{"password": "string"},
		},
	}
	if err := app.audit.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/audit?limit=10", nil, session)
	if list.Code != http.StatusOK {
		t.Fatalf("audit list = %d %s", list.Code, list.Body.String())
	}
	body := list.Body.String()
	for _, expected := range []string{"req_audit_test", "admin", "mcp_1", "query", "password"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("audit response missing %q: %s", expected, body)
		}
	}
	if strings.Contains(body, "super-secret-value") {
		t.Fatalf("audit leaked argument value: %s", body)
	}

	clear := requestJSON(t, handler, http.MethodDelete, "/api/audit", nil, session)
	if clear.Code != http.StatusOK {
		t.Fatalf("audit clear = %d %s", clear.Code, clear.Body.String())
	}
	list = requestJSON(t, handler, http.MethodGet, "/api/audit?limit=10", nil, session)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "\"events\":[]") {
		t.Fatalf("audit after clear = %d %s", list.Code, list.Body.String())
	}
}

func TestRequestIDPrefersValidIncomingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp/test", nil)
	req.Header.Set("X-Request-ID", "external-request-123")
	if got := requestID(req); got != "external-request-123" {
		t.Fatalf("request id = %q", got)
	}
}
