//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const s7AQCrashExit = 97

func TestS7AQCrashFixtureHelper(t *testing.T) {
	if os.Getenv("TPATCH_S7_AQ_CRASH_HELPER") != "1" {
		return
	}
	root := os.Getenv("TPATCH_S7_AQ_CRASH_ROOT")
	slug := os.Getenv("TPATCH_S7_AQ_CRASH_SLUG")
	phase := os.Getenv("TPATCH_S7_AQ_CRASH_PHASE")
	renames := 0
	prepareIntentpubHook = func(
		point intentpub.CrashPoint,
		_ *os.Root,
		entry *intentpub.Entry,
	) error {
		crash := false
		switch phase {
		case "CP3":
			crash = point == intentpub.PointAfterJournalDurable
		case "CP4":
			if point == intentpub.PointAfterEntryRename {
				renames++
				crash = renames == 2
			}
		case "CP5":
			crash = point == intentpub.PointAfterEntryRename && entry != nil &&
				entry.ArtifactID == intentpub.ArtifactAnalysisSidecar
		case "CP6":
			crash = point == intentpub.PointAfterEntryRename && entry != nil &&
				entry.ArtifactID == intentpub.ArtifactArchiveIndex
		case "CP7":
			crash = point == intentpub.PointAfterAllRenames
		}
		if crash {
			os.Exit(s7AQCrashExit)
		}
		return nil
	}
	command := buildRootCmd()
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	args := []string{"--path", root, "prepare", slug}
	switch prepareMode(os.Getenv("TPATCH_S7_AQ_CRASH_MODE")) {
	case prepareModeGenerate:
	case prepareModeManual:
		args = append(args, "--manual")
	default:
		args = append(args, "--regenerate", "--allow-heuristic")
	}
	args = append(args, "--json", "--quiet")
	command.SetArgs(args)
	os.Exit(execute(command, os.Stderr))
}

type s7AQRecoverySpies struct {
	afterRecovery        int
	lifecycle            int
	admissibility        int
	coherence            int
	provider             int
	staging              int
	archive              int
	redaction            int
	setRevalidation      int
	postRecoveryMutation int
}

type s7AQRecoveryObservation struct {
	root             string
	slug             string
	mode             prepareMode
	phase            string
	code             int
	stdout           string
	stderr           string
	report           preparePublishReport
	journal          intentpub.Journal
	expectedRestored []string
	canonicalCorrect bool
	blobsBefore      string
	blobsAfter       string
	postRecoveryTree string
	spies            s7AQRecoverySpies
	journalRemoved   bool
	allowHeuristic   bool
}

func TestS7AQTerminalRecoveryContracts(t *testing.T) {
	modes := []prepareMode{
		prepareModeGenerate,
		prepareModeManual,
		prepareModeRegenerate,
	}
	phases := []string{"CP3", "CP4", "CP5", "CP6", "CP7"}

	t.Run("PIB-483", func(t *testing.T) {
		observed := 0
		for _, mode := range modes {
			for _, phase := range phases {
				observation := s7AQObserveTerminalRecovery(t, mode, phase, false)
				if observation.code != 0 || observation.stderr != "" ||
					observation.report.Outcome != "recovered" ||
					observation.report.Action != "none" ||
					observation.report.Recovery == nil ||
					observation.report.Recovery.Kind != "journal-undo" ||
					!reflect.DeepEqual(
						observation.report.Recovery.RestoredEntries,
						observation.expectedRestored,
					) ||
					observation.report.Recovery.RetryCWD != "workspace-root" ||
					!s7AQHasAdvisory(observation.report, "recovered-prior-transaction") ||
					observation.report.Refusal != nil ||
					!observation.canonicalCorrect ||
					observation.blobsAfter != observation.blobsBefore ||
					!observation.journalRemoved {
					t.Fatalf("PIB-483 %s/%s terminal recovery = %+v",
						mode, phase, observation)
				}
				observed++
			}
		}
		if observed != 15 {
			t.Fatalf("PIB-483 mode/phase observations = %d, want 15", observed)
		}
	})

	t.Run("PIB-484", func(t *testing.T) {
		source := s6RepoFile(t, "internal/cli/prepare_publish.go")
		if err := validateS7AQTerminalRecoveryControlFlow(source); err != nil {
			t.Fatal(err)
		}
		observed := 0
		for _, mode := range modes {
			for _, phase := range phases {
				observation := s7AQObserveTerminalRecovery(t, mode, phase, true)
				if observation.code != 0 ||
					observation.spies.afterRecovery != 1 ||
					observation.spies.lifecycle != 0 ||
					observation.spies.admissibility != 0 ||
					observation.spies.coherence != 0 ||
					observation.spies.provider != 0 ||
					observation.spies.staging != 0 ||
					observation.spies.archive != 0 ||
					observation.spies.redaction != 0 ||
					observation.spies.setRevalidation != 0 ||
					observation.spies.postRecoveryMutation != 0 {
					t.Fatalf("PIB-484 %s/%s post-recovery effects = %+v",
						mode, phase, observation.spies)
				}
				observed++
			}
		}
		if observed != 15 {
			t.Fatalf("PIB-484 mode/phase observations = %d, want 15", observed)
		}
	})

	t.Run("PIB-485", func(t *testing.T) {
		observed := 0
		for _, fixture := range []struct {
			mode           prepareMode
			crashMode      prepareMode
			phase          string
			allowHeuristic bool
		}{
			{
				mode: prepareModeGenerate, crashMode: prepareModeGenerate,
				phase: "CP3",
			},
			{
				mode: prepareModeManual, crashMode: prepareModeRegenerate,
				phase: "CP4",
			},
			{
				mode: prepareModeRegenerate, crashMode: prepareModeRegenerate,
				phase: "CP3",
			},
		} {
			observation := s7AQObserveRetryableTerminalRecovery(
				t, fixture.mode, fixture.crashMode, fixture.phase,
				fixture.allowHeuristic,
			)
			if observation.code != 0 || observation.stderr != "" ||
				observation.report.Outcome != "recovered" ||
				observation.report.Action != "none" ||
				observation.report.Recovery == nil ||
				!observation.canonicalCorrect ||
				!observation.journalRemoved {
				t.Fatalf("PIB-485 %s/%s recovery = %+v",
					fixture.mode, fixture.phase, observation)
			}
			argv, err := s7APParseRenderedCommand(observation.report.Recovery.Retry)
			if err != nil {
				t.Fatal(err)
			}
			if got := snapshotTreeMetadata(
				t, "PIB-485 post-recovery", filepath.Join(observation.root, ".tpatch"),
			); got != observation.postRecoveryTree {
				t.Fatalf("PIB-485 %s/%s changed the recovered tree before retry",
					fixture.mode, fixture.phase)
			}
			attempt, restore := s7AQObserveRecoveryRetry(t, observation, argv)
			code, stdout, stderr := func() (int, string, string) {
				defer restore()
				return s7APRunFromWorkspace(t, observation.root, argv)
			}()
			report := prepareS4Report(t, stdout)
			retryChangedTree := snapshotTreeMetadata(
				t, "PIB-485 post-recovery", filepath.Join(observation.root, ".tpatch"),
			) != observation.postRecoveryTree
			if code != 0 || stderr != "" ||
				report.Outcome != "published" ||
				report.Outcome == "no-op" || report.Outcome == "rolled-back" ||
				report.Recovery != nil ||
				strings.Contains(stdout, `"outcome": "recovered"`) ||
				strings.Contains(stdout, `"outcome": "no-op"`) ||
				strings.Contains(stdout, `"outcome": "rolled-back"`) ||
				!s7AQRetryResultIsAdmissible(report, code) ||
				!attempt.provesModeAttempt(
					fixture.mode, report, retryChangedTree,
				) {
				t.Fatalf("PIB-485 %s/%s retry = exit:%d stderr:%q report:%+v attempt:%+v",
					fixture.mode, fixture.phase, code, stderr, report, attempt)
			}
			disabled := *attempt
			disabled.disableModeBoundary(fixture.mode)
			if disabled.provesModeAttempt(
				fixture.mode, report, retryChangedTree,
			) {
				t.Fatalf("PIB-485 %s accepted the same result without its observed mode boundary",
					fixture.mode)
			}
			if _, statErr := os.Stat(filepath.Join(
				observation.root,
				filepath.FromSlash(intentpub.JournalRel(observation.slug)),
			)); !os.IsNotExist(statErr) {
				t.Fatalf("PIB-485 %s/%s retry left a journal: %v",
					fixture.mode, fixture.phase, statErr)
			}
			observed++
		}
		if observed != 3 {
			t.Fatalf("PIB-485 normative mode/phase retries = %d, want 3", observed)
		}
		if s7AQRetryResultIsAdmissible(
			preparePublishReport{Outcome: "no-op", Action: "none"}, 0,
		) {
			t.Fatal("PIB-485 no-op is accepted as a terminal retry result")
		}
	})

	t.Run("PIB-488", func(t *testing.T) {
		cases := s7AQJournalBindCases()
		if len(cases) != 10 {
			t.Fatalf("PIB-488 J-bind fixtures = %d, want 10", len(cases))
		}
		observed := 0
		for _, test := range cases {
			root, slug := prepareS4Workspace(t, "AQ "+test.name)
			s7AQWriteJournalBindFixture(t, root, slug, test.name)
			before := snapshotTreeMetadata(t, "PIB-488", filepath.Join(root, ".tpatch"))
			code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			after := snapshotTreeMetadata(t, "PIB-488", filepath.Join(root, ".tpatch"))
			journalRel := intentpub.JournalRel(slug)
			laneRel := ".tpatch/local/intent-prepare/" + slug + "/"
			featureRel := ".tpatch/features/" + slug + "/"
			evidenceText := ""
			if report.Refusal != nil {
				evidenceText = report.Refusal.Message + " " + report.Refusal.Remediation
			}
			if code != 6 || stderr == "" || report.Outcome != "recovery-refused" ||
				report.Refusal == nil || report.Refusal.Code != test.code ||
				report.Recovery != nil || before != after ||
				!strings.Contains(evidenceText, journalRel) ||
				!strings.Contains(evidenceText, laneRel) ||
				!strings.Contains(evidenceText, featureRel) ||
				!strings.Contains(evidenceText, featureRel+"artifacts/intent-archive/") {
				t.Fatalf("PIB-488 %s = exit:%d stderr:%q report:%+v changed:%t",
					test.name, code, stderr, report, before != after)
			}
			observed++
		}

		for _, shape := range []string{"CP9", "undo-cas"} {
			observation := s7AQObserveRecoveryFailure(t, shape)
			evidenceText := ""
			if observation.report.Refusal != nil {
				evidenceText = observation.report.Refusal.Message + " " +
					observation.report.Refusal.Remediation
			}
			if observation.code != 6 || observation.stderr == "" ||
				observation.report.Outcome != "recovery-refused" ||
				observation.report.Refusal == nil ||
				observation.report.Refusal.Code != observation.wantCode ||
				observation.report.Recovery != nil ||
				observation.report.Outcome == "rolled-back" ||
				observation.expectedTree != observation.resultTree ||
				!strings.Contains(evidenceText, intentpub.JournalRel(observation.slug)) ||
				!strings.Contains(evidenceText, ".tpatch/features/"+observation.slug+"/") ||
				!strings.Contains(evidenceText, ".tpatch/local/intent-prepare/"+observation.slug+"/") {
				t.Fatalf("PIB-488 %s failure = %+v", shape, observation)
			}
			observed++
		}
		if observed != 12 {
			t.Fatalf("PIB-488 recovery failures = %d, want 12", observed)
		}
	})
}

func TestS7AQPostRecoveryControlFlowGuard(t *testing.T) {
	t.Run("PIB-487", func(t *testing.T) {
		source := s6RepoFile(t, "internal/cli/prepare_publish.go")
		if err := validateS7AQTerminalRecoveryControlFlow(source); err != nil {
			t.Fatal(err)
		}
		lifecycle := strings.Replace(
			source,
			"if afterRecoveryComplete != nil {\n\t\t\tafterRecoveryComplete()\n\t\t}",
			"if afterRecoveryComplete != nil {\n\t\t\tafterRecoveryComplete()\n\t\t}\n\t\tif report.FeatureState != \"\" {\n\t\t\treturn emitPreparePublishReport(cmd, report, 3)\n\t\t}",
			1,
		)
		if err := validateS7AQTerminalRecoveryControlFlow(lifecycle); err == nil {
			t.Fatal("PIB-487 same validator accepted a post-recovery lifecycle return")
		}
		admissibility := strings.Replace(
			source,
			"if afterRecoveryComplete != nil {\n\t\t\tafterRecoveryComplete()\n\t\t}",
			"if afterRecoveryComplete != nil {\n\t\t\tafterRecoveryComplete()\n\t\t}\n\t\tif len(report.Artifacts) != 0 {\n\t\t\treturn emitPreparePublishReport(cmd, report, 3)\n\t\t}",
			1,
		)
		if err := validateS7AQTerminalRecoveryControlFlow(admissibility); err == nil {
			t.Fatal("PIB-487 same validator accepted a post-recovery artifact return")
		}
		arbitrary := strings.Replace(
			source,
			"if afterRecoveryComplete != nil {\n\t\t\tafterRecoveryComplete()\n\t\t}",
			"if afterRecoveryComplete != nil {\n\t\t\tafterRecoveryComplete()\n\t\t}\n\t\tif false {\n\t\t\treturn emitPreparePublishReport(cmd, report, 0)\n\t\t}",
			1,
		)
		if err := validateS7AQTerminalRecoveryControlFlow(arbitrary); err == nil {
			t.Fatal("PIB-487 same validator accepted an arbitrary extra return")
		}
		callAnchor := "\t\t_ = release()\n\t\treturn emitPreparePublishReport(cmd, report, 0)"
		for _, sensitivity := range []struct {
			name   string
			insert string
			suffix string
		}{
			{
				name:   "direct-call",
				insert: "\t\t_ = prepareStateRefusal(report.FeatureState)\n",
			},
			{
				name:   "function-alias",
				insert: "\t\tgate := prepareStateRefusal\n\t\t_ = gate(report.FeatureState)\n",
			},
			{
				name: "method-value",
				insert: "\t\treceiver := s7AQRecoveryGateReceiver{}\n" +
					"\t\tgate := receiver.run\n\t\t_ = gate(report.FeatureState)\n",
				suffix: "\ntype s7AQRecoveryGateReceiver struct{}\n\n" +
					"func (s7AQRecoveryGateReceiver) run(state string) string {\n" +
					"\treturn prepareStateRefusal(state)\n}\n",
			},
			{
				name:   "local-wrapper",
				insert: "\t\t_ = s7AQRecoveryGateWrapper(report.FeatureState)\n",
				suffix: "\nfunc s7AQRecoveryGateWrapper(state string) string {\n" +
					"\treturn prepareStateRefusal(state)\n}\n",
			},
			{
				name: "function-parameter",
				insert: "\t\t_ = s7AQRecoveryGateParameter(" +
					"prepareStateRefusal, report.FeatureState)\n",
				suffix: "\nfunc s7AQRecoveryGateParameter(" +
					"gate func(string) string, state string) string {\n" +
					"\treturn gate(state)\n}\n",
			},
			{
				name:   "function-return",
				insert: "\t\tgate := s7AQRecoveryGateFactory()\n\t\t_ = gate(report.FeatureState)\n",
				suffix: "\nfunc s7AQRecoveryGateFactory() func(string) string {\n" +
					"\treturn prepareStateRefusal\n}\n",
			},
			{
				name: "unresolved-call",
				insert: "\t\tvar unknownGate func(string) string\n" +
					"\t\t_ = unknownGate(report.FeatureState)\n",
			},
		} {
			mutated := strings.Replace(
				source, callAnchor, sensitivity.insert+callAnchor, 1,
			) + sensitivity.suffix
			if mutated == source+sensitivity.suffix {
				t.Fatalf("PIB-487 %s mutation anchor missing", sensitivity.name)
			}
			if err := validateS7AQTerminalRecoveryControlFlow(mutated); err == nil {
				t.Fatalf("PIB-487 same validator accepted %s gate", sensitivity.name)
			}
		}
		advisoryAnchor := "func prepareAdvisory(code, artifactID, message string) prepareAdvisoryReport {\n" +
			"\treturn prepareAdvisoryReport{Code: code, ArtifactID: artifactID, Message: message}\n}"
		for _, sensitivity := range []struct {
			name   string
			body   string
			suffix string
		}{
			{
				name: "forbidden-gate-in-formerly-trusted-helper",
				body: "func prepareAdvisory(code, artifactID, message string) prepareAdvisoryReport {\n" +
					"\t_ = prepareStateRefusal(message)\n" +
					"\treturn prepareAdvisoryReport{Code: code, ArtifactID: artifactID, Message: message}\n}",
			},
			{
				name: "forbidden-gate-in-nested-helper",
				body: "func prepareAdvisory(code, artifactID, message string) prepareAdvisoryReport {\n" +
					"\t_ = s7AQNestedAdvisoryGate(message)\n" +
					"\treturn prepareAdvisoryReport{Code: code, ArtifactID: artifactID, Message: message}\n}",
				suffix: "\nfunc s7AQNestedAdvisoryGate(state string) string {\n" +
					"\treturn prepareStateRefusal(state)\n}\n",
			},
		} {
			mutated := strings.Replace(source, advisoryAnchor, sensitivity.body, 1) +
				sensitivity.suffix
			if mutated == source+sensitivity.suffix {
				t.Fatalf("PIB-487 %s mutation anchor missing", sensitivity.name)
			}
			if err := validateS7AQTerminalRecoveryControlFlow(mutated); err == nil {
				t.Fatalf("PIB-487 same validator accepted %s", sensitivity.name)
			}
		}
		for _, sensitivity := range []struct {
			name string
			body string
		}{
			{
				name: "conditional-production-seam-assignment",
				body: "\nfunc init() {\n" +
					"\tif os.Getenv(\"TPATCH_BAD_RECOVERY_CALLBACK\") != \"\" {\n" +
					"\t\tafterRecoveryComplete = func() {\n" +
					"\t\t\t_ = prepareStateRefusal(\"defined\")\n" +
					"\t\t}\n\t}\n}\n",
			},
			{
				name: "aliased-production-seam-assignment",
				body: "\nfunc init() {\n" +
					"\tcallback := func() { _ = prepareStateRefusal(\"defined\") }\n" +
					"\tafterRecoveryComplete = callback\n}\n",
			},
			{
				name: "helper-production-seam-assignment",
				body: "\nfunc s7AQHiddenRecoveryCallback() {\n" +
					"\t_ = prepareStateRefusal(\"defined\")\n}\n" +
					"func init() { afterRecoveryComplete = s7AQHiddenRecoveryCallback }\n",
			},
		} {
			mutated := source + sensitivity.body
			if err := validateS7AQTerminalRecoveryControlFlow(mutated); err == nil {
				t.Fatalf("PIB-487 same validator accepted %s", sensitivity.name)
			}
		}
	})
}

func s7AQObserveTerminalRecovery(
	t *testing.T,
	mode prepareMode,
	phase string,
	withSpies bool,
) s7AQRecoveryObservation {
	t.Helper()
	root, slug := prepareS4Workspace(t, fmt.Sprintf("AQ terminal %s %s", mode, phase))
	prepareS4WriteReadyBundle(t, root, slug, false)
	s7AQSeedReadyArchive(t, root, slug)
	featureRoot := filepath.Join(root, ".tpatch", "features", slug)
	s7AQCreateCrashJournal(t, root, slug, phase)
	journalRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug))))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := intentpub.DecodeJournal(journalRaw, slug)
	if err != nil {
		t.Fatal(err)
	}
	expectedRestored := s7AQExpectedRestoredEntries(t, root, journal)
	blobsRoot := filepath.Join(featureRoot, "artifacts", "intent-archive", "blobs")
	blobsBefore := snapshotTreeMetadata(t, "blobs", blobsRoot)

	oldAfter := afterRecoveryComplete
	oldProvider := prepareLoadProvider
	oldHook := prepareIntentpubHook
	oldRedaction := beforeRedactionScan
	oldRevalidate := beforePrepareSetRevalidation
	oldIndex := beforeIndexRewrite
	spies := s7AQRecoverySpies{}
	afterRecoveryComplete = func() {
		spies.afterRecovery++
		if !withSpies {
			return
		}
		prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
			spies.provider++
			return nil, provider.Config{}
		}
		prepareIntentpubHook = func(intentpub.CrashPoint, *os.Root, *intentpub.Entry) error {
			spies.staging++
			spies.postRecoveryMutation++
			return errors.New("post-recovery staging reached")
		}
		beforeRedactionScan = func() {
			spies.redaction++
			spies.postRecoveryMutation++
		}
		beforePrepareSetRevalidation = func() {
			spies.setRevalidation++
			spies.postRecoveryMutation++
		}
		beforeIndexRewrite = func(string) {
			spies.archive++
			spies.postRecoveryMutation++
		}
	}
	defer func() {
		afterRecoveryComplete = oldAfter
		prepareLoadProvider = oldProvider
		prepareIntentpubHook = oldHook
		beforeRedactionScan = oldRedaction
		beforePrepareSetRevalidation = oldRevalidate
		beforeIndexRewrite = oldIndex
	}()

	args := []string{"--path", root, "prepare", slug}
	switch mode {
	case prepareModeManual:
		args = append(args, "--manual")
	case prepareModeRegenerate:
		args = append(args, "--regenerate", "--allow-heuristic")
	}
	args = append(args, "--json", "--quiet")
	code, stdout, stderr, _ := runPrepare(t, args...)
	report := prepareS4Report(t, stdout)
	_, journalErr := os.Stat(filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug))))
	canonicalCorrect := true
	for _, entry := range journal.Entries {
		identity := s7APCaptureIdentity(t, root, entry.Rel)
		want := entry.Preimage
		if phase == "CP7" {
			want = entry.NewImage
		}
		if !identity.Equal(want) {
			canonicalCorrect = false
		}
	}
	return s7AQRecoveryObservation{
		root: root, slug: slug, mode: mode, phase: phase,
		code: code, stdout: stdout, stderr: stderr, report: report,
		journal: journal, expectedRestored: expectedRestored,
		canonicalCorrect: canonicalCorrect,
		blobsBefore:      blobsBefore,
		blobsAfter:       snapshotTreeMetadata(t, "blobs", blobsRoot),
		postRecoveryTree: snapshotTreeMetadata(t, "PIB-485 post-recovery", filepath.Join(root, ".tpatch")),
		spies:            spies,
		journalRemoved:   os.IsNotExist(journalErr),
		allowHeuristic:   mode == prepareModeRegenerate,
	}
}

func s7AQObserveRetryableTerminalRecovery(
	t *testing.T,
	mode prepareMode,
	crashMode prepareMode,
	phase string,
	allowHeuristic bool,
) s7AQRecoveryObservation {
	t.Helper()
	root, slug := prepareS4Workspace(t, fmt.Sprintf("AQ retryable %s %s", mode, phase))
	switch crashMode {
	case prepareModeManual:
		prepareS4WriteReadyBundle(t, root, slug, false)
	case prepareModeRegenerate:
		prepareS4WriteReadyBundle(t, root, slug, false)
		s7AQSeedReadyArchive(t, root, slug)
	}
	s7AQCreateModeCrashJournal(t, root, slug, phase, crashMode)
	journalRaw, err := os.ReadFile(
		filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug))),
	)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := intentpub.DecodeJournal(journalRaw, slug)
	if err != nil {
		t.Fatal(err)
	}
	expectedRestored := s7AQExpectedRestoredEntries(t, root, journal)
	args := []string{"--path", root, "prepare", slug}
	switch mode {
	case prepareModeManual:
		args = append(args, "--manual")
	case prepareModeRegenerate:
		args = append(args, "--regenerate")
		if allowHeuristic {
			args = append(args, "--allow-heuristic")
		}
	}
	args = append(args, "--json", "--quiet")
	code, stdout, stderr, _ := runPrepare(t, args...)
	report := prepareS4Report(t, stdout)
	_, journalErr := os.Stat(
		filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug))),
	)
	canonicalCorrect := true
	for _, entry := range journal.Entries {
		identity := s7APCaptureIdentity(t, root, entry.Rel)
		want := entry.Preimage
		if phase == "CP7" {
			want = entry.NewImage
		}
		if !identity.Equal(want) {
			canonicalCorrect = false
		}
	}
	return s7AQRecoveryObservation{
		root: root, slug: slug, mode: mode, phase: phase,
		code: code, stdout: stdout, stderr: stderr, report: report,
		journal: journal, expectedRestored: expectedRestored,
		canonicalCorrect: canonicalCorrect,
		postRecoveryTree: snapshotTreeMetadata(
			t, "PIB-485 post-recovery", filepath.Join(root, ".tpatch"),
		),
		journalRemoved: os.IsNotExist(journalErr),
		allowHeuristic: allowHeuristic,
	}
}

type s7AQRetryAttempt struct {
	providerLoads       int
	providerGenerations int
	redactions          int
	revalidations       int
	staging             int
	manualStatus        int
}

type s7AQRetryProvider struct {
	attempt *s7AQRetryAttempt
}

func (*s7AQRetryProvider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (fixture *s7AQRetryProvider) Generate(
	_ context.Context,
	_ provider.Config,
	request provider.GenerateRequest,
) (string, error) {
	fixture.attempt.providerGenerations++
	switch {
	case strings.Contains(request.SystemPrompt, "JSON response") &&
		strings.Contains(request.SystemPrompt, "compatibility"):
		return `{"summary":"S7 AQ deterministic analysis","compatibility":{"status":"unclear","reasoning":"test fixture"},"affected_areas":[],"acceptance_criteria":[],"implementation_notes":[],"unresolved_questions":[]}`, nil
	case strings.Contains(request.SystemPrompt, "Acceptance Criteria") &&
		strings.Contains(request.SystemPrompt, "Implementation Plan"):
		return "## Acceptance Criteria\n\n1. Deterministic publication.\n\n## Implementation Plan\n\n1. Publish the bundle.", nil
	case strings.Contains(request.SystemPrompt, "exploring a codebase") &&
		strings.Contains(request.SystemPrompt, "Relevant Files"):
		return "## Relevant Files\n\n- `internal/cli`: deterministic test fixture.\n\n## Minimal Changeset\n\nPublish the bundle.", nil
	default:
		return "", fmt.Errorf("PIB-485 unexpected provider request: %q", request.SystemPrompt)
	}
}

func s7AQObserveRecoveryRetry(
	t *testing.T,
	observation s7AQRecoveryObservation,
	argv []string,
) (*s7AQRetryAttempt, func()) {
	t.Helper()
	want := []string{"prepare", observation.slug}
	switch observation.mode {
	case prepareModeManual:
		want = append(want, "--manual")
	case prepareModeRegenerate:
		want = append(want, "--regenerate")
		if observation.allowHeuristic {
			want = append(want, "--allow-heuristic")
		}
	}
	want = append(want, "--json", "--quiet")
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("PIB-485 %s retry argv = %v, want %v", observation.mode, argv, want)
	}

	attempt := &s7AQRetryAttempt{}
	oldProvider := prepareLoadProvider
	oldProviderConfig := prepareLoadProviderConfig
	oldRedaction := beforeRedactionScan
	oldRevalidate := beforePrepareSetRevalidation
	oldHook := prepareIntentpubHook
	oldManualStatus := beforeManualStatusCAS
	config := provider.Config{
		Type:    "openai-compatible",
		BaseURL: "https://s7-aq.invalid",
		Model:   "s7-aq-deterministic",
	}
	fixtureProvider := &s7AQRetryProvider{attempt: attempt}
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		attempt.providerLoads++
		return fixtureProvider, config
	}
	prepareLoadProviderConfig = func(*store.Store) provider.Config {
		return config
	}
	restoreEnvironment := s7AQIsolateProviderEnvironment(t, observation.root)
	beforeRedactionScan = func() {
		attempt.redactions++
		if oldRedaction != nil {
			oldRedaction()
		}
	}
	beforePrepareSetRevalidation = func() {
		attempt.revalidations++
		if oldRevalidate != nil {
			oldRevalidate()
		}
	}
	prepareIntentpubHook = func(
		point intentpub.CrashPoint,
		root *os.Root,
		entry *intentpub.Entry,
	) error {
		attempt.staging++
		if oldHook != nil {
			return oldHook(point, root, entry)
		}
		return nil
	}
	beforeManualStatusCAS = func() {
		attempt.manualStatus++
		if oldManualStatus != nil {
			oldManualStatus()
		}
	}
	restore := func() {
		prepareLoadProvider = oldProvider
		prepareLoadProviderConfig = oldProviderConfig
		beforeRedactionScan = oldRedaction
		beforePrepareSetRevalidation = oldRevalidate
		prepareIntentpubHook = oldHook
		beforeManualStatusCAS = oldManualStatus
		restoreEnvironment()
	}
	return attempt, restore
}

func s7AQIsolateProviderEnvironment(t *testing.T, workspace string) func() {
	t.Helper()
	type priorEnvironment struct {
		value string
		set   bool
	}
	isolationRoot := filepath.Join(
		filepath.Dir(workspace), "."+filepath.Base(workspace)+"-pib485-provider",
	)
	environment := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(isolationRoot, "xdg"),
		"HOME":            filepath.Join(isolationRoot, "home"),
		"USERPROFILE":     filepath.Join(isolationRoot, "profile"),
		"APPDATA":         filepath.Join(isolationRoot, "appdata"),
		"LOCALAPPDATA":    filepath.Join(isolationRoot, "localappdata"),
	}
	prior := make(map[string]priorEnvironment, len(environment))
	for name, value := range environment {
		current, set := os.LookupEnv(name)
		prior[name] = priorEnvironment{value: current, set: set}
		if err := os.Setenv(name, value); err != nil {
			t.Fatalf("PIB-485 isolate provider environment %s: %v", name, err)
		}
	}
	return func() {
		for name, value := range prior {
			var err error
			if value.set {
				err = os.Setenv(name, value.value)
			} else {
				err = os.Unsetenv(name)
			}
			if err != nil {
				t.Errorf("PIB-485 restore provider environment %s: %v", name, err)
			}
		}
	}
}

func (attempt s7AQRetryAttempt) provesModeAttempt(
	mode prepareMode,
	report preparePublishReport,
	changedTree bool,
) bool {
	if report.Mode != mode || report.Outcome != "published" ||
		report.Refusal != nil || report.Action == "none" || !changedTree ||
		!s7AQRetryArtifactsMatchMode(mode, report) {
		return false
	}
	switch mode {
	case prepareModeGenerate:
		return report.Action == "complete" &&
			attempt.providerLoads == 1 && attempt.providerGenerations == 3 &&
			attempt.redactions == 0 && attempt.revalidations == 1 &&
			attempt.staging == 15 && attempt.manualStatus == 0
	case prepareModeManual:
		return report.Action == "adopt" &&
			attempt.providerLoads == 0 && attempt.providerGenerations == 0 &&
			attempt.redactions == 0 && attempt.revalidations == 0 &&
			attempt.staging == 0 && attempt.manualStatus == 1
	case prepareModeRegenerate:
		return report.Action == "regenerate" &&
			attempt.providerLoads == 1 && attempt.providerGenerations == 3 &&
			attempt.redactions == 1 && attempt.revalidations == 1 &&
			attempt.staging == 17 && attempt.manualStatus == 0
	default:
		return false
	}
}

func s7AQRetryArtifactsMatchMode(mode prepareMode, report preparePublishReport) bool {
	want := map[string]string{
		"analysis":         "",
		"spec":             "",
		"exploration":      "",
		"analysis_sidecar": "",
	}
	switch mode {
	case prepareModeGenerate:
		for id := range want {
			want[id] = "generated"
		}
	case prepareModeManual:
		for id := range want {
			want[id] = "preserved"
		}
		want["analysis_sidecar"] = "absent-optional"
	case prepareModeRegenerate:
		for id := range want {
			want[id] = "regenerated"
		}
		want["analysis_sidecar"] = "generated"
	default:
		return false
	}
	if len(report.Artifacts) != len(want) {
		return false
	}
	for _, artifact := range report.Artifacts {
		disposition, ok := want[artifact.ID]
		if !ok || artifact.Disposition != disposition {
			return false
		}
		delete(want, artifact.ID)
	}
	return len(want) == 0
}

func (attempt *s7AQRetryAttempt) disableModeBoundary(mode prepareMode) {
	switch mode {
	case prepareModeGenerate:
		attempt.revalidations = 0
	case prepareModeManual:
		attempt.manualStatus = 0
	case prepareModeRegenerate:
		attempt.providerLoads = 0
	}
}

func s7AQRetryResultIsAdmissible(report preparePublishReport, exit int) bool {
	if exit == 0 {
		return report.Refusal == nil &&
			report.Outcome == "published" && report.Action != "none"
	}
	return exit == 3 && report.Outcome == "refused" && report.Refusal != nil
}

func s7AQSeedReadyArchive(t *testing.T, root, slug string) {
	t.Helper()
	inputs := []struct {
		id   store.IntentArchiveArtifactID
		data []byte
	}{
		{id: store.IntentArchiveArtifactAnalysis, data: []byte("hand analysis\n")},
		{id: store.IntentArchiveArtifactSpec, data: []byte("hand specification\n")},
		{id: store.IntentArchiveArtifactExploration, data: []byte("hand exploration\n")},
	}
	replacements := make([]store.IntentArchiveReplacement, 0, len(inputs))
	blobs := make(map[string][]byte, len(inputs))
	for _, input := range inputs {
		replacement := intentArchiveCLIReplacement(
			t, input.id, input.data, store.IntentArchiveWireRetained,
		)
		replacements = append(replacements, replacement)
		blobs[replacement.ContentSHA256] = input.data
	}
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].ArtifactID < replacements[j].ArtifactID
	})
	generations := make([]store.IntentArchiveGeneration, 0, len(replacements))
	for _, replacement := range replacements {
		generations = append(
			generations,
			intentArchiveCLIGeneration(t, slug, replacement),
		)
	}
	sort.Slice(generations, func(i, j int) bool {
		return generations[i].GenerationID < generations[j].GenerationID
	})
	writeIntentArchiveCLIFixture(
		t, root, slug, intentArchiveCLIIndex(t, slug, generations...), blobs,
	)
}

func s7AQCreateCrashJournal(t *testing.T, root, slug, phase string) {
	s7AQCreateModeCrashJournal(t, root, slug, phase, prepareModeRegenerate)
}

func s7AQCreateModeCrashJournal(
	t *testing.T,
	root, slug, phase string,
	mode prepareMode,
) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestS7AQCrashFixtureHelper$")
	command.Env = append(os.Environ(),
		"TPATCH_S7_AQ_CRASH_HELPER=1",
		"TPATCH_S7_AQ_CRASH_ROOT="+root,
		"TPATCH_S7_AQ_CRASH_SLUG="+slug,
		"TPATCH_S7_AQ_CRASH_PHASE="+phase,
		"TPATCH_S7_AQ_CRASH_MODE="+string(mode),
	)
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != s7AQCrashExit {
		t.Fatalf("%s crash helper = err:%v output:%s", phase, err, output)
	}
}

func s7AQExpectedRestoredEntries(
	t *testing.T,
	root string,
	journal intentpub.Journal,
) []string {
	t.Helper()
	identities := make([]intentpub.Identity, len(journal.Entries))
	allNew := true
	for index, entry := range journal.Entries {
		identity := s7APCaptureIdentity(t, root, entry.Rel)
		identities[index] = identity
		if !identity.Equal(entry.NewImage) {
			allNew = false
		}
		if !identity.Equal(entry.NewImage) && !identity.Equal(entry.Preimage) {
			t.Fatalf("AQ crash entry %s is neither preimage nor new image", entry.Rel)
		}
	}
	if allNew {
		return []string{}
	}
	newEntries := make([]intentpub.ArtifactID, 0, len(journal.Entries))
	for index, entry := range journal.Entries {
		identity := identities[index]
		if identity.Equal(entry.NewImage) && !identity.Equal(entry.Preimage) {
			newEntries = append(newEntries, entry.ArtifactID)
		}
	}
	for left, right := 0, len(newEntries)-1; left < right; left, right = left+1, right-1 {
		newEntries[left], newEntries[right] = newEntries[right], newEntries[left]
	}
	return prepareArtifactIDPaths(journal.Slug, newEntries)
}

func s7AQHasAdvisory(report preparePublishReport, code string) bool {
	for _, advisory := range report.Advisories {
		if advisory.Code == code {
			return true
		}
	}
	return false
}

type s7AQJournalBindCase struct {
	name string
	code string
}

func s7AQJournalBindCases() []s7AQJournalBindCase {
	return []s7AQJournalBindCase{
		{name: "J1", code: "journal-corrupt"},
		{name: "J2", code: "journal-corrupt"},
		{name: "J3", code: "journal-version-mismatch"},
		{name: "J4", code: "journal-foreign"},
		{name: "J5", code: "journal-corrupt"},
		{name: "J6", code: "journal-corrupt"},
		{name: "J7", code: "journal-path-escape"},
		{name: "J8", code: "journal-forged"},
		{name: "J9", code: "journal-corrupt"},
		{name: "J10", code: "journal-corrupt"},
	}
}

func s7AQWriteJournalBindFixture(t *testing.T, root, slug, bind string) {
	t.Helper()
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	if err := os.MkdirAll(lane, 0o700); err != nil {
		t.Fatal(err)
	}
	journal := s6JournalFixture(t, root, slug)
	var body []byte
	switch bind {
	case "J1":
		body, _ = json.Marshal(journal)
		body = append(body, []byte(`{}`)...)
	case "J2":
		body, _ = json.Marshal(journal)
		body = bytes.Replace(body, []byte(`{"version":`), []byte(`{"unknown":true,"version":`), 1)
	case "J3":
		journal.Version++
	case "J4":
		journal.Slug = "foreign-feature"
	case "J5":
		journal.Mode = intentpub.ModeManual
	case "J6":
		journal.RunNonce = "ABC"
	case "J7":
		journal.Entries[0].Rel = "../escape"
	case "J8":
		journal.PlanDigest = strings.Repeat("f", 64)
	case "J9":
		journal.Entries = append(journal.Entries, journal.Entries[0])
		journal.PlanDigest, _ = intentpub.PlanDigest(journal.Entries)
	case "J10":
		body, _ = json.Marshal(journal)
		entries, _ := json.Marshal(journal.Entries)
		body = bytes.Replace(body, entries, []byte("null"), 1)
	default:
		t.Fatalf("unknown J-bind %s", bind)
	}
	if body == nil {
		var err error
		body, err = json.Marshal(journal)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(lane, "journal.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

type s7AQRecoveryFailureObservation struct {
	root         string
	slug         string
	code         int
	stderr       string
	report       preparePublishReport
	wantCode     string
	expectedTree string
	resultTree   string
}

func s7AQObserveRecoveryFailure(t *testing.T, shape string) s7AQRecoveryFailureObservation {
	t.Helper()
	root, slug := prepareS4Workspace(t, "AQ recovery failure "+shape)
	prepareS4WriteReadyBundle(t, root, slug, false)
	s7AQCreateCrashJournal(t, root, slug, "CP4")
	journalRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug))))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := intentpub.DecodeJournal(journalRaw, slug)
	if err != nil {
		t.Fatal(err)
	}
	wantCode := "recovery-divergent"
	var expectedTree string
	if shape == "CP9" {
		for _, entry := range journal.Entries {
			if s7APCaptureIdentity(t, root, entry.Rel).Equal(entry.NewImage) {
				if err := os.WriteFile(
					filepath.Join(root, filepath.FromSlash(entry.Rel)),
					[]byte("AQ CP9 third-party bytes\n"), 0o644,
				); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		expectedTree = snapshotTreeMetadata(
			t, "PIB-488 intended evidence", filepath.Join(root, ".tpatch"),
		)
	} else {
		wantCode = "recovery-divergent"
		oldHook := prepareIntentpubHook
		mutated := false
		prepareIntentpubHook = func(
			point intentpub.CrashPoint,
			_ *os.Root,
			entry *intentpub.Entry,
		) error {
			if point == intentpub.PointBeforeUndo && entry != nil && !mutated {
				mutated = true
				if err := os.WriteFile(
					filepath.Join(root, filepath.FromSlash(entry.Rel)),
					[]byte("AQ undo CAS third-party bytes\n"), 0o644,
				); err != nil {
					return err
				}
				expectedTree = snapshotTreeMetadata(
					t, "PIB-488 intended evidence", filepath.Join(root, ".tpatch"),
				)
			}
			return nil
		}
		t.Cleanup(func() { prepareIntentpubHook = oldHook })
	}
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if expectedTree == "" {
		t.Fatalf("PIB-488 %s did not capture the intended post-injection tree", shape)
	}
	return s7AQRecoveryFailureObservation{
		root: root, slug: slug, code: code, stderr: stderr,
		report: report, wantCode: wantCode, expectedTree: expectedTree,
		resultTree: snapshotTreeMetadata(
			t, "PIB-488 intended evidence", filepath.Join(root, ".tpatch"),
		),
	}
}

func validateS7AQTerminalRecoveryControlFlow(source string) error {
	if err := validateS7AQTerminalRecoveryCalls(source); err != nil {
		return err
	}
	file, err := parser.ParseFile(token.NewFileSet(), "prepare_publish.go", source, 0)
	if err != nil {
		return err
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "runPreparePublish" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil {
		return errors.New("runPreparePublish is missing")
	}
	var recoveryBranch *ast.IfStmt
	ast.Inspect(function.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok || recoveryBranch != nil {
			return true
		}
		text := s7AQNodeText(source, branch.Cond)
		if strings.Contains(text, "recovery.Outcome") &&
			strings.Contains(text, "intentpub.OutcomeRecovered") {
			recoveryBranch = branch
			return false
		}
		return true
	})
	if recoveryBranch == nil {
		return errors.New("terminal recovery branch is missing")
	}
	bodyText := s7AQNodeText(source, recoveryBranch.Body)
	if len(recoveryBranch.Body.List) < 3 {
		return errors.New("terminal recovery branch is incomplete")
	}
	returnCount := 0
	ast.Inspect(recoveryBranch.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.ReturnStmt); ok {
			returnCount++
		}
		return true
	})
	if returnCount != 1 {
		return fmt.Errorf("terminal recovery branch has %d returns, want exactly one", returnCount)
	}
	final, ok := recoveryBranch.Body.List[len(recoveryBranch.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(final.Results) != 1 ||
		!strings.Contains(s7AQNodeText(source, final.Results[0]), "emitPreparePublishReport") ||
		!strings.HasSuffix(strings.TrimSpace(s7AQNodeText(source, final.Results[0])), "report, 0)") {
		return errors.New("terminal recovery does not end in the exit-0 report return")
	}
	if strings.Count(bodyText, `report.Outcome = "recovered"`) != 1 ||
		strings.Count(bodyText, `report.Action = "none"`) != 1 {
		return errors.New("terminal recovery does not set the exact recovered/action-none result")
	}
	callbackIndex := -1
	for index, statement := range recoveryBranch.Body.List {
		text := s7AQNodeText(source, statement)
		if strings.Contains(text, "afterRecoveryComplete") {
			if callbackIndex != -1 {
				return errors.New("afterRecoveryComplete appears more than once")
			}
			callbackIndex = index
		}
	}
	if callbackIndex == -1 {
		return errors.New("afterRecoveryComplete ordering seam is missing")
	}
	if callbackIndex != len(recoveryBranch.Body.List)-3 {
		return errors.New("afterRecoveryComplete is not immediately before release and terminal return")
	}
	releaseText := strings.TrimSpace(
		s7AQNodeText(source, recoveryBranch.Body.List[callbackIndex+1]),
	)
	if releaseText != "_ = release()" {
		return fmt.Errorf("terminal recovery release statement = %q", releaseText)
	}
	return nil
}

type s7AQRecoveryCallable struct {
	function *types.Func
	literal  *ast.FuncLit
	dynamic  string
}

type s7AQRecoveryCallAnalyzer struct {
	pkg         *s6TypedPackage
	assignments map[types.Object][]ast.Expr
	functions   map[*types.Func]*ast.FuncDecl
	inventory   map[string]int
}

func validateS7AQTerminalRecoveryCalls(source string) error {
	if err := validateS7AQAfterRecoverySeamAuthority(source); err != nil {
		return err
	}
	model, err := s6BuildSourceTypeModel(map[string]string{
		"internal/cli/prepare_publish.go": source,
	})
	if err != nil {
		return fmt.Errorf("type-check terminal recovery graph: %w", err)
	}
	pkg := model.typedPackages["internal/cli"]
	if pkg == nil || pkg.info == nil {
		return errors.New("typed CLI package is missing")
	}
	file := pkg.relFiles["internal/cli/prepare_publish.go"]
	if file == nil {
		return errors.New("typed prepare_publish.go is missing")
	}
	analyzer := newS7AQRecoveryCallAnalyzer(pkg)
	branch := s7AQTypedRecoveryBranch(file)
	if branch == nil {
		return errors.New("typed terminal recovery branch is missing")
	}
	if err := analyzer.inspectBlock(
		branch.Body, map[types.Object]s7AQRecoveryCallable{}, map[string]bool{}, true,
	); err != nil {
		return err
	}
	want := map[string]int{
		"dynamic.builtin.append": 1,
		"dynamic.builtin.len":    1,
		"github.com/tesseracode/tesserapatch/internal/cli.emitPreparePublishReport":    1,
		"github.com/tesseracode/tesserapatch/internal/cli.inspectPrepareWithAuthority": 1,
		"github.com/tesseracode/tesserapatch/internal/cli.prepareAdvisory":             1,
		"github.com/tesseracode/tesserapatch/internal/cli.prepareArtifactIDPaths":      1,
		"github.com/tesseracode/tesserapatch/internal/cli.prepareArtifactRows":         1,
		"github.com/tesseracode/tesserapatch/internal/cli.prepareContainsArtifactID":   1,
		"github.com/tesseracode/tesserapatch/internal/cli.prepareRetryCommand":         1,
		"github.com/tesseracode/tesserapatch/internal/cli.refreshPrepareFeaturesIndex": 1,
		"fmt.Sprintf":                   1,
		"dynamic.afterRecoveryComplete": 1,
		"local.func-literal":            1,
	}
	if !reflect.DeepEqual(analyzer.inventory, want) {
		return fmt.Errorf("terminal recovery call inventory = %v, want %v",
			analyzer.inventory, want)
	}
	return nil
}

func validateS7AQAfterRecoverySeamAuthority(preparePublishSource string) error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read CLI production sources: %w", err)
	}
	declarations := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		source := preparePublishSource
		if name != "prepare_publish.go" {
			data, readErr := os.ReadFile(name)
			if readErr != nil {
				return fmt.Errorf("read production source %s: %w", name, readErr)
			}
			source = string(data)
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if parseErr != nil {
			return fmt.Errorf("parse production source %s: %w", name, parseErr)
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, raw := range general.Specs {
				spec, _ := raw.(*ast.ValueSpec)
				if spec == nil {
					continue
				}
				for _, identifier := range spec.Names {
					if identifier.Name != "afterRecoveryComplete" {
						continue
					}
					declarations++
					function, exact := spec.Type.(*ast.FuncType)
					if name != "prepare_publish.go" || len(spec.Names) != 1 ||
						len(spec.Values) != 0 || !exact ||
						function.Params == nil || len(function.Params.List) != 0 ||
						function.Results != nil {
						return fmt.Errorf(
							"afterRecoveryComplete is not the exact nil func() seam in prepare_publish.go",
						)
					}
				}
			}
		}
		var mutation ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if mutation != nil {
				return false
			}
			switch value := node.(type) {
			case *ast.AssignStmt:
				for _, left := range value.Lhs {
					if s7AQExpressionNamesRecoverySeam(left) {
						mutation = value
						return false
					}
				}
			case *ast.RangeStmt:
				if value.Tok == token.ASSIGN &&
					(s7AQExpressionNamesRecoverySeam(value.Key) ||
						s7AQExpressionNamesRecoverySeam(value.Value)) {
					mutation = value
					return false
				}
			case *ast.IncDecStmt:
				if s7AQExpressionNamesRecoverySeam(value.X) {
					mutation = value
					return false
				}
			case *ast.UnaryExpr:
				if value.Op == token.AND &&
					s7AQExpressionNamesRecoverySeam(value.X) {
					mutation = value
					return false
				}
			}
			return true
		})
		if mutation != nil {
			return fmt.Errorf(
				"production source %s mutates or escapes afterRecoveryComplete",
				name,
			)
		}
	}
	if declarations != 1 {
		return fmt.Errorf(
			"afterRecoveryComplete production declarations = %d, want exactly one",
			declarations,
		)
	}
	return nil
}

func s7AQExpressionNamesRecoverySeam(expression ast.Expr) bool {
	switch value := expression.(type) {
	case nil:
		return false
	case *ast.Ident:
		return value.Name == "afterRecoveryComplete"
	case *ast.ParenExpr:
		return s7AQExpressionNamesRecoverySeam(value.X)
	case *ast.StarExpr:
		return s7AQExpressionNamesRecoverySeam(value.X)
	case *ast.SelectorExpr:
		return s7AQExpressionNamesRecoverySeam(value.X) ||
			value.Sel.Name == "afterRecoveryComplete"
	case *ast.IndexExpr:
		return s7AQExpressionNamesRecoverySeam(value.X)
	case *ast.IndexListExpr:
		return s7AQExpressionNamesRecoverySeam(value.X)
	default:
		return false
	}
}

func newS7AQRecoveryCallAnalyzer(pkg *s6TypedPackage) *s7AQRecoveryCallAnalyzer {
	analyzer := &s7AQRecoveryCallAnalyzer{
		pkg: pkg, assignments: map[types.Object][]ast.Expr{},
		functions: map[*types.Func]*ast.FuncDecl{},
		inventory: map[string]int{},
	}
	for _, file := range pkg.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if object, _ := pkg.info.Defs[function.Name].(*types.Func); object != nil {
				analyzer.functions[object] = function
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.AssignStmt:
				if len(value.Lhs) != len(value.Rhs) {
					return true
				}
				for index, left := range value.Lhs {
					identifier, _ := left.(*ast.Ident)
					if identifier == nil || identifier.Name == "_" {
						continue
					}
					object := pkg.info.Defs[identifier]
					if object == nil {
						object = pkg.info.Uses[identifier]
					}
					if object != nil {
						analyzer.assignments[object] = append(
							analyzer.assignments[object], value.Rhs[index],
						)
					}
				}
			case *ast.ValueSpec:
				for index, name := range value.Names {
					if index >= len(value.Values) {
						continue
					}
					if object := pkg.info.Defs[name]; object != nil {
						analyzer.assignments[object] = append(
							analyzer.assignments[object], value.Values[index],
						)
					}
				}
			}
			return true
		})
	}
	return analyzer
}

func s7AQTypedRecoveryBranch(file *ast.File) *ast.IfStmt {
	var branch *ast.IfStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if branch != nil {
			return false
		}
		candidate, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		condition := s7APFormatExpression(candidate.Cond)
		if strings.Contains(condition, "recovery.Outcome") &&
			strings.Contains(condition, "intentpub.OutcomeRecovered") {
			branch = candidate
			return false
		}
		return true
	})
	return branch
}

func (analyzer *s7AQRecoveryCallAnalyzer) inspectBlock(
	body *ast.BlockStmt,
	bindings map[types.Object]s7AQRecoveryCallable,
	visiting map[string]bool,
	record bool,
) error {
	var validationErr error
	ast.Inspect(body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		target, resolved := analyzer.resolveCallable(
			call.Fun, bindings, map[types.Object]bool{},
		)
		if !resolved {
			if analyzer.pkg.info.Types[call.Fun].IsType() {
				return true
			}
			validationErr = fmt.Errorf(
				"terminal recovery has unresolved call %s",
				s7APFormatExpression(call.Fun),
			)
			return false
		}
		key := analyzer.callableKey(target)
		if record {
			analyzer.inventory[key]++
		}
		if analyzer.forbidden(target) {
			validationErr = fmt.Errorf("terminal recovery reaches forbidden gate %s", key)
			return false
		}
		if target.dynamic != "" {
			if analyzer.externalLeaf(key) {
				return true
			}
			validationErr = fmt.Errorf("terminal recovery has unclassified dynamic call %s", key)
			return false
		}
		next := analyzer.bindCall(target, call, bindings)
		switch {
		case target.function != nil && analyzer.functions[target.function] != nil:
			functionKey := s7APFunctionKey(target.function)
			if visiting[functionKey] {
				validationErr = fmt.Errorf("terminal recovery callable cycle at %s", key)
				return false
			}
			visiting[functionKey] = true
			validationErr = analyzer.inspectBlock(
				analyzer.functions[target.function].Body, next, visiting, false,
			)
			delete(visiting, functionKey)
		case target.literal != nil:
			validationErr = analyzer.inspectBlock(target.literal.Body, next, visiting, false)
		case analyzer.externalLeaf(key):
			validationErr = analyzer.inspectExternalCallableArguments(
				call, bindings, visiting,
			)
		default:
			validationErr = fmt.Errorf("terminal recovery has unclassified external call %s", key)
		}
		return validationErr == nil
	})
	return validationErr
}

func (analyzer *s7AQRecoveryCallAnalyzer) inspectExternalCallableArguments(
	call *ast.CallExpr,
	bindings map[types.Object]s7AQRecoveryCallable,
	visiting map[string]bool,
) error {
	for _, argument := range call.Args {
		if _, ok := analyzer.pkg.info.TypeOf(argument).Underlying().(*types.Signature); !ok {
			continue
		}
		target, resolved := analyzer.resolveCallable(
			argument, bindings, map[types.Object]bool{},
		)
		if !resolved {
			return fmt.Errorf("terminal recovery external callback %s is unresolved",
				s7APFormatExpression(argument))
		}
		next := analyzer.bindCall(target, call, bindings)
		switch {
		case target.function != nil && analyzer.functions[target.function] != nil:
			key := s7APFunctionKey(target.function)
			if visiting[key] {
				continue
			}
			visiting[key] = true
			err := analyzer.inspectBlock(
				analyzer.functions[target.function].Body, next, visiting, false,
			)
			delete(visiting, key)
			if err != nil {
				return err
			}
		case target.literal != nil:
			if err := analyzer.inspectBlock(target.literal.Body, next, visiting, false); err != nil {
				return err
			}
		default:
			return fmt.Errorf("terminal recovery external callback %s is unclassified",
				analyzer.callableKey(target))
		}
	}
	return nil
}

func (analyzer *s7AQRecoveryCallAnalyzer) resolveCallable(
	expression ast.Expr,
	bindings map[types.Object]s7AQRecoveryCallable,
	visiting map[types.Object]bool,
) (s7AQRecoveryCallable, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return analyzer.resolveCallable(value.X, bindings, visiting)
	case *ast.FuncLit:
		return s7AQRecoveryCallable{literal: value}, true
	case *ast.SelectorExpr:
		function, _ := analyzer.pkg.info.Uses[value.Sel].(*types.Func)
		if function == nil {
			if selection := analyzer.pkg.info.Selections[value]; selection != nil {
				function, _ = selection.Obj().(*types.Func)
			}
		}
		return s7AQRecoveryCallable{function: function}, function != nil
	case *ast.Ident:
		object := analyzer.pkg.info.ObjectOf(value)
		if builtin, _ := object.(*types.Builtin); builtin != nil {
			return s7AQRecoveryCallable{
				dynamic: "builtin." + builtin.Name(),
			}, true
		}
		if function, _ := object.(*types.Func); function != nil {
			return s7AQRecoveryCallable{function: function}, true
		}
		if object == nil || visiting[object] {
			return s7AQRecoveryCallable{}, false
		}
		if bound, ok := bindings[object]; ok {
			return bound, true
		}
		if analyzer.afterRecoveryObject(object) {
			return s7AQRecoveryCallable{dynamic: "afterRecoveryComplete"}, true
		}
		assigned := analyzer.assignments[object]
		if len(assigned) != 1 {
			return s7AQRecoveryCallable{}, false
		}
		visiting[object] = true
		defer delete(visiting, object)
		return analyzer.resolveCallable(assigned[0], bindings, visiting)
	case *ast.CallExpr:
		factory, ok := analyzer.resolveCallable(
			value.Fun, bindings, map[types.Object]bool{},
		)
		if !ok || factory.function == nil {
			return s7AQRecoveryCallable{}, false
		}
		declaration := analyzer.functions[factory.function]
		if declaration == nil {
			return s7AQRecoveryCallable{}, false
		}
		next := analyzer.bindCall(factory, value, bindings)
		var result s7AQRecoveryCallable
		found := false
		valid := true
		s7APInspectFunctionReturns(declaration.Body, func(statement *ast.ReturnStmt) bool {
			if len(statement.Results) == 0 {
				valid = false
				return false
			}
			target, resolved := analyzer.resolveCallable(
				statement.Results[0], next, map[types.Object]bool{},
			)
			if !resolved || found && analyzer.callableKey(target) != analyzer.callableKey(result) {
				valid = false
				return false
			}
			result = target
			found = true
			return true
		})
		return result, found && valid
	default:
		return s7AQRecoveryCallable{}, false
	}
}

func (analyzer *s7AQRecoveryCallAnalyzer) bindCall(
	target s7AQRecoveryCallable,
	call *ast.CallExpr,
	caller map[types.Object]s7AQRecoveryCallable,
) map[types.Object]s7AQRecoveryCallable {
	result := map[types.Object]s7AQRecoveryCallable{}
	var parameters *ast.FieldList
	if target.function != nil {
		if declaration := analyzer.functions[target.function]; declaration != nil {
			parameters = declaration.Type.Params
		}
	} else if target.literal != nil {
		parameters = target.literal.Type.Params
	}
	if parameters == nil {
		return result
	}
	argument := 0
	for _, field := range parameters.List {
		for _, name := range field.Names {
			if argument >= len(call.Args) {
				return result
			}
			object := analyzer.pkg.info.Defs[name]
			if callable, ok := analyzer.resolveCallable(
				call.Args[argument], caller, map[types.Object]bool{},
			); ok && object != nil {
				result[object] = callable
			}
			argument++
		}
	}
	return result
}

func (analyzer *s7AQRecoveryCallAnalyzer) callableKey(
	target s7AQRecoveryCallable,
) string {
	if target.dynamic != "" {
		return "dynamic." + target.dynamic
	}
	if target.function != nil {
		pkg := ""
		if target.function.Pkg() != nil {
			pkg = target.function.Pkg().Path()
		}
		if pkg == "" {
			return target.function.Name()
		}
		return pkg + "." + target.function.Name()
	}
	if target.literal != nil {
		return "local.func-literal"
	}
	return "unresolved"
}

func (analyzer *s7AQRecoveryCallAnalyzer) forbidden(
	target s7AQRecoveryCallable,
) bool {
	if target.function == nil || target.function.Pkg() == nil ||
		target.function.Pkg().Path() != "github.com/tesseracode/tesserapatch/internal/cli" {
		return false
	}
	switch target.function.Name() {
	case "prepareStateRefusal",
		"buildPreparePlan",
		"capturePrepareWithAuthority",
		"generatePrepareBundle",
		"prepareArchivePreflight",
		"stagePreparePublicationBase",
		"stagePrepareArchiveIndex",
		"publishPrepareTransaction":
		return true
	default:
		return false
	}
}

func (analyzer *s7AQRecoveryCallAnalyzer) externalLeaf(key string) bool {
	switch key {
	case "dynamic.builtin.append",
		"dynamic.builtin.len",
		"dynamic.builtin.make",
		"dynamic.afterRecoveryComplete",
		"bytes.TrimSpace",
		"crypto/sha256.Sum256",
		"encoding/hex.EncodeToString",
		"encoding/json.Encode",
		"encoding/json.NewEncoder",
		"encoding/json.SetIndent",
		"encoding/json.Unmarshal",
		"errors.Is",
		"errors.New",
		"fmt.Fprint",
		"fmt.Fprintf",
		"fmt.Fprintln",
		"github.com/spf13/cobra.ErrOrStderr",
		"github.com/spf13/cobra.Flags",
		"github.com/spf13/cobra.OutOrStdout",
		"github.com/spf13/pflag.GetBool",
		"github.com/tesseracode/tesserapatch/internal/intent.Inspect",
		"github.com/tesseracode/tesserapatch/internal/intent.NewRootOps",
		"github.com/tesseracode/tesserapatch/internal/intentlock.WithRoot",
		"github.com/tesseracode/tesserapatch/internal/intentlock.Release",
		"github.com/tesseracode/tesserapatch/internal/intentpub.AbsentIdentity",
		"github.com/tesseracode/tesserapatch/internal/intentpub.CanonicalPath",
		"github.com/tesseracode/tesserapatch/internal/intentpub.CaptureIdentity",
		"github.com/tesseracode/tesserapatch/internal/intentpub.DurableWrite",
		"github.com/tesseracode/tesserapatch/internal/store.ListFeatures",
		"fmt.Sprintf",
		"io.ReadFull",
		"io/fs.IsRegular",
		"io/fs.Mode",
		"io/fs.ModTime",
		"io/fs.Perm",
		"io/fs.Size",
		"io/fs.ValidPath",
		"os.Close",
		"os.Lstat",
		"os.OpenFile",
		"os.SameFile",
		"os.Stat",
		"strings.HasPrefix",
		"strings.Join",
		"strings.NewReplacer",
		"strings.Replace",
		"strings.Split",
		"strings.String",
		"strings.TrimSpace",
		"strings.WriteString",
		"time.Equal",
		"time.String":
		return true
	default:
		return false
	}
}

func (analyzer *s7AQRecoveryCallAnalyzer) afterRecoveryObject(object types.Object) bool {
	return object != nil && object.Name() == "afterRecoveryComplete" &&
		object.Pkg() != nil &&
		object.Pkg().Path() == "github.com/tesseracode/tesserapatch/internal/cli" &&
		object.Parent() == object.Pkg().Scope()
}

func s7AQNodeText(source string, node ast.Node) string {
	if node == nil {
		return ""
	}
	start := int(node.Pos()) - 1
	end := int(node.End()) - 1
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return source[start:end]
}

func s7AQSortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
