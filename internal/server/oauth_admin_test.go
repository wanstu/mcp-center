package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestOAuthClientAdminCRUDAndDynamicRegistration(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()
	session := bootstrapSession(t, app)

	metadata := requestJSON(t, handler, http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	if metadata.Code != http.StatusOK {
		t.Fatalf("metadata = %d %s", metadata.Code, metadata.Body.String())
	}
	if !strings.Contains(metadata.Body.String(), `"registration_endpoint":"http://127.0.0.1:8610/oauth/register"`) {
		t.Fatalf("metadata should advertise dynamic registration: %s", metadata.Body.String())
	}

	unauthorized := requestJSON(t, handler, http.MethodPost, "/api/oauth/clients", map[string]any{
		"client_name":   "Unauthorized",
		"redirect_uris": []string{"https://client.example/unauthorized"},
	})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized create = %d %s", unauthorized.Code, unauthorized.Body.String())
	}

	create := requestJSON(t, handler, http.MethodPost, "/api/oauth/clients", map[string]any{
		"client_name":   "Chat Client",
		"redirect_uris": []string{"https://client.example/callback"},
	}, session)
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	var created struct {
		Client struct {
			ID string `json:"client_id"`
		} `json:"client"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Client.ID == "" {
		t.Fatalf("missing client id: %s", create.Body.String())
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/oauth/clients", nil, session)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), created.Client.ID) || !strings.Contains(list.Body.String(), "Chat Client") {
		t.Fatalf("list = %d %s", list.Code, list.Body.String())
	}

	update := requestJSON(t, handler, http.MethodPut, "/api/oauth/clients/"+created.Client.ID, map[string]any{
		"client_name":   "Chat Client Updated",
		"redirect_uris": []string{"https://client.example/new-callback"},
	}, session)
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), "Chat Client Updated") {
		t.Fatalf("update = %d %s", update.Code, update.Body.String())
	}

	legacy := requestJSON(t, handler, http.MethodPost, "/oauth/register", map[string]any{
		"client_name":   "Dynamic Client",
		"redirect_uris": []string{"https://client.example/dynamic"},
	})
	if legacy.Code != http.StatusCreated {
		t.Fatalf("dynamic registration failed: %d %s", legacy.Code, legacy.Body.String())
	}
	list = requestJSON(t, handler, http.MethodGet, "/api/oauth/clients", nil, session)
	if !strings.Contains(list.Body.String(), "Dynamic Client") {
		t.Fatalf("registered client was not listed: %s", list.Body.String())
	}

	del := requestJSON(t, handler, http.MethodDelete, "/api/oauth/clients/"+created.Client.ID, nil, session)
	if del.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", del.Code, del.Body.String())
	}
	list = requestJSON(t, handler, http.MethodGet, "/api/oauth/clients", nil, session)
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), created.Client.ID) {
		t.Fatalf("client still present after delete: %d %s", list.Code, list.Body.String())
	}
}
