package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

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
