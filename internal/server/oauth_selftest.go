package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	oauthcore "github.com/wanstu/mcp-center-core/oauth"
)

const oauthSelfTestTTL = 15 * time.Minute

type oauthSelfTestStep struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type oauthSelfTest struct {
	ID        string              `json:"id"`
	MCPID     string              `json:"mcp_id"`
	MCPName   string              `json:"mcp_name"`
	Resource  string              `json:"resource"`
	Status    string              `json:"status"`
	Steps     []oauthSelfTestStep `json:"steps"`
	Error     string              `json:"error,omitempty"`
	ToolCount int                 `json:"tool_count,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	ExpiresAt time.Time           `json:"expires_at"`

	ClientID    string `json:"-"`
	RedirectURI string `json:"-"`
	Verifier    string `json:"-"`
	State       string `json:"-"`
}

type oauthSelfTestBearerTransport struct {
	Token string
	Base  http.RoundTripper
}

func (t oauthSelfTestBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("Authorization", "Bearer "+t.Token)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func oauthSelfTestSteps() []oauthSelfTestStep {
	return []oauthSelfTestStep{
		{Key: "pkce", Label: "临时 Client + PKCE", Status: "pending"},
		{Key: "authorize", Label: "浏览器授权", Status: "pending"},
		{Key: "token", Label: "Authorization Code 换取 Token", Status: "pending"},
		{Key: "mcp", Label: "OAuth Token 连接 MCP + tools/list", Status: "pending"},
		{Key: "refresh", Label: "Refresh Token 轮换", Status: "pending"},
		{Key: "refresh_mcp", Label: "新 Token 再次访问 MCP", Status: "pending"},
		{Key: "revoke", Label: "撤销 Token 并验证失效", Status: "pending"},
		{Key: "cleanup", Label: "清理临时 OAuth Client", Status: "pending"},
	}
}

func (a *App) handleOAuthTestStart(w http.ResponseWriter, r *http.Request) {
	if !a.requireOAuthEnabled(w) {
		return
	}
	var input struct {
		MCPRef string `json:"mcp_ref"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	def, err := a.core.GetByRef(input.MCPRef)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if def.Disabled {
		writeError(w, http.StatusConflict, errors.New("MCP 已禁用，无法执行 OAuth 自测"))
		return
	}

	id, err := oauthSelfTestRandom("ot", 18)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	state, err := oauthSelfTestRandom("state", 24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	verifier, err := oauthSelfTestRandom("", 48)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	redirectURI := strings.TrimRight(a.baseURL, "/") + "/oauth/test/callback"
	client, err := a.oauth.RegisterClient(oauthcore.ClientMetadata{
		ClientName:    "MCP Center OAuth Self-Test",
		RedirectURIs:  []string{redirectURI},
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("无法创建临时 OAuth Client：%w", err))
		return
	}

	now := time.Now().UTC()
	session := &oauthSelfTest{
		ID: id, MCPID: def.ID, MCPName: def.Name,
		Resource:  a.mcpResourceURL(def.Name),
		Status:    "waiting_authorization",
		Steps:     oauthSelfTestSteps(),
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(oauthSelfTestTTL),
		ClientID: client.ID, RedirectURI: redirectURI, Verifier: verifier, State: state,
	}
	oauthSelfTestSetStep(session, "pkce", "passed", "临时 Public Client 已创建，PKCE S256 已生成")
	oauthSelfTestSetStep(session, "authorize", "waiting", "请在新窗口中允许访问")

	a.oauthTestMu.Lock()
	a.pruneOAuthTestsLocked(now)
	a.oauthTests[id] = session
	a.oauthTestMu.Unlock()

	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", client.ID)
	values.Set("redirect_uri", redirectURI)
	values.Set("resource", session.Resource)
	values.Set("scope", "mcp:*")
	values.Set("state", state)
	values.Set("code_challenge", oauthcore.PKCEChallengeS256(verifier))
	values.Set("code_challenge_method", oauthcore.CodeChallengeMethodS256)
	authorizeURL := strings.TrimRight(a.baseURL, "/") + "/oauth/authorize?" + values.Encode()

	writeJSON(w, http.StatusCreated, map[string]any{
		"test":          oauthSelfTestPublic(session),
		"authorize_url": authorizeURL,
	})
}

func (a *App) handleOAuthTestStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	now := time.Now().UTC()

	var expiredClientID string
	a.oauthTestMu.Lock()
	a.pruneOAuthTestsLocked(now)
	session, ok := a.oauthTests[id]
	if ok && now.After(session.ExpiresAt) && session.Status != "passed" && session.Status != "failed" {
		session.Status = "failed"
		session.Error = "OAuth 自测已过期"
		session.UpdatedAt = now
		expiredClientID = session.ClientID
	}
	var public oauthSelfTest
	if ok {
		public = oauthSelfTestPublic(session)
	}
	a.oauthTestMu.Unlock()
	if expiredClientID != "" {
		_ = a.oauth.DeleteClient(expiredClientID)
	}

	if !ok {
		writeError(w, http.StatusNotFound, errors.New("OAuth 自测不存在或已清理"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"test": public})
}

func (a *App) handleOAuthTestCancel(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var clientID string
	a.oauthTestMu.Lock()
	if test := a.oauthTests[id]; test != nil {
		clientID = test.ClientID
		delete(a.oauthTests, id)
	}
	a.oauthTestMu.Unlock()
	if clientID != "" {
		_ = a.oauth.DeleteClient(clientID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleOAuthTestCallback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	session := a.oauthTestByState(state)
	if session == nil {
		writeOAuthSelfTestPage(w, "failed", "OAuth 自测状态无效或已过期")
		return
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		a.failOAuthSelfTest(session.ID, errors.New("OAuth 自测已过期"))
		writeOAuthSelfTestPage(w, "failed", "OAuth 自测已过期")
		return
	}
	if oauthErr := strings.TrimSpace(r.URL.Query().Get("error")); oauthErr != "" {
		detail := oauthErr
		if desc := strings.TrimSpace(r.URL.Query().Get("error_description")); desc != "" {
			detail += ": " + desc
		}
		a.failOAuthSelfTest(session.ID, errors.New(detail))
		writeOAuthSelfTestPage(w, "failed", detail)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		a.failOAuthSelfTest(session.ID, errors.New("授权回调缺少 code"))
		writeOAuthSelfTestPage(w, "failed", "授权回调缺少 code")
		return
	}

	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		test.Status = "running"
		oauthSelfTestSetStep(test, "authorize", "passed", "浏览器授权完成并收到 Authorization Code")
		oauthSelfTestSetStep(test, "token", "running", "")
	})

	tokenSet, _, err := a.exchangeOAuthToken(url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {session.ClientID},
		"code":          {code},
		"redirect_uri":  {session.RedirectURI},
		"resource":      {session.Resource},
		"code_verifier": {session.Verifier},
	})
	if err != nil {
		a.failOAuthSelfTest(session.ID, fmt.Errorf("Token Exchange 失败：%w", err))
		writeOAuthSelfTestPage(w, "failed", "Token Exchange 失败")
		return
	}
	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		oauthSelfTestSetStep(test, "token", "passed", "Access Token / Refresh Token 已签发")
		oauthSelfTestSetStep(test, "mcp", "running", "")
	})

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	toolCount, err := probeOAuthMCP(ctx, session.Resource, tokenSet.AccessToken)
	cancel()
	if err != nil {
		a.failOAuthSelfTest(session.ID, fmt.Errorf("OAuth Token 访问 MCP 失败：%w", err))
		writeOAuthSelfTestPage(w, "failed", "OAuth Token 访问 MCP 失败")
		return
	}
	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		test.ToolCount = toolCount
		oauthSelfTestSetStep(test, "mcp", "passed", fmt.Sprintf("MCP initialize / tools/list 成功，发现 %d 个工具", toolCount))
		oauthSelfTestSetStep(test, "refresh", "running", "")
	})

	refreshed, _, err := a.exchangeOAuthToken(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {session.ClientID},
		"refresh_token": {tokenSet.RefreshToken},
	})
	if err != nil {
		a.failOAuthSelfTest(session.ID, fmt.Errorf("Refresh Token 失败：%w", err))
		writeOAuthSelfTestPage(w, "failed", "Refresh Token 失败")
		return
	}
	if _, err := a.oauth.ValidateAccessToken(tokenSet.AccessToken); err == nil {
		a.failOAuthSelfTest(session.ID, errors.New("Refresh 后旧 Access Token 仍然有效"))
		writeOAuthSelfTestPage(w, "failed", "Refresh Token 轮换验证失败")
		return
	}
	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		oauthSelfTestSetStep(test, "refresh", "passed", "Refresh Token 已轮换，旧 Access Token 已失效")
		oauthSelfTestSetStep(test, "refresh_mcp", "running", "")
	})

	ctx, cancel = context.WithTimeout(r.Context(), 20*time.Second)
	refreshedToolCount, err := probeOAuthMCP(ctx, session.Resource, refreshed.AccessToken)
	cancel()
	if err != nil {
		a.failOAuthSelfTest(session.ID, fmt.Errorf("刷新后的 Token 访问 MCP 失败：%w", err))
		writeOAuthSelfTestPage(w, "failed", "刷新后的 Token 访问 MCP 失败")
		return
	}
	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		oauthSelfTestSetStep(test, "refresh_mcp", "passed", fmt.Sprintf("新 Access Token 正常访问 MCP，发现 %d 个工具", refreshedToolCount))
		oauthSelfTestSetStep(test, "revoke", "running", "")
	})

	if err := a.revokeOAuthToken(url.Values{
		"client_id": {session.ClientID},
		"token":     {refreshed.AccessToken},
	}); err != nil {
		a.failOAuthSelfTest(session.ID, fmt.Errorf("Token 撤销失败：%w", err))
		writeOAuthSelfTestPage(w, "failed", "Token 撤销失败")
		return
	}
	if _, err := a.oauth.ValidateAccessTokenForResource(refreshed.AccessToken, session.Resource); err == nil {
		a.failOAuthSelfTest(session.ID, errors.New("已撤销 Access Token 仍然有效"))
		writeOAuthSelfTestPage(w, "failed", "Token 撤销验证失败")
		return
	}
	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		oauthSelfTestSetStep(test, "revoke", "passed", "Access Token 已撤销并确认失效")
		oauthSelfTestSetStep(test, "cleanup", "running", "")
	})

	if err := a.oauth.DeleteClient(session.ClientID); err != nil {
		a.failOAuthSelfTest(session.ID, fmt.Errorf("清理临时 OAuth Client 失败：%w", err))
		writeOAuthSelfTestPage(w, "failed", "临时 Client 清理失败")
		return
	}
	a.updateOAuthTest(session.ID, func(test *oauthSelfTest) {
		oauthSelfTestSetStep(test, "cleanup", "passed", "临时 OAuth Client 与授权已清理")
		test.Status = "passed"
		test.Error = ""
	})

	writeOAuthSelfTestPage(w, "passed", "OAuth 自测通过。可以关闭此页面并返回 MCP Center。")
}

func probeOAuthMCP(ctx context.Context, resource, token string) (int, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-center-oauth-self-test", Version: "1.0.0"}, nil)
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
		Transport: oauthSelfTestBearerTransport{
			Token: token,
		},
	}
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             resource,
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		return 0, err
	}
	defer session.Close()
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return 0, err
	}
	return len(result.Tools), nil
}

func oauthSelfTestRandom(prefix string, bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	if prefix != "" {
		return prefix + "_" + value, nil
	}
	return value, nil
}

func oauthSelfTestSetStep(test *oauthSelfTest, key, status, detail string) {
	for i := range test.Steps {
		if test.Steps[i].Key == key {
			test.Steps[i].Status = status
			test.Steps[i].Detail = detail
			test.UpdatedAt = time.Now().UTC()
			return
		}
	}
}

func oauthSelfTestPublic(test *oauthSelfTest) oauthSelfTest {
	if test == nil {
		return oauthSelfTest{}
	}
	copy := *test
	copy.ClientID = ""
	copy.RedirectURI = ""
	copy.Verifier = ""
	copy.State = ""
	copy.Steps = append([]oauthSelfTestStep(nil), test.Steps...)
	return copy
}

func (a *App) oauthTestByState(state string) *oauthSelfTest {
	a.oauthTestMu.Lock()
	defer a.oauthTestMu.Unlock()
	now := time.Now().UTC()
	a.pruneOAuthTestsLocked(now)
	for _, test := range a.oauthTests {
		if test.State == state {
			copy := oauthSelfTestPublic(test)
			copy.ClientID = test.ClientID
			copy.RedirectURI = test.RedirectURI
			copy.Verifier = test.Verifier
			copy.State = test.State
			return &copy
		}
	}
	return nil
}

func (a *App) updateOAuthTest(id string, fn func(*oauthSelfTest)) {
	a.oauthTestMu.Lock()
	defer a.oauthTestMu.Unlock()
	if test := a.oauthTests[id]; test != nil {
		fn(test)
		test.UpdatedAt = time.Now().UTC()
	}
}

func (a *App) failOAuthSelfTest(id string, err error) {
	var clientID string
	a.oauthTestMu.Lock()
	if test := a.oauthTests[id]; test != nil {
		test.Status = "failed"
		test.Error = err.Error()
		test.UpdatedAt = time.Now().UTC()
		for i := range test.Steps {
			if test.Steps[i].Status == "running" || test.Steps[i].Status == "waiting" {
				test.Steps[i].Status = "failed"
				test.Steps[i].Detail = err.Error()
				break
			}
		}
		clientID = test.ClientID
	}
	a.oauthTestMu.Unlock()
	if clientID != "" {
		_ = a.oauth.DeleteClient(clientID)
	}
}

func (a *App) pruneOAuthTestsLocked(now time.Time) {
	for id, test := range a.oauthTests {
		if now.Sub(test.UpdatedAt) > time.Hour {
			if test.ClientID != "" {
				_ = a.oauth.DeleteClient(test.ClientID)
			}
			delete(a.oauthTests, id)
		}
	}
}

func writeOAuthSelfTestPage(w http.ResponseWriter, status, message string) {
	title := "OAuth 自测完成"
	if status != "passed" {
		title = "OAuth 自测失败"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>body{margin:0;background:#f6f7f9;color:#172033;font:14px "Segoe UI","Microsoft YaHei UI",sans-serif}.shell{min-height:100vh;display:grid;place-items:center;padding:24px}.card{width:min(520px,100%%);background:#fff;border:1px solid #dde3eb;border-radius:18px;padding:28px;box-shadow:0 18px 60px rgba(15,23,42,.08)}h1{margin:0 0 10px;font-size:22px}p{color:#647286;line-height:1.7}.status{display:inline-block;padding:5px 10px;border-radius:999px;background:%s;color:%s;font-weight:700}</style></head><body><div class="shell"><div class="card"><span class="status">%s</span><h1>%s</h1><p>%s</p><p>返回 MCP Center 后测试结果会自动刷新。</p></div></div></body></html>`,
		title,
		map[bool]string{true: "#e8f7ed", false: "#fdecec"}[status == "passed"],
		map[bool]string{true: "#18794e", false: "#b42318"}[status == "passed"],
		map[bool]string{true: "通过", false: "失败"}[status == "passed"],
		title,
		message,
	)
}
