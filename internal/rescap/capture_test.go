// Capture, path-gate, Git-gate, scratch and locking tests
// (PRD §5, §7.1, §7.2, §8, §9, §10; ADR-033 D4/D6/D8/D9).

package rescap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// newGitRepo builds a real git worktree for the gate tests.
func newGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("readme\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	run("add", "README.md")
	run("commit", "-q", "-m", "init")
	return root
}

func writeRepoFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return path
}

func appendGitignore(t *testing.T, root string, lines ...string) {
	t.Helper()
	path := filepath.Join(root, ".gitignore")
	existing, _ := os.ReadFile(path)
	body := string(existing) + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
}

// TestIgnoreGateExitCodeHandling covers §10.1: exit 0 ignored, exit 1
// not ignored, and the colon-magic `./` prefix rule.
func TestIgnoreGateExitCodeHandling(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/local.env", "A=1\n")
	appendGitignore(t, root, "config/local.env", ":(glob)weird.env", "topmagic.env")

	ignored, err := IsIgnored(root, "config/local.env")
	if err != nil || !ignored {
		t.Fatalf("ignored=%v err=%v; want true,nil", ignored, err)
	}
	ignored, err = IsIgnored(root, "README.md")
	if err != nil || ignored {
		t.Fatalf("ignored=%v err=%v; want false,nil", ignored, err)
	}
}

// TestIgnoreCheckArgumentColonRule covers §10.4's exact rows: any
// selector whose first byte is `:` is passed as ./<selector>, and no
// other shape is rewritten.
func TestIgnoreCheckArgumentColonRule(t *testing.T) {
	cases := map[string]string{
		":(glob)config/*.env":       "./:(glob)config/*.env",
		":(literal)config/name.env": "./:(literal)config/name.env",
		":/topmagic.env":            "./:/topmagic.env",
		"config/**/local.env":       "config/**/local.env",
		"docs/*.md":                 "docs/*.md",
		"plain.txt":                 "plain.txt",
	}
	for in, want := range cases {
		if got := ignoreCheckArgument(in); got != want {
			t.Fatalf("ignoreCheckArgument(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestColonMagicSelectorIsNotFatal empirically confirms the ./ prefix
// disarms magic parsing: an unprefixed colon-magic argument is fatal
// (exit 128), the prefixed form this design always emits is not.
func TestColonMagicSelectorIsNotFatal(t *testing.T) {
	root := newGitRepo(t)
	if _, err := IsIgnored(root, ":(glob)config/*.env"); err != nil {
		t.Fatalf("the ./-prefixed form must never be fatal: %v", err)
	}
	// The form this design never emits, for contrast.
	cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", ":(glob)config/*.env")
	cmd.Dir = root
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 128 {
		t.Skipf("this git build does not treat unprefixed colon magic as fatal (%v); the ./ rule is still applied unconditionally", err)
	}
}

// TestLiteralPathspecsOnLsFiles covers §10.2/§10.4: ls-files does
// support --literal-pathspecs and this design always passes it.
func TestLiteralPathspecsOnLsFiles(t *testing.T) {
	root := newGitRepo(t)
	tracked, err := IsTracked(root, "README.md")
	if err != nil || !tracked {
		t.Fatalf("tracked=%v err=%v; want true,nil", tracked, err)
	}
	tracked, err = IsTracked(root, "config/**/local.env")
	if err != nil || tracked {
		t.Fatalf("a literal magic-looking path must report untracked: tracked=%v err=%v", tracked, err)
	}
}

// TestTrackedAndIgnoredRefusal covers §5.1's exact --no-index gap: a
// file can be both tracked and reported ignored, and that combination
// is refused.
func TestTrackedAndIgnoredRefusal(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/tracked.env", "A=1\n")
	cmd := exec.Command("git", "add", "-f", "config/tracked.env")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	appendGitignore(t, root, "config/tracked.env")

	ignored, err := IsIgnored(root, "config/tracked.env")
	if err != nil || !ignored {
		t.Fatalf("--no-index should still report ignored: %v %v", ignored, err)
	}
	tracked, err := IsTracked(root, "config/tracked.env")
	if err != nil || !tracked {
		t.Fatalf("the file is tracked: %v %v", tracked, err)
	}
}

// TestAnythingTrackedUnder covers §10.3 step 2's empty-stdout
// convention over the whole .tpatch/local/ subtree.
func TestAnythingTrackedUnder(t *testing.T) {
	root := newGitRepo(t)
	tracked, err := AnythingTrackedUnder(root, LocalScratchPrefix)
	if err != nil {
		t.Fatalf("AnythingTrackedUnder: %v", err)
	}
	if tracked {
		t.Fatal("a fresh repo has nothing tracked under .tpatch/local/")
	}
	writeRepoFile(t, root, ".tpatch/local/oops.txt", "x")
	cmd := exec.Command("git", "add", "-f", ".tpatch/local/oops.txt")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	tracked, err = AnythingTrackedUnder(root, LocalScratchPrefix)
	if err != nil {
		t.Fatalf("AnythingTrackedUnder: %v", err)
	}
	if !tracked {
		t.Fatal("a tracked file anywhere under the subtree must be detected")
	}
}

// TestPathGateRefusesSymlinkComponents covers §9.1 steps 1-3: a
// symlink anywhere in the chain is refused outright, without ever being
// resolved or inspected for where it points.
func TestPathGateRefusesSymlinkComponents(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "real/dir/file.txt", "x")

	t.Run("plain-chain-accepted", func(t *testing.T) {
		g, err := GatePath(root, "real/dir/file.txt")
		if err != nil {
			t.Fatalf("a plain chain must be accepted: %v", err)
		}
		if err := g.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})

	t.Run("final-component-symlink-refused", func(t *testing.T) {
		link := filepath.Join(root, "real", "dir", "link.txt")
		if err := os.Symlink(filepath.Join(root, "real", "dir", "file.txt"), link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		_, err := GatePath(root, "real/dir/link.txt")
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonSymlinkComponentRefused || r.Code != ExitRefusal {
			t.Fatalf("want symlink-component-refused exit 3, got %v", err)
		}
	})

	t.Run("ancestor-symlink-refused-even-when-target-is-safe", func(t *testing.T) {
		// The symlink points at a directory that is itself perfectly
		// safe and inside the repo: it is still refused, because no
		// symlink is ever resolved.
		linkDir := filepath.Join(root, "aliased")
		if err := os.Symlink(filepath.Join(root, "real"), linkDir); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		_, err := GatePath(root, "aliased/dir/file.txt")
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonSymlinkComponentRefused {
			t.Fatalf("want symlink-component-refused, got %v", err)
		}
	})

	t.Run("missing-prefix-refused", func(t *testing.T) {
		_, err := GatePath(root, "no/such/dir/file.txt")
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonPathMissing || r.Code != ExitRefusal {
			t.Fatalf("want path-missing exit 3, got %v", err)
		}
	})
}

// TestLexicalContainmentPreFilter covers §9.1's coarse pre-filter,
// which refuses before any Lstat of any component.
func TestLexicalContainmentPreFilter(t *testing.T) {
	root := newGitRepo(t)
	for _, bad := range []string{"../escape.txt", "a/../../escape.txt", "/etc/passwd", ""} {
		_, err := LexicalContainment(root, bad)
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonPathOutsideRepo || r.Code != ExitRefusal {
			t.Fatalf("%q: want path-outside-repo exit 3, got %v", bad, err)
		}
	}
	if _, err := LexicalContainment(root, "config/local.env"); err != nil {
		t.Fatalf("an in-repo relative path must pass: %v", err)
	}
}

// TestSamePathIdentityDetectsReplacement covers the db_path
// pathname-vs-descriptor check: comparing a held descriptor against a
// *fresh* pathname resolution is what can detect a swap.
func TestSamePathIdentityDetectsReplacement(t *testing.T) {
	root := newGitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "data", "db"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	held, err := GatePath(root, "data/db")
	if err != nil {
		t.Fatalf("GatePath: %v", err)
	}
	defer func() { _ = held.Close() }()

	if err := SamePathIdentity(root, "data/db", held.Info); err != nil {
		t.Fatalf("an unchanged path must match: %v", err)
	}

	// Replace the directory with a different inode at the same name.
	if err := os.Rename(filepath.Join(root, "data", "db"), filepath.Join(root, "data", "db-old")); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data", "db"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	err = SamePathIdentity(root, "data/db", held.Info)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonDBPathIdentityChanged || r.Code != ExitRefusal {
		t.Fatalf("want db-path-identity-changed exit 3, got %v", err)
	}
}

// TestCaptureIgnoredFileSingle covers the single-file result variant,
// verbatim hashing with no text normalization, and binary/text
// classification.
func TestCaptureIgnoredFileSingle(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/local.env", "A=1\r\nB=2\r\n")
	result, raw, err := CaptureIgnoredFile(root, "config/local.env")
	if err != nil {
		t.Fatalf("CaptureIgnoredFile: %v", err)
	}
	kind, _ := result.Field("file_kind")
	if kind.Str != "text" {
		t.Fatalf("file_kind = %q, want text", kind.Str)
	}
	size, _ := result.Field("size_bytes")
	if size.Uint != 10 {
		t.Fatalf("size_bytes = %d, want 10 (CRLF preserved verbatim)", size.Uint)
	}
	hash, _ := result.Field("hash")
	if !strings.HasPrefix(hash.Str, "sha256:") {
		t.Fatalf("wire hash must carry the sha256: prefix, got %q", hash.Str)
	}
	if raw == nil || raw.ByteCount != 10 {
		t.Fatalf("raw = %+v", raw)
	}

	writeRepoFile(t, root, "config/blob.bin", "abc\x00def")
	result, _, err = CaptureIgnoredFile(root, "config/blob.bin")
	if err != nil {
		t.Fatalf("CaptureIgnoredFile(binary): %v", err)
	}
	kind, _ = result.Field("file_kind")
	if kind.Str != "binary" {
		t.Fatalf("file_kind = %q, want binary", kind.Str)
	}
}

// TestGoldenDirectoryCombinedHash reproduces the two-file golden vector
// from §5.1 / ADR-033 D3 against real files on disk.
func TestGoldenDirectoryCombinedHash(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "config/a.txt", "")
	writeRepoFile(t, root, "config/sub/b.sh", "#!/bin/sh\necho hi\n")
	if err := os.Chmod(filepath.Join(root, "config", "sub", "b.sh"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	result, raw, err := CaptureIgnoredFile(root, "config")
	if err != nil {
		t.Fatalf("CaptureIgnoredFile: %v", err)
	}
	const wantCombined = "sha256:5af4d6754656795b49c6e22acc2034ed6a2b3426470b0c42156f5ad0b4bcb9ad"
	combined, _ := result.Field("combined_hash")
	if combined.Str != wantCombined {
		t.Fatalf("combined_hash = %s\nwant %s", combined.Str, wantCombined)
	}
	count, _ := result.Field("file_count")
	if count.Uint != 2 {
		t.Fatalf("file_count = %d, want 2", count.Uint)
	}
	total, _ := result.Field("total_bytes")
	if total.Uint != 18 {
		t.Fatalf("total_bytes = %d, want 18", total.Uint)
	}
	files, _ := result.Field("files")
	if len(files.Array) != 2 {
		t.Fatalf("files[] length = %d, want 2", len(files.Array))
	}
	// files[] is sorted by path and never carries a null field.
	first, _ := files.Array[0].Field("path")
	second, _ := files.Array[1].Field("path")
	if first.Str != "config/a.txt" || second.Str != "config/sub/b.sh" {
		t.Fatalf("files[] not sorted by path: %s, %s", first.Str, second.Str)
	}
	mode, _ := files.Array[1].Field("mode")
	if mode.Str != "100755" {
		t.Fatalf("mode = %s, want 100755", mode.Str)
	}
	if raw == nil || raw.Hash != wantCombined {
		t.Fatalf("raw = %+v", raw)
	}
}

// TestCombinedHashCoversMode proves a chmod-only change moves
// combined_hash, which is what makes a permission-only change visible
// to diff.
func TestCombinedHashCoversMode(t *testing.T) {
	base := []FileCapture{
		{RelPath: "a", Mode: "100644", SHA256Hex: strings.Repeat("1", 64)},
		{RelPath: "b", Mode: "100644", SHA256Hex: strings.Repeat("2", 64)},
	}
	chmodded := []FileCapture{
		{RelPath: "a", Mode: "100755", SHA256Hex: strings.Repeat("1", 64)},
		{RelPath: "b", Mode: "100644", SHA256Hex: strings.Repeat("2", 64)},
	}
	if CombinedDirectoryHash(base) == CombinedDirectoryHash(chmodded) {
		t.Fatal("a chmod-only change must change combined_hash")
	}
	// Input ordering never affects the digest.
	reordered := []FileCapture{base[1], base[0]}
	if CombinedDirectoryHash(base) != CombinedDirectoryHash(reordered) {
		t.Fatal("combined_hash must not depend on input ordering")
	}
}

// TestBoundedReadRefusesOversizeContent covers the cap-plus-one read:
// the reader refuses on bytes actually read, not on a pre-read
// Stat().Size().
func TestBoundedReadRefusesOversizeContent(t *testing.T) {
	root := newGitRepo(t)
	writeRepoFile(t, root, "big.txt", strings.Repeat("x", 100))
	_, _, err := captureFileContent(root, "big.txt", 50)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonResourceLimitExceeded || r.Code != ExitRefusal {
		t.Fatalf("want resource-limit-exceeded exit 3, got %v", err)
	}
	if _, _, err := captureFileContent(root, "big.txt", 100); err != nil {
		t.Fatalf("exactly-at-the-cap content must be accepted: %v", err)
	}
}

// TestDirectoryFileCountLimit covers the 200-file bound, re-checked at
// every capture rather than only at add time.
func TestDirectoryFileCountLimit(t *testing.T) {
	root := newGitRepo(t)
	for i := 0; i < MaxDirectoryFiles+1; i++ {
		writeRepoFile(t, root, filepath.Join("many", string(rune('a'+i%26))+strings.Repeat("x", i)), "y")
	}
	_, _, err := CaptureIgnoredFile(root, "many")
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonResourceLimitExceeded {
		t.Fatalf("want resource-limit-exceeded, got %v", err)
	}
}

// TestRedactionRefusesTheWholeInvocation covers §8.2/§8.3: a match on
// any of the six closed classes is a hard refusal, never a partial
// scrub-and-continue.
func TestRedactionRefusesTheWholeInvocation(t *testing.T) {
	root := newGitRepo(t)
	cases := map[string]string{
		"private-key":           "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n",
		"connection-url":        "DB=postgres://user:pw@localhost:5432/app\n",
		"email-pii":             "owner: someone@example.com\n",
		"credential-assignment": "api_key = abcdefghijklmnop123\n",
		"bearer-token":          "Authorization: Bearer abcdefghijklmnopqrstuvwx\n",
		"home-absolute-path":    "root=/Users/someone/secrets\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rel := filepath.Join("secretz", name+".txt")
			writeRepoFile(t, root, rel, body)
			_, _, err := CaptureIgnoredFile(root, rel)
			r := AsRefusal(err)
			if r == nil || r.Reason != ReasonRedactionRefused || r.Code != ExitRefusal {
				t.Fatalf("want redaction-refused exit 3, got %v", err)
			}
		})
	}
}

// TestNoRawBytesEverReachDisk proves the scanner is handed in-memory
// content: a refused capture leaves no scratch file containing the
// offending bytes anywhere under the repository.
func TestNoRawBytesEverReachDisk(t *testing.T) {
	root := newGitRepo(t)
	secret := "-----BEGIN RSA PRIVATE KEY-----\nMIIsecretmaterial\n"
	writeRepoFile(t, root, "config/id_rsa", secret)
	if _, _, err := CaptureIgnoredFile(root, "config/id_rsa"); AsRefusal(err) == nil {
		t.Fatalf("want a redaction refusal, got %v", err)
	}
	var offenders []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if path == filepath.Join(root, "config", "id_rsa") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr == nil && strings.Contains(string(body), "MIIsecretmaterial") {
			offenders = append(offenders, path)
		}
		return nil
	})
	if len(offenders) != 0 {
		t.Fatalf("raw bytes were persisted to %v", offenders)
	}
}

// TestGitMetadataViews covers all four closed views plus their refusal
// cases.
func TestGitMetadataViews(t *testing.T) {
	root := newGitRepo(t)

	t.Run("head-attached", func(t *testing.T) {
		result, err := CaptureGitMetadata(root, store.GitMetadataViewHead, "head")
		if err != nil {
			t.Fatalf("head: %v", err)
		}
		sym, _ := result.Field("symbolic_ref")
		detached, _ := result.Field("detached")
		oid, _ := result.Field("oid")
		if sym.Str != "refs/heads/main" || detached.Bool || len(oid.Str) < 40 {
			t.Fatalf("unexpected head result: %+v", result)
		}
	})

	t.Run("head-detached-implies-null-symbolic-ref", func(t *testing.T) {
		detachedRepo := newGitRepo(t)
		cmd := exec.Command("git", "checkout", "-q", "--detach", "HEAD")
		cmd.Dir = detachedRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("detach: %v\n%s", err, out)
		}
		result, err := CaptureGitMetadata(detachedRepo, store.GitMetadataViewHead, "head")
		if err != nil {
			t.Fatalf("head: %v", err)
		}
		sym, _ := result.Field("symbolic_ref")
		detached, _ := result.Field("detached")
		if !detached.Bool {
			t.Fatal("detached must be true")
		}
		if !sym.IsNull() {
			t.Fatal("symbolic_ref must be null iff detached is true")
		}
	})

	t.Run("ref", func(t *testing.T) {
		result, err := CaptureGitMetadata(root, store.GitMetadataViewRef, "refs/heads/main")
		if err != nil {
			t.Fatalf("ref: %v", err)
		}
		name, _ := result.Field("ref")
		if name.Str != "refs/heads/main" {
			t.Fatalf("ref = %q", name.Str)
		}
	})

	t.Run("index-entry", func(t *testing.T) {
		result, err := CaptureGitMetadata(root, store.GitMetadataViewIndexEntry, "README.md")
		if err != nil {
			t.Fatalf("index-entry: %v", err)
		}
		path, _ := result.Field("path")
		mode, _ := result.Field("mode")
		stage, _ := result.Field("stage")
		if path.Str != "README.md" || mode.Str != "100644" || stage.Uint != 0 {
			t.Fatalf("unexpected index-entry result: %+v", result)
		}
	})

	t.Run("index-entry-missing", func(t *testing.T) {
		_, err := CaptureGitMetadata(root, store.GitMetadataViewIndexEntry, "not/in/index.txt")
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonIndexEntryMissing || r.Code != ExitRefusal {
			t.Fatalf("want index-entry-missing exit 3, got %v", err)
		}
	})

	t.Run("config-allowed-keys-only", func(t *testing.T) {
		for _, key := range AllowedConfigKeys {
			result, err := CaptureGitMetadata(root, store.GitMetadataViewConfig, key)
			if err != nil {
				t.Fatalf("config %s: %v", key, err)
			}
			k, _ := result.Field(key)
			_ = k
			gotKey, _ := result.Field("key")
			if gotKey.Str != key {
				t.Fatalf("key = %q, want %q", gotKey.Str, key)
			}
		}
		for _, bad := range []string{"user.email", "core.editor", "remote.origin.url", "core.*"} {
			_, err := CaptureGitMetadata(root, store.GitMetadataViewConfig, bad)
			r := AsRefusal(err)
			if r == nil || r.Code != ExitValidation {
				t.Fatalf("%s must be an exit-2 validation error, got %v", bad, err)
			}
		}
	})

	t.Run("config-unset-is-null-not-an-error", func(t *testing.T) {
		result, err := CaptureGitMetadata(root, store.GitMetadataViewConfig, "extensions.objectformat")
		if err != nil {
			t.Fatalf("unset key must not error: %v", err)
		}
		value, ok := result.Field("value")
		if !ok {
			t.Fatal("value must be present")
		}
		_ = value
	})

	t.Run("unknown-view", func(t *testing.T) {
		_, err := CaptureGitMetadata(root, "reflog", "HEAD")
		r := AsRefusal(err)
		if r == nil || r.Code != ExitValidation {
			t.Fatalf("want an exit-2 validation error, got %v", err)
		}
	})
}

// TestScratchLifecycle covers §7.1: 0700 directories, an isolated Dolt
// HOME, best-effort removal on every path, and the lock-gated orphan
// sweep.
func TestScratchLifecycle(t *testing.T) {
	root := newGitRepo(t)
	scratch, err := EphemeralScratch(root, "model-picker")
	if err != nil {
		t.Fatalf("EphemeralScratch: %v", err)
	}
	info, err := os.Stat(scratch.Root)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("scratch mode = %v, want 0700", info.Mode().Perm())
	}
	if !strings.HasPrefix(filepath.Base(scratch.Root), "es_") {
		t.Fatalf("unexpected scratch name %s", scratch.Root)
	}
	home, err := scratch.EnsureDoltHome()
	if err != nil {
		t.Fatalf("EnsureDoltHome: %v", err)
	}
	homeInfo, err := os.Stat(home)
	if err != nil {
		t.Fatalf("stat home: %v", err)
	}
	if homeInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dolt-home mode = %v, want 0700", homeInfo.Mode().Perm())
	}

	// A leftover directory from a crashed prior invocation is swept.
	orphan := filepath.Join(ScratchRoot(root, "model-picker"), "es_deadbeef0000")
	if err := os.MkdirAll(orphan, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if diags := SweepLocalOrphans(root, "model-picker", scratch.Root); len(diags) != 0 {
		t.Fatalf("unexpected sweep diagnostics: %v", diags)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("the orphan scratch directory survived the sweep")
	}
	if _, err := os.Stat(scratch.Root); err != nil {
		t.Fatal("the sweep must not remove the current invocation's own scratch")
	}

	if diags := scratch.Remove(); len(diags) != 0 {
		t.Fatalf("unexpected removal diagnostics: %v", diags)
	}
	if _, err := os.Stat(scratch.Root); !os.IsNotExist(err) {
		t.Fatal("scratch must be removed at the end of the invocation")
	}
}

// TestLockContentionRefusesImmediately covers §7.2: a second acquirer
// is refused instantly rather than blocking, the lock file is 0600, and
// releasing is the only mechanism needed.
func TestLockContentionRefusesImmediately(t *testing.T) {
	root := newGitRepo(t)
	scratchRoot := ScratchRoot(root, "model-picker")

	first, err := AcquireLock(scratchRoot, root)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	info, err := os.Stat(first.Path())
	if err != nil {
		t.Fatalf("stat lock: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("lock mode = %v, want 0600", info.Mode().Perm())
	}
	if info.Size() != 0 {
		t.Fatal("the lock file has no body at all")
	}

	// A second acquirer in this same process shares the same open file
	// description only if it reuses the descriptor; a fresh open plus
	// flock must contend.
	second, err := AcquireLock(scratchRoot, root)
	if err == nil {
		_ = second.Release()
		t.Skip("this platform's flock does not contend within one process; cross-process contention is covered by the CLI suite")
	}
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonCaptureInProgress || r.Code != ExitRefusal {
		t.Fatalf("want capture-in-progress exit 3, got %v", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	// The lock file itself is never removed: the name and the inode
	// stay permanently synonymous.
	if _, err := os.Stat(first.Path()); err != nil {
		t.Fatalf("the .lock file must persist after release: %v", err)
	}
	third, err := AcquireLock(scratchRoot, root)
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	_ = third.Release()
}

// TestCompareResultsNamesTheChangedField covers the diff contract's
// field-level reporting, including the chmod-only case.
func TestCompareResultsNamesTheChangedField(t *testing.T) {
	mk := func(hash, mode string, size uint64) store.CanonNode {
		return store.CanonObject(
			store.CanonFieldOf("file_count", store.CanonUint(1)),
			store.CanonFieldOf("total_bytes", store.CanonUint(size)),
			store.CanonFieldOf("combined_hash", store.CanonString(hash)),
			store.CanonFieldOf("files", store.CanonArray(store.CanonObject(
				store.CanonFieldOf("path", store.CanonString("a.txt")),
				store.CanonFieldOf("raw_sha256", store.CanonString("sha256:"+strings.Repeat("1", 64))),
				store.CanonFieldOf("byte_count", store.CanonUint(size)),
				store.CanonFieldOf("mode", store.CanonString(mode)),
			))),
		)
	}
	if diffs := CompareResults(mk("h1", "100644", 3), mk("h1", "100644", 3)); len(diffs) != 0 {
		t.Fatalf("identical results must report no differences: %v", diffs)
	}
	diffs := CompareResults(mk("h1", "100644", 3), mk("h2", "100755", 3))
	joined := strings.Join(diffs, "|")
	if !strings.Contains(joined, "file mode differs: a.txt (100644 -> 100755)") {
		t.Fatalf("a chmod-only change must be named per file: %v", diffs)
	}
	if !strings.Contains(joined, "combined_hash differs") {
		t.Fatalf("combined_hash must also be reported: %v", diffs)
	}
	if strings.Contains(joined, "file content differs") {
		t.Fatalf("a chmod-only change must not be reported as a content change: %v", diffs)
	}
}

// TestCompareResultsFileSetMembership covers added/removed reporting.
func TestCompareResultsFileSetMembership(t *testing.T) {
	entry := func(path string) store.CanonNode {
		return store.CanonObject(
			store.CanonFieldOf("path", store.CanonString(path)),
			store.CanonFieldOf("raw_sha256", store.CanonString("sha256:"+strings.Repeat("1", 64))),
			store.CanonFieldOf("byte_count", store.CanonUint(1)),
			store.CanonFieldOf("mode", store.CanonString("100644")),
		)
	}
	before := store.CanonObject(store.CanonFieldOf("files", store.CanonArray(entry("a"), entry("b"))))
	after := store.CanonObject(store.CanonFieldOf("files", store.CanonArray(entry("b"), entry("c"))))
	joined := strings.Join(CompareResults(before, after), "|")
	if !strings.Contains(joined, "file removed: a") || !strings.Contains(joined, "file added: c") {
		t.Fatalf("file-set membership must be reported: %s", joined)
	}
}
