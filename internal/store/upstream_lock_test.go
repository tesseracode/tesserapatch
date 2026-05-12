package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestParseUpstreamLock_Empty(t *testing.T) {
	lock := ParseUpstreamLock("")
	if lock != (UpstreamLock{}) {
		t.Fatalf("empty content should parse to zero value, got %+v", lock)
	}
}

func TestParseUpstreamLock_AllFields(t *testing.T) {
	content := `# Upstream Lock
remote: origin
branch: main
commit: 1a2b3c4d5e6f
url: "https://github.com/example/repo.git"
`
	lock := ParseUpstreamLock(content)
	want := UpstreamLock{
		Remote: "origin",
		Branch: "main",
		Commit: "1a2b3c4d5e6f",
		URL:    "https://github.com/example/repo.git",
	}
	if lock != want {
		t.Fatalf("full parse mismatch:\n got: %+v\nwant: %+v", lock, want)
	}
}

func TestParseUpstreamLock_MissingFields(t *testing.T) {
	// Only `commit:` is populated — the others stay empty.
	content := `remote: ""
branch: ""
commit: deadbeef
url: ""
`
	lock := ParseUpstreamLock(content)
	if lock.Commit != "deadbeef" {
		t.Errorf("commit: got %q want deadbeef", lock.Commit)
	}
	if lock.Remote != "" || lock.Branch != "" || lock.URL != "" {
		t.Errorf("expected blank remote/branch/url, got %+v", lock)
	}
}

func TestParseUpstreamLock_Malformed(t *testing.T) {
	// Lines without colons, naked keys, and pure comments must not
	// crash or pollute the struct.
	content := `# header comment
this is not a yaml line
remote
branch:
commit: abc123
   url:    "ssh://git@host/repo"   # trailing comment
random: value
`
	lock := ParseUpstreamLock(content)
	if lock.Commit != "abc123" {
		t.Errorf("commit: got %q want abc123", lock.Commit)
	}
	if lock.URL != "ssh://git@host/repo" {
		t.Errorf("url: got %q want ssh://git@host/repo", lock.URL)
	}
	if lock.Remote != "" {
		t.Errorf("remote should be empty (naked key has no colon), got %q", lock.Remote)
	}
	if lock.Branch != "" {
		t.Errorf("branch should be empty (no value after colon), got %q", lock.Branch)
	}
}

func TestParseUpstreamLock_TrailingWhitespace(t *testing.T) {
	content := "remote:   origin   \nbranch:\tmain\t\ncommit:  1a2b3c   \nurl:   \n"
	lock := ParseUpstreamLock(content)
	if lock.Remote != "origin" || lock.Branch != "main" || lock.Commit != "1a2b3c" {
		t.Fatalf("whitespace handling failed: %+v", lock)
	}
	if lock.URL != "" {
		t.Errorf("blank url should parse to empty string, got %q", lock.URL)
	}
}

func TestLoadUpstreamLock_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	s := &Store{Root: tmp}
	if err := os.MkdirAll(filepath.Join(tmp, ".tpatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUpstreamLock(s)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file should return fs.ErrNotExist, got %v", err)
	}
}

func TestLoadUpstreamLock_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	s := &Store{Root: tmp}
	if err := os.MkdirAll(filepath.Join(tmp, ".tpatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "remote: upstream\nbranch: main\ncommit: cafebabe\nurl: \"https://example.org\"\n"
	if err := os.WriteFile(s.UpstreamLockPath(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := LoadUpstreamLock(s)
	if err != nil {
		t.Fatalf("LoadUpstreamLock: %v", err)
	}
	if lock.Remote != "upstream" || lock.Branch != "main" ||
		lock.Commit != "cafebabe" || lock.URL != "https://example.org" {
		t.Fatalf("round-trip mismatch: %+v", lock)
	}
}
