package engine

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// claudeScanLines is how far into a Claude jsonl to look for cwd. Claude puts cwd
	// on a message line (often line 2-3), and line 1 can be very large.
	claudeScanLines = 60
	// codexScanLines is how far into a Codex rollout to look for session_meta.
	codexScanLines = 5
	// maxLineBytes caps a single buffered line; a pathological line is skipped rather
	// than read into memory unbounded.
	maxLineBytes = 8 << 20
)

// GroupProjects joins session costs with a session-id -> cwd map and rolls them up by
// working directory, sorted by cost desc. Unmapped sessions land in "Unknown".
func GroupProjects(sessions []SessionRow, cwdBySession map[string]string) []ProjectSlice {
	type agg struct {
		cost   float64
		tokens int64
	}
	byPath := map[string]*agg{}
	for _, s := range sessions {
		p := cwdBySession[s.Period]
		a, ok := byPath[p]
		if !ok {
			a = &agg{}
			byPath[p] = a
		}
		a.cost += s.TotalCost
		a.tokens += s.TotalTokens
	}

	// A leaf name shared by two different paths ("api" under both /x and /y) is
	// ambiguous, so those get their parent prepended.
	leafCounts := map[string]int{}
	for p := range byPath {
		if p != "" {
			leafCounts[leafName(p)]++
		}
	}

	out := make([]ProjectSlice, 0, len(byPath))
	for p, a := range byPath {
		out = append(out, ProjectSlice{
			Project: projectDisplayName(p, leafCounts),
			Path:    p,
			Cost:    a.cost,
			Tokens:  a.tokens,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cost != out[j].Cost {
			return out[i].Cost > out[j].Cost
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// projectDisplayName is the label for a cwd: its leaf, disambiguated with the parent
// directory when another path shares the same leaf.
func projectDisplayName(p string, leafCounts map[string]int) string {
	if p == "" {
		return "Unknown"
	}
	leaf := leafName(p)
	if leafCounts[leaf] > 1 {
		if parent := leafName(parentPath(p)); parent != "" {
			return parent + "/" + leaf
		}
	}
	return leaf
}

// leafName is the last path component, handling both separators since cwds recorded
// on Windows and Unix both flow through here.
func leafName(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// parentPath drops the last component of p.
func parentPath(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i]
	}
	return ""
}

// DefaultProjectDirs are the Claude and Codex roots under the user's home, i.e.
// %USERPROFILE%\.claude and %USERPROFILE%\.codex on Windows.
func DefaultProjectDirs() (claudeDir, codexDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	return filepath.Join(home, ".claude"), filepath.Join(home, ".codex")
}

// cachedCwd is one parsed session log, remembered so unchanged files are never
// re-parsed.
type cachedCwd struct {
	sessionID string
	cwd       string
	mtime     time.Time
}

// cwdCache is keyed by file path. A session log's cwd never changes after the first
// lines are written, so once a file is parsed we only stat it on later scans. That
// turns the second and later builds from "open 2,400 files" into "stat 2,400 files,
// open the handful that changed".
var (
	cwdCacheMu sync.Mutex
	cwdCache   = map[string]cachedCwd{}
)

// AttributeProjects builds a session-id -> cwd map by reading the first cwd-bearing
// line of every Claude and Codex session log. Files whose mtime hasn't moved since the
// last call are served from cache. Missing directories are not an error: a machine may
// use only one of the two tools.
func AttributeProjects(claudeDir, codexDir string) map[string]string {
	out := map[string]string{}
	if claudeDir != "" {
		scanClaudeLogs(filepath.Join(claudeDir, "projects"), out)
	}
	if codexDir != "" {
		scanCodexLogs(filepath.Join(codexDir, "sessions"), out)
	}
	return out
}

// scanClaudeLogs walks ~/.claude/projects/<project>/<session>.jsonl (one level deep,
// as Claude Code writes it). The session id is the file name.
func scanClaudeLogs(root string, out map[string]string) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, dir.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}
			p := filepath.Join(root, dir.Name(), f.Name())
			if hit, ok := lookupCache(p); ok {
				out[hit.sessionID] = hit.cwd
				continue
			}
			cwd, ok := firstCwd(p)
			if !ok {
				continue
			}
			sid := strings.TrimSuffix(f.Name(), ".jsonl")
			out[sid] = cwd
			storeCache(p, sid, cwd)
		}
	}
}

// scanCodexLogs walks ~/.codex/sessions/** for rollout-*.jsonl. Codex records the
// session id inside the file, not in its name.
func scanCodexLogs(root string, out map[string]string) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip it, keep scanning the rest
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || filepath.Ext(name) != ".jsonl" {
			return nil
		}
		if hit, ok := lookupCache(p); ok {
			out[hit.sessionID] = hit.cwd
			return nil
		}
		sid, cwd, ok := codexIDAndCwd(p)
		if !ok {
			return nil
		}
		out[sid] = cwd
		storeCache(p, sid, cwd)
		return nil
	})
}

// lookupCache returns the cached entry for p iff its mtime matches what we last saw.
func lookupCache(p string) (cachedCwd, bool) {
	info, err := os.Stat(p)
	if err != nil {
		return cachedCwd{}, false
	}
	cwdCacheMu.Lock()
	defer cwdCacheMu.Unlock()
	e, ok := cwdCache[p]
	if !ok || !e.mtime.Equal(info.ModTime()) {
		return cachedCwd{}, false
	}
	return e, true
}

// storeCache remembers a parsed file keyed by its current mtime.
func storeCache(p, sessionID, cwd string) {
	info, err := os.Stat(p)
	if err != nil {
		return
	}
	cwdCacheMu.Lock()
	defer cwdCacheMu.Unlock()
	cwdCache[p] = cachedCwd{sessionID: sessionID, cwd: cwd, mtime: info.ModTime()}
}

// firstCwd returns the first "cwd" value in a Claude jsonl.
func firstCwd(p string) (string, bool) {
	var found string
	scanLines(p, claudeScanLines, func(line []byte) bool {
		var obj struct {
			Cwd *string `json:"cwd"`
		}
		if json.Unmarshal(line, &obj) == nil && obj.Cwd != nil && *obj.Cwd != "" {
			found = *obj.Cwd
			return false
		}
		return true
	})
	return found, found != ""
}

// codexIDAndCwd reads a Codex rollout's session_meta (typically line 1) for its id and
// cwd. The fields may sit at the top level or nested under "payload".
func codexIDAndCwd(p string) (string, string, bool) {
	var id, cwd string
	scanLines(p, codexScanLines, func(line []byte) bool {
		var obj struct {
			ID      *string `json:"id"`
			Cwd     *string `json:"cwd"`
			Payload *struct {
				ID  *string `json:"id"`
				Cwd *string `json:"cwd"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &obj) != nil {
			return true
		}
		gotID, gotCwd := obj.ID, obj.Cwd
		if obj.Payload != nil {
			gotID, gotCwd = obj.Payload.ID, obj.Payload.Cwd
		}
		if gotID != nil && gotCwd != nil && *gotID != "" && *gotCwd != "" {
			id, cwd = *gotID, *gotCwd
			return false
		}
		return true
	})
	return id, cwd, id != "" && cwd != ""
}

// scanLines streams up to maxLines newline-delimited lines from p, calling visit until
// it returns false. Lines longer than maxLineBytes end the scan rather than blowing up
// memory.
func scanLines(p string, maxLines int, visit func(line []byte) bool) {
	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	for i := 0; i < maxLines && scanner.Scan(); i++ {
		if !visit(scanner.Bytes()) {
			return
		}
	}
}
