package patchobs

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

func TestRGAS5GitlinkIdentityProofIsIndependentOfImageBudget(t *testing.T) {
	root := obsRepo(t)
	oldID, newID := strings.Repeat("1", 40), strings.Repeat("2", 40)
	obsGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oldID+",z-link")
	obsGit(t, root, "commit", "-qm", "gitlink reference")
	commit := obsHead(t, root)
	largeBody := strings.Repeat("x", int(imageRetentionLimit)-1) + "\n"
	largePatch := "diff --git a/a-large.txt b/a-large.txt\nnew file mode 100644\n--- /dev/null\n+++ b/a-large.txt\n@@ -0,0 +1 @@\n+" + largeBody
	identityHash := fmt.Sprintf("%x", sha256.Sum256([]byte(newID)))
	textHash := fmt.Sprintf("%x", sha256.Sum256([]byte("Subproject commit "+newID+"\n")))

	for _, exhausted := range []bool{false, true} {
		for _, tc := range []struct {
			name, post string
			valid      bool
		}{
			{"valid", "Subproject commit " + newID + "\n", true},
			{"missing-prefix", newID + "\n", false},
			{"short-id", "Subproject commit " + newID[:39] + "\n", false},
			{"uppercase-id", "Subproject commit " + strings.Repeat("A", 40) + "\n", false},
			{"oversized-id", "Subproject commit " + strings.Repeat("2", 128) + "\n", false},
			{"extra-line", "Subproject commit " + newID + "\nextra\n", false},
		} {
			t.Run(fmt.Sprintf("exhausted=%v/%s", exhausted, tc.name), func(t *testing.T) {
				lines := strings.Split(strings.TrimSuffix(tc.post, "\n"), "\n")
				var patch strings.Builder
				if exhausted {
					patch.WriteString(largePatch)
				}
				patch.WriteString("diff --git a/z-link b/z-link\nindex 1111111..2222222 160000\n--- a/z-link\n+++ b/z-link\n")
				fmt.Fprintf(&patch, "@@ -1 +1,%d @@\n-Subproject commit %s\n", len(lines), oldID)
				for _, line := range lines {
					fmt.Fprintf(&patch, "+%s\n", line)
				}
				effects, err := Reconstruct(root, commit, patch.String())
				if !tc.valid {
					if err == nil || !strings.Contains(err.Error(), "invalid gitlink payload") {
						t.Fatalf("malformed gitlink gained proof: effects=%d err=%v", len(effects), err)
					}
					return
				}
				wantEffects := 1
				if exhausted {
					wantEffects++
				}
				if err != nil || len(effects) != wantEffects {
					t.Fatalf("valid gitlink refused: effects=%d err=%v", len(effects), err)
				}
				if exhausted && len(effects[0].Post.Bytes) != int(imageRetentionLimit) {
					t.Fatal("fixture did not exhaust the retained-image budget before the gitlink")
				}
				post := effects[len(effects)-1].Post
				if !post.Available || !post.Present || post.Mode != gitutil.ModeGitlink ||
					post.SHA256 != identityHash || post.SHA256 == textHash ||
					post.DigestProved || len(post.Bytes) != 0 {
					t.Fatalf("gitlink identity used a textual/budget-only proof: %+v", post)
				}
				var retained int64
				for _, effect := range effects {
					retained += int64(len(effect.Pre.Bytes) + len(effect.Post.Bytes))
				}
				if retained > imageRetentionLimit {
					t.Fatalf("gitlink scratch leaked into retained images: %d", retained)
				}
			})
		}
	}
}
