package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestConnectionInfoAPI(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	rec := requestJSON(t, handler, http.MethodGet, "/api/connection-info", nil, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("connection info = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"\"base_url\":\"http://127.0.0.1:8610\"",
		"\"mcp_endpoint_template\":\"http://127.0.0.1:8610/mcp/{id}\"",
		"\"mcp_id_endpoint_template\":\"http://127.0.0.1:8610/mcp/{id}\"",
		"\"mcp_name_endpoint_template\":\"http://127.0.0.1:8610/mcp/{name}\"",
		"\"is_loopback\":true",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in %s", expected, body)
		}
	}
}

func TestConnectionInfoReportsActualListenAddress(t *testing.T) {
	app := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, listener, err := app.ListenAndServe(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	session := bootstrapSession(t, app)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/api/connection-info", nil, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("connection info = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"listen_address\":\""+listener.Addr().String()+"\"") {
		t.Fatalf("actual listener not reported: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"data_dir\":") || !strings.Contains(rec.Body.String(), "\"restart_required_for\"") {
		t.Fatalf("runtime configuration incomplete: %s", rec.Body.String())
	}
}

func TestConnectionInfoRequiresAdmin(t *testing.T) {
	app := newTestApp(t)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/api/connection-info", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
