//go:build !windows

package candidate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

func executionFixture(t *testing.T) (root string, reference, upstream *ReferenceRoot, patch string, effects []gitutil.PatchEffect) {
	t.Helper()
	root, _ = initCandidateRepo(t)
	onePath := filepath.Join(root, "one.go")
	twoPath := filepath.Join(root, "two.go")
	baseOne := "package demo\n\nfunc one() []string {\n\treturn []string{\n\t\t\"base\",\n\t}\n}\n"
	baseTwo := "package demo\n\nfunc two() []string {\n\treturn []string{\n\t\t\"base\",\n\t}\n}\n"
	if err := os.WriteFile(onePath, []byte(baseOne), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoPath, []byte(baseTwo), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "one.go", "two.go")
	candidateGit(t, root, "commit", "-qm", "reference")
	baseCommit := candidateGit(t, root, "rev-parse", "HEAD")

	featureOne := "package demo\n\nfunc one() []string {\n\treturn []string{\n\t\t\"base\",\n\t\t\"feature\",\n\t}\n}\n"
	featureTwo := "package demo\n\nfunc two() []string {\n\treturn []string{\n\t\t\"base\",\n\t\t\"feature\",\n\t}\n}\n"
	if err := os.WriteFile(onePath, []byte(featureOne), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoPath, []byte(featureTwo), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "one.go", "two.go")
	candidateGit(t, root, "commit", "-qm", "feature")
	patch = string(derivationGitBytes(t, root, "diff", "--binary", baseCommit, "HEAD", "--", "one.go", "two.go"))
	rows, err := patchobs.ReconstructOffline(root, baseCommit, patch)
	if err != nil {
		t.Fatal(err)
	}
	effects = make([]gitutil.PatchEffect, len(rows))
	for i, row := range rows {
		e := row.Effect
		e.PreimageObserved, e.PreimagePresent, e.PreimageSHA256 = true, row.Pre.Present, row.Pre.SHA256
		e.PostimageObserved, e.PostimagePresent, e.PostimageSHA256 = true, row.Post.Present, row.Post.SHA256
		e.OldMode, e.NewMode = row.Pre.Mode, row.Post.Mode
		entry := patchobs.EffectObservation{Effect: e}
		entry.Bytes.Preimage, entry.Bytes.Postimage = row.Pre.Bytes, row.Post.Bytes
		e.ContentKind, e.ObjectKind = patchobs.ClassifyEffectObservation(e, entry.Bytes)
		effects[i] = e
	}

	if err := os.WriteFile(onePath, []byte(baseOne), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoPath, []byte(featureTwo), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "one.go", "two.go")
	candidateGit(t, root, "commit", "-qm", "upstream")
	upstreamCommit := candidateGit(t, root, "rev-parse", "HEAD")

	reference, err = OpenReferenceRoot(root, baseCommit)
	if err != nil {
		t.Fatal(err)
	}
	upstream, err = OpenReferenceRoot(root, upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reference.Close()
		_ = upstream.Close()
	})
	return root, reference, upstream, patch, effects
}

func trackedBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := candidateGit(t, root, "ls-files", "-z")
	paths := strings.Split(out, "\x00")
	result := map[string]string{}
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		result[rel] = string(body)
	}
	return result
}

func TestS4ExecuteCandidateAppliesMixedEffectsAndLeavesLiveTrackedStateUntouched(t *testing.T) {
	root, reference, upstream, patch, effects := executionFixture(t)
	beforeHead := candidateGit(t, root, "rev-parse", "HEAD")
	beforeTracked := trackedBytes(t, root)
	beforeIndex, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	beforeHeadFile, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	featureLock, globalLock := acquireCandidateLocks(t, root, "s4-demo")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	result, err := Execute(reference, upstream, patch, effects, root, "s4-demo", "rc_111111111111", featureLock, globalLock)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	t.Cleanup(func() {
		if result.Root != nil && !result.Root.closed {
			_ = result.Root.Prune(featureLock, globalLock)
		}
	})
	presentCount, applicableCount, conflictCount := result.Plan.Counts()
	if presentCount != 1 || applicableCount != 1 || conflictCount != 0 {
		t.Fatalf("counts = %d/%d/%d, want 1/1/0", presentCount, applicableCount, conflictCount)
	}
	var states []string
	for _, effect := range result.EffectMap {
		states = append(states, effect.State)
	}
	slices.Sort(states)
	if fmt.Sprint(states) != fmt.Sprint([]string{"absorbed-present", "candidate-applicable"}) {
		t.Fatalf("effect map states = %v", states)
	}

	oneBody, err := result.Root.ReadFile("one.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(oneBody), "\"feature\"") {
		t.Fatalf("candidate one.go does not include feature insertion: %q", oneBody)
	}
	twoBody, err := result.Root.ReadFile("two.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(twoBody), "\"feature\"") {
		t.Fatalf("candidate two.go lost absorbed insertion: %q", twoBody)
	}
	if afterHead := candidateGit(t, root, "rev-parse", "HEAD"); afterHead != beforeHead {
		t.Fatalf("live HEAD changed: before=%s after=%s", beforeHead, afterHead)
	}
	afterTracked := trackedBytes(t, root)
	if fmt.Sprint(afterTracked) != fmt.Sprint(beforeTracked) {
		t.Fatalf("live tracked bytes changed: before=%v after=%v", beforeTracked, afterTracked)
	}
	afterIndex, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterIndex) != string(beforeIndex) {
		t.Fatal("live index changed during candidate generation")
	}
	afterHeadFile, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterHeadFile) != string(beforeHeadFile) {
		t.Fatal("live HEAD ref file changed during candidate generation")
	}
	if _, err := os.Stat(filepath.Join(root, ".tpatch", "shadow")); err == nil {
		entries, readErr := os.ReadDir(filepath.Join(root, ".tpatch", "shadow"))
		if readErr == nil && len(entries) != 0 {
			t.Fatal("candidate generation touched resolver shadows")
		}
	}
}

func TestS4UnknownOperationKindFailsClosedAndAbandonsCandidate(t *testing.T) {
	root, head := initCandidateRepo(t)
	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	featureLock, globalLock := acquireCandidateLocks(t, root, "s4-unknown")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{
			{kind: "create-file", path: "ok.txt", payload: []byte("ok\n"), effectOrdinal: 1},
			{kind: "mystery-op", path: "ok.txt", payload: []byte("x\n"), effectOrdinal: 1},
		},
		effects: []effectClassification{{ordinal: 1, path: "ok.txt", state: applicable}},
	}, root, "s4-unknown", "rc_222222222222", featureLock, globalLock)
	if refusalCode(err) != "candidate-operation-kind-unsupported" {
		t.Fatalf("unexpected refusal: %v", err)
	}
	worktree := filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-unknown", "rc_222222222222", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("abandoned candidate worktree still exists: %v", statErr)
	}
}

func TestS4PreconditionAndWriteFailureAbandonWholeCandidate(t *testing.T) {
	root, head := initCandidateRepo(t)
	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	featureLock, globalLock := acquireCandidateLocks(t, root, "s4-failure")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{
			{kind: "create-file", path: "dup.txt", payload: []byte("first\n"), effectOrdinal: 1},
			{kind: "create-file", path: "dup.txt", payload: []byte("second\n"), effectOrdinal: 1},
		},
		effects: []effectClassification{{ordinal: 1, path: "dup.txt", state: applicable}},
	}, root, "s4-failure", "rc_333333333333", featureLock, globalLock)
	if refusalCode(err) != "candidate-postcondition-failed" {
		t.Fatalf("duplicate create did not fail as precondition refusal: %v", err)
	}
	worktree := filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-failure", "rc_333333333333", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("precondition failure left worktree behind: %v", statErr)
	}

	originalInsert := candidateWriterRegistry["insert-by-boundary"]
	candidateWriterRegistry["insert-by-boundary"] = func(root *CandidateRoot, op operation) error {
		return fmt.Errorf("forced write failure")
	}
	t.Cleanup(func() { candidateWriterRegistry["insert-by-boundary"] = originalInsert })

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{
			{kind: "create-file", path: "seed.txt", payload: []byte("seed\n"), effectOrdinal: 1},
			{kind: "insert-by-boundary", path: "seed.txt", byteOffset: len("seed\n"), payload: []byte("tail\n"), effectOrdinal: 1},
		},
		effects: []effectClassification{{ordinal: 1, path: "seed.txt", state: applicable}},
	}, root, "s4-failure", "rc_444444444444", featureLock, globalLock)
	if err == nil || !strings.Contains(err.Error(), "forced write failure") {
		t.Fatalf("forced write failure did not abort candidate: %v", err)
	}
	worktree = filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-failure", "rc_444444444444", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("write failure left worktree behind: %v", statErr)
	}
}

func TestS4ValidationRefusesPathSetModeAndConflictMarkers(t *testing.T) {
	root, head := initCandidateRepo(t)
	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	featureLock, globalLock := acquireCandidateLocks(t, root, "s4-guards")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	originalCreate := candidateWriterRegistry["create-file"]
	candidateWriterRegistry["create-file"] = func(root *CandidateRoot, op operation) error {
		if err := originalCreate(root, op); err != nil {
			return err
		}
		return os.Chmod(filepath.Join(root.WorktreePath(), op.path), 0o755)
	}
	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{{kind: "create-file", path: "mode.txt", payload: []byte("x\n"), effectOrdinal: 1}},
		effects:    []effectClassification{{ordinal: 1, path: "mode.txt", state: applicable}},
	}, root, "s4-guards", "rc_555555555555", featureLock, globalLock)
	candidateWriterRegistry["create-file"] = originalCreate
	if refusalCode(err) != "candidate-mode-drift" {
		t.Fatalf("mode drift refusal mismatch: %v", err)
	}

	originalCreate = candidateWriterRegistry["create-file"]
	candidateWriterRegistry["create-file"] = func(root *CandidateRoot, op operation) error {
		if err := originalCreate(root, op); err != nil {
			return err
		}
		return writeRootedFile(root.worktree.root, "extra.txt", []byte("extra\n"))
	}
	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{{kind: "create-file", path: "path.txt", payload: []byte("x\n"), effectOrdinal: 1}},
		effects:    []effectClassification{{ordinal: 1, path: "path.txt", state: applicable}},
	}, root, "s4-guards", "rc_666666666666", featureLock, globalLock)
	candidateWriterRegistry["create-file"] = originalCreate
	if refusalCode(err) != "candidate-path-set-mismatch" {
		t.Fatalf("path-set mismatch refusal mismatch: %v", err)
	}

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{{
			kind: "create-file", path: "markers.txt",
			payload: []byte("<<<<<<< ours\n=======\n>>>>>>> theirs\n"), effectOrdinal: 1,
		}},
		effects: []effectClassification{{ordinal: 1, path: "markers.txt", state: applicable}},
	}, root, "s4-guards", "rc_777777777777", featureLock, globalLock)
	if refusalCode(err) != "candidate-conflict-marker" {
		t.Fatalf("conflict marker refusal mismatch: %v", err)
	}
}

func TestS4IdempotencyRefusalCoversNonPresentSecondPass(t *testing.T) {
	err := validateSecondPassPlan(Plan{
		effects: []effectClassification{
			{ordinal: 1, path: "a.txt", state: present},
			{ordinal: 2, path: "b.txt", state: applicable},
		},
	}, 2)
	if refusalCode(err) != "candidate-not-idempotent" {
		t.Fatalf("expected candidate-not-idempotent, got %v", err)
	}
}
