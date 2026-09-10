package gitutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRGAS5BinaryPayloadIsCheckedRatherThanCalledLimited(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	pre, post := []byte("old\x00payload\n"), []byte("new\x00payload\n")
	path := filepath.Join(root, "binary.dat")
	if err := os.WriteFile(path, pre, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(root, "add", "binary.dat"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(root, "commit", "-qm", "binary reference"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, post, 0644); err != nil {
		t.Fatal(err)
	}
	patch, err := runGit(root, "diff", "--binary", "--", "binary.dat")
	if err != nil {
		t.Fatal(err)
	}
	effects, err := NormalizePatchEffects(patch)
	if err != nil || len(effects) != 1 {
		t.Fatalf("effects=%d %v", len(effects), err)
	}
	got, available, err := ReconstructPatchPostimage(patch, effects[0], pre, 1024)
	if err != nil || !available || !bytes.Equal(got, post) {
		t.Fatalf("binary reconstruction available=%v error=%v", available, err)
	}
	broken := strings.Replace(patch, "literal ", "broken ", 1)
	if broken == patch {
		t.Fatal("literal payload fixture unexpectedly became a delta")
	}
	effects, err = NormalizePatchEffects(broken)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReconstructPatchPostimage(broken, effects[0], pre, 1024); err == nil {
		t.Fatal("malformed reconstructable binary payload became a limitation")
	}
	// Git's delta format: base length 3, output length 4, copy 3 at
	// offset zero, then insert one byte.
	delta := []byte{3, 4, 0x90, 3, 1, '!'}
	got, err = reconstructBinaryDelta([]byte("abc"), delta, 4)
	if err != nil || string(got) != "abc!" {
		t.Fatalf("delta: %v", err)
	}
	if _, err := reconstructBinaryDelta([]byte("wrong-size"), delta, 4); err == nil {
		t.Fatal("delta accepted wrong preimage")
	}
}

func TestRGAS5ExactMemoryReconstruction(t *testing.T) {
	header := "diff --git a/a b/a\nindex 123..456 100644\n--- a/a\n+++ b/a\n"
	for _, tc := range []struct {
		name, patch, pre, post string
		fails, limited         bool
	}{
		{"text", header + "@@ -1 +1 @@\n-old\n+new\n", "old\n", "new\n", false, false},
		{"crlf", header + "@@ -1 +1 @@\n-old\r\n+new\r\n", "old\r\n", "new\r\n", false, false},
		{"no-newline", header + "@@ -1 +1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file\n", "old", "new", false, false},
		{"multiple", header + "@@ -1 +1 @@\n-a\n+A\n@@ -3 +3 @@\n-c\n+C\n", "a\nb\nc\n", "A\nb\nC\n", false, false},
		{"empty-add", "diff --git a/a b/a\nnew file mode 100644\nindex 0000000..e69de29\n", "", "", false, false},
		{"delete", "diff --git a/a b/a\ndeleted file mode 100644\n--- a/a\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n", "old\n", "", false, false},
		{"wrong-context", header + "@@ -1 +1 @@\n-old\n+new\n", "different\n", "", true, false},
		{"no-fuzz", header + "@@ -1 +1 @@\n-old\n+new\n", "prefix\nold\n", "", true, false},
		{"truncated", header + "@@ -1 +1 @@\n-old\n", "old\n", "", true, false},
		{"overlap", header + "@@ -1 +1 @@\n-old\n+new\n@@ -1 +1 @@\n-old\n+new\n", "old\n", "", true, false},
		{"bad-new-offset", header + "@@ -1 +2 @@\n-old\n+new\n", "old\n", "", true, false},
		{"bad-eof", header + "@@ -1 +1,2 @@\n-old\n+new\n\\ No newline at end of file\n+extra\n", "old\n", "", true, false},
		{"binary-stub", "diff --git a/a b/a\nindex 123..456 100644\nBinary files a/a and b/a differ\n", "old\x00", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			effects, err := NormalizePatchEffects(tc.patch)
			if err != nil {
				t.Fatal(err)
			}
			post, available, err := ReconstructPatchPostimage(tc.patch, effects[0], []byte(tc.pre), 1024)
			if (err != nil) != tc.fails || (err == nil && available == tc.limited) ||
				(err == nil && available && !bytes.Equal(post, []byte(tc.post))) {
				t.Fatalf("available=%v error=%v length=%d", available, err, len(post))
			}
			if err == nil && available && len(post) > 0 {
				if _, _, err := ReconstructPatchPostimage(tc.patch, effects[0], []byte(tc.pre), int64(len(post)-1)); err == nil {
					t.Fatal("same transformation accepted insufficient retention budget")
				}
			}
			wrong := effects[0]
			wrong.Path = "another"
			if _, _, err := ReconstructPatchPostimage(tc.patch, wrong, []byte(tc.pre), 1024); err == nil {
				t.Fatal("same validator accepted a substituted normalized effect")
			}
		})
	}
}
