package upstreamsecret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithCreatedBatchRollbackCollisionAndEncryptedDisk(t *testing.T) {
	root := t.TempDir()
	location := filepath.Join(root, "secrets.json")
	store, err := New(location, filepath.Join(root, "master.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Set("SHARED_KEY", "kept"); err != nil {
		t.Fatal(err)
	}
	fail := errors.New("simulate core import failure")
	err = store.WithCreatedBatch(map[string]string{"APP_TOKEN": "sensitive-rollback-test"}, func() error { return fail })
	if !errors.Is(err, fail) {
		t.Fatalf("failed apply should propagate original error: %v", err)
	}
	if _, exists := store.Lookup("APP_TOKEN"); exists {
		t.Fatal("rollback left plaintext variable in memory")
	}
	disk, err := os.ReadFile(location)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(disk), "sensitive-rollback-test") || strings.Contains(string(disk), "kept") {
		t.Fatal("secret file leaked plaintext")
	}
	reopened, err := New(location, filepath.Join(root, "master.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := reopened.Lookup("APP_TOKEN"); exists {
		t.Fatal("failed apply left variable on disk")
	}
	if got, exists := reopened.Lookup("SHARED_KEY"); !exists || got != "kept" {
		t.Fatal("rollback removed old secret")
	}
	called := false
	err = store.WithCreatedBatch(map[string]string{"SHARED_KEY": "should-not-override"}, func() error { called = true; return nil })
	if err == nil || called {
		t.Fatal("existing variable overwritten or apply called")
	}
	if got, _ := store.Lookup("SHARED_KEY"); got != "kept" {
		t.Fatal("collision overwrote secret")
	}
	err = store.WithCreatedBatch(map[string]string{"SHARED_KEY": "kept", "APP_TOKEN": "saved-value"}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := store.Lookup("APP_TOKEN"); !ok || got != "saved-value" {
		t.Fatal("successful batch did not persist")
	}
	reloaded, err := New(location, filepath.Join(root, "master.key"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := reloaded.Lookup("APP_TOKEN"); !ok || got != "saved-value" {
		t.Fatal("successful batch not encrypted/persisted")
	}
}
