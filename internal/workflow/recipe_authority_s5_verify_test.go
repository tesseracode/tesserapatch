package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRGAS5PreimageAtTreeExactPostimage(t *testing.T) {
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	mustGit(t, root, "config", "user.name", "S5")
	mustGit(t, root, "config", "user.email", "s5@example.invalid")
	mustGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("post\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "add", "target.txt", "empty.txt")
	mustGit(t, root, "commit", "-qm", "immutable baseline")
	ctx := &verifyRunContext{root: root, floorOK: true}
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("live drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, gate, post string
		ok                     bool
	}{
		{"empty-gate-postimage", "target.txt", "", "post\n", true},
		{"hash-gate-postimage", "target.txt", hashOf([]byte("pre\n")), "post\n", true},
		{"empty-postimage", "empty.txt", "", "", true},
		{"hash-gate-empty-postimage", "empty.txt", hashOf([]byte("pre\n")), "", true},
		{"collision", "target.txt", "", "different\n", false},
		{"drift", "target.txt", hashOf([]byte("pre\n")), "different\n", false},
		{"matching-preimage", "target.txt", hashOf([]byte("post\n")), "different\n", true},
		{"missing-target", "absent.txt", hashOf(nil), "", false},
		{"creation", "absent.txt", "", "", true},
		{"malformed-gate", "target.txt", "malformed", "post\n", false},
		{"uppercase-gate", "target.txt", "sha256:" + strings.Repeat("A", 64), "post\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := RecipeOperation{Type: "write-file", Path: tc.path, Content: tc.post, PreimageHash: ptr(tc.gate), CreatedBy: "parent"}
			ok, msg, observed := ctx.preimageAtTree("HEAD", "demo", 1, op)
			if ok != tc.ok {
				t.Fatalf("tree classification: ok=%v message=%s observed=%s", ok, msg, observed)
			}
			if strings.Contains(msg, "post\n") || strings.Contains(msg, "live drift\n") {
				t.Fatalf("tree diagnostic exposed source bytes: %s", msg)
			}
		})
	}
	op := RecipeOperation{Type: "write-file", Path: "target.txt", Content: "post\n", PreimageHash: ptr("")}
	if ok, msg, _ := ctx.preimageAtTree(strings.Repeat("0", 40), "demo", 1, op); ok || !strings.Contains(msg, "cannot read the baseline tree") {
		t.Fatalf("unreadable baseline passed exact-postimage recognition: %v %s", ok, msg)
	}
}
