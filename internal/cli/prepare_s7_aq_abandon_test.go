//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7AQAbandonContracts(t *testing.T) {
	t.Run("PIB-492", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		for _, gitMode := range []string{"absent", "unusable"} {
			t.Setenv("PATH", originalPath)
			root, slug := prepareS4Workspace(t, "AQ abandon git "+gitMode)
			s6WriteJournalFixture(t, root, slug, "journal-corrupt")
			bin := filepath.Join(root, "git-spy")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			trace := filepath.Join(root, "git-spawns.log")
			if gitMode == "unusable" {
				script := "#!/bin/sh\nprintf 'spawned\\n' >> \"$TPATCH_S7_AQ_GIT_TRACE\"\nexit 88\n"
				if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			t.Setenv("TPATCH_S7_AQ_GIT_TRACE", trace)
			code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "prepare", slug,
				"--abandon-transaction", "--yes", "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			_, traceErr := os.Stat(trace)
			if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
				report.Abandoned == nil || len(report.Abandoned.Moved) == 0 ||
				report.Refusal != nil ||
				strings.Contains(stdout, "local-lane-unverifiable") ||
				!os.IsNotExist(traceErr) {
				t.Fatalf("PIB-492 %s = exit:%d stderr:%q report:%+v trace:%v",
					gitMode, code, stderr, report, traceErr)
			}
			movedJournal := filepath.Join(
				root,
				filepath.FromSlash(strings.TrimSuffix(report.Abandoned.Directory, "/")),
				"journal.json",
			)
			if _, err := os.Stat(movedJournal); err != nil {
				t.Fatalf("PIB-492 %s did not move corrupt evidence: %v", gitMode, err)
			}
		}
	})

	t.Run("PIB-493", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AQ abandon unignored lane")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		laneRel := ".tpatch/local/intent-prepare/" + slug + "/journal.json"
		beforeIgnored := s7AQGitCheckIgnored(t, root, laneRel)
		statusPath := filepath.Join(root, ".tpatch", "features", slug, "status.json")
		statusBefore, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		statusAfter, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		afterIgnored := s7AQGitCheckIgnored(t, root, laneRel)
		if beforeIgnored || afterIgnored || code != 0 || stderr != "" ||
			report.Outcome != "abandoned" || report.Abandoned == nil ||
			strings.Contains(stdout, "local-lane-not-ignored") ||
			!bytes.Equal(statusBefore, statusAfter) {
			t.Fatalf("PIB-493 unignored abandon = before:%t after:%t exit:%d stderr:%q report:%+v statusChanged:%t",
				beforeIgnored, afterIgnored, code, stderr, report,
				!bytes.Equal(statusBefore, statusAfter))
		}
	})

	t.Run("PIB-494", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AQ abandon ordering")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		s7AQSeedPendingArchiveAlongsideJournal(t, root, slug)
		bin := filepath.Join(root, "git-spy")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		trace := filepath.Join(root, "git-spawns.log")
		if err := os.WriteFile(
			filepath.Join(bin, "git"),
			[]byte("#!/bin/sh\nprintf 'spawned\\n' >> \"$TPATCH_S7_AQ_GIT_TRACE\"\nexit 88\n"),
			0o755,
		); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		t.Setenv("TPATCH_S7_AQ_GIT_TRACE", trace)

		oldSupported := prepareMutationAuthoritySupported
		oldAcquire := prepareAcquireAuthority
		oldBranch := beforeAbandonBranch
		platformChecks, classifies, acquires, branches := 0, 0, 0, 0
		prepareMutationAuthoritySupported = func() bool {
			platformChecks++
			return true
		}
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return intentlock.AcquireWithFilesystemClassifier(
				path,
				func(*os.File) (string, bool, error) {
					classifies++
					return "aq-local", false, nil
				},
			)
		}
		beforeAbandonBranch = func() { branches++ }
		defer func() {
			prepareMutationAuthoritySupported = oldSupported
			prepareAcquireAuthority = oldAcquire
			beforeAbandonBranch = oldBranch
		}()

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		_, traceErr := os.Stat(trace)
		source := s6RepoFile(t, "internal/cli/prepare_publish.go")
		if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
			platformChecks != 1 || classifies != 1 || acquires != 1 || branches != 1 ||
			!os.IsNotExist(traceErr) {
			t.Fatalf("PIB-494 gate ordering = exit:%d stderr:%q report:%+v platform:%d classify:%d acquire:%d branch:%d git:%v",
				code, stderr, report, platformChecks, classifies, acquires, branches, traceErr)
		}
		if err := validateS7APAbandonControlFlow(source); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PIB-495", func(t *testing.T) {
		for _, gate := range []string{
			"prepare-unsupported-platform",
			"lock-filesystem-unsupported",
			"directory-flock-unavailable",
		} {
			for _, evidence := range []bool{true, false} {
				root, slug := prepareS4Workspace(
					t, fmt.Sprintf("AQ abandon gate %s %t", gate, evidence),
				)
				laneRel := ".tpatch/local/intent-prepare/" + slug + "/"
				laneAbs := filepath.Join(root, filepath.FromSlash(laneRel))
				if err := os.MkdirAll(laneAbs, 0o700); err != nil {
					t.Fatal(err)
				}
				if evidence {
					if err := os.WriteFile(
						filepath.Join(laneAbs, "journal.json"), []byte("{bad"), 0o600,
					); err != nil {
						t.Fatal(err)
					}
				}
				restore := s7AQInstallAbandonGateFailure(t, gate)
				code, stdout, stderr, _ := runPrepare(
					t, "--path", root, "prepare", slug,
					"--abandon-transaction", "--yes", "--json", "--quiet",
				)
				restore()
				report := prepareS4Report(t, stdout)
				text := ""
				if report.Refusal != nil {
					text = report.Refusal.Message + " " + report.Refusal.Remediation
				}
				hasProcedure := strings.Contains(text, "rm -rf "+laneRel)
				if code != 3 || stderr == "" || report.Refusal == nil ||
					report.Refusal.Code != gate || strings.Contains(text, root) ||
					(evidence && (!hasProcedure ||
						!strings.Contains(text, "undo evidence") ||
						!strings.Contains(text, ".tpatch/features/"))) ||
					(!evidence && hasProcedure) {
					t.Fatalf("PIB-495 %s evidence=%t = exit:%d stderr:%q report:%+v",
						gate, evidence, code, stderr, report)
				}
			}
		}

		root, slug := prepareS4Workspace(t, "AQ abandon contention omission")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatal(err)
		}
		defer authority.Release()
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		text := ""
		if report.Refusal != nil {
			text = report.Refusal.Message + " " + report.Refusal.Remediation
		}
		if code != 3 || stderr == "" || report.Refusal == nil ||
			report.Refusal.Code != "transaction-in-progress" ||
			strings.Contains(text, "rm -rf") ||
			strings.Contains(text, ".tpatch/local/intent-prepare/") {
			t.Fatalf("PIB-495 contention procedure = exit:%d stderr:%q report:%+v",
				code, stderr, report)
		}
	})

	t.Run("PIB-499", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AQ repeated abandon")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		first := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || first.Abandoned == nil {
			t.Fatalf("PIB-499 first abandon = exit:%d stderr:%q report:%+v", code, stderr, first)
		}
		existing := first.Abandoned.Directory
		residueAbs := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(existing, "/")))
		before := snapshotTreeMetadata(t, "PIB-499 residue", residueAbs)
		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		second := prepareS4Report(t, stdout)
		after := snapshotTreeMetadata(t, "PIB-499 residue", residueAbs)
		wantRemove := "rm -rf " + existing
		if code != 3 || stderr == "" || second.Refusal == nil ||
			second.Refusal.Code != "no-pending-transaction" ||
			second.Abandoned == nil ||
			!reflect.DeepEqual(second.Abandoned.Existing, []string{existing}) ||
			!strings.Contains(second.Refusal.Remediation, wantRemove) ||
			before != after {
			t.Fatalf("PIB-499 second abandon = exit:%d stderr:%q report:%+v changed:%t",
				code, stderr, second, before != after)
		}
		matches, err := filepath.Glob(filepath.Join(
			root, ".tpatch", "local", "intent-prepare", slug, "abandoned-*",
		))
		if err != nil || !reflect.DeepEqual(matches, []string{residueAbs}) {
			t.Fatalf("PIB-499 residue layout = %v err=%v", matches, err)
		}
		code, human, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes",
		)
		if code != 3 || !strings.Contains(human, existing) ||
			!strings.Contains(human, wantRemove) ||
			strings.Contains(human, root) {
			t.Fatalf("PIB-499 human existing residue = exit:%d stderr:%q\n%s",
				code, stderr, human)
		}
	})
}

func s7AQGitCheckIgnored(t *testing.T, root, rel string) bool {
	t.Helper()
	command := exec.Command("git", "check-ignore", "-q", "--", rel)
	command.Dir = root
	err := command.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore %s: %v", rel, err)
	return false
}

func s7AQSeedPendingArchiveAlongsideJournal(t *testing.T, root, slug string) {
	t.Helper()
	data := []byte("AQ abandon pending archive bytes\n")
	pending := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactAnalysis,
		data,
		store.IntentArchiveWireRemovalPending,
	)
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, pending)),
		map[string][]byte{pending.ContentSHA256: data},
	)
}

func s7AQInstallAbandonGateFailure(t *testing.T, gate string) func() {
	t.Helper()
	oldSupported := prepareMutationAuthoritySupported
	oldAcquire := prepareAcquireAuthority
	switch gate {
	case "prepare-unsupported-platform":
		prepareMutationAuthoritySupported = func() bool { return false }
	case "lock-filesystem-unsupported":
		prepareMutationAuthoritySupported = func() bool { return true }
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			return intentlock.AcquireWithFilesystemClassifier(
				path,
				func(*os.File) (string, bool, error) { return "nfs", true, nil },
			)
		}
	case "directory-flock-unavailable":
		prepareMutationAuthoritySupported = func() bool { return true }
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			return intentlock.AcquireWithFilesystemClassifier(
				path,
				func(*os.File) (string, bool, error) {
					return "", false, errors.New("AQ classifier unavailable")
				},
			)
		}
	default:
		t.Fatalf("unknown abandon gate %s", gate)
	}
	return func() {
		prepareMutationAuthoritySupported = oldSupported
		prepareAcquireAuthority = oldAcquire
	}
}
