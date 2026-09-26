package server

import (
	"net/http"

	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

func (a *App) handleOAuthClientList(w http.ResponseWriter, _ *http.Request) {
	items, err := a.oauth.ListClients()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clients": items})
}

func (a *App) handleOAuthClientCreate(w http.ResponseWriter, r *http.Request) {
	var metadata oauthcore.ClientMetadata
	if err := decodeJSON(r, &metadata); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client, err := a.oauth.RegisterClient(metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"client": client})
}

func (a *App) handleOAuthClientUpdate(w http.ResponseWriter, r *http.Request) {
	var metadata oauthcore.ClientMetadata
	if err := decodeJSON(r, &metadata); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client, err := a.oauth.UpdateClient(r.PathValue("id"), metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": client})
}

func (a *App) handleOAuthClientDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.oauth.DeleteClient(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
