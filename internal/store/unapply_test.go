package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnappliedIsValidFeatureState(t *testing.T) {
	if !ValidFeatureState(StateUnapplied) {
		t.Fatal("StateUnapplied must be accepted by ValidFeatureState")
	}
	if StateUnapplied != "unapplied" {
		t.Fatalf("wire value = %q, want unapplied", StateUnapplied)
	}
}

func TestAllFeatureStatesRemainValid(t *testing.T) {
	states := []FeatureState{
		StateRequested,
		StateAnalyzed,
		StateDefined,
		StateImplementing,
		StateApplied,
		StateActive,
		StateReconciling,
		StateReconcilingShadow,
		StateBlocked,
		StateUpstreamMerged,
		StateRejected,
		StateUnapplied,
	}
	if len(states) != 12 {
		t.Fatalf("state inventory has %d entries, want 12", len(states))
	}
	for _, state := range states {
		if !ValidFeatureState(state) {
			t.Errorf("ValidFeatureState(%q) = false", state)
		}
	}
}

func TestWriteFileAtomicRenameFailurePreservesTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "status.json")
	before := []byte("{\"state\":\"applied\"}\n")
	if err := os.WriteFile(target, before, 0o644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("rename failed")
	err := writeFileAtomicWithRename(target, []byte("{\"state\":\"unapplied\"}\n"), 0o644, func(_, _ string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("target changed after failed rename:\n got %q\nwant %q", after, before)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, ".status.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary files remain after failed atomic write: %v", leftovers)
	}
}

func TestWriteFileAtomicPreservesExistingMode(t *testing.T) {
	target := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(target, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestMarkFeatureStateOnlyApplyLeavesUnapplied(t *testing.T) {
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	status, err := s.AddFeature(AddFeatureInput{Title: "Feature", Slug: "feature", Request: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	status.State = StateUnapplied
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState(status.Slug, StateImplementing, "implement", "bad transition"); err == nil {
		t.Fatal("unapplied -> implementing must be refused")
	}
	after, err := s.LoadFeatureStatus(status.Slug)
	if err != nil || after.State != StateUnapplied {
		t.Fatalf("state after refusal = %q, %v", after.State, err)
	}
	if err := s.MarkFeatureState(status.Slug, StateApplied, "apply", "reapplied"); err != nil {
		t.Fatalf("unapplied -> applied must be allowed: %v", err)
	}
}
