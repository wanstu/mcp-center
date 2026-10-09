package main

import (
	"fmt"
	"io"
	"strings"
)

const commandHelp = `MCP Center

Usage:
  mcp-center serve [--listen ADDRESS] [--data-dir PATH] [--base-url URL]
  mcp-center status
  mcp-center doctor
  sudo mcp-center start|stop|restart
  sudo mcp-center set-port PORT
  sudo mcp-center autostart on|off
  mcp-center --version

Service management uses systemd internally on Linux. Existing startup flags
and the older 'mcp-center service <command>' form remain supported.
`

// dispatchCLI runs non-server commands before the HTTP server and its
// listener can be constructed. Never interpret an unknown positional
// argument as permission to start another server process.
func dispatchCLI(args []string, out io.Writer) ([]string, bool, error) {
	if len(args) == 0 {
		return nil, false, nil
	} // Backwards-compatible foreground start
	command := args[0]
	switch command {
	case "help":
		if len(args) > 1 {
			return nil, true, fmt.Errorf("usage: mcp-center help")
		}
		_, _ = fmt.Fprint(out, commandHelp)
		return nil, true, nil
	case "serve":
		return args[1:], false, nil
	case "service":
		return nil, true, runServiceCLI(args[1:], out)
	case "doctor":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("usage: mcp-center doctor")
		}
		return nil, true, runServiceDoctor(out)
	case "status", "start", "stop", "restart", "set-port", "autostart":
		return nil, true, runServiceCLI(args, out)
	default:
		if strings.HasPrefix(command, "-") {
			return args, false, nil
		}
		return nil, true, fmt.Errorf("unknown command %q; run 'mcp-center help'", command)
	}
}
