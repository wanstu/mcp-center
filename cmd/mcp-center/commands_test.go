package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIRoutesWithoutStartingServer(t *testing.T) {
	for _, command := range []string{"status", "stop", "doctor", "start", "restart", "set-port", "autostart"} {
		// Every command must dispatch independently of server creation.
		// Validation arguments short circuit invalid invocations before systemctl.
		if command == "doctor" {
			_, done, err := dispatchCLI([]string{"doctor", "extra"}, &bytes.Buffer{})
			if !done || err == nil {
				t.Fatal("doctor should reject trailing args")
			}
			continue
		}
		switch command {
		case "status":
			_, done, err := dispatchCLI([]string{"status", "extra"}, &bytes.Buffer{})
			if !done || err == nil {
				t.Fatal("status should reject trailing args")
			}
		case "set-port":
			_, done, err := dispatchCLI([]string{"set-port", "wrong"}, &bytes.Buffer{})
			if !done || err == nil {
				t.Fatal("set-port must reject invalid ports")
			}
		case "autostart":
			_, done, err := dispatchCLI([]string{"autostart", "invalid"}, &bytes.Buffer{})
			if !done || err == nil {
				t.Fatal("autostart must reject invalid setting")
			}
		default:
			_, done, err := dispatchCLI([]string{command, "unexpected"}, &bytes.Buffer{})
			if !done || err == nil {
				t.Fatalf("%s trailing args must fail", command)
			}
		}
	}
	for _, command := range []string{"foo", "statuz", "doctro", "--listen=8610", "serve", "service", "--version"} {
		var out bytes.Buffer
		args, done, err := dispatchCLI([]string{command}, &out)
		switch command {
		case "foo", "statuz", "doctro":
			if !done || err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("%s accepted unexpectedly: done=%v err=%v", command, done, err)
			}
		case "service":
			if !done || err != nil || !strings.Contains(out.String(), "service status") {
				t.Fatalf("old service form was broken: err=%v text=%q", err, out.String())
			}
		case "serve", "--listen=8610", "--version":
			if done || err != nil {
				t.Fatalf("foreground command %s: done=%v err=%v", command, done, err)
			}
			if command == "serve" && len(args) != 0 {
				t.Fatalf("serve args=%v", args)
			}
		}
	}
	var out bytes.Buffer
	_, done, err := dispatchCLI([]string{"help"}, &out)
	if !done || err != nil || !strings.Contains(out.String(), "mcp-center doctor") {
		t.Fatal("help missing CLI commands")
	}
	args, done, err := dispatchCLI([]string{"serve", "--listen", "0.0.0.0:8686"}, &out)
	if done || err != nil || strings.Join(args, ",") != "--listen,0.0.0.0:8686" {
		t.Fatalf("serve did not keep flags: %v %v", args, err)
	}
}

func TestUnifiedHelpFlagsAndServeHelp(t *testing.T) {
	for _, arguments := range [][]string{
		{"help"}, {"-h"}, {"--help"}, {"serve", "-h"}, {"serve", "--help"},
	} {
		var out bytes.Buffer
		serverArgs, handled, err := dispatchCLI(arguments, &out)
		if err != nil || !handled || len(serverArgs) != 0 {
			t.Fatalf("%q must display help without starting server: handled=%v serverArgs=%v err=%v", arguments, handled, serverArgs, err)
		}
		for _, expected := range []string{
			"mcp-center status", "mcp-center doctor", "mcp-center set-port PORT",
			"mcp-center autostart on|off", "mcp-center serve [options]",
			"--listen ADDRESS", "--data-dir PATH", "--base-url URL", "--version",
		} {
			if !strings.Contains(out.String(), expected) {
				t.Errorf("%q help missing %q", arguments, expected)
			}
		}
	}
	for _, arguments := range [][]string{{"-h", "unexpected"}, {"--help", "unexpected"}} {
		_, handled, err := dispatchCLI(arguments, &bytes.Buffer{})
		if !handled || err == nil {
			t.Fatalf("unexpected extra arguments accepted: %v", arguments)
		}
	}
}
