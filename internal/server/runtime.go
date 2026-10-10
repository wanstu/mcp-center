package server

import (
	"context"
	"net/http"
	"time"
)

func (a *App) handleRuntimeList(w http.ResponseWriter, _ *http.Request) {
	items, err := a.core.RuntimeStatuses()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runtime":          items,
		"scheduled_probes": a.scheduledProbeViews(),
	})
}

func (a *App) handleRuntimeRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	id := r.PathValue("id")
	tools, err := a.core.Refresh(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	status, err := a.core.RuntimeStatus(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "tools": tools})
}

func (a *App) handleRuntimeDisconnect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.core.Disconnect(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	status, err := a.core.RuntimeStatus(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.invalidateProxy(id)
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}
