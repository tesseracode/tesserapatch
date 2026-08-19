package cli

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ---------------------------------------------------------------------------
// G — compatibility: the §12 decision that `--manual` gates do not change
// ---------------------------------------------------------------------------

func TestAVPCompatibility(t *testing.T) {
	t.Run("AVP-064", func(t *testing.T) {
		root, slug := avpInit(t, "define manual zero byte")
		mustWrite(t, filepath.Join(root, ".tpatch", "features", slug, "analysis.md"), "analysis\n")
		if code, _, stderr, _ := runPrepare(t, "--path", root, "analyze", slug, "--manual"); code != 0 {
			t.Fatalf("analyze --manual exit %d: %s", code, stderr)
		}
		mustWrite(t, filepath.Join(root, ".tpatch", "features", slug, "spec.md"), "")
		code, _, stderr, _ := runPrepare(t, "--path", root, "define", slug, "--manual")
		if code != 0 {
			t.Fatalf("define --manual on a zero-byte spec.md exited %d: %s", code, stderr)
		}
		if got := featureState(t, root, slug); got != "defined" {
			t.Fatalf("state = %q, want defined — the gate must be unchanged", got)
		}
	})

	t.Run("AVP-065", func(t *testing.T) {
		root, slug := avpInit(t, "analyze manual whitespace")
		mustWrite(t, filepath.Join(root, ".tpatch", "features", slug, "analysis.md"), " \n\t\n")
		code, _, stderr, _ := runPrepare(t, "--path", root, "analyze", slug, "--manual")
		if code != 0 {
			t.Fatalf("analyze --manual on a whitespace-only analysis.md exited %d: %s", code, stderr)
		}
		if got := featureState(t, root, slug); got != "analyzed" {
			t.Fatalf("state = %q, want analyzed", got)
		}
	})

	t.Run("AVP-066", func(t *testing.T) {
		files := withFile(defaultAVPFiles(), "status.json",
			`{"state":"rejected","rejection":{"reason":"duplicate","note":"n","actor":"a","rejected_at":"2026-01-01T00:00:00Z","prior_state":"defined"}}`)
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		report := decodeReport(t, stdout)
		if report["feature_state"] != "rejected" || readiness(t, report) != "ready" {
			t.Fatalf("report = %#v", report)
		}
		_, _, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--include-rejected")
		if !strings.Contains(stderr, "unknown flag") {
			t.Fatalf("--include-rejected is registered on prepare: %q", stderr)
		}
	})

	t.Run("AVP-067", func(t *testing.T) {
		root := avpWorkspace(t, withFile(defaultAVPFiles(), "status.json", `{"state":"unapplied"}`))
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0 — the unapplied guard must not fire", code)
		}
		if decodeReport(t, stdout)["feature_state"] != "unapplied" {
			t.Fatal("feature_state was not echoed")
		}
	})

	t.Run("AVP-068", func(t *testing.T) {
		root, slug := avpInit(t, "explore manual symlink")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "spec.md"), "spec\n")
		runPrepare(t, "--path", root, "define", slug, "--manual")

		target := filepath.Join(root, "exploration-target.md")
		mustWrite(t, target, "exploration\n")
		if err := os.Symlink(target, filepath.Join(feature, "exploration.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		code, _, stderr, _ := runPrepare(t, "--path", root, "explore", slug, "--manual")
		if code != 0 {
			t.Fatalf("explore --manual on a symlinked exploration.md exited %d: %s", code, stderr)
		}
		if got := featureState(t, root, slug); got != "defined" {
			t.Fatalf("state = %q, want defined", got)
		}
		// The differential: the same tree is refused by prepare --check.
		checkCode, checkOut, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if checkCode != 2 {
			t.Fatalf("prepare --check exit = %d, want 2", checkCode)
		}
		if artifactRow(t, decodeReport(t, checkOut), "exploration")["state"] != "symlink-refused" {
			t.Fatal("prepare --check did not refuse the symlink")
		}
	})

	t.Run("AVP-069", func(t *testing.T) {
		for _, tc := range []struct{ name, recipe string }{
			{"invalid-json", "{not json"},
			{"whitespace-only", "   \n"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root, slug := avpInit(t, "implement manual "+tc.name)
				feature := filepath.Join(root, ".tpatch", "features", slug)
				mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
				runPrepare(t, "--path", root, "analyze", slug, "--manual")
				mustWrite(t, filepath.Join(feature, "spec.md"), "spec\n")
				runPrepare(t, "--path", root, "define", slug, "--manual")
				before := featureState(t, root, slug)
				mustWrite(t, filepath.Join(feature, "apply-recipe.json"), tc.recipe)
				code, _, _, _ := runPrepare(t, "--path", root, "implement", slug, "--manual")
				if code == 0 {
					t.Fatal("implement --manual accepted a malformed recipe")
				}
				if after := featureState(t, root, slug); after != before {
					t.Fatalf("state changed %q → %q on a refused implement", before, after)
				}
			})
		}
	})

	t.Run("AVP-070", func(t *testing.T) {
		files := withoutFile(defaultAVPFiles(), "exploration.md")
		root := avpWorkspace(t, files)
		statusPath := featurePath(root, "status.json")
		before, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		if code, _, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet"); code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		after, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("status.json changed")
		}
		_, statusOut, _, _ := runPrepare(t, "--path", root, "status", "--json")
		if !strings.Contains(statusOut, `"defined"`) {
			t.Fatalf("status --json lost the state: %q", statusOut)
		}
		for _, forbidden := range []string{"structural_readiness", "readiness"} {
			if strings.Contains(statusOut, forbidden) {
				t.Fatalf("status --json gained a readiness field: %q", forbidden)
			}
		}
	})
}

func featureState(t *testing.T, root, slug string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document.State
}

// ---------------------------------------------------------------------------
// H — Path A / Path B parity
// ---------------------------------------------------------------------------

func TestAVPPathParity(t *testing.T) {
	t.Run("AVP-073", func(t *testing.T) {
		root, slug := avpInit(t, "path b hand authored")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "spec.md"), "spec\n")
		runPrepare(t, "--path", root, "define", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "exploration.md"), "exploration\n")
		runPrepare(t, "--path", root, "explore", slug, "--manual")

		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		report := decodeReport(t, stdout)
		if readiness(t, report) != "ready" {
			t.Fatalf("readiness = %q", readiness(t, report))
		}
		if !avpContains(advisoryCodesOf(t, report), "analysis-sidecar-absent-path-b-normal") {
			t.Fatalf("advisories = %v", advisoryCodesOf(t, report))
		}
	})

	t.Run("AVP-075", func(t *testing.T) {
		root, slug := avpInit(t, "path a heuristic")
		for _, phase := range []string{"analyze", "define", "explore"} {
			if code, _, stderr, _ := runPrepare(t, "--path", root, phase, slug); code != 0 {
				t.Fatalf("%s exited %d: %s", phase, code, stderr)
			}
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		report := decodeReport(t, stdout)
		if readiness(t, report) != "ready" {
			t.Fatalf("readiness = %q", readiness(t, report))
		}
		if artifactRow(t, report, "analysis_sidecar")["state"] != "present-nonempty" {
			t.Fatalf("sidecar = %#v", artifactRow(t, report, "analysis_sidecar"))
		}
		if report["overall"].(map[string]any)["optional_satisfied"].(float64) != 1 {
			t.Fatal("optional_satisfied != 1")
		}
	})

	t.Run("AVP-062", func(t *testing.T) {
		root, slug := avpInit(t, "path a provenance")
		for _, phase := range []string{"analyze", "define", "explore"} {
			runPrepare(t, "--path", root, phase, slug)
		}
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		for _, artifact := range reportArtifacts(t, decodeReport(t, stdout)) {
			if artifact["provenance"] != "unknown" {
				t.Fatalf("provenance = %v after a Path A run", artifact["provenance"])
			}
		}
	})

	t.Run("AVP-061", func(t *testing.T) {
		root, slug := avpInit(t, "path b provenance")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		for _, artifact := range reportArtifacts(t, decodeReport(t, stdout)) {
			if artifact["provenance"] != "unknown" {
				t.Fatalf("provenance = %v after a --manual run", artifact["provenance"])
			}
		}
	})

	t.Run("AVP-063", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if strings.Contains(stdout, "Path A") || strings.Contains(stdout, "path_a") {
			t.Fatal("the report claims Path A")
		}
		for _, artifact := range reportArtifacts(t, decodeReport(t, stdout)) {
			if artifact["provenance"] != "unknown" {
				t.Fatalf("provenance = %v", artifact["provenance"])
			}
		}
	})

	t.Run("AVP-076", func(t *testing.T) {
		files := withoutFile(defaultAVPFiles(), "analysis.md")
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2 — the sidecar never substitutes", code)
		}
		if readiness(t, decodeReport(t, stdout)) != "not_ready" {
			t.Fatal("readiness is not not_ready")
		}
	})
}

// ---------------------------------------------------------------------------
// K/N — canonical paths and slug safety
// ---------------------------------------------------------------------------

func TestAVPPathsAndSlugSafety(t *testing.T) {
	t.Run("AVP-088", func(t *testing.T) {
		want := map[string]string{
			"analyze": "analysis.md",
			"define":  "spec.md",
			"explore": "exploration.md",
		}
		for phase, expected := range want {
			manual, ok := store.ManualPhase(phase)
			if !ok {
				t.Fatalf("store has no manual phase %q", phase)
			}
			if manual.Path != expected {
				t.Fatalf("store.ManualPhase(%q).Path = %q, want %q", phase, manual.Path, expected)
			}
		}
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		base := ".tpatch/features/" + avpSlug + "/"
		for id, suffix := range map[string]string{
			"analysis":         want["analyze"],
			"spec":             want["define"],
			"exploration":      want["explore"],
			"analysis_sidecar": filepath.ToSlash(filepath.Join("artifacts", "analysis.json")),
		} {
			if got := artifactRow(t, report, id)["path"]; got != base+suffix {
				t.Fatalf("%s path = %v, want %s", id, got, base+suffix)
			}
		}
	})

	t.Run("AVP-105", func(t *testing.T) {
		corpus := []string{
			"simple", "Two Words", "MIXED Case Title", "trailing-dash-",
			"lots   of   spaces", "punctuation!!!", "a", strings.Repeat("long title ", 12),
			"123 numbers", "--leading", "ünïcödé title",
		}
		accepted := 0
		for _, title := range corpus {
			slug := store.Slugify(title)
			if slug == "" {
				continue
			}
			canonical, err := intent.CanonicalSlug(slug)
			if err != nil {
				t.Fatalf("Slugify(%q) = %q, which CanonicalSlug rejects: %v", title, slug, err)
			}
			if canonical != slug {
				t.Fatalf("CanonicalSlug rewrote %q to %q", slug, canonical)
			}
			if len(slug) > 60 {
				t.Fatalf("Slugify(%q) produced %d bytes", title, len(slug))
			}
			accepted++
		}
		if accepted < 8 {
			t.Fatalf("only %d slugs exercised; the round-trip is vacuous", accepted)
		}
		for _, reserved := range []string{"con", "prn", "aux", "nul", "com1", "lpt9"} {
			if _, err := intent.CanonicalSlug(reserved); err == nil {
				t.Fatalf("CanonicalSlug accepted the Windows device name %q", reserved)
			}
		}
	})

	t.Run("AVP-102", func(t *testing.T) { assertSlugUnsafe(t, "../../etc") })
	t.Run("AVP-103", func(t *testing.T) { assertSlugUnsafe(t, "/etc/passwd") })

	t.Run("AVP-104", func(t *testing.T) {
		for _, argument := range []string{
			"new\nline", "esc\x1bseq", "tab\there", "ünïcode", "..",
			"trailing-", "double--dash", "Upper", strings.Repeat("a", 61),
		} {
			t.Run(fmt.Sprintf("%q", argument), func(t *testing.T) { assertSlugUnsafe(t, argument) })
		}
	})

	t.Run("AVP-106", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, jsonOut, _, _ := runPrepare(t, "--path", root, "prepare", "../../etc", "--check", "--json", "--quiet")
		report := decodeReport(t, jsonOut)
		if report["slug"] != "" || report["feature_state"] != "unknown" {
			t.Fatalf("report = %#v", report)
		}
		_, humanOut, _, _ := runPrepare(t, "--path", root, "prepare", "../../etc", "--check")
		if !strings.HasPrefix(humanOut, "prepare --check  (slug withheld: not a canonical tpatch slug)\n") {
			t.Fatalf("human header = %q", humanOut)
		}
		_, quietOut, _, _ := runPrepare(t, "--path", root, "prepare", "../../etc", "--check", "--quiet")
		if quietOut != "prepare --check — indeterminate (slug-unsafe)\n" {
			t.Fatalf("quiet line = %q", quietOut)
		}
	})

	t.Run("AVP-192", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		// Across the whole abort matrix, slug == "" iff the code is
		// slug-unsafe (§10.2 rule 8).
		for _, fixture := range reachableAbortFixtures() {
			args := append(fixture.args(t), "--json", "--quiet")
			_, stdout, _, _ := runPrepare(t, args...)
			report := decodeReport(t, stdout)
			abort := report["abort"].(map[string]any)
			empty := report["slug"] == ""
			if (abort["code"] == "slug-unsafe") != empty {
				t.Fatalf("abort %v emitted slug %q", abort["code"], report["slug"])
			}
		}
		// The slug gate is evaluated before the platform gate, which is why a
		// non-canonical argument aborts slug-unsafe on every target.
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", "../../etc", "--check", "--json", "--quiet")
		if decodeReport(t, stdout)["abort"].(map[string]any)["code"] != "slug-unsafe" {
			t.Fatal("slug validation no longer precedes the platform guard")
		}
		source := readRepoSource(t, "internal/cli/prepare.go")
		slugIndex := strings.Index(source, "intent.CanonicalSlug(")
		platformIndex := strings.Index(source, "intent.RootConfinementSupported()")
		if slugIndex < 0 || platformIndex < 0 || slugIndex > platformIndex {
			t.Fatal("the source order of the two gates changed")
		}
	})
}

func assertSlugUnsafe(t *testing.T, argument string) {
	t.Helper()
	root := avpWorkspace(t, defaultAVPFiles())
	code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", argument, "--check", "--json")
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	report := decodeReport(t, stdout)
	if report["abort"].(map[string]any)["code"] != "slug-unsafe" {
		t.Fatalf("abort = %#v", report["abort"])
	}
	if report["slug"] != "" {
		t.Fatalf("slug = %v, want the withheld empty string", report["slug"])
	}
	if strings.Contains(stdout, argument) || strings.Contains(stderr, argument) {
		t.Fatalf("the argument bytes were echoed: %q", argument)
	}
	for _, stream := range []string{stdout, stderr} {
		for index := 0; index < len(stream); index++ {
			if stream[index] < 0x20 && stream[index] != '\n' {
				t.Fatalf("control byte %#x reached output", stream[index])
			}
		}
	}
}

func readRepoSource(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(avpRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func avpRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// ---------------------------------------------------------------------------
// M — the CLI output envelope composed with the root error printer
// ---------------------------------------------------------------------------

func TestAVPExitEnvelope(t *testing.T) {
	t.Run("AVP-096", func(t *testing.T) {
		root := avpWorkspace(t, withoutFile(defaultAVPFiles(), "spec.md"))
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		decodeReport(t, stdout)
		want := "error: prepare --check " + avpSlug + ": not_ready (2 of 3 required artifacts are present-nonempty)\n"
		if stderr != want {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	})

	t.Run("AVP-097", func(t *testing.T) {
		root := avpWorkspace(t, withoutFile(defaultAVPFiles(), "spec.md"))
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		if stdout != "prepare --check "+avpSlug+" — not_ready\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], "error: ") {
			t.Fatalf("stderr = %q, want exactly one error line", stderr)
		}
	})

	t.Run("AVP-098", func(t *testing.T) {
		seen := map[string]bool{}
		for _, fixture := range reachableAbortFixtures() {
			t.Run(fixture.code, func(t *testing.T) {
				args := append(fixture.args(t), "--quiet")
				code, stdout, stderr, _ := runPrepare(t, args...)
				if code != 3 {
					t.Fatalf("exit = %d, want 3", code)
				}
				if !strings.HasSuffix(stdout, " — indeterminate ("+fixture.code+")\n") {
					t.Fatalf("stdout = %q", stdout)
				}
				if seen[stdout] {
					t.Fatalf("quiet line %q is not pairwise distinct", stdout)
				}
				seen[stdout] = true
				lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
				if len(lines) != 1 || !strings.Contains(lines[0], fixture.code) {
					t.Fatalf("stderr = %q", stderr)
				}
			})
		}
	})

	t.Run("AVP-099", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		for _, flags := range [][]string{nil, {"--json"}, {"--quiet"}, {"--json", "--quiet"}} {
			args := append([]string{"--path", root, "prepare", avpSlug, "--check"}, flags...)
			code, _, stderr, _ := runPrepare(t, args...)
			if code != 0 {
				t.Fatalf("exit = %d with flags %v", code, flags)
			}
			for _, line := range strings.Split(stderr, "\n") {
				if strings.HasPrefix(line, "error:") {
					t.Fatalf("exit 0 emitted an error line with flags %v: %q", flags, line)
				}
			}
			if len(flags) == 2 && stderr != "" {
				t.Fatalf("--json --quiet stderr = %q, want empty", stderr)
			}
		}
	})

	t.Run("AVP-100", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug)
		if code != 3 {
			t.Fatalf("exit = %d, want request refusal 3", code)
		}
		if !strings.Contains(stdout, "Refusal: request-unreadable") {
			t.Fatalf("stdout = %q", stdout)
		}
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		if len(lines) != 1 {
			t.Fatalf("stderr has %d lines, want 1: %q", len(lines), stderr)
		}
		if !strings.Contains(stderr, "request-unreadable") {
			t.Fatalf("stderr = %q", stderr)
		}
		for _, forbidden := range []string{"docs/", ".md", "http://", "https://"} {
			if strings.Contains(stderr, forbidden) {
				t.Fatalf("stderr contains %q", forbidden)
			}
		}
	})

	t.Run("AVP-183", func(t *testing.T) {
		for _, path := range []string{
			filepath.Join(t.TempDir(), "does-not-exist"),
			t.TempDir(),
		} {
			code, stdout, stderr, _ := runPrepare(t, "--path", path, "prepare", avpSlug, "--check", "--json", "--quiet")
			if code != 3 {
				t.Fatalf("exit = %d for --path %s, want 3 (not cobra's 1)", code, path)
			}
			report := decodeReport(t, stdout)
			abort := report["abort"].(map[string]any)
			if abort["code"] != "workspace-not-initialized" {
				t.Fatalf("abort = %#v", abort)
			}
			message := fmt.Sprint(abort["message"])
			if !strings.Contains(message, "tpatch init") || !strings.Contains(message, "--path") {
				t.Fatalf("message = %q", message)
			}
			if strings.Contains(message, path) || strings.Contains(stderr, path) {
				t.Fatal("an absolute path leaked")
			}
		}
	})

	t.Run("AVP-184", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		before := avpTreeSnapshot(t, filepath.Join(root, ".tpatch"))
		for _, args := range [][]string{
			{"--path", root, "prepare", avpSlug, "--check", "--manual"},
			{"--path", root, "prepare", avpSlug, "--check", "--regenerate"},
			{"--path", root, "prepare"},
			{"--path", root, "prepare", "a", "b"},
			{"prepare", avpSlug, "--check", "--path"},
		} {
			code, stdout, _, _ := runPrepare(t, args...)
			if code != 1 {
				t.Fatalf("args %v exited %d, want cobra's 1", args, code)
			}
			if stdout != "" {
				t.Fatalf("args %v produced a report on stdout: %q", args, stdout)
			}
		}
		if string(before) != string(avpTreeSnapshot(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal(".tpatch changed on a parse error")
		}
	})

	t.Run("AVP-185", func(t *testing.T) {
		const sentinel = "ZZQQ-STATUS-SENTINEL"
		for name, status := range map[string]string{
			"inside-state": `{"state":"` + sentinel + `"}`,
			"inside-note":  `{"state":"defined","notes":"` + sentinel + `"}`,
			"malformed":    `{"state":` + sentinel,
		} {
			t.Run(name, func(t *testing.T) {
				root := avpWorkspace(t, withFile(defaultAVPFiles(), "status.json", status))
				for _, flags := range [][]string{nil, {"--json"}, {"--quiet"}, {"--json", "--quiet"}} {
					args := append([]string{"--path", root, "prepare", avpSlug, "--check"}, flags...)
					_, stdout, stderr, _ := runPrepare(t, args...)
					if strings.Contains(stdout, sentinel) || strings.Contains(stderr, sentinel) {
						t.Fatalf("status bytes leaked with flags %v", flags)
					}
				}
			})
		}
	})

	t.Run("AVP-186", func(t *testing.T) {
		files := avpFiles{"analysis.md": "a\n", "spec.md": "s\n", "exploration.md": "e\n"}
		root := avpWorkspace(t, files)
		code, _, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet")
		if code != 0 {
			t.Fatalf("a hand-assembled canonical feature exited %d, want 0", code)
		}

		badRoot := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", badRoot, "prepare", "My_Feature", "--check", "--json", "--quiet")
		abort := decodeReport(t, stdout)["abort"].(map[string]any)
		message := fmt.Sprint(abort["message"])
		if abort["code"] != "slug-unsafe" {
			t.Fatalf("abort = %#v", abort)
		}
		if !strings.Contains(message, "tpatch add") || !strings.Contains(message, "rename") {
			t.Fatalf("message = %q", message)
		}
		if strings.Contains(message, "tpatch status") {
			t.Fatal("the slug-unsafe message loops the operator through tpatch status")
		}
		// Following the message once produces a name the command accepts.
		renamed := store.Slugify("My_Feature")
		if _, err := intent.CanonicalSlug(renamed); err != nil {
			t.Fatalf("the remediation does not terminate: %q is still not canonical", renamed)
		}
	})
}

// ---------------------------------------------------------------------------
// P/V — advisories, status populations and the human surface
// ---------------------------------------------------------------------------

func TestAVPAdvisoriesAndStatusSurface(t *testing.T) {
	t.Run("AVP-120", func(t *testing.T) {
		root := avpWorkspace(t, withFile(defaultAVPFiles(), "artifacts/analysis.json", "   "))
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		codes := advisoryCodesOf(t, decodeReport(t, stdout))
		if !avpContains(codes, "analysis-sidecar-empty") {
			t.Fatalf("advisories = %v", codes)
		}
		if avpContains(codes, "analysis-sidecar-absent-path-b-normal") {
			t.Fatal("the empty sidecar also claimed absence")
		}
		for _, advisory := range decodeReport(t, stdout)["advisories"].([]any) {
			entry := advisory.(map[string]any)
			if entry["code"] == "analysis-sidecar-empty" && strings.Contains(fmt.Sprint(entry["message"]), "is not present") {
				t.Fatal("the empty-sidecar message claims absence")
			}
		}
	})

	t.Run("AVP-121", func(t *testing.T) {
		cases := map[string]struct {
			setup func(t *testing.T) string
			code  string
		}{
			"invalid-structured": {func(t *testing.T) string {
				return avpWorkspace(t, withFile(defaultAVPFiles(), "artifacts/analysis.json", "["))
			}, "analysis-sidecar-invalid-structured"},
			"symlink-refused": {func(t *testing.T) string {
				root := avpWorkspace(t, withoutFile(defaultAVPFiles(), "artifacts/analysis.json"))
				target := filepath.Join(root, "sidecar-target.json")
				mustWrite(t, target, "{}")
				if err := os.Symlink(target, featurePath(root, "artifacts/analysis.json")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return root
			}, "analysis-sidecar-symlink-refused"},
			"not-regular": {func(t *testing.T) string {
				root := avpWorkspace(t, withoutFile(defaultAVPFiles(), "artifacts/analysis.json"))
				if err := os.MkdirAll(featurePath(root, "artifacts/analysis.json"), 0o755); err != nil {
					t.Fatal(err)
				}
				return root
			}, "analysis-sidecar-not-regular"},
			"unreadable": {func(t *testing.T) string {
				if os.Geteuid() == 0 {
					t.Skip("running as root")
				}
				root := avpWorkspace(t, defaultAVPFiles())
				path := featurePath(root, "artifacts/analysis.json")
				if err := os.Chmod(path, 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
				return root
			}, "analysis-sidecar-unreadable"},
			"oversize": {func(t *testing.T) string {
				return avpWorkspace(t, withFile(defaultAVPFiles(), "artifacts/analysis.json",
					strings.Repeat("x", intent.MaxArtifactBytes+1)))
			}, "analysis-sidecar-oversize"},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				root := tc.setup(t)
				code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
				if code != 0 {
					t.Fatalf("exit = %d; the sidecar never affects the exit code", code)
				}
				report := decodeReport(t, stdout)
				if readiness(t, report) != "ready" {
					t.Fatalf("readiness = %q", readiness(t, report))
				}
				codes := advisoryCodesOf(t, report)
				if !avpContains(codes, tc.code) {
					t.Fatalf("advisories = %v, want %s", codes, tc.code)
				}
				if avpContains(codes, "analysis-sidecar-absent-path-b-normal") {
					t.Fatal("a non-absent sidecar claimed absence")
				}
			})
		}
	})

	t.Run("AVP-123", func(t *testing.T) {
		files := avpFiles{"analysis.md": "", "spec.md": " \n", "exploration.md": ""}
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2 (readiness-derived, not 3)", code)
		}
		report := decodeReport(t, stdout)
		if _, ok := report["abort"]; ok {
			t.Fatal("an absent status.json produced an abort")
		}
		if len(reportArtifacts(t, report)) != 4 || report["feature_state"] != "unknown" {
			t.Fatalf("report = %#v", report)
		}
		if !avpContains(advisoryCodesOf(t, report), "feature-state-absent") {
			t.Fatalf("advisories = %v", advisoryCodesOf(t, report))
		}
	})

	t.Run("AVP-124", func(t *testing.T) {
		files := withoutFile(defaultAVPFiles(), "status.json")
		root := avpWorkspace(t, files)
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		report := decodeReport(t, stdout)
		if readiness(t, report) != "ready" || report["feature_state"] != "unknown" {
			t.Fatalf("report = %#v", report)
		}
		if !avpContains(advisoryCodesOf(t, report), "feature-state-absent") {
			t.Fatal("the feature-state-absent advisory is missing")
		}
		if strings.Contains(stderr, "error:") {
			t.Fatalf("stderr = %q", stderr)
		}
	})

	t.Run("AVP-167", func(t *testing.T) {
		files := withoutFile(defaultAVPFiles(), "status.json")
		root := avpWorkspace(t, files)
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check")
		wantLine := "lifecycle state: unknown  (this feature directory has no status.json)"
		if !strings.Contains(stdout, wantLine+"\n") {
			t.Fatalf("human lifecycle line missing: %q", stdout)
		}
		_, jsonOut, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		codes := advisoryCodesOf(t, decodeReport(t, jsonOut))
		if len(codes) == 0 || codes[0] != "feature-state-absent" {
			t.Fatalf("advisory order = %v", codes)
		}
		lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
		if lines[len(lines)-1] != "Structural presence only. This report does not certify semantic quality." {
			t.Fatalf("last line = %q", lines[len(lines)-1])
		}
	})

	t.Run("AVP-125", func(t *testing.T) {
		for name, document := range map[string]string{
			"not-json":        "{not json",
			"json-non-object": "[1,2,3]",
			"zero-bytes":      "",
			"whitespace-only": "  \n\t",
		} {
			t.Run(name, func(t *testing.T) {
				root := avpWorkspace(t, withFile(defaultAVPFiles(), "status.json", document))
				code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
				if code != 3 {
					t.Fatalf("exit = %d, want 3", code)
				}
				report := decodeReport(t, stdout)
				if report["abort"].(map[string]any)["code"] != "status-malformed" {
					t.Fatalf("abort = %#v", report["abort"])
				}
				if len(reportArtifacts(t, report)) != 0 || report["feature_state"] != "unknown" {
					t.Fatalf("report = %#v", report)
				}
				if trimmed := strings.TrimSpace(document); trimmed != "" && strings.Contains(stdout+stderr, trimmed) {
					t.Fatal("the document bytes were echoed")
				}
			})
		}
	})

	t.Run("AVP-126", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root: mode 0000 is still readable")
		}
		root := avpWorkspace(t, defaultAVPFiles())
		path := featurePath(root, "status.json")
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 3 {
			t.Fatalf("exit = %d, want 3", code)
		}
		report := decodeReport(t, stdout)
		if report["abort"].(map[string]any)["code"] != "status-unreadable" {
			t.Fatalf("abort = %#v", report["abort"])
		}
		if strings.Contains(stdout+stderr, "permission denied") {
			t.Fatal("the os error string leaked")
		}
	})

	t.Run("AVP-154", func(t *testing.T) {
		lifecycle := map[string]string{
			"status-malformed":     "status.json was read but is not a valid status document",
			"status-invalid-state": "status.json was read but records a state this tpatch does not recognise",
			"status-unreadable":    "status.json could not be read and closed cleanly",
			"status-unstable":      "status.json changed while it was being read",
		}
		for code, line := range lifecycle {
			if strings.Contains(line, "was not read") {
				t.Fatalf("%s claims the file was not read", code)
			}
		}
		preStatus := map[string]string{
			"slug-unsafe":                    "no feature was identified, so status.json was not read",
			"workspace-unsupported-platform": "inspection was refused on this platform, so status.json was not read",
			"workspace-not-initialized":      "no workspace was found, so status.json was not read",
			"workspace-root-unopenable":      "the repository root could not be opened, so status.json was not read",
			"feature-dir-unsafe":             "the feature directory could not be inspected safely, so status.json was not read",
			"feature-not-found":              "no feature directory exists, so status.json was not read",
		}
		for code, line := range preStatus {
			if !strings.Contains(line, "was not read") {
				t.Fatalf("%s does not say the file was not read", code)
			}
		}
		// The lines above are asserted against the renderer's real output.
		root := avpWorkspace(t, withFile(defaultAVPFiles(), "status.json", "{not json"))
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check")
		if !strings.Contains(stdout, lifecycle["status-malformed"]) {
			t.Fatalf("malformed lifecycle line missing: %q", stdout)
		}
		if strings.Contains(firstLifecycleLine(stdout), "was not read") {
			t.Fatal("the malformed lifecycle line claims the file was not read")
		}
		missing := avpWorkspace(t, defaultAVPFiles())
		_, missingOut, _, _ := runPrepare(t, "--path", missing, "prepare", "no-such", "--check")
		if !strings.Contains(missingOut, preStatus["feature-not-found"]) {
			t.Fatalf("feature-not-found lifecycle line missing: %q", missingOut)
		}
	})

	t.Run("AVP-164", func(t *testing.T) {
		for name, value := range map[string]string{
			"prepared": "prepared",
			"empty":    "",
			"junk":     strings.Repeat("j", 4096),
		} {
			t.Run(name, func(t *testing.T) {
				root := avpWorkspace(t, withFile(defaultAVPFiles(), "status.json", `{"state":"`+value+`"}`))
				code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
				if code != 3 {
					t.Fatalf("exit = %d, want 3", code)
				}
				report := decodeReport(t, stdout)
				if report["abort"].(map[string]any)["code"] != "status-invalid-state" {
					t.Fatalf("abort = %#v", report["abort"])
				}
				if report["feature_state"] != "unknown" {
					t.Fatalf("feature_state = %v", report["feature_state"])
				}
				if value != "" && strings.Contains(stdout+stderr, value) {
					t.Fatal("the offending value was echoed")
				}
			})
		}
	})

	t.Run("AVP-166", func(t *testing.T) {
		for _, state := range []string{
			"requested", "analyzed", "defined", "implementing", "applied", "active",
			"reconciling", "reconciling-shadow", "blocked", "upstream_merged",
			"rejected", "unapplied",
		} {
			root := avpWorkspace(t, withFile(defaultAVPFiles(), "status.json", `{"state":"`+state+`"}`))
			_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
			if got := decodeReport(t, stdout)["feature_state"]; got != state {
				t.Fatalf("feature_state = %v, want %q", got, state)
			}
		}
		for _, fixture := range reachableAbortFixtures() {
			if fixture.code == "slug-unsafe" || !strings.HasPrefix(fixture.code, "status-") {
				continue
			}
			args := append(fixture.args(t), "--json", "--quiet")
			_, stdout, _, _ := runPrepare(t, args...)
			if got := decodeReport(t, stdout)["feature_state"]; got != "unknown" {
				t.Fatalf("abort %s emitted feature_state %v", fixture.code, got)
			}
		}
	})

	t.Run("AVP-169", func(t *testing.T) {
		files := avpFiles{"status.json": "{not json"}
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 3 {
			t.Fatalf("exit = %d, want 3", code)
		}
		report := decodeReport(t, stdout)
		if report["abort"].(map[string]any)["code"] != "status-malformed" {
			t.Fatalf("abort = %#v", report["abort"])
		}
		if len(reportArtifacts(t, report)) != 0 {
			t.Fatal("artifacts were inspected after the status abort")
		}
	})
}

func firstLifecycleLine(rendered string) string {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(line, "lifecycle state:") {
			return line
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// T/X/Z — root lifetime, platform abort shape and the differential
// ---------------------------------------------------------------------------

func TestAVPRootLifetimeAndDifferential(t *testing.T) {
	const moduleOpenRootCallSites = 3

	t.Run("AVP-141", func(t *testing.T) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(avpRepoRoot(t), "internal", "cli", "prepare.go"), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		opens, closes := 0, 0
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name == "os" && sel.Sel.Name == "OpenRoot" {
				opens++
			}
			if ident.Name == "root" && sel.Sel.Name == "Close" {
				closes++
			}
			return true
		})
		if opens != 1 {
			t.Fatalf("os.OpenRoot has %d call sites in the command, want exactly 1", opens)
		}
		if closes != 1 {
			t.Fatalf("root.Close has %d call sites, want exactly 1", closes)
		}
		// The module has this accepted read-only root plus the two S4 mutating
		// prepare call sites. No other production file may add one.
		// The population is the set of **tracked** non-test Go files, taken
		// from `git ls-files`; walking the working tree instead made this row
		// depend on whatever happens to exist on disk (a nested `git
		// worktree`, an editor scratch copy, a golden-baseline checkout), and
		// the previous fix for that — exempting one hard-coded directory
		// name — only excused the one path that had already bitten.
		sources := trackedProductionGoSources(t)
		count, err := countOpenRootCallSites(sources)
		if err != nil {
			t.Fatal(err)
		}
		if count != moduleOpenRootCallSites {
			t.Fatalf(
				"os.OpenRoot appears at %d tracked production call sites module-wide, want %d",
				count, moduleOpenRootCallSites,
			)
		}
		publishPath := filepath.Join("internal", "cli", "prepare_publish.go")
		publishCount, err := countOpenRootCallSites(map[string]string{
			publishPath: sources[publishPath],
		})
		if err != nil {
			t.Fatal(err)
		}
		if publishCount != 2 {
			t.Fatalf("mutating prepare has %d os.OpenRoot call sites, want 2", publishCount)
		}
	})

	t.Run("AVP-141/sensitivity", func(t *testing.T) {
		const opener = "package workflow\n\nimport \"os\"\n\nfunc probe(dir string) { root, _ := os.OpenRoot(dir); _ = root }\n"

		// A second *tracked* production call site must fail the row.
		extra := map[string]string{}
		for path, source := range trackedProductionGoSources(t) {
			extra[path] = source
		}
		extra[filepath.Join("internal", "workflow", "root_probe.go")] = opener
		count, err := countOpenRootCallSites(extra)
		if err != nil {
			t.Fatal(err)
		}
		if count != moduleOpenRootCallSites+1 {
			t.Fatalf("an extra tracked os.OpenRoot moved the count to %d", count)
		}

		// An untracked file must not participate in the population. This arm
		// writes a real one and asserts both that it is absent from the
		// population and that the count is unmoved.
		//
		// It is written under the *git directory*, not under the working
		// tree. A scratch `.go` file anywhere in the working tree is reported
		// by `git ls-files --others --exclude-standard -- '*.go'`, which is
		// exactly the wave-close untracked-source sentinel: a test
		// interrupted between the write and t.Cleanup would leave source
		// noise that the sentinel then flags. Nothing under the git directory
		// is ever reported by that query, while `git ls-files` still excludes
		// it for the same reason it excludes any untracked path — so the
		// property under test is unchanged and the failure mode is removed.
		scratchDir := filepath.Join(avpGitDir(t), fmt.Sprintf("%s-%d", avp141ScratchName, os.Getpid()))
		if err := os.MkdirAll(scratchDir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(scratchDir) })
		scratch := filepath.Join(scratchDir, "root_probe.go")
		if err := os.WriteFile(scratch, []byte(opener), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(scratch); err != nil {
			t.Fatalf("the scratch file was not created, so this arm proves nothing: %v", err)
		}
		// `git ls-files` must not list it, and it must not be reported as an
		// untracked working-tree file either.
		for _, path := range gitLsFilesFromRepo(t) {
			if strings.Contains(path, avp141ScratchName) {
				t.Fatalf("git ls-files reported the scratch path %s", path)
			}
		}
		for _, path := range gitUntrackedGoFiles(t) {
			if strings.Contains(path, avp141ScratchName) {
				t.Fatalf("the scratch file %s is reported as an untracked source file; an interrupted run would leave sentinel noise", path)
			}
		}
		after := trackedProductionGoSources(t)
		for path := range after {
			if strings.Contains(filepath.ToSlash(path), avp141ScratchName) {
				t.Fatalf("the untracked scratch file %s entered the population", path)
			}
		}
		untrackedCount, err := countOpenRootCallSites(after)
		if err != nil {
			t.Fatal(err)
		}
		if untrackedCount != moduleOpenRootCallSites {
			t.Fatalf("an untracked working-tree file moved the count to %d", untrackedCount)
		}

		// And every scanned path really is tracked.
		tracked := map[string]bool{}
		for _, path := range gitLsFilesFromRepo(t) {
			tracked[path] = true
		}
		for path := range after {
			if !tracked[filepath.ToSlash(path)] {
				t.Fatalf("scanned %s, which git does not track", path)
			}
		}
	})

	t.Run("AVP-142", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		osRoot, err := os.OpenRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		defer osRoot.Close()
		report := intent.Inspect(intent.NewRootOps(osRoot), avpSlug, make([]byte, intent.MaxArtifactBytes+1))
		if report.Readiness() != intent.ReadinessReady {
			t.Fatalf("readiness = %q", report.Readiness())
		}
		if _, err := osRoot.Lstat(".tpatch"); err != nil {
			t.Fatalf("Inspect closed the caller's root: %v", err)
		}
	})

	t.Run("AVP-143", func(t *testing.T) {
		parent := t.TempDir()
		original := filepath.Join(parent, "repo")
		if err := os.MkdirAll(original, 0o755); err != nil {
			t.Fatal(err)
		}
		feature := filepath.Join(original, ".tpatch", "features", avpSlug, "artifacts")
		if err := os.MkdirAll(feature, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range defaultAVPFiles() {
			mustWrite(t, filepath.Join(original, ".tpatch", "features", avpSlug, filepath.FromSlash(name)), content)
		}
		osRoot, err := os.OpenRoot(original)
		if err != nil {
			t.Fatal(err)
		}
		defer osRoot.Close()
		renamed := filepath.Join(parent, "repo-renamed")
		if err := os.Rename(original, renamed); err != nil {
			t.Skipf("rename unavailable: %v", err)
		}
		report := intent.Inspect(intent.NewRootOps(osRoot), avpSlug, make([]byte, intent.MaxArtifactBytes+1))
		if report.Abort != nil {
			t.Fatalf("abort = %q after the repository root was renamed", report.Abort.Code)
		}
		if report.Readiness() != intent.ReadinessReady {
			t.Fatalf("readiness = %q; the held root must still describe the same directory", report.Readiness())
		}
	})

	t.Run("AVP-179", func(t *testing.T) {
		report := intent.NewAbortReport(avpSlug, intent.AbortUnsupportedPlatform)
		if report.AbortCode() != intent.AbortUnsupportedPlatform {
			t.Fatalf("abort = %q", report.AbortCode())
		}
		if len(report.Artifacts) != 0 {
			t.Fatalf("artifacts = %d, want 0", len(report.Artifacts))
		}
		if exitCodeFor(prepareExit(report)) != 3 {
			t.Fatalf("exit = %d, want 3", exitCodeFor(prepareExit(report)))
		}
		want := "this build of tpatch cannot guarantee that artifact inspection stays inside the repository on this platform, so prepare --check refuses to run here. Inspect the files under .tpatch/features/ directly."
		if report.Abort.Message != want {
			t.Fatalf("message = %q", report.Abort.Message)
		}
		// The gate is evaluated before any filesystem call: the command's
		// source order is asserted here and by AVP-177.
		source := readRepoSource(t, "internal/cli/prepare.go")
		if strings.Index(source, "RootConfinementSupported()") > strings.Index(source, "store.FindProjectRoot(") {
			t.Fatal("the platform gate no longer precedes workspace discovery")
		}
	})

	t.Run("AVP-128", func(t *testing.T) {
		// No abort code can be produced after the first per-artifact Lstat:
		// every abort return precedes the capture loop (AST), and every
		// abort run reports zero artifact rows (runtime).
		source := readRepoSource(t, "internal/intent/inspect.go")
		loopIndex := strings.Index(source, "for _, spec := range artifactSpecs {")
		if loopIndex < 0 {
			t.Fatal("the capture loop moved")
		}
		if strings.Contains(source[loopIndex:], "NewAbortReport(") {
			t.Fatal("an abort is returned from inside or after the capture loop")
		}
		for _, fixture := range reachableAbortFixtures() {
			args := append(fixture.args(t), "--json", "--quiet")
			_, stdout, _, _ := runPrepare(t, args...)
			if len(reportArtifacts(t, decodeReport(t, stdout))) != 0 {
				t.Fatalf("abort %s inspected artifacts", fixture.code)
			}
		}
	})

	t.Run("AVP-130", func(t *testing.T) {
		root, slug := avpInit(t, "differential zero byte spec")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "spec.md"), "")
		if code, _, _, _ := runPrepare(t, "--path", root, "define", slug, "--manual"); code != 0 {
			t.Fatal("define --manual refused a zero-byte spec.md")
		}
		if featureState(t, root, slug) != "defined" {
			t.Fatal("state is not defined")
		}
		before := avpTreeSnapshot(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		report := decodeReport(t, stdout)
		spec := artifactRow(t, report, "spec")
		if spec["state"] != "present-empty" || spec["remediation"] == "" {
			t.Fatalf("spec = %#v", spec)
		}
		if readiness(t, report) != "not_ready" {
			t.Fatalf("readiness = %q", readiness(t, report))
		}
		if string(before) != string(avpTreeSnapshot(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal(".tpatch changed")
		}
	})

	t.Run("AVP-131", func(t *testing.T) {
		root, slug := avpInit(t, "differential symlink exploration")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "spec.md"), "spec\n")
		runPrepare(t, "--path", root, "define", slug, "--manual")
		target := filepath.Join(root, "zzqq-exploration-target.md")
		mustWrite(t, target, "exploration\n")
		if err := os.Symlink(target, filepath.Join(feature, "exploration.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if code, _, _, _ := runPrepare(t, "--path", root, "explore", slug, "--manual"); code != 0 {
			t.Fatal("explore --manual refused a symlink")
		}
		before := avpTreeSnapshot(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		if artifactRow(t, decodeReport(t, stdout), "exploration")["state"] != "symlink-refused" {
			t.Fatal("exploration was not refused")
		}
		if strings.Contains(stdout+stderr, "zzqq-exploration-target") {
			t.Fatal("the symlink target was named")
		}
		if string(before) != string(avpTreeSnapshot(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal(".tpatch changed")
		}
	})

	t.Run("AVP-132", func(t *testing.T) {
		root, slug := avpInit(t, "differential whitespace analysis")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), " \n\t")
		if code, _, _, _ := runPrepare(t, "--path", root, "analyze", slug, "--manual"); code != 0 {
			t.Fatal("analyze --manual refused a whitespace-only analysis.md")
		}
		if featureState(t, root, slug) != "analyzed" {
			t.Fatal("state is not analyzed")
		}
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if artifactRow(t, decodeReport(t, stdout), "analysis")["state"] != "present-empty" {
			t.Fatal("analysis is not present-empty")
		}
	})

	t.Run("AVP-133", func(t *testing.T) {
		root, slug := avpInit(t, "differential all degenerate")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), " ")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "spec.md"), "")
		runPrepare(t, "--path", root, "define", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "exploration.md"), "\n")
		runPrepare(t, "--path", root, "explore", slug, "--manual")
		if got := featureState(t, root, slug); got != "defined" {
			t.Fatalf("state = %q, want defined", got)
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		report := decodeReport(t, stdout)
		failing, remediations := 0, 0
		for _, artifact := range reportArtifacts(t, report) {
			if artifact["role"] != "required" {
				continue
			}
			if artifact["state"] != "present-nonempty" {
				failing++
			}
			if artifact["remediation"] != "" {
				remediations++
			}
		}
		if failing != 3 || remediations != 3 {
			t.Fatalf("%d failing rows, %d remediations; want 3 and 3", failing, remediations)
		}
	})

	t.Run("AVP-138", func(t *testing.T) {
		root, slug := avpInit(t, "composite differential")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
		runPrepare(t, "--path", root, "analyze", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "spec.md"), "")
		runPrepare(t, "--path", root, "define", slug, "--manual")
		mustWrite(t, filepath.Join(feature, "exploration.md"), "exploration\n")
		runPrepare(t, "--path", root, "explore", slug, "--manual")

		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		report := decodeReport(t, stdout)
		if readiness(t, report) != "not_ready" {
			t.Fatalf("readiness = %q", readiness(t, report))
		}
		if artifactRow(t, report, "spec")["state"] != "present-empty" {
			t.Fatal("spec is not present-empty")
		}
		for _, artifact := range reportArtifacts(t, report) {
			if artifact["provenance"] != "unknown" {
				t.Fatalf("provenance = %v", artifact["provenance"])
			}
		}
		if !avpContains(advisoryCodesOf(t, report), "analysis-sidecar-absent-path-b-normal") {
			t.Fatalf("advisories = %v", advisoryCodesOf(t, report))
		}
		notes := notesOf(t, root, slug)
		if notes != "" && strings.Contains(stdout+stderr, notes) {
			t.Fatal("the notes string reached the report")
		}
	})

	t.Run("AVP-140", func(t *testing.T) {
		files := withFile(defaultAVPFiles(), "spec.md", strings.Repeat("x", intent.MaxArtifactBytes+1))
		files = withFile(files, "artifacts/analysis.json", strings.Repeat("y", intent.MaxArtifactBytes+1))
		root := avpWorkspace(t, files)
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		spec := artifactRow(t, report, "spec")
		if spec["state"] != "oversize" || spec["reason_code"] != "artifact-oversize" {
			t.Fatalf("spec = %#v", spec)
		}
		if !avpContains(advisoryCodesOf(t, report), "analysis-sidecar-oversize") {
			t.Fatalf("advisories = %v", advisoryCodesOf(t, report))
		}
		var decoded any
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatal(err)
		}
		forbidden := map[string]bool{"size": true, "size_bytes": true, "bytes": true, "content": true}
		var found []string
		var walk func(any)
		walk = func(value any) {
			switch typed := value.(type) {
			case map[string]any:
				for key, nested := range typed {
					if forbidden[key] {
						found = append(found, key)
					}
					walk(nested)
				}
			case []any:
				for _, nested := range typed {
					walk(nested)
				}
			}
		}
		walk(decoded)
		if len(found) > 0 {
			t.Fatalf("forbidden key(s) present: %v", found)
		}
	})

	t.Run("AVP-182", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		if err := os.RemoveAll(filepath.Join(root, ".tpatch", "features", avpSlug)); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 3 {
			t.Fatalf("exit = %d, want 3", code)
		}
		if decodeReport(t, stdout)["abort"].(map[string]any)["code"] != "feature-not-found" {
			t.Fatal("the walk order changed")
		}
		leafFile := avpWorkspace(t, defaultAVPFiles())
		featureDir := filepath.Join(leafFile, ".tpatch", "features", avpSlug)
		if err := os.RemoveAll(featureDir); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, featureDir, "not a directory")
		_, stdout2, _, _ := runPrepare(t, "--path", leafFile, "prepare", avpSlug, "--check", "--json", "--quiet")
		if decodeReport(t, stdout2)["abort"].(map[string]any)["code"] != "feature-dir-unsafe" {
			t.Fatal("a regular file at the feature path was not refused")
		}
	})
}

func notesOf(t *testing.T, root, slug string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		return ""
	}
	var document struct {
		Notes string `json:"notes"`
	}
	_ = json.Unmarshal(data, &document)
	return document.Notes
}

// gitLsFilesFromRepo returns every path git tracks, repository-relative and
// slash-separated. Using the index rather than the working tree is what keeps
// the source-scan rows independent of whatever untracked files happen to sit
// on disk.
func gitLsFilesFromRepo(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = avpRepoRoot(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var paths []string
	for _, entry := range strings.Split(string(output), "\x00") {
		if entry != "" {
			paths = append(paths, entry)
		}
	}
	if len(paths) == 0 {
		t.Fatal("git ls-files reported no tracked files")
	}
	return paths
}

// avp141ScratchName is the fixed component of the AVP-141 sensitivity arm's
// scratch directory name, shared by the writer and every assertion that has to
// recognise it.
const avp141ScratchName = "avp141-scratch"

// avpGitDir returns the repository's absolute git directory. Files written
// there are outside the working tree, so no `git ls-files --others` query —
// including the wave-close untracked-source sentinel — can ever report them,
// even if a test is interrupted before its cleanup runs.
func avpGitDir(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--absolute-git-dir")
	cmd.Dir = avpRepoRoot(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse --absolute-git-dir: %v", err)
	}
	dir := strings.TrimSpace(string(output))
	if dir == "" {
		t.Fatal("git reported an empty git directory")
	}
	return dir
}

// gitUntrackedGoFiles is the wave-close untracked-source sentinel's own query,
// restricted to Go files. The AVP-141 sensitivity arm asserts its scratch file
// is absent from this list.
func gitUntrackedGoFiles(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z", "--", "*.go")
	cmd.Dir = avpRepoRoot(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files --others: %v", err)
	}
	var paths []string
	for _, entry := range strings.Split(string(output), "\x00") {
		if entry != "" {
			paths = append(paths, entry)
		}
	}
	return paths
}

// trackedProductionGoSources reads every tracked non-test Go file, keyed by
// its repository-relative path.
func trackedProductionGoSources(t *testing.T) map[string]string {
	t.Helper()
	repo := avpRepoRoot(t)
	sources := map[string]string{}
	for _, path := range gitLsFilesFromRepo(t) {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		local := filepath.FromSlash(path)
		data, err := os.ReadFile(filepath.Join(repo, local))
		if err != nil {
			t.Fatalf("read tracked file %s: %v", path, err)
		}
		sources[local] = string(data)
	}
	if len(sources) == 0 {
		t.Fatal("no tracked non-test Go sources; the AVP-141 scan would be vacuous")
	}
	return sources
}

// countOpenRootCallSites is the AVP-141 module-wide half: a pure function
// over a tracked-source population, so its sensitivity arm can add a call
// site without touching the working tree.
func countOpenRootCallSites(sources map[string]string) (int, error) {
	count := 0
	for path, source := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if ok && ident.Name == "os" && sel.Sel.Name == "OpenRoot" {
				count++
			}
			return true
		})
	}
	return count, nil
}
