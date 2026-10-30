//go:build !windows

package candidate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

func derivationGitBytes(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func derivationFixture(t *testing.T, base, feature, upstream string, creation bool) (string, *ReferenceRoot, *ReferenceRoot, string, []gitutil.PatchEffect) {
	t.Helper()
	root, _ := initCandidateRepo(t)
	path := filepath.Join(root, "args.go")
	if !creation {
		if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
			t.Fatal(err)
		}
		candidateGit(t, root, "add", "args.go")
		candidateGit(t, root, "commit", "-qm", "reference")
	}
	baseCommit := candidateGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(path, []byte(feature), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "args.go")
	candidateGit(t, root, "commit", "-qm", "feature")
	patch := string(derivationGitBytes(t, root, "diff", "--binary", baseCommit, "HEAD", "--", "args.go"))
	rows, err := patchobs.ReconstructOffline(root, baseCommit, patch)
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
	if creation && upstream == "" {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		candidateGit(t, root, "add", "args.go")
	} else {
		if err := os.WriteFile(path, []byte(upstream), 0o644); err != nil {
			t.Fatal(err)
		}
		candidateGit(t, root, "add", "args.go")
	}
	candidateGit(t, root, "commit", "-qm", "upstream", "--allow-empty")
	upstreamCommit := candidateGit(t, root, "rev-parse", "HEAD")
	ref, err := OpenReferenceRoot(root, baseCommit)
	if err != nil {
		t.Fatal(err)
	}
	up, err := OpenReferenceRoot(root, upstreamCommit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ref.Close(); _ = up.Close() })
	return root, ref, up, patch, effects
}

func applyPlanBytes(t *testing.T, plan Plan, upstream []byte) []byte {
	t.Helper()
	result := bytes.Clone(upstream)
	last := len(upstream) + 1
	for _, op := range plan.operations {
		if op.kind == "create-file" {
			result = bytes.Clone(op.payload)
			continue
		}
		if op.byteOffset >= last {
			t.Fatal("operations are not in descending offset order")
		}
		last = op.byteOffset
		result = append(append(append([]byte(nil), result[:op.byteOffset]...), op.payload...), result[op.byteOffset:]...)
	}
	return result
}

func TestS3MotivatingAdjacentArgumentsBothVariants(t *testing.T) {
	const base = "args := []string{\n\t\"--old-a\",\n\t\"--old-b\",\n}\nreturn run(args)\n"
	for _, row := range []struct{ name, feature, upstream, want string }{
		{"delete-all",
			"args := []string{\n\t\"--old-a\",\n\t\"--old-b\",\n\t\"--feature-x\",\n\t\"--feature-y\",\n}\nreturn run(args)\n",
			"args := []string{\n}\nreturn run(args)\n",
			"args := []string{\n\t\"--feature-x\",\n\t\"--feature-y\",\n}\nreturn run(args)\n"},
		{"delete-first",
			"args := []string{\n\t\"--old-a\",\n\t\"--feature-x\",\n\t\"--feature-y\",\n\t\"--old-b\",\n}\nreturn run(args)\n",
			"args := []string{\n\t\"--old-b\",\n}\nreturn run(args)\n",
			"args := []string{\n\t\"--feature-x\",\n\t\"--feature-y\",\n\t\"--old-b\",\n}\nreturn run(args)\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			root, ref, up, patch, effects := derivationFixture(t, base, row.feature, row.upstream, false)
			before := candidateGit(t, root, "rev-parse", "HEAD")
			plan, err := Derive(ref, up, patch, effects)
			p, a, c := plan.Counts()
			if err != nil || p != 0 || a != 1 || c != 0 || len(plan.operations) != 1 {
				t.Fatalf("plan %+v counts%d/%d/%d err%v", plan, p, a, c, err)
			}
			if string(applyPlanBytes(t, plan, []byte(row.upstream))) != row.want {
				t.Fatal("deleted arguments resurrected or feature arguments lost")
			}
			body, err := os.ReadFile(filepath.Join(root, "args.go"))
			if err != nil || string(body) != row.upstream || candidateGit(t, root, "rev-parse", "HEAD") != before {
				t.Fatal("derivation mutated the live repository")
			}
			if _, err := json.Marshal(plan); err == nil {
				t.Fatal("ephemeral plan serialized")
			}
			if err := os.WriteFile(filepath.Join(root, "args.go"), []byte(row.want), 0o644); err != nil {
				t.Fatal(err)
			}
			candidateGit(t, root, "add", "args.go")
			candidateGit(t, root, "commit", "-qm", "already upstream")
			presentRoot, err := OpenReferenceRoot(root, candidateGit(t, root, "rev-parse", "HEAD"))
			if err != nil {
				t.Fatal(err)
			}
			defer presentRoot.Close()
			again, err := Derive(ref, presentRoot, patch, effects)
			p, a, c = again.Counts()
			if err != nil || p != 1 || a != 0 || c != 0 || len(again.operations) != 0 {
				t.Fatalf("exact gap not present: %+v %v", again, err)
			}
		})
	}
}

func TestS3CreationClassificationAndMode(t *testing.T) {
	for _, row := range []struct {
		name, upstream string
		want           classification
	}{
		{"absent", "", applicable}, {"present", "feature\n", present}, {"conflict", "different\n", conflict},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, ref, up, patch, effects := derivationFixture(t, "", "feature\n", row.upstream, true)
			plan, err := Derive(ref, up, patch, effects)
			if err != nil || len(plan.effects) != 1 || plan.effects[0].state != row.want {
				t.Fatalf("creation %+v %v", plan, err)
			}
			if row.want != applicable && len(plan.operations) != 0 {
				t.Fatal("nonapplicable creation acquired a writer")
			}
		})
	}
	root, ref, _, patch, effects := derivationFixture(t, "", "feature\n", "feature\n", true)
	if err := os.Chmod(filepath.Join(root, "args.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "args.go")
	candidateGit(t, root, "commit", "-qm", "executable upstream")
	up, err := OpenReferenceRoot(root, candidateGit(t, root, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	plan, err := Derive(ref, up, patch, effects)
	if err != nil || plan.effects[0].state != conflict {
		t.Fatalf("wrong mode granted presence: %+v %v", plan, err)
	}
}

func TestS3DomainPostimageAndAvailabilityRefusals(t *testing.T) {
	_, ref, up, patch, effects := derivationFixture(t, "a\nb\n", "a\nx\nb\n", "a\nb\n", false)
	for _, row := range []struct {
		code   string
		mutate func(*gitutil.PatchEffect)
	}{
		{"effect-content-kind-unsupported", func(e *gitutil.PatchEffect) { e.ContentKind = gitutil.ContentKindBinary }},
		{"effect-object-kind-unsupported", func(e *gitutil.PatchEffect) { e.ObjectKind = gitutil.ObjectKindExecutable }},
		{"effect-mode-change-unsupported", func(e *gitutil.PatchEffect) { e.NewMode = gitutil.ModeExecutable }},
		{"effect-binding-drift", func(e *gitutil.PatchEffect) { e.PostimageObserved = false }},
		{"derivation-postimage-mismatch", func(e *gitutil.PatchEffect) { e.PostimageSHA256 = digestBytes([]byte("forged\n")) }},
	} {
		bad := append([]gitutil.PatchEffect(nil), effects...)
		row.mutate(&bad[0])
		plan, err := Derive(ref, up, patch, bad)
		if refusalCode(err) != row.code || len(plan.operations) != 0 {
			t.Fatalf("domain %s: %+v %v", row.code, plan, err)
		}
	}
	_, replaceRef, replaceUp, replacePatch, replaceEffects := derivationFixture(t, "a\n", "b\n", "a\n", false)
	if plan, err := Derive(replaceRef, replaceUp, replacePatch, replaceEffects); refusalCode(err) != "effect-replacement-unsupported" || len(plan.operations) != 0 {
		t.Fatalf("replacement was replayed: %+v %v", plan, err)
	}
}

func TestS3SnapshotDoesNotRereadPaths(t *testing.T) {
	root, head := initCandidateRepo(t)
	ref, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()
	snapshot, err := ref.snapshotPaths([]string{"tracked.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("different\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "tracked.txt")
	candidateGit(t, root, "commit", "-qm", "advance live ref")
	body, err := snapshot.read("tracked.txt", alignmentMaxBytes)
	if err != nil || string(body) != "v1\n" {
		t.Fatalf("snapshot reread mutable state: %q %v", body, err)
	}
}

func TestS3DescendingMultipleBoundaryApplication(t *testing.T) {
	const base = "a\nb\nc\nd\n"
	const feature = "a\nx\nb\nc\ny\nd\n"
	_, ref, up, patch, effects := derivationFixture(t, base, feature, base, false)
	plan, err := Derive(ref, up, patch, effects)
	if err != nil || len(plan.operations) != 2 {
		t.Fatalf("multi boundary: %+v %v", plan, err)
	}
	if string(applyPlanBytes(t, plan, []byte(base))) != feature {
		t.Fatal("offset shift corrupted later insertion")
	}
}
