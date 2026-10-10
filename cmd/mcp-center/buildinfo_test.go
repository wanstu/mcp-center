package main

import (
	"testing"

	"github.com/wanstu/mcp-center/internal/buildinfo"
)

func TestBuildInfoReportsDesktopKitDependency(t *testing.T) {
	if got := buildinfo.Current().KitVersion; got != "v0.11.3" {
		t.Fatalf("kit version = %q, want v0.11.3", got)
	}
}
