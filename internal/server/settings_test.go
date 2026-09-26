package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestSettingsAPIUpdatesAuditRetentionAndPrunesImmediately(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	get := requestJSON(t, handler, http.MethodGet, "/api/settings", nil, session)
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "\"audit_retention_days\":30") {
		t.Fatalf("default settings = %d %s", get.Code, get.Body.String())
	}

	now := time.Now().UTC()
	for _, event := range []mcpcenter.AuditEvent{
		{RequestID: "old", MCPID: "m1", ToolName: "old", StartedAt: now.Add(-10 * 24 * time.Hour), Status: "success"},
		{RequestID: "keep", MCPID: "m1", ToolName: "keep", StartedAt: now.Add(-time.Hour), Status: "success"},
	} {
		if err := app.audit.Record(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	update := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{
		"audit_retention_days": 7,
	}, session)
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), "\"audit_retention_days\":7") {
		t.Fatalf("update settings = %d %s", update.Code, update.Body.String())
	}
	items, err := app.audit.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RequestID != "keep" {
		t.Fatalf("retention did not prune immediately: %#v", items)
	}

	invalid := requestJSON(t, handler, http.MethodPut, "/api/settings", map[string]any{
		"audit_retention_days": 0,
	}, session)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid retention = %d %s", invalid.Code, invalid.Body.String())
	}
}
