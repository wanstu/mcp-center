package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// runServiceDoctor checks an installed Linux service without starting a
// second HTTP listener or altering the service.
func runServiceDoctor(out io.Writer) error {
	if err := showServiceStatus(out); err != nil {
		return fmt.Errorf("service inspection failed: %w", err)
	}
	props, err := getUnitProperties()
	if err != nil {
		return err
	}
	if props["ActiveState"] != "active" {
		return fmt.Errorf("service is not running (ActiveState=%s)", props["ActiveState"])
	}
	supported, err := serviceUnitSupportsPortFile()
	if err != nil {
		return err
	}
	if !supported {
		_, _ = fmt.Fprintln(out, "Port test skipped: custom/older systemd unit; upgrade to enable persistent port inspection.")
		return nil
	}
	address, _, err := serviceListenFromFile(serviceConfigPath)
	if err != nil {
		return err
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid service port %q", port)
	}
	local := net.JoinHostPort("127.0.0.1", port)
	conn, err := net.DialTimeout("tcp", local, 2*time.Second)
	if err != nil {
		return fmt.Errorf("service is active, but TCP %s is not reachable: %w", local, err)
	}
	conn.Close()
	_, _ = fmt.Fprintf(out, "TCP: 127.0.0.1:%s reachable\n", port)
	client := &http.Client{Timeout: 3 * time.Second}
	r, err := client.Get("http://" + local + "/api/setup")
	if err != nil {
		return fmt.Errorf("HTTP probe failed: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK && r.StatusCode != http.StatusForbidden && r.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("HTTP probe returned %s", r.Status)
	}
	if r.StatusCode == http.StatusOK {
		_, _ = fmt.Fprintln(out, "HTTP: /api/setup OK")
	} else {
		_, _ = fmt.Fprintf(out, "HTTP: service reachable, status %d (access policy may restrict localhost)\n", r.StatusCode)
	}
	return nil
}
