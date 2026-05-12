package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestUpdateUpstreamLock_WriterNormalization verifies the v0.8 writer
// fix from PRD-reconcile-lock-guard §5.3:
//
//   - `remote:` is derived from the operator-supplied
//     `<remote>/<branch>` ref, not hard-coded to "upstream".
//   - `branch:` records only the branch tail, not the full ref —
//     otherwise the lock-guard reassembles `<remote>/<remote>/<branch>`
//     and refuses every lock written by pre-v0.8 reconciles.
//   - `url:` is populated from `git remote get-url <remote>` so the
//     lock is portable across clones with different remote URLs.
func TestUpdateUpstreamLock_WriterNormalization(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	// Configure a non-default remote name to confirm the writer
	// no longer hard-codes "upstream".
	cmd := exec.Command("git", "remote", "add", "origin", "https://example.invalid/repo.git")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %s: %v", out, err)
	}

	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}

	updateUpstreamLock(s, "origin/feat-branch", "deadbeef")

	data, err := os.ReadFile(filepath.Join(s.TpatchDir(), "upstream.lock"))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `remote: "origin"`) {
		t.Errorf("expected remote: \"origin\":\n%s", content)
	}
	if !strings.Contains(content, `branch: "feat-branch"`) {
		t.Errorf("expected branch: \"feat-branch\":\n%s", content)
	}
	if strings.Contains(content, "branch: origin/feat-branch") {
		t.Errorf("writer regressed: branch contains full ref:\n%s", content)
	}
	if !strings.Contains(content, `url: "https://example.invalid/repo.git"`) {
		t.Errorf("expected populated url:\n%s", content)
	}
}

// TestUpdateUpstreamLock_MalformedRefSkipsWrite verifies that the
// writer refuses to corrupt an existing lock when handed a ref it
// cannot split (no slash, or multiple slashes).
func TestUpdateUpstreamLock_MalformedRefSkipsWrite(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}

	// Pre-seed a known-good lock.
	lockPath := filepath.Join(s.TpatchDir(), "upstream.lock")
	preExisting := "# pre-existing\nremote: \"upstream\"\nbranch: \"main\"\ncommit: \"abc123\"\nurl: \"\"\n"
	if err := os.WriteFile(lockPath, []byte(preExisting), 0o644); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	updateUpstreamLock(s, "no-slash-here", "newsha")

	got, _ := os.ReadFile(lockPath)
	if string(got) != preExisting {
		t.Errorf("malformed ref should have skipped write, but file changed:\n%s", got)
	}
}

// TestUpdateUpstreamLock_NoRemoteURL_StillWritesLock verifies that a
// missing remote URL (e.g. local-only repo) does not block the writer.
// The url field is empty but the lock is still produced.
func TestUpdateUpstreamLock_NoRemoteURL_StillWritesLock(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}

	// No `git remote add` — the named remote does not exist.
	updateUpstreamLock(s, "upstream/main", "feedface")

	data, err := os.ReadFile(filepath.Join(s.TpatchDir(), "upstream.lock"))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `remote: "upstream"`) {
		t.Errorf("expected remote: \"upstream\":\n%s", content)
	}
	if !strings.Contains(content, `branch: "main"`) {
		t.Errorf("expected branch: \"main\":\n%s", content)
	}
	if !strings.Contains(content, `url: ""`) {
		t.Errorf("expected empty url:\n%s", content)
	}
}
