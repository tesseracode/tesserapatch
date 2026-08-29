//go:build (linux && !android) || (darwin && !ios)

package cli

// Literal owning acceptance subtests for the aggregate-ledger rows whose only
// prior evidence was a subtest whose name is produced at run time.
//
// `TestAVPGrammarAndSurface` registers its `--manual`/`--regenerate` pair with
// `t.Run(flag.id, …)` over a table keyed on `id`, `TestAVPReadinessAndOutput`
// registers `AVP-052`'s children with `t.Run(tc.name, …)` over a positional
// table, and `TestS6ArchiveRefusalMappings`/`TestS6IntentpubRefusalMappings`
// register theirs with `t.Run(code, …)` where `code` is a loop-local shadow.
// None of those labels is a literal the sources state, so no static acceptance
// identity can name them.
//
// Every leaf below is a literal `t.Run("PIB-NNN", …)` that performs the row's
// own setup, drives the real CLI through the real root error printer, and
// asserts the exact observable §18's matrix line names for it.

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ---------------------------------------------------------------------------
// §18.2 A — `--check` is mutually exclusive with every mutating mode flag.
// ---------------------------------------------------------------------------

func TestPIBRowPrepareCheckModeFlagMutex(t *testing.T) {
	t.Run("PIB-005", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 005")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		spy := pibRowInstallWriteSpy(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--manual",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --manual = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if len(spy.opens)+len(spy.renames) != 0 {
			t.Fatalf("PIB-005: the rejected combination wrote %v / %v", spy.opens, spy.renames)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-005: the rejected combination changed the .tpatch tree")
		}
	})

	t.Run("PIB-006", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 006")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		spy := pibRowInstallWriteSpy(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--regenerate",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --regenerate = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if len(spy.opens)+len(spy.renames) != 0 {
			t.Fatalf("PIB-006: the rejected combination wrote %v / %v", spy.opens, spy.renames)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-006: the rejected combination changed the .tpatch tree")
		}
	})

	t.Run("PIB-011", func(t *testing.T) {
		// The zero-mutation restatement freezes a *populated* workspace, so a
		// write anywhere under `.tpatch/` — not only in the feature lane — bites.
		root, slug := prepareS4Workspace(t, "PIB row 011")
		prepareS4WriteReadyBundle(t, root, slug, true)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--manual",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --manual = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-011: the rejected combination changed the .tpatch tree")
		}
	})

	t.Run("PIB-012", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 012")
		prepareS4WriteReadyBundle(t, root, slug, true)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--regenerate",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --regenerate = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-012: the rejected combination changed the .tpatch tree")
		}
	})
}

// ---------------------------------------------------------------------------
// §18.14 N — refusal remediation vocabulary.
// ---------------------------------------------------------------------------

var (
	pibRowFlagPattern    = regexp.MustCompile("(?:^|[\\s`\"'(\\[])(--[a-z][a-z0-9-]*)")
	pibRowCommandPattern = regexp.MustCompile("`(tpatch[^`]*)`")
	pibRowAbsolutePath   = regexp.MustCompile("(?:^|[\\s`\"'(])(/[A-Za-z0-9_][A-Za-z0-9_./-]*)")
)

// pibRowShippedCLISurface derives the shipped command paths and the shipped
// flag names from the *real* cobra tree, so the vocabulary a remediation is
// checked against is the one the binary actually registers.
func pibRowShippedCLISurface(t *testing.T) (map[string]bool, map[string]bool) {
	t.Helper()
	paths := map[string]bool{}
	flags := map[string]bool{}
	var walk func(command *cobra.Command, prefix []string)
	walk = func(command *cobra.Command, prefix []string) {
		command.Flags().VisitAll(func(flag *pflag.Flag) { flags["--"+flag.Name] = true })
		command.PersistentFlags().VisitAll(func(flag *pflag.Flag) { flags["--"+flag.Name] = true })
		for _, child := range command.Commands() {
			next := append(append([]string{}, prefix...), child.Name())
			paths[strings.Join(next, " ")] = true
			walk(child, next)
		}
	}
	walk(buildRootCmd(), nil)
	return paths, flags
}

func pibRowTokens(pattern *regexp.Regexp, text string) []string {
	var out []string
	for _, match := range pattern.FindAllStringSubmatch(text, -1) {
		out = append(out, match[1])
	}
	return out
}

// pibRowNamesShippedCommand reports whether a `tpatch …` phrase begins with a
// command path the real tree registers.
func pibRowNamesShippedCommand(paths map[string]bool, command string) bool {
	fields := strings.Fields(command)
	if len(fields) < 2 || fields[0] != "tpatch" {
		return false
	}
	for depth := len(fields) - 1; depth >= 1; depth-- {
		if paths[strings.Join(fields[1:depth+1], " ")] {
			return true
		}
	}
	return false
}

// TestPIBRowPIB179RefusalRemediationNamesShippedSurface owns PIB-179: for every
// code in the closed §10.4 refusal catalog the emitted remediation must be
// non-empty and may only name vocabulary the shipped CLI registers — no
// unregistered flag, no unshipped command and no absolute filesystem path.
func TestPIBRowPIB179RefusalRemediationNamesShippedSurface(t *testing.T) {
	paths, flags := pibRowShippedCLISurface(t)
	if len(paths) == 0 || len(flags) == 0 {
		t.Fatal("PIB-179: no command path or flag was derived from the real cobra tree")
	}
	if !paths["prepare"] || !flags["--json"] {
		t.Fatalf("PIB-179: the derived surface is not the shipped one: %d paths, %d flags",
			len(paths), len(flags))
	}
	catalog, evidence, err := s6RefusalEvidence(t)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) == 0 {
		t.Fatal("PIB-179: the refusal catalog is empty, so the row proves nothing")
	}
	checked := 0
	for _, code := range catalog {
		remediation := evidence[code].remediation
		if strings.TrimSpace(remediation) == "" {
			t.Fatalf("PIB-179: refusal %s carries no remediation", code)
		}
		for _, flag := range pibRowTokens(pibRowFlagPattern, remediation) {
			if !flags[flag] {
				t.Fatalf("PIB-179: refusal %s remediation names unregistered flag %s: %q",
					code, flag, remediation)
			}
		}
		for _, command := range pibRowTokens(pibRowCommandPattern, remediation) {
			if !pibRowNamesShippedCommand(paths, command) {
				t.Fatalf("PIB-179: refusal %s remediation names unshipped command %q",
					code, command)
			}
		}
		for _, path := range pibRowTokens(pibRowAbsolutePath, remediation) {
			t.Fatalf("PIB-179: refusal %s remediation names absolute path %q", code, path)
		}
		checked++
	}
	if checked != len(catalog) {
		t.Fatalf("PIB-179: %d of %d refusal remediations were checked", checked, len(catalog))
	}
}

// ---------------------------------------------------------------------------
// §18.20 T — a present-empty optional sidecar is preserved, not refused.
// ---------------------------------------------------------------------------

// TestPIBRowPIB252DefaultModePreservesPresentEmptySidecar owns PIB-252: with
// `analysis.md` preserved and the optional sidecar present but zero bytes, the
// default mode completes the missing artifacts, causes no refusal, and leaves
// the sidecar byte-identical at zero bytes.
func TestPIBRowPIB252DefaultModePreservesPresentEmptySidecar(t *testing.T) {
	root, slug := prepareS4Workspace(t, "PIB row 252")
	feature := pibRowFeature(root, slug)
	if err := os.WriteFile(
		filepath.Join(feature, "analysis.md"), []byte("preserved analysis\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(feature, "artifacts", "analysis.json")
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sidecarBefore := pibRowFileIdentity(t, sidecar)

	code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-252: present-empty sidecar run = exit %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := prepareS4Report(t, stdout)
	if report.Refusal != nil {
		t.Fatalf("PIB-252: the present-empty sidecar caused refusal %#v", report.Refusal)
	}
	if report.Action != "complete" && report.Action != "none" {
		t.Fatalf("PIB-252: action = %q, want complete or the Markdown no-op", report.Action)
	}
	info, err := os.Stat(sidecar)
	if err != nil {
		t.Fatalf("PIB-252: the sidecar disappeared: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("PIB-252: the preserved sidecar is %d bytes, want 0", info.Size())
	}
	if got := pibRowFileIdentity(t, sidecar); got != sidecarBefore {
		t.Fatalf("PIB-252: the preserved sidecar changed\n got %s\nwant %s", got, sidecarBefore)
	}
	analysis, err := os.ReadFile(filepath.Join(feature, "analysis.md"))
	if err != nil || string(analysis) != "preserved analysis\n" {
		t.Fatalf("PIB-252: analysis.md was not preserved: %q (%v)", analysis, err)
	}
}
