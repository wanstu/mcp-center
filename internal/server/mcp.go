package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func (a *App) handleMCPList(w http.ResponseWriter, _ *http.Request) {
	items, err := a.core.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mcps": items})
}

func (a *App) handleMCPAdd(w http.ResponseWriter, r *http.Request) {
	var cfg mcpcenter.Config
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := a.core.Add(cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"mcp": item})
}

func (a *App) handleMCPUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var cfg mcpcenter.Config
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := a.core.Update(id, cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidateProxy(id)
	writeJSON(w, http.StatusOK, map[string]any{"mcp": item})
}

func (a *App) handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.core.Remove(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	a.invalidateProxy(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleMCPProbe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	status, err := a.core.Probe(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *App) handleMCPTools(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tools, err := a.core.ListTools(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (a *App) handleMCPCall(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	result, err := a.core.CallTool(ctx, r.PathValue("id"), input.Name, input.Arguments)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleMCPProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		a.writeBearerChallenge(w, id)
		return
	}
	resource := a.baseURL + "/mcp/" + id
	grant, err := a.oauth.ValidateAccessTokenForResource(token, resource)
	if err != nil || !scopeContains(grant.Scope, "mcp:*") {
		a.writeBearerChallenge(w, id)
		return
	}
	handler, err := a.proxyHandler(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	handler.ServeHTTP(w, r)
}

func (a *App) proxyHandler(ctx context.Context, id string) (http.Handler, error) {
	a.proxyMu.Lock()
	if handler := a.proxies[id]; handler != nil {
		a.proxyMu.Unlock()
		return handler, nil
	}
	a.proxyMu.Unlock()

	buildCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	handler, err := a.core.NewProxyHTTPHandler(buildCtx, id)
	if err != nil {
		return nil, err
	}
	a.proxyMu.Lock()
	defer a.proxyMu.Unlock()
	if existing := a.proxies[id]; existing != nil {
		return existing, nil
	}
	a.proxies[id] = handler
	return handler, nil
}

func (a *App) invalidateProxy(id string) {
	a.proxyMu.Lock()
	delete(a.proxies, id)
	a.proxyMu.Unlock()
}

func (a *App) writeBearerChallenge(w http.ResponseWriter, id string) {
	metadata := a.baseURL + "/.well-known/oauth-protected-resource/mcp/" + id
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+metadata+`", scope="mcp:*"`)
	writeError(w, http.StatusUnauthorized, errors.New("valid OAuth bearer token with mcp:* scope is required"))
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

func scopeContains(scope, required string) bool {
	for _, item := range strings.Fields(scope) {
		if item == required {
			return true
		}
	}
	return false
}

// force json import to remain checked when CallToolResult evolves in the SDK.
var _ = json.Valid
