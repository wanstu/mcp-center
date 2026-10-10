package upstreamsecret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Info struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type entry struct {
	Ciphertext string    `json:"ciphertext"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type state struct {
	Version int              `json:"version"`
	Entries map[string]entry `json:"entries"`
}

type Store struct {
	path string
	key  []byte

	mu      sync.RWMutex
	entries map[string]entry
}

func New(path, keyPath string) (*Store, error) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(keyPath) == "" {
		return nil, errors.New("upstreamsecret: state path and key path are required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("upstreamsecret: create data directory: %w", err)
	}
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, key: key, entries: map[string]entry{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) List() []Info {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Info, 0, len(s.entries))
	for name, item := range s.entries {
		items = append(items, Info{Name: name, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items
}

func (s *Store) Set(name, value string) (Info, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return Info{}, errors.New("变量名必须使用字母/数字/下划线，且不能以数字开头")
	}
	if value == "" {
		return Info{}, errors.New("变量值不能为空")
	}
	if len(value) > 64*1024 {
		return Info{}, errors.New("变量值不能超过 64 KB")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ciphertext, err := encrypt(s.key, name, value)
	if err != nil {
		return Info{}, err
	}
	now := time.Now().UTC()
	current, exists := s.entries[name]
	if !exists {
		current.CreatedAt = now
	}
	current.Ciphertext = ciphertext
	current.UpdatedAt = now
	s.entries[name] = current
	if err := s.saveLocked(); err != nil {
		return Info{}, err
	}
	return Info{Name: name, CreatedAt: current.CreatedAt, UpdatedAt: current.UpdatedAt}, nil
}

// WithCreatedBatch saves new encrypted variables, performs apply, and
// restores the previous secret store if apply fails. The store lock prevents
// other secret mutations from interleaving with the import. Existing variables
// can only be reused when the underlying plaintext matches exactly; their
// values are never silently overwritten.
func (s *Store) WithCreatedBatch(values map[string]string, apply func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := make(map[string]entry, len(s.entries))
	for k, v := range s.entries {
		previous[k] = v
	}
	next := make(map[string]entry, len(previous)+len(values))
	for k, v := range previous {
		next[k] = v
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	now := time.Now().UTC()
	changed := false
	for _, name := range names {
		value := values[name]
		if !namePattern.MatchString(name) || len(name) > 128 {
			return fmt.Errorf("变量名 %q 不合法", name)
		}
		if value == "" || len(value) > 64*1024 {
			return fmt.Errorf("变量 %q 为空或超过 64 KB", name)
		}
		if existing, ok := next[name]; ok {
			current, err := decrypt(s.key, name, existing.Ciphertext)
			if err != nil {
				return fmt.Errorf("无法读取已有变量 %q", name)
			}
			if current != value {
				return fmt.Errorf("变量 %q 已存在且值不同，请先在设置中修改或删除变量", name)
			}
			continue
		}
		encrypted, err := encrypt(s.key, name, value)
		if err != nil {
			return err
		}
		next[name] = entry{Ciphertext: encrypted, CreatedAt: now, UpdatedAt: now}
		changed = true
	}
	if changed {
		s.entries = next
		if err := s.saveLocked(); err != nil {
			s.entries = previous
			return fmt.Errorf("无法持久化加密变量: %w", err)
		}
	}
	if err := apply(); err != nil {
		if changed {
			s.entries = previous
			if rollbackErr := s.saveLocked(); rollbackErr != nil {
				return fmt.Errorf("导入失败且加密变量回滚失败，请检查存储: import: %v; rollback: %w", err, rollbackErr)
			}
		}
		return err
	}
	return nil
}

func (s *Store) Delete(name string) error {
	name = strings.TrimSpace(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[name]; !ok {
		return fmt.Errorf("变量 %q 不存在", name)
	}
	delete(s.entries, name)
	return s.saveLocked()
}

func (s *Store) Lookup(name string) (string, bool) {
	s.mu.RLock()
	item, ok := s.entries[name]
	if !ok {
		s.mu.RUnlock()
		return "", false
	}
	key := append([]byte(nil), s.key...)
	s.mu.RUnlock()

	value, err := decrypt(key, name, item.Ciphertext)
	if err != nil {
		return "", false
	}
	return value, true
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("upstreamsecret: read state: %w", err)
	}
	var st state
	if len(data) > 0 {
		if err := json.Unmarshal(data, &st); err != nil {
			return fmt.Errorf("upstreamsecret: decode state: %w", err)
		}
	}
	if st.Entries != nil {
		s.entries = st.Entries
	}
	return nil
}

func (s *Store) saveLocked() error {
	st := state{Version: 1, Entries: s.entries}
	data, err := json.MarshalIndent(st, "", "  ")
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

func loadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != 32 {
			return nil, errors.New("upstreamsecret: invalid master key length")
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("upstreamsecret: read master key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadOrCreateKey(path)
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

func encrypt(key []byte, name, value string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), []byte(name))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func decrypt(key []byte, name, encoded string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("upstreamsecret: ciphertext is too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(name))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
