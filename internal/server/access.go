package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	accesscore "github.com/wanstu/mcp-center-core/access"
)

type mcpPrincipal struct {
	ID          string
	DisplayName string
	Kind        string
	Scopes      []string
	AuthChannel string
}

type credentialCandidate struct {
	Secret  string
	Channel string
}

func (a *App) handleAccessSettingsGet(w http.ResponseWriter, _ *http.Request) {
	settings, err := a.access.Settings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (a *App) handleAccessSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	var input accesscore.Settings
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	settings, err := a.access.UpdateSettings(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (a *App) handleAccessCredentialList(w http.ResponseWriter, _ *http.Request) {
	items, err := a.access.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credentials": items})
}

func (a *App) handleAccessCredentialCreate(w http.ResponseWriter, r *http.Request) {
	var input accesscore.CreateInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := a.access.Create(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *App) handleAccessCredentialUpdate(w http.ResponseWriter, r *http.Request) {
	var input accesscore.UpdateInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := a.access.Update(r.PathValue("id"), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credential": item})
}

func (a *App) handleAccessCredentialDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.access.Delete(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) authenticateMCPRequest(r *http.Request, id string) (mcpPrincipal, error) {
	settings, err := a.access.Settings()
	if err != nil {
		return mcpPrincipal{}, err
	}

	candidates := make([]credentialCandidate, 0, 3)
	if settings.BearerEnabled {
		if token, ok := bearerToken(r.Header.Get("Authorization")); ok {
			candidates = append(candidates, credentialCandidate{Secret: token, Channel: accesscore.ChannelBearer})
		}
	}
	if settings.HeaderEnabled {
		if value := strings.TrimSpace(r.Header.Get(settings.HeaderName)); value != "" {
			candidates = append(candidates, credentialCandidate{Secret: value, Channel: accesscore.ChannelHeader})
		}
	}
	if settings.QueryEnabled {
		if value := strings.TrimSpace(r.URL.Query().Get(settings.QueryName)); value != "" {
			candidates = append(candidates, credentialCandidate{Secret: value, Channel: accesscore.ChannelQuery})
		}
	}
	if len(candidates) == 0 {
		return mcpPrincipal{}, errors.New("no enabled MCP credential was provided")
	}
	if len(candidates) > 1 {
		return mcpPrincipal{}, errors.New("multiple MCP credentials were provided")
	}

	candidate := candidates[0]
	resource := a.baseURL + "/mcp/" + id
	if candidate.Channel == accesscore.ChannelBearer {
		grant, oauthErr := a.oauth.ValidateAccessTokenForResource(candidate.Secret, resource)
		if oauthErr == nil {
			if !scopeContains(grant.Scope, "mcp:*") {
				return mcpPrincipal{}, errors.New("OAuth token does not grant mcp:*")
			}
			return mcpPrincipal{
				ID: grant.UserID, DisplayName: grant.ClientID, Kind: "oauth",
				Scopes: strings.Fields(grant.Scope), AuthChannel: accesscore.ChannelBearer,
			}, nil
		}
	}

	principal, err := a.access.Verify(candidate.Secret, candidate.Channel)
	if err != nil {
		return mcpPrincipal{}, err
	}
	if !accesscore.ScopeContains(principal.Scopes, "mcp:*") {
		return mcpPrincipal{}, fmt.Errorf("credential %q does not grant mcp:*", principal.ID)
	}
	return mcpPrincipal{
		ID: principal.ID, DisplayName: principal.DisplayName, Kind: "access_credential",
		Scopes: principal.Scopes, AuthChannel: principal.AuthChannel,
	}, nil
}
