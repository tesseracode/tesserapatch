package gitutil

import "testing"

func TestPatchSignature_Deterministic(t *testing.T) {
	patch := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"
	s1, n1 := PatchSignature(patch)
	s2, n2 := PatchSignature(patch)
	if s1 != s2 || n1 != n2 {
		t.Fatalf("PatchSignature must be deterministic, got (%s,%d) vs (%s,%d)", s1, n1, s2, n2)
	}
	if n1 != len(patch) {
		t.Fatalf("byte length should equal len(patch)=%d, got %d", len(patch), n1)
	}
	if len(s1) != 64 {
		t.Fatalf("sha256 hex should be 64 chars, got %d (%q)", len(s1), s1)
	}
}

func TestPatchSignature_DifferentBytesDifferentDigest(t *testing.T) {
	a, _ := PatchSignature("a")
	b, _ := PatchSignature("b")
	if a == b {
		t.Fatalf("expected distinct digests for distinct inputs")
	}
}

func TestPatchSignature_EmptyString(t *testing.T) {
	// Defensively allow callers to ask for the empty-patch signature;
	// the collision scan skips empty patches at the policy layer
	// (PRD §4 step 0), not here.
	s, n := PatchSignature("")
	if n != 0 {
		t.Fatalf("expected byte length 0 for empty patch, got %d", n)
	}
	// sha256("") is well-known.
	const sha256Empty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if s != sha256Empty {
		t.Fatalf("sha256(\"\") mismatch: got %s", s)
	}
}
