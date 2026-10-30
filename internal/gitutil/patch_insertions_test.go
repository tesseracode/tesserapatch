package gitutil

import (
	"errors"
	"reflect"
	"testing"
)

func TestS3InsertionRunsUseStrictWalker(t *testing.T) {
	patch := "diff --git a/a b/a\nindex 1111111..2222222 100644\n--- a/a\n+++ b/a\n@@ -1,2 +1,5 @@\n+x\r\n a\r\n+y\n+z\n b\n"
	effects, err := NormalizePatchEffects(patch)
	if err != nil {
		t.Fatal(err)
	}
	runs, digest, err := ExtractPatchInsertionRuns(patch, effects[0], []byte("a\r\nb\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []PatchInsertionRun{
		{Ordinal: 1, Boundary: 0, Records: [][]byte{[]byte("x\r\n")}},
		{Ordinal: 2, Boundary: 1, Records: [][]byte{[]byte("y\n"), []byte("z\n")}},
	}
	if !reflect.DeepEqual(runs, want) {
		t.Fatalf("runs=%#v want=%#v", runs, want)
	}
	post, available, err := ReconstructPatchPostimage(patch, effects[0], []byte("a\r\nb\n"), 1024)
	if err != nil || !available || string(post) != "x\r\na\r\ny\nz\nb\n" {
		t.Fatalf("postimage: %q %v", post, err)
	}
	_, proofErr := reconstructTextRecord(patch, []byte("a\r\nb\n"), 0)
	var proof *PatchImageBudgetProof
	if !errors.As(proofErr, &proof) || proof.SHA256 != digest {
		t.Fatal("walker and insertion projection disagree")
	}
	if _, _, err := ExtractPatchInsertionRuns(patch, effects[0], []byte("wrong\n")); !errors.Is(err, ErrInsertionBoundary) {
		t.Fatalf("bad base accepted: %v", err)
	}
	bad := effects[0]
	bad.FragmentSHA256 = "forged"
	if _, _, err := ExtractPatchInsertionRuns(patch, bad, []byte("a\r\nb\n")); !errors.Is(err, ErrInsertionFragment) {
		t.Fatalf("forged effect accepted: %v", err)
	}
}

func TestS3InsertionRunsPreserveNoNewlineAndRefuseReplacement(t *testing.T) {
	for _, row := range []struct {
		patch, base string
		replacement bool
	}{
		{"diff --git a/a b/a\nindex 1111111..2222222 100644\n--- a/a\n+++ b/a\n@@ -1 +1,2 @@\n a\n+x\n\\ No newline at end of file\n", "a\n", false},
		{"diff --git a/a b/a\nindex 1111111..2222222 100644\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-a\n+x\n", "a\n", true},
	} {
		effects, err := NormalizePatchEffects(row.patch)
		if err != nil {
			t.Fatal(err)
		}
		runs, _, err := ExtractPatchInsertionRuns(row.patch, effects[0], []byte(row.base))
		if row.replacement {
			if !errors.Is(err, ErrInsertionReplacement) || runs != nil {
				t.Fatalf("replacement accepted: %v", err)
			}
		} else if err != nil || len(runs) != 1 || string(runs[0].Records[0]) != "x" || runs[0].Boundary != 1 {
			t.Fatalf("no-newline record lost: %#v %v", runs, err)
		}
	}
}
