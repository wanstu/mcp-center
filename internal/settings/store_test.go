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

func TestPublicBaseURLPersistedAndValidated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "settings.json")
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	values, err := store.Get()
	if err != nil {
		t.Fatal(err)
	}
	if values.PublicBaseURL != "" {
		t.Fatal("new install must not assume public origin")
	}
	values.PublicBaseURL = "https://cmcps.wanstu.cn"
	if _, err = store.Update(values); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get()
	if err != nil || got.PublicBaseURL != "https://cmcps.wanstu.cn" {
		t.Fatalf("url not persisted %+v %v", got, err)
	}
	invalid := []string{"http://cmcps.wanstu.cn", "https://cmcps.wanstu.cn/path", "https://cmcps.wanstu.cn/", "https://cmcps.wanstu.cn?token=x", "https://user:pw@cmcps.wanstu.cn", "https://cmcps.wanstu.cn#section"}
	for _, v := range invalid {
		bad := got
		bad.PublicBaseURL = v
		if _, err := store.Update(bad); err == nil {
			t.Errorf("invalid URL accepted: %s", v)
		}
	}
	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	last, err := reloaded.Get()
	if err != nil || last.PublicBaseURL != "https://cmcps.wanstu.cn" {
		t.Fatalf("invalid writes mutated stored URL: %+v %v", last, err)
	}
}
