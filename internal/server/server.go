package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
	accesscore "github.com/wanstu/mcp-center-core/access"
	account "github.com/wanstu/mcp-center-core/auth"
	oauthcore "github.com/wanstu/mcp-center-core/oauth"
	auditstore "github.com/wanstu/mcp-center/internal/audit"
)

const sessionCookie = "mcp_center_session"

type Config struct {
	DataDir string
	BaseURL string
	Assets  fs.FS
}

type App struct {
	core    *mcpcenter.Service
	access  *accesscore.Service
	account *account.Service
	oauth   *oauthcore.Service
	audit   *auditstore.Store
	assets  fs.FS
	baseURL string
	secure  bool

	proxyMu sync.Mutex
	proxies map[string]http.Handler
}

func New(cfg Config) (*App, error) {
	if strings.TrimSpace(cfg.DataDir) == "" {
		return nil, errors.New("server: data directory is required")
	}
	if cfg.Assets == nil {
		return nil, errors.New("server: assets are required")
	}
	base, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	store, err := mcpcenter.NewFileStore(filepath.Join(cfg.DataDir, "mcps.json"))
	if err != nil {
		return nil, err
	}
	accounts, err := account.New(filepath.Join(cfg.DataDir, "accounts.json"))
	if err != nil {
		return nil, err
	}
	oauthService, err := oauthcore.New(filepath.Join(cfg.DataDir, "oauth.json"))
	if err != nil {
		return nil, err
	}
	accessService, err := accesscore.New(filepath.Join(cfg.DataDir, "access.json"))
	if err != nil {
		return nil, err
	}
	auditLog, err := auditstore.New(filepath.Join(cfg.DataDir, "audit.jsonl"))
	if err != nil {
		return nil, err
	}
	coreService := mcpcenter.New(store)
	coreService.SetAuditSink(auditLog)
	return &App{
		core: coreService, access: accessService, account: accounts, oauth: oauthService, audit: auditLog,
		assets: cfg.Assets, baseURL: base, secure: strings.HasPrefix(base, "https://"),
		proxies: map[string]http.Handler{},
	}, nil
}

func (a *App) Core() *mcpcenter.Service    { return a.core }
func (a *App) Access() *accesscore.Service { return a.access }
func (a *App) Account() *account.Service   { return a.account }
func (a *App) OAuth() *oauthcore.Service   { return a.oauth }
func (a *App) BaseURL() string             { return a.baseURL }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/setup", a.handleSetupStatus)
	mux.HandleFunc("POST /api/setup/register", a.handleSetupRegister)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.Handle("GET /api/me", a.requireAdmin(http.HandlerFunc(a.handleMe)))
	mux.Handle("POST /api/account/password", a.requireAdmin(http.HandlerFunc(a.handleChangePassword)))

	mux.Handle("GET /api/mcps", a.requireAdmin(http.HandlerFunc(a.handleMCPList)))
	mux.Handle("POST /api/mcps", a.requireAdmin(http.HandlerFunc(a.handleMCPAdd)))
	mux.Handle("PUT /api/mcps/{id}", a.requireAdmin(http.HandlerFunc(a.handleMCPUpdate)))
	mux.Handle("DELETE /api/mcps/{id}", a.requireAdmin(http.HandlerFunc(a.handleMCPDelete)))
	mux.Handle("POST /api/mcps/{id}/probe", a.requireAdmin(http.HandlerFunc(a.handleMCPProbe)))
	mux.Handle("GET /api/mcps/{id}/tools", a.requireAdmin(http.HandlerFunc(a.handleMCPTools)))
	mux.Handle("POST /api/mcps/{id}/call", a.requireAdmin(http.HandlerFunc(a.handleMCPCall)))
	mux.Handle("POST /api/mcps/import", a.requireAdmin(http.HandlerFunc(a.handleMCPImport)))
	mux.Handle("GET /api/mcps/export", a.requireAdmin(http.HandlerFunc(a.handleMCPExport)))

	mux.Handle("GET /api/runtime", a.requireAdmin(http.HandlerFunc(a.handleRuntimeList)))
	mux.Handle("POST /api/runtime/{id}/refresh", a.requireAdmin(http.HandlerFunc(a.handleRuntimeRefresh)))
	mux.Handle("POST /api/runtime/{id}/disconnect", a.requireAdmin(http.HandlerFunc(a.handleRuntimeDisconnect)))

	mux.Handle("GET /api/access/settings", a.requireAdmin(http.HandlerFunc(a.handleAccessSettingsGet)))
	mux.Handle("PUT /api/access/settings", a.requireAdmin(http.HandlerFunc(a.handleAccessSettingsUpdate)))
	mux.Handle("GET /api/access/credentials", a.requireAdmin(http.HandlerFunc(a.handleAccessCredentialList)))
	mux.Handle("POST /api/access/credentials", a.requireAdmin(http.HandlerFunc(a.handleAccessCredentialCreate)))
	mux.Handle("PUT /api/access/credentials/{id}", a.requireAdmin(http.HandlerFunc(a.handleAccessCredentialUpdate)))
	mux.Handle("DELETE /api/access/credentials/{id}", a.requireAdmin(http.HandlerFunc(a.handleAccessCredentialDelete)))
	mux.Handle("GET /api/audit", a.requireAdmin(http.HandlerFunc(a.handleAuditList)))
	mux.Handle("DELETE /api/audit", a.requireAdmin(http.HandlerFunc(a.handleAuditClear)))
	mux.Handle("GET /api/oauth/clients", a.requireAdmin(http.HandlerFunc(a.handleOAuthClientList)))
	mux.Handle("POST /api/oauth/clients", a.requireAdmin(http.HandlerFunc(a.handleOAuthClientCreate)))
	mux.Handle("PUT /api/oauth/clients/{id}", a.requireAdmin(http.HandlerFunc(a.handleOAuthClientUpdate)))
	mux.Handle("DELETE /api/oauth/clients/{id}", a.requireAdmin(http.HandlerFunc(a.handleOAuthClientDelete)))

	mux.HandleFunc("GET /.well-known/oauth-authorization-server", a.handleAuthorizationServerMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", a.handleProtectedResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/{rest...}", a.handleProtectedResourceMetadata)
	mux.HandleFunc("GET /oauth/authorize", a.handleOAuthAuthorizeGet)
	mux.HandleFunc("POST /oauth/authorize", a.handleOAuthAuthorizePost)
	mux.HandleFunc("POST /oauth/token", a.handleOAuthToken)
	mux.HandleFunc("POST /oauth/revoke", a.handleOAuthRevoke)

	mux.HandleFunc("/mcp/{id}", a.handleMCPProxy)
	mux.HandleFunc("/mcp/{id}/", a.handleMCPProxy)

	static := http.FileServer(http.FS(a.assets))
	mux.Handle("/", static)

	return a.securityHeaders(a.cors(mux))
}

func (a *App) ListenAndServe(ctx context.Context, listen string) (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, nil, err
	}
	srv := &http.Server{
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       90 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	go func() {
		_ = srv.Serve(listener)
	}()
	return srv, listener, nil
}

func (a *App) handleSetupStatus(w http.ResponseWriter, _ *http.Request) {
	needed, err := a.account.NeedsBootstrap()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"needs_bootstrap": needed})
}

func (a *App) handleSetupRegister(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := a.account.BootstrapAdmin(input.Username, input.Password)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	token, _, err := a.account.Login(input.Username, input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	token, user, err := a.account.Login(input.Username, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	a.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = a.account.Logout(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: a.secure, MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.currentUser(r)
		if err != nil || user.Role != "admin" {
			writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) currentUser(r *http.Request) (account.User, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return account.User{}, err
	}
	return a.account.Authenticate(cookie.Value)
}

func (a *App) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: a.secure, MaxAge: 24 * 60 * 60,
	})
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self' data:; connect-src 'self' http://127.0.0.1:* http://localhost:*")
		next.ServeHTTP(w, r)
	})
}

func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://wails.localhost" || origin == "https://wails.localhost" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		raw = "http://127.0.0.1:8610"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("server: invalid base URL %q", raw)
	}
	return raw, nil
}

func decodeJSON(r *http.Request, dst any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("content-type must be application/json")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nilResponseWriter{}, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

// nilResponseWriter is used only to satisfy MaxBytesReader where no early 413
// response is needed. JSON endpoints still reject oversized bodies on read.
type nilResponseWriter struct{}

func (nilResponseWriter) Header() http.Header       { return make(http.Header) }
func (nilResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (nilResponseWriter) WriteHeader(int)           {}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}
