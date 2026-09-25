package server

import (
	"net/http"
	"strings"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func (a *App) handleMCPImport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Format   string `json:"format"`
		Conflict string `json:"conflict"`
		Content  string `json:"content"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.core.ImportJSON([]byte(input.Content), input.Format, input.Conflict)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidateAllProxies()
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleMCPExport(w http.ResponseWriter, r *http.Request) {
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = mcpcenter.ImportFormatCore
	}
	data, err := a.core.ExportJSON(format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	filename := "mcp-center-export.json"
	if format == mcpcenter.ImportFormatMCPServers {
		filename = "mcp-servers-export.json"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
