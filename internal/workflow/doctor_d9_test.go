package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD9CleanWorkspaceAndLostJournalBoundary(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Ordinary Partial", Slug: "ordinary-partial"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", "ordinary-partial", "analysis.md"), []byte("# ordinary lifecycle bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := doctorD9SnapshotTree(t, root)
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("clean/ordinary partial workspace produced D9 findings: %#v", report.Findings)
	}
	if report.Summary.ChecksRun != 1 || len(report.Checks) != 1 || report.Checks[0].CheckID != "D9" {
		t.Fatalf("D9 selection = %#v", report)
	}
	if got := DoctorExitCode(report); got != 0 {
		t.Fatalf("clean D9 exit = %d", got)
	}
	doctorD9AssertTreeEqual(t, before, doctorD9SnapshotTree(t, root))
}

func TestDoctorD9PrepareEvidenceTaxonomyAndOrdering(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"armed-feature", "stale-feature", "abandoned-feature"} {
		if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug}); err != nil {
			t.Fatal(err)
		}
	}
	doctorD9Write(t, root, ".tpatch/local/intent-prepare/armed-feature/journal.json", "{}\n")
	doctorD9Mkdir(t, root, ".tpatch/local/intent-prepare/armed-feature/stage-aaaaaaaaaaaa")
	doctorD9Mkdir(t, root, ".tpatch/local/intent-prepare/stale-feature/stage-bbbbbbbbbbbb")
	doctorD9Write(t, root, ".tpatch/local/intent-prepare/stale-feature/index.preimage.json", "{}\n")
	doctorD9Write(t, root, ".tpatch/local/intent-prepare/stale-feature/status.preimage.json", "{}\n")
	doctorD9Write(t, root, ".tpatch/local/intent-prepare/stale-feature/.journal.json.tmp-cccccccccccc", "{}\n")
	doctorD9Mkdir(t, root, ".tpatch/local/intent-prepare/abandoned-feature/abandoned-dddddddddddd")
	doctorD9Mkdir(t, root, ".tpatch/local/intent-prepare/abandoned-feature/abandoned-eeeeeeeeeeee")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
	if err != nil {
		t.Fatal(err)
	}
	wantCodes := map[string]int{
		"prepare-transaction-pending": 1,
		"prepare-stage-stale":         1,
		"prepare-preimage-stale":      2,
		"prepare-temp-stale":          1,
		"prepare-abandoned-evidence":  2,
	}
	gotCodes := map[string]int{}
	var keys []string
	for _, finding := range report.Findings {
		gotCodes[finding.Code]++
		keys = append(keys, doctorFindingSortKey(finding))
		if finding.CheckID != "D9" || finding.Severity != "warning" || finding.Fixable {
			t.Fatalf("D9 finding shape = %#v", finding)
		}
		if filepath.IsAbs(finding.Path) || strings.ContainsAny(finding.Path, "\r\n\t") {
			t.Fatalf("unsafe finding path %q", finding.Path)
		}
	}
	if fmt.Sprint(gotCodes) != fmt.Sprint(wantCodes) {
		t.Fatalf("D9 prepare taxonomy = %#v, want %#v", gotCodes, wantCodes)
	}
	if !sort.StringsAreSorted(keys) {
		t.Fatalf("D9 findings are not stable: %#v", report.Findings)
	}
	for _, finding := range report.Findings {
		switch finding.Code {
		case "prepare-stage-stale":
			if !strings.Contains(finding.Message, "next successful mutating prepare removes") ||
				finding.Remediation != "run tpatch prepare stale-feature" {
				t.Fatalf("stage finding = %#v", finding)
			}
		case "prepare-abandoned-evidence":
			if !strings.HasPrefix(finding.Remediation, "rm -rf -- '.tpatch/local/intent-prepare/abandoned-feature/") {
				t.Fatalf("abandoned remediation = %q", finding.Remediation)
			}
		}
		if finding.Feature == "armed-feature" && finding.Code == "prepare-stage-stale" {
			t.Fatal("journal-owned stage was misreported as unarmed stale residue")
		}
	}
	if report.Summary.Warnings != 7 || DoctorExitCode(report) != 0 {
		t.Fatalf("warning-only summary/exit = %#v exit=%d", report.Summary, DoctorExitCode(report))
	}
}

func TestDoctorD9ArchiveEvidenceClassesAndRoutes(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	const slug = "archive-evidence"
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Archive Evidence", Slug: slug}); err != nil {
		t.Fatal(err)
	}
	pendingBytes := []byte("pending")
	pendingBytes2 := []byte("pending-two")
	corruptExpected := []byte("expected")
	danglingBytes := []byte("dangling")
	danglingBytes2 := []byte("dangling-two")
	mixedBytes := []byte("mixed")
	mixedBytes2 := []byte("mixed-two")
	orphanBytes := []byte("orphan")
	orphanBytes2 := []byte("orphan-two")
	pending := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, pendingBytes, store.IntentArchiveWireRemovalPending)
	pending2 := doctorD9Replacement(store.IntentArchiveArtifactSpec, pendingBytes2, store.IntentArchiveWireRemovalPending)
	corrupt := doctorD9Replacement(store.IntentArchiveArtifactAnalysisSidecar, corruptExpected, store.IntentArchiveWireRetained)
	dangling := doctorD9Replacement(store.IntentArchiveArtifactExploration, danglingBytes, store.IntentArchiveWireRetained)
	dangling2 := doctorD9Replacement(store.IntentArchiveArtifactAnalysisSidecar, danglingBytes2, store.IntentArchiveWireRetained)
	mixedLive := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, mixedBytes, store.IntentArchiveWireRetained)
	mixedTombstone := doctorD9Replacement(store.IntentArchiveArtifactSpec, mixedBytes, store.IntentArchiveWireTombstoned)
	mixedLive2 := doctorD9Replacement(store.IntentArchiveArtifactExploration, mixedBytes2, store.IntentArchiveWireRetained)
	mixedTombstone2 := doctorD9Replacement(store.IntentArchiveArtifactAnalysisSidecar, mixedBytes2, store.IntentArchiveWireTombstoned)
	orphan := doctorD9Replacement(store.IntentArchiveArtifactExploration, orphanBytes, store.IntentArchiveWireTombstoned)
	orphan2 := doctorD9Replacement(store.IntentArchiveArtifactSpec, orphanBytes2, store.IntentArchiveWireTombstoned)
	tombstoneCorrupt := doctorD9Replacement(store.IntentArchiveArtifactAnalysisSidecar, []byte("tombstone-corrupt"), store.IntentArchiveWireTombstoned)
	index := store.IntentArchiveIndex{
		SchemaVersion: store.IntentArchiveSchemaVersion,
		Feature:       slug,
		Generations: []store.IntentArchiveGeneration{
			doctorD9Generation(t, slug, pending),
			doctorD9Generation(t, slug, pending2),
			doctorD9Generation(t, slug, corrupt),
			doctorD9Generation(t, slug, dangling),
			doctorD9Generation(t, slug, dangling2),
			doctorD9Generation(t, slug, mixedLive),
			doctorD9Generation(t, slug, mixedTombstone),
			doctorD9Generation(t, slug, mixedLive2),
			doctorD9Generation(t, slug, mixedTombstone2),
			doctorD9Generation(t, slug, orphan),
			doctorD9Generation(t, slug, orphan2),
			doctorD9Generation(t, slug, tombstoneCorrupt),
		},
	}
	doctorD9WriteArchive(t, root, index)
	doctorD9WriteBlob(t, root, slug, pending.ContentSHA256, []byte("owned-wrong-bytes"))
	doctorD9WriteBlob(t, root, slug, pending2.ContentSHA256, []byte("owned-wrong-bytes-two"))
	doctorD9WriteBlob(t, root, slug, corrupt.ContentSHA256, []byte("wrong"))
	doctorD9WriteBlob(t, root, slug, mixedLive.ContentSHA256, mixedBytes)
	doctorD9WriteBlob(t, root, slug, mixedLive2.ContentSHA256, mixedBytes2)
	doctorD9WriteBlob(t, root, slug, orphan.ContentSHA256, orphanBytes)
	doctorD9WriteBlob(t, root, slug, orphan2.ContentSHA256, orphanBytes2)
	tombstoneCorruptRel, _ := store.IntentArchiveBlobRel(slug, tombstoneCorrupt.ContentSHA256)
	doctorD9Mkdir(t, root, tombstoneCorruptRel)
	unindexedHash := strings.Repeat("f", 64)
	doctorD9Mkdir(t, root, ".tpatch/features/"+slug+"/artifacts/intent-archive/blobs/"+unindexedHash+".blob")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := map[string]int{
		"archive-purge-pending":                                 1,
		string(store.IntentArchiveCodeBlobCorrupt):              1,
		string(store.IntentArchiveCodeBlobDangling):             1,
		string(store.IntentArchiveCodeIndexStorageInconsistent): 1,
		"archive-orphan":                                        1,
	}
	gotCounts := map[string]int{}
	var classTags []string
	var human bytes.Buffer
	WriteDoctorHuman(&human, report)
	for _, finding := range report.Findings {
		gotCounts[finding.Code]++
		if finding.Tag == string(store.IntentArchiveRepairCorruptObject) ||
			finding.Tag == string(store.IntentArchiveRepairDanglingReference) ||
			finding.Tag == string(store.IntentArchiveRepairMixedReference) ||
			finding.Tag == string(store.IntentArchiveRepairUnreferencedResidue) {
			classTags = append(classTags, finding.Tag)
		}
		if filepath.IsAbs(finding.Path) || strings.Contains(human.String(), root) {
			t.Fatalf("D9 leaked an absolute path: %#v\n%s", finding, human.String())
		}
	}
	if fmt.Sprint(gotCounts) != fmt.Sprint(wantCounts) {
		t.Fatalf("archive D9 classes = %#v, want %#v\n%s", gotCounts, wantCounts, human.String())
	}
	if got := doctorD9Unique(classTags); fmt.Sprint(got) != fmt.Sprint([]string{
		string(store.IntentArchiveRepairCorruptObject),
		string(store.IntentArchiveRepairDanglingReference),
		string(store.IntentArchiveRepairMixedReference),
		string(store.IntentArchiveRepairUnreferencedResidue),
	}) {
		t.Fatalf("repair class order = %#v", got)
	}
	for _, finding := range report.Findings {
		if finding.Path != "" {
			t.Fatalf("multi-instance class should use an empty class-level path: %#v", finding)
		}
		switch finding.Code {
		case "archive-purge-pending":
			if finding.Remediation != doctorD9BlobPurgeCommand(slug, []string{pending.ContentSHA256, pending2.ContentSHA256}) {
				t.Fatalf("pending remediation = %q", finding.Remediation)
			}
			doctorD9AssertInstanceOnce(t, finding.Message, pending.ContentSHA256)
			doctorD9AssertInstanceOnce(t, finding.Message, pending2.ContentSHA256)
			doctorD9AssertInstanceOnce(t, finding.Remediation, "--blob "+pending.ContentSHA256)
			doctorD9AssertInstanceOnce(t, finding.Remediation, "--blob "+pending2.ContentSHA256)
		case string(store.IntentArchiveCodeBlobCorrupt):
			for _, want := range []string{"rank-1", "blocks every tpatch repair selector"} {
				if !strings.Contains(finding.Message, want) {
					t.Fatalf("corrupt message missing %q: %q", want, finding.Message)
				}
			}
			if !strings.Contains(finding.Remediation, "rm -rf -- ") ||
				strings.Contains(finding.Remediation, "--orphans") {
				t.Fatalf("corrupt remediation = %q", finding.Remediation)
			}
			for _, rel := range []string{
				doctorD9BlobRel(t, slug, corrupt.ContentSHA256),
				tombstoneCorruptRel,
				".tpatch/features/" + slug + "/artifacts/intent-archive/blobs/" + unindexedHash + ".blob",
			} {
				doctorD9AssertInstanceOnce(t, finding.Message, rel)
				doctorD9AssertInstanceOnce(t, finding.Remediation, doctorD9ShellQuote(rel))
			}
		case string(store.IntentArchiveCodeBlobDangling), string(store.IntentArchiveCodeIndexStorageInconsistent):
			if !strings.Contains(finding.Remediation, " --blob ") ||
				strings.Contains(finding.Remediation, "--orphans") {
				t.Fatalf("hash-class remediation = %q", finding.Remediation)
			}
			hashes := []string{dangling.ContentSHA256, dangling2.ContentSHA256}
			if finding.Code == string(store.IntentArchiveCodeIndexStorageInconsistent) {
				hashes = []string{mixedLive.ContentSHA256, mixedLive2.ContentSHA256}
			}
			for _, hash := range hashes {
				doctorD9AssertInstanceOnce(t, finding.Message, hash)
				doctorD9AssertInstanceOnce(t, finding.Remediation, "--blob "+hash)
			}
		case "archive-orphan":
			if finding.Remediation != "tpatch feature intent-archive purge "+slug+" --orphans --yes" ||
				!strings.Contains(finding.Message, "2 globally unreferenced") {
				t.Fatalf("orphan finding = %#v", finding)
			}
			doctorD9AssertInstanceOnce(t, finding.Message, orphan.ContentSHA256)
			doctorD9AssertInstanceOnce(t, finding.Message, orphan2.ContentSHA256)
		}
		if finding.Tag == string(store.IntentArchiveRepairCorruptObject) ||
			finding.Tag == string(store.IntentArchiveRepairDanglingReference) ||
			finding.Tag == string(store.IntentArchiveRepairMixedReference) ||
			finding.Tag == string(store.IntentArchiveRepairUnreferencedResidue) {
			if !strings.Contains(finding.Message, "Repair work is staged") ||
				!strings.Contains(finding.Message, "remaining stages") {
				t.Fatalf("multi-class finding does not describe stages: %#v", finding)
			}
		}
	}
}

func TestDoctorD9MalformedAndUnsafeEvidenceFailsClosed(t *testing.T) {
	t.Run("corrupt-index-offers-list-only", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Bad Index", Slug: "bad-index"}); err != nil {
			t.Fatal(err)
		}
		doctorD9Write(t, root, ".tpatch/features/bad-index/artifacts/intent-archive/index.json", "{not-json\n")
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		finding := assertFinding(t, report, "D9", "archive-index-invalid", "bad-index")
		if finding.Remediation != "run tpatch feature intent-archive list bad-index" ||
			strings.Contains(finding.Remediation, "purge") ||
			strings.Contains(finding.Remediation, "rm ") ||
			!strings.Contains(finding.Message, "archive-index-corrupt") {
			t.Fatalf("corrupt index finding = %#v", finding)
		}
	})

	t.Run("symlink-target-not-followed-or-echoed", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		const slug = "symlink-blob"
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Symlink Blob", Slug: slug}); err != nil {
			t.Fatal(err)
		}
		targetBytes := []byte("private-target-bytes")
		target := filepath.Join(root, "outside-private")
		if err := os.WriteFile(target, targetBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		replacement := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, targetBytes, store.IntentArchiveWireRetained)
		index := store.IntentArchiveIndex{
			SchemaVersion: store.IntentArchiveSchemaVersion,
			Feature:       slug,
			Generations:   []store.IntentArchiveGeneration{doctorD9Generation(t, slug, replacement)},
		}
		doctorD9WriteArchive(t, root, index)
		blobsRel, _ := store.IntentArchiveBlobsRel(slug)
		doctorD9Mkdir(t, root, blobsRel)
		blobRel, _ := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
		blobPath := filepath.Join(root, filepath.FromSlash(blobRel))
		if err := os.Symlink(target, blobPath); err != nil {
			t.Fatal(err)
		}
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		var human, structured bytes.Buffer
		WriteDoctorHuman(&human, report)
		if err := WriteDoctorJSON(&structured, report); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{target, string(targetBytes), root} {
			if strings.Contains(human.String(), forbidden) || strings.Contains(structured.String(), forbidden) {
				t.Fatalf("D9 echoed symlink target/content %q:\n%s\n%s", forbidden, human.String(), structured.String())
			}
		}
		finding := assertFinding(t, report, "D9", string(store.IntentArchiveCodeBlobCorrupt), slug)
		if finding.Path != blobRel || !strings.Contains(finding.Remediation, "rm -rf -- ") {
			t.Fatalf("symlink finding = %#v", finding)
		}
	})

	t.Run("control-character-name-not-echoed", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		const slug = "control-name"
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Control Name", Slug: slug}); err != nil {
			t.Fatal(err)
		}
		doctorD9Mkdir(t, root, ".tpatch/features/"+slug+"/artifacts/intent-archive/blobs")
		badName := "bad\nname.blob"
		if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs", badName), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := WriteDoctorJSON(&output, report); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), badName) || strings.Contains(output.String(), "bad\\nname") {
			t.Fatalf("unsafe name leaked: %s", output.String())
		}
		finding := assertFinding(t, report, "D9", "archive-index-invalid", slug)
		blobsRel, _ := store.IntentArchiveBlobsRel(slug)
		if finding.Path != blobsRel {
			t.Fatalf("unsafe name finding path = %q, want safe parent %q", finding.Path, blobsRel)
		}
	})
}

func TestDoctorD9ConfinedReadSwapRegressions(t *testing.T) {
	t.Run("ancestor-swap-outside-root", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "root")
		outside := filepath.Join(parent, "outside")
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Confined Ancestor", Slug: "confined-ancestor"}); err != nil {
			t.Fatal(err)
		}
		secret := "ancestor-outside-secret-marker"
		if err := os.MkdirAll(outside, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outside, "secret"), []byte(secret), 0o600); err != nil {
			t.Fatal(err)
		}
		featuresPath := filepath.Join(root, filepath.FromSlash(doctorD9FeaturesRel))
		originalPath := featuresPath + "-original"
		swapped := false
		boundary := &doctorD9OSBoundary{root: root}
		boundary.beforeOpen = func(rel string) {
			if rel != doctorD9FeaturesRel || swapped {
				return
			}
			swapped = true
			if err := os.Rename(featuresPath, originalPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, featuresPath); err != nil {
				t.Skipf("symlink fixture unavailable: %v", err)
			}
		}
		spy := &doctorD9BoundarySpy{delegate: boundary}
		previous := newDoctorD9Boundary
		newDoctorD9Boundary = func(string) doctorD9Boundary { return spy }
		t.Cleanup(func() { newDoctorD9Boundary = previous })

		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		doctorD9AssertUnsafeWithoutLeak(t, report, secret)
		doctorD9AssertNoForbiddenCapabilities(t, spy)
	})

	t.Run("final-component-swap-outside-root", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "root")
		outside := filepath.Join(parent, "outside-index.json")
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		const slug = "confined-final"
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Confined Final", Slug: slug}); err != nil {
			t.Fatal(err)
		}
		replacement := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, []byte("inside"), store.IntentArchiveWireRetained)
		doctorD9WriteArchive(t, root, store.IntentArchiveIndex{
			SchemaVersion: store.IntentArchiveSchemaVersion,
			Feature:       slug,
			Generations:   []store.IntentArchiveGeneration{doctorD9Generation(t, slug, replacement)},
		})
		doctorD9WriteBlob(t, root, slug, replacement.ContentSHA256, []byte("inside"))
		secret := "final-outside-secret-marker"
		if err := os.WriteFile(outside, []byte(secret), 0o600); err != nil {
			t.Fatal(err)
		}
		indexRel, _ := store.IntentArchiveIndexRel(slug)
		indexPath := filepath.Join(root, filepath.FromSlash(indexRel))
		originalPath := indexPath + ".original"
		swapped := false
		boundary := &doctorD9OSBoundary{root: root}
		boundary.afterOpen = func(rel string) {
			if rel != indexRel || swapped {
				return
			}
			swapped = true
			if err := os.Rename(indexPath, originalPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, indexPath); err != nil {
				t.Skipf("symlink fixture unavailable: %v", err)
			}
		}
		spy := &doctorD9BoundarySpy{delegate: boundary}
		previous := newDoctorD9Boundary
		newDoctorD9Boundary = func(string) doctorD9Boundary { return spy }
		t.Cleanup(func() { newDoctorD9Boundary = previous })

		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		doctorD9AssertUnsafeWithoutLeak(t, report, secret)
		doctorD9AssertNoForbiddenCapabilities(t, spy)
	})
}

func TestDoctorD9UnavailableBlobEvidenceNeverOffersDestructiveRepair(t *testing.T) {
	setup := func(t *testing.T, slug string) (string, *store.Store, string) {
		t.Helper()
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug}); err != nil {
			t.Fatal(err)
		}
		content := []byte("retained archive bytes")
		replacement := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, content, store.IntentArchiveWireRetained)
		doctorD9WriteArchive(t, root, store.IntentArchiveIndex{
			SchemaVersion: store.IntentArchiveSchemaVersion,
			Feature:       slug,
			Generations:   []store.IntentArchiveGeneration{doctorD9Generation(t, slug, replacement)},
		})
		doctorD9WriteBlob(t, root, slug, replacement.ContentSHA256, content)
		return root, s, doctorD9BlobRel(t, slug, replacement.ContentSHA256)
	}

	t.Run("injected-read-error", func(t *testing.T) {
		for _, faultCase := range []struct {
			name   string
			marker string
			err    error
		}{
			{name: "io", marker: "outside-secret-io-error", err: errors.New("outside-secret-io-error")},
			{name: "permission", marker: "outside-secret-permission-error", err: fmt.Errorf("outside-secret-permission-error: %w", fs.ErrPermission)},
		} {
			t.Run(faultCase.name, func(t *testing.T) {
				root, s, blobRel := setup(t, "read-unavailable-"+faultCase.name)
				fault := &doctorD9ReadFaultBoundary{
					doctorD9Boundary: &doctorD9OSBoundary{root: root},
					rel:              blobRel,
					err:              faultCase.err,
				}
				spy := &doctorD9BoundarySpy{delegate: fault}
				previous := newDoctorD9Boundary
				newDoctorD9Boundary = func(string) doctorD9Boundary { return spy }
				t.Cleanup(func() { newDoctorD9Boundary = previous })

				first, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
				if err != nil {
					t.Fatal(err)
				}
				second, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
				if err != nil {
					t.Fatal(err)
				}
				doctorD9AssertUnavailableFinding(t, first, faultCase.marker)
				doctorD9AssertUnavailableFinding(t, second, faultCase.marker)
				if fmt.Sprint(first) != fmt.Sprint(second) {
					t.Fatalf("unavailable-evidence report is nondeterministic:\n%#v\n%#v", first, second)
				}
				doctorD9AssertNoForbiddenCapabilities(t, spy)
			})
		}
	})

	t.Run("identity-race", func(t *testing.T) {
		root, s, blobRel := setup(t, "identity-race")
		blobPath := filepath.Join(root, filepath.FromSlash(blobRel))
		swapped := false
		boundary := &doctorD9OSBoundary{root: root}
		boundary.afterRead = func(rel string) {
			if rel != blobRel || swapped {
				return
			}
			swapped = true
			if err := os.Rename(blobPath, blobPath+".original"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(blobPath, []byte("replacement-secret-marker"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		spy := &doctorD9BoundarySpy{delegate: boundary}
		previous := newDoctorD9Boundary
		newDoctorD9Boundary = func(string) doctorD9Boundary { return spy }
		t.Cleanup(func() { newDoctorD9Boundary = previous })

		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		doctorD9AssertUnavailableFinding(t, report, "replacement-secret-marker")
		doctorD9AssertNoForbiddenCapabilities(t, spy)
	})
}

func TestDoctorD9PendingOwnershipSurvivesUnrelatedUnavailableBlob(t *testing.T) {
	for _, faultCase := range []struct {
		name string
		err  error
	}{
		{name: "permission", err: fs.ErrPermission},
		{name: "identity", err: errors.New("injected blob identity race")},
	} {
		t.Run(faultCase.name, func(t *testing.T) {
			root := t.TempDir()
			s, err := store.Init(root)
			if err != nil {
				t.Fatal(err)
			}
			const slug = "pending-plus-unavailable"
			if _, err := s.AddFeature(store.AddFeatureInput{Title: "Pending Plus Unavailable", Slug: slug}); err != nil {
				t.Fatal(err)
			}
			pendingBytes := []byte("owned pending bytes")
			unrelatedBytes := []byte("unrelated retained bytes")
			pending := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, pendingBytes, store.IntentArchiveWireRemovalPending)
			unrelated := doctorD9Replacement(store.IntentArchiveArtifactSpec, unrelatedBytes, store.IntentArchiveWireRetained)
			doctorD9WriteArchive(t, root, store.IntentArchiveIndex{
				SchemaVersion: store.IntentArchiveSchemaVersion,
				Feature:       slug,
				Generations: []store.IntentArchiveGeneration{
					doctorD9Generation(t, slug, pending),
					doctorD9Generation(t, slug, unrelated),
				},
			})
			doctorD9WriteBlob(t, root, slug, pending.ContentSHA256, pendingBytes)
			doctorD9WriteBlob(t, root, slug, unrelated.ContentSHA256, unrelatedBytes)
			fault := &doctorD9ReadFaultBoundary{
				doctorD9Boundary: &doctorD9OSBoundary{root: root},
				rel:              doctorD9BlobRel(t, slug, unrelated.ContentSHA256),
				err:              faultCase.err,
			}
			previous := newDoctorD9Boundary
			newDoctorD9Boundary = func(string) doctorD9Boundary { return fault }
			t.Cleanup(func() { newDoctorD9Boundary = previous })

			first, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
			if err != nil {
				t.Fatal(err)
			}
			second, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(first) != fmt.Sprint(second) {
				t.Fatalf("pending-plus-unavailable report is nondeterministic:\n%#v\n%#v", first, second)
			}
			if len(first.Findings) != 2 ||
				first.Findings[0].Code != "archive-purge-pending" ||
				first.Findings[1].Code != "persistent-evidence-unsafe" {
				t.Fatalf("pending-plus-unavailable findings = %#v", first.Findings)
			}
			pendingFinding := first.Findings[0]
			wantRoute := doctorD9BlobPurgeCommand(slug, []string{pending.ContentSHA256})
			if pendingFinding.Remediation != wantRoute ||
				strings.Contains(pendingFinding.Remediation, unrelated.ContentSHA256) {
				t.Fatalf("pending ownership route = %#v", pendingFinding)
			}
			unsafe := first.Findings[1]
			if unsafe.Remediation != "run tpatch feature intent-archive list "+slug ||
				strings.Contains(unsafe.Remediation, "purge") ||
				strings.Contains(unsafe.Remediation, "rm ") {
				t.Fatalf("unavailable unrelated evidence gained a destructive route: %#v", unsafe)
			}
			for _, finding := range first.Findings {
				switch finding.Code {
				case "archive-purge-pending", "persistent-evidence-unsafe":
				default:
					t.Fatalf("uncertain lower repair class escaped suppression: %#v", finding)
				}
			}
		})
	}

	t.Run("corrupt-index-does-not-infer-pending", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		const slug = "corrupt-pending-lookalike"
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Corrupt Pending Lookalike", Slug: slug}); err != nil {
			t.Fatal(err)
		}
		indexRel, _ := store.IntentArchiveIndexRel(slug)
		doctorD9Write(t, root, indexRel,
			`{"feature":"`+slug+`","purge_pending":true,"content_sha256":"`+strings.Repeat("a", 64)+`"`)
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Findings) != 1 || report.Findings[0].Code != "archive-index-invalid" {
			t.Fatalf("corrupt index inferred pending ownership: %#v", report.Findings)
		}
	})
}

func TestDoctorD9OnlyBypassesLegacyFeatureLoader(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		install func(*testing.T, string)
	}{
		{
			name: "malformed-status",
			install: func(t *testing.T, statusPath string) {
				t.Helper()
				if err := os.WriteFile(statusPath, []byte("{malformed"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlinked-status",
			install: func(t *testing.T, statusPath string) {
				t.Helper()
				outside := statusPath + ".outside"
				if err := os.WriteFile(outside, []byte("{outside-malformed"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(statusPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, statusPath); err != nil {
					t.Skipf("symlink fixture unavailable: %v", err)
				}
			},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := t.TempDir()
			s, err := store.Init(root)
			if err != nil {
				t.Fatal(err)
			}
			const slug = "loader-boundary"
			if _, err := s.AddFeature(store.AddFeatureInput{Title: "Loader Boundary", Slug: slug}); err != nil {
				t.Fatal(err)
			}
			statusPath := filepath.Join(root, ".tpatch", "features", slug, "status.json")
			fixture.install(t, statusPath)

			d9, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
			if err != nil {
				t.Fatalf("D9-only selection invoked the legacy loader: %v", err)
			}
			if len(d9.Findings) != 0 {
				t.Fatalf("D9-only selection consumed status.json: %#v", d9.Findings)
			}
			d1, err := RunDoctor(s, DoctorOptions{Checks: []string{"D1"}})
			if err != nil {
				t.Fatal(err)
			}
			assertFinding(t, d1, "D1", "feature-metadata-malformed", slug)
			all, err := RunDoctor(s, DoctorOptions{})
			if err != nil {
				t.Fatal(err)
			}
			assertFinding(t, all, "D1", "feature-metadata-malformed", slug)
		})
	}
}

func TestDoctorD9FixDryRunDeterminismPrivacyAndZeroWrite(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Deterministic", Slug: "deterministic"}); err != nil {
		t.Fatal(err)
	}
	doctorD9Mkdir(t, root, ".tpatch/local/intent-prepare/deterministic/stage-abcdefabcdef")
	before := doctorD9SnapshotTree(t, root)
	first, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	fixed, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	doctorD9AssertTreeEqual(t, before, doctorD9SnapshotTree(t, root))
	if !first.DryRun || !second.DryRun || fixed.DryRun || !fixed.Fix {
		t.Fatalf("doctor mode flags = first:%#v second:%#v fix:%#v", first, second, fixed)
	}
	var firstJSON, againJSON, firstHuman, againHuman bytes.Buffer
	if err := WriteDoctorJSON(&firstJSON, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteDoctorJSON(&againJSON, first); err != nil {
		t.Fatal(err)
	}
	WriteDoctorHuman(&firstHuman, first)
	WriteDoctorHuman(&againHuman, first)
	if firstJSON.String() != againJSON.String() || firstHuman.String() != againHuman.String() {
		t.Fatal("D9 renderers are not deterministic")
	}
	for _, output := range []string{firstJSON.String(), firstHuman.String()} {
		for _, forbidden := range []string{root, "pid", "timestamp", "duration", "holder identity", "seconds", "minutes", "hours"} {
			if strings.Contains(strings.ToLower(output), strings.ToLower(forbidden)) {
				t.Fatalf("D9 output contains private/clock/authority token %q:\n%s", forbidden, output)
			}
		}
	}
	if DoctorExitCode(first) != 0 || DoctorExitCode(fixed) != 0 {
		t.Fatalf("D9 warnings changed exit: dry=%d fix=%d", DoctorExitCode(first), DoctorExitCode(fixed))
	}
}

func TestDoctorD9ConcurrencyRows(t *testing.T) {
	t.Run("PIB-381", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("workspace authority is unsupported on this platform")
		}
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Live Authority", Slug: "live-authority"}); err != nil {
			t.Fatal(err)
		}
		doctorD9Write(t, root, ".tpatch/local/intent-prepare/live-authority/journal.json", "{}\n")
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatal(err)
		}

		type result struct {
			report DoctorReport
			err    error
		}
		results := make(chan result, 2)
		for range 2 {
			go func() {
				report, runErr := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
				results <- result{report: report, err: runErr}
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
		third, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		reports = append(reports, third)
		reacquired, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatalf("doctor perturbed authority release/acquisition: %v", err)
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
				"authority held",
				"authority is free",
				"no holder",
				"holder identity",
				"process id",
			} {
				if strings.Contains(strings.ToLower(output.String()), forbidden) {
					t.Fatalf("doctor made live-authority claim %q:\n%s", forbidden, output.String())
				}
			}
		}
		if rendered[0] != rendered[1] || rendered[1] != rendered[2] {
			t.Fatalf("doctors observed authority availability or each other:\n%s\n---\n%s\n---\n%s", rendered[0], rendered[1], rendered[2])
		}
	})
}

func TestDoctorD9RemovedJournalAndFreshCloneDoNotInventLoss(t *testing.T) {
	t.Run("removed-journal", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Removed Journal", Slug: "removed-journal"}); err != nil {
			t.Fatal(err)
		}
		lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", "removed-journal")
		if err := os.MkdirAll(lane, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(lane); err != nil {
			t.Fatal(err)
		}
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		doctorD9AssertNoJournalLossClaim(t, report)
	})

	t.Run("fresh-clone-healthy-archive", func(t *testing.T) {
		root := t.TempDir()
		s, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		const slug = "fresh-clone"
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Fresh Clone", Slug: slug}); err != nil {
			t.Fatal(err)
		}
		content := []byte("retained committed archive bytes")
		replacement := doctorD9Replacement(store.IntentArchiveArtifactAnalysis, content, store.IntentArchiveWireRetained)
		index := store.IntentArchiveIndex{
			SchemaVersion: store.IntentArchiveSchemaVersion,
			Feature:       slug,
			Generations:   []store.IntentArchiveGeneration{doctorD9Generation(t, slug, replacement)},
		}
		doctorD9WriteArchive(t, root, index)
		doctorD9WriteBlob(t, root, slug, replacement.ContentSHA256, content)
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Findings) != 0 {
			t.Fatalf("healthy committed archive was treated as loss evidence: %#v", report.Findings)
		}
		doctorD9AssertNoJournalLossClaim(t, report)
	})
}

func TestDoctorD9ConnectedCapabilitySpiesRecordZeroProbesProcessesAndWrites(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Spy", Slug: "spy"}); err != nil {
		t.Fatal(err)
	}
	doctorD9Write(t, root, ".tpatch/local/intent-prepare/spy/journal.json", "{}\n")
	previous := newDoctorD9Boundary
	spy := &doctorD9BoundarySpy{delegate: &doctorD9OSBoundary{root: root}}
	newDoctorD9Boundary = func(string) doctorD9Boundary { return spy }
	t.Cleanup(func() { newDoctorD9Boundary = previous })

	before := doctorD9SnapshotTree(t, root)
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D9"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) == 0 || spy.readCalls == 0 {
		t.Fatalf("spy was not connected to D9 reads: findings=%#v spy=%#v", report.Findings, spy)
	}
	doctorD9AssertNoForbiddenCapabilities(t, spy)
	doctorD9AssertTreeEqual(t, before, doctorD9SnapshotTree(t, root))
}

func TestDoctorD9SourceGuardForbidsAuthoritySyscallAndProcessAliases(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(current), "doctor_d9.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "os.OpenInRoot") {
		t.Fatal("doctor_d9.go lost confined ordinary-file opens")
	}
	for _, forbidden := range []string{
		"os.OpenRoot",
		"os.Open(",
		"os.ReadDir(",
		"os.ReadFile(",
		`Open(".")`,
		"SyscallConn",
		"syscall.Flock",
		"unix.Flock",
		"syscall.Fstatfs",
		"unix.Fstatfs",
		".Unlock()",
		"os/exec",
		"exec.Command",
		"intentlock.",
		"net/http",
		"http.",
		"provider.",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("doctor_d9.go contains forbidden authority/process alias %q", forbidden)
		}
	}
}

func TestDoctorD9RegistryOrderAndTaxonomySensitivity(t *testing.T) {
	if got := DoctorCheckIDs(); fmt.Sprint(got) != fmt.Sprint([]string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9"}) {
		t.Fatalf("doctor registry = %#v", got)
	}
	expected := []string{
		"archive-blob-corrupt",
		"archive-blob-dangling",
		"archive-index-invalid",
		"archive-index-storage-inconsistent",
		"archive-orphan",
		"archive-purge-pending",
		"persistent-evidence-unsafe",
		"prepare-abandoned-evidence",
		"prepare-preimage-stale",
		"prepare-stage-stale",
		"prepare-temp-stale",
		"prepare-transaction-pending",
	}
	if err := validateDoctorD9Taxonomy(expected); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(doctorD9PersistentEvidenceCodes[:], expected) {
		t.Fatalf("production taxonomy = %v, want %v", doctorD9PersistentEvidenceCodes, expected)
	}
	if err := validateDoctorD9Taxonomy(append(append([]string(nil), expected...), "synthetic-residue")); err == nil {
		t.Fatal("D9 taxonomy guard accepted a synthetic residue class")
	}
	if err := validateDoctorD9Taxonomy(expected[:len(expected)-1]); err == nil {
		t.Fatal("D9 taxonomy guard accepted a missing residue class")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("D9 production emission accepted an unregistered residue class")
		}
	}()
	doctorD9AddFinding(&doctorContext{}, DoctorFinding{CheckID: "D9", Code: "synthetic-residue"})
}

type doctorD9BoundarySpy struct {
	delegate      doctorD9Boundary
	readCalls     int
	openRootCalls int
	openDotCalls  int
	controlCalls  int
	flockCalls    int
	fstatfsCalls  int
	unlockCalls   int
	processCalls  int
	writeCalls    int
}

type doctorD9ReadFaultBoundary struct {
	doctorD9Boundary
	rel string
	err error
}

func (boundary *doctorD9ReadFaultBoundary) ReadRegular(rel string, maxBytes int64) ([]byte, fs.FileInfo, error) {
	if rel == boundary.rel {
		return nil, nil, boundary.err
	}
	return boundary.doctorD9Boundary.ReadRegular(rel, maxBytes)
}

func (spy *doctorD9BoundarySpy) Lstat(rel string) (fs.FileInfo, error) {
	spy.readCalls++
	return spy.delegate.Lstat(rel)
}

func (spy *doctorD9BoundarySpy) ReadDir(rel string) ([]os.DirEntry, error) {
	spy.readCalls++
	return spy.delegate.ReadDir(rel)
}

func (spy *doctorD9BoundarySpy) ReadRegular(rel string, maxBytes int64) ([]byte, fs.FileInfo, error) {
	spy.readCalls++
	return spy.delegate.ReadRegular(rel, maxBytes)
}

func (spy *doctorD9BoundarySpy) OpenRoot(root string) error {
	spy.openRootCalls++
	return errors.New("unexpected OpenRoot")
}

func (spy *doctorD9BoundarySpy) OpenDot() error {
	spy.openDotCalls++
	return errors.New("unexpected OpenDot")
}

func (spy *doctorD9BoundarySpy) Control() error {
	spy.controlCalls++
	return errors.New("unexpected Control")
}

func (spy *doctorD9BoundarySpy) Flock() error {
	spy.flockCalls++
	return errors.New("unexpected Flock")
}

func (spy *doctorD9BoundarySpy) Fstatfs() error {
	spy.fstatfsCalls++
	return errors.New("unexpected Fstatfs")
}

func (spy *doctorD9BoundarySpy) Unlock() error {
	spy.unlockCalls++
	return errors.New("unexpected Unlock")
}

func (spy *doctorD9BoundarySpy) RunProcess(name string, args ...string) error {
	spy.processCalls++
	return errors.New("unexpected process")
}

func (spy *doctorD9BoundarySpy) Write(rel string, data []byte) error {
	spy.writeCalls++
	return errors.New("unexpected write")
}

func doctorD9AssertNoForbiddenCapabilities(t *testing.T, spy *doctorD9BoundarySpy) {
	t.Helper()
	if spy.openRootCalls != 0 || spy.openDotCalls != 0 || spy.controlCalls != 0 ||
		spy.flockCalls != 0 || spy.fstatfsCalls != 0 || spy.unlockCalls != 0 ||
		spy.processCalls != 0 || spy.writeCalls != 0 {
		t.Fatalf("D9 used a forbidden capability: %#v", spy)
	}
}

func doctorD9AssertUnsafeWithoutLeak(t *testing.T, report DoctorReport, secret string) {
	t.Helper()
	finding := doctorD9FindingByCode(t, report, "persistent-evidence-unsafe")
	if finding.Severity != "warning" || finding.Fixable {
		t.Fatalf("unsafe confined-read finding = %#v", finding)
	}
	var output bytes.Buffer
	if err := WriteDoctorJSON(&output, report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), secret) {
		t.Fatalf("confined read leaked outside marker %q:\n%s", secret, output.String())
	}
}

func doctorD9AssertUnavailableFinding(t *testing.T, report DoctorReport, secret string) {
	t.Helper()
	finding := doctorD9FindingByCode(t, report, "persistent-evidence-unsafe")
	if finding.Severity != "warning" || finding.Fixable ||
		!strings.HasPrefix(finding.Remediation, "run tpatch feature intent-archive list ") {
		t.Fatalf("unavailable archive finding = %#v", finding)
	}
	var output bytes.Buffer
	if err := WriteDoctorJSON(&output, report); err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(output.String())
	for _, forbidden := range []string{strings.ToLower(secret), "rm -", " --blob ", "--orphans", "intent-archive purge"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("unavailable evidence offered destructive/leaking advice %q:\n%s", forbidden, output.String())
		}
	}
}

func doctorD9FindingByCode(t *testing.T, report DoctorReport, code string) DoctorFinding {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.CheckID == "D9" && finding.Code == code {
			return finding
		}
	}
	t.Fatalf("missing D9 finding %q in %#v", code, report.Findings)
	return DoctorFinding{}
}

func doctorD9Replacement(id store.IntentArchiveArtifactID, data []byte, state store.IntentArchiveWireState) store.IntentArchiveReplacement {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	pathByID := map[store.IntentArchiveArtifactID]string{
		store.IntentArchiveArtifactAnalysis:        "analysis.md",
		store.IntentArchiveArtifactAnalysisSidecar: "artifacts/analysis.json",
		store.IntentArchiveArtifactExploration:     "exploration.md",
		store.IntentArchiveArtifactSpec:            "spec.md",
	}
	replacement := store.IntentArchiveReplacement{
		ArtifactID:    id,
		Path:          pathByID[id],
		ContentSHA256: hash,
		Blob:          hash,
		SizeBytes:     int64(len(data)),
	}
	switch state {
	case store.IntentArchiveWireRemovalPending:
		replacement.PurgePending = true
	case store.IntentArchiveWireTombstoned:
		replacement.Blob = ""
		replacement.Purged = true
	}
	return replacement
}

func doctorD9Generation(t *testing.T, slug string, replacement store.IntentArchiveReplacement) store.IntentArchiveGeneration {
	t.Helper()
	id, _, err := store.ComputeIntentArchiveGenerationID(slug, []store.IntentArchiveReplacement{replacement})
	if err != nil {
		t.Fatal(err)
	}
	return store.IntentArchiveGeneration{
		GenerationID: id,
		Mode:         store.IntentArchiveModeRegenerate,
		Replaced:     []store.IntentArchiveReplacement{replacement},
	}
}

func doctorD9WriteArchive(t *testing.T, root string, index store.IntentArchiveIndex) {
	t.Helper()
	data, err := store.EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := store.IntentArchiveIndexRel(index.Feature)
	if err != nil {
		t.Fatal(err)
	}
	doctorD9Write(t, root, indexRel, string(data))
}

func doctorD9WriteBlob(t *testing.T, root, slug, hash string, data []byte) {
	t.Helper()
	rel := doctorD9BlobRel(t, slug, hash)
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func doctorD9Write(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func doctorD9BlobRel(t *testing.T, slug, hash string) string {
	t.Helper()
	rel, err := store.IntentArchiveBlobRel(slug, hash)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func doctorD9AssertInstanceOnce(t *testing.T, text, instance string) {
	t.Helper()
	if count := strings.Count(text, instance); count != 1 {
		t.Fatalf("instance %q occurs %d times, want exactly once in %q", instance, count, text)
	}
}

func doctorD9Mkdir(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
}

func doctorD9Unique(values []string) []string {
	var out []string
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func doctorD9SnapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := os.Lstat(filePath)
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			value += ":" + hex.EncodeToString(sum[:])
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(filePath)
			if err != nil {
				return err
			}
			value += ":" + target
		}
		snapshot[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func doctorD9AssertTreeEqual(t *testing.T, want, got map[string]string) {
	t.Helper()
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if !bytes.Equal(wantJSON, gotJSON) {
		t.Fatalf("tree changed:\nwant %s\ngot  %s", wantJSON, gotJSON)
	}
}

func doctorD9AssertNoJournalLossClaim(t *testing.T, report DoctorReport) {
	t.Helper()
	var output bytes.Buffer
	if err := WriteDoctorJSON(&output, report); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"lost journal",
		"journal loss detected",
		"interrupted transaction detected",
		"recover the removed journal",
	} {
		if strings.Contains(strings.ToLower(output.String()), forbidden) {
			t.Fatalf("D9 invented journal-loss evidence %q:\n%s", forbidden, output.String())
		}
	}
}

func validateDoctorD9Taxonomy(got []string) error {
	want := map[string]bool{
		"archive-blob-corrupt":               true,
		"archive-blob-dangling":              true,
		"archive-index-invalid":              true,
		"archive-index-storage-inconsistent": true,
		"archive-orphan":                     true,
		"archive-purge-pending":              true,
		"persistent-evidence-unsafe":         true,
		"prepare-abandoned-evidence":         true,
		"prepare-preimage-stale":             true,
		"prepare-stage-stale":                true,
		"prepare-temp-stale":                 true,
		"prepare-transaction-pending":        true,
	}
	seen := map[string]bool{}
	for _, code := range got {
		if !want[code] || seen[code] {
			return fmt.Errorf("unexpected or duplicate D9 taxonomy code %q", code)
		}
		seen[code] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("D9 taxonomy has %d codes, want %d", len(seen), len(want))
	}
	return nil
}
