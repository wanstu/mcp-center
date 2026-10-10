package buildinfo

import (
	"runtime"
	"testing"
)

func TestCurrentIncludesRuntimeDiagnostics(t *testing.T) {
	info := Current()
	if info.Platform != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", info.Platform, runtime.GOOS)
	}
	if info.Architecture != runtime.GOARCH {
		t.Fatalf("architecture = %q, want %q", info.Architecture, runtime.GOARCH)
	}
	if info.GoVersion == "" {
		t.Fatal("go version is empty")
	}
}
