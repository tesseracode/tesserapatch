package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

type rocS7CandidateFixture struct {
	root        string
	store       *store.Store
	slug        string
	pointerPath string
	artifact    candidate.CandidateArtifact
	patch       []byte
}

type rocS7FailWriter struct{}

func (rocS7FailWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("forced output failure")
}

func newROCS7CandidateFixture(t *testing.T) *rocS7CandidateFixture {
	t.Helper()
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	gitInitTestRepo(t, root)
	upstream := gitHead(t, root)
	status, err := s.AddFeature(store.AddFeatureInput{Title: "S7 candidate fixture"})
	if err != nil {
		t.Fatal(err)
	}
	status.State = store.StateApplied
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	artifact := rocS7CandidateArtifact(t, status.Slug, upstream)
	pointerPath, err := candidate.CandidatePointerPath(status.Slug, artifact.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	featureLock, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "reconcile", status.Slug+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	globalLock, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock"))
	if err != nil {
		_ = featureLock.Release()
		t.Fatal(err)
	}
	candidateRoot, err := candidate.CreateCandidateRoot(root, status.Slug, artifact.CandidateID, upstream, featureLock, globalLock)
	if err != nil {
		_ = globalLock.Release()
		_ = featureLock.Release()
		t.Fatal(err)
	}
	if err := candidateRoot.Close(); err != nil {
		t.Fatal(err)
	}
	if err := globalLock.Release(); err != nil {
		t.Fatal(err)
	}
	if err := featureLock.Release(); err != nil {
		t.Fatal(err)
	}

	artifactRaw, err := candidate.EncodeCandidateArtifact(artifact)
	if err != nil {
		t.Fatal(err)
	}
	stateRaw, err := candidate.EncodeCandidateState(candidate.CandidateState{
		SchemaVersion:    candidate.CandidateSchemaVersion,
		CandidateID:      artifact.CandidateID,
		Status:           candidate.CandidateStatusPending,
		AcceptValidation: candidate.CandidateAcceptValidationNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	patch := []byte("diff --git a/new.txt b/new.txt\nnew file mode 100644\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+new\n")
	base := filepath.Join(root, filepath.FromSlash(pointerPath))
	if err := os.WriteFile(filepath.Join(base, candidate.CandidateMetadataName), artifactRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, candidate.CandidateStateName), stateRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, candidate.CandidatePatchName), patch, 0o644); err != nil {
		t.Fatal(err)
	}
	status.Reconcile.OperationCandidate = &store.OperationCandidateSummary{
		CandidateID:        artifact.CandidateID,
		Path:               pointerPath,
		Status:             candidate.CandidateStatusPending,
		SourceFeatureState: string(store.StateApplied),
	}
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	return &rocS7CandidateFixture{root: root, store: s, slug: status.Slug, pointerPath: pointerPath, artifact: artifact, patch: patch}
}

func rocS7CandidateArtifact(t *testing.T, slug, upstream string) candidate.CandidateArtifact {
	t.Helper()
	operation := candidate.CandidateOperation{
		Ordinal: 1, Kind: "create-file", Path: "new.txt", EffectOrdinal: 1,
		EffectSHA256: strings.Repeat("a", 64), RunOrdinals: []int{},
		InsertionSHA256: strings.Repeat("b", 64), InsertionSize: 4, Classification: "applicable",
	}
	effect := candidate.CandidateEffectMapEntry{
		EffectOrdinal: 1, EffectSHA256: strings.Repeat("a", 64), Path: "new.txt",
		Resolution: "candidate-applicable", OperationOrdinals: []int{1}, ReasonCodes: []string{},
	}
	bindings := candidate.CandidateBindings{
		CaptureEventSHA256: strings.Repeat("1", 64), CoverageSHA256: strings.Repeat("2", 64),
		PatchSHA256: strings.Repeat("3", 64), RecipeSHA256: strings.Repeat("4", 64),
	}
	id, _, _, err := candidate.ComputeCandidateID(candidate.CandidateIdentityInput{
		SchemaVersion: candidate.CandidateSchemaVersion, Feature: slug, SourceFeatureState: string(store.StateApplied),
		UpstreamCommit: upstream, ReferenceCommit: upstream, Bindings: bindings,
		DerivationMode: candidate.CandidateDerivationModeSingle,
		Operations:     []candidate.CandidateOperation{operation}, EffectMap: []candidate.CandidateEffectMapEntry{effect},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact := candidate.CandidateArtifact{
		SchemaVersion: candidate.CandidateSchemaVersion, CandidateID: id, Feature: slug,
		SourceFeatureState: string(store.StateApplied), UpstreamCommit: upstream, ReferenceCommit: upstream,
		DerivationMode: candidate.CandidateDerivationModeSingle, Bindings: bindings,
		Derivation: candidate.CandidateDerivation{
			EffectCount: 1, CandidateApplicableCount: 1,
			Operations: []candidate.CandidateOperation{operation}, EffectMap: []candidate.CandidateEffectMapEntry{effect},
		},
		Result: candidate.CandidateResult{
			CandidateDiffSHA256: strings.Repeat("d", 64), CandidateTreeSHA256: strings.Repeat("e", 64),
			Validated: candidate.CandidateValidationStructural,
		},
	}
	if err := artifact.Validate(); err != nil {
		t.Fatal(err)
	}
	return artifact
}

func rocS7ReconcileCommand(t *testing.T) *cobra.Command {
	t.Helper()
	root := buildRootCmd()
	cmd, _, err := root.Find([]string{"reconcile"})
	if err != nil {
		t.Fatal(err)
	}
	cmd.InitDefaultHelpFlag()
	return cmd
}

func rocS7BehavioralFlags(cmd *cobra.Command, excludeHelp bool) []string {
	newFlags := map[string]bool{}
	for _, name := range operationCandidateTerminalFlagNames {
		newFlags[name] = true
	}
	if excludeHelp {
		return operationCandidateBehavioralFlags(cmd, newFlags, false)
	}
	var names []string
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "path" || flag.Name == "format" || newFlags[flag.Name] {
			return
		}
		names = append(names, flag.Name)
	})
	sort.Strings(names)
	return names
}

func rocS7ExplicitFlagArg(flag *pflag.Flag) string {
	if flag.NoOptDefVal != "" {
		return "--" + flag.Name
	}
	switch flag.Value.Type() {
	case "duration":
		return "--" + flag.Name + "=1s"
	case "int":
		return "--" + flag.Name + "=1"
	default:
		return "--" + flag.Name + "=fixture"
	}
}

func TestROCS7FlagInventoryAndExclusivity(t *testing.T) {
	cmd := rocS7ReconcileCommand(t)
	for _, name := range operationCandidateTerminalFlagNames {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("new terminal flag --%s is not registered", name)
		}
	}
	terminalNames := append([]string(nil), operationCandidateTerminalFlagNames...)
	terminalNames = append(terminalNames, "accept", "reject", "shadow-diff")
	if len(terminalNames) != 8 {
		t.Fatalf("terminal action inventory = %d, want 8", len(terminalNames))
	}
	for _, name := range terminalNames {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("terminal action --%s is not registered", name)
		}
	}
	behavioral := rocS7BehavioralFlags(cmd, true)
	if len(behavioral) != 15 {
		t.Fatalf("behavioral inventory = %d, want 15: %v", len(behavioral), behavioral)
	}

	refusals := 0
	for i, left := range operationCandidateTerminalFlagNames {
		for _, right := range operationCandidateTerminalFlagNames[i+1:] {
			_, _, code := runCmdExit("reconcile", "--"+left, "a", "--"+right, "b")
			if code != 2 {
				t.Fatalf("--%s + --%s exit %d, want 2", left, right, code)
			}
			refusals++
		}
	}
	for _, terminal := range operationCandidateTerminalFlagNames {
		for _, existing := range behavioral {
			flag := cmd.Flags().Lookup(existing)
			_, _, code := runCmdExit("reconcile", "--"+terminal, "a", rocS7ExplicitFlagArg(flag))
			if code != 2 {
				t.Fatalf("--%s + --%s exit %d, want 2", terminal, existing, code)
			}
			refusals++
		}
	}
	if refusals != 85 {
		t.Fatalf("refusing pairs = %d, want 85", refusals)
	}
}

func TestROCS7HelpExclusionMutationGuard(t *testing.T) {
	cmd := rocS7ReconcileCommand(t)
	if got := len(rocS7BehavioralFlags(cmd, false)); got != 16 {
		t.Fatalf("fixture that counts help should expose 16 behavioral flags, got %d", got)
	}
	if got := 10 + 5*len(rocS7BehavioralFlags(cmd, false)); got != 90 {
		t.Fatalf("fixture that counts help should report 90 refusing pairs, got %d", got)
	}
	if got := 10 + 5*len(rocS7BehavioralFlags(cmd, true)); got != 85 {
		t.Fatalf("production inventory should report 85 refusing pairs, got %d", got)
	}
	for _, terminal := range operationCandidateTerminalFlagNames {
		_, _, code := runCmdExit("reconcile", "--"+terminal, "fixture", "--help")
		if code != 0 {
			t.Fatalf("--%s + --help exit %d, want 0", terminal, code)
		}
	}
}

func TestROCS7SourceDerivedInventoryMutationGuard(t *testing.T) {
	cmd := reconcileCmd()
	cmd.Flags().Bool("future-behavior", false, "mutation fixture")
	if err := cmd.ParseFlags([]string{"--candidate-diff", "fixture", "--future-behavior"}); err != nil {
		t.Fatal(err)
	}
	if err := validateOperationCandidateTerminalFlags(cmd, nil); err == nil ||
		!strings.Contains(err.Error(), "--future-behavior") {
		t.Fatalf("source-derived validator missed synthetic flag: %v", err)
	}

	hardcoded := map[string]bool{}
	for _, name := range rocS7BehavioralFlags(rocS7ReconcileCommand(t), true) {
		hardcoded[name] = true
	}
	if hardcoded["future-behavior"] {
		t.Fatal("hardcoded fixture unexpectedly learned the synthetic flag")
	}
}

func TestROCS7ChangedNotDefaultMutationGuard(t *testing.T) {
	cmd := reconcileCmd()
	if err := cmd.ParseFlags([]string{"--accept-candidate", "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := validateOperationCandidateTerminalFlags(cmd, nil); err != nil {
		t.Fatalf("default-valued flags were mistaken for explicit flags: %v", err)
	}
	for _, name := range []string{"upstream-ref", "timeout", "format"} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil || flag.Changed {
			t.Fatalf("expected unchanged default for --%s", name)
		}
	}

	explicit := reconcileCmd()
	if err := explicit.ParseFlags([]string{"--accept-candidate", "fixture", "--upstream-ref", "HEAD"}); err != nil {
		t.Fatal(err)
	}
	if err := validateOperationCandidateTerminalFlags(explicit, nil); err == nil {
		t.Fatal("explicit --upstream-ref should refuse")
	}
}

func TestROCS7ExistingValidationMessagesUnchanged(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"reconcile", "--accept", "a", "--reject", "b"}, "reconcile: --accept, --reject, --shadow-diff are mutually exclusive"},
		{[]string{"reconcile", "--accept", "a", "--resolve"}, "reconcile: --accept/--reject/--shadow-diff cannot be combined with --resolve/--apply"},
		{[]string{"reconcile", "--apply"}, "reconcile: --apply requires --resolve"},
	}
	for _, tc := range cases {
		_, stderr, _ := runCmdExit(tc.args...)
		if stderr != "error: "+tc.want+"\n" {
			t.Fatalf("%v stderr = %q, want %q", tc.args, stderr, "error: "+tc.want+"\n")
		}
	}
}

func TestROCS7TerminalSlugArityAndAbsentTransactions(t *testing.T) {
	for _, name := range operationCandidateTerminalFlagNames {
		_, _, code := runCmdExit("reconcile", "--"+name)
		if code != 2 {
			t.Fatalf("--%s without slug exit %d, want 2", name, code)
		}
		_, _, code = runCmdExit("reconcile", "--"+name, "one", "two")
		if code != 2 {
			t.Fatalf("--%s with extra positional slug exit %d, want 2", name, code)
		}
	}

	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"accept-candidate", "resume-candidate-recovery"} {
		_, stderr, code := runCmdExit("reconcile", "--path", root, "--"+name, "fixture")
		if code != 2 || !strings.Contains(stderr, "candidate-absent") {
			t.Fatalf("--%s got exit=%d stderr=%q", name, code, stderr)
		}
	}
}

func TestROCS7ExitCodeMapping(t *testing.T) {
	want := map[string]int{
		"candidate-pending": 3, "shadow-pending": 3, "candidate-absent": 2,
		"candidate-not-pending": 3, "candidate-already-accepted": 3,
		"candidate-feature-state-changed": 3, "head-diverged": 2,
		"candidate-cas-failed": 2, "candidate-path-occupied": 2,
		"candidate-binding-changed": 2, "candidate-test-failed": 2,
		"candidate-lock-held": 3, "lock-unsupported": 3,
		"lock-filesystem-unsupported": 3, "recovery-required": 3,
		"coverage-publication-failed": 3, "acceptance-rolled-back": 3,
		"acceptance-rollback-failed": 1,
	}
	for code, expected := range want {
		err := operationCandidateExitError(&workflow.OperationCandidateActionError{Code: code})
		if got := exitCodeFor(err); got != expected {
			t.Errorf("%s exit %d, want %d", code, got, expected)
		}
	}
	if got := exitCodeFor(operationCandidateExitError(fmt.Errorf("internal"))); got != 1 {
		t.Fatalf("unexpected internal error exit %d, want 1", got)
	}
}

func TestROCS7ResolverTerminalStaysResolverOnly(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := s.AddFeature(store.AddFeatureInput{Title: "Resolver only"})
	if err != nil {
		t.Fatal(err)
	}
	status.State = store.StateApplied
	status.Reconcile.OperationCandidate = &store.OperationCandidateSummary{
		CandidateID:        strings.Repeat("a", 64),
		Path:               ".tpatch/reconcile-candidates/resolver-only/" + strings.Repeat("a", 64),
		Status:             candidate.CandidateStatusPending,
		SourceFeatureState: string(store.StateApplied),
	}
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"accept", "reject", "shadow-diff"} {
		_, stderr, code := runCmdExit("reconcile", "--path", root, "--"+name, status.Slug)
		if code != 3 || !strings.Contains(stderr, "candidate-pending") ||
			!strings.Contains(stderr, "--reject-candidate") || !strings.Contains(stderr, "--accept-candidate") {
			t.Fatalf("--%s rerouted candidate: exit=%d stderr=%q", name, code, stderr)
		}
	}
}

func TestROCS7ResolveSweepExpandsBeforeCandidateRefusal(t *testing.T) {
	f := newROCS7CandidateFixture(t)
	_, stderr, code := runCmdExit("reconcile", "--path", f.root, "--resolve", "--allow-dirty")
	if code != 3 || !strings.Contains(stderr, "candidate-pending") ||
		!strings.Contains(stderr, f.slug) || strings.Contains(stderr, "provider") {
		t.Fatalf("empty-args resolver sweep was not refused: exit=%d stderr=%q", code, stderr)
	}

	_, stderr, code = runCmdExit(
		"reconcile", "--path", f.root, "--resolve", "--preflight",
		"--allow-dirty", "--upstream-ref", "HEAD",
	)
	if code != 0 || strings.Contains(stderr, "candidate-pending") {
		t.Fatalf("preflight was routed through candidate refusal: exit=%d stderr=%q", code, stderr)
	}

	_, stderr, code = runCmdExit(
		"reconcile", "--path", f.root, "--resolve", "--check-applied-only",
		"--upstream-ref", "HEAD", f.slug,
	)
	if strings.Contains(stderr, "candidate-pending") {
		t.Fatalf("check-applied-only was routed through candidate refusal: exit=%d stderr=%q", code, stderr)
	}
}

func TestROCS7CandidateDiffRealLifecycle(t *testing.T) {
	f := newROCS7CandidateFixture(t)
	base := filepath.Join(f.root, filepath.FromSlash(f.pointerPath))
	beforeArtifact := mustRead(t, filepath.Join(base, candidate.CandidateMetadataName))
	beforeState := mustRead(t, filepath.Join(base, candidate.CandidateStateName))
	statusPath := filepath.Join(f.root, ".tpatch", "features", f.slug, "status.json")
	beforeStatus := mustRead(t, statusPath)

	stdout, stderr, code := runCmdExit("reconcile", "--path", f.root, "--candidate-diff", f.slug)
	if code != 0 || stderr != "" || stdout != string(f.patch) {
		t.Fatalf("candidate diff: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := mustRead(t, filepath.Join(base, candidate.CandidateMetadataName)); string(got) != string(beforeArtifact) {
		t.Fatal("candidate diff changed candidate.json")
	}
	if got := mustRead(t, filepath.Join(base, candidate.CandidateStateName)); string(got) != string(beforeState) {
		t.Fatal("candidate diff changed state.json")
	}
	if got := mustRead(t, statusPath); string(got) != string(beforeStatus) {
		t.Fatal("candidate diff changed feature status")
	}

	emptyRoot := t.TempDir()
	emptyStore, err := store.Init(emptyRoot)
	if err != nil {
		t.Fatal(err)
	}
	gitInitTestRepo(t, emptyRoot)
	emptyStatus, err := emptyStore.AddFeature(store.AddFeatureInput{Title: "Absent candidate"})
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCmdExit("reconcile", "--path", emptyRoot, "--candidate-diff", emptyStatus.Slug)
	if code != 2 || !strings.Contains(stderr, "candidate-absent") {
		t.Fatalf("absent candidate diff: exit=%d stderr=%q", code, stderr)
	}
}

func TestROCS7RejectCandidateRealLifecycle(t *testing.T) {
	f := newROCS7CandidateFixture(t)
	base := filepath.Join(f.root, filepath.FromSlash(f.pointerPath))
	stdout, stderr, code := runCmdExit("reconcile", "--path", f.root, "--reject-candidate", f.slug)
	if code != 0 || stderr != "" || !strings.Contains(stdout, f.artifact.CandidateID) {
		t.Fatalf("reject candidate: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	status, err := f.store.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.Reconcile.OperationCandidate != nil {
		t.Fatalf("reject left pointer: %+v", status.Reconcile.OperationCandidate)
	}
	state, err := candidate.DecodeCandidateState(mustRead(t, filepath.Join(base, candidate.CandidateStateName)))
	if err != nil || state.Status != candidate.CandidateStatusRejected {
		t.Fatalf("reject state=%+v err=%v", state, err)
	}
	if _, err := os.Stat(filepath.Join(base, candidate.CandidateWorktreeName)); !os.IsNotExist(err) {
		t.Fatalf("reject did not prune worktree: %v", err)
	}
	rejectedPath := filepath.Join(f.root, ".tpatch", "reconcile-candidates", f.slug, candidate.CandidateRejectedName)
	if rejected := string(mustRead(t, rejectedPath)); !strings.Contains(rejected, f.artifact.CandidateID) {
		t.Fatalf("rejected.jsonl missing candidate: %q", rejected)
	}

	notPending := newROCS7CandidateFixture(t)
	notPendingBase := filepath.Join(notPending.root, filepath.FromSlash(notPending.pointerPath))
	accepted, err := candidate.EncodeCandidateState(candidate.CandidateState{
		SchemaVersion: candidate.CandidateSchemaVersion, CandidateID: notPending.artifact.CandidateID,
		Status: candidate.CandidateStatusAccepted, AcceptValidation: candidate.CandidateAcceptValidationNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notPendingBase, candidate.CandidateStateName), accepted, 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCmdExit("reconcile", "--path", notPending.root, "--reject-candidate", notPending.slug)
	if code != 3 || !strings.Contains(stderr, "candidate-not-pending") {
		t.Fatalf("not-pending reject: exit=%d stderr=%q", code, stderr)
	}
}

func TestROCS7ClearCandidateRejectionsRealLifecycle(t *testing.T) {
	f := newROCS7CandidateFixture(t)
	if _, _, code := runCmdExit("reconcile", "--path", f.root, "--reject-candidate", f.slug); code != 0 {
		t.Fatalf("fixture rejection exit=%d", code)
	}
	base := filepath.Join(f.root, filepath.FromSlash(f.pointerPath))
	rejectedPath := filepath.Join(f.root, ".tpatch", "reconcile-candidates", f.slug, candidate.CandidateRejectedName)
	beforeArtifact := mustRead(t, filepath.Join(base, candidate.CandidateMetadataName))
	beforeState := mustRead(t, filepath.Join(base, candidate.CandidateStateName))
	statusPath := filepath.Join(f.root, ".tpatch", "features", f.slug, "status.json")
	beforeStatus := mustRead(t, statusPath)

	_, stderr, code := runCmdExit("reconcile", "--path", f.root, "--clear-candidate-rejections", f.slug)
	if code != 0 || stderr != "" {
		t.Fatalf("clear candidate rejections: exit=%d stderr=%q", code, stderr)
	}
	if got := mustRead(t, rejectedPath); len(got) != 0 {
		t.Fatalf("rejected.jsonl not truncated: %q", got)
	}
	if string(mustRead(t, filepath.Join(base, candidate.CandidateMetadataName))) != string(beforeArtifact) ||
		string(mustRead(t, filepath.Join(base, candidate.CandidateStateName))) != string(beforeState) ||
		string(mustRead(t, statusPath)) != string(beforeStatus) {
		t.Fatal("clear rejections changed state, pointer or candidate metadata")
	}
}

func TestROCS7CandidateLockHeldMapsExitThree(t *testing.T) {
	f := newROCS7CandidateFixture(t)
	lock, err := candidatelock.TryAcquire(filepath.Join(f.root, ".tpatch", "locks", "reconcile", f.slug+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Release() }()
	_, stderr, code := runCmdExit("reconcile", "--path", f.root, "--candidate-diff", f.slug)
	if code != 3 || !strings.Contains(stderr, "candidate-lock-held") {
		t.Fatalf("held candidate lock: exit=%d stderr=%q", code, stderr)
	}
}

func TestROCS7TerminalOutputFailuresExitOne(t *testing.T) {
	cases := []struct {
		name   string
		format string
		args   func(*rocS7CandidateFixture) []string
	}{
		{"diff-json", "json", func(f *rocS7CandidateFixture) []string { return []string{"--candidate-diff", f.slug} }},
		{"reject-human", "human", func(f *rocS7CandidateFixture) []string { return []string{"--reject-candidate", f.slug} }},
		{"clear-human", "human", func(f *rocS7CandidateFixture) []string {
			if _, _, code := runCmdExit("reconcile", "--path", f.root, "--reject-candidate", f.slug); code != 0 {
				t.Fatalf("fixture rejection exit=%d", code)
			}
			return []string{"--clear-candidate-rejections", f.slug}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newROCS7CandidateFixture(t)
			args := []string{"reconcile", "--path", f.root}
			args = append(args, tc.args(f)...)
			if tc.format == "json" {
				args = append(args, "--format", "json")
			}
			root := buildRootCmd()
			root.SetOut(rocS7FailWriter{})
			root.SetArgs(args)
			err := root.Execute()
			if exitCodeFor(err) != 1 || !strings.Contains(err.Error(), "forced output failure") {
				t.Fatalf("output failure: exit=%d err=%v", exitCodeFor(err), err)
			}
		})
	}
}
