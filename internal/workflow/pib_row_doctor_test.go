//go:build (linux && !android) || (darwin && !ios)

package workflow

// Owning acceptance test for the aggregate-ledger row whose only prior
// authoritative target could skip. Under this build tag the workspace
// authority is always supported, so the fixture asserts that fact rather than
// skipping on it.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestPIBRowPIB381DoctorNeverObservesLiveAuthority owns PIB-381: two concurrent
// doctor runs on one workspace, plus a third with an acquirable root, observe
// neither the authority nor each other, and no output claims a live authority
// or its absence.
func TestPIBRowPIB381DoctorNeverObservesLiveAuthority(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Fatal("this build tag selects the authority-supported targets; the constant disagrees")
	}
	root := t.TempDir()
	repoStore, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repoStore.AddFeature(store.AddFeatureInput{
		Title: "Live Authority", Slug: "live-authority",
	}); err != nil {
		t.Fatal(err)
	}
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", "live-authority")
	if err := os.MkdirAll(lane, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	type observation struct {
		report DoctorReport
		err    error
	}
	results := make(chan observation, 2)
	for range 2 {
		go func() {
			report, runErr := RunDoctor(repoStore, DoctorOptions{Checks: []string{"D9"}})
			results <- observation{report: report, err: runErr}
		}()
	}
	var reports []DoctorReport
	for range 2 {
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		reports = append(reports, got.report)
	}
	if err := authority.Release(); err != nil {
		t.Fatal(err)
	}
	third, err := RunDoctor(repoStore, DoctorOptions{Checks: []string{"D9"}})
	if err != nil {
		t.Fatal(err)
	}
	reports = append(reports, third)

	reacquired, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatalf("PIB-381: doctor perturbed authority release or acquisition: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatal(err)
	}

	var rendered []string
	for _, report := range reports {
		var output bytes.Buffer
		if err := WriteDoctorJSON(&output, report); err != nil {
			t.Fatal(err)
		}
		rendered = append(rendered, output.String())
		for _, forbidden := range []string{
			"authority held", "authority is free", "no holder", "holder identity", "process id",
		} {
			if strings.Contains(strings.ToLower(output.String()), forbidden) {
				t.Fatalf("PIB-381: doctor made the live-authority claim %q:\n%s", forbidden, output.String())
			}
		}
	}
	if len(rendered) != 3 {
		t.Fatalf("PIB-381: rendered %d reports, want 3", len(rendered))
	}
	if rendered[0] != rendered[1] || rendered[1] != rendered[2] {
		t.Fatalf("PIB-381: the doctors observed the authority or each other:\n%s\n---\n%s\n---\n%s",
			rendered[0], rendered[1], rendered[2])
	}
}
