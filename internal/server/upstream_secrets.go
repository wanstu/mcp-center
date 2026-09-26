package server

import (
	"net/http"
	"net/url"
	"strings"
)

func (a *App) handleUpstreamSecretList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"secrets": a.upstreamSecrets.List()})
}

func (a *App) handleUpstreamSecretSet(w http.ResponseWriter, r *http.Request) {
	name, err := url.PathUnescape(strings.TrimSpace(r.PathValue("name")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := a.upstreamSecrets.Set(name, input.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.core.DisconnectAll()
	a.invalidateAllProxies()
	writeJSON(w, http.StatusOK, map[string]any{"secret": info})
}

func (a *App) handleUpstreamSecretDelete(w http.ResponseWriter, r *http.Request) {
	name, err := url.PathUnescape(strings.TrimSpace(r.PathValue("name")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.upstreamSecrets.Delete(name); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	a.core.DisconnectAll()
	a.invalidateAllProxies()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
