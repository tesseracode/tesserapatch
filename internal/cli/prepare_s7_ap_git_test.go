//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestS7APPrepareGitRuntime(t *testing.T) {
	t.Run("PIB-475", func(t *testing.T) {
		fixture := s7APGlobalIgnoreFixture(t, true)
		prepareS4WriteReadyBundle(t, fixture.root, fixture.slug, false)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", fixture.root, "prepare", fixture.slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Refusal != nil ||
			report.Outcome != "published" {
			t.Fatalf("PIB-475 global-excludes-only prepare = exit:%d stderr:%q report:%+v",
				code, stderr, report)
		}
		s7APAssertGitWrapperLog(t, fixture, []string{
			"rev-parse --is-inside-work-tree",
			"check-ignore -q --no-index -- .tpatch/local/intent-prepare/" + fixture.slug,
			"--literal-pathspecs ls-files -- .tpatch/local/",
			"ls-files -- .tpatch",
		})
		if _, err := os.Stat(filepath.Join(fixture.root, ".gitignore")); !os.IsNotExist(err) {
			t.Fatalf("PIB-475 repository unexpectedly has .gitignore: %v", err)
		}
	})

	t.Run("PIB-476", func(t *testing.T) {
		gitSource := s6RepoFile(t, "internal/gitutil/ignore.go")
		reportSources := map[string]string{
			"internal/cli/prepare_publish.go":        s6RepoFile(t, "internal/cli/prepare_publish.go"),
			"internal/cli/feature_intent_archive.go": s6RepoFile(t, "internal/cli/feature_intent_archive.go"),
			"internal/workflow/doctor.go":            s6RepoFile(t, "internal/workflow/doctor.go"),
		}
		if err := validateS7APReportFieldInventory(reportSources); err != nil {
			t.Fatal(err)
		}
		success := s7APGlobalIgnoreFixture(t, true)
		prepareS4WriteReadyBundle(t, success.root, success.slug, false)
		code, successOut, stderr, _ := runPrepare(
			t, "--path", success.root, "prepare", success.slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-476 success fixture = exit:%d stderr:%q", code, stderr)
		}
		s7APAssertGitWrapperLog(t, success, []string{
			"rev-parse --is-inside-work-tree",
			"check-ignore -q --no-index -- .tpatch/local/intent-prepare/" + success.slug,
			"--literal-pathspecs ls-files -- .tpatch/local/",
			"ls-files -- .tpatch",
		})
		refusal := s7APGlobalIgnoreFixture(t, false)
		prepareS4WriteReadyBundle(t, refusal.root, refusal.slug, false)
		code, refusalOut, stderr, _ := runPrepare(
			t, "--path", refusal.root, "prepare", refusal.slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		refusalReport := prepareS4Report(t, refusalOut)
		if code != 3 || stderr == "" || refusalReport.Refusal == nil ||
			refusalReport.Refusal.Code != "local-lane-not-ignored" {
			t.Fatalf("PIB-476 refusal fixture = exit:%d stderr:%q report:%+v",
				code, stderr, refusalReport)
		}
		payloads := map[string]any{}
		for name, raw := range map[string]string{"success": successOut, "refusal": refusalOut} {
			payloads[name] = s7APDecodeGitPayload(t, name, raw)
		}
		authorityRoot, authoritySlug := prepareS4Workspace(t, "S7 AP authority refusal")
		previousAcquire := prepareAcquireAuthority
		prepareAcquireAuthority = func(string) (*intentlock.WorkspaceAuthority, error) {
			return nil, &intentlock.Error{
				Code:   intentlock.CodeDirectoryFlockUnavailable,
				Class:  "unknown-local",
				Detail: "classified without a path",
			}
		}
		code, authorityOut, stderr, _ := runPrepare(
			t, "--path", authorityRoot, "prepare", authoritySlug,
			"--allow-heuristic", "--json", "--quiet",
		)
		prepareAcquireAuthority = previousAcquire
		authorityReport := prepareS4Report(t, authorityOut)
		if code != 3 || stderr == "" || authorityReport.Refusal == nil ||
			authorityReport.Refusal.Code != "directory-flock-unavailable" {
			t.Fatalf("PIB-476 authority refusal fixture = exit:%d stderr:%q report:%+v",
				code, stderr, authorityReport)
		}
		payloads["authority-refusal"] = s7APDecodeGitPayload(
			t, "authority-refusal", authorityOut,
		)
		s7APActivateGitFixture(t, success)
		advisoryRoot, advisorySlug := prepareS4Workspace(t, "S7 AP Git advisory")
		code, advisoryOut, stderr, _ := runPrepare(
			t, "--path", advisoryRoot, "prepare", advisorySlug,
			"--allow-heuristic", "--json", "--quiet",
		)
		advisoryReport := prepareS4Report(t, advisoryOut)
		if code != 0 || stderr != "" || len(advisoryReport.Advisories) == 0 {
			t.Fatalf("PIB-476 advisory fixture = exit:%d stderr:%q report:%+v",
				code, stderr, advisoryReport)
		}
		payloads["advisory"] = s7APDecodeGitPayload(t, "advisory", advisoryOut)

		abandonRoot, abandonSlug := prepareS4Workspace(t, "S7 AP Git abandon")
		s6WriteJournalFixture(t, abandonRoot, abandonSlug, "journal-corrupt")
		code, abandonOut, stderr, _ := runPrepare(
			t, "--path", abandonRoot, "prepare", abandonSlug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		if code != 0 || stderr != "" || prepareS4Report(t, abandonOut).Abandoned == nil {
			t.Fatalf("PIB-476 abandon fixture = exit:%d stderr:%q\n%s",
				code, stderr, abandonOut)
		}
		payloads["abandon"] = s7APDecodeGitPayload(t, "abandon", abandonOut)

		recoveryRoot, recoverySlug := prepareS4Workspace(t, "S7 AP Git recovery")
		s7APCreateCP4Journal(t, recoveryRoot, recoverySlug)
		code, recoveryOut, stderr, _ := runPrepare(
			t, "--path", recoveryRoot, "prepare", recoverySlug,
			"--allow-heuristic", "--json", "--quiet",
		)
		if code != 0 || stderr != "" || prepareS4Report(t, recoveryOut).Recovery == nil {
			t.Fatalf("PIB-476 prepare recovery fixture = exit:%d stderr:%q\n%s",
				code, stderr, recoveryOut)
		}
		payloads["prepare-recovery"] = s7APDecodeGitPayload(t, "prepare-recovery", recoveryOut)

		partial := s7APRunPartialPurge(t)
		payloads["purge-partial"] = s7APJSONPayload(t, "purge-partial", partial.report)
		divergent := s7APRunDivergentPurge(t)
		if divergent.code != 6 || divergent.report.Divergence == nil {
			t.Fatalf("PIB-476 divergence fixture = exit:%d report:%+v",
				divergent.code, divergent.report)
		}
		payloads["purge-divergence"] = s7APJSONPayload(
			t, "purge-divergence", divergent.report,
		)
		retryArgv, err := s7APParseRenderedCommand(partial.report.PurgeProgress.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, purgeRecoveryOut, stderr := s7APRunFromWorkspace(t, partial.root, retryArgv)
		purgeRecovery := decodeIntentArchivePurgeReport(t, purgeRecoveryOut)
		if code != 0 || stderr != "" || purgeRecovery.Recovery == nil {
			t.Fatalf("PIB-476 purge recovery fixture = exit:%d stderr:%q report:%+v",
				code, stderr, purgeRecovery)
		}
		payloads["purge-recovery"] = s7APDecodeGitPayload(t, "purge-recovery", purgeRecoveryOut)
		completionArgv, err := s7APParseRenderedCommand(purgeRecovery.Recovery.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, purgeCompleteOut, stderr := s7APRunFromWorkspace(t, partial.root, completionArgv)
		if code != 0 || stderr != "" ||
			decodeIntentArchivePurgeReport(t, purgeCompleteOut).Outcome != "purged" {
			t.Fatalf("PIB-476 purge completion fixture = exit:%d stderr:%q\n%s",
				code, stderr, purgeCompleteOut)
		}
		payloads["purge-completion"] = s7APDecodeGitPayload(t, "purge-completion", purgeCompleteOut)

		pending := s7APRunPartialPurge(t)
		code, pendingOut, stderr, _ := runPrepare(
			t, "--path", pending.root, "feature", "intent-archive", "purge", pending.slug,
			"--all", "--json", "--quiet",
		)
		pendingReport := decodeIntentArchivePurgeReport(t, pendingOut)
		if code != 0 || stderr != "" || pendingReport.PendingPurge == nil ||
			pendingReport.Outcome != "recovery-required" {
			t.Fatalf("PIB-476 pending preview fixture = exit:%d stderr:%q report:%+v",
				code, stderr, pendingReport)
		}
		payloads["purge-pending-preview"] = s7APDecodeGitPayload(t, "purge-pending-preview", pendingOut)

		remainingRoot, remainingSlug := intentArchiveCLIWorkspace(t)
		remainingFirst := intentArchiveCLIReplacement(
			t, "analysis", []byte("S7 AP remaining first\n"), "retained",
		)
		remainingSecond := intentArchiveCLIReplacement(
			t, "spec", []byte("S7 AP remaining second\n"), "retained",
		)
		writeIntentArchiveCLIFixture(
			t,
			remainingRoot,
			remainingSlug,
			intentArchiveCLIIndex(
				t,
				remainingSlug,
				intentArchiveCLIGeneration(t, remainingSlug, remainingFirst),
				intentArchiveCLIGeneration(t, remainingSlug, remainingSecond),
			),
			map[string][]byte{},
		)
		code, remainingOut, stderr, _ := runPrepare(
			t, "--path", remainingRoot, "feature", "intent-archive", "purge", remainingSlug,
			"--blob", remainingFirst.ContentSHA256, "--yes", "--json", "--quiet",
		)
		remainingReport := decodeIntentArchivePurgeReport(t, remainingOut)
		if code != 3 || stderr == "" || remainingReport.RemainingRepairs == nil {
			t.Fatalf("PIB-476 remaining-repairs fixture = exit:%d stderr:%q report:%+v",
				code, stderr, remainingReport)
		}
		payloads["purge-remaining-repairs"] = s7APDecodeGitPayload(
			t, "purge-remaining-repairs", remainingOut,
		)

		previewRoot, previewSlug := intentArchiveCLIWorkspace(t)
		previewBody := []byte("S7 AP planned preview\n")
		previewReplacement := intentArchiveCLIReplacement(
			t, "analysis", previewBody, "retained",
		)
		writeIntentArchiveCLIFixture(
			t,
			previewRoot,
			previewSlug,
			intentArchiveCLIIndex(
				t, previewSlug, intentArchiveCLIGeneration(t, previewSlug, previewReplacement),
			),
			map[string][]byte{previewReplacement.ContentSHA256: previewBody},
		)
		code, previewOut, stderr, _ := runPrepare(
			t, "--path", previewRoot, "feature", "intent-archive", "purge", previewSlug,
			"--all", "--json", "--quiet",
		)
		previewReport := decodeIntentArchivePurgeReport(t, previewOut)
		if code != 0 || stderr != "" || previewReport.Outcome != "planned" ||
			previewReport.Confirmed {
			t.Fatalf("PIB-476 planned-preview fixture = exit:%d stderr:%q report:%+v",
				code, stderr, previewReport)
		}
		payloads["purge-planned-preview"] = s7APDecodeGitPayload(
			t, "purge-planned-preview", previewOut,
		)

		listRoot, listSlug, _, _ := s7APDanglingWorkspace(t)
		code, listOut, stderr, _ := runPrepare(
			t, "--path", listRoot, "feature", "intent-archive", "list", listSlug,
			"--json", "--quiet",
		)
		if code != 3 || stderr == "" {
			t.Fatalf("PIB-476 list fixture = exit:%d stderr:%q", code, stderr)
		}
		payloads["archive-list"] = s7APDecodeGitPayload(t, "archive-list", listOut)
		code, doctorOut, stderr, _ := runPrepare(
			t, "--path", listRoot, "doctor", "--check", "D9", "--json",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-476 doctor fixture = exit:%d stderr:%q", code, stderr)
		}
		payloads["doctor-d9"] = s7APDecodeGitPayload(t, "doctor-d9", doctorOut)

		forbidden := []string{
			success.root, success.home, success.globalConfig, success.systemConfig,
			refusal.root, refusal.home, refusal.globalConfig, refusal.systemConfig,
			advisoryRoot, abandonRoot, recoveryRoot, partial.root, divergent.root,
			pending.root, remainingRoot, previewRoot, listRoot, authorityRoot,
		}
		if err := validateS7APGitPrivacy(gitSource, reportSources, payloads, forbidden); err != nil {
			t.Fatal(err)
		}
		wrongSource := strings.Replace(
			gitSource,
			`"--", ".tpatch/local/"`,
			`"--", "/absolute/.tpatch/local/"`,
			1,
		)
		if err := validateS7APGitPrivacy(wrongSource, reportSources, payloads, forbidden); err == nil {
			t.Fatal("PIB-476 same validator accepted an absolute G3 lane argument")
		}
		wrongReports := make(map[string]string, len(reportSources))
		for name, source := range reportSources {
			wrongReports[name] = source
		}
		const archiveRel = "internal/cli/feature_intent_archive.go"
		wrongReports[archiveRel] = strings.Replace(
			wrongReports[archiveRel],
			"report.Refusal.RetryCWD = store.IntentArchiveRepairCWD",
			`report.Refusal.RetryCWD = "/absolute/untested-archive-emitter"`,
			1,
		)
		if err := validateS7APGitPrivacy(gitSource, wrongReports, payloads, forbidden); err == nil {
			t.Fatal("PIB-476 same validator accepted an absolute production emitter field")
		}
		for _, sensitivity := range []struct {
			name   string
			rel    string
			mutate func(string) string
		}{
			{"empty-left-concat", archiveRel, func(source string) string {
				return strings.Replace(source, "Index:           indexRel,",
					`Index:           "" + "/absolute/empty-left",`, 1)
			}},
			{"right-operand-concat", archiveRel, func(source string) string {
				return strings.Replace(source, "Index:           indexRel,",
					`Index:           strings.TrimSpace("") + "/absolute/right-operand",`, 1)
			}},
			{"relative-prefix-environment", archiveRel, func(source string) string {
				return strings.Replace(source, "Index:           indexRel,",
					`Index:           ".tpatch/" + os.Getenv("HOME"),`, 1)
			}},
			{"relative-prefix-helper", archiveRel, func(source string) string {
				source = strings.Replace(source, "Index:           indexRel,",
					`Index:           ".tpatch/" + s7APUnsafeConcatHelper(),`, 1)
				return source + `
func s7APUnsafeConcatHelper() string { return os.Getenv("HOME") }
`
			}},
			{"relative-prefix-absolute-identifier", archiveRel, func(source string) string {
				source = strings.Replace(
					source,
					"report := &intentArchiveDivergenceReport{",
					"absoluteSuffix := \"/absolute/identifier\"\n\treport := &intentArchiveDivergenceReport{",
					1,
				)
				return strings.Replace(source, "Index:           indexRel,",
					`Index:           ".tpatch/" + absoluteSuffix,`, 1)
			}},
			{"relative-prefix-absolute-helper", archiveRel, func(source string) string {
				source = strings.Replace(
					source, "\t\"path\"\n", "\t\"path\"\n\t\"path/filepath\"\n", 1,
				)
				source = strings.Replace(source, "Index:           indexRel,",
					`Index:           ".tpatch/" + s7APAbsoluteConcatHelper(),`, 1)
				return source + `
func s7APAbsoluteConcatHelper() string {
	absolute, _ := filepath.Abs(".tpatch")
	return absolute
}
`
			}},
			{"relative-prefix-traversal", archiveRel, func(source string) string {
				return strings.Replace(source, "Index:           indexRel,",
					`Index:           ".tpatch/" + "/../escape",`, 1)
			}},
			{"identifier-derived", archiveRel, func(source string) string {
				source = strings.Replace(
					source,
					"report := &intentArchiveDivergenceReport{",
					"absoluteIndex := \"/absolute/identifier-derived\"\n\treport := &intentArchiveDivergenceReport{",
					1,
				)
				return strings.Replace(source, "Index:           indexRel,",
					"Index:           absoluteIndex,", 1)
			}},
			{"helper-derived", archiveRel, func(source string) string {
				source = strings.Replace(source, "Index:           indexRel,",
					"Index:           s7APAbsoluteReportPath(),", 1)
				return source + "\nfunc s7APAbsoluteReportPath() string { return \"/absolute/helper-derived\" }\n"
			}},
			{"repo-root-parameter", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = repoRoot`, 1)
			}},
			{"repo-root-code", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Code = repoRoot`, 1)
			}},
			{"repo-root-join", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = filepath.Join(repoRoot, ".tpatch")`, 1)
			}},
			{"relative-prefix-repo-root", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = ".tpatch/" + repoRoot`, 1)
			}},
			{"external-unknown", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = os.Getenv("HOME")`, 1)
			}},
			{"unknown-parameter-code", "internal/cli/prepare_publish.go", func(source string) string {
				return source + `
func s7APUnknownCode(report preparePublishReport, unknown string) preparePublishReport {
	report.Refusal.Code = unknown
	return report
}
`
			}},
			{"unknown-helper-code", "internal/cli/prepare_publish.go", func(source string) string {
				return source + `
func s7APUnknownCodeSource() string { return os.Getenv("HOME") }
func s7APUnknownHelperCode(report preparePublishReport) preparePublishReport {
	report.Refusal.Code = s7APUnknownCodeSource()
	return report
}
`
			}},
			{"retry-helper-environment", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(
					source,
					`return strings.Join(args, " ")`,
					`return os.Getenv("HOME")`,
					1,
				)
			}},
			{"relative-helper-absolute", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(
					source,
					`func prepareFeatureRel(slug string) string {
	return ".tpatch/features/" + slug
}`,
					`func prepareFeatureRel(slug string) string {
	absolute, _ := filepath.Abs(".tpatch/features/" + slug)
	return absolute
}`,
					1,
				)
			}},
			{"helper-passed-repo-root", "internal/cli/prepare_publish.go", func(source string) string {
				source = strings.Replace(
					source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = s7APPassRoot(repoRoot)`,
					1,
				)
				return source + `
func s7APPassRoot(value string) string { return value }
`
			}},
			{"named-mode-environment", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = string(prepareMode(os.Getenv("HOME")))`, 1)
			}},
			{"named-mode-repo-root", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = string(prepareMode(repoRoot))`, 1)
			}},
			{"named-alias-helper", "internal/cli/prepare_publish.go", func(source string) string {
				source = strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	report.Refusal.Message = string(s7APUnsafeNamedAlias())`, 1)
				return source + `
type s7APPrepareModeAlias prepareMode
func s7APUnsafeNamedAlias() s7APPrepareModeAlias {
	return s7APPrepareModeAlias(os.Getenv("HOME"))
}
`
			}},
			{"named-selector-assignment", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	s7APOptions := prepareOptions{mode: prepareModeGenerate}
	s7APOptions.mode = prepareMode(os.Getenv("HOME"))
	report.Refusal.Message = string(s7APOptions.mode)`, 1)
			}},
			{"named-selector-composite", "internal/cli/prepare_publish.go", func(source string) string {
				return strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	s7APOptions := prepareOptions{mode: prepareMode(os.Getenv("HOME"))}
	report.Refusal.Message = string(s7APOptions.mode)`, 1)
			}},
			{"named-selector-helper", "internal/cli/prepare_publish.go", func(source string) string {
				source = strings.Replace(source,
					`report = refusePrepare(report, code, "")`,
					`report = refusePrepare(report, code, "")
	s7APOptions := prepareOptions{mode: s7APUnknownPrepareMode()}
	report.Refusal.Message = string(s7APOptions.mode)`, 1)
				return source + `
func s7APUnknownPrepareMode() prepareMode {
	return prepareMode(os.Getenv("HOME"))
}
`
			}},
		} {
			mutated := s7APCloneStringMap(reportSources)
			before := mutated[sensitivity.rel]
			mutated[sensitivity.rel] = sensitivity.mutate(before)
			if mutated[sensitivity.rel] == before {
				t.Fatalf("PIB-476 %s mutation anchor missing", sensitivity.name)
			}
			if err := validateS7APGitPrivacy(gitSource, mutated, payloads, forbidden); err == nil {
				t.Fatalf("PIB-476 same validator accepted %s absolute production emitter",
					sensitivity.name)
			}
		}
		enumPositive := s7APCloneStringMap(reportSources)
		const publishRel = "internal/cli/prepare_publish.go"
		enumPositive[publishRel] = strings.Replace(
			enumPositive[publishRel],
			`report = refusePrepare(report, code, "")`,
			`report = refusePrepare(report, code, "")
	report.Refusal.Message = string(prepareModeRegenerate)`,
			1,
		)
		if err := validateS7APGitPrivacy(
			gitSource, enumPositive, payloads, forbidden,
		); err != nil {
			t.Fatalf("PIB-476 declared enum constant positive control failed: %v", err)
		}
		selectorPositive := s7APCloneStringMap(reportSources)
		selectorPositive[publishRel] = strings.Replace(
			selectorPositive[publishRel],
			`report = refusePrepare(report, code, "")`,
			`report = refusePrepare(report, code, "")
	s7APOptions := prepareOptions{mode: prepareModeGenerate}
	s7APOptions.mode = prepareModeRegenerate
	report.Refusal.Message = string(s7APOptions.mode)`,
			1,
		)
		if err := validateS7APGitPrivacy(
			gitSource, selectorPositive, payloads, forbidden,
		); err != nil {
			t.Fatalf("PIB-476 named enum selector positive control failed: %v", err)
		}
		defaultPositive := s7APCloneStringMap(reportSources)
		defaultPositive[publishRel] = strings.Replace(
			defaultPositive[publishRel],
			`report = refusePrepare(report, code, "")`,
			`report = refusePrepare(report, code, "")
	s7APOptions := prepareOptions{}
	report.Refusal.Message = string(s7APOptions.mode)`,
			1,
		)
		if err := validateS7APGitPrivacy(
			gitSource, defaultPositive, payloads, forbidden,
		); err != nil {
			t.Fatalf("PIB-476 named enum selector default positive control failed: %v", err)
		}
	})
}

func s7APDecodeGitPayload(t *testing.T, name, raw string) any {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("decode %s Git report: %v", name, err)
	}
	return decoded
}

func s7APJSONPayload(t *testing.T, name string, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s Git report: %v", name, err)
	}
	return s7APDecodeGitPayload(t, name, string(raw))
}

type s7APGlobalIgnoreSetup struct {
	root         string
	slug         string
	home         string
	globalConfig string
	systemConfig string
	logPath      string
	envLine      string
	pathEnv      string
}

func s7APGlobalIgnoreFixture(t *testing.T, ignored bool) s7APGlobalIgnoreSetup {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root, slug := prepareS4Workspace(t, "S7 AP global excludes")
	if err := os.Remove(filepath.Join(root, ".gitignore")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	home := t.TempDir()
	configDir := t.TempDir()
	excludes := filepath.Join(configDir, "global-excludes")
	excludesBody := ""
	if ignored {
		excludesBody = ".tpatch/local/\n"
	}
	if err := os.WriteFile(excludes, []byte(excludesBody), 0o600); err != nil {
		t.Fatal(err)
	}
	globalConfig := filepath.Join(configDir, "global.gitconfig")
	globalBody := "[core]\n\texcludesFile = " + excludes + "\n"
	if err := os.WriteFile(globalConfig, []byte(globalBody), 0o600); err != nil {
		t.Fatal(err)
	}
	systemConfig := filepath.Join(configDir, "system.gitconfig")
	if err := os.WriteFile(systemConfig, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git.log")
	script := "#!/bin/sh\n" +
		"printf '%s|%s|%s|%s\\n' \"$*\" \"$GIT_CONFIG_GLOBAL\" \"$GIT_CONFIG_SYSTEM\" \"$HOME\" >> \"$S7_AP_GIT_LOG\"\n" +
		"exec " + strconv.Quote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	pathEnv := bin + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", pathEnv)
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", systemConfig)
	t.Setenv("S7_AP_GIT_LOG", logPath)
	return s7APGlobalIgnoreSetup{
		root: root, slug: slug, home: home,
		globalConfig: globalConfig, systemConfig: systemConfig, logPath: logPath,
		envLine: globalConfig + "|" + systemConfig + "|" + home,
		pathEnv: pathEnv,
	}
}

func s7APActivateGitFixture(t *testing.T, setup s7APGlobalIgnoreSetup) {
	t.Helper()
	t.Setenv("PATH", setup.pathEnv)
	t.Setenv("HOME", setup.home)
	t.Setenv("GIT_CONFIG_GLOBAL", setup.globalConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", setup.systemConfig)
	t.Setenv("S7_AP_GIT_LOG", setup.logPath)
}

func s7APAssertGitWrapperLog(t *testing.T, setup s7APGlobalIgnoreSetup, wantArgs []string) {
	t.Helper()
	raw, err := os.ReadFile(setup.logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != len(wantArgs) {
		t.Fatalf("Git request count = %d, want %d\n%s", len(lines), len(wantArgs), raw)
	}
	for index, line := range lines {
		want := wantArgs[index] + "|" + setup.envLine
		if line != want {
			t.Fatalf("Git request %d = %q, want %q", index+1, line, want)
		}
	}
}

func validateS7APGitPrivacy(
	gitSource string,
	reportSources map[string]string,
	payloads map[string]any,
	forbidden []string,
) error {
	if err := validateS7APReportFieldInventory(reportSources); err != nil {
		return err
	}
	wantPopulations := []string{
		"abandon", "advisory", "archive-list", "authority-refusal", "doctor-d9",
		"prepare-recovery", "purge-completion", "purge-divergence",
		"purge-partial", "purge-pending-preview", "purge-planned-preview",
		"purge-recovery", "purge-remaining-repairs", "refusal", "success",
	}
	gotPopulations := make([]string, 0, len(payloads))
	for name := range payloads {
		gotPopulations = append(gotPopulations, name)
	}
	sort.Strings(gotPopulations)
	if fmt.Sprint(gotPopulations) != fmt.Sprint(wantPopulations) {
		return fmt.Errorf("Git report populations = %v, want %v",
			gotPopulations, wantPopulations)
	}
	cliSource, ok := reportSources["internal/cli/prepare_publish.go"]
	if !ok {
		return errors.New("prepare report source missing from inventory")
	}
	if err := validateS7APGitSourcePrivacy(gitSource, cliSource); err != nil {
		return err
	}
	for name, payload := range payloads {
		if err := s7APWalkGitReport(name, payload, forbidden); err != nil {
			return err
		}
	}
	return nil
}

func validateS7APReportFieldInventory(sources map[string]string) error {
	expected := map[string][]string{
		"internal/cli/prepare_publish.go:prepareArtifactReport": {
			"ArchivedBlob", "Disposition", "Generator", "ID", "Path", "Role",
		},
		"internal/cli/prepare_publish.go:prepareActionReport": {
			"Action", "ID", "Path",
		},
		"internal/cli/prepare_publish.go:prepareArchiveReport": {
			"BlobsDir", "GenerationID",
		},
		"internal/cli/prepare_publish.go:prepareAdvisoryReport": {
			"ArtifactID", "Code", "Message",
		},
		"internal/cli/prepare_publish.go:prepareRefusalReport": {
			"Code", "Message", "Remediation", "Retry", "RetryCWD",
		},
		"internal/cli/prepare_publish.go:prepareRecoveryReport": {
			"FinalizedHashes", "Kind", "RestoredEntries", "Retry", "RetryCWD",
		},
		"internal/cli/prepare_publish.go:prepareAbandonedReport": {
			"Directory", "Existing", "Moved", "RemoveCommand",
		},
		"internal/cli/prepare_publish.go:preparePurgeProgressReport": {
			"CompletedHashes", "PendingHash", "RemainingHashes", "Resume", "Retry", "RetryCWD", "State",
		},
		"internal/cli/prepare_publish.go:preparePublishReport": {
			"Abandoned", "Action", "Actions", "Advisories", "Archive", "Artifacts",
			"Command", "Disclaimer", "DryRun", "ExecutionPreflight", "FeatureState",
			"Mode", "OrphanBlobs", "Outcome", "PlanNote", "PurgeProgress",
			"Recovery", "Refusal", "SchemaVersion", "Slug",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveRefusalReport": {
			"Code", "Message", "Remediation", "Retry", "RetryCWD",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveListEntryReport": {
			"ArtifactID", "Availability", "Blob", "BlobPath", "BlobSizeBytes",
			"ContentSHA256", "LiveGenerationIDs", "Path", "Present", "PurgePending",
			"Purged", "Repair", "Retry", "RetryCWD", "SizeBytes", "Storage",
			"TombstoneGenerationIDs",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveListGenerationReport": {
			"Entries", "GenerationID", "Mode",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveListOrphanReport": {
			"Hash", "Path", "Present", "Repair", "Retry", "RetryCWD", "SizeBytes", "Storage",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveListCorruptObjectReport": {
			"Hash", "Kind", "Path", "Repair", "Retry", "RetryCWD",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveListReport": {
			"Command", "CorruptObjects", "Generations", "HistoryDisclosure", "Index",
			"Orphans", "Outcome", "Refusal", "SchemaVersion", "Slug",
		},
		"internal/cli/feature_intent_archive.go:intentArchivePurgeReferenceReport": {
			"ArtifactID", "GenerationID", "Hash", "Path", "WireState",
		},
		"internal/cli/feature_intent_archive.go:intentArchivePurgeBlobReport": {
			"Hash", "Path", "Present", "Removed", "SizeBytes",
		},
		"internal/cli/feature_intent_archive.go:intentArchivePendingHashReport": {
			"Blob", "Hash", "Index", "Plan",
		},
		"internal/cli/feature_intent_archive.go:intentArchivePendingPurgeReport": {
			"PendingHashes", "RecoveryRequired", "Retry", "RetryCWD", "Selector",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveRecoveryReport": {
			"FinalizedHashes", "Kind", "RestoredEntries", "Retry", "RetryCWD",
		},
		"internal/cli/feature_intent_archive.go:intentArchivePurgeProgressReport": {
			"CompletedHashes", "PendingHash", "RemainingHashes", "Resume", "Retry", "RetryCWD", "State",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveRepairNextReport": {
			"Class", "Kind", "Ordinal",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveRepairStageReport": {
			"AfterPrerequisite", "Class", "Hashes", "Kind", "Ordinal", "Paths",
			"Repair", "RepairCWD", "ResultingClasses",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveRemainingRepairsReport": {
			"NextStage", "RepairedClass", "RerunRequired", "Stages", "StagesRemaining",
		},
		"internal/cli/feature_intent_archive.go:intentArchiveDivergenceReport": {
			"Blob", "CompletedHashes", "Cost", "Index", "Kind", "PendingHash",
			"RemainingHashes", "RemoveCommand", "RestoreInstruction", "Retry",
			"RetryCWD", "Warning",
		},
		"internal/cli/feature_intent_archive.go:intentArchivePurgeReport": {
			"Action", "Advisories", "BlastRadius", "Blobs", "Command", "Confirmed",
			"Divergence", "GenerationIDs", "Hashes", "HistoryDisclosure", "OrphanBlobs",
			"Outcome", "PendingPurge", "PurgeProgress", "Recovery", "References",
			"Refusal", "RemainingRepairs", "Retry", "RetryCWD", "SchemaVersion",
			"Selector", "Slug",
		},
		"internal/workflow/doctor.go:DoctorReport": {
			"Checks", "Command", "DryRun", "Findings", "Fix", "SchemaVersion", "Summary",
		},
		"internal/workflow/doctor.go:DoctorSummary": {
			"ChecksRun", "Errors", "Findings", "Fixed", "Warnings",
		},
		"internal/workflow/doctor.go:DoctorFinding": {
			"BackupPath", "CheckID", "Code", "Feature", "Field", "Fixable", "Line",
			"Message", "Path", "Remediation", "Severity", "Tag",
		},
		"internal/workflow/doctor.go:DoctorCheckStatus": {
			"CheckID", "Error", "Status",
		},
	}
	for key := range expected {
		sort.Strings(expected[key])
	}
	actual := map[string][]string{}
	stringFields := map[string]bool{}
	reportTypes := map[string]bool{}
	for key := range expected {
		parts := strings.Split(key, ":")
		reportTypes[parts[len(parts)-1]] = true
	}
	wantSources := []string{
		"internal/cli/feature_intent_archive.go",
		"internal/cli/prepare_publish.go",
		"internal/workflow/doctor.go",
	}
	gotSources := make([]string, 0, len(sources))
	for name := range sources {
		gotSources = append(gotSources, name)
	}
	sort.Strings(gotSources)
	if fmt.Sprint(gotSources) != fmt.Sprint(wantSources) {
		return fmt.Errorf("report source inventory = %v, want %v", gotSources, wantSources)
	}
	analysisSources, err := s7APReportAnalysisSources(sources)
	if err != nil {
		return err
	}
	typedPackages, err := s7APTypeCheckReportPackages(analysisSources)
	if err != nil {
		return fmt.Errorf("type-check report producers: %w", err)
	}
	flows := map[string]*s7APReportStringFlow{}
	universe := map[string]s7APFunctionDefinition{}
	for packageName, typedPackage := range typedPackages {
		flow := newS7APReportStringFlow(typedPackage)
		flows[packageName] = flow
		for function, declaration := range flow.functions {
			universe[s7APFunctionKey(function)] = s7APFunctionDefinition{
				flow: flow, function: function, declaration: declaration,
			}
		}
	}
	for _, flow := range flows {
		flow.universe = universe
	}
	enumUniverse := map[string][]s7APEnumFieldAssignment{}
	enumDefaultUniverse := map[string]bool{}
	for _, flow := range flows {
		for key, expressions := range flow.enumAssignments {
			for _, expression := range expressions {
				enumUniverse[key] = append(enumUniverse[key], s7APEnumFieldAssignment{
					flow: flow, expression: expression,
				})
			}
		}
		for key := range flow.enumDefaults {
			enumDefaultUniverse[key] = true
		}
	}
	for _, flow := range flows {
		flow.enumUniverse = enumUniverse
		flow.enumDefaultUniverse = enumDefaultUniverse
	}
	var producerInventory []string
	for _, name := range wantSources {
		packageName := filepath.ToSlash(filepath.Dir(name))
		typedPackage := typedPackages[packageName]
		if typedPackage == nil || typedPackage.info == nil {
			return fmt.Errorf("typed report package %s is missing", packageName)
		}
		file := typedPackage.relFiles[name]
		if file == nil {
			return fmt.Errorf("typed report source %s is missing", name)
		}
		flow := flows[packageName]
		for _, declaration := range file.Decls {
			typeDecl, ok := declaration.(*ast.GenDecl)
			if !ok || typeDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range typeDecl.Specs {
				typeSpec, _ := spec.(*ast.TypeSpec)
				structType, _ := typeSpec.Type.(*ast.StructType)
				if typeSpec == nil || structType == nil {
					continue
				}
				hasJSON := false
				for _, field := range structType.Fields.List {
					if field.Tag != nil && strings.Contains(field.Tag.Value, `json:`) {
						hasJSON = true
						break
					}
				}
				if hasJSON && !reportTypes[typeSpec.Name.Name] {
					return fmt.Errorf("%s introduces unclassified JSON report type %s",
						name, typeSpec.Name.Name)
				}
				if !reportTypes[typeSpec.Name.Name] {
					continue
				}
				key := name + ":" + typeSpec.Name.Name
				for _, field := range structType.Fields.List {
					if field.Tag == nil || len(field.Names) != 1 {
						continue
					}
					tag, err := strconv.Unquote(field.Tag.Value)
					if err != nil || !strings.Contains(tag, `json:"`) {
						return fmt.Errorf("%s.%s lacks a JSON tag", key, field.Names[0].Name)
					}
					fieldName := field.Names[0].Name
					actual[key] = append(actual[key], fieldName)
					if s7APReportStringType(field.Type) {
						stringFields[fieldName] = true
					}
				}
				sort.Strings(actual[key])
			}
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			produces := false
			ast.Inspect(function.Body, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.CallExpr:
					called := flow.calledFunction(value)
					if called == nil || !s7APControlledReportConstructor(called.Name()) {
						break
					}
					for index, argument := range value.Args {
						if !s7APReportStringValueType(flow.info.TypeOf(argument)) {
							continue
						}
						if !flow.provesNoAbsolute(argument, nil, nil, nil) {
							err = fmt.Errorf("%s passes an unresolved value to %s argument %d via %s",
								function.Name.Name, called.Name(), index, s7APFormatExpression(argument))
							return false
						}
					}
				case *ast.CompositeLit:
					identifier, _ := value.Type.(*ast.Ident)
					if identifier != nil && reportTypes[identifier.Name] {
						produces = true
						for _, element := range value.Elts {
							kv, _ := element.(*ast.KeyValueExpr)
							field, _ := kv.Key.(*ast.Ident)
							if field != nil && stringFields[field.Name] {
								if !flow.provesNoAbsolute(kv.Value, nil, nil, nil) {
									err = fmt.Errorf("%s assigns an unresolved value to %s via %s",
										function.Name.Name, field.Name, s7APFormatExpression(kv.Value))
									return false
								}
							}
						}
					}
				case *ast.AssignStmt:
					for index, left := range value.Lhs {
						selector, _ := left.(*ast.SelectorExpr)
						if selector == nil || !stringFields[selector.Sel.Name] ||
							index >= len(value.Rhs) {
							continue
						}
						produces = true
						if !flow.provesNoAbsolute(value.Rhs[index], nil, nil, nil) {
							err = fmt.Errorf("%s assigns an unresolved value to %s via %s",
								function.Name.Name, selector.Sel.Name,
								s7APFormatExpression(value.Rhs[index]))
							return false
						}
					}
				}
				return err == nil
			})
			if err != nil {
				return err
			}
			if produces {
				producerInventory = append(producerInventory, name+":"+function.Name.Name)
			}
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("report field inventory drift:\ngot  %#v\nwant %#v", actual, expected)
	}
	sort.Strings(producerInventory)
	wantProducers := []string{
		"internal/cli/feature_intent_archive.go:applyIntentArchivePurgePlan",
		"internal/cli/feature_intent_archive.go:applyIntentArchivePurgeResult",
		"internal/cli/feature_intent_archive.go:buildIntentArchiveDivergence",
		"internal/cli/feature_intent_archive.go:buildIntentArchiveListReport",
		"internal/cli/feature_intent_archive.go:buildIntentArchivePendingPurge",
		"internal/cli/feature_intent_archive.go:buildIntentArchivePurgeProgress",
		"internal/cli/feature_intent_archive.go:buildIntentArchiveRemainingRepairsReport",
		"internal/cli/feature_intent_archive.go:buildIntentArchiveUnindexedCorruptObjects",
		"internal/cli/feature_intent_archive.go:clearIntentArchiveLowerPrecedenceReport",
		"internal/cli/feature_intent_archive.go:emitIntentArchivePurgeFailure",
		"internal/cli/feature_intent_archive.go:intentArchiveListRepair",
		"internal/cli/feature_intent_archive.go:intentArchivePendingJournalRefusal",
		"internal/cli/feature_intent_archive.go:intentArchiveRefusalFromError",
		"internal/cli/feature_intent_archive.go:intentArchiveSimpleRefusal",
		"internal/cli/feature_intent_archive.go:newIntentArchiveListReport",
		"internal/cli/feature_intent_archive.go:newIntentArchivePurgeReport",
		"internal/cli/feature_intent_archive.go:normalizeIntentArchivePurgeReport",
		"internal/cli/feature_intent_archive.go:runFeatureIntentArchiveList",
		"internal/cli/feature_intent_archive.go:runFeatureIntentArchivePurge",
		"internal/cli/feature_intent_archive.go:runFeatureIntentArchivePurgeConfirmed",
		"internal/cli/feature_intent_archive.go:runFeatureIntentArchivePurgePreview",
		"internal/cli/prepare_publish.go:ProbeBlob",
		"internal/cli/prepare_publish.go:applyPrepareArchiveObservation",
		"internal/cli/prepare_publish.go:applyPrepareGenerationReport",
		"internal/cli/prepare_publish.go:applyPrepareOptionFields",
		"internal/cli/prepare_publish.go:buildPreparePlan",
		"internal/cli/prepare_publish.go:newPreparePublishReport",
		"internal/cli/prepare_publish.go:prepareAdvisory",
		"internal/cli/prepare_publish.go:prepareArtifactRow",
		"internal/cli/prepare_publish.go:prepareAuthorityRefusal",
		"internal/cli/prepare_publish.go:prepareGenerationFailure",
		"internal/cli/prepare_publish.go:prepareIntentpubFailure",
		"internal/cli/prepare_publish.go:preparePlanActions",
		"internal/cli/prepare_publish.go:prepareRawPreimageFailure",
		"internal/cli/prepare_publish.go:prepareRecoveryIntentpubFailure",
		"internal/cli/prepare_publish.go:prepareStagingFailure",
		"internal/cli/prepare_publish.go:prepareStoreArchiveFailure",
		"internal/cli/prepare_publish.go:prepareValidateArchiveSnapshot",
		"internal/cli/prepare_publish.go:publishPrepareStatusOnly",
		"internal/cli/prepare_publish.go:publishPrepareTransaction",
		"internal/cli/prepare_publish.go:refreshPrepareOrphanTruth",
		"internal/cli/prepare_publish.go:refusePrepare",
		"internal/cli/prepare_publish.go:runPrepareAbandon",
		"internal/cli/prepare_publish.go:runPrepareDryPlan",
		"internal/cli/prepare_publish.go:runPreparePublish",
		"internal/cli/prepare_publish.go:writePreparePublishReport",
		"internal/workflow/doctor.go:RunDoctor",
		"internal/workflow/doctor.go:addFinding",
	}
	if fmt.Sprint(producerInventory) != fmt.Sprint(wantProducers) {
		return fmt.Errorf("report producer inventory = %#v", producerInventory)
	}
	return nil
}

func s7APReportAnalysisSources(sources map[string]string) (map[string]string, error) {
	result := s7APCloneStringMap(sources)
	root, err := s6FindModuleRoot()
	if err != nil {
		return nil, err
	}
	for _, rel := range []string{
		"internal/intent/inspect.go",
		"internal/intentpub/plan.go",
		"internal/redact/redact.go",
		"internal/store/intent_archive.go",
	} {
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			return nil, fmt.Errorf("read report helper source %s: %w", rel, readErr)
		}
		result[rel] = string(body)
	}
	return result, nil
}

func s7APTypeCheckReportPackages(sources map[string]string) (map[string]*s6TypedPackage, error) {
	root, err := s6FindModuleRoot()
	if err != nil {
		return nil, err
	}
	packageNames := map[string]bool{}
	for name := range sources {
		packageNames[filepath.ToSlash(filepath.Dir(name))] = true
	}
	result := map[string]*s6TypedPackage{}
	for packageName := range packageNames {
		directory := filepath.Join(root, filepath.FromSlash(packageName))
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		fileset := token.NewFileSet()
		var files []*ast.File
		relFiles := map[string]*ast.File{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			matched, matchErr := build.Default.MatchFile(directory, entry.Name())
			if matchErr != nil {
				return nil, matchErr
			}
			if !matched {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(packageName, entry.Name()))
			body, ok := sources[rel]
			if !ok {
				raw, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
				if readErr != nil {
					return nil, readErr
				}
				body = string(raw)
			}
			file, parseErr := parser.ParseFile(fileset, rel, body, 0)
			if parseErr != nil {
				return nil, parseErr
			}
			files = append(files, file)
			relFiles[rel] = file
		}
		info := &types.Info{
			Types:      map[ast.Expr]types.TypeAndValue{},
			Defs:       map[*ast.Ident]types.Object{},
			Uses:       map[*ast.Ident]types.Object{},
			Selections: map[*ast.SelectorExpr]*types.Selection{},
		}
		var typeErrors []string
		config := types.Config{
			Importer: s6ModuleExportImporter(root),
			Error: func(typeErr error) {
				typeErrors = append(typeErrors, typeErr.Error())
			},
		}
		checked, checkErr := config.Check(
			s6FullModulePackagePath(packageName), fileset, files, info,
		)
		if checkErr != nil || len(typeErrors) != 0 {
			return nil, fmt.Errorf("%s: %v %s",
				packageName, checkErr, strings.Join(typeErrors, "; "))
		}
		result[packageName] = &s6TypedPackage{
			path: packageName, pkg: checked, info: info, files: files,
			relFiles: relFiles, complete: true,
		}
	}
	return result, nil
}

func s7APReportStringType(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "string"
	case *ast.ArrayType:
		identifier, _ := value.Elt.(*ast.Ident)
		return identifier != nil && identifier.Name == "string"
	default:
		return false
	}
}

func s7APReportStringValueType(value types.Type) bool {
	if value == nil {
		return false
	}
	switch underlying := value.Underlying().(type) {
	case *types.Basic:
		return underlying.Kind() == types.String || underlying.Kind() == types.UntypedString
	case *types.Slice:
		basic, _ := underlying.Elem().Underlying().(*types.Basic)
		return basic != nil && (basic.Kind() == types.String || basic.Kind() == types.UntypedString)
	default:
		return false
	}
}

func s7APFormatExpression(expression ast.Expr) string {
	var rendered bytes.Buffer
	if err := format.Node(&rendered, token.NewFileSet(), expression); err != nil {
		return fmt.Sprintf("%T", expression)
	}
	return rendered.String()
}

type s7APReportStringFlow struct {
	info                *types.Info
	assignments         map[types.Object][]ast.Expr
	arguments           map[types.Object][]ast.Expr
	functions           map[*types.Func]*ast.FuncDecl
	multiResults        map[types.Object]s7APCallResult
	builderWrites       map[types.Object][]ast.Expr
	enumAssignments     map[string][]ast.Expr
	enumDefaults        map[string]bool
	enumUniverse        map[string][]s7APEnumFieldAssignment
	enumDefaultUniverse map[string]bool
	universe            map[string]s7APFunctionDefinition
}

type s7APCallResult struct {
	call  *ast.CallExpr
	index int
}

type s7APFunctionDefinition struct {
	flow        *s7APReportStringFlow
	function    *types.Func
	declaration *ast.FuncDecl
}

type s7APEnumFieldAssignment struct {
	flow       *s7APReportStringFlow
	expression ast.Expr
}

func newS7APReportStringFlow(pkg *s6TypedPackage) *s7APReportStringFlow {
	flow := &s7APReportStringFlow{
		info:            pkg.info,
		assignments:     map[types.Object][]ast.Expr{},
		arguments:       map[types.Object][]ast.Expr{},
		functions:       map[*types.Func]*ast.FuncDecl{},
		multiResults:    map[types.Object]s7APCallResult{},
		builderWrites:   map[types.Object][]ast.Expr{},
		enumAssignments: map[string][]ast.Expr{},
		enumDefaults:    map[string]bool{},
	}
	for _, file := range pkg.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok {
				if object, _ := pkg.info.Defs[function.Name].(*types.Func); object != nil {
					flow.functions[object] = function
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.ValueSpec:
				for index, name := range value.Names {
					if index >= len(value.Values) {
						if object := pkg.info.Defs[name]; object != nil {
							s7APRecordNamedEnumDefaults(
								flow, object.Type(), nil,
							)
						}
						continue
					}
					if object := pkg.info.Defs[name]; object != nil {
						flow.assignments[object] = append(flow.assignments[object], value.Values[index])
					}
				}
			case *ast.AssignStmt:
				for index, left := range value.Lhs {
					identifier, _ := left.(*ast.Ident)
					selector, _ := left.(*ast.SelectorExpr)
					if identifier != nil && identifier.Name == "_" {
						continue
					}
					var object types.Object
					if identifier != nil {
						object = pkg.info.Defs[identifier]
						if object == nil {
							object = pkg.info.Uses[identifier]
						}
					} else if selector != nil {
						object = pkg.info.ObjectOf(selector.Sel)
					}
					if object == nil {
						continue
					}
					var assigned ast.Expr
					switch {
					case len(value.Lhs) == len(value.Rhs):
						assigned = value.Rhs[index]
						flow.assignments[object] = append(flow.assignments[object], assigned)
					case len(value.Rhs) == 1:
						assigned = value.Rhs[0]
						flow.assignments[object] = append(flow.assignments[object], assigned)
						call, _ := value.Rhs[0].(*ast.CallExpr)
						if call != nil {
							flow.multiResults[object] = s7APCallResult{call: call, index: index}
						}
					}
					if selector != nil && assigned != nil {
						s7APRecordNamedEnumAssignment(
							flow,
							pkg.info.TypeOf(selector.X),
							selector.Sel.Name,
							object,
							assigned,
						)
					}
				}
			case *ast.CompositeLit:
				compositeType := pkg.info.TypeOf(value)
				if compositeType == nil {
					return true
				}
				if pointer, _ := compositeType.(*types.Pointer); pointer != nil {
					compositeType = pointer.Elem()
				}
				structure, _ := compositeType.Underlying().(*types.Struct)
				if structure == nil {
					return true
				}
				assignedFields := map[types.Object]bool{}
				for index, element := range value.Elts {
					if keyed, _ := element.(*ast.KeyValueExpr); keyed != nil {
						key, _ := keyed.Key.(*ast.Ident)
						field, _ := pkg.info.ObjectOf(key).(*types.Var)
						if field != nil && field.IsField() {
							assignedFields[field] = true
							flow.assignments[field] = append(
								flow.assignments[field],
								keyed.Value,
							)
							s7APRecordNamedEnumAssignment(
								flow,
								compositeType,
								field.Name(),
								field,
								keyed.Value,
							)
						}
						continue
					}
					if index < structure.NumFields() {
						field := structure.Field(index)
						assignedFields[field] = true
						flow.assignments[field] = append(
							flow.assignments[field],
							element,
						)
						s7APRecordNamedEnumAssignment(
							flow,
							compositeType,
							field.Name(),
							field,
							element,
						)
					}
				}
				s7APRecordNamedEnumDefaults(flow, compositeType, assignedFields)
			case *ast.RangeStmt:
				identifier, _ := value.Value.(*ast.Ident)
				if identifier != nil && identifier.Name != "_" {
					object := pkg.info.Defs[identifier]
					if object == nil {
						object = pkg.info.Uses[identifier]
					}
					if object != nil {
						flow.assignments[object] = append(flow.assignments[object], value.X)
					}
				}
			case *ast.CallExpr:
				selector, _ := value.Fun.(*ast.SelectorExpr)
				if selector != nil && selector.Sel.Name == "WriteString" &&
					len(value.Args) == 1 {
					identifier, _ := selector.X.(*ast.Ident)
					if object := pkg.info.ObjectOf(identifier); object != nil {
						flow.builderWrites[object] = append(
							flow.builderWrites[object], value.Args[0],
						)
					}
				}
			}
			return true
		})
	}
	for _, file := range pkg.files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, _ := node.(*ast.CallExpr)
			if call == nil {
				return true
			}
			function := flow.calledFunction(call)
			declaration := flow.functions[function]
			if declaration == nil || declaration.Type.Params == nil {
				return true
			}
			argument := 0
			for _, field := range declaration.Type.Params.List {
				for _, name := range field.Names {
					if argument >= len(call.Args) {
						break
					}
					if object := pkg.info.Defs[name]; object != nil {
						flow.arguments[object] = append(flow.arguments[object], call.Args[argument])
					}
					argument++
				}
			}
			return true
		})
	}
	return flow
}

func s7APRecordNamedEnumDefaults(
	flow *s7APReportStringFlow,
	value types.Type,
	assigned map[types.Object]bool,
) {
	if flow == nil || value == nil {
		return
	}
	if pointer, _ := value.(*types.Pointer); pointer != nil {
		value = pointer.Elem()
	}
	structure, _ := value.Underlying().(*types.Struct)
	if structure == nil {
		return
	}
	for index := 0; index < structure.NumFields(); index++ {
		field := structure.Field(index)
		if !assigned[field] && s7APNamedEnumStringType(field.Type()) {
			if key := s7APEnumFieldKey(value, field.Name()); key != "" {
				flow.enumDefaults[key] = true
			}
		}
	}
}

func s7APRecordNamedEnumAssignment(
	flow *s7APReportStringFlow,
	receiver types.Type,
	fieldName string,
	field types.Object,
	expression ast.Expr,
) {
	if flow == nil || field == nil || expression == nil ||
		!s7APNamedEnumStringType(field.Type()) {
		return
	}
	if key := s7APEnumFieldKey(receiver, fieldName); key != "" {
		flow.enumAssignments[key] = append(flow.enumAssignments[key], expression)
	}
}

func s7APEnumFieldKey(receiver types.Type, fieldName string) string {
	if receiver == nil || fieldName == "" {
		return ""
	}
	receiver = types.Unalias(receiver)
	if pointer, _ := receiver.(*types.Pointer); pointer != nil {
		receiver = types.Unalias(pointer.Elem())
	}
	named, _ := receiver.(*types.Named)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + fieldName
}

func (flow *s7APReportStringFlow) provesNoAbsolute(
	expression ast.Expr,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	if expression == nil {
		return false
	}
	if s7APExpressionUsesIdentifier(expression, "repoRoot") {
		return false
	}
	if visitingObjects == nil {
		visitingObjects = map[types.Object]bool{}
	}
	if visitingFunctions == nil {
		visitingFunctions = map[string]bool{}
	}
	if binary, _ := expression.(*ast.BinaryExpr); binary != nil &&
		binary.Op == token.ADD {
		return flow.provesSafeConcatenation(
			binary, bindings, visitingObjects, visitingFunctions,
		)
	}
	if call, _ := expression.(*ast.CallExpr); call != nil &&
		flow.info.Types[call.Fun].IsType() {
		return flow.provesCallResult(
			call, 0, bindings, visitingObjects, visitingFunctions,
		)
	}
	if value := flow.info.Types[expression].Value; value != nil &&
		value.Kind() == constant.String {
		if s7APNamedEnumStringType(flow.info.TypeOf(expression)) {
			return s7APProvenNamedEnumConstant(flow.info, expression)
		}
		text := constant.StringVal(value)
		return !s7APStringContainsAbsolutePath(text) &&
			!s7APStringContainsTraversal(text)
	}
	if expressionType := flow.info.TypeOf(expression); expressionType != nil {
		if basic, ok := expressionType.Underlying().(*types.Basic); ok &&
			basic.Kind() != types.String && basic.Kind() != types.UntypedString {
			return true
		}
	}
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(value.Value)
		return err == nil && !s7APStringContainsAbsolutePath(text) &&
			!s7APStringContainsTraversal(text)
	case *ast.Ident:
		if value.Name == "nil" {
			return true
		}
		object := flow.info.Uses[value]
		if object == nil {
			object = flow.info.Defs[value]
		}
		if object == nil {
			return false
		}
		if result, ok := flow.multiResults[object]; ok {
			return flow.provesCallResult(
				result.call, result.index, bindings, visitingObjects, visitingFunctions,
			)
		}
		if visitingObjects[object] {
			return false
		}
		if bound := bindings[object]; bound != nil {
			return flow.provesNoAbsolute(bound, bindings, visitingObjects, visitingFunctions)
		}
		assigned := flow.assignments[object]
		if len(assigned) == 0 {
			assigned = flow.arguments[object]
		}
		if len(assigned) == 0 {
			return false
		}
		visitingObjects[object] = true
		defer delete(visitingObjects, object)
		for _, candidate := range assigned {
			if !flow.provesNoAbsolute(candidate, bindings, visitingObjects, visitingFunctions) {
				return false
			}
		}
		return true
	case *ast.BinaryExpr:
		return false
	case *ast.ParenExpr:
		return flow.provesNoAbsolute(value.X, bindings, visitingObjects, visitingFunctions)
	case *ast.UnaryExpr:
		return flow.provesNoAbsolute(value.X, bindings, visitingObjects, visitingFunctions)
	case *ast.SelectorExpr:
		if s7APProvenRelativeSelector(flow.info.TypeOf(value.X), value.Sel.Name) {
			return true
		}
		if s7APNamedEnumStringType(flow.info.TypeOf(value)) {
			return flow.provesNamedEnumSelector(
				value, bindings, visitingObjects, visitingFunctions,
			)
		}
		object := flow.info.Uses[value.Sel]
		if object == nil || visitingObjects[object] {
			return false
		}
		assigned := flow.assignments[object]
		if len(assigned) == 0 {
			return false
		}
		visitingObjects[object] = true
		defer delete(visitingObjects, object)
		for _, candidate := range assigned {
			if !flow.provesNoAbsolute(candidate, bindings, visitingObjects, visitingFunctions) {
				return false
			}
		}
		return true
	case *ast.CallExpr:
		return flow.provesCallResult(value, 0, bindings, visitingObjects, visitingFunctions)
	case *ast.CompositeLit:
		for _, element := range value.Elts {
			candidate, _ := element.(ast.Expr)
			if candidate == nil ||
				!flow.provesNoAbsolute(candidate, bindings, visitingObjects, visitingFunctions) {
				return false
			}
		}
		return true
	case *ast.KeyValueExpr:
		return flow.provesNoAbsolute(value.Value, bindings, visitingObjects, visitingFunctions)
	case *ast.IndexExpr:
		return flow.provesNoAbsolute(value.X, bindings, visitingObjects, visitingFunctions)
	case *ast.IndexListExpr:
		return flow.provesNoAbsolute(value.X, bindings, visitingObjects, visitingFunctions)
	case *ast.SliceExpr:
		return flow.provesNoAbsolute(value.X, bindings, visitingObjects, visitingFunctions)
	default:
		return false
	}
}

func (flow *s7APReportStringFlow) provesNamedEnumSelector(
	selector *ast.SelectorExpr,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	if selector == nil {
		return false
	}
	key := s7APEnumFieldKey(flow.info.TypeOf(selector.X), selector.Sel.Name)
	if key == "" || visitingFunctions["enum-field:"+key] {
		return false
	}
	assignments := flow.enumUniverse[key]
	if len(assignments) == 0 && !flow.enumDefaultUniverse[key] {
		return false
	}
	visitingFunctions["enum-field:"+key] = true
	defer delete(visitingFunctions, "enum-field:"+key)
	for _, assignment := range assignments {
		if assignment.flow == nil || !assignment.flow.provesNamedEnumValue(
			assignment.expression,
			nil,
			map[types.Object]bool{},
			visitingFunctions,
		) {
			return false
		}
	}
	return true
}

func (flow *s7APReportStringFlow) provesNamedEnumValue(
	expression ast.Expr,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	if expression == nil {
		return false
	}
	if s7APProvenNamedEnumConstant(flow.info, expression) {
		return true
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return flow.provesNamedEnumValue(
			value.X, bindings, visitingObjects, visitingFunctions,
		)
	case *ast.Ident:
		object := flow.info.ObjectOf(value)
		if object == nil || visitingObjects[object] {
			return false
		}
		if bound := bindings[object]; bound != nil {
			return flow.provesNamedEnumValue(
				bound, bindings, visitingObjects, visitingFunctions,
			)
		}
		candidates := append(
			append([]ast.Expr{}, flow.assignments[object]...),
			flow.arguments[object]...,
		)
		if len(candidates) == 0 {
			return false
		}
		visitingObjects[object] = true
		defer delete(visitingObjects, object)
		for _, candidate := range candidates {
			if !flow.provesNamedEnumValue(
				candidate, bindings, visitingObjects, visitingFunctions,
			) {
				return false
			}
		}
		return true
	case *ast.SelectorExpr:
		return flow.provesNamedEnumSelector(
			value, bindings, visitingObjects, visitingFunctions,
		)
	case *ast.CallExpr:
		if flow.info.Types[value.Fun].IsType() {
			return len(value.Args) == 1 &&
				s7APProvenNamedEnumConstant(flow.info, value.Args[0])
		}
		function := flow.calledFunction(value)
		if function == nil {
			return false
		}
		key := s7APFunctionKey(function)
		definition, ok := flow.universe[key]
		if !ok || definition.declaration == nil || definition.flow == nil ||
			visitingFunctions[key] {
			return false
		}
		visitingFunctions[key] = true
		defer delete(visitingFunctions, key)
		found := false
		safe := true
		s7APInspectFunctionReturns(
			definition.declaration.Body,
			func(statement *ast.ReturnStmt) bool {
				if len(statement.Results) != 1 {
					safe = false
					return false
				}
				found = true
				if !definition.flow.provesNamedEnumValue(
					statement.Results[0],
					nil,
					map[types.Object]bool{},
					visitingFunctions,
				) {
					safe = false
					return false
				}
				return true
			},
		)
		return found && safe
	default:
		return false
	}
}

func (flow *s7APReportStringFlow) provesNonEmptyRelative(
	expression ast.Expr,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	if value, ok := flow.resolveBoundString(expression, bindings, nil); ok {
		return value != "" && !s7APStringContainsAbsolutePath(value)
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return flow.provesNonEmptyRelative(
			value.X, bindings, visitingObjects, visitingFunctions,
		)
	case *ast.BinaryExpr:
		return value.Op == token.ADD &&
			flow.provesSafeConcatenation(
				value, bindings, visitingObjects, visitingFunctions,
			) &&
			flow.concatenationProvesNonEmptyRelative(
				value, bindings, visitingObjects, visitingFunctions,
			)
	case *ast.Ident:
		object := flow.info.ObjectOf(value)
		if object == nil || visitingObjects[object] {
			return false
		}
		if result, ok := flow.multiResults[object]; ok {
			return flow.provesCallResultNonEmpty(
				result.call, result.index, bindings, visitingObjects, visitingFunctions,
			)
		}
		if bound := bindings[object]; bound != nil {
			return flow.provesNonEmptyRelative(
				bound, bindings, visitingObjects, visitingFunctions,
			)
		}
		assigned := flow.assignments[object]
		if len(assigned) == 0 {
			return false
		}
		visitingObjects[object] = true
		defer delete(visitingObjects, object)
		for _, candidate := range assigned {
			if !flow.provesNonEmptyRelative(
				candidate, bindings, visitingObjects, visitingFunctions,
			) {
				return false
			}
		}
		return true
	case *ast.CallExpr:
		return flow.provesCallResultNonEmpty(
			value, 0, bindings, visitingObjects, visitingFunctions,
		)
	default:
		return false
	}
}

func (flow *s7APReportStringFlow) provesSafeConcatenation(
	expression *ast.BinaryExpr,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	var parts []ast.Expr
	s7APFlattenConcatenation(expression, &parts)
	if len(parts) < 2 {
		return false
	}
	var composed strings.Builder
	prefixNonEmpty := false
	for _, part := range parts {
		if text, ok := s7APDirectConcatenationLiteral(part); ok {
			if !s7APSafeConcatenationLiteral(text, prefixNonEmpty) {
				return false
			}
			composed.WriteString(text)
			if s7APLiteralEstablishesRelativePrefix(text, prefixNonEmpty) {
				prefixNonEmpty = true
			}
			continue
		}
		if identifier, _ := part.(*ast.Ident); identifier != nil {
			object := flow.info.ObjectOf(identifier)
			if object != nil && visitingObjects[object] {
				if !flow.provesSafeAccumulator(
					object, bindings, visitingObjects, visitingFunctions,
				) {
					return false
				}
				composed.WriteString("s7-safe")
				continue
			}
		}
		if !flow.provesNoAbsolute(
			part, bindings, visitingObjects, visitingFunctions,
		) {
			return false
		}
		composed.WriteString("s7-safe")
		if flow.provesNonEmptyRelative(
			part, bindings, visitingObjects, visitingFunctions,
		) {
			prefixNonEmpty = true
		}
	}
	return s7APNormalizedRelativeString(composed.String())
}

func (flow *s7APReportStringFlow) concatenationProvesNonEmptyRelative(
	expression *ast.BinaryExpr,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	var parts []ast.Expr
	s7APFlattenConcatenation(expression, &parts)
	prefixNonEmpty := false
	for _, part := range parts {
		if text, ok := s7APDirectConcatenationLiteral(part); ok {
			if !s7APSafeConcatenationLiteral(text, prefixNonEmpty) {
				return false
			}
			if s7APLiteralEstablishesRelativePrefix(text, prefixNonEmpty) {
				prefixNonEmpty = true
			}
			continue
		}
		if identifier, _ := part.(*ast.Ident); identifier != nil {
			object := flow.info.ObjectOf(identifier)
			if object != nil && visitingObjects[object] {
				if !flow.provesSafeAccumulator(
					object, bindings, visitingObjects, visitingFunctions,
				) {
					return false
				}
				continue
			}
		}
		if !flow.provesNoAbsolute(
			part, bindings, visitingObjects, visitingFunctions,
		) {
			return false
		}
		if flow.provesNonEmptyRelative(
			part, bindings, visitingObjects, visitingFunctions,
		) {
			prefixNonEmpty = true
		}
	}
	return prefixNonEmpty
}

func s7APDirectConcatenationLiteral(expression ast.Expr) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.ParenExpr:
		return s7APDirectConcatenationLiteral(value.X)
	default:
		return "", false
	}
}

func (flow *s7APReportStringFlow) provesSafeAccumulator(
	object types.Object,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	found := false
	for _, candidate := range flow.assignments[object] {
		if s7APExpressionUsesObject(candidate, flow.info, object) {
			continue
		}
		found = true
		if !flow.provesNoAbsolute(
			candidate, bindings, visitingObjects, visitingFunctions,
		) {
			return false
		}
	}
	return found
}

func s7APExpressionUsesObject(
	expression ast.Expr,
	info *types.Info,
	want types.Object,
) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, _ := node.(*ast.Ident)
		if identifier != nil && info.ObjectOf(identifier) == want {
			found = true
			return false
		}
		return !found
	})
	return found
}

func s7APFlattenConcatenation(expression ast.Expr, parts *[]ast.Expr) {
	binary, _ := expression.(*ast.BinaryExpr)
	if binary == nil || binary.Op != token.ADD {
		*parts = append(*parts, expression)
		return
	}
	s7APFlattenConcatenation(binary.X, parts)
	s7APFlattenConcatenation(binary.Y, parts)
}

func s7APSafeConcatenationLiteral(value string, prefixNonEmpty bool) bool {
	if strings.ContainsRune(value, '\x00') ||
		s7APStringContainsTraversal(value) {
		return false
	}
	if value != "" && strings.Trim(value, `/\`) == "" {
		return true
	}
	if !s7APStringContainsAbsolutePath(value) {
		return true
	}
	if !prefixNonEmpty || strings.ContainsAny(value, " \t") ||
		(!strings.HasPrefix(value, "/") && !strings.HasPrefix(value, `\`)) {
		return false
	}
	remainder := strings.TrimLeft(value, `/\`)
	return remainder != "" &&
		!s7APStringContainsAbsolutePath(remainder) &&
		!s7APStringContainsTraversal(remainder)
}

func s7APLiteralEstablishesRelativePrefix(value string, prefixNonEmpty bool) bool {
	if value == "" {
		return false
	}
	trimmed := strings.Trim(value, `/\`)
	if trimmed == "" {
		return false
	}
	if prefixNonEmpty && (strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`)) {
		return true
	}
	return !s7APStringContainsAbsolutePath(value) &&
		!s7APStringContainsTraversal(value)
}

func s7APNormalizedRelativeString(value string) bool {
	if s7APStringContainsAbsolutePath(value) ||
		s7APStringContainsTraversal(value) {
		return false
	}
	normalized := path.Clean(strings.ReplaceAll(value, `\`, "/"))
	return normalized != ".." && !strings.HasPrefix(normalized, "../") &&
		!strings.HasPrefix(normalized, "/")
}

func s7APStringContainsTraversal(value string) bool {
	for _, segment := range strings.FieldsFunc(
		strings.ReplaceAll(value, `\`, "/"),
		func(r rune) bool {
			switch r {
			case '/', ' ', '\t', '"', '\'', '`', '(', ')', '[', ']', '{', '}', ',', ';':
				return true
			default:
				return false
			}
		},
	) {
		if segment == ".." {
			return true
		}
	}
	return false
}

func (flow *s7APReportStringFlow) provesCallResultNonEmpty(
	call *ast.CallExpr,
	resultIndex int,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	function := flow.calledFunction(call)
	if function == nil {
		return false
	}
	key := s7APFunctionKey(function)
	definition, ok := flow.universe[key]
	visit := key + "#" + strconv.Itoa(resultIndex) + "#nonempty"
	if !ok || definition.declaration == nil || definition.flow == nil ||
		visitingFunctions[visit] {
		return false
	}
	visitingFunctions[visit] = true
	defer delete(visitingFunctions, visit)

	callBindings := map[types.Object]ast.Expr{}
	argumentIndex := 0
	if definition.declaration.Type.Params != nil {
		for _, field := range definition.declaration.Type.Params.List {
			for _, parameter := range field.Names {
				if argumentIndex >= len(call.Args) {
					return false
				}
				argument := call.Args[argumentIndex]
				if s7APReportStringValueType(flow.info.TypeOf(argument)) {
					if !flow.provesNoAbsolute(
						argument, bindings, visitingObjects, visitingFunctions,
					) {
						return false
					}
					if object := definition.flow.info.Defs[parameter]; object != nil {
						callBindings[object] = &ast.BasicLit{
							Kind: token.STRING, Value: strconv.Quote("s7-safe"),
						}
					}
				}
				argumentIndex++
			}
		}
	}
	found := false
	safe := true
	s7APInspectFunctionReturns(definition.declaration.Body, func(statement *ast.ReturnStmt) bool {
		if resultIndex >= len(statement.Results) {
			safe = false
			return false
		}
		selected := statement.Results[resultIndex]
		if text, ok := definition.flow.resolveBoundString(selected, callBindings, nil); ok &&
			text == "" && s7APReturnHasNonNilErrorSibling(
			statement, resultIndex, definition.flow.info,
		) {
			return true
		}
		found = true
		if !definition.flow.provesNonEmptyRelative(
			selected, callBindings, map[types.Object]bool{}, visitingFunctions,
		) {
			safe = false
			return false
		}
		return true
	})
	return found && safe
}

func s7APReturnHasNonNilErrorSibling(
	statement *ast.ReturnStmt,
	selected int,
	info *types.Info,
) bool {
	errorObject := types.Universe.Lookup("error")
	if errorObject == nil {
		return false
	}
	for index, result := range statement.Results {
		if index == selected {
			continue
		}
		identifier, _ := result.(*ast.Ident)
		if identifier != nil && identifier.Name == "nil" {
			continue
		}
		resultType := info.TypeOf(result)
		if resultType != nil && types.AssignableTo(resultType, errorObject.Type()) {
			return true
		}
	}
	return false
}

func (flow *s7APReportStringFlow) resolveBoundString(
	expression ast.Expr,
	bindings map[types.Object]ast.Expr,
	visiting map[types.Object]bool,
) (string, bool) {
	if expression == nil {
		return "", false
	}
	if value := flow.info.Types[expression].Value; value != nil &&
		value.Kind() == constant.String {
		return constant.StringVal(value), true
	}
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.ParenExpr:
		return flow.resolveBoundString(value.X, bindings, visiting)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := flow.resolveBoundString(value.X, bindings, visiting)
		right, rightOK := flow.resolveBoundString(value.Y, bindings, visiting)
		return left + right, leftOK && rightOK
	case *ast.Ident:
		object := flow.info.ObjectOf(value)
		if object == nil || visiting[object] {
			return "", false
		}
		bound := bindings[object]
		if bound == nil {
			assigned := flow.assignments[object]
			if len(assigned) == 1 {
				bound = assigned[0]
			}
		}
		if bound == nil {
			return "", false
		}
		next := maps.Clone(visiting)
		if next == nil {
			next = map[types.Object]bool{}
		}
		next[object] = true
		return flow.resolveBoundString(bound, bindings, next)
	default:
		return "", false
	}
}

func (flow *s7APReportStringFlow) provesCallResult(
	call *ast.CallExpr,
	resultIndex int,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	if call == nil {
		return false
	}
	function := flow.calledFunction(call)
	if s7APProvenExternalStringCall(
		flow, call, function, bindings, visitingObjects, visitingFunctions,
	) {
		return resultIndex == 0
	}
	if function == nil {
		return false
	}
	key := s7APFunctionKey(function)
	definition, ok := flow.universe[key]
	if !ok || definition.declaration == nil || definition.flow == nil ||
		visitingFunctions[key] {
		return false
	}
	if s7APProvenSanitizerResult(definition, resultIndex) {
		return true
	}
	visitingFunctions[key] = true
	defer delete(visitingFunctions, key)

	callBindings := map[types.Object]ast.Expr{}
	argumentIndex := 0
	if definition.declaration.Type.Params != nil {
		for _, field := range definition.declaration.Type.Params.List {
			for _, parameter := range field.Names {
				if argumentIndex >= len(call.Args) {
					return false
				}

				argument := call.Args[argumentIndex]
				if s7APReportStringValueType(flow.info.TypeOf(argument)) &&
					!flow.provesNoAbsolute(
						argument, bindings, visitingObjects, visitingFunctions,
					) {
					return false
				}
				if object := definition.flow.info.Defs[parameter]; object != nil &&
					s7APReportStringValueType(object.Type()) {
					callBindings[object] = &ast.BasicLit{
						Kind: token.STRING, Value: strconv.Quote("s7-safe"),
					}
				}
				argumentIndex++
			}
		}
	}
	found := false
	safe := true
	s7APInspectFunctionReturns(definition.declaration.Body, func(statement *ast.ReturnStmt) bool {
		if resultIndex >= len(statement.Results) {
			safe = false
			return false
		}
		found = true
		if !definition.flow.provesNoAbsolute(
			statement.Results[resultIndex],
			callBindings,
			map[types.Object]bool{},
			visitingFunctions,
		) {
			safe = false
			return false
		}
		return true
	})
	return found && safe
}

func s7APProvenSanitizerResult(
	definition s7APFunctionDefinition,
	resultIndex int,
) bool {
	key := s7APFunctionKey(definition.function) + "|" + strconv.Itoa(resultIndex)
	expected := map[string]string{
		"github.com/tesseracode/tesserapatch/internal/intent||CanonicalSlug|0": "f7dc7d0c4d2dff3e04400e87a7d2a003ca93190525118f62b404b99862fe4bab",
		"github.com/tesseracode/tesserapatch/internal/redact||Scan|0":          "0ea9568e490b119f94a56aa6cec42b058abd6407cec806d51f796f562d6c14f3",
	}
	want, ok := expected[key]
	if !ok {
		return false
	}
	return s7APFunctionBodyHash(definition.declaration) == want
}

func s7APFunctionBodyHash(function *ast.FuncDecl) string {
	if function == nil || function.Body == nil {
		return ""
	}
	var body bytes.Buffer
	if err := format.Node(&body, token.NewFileSet(), function.Body); err != nil {
		return ""
	}
	sum := sha256.Sum256(body.Bytes())
	return fmt.Sprintf("%x", sum[:])
}

func s7APInspectFunctionReturns(body *ast.BlockStmt, visit func(*ast.ReturnStmt) bool) {
	if body == nil {
		return
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			return visit(value)
		default:
			return true
		}
	})
}

func s7APProvenExternalStringCall(
	flow *s7APReportStringFlow,
	call *ast.CallExpr,
	function *types.Func,
	bindings map[types.Object]ast.Expr,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) bool {
	if typed := flow.info.Types[call.Fun]; typed.IsType() {
		if s7APNamedEnumStringType(flow.info.TypeOf(call)) {
			return len(call.Args) == 1 &&
				s7APProvenNamedEnumConstant(flow.info, call.Args[0])
		}
		return len(call.Args) == 1 &&
			flow.provesNoAbsolute(
				call.Args[0], bindings, visitingObjects, visitingFunctions,
			)
	}
	identifier, _ := call.Fun.(*ast.Ident)
	if identifier != nil {
		switch identifier.Name {
		case "append":
			for index, argument := range call.Args {
				if index == 0 {
					if accumulator, _ := argument.(*ast.Ident); accumulator != nil {
						object := flow.info.Uses[accumulator]
						if object == nil {
							object = flow.info.Defs[accumulator]
						}
						if object != nil && visitingObjects[object] {
							continue
						}
					}
				}
				if !flow.provesNoAbsolute(
					argument, bindings, visitingObjects, visitingFunctions,
				) {
					return false
				}
			}
			return len(call.Args) != 0
		case "make":
			return true
		case "string":
			return len(call.Args) == 1 &&
				flow.provesNoAbsolute(
					call.Args[0], bindings, visitingObjects, visitingFunctions,
				)
		}
	}
	if s7APIsStringsBuilderMethod(function, "String") {
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil {
			return false
		}
		identifier, _ := selector.X.(*ast.Ident)
		object := flow.info.ObjectOf(identifier)
		writes := flow.builderWrites[object]
		if object == nil || len(writes) == 0 {
			return false
		}
		for _, write := range writes {
			if !flow.provesNoAbsolute(
				write, bindings, visitingObjects, visitingFunctions,
			) {
				return false
			}
		}
		return true
	}
	if s7APIsNamedMethod(function, "time", "Duration", "String") {
		return true
	}
	if s7APIsNamedMethod(function, "os", "File", "Readdirnames") {
		return true
	}
	if function == nil || function.Pkg() == nil {
		return false
	}
	key := function.Pkg().Path() + "." + function.Name()
	switch key {
	case "encoding/hex.EncodeToString",
		"fmt.Sprint",
		"fmt.Sprintf",
		"path.Base",
		"path.Join",
		"path/filepath.Join",
		"path/filepath.ToSlash",
		"strconv.FormatInt",
		"strconv.FormatUint",
		"strconv.Itoa",
		"strings.Join",
		"strings.ReplaceAll",
		"strings.Split",
		"strings.TrimPrefix",
		"strings.TrimSpace",
		"strings.TrimSuffix":
		for _, argument := range call.Args {
			if s7APReportStringValueType(flow.info.TypeOf(argument)) &&
				!flow.provesNoAbsolute(
					argument, bindings, visitingObjects, visitingFunctions,
				) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func s7APExpressionUsesIdentifier(expression ast.Expr, name string) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func s7APIsStringsBuilderMethod(function *types.Func, name string) bool {
	return s7APIsNamedMethod(function, "strings", "Builder", name)
}

func s7APIsNamedMethod(
	function *types.Func,
	packagePath, receiverName, methodName string,
) bool {
	if function == nil || function.Name() != methodName {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return false
	}
	receiver := signature.Recv().Type()
	if pointer, _ := receiver.(*types.Pointer); pointer != nil {
		receiver = pointer.Elem()
	}
	named, _ := receiver.(*types.Named)
	return named != nil && named.Obj() != nil && named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == receiverName
}

func s7APProvenRelativeSelector(receiver types.Type, field string) bool {
	for {
		pointer, ok := receiver.(*types.Pointer)
		if !ok {
			break
		}
		receiver = pointer.Elem()
	}
	named, _ := receiver.(*types.Named)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	key := named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + field
	switch key {
	// Exact validated model fields: path-bearing members are repository-relative,
	// while the remaining members are closed hashes, IDs, actions, or prose
	// assembled by the archive/prepare presentation model. Destination field
	// names are never used as evidence of safety.
	case "github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReplacement.Path",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReplacement.Blob",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveBlobObservation.Path",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveBlobProbe.Kind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceReport.Path",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceReport.GenerationID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceReport.Disposition",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceTarget.Path",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceTarget.ArtifactID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceTarget.GenerationID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceTarget.Hash",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReferenceTarget.WireState",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveBlobObservation.Hash",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.Hashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.PredictedHashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.Repair",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.Kind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.Class",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.ResultingClasses",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveOrphan.Path",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairInstance.Path",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairClassReport.Hashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairClassReport.Paths",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.Paths",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStage.RepairCWD",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRemainingRepairs.RepairedClass",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveNextRepairStage.Kind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveNextRepairStage.Class",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveSnapshot.Feature",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.Feature",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.Action",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.Outcome",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.GenerationIDs",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.Hashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.PendingHashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgePlan.SelectorKind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveAppendResult.NewOrphanHashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePreimageReference.ContentSHA256",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePreimageReference.BlobRel",
		"github.com/tesseracode/tesserapatch/internal/intentpub.StageResult.StageRel",
		"github.com/tesseracode/tesserapatch/internal/intentpub.Result.Orphans",
		"github.com/tesseracode/tesserapatch/internal/intentpub.Result.Published",
		"github.com/tesseracode/tesserapatch/internal/intentpub.Result.Restored",
		"github.com/tesseracode/tesserapatch/internal/intentpub.Error.ArtifactID",
		"github.com/tesseracode/tesserapatch/internal/intent.Report.Slug",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenNote.Generator",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenNote.Advisories",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenNote.ErrorClass",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenNote.DeadlineClass",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveError.Hash",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveError.ArtifactID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveError.GenerationID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveError.Class",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveError.Code",
		"github.com/tesseracode/tesserapatch/internal/cli.intentArchiveRepairPresentation.Repair",
		"github.com/tesseracode/tesserapatch/internal/cli.intentArchiveRepairPresentation.Retry",
		"github.com/tesseracode/tesserapatch/internal/cli.intentArchiveRepairPresentation.RetryCWD":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.prepareRecoveryReport.Retry",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareRecoveryReport.RestoredEntries",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareArtifactReport.Path",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareActionReport.Path",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.CompletedHashes",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.PendingHash",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.RemainingHashes",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.Resume",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.Retry",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.RetryCWD",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePurgeProgressReport.State":
		return true
	case "github.com/tesseracode/tesserapatch/internal/intentlock.Error.Code",
		"github.com/tesseracode/tesserapatch/internal/intentlock.Error.Class":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.intentArchivePurgeOptions.blobs",
		"github.com/tesseracode/tesserapatch/internal/cli.intentArchivePurgeOptions.generations":
		return true
	case "github.com/tesseracode/tesserapatch/internal/workflow.doctorCheck.id":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.prepareGeneratedArtifact.deadlineScope",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareGeneratedFailure.deadlineScope",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareGeneratedArtifact.id",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareGeneratedFailure.artifactID":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.preparePreimageReferences.indexRaw",
		"github.com/tesseracode/tesserapatch/internal/cli.preparePreimageReferences.statusRaw":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.intentArchiveListReport.Slug",
		"github.com/tesseracode/tesserapatch/internal/cli.intentArchivePurgeReport.Slug":
		return true
	case "github.com/tesseracode/tesserapatch/internal/intent.Report.FeatureState":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.preparePlan.action":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.preparePlan.generated":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.preparePublishReport.Slug",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareArchiveReport.BlobsDir",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareArtifactReport.ArchivedBlob":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.prepareAbandonEvidenceError.rel":
		return true
	case "github.com/tesseracode/tesserapatch/internal/cli.prepareAbandonMoveError.entryRel",
		"github.com/tesseracode/tesserapatch/internal/cli.prepareAbandonMoveError.evidenceRel":
		return true
	// Closed identifiers, enum strings and content hashes are non-path values.
	case "github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.Action",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.Outcome",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.CompletedHashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.FinalizedHashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.RemovedBlobs",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.PendingHash",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.RemainingHashes",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.Resume",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResult.State",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveAppendPlan.generationID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveGeneration.GenerationID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveGeneration.Mode",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReplacement.ArtifactID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReplacement.ContentSHA256",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveReplacementInput.ArtifactID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveOrphan.Hash",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairInstance.Hash":
		return true
	default:
		return false
	}
}

func s7APNamedEnumStringType(value types.Type) bool {
	named, _ := value.(*types.Named)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	key := named.Obj().Pkg().Path() + "." + named.Obj().Name()
	switch key {
	case "github.com/tesseracode/tesserapatch/internal/cli.prepareMode",
		"github.com/tesseracode/tesserapatch/internal/intentpub.ArtifactID",
		"github.com/tesseracode/tesserapatch/internal/workflow.GeneratorKind",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenAdvisory",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenDeadlineClass",
		"github.com/tesseracode/tesserapatch/internal/workflow.GenErrorClass",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveAction",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveArtifactID",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveBlobKind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveBlobState",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveDisposition",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveErrorCode",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveIdentityToken",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeOutcome",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchivePurgeResume",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairClass",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveRepairStageKind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveSelectorKind",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveStoragePhase",
		"github.com/tesseracode/tesserapatch/internal/store.IntentArchiveWireState":
		return true
	default:
		return false
	}
}

func s7APProvenNamedEnumConstant(info *types.Info, expression ast.Expr) bool {
	for {
		paren, _ := expression.(*ast.ParenExpr)
		if paren == nil {
			break
		}
		expression = paren.X
	}
	var object types.Object
	switch value := expression.(type) {
	case *ast.Ident:
		object = info.ObjectOf(value)
	case *ast.SelectorExpr:
		object = info.ObjectOf(value.Sel)
	}
	declared, _ := object.(*types.Const)
	if declared == nil || !s7APNamedEnumStringType(declared.Type()) ||
		declared.Val() == nil || declared.Val().Kind() != constant.String {
		return false
	}
	text := constant.StringVal(declared.Val())
	return !s7APStringContainsAbsolutePath(text) &&
		!s7APStringContainsTraversal(text)
}

func s7APControlledReportConstructor(name string) bool {
	switch name {
	case "buildIntentArchivePendingPurge",
		"intentArchivePendingJournalRefusal",
		"intentArchiveRefusalFromError",
		"intentArchiveSimpleRefusal",
		"newIntentArchiveListReport",
		"newIntentArchivePurgeReport",
		"newPreparePublishReport",
		"prepareAdvisory",
		"refusePrepare":
		return true
	default:
		return false
	}
}

func (flow *s7APReportStringFlow) calledFunction(call *ast.CallExpr) *types.Func {
	if call == nil {
		return nil
	}
	switch function := call.Fun.(type) {
	case *ast.Ident:
		object := flow.info.Uses[function]
		if typed, _ := object.(*types.Func); typed != nil {
			return typed
		}
		variable, _ := object.(*types.Var)
		if variable == nil {
			return nil
		}
		return flow.functionAssignedTo(variable, map[types.Object]bool{})
	case *ast.SelectorExpr:
		typed, _ := flow.info.Uses[function.Sel].(*types.Func)
		return typed
	default:
		return nil
	}
}

func (flow *s7APReportStringFlow) functionAssignedTo(
	object types.Object,
	visiting map[types.Object]bool,
) *types.Func {
	if object == nil || visiting[object] {
		return nil
	}
	visiting[object] = true
	defer delete(visiting, object)
	for _, expression := range flow.assignments[object] {
		switch value := expression.(type) {
		case *ast.Ident:
			target := flow.info.Uses[value]
			if function, _ := target.(*types.Func); function != nil {
				return function
			}
			if function := flow.functionAssignedTo(target, visiting); function != nil {
				return function
			}
		case *ast.SelectorExpr:
			if function, _ := flow.info.Uses[value.Sel].(*types.Func); function != nil {
				return function
			}
		}
	}
	return nil
}

func s7APFunctionKey(function *types.Func) string {
	if function == nil {
		return ""
	}
	pkg := ""
	if function.Pkg() != nil {
		pkg = function.Pkg().Path()
	}
	receiver := ""
	if signature, _ := function.Type().(*types.Signature); signature != nil &&
		signature.Recv() != nil {
		receiver = types.TypeString(signature.Recv().Type(), func(other *types.Package) string {
			if other == nil {
				return ""
			}
			return other.Path()
		})
	}
	return pkg + "|" + receiver + "|" + function.Name()
}

func s7APProvenRelativeSelectorExpression(info *types.Info, expression ast.Expr) bool {
	selector, _ := expression.(*ast.SelectorExpr)
	return selector != nil &&
		s7APProvenRelativeSelector(info.TypeOf(selector.X), selector.Sel.Name)
}

func s7APStringContainsAbsolutePath(value string) bool {
	if value != "/" && filepath.IsAbs(value) {
		return true
	}
	for _, field := range strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '"', '\'', '`', '(', ')', '[', ']', '{', '}', ',', ';':
			return true
		default:
			return false
		}
	}) {
		if field != "/" && filepath.IsAbs(field) {
			return true
		}
		if len(field) >= 3 &&
			((field[0] >= 'A' && field[0] <= 'Z') || (field[0] >= 'a' && field[0] <= 'z')) &&
			field[1] == ':' && (field[2] == '\\' || field[2] == '/') {
			return true
		}
		if strings.HasPrefix(field, `\\`) {
			return true
		}
	}
	return false
}

func validateS7APGitSourcePrivacy(gitSource, cliSource string) error {
	fileset := token.NewFileSet()
	gitFile, err := parser.ParseFile(fileset, "ignore.go", gitSource, 0)
	if err != nil {
		return err
	}
	expected := map[string][]string{
		"DiscoverGitState":              {`"rev-parse"`, `"--is-inside-work-tree"`},
		"IsIgnoredWithState":            {`"check-ignore"`, `"-q"`, `"--no-index"`, `"--"`, "disarmLeadingColon(repoRelative)"},
		"AnythingTrackedUnderWithState": {`"--literal-pathspecs"`, `"ls-files"`, `"--"`, `".tpatch/local/"`},
		"IsTpatchTrackedWithState":      {`"ls-files"`, `"--"`, `".tpatch"`},
	}
	for functionName, wantArgs := range expected {
		function, err := s7APGitFunction(gitFile, functionName)
		if err != nil {
			return err
		}
		calls := s7APGitCalls(function.Body, "runGitProcess")
		if len(calls) != 1 {
			return fmt.Errorf("%s Git process requests = %d, want 1", functionName, len(calls))
		}
		args, gates, err := s7APGitRequest(fileset, calls[0])
		if err != nil {
			return err
		}
		if fmt.Sprint(args) != fmt.Sprint(wantArgs) || gates != 1 {
			return fmt.Errorf("%s argv/privacy = %v/%d, want %v/1",
				functionName, args, gates, wantArgs)
		}
		for _, arg := range args {
			if strings.HasPrefix(strings.Trim(arg, `"`), "/") {
				return fmt.Errorf("%s has absolute Git argument %q", functionName, arg)
			}
		}
	}
	cliFile, err := parser.ParseFile(fileset, "prepare_publish.go", cliSource, 0)
	if err != nil {
		return err
	}
	runPrepare, err := s7APGitFunction(cliFile, "runPreparePublish")
	if err != nil {
		return err
	}
	for name := range expected {
		count := 0
		ast.Inspect(runPrepare.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, _ := call.Fun.(*ast.SelectorExpr)
			if selector == nil {
				return true
			}
			pkg, _ := selector.X.(*ast.Ident)
			if pkg != nil && pkg.Name == "gitutil" &&
				selector.Sel.Name == name {
				count++
			}
			return true
		})
		if count != 1 {
			return fmt.Errorf("runPreparePublish %s calls = %d, want 1", name, count)
		}
	}
	return nil
}

func s7APGitFunction(file *ast.File, name string) (*ast.FuncDecl, error) {
	var found *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Body == nil || function.Name.Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("duplicate function %s", name)
		}
		found = function
	}
	if found == nil {
		return nil, fmt.Errorf("missing function %s", name)
	}
	return found, nil
}

func s7APGitCalls(body *ast.BlockStmt, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, _ := call.Fun.(*ast.Ident)
		if ident != nil && ident.Name == name {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func s7APGitRequest(
	fileset *token.FileSet,
	call *ast.CallExpr,
) ([]string, int, error) {
	if len(call.Args) != 1 {
		return nil, 0, errors.New("runGitProcess request is not singular")
	}
	request, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return nil, 0, errors.New("runGitProcess request is not literal")
	}
	var args []string
	gates := 0
	for _, element := range request.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, _ := kv.Key.(*ast.Ident)
		if key == nil {
			continue
		}
		switch key.Name {
		case "args":
			values, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				return nil, 0, errors.New("Git args are not literal")
			}
			for _, value := range values.Elts {
				var rendered bytes.Buffer
				if err := format.Node(&rendered, fileset, value); err != nil {
					return nil, 0, err
				}
				args = append(args, rendered.String())
			}
		case "env":
			ast.Inspect(kv.Value, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, _ := call.Fun.(*ast.Ident)
				if ident != nil && ident.Name == "prepareGitEnvironment" {
					gates++
				}
				return true
			})
		}
	}
	return args, gates, nil
}

func s7APWalkGitReport(path string, value any, forbidden []string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if err := s7APWalkGitReport(path+"."+key, child, forbidden); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := s7APWalkGitReport(fmt.Sprintf("%s[%d]", path, index), child, forbidden); err != nil {
				return err
			}
		}
	case string:
		if filepath.IsAbs(typed) {
			return fmt.Errorf("%s contains absolute path %q", path, typed)
		}
		for _, secret := range forbidden {
			if secret != "" && strings.Contains(typed, secret) {
				return fmt.Errorf("%s leaks absolute input %q", path, secret)
			}
		}
	case nil, bool, float64:
	default:
		return errors.New("unsupported JSON report value at " + path)
	}
	return nil
}
