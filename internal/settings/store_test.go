package settings

import (
	"path/filepath"
	"testing"
)

func TestStoreDefaultsPersistsAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	values, err := store.Get()
	if err != nil {
		t.Fatal(err)
	}
	if values.AuditRetentionDays != 30 {
		t.Fatalf("default retention = %d", values.AuditRetentionDays)
	}

	updated, err := store.Update(Values{AuditRetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AuditRetentionDays != 7 {
		t.Fatalf("updated retention = %d", updated.AuditRetentionDays)
	}

	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	values, err = reopened.Get()
	if err != nil {
		t.Fatal(err)
	}
	if values.AuditRetentionDays != 7 {
		t.Fatalf("persisted retention = %d", values.AuditRetentionDays)
	}

	for _, invalid := range []int{0, 3651} {
		if _, err := store.Update(Values{AuditRetentionDays: invalid}); err == nil {
			t.Fatalf("expected validation error for %d", invalid)
		}
	}
}
