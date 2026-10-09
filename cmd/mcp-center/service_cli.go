package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wanstu/wails-desktop-kit/servicecontrol"
)

const serviceName = "mcp-center.service"
const serviceConfigPath = "/etc/default/mcp-center"
const defaultServiceListen = "0.0.0.0:8610"

// The service CLI only manages systemd installations. Existing foreground
// --listen, --data-dir, --base-url and --version flags remain unchanged.
func runServiceCLI(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, _ = fmt.Fprintln(out, "Usage:\n  mcp-center service status\n  sudo mcp-center service set-port <1-65535>\n  sudo mcp-center service autostart on|off\n  sudo mcp-center service start|stop|restart\n\n  The installed systemd service listens on all interfaces by default.\n  Direct foreground runs still use --listen / --data-dir / --base-url.")
		return nil
	}
	if runtime.GOOS != "linux" {
		return errors.New("systemd service management is only supported on Linux")
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("usage: mcp-center service status")
		}
		return showServiceStatus(out)
	case "set-port":
		if len(args) != 2 {
			return fmt.Errorf("usage: sudo mcp-center service set-port <1-65535>")
		}
		port, err := strconv.Atoi(args[1])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("port must be an integer from 1 to 65535")
		}
		return updateServicePort(out, port)
	case "autostart":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			return errors.New("usage: sudo mcp-center service autostart on|off")
		}
		command := "enable"
		if args[1] == "off" {
			command = "disable"
		}
		return controlService(out, command)
	case "start", "stop", "restart":
		if len(args) != 1 {
			return fmt.Errorf("usage: sudo mcp-center service %s", args[0])
		}
		return controlService(out, args[0])
	default:
		return fmt.Errorf("unknown service command %q (use mcp-center service help)", args[0])
	}
}

// Delegate standard Linux service actions to the reusable Kit servicecontrol.
// Center owns only its application-specific port configuration and diagnostics.
func controlService(out io.Writer, action string) error {
	return servicecontrol.Run(context.Background(), serviceName, action, out, os.Stderr)
}

func systemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	// Avoid indefinite waits if systemd is unavailable or stuck.
	done := make(chan error, 1)
	var b strings.Builder
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Start(); err != nil {
		return "", err
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return strings.TrimSpace(b.String()), err
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return "", fmt.Errorf("systemctl timed out")
	}
}

func getUnitProperties() (map[string]string, error) {
	res, err := systemctl("show", "--no-pager", "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState", serviceName)
	if err != nil {
		return nil, fmt.Errorf("systemctl show: %w (%s)", err, res)
	}
	props := map[string]string{}
	for _, line := range strings.Split(res, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			props[parts[0]] = parts[1]
		}
	}
	if props["LoadState"] != "loaded" {
		return nil, fmt.Errorf("%s not installed (LoadState=%s)", serviceName, props["LoadState"])
	}
	return props, nil
}

func serviceListenFromFile(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultServiceListen, "default", nil
	}
	if err != nil {
		return "", "", err
	}
	found := false
	val := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MCP_CENTER_LISTEN=") {
			val = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "MCP_CENTER_LISTEN=")), `"'`)
			found = true
		}
	}
	if !found {
		return defaultServiceListen, "default", nil
	}
	host, port, err := net.SplitHostPort(val)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || n < 1 || n > 65535 || host != "0.0.0.0" {
		return "", "", fmt.Errorf("invalid MCP_CENTER_LISTEN in %s: %q", path, val)
	}
	return val, path, nil
}

// A previous package version used a fixed ExecStart without EnvironmentFile.
// Never accept set-port for a unit that cannot read the persistent setting.
func serviceUnitSupportsPortFile() (bool, error) {
	body, err := systemctl("cat", serviceName)
	if err != nil {
		return false, fmt.Errorf("read systemd unit: %w (%s)", err, body)
	}
	return strings.Contains(body, "EnvironmentFile=-"+serviceConfigPath) &&
		strings.Contains(body, "${MCP_CENTER_LISTEN}"), nil
}

func showServiceStatus(out io.Writer) error {
	props, err := getUnitProperties()
	if err != nil {
		return err
	}
	supports, err := serviceUnitSupportsPortFile()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Service: %s\nLoaded: %s\nAutostart: %s\nActive: %s\n", serviceName, props["LoadState"], props["UnitFileState"], props["ActiveState"])
	if !supports {
		_, _ = fmt.Fprintln(out, "Listen: managed by an older/custom systemd unit; see systemctl status mcp-center")
		_, _ = fmt.Fprintln(out, "Port management: upgrade to a package that supports /etc/default/mcp-center")
		return nil
	}
	addr, source, err := serviceListenFromFile(serviceConfigPath)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Configured listen: %s (%s)\n", addr, source)
	if props["ActiveState"] == "active" {
		_, _ = fmt.Fprintln(out, "Note: inspect systemctl status for the currently running process.")
	}
	return nil
}

func updateServicePort(out io.Writer, port int) error {
	if os.Geteuid() != 0 {
		return errors.New("requires root: sudo mcp-center service set-port <port>")
	}
	if _, err := getUnitProperties(); err != nil {
		return err
	}
	supported, err := serviceUnitSupportsPortFile()
	if err != nil {
		return err
	}
	if !supported {
		return errors.New("installed unit does not support port configuration; upgrade the mcp-center .deb first (custom ExecStart overrides may also need removing)")
	}
	target := fmt.Sprintf("0.0.0.0:%d", port)
	old, err := os.ReadFile(serviceConfigPath)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return err
	}
	updated := replaceServiceListen(old, target)
	if err = writeServiceEnvFile(serviceConfigPath, updated); err != nil {
		return err
	}
	// This rereads the EnvironmentFile without daemon-reload. try-restart
	// preserves an intentionally stopped service.
	result, err := systemctl("try-restart", serviceName)
	if err != nil {
		var rollback error
		if missing {
			rollback = os.Remove(serviceConfigPath)
		} else {
			rollback = writeServiceEnvFile(serviceConfigPath, old)
		}
		_, _ = systemctl("try-restart", serviceName)
		if rollback != nil {
			return fmt.Errorf("restart failed (%s): %w; rollback failed: %v", result, err, rollback)
		}
		return fmt.Errorf("restart failed, original port restored (%s): %w", result, err)
	}
	_, _ = fmt.Fprintf(out, "Configured %s to listen on %s\n", serviceName, target)
	_, _ = fmt.Fprintln(out, "Running services were restarted. If the service was stopped, the new port applies at next start.")
	return nil
}

func replaceServiceListen(original []byte, value string) []byte {
	lines := strings.Split(string(original), "\n")
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "MCP_CENTER_LISTEN=") {
			continue
		}
		out = append(out, line)
	}
	payload := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if payload != "" {
		payload += "\n"
	}
	return []byte(payload + "MCP_CENTER_LISTEN=" + value + "\n")
}

func writeServiceEnvFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".mcp-center-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
