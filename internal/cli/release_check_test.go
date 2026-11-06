package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// OV-D18 (2026-11-06): `make release-check` enforces the release-candidate
// rule. The CHANGELOG section and the release-guard pin must be in the
// gated commit, and the tag commit may differ from it only in the tracking
// files. Each case builds a real repository with a gated commit, a
// tracking-only release commit and a bare origin.

const (
	rcScript  = "scripts/release-check.sh"
	rcRelease = "v0.20.0"
)

type rcFixture struct {
	repo, origin, gated string
}

type rcCase struct {
	name     string
	gated    func(t *testing.T, repo string)
	release  func(t *testing.T, repo string)
	after    func(t *testing.T, f *rcFixture)
	args     func(f *rcFixture) []string
	env      []string
	subdir   string
	wantExit int
	wantStep string
	wantMsg  string
}

func rcCases() []rcCase {
	release := func(args ...string) func(*rcFixture) []string {
		return func(f *rcFixture) []string { return append(args, f.gated) }
	}
	return []rcCase{
		{name: "pass"},
		{
			name:  "untracked-files-allowed",
			after: func(t *testing.T, f *rcFixture) { gh9Write(t, f.repo, "notes.txt", "scratch\n") },
		},
		{name: "usage-no-args", args: func(*rcFixture) []string { return nil }, wantExit: 2, wantMsg: "usage"},
		{name: "usage-missing-gated", args: func(*rcFixture) []string { return []string{rcRelease} }, wantExit: 2, wantMsg: "usage"},
		{name: "usage-empty-release", args: release(""), wantExit: 2, wantMsg: "usage"},
		{
			name:     "usage-extra-argument",
			args:     func(f *rcFixture) []string { return []string{rcRelease, f.gated, "extra"} },
			wantExit: 2, wantMsg: "usage",
		},
		{name: "not-top-level", subdir: "docs", wantExit: 2, wantMsg: "top level"},
		{name: "version-without-v", args: release("0.20.0"), wantExit: 1, wantStep: "[1/9]", wantMsg: "not vMAJOR.MINOR.PATCH"},
		{name: "version-two-parts", args: release("v0.20"), wantExit: 1, wantStep: "[1/9]", wantMsg: "not vMAJOR.MINOR.PATCH"},
		{name: "version-prerelease", args: release("v0.20.0-rc.1"), wantExit: 1, wantStep: "[1/9]", wantMsg: "not vMAJOR.MINOR.PATCH"},
		{name: "version-leading-zero", args: release("v00.20.0"), wantExit: 1, wantStep: "[1/9]", wantMsg: "not vMAJOR.MINOR.PATCH"},
		{
			name:     "dirty-tracked-file",
			after:    func(t *testing.T, f *rcFixture) { gh9Write(t, f.repo, "src.go", "package src\n// edited\n") },
			wantExit: 1, wantStep: "[2/9]", wantMsg: "uncommitted tracked changes",
		},
		{
			name: "staged-change",
			after: func(t *testing.T, f *rcFixture) {
				gh9Write(t, f.repo, "src.go", "package src\n// staged\n")
				gh9Git(t, f.repo, "add", "--", "src.go")
			},
			wantExit: 1, wantStep: "[2/9]", wantMsg: "uncommitted tracked changes",
		},
		{name: "gated-symbolic-ref", args: func(*rcFixture) []string { return []string{rcRelease, "HEAD~1"} }, wantExit: 1, wantStep: "[3/9]", wantMsg: "lowercase commit SHA"},
		{
			name:     "gated-uppercase-sha",
			args:     func(f *rcFixture) []string { return []string{rcRelease, strings.ToUpper(f.gated)} },
			wantExit: 1, wantStep: "[3/9]", wantMsg: "lowercase commit SHA",
		},
		{name: "gated-unknown", args: func(*rcFixture) []string { return []string{rcRelease, "0123456789abcdef"} }, wantExit: 1, wantStep: "[3/9]", wantMsg: "does not name a commit"},
		{
			name: "gated-not-ancestor",
			args: func(f *rcFixture) []string {
				return []string{rcRelease, strings.TrimSpace(rcGitIn(f.repo, "commit-tree", "HEAD^{tree}", "-m", "side"))}
			},
			wantExit: 1, wantStep: "[3/9]", wantMsg: "not HEAD or an ancestor",
		},
		{
			name:     "release-changes-changelog",
			release:  func(t *testing.T, repo string) { rcAppend(t, repo, "CHANGELOG.md", "- late entry\n") },
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name: "release-changes-pin-file",
			release: func(t *testing.T, repo string) {
				rcAppend(t, repo, "assets/recipe_authority_parity_test.go", "// late\n")
			},
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name:     "release-adds-source-file",
			release:  func(t *testing.T, repo string) { gh9Write(t, repo, "internal/new.go", "package internal\n") },
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name:     "release-deletes-source-file",
			release:  func(t *testing.T, repo string) { rcRemove(t, repo, "src.go") },
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			// With rename detection, the diff would list only the tracking
			// name and hide the deleted source file.
			name:  "release-renames-source-into-tracking",
			gated: func(t *testing.T, repo string) { rcRemove(t, repo, "docs/handoff/HISTORY.md") },
			release: func(t *testing.T, repo string) {
				rcRemove(t, repo, "docs/handoff/HISTORY.md")
				gh9Git(t, repo, "mv", "src.go", "docs/handoff/HISTORY.md")
			},
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name:     "literal-pathspecs-environment",
			release:  func(t *testing.T, repo string) { rcAppend(t, repo, "CHANGELOG.md", "- late entry\n") },
			env:      []string{"GIT_LITERAL_PATHSPECS=1"},
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name: "tracking-file-replaced-by-directory",
			release: func(t *testing.T, repo string) {
				rcRemove(t, repo, "docs/ROADMAP.md")
				gh9Write(t, repo, "docs/ROADMAP.md/extra.go", "package extra\n")
			},
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name:     "tracking-name-prefix",
			release:  func(t *testing.T, repo string) { gh9Write(t, repo, "docs/ROADMAP.md.orig", "# Roadmap\n") },
			wantExit: 1, wantStep: "[4/9]", wantMsg: "outside tracking",
		},
		{
			name: "changelog-older-release-on-top",
			gated: func(t *testing.T, repo string) {
				gh9Write(t, repo, "CHANGELOG.md", "# Changelog\n\n## v0.19.0 — 2026-11-06 — previous\n")
			},
			wantExit: 1, wantStep: "[5/9]", wantMsg: "top CHANGELOG heading",
		},
		{
			name: "changelog-unreleased-on-top",
			gated: func(t *testing.T, repo string) {
				gh9Write(t, repo, "CHANGELOG.md", "# Changelog\n\n## Unreleased\n\n## v0.20.0 — 2026-11-07 — fixture\n")
			},
			wantExit: 1, wantStep: "[5/9]", wantMsg: "top CHANGELOG heading",
		},
		{
			name:     "changelog-heading-without-title",
			gated:    func(t *testing.T, repo string) { gh9Write(t, repo, "CHANGELOG.md", "# Changelog\n\n## v0.20.0\n") },
			wantExit: 1, wantStep: "[5/9]", wantMsg: "top CHANGELOG heading",
		},
		{
			name:     "pin-mismatch",
			gated:    func(t *testing.T, repo string) { rcPin(t, repo, "const rgaCurrentChangelogRelease = \"0.19.0\"\n") },
			wantExit: 1, wantStep: "[6/9]", wantMsg: "found 0",
		},
		{
			name: "pin-duplicated",
			gated: func(t *testing.T, repo string) {
				rcPin(t, repo, "const rgaCurrentChangelogRelease = \"0.20.0\"\nconst rgaCurrentChangelogRelease = \"0.20.0\"\n")
			},
			wantExit: 1, wantStep: "[6/9]", wantMsg: "found 2",
		},
		{
			name:     "cluster-state-in-progress",
			release:  func(t *testing.T, repo string) { rcCurrent(t, repo, "**Cluster state**: IN PROGRESS\n") },
			wantExit: 1, wantStep: "[7/9]", wantMsg: "not terminal",
		},
		{
			name: "cluster-state-duplicated",
			release: func(t *testing.T, repo string) {
				rcCurrent(t, repo, "**Cluster state**: ACCEPTED\n\n**Cluster state**: ACCEPTED\n")
			},
			wantExit: 1, wantStep: "[7/9]", wantMsg: "found 2",
		},
		{
			name:     "cluster-state-missing",
			release:  func(t *testing.T, repo string) { rcCurrent(t, repo, "Cluster state: ACCEPTED\n") },
			wantExit: 1, wantStep: "[7/9]", wantMsg: "found 0",
		},
		{
			name: "log-does-not-record-gated",
			release: func(t *testing.T, repo string) {
				gh9Write(t, repo, "docs/supervisor/LOG.md", "# Log\n\n## Wave close — PASS\n")
			},
			wantExit: 1, wantStep: "[8/9]", wantMsg: "never mentions",
		},
		{
			name:     "tag-exists-locally",
			after:    func(t *testing.T, f *rcFixture) { gh9Git(t, f.repo, "tag", rcRelease) },
			wantExit: 1, wantStep: "[9/9]", wantMsg: "already exists locally",
		},
		{
			name: "tag-exists-on-origin",
			after: func(t *testing.T, f *rcFixture) {
				gh9Git(t, f.repo, "push", "-q", "origin", "HEAD:refs/tags/"+rcRelease)
			},
			wantExit: 1, wantStep: "[9/9]", wantMsg: "already exists on origin",
		},
		{
			name: "origin-unreachable",
			after: func(t *testing.T, f *rcFixture) {
				gh9Git(t, f.repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			},
			wantExit: 1, wantStep: "[9/9]", wantMsg: "could not query origin",
		},
	}
}

func TestReleaseCheckScript(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	script := filepath.Join(avpRepoRoot(t), rcScript)
	for _, c := range rcCases() {
		t.Run(c.name, func(t *testing.T) {
			if err := rcCheckCase(t, script, c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Every check must be load-bearing: each mutation of the real script must
// make its corresponding case fail.
func TestReleaseCheckScriptMutations(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	body := gh9ReadFile(t, filepath.Join(avpRepoRoot(t), rcScript))
	cases := map[string]rcCase{}
	for _, c := range rcCases() {
		cases[c.name] = c
	}
	for _, m := range []struct{ name, old, new, breaks string }{
		{"usage-check-removed", `if [ "$#" -ne 2 ] || [ -z "$release" ] || [ -z "$gated" ]; then`, "if false; then", "usage-missing-gated"},
		{"top-level-check-removed", `|| [ -n "$cdup" ]; then`, "; then", "not-top-level"},
		{"version-anchor-dropped", `grep -Eqx 'v(0|[1-9][0-9]*)`, `grep -Eq 'v(0|[1-9][0-9]*)`, "version-prerelease"},
		{"version-leading-zero-allowed", `'v(0|[1-9][0-9]*)\.`, `'v([0-9]+)\.`, "version-leading-zero"},
		{"dirty-tree-ignored", `if [ -n "$changes" ]; then`, "if false; then", "dirty-tracked-file"},
		{"gated-syntax-unchecked", `grep -Eqx '[0-9a-f]{7,40}'`, "true", "gated-symbolic-ref"},
		{"ancestry-unchecked", `if ! git merge-base --is-ancestor "$gated_full" "$head_full" 2>/dev/null; then`, "if false; then", "gated-not-ancestor"},
		{"extra-files-ignored", `if [ -n "$extra" ]; then`, "if false; then", "release-changes-changelog"},
		{"changelog-treated-as-tracking", `docs/ROADMAP.md"`, `docs/ROADMAP.md CHANGELOG.md"`, "release-changes-changelog"},
		{"rename-detection-restored", "git diff --no-renames --name-only", "git diff --name-only", "release-renames-source-into-tracking"},
		{"tracking-prefix-allowed", `if [ "$path" = "$f" ]; then`, `if case "$path" in ("$f"*) true ;; (*) false ;; esac; then`, "tracking-name-prefix"},
		{"pathspec-filter-restored-environment", rcExactFilter, rcPathspecFilter, "literal-pathspecs-environment"},
		{"pathspec-filter-restored-directory", rcExactFilter, rcPathspecFilter, "tracking-file-replaced-by-directory"},
		{"changelog-any-heading", `grep -m1 '^## '`, `grep -m1 "^## $release"`, "changelog-unreleased-on-top"},
		{"changelog-title-optional", `"## $release — "?*) ;;`, `"## $release"*) ;;`, "changelog-heading-without-title"},
		{"pin-duplicates-allowed", `if [ "$pins" != 1 ]; then`, `if [ "$pins" = 0 ]; then`, "pin-duplicated"},
		{"pin-unchecked", `if [ "$pins" != 1 ]; then`, "if false; then", "pin-mismatch"},
		{"state-duplicates-allowed", `if [ "$states" != 1 ]; then`, `if [ "$states" = 0 ]; then`, "cluster-state-duplicated"},
		{"state-mid-cycle-allowed", `SHIPPED | APPROVED | ACCEPTED | IDLE) ;;`, `SHIPPED | APPROVED | ACCEPTED | IDLE | "IN PROGRESS") ;;`, "cluster-state-in-progress"},
		{"log-unchecked", `if ! printf '%s\n' "$log_body" | grep -q -F "$gated_short"; then`, "if false; then", "log-does-not-record-gated"},
		{"local-tag-unchecked", `if git rev-parse --verify --quiet "refs/tags/$release" >/dev/null 2>&1; then`, "if false; then", "tag-exists-locally"},
		{"remote-tag-ignored", `0) fail "tag $release already exists on origin" ;;`, "0) ;;", "tag-exists-on-origin"},
		{"remote-failure-fails-open", `*) fail "could not query origin`, `*) : "could not query origin`, "origin-unreachable"},
	} {
		t.Run(m.name, func(t *testing.T) {
			mutated := strings.Replace(body, m.old, m.new, 1)
			if mutated == body {
				t.Fatal("mutation changed nothing")
			}
			c, ok := cases[m.breaks]
			if !ok {
				t.Fatalf("unknown case %q", m.breaks)
			}
			script := filepath.Join(t.TempDir(), "release-check.sh")
			if err := os.WriteFile(script, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
			// A mutant the shell cannot parse fails every case without
			// proving that the guarded check is what catches it.
			if out, err := exec.Command("sh", "-n", script).CombinedOutput(); err != nil {
				t.Fatalf("mutant does not parse: %v\n%s", err, out)
			}
			if err := rcCheckCase(t, script, c); err == nil {
				t.Fatalf("mutated script still satisfied %q", c.name)
			}
		})
	}
}

// rcExactFilter is the [4/9] filter; rcPathspecFilter is the exclude-pathspec
// filter it replaced, which both GIT_LITERAL_PATHSPECS and a directory that
// replaces a tracking file defeat.
const (
	rcExactFilter = `if ! changed=$(git diff --no-renames --name-only "$gated_full" "$head_full" 2>/dev/null); then
	fail "git diff failed"
fi
extra=$(printf '%s\n' "$changed" | while IFS= read -r path; do
	[ -n "$path" ] || continue
	allowed=no
	for f in $tracking_files; do
		if [ "$path" = "$f" ]; then
			allowed=yes
		fi
	done
	if [ "$allowed" = no ]; then
		printf '%s\n' "$path"
	fi
done)
`
	rcPathspecFilter = `set -- --
for f in $tracking_files; do
	set -- "$@" ":(exclude,literal)$f"
done
if ! extra=$(git diff --no-renames --name-only "$gated_full" "$head_full" "$@" 2>/dev/null); then
	fail "git diff failed"
fi
`
)

// The Makefile target passes RELEASE and GATED through the environment
// (make exports command-line variables), so their values are never parsed
// as shell syntax. The extracted target runs under the real make.
func TestReleaseCheckMakefileWiring(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}
	root := avpRepoRoot(t)
	makefile := gh9ReadFile(t, filepath.Join(root, "Makefile"))
	script := gh9ReadFile(t, filepath.Join(root, rcScript))
	phony := strings.SplitN(makefile, "\n", 2)[0]
	if !strings.HasPrefix(phony, ".PHONY:") || !strings.Contains(phony+" ", " release-check ") {
		t.Fatalf("release-check is not .PHONY: %q", phony)
	}
	if err := rcCheckMakefileWiring(t, makefile, script); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ name, old, new string }{
		{"values-pasted-into-shell", `"$$RELEASE" "$$GATED"`, `"$(RELEASE)" "$(GATED)"`},
		{"arguments-swapped", `"$$RELEASE" "$$GATED"`, `"$$GATED" "$$RELEASE"`},
		{"script-not-invoked", "\t@sh " + rcScript + " ", "\t@true "},
	} {
		t.Run(m.name, func(t *testing.T) {
			mutated := strings.Replace(makefile, m.old, m.new, 1)
			if mutated == makefile {
				t.Fatal("mutation changed nothing")
			}
			if err := rcCheckMakefileWiring(t, mutated, script); err == nil {
				t.Fatal("mutated Makefile wiring was accepted")
			}
		})
	}
}

func rcCheckMakefileWiring(t *testing.T, makefile, script string) error {
	t.Helper()
	target, err := rcMakefileTarget(makefile)
	if err != nil {
		return err
	}
	f := rcBuild(t, rcCase{})
	gh9Write(t, f.repo, rcScript, script)
	mk := filepath.Join(t.TempDir(), "release-check.mk")
	if err := os.WriteFile(mk, []byte(target), 0o600); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, kv := range gh9GitEnv(t) {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "MAKEFLAGS", "MFLAGS", "MAKELEVEL", "RELEASE", "GATED":
		default:
			env = append(env, kv)
		}
	}
	run := func(vars ...string) (string, int, error) {
		cmd := exec.Command("make", append([]string{"-s", "-f", mk, "release-check"}, vars...)...)
		cmd.Dir = f.repo
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode(), nil
		}
		return string(out), 0, err
	}
	out, exit, err := run("RELEASE="+rcRelease, "GATED="+f.gated)
	if err != nil || exit != 0 || !strings.HasSuffix(out, "=== Release check: PASS ===\n") {
		return fmt.Errorf("valid release: exit %d err %v\n%s", exit, err, out)
	}
	marker := filepath.Join(f.repo, "injected")
	out, exit, err = run(`RELEASE=v0.20.0"; touch injected; echo "`, "GATED="+f.gated)
	if _, statErr := os.Stat(marker); statErr == nil {
		return fmt.Errorf("RELEASE value was executed as shell syntax\n%s", out)
	}
	if err != nil || exit == 0 || !strings.Contains(out, "not vMAJOR.MINOR.PATCH") {
		return fmt.Errorf("injected RELEASE: exit %d err %v\n%s", exit, err, out)
	}
	out, exit, err = run("RELEASE=" + rcRelease)
	if err != nil || exit == 0 || !strings.Contains(out, "usage") {
		return fmt.Errorf("missing GATED: exit %d err %v\n%s", exit, err, out)
	}
	return nil
}

// rcMakefileTarget returns the release-check rule and its recipe lines,
// verbatim, as a standalone makefile.
func rcMakefileTarget(makefile string) (string, error) {
	const rule = "\nrelease-check:\n"
	start := strings.Index(makefile, rule)
	if start < 0 || strings.Count(makefile, "\nrelease-check:") != 1 {
		return "", errors.New("exactly one release-check target required")
	}
	target := rule[1:]
	recipe := 0
	for _, line := range strings.Split(makefile[start+len(rule):], "\n") {
		if !strings.HasPrefix(line, "\t") {
			break
		}
		target += line + "\n"
		recipe++
	}
	if recipe == 0 {
		return "", errors.New("release-check has no recipe")
	}
	return target, nil
}

func rcCheckCase(t *testing.T, script string, c rcCase) error {
	t.Helper()
	f := rcBuild(t, c)
	args := []string{rcRelease, f.gated}
	if c.args != nil {
		args = c.args(f)
	}
	cmd := exec.Command("sh", append([]string{script}, args...)...)
	cmd.Dir = filepath.Join(f.repo, c.subdir)
	cmd.Env = append(gh9GitEnv(t), c.env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	exit := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit = exitErr.ExitCode()
	} else if err != nil {
		return fmt.Errorf("%s: run release-check: %v", c.name, err)
	}
	out := stdout.String()
	if exit != c.wantExit || stderr.Len() != 0 {
		return fmt.Errorf("%s: exit %d stderr %q, want exit %d\n%s", c.name, exit, stderr.String(), c.wantExit, out)
	}
	if c.wantExit == 0 {
		if !strings.HasSuffix(out, "\n=== Release check: PASS ===\n") || strings.Count(out, "\n  OK") != 9 {
			return fmt.Errorf("%s: want nine OK checks and PASS\n%s", c.name, out)
		}
		return nil
	}
	if !strings.Contains(out, "  FAIL: ") || !strings.Contains(out, c.wantMsg) {
		return fmt.Errorf("%s: want a FAIL mentioning %q\n%s", c.name, c.wantMsg, out)
	}
	if c.wantExit == 2 {
		return nil
	}
	lastStep := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "[") {
			lastStep = line
		}
	}
	if !strings.HasPrefix(lastStep, c.wantStep) || !strings.HasSuffix(out, "\n=== Release check: FAIL ===\n") {
		return fmt.Errorf("%s: failed at %q, want %s\n%s", c.name, lastStep, c.wantStep, out)
	}
	return nil
}

// rcBuild creates a bare origin and a repository whose gated commit carries
// the v0.20.0 CHANGELOG section and release-guard pin, followed by a
// tracking-only release commit that records the gate in the LOG.
func rcBuild(t *testing.T, c rcCase) *rcFixture {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gh9Git(t, filepath.Dir(origin), "init", "-q", "--bare", origin)
	repo := gh9NewRepo(t)
	gh9Git(t, repo, "remote", "add", "origin", origin)
	gh9Write(t, repo, "CHANGELOG.md", "# Changelog\n\n## v0.20.0 — 2026-11-07 — fixture release\n\n- change\n\n## v0.19.0 — 2026-11-06 — previous\n")
	rcPin(t, repo, "const rgaCurrentChangelogRelease = \"0.20.0\"\n")
	rcCurrent(t, repo, "**Cluster state**: APPROVED\n")
	gh9Write(t, repo, "docs/handoff/HISTORY.md", "# History\n")
	gh9Write(t, repo, "docs/supervisor/LOG.md", "# Log\n")
	gh9Write(t, repo, "docs/ROADMAP.md", "# Roadmap\n")
	gh9Write(t, repo, "src.go", "package src\n\nfunc F() int { return 1 }\n")
	if c.gated != nil {
		c.gated(t, repo)
	}
	rcCommitAll(t, repo, "gated source")
	gated := strings.TrimSpace(gh9Git(t, repo, "rev-parse", "HEAD"))

	rcCurrent(t, repo, "**Cluster state**: ACCEPTED\n")
	gh9Write(t, repo, "docs/supervisor/LOG.md", "# Log\n\n## Wave close — gate PASS at `"+gated[:7]+"`\n")
	gh9Write(t, repo, "docs/handoff/HISTORY.md", "# History\n\n## archived window\n")
	gh9Write(t, repo, "docs/ROADMAP.md", "# Roadmap\n\n- released\n")
	if c.release != nil {
		c.release(t, repo)
	}
	rcCommitAll(t, repo, "release tracking")
	gh9Git(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	f := &rcFixture{repo: repo, origin: origin, gated: gated}
	if c.after != nil {
		c.after(t, f)
	}
	return f
}

func rcPin(t *testing.T, repo, consts string) {
	t.Helper()
	gh9Write(t, repo, "assets/recipe_authority_parity_test.go", "package assets\n\n"+consts)
}

func rcCurrent(t *testing.T, repo, field string) {
	t.Helper()
	gh9Write(t, repo, "docs/handoff/CURRENT.md", "# Current Handoff\n\n## Status\n\n"+field)
}

func rcAppend(t *testing.T, repo, rel, text string) {
	t.Helper()
	gh9Write(t, repo, rel, gh9ReadFile(t, filepath.Join(repo, filepath.FromSlash(rel)))+text)
}

func rcRemove(t *testing.T, repo, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func rcCommitAll(t *testing.T, repo, msg string) {
	t.Helper()
	gh9Git(t, repo, "add", "-A")
	gh9Git(t, repo, "-c", "user.name=rc", "-c", "user.email=rc@example.invalid", "commit", "-q", "-m", msg)
}

// rcGitIn runs Git for an args callback, which has no *testing.T; a failure
// returns its output, which the script then rejects as GATED.
func rcGitIn(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-c", "user.name=rc", "-c", "user.email=rc@example.invalid"}, args...)...)
	cmd.Dir = dir
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, _ := cmd.CombinedOutput()
	return string(out)
}
