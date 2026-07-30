package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestSessionPurgeRefusesNoSlugNoAll enforces the F-INT-γ-1 HIGH
// closure: PRD §6 D14 lists the `--all` / `<slug>` pair as a mutex,
// which implies "one of" semantics. Rev-0 permitted `session purge`
// with neither, which would iterate every session under every feature
// (an unbounded blast radius the operator hadn't asked for).
//
// Rev-1 refuses. Contract:
//   - Non-zero exit.
//   - Stderr cites PRD §6 D14 and names both remediation options.
//   - Zero filesystem mutation: pre-run and post-run recursive hash
//     of the .tpatch/ tree MUST be identical.
func TestSessionPurgeRefusesNoSlugNoAll(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Purge no-args refusal")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}

	tpatchRoot := filepath.Join(tmp, ".tpatch")
	before := hashDirTree(t, tpatchRoot)

	_, stderr, code := runSessionCmd("session", "purge", "--path", tmp, "--yes")
	if code == 0 {
		t.Fatalf("expected session purge with neither <slug> nor --all to refuse; got success (stderr=%q)", stderr)
	}
	if !strings.Contains(stderr, "PRD §6 D14") {
		t.Fatalf("expected stderr to cite PRD §6 D14; got %q", stderr)
	}
	if !strings.Contains(stderr, "--all") {
		t.Fatalf("expected stderr to name --all remediation; got %q", stderr)
	}
	if !strings.Contains(stderr, "<slug>") {
		t.Fatalf("expected stderr to name <slug> remediation; got %q", stderr)
	}

	after := hashDirTree(t, tpatchRoot)
	if before != after {
		t.Fatalf("session purge refusal MUST NOT mutate .tpatch/; hash changed:\n  before=%s\n  after =%s", before, after)
	}
}

// hashDirTree walks a directory tree deterministically (lexicographic
// path order) and returns the hex sha256 of "path=size\n" lines. This
// is sensitive to file creation, deletion, and size change — enough to
// prove "no filesystem mutation" without exhaustively hashing every
// file body.
func hashDirTree(t *testing.T, root string) string {
	t.Helper()
	type entry struct {
		path string
		size int64
	}
	var entries []entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		size := int64(0)
		if !d.IsDir() {
			size = info.Size()
		}
		entries = append(entries, entry{path: rel, size: size})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.path))
		h.Write([]byte("="))
		h.Write([]byte(strconv.FormatInt(e.size, 10)))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
