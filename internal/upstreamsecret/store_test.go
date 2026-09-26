package upstreamsecret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedStoreRoundTripAndNoPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upstream-secrets.json")
	keyPath := filepath.Join(dir, "upstream-secrets.key")

	store, err := New(path, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Set("MCP_TOKEN", "super-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "MCP_TOKEN" || info.CreatedAt.IsZero() || info.UpdatedAt.IsZero() {
		t.Fatalf("unexpected info: %#v", info)
	}
	if got, ok := store.Lookup("MCP_TOKEN"); !ok || got != "super-secret-value" {
		t.Fatalf("lookup = %q, %v", got, ok)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret-value") {
		t.Fatalf("state leaked plaintext secret: %s", raw)
	}

	reopened, err := New(path, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := reopened.Lookup("MCP_TOKEN"); !ok || got != "super-secret-value" {
		t.Fatalf("reopened lookup = %q, %v", got, ok)
	}
	items := reopened.List()
	if len(items) != 1 || items[0].Name != "MCP_TOKEN" {
		t.Fatalf("unexpected list: %#v", items)
	}
	if err := reopened.Delete("MCP_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.Lookup("MCP_TOKEN"); ok {
		t.Fatal("deleted secret still resolves")
	}
}

func TestStoreValidationAndReplacement(t *testing.T) {
	dir := t.TempDir()
	store, err := New(filepath.Join(dir, "state.json"), filepath.Join(dir, "key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set("1BAD", "value"); err == nil {
		t.Fatal("expected invalid name error")
	}
	if _, err := store.Set("EMPTY", ""); err == nil {
		t.Fatal("expected empty value error")
	}
	first, err := store.Set("TOKEN", "one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Set("TOKEN", "two")
	if err != nil {
		t.Fatal(err)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) || !second.UpdatedAt.After(first.UpdatedAt) && !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("replacement timestamps unexpected: first=%#v second=%#v", first, second)
	}
	if got, _ := store.Lookup("TOKEN"); got != "two" {
		t.Fatalf("replacement value = %q", got)
	}
}
