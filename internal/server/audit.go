package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func (a *App) handleAuditList(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	items, err := a.audit.List(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": items})
}

func (a *App) handleAuditClear(w http.ResponseWriter, _ *http.Request) {
	if err := a.audit.Clear(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func auditContextForAdmin(r *http.Request, userID, username string) context.Context {
	return mcpcenter.WithAuditContext(r.Context(), mcpcenter.AuditContext{
		RequestID:     requestID(r),
		Source:        "console",
		PrincipalID:   userID,
		PrincipalName: username,
		AuthChannel:   "session",
	})
}

func auditContextForMCP(r *http.Request, principal mcpPrincipal) context.Context {
	return mcpcenter.WithAuditContext(r.Context(), mcpcenter.AuditContext{
		RequestID:     requestID(r),
		Source:        "proxy",
		PrincipalID:   principal.ID,
		PrincipalName: principal.DisplayName,
		AuthChannel:   principal.AuthChannel,
	})
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" && len(value) <= 128 {
		return value
	}
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "req_" + hex.EncodeToString(raw[:])
	}
	return "req_unknown"
}
