//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7APDryRunContracts(t *testing.T) {
	t.Run("PIB-461", func(t *testing.T) {
		type fixture struct {
			name string
			args func(*testing.T, string, string) []string
		}
		fixtures := []fixture{
			{name: "generate-admissible", args: func(_ *testing.T, _, _ string) []string { return nil }},
			{name: "manual-admissible", args: func(t *testing.T, root, slug string) []string {
				prepareS4WriteReadyBundle(t, root, slug, false)
				return []string{"--manual"}
			}},
			{name: "regenerate-admissible", args: func(t *testing.T, root, slug string) []string {
				prepareS4WriteReadyBundle(t, root, slug, false)
				return []string{"--regenerate", "--allow-heuristic"}
			}},
			{name: "recovery-pending", args: func(t *testing.T, root, slug string) []string {
				lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
				if err := os.MkdirAll(lane, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{bad"), 0o600); err != nil {
					t.Fatal(err)
				}
				return nil
			}},
			{name: "request-unreadable", args: func(t *testing.T, root, slug string) []string {
				if err := os.Remove(filepath.Join(root, ".tpatch", "features", slug, "request.md")); err != nil {
					t.Fatal(err)
				}
				return nil
			}},
			{name: "artifact-empty", args: func(t *testing.T, root, slug string) []string {
				if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", slug, "analysis.md"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
				return nil
			}},
			{name: "incoherent-gap", args: func(t *testing.T, root, slug string) []string {
				feature := filepath.Join(root, ".tpatch", "features", slug)
				if err := os.WriteFile(filepath.Join(feature, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return nil
			}},
			{name: "archive-index-corrupt", args: func(t *testing.T, root, slug string) []string {
				prepareS4WriteReadyBundle(t, root, slug, false)
				archive := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive")
				if err := os.MkdirAll(archive, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(archive, "index.json"), []byte("{bad"), 0o644); err != nil {
					t.Fatal(err)
				}
				return []string{"--regenerate", "--allow-heuristic"}
			}},
			{name: "provider-config", args: func(t *testing.T, root, slug string) []string {
				prepareS4WriteReadyBundle(t, root, slug, false)
				return []string{"--regenerate"}
			}},
		}
		for _, fixture := range fixtures {
			root, slug := prepareS4Workspace(t, "AP dry "+fixture.name)
			args := fixture.args(t, root, slug)
			before := readTree(t, filepath.Join(root, ".tpatch"))
			acquires, providerCalls := 0, 0
			oldAcquire, oldProvider := prepareAcquireAuthority, prepareLoadProvider
			prepareAcquireAuthority = func(string) (*intentlock.WorkspaceAuthority, error) {
				acquires++
				return nil, errors.New("dry-run acquired authority")
			}
			prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
				providerCalls++
				return nil, provider.Config{}
			}
			commandArgs := []string{"--path", root, "prepare", slug}
			commandArgs = append(commandArgs, args...)
			commandArgs = append(commandArgs, "--dry-run", "--json", "--quiet")
			code, stdout, stderr, _ := runPrepare(t, commandArgs...)
			prepareAcquireAuthority, prepareLoadProvider = oldAcquire, oldProvider
			var report preparePublishReport
			s7APDecodeJSONReport(t, stdout, &report)
			if (code == 0 && stderr != "") || (code != 0 && code != 2 && code != 3) ||
				!report.DryRun || report.ExecutionPreflight != "not_evaluated" ||
				acquires != 0 || providerCalls != 0 ||
				!bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatalf("PIB-461 %s = exit:%d stderr:%q report:%+v authority:%d provider:%d",
					fixture.name, code, stderr, report, acquires, providerCalls)
			}
		}

		evaluated := s7APDryRunSectionCodes(t, "evaluated")
		if len(evaluated) == 0 {
			t.Fatal("evaluated dry-run catalog is empty")
		}
		gitBin := t.TempDir()
		gitLog := filepath.Join(t.TempDir(), "git.log")
		script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$S7_AP_DRY_GIT_LOG\"\nexit 99\n"
		if err := os.WriteFile(filepath.Join(gitBin, "git"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("S7_AP_DRY_GIT_LOG", gitLog)

		oldAcquire := prepareAcquireAuthority
		oldLoad := prepareLoadProvider
		oldBeforeLock := beforeLockAcquire
		acquires, providerLoads, lockCalls := 0, 0, 0
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return oldAcquire(path)
		}
		prepareLoadProvider = func(repoStore *store.Store) (provider.Provider, provider.Config) {
			providerLoads++
			return oldLoad(repoStore)
		}
		beforeLockAcquire = func() { lockCalls++ }
		t.Cleanup(func() {
			prepareAcquireAuthority = oldAcquire
			prepareLoadProvider = oldLoad
			beforeLockAcquire = oldBeforeLock
		})

		for _, code := range evaluated {
			if code == "workspace-unsupported-platform" {
				continue
			}
			func() {
				s7APDryRunObservationMode = true
				s7APDryRunAllowExternalFixtureMutation = code == "artifact-unstable"
				s7APDryRunGitBin = gitBin
				defer func() {
					s7APDryRunObservationMode = false
					s7APDryRunAllowExternalFixtureMutation = false
					s7APDryRunGitBin = ""
				}()

				observation := s7APObserveRefusalOnce(t, code)
				if observation.exit != s6ExpectedRefusalExit(code) || observation.code != code {
					t.Fatalf(
						"evaluated dry-run %s = exit:%d code:%q, want exit:%d code:%q",
						code, observation.exit, observation.code, s6ExpectedRefusalExit(code), code,
					)
				}

				root, slug := prepareS4Workspace(t, "S7 AP admissible "+code)
				admissible := s6PrepareObservation(
					t, "--path", root, "prepare", slug, "--json", "--quiet",
				)
				if admissible.exit != 0 || admissible.code != "" {
					t.Fatalf("admissible dry-run paired with %s = %+v", code, admissible)
				}
			}()
		}
		if acquires != 0 || providerLoads != 0 || lockCalls != 0 {
			t.Fatalf(
				"dry-run evaluated matrix reached mutation: authority=%d provider=%d lock=%d",
				acquires, providerLoads, lockCalls,
			)
		}
		if _, err := os.Stat(gitLog); !os.IsNotExist(err) {
			t.Fatalf("dry-run evaluated matrix spawned Git: %v", err)
		}
	})

	t.Run("PIB-462", func(t *testing.T) {
		const sentence = "Plan only. Generation was not attempted and may still fail. Execution preflight was not evaluated: the actual mutation can still refuse on platform, filesystem, Git, lock or recovery grounds."
		root, slug := prepareS4Workspace(t, "AP dry report")
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--dry-run", "--json", "--quiet",
		)
		var report preparePublishReport
		s7APDecodeJSONReport(t, stdout, &report)
		if code != 0 || stderr != "" || report.ExecutionPreflight != "not_evaluated" ||
			report.PlanNote != sentence {
			t.Fatalf("PIB-462 JSON report = exit:%d stderr:%q report:%+v", code, stderr, report)
		}
		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "prepare", slug, "--dry-run",
		)
		if code != 0 || stderr != "" ||
			strings.Count(stdout, sentence) != 1 ||
			!strings.Contains(stdout, "Execution preflight: not_evaluated") {
			t.Fatalf("PIB-462 human report = exit:%d stderr:%q\n%s", code, stderr, stdout)
		}
	})

	t.Run("PIB-463", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AP dry skipped mutation gates")
		oldSupported, oldAcquire := prepareMutationAuthoritySupported, prepareAcquireAuthority
		prepareMutationAuthoritySupported = func() bool { return false }
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			return intentlock.AcquireWithFilesystemClassifier(
				path,
				func(*os.File) (string, bool, error) { return "nfs", true, nil },
			)
		}
		t.Cleanup(func() {
			prepareMutationAuthoritySupported, prepareAcquireAuthority = oldSupported, oldAcquire
		})
		bin := filepath.Join(root, "git-spy")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		logPath := filepath.Join(root, "git.log")
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(
			"#!/bin/sh\nprintf x >> \"$TPATCH_S7_AP_DRY_GIT\"\nexit 88\n",
		), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		t.Setenv("TPATCH_S7_AP_DRY_GIT", logPath)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--dry-run", "--json", "--quiet",
		)
		var report preparePublishReport
		s7APDecodeJSONReport(t, stdout, &report)
		_, gitErr := os.Stat(logPath)
		forbidden := []string{
			"prepare-unsupported-platform",
			"lock-filesystem-unsupported",
			"local-lane-unverifiable",
		}
		if code != 0 || stderr != "" || report.Outcome != "planned" ||
			report.ExecutionPreflight != "not_evaluated" || !os.IsNotExist(gitErr) {
			t.Fatalf("PIB-463 skipped gates = exit:%d stderr:%q report:%+v git:%v",
				code, stderr, report, gitErr)
		}
		for _, code := range forbidden {
			if strings.Contains(stdout, code) {
				t.Fatalf("PIB-463 dry-run emitted non-evaluated refusal %q: %s", code, stdout)
			}
		}
	})

	t.Run("PIB-464", func(t *testing.T) {
		prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
		baseline, err := s6ContractBaseline(t)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7APDryRunPartition(prd, baseline.refusalCatalog); err != nil {
			t.Fatal(err)
		}
		added := strings.Replace(
			prd,
			"`slug-unsafe`, `workspace-not-initialized`",
			"`ap-injected-refusal`, `slug-unsafe`, `workspace-not-initialized`",
			1,
		)
		if err := validateS7APDryRunPartition(added, baseline.refusalCatalog); err == nil {
			t.Fatal("PIB-464 same validator accepted an added dry-run class")
		}
		duplicated := strings.Replace(
			prd,
			"`prepare-unsupported-platform`, `lock-filesystem-unsupported`",
			"`slug-unsafe`, `prepare-unsupported-platform`, `lock-filesystem-unsupported`",
			1,
		)
		if err := validateS7APDryRunPartition(duplicated, baseline.refusalCatalog); err == nil {
			t.Fatal("PIB-464 same validator accepted a duplicate partition member")
		}
	})
}

func s7APDryRunSectionCodes(t *testing.T, population string) []string {
	t.Helper()
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	start := strings.Index(prd, "**What dry-run reproduces, and what it deliberately does not evaluate.**")
	end := strings.Index(prd[start:], "The redaction scan is deliberately")
	if start < 0 || end < 0 {
		t.Fatal("dry-run partition section is missing")
	}
	column := 1
	if population == "not-evaluated" {
		column = 2
	} else if population != "evaluated" {
		t.Fatalf("unknown dry-run population %q", population)
	}
	var codes []string
	for _, line := range strings.Split(prd[start:start+end], "\n") {
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") ||
			strings.Contains(line, "Reproduced by") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) >= 4 {
			codes = append(codes, s7APBacktickValues(cells[column])...)
		}
	}
	return codes
}

func validateS7APDryRunPartition(prd string, productionCatalog []string) error {
	start := strings.Index(prd, "**What dry-run reproduces, and what it deliberately does not evaluate.**")
	end := strings.Index(prd[start:], "The redaction scan is deliberately")
	if start < 0 || end < 0 {
		return errors.New("dry-run partition section is missing")
	}
	section := prd[start : start+end]
	evaluated := map[string]bool{}
	notEvaluated := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") ||
			strings.Contains(line, "Reproduced by") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		for index, destination := range []map[string]bool{evaluated, notEvaluated} {
			for _, code := range s7APBacktickValues(cells[index+1]) {
				if evaluated[code] || notEvaluated[code] {
					return fmt.Errorf("dry-run refusal %q appears more than once", code)
				}
				destination[code] = true
			}
		}
	}
	if len(evaluated) == 0 || len(notEvaluated) == 0 {
		return errors.New("dry-run partition has an empty side")
	}
	union := map[string]bool{}
	for code := range evaluated {
		union[code] = true
	}
	for code := range notEvaluated {
		union[code] = true
	}
	want := append([]string(nil), productionCatalog...)
	sort.Strings(want)
	got := make([]string, 0, len(union))
	for code := range union {
		got = append(got, code)
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		return fmt.Errorf("dry-run partition/catalog mismatch:\ngot  %v\nwant %v", got, want)
	}
	return nil
}

func s7APBacktickValues(cell string) []string {
	var values []string
	for {
		start := strings.IndexByte(cell, '`')
		if start < 0 {
			break
		}
		cell = cell[start+1:]
		end := strings.IndexByte(cell, '`')
		if end < 0 {
			break
		}
		value := cell[:end]
		cell = cell[end+1:]
		if strings.Contains(value, "-") && !strings.ContainsAny(value, " <>[]") {
			values = append(values, value)
		}
	}
	return values
}
