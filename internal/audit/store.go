package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("audit: path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

func (s *Store) Record(_ context.Context, event mcpcenter.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *Store) List(limit int) ([]mcpcenter.AuditEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []mcpcenter.AuditEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	items := make([]mcpcenter.AuditEvent, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var event mcpcenter.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if len(items) == limit {
			copy(items, items[1:])
			items[len(items)-1] = event
		} else {
			items = append(items, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

type Summary struct {
	Since        time.Time              `json:"since"`
	Total        int                    `json:"total"`
	Success      int                    `json:"success"`
	Errors       int                    `json:"errors"`
	RecentErrors []mcpcenter.AuditEvent `json:"recent_errors"`
}

func (s *Store) SummarySince(since time.Time, recentErrorLimit int) (Summary, error) {
	if recentErrorLimit <= 0 {
		recentErrorLimit = 5
	}
	if recentErrorLimit > 50 {
		recentErrorLimit = 50
	}
	summary := Summary{Since: since.UTC(), RecentErrors: []mcpcenter.AuditEvent{}}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return summary, nil
	}
	if err != nil {
		return Summary{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var event mcpcenter.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.StartedAt.Before(summary.Since) {
			continue
		}
		summary.Total++
		if event.Status == "error" {
			summary.Errors++
			if len(summary.RecentErrors) == recentErrorLimit {
				copy(summary.RecentErrors, summary.RecentErrors[1:])
				summary.RecentErrors[len(summary.RecentErrors)-1] = event
			} else {
				summary.RecentErrors = append(summary.RecentErrors, event)
			}
		} else {
			summary.Success++
		}
	}
	if err := scanner.Err(); err != nil {
		return Summary{}, err
	}
	for i, j := 0, len(summary.RecentErrors)-1; i < j; i, j = i+1, j-1 {
		summary.RecentErrors[i], summary.RecentErrors[j] = summary.RecentErrors[j], summary.RecentErrors[i]
	}
	return summary, nil
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.path, nil, 0o600)
}
