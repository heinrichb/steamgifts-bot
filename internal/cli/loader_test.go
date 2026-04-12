package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heinrichb/steamgifts-bot/internal/config"
)

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := `accounts:
  - name: test
    cookie: abc123def
defaults:
  min_points: 100
  pause_minutes: 10
  filters:
    - all
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, gotPath, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if gotPath != path {
		t.Errorf("path: %s", gotPath)
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(cfg.Accounts))
	}
	if cfg.Accounts[0].Name != "test" {
		t.Errorf("account name: %s", cfg.Accounts[0].Name)
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	_, _, err := loadConfig("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestLoadValidConfigMissing(t *testing.T) {
	_, _, err := loadValidConfig("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no config found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadValidConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	// Valid YAML but no accounts — validation should fail.
	if err := os.WriteFile(path, []byte("defaults:\n  min_points: 50\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := loadValidConfig(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "invalid config") {
		t.Errorf("expected 'invalid config' error, got: %v", err)
	}
}

func TestLoadValidConfigValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := `accounts:
  - name: main
    cookie: realcookie123
defaults:
  min_points: 50
  pause_minutes: 15
  filters:
    - all
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := loadValidConfig(path)
	if err != nil {
		t.Fatalf("loadValidConfig: %v", err)
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(cfg.Accounts))
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yml")

	cfg := config.Defaults()
	cfg.Accounts = []config.Account{{Name: "test", Cookie: "abc"}}

	if err := saveConfig(&cfg, path); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "steamgifts-bot config") {
		t.Error("expected header comment")
	}
	if !strings.Contains(content, "test") {
		t.Error("expected account name in output")
	}

	// Verify file permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions: %o, want 600", info.Mode().Perm())
	}
}

func TestSaveConfigEmptyPath(t *testing.T) {
	cfg := config.Defaults()
	if err := saveConfig(&cfg, ""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestConfigCandidatesReturnsNonEmpty(t *testing.T) {
	paths := configCandidates()
	if len(paths) == 0 {
		t.Error("expected at least one candidate path")
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, "config.yml") {
			t.Errorf("candidate should end with config.yml: %s", p)
		}
	}
}

func TestDefaultSavePathEndsWithConfigYml(t *testing.T) {
	path := defaultSavePath()
	if !strings.HasSuffix(path, "config.yml") {
		t.Errorf("defaultSavePath should end with config.yml: %s", path)
	}
}
