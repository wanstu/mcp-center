package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>MCP Center</title>")},
	}
	app, err := New(Config{
		DataDir: t.TempDir(),
		BaseURL: "http://127.0.0.1:8610",
		Assets:  fs.FS(assets),
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookie && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatal("session cookie not found")
	return nil
}

func TestBootstrapLoginAndMCPCRUD(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	setup := requestJSON(t, handler, http.MethodGet, "/api/setup", nil)
	if setup.Code != http.StatusOK || !bytes.Contains(setup.Body.Bytes(), []byte("\"needs_bootstrap\":true")) {
		t.Fatalf("setup status = %d %s", setup.Code, setup.Body.String())
	}

	register := requestJSON(t, handler, http.MethodPost, "/api/setup/register", map[string]any{
		"username": "admin",
		"password": "very-secure-password",
	})
	if register.Code != http.StatusCreated {
		t.Fatalf("register = %d %s", register.Code, register.Body.String())
	}
	session := sessionFrom(t, register)

	me := requestJSON(t, handler, http.MethodGet, "/api/me", nil, session)
	if me.Code != http.StatusOK || !bytes.Contains(me.Body.Bytes(), []byte("\"username\":\"admin\"")) {
		t.Fatalf("me = %d %s", me.Code, me.Body.String())
	}

	second := requestJSON(t, handler, http.MethodPost, "/api/setup/register", map[string]any{
		"username": "other",
		"password": "another-secure-password",
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("second bootstrap = %d %s", second.Code, second.Body.String())
	}

	add := requestJSON(t, handler, http.MethodPost, "/api/mcps", map[string]any{
		"name":      "example",
		"transport": "streamable-http",
		"auth_mode": "none",
		"endpoint":  "https://example.com/mcp",
		"health_policy": map[string]any{
			"health_check_enabled": false,
			"auto_reconnect":       false,
		},
	}, session)
	if add.Code != http.StatusCreated {
		t.Fatalf("add mcp = %d %s", add.Code, add.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(add.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	mcpValue, ok := payload["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("invalid add payload: %#v", payload)
	}
	id, _ := mcpValue["id"].(string)
	if id == "" {
		t.Fatal("new MCP id is empty")
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/mcps", nil, session)
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte("\"name\":\"example\"")) {
		t.Fatalf("list mcp = %d %s", list.Code, list.Body.String())
	}

	del := requestJSON(t, handler, http.MethodDelete, "/api/mcps/"+id, nil, session)
	if del.Code != http.StatusOK {
		t.Fatalf("delete mcp = %d %s", del.Code, del.Body.String())
	}
}

func TestAdminAPIRequiresSession(t *testing.T) {
	app := newTestApp(t)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/api/mcps", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestOAuthMetadataIsPublic(t *testing.T) {
	app := newTestApp(t)
	rec := requestJSON(t, app.Handler(), http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata = %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("\"code_challenge_methods_supported\":[\"S256\"]")) {
		t.Fatalf("metadata missing PKCE S256: %s", rec.Body.String())
	}
}
