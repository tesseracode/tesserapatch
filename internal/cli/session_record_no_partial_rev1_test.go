package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// v0.12.0 Wave γ rev-1 Slice R4 (F-EXT-γ-4 HIGH). Rev-0 applied the
// `--from-session requires --with-session` mutex check late in `record`
// RunE, AFTER the patch had been captured, written to
// `.tpatch/features/<slug>/patches/001-record.patch`, and
// `artifacts/post-apply.patch`. Rev-1 hoists the check to the top of
// RunE — validate-before-mutate — so a bad invocation refuses cleanly
// with zero on-disk side effects.

// TestRecordFromSessionRefusalLeavesNoArtifacts stages a change so
// record would otherwise write patches, invokes
// `record --from-session cs_...` WITHOUT `--with-session`, and asserts:
//
//  1. The refusal fires (exit non-zero).
//  2. The refusal message cites PRD §8.8.
//  3. Neither `patches/*.patch` nor `artifacts/post-apply.patch`
//     exists under the feature's artifact directory.
//
// Precondition: setupSessionRepo initializes the feature but has NOT
// yet recorded any patches. If the fix ever regresses, the patch files
// will appear on disk and this test will fail loudly.
func TestRecordFromSessionRefusalLeavesNoArtifacts(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "R4 from-session refusal no artifacts")

	// Stage a real change so record would otherwise capture something
	// non-empty. If the mutex validation ran late, patches/001-record.patch
	// would already be on disk before the refusal fires.
	writeStagedFileForRecord(t, tmp, "README.txt", "wave-γ R4 fixture\n")

	// Provide a dummy cs_ id — the refusal must fire on the mutex
	// alone, before any session lookup, and before any capture.
	_, errMsg, code := runSessionCmd(
		"record", "--path", tmp, slug,
		"--from-session", "cs_deadbeef0011",
	)
	if code == 0 {
		t.Fatalf("expected refusal for --from-session without --with-session; got success")
	}
	if !strings.Contains(errMsg, "requires --with-session") {
		t.Fatalf("expected 'requires --with-session' in stderr; got %q", errMsg)
	}
	if !strings.Contains(errMsg, "§8.8") {
		t.Fatalf("expected PRD §8.8 citation in stderr; got %q", errMsg)
	}

	// Assert NO artifacts exist on disk. Patch files land under
	// .tpatch/features/<slug>/patches/ ; post-apply.patch lands
	// under .tpatch/features/<slug>/artifacts/.
	featureDir := filepath.Join(tmp, ".tpatch", "features", slug)
	assertNoPatchFile(t, featureDir, "patches", ".patch")
	assertNoNamedFile(t, featureDir, "artifacts", "post-apply.patch")
}

func assertNoPatchFile(t *testing.T, featureDir, subdir, suffix string) {
	t.Helper()
	root := filepath.Join(featureDir, subdir)
	entries, err := os.ReadDir(root)
	if err != nil {
		// Missing subdir is stronger evidence of no writes — success.
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read %s: %v", root, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			t.Fatalf("refusal left partial artifact %s/%s on disk (F-EXT-γ-4 regression)", root, e.Name())
		}
	}
}

func assertNoNamedFile(t *testing.T, featureDir, subdir, name string) {
	t.Helper()
	full := filepath.Join(featureDir, subdir, name)
	if _, err := os.Stat(full); err == nil {
		t.Fatalf("refusal left %s on disk (F-EXT-γ-4 regression)", full)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", full, err)
	}
}
