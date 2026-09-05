package app

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mafex11/Burnt/windows/ui"
)

func TestAssetServerServesEmbeddedUI(t *testing.T) {
	srv, err := startAssetServer(ui.FS)
	if err != nil {
		t.Fatalf("startAssetServer: %v", err)
	}
	defer srv.Close()

	if !strings.HasSuffix(srv.URL, "/index.html") {
		t.Errorf("URL = %q, want it to point at index.html", srv.URL)
	}
	base := strings.TrimSuffix(srv.URL, "index.html")

	for _, name := range []string{"index.html", "app.js", "styles.css"} {
		body, status := get(t, base+name)
		if status != http.StatusOK {
			t.Errorf("%s: status %d", name, status)
		}
		if len(body) == 0 {
			t.Errorf("%s: empty body", name)
		}
	}

	// mock.js is not embedded; it must still answer 200 with an empty script so the
	// popover's console has no 404 in it.
	body, status := get(t, base+"mock.js")
	if status != http.StatusOK || len(body) != 0 {
		t.Errorf("mock.js: status %d, %d bytes; want 200 and empty", status, len(body))
	}

	// Nothing outside the random prefix is reachable.
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if _, status := get(t, "http://"+u.Host+"/index.html"); status == http.StatusOK {
		t.Error("index.html served outside the random prefix")
	}
}

func TestEmbeddedUIExcludesMockJS(t *testing.T) {
	names, err := ui.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var got []string
	for _, e := range names {
		got = append(got, e.Name())
	}
	want := map[string]bool{"index.html": true, "app.js": true, "styles.css": true}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected embedded file %q (mock.js must never ship)", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("missing embedded file %q", name)
	}
}

func TestOpenLogFileTruncatesWhenTooBig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "burnt.log")

	f, err := openLogFile(path, 32)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	if _, err := f.WriteString(strings.Repeat("a", 64)); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	// Under the cap: appends.
	f, err = openLogFile(path, 1<<20)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	f.WriteString("b")
	f.Close()
	if info, _ := os.Stat(path); info.Size() != 65 {
		t.Errorf("size = %d, want the file appended to", info.Size())
	}

	// Over the cap: starts fresh.
	f, err = openLogFile(path, 32)
	if err != nil {
		t.Fatalf("reopen small: %v", err)
	}
	f.Close()
	if info, _ := os.Stat(path); info.Size() != 0 {
		t.Errorf("size = %d, want a truncated log", info.Size())
	}
}

func get(t *testing.T, url string) ([]byte, int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return body, resp.StatusCode
}
