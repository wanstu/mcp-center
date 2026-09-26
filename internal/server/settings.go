package server

import (
	"net/http"

	settingsstore "github.com/wanstu/mcp-center/internal/settings"
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
	var input settingsstore.Values
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	values, err := a.settings.Update(input)
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
