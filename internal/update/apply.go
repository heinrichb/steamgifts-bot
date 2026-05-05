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

// Apply downloads the release asset matching the current OS/arch, extracts
// the binary, and replaces the running executable. Returns the path to the
// new binary on success.
func Apply(ctx context.Context, rel Release) (string, error) {
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
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := download(ctx, asset.BrowserDownloadURL, archivePath); err != nil {
		return "", err
	}

	newBin := filepath.Join(tmpDir, binaryName+exeSuffix())
	if err := extract(archivePath, newBin); err != nil {
		return "", err
	}

	if err := replaceBinary(exe, newBin); err != nil {
		return "", err
	}
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

func download(ctx context.Context, url, dst string) error {
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

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("update: create archive: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
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
	defer gz.Close()

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
