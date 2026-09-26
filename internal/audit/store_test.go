package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func TestSummarySinceCountsFullWindowAndRecentErrors(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	events := []mcpcenter.AuditEvent{
		{RequestID: "old", MCPID: "m1", ToolName: "old", StartedAt: now.Add(-25 * time.Hour), Status: "error", ErrorKind: "old_error"},
		{RequestID: "ok", MCPID: "m1", ToolName: "read", StartedAt: now.Add(-3 * time.Hour), Status: "success"},
		{RequestID: "err1", MCPID: "m2", ToolName: "call", StartedAt: now.Add(-2 * time.Hour), Status: "error", ErrorKind: "timeout"},
		{RequestID: "err2", MCPID: "m2", ToolName: "call2", StartedAt: now.Add(-time.Hour), Status: "error", ErrorKind: "auth_failure"},
	}
	for _, event := range events {
		if err := store.Record(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := store.SummarySince(now.Add(-24*time.Hour), 1)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 3 || summary.Success != 1 || summary.Errors != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if len(summary.RecentErrors) != 1 || summary.RecentErrors[0].RequestID != "err2" {
		t.Fatalf("unexpected recent errors: %#v", summary.RecentErrors)
	}
}

func TestRecordListAndClear(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := store.Record(context.Background(), mcpcenter.AuditEvent{
			RequestID: "req", MCPID: "mcp", ToolName: "tool",
			StartedAt: time.Now().Add(time.Duration(i) * time.Second), Status: "success",
		}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.List(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	items, err = store.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("after clear len = %d", len(items))
	}
}
