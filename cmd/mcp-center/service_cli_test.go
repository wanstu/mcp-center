package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestServiceListenDefaultsAndUpdates(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "mcp-center")
	addr, origin, err := serviceListenFromFile(config)
	if err != nil || addr != "0.0.0.0:8610" || origin != "default" {
		t.Fatalf("default got %q %q %v", addr, origin, err)
	}
	original := []byte("# managed by administrator\nOTHER=preserve\nMCP_CENTER_LISTEN=0.0.0.0:8611\n")
	newData := replaceServiceListen(original, "0.0.0.0:8620")
	if bytes.Count(newData, []byte("MCP_CENTER_LISTEN=")) != 1 || !bytes.Contains(newData, []byte("OTHER=preserve")) {
		t.Fatal(string(newData))
	}
	if err := writeServiceEnvFile(config, newData); err != nil {
		t.Fatal(err)
	}
	addr, origin, err = serviceListenFromFile(config)
	if err != nil || addr != "0.0.0.0:8620" || origin != config {
		t.Fatalf("override got %q %q %v", addr, origin, err)
	}
	info, err := os.Stat(config)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Fatalf("file permission = %v", info.Mode())
	}
	if err := os.WriteFile(config, []byte("MCP_CENTER_LISTEN=garbage\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err = serviceListenFromFile(config)
	if err == nil {
		t.Fatal("invalid port must fail")
	}
}

func TestServiceCommandValidation(t *testing.T) {
	var out bytes.Buffer
	if err := runServiceCLI([]string{"help"}, &out); err != nil || !strings.Contains(out.String(), "set-port") {
		t.Fatalf("help = %v %s", err, out.String())
	}
	for _, value := range []string{"0", "65536", "foo", "-1"} {
		if err := runServiceCLI([]string{"set-port", value}, &out); err == nil {
			t.Fatalf("port %q passed", value)
		}
	}
	if err := runServiceCLI([]string{"unsupported"}, &out); err == nil {
		t.Fatal("unknown command accepted")
	}
	for _, args := range [][]string{{"autostart"}, {"autostart", "unknown"}, {"stop", "extra"}, {"set-port", "1", "extra"}} {
		if err := runServiceCLI(args, &out); err == nil {
			t.Fatalf("expected error: %v", args)
		}
	}
}
