package server

import (
	"net/http"

	"github.com/wanstu/mcp-center/internal/buildinfo"
)

func (a *App) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, buildinfo.Current())
}
