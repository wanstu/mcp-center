package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

func (a *App) handleMCPImportPreview(w http.ResponseWriter, r *http.Request) {
	var input mcpImportRequest
	if err := decodeMCPImportRequest(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.parseMCPImport(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleMCPImport(w http.ResponseWriter, r *http.Request) {
	var input mcpImportRequest
	if err := decodeMCPImportRequest(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	preview, err := a.parseMCPImport(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	selected := make([]mcpcenter.Config, 0, len(preview.Candidates))
	requested := map[string]bool{}
	if input.SelectedNames != nil {
		if len(*input.SelectedNames) == 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("请至少选择一个 MCP"))
			return
		}
		for _, name := range *input.SelectedNames {
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" || requested[key] {
				writeError(w, http.StatusBadRequest, fmt.Errorf("选择项重复或为空"))
				return
			}
			requested[key] = true
		}
	}
	for _, c := range preview.Candidates {
		key := strings.ToLower(strings.TrimSpace(c.Name))
		if input.SelectedNames != nil && !requested[key] {
			continue
		}
		if len(c.Errors) > 0 {
			// Do not include source content (may contain literal secrets) in errors.
			writeError(w, http.StatusBadRequest, fmt.Errorf("MCP %q: %s", c.Name, c.Errors[0].Message))
			return
		}
		selected = append(selected, c.config)
		delete(requested, key)
	}
	if len(requested) > 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("所选 MCP 不在当前配置中"))
		return
	}
	if len(selected) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("没有可导入的 MCP"))
		return
	}
	// The Core validates the whole batch and persists it atomically.
	encoded, err := json.Marshal(map[string]any{"version": 1, "mcps": selected})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := a.core.ImportJSON(encoded, mcpcenter.ImportFormatCore, input.Conflict)
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

func decodeMCPImportRequest(r *http.Request, input *mcpImportRequest) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("content-type must be application/json")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nilResponseWriter{}, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("导入请求必须只包含一个 JSON 对象")
	}
	return nil
}
