package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestRGAS5VerifyCoverageLadderRowsAndInstability(t *testing.T) {
	s, complete := rgaS5ReadFixture(t)
	status, err := s.LoadFeatureStatus("s5")
	if err != nil {
		t.Fatal(err)
	}
	incomplete := complete
	incomplete.Events.StaleMarkerPresent = true
	artifact := func(read RecipeArtifactRead) artifactSnapshot {
		presence := PresenceAbsent
		if read.Exists && read.Err == nil {
			presence = PresenceNonEmpty
			if len(read.Bytes) == 0 {
				presence = PresenceEmpty
			}
		}
		return artifactSnapshot{Presence: presence, Bytes: read.Bytes, Err: read.Err}
	}
	for _, tc := range []struct {
		name   string
		rung   int
		in     RecipeCoverageInput
		change func(*RecipeCoverageSnapshot)
	}{
		{"malformed", 1, complete, func(s *RecipeCoverageSnapshot) { s.Coverage.Err = errors.New("EIO") }},
		{"evidence", 2, complete, func(s *RecipeCoverageSnapshot) { s.Event = RecipeArtifactRead{} }},
		{"incomplete", 3, incomplete, nil},
		{"marker", 4, complete, func(s *RecipeCoverageSnapshot) { s.Marker.Exists = true }},
		{"legacy-marker", 5, complete, func(s *RecipeCoverageSnapshot) { s.Coverage = RecipeArtifactRead{}; s.Marker.Exists = true }},
		{"complete", 6, complete, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := rgaS5ReadSnapshot(t, tc.in)
			if tc.change != nil {
				tc.change(&snap)
			}
			entry := &inventoryEntry{Slug: "s5", Status: &status, Recipe: artifact(snap.Recipe), Patch: artifact(snap.Patch),
				Coverage: artifact(snap.Coverage), CaptureEvent: artifact(snap.Event), StaleMarker: artifact(snap.Marker),
				Provenance: artifact(RecipeArtifactRead{})}
			ctx := &verifyRunContext{root: s.Root, floorOK: true, inv: &featureInventory{
				Order: []string{"s5"}, Entries: map[string]*inventoryEntry{"s5": entry}}}
			if _, err := entry.ReadErr(); err != nil {
				t.Fatal("coverage errors escaped through the legacy inventory shortcut")
			}
			row := checkRecipeGenerationCoverage(ctx, s, "s5")
			validate := func(row store.VerifyCheckResult) error {
				severity := SeverityWarn
				if tc.rung < 3 || tc.rung == 6 {
					severity = SeverityBlock
				}
				if row.ID != CheckRecipeGenerationCoverage || row.Severity != severity || row.Passed != (tc.rung == 6) || row.Skipped {
					return fmt.Errorf("wrong coverage row: %+v", row)
				}
				return nil
			}
			if err := validate(row); err != nil {
				t.Fatal(err)
			}
			wrong := row
			wrong.Severity = SeverityBlock
			if row.Severity == SeverityBlock {
				wrong.Severity = SeverityWarn
			}
			if err := validate(wrong); err == nil {
				t.Fatal("same row validator accepted changed precedence severity")
			}
			if tc.rung == 6 && row.Remediation != "" {
				t.Fatal("passing row acquired gratuitous fields")
			}
		})
	}
	before, err := buildInventory(s)
	if err != nil {
		t.Fatal(err)
	}
	original := ReadRecipeArtifact
	t.Cleanup(func() { ReadRecipeArtifact = original })
	ReadRecipeArtifact = func(path string) RecipeArtifactRead {
		if filepath.Base(path) == "recipe-capture-event.json" {
			return RecipeArtifactRead{Exists: true, Err: errors.New("portable transition")}
		}
		return original(path)
	}
	if got := inventoryInstability(s, before); !strings.Contains(got, "recipe-capture-event.json") {
		t.Fatalf("companion readability transition missed: %q", got)
	}
}

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
}

func TestRGAS5PreimageTreeReadErrorSeamAndSensitivity(t *testing.T) {
	readFailure := errors.New("injected immutable blob read failure")
	validate := func(readBlob func(string, string) ([]byte, bool, error)) error {
		for _, gate := range []string{"", hashOf([]byte("pre\n"))} {
			op := RecipeOperation{Type: "write-file", Path: "target.txt", Content: "post\n", PreimageHash: ptr(gate)}
			ok, msg, observed := preimageAtTreeWithReader("captured-tree", "demo", 1, op, readBlob)
			if ok || observed != "" || !strings.Contains(msg, readFailure.Error()) ||
				!strings.Contains(msg, "cannot read the baseline tree captured-tree") || strings.Contains(msg, "post\n") {
				return fmt.Errorf("read failure became absence, success or source bytes: ok=%v msg=%q observed=%q", ok, msg, observed)
			}
		}
		return nil
	}
	if err := validate(func(tree, path string) ([]byte, bool, error) {
		if tree != "captured-tree" || path != "target.txt" {
			t.Fatalf("reader received the wrong immutable target: %s:%s", tree, path)
		}
		return nil, false, readFailure
	}); err != nil {
		t.Fatal(err)
	}
	if err := validate(func(string, string) ([]byte, bool, error) {
		return nil, false, nil
	}); err == nil {
		t.Fatal("the same read-error validator accepted a swallowed failure")
	}
}
