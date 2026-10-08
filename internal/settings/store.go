package settings

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Values struct {
	AuditRetentionDays   int      `json:"audit_retention_days"`
	OAuthDisabled        bool     `json:"oauth_disabled,omitempty"`
	HostWhitelistEnabled bool     `json:"host_whitelist_enabled"`
	AllowedHosts         []string `json:"allowed_hosts"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func Default() Values {
	return Values{AuditRetentionDays: 30, AllowedHosts: []string{}}
}

func New(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("settings: path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	store := &Store{path: path}
	store.mu.Lock()
	defer store.mu.Unlock()

	values, exists, err := store.loadLocked()
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := store.saveLocked(Default()); err != nil {
			return nil, err
		}
		return store, nil
	}
	if err := validate(values); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Get() (Values, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, _, err := s.loadLocked()
	return values, err
}

func (s *Store) Update(values Values) (Values, error) {
	if err := validate(values); err != nil {
		return Values{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.saveLocked(values); err != nil {
		return Values{}, err
	}
	return values, nil
}

func validate(values Values) error {
	if values.AuditRetentionDays < 1 || values.AuditRetentionDays > 3650 {
		return errors.New("审计日志保留天数必须在 1 到 3650 天之间")
	}
	if values.HostWhitelistEnabled && len(values.AllowedHosts) == 0 {
		return errors.New("启用域名/IP 白名单时至少填写一个允许的访问地址")
	}
	if len(values.AllowedHosts) > 100 {
		return errors.New("白名单最多支持 100 个地址")
	}
	for _, host := range values.AllowedHosts {
		if !ValidHostEntry(host) {
			return errors.New("白名单地址无效，请填写域名或 IP，不含协议、端口或路径: " + host)
		}
	}
	return nil
}

var hostnamePattern = regexp.MustCompile("^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$")

// ValidHostEntry accepts an exact DNS name, localhost, IPv4 or IPv6. Ports and wildcard rules are not allowed.
func ValidHostEntry(raw string) bool {
	if raw == "" || raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "/:@\\* %") {
		if net.ParseIP(raw) == nil {
			return false
		}
	}
	if net.ParseIP(raw) != nil {
		return true
	}
	if len(raw) > 253 || strings.Contains(raw, "..") {
		return false
	}
	if raw == "localhost" {
		return true
	}
	return hostnamePattern.MatchString(strings.ToLower(raw)) && (strings.Contains(raw, ".") || strings.EqualFold(raw, "localhost"))
}

func (s *Store) loadLocked() (Values, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Values{}, false, nil
	}
	if err != nil {
		return Values{}, false, err
	}
	var values Values
	if err := json.Unmarshal(data, &values); err != nil {
		return Values{}, true, err
	}
	return values, true, nil
}

func (s *Store) saveLocked(values Values) error {
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(s.path)
		if err2 := os.Rename(tmp, s.path); err2 != nil {
			return err2
		}
	}
	return nil
}
