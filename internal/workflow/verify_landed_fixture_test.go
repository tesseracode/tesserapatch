package workflow

// Shared fixtures for the v0.15.1 Wave C / GH #8 landed-verification
// acceptance matrix (PRD-verify-freshness §7.1).
//
// Tier W rows use a real temp Git repo plus a real store.Store and call
// RunVerify directly. Where a row must observe, count or perturb Git
// behaviour it uses a `PATH` git wrapper — a test-only shim first on
// PATH that forwards to the real git and records argv and environment.
// There is NO production seam, no build tag and no exported hook.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ── Git helpers ──────────────────────────────────────────────────────────

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func tryGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func gitHeadOf(t *testing.T, dir string) string {
	t.Helper()
	return mustGit(t, dir, "rev-parse", "HEAD")
}

// ── Landed-feature fixture ───────────────────────────────────────────────

// landedFixture is a repo with one feature whose canonical artifacts are
// recorded and which has (optionally) been landed with a real
// four-trailer commit.
type landedFixture struct {
	t     *testing.T
	Store *store.Store
	Root  string
	Slug  string
	// BaseCommit is the commit the feature was recorded against — the
	// value that goes into `Tpatch-Base-Commit`.
	BaseCommit string
	// LandingCommit is the most recent landing commit, when landed.
	LandingCommit string
	// FilePath is the repo-relative path the feature patches.
	FilePath string
}

const fixtureSeedBody = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n"

// newLandedFixture builds a repo whose single feature patches one file,
// with the canonical `post-apply.patch` and `apply-recipe.json` recorded.
// The feature is NOT landed yet — call Land().
func newLandedFixture(t *testing.T, slug string) *landedFixture {
	t.Helper()
	root := t.TempDir()
	gitInitVerifyTest(t, root)

	f := &landedFixture{t: t, Root: root, Slug: slug, FilePath: "src/app.txt"}
	mustWriteFile(t, filepath.Join(root, f.FilePath), fixtureSeedBody)
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "seed")

	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	f.Store = s
	if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: "x"}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if err := s.MarkFeatureState(slug, store.StateApplied, "apply", ""); err != nil {
		t.Fatalf("MarkFeatureState: %v", err)
	}
	writeIntentFiles(t, s, slug)

	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "tpatch scaffolding")
	f.BaseCommit = gitHeadOf(t, root)
	return f
}

// Implement edits the feature's file in the working tree and commits the
// result, then records the canonical patch from BaseCommit..HEAD.
func (f *landedFixture) Implement(newBody string) {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), newBody)
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "implement "+f.Slug)
	f.Record()
}

// Record captures the canonical patch for BaseCommit..HEAD over the
// feature's file and stores it plus a matching recipe.
func (f *landedFixture) Record() {
	f.t.Helper()
	patch, err := gitutil.CapturePatchFromCommitsScoped(f.Root, f.BaseCommit, "HEAD", []string{f.FilePath})
	if err != nil {
		f.t.Fatalf("CapturePatchFromCommitsScoped: %v", err)
	}
	if strings.TrimSpace(patch) == "" {
		f.t.Fatalf("captured an empty canonical patch for %s", f.Slug)
	}
	f.WritePatch(patch)

	body, err := os.ReadFile(filepath.Join(f.Root, f.FilePath))
	if err != nil {
		f.t.Fatalf("read feature file: %v", err)
	}
	f.WriteRecipe(ApplyRecipe{
		Feature:    f.Slug,
		Operations: []RecipeOperation{{Type: "write-file", Path: f.FilePath, Content: string(body)}},
	})
	st, err := f.Store.LoadFeatureStatus(f.Slug)
	if err != nil {
		f.t.Fatalf("load status: %v", err)
	}
	st.Apply.BaseCommit = f.BaseCommit
	st.Apply.HasPatch = true
	if err := f.Store.SaveFeatureStatus(st); err != nil {
		f.t.Fatalf("save status: %v", err)
	}
}

func (f *landedFixture) WritePatch(patch string) {
	f.t.Helper()
	if err := f.Store.WriteArtifact(f.Slug, "post-apply.patch", patch); err != nil {
		f.t.Fatalf("write patch: %v", err)
	}
}

func (f *landedFixture) WriteRecipe(r ApplyRecipe) {
	f.t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		f.t.Fatalf("marshal recipe: %v", err)
	}
	if err := f.Store.WriteArtifact(f.Slug, "apply-recipe.json", string(data)); err != nil {
		f.t.Fatalf("write recipe: %v", err)
	}
}

func (f *landedFixture) PatchBytes() []byte {
	f.t.Helper()
	data, err := os.ReadFile(artifactPath(f.Root, f.Slug, "post-apply.patch"))
	if err != nil {
		f.t.Fatalf("read patch: %v", err)
	}
	return data
}

func (f *landedFixture) RecipeBytes() []byte {
	f.t.Helper()
	data, err := os.ReadFile(artifactPath(f.Root, f.Slug, "apply-recipe.json"))
	if err != nil {
		f.t.Fatalf("read recipe: %v", err)
	}
	return data
}

// TrailerBlock renders the exact four-trailer block `land` emits for the
// CURRENT artifact bytes.
func (f *landedFixture) TrailerBlock(slug string) string {
	f.t.Helper()
	return trailerBlockFor(f.t, f.Store, f.Root, slug)
}

func trailerBlockFor(t *testing.T, s *store.Store, root, slug string) string {
	t.Helper()
	patch, err := os.ReadFile(artifactPath(root, slug, "post-apply.patch"))
	if err != nil {
		patch = nil
	}
	recipeSHA := "none"
	if raw, rerr := os.ReadFile(artifactPath(root, slug, "apply-recipe.json")); rerr == nil && strings.TrimSpace(string(raw)) != "" {
		recipeSHA = sha256Hex(raw)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("load status for trailer block: %v", err)
	}
	return fmt.Sprintf("Tpatch-Feature: %s\nTpatch-Patch-SHA: %s\nTpatch-Recipe-SHA: %s\nTpatch-Base-Commit: %s\n",
		slug, sha256Hex(patch), recipeSHA, st.Apply.BaseCommit)
}

// Land creates a real landing commit carrying the four-trailer block as
// the LAST paragraph of the message, exactly like `tpatch land`.
func (f *landedFixture) Land() string {
	f.t.Helper()
	return f.LandWithBlock(f.TrailerBlock(f.Slug))
}

// LandWithBlock creates a landing commit with an arbitrary trailer block
// so adversarial grammar rows can be constructed.
func (f *landedFixture) LandWithBlock(block string) string {
	f.t.Helper()
	// The landing must contain a real change, otherwise `git commit`
	// refuses. Landing an already-committed feature is modelled by
	// committing the .tpatch artifacts, which is what `land` does when
	// the user-code change is already in history.
	mustWriteFile(f.t, filepath.Join(f.Root, ".tpatch", "features", f.Slug, "landing-marker.txt"),
		fmt.Sprintf("landed %d\n", landingCounter))
	landingCounter++
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "feat: land "+f.Slug, "-m", block)
	f.LandingCommit = gitHeadOf(f.t, f.Root)
	return f.LandingCommit
}

var landingCounter int

// LandUserChange commits the feature's user-code change TOGETHER with the
// trailer block, which is the shape `tpatch land` produces when the
// feature was implemented in the working tree.
func (f *landedFixture) LandUserChange(newBody string) string {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), newBody)
	patch, err := gitutil.CapturePatchScoped(f.Root, []string{f.FilePath})
	if err != nil {
		f.t.Fatalf("CapturePatchScoped: %v", err)
	}
	f.WritePatch(patch)
	f.WriteRecipe(ApplyRecipe{
		Feature:    f.Slug,
		Operations: []RecipeOperation{{Type: "write-file", Path: f.FilePath, Content: newBody}},
	})
	st, _ := f.Store.LoadFeatureStatus(f.Slug)
	st.Apply.BaseCommit = f.BaseCommit
	st.Apply.HasPatch = true
	if err := f.Store.SaveFeatureStatus(st); err != nil {
		f.t.Fatalf("save status: %v", err)
	}
	block := f.TrailerBlock(f.Slug)
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "feat: land "+f.Slug, "-m", block)
	f.LandingCommit = gitHeadOf(f.t, f.Root)
	return f.LandingCommit
}

// Verify runs the real pipeline read-only.
func (f *landedFixture) Verify() *VerifyReport {
	f.t.Helper()
	report, err := RunVerify(f.Store, f.Slug, VerifyOptions{NoWrite: true})
	if report == nil {
		f.t.Fatalf("RunVerify returned no report: %v", err)
	}
	return report
}

// EditTracked rewrites the feature file at HEAD with an extra commit —
// used by the ladder rows.
func (f *landedFixture) EditTracked(body, msg string) string {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), body)
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", msg)
	return gitHeadOf(f.t, f.Root)
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ── Report helpers ───────────────────────────────────────────────────────

func checkByID(t *testing.T, r *VerifyReport, id string) store.VerifyCheckResult {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("check %q not present in report", id)
	return store.VerifyCheckResult{}
}

func hasAdvisory(r *VerifyReport, code string) bool {
	for _, a := range r.Advisories {
		if a.Code == code {
			return true
		}
	}
	return false
}

func advisoryByCode(r *VerifyReport, code string) (VerifyAdvisory, bool) {
	for _, a := range r.Advisories {
		if a.Code == code {
			return a, true
		}
	}
	return VerifyAdvisory{}, false
}

// ── PATH git wrapper ─────────────────────────────────────────────────────

// gitWrapper installs a shim named `git` first on PATH that forwards to
// the real git while appending one record per invocation to a log file.
// Each record is argv fields separated by the 0x1f unit separator, followed
// by ENV fields. The shell emits it as \037 because POSIX printf does not
// require support for \xHH escapes (Ubuntu /bin/sh does not support them).
type gitWrapper struct {
	t       *testing.T
	dir     string
	logPath string
	oldPath string
}

func installGitWrapper(t *testing.T) *gitWrapper {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := fmt.Sprintf(`#!/bin/sh
{
  printf '%%s' "ARGS"
  for a in "$@"; do printf '\037%%s' "$a"; done
  printf '\037ENV\037GIT_NO_LAZY_FETCH=%%s\037LC_ALL=%%s\037GIT_INDEX_FILE=%%s\n' "${GIT_NO_LAZY_FETCH-}" "${LC_ALL-}" "${GIT_INDEX_FILE-}"
} >> %q
exec %q "$@"
`, logPath, realGit)
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write git shim: %v", err)
	}
	w := &gitWrapper{t: t, dir: dir, logPath: logPath, oldPath: os.Getenv("PATH")}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+w.oldPath)
	return w
}

// installFakeVersionGit installs a shim that answers `--version` with the
// given string and FAILS every other subcommand, so a below-floor run can
// be proven to issue no object or log command (AC-L134).
func installFakeVersionGit(t *testing.T, version string) *gitWrapper {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := fmt.Sprintf(`#!/bin/sh
{
  printf '%%s' "ARGS"
  for a in "$@"; do printf '\037%%s' "$a"; done
  printf '\n'
} >> %q
if [ "$1" = "--version" ]; then
  echo %q
  exit 0
fi
echo "fatal: shim refuses subcommand $1" >&2
exit 128
`, logPath, version)
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write git shim: %v", err)
	}
	w := &gitWrapper{t: t, dir: dir, logPath: logPath, oldPath: os.Getenv("PATH")}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+w.oldPath)
	return w
}

// Calls returns one entry per git invocation since Reset.
func (w *gitWrapper) Calls() []gitCall {
	w.t.Helper()
	data, err := os.ReadFile(w.logPath)
	if err != nil {
		return nil
	}
	var out []gitCall
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x1f")
		call := gitCall{Env: map[string]string{}}
		inEnv := false
		for _, f := range fields[1:] {
			if f == "ENV" {
				inEnv = true
				continue
			}
			if inEnv {
				kv := strings.SplitN(f, "=", 2)
				if len(kv) == 2 {
					call.Env[kv[0]] = kv[1]
				}
				continue
			}
			call.Args = append(call.Args, f)
		}
		out = append(out, call)
	}
	return out
}

func (w *gitWrapper) Reset() {
	_ = os.Remove(w.logPath)
}

type gitCall struct {
	Args []string
	Env  map[string]string
}

func (c gitCall) Subcommand() string {
	for _, a := range c.Args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

func (c gitCall) Has(flag string) bool {
	for _, a := range c.Args {
		if a == flag {
			return true
		}
	}
	return false
}

func (c gitCall) Joined() string { return strings.Join(c.Args, " ") }

func callsWithSubcommand(calls []gitCall, sub string) []gitCall {
	var out []gitCall
	for _, c := range calls {
		if c.Subcommand() == sub {
			out = append(out, c)
		}
	}
	return out
}

// RelandIdentical reverts the most recent landing's content and then
// re-introduces exactly the same change with the same trailer block.
// Both landings therefore introduce the SAME normalized change, which is
// the `duplicate-equivalent` / cherry-pick-and-merge-back class.
func (f *landedFixture) RelandIdentical() string {
	f.t.Helper()
	pre, _, _, err := gitutilBlob(f.Root, f.LandingCommit+"^", f.FilePath)
	if err != nil {
		f.t.Fatalf("read pre-landing blob: %v", err)
	}
	post, _, _, err := gitutilBlob(f.Root, f.LandingCommit, f.FilePath)
	if err != nil {
		f.t.Fatalf("read landing blob: %v", err)
	}
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), string(pre))
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "revert "+f.Slug)
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), string(post))
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "re-land "+f.Slug, "-m", f.TrailerBlock(f.Slug))
	f.LandingCommit = gitHeadOf(f.t, f.Root)
	return f.LandingCommit
}

// ExtendAndReland adds a SECOND change to the feature, re-records the
// canonical artifacts over the whole range, and lands them again. The
// newest landing then attests the current artifacts while its own parent
// already materializes the first half — the measured E30 shape where the
// attestation commit and the replay anchor differ.
func (f *landedFixture) ExtendAndReland(newBody string) string {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), newBody)
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "extend "+f.Slug)
	f.Record()
	mustWriteFile(f.t, filepath.Join(f.Root, ".tpatch", "features", f.Slug, "reland-marker.txt"), "reland\n")
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "feat: re-land "+f.Slug, "-m", f.TrailerBlock(f.Slug))
	f.LandingCommit = gitHeadOf(f.t, f.Root)
	return f.LandingCommit
}

// LandTrailerOnlyAfterChange commits the feature's user-code change
// WITHOUT a trailer and then commits the trailer block on top. The only
// landing candidate's parent therefore already materializes the patch,
// so no candidate forward-qualifies while anchor C stays clean.
func (f *landedFixture) LandTrailerOnlyAfterChange(newBody string) string {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.Root, f.FilePath), newBody)
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "implement without a trailer")
	f.Record()
	mustWriteFile(f.t, filepath.Join(f.Root, ".tpatch", "features", f.Slug, "landing-marker.txt"), "trailer-only\n")
	mustGit(f.t, f.Root, "add", "-A")
	mustGit(f.t, f.Root, "commit", "-q", "-m", "feat: attest "+f.Slug, "-m", f.TrailerBlock(f.Slug))
	f.LandingCommit = gitHeadOf(f.t, f.Root)
	return f.LandingCommit
}

// StaleTrailerBlock renders a well-formed four-trailer block whose
// digests do NOT match the current artifacts. Used to build replay-anchor
// candidates that can never become attestations (the D14 integrity
// boundary).
func (f *landedFixture) StaleTrailerBlock() string {
	f.t.Helper()
	st, err := f.Store.LoadFeatureStatus(f.Slug)
	if err != nil {
		f.t.Fatalf("load status: %v", err)
	}
	return fmt.Sprintf("Tpatch-Feature: %s\nTpatch-Patch-SHA: %s\nTpatch-Recipe-SHA: %s\nTpatch-Base-Commit: %s\n",
		f.Slug, strings.Repeat("a", 64), strings.Repeat("b", 64), st.Apply.BaseCommit)
}

func gitutilBlob(root, treeish, path string) ([]byte, bool, string, error) {
	return gitutil.BlobAtTree(root, treeish, path)
}

// Small indirections so fixtures in other files do not each import store.
func storeInit(root string) (*store.Store, error) { return store.Init(root) }
func storeOpen(root string) (*store.Store, error) { return store.Open(root) }
func storeStateApplied() store.FeatureState       { return store.StateApplied }
func storeAddInput(slug string) store.AddFeatureInput {
	return store.AddFeatureInput{Title: slug, Slug: slug, Request: "x"}
}

func jsonMarshal(v any) (string, error) {
	data, err := json.Marshal(v)
	return string(data), err
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func readFileBytes(path string) ([]byte, error) { return os.ReadFile(path) }

func mustLookGit(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	return p
}
