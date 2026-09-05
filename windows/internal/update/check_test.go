package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// releaseJSON builds a GitHub releases/latest payload.
func releaseJSON(tag string, assets ...string) string {
	body := ""
	for i, name := range assets {
		if i > 0 {
			body += ","
		}
		body += fmt.Sprintf(`{"name":%q,"browser_download_url":"https://example.test/%s"}`, name, name)
	}
	return fmt.Sprintf(`{"tag_name":%q,"html_url":"https://example.test/releases/%s","assets":[%s]}`,
		tag, tag, body)
}

func serve(t *testing.T, status int, body string) (*httptest.Server, *[]string) {
	t.Helper()
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.Header.Get("User-Agent"))
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &agents
}

func TestCheckUpdateAvailable(t *testing.T) {
	srv, agents := serve(t, 200, releaseJSON("v1.4.0", "Burnt.zip", AssetName))
	got, err := Check(context.Background(), srv.Client(), srv.URL, "1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available {
		t.Error("expected an update to be available")
	}
	if got.Latest == nil {
		t.Fatal("Latest should be set")
	}
	if got.Latest.Tag != "v1.4.0" || got.Latest.Version != "1.4.0" {
		t.Errorf("tag/version = %q/%q", got.Latest.Tag, got.Latest.Version)
	}
	if got.Latest.AssetURL != "https://example.test/"+AssetName {
		t.Errorf("assetURL = %q", got.Latest.AssetURL)
	}
	if got.Latest.HTMLURL != "https://example.test/releases/v1.4.0" {
		t.Errorf("htmlURL = %q", got.Latest.HTMLURL)
	}
	if len(*agents) != 1 || (*agents)[0] != "Burnt-Windows/1.3.0" {
		t.Errorf("User-Agent = %v, want Burnt-Windows/1.3.0", *agents)
	}
}

func TestCheckUpToDate(t *testing.T) {
	srv, _ := serve(t, 200, releaseJSON("v1.3.0", AssetName))
	for _, current := range []string{"1.3.0", "v1.3.0", "1.4.0"} {
		got, err := Check(context.Background(), srv.Client(), srv.URL, current)
		if err != nil {
			t.Fatalf("current %q: %v", current, err)
		}
		if got.Available {
			t.Errorf("current %q: should not report an update", current)
		}
		if got.Latest == nil {
			t.Errorf("current %q: Latest should still be reported", current)
		}
	}
}

func TestCheckRequiresWindowsAsset(t *testing.T) {
	srv, _ := serve(t, 200, releaseJSON("v9.0.0", "Burnt.zip", "Burnt-macos.zip"))
	got, err := Check(context.Background(), srv.Client(), srv.URL, "1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Available {
		t.Error("no Windows asset means nothing to install")
	}
	if got.Latest == nil || got.Latest.AssetURL != "" {
		t.Errorf("Latest = %+v, want an entry with no assetURL", got.Latest)
	}
}

func TestCheckDevBuildNeverUpdates(t *testing.T) {
	srv, agents := serve(t, 200, releaseJSON("v9.9.9", AssetName))
	for _, current := range []string{"", "dev", "DEV"} {
		got, err := Check(context.Background(), srv.Client(), srv.URL, current)
		if err != nil {
			t.Fatalf("current %q: %v", current, err)
		}
		if got.Available {
			t.Errorf("current %q: dev builds should not self-update", current)
		}
	}
	for _, a := range *agents {
		if a == "" {
			t.Error("User-Agent must always be set")
		}
	}
	if (*agents)[0] != "Burnt-Windows/dev" {
		t.Errorf("empty version UA = %q, want Burnt-Windows/dev", (*agents)[0])
	}
}

func TestCheckErrors(t *testing.T) {
	t.Run("http status", func(t *testing.T) {
		srv, _ := serve(t, 404, `{"message":"Not Found"}`)
		if _, err := Check(context.Background(), srv.Client(), srv.URL, "1.0.0"); err == nil {
			t.Fatal("expected an error for 404")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		srv, _ := serve(t, 200, `{nope`)
		if _, err := Check(context.Background(), srv.Client(), srv.URL, "1.0.0"); err == nil {
			t.Fatal("expected an error for malformed JSON")
		}
	})
	t.Run("missing tag", func(t *testing.T) {
		srv, _ := serve(t, 200, `{"assets":[]}`)
		if _, err := Check(context.Background(), srv.Client(), srv.URL, "1.0.0"); err == nil {
			t.Fatal("expected an error when tag_name is absent")
		}
	})
	t.Run("cancelled context", func(t *testing.T) {
		srv, _ := serve(t, 200, releaseJSON("v2.0.0", AssetName))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := Check(ctx, srv.Client(), srv.URL, "1.0.0"); err == nil {
			t.Fatal("expected an error for a cancelled context")
		}
	})
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"1.2.3", "1.2.4", true},
		{"1.2.3", "1.3.0", true},
		{"1.2.3", "2.0.0", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.2", false},
		{"1.2.3", "1.1.9", false},
		{"v1.2.3", "v1.2.4", true},
		{"1.2.3", "v1.2.3", false},
		{"1.2", "1.2.1", true},
		{"1.2.1", "1.2", false},
		{"1.2.0", "1.2", false},
		{"1.9.0", "1.10.0", true},
		{"1.2.3", "1.2.4-beta.1", true},
		{"1.2.3-beta.1", "1.2.3", false},
		{"1.2.3", "garbage", false},
		{"dev", "9.9.9", false},
		{"", "9.9.9", false},
		{"  1.2.3  ", "1.2.4", true},
	}
	for _, tt := range tests {
		if got := IsNewer(tt.current, tt.latest); got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestDefaultAPIURL(t *testing.T) {
	if DefaultAPIURL != "https://api.github.com/repos/mafex11/Burnt/releases/latest" {
		t.Errorf("DefaultAPIURL = %q", DefaultAPIURL)
	}
	if AssetName != "Burnt-windows-x64.zip" {
		t.Errorf("AssetName = %q", AssetName)
	}
}
