package update

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownload(t *testing.T) {
	payload := bytes.Repeat([]byte("burnt"), 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent must be set")
		}
		w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "nested", "Burnt.zip")
	if err := Download(context.Background(), srv.Client(), srv.URL, dest); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("downloaded %d bytes, want %d", len(got), len(payload))
	}
	entries, err := os.ReadDir(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestDownloadOverwritesAndErrors(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "Burnt.zip")
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new"))
	}))
	defer ok.Close()
	if err := Download(context.Background(), ok.Client(), ok.URL, dest); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "new" {
		t.Errorf("dest = %q, want new", b)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer bad.Close()
	if err := Download(context.Background(), bad.Client(), bad.URL, filepath.Join(dir, "x.zip")); err == nil {
		t.Error("expected an error for 404")
	}
	if _, err := os.Stat(filepath.Join(dir, "x.zip")); err == nil {
		t.Error("failed download must not leave a file")
	}
}

// writeZip builds a zip from name -> contents pairs.
func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	writeZip(t, zipPath, map[string]string{
		"burnt.exe":          "MZ-burnt",
		"ccusage.exe":        "MZ-ccusage",
		"docs/README.txt":    "hello",
		"empty/nested/f.txt": "x",
	})

	out := filepath.Join(dir, "out")
	if err := ExtractZip(zipPath, out); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	for name, want := range map[string]string{
		"burnt.exe":          "MZ-burnt",
		"ccusage.exe":        "MZ-ccusage",
		"docs/README.txt":    "hello",
		"empty/nested/f.txt": "x",
	} {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestExtractZipRejectsZipSlip(t *testing.T) {
	for _, evil := range []string{
		"../escaped.txt",
		"../../escaped.txt",
		"docs/../../escaped.txt",
		`..\escaped.txt`,
		"/abs.txt",
	} {
		dir := t.TempDir()
		zipPath := filepath.Join(dir, "evil.zip")
		writeZip(t, zipPath, map[string]string{evil: "pwned"})
		out := filepath.Join(dir, "out")
		// Absolute names are rejected too: a valid zip only holds relative paths.
		if err := ExtractZip(zipPath, out); err == nil {
			t.Errorf("%q: expected rejection", evil)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "escaped.txt")); statErr == nil {
			t.Errorf("%q: wrote outside the destination", evil)
		}
	}
}

func TestExtractZipMissingFile(t *testing.T) {
	if err := ExtractZip(filepath.Join(t.TempDir(), "nope.zip"), t.TempDir()); err == nil {
		t.Error("expected an error for a missing zip")
	}
}

func TestBuildUpdateScript(t *testing.T) {
	script := buildUpdateScript(4242,
		`C:\Users\mafex\AppData\Local\Programs\Burnt\.update`,
		`C:\Users\mafex\AppData\Local\Programs\Burnt`,
		`C:\Users\mafex\AppData\Local\Programs\Burnt\burnt.exe`)

	for _, want := range []string{
		"@echo off",
		"set PID=4242",
		`set "UPDATE_DIR=C:\Users\mafex\AppData\Local\Programs\Burnt\.update"`,
		`set "INSTALL_DIR=C:\Users\mafex\AppData\Local\Programs\Burnt"`,
		`set "EXE=C:\Users\mafex\AppData\Local\Programs\Burnt\burnt.exe"`,
		`tasklist /FI "PID eq %PID%" | find "%PID%"`,
		"goto waitloop",
		`robocopy "%UPDATE_DIR%" "%INSTALL_DIR%" /E /IS /IT /MOVE`,
		`xcopy "%UPDATE_DIR%\*" "%INSTALL_DIR%\" /E /Y /I`,
		`rmdir /S /Q "%UPDATE_DIR%"`,
		`start "" "%EXE%"`,
		`del "%~f0"`,
	} {
		if !bytes.Contains([]byte(script), []byte(want)) {
			t.Errorf("script missing %q\n---\n%s", want, script)
		}
	}
	if !bytes.Contains([]byte(script), []byte("\r\n")) {
		t.Error("script should use CRLF line endings")
	}
	// The wait must come before the copy, or we clobber a running exe.
	if bytes.Index([]byte(script), []byte(":waitloop")) > bytes.Index([]byte(script), []byte("robocopy")) {
		t.Error("wait loop must precede robocopy")
	}
}
