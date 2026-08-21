//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func TestS7PIB435NestedRecoveryPublicationAndPurgeAcquireOnce(t *testing.T) {
	t.Run("publication", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 PIB 435 publication")
		acquires := s7CountPrepareAuthorityAcquisitions(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--allow-heuristic", "--json", "--quiet",
		)
		if code != 0 || stderr != "" || prepareS4Report(t, stdout).Outcome != "published" ||
			*acquires != 1 {
			t.Fatalf("publication = exit:%d stderr:%q acquires:%d\n%s", code, stderr, *acquires, stdout)
		}
	})

	t.Run("recovery", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 PIB 435 recovery")
		prepareS5InterruptAfterJournal(t, root, slug)
		acquires := s7CountPrepareAuthorityAcquisitions(t)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		if code != 0 || stderr != "" || prepareS4Report(t, stdout).Outcome != "recovered" ||
			*acquires != 1 {
			t.Fatalf("recovery = exit:%d stderr:%q acquires:%d\n%s", code, stderr, *acquires, stdout)
		}
	})

	t.Run("purge", func(t *testing.T) {
		root, slug := intentArchiveCLIWorkspace(t)
		body := []byte("purge under one holder\n")
		replacement := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireRetained,
		)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
			map[string][]byte{replacement.ContentSHA256: body},
		)
		oldAcquire := intentArchiveAcquireAuthority
		acquires := 0
		intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return oldAcquire(path)
		}
		t.Cleanup(func() { intentArchiveAcquireAuthority = oldAcquire })
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--blob", replacement.ContentSHA256, "--yes", "--json", "--quiet",
		)
		report := decodeIntentArchivePurgeReport(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "purged" || acquires != 1 {
			t.Fatalf("purge = exit:%d stderr:%q acquires:%d report:%+v", code, stderr, acquires, report)
		}
	})
}

func TestS7PIB436RealRootRenameBeforeAndAfterPublicationBoundary(t *testing.T) {
	before := s7ObservePrepareRootReplacement(t, false)
	after := s7ObservePrepareRootReplacement(t, true)
	if before.code != 5 || before.refusalCode != "workspace-root-changed" ||
		!before.replacementPreserved ||
		after.code != 6 || after.refusalCode != "workspace-root-replaced-after-publication" ||
		!after.replacementPreserved || !after.evidencePreserved {
		t.Fatalf("root rename boundary: before=%+v after=%+v", before, after)
	}
}

func TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects(t *testing.T) {
	for _, test := range []struct {
		name     string
		mode     []string
		mutate   func(*testing.T, string, string)
		wantExit int
		wantCode string
	}{
		{name: "generate", wantExit: 0},
		{name: "manual", mode: []string{"--manual"}, wantExit: 0},
		{name: "regenerate", mode: []string{"--regenerate", "--allow-heuristic"}, wantExit: 0},
		{name: "missing-feature", wantExit: 3, wantCode: "feature-not-found"},
		{
			name: "malformed-status", wantExit: 3, wantCode: "status-malformed",
			mutate: func(t *testing.T, root, slug string) {
				t.Helper()
				if err := os.WriteFile(
					filepath.Join(root, ".tpatch", "features", slug, "status.json"),
					[]byte("{bad"), 0o644,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "pending-journal-not-evaluated", wantExit: 3,
			mutate: func(t *testing.T, root, slug string) {
				t.Helper()
				lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
				if err := os.MkdirAll(lane, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{bad"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, slug := prepareS4Workspace(t, "S7 PIB 440 "+test.name)
			if test.name == "missing-feature" {
				slug = "missing-feature"
			}
			if test.name == "manual" || test.name == "regenerate" {
				prepareS4WriteReadyBundle(t, root, slug, false)
			}
			if test.mutate != nil {
				test.mutate(t, root, slug)
			}
			gitBin := filepath.Join(root, "s7-dry-run-git-bin")
			gitLog := filepath.Join(root, "s7-dry-run-git.log")
			if err := os.MkdirAll(gitBin, 0o755); err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$S7_DRY_RUN_GIT_LOG\"\nexit 99\n"
			if err := os.WriteFile(filepath.Join(gitBin, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", gitBin)
			t.Setenv("S7_DRY_RUN_GIT_LOG", gitLog)
			before := readTree(t, root)
			oldAcquire := prepareAcquireAuthority
			oldLoad := prepareLoadProvider
			oldBeforeLock := beforeLockAcquire
			acquires, providerLoads, lockCalls := 0, 0, 0
			prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
				acquires++
				return oldAcquire(path)
			}
			prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
				providerLoads++
				return nil, provider.Config{}
			}
			beforeLockAcquire = func() { lockCalls++ }
			t.Cleanup(func() {
				prepareAcquireAuthority = oldAcquire
				prepareLoadProvider = oldLoad
				beforeLockAcquire = oldBeforeLock
			})
			args := []string{"--path", root, "prepare", slug}
			args = append(args, test.mode...)
			args = append(args, "--dry-run", "--json", "--quiet")
			code, stdout, _, _ := runPrepare(t, args...)
			report := prepareS4Report(t, stdout)
			if code != test.wantExit || acquires != 0 || providerLoads != 0 || lockCalls != 0 {
				t.Fatalf(
					"dry-run %s = exit:%d want:%d authority:%d provider:%d lock:%d report:%+v",
					test.name, code, test.wantExit, acquires, providerLoads, lockCalls, report,
				)
			}
			if test.wantCode != "" && (report.Refusal == nil || report.Refusal.Code != test.wantCode) {
				t.Fatalf("dry-run %s refusal = %+v", test.name, report.Refusal)
			}
			if report.ExecutionPreflight != "not_evaluated" ||
				!bytes.Equal(before, readTree(t, root)) {
				t.Fatalf("dry-run %s evaluated execution or changed filesystem: %+v", test.name, report)
			}
			if _, err := os.Stat(gitLog); !os.IsNotExist(err) {
				t.Fatalf("dry-run %s spawned Git: %v", test.name, err)
			}
		})
	}
}

func TestS7PIB439GitProcessTableUsesExactArgvAndPinnedEnvironment(t *testing.T) {
	authoritativeScrub := s7PIB439AuthoritativeGitScrubExamples(t)
	runtimeScrub := append(
		append([]string(nil), authoritativeScrub...),
		s7PIB439OverflowIndexedNames()...,
	)
	for _, test := range []struct {
		name      string
		worktree  bool
		args      []string
		wantArgv  []string
		ready     bool
		wantState string
	}{
		{
			name: "worktree-non-regenerate", worktree: true, ready: true,
			args: []string{"--manual"},
			wantArgv: []string{
				"rev-parse --is-inside-work-tree",
				"check-ignore -q --no-index -- LANE",
				"--literal-pathspecs ls-files -- .tpatch/local/",
			},
			wantState: "true",
		},
		{
			name: "worktree-regenerate", worktree: true, ready: true,
			args: []string{"--regenerate"},
			wantArgv: []string{
				"rev-parse --is-inside-work-tree",
				"check-ignore -q --no-index -- LANE",
				"--literal-pathspecs ls-files -- .tpatch/local/",
				"ls-files -- .tpatch",
			},
			wantState: "true",
		},
		{
			name: "established-non-worktree",
			wantArgv: []string{
				"rev-parse --is-inside-work-tree",
			},
			wantState: "false",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var root, slug string
			if test.worktree {
				root, slug = prepareS4Workspace(t, "S7 PIB 439 "+test.name)
			} else {
				root, slug = intentArchiveCLIWorkspace(t)
			}
			if test.ready {
				prepareS4WriteReadyBundle(t, root, slug, false)
			}
			bin := filepath.Join(root, "s7-git-spy-bin")
			logPath := filepath.Join(root, "s7-git-spy.log")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\n" +
				"printf '%s\\n' '---ENV---' >> \"$S7_GIT_LOG\"\n" +
				"env | LC_ALL=C sort >> \"$S7_GIT_LOG\"\n" +
				"printf '%s%s\\n' '---ARGV---' \"$*\" >> \"$S7_GIT_LOG\"\n" +
				"if [ \"$1\" = \"rev-parse\" ]; then printf '%s\\n' \"$S7_GIT_STATE\"; fi\n" +
				"exit 0\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("S7_GIT_LOG", logPath)
			t.Setenv("S7_GIT_STATE", test.wantState)
			for _, name := range runtimeScrub {
				t.Setenv(name, "S7-FOREIGN-"+name)
			}
			args := []string{"--path", root, "prepare", slug}
			args = append(args, test.args...)
			args = append(args, "--json", "--quiet")
			_, _, _, _ = runPrepare(t, args...)
			raw, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			observations, err := parseS7PIB439GitLog(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateS7PIB439GitObservations(observations, test.wantArgv, root, slug); err != nil {
				t.Fatal(err)
			}
			for _, name := range authoritativeScrub {
				wrong := cloneS7PIB439GitObservations(observations)
				wrong[0].environment[name] = "selectively-reintroduced"
				if err := validateS7PIB439GitObservations(wrong, test.wantArgv, root, slug); err == nil {
					t.Fatalf("Git process-table validator accepted reintroduced %s", name)
				}
			}
			for _, name := range s7PIB439OverflowIndexedNames() {
				for processIndex := range observations {
					wrong := cloneS7PIB439GitObservations(observations)
					wrong[processIndex].environment[name] = "overflow-index-reintroduced"
					if err := validateS7PIB439GitObservations(
						wrong, test.wantArgv, root, slug,
					); err == nil {
						t.Fatalf(
							"Git process-table validator accepted overflow %s in process %d",
							name,
							processIndex+1,
						)
					}
				}
			}
		})
	}
}

type s7PIB439GitObservation struct {
	environment map[string]string
	argv        string
}

func s7PIB439GitScrubExamples() []string {
	return []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_COMMON_DIR",
		"GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_PREFIX",
		"GIT_IMPLICIT_WORK_TREE",
		"GIT_SUPER_PREFIX",
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0",
		"GIT_CONFIG_KEY_999",
		"GIT_CONFIG_VALUE_999",
	}
}

func s7PIB439OverflowIndexedNames() []string {
	suffix := strings.Repeat("9", 96)
	return []string{
		"GIT_CONFIG_KEY_" + suffix,
		"GIT_CONFIG_VALUE_" + suffix,
	}
}

func s7PIB439AuthoritativeGitScrubExamples(t *testing.T) []string {
	t.Helper()
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	start := strings.Index(prd, "#### The pinned environment scrub")
	end := strings.Index(prd, "**Global and system ignore configuration remains intentionally available.**")
	if start < 0 || end <= start {
		t.Fatal("PRD §7.13 pinned environment subsection is missing")
	}
	names := map[string]bool{}
	pattern := regexp.MustCompile("`(GIT_[A-Z_]+(?:<n>)?)`")
	for _, match := range pattern.FindAllStringSubmatch(prd[start:end], -1) {
		switch match[1] {
		case "GIT_CONFIG_KEY_<n>":
			names["GIT_CONFIG_KEY_0"] = true
			names["GIT_CONFIG_KEY_999"] = true
		case "GIT_CONFIG_VALUE_<n>":
			names["GIT_CONFIG_VALUE_0"] = true
			names["GIT_CONFIG_VALUE_999"] = true
		default:
			names[match[1]] = true
		}
	}
	derived := make([]string, 0, len(names))
	for name := range names {
		derived = append(derived, name)
	}
	sort.Strings(derived)
	expected := s7PIB439GitScrubExamples()
	sort.Strings(expected)
	if !s7PIB424EqualStrings(derived, expected) {
		t.Fatalf("derived §7.13 Git scrub = %q, want %q", derived, expected)
	}
	return derived
}

func parseS7PIB439GitLog(raw []byte) ([]s7PIB439GitObservation, error) {
	var observations []s7PIB439GitObservation
	for _, block := range strings.Split(string(raw), "---ENV---\n")[1:] {
		parts := strings.SplitN(block, "---ARGV---", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Git spy block lacks argv delimiter: %q", block)
		}
		environment := map[string]string{}
		for _, line := range strings.Split(strings.TrimSuffix(parts[0], "\n"), "\n") {
			name, value, ok := strings.Cut(line, "=")
			if ok {
				environment[name] = value
			}
		}
		argv := strings.TrimSuffix(parts[1], "\n")
		observations = append(observations, s7PIB439GitObservation{
			environment: environment,
			argv:        argv,
		})
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("Git spy recorded no process environments")
	}
	return observations, nil
}

func validateS7PIB439GitObservations(
	observations []s7PIB439GitObservation,
	wantArgv []string,
	root, slug string,
) error {
	if len(observations) != len(wantArgv) {
		return fmt.Errorf("Git process count = %d, want %d", len(observations), len(wantArgv))
	}
	for index, observation := range observations {
		if observation.environment["LC_ALL"] != "C" || observation.environment["LANG"] != "C" {
			return fmt.Errorf("Git[%d] locale = LC_ALL:%q LANG:%q",
				index, observation.environment["LC_ALL"], observation.environment["LANG"])
		}
		for name := range observation.environment {
			if s7PIB439GitEnvironmentForbidden(name) {
				return fmt.Errorf("Git[%d] retained forbidden environment %s", index, name)
			}
		}
		argv := strings.ReplaceAll(
			observation.argv, ".tpatch/local/intent-prepare/"+slug, "LANE",
		)
		if argv != wantArgv[index] || strings.Contains(argv, root) {
			return fmt.Errorf("Git[%d] argv = %q, want %q", index, argv, wantArgv[index])
		}
	}
	return nil
}

func s7PIB439GitEnvironmentForbidden(name string) bool {
	for _, exact := range s7PIB439GitScrubExamples()[:13] {
		if name == exact {
			return true
		}
	}
	for _, prefix := range []string{"GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if suffix == "" {
			return false
		}
		for _, character := range suffix {
			if character < '0' || character > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func cloneS7PIB439GitObservations(
	observations []s7PIB439GitObservation,
) []s7PIB439GitObservation {
	clone := make([]s7PIB439GitObservation, len(observations))
	for index, observation := range observations {
		environment := make(map[string]string, len(observation.environment))
		for name, value := range observation.environment {
			environment[name] = value
		}
		clone[index] = s7PIB439GitObservation{
			environment: environment,
			argv:        observation.argv,
		}
	}
	return clone
}

func TestS7PIB445LiveMutatingPrepareIsUnchangedByD9(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 PIB 445 live D9")
	entered := make(chan struct{})
	release := make(chan struct{})
	oldHook := beforePrepareSetRevalidation
	beforePrepareSetRevalidation = func() {
		close(entered)
		<-release
	}
	t.Cleanup(func() { beforePrepareSetRevalidation = oldHook })
	type result struct {
		code   int
		stdout string
		stderr string
	}
	done := make(chan result, 1)
	go func() {
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		done <- result{code: code, stdout: stdout, stderr: stderr}
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("mutating prepare did not reach held-authority publication")
	}
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	before := readTree(t, root)
	report, err := workflow.RunDoctor(
		&store.Store{Root: root}, workflow.DoctorOptions{Checks: []string{"D9"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, readTree(t, root)) {
		t.Fatal("D9 mutated the live prepare workspace")
	}
	for _, forbidden := range []string{"authority held", "authority is free", "holder identity", "process id"} {
		if strings.Contains(strings.ToLower(string(rendered)), forbidden) {
			t.Fatalf("D9 made live-authority claim %q: %s", forbidden, rendered)
		}
	}
	close(release)
	released = true
	select {
	case got := <-done:
		published := prepareS4Report(t, got.stdout)
		if got.code != 0 || got.stderr != "" ||
			published.Outcome != "published" || published.Action != "complete" {
			t.Fatalf("D9 perturbed live prepare: %+v report:%+v", got, published)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("live prepare did not complete after D9")
	}
}

func s7CountPrepareAuthorityAcquisitions(t *testing.T) *int {
	t.Helper()
	oldAcquire := prepareAcquireAuthority
	count := new(int)
	prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		*count++
		return oldAcquire(path)
	}
	t.Cleanup(func() { prepareAcquireAuthority = oldAcquire })
	return count
}
