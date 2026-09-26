package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Values struct {
	AuditRetentionDays int `json:"audit_retention_days"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func Default() Values {
	return Values{AuditRetentionDays: 30}
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
	return nil
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
