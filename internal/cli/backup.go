package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up config, state, and logs to a zip file",
	}
	cmd.AddCommand(newBackupCreateCmd(), newBackupRestoreCmd())
	return cmd
}

func newBackupCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [output.zip]",
		Short: "Create a backup zip of config.yml, state.json, and logs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			if configPath == "" {
				configPath = findConfig()
			}
			if configPath == "" {
				return fmt.Errorf("no config found — nothing to back up")
			}
			outPath := ""
			if len(args) > 0 {
				outPath = args[0]
			} else {
				outPath = fmt.Sprintf("steamgifts-bot-backup-%s.zip",
					time.Now().Format("2006-01-02-150405"))
			}

			candidates := backupCandidates(configPath)

			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("backup: create %s: %w", outPath, err)
			}
			defer f.Close()

			w := zip.NewWriter(f)
			defer w.Close()

			count := 0
			for _, path := range candidates {
				if _, err := os.Stat(path); err != nil {
					continue
				}
				if err := addToZip(w, path, filepath.Base(path)); err != nil {
					return fmt.Errorf("backup: add %s: %w", path, err)
				}
				count++
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", filepath.Base(path))
			}
			if count == 0 {
				os.Remove(outPath)
				return fmt.Errorf("no files found to back up")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ backed up %d files to %s\n", count, outPath)
			return nil
		},
	}
}

func newBackupRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <backup.zip>",
		Short: "Restore config, state, and logs from a backup zip",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			zipPath := args[0]
			configPath, _ := cmd.Flags().GetString("config")
			if configPath == "" {
				configPath = findConfig()
			}
			destDir := "."
			if configPath != "" {
				destDir = filepath.Dir(configPath)
			}

			r, err := zip.OpenReader(zipPath)
			if err != nil {
				return fmt.Errorf("restore: open %s: %w", zipPath, err)
			}
			defer r.Close()

			allowed := map[string]bool{
				"config.yml":         true,
				"state.json":         true,
				"steamgifts-bot.log": true,
			}

			count := 0
			for _, f := range r.File {
				name := filepath.Base(f.Name)
				if !allowed[name] {
					fmt.Fprintf(cmd.OutOrStdout(), "  ? skipping unknown file: %s\n", f.Name)
					continue
				}
				destPath := filepath.Join(destDir, name)
				if err := extractFromZip(f, destPath); err != nil {
					return fmt.Errorf("restore: extract %s: %w", name, err)
				}
				if name == "config.yml" {
					_ = os.Chmod(destPath, 0o600)
				}
				count++
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", name)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ restored %d files from %s\n", count, zipPath)
			return nil
		},
	}
}

func backupCandidates(configPath string) []string {
	dir := filepath.Dir(configPath)
	return []string{
		configPath,
		filepath.Join(dir, "state.json"),
		filepath.Join(dir, "steamgifts-bot.log"),
	}
}

func addToZip(w *zip.Writer, srcPath, name string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}

func createBackupInline(configPath string) string {
	if configPath == "" {
		configPath = findConfig()
	}
	if configPath == "" {
		return statusErr("No config found — nothing to back up")
	}

	outPath := fmt.Sprintf("steamgifts-bot-backup-%s.zip",
		time.Now().Format("2006-01-02-150405"))

	candidates := backupCandidates(configPath)

	f, err := os.Create(outPath)
	if err != nil {
		return statusErr("Create failed: " + err.Error())
	}
	w := zip.NewWriter(f)

	var backed []string
	for _, path := range candidates {
		if _, serr := os.Stat(path); serr != nil {
			continue
		}
		if aerr := addToZip(w, path, filepath.Base(path)); aerr != nil {
			w.Close()
			f.Close()
			return statusErr("Failed: " + aerr.Error())
		}
		backed = append(backed, filepath.Base(path))
	}
	w.Close()
	f.Close()

	if len(backed) == 0 {
		os.Remove(outPath)
		return statusErr("No files found to back up")
	}

	var msg strings.Builder
	for _, name := range backed {
		msg.WriteString("  + " + name + "\n")
	}
	msg.WriteString("\n" + statusOK("Backed up to "+outPath))
	return msg.String()
}

func extractFromZip(f *zip.File, destPath string) error {
	// Guard against path traversal.
	if strings.Contains(f.Name, "..") {
		return fmt.Errorf("suspicious path: %s", f.Name)
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
