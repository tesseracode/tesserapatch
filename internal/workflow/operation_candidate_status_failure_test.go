package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestROCS6AllPresentStatusWriteFailureIsReturned(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6Feature)
	if err := os.Remove(filepath.Join(f.s.Root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(f.s.Root, ".tpatch", "features", f.slug, "status.json")
	originalPath := statusPath + ".original"
	old := beforeReconcileStatusWrite
	t.Cleanup(func() { beforeReconcileStatusWrite = old })
	calls := 0
	beforeReconcileStatusWrite = func() {
		calls++
		if err := os.Rename(statusPath, originalPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(statusPath, 0755); err != nil {
			t.Fatal(err)
		}
	}
	results, err := RunReconcile(context.Background(), f.s, []string{f.slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if !errors.Is(err, ErrOperationCandidatePersistence) || len(results) != 1 || calls != 1 {
		t.Fatalf("retirement write failure reported success: results=%+v calls=%d err=%v", results, calls, err)
	}
	raw, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	var status store.FeatureStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateBlocked || status.Notes != "keep-s6-notes" {
		t.Fatalf("failed retirement overwrote the original status: %+v", status)
	}
}
