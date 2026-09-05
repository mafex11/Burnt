package engine

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sessionRow(id string, cost float64, tokens int64) SessionRow {
	return SessionRow{Agent: "claude", Period: id, TotalCost: cost, TotalTokens: tokens}
}

func TestGroupProjectsByCwdLeaf(t *testing.T) {
	got := GroupProjects(
		[]SessionRow{sessionRow("a", 5, 100), sessionRow("b", 3, 50), sessionRow("c", 2, 20)},
		map[string]string{
			"a": "/Users/me/code/personal",
			"b": "/Users/me/code/personal",
			"c": "/Users/me/code/work",
		})
	if len(got) != 2 {
		t.Fatalf("got %d slices, want 2: %+v", len(got), got)
	}
	if got[0].Project != "personal" || got[1].Project != "work" {
		t.Errorf("names = %q/%q, want personal/work", got[0].Project, got[1].Project)
	}
	if math.Abs(got[0].Cost-8) > 0.001 {
		t.Errorf("personal cost = %v, want 8", got[0].Cost)
	}
	if got[0].Tokens != 150 {
		t.Errorf("personal tokens = %d, want 150", got[0].Tokens)
	}
	if math.Abs(got[1].Cost-2) > 0.001 {
		t.Errorf("work cost = %v, want 2", got[1].Cost)
	}
}

func TestGroupProjectsBucketsUnmappedAsUnknown(t *testing.T) {
	got := GroupProjects(
		[]SessionRow{sessionRow("a", 5, 100), sessionRow("z", 4, 40)},
		map[string]string{"a": "/Users/me/code/personal"})
	var unknown *ProjectSlice
	for i := range got {
		if got[i].Project == "Unknown" {
			unknown = &got[i]
		}
	}
	if unknown == nil {
		t.Fatalf("no Unknown bucket in %+v", got)
	}
	if math.Abs(unknown.Cost-4) > 0.001 {
		t.Errorf("Unknown cost = %v, want 4", unknown.Cost)
	}
}

func TestGroupProjectsDisambiguatesLeafCollision(t *testing.T) {
	got := GroupProjects(
		[]SessionRow{sessionRow("a", 5, 10), sessionRow("b", 3, 10)},
		map[string]string{"a": "/x/api", "b": "/y/api"})
	names := map[string]bool{}
	for _, p := range got {
		names[p.Project] = true
	}
	if !names["x/api"] || !names["y/api"] {
		t.Errorf("names = %v, want x/api and y/api", names)
	}
}

// Windows cwds use backslashes; the leaf logic has to handle both separators.
func TestGroupProjectsHandlesWindowsPaths(t *testing.T) {
	got := GroupProjects(
		[]SessionRow{sessionRow("a", 5, 10)},
		map[string]string{"a": `C:\Users\me\code\burnt`})
	if got[0].Project != "burnt" {
		t.Errorf("Project = %q, want burnt", got[0].Project)
	}
}

func TestGroupProjectsIsDeterministicOnTiedCosts(t *testing.T) {
	sessions := []SessionRow{sessionRow("a", 5, 10), sessionRow("b", 5, 10)}
	cwds := map[string]string{"a": "/p/alpha", "b": "/p/beta"}
	first := GroupProjects(sessions, cwds)
	for i := 0; i < 20; i++ {
		again := GroupProjects(sessions, cwds)
		for j := range first {
			if first[j].Project != again[j].Project {
				t.Fatalf("ordering is not deterministic: %+v vs %+v", first, again)
			}
		}
	}
}

// claudeLog writes a Claude session log and returns its session id.
func claudeLog(t *testing.T, root, project, sid, contents string) string {
	t.Helper()
	dir := filepath.Join(root, ".claude", "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, sid+".jsonl")
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAttributeProjectsReadsCwdBeyondHugeFirstLine(t *testing.T) {
	root := t.TempDir()
	sid := "11111111-2222-3333-4444-555555555555"
	// Line 1 is a giant cwd-less object; line 2 carries the cwd.
	line1 := `{"type":"user","blob":"` + strings.Repeat("x", 80_000) + `"}`
	line2 := `{"type":"assistant","cwd":"/Users/me/code/myproj"}`
	claudeLog(t, root, "somedir", sid, line1+"\n"+line2+"\n")
	if err := os.MkdirAll(filepath.Join(root, ".codex", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := AttributeProjects(filepath.Join(root, ".claude"), filepath.Join(root, ".codex"))
	if got[sid] != "/Users/me/code/myproj" {
		t.Errorf("cwd for %s = %q, want /Users/me/code/myproj", sid, got[sid])
	}
}

// A session log's cwd is immutable once written, so a parsed file is cached by
// (path, mtime). Proof: read once, blank the contents while preserving the mtime, read
// again — the cached cwd survives, showing we didn't re-parse.
func TestAttributeProjectsCachesUnchangedFileByMtime(t *testing.T) {
	root := t.TempDir()
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	p := claudeLog(t, root, "d", sid, `{"cwd":"/Users/me/code/cached"}`+"\n")
	if err := os.MkdirAll(filepath.Join(root, ".codex", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pin a whole-second mtime so it round-trips exactly through the filesystem.
	mtime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	claudeDir, codexDir := filepath.Join(root, ".claude"), filepath.Join(root, ".codex")
	if got := AttributeProjects(claudeDir, codexDir); got[sid] != "/Users/me/code/cached" {
		t.Fatalf("first pass cwd = %q, want /Users/me/code/cached", got[sid])
	}

	if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if got := AttributeProjects(claudeDir, codexDir); got[sid] != "/Users/me/code/cached" {
		t.Errorf("second pass cwd = %q, want the cached value served without re-parsing", got[sid])
	}
}

func TestAttributeProjectsRereadsWhenMtimeMoves(t *testing.T) {
	root := t.TempDir()
	sid := "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
	p := claudeLog(t, root, "d", sid, `{"cwd":"/first"}`+"\n")
	claudeDir, codexDir := filepath.Join(root, ".claude"), filepath.Join(root, ".codex")
	if got := AttributeProjects(claudeDir, codexDir); got[sid] != "/first" {
		t.Fatalf("first pass cwd = %q, want /first", got[sid])
	}
	if err := os.WriteFile(p, []byte(`{"cwd":"/second"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	if got := AttributeProjects(claudeDir, codexDir); got[sid] != "/second" {
		t.Errorf("second pass cwd = %q, want /second after the mtime moved", got[sid])
	}
}

func TestAttributeProjectsReadsCodexRollouts(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".codex", "sessions", "2026", "06", "08")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Codex nests session_meta under "payload" and names the file, not the session.
	body := `{"type":"session_meta","payload":{"id":"019c0ec2-6c8f-7df2-943e-506d7d4c0c82","cwd":"/Users/me/code/burnt"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-06-08T10-00-00.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-rollout file in the same tree must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "other.jsonl"), []byte(`{"id":"nope","cwd":"/nope"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := AttributeProjects(filepath.Join(root, ".claude"), filepath.Join(root, ".codex"))
	if got["019c0ec2-6c8f-7df2-943e-506d7d4c0c82"] != "/Users/me/code/burnt" {
		t.Errorf("codex cwd map = %v, want the rollout's cwd", got)
	}
	if _, ok := got["nope"]; ok {
		t.Error("a non-rollout file was scanned")
	}
}

func TestAttributeProjectsAcceptsTopLevelCodexMeta(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".codex", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"flat-id","cwd":"/flat"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-flat.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := AttributeProjects("", filepath.Join(root, ".codex"))
	if got["flat-id"] != "/flat" {
		t.Errorf("cwd map = %v, want flat-id -> /flat", got)
	}
}

func TestAttributeProjectsToleratesMissingDirs(t *testing.T) {
	got := AttributeProjects(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "nope"))
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map", got)
	}
}

func TestDefaultProjectDirsUnderHome(t *testing.T) {
	claudeDir, codexDir := DefaultProjectDirs()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	if claudeDir != filepath.Join(home, ".claude") {
		t.Errorf("claudeDir = %q, want %q", claudeDir, filepath.Join(home, ".claude"))
	}
	if codexDir != filepath.Join(home, ".codex") {
		t.Errorf("codexDir = %q, want %q", codexDir, filepath.Join(home, ".codex"))
	}
}
