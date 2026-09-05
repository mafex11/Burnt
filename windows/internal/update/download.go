package update

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxAssetBytes caps a release download; the real zip is ~12 MB.
const maxAssetBytes = 256 << 20

// Download streams url into destPath. It writes to a sibling temp file first and
// renames on success, so destPath never exists in a half-written state.
func Download(ctx context.Context, client *http.Client, url, destPath string) error {
	if client == nil {
		client = http.DefaultClient
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent(""))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".download-*")
	if err != nil {
		return fmt.Errorf("create temp download: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, maxAssetBytes)); err != nil {
		tmp.Close()
		return fmt.Errorf("write download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close download: %w", err)
	}
	_ = os.Remove(destPath) // Windows rename will not clobber
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("move download into place: %w", err)
	}
	return nil
}

// ExtractZip unpacks zipPath into destDir, creating it if needed. Entries whose
// path escapes destDir (zip slip) or that are symlinks are rejected.
func ExtractZip(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer zr.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve extract dir: %w", err)
	}

	for _, f := range zr.File {
		target, err := safeJoin(absDest, f.Name)
		if err != nil {
			return err
		}
		info := f.FileInfo()
		switch {
		case info.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", f.Name, err)
			}
		case !info.Mode().IsRegular():
			// Symlinks and devices could redirect writes outside destDir.
			return fmt.Errorf("refusing zip entry %q: unsupported mode %s", f.Name, info.Mode())
		default:
			if err := extractFile(f, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// safeJoin resolves name inside absDest, refusing absolute paths and any ".."
// that would climb out of the destination directory.
func safeJoin(absDest, name string) (string, error) {
	clean := filepath.Clean(strings.ReplaceAll(name, `\`, "/"))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing zip entry %q: escapes destination", name)
	}
	target := filepath.Join(absDest, clean)
	rel, err := filepath.Rel(absDest, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing zip entry %q: escapes destination", name)
	}
	return target, nil
}

func extractFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", f.Name, err)
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer rc.Close()

	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(out, io.LimitReader(rc, maxAssetBytes)); err != nil {
		out.Close()
		return fmt.Errorf("write %s: %w", target, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", target, err)
	}
	return nil
}
