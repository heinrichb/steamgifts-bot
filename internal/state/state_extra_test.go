package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	os.WriteFile(p, []byte("{invalid json"), 0o644)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	os.WriteFile(p, []byte(""), 0o644)

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load empty file: %v", err)
	}
	if s.LastSync("any") != (time.Time{}) {
		t.Error("expected zero time for empty file")
	}
}

func TestLoadVersionZeroDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	os.WriteFile(p, []byte(`{"last_sync":{}}`), 0o644)

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Internal version should default to 1.
	if s.data.Version != 1 {
		t.Errorf("version: %d", s.data.Version)
	}
}

func TestLoadNilLastSync(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	os.WriteFile(p, []byte(`{"version":1}`), 0o644)

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.data.LastSync == nil {
		t.Error("LastSync map should be initialized even when null in JSON")
	}
}

func TestSaveEmptyPath(t *testing.T) {
	s := &Store{path: ""}
	err := s.save(file{Version: 1})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestSetLastSyncCreatesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "state.json")
	s, _ := Load(p)

	if err := s.SetLastSync("acct", time.Now()); err != nil {
		t.Fatalf("SetLastSync: %v", err)
	}

	if _, err := os.Stat(p); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestSetLastSyncNilMapInit(t *testing.T) {
	s := &Store{
		path: filepath.Join(t.TempDir(), "state.json"),
		data: file{Version: 1, LastSync: nil},
	}
	if err := s.SetLastSync("acct", time.Now()); err != nil {
		t.Fatalf("SetLastSync with nil map: %v", err)
	}
}
