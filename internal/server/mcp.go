package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func (a *App) handleMCPState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Disabled bool `json:"disabled"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := a.core.SetDisabled(id, input.Disabled)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	a.invalidateProxy(id)
	writeJSON(w, http.StatusOK, map[string]any{"mcp": item})
}

func (a *App) ensureMCPEnabled(id string) error {
	item, err := a.core.Get(id)
	if err != nil {
		return err
	}
	if item.Disabled {
		return errors.New("MCP 已禁用")
	}
	return nil
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
	if err := a.ensureMCPEnabled(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
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
	if err := a.ensureMCPEnabled(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
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
	if err := a.ensureMCPEnabled(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	var input struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	auditCtx := auditContextForAdmin(r, user.ID, user.Username)
	ctx, cancel := context.WithTimeout(auditCtx, 60*time.Second)
	defer cancel()
	result, err := a.core.CallTool(ctx, r.PathValue("id"), input.Name, input.Arguments)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleMCPProxy(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("id")
	diagnostic := newMCPDiscoveryDiagnostic(r, ref)
	defer func() {
		diagnostic.record(r.Context(), a)
	}()

	def, err := a.core.GetByRef(ref)
	if err != nil {
		diagnostic.fail("mcp_not_found")
		diagnostic.outputMeta["http_status"] = http.StatusNotFound
		writeError(w, http.StatusNotFound, err)
		return
	}
	diagnostic.mcpID = def.ID
	if def.Disabled {
		diagnostic.fail("mcp_disabled")
		diagnostic.outputMeta["http_status"] = http.StatusServiceUnavailable
		writeError(w, http.StatusServiceUnavailable, errors.New("MCP 已禁用"))
		return
	}

	principal, err := a.authenticateMCPRequest(r, ref)
	if err != nil {
		diagnostic.fail("authentication")
		diagnostic.outputMeta["http_status"] = http.StatusUnauthorized
		a.writeBearerChallenge(w, r, ref)
		return
	}
	diagnostic.principal = principal
	diagnostic.inputMeta["principal_kind"] = principal.Kind
	diagnostic.inputMeta["auth_channel"] = principal.AuthChannel

	r = r.WithContext(auditContextForMCP(r, principal))
	r = r.WithContext(withMCPDiscoveryDiagnostic(r.Context(), &diagnostic))

	cached := a.proxyCached(def.ID)
	diagnostic.outputMeta["proxy_cached"] = cached
	handler, err := a.proxyHandler(r.Context(), def.ID)
	if err != nil {
		diagnostic.fail("proxy_build")
		diagnostic.outputMeta["http_status"] = http.StatusBadGateway
		writeError(w, http.StatusBadGateway, err)
		return
	}

	tracked := &mcpDiagnosticResponseWriter{ResponseWriter: w}
	withOpenAIToolOAuthMetadata(handler).ServeHTTP(tracked, r)
	if tracked.status == 0 {
		tracked.status = http.StatusOK
	}
	diagnostic.outputMeta["http_status"] = tracked.status
	diagnostic.outputMeta["response_session_present"] = strings.TrimSpace(tracked.Header().Get("Mcp-Session-Id")) != ""
	diagnostic.outputMeta["response_content_type"] = strings.TrimSpace(tracked.Header().Get("Content-Type"))
	if tracked.status >= http.StatusBadRequest {
		diagnostic.fail("proxy_http")
	}
}

func (a *App) proxyCached(id string) bool {
	a.proxyMu.Lock()
	defer a.proxyMu.Unlock()
	return a.proxies[id] != nil
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
	server, err := a.core.NewProxyServer(buildCtx, id)
	if err != nil {
		return nil, err
	}
	handler := newAuthenticatedProxyHTTPHandler(server)
	a.proxyMu.Lock()
	defer a.proxyMu.Unlock()
	if existing := a.proxies[id]; existing != nil {
		return existing, nil
	}
	a.proxies[id] = handler
	return handler, nil
}

func newAuthenticatedProxyHTTPHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		SessionTimeout:               5 * time.Minute,
		PropagateRequestCancellation: true,
		MaxRequestBodyBytes:          4 << 20,
		DisableLocalhostProtection:   true,
	})
}

func (a *App) invalidateProxy(id string) {
	a.proxyMu.Lock()
	delete(a.proxies, id)
	a.proxyMu.Unlock()
}

func (a *App) invalidateAllProxies() {
	a.proxyMu.Lock()
	a.proxies = map[string]http.Handler{}
	a.proxyMu.Unlock()
}

func (a *App) writeBearerChallenge(w http.ResponseWriter, r *http.Request, ref string) {
	metadata := a.mcpProtectedResourceMetadataURLForRequest(r, ref)
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+metadata+`", scope="mcp:*"`)
	writeError(w, http.StatusUnauthorized, errors.New("valid OAuth token or MCP access credential with mcp:* scope is required"))
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
