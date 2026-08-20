package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func TestPrepareS5ProvenanceBoundaryRows(t *testing.T) {
	t.Run("PIB-140", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 provenance")
		if code, _, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet"); code != 0 {
			t.Fatalf("prepare = %d: %s", code, stderr)
		}
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("prepare --check = %d stderr=%q\n%s", code, stderr, stdout)
		}
		for _, artifact := range reportArtifacts(t, decodeReport(t, stdout)) {
			if artifact["provenance"] != "unknown" {
				t.Fatalf("provenance after mutating prepare = %v", artifact["provenance"])
			}
		}
	})

	t.Run("PIB-141", func(t *testing.T) {
		for _, rel := range []string{
			"internal/intent/inspect.go",
			"internal/intent/render.go",
			"internal/cli/prepare.go",
		} {
			source, err := os.ReadFile(filepath.Join(avpRepoRoot(t), filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(source), "intent-archive") ||
				strings.Contains(string(source), "IntentArchive") {
				t.Fatalf("%s reads or names the intent archive", rel)
			}
		}
	})

	t.Run("PIB-142", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 no authorship")
		_, mutating, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		_, checking, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		joined := strings.ToLower(mutating + checking)
		for _, forbidden := range []string{
			"path a", "path_a", "path b", "path_b",
			"authored by", "authored_by", "author identity",
		} {
			if strings.Contains(joined, forbidden) {
				t.Fatalf("report asserted provenance token %q:\n%s", forbidden, joined)
			}
		}
	})

	t.Run("PIB-143", func(t *testing.T) {
		root := avpRepoRoot(t)
		sources, err := prepareS5ProvenanceInferenceSources(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5ProvenanceInferenceSources(sources); err != nil {
			t.Fatal(err)
		}
		contract, err := os.ReadFile(filepath.Join(
			root, "docs", "prds", "PRD-artifact-validation-and-provenance.md",
		))
		if err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5ArchiveProvenanceContract(string(contract)); err != nil {
			t.Fatal(err)
		}
		workspace, slug := prepareS4Workspace(t, "S5 inference runtime")
		if code, _, stderr, _ := runPrepare(t, "--path", workspace, "prepare", slug, "--json", "--quiet"); code != 0 {
			t.Fatalf("prepare = %d: %s", code, stderr)
		}
		code, stdout, stderr, _ := runPrepare(t, "--path", workspace, "prepare", slug, "--check", "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("prepare --check = %d stderr=%q\n%s", code, stderr, stdout)
		}
		for _, artifact := range reportArtifacts(t, decodeReport(t, stdout)) {
			if artifact["provenance"] != "unknown" {
				t.Fatalf("runtime inferred provenance from durable metadata: %v", artifact)
			}
		}

		mutated := make(map[string][]byte, len(sources))
		for name, source := range sources {
			mutated[name] = append([]byte(nil), source...)
		}
		mutated["internal/intent/inspect.go"] = append(mutated["internal/intent/inspect.go"],
			[]byte("\nfunc forbiddenInference(status struct{ Notes string }) { _ = status.Notes }\n")...)
		if err := validatePrepareS5ProvenanceInferenceSources(mutated); err == nil {
			t.Fatal("status.notes provenance sensitivity escaped the production guard")
		}
		mutated["internal/intent/inspect.go"] = sources["internal/intent/inspect.go"]
		mutated["internal/store/intent_archive.go"] = append(mutated["internal/store/intent_archive.go"],
			[]byte("\ntype forbiddenArchiveAuthorship struct { Author string `json:\"author\"` }\n")...)
		if err := validatePrepareS5ProvenanceInferenceSources(mutated); err == nil {
			t.Fatal("archive authorship sensitivity escaped the production guard")
		}
		mutated["internal/store/intent_archive.go"] = sources["internal/store/intent_archive.go"]
		mutated["internal/intent/inspect.go"] = append(mutated["internal/intent/inspect.go"],
			[]byte("\nfunc forbiddenArchiveRead() { _ = IntentArchiveIndex{} }\n")...)
		if err := validatePrepareS5ProvenanceInferenceSources(mutated); err == nil {
			t.Fatal("intent-archive read sensitivity escaped the production guard")
		}
		renamed := strings.ReplaceAll(
			string(contract), "`intent-archive/**`", "`intent-archive/*`",
		)
		if err := validatePrepareS5ArchiveProvenanceContract(renamed); err == nil {
			t.Fatal("renamed forbidden-inference row escaped the contract guard")
		}
	})

	t.Run("PIB-144", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 generator schema")
		_, mutating, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		_, checking, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if !strings.Contains(mutating, `"generator":`) {
			t.Fatalf("mutating report lost generator field:\n%s", mutating)
		}
		if strings.Contains(checking, `"generator":`) {
			t.Fatalf("accepted check schema gained generator field:\n%s", checking)
		}
		fields, err := prepareS5GeneratorJSONFields(avpRepoRoot(t))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"internal/cli/prepare_publish.go:prepareArtifactReport.Generator"}
		if fmt.Sprint(fields) != fmt.Sprint(want) {
			t.Fatalf("generator-class JSON fields = %v, want mutating-report-only %v", fields, want)
		}
		fixture := map[string][]byte{
			"internal/cli/prepare_publish.go": []byte("package cli; type prepareArtifactReport struct { Generator string `json:\"generator\"` }"),
			"internal/store/persisted.go":     []byte("package store; type persisted struct { Generator string `json:\"generator\"` }"),
		}
		if got, err := prepareS5GeneratorJSONFieldsFromSources(fixture); err != nil || len(got) != 2 {
			t.Fatalf("generator persistence sensitivity did not trip: fields=%v err=%v", got, err)
		}
	})

	t.Run("PIB-145", func(t *testing.T) {
		claims, err := prepareS5WritePrecedentClaims(avpRepoRoot(t))
		if err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5WritePrecedentClaims(claims); err != nil {
			t.Fatal(err)
		}
		sourceMutation := clonePrepareS5Sources(claims)
		sourceMutation["internal/cli/prepare_publish.go"] = append(
			sourceMutation["internal/cli/prepare_publish.go"],
			[]byte("\n// ADR-034 governs persistence for prepare writes.\n")...,
		)
		if err := validatePrepareS5WritePrecedentClaims(sourceMutation); err == nil {
			t.Fatal("source-only ADR-034 persistence sensitivity escaped the guard")
		}
		docsMutation := clonePrepareS5Sources(claims)
		docsMutation["docs/prds/PRD-prepare-intent-bundle.md"] = append(
			docsMutation["docs/prds/PRD-prepare-intent-bundle.md"],
			[]byte("\nADR-034 governs every write in prepare.\n")...,
		)
		if err := validatePrepareS5WritePrecedentClaims(docsMutation); err == nil {
			t.Fatal("docs-only ADR-034 write sensitivity escaped the guard")
		}
		missingBoundary := clonePrepareS5Sources(claims)
		missingBoundary["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			missingBoundary["docs/prds/PRD-prepare-intent-bundle.md"],
			[]byte("Writes are a **new** surface and ADR-035 D2 governs them:"),
			[]byte("Writes are a new surface."),
			1,
		)
		if err := validatePrepareS5WritePrecedentClaims(missingBoundary); err == nil {
			t.Fatal("missing ADR-035 write-boundary claim escaped the guard")
		}
		movedPRDClaim := clonePrepareS5Sources(claims)
		requiredPRDClaim := []byte("Writes are a **new** surface and ADR-035 D2 governs them:")
		movedPRDClaim["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			movedPRDClaim["docs/prds/PRD-prepare-intent-bundle.md"],
			requiredPRDClaim,
			[]byte("Writes are a new surface governed by this section:"),
			1,
		)
		movedPRDClaim["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			movedPRDClaim["docs/prds/PRD-prepare-intent-bundle.md"],
			[]byte("## Revision history\n"),
			append([]byte("## Revision history\n\n"), append(requiredPRDClaim, '\n')...),
			1,
		)
		if err := validatePrepareS5WritePrecedentClaims(movedPRDClaim); err == nil {
			t.Fatal("PRD revision-history copy satisfied the normative write-boundary guard")
		}
		requiredPRDHeading := []byte(
			"### 13.2 Writes: rooted primitives, final-leaf refusal, and disclosed in-root redirects",
		)
		historicalBlock := append(append(append([]byte(nil), requiredPRDHeading...), '\n'), requiredPRDClaim...)
		historicalBlock = append(historicalBlock, []byte("\n\n### 13.3 Historical copied boundary\n")...)
		historicalCopy := clonePrepareS5Sources(claims)
		historicalCopy["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			historicalCopy["docs/prds/PRD-prepare-intent-bundle.md"],
			requiredPRDClaim,
			[]byte("Writes are a new surface governed by this section:"),
			1,
		)
		historicalCopy["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			historicalCopy["docs/prds/PRD-prepare-intent-bundle.md"],
			[]byte("## Revision history\n"),
			append([]byte("## Revision history\n\n"), historicalBlock...),
			1,
		)
		if err := validatePrepareS5WritePrecedentClaims(historicalCopy); err == nil {
			t.Fatal("complete revision-history heading/claim copy satisfied the normative path")
		}

		fencedCopy := clonePrepareS5Sources(claims)
		fencedCopy["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			fencedCopy["docs/prds/PRD-prepare-intent-bundle.md"],
			requiredPRDClaim,
			[]byte("Writes are a new surface governed by this section:"),
			1,
		)
		fence := append([]byte("```markdown\n"), historicalBlock...)
		fence = append(fence, []byte("```\n\n")...)
		fencedCopy["docs/prds/PRD-prepare-intent-bundle.md"] = append(
			fence, fencedCopy["docs/prds/PRD-prepare-intent-bundle.md"]...,
		)
		if err := validatePrepareS5WritePrecedentClaims(fencedCopy); err == nil {
			t.Fatal("fenced heading/claim copy satisfied the normative path")
		}

		duplicateHeading := clonePrepareS5Sources(claims)
		duplicate := append(append(append([]byte(nil), requiredPRDHeading...), '\n'), requiredPRDClaim...)
		duplicate = append(duplicate, append([]byte("\n\n"), requiredPRDHeading...)...)
		duplicateHeading["docs/prds/PRD-prepare-intent-bundle.md"] = bytes.Replace(
			duplicateHeading["docs/prds/PRD-prepare-intent-bundle.md"],
			requiredPRDHeading,
			duplicate,
			1,
		)
		if err := validatePrepareS5WritePrecedentClaims(duplicateHeading); err == nil {
			t.Fatal("duplicate normative PRD heading escaped ambiguity rejection")
		}

		movedADRClaim := clonePrepareS5Sources(claims)
		requiredADRClaim := []byte(
			"Every canonical read keeps ADR-034's accepted boundary. Every mutating write —\n" +
				"transactional publication, archive/index publication, the one-file `--manual`\n" +
				"status transition, and the best-effort derived-index refresh — uses a held\n" +
				"workspace `*os.Root` and a closed root-relative target list.",
		)
		movedADRClaim["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"] = bytes.Replace(
			movedADRClaim["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"],
			requiredADRClaim,
			[]byte("This decision defines the current rooted write boundary."),
			1,
		)
		movedADRClaim["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"] = bytes.Replace(
			movedADRClaim["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"],
			[]byte("## Alternatives considered\n"),
			append([]byte("## Alternatives considered\n\n"), append(requiredADRClaim, '\n')...),
			1,
		)
		if err := validatePrepareS5WritePrecedentClaims(movedADRClaim); err == nil {
			t.Fatal("ADR rejected-alternative copy satisfied the normative D2 guard")
		}
	})

	t.Run("PIB-146", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		prepareRoot, prepareSlug := prepareS4Workspace(t, "S5 heuristic parity")
		phaseRoot, phaseSlug := prepareS4Workspace(t, "S5 heuristic parity")
		if prepareSlug != phaseSlug {
			t.Fatalf("fixture slugs differ: %q %q", prepareSlug, phaseSlug)
		}
		if code, _, stderr, _ := runPrepare(t, "--path", prepareRoot, "prepare", prepareSlug, "--json", "--quiet"); code != 0 {
			t.Fatalf("prepare = %d: %s", code, stderr)
		}
		if code, _, stderr, _ := runPrepare(t, "--path", phaseRoot, "analyze", phaseSlug); code != 0 {
			t.Fatalf("analyze = %d: %s", code, stderr)
		}
		for _, rel := range []string{"analysis.md", "artifacts/analysis.json"} {
			fromPrepare, err := os.ReadFile(filepath.Join(prepareRoot, ".tpatch", "features", prepareSlug, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			fromAnalyze, err := os.ReadFile(filepath.Join(phaseRoot, ".tpatch", "features", phaseSlug, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(fromPrepare, fromAnalyze) {
				t.Fatalf("%s differs between prepare heuristic and analyze", rel)
			}
		}
		sidecar, err := os.ReadFile(filepath.Join(prepareRoot, ".tpatch", "features", prepareSlug, "artifacts", "analysis.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(sidecar, []byte(`"heuristic_mode": true`)) {
			t.Fatalf("heuristic sidecar lost mode flag: %s", sidecar)
		}
	})

	t.Run("PIB-147", func(t *testing.T) {
		index, err := store.NewIntentArchiveIndex("schema-boundary")
		if err != nil {
			t.Fatal(err)
		}
		wire, err := store.EncodeIntentArchiveIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			`"author"`, `"agent"`, `"model"`, `"provider"`,
			`"endpoint"`, `"path_a"`, `"path_b"`, `"generator"`,
		} {
			if bytes.Contains(bytes.ToLower(wire), []byte(forbidden)) {
				t.Fatalf("archive schema carries forbidden field %s: %s", forbidden, wire)
			}
		}
		injected := bytes.Replace(wire, []byte(`"feature": "schema-boundary"`),
			[]byte(`"feature": "schema-boundary", "author": "fixture"`), 1)
		if _, err := store.DecodeIntentArchiveIndex(injected, "schema-boundary"); err == nil {
			t.Fatal("archive decoder accepted a provenance sensitivity field")
		}
	})

	t.Run("PIB-378", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 no authorship evidence")
		_, mutating, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		_, checking, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		doctor := buildRootCmd()
		doctorCommand, _, err := doctor.Find([]string{"doctor"})
		if err != nil {
			t.Fatal(err)
		}
		texts := []string{mutating, checking, doctorCommand.Long}
		for _, rel := range []string{"SPEC.md", "docs/feature-layout.md", "docs/agent-as-provider.md"} {
			body, err := os.ReadFile(filepath.Join(avpRepoRoot(t), filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			texts = append(texts, string(body))
		}
		if err := validatePrepareS5NoAuthorshipEvidence(texts); err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5NoAuthorshipEvidence([]string{"The intent archive records the author of each artifact."}); err == nil {
			t.Fatal("archive-authorship sensitivity claim escaped the guard")
		}
		if err := validatePrepareS5NoAuthorshipEvidence([]string{"status.json.notes identifies who authored the artifact."}); err == nil {
			t.Fatal("notes-authorship sensitivity claim escaped the guard")
		}
	})
}

func TestPrepareS5MarkdownHeadingParserSensitivity(t *testing.T) {
	path := []string{"Root", "Parent", "Target"}
	tests := []struct {
		name    string
		source  string
		want    string
		wantErr bool
	}{
		{
			name: "same-marker-trailing-text-is-not-a-closer",
			source: "# Root\n## Parent\n````markdown\n" +
				"```` trailing text\n### Target ###\nfake\n````\n" +
				"### Target ###\nreal\n## End\n",
			want: "real",
		},
		{
			name:    "four-space-indented-heading-is-code",
			source:  "# Root\n## Parent\n    ### Target ###\nfake\n## End\n",
			wantErr: true,
		},
		{
			name:   "three-space-heading-is-valid",
			source: "# Root\n## Parent\n   ### Target ###\nreal\n## End\n",
			want:   "real",
		},
		{
			name: "shorter-and-different-fences-do-not-close",
			source: "# Root\n## Parent\n````markdown\n```\n~~~~\n" +
				"### Target ###\nfake\n````\n### Target ###\nreal\n## End\n",
			want: "real",
		},
		{
			name: "indented-backtick-fence-hides-headings",
			source: "# Root\n## Parent\n   ```markdown\n### Target ###\nfake\n   ```\n" +
				"### Target ###\nreal\n## End\n",
			want: "real",
		},
		{
			name: "indented-tilde-fence-hides-headings",
			source: "# Root\n## Parent\n  ~~~~markdown\n### Target ###\nfake\n ~~~~   \n" +
				"### Target ###\nreal\n## End\n",
			want: "real",
		},
		{
			name: "historical-copy-has-the-wrong-parent",
			source: "# Root\n## Revision history\n### Target ###\nhistorical\n" +
				"## Parent\n### Target ###\nnormative\n## End\n",
			want: "normative",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := prepareS5NormativeSection(test.source, path)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parser accepted false heading section %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(got) != test.want {
				t.Fatalf("section = %q, want %q", strings.TrimSpace(got), test.want)
			}
		})
	}
}

func TestPrepareS5LateCrashAndLifecycleRows(t *testing.T) {
	t.Run("PIB-319", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("workspace authority is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 blob reuse")
		prior := []byte("preexisting orphan bytes\n")
		hash := sha256Hex(prior)
		blobPath := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs", hash+".blob")
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
		defer authority.Release()
		blobRel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		result, err := newPrepareArchiveStorage(authority, nil).PublishBlob(blobRel, hash, prior)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Reused || result.Committed {
			t.Fatalf("blob reuse result = %#v", result)
		}
		after, err := os.Stat(blobPath)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
			t.Fatal("existing blob was rewritten rather than reused")
		}
		source, err := os.ReadFile(filepath.Join(avpRepoRoot(t), "internal", "cli", "prepare_publish.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(source, []byte(`"archive-blob-reused"`)) {
			t.Fatal("prepare no longer reports the blob-reused advisory")
		}
	})

	t.Run("PIB-324", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 external postcondition")
		if code, _, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet"); code != 0 {
			t.Fatalf("prepare = %d: %s", code, stderr)
		}
		specPath := filepath.Join(root, ".tpatch", "features", slug, "spec.md")
		if err := os.WriteFile(specPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("check after external emptying = %d\n%s", code, stdout)
		}
		if got := artifactRow(t, decodeReport(t, stdout), "spec")["state"]; got != "present-empty" {
			t.Fatalf("external bytes reported as %v, want present-empty", got)
		}
	})

	t.Run("PIB-325", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 concurrent cycle")
		specPath := filepath.Join(root, ".tpatch", "features", slug, "spec.md")
		oldHook := beforePrepareSetRevalidation
		t.Cleanup(func() { beforePrepareSetRevalidation = oldHook })
		var cycleBytes []byte
		beforePrepareSetRevalidation = func() {
			code, _, stderr, _ := runPrepare(t, "--path", root, "cycle", slug, "--skip-execute")
			if code != 0 {
				t.Fatalf("concurrent cycle = %d: %s", code, stderr)
			}
			var err error
			cycleBytes, err = os.ReadFile(specPath)
			if err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 5 || report.Refusal == nil || report.Refusal.Code != "entry-changed" {
			t.Fatalf("prepare/cycle CAS = %d %#v", code, report)
		}
		got, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(cycleBytes) == 0 || !bytes.Equal(got, cycleBytes) {
			t.Fatal("concurrent cycle bytes did not survive prepare CAS refusal")
		}
	})

	t.Run("PIB-379", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S5 notes semantics")
		if code, _, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet"); code != 0 {
			t.Fatalf("prepare = %d: %s", code, stderr)
		}
		if code, _, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet"); code != 0 {
			t.Fatalf("regenerate = %d: %s", code, stderr)
		}
		s := &store.Store{Root: root}
		status, err := s.LoadFeatureStatus(slug)
		if err != nil {
			t.Fatal(err)
		}
		regenerateNote := status.Notes
		if !strings.Contains(regenerateNote, "regenerated") {
			t.Fatalf("regenerate note = %q", regenerateNote)
		}
		status.State = store.StateAnalyzed
		if err := s.SaveFeatureStatus(status); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr, _ := runPrepare(t, "--path", root, "define", slug, "--manual"); code != 0 {
			t.Fatalf("later define = %d: %s", code, stderr)
		}
		later, err := s.LoadFeatureStatus(slug)
		if err != nil {
			t.Fatal(err)
		}
		if later.Notes == regenerateNote || strings.Contains(strings.ToLower(later.Notes), "regenerated") ||
			later.LastCommand != "define" {
			t.Fatalf("notes were treated as history: before=%q after=%#v", regenerateNote, later)
		}
	})
}

func TestPrepareS5DoctorConcurrencyRows(t *testing.T) {
	t.Run("PIB-380", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("workspace authority is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 live prepare doctor")
		entered := make(chan struct{})
		release := make(chan struct{})
		oldHook := beforePrepareSetRevalidation
		beforePrepareSetRevalidation = func() {
			close(entered)
			<-release
		}
		t.Cleanup(func() { beforePrepareSetRevalidation = oldHook })
		type result struct {
			code   int
			stdout string
			stderr string
		}
		done := make(chan result, 1)
		go func() {
			code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
			done <- result{code: code, stdout: stdout, stderr: stderr}
		}()
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
			t.Fatal("prepare did not reach the held-authority publication window")
		}
		released := false
		defer func() {
			if !released {
				close(release)
			}
		}()

		before := snapshotTreeMetadata(t, "workspace", root)
		report, err := workflow.RunDoctor(&store.Store{Root: root}, workflow.DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		after := snapshotTreeMetadata(t, "workspace", root)
		if after != before {
			t.Fatalf("doctor mutated a live prepare workspace\n--- before ---\n%s--- after ---\n%s", before, after)
		}
		rendered, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"authority held", "authority is free", "holder identity", "process id"} {
			if strings.Contains(strings.ToLower(string(rendered)), forbidden) {
				t.Fatalf("doctor made live-authority claim %q: %s", forbidden, rendered)
			}
		}

		close(release)
		released = true
		select {
		case got := <-done:
			if got.code != 0 || got.stderr != "" {
				t.Fatalf("live prepare was perturbed by doctor: code=%d stderr=%q\n%s", got.code, got.stderr, got.stdout)
			}
			published := prepareS4Report(t, got.stdout)
			if published.Outcome != "published" || published.Action != "complete" {
				t.Fatalf("live prepare outcome changed by doctor: %#v", published)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("live prepare did not complete after doctor")
		}
	})
}

func TestPrepareS5CheckWindowRows(t *testing.T) {
	t.Run("PIB-206", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("workspace authority is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 live check window")
		specPath := filepath.Join(root, ".tpatch", "features", slug, "spec.md")
		entered := make(chan struct{})
		release := make(chan struct{})
		oldHook := beforePrepareSetRevalidation
		oldAcquire := prepareAcquireAuthority
		acquires := 0
		prepareAcquireAuthority = func(repoRoot string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return oldAcquire(repoRoot)
		}
		beforePrepareSetRevalidation = func() {
			close(entered)
			<-release
		}
		t.Cleanup(func() {
			beforePrepareSetRevalidation = oldHook
			prepareAcquireAuthority = oldAcquire
		})
		type result struct {
			code   int
			stdout string
			stderr string
		}
		done := make(chan result, 1)
		go func() {
			code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
			done <- result{code: code, stdout: stdout, stderr: stderr}
		}()
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
			t.Fatal("prepare did not reach the deterministic publication window")
		}
		released := false
		defer func() {
			if !released {
				close(release)
			}
		}()
		if acquires != 1 {
			t.Fatalf("mutator authority acquisitions before check = %d, want 1", acquires)
		}
		if err := os.WriteFile(specPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshotTreeMetadata(t, "workspace", root)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--json", "--quiet",
		)
		after := snapshotTreeMetadata(t, "workspace", root)
		state, _ := artifactRow(t, decodeReport(t, stdout), "spec")["state"].(string)
		if err := validatePrepareS5CheckWindowObservation(code, state, before, after, acquires, stdout+stderr); err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5CheckWindowObservation(code, "absent", before, after, acquires, stdout+stderr); err == nil {
			t.Fatal("live-check structural-truth sensitivity accepted a proxy state")
		}
		if err := validatePrepareS5CheckWindowObservation(code, state, before, after, acquires+1, stdout+stderr); err == nil {
			t.Fatal("live-check authority sensitivity accepted a second acquisition")
		}

		close(release)
		released = true
		select {
		case got := <-done:
			report := prepareS4Report(t, got.stdout)
			if got.code != 5 || report.Refusal == nil || report.Refusal.Code != "entry-changed" {
				t.Fatalf("mutator did not preserve the external check-window truth: code=%d stderr=%q report=%#v", got.code, got.stderr, report)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("mutating prepare did not complete after the check")
		}
	})
}

func validatePrepareS5CheckWindowObservation(code int, state, before, after string, acquisitions int, output string) error {
	if code != 2 {
		return fmt.Errorf("live prepare --check exit = %d, want structural-not-ready 2", code)
	}
	if state != "present-empty" {
		return fmt.Errorf("live prepare --check spec state = %q, want current present-empty truth", state)
	}
	if before != after {
		return fmt.Errorf("live prepare --check mutated the workspace")
	}
	if acquisitions != 1 {
		return fmt.Errorf("live prepare --check acquired workspace authority: total acquisitions=%d", acquisitions)
	}
	lower := strings.ToLower(output)
	for _, forbidden := range []string{"transaction-in-progress", "recovered", "recovery"} {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("live prepare --check entered lock/recovery path %q", forbidden)
		}
	}
	return nil
}

func TestPrepareS5CycleCompatibilityRows(t *testing.T) {
	t.Run("PIB-209", func(t *testing.T) {
		binary := buildCurrentBinary(t)
		env := hermeticRoutingEnv(t)
		root := t.TempDir()
		runRoutingBinary(t, binary, root, env, "init")
		runRoutingBinary(t, binary, root, env, "add", "cycle demo")
		gotTranscript := runRoutingBinary(t, binary, root, env, "cycle", "cycle-demo", "--skip-execute")
		gotState := readStateGolden(t, root, "cycle-demo")
		wantTranscript, err := os.ReadFile(filepath.Join(routingGoldenDir, "cycle-skip-execute-transcript.txt"))
		if err != nil {
			t.Fatal(err)
		}
		wantState, err := os.ReadFile(filepath.Join(routingGoldenDir, "cycle-final-state.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5CycleCompatibility(
			gotTranscript, gotState, string(wantTranscript), string(wantState),
		); err != nil {
			t.Fatal(err)
		}
		if err := validatePrepareS5CycleCompatibility(
			gotTranscript+"proxy-output", gotState, string(wantTranscript), string(wantState),
		); err == nil {
			t.Fatal("cycle transcript proxy escaped the live compatibility guard")
		}
		if err := validatePrepareS5CycleCompatibility(
			gotTranscript, gotState+"proxy-state", string(wantTranscript), string(wantState),
		); err == nil {
			t.Fatal("cycle final-state proxy escaped the live compatibility guard")
		}
	})
}

func validatePrepareS5CycleCompatibility(gotTranscript, gotState, wantTranscript, wantState string) error {
	if gotTranscript != wantTranscript {
		return fmt.Errorf("cycle --skip-execute stdout/stderr/exit drifted from the frozen pre-change fixture")
	}
	if gotState != wantState {
		return fmt.Errorf("cycle --skip-execute final state drifted from the frozen pre-change fixture")
	}
	return nil
}

func TestPrepareS5NonInvalidationSourceRows(t *testing.T) {
	t.Run("PIB-213", func(t *testing.T) {
		root := avpRepoRoot(t)
		importers := []string{}
		publishCallers := []string{}
		err := filepath.WalkDir(filepath.Join(root, "internal"), func(filePath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			source, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, filePath)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if bytes.Contains(source, []byte(`"github.com/tesseracode/tesserapatch/internal/intentpub"`)) {
				importers = append(importers, rel)
			}
			file, err := parser.ParseFile(token.NewFileSet(), filePath, source, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, ok := call.Fun.(*ast.Ident)
				if ok && ident.Name == "runPreparePublish" {
					publishCallers = append(publishCallers, rel)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(importers)
		sort.Strings(publishCallers)
		if strings.Join(importers, ",") != strings.Join([]string{
			"internal/cli/feature_intent_archive.go",
			"internal/cli/prepare_publish.go",
		}, ",") {
			t.Fatalf("intentpub production importers = %v", importers)
		}
		if strings.Join(publishCallers, ",") != "internal/cli/prepare.go" {
			t.Fatalf("runPreparePublish callers = %v", publishCallers)
		}
		featureArchiveSource, err := os.ReadFile(filepath.Join(root, "internal", "cli", "feature_intent_archive.go"))
		if err != nil {
			t.Fatal(err)
		}
		featureArchiveAST, err := parser.ParseFile(token.NewFileSet(), "feature_intent_archive.go", featureArchiveSource, 0)
		if err != nil {
			t.Fatal(err)
		}
		allowedJournalSymbols := map[string]bool{"Journal": true, "JournalRel": true, "DecodeJournal": true}
		ast.Inspect(featureArchiveAST, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if ok && ident.Name == "intentpub" && !allowedJournalSymbols[selector.Sel.Name] {
				t.Errorf("feature intent-archive reaches intentpub publication symbol %s", selector.Sel.Name)
			}
			return true
		})
		phase2, err := os.ReadFile(filepath.Join(root, "internal", "cli", "phase2.go"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(phase2, []byte("runPreparePublish")) ||
			bytes.Contains(phase2, []byte(`OnComplete: "prepare"`)) {
			t.Fatal("prepare became a cycle/next completion step")
		}
	})

	t.Run("PIB-214", func(t *testing.T) {
		baseline := prepareS5StoreSourcesAtRevision(t, "54c227f")
		current := prepareS5CurrentStoreSources(t)
		want, err := prepareS5StoreFunctionSet(baseline)
		if err != nil {
			t.Fatal(err)
		}
		got, err := prepareS5StoreFunctionSet(current)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("internal/store function surface changed\nwant: %v\n got: %v", want, got)
		}
		sensitivity := clonePrepareS5Sources(current)
		sensitivity["internal/store/new_writer.go"] = []byte(
			"package store\nfunc NewUnexpectedWriter() error { return nil }\n",
		)
		changed, err := prepareS5StoreFunctionSet(sensitivity)
		if err != nil {
			t.Fatal(err)
		}
		if reflect.DeepEqual(changed, want) {
			t.Fatal("a synthetic store writer did not change the guarded function surface")
		}
	})
}

func prepareS5StoreSourcesAtRevision(t *testing.T, revision string) map[string][]byte {
	t.Helper()
	root := avpRepoRoot(t)
	command := exec.Command("git", "ls-tree", "-r", "--name-only", revision, "--", "internal/store")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list baseline store files: %v", err)
	}
	sources := map[string][]byte{}
	for _, rel := range strings.Fields(string(output)) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		show := exec.Command("git", "show", revision+":"+rel)
		show.Dir = root
		data, err := show.Output()
		if err != nil {
			t.Fatalf("read %s at %s: %v", rel, revision, err)
		}
		sources[rel] = data
	}
	return sources
}

func prepareS5CurrentStoreSources(t *testing.T) map[string][]byte {
	t.Helper()
	root := avpRepoRoot(t)
	command := exec.Command("git", "ls-files", "--cached", "--", "internal/store")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list current store files: %v", err)
	}
	sources := map[string][]byte{}
	for _, rel := range strings.Fields(string(output)) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read current %s: %v", rel, err)
		}
		sources[rel] = data
	}
	return sources
}

func prepareS5StoreFunctionSet(sources map[string][]byte) ([]string, error) {
	set := map[string]bool{}
	for rel, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), rel, source, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := function.Name.Name
			if function.Recv != nil && len(function.Recv.List) == 1 {
				name = prepareS5ReceiverName(function.Recv.List[0].Type) + "." + name
			}
			set[name] = true
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func prepareS5ReceiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return "*" + prepareS5ReceiverName(value.X)
	case *ast.IndexExpr:
		return prepareS5ReceiverName(value.X)
	case *ast.IndexListExpr:
		return prepareS5ReceiverName(value.X)
	default:
		return fmt.Sprintf("%T", expression)
	}
}

func TestPrepareS5PlatformRows(t *testing.T) {
	t.Run("PIB-221", func(t *testing.T) {
		vet := exec.Command("go", "vet", "./cmd/tpatch", "./internal/cli", "./internal/workflow")
		vet.Dir = avpRepoRoot(t)
		if output, err := vet.CombinedOutput(); err != nil {
			t.Fatalf("go vet: %v\n%s", err, output)
		}
		for _, target := range []struct {
			goos   string
			goarch string
		}{
			{goos: "linux", goarch: "amd64"},
			{goos: "linux", goarch: "arm64"},
			{goos: "darwin", goarch: "arm64"},
			{goos: "windows", goarch: "amd64"},
		} {
			target := target
			name := target.goos + "-" + target.goarch
			t.Run(name, func(t *testing.T) {
				outputPath := filepath.Join(t.TempDir(), "tpatch")
				if target.goos == "windows" {
					outputPath += ".exe"
				}
				command := exec.Command("go", "build", "-o", outputPath, "./cmd/tpatch")
				command.Dir = avpRepoRoot(t)
				command.Env = append(os.Environ(),
					"GOOS="+target.goos,
					"GOARCH="+target.goarch,
					"CGO_ENABLED=0",
				)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("GOOS=%s GOARCH=%s go build: %v\n%s", target.goos, target.goarch, err, output)
				}
			})
		}

	})

	t.Run("PIB-222", func(t *testing.T) {
		root := avpRepoRoot(t)
		publish, err := os.ReadFile(filepath.Join(root, "internal", "cli", "prepare_publish.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, function := range []string{"func runPreparePublish(", "func runPrepareAbandon("} {
			start := bytes.Index(publish, []byte(function))
			if start < 0 {
				t.Fatalf("missing %s", function)
			}
			rest := publish[start:]
			next := bytes.Index(rest[len(function):], []byte("\nfunc "))
			if next >= 0 {
				rest = rest[:len(function)+next]
			}
			unsupported := bytes.Index(rest, []byte("!intentlock.AuthoritySupported"))
			acquire := bytes.Index(rest, []byte("prepareAcquireAuthority(repoRoot)"))
			if unsupported < 0 || acquire < 0 || unsupported > acquire {
				t.Fatalf("%s does not refuse unsupported mutation before authority acquisition", function)
			}
		}
		checkSource, err := os.ReadFile(filepath.Join(root, "internal", "cli", "prepare.go"))
		if err != nil {
			t.Fatal(err)
		}
		checkDispatch := bytes.Index(checkSource, []byte("return runPrepareCheck(cmd, args[0])"))
		mutatingDispatch := bytes.Index(checkSource, []byte("return runPreparePublish(cmd, args[0], options)"))
		if checkDispatch < 0 || mutatingDispatch < 0 || checkDispatch > mutatingDispatch {
			t.Fatal("accepted read-only --check no longer dispatches before mutating prepare")
		}
		windowsRuntime, err := os.ReadFile(filepath.Join(root, "internal", "cli", "prepare_pib_golden_windows_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(windowsRuntime, []byte(`t.Run("PIB-222"`)) ||
			!bytes.Contains(windowsRuntime, []byte(`report.Refusal.Code != "prepare-unsupported-platform"`)) ||
			!bytes.Contains(windowsRuntime, []byte("acquires != 0")) {
			t.Fatal("native Windows PIB-222 runtime proof lost its refusal/authority sensitivity")
		}
		ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(ci, []byte(`-run '^TestPreparePIBUnsupportedPlatformRuntimeGolden$'`)) ||
			!bytes.Contains(ci, []byte(`--- PASS: TestPreparePIBUnsupportedPlatformRuntimeGolden`)) {
			t.Fatal("the existing blocking Windows selector no longer executes the PIB-222 parent test")
		}
	})

	t.Run("PIB-223", func(t *testing.T) {
		for _, goos := range []string{"freebsd", "openbsd"} {
			command := exec.Command("go", "list", "-f", "{{join .GoFiles \"\\n\"}}", "./internal/intentlock")
			command.Dir = avpRepoRoot(t)
			command.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0")
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("GOOS=%s go list: %v\n%s", goos, err, output)
			}
			files := string(output)
			if !strings.Contains(files, "acquire_unsupported.go") ||
				strings.Contains(files, "acquire_supported.go") {
				t.Fatalf("GOOS=%s intentlock files = %q", goos, files)
			}
		}
		unsupported, err := os.ReadFile(filepath.Join(avpRepoRoot(t), "internal", "intentlock", "acquire_unsupported.go"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(unsupported, []byte("os.OpenRoot")) ||
			!bytes.Contains(unsupported, []byte("CodePrepareUnsupportedPlatform")) {
			t.Fatal("unsupported mutation path can open the workspace or lost its fixed refusal")
		}
	})
}

func prepareS5GeneratorJSONFields(root string) ([]string, error) {
	sources := map[string][]byte{}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		source, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		sources[filepath.ToSlash(rel)] = source
		return nil
	})
	if err != nil {
		return nil, err
	}
	return prepareS5GeneratorJSONFieldsFromSources(sources)
}

func prepareS5GeneratorJSONFieldsFromSources(sources map[string][]byte) ([]string, error) {
	var fields []string
	for fileName, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), fileName, source, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, spec := range generic.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structure.Fields.List {
					if field.Tag == nil || len(field.Names) == 0 {
						continue
					}
					tagText, err := strconv.Unquote(field.Tag.Value)
					if err != nil {
						return nil, err
					}
					jsonName := strings.Split(reflect.StructTag(tagText).Get("json"), ",")[0]
					if jsonName != "generator" {
						continue
					}
					for _, name := range field.Names {
						fields = append(fields, fileName+":"+typeSpec.Name.Name+"."+name.Name)
					}
				}
			}
		}
	}
	sort.Strings(fields)
	return fields, nil
}

func validatePrepareS5NoAuthorshipEvidence(texts []string) error {
	for _, text := range texts {
		for _, sentence := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return r == '\n' || r == '!' || r == '?'
		}) {
			evidence := strings.Contains(sentence, "intent archive") ||
				strings.Contains(sentence, "intent-archive") ||
				strings.Contains(sentence, "status.json.notes")
			authorship := strings.Contains(sentence, "author") ||
				strings.Contains(sentence, "who created") ||
				strings.Contains(sentence, "provenance")
			negated := strings.Contains(sentence, " not ") ||
				strings.Contains(sentence, "never") ||
				strings.Contains(sentence, "none ") ||
				strings.Contains(sentence, "no ")
			if evidence && authorship && !negated {
				return fmt.Errorf("shipped text presents durable state as authorship evidence: %q", strings.TrimSpace(sentence))
			}
		}
	}
	return nil
}

func TestPrepareS5PersistentEvidenceRuntimeRows(t *testing.T) {
	t.Run("PIB-316", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("mutating prepare is unsupported on this platform")
		}
		for _, jsonMode := range []bool{true, false} {
			result := prepareS5RollbackWithTwoArchiveBlobs(t, jsonMode)
			if err := validatePrepareS5RollbackOrphanClaim(result.stdout); err != nil {
				t.Fatal(err)
			}
			if err := validatePrepareS5RollbackOrphanClaim(
				result.stdout + "\nThe working tree is byte-identical.\n",
			); err == nil {
				t.Fatal("byte-identical rollback sensitivity escaped the report guard")
			}
			command := "tpatch feature intent-archive purge " + result.slug + " --orphans --yes"
			if jsonMode {
				report := prepareS4Report(t, result.stdout)
				if report.Outcome != "rolled-back" ||
					!reflect.DeepEqual(report.OrphanBlobs, result.hashes) {
					t.Fatalf("JSON orphan hashes = %v, want actual files %v", report.OrphanBlobs, result.hashes)
				}
				matches := 0
				wantMessage := fmt.Sprintf(
					"%d orphan archive blob(s) remain; remove them with %s.",
					len(result.hashes), command,
				)
				for _, advisory := range report.Advisories {
					if advisory.Message == wantMessage {
						matches++
					}
				}
				if matches != 1 {
					t.Fatalf("JSON purge route count = %d, want 1: %#v", matches, report.Advisories)
				}
				continue
			}
			if strings.Count(result.stdout, "Remove them with: "+command) != 1 {
				t.Fatalf("human purge route is not exact-once:\n%s", result.stdout)
			}
			start := strings.Index(result.stdout, "Orphan archive blobs:\n")
			end := strings.Index(result.stdout, "Remove them with: "+command)
			if start < 0 || end <= start {
				t.Fatalf("human orphan section is missing:\n%s", result.stdout)
			}
			orphanSection := result.stdout[start:end]
			for _, hash := range result.hashes {
				if strings.Count(orphanSection, hash) != 1 {
					t.Fatalf("human orphan-list count for actual blob %s != 1:\n%s", hash, orphanSection)
				}
			}
		}
	})

	t.Run("PIB-317", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("mutating prepare is unsupported on this platform")
		}
		result := prepareS5RollbackWithTwoArchiveBlobs(t, true)
		report := prepareS4Report(t, result.stdout)
		if len(result.hashes) != 2 || len(report.OrphanBlobs) != len(result.hashes) {
			t.Fatalf("reported/file orphan counts = %d/%d, want 2/2", len(report.OrphanBlobs), len(result.hashes))
		}
		for _, hash := range result.hashes {
			if countPrepareS5String(report.OrphanBlobs, hash) != 1 {
				t.Fatalf("actual orphan %s is not listed exactly once: %v", hash, report.OrphanBlobs)
			}
		}
	})

	t.Run("PIB-318", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("mutating prepare is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 manual status rename crash")
		prepareS4WriteReadyBundle(t, root, slug, false)
		statusPath := filepath.Join(root, ".tpatch", "features", slug, "status.json")
		oldStatus, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}

		state := &prepareS5StatusPostRenameFault{}
		oldFactory := prepareIntentpubRootOps
		prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
			return &prepareS5StatusPostRenameFaultOps{
				RootOps: intentpub.NewRootOps(rooted),
				state:   state,
			}
		}
		t.Cleanup(func() { prepareIntentpubRootOps = oldFactory })
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		prepareIntentpubRootOps = oldFactory
		if !state.renamed || !state.syncFault || code != 6 {
			t.Fatalf(
				"manual post-rename crash = renamed:%v sync-fault:%v code:%d\n%s",
				state.renamed, state.syncFault, code, stdout,
			)
		}
		failed := prepareS4Report(t, stdout)
		if failed.Outcome != "recovery-refused" || failed.Refusal == nil ||
			failed.Refusal.Code != "post-publication-divergence" {
			t.Fatalf("manual post-rename report = %#v", failed)
		}
		crashedStatus, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var afterCrash store.FeatureStatus
		if err := json.Unmarshal(crashedStatus, &afterCrash); err != nil {
			t.Fatalf("status was partial after rename crash: %v\n%s", err, crashedStatus)
		}
		if bytes.Equal(crashedStatus, oldStatus) || afterCrash.State != store.StateDefined {
			t.Fatalf("status after crash is neither old nor new: %#v", afterCrash)
		}
		journal := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "journal.json")
		if _, err := os.Stat(journal); !os.IsNotExist(err) {
			t.Fatalf("status-only crash created a journal: %v", err)
		}

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("manual retry = %d stderr=%q\n%s", code, stderr, stdout)
		}
		retry := prepareS4Report(t, stdout)
		finalStatus, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var final store.FeatureStatus
		if err := json.Unmarshal(finalStatus, &final); err != nil || final.State != store.StateDefined {
			t.Fatalf("retry status = %#v err=%v\n%s", final, err, finalStatus)
		}
		if retry.Outcome != "published" && retry.Outcome != "no-op" {
			t.Fatalf("manual retry outcome = %#v", retry)
		}
		if _, err := os.Stat(journal); !os.IsNotExist(err) {
			t.Fatalf("manual retry created a journal: %v", err)
		}
	})

	t.Run("PIB-320", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("mutating prepare is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 preserve abandoned evidence")
		if code, _, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		); code != 0 {
			t.Fatalf("initial prepare = %d: %s", code, stderr)
		}
		if code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		); code != 0 {
			t.Fatalf("archive prepare = %d stderr=%q\n%s", code, stderr, stdout)
		}
		lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
		existing := filepath.Join(lane, "abandoned-111111111111")
		if err := os.MkdirAll(existing, 0o700); err != nil {
			t.Fatal(err)
		}
		marker := []byte("existing evidence\n")
		if err := os.WriteFile(filepath.Join(existing, "marker"), marker, 0o600); err != nil {
			t.Fatal(err)
		}
		existingInfo, err := os.Stat(existing)
		if err != nil {
			t.Fatal(err)
		}

		prepareS5InterruptAfterJournal(t, root, slug, "--regenerate", "--allow-heuristic")
		for _, name := range []string{
			"journal.json", "index.preimage.json", "status.preimage.json",
		} {
			if _, err := os.Stat(filepath.Join(lane, name)); err != nil {
				t.Fatalf("interrupted regenerate lacks %s: %v", name, err)
			}
		}
		extraStage := filepath.Join(lane, "stage-aaaaaaaaaaaa")
		if err := os.MkdirAll(extraStage, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extraStage, "extra"), []byte("owned stage\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stages, err := filepath.Glob(filepath.Join(lane, "stage-*"))
		if err != nil || len(stages) < 2 {
			t.Fatalf("interrupted regenerate stages = %v err=%v, want at least two", stages, err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		recovered := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || recovered.Outcome != "recovered" {
			t.Fatalf("recovery = %d stderr=%q %#v", code, stderr, recovered)
		}
		for _, name := range []string{
			"journal.json", "journal.clearing.json",
			"index.preimage.json", "status.preimage.json",
		} {
			if _, err := os.Stat(filepath.Join(lane, name)); !os.IsNotExist(err) {
				t.Fatalf("recovery retained owned control evidence %s: %v", name, err)
			}
		}
		if stages, err := filepath.Glob(filepath.Join(lane, "stage-*")); err != nil || len(stages) != 0 {
			t.Fatalf("recovery retained owned stages = %v err=%v", stages, err)
		}
		afterRecoveryInfo, err := os.Stat(existing)
		if err != nil || !os.SameFile(existingInfo, afterRecoveryInfo) {
			t.Fatalf("recovery replaced the prior abandoned directory: %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(existing, "marker")); err != nil || !bytes.Equal(got, marker) {
			t.Fatalf("recovery changed existing abandoned evidence: %v %q", err, got)
		}
		if entries, err := os.ReadDir(existing); err != nil || len(entries) != 1 ||
			entries[0].Name() != "marker" {
			t.Fatalf("recovery merged evidence into prior abandoned directory: %v err=%v", entries, err)
		}

		prepareS5InterruptAfterJournal(t, root, slug, "--regenerate", "--allow-heuristic")
		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		abandoned := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || abandoned.Outcome != "abandoned" ||
			abandoned.Abandoned == nil {
			t.Fatalf("abandon = %d stderr=%q %#v", code, stderr, abandoned)
		}
		directories, err := filepath.Glob(filepath.Join(lane, "abandoned-*"))
		if err != nil || len(directories) != 2 {
			t.Fatalf("abandoned directories = %v err=%v, want two", directories, err)
		}
		sort.Strings(directories)
		newDirectory := filepath.Join(root, filepath.FromSlash(
			strings.TrimSuffix(abandoned.Abandoned.Directory, "/"),
		))
		if newDirectory == existing || !prepareS5ContainsPath(directories, existing) ||
			!prepareS5ContainsPath(directories, newDirectory) {
			t.Fatalf("abandoned evidence merged or replaced: dirs=%v new=%s", directories, newDirectory)
		}
		if got, err := os.ReadFile(filepath.Join(existing, "marker")); err != nil || !bytes.Equal(got, marker) {
			t.Fatalf("real abandon changed old directory: %v %q", err, got)
		}
		if entries, err := os.ReadDir(existing); err != nil || len(entries) != 1 ||
			entries[0].Name() != "marker" {
			t.Fatalf("real abandon merged evidence into prior directory: %v err=%v", entries, err)
		}
		if _, err := os.Stat(filepath.Join(newDirectory, "journal.json")); err != nil {
			t.Fatalf("new abandoned directory did not receive journal evidence: %v", err)
		}
		for _, name := range []string{"index.preimage.json", "status.preimage.json"} {
			if _, err := os.Stat(filepath.Join(newDirectory, name)); err != nil {
				t.Fatalf("new abandoned directory did not receive %s: %v", name, err)
			}
		}
		if stages, err := filepath.Glob(filepath.Join(newDirectory, "stage-*")); err != nil || len(stages) == 0 {
			t.Fatalf("new abandoned directory did not receive owned stages: %v err=%v", stages, err)
		}
		if _, err := os.Stat(filepath.Join(newDirectory, "marker")); !os.IsNotExist(err) {
			t.Fatalf("new abandoned directory merged old marker evidence: %v", err)
		}
	})

	t.Run("PIB-321", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("mutating prepare is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 git clean journal boundary")
		prepareS5InterruptAfterJournal(t, root, slug)
		laneRel := ".tpatch/local/intent-prepare/" + slug
		prepareS5Git(t, root, "clean", "-fdx", "--", laneRel)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(laneRel))); !os.IsNotExist(err) {
			t.Fatalf("git clean did not remove runtime lane: %v", err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "published" ||
			report.Recovery != nil || report.Refusal != nil {
			t.Fatalf("ordinary preflight after git clean = %d stderr=%q %#v", code, stderr, report)
		}
		assertPrepareS5NoJournalLossClaim(t, stdout)
		doctorCode, doctorOut, doctorErr, _ := runPrepare(
			t, "--path", root, "doctor", "--check", "D9", "--json",
		)
		if doctorCode != 0 || doctorErr != "" {
			t.Fatalf("doctor after git clean = %d stderr=%q\n%s", doctorCode, doctorErr, doctorOut)
		}
		assertPrepareS5NoJournalLossClaim(t, doctorOut)
	})

	t.Run("PIB-322", func(t *testing.T) {
		if !intentlock.AuthoritySupported {
			t.Skip("mutating prepare is unsupported on this platform")
		}
		root, slug := prepareS4Workspace(t, "S5 fresh clone archive")
		if code, _, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		); code != 0 {
			t.Fatalf("initial prepare = %d: %s", code, stderr)
		}
		if code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		); code != 0 {
			t.Fatalf("archive prepare = %d stderr=%q\n%s", code, stderr, stdout)
		}
		beforeInterrupted := prepareS5ArchiveBlobHashes(t, root, slug)
		feature := filepath.Join(root, ".tpatch", "features", slug)
		first := []byte("interrupted committed archive markdown\n")
		second := []byte("{\"interrupted\":\"committed archive\"}\n")
		for rel, content := range map[string][]byte{
			"analysis.md":             first,
			"spec.md":                 first,
			"exploration.md":          second,
			"artifacts/analysis.json": second,
		} {
			if err := os.WriteFile(filepath.Join(feature, filepath.FromSlash(rel)), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		prepareS5InterruptAfterAllRenames(t, root, slug, "--regenerate", "--allow-heuristic")
		afterInterrupted := prepareS5ArchiveBlobHashes(t, root, slug)
		interruptedHashes := prepareS5StringDifference(afterInterrupted, beforeInterrupted)
		if len(interruptedHashes) != 2 {
			t.Fatalf(
				"interrupted regenerate published hashes = %v; before=%v after=%v",
				interruptedHashes, beforeInterrupted, afterInterrupted,
			)
		}
		prepareS5Git(t, root, "add", "--", ".gitignore", ".tpatch")
		prepareS5Git(t, root, "commit", "-q", "-m", "fixture archive state")

		clone := filepath.Join(t.TempDir(), "clone")
		prepareS5Git(t, "", "clone", "-q", "--no-local", root, clone)
		journal := filepath.Join(clone, ".tpatch", "local", "intent-prepare", slug, "journal.json")
		if _, err := os.Stat(journal); !os.IsNotExist(err) {
			t.Fatalf("fresh clone contains prepare journal: %v", err)
		}
		blobs, err := filepath.Glob(filepath.Join(
			clone, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs", "*.blob",
		))
		if err != nil || len(blobs) == 0 {
			t.Fatalf("fresh clone lost committed archive blobs: %v err=%v", blobs, err)
		}
		for _, hash := range interruptedHashes {
			if _, err := os.Stat(filepath.Join(
				clone, ".tpatch", "features", slug, "artifacts",
				"intent-archive", "blobs", hash+".blob",
			)); err != nil {
				t.Fatalf("fresh clone lost interrupted archive blob %s: %v", hash, err)
			}
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", clone, "prepare", slug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "no-op" ||
			report.Recovery != nil || report.Refusal != nil {
			t.Fatalf("fresh-clone preflight = %d stderr=%q %#v", code, stderr, report)
		}
		assertPrepareS5NoJournalLossClaim(t, stdout)
		doctorCode, doctorOut, doctorErr, _ := runPrepare(
			t, "--path", clone, "doctor", "--check", "D9", "--json",
		)
		if doctorCode != 0 || doctorErr != "" {
			t.Fatalf("fresh-clone doctor = %d stderr=%q\n%s", doctorCode, doctorErr, doctorOut)
		}
		assertPrepareS5NoJournalLossClaim(t, doctorOut)
		if strings.Contains(strings.ToLower(doctorOut), "prepare-transaction-pending") ||
			strings.Contains(strings.ToLower(doctorOut), "recovery-pending") {
			t.Fatalf("fresh-clone doctor invented journal/recovery state:\n%s", doctorOut)
		}
		for _, hash := range interruptedHashes {
			if strings.Contains(doctorOut, hash) {
				t.Fatalf("doctor misclassified referenced archive hash %s as residue:\n%s", hash, doctorOut)
			}
		}
	})
}

func validatePrepareS5RollbackOrphanClaim(report string) error {
	lower := strings.ToLower(report)
	for _, forbidden := range []string{
		"tree is byte-identical",
		"tree remains byte-identical",
		"working tree is byte-identical",
		"working tree remains byte-identical",
		"working tree was left byte-identical",
	} {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("orphan-bearing rollback overclaims byte identity with %q", forbidden)
		}
	}
	return nil
}

type prepareS5RollbackResult struct {
	slug   string
	hashes []string
	stdout string
}

func prepareS5RollbackWithTwoArchiveBlobs(t *testing.T, jsonMode bool) prepareS5RollbackResult {
	t.Helper()
	root, slug := prepareS4Workspace(t, "S5 two archive orphans")
	if code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	); code != 0 {
		t.Fatalf("initial prepare = %d: %s", code, stderr)
	}
	feature := filepath.Join(root, ".tpatch", "features", slug)
	first := []byte("shared prior markdown\n")
	second := []byte("{\"shared\":\"prior bytes\"}\n")
	for rel, content := range map[string][]byte{
		"analysis.md":             first,
		"spec.md":                 first,
		"exploration.md":          second,
		"artifacts/analysis.json": second,
	} {
		if err := os.WriteFile(filepath.Join(feature, filepath.FromSlash(rel)), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	oldHook := prepareIntentpubHook
	changedStage := false
	prepareIntentpubHook = func(
		point intentpub.CrashPoint,
		rooted *os.Root,
		entry *intentpub.Entry,
	) error {
		if point != intentpub.PointBeforeEntryCAS || entry == nil || changedStage {
			return nil
		}
		changedStage = true
		file, err := rooted.OpenFile(entry.StagedRel, os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if _, err := file.Write([]byte("changed staged bytes\n")); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	}
	args := []string{"--path", root, "prepare", slug, "--regenerate", "--allow-heuristic"}
	if jsonMode {
		args = append(args, "--json", "--quiet")
	}
	code, stdout, stderr, _ := runPrepare(t, args...)
	prepareIntentpubHook = oldHook
	if !changedStage || code != 5 || !strings.Contains(stderr, "refused entry-changed") {
		t.Fatalf("rollback = %d stderr=%q\n%s", code, stderr, stdout)
	}
	for rel, want := range map[string][]byte{
		"analysis.md":             first,
		"spec.md":                 first,
		"exploration.md":          second,
		"artifacts/analysis.json": second,
	} {
		got, err := os.ReadFile(filepath.Join(feature, filepath.FromSlash(rel)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("rollback did not restore %s: err=%v got=%q want=%q", rel, err, got, want)
		}
	}
	journal := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "journal.json")
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatalf("successful rollback retained a journal: %v", err)
	}
	blobDir := filepath.Join(feature, "artifacts", "intent-archive", "blobs")
	entries, err := os.ReadDir(blobDir)
	if err != nil {
		t.Fatal(err)
	}
	var hashes []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".blob") {
			hashes = append(hashes, strings.TrimSuffix(entry.Name(), ".blob"))
		}
	}
	sort.Strings(hashes)
	want := []string{sha256Hex(first), sha256Hex(second)}
	sort.Strings(want)
	if !reflect.DeepEqual(hashes, want) {
		t.Fatalf("actual archive blob hashes = %v, want exactly two published blobs %v", hashes, want)
	}
	return prepareS5RollbackResult{slug: slug, hashes: hashes, stdout: stdout}
}

type prepareS5StatusPostRenameFault struct {
	renamed   bool
	syncFault bool
}

type prepareS5StatusPostRenameFaultOps struct {
	intentpub.RootOps
	state *prepareS5StatusPostRenameFault
}

func (o *prepareS5StatusPostRenameFaultOps) Rename(oldName, newName string) error {
	if err := o.RootOps.Rename(oldName, newName); err != nil {
		return err
	}
	if strings.HasSuffix(filepath.ToSlash(newName), "/status.json") {
		o.state.renamed = true
	}
	return nil
}

func (o *prepareS5StatusPostRenameFaultOps) Open(name string) (intentpub.RootFile, error) {
	file, err := o.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	return &prepareS5StatusDirectorySyncFaultFile{
		RootFile: file,
		state:    o.state,
	}, nil
}

type prepareS5StatusDirectorySyncFaultFile struct {
	intentpub.RootFile
	state *prepareS5StatusPostRenameFault
}

func (f *prepareS5StatusDirectorySyncFaultFile) Sync() error {
	if f.state.renamed && !f.state.syncFault {
		f.state.syncFault = true
		return errors.New("injected post-rename directory sync failure")
	}
	return f.RootFile.Sync()
}

func prepareS5InterruptAfterJournal(t *testing.T, root, slug string, extra ...string) {
	t.Helper()
	oldHook := prepareIntentpubHook
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
		if point == intentpub.PointAfterJournalDurable {
			return errors.New("injected interruption after durable journal")
		}
		return nil
	}
	args := []string{"--path", root, "prepare", slug}
	args = append(args, extra...)
	args = append(args, "--json", "--quiet")
	code, stdout, _, _ := runPrepare(t, args...)
	prepareIntentpubHook = oldHook
	report := prepareS4Report(t, stdout)
	if code != 6 || report.Outcome != "recovery-refused" {
		t.Fatalf("journal interruption = %d %#v", code, report)
	}
	journal := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "journal.json")
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("durable journal is missing after interruption: %v", err)
	}
}

func prepareS5InterruptAfterAllRenames(t *testing.T, root, slug string, extra ...string) {
	t.Helper()
	oldHook := prepareIntentpubHook
	fired := false
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
		if point == intentpub.PointAfterAllRenames {
			fired = true
			return errors.New("injected interruption after all publication renames")
		}
		return nil
	}
	args := []string{"--path", root, "prepare", slug}
	args = append(args, extra...)
	args = append(args, "--json", "--quiet")
	code, stdout, _, _ := runPrepare(t, args...)
	prepareIntentpubHook = oldHook
	report := prepareS4Report(t, stdout)
	if !fired || code != 6 || report.Outcome != "recovery-refused" {
		t.Fatalf("post-rename interruption = fired:%v code:%d report:%#v", fired, code, report)
	}
	journal := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "journal.json")
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("post-rename interruption lost its durable journal: %v", err)
	}
}

func prepareS5Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	if dir != "" {
		command.Dir = dir
	}
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "missing-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(t.TempDir(), "missing-system"),
		"GIT_AUTHOR_NAME=S5 Fixture",
		"GIT_AUTHOR_EMAIL=s5@example.invalid",
		"GIT_COMMITTER_NAME=S5 Fixture",
		"GIT_COMMITTER_EMAIL=s5@example.invalid",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func countPrepareS5String(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func prepareS5ArchiveBlobHashes(t *testing.T, root, slug string) []string {
	t.Helper()
	directory := filepath.Join(
		root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs",
	)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var hashes []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".blob") {
			hashes = append(hashes, strings.TrimSuffix(entry.Name(), ".blob"))
		}
	}
	sort.Strings(hashes)
	return hashes
}

func prepareS5StringDifference(all, excluded []string) []string {
	excludedSet := make(map[string]bool, len(excluded))
	for _, value := range excluded {
		excludedSet[value] = true
	}
	var difference []string
	for _, value := range all {
		if !excludedSet[value] {
			difference = append(difference, value)
		}
	}
	return difference
}

func prepareS5ContainsPath(paths []string, want string) bool {
	for _, path := range paths {
		if filepath.Clean(path) == filepath.Clean(want) {
			return true
		}
	}
	return false
}

func assertPrepareS5NoJournalLossClaim(t *testing.T, output string) {
	t.Helper()
	lower := strings.ToLower(output)
	for _, forbidden := range []string{
		"removed prepare journal",
		"lost prepare journal",
		"journal loss",
		"journal-loss",
		"journal was removed",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("ordinary preflight invented journal-loss claim %q:\n%s", forbidden, output)
		}
	}
}

func prepareS5ProvenanceInferenceSources(root string) (map[string][]byte, error) {
	sources := map[string][]byte{}
	for _, rel := range []string{
		"internal/intent/inspect.go",
		"internal/intent/render.go",
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
		"internal/workflow/doctor_d9.go",
		"internal/store/intent_archive.go",
	} {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		sources[rel] = source
	}
	return sources, nil
}

func validatePrepareS5ProvenanceInferenceSources(sources map[string][]byte) error {
	inferenceFiles := map[string]bool{
		"internal/intent/inspect.go": true,
		"internal/intent/render.go":  true,
		"internal/cli/prepare.go":    true,
	}
	persistenceFiles := map[string]bool{
		"internal/cli/prepare_publish.go":  true,
		"internal/store/intent_archive.go": true,
	}
	forbiddenSelectors := map[string]bool{
		"Notes": true, "LastCommand": true, "UpdatedAt": true, "RequestedAt": true,
	}
	forbiddenWireKeys := map[string]bool{
		"author": true, "authored_by": true, "provenance": true, "path_a": true, "path_b": true,
	}
	unknownConstant := false
	provenanceAssignments := 0
	for fileName, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), fileName, source, 0)
		if err != nil {
			return err
		}
		if inferenceFiles[fileName] {
			ast.Inspect(file, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.SelectorExpr:
					if forbiddenSelectors[value.Sel.Name] || strings.HasPrefix(value.Sel.Name, "IntentArchive") {
						err = fmt.Errorf("%s references forbidden provenance source %s", fileName, value.Sel.Name)
						return false
					}
				case *ast.Ident:
					if strings.HasPrefix(value.Name, "IntentArchive") {
						err = fmt.Errorf("%s references intent archive metadata", fileName)
						return false
					}
				case *ast.BasicLit:
					if value.Kind == token.STRING {
						literal, unquoteErr := strconv.Unquote(value.Value)
						if unquoteErr == nil && strings.Contains(strings.ToLower(literal), "intent-archive") {
							err = fmt.Errorf("%s names intent-archive as an inference source", fileName)
							return false
						}
					}
				case *ast.KeyValueExpr:
					key, ok := value.Key.(*ast.Ident)
					if !ok || key.Name != "Provenance" {
						break
					}
					provenanceAssignments++
					ident, ok := value.Value.(*ast.Ident)
					if !ok || ident.Name != "ProvenanceUnknown" {
						err = fmt.Errorf("%s assigns provenance from %T instead of ProvenanceUnknown", fileName, value.Value)
						return false
					}
				}
				return err == nil
			})
			if err != nil {
				return err
			}
		}
		if fileName == "internal/intent/inspect.go" {
			for _, declaration := range file.Decls {
				generic, ok := declaration.(*ast.GenDecl)
				if !ok || generic.Tok != token.CONST {
					continue
				}
				for _, spec := range generic.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for index, name := range valueSpec.Names {
						if name.Name != "ProvenanceUnknown" || index >= len(valueSpec.Values) {
							continue
						}
						if value, ok := prepareS5StringLiteral(valueSpec.Values[index]); ok && value == "unknown" {
							unknownConstant = true
						}
					}
				}
			}
		}
		if persistenceFiles[fileName] {
			ast.Inspect(file, func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				if !ok || field.Tag == nil {
					return true
				}
				tagText, unquoteErr := strconv.Unquote(field.Tag.Value)
				if unquoteErr != nil {
					err = unquoteErr
					return false
				}
				jsonName := strings.Split(reflect.StructTag(tagText).Get("json"), ",")[0]
				if forbiddenWireKeys[jsonName] {
					err = fmt.Errorf("%s persists forbidden provenance key %q", fileName, jsonName)
					return false
				}
				return true
			})
			if err != nil {
				return err
			}
		}
		if fileName == "internal/workflow/doctor_d9.go" {
			lower := strings.ToLower(string(source))
			for _, forbidden := range []string{".notes", "lastcommand", "authored by", "author identity"} {
				if strings.Contains(lower, forbidden) {
					return fmt.Errorf("D9 source references forbidden inference token %q", forbidden)
				}
			}
		}
	}
	if !unknownConstant || provenanceAssignments == 0 {
		return fmt.Errorf("unknown-only provenance sink is not mechanically bound: constant=%v assignments=%d", unknownConstant, provenanceAssignments)
	}
	render := string(sources["internal/intent/render.go"])
	if !strings.Contains(render, `"provenance: unknown (all artifacts)"`) {
		return fmt.Errorf("human renderer lost the unknown-only provenance line")
	}
	return nil
}

func validatePrepareS5ArchiveProvenanceContract(source string) error {
	start := strings.Index(source, "### 11.2 Forbidden inference sources — explicit")
	end := strings.Index(source, "### 11.3 Alternatives for a future durable representation")
	if start < 0 || end <= start {
		return errors.New("AVP forbidden-inference section is missing")
	}
	rows := 0
	for _, line := range strings.Split(source[start:end], "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || strings.TrimSpace(cells[1]) != "`intent-archive/**`" {
			continue
		}
		rows++
		why := strings.Join(strings.Fields(strings.TrimSpace(cells[2])), " ")
		if why != "Replaced-byte/recovery data, not authorship/provenance; provenance must never be inferred from it (ADR-035 D9)." {
			return fmt.Errorf("intent-archive forbidden-inference rationale = %q", why)
		}
	}
	if rows != 1 {
		return fmt.Errorf("intent-archive forbidden-inference rows = %d, want 1", rows)
	}
	return nil
}

func prepareS5WritePrecedentClaims(root string) (map[string][]byte, error) {
	paths := []string{
		"docs/prds/PRD-prepare-intent-bundle.md",
		"docs/prds/PRD-artifact-validation-and-provenance.md",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
		"internal/cli/feature_intent_archive.go",
		"internal/cli/doctor.go",
		"internal/workflow/doctor_d9.go",
		"internal/store/intent_archive.go",
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "intentpub"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") &&
			!strings.HasSuffix(entry.Name(), "_test.go") {
			paths = append(paths, "internal/intentpub/"+entry.Name())
		}
	}
	sort.Strings(paths)
	claims := make(map[string][]byte, len(paths))
	for _, rel := range paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		claims[rel] = body
	}
	return claims, nil
}

func validatePrepareS5WritePrecedentClaims(claims map[string][]byte) error {
	preparePRD := string(claims["docs/prds/PRD-prepare-intent-bundle.md"])
	archiveADR := string(claims["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"])
	prdArchiveBoundary, err := prepareS5NormativeSection(
		preparePRD,
		[]string{
			"PRD — Prepare Intent Bundle — `tpatch prepare <slug>` (mutating modes)",
			"8. Non-destructive overwrite — the architecture gate",
		},
	)
	if err != nil {
		return err
	}
	prdWriteBoundary, err := prepareS5NormativeSection(
		preparePRD,
		[]string{
			"PRD — Prepare Intent Bundle — `tpatch prepare <slug>` (mutating modes)",
			"13. Security, privacy and determinism",
			"13.2 Writes: rooted primitives, final-leaf refusal, and disclosed in-root redirects",
		},
	)
	if err != nil {
		return err
	}
	adrWriteBoundary, err := prepareS5NormativeSection(
		archiveADR,
		[]string{
			"ADR-035 — Intent Bundle Publication and History",
			"Decisions",
			"D2 — Writes get their own boundary, and it is **rooted**",
		},
	)
	if err != nil {
		return err
	}
	for _, required := range []struct {
		name    string
		section string
		claim   string
	}{
		{
			name:    "PRD archive boundary",
			section: prdArchiveBoundary,
			claim:   "Every write path in this document is governed by ADR-035",
		},
		{
			name:    "PRD rooted write boundary",
			section: prdWriteBoundary,
			claim:   "Writes are a **new** surface and ADR-035 D2 governs them:",
		},
		{
			name:    "ADR-035 D2 write population",
			section: adrWriteBoundary,
			claim:   "Every canonical read keeps ADR-034's accepted boundary. Every mutating write —",
		},
		{
			name:    "ADR-035 D2 rooted authority",
			section: adrWriteBoundary,
			claim:   "workspace `*os.Root` and a closed root-relative target list.",
		},
	} {
		if !strings.Contains(required.section, required.claim) {
			return fmt.Errorf("%s lost normative claim %q", required.name, required.claim)
		}
	}

	forbidden := []string{
		"adr-034 governs persistence",
		"adr-034 governs every write",
		"adr-034 governs all writes",
		"adr-034 governs writes",
		"adr-034 governs provenance",
		"adr-034 is the persistence boundary",
		"adr-034 is the write boundary",
		"adr-034 provides persistence precedent",
		"adr-034 provides write precedent",
		"adr-034 provides provenance precedent",
		"adr-034 authorizes persistence",
		"adr-034 authorizes writes",
		"adr-034 authorizes provenance",
	}
	for rel, body := range claims {
		for number, line := range strings.Split(string(body), "\n") {
			normalized := strings.ToLower(strings.Join(strings.Fields(
				strings.NewReplacer("**", "", "`", "", "_", " ").Replace(line),
			), " "))
			for _, phrase := range forbidden {
				if strings.Contains(normalized, phrase) {
					return fmt.Errorf("%s:%d cites ADR-034 as write/persistence/provenance precedent", rel, number+1)
				}
			}
		}
	}
	return nil
}

type prepareS5MarkdownHeading struct {
	level        int
	text         string
	path         []string
	start        int
	contentStart int
}

func prepareS5NormativeSection(source string, path []string) (string, error) {
	headings := prepareS5MarkdownHeadings(source)
	var matches []int
	for index, heading := range headings {
		if reflect.DeepEqual(heading.path, path) {
			matches = append(matches, index)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("normative heading path %q matched %d times", strings.Join(path, " > "), len(matches))
	}
	selected := matches[0]
	end := len(source)
	for _, heading := range headings[selected+1:] {
		if heading.level <= headings[selected].level {
			end = heading.start
			break
		}
	}
	if end < headings[selected].contentStart {
		return "", fmt.Errorf("normative heading path %q has an invalid boundary", strings.Join(path, " > "))
	}
	return source[headings[selected].contentStart:end], nil
}

func prepareS5MarkdownHeadings(source string) []prepareS5MarkdownHeading {
	var headings []prepareS5MarkdownHeading
	var hierarchy []prepareS5MarkdownHeading
	offset := 0
	fenceByte := byte(0)
	fenceLength := 0
	for _, raw := range strings.SplitAfter(source, "\n") {
		line := strings.TrimSuffix(raw, "\n")
		marker, length, trailing, fence := prepareS5MarkdownFence(line)
		if fenceByte != 0 {
			if fence && marker == fenceByte && length >= fenceLength &&
				strings.Trim(trailing, " \t") == "" {
				fenceByte, fenceLength = 0, 0
			}
			offset += len(raw)
			continue
		}
		if fence && (marker != '`' || !strings.Contains(trailing, "`")) {
			fenceByte, fenceLength = marker, length
			offset += len(raw)
			continue
		}
		level, text, ok := prepareS5ATXHeading(line)
		if !ok {
			offset += len(raw)
			continue
		}
		for len(hierarchy) != 0 && hierarchy[len(hierarchy)-1].level >= level {
			hierarchy = hierarchy[:len(hierarchy)-1]
		}
		path := make([]string, 0, len(hierarchy)+1)
		for _, parent := range hierarchy {
			path = append(path, parent.text)
		}
		path = append(path, text)
		heading := prepareS5MarkdownHeading{
			level:        level,
			text:         text,
			path:         path,
			start:        offset,
			contentStart: offset + len(raw),
		}
		headings = append(headings, heading)
		hierarchy = append(hierarchy, heading)
		offset += len(raw)
	}
	return headings
}

func prepareS5ATXHeading(line string) (int, string, bool) {
	line = strings.TrimSuffix(line, "\r")
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 {
		return 0, "", false
	}
	trimmed := line[indent:]
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if level < len(trimmed) && trimmed[level] != ' ' && trimmed[level] != '\t' {
		return 0, "", false
	}
	content := strings.TrimRight(trimmed[level:], " \t")
	hashStart := len(content)
	for hashStart > 0 && content[hashStart-1] == '#' {
		hashStart--
	}
	if hashStart < len(content) && hashStart > 0 &&
		(content[hashStart-1] == ' ' || content[hashStart-1] == '\t') {
		content = strings.TrimRight(content[:hashStart], " \t")
	}
	text := strings.Join(strings.Fields(content), " ")
	return level, text, true
}

func prepareS5MarkdownFence(line string) (byte, int, string, bool) {
	line = strings.TrimSuffix(line, "\r")
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) ||
		line[indent] != '`' && line[indent] != '~' {
		return 0, 0, "", false
	}
	marker := line[indent]
	length := 0
	for indent+length < len(line) && line[indent+length] == marker {
		length++
	}
	if length < 3 {
		return 0, 0, "", false
	}
	return marker, length, line[indent+length:], true
}

func clonePrepareS5Sources(source map[string][]byte) map[string][]byte {
	clone := make(map[string][]byte, len(source))
	for name, body := range source {
		clone[name] = append([]byte(nil), body...)
	}
	return clone
}

func prepareS5StringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}
