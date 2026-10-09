package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

const (
	importAuto      = "auto"
	importOpenCode  = "opencode"
	importWorkBuddy = "workbuddy"
	importCodex     = "codex-plugin"
	importClaude    = "claude-code"
	importHub       = "mcphub"
)

type mcpImportRequest struct {
	Format          string    `json:"format"`
	Conflict        string    `json:"conflict,omitempty"`
	Content         string    `json:"content"`
	SourceScope     string    `json:"source_scope,omitempty"`
	SelectedNames   *[]string `json:"selected_names,omitempty"`
	ConvertLiterals bool      `json:"convert_literals,omitempty"`
}
type mcpImportIssue struct {
	Kind      string `json:"kind"`
	FieldPath string `json:"field_path,omitempty"`
	Message   string `json:"message"`
}
type mcpImportRequirement struct {
	ReferenceName string `json:"reference_name"`
	FieldPath     string `json:"field_path"`
}
type mcpImportCandidate struct {
	Name                  string                 `json:"name"`
	Transport             string                 `json:"transport,omitempty"`
	AuthMode              string                 `json:"auth_mode,omitempty"`
	ExistingMCPID         string                 `json:"existing_mcp_id,omitempty"`
	EndpointConfigured    bool                   `json:"endpoint_configured"`
	Warnings              []mcpImportIssue       `json:"warnings,omitempty"`
	Errors                []mcpImportIssue       `json:"errors,omitempty"`
	ReferenceRequirements []mcpImportRequirement `json:"reference_requirements,omitempty"`
	config                mcpcenter.Config
}
type mcpImportPreview struct {
	Format      string               `json:"format"`
	SourceScope string               `json:"source_scope,omitempty"`
	Candidates  []mcpImportCandidate `json:"candidates"`
}

func issue(kind, field, msg string) mcpImportIssue { return mcpImportIssue{kind, field, msg} }

// parseMCPImport is shared between preview and apply, so the selected
// configurations cannot follow a different decoder/validation path.
func (a *App) parseMCPImport(in mcpImportRequest) (mcpImportPreview, error) {
	if len(in.Content) > 2*1024*1024 {
		return mcpImportPreview{}, errors.New("导入内容不能超过 2 MB")
	}
	raw, err := readMCPJSONC(in.Content)
	if err != nil {
		return mcpImportPreview{}, err
	}
	format := strings.TrimSpace(in.Format)
	if format == "" || format == importAuto {
		switch {
		case raw["mcps"] != nil:
			format = mcpcenter.ImportFormatCore
		case nestedObject(raw, "mcp", "servers") != nil:
			format = importOpenCode
		case raw["projects"] != nil:
			format = importClaude
		case raw["servers"] != nil:
			format = importHub
		case raw["mcpServers"] != nil:
			format = mcpcenter.ImportFormatMCPServers
			if hasWorkBuddyMarker(asObject(raw["mcpServers"])) {
				format = importWorkBuddy
			}
		case directServerMap(raw):
			format = importCodex
		default:
			return mcpImportPreview{}, errors.New("无法识别 MCP 格式：请提供 mcpServers、mcps、mcp.servers 或支持的服务配置")
		}
	}
	var servers map[string]any
	var configs []mcpcenter.Config
	scope := ""
	switch format {
	case mcpcenter.ImportFormatCore:
		buf, err := json.Marshal(raw["mcps"])
		if err != nil {
			return mcpImportPreview{}, err
		}
		if err = json.Unmarshal(buf, &configs); err != nil {
			return mcpImportPreview{}, errors.New("MCP Center 格式的 mcps 必须是配置数组")
		}
	case importOpenCode:
		servers = nestedObject(raw, "mcp", "servers")
	case importWorkBuddy, mcpcenter.ImportFormatMCPServers:
		servers = asObject(raw["mcpServers"])
	case importCodex:
		servers = asObject(raw["mcpServers"])
		if servers == nil && directServerMap(raw) {
			servers = raw
		}
	case importHub:
		servers = asObject(raw["servers"])
		if servers == nil {
			servers = asObject(raw["mcpServers"])
		}
	case importClaude:
		servers = asObject(raw["mcpServers"])
		if servers == nil {
			projects := asObject(raw["projects"])
			if projects == nil {
				return mcpImportPreview{}, errors.New("Claude Code 格式缺少 projects 或 mcpServers")
			}
			scope = strings.TrimSpace(in.SourceScope)
			if scope == "" {
				if len(projects) != 1 {
					return mcpImportPreview{}, errors.New("Claude Code 包含多个项目，请先填写源项目路径")
				}
				for k := range projects {
					scope = k
				}
			}
			project := asObject(projects[scope])
			if project == nil {
				return mcpImportPreview{}, errors.New("找不到指定 Claude Code 项目路径")
			}
			servers = asObject(project["mcpServers"])
		}
	default:
		return mcpImportPreview{}, fmt.Errorf("不支持导入格式 %q", format)
	}
	if format != mcpcenter.ImportFormatCore && servers == nil {
		return mcpImportPreview{}, errors.New("导入格式缺少服务定义对象")
	}
	items := make([]mcpImportCandidate, 0)
	if format == mcpcenter.ImportFormatCore {
		for _, cfg := range configs {
			items = append(items, mcpImportCandidate{Name: cfg.Name, Transport: cfg.Transport, AuthMode: cfg.AuthMode, EndpointConfigured: cfg.Endpoint != "", config: cfg})
		}
	} else {
		names := make([]string, 0, len(servers))
		for k := range servers {
			names = append(names, k)
		}
		sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
		for _, name := range names {
			items = append(items, normalizeImportedServer(format, name, servers[name], in.ConvertLiterals))
		}
	}
	if len(items) == 0 {
		return mcpImportPreview{}, errors.New("配置中没有可导入的 MCP")
	}
	existing, err := a.core.List()
	if err != nil {
		return mcpImportPreview{}, err
	}
	byName := map[string]string{}
	for _, m := range existing {
		byName[strings.ToLower(m.Name)] = m.ID
	}
	seen := map[string]int{}
	for idx := range items {
		item := &items[idx]
		if item.config.Name == "" {
			item.config.Name = item.Name
		}
		item.ExistingMCPID = byName[strings.ToLower(item.Name)]
		key := strings.ToLower(strings.TrimSpace(item.Name))
		if key == "" {
			item.Errors = append(item.Errors, issue("invalid_name", "name", "MCP 名称不能为空"))
		}
		if first, ok := seen[key]; ok {
			duplicate := issue("duplicate_name", "name", "来源包含大小写不敏感的重名 MCP")
			item.Errors = append(item.Errors, duplicate)
			items[first].Errors = append(items[first].Errors, duplicate)
		} else {
			seen[key] = idx
		}
		if _, validateErr := mcpcenter.Validate(item.config); validateErr != nil {
			// Validation error strings may contain the source URL/credential.
			item.Errors = append(item.Errors, issue("invalid_configuration", "", "连接配置无效：请检查 URL、启动命令、认证变量引用"))
		}
	}
	return mcpImportPreview{Format: format, SourceScope: scope, Candidates: items}, nil
}
func normalizeImportedServer(format, name string, value any, convert bool) mcpImportCandidate {
	item := mcpImportCandidate{Name: name, config: mcpcenter.Config{Name: name, AuthMode: mcpcenter.AuthNone}}
	obj := asObject(value)
	if obj == nil {
		item.Errors = append(item.Errors, issue("invalid_server", "", "服务定义必须是对象"))
		return item
	}
	typ, _ := obj["type"].(string)
	norm := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(typ), "-", ""), "_", ""))
	if norm == "sse" {
		item.Errors = append(item.Errors, issue("unsupported_transport", "type", "不支持旧 SSE 传输，请改用 Streamable HTTP"))
		return item
	}
	var execString string
	var args []string
	var urlValue string
	if format == importOpenCode {
		switch norm {
		case "local":
			command, valid := stringArray(obj["command"])
			if !valid || len(command) == 0 {
				item.Errors = append(item.Errors, issue("invalid_command", "command", "OpenCode local 需要非空 command 数组"))
				return item
			}
			execString = command[0]
			args = command[1:]
		case "remote":
			urlValue, _ = obj["url"].(string)
		default:
			item.Errors = append(item.Errors, issue("unsupported_transport", "type", "OpenCode 必须指定 local 或 remote"))
			return item
		}
	} else {
		urlValue, _ = obj["url"].(string)
		if v, ok := obj["command"].(string); ok {
			execString = v
		}
		if obj["command"] != nil && execString == "" {
			item.Errors = append(item.Errors, issue("invalid_command", "command", "stdio command 必须是字符串"))
		}
		if obj["args"] != nil {
			var valid bool
			args, valid = stringArray(obj["args"])
			if !valid {
				item.Errors = append(item.Errors, issue("invalid_args", "args", "args 必须是字符串数组"))
			}
		}
	}
	isHTTP := strings.TrimSpace(urlValue) != "" || norm == "http" || norm == "streamablehttp" || norm == "remote"
	isStdio := strings.TrimSpace(execString) != "" || norm == "stdio" || norm == "local"
	if isHTTP && isStdio {
		item.Errors = append(item.Errors, issue("mixed_transport", "", "不能同时设置 HTTP URL 和 stdio 命令"))
	}
	if !isHTTP && !isStdio {
		item.Errors = append(item.Errors, issue("missing_transport", "", "需要 HTTP URL 或 stdio command"))
	}
	if isHTTP && !isStdio {
		item.config.Transport = mcpcenter.TransportStreamableHTTP
		item.config.Endpoint = urlValue
		item.config.HeaderRefs = normalizeImportedRefs(&item, name, "headers", obj["headers"], convert)
		if len(item.config.HeaderRefs) > 0 {
			item.config.AuthMode = mcpcenter.AuthHeaders
		}
	} else if isStdio && !isHTTP {
		item.config.Transport = mcpcenter.TransportStdio
		item.config.Executable = execString
		item.config.Args = args
		refs := obj["env"]
		field := "env"
		if format == importOpenCode {
			refs = obj["environment"]
			field = "environment"
		}
		item.config.EnvRefs = normalizeImportedRefs(&item, name, field, refs, convert)
	}
	if v, ok := obj["disabled"].(bool); ok {
		item.config.Disabled = v
	}
	if v, ok := obj["enabled"].(bool); ok && !v {
		item.config.Disabled = true
	}
	item.Transport = item.config.Transport
	item.AuthMode = item.config.AuthMode
	item.EndpointConfigured = item.config.Endpoint != ""
	known := map[string]bool{"type": true, "command": true, "args": true, "url": true, "headers": true, "env": true, "environment": true, "disabled": true, "enabled": true, "description": true, "version": true}
	for key := range obj {
		switch key {
		case "cwd", "timeout", "runtime", "npmRegistry", "install":
			item.Warnings = append(item.Warnings, issue("ignored_field", key, "来源中的此设置不会被 MCP Center 使用"))
		default:
			if !known[key] {
				item.Warnings = append(item.Warnings, issue("unsupported_field", key, "来源字段未映射到 MCP Center"))
			}
		}
	}
	sort.Slice(item.Warnings, func(i, j int) bool { return item.Warnings[i].FieldPath < item.Warnings[j].FieldPath })
	return item
}
func normalizeImportedRefs(item *mcpImportCandidate, serverName, field string, raw any, convert bool) map[string]string {
	if raw == nil {
		return nil
	}
	refs := asObject(raw)
	if refs == nil {
		item.Errors = append(item.Errors, issue("invalid_references", field, "必须是字符串键值对象"))
		return nil
	}
	result := make(map[string]string, len(refs))
	keys := make([]string, 0, len(refs))
	for k := range refs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		val, ok := refs[key].(string)
		if !ok {
			item.Errors = append(item.Errors, issue("invalid_reference", field+"."+key, "变量值必须是字符串"))
			continue
		}
		val = strings.TrimSpace(val)
		val = normalizeEnvBrace(val)
		if strings.Contains(val, "${") {
			result[key] = val
			continue
		}
		if !convert {
			item.Errors = append(item.Errors, issue("literal_reference", field+"."+key, "包含明文值；请替换成 ${NAME} environment reference，或勾选安全转换"))
			continue
		}
		ref := generatedMCPImportRef(serverName, field, key)
		placeholder := "${" + ref + "}"
		if field == "headers" {
			lower := strings.ToLower(val)
			for _, prefix := range []string{"bearer ", "basic "} {
				if strings.HasPrefix(lower, prefix) {
					placeholder = val[:len(prefix)] + placeholder
					break
				}
			}
		}
		result[key] = placeholder
		item.ReferenceRequirements = append(item.ReferenceRequirements, mcpImportRequirement{ReferenceName: ref, FieldPath: field + "." + key})
		item.Warnings = append(item.Warnings, issue("secret_not_imported", field+"."+key, "明文未保存；导入后请在设置 → 上游密钥变量中填写原值"))
	}
	return result
}
func generatedMCPImportRef(server, field, key string) string {
	parts := []string{"MCP_CENTER_IMPORT", server, field, key}
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteByte('_')
		for _, ch := range strings.ToUpper(part) {
			if ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
				builder.WriteRune(ch)
			} else {
				builder.WriteByte('_')
			}
		}
	}
	name := strings.Trim(strings.ReplaceAll(builder.String(), "__", "_"), "_")
	// Avoid collisions when source names normalize to the same identifier.
	digest := sha256.Sum256([]byte(server + "\x00" + field + "\x00" + key))
	return fmt.Sprintf("%s_%x", name, digest[:4])
}
func normalizeEnvBrace(v string) string {
	for {
		a := strings.Index(v, "{env:")
		if a < 0 {
			return v
		}
		z := strings.IndexByte(v[a:], '}')
		if z < 0 {
			return v
		}
		name := strings.TrimSpace(v[a+5 : a+z])
		if name == "" {
			return v
		}
		v = v[:a] + "${" + name + "}" + v[a+z+1:]
	}
}
func asObject(v any) map[string]any { m, _ := v.(map[string]any); return m }
func nestedObject(root map[string]any, key, inner string) map[string]any {
	return asObject(asObject(root[key])[inner])
}
func stringArray(raw any) ([]string, bool) {
	values, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
func directServerMap(root map[string]any) bool {
	if len(root) == 0 {
		return false
	}
	for _, v := range root {
		obj := asObject(v)
		if obj == nil {
			return false
		}
		if obj["command"] == nil && obj["url"] == nil {
			return false
		}
	}
	return true
}
func hasWorkBuddyMarker(servers map[string]any) bool {
	for _, v := range servers {
		o := asObject(v)
		for _, key := range []string{"runtime", "x-workbuddy", "npmRegistry", "install"} {
			if o[key] != nil {
				return true
			}
		}
		if t, _ := o["type"].(string); t == "streamableHttp" {
			return true
		}
	}
	return false
}
func readMCPJSONC(content string) (map[string]any, error) {
	cleaned, err := stripMCPJSONComments([]byte(content))
	if err != nil {
		return nil, err
	}
	cleaned = stripMCPJSONTrailingCommas(cleaned)
	decoder := json.NewDecoder(bytes.NewReader(cleaned))
	decoder.UseNumber()
	if err := verifyMCPJSONObjectKeys(decoder); err != nil {
		return nil, err
	}
	decoder = json.NewDecoder(bytes.NewReader(cleaned))
	decoder.UseNumber()
	var root map[string]any
	if err = decoder.Decode(&root); err != nil || root == nil {
		return nil, errors.New("请提供有效 JSON/JSONC 对象")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("JSON 内容末尾不能包含其他数据")
	}
	return root, nil
}
func stripMCPJSONComments(raw []byte) ([]byte, error) {
	out := make([]byte, 0, len(raw))
	inString := false
	escaped := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(raw) {
			if raw[i+1] == '/' {
				i += 2
				for i < len(raw) && raw[i] != '\n' && raw[i] != '\r' {
					i++
				}
				if i < len(raw) {
					out = append(out, raw[i])
				}
				continue
			}
			if raw[i+1] == '*' {
				i += 2
				closed := false
				for i+1 < len(raw) {
					if raw[i] == '*' && raw[i+1] == '/' {
						i++
						closed = true
						break
					}
					i++
				}
				if !closed {
					return nil, errors.New("JSONC 注释没有结束")
				}
				continue
			}
		}
		out = append(out, c)
	}
	return out, nil
}
func stripMCPJSONTrailingCommas(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escaped := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(raw) && unicode.IsSpace(rune(raw[j])) {
				j++
			}
			if j < len(raw) && (raw[j] == ']' || raw[j] == '}') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}
func verifyMCPJSONObjectKeys(decoder *json.Decoder) error {
	if err := walkMCPJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("JSON 包含多余数据")
	}
	return nil
}
func walkMCPJSONValue(decoder *json.Decoder) error {
	tok, err := decoder.Token()
	if err != nil {
		return errors.New("无效 JSON/JSONC")
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyTok, err := decoder.Token()
			if err != nil {
				return errors.New("无效 JSON 键")
			}
			key, ok := keyTok.(string)
			if !ok {
				return errors.New("无效 JSON 键")
			}
			if seen[key] {
				return errors.New("JSON 中存在重复配置键：" + key)
			}
			seen[key] = true
			if err = walkMCPJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err = walkMCPJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	}
	return nil
}
