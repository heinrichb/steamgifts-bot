// Package state owns the bot's small persistent state file.
//
// Today it stores per-account "last successful Steam sync" timestamps so
// the bot doesn't burn through the daily sync cooldown when it's restarted
// frequently (during testing, after a crash, on Docker container restart).
//
// The format is intentionally a single JSON file with named fields rather
// than positional state, so future fields (entry history, win log, scorer
// metadata) can land additively without breaking older state files.
//
// All writes are atomic (temp file + rename) so a crash mid-write can't
// leave a corrupted file. All public methods are safe for concurrent use.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// file is the on-disk JSON shape. Adding fields here is backwards-compatible:
// older state files just unmarshal with zero values for the new fields.
type file struct {
	Version  int                  `json:"version"`
	LastSync map[string]time.Time `json:"last_sync,omitempty"`
}

// Store is an in-memory cache backed by atomic writes to a path on disk.
type Store struct {
	mu   sync.RWMutex
	path string
	data file
}

// DefaultPathFor returns the default state-file path beside the given config file.
// (E.g. /etc/sgbot/config.yaml → /etc/sgbot/state.json.)
func DefaultPathFor(configPath string) string {
	if configPath == "" {
		return "state.json"
	}
	return filepath.Join(filepath.Dir(configPath), "state.json")
}

// Load reads the state file at path. A missing file is not an error — it
// returns an empty Store rooted at path so the first SetLastSync writes it.
func Load(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: file{Version: 1, LastSync: map[string]time.Time{}},
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if s.data.LastSync == nil {
		s.data.LastSync = map[string]time.Time{}
	}
	if s.data.Version == 0 {
		s.data.Version = 1
	}
	return s, nil
}

// Path returns the on-disk path the store reads and writes.
func (s *Store) Path() string { return s.path }

// LastSync returns the most recent successful Steam sync time for the named
// account, or the zero Time if there's no record.
func (s *Store) LastSync(account string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.LastSync[account]
}

// SetLastSync records `t` as the most recent successful sync for the named
// account and persists the file atomically.
//
// We deep-copy the LastSync map under the lock before releasing it. The
// snapshot is then marshalled outside the critical section, so multiple
// concurrent SetLastSync calls don't race on the same underlying map.
func (s *Store) SetLastSync(account string, t time.Time) error {
	s.mu.Lock()
	if s.data.LastSync == nil {
		s.data.LastSync = map[string]time.Time{}
	}
	s.data.LastSync[account] = t
	snapshot := file{
		Version:  s.data.Version,
		LastSync: make(map[string]time.Time, len(s.data.LastSync)),
	}
	for k, v := range s.data.LastSync {
		snapshot.LastSync[k] = v
	}
	s.mu.Unlock()
	return s.save(snapshot)
}

// save writes the file atomically: marshal → write to a temp sibling → rename.
// The rename is atomic on POSIX and on NTFS for files in the same directory,
// so a crash mid-write can't produce a half-written state.json.
func (s *Store) save(data file) error {
	if s.path == "" {
		return errors.New("state: no path configured")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("state: mkdir: %w", err)
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.json")
	if err != nil {
		return fmt.Errorf("state: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("state: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("state: close: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("state: chmod: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("state: rename: %w", err)
	}
	return nil
}
