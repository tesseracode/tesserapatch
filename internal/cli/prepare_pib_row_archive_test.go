//go:build (linux && !android) || (darwin && !ios)

package cli

// Owning acceptance tests for the `--regenerate`, archive and redaction rows of
// the aggregate ledger.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func pibRowArchiveDir(root, slug string) string {
	return filepath.Join(pibRowFeature(root, slug), "artifacts", "intent-archive")
}

func pibRowSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// pibRowPublishedWorkspace runs one successful default-mode publication and
// returns the workspace it published into.
func pibRowPublishedWorkspace(t *testing.T, title string) (string, string) {
	t.Helper()
	root, slug := prepareS4Workspace(t, title)
	if code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	); code != 0 {
		t.Fatalf("initial prepare = %d: %s", code, stderr)
	}
	return root, slug
}

// TestPIBRowRegenerateContracts owns the `--regenerate` rows.
func TestPIBRowRegenerateContracts(t *testing.T) {
	t.Run("PIB-066", func(t *testing.T) {
		root, slug := pibRowPublishedWorkspace(t, "PIB row 066")
		analysis := filepath.Join(pibRowFeature(root, slug), "analysis.md")
		if err := os.Remove(analysis); err != nil {
			t.Fatal(err)
		}
		// A directory at a required artifact path is unreadable for every
		// identity, so the fixture does not depend on the running uid.
		if err := os.Mkdir(analysis, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(pibRowArchiveDir(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-066: the fixture already holds archive state: %v", err)
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "artifact-unsafe" {
			t.Fatalf("--regenerate over an unreadable artifact = exit %d refusal=%#v", code, report.Refusal)
		}
		if report.Archive != nil {
			t.Fatalf("PIB-066: the refused run reported an archive generation: %#v", report.Archive)
		}
		if _, err := os.Stat(pibRowArchiveDir(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-066: the refused run created archive state: %v", err)
		}
	})

	t.Run("PIB-067", func(t *testing.T) {
		root, slug := pibRowPublishedWorkspace(t, "PIB row 067")
		spec := filepath.Join(pibRowFeature(root, slug), "spec.md")
		if err := os.WriteFile(spec, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "published" {
			t.Fatalf("--regenerate over a zero-byte spec.md = exit %d stderr=%q outcome=%q",
				code, stderr, report.Outcome)
		}
		replaced, err := os.ReadFile(spec)
		if err != nil || len(bytes.TrimSpace(replaced)) == 0 {
			t.Fatalf("PIB-067: the replacement spec.md is empty: %v", err)
		}
		emptyHash := pibRowSHA256(nil)
		blob := filepath.Join(pibRowArchiveDir(root, slug), "blobs", emptyHash+".blob")
		info, err := os.Stat(blob)
		if err != nil || info.Size() != 0 {
			t.Fatalf("PIB-067: the zero-byte prior image was not archived at %s: %v", blob, err)
		}
	})

	t.Run("PIB-070", func(t *testing.T) {
		root, slug := pibRowPublishedWorkspace(t, "PIB row 070")
		feature := pibRowFeature(root, slug)
		priors := map[string][]byte{
			"analysis.md":             []byte("prior analysis bytes\n"),
			"spec.md":                 []byte("prior specification bytes\n"),
			"exploration.md":          []byte("prior exploration bytes\n"),
			"artifacts/analysis.json": []byte("{\"prior\":true}\n"),
		}
		for rel, body := range priors {
			if err := os.WriteFile(filepath.Join(feature, filepath.FromSlash(rel)), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "published" {
			t.Fatalf("--regenerate = exit %d stderr=%q outcome=%q", code, stderr, report.Outcome)
		}
		for rel, prior := range priors {
			got, err := os.ReadFile(filepath.Join(feature, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(got, prior) {
				t.Fatalf("PIB-070: --regenerate did not replace %s", rel)
			}
		}
		if report.Archive == nil || report.Archive.GenerationID == "" {
			t.Fatalf("PIB-070: the replacement published no archive generation: %#v", report.Archive)
		}
		if len(report.Artifacts) != 4 {
			t.Fatalf("PIB-070: the report covers %d artifacts, want all four", len(report.Artifacts))
		}
		for _, artifact := range report.Artifacts {
			if artifact.Disposition == "untouched" || artifact.Disposition == "preserved" {
				t.Fatalf("PIB-070: %s was not in the replacement scope (disposition %q)",
					artifact.ID, artifact.Disposition)
			}
		}
	})

	t.Run("PIB-330", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 330")
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" {
			t.Fatalf("--regenerate over an empty bundle = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if report.Archive != nil {
			t.Fatalf("PIB-330: an all-absent regenerate reported an archive generation: %#v", report.Archive)
		}
		if len(report.OrphanBlobs) != 0 {
			t.Fatalf("PIB-330: an all-absent regenerate created blobs: %v", report.OrphanBlobs)
		}
		if _, err := os.Stat(pibRowArchiveDir(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-330: an all-absent regenerate created the archive directory: %v", err)
		}
	})

	t.Run("PIB-167", func(t *testing.T) {
		// Two workspaces holding identical trees under the same slug: the
		// heuristic generator and the report are functions of the tree alone.
		firstRoot, slug := prepareS4Workspace(t, "PIB row 167")
		secondRoot, secondSlug := prepareS4Workspace(t, "PIB row 167")
		if slug != secondSlug {
			t.Fatalf("PIB-167: the two fixtures disagree on the slug (%q versus %q)", slug, secondSlug)
		}
		prior := []byte("identical prior bytes\n")
		for _, root := range []string{firstRoot, secondRoot} {
			for _, rel := range []string{"analysis.md", "spec.md", "exploration.md"} {
				if err := os.WriteFile(
					filepath.Join(pibRowFeature(root, slug), rel), prior, 0o644,
				); err != nil {
					t.Fatal(err)
				}
			}
		}
		reports := map[string]string{}
		for _, root := range []string{firstRoot, secondRoot} {
			code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
			)
			if code != 0 || stderr != "" {
				t.Fatalf("heuristic regenerate in %s = exit %d stderr=%q", root, code, stderr)
			}
			reports[root] = stdout
		}
		if reports[firstRoot] != reports[secondRoot] {
			t.Fatalf("PIB-167: identical trees produced different reports\n%s\n%s",
				reports[firstRoot], reports[secondRoot])
		}
		for _, rel := range []string{
			"analysis.md", "spec.md", "exploration.md", "artifacts/analysis.json",
		} {
			first, err := os.ReadFile(filepath.Join(pibRowFeature(firstRoot, slug), filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			second, err := os.ReadFile(filepath.Join(pibRowFeature(secondRoot, slug), filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("PIB-167: %s differs between identical heuristic runs", rel)
			}
		}
	})
}

// pibRowSensitiveFixtures is the closed six-class trigger set: one deliberately
// sensitive line per §9 redaction class, each of which trips exactly its class.
func pibRowSensitiveFixtures() []struct {
	class string
	line  string
} {
	return []struct {
		class string
		line  string
	}{
		{"private-key", "-----BEGIN OPENSSH PRIVATE KEY-----"},
		{"connection-url", "postgres://db.invalid/app"},
		{"email-pii", "owner@example.invalid"},
		{"credential-assignment", "token = " + strings.Repeat("q", 20)},
		{"bearer-or-key-token", "Authorization: Bearer " + strings.Repeat("z", 24)},
		{"home-absolute-path", "see /home/operator/notes.txt"},
	}
}

// TestPIBRowArchiveRedactionContracts owns the redaction-gate rows.
func TestPIBRowArchiveRedactionContracts(t *testing.T) {
	t.Run("PIB-263", func(t *testing.T) {
		root, slug := pibRowPublishedWorkspace(t, "PIB row 263")
		secret := "Authorization: Bearer " + strings.Repeat("k", 28)
		spec := filepath.Join(pibRowFeature(root, slug), "spec.md")
		if err := os.WriteFile(spec, []byte("prior specification\n"+secret+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil ||
			report.Refusal.Code != "archive-content-refused-sensitive" {
			t.Fatalf("sensitive prior bytes = exit %d refusal=%#v", code, report.Refusal)
		}
		if !strings.Contains(report.Refusal.Message, "spec") {
			t.Fatalf("PIB-263: the refusal does not name the artifact: %q", report.Refusal.Message)
		}
		if !strings.Contains(report.Refusal.Message, "bearer-or-key-token") {
			t.Fatalf("PIB-263: the refusal does not name the matched class: %q", report.Refusal.Message)
		}
		for _, forbidden := range []string{
			strings.Repeat("k", 28), "offset", "line_number", "excerpt", "matched_bytes",
		} {
			if strings.Contains(stdout, forbidden) {
				t.Fatalf("PIB-263: the report leaked %q:\n%s", forbidden, stdout)
			}
		}
	})

	t.Run("PIB-264", func(t *testing.T) {
		observed := 0
		for _, fixture := range pibRowSensitiveFixtures() {
			root, slug := pibRowPublishedWorkspace(t, "PIB row 264 "+fixture.class)
			spec := filepath.Join(pibRowFeature(root, slug), "spec.md")
			if err := os.WriteFile(
				spec, []byte("prior specification\n"+fixture.line+"\n"), 0o644,
			); err != nil {
				t.Fatal(err)
			}
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "prepare", slug,
				"--regenerate", "--allow-heuristic", "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			if code != 3 || report.Refusal == nil ||
				report.Refusal.Code != "archive-content-refused-sensitive" {
				t.Fatalf("PIB-264 %s = exit %d refusal=%#v", fixture.class, code, report.Refusal)
			}
			if !strings.Contains(report.Refusal.Message, fixture.class) {
				t.Fatalf("PIB-264 %s: the refusal names another class: %q",
					fixture.class, report.Refusal.Message)
			}
			observed++
		}
		if observed != 6 {
			t.Fatalf("PIB-264: observed %d of the six classes", observed)
		}
	})

	t.Run("PIB-265", func(t *testing.T) {
		root, slug := pibRowPublishedWorkspace(t, "PIB row 265")
		spec := filepath.Join(pibRowFeature(root, slug), "spec.md")
		secret := "Authorization: Bearer " + strings.Repeat("m", 26)
		if err := os.WriteFile(spec, []byte("prior specification\n"+secret+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		refusedCode, refusedOut, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		refused := prepareS4Report(t, refusedOut)
		if refusedCode != 3 || refused.Refusal == nil ||
			refused.Refusal.Code != "archive-content-refused-sensitive" {
			t.Fatalf("PIB-265: the first regenerate = exit %d refusal=%#v", refusedCode, refused.Refusal)
		}
		if _, err := os.Stat(pibRowArchiveDir(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-265: the refusal left archive state behind: %v", err)
		}
		cleaned := []byte("prior specification without the sensitive line\n")
		if err := os.WriteFile(spec, cleaned, 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "published" {
			t.Fatalf("PIB-265: the cleaned regenerate = exit %d stderr=%q outcome=%q",
				code, stderr, report.Outcome)
		}
		blob := filepath.Join(pibRowArchiveDir(root, slug), "blobs", pibRowSHA256(cleaned)+".blob")
		archived, err := os.ReadFile(blob)
		if err != nil || !bytes.Equal(archived, cleaned) {
			t.Fatalf("PIB-265: the cleaned bytes were not archived at %s: %v", blob, err)
		}
	})
}

// TestPIBRowArchiveEvidenceContracts owns the rollback-evidence, blob-reuse and
// abandoned-directory rows that previously had only skipping targets.
func TestPIBRowArchiveEvidenceContracts(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Fatal("this build tag selects the authority-supported targets; the constant disagrees")
	}

	t.Run("PIB-316", func(t *testing.T) {
		result := prepareS5RollbackWithTwoArchiveBlobs(t, true)
		report := prepareS4Report(t, result.stdout)
		if report.Outcome != "rolled-back" {
			t.Fatalf("PIB-316: outcome = %q, want rolled-back", report.Outcome)
		}
		if len(result.hashes) != 2 {
			t.Fatalf("PIB-316: the fixture wrote %d blobs, want 2", len(result.hashes))
		}
		for _, hash := range result.hashes {
			if countPrepareS5String(report.OrphanBlobs, hash) != 1 {
				t.Fatalf("PIB-316: blob %s is not listed exactly once: %v", hash, report.OrphanBlobs)
			}
		}
		command := "tpatch feature intent-archive purge " + result.slug + " --orphans --yes"
		named := 0
		for _, advisory := range report.Advisories {
			if strings.Contains(advisory.Message, command) {
				named++
			}
		}
		if named != 1 {
			t.Fatalf("PIB-316: the exact purge command appears %d times: %#v", named, report.Advisories)
		}
		if err := validatePrepareS5RollbackOrphanClaim(result.stdout); err != nil {
			t.Fatalf("PIB-316: the shipped report failed its own guard: %v", err)
		}
		if err := validatePrepareS5RollbackOrphanClaim(
			result.stdout + "\nThe working tree is byte-identical.\n",
		); err == nil {
			t.Fatal("PIB-316: the guard accepted a byte-identical claim beside orphan blobs")
		}
	})

	t.Run("PIB-317", func(t *testing.T) {
		result := prepareS5RollbackWithTwoArchiveBlobs(t, true)
		report := prepareS4Report(t, result.stdout)
		if len(report.OrphanBlobs) != len(result.hashes) {
			t.Fatalf("PIB-317: reported orphans = %d, blob files created = %d",
				len(report.OrphanBlobs), len(result.hashes))
		}
		for _, hash := range result.hashes {
			if countPrepareS5String(report.OrphanBlobs, hash) != 1 {
				t.Fatalf("PIB-317: created blob %s is not reported exactly once: %v",
					hash, report.OrphanBlobs)
			}
		}
	})

	t.Run("PIB-318", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 318")
		prepareS4WriteReadyBundle(t, root, slug, false)
		statusPath := filepath.Join(pibRowFeature(root, slug), "status.json")
		before, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		state := &prepareS5StatusPostRenameFault{}
		old := prepareIntentpubRootOps
		t.Cleanup(func() { prepareIntentpubRootOps = old })
		prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
			return &prepareS5StatusPostRenameFaultOps{
				RootOps: intentpub.NewRootOps(rooted),
				state:   state,
			}
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		prepareIntentpubRootOps = old
		if !state.renamed || code != 6 {
			t.Fatalf("PIB-318: the CP10 fixture did not fire: renamed=%v exit=%d\n%s",
				state.renamed, code, stdout)
		}
		crashed, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(pibRowLaneJournal(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-318: the single-rename mode left a journal: %v", err)
		}
		var document map[string]any
		if err := json.Unmarshal(crashed, &document); err != nil {
			t.Fatalf("PIB-318: status.json is partial after the crash: %v", err)
		}
		if !bytes.Equal(crashed, before) && document["state"] != "defined" {
			t.Fatalf("PIB-318: status.json is neither the old nor the new image:\n%s", crashed)
		}
		nextCode, nextOut, nextErr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if nextCode != 0 || nextErr != "" {
			t.Fatalf("PIB-318: the next run = exit %d stderr=%q\n%s", nextCode, nextErr, nextOut)
		}
	})

	t.Run("PIB-319", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 319")
		prior := []byte("preexisting archive blob bytes\n")
		hash := pibRowSHA256(prior)
		blobPath := filepath.Join(pibRowArchiveDir(root, slug), "blobs", hash+".blob")
		if err := os.MkdirAll(filepath.Dir(blobPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(blobPath, prior, 0o644); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(blobPath)
		if err != nil {
			t.Fatal(err)
		}
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = authority.Release() }()
		blobRel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		result, err := newPrepareArchiveStorage(authority, nil).PublishBlob(blobRel, hash, prior)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Reused || result.Committed {
			t.Fatalf("PIB-319: the retry rewrote the blob instead of reusing it: %#v", result)
		}
		after, err := os.Stat(blobPath)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
			t.Fatal("PIB-319: the existing blob's inode or mtime changed")
		}
	})

	t.Run("PIB-320", func(t *testing.T) {
		root, slug := pibRowPublishedWorkspace(t, "PIB row 320")
		lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
		existing := filepath.Join(lane, "abandoned-111111111111")
		if err := os.MkdirAll(existing, 0o700); err != nil {
			t.Fatal(err)
		}
		marker := []byte("existing evidence\n")
		if err := os.WriteFile(filepath.Join(existing, "marker"), marker, 0o600); err != nil {
			t.Fatal(err)
		}
		prepareS5InterruptAfterJournal(t, root, slug, "--regenerate", "--allow-heuristic")
		if _, err := os.Stat(pibRowLaneJournal(root, slug)); err != nil {
			t.Fatalf("PIB-320: the interruption left no journal: %v", err)
		}
		stale := filepath.Join(lane, "stage-aaaaaaaaaaaa")
		if err := os.MkdirAll(stale, 0o700); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		); code != 0 {
			t.Fatalf("PIB-320: recovery = exit %d stderr=%q", code, stderr)
		}
		got, err := os.ReadFile(filepath.Join(existing, "marker"))
		if err != nil || !bytes.Equal(got, marker) {
			t.Fatalf("PIB-320: recovery touched the abandoned directory: %v %q", err, got)
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Fatalf("PIB-320: recovery left the stale stage directory: %v", err)
		}
		if _, err := os.Stat(pibRowLaneJournal(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-320: recovery left the journal: %v", err)
		}
		// A later abandon adds a second directory rather than merging with the
		// first, so both evidence sets survive independently.
		second := filepath.Join(lane, "stage-bbbbbbbbbbbb")
		if err := os.MkdirAll(second, 0o700); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--abandon-transaction", "--yes", "--json", "--quiet",
		); code != 0 {
			t.Fatalf("PIB-320: abandon = exit %d stderr=%q", code, stderr)
		}
		entries, err := os.ReadDir(lane)
		if err != nil {
			t.Fatal(err)
		}
		abandoned := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "abandoned-") {
				abandoned++
			}
		}
		if abandoned < 2 {
			t.Fatalf("PIB-320: the second abandon merged into the first (%d directories)", abandoned)
		}
		if got, err := os.ReadFile(filepath.Join(existing, "marker")); err != nil || !bytes.Equal(got, marker) {
			t.Fatalf("PIB-320: the first abandoned directory was rewritten: %v %q", err, got)
		}
	})
}

// TestPIBRowCheckInsidePublicationWindow owns PIB-206: a `--check` executed
// while a mutating publication is parked inside its deterministic window.
//
// The row's asserted observable is "reports what is on disk; a mid-rename
// artifact may be `unstable`; no crash, no lock, no write". What is on disk at
// the parked point is the *pre-publication* tree — the mutating run has staged
// into the local lane and has not yet performed a single canonical rename
// (§7.1, `beforePrepareSetRevalidation` precedes `publishPrepareTransaction`).
// The truthful report is therefore `not_ready` with none of the three required
// artifacts present, and §10.6's report-bearing exit envelope prints that
// verdict on stderr as a single `error:` line. Asserting empty stderr here
// would suppress a valid diagnostic and would contradict "reports what is on
// disk", so the concurrency obligation is asserted as what it actually is: no
// second authority acquisition, a byte-identical `.tpatch` tree across the
// check, the exact reporting exit class, and the exact reporting line.
func TestPIBRowCheckInsidePublicationWindow(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Fatal("this build tag selects the authority-supported targets; the constant disagrees")
	}
	root, slug := prepareS4Workspace(t, "PIB row 206")
	entered := make(chan struct{})
	release := make(chan struct{})
	oldWindow := beforePrepareSetRevalidation
	oldAcquire := prepareAcquireAuthority
	acquires := 0
	t.Cleanup(func() {
		beforePrepareSetRevalidation = oldWindow
		prepareAcquireAuthority = oldAcquire
	})
	prepareAcquireAuthority = func(repoRoot string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return oldAcquire(repoRoot)
	}
	beforePrepareSetRevalidation = func() {
		close(entered)
		<-release
	}
	type publication struct {
		code   int
		stdout string
		stderr string
	}
	done := make(chan publication, 1)
	go func() {
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		done <- publication{code: code, stdout: stdout, stderr: stderr}
	}()
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("PIB-206: the mutating run never reached the publication window")
	}
	if acquires != 1 {
		t.Fatalf("PIB-206: mutator authority acquisitions = %d, want 1", acquires)
	}

	before := readTree(t, filepath.Join(root, ".tpatch"))
	checkCode, checkOut, checkErr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--check", "--json", "--quiet",
	)
	after := readTree(t, filepath.Join(root, ".tpatch"))
	close(release)
	published := <-done

	if acquires != 1 {
		t.Fatalf("PIB-206: --check acquired the workspace authority (%d acquisitions)", acquires)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("PIB-206: --check wrote inside the publication window")
	}
	// No crash: the read-only check returns the shipped report-bearing exit for
	// a structurally not-ready bundle, not a panic and not cobra's generic 1.
	if checkCode != 2 {
		t.Fatalf("PIB-206: --check exit = %d, want the not_ready reporting exit 2\n%s", checkCode, checkOut)
	}
	// The valid diagnostic is asserted verbatim rather than suppressed.
	wantLine := "error: prepare --check " + slug +
		": not_ready (0 of 3 required artifacts are present-nonempty)\n"
	if checkErr != wantLine {
		t.Fatalf("PIB-206: --check stderr = %q, want %q", checkErr, wantLine)
	}
	// Reports what is on disk: the pre-publication tree, artifact by artifact.
	var observed intent.Report
	if err := json.Unmarshal([]byte(checkOut), &observed); err != nil {
		t.Fatalf("PIB-206: --check emitted no parseable report: %v\n%s", err, checkOut)
	}
	if observed.Slug != slug || len(observed.Artifacts) != 4 {
		t.Fatalf("PIB-206: --check did not report what is on disk: %#v", observed)
	}
	if observed.Overall.StructuralReadiness != intent.ReadinessNotReady ||
		observed.Overall.RequiredTotal != 3 || observed.Overall.RequiredSatisfied != 0 {
		t.Fatalf("PIB-206: --check counters = %#v", observed.Overall)
	}
	for _, artifact := range observed.Artifacts {
		// A mid-rename artifact may be `unstable`; nothing on this parked tree
		// may be reported as already published.
		if artifact.State == intent.StatePresentNonempty {
			t.Fatalf("PIB-206: --check reported %s as published before the first rename: %#v",
				artifact.ID, artifact)
		}
		if artifact.Provenance != intent.ProvenanceUnknown {
			t.Fatalf("PIB-206: --check claimed provenance for %s: %#v", artifact.ID, artifact)
		}
	}
	if published.code != 0 || published.stderr != "" {
		t.Fatalf("PIB-206: the parked publication did not finish: exit %d stderr=%q\n%s",
			published.code, published.stderr, published.stdout)
	}
	// Control: once the window has closed, the same read-only check over the
	// same workspace reports the published tree and emits no diagnostic — so
	// the assertions above pin the window, not a permanently broken check.
	settledCode, settledOut, settledErr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--check", "--json", "--quiet",
	)
	if settledCode != 0 || settledErr != "" {
		t.Fatalf("PIB-206: --check after the window = exit %d stderr=%q\n%s",
			settledCode, settledErr, settledOut)
	}
	if acquires != 1 {
		t.Fatalf("PIB-206: the settled --check acquired the authority (%d acquisitions)", acquires)
	}
}
