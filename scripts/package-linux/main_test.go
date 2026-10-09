package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxPackageLicenseMetadata(t *testing.T) {
	root := filepath.Join("..", "..")
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(license), "BSD 3-Clause License") ||
		!strings.Contains(string(license), "Copyright (c) 2026, wanstu") ||
		!strings.Contains(string(license), "Neither the name of the copyright holder") {
		t.Fatal("BSD-3-Clause copyright notice or conditions missing")
	}
	data, err := os.ReadFile(filepath.Join(root, "packaging", "mcp-center.metainfo.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		XMLName         xml.Name `xml:"component"`
		Type            string   `xml:"type,attr"`
		ID              string   `xml:"id"`
		ProjectLicense  string   `xml:"project_license"`
		MetadataLicense string   `xml:"metadata_license"`
		Binary          string   `xml:"provides>binary"`
	}
	if err := xml.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Type != "console-application" || manifest.ID != "com.github.wanstu.mcp-center" ||
		manifest.ProjectLicense != "BSD-3-Clause" || manifest.MetadataLicense != "CC0-1.0" ||
		manifest.Binary != "mcp-center" {
		t.Fatalf("invalid AppStream metadata: %+v", manifest)
	}
}
