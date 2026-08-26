//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/pflag"
	patchintent "github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7ARExitSixRouteGuard(t *testing.T) {
	t.Run("PIB-508", func(t *testing.T) {
		prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
		if err := validateS7ARExitSixRoutes(prd); err != nil {
			t.Fatal(err)
		}
		for _, fixture := range []struct {
			name  string
			scope string
			old   string
			new   string
		}{
			{
				name:  "archive-misrouted-to-abandon",
				scope: "| Exit-6 codes | The one route | Where it is defined |",
				old:   "| `archive-purge-evidence-divergent` | the repo-relative archive procedure:",
				new:   "| `archive-purge-evidence-divergent` | `tpatch prepare <slug> --abandon-transaction` and the repo-relative archive procedure:",
			},
			{
				name:  "journal-route-adds-archive-purge",
				scope: "| Exit-6 codes | The one route | Where it is defined |",
				old:   "the named repo-relative manual removal | §6.6 |",
				new:   "the named repo-relative manual removal plus `tpatch feature intent-archive purge <slug> --all --yes` | §6.6 |",
			},
			{
				name:  "journal-route-wrong-definition",
				scope: "| Exit-6 codes | The one route | Where it is defined |",
				old:   "the named repo-relative manual removal | §6.6 |",
				new:   "the named repo-relative manual removal | §9.7.2 |",
			},
			{
				name:  "archive-route-extra-definition",
				scope: "| Exit-6 codes | The one route | Where it is defined |",
				old:   "pending+absent recovery terminally tombstones | §9.7.2 |",
				new:   "pending+absent recovery terminally tombstones | §9.7.2 and §6.6 |",
			},
			{
				name:  "catalog-addition-without-route",
				scope: "### 10.4.1 Closed refusal catalog",
				old:   "| `undo-cas-mismatch`, `recovery-divergent`, `journal-corrupt`,",
				new:   "| `future-divergence`, `undo-cas-mismatch`, `recovery-divergent`, `journal-corrupt`,",
			},
			{
				name:  "catalog-removal-with-stale-route",
				scope: "### 10.4.1 Closed refusal catalog",
				old:   ", `journal-forged`, `post-publication-divergence`",
				new:   ", `post-publication-divergence`",
			},
			{
				name:  "route-removal-with-live-catalog",
				scope: "| Exit-6 codes | The one route | Where it is defined |",
				old:   ", `journal-forged`, `post-publication-divergence`, `workspace-root-replaced-after-publication` | `tpatch prepare",
				new:   ", `post-publication-divergence`, `workspace-root-replaced-after-publication` | `tpatch prepare",
			},
		} {
			mutated, changed := s7ARReplaceAfter(prd, fixture.scope, fixture.old, fixture.new)
			if !changed {
				t.Fatalf("PIB-508 %s sensitivity anchor missing", fixture.name)
			}
			if err := validateS7ARExitSixRoutes(mutated); err == nil {
				t.Fatalf("PIB-508 same validator accepted %s", fixture.name)
			}
		}
	})
}

func validateS7ARExitSixRoutes(prd string) error {
	catalog, err := s7ARExitSixCatalogFromPRD(prd)
	if err != nil {
		return err
	}
	section, err := s7ARSectionBetween(
		prd,
		"| Exit-6 codes | The one route | Where it is defined |",
		"\n\nEvery message in the first population",
	)
	if err != nil {
		return err
	}
	rows := s7ARMarkdownRows(section)
	if len(rows) != 2 {
		return fmt.Errorf("exit-6 route rows = %d, want 2", len(rows))
	}
	routed := map[string]int{}
	wantArchiveRoute := strings.Join(strings.Fields(
		"the repo-relative archive procedure: for the **blob** form, one type-total `rm -rf --` of the divergent managed `blobs/<hash>.blob` under the pending reference, printed under an explicit destructive warning and with no copy command promised; for the **index** form, restore an `index.json` that no longer decodes and remove nothing; then rerun the sanitized purge, whose pending+absent recovery terminally tombstones",
	), " ")
	wantJournalRoute := strings.Join(strings.Fields(
		"`tpatch prepare <slug> --abandon-transaction` — and, where the environment denies that mode, the named repo-relative manual removal",
	), " ")
	archiveCode := "archive-purge-evidence-divergent"
	for _, row := range rows {
		if len(row) != 3 {
			return fmt.Errorf("exit-6 route row has %d cells", len(row))
		}
		codes := regexp.MustCompile("`([a-z0-9-]+)`").FindAllStringSubmatch(row[0], -1)
		if len(codes) == 0 {
			return fmt.Errorf("exit-6 route row has no codes: %q", row[0])
		}
		for _, match := range codes {
			routed[match[1]]++
		}
		route := strings.Join(strings.Fields(row[1]), " ")
		definition := strings.Join(strings.Fields(row[2]), " ")
		if len(codes) == 1 && codes[0][1] == archiveCode {
			if route != wantArchiveRoute || definition != "§9.7.2" {
				return fmt.Errorf(
					"archive divergence route/definition = %q / %q, want %q / %q",
					route, definition, wantArchiveRoute, "§9.7.2",
				)
			}
		} else {
			wantCodes := make([]string, 0, len(catalog)-1)
			gotCodes := make([]string, 0, len(codes))
			for code := range catalog {
				if code != archiveCode {
					wantCodes = append(wantCodes, code)
				}
			}
			for _, match := range codes {
				gotCodes = append(gotCodes, match[1])
			}
			sort.Strings(wantCodes)
			sort.Strings(gotCodes)
			if !reflect.DeepEqual(gotCodes, wantCodes) ||
				route != wantJournalRoute || definition != "§6.6" {
				return fmt.Errorf(
					"journal/publication partition = codes:%v route:%q definition:%q, want %v / %q / %q",
					gotCodes, route, definition, wantCodes, wantJournalRoute, "§6.6",
				)
			}
		}
	}
	for code, count := range catalog {
		if count != 1 {
			return fmt.Errorf("normative exit-6 catalog code %s count = %d, want 1", code, count)
		}
		if routed[code] != 1 {
			return fmt.Errorf("normative exit-6 code %s route count = %d, want 1", code, routed[code])
		}
	}
	for code, count := range routed {
		if count != 1 || catalog[code] != 1 {
			return fmt.Errorf("route-only or duplicated exit-6 code %s count=%d catalog=%d",
				code, count, catalog[code])
		}
	}
	if len(catalog) != 10 || len(routed) != len(catalog) {
		return fmt.Errorf("exit-6 catalog/route sizes = %d/%d, want exact 10/10",
			len(catalog), len(routed))
	}
	return nil
}

func s7ARExitSixCatalogFromPRD(prd string) (map[string]int, error) {
	section, err := s7ARSectionBetween(
		prd,
		"### 10.4.1 Closed refusal catalog — code, exit, human and JSON shape",
		"\n\n`archive-index-changed` is publication/rehydration exit 5 only;",
	)
	if err != nil {
		return nil, err
	}
	catalog := map[string]int{}
	for _, row := range s7ARMarkdownRows(section) {
		if len(row) != 3 || row[0] == "Code(s)" || row[1] != "6" {
			continue
		}
		for _, match := range regexp.MustCompile("`([a-z0-9-]+)`").FindAllStringSubmatch(row[0], -1) {
			catalog[match[1]]++
		}
	}
	if len(catalog) == 0 {
		return nil, errors.New("normative §10.4.1 exit-6 catalog is empty")
	}
	return catalog, nil
}

func TestS7ARAbandonGateTableGuard(t *testing.T) {
	t.Run("PIB-511", func(t *testing.T) {
		prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
		sources := s7ARProductionSourceSet(t)
		programSources := s7ARPreAbandonProgramSources(
			s7ARCLIPackageSources(sources),
		)
		if len(programSources) != 2 ||
			programSources["internal/cli/prepare_publish.go"] == "" ||
			programSources["internal/cli/feature_intent_archive.go"] == "" {
			t.Fatalf(
				"PIB-511 pre-abandon program source boundary = %d/%t/%t",
				len(programSources),
				programSources["internal/cli/prepare_publish.go"] != "",
				programSources["internal/cli/feature_intent_archive.go"] != "",
			)
		}
		extendedProgramSources := s6CloneSourceSet(programSources)
		extendedProgramSources["internal/cli/zz_s7_ar_boundary.go"] =
			"package cli\n"
		extendedProgramSources = s7ARPreAbandonProgramSources(
			extendedProgramSources,
		)
		if len(extendedProgramSources) != 3 ||
			extendedProgramSources["internal/cli/zz_s7_ar_boundary.go"] == "" {
			t.Fatal("PIB-511 pre-abandon boundary dropped a synthetic fixture")
		}
		program, err := s7ARBuildPreAbandonProgram(s7ARCLIPackageSources(sources))
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7ARAbandonGateTableWithProgram(
			prd, sources, program,
		); err != nil {
			t.Fatal(err)
		}
		s7ARAssertAuthorityRefusalBehavior(t)
		passThrough := s6CloneSourceSet(sources)
		passThroughSource := strings.Replace(
			passThrough["internal/cli/prepare_publish.go"],
			"\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
			"\t\treturn emitPreparePublishReport(cmd, s7ARPassRefusedReport(refusePrepare(report, \"workspace-not-initialized\", \"\")), 3)",
			1,
		)
		passThroughSource = strings.Replace(
			passThroughSource,
			"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
			"func s7ARPassRefusedReport(report preparePublishReport) preparePublishReport {\n"+
				"\treturn report\n"+
				"}\n\n"+
				"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
			1,
		)
		if passThroughSource == passThrough["internal/cli/prepare_publish.go"] {
			t.Fatal("PIB-511 caller-refused pass-through sensitivity anchors missing")
		}
		passThrough["internal/cli/prepare_publish.go"] = passThroughSource
		passThroughProgram, err := s7ARBuildPreAbandonProgram(
			s7ARCLIPackageSources(passThrough),
		)
		if err != nil {
			t.Fatalf("PIB-511 caller-refused pass-through is not valid typed source: %v", err)
		}
		if err := validateS7ARAbandonGateTableWithProgram(
			prd, passThrough, passThroughProgram,
		); err != nil {
			t.Fatalf("PIB-511 rejected a fully refused caller parameter pass-through: %v", err)
		}
		irrelevant := s6CloneSourceSet(sources)
		irrelevantSource := strings.Replace(
			irrelevant["internal/cli/prepare_publish.go"],
			"\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
			"\t\treturn emitPreparePublishReport(cmd, s7ARSafeIrrelevantMutation(report, \"workspace-not-initialized\"), 3)",
			1,
		)
		irrelevantSource = strings.Replace(
			irrelevantSource,
			"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
			"func s7ARRewriteIrrelevant(value **string) { **value = \"changed\" }\n\n"+
				"func s7ARSafeIrrelevantMutation(report preparePublishReport, code string) preparePublishReport {\n"+
				"\tirrelevant := \"safe\"\n"+
				"\tpointer := &irrelevant\n"+
				"\twrapper := &pointer\n"+
				"\ts7ARRewriteIrrelevant(wrapper)\n"+
				"\treturn refusePrepare(report, code, \"\")\n"+
				"}\n\n"+
				"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
			1,
		)
		if irrelevantSource == irrelevant["internal/cli/prepare_publish.go"] {
			t.Fatal("PIB-511 irrelevant address-flow sensitivity anchors missing")
		}
		irrelevant["internal/cli/prepare_publish.go"] = irrelevantSource
		irrelevantProgram, err := s7ARBuildPreAbandonProgram(
			s7ARCLIPackageSources(irrelevant),
		)
		if err != nil {
			t.Fatalf("PIB-511 irrelevant address-flow fixture is not valid typed source: %v", err)
		}
		if err := validateS7ARAbandonGateTableWithProgram(
			prd, irrelevant, irrelevantProgram,
		); err != nil {
			t.Fatalf("PIB-511 definite irrelevant address flow created a false failure: %v", err)
		}
		for _, fixture := range []struct {
			name          string
			file          string
			old           string
			new           string
			extraOld      string
			extraNew      string
			wantErr       string
			authorityOnly bool
		}{
			{
				name: "helper-mediated-duplicate-existing-prebranch-refusal",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\tif options.mode == prepareModeAbandon && slug == rawSlug {\n" +
					"\t\treturn s7ARMutatedDuplicateWorkspaceRefusal(cmd, slug, options)\n" +
					"\t}\n\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARMutatedDuplicateWorkspaceRefusal(cmd *cobra.Command, slug string, options prepareOptions) error {\n" +
					"\treport := applyPrepareOptionFields(newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options)\n" +
					"\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)\n" +
					"}\n\nfunc runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name:     "mixed-refused-and-unchanged-helper-returns",
				file:     "internal/cli/prepare_publish.go",
				old:      "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new:      "\t\treturn emitPreparePublishReport(cmd, s7ARMixedRefusal(report), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARMixedRefusal(report preparePublishReport) preparePublishReport {\n" +
					"\tif report.Slug == \"\" {\n" +
					"\t\treturn refusePrepare(report, \"workspace-not-initialized\", \"\")\n" +
					"\t}\n" +
					"\treturn report\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "unresolved return alternative",
			},
			{
				name:     "parameter-rebound-before-return",
				file:     "internal/cli/prepare_publish.go",
				old:      "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new:      "\t\treturn emitPreparePublishReport(cmd, s7ARReboundRefusal(refusePrepare(report, \"workspace-not-initialized\", \"\")), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARReboundRefusal(report preparePublishReport) preparePublishReport {\n" +
					"\treport = refusePrepare(report, \"slug-unsafe\", \"\")\n" +
					"\treturn report\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name:     "refusal-code-parameter-rebound-before-return",
				file:     "internal/cli/prepare_publish.go",
				old:      "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new:      "\t\treturn emitPreparePublishReport(cmd, s7ARRecodeRefusal(report, \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARRecodeRefusal(report preparePublishReport, code string) preparePublishReport {\n" +
					"\tcode = \"slug-unsafe\"\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name: "refusal-code-pointer-helper-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, " +
					"s7ARPointerRecodeRefusal(report, \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARRewriteRefusalCode(code *string) {\n" +
					"\t*code = \"slug-unsafe\"\n" +
					"}\n\n" +
					"func s7ARPointerRecodeRefusal(report preparePublishReport, code string) preparePublishReport {\n" +
					"\ts7ARRewriteRefusalCode(&code)\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken refusal value",
			},
			{
				name: "refusal-code-pointer-alias-wrapper-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, " +
					"s7ARPointerAliasRecodeRefusal(report, \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARRewriteRefusalCodeAlias(code *string) {\n" +
					"\t*code = \"slug-unsafe\"\n" +
					"}\n\n" +
					"func s7ARRewriteRefusalCodeWrapper(code *string) {\n" +
					"\ts7ARRewriteRefusalCodeAlias(code)\n" +
					"}\n\n" +
					"func s7ARPointerAliasRecodeRefusal(report preparePublishReport, code string) preparePublishReport {\n" +
					"\talias := &code\n" +
					"\ts7ARRewriteRefusalCodeWrapper((alias))\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken refusal value",
			},
			{
				name: "refusal-code-invoked-closure-pointer-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, " +
					"s7ARClosurePointerRecodeRefusal(report, \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARClosurePointerRecodeRefusal(report preparePublishReport, code string) preparePublishReport {\n" +
					"\talias := &code\n" +
					"\t(func() { *alias = \"slug-unsafe\" })()\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken refusal value",
			},
			{
				name: "refused-report-pointer-helper-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treport = refusePrepare(report, \"workspace-not-initialized\", \"\")\n" +
					"\t\ts7ARRewriteRefusedReport(&report)\n" +
					"\t\treturn emitPreparePublishReport(cmd, report, 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARRewriteRefusedReport(report *preparePublishReport) {\n" +
					"\t*report = refusePrepare(*report, \"slug-unsafe\", \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken report value",
			},
			{
				name: "refusal-code-two-level-pointer-helper-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, " +
					"s7ARDoublePointerRecodeRefusal(report, \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARRewriteDoublePointerCode(code **string) {\n" +
					"\t**code = \"slug-unsafe\"\n" +
					"}\n\n" +
					"func s7ARDoublePointerRecodeRefusal(report preparePublishReport, code string) preparePublishReport {\n" +
					"\tpointer := &code\n" +
					"\twrapper := &pointer\n" +
					"\ts7ARRewriteDoublePointerCode(wrapper)\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken refusal value",
			},
			{
				name: "refused-report-two-level-pointer-helper-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treport = refusePrepare(report, \"workspace-not-initialized\", \"\")\n" +
					"\tpointer := &report\n" +
					"\twrapper := &pointer\n" +
					"\ts7ARRewriteDoublePointerReport(wrapper)\n" +
					"\treturn emitPreparePublishReport(cmd, report, 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARRewriteDoublePointerReport(report **preparePublishReport) {\n" +
					"\t**report = refusePrepare(**report, \"slug-unsafe\", \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken report value",
			},
			{
				name: "refusal-code-method-value-receiver-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, " +
					"s7ARMethodValueRecodeRefusal(report, \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "type s7ARCodeRewriter struct{ code *string }\n\n" +
					"func (rewriter s7ARCodeRewriter) rewrite() { *rewriter.code = \"slug-unsafe\" }\n\n" +
					"func s7ARMethodValueRecodeRefusal(report preparePublishReport, code string) preparePublishReport {\n" +
					"\trewriter := s7ARCodeRewriter{code: &code}\n" +
					"\trewrite := rewriter.rewrite\n" +
					"\trewrite()\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "address-taken refusal value",
			},
			{
				name: "refused-report-captured-callable-field-rebound",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treport = refusePrepare(report, \"workspace-not-initialized\", \"\")\n" +
					"\tpointer := &report\n" +
					"\tcallable := struct{ run func() }{run: func() {\n" +
					"\t\t*pointer = refusePrepare(*pointer, \"slug-unsafe\", \"\")\n" +
					"\t}}\n" +
					"\tcallable.run()\n" +
					"\treturn emitPreparePublishReport(cmd, report, 3)",
				wantErr: "address-taken report value",
			},
			{
				name:          "authority-evidence-predicate-removed",
				file:          "internal/cli/prepare_publish.go",
				old:           "\tif !prepareLaneHasPendingEvidence(repoRoot, slug) {\n\t\treturn report\n\t}",
				new:           "\tif false {\n\t\treturn report\n\t}",
				wantErr:       "authority refusal override",
				authorityOnly: true,
			},
			{
				name: "pending-evidence-predicate-polarity-reversed",
				file: "internal/cli/prepare_publish.go",
				old: "\tfor _, name := range names {\n" +
					"\t\tif preparePendingEvidenceName(name) {\n" +
					"\t\t\treturn true\n" +
					"\t\t}\n" +
					"\t}\n" +
					"\treturn false",
				new: "\tfor _, name := range names {\n" +
					"\t\tif !preparePendingEvidenceName(name) {\n" +
					"\t\t\treturn true\n" +
					"\t\t}\n" +
					"\t}\n" +
					"\treturn false",
				wantErr:       "pending-evidence helper",
				authorityOnly: true,
			},
			{
				name:          "pending-evidence-predicate-removed",
				file:          "internal/cli/prepare_publish.go",
				old:           "\t\tif preparePendingEvidenceName(name) {\n\t\t\treturn true\n\t\t}",
				new:           "\t\tif name != \"\" {\n\t\t\treturn true\n\t\t}",
				wantErr:       "pending-evidence helper",
				authorityOnly: true,
			},
			{
				name:          "pending-evidence-predicate-decoy-substitution",
				file:          "internal/cli/prepare_publish.go",
				old:           "\t\tif preparePendingEvidenceName(name) {\n\t\t\treturn true\n\t\t}",
				new:           "\t\tif prepareControlEvidenceName(name) {\n\t\t\treturn true\n\t\t}",
				wantErr:       "pending-evidence helper",
				authorityOnly: true,
			},
			{
				name:          "pending-name-control-only-reviewer-repro",
				file:          "internal/cli/prepare_publish.go",
				old:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new:           "return prepareControlEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name:          "pending-name-stage-only",
				file:          "internal/cli/prepare_publish.go",
				old:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new:           "return prepareStageEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name:          "pending-name-conjunction",
				file:          "internal/cli/prepare_publish.go",
				old:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new:           "return prepareControlEvidenceName(name) && prepareStageEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name:          "pending-name-stage-polarity-reversed",
				file:          "internal/cli/prepare_publish.go",
				old:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new:           "return prepareControlEvidenceName(name) || !prepareStageEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name:          "pending-name-stage-wrong-argument",
				file:          "internal/cli/prepare_publish.go",
				old:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(strings.TrimSpace(name))",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name: "pending-name-shadowed-stage-decoy",
				file: "internal/cli/prepare_publish.go",
				old:  "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new: "prepareStageEvidenceName := func(string) bool { return true }\n" +
					"\treturn prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name: "pending-name-decoy-call-and-control-only-return",
				file: "internal/cli/prepare_publish.go",
				old:  "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new: "_ = prepareStageEvidenceName(name)\n" +
					"\treturn prepareControlEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name:          "pending-name-duplicate-stage-call",
				file:          "internal/cli/prepare_publish.go",
				old:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)",
				new:           "return prepareControlEvidenceName(name) || prepareStageEvidenceName(name) || prepareStageEvidenceName(name)",
				wantErr:       "control-or-stage semantics",
				authorityOnly: true,
			},
			{
				name:          "pending-evidence-positive-match-returns-false",
				file:          "internal/cli/prepare_publish.go",
				old:           "\t\tif preparePendingEvidenceName(name) {\n\t\t\treturn true\n\t\t}",
				new:           "\t\tif preparePendingEvidenceName(name) {\n\t\t\treturn false\n\t\t}",
				wantErr:       "pending-evidence helper",
				authorityOnly: true,
			},
			{
				name:          "pending-evidence-unconditional-early-true",
				file:          "internal/cli/prepare_publish.go",
				old:           "\tfor _, name := range names {\n",
				new:           "\tif len(names) != 0 {\n\t\treturn true\n\t}\n\tfor _, name := range names {\n",
				wantErr:       "pending-evidence helper",
				authorityOnly: true,
			},
			{
				name:          "authority-manual-remediation-removed",
				file:          "internal/cli/prepare_publish.go",
				old:           "Nothing under that lane is tracked. Last resort, from the workspace root run rm -rf ",
				new:           "Last resort, discard it with rm -rf ",
				wantErr:       "authority refusal override",
				authorityOnly: true,
			},
			{
				name:          "authority-destructive-cost-drift",
				file:          "internal/cli/prepare_publish.go",
				old:           "but permanently discards the undo evidence; canonical artifacts remain exactly as the interrupted run left them.",
				new:           "and safely repairs every interrupted artifact.",
				wantErr:       "authority refusal override",
				authorityOnly: true,
			},
			{
				name: "expected-code-with-wrong-retry-remediation",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", " +
					"\"tpatch prepare <slug> --abandon-transaction\"), 3)",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name: "expected-code-with-wrong-remediation-text",
				file: "internal/cli/prepare_publish.go",
				old: "return \"No tpatch workspace was found.\", " +
					"\"Run from inside an initialized workspace or pass --path.\"",
				new: "return \"No tpatch workspace was found.\", " +
					"\"Run tpatch prepare <slug> --abandon-transaction.\"",
				wantErr: "refusal remediation",
			},
			{
				name: "nested-abandon-remediation-alternative",
				file: "internal/cli/prepare_publish.go",
				old: "case \"workspace-not-initialized\":\n" +
					"\t\treturn \"No tpatch workspace was found.\", " +
					"\"Run from inside an initialized workspace or pass --path.\"",
				new: "case \"workspace-not-initialized\":\n" +
					"\t\tmessage := \"No tpatch workspace was found.\"\n" +
					"\t\tif mode == prepareModeAbandon {\n" +
					"\t\t\treturn message, \"wrong remediation\"\n" +
					"\t\t}\n" +
					"\t\treturn message, \"Run from inside an initialized workspace or pass --path.\"",
				wantErr: "remediation alternatives",
			},
			{
				name: "assigned-abandon-remediation-alternative",
				file: "internal/cli/prepare_publish.go",
				old: "case \"workspace-not-initialized\":\n" +
					"\t\treturn \"No tpatch workspace was found.\", " +
					"\"Run from inside an initialized workspace or pass --path.\"",
				new: "case \"workspace-not-initialized\":\n" +
					"\t\tmessage := \"No tpatch workspace was found.\"\n" +
					"\t\tremediation := \"Run from inside an initialized workspace or pass --path.\"\n" +
					"\t\tif mode == prepareModeAbandon {\n" +
					"\t\t\tremediation = \"wrong remediation\"\n" +
					"\t\t}\n" +
					"\t\treturn message, remediation",
				wantErr: "remediation alternatives",
			},
			{
				name: "nested-switch-remediation-alternative",
				file: "internal/cli/prepare_publish.go",
				old: "case \"workspace-not-initialized\":\n" +
					"\t\treturn \"No tpatch workspace was found.\", " +
					"\"Run from inside an initialized workspace or pass --path.\"",
				new: "case \"workspace-not-initialized\":\n" +
					"\t\tmessage := \"No tpatch workspace was found.\"\n" +
					"\t\tif mode == prepareModeAbandon {\n" +
					"\t\t\tswitch retry {\n" +
					"\t\t\tcase \"\":\n" +
					"\t\t\t\treturn message, \"wrong remediation\"\n" +
					"\t\t\t}\n" +
					"\t\t}\n" +
					"\t\treturn message, \"Run from inside an initialized workspace or pass --path.\"",
				wantErr: "remediation alternatives",
			},
			{
				name: "local-variable-from-helper-then-emitted",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\trefused := s7ARLocalRefusal(report)\n" +
					"\t\treturn emitPreparePublishReport(cmd, refused, 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARLocalRefusal(report preparePublishReport) preparePublishReport {\n" +
					"\treturn refusePrepare(report, \"slug-unsafe\", \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name: "two-layer-mixed-return-reused-at-call-sites",
				file: "internal/cli/prepare_publish.go",
				old:  "\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)",
				new: "\t\treturn emitPreparePublishReport(cmd, " +
					"s7ARLayerOne(s7ARLayerOne(report, \"workspace-not-initialized\"), \"workspace-not-initialized\"), 3)",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARLayerOne(report preparePublishReport, code string) preparePublishReport {\n" +
					"\treturn s7ARLayerTwo(report, code)\n" +
					"}\n\n" +
					"func s7ARLayerTwo(report preparePublishReport, code string) preparePublishReport {\n" +
					"\tif report.Slug == \"\" {\n" +
					"\t\treturn report\n" +
					"\t}\n" +
					"\treturn refusePrepare(report, code, \"\")\n" +
					"}\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "unresolved return alternative",
			},
			{
				name: "helper-return-refusal-used-by-emitter",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\tif rawSlug == slug {\n" +
					"\t\treport := newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown)\n" +
					"\t\treturn emitPreparePublishReport(cmd, s7ARMutatedReturnedRefusal(report), 3)\n" +
					"\t}\n\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "type s7ARMutatedReportAlias = preparePublishReport\n\n" +
					"func s7ARMutatedReturnedRefusal(report s7ARMutatedReportAlias) s7ARMutatedReportAlias {\n" +
					"\trefused := (refusePrepare(report, \"workspace-not-initialized\", \"\"))\n" +
					"\treturn refused\n" +
					"}\n\nfunc runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name: "helper-stop-interleaves-at-call-site",
				file: "internal/cli/prepare_publish.go",
				old: "\trepoRoot, err := store.FindProjectRoot(start)\n" +
					"\tif err != nil {\n" +
					"\t\treport := applyPrepareOptionFields(\n" +
					"\t\t\tnewPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,\n" +
					"\t\t)\n" +
					"\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)\n" +
					"\t}\n" +
					"\tif !intent.RootConfinementSupported() {\n" +
					"\t\treport := applyPrepareOptionFields(\n" +
					"\t\t\tnewPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,\n" +
					"\t\t)\n" +
					"\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-unsupported-platform\", \"\"), 3)\n" +
					"\t}",
				new: "\tif err := s7ARMutatedPlatformGate(cmd, slug, options); err != nil {\n" +
					"\t\treturn err\n" +
					"\t}\n" +
					"\trepoRoot, err := store.FindProjectRoot(start)\n" +
					"\tif err != nil {\n" +
					"\t\treport := applyPrepareOptionFields(\n" +
					"\t\t\tnewPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,\n" +
					"\t\t)\n" +
					"\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-not-initialized\", \"\"), 3)\n" +
					"\t}",
				extraOld: "func runPreparePublish(cmd *cobra.Command, rawSlug string, options prepareOptions) error {",
				extraNew: "func s7ARMutatedPlatformGate(cmd *cobra.Command, slug string, options prepareOptions) error {\n" +
					"\tif !intent.RootConfinementSupported() {\n" +
					"\t\treport := applyPrepareOptionFields(newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options)\n" +
					"\t\treturn emitPreparePublishReport(cmd, refusePrepare(report, \"workspace-unsupported-platform\", \"\"), 3)\n" +
					"\t}\n" +
					"\treturn nil\n" +
					"}\n\nfunc runPreparePublish(cmd *cobra.Command, rawSlug string, options prepareOptions) error {",
				wantErr: "reachable pre-abandon stops",
			},
			{
				name: "mutually-recursive-reachable-helpers",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\ts7ARMutatedRecursiveA()\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "func s7ARMutatedRecursiveA() { s7ARMutatedRecursiveB() }\n" +
					"func s7ARMutatedRecursiveB() { s7ARMutatedRecursiveA() }\n\n" +
					"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "recursive pre-abandon call graph",
			},
			{
				name: "unresolved-report-valued-helper",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\tif rawSlug == slug {\n" +
					"\t\treport := newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown)\n" +
					"\t\treturn emitPreparePublishReport(cmd, s7ARMutatedUnresolvedReport(report), 3)\n" +
					"\t}\n\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				extraOld: "func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				extraNew: "var s7ARMutatedReportTransform func(preparePublishReport) preparePublishReport\n\n" +
					"func s7ARMutatedUnresolvedReport(report preparePublishReport) preparePublishReport {\n" +
					"\treturn s7ARMutatedReportTransform(report)\n" +
					"}\n\nfunc runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {",
				wantErr: "unresolved",
			},
			{
				name: "raw-feature-read",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\t_, _ = os.ReadFile(repoRoot + \"/.tpatch/features/\" + slug + \"/status.json\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "feature/status reads",
			},
			{
				name: "raw-os-open",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\t_, _ = os.Open(repoRoot + \"/.tpatch/features/\" + slug + \"/status.json\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "feature/status reads",
			},
			{
				name: "dynamic-feature-read-alias",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\treadFile := os.ReadFile\n" +
					"\t_, _ = readFile(repoRoot + \"/.tpatch/features/\" + slug + \"/status.json\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "feature/status reads",
			},
			{
				name: "parenthesized-feature-read-alias",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\topenFile := os.Open\n" +
					"\t_, _ = (openFile)(repoRoot + \"/.tpatch/features/\" + slug + \"/status.json\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "feature/status reads",
			},
			{
				name:     "raw-git-command",
				file:     "internal/cli/prepare_publish.go",
				extraOld: "\t\"os\"\n",
				extraNew: "\t\"os\"\n\t\"os/exec\"\n",
				old:      "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\t_ = exec.Command(\"git\", \"status\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "Git gates",
			},
			{
				name:     "dynamic-git-command-alias",
				file:     "internal/cli/prepare_publish.go",
				extraOld: "\t\"os\"\n",
				extraNew: "\t\"os\"\n\t\"os/exec\"\n",
				old:      "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\tcommand := exec.Command\n" +
					"\t_ = command(\"git\", \"status\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "Git gates",
			},
			{
				name:     "function-field-git-command-alias",
				file:     "internal/cli/prepare_publish.go",
				extraOld: "\t\"os\"\n",
				extraNew: "\t\"os\"\n\t\"os/exec\"\n",
				old:      "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\trunner := struct {\n\t\trun func(string, ...string) *exec.Cmd\n\t}{run: exec.Command}\n" +
					"\t_ = runner.run(\"git\", \"status\")\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "Git gates",
			},
			{
				name: "new-gitutil-wrapper",
				file: "internal/cli/prepare_publish.go",
				old:  "\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				new: "\t_, _ = gitutil.HeadCommit(repoRoot)\n" +
					"\tif options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon(cmd, repoRoot, slug, options)\n\t}",
				wantErr: "Git gates",
			},
			{
				name: "abandon-rerouted-through-feature-read",
				file: "internal/cli/prepare_publish.go",
				old:  "if options.mode == prepareModeAbandon {\n\t\treturn runPrepareAbandon",
				new:  "if options.mode == prepareModeManual {\n\t\treturn runPrepareAbandon",
			},
			{
				name:    "gate-row-wrong-exit",
				file:    "docs/prds/PRD-prepare-intent-bundle.md",
				old:     "| 2 | canonical slug grammar (§10.5 step 3) | `3`, `slug-unsafe` |",
				new:     "| 2 | canonical slug grammar (§10.5 step 3) | `6`, `slug-unsafe` |",
				wantErr: "abandon gate row 2",
			},
			{
				name:    "gate-row-wrong-remediation",
				file:    "docs/prds/PRD-prepare-intent-bundle.md",
				old:     "supply a slug that satisfies the accepted grammar. **No lane path is named**, because the accepted no-echo rule forbids composing or echoing a path from an unsafe slug, and there is no evidence to point at until one exists |",
				new:     "run `tpatch prepare <slug> --abandon-transaction` and retry |",
				wantErr: "abandon gate row 2",
			},
			{
				name: "unreachable-mutex-postparse-row",
				file: "docs/prds/PRD-prepare-intent-bundle.md",
				old:  "| — | nothing else | — | the branch runs: rules 4–13 above |",
				new:  "| 9 | post-parse `--check` branch | `3`, `status-malformed` | retry without `--check` |\n| — | nothing else | — | the branch runs: rules 4–13 above |",
			},
			{
				name: "syntactically-valid-domain",
				file: "docs/prds/PRD-prepare-intent-bundle.md",
				old:  "every argv that requests a *true* abandon, or that fails\nparsing while naming the flag",
				new:  "every syntactically valid argv that requests a true abandon",
			},
			{
				name: "false-value-in-domain",
				file: "docs/prds/PRD-prepare-intent-bundle.md",
				old:  "`--abandon-transaction=false` is a *false* boolean value that selects\nno mode at all",
				new:  "`--abandon-transaction=false` still names the flag and is inside the abandon domain",
			},
		} {
			mutatedPRD := prd
			mutatedSources := s6CloneSourceSet(sources)
			mutatedProgram := program
			original := mutatedPRD
			var authorityOnly bool
			if fixture.file != "docs/prds/PRD-prepare-intent-bundle.md" {
				original = mutatedSources[fixture.file]
				changedSource := strings.Replace(original, fixture.old, fixture.new, 1)
				if fixture.extraOld != "" {
					beforeExtra := changedSource
					changedSource = strings.Replace(changedSource, fixture.extraOld, fixture.extraNew, 1)
					if changedSource == beforeExtra {
						t.Fatalf("PIB-511 %s secondary sensitivity anchor missing", fixture.name)
					}
				}
				mutatedSources[fixture.file] = changedSource
				authorityOnly = fixture.authorityOnly
				if authorityOnly {
					mutatedProgram, err = s7ARBuildAbandonAuthorityProgram(mutatedSources)
				} else {
					mutatedProgram, err = s7ARBuildPreAbandonProgram(
						s7ARCLIPackageSources(mutatedSources),
					)
				}
				if err != nil {
					t.Fatalf("PIB-511 %s sensitivity is not valid typed source: %v",
						fixture.name, err)
				}
			} else {
				mutatedPRD = strings.Replace(prd, fixture.old, fixture.new, 1)
			}
			var changed string
			if fixture.file == "docs/prds/PRD-prepare-intent-bundle.md" {
				changed = mutatedPRD
			} else {
				changed = mutatedSources[fixture.file]
			}
			if changed == original {
				t.Fatalf("PIB-511 %s sensitivity anchor missing", fixture.name)
			}
			var err error
			if authorityOnly {
				err = s7ARValidateAuthorityRefusalOverride(mutatedProgram)
			} else {
				err = validateS7ARAbandonGateTableWithProgram(
					mutatedPRD, mutatedSources, mutatedProgram,
				)
			}
			if err == nil {
				t.Fatalf("PIB-511 same validator accepted %s", fixture.name)
			}
			if fixture.wantErr != "" && !strings.Contains(err.Error(), fixture.wantErr) {
				t.Fatalf("PIB-511 %s failed for %q, want %q",
					fixture.name, err, fixture.wantErr)
			}
		}

		root, slug := prepareS4Workspace(t, "AR gate domain")
		for _, args := range [][]string{
			{"--path", root, "prepare", slug, "--abandon-transaction", "--check"},
			{"--path", root, "prepare", slug, "--abandon-transaction", "--manual"},
			{"--path", root, "prepare", slug, "--abandon-transaction=true", "--regenerate"},
			{"--path", root, "prepare", slug, "--abandon-transaction", "--dry-run"},
			{"--path", root, "prepare", slug, "--check", "--abandon-transaction=false"},
			{"--path", root, "prepare", slug, "--abandon-transaction", "--unknown-ar-flag"},
			{"--path", root, "prepare", slug, "extra", "--abandon-transaction"},
			{"--path", root, "prepare", "--abandon-transaction"},
		} {
			code, _, _, _ := runPrepare(t, args...)
			if code != 1 {
				t.Fatalf("PIB-511 parse-domain argv %v exit=%d, want 1", args, code)
			}
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction=false", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || report.Mode != prepareModeGenerate || report.Outcome != "published" {
			t.Fatalf("PIB-511 false abandon did not select exact generate behavior: exit=%d report=%+v", code, report)
		}
		trueRoot, trueSlug := prepareS4Workspace(t, "AR explicit true domain")
		s6WriteJournalFixture(t, trueRoot, trueSlug, "journal-corrupt")
		code, stdout, _, _ = runPrepare(
			t, "--path", trueRoot, "prepare", trueSlug,
			"--abandon-transaction=true", "--json", "--quiet",
		)
		report = prepareS4Report(t, stdout)
		if code != 0 || report.Mode != prepareModeAbandon ||
			report.Outcome != "abandon-planned" || report.Abandoned == nil {
			t.Fatalf("PIB-511 explicit true did not select exact abandon preview: exit=%d report=%+v",
				code, report)
		}

		assertRefusal := func(want string, args ...string) {
			t.Helper()
			code, stdout, _, _ := runPrepare(t, args...)
			report := prepareS4Report(t, stdout)
			if code != 3 || report.Refusal == nil || report.Refusal.Code != want {
				t.Fatalf("PIB-511 argv %v = exit:%d report:%+v, want %s",
					args, code, report, want)
			}
		}
		assertRefusal(
			"slug-unsafe",
			"--path", root, "prepare", "../unsafe",
			"--abandon-transaction", "--json", "--quiet",
		)
		assertRefusal(
			"workspace-not-initialized",
			"--path", t.TempDir(), "prepare", slug,
			"--abandon-transaction", "--json", "--quiet",
		)
		oldSupported := prepareMutationAuthoritySupported
		prepareMutationAuthoritySupported = func() bool { return false }
		assertRefusal(
			"prepare-unsupported-platform",
			"--path", root, "prepare", slug,
			"--abandon-transaction", "--json", "--quiet",
		)
		prepareMutationAuthoritySupported = oldSupported
		for _, gate := range []string{
			"lock-filesystem-unsupported",
			"directory-flock-unavailable",
		} {
			restore := s7AQInstallAbandonGateFailure(t, gate)
			assertRefusal(
				gate,
				"--path", root, "prepare", slug,
				"--abandon-transaction", "--json", "--quiet",
			)
			restore()
		}
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatal(err)
		}
		assertRefusal(
			"transaction-in-progress",
			"--path", root, "prepare", slug,
			"--abandon-transaction", "--json", "--quiet",
		)
		if err := authority.Release(); err != nil {
			t.Fatal(err)
		}
	})
}

func validateS7ARAbandonGateTable(prd string, sources map[string]string) error {
	program, err := s7ARBuildPreAbandonProgram(s7ARCLIPackageSources(sources))
	if err != nil {
		return fmt.Errorf("build abandon source graph: %w", err)
	}
	return validateS7ARAbandonGateTableWithProgram(prd, sources, program)
}

func validateS7ARAbandonGateTableWithProgram(
	prd string,
	sources map[string]string,
	program *s6EmissionProgram,
) error {
	requiredDomain := []string{
		"every argv that requests a *true* abandon, or that fails\nparsing while naming the flag",
		"`--abandon-transaction=false` is a *false* boolean value that selects\nno mode at all",
		"`--check --abandon-transaction=false` also stops at row 1",
	}
	for _, phrase := range requiredDomain {
		if !strings.Contains(prd, phrase) {
			return fmt.Errorf("abandon domain clause missing %q", phrase)
		}
	}
	section, err := s7ARSectionBetween(
		prd,
		"| # | Stop, in order | Exit / code | The route this refusal must offer |",
		"\n\nTwo absences in that table",
	)
	if err != nil {
		return err
	}
	rows := s7ARMarkdownRows(section)
	if len(rows) != 9 {
		return fmt.Errorf("abandon gate table rows = %d, want 8 plus terminal branch", len(rows))
	}
	wantRows := s7ARExpectedAbandonGateRows()
	if len(rows) != len(wantRows) {
		return fmt.Errorf("abandon gate table rows = %d, want %d", len(rows), len(wantRows))
	}
	for index, want := range wantRows {
		got := rows[index]
		if len(got) != 4 {
			return fmt.Errorf("abandon gate row %d has %d cells, want 4", index+1, len(got))
		}
		for cell := range got {
			got[cell] = strings.Join(strings.Fields(got[cell]), " ")
		}
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("abandon gate row %d = %#v, want %#v", index+1, got, want)
		}
	}
	if err := s7ARValidateAuthorityRefusalOverride(program); err != nil {
		return err
	}
	analysis, err := s7ARAnalyzeAbandonPrebranchWithProgram(sources, program)
	if err != nil {
		return err
	}
	remediations := analysis.remediations
	wantRemediations := map[string]string{
		"slug-unsafe":                    "Use a lowercase kebab-case feature slug.",
		"workspace-not-initialized":      "Run from inside an initialized workspace or pass --path.",
		"workspace-unsupported-platform": "Run the command on a supported tpatch platform.",
		"prepare-unsupported-platform":   "Run mutating prepare on non-mobile Linux or Darwin.",
		"lock-filesystem-unsupported":    "Move the workspace to a supported local filesystem and retry.",
		"transaction-in-progress":        "Wait for the live operation to finish, then retry.",
		"directory-flock-unavailable":    "Fix workspace access and retry.",
	}
	for code, want := range wantRemediations {
		if got := remediations[code]; got != want {
			return fmt.Errorf("refusal remediation for %s = %q, want %q",
				code, got, want)
		}
	}
	for index := range analysis.stops {
		remediation, present := remediations[analysis.stops[index].code]
		if !present {
			return fmt.Errorf("refusal remediation for %s is absent", analysis.stops[index].code)
		}
		analysis.stops[index].remediation = remediation
	}
	wantImplementation := []s7ARAbandonStopSite{
		{
			code: "slug-unsafe", exit: 3, route: "standard",
			remediation: "Use a lowercase kebab-case feature slug.",
		},
		{
			code: "workspace-not-initialized", exit: 3, route: "standard",
			remediation: "Run from inside an initialized workspace or pass --path.",
		},
		{
			code: "workspace-unsupported-platform", exit: 3, route: "standard",
			remediation: "Run the command on a supported tpatch platform.",
		},
		{
			code: "prepare-unsupported-platform", exit: 3, route: "authority",
			remediation: "Run mutating prepare on non-mobile Linux or Darwin.",
		},
		{
			code: "lock-filesystem-unsupported", exit: 3, route: "authority",
			remediation: "Move the workspace to a supported local filesystem and retry.",
		},
		{
			code: "transaction-in-progress", exit: 3, route: "authority",
			remediation: "Wait for the live operation to finish, then retry.",
		},
		{
			code: "directory-flock-unavailable", exit: 3, route: "authority",
			remediation: "Fix workspace access and retry.",
		},
	}
	if !reflect.DeepEqual(analysis.stops, wantImplementation) {
		return fmt.Errorf(
			"reachable pre-abandon stops = %v, want exact ordered sites %v",
			analysis.stops, wantImplementation,
		)
	}
	if len(analysis.gitCalls) != 0 {
		return fmt.Errorf("pre-abandon flow reaches Git gates: %v", s7ARSortedKeys(analysis.gitCalls))
	}
	if len(analysis.featureReads) != 0 {
		return fmt.Errorf("pre-abandon flow reaches feature/status reads: %v",
			s7ARSortedKeys(analysis.featureReads))
	}
	if len(analysis.stepSixCalls) != 0 {
		return fmt.Errorf("pre-abandon flow reaches step-6 recovery/pending gates: %v",
			s7ARSortedKeys(analysis.stepSixCalls))
	}
	return nil
}

func s7ARValidateAuthorityRefusalOverride(
	program *s6EmissionProgram,
) error {
	function, err := s7ARUniqueProgramFunction(
		program, "internal/cli.prepareAuthorityRefusal",
	)
	if err != nil {
		return err
	}
	const expected = `{
		report = refusePrepare(report, code, "")
		if code == "transaction-in-progress" {
			return report
		}
		if code != "prepare-unsupported-platform" &&
			code != "lock-filesystem-unsupported" &&
			code != "directory-flock-unavailable" {
			return report
		}
		if !prepareLaneHasPendingEvidence(repoRoot, slug) {
			return report
		}
		lane := ".tpatch/local/intent-prepare/" + slug + "/"
		detail := ""
		if class != "" {
			detail = " (" + class + ")"
		}
		report.Refusal.Message += " Pending evidence remains in " + lane + detail + "."
		report.Refusal.Remediation =
			"Nothing under that lane is tracked. Last resort, from the workspace root run rm -rf " + lane +
				". This unblocks the slug without changing .tpatch/features/, but permanently discards the undo evidence; canonical artifacts remain exactly as the interrupted run left them."
		return report
	}`
	got := strings.Join(strings.Fields(s7ARNodeString(function.function.Body)), " ")
	want := strings.Join(strings.Fields(expected), " ")
	if got != want {
		return fmt.Errorf(
			"authority refusal override does not exactly bind the evidence predicate, "+
				"eligible ordered codes, repo-relative lane, manual route and destructive cost:\n"+
				"got:  %s\nwant: %s",
			got, want,
		)
	}
	if err := s7ARValidatePendingEvidenceHelper(program, function); err != nil {
		return err
	}
	return nil
}

func s7ARValidatePendingEvidenceHelper(
	program *s6EmissionProgram,
	authority *s6EmissionFunction,
) error {
	helper, err := s7ARUniqueProgramFunction(
		program, "internal/cli.prepareLaneHasPendingEvidence",
	)
	if err != nil {
		return err
	}
	predicate, err := s7ARUniqueProgramFunction(
		program, "internal/cli.preparePendingEvidenceName",
	)
	if err != nil {
		return err
	}
	controlPredicate, err := s7ARUniqueProgramFunction(
		program, "internal/cli.prepareControlEvidenceName",
	)
	if err != nil {
		return err
	}
	stagePredicate, err := s7ARUniqueProgramFunction(
		program, "internal/cli.prepareStageEvidenceName",
	)
	if err != nil {
		return err
	}
	const expected = `{
		root, err := os.OpenRoot(repoRoot)
		if err != nil {
			return false
		}
		defer root.Close()
		lane := ".tpatch/local/intent-prepare/" + slug
		info, err := root.Lstat(lane)
		if err != nil || prepareRefusedInfo(info) || !info.IsDir() {
			return false
		}
		directory, err := root.Open(lane)
		if err != nil {
			return false
		}
		names, readErr := directory.Readdirnames(-1)
		closeErr := directory.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) || closeErr != nil {
			return false
		}
		for _, name := range names {
			if preparePendingEvidenceName(name) {
				return true
			}
		}
		return false
	}`
	got := strings.Join(strings.Fields(s7ARNodeString(helper.function.Body)), " ")
	want := strings.Join(strings.Fields(expected), " ")
	if got != want {
		return fmt.Errorf(
			"pending-evidence helper does not exactly bind positive canonical "+
				"entry matching and false fallthrough:\ngot:  %s\nwant: %s",
			got, want,
		)
	}
	const expectedPredicate = `{
		return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)
	}`
	gotPredicate := strings.Join(
		strings.Fields(s7ARNodeString(predicate.function.Body)), " ",
	)
	wantPredicate := strings.Join(strings.Fields(expectedPredicate), " ")
	if gotPredicate != wantPredicate {
		return fmt.Errorf(
			"pending-evidence predicate does not exactly bind the canonical "+
				"control-or-stage semantics:\ngot:  %s\nwant: %s",
			gotPredicate, wantPredicate,
		)
	}
	helperObject, ok := program.model.definitions[helper.function.Name].(*types.Func)
	if !ok {
		return errors.New("pending-evidence helper has no canonical typed function")
	}
	predicateObject, ok := program.model.definitions[predicate.function.Name].(*types.Func)
	if !ok {
		return errors.New("pending-evidence predicate has no canonical typed function")
	}
	controlObject, ok := program.model.definitions[controlPredicate.function.Name].(*types.Func)
	if !ok {
		return errors.New("control-evidence predicate has no canonical typed function")
	}
	stageObject, ok := program.model.definitions[stagePredicate.function.Name].(*types.Func)
	if !ok {
		return errors.New("stage-evidence predicate has no canonical typed function")
	}
	if predicate.function.Type.Params == nil ||
		len(predicate.function.Type.Params.List) != 1 ||
		len(predicate.function.Type.Params.List[0].Names) != 1 {
		return errors.New("pending-evidence predicate has no unique canonical parameter")
	}
	parameter := predicate.function.Type.Params.List[0].Names[0]
	parameterObject := program.model.definitions[parameter]
	if parameterObject == nil {
		return errors.New("pending-evidence predicate parameter has no typed object")
	}
	authorityCalls := 0
	ast.Inspect(authority.function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident)
		if ok && program.model.uses[identifier] == helperObject {
			authorityCalls++
		}
		return true
	})
	predicateCalls := 0
	ast.Inspect(helper.function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident)
		if ok && program.model.uses[identifier] == predicateObject {
			predicateCalls++
		}
		return true
	})
	controlCalls := 0
	stageCalls := 0
	wrongArguments := 0
	ast.Inspect(predicate.function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident)
		if !ok {
			return true
		}
		object := program.model.uses[identifier]
		if object != controlObject && object != stageObject {
			return true
		}
		if object == controlObject {
			controlCalls++
		} else {
			stageCalls++
		}
		if len(call.Args) != 1 {
			wrongArguments++
			return true
		}
		argument, ok := s7ARUnwrapCallExpression(call.Args[0]).(*ast.Ident)
		if !ok || program.model.uses[argument] != parameterObject {
			wrongArguments++
		}
		return true
	})
	if authorityCalls != 1 || predicateCalls != 1 {
		return fmt.Errorf(
			"pending-evidence typed call flow = authority:%d predicate:%d, want 1/1",
			authorityCalls, predicateCalls,
		)
	}
	if controlCalls != 1 || stageCalls != 1 || wrongArguments != 0 {
		return fmt.Errorf(
			"pending-evidence predicate typed call flow = control:%d stage:%d "+
				"wrong-arguments:%d, want 1/1/0",
			controlCalls, stageCalls, wrongArguments,
		)
	}
	return nil
}

func s7ARAssertAuthorityRefusalBehavior(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	const slug = "s7-ar-authority"
	base := map[string]string{
		"prepare-unsupported-platform": "Run mutating prepare on non-mobile Linux or Darwin.",
		"lock-filesystem-unsupported":  "Move the workspace to a supported local filesystem and retry.",
		"directory-flock-unavailable":  "Fix workspace access and retry.",
		"transaction-in-progress":      "Wait for the live operation to finish, then retry.",
	}
	reportFor := func(code, class string) preparePublishReport {
		return prepareAuthorityRefusal(
			root,
			slug,
			preparePublishReport{Mode: prepareModeAbandon, Slug: slug},
			code,
			class,
		)
	}
	for _, code := range []string{
		"prepare-unsupported-platform",
		"lock-filesystem-unsupported",
		"directory-flock-unavailable",
		"transaction-in-progress",
	} {
		report := reportFor(code, "test-class")
		if report.Refusal == nil || report.Refusal.Code != code ||
			report.Refusal.Remediation != base[code] ||
			strings.Contains(report.Refusal.Message, ".tpatch/local/") {
			t.Fatalf("PIB-511 empty-lane %s refusal = %#v", code, report.Refusal)
		}
	}
	lane := ".tpatch/local/intent-prepare/" + slug + "/"
	absoluteLane := filepath.Join(root, filepath.FromSlash(lane))
	if err := os.MkdirAll(absoluteLane, 0o755); err != nil {
		t.Fatal(err)
	}
	const class = "test-class"
	wantRemediation := "Nothing under that lane is tracked. Last resort, from the workspace root run rm -rf " +
		lane +
		". This unblocks the slug without changing .tpatch/features/, but permanently discards the undo evidence; canonical artifacts remain exactly as the interrupted run left them."
	assertEvidence := func(name string, wantEvidence bool) {
		t.Helper()
		target := filepath.Join(absoluteLane, name)
		if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, code := range []string{
			"prepare-unsupported-platform",
			"lock-filesystem-unsupported",
			"directory-flock-unavailable",
		} {
			report := reportFor(code, class)
			baseMessage, _ := prepareRefusalText(code, prepareModeAbandon, slug, "")
			if !wantEvidence {
				if report.Refusal == nil ||
					report.Refusal.Message != baseMessage ||
					report.Refusal.Remediation != base[code] {
					t.Fatalf("PIB-511 unrelated entry %q changed %s refusal: %#v",
						name, code, report.Refusal)
				}
				continue
			}
			wantMessage := baseMessage + " Pending evidence remains in " + lane + " (" + class + ")."
			if report.Refusal == nil ||
				report.Refusal.Code != code ||
				report.Refusal.Message != wantMessage ||
				report.Refusal.Remediation != wantRemediation {
				t.Fatalf("PIB-511 evidence %q %s refusal = %#v, want message %q remediation %q",
					name, code, report.Refusal, wantMessage, wantRemediation)
			}
			if strings.Contains(report.Refusal.Message, root) ||
				strings.Contains(report.Refusal.Remediation, root) {
				t.Fatalf("PIB-511 evidence %q %s leaked absolute root %q",
					name, code, root)
			}
		}
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
	}
	assertEvidence("unrelated.txt", false)
	assertEvidence("journal.json", true)
	assertEvidence("stage-0123456789ab", true)
	if preparePendingEvidenceName("unrelated.txt") {
		t.Fatal("PIB-511 unrelated entry satisfied the compiled pending-evidence predicate")
	}
	contentionEvidence := filepath.Join(absoluteLane, "stage-fedcba987654")
	if err := os.WriteFile(contentionEvidence, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	contention := reportFor("transaction-in-progress", class)
	if contention.Refusal == nil ||
		contention.Refusal.Remediation != base["transaction-in-progress"] ||
		strings.Contains(contention.Refusal.Message, lane) ||
		strings.Contains(contention.Refusal.Remediation, "rm -rf") {
		t.Fatalf("PIB-511 contention with evidence offered manual removal: %#v",
			contention.Refusal)
	}
	if err := os.Remove(contentionEvidence); err != nil {
		t.Fatal(err)
	}
}

func s7ARExpectedAbandonGateRows() [][]string {
	return [][]string{
		{
			"1",
			"cobra/pflag parse, arity, or a mode mutex — the stop every argv that names this flag but does not parse, or combines it with `--check` or `--dry-run`, takes, whatever boolean value it spells",
			"`1`, pflag's own text",
			"fix the command line; nothing was read, opened or locked",
		},
		{
			"2",
			"canonical slug grammar (§10.5 step 3)",
			"`3`, `slug-unsafe`",
			"supply a slug that satisfies the accepted grammar. **No lane path is named**, because the accepted no-echo rule forbids composing or echoing a path from an unsafe slug, and there is no evidence to point at until one exists",
		},
		{
			"3",
			"workspace discovery (§10.5 step 4)",
			"`3`, `workspace-not-initialized`",
			"**truly unavoidable**: with no workspace root there is no repo-relative lane to name and no `.tpatch/` to inspect. The remediation is to run from inside a tpatch workspace, or pass `--path`; it offers no manual removal because it cannot honestly name a target",
		},
		{
			"4",
			"read-boundary platform allowlist (§10.5 step 5)",
			"`3`, `workspace-unsupported-platform`",
			"**truly unavoidable**: this is the boundary that resolves the lane at all, so no procedure this document could print would be executable on that host. It states that and stops",
		},
		{
			"5",
			"mutating platform gate (§7.4.2, §10.5 step 8)",
			"`3`, `prepare-unsupported-platform`",
			"with evidence present: the repo-relative lane and the last-resort `rm -rf`, with its cost, exactly as the block below spells out. With an empty lane: nothing to remove, so nothing is offered",
		},
		{
			"6",
			"root-filesystem classification (§7.4.2, §10.5 step 8)",
			"`3`, `lock-filesystem-unsupported`",
			"identical to row 5",
		},
		{
			"7",
			"**lock contention** (§10.5 step 9)",
			"`3`, `transaction-in-progress`",
			"**wait and retry — and nothing else.** No manual removal is named here even when evidence exists, because the evidence may be the *live* undo journal of a publication a sibling process is executing right now, and deleting it under that holder would destroy the undo evidence of a running transaction. The holder's identity is unknowable (§12.5), so the only safe observation is to retry (PIB-512)",
		},
		{
			"8",
			"**the authority could not be established at all** (§7.4.1, §10.5 step 9) — the workspace `*os.Root` failed to open, `root.Open(\".\")` failed, or `flock` failed for any non-contention reason",
			"`3`, `directory-flock-unavailable`",
			"identical to row 5: no lock was *taken at all*, so no holder is implied and the manual procedure is safe to name",
		},
		{"—", "nothing else", "—", "the branch runs: rules 4–13 above"},
	}
}

func s7ARAbandonRefusalRemediations(
	program *s6EmissionProgram,
) (map[string]string, error) {
	function, err := s7ARUniqueProgramFunction(
		program, "internal/cli.prepareRefusalText",
	)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	required := map[string]bool{
		"slug-unsafe":                    true,
		"workspace-not-initialized":      true,
		"workspace-unsupported-platform": true,
		"prepare-unsupported-platform":   true,
		"lock-filesystem-unsupported":    true,
		"transaction-in-progress":        true,
		"directory-flock-unavailable":    true,
	}
	frame := &s7ARPreAbandonFrame{
		node: function, parameters: map[types.Object]s7ARBoundExpression{},
	}
	var extractErr error
	ast.Inspect(function.function.Body, func(node ast.Node) bool {
		if extractErr != nil {
			return false
		}
		clause, ok := node.(*ast.CaseClause)
		if !ok || len(clause.List) == 0 {
			return true
		}
		var codes []string
		for _, expression := range clause.List {
			typed, present := program.model.expressionTypes[expression]
			if !present || typed.Value == nil ||
				typed.Value.Kind() != constant.String {
				continue
			}
			code := constant.StringVal(typed.Value)
			if required[code] {
				codes = append(codes, code)
			}
		}
		if len(codes) == 0 {
			return true
		}
		var remediations []string
		ast.Inspect(&ast.BlockStmt{List: clause.Body}, func(candidate ast.Node) bool {
			if _, nested := candidate.(*ast.FuncLit); nested {
				return false
			}
			returnStatement, ok := candidate.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(returnStatement.Results) != 2 {
				extractErr = fmt.Errorf(
					"prepareRefusalText codes %v have non-pair return", codes,
				)
				return false
			}
			values, resolved, resolveErr := s7ARResolveRefusalCodes(
				program, frame, returnStatement.Results[1],
				false, nil, map[string]bool{},
			)
			if resolveErr != nil || !resolved || len(values) == 0 {
				if resolveErr == nil {
					resolveErr = errors.New("remediation value did not resolve")
				}
				extractErr = fmt.Errorf(
					"prepareRefusalText codes %v remediation is unresolved: %w",
					codes, resolveErr,
				)
				return false
			}
			remediations = append(remediations, values...)
			return true
		})
		if extractErr != nil {
			return false
		}
		if len(remediations) == 0 {
			extractErr = fmt.Errorf(
				"prepareRefusalText codes %v have no remediation return", codes,
			)
			return false
		}
		remediation := remediations[0]
		for _, alternative := range remediations[1:] {
			if alternative != remediation {
				extractErr = fmt.Errorf(
					"prepareRefusalText codes %v have remediation alternatives %q and %q",
					codes, remediation, alternative,
				)
				return false
			}
		}
		for _, code := range codes {
			if _, duplicate := result[code]; duplicate {
				extractErr = fmt.Errorf(
					"prepareRefusalText code %s has duplicate remediation", code,
				)
				return false
			}
			result[code] = remediation
		}
		return true
	})
	if extractErr != nil {
		return nil, extractErr
	}
	if len(result) == 0 {
		return nil, errors.New("prepareRefusalText remediation map is empty")
	}
	return result, nil
}

type s7ARAbandonStopSite struct {
	code        string
	exit        int
	route       string
	retry       string
	remediation string
}

type s7ARAbandonFlowEvidence struct {
	stops        []s7ARAbandonStopSite
	gitCalls     map[string]bool
	featureReads map[string]bool
	stepSixCalls map[string]bool
	remediations map[string]string
}

func s7ARProductionSourceSet(t *testing.T) map[string]string {
	t.Helper()
	sources, err := s6CatalogProductionSources(t)
	if err != nil {
		t.Fatal(err)
	}
	return sources
}

func s7ARCLIPackageSources(sources map[string]string) map[string]string {
	result := map[string]string{}
	for rel, source := range sources {
		if strings.HasPrefix(rel, "internal/cli/") &&
			strings.HasSuffix(rel, ".go") &&
			!strings.HasSuffix(rel, "_test.go") {
			result[rel] = source
		}
	}
	return result
}

func s7ARBuildPreAbandonProgram(
	sources map[string]string,
) (*s6EmissionProgram, error) {
	sources = s7ARPreAbandonProgramSources(sources)
	model, err := s6BuildSourceTypeModel(s6EmissionTypeSources(sources))
	if err != nil {
		return nil, err
	}
	program := &s6EmissionProgram{
		functions: map[string]*s6EmissionFunction{},
		byBase:    map[string][]string{},
		model:     model,
	}
	rels := make([]string, 0, len(sources))
	for rel := range sources {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		file := model.files[rel]
		if file == nil {
			return nil, fmt.Errorf("pre-abandon typed source %s is absent", rel)
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		imports := s6FileImportDirs(file)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			baseKey := s6PrepareFunctionKey(pkg, function)
			key := s6PrepareReachabilityKey(rel, pkg, function)
			program.functions[key] = &s6EmissionFunction{
				key: key, baseKey: baseKey, rel: rel, pkg: pkg, file: file,
				function: function, imports: imports,
				locals:         s6FunctionLocalTypes(pkg, file, function, model),
				bindings:       s6FunctionBindingValues(function),
				objectBindings: s6FunctionObjectBindingValues(function),
				literalCalls:   s6FunctionLiteralCalls(function),
				closureCache:   map[*ast.FuncLit]s6ClosureInvocationSummary{},
			}
			program.byBase[baseKey] = append(program.byBase[baseKey], key)
		}
	}
	return program, nil
}

func s7ARPreAbandonProgramSources(
	sources map[string]string,
) map[string]string {
	const (
		publishRel = "internal/cli/prepare_publish.go"
		archiveRel = "internal/cli/feature_intent_archive.go"
	)
	publish, present := sources[publishRel]
	if !present {
		return sources
	}
	selected := map[string]string{publishRel: publish}
	if archive, ok := sources[archiveRel]; ok {
		selected[archiveRel] = archive
	}
	for rel, source := range sources {
		if strings.HasPrefix(filepath.Base(rel), "zz_s7_ar_") {
			selected[rel] = source
		}
	}
	return selected
}

func s7ARBuildAbandonAuthorityProgram(
	sources map[string]string,
) (*s6EmissionProgram, error) {
	const rel = "internal/cli/prepare_publish.go"
	source, ok := sources[rel]
	if !ok {
		return nil, errors.New("prepare authority source is absent")
	}
	functions := map[string]string{}
	file, err := s6ParseSource(rel, source)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		switch function.Name.Name {
		case "prepareAuthorityRefusal",
			"prepareLaneHasPendingEvidence",
			"preparePendingEvidenceName",
			"prepareControlEvidenceName",
			"prepareStageEvidenceName",
			"validPrepareHex":
			if _, duplicate := functions[function.Name.Name]; duplicate {
				return nil, fmt.Errorf(
					"prepare authority function %s is ambiguous",
					function.Name.Name,
				)
			}
			functions[function.Name.Name] = s7ARNodeString(function)
		}
	}
	for _, name := range []string{
		"prepareAuthorityRefusal",
		"prepareLaneHasPendingEvidence",
		"preparePendingEvidenceName",
		"prepareControlEvidenceName",
		"prepareStageEvidenceName",
		"validPrepareHex",
	} {
		if functions[name] == "" {
			return nil, fmt.Errorf("prepare authority function %s is absent", name)
		}
	}
	synthetic := map[string]string{
		"internal/cli/zz_s7_ar_authority_helper.go": `package cli

import (
	"errors"
	"io"
	"os"
	"strings"
)

func prepareRefusedInfo(os.FileInfo) bool { return false }

` + functions["validPrepareHex"] + "\n\n" +
			functions["prepareControlEvidenceName"] + "\n\n" +
			functions["prepareStageEvidenceName"] + "\n\n" +
			functions["preparePendingEvidenceName"] + "\n\n" +
			functions["prepareLaneHasPendingEvidence"] + "\n",
		"internal/cli/zz_s7_ar_authority_refusal.go": `package cli

type prepareRefusalReport struct {
	Message string
	Remediation string
}

type preparePublishReport struct {
	Refusal *prepareRefusalReport
}

func refusePrepare(report preparePublishReport, code, remediation string) preparePublishReport {
	return report
}

` + functions["prepareAuthorityRefusal"] + "\n",
	}
	return s7ARBuildPreAbandonProgram(synthetic)
}

func validateS7ARAbandonReadOrdering(sources map[string]string) error {
	_, err := s7ARAnalyzeAbandonPrebranch(sources)
	return err
}

func s7ARAnalyzeAbandonPrebranch(
	sources map[string]string,
) (s7ARAbandonFlowEvidence, error) {
	program, err := s7ARBuildPreAbandonProgram(s7ARCLIPackageSources(sources))
	if err != nil {
		return s7ARAbandonFlowEvidence{}, fmt.Errorf(
			"build abandon source graph: %w", err,
		)
	}
	return s7ARAnalyzeAbandonPrebranchWithProgram(sources, program)
}

func s7ARAnalyzeAbandonPrebranchWithProgram(
	sources map[string]string,
	program *s6EmissionProgram,
) (s7ARAbandonFlowEvidence, error) {
	evidence := s7ARAbandonFlowEvidence{
		stops:        []s7ARAbandonStopSite{},
		gitCalls:     map[string]bool{},
		featureReads: map[string]bool{},
		stepSixCalls: map[string]bool{},
		remediations: map[string]string{},
	}
	outer, err := s7ARUniqueProgramFunction(program, "internal/cli.runPreparePublish")
	if err != nil {
		return evidence, err
	}
	abandon, err := s7ARUniqueProgramFunction(program, "internal/cli.runPrepareAbandon")
	if err != nil {
		return evidence, err
	}
	branchIndex := -1
	for index, statement := range outer.function.Body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok || s7ARNodeString(branch.Cond) != "options.mode == prepareModeAbandon" {
			continue
		}
		if len(branch.Body.List) != 1 {
			return evidence, errors.New("abandon branch body is not a sole return")
		}
		returnStatement, ok := branch.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(returnStatement.Results) != 1 {
			return evidence, errors.New("abandon branch is not a sole return")
		}
		call, ok := returnStatement.Results[0].(*ast.CallExpr)
		if !ok || s6CallName(call) != "runPrepareAbandon" {
			return evidence, errors.New("abandon branch does not return runPrepareAbandon")
		}
		branchIndex = index
		break
	}
	if branchIndex < 0 {
		return evidence, errors.New("exact abandon branch is absent")
	}
	hookIndex := -1
	acquireIndex := -1
	for index, statement := range abandon.function.Body.List {
		ast.Inspect(statement, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch s6CallName(call) {
			case "prepareAcquireAuthority":
				acquireIndex = index
			case "beforeAbandonBranch":
				hookIndex = index
			}
			return true
		})
	}
	if acquireIndex < 0 || hookIndex < 0 || acquireIndex >= hookIndex {
		return evidence, fmt.Errorf(
			"authority/branch ordering = acquire:%d branch:%d", acquireIndex, hookIndex,
		)
	}
	outerPrefix := outer.function.Body.List[:branchIndex+1]
	abandonPrefix := abandon.function.Body.List[:hookIndex+1]
	intentlockSource := sources["internal/intentlock/error.go"]
	if intentlockSource == "" {
		return evidence, errors.New("intentlock error source absent from source universe")
	}
	authorityCodes, err := s7ARIntentlockAuthorityCodes(intentlockSource)
	if err != nil {
		return evidence, err
	}
	evidence.remediations, err = s7ARAbandonRefusalRemediations(program)
	if err != nil {
		return evidence, err
	}
	active := map[string]bool{}
	if err := s7ARScanPreAbandonCalls(
		program,
		&s7ARPreAbandonFrame{node: outer, parameters: map[types.Object]s7ARBoundExpression{}},
		outerPrefix, abandon, abandonPrefix,
		authorityCodes, active, outer.key, &evidence,
	); err != nil {
		return evidence, err
	}
	return evidence, nil
}

func s7ARUniqueProgramFunction(
	program *s6EmissionProgram,
	base string,
) (*s6EmissionFunction, error) {
	keys := program.byBase[base]
	if len(keys) != 1 {
		return nil, fmt.Errorf("%s declarations = %d, want 1", base, len(keys))
	}
	return program.functions[keys[0]], nil
}

func s7ARIntentlockAuthorityCodes(source string) ([]string, error) {
	var result []string
	for _, name := range []string{
		"CodeLockFilesystemUnsupported",
		"CodeTransactionInProgress",
		"CodeDirectoryFlockUnavailable",
	} {
		match := regexp.MustCompile(
			`\b` + regexp.QuoteMeta(name) + `\s+Code\s*=\s*"([^"]+)"`,
		).FindStringSubmatch(source)
		if len(match) != 2 {
			return nil, fmt.Errorf("intentlock authority code %s missing or ambiguous", name)
		}
		result = append(result, match[1])
	}
	return result, nil
}

type s7ARPreAbandonFrame struct {
	node       *s6EmissionFunction
	parameters map[types.Object]s7ARBoundExpression
}

type s7ARBoundExpression struct {
	frame      *s7ARPreAbandonFrame
	expression ast.Expr
}

type s7ARRefusalAlternative struct {
	site s7ARAbandonStopSite
}

func s7ARRefusalAlternativesFromReport(
	program *s6EmissionProgram,
	frame *s7ARPreAbandonFrame,
	expression ast.Expr,
	exit int,
	authorityCodes []string,
	active map[string]bool,
) ([]s7ARRefusalAlternative, error) {
	expression = s7ARUnwrapCallExpression(expression)
	if identifier, ok := expression.(*ast.Ident); ok {
		object := program.model.uses[identifier]
		if object == nil {
			object = program.model.definitions[identifier]
		}
		if detail, changed := s7ARAddressTakenValueSideEffect(
			program.model, frame.node.function, identifier, object,
		); changed {
			return nil, fmt.Errorf(
				"%s:%s address-taken report value %s has unresolved side effects at %s",
				frame.node.rel, frame.node.function.Name.Name,
				identifier.Name, detail,
			)
		}
		candidates, dominates := s7ARDominatingReportBindings(
			program.model, frame.node.function, identifier, object,
		)
		if !dominates {
			candidates = s6FunctionBindingsBefore(
				frame.node.function, expression.Pos(),
			).candidates(identifier)
		}
		if len(candidates) != 0 {
			var result []s7ARRefusalAlternative
			for _, candidate := range candidates {
				alternatives, err := s7ARRefusalAlternativesFromReport(
					program, frame, candidate, exit, authorityCodes, active,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"%s:%s report binding %s has an unresolved alternative: %w",
						frame.node.rel, frame.node.function.Name.Name,
						identifier.Name, err,
					)
				}
				result = append(result, alternatives...)
			}
			if bound, present := frame.parameters[object]; present && !dominates {
				alternatives, err := s7ARRefusalAlternativesFromReport(
					program, bound.frame, bound.expression, exit, authorityCodes, active,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"%s:%s unchanged parameter alternative %s is unresolved: %w",
						frame.node.rel, frame.node.function.Name.Name,
						identifier.Name, err,
					)
				}
				result = append(result, alternatives...)
			}
			if len(result) == 0 {
				return nil, fmt.Errorf(
					"%s:%s report binding %s has no alternatives",
					frame.node.rel, frame.node.function.Name.Name, identifier.Name,
				)
			}
			return result, nil
		}
		if bound, present := frame.parameters[object]; present {
			return s7ARRefusalAlternativesFromReport(
				program, bound.frame, bound.expression, exit, authorityCodes, active,
			)
		}
	}
	call, ok := expression.(*ast.CallExpr)
	if ok {
		switch {
		case s7ARTypedCallMatches(
			program, frame.node, call, "internal/cli.refusePrepare",
		):
			if len(call.Args) != 3 {
				return nil, fmt.Errorf("%s:%s refusePrepare arity = %d",
					frame.node.rel, frame.node.function.Name.Name, len(call.Args))
			}
			codes, resolved, err := s7ARResolveRefusalCodes(
				program, frame, call.Args[1], false, authorityCodes, map[string]bool{},
			)
			if err != nil || !resolved || len(codes) == 0 {
				if err == nil {
					err = errors.New("standard refusal code did not resolve")
				}
				return nil, err
			}
			retries, retryResolved, retryErr := s7ARResolveRefusalCodes(
				program, frame, call.Args[2], false, authorityCodes, map[string]bool{},
			)
			if retryErr != nil || !retryResolved || len(retries) == 0 {
				if retryErr == nil {
					retryErr = errors.New("standard refusal retry did not resolve")
				}
				return nil, retryErr
			}
			result := make([]s7ARRefusalAlternative, 0, len(codes)*len(retries))
			for _, code := range codes {
				for _, retry := range retries {
					result = append(result, s7ARRefusalAlternative{
						site: s7ARAbandonStopSite{
							code: code, exit: exit, route: "standard", retry: retry,
						},
					})
				}
			}
			return result, nil
		case s7ARTypedCallMatches(
			program, frame.node, call, "internal/cli.prepareAuthorityRefusal",
		):
			if len(call.Args) != 5 {
				return nil, fmt.Errorf("%s:%s prepareAuthorityRefusal arity = %d",
					frame.node.rel, frame.node.function.Name.Name, len(call.Args))
			}
			codes, resolved, err := s7ARResolveRefusalCodes(
				program, frame, call.Args[3], true, authorityCodes, map[string]bool{},
			)
			if err != nil || !resolved || len(codes) == 0 {
				if err == nil {
					err = errors.New("authority refusal code did not resolve")
				}
				return nil, err
			}
			result := make([]s7ARRefusalAlternative, 0, len(codes))
			for _, code := range codes {
				result = append(result, s7ARRefusalAlternative{
					site: s7ARAbandonStopSite{
						code: code, exit: exit, route: "authority", retry: "",
					},
				})
			}
			return result, nil
		}
		if s7ARPrebranchBuiltinOrConversion(program, call) && len(call.Args) == 1 {
			return s7ARRefusalAlternativesFromReport(
				program, frame, call.Args[0], exit, authorityCodes, active,
			)
		}
		identities, unresolved := s7ARPreAbandonCallIdentities(program, frame.node, call)
		if unresolved || len(identities) != 1 {
			return nil, fmt.Errorf(
				"%s:%s report-valued call %s is unresolved",
				frame.node.rel, frame.node.function.Name.Name, s6CallName(call),
			)
		}
		identity := identities[0]
		if identity.literal != nil {
			return nil, fmt.Errorf(
				"%s:%s report-valued function literal is unsupported",
				frame.node.rel, frame.node.function.Name.Name,
			)
		}
		targets := program.byBase[identity.functionKey()]
		if identity.pkg != "internal/cli" || len(targets) != 1 {
			return nil, fmt.Errorf(
				"%s:%s report-valued helper %s has no unique source body",
				frame.node.rel, frame.node.function.Name.Name, identity.functionKey(),
			)
		}
		callee := program.functions[targets[0]]
		key := "report:" + callee.key
		if active[key] {
			return nil, fmt.Errorf(
				"reachable recursive report helper %s", identity.functionKey(),
			)
		}
		active[key] = true
		defer delete(active, key)
		calleeFrame, err := s7ARBindPreAbandonCall(program, frame, call, callee)
		if err != nil {
			return nil, err
		}
		returns := s7ARReturnExpressions(callee.function)
		if len(returns) == 0 {
			return nil, fmt.Errorf(
				"report-valued helper %s has no return expression", identity.functionKey(),
			)
		}
		var result []s7ARRefusalAlternative
		for _, returned := range returns {
			alternatives, returnErr := s7ARRefusalAlternativesFromReport(
				program, calleeFrame, returned, exit, authorityCodes, active,
			)
			if returnErr != nil {
				return nil, fmt.Errorf(
					"report-valued helper %s has an unresolved return alternative: %w",
					identity.functionKey(), returnErr,
				)
			}
			result = append(result, alternatives...)
		}
		return result, nil
	}
	if s7ARExpressionHasNamedType(
		program.model, expression, "preparePublishReport",
	) {
		return nil, fmt.Errorf(
			"%s:%s bare report value has no proven refusal",
			frame.node.rel, frame.node.function.Name.Name,
		)
	}
	return nil, fmt.Errorf(
		"%s:%s report expression %s is unresolved",
		frame.node.rel, frame.node.function.Name.Name, s7ARNodeString(expression),
	)
}

func s7ARDominatingReportBindings(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	identifier *ast.Ident,
	object types.Object,
) ([]ast.Expr, bool) {
	var block *ast.BlockStmt
	statementIndex := -1
	ast.Inspect(function.Body, func(node ast.Node) bool {
		candidate, ok := node.(*ast.BlockStmt)
		if !ok || identifier.Pos() < candidate.Pos() || identifier.Pos() > candidate.End() {
			return true
		}
		for index, statement := range candidate.List {
			if identifier.Pos() >= statement.Pos() && identifier.Pos() <= statement.End() {
				block = candidate
				statementIndex = index
				break
			}
		}
		return true
	})
	if block == nil || statementIndex < 0 {
		return nil, false
	}
	for index := statementIndex - 1; index >= 0; index-- {
		statement := block.List[index]
		if values, assigned := s7ARDirectReportAssignment(
			model, statement, identifier.Name, object,
		); assigned {
			return values, true
		}
		if s7ARStatementMayAssignReport(
			model, statement, identifier.Name, object,
		) {
			return nil, false
		}
	}
	return nil, false
}

func s7ARDirectReportAssignment(
	model *s6SourceTypeModel,
	statement ast.Stmt,
	name string,
	object types.Object,
) ([]ast.Expr, bool) {
	switch typed := statement.(type) {
	case *ast.AssignStmt:
		for index, left := range typed.Lhs {
			target, ok := s7ARUnwrapCallExpression(left).(*ast.Ident)
			if !ok || !s7ARSameTypedIdentifier(model, target, name, object) {
				continue
			}
			right, resolved := s7ARAssignmentRight(typed.Rhs, index)
			if !resolved {
				return nil, true
			}
			return []ast.Expr{right}, true
		}
	case *ast.DeclStmt:
		declaration, ok := typed.Decl.(*ast.GenDecl)
		if !ok {
			return nil, false
		}
		for _, raw := range declaration.Specs {
			spec, ok := raw.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, target := range spec.Names {
				if !s7ARSameTypedIdentifier(model, target, name, object) {
					continue
				}
				right, resolved := s7ARAssignmentRight(spec.Values, index)
				if !resolved {
					return nil, true
				}
				return []ast.Expr{right}, true
			}
		}
	}
	return nil, false
}

func s7ARStatementMayAssignReport(
	model *s6SourceTypeModel,
	statement ast.Stmt,
	name string,
	object types.Object,
) bool {
	found := false
	ast.Inspect(statement, func(node ast.Node) bool {
		if found || node == nil {
			return false
		}
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for _, left := range typed.Lhs {
				target, ok := s7ARUnwrapCallExpression(left).(*ast.Ident)
				if ok && s7ARSameTypedIdentifier(model, target, name, object) {
					found = true
					return false
				}
			}
		case *ast.ValueSpec:
			for _, target := range typed.Names {
				if s7ARSameTypedIdentifier(model, target, name, object) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

func s7ARSameTypedIdentifier(
	model *s6SourceTypeModel,
	identifier *ast.Ident,
	name string,
	object types.Object,
) bool {
	if identifier == nil || identifier.Name != name {
		return false
	}
	candidate := model.definitions[identifier]
	if candidate == nil {
		candidate = model.uses[identifier]
	}
	return object == nil || candidate == object
}

func s7ARAddressTakenValueSideEffect(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	identifier *ast.Ident,
	object types.Object,
) (string, bool) {
	if object == nil {
		return "", false
	}
	hasAddress := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if hasAddress || node == nil || node.Pos() >= identifier.Pos() {
			return false
		}
		address, ok := node.(*ast.UnaryExpr)
		if !ok || address.Op != token.AND {
			return true
		}
		target, ok := s7ARUnwrapCallExpression(address.X).(*ast.Ident)
		if !ok {
			return true
		}
		candidate := model.uses[target]
		if candidate == nil {
			candidate = model.definitions[target]
		}
		hasAddress = candidate == object
		return !hasAddress
	})
	if !hasAddress {
		return "", false
	}
	aliases := map[types.Object]bool{}
	invokedLiterals := s7ARInvokedFunctionLiterals(model, function)
	for changed := true; changed; {
		changed = false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if node == nil || node.Pos() >= identifier.Pos() {
				return node != nil
			}
			if literal, nested := node.(*ast.FuncLit); nested {
				return invokedLiterals[literal]
			}
			switch typed := node.(type) {
			case *ast.ValueSpec:
				for index, name := range typed.Names {
					right, resolved := s7ARAssignmentRight(typed.Values, index)
					if !resolved || !s7ARExpressionCarriesAddressOf(
						model, right, object, aliases,
					) {
						continue
					}
					alias := model.definitions[name]
					if alias != nil && !aliases[alias] {
						aliases[alias] = true
						changed = true
					}
				}
			case *ast.AssignStmt:
				for index, left := range typed.Lhs {
					target, ok := s7ARUnwrapCallExpression(left).(*ast.Ident)
					if !ok {
						continue
					}
					right, resolved := s7ARAssignmentRight(typed.Rhs, index)
					if !resolved || !s7ARExpressionCarriesAddressOf(
						model, right, object, aliases,
					) {
						continue
					}
					alias := model.definitions[target]
					if alias == nil {
						alias = model.uses[target]
					}
					if alias != nil && !aliases[alias] {
						aliases[alias] = true
						changed = true
					}
				}
			}
			return true
		})
	}
	var detail string
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if detail != "" || node == nil || node.Pos() >= identifier.Pos() {
			return false
		}
		if literal, nested := node.(*ast.FuncLit); nested {
			return invokedLiterals[literal]
		}
		switch typed := node.(type) {
		case *ast.CallExpr:
			expressions := append([]ast.Expr{typed.Fun}, typed.Args...)
			for _, expression := range expressions {
				if s7ARExpressionCarriesAddressOf(model, expression, object, aliases) {
					detail = s7ARNodeString(typed)
					return false
				}
			}
		case *ast.AssignStmt:
			for _, left := range typed.Lhs {
				dereference, ok := s7ARUnwrapCallExpression(left).(*ast.StarExpr)
				if ok && s7ARExpressionCarriesAddressOf(
					model, dereference.X, object, aliases,
				) {
					detail = s7ARNodeString(typed)
					return false
				}
			}
			for index, right := range typed.Rhs {
				if !s7ARExpressionCarriesAddressOf(model, right, object, aliases) {
					continue
				}
				if index >= len(typed.Lhs) {
					detail = s7ARNodeString(typed)
					return false
				}
				if _, ok := s7ARUnwrapCallExpression(typed.Lhs[index]).(*ast.Ident); !ok {
					detail = s7ARNodeString(typed)
					return false
				}
			}
		case *ast.ReturnStmt:
			for _, result := range typed.Results {
				if s7ARExpressionCarriesAddressOf(model, result, object, aliases) {
					detail = s7ARNodeString(typed)
					return false
				}
			}
		}
		return true
	})
	return detail, detail != ""
}

func s7ARExpressionCarriesAddressOf(
	model *s6SourceTypeModel,
	expression ast.Expr,
	object types.Object,
	aliases map[types.Object]bool,
) bool {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.UnaryExpr:
		if typed.Op != token.AND {
			return false
		}
		target, ok := s7ARUnwrapCallExpression(typed.X).(*ast.Ident)
		if ok {
			candidate := model.uses[target]
			if candidate == nil {
				candidate = model.definitions[target]
			}
			if candidate == object {
				return true
			}
		}
		return s7ARExpressionCarriesAddressOf(
			model, typed.X, object, aliases,
		)
	case *ast.Ident:
		candidate := model.uses[typed]
		if candidate == nil {
			candidate = model.definitions[typed]
		}
		return aliases[candidate]
	case *ast.SelectorExpr:
		return s7ARExpressionCarriesAddressOf(
			model, typed.X, object, aliases,
		)
	case *ast.IndexExpr:
		return s7ARExpressionCarriesAddressOf(
			model, typed.X, object, aliases,
		)
	case *ast.IndexListExpr:
		return s7ARExpressionCarriesAddressOf(
			model, typed.X, object, aliases,
		)
	case *ast.StarExpr:
		return s7ARExpressionCarriesAddressOf(
			model, typed.X, object, aliases,
		)
	case *ast.FuncLit:
		return s7ARFunctionLiteralCarriesAddressEffect(
			model, typed, object, aliases,
		)
	case *ast.CompositeLit:
		for _, element := range typed.Elts {
			if s7ARExpressionCarriesAddressOf(
				model, element, object, aliases,
			) {
				return true
			}
		}
		return false
	case *ast.KeyValueExpr:
		return s7ARExpressionCarriesAddressOf(
			model, typed.Value, object, aliases,
		)
	case *ast.CallExpr:
		if len(typed.Args) != 1 {
			return false
		}
		identifier, ok := s7ARUnwrapCallExpression(typed.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		if _, conversion := model.uses[identifier].(*types.TypeName); !conversion {
			return false
		}
		return s7ARExpressionCarriesAddressOf(
			model, typed.Args[0], object, aliases,
		)
	default:
		return false
	}
}

func s7ARFunctionLiteralCarriesAddressEffect(
	model *s6SourceTypeModel,
	literal *ast.FuncLit,
	object types.Object,
	aliases map[types.Object]bool,
) bool {
	found := false
	ast.Inspect(literal.Body, func(node ast.Node) bool {
		if found || node == nil {
			return false
		}
		if nested, ok := node.(*ast.FuncLit); ok && nested != literal {
			return false
		}
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for _, left := range typed.Lhs {
				if identifier, ok := s7ARUnwrapCallExpression(left).(*ast.Ident); ok {
					candidate := model.uses[identifier]
					if candidate == nil {
						candidate = model.definitions[identifier]
					}
					if candidate == object {
						found = true
						return false
					}
				}
				if s7ARExpressionCarriesAddressOf(
					model, left, object, aliases,
				) {
					found = true
					return false
				}
			}
		case *ast.CallExpr:
			for _, expression := range append([]ast.Expr{typed.Fun}, typed.Args...) {
				if s7ARExpressionCarriesAddressOf(
					model, expression, object, aliases,
				) {
					found = true
					return false
				}
			}
		case *ast.ReturnStmt:
			for _, expression := range typed.Results {
				if s7ARExpressionCarriesAddressOf(
					model, expression, object, aliases,
				) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

func s7ARResolveRefusalCodes(
	program *s6EmissionProgram,
	frame *s7ARPreAbandonFrame,
	expression ast.Expr,
	authority bool,
	authorityCodes []string,
	resolving map[string]bool,
) ([]string, bool, error) {
	expression = s7ARUnwrapCallExpression(expression)
	if typed, present := program.model.expressionTypes[expression]; present &&
		typed.Value != nil && typed.Value.Kind() == constant.String {
		return []string{constant.StringVal(typed.Value)}, true, nil
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return nil, false, nil
	}
	object := program.model.uses[identifier]
	if object == nil {
		object = program.model.definitions[identifier]
	}
	if detail, changed := s7ARAddressTakenValueSideEffect(
		program.model, frame.node.function, identifier, object,
	); changed {
		return nil, false, fmt.Errorf(
			"%s:%s address-taken refusal value %s has unresolved side effects at %s",
			frame.node.rel, frame.node.function.Name.Name,
			identifier.Name, detail,
		)
	}
	key := frame.node.key + ":" + identifier.Name
	if resolving[key] {
		return nil, false, fmt.Errorf("recursive refusal code value %s", key)
	}
	resolving[key] = true
	defer delete(resolving, key)
	candidates, dominates := s7ARDominatingReportBindings(
		program.model, frame.node.function, identifier, object,
	)
	if !dominates {
		candidates = s6FunctionBindingsBefore(
			frame.node.function, expression.Pos(),
		).candidates(identifier)
	}
	var result []string
	for _, candidate := range candidates {
		values, candidateResolved, err := s7ARResolveRefusalCodes(
			program, frame, candidate, authority, authorityCodes, resolving,
		)
		if err != nil {
			return nil, false, err
		}
		if !candidateResolved || len(values) == 0 {
			if authority && identifier.Name == "code" &&
				s7ARAuthorityCodeSource(candidate) {
				values = append([]string(nil), authorityCodes...)
			} else {
				return nil, false, fmt.Errorf(
					"%s:%s refusal value %s has an unresolved alternative %s",
					frame.node.rel, frame.node.function.Name.Name,
					identifier.Name, s7ARNodeString(candidate),
				)
			}
		}
		result = append(result, values...)
	}
	if dominates {
		if len(result) == 0 {
			return nil, false, fmt.Errorf(
				"%s:%s refusal value %s has no dominating alternative",
				frame.node.rel, frame.node.function.Name.Name, identifier.Name,
			)
		}
		return result, true, nil
	}
	if bound, present := frame.parameters[object]; present {
		values, boundResolved, err := s7ARResolveRefusalCodes(
			program, bound.frame, bound.expression, authority, authorityCodes, resolving,
		)
		if err != nil {
			return nil, false, err
		}
		if !boundResolved || len(values) == 0 {
			return nil, false, fmt.Errorf(
				"%s:%s caller value for %s has an unresolved alternative",
				frame.node.rel, frame.node.function.Name.Name, identifier.Name,
			)
		}
		result = append(result, values...)
	}
	if len(result) != 0 {
		return result, true, nil
	}
	if authority && identifier.Name == "code" {
		return append([]string(nil), authorityCodes...), true, nil
	}
	return nil, false, nil
}

func s7ARAuthorityCodeSource(expression ast.Expr) bool {
	call, ok := s7ARUnwrapCallExpression(expression).(*ast.CallExpr)
	return ok && s6CallName(call) == "prepareAuthorityError"
}

func s7ARReturnExpressions(function *ast.FuncDecl) []ast.Expr {
	var result []ast.Expr
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok {
			_ = literal
			return false
		}
		returnStatement, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		if len(returnStatement.Results) == 1 {
			result = append(result, returnStatement.Results[0])
		}
		return false
	})
	sort.Slice(result, func(left, right int) bool {
		return result[left].Pos() < result[right].Pos()
	})
	return result
}

func s7ARScanPreAbandonCalls(
	program *s6EmissionProgram,
	frame *s7ARPreAbandonFrame,
	statements []ast.Stmt,
	abandon *s6EmissionFunction,
	abandonPrefix []ast.Stmt,
	authorityCodes []string,
	active map[string]bool,
	scopeKey string,
	evidence *s7ARAbandonFlowEvidence,
) error {
	if active[scopeKey] {
		return fmt.Errorf("reachable recursive pre-abandon call graph at %s", scopeKey)
	}
	active[scopeKey] = true
	defer delete(active, scopeKey)
	for _, statement := range statements {
		var calls []*ast.CallExpr
		ast.Inspect(statement, func(current ast.Node) bool {
			if current == nil {
				return true
			}
			if _, ok := current.(*ast.FuncLit); ok {
				return false
			}
			call, ok := current.(*ast.CallExpr)
			if ok {
				calls = append(calls, call)
			}
			return true
		})
		sort.SliceStable(calls, func(left, right int) bool {
			return calls[left].End() < calls[right].End()
		})
		for _, call := range calls {
			if err := s7ARScanPreAbandonCall(
				program, frame, call, abandon, abandonPrefix,
				authorityCodes, active, evidence,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func s7ARScanPreAbandonCall(
	program *s6EmissionProgram,
	frame *s7ARPreAbandonFrame,
	call *ast.CallExpr,
	abandon *s6EmissionFunction,
	abandonPrefix []ast.Stmt,
	authorityCodes []string,
	active map[string]bool,
	evidence *s7ARAbandonFlowEvidence,
) error {
	node := frame.node
	if s7ARPrebranchBuiltinOrConversion(program, call) {
		return nil
	}
	if literal, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.FuncLit); ok {
		return s7ARScanPreAbandonCalls(
			program, frame, literal.Body.List,
			abandon, abandonPrefix, authorityCodes, active,
			fmt.Sprintf("%s:literal:%d", node.key, literal.Pos()), evidence,
		)
	}
	identities, unresolved := s7ARPreAbandonCallIdentities(program, node, call)
	if len(identities) == 0 {
		if identifier, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident); ok {
			resolvedLiteral := false
			for _, binding := range s6FunctionBindingsBefore(
				node.function, call.Pos(),
			).candidates(identifier) {
				literal, ok := s7ARUnwrapCallExpression(binding).(*ast.FuncLit)
				if !ok {
					continue
				}
				resolvedLiteral = true
				if err := s7ARScanPreAbandonCalls(
					program, frame, literal.Body.List,
					abandon, abandonPrefix, authorityCodes, active,
					fmt.Sprintf("%s:literal:%d", node.key, literal.Pos()), evidence,
				); err != nil {
					return err
				}
			}
			if resolvedLiteral {
				return nil
			}
		}
	}
	if unresolved &&
		!s7ARAllowedUnresolvedPrebranchCall(program, node, call) &&
		!s7ARAllowedPartiallyResolvedPrebranchCall(call, identities) &&
		!s7ARAllPrebranchSinkIdentities(identities) &&
		!s7ARDirectImportedCallable(node, call, identities) {
		return fmt.Errorf("%s:%s call %s is unresolved",
			node.rel, node.function.Name.Name, s6CallName(call))
	}
	if len(identities) == 0 {
		if s7ARAllowedUnresolvedPrebranchCall(program, node, call) {
			return nil
		}
		return fmt.Errorf("%s:%s call %s has no typed callable identity",
			node.rel, node.function.Name.Name, s6CallName(call))
	}
	for _, identity := range identities {
		callKey := identity.functionKey()
		if callKey == "internal/cli.emitPreparePublishReport" {
			if len(call.Args) != 3 {
				return fmt.Errorf("%s:%s emitPreparePublishReport arity = %d",
					node.rel, node.function.Name.Name, len(call.Args))
			}
			exitLiteral, ok := s7ARUnwrapCallExpression(call.Args[2]).(*ast.BasicLit)
			if !ok || exitLiteral.Kind != token.INT {
				return fmt.Errorf("%s:%s refusal exit is not a literal integer",
					node.rel, node.function.Name.Name)
			}
			exit, err := strconv.Atoi(exitLiteral.Value)
			if err != nil {
				return err
			}
			alternatives, err := s7ARRefusalAlternativesFromReport(
				program, frame, call.Args[1], exit, authorityCodes, map[string]bool{},
			)
			if err != nil {
				return fmt.Errorf("%s:%s resolve emitted report: %w",
					node.rel, node.function.Name.Name, err)
			}
			if len(alternatives) == 0 {
				return fmt.Errorf(
					"%s:%s reachable report emitter has no report-value alternative",
					node.rel, node.function.Name.Name,
				)
			}
			for index, alternative := range alternatives {
				if alternative.site.code == "" ||
					alternative.site.exit != exit ||
					(alternative.site.route != "standard" &&
						alternative.site.route != "authority") {
					return fmt.Errorf(
						"%s:%s report alternative %d is not exactly one permitted refusal: %+v",
						node.rel, node.function.Name.Name, index+1, alternative,
					)
				}
				evidence.stops = append(evidence.stops, alternative.site)
			}
			continue
		}
		switch {
		case identity.literal != nil:
			if err := s7ARScanPreAbandonCalls(
				program, frame, identity.literal.Body.List,
				abandon, abandonPrefix, authorityCodes, active,
				fmt.Sprintf("%s:literal:%d", node.key, identity.literal.Pos()), evidence,
			); err != nil {
				return err
			}
			continue
		case identity.pkg == "internal/gitutil" ||
			strings.HasSuffix(identity.pkg, "/internal/gitutil"):
			evidence.gitCalls[callKey] = true
			continue
		case identity.pkg == "os" && s7AROSReadCall(identity):
			if !s7ARAllowedOSControlRead(node, identity) {
				evidence.featureReads[callKey] = true
			}
			continue
		case identity.pkg == "os/exec" &&
			(identity.name == "Command" || identity.name == "CommandContext"):
			evidence.gitCalls[callKey] = true
			continue
		case callKey == "internal/cli.inspectPrepareReadState" ||
			callKey == "internal/cli.inspectPrepareWithAuthority" ||
			callKey == "internal/intent.Inspect" ||
			callKey == "internal/cli.capturePrepareFileWithScratch":
			evidence.featureReads[callKey] = true
			continue
		case callKey == "internal/intentpub.Recover" ||
			callKey == "internal/cli.preparePendingArchiveHashes" ||
			callKey == "internal/cli.prepareLoadProvider" ||
			callKey == "internal/cli.runPreparePublishTransaction":
			evidence.stepSixCalls[callKey] = true
			continue
		}
		if callKey == "internal/cli.runPrepareAbandon" {
			abandonFrame, err := s7ARBindPreAbandonCall(program, frame, call, abandon)
			if err != nil {
				return err
			}
			if err := s7ARScanPreAbandonCalls(
				program, abandonFrame, abandonPrefix, abandon, abandonPrefix,
				authorityCodes, active, abandon.key+":prefix", evidence,
			); err != nil {
				return err
			}
			continue
		}
		targets := program.byBase[callKey]
		if identity.pkg != "internal/cli" {
			if !s7ARAllowedTypedPrebranchCall(node, identity) {
				return fmt.Errorf(
					"%s:%s reachable call %s is not in the typed prebranch allowlist",
					node.rel, node.function.Name.Name, callKey,
				)
			}
			continue
		}
		if len(targets) == 0 {
			return fmt.Errorf(
				"%s:%s internal callable %s has no source body",
				node.rel, node.function.Name.Name, callKey,
			)
		}
		for _, target := range targets {
			callee := program.functions[target]
			calleeFrame, err := s7ARBindPreAbandonCall(program, frame, call, callee)
			if err != nil {
				return err
			}
			if err := s7ARScanPreAbandonCalls(
				program, calleeFrame, callee.function.Body.List,
				abandon, abandonPrefix, authorityCodes, active, callee.key, evidence,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func s7ARPreAbandonCallIdentities(
	program *s6EmissionProgram,
	node *s6EmissionFunction,
	call *ast.CallExpr,
) ([]s6CallIdentity, bool) {
	identities, unresolved := program.resolveCallableIdentities(
		node, call, map[*ast.Object]bool{}, map[string]bool{},
	)
	if unresolved || len(identities) == 0 {
		fallback, complete := s7ARResolveBoundCallableIdentities(program, node, call)
		if len(fallback) != 0 {
			identities = s6UniqueCallIdentities(append(identities, fallback...))
		}
		if complete {
			unresolved = false
		}
	}
	if len(identities) == 0 {
		if identity, ok := s7ARTypedCallableIdentity(program, call); ok {
			identities = []s6CallIdentity{identity}
			unresolved = false
		}
	}
	return identities, unresolved
}

func s7ARBindPreAbandonCall(
	program *s6EmissionProgram,
	caller *s7ARPreAbandonFrame,
	call *ast.CallExpr,
	callee *s6EmissionFunction,
) (*s7ARPreAbandonFrame, error) {
	parameters := map[types.Object]s7ARBoundExpression{}
	argument := 0
	if callee.function.Type.Params != nil {
		for _, field := range callee.function.Type.Params.List {
			for _, name := range field.Names {
				if argument >= len(call.Args) {
					return nil, fmt.Errorf(
						"%s call has %d arguments for parameter %s",
						callee.baseKey, len(call.Args), name.Name,
					)
				}
				object := program.model.definitions[name]
				if object == nil {
					return nil, fmt.Errorf(
						"%s parameter %s has no typed object", callee.baseKey, name.Name,
					)
				}
				parameters[object] = s7ARBoundExpression{
					frame: caller, expression: call.Args[argument],
				}
				argument++
			}
		}
	}
	return &s7ARPreAbandonFrame{node: callee, parameters: parameters}, nil
}

func s7ARUnwrapCallExpression(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

func s7ARPrebranchBuiltinOrConversion(
	program *s6EmissionProgram,
	call *ast.CallExpr,
) bool {
	expression := s7ARUnwrapCallExpression(call.Fun)
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false
	}
	object := program.model.uses[identifier]
	if object == nil {
		object = program.model.definitions[identifier]
	}
	switch object.(type) {
	case *types.Builtin, *types.TypeName:
		return true
	}
	return false
}

func s7ARAllowedUnresolvedPrebranchCall(
	program *s6EmissionProgram,
	node *s6EmissionFunction,
	call *ast.CallExpr,
) bool {
	if node.pkg != "internal/cli" {
		return false
	}
	identifier, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	object, ok := program.model.uses[identifier].(*types.Var)
	if !ok {
		object, ok = program.model.definitions[identifier].(*types.Var)
	}
	if !ok {
		return false
	}
	signature, ok := object.Type().Underlying().(*types.Signature)
	if !ok {
		return false
	}
	switch identifier.Name {
	case "beforeLockAcquire", "beforeAbandonBranch":
		return signature.Params().Len() == 0 && signature.Results().Len() == 0
	case "prepareMutationAuthoritySupported":
		return signature.Params().Len() == 0 && signature.Results().Len() == 1 &&
			types.Identical(signature.Results().At(0).Type(), types.Typ[types.Bool])
	case "prepareAcquireAuthority":
		return signature.Params().Len() == 1 && signature.Results().Len() == 2
	default:
		return false
	}
}

func s7ARAllowedPartiallyResolvedPrebranchCall(
	call *ast.CallExpr,
	identities []s6CallIdentity,
) bool {
	if len(identities) != 0 {
		allLiterals := true
		for _, identity := range identities {
			if identity.literal == nil {
				allLiterals = false
				break
			}
		}
		if allLiterals {
			return true
		}
	}
	identifier, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident)
	if !ok || identifier.Name != "prepareAcquireAuthority" || len(identities) != 1 {
		return false
	}
	return identities[0].functionKey() == "internal/intentlock.Acquire"
}

func s7ARResolveBoundCallableIdentities(
	program *s6EmissionProgram,
	node *s6EmissionFunction,
	call *ast.CallExpr,
) ([]s6CallIdentity, bool) {
	bindings := s6FunctionBindingsBefore(node.function, call.Pos())
	var candidates []ast.Expr
	switch callable := s7ARUnwrapCallExpression(call.Fun).(type) {
	case *ast.Ident:
		candidates = bindings.candidates(callable)
	case *ast.SelectorExpr:
		if base, ok := s7ARUnwrapCallExpression(callable.X).(*ast.Ident); ok {
			candidates = bindings.fieldCandidates(base, callable.Sel.Name)
			for _, binding := range bindings.candidates(base) {
				composite, ok := s7ARUnwrapCallExpression(binding).(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, element := range composite.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					identifier, ok := field.Key.(*ast.Ident)
					if ok && identifier.Name == callable.Sel.Name {
						candidates = append(candidates, field.Value)
					}
				}
			}
		}
	}
	if len(candidates) == 0 {
		return nil, false
	}
	var identities []s6CallIdentity
	for _, candidate := range candidates {
		candidateCall := &ast.CallExpr{Fun: candidate}
		if identity, ok := s7ARTypedCallableIdentity(program, candidateCall); ok {
			identities = append(identities, identity)
			continue
		}
		resolved, unresolved := program.resolveCallableIdentities(
			node, candidate, map[*ast.Object]bool{}, map[string]bool{},
		)
		if unresolved || len(resolved) == 0 {
			return s6UniqueCallIdentities(identities), false
		}
		identities = append(identities, resolved...)
	}
	return s6UniqueCallIdentities(identities), true
}

func s7ARAllPrebranchSinkIdentities(identities []s6CallIdentity) bool {
	if len(identities) == 0 {
		return false
	}
	for _, identity := range identities {
		callKey := identity.functionKey()
		if identity.pkg == "internal/gitutil" ||
			strings.HasSuffix(identity.pkg, "/internal/gitutil") ||
			identity.pkg == "os" && s7AROSReadCall(identity) ||
			identity.pkg == "os/exec" &&
				(identity.name == "Command" || identity.name == "CommandContext") ||
			callKey == "internal/cli.inspectPrepareReadState" ||
			callKey == "internal/cli.inspectPrepareWithAuthority" ||
			callKey == "internal/intent.Inspect" ||
			callKey == "internal/cli.capturePrepareFileWithScratch" ||
			callKey == "internal/intentpub.Recover" ||
			callKey == "internal/cli.preparePendingArchiveHashes" ||
			callKey == "internal/cli.prepareLoadProvider" ||
			callKey == "internal/cli.runPreparePublishTransaction" {
			continue
		}
		return false
	}
	return true
}

func s7ARDirectImportedCallable(
	node *s6EmissionFunction,
	call *ast.CallExpr,
	identities []s6CallIdentity,
) bool {
	selector, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.SelectorExpr)
	if !ok || len(identities) != 1 {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && node.imports[qualifier.Name] != ""
}

func s7ARTypedCallableIdentity(
	program *s6EmissionProgram,
	call *ast.CallExpr,
) (s6CallIdentity, bool) {
	expression := s7ARUnwrapCallExpression(call.Fun)
	var identifier *ast.Ident
	switch typed := expression.(type) {
	case *ast.Ident:
		identifier = typed
	case *ast.SelectorExpr:
		identifier = typed.Sel
	default:
		return s6CallIdentity{}, false
	}
	object, ok := program.model.uses[identifier].(*types.Func)
	if !ok || object.Pkg() == nil {
		return s6CallIdentity{}, false
	}
	pkg := strings.TrimPrefix(
		object.Pkg().Path(),
		"github.com/tesseracode/tesserapatch/",
	)
	identity := s6CallIdentity{pkg: pkg, name: object.Name(), known: true}
	if signature, ok := object.Type().(*types.Signature); ok && signature.Recv() != nil {
		identity.receiver = s7ARTypeName(signature.Recv().Type())
	}
	return identity, true
}

func s7ARTypedCallMatches(
	program *s6EmissionProgram,
	node *s6EmissionFunction,
	call *ast.CallExpr,
	want string,
) bool {
	identities, _ := program.resolveCallableIdentities(
		node, call, map[*ast.Object]bool{}, map[string]bool{},
	)
	if len(identities) == 0 {
		if identity, ok := s7ARTypedCallableIdentity(program, call); ok {
			identities = []s6CallIdentity{identity}
		}
	}
	return len(identities) == 1 && identities[0].functionKey() == want
}

func s7ARTypeName(value types.Type) string {
	switch typed := value.(type) {
	case *types.Pointer:
		return "*" + s7ARTypeName(typed.Elem())
	case *types.Named:
		return typed.Obj().Name()
	default:
		return types.TypeString(value, func(*types.Package) string { return "" })
	}
}

func s7AROSReadCall(identity s6CallIdentity) bool {
	switch identity.name {
	case "Open", "OpenFile", "OpenRoot", "ReadFile", "ReadDir", "Stat", "Lstat":
		return true
	default:
		return false
	}
}

func s7ARAllowedOSControlRead(
	node *s6EmissionFunction,
	identity s6CallIdentity,
) bool {
	if node.baseKey != "internal/cli.prepareLaneHasPendingEvidence" {
		return false
	}
	switch identity.functionKey() {
	case "os.OpenRoot",
		"os.Root.Lstat", "os.*Root.Lstat",
		"os.Root.Open", "os.*Root.Open":
		return true
	default:
		return false
	}
}

func s7ARAllowedTypedPrebranchCall(
	node *s6EmissionFunction,
	identity s6CallIdentity,
) bool {
	callKey := identity.functionKey()
	switch callKey {
	case "internal/intent.CanonicalSlug",
		"internal/intent.RootConfinementSupported",
		"internal/store.FindProjectRoot",
		"internal/intentlock.Acquire":
		return true
	case "os.OpenRoot", "os.Root.Lstat", "os.*Root.Lstat":
		return node.baseKey == "internal/cli.prepareLaneHasPendingEvidence"
	case "os.Root.Open", "os.*Root.Open",
		"os.Root.Close", "os.*Root.Close",
		"os.File.Readdirnames", "os.*File.Readdirnames",
		"os.File.Close", "os.*File.Close":
		return node.baseKey == "internal/cli.prepareLaneHasPendingEvidence"
	case "io/fs.FileInfo.Mode", "io/fs.FileInfo.IsDir":
		return node.baseKey == "internal/cli.prepareRefusedInfo" ||
			node.baseKey == "internal/cli.prepareLaneHasPendingEvidence"
	case "errors.As", "errors.Is",
		"fmt.Fprint", "fmt.Fprintln", "fmt.Fprintf", "fmt.Sprintf",
		"encoding/json.NewEncoder",
		"encoding/json.Encoder.Encode", "encoding/json.*Encoder.Encode",
		"encoding/json.Encoder.SetIndent", "encoding/json.*Encoder.SetIndent",
		"strings.HasPrefix", "strings.Join", "strings.Split", "strings.TrimPrefix":
		return true
	case "github.com/spf13/cobra.Command.Flags",
		"github.com/spf13/cobra.Command.OutOrStdout",
		"github.com/spf13/cobra.Command.ErrOrStderr",
		"github.com/spf13/cobra.*Command.Flags",
		"github.com/spf13/cobra.*Command.OutOrStdout",
		"github.com/spf13/cobra.*Command.ErrOrStderr",
		"github.com/spf13/pflag.FlagSet.GetBool",
		"github.com/spf13/pflag.*FlagSet.GetBool",
		"github.com/spf13/pflag.FlagSet.GetString",
		"github.com/spf13/pflag.*FlagSet.GetString":
		return true
	case "internal/intentlock.WorkspaceAuthority.Release",
		"internal/intentlock.*WorkspaceAuthority.Release":
		return true
	default:
		return false
	}
}

func s7ARNodeString(node ast.Node) string {
	var output bytes.Buffer
	_ = printer.Fprint(&output, token.NewFileSet(), node)
	return output.String()
}

func s7ARReplaceAfter(source, scope, old, replacement string) (string, bool) {
	scopeAt := strings.Index(source, scope)
	if scopeAt < 0 {
		return source, false
	}
	tail := source[scopeAt:]
	mutated := strings.Replace(tail, old, replacement, 1)
	if mutated == tail {
		return source, false
	}
	return source[:scopeAt] + mutated, true
}

func s7ARSortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func TestS7ARPurgeProgressGuard(t *testing.T) {
	t.Run("PIB-518", func(t *testing.T) {
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		cliSource := sources["internal/cli/feature_intent_archive.go"]
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		observations := s7ARProductionPurgeProgressReports(resumes)
		model, err := s6BuildSourceTypeModel(s6EmissionTypeSources(s7ARCLIPackageSources(sources)))
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, sources, model,
		); err != nil {
			t.Fatal(err)
		}
		for _, fixture := range []struct {
			name   string
			mutate func([]s7ARPurgeProgressObservation)
		}{
			{
				name: "completion-promises-recovered",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					for index := range reports {
						if reports[index].report.PurgeProgress.Resume == string(store.IntentArchiveResumeCompletionOnly) {
							reports[index].instruction = "The first retry reports recovered. A second retry completes the remaining hashes."
							reports[index].human = s7ARRenderPartialHuman(
								reports[index].report, reports[index].instruction,
							)
						}
					}
				},
			},
			{
				name: "orphan-promises-recovered",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					for index := range reports {
						if reports[index].report.PurgeProgress.Resume == string(store.IntentArchiveResumeOrphanScan) {
							reports[index].instruction = "The first retry reports recovered. A second retry removes the remaining orphan blobs."
							reports[index].human = s7ARRenderPartialHuman(
								reports[index].report, reports[index].instruction,
							)
						}
					}
				},
			},
			{
				name: "pending-promises-one-run",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					for index := range reports {
						if reports[index].report.PurgeProgress.Resume == string(store.IntentArchiveResumePendingRecoveryThenCompletion) {
							reports[index].instruction = "Exactly one retry completes all remaining work without a recovered outcome."
							reports[index].human = s7ARRenderPartialHuman(
								reports[index].report, reports[index].instruction,
							)
						}
					}
				},
			},
			{
				name: "pending-omits-hash",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					for index := range reports {
						if reports[index].report.PurgeProgress.Resume == string(store.IntentArchiveResumePendingRecoveryThenCompletion) {
							reports[index].report.PurgeProgress.PendingHash = ""
							reports[index].human = s7ARRenderPartialHuman(
								reports[index].report, reports[index].instruction,
							)
						}
					}
				},
			},
			{
				name: "completion-adds-pending-hash",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					for index := range reports {
						if reports[index].report.PurgeProgress.Resume == string(store.IntentArchiveResumeCompletionOnly) {
							reports[index].report.PurgeProgress.PendingHash = strings.Repeat("a", 64)
							reports[index].human = s7ARRenderPartialHuman(
								reports[index].report, reports[index].instruction,
							)
						}
					}
				},
			},
			{
				name: "wrong-outcome",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					reports[0].report.Outcome = string(store.IntentArchivePurgePurged)
					reports[0].human = s7ARRenderPartialHuman(
						reports[0].report, reports[0].instruction,
					)
				},
			},
			{
				name: "wrong-state",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					reports[0].report.PurgeProgress.State = "index-decodes-but-state-is-unknown"
					reports[0].human = s7ARRenderPartialHuman(
						reports[0].report, reports[0].instruction,
					)
				},
			},
			{
				name: "wrong-hash-identity",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					reports[0].report.PurgeProgress.CompletedHashes =
						[]string{strings.Repeat("d", 64)}
					reports[0].human = s7ARRenderPartialHuman(
						reports[0].report, reports[0].instruction,
					)
				},
			},
			{
				name: "wrong-retry-argv",
				mutate: func(reports []s7ARPurgeProgressObservation) {
					reports[0].report.PurgeProgress.Retry =
						"tpatch feature intent-archive purge ar-purge-progress --orphans --yes --json"
					reports[0].human = s7ARRenderPartialHuman(
						reports[0].report, reports[0].instruction,
					)
				},
			},
		} {
			cloned := s7ARClonePurgeProgressObservations(observations)
			fixture.mutate(cloned)
			if err := validateS7ARPurgeProgressWithModel(
				cloned, sources, model,
			); err == nil {
				t.Fatalf("PIB-518 same validator accepted %s", fixture.name)
			}
		}
		for _, fixture := range []struct {
			name             string
			old              string
			new              string
			extraRel         string
			extraSource      string
			wantErr          string
			wantActualWrites int
		}{
			{
				name: "duplicate-builder-emitter",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			},
			{
				name: "direct-composite-emitter",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new:  "report.PurgeProgress = &intentArchivePurgeProgressReport{}",
			},
			{
				name: "enclosing-composite-field",
				old:  "report := intentArchivePurgeReport{",
				new:  "report := intentArchivePurgeReport{PurgeProgress: buildIntentArchivePurgeProgress(store.IntentArchivePurgeResult{}, slug, options),",
			},
			{
				name: "pointer-alias-write",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tslot := &report.PurgeProgress\n" +
					"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				wantErr: "purge_progress inventory",
			},
			{
				name: "promoted-embedded-pointer-field-write",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\twrapped := struct{ *intentArchivePurgeReport }{&report}\n" +
					"\t\twrapped.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				wantErr: "purge_progress inventory",
			},
			{
				name: "invoked-parenthesized-closure-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t(func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t})()",
				wantErr: "purge_progress inventory",
			},
			{
				name: "invoked-closure-captured-alias-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tslot := &report.PurgeProgress\n" +
					"\t\tinvoke := func() {\n" +
					"\t\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\t(invoke)()",
				wantErr: "purge_progress inventory",
			},
			{
				name: "callable-parameter-forwarded-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tinvoke := func(fn func()) { fn() }\n" +
					"\t\tinvoke(func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t})",
				wantErr: "purge_progress inventory",
			},
			{
				name: "value-receiver-method-expression-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\ts7ARValueCallbackInvoker.invoke(s7ARValueCallbackInvoker{}, callback)",
				extraRel: "internal/cli/zz_s7_ar_progress_method_expression.go",
				extraSource: "package cli\n\n" +
					"type s7ARValueCallbackInvoker struct{}\n\n" +
					"func (s7ARValueCallbackInvoker) invoke(callback func()) { callback() }\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "pointer-receiver-method-expression-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\t(*s7ARPointerCallbackInvoker).invoke(&s7ARPointerCallbackInvoker{}, callback)",
				extraRel: "internal/cli/zz_s7_ar_progress_method_expression.go",
				extraSource: "package cli\n\n" +
					"type s7ARPointerCallbackInvoker struct{}\n\n" +
					"func (*s7ARPointerCallbackInvoker) invoke(callback func()) { callback() }\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "method-value-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\tinvoker := s7ARMethodValueCallbackInvoker{}\n" +
					"\t\tinvoker.invoke(callback)",
				extraRel: "internal/cli/zz_s7_ar_progress_method_value.go",
				extraSource: "package cli\n\n" +
					"type s7ARMethodValueCallbackInvoker struct{}\n\n" +
					"func (s7ARMethodValueCallbackInvoker) invoke(callback func()) { callback() }\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "value-receiver-method-expression-held-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\treceiver := s7ARValueReceiverOriginInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\ts7ARValueReceiverOriginInvoker.invoke(receiver, func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_receiver_origin.go",
				extraSource: "package cli\n\n" +
					"type s7ARValueReceiverOriginInvoker struct{ before func() }\n\n" +
					"func (receiver s7ARValueReceiverOriginInvoker) invoke(callback func()) {\n" +
					"\treceiver.before()\n" +
					"\tcallback()\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "pointer-receiver-method-expression-held-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\treceiver := &s7ARPointerReceiverOriginInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\talias := receiver\n" +
					"\t\t(*s7ARPointerReceiverOriginInvoker).invoke(alias, func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_receiver_origin.go",
				extraSource: "package cli\n\n" +
					"type s7ARPointerReceiverOriginInvoker struct{ before func() }\n\n" +
					"func (receiver *s7ARPointerReceiverOriginInvoker) invoke(callback func()) {\n" +
					"\treceiver.before()\n" +
					"\tcallback()\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "method-value-receiver-held-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\treceiver := s7ARMethodValueReceiverOriginInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\treceiver.invoke(func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_receiver_origin.go",
				extraSource: "package cli\n\n" +
					"type s7ARMethodValueReceiverOriginInvoker struct{ before func() }\n\n" +
					"func (receiver s7ARMethodValueReceiverOriginInvoker) invoke(callback func()) {\n" +
					"\treceiver.before()\n" +
					"\tcallback()\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "rev13-method-value-transport-matrix",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tpointerReceiver := &s7ARMatrixPointerInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tpointerAlias := pointerReceiver\n" +
					"\t\tpointerAddress := &pointerAlias\n" +
					"\t\tpointerInvoke := (*pointerAddress).invoke\n" +
					"\t\tpointerInvoke(func() {})\n" +
					"\t\tpassedReceiver := s7ARMatrixPassedInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\ts7ARMatrixInvokePassed(passedReceiver.invoke)\n" +
					"\t\treturnedReceiver := s7ARMatrixReturnedInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\ts7ARMatrixReturnNamed(returnedReceiver)(func() {})\n" +
					"\t\tinterfaceReceiver := s7ARMatrixInterfaceInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar dispatched s7ARMatrixInterface = interfaceReceiver\n" +
					"\t\tinterfaceInvoke := dispatched.invoke\n" +
					"\t\tinterfaceInvoke(func() {})\n" +
					"\t\texpressionReceiver := s7ARMatrixExpressionInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\texpressionInvoke := s7ARMatrixExpressionInvoker.invoke\n" +
					"\t\texpressionInvoke(expressionReceiver, func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_method_value_matrix.go",
				extraSource: "package cli\n\n" +
					"type s7ARMatrixPointerInvoker struct{ before func() }\n" +
					"type s7ARMatrixPassedInvoker struct{ before func() }\n" +
					"type s7ARMatrixReturnedInvoker struct{ before func() }\n" +
					"type s7ARMatrixInterface interface{ invoke(func()) }\n" +
					"type s7ARMatrixInterfaceInvoker struct{ before func() }\n" +
					"type s7ARMatrixExpressionInvoker struct{ before func() }\n\n" +
					"func (receiver *s7ARMatrixPointerInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver s7ARMatrixPassedInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver s7ARMatrixReturnedInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver s7ARMatrixInterfaceInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver s7ARMatrixExpressionInvoker) invoke(callback func()) { receiver.before(); callback() }\n\n" +
					"func s7ARMatrixInvokePassed(invoke func(func())) { alias := invoke; alias(func() {}) }\n" +
					"func s7ARMatrixReturnNamed(receiver s7ARMatrixReturnedInvoker) (invoke func(func())) {\n" +
					"\tlocal := receiver.invoke\n" +
					"\tinvoke = local\n" +
					"\treturn\n" +
					"}\n",
				wantErr:          "purge_progress inventory",
				wantActualWrites: 8,
			},
			{
				name: "rev14-direct-interface-dispatch-matrix",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvalueReceiver := s7ARRev14ValueInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar valueDispatched s7ARRev14DirectInterface = valueReceiver\n" +
					"\t\tvalueDispatched.invoke(func() {})\n" +
					"\t\tpointerReceiver := &s7ARRev14PointerInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar pointerDispatched s7ARRev14DirectInterface = pointerReceiver\n" +
					"\t\tpointerDispatched.invoke(func() {})\n" +
					"\t\tpassedReceiver := s7ARRev14ValueInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\ts7ARRev14InvokePassedInterface(passedReceiver)\n" +
					"\t\treturnedReceiver := s7ARRev14ValueInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\ts7ARRev14ReturnInterface(returnedReceiver).invoke(func() {})\n" +
					"\t\texpressionReceiver := s7ARRev14ValueInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar expressionDispatched s7ARRev14DirectInterface = expressionReceiver\n" +
					"\t\ts7ARRev14DirectInterface.invoke(expressionDispatched, func() {})\n" +
					"\t\tvariadicReceiver := &s7ARRev15MatrixPointerInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar variadicDispatched s7ARRev15MatrixInterface = variadicReceiver\n" +
					"\t\ts7ARRev15MatrixInterface.invoke(variadicDispatched, func() {})\n" +
					"\t\tfixedTailReceiver := &s7ARRev16FixedTailInvoker{}\n" +
					"\t\tvar fixedTailDispatched s7ARRev16FixedTailInterface = fixedTailReceiver\n" +
					"\t\ts7ARRev16FixedTailInterface.invoke(fixedTailDispatched, \"run\", func() {}, func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t})\n" +
					"\t\tfixedCallbackReceiver := &s7ARRev16FixedCallbackInvoker{}\n" +
					"\t\tvar fixedCallbackDispatched s7ARRev16FixedCallbackInterface = fixedCallbackReceiver\n" +
					"\t\ts7ARRev16FixedCallbackInterface.invoke(fixedCallbackDispatched, func() {}, func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}, func() {}, func() {})\n" +
					"\t\texpandedReceiver := &s7ARRev16FixedTailInvoker{}\n" +
					"\t\tvar expandedDispatched s7ARRev16FixedTailInterface = expandedReceiver\n" +
					"\t\texpandedDispatched.invoke(\"run\", s7ARRev16KnownCallbacks(func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t})...)",
				extraRel: "internal/cli/zz_s7_ar_progress_direct_interface.go",
				extraSource: "package cli\n\n" +
					"type s7ARRev14DirectInterface interface{ invoke(func()) }\n" +
					"type s7ARRev14ValueInvoker struct{ before func() }\n" +
					"type s7ARRev14PointerInvoker struct{ before func() }\n" +
					"type s7ARRev15MatrixCallback func()\n" +
					"type s7ARRev15MatrixInterface interface{ invoke(...s7ARRev15MatrixCallback) }\n" +
					"type s7ARRev15MatrixPointerInvoker struct{ before func() }\n\n" +
					"type s7ARRev16FixedTailCallback func()\n" +
					"type s7ARRev16FixedTailCallbacks []s7ARRev16FixedTailCallback\n" +
					"type s7ARRev16FixedTailInterface interface{ invoke(string, ...s7ARRev16FixedTailCallback) }\n" +
					"type s7ARRev16FixedTailInvoker struct{}\n\n" +
					"type s7ARRev16FixedCallbackInterface interface{ invoke(func(), func(), ...s7ARRev16FixedTailCallback) }\n" +
					"type s7ARRev16FixedCallbackInvoker struct{}\n\n" +
					"func (receiver s7ARRev14ValueInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver *s7ARRev14PointerInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver *s7ARRev15MatrixPointerInvoker) invoke(callbacks ...s7ARRev15MatrixCallback) {\n" +
					"\treceiver.before()\n" +
					"\tfor _, callback := range callbacks { callback() }\n" +
					"}\n" +
					"func s7ARRev14InvokePassedInterface(receiver s7ARRev14DirectInterface) {\n" +
					"\treceiver.invoke(func() {})\n" +
					"}\n" +
					"func s7ARRev14ReturnInterface(receiver s7ARRev14DirectInterface) s7ARRev14DirectInterface {\n" +
					"\treturn receiver\n" +
					"}\n" +
					"func (*s7ARRev16FixedTailInvoker) invoke(prefix string, callbacks ...s7ARRev16FixedTailCallback) {\n" +
					"\tif prefix == \"run\" { callbacks[1]() }\n" +
					"}\n" +
					"func (*s7ARRev16FixedCallbackInvoker) invoke(first, second func(), callbacks ...s7ARRev16FixedTailCallback) {\n" +
					"\tsecond()\n" +
					"}\n" +
					"func s7ARRev16KnownCallbacks(callback s7ARRev16FixedTailCallback) s7ARRev16FixedTailCallbacks {\n" +
					"\tcallbacks := s7ARRev16FixedTailCallbacks{func() {}, callback}\n" +
					"\talias := callbacks\n" +
					"\treturn alias\n" +
					"}\n",
				wantErr:          "purge_progress inventory",
				wantActualWrites: 12,
			},
			{
				name: "promoted-receiver-held-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\treceiver := s7ARPromotedReceiverOriginInvoker{s7ARReceiverOriginBase{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}}\n" +
					"\t\ts7ARPromotedReceiverOriginInvoker.invoke(receiver, func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_receiver_origin.go",
				extraSource: "package cli\n\n" +
					"type s7ARReceiverOriginBase struct{ before func() }\n" +
					"type s7ARPromotedReceiverOriginInvoker struct{ s7ARReceiverOriginBase }\n\n" +
					"func (receiver s7ARPromotedReceiverOriginInvoker) invoke(callback func()) {\n" +
					"\treceiver.before()\n" +
					"\tcallback()\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "unresolved-invoked-receiver-origin",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvar receiver s7ARUnresolvedReceiverOriginInvoker\n" +
					"\t\treceiver.invoke(func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_receiver_origin.go",
				extraSource: "package cli\n\n" +
					"type s7ARUnresolvedReceiverOriginInvoker struct{ before func() }\n\n" +
					"func (receiver s7ARUnresolvedReceiverOriginInvoker) invoke(callback func()) {\n" +
					"\treceiver.before()\n" +
					"\tcallback()\n" +
					"}\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "rev14-unresolved-direct-interface-method-value",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvar dispatched s7ARRev14UnknownDirectInterface\n" +
					"\t\tdispatched.invoke(func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_unknown_direct_interface.go",
				extraSource: "package cli\n\n" +
					"type s7ARRev14UnknownDirectInterface interface{ invoke(func()) }\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "rev14-unresolved-direct-interface-method-expression",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvar dispatched s7ARRev14UnknownExpressionInterface\n" +
					"\t\ts7ARRev14UnknownExpressionInterface.invoke(dispatched, func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_unknown_direct_interface.go",
				extraSource: "package cli\n\n" +
					"type s7ARRev14UnknownExpressionInterface interface{ invoke(func()) }\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "rev15-unresolved-direct-variadic-interface-method-value",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvar dispatched s7ARRev15UnknownVariadicInterface\n" +
					"\t\tdispatched.invoke(func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_unknown_variadic_interface.go",
				extraSource: "package cli\n\n" +
					"type s7ARRev15UnknownCallback func()\n" +
					"type s7ARRev15UnknownVariadicInterface interface{ invoke(...s7ARRev15UnknownCallback) }\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "rev15-unresolved-direct-variadic-interface-method-expression",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvar dispatched s7ARRev15UnknownVariadicExpressionInterface\n" +
					"\t\ts7ARRev15UnknownVariadicExpressionInterface.invoke(dispatched, func() {})",
				extraRel: "internal/cli/zz_s7_ar_progress_unknown_variadic_interface.go",
				extraSource: "package cli\n\n" +
					"type s7ARRev15UnknownExpressionCallback = func()\n" +
					"type s7ARRev15UnknownVariadicExpressionInterface interface{ invoke(...s7ARRev15UnknownExpressionCallback) }\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "rev16-unresolved-expanded-variadic-slice",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tvar dispatched s7ARRev16ExpandedInterface = &s7ARRev16ExpandedInvoker{}\n" +
					"\t\tvar callbacks s7ARRev16ExpandedCallbacks\n" +
					"\t\tdispatched.invoke(\"run\", callbacks...)",
				extraRel: "internal/cli/zz_s7_ar_progress_unresolved_expansion.go",
				extraSource: "package cli\n\n" +
					"type s7ARRev16ExpandedCallback func()\n" +
					"type s7ARRev16ExpandedCallbacks []s7ARRev16ExpandedCallback\n" +
					"type s7ARRev16ExpandedInterface interface{ invoke(string, ...s7ARRev16ExpandedCallback) }\n" +
					"type s7ARRev16ExpandedInvoker struct{}\n\n" +
					"func (*s7ARRev16ExpandedInvoker) invoke(prefix string, callbacks ...s7ARRev16ExpandedCallback) {\n" +
					"\tif prefix == \"run\" { callbacks[0]() }\n" +
					"}\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "ordinary-function-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\ts7AROrdinaryCallbackInvoker(callback)",
				extraRel: "internal/cli/zz_s7_ar_progress_ordinary_function.go",
				extraSource: "package cli\n\n" +
					"func s7AROrdinaryCallbackInvoker(callback func()) { callback() }\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "callable-composite-field-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallable := struct{ run func() }{run: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tcallable.run()",
				wantErr: "purge_progress inventory",
			},
			{
				name: "named-helper-alias-forwarded-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\ts7ARInvokePurgeProgressCallbackAlias(callback)",
				extraRel: "internal/cli/zz_s7_ar_progress_callable.go",
				extraSource: "package cli\n\n" +
					"func s7ARInvokePurgeProgressCallback(fn func()) { s7ARForwardPurgeProgressCallback(fn) }\n" +
					"func s7ARForwardPurgeProgressCallback(fn func()) { fn() }\n" +
					"var s7ARInvokePurgeProgressCallbackAlias = s7ARInvokePurgeProgressCallback\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "local-factory-returned-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tfactory := func() func() {\n" +
					"\t\t\treturn func() {\n" +
					"\t\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t\t}\n" +
					"\t\t}\n" +
					"\t\tcallback := factory()\n" +
					"\t\tcallback()",
				wantErr: "purge_progress inventory",
			},
			{
				name: "named-helper-factory-returned-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := s7ARNamedPurgeProgressFactory(&report, result, options)\n" +
					"\t\tcallback()",
				extraRel: "internal/cli/zz_s7_ar_progress_factory.go",
				extraSource: "package cli\n\n" +
					"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
					"func s7ARNamedPurgeProgressFactory(report *intentArchivePurgeReport, result store.IntentArchivePurgeResult, options intentArchivePurgeOptions) func() {\n" +
					"\treturn func() {\n" +
					"\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t}\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "nested-forwarded-factory-result",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := s7ARForwardedPurgeProgressFactory(&report, result, options)\n" +
					"\t\tcallback()",
				extraRel: "internal/cli/zz_s7_ar_progress_factory.go",
				extraSource: "package cli\n\n" +
					"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
					"func s7ARForwardedPurgeProgressFactory(report *intentArchivePurgeReport, result store.IntentArchivePurgeResult, options intentArchivePurgeOptions) func() {\n" +
					"\treturn s7ARInnerPurgeProgressFactory(report, result, options)\n" +
					"}\n\n" +
					"func s7ARInnerPurgeProgressFactory(report *intentArchivePurgeReport, result store.IntentArchivePurgeResult, options intentArchivePurgeOptions) func() {\n" +
					"\treturn func() {\n" +
					"\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t}\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "factory-result-assigned-after-declaration",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tfactory := func() func() {\n" +
					"\t\t\treturn func() {\n" +
					"\t\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t\t}\n" +
					"\t\t}\n" +
					"\t\tvar callback func()\n" +
					"\t\tcallback = factory()\n" +
					"\t\talias := callback\n" +
					"\t\talias()",
				wantErr: "purge_progress inventory",
			},
			{
				name: "named-result-bare-return-reviewer-repro",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\ts7ARNamedResultPurgeProgressFactory(&report, result, options)()",
				extraRel: "internal/cli/zz_s7_ar_progress_named_result.go",
				extraSource: "package cli\n\n" +
					"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
					"func s7ARNamedResultPurgeProgressFactory(report *intentArchivePurgeReport, result store.IntentArchivePurgeResult, options intentArchivePurgeOptions) (callback func()) {\n" +
					"\tcallback = func() {\n" +
					"\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t}\n" +
					"\treturn\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "named-result-declared-later-parenthesized-alias",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := s7ARNamedAliasPurgeProgressFactory(&report, result, options)\n" +
					"\t\talias := (callback)\n" +
					"\t\talias()",
				extraRel: "internal/cli/zz_s7_ar_progress_named_result.go",
				extraSource: "package cli\n\n" +
					"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
					"func s7ARNamedAliasPurgeProgressFactory(report *intentArchivePurgeReport, result store.IntentArchivePurgeResult, options intentArchivePurgeOptions) (callback func()) {\n" +
					"\tvar assigned func()\n" +
					"\tassigned = func() {\n" +
					"\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t}\n" +
					"\tcallback = (assigned)\n" +
					"\treturn\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "named-result-branch-alternatives",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tcallback := s7ARBranchedNamedPurgeProgressFactory(&report, result, options)\n" +
					"\t\tcallback()",
				extraRel: "internal/cli/zz_s7_ar_progress_named_result.go",
				extraSource: "package cli\n\n" +
					"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
					"func s7ARBranchedNamedPurgeProgressFactory(report *intentArchivePurgeReport, result store.IntentArchivePurgeResult, options intentArchivePurgeOptions) (callback func()) {\n" +
					"\tif options.orphans {\n" +
					"\t\tcallback = func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t} else {\n" +
					"\t\tcallback = func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t}\n" +
					"\treturn\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "named-result-unresolved-factory-origin",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\ts7ARUnresolvedNamedPurgeProgressFactory()()",
				extraRel: "internal/cli/zz_s7_ar_progress_named_result.go",
				extraSource: "package cli\n\n" +
					"var s7ARUnknownPurgeProgressFactory func() func()\n\n" +
					"func s7ARUnresolvedNamedPurgeProgressFactory() (callback func()) {\n" +
					"\tcallback = s7ARUnknownPurgeProgressFactory()\n" +
					"\treturn\n" +
					"}\n",
				wantErr: "unresolved callable origins",
			},
			{
				name: "two-level-canonical-pointer-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tslot := &report.PurgeProgress\n" +
					"\t\ttwice := &slot\n" +
					"\t\t**twice = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				wantErr: "purge_progress inventory",
			},
			{
				name: "same-shaped-decoy-replaces-canonical-store",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "\t\tdecoy := s7ARDecoyPurgeReport{}\n" +
					"\t\tdecoy.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t_ = decoy",
				extraRel: "internal/cli/zz_s7_ar_progress_decoy.go",
				extraSource: "package cli\n\n" +
					"type s7ARDecoyPurgeReport struct {\n" +
					"\tPurgeProgress *intentArchivePurgeProgressReport\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "conditional-canonical-decoy-pointer-origin",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tdecoy := s7ARDecoyPurgeReport{}\n" +
					"\t\tslot := &report.PurgeProgress\n" +
					"\t\tif options.orphans {\n" +
					"\t\t\tslot = &decoy.PurgeProgress\n" +
					"\t\t}\n" +
					"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				extraRel: "internal/cli/zz_s7_ar_progress_decoy.go",
				extraSource: "package cli\n\n" +
					"type s7ARDecoyPurgeReport struct {\n" +
					"\tPurgeProgress *intentArchivePurgeProgressReport\n" +
					"}\n",
				wantErr: "ambiguous",
			},
			{
				name: "escaped-canonical-pointer-origin",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tslot := &report.PurgeProgress\n" +
					"\t\ts7AREscapePurgeProgressSlot(&slot)\n" +
					"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				extraRel: "internal/cli/zz_s7_ar_progress_escape.go",
				extraSource: "package cli\n\n" +
					"var s7AREscapePurgeProgressSlot func(***intentArchivePurgeProgressReport)\n",
				wantErr: "unresolved",
			},
			{
				name: "chained-assignment-alias-write",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\tfirst := &report.PurgeProgress\n" +
					"\t\tsecond := first\n" +
					"\t\tvar slot **intentArchivePurgeProgressReport\n" +
					"\t\tslot = second\n" +
					"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				wantErr: "purge_progress inventory",
			},
			{
				name: "cross-file-helper-pointer-write",
				old:  "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				new: "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\ts7ARMutatedProgressHelper(&report.PurgeProgress, result, report.Slug, options)",
				extraRel: "internal/cli/zz_s7_ar_progress_mutation.go",
				extraSource: "package cli\n\n" +
					"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
					"func s7ARMutatedProgressHelper(slot **intentArchivePurgeProgressReport, result store.IntentArchivePurgeResult, slug string, options intentArchivePurgeOptions) {\n" +
					"\ts7ARMutatedProgressStore(slot, result, slug, options)\n" +
					"}\n\n" +
					"func s7ARMutatedProgressStore(slot **intentArchivePurgeProgressReport, result store.IntentArchivePurgeResult, slug string, options intentArchivePurgeOptions) {\n" +
					"\t*slot = buildIntentArchivePurgeProgress(result, slug, options)\n" +
					"}\n",
				wantErr: "purge_progress inventory",
			},
			{
				name: "unkeyed-composite-field",
				old:  "report := intentArchivePurgeReport{",
				new: "_ = intentArchivePurgeReport{0, \"\", \"\", \"\", \"\", \"\", false, nil, nil, nil, nil, nil, nil, \"\", \"\", \"\", \"\", nil, nil, nil, buildIntentArchivePurgeProgress(store.IntentArchivePurgeResult{}, slug, options), nil, nil}\n" +
					"\treport := intentArchivePurgeReport{",
				wantErr: "purge_progress inventory",
			},
		} {
			mutated := strings.Replace(cliSource, fixture.old, fixture.new, 1)
			if mutated == cliSource {
				t.Fatalf("PIB-518 %s source sensitivity anchor missing", fixture.name)
			}
			mutatedSources := s6CloneSourceSet(sources)
			mutatedSources["internal/cli/feature_intent_archive.go"] = mutated
			if fixture.extraRel != "" {
				mutatedSources[fixture.extraRel] = fixture.extraSource
			}
			mutatedModel, err := s6BuildSourceTypeModel(
				s6EmissionTypeSources(s7ARCLIPackageSources(mutatedSources)),
			)
			if err != nil {
				t.Fatalf("PIB-518 %s sensitivity is not valid typed source: %v",
					fixture.name, err)
			}
			err = validateS7ARPurgeProgressWithModel(
				observations, mutatedSources, mutatedModel,
			)
			if err == nil {
				t.Fatalf("PIB-518 same validator accepted %s", fixture.name)
			}
			if fixture.wantErr != "" && !strings.Contains(err.Error(), fixture.wantErr) {
				t.Fatalf("PIB-518 %s failed for %q, want %q",
					fixture.name, err, fixture.wantErr)
			}
			if fixture.wantActualWrites != 0 {
				got, ok := s7ARInventoryActualWriteCount(err)
				if !ok || got != fixture.wantActualWrites {
					t.Fatalf("PIB-518 %s actual inventory writes = %d, %v; want %d",
						fixture.name, got, ok, fixture.wantActualWrites)
				}
			}
		}
		for _, fixture := range []struct {
			name        string
			insertion   string
			extraSource string
		}{
			{
				name: "inert-receiver-held-canonical-store",
				insertion: "\n" +
					"\t\treceiver := s7ARInertReceiverOriginInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\ts7ARInertReceiverOriginInvoker.invoke(receiver, func() {})",
				extraSource: "package cli\n\n" +
					"type s7ARInertReceiverOriginInvoker struct{ before func() }\n\n" +
					"func (s7ARInertReceiverOriginInvoker) invoke(callback func()) { callback() }\n",
			},
			{
				name: "invoked-receiver-held-decoy-store",
				insertion: "\n" +
					"\t\tdecoy := s7ARReceiverOriginDecoyReport{}\n" +
					"\t\treceiver := s7ARDecoyReceiverOriginInvoker{before: func() {\n" +
					"\t\t\tdecoy.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\treceiver.invoke(func() {})",
				extraSource: "package cli\n\n" +
					"type s7ARReceiverOriginDecoyReport struct {\n" +
					"\tPurgeProgress *intentArchivePurgeProgressReport\n" +
					"}\n" +
					"type s7ARDecoyReceiverOriginInvoker struct{ before func() }\n\n" +
					"func (receiver s7ARDecoyReceiverOriginInvoker) invoke(callback func()) {\n" +
					"\treceiver.before()\n" +
					"\tcallback()\n" +
					"}\n",
			},
			{
				name: "rev13-method-value-inert-and-decoy-controls",
				insertion: "\n" +
					"\t\tinertReceiver := s7ARMatrixInertInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tinertInvoke := inertReceiver.invoke\n" +
					"\t\t_ = inertInvoke\n" +
					"\t\tdecoy := s7ARMatrixDecoyReport{}\n" +
					"\t\tdecoyReceiver := s7ARMatrixDecoyInvoker{before: func() {\n" +
					"\t\t\tdecoy.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tdecoyInvoke := decoyReceiver.invoke\n" +
					"\t\tdecoyInvoke(func() {})\n" +
					"\t\tordinaryInvoke := s7ARMatrixOrdinaryInvoke\n" +
					"\t\tordinaryInvoke(func() {\n" +
					"\t\t\tdecoy.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t})\n" +
					"\t\tcallable := struct{ run func() }{run: func() {\n" +
					"\t\t\tdecoy.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tfieldInvoke := callable.run\n" +
					"\t\tfieldInvoke()\n" +
					"\t\tvar unresolved s7ARMatrixInertInterface\n" +
					"\t\tunresolvedInvoke := unresolved.invoke\n" +
					"\t\t_ = unresolvedInvoke",
				extraSource: "package cli\n\n" +
					"type s7ARMatrixDecoyReport struct {\n" +
					"\tPurgeProgress *intentArchivePurgeProgressReport\n" +
					"}\n" +
					"type s7ARMatrixInertInvoker struct{ before func() }\n" +
					"type s7ARMatrixDecoyInvoker struct{ before func() }\n" +
					"type s7ARMatrixInertInterface interface{ invoke(func()) }\n\n" +
					"func (receiver s7ARMatrixInertInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func (receiver s7ARMatrixDecoyInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func s7ARMatrixOrdinaryInvoke(callback func()) { callback() }\n",
			},
			{
				name: "rev14-direct-interface-inert-and-ordinary-controls",
				insertion: "\n" +
					"\t\treceiver := s7ARRev14InertInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar dispatched s7ARRev14InertInterface = receiver\n" +
					"\t\tmaterialized := dispatched.invoke\n" +
					"\t\ts7ARRev14RetainInterfaceMethod(materialized)\n" +
					"\t\treturned := s7ARRev14ReturnInterfaceMethod(materialized)\n" +
					"\t\tstored := struct{ invoke func(func()) }{invoke: returned}\n" +
					"\t\t_ = stored\n" +
					"\t\tordinaryField := struct{ invoke func(func()) }{invoke: func(callback func()) { callback() }}\n" +
					"\t\tordinaryField.invoke(func() {})\n" +
					"\t\ts7ARRev14OrdinaryFunction(func() {})\n" +
					"\t\tvariadicReceiver := s7ARRev15InertInvoker{before: func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}}\n" +
					"\t\tvar variadicDispatched s7ARRev15InertInterface = variadicReceiver\n" +
					"\t\tvariadicMaterialized := variadicDispatched.invoke\n" +
					"\t\tvariadicAssigned := variadicMaterialized\n" +
					"\t\ts7ARRev15RetainVariadicInterfaceMethod(variadicAssigned)\n" +
					"\t\tvariadicReturned := s7ARRev15ReturnVariadicInterfaceMethod(variadicAssigned)\n" +
					"\t\tvariadicStored := struct{ invoke func(...func()) }{invoke: variadicReturned}\n" +
					"\t\t_ = variadicStored\n" +
					"\t\tvar containers s7ARRev15FixedContainerInterface\n" +
					"\t\tcontainers.retain(nil, nil, nil, struct{ run func() }{})\n" +
					"\t\tordinary := func() {\n" +
					"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
					"\t\t}\n" +
					"\t\ts7ARRev15OrdinaryVariadic(ordinary)\n" +
					"\t\tvariadicField := struct{ invoke func(...func()) }{invoke: func(...func()) {}}\n" +
					"\t\tvariadicField.invoke(ordinary)",
				extraSource: "package cli\n\n" +
					"type s7ARRev14InertInterface interface{ invoke(func()) }\n" +
					"type s7ARRev14InertInvoker struct{ before func() }\n\n" +
					"func (receiver s7ARRev14InertInvoker) invoke(callback func()) { receiver.before(); callback() }\n" +
					"func s7ARRev14RetainInterfaceMethod(func(func())) {}\n" +
					"func s7ARRev14ReturnInterfaceMethod(invoke func(func())) func(func()) { return invoke }\n" +
					"func s7ARRev14OrdinaryFunction(callback func()) { callback() }\n\n" +
					"type s7ARRev15InertInterface interface{ invoke(...func()) }\n" +
					"type s7ARRev15InertInvoker struct{ before func() }\n" +
					"type s7ARRev15FixedContainerInterface interface {\n" +
					"\tretain([]func(), map[string]func(), chan func(), struct{ run func() })\n" +
					"}\n\n" +
					"func (receiver s7ARRev15InertInvoker) invoke(callbacks ...func()) {\n" +
					"\treceiver.before()\n" +
					"\tfor _, callback := range callbacks { callback() }\n" +
					"}\n" +
					"func s7ARRev15RetainVariadicInterfaceMethod(func(...func())) {}\n" +
					"func s7ARRev15ReturnVariadicInterfaceMethod(invoke func(...func())) func(...func()) { return invoke }\n" +
					"func s7ARRev15OrdinaryVariadic(...func()) {}\n",
			},
		} {
			mutatedSources := s6CloneSourceSet(sources)
			mutatedSources["internal/cli/feature_intent_archive.go"] = strings.Replace(
				cliSource,
				"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
				"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"+
					fixture.insertion,
				1,
			)
			mutatedSources["internal/cli/zz_s7_ar_progress_receiver_control.go"] =
				fixture.extraSource
			mutatedModel, err := s6BuildSourceTypeModel(
				s6EmissionTypeSources(s7ARCLIPackageSources(mutatedSources)),
			)
			if err != nil {
				t.Fatalf("PIB-518 %s is not valid typed source: %v", fixture.name, err)
			}
			if err := validateS7ARPurgeProgressWithModel(
				observations, mutatedSources, mutatedModel,
			); err != nil {
				t.Fatalf("PIB-518 %s created a false inventory: %v", fixture.name, err)
			}
		}
		decoySources := s6CloneSourceSet(sources)
		decoySource := strings.Replace(
			cliSource,
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\tdecoy := s7ARDecoyPurgeReport{}\n"+
				"\t\tdecoy.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t_ = decoy",
			1,
		)
		if decoySource == cliSource {
			t.Fatal("PIB-518 unrelated same-shaped field sensitivity anchor missing")
		}
		decoySources["internal/cli/feature_intent_archive.go"] = decoySource
		decoySources["internal/cli/zz_s7_ar_progress_decoy.go"] =
			"package cli\n\n" +
				"type s7ARDecoyPurgeReport struct {\n" +
				"\tPurgeProgress *intentArchivePurgeProgressReport\n" +
				"}\n"
		decoyModel, err := s6BuildSourceTypeModel(s6EmissionTypeSources(s7ARCLIPackageSources(decoySources)))
		if err != nil {
			t.Fatalf("PIB-518 unrelated same-shaped field is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, decoySources, decoyModel,
		); err != nil {
			t.Fatalf("PIB-518 unrelated same-shaped field created a false emitter: %v", err)
		}
		decoyPointerSources := s6CloneSourceSet(sources)
		decoyPointerSource := strings.Replace(
			cliSource,
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\tdecoy := s7ARDecoyPurgeReport{}\n"+
				"\t\tslot := &decoy.PurgeProgress\n"+
				"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t_ = decoy",
			1,
		)
		if decoyPointerSource == cliSource {
			t.Fatal("PIB-518 unrelated decoy-pointer sensitivity anchor missing")
		}
		decoyPointerSources["internal/cli/feature_intent_archive.go"] = decoyPointerSource
		decoyPointerSources["internal/cli/zz_s7_ar_progress_decoy.go"] =
			decoySources["internal/cli/zz_s7_ar_progress_decoy.go"]
		decoyPointerModel, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(decoyPointerSources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 unrelated decoy pointer is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, decoyPointerSources, decoyPointerModel,
		); err != nil {
			t.Fatalf("PIB-518 definite unrelated decoy pointer created a false failure: %v", err)
		}
		decoyDoublePointerSources := s6CloneSourceSet(decoyPointerSources)
		decoyDoublePointerSource := strings.Replace(
			decoyPointerSource,
			"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"\t\ttwice := &slot\n"+
				"\t\t**twice = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			1,
		)
		if decoyDoublePointerSource == decoyPointerSource {
			t.Fatal("PIB-518 two-level decoy-pointer sensitivity anchor missing")
		}
		decoyDoublePointerSources["internal/cli/feature_intent_archive.go"] =
			decoyDoublePointerSource
		decoyDoublePointerModel, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(decoyDoublePointerSources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 two-level decoy pointer is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, decoyDoublePointerSources, decoyDoublePointerModel,
		); err != nil {
			t.Fatalf("PIB-518 definite two-level decoy pointer created a false failure: %v", err)
		}
		decoyHelperSources := s6CloneSourceSet(decoyPointerSources)
		decoyHelperSources["internal/cli/feature_intent_archive.go"] = strings.Replace(
			decoyPointerSource,
			"\t\t*slot = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"\t\ts7ARWriteDecoyProgress(slot, result, report.Slug, options)",
			1,
		)
		decoyHelperSources["internal/cli/zz_s7_ar_progress_decoy.go"] =
			"package cli\n\n" +
				"import \"github.com/tesseracode/tesserapatch/internal/store\"\n\n" +
				"type s7ARDecoyPurgeReport struct {\n" +
				"\tPurgeProgress *intentArchivePurgeProgressReport\n" +
				"}\n\n" +
				"func s7ARWriteDecoyProgress(slot **intentArchivePurgeProgressReport, result store.IntentArchivePurgeResult, slug string, options intentArchivePurgeOptions) {\n" +
				"\t*slot = buildIntentArchivePurgeProgress(result, slug, options)\n" +
				"}\n"
		decoyHelperModel, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(decoyHelperSources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 unrelated decoy helper is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, decoyHelperSources, decoyHelperModel,
		); err != nil {
			t.Fatalf("PIB-518 definite unrelated decoy helper created a false failure: %v", err)
		}
		uninvokedSources := s6CloneSourceSet(sources)
		uninvokedSource := strings.Replace(
			cliSource,
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\tunused := func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t}\n"+
				"\t\t_ = unused",
			1,
		)
		if uninvokedSource == cliSource {
			t.Fatal("PIB-518 uninvoked closure sensitivity anchor missing")
		}
		uninvokedSources["internal/cli/feature_intent_archive.go"] = uninvokedSource
		uninvokedModel, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(uninvokedSources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 uninvoked closure is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, uninvokedSources, uninvokedModel,
		); err != nil {
			t.Fatalf("PIB-518 uninvoked closure created a false emitter: %v", err)
		}
		uninvokedFactorySources := s6CloneSourceSet(sources)
		uninvokedFactorySource := strings.Replace(
			cliSource,
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\tfactory := func() func() {\n"+
				"\t\t\treturn func() {\n"+
				"\t\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t\t}\n"+
				"\t\t}\n"+
				"\t\t_ = factory",
			1,
		)
		if uninvokedFactorySource == cliSource {
			t.Fatal("PIB-518 uninvoked returned closure sensitivity anchor missing")
		}
		uninvokedFactorySources["internal/cli/feature_intent_archive.go"] =
			uninvokedFactorySource
		uninvokedFactoryModel, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(uninvokedFactorySources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 uninvoked returned closure is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, uninvokedFactorySources, uninvokedFactoryModel,
		); err != nil {
			t.Fatalf("PIB-518 uninvoked returned closure created a false emitter: %v", err)
		}
		uninvokedResultSources := s6CloneSourceSet(sources)
		uninvokedResultSource := strings.Replace(
			cliSource,
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)",
			"report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\tfactory := func() (callback func()) {\n"+
				"\t\t\tcallback = func() {\n"+
				"\t\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t\t}\n"+
				"\t\t\treturn\n"+
				"\t\t}\n"+
				"\t\t_ = factory()",
			1,
		)
		if uninvokedResultSource == cliSource {
			t.Fatal("PIB-518 uninvoked named-result sensitivity anchor missing")
		}
		uninvokedResultSources["internal/cli/feature_intent_archive.go"] =
			uninvokedResultSource
		uninvokedResultModel, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(uninvokedResultSources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 uninvoked named result is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			observations, uninvokedResultSources, uninvokedResultModel,
		); err != nil {
			t.Fatalf("PIB-518 uninvoked named result created a false emitter: %v", err)
		}
	})
}

func s7ARDerivePurgeResumeDomain(source string) ([]store.IntentArchivePurgeResume, error) {
	matches := regexp.MustCompile(
		`(?m)^\s*IntentArchiveResume[A-Za-z0-9_]*\s+IntentArchivePurgeResume\s*=\s*"([^"]+)"`,
	).FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		return nil, errors.New("purge resume domain is empty")
	}
	resumes := make([]store.IntentArchivePurgeResume, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if seen[match[1]] {
			return nil, fmt.Errorf("duplicate purge resume %q", match[1])
		}
		seen[match[1]] = true
		resumes = append(resumes, store.IntentArchivePurgeResume(match[1]))
	}
	return resumes, nil
}

type s7ARPurgeProgressObservation struct {
	report      intentArchivePurgeReport
	instruction string
	human       string
}

func s7ARProductionPurgeProgressReports(
	resumes []store.IntentArchivePurgeResume,
) []s7ARPurgeProgressObservation {
	hash := strings.Repeat("a", 64)
	reports := make([]s7ARPurgeProgressObservation, 0, len(resumes))
	for _, resume := range resumes {
		result := store.IntentArchivePurgeResult{
			CompletedHashes: []string{strings.Repeat("b", 64)},
			RemainingHashes: []string{strings.Repeat("c", 64)},
			Resume:          resume,
			State:           store.IntentArchivePurgeStateConsistent,
		}
		if resume == store.IntentArchiveResumePendingRecoveryThenCompletion {
			result.PendingHash = hash
		}
		progress := buildIntentArchivePurgeProgress(
			result,
			"ar-purge-progress",
			intentArchivePurgeOptions{all: true, yes: true, asJSON: true},
		)
		full := newIntentArchivePurgeReport(
			"ar-purge-progress",
			intentArchivePurgeOptions{all: true, yes: true, asJSON: true},
		)
		full.Outcome = string(store.IntentArchivePurgePartial)
		full.PurgeProgress = progress
		var human bytes.Buffer
		writeIntentArchivePurgeHuman(&human, full)
		reports = append(reports, s7ARPurgeProgressObservation{
			report:      full,
			instruction: intentArchivePurgeResumeInstruction(progress.Resume),
			human:       human.String(),
		})
	}
	return reports
}

func validateS7ARPurgeProgress(
	observations []s7ARPurgeProgressObservation,
	sources map[string]string,
) error {
	model, err := s6BuildSourceTypeModel(s6EmissionTypeSources(s7ARCLIPackageSources(sources)))
	if err != nil {
		return fmt.Errorf("type-check purge_progress emitters: %w", err)
	}
	return validateS7ARPurgeProgressWithModel(observations, sources, model)
}

func validateS7ARPurgeProgressWithModel(
	observations []s7ARPurgeProgressObservation,
	sources map[string]string,
	model *s6SourceTypeModel,
) error {
	if err := validateS7ARPurgeProgressEmittersWithModel(sources, model); err != nil {
		return err
	}
	want := map[string]bool{
		string(store.IntentArchiveResumePendingRecoveryThenCompletion): true,
		string(store.IntentArchiveResumeCompletionOnly):                true,
		string(store.IntentArchiveResumeOrphanScan):                    true,
	}
	instructions := map[string]string{
		string(store.IntentArchiveResumePendingRecoveryThenCompletion): "The first retry finalizes the pending hash and exits 0 recovered without processing the selector. Run the same command a second time to complete the remaining work.",
		string(store.IntentArchiveResumeCompletionOnly):                "Exactly one retry completes the remaining hashes. It does not produce or promise a recovered outcome.",
		string(store.IntentArchiveResumeOrphanScan):                    "Exactly one retry rescans the archive and removes the remaining orphan blobs. It does not produce or promise a recovered outcome.",
	}
	if len(observations) != len(want) {
		return fmt.Errorf("purge progress branch count = %d, want %d", len(observations), len(want))
	}
	seen := map[string]bool{}
	for _, observation := range observations {
		report := observation.report
		if report.PurgeProgress == nil {
			return errors.New("purge progress report omitted purge_progress")
		}
		resume := report.PurgeProgress.Resume
		if !want[resume] || seen[resume] {
			return fmt.Errorf("purge progress resume is unknown or duplicated: %q", resume)
		}
		seen[resume] = true
		expected := s7ARExpectedGuardPurgeProgress(store.IntentArchivePurgeResume(resume))
		if !reflect.DeepEqual(report, expected) {
			return fmt.Errorf("resume %s report mismatch\ngot:  %#v\nwant: %#v",
				resume, report, expected)
		}
		if observation.instruction != instructions[resume] {
			return fmt.Errorf("resume %s instruction = %q, want %q",
				resume, observation.instruction, instructions[resume])
		}
		wantHuman := s7ARRenderPartialHuman(expected, instructions[resume])
		if observation.human != wantHuman {
			return fmt.Errorf("resume %s human rendering mismatch\nwant:\n%s\ngot:\n%s",
				resume, wantHuman, observation.human)
		}
	}
	return nil
}

func s7ARExpectedGuardPurgeProgress(
	resume store.IntentArchivePurgeResume,
) intentArchivePurgeReport {
	report := s7ARExpectedPartialReport(
		"ar-purge-progress",
		"all",
		[]string{},
		[]string{},
		[]intentArchivePurgeReferenceReport{},
		[]intentArchivePurgeBlobReport{},
		[]string{},
		resume,
		strings.Repeat("b", 64),
		strings.Repeat("c", 64),
	)
	if resume == store.IntentArchiveResumePendingRecoveryThenCompletion {
		report.PurgeProgress.PendingHash = strings.Repeat("a", 64)
	}
	return report
}

func TestS7ARRev11ReviewerRepros(t *testing.T) {
	t.Run("PIB-518-method-expression", func(t *testing.T) {
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+"\n"+
				"\t\tcallback := func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t}\n"+
				"\t\ts7ARRev11Invoker.invoke(s7ARRev11Invoker{}, callback)",
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 method-expression reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev11_method_expression.go"] =
			"package cli\n\n" +
				"type s7ARRev11Invoker struct{}\n\n" +
				"func (s7ARRev11Invoker) invoke(callback func()) { callback() }\n"
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 method-expression reviewer repro is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		); err == nil {
			t.Fatal("PIB-518 same complete validator omitted a method-expression callback")
		}
	})

	t.Run("PIB-519-dependent-cache", func(t *testing.T) {
		sources := s7ARShippedClaimSources(t)
		const linuxPrefix = "internal/cli/zz_s7_ar_rev11_prefix_linux.go"
		const darwinPrefix = "internal/cli/zz_s7_ar_rev11_prefix_darwin.go"
		const claimSource = "internal/cli/zz_s7_ar_rev11_claim_unix.go"
		sources[linuxPrefix] = "//go:build linux\n\npackage cli\n\n" +
			"const s7ARRev11Prefix = \"The archive can be \"\n"
		sources[darwinPrefix] = "//go:build darwin\n\npackage cli\n\n" +
			"const s7ARRev11Prefix = \"The archive can be \"\n"
		sources[claimSource] = "//go:build linux || darwin\n\npackage cli\n\n" +
			"const s7ARRev11Claim = s7ARRev11Prefix + \"stranded.\"\n"
		state, err := s7ARPrepareClaimSourceState(sources)
		if err != nil {
			t.Fatal(err)
		}
		baseline, err := s7ARDerivePermanentClaimInventoryWithState(sources, state)
		if err != nil {
			t.Fatal(err)
		}
		if len(baseline) != 8 {
			t.Fatalf("PIB-519 dependent-cache baseline inventory = %d, want 8", len(baseline))
		}
		assertAuthoritative := func(name, changedFile string) {
			t.Helper()
			mutated := cloneS7AQSources(sources)
			mutated[changedFile] = strings.Replace(
				mutated[changedFile],
				"\"The archive can be \"",
				"\"The archive cannot be \"",
				1,
			)
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, state,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", name, deriveErr)
			}
			if len(got) != len(baseline)+1 {
				t.Fatalf("PIB-519 %s inventory = %d, want %d",
					name, len(got), len(baseline)+1)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, got, state,
			); err == nil {
				t.Fatalf("PIB-519 same complete validator reused stale %s claim state", name)
			}
		}
		assertHarmless := func(name string) {
			t.Helper()
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				sources, state,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", name, deriveErr)
			}
			if !reflect.DeepEqual(got, baseline) {
				t.Fatalf("PIB-519 %s leaked a stale authoritative claim: %v", name, got)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				sources, got, state,
			); err != nil {
				t.Fatalf("PIB-519 %s rejected restored harmless state: %v", name, err)
			}
		}
		assertAuthoritative("linux-authoritative", linuxPrefix)
		assertHarmless("restored-after-linux")
		assertAuthoritative("darwin-authoritative-order-permutation", darwinPrefix)
		assertHarmless("restored-after-darwin")
	})

	t.Run("PIB-519-selector-specific-route", func(t *testing.T) {
		sources := s7ARShippedClaimSources(t)
		state, err := s7ARPrepareClaimSourceState(sources)
		if err != nil {
			t.Fatal(err)
		}
		baseline, err := s7ARDerivePermanentClaimInventoryWithState(sources, state)
		if err != nil {
			t.Fatal(err)
		}
		mutated := cloneS7AQSources(sources)
		mutated["assets/skills/cursor/tessera-patch.mdc"] +=
			"\nThe archive cannot be stranded; operators must run " +
				"`tpatch feature intent-archive purge <slug> --blob <slug> --yes`.\n"
		got, err := s7ARDerivePermanentClaimInventoryWithState(mutated, state)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(baseline)+1 {
			t.Fatalf("PIB-519 selector reviewer repro inventory = %d, want %d",
				len(got), len(baseline)+1)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, got, state,
		); err == nil {
			t.Fatal("PIB-519 same complete validator accepted --blob <slug>")
		}
	})

	t.Run("PIB-519-negative-quantifier", func(t *testing.T) {
		sources := s7ARShippedClaimSources(t)
		state, err := s7ARPrepareClaimSourceState(sources)
		if err != nil {
			t.Fatal(err)
		}
		baseline, err := s7ARDerivePermanentClaimInventoryWithState(sources, state)
		if err != nil {
			t.Fatal(err)
		}
		mutated := cloneS7AQSources(sources)
		mutated["assets/skills/cursor/tessera-patch.mdc"] +=
			"\nNo operator can strand the archive.\n"
		got, err := s7ARDerivePermanentClaimInventoryWithState(mutated, state)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(baseline)+1 {
			t.Fatalf("PIB-519 negative-quantifier reviewer repro inventory = %d, want %d",
				len(got), len(baseline)+1)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, got, state,
		); err == nil {
			t.Fatal("PIB-519 same complete validator omitted a negative-quantifier claim")
		}
	})
}

func TestS7ARRev12ReviewerRepros(t *testing.T) {
	t.Run("PIB-518-method-expression-receiver-origin", func(t *testing.T) {
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+"\n"+
				"\t\treceiver := s7ARRev12ReceiverInvoker{before: func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t}}\n"+
				"\t\ts7ARRev12ReceiverInvoker.invoke(receiver, func() {})",
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 method-expression receiver reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev12_receiver_origin.go"] =
			"package cli\n\n" +
				"type s7ARRev12ReceiverInvoker struct{ before func() }\n\n" +
				"func (receiver s7ARRev12ReceiverInvoker) invoke(callback func()) {\n" +
				"\treceiver.before()\n" +
				"\tcallback()\n" +
				"}\n"
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 method-expression receiver reviewer repro is not valid typed source: %v", err)
		}
		if err := validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		); err == nil {
			t.Fatal("PIB-518 same complete validator omitted an invoked receiver-held callback")
		}
	})

	t.Run("PIB-519-production-slug-domain", func(t *testing.T) {
		sources := s7ARShippedClaimSources(t)
		state, err := s7ARPrepareClaimSourceState(sources)
		if err != nil {
			t.Fatal(err)
		}
		baseline, err := s7ARDerivePermanentClaimInventoryWithState(sources, state)
		if err != nil {
			t.Fatal(err)
		}
		for _, fixture := range []struct {
			name string
			slug string
		}{
			{name: "windows-reserved-con", slug: "con"},
			{name: "sixty-one-bytes", slug: strings.Repeat("a", 61)},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				mutated := cloneS7AQSources(sources)
				mutated["assets/skills/cursor/tessera-patch.mdc"] +=
					"\nThe archive cannot be stranded; operators must run `tpatch prepare " +
						fixture.slug + " --abandon-transaction`.\n"
				got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
					mutated, state,
				)
				if deriveErr != nil {
					t.Fatalf("PIB-519 slug %q inventory: %v", fixture.slug, deriveErr)
				}
				if len(got) != len(baseline)+1 {
					t.Fatalf("PIB-519 slug %q inventory = %d, want %d",
						fixture.slug, len(got), len(baseline)+1)
				}
				if err := validateS7ARPermanentBlockClaimsWithState(
					mutated, got, state,
				); err == nil {
					t.Fatalf("PIB-519 same complete validator accepted production-unsafe slug %q", fixture.slug)
				}
			})
		}
	})

	t.Run("PIB-519-punctuated-negative-quantifier", func(t *testing.T) {
		sources := s7ARShippedClaimSources(t)
		state, err := s7ARPrepareClaimSourceState(sources)
		if err != nil {
			t.Fatal(err)
		}
		baseline, err := s7ARDerivePermanentClaimInventoryWithState(sources, state)
		if err != nil {
			t.Fatal(err)
		}
		for _, fixture := range []struct {
			name  string
			claim string
		}{
			{name: "comma-delimited-ever", claim: "No operator, ever, can strand the archive."},
			{name: "ever-before-modal", claim: "No operator ever can strand the archive."},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				mutated := cloneS7AQSources(sources)
				mutated["assets/skills/cursor/tessera-patch.mdc"] += "\n" + fixture.claim + "\n"
				got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
					mutated, state,
				)
				if deriveErr != nil {
					t.Fatalf("PIB-519 claim %q inventory: %v", fixture.claim, deriveErr)
				}
				if len(got) != len(baseline)+1 {
					t.Fatalf("PIB-519 claim %q inventory = %d, want %d",
						fixture.claim, len(got), len(baseline)+1)
				}
				if err := validateS7ARPermanentBlockClaimsWithState(
					mutated, got, state,
				); err == nil {
					t.Fatalf("PIB-519 same complete validator omitted claim %q", fixture.claim)
				}
			})
		}
	})
}

func TestS7ARRev13ReviewerRepro(t *testing.T) {
	storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
	sources := s7ARProductionSourceSet(t)
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	mutated := strings.Replace(
		sources["internal/cli/feature_intent_archive.go"],
		storeCall,
		storeCall+"\n"+
			"\t\treceiver := s7ARRev13ReceiverInvoker{before: func() {\n"+
			"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
			"\t\t}}\n"+
			"\t\tinvoke := receiver.invoke\n"+
			"\t\tinvoke(func() {})",
		1,
	)
	if mutated == sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("PIB-518 aliased method-value reviewer repro anchor missing")
	}
	sources["internal/cli/feature_intent_archive.go"] = mutated
	sources["internal/cli/zz_s7_ar_rev13_method_value.go"] =
		"package cli\n\n" +
			"type s7ARRev13ReceiverInvoker struct{ before func() }\n\n" +
			"func (receiver s7ARRev13ReceiverInvoker) invoke(callback func()) {\n" +
			"\treceiver.before()\n" +
			"\tcallback()\n" +
			"}\n"
	resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
	if err != nil {
		t.Fatal(err)
	}
	model, err := s6BuildSourceTypeModel(
		s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
	)
	if err != nil {
		t.Fatalf("PIB-518 aliased method-value reviewer repro is not valid typed source: %v", err)
	}
	if err := validateS7ARPurgeProgressWithModel(
		s7ARProductionPurgeProgressReports(resumes), sources, model,
	); err == nil {
		t.Fatal("PIB-518 same complete validator omitted an invoked aliased method value")
	}

	unsupportedSources := s7ARProductionSourceSet(t)
	unsupported := strings.Replace(
		unsupportedSources["internal/cli/feature_intent_archive.go"],
		storeCall,
		storeCall+"\n"+
			"\t\tvar dispatched s7ARRev13UnknownInvoker\n"+
			"\t\tinvoke := dispatched.invoke\n"+
			"\t\tinvoke(func() {})",
		1,
	)
	if unsupported == unsupportedSources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("PIB-518 unresolved interface method-value anchor missing")
	}
	unsupportedSources["internal/cli/feature_intent_archive.go"] = unsupported
	unsupportedSources["internal/cli/zz_s7_ar_rev13_interface_method.go"] =
		"package cli\n\n" +
			"type s7ARRev13UnknownInvoker interface{ invoke(func()) }\n"
	unsupportedModel, err := s6BuildSourceTypeModel(
		s6EmissionTypeSources(s7ARCLIPackageSources(unsupportedSources)),
	)
	if err != nil {
		t.Fatalf("PIB-518 unresolved interface method value is not valid typed source: %v", err)
	}
	err = validateS7ARPurgeProgressWithModel(
		s7ARProductionPurgeProgressReports(resumes), unsupportedSources,
		unsupportedModel,
	)
	if err == nil || !strings.Contains(err.Error(), "unresolved callable origins") {
		t.Fatalf("PIB-518 unsupported invoked interface dispatch = %v, want deterministic unresolved evidence", err)
	}
}

func TestS7ARRev14ReviewerRepro(t *testing.T) {
	storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
	sources := s7ARProductionSourceSet(t)
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	mutated := strings.Replace(
		sources["internal/cli/feature_intent_archive.go"],
		storeCall,
		storeCall+"\n"+
			"\t\treceiver := s7ARRev14ConcreteInvoker{before: func() {\n"+
			"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
			"\t\t}}\n"+
			"\t\tvar dispatched s7ARRev14InterfaceInvoker = receiver\n"+
			"\t\tdispatched.invoke(func() {})",
		1,
	)
	if mutated == sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("PIB-518 direct interface method-value reviewer repro anchor missing")
	}
	sources["internal/cli/feature_intent_archive.go"] = mutated
	sources["internal/cli/zz_s7_ar_rev14_direct_interface.go"] =
		"package cli\n\n" +
			"type s7ARRev14InterfaceInvoker interface{ invoke(func()) }\n\n" +
			"type s7ARRev14ConcreteInvoker struct{ before func() }\n\n" +
			"func (receiver s7ARRev14ConcreteInvoker) invoke(callback func()) {\n" +
			"\treceiver.before()\n" +
			"\tcallback()\n" +
			"}\n"
	resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
	if err != nil {
		t.Fatal(err)
	}
	model, err := s6BuildSourceTypeModel(
		s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
	)
	if err != nil {
		t.Fatalf("PIB-518 direct interface method-value reviewer repro is not valid typed source: %v", err)
	}
	err = validateS7ARPurgeProgressWithModel(
		s7ARProductionPurgeProgressReports(resumes), sources, model,
	)
	actual, ok := s7ARInventoryActualWriteCount(err)
	if !ok || actual != 4 {
		t.Fatalf(
			"PIB-518 same complete validator omitted direct interface dispatch: error=%v writes=%d counted=%t, want four canonical writes",
			err, actual, ok,
		)
	}
}

func TestS7ARRev15ReviewerRepro(t *testing.T) {
	storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
	sources := s7ARProductionSourceSet(t)
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	mutated := strings.Replace(
		sources["internal/cli/feature_intent_archive.go"],
		storeCall,
		storeCall+"\n"+
			"\t\treceiver := s7ARRev15ConcreteInvoker{before: func() {\n"+
			"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
			"\t\t}}\n"+
			"\t\tvar dispatched s7ARRev15InterfaceInvoker = receiver\n"+
			"\t\tdispatched.invoke(func() {})",
		1,
	)
	if mutated == sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("PIB-518 direct variadic interface method-value reviewer repro anchor missing")
	}
	sources["internal/cli/feature_intent_archive.go"] = mutated
	sources["internal/cli/zz_s7_ar_rev15_variadic_interface.go"] =
		"package cli\n\n" +
			"type s7ARRev15InterfaceInvoker interface{ invoke(...func()) }\n\n" +
			"type s7ARRev15ConcreteInvoker struct{ before func() }\n\n" +
			"func (receiver s7ARRev15ConcreteInvoker) invoke(callbacks ...func()) {\n" +
			"\treceiver.before()\n" +
			"\tfor _, callback := range callbacks {\n" +
			"\t\tcallback()\n" +
			"\t}\n" +
			"}\n"
	resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
	if err != nil {
		t.Fatal(err)
	}
	model, err := s6BuildSourceTypeModel(
		s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
	)
	if err != nil {
		t.Fatalf("PIB-518 direct variadic interface method-value reviewer repro is not valid typed source: %v", err)
	}
	err = validateS7ARPurgeProgressWithModel(
		s7ARProductionPurgeProgressReports(resumes), sources, model,
	)
	actual, ok := s7ARInventoryActualWriteCount(err)
	if !ok || actual != 4 {
		t.Fatalf(
			"PIB-518 same complete validator omitted direct variadic interface dispatch: error=%v writes=%d counted=%t, want four canonical writes",
			err, actual, ok,
		)
	}
}

func TestS7ARRev16ReviewerRepro(t *testing.T) {
	storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
	sources := s7ARProductionSourceSet(t)
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	mutated := strings.Replace(
		sources["internal/cli/feature_intent_archive.go"],
		storeCall,
		storeCall+"\n"+
			"\t\tvar dispatched s7ARRev16InterfaceInvoker = s7ARRev16ConcreteInvoker{}\n"+
			"\t\tdispatched.invoke(func() {}, func() {\n"+
			"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
			"\t\t})",
		1,
	)
	if mutated == sources["internal/cli/feature_intent_archive.go"] {
		t.Fatal("PIB-518 later variadic callback reviewer repro anchor missing")
	}
	sources["internal/cli/feature_intent_archive.go"] = mutated
	sources["internal/cli/zz_s7_ar_rev16_variadic_binding.go"] =
		"package cli\n\n" +
			"type s7ARRev16InterfaceInvoker interface{ invoke(...func()) }\n\n" +
			"type s7ARRev16ConcreteInvoker struct{}\n\n" +
			"func (s7ARRev16ConcreteInvoker) invoke(callbacks ...func()) {\n" +
			"\tcallbacks[1]()\n" +
			"}\n"
	resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
	if err != nil {
		t.Fatal(err)
	}
	model, err := s6BuildSourceTypeModel(
		s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
	)
	if err != nil {
		t.Fatalf("PIB-518 later variadic callback reviewer repro is not valid typed source: %v", err)
	}
	err = validateS7ARPurgeProgressWithModel(
		s7ARProductionPurgeProgressReports(resumes), sources, model,
	)
	actual, ok := s7ARInventoryActualWriteCount(err)
	if !ok || actual != 4 {
		t.Fatalf(
			"PIB-518 same complete validator omitted later variadic callback: error=%v writes=%d counted=%t, want four canonical writes",
			err, actual, ok,
		)
	}
}

func TestS7ARRev17ReviewerRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraRel string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-17 reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources[extraRel] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-17 reviewer repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const interfaceSource = "package cli\n\n" +
		"type s7ARRev17Interface interface{ invoke(...func()) }\n\n" +
		"type s7ARRev17Invoker struct{}\n\n" +
		"func (s7ARRev17Invoker) invoke(callbacks ...func()) {\n" +
		"\tcallbacks[0]()\n" +
		"}\n"

	t.Run("inverse-direct-method-value", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n"+
				"\t\tdispatched.invoke(func() {}, func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t})",
			"internal/cli/zz_s7_ar_rev17_inverse_method_value.go",
			interfaceSource,
		)
		if err != nil {
			t.Fatalf("PIB-518 invented authority from uninvoked index 1: %v", err)
		}
	})

	t.Run("inverse-direct-method-expression", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n"+
				"\t\ts7ARRev17Interface.invoke(dispatched, func() {}, func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t})",
			"internal/cli/zz_s7_ar_rev17_inverse_method_expression.go",
			interfaceSource,
		)
		if err != nil {
			t.Fatalf("PIB-518 MethodExpr invented authority from uninvoked index 1: %v", err)
		}
	})

	for _, fixture := range []struct {
		name      string
		insertion string
	}{
		{
			name: "direct-index-write",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tcallbacks[0] = func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}\n" +
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
		{
			name: "copy-write",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\treplacement := []func(){func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}}\n" +
				"\t\tcopy(callbacks, replacement)\n" +
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
		{
			name: "alias-index-write",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\talias := callbacks\n" +
				"\t\talias[0] = func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}\n" +
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(
				t,
				fixture.insertion,
				"internal/cli/zz_s7_ar_rev17_mutation.go",
				interfaceSource,
			)
			actual, ok := s7ARInventoryActualWriteCount(err)
			if !ok || actual != 4 {
				t.Fatalf(
					"PIB-518 omitted %s authority: error=%v writes=%d counted=%t, want four canonical writes",
					fixture.name, err, actual, ok,
				)
			}
		})
	}

	t.Run("inert-named-expanded-slice", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\tvar callbacks []func()\n"+
				"\t\ts7ARRev17Retain(callbacks...)",
			"internal/cli/zz_s7_ar_rev17_inert_named.go",
			"package cli\n\nfunc s7ARRev17Retain(callbacks ...func()) {}\n",
		)
		if err != nil {
			t.Fatalf("PIB-518 inert named expansion failed closed: %v", err)
		}
	})

	t.Run("inert-field-expanded-slice", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\tvar callbacks []func()\n"+
				"\t\tholder := struct{ retain func(...func()) }{retain: func(callbacks ...func()) {}}\n"+
				"\t\tholder.retain(callbacks...)",
			"internal/cli/zz_s7_ar_rev17_inert_field.go",
			"package cli\n",
		)
		if err != nil {
			t.Fatalf("PIB-518 inert function-field expansion failed closed: %v", err)
		}
	})

	for _, fixture := range []struct {
		name        string
		insertion   string
		extraSource string
	}{
		{
			name: "positive-direct-method-value-index-one",
			insertion: "\n" +
				"\t\tvar dispatched s7ARRev17PositiveInterface = s7ARRev17PositiveInvoker{}\n" +
				"\t\tdispatched.invoke(func() {}, func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t})",
			extraSource: "package cli\n\n" +
				"type s7ARRev17PositiveInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev17PositiveInvoker struct{}\n\n" +
				"func (s7ARRev17PositiveInvoker) invoke(callbacks ...func()) { callbacks[1]() }\n",
		},
		{
			name: "positive-direct-method-expression-index-one",
			insertion: "\n" +
				"\t\tvar dispatched s7ARRev17PositiveInterface = s7ARRev17PositiveInvoker{}\n" +
				"\t\ts7ARRev17PositiveInterface.invoke(dispatched, func() {}, func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t})",
			extraSource: "package cli\n\n" +
				"type s7ARRev17PositiveInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev17PositiveInvoker struct{}\n\n" +
				"func (s7ARRev17PositiveInvoker) invoke(callbacks ...func()) { callbacks[1]() }\n",
		},
		{
			name: "positive-dynamic-index",
			insertion: "\n" +
				"\t\tvar dispatched s7ARRev17DynamicInterface = s7ARRev17DynamicInvoker{}\n" +
				"\t\tdispatched.invoke(1, func() {}, func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t})",
			extraSource: "package cli\n\n" +
				"type s7ARRev17DynamicInterface interface{ invoke(int, ...func()) }\n" +
				"type s7ARRev17DynamicInvoker struct{}\n\n" +
				"func (s7ARRev17DynamicInvoker) invoke(index int, callbacks ...func()) { callbacks[index]() }\n",
		},
		{
			name: "positive-range-traversal",
			insertion: "\n" +
				"\t\tvar dispatched s7ARRev17RangeInterface = s7ARRev17RangeInvoker{}\n" +
				"\t\tdispatched.invoke(func() {}, func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t})",
			extraSource: "package cli\n\n" +
				"type s7ARRev17RangeInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev17RangeInvoker struct{}\n\n" +
				"func (s7ARRev17RangeInvoker) invoke(callbacks ...func()) {\n" +
				"\tfor _, callback := range callbacks { callback() }\n" +
				"}\n",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(
				t,
				fixture.insertion,
				"internal/cli/zz_s7_ar_rev17_positive.go",
				fixture.extraSource,
			)
			actual, ok := s7ARInventoryActualWriteCount(err)
			if !ok || actual != 4 {
				t.Fatalf(
					"PIB-518 omitted %s authority: error=%v writes=%d counted=%t, want four canonical writes",
					fixture.name, err, actual, ok,
				)
			}
		})
	}

	for _, fixture := range []struct {
		name      string
		insertion string
	}{
		{
			name: "inverse-overwritten-index-initializer",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}}\n" +
				"\t\tcallbacks[0] = func() {}\n" +
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
		{
			name: "inverse-copy-overwrites-initializer",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}}\n" +
				"\t\treplacement := []func(){func() {}}\n" +
				"\t\tcopy(callbacks, replacement)\n" +
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
		{
			name: "inverse-alias-overwrites-initializer",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}}\n" +
				"\t\talias := callbacks\n" +
				"\t\talias[0] = func() {}\n" +
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(
				t,
				fixture.insertion,
				"internal/cli/zz_s7_ar_rev17_inverse_mutation.go",
				interfaceSource,
			)
			if err != nil {
				t.Fatalf("PIB-518 retained stale %s authority: %v", fixture.name, err)
			}
		})
	}

	t.Run("conditional-index-mutation-fails-closed", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\tcallbacks := []func(){func() {}}\n"+
				"\t\tif options.orphans {\n"+
				"\t\t\tcallbacks[0] = func() {\n"+
				"\t\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t\t}\n"+
				"\t\t}\n"+
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n"+
				"\t\tdispatched.invoke(callbacks...)",
			"internal/cli/zz_s7_ar_rev17_conditional_mutation.go",
			interfaceSource,
		)
		if err == nil || !strings.Contains(err.Error(), "unresolved callable origins") {
			t.Fatalf("PIB-518 conditional mutation = %v, want unresolved callable origins", err)
		}
	})

	t.Run("transitive-invoked-expansion-fails-closed", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\tvar callbacks []func()\n"+
				"\t\ts7ARRev17Forward(callbacks...)",
			"internal/cli/zz_s7_ar_rev17_transitive.go",
			"package cli\n\n"+
				"func s7ARRev17Forward(callbacks ...func()) { s7ARRev17Sink(callbacks...) }\n"+
				"func s7ARRev17Sink(callbacks ...func()) { callbacks[0]() }\n",
		)
		if err == nil || !strings.Contains(err.Error(), "unresolved callable origins") {
			t.Fatalf("PIB-518 transitive expansion = %v, want unresolved callable origins", err)
		}
	})

	t.Run("fixed-slice-invocation-remains-excluded", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\ts7ARRev17FixedSlice([]func(){func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t}})",
			"internal/cli/zz_s7_ar_rev17_fixed_slice.go",
			"package cli\n\n"+
				"func s7ARRev17FixedSlice(callbacks []func()) { callbacks[0]() }\n",
		)
		if err != nil {
			t.Fatalf("PIB-518 fixed []func() became variadic authority: %v", err)
		}
	})

	for _, fixture := range []struct {
		name      string
		insertion string
	}{
		{
			name: "append-route-fails-closed",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tcallbacks = append(callbacks, func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t})\n" +
				"\t\tvar dispatched s7ARRev17LaterInterface = s7ARRev17LaterInvoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
		{
			name: "reslice-alias-route-fails-closed",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}, func() {}}\n" +
				"\t\twindow := callbacks[1:]\n" +
				"\t\twindow[0] = func() {\n" +
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
				"\t\t}\n" +
				"\t\tvar dispatched s7ARRev17LaterInterface = s7ARRev17LaterInvoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(
				t,
				fixture.insertion,
				"internal/cli/zz_s7_ar_rev17_ambiguous_sequence.go",
				"package cli\n\n"+
					"type s7ARRev17LaterInterface interface{ invoke(...func()) }\n"+
					"type s7ARRev17LaterInvoker struct{}\n\n"+
					"func (s7ARRev17LaterInvoker) invoke(callbacks ...func()) { callbacks[1]() }\n",
			)
			if err == nil || !strings.Contains(err.Error(), "unresolved callable origins") {
				t.Fatalf("PIB-518 %s = %v, want unresolved callable origins", fixture.name, err)
			}
		})
	}

	t.Run("package-sequence-mutation-fails-closed", func(t *testing.T) {
		err := validate(
			t,
			"\n"+
				"\t\ts7ARRev17PackageCallbacks[0] = func() {\n"+
				"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n"+
				"\t\t}\n"+
				"\t\tvar dispatched s7ARRev17Interface = s7ARRev17Invoker{}\n"+
				"\t\tdispatched.invoke(s7ARRev17PackageCallbacks...)",
			"internal/cli/zz_s7_ar_rev17_package_sequence.go",
			interfaceSource+
				"\nvar s7ARRev17PackageCallbacks = []func(){func() {}}\n",
		)
		if err == nil || !strings.Contains(err.Error(), "unresolved callable origins") {
			t.Fatalf("PIB-518 package mutation = %v, want unresolved callable origins", err)
		}
	})
}

func TestS7ARRev18ReviewerRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-18 reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev18_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-18 reviewer repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const baseSource = "package cli\n\n" +
		"type s7ARRev18Interface interface{ invoke(...func()) }\n" +
		"type s7ARRev18Invoker struct{}\n\n" +
		"func (s7ARRev18Invoker) invoke(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev18Retain(callbacks ...func()) {}\n"
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"

	type expectation int
	const (
		wantClean expectation = iota
		wantFourWrites
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "selected-unknown-explicit-method-value",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callback)",
			extraSource: baseSource,
			want:        wantUnresolved,
			wantRoute:   "callbacks[0]",
		},
		{
			name: "selected-unknown-explicit-method-expression",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\ts7ARRev18Interface.invoke(dispatched, callback)",
			extraSource: baseSource,
			want:        wantUnresolved,
			wantRoute:   "callbacks[0]",
		},
		{
			name: "selected-known-explicit-inverse",
			insertion: "\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(" + canonical + ")",
			extraSource: baseSource,
			want:        wantFourWrites,
		},
		{
			name: "unknown-unselected-explicit-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(" + canonical + ", callback)",
			extraSource: baseSource,
			want:        wantFourWrites,
		},
		{
			name: "inert-unknown-explicit-transport",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\ts7ARRev18Retain(callback)",
			extraSource: baseSource,
			want:        wantClean,
		},
		{
			name: "selected-out-of-range-explicit",
			insertion: "\n" +
				"\t\tvar dispatched s7ARRev18OutOfRangeInterface = s7ARRev18OutOfRangeInvoker{}\n" +
				"\t\tdispatched.invoke(" + canonical + ")",
			extraSource: "package cli\n\n" +
				"type s7ARRev18OutOfRangeInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev18OutOfRangeInvoker struct{}\n" +
				"func (s7ARRev18OutOfRangeInvoker) invoke(callbacks ...func()) { callbacks[2]() }\n",
			want:      wantUnresolved,
			wantRoute: "callbacks[2]",
		},
		{
			name: "expanded-exact-unknown-unselected-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource,
			want:        wantFourWrites,
		},
		{
			name: "expanded-exact-selected-unknown-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\tvar dispatched s7ARRev18IndexOneInterface = s7ARRev18IndexOneInvoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: "package cli\n\n" +
				"type s7ARRev18IndexOneInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev18IndexOneInvoker struct{}\n" +
				"func (s7ARRev18IndexOneInvoker) invoke(callbacks ...func()) { callbacks[1]() }\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "expanded-dynamic-demand",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\tvar dispatched s7ARRev18DynamicInterface = s7ARRev18DynamicInvoker{}\n" +
				"\t\tdispatched.invoke(0, callbacks...)",
			extraSource: "package cli\n\n" +
				"type s7ARRev18DynamicInterface interface{ invoke(int, ...func()) }\n" +
				"type s7ARRev18DynamicInvoker struct{}\n" +
				"func (s7ARRev18DynamicInvoker) invoke(index int, callbacks ...func()) { callbacks[index]() }\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "expanded-range-demand",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\tvar dispatched s7ARRev18RangeInterface = s7ARRev18RangeInvoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: "package cli\n\n" +
				"type s7ARRev18RangeInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev18RangeInvoker struct{}\n" +
				"func (s7ARRev18RangeInvoker) invoke(callbacks ...func()) {\n" +
				"\tfor _, callback := range callbacks { callback() }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "explicit-dynamic-demand",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar dispatched s7ARRev18DynamicInterface = s7ARRev18DynamicInvoker{}\n" +
				"\t\tdispatched.invoke(0, " + canonical + ", callback)",
			extraSource: "package cli\n\n" +
				"type s7ARRev18DynamicInterface interface{ invoke(int, ...func()) }\n" +
				"type s7ARRev18DynamicInvoker struct{}\n" +
				"func (s7ARRev18DynamicInvoker) invoke(index int, callbacks ...func()) { callbacks[index]() }\n",
			want:      wantUnresolved,
			wantRoute: "callbacks[index]",
		},
		{
			name: "explicit-range-demand",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar dispatched s7ARRev18RangeInterface = s7ARRev18RangeInvoker{}\n" +
				"\t\tdispatched.invoke(" + canonical + ", callback)",
			extraSource: "package cli\n\n" +
				"type s7ARRev18RangeInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev18RangeInvoker struct{}\n" +
				"func (s7ARRev18RangeInvoker) invoke(callbacks ...func()) {\n" +
				"\tfor _, callback := range callbacks { callback() }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "callback",
		},
		{
			name: "transitive-exact-unknown-unselected-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev18Forward(callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev18Forward(callbacks ...func()) { s7ARRev18Sink(callbacks...) }\n" +
				"func s7ARRev18Sink(callbacks ...func()) { callbacks[0]() }\n",
			want: wantFourWrites,
		},
		{
			name: "transitive-alias-exact-unknown-unselected-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev18Forward(callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev18Forward(callbacks ...func()) {\n" +
				"\talias := callbacks\n" +
				"\ts7ARRev18Sink(alias...)\n" +
				"}\n" +
				"func s7ARRev18Sink(callbacks ...func()) { callbacks[0]() }\n",
			want: wantFourWrites,
		},
		{
			name: "transitive-element-alias-exact-unknown-unselected-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev18Forward(callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev18Forward(callbacks ...func()) {\n" +
				"\tselected := callbacks[0]\n" +
				"\tselected()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "transitive-exact-selected-unknown-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev18Forward(callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev18Forward(callbacks ...func()) { s7ARRev18Sink(callbacks...) }\n" +
				"func s7ARRev18Sink(callbacks ...func()) { callbacks[1]() }\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev18Forward",
		},
		{
			name: "method-expression-expanded-exact-unselected",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\ts7ARRev18Interface.invoke(dispatched, callbacks...)",
			extraSource: baseSource,
			want:        wantFourWrites,
		},
		{
			name: "local-helper-mutation-before-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tcanonical := " + canonical + "\n" +
				"\t\ts7ARRev18Mutate(callbacks, canonical)\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource +
				"func s7ARRev18Mutate(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "local-helper-alias-mutation-before-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tcanonical := " + canonical + "\n" +
				"\t\ts7ARRev18MutateAlias(callbacks, canonical)\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource +
				"func s7ARRev18MutateAlias(callbacks []func(), replacement func()) {\n" +
				"\talias := callbacks\n" +
				"\talias[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "unresolved-helper-mutation-before-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\ts7ARRev18UnknownMutation(callbacks)\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource +
				"var s7ARRev18UnknownMutation func([]func())\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "guaranteed-append-backing-reuse-before-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tvar replacement func()\n" +
				"\t\talias := append(callbacks[:0], replacement)\n" +
				"\t\t_ = alias\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource,
			want:        wantUnresolved,
			wantRoute:   "dispatched.invoke",
		},
		{
			name: "reslice-alias-mutation-before-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}, func() {}}\n" +
				"\t\talias := callbacks[1:]\n" +
				"\t\talias[0] = " + canonical + "\n" +
				"\t\tvar dispatched s7ARRev18IndexOneInterface = s7ARRev18IndexOneInvoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: "package cli\n\n" +
				"type s7ARRev18IndexOneInterface interface{ invoke(...func()) }\n" +
				"type s7ARRev18IndexOneInvoker struct{}\n" +
				"func (s7ARRev18IndexOneInvoker) invoke(callbacks ...func()) { callbacks[1]() }\n",
			want:      wantUnresolved,
			wantRoute: "dispatched.invoke",
		},
		{
			name: "helper-mutation-after-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)\n" +
				"\t\ts7ARRev18Mutate(callbacks, " + canonical + ")",
			extraSource: baseSource +
				"func s7ARRev18Mutate(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n",
			want: wantClean,
		},
		{
			name: "read-only-helper-before-sink",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\ts7ARRev18ReadOnly(callbacks)\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource +
				"func s7ARRev18ReadOnly(callbacks []func()) { _ = len(callbacks) }\n",
			want: wantClean,
		},
		{
			name: "helper-mutation-of-uninvoked-index",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}, func() {}}\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev18MutateIndexOne(callbacks, replacement)\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource +
				"func s7ARRev18MutateIndexOne(callbacks []func(), replacement func()) { callbacks[1] = replacement }\n",
			want: wantClean,
		},
		{
			name: "read-only-reslice-alias",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}, func() {}}\n" +
				"\t\talias := callbacks[1:]\n" +
				"\t\t_ = alias\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource,
			want:        wantClean,
		},
		{
			name: "alias-reassignment-disjoint-append",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tvar replacement func()\n" +
				"\t\talias := callbacks\n" +
				"\t\talias = append([]func(){}, replacement)\n" +
				"\t\t_ = alias\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource,
			want:        wantClean,
		},
		{
			name: "forced-append-allocation",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\tvar replacement func()\n" +
				"\t\talias := append(callbacks[:0:0], replacement)\n" +
				"\t\t_ = alias\n" +
				"\t\tvar dispatched s7ARRev18Interface = s7ARRev18Invoker{}\n" +
				"\t\tdispatched.invoke(callbacks...)",
			extraSource: baseSource,
			want:        wantClean,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantClean:
				if err != nil {
					t.Fatalf("PIB-518 %s = %v, want clean", fixture.name, err)
				}
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev19ReviewerRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-19 reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev19_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-19 reviewer repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev19Sink(callbacks ...func()) { callbacks[0]() }\n"

	type expectation int
	const (
		wantClean expectation = iota
		wantFourWrites
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "variadic-parameter-replaced-known-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceKnown(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceKnown(known func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "variadic-parameter-replaced-unknown-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar replacement func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceUnknown(replacement, callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceUnknown(replacement func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19Sink",
		},
		{
			name: "sequence-alias-replaced-known-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceSequenceAlias(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceSequenceAlias(known func(), callbacks ...func()) {\n" +
				"\talias := callbacks\n" +
				"\talias = []func(){known}\n" +
				"\ts7ARRev19Sink(alias...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "sequence-alias-reassignment-does-not-rewrite-original",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ForwardOriginalAfterAliasReplace(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ForwardOriginalAfterAliasReplace(known func(), callbacks ...func()) {\n" +
				"\talias := callbacks\n" +
				"\talias = []func(){known}\n" +
				"\t_ = alias\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19ForwardOriginalAfterAliasReplace",
		},
		{
			name: "element-alias-overwritten-known-before-invocation",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceElementKnown(" + canonical + ", callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev19ReplaceElementKnown(known func(), callbacks ...func()) {\n" +
				"\tselected := callbacks[0]\n" +
				"\tselected = known\n" +
				"\tselected()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "element-alias-overwritten-unknown-before-invocation",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar replacement func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceElementUnknown(replacement, callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev19ReplaceElementUnknown(replacement func(), callbacks ...func()) {\n" +
				"\tselected := callbacks[0]\n" +
				"\tselected = replacement\n" +
				"\tselected()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "selected",
		},
		{
			name: "assignment-after-forward-cannot-contaminate-earlier-call",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19AssignAfterForward(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19AssignAfterForward(known func(), callbacks ...func()) {\n" +
				"\talias := []func(){known}\n" +
				"\ts7ARRev19Sink(alias...)\n" +
				"\talias = callbacks\n" +
				"\t_ = alias\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "assignment-before-forward-affects-call",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19AssignBeforeForward(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19AssignBeforeForward(known func(), callbacks ...func()) {\n" +
				"\talias := []func(){known}\n" +
				"\talias = callbacks\n" +
				"\ts7ARRev19Sink(alias...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19AssignBeforeForward",
		},
		{
			name: "conditional-one-branch-replacement-retains-entry",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceOneBranch(true, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceOneBranch(condition bool, known func(), callbacks ...func()) {\n" +
				"\tif condition { callbacks = []func(){known} }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19ReplaceOneBranch",
		},
		{
			name: "conditional-all-branch-replacement-kills-entry",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceAllBranches(true, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceAllBranches(condition bool, known func(), callbacks ...func()) {\n" +
				"\tif condition { callbacks = []func(){known} } else { callbacks = []func(){known} }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "possibly-zero-loop-replacement-retains-entry",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceInMaybeLoop(false, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceInMaybeLoop(condition bool, known func(), callbacks ...func()) {\n" +
				"\tfor condition { callbacks = []func(){known}; condition = false }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19ReplaceInMaybeLoop",
		},
		{
			name: "loop-body-replacement-before-only-invocation",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceInsideInvokingLoop(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceInsideInvokingLoop(known func(), callbacks ...func()) {\n" +
				"\tfor index := 0; index < 1; index++ {\n" +
				"\t\tcallbacks = []func(){known}\n" +
				"\t\ts7ARRev19Sink(callbacks...)\n" +
				"\t}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multiple-calls-use-state-at-each-program-point",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19TwoResolvedCalls(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19TwoResolvedCalls(known func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"\tcallbacks = []func(){func() {}}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multiple-calls-later-unknown-remains-unresolved",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar replacement func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ResolvedThenUnknown(replacement, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ResolvedThenUnknown(replacement func(), known func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19Sink",
		},
		{
			name: "two-hop-replacement-preserves-program-order",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceBeforeMiddle(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19Middle(callbacks ...func()) { s7ARRev19Sink(callbacks...) }\n" +
				"func s7ARRev19ReplaceBeforeMiddle(known func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Middle(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "reslice-alias-replacement-kills-alias-route",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceResliceAlias(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceResliceAlias(known func(), callbacks ...func()) {\n" +
				"\talias := callbacks[:]\n" +
				"\talias = []func(){known}\n" +
				"\ts7ARRev19Sink(alias...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "reslice-alias-replacement-preserves-original",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19OriginalAfterResliceReplace(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19OriginalAfterResliceReplace(known func(), callbacks ...func()) {\n" +
				"\talias := callbacks[:]\n" +
				"\talias = []func(){known}\n" +
				"\t_ = alias\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19OriginalAfterResliceReplace",
		},
		{
			name: "exact-index-keeps-unknown-unselected-sibling-clean",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev19Sink(callbacks...)",
			extraSource: sinkSource,
			want:        wantFourWrites,
		},
		{
			name: "dynamic-index-demands-unknown-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev19Dynamic(0, callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev19Dynamic(index int, callbacks ...func()) { callbacks[index]() }\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19Dynamic",
		},
		{
			name: "range-demands-unknown-sibling",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){" + canonical + ", callback}\n" +
				"\t\ts7ARRev19Range(callbacks...)",
			extraSource: "package cli\n\n" +
				"func s7ARRev19Range(callbacks ...func()) {\n" +
				"\tfor _, callback := range callbacks { callback() }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "callback",
		},
		{
			name: "method-value-replacement-preserves-receiver-offset",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19Invoker{}.replace(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"type s7ARRev19Invoker struct{}\n" +
				"func (s7ARRev19Invoker) replace(known func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "method-expression-replacement-preserves-receiver-offset",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19Invoker.replace(s7ARRev19Invoker{}, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"type s7ARRev19Invoker struct{}\n" +
				"func (s7ARRev19Invoker) replace(known func(), callbacks ...func()) {\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "direct-element-overwrite-known-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19OverwriteElement(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19OverwriteElement(known func(), callbacks ...func()) {\n" +
				"\tcallbacks[0] = known\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "backing-alias-element-overwrite-known-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19OverwriteThroughAlias(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19OverwriteThroughAlias(known func(), callbacks ...func()) {\n" +
				"\talias := callbacks\n" +
				"\talias[0] = known\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "element-write-after-alias-reassignment-preserves-original",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19WriteAfterAliasReplace(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19WriteAfterAliasReplace(known func(), callbacks ...func()) {\n" +
				"\talias := callbacks\n" +
				"\talias = []func(){func() {}}\n" +
				"\talias[0] = known\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19WriteAfterAliasReplace",
		},
		{
			name: "returning-branch-does-not-reach-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceOrReturn(true, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceOrReturn(condition bool, known func(), callbacks ...func()) {\n" +
				"\tif condition { callbacks = []func(){known} } else { return }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "switch-all-cases-replace-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceSwitch(0, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceSwitch(choice int, known func(), callbacks ...func()) {\n" +
				"\tswitch choice {\n" +
				"\tcase 0: callbacks = []func(){known}\n" +
				"\tdefault: callbacks = []func(){known}\n" +
				"\t}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multi-assignment-uses-rhs-before-lhs",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19SwapSequence(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19SwapSequence(known func(), callbacks ...func()) {\n" +
				"\treplacement := []func(){known}\n" +
				"\tcallbacks, replacement = replacement, callbacks\n" +
				"\t_ = replacement\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "helper-disjoint-alias-write-is-inert",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\ts7ARRev19MutateDisjoint(callbacks, " + canonical + ")\n" +
				"\t\ts7ARRev19Sink(callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19MutateDisjoint(callbacks []func(), replacement func()) {\n" +
				"\talias := callbacks\n" +
				"\talias = []func(){func() {}}\n" +
				"\talias[0] = replacement\n" +
				"}\n",
			want: wantClean,
		},
		{
			name: "helper-unreachable-write-is-inert",
			insertion: "\n" +
				"\t\tcallbacks := []func(){func() {}}\n" +
				"\t\ts7ARRev19UnreachableMutation(callbacks, " + canonical + ")\n" +
				"\t\ts7ARRev19Sink(callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19UnreachableMutation(callbacks []func(), replacement func()) {\n" +
				"\treturn\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want: wantClean,
		},
		{
			name: "forwarding-function-alias-replaced-by-inert",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceForwarder(callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19Retain(callbacks ...func()) {}\n" +
				"func s7ARRev19ReplaceForwarder(callbacks ...func()) {\n" +
				"\tforward := s7ARRev19Sink\n" +
				"\tforward = s7ARRev19Retain\n" +
				"\tforward(callbacks...)\n" +
				"}\n",
			want: wantClean,
		},
		{
			name: "forwarding-function-alias-replaced-by-sink",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19SelectForwarder(callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19Retain(callbacks ...func()) {}\n" +
				"func s7ARRev19SelectForwarder(callbacks ...func()) {\n" +
				"\tforward := s7ARRev19Retain\n" +
				"\tforward = s7ARRev19Sink\n" +
				"\tforward(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19SelectForwarder",
		},
		{
			name: "later-forwarder-assignment-cannot-contaminate-earlier-call",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19AssignForwarderAfter(callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19Retain(callbacks ...func()) {}\n" +
				"func s7ARRev19AssignForwarderAfter(callbacks ...func()) {\n" +
				"\tforward := s7ARRev19Retain\n" +
				"\tforward(callbacks...)\n" +
				"\tforward = s7ARRev19Sink\n" +
				"\t_ = forward\n" +
				"}\n",
			want: wantClean,
		},
		{
			name: "shadowed-sequence-object-keeps-inner-state-separate",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ShadowSequence(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ShadowSequence(known func(), callbacks ...func()) {\n" +
				"\t{\n" +
				"\t\tcallbacks := []func(){known}\n" +
				"\t\ts7ARRev19Sink(callbacks...)\n" +
				"\t}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "inert-closure-body-is-not-declaration-time-flow",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19InertClosure(" + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19InertClosure(known func(), callbacks ...func()) {\n" +
				"\tclosure := func() { s7ARRev19Sink(callbacks...) }\n" +
				"\t_ = closure\n" +
				"\tcallbacks = []func(){known}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "direct-element-overwrite-unknown-remains-unresolved",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar replacement func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19OverwriteElementUnknown(replacement, callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19OverwriteElementUnknown(replacement func(), callbacks ...func()) {\n" +
				"\tcallbacks[0] = replacement\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19Sink",
		},
		{
			name: "conditional-correlated-known-and-unknown-alternatives",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tvar replacement func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceMixed(true, " + canonical + ", replacement, callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceMixed(condition bool, known func(), replacement func(), callbacks ...func()) {\n" +
				"\tif condition { callbacks = []func(){known} } else { callbacks = []func(){replacement} }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19Sink",
		},
		{
			name: "select-all-clauses-replace-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceSelect(make(chan struct{}), " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceSelect(signal <-chan struct{}, known func(), callbacks ...func()) {\n" +
				"\tselect {\n" +
				"\tcase <-signal: callbacks = []func(){known}\n" +
				"\tdefault: callbacks = []func(){known}\n" +
				"\t}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "type-switch-all-cases-replace-before-forward",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19ReplaceTypeSwitch(0, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19ReplaceTypeSwitch(value any, known func(), callbacks ...func()) {\n" +
				"\tswitch value.(type) {\n" +
				"\tcase int: callbacks = []func(){known}\n" +
				"\tdefault: callbacks = []func(){known}\n" +
				"\t}\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "conditional-all-branch-element-overwrite-kills-entry",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19OverwriteAllBranches(true, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19OverwriteAllBranches(condition bool, known func(), callbacks ...func()) {\n" +
				"\tif condition { callbacks[0] = known } else { callbacks[0] = known }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "conditional-one-branch-element-overwrite-retains-entry",
			insertion: "\n" +
				"\t\tvar callback func()\n" +
				"\t\tcallbacks := []func(){callback}\n" +
				"\t\ts7ARRev19OverwriteOneBranch(true, " + canonical + ", callbacks...)",
			extraSource: sinkSource +
				"func s7ARRev19OverwriteOneBranch(condition bool, known func(), callbacks ...func()) {\n" +
				"\tif condition { callbacks[0] = known }\n" +
				"\ts7ARRev19Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev19OverwriteOneBranch",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantClean:
				if err != nil {
					t.Fatalf("PIB-518 %s = %v, want clean", fixture.name, err)
				}
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev20ReviewerRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-20 reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev20_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-20 reviewer repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev20Sink(callbacks ...func()) { callbacks[0]() }\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "defer-expanded-direct-backing-overwrite",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferDirect(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferDirect(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "defer-expanded-alias-backing-overwrite",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferAlias(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferAlias(replacement func(), callbacks ...func()) {\n" +
				"\talias := callbacks[:]\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\talias[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "go-expanded-concurrent-backing-overwrite",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20GoDirect(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20GoDirect(replacement func(), callbacks ...func()) {\n" +
				"\tgo s7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "nonvariadic-scalar-post-call-assignment-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20ScalarAfter(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20ScalarAfter(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback()\n" +
				"\tcallback = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "nonvariadic-fixed-slice-post-call-write-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20FixedAfter([]func(){" + canonical + "}, replacement)",
			extraSource: sinkSource +
				"func s7ARRev20FixedAfter(callbacks []func(), replacement func()) {\n" +
				"\ts7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-expanded-known-without-mutation",
			insertion: "\n" +
				"\t\ts7ARRev20DeferKnown(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferKnown(callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "go-expanded-known-without-mutation",
			insertion: "\n" +
				"\t\ts7ARRev20GoKnown(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20GoKnown(callbacks ...func()) {\n" +
				"\tgo s7ARRev20Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-descriptor-reassignment-is-disjoint",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferDescriptor(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferDescriptor(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "go-descriptor-reassignment-is-disjoint",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20GoDescriptor(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20GoDescriptor(replacement func(), callbacks ...func()) {\n" +
				"\tgo s7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-explicit-scalar-is-value-captured",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferScalar(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20DeferScalar(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tdefer callback()\n" +
				"\tcallback = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-unknown-write-before-capture",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferBefore(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferBefore(replacement func(), callbacks ...func()) {\n" +
				"\tcallbacks[0] = replacement\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "go-unknown-statement-target-not-laundered",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20GoUnknownThenKnown(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20GoUnknownThenKnown(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\tgo s7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks[0] = known\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "nonvariadic-scalar-pre-call-assignment-is-observed",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20ScalarBefore(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20ScalarBefore(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback = replacement\n" +
				"\tcallback()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "callback",
		},
		{
			name: "nonvariadic-fixed-slice-pre-call-write-is-observed",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20FixedBefore([]func(){" + canonical + "}, replacement)",
			extraSource: sinkSource +
				"func s7ARRev20FixedBefore(callbacks []func(), replacement func()) {\n" +
				"\tcallbacks[0] = replacement\n" +
				"\ts7ARRev20Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "nonvariadic-multiple-calls-use-own-program-point",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20ScalarCalls(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20ScalarCalls(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback()\n" +
				"\tcallback = func() {}\n" +
				"\tcallback()\n" +
				"\tcallback = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-one-branch-backing-mutation-retains-unknown",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferOneBranch(true, replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferOneBranch(condition bool, replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tif condition { callbacks[0] = replacement }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "defer-all-path-known-overwrite-kills-entry",
			insertion: "\n" +
				"\t\ts7ARRev20DeferAllPaths(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferAllPaths(known func()) {\n" +
				"\tcallbacks := []func(){func() {}}\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tif len(callbacks) == 1 { callbacks[0] = known } else { callbacks[0] = known }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-unreachable-post-return-write-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferReturn(" + canonical + ", replacement)",
			extraSource: sinkSource +
				"func s7ARRev20DeferReturn(known, replacement func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\treturn\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-disjoint-alias-write-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferDisjoint(" + canonical + ", replacement)",
			extraSource: sinkSource +
				"func s7ARRev20DeferDisjoint(known, replacement func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\talias := []func(){func() {}}\n" +
				"\talias[0] = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-copy-backing-mutation-is-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferCopy(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferCopy(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tcopy(callbacks, []func(){replacement})\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "defer-copy-disjoint-offset-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferCopyDisjoint(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferCopyDisjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known, func() {}}\n" +
				"\tdefer s7ARRev20Sink(callbacks[:1]...)\n" +
				"\tcopy(callbacks[1:], []func(){replacement})\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-append-reused-backing-is-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferAppendReuse(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferAppendReuse(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\talias := append(callbacks[:0], replacement)\n" +
				"\t_ = alias\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "defer-append-beyond-captured-length-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferAppendBeyond(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferAppendBeyond(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\talias := append(callbacks, replacement)\n" +
				"\t_ = alias\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-local-helper-backing-mutation-is-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferHelper(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20Mutate(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n" +
				"func s7ARRev20DeferHelper(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\ts7ARRev20Mutate(callbacks, replacement)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "defer-reslice-offset-backing-mutation-is-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferOffset(replacement, func() {}, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferOffset(replacement func(), callbacks ...func()) {\n" +
				"\talias := callbacks[1:]\n" +
				"\tdefer s7ARRev20Sink(alias...)\n" +
				"\tcallbacks[1] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "defer-forced-append-allocation-is-disjoint",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferAppendDetached(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferAppendDetached(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\talias := append(callbacks[:0:0], replacement)\n" +
				"\t_ = alias\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-read-only-helper-is-inert",
			insertion: "\n" +
				"\t\ts7ARRev20DeferReadOnly(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20ReadOnly(callbacks []func()) { _ = len(callbacks) }\n" +
				"func s7ARRev20DeferReadOnly(callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\ts7ARRev20ReadOnly(callbacks)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "deferred-mutator-is-conservatively-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferredMutator(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferredWrite(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n" +
				"func s7ARRev20DeferredMutator(replacement func(), callbacks ...func()) {\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tdefer s7ARRev20DeferredWrite(callbacks, replacement)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "go-helper-backing-mutation-is-concurrent",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20GoHelper(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20GoMutate(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n" +
				"func s7ARRev20GoHelper(replacement func(), callbacks ...func()) {\n" +
				"\tgo s7ARRev20Sink(callbacks...)\n" +
				"\ts7ARRev20GoMutate(callbacks, replacement)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "go-rhs-mutation-before-descriptor-reassignment-is-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20GoRHS(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20MutateAndDetach(callbacks []func(), replacement func()) []func() {\n" +
				"\tcallbacks[0] = replacement\n" +
				"\treturn []func(){func() {}}\n" +
				"}\n" +
				"func s7ARRev20GoRHS(replacement func(), callbacks ...func()) {\n" +
				"\tgo s7ARRev20Sink(callbacks...)\n" +
				"\tcallbacks = s7ARRev20MutateAndDetach(callbacks, replacement)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "invoked-closure-backing-mutation-is-visible",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferClosure(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferClosure(replacement func(), callbacks ...func()) {\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"\tmutate()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev20Sink",
		},
		{
			name: "declared-but-inert-closure-does-not-mutate",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20DeferInertClosure(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev20DeferInertClosure(replacement func(), callbacks ...func()) {\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\t_ = mutate\n" +
				"\tdefer s7ARRev20Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "nonvariadic-scalar-tuple-swap-uses-rhs-state",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20ScalarSwap(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20ScalarSwap(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback, replacement = replacement, callback\n" +
				"\tcallback()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "callback",
		},
		{
			name: "nonvariadic-shadowed-scalar-object-is-distinct",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20ScalarShadow(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20ScalarShadow(known, replacement func()) {\n" +
				"\tcallback := replacement\n" +
				"\t{\n" +
				"\t\tcallback := known\n" +
				"\t\tcallback()\n" +
				"\t}\n" +
				"\t_ = callback\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "nonvariadic-method-value-post-call-assignment-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20Invoker{}.scalarAfter(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"type s7ARRev20Invoker struct{}\n" +
				"func (s7ARRev20Invoker) scalarAfter(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback()\n" +
				"\tcallback = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "nonvariadic-method-expression-post-call-assignment-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20Invoker.scalarAfter(s7ARRev20Invoker{}, " + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"type s7ARRev20Invoker struct{}\n" +
				"func (s7ARRev20Invoker) scalarAfter(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback()\n" +
				"\tcallback = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "nonvariadic-two-hop-post-call-assignment-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev20ScalarOuter(" + canonical + ", replacement)",
			extraSource: "package cli\n\n" +
				"func s7ARRev20ScalarMiddle(known, replacement func()) {\n" +
				"\tcallback := known\n" +
				"\tcallback()\n" +
				"\tcallback = replacement\n" +
				"}\n" +
				"func s7ARRev20ScalarOuter(known, replacement func()) {\n" +
				"\ts7ARRev20ScalarMiddle(known, replacement)\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev21ReviewerRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-21 reviewer repro anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev21_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-21 reviewer repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev21Sink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev21Write(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n" +
		"func s7ARRev21Noop(callbacks []func()) { _ = len(callbacks) }\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "async-helper-caller-shared-backing-write",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21OuterShared(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21Launch(callbacks []func()) { go s7ARRev21Sink(callbacks...) }\n" +
				"func s7ARRev21OuterShared(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21Launch(callbacks)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Launch",
		},
		{
			name: "async-helper-without-caller-mutation",
			insertion: "\n" +
				"\t\ts7ARRev21OuterKnown(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21Launch(callbacks []func()) { go s7ARRev21Sink(callbacks...) }\n" +
				"func s7ARRev21OuterKnown(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21Launch(callbacks)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-helper-disjoint-caller-backing-write",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21OuterDisjoint(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21Launch(callbacks []func()) { go s7ARRev21Sink(callbacks...) }\n" +
				"func s7ARRev21OuterDisjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tother := []func(){func() {}}\n" +
				"\ts7ARRev21Launch(callbacks)\n" +
				"\tother[0] = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-helper-descriptor-reassignment-is-disjoint",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21OuterDescriptor(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21Launch(callbacks []func()) { go s7ARRev21Sink(callbacks...) }\n" +
				"func s7ARRev21OuterDescriptor(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21Launch(callbacks)\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"\t_ = callbacks\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-helper-two-hop-caller-shared-backing-write",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21OuterTwoHop(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21Launch(callbacks ...func()) { go s7ARRev21Sink(callbacks...) }\n" +
				"func s7ARRev21Middle(callbacks ...func()) { s7ARRev21Launch(callbacks...) }\n" +
				"func s7ARRev21OuterTwoHop(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21Middle(callbacks...)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Middle",
		},
		{
			name: "async-statement-unknown-is-not-laundered",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21OuterUnknown(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21Launch(callbacks []func()) { go s7ARRev21Sink(callbacks...) }\n" +
				"func s7ARRev21OuterUnknown(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ts7ARRev21Launch(callbacks)\n" +
				"\tcallbacks[0] = known\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Sink",
		},
		{
			name: "deferred-zero-argument-closure-capture",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21DeferredCapture(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21DeferredCapture(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"\tdefer func() { callbacks[0] = replacement }()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Sink",
		},
		{
			name: "go-zero-argument-closure-capture",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21GoCapture(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21GoCapture(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tgo s7ARRev21Sink(callbacks...)\n" +
				"\tgo func() { callbacks[0] = replacement }()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Sink",
		},
		{
			name: "inert-closure-declaration-remains-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21InertClosure(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21InertClosure(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\t_ = mutate\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "deferred-explicit-slice-parameter-captures-descriptor",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ExplicitParameter(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ExplicitParameter(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"\tdefer func(xs []func()) { xs[0] = replacement }(callbacks)\n" +
				"\tcallbacks = []func(){func() {}}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Sink",
		},
		{
			name: "deferred-captured-slice-variable-observes-reassignment",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21CapturedDescriptor(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21CapturedDescriptor(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"\tdefer func() { callbacks[0] = replacement }()\n" +
				"\tcallbacks = []func(){func() {}}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "defer-mutator-executes-before-sink",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21MutatorBeforeSink(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21MutatorBeforeSink(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"\tdefer s7ARRev21Write(callbacks, replacement)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21Sink",
		},
		{
			name: "defer-sink-executes-before-mutator",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21SinkBeforeMutator(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21SinkBeforeMutator(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21Write(callbacks, replacement)\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "three-defers-are-processed-sequentially",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ThreeDefers(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ThreeDefers(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21Write(callbacks, replacement)\n" +
				"\tdefer s7ARRev21Sink(callbacks...)\n" +
				"\tdefer s7ARRev21Noop(callbacks)\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev23ScheduledClosureAliasRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-23 scheduled closure alias anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev23_alias_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-23 alias repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev23Sink(callbacks ...func()) { callbacks[0]() }\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "deferred-zero-argument-alias-mutator",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23DeferredAlias(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23DeferredAlias(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer mutate()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23Sink",
		},
		{
			name: "async-zero-argument-alias-captured-sink",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23AsyncAlias(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23AsyncAlias(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tlaunch := func() { s7ARRev23Sink(callbacks...) }\n" +
				"\tgo launch()\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23Sink",
		},
		{
			name: "registered-unsafe-target-remains-unsafe-after-safe-reassignment",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23UnsafeThenSafe(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23UnsafeThenSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tunsafe := func() { callbacks[0] = replacement }\n" +
				"\tsafe := func() {}\n" +
				"\ttarget := unsafe\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = safe\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23Sink",
		},
		{
			name: "registered-safe-target-remains-safe-after-unsafe-reassignment",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23SafeThenUnsafe(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23SafeThenUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tsafe := func() {}\n" +
				"\tunsafe := func() { callbacks[0] = replacement }\n" +
				"\ttarget := safe\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = unsafe\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "reassignment-before-registration-uses-new-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23BeforeRegistration(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23BeforeRegistration(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tsafe := func() {}\n" +
				"\tunsafe := func() { callbacks[0] = replacement }\n" +
				"\ttarget := safe\n" +
				"\ttarget = unsafe\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23Sink",
		},
		{
			name: "unresolved-alias-target-fails-closed",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23UnresolvedAlias(true, replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23UnresolvedAlias(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {}\n" +
				"\tif condition { target = replacement }\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23UnresolvedAlias",
		},
		{
			name: "inert-alias-declaration-remains-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23InertAlias(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23InertAlias(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\t_ = mutate\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "scheduled-alias-without-mutation-retains-authority",
			insertion: "\n" +
				"\t\ts7ARRev23NoMutation(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23NoMutation(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tinspect := func() { _ = len(callbacks) }\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer inspect()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "captured-descriptor-reassignment-is-distinct",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23DescriptorOnly(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23DescriptorOnly(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer mutate()\n" +
				"\tcallbacks = []func(){func() {}}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "scheduled-alias-disjoint-backing-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23Disjoint(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23Disjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tother := []func(){func() {}}\n" +
				"\tmutate := func() { other[0] = replacement }\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer mutate()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "captured-free-callable-reassignment-is-by-reference",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23CapturedCallable(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23CapturedCallable(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvalue := known\n" +
				"\tmutate := func() { callbacks[0] = value }\n" +
				"\tdefer s7ARRev23Sink(callbacks...)\n" +
				"\tdefer mutate()\n" +
				"\tvalue = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23Sink",
		},
		{
			name: "async-alias-without-later-mutation-retains-authority",
			insertion: "\n" +
				"\t\ts7ARRev23AsyncKnown(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23AsyncKnown(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tlaunch := func() { s7ARRev23Sink(callbacks...) }\n" +
				"\tgo launch()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-alias-captured-descriptor-reassignment-is-by-reference",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23AsyncDescriptor(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23AsyncDescriptor(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tlaunch := func() { s7ARRev23Sink(callbacks...) }\n" +
				"\tgo launch()\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"\t_ = callbacks\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23Sink",
		},
		{
			name: "async-registered-function-reassignment-is-value-captured",
			insertion: "\n" +
				"\t\ts7ARRev23AsyncFunctionReassignment(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23AsyncFunctionReassignment(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tlaunch := func() { s7ARRev23Sink(callbacks...) }\n" +
				"\tgo launch()\n" +
				"\tlaunch = func() {}\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev24ScheduledSelfCaptureRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-24 scheduled self-capture anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev24_self_capture.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-24 self-capture repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev24Sink(callbacks ...func()) { callbacks[0]() }\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "self-capturing-registered-unsafe-remains-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24UnsafeThenSafe(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev24UnsafeThenSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tunsafe := func() { callbacks[0] = replacement; _ = target }\n" +
				"\tsafe := func() {}\n" +
				"\ttarget = unsafe\n" +
				"\tdefer s7ARRev24Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = safe\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24Sink",
		},
		{
			name: "self-capturing-registered-safe-remains-safe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24SafeThenUnsafe(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev24SafeThenUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tsafe := func() { _ = target }\n" +
				"\tunsafe := func() { callbacks[0] = replacement }\n" +
				"\ttarget = safe\n" +
				"\tdefer s7ARRev24Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = unsafe\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev24StaticChallenges(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-24 static challenge anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev24_challenge.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-24 challenge is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"func s7ARRev24ChallengeSink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev24ChallengeNamedNoop() {}\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "parenthesized-multi-hop-self-capture-unsafe-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeParenthesizedUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeParenthesizedUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tunsafe := func() { callbacks[0] = replacement; _ = target }\n" +
				"\tfirst := unsafe\n" +
				"\tsecond := first\n" +
				"\ttarget = second\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer (((target)))()\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "parenthesized-multi-hop-self-capture-safe-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeParenthesizedSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeParenthesizedSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tsafe := func() { _ = target }\n" +
				"\tfirst := safe\n" +
				"\tsecond := first\n" +
				"\ttarget = second\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer (((target)))()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-self-capture-registered-unsafe-remains-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeAsyncUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeAsyncUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tunsafe := func() { callbacks[0] = replacement; _ = target }\n" +
				"\ttarget = unsafe\n" +
				"\tgo target()\n" +
				"\ttarget = func() {}\n" +
				"\ts7ARRev24ChallengeSink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "async-self-capture-registered-safe-remains-safe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeAsyncSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeAsyncSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tsafe := func() { _ = target }\n" +
				"\ttarget = safe\n" +
				"\tgo target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"\ts7ARRev24ChallengeSink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "stored-outer-target-uses-refreshed-inner-callable",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeInnerUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeInnerUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\touter := func() { target() }\n" +
				"\ttarget = outer\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "stored-outer-target-uses-refreshed-safe-inner-callable",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeInnerSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeInnerSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\touter := func() { target() }\n" +
				"\ttarget = outer\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() {}\n" +
				"\t_ = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "three-defer-lifo-keeps-registered-unsafe-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeLIFO(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeLIFO(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\ttarget = func() { callbacks[0] = replacement; _ = target }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer s7ARRev24ChallengeNamedNoop()\n" +
				"\tdefer target()\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "stored-target-executes-nested-deferred-mutation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeNestedDeferUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeNestedDeferUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\touter := func() { defer func() { callbacks[0] = replacement }(); _ = target }\n" +
				"\ttarget = outer\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "stored-target-keeps-nested-deferred-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeNestedDeferSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeNestedDeferSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\touter := func() { defer func() { _ = target }() }\n" +
				"\ttarget = outer\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "stored-target-publishes-nested-async-mutation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeNestedAsyncUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeNestedAsyncUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\touter := func() { go func() { callbacks[0] = replacement }(); _ = target }\n" +
				"\ttarget = outer\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "stored-target-keeps-nested-async-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeNestedAsyncSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeNestedAsyncSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\touter := func() { go func() { _ = target }() }\n" +
				"\ttarget = outer\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "explicit-parameter-is-frozen-while-free-capture-refreshes",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeExplicitSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeExplicitSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tfree := known\n" +
				"\trun := func(value func()) { callbacks[0] = value; _ = free }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer run(known)\n" +
				"\tfree = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "refreshed-free-capture-is-not-frozen-with-parameter",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeExplicitUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeExplicitUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tfree := known\n" +
				"\trun := func(value func()) { callbacks[0] = free; _ = value }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer run(known)\n" +
				"\tfree = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "branch-joined-registration-retains-unsafe-alternative",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeBranchUnsafe(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeBranchUnsafe(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tsafe := func() { _ = target }\n" +
				"\tunsafe := func() { callbacks[0] = replacement; _ = target }\n" +
				"\tif condition { target = safe } else { target = unsafe }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = safe\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeSink",
		},
		{
			name: "branch-joined-safe-registration-remains-exact",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeBranchSafe(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeBranchSafe(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tleft := func() { _ = target }\n" +
				"\tright := func() { _ = target; _ = len(callbacks) }\n" +
				"\tif condition { target = left } else { target = right }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "mixed-unresolved-registration-target-fails-closed",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeMixed(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeMixed(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\ttarget = func() { _ = target }\n" +
				"\tif condition { target = replacement }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev24ChallengeMixed",
		},
		{
			name: "separate-call-sites-keep-registration-snapshots",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeCallSites(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeCallSites(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar first, second func()\n" +
				"\tfirst = func() { _ = first }\n" +
				"\tsecond = func() { _ = second }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer first()\n" +
				"\tdefer second()\n" +
				"\tfirst = func() { callbacks[0] = replacement }\n" +
				"\tsecond = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "direct-named-and-inert-controls-remain-exact",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeControls(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeControls(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tinert := func() { callbacks[0] = replacement }\n" +
				"\t_ = inert\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer s7ARRev24ChallengeNamedNoop()\n" +
				"\tdefer func() { _ = len(callbacks) }()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "descriptor-only-replacement-keeps-old-sink-backing",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeDescriptor(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeDescriptor(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\ttarget = func() { callbacks[0] = replacement; _ = target }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\tcallbacks = []func(){func() {}}\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "disjoint-backing-self-capture-remains-exact",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev24ChallengeDisjoint(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev24ChallengeDisjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tother := []func(){func() {}}\n" +
				"\tvar target func()\n" +
				"\ttarget = func() { other[0] = replacement; _ = target }\n" +
				"\tdefer s7ARRev24ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev25StoredLiteralReturnRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-25 stored literal return anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev25_return_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-25 return repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"func s7ARRev25Sink(callbacks ...func()) { callbacks[0]() }\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "deferred-stored-literal-mutation-before-return",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25DeferredReturnUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25DeferredReturnUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { callbacks[0] = replacement; return }\n" +
				"\tdefer s7ARRev25Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25Sink",
		},
		{
			name: "deferred-stored-literal-safe-return",
			insertion: "\n" +
				"\t\ts7ARRev25DeferredReturnSafe(" + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25DeferredReturnSafe(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { _ = len(callbacks); return }\n" +
				"\tdefer s7ARRev25Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-stored-literal-mutation-before-return",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25AsyncReturnUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25AsyncReturnUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { callbacks[0] = replacement; return }\n" +
				"\tgo target()\n" +
				"\ts7ARRev25Sink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25Sink",
		},
		{
			name: "async-stored-literal-safe-return",
			insertion: "\n" +
				"\t\ts7ARRev25AsyncReturnSafe(" + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25AsyncReturnSafe(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { _ = len(callbacks); return }\n" +
				"\tgo target()\n" +
				"\ts7ARRev25Sink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "stored-literal-return-runs-nested-defer-before-outer-sink",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25NestedDeferredReturnUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25NestedDeferredReturnUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tdefer func() { callbacks[0] = replacement }()\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev25Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25Sink",
		},
		{
			name: "stored-literal-return-runs-safe-nested-defer",
			insertion: "\n" +
				"\t\ts7ARRev25NestedDeferredReturnSafe(" + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25NestedDeferredReturnSafe(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tdefer func() { _ = len(callbacks) }()\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev25Sink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, batch := range []struct {
		name string
		want expectation
	}{
		{name: "unsafe-route-batch", want: wantUnresolved},
		{name: "safe-inverse-batch", want: wantFourWrites},
	} {
		t.Run(batch.name, func(t *testing.T) {
			insertion := "\n\t\tvar replacement func()\n\t\t_ = replacement\n\t\tknown := " + canonical
			extraSource := supportSource
			var routes []string
			members := 0
			for index, fixture := range fixtures {
				if fixture.want != batch.want {
					continue
				}
				call := strings.Replace(
					fixture.insertion,
					"\n\t\tvar replacement func()\n", "", 1,
				)
				call = strings.Replace(call, canonical, "known", 1)
				insertion += "\n" + strings.TrimPrefix(call, "\n")
				source := strings.TrimPrefix(fixture.extraSource, supportSource)
				if source == fixture.extraSource {
					t.Fatalf("PIB-518 %s did not share canonical support source", fixture.name)
				}
				route := fixture.wantRoute
				if batch.want == wantUnresolved && route == "s7ARRev25Sink" {
					route = fmt.Sprintf("s7ARRev25SinkBatch%d", index)
					source = strings.ReplaceAll(
						source, "s7ARRev25Sink", route,
					)
					extraSource += fmt.Sprintf(
						"func %s(callbacks ...func()) { callbacks[0]() }\n",
						route,
					)
				}
				extraSource += source
				if route != "" {
					routes = append(routes, route)
				}
				members++
			}
			if members != 3 {
				t.Fatalf("PIB-518 %s has %d members, want 3", batch.name, members)
			}
			err := validate(t, insertion, extraSource)
			switch batch.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						batch.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins",
						batch.name, err,
					)
				}
				for _, route := range routes {
					if !strings.Contains(err.Error(), route) {
						t.Errorf(
							"PIB-518 %s omitted unresolved route %q: %v",
							batch.name, route, err,
						)
					}
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						batch.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", batch.name, batch.want)
			}
		})
	}
}

func TestS7ARRev25StaticChallenges(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-25 static challenge anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev25_challenge.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-25 challenge is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"func s7ARRev25ChallengeSink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev25ChallengeMutate(callbacks []func(), replacement func()) int {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn 1\n" +
		"}\n" +
		"func s7ARRev25ChallengeEvaluate(callbacks []func()) int { return len(callbacks) }\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "nested-block-return-preserves-unsafe-terminal",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeNestedBlockUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeNestedBlockUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\t{ callbacks[0] = replacement; return }\n" +
				"\t\tcallbacks[0] = known\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "nested-block-return-does-not-fall-through",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeNestedBlockSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeNestedBlockSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\t{ return }\n" +
				"\t\tcallbacks[0] = replacement\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "if-return-terminal-survives-safe-fallthrough",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeIfUnsafe(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeIfUnsafe(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tif condition { callbacks[0] = replacement; return }\n" +
				"\t\t_ = len(callbacks)\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "if-return-and-fallthrough-safe-inverse",
			insertion: "\n" +
				"\t\ts7ARRev25ChallengeIfSafe(true, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeIfSafe(condition bool, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tif condition { return }\n" +
				"\t\t_ = len(callbacks)\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "for-return-terminal-never-loops-back",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeForUnsafe(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeForUnsafe(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tfor condition { callbacks[0] = replacement; return; callbacks[0] = known }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "for-return-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeForSafe(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeForSafe(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tfor condition { return; callbacks[0] = replacement }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "range-return-terminal-never-loops-back",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeRangeUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeRangeUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tfor range []int{1} { callbacks[0] = replacement; return; callbacks[0] = known }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "range-return-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeRangeSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeRangeSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tfor range []int{1} { return; callbacks[0] = replacement }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "switch-return-terminal-propagates",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeSwitchUnsafe(1, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeSwitchUnsafe(selector int, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tswitch selector { case 1: callbacks[0] = replacement; return; callbacks[0] = known; default: }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "switch-return-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeSwitchSafe(1, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeSwitchSafe(selector int, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tswitch selector { case 1: return; callbacks[0] = replacement; default: }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "type-switch-return-terminal-propagates",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeTypeSwitchUnsafe(1, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeTypeSwitchUnsafe(value any, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tswitch value.(type) { case int: callbacks[0] = replacement; return; callbacks[0] = known; default: }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "type-switch-return-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeTypeSwitchSafe(1, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeTypeSwitchSafe(value any, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tswitch value.(type) { case int: return; callbacks[0] = replacement; default: }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "select-return-terminal-propagates",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeSelectUnsafe(make(chan struct{}), replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeSelectUnsafe(done <-chan struct{}, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case <-done: callbacks[0] = replacement; return; callbacks[0] = known; default: }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "select-return-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeSelectSafe(make(chan struct{}), replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeSelectSafe(done <-chan struct{}, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case <-done: return; callbacks[0] = replacement; default: }\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "return-expression-mutation-precedes-defers",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeReturnExprUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeReturnExprUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() int { return s7ARRev25ChallengeMutate(callbacks, replacement) }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "return-expression-safe-evaluation-precedes-defers",
			insertion: "\n" +
				"\t\ts7ARRev25ChallengeReturnExprSafe(" + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeReturnExprSafe(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() int { return s7ARRev25ChallengeEvaluate(callbacks) }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multiple-return-sites-keep-unsafe-terminal",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeMultipleReturnsUnsafe(true, false, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeMultipleReturnsUnsafe(left, right bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tif left { callbacks[0] = replacement; return }\n" +
				"\t\tif right { return }\n" +
				"\t\t_ = len(callbacks)\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "multiple-return-sites-and-natural-safe-inverse",
			insertion: "\n" +
				"\t\ts7ARRev25ChallengeMultipleReturnsSafe(true, false, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeMultipleReturnsSafe(left, right bool, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tif left { return }\n" +
				"\t\tif right { return }\n" +
				"\t\t_ = len(callbacks)\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "return-runs-nested-defers-in-lifo-order",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeNestedLIFOUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeNestedLIFOUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tdefer func() { callbacks[0] = replacement }()\n" +
				"\t\tdefer func() { callbacks[0] = known }()\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "return-runs-nested-defers-once-safe-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeNestedLIFOSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeNestedLIFOSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tdefer func() { callbacks[0] = known }()\n" +
				"\t\tdefer func() { callbacks[0] = replacement }()\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "return-preserves-nested-async-publication",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeNestedAsyncUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeNestedAsyncUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { go func() { callbacks[0] = replacement }(); return }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "return-preserves-nested-async-safe-inverse",
			insertion: "\n" +
				"\t\ts7ARRev25ChallengeNestedAsyncSafe(" + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeNestedAsyncSafe(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { go func() { _ = len(callbacks) }(); return }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "self-capturing-return-keeps-registered-unsafe-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeSelfCaptureUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeSelfCaptureUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tunsafe := func() { callbacks[0] = replacement; _ = target; return }\n" +
				"\ttarget = unsafe\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "self-capturing-return-keeps-registered-safe-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeSelfCaptureSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeSelfCaptureSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tsafe := func() { _ = target; return }\n" +
				"\ttarget = safe\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "branch-joined-returning-target-keeps-unsafe-alternative",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeBranchTargetsUnsafe(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeBranchTargetsUnsafe(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tif condition { target = func() { return } } else { target = func() { callbacks[0] = replacement; return } }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "branch-joined-returning-safe-targets-remain-exact",
			insertion: "\n" +
				"\t\ts7ARRev25ChallengeBranchTargetsSafe(true, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeBranchTargetsSafe(condition bool, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\tif condition { target = func() { return } } else { target = func() { _ = len(callbacks); return } }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "recursive-returning-stored-target-fails-closed",
			insertion: "\n" +
				"\t\ts7ARRev25ChallengeRecursive(" + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeRecursive(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar target func()\n" +
				"\ttarget = func() { defer target(); return }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeSink",
		},
		{
			name: "unresolved-returning-target-fails-closed",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeUnresolved(true, replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeUnresolved(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { return }\n" +
				"\tif condition { target = replacement }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev25ChallengeUnresolved",
		},
		{
			name: "returning-target-on-disjoint-backing-remains-exact",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeDisjoint(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeDisjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tother := []func(){func() {}}\n" +
				"\ttarget := func() { other[0] = replacement; return }\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "inert-returning-literal-remains-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev25ChallengeInert(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev25ChallengeInert(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { callbacks[0] = replacement; return }\n" +
				"\t_ = target\n" +
				"\tdefer s7ARRev25ChallengeSink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, batch := range []struct {
		name    string
		want    expectation
		members int
	}{
		{name: "unsafe-route-batch", want: wantUnresolved, members: 15},
		{name: "safe-inverse-batch", want: wantFourWrites, members: 15},
	} {
		t.Run(batch.name, func(t *testing.T) {
			insertion := "\n\t\tvar replacement func()\n\t\tknown := " + canonical
			extraSource := supportSource
			var routes []string
			members := 0
			for index, fixture := range fixtures {
				if fixture.want != batch.want {
					continue
				}
				call := strings.Replace(
					fixture.insertion,
					"\n\t\tvar replacement func()\n", "", 1,
				)
				call = strings.Replace(call, canonical, "known", 1)
				insertion += "\n" + strings.TrimPrefix(call, "\n")
				source := strings.TrimPrefix(fixture.extraSource, supportSource)
				if source == fixture.extraSource {
					t.Fatalf("PIB-518 %s did not share challenge support source", fixture.name)
				}
				route := fixture.wantRoute
				if batch.want == wantUnresolved &&
					route == "s7ARRev25ChallengeSink" {
					route = fmt.Sprintf("s7ARRev25ChallengeSinkBatch%d", index)
					source = strings.ReplaceAll(
						source, "s7ARRev25ChallengeSink", route,
					)
					extraSource += fmt.Sprintf(
						"func %s(callbacks ...func()) { callbacks[0]() }\n",
						route,
					)
				}
				extraSource += source
				if route != "" {
					routes = append(routes, route)
				}
				members++
			}
			if members != batch.members {
				t.Fatalf(
					"PIB-518 %s has %d members, want %d",
					batch.name, members, batch.members,
				)
			}
			err := validate(t, insertion, extraSource)
			switch batch.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						batch.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins",
						batch.name, err,
					)
				}
				for _, route := range routes {
					if !strings.Contains(err.Error(), route) {
						t.Errorf(
							"PIB-518 %s omitted unresolved route %q: %v",
							batch.name, route, err,
						)
					}
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						batch.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", batch.name, batch.want)
			}
		})
	}
}

func TestS7ARRev26SelectPreselectionRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-26 select preselection anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev26_select_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-26 select repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"func s7ARRev26Sink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev26MutatingChannel(callbacks []func(), replacement func()) chan<- func() {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn nil\n" +
		"}\n"

	t.Run("send-channel-nil-default-unsafe-bite", func(t *testing.T) {
		err := validate(
			t,
			"\n\t\tvar replacement func()\n"+
				"\t\ts7ARRev26SelectChannelUnsafe(replacement, "+canonical+")",
			supportSource+
				"func s7ARRev26SelectChannelUnsafe(replacement, known func()) {\n"+
				"\tcallbacks := []func(){known}\n"+
				"\ttarget := func() {\n"+
				"\t\tselect {\n"+
				"\t\tcase s7ARRev26MutatingChannel(callbacks, replacement) <- known:\n"+
				"\t\tdefault:\n"+
				"\t\t}\n"+
				"\t\treturn\n"+
				"\t}\n"+
				"\tdefer s7ARRev26Sink(callbacks...)\n"+
				"\tdefer target()\n"+
				"}\n",
		)
		if err == nil ||
			!strings.Contains(err.Error(), "unresolved callable origins") ||
			!strings.Contains(err.Error(), "s7ARRev26Sink") {
			t.Fatalf(
				"PIB-518 send-channel preselection bite = %v, want unresolved callable origins at s7ARRev26Sink",
				err,
			)
		}
		if _, counted := s7ARInventoryActualWriteCount(err); counted {
			t.Fatalf(
				"PIB-518 send-channel preselection bite used inventory-count failure instead of callable incompleteness: %v",
				err,
			)
		}
	})

	t.Run("send-channel-nil-default-restoring-inverse", func(t *testing.T) {
		err := validate(
			t,
			"\n\t\tvar replacement func()\n"+
				"\t\ts7ARRev26SelectChannelSafe(replacement, "+canonical+")",
			supportSource+
				"func s7ARRev26SelectChannelSafe(replacement, known func()) {\n"+
				"\tcallbacks := []func(){replacement}\n"+
				"\ttarget := func() {\n"+
				"\t\tselect {\n"+
				"\t\tcase s7ARRev26MutatingChannel(callbacks, known) <- known:\n"+
				"\t\tdefault:\n"+
				"\t\t}\n"+
				"\t\treturn\n"+
				"\t}\n"+
				"\tdefer s7ARRev26Sink(callbacks...)\n"+
				"\tdefer target()\n"+
				"}\n",
		)
		actual, counted := s7ARInventoryActualWriteCount(err)
		if !counted || actual != 4 {
			t.Fatalf(
				"PIB-518 send-channel preselection restoring inverse: error=%v writes=%d counted=%t, want four canonical writes",
				err, actual, counted,
			)
		}
	})
}

func TestS7ARRev27CallOperandSnapshotRepros(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-27 call operand snapshot anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev27_operand_repro.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-27 operand repro is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"func s7ARRev27Sink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev27Mutate(callbacks []func(), value func(), _ int) chan<- func() {\n" +
		"\tcallbacks[0] = value\n" +
		"\treturn nil\n" +
		"}\n"

	t.Run("slice-descriptor-rebinding-unsafe-bite", func(t *testing.T) {
		err := validate(
			t,
			"\n\t\tvar replacement func()\n"+
				"\t\ts7ARRev27DescriptorUnsafe(replacement, "+canonical+")",
			supportSource+
				"func s7ARRev27DescriptorUnsafe(replacement, known func()) {\n"+
				"\toriginal := []func(){known}\n"+
				"\tother := []func(){known}\n"+
				"\tcallbacks := original\n"+
				"\ttarget := func() {\n"+
				"\t\tselect {\n"+
				"\t\tcase s7ARRev27Mutate(callbacks, replacement, func() int { callbacks = other; return 0 }()) <- known:\n"+
				"\t\tdefault:\n"+
				"\t\t}\n"+
				"\t\treturn\n"+
				"\t}\n"+
				"\tdefer s7ARRev27Sink(original...)\n"+
				"\tdefer target()\n"+
				"}\n",
		)
		if err == nil ||
			!strings.Contains(err.Error(), "unresolved callable origins") ||
			!strings.Contains(err.Error(), "s7ARRev27Sink") {
			t.Fatalf(
				"PIB-518 call operand descriptor bite = %v, want unresolved callable origins at s7ARRev27Sink",
				err,
			)
		}
		if _, counted := s7ARInventoryActualWriteCount(err); counted {
			t.Fatalf(
				"PIB-518 call operand descriptor bite used inventory-count failure instead of callable incompleteness: %v",
				err,
			)
		}
	})

	t.Run("slice-descriptor-rebinding-restoring-inverse", func(t *testing.T) {
		err := validate(
			t,
			"\n\t\tvar replacement func()\n"+
				"\t\ts7ARRev27DescriptorSafe(replacement, "+canonical+")",
			supportSource+
				"func s7ARRev27DescriptorSafe(replacement, known func()) {\n"+
				"\toriginal := []func(){replacement}\n"+
				"\tother := []func(){replacement}\n"+
				"\tcallbacks := original\n"+
				"\ttarget := func() {\n"+
				"\t\tselect {\n"+
				"\t\tcase s7ARRev27Mutate(callbacks, known, func() int { callbacks = other; return 0 }()) <- known:\n"+
				"\t\tdefault:\n"+
				"\t\t}\n"+
				"\t\treturn\n"+
				"\t}\n"+
				"\tdefer s7ARRev27Sink(original...)\n"+
				"\tdefer target()\n"+
				"}\n",
		)
		actual, counted := s7ARInventoryActualWriteCount(err)
		if !counted || actual != 4 {
			t.Fatalf(
				"PIB-518 call operand descriptor restoring inverse: error=%v writes=%d counted=%t, want four canonical writes",
				err, actual, counted,
			)
		}
	})
}

func TestS7ARRev27StaticChallenges(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-27 static challenge anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev27_operand_challenge.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-27 challenge is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"type s7ARRev27Callbacks []func()\n" +
		"func s7ARRev27ChallengeSink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev27ChallengeMutate(callbacks []func(), value func(), _ int) chan<- func() {\n" +
		"\tcallbacks[0] = value\n" +
		"\treturn nil\n" +
		"}\n" +
		"func (callbacks s7ARRev27Callbacks) mutate(value func(), _ int) chan<- func() {\n" +
		"\tcallbacks[0] = value\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev27ChallengeInvoke(callback func(), _ int) chan<- func() {\n" +
		"\tcallback()\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev27ChallengeInstall(callback func(), callbacks []func(), _ int) chan<- func() {\n" +
		"\tcallbacks[0] = callback\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev27ChallengeMutateTwo(first, second []func(), firstValue, secondValue func(), _ int) chan<- func() {\n" +
		"\tfirst[0] = firstValue\n" +
		"\tsecond[0] = secondValue\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev27ChallengeSet(callbacks []func(), value func()) int {\n" +
		"\tcallbacks[0] = value\n" +
		"\treturn 0\n" +
		"}\n" +
		"func s7ARRev27ChallengeNested(callbacks []func(), value func(), _ int) chan<- func() {\n" +
		"\tdefer func(callbacks []func(), value func()) { callbacks[0] = value }(callbacks, value)\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev27ChallengeRecursive(callbacks []func(), value func(), depth int) chan<- func() {\n" +
		"\tif depth > 0 { return s7ARRev27ChallengeRecursive(callbacks, value, depth-1) }\n" +
		"\tcallbacks[0] = value\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev27ChallengeVariadic(value func(), callbacks ...[]func()) chan<- func() {\n" +
		"\tcallbacks[0][0] = value\n" +
		"\treturn nil\n" +
		"}\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "scalar-callable-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27ScalarUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27ScalarUnsafe(replacement, known func()) {\n" +
				"\tsource, target := []func(){replacement}, []func(){known}\n" +
				"\tselect { case s7ARRev27ChallengeInstall(source[0], target, func() int { source[0] = known; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(target...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "scalar-callable-snapshot-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27ScalarSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27ScalarSafe(replacement, known func()) {\n" +
				"\tsource, target := []func(){known}, []func(){replacement}\n" +
				"\tselect { case s7ARRev27ChallengeInstall(source[0], target, func() int { source[0] = replacement; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(target...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "direct-scalar-invocation-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27DirectScalarUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27DirectScalarUnsafe(replacement, known func()) {\n" +
				"\tsource := []func(){replacement}\n" +
				"\tselect { case s7ARRev27ChallengeInvoke(source[0], func() int { source[0] = known; return 0 }()) <- known: default: }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeInvoke",
		},
		{
			name: "direct-scalar-invocation-snapshot-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27DirectScalarSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27DirectScalarSafe(replacement, known func()) {\n" +
				"\tsource := []func(){known}\n" +
				"\tselect { case s7ARRev27ChallengeInvoke(source[0], func() int { source[0] = replacement; return 0 }()) <- known: default: }\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "method-value-receiver-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27MethodValueUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27MethodValueUnsafe(replacement, known func()) {\n" +
				"\toriginal := s7ARRev27Callbacks{known}\n" +
				"\tother := s7ARRev27Callbacks{known}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case callbacks.mutate(replacement, func() int { callbacks = other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "method-value-receiver-snapshot-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27MethodValueSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27MethodValueSafe(replacement, known func()) {\n" +
				"\toriginal := s7ARRev27Callbacks{replacement}\n" +
				"\tother := s7ARRev27Callbacks{replacement}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case callbacks.mutate(known, func() int { callbacks = other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "method-expression-receiver-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27MethodExpressionUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27MethodExpressionUnsafe(replacement, known func()) {\n" +
				"\toriginal := s7ARRev27Callbacks{known}\n" +
				"\tother := s7ARRev27Callbacks{known}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27Callbacks.mutate(callbacks, replacement, func() int { callbacks = other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "method-expression-receiver-snapshot-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27MethodExpressionSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27MethodExpressionSafe(replacement, known func()) {\n" +
				"\toriginal := s7ARRev27Callbacks{replacement}\n" +
				"\tother := s7ARRev27Callbacks{replacement}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27Callbacks.mutate(callbacks, known, func() int { callbacks = other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "aliased-arguments-ordered-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27AliasedUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27AliasedUnsafe(replacement, known func()) {\n" +
				"\toriginal := []func(){known}\n" +
				"\tother := []func(){known}\n" +
				"\tfirst, second := original, original\n" +
				"\tselect { case s7ARRev27ChallengeMutateTwo(first, second, known, replacement, func() int { first, second = other, other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "aliased-arguments-ordered-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27AliasedSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27AliasedSafe(replacement, known func()) {\n" +
				"\toriginal := []func(){replacement}\n" +
				"\tother := []func(){replacement}\n" +
				"\tfirst, second := original, original\n" +
				"\tselect { case s7ARRev27ChallengeMutateTwo(first, second, replacement, known, func() int { first, second = other, other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "branching-later-rebinding-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27BranchUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27BranchUnsafe(replacement, known func()) {\n" +
				"\toriginal := []func(){known}\n" +
				"\tleft, right := []func(){known}, []func(){known}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27ChallengeMutate(callbacks, replacement, func() int { if replacement == nil { callbacks = left } else { callbacks = right }; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "branching-later-rebinding-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27BranchSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27BranchSafe(replacement, known func()) {\n" +
				"\toriginal := []func(){replacement}\n" +
				"\tleft, right := []func(){replacement}, []func(){replacement}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27ChallengeMutate(callbacks, known, func() int { if replacement == nil { callbacks = left } else { callbacks = right }; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multiple-later-rebindings-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27MultipleRebindUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27MultipleRebindUnsafe(replacement, known func()) {\n" +
				"\toriginal := []func(){known}\n" +
				"\tother, terminal := []func(){known}, []func(){known}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27ChallengeMutate(callbacks, replacement, func() int { callbacks = other; callbacks = terminal; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "multiple-later-rebindings-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27MultipleRebindSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27MultipleRebindSafe(replacement, known func()) {\n" +
				"\toriginal := []func(){replacement}\n" +
				"\tother, terminal := []func(){replacement}, []func(){replacement}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27ChallengeMutate(callbacks, known, func() int { callbacks = other; callbacks = terminal; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "same-backing-later-mutation-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27SameBackingUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27SameBackingUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\tselect { case s7ARRev27ChallengeMutate(callbacks, replacement, s7ARRev27ChallengeSet(callbacks, known)) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "same-backing-later-mutation-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27SameBackingSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27SameBackingSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tselect { case s7ARRev27ChallengeMutate(callbacks, known, s7ARRev27ChallengeSet(callbacks, replacement)) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "disjoint-backing-mutation-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27DisjointUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27DisjointUnsafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){known}, []func(){replacement}\n" +
				"\tselect { case s7ARRev27ChallengeMutate(original, replacement, s7ARRev27ChallengeSet(other, known)) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "disjoint-backing-mutation-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27DisjointSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27DisjointSafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){replacement}, []func(){known}\n" +
				"\tselect { case s7ARRev27ChallengeMutate(original, known, s7ARRev27ChallengeSet(other, replacement)) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "nested-helper-defer-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27NestedUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27NestedUnsafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){known}, []func(){known}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27ChallengeNested(callbacks, replacement, func() int { callbacks = other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "nested-helper-defer-snapshot-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27NestedSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27NestedSafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){replacement}, []func(){replacement}\n" +
				"\tcallbacks := original\n" +
				"\tselect { case s7ARRev27ChallengeNested(callbacks, known, func() int { callbacks = other; return 0 }()) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "recursive-helper-fails-closed",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27RecursiveControl(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27RecursiveControl(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tselect { case s7ARRev27ChallengeRecursive(callbacks, replacement, 1) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "variadic-helper-fails-closed",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27VariadicControl(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27VariadicControl(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tselect { case s7ARRev27ChallengeVariadic(replacement, callbacks) <- known: default: }\n" +
				"\tdefer s7ARRev27ChallengeSink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeVariadic",
		},
		{
			name: "deferred-call-argument-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27DeferredUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27DeferredUnsafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){known}, []func(){known}\n" +
				"\tcallbacks := original\n" +
				"\ttarget := func(callbacks []func(), value func(), _ int) { callbacks[0] = value }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"\tdefer target(callbacks, replacement, func() int { callbacks = other; return 0 }())\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "deferred-call-argument-snapshot-safe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27DeferredSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27DeferredSafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){replacement}, []func(){replacement}\n" +
				"\tcallbacks := original\n" +
				"\ttarget := func(callbacks []func(), value func(), _ int) { callbacks[0] = value }\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"\tdefer target(callbacks, known, func() int { callbacks = other; return 0 }())\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-call-argument-snapshot-unsafe",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27AsyncUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27AsyncUnsafe(replacement, known func()) {\n" +
				"\toriginal, other := []func(){known}, []func(){known}\n" +
				"\tcallbacks := original\n" +
				"\tgo s7ARRev27ChallengeMutate(callbacks, replacement, func() int { callbacks = other; return 0 }())\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev27ChallengeSink",
		},
		{
			name: "async-disjoint-precision-control",
			insertion: "\n\t\tvar replacement func()\n" +
				"\t\ts7ARRev27AsyncDisjoint(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev27AsyncDisjoint(replacement, known func()) {\n" +
				"\toriginal, other := []func(){known}, []func(){known}\n" +
				"\tcallbacks := other\n" +
				"\tgo s7ARRev27ChallengeMutate(callbacks, replacement, func() int { callbacks = other; return 0 }())\n" +
				"\tdefer s7ARRev27ChallengeSink(original...)\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	routePattern := regexp.MustCompile(
		`(s7ARRev27[A-Za-z0-9]+)\(replacement,`,
	)
	runBatch := func(
		t *testing.T,
		name string,
		want expectation,
	) {
		t.Helper()
		extraSource := supportSource
		var calls []string
		var routes []string
		var required []string
		for _, fixture := range fixtures {
			if fixture.want != want {
				continue
			}
			suffix := strings.TrimPrefix(
				fixture.extraSource, supportSource,
			)
			if suffix == fixture.extraSource {
				t.Fatalf(
					"PIB-518 %s does not share the rev-27 support source",
					fixture.name,
				)
			}
			extraSource += suffix
			match := routePattern.FindStringSubmatch(
				fixture.insertion,
			)
			if len(match) != 2 {
				t.Fatalf(
					"PIB-518 %s has no batchable route call",
					fixture.name,
				)
			}
			route := match[1]
			routes = append(routes, route)
			calls = append(
				calls, "\t"+route+"(replacement, known)\n",
			)
			if want == wantUnresolved {
				required = append(required, fixture.wantRoute)
				if fixture.wantRoute ==
					"s7ARRev27ChallengeSink" {
					required = append(required, route)
				}
			}
		}
		if len(routes) == 0 {
			t.Fatalf("PIB-518 %s has no routes", name)
		}
		batch := "s7ARRev27" + name
		extraSource += "func " + batch +
			"(replacement, known func()) {\n" +
			strings.Join(calls, "") + "}\n"
		err := validate(
			t,
			"\n\t\tvar replacement func()\n"+
				"\t\t"+batch+"(replacement, "+canonical+")",
			extraSource,
		)
		switch want {
		case wantFourWrites:
			actual, counted := s7ARInventoryActualWriteCount(err)
			if !counted || actual != 4 {
				t.Fatalf(
					"PIB-518 %s: error=%v writes=%d counted=%t, want every safe route active and exactly four canonical writes",
					name, err, actual, counted,
				)
			}
		case wantUnresolved:
			if err == nil ||
				!strings.Contains(
					err.Error(), "unresolved callable origins",
				) {
				t.Fatalf(
					"PIB-518 %s = %v, want unresolved callable origins",
					name, err,
				)
			}
			for _, route := range required {
				if !strings.Contains(err.Error(), route) {
					t.Errorf(
						"PIB-518 %s omitted required route %q: %v",
						name, route, err,
					)
				}
			}
			if _, counted := s7ARInventoryActualWriteCount(err); counted {
				t.Fatalf(
					"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
					name, err,
				)
			}
		default:
			t.Fatalf("PIB-518 %s has unknown expectation %d", name, want)
		}
	}
	t.Run("UnsafeBatch", func(t *testing.T) {
		runBatch(t, "UnsafeBatch", wantUnresolved)
	})
	t.Run("SafeBatch", func(t *testing.T) {
		runBatch(t, "SafeBatch", wantFourWrites)
	})
}

func TestS7ARRev26StaticChallenges(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-26 static challenge anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev26_select_challenge.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-26 challenge is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const supportSource = "package cli\n\n" +
		"type s7ARRev26ChallengeHolder struct { value func() }\n" +
		"type s7ARRev26ChallengeCallbacks []func()\n" +
		"func (callbacks s7ARRev26ChallengeCallbacks) sendChannel(replacement func()) chan<- func() {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev26ChallengeSink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev26ChallengeNilSend() chan<- func() { return nil }\n" +
		"func s7ARRev26ChallengeNilReceive() <-chan func() { return nil }\n" +
		"func s7ARRev26ChallengeSendChannel(callbacks []func(), replacement func()) chan<- func() {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev26ChallengeBufferedSend(callbacks []func(), replacement func()) chan<- func() {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn make(chan func(), 1)\n" +
		"}\n" +
		"func s7ARRev26ChallengeDeferredSendChannel(callbacks []func(), replacement func()) chan<- func() {\n" +
		"\tdefer func() { callbacks[0] = replacement }()\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev26ChallengeSendValue(callbacks []func(), replacement func()) func() {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn replacement\n" +
		"}\n" +
		"func s7ARRev26ChallengeReceiveChannel(callbacks []func(), replacement func()) <-chan func() {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn nil\n" +
		"}\n" +
		"func s7ARRev26ChallengeReceiveLHS(callbacks []func(), replacement func()) *s7ARRev26ChallengeHolder {\n" +
		"\tcallbacks[0] = replacement\n" +
		"\treturn &s7ARRev26ChallengeHolder{}\n" +
		"}\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
		separate    bool
	}{
		{
			name: "send-value-preselection-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeSendValueUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeSendValueUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeNilSend() <- s7ARRev26ChallengeSendValue(callbacks, replacement): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "send-value-preselection-restores-known",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeSendValueSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeSendValueSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeNilSend() <- s7ARRev26ChallengeSendValue(callbacks, known): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "send-channel-then-value-source-order-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeSendOrderUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeSendOrderUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeSendChannel(callbacks, known) <- s7ARRev26ChallengeSendValue(callbacks, replacement): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "send-channel-then-value-source-order-safe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeSendOrderSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeSendOrderSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeSendChannel(callbacks, replacement) <- s7ARRev26ChallengeSendValue(callbacks, known): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "named-channel-helper-defer-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeHelperDeferUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeHelperDeferUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeDeferredSendChannel(callbacks, replacement) <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "named-channel-helper-defer-restores-known",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeHelperDeferSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeHelperDeferSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeDeferredSendChannel(callbacks, known) <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "named-slice-method-channel-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeMethodUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeMethodUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := s7ARRev26ChallengeCallbacks{known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case callbacks.sendChannel(replacement) <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "named-slice-method-channel-restores-known",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeMethodSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeMethodSafe(replacement, known func()) {\n" +
				"\tcallbacks := s7ARRev26ChallengeCallbacks{replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case callbacks.sendChannel(known) <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "receive-channel-preselection-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeReceiveUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeReceiveUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case <-s7ARRev26ChallengeReceiveChannel(callbacks, replacement): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "receive-channel-preselection-restores-known",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeReceiveSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeReceiveSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case <-s7ARRev26ChallengeReceiveChannel(callbacks, known): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "receive-assignment-channel-common-to-default",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeReceiveAssignUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeReceiveAssignUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvar holder s7ARRev26ChallengeHolder\n" +
				"\ttarget := func() {\n" +
				"\t\tselect {\n" +
				"\t\tcase holder.value = <-s7ARRev26ChallengeReceiveChannel(callbacks, replacement): callbacks[0] = known\n" +
				"\t\tdefault:\n" +
				"\t\t}\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "receive-assignment-channel-restoring-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeReceiveAssignSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeReceiveAssignSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\tvar holder s7ARRev26ChallengeHolder\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case holder.value = <-s7ARRev26ChallengeReceiveChannel(callbacks, known): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "receive-assignment-lhs-selected-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeReceiveLHSUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeReceiveLHSUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeReceiveLHS(callbacks, replacement).value = <-s7ARRev26ChallengeNilReceive(): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "receive-assignment-lhs-disjoint-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeReceiveLHSSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeReceiveLHSSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tother := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case s7ARRev26ChallengeReceiveLHS(other, replacement).value = <-s7ARRev26ChallengeNilReceive(): default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multiple-communications-source-order-unsafe",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeOrderUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeOrderUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect {\n" +
				"\t\tcase s7ARRev26ChallengeSendChannel(callbacks, known) <- known:\n" +
				"\t\tcase s7ARRev26ChallengeSendChannel(callbacks, replacement) <- known:\n" +
				"\t\tdefault:\n" +
				"\t\t}\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "multiple-communications-source-order-restores-known",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeOrderSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeOrderSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tselect {\n" +
				"\t\tcase s7ARRev26ChallengeSendChannel(callbacks, replacement) <- known:\n" +
				"\t\tcase s7ARRev26ChallengeSendChannel(callbacks, known) <- known:\n" +
				"\t\tdefault:\n" +
				"\t\t}\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "ordinary-send-applies-channel-mutation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeOrdinarySendUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeOrdinarySendUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() { s7ARRev26ChallengeBufferedSend(callbacks, replacement) <- known; return }\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
		},
		{
			name: "ordinary-send-restoring-inverse",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeOrdinarySendSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeOrdinarySendSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() { s7ARRev26ChallengeBufferedSend(callbacks, known) <- known; return }\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "select-return-preserves-preselection-before-nested-defer",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeNestedReturnUnsafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeNestedReturnUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {\n" +
				"\t\tdefer func() { _ = len(callbacks) }()\n" +
				"\t\tselect { case s7ARRev26ChallengeSendChannel(callbacks, replacement) <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev26ChallengeSink",
			separate:  true,
		},
		{
			name: "select-return-runs-restoring-nested-defer",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeNestedReturnSafe(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeNestedReturnSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){replacement}\n" +
				"\ttarget := func() {\n" +
				"\t\tdefer func() { callbacks[0] = known }()\n" +
				"\t\tselect { case s7ARRev26ChallengeSendChannel(callbacks, replacement) <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "side-effect-free-send-receive-select-remains-exact",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev26ChallengeSideEffectFree(replacement, " + canonical + ")",
			extraSource: supportSource +
				"func s7ARRev26ChallengeSideEffectFree(replacement, known func()) {\n" +
				"\t_ = replacement\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tchannel := make(chan func(), 1)\n" +
				"\ttarget := func() {\n" +
				"\t\tselect { case <-channel: default: }\n" +
				"\t\tvar received func()\n" +
				"\t\tselect { case received = <-channel: _ = received; default: }\n" +
				"\t\tselect { case received, ok := <-channel: _, _ = received, ok; default: }\n" +
				"\t\tselect { case channel <- known: default: }\n" +
				"\t\treturn\n" +
				"\t}\n" +
				"\tdefer s7ARRev26ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, batch := range []struct {
		name     string
		want     expectation
		members  int
		separate bool
	}{
		{name: "unsafe-route-batch", want: wantUnresolved, members: 9},
		{
			name: "unsafe-nested-defer-batch", want: wantUnresolved,
			members: 1, separate: true,
		},
		{name: "safe-inverse-batch", want: wantFourWrites, members: 11},
	} {
		t.Run(batch.name, func(t *testing.T) {
			insertion := "\n\t\tvar replacement func()\n\t\tknown := " + canonical
			extraSource := supportSource
			var routes []string
			members := 0
			for index, fixture := range fixtures {
				if fixture.want != batch.want ||
					fixture.separate != batch.separate {
					continue
				}
				call := strings.Replace(
					fixture.insertion,
					"\n\t\tvar replacement func()\n", "", 1,
				)
				call = strings.Replace(call, canonical, "known", 1)
				insertion += "\n" + strings.TrimPrefix(call, "\n")
				source := strings.TrimPrefix(fixture.extraSource, supportSource)
				if source == fixture.extraSource {
					t.Fatalf("PIB-518 %s did not share challenge support source", fixture.name)
				}
				route := fixture.wantRoute
				if batch.want == wantUnresolved &&
					route == "s7ARRev26ChallengeSink" {
					route = fmt.Sprintf("s7ARRev26ChallengeSinkBatch%d", index)
					source = strings.ReplaceAll(
						source, "s7ARRev26ChallengeSink", route,
					)
					extraSource += fmt.Sprintf(
						"func %s(callbacks ...func()) { callbacks[0]() }\n",
						route,
					)
				}
				extraSource += source
				if route != "" {
					routes = append(routes, route)
				}
				members++
			}
			if members != batch.members {
				t.Fatalf(
					"PIB-518 %s has %d members, want %d",
					batch.name, members, batch.members,
				)
			}
			err := validate(t, insertion, extraSource)
			switch batch.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						batch.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins",
						batch.name, err,
					)
				}
				for _, route := range routes {
					if !strings.Contains(err.Error(), route) {
						t.Errorf(
							"PIB-518 %s omitted unresolved route %q: %v",
							batch.name, route, err,
						)
					}
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						batch.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", batch.name, batch.want)
			}
		})
	}
}

func TestS7ARRev23StaticChallenges(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-23 static challenge anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev23_challenge.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-23 challenge is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev23ChallengeSink(callbacks ...func()) { callbacks[0]() }\n" +
		"func s7ARRev23ChallengeNamedNoop() {}\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "parenthesized-alias-target",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeParenthesized(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeParenthesized(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer (mutate)()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "branch-joined-target-includes-unsafe-alternative",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeBranch(true, replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeBranch(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {}\n" +
				"\tif condition { target = func() { callbacks[0] = replacement } }\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "branch-joined-safe-targets-remain-exact",
			insertion: "\n" +
				"\t\ts7ARRev23ChallengeSafeBranch(true, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeSafeBranch(condition bool, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ttarget := func() {}\n" +
				"\tif condition { target = func() { _ = len(callbacks) } }\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "multi-hop-alias-chain",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeAliasChain(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeAliasChain(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tfirst := func() { callbacks[0] = replacement }\n" +
				"\tsecond := first\n" +
				"\tthird := second\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer third()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "nested-alias-invocation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeNestedAlias(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeNestedAlias(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\trun := func() {\n" +
				"\t\tinner := func() { callbacks[0] = replacement }\n" +
				"\t\tinner()\n" +
				"\t}\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer run()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "captured-callable-safe-reassignment-is-by-reference",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeCapturedSafe(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeCapturedSafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvalue := func() { callbacks[0] = replacement }\n" +
				"\trun := func() { value() }\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer run()\n" +
				"\tvalue = func() {}\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "captured-callable-unsafe-reassignment-is-by-reference",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeCapturedUnsafe(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeCapturedUnsafe(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvalue := func() {}\n" +
				"\trun := func() { value() }\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer run()\n" +
				"\tvalue = func() { callbacks[0] = replacement }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "alias-chain-target-reassignment-after-registration",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeRegisteredChain(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeRegisteredChain(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tfirst := func() { callbacks[0] = replacement }\n" +
				"\ttarget := first\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer target()\n" +
				"\ttarget = func() {}\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "multiple-scheduled-call-sites-keep-separate-snapshots",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeCallSites(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeCallSites(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tother := []func(){func() {}}\n" +
				"\trun := func(callback func()) { callback() }\n" +
				"\tsafe := func() { _ = len(callbacks) }\n" +
				"\tdisjoint := func() { other[0] = replacement }\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer run(safe)\n" +
				"\tdefer run(disjoint)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "async-parenthesized-alias-retains-later-backing",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeAsyncParenthesized(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeAsyncParenthesized(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tlaunch := func() { s7ARRev23ChallengeSink(callbacks...) }\n" +
				"\tgo (launch)()\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev23ChallengeSink",
		},
		{
			name: "direct-named-noop-remains-inert",
			insertion: "\n" +
				"\t\ts7ARRev23ChallengeNamed(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeNamed(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"\tdefer s7ARRev23ChallengeNamedNoop()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "declared-but-unscheduled-alias-remains-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev23ChallengeDeclared(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev23ChallengeDeclared(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tmutate := func() { callbacks[0] = replacement }\n" +
				"\t_ = mutate\n" +
				"\ts7ARRev23ChallengeNamedNoop()\n" +
				"\tdefer s7ARRev23ChallengeSink(callbacks...)\n" +
				"}\n",
			want: wantFourWrites,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func TestS7ARRev21StaticChallenges(t *testing.T) {
	const storeCall = "report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)"
	validate := func(
		t *testing.T,
		insertion string,
		extraSource string,
	) error {
		t.Helper()
		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		sources := s7ARProductionSourceSet(t)
		mutated := strings.Replace(
			sources["internal/cli/feature_intent_archive.go"],
			storeCall,
			storeCall+insertion,
			1,
		)
		if mutated == sources["internal/cli/feature_intent_archive.go"] {
			t.Fatal("PIB-518 rev-21 static challenge anchor missing")
		}
		sources["internal/cli/feature_intent_archive.go"] = mutated
		sources["internal/cli/zz_s7_ar_rev21_challenge.go"] = extraSource
		resumes, err := s7ARDerivePurgeResumeDomain(storeSource)
		if err != nil {
			t.Fatal(err)
		}
		model, err := s6BuildSourceTypeModel(
			s6EmissionTypeSources(s7ARCLIPackageSources(sources)),
		)
		if err != nil {
			t.Fatalf("PIB-518 rev-21 static challenge is not valid typed source: %v", err)
		}
		return validateS7ARPurgeProgressWithModel(
			s7ARProductionPurgeProgressReports(resumes), sources, model,
		)
	}
	const canonical = "func() {\n" +
		"\t\t\treport.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)\n" +
		"\t\t}"
	const sinkSource = "package cli\n\n" +
		"func s7ARRev21ChallengeSink(callbacks ...func()) { callbacks[0]() }\n"
	const launchSource = "func s7ARRev21ChallengeLaunch(callbacks []func()) { go s7ARRev21ChallengeSink(callbacks...) }\n"
	stateCapSource := sinkSource +
		"func s7ARRev21ChallengeStateCap(conditions [7]bool, known func()) {\n" +
		"\tcallbacks := []func(){known}\n"
	for index := 0; index < 7; index++ {
		stateCapSource += fmt.Sprintf(
			"\tif conditions[%d] { go s7ARRev21ChallengeSink(callbacks...) }\n",
			index,
		)
	}
	stateCapSource += "}\n"

	type expectation int
	const (
		wantFourWrites expectation = iota
		wantUnresolved
	)
	fixtures := []struct {
		name        string
		insertion   string
		extraSource string
		want        expectation
		wantRoute   string
	}{
		{
			name: "caller-alias-write-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeAlias(replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeAlias(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\talias := callbacks[:]\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\talias[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeLaunch",
		},
		{
			name: "caller-reslice-offset-write-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeOffset(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeLaunchOffset(callbacks ...func()) { go s7ARRev21ChallengeSink(callbacks...) }\n" +
				"func s7ARRev21ChallengeOffset(replacement, known func()) {\n" +
				"\tcallbacks := []func(){func() {}, known}\n" +
				"\talias := callbacks[1:]\n" +
				"\ts7ARRev21ChallengeLaunchOffset(alias...)\n" +
				"\tcallbacks[1] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeLaunch",
		},
		{
			name: "caller-reslice-disjoint-write-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeOffsetDisjoint(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeLaunchOffset(callbacks ...func()) { go s7ARRev21ChallengeSink(callbacks...) }\n" +
				"func s7ARRev21ChallengeOffsetDisjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){func() {}, known}\n" +
				"\talias := callbacks[1:]\n" +
				"\ts7ARRev21ChallengeLaunchOffset(alias...)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "caller-copy-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeCopy(replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeCopy(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\tcopy(callbacks, []func(){replacement})\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeLaunch",
		},
		{
			name: "caller-copy-disjoint-offset-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeCopyDisjoint(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeLaunchOffset(callbacks ...func()) { go s7ARRev21ChallengeSink(callbacks...) }\n" +
				"func s7ARRev21ChallengeCopyDisjoint(replacement, known func()) {\n" +
				"\tcallbacks := []func(){func() {}, known}\n" +
				"\ts7ARRev21ChallengeLaunchOffset(callbacks[1:]...)\n" +
				"\tcopy(callbacks[:1], []func(){replacement})\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "caller-append-reuse-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeAppendReuse(replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeAppendReuse(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\talias := append(callbacks[:0], replacement)\n" +
				"\t_ = alias\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeLaunch",
		},
		{
			name: "caller-forced-append-allocation-after-async-escape",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeAppendDetached(replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeAppendDetached(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\talias := append(callbacks[:0:0], replacement)\n" +
				"\t_ = alias\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "branch-return-preserves-escaped-mutation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeBranch(true, replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeBranch(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\tif condition { callbacks[0] = replacement; return }\n" +
				"\t_ = callbacks\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeLaunch",
		},
		{
			name: "unreachable-post-return-escaped-mutation-is-inert",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeReturn(replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeReturn(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\treturn\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "caller-go-mutator-helper-shares-escaped-backing",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeGoHelpers(replacement, " + canonical + ")",
			extraSource: sinkSource + launchSource +
				"func s7ARRev21ChallengeGoMutate(callbacks []func(), replacement func()) {\n" +
				"\tgo func() { callbacks[0] = replacement }()\n" +
				"}\n" +
				"func s7ARRev21ChallengeGoHelpers(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeLaunch(callbacks)\n" +
				"\ts7ARRev21ChallengeGoMutate(callbacks, replacement)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeGoMutate",
		},
		{
			name: "caller-go-captured-sink-helper-shares-escaped-backing",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeGoCapturedOuter(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeGoCaptured(callbacks []func()) {\n" +
				"\tgo func() { s7ARRev21ChallengeSink(callbacks...) }()\n" +
				"}\n" +
				"func s7ARRev21ChallengeGoCapturedOuter(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeGoCaptured(callbacks)\n" +
				"\tcallbacks[0] = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeGoCaptured",
		},
		{
			name: "caller-go-captured-sink-helper-without-mutation",
			insertion: "\n" +
				"\t\ts7ARRev21ChallengeGoCapturedKnown(" + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeGoCaptured(callbacks []func()) {\n" +
				"\tgo func() { s7ARRev21ChallengeSink(callbacks...) }()\n" +
				"}\n" +
				"func s7ARRev21ChallengeGoCapturedKnown(known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeGoCaptured(callbacks)\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "caller-go-captured-sink-helper-descriptor-reassignment",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeGoCapturedDescriptor(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeGoCaptured(callbacks []func()) {\n" +
				"\tgo func() { s7ARRev21ChallengeSink(callbacks...) }()\n" +
				"}\n" +
				"func s7ARRev21ChallengeGoCapturedDescriptor(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\ts7ARRev21ChallengeGoCaptured(callbacks)\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"\t_ = callbacks\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "go-mutator-registered-before-go-sink",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeGoOrder(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeGoOrder(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tgo func() { callbacks[0] = replacement }()\n" +
				"\tgo s7ARRev21ChallengeSink(callbacks...)\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeSink",
		},
		{
			name: "deferred-captured-scalar-observes-later-reassignment",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeCapturedScalar(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeCapturedScalar(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvalue := known\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tdefer func() { callbacks[0] = value }()\n" +
				"\tvalue = replacement\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeSink",
		},
		{
			name: "deferred-explicit-scalar-parameter-is-value-captured",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeExplicitScalar(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeExplicitScalar(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tvalue := known\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tdefer func(callback func()) { callbacks[0] = callback }(value)\n" +
				"\tvalue = replacement\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "deferred-closure-nested-helper-mutation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeNestedHelper(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeWrite(callbacks []func(), replacement func()) { callbacks[0] = replacement }\n" +
				"func s7ARRev21ChallengeNestedHelper(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tdefer func() { s7ARRev21ChallengeWrite(callbacks, replacement) }()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeSink",
		},
		{
			name: "deferred-closure-nested-invoked-closure",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeNestedClosure(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeNestedClosure(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tdefer func() {\n" +
				"\t\tmutate := func() { callbacks[0] = replacement }\n" +
				"\t\tmutate()\n" +
				"\t}()\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeSink",
		},
		{
			name: "deferred-closure-nested-inert-closure",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeNestedInert(replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeNestedInert(replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tdefer func() {\n" +
				"\t\tmutate := func() { callbacks[0] = replacement }\n" +
				"\t\t_ = mutate\n" +
				"\t}()\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "deferred-explicit-return-terminal-mutation",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeReturnDefer(true, replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeReturnDefer(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tif condition { callbacks[0] = replacement; return }\n" +
				"\treturn\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeSink",
		},
		{
			name: "deferred-all-terminal-descriptor-replacement-is-disjoint",
			insertion: "\n" +
				"\t\tvar replacement func()\n" +
				"\t\ts7ARRev21ChallengeReturnDescriptor(true, replacement, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeReturnDescriptor(condition bool, replacement, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tdefer s7ARRev21ChallengeSink(callbacks...)\n" +
				"\tif condition { callbacks = []func(){replacement}; return }\n" +
				"\tcallbacks = []func(){replacement}\n" +
				"\treturn\n" +
				"}\n",
			want: wantFourWrites,
		},
		{
			name: "scheduled-loop-limit-fails-closed",
			insertion: "\n" +
				"\t\ts7ARRev21ChallengeLoop(true, " + canonical + ")",
			extraSource: sinkSource +
				"func s7ARRev21ChallengeLoop(condition bool, known func()) {\n" +
				"\tcallbacks := []func(){known}\n" +
				"\tfor condition { go s7ARRev21ChallengeSink(callbacks...) }\n" +
				"}\n",
			want:      wantUnresolved,
			wantRoute: "s7ARRev21ChallengeSink",
		},
		{
			name: "scheduled-state-cap-fails-closed",
			insertion: "\n" +
				"\t\ts7ARRev21ChallengeStateCap([7]bool{}, " + canonical + ")",
			extraSource: stateCapSource,
			want:        wantUnresolved,
			wantRoute:   "s7ARRev21ChallengeSink",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			err := validate(t, fixture.insertion, fixture.extraSource)
			switch fixture.want {
			case wantFourWrites:
				actual, counted := s7ARInventoryActualWriteCount(err)
				if !counted || actual != 4 {
					t.Fatalf(
						"PIB-518 %s: error=%v writes=%d counted=%t, want four canonical writes",
						fixture.name, err, actual, counted,
					)
				}
			case wantUnresolved:
				if err == nil ||
					!strings.Contains(err.Error(), "unresolved callable origins") ||
					!strings.Contains(err.Error(), fixture.wantRoute) {
					t.Fatalf(
						"PIB-518 %s = %v, want unresolved callable origins at %q",
						fixture.name, err, fixture.wantRoute,
					)
				}
				if _, counted := s7ARInventoryActualWriteCount(err); counted {
					t.Fatalf(
						"PIB-518 %s used inventory-count failure instead of callable incompleteness: %v",
						fixture.name, err,
					)
				}
			default:
				t.Fatalf("PIB-518 %s has unknown expectation %d", fixture.name, fixture.want)
			}
		})
	}
}

func s7ARClonePurgeProgressObservations(
	observations []s7ARPurgeProgressObservation,
) []s7ARPurgeProgressObservation {
	cloned := make([]s7ARPurgeProgressObservation, len(observations))
	for index, observation := range observations {
		cloned[index] = observation
		cloned[index].report = observation.report
		if observation.report.PurgeProgress != nil {
			progress := *observation.report.PurgeProgress
			progress.CompletedHashes = append([]string(nil), progress.CompletedHashes...)
			progress.RemainingHashes = append([]string(nil), progress.RemainingHashes...)
			cloned[index].report.PurgeProgress = &progress
		}
	}
	return cloned
}

func s7ARInventoryActualWriteCount(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	const prefix = " writes:["
	text := err.Error()
	start := strings.Index(text, prefix)
	if start < 0 {
		return 0, false
	}
	actual := text[start+len(prefix):]
	end := strings.Index(actual, "], want one builder")
	if end < 0 {
		return 0, false
	}
	actual = strings.TrimSpace(actual[:end])
	if actual == "" {
		return 0, true
	}
	return len(strings.Fields(actual)), true
}

func validateS7ARPurgeProgressEmitters(sources map[string]string) error {
	model, err := s6BuildSourceTypeModel(s6EmissionTypeSources(s7ARCLIPackageSources(sources)))
	if err != nil {
		return fmt.Errorf("type-check purge_progress emitters: %w", err)
	}
	return validateS7ARPurgeProgressEmittersWithModel(sources, model)
}

func validateS7ARPurgeProgressEmittersWithModel(
	sources map[string]string,
	model *s6SourceTypeModel,
) error {
	canonicalField, err := s7ARCanonicalPurgeProgressField(model)
	if err != nil {
		return err
	}
	rels := make([]string, 0, len(model.files))
	for rel := range model.files {
		if strings.HasPrefix(rel, "internal/cli/") &&
			strings.HasSuffix(rel, ".go") &&
			!strings.HasSuffix(rel, "_test.go") {
			rels = append(rels, rel)
		}
	}
	sort.Strings(rels)
	if len(rels) == 0 {
		return errors.New("typed purge_progress package source is absent")
	}
	functions := map[string]*s7ARPurgeProgressFunction{}
	for _, rel := range rels {
		for _, declaration := range model.files[rel].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			key := s6PrepareFunctionKey("internal/cli", function)
			if _, duplicate := functions[key]; duplicate {
				return fmt.Errorf("purge_progress function %s is ambiguous", key)
			}
			parameters := map[types.Object]int{}
			parameterIndex := 0
			if function.Type.Params != nil {
				for _, field := range function.Type.Params.List {
					for _, name := range field.Names {
						object := model.definitions[name]
						if object == nil {
							return fmt.Errorf(
								"purge_progress function %s parameter %s is untyped",
								key, name.Name,
							)
						}
						parameters[object] = parameterIndex
						parameterIndex++
					}
				}
			}
			functions[key] = &s7ARPurgeProgressFunction{
				key: key, rel: rel, function: function, parameters: parameters,
			}
		}
	}
	var builderComposites []string
	var writes []s7ARPurgeProgressWrite
	edges := map[string]map[string]bool{}
	seeds := map[string]bool{}
	nonTargetSeeds := map[string]bool{}
	var dependent []s7ARPurgeProgressDependentWrite
	invokedLiterals, invokedErr := s7ARInvokedFunctionLiteralsForPackage(model)
	if invokedErr != nil {
		return invokedErr
	}
	for _, rel := range rels {
		file := model.files[rel]
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			key := s6PrepareFunctionKey("internal/cli", function)
			info := functions[key]
			if info == nil {
				return fmt.Errorf("purge_progress function %s has no package descriptor", key)
			}
			functionWrites, functionBuilders, functionEdges, functionSeeds,
				functionNonTargetSeeds, functionDependent, analyzeErr :=
				s7ARAnalyzePurgeProgressFunction(
					model, canonicalField, info, functions, invokedLiterals,
				)
			if analyzeErr != nil {
				return analyzeErr
			}
			writes = append(writes, functionWrites...)
			builderComposites = append(builderComposites, functionBuilders...)
			for from, targets := range functionEdges {
				if edges[from] == nil {
					edges[from] = map[string]bool{}
				}
				for target := range targets {
					edges[from][target] = true
				}
			}
			for seed := range functionSeeds {
				seeds[seed] = true
			}
			for seed := range functionNonTargetSeeds {
				nonTargetSeeds[seed] = true
			}
			dependent = append(dependent, functionDependent...)
		}
	}
	queue := make([]string, 0, len(seeds))
	reachable := map[string]bool{}
	for seed := range seeds {
		reachable[seed] = true
		queue = append(queue, seed)
	}
	for len(queue) != 0 {
		node := queue[0]
		queue = queue[1:]
		for next := range edges[node] {
			if reachable[next] {
				continue
			}
			reachable[next] = true
			queue = append(queue, next)
		}
	}
	nonTargetQueue := make([]string, 0, len(nonTargetSeeds))
	nonTargetReachable := map[string]bool{}
	for seed := range nonTargetSeeds {
		nonTargetReachable[seed] = true
		nonTargetQueue = append(nonTargetQueue, seed)
	}
	for len(nonTargetQueue) != 0 {
		node := nonTargetQueue[0]
		nonTargetQueue = nonTargetQueue[1:]
		for next := range edges[node] {
			if nonTargetReachable[next] {
				continue
			}
			nonTargetReachable[next] = true
			nonTargetQueue = append(nonTargetQueue, next)
		}
	}
	for _, candidate := range dependent {
		target := reachable[candidate.parameter]
		nonTarget := nonTargetReachable[candidate.parameter]
		if target && nonTarget {
			return fmt.Errorf(
				"%s purge_progress pointer store has ambiguous canonical/decoy origin",
				candidate.write.label,
			)
		}
		if !target && !nonTarget {
			return fmt.Errorf(
				"%s purge_progress pointer store target is not connected to the report field",
				candidate.write.label,
			)
		}
		if target {
			writes = append(writes, candidate.write)
		}
	}
	sort.Slice(writes, func(left, right int) bool {
		if writes[left].rel != writes[right].rel {
			return writes[left].rel < writes[right].rel
		}
		return writes[left].position < writes[right].position
	})
	uniqueWrites := writes[:0]
	seenWrites := map[string]bool{}
	for _, write := range writes {
		key := fmt.Sprintf("%s:%d", write.rel, write.position)
		if seenWrites[key] {
			continue
		}
		seenWrites[key] = true
		uniqueWrites = append(uniqueWrites, write)
	}
	writes = uniqueWrites
	gotWrites := make([]string, 0, len(writes))
	for _, write := range writes {
		gotWrites = append(gotWrites, write.label)
	}
	wantWrites := []string{
		"applyIntentArchivePurgeResult:direct-builder",
		"emitIntentArchivePurgeFailure:direct-builder",
		"emitIntentArchivePurgeFailure:direct-nil",
	}
	if len(builderComposites) != 1 ||
		!strings.HasPrefix(builderComposites[0], "buildIntentArchivePurgeProgress:") ||
		!reflect.DeepEqual(gotWrites, wantWrites) {
		return fmt.Errorf(
			"purge_progress inventory = builders:%v writes:%v, want one builder / %v",
			builderComposites, gotWrites, wantWrites,
		)
	}
	return nil
}

type s7ARPurgeProgressWrite struct {
	rel      string
	position token.Pos
	label    string
}

type s7ARPurgeProgressFunction struct {
	key        string
	rel        string
	function   *ast.FuncDecl
	parameters map[types.Object]int
}

type s7ARPurgeProgressOrigin struct {
	target     bool
	nonTarget  bool
	parameters map[string]bool
	unknown    bool
}

type s7ARPurgeProgressDependentWrite struct {
	parameter string
	write     s7ARPurgeProgressWrite
}

func s7ARAnalyzePurgeProgressFunction(
	model *s6SourceTypeModel,
	canonicalField *types.Var,
	function *s7ARPurgeProgressFunction,
	functions map[string]*s7ARPurgeProgressFunction,
	invokedLiterals map[*ast.FuncLit]bool,
) (
	[]s7ARPurgeProgressWrite,
	[]string,
	map[string]map[string]bool,
	map[string]bool,
	map[string]bool,
	[]s7ARPurgeProgressDependentWrite,
	error,
) {
	aliases := map[types.Object]s7ARPurgeProgressOrigin{}
	for object, index := range function.parameters {
		if !s7ARPurgeProgressPointerCapable(object.Type()) {
			continue
		}
		aliases[object] = s7ARPurgeProgressOrigin{
			parameters: map[string]bool{s7ARPurgeProgressParameter(function.key, index): true},
		}
	}
	var writes []s7ARPurgeProgressWrite
	var builders []string
	edges := map[string]map[string]bool{}
	seeds := map[string]bool{}
	nonTargetSeeds := map[string]bool{}
	var dependent []s7ARPurgeProgressDependentWrite
	var analyzeErr error
	parents := s6ASTParents(function.function.Body)
	ast.Inspect(function.function.Body, func(node ast.Node) bool {
		if analyzeErr != nil {
			return false
		}
		if literal, ok := node.(*ast.FuncLit); ok {
			return invokedLiterals[literal]
		}
		switch typed := node.(type) {
		case *ast.CompositeLit:
			if s7ARExpressionHasNamedType(model, typed, "intentArchivePurgeProgressReport") {
				builders = append(
					builders,
					function.function.Name.Name+":"+s7ARNodeString(typed),
				)
			}
			if !s7ARExpressionHasNamedType(model, typed, "intentArchivePurgeReport") ||
				len(typed.Elts) == 0 {
				return true
			}
			keyed := false
			for _, element := range typed.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				keyed = true
				identifier, ok := field.Key.(*ast.Ident)
				if ok && identifier.Name == "PurgeProgress" {
					writes = append(writes, s7ARPurgeProgressWrite{
						rel: function.rel, position: typed.Pos(),
						label: function.function.Name.Name + ":composite-keyed",
					})
				}
			}
			if !keyed {
				fieldIndex, fieldErr := s7ARPurgeProgressFieldIndex(
					model.expressionTypes[typed].Type,
				)
				label := function.function.Name.Name + ":composite-unresolved"
				if fieldErr == nil && len(typed.Elts) <= fieldIndex {
					return true
				}
				if fieldErr == nil {
					label = function.function.Name.Name + ":composite-unkeyed"
				}
				writes = append(writes, s7ARPurgeProgressWrite{
					rel: function.rel, position: typed.Pos(), label: label,
				})
			}
		case *ast.ValueSpec:
			s7ARUpdatePurgeProgressOrigins(
				model, canonicalField, function, aliases, typed.Names, typed.Values, false,
			)
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				right, resolved := s7ARAssignmentRight(typed.Rhs, index)
				switch {
				case s7ARIsPurgeProgressSelector(model, canonicalField, left):
					kind := "direct-unresolved"
					if resolved {
						kind = "direct-" + s7ARPurgeProgressValueKind(right)
					}
					writes = append(writes, s7ARPurgeProgressWrite{
						rel: function.rel, position: left.Pos(),
						label: function.function.Name.Name + ":" + kind,
					})
				case s7ARIsIndirectPurgeProgressWrite(model, left):
					origin := s7ARPurgeProgressExpressionOrigin(
						model, canonicalField, function, aliases, s7ARDerefExpression(left),
					)
					write := s7ARPurgeProgressWrite{
						rel: function.rel, position: left.Pos(),
						label: function.function.Name.Name + ":alias-unresolved",
					}
					if resolved {
						write.label = function.function.Name.Name +
							":alias-" + s7ARPurgeProgressValueKind(right)
					}
					if origin.target && origin.nonTarget {
						analyzeErr = fmt.Errorf(
							"%s:%s purge_progress pointer store has ambiguous canonical/decoy origin",
							function.rel, function.function.Name.Name,
						)
						return false
					}
					if origin.unknown ||
						!origin.target && !origin.nonTarget && len(origin.parameters) == 0 {
						analyzeErr = fmt.Errorf(
							"%s:%s purge_progress pointer store target is unresolved",
							function.rel, function.function.Name.Name,
						)
						return false
					}
					if origin.target {
						writes = append(writes, write)
					}
					for parameter := range origin.parameters {
						dependent = append(dependent, s7ARPurgeProgressDependentWrite{
							parameter: parameter, write: write,
						})
					}
				}
			}
			s7ARUpdatePurgeProgressOrigins(
				model, canonicalField, function, aliases,
				s7ARAssignmentIdentifiers(typed.Lhs), typed.Rhs,
				s7ARPurgeProgressConditionalAssignment(
					parents, typed, function.function.Body,
				),
			)
		case *ast.CallExpr:
			for index, argument := range typed.Args {
				origin := s7ARPurgeProgressExpressionOrigin(
					model, canonicalField, function, aliases, argument,
				)
				if !origin.target && !origin.nonTarget &&
					len(origin.parameters) == 0 && !origin.unknown {
					continue
				}
				if origin.unknown || origin.target && origin.nonTarget {
					analyzeErr = fmt.Errorf(
						"%s:%s pointer-capable call %s has ambiguous origin",
						function.rel, function.function.Name.Name, s6CallName(typed),
					)
					return false
				}
				calleeKey, resolved := s7ARPurgeProgressLocalCallee(model, typed)
				callee := functions[calleeKey]
				if !resolved || callee == nil {
					analyzeErr = fmt.Errorf(
						"%s:%s pointer-capable call %s is unresolved",
						function.rel, function.function.Name.Name, s6CallName(typed),
					)
					return false
				}
				if index >= len(callee.parameters) {
					analyzeErr = fmt.Errorf(
						"%s:%s pointer-capable argument %d has no resolved callee parameter",
						function.rel, function.function.Name.Name, index,
					)
					return false
				}
				target := s7ARPurgeProgressParameter(callee.key, index)
				if origin.target {
					seeds[target] = true
				}
				if origin.nonTarget {
					nonTargetSeeds[target] = true
				}
				for source := range origin.parameters {
					if edges[source] == nil {
						edges[source] = map[string]bool{}
					}
					edges[source][target] = true
				}
			}
		}
		return true
	})
	return writes, builders, edges, seeds, nonTargetSeeds, dependent, analyzeErr
}

type s7ARNamedCallableTarget struct {
	declaration   *ast.FuncDecl
	receiver      ast.Expr
	receiverBound bool
}

type s7ARCallableResolutionContext struct {
	arguments   []ast.Expr
	invoked     bool
	transported bool
}

type s7ARCallableExpressionOrigin struct {
	function   *ast.FuncDecl
	expression ast.Expr
}

type s7ARCallableSequenceAlternative []s7ARCallableExpressionOrigin

type s7ARCallableSequenceOrigins struct {
	alternatives     []s7ARCallableSequenceAlternative
	incomplete       bool
	uncertainIndices map[int]bool
}

type s7ARCallableOriginState struct {
	values            map[types.Object][]ast.Expr
	bindings          map[types.Object][]s7ARCallableExpressionOrigin
	snapshotBindings  map[types.Object]bool
	sequences         map[types.Object]s7ARCallableSequenceOrigins
	elements          map[types.Object]s7ARCallableSequenceOrigins
	elementIncomplete map[types.Object]bool
}

func s7ARNewCallableOriginState() *s7ARCallableOriginState {
	return &s7ARCallableOriginState{
		values:            map[types.Object][]ast.Expr{},
		bindings:          map[types.Object][]s7ARCallableExpressionOrigin{},
		snapshotBindings:  map[types.Object]bool{},
		sequences:         map[types.Object]s7ARCallableSequenceOrigins{},
		elements:          map[types.Object]s7ARCallableSequenceOrigins{},
		elementIncomplete: map[types.Object]bool{},
	}
}

type s7ARCallableInvocationDemand struct {
	all     bool
	indices map[int]bool
}

func (demand s7ARCallableInvocationDemand) any() bool {
	return demand.all || len(demand.indices) != 0
}

func (demand *s7ARCallableInvocationDemand) addIndex(index int) {
	if demand == nil || demand.all {
		return
	}
	if index < 0 {
		demand.all = true
		demand.indices = nil
		return
	}
	if demand.indices == nil {
		demand.indices = map[int]bool{}
	}
	demand.indices[index] = true
}

func (demand *s7ARCallableInvocationDemand) addAll() {
	if demand == nil {
		return
	}
	demand.all = true
	demand.indices = nil
}

func (demand *s7ARCallableInvocationDemand) merge(
	source s7ARCallableInvocationDemand,
) {
	if source.all {
		demand.addAll()
		return
	}
	for index := range source.indices {
		demand.addIndex(index)
	}
}

func s7ARCloneCallableInvocationDemand(
	source s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	result := s7ARCallableInvocationDemand{all: source.all}
	for index := range source.indices {
		result.addIndex(index)
	}
	return result
}

func s7ARCallableInvocationDemandEqual(
	left s7ARCallableInvocationDemand,
	right s7ARCallableInvocationDemand,
) bool {
	if left.all != right.all || len(left.indices) != len(right.indices) {
		return false
	}
	for index := range left.indices {
		if !right.indices[index] {
			return false
		}
	}
	return true
}

func s7ARInvokedFunctionLiterals(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
) map[*ast.FuncLit]bool {
	invoked, _ := s7ARInvokedFunctionLiteralsFromRoots(model, function)
	return invoked
}

func s7ARInvokedFunctionLiteralsForPackage(
	model *s6SourceTypeModel,
) (map[*ast.FuncLit]bool, error) {
	var roots []*ast.FuncDecl
	for _, file := range model.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Body != nil {
				roots = append(roots, function)
			}
		}
	}
	return s7ARInvokedFunctionLiteralsFromRoots(model, roots...)
}

func s7ARInvokedFunctionLiteralsFromRoots(
	model *s6SourceTypeModel,
	roots ...*ast.FuncDecl,
) (map[*ast.FuncLit]bool, error) {
	namedFunctions := map[*types.Func]*ast.FuncDecl{}
	for _, file := range model.files {
		for _, declaration := range file.Decls {
			candidate, ok := declaration.(*ast.FuncDecl)
			if !ok || candidate.Body == nil {
				continue
			}
			object, ok := model.definitions[candidate.Name].(*types.Func)
			if ok {
				namedFunctions[object] = candidate
			}
		}
	}
	aliases := map[types.Object]map[*ast.FuncLit]bool{}
	origins := s7ARNewCallableOriginState()
	receiverObjects := map[types.Object]bool{}
	activeFunctions := map[*ast.FuncDecl]bool{}
	for _, function := range roots {
		if function != nil && function.Body != nil {
			activeFunctions[function] = true
		}
	}
	invoked := map[*ast.FuncLit]bool{}
	for changed := true; changed; {
		changed = false
		functions := make([]*ast.FuncDecl, 0, len(activeFunctions))
		for candidate := range activeFunctions {
			functions = append(functions, candidate)
		}
		sort.Slice(functions, func(left, right int) bool {
			return functions[left].Pos() < functions[right].Pos()
		})
		for _, active := range functions {
			callStates := s7ARCallableReachingStatesAtCalls(
				model, active, aliases, origins, receiverObjects, namedFunctions,
			)
			ast.Inspect(active.Body, func(node ast.Node) bool {
				if literal, ok := node.(*ast.FuncLit); ok {
					return invoked[literal]
				}
				if ranged, ok := node.(*ast.RangeStmt); ok {
					identifier, _ := ranged.Value.(*ast.Ident)
					if identifier != nil && identifier.Name != "_" &&
						s7ARCallableSequenceDerivesFromBoundOrigin(
							model, active, ranged.X, origins,
							map[ast.Expr]bool{},
						) {
						object := model.definitions[identifier]
						sequence := s7ARExpandedCallableExpressions(
							model, active, ranged.X, aliases, origins,
							receiverObjects, map[ast.Expr]bool{},
						)
						elements := s7ARCallableDynamicElementOrigins(sequence)
						merged, elementChanged := s7ARMergeCallableSequenceOrigins(
							origins.elements[object], elements,
						)
						if elementChanged {
							origins.elements[object] = merged
							changed = true
						}
					}
				}
				var left []*ast.Ident
				var right []ast.Expr
				switch typed := node.(type) {
				case *ast.ValueSpec:
					left, right = typed.Names, typed.Values
				case *ast.AssignStmt:
					left, right = s7ARAssignmentIdentifiers(typed.Lhs), typed.Rhs
				}
				for index, identifier := range left {
					if identifier == nil {
						continue
					}
					expression, resolved := s7ARAssignmentRight(right, index)
					if !resolved {
						continue
					}
					object := model.definitions[identifier]
					if object == nil {
						object = model.uses[identifier]
					}
					if object == nil {
						continue
					}
					if !s7ARExpressionSliceContains(origins.values[object], expression) {
						origins.values[object] = append(origins.values[object], expression)
					}
					literals, _ := s7ARFunctionLiteralExpressions(
						model, active, expression, aliases, origins,
						receiverObjects,
						map[ast.Expr]bool{},
					)
					if len(literals) == 0 {
						continue
					}
					if aliases[object] == nil {
						aliases[object] = map[*ast.FuncLit]bool{}
					}
					for literal := range literals {
						if aliases[object][literal] {
							continue
						}
						aliases[object][literal] = true
						changed = true
					}
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if s7ARFixedCallableSliceElementInvocation(
					model, active, call.Fun,
				) {
					return true
				}
				reaching := callStates[call]
				literalTargets, _ :=
					s7ARFunctionLiteralExpressionsAtDemandStates(
						model, active, call.Fun, reaching,
						aliases, origins, receiverObjects,
					)
				for literal := range literalTargets {
					if !invoked[literal] {
						invoked[literal] = true
						changed = true
					}
					if s7ARBindCallableArguments(
						model, active, call.Args, literal.Type.Params,
						call.Ellipsis != token.NoPos,
						aliases, origins, receiverObjects, callStates[call],
					) {
						changed = true
					}
				}
				namedTargets, _ := s7ARNamedCallableTargetsAtDemandStates(
					model, active, call.Fun, reaching,
					aliases, origins, receiverObjects, namedFunctions,
					s7ARCallableResolutionContext{
						arguments: call.Args,
						invoked:   true,
					},
				)
				for _, target := range namedTargets {
					if !activeFunctions[target.declaration] {
						activeFunctions[target.declaration] = true
						changed = true
					}
					argumentOffset := 0
					receiverArgument := target.receiver
					if target.declaration.Recv != nil && !target.receiverBound {
						argumentOffset = 1
						if len(call.Args) != 0 {
							receiverArgument = call.Args[0]
						}
					}
					if receiverArgument != nil &&
						s7ARBindCallableArguments(
							model, active, []ast.Expr{receiverArgument}, target.declaration.Recv,
							false,
							aliases, origins, receiverObjects, callStates[call],
						) {
						changed = true
					}
					if receiverArgument != nil &&
						s7ARMarkCallableReceiverObjects(
							model, target.declaration.Recv, receiverObjects,
						) {
						changed = true
					}
					if s7ARBindCallableArguments(
						model, active, call.Args[argumentOffset:], target.declaration.Type.Params,
						call.Ellipsis != token.NoPos,
						aliases, origins, receiverObjects, callStates[call],
					) {
						changed = true
					}
				}
				if len(namedTargets) == 0 && len(literalTargets) == 0 {
					for _, argument := range call.Args {
						literals, _ := s7ARFunctionLiteralExpressions(
							model, active, argument, aliases, origins,
							receiverObjects,
							map[ast.Expr]bool{},
						)
						for literal := range literals {
							if !invoked[literal] {
								invoked[literal] = true
								changed = true
							}
						}
					}
				}
				return true
			})
		}
	}
	var unresolved []string
	for function := range activeFunctions {
		callStates := s7ARCallableReachingStatesAtCalls(
			model, function, aliases, origins, receiverObjects, namedFunctions,
		)
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if literal, ok := node.(*ast.FuncLit); ok {
				return invoked[literal]
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if s7ARFixedCallableSliceElementInvocation(
				model, function, call.Fun,
			) {
				return true
			}
			reaching := callStates[call]
			literalTargets, literalIncomplete :=
				s7ARFunctionLiteralExpressionsAtDemandStates(
					model, function, call.Fun, reaching, aliases, origins,
					receiverObjects,
				)
			namedTargets, namedIncomplete :=
				s7ARNamedCallableTargetsAtDemandStates(
					model, function, call.Fun, reaching, aliases, origins,
					receiverObjects, namedFunctions,
					s7ARCallableResolutionContext{
						arguments: call.Args,
						invoked:   true,
					},
				)
			expansionIncomplete := false
			if call.Ellipsis != token.NoPos {
				for literal := range literalTargets {
					if s7ARExpandedCallableArgumentsIncomplete(
						model, function, call.Args, literal.Type.Params,
						literal.Body,
						aliases, origins, receiverObjects, namedFunctions,
						reaching,
					) {
						expansionIncomplete = true
					}
				}
				for _, target := range namedTargets {
					argumentOffset := 0
					if target.declaration.Recv != nil && !target.receiverBound {
						argumentOffset = 1
					}
					if s7ARExpandedCallableArgumentsIncomplete(
						model, function, call.Args[argumentOffset:],
						target.declaration.Type.Params,
						target.declaration.Body,
						aliases, origins, receiverObjects, namedFunctions,
						reaching,
					) {
						expansionIncomplete = true
					}
				}
			}
			if namedIncomplete ||
				(literalIncomplete && len(namedTargets) == 0) ||
				expansionIncomplete {
				unresolved = append(
					unresolved,
					fmt.Sprintf("%s:%s", function.Name.Name, s7ARNodeString(call.Fun)),
				)
			}
			return true
		})
	}
	if len(unresolved) != 0 {
		sort.Strings(unresolved)
		return invoked, fmt.Errorf(
			"invoked function-valued result has unresolved callable origins: %v",
			unresolved,
		)
	}
	return invoked, nil
}

func s7ARFixedCallableSliceElementInvocation(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
) bool {
	indexed, ok := s7ARUnwrapCallExpression(expression).(*ast.IndexExpr)
	if !ok || function == nil {
		return false
	}
	object := s7ARCallableSequenceIdentifierObject(model, indexed.X)
	if object == nil || !s7ARCallableSequenceObject(object) {
		return false
	}
	parameters, variadic := s7ARCallableParameters(function.Type.Params)
	for index, parameter := range parameters {
		if parameter == nil || model.definitions[parameter] != object {
			continue
		}
		return !variadic || index != len(parameters)-1
	}
	return false
}

func s7ARNamedCallableTargets(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	resolving map[ast.Expr]bool,
	context s7ARCallableResolutionContext,
) ([]s7ARNamedCallableTarget, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	if expression == nil || resolving[expression] {
		return nil, expression != nil
	}
	resolving[expression] = true
	defer delete(resolving, expression)

	switch typed := expression.(type) {
	case *ast.FuncLit:
		return nil, false
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		if functionObject, ok := object.(*types.Func); ok {
			if declaration := namedFunctions[functionObject]; declaration != nil {
				return []s7ARNamedCallableTarget{{declaration: declaration}}, false
			}
			return nil, false
		}
		if !s7ARFunctionValuedObjectOrExpression(model, expression) {
			return nil, false
		}
		var candidates []ast.Expr
		candidates = append(candidates, origins.values[object]...)
		if s7ARPackageScopeObject(object) {
			candidates = append(candidates, s7ARObjectInitializers(model, object)...)
		}
		candidates = s7ARUniqueExpressions(candidates)
		elements, hasElements := origins.elements[object]
		if len(candidates) == 0 && !hasElements {
			return nil, context.transported
		}
		var result []s7ARNamedCallableTarget
		incomplete := hasElements &&
			(elements.incomplete || origins.elementIncomplete[object])
		for _, candidate := range candidates {
			if !s7ARFunctionValuedObjectOrExpression(model, candidate) {
				continue
			}
			targets, unresolved := s7ARNamedCallableTargets(
				model, function, candidate, aliases, origins, receiverObjects,
				namedFunctions, resolving, s7ARCallableResolutionContext{
					arguments:   context.arguments,
					invoked:     context.invoked,
					transported: true,
				},
			)
			incomplete = incomplete || unresolved
			result = s7ARAppendNamedCallableTargets(result, targets...)
		}
		for _, alternative := range elements.alternatives {
			for _, origin := range alternative {
				if origin.expression == nil {
					incomplete = true
					continue
				}
				originFunction := origin.function
				if originFunction == nil {
					originFunction = function
				}
				targets, unresolved := s7ARNamedCallableTargets(
					model, originFunction, origin.expression, aliases, origins,
					receiverObjects, namedFunctions, resolving,
					s7ARCallableResolutionContext{
						arguments:   context.arguments,
						invoked:     context.invoked,
						transported: true,
					},
				)
				literals, literalUnresolved := s7ARFunctionLiteralExpressions(
					model, originFunction, origin.expression, aliases, origins,
					receiverObjects, map[ast.Expr]bool{},
				)
				incomplete = incomplete || unresolved || literalUnresolved ||
					len(targets) == 0 && len(literals) == 0
				result = s7ARAppendNamedCallableTargets(result, targets...)
			}
		}
		return result, incomplete
	case *ast.SelectorExpr:
		selection := model.selections[typed]
		if selection == nil {
			object, _ := model.uses[typed.Sel].(*types.Func)
			if declaration := namedFunctions[object]; declaration != nil {
				return []s7ARNamedCallableTarget{{declaration: declaration}}, false
			}
			return nil, false
		}
		switch selection.Kind() {
		case types.MethodExpr:
			object, _ := selection.Obj().(*types.Func)
			if declaration := namedFunctions[object]; declaration != nil {
				return []s7ARNamedCallableTarget{{declaration: declaration}}, false
			}
			if !s7ARInterfaceMethodSelection(model, function, selection) {
				return nil, false
			}
			if !context.invoked {
				return nil, false
			}
			if len(context.arguments) == 0 {
				return nil, true
			}
			return s7ARConcreteInterfaceMethodTargets(
				model, function, context.arguments[0], selection, origins,
				namedFunctions, false,
			)
		case types.MethodVal:
			object, _ := selection.Obj().(*types.Func)
			if declaration := namedFunctions[object]; declaration != nil {
				return []s7ARNamedCallableTarget{{
					declaration:   declaration,
					receiver:      typed.X,
					receiverBound: true,
				}}, false
			}
			if !s7ARInterfaceMethodSelection(model, function, selection) {
				return nil, false
			}
			if !context.invoked {
				return nil, false
			}
			return s7ARConcreteInterfaceMethodTargets(
				model, function, typed.X, selection, origins, namedFunctions,
				true,
			)
		default:
			return nil, false
		}
	case *ast.IndexExpr:
		if !s7ARCallableSequenceDerivesFromBoundOrigin(
			model, function, typed.X, origins, map[ast.Expr]bool{},
		) {
			return s7ARNamedCallableTargets(
				model, function, typed.X, aliases, origins, receiverObjects,
				namedFunctions, resolving, context,
			)
		}
		sequence := s7ARExpandedCallableExpressions(
			model, function, typed.X, aliases, origins, receiverObjects,
			map[ast.Expr]bool{},
		)
		candidates, incomplete := s7ARCallableSequenceIndexOrigins(
			model, sequence, typed.Index,
		)
		var result []s7ARNamedCallableTarget
		for _, candidate := range candidates {
			originFunction := candidate.function
			if originFunction == nil {
				originFunction = function
			}
			targets, unresolved := s7ARNamedCallableTargets(
				model, originFunction, candidate.expression, aliases, origins,
				receiverObjects, namedFunctions, resolving, context,
			)
			literals, literalUnresolved := s7ARFunctionLiteralExpressions(
				model, originFunction, candidate.expression, aliases, origins,
				receiverObjects, map[ast.Expr]bool{},
			)
			incomplete = incomplete || unresolved || literalUnresolved ||
				len(targets) == 0 && len(literals) == 0
			result = s7ARAppendNamedCallableTargets(result, targets...)
		}
		return result, incomplete
	case *ast.IndexListExpr:
		return s7ARNamedCallableTargets(
			model, function, typed.X, aliases, origins, receiverObjects,
			namedFunctions, resolving, context,
		)
	case *ast.CallExpr:
		if len(typed.Args) == 1 {
			if value, ok := model.expressionTypes[typed.Fun]; ok && value.IsType() {
				return s7ARNamedCallableTargets(
					model, function, typed.Args[0], aliases, origins, receiverObjects,
					namedFunctions, resolving, s7ARCallableResolutionContext{
						arguments:   context.arguments,
						invoked:     context.invoked,
						transported: true,
					},
				)
			}
		}
		if !s7ARFunctionValuedExpression(model, typed) {
			return nil, false
		}
		factories, factoryIncomplete := s7ARNamedCallableTargets(
			model, function, typed.Fun, aliases, origins, receiverObjects,
			namedFunctions, resolving, s7ARCallableResolutionContext{
				arguments: typed.Args,
				invoked:   true,
			},
		)
		literalFactories, literalIncomplete := s7ARFunctionLiteralExpressions(
			model, function, typed.Fun, aliases, origins, receiverObjects,
			map[ast.Expr]bool{},
		)
		incomplete := factoryIncomplete || literalIncomplete
		var result []s7ARNamedCallableTarget
		for _, factory := range factories {
			returned := s7ARCallableReturnExpressions(
				model, factory.declaration.Type, factory.declaration.Body,
			)
			if len(returned) == 0 {
				incomplete = true
			}
			for _, candidate := range returned {
				targets, unresolved := s7ARNamedCallableTargets(
					model, factory.declaration, candidate, aliases, origins,
					receiverObjects, namedFunctions, resolving,
					s7ARCallableResolutionContext{
						arguments:   context.arguments,
						invoked:     context.invoked,
						transported: true,
					},
				)
				incomplete = incomplete || unresolved
				result = s7ARAppendNamedCallableTargets(result, targets...)
			}
		}
		for factory := range literalFactories {
			returned := s7ARCallableReturnExpressions(
				model, factory.Type, factory.Body,
			)
			if len(returned) == 0 {
				incomplete = true
			}
			for _, candidate := range returned {
				targets, unresolved := s7ARNamedCallableTargets(
					model, function, candidate, aliases, origins, receiverObjects,
					namedFunctions, resolving, s7ARCallableResolutionContext{
						arguments:   context.arguments,
						invoked:     context.invoked,
						transported: true,
					},
				)
				incomplete = incomplete || unresolved
				result = s7ARAppendNamedCallableTargets(result, targets...)
			}
		}
		if len(factories) == 0 && len(literalFactories) == 0 {
			incomplete = true
		}
		return result, incomplete
	default:
		return nil, false
	}
}

func s7ARConcreteInterfaceMethodTargets(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	receiver ast.Expr,
	selection *types.Selection,
	origins *s7ARCallableOriginState,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	receiverBound bool,
) ([]s7ARNamedCallableTarget, bool) {
	concreteTypes, incomplete := s7ARConcreteReceiverTypes(
		model, function, receiver, origins, map[ast.Expr]bool{},
	)
	var result []s7ARNamedCallableTarget
	for _, concreteType := range concreteTypes {
		object, _, _ := types.LookupFieldOrMethod(
			concreteType, true, selection.Obj().Pkg(), selection.Obj().Name(),
		)
		method, ok := object.(*types.Func)
		if !ok {
			incomplete = true
			continue
		}
		declaration := namedFunctions[method]
		if declaration == nil {
			incomplete = true
			continue
		}
		target := s7ARNamedCallableTarget{
			declaration:   declaration,
			receiverBound: receiverBound,
		}
		if receiverBound {
			target.receiver = receiver
		}
		result = s7ARAppendNamedCallableTargets(result, target)
	}
	return result, incomplete
}

func s7ARConcreteReceiverTypes(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	origins *s7ARCallableOriginState,
	resolving map[ast.Expr]bool,
) ([]types.Type, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	if expression == nil || resolving[expression] {
		return nil, expression != nil
	}
	resolving[expression] = true
	defer delete(resolving, expression)

	var result []types.Type
	incomplete := false
	if typed, ok := model.expressionTypes[expression]; ok &&
		typed.Type != nil && !s7ARInterfaceType(typed.Type) {
		result = s7ARAppendUniqueTypes(result, typed.Type)
	}
	if len(result) != 0 {
		return result, false
	}
	switch typed := expression.(type) {
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		var candidates []ast.Expr
		candidates = append(candidates, origins.values[object]...)
		if s7ARPackageScopeObject(object) {
			candidates = append(candidates, s7ARObjectInitializers(model, object)...)
		}
		candidates = s7ARUniqueExpressions(candidates)
		if len(result) == 0 && len(candidates) == 0 {
			incomplete = true
		}
		for _, candidate := range candidates {
			typesFound, unresolved := s7ARConcreteReceiverTypes(
				model, function, candidate, origins, resolving,
			)
			incomplete = incomplete || unresolved
			for _, concreteType := range typesFound {
				result = s7ARAppendUniqueTypes(result, concreteType)
			}
		}
	case *ast.UnaryExpr:
		if typed.Op == token.AND || typed.Op == token.MUL {
			typesFound, unresolved := s7ARConcreteReceiverTypes(
				model, function, typed.X, origins, resolving,
			)
			incomplete = incomplete || unresolved
			for _, concreteType := range typesFound {
				result = s7ARAppendUniqueTypes(result, concreteType)
			}
		}
	case *ast.StarExpr:
		typesFound, unresolved := s7ARConcreteReceiverTypes(
			model, function, typed.X, origins, resolving,
		)
		incomplete = incomplete || unresolved
		for _, concreteType := range typesFound {
			result = s7ARAppendUniqueTypes(result, concreteType)
		}
	case *ast.SelectorExpr:
		selection := model.selections[typed]
		if selection != nil && selection.Kind() == types.FieldVal {
			candidates, unresolved := s7ARCallableFieldExpressions(
				model, function, typed.X, selection.Index(), typed.Sel.Name,
				origins, map[types.Object]bool{}, map[ast.Expr]bool{}, false,
			)
			incomplete = incomplete || unresolved
			for _, candidate := range candidates {
				typesFound, unresolved := s7ARConcreteReceiverTypes(
					model, function, candidate, origins, resolving,
				)
				incomplete = incomplete || unresolved
				for _, concreteType := range typesFound {
					result = s7ARAppendUniqueTypes(result, concreteType)
				}
			}
		}
	case *ast.CallExpr:
		if len(typed.Args) == 1 {
			if value, ok := model.expressionTypes[typed.Fun]; ok && value.IsType() {
				typesFound, unresolved := s7ARConcreteReceiverTypes(
					model, function, typed.Args[0], origins, resolving,
				)
				incomplete = incomplete || unresolved
				for _, concreteType := range typesFound {
					result = s7ARAppendUniqueTypes(result, concreteType)
				}
				return result, incomplete
			}
		}
		declaration := s7ARNamedFunctionDeclaration(model, typed.Fun)
		if declaration == nil {
			incomplete = true
			break
		}
		returned := s7ARCallableReturnExpressions(
			model, declaration.Type, declaration.Body,
		)
		if len(returned) == 0 {
			incomplete = true
		}
		for _, candidate := range returned {
			typesFound, unresolved := s7ARConcreteReceiverTypes(
				model, declaration, candidate, origins, resolving,
			)
			incomplete = incomplete || unresolved
			for _, concreteType := range typesFound {
				result = s7ARAppendUniqueTypes(result, concreteType)
			}
		}
	}
	return result, incomplete
}

func s7ARInterfaceMethodSelection(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	selection *types.Selection,
) bool {
	if selection == nil {
		return false
	}
	receiver := types.Unalias(selection.Recv())
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	if _, ok := receiver.Underlying().(*types.Interface); !ok {
		return false
	}
	method, _ := selection.Obj().(*types.Func)
	caller, _ := model.definitions[function.Name].(*types.Func)
	return method != nil && method.Pkg() != nil &&
		caller != nil && caller.Pkg() != nil &&
		method.Pkg().Path() == caller.Pkg().Path() &&
		s7ARSignatureCarriesCallable(method)
}

func s7ARSignatureCarriesCallable(method *types.Func) bool {
	signature, _ := method.Type().(*types.Signature)
	if signature == nil {
		return false
	}
	for _, tuple := range []*types.Tuple{signature.Params(), signature.Results()} {
		for index := 0; tuple != nil && index < tuple.Len(); index++ {
			if s7ARFunctionValuedType(tuple.At(index).Type()) {
				return true
			}
		}
	}
	if signature.Variadic() && signature.Params() != nil &&
		signature.Params().Len() != 0 {
		last := signature.Params().At(signature.Params().Len() - 1).Type()
		slice, _ := types.Unalias(last).Underlying().(*types.Slice)
		if slice != nil && s7ARFunctionValuedType(slice.Elem()) {
			return true
		}
	}
	return false
}

func s7ARInterfaceType(value types.Type) bool {
	value = types.Unalias(value)
	if pointer, ok := value.(*types.Pointer); ok {
		value = types.Unalias(pointer.Elem())
	}
	_, ok := value.Underlying().(*types.Interface)
	return ok
}

func s7ARAppendUniqueTypes(values []types.Type, candidate types.Type) []types.Type {
	for _, value := range values {
		if types.Identical(value, candidate) {
			return values
		}
	}
	return append(values, candidate)
}

func s7ARAppendNamedCallableTargets(
	targets []s7ARNamedCallableTarget,
	candidates ...s7ARNamedCallableTarget,
) []s7ARNamedCallableTarget {
	for _, candidate := range candidates {
		found := false
		for _, target := range targets {
			if target.declaration == candidate.declaration &&
				target.receiver == candidate.receiver &&
				target.receiverBound == candidate.receiverBound {
				found = true
				break
			}
		}
		if !found && candidate.declaration != nil {
			targets = append(targets, candidate)
		}
	}
	return targets
}

func s7ARUniqueExpressions(expressions []ast.Expr) []ast.Expr {
	var result []ast.Expr
	for _, expression := range expressions {
		if expression != nil && !s7ARExpressionSliceContains(result, expression) {
			result = append(result, expression)
		}
	}
	return result
}

func s7ARPackageScopeObject(object types.Object) bool {
	return object != nil && object.Pkg() != nil &&
		object.Parent() == object.Pkg().Scope()
}

func s7ARFunctionLiteralExpressions(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	resolving map[ast.Expr]bool,
) (map[*ast.FuncLit]bool, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	if expression == nil || resolving[expression] {
		return nil, expression != nil
	}
	resolving[expression] = true
	defer delete(resolving, expression)
	if literal, ok := expression.(*ast.FuncLit); ok {
		return map[*ast.FuncLit]bool{literal: true}, false
	}
	switch typed := expression.(type) {
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		result := s7ARCloneFunctionLiteralSet(aliases[object])
		incomplete := false
		for _, origin := range origins.values[object] {
			if !s7ARCallableOriginMayReturnFunction(model, origin) {
				continue
			}
			literals, unresolved := s7ARFunctionLiteralExpressions(
				model, function, origin, aliases, origins, receiverObjects, resolving,
			)
			s7ARMergeFunctionLiteralSet(result, literals)
			incomplete = incomplete || unresolved
		}
		if elements, ok := origins.elements[object]; ok {
			incomplete = incomplete || elements.incomplete ||
				origins.elementIncomplete[object]
			for _, alternative := range elements.alternatives {
				for _, origin := range alternative {
					originFunction := origin.function
					if originFunction == nil {
						originFunction = function
					}
					literals, unresolved := s7ARFunctionLiteralExpressions(
						model, originFunction, origin.expression, aliases, origins,
						receiverObjects, resolving,
					)
					s7ARMergeFunctionLiteralSet(result, literals)
					incomplete = incomplete || unresolved || len(literals) == 0
				}
			}
		}
		return result, incomplete
	case *ast.SelectorExpr:
		selection := model.selections[typed]
		if selection == nil || selection.Kind() != types.FieldVal ||
			!s7ARFunctionValuedType(selection.Obj().Type()) {
			return nil, false
		}
		result := map[*ast.FuncLit]bool{}
		candidates, incomplete := s7ARCallableFieldExpressions(
			model, function, typed.X, selection.Index(), typed.Sel.Name,
			origins, receiverObjects, map[ast.Expr]bool{}, false,
		)
		for _, candidate := range candidates {
			literals, unresolved := s7ARFunctionLiteralExpressions(
				model, function, candidate, aliases, origins, receiverObjects, resolving,
			)
			incomplete = incomplete || unresolved
			for literal := range literals {
				result[literal] = true
			}
		}
		return result, incomplete
	case *ast.IndexExpr:
		if !s7ARCallableSequenceDerivesFromBoundOrigin(
			model, function, typed.X, origins, map[ast.Expr]bool{},
		) {
			return s7ARFunctionLiteralExpressions(
				model, function, typed.X, aliases, origins, receiverObjects,
				resolving,
			)
		}
		sequence := s7ARExpandedCallableExpressions(
			model, function, typed.X, aliases, origins, receiverObjects,
			map[ast.Expr]bool{},
		)
		candidates, incomplete := s7ARCallableSequenceIndexOrigins(
			model, sequence, typed.Index,
		)
		result := map[*ast.FuncLit]bool{}
		for _, candidate := range candidates {
			originFunction := candidate.function
			if originFunction == nil {
				originFunction = function
			}
			literals, unresolved := s7ARFunctionLiteralExpressions(
				model, originFunction, candidate.expression, aliases, origins,
				receiverObjects, resolving,
			)
			s7ARMergeFunctionLiteralSet(result, literals)
			incomplete = incomplete || unresolved || len(literals) == 0
		}
		return result, incomplete
	case *ast.IndexListExpr:
		return s7ARFunctionLiteralExpressions(
			model, function, typed.X, aliases, origins, receiverObjects, resolving,
		)
	case *ast.CallExpr:
		if len(typed.Args) == 1 {
			identifier, ok := s7ARUnwrapCallExpression(typed.Fun).(*ast.Ident)
			if ok {
				if _, conversion := model.uses[identifier].(*types.TypeName); conversion {
					return s7ARFunctionLiteralExpressions(
						model, function, typed.Args[0], aliases, origins, receiverObjects, resolving,
					)
				}
			}
		}
		if !s7ARFunctionValuedExpression(model, typed) {
			return nil, false
		}
		result := map[*ast.FuncLit]bool{}
		incomplete := false
		factories, factoryIncomplete := s7ARFunctionLiteralExpressions(
			model, function, typed.Fun, aliases, origins, receiverObjects, resolving,
		)
		incomplete = incomplete || factoryIncomplete
		for factory := range factories {
			returned := s7ARCallableReturnExpressions(
				model, factory.Type, factory.Body,
			)
			if len(returned) == 0 {
				incomplete = true
			}
			for _, candidate := range returned {
				literals, unresolved := s7ARFunctionLiteralExpressions(
					model, function, candidate, aliases, origins, receiverObjects, resolving,
				)
				incomplete = incomplete || unresolved || len(literals) == 0
				for literal := range literals {
					result[literal] = true
				}
			}
		}
		if declaration := s7ARNamedFunctionDeclaration(model, typed.Fun); declaration != nil {
			returned := s7ARCallableReturnExpressions(
				model, declaration.Type, declaration.Body,
			)
			if len(returned) == 0 {
				incomplete = true
			}
			for _, candidate := range returned {
				literals, unresolved := s7ARFunctionLiteralExpressions(
					model, declaration, candidate, aliases, origins, receiverObjects, resolving,
				)
				incomplete = incomplete || unresolved || len(literals) == 0
				for literal := range literals {
					result[literal] = true
				}
			}
		} else if len(factories) == 0 {
			incomplete = true
		}
		return result, incomplete
	default:
		return nil, false
	}
}

func s7ARCallableFieldExpressions(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	receiver ast.Expr,
	fieldPath []int,
	fieldName string,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	resolving map[ast.Expr]bool,
	strict bool,
) ([]ast.Expr, bool) {
	receiver = s7ARUnwrapCallExpression(receiver)
	if receiver == nil || len(fieldPath) == 0 || resolving[receiver] {
		return nil, strict
	}
	resolving[receiver] = true
	defer delete(resolving, receiver)

	switch typed := receiver.(type) {
	case *ast.UnaryExpr:
		if typed.Op == token.AND || typed.Op == token.MUL {
			return s7ARCallableFieldExpressions(
				model, function, typed.X, fieldPath, fieldName, origins,
				receiverObjects, resolving, strict,
			)
		}
	case *ast.StarExpr:
		return s7ARCallableFieldExpressions(
			model, function, typed.X, fieldPath, fieldName, origins,
			receiverObjects, resolving, strict,
		)
	case *ast.CompositeLit:
		value, ok := s7ARCompositeFieldExpression(model, typed, fieldPath[0])
		if !ok {
			return nil, strict
		}
		if len(fieldPath) == 1 {
			return []ast.Expr{value}, false
		}
		return s7ARCallableFieldExpressions(
			model, function, value, fieldPath[1:], fieldName, origins,
			receiverObjects, resolving, strict,
		)
	case *ast.Ident:
		var result []ast.Expr
		bindings := s6FunctionBindingsBefore(function, receiver.Pos())
		if len(fieldPath) == 1 {
			for _, candidate := range bindings.fieldCandidates(typed, fieldName) {
				if !s7ARExpressionSliceContains(result, candidate) {
					result = append(result, candidate)
				}
			}
		}
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		strict = strict || receiverObjects[object]
		var candidates []ast.Expr
		candidates = append(candidates, origins.values[object]...)
		candidates = append(candidates, bindings.candidates(typed)...)
		candidates = append(candidates, s7ARObjectInitializers(model, object)...)
		incomplete := false
		for _, candidate := range candidates {
			values, unresolved := s7ARCallableFieldExpressions(
				model, function, candidate, fieldPath, fieldName, origins,
				receiverObjects, resolving, strict,
			)
			incomplete = incomplete || unresolved
			for _, value := range values {
				if !s7ARExpressionSliceContains(result, value) {
					result = append(result, value)
				}
			}
		}
		if len(result) == 0 {
			incomplete = strict
		}
		return result, incomplete
	case *ast.SelectorExpr:
		selection := model.selections[typed]
		if selection == nil || selection.Kind() != types.FieldVal {
			return nil, strict
		}
		receivers, incomplete := s7ARCallableFieldExpressions(
			model, function, typed.X, selection.Index(), typed.Sel.Name,
			origins, receiverObjects, resolving, strict,
		)
		var result []ast.Expr
		for _, candidate := range receivers {
			values, unresolved := s7ARCallableFieldExpressions(
				model, function, candidate, fieldPath, fieldName, origins,
				receiverObjects, resolving, strict,
			)
			incomplete = incomplete || unresolved
			for _, value := range values {
				if !s7ARExpressionSliceContains(result, value) {
					result = append(result, value)
				}
			}
		}
		if len(result) == 0 {
			incomplete = strict
		}
		return result, incomplete
	case *ast.CallExpr:
		if len(typed.Args) == 1 {
			identifier, ok := s7ARUnwrapCallExpression(typed.Fun).(*ast.Ident)
			if ok {
				if _, conversion := model.uses[identifier].(*types.TypeName); conversion {
					return s7ARCallableFieldExpressions(
						model, function, typed.Args[0], fieldPath, fieldName,
						origins, receiverObjects, resolving, strict,
					)
				}
			}
		}
		declaration := s7ARNamedFunctionDeclaration(model, typed.Fun)
		if declaration == nil {
			return nil, strict
		}
		returned := s7ARCallableReturnExpressions(
			model, declaration.Type, declaration.Body,
		)
		incomplete := strict && len(returned) == 0
		var result []ast.Expr
		for _, candidate := range returned {
			values, unresolved := s7ARCallableFieldExpressions(
				model, declaration, candidate, fieldPath, fieldName,
				origins, receiverObjects, resolving, strict,
			)
			incomplete = incomplete || unresolved
			for _, value := range values {
				if !s7ARExpressionSliceContains(result, value) {
					result = append(result, value)
				}
			}
		}
		if len(result) == 0 {
			incomplete = strict
		}
		return result, incomplete
	}
	return nil, strict
}

func s7ARCompositeFieldExpression(
	model *s6SourceTypeModel,
	composite *ast.CompositeLit,
	fieldIndex int,
) (ast.Expr, bool) {
	typed, ok := model.expressionTypes[composite]
	if !ok || typed.Type == nil {
		return nil, false
	}
	value := types.Unalias(typed.Type)
	if pointer, ok := value.(*types.Pointer); ok {
		value = types.Unalias(pointer.Elem())
	}
	structure, ok := value.Underlying().(*types.Struct)
	if !ok || fieldIndex < 0 || fieldIndex >= structure.NumFields() {
		return nil, false
	}
	keyed := false
	for _, element := range composite.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyed = true
		identifier, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		object := model.uses[identifier]
		if object == nil {
			object = model.definitions[identifier]
		}
		if object == structure.Field(fieldIndex) ||
			identifier.Name == structure.Field(fieldIndex).Name() {
			return field.Value, true
		}
	}
	if keyed || fieldIndex >= len(composite.Elts) {
		return nil, false
	}
	return composite.Elts[fieldIndex], true
}

func s7ARObjectInitializers(
	model *s6SourceTypeModel,
	object types.Object,
) []ast.Expr {
	if object == nil {
		return nil
	}
	var result []ast.Expr
	for _, file := range model.files {
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range generic.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, identifier := range value.Names {
					if model.definitions[identifier] != object {
						continue
					}
					expression, resolved := s7ARAssignmentRight(value.Values, index)
					if resolved && !s7ARExpressionSliceContains(result, expression) {
						result = append(result, expression)
					}
				}
			}
		}
	}
	return result
}

func s7ARExpressionSliceContains(expressions []ast.Expr, target ast.Expr) bool {
	for _, expression := range expressions {
		if expression == target {
			return true
		}
	}
	return false
}

func s7ARCloneFunctionLiteralSet(
	source map[*ast.FuncLit]bool,
) map[*ast.FuncLit]bool {
	if len(source) == 0 {
		return map[*ast.FuncLit]bool{}
	}
	result := make(map[*ast.FuncLit]bool, len(source))
	for literal := range source {
		result[literal] = true
	}
	return result
}

func s7ARMergeFunctionLiteralSet(
	target map[*ast.FuncLit]bool,
	source map[*ast.FuncLit]bool,
) {
	for literal := range source {
		target[literal] = true
	}
}

func s7ARCallableOriginMayReturnFunction(
	model *s6SourceTypeModel,
	expression ast.Expr,
) bool {
	expression = s7ARUnwrapCallExpression(expression)
	switch expression.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return s7ARFunctionValuedExpression(model, expression)
	case *ast.CallExpr:
		return s7ARFunctionValuedExpression(model, expression)
	default:
		return false
	}
}

func s7ARFunctionValuedExpression(
	model *s6SourceTypeModel,
	expression ast.Expr,
) bool {
	typed, ok := model.expressionTypes[expression]
	if !ok || typed.Type == nil {
		return false
	}
	return s7ARFunctionValuedType(typed.Type)
}

func s7ARFunctionValuedObjectOrExpression(
	model *s6SourceTypeModel,
	expression ast.Expr,
) bool {
	if s7ARFunctionValuedExpression(model, expression) {
		return true
	}
	identifier, ok := s7ARUnwrapCallExpression(expression).(*ast.Ident)
	if !ok {
		return false
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	return object != nil && s7ARFunctionValuedType(object.Type())
}

func s7ARFunctionValuedType(value types.Type) bool {
	if value == nil {
		return false
	}
	_, ok := types.Unalias(value).Underlying().(*types.Signature)
	return ok
}

func s7ARNamedFunctionDeclaration(
	model *s6SourceTypeModel,
	expression ast.Expr,
) *ast.FuncDecl {
	expression = s7ARUnwrapCallExpression(expression)
	var identifier *ast.Ident
	switch typed := expression.(type) {
	case *ast.Ident:
		identifier = typed
	case *ast.SelectorExpr:
		identifier = typed.Sel
	default:
		return nil
	}
	object, ok := model.uses[identifier].(*types.Func)
	if !ok {
		object, _ = model.definitions[identifier].(*types.Func)
	}
	if object == nil {
		return nil
	}
	for _, file := range model.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && model.definitions[function.Name] == object {
				return function
			}
		}
	}
	return nil
}

func s7ARCallableReturnExpressions(
	model *s6SourceTypeModel,
	signature *ast.FuncType,
	body *ast.BlockStmt,
) []ast.Expr {
	if body == nil {
		return nil
	}
	var namedResults []ast.Expr
	if signature != nil && signature.Results != nil {
		for _, field := range signature.Results.List {
			for _, identifier := range field.Names {
				object := model.definitions[identifier]
				if object == nil || object.Type() == nil {
					continue
				}
				if _, ok := object.Type().Underlying().(*types.Signature); ok {
					namedResults = append(namedResults, identifier)
				}
			}
		}
	}
	var result []ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(typed.Results) == 0 {
				result = append(result, namedResults...)
			} else {
				result = append(result, typed.Results...)
			}
			return false
		default:
			return true
		}
	})
	return result
}

func s7ARBindCallableArguments(
	model *s6SourceTypeModel,
	caller *ast.FuncDecl,
	arguments []ast.Expr,
	fields *ast.FieldList,
	expanded bool,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	reaching []s7ARCallableDemandState,
) bool {
	if fields == nil {
		return false
	}
	parameters, variadic := s7ARCallableParameters(fields)
	if len(parameters) == 0 {
		return false
	}
	fixedCount := len(parameters)
	if variadic {
		fixedCount--
	}
	changed := false
	for index := 0; index < fixedCount; index++ {
		if index >= len(arguments) {
			break
		}
		if s7ARBindCallableArgument(
			model, caller, parameters[index], arguments[index],
			aliases, origins, receiverObjects, reaching,
		) {
			changed = true
		}
	}
	if !variadic {
		return changed
	}
	variadicParameter := parameters[len(parameters)-1]
	if !s7ARVariadicParameterCarriesCallable(model, variadicParameter) {
		var variadicArguments []ast.Expr
		if expanded {
			if fixedCount < len(arguments) {
				variadicArguments = arguments[fixedCount : fixedCount+1]
			}
		} else if fixedCount < len(arguments) {
			variadicArguments = arguments[fixedCount:]
		}
		for _, argument := range variadicArguments {
			if s7ARBindCallableArgument(
				model, caller, variadicParameter, argument,
				aliases, origins, receiverObjects, reaching,
			) {
				changed = true
			}
		}
		return changed
	}

	var sequence s7ARCallableSequenceOrigins
	if expanded {
		if len(arguments) != fixedCount+1 {
			sequence.incomplete = true
		} else {
			sequence = s7ARExpandedCallableExpressionsAtDemandStates(
				model, caller, arguments[fixedCount], reaching,
				aliases, origins, receiverObjects,
			)
		}
	} else {
		alternative := make(
			s7ARCallableSequenceAlternative, 0, len(arguments)-fixedCount,
		)
		for _, argument := range arguments[fixedCount:] {
			alternative = append(alternative, s7ARCallableExpressionOrigin{
				function: caller, expression: argument,
			})
		}
		sequence.alternatives = append(sequence.alternatives, alternative)
	}
	if expanded && len(sequence.alternatives) == 0 {
		return changed
	}
	object := model.definitions[variadicParameter]
	if object == nil {
		return changed
	}
	merged, sequenceChanged := s7ARMergeCallableSequenceOrigins(
		origins.sequences[object], sequence,
	)
	if sequenceChanged {
		origins.sequences[object] = merged
		changed = true
	}
	return changed
}

func s7ARCallableParameters(fields *ast.FieldList) ([]*ast.Ident, bool) {
	if fields == nil {
		return nil, false
	}
	var parameters []*ast.Ident
	variadic := false
	for index, field := range fields.List {
		if len(field.Names) == 0 {
			parameters = append(parameters, nil)
		} else {
			parameters = append(parameters, field.Names...)
		}
		if index == len(fields.List)-1 {
			_, variadic = field.Type.(*ast.Ellipsis)
		}
	}
	return parameters, variadic
}

func s7ARBindCallableArgument(
	model *s6SourceTypeModel,
	caller *ast.FuncDecl,
	parameter *ast.Ident,
	argument ast.Expr,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	reaching []s7ARCallableDemandState,
) bool {
	if parameter == nil || argument == nil {
		return false
	}
	literals, _ := s7ARFunctionLiteralExpressionsAtDemandStates(
		model, caller, argument, reaching, aliases, origins, receiverObjects,
	)
	object := model.definitions[parameter]
	if object == nil {
		return false
	}
	changed := false
	if s7ARCallableSequenceObject(object) &&
		s7ARCallableSequenceExpression(model, argument) {
		sequence := s7ARExpandedCallableExpressionsAtDemandStates(
			model, caller, argument, reaching,
			aliases, origins, receiverObjects,
		)
		merged, sequenceChanged := s7ARMergeCallableSequenceOrigins(
			origins.sequences[object], sequence,
		)
		if sequenceChanged {
			origins.sequences[object] = merged
			changed = true
		}
	}
	bindings := []s7ARCallableExpressionOrigin{{
		function: caller, expression: argument,
	}}
	snapshotReaching := len(reaching) != 0
	for _, state := range reaching {
		snapshotReaching = snapshotReaching && state.operandSnapshot
	}
	if snapshotReaching &&
		object.Type() != nil &&
		s7ARFunctionValuedType(object.Type()) {
		if !origins.snapshotBindings[object] {
			origins.snapshotBindings[object] = true
			changed = true
		}
		var snapshots []s7ARCallableExpressionOrigin
		reliable := true
		analysis := &s7ARCallableDemandAnalysis{
			model: model,
			owner: caller,
		}
		for _, state := range reaching {
			resolved, exact :=
				s7ARCallableScalarOriginsForDemandState(
					analysis, state, argument,
				)
			if !exact || len(resolved) == 0 {
				reliable = false
				break
			}
			for _, candidate := range resolved {
				present := false
				for _, existing := range snapshots {
					if existing == candidate {
						present = true
						break
					}
				}
				if !present {
					snapshots = append(snapshots, candidate)
				}
			}
		}
		if reliable && len(snapshots) != 0 {
			bindings = snapshots
		}
	}
	for _, binding := range bindings {
		bindingPresent := false
		for _, candidate := range origins.bindings[object] {
			if candidate == binding {
				bindingPresent = true
				break
			}
		}
		if !bindingPresent {
			origins.bindings[object] = append(
				origins.bindings[object], binding,
			)
			changed = true
		}
	}
	if !s7ARExpressionSliceContains(origins.values[object], argument) {
		origins.values[object] = append(origins.values[object], argument)
		changed = true
	}
	if len(literals) == 0 {
		return changed
	}
	if aliases[object] == nil {
		aliases[object] = map[*ast.FuncLit]bool{}
	}
	for literal := range literals {
		if aliases[object][literal] {
			continue
		}
		aliases[object][literal] = true
		changed = true
	}
	return changed
}

func s7ARExpandedCallableArgumentsIncomplete(
	model *s6SourceTypeModel,
	caller *ast.FuncDecl,
	arguments []ast.Expr,
	fields *ast.FieldList,
	body *ast.BlockStmt,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	reaching []s7ARCallableDemandState,
) bool {
	parameters, variadic := s7ARCallableParameters(fields)
	if !variadic || len(parameters) == 0 {
		return false
	}
	if !s7ARVariadicParameterCarriesCallable(
		model, parameters[len(parameters)-1],
	) {
		return false
	}
	demand := s7ARCallableVariadicParameterDemand(
		model, caller, body, parameters[len(parameters)-1],
		aliases, origins, receiverObjects, namedFunctions,
		map[*ast.BlockStmt]bool{}, map[*ast.BlockStmt]s7ARCallableInvocationDemand{},
	)
	if !demand.any() {
		return false
	}
	fixedCount := len(parameters) - 1
	if len(arguments) != fixedCount+1 {
		return true
	}
	expanded := s7ARExpandedCallableExpressionsAtDemandStates(
		model, caller, arguments[fixedCount], reaching, aliases, origins,
		receiverObjects,
	)
	if expanded.incomplete {
		return true
	}
	indices := make([]int, 0, len(demand.indices))
	if demand.all {
		maximum := 0
		for _, alternative := range expanded.alternatives {
			if len(alternative) > maximum {
				maximum = len(alternative)
			}
		}
		for index := 0; index < maximum; index++ {
			indices = append(indices, index)
		}
		if maximum == 0 {
			return true
		}
	} else {
		for index := range demand.indices {
			indices = append(indices, index)
		}
		sort.Ints(indices)
	}
	for _, index := range indices {
		if expanded.uncertainIndices[index] {
			return true
		}
		for _, alternative := range expanded.alternatives {
			if index < 0 || index >= len(alternative) ||
				alternative[index].expression == nil {
				return true
			}
			origin := alternative[index]
			originFunction := origin.function
			if originFunction == nil {
				originFunction = caller
			}
			literals, literalIncomplete := s7ARFunctionLiteralExpressions(
				model, originFunction, origin.expression, aliases, origins,
				receiverObjects, map[ast.Expr]bool{},
			)
			named, namedIncomplete := s7ARNamedCallableTargets(
				model, originFunction, origin.expression, aliases, origins,
				receiverObjects, namedFunctions, map[ast.Expr]bool{},
				s7ARCallableResolutionContext{invoked: true, transported: true},
			)
			if literalIncomplete || namedIncomplete ||
				len(literals) == 0 && len(named) == 0 {
				return true
			}
		}
	}
	if len(expanded.alternatives) == 0 {
		return true
	}
	return false
}

type s7ARCallableDemandView struct {
	all     bool
	offsets map[int]bool
}

func (view s7ARCallableDemandView) any() bool {
	return view.all || len(view.offsets) != 0
}

func s7ARMergeCallableDemandView(
	target s7ARCallableDemandView,
	source s7ARCallableDemandView,
) (s7ARCallableDemandView, bool) {
	if target.all {
		return target, false
	}
	if source.all {
		return s7ARCallableDemandView{all: true}, true
	}
	if target.offsets == nil {
		target.offsets = map[int]bool{}
	}
	changed := false
	for offset := range source.offsets {
		if !target.offsets[offset] {
			target.offsets[offset] = true
			changed = true
		}
	}
	return target, changed
}

func s7ARCallableVariadicParameterDemand(
	model *s6SourceTypeModel,
	owner *ast.FuncDecl,
	body *ast.BlockStmt,
	parameter *ast.Ident,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	resolving map[*ast.BlockStmt]bool,
	cache map[*ast.BlockStmt]s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	if body == nil || parameter == nil {
		return s7ARCallableInvocationDemand{}
	}
	if cached, ok := cache[body]; ok {
		return cached
	}
	if resolving[body] {
		return s7ARCallableInvocationDemand{all: true}
	}
	resolving[body] = true
	defer delete(resolving, body)
	parameterObject := model.definitions[parameter]
	if parameterObject == nil {
		return s7ARCallableInvocationDemand{all: true}
	}
	demand := s7ARCallableInvocationDemand{}
	analysis := &s7ARCallableDemandAnalysis{
		model:                model,
		owner:                owner,
		aliases:              aliases,
		origins:              origins,
		receiverObjects:      receiverObjects,
		namedFunctions:       namedFunctions,
		resolving:            resolving,
		cache:                cache,
		demand:               &demand,
		invocation:           true,
		mutationResolving:    map[types.Object]bool{},
		mutationCache:        map[types.Object]s7ARCallableInvocationDemand{},
		asyncEscapeResolving: map[types.Object]bool{},
		asyncEscapeCache:     map[types.Object]s7ARCallableInvocationDemand{},
	}
	initial := s7ARCallableDemandState{
		sequenceViews: map[types.Object]s7ARCallableDemandView{
			parameterObject: {offsets: map[int]bool{0: true}},
		},
		elementDemands:   map[types.Object]s7ARCallableInvocationDemand{},
		sequenceOrigins:  map[types.Object]s7ARCallableSequenceOrigins{},
		sequenceReliable: map[types.Object]bool{},
		scalarOrigins:    map[types.Object][]s7ARCallableExpressionOrigin{},
		scalarReliable:   map[types.Object]bool{},
		scalarSlots:      map[types.Object]bool{},
		sequenceBackings: map[types.Object]s7ARCallableDemandBacking{
			parameterObject: {
				identity: parameter.Pos(),
				exact:    true,
			},
		},
		backingOverrides:   map[token.Pos]map[int]s7ARCallableInvocationDemand{},
		backingOverrideSet: map[token.Pos]map[int]bool{},
		backingOrigins:     map[token.Pos]map[int]s7ARCallableOriginOverride{},
		backingUncertain:   map[token.Pos]s7ARCallableInvocationDemand{},
		escapedBackings:    map[token.Pos]s7ARCallableInvocationDemand{},
	}
	if sequence, ok := origins.sequences[parameterObject]; ok &&
		!sequence.incomplete {
		initial.sequenceOrigins[parameterObject] =
			s7ARCloneCallableSequenceOrigins(sequence)
		initial.sequenceReliable[parameterObject] = true
	}
	flow := s7ARWalkCallableDemandBlock(
		analysis, body, []s7ARCallableDemandState{initial},
	)
	s7ARCompleteCallableDemandFlow(analysis, flow)
	cache[body] = demand
	return demand
}

const s7ARCallableDemandStateLimit = 64

type s7ARCallableDemandState struct {
	sequenceViews      map[types.Object]s7ARCallableDemandView
	elementDemands     map[types.Object]s7ARCallableInvocationDemand
	sequenceOrigins    map[types.Object]s7ARCallableSequenceOrigins
	sequenceReliable   map[types.Object]bool
	scalarOrigins      map[types.Object][]s7ARCallableExpressionOrigin
	scalarReliable     map[types.Object]bool
	scalarSlots        map[types.Object]bool
	sequenceBackings   map[types.Object]s7ARCallableDemandBacking
	backingOverrides   map[token.Pos]map[int]s7ARCallableInvocationDemand
	backingOverrideSet map[token.Pos]map[int]bool
	backingOrigins     map[token.Pos]map[int]s7ARCallableOriginOverride
	backingUncertain   map[token.Pos]s7ARCallableInvocationDemand
	escapedBackings    map[token.Pos]s7ARCallableInvocationDemand
	scheduledCalls     []s7ARCallableScheduledCall
	operandSnapshot    bool
	incomplete         bool
}

type s7ARCallableOriginOverride struct {
	origins  []s7ARCallableExpressionOrigin
	reliable bool
}

type s7ARCallableDemandBacking struct {
	identity token.Pos
	offset   int
	exact    bool
	derived  bool
}

type s7ARCallableScheduledCall struct {
	call         *ast.CallExpr
	declaration  *ast.FuncDecl
	statement    *s7ARCallableDemandState
	backings     []token.Pos
	captures     []types.Object
	literals     []*ast.FuncLit
	asynchronous bool
}

type s7ARCallableDemandFlow struct {
	next      []s7ARCallableDemandState
	returns   []s7ARCallableDemandState
	breaks    []s7ARCallableDemandState
	continues []s7ARCallableDemandState
}

type s7ARCallableDemandOperandSnapshot struct {
	state s7ARCallableDemandState
}

type s7ARCallableDemandEvaluatedCall struct {
	state     s7ARCallableDemandState
	function  s7ARCallableDemandOperandSnapshot
	arguments []s7ARCallableDemandOperandSnapshot
}

type s7ARCallableDemandAnalysis struct {
	model                *s6SourceTypeModel
	owner                *ast.FuncDecl
	aliases              map[types.Object]map[*ast.FuncLit]bool
	origins              *s7ARCallableOriginState
	receiverObjects      map[types.Object]bool
	namedFunctions       map[*types.Func]*ast.FuncDecl
	resolving            map[*ast.BlockStmt]bool
	cache                map[*ast.BlockStmt]s7ARCallableInvocationDemand
	demand               *s7ARCallableInvocationDemand
	observed             map[*ast.CallExpr][]s7ARCallableDemandState
	invocation           bool
	mutationDemand       *s7ARCallableInvocationDemand
	mutationResolving    map[types.Object]bool
	mutationCache        map[types.Object]s7ARCallableInvocationDemand
	asyncEscapeDemand    *s7ARCallableInvocationDemand
	asyncEscapeResolving map[types.Object]bool
	asyncEscapeCache     map[types.Object]s7ARCallableInvocationDemand
	scheduledResolving   map[*ast.FuncLit]bool
	evaluatedResolving   map[*ast.FuncDecl]bool
	overflow             bool
	incomplete           bool
}

func s7ARCloneCallableDemandView(
	source s7ARCallableDemandView,
) s7ARCallableDemandView {
	result := s7ARCallableDemandView{all: source.all}
	if source.all {
		return result
	}
	result.offsets = map[int]bool{}
	for offset := range source.offsets {
		result.offsets[offset] = true
	}
	return result
}

func s7ARCallableDemandViewEqual(
	left s7ARCallableDemandView,
	right s7ARCallableDemandView,
) bool {
	if left.all != right.all || len(left.offsets) != len(right.offsets) {
		return false
	}
	for offset := range left.offsets {
		if !right.offsets[offset] {
			return false
		}
	}
	return true
}

func s7ARCloneCallableDemandState(
	source s7ARCallableDemandState,
) s7ARCallableDemandState {
	result := s7ARCallableDemandState{
		sequenceViews:      map[types.Object]s7ARCallableDemandView{},
		elementDemands:     map[types.Object]s7ARCallableInvocationDemand{},
		sequenceOrigins:    map[types.Object]s7ARCallableSequenceOrigins{},
		sequenceReliable:   map[types.Object]bool{},
		scalarOrigins:      map[types.Object][]s7ARCallableExpressionOrigin{},
		scalarReliable:     map[types.Object]bool{},
		scalarSlots:        map[types.Object]bool{},
		sequenceBackings:   map[types.Object]s7ARCallableDemandBacking{},
		backingOverrides:   map[token.Pos]map[int]s7ARCallableInvocationDemand{},
		backingOverrideSet: map[token.Pos]map[int]bool{},
		backingOrigins:     map[token.Pos]map[int]s7ARCallableOriginOverride{},
		backingUncertain:   map[token.Pos]s7ARCallableInvocationDemand{},
		escapedBackings:    map[token.Pos]s7ARCallableInvocationDemand{},
		operandSnapshot:    source.operandSnapshot,
		incomplete:         source.incomplete,
	}
	for object, view := range source.sequenceViews {
		if view.any() {
			result.sequenceViews[object] = s7ARCloneCallableDemandView(view)
		}
	}
	for object, demand := range source.elementDemands {
		if demand.any() {
			result.elementDemands[object] =
				s7ARCloneCallableInvocationDemand(demand)
		}
	}
	for object, sequence := range source.sequenceOrigins {
		result.sequenceOrigins[object] =
			s7ARCloneCallableSequenceOrigins(sequence)
	}
	for object, reliable := range source.sequenceReliable {
		if reliable {
			result.sequenceReliable[object] = true
		}
	}
	for object, origins := range source.scalarOrigins {
		result.scalarOrigins[object] =
			append([]s7ARCallableExpressionOrigin(nil), origins...)
	}
	for object, reliable := range source.scalarReliable {
		if reliable {
			result.scalarReliable[object] = true
		}
	}
	for object, relevant := range source.scalarSlots {
		if relevant {
			result.scalarSlots[object] = true
		}
	}
	for object, backing := range source.sequenceBackings {
		result.sequenceBackings[object] = backing
	}
	for identity, overrides := range source.backingOverrides {
		result.backingOverrides[identity] =
			map[int]s7ARCallableInvocationDemand{}
		for index, demand := range overrides {
			result.backingOverrides[identity][index] =
				s7ARCloneCallableInvocationDemand(demand)
		}
	}
	for identity, overrides := range source.backingOverrideSet {
		result.backingOverrideSet[identity] = map[int]bool{}
		for index, set := range overrides {
			if set {
				result.backingOverrideSet[identity][index] = true
			}
		}
		for identity, overrides := range source.backingOrigins {
			result.backingOrigins[identity] =
				map[int]s7ARCallableOriginOverride{}
			for index, override := range overrides {
				result.backingOrigins[identity][index] =
					s7ARCallableOriginOverride{
						origins: append(
							[]s7ARCallableExpressionOrigin(nil),
							override.origins...,
						),
						reliable: override.reliable,
					}
			}
		}
	}
	for identity, demand := range source.backingUncertain {
		result.backingUncertain[identity] =
			s7ARCloneCallableInvocationDemand(demand)
	}
	for identity, demand := range source.escapedBackings {
		result.escapedBackings[identity] =
			s7ARCloneCallableInvocationDemand(demand)
	}
	for _, scheduled := range source.scheduledCalls {
		snapshot := s7ARCloneCallableDemandState(*scheduled.statement)
		snapshot.scheduledCalls = nil
		result.scheduledCalls = append(
			result.scheduledCalls,
			s7ARCallableScheduledCall{
				call:         scheduled.call,
				declaration:  scheduled.declaration,
				statement:    &snapshot,
				backings:     append([]token.Pos(nil), scheduled.backings...),
				captures:     append([]types.Object(nil), scheduled.captures...),
				literals:     append([]*ast.FuncLit(nil), scheduled.literals...),
				asynchronous: scheduled.asynchronous,
			},
		)
	}
	return result
}

func s7ARCloneCallableDemandStateWithoutScheduledCalls(
	source s7ARCallableDemandState,
) s7ARCallableDemandState {
	source.scheduledCalls = nil
	return s7ARCloneCallableDemandState(source)
}

func s7ARCallableDemandStateEqual(
	left s7ARCallableDemandState,
	right s7ARCallableDemandState,
) bool {
	if len(left.sequenceViews) != len(right.sequenceViews) ||
		len(left.elementDemands) != len(right.elementDemands) ||
		len(left.sequenceOrigins) != len(right.sequenceOrigins) ||
		len(left.sequenceReliable) != len(right.sequenceReliable) ||
		len(left.scalarOrigins) != len(right.scalarOrigins) ||
		len(left.scalarReliable) != len(right.scalarReliable) ||
		len(left.scalarSlots) != len(right.scalarSlots) ||
		len(left.sequenceBackings) != len(right.sequenceBackings) ||
		len(left.backingOverrides) != len(right.backingOverrides) ||
		len(left.backingOverrideSet) != len(right.backingOverrideSet) ||
		len(left.backingOrigins) != len(right.backingOrigins) ||
		len(left.backingUncertain) != len(right.backingUncertain) ||
		len(left.escapedBackings) != len(right.escapedBackings) ||
		len(left.scheduledCalls) != len(right.scheduledCalls) ||
		left.operandSnapshot != right.operandSnapshot ||
		left.incomplete != right.incomplete {
		return false
	}
	for object, view := range left.sequenceViews {
		if !s7ARCallableDemandViewEqual(view, right.sequenceViews[object]) {
			return false
		}
	}
	for object, demand := range left.elementDemands {
		if !s7ARCallableInvocationDemandEqual(
			demand, right.elementDemands[object],
		) {
			return false
		}
	}
	for object, sequence := range left.sequenceOrigins {
		if !s7ARCallableSequenceOriginsEqual(
			sequence, right.sequenceOrigins[object],
		) {
			return false
		}
	}
	for object, reliable := range left.sequenceReliable {
		if reliable != right.sequenceReliable[object] {
			return false
		}
	}
	for object, origins := range left.scalarOrigins {
		other := right.scalarOrigins[object]
		if len(origins) != len(other) {
			return false
		}
		for index := range origins {
			if origins[index] != other[index] {
				return false
			}
		}
	}
	for object, reliable := range left.scalarReliable {
		if reliable != right.scalarReliable[object] {
			return false
		}
	}
	for object, relevant := range left.scalarSlots {
		if relevant != right.scalarSlots[object] {
			return false
		}
	}
	for object, backing := range left.sequenceBackings {
		if backing != right.sequenceBackings[object] {
			return false
		}
	}
	for identity, overrides := range left.backingOverrides {
		other := right.backingOverrides[identity]
		if len(overrides) != len(other) {
			return false
		}
		for index, demand := range overrides {
			if !s7ARCallableInvocationDemandEqual(demand, other[index]) {
				return false
			}
		}
	}
	for identity, overrides := range left.backingOverrideSet {
		other := right.backingOverrideSet[identity]
		if len(overrides) != len(other) {
			return false
		}
		for index, set := range overrides {
			if set != other[index] {
				return false
			}
		}
		for identity, overrides := range left.backingOrigins {
			other := right.backingOrigins[identity]
			if len(overrides) != len(other) {
				return false
			}
			for index, override := range overrides {
				candidate, ok := other[index]
				if !ok || override.reliable != candidate.reliable ||
					len(override.origins) != len(candidate.origins) {
					return false
				}
				for origin := range override.origins {
					if override.origins[origin] != candidate.origins[origin] {
						return false
					}
				}
			}
		}
	}
	for identity, demand := range left.backingUncertain {
		if !s7ARCallableInvocationDemandEqual(
			demand, right.backingUncertain[identity],
		) {
			return false
		}
	}
	for identity, demand := range left.escapedBackings {
		if !s7ARCallableInvocationDemandEqual(
			demand, right.escapedBackings[identity],
		) {
			return false
		}
	}
	for index, scheduled := range left.scheduledCalls {
		other := right.scheduledCalls[index]
		if scheduled.call != other.call ||
			scheduled.declaration != other.declaration ||
			scheduled.asynchronous != other.asynchronous ||
			len(scheduled.backings) != len(other.backings) ||
			len(scheduled.captures) != len(other.captures) ||
			len(scheduled.literals) != len(other.literals) ||
			scheduled.statement == nil || other.statement == nil ||
			!s7ARCallableDemandStateEqual(
				*scheduled.statement, *other.statement,
			) {
			return false
		}
		for backing := range scheduled.backings {
			if scheduled.backings[backing] != other.backings[backing] {
				return false
			}
		}
		for capture := range scheduled.captures {
			if scheduled.captures[capture] != other.captures[capture] {
				return false
			}
		}
		for literal := range scheduled.literals {
			if scheduled.literals[literal] != other.literals[literal] {
				return false
			}
		}
	}
	return true
}

func s7ARCallableSequenceOriginsEqual(
	left s7ARCallableSequenceOrigins,
	right s7ARCallableSequenceOrigins,
) bool {
	if left.incomplete != right.incomplete ||
		len(left.alternatives) != len(right.alternatives) ||
		len(left.uncertainIndices) != len(right.uncertainIndices) {
		return false
	}
	for index := range left.alternatives {
		if len(left.alternatives[index]) != len(right.alternatives[index]) {
			return false
		}
		for element := range left.alternatives[index] {
			if left.alternatives[index][element] !=
				right.alternatives[index][element] {
				return false
			}
		}
	}
	for index := range left.uncertainIndices {
		if !right.uncertainIndices[index] {
			return false
		}
	}
	return true
}

func s7ARMergeCallableDemandStates(
	analysis *s7ARCallableDemandAnalysis,
	groups ...[]s7ARCallableDemandState,
) []s7ARCallableDemandState {
	var result []s7ARCallableDemandState
	for _, group := range groups {
		for _, candidate := range group {
			duplicate := false
			for _, existing := range result {
				if s7ARCallableDemandStateEqual(existing, candidate) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			result = append(result, candidate)
			if len(result) > s7ARCallableDemandStateLimit {
				analysis.overflow = true
				s7ARCallableAnalysisFailClosed(analysis)
				return result[:s7ARCallableDemandStateLimit]
			}
		}
	}
	return result
}

func s7ARCallableAnalysisFailClosed(
	analysis *s7ARCallableDemandAnalysis,
) {
	if analysis == nil {
		return
	}
	analysis.incomplete = true
	if analysis.demand != nil {
		analysis.demand.addAll()
	}
	if analysis.mutationDemand != nil {
		analysis.mutationDemand.addAll()
	}
}

func s7ARCallableDemandsIntersect(
	left s7ARCallableInvocationDemand,
	right s7ARCallableInvocationDemand,
) bool {
	if !left.any() || !right.any() {
		return false
	}
	if left.all || right.all {
		return true
	}
	for index := range left.indices {
		if right.indices[index] {
			return true
		}
	}
	return false
}

func s7ARMarkCallableDemandBackingEscaped(
	state *s7ARCallableDemandState,
	backing s7ARCallableDemandBacking,
	demand s7ARCallableInvocationDemand,
) {
	if state == nil || !demand.any() {
		return
	}
	if backing.identity == token.NoPos || !backing.exact {
		state.incomplete = true
		return
	}
	mapped := s7ARCallableInvocationDemand{}
	if demand.all {
		mapped.addAll()
	} else {
		for index := range demand.indices {
			mapped.addIndex(backing.offset + index)
		}
	}
	existing := state.escapedBackings[backing.identity]
	existing.merge(mapped)
	state.escapedBackings[backing.identity] = existing
}

func s7ARMarkCallableDemandEscapedMutation(
	state *s7ARCallableDemandState,
	identity token.Pos,
	demand s7ARCallableInvocationDemand,
) {
	if state == nil || identity == token.NoPos || !demand.any() {
		return
	}
	if s7ARCallableDemandsIntersect(
		state.escapedBackings[identity], demand,
	) {
		state.incomplete = true
	}
}

func s7ARMarkCallableDemandBackingUncertain(
	state *s7ARCallableDemandState,
	backing s7ARCallableDemandBacking,
	demand s7ARCallableInvocationDemand,
) {
	if state == nil || backing.identity == token.NoPos || !demand.any() {
		return
	}
	mapped := s7ARCallableInvocationDemand{}
	if demand.all || !backing.exact {
		mapped.addAll()
	} else {
		for index := range demand.indices {
			mapped.addIndex(backing.offset + index)
		}
	}
	s7ARMarkCallableDemandEscapedMutation(
		state, backing.identity, mapped,
	)
	existing := state.backingUncertain[backing.identity]
	existing.merge(mapped)
	state.backingUncertain[backing.identity] = existing
	for object, candidate := range state.sequenceBackings {
		if candidate.identity != backing.identity ||
			!state.sequenceReliable[object] {
			continue
		}
		sequence := s7ARCloneCallableSequenceOrigins(
			state.sequenceOrigins[object],
		)
		if mapped.all {
			sequence.incomplete = true
		} else {
			for absolute := range mapped.indices {
				s7ARMarkCallableSequenceIndexUncertain(
					&sequence, absolute-candidate.offset,
				)
			}
		}
		state.sequenceOrigins[object] = sequence
	}
}

func s7ARSetCallableDemandBackingOverride(
	state *s7ARCallableDemandState,
	backing s7ARCallableDemandBacking,
	index int,
	demand s7ARCallableInvocationDemand,
	origins []s7ARCallableExpressionOrigin,
	reliable bool,
) {
	if state == nil || backing.identity == token.NoPos ||
		!backing.exact || index < 0 {
		if state != nil {
			state.incomplete = true
		}
		return
	}
	absolute := backing.offset + index
	s7ARMarkCallableDemandEscapedMutation(
		state, backing.identity,
		s7ARCallableInvocationDemand{
			indices: map[int]bool{absolute: true},
		},
	)
	if state.backingOverrides[backing.identity] == nil {
		state.backingOverrides[backing.identity] =
			map[int]s7ARCallableInvocationDemand{}
	}
	if state.backingOverrideSet[backing.identity] == nil {
		state.backingOverrideSet[backing.identity] = map[int]bool{}
	}
	if state.backingOrigins[backing.identity] == nil {
		state.backingOrigins[backing.identity] =
			map[int]s7ARCallableOriginOverride{}
	}
	state.backingOverrides[backing.identity][absolute] =
		s7ARCloneCallableInvocationDemand(demand)
	state.backingOverrideSet[backing.identity][absolute] = true
	state.backingOrigins[backing.identity][absolute] =
		s7ARCallableOriginOverride{
			origins: append(
				[]s7ARCallableExpressionOrigin(nil), origins...,
			),
			reliable: reliable,
		}
	uncertain := state.backingUncertain[backing.identity]
	if !uncertain.all {
		delete(uncertain.indices, absolute)
		if uncertain.any() {
			state.backingUncertain[backing.identity] = uncertain
		} else {
			delete(state.backingUncertain, backing.identity)
		}
	}
	s7ARApplyCallableDemandOriginOverride(
		*state, backing.identity, absolute, origins, reliable,
	)
}

func s7ARCallableScheduledCaptures(
	model *s6SourceTypeModel,
	literal *ast.FuncLit,
) []types.Object {
	if model == nil || literal == nil || literal.Body == nil {
		return nil
	}
	local := map[types.Object]bool{}
	ast.Inspect(literal, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if object := model.definitions[identifier]; object != nil {
			local[object] = true
		}
		return true
	})
	captured := map[types.Object]bool{}
	ast.Inspect(literal.Body, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		object := model.uses[identifier]
		if object == nil || local[object] || object.Type() == nil {
			return true
		}
		if s7ARCallableSequenceObject(object) ||
			s7ARFunctionValuedType(object.Type()) {
			captured[object] = true
		}
		return true
	})
	result := make([]types.Object, 0, len(captured))
	for object := range captured {
		result = append(result, object)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Pos() < result[right].Pos()
	})
	return result
}

func s7ARCallableScheduledCaptureClosure(
	analysis *s7ARCallableDemandAnalysis,
	literals []*ast.FuncLit,
) []types.Object {
	if analysis == nil {
		return nil
	}
	captured := map[types.Object]bool{}
	visited := map[*ast.FuncLit]bool{}
	queue := append([]*ast.FuncLit(nil), literals...)
	for len(queue) != 0 {
		literal := queue[0]
		queue = queue[1:]
		if literal == nil || visited[literal] {
			continue
		}
		visited[literal] = true
		for _, object := range s7ARCallableScheduledCaptures(
			analysis.model, literal,
		) {
			captured[object] = true
			if object == nil || object.Type() == nil ||
				!s7ARFunctionValuedType(object.Type()) {
				continue
			}
			for target := range analysis.aliases[object] {
				if !visited[target] {
					queue = append(queue, target)
				}
			}
		}
	}
	result := make([]types.Object, 0, len(captured))
	for object := range captured {
		result = append(result, object)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Pos() < result[right].Pos()
	})
	return result
}

func s7ARCopyCallableDemandObjectState(
	target *s7ARCallableDemandState,
	source s7ARCallableDemandState,
	object types.Object,
) {
	if target == nil || object == nil {
		return
	}
	delete(target.sequenceViews, object)
	delete(target.elementDemands, object)
	delete(target.sequenceOrigins, object)
	delete(target.sequenceReliable, object)
	delete(target.scalarOrigins, object)
	delete(target.scalarReliable, object)
	delete(target.scalarSlots, object)
	delete(target.sequenceBackings, object)
	if view := source.sequenceViews[object]; view.any() {
		target.sequenceViews[object] = s7ARCloneCallableDemandView(view)
	}
	if demand := source.elementDemands[object]; demand.any() {
		target.elementDemands[object] =
			s7ARCloneCallableInvocationDemand(demand)
	}
	if source.sequenceReliable[object] {
		target.sequenceOrigins[object] =
			s7ARCloneCallableSequenceOrigins(source.sequenceOrigins[object])
		target.sequenceReliable[object] = true
	}
	if origins, ok := source.scalarOrigins[object]; ok {
		target.scalarOrigins[object] =
			append([]s7ARCallableExpressionOrigin(nil), origins...)
	}
	if source.scalarReliable[object] {
		target.scalarReliable[object] = true
	}
	if source.scalarSlots[object] {
		target.scalarSlots[object] = true
	}
	if backing, ok := source.sequenceBackings[object]; ok {
		target.sequenceBackings[object] = backing
	}
}

func s7ARCallableScheduledBackingIdentities(
	scheduled s7ARCallableScheduledCall,
	states ...s7ARCallableDemandState,
) map[token.Pos]bool {
	result := map[token.Pos]bool{}
	for _, identity := range scheduled.backings {
		if identity != token.NoPos {
			result[identity] = true
		}
	}
	for _, state := range states {
		for _, object := range scheduled.captures {
			if backing := state.sequenceBackings[object]; backing.identity != token.NoPos {
				result[backing.identity] = true
			}
		}
	}
	return result
}

func s7ARTransferScheduledCallableEffects(
	base s7ARCallableDemandState,
	executed s7ARCallableDemandState,
	scheduled s7ARCallableScheduledCall,
) s7ARCallableDemandState {
	result := s7ARCloneCallableDemandState(base)
	result.incomplete = result.incomplete || executed.incomplete
	for _, object := range scheduled.captures {
		s7ARCopyCallableDemandObjectState(&result, executed, object)
	}
	for identity := range s7ARCallableScheduledBackingIdentities(
		scheduled, base, executed,
	) {
		delete(result.backingOverrides, identity)
		delete(result.backingOverrideSet, identity)
		delete(result.backingOrigins, identity)
		delete(result.backingUncertain, identity)
		if overrides := executed.backingOverrides[identity]; len(overrides) != 0 {
			result.backingOverrides[identity] =
				map[int]s7ARCallableInvocationDemand{}
			for index, demand := range overrides {
				result.backingOverrides[identity][index] =
					s7ARCloneCallableInvocationDemand(demand)
			}
		}
		if overrides := executed.backingOverrideSet[identity]; len(overrides) != 0 {
			result.backingOverrideSet[identity] = map[int]bool{}
			for index, set := range overrides {
				if set {
					result.backingOverrideSet[identity][index] = true
				}
			}
		}
		if overrides := executed.backingOrigins[identity]; len(overrides) != 0 {
			result.backingOrigins[identity] =
				map[int]s7ARCallableOriginOverride{}
			for index, override := range overrides {
				result.backingOrigins[identity][index] =
					s7ARCallableOriginOverride{
						origins: append(
							[]s7ARCallableExpressionOrigin(nil),
							override.origins...,
						),
						reliable: override.reliable,
					}
			}
		}
		if demand := executed.backingUncertain[identity]; demand.any() {
			result.backingUncertain[identity] =
				s7ARCloneCallableInvocationDemand(demand)
		}
	}
	return result
}

func s7ARCallableScheduledExecutionState(
	scheduled s7ARCallableScheduledCall,
	current s7ARCallableDemandState,
) s7ARCallableDemandState {
	if scheduled.statement == nil {
		return s7ARCallableDemandState{incomplete: true}
	}
	result := s7ARCloneCallableDemandState(*scheduled.statement)
	result.scheduledCalls = nil
	result.incomplete = result.incomplete || current.incomplete
	for _, object := range scheduled.captures {
		s7ARCopyCallableDemandObjectState(&result, current, object)
		if current.scalarSlots[object] && !current.scalarReliable[object] {
			result.incomplete = true
		}
	}
	for identity := range s7ARCallableScheduledBackingIdentities(
		scheduled, current,
	) {
		for index, demand := range current.backingOverrides[identity] {
			if result.backingOverrides[identity] == nil {
				result.backingOverrides[identity] =
					map[int]s7ARCallableInvocationDemand{}
			}
			if result.backingOverrideSet[identity] == nil {
				result.backingOverrideSet[identity] = map[int]bool{}
			}
			result.backingOverrides[identity][index] =
				s7ARCloneCallableInvocationDemand(demand)
			result.backingOverrideSet[identity][index] = true
			override, ok := current.backingOrigins[identity][index]
			if !ok {
				result.incomplete = true
				continue
			}
			if result.backingOrigins[identity] == nil {
				result.backingOrigins[identity] =
					map[int]s7ARCallableOriginOverride{}
			}
			result.backingOrigins[identity][index] =
				s7ARCallableOriginOverride{
					origins: append(
						[]s7ARCallableExpressionOrigin(nil),
						override.origins...,
					),
					reliable: override.reliable,
				}
			s7ARApplyCallableDemandOriginOverride(
				result, identity, index, override.origins, override.reliable,
			)
		}
		if uncertain := current.backingUncertain[identity]; uncertain.any() {
			s7ARMarkCallableDemandBackingUncertain(
				&result,
				s7ARCallableDemandBacking{identity: identity, exact: true},
				uncertain,
			)
		}
	}
	return result
}

func s7ARApplyScheduledCallableCallMutation(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	scheduled s7ARCallableScheduledCall,
) []s7ARCallableDemandState {
	if analysis == nil || len(states) == 0 {
		return states
	}
	if scheduled.declaration != nil {
		if scheduled.declaration.Body == nil {
			s7ARCallableAnalysisFailClosed(analysis)
			return states
		}
		if analysis.evaluatedResolving == nil {
			analysis.evaluatedResolving = map[*ast.FuncDecl]bool{}
		}
		if analysis.evaluatedResolving[scheduled.declaration] {
			s7ARCallableAnalysisFailClosed(analysis)
			result := make([]s7ARCallableDemandState, 0, len(states))
			for _, state := range states {
				updated := s7ARCloneCallableDemandState(state)
				updated.incomplete = true
				result = append(result, updated)
			}
			return result
		}
		analysis.evaluatedResolving[scheduled.declaration] = true
		defer delete(
			analysis.evaluatedResolving, scheduled.declaration,
		)
		owner := analysis.owner
		analysis.owner = scheduled.declaration
		var result []s7ARCallableDemandState
		for _, state := range states {
			flow := s7ARWalkCallableDemandBlock(
				analysis, scheduled.declaration.Body,
				[]s7ARCallableDemandState{
					s7ARCloneCallableDemandState(state),
				},
			)
			result = append(
				result,
				s7ARCompleteCallableDemandFlow(analysis, flow)...,
			)
		}
		analysis.owner = owner
		for index := range result {
			result[index].scheduledCalls = nil
		}
		return s7ARMergeCallableDemandStates(analysis, result)
	}
	if len(scheduled.literals) == 0 {
		return s7ARApplyCallableDemandCallMutation(
			analysis, states, scheduled.call,
		)
	}
	if analysis.scheduledResolving == nil {
		analysis.scheduledResolving = map[*ast.FuncLit]bool{}
	}
	var result []s7ARCallableDemandState
	for _, state := range states {
		for _, literal := range scheduled.literals {
			if literal == nil || literal.Body == nil ||
				analysis.scheduledResolving[literal] {
				updated := s7ARCloneCallableDemandState(state)
				updated.incomplete = true
				result = append(result, updated)
				s7ARCallableAnalysisFailClosed(analysis)
				continue
			}
			analysis.scheduledResolving[literal] = true
			flow := s7ARWalkCallableDemandBlock(
				analysis, literal.Body,
				[]s7ARCallableDemandState{
					s7ARCloneCallableDemandState(state),
				},
			)
			completed := s7ARCompleteCallableDemandFlow(analysis, flow)
			delete(analysis.scheduledResolving, literal)
			for _, candidate := range completed {
				candidate.scheduledCalls = nil
				result = append(result, candidate)
			}
		}
	}
	return s7ARMergeCallableDemandStates(analysis, result)
}

func s7ARObserveScheduledCallableInvocation(
	analysis *s7ARCallableDemandAnalysis,
	execution s7ARCallableDemandState,
	scheduled s7ARCallableScheduledCall,
) {
	if analysis == nil {
		return
	}
	observation := execution
	if len(scheduled.literals) != 0 ||
		scheduled.declaration != nil {
		if scheduled.statement == nil {
			s7ARCallableAnalysisFailClosed(analysis)
			return
		}
		if analysis.observed != nil {
			analysis.observed[scheduled.call] =
				s7ARMergeCallableDemandStates(
					analysis, analysis.observed[scheduled.call],
					[]s7ARCallableDemandState{observation},
				)
		}
		if analysis.invocation {
			analysis.demand.merge(
				s7ARCallableElementDemandAtState(
					analysis.model, scheduled.call.Fun,
					observation,
				),
			)
		}
		return
	}
	s7ARObserveCallableDemandExpressions(
		analysis, []s7ARCallableDemandState{observation},
		scheduled.call,
	)
}

func s7ARApplyDeferredScheduledCallableMutations(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
) []s7ARCallableDemandState {
	if analysis == nil {
		return states
	}
	var result []s7ARCallableDemandState
	for _, state := range states {
		executionStates := []s7ARCallableDemandState{state}
		for index := len(state.scheduledCalls) - 1; index >= 0; index-- {
			scheduled := state.scheduledCalls[index]
			if scheduled.asynchronous {
				continue
			}
			var next []s7ARCallableDemandState
			for _, current := range executionStates {
				execution := s7ARCallableScheduledExecutionState(
					scheduled, current,
				)
				s7ARObserveScheduledCallableInvocation(
					analysis, execution, scheduled,
				)
				mutated := s7ARApplyScheduledCallableCallMutation(
					analysis, []s7ARCallableDemandState{execution},
					scheduled,
				)
				for _, candidate := range mutated {
					next = append(
						next,
						s7ARTransferScheduledCallableEffects(
							current, candidate, scheduled,
						),
					)
				}
			}
			executionStates = s7ARMergeCallableDemandStates(
				analysis, next,
			)
		}
		for _, completed := range executionStates {
			completed.scheduledCalls = nil
			result = append(result, completed)
		}
	}
	return s7ARMergeCallableDemandStates(analysis, result)
}

func s7ARCompleteCallableDemandFlow(
	analysis *s7ARCallableDemandAnalysis,
	flow s7ARCallableDemandFlow,
) []s7ARCallableDemandState {
	if analysis == nil {
		return append(
			append([]s7ARCallableDemandState(nil), flow.next...),
			flow.returns...,
		)
	}
	natural := s7ARApplyDeferredScheduledCallableMutations(
		analysis, flow.next,
	)
	returned := s7ARApplyDeferredScheduledCallableMutations(
		analysis, flow.returns,
	)
	return s7ARMergeCallableDemandStates(analysis, natural, returned)
}

func s7ARPublishScheduledCallableCalls(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	asynchronous bool,
) {
	if analysis == nil {
		return
	}
	for _, state := range states {
		if asynchronous {
			for _, scheduled := range state.scheduledCalls {
				if !scheduled.asynchronous {
					continue
				}
				execution := s7ARCallableScheduledExecutionState(
					scheduled, state,
				)
				s7ARObserveScheduledCallableInvocation(
					analysis, execution, scheduled,
				)
				if len(scheduled.literals) != 0 {
					s7ARApplyScheduledCallableCallMutation(
						analysis, []s7ARCallableDemandState{execution},
						scheduled,
					)
				}
			}
			continue
		}
		s7ARApplyDeferredScheduledCallableMutations(
			analysis, []s7ARCallableDemandState{state},
		)
	}
}

func s7ARScheduledCallableLiteralTargets(
	analysis *s7ARCallableDemandAnalysis,
	state s7ARCallableDemandState,
	expression ast.Expr,
) ([]*ast.FuncLit, bool, bool) {
	if analysis == nil || expression == nil {
		return nil, false, false
	}
	expression = s7ARUnwrapCallExpression(expression)
	if literal, ok := expression.(*ast.FuncLit); ok {
		return []*ast.FuncLit{literal}, false, false
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return nil, false, false
	}
	object := s7ARCallableDemandObject(analysis.model, identifier)
	if object == nil || object.Type() == nil ||
		!s7ARFunctionValuedType(object.Type()) {
		return nil, false, false
	}
	if _, ok := object.(*types.Var); !ok {
		return nil, false, false
	}
	if !state.scalarSlots[object] {
		return nil, false, false
	}
	if analysis.origins == nil {
		return nil, true, true
	}
	targets, incomplete := s7ARFunctionLiteralExpressionsAtDemandStates(
		analysis.model, analysis.owner, expression,
		[]s7ARCallableDemandState{state},
		analysis.aliases, analysis.origins, analysis.receiverObjects,
	)
	literals := make([]*ast.FuncLit, 0, len(targets))
	for literal := range targets {
		literals = append(literals, literal)
	}
	sort.Slice(literals, func(left, right int) bool {
		return literals[left].Pos() < literals[right].Pos()
	})
	return literals, true,
		incomplete || !state.scalarReliable[object] || len(literals) == 0
}

func s7ARObserveCallableAsyncEscape(
	analysis *s7ARCallableDemandAnalysis,
	state s7ARCallableDemandState,
	call *ast.CallExpr,
	literals []*ast.FuncLit,
) {
	if analysis == nil || analysis.asyncEscapeDemand == nil || call == nil {
		return
	}
	if len(literals) != 0 {
		for _, literal := range literals {
			if literal == nil || literal.Body == nil {
				analysis.asyncEscapeDemand.addAll()
				continue
			}
			nested := *analysis
			nested.demand = analysis.asyncEscapeDemand
			nested.invocation = true
			nested.observed = nil
			execution :=
				s7ARCloneCallableDemandStateWithoutScheduledCalls(state)
			flow := s7ARWalkCallableDemandBlock(
				&nested, literal.Body,
				[]s7ARCallableDemandState{execution},
			)
			s7ARCompleteCallableDemandFlow(&nested, flow)
			if nested.overflow || nested.incomplete {
				analysis.asyncEscapeDemand.addAll()
			}
		}
		return
	}
	if literal, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.FuncLit); ok {
		nested := *analysis
		nested.demand = analysis.asyncEscapeDemand
		nested.invocation = true
		nested.observed = nil
		execution :=
			s7ARCloneCallableDemandStateWithoutScheduledCalls(state)
		flow := s7ARWalkCallableDemandBlock(
			&nested, literal.Body,
			[]s7ARCallableDemandState{execution},
		)
		s7ARCompleteCallableDemandFlow(&nested, flow)
		if nested.overflow || nested.incomplete {
			analysis.asyncEscapeDemand.addAll()
		}
		return
	}
	analysis.asyncEscapeDemand.merge(
		s7ARCallableElementDemandAtState(
			analysis.model, call.Fun, state,
		),
	)
	if call.Ellipsis == token.NoPos || len(call.Args) == 0 {
		return
	}
	forwarded := s7ARCallableForwardedDemandAtState(
		analysis.model, analysis.owner, call, state,
		analysis.aliases, analysis.origins, analysis.receiverObjects,
		analysis.namedFunctions, analysis.resolving, analysis.cache,
	)
	analysis.asyncEscapeDemand.merge(
		s7ARMapCallableDemandThroughExpressionAtState(
			analysis.model, forwarded,
			call.Args[len(call.Args)-1], state,
		),
	)
}

func s7ARScheduleCallableCall(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	call *ast.CallExpr,
	asynchronous bool,
) []s7ARCallableDemandState {
	if analysis == nil || call == nil {
		return states
	}
	evaluated := s7AREvaluateCallableDemandCallOperands(
		analysis, states, call,
	)
	result := make([]s7ARCallableDemandState, 0, len(evaluated))
	for _, context := range evaluated {
		state := context.state
		var backings []token.Pos
		literalTargets, scalarAlias, literalIncomplete :=
			s7ARScheduledCallableLiteralTargets(
				analysis, context.function.state, call.Fun,
			)
		captures := s7ARCallableScheduledCaptureClosure(
			analysis, literalTargets,
		)
		stateRelevant := scalarAlias || len(captures) != 0
		for index, argument := range call.Args {
			if s7ARFunctionValuedExpression(
				analysis.model, argument,
			) {
				stateRelevant = true
			}
			if !s7ARCallableSequenceExpression(analysis.model, argument) {
				continue
			}
			stateRelevant = true
			backing := s7ARCallableBackingForDemandExpression(
				analysis.model, context.arguments[index].state,
				argument,
			)
			if backing.identity == token.NoPos || !backing.exact {
				continue
			}
			duplicate := false
			for _, identity := range backings {
				if identity == backing.identity {
					duplicate = true
					break
				}
			}
			if !duplicate {
				backings = append(backings, backing.identity)
			}
		}

		if !stateRelevant {
			result = append(result, state)
			continue
		}
		updated := s7ARCloneCallableDemandState(state)
		snapshot :=
			s7ARCloneCallableDemandStateWithoutScheduledCalls(state)
		snapshot.incomplete = snapshot.incomplete || literalIncomplete
		if scalarAlias &&
			!s7ARCopyCallableDemandExpressionSnapshot(
				analysis, &snapshot,
				context.function.state, call.Fun,
			) {
			snapshot.incomplete = true
		}
		if len(literalTargets) == 0 {
			observedObjects := map[types.Object]s7ARCallableDemandState{}
			for index, argument := range call.Args {
				source := context.arguments[index].state
				object := s7ARCallableDemandSnapshotObject(
					analysis.model, argument,
				)
				if object == nil {
					if s7ARCallableSequenceExpression(
						analysis.model, argument,
					) || s7ARFunctionValuedExpression(
						analysis.model, argument,
					) {
						snapshot.incomplete = true
					}
					continue
				}
				if prior, ok := observedObjects[object]; ok &&
					!s7ARCallableDemandObjectSnapshotEqual(
						prior, source, object,
					) {
					snapshot.incomplete = true
					continue
				}
				observedObjects[object] = source
				s7ARCopyCallableDemandObjectState(
					&snapshot, source, object,
				)
				if s7ARCallableSequenceExpression(
					analysis.model, argument,
				) {
					s7ARRefreshCallableDemandObjectBacking(
						&snapshot, object,
					)
				}
			}
		}
		for _, literal := range literalTargets {
			parameters, variadic := s7ARCallableParameters(
				literal.Type.Params,
			)
			if variadic || len(parameters) != len(call.Args) {
				snapshot.incomplete = true
			} else {
				for index := range parameters {
					if !s7ARBindCallableDemandOperand(
						analysis, &snapshot, parameters[index],
						call.Args[index], context.arguments[index],
					) {
						snapshot.incomplete = true
					}
				}
			}
		}
		if asynchronous {
			s7ARObserveCallableAsyncEscape(
				analysis, snapshot, call, literalTargets,
			)
			if len(literalTargets) == 0 {
				s7ARObserveCallableAsyncEscapeCallAtSnapshots(
					analysis, context, call,
				)
			}
		}
		if len(backings) == 0 {
			if len(captures) == 0 && len(literalTargets) == 0 {
				snapshot.incomplete = true
			}
		}
		updated.scheduledCalls = append(
			updated.scheduledCalls,
			s7ARCallableScheduledCall{
				call:         call,
				statement:    &snapshot,
				backings:     backings,
				captures:     captures,
				literals:     append([]*ast.FuncLit(nil), literalTargets...),
				asynchronous: asynchronous,
			},
		)
		result = append(result, updated)
	}
	result = s7ARMergeCallableDemandStates(analysis, result)
	if asynchronous {
		s7ARPublishScheduledCallableCalls(analysis, result, true)
		var alternatives []s7ARCallableDemandState
		for _, state := range result {
			alternatives = append(alternatives, state)
			if len(state.scheduledCalls) == 0 {
				continue
			}
			scheduled := state.scheduledCalls[len(state.scheduledCalls)-1]
			execution := s7ARCallableScheduledExecutionState(
				scheduled, state,
			)
			mutated := s7ARApplyScheduledCallableCallMutation(
				analysis, []s7ARCallableDemandState{execution},
				scheduled,
			)
			for _, candidate := range mutated {
				alternatives = append(
					alternatives,
					s7ARTransferScheduledCallableEffects(
						state, candidate, scheduled,
					),
				)
			}
		}
		result = s7ARMergeCallableDemandStates(
			analysis, alternatives,
		)
	}
	return result
}

func s7ARCallableDemandObject(
	model *s6SourceTypeModel,
	identifier *ast.Ident,
) types.Object {
	if identifier == nil || identifier.Name == "_" {
		return nil
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	return object
}

func s7ARWalkCallableDemandBlock(
	analysis *s7ARCallableDemandAnalysis,
	block *ast.BlockStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	flow := s7ARCallableDemandFlow{next: states}
	if block == nil {
		return flow
	}
	for _, statement := range block.List {
		if len(flow.next) == 0 || analysis.overflow {
			break
		}
		step := s7ARWalkCallableDemandStatement(
			analysis, statement, flow.next,
		)
		s7ARPublishScheduledCallableCalls(analysis, step.next, true)
		flow.next = step.next
		flow.returns = s7ARMergeCallableDemandStates(
			analysis, flow.returns, step.returns,
		)
		flow.breaks = s7ARMergeCallableDemandStates(
			analysis, flow.breaks, step.breaks,
		)
		flow.continues = s7ARMergeCallableDemandStates(
			analysis, flow.continues, step.continues,
		)
	}
	return flow
}

func s7ARWalkCallableDemandStatement(
	analysis *s7ARCallableDemandAnalysis,
	statement ast.Stmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	if statement == nil || len(states) == 0 {
		return s7ARCallableDemandFlow{next: states}
	}
	switch typed := statement.(type) {
	case *ast.BlockStmt:
		return s7ARWalkCallableDemandBlock(analysis, typed, states)
	case *ast.DeclStmt:
		declaration, _ := typed.Decl.(*ast.GenDecl)
		next := states
		if declaration != nil {
			for _, specification := range declaration.Specs {
				values, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				s7ARObserveCallableDemandExpressions(
					analysis, next, values.Values...,
				)
				next = s7ARApplyCallableDemandExpressionMutations(
					analysis, next, values.Values...,
				)
				s7ARPublishScheduledCallableCalls(analysis, next, true)
				next = s7ARApplyCallableDemandAssignment(
					analysis, next, values.Names, values.Values,
				)
			}
		}
		return s7ARCallableDemandFlow{next: next}
	case *ast.AssignStmt:
		s7ARObserveCallableDemandExpressions(
			analysis, states, typed.Rhs...,
		)
		afterCalls := s7ARApplyCallableDemandExpressionMutations(
			analysis, states, typed.Rhs...,
		)
		s7ARPublishScheduledCallableCalls(analysis, afterCalls, true)
		mutated := s7ARApplyCallableDemandIndexMutations(
			analysis, afterCalls, typed,
		)
		return s7ARCallableDemandFlow{next: s7ARApplyCallableDemandAssignment(
			analysis, mutated, s7ARAssignmentIdentifiers(typed.Lhs), typed.Rhs,
		)}
	case *ast.ExprStmt:
		s7ARObserveCallableDemandExpressions(analysis, states, typed.X)
		return s7ARCallableDemandFlow{
			next: s7ARApplyCallableDemandExpressionMutations(
				analysis, states, typed.X,
			),
		}
	case *ast.GoStmt:
		return s7ARCallableDemandFlow{next: s7ARScheduleCallableCall(
			analysis, states, typed.Call, true,
		)}
	case *ast.DeferStmt:
		return s7ARCallableDemandFlow{next: s7ARScheduleCallableCall(
			analysis, states, typed.Call, false,
		)}
	case *ast.ReturnStmt:
		s7ARObserveCallableDemandExpressions(analysis, states, typed.Results...)
		returns := s7ARApplyCallableDemandExpressionMutations(
			analysis, states, typed.Results...,
		)
		s7ARPublishScheduledCallableCalls(analysis, returns, true)
		for _, state := range returns {
			if state.incomplete {
				s7ARCallableAnalysisFailClosed(analysis)
				break
			}
		}
		return s7ARCallableDemandFlow{returns: returns}
	case *ast.BranchStmt:
		if typed.Label != nil {
			s7ARCallableAnalysisFailClosed(analysis)
			return s7ARCallableDemandFlow{}
		}
		switch typed.Tok {
		case token.BREAK:
			return s7ARCallableDemandFlow{breaks: states}
		case token.CONTINUE:
			return s7ARCallableDemandFlow{continues: states}
		case token.FALLTHROUGH, token.GOTO:
			s7ARCallableAnalysisFailClosed(analysis)
			return s7ARCallableDemandFlow{}
		default:
			return s7ARCallableDemandFlow{}
		}

	case *ast.IfStmt:
		return s7ARWalkCallableDemandIf(analysis, typed, states)
	case *ast.ForStmt:
		return s7ARWalkCallableDemandFor(analysis, typed, states)
	case *ast.RangeStmt:
		return s7ARWalkCallableDemandRange(analysis, typed, states)
	case *ast.SwitchStmt:
		return s7ARWalkCallableDemandSwitch(analysis, typed, states)
	case *ast.TypeSwitchStmt:
		return s7ARWalkCallableDemandTypeSwitch(analysis, typed, states)
	case *ast.SelectStmt:
		return s7ARWalkCallableDemandSelect(analysis, typed, states)
	case *ast.LabeledStmt:
		return s7ARWalkCallableDemandStatement(analysis, typed.Stmt, states)
	case *ast.IncDecStmt:
		s7ARObserveCallableDemandExpressions(analysis, states, typed.X)
		return s7ARCallableDemandFlow{next: states}
	case *ast.SendStmt:
		return s7ARCallableDemandFlow{
			next: s7AREvaluateCallableDemandExpressions(
				analysis, states, typed.Chan, typed.Value,
			),
		}
	case *ast.EmptyStmt:
		return s7ARCallableDemandFlow{next: states}
	default:
		s7ARCallableAnalysisFailClosed(analysis)
		return s7ARCallableDemandFlow{next: states}
	}
}

func s7ARApplyCallableDemandIndexMutations(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	assignment *ast.AssignStmt,
) []s7ARCallableDemandState {
	if assignment == nil {
		return states
	}
	result := make([]s7ARCallableDemandState, 0, len(states))
	for _, state := range states {
		updated := s7ARCloneCallableDemandState(state)
		replacements := make([]s7ARCallableInvocationDemand, len(assignment.Lhs))
		replacementOrigins := make(
			[][]s7ARCallableExpressionOrigin, len(assignment.Lhs),
		)
		replacementReliable := make([]bool, len(assignment.Lhs))
		for index := range assignment.Lhs {
			right, resolved := s7ARAssignmentRight(assignment.Rhs, index)
			if !resolved {
				continue
			}
			replacements[index] = s7ARCallableElementDemandForExpression(
				analysis.model, right, state.sequenceViews,
				state.elementDemands,
			)
			replacementOrigins[index], replacementReliable[index] =
				s7ARCallableScalarOriginsForDemandState(
					analysis, state, right,
				)
		}
		for index, left := range assignment.Lhs {
			indexed, ok := s7ARUnwrapCallExpression(left).(*ast.IndexExpr)
			if !ok {
				continue
			}
			_, resolved := s7ARAssignmentRight(assignment.Rhs, index)
			if !resolved {
				s7ARCallableAnalysisFailClosed(analysis)
				continue
			}
			object := s7ARCallableSequenceIdentifierObject(
				analysis.model, indexed.X,
			)
			backing := updated.sequenceBackings[object]
			element, exact := s7ARConstantIndex(
				analysis.model, indexed.Index,
			)
			if object == nil || backing.identity == token.NoPos ||
				!backing.exact || !exact || element < 0 {
				if s7ARCallableDemandViewForExpression(
					analysis.model, indexed.X, updated.sequenceViews,
				).any() {
					s7ARCallableAnalysisFailClosed(analysis)
				}
				continue
			}
			absolute := backing.offset + element
			s7ARMarkCallableDemandEscapedMutation(
				&updated, backing.identity,
				s7ARCallableInvocationDemand{
					indices: map[int]bool{absolute: true},
				},
			)
			if analysis.mutationDemand != nil {
				analysis.mutationDemand.merge(
					s7ARCallableElementDemandAtState(
						analysis.model, indexed, state,
					),
				)
			}
			if backing.derived {
				s7ARMarkCallableDemandBackingUncertain(
					&updated, backing,
					s7ARCallableInvocationDemand{
						indices: map[int]bool{element: true},
					},
				)
				continue
			}
			if updated.backingOverrides[backing.identity] == nil {
				updated.backingOverrides[backing.identity] =
					map[int]s7ARCallableInvocationDemand{}
			}
			if updated.backingOverrideSet[backing.identity] == nil {
				updated.backingOverrideSet[backing.identity] = map[int]bool{}
			}
			if updated.backingOrigins[backing.identity] == nil {
				updated.backingOrigins[backing.identity] =
					map[int]s7ARCallableOriginOverride{}
			}
			updated.backingOverrides[backing.identity][absolute] =
				s7ARCloneCallableInvocationDemand(replacements[index])
			updated.backingOverrideSet[backing.identity][absolute] = true
			updated.backingOrigins[backing.identity][absolute] =
				s7ARCallableOriginOverride{
					origins: append(
						[]s7ARCallableExpressionOrigin(nil),
						replacementOrigins[index]...,
					),
					reliable: replacementReliable[index],
				}
			uncertain := updated.backingUncertain[backing.identity]
			if !uncertain.all {
				delete(uncertain.indices, absolute)
				if uncertain.any() {
					updated.backingUncertain[backing.identity] = uncertain
				} else {
					delete(updated.backingUncertain, backing.identity)
				}
			}
			s7ARApplyCallableDemandOriginOverride(
				updated, backing.identity, absolute,
				replacementOrigins[index], replacementReliable[index],
			)
		}
		result = append(result, updated)
	}
	return s7ARMergeCallableDemandStates(analysis, result)
}

func s7ARApplyCallableDemandOriginOverride(
	state s7ARCallableDemandState,
	identity token.Pos,
	index int,
	origins []s7ARCallableExpressionOrigin,
	reliable bool,
) {
	for object, backing := range state.sequenceBackings {
		if backing.identity != identity ||
			!state.sequenceReliable[object] {
			continue
		}
		sequence := s7ARCloneCallableSequenceOrigins(
			state.sequenceOrigins[object],
		)
		localIndex := index - backing.offset
		if !reliable || localIndex < 0 {
			s7ARMarkCallableSequenceIndexUncertain(&sequence, localIndex)
			state.sequenceOrigins[object] = sequence
			continue
		}
		var alternatives []s7ARCallableSequenceAlternative
		for _, alternative := range sequence.alternatives {
			if localIndex >= len(alternative) {
				sequence.incomplete = true
				continue
			}
			for _, origin := range origins {
				replaced := append(
					s7ARCallableSequenceAlternative(nil), alternative...,
				)
				replaced[localIndex] = origin
				alternatives = append(alternatives, replaced)
			}
		}
		if len(origins) == 0 {
			sequence.incomplete = true
		} else {
			sequence.alternatives = alternatives
			delete(sequence.uncertainIndices, localIndex)
		}
		state.sequenceOrigins[object] = sequence
	}
}

func s7ARApplyCallableDemandAssignment(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	left []*ast.Ident,
	right []ast.Expr,
) []s7ARCallableDemandState {
	result := make([]s7ARCallableDemandState, 0, len(states))
	for _, state := range states {
		views := make([]s7ARCallableDemandView, len(left))
		elements := make([]s7ARCallableInvocationDemand, len(left))
		sequences := make([]s7ARCallableSequenceOrigins, len(left))
		sequenceReliable := make([]bool, len(left))
		scalars := make([][]s7ARCallableExpressionOrigin, len(left))
		scalarReliable := make([]bool, len(left))
		scalarSlots := make([]bool, len(left))
		backings := make([]s7ARCallableDemandBacking, len(left))
		resolved := make([]bool, len(left))
		for index := range left {
			expression, ok := s7ARAssignmentRight(right, index)
			if !ok {
				continue
			}
			resolved[index] = true
			views[index] = s7ARCallableDemandViewForExpression(
				analysis.model, expression, state.sequenceViews,
			)
			elements[index] = s7ARCallableElementDemandForExpression(
				analysis.model, expression, state.sequenceViews,
				state.elementDemands,
			)
			object := s7ARCallableDemandObject(analysis.model, left[index])
			if s7ARCallableSequenceObject(object) {
				sequences[index], sequenceReliable[index] =
					s7ARCallableSequenceOriginsForDemandState(
						analysis, state, expression,
					)
				backings[index] = s7ARCallableBackingForDemandExpression(
					analysis.model, state, expression,
				)
				if backings[index].identity == token.NoPos {
					backings[index] = s7ARCallableDemandBacking{
						identity: expression.Pos(),
						exact:    expression.Pos() != token.NoPos,
					}
				}
			}
			if object != nil && object.Type() != nil &&
				s7ARFunctionValuedType(object.Type()) {
				scalars[index], scalarReliable[index] =
					s7ARCallableScalarOriginsForDemandState(
						analysis, state, expression,
					)
				scalarSlots[index] =
					s7ARCallableScalarSlotForDemandState(
						analysis.model, state, expression,
					)
			}
		}
		updated := s7ARCloneCallableDemandState(state)
		for index, identifier := range left {
			object := s7ARCallableDemandObject(analysis.model, identifier)
			if object == nil {
				continue
			}
			priorElementDemand := updated.elementDemands[object]
			priorScalarSlot := updated.scalarSlots[object]
			delete(updated.sequenceViews, object)
			delete(updated.elementDemands, object)
			delete(updated.sequenceOrigins, object)
			delete(updated.sequenceReliable, object)
			delete(updated.scalarOrigins, object)
			delete(updated.scalarReliable, object)
			delete(updated.scalarSlots, object)
			delete(updated.sequenceBackings, object)
			if !resolved[index] {
				continue
			}
			if views[index].any() {
				updated.sequenceViews[object] =
					s7ARCloneCallableDemandView(views[index])
			}
			if elements[index].any() {
				updated.elementDemands[object] =
					s7ARCloneCallableInvocationDemand(elements[index])
			}
			if sequenceReliable[index] {
				updated.sequenceOrigins[object] =
					s7ARCloneCallableSequenceOrigins(sequences[index])
				updated.sequenceReliable[object] = true
			}
			if backings[index].identity != token.NoPos {
				updated.sequenceBackings[object] = backings[index]
			}
			if scalarReliable[index] {
				updated.scalarOrigins[object] = append(
					[]s7ARCallableExpressionOrigin(nil), scalars[index]...,
				)
				updated.scalarReliable[object] = true
			}
			if object.Type() != nil &&
				s7ARFunctionValuedType(object.Type()) &&
				(scalarSlots[index] || priorScalarSlot ||
					priorElementDemand.any()) {
				updated.scalarSlots[object] = true
			}
		}
		result = append(result, updated)
	}
	return s7ARMergeCallableDemandStates(analysis, result)
}

func s7ARCallableBackingForDemandExpression(
	model *s6SourceTypeModel,
	state s7ARCallableDemandState,
	expression ast.Expr,
) s7ARCallableDemandBacking {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		return state.sequenceBackings[s7ARCallableDemandObject(model, typed)]
	case *ast.SliceExpr:
		backing := s7ARCallableBackingForDemandExpression(
			model, state, typed.X,
		)
		if backing.identity == token.NoPos {
			return backing
		}
		if typed.Low != nil {
			offset, exact := s7ARConstantIndex(model, typed.Low)
			if !exact || !backing.exact {
				backing.exact = false
			} else {
				backing.offset += offset
			}
		}
		backing.derived = true
		return backing
	case *ast.CompositeLit:
		return s7ARCallableDemandBacking{
			identity: typed.Pos(),
			exact:    typed.Pos() != token.NoPos,
		}
	default:
		return s7ARCallableDemandBacking{}
	}
}

func s7ARCallableSequenceOriginsForDemandState(
	analysis *s7ARCallableDemandAnalysis,
	state s7ARCallableDemandState,
	expression ast.Expr,
) (s7ARCallableSequenceOrigins, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := s7ARCallableDemandObject(analysis.model, typed)
		if !state.sequenceReliable[object] {
			return s7ARCallableSequenceOrigins{}, false
		}
		return s7ARCloneCallableSequenceOrigins(
			state.sequenceOrigins[object],
		), true
	case *ast.CompositeLit:
		return s7ARCallableCompositeSequence(
			analysis.model, analysis.owner, typed,
		), true
	case *ast.SliceExpr:
		source, reliable := s7ARCallableSequenceOriginsForDemandState(
			analysis, state, typed.X,
		)
		if !reliable {
			return s7ARCallableSequenceOrigins{}, false
		}
		return s7ARSliceCallableSequence(
			analysis.model, source, typed.Low, typed.High,
		), true
	default:
		return s7ARCallableSequenceOrigins{}, false
	}
}

func s7ARCallableScalarOriginsForDemandState(
	analysis *s7ARCallableDemandAnalysis,
	state s7ARCallableDemandState,
	expression ast.Expr,
) ([]s7ARCallableExpressionOrigin, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	if identifier, ok := expression.(*ast.Ident); ok {
		object := s7ARCallableDemandObject(analysis.model, identifier)
		if state.scalarReliable[object] {
			return append(
				[]s7ARCallableExpressionOrigin(nil),
				state.scalarOrigins[object]...,
			), true
		}
	}
	if indexed, ok := expression.(*ast.IndexExpr); ok {
		object := s7ARCallableSequenceIdentifierObject(
			analysis.model, indexed.X,
		)
		if object != nil && state.sequenceReliable[object] {
			index, exact := s7ARConstantIndex(
				analysis.model, indexed.Index,
			)
			sequence := state.sequenceOrigins[object]
			if !exact || index < 0 || sequence.incomplete ||
				sequence.uncertainIndices[index] {
				return nil, false
			}
			var result []s7ARCallableExpressionOrigin
			for _, alternative := range sequence.alternatives {
				if index >= len(alternative) {
					return nil, false
				}
				result = append(result, alternative[index])
			}
			if len(result) != 0 {
				return result, true
			}
		}
	}
	if expression == nil {
		return nil, false
	}
	return []s7ARCallableExpressionOrigin{{
		function: analysis.owner, expression: expression,
	}}, true
}

func s7ARCallableScalarSlotForDemandState(
	model *s6SourceTypeModel,
	state s7ARCallableDemandState,
	expression ast.Expr,
) bool {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := s7ARCallableDemandObject(model, typed)
		return state.scalarSlots[object] ||
			state.elementDemands[object].any()
	case *ast.IndexExpr:
		return s7ARCallableElementDemandAtState(
			model, typed, state,
		).any()
	case *ast.FuncLit:
		return true
	default:
		return false
	}
}

func s7ARCallableSequenceIdentifierObject(
	model *s6SourceTypeModel,
	expression ast.Expr,
) types.Object {
	expression = s7ARUnwrapCallExpression(expression)
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return nil
	}
	return s7ARCallableDemandObject(model, identifier)
}

func s7ARObserveCallableDemandExpressions(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	expressions ...ast.Expr,
) {
	for _, expression := range expressions {
		if expression == nil {
			continue
		}
		ast.Inspect(expression, func(node ast.Node) bool {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			s7ARObserveCallableDemandCall(analysis, states, call)
			return true
		})
	}
}

func s7ARObserveCallableDemandCall(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	call *ast.CallExpr,
) {
	if analysis == nil || call == nil {
		return
	}
	if analysis.observed != nil {
		analysis.observed[call] = s7ARMergeCallableDemandStates(
			analysis, analysis.observed[call], states,
		)
	}
	for _, state := range states {
		direct := s7ARCallableElementDemandAtState(
			analysis.model, call.Fun, state,
		)
		if analysis.invocation {
			analysis.demand.merge(direct)
		}
		s7ARObserveCallableMutationCall(analysis, state, call)
		s7ARObserveCallableAsyncEscapeCall(analysis, state, call)
		if call.Ellipsis == token.NoPos || len(call.Args) == 0 {
			continue
		}
		view := s7ARCallableDemandViewForExpression(
			analysis.model, call.Args[len(call.Args)-1],
			state.sequenceViews,
		)
		if !view.any() || !analysis.invocation {
			continue
		}
		forwarded := s7ARCallableForwardedDemandAtState(
			analysis.model, analysis.owner, call, state,
			analysis.aliases, analysis.origins,
			analysis.receiverObjects, analysis.namedFunctions,
			analysis.resolving, analysis.cache,
		)
		analysis.demand.merge(
			s7ARMapCallableDemandThroughExpressionAtState(
				analysis.model, forwarded,
				call.Args[len(call.Args)-1], state,
			),
		)
	}
}

func s7AREvaluateCallableDemandExpressions(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	expressions ...ast.Expr,
) []s7ARCallableDemandState {
	result := states
	for _, expression := range expressions {
		result = s7AREvaluateCallableDemandExpression(
			analysis, result, expression,
		)
	}
	return result
}

func s7AREvaluateCallableDemandExpression(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	expression ast.Expr,
) []s7ARCallableDemandState {
	if expression == nil || len(states) == 0 {
		return states
	}
	switch typed := expression.(type) {
	case *ast.BasicLit, *ast.Ident, *ast.FuncLit:
		return states
	case *ast.ParenExpr:
		return s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
	case *ast.Ellipsis:
		return s7AREvaluateCallableDemandExpression(
			analysis, states, typed.Elt,
		)
	case *ast.SelectorExpr:
		return s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
	case *ast.IndexExpr:
		return s7AREvaluateCallableDemandExpressions(
			analysis, states, typed.X, typed.Index,
		)
	case *ast.IndexListExpr:
		result := s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
		return s7AREvaluateCallableDemandExpressions(
			analysis, result, typed.Indices...,
		)
	case *ast.SliceExpr:
		return s7AREvaluateCallableDemandExpressions(
			analysis, states,
			typed.X, typed.Low, typed.High, typed.Max,
		)
	case *ast.TypeAssertExpr:
		return s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
	case *ast.StarExpr:
		return s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
	case *ast.UnaryExpr:
		return s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
	case *ast.BinaryExpr:
		left := s7AREvaluateCallableDemandExpression(
			analysis, states, typed.X,
		)
		right := s7AREvaluateCallableDemandExpression(
			analysis, left, typed.Y,
		)
		if typed.Op == token.LAND || typed.Op == token.LOR {
			return s7ARMergeCallableDemandStates(
				analysis, left, right,
			)
		}
		return right
	case *ast.KeyValueExpr:
		return s7AREvaluateCallableDemandExpressions(
			analysis, states, typed.Key, typed.Value,
		)
	case *ast.CompositeLit:
		result := states
		for _, element := range typed.Elts {
			expression, ok := element.(ast.Expr)
			if !ok {
				s7ARCallableAnalysisFailClosed(analysis)
				continue
			}
			result = s7AREvaluateCallableDemandExpression(
				analysis, result, expression,
			)
		}
		return result
	case *ast.CallExpr:
		evaluated := s7AREvaluateCallableDemandCallOperands(
			analysis, states, typed,
		)
		var result []s7ARCallableDemandState
		for _, context := range evaluated {
			s7ARObserveEvaluatedCallableDemandCall(
				analysis, context, typed,
			)
			result = append(
				result,
				s7ARApplyEvaluatedCallableDemandCallMutation(
					analysis, context, typed,
				)...,
			)
		}
		result = s7ARMergeCallableDemandStates(analysis, result)
		s7ARPublishScheduledCallableCalls(analysis, result, true)
		return result
	case *ast.BadExpr:
		s7ARCallableAnalysisFailClosed(analysis)
		return states
	default:
		if _, ok := analysis.model.expressionTypes[expression]; ok {
			return states
		}
		s7ARCallableAnalysisFailClosed(analysis)
		return states
	}
}

func s7ARCallableDemandSnapshot(
	state s7ARCallableDemandState,
) s7ARCallableDemandOperandSnapshot {
	snapshot := state
	snapshot.scheduledCalls = nil
	return s7ARCallableDemandOperandSnapshot{state: snapshot}
}

func s7ARLimitCallableDemandEvaluatedCalls(
	analysis *s7ARCallableDemandAnalysis,
	contexts []s7ARCallableDemandEvaluatedCall,
) []s7ARCallableDemandEvaluatedCall {
	if len(contexts) <= s7ARCallableDemandStateLimit {
		return contexts
	}
	analysis.overflow = true
	s7ARCallableAnalysisFailClosed(analysis)
	contexts = contexts[:s7ARCallableDemandStateLimit]
	for index := range contexts {
		contexts[index].state.incomplete = true
		contexts[index].function.state.incomplete = true
		for argument := range contexts[index].arguments {
			contexts[index].arguments[argument].state.incomplete = true
		}
	}
	return contexts
}

func s7AREvaluateCallableDemandCallOperand(
	analysis *s7ARCallableDemandAnalysis,
	contexts []s7ARCallableDemandEvaluatedCall,
	expression ast.Expr,
	assign func(
		*s7ARCallableDemandEvaluatedCall,
		s7ARCallableDemandOperandSnapshot,
	),
) []s7ARCallableDemandEvaluatedCall {
	var result []s7ARCallableDemandEvaluatedCall
	for _, context := range contexts {
		states := s7AREvaluateCallableDemandExpression(
			analysis,
			[]s7ARCallableDemandState{context.state},
			expression,
		)
		for _, state := range states {
			next := s7ARCallableDemandEvaluatedCall{
				state:    state,
				function: context.function,
				arguments: append(
					[]s7ARCallableDemandOperandSnapshot(nil),
					context.arguments...,
				),
			}
			assign(&next, s7ARCallableDemandSnapshot(state))
			result = append(result, next)
		}
	}
	return s7ARLimitCallableDemandEvaluatedCalls(analysis, result)
}

func s7AREvaluateCallableDemandCallOperands(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	call *ast.CallExpr,
) []s7ARCallableDemandEvaluatedCall {
	if analysis == nil || call == nil {
		return nil
	}
	contexts := make([]s7ARCallableDemandEvaluatedCall, 0, len(states))
	for _, state := range states {
		contexts = append(
			contexts,
			s7ARCallableDemandEvaluatedCall{state: state},
		)
	}
	contexts = s7AREvaluateCallableDemandCallOperand(
		analysis, contexts, call.Fun,
		func(
			context *s7ARCallableDemandEvaluatedCall,
			snapshot s7ARCallableDemandOperandSnapshot,
		) {
			context.function = snapshot
		},
	)
	for argument := range call.Args {
		contexts = s7AREvaluateCallableDemandCallOperand(
			analysis, contexts, call.Args[argument],
			func(
				context *s7ARCallableDemandEvaluatedCall,
				snapshot s7ARCallableDemandOperandSnapshot,
			) {
				context.arguments = append(context.arguments, snapshot)
			},
		)
		if len(contexts) == 0 {
			break
		}
	}
	return contexts
}

func s7ARObserveEvaluatedCallableDemandCall(
	analysis *s7ARCallableDemandAnalysis,
	context s7ARCallableDemandEvaluatedCall,
	call *ast.CallExpr,
) {
	if analysis == nil || call == nil {
		return
	}
	functionState := context.function.state
	if analysis.observed != nil {
		observation := s7ARCloneCallableDemandState(context.state)
		observation.operandSnapshot = true
		observedObjects := map[types.Object]s7ARCallableDemandState{}
		if object := s7ARCallableDemandSnapshotObject(
			analysis.model, call.Fun,
		); object != nil {
			observedObjects[object] = functionState
			s7ARCopyCallableDemandObjectState(
				&observation, functionState, object,
			)
		}
		for index, argument := range call.Args {
			if index >= len(context.arguments) {
				observation.incomplete = true
				break
			}
			object := s7ARCallableDemandSnapshotObject(
				analysis.model, argument,
			)
			if object == nil {
				continue
			}
			source := context.arguments[index].state
			if prior, ok := observedObjects[object]; ok &&
				!s7ARCallableDemandObjectSnapshotEqual(
					prior, source, object,
				) {
				observation.incomplete = true
				continue
			}
			observedObjects[object] = source
			s7ARCopyCallableDemandObjectState(
				&observation, source, object,
			)
			if s7ARCallableSequenceExpression(
				analysis.model, argument,
			) {
				s7ARRefreshCallableDemandObjectBacking(
					&observation, object,
				)
			}
		}
		analysis.observed[call] = s7ARMergeCallableDemandStates(
			analysis, analysis.observed[call],
			[]s7ARCallableDemandState{observation},
		)
	}
	direct := s7ARCallableElementDemandAtState(
		analysis.model, call.Fun, functionState,
	)
	if analysis.invocation {
		analysis.demand.merge(direct)
	}
	s7ARObserveCallableMutationCallAtSnapshots(
		analysis, context, call,
	)
	s7ARObserveCallableAsyncEscapeCallAtSnapshots(
		analysis, context, call,
	)
	if call.Ellipsis == token.NoPos || len(call.Args) == 0 ||
		len(context.arguments) != len(call.Args) {
		return
	}
	last := len(call.Args) - 1
	argumentState := context.arguments[last].state
	view := s7ARCallableDemandViewForExpression(
		analysis.model, call.Args[last],
		argumentState.sequenceViews,
	)
	if !view.any() || !analysis.invocation {
		return
	}
	forwarded := s7ARCallableForwardedDemandAtState(
		analysis.model, analysis.owner, call, functionState,
		analysis.aliases, analysis.origins,
		analysis.receiverObjects, analysis.namedFunctions,
		analysis.resolving, analysis.cache,
	)
	analysis.demand.merge(
		s7ARMapCallableDemandThroughExpressionAtState(
			analysis.model, forwarded,
			call.Args[last], argumentState,
		),
	)
}

func s7ARApplyEvaluatedCallableDemandCallMutation(
	analysis *s7ARCallableDemandAnalysis,
	context s7ARCallableDemandEvaluatedCall,
	call *ast.CallExpr,
) []s7ARCallableDemandState {
	if analysis == nil || call == nil {
		return []s7ARCallableDemandState{context.state}
	}
	declaration := s7ARNamedFunctionDeclaration(
		analysis.model, call.Fun,
	)
	if declaration == nil || declaration.Body == nil {
		return s7ARApplyCallableDemandCallMutation(
			analysis,
			[]s7ARCallableDemandState{context.state},
			call, context,
		)
	}
	failClosed := func() []s7ARCallableDemandState {
		s7ARCallableAnalysisFailClosed(analysis)
		updated := s7ARCloneCallableDemandState(context.state)
		updated.incomplete = true
		return []s7ARCallableDemandState{updated}
	}
	arguments := call.Args
	argumentSnapshots := context.arguments
	var receiverParameters []*ast.Ident
	var receiverArguments []ast.Expr
	var receiverSnapshots []s7ARCallableDemandOperandSnapshot
	if declaration.Recv != nil {
		selector, ok := s7ARUnwrapCallExpression(
			call.Fun,
		).(*ast.SelectorExpr)
		if !ok {
			return failClosed()
		}
		selection := analysis.model.selections[selector]
		if selection == nil {
			return failClosed()
		}
		receiverParameters, _ = s7ARCallableParameters(
			declaration.Recv,
		)
		if len(receiverParameters) != 1 ||
			!s7ARCallableSequenceObject(
				analysis.model.definitions[receiverParameters[0]],
			) {
			return failClosed()
		}
		switch selection.Kind() {
		case types.MethodVal:
			receiverArguments = []ast.Expr{selector.X}
			receiverSnapshots = []s7ARCallableDemandOperandSnapshot{
				context.function,
			}
		case types.MethodExpr:
			if len(call.Args) == 0 {
				return failClosed()
			}
			receiverArguments = call.Args[:1]
			receiverSnapshots = context.arguments[:1]
			arguments = call.Args[1:]
			argumentSnapshots = context.arguments[1:]
		default:
			return failClosed()
		}
	}
	parameters, variadic := s7ARCallableParameters(
		declaration.Type.Params,
	)
	if variadic || len(parameters) != len(arguments) {
		return failClosed()
	}
	if analysis.evaluatedResolving == nil {
		analysis.evaluatedResolving = map[*ast.FuncDecl]bool{}
	}
	if analysis.evaluatedResolving[declaration] {
		return failClosed()
	}

	analysis.evaluatedResolving[declaration] = true
	defer delete(analysis.evaluatedResolving, declaration)

	execution := s7ARCloneCallableDemandStateWithoutScheduledCalls(
		context.state,
	)
	bind := func(
		parameters []*ast.Ident,
		arguments []ast.Expr,
		snapshots []s7ARCallableDemandOperandSnapshot,
	) bool {
		if len(parameters) != len(arguments) ||
			len(arguments) != len(snapshots) {
			return false
		}
		for index := range parameters {
			if !s7ARBindCallableDemandOperand(
				analysis, &execution, parameters[index],
				arguments[index], snapshots[index],
			) {
				return false
			}
		}
		return true
	}
	if !bind(receiverParameters, receiverArguments, receiverSnapshots) ||
		!bind(parameters, arguments, argumentSnapshots) {
		return failClosed()
	}
	var backings []token.Pos
	collectBackings := func(
		arguments []ast.Expr,
		snapshots []s7ARCallableDemandOperandSnapshot,
	) {
		for index, argument := range arguments {
			if !s7ARCallableSequenceExpression(
				analysis.model, argument,
			) {
				continue
			}
			backing := s7ARCallableBackingForDemandExpression(
				analysis.model, snapshots[index].state, argument,
			)
			if backing.identity == token.NoPos || !backing.exact {
				s7ARCallableAnalysisFailClosed(analysis)
				continue
			}
			duplicate := false
			for _, identity := range backings {
				if identity == backing.identity {
					duplicate = true
					break
				}
			}
			if !duplicate {
				backings = append(backings, backing.identity)
			}
		}
	}
	collectBackings(receiverArguments, receiverSnapshots)
	collectBackings(arguments, argumentSnapshots)
	owner := analysis.owner
	analysis.owner = declaration
	flow := s7ARWalkCallableDemandBlock(
		analysis, declaration.Body,
		[]s7ARCallableDemandState{execution},
	)
	completedStates := s7ARCompleteCallableDemandFlow(
		analysis, flow,
	)
	analysis.owner = owner
	var result []s7ARCallableDemandState
	for _, completed := range completedStates {
		result = append(
			result,
			s7ARTransferScheduledCallableEffects(
				context.state, completed,
				s7ARCallableScheduledCall{
					backings: backings,
				},
			),
		)
	}
	return s7ARMergeCallableDemandStates(analysis, result)
}

func s7ARBindCallableDemandOperand(
	analysis *s7ARCallableDemandAnalysis,
	target *s7ARCallableDemandState,
	parameter *ast.Ident,
	argument ast.Expr,
	snapshot s7ARCallableDemandOperandSnapshot,
) bool {
	if parameter != nil && parameter.Name == "_" {
		return true
	}
	if analysis == nil || target == nil || parameter == nil ||
		argument == nil || snapshot.state.incomplete {
		return false
	}
	bound := s7ARApplyCallableDemandAssignment(
		analysis,
		[]s7ARCallableDemandState{snapshot.state},
		[]*ast.Ident{parameter},
		[]ast.Expr{argument},
	)
	if len(bound) != 1 {
		return false
	}
	object := s7ARCallableDemandObject(analysis.model, parameter)
	if object == nil {
		return false
	}
	s7ARCopyCallableDemandObjectState(target, bound[0], object)
	s7ARRefreshCallableDemandObjectBacking(target, object)
	return true
}

func s7ARRefreshCallableDemandObjectBacking(
	target *s7ARCallableDemandState,
	object types.Object,
) {
	if target == nil || object == nil {
		return
	}
	backing := target.sequenceBackings[object]
	if backing.identity != token.NoPos {
		for index, override := range target.backingOrigins[backing.identity] {
			s7ARApplyCallableDemandOriginOverride(
				*target, backing.identity, index,
				override.origins, override.reliable,
			)
		}
	}
}

func s7ARCopyCallableDemandExpressionSnapshot(
	analysis *s7ARCallableDemandAnalysis,
	target *s7ARCallableDemandState,
	source s7ARCallableDemandState,
	expression ast.Expr,
) bool {
	if analysis == nil || target == nil || expression == nil ||
		source.incomplete {
		return false
	}
	object := s7ARCallableDemandSnapshotObject(
		analysis.model, expression,
	)
	if object == nil {
		return false
	}
	s7ARCopyCallableDemandObjectState(target, source, object)
	return true
}

func s7ARCallableDemandSnapshotObject(
	model *s6SourceTypeModel,
	expression ast.Expr,
) types.Object {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		return s7ARCallableDemandObject(model, typed)
	case *ast.IndexExpr:
		return s7ARCallableDemandSnapshotObject(model, typed.X)
	case *ast.SliceExpr:
		return s7ARCallableDemandSnapshotObject(model, typed.X)
	default:
		return nil
	}
}

func s7ARCallableDemandObjectSnapshotEqual(
	left s7ARCallableDemandState,
	right s7ARCallableDemandState,
	object types.Object,
) bool {
	if !s7ARCallableDemandViewEqual(
		left.sequenceViews[object], right.sequenceViews[object],
	) || !s7ARCallableInvocationDemandEqual(
		left.elementDemands[object], right.elementDemands[object],
	) || !s7ARCallableSequenceOriginsEqual(
		left.sequenceOrigins[object], right.sequenceOrigins[object],
	) || left.sequenceReliable[object] != right.sequenceReliable[object] ||
		left.scalarReliable[object] != right.scalarReliable[object] ||
		left.scalarSlots[object] != right.scalarSlots[object] ||
		left.sequenceBackings[object] != right.sequenceBackings[object] {
		return false
	}
	leftOrigins := left.scalarOrigins[object]
	rightOrigins := right.scalarOrigins[object]
	if len(leftOrigins) != len(rightOrigins) {
		return false
	}
	for index := range leftOrigins {
		if leftOrigins[index] != rightOrigins[index] {
			return false
		}
	}
	backing := left.sequenceBackings[object]
	if backing.identity == token.NoPos {
		return true
	}
	return reflect.DeepEqual(
		left.backingOverrideSet[backing.identity],
		right.backingOverrideSet[backing.identity],
	) && reflect.DeepEqual(
		left.backingOrigins[backing.identity],
		right.backingOrigins[backing.identity],
	) && s7ARCallableInvocationDemandEqual(
		left.backingUncertain[backing.identity],
		right.backingUncertain[backing.identity],
	)
}

func s7ARCallableElementDemandAtState(
	model *s6SourceTypeModel,
	expression ast.Expr,
	state s7ARCallableDemandState,
) s7ARCallableInvocationDemand {
	indexed, ok := s7ARUnwrapCallExpression(expression).(*ast.IndexExpr)
	if !ok {
		return s7ARCallableElementDemandForExpression(
			model, expression, state.sequenceViews, state.elementDemands,
		)
	}
	object := s7ARCallableSequenceIdentifierObject(model, indexed.X)
	backing := state.sequenceBackings[object]
	index, exact := s7ARConstantIndex(model, indexed.Index)
	if backing.identity != token.NoPos && backing.exact && exact && index >= 0 {
		absolute := backing.offset + index
		if state.backingOverrideSet[backing.identity][absolute] {
			return s7ARCloneCallableInvocationDemand(
				state.backingOverrides[backing.identity][absolute],
			)
		}
	}
	return s7ARCallableElementDemandForExpression(
		model, expression, state.sequenceViews, state.elementDemands,
	)
}

func s7ARMapCallableDemandThroughExpressionAtState(
	model *s6SourceTypeModel,
	demand s7ARCallableInvocationDemand,
	expression ast.Expr,
	state s7ARCallableDemandState,
) s7ARCallableInvocationDemand {
	view := s7ARCallableDemandViewForExpression(
		model, expression, state.sequenceViews,
	)
	if !demand.any() || !view.any() {
		return s7ARCallableInvocationDemand{}
	}
	if demand.all {
		return s7ARMapCallableDemandThroughView(demand, view)
	}
	backing := s7ARCallableBackingForDemandExpression(
		model, state, expression,
	)
	result := s7ARCallableInvocationDemand{}
	for index := range demand.indices {
		if backing.identity != token.NoPos && backing.exact {
			absolute := backing.offset + index
			if state.backingOverrideSet[backing.identity][absolute] {
				result.merge(
					state.backingOverrides[backing.identity][absolute],
				)
				continue
			}
		}
		local := s7ARCallableInvocationDemand{}
		local.addIndex(index)
		result.merge(s7ARMapCallableDemandThroughView(local, view))
	}
	return result
}

func s7ARObserveCallableMutationCall(
	analysis *s7ARCallableDemandAnalysis,
	state s7ARCallableDemandState,
	call *ast.CallExpr,
) {
	context := s7ARCallableDemandEvaluatedCall{
		state:    state,
		function: s7ARCallableDemandSnapshot(state),
	}
	for range call.Args {
		context.arguments = append(
			context.arguments, s7ARCallableDemandSnapshot(state),
		)
	}
	s7ARObserveCallableMutationCallAtSnapshots(
		analysis, context, call,
	)
}

func s7ARObserveCallableMutationCallAtSnapshots(
	analysis *s7ARCallableDemandAnalysis,
	context s7ARCallableDemandEvaluatedCall,
	call *ast.CallExpr,
) {
	if analysis == nil || analysis.mutationDemand == nil || call == nil {
		return
	}
	name := s6CallName(call)
	if name == "len" || name == "cap" {
		return
	}
	if value, ok := analysis.model.expressionTypes[call.Fun]; ok &&
		value.IsType() {
		return
	}
	if name == "copy" && len(call.Args) == 2 {
		if s7ARCallableDemandViewForExpression(
			analysis.model, call.Args[0],
			context.arguments[0].state.sequenceViews,
		).any() {
			analysis.mutationDemand.addAll()
		}
		return
	}
	if name == "append" {
		if len(call.Args) > 1 &&
			!s7ARCallableAppendProvablyAllocates(
				analysis.model, call.Args[0],
			) &&
			s7ARCallableDemandViewForExpression(
				analysis.model, call.Args[0],
				context.arguments[0].state.sequenceViews,
			).any() {
			analysis.mutationDemand.addAll()
		}
		return
	}
	declaration := s7ARNamedFunctionDeclaration(analysis.model, call.Fun)
	if declaration == nil || declaration.Body == nil {
		for index, argument := range call.Args {
			if s7ARCallableDemandViewForExpression(
				analysis.model, argument,
				context.arguments[index].state.sequenceViews,
			).any() {
				analysis.mutationDemand.addAll()
			}
		}
		return
	}
	parameters, _ := s7ARCallableParameters(declaration.Type.Params)
	argumentOffset := 0
	if declaration.Recv != nil {
		if selector, ok := s7ARUnwrapCallExpression(
			call.Fun,
		).(*ast.SelectorExpr); ok {
			if selection := analysis.model.selections[selector]; selection != nil &&
				selection.Kind() == types.MethodExpr {
				argumentOffset = 1
			}
		}
	}
	for index, parameter := range parameters {
		if parameter == nil || index+argumentOffset >= len(call.Args) {
			continue
		}
		argumentIndex := index + argumentOffset
		argument := call.Args[argumentIndex]
		view := s7ARCallableDemandViewForExpression(
			analysis.model, argument,
			context.arguments[argumentIndex].state.sequenceViews,
		)
		if !view.any() {
			continue
		}
		object := analysis.model.definitions[parameter]
		if !s7ARCallableSequenceObject(object) {
			continue
		}
		nested := s7ARCallableParameterMutationDemand(
			analysis.model, declaration, object,
			analysis.mutationResolving, analysis.mutationCache,
		)
		analysis.mutationDemand.merge(
			s7ARMapCallableDemandThroughView(nested, view),
		)
	}
}

func s7ARObserveCallableAsyncEscapeCall(
	analysis *s7ARCallableDemandAnalysis,
	state s7ARCallableDemandState,
	call *ast.CallExpr,
) {
	context := s7ARCallableDemandEvaluatedCall{
		state:    state,
		function: s7ARCallableDemandSnapshot(state),
	}
	for range call.Args {
		context.arguments = append(
			context.arguments, s7ARCallableDemandSnapshot(state),
		)
	}
	s7ARObserveCallableAsyncEscapeCallAtSnapshots(
		analysis, context, call,
	)
}

func s7ARObserveCallableAsyncEscapeCallAtSnapshots(
	analysis *s7ARCallableDemandAnalysis,
	context s7ARCallableDemandEvaluatedCall,
	call *ast.CallExpr,
) {
	if analysis == nil || analysis.asyncEscapeDemand == nil || call == nil {
		return
	}
	declaration := s7ARNamedFunctionDeclaration(analysis.model, call.Fun)
	if declaration == nil || declaration.Body == nil {
		return
	}
	parameters, _ := s7ARCallableParameters(declaration.Type.Params)
	argumentOffset := 0
	if declaration.Recv != nil {
		if selector, ok := s7ARUnwrapCallExpression(
			call.Fun,
		).(*ast.SelectorExpr); ok {
			if selection := analysis.model.selections[selector]; selection != nil &&
				selection.Kind() == types.MethodExpr {
				argumentOffset = 1
			}
		}
	}
	for index, parameter := range parameters {
		if parameter == nil || index+argumentOffset >= len(call.Args) {
			continue
		}
		argumentIndex := index + argumentOffset
		view := s7ARCallableDemandViewForExpression(
			analysis.model, call.Args[argumentIndex],
			context.arguments[argumentIndex].state.sequenceViews,
		)
		if !view.any() {
			continue
		}
		object := analysis.model.definitions[parameter]
		if !s7ARCallableSequenceObject(object) {
			continue
		}
		nested := s7ARCallableParameterAsyncEscapeDemand(
			analysis.model, declaration, object,
			analysis.aliases, analysis.origins, analysis.receiverObjects,
			analysis.namedFunctions, analysis.asyncEscapeResolving,
			analysis.asyncEscapeCache,
		)
		analysis.asyncEscapeDemand.merge(
			s7ARMapCallableDemandThroughView(nested, view),
		)
	}
}

func s7ARApplyCallableDemandExpressionMutations(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	expressions ...ast.Expr,
) []s7ARCallableDemandState {
	result := states
	for _, expression := range expressions {
		if expression == nil {
			continue
		}
		var calls []*ast.CallExpr
		ast.Inspect(expression, func(node ast.Node) bool {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			if call, ok := node.(*ast.CallExpr); ok {
				calls = append(calls, call)
			}
			return true
		})
		for index := len(calls) - 1; index >= 0; index-- {
			result = s7ARApplyCallableDemandCallMutation(
				analysis, result, calls[index],
			)
		}
	}
	return result
}

func s7ARApplyCallableDemandCallMutation(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	call *ast.CallExpr,
	evaluated ...s7ARCallableDemandEvaluatedCall,
) []s7ARCallableDemandState {
	if analysis == nil || call == nil || len(states) == 0 {
		return states
	}
	name := s6CallName(call)
	if name == "len" || name == "cap" {
		return states
	}
	if value, ok := analysis.model.expressionTypes[call.Fun]; ok &&
		value.IsType() {
		return states
	}
	if name == "copy" && len(call.Args) == 2 {
		result := make([]s7ARCallableDemandState, 0, len(states))
		for stateIndex, state := range states {
			argumentStates := []s7ARCallableDemandState{state, state}
			if len(evaluated) == len(states) &&
				len(evaluated[stateIndex].arguments) == len(call.Args) {
				for index := range argumentStates {
					argumentStates[index] =
						evaluated[stateIndex].arguments[index].state
				}
			}
			updated := s7ARCloneCallableDemandState(state)
			backing := s7ARCallableBackingForDemandExpression(
				analysis.model, argumentStates[0], call.Args[0],
			)
			demand := s7ARCallableInvocationDemand{all: true}
			destinationLength, destinationExact :=
				s7ARCallableDemandSequenceLength(
					analysis.model, argumentStates[0], call.Args[0],
				)
			sourceLength, sourceExact := s7ARCallableDemandSequenceLength(
				analysis.model, argumentStates[1], call.Args[1],
			)
			if destinationExact && sourceExact {
				if sourceLength < destinationLength {
					destinationLength = sourceLength
				}
				source, sourceReliable :=
					s7ARCallableSequenceAtDemandState(
						analysis.model, call.Args[1], argumentStates[1],
					)
				if sourceReliable && !source.incomplete {
					demand = s7ARCallableInvocationDemand{}
					for index := 0; index < destinationLength; index++ {
						var origins []s7ARCallableExpressionOrigin
						reliable := !source.uncertainIndices[index]
						for _, alternative := range source.alternatives {
							if index >= len(alternative) {
								reliable = false
								break
							}
							origins = append(origins, alternative[index])
						}
						if !reliable || len(origins) == 0 {
							demand.addIndex(index)
							continue
						}
						s7ARSetCallableDemandBackingOverride(
							&updated, backing, index,
							s7ARMapCallableDemandThroughExpressionAtState(
								analysis.model,
								s7ARCallableInvocationDemand{
									indices: map[int]bool{index: true},
								},
								call.Args[1], argumentStates[1],
							),
							origins, true,
						)
					}
				} else {
					demand = s7ARCallableInvocationDemand{}
					for index := 0; index < destinationLength; index++ {
						demand.addIndex(index)
					}
				}
			}
			s7ARMarkCallableDemandBackingUncertain(
				&updated, backing, demand,
			)
			result = append(result, updated)
		}
		return s7ARMergeCallableDemandStates(analysis, result)
	}
	if name == "append" {
		if len(call.Args) < 2 ||
			s7ARCallableAppendProvablyAllocates(
				analysis.model, call.Args[0],
			) {
			return states
		}
		result := make([]s7ARCallableDemandState, 0, len(states))
		for stateIndex, state := range states {
			argumentStates := make(
				[]s7ARCallableDemandState, len(call.Args),
			)
			for index := range argumentStates {
				argumentStates[index] = state
			}
			if len(evaluated) == len(states) &&
				len(evaluated[stateIndex].arguments) == len(call.Args) {
				for index := range argumentStates {
					argumentStates[index] =
						evaluated[stateIndex].arguments[index].state
				}
			}
			updated := s7ARCloneCallableDemandState(state)
			backing := s7ARCallableBackingForDemandExpression(
				analysis.model, argumentStates[0], call.Args[0],
			)
			demand := s7ARCallableInvocationDemand{all: true}
			baseLength, baseExact := s7ARCallableDemandSequenceLength(
				analysis.model, argumentStates[0], call.Args[0],
			)
			count := len(call.Args) - 1
			countExact := call.Ellipsis == token.NoPos
			if !countExact && len(call.Args) == 2 {
				count, countExact = s7ARCallableDemandSequenceLength(
					analysis.model, argumentStates[1], call.Args[1],
				)
			}
			if baseExact && countExact {
				demand = s7ARCallableInvocationDemand{}
				for index := 0; index < count; index++ {
					demand.addIndex(baseLength + index)
				}
			}
			s7ARMarkCallableDemandBackingUncertain(
				&updated, backing, demand,
			)
			result = append(result, updated)
		}
		return s7ARMergeCallableDemandStates(analysis, result)
	}
	directLiteral, _ := s7ARUnwrapCallExpression(call.Fun).(*ast.FuncLit)
	if analysis.origins == nil && directLiteral == nil {
		return states
	}
	result := make([]s7ARCallableDemandState, 0, len(states))
	for stateIndex, state := range states {
		functionState := state
		argumentStates := make(
			[]s7ARCallableDemandState, len(call.Args),
		)
		for index := range argumentStates {
			argumentStates[index] = state
		}
		if len(evaluated) == len(states) {
			functionState = evaluated[stateIndex].function.state
			if len(evaluated[stateIndex].arguments) == len(call.Args) {
				for index := range argumentStates {
					argumentStates[index] =
						evaluated[stateIndex].arguments[index].state
				}
			}
		}
		literalTargets := map[*ast.FuncLit]bool{}
		if directLiteral != nil {
			literalTargets[directLiteral] = true
		} else {
			literalTargets, _ =
				s7ARFunctionLiteralExpressionsAtDemandStates(
					analysis.model, analysis.owner, call.Fun,
					[]s7ARCallableDemandState{functionState},
					analysis.aliases, analysis.origins, analysis.receiverObjects,
				)
		}
		if len(literalTargets) != 0 {
			for literal := range literalTargets {
				execution :=
					s7ARCloneCallableDemandStateWithoutScheduledCalls(state)
				flow := s7ARWalkCallableDemandBlock(
					analysis, literal.Body,
					[]s7ARCallableDemandState{
						execution,
					},
				)
				completed := s7ARCompleteCallableDemandFlow(
					analysis, flow,
				)
				for _, candidate := range completed {
					pending := s7ARCloneCallableDemandState(state)
					candidate.scheduledCalls = pending.scheduledCalls
					result = append(result, candidate)
				}
			}
			continue
		}
		updated := s7ARCloneCallableDemandState(state)
		declaration := s7ARNamedFunctionDeclaration(
			analysis.model, call.Fun,
		)
		if declaration == nil || declaration.Body == nil {
			for index, argument := range call.Args {
				if !s7ARCallableSequenceExpression(
					analysis.model, argument,
				) {
					continue
				}
				backing := s7ARCallableBackingForDemandExpression(
					analysis.model, argumentStates[index], argument,
				)
				s7ARMarkCallableDemandBackingUncertain(
					&updated, backing,
					s7ARCallableInvocationDemand{all: true},
				)
			}
			result = append(result, updated)
			continue
		}
		parameters, variadic := s7ARCallableParameters(
			declaration.Type.Params,
		)
		argumentOffset := 0
		if declaration.Recv != nil {
			if selector, ok := s7ARUnwrapCallExpression(
				call.Fun,
			).(*ast.SelectorExpr); ok {
				if selection := analysis.model.selections[selector]; selection != nil &&
					selection.Kind() == types.MethodExpr {
					argumentOffset = 1
				}
			}
		}
		for index, parameter := range parameters {
			if parameter == nil || index+argumentOffset >= len(call.Args) {
				continue
			}
			if variadic && index == len(parameters)-1 &&
				call.Ellipsis == token.NoPos {
				continue
			}
			object := analysis.model.definitions[parameter]
			if !s7ARCallableSequenceObject(object) {
				continue
			}
			demand := s7ARCallableParameterMutationDemand(
				analysis.model, declaration, object,
				analysis.mutationResolving, analysis.mutationCache,
			)
			backing := s7ARCallableBackingForDemandExpression(
				analysis.model, argumentStates[index+argumentOffset],
				call.Args[index+argumentOffset],
			)
			if demand.any() {
				s7ARMarkCallableDemandBackingUncertain(
					&updated, backing, demand,
				)
			}
			escaped := s7ARCallableParameterAsyncEscapeDemand(
				analysis.model, declaration, object,
				analysis.aliases, analysis.origins,
				analysis.receiverObjects, analysis.namedFunctions,
				analysis.asyncEscapeResolving, analysis.asyncEscapeCache,
			)
			s7ARMarkCallableDemandBackingEscaped(
				&updated, backing, escaped,
			)
		}
		result = append(result, updated)
	}
	return s7ARMergeCallableDemandStates(analysis, result)
}

func s7ARCallableDemandSequenceLength(
	model *s6SourceTypeModel,
	state s7ARCallableDemandState,
	expression ast.Expr,
) (int, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	if composite, ok := expression.(*ast.CompositeLit); ok {
		return len(composite.Elts), true
	}
	sequence, reliable := s7ARCallableSequenceAtDemandState(
		model, expression, state,
	)
	if !reliable || sequence.incomplete ||
		len(sequence.alternatives) == 0 {
		return 0, false
	}
	length := len(sequence.alternatives[0])
	for _, alternative := range sequence.alternatives[1:] {
		if len(alternative) != length {
			return 0, false
		}
	}
	return length, true
}

func s7ARCallableAppendProvablyAllocates(
	model *s6SourceTypeModel,
	expression ast.Expr,
) bool {
	sliced, ok := s7ARUnwrapCallExpression(expression).(*ast.SliceExpr)
	if !ok || sliced.Max == nil {
		return false
	}
	low := 0
	if sliced.Low != nil {
		value, exact := s7ARConstantIndex(model, sliced.Low)
		if !exact {
			return false
		}
		low = value
	}
	maximum, exact := s7ARConstantIndex(model, sliced.Max)
	return exact && maximum == low
}

func s7ARWalkCallableDemandIf(
	analysis *s7ARCallableDemandAnalysis,
	statement *ast.IfStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	prefix := s7ARCallableDemandFlow{next: states}
	if statement.Init != nil {
		prefix = s7ARWalkCallableDemandStatement(
			analysis, statement.Init, prefix.next,
		)
	}
	s7ARObserveCallableDemandExpressions(analysis, prefix.next, statement.Cond)
	prefix.next = s7ARApplyCallableDemandExpressionMutations(
		analysis, prefix.next, statement.Cond,
	)
	s7ARPublishScheduledCallableCalls(analysis, prefix.next, true)
	thenFlow := s7ARWalkCallableDemandBlock(
		analysis, statement.Body, prefix.next,
	)
	elseFlow := s7ARCallableDemandFlow{next: prefix.next}
	if statement.Else != nil {
		elseFlow = s7ARWalkCallableDemandStatement(
			analysis, statement.Else, prefix.next,
		)
	}
	return s7ARCallableDemandFlow{
		next: s7ARMergeCallableDemandStates(
			analysis, thenFlow.next, elseFlow.next,
		),
		returns: s7ARMergeCallableDemandStates(
			analysis, prefix.returns, thenFlow.returns, elseFlow.returns,
		),
		breaks: s7ARMergeCallableDemandStates(
			analysis, prefix.breaks, thenFlow.breaks, elseFlow.breaks,
		),
		continues: s7ARMergeCallableDemandStates(
			analysis, prefix.continues,
			thenFlow.continues, elseFlow.continues,
		),
	}
}

func s7ARWalkCallableDemandFor(
	analysis *s7ARCallableDemandAnalysis,
	statement *ast.ForStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	prefix := s7ARCallableDemandFlow{next: states}
	if statement.Init != nil {
		prefix = s7ARWalkCallableDemandStatement(
			analysis, statement.Init, prefix.next,
		)
	}
	headers := s7ARMergeCallableDemandStates(analysis, prefix.next)
	pending := headers
	exits := headers
	var breaks []s7ARCallableDemandState
	returns := append(
		[]s7ARCallableDemandState(nil), prefix.returns...,
	)
	for iteration := 0; len(pending) != 0 && iteration < 32; iteration++ {
		s7ARObserveCallableDemandExpressions(analysis, pending, statement.Cond)
		pending = s7ARApplyCallableDemandExpressionMutations(
			analysis, pending, statement.Cond,
		)
		s7ARPublishScheduledCallableCalls(analysis, pending, true)
		exits = s7ARMergeCallableDemandStates(analysis, exits, pending)
		body := s7ARWalkCallableDemandBlock(
			analysis, statement.Body, pending,
		)
		breaks = s7ARMergeCallableDemandStates(
			analysis, breaks, body.breaks,
		)
		returns = s7ARMergeCallableDemandStates(
			analysis, returns, body.returns,
		)
		back := s7ARMergeCallableDemandStates(
			analysis, body.next, body.continues,
		)
		if statement.Post != nil {
			post := s7ARWalkCallableDemandStatement(
				analysis, statement.Post, back,
			)
			back = post.next
			breaks = s7ARMergeCallableDemandStates(
				analysis, breaks, post.breaks,
			)
			returns = s7ARMergeCallableDemandStates(
				analysis, returns, post.returns,
			)
		}
		merged := s7ARMergeCallableDemandStates(analysis, headers, back)
		pending = s7ARCallableDemandStateDifference(merged, headers)
		headers = merged
		exits = s7ARMergeCallableDemandStates(analysis, exits, pending)
		if analysis.overflow {
			break
		}
		if iteration == 31 && len(pending) != 0 {
			s7ARCallableAnalysisFailClosed(analysis)
		}
	}
	return s7ARCallableDemandFlow{
		next: s7ARMergeCallableDemandStates(
			analysis, prefix.breaks, exits, breaks,
		),
		returns:   returns,
		continues: prefix.continues,
	}
}

func s7ARCallableDemandStateDifference(
	left []s7ARCallableDemandState,
	right []s7ARCallableDemandState,
) []s7ARCallableDemandState {
	var result []s7ARCallableDemandState
	for _, candidate := range left {
		found := false
		for _, existing := range right {
			if s7ARCallableDemandStateEqual(candidate, existing) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, candidate)
		}
	}
	return result
}

func s7ARWalkCallableDemandRange(
	analysis *s7ARCallableDemandAnalysis,
	statement *ast.RangeStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	headers := s7ARMergeCallableDemandStates(analysis, states)
	pending := headers
	exits := headers
	var breaks []s7ARCallableDemandState
	var returns []s7ARCallableDemandState
	for iteration := 0; len(pending) != 0 && iteration < 32; iteration++ {
		s7ARObserveCallableDemandExpressions(analysis, pending, statement.X)
		pending = s7ARApplyCallableDemandExpressionMutations(
			analysis, pending, statement.X,
		)
		s7ARPublishScheduledCallableCalls(analysis, pending, true)
		iterationStates := make([]s7ARCallableDemandState, 0, len(pending))
		for _, state := range pending {
			updated := s7ARCloneCallableDemandState(state)
			if key, ok := statement.Key.(*ast.Ident); ok {
				object := s7ARCallableDemandObject(analysis.model, key)
				delete(updated.sequenceViews, object)
				delete(updated.elementDemands, object)
			}
			if value, ok := statement.Value.(*ast.Ident); ok {
				object := s7ARCallableDemandObject(analysis.model, value)
				delete(updated.sequenceViews, object)
				delete(updated.elementDemands, object)
				view := s7ARCallableDemandViewForExpression(
					analysis.model, statement.X, state.sequenceViews,
				)
				if view.any() {
					demand := s7ARCallableInvocationDemand{all: true}
					updated.elementDemands[object] = demand
				}
			}
			iterationStates = append(iterationStates, updated)
		}
		body := s7ARWalkCallableDemandBlock(
			analysis, statement.Body, iterationStates,
		)
		breaks = s7ARMergeCallableDemandStates(
			analysis, breaks, body.breaks,
		)
		returns = s7ARMergeCallableDemandStates(
			analysis, returns, body.returns,
		)
		back := s7ARMergeCallableDemandStates(
			analysis, body.next, body.continues,
		)
		merged := s7ARMergeCallableDemandStates(analysis, headers, back)
		pending = s7ARCallableDemandStateDifference(merged, headers)
		headers = merged
		exits = s7ARMergeCallableDemandStates(analysis, exits, pending)
		if analysis.overflow {
			break
		}
		if iteration == 31 && len(pending) != 0 {
			s7ARCallableAnalysisFailClosed(analysis)
		}
	}
	return s7ARCallableDemandFlow{
		next:    s7ARMergeCallableDemandStates(analysis, exits, breaks),
		returns: returns,
	}
}

func s7ARWalkCallableDemandSwitch(
	analysis *s7ARCallableDemandAnalysis,
	statement *ast.SwitchStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	prefix := s7ARCallableDemandFlow{next: states}
	if statement.Init != nil {
		prefix = s7ARWalkCallableDemandStatement(
			analysis, statement.Init, prefix.next,
		)
	}
	s7ARObserveCallableDemandExpressions(analysis, prefix.next, statement.Tag)
	prefix.next = s7ARApplyCallableDemandExpressionMutations(
		analysis, prefix.next, statement.Tag,
	)
	s7ARPublishScheduledCallableCalls(analysis, prefix.next, true)
	return s7ARWalkCallableDemandClauses(
		analysis, statement.Body, prefix, false,
	)
}

func s7ARWalkCallableDemandTypeSwitch(
	analysis *s7ARCallableDemandAnalysis,
	statement *ast.TypeSwitchStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	prefix := s7ARCallableDemandFlow{next: states}
	if statement.Init != nil {
		prefix = s7ARWalkCallableDemandStatement(
			analysis, statement.Init, prefix.next,
		)
	}
	if statement.Assign != nil {
		prefix = s7ARWalkCallableDemandStatement(
			analysis, statement.Assign, prefix.next,
		)
	}
	return s7ARWalkCallableDemandClauses(
		analysis, statement.Body, prefix, false,
	)
}

func s7ARWalkCallableDemandSelect(
	analysis *s7ARCallableDemandAnalysis,
	statement *ast.SelectStmt,
	states []s7ARCallableDemandState,
) s7ARCallableDemandFlow {
	if statement == nil || statement.Body == nil {
		s7ARCallableAnalysisFailClosed(analysis)
		return s7ARCallableDemandFlow{next: states}
	}
	preselected := states
	for _, clauseStatement := range statement.Body.List {
		clause, ok := clauseStatement.(*ast.CommClause)
		if !ok {
			s7ARCallableAnalysisFailClosed(analysis)
			continue
		}
		preselected = s7ARPreselectCallableDemandCommunication(
			analysis, preselected, clause.Comm,
		)
	}
	return s7ARWalkCallableDemandClauses(
		analysis, statement.Body,
		s7ARCallableDemandFlow{next: preselected}, true,
	)
}

func s7ARPreselectCallableDemandCommunication(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	communication ast.Stmt,
) []s7ARCallableDemandState {
	if communication == nil {
		return states
	}
	switch typed := communication.(type) {
	case *ast.SendStmt:
		return s7AREvaluateCallableDemandExpressions(
			analysis, states, typed.Chan, typed.Value,
		)
	case *ast.ExprStmt:
		channel, ok := s7ARCallableDemandReceiveChannel(typed.X)
		if !ok {
			s7ARCallableAnalysisFailClosed(analysis)
			return states
		}
		return s7AREvaluateCallableDemandExpression(
			analysis, states, channel,
		)
	case *ast.AssignStmt:
		if len(typed.Rhs) != 1 {
			s7ARCallableAnalysisFailClosed(analysis)
			return states
		}
		channel, ok := s7ARCallableDemandReceiveChannel(typed.Rhs[0])
		if !ok {
			s7ARCallableAnalysisFailClosed(analysis)
			return states
		}
		return s7AREvaluateCallableDemandExpression(
			analysis, states, channel,
		)
	default:
		s7ARCallableAnalysisFailClosed(analysis)
		return states
	}
}

func s7ARCallableDemandReceiveChannel(
	expression ast.Expr,
) (ast.Expr, bool) {
	receive, ok := s7ARUnwrapCallExpression(expression).(*ast.UnaryExpr)
	if !ok || receive.Op != token.ARROW {
		return nil, false
	}
	return receive.X, true
}

func s7ARApplySelectedCallableDemandCommunication(
	analysis *s7ARCallableDemandAnalysis,
	states []s7ARCallableDemandState,
	communication ast.Stmt,
) []s7ARCallableDemandState {
	if communication == nil {
		return states
	}
	switch typed := communication.(type) {
	case *ast.SendStmt, *ast.ExprStmt:
		return states
	case *ast.AssignStmt:
		if len(typed.Rhs) != 1 {
			s7ARCallableAnalysisFailClosed(analysis)
			return states
		}
		if _, ok := s7ARCallableDemandReceiveChannel(typed.Rhs[0]); !ok {
			s7ARCallableAnalysisFailClosed(analysis)
			return states
		}
		afterLeft := s7AREvaluateCallableDemandExpressions(
			analysis, states, typed.Lhs...,
		)
		mutated := s7ARApplyCallableDemandIndexMutations(
			analysis, afterLeft, typed,
		)
		return s7ARApplyCallableDemandAssignment(
			analysis, mutated,
			s7ARAssignmentIdentifiers(typed.Lhs), typed.Rhs,
		)
	default:
		s7ARCallableAnalysisFailClosed(analysis)
		return states
	}
}

func s7ARWalkCallableDemandClauses(
	analysis *s7ARCallableDemandAnalysis,
	body *ast.BlockStmt,
	prefix s7ARCallableDemandFlow,
	selectClauses bool,
) s7ARCallableDemandFlow {
	result := s7ARCallableDemandFlow{
		breaks:    prefix.breaks,
		continues: prefix.continues,
		returns:   prefix.returns,
	}
	defaultPresent := false
	if body == nil {
		result.next = prefix.next
		return result
	}
	for _, statement := range body.List {
		var list []ast.Stmt
		states := prefix.next
		switch clause := statement.(type) {
		case *ast.CaseClause:
			defaultPresent = defaultPresent || len(clause.List) == 0
			s7ARObserveCallableDemandExpressions(
				analysis, states, clause.List...,
			)
			states = s7ARApplyCallableDemandExpressionMutations(
				analysis, states, clause.List...,
			)
			s7ARPublishScheduledCallableCalls(analysis, states, true)
			list = clause.Body
		case *ast.CommClause:
			defaultPresent = defaultPresent || clause.Comm == nil
			if clause.Comm != nil {
				if selectClauses {
					states = s7ARApplySelectedCallableDemandCommunication(
						analysis, states, clause.Comm,
					)
				} else {
					communication := s7ARWalkCallableDemandStatement(
						analysis, clause.Comm, states,
					)
					states = communication.next
					result.continues = s7ARMergeCallableDemandStates(
						analysis, result.continues,
						communication.continues,
					)
					result.returns = s7ARMergeCallableDemandStates(
						analysis, result.returns,
						communication.returns,
					)
				}
			}
			list = clause.Body
		default:
			s7ARCallableAnalysisFailClosed(analysis)
			continue
		}
		clauseFlow := s7ARWalkCallableDemandBlock(
			analysis, &ast.BlockStmt{List: list}, states,
		)
		result.next = s7ARMergeCallableDemandStates(
			analysis, result.next, clauseFlow.next, clauseFlow.breaks,
		)
		result.continues = s7ARMergeCallableDemandStates(
			analysis, result.continues, clauseFlow.continues,
		)
		result.returns = s7ARMergeCallableDemandStates(
			analysis, result.returns, clauseFlow.returns,
		)
	}
	if !defaultPresent && !selectClauses {
		result.next = s7ARMergeCallableDemandStates(
			analysis, result.next, prefix.next,
		)
	}
	return result
}

func s7ARCallableReachingStatesAtCalls(
	model *s6SourceTypeModel,
	owner *ast.FuncDecl,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
) map[*ast.CallExpr][]s7ARCallableDemandState {
	if owner == nil || owner.Body == nil {
		return nil
	}
	parameters, variadic := s7ARCallableParameters(owner.Type.Params)
	demand := s7ARCallableInvocationDemand{}
	resolving := map[*ast.BlockStmt]bool{owner.Body: true}
	analysis := &s7ARCallableDemandAnalysis{
		model:                model,
		owner:                owner,
		aliases:              aliases,
		origins:              origins,
		receiverObjects:      receiverObjects,
		namedFunctions:       namedFunctions,
		resolving:            resolving,
		cache:                map[*ast.BlockStmt]s7ARCallableInvocationDemand{},
		demand:               &demand,
		observed:             map[*ast.CallExpr][]s7ARCallableDemandState{},
		invocation:           true,
		mutationResolving:    map[types.Object]bool{},
		mutationCache:        map[types.Object]s7ARCallableInvocationDemand{},
		asyncEscapeResolving: map[types.Object]bool{},
		asyncEscapeCache:     map[types.Object]s7ARCallableInvocationDemand{},
	}
	initial := s7ARCallableDemandState{
		sequenceViews:      map[types.Object]s7ARCallableDemandView{},
		elementDemands:     map[types.Object]s7ARCallableInvocationDemand{},
		sequenceOrigins:    map[types.Object]s7ARCallableSequenceOrigins{},
		sequenceReliable:   map[types.Object]bool{},
		scalarOrigins:      map[types.Object][]s7ARCallableExpressionOrigin{},
		scalarReliable:     map[types.Object]bool{},
		scalarSlots:        map[types.Object]bool{},
		sequenceBackings:   map[types.Object]s7ARCallableDemandBacking{},
		backingOverrides:   map[token.Pos]map[int]s7ARCallableInvocationDemand{},
		backingOverrideSet: map[token.Pos]map[int]bool{},
		backingOrigins:     map[token.Pos]map[int]s7ARCallableOriginOverride{},
		backingUncertain:   map[token.Pos]s7ARCallableInvocationDemand{},
		escapedBackings:    map[token.Pos]s7ARCallableInvocationDemand{},
	}
	allParameters := append([]*ast.Ident(nil), parameters...)
	receivers, _ := s7ARCallableParameters(owner.Recv)
	allParameters = append(allParameters, receivers...)
	for index, parameter := range allParameters {
		if parameter == nil {
			continue
		}
		object := model.definitions[parameter]
		if object == nil || object.Type() == nil {
			continue
		}
		if s7ARCallableSequenceObject(object) {
			initial.sequenceBackings[object] =
				s7ARCallableDemandBacking{
					identity: parameter.Pos(),
					exact:    true,
				}
			if sequence, ok := origins.sequences[object]; ok {
				initial.sequenceOrigins[object] =
					s7ARCloneCallableSequenceOrigins(sequence)
				initial.sequenceReliable[object] = true
			}
			if index == len(parameters)-1 && variadic &&
				s7ARVariadicParameterCarriesCallable(model, parameter) {
				initial.sequenceViews[object] =
					s7ARCallableDemandView{offsets: map[int]bool{0: true}}
			}
			continue
		}
		if s7ARFunctionValuedType(object.Type()) {
			if bindings := origins.bindings[object]; len(bindings) != 0 {
				initial.scalarOrigins[object] = append(
					[]s7ARCallableExpressionOrigin(nil), bindings...,
				)
				initial.scalarReliable[object] = true
			}
			if len(aliases[object]) != 0 ||
				origins.snapshotBindings[object] {
				initial.scalarSlots[object] = true
			}
		}
	}
	flow := s7ARWalkCallableDemandBlock(
		analysis, owner.Body, []s7ARCallableDemandState{initial},
	)
	completed := s7ARCompleteCallableDemandFlow(analysis, flow)
	for _, state := range completed {
		if state.incomplete {
			s7ARCallableAnalysisFailClosed(analysis)
			break
		}
	}
	if analysis.overflow || analysis.incomplete {
		failClosed := s7ARCallableDemandState{incomplete: true}
		ast.Inspect(owner.Body, func(node ast.Node) bool {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			if call, ok := node.(*ast.CallExpr); ok {
				analysis.observed[call] = s7ARMergeCallableDemandStates(
					analysis, analysis.observed[call],
					[]s7ARCallableDemandState{failClosed},
				)
			}
			return true
		})
		for call, states := range analysis.observed {
			for index := range states {
				states[index].incomplete = true
			}
			analysis.observed[call] = states
		}
	}
	return analysis.observed
}

func s7ARFunctionLiteralExpressionsAtDemandStates(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	states []s7ARCallableDemandState,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
) (map[*ast.FuncLit]bool, bool) {
	for _, state := range states {
		if state.incomplete {
			return nil, true
		}
	}
	identifier, ok := s7ARUnwrapCallExpression(expression).(*ast.Ident)
	if !ok || len(states) == 0 {
		return s7ARFunctionLiteralExpressions(
			model, function, expression, aliases, origins, receiverObjects,
			map[ast.Expr]bool{},
		)
	}
	object := s7ARCallableDemandObject(model, identifier)
	for _, state := range states {
		if !state.scalarReliable[object] {
			return s7ARFunctionLiteralExpressions(
				model, function, expression, aliases, origins, receiverObjects,
				map[ast.Expr]bool{},
			)
		}
	}
	result := map[*ast.FuncLit]bool{}
	incomplete := false
	relevant := false
	for _, state := range states {
		relevant = relevant || state.elementDemands[object].any() ||
			state.scalarSlots[object]
		candidates := state.scalarOrigins[object]
		if len(candidates) == 0 {
			incomplete = true
			continue
		}
		for _, candidate := range candidates {
			originFunction := candidate.function
			if originFunction == nil {
				originFunction = function
			}
			literals, unresolved := s7ARFunctionLiteralExpressions(
				model, originFunction, candidate.expression, aliases, origins,
				receiverObjects, map[ast.Expr]bool{},
			)
			incomplete = incomplete || unresolved
			for literal := range literals {
				result[literal] = true
			}
		}
	}
	if relevant && len(result) == 0 {
		incomplete = true
	}
	return result, incomplete
}

func s7ARNamedCallableTargetsAtDemandStates(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	states []s7ARCallableDemandState,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	context s7ARCallableResolutionContext,
) ([]s7ARNamedCallableTarget, bool) {
	for _, state := range states {
		if state.incomplete {
			return nil, true
		}
	}
	identifier, ok := s7ARUnwrapCallExpression(expression).(*ast.Ident)
	if !ok || len(states) == 0 {
		return s7ARNamedCallableTargets(
			model, function, expression, aliases, origins, receiverObjects,
			namedFunctions, map[ast.Expr]bool{}, context,
		)
	}
	object := s7ARCallableDemandObject(model, identifier)
	for _, state := range states {
		if !state.scalarReliable[object] {
			return s7ARNamedCallableTargets(
				model, function, expression, aliases, origins, receiverObjects,
				namedFunctions, map[ast.Expr]bool{}, context,
			)
		}
	}
	var result []s7ARNamedCallableTarget
	incomplete := false
	for _, state := range states {
		candidates := state.scalarOrigins[object]
		if len(candidates) == 0 {
			incomplete = true
			continue
		}
		for _, candidate := range candidates {
			originFunction := candidate.function
			if originFunction == nil {
				originFunction = function
			}
			targets, unresolved := s7ARNamedCallableTargets(
				model, originFunction, candidate.expression, aliases, origins,
				receiverObjects, namedFunctions, map[ast.Expr]bool{}, context,
			)
			incomplete = incomplete || unresolved
			result = s7ARAppendNamedCallableTargets(result, targets...)
		}
	}
	return result, incomplete
}

func s7ARExpandedCallableExpressionsAtDemandStates(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	states []s7ARCallableDemandState,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
) s7ARCallableSequenceOrigins {
	if len(states) == 0 {
		return s7ARExpandedCallableExpressions(
			model, function, expression, aliases, origins, receiverObjects,
			map[ast.Expr]bool{},
		)
	}
	var result s7ARCallableSequenceOrigins
	var first s7ARCallableSequenceOrigins
	firstSet := false
	for _, state := range states {
		if state.incomplete {
			result.incomplete = true
			continue
		}
		sequence, reliable := s7ARCallableSequenceAtDemandState(
			model, expression, state,
		)
		if !reliable {
			return s7ARExpandedCallableExpressions(
				model, function, expression, aliases, origins, receiverObjects,
				map[ast.Expr]bool{},
			)
		}
		if !firstSet {
			first = s7ARCloneCallableSequenceOrigins(sequence)
			firstSet = true
		} else if !s7ARCallableSequenceOriginsEquivalent(
			model, first, sequence,
		) {
			result.incomplete = true
		}
		result, _ = s7ARMergeCallableSequenceOrigins(result, sequence)
	}
	return result
}

func s7ARCallableSequenceOriginsEquivalent(
	model *s6SourceTypeModel,
	left s7ARCallableSequenceOrigins,
	right s7ARCallableSequenceOrigins,
) bool {
	if left.incomplete != right.incomplete ||
		len(left.alternatives) != len(right.alternatives) ||
		len(left.uncertainIndices) != len(right.uncertainIndices) {
		return false
	}
	for index := range left.uncertainIndices {
		if !right.uncertainIndices[index] {
			return false
		}
	}
	signatures := func(
		sequence s7ARCallableSequenceOrigins,
	) []string {
		result := make([]string, 0, len(sequence.alternatives))
		for _, alternative := range sequence.alternatives {
			var elements []string
			for _, origin := range alternative {
				expression := s7ARUnwrapCallExpression(origin.expression)
				if identifier, ok := expression.(*ast.Ident); ok {
					elements = append(
						elements,
						fmt.Sprintf(
							"%p:%p",
							origin.function,
							s7ARCallableDemandObject(model, identifier),
						),
					)
					continue
				}
				elements = append(
					elements,
					fmt.Sprintf(
						"%p:%T:%d:%s",
						origin.function, expression,
						expression.Pos(), s7ARNodeString(expression),
					),
				)
			}
			result = append(result, strings.Join(elements, "|"))
		}
		sort.Strings(result)
		return result
	}
	leftSignatures := signatures(left)
	rightSignatures := signatures(right)
	if len(leftSignatures) != len(rightSignatures) {
		return false
	}
	for index := range leftSignatures {
		if leftSignatures[index] != rightSignatures[index] {
			return false
		}
	}
	return true
}

func s7ARCallableSequenceAtDemandState(
	model *s6SourceTypeModel,
	expression ast.Expr,
	state s7ARCallableDemandState,
) (s7ARCallableSequenceOrigins, bool) {
	if state.incomplete {
		return s7ARCallableSequenceOrigins{incomplete: true}, true
	}
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := s7ARCallableDemandObject(model, typed)
		if !state.sequenceReliable[object] {
			return s7ARCallableSequenceOrigins{}, false
		}
		return s7ARCloneCallableSequenceOrigins(
			state.sequenceOrigins[object],
		), true
	case *ast.SliceExpr:
		source, reliable := s7ARCallableSequenceAtDemandState(
			model, typed.X, state,
		)
		if !reliable {
			return s7ARCallableSequenceOrigins{}, false
		}
		return s7ARSliceCallableSequence(
			model, source, typed.Low, typed.High,
		), true
	default:
		return s7ARCallableSequenceOrigins{}, false
	}
}

func s7ARCallableDemandViewForExpression(
	model *s6SourceTypeModel,
	expression ast.Expr,
	views map[types.Object]s7ARCallableDemandView,
) s7ARCallableDemandView {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		return views[object]
	case *ast.SliceExpr:
		base := s7ARCallableDemandViewForExpression(model, typed.X, views)
		if !base.any() {
			return s7ARCallableDemandView{}
		}
		if base.all {
			return base
		}
		offset := 0
		if typed.Low != nil {
			resolved, exact := s7ARConstantIndex(model, typed.Low)
			if !exact {
				return s7ARCallableDemandView{all: true}
			}
			offset = resolved
		}
		result := s7ARCallableDemandView{offsets: map[int]bool{}}
		for candidate := range base.offsets {
			result.offsets[candidate+offset] = true
		}
		return result
	case *ast.CallExpr:
		if s6CallName(typed) != "append" || len(typed.Args) == 0 {
			return s7ARCallableDemandView{}
		}
		base := s7ARCallableDemandViewForExpression(model, typed.Args[0], views)
		if base.any() {
			return s7ARCallableDemandView{all: true}
		}
		if typed.Ellipsis != token.NoPos && len(typed.Args) == 2 {
			tail := s7ARCallableDemandViewForExpression(model, typed.Args[1], views)
			if tail.any() {
				return s7ARCallableDemandView{all: true}
			}
		}
		return s7ARCallableDemandView{}
	default:
		return s7ARCallableDemandView{}
	}
}

func s7ARCallableElementDemandForExpression(
	model *s6SourceTypeModel,
	expression ast.Expr,
	views map[types.Object]s7ARCallableDemandView,
	elements map[types.Object]s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		return elements[object]
	case *ast.IndexExpr:
		view := s7ARCallableDemandViewForExpression(model, typed.X, views)
		if !view.any() {
			return s7ARCallableInvocationDemand{}
		}
		index, exact := s7ARConstantIndex(model, typed.Index)
		if !exact || view.all {
			return s7ARCallableInvocationDemand{all: true}
		}
		result := s7ARCallableInvocationDemand{}
		for offset := range view.offsets {
			result.addIndex(offset + index)
		}
		return result
	default:
		return s7ARCallableInvocationDemand{}
	}
}

func s7ARMapCallableDemandThroughView(
	demand s7ARCallableInvocationDemand,
	view s7ARCallableDemandView,
) s7ARCallableInvocationDemand {
	if !demand.any() || !view.any() {
		return s7ARCallableInvocationDemand{}
	}
	if demand.all || view.all {
		return s7ARCallableInvocationDemand{all: true}
	}
	result := s7ARCallableInvocationDemand{}
	for offset := range view.offsets {
		for index := range demand.indices {
			result.addIndex(offset + index)
		}
	}
	return result
}

func s7ARCallableForwardedDemand(
	model *s6SourceTypeModel,
	owner *ast.FuncDecl,
	call *ast.CallExpr,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	resolving map[*ast.BlockStmt]bool,
	cache map[*ast.BlockStmt]s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	result := s7ARCallableInvocationDemand{}
	literalTargets, literalIncomplete := s7ARFunctionLiteralExpressions(
		model, owner, call.Fun, aliases, origins, receiverObjects,
		map[ast.Expr]bool{},
	)
	namedTargets, namedIncomplete := s7ARNamedCallableTargets(
		model, owner, call.Fun, aliases, origins, receiverObjects,
		namedFunctions, map[ast.Expr]bool{},
		s7ARCallableResolutionContext{arguments: call.Args, invoked: true},
	)
	resolved := false
	for literal := range literalTargets {
		parameters, variadic := s7ARCallableParameters(literal.Type.Params)
		if !variadic || len(parameters) == 0 ||
			!s7ARVariadicParameterCarriesCallable(
				model, parameters[len(parameters)-1],
			) {
			continue
		}
		resolved = true
		result.merge(s7ARCallableVariadicParameterDemand(
			model, owner, literal.Body, parameters[len(parameters)-1],
			aliases, origins, receiverObjects, namedFunctions, resolving, cache,
		))
	}
	for _, target := range namedTargets {
		parameters, variadic := s7ARCallableParameters(
			target.declaration.Type.Params,
		)
		if !variadic || len(parameters) == 0 ||
			!s7ARVariadicParameterCarriesCallable(
				model, parameters[len(parameters)-1],
			) {
			continue
		}
		resolved = true
		result.merge(s7ARCallableVariadicParameterDemand(
			model, target.declaration, target.declaration.Body,
			parameters[len(parameters)-1],
			aliases, origins, receiverObjects, namedFunctions, resolving, cache,
		))
	}
	if literalIncomplete || namedIncomplete ||
		!resolved && s7ARFunctionValuedExpression(model, call.Fun) {
		result.addAll()
	}
	return result
}

func s7ARCallableForwardedDemandAtState(
	model *s6SourceTypeModel,
	owner *ast.FuncDecl,
	call *ast.CallExpr,
	state s7ARCallableDemandState,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	resolving map[*ast.BlockStmt]bool,
	cache map[*ast.BlockStmt]s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	result := s7ARCallableInvocationDemand{}
	states := []s7ARCallableDemandState{state}
	literalTargets, literalIncomplete :=
		s7ARFunctionLiteralExpressionsAtDemandStates(
			model, owner, call.Fun, states, aliases, origins, receiverObjects,
		)
	namedTargets, namedIncomplete := s7ARNamedCallableTargetsAtDemandStates(
		model, owner, call.Fun, states, aliases, origins, receiverObjects,
		namedFunctions,
		s7ARCallableResolutionContext{arguments: call.Args, invoked: true},
	)
	resolved := false
	for literal := range literalTargets {
		parameters, variadic := s7ARCallableParameters(literal.Type.Params)
		if !variadic || len(parameters) == 0 ||
			!s7ARVariadicParameterCarriesCallable(
				model, parameters[len(parameters)-1],
			) {
			continue
		}
		resolved = true
		result.merge(s7ARCallableVariadicParameterDemand(
			model, owner, literal.Body, parameters[len(parameters)-1],
			aliases, origins, receiverObjects, namedFunctions, resolving, cache,
		))
	}
	for _, target := range namedTargets {
		parameters, variadic := s7ARCallableParameters(
			target.declaration.Type.Params,
		)
		if !variadic || len(parameters) == 0 ||
			!s7ARVariadicParameterCarriesCallable(
				model, parameters[len(parameters)-1],
			) {
			continue
		}
		resolved = true
		result.merge(s7ARCallableVariadicParameterDemand(
			model, target.declaration, target.declaration.Body,
			parameters[len(parameters)-1],
			aliases, origins, receiverObjects, namedFunctions, resolving, cache,
		))
	}
	if literalIncomplete || namedIncomplete ||
		!resolved && s7ARFunctionValuedExpression(model, call.Fun) {
		result.addAll()
	}
	return result
}

func s7ARVariadicParameterCarriesCallable(
	model *s6SourceTypeModel,
	parameter *ast.Ident,
) bool {
	if parameter == nil {
		return false
	}
	object := model.definitions[parameter]
	if object == nil || object.Type() == nil {
		return false
	}
	slice, ok := types.Unalias(object.Type()).Underlying().(*types.Slice)
	return ok && s7ARFunctionValuedType(slice.Elem())
}

func s7ARExpandedCallableExpressions(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	resolving map[ast.Expr]bool,
) s7ARCallableSequenceOrigins {
	expression = s7ARUnwrapCallExpression(expression)
	if expression == nil || resolving[expression] ||
		!s7ARCallableSequenceExpression(model, expression) {
		return s7ARCallableSequenceOrigins{incomplete: true}
	}
	resolving[expression] = true
	defer delete(resolving, expression)

	switch typed := expression.(type) {
	case *ast.CompositeLit:
		return s7ARCallableCompositeSequence(model, function, typed)
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		if function != nil {
			sequence := s7ARCallableSequenceStateBefore(
				model, function, object, typed.Pos(), aliases, origins,
				receiverObjects, resolving,
			)
			if len(sequence.alternatives) != 0 || sequence.incomplete {
				return sequence
			}
		}
		if sequence, ok := origins.sequences[object]; ok {
			return s7ARCloneCallableSequenceOrigins(sequence)
		}
		var candidates []ast.Expr
		candidates = append(candidates, origins.values[object]...)
		if s7ARPackageScopeObject(object) {
			candidates = append(candidates, s7ARObjectInitializers(model, object)...)
		}
		candidates = s7ARUniqueExpressions(candidates)
		if len(candidates) == 0 {
			return s7ARCallableSequenceOrigins{incomplete: true}
		}
		var result s7ARCallableSequenceOrigins
		for _, candidate := range candidates {
			values := s7ARExpandedCallableExpressions(
				model, function, candidate, aliases, origins,
				receiverObjects, resolving,
			)
			result, _ = s7ARMergeCallableSequenceOrigins(result, values)
		}
		if s7ARPackageScopeObject(object) {
			result.incomplete = true
		}
		return result
	case *ast.SliceExpr:
		base := s7ARExpandedCallableExpressions(
			model, function, typed.X, aliases, origins, receiverObjects, resolving,
		)
		return s7ARSliceCallableSequence(model, base, typed.Low, typed.High)
	case *ast.SelectorExpr:
		selection := model.selections[typed]
		if selection == nil || selection.Kind() != types.FieldVal {
			return s7ARCallableSequenceOrigins{incomplete: true}
		}
		candidates, incomplete := s7ARCallableFieldExpressions(
			model, function, typed.X, selection.Index(), typed.Sel.Name,
			origins, receiverObjects, map[ast.Expr]bool{}, true,
		)
		result := s7ARCallableSequenceOrigins{incomplete: incomplete}
		for _, candidate := range candidates {
			values := s7ARExpandedCallableExpressions(
				model, function, candidate, aliases, origins,
				receiverObjects, resolving,
			)
			result, _ = s7ARMergeCallableSequenceOrigins(result, values)
		}
		result.incomplete = result.incomplete || len(candidates) == 0
		return result
	case *ast.CallExpr:
		if len(typed.Args) == 1 {
			if value, ok := model.expressionTypes[typed.Fun]; ok && value.IsType() {
				return s7ARExpandedCallableExpressions(
					model, function, typed.Args[0], aliases, origins,
					receiverObjects, resolving,
				)
			}
		}
		if s6CallName(typed) == "append" && len(typed.Args) != 0 {
			result := s7ARExpandedCallableExpressions(
				model, function, typed.Args[0], aliases, origins,
				receiverObjects, resolving,
			)
			result.incomplete = true
			if typed.Ellipsis != token.NoPos && len(typed.Args) == 2 {
				tail := s7ARExpandedCallableExpressions(
					model, function, typed.Args[1], aliases, origins,
					receiverObjects, resolving,
				)
				return s7ARAppendCallableSequenceOrigins(result, tail)
			}
			for _, argument := range typed.Args[1:] {
				result = s7ARAppendCallableExpressionOrigin(
					result,
					s7ARCallableExpressionOrigin{
						function: function, expression: argument,
					},
				)
			}
			return result
		}
		var factories []*ast.FuncLit
		if literal, ok := s7ARUnwrapCallExpression(typed.Fun).(*ast.FuncLit); ok {
			factories = append(factories, literal)
		} else {
			resolved, _ := s7ARFunctionLiteralExpressions(
				model, function, typed.Fun, aliases, origins,
				receiverObjects, map[ast.Expr]bool{},
			)
			for literal := range resolved {
				factories = append(factories, literal)
			}
		}
		var result s7ARCallableSequenceOrigins
		for _, factory := range factories {
			returns := s7ARCallableSequenceReturnExpressions(
				model, factory.Type, factory.Body,
			)
			if len(returns) == 0 {
				result.incomplete = true
			}
			for _, returned := range returns {
				values := s7ARExpandedCallableExpressions(
					model, function, returned, aliases, origins,
					receiverObjects, resolving,
				)
				result, _ = s7ARMergeCallableSequenceOrigins(result, values)
			}
		}
		if declaration := s7ARNamedFunctionDeclaration(model, typed.Fun); declaration != nil {
			returns := s7ARCallableSequenceReturnExpressions(
				model, declaration.Type, declaration.Body,
			)
			if len(returns) == 0 {
				result.incomplete = true
			}
			for _, returned := range returns {
				values := s7ARExpandedCallableExpressions(
					model, declaration, returned, aliases, origins,
					receiverObjects, resolving,
				)
				result, _ = s7ARMergeCallableSequenceOrigins(result, values)
			}
		}
		if len(factories) == 0 &&
			s7ARNamedFunctionDeclaration(model, typed.Fun) == nil {
			result.incomplete = true
		}
		return result
	default:
		return s7ARCallableSequenceOrigins{incomplete: true}
	}
}

func s7ARCloneCallableSequenceOrigins(
	source s7ARCallableSequenceOrigins,
) s7ARCallableSequenceOrigins {
	result := s7ARCallableSequenceOrigins{
		incomplete:       source.incomplete,
		uncertainIndices: map[int]bool{},
	}
	for index := range source.uncertainIndices {
		result.uncertainIndices[index] = true
	}
	for _, alternative := range source.alternatives {
		result.alternatives = append(
			result.alternatives,
			append(s7ARCallableSequenceAlternative(nil), alternative...),
		)
	}
	return result
}

func s7ARMergeCallableSequenceOrigins(
	target s7ARCallableSequenceOrigins,
	source s7ARCallableSequenceOrigins,
) (s7ARCallableSequenceOrigins, bool) {
	result := s7ARCloneCallableSequenceOrigins(target)
	changed := false
	if source.incomplete && !result.incomplete {
		result.incomplete = true
		changed = true
	}
	if result.uncertainIndices == nil {
		result.uncertainIndices = map[int]bool{}
	}
	for index := range source.uncertainIndices {
		if !result.uncertainIndices[index] {
			result.uncertainIndices[index] = true
			changed = true
		}
	}
	for _, candidate := range source.alternatives {
		found := false
		for _, existing := range result.alternatives {
			if s7ARCallableSequenceAlternativeEqual(existing, candidate) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if len(result.alternatives) >= 64 {
			if !result.incomplete {
				result.incomplete = true
				changed = true
			}
			continue
		}
		result.alternatives = append(
			result.alternatives,
			append(s7ARCallableSequenceAlternative(nil), candidate...),
		)
		changed = true
	}
	return result, changed
}

func s7ARCallableSequenceAlternativeEqual(
	left s7ARCallableSequenceAlternative,
	right s7ARCallableSequenceAlternative,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].function != right[index].function ||
			left[index].expression != right[index].expression {
			return false
		}
	}
	return true
}

func s7ARCallableCompositeSequence(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	composite *ast.CompositeLit,
) s7ARCallableSequenceOrigins {
	alternative := s7ARCallableSequenceAlternative{}
	nextIndex := 0
	incomplete := false
	for _, element := range composite.Elts {
		index := nextIndex
		value := element
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			resolved, exact := s7ARConstantIndex(model, keyed.Key)
			if !exact || resolved < 0 {
				incomplete = true
				continue
			}
			index = resolved
			value = keyed.Value
		}
		for len(alternative) <= index {
			alternative = append(alternative, s7ARCallableExpressionOrigin{
				function: function,
			})
		}
		if !s7ARFunctionValuedObjectOrExpression(model, value) {
			incomplete = true
		} else {
			alternative[index] = s7ARCallableExpressionOrigin{
				function: function, expression: value,
			}
		}
		nextIndex = index + 1
	}
	return s7ARCallableSequenceOrigins{
		alternatives: []s7ARCallableSequenceAlternative{alternative},
		incomplete:   incomplete,
	}
}

func s7ARConstantIndex(
	model *s6SourceTypeModel,
	expression ast.Expr,
) (int, bool) {
	value, ok := model.expressionTypes[expression]
	if !ok || value.Value == nil || value.Value.Kind() != constant.Int {
		return 0, false
	}
	resolved, exact := constant.Int64Val(value.Value)
	if !exact || resolved < 0 || resolved > int64(^uint(0)>>1) {
		return 0, false
	}
	return int(resolved), true
}

func s7ARCallableSequenceIndexOrigins(
	model *s6SourceTypeModel,
	sequence s7ARCallableSequenceOrigins,
	index ast.Expr,
) ([]s7ARCallableExpressionOrigin, bool) {
	resolved, constantIndex := s7ARConstantIndex(model, index)
	var result []s7ARCallableExpressionOrigin
	incomplete := sequence.incomplete
	if constantIndex {
		incomplete = incomplete || sequence.uncertainIndices[resolved]
	} else if len(sequence.uncertainIndices) != 0 {
		incomplete = true
	}
	for _, alternative := range sequence.alternatives {
		if constantIndex {
			if resolved >= len(alternative) {
				incomplete = true
				continue
			}
			if alternative[resolved].expression == nil {
				incomplete = true
				continue
			}
			result = s7ARAppendCallableExpressionOrigins(
				result, alternative[resolved],
			)
			continue
		}
		for _, candidate := range alternative {
			if candidate.expression == nil {
				incomplete = true
				continue
			}
			result = s7ARAppendCallableExpressionOrigins(result, candidate)
		}
	}
	if len(sequence.alternatives) == 0 {
		incomplete = true
	}
	return result, incomplete
}

func s7ARCallableDynamicElementOrigins(
	sequence s7ARCallableSequenceOrigins,
) s7ARCallableSequenceOrigins {
	result := s7ARCallableSequenceOrigins{
		incomplete: sequence.incomplete || len(sequence.uncertainIndices) != 0,
	}
	for _, alternative := range sequence.alternatives {
		for _, candidate := range alternative {
			if candidate.expression == nil {
				continue
			}
			result.alternatives = append(
				result.alternatives,
				s7ARCallableSequenceAlternative{candidate},
			)
		}
	}
	return result
}

func s7ARCallableSequenceDerivesFromBoundOrigin(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	origins *s7ARCallableOriginState,
	resolving map[ast.Expr]bool,
) bool {
	expression = s7ARUnwrapCallExpression(expression)
	if expression == nil || resolving[expression] {
		return false
	}
	resolving[expression] = true
	defer delete(resolving, expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		if _, ok := origins.sequences[object]; ok {
			return true
		}
		if function == nil {
			return false
		}
		bindings := s6FunctionBindingsBefore(function, expression.Pos())
		for _, candidate := range bindings.candidates(typed) {
			if s7ARCallableSequenceDerivesFromBoundOrigin(
				model, function, candidate, origins, resolving,
			) {
				return true
			}
		}
		return false
	case *ast.SliceExpr:
		return s7ARCallableSequenceDerivesFromBoundOrigin(
			model, function, typed.X, origins, resolving,
		)
	case *ast.CallExpr:
		return s6CallName(typed) == "append" && len(typed.Args) != 0 &&
			s7ARCallableSequenceDerivesFromBoundOrigin(
				model, function, typed.Args[0], origins, resolving,
			)
	default:
		return false
	}
}

func s7ARAppendCallableExpressionOrigins(
	target []s7ARCallableExpressionOrigin,
	candidates ...s7ARCallableExpressionOrigin,
) []s7ARCallableExpressionOrigin {
	for _, candidate := range candidates {
		if candidate.expression == nil {
			continue
		}
		found := false
		for _, existing := range target {
			if existing.function == candidate.function &&
				existing.expression == candidate.expression {
				found = true
				break
			}
		}
		if !found {
			target = append(target, candidate)
		}
	}
	return target
}

func s7ARAppendCallableExpressionOrigin(
	sequence s7ARCallableSequenceOrigins,
	origin s7ARCallableExpressionOrigin,
) s7ARCallableSequenceOrigins {
	result := s7ARCloneCallableSequenceOrigins(sequence)
	if len(result.alternatives) == 0 {
		result.alternatives = append(
			result.alternatives, s7ARCallableSequenceAlternative{},
		)
	}
	for index := range result.alternatives {
		result.alternatives[index] = append(result.alternatives[index], origin)
	}
	return result
}

func s7ARAppendCallableSequenceOrigins(
	left s7ARCallableSequenceOrigins,
	right s7ARCallableSequenceOrigins,
) s7ARCallableSequenceOrigins {
	result := s7ARCallableSequenceOrigins{
		incomplete: left.incomplete || right.incomplete,
	}
	result.uncertainIndices = map[int]bool{}
	for index := range left.uncertainIndices {
		result.uncertainIndices[index] = true
	}
	leftLength := -1
	for _, alternative := range left.alternatives {
		if leftLength < 0 {
			leftLength = len(alternative)
		} else if leftLength != len(alternative) {
			result.incomplete = true
		}
	}
	if leftLength >= 0 {
		for index := range right.uncertainIndices {
			result.uncertainIndices[leftLength+index] = true
		}
	} else if len(right.uncertainIndices) != 0 {
		result.incomplete = true
	}
	if len(left.alternatives) == 0 || len(right.alternatives) == 0 {
		result.incomplete = true
		return result
	}
	for _, leftAlternative := range left.alternatives {
		for _, rightAlternative := range right.alternatives {
			if len(result.alternatives) >= 64 {
				result.incomplete = true
				return result
			}
			combined := append(
				s7ARCallableSequenceAlternative(nil), leftAlternative...,
			)
			combined = append(combined, rightAlternative...)
			result.alternatives = append(result.alternatives, combined)
		}
	}
	return result
}

func s7ARSliceCallableSequence(
	model *s6SourceTypeModel,
	source s7ARCallableSequenceOrigins,
	low ast.Expr,
	high ast.Expr,
) s7ARCallableSequenceOrigins {
	result := s7ARCallableSequenceOrigins{
		incomplete:       source.incomplete,
		uncertainIndices: map[int]bool{},
	}
	sharedStart := -1
	sharedEnd := -1
	for _, alternative := range source.alternatives {
		start := 0
		end := len(alternative)
		var ok bool
		if low != nil {
			start, ok = s7ARConstantIndex(model, low)
			if !ok {
				result.incomplete = true
				continue
			}
		}
		if high != nil {
			end, ok = s7ARConstantIndex(model, high)
			if !ok {
				result.incomplete = true
				continue
			}
		}
		if start < 0 || end < start || end > len(alternative) {
			result.incomplete = true
			continue
		}
		if sharedStart < 0 {
			sharedStart, sharedEnd = start, end
		} else if sharedStart != start || sharedEnd != end {
			result.incomplete = true
		}
		result.alternatives = append(
			result.alternatives,
			append(s7ARCallableSequenceAlternative(nil), alternative[start:end]...),
		)
	}
	if len(source.alternatives) == 0 {
		result.incomplete = true
	}
	if sharedStart >= 0 {
		for index := range source.uncertainIndices {
			if index >= sharedStart && index < sharedEnd {
				result.uncertainIndices[index-sharedStart] = true
			}
		}
	}
	return result
}

func s7ARCallableSequenceStateBefore(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	target types.Object,
	before token.Pos,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	resolving map[ast.Expr]bool,
) s7ARCallableSequenceOrigins {
	if function == nil || function.Body == nil || target == nil {
		return s7ARCallableSequenceOrigins{}
	}
	states := map[types.Object]*s7ARCallableSequenceOrigins{}
	backings := map[types.Object]s7ARCallableSliceBacking{}
	for object, sequence := range origins.sequences {
		cloned := s7ARCloneCallableSequenceOrigins(sequence)
		states[object] = &cloned
	}
	parents := s6ASTParents(function.Body)
	resolve := func(expression ast.Expr) s7ARCallableSequenceOrigins {
		return s7ARCallableSequenceFromLocalExpression(
			model, function, expression, states, aliases, origins,
			receiverObjects, resolving,
		)
	}
	assign := func(
		object types.Object,
		source *s7ARCallableSequenceOrigins,
		backing *s7ARCallableSliceBacking,
		conditional bool,
	) {
		if object == nil || source == nil {
			return
		}
		if !conditional {
			states[object] = source
			if backing == nil {
				delete(backings, object)
			} else {
				backings[object] = *backing
			}
			return
		}
		merged := s7ARCallableSequenceOrigins{incomplete: true}
		if current := states[object]; current != nil {
			merged, _ = s7ARMergeCallableSequenceOrigins(merged, *current)
		}
		merged, _ = s7ARMergeCallableSequenceOrigins(merged, *source)
		states[object] = &merged
		delete(backings, object)
	}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if node == nil || node.Pos() >= before {
			return false
		}
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		conditional := s7ARCallableMutationConditional(node, before, parents)
		switch typed := node.(type) {
		case *ast.ValueSpec:
			for index, identifier := range typed.Names {
				object := model.definitions[identifier]
				if !s7ARCallableSequenceObject(object) {
					continue
				}
				expression, ok := s7ARAssignmentRight(typed.Values, index)
				if !ok {
					continue
				}
				if source := s7ARCallableSequenceAliasState(
					model, expression, states,
				); source != nil {
					backing := s7ARCallableAliasBacking(
						model, expression, backings,
					)
					assign(object, source, backing, conditional)
					continue
				}
				s7ARApplyCallableAppendBackingMutation(
					model, expression, states, backings,
				)
				state := resolve(expression)
				backing := s7ARCallableResliceBacking(
					model, expression, states, backings,
				)
				assign(object, &state, backing, conditional)
			}
			return false
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				expression, ok := s7ARAssignmentRight(typed.Rhs, index)
				if !ok {
					continue
				}
				if indexed, ok := s7ARUnwrapCallExpression(left).(*ast.IndexExpr); ok {
					s7ARApplyCallableIndexMutation(
						model, function, indexed, expression, states,
						conditional,
					)
					s7ARPropagateCallableBackingIndexMutation(
						model, indexed, backings,
					)
					continue
				}
				identifier, ok := s7ARUnwrapCallExpression(left).(*ast.Ident)
				if !ok {
					continue
				}
				object := model.uses[identifier]
				if object == nil {
					object = model.definitions[identifier]
				}
				if !s7ARCallableSequenceObject(object) {
					continue
				}
				if source := s7ARCallableSequenceAliasState(
					model, expression, states,
				); source != nil {
					backing := s7ARCallableAliasBacking(
						model, expression, backings,
					)
					assign(object, source, backing, conditional)
					continue
				}
				s7ARApplyCallableAppendBackingMutation(
					model, expression, states, backings,
				)
				state := resolve(expression)
				backing := s7ARCallableResliceBacking(
					model, expression, states, backings,
				)
				assign(object, &state, backing, conditional)
			}
			return false
		case *ast.CallExpr:
			if typed.End() >= before {
				return true
			}
			if s6CallName(typed) == "copy" && len(typed.Args) == 2 {
				s7ARApplyCallableCopyMutation(
					model, function, typed.Args[0], typed.Args[1],
					states, resolve, conditional,
				)
				return true
			}
			s7ARApplyCallableCallMutations(
				model, typed, states, backings,
			)
		}
		return true
	})
	if state := states[target]; state != nil {
		return s7ARCloneCallableSequenceOrigins(*state)
	}
	return s7ARCallableSequenceOrigins{}
}

func s7ARCallableSequenceObject(object types.Object) bool {
	if object == nil || object.Type() == nil {
		return false
	}
	value := types.Unalias(object.Type()).Underlying()
	switch sequence := value.(type) {
	case *types.Slice:
		return s7ARFunctionValuedType(sequence.Elem())
	case *types.Array:
		return s7ARFunctionValuedType(sequence.Elem())
	default:
		return false
	}
}

func s7ARCallableSequenceAliasState(
	model *s6SourceTypeModel,
	expression ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
) *s7ARCallableSequenceOrigins {
	expression = s7ARUnwrapCallExpression(expression)
	if identifier, ok := expression.(*ast.Ident); ok {
		object := model.uses[identifier]
		if object == nil {
			object = model.definitions[identifier]
		}
		return states[object]
	}
	if sliced, ok := expression.(*ast.SliceExpr); ok &&
		sliced.Low == nil && sliced.High == nil && sliced.Max == nil {
		return s7ARCallableSequenceAliasState(model, sliced.X, states)
	}
	return nil
}

type s7ARCallableSliceBacking struct {
	state  *s7ARCallableSequenceOrigins
	offset int
	exact  bool
}

func s7ARCallableAliasBacking(
	model *s6SourceTypeModel,
	expression ast.Expr,
	backings map[types.Object]s7ARCallableSliceBacking,
) *s7ARCallableSliceBacking {
	expression = s7ARUnwrapCallExpression(expression)
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		sliced, slicedOK := expression.(*ast.SliceExpr)
		if !slicedOK || sliced.Low != nil || sliced.High != nil ||
			sliced.Max != nil {
			return nil
		}
		identifier, _ = s7ARUnwrapCallExpression(sliced.X).(*ast.Ident)
	}
	if identifier == nil {
		return nil
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	backing, ok := backings[object]
	if !ok {
		return nil
	}
	return &backing
}

func s7ARCallableResliceBacking(
	model *s6SourceTypeModel,
	expression ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	backings map[types.Object]s7ARCallableSliceBacking,
) *s7ARCallableSliceBacking {
	sliced, ok := s7ARUnwrapCallExpression(expression).(*ast.SliceExpr)
	if !ok || sliced.Low == nil && sliced.High == nil && sliced.Max == nil {
		return nil
	}
	identifier, ok := s7ARUnwrapCallExpression(sliced.X).(*ast.Ident)
	if !ok {
		return nil
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	state := states[object]
	if state == nil {
		return nil
	}
	backing := s7ARCallableSliceBacking{state: state, exact: true}
	if existing, ok := backings[object]; ok {
		backing = existing
	}
	if sliced.Low != nil {
		offset, exact := s7ARConstantIndex(model, sliced.Low)
		if !exact || !backing.exact {
			backing.exact = false
		} else {
			backing.offset += offset
		}
	}
	return &backing
}

func s7ARPropagateCallableBackingIndexMutation(
	model *s6SourceTypeModel,
	indexed *ast.IndexExpr,
	backings map[types.Object]s7ARCallableSliceBacking,
) {
	identifier, ok := s7ARUnwrapCallExpression(indexed.X).(*ast.Ident)
	if !ok {
		return
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	backing, ok := backings[object]
	if !ok || backing.state == nil {
		return
	}
	index, exact := s7ARConstantIndex(model, indexed.Index)
	if !exact || !backing.exact {
		backing.state.incomplete = true
		return
	}
	s7ARMarkCallableSequenceIndexUncertain(
		backing.state, backing.offset+index,
	)
}

func s7ARMarkCallableSequenceIndexUncertain(
	state *s7ARCallableSequenceOrigins,
	index int,
) {
	if state == nil || index < 0 {
		if state != nil {
			state.incomplete = true
		}
		return
	}
	if state.uncertainIndices == nil {
		state.uncertainIndices = map[int]bool{}
	}
	state.uncertainIndices[index] = true
}

func s7ARMarkCallableSequenceDemandUncertain(
	state *s7ARCallableSequenceOrigins,
	demand s7ARCallableInvocationDemand,
) {
	if state == nil || !demand.any() {
		return
	}
	if demand.all {
		maximum := 0
		for _, alternative := range state.alternatives {
			if len(alternative) > maximum {
				maximum = len(alternative)
			}
		}
		if maximum == 0 {
			state.incomplete = true
			return
		}
		for index := 0; index < maximum; index++ {
			s7ARMarkCallableSequenceIndexUncertain(state, index)
		}
		return
	}
	for index := range demand.indices {
		s7ARMarkCallableSequenceIndexUncertain(state, index)
	}
}

type s7ARCallableBackingView struct {
	state         *s7ARCallableSequenceOrigins
	offset        int
	length        int
	lengthExact   bool
	capacity      int
	capacityExact bool
	exact         bool
}

func s7ARCallableSequenceLength(
	state *s7ARCallableSequenceOrigins,
) (int, bool) {
	if state == nil || len(state.alternatives) == 0 {
		return 0, false
	}
	length := len(state.alternatives[0])
	for _, alternative := range state.alternatives[1:] {
		if len(alternative) != length {
			return 0, false
		}
	}
	return length, true
}

func s7ARCallableBackingViewForExpression(
	model *s6SourceTypeModel,
	expression ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	backings map[types.Object]s7ARCallableSliceBacking,
) s7ARCallableBackingView {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.Ident:
		object := model.uses[typed]
		if object == nil {
			object = model.definitions[typed]
		}
		state := states[object]
		if state == nil {
			return s7ARCallableBackingView{}
		}
		length, lengthExact := s7ARCallableSequenceLength(state)
		result := s7ARCallableBackingView{
			state:       state,
			length:      length,
			lengthExact: lengthExact,
			exact:       true,
		}
		if backing, ok := backings[object]; ok {
			result.state = backing.state
			result.offset = backing.offset
			result.exact = backing.exact
		}
		return result
	case *ast.SliceExpr:
		result := s7ARCallableBackingViewForExpression(
			model, typed.X, states, backings,
		)
		if result.state == nil {
			return result
		}
		low := 0
		if typed.Low != nil {
			resolved, exact := s7ARConstantIndex(model, typed.Low)
			if !exact {
				result.exact = false
				result.lengthExact = false
				return result
			}
			low = resolved
		}
		high := result.length
		if typed.High != nil {
			resolved, exact := s7ARConstantIndex(model, typed.High)
			if !exact {
				result.exact = false
				result.lengthExact = false
				return result
			}
			high = resolved
		} else if !result.lengthExact {
			result.exact = false
		}
		if low < 0 || high < low {
			result.exact = false
			result.lengthExact = false
			return result
		}
		result.offset += low
		result.length = high - low
		result.lengthExact = result.lengthExact || typed.High != nil
		if typed.Max != nil {
			maximum, exact := s7ARConstantIndex(model, typed.Max)
			if !exact || maximum < high {
				result.exact = false
			} else {
				result.capacity = maximum - low
				result.capacityExact = true
			}
		}
		return result
	default:
		return s7ARCallableBackingView{}
	}
}

func s7ARApplyCallableAppendBackingMutation(
	model *s6SourceTypeModel,
	expression ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	backings map[types.Object]s7ARCallableSliceBacking,
) {
	call, ok := s7ARUnwrapCallExpression(expression).(*ast.CallExpr)
	if !ok || s6CallName(call) != "append" || len(call.Args) < 2 {
		return
	}
	view := s7ARCallableBackingViewForExpression(
		model, call.Args[0], states, backings,
	)
	if view.state == nil {
		return
	}
	if !view.exact || !view.lengthExact {
		view.state.incomplete = true
		return
	}
	count := len(call.Args) - 1
	countExact := call.Ellipsis == token.NoPos
	if !countExact {
		tail := s7ARCallableBackingViewForExpression(
			model, call.Args[len(call.Args)-1], states, backings,
		)
		if tail.lengthExact {
			count = tail.length
			countExact = true
		}
	}
	if view.capacityExact && countExact &&
		view.length+count > view.capacity {
		return
	}
	rootLength, rootLengthExact := s7ARCallableSequenceLength(view.state)
	if !rootLengthExact || !countExact {
		view.state.incomplete = true
		return
	}
	for index := 0; index < count; index++ {
		rootIndex := view.offset + view.length + index
		if rootIndex >= rootLength {
			continue
		}
		s7ARMarkCallableSequenceIndexUncertain(view.state, rootIndex)
	}
}

func s7ARApplyCallableCallMutations(
	model *s6SourceTypeModel,
	call *ast.CallExpr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	backings map[types.Object]s7ARCallableSliceBacking,
) {
	if call == nil || s6CallName(call) == "append" ||
		s6CallName(call) == "copy" ||
		s6CallName(call) == "len" || s6CallName(call) == "cap" {
		return
	}
	if value, ok := model.expressionTypes[call.Fun]; ok && value.IsType() {
		return
	}
	declaration := s7ARNamedFunctionDeclaration(model, call.Fun)
	if declaration == nil || declaration.Body == nil {
		for _, argument := range call.Args {
			view := s7ARCallableBackingViewForExpression(
				model, argument, states, backings,
			)
			if view.state != nil {
				s7ARMarkCallableSequenceDemandUncertain(
					view.state, s7ARCallableInvocationDemand{all: true},
				)
			}
		}
		return
	}
	parameters, _ := s7ARCallableParameters(declaration.Type.Params)
	argumentOffset := 0
	if declaration.Recv != nil {
		if selector, ok := s7ARUnwrapCallExpression(call.Fun).(*ast.SelectorExpr); ok {
			if selection := model.selections[selector]; selection != nil &&
				selection.Kind() == types.MethodExpr {
				argumentOffset = 1
			}
		}
	}
	resolving := map[types.Object]bool{}
	cache := map[types.Object]s7ARCallableInvocationDemand{}
	for index, parameter := range parameters {
		if parameter == nil || index+argumentOffset >= len(call.Args) {
			continue
		}
		object := model.definitions[parameter]
		if !s7ARCallableSequenceObject(object) {
			continue
		}
		demand := s7ARCallableParameterMutationDemand(
			model, declaration, object, resolving, cache,
		)
		if !demand.any() {
			continue
		}
		view := s7ARCallableBackingViewForExpression(
			model, call.Args[index+argumentOffset], states, backings,
		)
		if view.state == nil {
			continue
		}
		if !view.exact {
			view.state.incomplete = true
			continue
		}
		mapped := s7ARCallableInvocationDemand{}
		if demand.all {
			mapped.addAll()
		} else {
			for element := range demand.indices {
				mapped.addIndex(view.offset + element)
			}
		}
		s7ARMarkCallableSequenceDemandUncertain(view.state, mapped)
	}
}

func s7ARCallableParameterMutationDemand(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	parameter types.Object,
	resolving map[types.Object]bool,
	cache map[types.Object]s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	if cached, ok := cache[parameter]; ok {
		return cached
	}
	if function == nil || function.Body == nil || parameter == nil ||
		resolving[parameter] {
		return s7ARCallableInvocationDemand{all: true}
	}
	resolving[parameter] = true
	defer delete(resolving, parameter)
	result := s7ARCallableInvocationDemand{}
	ignoredInvocations := s7ARCallableInvocationDemand{}
	analysis := &s7ARCallableDemandAnalysis{
		model:             model,
		owner:             function,
		demand:            &ignoredInvocations,
		invocation:        false,
		mutationDemand:    &result,
		mutationResolving: resolving,
		mutationCache:     cache,
	}
	initial := s7ARCallableDemandState{
		sequenceViews: map[types.Object]s7ARCallableDemandView{
			parameter: {offsets: map[int]bool{0: true}},
		},
		elementDemands:   map[types.Object]s7ARCallableInvocationDemand{},
		sequenceOrigins:  map[types.Object]s7ARCallableSequenceOrigins{},
		sequenceReliable: map[types.Object]bool{},
		scalarOrigins:    map[types.Object][]s7ARCallableExpressionOrigin{},
		scalarReliable:   map[types.Object]bool{},
		scalarSlots:      map[types.Object]bool{},
		sequenceBackings: map[types.Object]s7ARCallableDemandBacking{
			parameter: {
				identity: parameter.Pos(),
				exact:    true,
			},
		},
		backingOverrides:   map[token.Pos]map[int]s7ARCallableInvocationDemand{},
		backingOverrideSet: map[token.Pos]map[int]bool{},
		backingOrigins:     map[token.Pos]map[int]s7ARCallableOriginOverride{},
		backingUncertain:   map[token.Pos]s7ARCallableInvocationDemand{},
		escapedBackings:    map[token.Pos]s7ARCallableInvocationDemand{},
	}
	flow := s7ARWalkCallableDemandBlock(
		analysis, function.Body, []s7ARCallableDemandState{initial},
	)
	s7ARCompleteCallableDemandFlow(analysis, flow)
	cache[parameter] = result
	return result
}

func s7ARCallableParameterAsyncEscapeDemand(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	parameter types.Object,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	namedFunctions map[*types.Func]*ast.FuncDecl,
	resolving map[types.Object]bool,
	cache map[types.Object]s7ARCallableInvocationDemand,
) s7ARCallableInvocationDemand {
	if cache == nil {
		cache = map[types.Object]s7ARCallableInvocationDemand{}
	}
	if resolving == nil {
		resolving = map[types.Object]bool{}
	}
	if cached, ok := cache[parameter]; ok {
		return cached
	}
	if function == nil || function.Body == nil || parameter == nil ||
		resolving[parameter] {
		return s7ARCallableInvocationDemand{all: true}
	}
	resolving[parameter] = true
	defer delete(resolving, parameter)
	result := s7ARCallableInvocationDemand{}
	ignoredInvocations := s7ARCallableInvocationDemand{}
	analysis := &s7ARCallableDemandAnalysis{
		model:                model,
		owner:                function,
		aliases:              aliases,
		origins:              origins,
		receiverObjects:      receiverObjects,
		namedFunctions:       namedFunctions,
		resolving:            map[*ast.BlockStmt]bool{function.Body: true},
		cache:                map[*ast.BlockStmt]s7ARCallableInvocationDemand{},
		demand:               &ignoredInvocations,
		invocation:           false,
		mutationResolving:    map[types.Object]bool{},
		mutationCache:        map[types.Object]s7ARCallableInvocationDemand{},
		asyncEscapeDemand:    &result,
		asyncEscapeResolving: resolving,
		asyncEscapeCache:     cache,
	}
	initial := s7ARCallableDemandState{
		sequenceViews: map[types.Object]s7ARCallableDemandView{
			parameter: {offsets: map[int]bool{0: true}},
		},
		elementDemands:   map[types.Object]s7ARCallableInvocationDemand{},
		sequenceOrigins:  map[types.Object]s7ARCallableSequenceOrigins{},
		sequenceReliable: map[types.Object]bool{},
		scalarOrigins:    map[types.Object][]s7ARCallableExpressionOrigin{},
		scalarReliable:   map[types.Object]bool{},
		scalarSlots:      map[types.Object]bool{},
		sequenceBackings: map[types.Object]s7ARCallableDemandBacking{
			parameter: {
				identity: parameter.Pos(),
				exact:    true,
			},
		},
		backingOverrides:   map[token.Pos]map[int]s7ARCallableInvocationDemand{},
		backingOverrideSet: map[token.Pos]map[int]bool{},
		backingOrigins:     map[token.Pos]map[int]s7ARCallableOriginOverride{},
		backingUncertain:   map[token.Pos]s7ARCallableInvocationDemand{},
		escapedBackings:    map[token.Pos]s7ARCallableInvocationDemand{},
	}
	flow := s7ARWalkCallableDemandBlock(
		analysis, function.Body, []s7ARCallableDemandState{initial},
	)
	s7ARCompleteCallableDemandFlow(analysis, flow)
	if analysis.overflow || analysis.incomplete {
		result.addAll()
	}
	cache[parameter] = result
	return result
}

func s7ARCallableSequenceFromLocalExpression(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	expression ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	aliases map[types.Object]map[*ast.FuncLit]bool,
	origins *s7ARCallableOriginState,
	receiverObjects map[types.Object]bool,
	resolving map[ast.Expr]bool,
) s7ARCallableSequenceOrigins {
	expression = s7ARUnwrapCallExpression(expression)
	if identifier, ok := expression.(*ast.Ident); ok {
		object := model.uses[identifier]
		if object == nil {
			object = model.definitions[identifier]
		}
		if state := states[object]; state != nil {
			return s7ARCloneCallableSequenceOrigins(*state)
		}
	}
	if composite, ok := expression.(*ast.CompositeLit); ok {
		return s7ARCallableCompositeSequence(model, function, composite)
	}
	if sliced, ok := expression.(*ast.SliceExpr); ok {
		base := s7ARCallableSequenceFromLocalExpression(
			model, function, sliced.X, states, aliases, origins,
			receiverObjects, resolving,
		)
		result := s7ARSliceCallableSequence(
			model, base, sliced.Low, sliced.High,
		)
		return result
	}
	return s7ARExpandedCallableExpressions(
		model, function, expression, aliases, origins,
		receiverObjects, resolving,
	)
}

func s7ARCallableMutationConditional(
	node ast.Node,
	before token.Pos,
	parents map[ast.Node]ast.Node,
) bool {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		switch typed := parent.(type) {
		case *ast.BlockStmt:
			switch parents[parent].(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
				if before < typed.Pos() || before > typed.End() {
					return true
				}
			}
		case *ast.CaseClause:
			if before < typed.Pos() || before > typed.End() {
				return true
			}
		case *ast.CommClause:
			if before < typed.Pos() || before > typed.End() {
				return true
			}
		}
	}
	return false
}

func s7ARApplyCallableIndexMutation(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	indexed *ast.IndexExpr,
	value ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	conditional bool,
) {
	identifier, ok := s7ARUnwrapCallExpression(indexed.X).(*ast.Ident)
	if !ok {
		return
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	state := states[object]
	if state == nil {
		return
	}
	original := s7ARCloneCallableSequenceOrigins(*state)
	updated := s7ARCloneCallableSequenceOrigins(*state)
	origin := s7ARCallableExpressionOrigin{
		function: function, expression: value,
	}
	index, exact := s7ARConstantIndex(model, indexed.Index)
	if exact {
		for alternativeIndex := range updated.alternatives {
			if index >= len(updated.alternatives[alternativeIndex]) {
				updated.incomplete = true
				continue
			}
			updated.alternatives[alternativeIndex][index] = origin
		}
		if !conditional {
			delete(updated.uncertainIndices, index)
		}
	} else {
		updated.incomplete = true
		var alternatives []s7ARCallableSequenceAlternative
		for _, alternative := range updated.alternatives {
			for candidateIndex := range alternative {
				candidate := append(
					s7ARCallableSequenceAlternative(nil), alternative...,
				)
				candidate[candidateIndex] = origin
				alternatives = append(alternatives, candidate)
			}
		}
		updated.alternatives = alternatives
	}
	if conditional {
		if exact {
			if updated.uncertainIndices == nil {
				updated.uncertainIndices = map[int]bool{}
			}
			updated.uncertainIndices[index] = true
		} else {
			updated.incomplete = true
		}
		updated, _ = s7ARMergeCallableSequenceOrigins(updated, original)
	}
	*state = updated
}

func s7ARApplyCallableCopyMutation(
	model *s6SourceTypeModel,
	function *ast.FuncDecl,
	destination ast.Expr,
	source ast.Expr,
	states map[types.Object]*s7ARCallableSequenceOrigins,
	resolve func(ast.Expr) s7ARCallableSequenceOrigins,
	conditional bool,
) {
	identifier, ok := s7ARUnwrapCallExpression(destination).(*ast.Ident)
	if !ok {
		if sliced, slicedOK := s7ARUnwrapCallExpression(destination).(*ast.SliceExpr); slicedOK {
			identifier, _ = s7ARUnwrapCallExpression(sliced.X).(*ast.Ident)
		}
	}
	if identifier == nil {
		return
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	destinationState := states[object]
	if destinationState == nil {
		return
	}
	if _, sliced := s7ARUnwrapCallExpression(destination).(*ast.SliceExpr); sliced {
		destinationState.incomplete = true
		return
	}
	sourceState := resolve(source)
	original := s7ARCloneCallableSequenceOrigins(*destinationState)
	result := s7ARCallableSequenceOrigins{
		incomplete:       destinationState.incomplete || sourceState.incomplete,
		uncertainIndices: map[int]bool{},
	}
	for _, destinationAlternative := range destinationState.alternatives {
		for _, sourceAlternative := range sourceState.alternatives {
			if len(result.alternatives) >= 64 {
				result.incomplete = true
				break
			}
			candidate := append(
				s7ARCallableSequenceAlternative(nil), destinationAlternative...,
			)
			count := len(candidate)
			if len(sourceAlternative) < count {
				count = len(sourceAlternative)
			}
			copy(candidate[:count], sourceAlternative[:count])
			for index := range destinationState.uncertainIndices {
				if index >= count {
					result.uncertainIndices[index] = true
				}
			}
			for index := range sourceState.uncertainIndices {
				if index < count {
					result.uncertainIndices[index] = true
				}
			}
			result.alternatives = append(result.alternatives, candidate)
		}
	}
	if len(result.alternatives) == 0 {
		result.incomplete = true
	}
	if conditional {
		for _, alternative := range result.alternatives {
			for index := range alternative {
				if result.uncertainIndices == nil {
					result.uncertainIndices = map[int]bool{}
				}
				result.uncertainIndices[index] = true
			}
		}
		result, _ = s7ARMergeCallableSequenceOrigins(result, original)
	}
	*destinationState = result
	_ = function
}

func s7ARCallableSequenceExpression(
	model *s6SourceTypeModel,
	expression ast.Expr,
) bool {
	typed, ok := model.expressionTypes[expression]
	if !ok || typed.Type == nil {
		return false
	}
	value := types.Unalias(typed.Type).Underlying()
	switch sequence := value.(type) {
	case *types.Slice:
		return s7ARFunctionValuedType(sequence.Elem())
	case *types.Array:
		return s7ARFunctionValuedType(sequence.Elem())
	default:
		return false
	}
}

func s7ARCallableSequenceReturnExpressions(
	model *s6SourceTypeModel,
	signature *ast.FuncType,
	body *ast.BlockStmt,
) []ast.Expr {
	if body == nil {
		return nil
	}
	var namedResults []ast.Expr
	if signature != nil && signature.Results != nil {
		for _, field := range signature.Results.List {
			for _, identifier := range field.Names {
				object := model.definitions[identifier]
				if object == nil || object.Type() == nil {
					continue
				}
				value := types.Unalias(object.Type()).Underlying()
				slice, sliceOK := value.(*types.Slice)
				array, arrayOK := value.(*types.Array)
				if sliceOK && s7ARFunctionValuedType(slice.Elem()) ||
					arrayOK && s7ARFunctionValuedType(array.Elem()) {
					namedResults = append(namedResults, identifier)
				}
			}
		}
	}
	var result []ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(typed.Results) == 0 {
				result = append(result, namedResults...)
			} else {
				result = append(result, typed.Results...)
			}
			return false
		default:
			return true
		}
	})
	return s7ARUniqueExpressions(result)
}

func s7ARMarkCallableReceiverObjects(
	model *s6SourceTypeModel,
	fields *ast.FieldList,
	receiverObjects map[types.Object]bool,
) bool {
	if fields == nil {
		return false
	}
	changed := false
	for _, field := range fields.List {
		for _, identifier := range field.Names {
			object := model.definitions[identifier]
			if object != nil && !receiverObjects[object] {
				receiverObjects[object] = true
				changed = true
			}
		}
	}
	return changed
}

func s7ARPurgeProgressParameter(function string, index int) string {
	return function + "#" + strconv.Itoa(index)
}

func s7ARUpdatePurgeProgressOrigins(
	model *s6SourceTypeModel,
	canonicalField *types.Var,
	function *s7ARPurgeProgressFunction,
	aliases map[types.Object]s7ARPurgeProgressOrigin,
	left []*ast.Ident,
	right []ast.Expr,
	merge bool,
) {
	for index, identifier := range left {
		if identifier == nil {
			continue
		}
		object := model.definitions[identifier]
		if object == nil {
			object = model.uses[identifier]
		}
		if object == nil {
			continue
		}
		expression, resolved := s7ARAssignmentRight(right, index)
		if !resolved {
			delete(aliases, object)
			continue
		}
		origin := s7ARPurgeProgressExpressionOrigin(
			model, canonicalField, function, aliases, expression,
		)
		if merge {
			origin = s7ARMergePurgeProgressOrigins(aliases[object], origin)
		}
		if !origin.target && !origin.nonTarget &&
			len(origin.parameters) == 0 && !origin.unknown {
			delete(aliases, object)
			continue
		}
		aliases[object] = origin
	}
}

func s7ARPurgeProgressExpressionOrigin(
	model *s6SourceTypeModel,
	canonicalField *types.Var,
	function *s7ARPurgeProgressFunction,
	aliases map[types.Object]s7ARPurgeProgressOrigin,
	expression ast.Expr,
) s7ARPurgeProgressOrigin {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.UnaryExpr:
		if typed.Op == token.AND {
			if s7ARIsPurgeProgressSelector(model, canonicalField, typed.X) {
				return s7ARPurgeProgressOrigin{
					target: true, parameters: map[string]bool{},
				}
			}
			if s7ARIsDefiniteNoncanonicalPurgeProgressSelector(
				model, canonicalField, typed.X,
			) {
				return s7ARPurgeProgressOrigin{
					nonTarget: true, parameters: map[string]bool{},
				}
			}
			return s7ARPurgeProgressExpressionOrigin(
				model, canonicalField, function, aliases, typed.X,
			)
		}
	case *ast.StarExpr:
		return s7ARPurgeProgressExpressionOrigin(
			model, canonicalField, function, aliases, typed.X,
		)
	case *ast.CallExpr:
		if len(typed.Args) == 1 {
			identifier, ok := s7ARUnwrapCallExpression(typed.Fun).(*ast.Ident)
			if ok {
				if _, conversion := model.uses[identifier].(*types.TypeName); conversion {
					return s7ARPurgeProgressExpressionOrigin(
						model, canonicalField, function, aliases, typed.Args[0],
					)
				}
			}
		}
	}
	identifier, ok := expression.(*ast.Ident)
	if ok {
		object := model.uses[identifier]
		if object == nil {
			object = model.definitions[identifier]
		}
		if origin, present := aliases[object]; present {
			return s7ARClonePurgeProgressOrigin(origin)
		}
	}
	typed, present := model.expressionTypes[expression]
	if present && typed.Type != nil && s7ARPurgeProgressPointerCapable(typed.Type) {
		return s7ARPurgeProgressOrigin{parameters: map[string]bool{}, unknown: true}
	}
	return s7ARPurgeProgressOrigin{parameters: map[string]bool{}}
}

func s7ARClonePurgeProgressOrigin(
	origin s7ARPurgeProgressOrigin,
) s7ARPurgeProgressOrigin {
	cloned := s7ARPurgeProgressOrigin{
		target: origin.target, nonTarget: origin.nonTarget,
		unknown: origin.unknown, parameters: map[string]bool{},
	}
	for parameter := range origin.parameters {
		cloned.parameters[parameter] = true
	}
	return cloned
}

func s7ARMergePurgeProgressOrigins(
	left, right s7ARPurgeProgressOrigin,
) s7ARPurgeProgressOrigin {
	result := s7ARPurgeProgressOrigin{
		target:     left.target || right.target,
		nonTarget:  left.nonTarget || right.nonTarget,
		unknown:    left.unknown || right.unknown,
		parameters: map[string]bool{},
	}
	for parameter := range left.parameters {
		result.parameters[parameter] = true
	}
	for parameter := range right.parameters {
		result.parameters[parameter] = true
	}
	return result
}

func s7ARPurgeProgressConditionalAssignment(
	parents map[ast.Node]ast.Node,
	node ast.Node,
	functionBody *ast.BlockStmt,
) bool {
	for parent := parents[node]; parent != nil && parent != functionBody; parent = parents[parent] {
		switch parent.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			return true
		}
	}
	return false
}

func s7ARPurgeProgressPointerCapable(value types.Type) bool {
	value = types.Unalias(value)
	depth := 0
	for {
		pointer, ok := value.(*types.Pointer)
		if !ok {
			break
		}
		depth++
		value = types.Unalias(pointer.Elem())
	}
	named, ok := value.(*types.Named)
	return depth >= 2 && ok &&
		named.Obj().Name() == "intentArchivePurgeProgressReport"
}

func s7ARPurgeProgressLocalCallee(
	model *s6SourceTypeModel,
	call *ast.CallExpr,
) (string, bool) {
	expression := s7ARUnwrapCallExpression(call.Fun)
	var identifier *ast.Ident
	switch typed := expression.(type) {
	case *ast.Ident:
		identifier = typed
	case *ast.SelectorExpr:
		identifier = typed.Sel
	default:
		return "", false
	}
	object, ok := model.uses[identifier].(*types.Func)
	if !ok || object.Pkg() == nil ||
		object.Pkg().Path() != "github.com/tesseracode/tesserapatch/internal/cli" {
		return "", false
	}
	return "internal/cli." + object.Name(), true
}

func s7ARExpressionHasNamedType(
	model *s6SourceTypeModel,
	expression ast.Expr,
	name string,
) bool {
	typed, ok := model.expressionTypes[expression]
	if !ok || typed.Type == nil {
		return false
	}
	return strings.TrimPrefix(s7ARTypeName(types.Unalias(typed.Type)), "*") == name
}

func s7ARPurgeProgressFieldIndex(value types.Type) (int, error) {
	value = types.Unalias(value)
	if pointer, ok := value.(*types.Pointer); ok {
		value = types.Unalias(pointer.Elem())
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj().Name() != "intentArchivePurgeReport" {
		return -1, fmt.Errorf("unexpected report type %s", value)
	}
	structure, ok := named.Underlying().(*types.Struct)
	if !ok {
		return -1, fmt.Errorf("report underlying type is %T", named.Underlying())
	}
	for index := 0; index < structure.NumFields(); index++ {
		if structure.Field(index).Name() == "PurgeProgress" {
			return index, nil
		}
	}
	return -1, errors.New("PurgeProgress field is absent")
}

func s7ARIsPurgeProgressSelector(
	model *s6SourceTypeModel,
	canonicalField *types.Var,
	expression ast.Expr,
) bool {
	selector, ok := s7ARUnwrapCallExpression(expression).(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "PurgeProgress" {
		return false
	}
	selection := model.selections[selector]
	if selection == nil || selection.Kind() != types.FieldVal {
		return false
	}
	field, ok := selection.Obj().(*types.Var)
	return ok && field.IsField() && field == canonicalField
}

func s7ARIsDefiniteNoncanonicalPurgeProgressSelector(
	model *s6SourceTypeModel,
	canonicalField *types.Var,
	expression ast.Expr,
) bool {
	expression = s7ARUnwrapCallExpression(expression)
	if selector, ok := expression.(*ast.SelectorExpr); ok {
		selection := model.selections[selector]
		if selection == nil || selection.Kind() != types.FieldVal {
			return false
		}
		field, ok := selection.Obj().(*types.Var)
		return ok && field.IsField() && field != canonicalField &&
			types.Identical(
				types.Unalias(field.Type()),
				types.Unalias(canonicalField.Type()),
			)
	}
	typed, ok := model.expressionTypes[expression]
	return ok && typed.Type != nil &&
		types.Identical(
			types.Unalias(typed.Type),
			types.Unalias(canonicalField.Type()),
		)
}

func s7ARCanonicalPurgeProgressField(model *s6SourceTypeModel) (*types.Var, error) {
	var reportType *types.TypeName
	for identifier, object := range model.definitions {
		candidate, ok := object.(*types.TypeName)
		if !ok || identifier.Name != "intentArchivePurgeReport" ||
			candidate.Pkg() == nil ||
			!strings.HasSuffix(candidate.Pkg().Path(), "/internal/cli") {
			continue
		}
		if reportType != nil && reportType != candidate {
			return nil, errors.New("canonical intentArchivePurgeReport type is ambiguous")
		}
		reportType = candidate
	}
	if reportType == nil {
		return nil, errors.New("canonical intentArchivePurgeReport type is absent")
	}
	named, ok := types.Unalias(reportType.Type()).(*types.Named)
	if !ok {
		return nil, errors.New("canonical intentArchivePurgeReport is not named")
	}
	structure, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, errors.New("canonical intentArchivePurgeReport is not a struct")
	}
	var result *types.Var
	for index := 0; index < structure.NumFields(); index++ {
		field := structure.Field(index)
		if field.Name() != "PurgeProgress" {
			continue
		}
		if result != nil {
			return nil, errors.New("canonical PurgeProgress field is ambiguous")
		}
		result = field
	}
	if result == nil {
		return nil, errors.New("canonical PurgeProgress field is absent")
	}
	return result, nil
}

func s7ARAssignmentIdentifiers(expressions []ast.Expr) []*ast.Ident {
	result := make([]*ast.Ident, len(expressions))
	for index, expression := range expressions {
		result[index], _ = s7ARUnwrapCallExpression(expression).(*ast.Ident)
	}
	return result
}

func s7ARUpdatePurgeProgressAliases(
	model *s6SourceTypeModel,
	aliases map[types.Object]bool,
	left []*ast.Ident,
	right []ast.Expr,
) {
	for index, identifier := range left {
		if identifier == nil {
			continue
		}
		object := model.definitions[identifier]
		if object == nil {
			object = model.uses[identifier]
		}
		if object == nil {
			continue
		}
		expression, resolved := s7ARAssignmentRight(right, index)
		if !resolved {
			delete(aliases, object)
			continue
		}
		aliases[object] = s7ARPurgeProgressAliasTarget(model, aliases, expression)
	}
}

func s7ARAssignmentRight(
	right []ast.Expr,
	index int,
) (ast.Expr, bool) {
	if len(right) == 1 {
		return right[0], true
	}
	if index >= len(right) {
		return nil, false
	}
	return right[index], true
}

func s7ARPurgeProgressAliasTarget(
	model *s6SourceTypeModel,
	aliases map[types.Object]bool,
	expression ast.Expr,
) bool {
	expression = s7ARUnwrapCallExpression(expression)
	if address, ok := expression.(*ast.UnaryExpr); ok && address.Op == token.AND {
		canonicalField, err := s7ARCanonicalPurgeProgressField(model)
		return err == nil && s7ARIsPurgeProgressSelector(model, canonicalField, address.X)
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false
	}
	object := model.uses[identifier]
	if object == nil {
		object = model.definitions[identifier]
	}
	return aliases[object]
}

func s7ARIsIndirectPurgeProgressWrite(
	model *s6SourceTypeModel,
	expression ast.Expr,
) bool {
	dereference, ok := s7ARUnwrapCallExpression(expression).(*ast.StarExpr)
	if !ok {
		return false
	}
	typed, ok := model.expressionTypes[dereference]
	if !ok || typed.Type == nil {
		return true
	}
	return strings.TrimPrefix(
		s7ARTypeName(types.Unalias(typed.Type)), "*",
	) == "intentArchivePurgeProgressReport"
}

func s7ARDerefExpression(expression ast.Expr) ast.Expr {
	dereference, _ := s7ARUnwrapCallExpression(expression).(*ast.StarExpr)
	if dereference == nil {
		return nil
	}
	return dereference.X
}

func s7ARPurgeProgressValueKind(expression ast.Expr) string {
	expression = s7ARUnwrapCallExpression(expression)
	if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "nil" {
		return "nil"
	}
	if call, ok := expression.(*ast.CallExpr); ok &&
		s6CallName(call) == "buildIntentArchivePurgeProgress" {
		return "builder"
	}
	return "other"
}

func TestS7ARPermanentBlockClaimsGuard(t *testing.T) {
	t.Run("PIB-519", func(t *testing.T) {
		sources := s7ARShippedClaimSources(t)
		claimState, err := s7ARPrepareClaimSourceState(sources)
		if err != nil {
			t.Fatal(err)
		}
		inventory, err := s7ARDerivePermanentClaimInventoryWithState(
			sources, claimState,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(inventory) != 8 {
			claims, collectErr := s7ARCollectPermanentClaimsWithState(
				sources, claimState,
			)
			t.Fatalf("PIB-519 shipped claim inventory = %d, want exact 8: %v (collect error: %v)",
				len(inventory), claims, collectErr)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			sources, inventory, claimState,
		); err != nil {
			t.Fatal(err)
		}
		mutated := cloneS7AQSources(sources)
		mutated["internal/cli/prepare_publish.go"] +=
			"\nconst s7ARWrongPermanentClaim = \"The slug is never permanently blocked.\"\n"
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, inventory, claimState,
		); err == nil {
			t.Fatal("PIB-519 same validator accepted a permanent-block claim without a route")
		}
		mutated = cloneS7AQSources(sources)
		mutated["internal/cli/prepare_publish.go"] +=
			"\nconst s7ARWrongCitedPermanentClaim = \"The slug is never permanently blocked; see §6.6.\"\n"
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, inventory, claimState,
		); err == nil {
			t.Fatal("PIB-519 same validator accepted a generic citation instead of an executable route")
		}
		for _, fixture := range []struct {
			name  string
			claim string
		}{
			{name: "positive-active-restore", claim: "Operators can always restore the archive."},
			{name: "positive-recovery-possible", claim: "Recovery of the archive is always possible."},
			{name: "positive-impossible-to-strand", claim: "It is impossible to strand the archive."},
			{name: "positive-active-unblock", claim: "We can always unblock the slug."},
			{name: "positive-active-never-strand-reviewer-repro", claim: "Operators can never strand the archive."},
			{name: "positive-passive-never-stranded", claim: "The archive can never be stranded."},
			{name: "positive-reordered-never-strand", claim: "Never can operators strand the archive."},
			{name: "positive-never-leave-stranded", claim: "Operators will never leave the archive stranded."},
			{name: "positive-no-operator-strand-reviewer-repro", claim: "No operator can strand the archive."},
			{name: "positive-no-operators-ever-block", claim: "No operators can ever block the transaction."},
			{name: "positive-no-user-will-strand", claim: "No user will strand the slug."},
			{name: "positive-no-users-leave-stranded", claim: "No users can ever leave the archive stranded."},
			{name: "positive-no-operator-comma-ever-reviewer-repro", claim: "No operator, ever, can strand the archive."},
			{name: "positive-no-operator-ever-before-modal-reviewer-repro", claim: "No operator ever can strand the archive."},
			{name: "positive-no-users-comma-ever-will-block", claim: "No users, ever, will block the transaction."},
			{name: "positive-no-user-must-ever-leave-blocked", claim: "No user must ever leave the slug blocked."},
			{name: "positive-no-operators-em-dash-ever", claim: "No operators — ever — can leave the archive stranded."},
		} {
			mutated = cloneS7AQSources(sources)
			mutated["assets/skills/cursor/tessera-patch.mdc"] += "\n" + fixture.claim + "\n"
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", fixture.name, deriveErr)
			}
			if len(got) != len(inventory)+1 {
				t.Fatalf("PIB-519 %s inventory = %d, want %d",
					fixture.name, len(got), len(inventory)+1)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, got, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator accepted route-less %s", fixture.name)
			}
		}
		const claimMarker = "Exit 6 must never be a permanent state **without a named, applicable route out**"
		for _, fixture := range []struct {
			name        string
			replacement string
		}{
			{
				name:        "mandated-interrupted-purge-slogan",
				replacement: "no interrupted purge can leave the archive permanently blocked",
			},
			{
				name:        "equivalent-permanently-stranded-slogan",
				replacement: "The archive can never be permanently stranded",
			},
			{
				name:        "equivalent-unqualified-stranded-slogan",
				replacement: "The archive cannot be stranded.",
			},
			{
				name:        "unqualified-unrecoverable-slogan",
				replacement: "The archive cannot become unrecoverable.",
			},
			{
				name: "route-in-unrelated-sentence",
				replacement: "no interrupted purge can leave the archive permanently blocked. " +
					"For an unrelated journal cleanup, run `tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "route-in-unrelated-prior-sentence",
				replacement: "Operators must run `tpatch prepare <slug> --abandon-transaction` for an unrelated journal cleanup. " +
					"No interrupted purge can leave the archive permanently blocked.",
			},
			{
				name: "conditional-route",
				replacement: "No interrupted purge can leave the archive permanently blocked; " +
					"if asked, run `tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "conditional-then-run-reviewer-repro",
				replacement: "The archive cannot be stranded; if approved, then run " +
					"`tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "hypothetical-route",
				replacement: "No interrupted purge can leave the archive permanently blocked because operators " +
					"could run `tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "quoted-example-only-route",
				replacement: "No interrupted purge can leave the archive permanently blocked; " +
					"for example, \"run `tpatch prepare <slug> --abandon-transaction`\".",
			},
			{
				name: "route-under-negation",
				replacement: "no interrupted purge can leave the archive permanently blocked because operators " +
					"must not run `tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "route-under-prohibition",
				replacement: "no interrupted purge can leave the archive permanently blocked because operators " +
					"are forbidden to run `tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "route-under-not-permitted-prohibition",
				replacement: "no interrupted purge can leave the archive permanently blocked because operators " +
					"are not permitted to run `tpatch prepare <slug> --abandon-transaction`.",
			},
			{
				name: "route-suffix-prohibition",
				replacement: "The archive cannot be stranded; " +
					"`tpatch prepare <slug> --abandon-transaction` is prohibited.",
			},
			{
				name: "route-suffix-condition",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` if asked.",
			},
			{
				name: "route-suffix-prohibition-after-comma",
				replacement: "The archive cannot be stranded; " +
					"`tpatch prepare <slug> --abandon-transaction`, according to policy, is forbidden.",
			},
			{
				name: "route-suffix-only-if-condition",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` only if recovery is requested.",
			},
			{
				name: "route-semicolon-later-prohibition",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction`; however, that command is prohibited.",
			},
			{
				name: "route-suffix-when-condition",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` when asked.",
			},
			{
				name: "route-suffix-subject-to-approval",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` subject to approval.",
			},
			{
				name: "route-suffix-unless-approved",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` unless approval is denied.",
			},
			{
				name: "route-suffix-provided-that-approved",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` provided that approval exists.",
			},
			{
				name: "route-newline-subject-to-approval",
				replacement: "The archive cannot be stranded; operators must run\n" +
					"`tpatch prepare <slug> --abandon-transaction`\nsubject to approval.",
			},
			{
				name: "route-second-clause-approval-required",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction`; approval is required.",
			},
			{
				name: "route-newline-second-clause-prohibition",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction`\n" +
					"except when the operator lacks approval.",
			},
			{
				name: "route-open-suffix-extension",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction` " +
					"to unblock the slug without touching any canonical file and any sibling lane.",
			},
			{
				name: "abandon-boolean-suffix",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction=false`.",
			},
			{
				name: "abandon-extra-flag",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction --check`.",
			},
			{
				name: "abandon-extra-argv-token",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare <slug> --abandon-transaction unexpected`.",
			},
			{
				name: "abandon-windows-reserved-slug-reviewer-repro",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch prepare con --abandon-transaction`.",
			},
			{
				name: "abandon-sixty-one-byte-slug-reviewer-repro",
				replacement: "The archive cannot be stranded; operators must run `tpatch prepare " +
					strings.Repeat("a", 61) + " --abandon-transaction`.",
			},
			{
				name: "purge-route-missing-confirmation",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --all`.",
			},
			{
				name: "purge-windows-reserved-slug",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge con --all --yes`.",
			},
			{
				name: "purge-sixty-one-byte-slug",
				replacement: "The archive cannot be stranded; operators must run `tpatch feature " +
					"intent-archive purge " + strings.Repeat("a", 61) + " --all --yes`.",
			},
			{
				name: "purge-route-mixed-selectors",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --all --blob <hash> --yes`.",
			},
			{
				name: "purge-blob-slug-placeholder-reviewer-repro",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --blob <slug> --yes`.",
			},
			{
				name: "purge-blob-generation-placeholder",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --blob <id> --yes`.",
			},
			{
				name: "purge-generation-hash-placeholder",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --generation <hash> --yes`.",
			},
			{
				name: "purge-generation-slug-placeholder",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --generation <slug> --yes`.",
			},
			{
				name: "purge-blob-short-value",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --blob abc123 --yes`.",
			},
			{
				name: "purge-blob-nonhex-value",
				replacement: "The archive cannot be stranded; operators must run `tpatch feature " +
					"intent-archive purge <slug> --blob " + strings.Repeat("g", 64) + " --yes`.",
			},
			{
				name: "purge-generation-short-value",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --generation 0123456789abcdef --yes`.",
			},
			{
				name: "purge-generation-uppercase-value",
				replacement: "The archive cannot be stranded; operators must run `tpatch feature " +
					"intent-archive purge <slug> --generation " + strings.Repeat("A", 64) + " --yes`.",
			},
			{
				name: "purge-route-extra-argv",
				replacement: "The archive cannot be stranded; operators must run " +
					"`tpatch feature intent-archive purge <slug> --orphans --yes --json`.",
			},
			{
				name: "manual-intent-lane-parent-traversal",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare/<slug>/../other/`.",
			},
			{
				name: "manual-intent-lane-repeated-separator",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare//<slug>/`.",
			},
			{
				name: "manual-intent-lane-wildcard-reviewer-repro",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare/<slug>/*`.",
			},
			{
				name: "manual-intent-lane-windows-reserved-slug",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare/con/`.",
			},
			{
				name: "manual-intent-lane-sixty-one-byte-slug",
				replacement: "The archive cannot be stranded; operators must run `rm -rf " +
					".tpatch/local/intent-prepare/" + strings.Repeat("a", 61) + "/`.",
			},
			{
				name: "manual-intent-lane-missing-canonical-slash",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare/<slug>`.",
			},
			{
				name: "manual-intent-lane-brace-glob",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare/{<slug>,other}/`.",
			},
			{
				name: "manual-intent-lane-shell-redirection",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/local/intent-prepare/<slug>/ >other`.",
			},
			{
				name: "manual-blob-child-extension",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob/child`.",
			},
			{
				name: "manual-blob-wildcard-target",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/*.blob`.",
			},
			{
				name: "manual-blob-trailing-slash-extension",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob/`.",
			},
			{
				name: "manual-blob-parent-traversal",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob/../other`.",
			},
			{
				name: "manual-blob-missing-option-terminator",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob`.",
			},
			{
				name: "manual-blob-windows-reserved-slug",
				replacement: "The archive cannot be stranded; operators must run " +
					"`rm -rf -- .tpatch/features/con/artifacts/intent-archive/blobs/<hash>.blob`.",
			},
			{
				name: "manual-blob-sixty-one-byte-slug",
				replacement: "The archive cannot be stranded; operators must run `rm -rf -- " +
					".tpatch/features/" + strings.Repeat("a", 61) +
					"/artifacts/intent-archive/blobs/<hash>.blob`.",
			},
			{
				name:        "positive-always-recovered",
				replacement: "The archive can always be recovered.",
			},
			{
				name:        "positive-active-always-recovered",
				replacement: "Operators can always recover the archive.",
			},
			{
				name: "route-in-attributed-quotation",
				replacement: "No interrupted purge can leave the archive permanently blocked because " +
					"the documentation says \"operators should run `tpatch prepare <slug> --abandon-transaction`\".",
			},
		} {
			mutated = cloneS7AQSources(sources)
			replaced, changed := s7ARReplaceClaimParagraph(
				mutated["docs/prds/PRD-prepare-intent-bundle.md"],
				claimMarker,
				fixture.replacement,
			)
			if !changed {
				t.Fatalf("PIB-519 %s sensitivity anchor missing", fixture.name)
			}
			mutated["docs/prds/PRD-prepare-intent-bundle.md"] = replaced
			if _, err := s7ARValidatedRouteAuthorities(mutated); err != nil {
				t.Fatalf("PIB-519 %s damaged route authority: %v", fixture.name, err)
			}
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s could not derive mutated inventory: %v",
					fixture.name, deriveErr)
			}
			if !reflect.DeepEqual(got, inventory) {
				t.Fatalf("PIB-519 %s changed claim inventory:\ngot:  %v\nwant: %v",
					fixture.name, got, inventory)
			}
			err := validateS7ARPermanentBlockClaimsWithState(
				mutated, inventory, claimState,
			)
			if err == nil {
				t.Fatalf("PIB-519 same validator accepted %s", fixture.name)
			}
			if !strings.Contains(err.Error(), "no affirmative accepted command") {
				t.Fatalf("PIB-519 %s failed for %q, want clause-local route rejection",
					fixture.name, err)
			}
		}
		slugFixtures := []struct {
			slug string
			want bool
		}{
			{slug: "", want: false},
			{slug: "a", want: true},
			{slug: "a0", want: true},
			{slug: "a-b", want: true},
			{slug: strings.Repeat("a", 60), want: true},
			{slug: strings.Repeat("a", 61), want: false},
			{slug: "A", want: false},
			{slug: "a_b", want: false},
			{slug: "-a", want: false},
			{slug: "a-", want: false},
			{slug: "a--b", want: false},
			{slug: "café", want: false},
			{slug: "con", want: false},
			{slug: "prn", want: false},
			{slug: "aux", want: false},
			{slug: "nul", want: false},
			{slug: "console", want: true},
			{slug: "com0", want: true},
			{slug: "com10", want: true},
			{slug: "lpt10", want: true},
		}
		for index := 1; index <= 9; index++ {
			slugFixtures = append(
				slugFixtures,
				struct {
					slug string
					want bool
				}{slug: "com" + strconv.Itoa(index), want: false},
				struct {
					slug string
					want bool
				}{slug: "lpt" + strconv.Itoa(index), want: false},
			)
		}
		for _, fixture := range slugFixtures {
			canonical, canonicalErr := patchintent.CanonicalSlug(fixture.slug)
			productionAccepts := canonicalErr == nil && canonical == fixture.slug
			if productionAccepts != fixture.want {
				t.Fatalf("PIB-519 production slug fixture %q = %t, want %t",
					fixture.slug, productionAccepts, fixture.want)
			}
			if got := s7ARValidRouteSlug(fixture.slug); got != productionAccepts {
				t.Fatalf("PIB-519 route slug %q = %t, production = %t",
					fixture.slug, got, productionAccepts)
			}
		}
		if !s7ARValidRouteSlug("<slug>") {
			t.Fatal("PIB-519 normative route placeholder <slug> was rejected")
		}
		for _, fixture := range []struct {
			name        string
			source      string
			extraRel    string
			extraSource string
		}{
			{
				name: "split-concatenated-mandated-slogan",
				source: "\nconst s7ARSplitPermanentClaim = " +
					"\"no interrupted purge can leave the archive \" + \"permanently blocked\"\n",
			},
			{
				name: "named-constant-concatenated-stranding-claim",
				source: "\nconst s7ARPermanentClaimPrefix = \"The archive can never be \"\n" +
					"const s7ARNamedPermanentClaim = s7ARPermanentClaimPrefix + \"permanently stranded\"\n",
			},
			{
				name: "function-local-constant-unrecoverable-claim",
				source: "\nfunc s7ARLocalPermanentClaim() {\n" +
					"\tconst prefix = \"The archive cannot become \"\n" +
					"\tconst claim = (prefix + \"unrecoverable.\")\n" +
					"\t_ = claim\n" +
					"}\n",
			},
			{
				name:     "cross-file-named-constant-unrecoverable-claim",
				source:   "\nconst s7ARCrossFileClaimPrefix = \"The archive cannot become \"\n",
				extraRel: "internal/cli/zz_s7_ar_claim_mutation.go",
				extraSource: "package cli\n\n" +
					"const s7ARCrossFileClaim = (s7ARCrossFileClaimPrefix + \"unrecoverable.\")\n",
			},
			{
				name: "rune-to-string-reviewer-repro",
				source: "\nconst s7ARRuneConvertedClaim = " +
					"\"The ar\" + string('c') + \"hive cannot be stranded.\"\n",
			},
			{
				name: "typed-string-constant-claim",
				source: "\nconst s7ARTypedClaimPrefix string = \"The archive cannot be \"\n" +
					"const s7ARTypedClaim = (s7ARTypedClaimPrefix + \"stranded.\")\n",
			},
			{
				name: "lexically-shadowed-local-constant-claim",
				source: "\nfunc s7ARBenignLocalClaimCollision() {\n" +
					"\tconst prefix = \"ordinary text\"\n" +
					"\t_ = prefix\n" +
					"}\n\n" +
					"func s7ARAuthoritativeLocalClaimCollision() {\n" +
					"\tconst prefix = \"The archive cannot be \"\n" +
					"\tconst claim = prefix + \"stranded.\"\n" +
					"\t_ = claim\n" +
					"}\n",
			},
		} {
			mutated = cloneS7AQSources(sources)
			mutated["internal/cli/prepare_publish.go"] += fixture.source
			if fixture.extraRel != "" {
				mutated[fixture.extraRel] = fixture.extraSource
			}
			err := validateS7ARPermanentBlockClaimsWithState(
				mutated, inventory, claimState,
			)
			if err == nil {
				t.Fatalf("PIB-519 same validator accepted %s", fixture.name)
			}
			if !strings.Contains(err.Error(), "no affirmative accepted command") {
				t.Fatalf("PIB-519 %s failed for %q, want folded claim route rejection",
					fixture.name, err)
			}
		}
		for _, fixture := range []struct {
			name      string
			extra     map[string]string
			wantDelta int
		}{
			{
				name: "build-variant-candidate-cross-product",
				extra: map[string]string{
					"internal/cli/zz_s7_ar_prefix_linux.go": "//go:build linux\n\npackage cli\n\n" +
						"const s7ARBuildClaimPrefix = \"The archive cannot be \"\n",
					"internal/cli/zz_s7_ar_prefix_darwin.go": "//go:build darwin\n\npackage cli\n\n" +
						"const s7ARBuildClaimPrefix = \"The transaction cannot be \"\n",
					"internal/cli/zz_s7_ar_suffix_linux.go": "//go:build linux\n\npackage cli\n\n" +
						"const s7ARBuildClaimSuffix = \"stranded.\"\n",
					"internal/cli/zz_s7_ar_suffix_darwin.go": "//go:build darwin\n\npackage cli\n\n" +
						"const s7ARBuildClaimSuffix = \"blocked.\"\n",
					"internal/cli/zz_s7_ar_build_claim.go": "package cli\n\n" +
						"const s7ARBuildVariantClaim = s7ARBuildClaimPrefix + s7ARBuildClaimSuffix\n",
				},
				wantDelta: 4,
			},
			{
				name: "build-variant-candidate-dedup",
				extra: map[string]string{
					"internal/cli/zz_s7_ar_duplicate_linux.go": "//go:build linux\n\npackage cli\n\n" +
						"const s7ARDuplicateClaimPrefix = \"The archive cannot be \"\n",
					"internal/cli/zz_s7_ar_duplicate_darwin.go": "//go:build darwin\n\npackage cli\n\n" +
						"const s7ARDuplicateClaimPrefix = \"The archive cannot be \"\n",
					"internal/cli/zz_s7_ar_duplicate_claim.go": "package cli\n\n" +
						"const s7ARDeduplicatedBuildClaim = s7ARDuplicateClaimPrefix + \"stranded.\"\n",
				},
				wantDelta: 1,
			},
		} {
			mutated = cloneS7AQSources(sources)
			for rel, source := range fixture.extra {
				mutated[rel] = source
			}
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", fixture.name, deriveErr)
			}
			if len(got) != len(inventory)+fixture.wantDelta {
				t.Fatalf("PIB-519 %s inventory = %d, want %d",
					fixture.name, len(got), len(inventory)+fixture.wantDelta)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, got, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator accepted route-less %s", fixture.name)
			}
		}
		for _, fixture := range []struct {
			name   string
			source string
		}{
			{
				name: "cyclic-claim-constant",
				source: "\nconst s7ARCyclicClaimA string = s7ARCyclicClaimB\n" +
					"const s7ARCyclicClaimB string = s7ARCyclicClaimA + " +
					"\"The archive cannot be stranded.\"\n",
			},
			{
				name: "unresolved-claim-constant",
				source: "\nconst s7ARUnresolvedClaim string = " +
					"s7ARMissingClaimPrefix + \"stranded.\"\n",
			},
		} {
			mutated = cloneS7AQSources(sources)
			mutated["internal/cli/prepare_publish.go"] += fixture.source
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, inventory, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator silently omitted %s", fixture.name)
			}
		}
		mutated = cloneS7AQSources(sources)
		mutated["internal/cli/prepare_publish.go"] +=
			"\nconst s7ARCompletePermanentClaim = " +
				"\"The archive cannot be stranded; operators must run `tpatch prepare <slug> --abandon-transaction`.\"\n" +
				"const s7ARCompletePermanentClaimAlias = s7ARCompletePermanentClaim\n"
		aliasedInventory, err := s7ARDerivePermanentClaimInventoryWithState(
			mutated, claimState,
		)
		if err != nil {
			t.Fatalf("PIB-519 constant-alias inventory: %v", err)
		}
		if len(aliasedInventory) != len(inventory)+1 {
			t.Fatalf("PIB-519 constant alias inflated inventory to %d, want %d",
				len(aliasedInventory), len(inventory)+1)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, aliasedInventory, claimState,
		); err != nil {
			t.Fatalf("PIB-519 rejected deduplicated complete alias claim: %v", err)
		}
		cacheMutation := func(body string) map[string]string {
			candidate := cloneS7AQSources(sources)
			candidate["internal/cli/prepare_publish.go"] +=
				"\nconst s7ARClaimCacheMutation = " + strconv.Quote(body) + "\n"
			return candidate
		}
		cacheInvalid := cacheMutation("The archive cannot be stranded.")
		cacheInvalidInventory, err := s7ARDerivePermanentClaimInventoryWithState(
			cacheInvalid, claimState,
		)
		if err != nil {
			t.Fatalf("PIB-519 invalid cache mutation inventory: %v", err)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			cacheInvalid, cacheInvalidInventory, claimState,
		); err == nil {
			t.Fatal("PIB-519 exact-key cache accepted a route-less Go claim")
		}
		cacheValid := cacheMutation(
			"The archive cannot be stranded; operators must run " +
				"`tpatch prepare <slug> --abandon-transaction`.",
		)
		cacheValidInventory, err := s7ARDerivePermanentClaimInventoryWithState(
			cacheValid, claimState,
		)
		if err != nil {
			t.Fatalf("PIB-519 valid cache mutation inventory: %v", err)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			cacheValid, cacheValidInventory, claimState,
		); err != nil {
			t.Fatalf("PIB-519 exact-key cache reused stale invalid state: %v", err)
		}
		cacheInvalidAgain, err := s7ARDerivePermanentClaimInventoryWithState(
			cacheInvalid, claimState,
		)
		if err != nil {
			t.Fatalf("PIB-519 revisited cache mutation inventory: %v", err)
		}
		if !reflect.DeepEqual(cacheInvalidAgain, cacheInvalidInventory) {
			t.Fatalf(
				"PIB-519 exact-key cache changed by test order: got %v, want %v",
				cacheInvalidAgain, cacheInvalidInventory,
			)
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			cacheInvalid, cacheInvalidAgain, claimState,
		); err == nil {
			t.Fatal("PIB-519 exact-key cache leaked valid state into invalid mutation")
		}
		for _, fixture := range []struct {
			name  string
			claim string
		}{
			{
				name:  "negative-not-always-recovered",
				claim: "The archive cannot always be recovered.",
			},
			{
				name:  "negative-not-always-recoverable",
				claim: "The archive is not always recoverable.",
			},
			{
				name:  "quoted-example-recovered",
				claim: "For example, \"The archive can always be recovered.\"",
			},
			{
				name:  "negative-can-never-recover",
				claim: "Operators can never recover the archive.",
			},
			{
				name:  "conditional-never-strand",
				claim: "If approval arrives, operators can never strand the archive.",
			},
			{
				name:  "quoted-never-strand",
				claim: "The documentation says \"Operators can never strand the archive.\"",
			},
			{
				name:  "historical-never-strand",
				claim: "An earlier revision said operators can never strand the archive.",
			},
			{
				name:  "unrelated-subject-never-strand",
				claim: "Operators can never strand the unrelated database.",
			},
			{
				name:  "conditional-no-operator-strand",
				claim: "If approval arrives, no operator can strand the archive.",
			},
			{
				name:  "hypothetical-no-user-block",
				claim: "Hypothetically, no user can block the transaction.",
			},
			{
				name:  "quoted-attributed-no-operator-strand",
				claim: "The guide says \"No operator can strand the archive.\"",
			},
			{
				name:  "historical-no-operator-strand",
				claim: "An earlier revision said no operator can strand the archive.",
			},
			{
				name:  "unrelated-subject-no-operator-strand",
				claim: "No operator can strand the unrelated database.",
			},
			{
				name:  "unrelated-verb-no-operator",
				claim: "No operator can inspect the archive.",
			},
			{
				name:  "double-negation-no-operator-cannot-strand",
				claim: "No operator cannot strand the archive.",
			},
			{
				name:  "negative-quantifier-never-recover",
				claim: "No operator can ever recover the archive.",
			},
			{
				name:  "negative-quantifier-ever-before-modal-recover",
				claim: "No operator ever can recover the archive.",
			},
			{
				name:  "negative-quantifier-conditional-suffix",
				claim: "No operator can strand the archive if approval is denied.",
			},
			{
				name:  "negative-quantifier-unless-qualifier",
				claim: "No operator ever can strand the archive unless an exception applies.",
			},
			{
				name:  "negative-quantifier-except-qualifier",
				claim: "No operator, except an administrator, can strand the archive.",
			},
			{
				name:  "negative-quantifier-hypothetical-modal",
				claim: "No operator could strand the archive.",
			},
			{
				name:  "negative-quantifier-punctuated-double-negation",
				claim: "No operator, ever, cannot strand the archive.",
			},
			{
				name:  "unsupported-there-is-no-operator",
				claim: "There is no operator who can strand the archive.",
			},
			{
				name:  "conditional-active-recoverability",
				claim: "If asked, operators can always recover the archive.",
			},
			{
				name:  "future-active-recoverability",
				claim: "In the future, operators can always recover the archive.",
			},
			{
				name:  "quoted-active-recoverability",
				claim: "The documentation says \"Operators can always recover the archive.\"",
			},
			{
				name:  "negative-cannot-always-restore",
				claim: "Operators cannot always restore the archive.",
			},
			{
				name:  "negative-recovery-not-always-possible",
				claim: "Recovery of the archive is not always possible.",
			},
			{
				name:  "negative-not-impossible-to-strand",
				claim: "It is not impossible to strand the archive.",
			},
			{
				name:  "negative-can-never-unblock",
				claim: "We can never unblock the slug.",
			},
			{
				name:  "conditional-restore",
				claim: "If approval arrives, operators can always restore the archive.",
			},
			{
				name:  "hypothetical-recovery-possible",
				claim: "Hypothetically, recovery of the archive is always possible.",
			},
			{
				name:  "future-unblock",
				claim: "In the future, we can always unblock the slug.",
			},
			{
				name:  "aspirational-impossible-to-strand",
				claim: "We aspire to make it impossible to strand the archive.",
			},
			{
				name:  "quoted-restore",
				claim: "The guide says \"Operators can always restore the archive.\"",
			},
			{
				name:  "historical-restore",
				claim: "An earlier revision said operators can always restore the archive.",
			},
			{
				name:  "unrelated-possible",
				claim: "Recovery of the unrelated database is always possible.",
			},
		} {
			mutated = cloneS7AQSources(sources)
			mutated["assets/skills/cursor/tessera-patch.mdc"] += "\n" + fixture.claim + "\n"
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", fixture.name, deriveErr)
			}
			if !reflect.DeepEqual(got, inventory) {
				t.Fatalf("PIB-519 %s created a false claim inventory: %v",
					fixture.name, got)
			}
			err := validateS7ARPermanentBlockClaimsWithState(
				mutated, inventory, claimState,
			)
			if err != nil {
				t.Fatalf("PIB-519 %s created a false claim: %v", fixture.name, err)
			}
		}
		for _, reference := range []string{"D130", "D13a", "pre-D13-post"} {
			mutated = cloneS7AQSources(sources)
			mutated["assets/skills/cursor/tessera-patch.mdc"] +=
				"\nThe archive cannot be stranded because the route belongs to " +
					reference + ".\n"
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", reference, deriveErr)
			}
			if len(got) != len(inventory)+1 {
				t.Fatalf("PIB-519 %s inventory size = %d, want %d",
					reference, len(got), len(inventory)+1)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, got, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator accepted %s as exact D13 authority",
					reference)
			}
		}
		for _, fixture := range []struct {
			name  string
			claim string
		}{
			{
				name:  "negated-exact-reference",
				claim: "The archive cannot be stranded because no route belongs to D13.",
			},
			{
				name: "quoted-exact-reference",
				claim: "The archive cannot be stranded because the documentation says " +
					"\"the route belongs to D13\".",
			},
			{
				name: "example-only-exact-reference",
				claim: "The archive cannot be stranded; for example, " +
					"the route belongs to D13.",
			},
			{
				name:  "negated-governed-reference",
				claim: "The archive cannot be stranded because this is not governed by D13.",
			},
			{
				name:  "conditional-exact-reference",
				claim: "The archive cannot be stranded because the route belongs to D13 subject to approval.",
			},
			{
				name: "second-clause-negated-reference",
				claim: "The archive cannot be stranded because the route belongs to D13; " +
					"however, this is not governed by D13.",
			},
		} {
			mutated = cloneS7AQSources(sources)
			mutated["assets/skills/cursor/tessera-patch.mdc"] +=
				"\n" + fixture.claim + "\n"
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 %s inventory: %v", fixture.name, deriveErr)
			}
			if len(got) != len(inventory)+1 {
				t.Fatalf("PIB-519 %s inventory size = %d, want %d",
					fixture.name, len(got), len(inventory)+1)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, got, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator accepted %s", fixture.name)
			}
		}
		for _, claim := range []string{
			"The archive cannot be stranded because the route belongs to D13.",
			"The archive can always be recovered; operators must run " +
				"`tpatch prepare <slug> --abandon-transaction`.",
			"The archive can always be recovered; operators must run `tpatch prepare " +
				strings.Repeat("a", 60) + " --abandon-transaction`.",
			"The archive can always be recovered; operators must run " +
				"`tpatch prepare console --abandon-transaction`.",
			"The archive can always be recovered; operators must run " +
				"`tpatch feature intent-archive purge <slug> --blob <hash> --yes`.",
			"The archive can always be recovered; operators must run " +
				"`tpatch feature intent-archive purge demo --blob " + strings.Repeat("a", 64) + " --yes`.",
			"The archive can always be recovered; operators must run " +
				"`tpatch feature intent-archive purge <slug> --generation <id> --yes`.",
			"The archive can always be recovered; operators must run " +
				"`tpatch feature intent-archive purge demo --generation " + strings.Repeat("f", 64) + " --yes`.",
			"The archive can always be recovered; operators must run " +
				"`tpatch feature intent-archive purge com0 --all --yes`.",
			"The archive can always be recovered; operators must run " +
				"`rm -rf .tpatch/local/intent-prepare/<slug>/`.",
			"The archive can always be recovered; operators must run " +
				"`rm -rf .tpatch/local/intent-prepare/com10/`.",
			"The archive can always be recovered; operators must run " +
				"`rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob`.",
			"The archive can always be recovered; operators must run " +
				"`rm -rf -- .tpatch/features/lpt10/artifacts/intent-archive/blobs/<hash>.blob`.",
		} {
			mutated = cloneS7AQSources(sources)
			mutated["assets/skills/cursor/tessera-patch.mdc"] += "\n" + claim + "\n"
			got, deriveErr := s7ARDerivePermanentClaimInventoryWithState(
				mutated, claimState,
			)
			if deriveErr != nil {
				t.Fatalf("PIB-519 legitimate route inventory: %v", deriveErr)
			}
			if len(got) != len(inventory)+1 {
				t.Fatalf("PIB-519 legitimate route inventory size = %d, want %d",
					len(got), len(inventory)+1)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, got, claimState,
			); err != nil {
				t.Fatalf("PIB-519 rejected legitimate exact route authority: %v", err)
			}
		}
		for _, rel := range []string{
			"assets/skills/cursor/tessera-patch.mdc",
			"assets/skills/windsurf/windsurfrules",
		} {
			if _, present := sources[rel]; !present {
				t.Fatalf("PIB-519 embedded textual inventory omitted %s", rel)
			}
			mutated = cloneS7AQSources(sources)
			mutated[rel] += "\nThe archive cannot be stranded.\n"
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, inventory, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator missed route-less claim in %s", rel)
			}
		}
		mutated = cloneS7AQSources(sources)
		mainSource, ok := mutated["cmd/tpatch/main.go"]
		if !ok {
			t.Fatal("PIB-519 repository-wide source inventory omitted cmd/tpatch/main.go")
		}
		mutated["cmd/tpatch/main.go"] = mainSource +
			"\nconst s7AROutOfOldInventoryClaim = \"The archive is never permanently blocked.\"\n"
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, inventory, claimState,
		); err == nil {
			t.Fatal("PIB-519 same validator missed an out-of-old-list shipped source")
		}
		mutated = cloneS7AQSources(sources)
		const route = "| `undo-cas-mismatch`, `recovery-divergent`, `journal-corrupt`, `journal-version-mismatch`, `journal-foreign`, `journal-path-escape`, `journal-forged`, `post-publication-divergence`, `workspace-root-replaced-after-publication` | `tpatch prepare <slug> --abandon-transaction`"
		mutated["docs/prds/PRD-prepare-intent-bundle.md"] = strings.Replace(
			mutated["docs/prds/PRD-prepare-intent-bundle.md"],
			route,
			strings.Replace(route, "--abandon-transaction", "--check", 1),
			1,
		)
		if mutated["docs/prds/PRD-prepare-intent-bundle.md"] ==
			sources["docs/prds/PRD-prepare-intent-bundle.md"] {
			t.Fatal("PIB-519 accepted-route sensitivity anchor missing")
		}
		if err := validateS7ARPermanentBlockClaimsWithState(
			mutated, inventory, claimState,
		); err == nil {
			t.Fatal("PIB-519 same validator accepted a corrupted shipped route")
		}
		for _, fixture := range []struct {
			name   string
			mutate func(map[string]string) bool
		}{
			{
				name: "missing-reference-target",
				mutate: func(mutated map[string]string) bool {
					const rel = "docs/adrs/ADR-035-intent-bundle-publication-and-history.md"
					before := mutated[rel]
					mutated[rel] = strings.Replace(before, "### D13 —", "### D13 missing —", 1)
					return mutated[rel] != before
				},
			},
			{
				name: "negative-reference-target",
				mutate: func(mutated map[string]string) bool {
					const rel = "docs/adrs/ADR-035-intent-bundle-publication-and-history.md"
					before := mutated[rel]
					start := strings.Index(before, "### D13 —")
					if start < 0 {
						return false
					}
					endOffset := strings.Index(before[start:], "\n### D14")
					if endOffset < 0 {
						return false
					}
					end := start + endOffset
					mutated[rel] = before[:start] +
						"### D13 — negative reference fixture\n\n" +
						"Operators must not run `tpatch prepare <slug> --abandon-transaction`; " +
						"no manual fallback is executable. The archive-purge-evidence-divergent " +
						"archive procedure is prohibited.\n" +
						before[end:]
					return true
				},
			},
		} {
			mutated = cloneS7AQSources(sources)
			if !fixture.mutate(mutated) {
				t.Fatalf("PIB-519 %s sensitivity anchor missing", fixture.name)
			}
			if err := validateS7ARPermanentBlockClaimsWithState(
				mutated, inventory, claimState,
			); err == nil {
				t.Fatalf("PIB-519 same validator accepted %s", fixture.name)
			}
		}
	})
}

func validateS7ARPermanentBlockClaims(
	sources map[string]string,
	wantInventory []string,
) error {
	state, err := s7ARPrepareClaimSourceState(sources)
	if err != nil {
		return err
	}
	return validateS7ARPermanentBlockClaimsWithState(
		sources, wantInventory, state,
	)
}

func validateS7ARPermanentBlockClaimsWithState(
	sources map[string]string,
	wantInventory []string,
	state *s7ARClaimSourceState,
) error {
	prd, ok := sources["docs/prds/PRD-prepare-intent-bundle.md"]
	if !ok {
		return errors.New("shipped claim inventory omits the prepare PRD")
	}
	if err := validateS7ARExitSixRoutes(prd); err != nil {
		return fmt.Errorf("shipped exit-6 route authority: %w", err)
	}
	authorities, err := s7ARValidatedRouteAuthorities(sources)
	if err != nil {
		return err
	}
	claims, err := s7ARCollectPermanentClaimsWithState(sources, state)
	if err != nil {
		return err
	}
	gotInventory := make([]string, 0, len(claims))
	for _, claim := range claims {
		gotInventory = append(gotInventory, claim.key)
		if !s7ARClaimHasExecutableRoute(claim.routeText) &&
			!s7ARClaimHasValidatedRouteReference(claim.heading, claim.routeText, authorities) {
			return fmt.Errorf("%s permanent-block claim under %q has no affirmative accepted command, manual procedure or resolved reference: %q",
				claim.key, claim.heading, claim.text)
		}
	}
	if !reflect.DeepEqual(gotInventory, wantInventory) {
		return fmt.Errorf("permanent-block claim inventory = %v, want %v",
			gotInventory, wantInventory)
	}
	return nil
}

type s7ARPermanentClaim struct {
	key       string
	heading   string
	text      string
	routeText string
	origin    string
}

func s7ARDerivePermanentClaimInventory(
	sources map[string]string,
) ([]string, error) {
	state, err := s7ARPrepareClaimSourceState(sources)
	if err != nil {
		return nil, err
	}
	return s7ARDerivePermanentClaimInventoryWithState(sources, state)
}

func s7ARDerivePermanentClaimInventoryWithState(
	sources map[string]string,
	state *s7ARClaimSourceState,
) ([]string, error) {
	claims, err := s7ARCollectPermanentClaimsWithState(sources, state)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(claims))
	for _, claim := range claims {
		result = append(result, claim.key)
	}
	return result, nil
}

func s7ARCollectPermanentClaims(
	sources map[string]string,
) ([]s7ARPermanentClaim, error) {
	state, err := s7ARPrepareClaimSourceState(sources)
	if err != nil {
		return nil, err
	}
	return s7ARCollectPermanentClaimsWithState(sources, state)
}

func s7ARCollectPermanentClaimsWithState(
	sources map[string]string,
	state *s7ARClaimSourceState,
) ([]s7ARPermanentClaim, error) {
	var err error
	state, err = s7ARClaimStateForSources(sources, state)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	var result []s7ARPermanentClaim
	seenOrigins := map[string]bool{}
	for _, name := range names {
		cacheKey := s7ARClaimSourceCacheKey(name, sources[name])
		cache, cached := state.sourceClaims[name]
		var candidates []s7ARPermanentClaim
		if cached && cache.key == cacheKey {
			candidates = cache.claims
		} else {
			var sections []s7ARClaimSection
			if strings.HasSuffix(name, ".go") {
				sections = state.goSections[name]
			} else {
				sections = s7ARClaimSections(sources[name])
			}
			candidates = s7ARClaimsForSource(name, sections)
		}
		for _, claim := range candidates {
			if claim.origin != "" {
				origin := claim.origin + "\x00" +
					strings.ToLower(strings.Join(strings.Fields(claim.text), " "))
				if seenOrigins[origin] {
					continue
				}
				seenOrigins[origin] = true
			}
			result = append(result, claim)
		}
	}
	return result, nil
}

func s7ARClaimsForSource(
	name string,
	sections []s7ARClaimSection,
) []s7ARPermanentClaim {
	var result []s7ARPermanentClaim
	for sectionIndex, section := range sections {
		claimIndex := 0
		sentences := s7ARClaimSentences(section.body)
		for sentenceIndex, sentence := range sentences {
			lower := strings.ToLower(strings.Join(strings.Fields(sentence), " "))
			if !s7ARIsPermanentClaim(lower) ||
				s7ARExcludedClaimSection(section.heading, lower) ||
				s7ARHistoricalClaimSentence(lower) {
				continue
			}
			claim := s7ARPermanentClaim{
				key: fmt.Sprintf(
					"%s#%d:%d", name, sectionIndex, claimIndex,
				),
				heading:   section.heading,
				text:      strings.Join(strings.Fields(sentence), " "),
				routeText: strings.Join(strings.Fields(sentence), " "),
				origin:    section.origin,
			}
			if sentenceIndex+1 < len(sentences) &&
				s7ARRelatedRouteContinuation(sentences[sentenceIndex+1]) {
				claim.routeText += " " +
					strings.Join(strings.Fields(sentences[sentenceIndex+1]), " ")
			}
			result = append(result, claim)
			claimIndex++
		}
	}
	return result
}

func s7ARRelatedRouteContinuation(sentence string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(sentence), " "))
	if strings.Contains(lower, "unrelated") {
		return false
	}
	return regexp.MustCompile(
		`^(?:lock ownership|the route|that route|this route|the escape|the one exit|where the|on a supported)`,
	).MatchString(lower)
}

type s7ARClaimSection struct {
	heading string
	body    string
	origin  string
}

type s7ARClaimSourceState struct {
	goKey        string
	goSources    map[string]string
	goSections   map[string][]s7ARClaimSection
	sourceClaims map[string]s7ARClaimSourceCache
	derived      map[string]*s7ARClaimSourceState
}

type s7ARClaimSourceCache struct {
	key    string
	claims []s7ARPermanentClaim
}

func s7ARPrepareClaimSourceState(
	sources map[string]string,
) (*s7ARClaimSourceState, error) {
	goSources := s7ARClaimGoSources(sources)
	sections, err := s7ARClaimSectionsForGoSources(goSources)
	if err != nil {
		return nil, err
	}
	state := &s7ARClaimSourceState{
		goKey:        s7ARClaimGoSourceKey(goSources),
		goSources:    s7ARCloneStringMap(goSources),
		goSections:   sections,
		sourceClaims: map[string]s7ARClaimSourceCache{},
		derived:      map[string]*s7ARClaimSourceState{},
	}
	for name, source := range sources {
		sourceSections := sections[name]
		if !strings.HasSuffix(name, ".go") {
			sourceSections = s7ARClaimSections(source)
		}
		state.sourceClaims[name] = s7ARClaimSourceCache{
			key:    s7ARClaimSourceCacheKey(name, source),
			claims: s7ARClaimsForSource(name, sourceSections),
		}
	}
	return state, nil
}

func s7ARClaimStateForSources(
	sources map[string]string,
	state *s7ARClaimSourceState,
) (*s7ARClaimSourceState, error) {
	goSources := s7ARClaimGoSources(sources)
	key := s7ARClaimGoSourceKey(goSources)
	if state == nil {
		return s7ARPrepareClaimSourceState(sources)
	}
	if state.goKey == key {
		return state, nil
	}
	if cached := state.derived[key]; cached != nil {
		return cached, nil
	}
	incremental, ok, err := s7ARIncrementalClaimSourceState(
		sources, goSources, key, state,
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s7ARPrepareClaimSourceState(sources)
	}
	if state.derived == nil {
		state.derived = map[string]*s7ARClaimSourceState{}
	}
	state.derived[key] = incremental
	return incremental, nil
}

func s7ARIncrementalClaimSourceState(
	sources map[string]string,
	goSources map[string]string,
	key string,
	state *s7ARClaimSourceState,
) (*s7ARClaimSourceState, bool, error) {
	for name := range state.goSources {
		if _, present := goSources[name]; !present {
			return nil, false, nil
		}
	}
	groups := map[string]map[string]string{}
	changed := map[string]bool{}
	affected := map[string]bool{}
	for name, source := range goSources {
		previous, present := state.goSources[name]
		if present && source == previous {
			continue
		}
		directory := filepath.ToSlash(filepath.Dir(name))
		if groups[directory] == nil {
			groups[directory] = map[string]string{}
		}
		changed[name] = true
	}
	if len(changed) == 0 {
		return nil, false, nil
	}
	for name, source := range goSources {
		directory := filepath.ToSlash(filepath.Dir(name))
		if groups[directory] != nil {
			groups[directory][name] = source
			affected[name] = true
		}
	}
	sections := make(map[string][]s7ARClaimSection, len(state.goSections)+len(changed))
	for name, current := range state.goSections {
		sections[name] = append([]s7ARClaimSection(nil), current...)
	}
	for _, group := range groups {
		added, sectionErr := s7ARClaimSectionsForSelectedGoSources(
			group, nil,
		)
		if sectionErr != nil {
			return nil, false, sectionErr
		}
		for name, current := range added {
			sections[name] = append([]s7ARClaimSection(nil), current...)
		}
	}
	sourceClaims := make(
		map[string]s7ARClaimSourceCache, len(state.sourceClaims),
	)
	for name, cached := range state.sourceClaims {
		sourceClaims[name] = cached
	}
	result := &s7ARClaimSourceState{
		goKey:        key,
		goSources:    s7ARCloneStringMap(goSources),
		goSections:   sections,
		sourceClaims: sourceClaims,
		derived:      map[string]*s7ARClaimSourceState{},
	}
	for name, source := range sources {
		if !affected[name] {
			continue
		}
		result.sourceClaims[name] = s7ARClaimSourceCache{
			key:    s7ARClaimSourceCacheKey(name, source),
			claims: s7ARClaimsForSource(name, sections[name]),
		}
	}
	return result, true, nil
}

func s7ARCloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for name, body := range source {
		cloned[name] = body
	}
	return cloned
}

func s7ARClaimSourceCacheKey(name, source string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(strconv.Itoa(len(name))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(name))
	_, _ = hash.Write([]byte(strconv.Itoa(len(source))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(source))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func s7ARClaimSectionsForSources(
	sources map[string]string,
) (map[string][]s7ARClaimSection, error) {
	state, err := s7ARPrepareClaimSourceState(sources)
	if err != nil {
		return nil, err
	}
	return s7ARClaimSectionsForSourcesWithState(sources, state)
}

func s7ARClaimSectionsForSourcesWithState(
	sources map[string]string,
	state *s7ARClaimSourceState,
) (map[string][]s7ARClaimSection, error) {
	result := make(map[string][]s7ARClaimSection, len(sources))
	for name, source := range sources {
		if strings.HasSuffix(name, ".go") {
			continue
		}
		result[name] = s7ARClaimSections(source)
	}
	goSources := s7ARClaimGoSources(sources)
	if len(goSources) == 0 {
		return result, nil
	}
	var err error
	state, err = s7ARClaimStateForSources(sources, state)
	if err != nil {
		return nil, err
	}
	for name, sections := range state.goSections {
		result[name] = sections
	}
	return result, nil
}

func s7ARClaimGoSources(sources map[string]string) map[string]string {
	result := map[string]string{}
	for name, source := range sources {
		if strings.HasSuffix(name, ".go") {
			result[name] = source
		}
	}
	return result
}

func s7ARClaimGoSourceKey(sources map[string]string) string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		_, _ = hash.Write([]byte(strconv.Itoa(len(name))))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(name))
		_, _ = hash.Write([]byte(strconv.Itoa(len(sources[name]))))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(sources[name]))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func s7ARClaimSectionsForGoSources(
	goSources map[string]string,
) (map[string][]s7ARClaimSection, error) {
	return s7ARClaimSectionsForSelectedGoSources(goSources, nil)
}

func s7ARClaimSectionsForSelectedGoSources(
	goSources map[string]string,
	selected map[string]bool,
) (map[string][]s7ARClaimSection, error) {
	result := make(map[string][]s7ARClaimSection, len(goSources))
	if len(goSources) == 0 {
		return result, nil
	}
	files := make(map[string]*ast.File, len(goSources))
	names := make([]string, 0, len(goSources))
	for name, source := range goSources {
		file, err := s6ParseSource(name, source)
		if err != nil {
			return nil, fmt.Errorf("parse shipped claim source %s: %w", name, err)
		}
		files[name] = file
		names = append(names, name)
	}
	sort.Strings(names)
	resolver, err := s7ARBuildGoClaimResolver(names, files)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if selected != nil && !selected[name] {
			continue
		}
		sections, sectionErr := s7ARLightweightGoClaimSections(
			name, files[name], resolver,
		)
		if sectionErr != nil {
			return nil, sectionErr
		}
		result[name] = sections
	}
	return result, nil
}

type s7ARGoClaimBinding struct {
	expression ast.Expr
	origin     string
	pkg        string
}

type s7ARGoClaimResolver struct {
	byObject map[*ast.Object]*s7ARGoClaimBinding
	globals  map[string]map[string][]*s7ARGoClaimBinding
}

func s7ARBuildGoClaimResolver(
	names []string,
	files map[string]*ast.File,
) (*s7ARGoClaimResolver, error) {
	resolver := &s7ARGoClaimResolver{
		byObject: map[*ast.Object]*s7ARGoClaimBinding{},
		globals:  map[string]map[string][]*s7ARGoClaimBinding{},
	}
	for _, name := range names {
		file := files[name]
		pkg := filepath.ToSlash(filepath.Dir(name))
		if resolver.globals[pkg] == nil {
			resolver.globals[pkg] = map[string][]*s7ARGoClaimBinding{}
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.CONST {
				continue
			}
			if err := s7ARRecordGoClaimConstDeclaration(
				name, pkg, general, resolver, true,
			); err != nil {
				return nil, err
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			general, ok := node.(*ast.GenDecl)
			if !ok || general.Tok != token.CONST {
				return true
			}
			_ = s7ARRecordGoClaimConstDeclaration(
				name, pkg, general, resolver, false,
			)
			return true
		})
	}
	return resolver, nil
}

func s7ARRecordGoClaimConstDeclaration(
	name string,
	pkg string,
	declaration *ast.GenDecl,
	resolver *s7ARGoClaimResolver,
	global bool,
) error {
	var previous []ast.Expr
	for _, raw := range declaration.Specs {
		spec, ok := raw.(*ast.ValueSpec)
		if !ok {
			continue
		}
		values := spec.Values
		if len(values) == 0 {
			values = previous
		} else {
			previous = values
		}
		for index, identifier := range spec.Names {
			if identifier.Obj == nil || identifier.Name == "_" || len(values) == 0 {
				continue
			}
			valueIndex := index
			if valueIndex >= len(values) {
				valueIndex = len(values) - 1
			}
			binding := &s7ARGoClaimBinding{
				expression: values[valueIndex],
				origin: fmt.Sprintf(
					"%s:const:%d:%s", name, identifier.Pos(), identifier.Name,
				),
				pkg: pkg,
			}
			if existing := resolver.byObject[identifier.Obj]; existing != nil {
				binding = existing
			} else {
				resolver.byObject[identifier.Obj] = binding
			}
			if !global {
				continue
			}
			existing := resolver.globals[pkg][identifier.Name]
			seen := false
			for _, candidate := range existing {
				if candidate == binding {
					seen = true
					break
				}
			}
			if !seen {
				resolver.globals[pkg][identifier.Name] = append(existing, binding)
			}
		}
	}
	return nil
}

func s7ARLightweightGoClaimSections(
	name string,
	file *ast.File,
	resolver *s7ARGoClaimResolver,
) ([]s7ARClaimSection, error) {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) != 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	pkg := filepath.ToSlash(filepath.Dir(name))
	var sections []s7ARClaimSection
	var sectionErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if sectionErr != nil {
			return false
		}
		expression, ok := node.(ast.Expr)
		if !ok || !s7ARGoStringValuePosition(parents[node], expression) {
			return true
		}
		values, complete := s7AREvaluateGoClaimConstants(
			expression, pkg, resolver, map[*s7ARGoClaimBinding]bool{},
		)
		stringsFound := s7ARGoClaimConstantStrings(values)
		if !complete &&
			s7ARGoClaimExpressionMustResolve(expression, parents) {
			sectionErr = fmt.Errorf(
				"shipped claim constant %s:%d has unresolved or cyclic candidates",
				name, expression.Pos(),
			)
			return false
		}
		if len(stringsFound) == 0 {
			return true
		}
		if parent, ok := parents[node].(ast.Expr); ok {
			parentValues, parentComplete := s7AREvaluateGoClaimConstants(
				parent, pkg, resolver, map[*s7ARGoClaimBinding]bool{},
			)
			if parentComplete && len(s7ARGoClaimConstantStrings(parentValues)) != 0 {
				return true
			}
		}
		origin := s7ARLightweightGoClaimOrigin(
			name, pkg, expression, parents, resolver,
		)
		for _, value := range stringsFound {
			sections = append(sections, s7ARClaimSection{
				heading: name + ":folded-string",
				body:    value,
				origin:  origin,
			})
		}
		return false
	})
	if sectionErr != nil {
		return nil, sectionErr
	}
	return sections, nil
}

func s7AREvaluateGoClaimConstants(
	expression ast.Expr,
	pkg string,
	resolver *s7ARGoClaimResolver,
	resolving map[*s7ARGoClaimBinding]bool,
) (map[string]constant.Value, bool) {
	expression = s7ARUnwrapCallExpression(expression)
	switch typed := expression.(type) {
	case *ast.BasicLit:
		value := constant.MakeFromLiteral(typed.Value, typed.Kind, 0)
		if value.Kind() == constant.Unknown {
			return nil, false
		}
		return s7ARSingleGoClaimConstant(value), true
	case *ast.Ident:
		bindings := s7ARResolveGoClaimBindings(typed, pkg, resolver)
		if len(bindings) == 0 {
			return nil, false
		}
		result := map[string]constant.Value{}
		complete := true
		for _, binding := range bindings {
			if resolving[binding] {
				complete = false
				continue
			}
			resolving[binding] = true
			values, resolved := s7AREvaluateGoClaimConstants(
				binding.expression, binding.pkg, resolver, resolving,
			)
			delete(resolving, binding)
			s7ARMergeGoClaimConstants(result, values)
			complete = complete && resolved
		}
		return result, complete && len(result) != 0
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return nil, false
		}
		left, leftOK := s7AREvaluateGoClaimConstants(
			typed.X, pkg, resolver, resolving,
		)
		right, rightOK := s7AREvaluateGoClaimConstants(
			typed.Y, pkg, resolver, resolving,
		)
		result := map[string]constant.Value{}
		complete := leftOK && rightOK
		for _, leftValue := range left {
			for _, rightValue := range right {
				if leftValue.Kind() != rightValue.Kind() ||
					(leftValue.Kind() != constant.String &&
						leftValue.Kind() != constant.Int) {
					complete = false
					continue
				}
				value := constant.BinaryOp(leftValue, token.ADD, rightValue)
				if value.Kind() == constant.Unknown {
					complete = false
					continue
				}
				s7ARAddGoClaimConstant(result, value)
			}
		}
		return result, complete && len(result) != 0
	case *ast.CallExpr:
		identifier, ok := s7ARUnwrapCallExpression(typed.Fun).(*ast.Ident)
		if !ok || identifier.Name != "string" || len(typed.Args) != 1 {
			return nil, false
		}
		values, complete := s7AREvaluateGoClaimConstants(
			typed.Args[0], pkg, resolver, resolving,
		)
		result := map[string]constant.Value{}
		for _, value := range values {
			converted, ok := s7ARConvertGoClaimConstantToString(value)
			if !ok {
				complete = false
				continue
			}
			s7ARAddGoClaimConstant(result, converted)
		}
		return result, complete && len(result) != 0
	case *ast.UnaryExpr:
		if typed.Op != token.ADD && typed.Op != token.SUB && typed.Op != token.XOR {
			return nil, false
		}
		values, complete := s7AREvaluateGoClaimConstants(
			typed.X, pkg, resolver, resolving,
		)
		result := map[string]constant.Value{}
		for _, value := range values {
			if value.Kind() != constant.Int {
				complete = false
				continue
			}
			s7ARAddGoClaimConstant(result, constant.UnaryOp(typed.Op, value, 0))
		}
		return result, complete && len(result) != 0
	default:
		return nil, false
	}
}

func s7ARSingleGoClaimConstant(value constant.Value) map[string]constant.Value {
	result := map[string]constant.Value{}
	s7ARAddGoClaimConstant(result, value)
	return result
}

func s7ARAddGoClaimConstant(
	target map[string]constant.Value,
	value constant.Value,
) {
	if value == nil || value.Kind() == constant.Unknown {
		return
	}
	key := fmt.Sprintf("%d:%s", value.Kind(), value.ExactString())
	target[key] = value
}

func s7ARMergeGoClaimConstants(
	target map[string]constant.Value,
	source map[string]constant.Value,
) {
	for _, value := range source {
		s7ARAddGoClaimConstant(target, value)
	}
}

func s7ARGoClaimConstantStrings(
	values map[string]constant.Value,
) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		if value.Kind() != constant.String {
			continue
		}
		text := constant.StringVal(value)
		if seen[text] {
			continue
		}
		seen[text] = true
		result = append(result, text)
	}
	sort.Strings(result)
	return result
}

func s7ARConvertGoClaimConstantToString(
	value constant.Value,
) (constant.Value, bool) {
	switch value.Kind() {
	case constant.String:
		return value, true
	case constant.Int:
		integer, ok := constant.Int64Val(value)
		if !ok {
			return nil, false
		}
		character := rune(integer)
		if !utf8.ValidRune(character) {
			character = utf8.RuneError
		}
		return constant.MakeString(string(character)), true
	default:
		return nil, false
	}
}

func s7ARResolveGoClaimBindings(
	identifier *ast.Ident,
	pkg string,
	resolver *s7ARGoClaimResolver,
) []*s7ARGoClaimBinding {
	if identifier.Obj != nil {
		if binding := resolver.byObject[identifier.Obj]; binding != nil {
			return []*s7ARGoClaimBinding{binding}
		}
		return nil
	}
	candidates := resolver.globals[pkg][identifier.Name]
	result := append([]*s7ARGoClaimBinding(nil), candidates...)
	sort.Slice(result, func(left, right int) bool {
		return result[left].origin < result[right].origin
	})
	return result
}

func s7ARResolveUniqueGoClaimBinding(
	identifier *ast.Ident,
	pkg string,
	resolver *s7ARGoClaimResolver,
) *s7ARGoClaimBinding {
	candidates := s7ARResolveGoClaimBindings(identifier, pkg, resolver)
	if len(candidates) == 1 {
		return candidates[0]
	}
	return nil
}

func s7ARGoClaimExpressionMustResolve(
	expression ast.Expr,
	parents map[ast.Node]ast.Node,
) bool {
	spec, ok := parents[expression].(*ast.ValueSpec)
	if !ok {
		return false
	}
	declaration, ok := parents[spec].(*ast.GenDecl)
	if !ok || declaration.Tok != token.CONST {
		return false
	}
	if identifier, ok := spec.Type.(*ast.Ident); ok && identifier.Name == "string" {
		return true
	}
	mustResolve := false
	ast.Inspect(expression, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if ok && (literal.Kind == token.STRING || literal.Kind == token.CHAR) {
			mustResolve = true
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if ok {
			identifier, _ := s7ARUnwrapCallExpression(call.Fun).(*ast.Ident)
			if identifier != nil && identifier.Name == "string" {
				mustResolve = true
				return false
			}
		}
		return !mustResolve
	})
	return mustResolve
}

func s7ARLightweightGoClaimOrigin(
	name string,
	pkg string,
	expression ast.Expr,
	parents map[ast.Node]ast.Node,
	resolver *s7ARGoClaimResolver,
) string {
	if spec, ok := parents[expression].(*ast.ValueSpec); ok {
		for index, value := range spec.Values {
			if value != expression {
				continue
			}
			nameIndex := index
			if nameIndex >= len(spec.Names) {
				nameIndex = len(spec.Names) - 1
			}
			if nameIndex >= 0 {
				if binding := resolver.byObject[spec.Names[nameIndex].Obj]; binding != nil {
					return s7ARGoClaimBindingOrigin(
						binding, resolver, map[*s7ARGoClaimBinding]bool{},
					)
				}
			}
		}
	}
	if identifier, ok := s7ARUnwrapCallExpression(expression).(*ast.Ident); ok {
		if binding := s7ARResolveUniqueGoClaimBinding(identifier, pkg, resolver); binding != nil {
			return s7ARGoClaimBindingOrigin(
				binding, resolver, map[*s7ARGoClaimBinding]bool{},
			)
		}
	}
	return fmt.Sprintf("%s:expression:%d", name, expression.Pos())
}

func s7ARGoClaimBindingOrigin(
	binding *s7ARGoClaimBinding,
	resolver *s7ARGoClaimResolver,
	resolving map[*s7ARGoClaimBinding]bool,
) string {
	if binding == nil || resolving[binding] {
		return ""
	}
	resolving[binding] = true
	defer delete(resolving, binding)
	identifier, ok := s7ARUnwrapCallExpression(binding.expression).(*ast.Ident)
	if ok {
		if target := s7ARResolveUniqueGoClaimBinding(identifier, binding.pkg, resolver); target != nil {
			return s7ARGoClaimBindingOrigin(target, resolver, resolving)
		}
	}
	return binding.origin
}

func s7ARTypedGoClaimSections(
	name string,
	file *ast.File,
	model *s6SourceTypeModel,
	constInitializers map[*types.Const]ast.Expr,
) ([]s7ARClaimSection, error) {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) != 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	var sections []s7ARClaimSection
	ast.Inspect(file, func(node ast.Node) bool {
		expression, ok := node.(ast.Expr)
		if !ok || !s7ARGoStringValuePosition(parents[node], expression) {
			return true
		}
		typed, present := model.expressionTypes[expression]
		if !present || typed.Value == nil || typed.Value.Kind() != constant.String {
			return true
		}
		if parent, parentOK := parents[node].(ast.Expr); parentOK {
			parentTyped, parentPresent := model.expressionTypes[parent]
			if parentPresent && parentTyped.Value != nil &&
				parentTyped.Value.Kind() == constant.String {
				return true
			}
		}
		if identifier, identifierOK := expression.(*ast.Ident); identifierOK {
			if _, constantUse := model.uses[identifier].(*types.Const); constantUse {
				if _, declaration := parents[node].(*ast.ValueSpec); !declaration {
					return true
				}
			}
		}
		value := constant.StringVal(typed.Value)
		sections = append(sections, s7ARClaimSection{
			heading: name + ":folded-string",
			body:    value,
			origin: s7ARGoStringOrigin(
				name, expression, parents, model, constInitializers, map[*types.Const]bool{},
			),
		})
		return false
	})
	return sections, nil
}

func s7ARGoConstInitializers(
	model *s6SourceTypeModel,
) map[*types.Const]ast.Expr {
	result := map[*types.Const]ast.Expr{}
	for _, file := range model.files {
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for index, name := range spec.Names {
				object, ok := model.definitions[name].(*types.Const)
				if !ok || len(spec.Values) == 0 {
					continue
				}
				valueIndex := index
				if len(spec.Values) == 1 {
					valueIndex = 0
				}
				if valueIndex < len(spec.Values) {
					result[object] = spec.Values[valueIndex]
				}
			}
			return false
		})
	}
	return result
}

func s7ARGoStringOrigin(
	name string,
	expression ast.Expr,
	parents map[ast.Node]ast.Node,
	model *s6SourceTypeModel,
	constInitializers map[*types.Const]ast.Expr,
	resolving map[*types.Const]bool,
) string {
	expression = s7ARUnwrapCallExpression(expression)
	if identifier, ok := expression.(*ast.Ident); ok {
		object, ok := model.uses[identifier].(*types.Const)
		if !ok {
			object, _ = model.definitions[identifier].(*types.Const)
		}
		if object != nil {
			return s7ARGoConstOrigin(
				object, name, parents, model, constInitializers, resolving,
			)
		}
	}
	if spec, ok := parents[expression].(*ast.ValueSpec); ok {
		for index, value := range spec.Values {
			if value != expression {
				continue
			}
			nameIndex := index
			if len(spec.Values) == 1 {
				nameIndex = 0
			}
			if nameIndex < len(spec.Names) {
				if object, ok := model.definitions[spec.Names[nameIndex]].(*types.Const); ok {
					return s7ARGoConstOrigin(
						object, name, parents, model, constInitializers, resolving,
					)
				}
			}
		}
	}
	return fmt.Sprintf("%s:expression:%d", name, expression.Pos())
}

func s7ARGoConstOrigin(
	object *types.Const,
	name string,
	parents map[ast.Node]ast.Node,
	model *s6SourceTypeModel,
	constInitializers map[*types.Const]ast.Expr,
	resolving map[*types.Const]bool,
) string {
	if resolving[object] {
		return fmt.Sprintf("%s:const:%d", name, object.Pos())
	}
	resolving[object] = true
	defer delete(resolving, object)
	initializer := s7ARUnwrapCallExpression(constInitializers[object])
	if identifier, ok := initializer.(*ast.Ident); ok {
		if target, ok := model.uses[identifier].(*types.Const); ok {
			return s7ARGoConstOrigin(
				target, name, parents, model, constInitializers, resolving,
			)
		}
	}
	position := model.fileSet.Position(object.Pos())
	return fmt.Sprintf("%s:const:%d:%d", filepath.ToSlash(position.Filename),
		position.Line, position.Column)
}

func s7ARGoStringValuePosition(parent ast.Node, expression ast.Expr) bool {
	switch typed := parent.(type) {
	case *ast.ValueSpec:
		for _, value := range typed.Values {
			if value == expression {
				return true
			}
		}
		return false
	case *ast.AssignStmt:
		for _, value := range typed.Rhs {
			if value == expression {
				return true
			}
		}
		return false
	case *ast.ReturnStmt:
		for _, value := range typed.Results {
			if value == expression {
				return true
			}
		}
		return false
	case *ast.CallExpr:
		for _, value := range typed.Args {
			if value == expression {
				return true
			}
		}
		return false
	case *ast.BinaryExpr:
		return typed.X == expression || typed.Y == expression
	case *ast.ParenExpr:
		return typed.X == expression
	case *ast.KeyValueExpr:
		return typed.Key == expression || typed.Value == expression
	case *ast.CompositeLit:
		for _, value := range typed.Elts {
			if value == expression {
				return true
			}
		}
		return false
	case *ast.UnaryExpr:
		return typed.X == expression
	default:
		return false
	}
}

func s7ARClaimSections(source string) []s7ARClaimSection {
	var sections []s7ARClaimSection
	current := s7ARClaimSection{heading: "<preamble>"}
	flush := func(section s7ARClaimSection) {
		for _, paragraph := range strings.Split(section.body, "\n\n") {
			if strings.TrimSpace(paragraph) == "" {
				continue
			}
			sections = append(sections, s7ARClaimSection{
				heading: section.heading,
				body:    paragraph,
			})
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			if strings.TrimSpace(current.body) != "" {
				flush(current)
			}
			current = s7ARClaimSection{heading: strings.TrimSpace(line)}
			continue
		}
		current.body += line + "\n"
	}
	if strings.TrimSpace(current.body) != "" {
		flush(current)
	}
	return sections
}

func s7ARClaimSentences(body string) []string {
	normalized := strings.Join(strings.Fields(body), " ")
	if normalized == "" {
		return nil
	}
	var result []string
	start := 0
	for index := 0; index < len(normalized); index++ {
		switch normalized[index] {
		case '.', '!', '?':
		default:
			continue
		}
		next := index + 1
		for next < len(normalized) && strings.ContainsRune("*_`\"')]", rune(normalized[next])) {
			next++
		}
		if next < len(normalized) && normalized[next] != ' ' {
			continue
		}
		sentence := strings.TrimSpace(normalized[start:next])
		if sentence != "" {
			result = append(result, sentence)
		}
		for next < len(normalized) && normalized[next] == ' ' {
			next++
		}
		start = next
		index = next - 1
	}
	if tail := strings.TrimSpace(normalized[start:]); tail != "" {
		result = append(result, tail)
	}
	return result
}

var (
	s7ARPermanentSubjectRE = regexp.MustCompile(
		`\b(slug|archive|transaction|exit[- ]?6|crash|interrupted (?:prepare|purge|run))\b`,
	)
	s7ARExitSixNeverTerminalRE = regexp.MustCompile(
		`\bexit[- ]?6\b[^.!?]{0,40}\bnever terminal\b`,
	)
	s7ARBlockedWithoutRouteRE = regexp.MustCompile(
		`\b(?:blocked|stranded)\b[^.!?]{0,80}\bwithout (?:an? )?(?:named|applicable|executable) route\b`,
	)
	s7ARInterruptedPermanentRE = regexp.MustCompile(
		`\bno (?:crash|interrupted (?:prepare|purge|run))\b[^.!?]{0,100}\bleaves?\b[^.!?]{0,80}\b(?:slug|archive|transaction)\b[^.!?]{0,40}\b(?:permanently )?(?:blocked|stranded|unrecoverable|unable to recover)\b`,
	)
	s7ARNeverUnrecoverableRE = regexp.MustCompile(
		`\b(?:the )?(?:slug|archive|transaction)\s+` +
			`(?:is never|cannot|can never|will never|must never)\s+` +
			`(?:be |become |remain )?(?:permanently )?` +
			`(?:blocked|stranded|unrecoverable|unable to recover)\b`,
	)
	s7ARPositiveRecoverabilityRE = regexp.MustCompile(
		`\b(?:(?:the )?(?:slug|archive|transaction)\s+` +
			`(?:(?:can|will|must)\s+always\s+(?:be\s+)?(?:recovered|restored|unblocked|recoverable)|` +
			`(?:is|remains)\s+always\s+recoverable)|` +
			`(?:operators?|users?|you|we|tpatch)\s+` +
			`(?:can|will|must)\s+always\s+(?:recover|restore|unblock)\s+` +
			`(?:the )?(?:slug|archive|transaction)|` +
			`recovery of (?:the )?(?:slug|archive|transaction)\s+` +
			`(?:is|remains)\s+always\s+possible|` +
			`it is impossible to (?:block|strand)\s+` +
			`(?:the )?(?:slug|archive|transaction)|` +
			`(?:the )?(?:slug|archive|transaction)\s+` +
			`is impossible to (?:block|strand))\b`,
	)
	s7ARActiveAntiStrandingRE = regexp.MustCompile(
		`\b(?:(?:operators?|users?|you|we|tpatch)\s+` +
			`(?:(?:can|will|must)\s+never|cannot|will not|must not)\s+` +
			`(?:permanently\s+)?(?:block|strand)\s+(?:the\s+)?` +
			`(?:slug|archive|transaction)|` +
			`never\s+(?:can|will|must)\s+` +
			`(?:operators?|users?|you|we|tpatch)\s+` +
			`(?:permanently\s+)?(?:block|strand)\s+(?:the\s+)?` +
			`(?:slug|archive|transaction)|` +
			`(?:operators?|users?|you|we|tpatch)\s+` +
			`(?:(?:can|will|must)\s+never|cannot|will not|must not)\s+leave\s+` +
			`(?:the\s+)?(?:slug|archive|transaction)\s+` +
			`(?:permanently\s+)?(?:blocked|stranded))\b`,
	)
	s7ARNegatedRecoverabilityRE = regexp.MustCompile(
		`\b(?:cannot|can never|not always|never|is not|isn't|not impossible)\b` +
			`[^.!?]{0,80}\b(?:recover|restore|unblock|possible|impossible)\w*\b`,
	)
	s7ARNegatedAntiStrandingRE = regexp.MustCompile(
		`\bno\s+(?:operators?|users?)\s+` +
			`(?:(?:can|will|must)\s+never|cannot|will not|must not)\s+` +
			`(?:permanently\s+)?(?:block|strand|leave)\b`,
	)
	s7ARRelevantPermanenceRE = regexp.MustCompile(
		`\b(?:operators?|users?|you|we|tpatch|it|` +
			`(?:the )?(?:slug|archive|transaction)|` +
			`recovery of (?:the )?(?:slug|archive|transaction))\b` +
			`[^.!?]{0,40}\b(?:always|impossible)\b[^.!?]{0,40}` +
			`\b(?:recover|restore|unblock|block|strand|possible)\w*\b|` +
			`\b(?:operators?|users?|you|we|tpatch|it|` +
			`(?:the )?(?:slug|archive|transaction)|` +
			`recovery of (?:the )?(?:slug|archive|transaction))\b` +
			`[^.!?]{0,40}\b(?:recover|restore|unblock|block|strand|possible)\w*\b` +
			`[^.!?]{0,40}\b(?:always|impossible)\b`,
	)
	s7ARUnqualifiedRecoverabilityClaimRE = regexp.MustCompile(
		`\bclaims?\b[^.!?]{0,60}\bunqualified\b[^.!?]{0,30}\balways recoverable\b`,
	)
	s7AREveryExitSixRouteRE = regexp.MustCompile(
		`\bevery exit[- ]?6 population\b[^.!?]{0,80}\bexactly one\b[^.!?]{0,80}\broute\b`,
	)
	s7ARHistoricalClaimRE = regexp.MustCompile(
		`^(?:rev-[0-9]+|an earlier revision|the prior revision)\s+(?:said|claimed|stated|described)\b`,
	)
	s7ARRouteGrammarAffirmativeRE = regexp.MustCompile(
		`\b(?:is qualified by an executable route|` +
			`route is always the one|` +
			`states which route belongs|` +
			`every refusal names either the command|` +
			`refusal names (?:either )?(?:the )?(?:command|manual procedure|route)|` +
			`escape is reachable through|` +
			`every exit[- ]?6 population has exactly one (?:(?:applicable|named|executable),? ){1,3}route|` +
			`is routed to (?:its|the) own (?:archive )?procedure|` +
			`route belongs to|` +
			`no (?:crash|interrupted (?:prepare|purge|run)) leaves? (?:a )?` +
			`(?:slug|archive|transaction) blocked without a named(?:, applicable)? route out)\b`,
	)
	s7ARRouteNounAffirmativeRE = regexp.MustCompile(
		`(?:the |this |one |named |exact |executable |manual |archive |repo-relative )*` +
			`(?:route|repair|command|procedure|escape) ` +
			`(?:is|is to|uses|runs|executes|removes|restores|is the literal)$`,
	)
	s7ARRouteNonAffirmativeContextRE = regexp.MustCompile(
		`\b(?:if|when|unless|assuming|supposing|provided|whenever)\b|` +
			`\b(?:subject to|only if|except(?: when| if)?|until)\b|` +
			`\b(?:could|might|would)\s+(?:safely\s+)?(?:run|rerun|retry|use|execute|remove|restore)\b|` +
			`\b(?:cannot|can't|mustn't|shouldn't)\b|` +
			`\b(?:must|should|may|can|is|are|do|does)\s+not\b|` +
			`\bnot\s+(?:permitted|allowed|authorized)\b|` +
			`\b(?:forbidden|prohibited|disallowed|barred|proscribed)\b|` +
			`\bno\s+(?:operator|route|command|procedure|escape)\b|` +
			`\b(?:for example|example[- ]only|hypothetically)\b|` +
			`\b(?:documentation|document|docs|example|text)\s+(?:says|states|shows|quotes)\b`,
	)
	s7ARNonAssertiveClaimStartRE = regexp.MustCompile(
		`^(?:for example|example|hypothetically|suppose|if|when|` +
			`in the future|eventually|aspirationally|once)\b`,
	)
	s7ARNonAssertiveClaimPrefixRE = regexp.MustCompile(
		`\b(?:could|might|would)\b|` +
			`\b(?:plan|plans|planned|aim|aims|aspire|aspires|hope|hopes)\s+to\b|` +
			`\b(?:someday|eventually|in the future)\b|` +
			`\b(?:documentation|document|docs|example|text|guide|manual|policy|comment|speaker)\s+` +
			`(?:says|said|states|stated|shows|showed|quotes|quoted)\b`,
	)
	s7ARRouteGrammarBadContextRE = regexp.MustCompile(
		`\b(?:if|when|unless|assuming|supposing|provided|whenever|` +
			`could|might|would|forbidden|prohibited|disallowed|barred|` +
			`proscribed|for example|hypothetically)\b|` +
			`\bnot\s+(?:permitted|allowed|authorized)\b|` +
			`\b(?:documentation|document|docs|example|text)\s+` +
			`(?:says|states|shows|quotes)\b`,
	)
	s7ARRouteImmediateNegativeRE = regexp.MustCompile(
		`\b(?:no|not|never|neither|without)(?:\s+the)?\s*$`,
	)
	s7ARReferencePrefixRE = regexp.MustCompile(
		`^(?:(?:(?:the|this|that) (?:claim|route|procedure|escape) (?:is )?)?` +
			`(?:under|defined by|specified by|governed by|routed by|` +
			`route in|procedure in)|see|per)$`,
	)
	s7ARRouteSelectorIDRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func s7ARIsPermanentClaim(lower string) bool {
	if s7ARNonAssertivePermanentClaim(lower) {
		return false
	}
	if s7ARNegatedAntiStrandingRE.MatchString(lower) {
		return false
	}
	subject := s7ARPermanentSubjectRE.MatchString(lower)
	knownClaim := s7ARUnqualifiedRecoverabilityClaimRE.MatchString(lower) ||
		s7ARPositiveRecoverabilityRE.MatchString(lower) ||
		s7ARActiveAntiStrandingRE.MatchString(lower) ||
		s7ARNegativeQuantifierAntiStranding(lower) ||
		s7ARExitSixNeverTerminalRE.MatchString(lower) ||
		s7ARBlockedWithoutRouteRE.MatchString(lower) ||
		s7ARInterruptedPermanentRE.MatchString(lower) ||
		s7ARNeverUnrecoverableRE.MatchString(lower) ||
		strings.Contains(lower, "must never be a permanent state") ||
		s7AREveryExitSixRouteRE.MatchString(lower)
	if subject && knownClaim {
		return true
	}
	return subject &&
		!s7ARNegatedRecoverabilityRE.MatchString(lower) &&
		s7ARRelevantPermanenceRE.MatchString(lower)
}

func s7ARNegativeQuantifierAntiStranding(lower string) bool {
	normalized := strings.TrimSpace(lower)
	if normalized == "" ||
		strings.ContainsAny(normalized, "\"'`“”‘’;\n\r:") {
		return false
	}
	normalized = strings.TrimSpace(strings.TrimRight(normalized, ".!?"))
	if strings.ContainsAny(normalized, ".!?") {
		return false
	}
	replacer := strings.NewReplacer(
		",", " ",
		"(", " ",
		")", " ",
		"—", " ",
		"–", " ",
	)
	tokens := strings.Fields(replacer.Replace(normalized))
	if len(tokens) < 6 || tokens[0] != "no" ||
		!s7ARStringMember(tokens[1], "operator", "operators", "user", "users") {
		return false
	}
	index := 2
	if index < len(tokens) && tokens[index] == "ever" {
		index++
	}
	if index >= len(tokens) ||
		!s7ARStringMember(tokens[index], "can", "will", "must") {
		return false
	}
	index++
	if index < len(tokens) && tokens[index] == "ever" {
		index++
	}
	if index < len(tokens) && tokens[index] == "permanently" {
		index++
	}
	if index >= len(tokens) {
		return false
	}
	verb := tokens[index]
	index++
	if verb == "leave" {
		if index < len(tokens) && tokens[index] == "the" {
			index++
		}
		if index >= len(tokens) ||
			!s7ARStringMember(tokens[index], "slug", "archive", "transaction") {
			return false
		}
		index++
		if index < len(tokens) && tokens[index] == "permanently" {
			index++
		}
		return index+1 == len(tokens) &&
			s7ARStringMember(tokens[index], "blocked", "stranded")
	}
	if !s7ARStringMember(verb, "block", "strand") {
		return false
	}
	if index < len(tokens) && tokens[index] == "the" {
		index++
	}
	return index+1 == len(tokens) &&
		s7ARStringMember(tokens[index], "slug", "archive", "transaction")
}

func s7ARStringMember(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func s7ARNonAssertivePermanentClaim(lower string) bool {
	normalized := strings.TrimSpace(strings.Trim(lower, "`*_()[]{}\"'"))
	subject := s7ARPermanentSubjectRE.FindStringIndex(normalized)
	if subject == nil {
		return false
	}
	prefix := normalized[:subject[0]]
	if s7ARNonAssertiveClaimStartRE.MatchString(normalized) ||
		s7ARNonAssertiveClaimPrefixRE.MatchString(prefix) {
		return true
	}
	return false
}

func s7ARHistoricalClaimSentence(lower string) bool {
	return s7ARHistoricalClaimRE.MatchString(strings.TrimSpace(lower))
}

func s7ARExcludedClaimSection(heading, lower string) bool {
	heading = strings.ToLower(heading)
	if s7ARContainsAny(
		heading,
		"revision history", "risks", "acceptance matrix",
		"18.", "sensitivity requirement", "alternatives considered",
	) {
		return true
	}
	return strings.Contains(lower, "this section is a sensitivity fixture only") ||
		strings.Contains(lower, "| rev | disposition | what changed |")
}

func s7ARClaimHasExecutableRoute(section string) bool {
	exactAbandon := s7ARHasAffirmativeRouteCommand(section, "abandon")
	exactPurge := s7ARHasAffirmativeRouteCommand(section, "purge")
	exactManual := s7ARHasAffirmativeRouteCommand(section, "manual")
	exactIndex := strings.Contains(strings.ToLower(section), "restore") &&
		strings.Contains(section, "index.json") &&
		strings.Contains(strings.ToLower(section), "rerun") &&
		s7ARHasAffirmativeRouteMention(section, "restore")
	return exactAbandon || exactPurge || exactManual || exactIndex
}

func s7ARHasAffirmativeRouteCommand(section, kind string) bool {
	lower := strings.ToLower(section)
	needle := map[string]string{
		"abandon": "tpatch prepare ",
		"purge":   "tpatch feature intent-archive purge ",
		"manual":  "rm -rf ",
	}[kind]
	if needle == "" {
		return false
	}
	for offset := 0; ; {
		found := strings.Index(lower[offset:], needle)
		if found < 0 {
			return false
		}
		found += offset
		command, commandStart, commandEnd := s7ARRouteCommandSpan(
			section, found,
		)
		valid := false
		switch kind {
		case "abandon":
			valid = s7ARValidAbandonRouteArgv(command)
		case "purge":
			valid = s7ARValidPurgeRouteArgv(command)
		case "manual":
			valid = s7ARValidManualRouteArgv(command)
		}
		if valid {
			clauseStart, clauseEnd := 0, 0
			if s7ARRouteCommandInCodeBlock(section, commandStart) {
				clauseStart = strings.LastIndex(section[:commandStart], "\n") + 1
				clauseEnd = commandEnd
			} else {
				clauseStart, clauseEnd = s7ARRouteClauseBounds(
					lower, commandStart, commandEnd,
				)
			}
			clause := lower[clauseStart:clauseEnd]
			tokenOffset := commandStart - clauseStart
			prefix := clause[:tokenOffset]
			if s7ARAffirmativeRouteClause(
				clause, prefix, strings.ToLower(command), tokenOffset,
			) {
				return true
			}
		}
		offset = found + len(needle)
	}
}

func s7ARRouteCommandInCodeBlock(source string, start int) bool {
	return start >= 0 && start <= len(source) &&
		strings.Count(source[:start], "```")%2 == 1
}

func s7ARRouteCommandSpan(source string, start int) (string, int, int) {
	if s7ARRouteCommandInCodeBlock(source, start) {
		end := len(source)
		if lineEnd := strings.IndexByte(source[start:], '\n'); lineEnd >= 0 {
			end = start + lineEnd
		}
		command := strings.TrimSpace(source[start:end])
		return command, start, start + len(command)
	}
	backtickStart := strings.LastIndex(source[:start], "`")
	if backtickStart >= 0 &&
		strings.Count(source[:start], "`")%2 == 1 {
		if closeOffset := strings.Index(source[start:], "`"); closeOffset >= 0 {
			end := start + closeOffset
			return strings.TrimSpace(source[backtickStart+1 : end]),
				backtickStart + 1, end
		}
	}
	end := len(source)
	for index := start; index < len(source); index++ {
		switch source[index] {
		case '\n', '\r', ';', '|':
			end = index
			index = len(source)
		case '.', '!', '?':
			if s7ARSentenceBoundaryAt(source, index) {
				end = index
				index = len(source)
			}
		default:
			if strings.HasPrefix(source[index:], " —") {
				end = index
				index = len(source)
			}
		}
	}
	command := strings.TrimSpace(strings.Trim(source[start:end], "`*_()[]{}\"'"))
	return command, start, start + len(command)
}

func s7ARValidAbandonRouteArgv(command string) bool {
	fields := strings.Fields(command)
	return s7ARRouteCommandBytesAreExact(command, fields) &&
		len(fields) == 4 &&
		fields[0] == "tpatch" &&
		fields[1] == "prepare" &&
		s7ARValidRouteSlug(fields[2]) &&
		fields[3] == "--abandon-transaction"
}

func s7ARValidPurgeRouteArgv(command string) bool {
	fields := strings.Fields(command)
	if !s7ARRouteCommandBytesAreExact(command, fields) ||
		len(fields) < 7 ||
		fields[0] != "tpatch" ||
		fields[1] != "feature" ||
		fields[2] != "intent-archive" ||
		fields[3] != "purge" ||
		!s7ARValidRouteSlug(fields[4]) ||
		fields[len(fields)-1] != "--yes" {
		return false
	}
	selector := fields[5 : len(fields)-1]
	switch selector[0] {
	case "--all", "--orphans":
		return len(selector) == 1
	case "--generation":
		return len(selector) == 2 && s7ARValidRouteGeneration(selector[1])
	case "--blob":
		if len(selector)%2 != 0 {
			return false
		}
		for index := 0; index < len(selector); index += 2 {
			if selector[index] != "--blob" ||
				!s7ARValidRouteBlob(selector[index+1]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func s7ARValidManualRouteArgv(command string) bool {
	fields := strings.Fields(command)
	if !s7ARRouteCommandBytesAreExact(command, fields) ||
		(len(fields) != 3 && len(fields) != 4) {
		return false
	}
	if fields[0] != "rm" || fields[1] != "-rf" {
		return false
	}
	pathIndex := 2
	if len(fields) == 4 {
		if fields[2] != "--" {
			return false
		}
		pathIndex = 3
	}
	target := fields[pathIndex]
	if target == "" ||
		strings.HasPrefix(target, "/") ||
		strings.Contains(target, "//") {
		return false
	}
	hasTrailingSlash := strings.HasSuffix(target, "/")
	segments := strings.Split(strings.TrimSuffix(target, "/"), "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	if len(segments) == 4 &&
		reflect.DeepEqual(segments[:3], []string{".tpatch", "local", "intent-prepare"}) &&
		s7ARValidRouteSlug(segments[3]) {
		return len(fields) == 3 && hasTrailingSlash
	}
	if len(segments) != 7 ||
		hasTrailingSlash ||
		!reflect.DeepEqual(
			segments[:2], []string{".tpatch", "features"},
		) ||
		!s7ARValidRouteSlug(segments[2]) ||
		!reflect.DeepEqual(
			segments[3:6], []string{"artifacts", "intent-archive", "blobs"},
		) ||
		len(fields) != 4 || fields[2] != "--" {
		return false
	}
	blob := segments[6]
	if blob == "<hash>.blob" {
		return true
	}
	hash := strings.TrimSuffix(blob, ".blob")
	return strings.HasSuffix(blob, ".blob") &&
		len(hash) == 64 &&
		regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash)
}

func s7ARRouteCommandBytesAreExact(command string, fields []string) bool {
	if len(fields) == 0 || command != strings.Join(fields, " ") {
		return false
	}
	return !strings.ContainsAny(command, "*?[]\\$;&|'\"`{}()~#\r\n\t")
}

func s7ARValidRouteSlug(value string) bool {
	if value == "<slug>" {
		return true
	}
	canonical, err := patchintent.CanonicalSlug(value)
	return err == nil && canonical == value
}

func s7ARValidRouteBlob(value string) bool {
	return value == "<hash>" || s7ARRouteSelectorIDRE.MatchString(value)
}

func s7ARValidRouteGeneration(value string) bool {
	return value == "<id>" || s7ARRouteSelectorIDRE.MatchString(value)
}

func s7ARHasAffirmativeRouteMention(sentence, token string) bool {
	lower := strings.ToLower(sentence)
	token = strings.ToLower(token)
	for offset := 0; ; {
		found := strings.Index(lower[offset:], token)
		if found < 0 {
			return false
		}
		found += offset
		clauseStart, clauseEnd := s7ARRouteClauseBounds(
			lower, found, found+len(token),
		)
		clause := lower[clauseStart:clauseEnd]
		tokenOffset := found - clauseStart
		prefix := clause[:tokenOffset]
		if s7ARAffirmativeRouteClause(clause, prefix, token, tokenOffset) {
			return true
		}
		offset = found + len(token)
	}
}

func s7ARRouteClauseBounds(source string, tokenStart, tokenEnd int) (int, int) {
	start := 0
	for index := tokenStart - 1; index >= 0; index-- {
		if source[index] == ';' || source[index] == '|' ||
			s7ARSentenceBoundaryAt(source, index) {
			start = index + 1
			break
		}
	}
	end := len(source)
	for index := tokenEnd; index < len(source); index++ {
		if source[index] == '|' ||
			s7ARSentenceBoundaryAt(source, index) {
			end = index
			break
		}
	}
	return start, end
}

func s7ARSentenceBoundaryAt(source string, index int) bool {
	if index < 0 || index >= len(source) ||
		!strings.ContainsRune(".!?", rune(source[index])) {
		return false
	}
	next := index + 1
	for next < len(source) && strings.ContainsRune("*_`\"')]", rune(source[next])) {
		next++
	}
	return next == len(source) ||
		source[next] == ' ' || source[next] == '\n' ||
		source[next] == '\r' || source[next] == '\t'
}

func s7ARAffirmativeRouteClause(
	clause, prefix, token string,
	tokenOffset int,
) bool {
	if tokenOffset < 0 || tokenOffset+len(token) > len(clause) ||
		s7ARRouteMentionIsQuoted(clause, tokenOffset) {
		return false
	}
	normalizedClause := strings.ToLower(strings.Join(strings.Fields(clause), " "))
	if s7ARRouteNonAffirmativeContextRE.MatchString(normalizedClause) {
		return false
	}
	rawPrefix := clause[:tokenOffset]
	rawSuffix := clause[tokenOffset+len(token):]
	prefix = strings.TrimSpace(strings.Trim(rawPrefix, "`*_()[]{}\"'"))
	prefix = strings.Join(strings.Fields(prefix), " ")
	if prefix == "" {
		trimmed := strings.TrimLeft(clause, "`*_()[]{}\"' ")
		if !strings.HasPrefix(trimmed, token) {
			return false
		}
	} else if !s7ARAffirmativeRoutePrefix(prefix, token) {
		return false
	}
	return s7ARAffirmativeRouteSuffix(token, rawSuffix)
}

func s7ARAffirmativeRouteSuffix(token, suffix string) bool {
	suffix = strings.TrimSpace(strings.Trim(suffix, "`*_()[]{}\"'"))
	suffix = strings.NewReplacer("`", "", "**", "", "__", "").Replace(suffix)
	suffix = strings.Join(strings.Fields(suffix), " ")
	suffix = strings.TrimSpace(strings.TrimRight(suffix, ".,"))
	if suffix == "" {
		return true
	}
	if token == "restore" {
		return suffix ==
			"an index.json that no longer decodes and remove nothing; then rerun the sanitized purge, whose pending+absent recovery terminally tombstones" ||
			suffix ==
				"index.json from the operator's own version control or backup and rerun" ||
			suffix ==
				"index.json from the operator's own version control or backup and rerun; "+
					"that form names no removal command and no blob path at all, and removing the "+
					"index is never offered, because it would discard every generation record to resolve one hash"
	}
	switch {
	case suffix == "to unblock the slug without touching any canonical file (§6.6)" ||
		suffix == "to unblock the slug without touching any canonical file":
		return true
	case suffix ==
		"— and, where the environment denies that mode, the named repo-relative manual removal":
		return true
	case suffix == ", with the manual fallback below; - archive-purge-evidence-divergent → d16's archive procedure: "+
		"report the pending hash and the repo-relative managed blob and index.json paths, offer to preserve "+
		"the unexpected bytes, remove the divergent managed blob path (or restore an index that stopped "+
		"strict-decoding), then rerun the sanitized purge, whose pending+absent case finalizes the tombstone terminally":
		return true
	default:
		return false
	}
}

func s7ARRouteMentionIsQuoted(clause string, tokenOffset int) bool {
	doubleQuotes := 0
	singleQuotes := 0
	for _, character := range clause[:tokenOffset] {
		switch character {
		case '"', '“', '”':
			doubleQuotes++
		case '\'', '‘', '’':
			singleQuotes++
		}
	}
	return doubleQuotes%2 != 0 || singleQuotes%2 != 0
}

func s7ARAffirmativeRoutePrefix(prefix, token string) bool {
	prefix = strings.TrimSpace(strings.Trim(prefix, "`*_()[]{}\"'"))
	prefix = strings.Join(strings.Fields(prefix), " ")
	if strings.HasSuffix(prefix, "→") || strings.HasSuffix(prefix, "->") {
		return true
	}
	if strings.HasPrefix(token, "rm -rf") &&
		(strings.HasSuffix(prefix, "single type-total") ||
			strings.HasSuffix(prefix, "one type-total") ||
			strings.HasSuffix(prefix, "last-resort")) {
		return true
	}
	if strings.HasSuffix(prefix, "run from the workspace root:") ||
		strings.HasSuffix(prefix, "run from the workspace root") {
		return true
	}
	words := strings.FieldsFunc(prefix, func(character rune) bool {
		return character < 'a' || character > 'z'
	})
	if len(words) == 0 {
		return false
	}
	verb := words[len(words)-1]
	if s7ARContainsAnyWord(
		verb, "run", "rerun", "retry", "use", "execute", "remove", "restore",
	) {
		if len(words) == 1 {
			return true
		}
		if len(words) >= 3 &&
			s7ARContainsAnyWord(words[len(words)-2], "can", "may", "should", "must") &&
			s7ARContainsAnyWord(words[len(words)-3], "operator", "operators", "you") &&
			len(words) == 3 {
			return true
		}
		if len(words) >= 4 && words[len(words)-2] == "safely" &&
			s7ARContainsAnyWord(words[len(words)-3], "can", "may", "should", "must") &&
			s7ARContainsAnyWord(words[len(words)-4], "operator", "operators", "you") &&
			len(words) == 4 {
			return true
		}
		return strings.HasSuffix(prefix, "from the workspace root "+verb)
	}
	if s7ARContainsAnyWord(token, "restore", "remove") &&
		s7ARContainsAny(prefix, "for the index form", "for the blob form", "then") {
		return true
	}
	return s7ARRouteNounAffirmativeRE.MatchString(prefix)
}

func s7ARContainsAnyWord(value string, expected ...string) bool {
	for _, candidate := range expected {
		if value == candidate {
			return true
		}
	}
	return false
}

func s7ARClaimHasValidatedRouteReference(
	heading, section string,
	authorities map[string]bool,
) bool {
	normalized := strings.Join(strings.Fields(section), " ")
	lower := strings.ToLower(normalized)
	headingLower := strings.ToLower(heading)
	for _, reference := range []string{"§6.6", "§9.7.2", "§10.4", "D13", "D16"} {
		headingReference := strings.TrimPrefix(strings.ToLower(reference), "§")
		if !authorities[reference] {
			continue
		}
		if (s7ARContainsExactReference(heading, reference) ||
			s7ARContainsExactReference(headingLower, headingReference)) &&
			s7ARHasAffirmativeRouteGrammar(lower) {
			return true
		}
		for _, offset := range s7ARExactReferenceOffsets(normalized, reference) {
			clauseStart, clauseEnd := s7ARRouteClauseBounds(
				lower, offset, offset+len(reference),
			)
			clause := lower[clauseStart:clauseEnd]
			if s7ARReferenceMentionIsAffirmative(
				clause, offset-clauseStart, strings.ToLower(reference),
			) {
				return true
			}
		}
	}
	return false
}

func s7ARContainsExactReference(source, reference string) bool {
	return len(s7ARExactReferenceOffsets(source, reference)) != 0
}

func s7ARExactReferenceOffsets(source, reference string) []int {
	sourceLower := strings.ToLower(source)
	referenceLower := strings.ToLower(reference)
	var result []int
	for offset := 0; ; {
		found := strings.Index(sourceLower[offset:], referenceLower)
		if found < 0 {
			return result
		}
		found += offset
		beforeOK := found == 0 ||
			!s7ARReferenceIdentifierByte(sourceLower[found-1])
		after := found + len(referenceLower)
		afterOK := after == len(sourceLower) ||
			!s7ARReferenceIdentifierByte(sourceLower[after])
		if afterOK && after+1 < len(sourceLower) &&
			sourceLower[after] == '.' &&
			sourceLower[after+1] >= '0' && sourceLower[after+1] <= '9' {
			afterOK = false
		}
		if beforeOK && afterOK {
			result = append(result, found)
		}
		offset = found + len(referenceLower)
	}
}

func s7ARReferenceIdentifierByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == '-' || value == '_'
}

func s7ARHasAffirmativeRouteGrammar(lower string) bool {
	lower = strings.Join(strings.Fields(lower), " ")
	for offset := 0; offset < len(lower); {
		match := s7ARRouteGrammarAffirmativeRE.FindStringIndex(lower[offset:])
		if match == nil {
			return false
		}
		start := offset + match[0]
		end := offset + match[1]
		contextStart := 0
		for _, delimiter := range []string{";", ":"} {
			if found := strings.LastIndex(lower[:start], delimiter); found >= contextStart {
				contextStart = found + 1
			}
		}
		context := lower[contextStart:]
		if !s7ARRouteMentionIsQuoted(lower, start) &&
			!s7ARRouteGrammarBadContextRE.MatchString(context) {
			leading := strings.TrimSpace(lower[contextStart:start])
			if !s7ARRouteImmediateNegativeRE.MatchString(leading) {
				return true
			}
		}
		offset = end
	}
	return false
}

func s7ARReferenceMentionIsAffirmative(
	clause string,
	referenceOffset int,
	reference string,
) bool {
	if referenceOffset < 0 ||
		referenceOffset+len(reference) > len(clause) ||
		s7ARRouteMentionIsQuoted(clause, referenceOffset) {
		return false
	}
	prefix := strings.TrimSpace(strings.Trim(
		clause[:referenceOffset], "`*_()[]{}\"'",
	))
	suffix := strings.TrimSpace(strings.Trim(
		clause[referenceOffset+len(reference):], "`*_()[]{}\"'",
	))
	if !s7ARAffirmativeReferenceSuffix(suffix) {
		return false
	}
	if s7ARHasAffirmativeRouteGrammar(prefix) {
		return true
	}
	return s7ARReferencePrefixRE.MatchString(prefix)
}

func s7ARAffirmativeReferenceSuffix(suffix string) bool {
	suffix = strings.TrimSpace(strings.TrimRight(suffix, ".,;:"))
	if suffix == "" {
		return true
	}
	suffix = strings.TrimSpace(strings.Trim(suffix, "()[]"))
	if suffix == "" {
		return true
	}
	for _, token := range strings.FieldsFunc(suffix, func(character rune) bool {
		return character == ',' || character == ';' || character == ' ' ||
			character == '\t' || character == '\n'
	}) {
		lower := strings.ToLower(strings.Trim(token, "`*_()[]{}\"'"))
		if lower == "" || lower == "and" ||
			regexp.MustCompile(`^(?:d[0-9]+|pib-[0-9]+|§[0-9]+(?:\.[0-9]+)*)$`).MatchString(lower) {
			continue
		}
		return false
	}
	return true
}

func s7ARShippedClaimSources(t *testing.T) map[string]string {
	t.Helper()
	root := avpRepoRoot(t)
	embedSource, err := os.ReadFile(filepath.Join(root, "assets", "embed.go"))
	if err != nil {
		t.Fatal(err)
	}
	embedPatterns, err := s7AREmbeddedAssetPatterns(string(embedSource))
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	found := map[string]bool{"production": false, "assets": false, "docs": false}
	for _, raw := range bytes.Split(output, []byte{0}) {
		rel := filepath.ToSlash(string(raw))
		if rel == "" {
			continue
		}
		category := ""
		switch {
		case (strings.HasPrefix(rel, "cmd/") || strings.HasPrefix(rel, "internal/")) &&
			strings.HasSuffix(rel, ".go") &&
			!strings.HasSuffix(rel, "_test.go"):
			category = "production"
		case s7AREmbeddedAssetPath(rel, embedPatterns):
			category = "assets"
		case s7ARApplicableShippedDocument(rel):
			category = "docs"
		default:
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		sources[rel] = string(body)
		found[category] = true
	}
	for category, present := range found {
		if !present {
			t.Fatalf("PIB-519 shipped %s source inventory is empty", category)
		}
	}
	for _, document := range []string{
		"cmd/tpatch/main.go",
		"docs/prds/PRD-prepare-intent-bundle.md",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
	} {
		if _, present := sources[document]; !present {
			t.Fatalf("PIB-519 repository-wide shipped inventory omitted %s", document)
		}
	}
	return sources
}

func s7AREmbeddedAssetPatterns(source string) ([]string, error) {
	var result []string
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//go:embed ") {
			continue
		}
		for _, pattern := range strings.Fields(strings.TrimPrefix(line, "//go:embed ")) {
			pattern = strings.Trim(pattern, "`\"")
			if pattern == "" {
				return nil, errors.New("assets embed directive contains an empty pattern")
			}
			result = append(result, pattern)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("assets embed authority has no patterns")
	}
	return result, nil
}

func s7AREmbeddedAssetPath(rel string, patterns []string) bool {
	if !strings.HasPrefix(rel, "assets/") {
		return false
	}
	assetRel := strings.TrimPrefix(rel, "assets/")
	for _, rawPattern := range patterns {
		pattern := strings.TrimPrefix(rawPattern, "all:")
		if matched, err := path.Match(pattern, assetRel); err == nil && matched {
			return true
		}
		if !strings.ContainsAny(pattern, "*?[") &&
			(assetRel == pattern || strings.HasPrefix(assetRel, strings.TrimSuffix(pattern, "/")+"/")) {
			return true
		}
	}
	return false
}

func s7ARApplicableShippedDocument(rel string) bool {
	if !strings.HasSuffix(rel, ".md") {
		return false
	}
	if !strings.Contains(rel, "/") {
		return true
	}
	if !strings.HasPrefix(rel, "docs/") {
		return false
	}
	for _, excluded := range []string{
		"docs/handoff/",
		"docs/supervisor/",
		"docs/state-of-the-art/",
		"docs/whitepapers/",
		"docs/market-research/",
		"docs/milestones/",
		"docs/harnesses/",
	} {
		if strings.HasPrefix(rel, excluded) {
			return false
		}
	}
	return true
}

func s7ARValidatedRouteAuthorities(sources map[string]string) (map[string]bool, error) {
	prd := sources["docs/prds/PRD-prepare-intent-bundle.md"]
	adr := sources["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"]
	if prd == "" || adr == "" {
		return nil, errors.New("route authority sources are incomplete")
	}
	if err := validateS7ARExitSixRoutes(prd); err != nil {
		return nil, fmt.Errorf("§10.4 route authority: %w", err)
	}
	section66, err := s7ARSectionBetween(prd, "### 6.6 `--abandon-transaction`", "\n## 7.")
	if err != nil {
		return nil, err
	}
	section972, err := s7ARSectionBetween(prd, "#### 9.7.2 Honest purge procedure", "\n### 9.8")
	if err != nil {
		return nil, err
	}
	section104, err := s7ARSectionBetween(prd, "### 10.4 Exit codes", "\n### 10.5")
	if err != nil {
		return nil, err
	}
	d13, err := s7ARSectionBetween(adr, "### D13 —", "\n### D14")
	if err != nil {
		return nil, err
	}
	d16, err := s7ARSectionBetween(adr, "### D16 —", "\n### D17")
	if err != nil {
		return nil, err
	}
	authorities := map[string]bool{
		"§6.6": s7ARContainsAll(
			section66,
			"--abandon-transaction",
			"rm -rf .tpatch/local/intent-prepare/<slug>/",
			"archive-purge-evidence-divergent",
			"§9.7.2",
		) && s7ARClaimHasExecutableRoute(section66),
		"§9.7.2": s7ARContainsAll(
			section972,
			"archive-purge-evidence-divergent",
			"rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob",
			"restore",
			"retry_cwd: \"workspace-root\"",
		) && s7ARClaimHasExecutableRoute(section972),
		"§10.4": s7ARClaimHasExecutableRoute(section104),
		"D13": s7ARContainsAll(
			d13,
			"tpatch prepare <slug> --abandon-transaction",
			"manual fallback",
			"archive-purge-evidence-divergent",
			"archive procedure",
		) && s7ARClaimHasExecutableRoute(d13),
		"D16": s7ARContainsAll(
			d16,
			"archive-purge-evidence-divergent",
			"rm -rf --",
			"retry_cwd",
		) && s7ARClaimHasExecutableRoute(d16),
	}
	for reference, valid := range authorities {
		if !valid {
			if reference == "§6.6" {
				return nil, fmt.Errorf(
					"route authority %s did not resolve to accepted content "+
						"(required text=%t executable route=%t)",
					reference,
					s7ARContainsAll(
						section66,
						"--abandon-transaction",
						"rm -rf .tpatch/local/intent-prepare/<slug>/",
						"archive-purge-evidence-divergent",
						"§9.7.2",
					),
					s7ARClaimHasExecutableRoute(section66),
				)
			}
			return nil, fmt.Errorf("route authority %s did not resolve to accepted content", reference)
		}
	}
	return authorities, nil
}

func s7ARContainsAll(source string, values ...string) bool {
	for _, value := range values {
		if !strings.Contains(source, value) {
			return false
		}
	}
	return true
}

func s7ARReplaceClaimParagraph(source, marker, replacement string) (string, bool) {
	at := strings.Index(source, marker)
	if at < 0 {
		return source, false
	}
	start := strings.LastIndex(source[:at], "\n\n")
	if start < 0 {
		start = 0
	} else {
		start += 2
	}
	endOffset := strings.Index(source[at:], "\n\n")
	if endOffset < 0 {
		return source, false
	}
	end := at + endOffset
	return source[:start] + replacement + source[end:], true
}

func TestS7ARPrepareGrammarGuard(t *testing.T) {
	t.Run("PIB-520", func(t *testing.T) {
		prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
		registered := s7ARRegisteredPrepareFlags(t)
		if err := validateS7ARPrepareGrammar(prd, registered); err != nil {
			t.Fatal(err)
		}

		extra := append(append([]string(nil), registered...), "future-flag")
		if err := validateS7ARPrepareGrammar(prd, extra); err == nil {
			t.Fatal("PIB-520 same validator accepted a registered-but-absent flag")
		}
		mutated := strings.Replace(
			prd,
			"tpatch prepare <slug> --manual      [--json] [--quiet] [--path <dir>] [--dry-run]",
			"tpatch prepare <slug> --manual      [--json] [--quiet] [--path <dir>] [--dry-run] [--timeout <d>]",
			1,
		)
		if mutated == prd {
			t.Fatal("PIB-520 illegal-mode sensitivity anchor missing")
		}
		if err := validateS7ARPrepareGrammar(mutated, registered); err == nil {
			t.Fatal("PIB-520 same validator accepted timeout in manual mode")
		}
		mutated = strings.Replace(
			prd,
			" [--dry-run] [--allow-heuristic]\ntpatch prepare <slug> --manual",
			" [--dry-run]\ntpatch prepare <slug> --manual",
			1,
		)
		if mutated == prd {
			t.Fatal("PIB-520 generate allow-heuristic sensitivity anchor missing")
		}
		if err := validateS7ARPrepareGrammar(mutated, registered); err == nil {
			t.Fatal("PIB-520 same validator accepted missing generate allow-heuristic")
		}
		mutated = strings.Replace(
			prd,
			"tpatch prepare <slug> --manual      [--json]",
			"tpatch prepare <slug> [--manual]      [--json]",
			1,
		)
		if mutated == prd {
			t.Fatal("PIB-520 required-mode sensitivity anchor missing")
		}
		if err := validateS7ARPrepareGrammar(mutated, registered); err == nil {
			t.Fatal("PIB-520 same validator accepted optional manual mode")
		}
		mutated = strings.Replace(
			prd,
			"[--json] [--quiet]",
			"(--json | --quiet)",
			1,
		)
		if mutated == prd {
			t.Fatal("PIB-520 alternative-shape sensitivity anchor missing")
		}
		if err := validateS7ARPrepareGrammar(mutated, registered); err == nil {
			t.Fatal("PIB-520 same validator accepted malformed grammar alternatives")
		}
		mutated, changed := s7ARReplaceAfter(
			prd,
			"### 5.1 Authorized grammar (v1, complete)",
			"tpatch prepare <slug> --abandon-transaction",
			"tpatch prepare <slug> --abandon-transaction\n tpatch prepare <slug> --future",
		)
		if !changed {
			t.Fatal("PIB-520 leading-whitespace production sensitivity anchor missing")
		}
		if err := validateS7ARPrepareGrammar(mutated, registered); err == nil {
			t.Fatal("PIB-520 same validator ignored a leading-whitespace production")
		}
	})
}

func s7ARRegisteredPrepareFlags(t *testing.T) []string {
	t.Helper()
	command := prepareCmd()
	var flags []string
	command.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
		flags = append(flags, flag.Name)
	})
	sort.Strings(flags)
	return flags
}

func validateS7ARPrepareGrammar(prd string, registered []string) error {
	section, err := s7ARSectionBetween(
		prd,
		"### 5.1 Authorized grammar (v1, complete)",
		"\n\n- `<slug>`",
	)
	if err != nil {
		return err
	}
	var lines []string
	inFence := false
	for _, line := range strings.Split(section, "\n") {
		if strings.TrimSpace(line) == "```text" {
			inFence = true
			continue
		}
		if strings.TrimSpace(line) == "```" {
			break
		}
		logical := strings.TrimSpace(line)
		if inFence && strings.HasPrefix(logical, "tpatch prepare ") {
			lines = append(lines, logical)
		}
	}
	if len(lines) != 5 {
		return fmt.Errorf("prepare grammar lines = %d, want 5", len(lines))
	}
	expected := []s7ARPrepareProduction{
		{mode: "check", required: "--check", optional: []string{"--json", "--quiet", "--path <dir>"}},
		{mode: "generate", optional: []string{"--json", "--quiet", "--path <dir>", "--timeout <d>", "--timeout-phase <d>", "--no-retry", "--dry-run", "--allow-heuristic"}},
		{mode: "manual", required: "--manual", optional: []string{"--json", "--quiet", "--path <dir>", "--dry-run"}},
		{mode: "regenerate", required: "--regenerate", optional: []string{"--json", "--quiet", "--path <dir>", "--timeout <d>", "--timeout-phase <d>", "--no-retry", "--dry-run", "--allow-heuristic"}},
		{mode: "abandon", required: "--abandon-transaction", optional: []string{"--json", "--quiet", "--path <dir>", "--yes"}},
	}
	seen := map[string]int{}
	for index, line := range lines {
		production, parseErr := s7ARParsePrepareProduction(line)
		if parseErr != nil {
			return fmt.Errorf("grammar line %d: %w", index+1, parseErr)
		}
		want := expected[index]
		if production.mode != want.mode ||
			production.required != want.required ||
			fmt.Sprint(production.optional) != fmt.Sprint(want.optional) {
			return fmt.Errorf("grammar line %d = %+v, want %+v", index+1, production, want)
		}
		for _, flag := range production.flags() {
			seen[flag]++
		}
	}
	sort.Strings(registered)
	wantRegistered := []string{
		"abandon-transaction", "allow-heuristic", "check", "dry-run",
		"json", "manual", "no-retry", "quiet", "regenerate",
		"timeout", "timeout-phase", "yes",
	}
	if fmt.Sprint(registered) != fmt.Sprint(wantRegistered) {
		return fmt.Errorf("registered prepare flags = %v, want %v", registered, wantRegistered)
	}
	for _, flag := range registered {
		if seen[flag] == 0 {
			return fmt.Errorf("registered prepare flag --%s is absent from grammar", flag)
		}
	}
	if seen["allow-heuristic"] != 2 ||
		s7ARStringSliceContains(expected[0].flags(), "allow-heuristic") ||
		!s7ARStringSliceContains(expected[1].flags(), "allow-heuristic") ||
		s7ARStringSliceContains(expected[2].flags(), "allow-heuristic") ||
		!s7ARStringSliceContains(expected[3].flags(), "allow-heuristic") ||
		s7ARStringSliceContains(expected[4].flags(), "allow-heuristic") {
		return errors.New("--allow-heuristic is not present on generate and regenerate only")
	}
	if strings.Contains(section, "--mode") {
		return errors.New("prepare grammar contains forbidden --mode surface")
	}
	return nil
}

type s7ARPrepareProduction struct {
	mode     string
	required string
	optional []string
}

func (production s7ARPrepareProduction) flags() []string {
	var flags []string
	if production.required != "" {
		flags = append(flags, strings.TrimPrefix(production.required, "--"))
	}
	for _, group := range production.optional {
		flag := strings.Fields(group)[0]
		flags = append(flags, strings.TrimPrefix(flag, "--"))
	}
	return flags
}

func s7ARParsePrepareProduction(line string) (s7ARPrepareProduction, error) {
	const prefix = "tpatch prepare <slug>"
	if !strings.HasPrefix(line, prefix) {
		return s7ARPrepareProduction{}, errors.New("command prefix or slug shape changed")
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	production := s7ARPrepareProduction{mode: "generate"}
	if strings.HasPrefix(rest, "--") {
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			production.required = rest
			rest = ""
		} else {
			production.required = rest[:end]
			rest = strings.TrimSpace(rest[end:])
		}
		switch production.required {
		case "--check":
			production.mode = "check"
		case "--manual":
			production.mode = "manual"
		case "--regenerate":
			production.mode = "regenerate"
		case "--abandon-transaction":
			production.mode = "abandon"
		default:
			return production, fmt.Errorf("unknown required mode token %q", production.required)
		}
	}
	for rest != "" {
		if rest[0] != '[' {
			return production, fmt.Errorf("non-optional trailing grammar %q", rest)
		}
		closeAt := strings.IndexByte(rest, ']')
		if closeAt < 0 {
			return production, errors.New("unterminated optional group")
		}
		group := strings.TrimSpace(rest[1:closeAt])
		if group == "" || strings.ContainsAny(group, "[]|()") {
			return production, fmt.Errorf("malformed optional group %q", group)
		}
		fields := strings.Fields(group)
		if len(fields) < 1 || len(fields) > 2 ||
			!regexp.MustCompile(`^--[a-z][a-z0-9-]*$`).MatchString(fields[0]) {
			return production, fmt.Errorf("malformed optional tokens %q", group)
		}
		if len(fields) == 2 {
			wantValue := map[string]string{
				"--path": "<dir>", "--timeout": "<d>", "--timeout-phase": "<d>",
			}[fields[0]]
			if fields[1] != wantValue || wantValue == "" {
				return production, fmt.Errorf("illegal value shape %q", group)
			}
		} else if fields[0] == "--path" ||
			fields[0] == "--timeout" ||
			fields[0] == "--timeout-phase" {
			return production, fmt.Errorf("value-bearing flag lacks metavariable: %q", group)
		}
		production.optional = append(production.optional, group)
		rest = strings.TrimSpace(rest[closeAt+1:])
	}
	return production, nil
}

func s7ARSectionBetween(source, start, end string) (string, error) {
	startAt := strings.Index(source, start)
	if startAt < 0 {
		return "", fmt.Errorf("section start missing: %q", start)
	}
	startAt += len(start)
	endAt := strings.Index(source[startAt:], end)
	if endAt < 0 {
		return "", fmt.Errorf("section end missing after %q: %q", start, end)
	}
	return source[startAt : startAt+endAt], nil
}

func s7ARMarkdownRows(section string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") ||
			strings.HasPrefix(line, "|---") ||
			strings.HasPrefix(line, "|---:") {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		cells := make([]string, 0, len(parts))
		for _, part := range parts {
			cells = append(cells, strings.TrimSpace(part))
		}
		rows = append(rows, cells)
	}
	return rows
}
