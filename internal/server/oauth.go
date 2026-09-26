package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

var authorizeTemplate = template.Must(template.New("authorize").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>MCP Center OAuth</title>
<style>
body{margin:0;background:#f6f7f9;color:#172033;font:14px "Segoe UI","Microsoft YaHei UI",sans-serif}
.shell{min-height:100vh;display:grid;place-items:center;padding:24px}.card{width:min(480px,100%);background:#fff;border:1px solid #dde3eb;border-radius:18px;padding:28px;box-shadow:0 18px 60px rgba(15,23,42,.08)}
.logo{width:42px;height:42px;border-radius:12px;background:#2563eb;color:#fff;display:grid;place-items:center;font-weight:800;margin-bottom:20px}
h1{font-size:22px;margin:0 0 8px}p{color:#647286;line-height:1.65;margin:0 0 20px}.meta{background:#f8fafc;border:1px solid #edf0f4;border-radius:11px;padding:12px;margin:14px 0;word-break:break-all}.meta strong{display:block;margin-bottom:5px}
label{display:grid;gap:6px;margin:12px 0;color:#526173;font-weight:650}input{border:1px solid #dde3eb;border-radius:9px;padding:10px;outline:none}input:focus{border-color:#2563eb;box-shadow:0 0 0 3px rgba(37,99,235,.12)}
.actions{display:flex;gap:9px;margin-top:18px}.btn{flex:1;border:1px solid #d9e0e8;background:#fff;border-radius:9px;padding:10px 12px;font-weight:700;cursor:pointer}.btn.primary{background:#2563eb;border-color:#2563eb;color:#fff}
.small{font-size:11px;color:#8a96a6;margin-top:16px}
</style></head><body><div class="shell"><div class="card"><div class="logo">M</div>
{{if .Login}}
<h1>登录 MCP Center</h1><p>完成登录后继续 OAuth 授权。管理后台 Session 不会作为 MCP Access Token 使用。</p>
<form method="post" action="/oauth/authorize">
{{template "hidden" .}}
<label>用户名<input name="username" autocomplete="username" required></label>
<label>密码<input name="password" type="password" autocomplete="current-password" required></label>
<div class="actions"><button class="btn primary" type="submit">登录并继续</button></div>
</form>
{{else}}
<h1>授权 MCP 客户端</h1><p><strong>{{.ClientName}}</strong> 请求访问 MCP Center 中的一个 MCP 服务。</p>
<div class="meta"><strong>资源</strong>{{.Resource}}</div>
<div class="meta"><strong>权限</strong>{{.Scope}}</div>
<form method="post" action="/oauth/authorize">
{{template "hidden" .}}
<div class="actions"><button class="btn" name="decision" value="deny" type="submit">拒绝</button><button class="btn primary" name="decision" value="approve" type="submit">允许访问</button></div>
</form>
<div class="small">授权码使用 PKCE S256，Access Token 绑定到上方资源。</div>
{{end}}
</div></div></body></html>
{{define "hidden"}}
<input type="hidden" name="response_type" value="{{.ResponseType}}">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="resource" value="{{.Resource}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
{{end}}`))

type authorizeView struct {
	Login               bool
	ClientName          string
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Resource            string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

func (a *App) handleAuthorizationServerMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         a.baseURL,
		"authorization_endpoint":                         a.baseURL + "/oauth/authorize",
		"token_endpoint":                                 a.baseURL + "/oauth/token",
		"revocation_endpoint":                            a.baseURL + "/oauth/revoke",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"scopes_supported":                               []string{"mcp:*"},
		"authorization_response_iss_parameter_supported": true,
	})
}

func (a *App) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	resource := a.baseURL + "/mcp"
	suffix := strings.TrimPrefix(r.URL.Path, "/.well-known/oauth-protected-resource")
	if suffix != "" && suffix != "/" {
		resource = a.baseURL + suffix
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 resource,
		"authorization_servers":    []string{a.baseURL},
		"scopes_supported":         []string{"mcp:*"},
		"bearer_methods_supported": []string{"header"},
	})
}

func (a *App) handleOAuthAuthorizeGet(w http.ResponseWriter, r *http.Request) {
	view, err := a.authorizeViewFromValues(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	client, err := a.oauth.ValidateAuthorization(view.ClientID, view.RedirectURI, view.CodeChallenge, view.CodeChallengeMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	view.ClientName = client.ClientName
	if view.ClientName == "" {
		view.ClientName = client.ID
	}
	if _, err := a.currentUser(r); err != nil {
		view.Login = true
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := authorizeTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) handleOAuthAuthorizePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	view, err := a.authorizeViewFromValues(r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := a.oauth.ValidateAuthorization(view.ClientID, view.RedirectURI, view.CodeChallenge, view.CodeChallengeMethod); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, sessionErr := a.currentUser(r)
	if sessionErr != nil && r.PostForm.Get("username") != "" {
		token, loggedIn, err := a.account.Login(r.PostForm.Get("username"), r.PostForm.Get("password"))
		if err != nil {
			view.Login = true
			view.ClientName = "OAuth 客户端"
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = authorizeTemplate.Execute(w, view)
			return
		}
		a.setSessionCookie(w, token)
		user = loggedIn
		sessionErr = nil
	}
	if sessionErr != nil {
		http.Redirect(w, r, "/oauth/authorize?"+view.query().Encode(), http.StatusSeeOther)
		return
	}

	if r.PostForm.Get("decision") == "deny" {
		redirectOAuthResult(w, r, view.RedirectURI, map[string]string{
			"error": "access_denied", "state": view.State, "iss": a.baseURL,
		})
		return
	}
	if r.PostForm.Get("decision") != "approve" {
		http.Redirect(w, r, "/oauth/authorize?"+view.query().Encode(), http.StatusSeeOther)
		return
	}

	code, err := a.oauth.Approve(oauthcore.AuthorizationRequest{
		UserID: user.ID, ClientID: view.ClientID, RedirectURI: view.RedirectURI,
		Resource: view.Resource, Scope: view.Scope,
		CodeChallenge: view.CodeChallenge, CodeChallengeMethod: view.CodeChallengeMethod,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirectOAuthResult(w, r, view.RedirectURI, map[string]string{
		"code": code, "state": view.State, "iss": a.baseURL,
	})
}

func (a *App) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid form")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
	if clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id is required")
		return
	}

	var (
		set oauthcore.TokenSet
		err error
	)
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		set, err = a.oauth.ExchangeCode(
			clientID,
			r.PostForm.Get("code"),
			r.PostForm.Get("redirect_uri"),
			r.PostForm.Get("resource"),
			r.PostForm.Get("code_verifier"),
		)
	case "refresh_token":
		set, err = a.oauth.Refresh(clientID, r.PostForm.Get("refresh_token"))
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported grant_type")
		return
	}
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (a *App) handleOAuthRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid form")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	clientID := strings.TrimSpace(r.PostForm.Get("client_id"))
	token := strings.TrimSpace(r.PostForm.Get("token"))
	if clientID == "" || token == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id and token are required")
		return
	}
	if err := a.oauth.RevokeToken(clientID, token); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *App) authorizeViewFromValues(values url.Values) (authorizeView, error) {
	view := authorizeView{
		ResponseType:        values.Get("response_type"),
		ClientID:            strings.TrimSpace(values.Get("client_id")),
		RedirectURI:         values.Get("redirect_uri"),
		Resource:            strings.TrimRight(values.Get("resource"), "/"),
		Scope:               strings.TrimSpace(values.Get("scope")),
		State:               values.Get("state"),
		CodeChallenge:       values.Get("code_challenge"),
		CodeChallengeMethod: values.Get("code_challenge_method"),
	}
	if view.ResponseType != "code" {
		return authorizeView{}, errors.New("response_type must be code")
	}
	if view.ClientID == "" || view.RedirectURI == "" || view.CodeChallenge == "" {
		return authorizeView{}, errors.New("client_id, redirect_uri and code_challenge are required")
	}
	if view.CodeChallengeMethod != oauthcore.CodeChallengeMethodS256 {
		return authorizeView{}, errors.New("code_challenge_method must be S256")
	}
	if view.Scope == "" {
		view.Scope = "mcp:*"
	}
	if view.Scope != "mcp:*" {
		return authorizeView{}, errors.New("this version supports only mcp:* scope")
	}
	if !a.isLocalMCPResource(view.Resource) {
		return authorizeView{}, errors.New("resource must identify an MCP Center /mcp/{id} endpoint")
	}
	return view, nil
}

func (a *App) isLocalMCPResource(resource string) bool {
	prefix := a.baseURL + "/mcp/"
	return strings.HasPrefix(resource, prefix) && len(strings.TrimPrefix(resource, prefix)) > 0
}

func (v authorizeView) query() url.Values {
	values := url.Values{}
	values.Set("response_type", v.ResponseType)
	values.Set("client_id", v.ClientID)
	values.Set("redirect_uri", v.RedirectURI)
	values.Set("resource", v.Resource)
	values.Set("scope", v.Scope)
	values.Set("state", v.State)
	values.Set("code_challenge", v.CodeChallenge)
	values.Set("code_challenge_method", v.CodeChallengeMethod)
	return values
}

func redirectOAuthResult(w http.ResponseWriter, r *http.Request, target string, params map[string]string) {
	u, err := url.Parse(target)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	query := u.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	u.RawQuery = query.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]any{
		"error": code, "error_description": description,
	})
}
