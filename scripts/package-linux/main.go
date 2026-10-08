// package-linux builds MCP Center Linux packages using the reusable Desktop Kit
// packaging module. This standalone command is also useful before a tagged release.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile("^v[0-9]+\\.[0-9]+\\.[0-9]+(?:[-.][0-9A-Za-z.-]+)?$")

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "linux packaging:", err)
		os.Exit(1)
	}
}
func run() error {
	version := flag.String("version", "", "version such as v0.1.1-rc.1")
	kit := flag.String("kit", "../wails-desktop-kit", "path to reusable Kit repository")
	out := flag.String("out", "", "output directory, default dist/<version>")
	build := flag.Bool("build", false, "also cross-compile Linux raw binaries from current source")
	flag.Parse()
	if !versionPattern.MatchString(*version) {
		return fmt.Errorf("invalid -version %q", *version)
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	kitRoot, err := filepath.Abs(*kit)
	if err != nil {
		return err
	}
	if st, e := os.Stat(filepath.Join(kitRoot, "packaging", "linux_systemd.go")); e != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("Kit systemd packaging is unavailable at %s", kitRoot)
	}
	tag := "v0.10.1"
	resolved := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", kitRoot}, args...)...)
		result, err := cmd.Output()
		return strings.TrimSpace(string(result)), err
	}
	tagCommit, err := resolved("rev-list", "-n", "1", tag)
	if err != nil || tagCommit == "" {
		return fmt.Errorf("Kit packaging requires tag %s in %s", tag, kitRoot)
	}
	head, err := resolved("rev-parse", "HEAD")
	if err != nil || head != tagCommit {
		return fmt.Errorf("Kit packaging requires checkout %s (HEAD %s)", tag, head)
	}
	dest := *out
	if dest == "" {
		dest = filepath.Join("dist", *version)
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	debVersion := strings.TrimPrefix(*version, "v")
	if i := strings.IndexByte(debVersion, '-'); i >= 0 {
		debVersion = debVersion[:i] + "~" + debVersion[i+1:]
	}
	for _, arch := range []string{"amd64", "arm64"} {
		raw := filepath.Join(dest, "mcp-center-"+*version+"-linux-"+arch)
		if *build {
			cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w -X github.com/wanstu/mcp-center/internal/buildinfo.Version="+*version, "-o", raw, "./cmd/mcp-center")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			fmt.Printf("Building Linux %s binary...\n", arch)
			if err = cmd.Run(); err != nil {
				return fmt.Errorf("Linux %s binary: %w", arch, err)
			}
		} else {
			if _, err = os.Stat(raw); err != nil {
				return fmt.Errorf("missing existing Linux binary %s: %w", raw, err)
			}
		}
		args := []string{"-C", kitRoot, "run", "./cmd/desktopkit", "package", "linux",
			"--input", raw, "--dist", dest, "--app-name", "mcp-center",
			"--asset-base", "mcp-center-" + *version, "--package-name", "mcp-center",
			"--package-version", debVersion, "--arch", arch,
			"--description", "MCP Center MCP gateway and web control service",
			"--maintainer", "MCP Center", "--depends", "systemd, passwd",
			"--formats", "deb", "--systemd", "--service-name", "mcp-center",
			"--service-user", "mcp-center", "--service-group", "mcp-center",
			"--service-data-dir", "/var/lib/mcp-center",
			"--service-arg=--listen", "--service-arg=0.0.0.0:8610",
			"--service-arg=--data-dir", "--service-arg=/var/lib/mcp-center"}
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Packaging Linux %s systemd .deb via Kit...\n", arch)
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("Linux %s .deb: %w", arch, err)
		}
	}
	return nil
}
