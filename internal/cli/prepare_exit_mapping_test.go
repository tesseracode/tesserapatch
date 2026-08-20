package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentpub"
)

func TestPrepareRawPreimageFailuresPreserveExitClass(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		seedIndex      bool
		committedFault bool
		wantExit       int
		wantOutcome    string
		wantCode       string
	}{
		{
			name:           "index-committed",
			target:         "index.preimage.json",
			seedIndex:      true,
			committedFault: true,
			wantExit:       6,
			wantOutcome:    "recovery-refused",
			wantCode:       "post-publication-divergence",
		},
		{
			name:           "status-committed",
			target:         "status.preimage.json",
			committedFault: true,
			wantExit:       6,
			wantOutcome:    "recovery-refused",
			wantCode:       "post-publication-divergence",
		},
		{
			name:        "index-precommit",
			target:      "index.preimage.json",
			seedIndex:   true,
			wantExit:    5,
			wantOutcome: "rolled-back",
			wantCode:    "entry-changed",
		},
		{
			name:        "status-precommit",
			target:      "status.preimage.json",
			wantExit:    5,
			wantOutcome: "rolled-back",
			wantCode:    "entry-changed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, slug := prepareS4Workspace(t, "raw preimage "+test.name)
			runSuccessfulPrepareMode(t, root, slug)
			if test.seedIndex {
				runSuccessfulPrepareMode(t, root, slug, "--regenerate", "--allow-heuristic")
			}

			state := &preparePublicationFaultState{
				targetSuffix:  "/" + test.target,
				failPrecommit: !test.committedFault,
			}
			installPreparePublicationFault(t, state)
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "prepare", slug,
				"--regenerate", "--allow-heuristic", "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			rawRel := ".tpatch/local/intent-prepare/" + slug + "/" + test.target
			rawPath := filepath.Join(root, filepath.FromSlash(rawRel))
			if code != test.wantExit || report.Outcome != test.wantOutcome ||
				report.Refusal == nil || report.Refusal.Code != test.wantCode ||
				!state.failed || !hasPrepareAdvisory(report, "staging-retained") {
				t.Fatalf("raw preimage failure = code=%d report=%#v state=%+v", code, report, state)
			}
			if test.committedFault {
				if !state.renamed || !state.syncFault ||
					!strings.Contains(report.Refusal.Message, rawRel) {
					t.Fatalf("committed raw evidence was not named: report=%#v state=%+v", report, state)
				}
				if _, err := os.Stat(rawPath); err != nil {
					t.Fatalf("committed raw evidence missing: %v", err)
				}
			} else {
				if state.renamed || state.syncFault {
					t.Fatalf("precommit raw failure crossed rename: %+v", state)
				}
				if _, err := os.Stat(rawPath); !os.IsNotExist(err) {
					t.Fatalf("precommit raw failure retained canonical raw path: %v", err)
				}
			}
			journal := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "journal.json")
			if _, err := os.Stat(journal); !os.IsNotExist(err) {
				t.Fatalf("raw preimage failure unexpectedly created a journal: %v", err)
			}
		})
	}
}

func TestPrepareArchiveAppendCommittedExitSixIsNotDowngraded(t *testing.T) {
	root, slug := prepareS4Workspace(t, "archive committed exit six")
	runSuccessfulPrepareMode(t, root, slug)
	state := &preparePublicationFaultState{
		targetContains: "/artifacts/intent-archive/blobs/",
		targetSuffix:   ".blob",
	}
	installPreparePublicationFault(t, state)

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 6 || report.Outcome != "recovery-refused" ||
		report.Refusal == nil || report.Refusal.Code != "post-publication-divergence" ||
		!state.renamed || !state.syncFault ||
		len(report.OrphanBlobs) != 1 ||
		!strings.Contains(report.Refusal.Message, report.OrphanBlobs[0]) ||
		!hasPrepareAdvisory(report, "staging-retained") ||
		!hasPrepareAdvisory(report, "archive-orphan-blobs") {
		t.Fatalf("archive committed exit six = code=%d report=%#v state=%+v", code, report, state)
	}
	if !strings.HasSuffix(state.target, report.OrphanBlobs[0]+".blob") {
		t.Fatalf("orphan truth does not match committed target: target=%q orphans=%v", state.target, report.OrphanBlobs)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(state.target))); err != nil {
		t.Fatalf("committed orphan blob evidence missing: %v", err)
	}
}

func runSuccessfulPrepareMode(t *testing.T, root, slug string, mode ...string) {
	t.Helper()
	args := []string{"--path", root, "prepare", slug}
	args = append(args, mode...)
	args = append(args, "--json", "--quiet")
	code, stdout, stderr, _ := runPrepare(t, args...)
	if code != 0 {
		t.Fatalf("prepare seed failed: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
}

func hasPrepareAdvisory(report preparePublishReport, code string) bool {
	for _, advisory := range report.Advisories {
		if advisory.Code == code {
			return true
		}
	}
	return false
}

type preparePublicationFaultState struct {
	targetSuffix   string
	targetContains string
	failPrecommit  bool
	target         string
	renamed        bool
	syncFault      bool
	failed         bool
}

type preparePublicationFaultOps struct {
	intentpub.RootOps
	state *preparePublicationFaultState
}

func (ops *preparePublicationFaultOps) Rename(oldName, newName string) error {
	if !ops.state.matches(newName) {
		return ops.RootOps.Rename(oldName, newName)
	}
	ops.state.target = newName
	if ops.state.failPrecommit {
		ops.state.failed = true
		return errors.New("injected precommit publication failure")
	}
	if err := ops.RootOps.Rename(oldName, newName); err != nil {
		return err
	}
	ops.state.renamed = true
	return nil
}

func (ops *preparePublicationFaultOps) Open(name string) (intentpub.RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	return &preparePublicationFaultFile{RootFile: file, state: ops.state}, nil
}

type preparePublicationFaultFile struct {
	intentpub.RootFile
	state *preparePublicationFaultState
}

func (file *preparePublicationFaultFile) Sync() error {
	if file.state.renamed && !file.state.failed {
		file.state.failed = true
		file.state.syncFault = true
		return errors.New("injected committed directory sync failure")
	}
	return file.RootFile.Sync()
}

func (state *preparePublicationFaultState) matches(path string) bool {
	return strings.Contains(path, state.targetContains) &&
		strings.HasSuffix(path, state.targetSuffix)
}

func installPreparePublicationFault(t *testing.T, state *preparePublicationFaultState) {
	t.Helper()
	old := prepareIntentpubRootOps
	t.Cleanup(func() { prepareIntentpubRootOps = old })
	prepareIntentpubRootOps = func(root *os.Root) intentpub.RootOps {
		return &preparePublicationFaultOps{
			RootOps: intentpub.NewRootOps(root),
			state:   state,
		}
	}
}
