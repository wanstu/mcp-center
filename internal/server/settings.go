package server

import (
	"errors"
	"net/http"
)

func (a *App) oauthEnabled() (bool, error) {
	values, err := a.settings.Get()
	if err != nil {
		return false, err
	}
	return !values.OAuthDisabled, nil
}

func (a *App) requireOAuthEnabled(w http.ResponseWriter) bool {
	enabled, err := a.oauthEnabled()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return false
	}
	if !enabled {
		writeError(w, http.StatusServiceUnavailable, errors.New("OAuth 已关闭"))
		return false
	}
	return true
}

func (a *App) handleSettingsGet(w http.ResponseWriter, _ *http.Request) {
	values, err := a.settings.Get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": values})
}

func (a *App) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AuditRetentionDays   *int      `json:"audit_retention_days,omitempty"`
		OAuthDisabled        *bool     `json:"oauth_disabled,omitempty"`
		HostWhitelistEnabled *bool     `json:"host_whitelist_enabled,omitempty"`
		AllowedHosts         *[]string `json:"allowed_hosts,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	current, err := a.settings.Get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if input.AuditRetentionDays != nil {
		current.AuditRetentionDays = *input.AuditRetentionDays
	}
	if input.OAuthDisabled != nil {
		current.OAuthDisabled = *input.OAuthDisabled
	}
	if input.HostWhitelistEnabled != nil {
		current.HostWhitelistEnabled = *input.HostWhitelistEnabled
	}
	if input.AllowedHosts != nil {
		current.AllowedHosts = *input.AllowedHosts
	}
	values, err := a.settings.Update(current)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.audit.SetRetentionDays(values.AuditRetentionDays); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": values})
}
