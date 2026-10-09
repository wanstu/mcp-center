package main

import (
	"fmt"
	"io"
	"strings"
)

const commandHelp = `MCP Center

Usage:
  mcp-center status                  Show installed service, autostart and port
  mcp-center doctor                  Check service and local HTTP availability
  sudo mcp-center start              Start installed service
  sudo mcp-center stop               Stop installed service
  sudo mcp-center restart            Restart installed service
  sudo mcp-center set-port PORT      Persist service port (1-65535)
  sudo mcp-center autostart on|off   Enable or disable boot autostart

  mcp-center serve [options]        Run HTTP server in the foreground
  mcp-center [options]              Same as serve (legacy)
  mcp-center --version              Print version
  mcp-center -h | --help | help      Show this help

Foreground server options:
  --listen ADDRESS    HTTP listen address (default: 0.0.0.0:8610)
  --data-dir PATH     Persistent data directory (default: user config directory)
  --base-url URL      Optional canonical public URL for OAuth metadata
  --version           Print version and exit

Installed Linux services use systemd internally. The legacy
'mcp-center service <command>' form remains supported.
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
	case "help", "-h", "--help":
		if len(args) > 1 {
			return nil, true, fmt.Errorf("usage: mcp-center help")
		}
		_, _ = fmt.Fprint(out, commandHelp)
		return nil, true, nil
	case "serve":
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			_, _ = fmt.Fprint(out, commandHelp)
			return nil, true, nil
		}
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
