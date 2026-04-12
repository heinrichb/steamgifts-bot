package cli

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddToZipAndExtract(t *testing.T) {
	dir := t.TempDir()

	// Create a source file.
	srcPath := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("hello zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a zip file and add the source.
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	if err := addToZip(w, srcPath, "source.txt"); err != nil {
		t.Fatalf("addToZip: %v", err)
	}
	w.Close()
	f.Close()

	// Extract from the zip.
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("expected 1 file in zip, got %d", len(r.File))
	}

	destPath := filepath.Join(dir, "extracted.txt")
	if err := extractFromZip(r.File[0], destPath); err != nil {
		t.Fatalf("extractFromZip: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello zip" {
		t.Errorf("extracted content: %q", data)
	}
}

func TestAddToZipNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	defer w.Close()
	defer f.Close()

	err := addToZip(w, "/nonexistent/file.txt", "file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestExtractFromZipPathTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")

	// Create a zip with a path-traversal entry.
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	entry, _ := w.Create("../../../etc/passwd")
	entry.Write([]byte("evil"))
	w.Close()
	f.Close()

	r, _ := zip.OpenReader(zipPath)
	defer r.Close()

	for _, file := range r.File {
		err := extractFromZip(file, filepath.Join(dir, filepath.Base(file.Name)))
		if strings.Contains(file.Name, "..") {
			if err == nil {
				t.Error("expected error for path traversal")
			}
			if !strings.Contains(err.Error(), "suspicious") {
				t.Errorf("expected suspicious path error, got: %v", err)
			}
		}
	}
}

func TestVersionCmd(t *testing.T) {
	cmd := newVersionCmd("1.2.3", "abc123", "2024-01-01")
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.Execute()
	out := buf.String()
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("expected version in output: %s", out)
	}
	if !strings.Contains(out, "abc123") {
		t.Errorf("expected commit in output: %s", out)
	}
}
