package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

func (a *App) handleUpstreamSecretList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"secrets": a.upstreamSecrets.List()})
}

// Reveal is an explicit admin-only action. The list endpoint never exposes
// decrypted secret values, and the response must not be cached by clients.
func (a *App) handleUpstreamSecretReveal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	name, err := url.PathUnescape(strings.TrimSpace(r.PathValue("name")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	value, ok := a.upstreamSecrets.Lookup(name)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("变量不存在"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "value": value})
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
