package server

import (
	"net/http"
)

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
		PublicBaseURL        *string   `json:"public_base_url,omitempty"`
		AuditRetentionDays   *int      `json:"audit_retention_days,omitempty"`
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
	if input.PublicBaseURL != nil {
		current.PublicBaseURL = *input.PublicBaseURL
	}
	if input.AuditRetentionDays != nil {
		current.AuditRetentionDays = *input.AuditRetentionDays
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
