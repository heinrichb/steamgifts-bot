package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heinrichb/steamgifts-bot/internal/config"
)

func TestSaveConfigYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yml")

	cfg := config.Defaults()
	cfg.Accounts = []config.Account{{Name: "wizard-test", Cookie: "cookie123"}}

	if err := saveConfigYAML(&cfg, path); err != nil {
		t.Fatalf("saveConfigYAML: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "steamgifts-bot config") {
		t.Error("expected header comment")
	}
	if !strings.Contains(content, "wizard-test") {
		t.Error("expected account name in output")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions: %o, want 600", info.Mode().Perm())
	}
}

func TestSaveConfigYAMLCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "config.yml")

	cfg := config.Defaults()
	if err := saveConfigYAML(&cfg, path); err != nil {
		t.Fatalf("saveConfigYAML: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}
