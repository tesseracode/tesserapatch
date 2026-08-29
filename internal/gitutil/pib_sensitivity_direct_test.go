//go:build (linux && !android) || (darwin && !ios)

package gitutil

// Direct per-row §18.53 sensitivity fixtures for PIB-326 and PIB-473.
//
// `TestS7APGitContracts/PIB-473` and `TestS7APGitContracts/PIB-476` are already
// the wrong-input fixtures of the rows that own them, and neither row's subject
// is PIB-326's or PIB-473's. Each body below drives the row's *own* validator
// over the shipped source and then over a mutation of the input that validator
// governs: the closed G1–G4 argv allowlist for PIB-326, and the compatibility
// wrappers' inherited environment, argv, exit code and output for PIB-473.

import (
	"strings"
	"testing"
)

// TestPIBSensitivityPIB326ClosedGitArgvAllowlist owns PIB-326: the prepare Git
// seam may spawn only G1–G4. Adding `git status` — or any argv outside the four
// — to a fixture source must fail the same allowlist/count guard that accepts
// the shipped source.
func TestPIBSensitivityPIB326ClosedGitArgvAllowlist(t *testing.T) {
	gitSource := s7GitutilSource(t, "ignore.go")
	cliSource := s7RepositorySource(t, "internal/cli/prepare_publish.go")
	if err := validateS7PIB438GitSources(gitSource, cliSource); err != nil {
		t.Fatalf("PIB-326: the shipped closed Git argv allowlist failed its own guard: %v", err)
	}

	statusArgv := strings.Replace(gitSource,
		`[]string{"ls-files", "--", ".tpatch"}`,
		`[]string{"status", "--short"}`, 1)
	if statusArgv == gitSource {
		t.Fatal("PIB-326: the G4 argv anchor is missing from the shipped executor")
	}
	if err := validateS7PIB438GitSources(statusArgv, cliSource); err == nil {
		t.Fatal("PIB-326: the allowlist accepted a `git status` argv outside G1–G4")
	}

	widenedG2 := strings.Replace(gitSource,
		`[]string{"check-ignore", "-q", "--no-index", "--", disarmLeadingColon(repoRelative)}`,
		`[]string{"check-ignore", "-q", "--no-index", "-v", "--", disarmLeadingColon(repoRelative)}`, 1)
	if widenedG2 == gitSource {
		t.Fatal("PIB-326: the G2 argv anchor is missing from the shipped executor")
	}
	if err := validateS7PIB438GitSources(widenedG2, cliSource); err == nil {
		t.Fatal("PIB-326: the allowlist accepted a widened G2 argv")
	}

	droppedProbe := strings.Replace(cliSource,
		"gitutil.IsTpatchTrackedWithState(",
		"gitutilRemovedProbe(", 1)
	if droppedProbe == cliSource {
		t.Fatal("PIB-326: prepare no longer calls the G4 helper the count guard bounds")
	}
	if err := validateS7PIB438GitSources(gitSource, droppedProbe); err == nil {
		t.Fatal("PIB-326: the count guard accepted a prepare that stops routing G4 through the seam")
	}
}

// TestPIBSensitivityPIB473CompatibilityWrapperEnvelope owns PIB-473: every
// legacy caller of the ignore/tracked helpers keeps its environment, argv shape,
// exit-code interpretation and output through its explicit compatibility
// wrapper. A wrapper that rewrites any of them must fail the same validator that
// accepts the shipped wrappers.
func TestPIBSensitivityPIB473CompatibilityWrapperEnvelope(t *testing.T) {
	gitSource := s7GitutilSource(t, "ignore.go")
	rescapSource := s7RepositorySource(t, "internal/rescap/gitgate.go")
	if err := validateS7APCompatibilityCallGraph(gitSource, rescapSource); err != nil {
		t.Fatalf("PIB-473: the shipped compatibility wrappers failed their own guard: %v", err)
	}

	rewrittenEnv := strings.Replace(gitSource,
		"repoRoot:      repoRoot,\n\t\targs:          append([]string(nil), args...),",
		"repoRoot:      repoRoot,\n\t\targs:          append([]string(nil), args...),\n\t\tenv:           append(os.Environ(), \"GIT_PAGER=cat\"),",
		1)
	if rewrittenEnv == gitSource {
		t.Fatal("PIB-473: the compatibility request anchor is missing from the shipped wrapper")
	}
	if err := validateS7APCompatibilityCallGraph(rewrittenEnv, rescapSource); err == nil {
		t.Fatal("PIB-473: the guard accepted a compatibility wrapper that rewrites its caller's environment")
	}

	rewrittenOutput := strings.Replace(gitSource,
		"return result.stdout, result.stderr, result.exitCode, result.err",
		"return \"\", result.stderr, 0, result.err",
		1)
	if rewrittenOutput == gitSource {
		t.Fatal("PIB-473: the compatibility return anchor is missing from the shipped wrapper")
	}
	if err := validateS7APCompatibilityCallGraph(rewrittenOutput, rescapSource); err == nil {
		t.Fatal("PIB-473: the guard accepted a compatibility wrapper that rewrites output and exit code")
	}

	bypassedWrapper := strings.Replace(rescapSource,
		"gitutil.RunGitCompatibility(",
		"gitutil.RunGitCompatibilityBypassed(",
		1)
	if bypassedWrapper == rescapSource {
		t.Fatal("PIB-473: no legacy caller reaches the compatibility wrapper in the shipped source")
	}
	if err := validateS7APCompatibilityCallGraph(gitSource, bypassedWrapper); err == nil {
		t.Fatal("PIB-473: the guard accepted a legacy caller that bypasses its compatibility wrapper")
	}
}
