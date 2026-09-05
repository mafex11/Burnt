// Package update checks GitHub releases for a newer Burnt build, downloads the
// Windows asset, and (on Windows) hands the swap over to a detached script.
//
// Everything except apply_windows.go is portable so it can be tested on macOS.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// DefaultAPIURL is the GitHub endpoint describing the newest published release.
const DefaultAPIURL = "https://api.github.com/repos/mafex11/Burnt/releases/latest"

// AssetName is the release asset the Windows updater knows how to install.
const AssetName = "Burnt-windows-x64.zip"

// maxBodyBytes caps how much of the API response we are willing to read.
const maxBodyBytes = 4 << 20

// Release describes one published GitHub release.
type Release struct {
	Tag      string `json:"tag"`      // as published, e.g. "v1.3.0"
	Version  string `json:"version"`  // tag without the leading "v"
	AssetURL string `json:"assetUrl"` // browser_download_url of AssetName
	HTMLURL  string `json:"htmlUrl"`  // release page, for "View release notes"
}

// Result is the outcome of a check. Latest is non-nil whenever the API returned
// a usable release, even if it is not newer than the running build.
type Result struct {
	Available bool     `json:"available"`
	Latest    *Release `json:"latest,omitempty"`
}

// githubRelease is the subset of the API payload we care about.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check asks apiURL for the latest release and compares it to currentVersion.
//
// A release only counts when it ships an asset named AssetName; without it
// there is nothing the updater could install, so Available stays false. A
// currentVersion of "" or "dev" (an unversioned local build) never reports an
// update, so developer builds are not nagged into replacing themselves.
func Check(ctx context.Context, client *http.Client, apiURL, currentVersion string) (Result, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build update request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent(currentVersion))
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("fetch latest release: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return Result{}, fmt.Errorf("read latest release: %w", err)
	}

	var gh githubRelease
	if err := json.Unmarshal(body, &gh); err != nil {
		return Result{}, fmt.Errorf("parse latest release: %w", err)
	}
	if gh.TagName == "" {
		return Result{}, fmt.Errorf("parse latest release: missing tag_name")
	}

	rel := Release{
		Tag:     gh.TagName,
		Version: strings.TrimPrefix(gh.TagName, "v"),
		HTMLURL: gh.HTMLURL,
	}
	for _, a := range gh.Assets {
		if a.Name == AssetName {
			rel.AssetURL = a.URL
			break
		}
	}
	if rel.AssetURL == "" {
		// Nothing installable (mac-only release, or assets still uploading).
		return Result{Available: false, Latest: &rel}, nil
	}

	return Result{Available: IsNewer(currentVersion, rel.Version), Latest: &rel}, nil
}

// UserAgent is the User-Agent header Burnt sends to GitHub.
func UserAgent(version string) string {
	if version == "" {
		version = "dev"
	}
	return "Burnt-Windows/" + version
}

// IsNewer reports whether latest is strictly greater than current using
// component-wise numeric comparison. A leading "v" is tolerated on either side,
// missing or non-numeric components count as 0, and an unversioned current
// build ("" or "dev") is treated as already newest.
func IsNewer(current, latest string) bool {
	switch strings.ToLower(strings.TrimSpace(current)) {
	case "", "dev":
		return false
	}
	c, l := versionParts(current), versionParts(latest)
	n := max(len(c), len(l))
	for i := range n {
		var a, b int
		if i < len(c) {
			a = c[i]
		}
		if i < len(l) {
			b = l[i]
		}
		if b != a {
			return b > a
		}
	}
	return false
}

// versionParts splits "v1.2.3-beta.1" into [1 2 3], dropping any pre-release or
// build suffix and mapping unparseable components to 0.
func versionParts(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	fields := strings.Split(v, ".")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
