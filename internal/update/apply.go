package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const binaryName = "steamgifts-bot"

// ProgressFunc is called during an update with the current phase and
// completion percentage (0.0–1.0) within that phase.
// Phases: "downloading", "extracting", "installing", "done".
type ProgressFunc func(phase string, pct float64)

// Apply downloads the release asset matching the current OS/arch, extracts
// the binary, and replaces the running executable. Returns the path to the
// new binary on success.
func Apply(ctx context.Context, rel Release) (string, error) {
	return ApplyWithProgress(ctx, rel, nil)
}

// ApplyWithProgress is like Apply but calls progress at each phase.
func ApplyWithProgress(ctx context.Context, rel Release, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = func(string, float64) {}
	}

	asset, err := matchAsset(rel.Assets)
	if err != nil {
		return "", err
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("update: locate self: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("update: resolve self: %w", err)
	}

	tmpDir, err := os.MkdirTemp(filepath.Dir(exe), ".update-*")
	if err != nil {
		return "", fmt.Errorf("update: create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, asset.Name)
	progress("downloading", 0)
	if err := downloadWithProgress(ctx, asset.BrowserDownloadURL, archivePath, asset.Size, func(pct float64) {
		progress("downloading", pct)
	}); err != nil {
		return "", err
	}

	progress("extracting", 0)
	newBin := filepath.Join(tmpDir, binaryName+exeSuffix())
	if err := extract(archivePath, newBin); err != nil {
		return "", err
	}

	progress("installing", 0)
	if err := replaceBinary(exe, newBin); err != nil {
		return "", err
	}

	progress("done", 1.0)
	return exe, nil
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// matchAsset finds the release asset for the current OS and architecture.
func matchAsset(assets []Asset) (Asset, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	archName := goarch
	if goarch == "amd64" {
		archName = "x86_64"
	}

	var ext string
	switch goos {
	case "windows":
		ext = ".zip"
	default:
		ext = ".tar.gz"
	}

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, goos) && strings.Contains(name, strings.ToLower(archName)) && strings.HasSuffix(name, ext) {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("update: no asset found for %s/%s", goos, goarch)
}

type progressReader struct {
	reader     io.Reader
	total      int64
	read       int64
	onProgress func(float64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)
	if pr.total > 0 && pr.onProgress != nil {
		pr.onProgress(float64(pr.read) / float64(pr.total))
	}
	return n, err
}

func downloadWithProgress(ctx context.Context, url, dst string, expectedSize int64, onProgress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("update: download request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download returned %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total = expectedSize
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("update: create archive: %w", err)
	}
	defer f.Close()

	reader := io.Reader(resp.Body)
	if total > 0 && onProgress != nil {
		reader = &progressReader{reader: resp.Body, total: total, onProgress: onProgress}
	}

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("update: write archive: %w", err)
	}
	return f.Close()
}

func extract(archivePath, dstBinary string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, dstBinary)
	}
	return extractTarGz(archivePath, dstBinary)
}

func extractTarGz(archivePath, dstBinary string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("update: open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("update: gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	target := binaryName + exeSuffix()
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("update: tar: %w", err)
		}
		if filepath.Base(hdr.Name) == target && hdr.Typeflag == tar.TypeReg {
			out, err := os.OpenFile(dstBinary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return fmt.Errorf("update: extract binary: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("update: extract binary: %w", err)
			}
			return out.Close()
		}
	}
	return fmt.Errorf("update: binary %q not found in archive", target)
}

func extractZip(archivePath, dstBinary string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("update: open zip: %w", err)
	}
	defer r.Close()

	target := binaryName + exeSuffix()
	for _, f := range r.File {
		if filepath.Base(f.Name) == target {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("update: open zip entry: %w", err)
			}
			defer rc.Close()

			out, err := os.OpenFile(dstBinary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return fmt.Errorf("update: extract binary: %w", err)
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				return fmt.Errorf("update: extract binary: %w", err)
			}
			return out.Close()
		}
	}
	return fmt.Errorf("update: binary %q not found in zip", target)
}

// replaceBinary swaps the running exe with the new one. On Windows, we
// rename the running binary first since it can't be overwritten while locked.
func replaceBinary(oldPath, newPath string) error {
	if runtime.GOOS == "windows" {
		bakPath := oldPath + ".old"
		_ = os.Remove(bakPath)
		if err := os.Rename(oldPath, bakPath); err != nil {
			return fmt.Errorf("update: rename old binary: %w", err)
		}
		if err := os.Rename(newPath, oldPath); err != nil {
			_ = os.Rename(bakPath, oldPath)
			return fmt.Errorf("update: install new binary: %w", err)
		}
		return nil
	}
	if err := os.Rename(newPath, oldPath); err != nil {
		return fmt.Errorf("update: replace binary: %w", err)
	}
	return nil
}
