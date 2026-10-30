//go:build !windows

package candidate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
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
	effects = reconstructEffects(t, root, baseCommit, patch)

	if err := os.WriteFile(onePath, []byte(baseOne), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoPath, []byte(featureTwo), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "one.go", "two.go")
	candidateGit(t, root, "commit", "-qm", "upstream")
	upstreamCommit := candidateGit(t, root, "rev-parse", "HEAD")

	var err error
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

func reconstructEffects(t *testing.T, repoRoot, baseCommit, patch string) []gitutil.PatchEffect {
	t.Helper()
	rows, err := patchobs.ReconstructOffline(repoRoot, baseCommit, patch)
	if err != nil {
		t.Fatal(err)
	}
	effects := make([]gitutil.PatchEffect, len(rows))
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
	return effects
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

func acquireFeatureLock(t *testing.T, root, slug string) *candidatelock.Handle {
	t.Helper()
	lockPath := filepath.Join(root, ".tpatch", "locks", "reconcile", slug+".lock")
	lock, err := candidatelock.TryAcquire(lockPath)
	if err != nil {
		t.Fatalf("acquire feature lock %s: %v", lockPath, err)
	}
	return lock
}

func acquireGlobalLock(t *testing.T, root string) *candidatelock.Handle {
	t.Helper()
	lockPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")
	lock, err := candidatelock.TryAcquire(lockPath)
	if err != nil {
		t.Fatalf("acquire global lock %s: %v", lockPath, err)
	}
	return lock
}

func releaseLock(t *testing.T, lock *candidatelock.Handle) {
	t.Helper()
	if lock == nil {
		return
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}
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
	featureLock := acquireFeatureLock(t, root, "s4-demo")
	defer releaseLock(t, featureLock)

	result, err := Execute(reference, upstream, patch, effects, "s4-demo", "rc_111111111111", featureLock)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	t.Cleanup(func() {
		if result.Root != nil && !result.Root.closed {
			globalLock := acquireGlobalLock(t, root)
			defer releaseLock(t, globalLock)
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
	featureLock := acquireFeatureLock(t, root, "s4-unknown")
	defer releaseLock(t, featureLock)

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{
			{kind: "create-file", path: "ok.txt", payload: []byte("ok\n"), effectOrdinal: 1},
			{kind: "mystery-op", path: "ok.txt", payload: []byte("x\n"), effectOrdinal: 1},
		},
		effects: []effectClassification{{ordinal: 1, path: "ok.txt", state: applicable}},
	}, "s4-unknown", "rc_222222222222", featureLock)
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
	featureLock := acquireFeatureLock(t, root, "s4-failure")
	defer releaseLock(t, featureLock)

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{
			{kind: "create-file", path: "dup.txt", payload: []byte("first\n"), effectOrdinal: 1},
			{kind: "create-file", path: "dup.txt", payload: []byte("second\n"), effectOrdinal: 1},
		},
		effects: []effectClassification{{ordinal: 1, path: "dup.txt", state: applicable}},
	}, "s4-failure", "rc_333333333333", featureLock)
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
	}, "s4-failure", "rc_444444444444", featureLock)
	if err == nil || !strings.Contains(err.Error(), "forced write failure") {
		t.Fatalf("forced write failure did not abort candidate: %v", err)
	}
	worktree = filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-failure", "rc_444444444444", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("write failure left worktree behind: %v", statErr)
	}
}

func TestS4ExecuteHoldsFeatureBorrowAndReleasesGlobalDuringApply(t *testing.T) {
	root, reference, upstream, patch, effects := executionFixture(t)
	featureLock := acquireFeatureLock(t, root, "s4-lock-window")
	defer releaseLock(t, featureLock)
	featureCopy := *featureLock
	globalPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")

	started := make(chan struct{}, 1)
	unblock := make(chan struct{})
	originalInsert := candidateWriterRegistry["insert-by-boundary"]
	candidateWriterRegistry["insert-by-boundary"] = func(root *CandidateRoot, op operation) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-unblock
		return originalInsert(root, op)
	}
	t.Cleanup(func() { candidateWriterRegistry["insert-by-boundary"] = originalInsert })

	type result struct {
		exec ExecutionResult
		err  error
	}
	done := make(chan result, 1)
	go func() {
		execResult, err := Execute(reference, upstream, patch, effects, "s4-lock-window", "rc_e55555555555", featureLock)
		done <- result{exec: execResult, err: err}
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for execution apply phase")
	}
	parallelGlobal, err := candidatelock.TryAcquire(globalPath)
	if err != nil {
		t.Fatalf("expected global lock to be acquirable during apply, got %v", err)
	}
	if releaseErr := featureCopy.Release(); releaseErr == nil || !strings.Contains(releaseErr.Error(), "borrowed") {
		t.Fatalf("feature lock copy release should fail while Execute is running, got %v", releaseErr)
	}
	releaseLock(t, parallelGlobal)
	close(unblock)

	out := <-done
	if out.err != nil {
		t.Fatalf("Execute: %v", out.err)
	}
	t.Cleanup(func() {
		if out.exec.Root != nil && !out.exec.Root.closed {
			global := acquireGlobalLock(t, root)
			defer releaseLock(t, global)
			_ = out.exec.Root.Prune(featureLock, global)
		}
	})
}

func TestS4ExecuteRefusesBeforeMutationWhenFeatureCapabilityIsInvalid(t *testing.T) {
	root, reference, upstream, patch, effects := executionFixture(t)
	for _, tc := range []struct {
		name         string
		slug         string
		lock         *candidatelock.Handle
		prepare      func(*testing.T) *candidatelock.Handle
		wantContains string
	}{
		{
			name:         "nil-lock",
			slug:         "s4-invalid-nil",
			lock:         nil,
			wantContains: "lock handle is not initialized",
		},
		{
			name: "released-lock",
			slug: "s4-invalid-released",
			prepare: func(t *testing.T) *candidatelock.Handle {
				lock := acquireFeatureLock(t, root, "s4-invalid-released")
				releaseLock(t, lock)
				return lock
			},
			wantContains: "lock handle is released",
		},
		{
			name: "slug-mismatch",
			slug: "s4-invalid-expected",
			prepare: func(t *testing.T) *candidatelock.Handle {
				return acquireFeatureLock(t, root, "s4-invalid-other")
			},
			wantContains: "lock handle path mismatch",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.lock
			if tc.prepare != nil {
				lock = tc.prepare(t)
				if lock != nil && lock.Path() != "" {
					t.Cleanup(func() {
						_ = lock.Release()
					})
				}
			}
			_, err := Execute(reference, upstream, patch, effects, tc.slug, "rc_f66666666666", lock)
			if err == nil || !strings.Contains(err.Error(), tc.wantContains) {
				t.Fatalf("Execute error = %v, want contains %q", err, tc.wantContains)
			}
			worktree := filepath.Join(root, ".tpatch", "reconcile-candidates", tc.slug, "rc_f66666666666", "worktree")
			if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
				t.Fatalf("invalid feature capability created candidate worktree: %v", statErr)
			}
		})
	}
}

func TestS4ExecuteCreateAndPruneRespectGlobalLockWindows(t *testing.T) {
	root, reference, upstream, patch, effects := executionFixture(t)
	featureLock := acquireFeatureLock(t, root, "s4-global-window")
	defer releaseLock(t, featureLock)

	globalHeld := acquireGlobalLock(t, root)
	_, err := Execute(reference, upstream, patch, effects, "s4-global-window", "rc_011111111111", featureLock)
	if err == nil || !candidatelock.IsCode(err, candidatelock.CodeCandidateLockHeld) {
		t.Fatalf("expected create path to fail with global lock held, got %v", err)
	}
	releaseLock(t, globalHeld)
	worktree := filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-global-window", "rc_011111111111", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("create failure left candidate worktree: %v", statErr)
	}

	started := make(chan struct{}, 1)
	unblock := make(chan struct{})
	originalInsert := candidateWriterRegistry["insert-by-boundary"]
	candidateWriterRegistry["insert-by-boundary"] = func(root *CandidateRoot, op operation) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-unblock
		return fmt.Errorf("forced write failure")
	}
	t.Cleanup(func() { candidateWriterRegistry["insert-by-boundary"] = originalInsert })

	errs := make(chan error, 1)
	go func() {
		_, runErr := Execute(reference, upstream, patch, effects, "s4-global-window", "rc_022222222222", featureLock)
		errs <- runErr
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for writer failure window")
	}
	globalHeld = acquireGlobalLock(t, root)
	close(unblock)
	runErr := <-errs
	releaseLock(t, globalHeld)
	if runErr == nil || !strings.Contains(runErr.Error(), "forced write failure") ||
		!strings.Contains(runErr.Error(), "acquire worktree mutation lock for candidate prune") {
		t.Fatalf("expected combined write/prune lock failure, got %v", runErr)
	}
	worktree = filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-global-window", "rc_022222222222", "worktree")
	if _, statErr := os.Stat(worktree); statErr != nil {
		t.Fatalf("expected candidate to remain after prune lock failure, stat=%v", statErr)
	}
}

func TestS4ExecuteWithPlanRejectsRepositoryRootMismatch(t *testing.T) {
	repoA, headA := initCandidateRepo(t)
	repoB, headB := initCandidateRepo(t)
	reference, err := OpenReferenceRoot(repoA, headA)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(repoB, headB)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	featureLock := acquireFeatureLock(t, repoA, "s4-mismatch")
	defer releaseLock(t, featureLock)

	_, err = executeWithPlan(reference, upstream, "", nil, Plan{
		operations: []operation{{kind: "create-file", path: "x.txt", payload: []byte("x\n"), effectOrdinal: 1}},
		effects:    []effectClassification{{ordinal: 1, path: "x.txt", state: applicable}},
	}, "s4-mismatch", "rc_033333333333", featureLock)
	if err == nil || !strings.Contains(err.Error(), "same repository root") {
		t.Fatalf("expected repository root mismatch refusal, got %v", err)
	}
	worktree := filepath.Join(repoA, ".tpatch", "reconcile-candidates", "s4-mismatch", "rc_033333333333", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("root mismatch created candidate worktree: %v", statErr)
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
	featureLock := acquireFeatureLock(t, root, "s4-guards")
	defer releaseLock(t, featureLock)

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
	}, "s4-guards", "rc_555555555555", featureLock)
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
	}, "s4-guards", "rc_666666666666", featureLock)
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
	}, "s4-guards", "rc_777777777777", featureLock)
	if refusalCode(err) != "candidate-conflict-marker" {
		t.Fatalf("conflict marker refusal mismatch: %v", err)
	}
}

func TestS4ExecuteRefusesPostconditionWhenInsertedPayloadIsCorrupted(t *testing.T) {
	root, reference, upstream, patch, effects := executionFixture(t)
	featureLock := acquireFeatureLock(t, root, "s4-postcondition-insert")
	defer releaseLock(t, featureLock)

	originalInsert := candidateWriterRegistry["insert-by-boundary"]
	candidateWriterRegistry["insert-by-boundary"] = func(root *CandidateRoot, op operation) error {
		mutated := op
		mutated.payload = append([]byte(nil), op.payload...)
		if len(mutated.payload) > 0 {
			mutated.payload[0] = mutated.payload[0] ^ 0x1
		}
		return originalInsert(root, mutated)
	}
	t.Cleanup(func() { candidateWriterRegistry["insert-by-boundary"] = originalInsert })

	_, err := Execute(reference, upstream, patch, effects, "s4-postcondition-insert", "rc_a11111111111", featureLock)
	if refusalCode(err) != "candidate-postcondition-failed" {
		t.Fatalf("expected candidate-postcondition-failed, got %v", err)
	}
	var refusalErr *refusal
	if !errors.As(err, &refusalErr) {
		t.Fatalf("expected refusal error, got %T", err)
	}
	if !strings.Contains(refusalErr.detail, "inserted payload mismatch") {
		t.Fatalf("expected inserted payload mismatch detail, got %q", refusalErr.detail)
	}
	worktree := filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-postcondition-insert", "rc_a11111111111", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("postcondition failure left candidate worktree behind: %v", statErr)
	}
}

func TestS4ExecuteRefusesPostconditionWhenCreatedPayloadIsCorrupted(t *testing.T) {
	root, baseCommit := initCandidateRepo(t)
	createdPath := filepath.Join(root, "created.txt")
	if err := os.WriteFile(createdPath, []byte("created payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "created.txt")
	candidateGit(t, root, "commit", "-qm", "feature")
	patch := string(derivationGitBytes(t, root, "diff", "--binary", baseCommit, "HEAD", "--", "created.txt"))
	effects := reconstructEffects(t, root, baseCommit, patch)

	candidateGit(t, root, "rm", "-q", "created.txt")
	candidateGit(t, root, "commit", "-qm", "upstream")
	upstreamCommit := candidateGit(t, root, "rev-parse", "HEAD")

	reference, err := OpenReferenceRoot(root, baseCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(root, upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()

	featureLock := acquireFeatureLock(t, root, "s4-postcondition-create")
	defer releaseLock(t, featureLock)
	originalCreate := candidateWriterRegistry["create-file"]
	candidateWriterRegistry["create-file"] = func(root *CandidateRoot, op operation) error {
		mutated := op
		mutated.payload = append([]byte(nil), op.payload...)
		if len(mutated.payload) > 0 {
			mutated.payload[0] = mutated.payload[0] ^ 0x1
		}
		return originalCreate(root, mutated)
	}
	t.Cleanup(func() { candidateWriterRegistry["create-file"] = originalCreate })

	_, err = Execute(reference, upstream, patch, effects, "s4-postcondition-create", "rc_b22222222222", featureLock)
	if refusalCode(err) != "candidate-postcondition-failed" {
		t.Fatalf("expected candidate-postcondition-failed, got %v", err)
	}
	var refusalErr *refusal
	if !errors.As(err, &refusalErr) {
		t.Fatalf("expected refusal error, got %T", err)
	}
	if !strings.Contains(refusalErr.detail, "created payload mismatch") {
		t.Fatalf("expected created payload mismatch detail, got %q", refusalErr.detail)
	}
	worktree := filepath.Join(root, ".tpatch", "reconcile-candidates", "s4-postcondition-create", "rc_b22222222222", "worktree")
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("postcondition failure left candidate worktree behind: %v", statErr)
	}
}

func TestS4ExecuteReportsUnmodeledChangeWhenPayloadsRemainIntact(t *testing.T) {
	root, reference, upstream, patch, effects := executionFixture(t)
	featureLock := acquireFeatureLock(t, root, "s4-unmodeled")
	defer releaseLock(t, featureLock)

	originalInsert := candidateWriterRegistry["insert-by-boundary"]
	candidateWriterRegistry["insert-by-boundary"] = func(root *CandidateRoot, op operation) error {
		if err := originalInsert(root, op); err != nil {
			return err
		}
		body, err := root.ReadFile(op.path)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return nil
		}
		body[0] = body[0] ^ 0x1
		return writeRootedFile(root.worktree.root, op.path, body)
	}
	t.Cleanup(func() { candidateWriterRegistry["insert-by-boundary"] = originalInsert })

	_, err := Execute(reference, upstream, patch, effects, "s4-unmodeled", "rc_c33333333333", featureLock)
	if refusalCode(err) != "candidate-unmodeled-change" {
		t.Fatalf("expected candidate-unmodeled-change, got %v", err)
	}
}

func TestS4ExecuteDescendingInsertOffsetsPassPostconditionPositionChecks(t *testing.T) {
	root, _ := initCandidateRepo(t)
	targetPath := filepath.Join(root, "multi.txt")
	if err := os.WriteFile(targetPath, []byte("a\nb\nc\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "multi.txt")
	candidateGit(t, root, "commit", "-qm", "reference")
	baseCommit := candidateGit(t, root, "rev-parse", "HEAD")

	if err := os.WriteFile(targetPath, []byte("a\nx\nb\nc\ny\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "multi.txt")
	candidateGit(t, root, "commit", "-qm", "feature")
	patch := string(derivationGitBytes(t, root, "diff", "--binary", baseCommit, "HEAD", "--", "multi.txt"))
	effects := reconstructEffects(t, root, baseCommit, patch)

	if err := os.WriteFile(targetPath, []byte("a\nb\nc\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "multi.txt")
	candidateGit(t, root, "commit", "-qm", "upstream")
	upstreamCommit := candidateGit(t, root, "rev-parse", "HEAD")

	reference, err := OpenReferenceRoot(root, baseCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(root, upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()

	featureLock := acquireFeatureLock(t, root, "s4-multi-offsets")
	defer releaseLock(t, featureLock)
	result, err := Execute(reference, upstream, patch, effects, "s4-multi-offsets", "rc_d44444444444", featureLock)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	t.Cleanup(func() {
		if result.Root != nil && !result.Root.closed {
			globalLock := acquireGlobalLock(t, root)
			defer releaseLock(t, globalLock)
			_ = result.Root.Prune(featureLock, globalLock)
		}
	})
	body, err := result.Root.ReadFile("multi.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "a\nx\nb\nc\ny\nd\n" {
		t.Fatalf("multi.txt body=%q", string(body))
	}
}

func TestS4ExecuteReturnsNotIdempotentWhenSecondPassReappliesApplicableEffect(t *testing.T) {
	root, _ := initCandidateRepo(t)
	targetPath := filepath.Join(root, "target.txt")
	if err := os.WriteFile(targetPath, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "target.txt")
	candidateGit(t, root, "commit", "-qm", "reference")
	baseCommit := candidateGit(t, root, "rev-parse", "HEAD")

	if err := os.WriteFile(targetPath, []byte("a\nb\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "target.txt")
	candidateGit(t, root, "commit", "-qm", "feature")
	patch := string(derivationGitBytes(t, root, "diff", "--binary", baseCommit, "HEAD", "--", "target.txt"))
	effects := reconstructEffects(t, root, baseCommit, patch)

	if err := os.WriteFile(targetPath, []byte("a\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "target.txt")
	candidateGit(t, root, "commit", "-qm", "upstream")
	upstreamCommit := candidateGit(t, root, "rev-parse", "HEAD")

	reference, err := OpenReferenceRoot(root, baseCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	upstream, err := OpenReferenceRoot(root, upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()

	featureLock := acquireFeatureLock(t, root, "s4-reapply")
	defer releaseLock(t, featureLock)
	_, err = Execute(reference, upstream, patch, effects, "s4-reapply", "rc_888888888888", featureLock)
	if refusalCode(err) != "candidate-not-idempotent" {
		t.Fatalf("expected candidate-not-idempotent, got %v", err)
	}
	var refusalErr *refusal
	if !errors.As(err, &refusalErr) {
		t.Fatalf("expected refusal error, got %T", err)
	}
	if !strings.Contains(refusalErr.detail, "target.txt") || !strings.Contains(refusalErr.detail, "state=applicable") {
		t.Fatalf("idempotency refusal missing effect diagnostics: %q", refusalErr.detail)
	}
}

func TestS4ModeledContentValidationSupportsMultiPathAndMultiOperationPlans(t *testing.T) {
	root, head := initCandidateRepo(t)
	upstream, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	featureLock, globalLock := acquireCandidateLocks(t, root, "s4-modeled")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	plan := Plan{
		operations: []operation{
			{kind: "create-file", path: "a.txt", payload: []byte("a1\n"), effectOrdinal: 1},
			{kind: "insert-by-boundary", path: "a.txt", byteOffset: len("a1\n"), payload: []byte("a2\n"), effectOrdinal: 1},
			{kind: "create-file", path: "b.txt", payload: []byte("b1\n"), effectOrdinal: 2},
			{kind: "insert-by-boundary", path: "b.txt", byteOffset: len("b1\n"), payload: []byte("b2\n"), effectOrdinal: 2},
		},
		effects: []effectClassification{
			{ordinal: 1, path: "a.txt", state: applicable},
			{ordinal: 2, path: "b.txt", state: applicable},
		},
	}
	candidateRoot, err := CreateCandidateRoot(root, "s4-modeled", "rc_999999999999", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	t.Cleanup(func() {
		if candidateRoot != nil && !candidateRoot.closed {
			_ = candidateRoot.Prune(featureLock, globalLock)
		}
	})
	if err := applyCandidateOperations(candidateRoot, plan.operations); err != nil {
		t.Fatalf("applyCandidateOperations: %v", err)
	}
	if err := validateExpectedModeledContent(upstream, candidateRoot, plan); err != nil {
		t.Fatalf("validateExpectedModeledContent: %v", err)
	}

	aBody, err := candidateRoot.ReadFile("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(aBody) != "a1\na2\n" {
		t.Fatalf("a.txt body=%q, want %q", string(aBody), "a1\na2\n")
	}
	bBody, err := candidateRoot.ReadFile("b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(bBody) != "b1\nb2\n" {
		t.Fatalf("b.txt body=%q, want %q", string(bBody), "b1\nb2\n")
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
