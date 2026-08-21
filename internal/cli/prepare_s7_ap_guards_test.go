//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestS7APProvenanceGuard(t *testing.T) {
	t.Run("PIB-477", func(t *testing.T) {
		sources, err := s6PrepareWriteSources(t)
		if err != nil {
			t.Fatal(err)
		}
		sources["internal/workflow/generate_analysis.go"] =
			s6RepoFile(t, "internal/workflow/generate_analysis.go")
		if err := validateS7APProducedBySources(sources); err != nil {
			t.Fatal(err)
		}
		wrong := s6CloneSourceSet(sources)
		source := wrong["internal/cli/prepare_publish.go"]
		anchor := "type prepareArchiveReport struct {"
		if !strings.Contains(source, anchor) {
			t.Fatal("PIB-477 schema mutation anchor missing")
		}
		wrong["internal/cli/prepare_publish.go"] = strings.Replace(
			source,
			anchor,
			"type prepareArchiveReport struct {\n\tProducedBy string `json:\"produced_by\"`",
			1,
		)
		if err := validateS7APProducedBySources(wrong); err == nil {
			t.Fatal("PIB-477 same validator accepted produced_by as a controlled schema key")
		}
		for name, body := range map[string]string{
			"computed-index-key": `
	const key = "produced_" + "by"
	schemaMap := map[string]string{}
	schemaMap[key] = "generator"
	_, _ = json.Marshal(schemaMap)
`,
			"computed-composite-key": `
	const key = "produced_" + "by"
	_, _ = json.Marshal(map[string]string{key: "generator"})
`,
			"unresolved-dynamic-key": `
	schemaMap := map[string]string{}
	schemaMap[report.Command] = "generator"
	_, _ = json.Marshal(schemaMap)
`,
		} {
			mutated := s6CloneSourceSet(sources)
			const producer = "func writePreparePublishReport(cmd *cobra.Command, report preparePublishReport) error {"
			mutated["internal/cli/prepare_publish.go"] = strings.Replace(
				mutated["internal/cli/prepare_publish.go"],
				producer,
				producer+body,
				1,
			)
			if err := validateS7APProducedBySources(mutated); err == nil {
				t.Fatalf("PIB-477 same validator accepted %s", name)
			}
		}
		for rel, mutation := range map[string]struct {
			anchor string
			body   string
		}{
			"internal/store/intent_archive.go": {
				anchor: "func EncodeIntentArchiveIndex(index IntentArchiveIndex) ([]byte, error) {",
				body: `
	type s7APCustomValue string
	_, _ = json.Marshal(map[string]s7APCustomValue{"produced_" + "by": "generator"})`,
			},
			"internal/intentpub/journal.go": {
				anchor: "func EncodeJournal(journal Journal) ([]byte, error) {",
				body: `
	_, _ = json.Marshal(map[string]string{"produced_" + "by": "generator"})`,
			},
		} {
			mutated := s6CloneSourceSet(sources)
			before := mutated[rel]
			mutated[rel] = strings.Replace(
				before,
				mutation.anchor,
				mutation.anchor+mutation.body,
				1,
			)
			if mutated[rel] == before {
				t.Fatalf("PIB-477 persisted producer mutation anchor missing in %s", rel)
			}
			if err := validateS7APProducedBySources(mutated); err == nil {
				t.Fatalf("PIB-477 same validator accepted produced_by in %s", rel)
			}
		}
		interfaceMap := s6CloneSourceSet(sources)
		const journalAnchor = "func EncodeJournal(journal Journal) ([]byte, error) {"
		interfaceMap["internal/intentpub/journal.go"] = strings.Replace(
			interfaceMap["internal/intentpub/journal.go"],
			journalAnchor,
			journalAnchor+`
	var controlled any = map[string]string{"produced_" + "by": "generator"}
	_, _ = json.Marshal(controlled)`,
			1,
		)
		if err := validateS7APProducedBySources(interfaceMap); err == nil {
			t.Fatal("PIB-477 same validator accepted a string-key map through interface dataflow")
		}
		for name, body := range map[string]string{
			"direct-selector-composite": `
	type s7APSelectorPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	var controlled s7APSelectorPayload
	controlled.Metadata = map[string]string{"produced_" + "by": "generator"}
	_, _ = json.Marshal(controlled)
`,
			"pointer-alias-nested-selector": `
	type s7APNestedMetadata struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	type s7APNestedPayload struct {
		Inner s7APNestedMetadata ` + "`json:\"inner\"`" + `
	}
	controlled := &s7APNestedPayload{}
	alias := controlled
	alias.Inner.Metadata = map[string]string{"produced_" + "by": "generator"}
	_, _ = json.Marshal(controlled)
`,
			"field-map-index-write": `
	type s7APIndexedPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APIndexedPayload{}
	controlled.Metadata = map[string]string{}
	const key = "produced_" + "by"
	controlled.Metadata[key] = "generator"
	_, _ = json.Marshal(controlled)
`,
			"field-map-alias-write": `
	type s7APAliasedMapPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APAliasedMapPayload{}
	controlled.Metadata = map[string]string{}
	metadata := controlled.Metadata
	const key = "produced_" + "by"
	metadata[key] = "generator"
	_, _ = json.Marshal(controlled)
`,
			"field-map-dynamic-write": `
	type s7APDynamicPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APDynamicPayload{}
	controlled.Metadata = map[string]string{}
	controlled.Metadata[report.Command] = "generator"
	_, _ = json.Marshal(controlled)
`,
		} {
			mutated := s6CloneSourceSet(sources)
			const producer = "func writePreparePublishReport(cmd *cobra.Command, report preparePublishReport) error {"
			before := mutated["internal/cli/prepare_publish.go"]
			mutated["internal/cli/prepare_publish.go"] = strings.Replace(
				before, producer, producer+body, 1,
			)
			if mutated["internal/cli/prepare_publish.go"] == before {
				t.Fatalf("PIB-477 %s selector mutation anchor missing", name)
			}
			if err := validateS7APProducedBySources(mutated); err == nil {
				t.Fatalf("PIB-477 same validator accepted %s", name)
			}
		}
		for _, sensitivity := range []struct {
			name       string
			body       string
			suffix     string
			importMaps bool
		}{
			{
				name: "parameter-side-write",
				body: `
	type s7APCallPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APCallPayload{Metadata: map[string]string{}}
	s7APSetMetadata(controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
				suffix: `
func s7APSetMetadata(m map[string]string) {
	m["produced_"+"by"] = "generator"
}
`,
			},
			{
				name: "nested-helper-chain",
				body: `
	type s7APNestedCallPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APNestedCallPayload{Metadata: map[string]string{}}
	s7APOuterMetadata(controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
				suffix: `
func s7APOuterMetadata(m map[string]string) { s7APInnerMetadata(m) }
func s7APInnerMetadata(m map[string]string) {
	m["produced_"+"by"] = "generator"
}
`,
			},
			{
				name: "pointer-to-map",
				body: `
	type s7APPointerPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APPointerPayload{Metadata: map[string]string{}}
	s7APMutateMetadataPointer(&controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
				suffix: `
func s7APMutateMetadataPointer(m *map[string]string) {
	(*m)["produced_"+"by"] = "generator"
}
`,
			},
			{
				name: "returning-wrapper-chain",
				body: `
	type s7APReturnPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APReturnPayload{Metadata: map[string]string{}}
	controlled.Metadata = s7APWrapMetadata(controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
				suffix: `
func s7APWrapMetadata(m map[string]string) map[string]string {
	return s7APReturnMetadata(m)
}
func s7APReturnMetadata(m map[string]string) map[string]string {
	m["produced_"+"by"] = "generator"
	return m
}
`,
			},
			{
				name: "clear-then-write",
				body: `
	type s7APClearPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APClearPayload{Metadata: map[string]string{}}
	s7APClearMetadata(controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
				suffix: `
func s7APClearMetadata(m map[string]string) {
	clear(m)
	m["produced_"+"by"] = "generator"
}
`,
			},
			{
				name: "dynamic-merge",
				body: `
	type s7APMergePayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APMergePayload{Metadata: map[string]string{}}
	s7APMergeMetadata(controlled.Metadata, map[string]string{"safe": "value"})
	_, _ = json.Marshal(controlled)
`,
				suffix: `
func s7APMergeMetadata(dst, src map[string]string) {
	for key, value := range src {
		dst[key] = value
	}
}
`,
			},
			{
				name: "unresolved-external-mutator",
				body: `
	type s7APExternalPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APExternalPayload{Metadata: map[string]string{}}
	maps.Copy(controlled.Metadata, map[string]string{"safe": "value"})
	_, _ = json.Marshal(controlled)
`,
				importMaps: true,
			},
			{
				name: "unresolved-function-mutator-without-local-write",
				body: `
	type s7APUnknownPayload struct {
		Metadata map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APUnknownPayload{Metadata: map[string]string{"safe": "value"}}
	var unknownMutator func(map[string]string)
	unknownMutator(controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
			},
			{
				name: "nested-index-write",
				body: `
	controlled := map[string]map[string]string{
		"metadata": {"safe": "value"},
	}
	nested := controlled["metadata"]
	nested["produced_"+"by"] = "generator"
	_, _ = json.Marshal(controlled)
`,
			},
			{
				name: "nested-range-write",
				body: `
	controlled := map[string]map[string]string{
		"metadata": {"safe": "value"},
	}
	for _, nested := range controlled {
		nested["produced_"+"by"] = "generator"
	}
	_, _ = json.Marshal(controlled)
`,
			},
			{
				name: "nested-interface-assertion-write",
				body: `
	controlled := map[string]any{
		"metadata": map[string]string{"safe": "value"},
	}
	nested := controlled["metadata"].(map[string]string)
	nested["produced_"+"by"] = "generator"
	_, _ = json.Marshal(controlled)
`,
			},
			{
				name: "chained-selector-index-alias",
				body: `
	type s7APChainedPayload struct {
		Metadata map[string]map[string]string ` + "`json:\"metadata\"`" + `
	}
	controlled := s7APChainedPayload{
		Metadata: map[string]map[string]string{
			"metadata": {"safe": "value"},
		},
	}
	nested := controlled.Metadata["metadata"]
	alias := nested
	alias["produced_"+"by"] = "generator"
	_, _ = json.Marshal(controlled)
`,
			},
			{
				name: "unresolved-nested-index",
				body: `
	controlled := map[string]map[string]string{
		"metadata": {"safe": "value"},
	}
	nested := controlled[report.Command]
	_ = nested
	_, _ = json.Marshal(controlled)
`,
			},
		} {
			mutated := s6CloneSourceSet(sources)
			const producer = "func writePreparePublishReport(cmd *cobra.Command, report preparePublishReport) error {"
			rel := "internal/cli/prepare_publish.go"
			before := mutated[rel]
			mutated[rel] = strings.Replace(before, producer, producer+sensitivity.body, 1)
			if sensitivity.importMaps {
				mutated[rel] = strings.Replace(
					mutated[rel], "\t\"io/fs\"\n", "\t\"io/fs\"\n\t\"maps\"\n", 1,
				)
			}
			mutated[rel] += sensitivity.suffix
			if mutated[rel] == before {
				t.Fatalf("PIB-477 %s interprocedural mutation anchor missing", sensitivity.name)
			}
			if err := validateS7APProducedBySources(mutated); err == nil {
				t.Fatalf("PIB-477 same validator accepted %s", sensitivity.name)
			}
		}
		safeMutator := s6CloneSourceSet(sources)
		const safeProducer = "func writePreparePublishReport(cmd *cobra.Command, report preparePublishReport) error {"
		safeMutator["internal/cli/prepare_publish.go"] = strings.Replace(
			safeMutator["internal/cli/prepare_publish.go"],
			safeProducer,
			safeProducer+`
	type s7APSafePayload struct {
		Metadata map[string]string `+"`json:\"metadata\"`"+`
	}
	controlled := s7APSafePayload{Metadata: map[string]string{"safe": "value"}}
	s7APObserveMetadata(controlled.Metadata)
	_, _ = json.Marshal(controlled)
`,
			1,
		) + `
func s7APObserveMetadata(metadata map[string]string) {
	metadata["safe"] = "value"
}
`
		if err := validateS7APProducedBySources(safeMutator); err != nil {
			t.Fatalf("PIB-477 resolved safe mutator positive control failed: %v", err)
		}
		safeNested := s6CloneSourceSet(sources)
		safeNested["internal/cli/prepare_publish.go"] = strings.Replace(
			safeNested["internal/cli/prepare_publish.go"],
			safeProducer,
			safeProducer+`
	controlled := map[string]map[string]string{
		"metadata": {"safe": "value"},
	}
	nested := controlled["metadata"]
	nested["safe"] = "updated"
	for _, ranged := range controlled {
		ranged["also_safe"] = "value"
	}
	_, _ = json.Marshal(controlled)
`,
			1,
		)
		if err := validateS7APProducedBySources(safeNested); err != nil {
			t.Fatalf("PIB-477 nested map positive control failed: %v", err)
		}
		statusKey := s6CloneSourceSet(sources)
		statusKey["internal/cli/prepare_publish.go"] = strings.Replace(
			statusKey["internal/cli/prepare_publish.go"],
			`updates := []prepareStatusField{`,
			`updates := []prepareStatusField{
		{name: "produced_" + "by", raw: mustPrepareJSONRaw("generator")},`,
			1,
		)
		if err := validateS7APProducedBySources(statusKey); err == nil {
			t.Fatal("PIB-477 same validator accepted produced_by in status.json producer")
		}
		newProducer := s6CloneSourceSet(sources)
		newProducer["internal/store/intent_archive.go"] += `
func s7APUnexpectedIndexProducer(index IntentArchiveIndex) ([]byte, error) {
	return json.Marshal(index)
}
`
		if err := validateS7APProducedBySources(newProducer); err == nil {
			t.Fatal("PIB-477 same validator accepted an unclassified persisted-schema producer")
		}
		prose := s6CloneSourceSet(sources)
		prose["internal/cli/prepare_publish.go"] +=
			"\nconst s7APCanonicalProse = \"produced_by is not durable provenance\"\n"
		if err := validateS7APProducedBySources(prose); err != nil {
			t.Fatalf("PIB-477 canonical prose positive control failed: %v", err)
		}
	})
}

func TestS7APReferenceTruthGuard(t *testing.T) {
	t.Run("PIB-482", func(t *testing.T) {
		prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
		adr := s6RepoFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
		expected, err := s7APReferenceGraph(prd, adr)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7APReferenceTruth(prd, adr, expected); err != nil {
			t.Fatal(err)
		}
		extraction := strings.Replace(
			prd,
			"## Summary",
			"## Summary\n\nThe prepare lock is extracted from `internal/rescap` and reused as its authority.",
			1,
		)
		if err := validateS7APReferenceTruth(extraction, adr, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a rescap lock extraction claim")
		}
		origin := strings.Replace(
			prd,
			"## Summary",
			"## Summary\n\nThe prepare lock comes from `internal/rescap` and supplies its authority.",
			1,
		)
		if err := validateS7APReferenceTruth(origin, adr, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a rescap authority-origin claim")
		}
		mixed := strings.Replace(
			prd,
			"## Summary",
			"## Summary\n\nThe prepare authority comes from internal/rescap, unlike the rejected precedent.",
			1,
		)
		if err := validateS7APReferenceTruth(mixed, adr, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a positive origin hidden by rejected-precedent prose")
		}
		copied := strings.Replace(
			prd,
			"## Summary",
			"## Summary\n\nThe prepare lock is copied from internal/rescap.",
			1,
		)
		if err := validateS7APReferenceTruth(copied, adr, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a copied rescap lock claim")
		}
		d4Extraction := strings.Replace(
			adr,
			"the shipped kernel-lock precedent D4 adapts without extraction",
			"the shipped kernel-lock precedent D4 adapts by extraction",
			1,
		)
		if d4Extraction == adr {
			t.Fatal("PIB-482 D4 rescap-clause mutation anchor missing")
		}
		if err := validateS7APReferenceTruth(prd, d4Extraction, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted D4 rescap extraction")
		}
		extracted := strings.Replace(
			prd,
			"§7.4.4, §17.1",
			"the directory-authority sections",
			1,
		)
		if err := validateS7APReferenceTruth(extracted, adr, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted an extracted normative reference")
		}
		danglingSection := strings.Replace(adr, "## Context", "## Context\n\nSee §99.99.", 1)
		if err := validateS7APReferenceTruth(prd, danglingSection, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a dangling section reference")
		}
		danglingRow := strings.Replace(prd, "## Summary", "## Summary\n\nSee PIB-999.", 1)
		if err := validateS7APReferenceTruth(danglingRow, adr, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a dangling PIB reference")
		}
		danglingDecision := strings.Replace(adr, "## Context", "## Context\n\nSee D999.", 1)
		if err := validateS7APReferenceTruth(prd, danglingDecision, expected); err == nil {
			t.Fatal("PIB-482 same validator accepted a dangling decision reference")
		}
	})
}

func validateS7APProducedBySources(sources map[string]string) error {
	if len(sources) == 0 {
		return errors.New("produced_by source set is empty")
	}
	expectedSchemas := map[string][]string{
		"internal/cli/feature_intent_archive.go": {
			"intentArchiveDivergenceReport", "intentArchiveListCorruptObjectReport",
			"intentArchiveListEntryReport", "intentArchiveListGenerationReport",
			"intentArchiveListOrphanReport", "intentArchiveListReport",
			"intentArchivePendingHashReport", "intentArchivePendingPurgeReport",
			"intentArchivePurgeBlobReport", "intentArchivePurgeProgressReport",
			"intentArchivePurgeReferenceReport", "intentArchivePurgeReport",
			"intentArchiveRecoveryReport", "intentArchiveRefusalReport",
			"intentArchiveRemainingRepairsReport", "intentArchiveRepairNextReport",
			"intentArchiveRepairStageReport",
		},
		"internal/cli/prepare_publish.go": {
			"prepareAbandonedReport", "prepareActionReport", "prepareAdvisoryReport",
			"prepareArchiveReport", "prepareArtifactReport", "preparePublishReport",
			"preparePurgeProgressReport", "prepareRecoveryReport", "prepareRefusalReport",
		},
		"internal/intentpub/types.go": {
			"Entry", "Identity", "Journal",
		},
		"internal/store/intent_archive.go": {
			"IntentArchiveGeneration", "IntentArchiveIndex", "IntentArchiveReplacement",
			"intentArchiveGenerationBody", "intentArchiveImmutableReplacementBody",
		},
		"internal/store/status.go": {
			"DivergenceDetail", "EvidenceRef", "RejectionHistoryEntry", "RejectionStatus",
		},
		"internal/store/types.go": {
			"ApplySession", "ApplySummary", "Config", "Dependency", "FeatureStatus",
			"PatchIDMatch", "ProviderConfig", "ReconcileSummary", "UpstreamLock",
			"VerifyCheckResult", "VerifyRecord",
		},
		"internal/workflow/generate_analysis.go": {
			"AnalysisResult", "CompatibilityResult",
		},
	}
	expectedSchemaDigests := map[string]string{
		"internal/cli/feature_intent_archive.go": "f1faace7f3655136d7f7a7fda0324541e326efe210e56fc0de3190355f5223a8",
		"internal/cli/prepare_publish.go":        "e99aa48ce42fd1fda4c305af373035189196df70bc9df066ed4337a49fbf8841",
		"internal/intentpub/types.go":            "708ccc07128f862210408d7c8fd5f8ecf4c7254b927b94e53a5a25f5a9c5ab14",
		"internal/store/intent_archive.go":       "b03b5c4621e551ad99bbf6bded308b11f3807f52d3170f651342042d8a6c6ede",
		"internal/store/status.go":               "6854d2310513867187c124a08343604af931a10c52d15171afc175cbcd136d1b",
		"internal/store/types.go":                "00d6e584ee67c28aaa5364725f5b1e7e8f1c14d406da5570448eaa5c49cbf025",
		"internal/workflow/generate_analysis.go": "4ca21afd6ec0267af39a5e78b4f6d55fc7a38fc05ea94ea56ebe03b347e32d9b",
	}
	expectedProducers := map[string][]string{
		"internal/cli/feature_intent_archive.go": {
			"emitIntentArchiveListReport", "emitIntentArchivePurgeReport",
		},
		"internal/cli/prepare_publish.go": {
			"encodePrepareStatusFields", "generatePrepareBundle",
			"mustPrepareJSONRaw", "writePreparePublishReport",
		},
		"internal/intentpub/journal.go": {"EncodeJournal"},
		"internal/store/intent_archive.go": {
			"ComputeIntentArchiveGenerationID", "EncodeIntentArchiveIndex",
			"sealIntentArchiveAppendPlan",
		},
	}
	controlled := map[string]string{}
	for rel := range expectedSchemas {
		source, ok := sources[rel]
		if !ok {
			return fmt.Errorf("controlled schema source %s is missing", rel)
		}
		controlled[rel] = source
	}
	for rel := range expectedProducers {
		source, ok := sources[rel]
		if !ok {
			return fmt.Errorf("controlled producer source %s is missing", rel)
		}
		controlled[rel] = source
	}
	typedPackages, err := s7APTypeCheckReportPackages(controlled)
	if err != nil {
		return fmt.Errorf("type-check controlled schemas: %w", err)
	}
	var schemaDigestDrifts []string
	for rel, expected := range expectedSchemas {
		typed := typedPackages[s7APSourcePackage(rel)]
		if typed == nil || typed.relFiles[rel] == nil {
			return fmt.Errorf("typed controlled schema source %s is missing", rel)
		}
		actual := s7APJSONSchemaDeclarations(typed.relFiles[rel])
		sort.Strings(expected)
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%s schema inventory drift:\ngot  %#v\nwant %#v",
				rel, actual, expected)
		}
		digest := s7APJSONSchemaDigest(typed.relFiles[rel])
		if digest != expectedSchemaDigests[rel] {
			schemaDigestDrifts = append(schemaDigestDrifts,
				fmt.Sprintf("%s=%s", rel, digest))
		}
	}
	if len(schemaDigestDrifts) != 0 {
		return fmt.Errorf("controlled schema field inventory drift:\n%s",
			strings.Join(schemaDigestDrifts, "\n"))
	}
	for rel, expected := range expectedProducers {
		typed := typedPackages[s7APSourcePackage(rel)]
		if typed == nil || typed.relFiles[rel] == nil {
			return fmt.Errorf("typed controlled producer source %s is missing", rel)
		}
		actual := s7APJSONProducerFunctions(typed.relFiles[rel])
		sort.Strings(expected)
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%s JSON producer inventory drift:\ngot  %#v\nwant %#v",
				rel, actual, expected)
		}
		if err := s7APValidateControlledProducerMaps(
			typed.relFiles[rel], typed.info, expected,
		); err != nil {
			return fmt.Errorf("%s controlled map flow: %w", rel, err)
		}
	}
	prepareTyped := typedPackages["internal/cli"]
	if prepareTyped == nil {
		return errors.New("typed prepare package is missing")
	}
	if err := s7APValidateStatusSchemaProducer(
		prepareTyped.relFiles["internal/cli/prepare_publish.go"],
		prepareTyped.info,
	); err != nil {
		return err
	}
	for rel, source := range sources {
		var file *ast.File
		if _, ok := controlled[rel]; ok {
			file = typedPackages[s7APSourcePackage(rel)].relFiles[rel]
			if file == nil {
				return fmt.Errorf("typed controlled schema source %s is missing", rel)
			}
		} else {
			var parseErr error
			file, parseErr = parser.ParseFile(token.NewFileSet(), rel, source, 0)
			if parseErr != nil {
				return parseErr
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if err != nil {
				return false
			}
			switch value := node.(type) {
			case *ast.Field:
				if value.Tag == nil {
					return true
				}
				tag, unquoteErr := strconv.Unquote(value.Tag.Value)
				if unquoteErr != nil {
					err = unquoteErr
					return false
				}
				for _, component := range strings.Fields(tag) {
					key := strings.TrimSuffix(strings.TrimPrefix(component, `json:"`), `"`)
					key = strings.Split(key, ",")[0]
					if s7APGeneratorSchemaKey(key) &&
						!(rel == "internal/cli/prepare_publish.go" &&
							s7APParentTypeName(file, value) == "prepareArtifactReport" &&
							s7APFieldName(value) == "Generator" && key == "generator") {
						err = fmt.Errorf("%s accepts forbidden generator-class JSON key %q", rel, key)
						return false
					}
				}
			}
			return err == nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func s7APValidateStatusSchemaProducer(file *ast.File, info *types.Info) error {
	var updates, order []string
	for _, declaration := range file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		if function == nil || function.Name.Name != "prepareStatusBytes" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CompositeLit:
				named, _ := info.TypeOf(value).(*types.Named)
				if named == nil || named.Obj().Name() != "prepareStatusField" {
					return true
				}
				for _, element := range value.Elts {
					keyValue, _ := element.(*ast.KeyValueExpr)
					if keyValue == nil {
						continue
					}
					identifier, _ := keyValue.Key.(*ast.Ident)
					if identifier == nil || identifier.Name != "name" {
						continue
					}
					key, err := s7APControlledSchemaKey(info, keyValue.Value)
					if err != nil {
						updates = append(updates, "<unresolved>")
					} else {
						updates = append(updates, key)
					}
				}
			case *ast.AssignStmt:
				if len(value.Lhs) != 1 || len(value.Rhs) != 1 {
					return true
				}
				identifier, _ := value.Lhs[0].(*ast.Ident)
				composite, _ := value.Rhs[0].(*ast.CompositeLit)
				if identifier == nil || identifier.Name != "order" || composite == nil {
					return true
				}
				for _, element := range composite.Elts {
					expression, _ := element.(ast.Expr)
					key, err := s7APControlledSchemaKey(info, expression)
					if err != nil {
						order = append(order, "<unresolved>")
					} else {
						order = append(order, key)
					}
				}
			}
			return true
		})
	}
	wantUpdates := []string{"state", "updated_at", "last_command", "notes"}
	wantOrder := []string{
		"id", "slug", "title", "state", "compatibility", "requested_at",
		"updated_at", "last_command", "notes", "apply", "reconcile",
		"depends_on", "verify", "rejection", "rejection_history",
	}
	if !reflect.DeepEqual(updates, wantUpdates) || !reflect.DeepEqual(order, wantOrder) {
		return fmt.Errorf("status.json producer inventory drift: updates=%#v order=%#v",
			updates, order)
	}
	for _, key := range append(append([]string{}, updates...), order...) {
		if s7APGeneratorSchemaKey(key) {
			return fmt.Errorf("status.json producer accepts generator-class key %q", key)
		}
	}
	return nil
}

func s7APSourcePackage(rel string) string {
	index := strings.LastIndex(rel, "/")
	if index < 0 {
		return "."
	}
	return rel[:index]
}

func s7APJSONSchemaDeclarations(file *ast.File) []string {
	var result []string
	for _, declaration := range file.Decls {
		group, _ := declaration.(*ast.GenDecl)
		if group == nil || group.Tok != token.TYPE {
			continue
		}
		for _, specification := range group.Specs {
			typed, _ := specification.(*ast.TypeSpec)
			structure, _ := typed.Type.(*ast.StructType)
			if typed == nil || structure == nil {
				continue
			}
			hasJSON := false
			for _, field := range structure.Fields.List {
				if field.Tag != nil && strings.Contains(field.Tag.Value, `json:"`) {
					hasJSON = true
					break
				}
			}
			if hasJSON {
				result = append(result, typed.Name.Name)
			}
		}
	}
	sort.Strings(result)
	return result
}

func s7APJSONSchemaDigest(file *ast.File) string {
	var fields []string
	for _, declaration := range file.Decls {
		group, _ := declaration.(*ast.GenDecl)
		if group == nil || group.Tok != token.TYPE {
			continue
		}
		for _, specification := range group.Specs {
			typed, _ := specification.(*ast.TypeSpec)
			if typed == nil {
				continue
			}
			structure, _ := typed.Type.(*ast.StructType)
			if structure == nil {
				continue
			}
			for _, field := range structure.Fields.List {
				if field.Tag == nil {
					continue
				}
				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				jsonKey := ""
				for _, component := range strings.Fields(tag) {
					if strings.HasPrefix(component, `json:"`) {
						jsonKey = strings.Split(
							strings.TrimSuffix(strings.TrimPrefix(component, `json:"`), `"`),
							",",
						)[0]
						break
					}
				}
				if jsonKey == "" {
					continue
				}
				fieldName := s7APFieldName(field)
				if fieldName == "" {
					fieldName = "<embedded>"
				}
				fields = append(fields, typed.Name.Name+"."+fieldName+"="+jsonKey)
			}
		}
	}
	sort.Strings(fields)
	sum := sha256.Sum256([]byte(strings.Join(fields, "\n")))
	return fmt.Sprintf("%x", sum[:])
}

func s7APJSONProducerFunctions(file *ast.File) []string {
	var result []string
	for _, declaration := range file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		if function == nil || function.Body == nil {
			continue
		}
		produces := false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, _ := node.(*ast.CallExpr)
			if call == nil {
				return true
			}
			selector, _ := call.Fun.(*ast.SelectorExpr)
			if selector == nil {
				return true
			}
			pkg, _ := selector.X.(*ast.Ident)
			if pkg != nil && pkg.Name == "json" {
				switch selector.Sel.Name {
				case "Marshal", "MarshalIndent", "NewEncoder":
					produces = true
					return false
				}
			}
			return !produces
		})
		if produces {
			result = append(result, function.Name.Name)
		}
	}
	sort.Strings(result)
	return result
}

func s7APGeneratorSchemaKey(key string) bool {
	switch key {
	case "created_by", "generated_by", "generator", "generator_kind",
		"generator_name", "generator_type", "produced_by":
		return true
	default:
		return false
	}
}

func s7APFieldName(field *ast.Field) string {
	if len(field.Names) != 1 {
		return ""
	}
	return field.Names[0].Name
}

func s7APParentTypeName(root ast.Node, target ast.Node) string {
	var result string
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if node == target {
			for index := len(stack) - 1; index >= 0; index-- {
				specification, _ := stack[index].(*ast.TypeSpec)
				if specification != nil {
					result = specification.Name.Name
					return false
				}
			}
		}
		stack = append(stack, node)
		return result == ""
	})
	return result
}

type s7APControlledMapFlow struct {
	info        *types.Info
	assignments map[types.Object][]ast.Expr
	arguments   map[types.Object][]ast.Expr
	writes      map[types.Object][]ast.Expr
	indexes     map[types.Object][]ast.Expr
	aliases     map[types.Object][]types.Object
	returns     map[*types.Func][]ast.Expr
	local       map[*types.Func]bool
	unresolved  map[types.Object][]string
	visiting    map[types.Object]bool
	calling     map[*types.Func]bool
}

func s7APValidateControlledProducerMaps(
	file *ast.File,
	info *types.Info,
	producers []string,
) error {
	expected := map[string]bool{}
	for _, name := range producers {
		expected[name] = true
	}
	assignments, arguments, writes, indexes, aliases, returns, local, unresolved :=
		s7APControlledMapDataflow(file, info)
	for _, declaration := range file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		if function == nil || function.Body == nil || !expected[function.Name.Name] {
			continue
		}
		flow := &s7APControlledMapFlow{
			info:        info,
			assignments: assignments,
			arguments:   arguments,
			writes:      writes,
			indexes:     indexes,
			aliases:     aliases,
			returns:     returns,
			local:       local,
			unresolved:  unresolved,
			visiting:    map[types.Object]bool{},
			calling:     map[*types.Func]bool{},
		}
		var validationErr error
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if validationErr != nil {
				return false
			}
			call, _ := node.(*ast.CallExpr)
			if call == nil {
				return true
			}
			payload := s7APControlledJSONPayload(info, call)
			if payload == nil ||
				(!s7APTypeContainsStringMap(info.TypeOf(payload), nil) &&
					!s7APInterfaceType(info.TypeOf(payload))) {
				return true
			}
			if err := flow.validate(payload); err != nil {
				validationErr = fmt.Errorf("%s: %w", function.Name.Name, err)
				return false
			}
			return true
		})
		if validationErr != nil {
			return validationErr
		}
	}
	return nil
}

func (flow *s7APControlledMapFlow) validate(expression ast.Expr) error {
	if expression == nil {
		return nil
	}
	if !s7APTypeContainsStringMap(flow.info.TypeOf(expression), nil) &&
		!s7APInterfaceType(flow.info.TypeOf(expression)) {
		return nil
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return flow.validate(value.X)
	case *ast.UnaryExpr:
		return flow.validate(value.X)
	case *ast.TypeAssertExpr:
		return flow.validate(value.X)
	case *ast.CallExpr:
		typed := flow.info.Types[value.Fun]
		if typed.IsType() && len(value.Args) == 1 {
			return flow.validate(value.Args[0])
		}
		identifier, _ := value.Fun.(*ast.Ident)
		if identifier != nil && identifier.Name == "make" {
			return nil
		}
		function := s7APCalledFunction(flow.info, value)
		if function != nil && flow.local[function] && !flow.calling[function] {
			returns := flow.returns[function]
			if len(returns) == 0 {
				return fmt.Errorf("map-bearing helper %s has no proven return", function.Name())
			}
			flow.calling[function] = true
			defer delete(flow.calling, function)
			for _, result := range returns {
				if err := flow.validate(result); err != nil {
					return err
				}
			}
			return nil
		}
		return fmt.Errorf("map-bearing encoder input uses unresolved call %T", value.Fun)
	case *ast.CompositeLit:
		if s7APControlledStringMap(flow.info.TypeOf(value)) {
			for _, element := range value.Elts {
				keyed, _ := element.(*ast.KeyValueExpr)
				if keyed == nil {
					return errors.New("controlled map uses an unkeyed element")
				}
				if err := s7APValidateControlledMapKey(flow.info, keyed.Key); err != nil {
					return err
				}
				if err := flow.validate(keyed.Value); err != nil {
					return err
				}
			}
			return nil
		}
		for _, element := range value.Elts {
			switch candidate := element.(type) {
			case *ast.KeyValueExpr:
				if err := flow.validate(candidate.Value); err != nil {
					return err
				}
			case ast.Expr:
				if err := flow.validate(candidate); err != nil {
					return err
				}
			}
		}
		return nil
	case *ast.Ident:
		if value.Name == "nil" {
			return nil
		}
		object := flow.info.ObjectOf(value)
		if object == nil {
			return fmt.Errorf("controlled map source %q is unresolved or recursive", value.Name)
		}
		if flow.visiting[object] {
			return nil
		}
		flow.visiting[object] = true
		defer delete(flow.visiting, object)
		if err := flow.validateObjectKeys(object, nil); err != nil {
			return err
		}
		assignments := append(
			append([]ast.Expr{}, flow.assignments[object]...),
			flow.arguments[object]...,
		)
		mapValue := s7APControlledStringMap(flow.info.TypeOf(value)) ||
			s7APInterfaceType(flow.info.TypeOf(value))
		if len(assignments) == 0 && mapValue {
			return fmt.Errorf("controlled map source %q has no proven assignment", value.Name)
		}
		for _, assignment := range assignments {
			if err := flow.validate(assignment); err != nil {
				return err
			}
		}
		return flow.validateMapFields(flow.info.TypeOf(value), nil)
	case *ast.SelectorExpr:
		object := flow.info.ObjectOf(value.Sel)
		if object == nil {
			return fmt.Errorf("controlled map field %q is unresolved or recursive", value.Sel.Name)
		}
		if flow.visiting[object] {
			return nil
		}
		flow.visiting[object] = true
		defer delete(flow.visiting, object)
		if err := flow.validateObjectKeys(object, nil); err != nil {
			return err
		}
		assignments := flow.assignments[object]
		if len(assignments) == 0 {
			return fmt.Errorf("controlled map field %q has no proven assignment", value.Sel.Name)
		}
		for _, assignment := range assignments {
			if err := flow.validate(assignment); err != nil {
				return err
			}
		}
		return flow.validateMapFields(flow.info.TypeOf(value), nil)
	case *ast.IndexExpr:
		if s7APControlledStringMap(flow.info.TypeOf(value.X)) {
			if err := s7APValidateControlledMapKey(flow.info, value.Index); err != nil {
				return err
			}
		}
		return flow.validate(value.X)
	default:
		return fmt.Errorf("controlled map source %T is unresolved", expression)
	}
}

func (flow *s7APControlledMapFlow) validateMapFields(
	value types.Type,
	visiting map[types.Type]bool,
) error {
	if value == nil {
		return nil
	}
	for {
		pointer, _ := value.(*types.Pointer)
		if pointer == nil {
			break
		}
		value = pointer.Elem()
	}
	if visiting == nil {
		visiting = map[types.Type]bool{}
	}
	if visiting[value] {
		return nil
	}
	visiting[value] = true
	defer delete(visiting, value)
	structure, _ := value.Underlying().(*types.Struct)
	if structure == nil {
		return nil
	}
	for index := 0; index < structure.NumFields(); index++ {
		field := structure.Field(index)
		fieldType := field.Type()
		if s7APControlledStringMap(fieldType) || s7APInterfaceType(fieldType) {
			if flow.visiting[field] {
				return fmt.Errorf("controlled map field %q is recursive", field.Name())
			}
			flow.visiting[field] = true
			if err := flow.validateObjectKeys(field, nil); err != nil {
				delete(flow.visiting, field)
				return err
			}
			assignments := flow.assignments[field]
			if len(assignments) == 0 {
				delete(flow.visiting, field)
				return fmt.Errorf("controlled map field %q has no proven assignment", field.Name())
			}
			for _, assignment := range assignments {
				if err := flow.validate(assignment); err != nil {
					delete(flow.visiting, field)
					return err
				}
			}
			delete(flow.visiting, field)
			continue
		}
		if s7APTypeContainsStringMap(fieldType, nil) {
			if err := flow.validateMapFields(fieldType, visiting); err != nil {
				return err
			}
		}
	}
	return nil
}

func (flow *s7APControlledMapFlow) validateObjectKeys(
	object types.Object,
	visiting map[types.Object]bool,
) error {
	if object == nil {
		return errors.New("controlled map object is unresolved")
	}
	if visiting == nil {
		visiting = map[types.Object]bool{}
	}
	if visiting[object] {
		return errors.New("controlled map alias cycle is unresolved")
	}
	visiting[object] = true
	defer delete(visiting, object)
	if calls := flow.unresolved[object]; len(calls) != 0 {
		return fmt.Errorf("controlled map reaches unresolved external mutator %s",
			strings.Join(calls, ", "))
	}
	for _, key := range flow.indexes[object] {
		if err := s7APValidateControlledMapKey(flow.info, key); err != nil {
			return err
		}
	}
	for _, key := range flow.writes[object] {
		if err := s7APValidateControlledMapKey(flow.info, key); err != nil {
			return err
		}
	}
	for _, alias := range flow.aliases[object] {
		if err := flow.validateObjectKeys(alias, visiting); err != nil {
			return err
		}
	}
	return nil
}

func s7APControlledMapDataflow(
	file *ast.File,
	info *types.Info,
) (
	map[types.Object][]ast.Expr,
	map[types.Object][]ast.Expr,
	map[types.Object][]ast.Expr,
	map[types.Object][]ast.Expr,
	map[types.Object][]types.Object,
	map[*types.Func][]ast.Expr,
	map[*types.Func]bool,
	map[types.Object][]string,
) {
	assignments := map[types.Object][]ast.Expr{}
	arguments := map[types.Object][]ast.Expr{}
	writes := map[types.Object][]ast.Expr{}
	indexes := map[types.Object][]ast.Expr{}
	aliases := map[types.Object][]types.Object{}
	returns := map[*types.Func][]ast.Expr{}
	local := map[*types.Func]bool{}
	unresolved := map[types.Object][]string{}
	for _, declaration := range file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		if function == nil || function.Body == nil {
			continue
		}
		object, _ := info.Defs[function.Name].(*types.Func)
		if object == nil {
			continue
		}
		local[object] = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			statement, _ := node.(*ast.ReturnStmt)
			if statement != nil {
				for _, result := range statement.Results {
					if s7APTypeContainsStringMap(info.TypeOf(result), nil) ||
						s7APInterfaceType(info.TypeOf(result)) {
						returns[object] = append(returns[object], result)
					}
				}
			}
			return true
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.ValueSpec:
			for index, name := range value.Names {
				if index < len(value.Values) {
					if object := info.Defs[name]; object != nil {
						assignments[object] = append(assignments[object], value.Values[index])
					}
				}
			}
		case *ast.AssignStmt:
			for index, left := range value.Lhs {
				if identifier, _ := left.(*ast.Ident); identifier != nil &&
					index < len(value.Rhs) {
					object := info.ObjectOf(identifier)
					if object != nil {
						assignments[object] = append(assignments[object], value.Rhs[index])
						if source := s7APControlledMapObject(info, value.Rhs[index]); source != nil &&
							(s7APTypeContainsStringMap(object.Type(), nil) ||
								s7APInterfaceType(object.Type())) {
							s7APRecordControlledMapIndexes(
								info, value.Rhs[index], source, indexes,
							)
							aliases[source] = append(aliases[source], object)
						}
					}
				}
				if selector, _ := left.(*ast.SelectorExpr); selector != nil &&
					index < len(value.Rhs) {
					object := info.ObjectOf(selector.Sel)
					if object != nil {
						assignments[object] = append(assignments[object], value.Rhs[index])
						if source := s7APControlledMapObject(info, value.Rhs[index]); source != nil &&
							(s7APTypeContainsStringMap(object.Type(), nil) ||
								s7APInterfaceType(object.Type())) {
							s7APRecordControlledMapIndexes(
								info, value.Rhs[index], source, indexes,
							)
							aliases[source] = append(aliases[source], object)
						}
					}
				}
				indexed, _ := left.(*ast.IndexExpr)
				if indexed != nil && s7APControlledStringMap(info.TypeOf(indexed.X)) {
					if object := s7APControlledMapObject(info, indexed.X); object != nil {
						writes[object] = append(writes[object], indexed.Index)
					}
				}
			}
		case *ast.RangeStmt:
			source := s7APControlledMapObject(info, value.X)
			if source == nil {
				return true
			}
			s7APRecordControlledMapIndexes(info, value.X, source, indexes)
			for _, expression := range []ast.Expr{value.Key, value.Value} {
				identifier, _ := expression.(*ast.Ident)
				if identifier == nil || identifier.Name == "_" {
					continue
				}
				object := info.ObjectOf(identifier)
				if object != nil &&
					(s7APTypeContainsStringMap(object.Type(), nil) ||
						s7APInterfaceType(object.Type())) {
					aliases[source] = append(aliases[source], object)
				}
			}
		case *ast.CompositeLit:
			compositeType := info.TypeOf(value)
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
			for index, element := range value.Elts {
				if keyed, _ := element.(*ast.KeyValueExpr); keyed != nil {
					key, _ := keyed.Key.(*ast.Ident)
					field, _ := info.ObjectOf(key).(*types.Var)
					if field != nil && field.IsField() {
						assignments[field] = append(assignments[field], keyed.Value)
					}
					continue
				}
				if index < structure.NumFields() {
					assignments[structure.Field(index)] = append(
						assignments[structure.Field(index)],
						element,
					)
				}
			}
		case *ast.CallExpr:
			function := s7APCalledFunction(info, value)
			if function == nil {
				for _, argument := range value.Args {
					if !s7APTypeContainsStringMap(info.TypeOf(argument), nil) &&
						!s7APInterfaceType(info.TypeOf(argument)) {
						continue
					}
					source := s7APControlledMapObject(info, argument)
					if source != nil && !s7APKnownReadOnlyBuiltinMapCall(info, value) {
						unresolved[source] = append(
							unresolved[source],
							"unresolved map-bearing call",
						)
					}
				}
				return true
			}
			signature, _ := function.Type().(*types.Signature)
			if signature == nil {
				return true
			}
			for index := 0; index < signature.Params().Len() && index < len(value.Args); index++ {
				parameter := signature.Params().At(index)
				arguments[parameter] = append(arguments[parameter], value.Args[index])
				argument := value.Args[index]
				if !s7APTypeContainsStringMap(info.TypeOf(argument), nil) &&
					!s7APInterfaceType(info.TypeOf(argument)) {
					continue
				}
				source := s7APControlledMapObject(info, argument)
				if source == nil {
					continue
				}
				s7APRecordControlledMapIndexes(info, argument, source, indexes)
				if local[function] {
					aliases[source] = append(aliases[source], parameter)
				} else if !s7APKnownReadOnlyMapCall(function) {
					unresolved[source] = append(
						unresolved[source],
						function.Pkg().Path()+"."+function.Name(),
					)
				}
			}
		}
		return true
	})
	return assignments, arguments, writes, indexes, aliases, returns, local, unresolved
}

func s7APKnownReadOnlyBuiltinMapCall(info *types.Info, call *ast.CallExpr) bool {
	identifier, _ := call.Fun.(*ast.Ident)
	builtin, _ := info.Uses[identifier].(*types.Builtin)
	if builtin == nil {
		return false
	}
	switch builtin.Name() {
	case "cap", "len":
		return true
	default:
		return false
	}
}

func s7APKnownReadOnlyMapCall(function *types.Func) bool {
	if function == nil || function.Pkg() == nil {
		return false
	}
	key := function.Pkg().Path() + "." + function.Name()
	switch key {
	case "encoding/json.Marshal", "encoding/json.MarshalIndent", "maps.Clone":
		return true
	default:
		return false
	}
}

func s7APValidateControlledMapKey(info *types.Info, expression ast.Expr) error {
	key, err := s7APControlledSchemaKey(info, expression)
	if err != nil {
		return fmt.Errorf("unresolved controlled schema key: %w", err)
	}
	if s7APGeneratorSchemaKey(key) {
		return fmt.Errorf("generator-class controlled schema key %q", key)
	}
	return nil
}

func s7APControlledJSONPayload(info *types.Info, call *ast.CallExpr) ast.Expr {
	function := s7APCalledFunction(info, call)
	if function == nil || function.Pkg() == nil {
		return nil
	}
	if function.Pkg().Path() == "encoding/json" {
		switch function.Name() {
		case "Marshal", "MarshalIndent":
			if len(call.Args) != 0 {
				return call.Args[0]
			}
		case "Encode":
			signature, _ := function.Type().(*types.Signature)
			if signature != nil && s7APNamedReceiver(signature.Recv(), "encoding/json", "Encoder") &&
				len(call.Args) != 0 {
				return call.Args[0]
			}
		}
	}
	return nil
}

func s7APCalledFunction(info *types.Info, call *ast.CallExpr) *types.Func {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		result, _ := info.Uses[function].(*types.Func)
		return result
	case *ast.SelectorExpr:
		result, _ := info.Uses[function.Sel].(*types.Func)
		return result
	default:
		return nil
	}
}

func s7APNamedReceiver(
	receiver *types.Var,
	packagePath, name string,
) bool {
	if receiver == nil {
		return false
	}
	value := receiver.Type()
	if pointer, _ := value.(*types.Pointer); pointer != nil {
		value = pointer.Elem()
	}
	named, _ := value.(*types.Named)
	return named != nil && named.Obj() != nil && named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == name
}

func s7APControlledMapObject(info *types.Info, expression ast.Expr) types.Object {
	for {
		switch value := expression.(type) {
		case *ast.Ident:
			return info.ObjectOf(value)
		case *ast.ParenExpr:
			expression = value.X
		case *ast.SelectorExpr:
			return info.ObjectOf(value.Sel)
		case *ast.IndexExpr:
			expression = value.X
		case *ast.TypeAssertExpr:
			expression = value.X
		case *ast.UnaryExpr:
			expression = value.X
		case *ast.StarExpr:
			expression = value.X
		default:
			return nil
		}
	}
}

func s7APRecordControlledMapIndexes(
	info *types.Info,
	expression ast.Expr,
	root types.Object,
	indexes map[types.Object][]ast.Expr,
) {
	if root == nil {
		return
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		s7APRecordControlledMapIndexes(info, value.X, root, indexes)
	case *ast.UnaryExpr:
		s7APRecordControlledMapIndexes(info, value.X, root, indexes)
	case *ast.StarExpr:
		s7APRecordControlledMapIndexes(info, value.X, root, indexes)
	case *ast.TypeAssertExpr:
		s7APRecordControlledMapIndexes(info, value.X, root, indexes)
	case *ast.IndexExpr:
		s7APRecordControlledMapIndexes(info, value.X, root, indexes)
		if s7APControlledStringMap(info.TypeOf(value.X)) {
			indexes[root] = append(indexes[root], value.Index)
		}
	}
}

func s7APTypeContainsStringMap(value types.Type, visiting map[types.Type]bool) bool {
	if value == nil || visiting[value] {
		return false
	}
	if s7APControlledStringMap(value) {
		return true
	}
	if visiting == nil {
		visiting = map[types.Type]bool{}
	}
	visiting[value] = true
	defer delete(visiting, value)
	switch typed := value.Underlying().(type) {
	case *types.Pointer:
		return s7APTypeContainsStringMap(typed.Elem(), visiting)
	case *types.Array:
		return s7APTypeContainsStringMap(typed.Elem(), visiting)
	case *types.Slice:
		return s7APTypeContainsStringMap(typed.Elem(), visiting)
	case *types.Struct:
		for index := 0; index < typed.NumFields(); index++ {
			if s7APTypeContainsStringMap(typed.Field(index).Type(), visiting) {
				return true
			}
		}
	}
	return false
}

func s7APInterfaceType(value types.Type) bool {
	if value == nil {
		return false
	}
	_, ok := value.Underlying().(*types.Interface)
	return ok
}

func s7APControlledStringMap(value types.Type) bool {
	named, _ := value.(*types.Named)
	if named != nil {
		value = named.Underlying()
	}
	mapping, _ := value.(*types.Map)
	if mapping == nil {
		return false
	}
	key, _ := mapping.Key().Underlying().(*types.Basic)
	if key == nil || key.Kind() != types.String {
		return false
	}
	return true
}

func s7APControlledSchemaKey(info *types.Info, expression ast.Expr) (string, error) {
	value := info.Types[expression].Value
	if value == nil || value.Kind() != constant.String {
		return "", fmt.Errorf("key %T is not a compile-time string constant", expression)
	}
	return constant.StringVal(value), nil
}

func s7APParentComposite(root ast.Node, target ast.Node) *ast.CompositeLit {
	var result *ast.CompositeLit
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if node == target {
			for index := len(stack) - 1; index >= 0; index-- {
				if composite, ok := stack[index].(*ast.CompositeLit); ok {
					result = composite
					return false
				}
			}
		}
		stack = append(stack, node)
		return result == nil
	})
	return result
}

type s7APReferenceSet struct {
	graph     map[string]int
	sections  map[string]bool
	rows      map[string]bool
	decisions map[string]bool
}

func s7APReferenceGraph(prd, adr string) (s7APReferenceSet, error) {
	result := s7APReferenceSet{
		graph: map[string]int{}, sections: map[string]bool{},
		rows: map[string]bool{}, decisions: map[string]bool{},
	}
	for _, match := range regexp.MustCompile(`(?m)^#{2,6}\s+([0-9]+(?:\.[0-9]+)*)\b`).FindAllStringSubmatch(prd, -1) {
		result.sections[match[1]] = true
	}
	for _, match := range regexp.MustCompile(`(?m)^\| (PIB-[0-9]{3}) \|`).FindAllStringSubmatch(prd, -1) {
		result.rows[match[1]] = true
	}
	for _, match := range regexp.MustCompile(`(?m)^### (D[0-9]+)\s+—`).FindAllStringSubmatch(adr, -1) {
		result.decisions[match[1]] = true
	}
	if len(result.sections) == 0 || len(result.rows) != 567 || len(result.decisions) == 0 {
		return result, fmt.Errorf("reference targets = sections:%d rows:%d decisions:%d",
			len(result.sections), len(result.rows), len(result.decisions))
	}
	documents := []struct {
		name         string
		body         string
		historyStart string
		historyEnd   string
	}{
		{
			name: "PRD", body: prd,
			historyStart: "\n## Revision history\n",
			historyEnd:   "\n## Summary\n",
		},
		{
			name: "ADR", body: adr,
			historyStart: "\n**Revision history**\n",
			historyEnd:   "\n## Context\n",
		},
	}
	for _, document := range documents {
		start := strings.Index(document.body, document.historyStart)
		end := strings.Index(document.body, document.historyEnd)
		if start < 0 || end < 0 || end <= start ||
			strings.Count(document.body, document.historyStart) != 1 ||
			strings.Count(document.body, document.historyEnd) != 1 {
			return result, fmt.Errorf("%s historical exclusion is not uniquely bounded", document.name)
		}
		body := document.body[:start] + document.body[end:]
		if err := validateS7APNoRescapAuthorityDerivationClaim(document.name, body); err != nil {
			return result, err
		}
		for _, sections := range regexp.MustCompile(`§{1,2}([0-9]+(?:\.[0-9]+)*(?:/[0-9]+(?:\.[0-9]+)*)*)`).FindAllStringSubmatch(body, -1) {
			for _, section := range strings.Split(sections[1], "/") {
				result.graph[document.name+"|section|"+section]++
				if !result.sections[section] {
					return result, fmt.Errorf("%s has dangling section reference §%s", document.name, section)
				}
			}
		}
		rangePattern := regexp.MustCompile(`PIB-([0-9]{3})\s*(?:…|–|—)\s*PIB-([0-9]{3})`)
		rangeSpans := rangePattern.FindAllStringSubmatchIndex(body, -1)
		masked := []byte(body)
		for _, span := range rangeSpans {
			first, _ := strconv.Atoi(body[span[2]:span[3]])
			last, _ := strconv.Atoi(body[span[4]:span[5]])
			if first > last {
				return result, fmt.Errorf("%s has descending PIB range %d…%d", document.name, first, last)
			}
			for number := first; number <= last; number++ {
				id := fmt.Sprintf("PIB-%03d", number)
				result.graph[document.name+"|row|"+id]++
				if !result.rows[id] {
					return result, fmt.Errorf("%s has dangling row reference %s", document.name, id)
				}
			}
			for index := span[0]; index < span[1]; index++ {
				masked[index] = ' '
			}
		}
		for _, match := range regexp.MustCompile(`\bPIB-[0-9]{3}\b`).FindAllString(string(masked), -1) {
			result.graph[document.name+"|row|"+match]++
			if !result.rows[match] {
				return result, fmt.Errorf("%s has dangling row reference %s", document.name, match)
			}
		}
		for _, decision := range regexp.MustCompile(`\bD[0-9]+\b`).FindAllString(body, -1) {
			result.graph[document.name+"|decision|"+decision]++
			if !result.decisions[decision] {
				return result, fmt.Errorf("%s has dangling ADR decision %s", document.name, decision)
			}
		}
	}
	return result, nil
}

var s7APAcceptedRescapAuthorityClauses = map[string]map[string]bool{
	"PRD": {
		"039719ecee296f9d3c2f6931c8d5205cfbf32e2cca4d3c1e295dbb5a69c4d71e": true,
		"079a918d01c2a9ff6dda033ae93d38cb727968b64959fd1965d6ba67e8f74ea9": true,
		"08072ec4084f565527445c7f16e41cad7a23a22415af52581a99dae64bd1910b": true,
		"11f3ebab9cd979318f326a6cf72f657f0d4a96352b1badeb9f8f4c2d718bd9a5": true,
		"179157fa6b13155740eaef5c03abc4e8dd5ffdce08febb2d4721b257e322a713": true,
		"2a2da8d58f0b4def0b8b7b925e3dea75c440aa1696edf2c970b22f6863d71933": true,
		"3296a767dc87aed208ca2d39613560d5358eef85f7029bf0a583b2a9b3e1d8e4": true,
		"33ab174a577ae441a56e97521dcd18636e8b56c6e1bb95fc8ae294fc289f994c": true,
		"371584c968282d86d3e144077d1f086b450fe1a6c672fc20cd16bc9e8ae3b11f": true,
		"3b2a8e3c6c2349bb4255b6436690a92939ab560bada712c9c676c4ee888f7ab2": true,
		"3f07616d1ca4710596ef0de3036ec0cd58b36884374181010fa9bdfd13ed8d7a": true,
		"3f365424222a180866fa05abca199518a41754901f3d5ed0f4ea4d8e6bae1d44": true,
		"4365acf3253f22d0aeb10684277a9fc1aec80aedbcc08065b08900166f2bea7e": true,
		"4f8f3787e0d91c90b659f896304926165f24f6b813ab6c0f08ffaec9f133ce2d": true,
		"5bbe82113b728aa6e1c05fd866e3e177084227b4c160fb7acdd4d9dda1800937": true,
		"5ed49954b064f9700b920042f4a42483db8151f0fbc57ca3cfbd013c42f8db10": true,
		"6286db592a8bc76b3a458d65987d27ce571bf2caff659151a6fa7fd38f92b942": true,
		"63d6b1d7f9a89f067197f63702ddebe4471f1d179a4bdab80445c876f5683a26": true,
		"6e21b3042d0d22ad22929904aa5f0e6aa0f32ae5ab91fba33e70912a81d8d80e": true,
		"6e417be788f3d814538c4a0f009459540eb51afd86db7cb530bc40d637a8daf8": true,
		"71210689eef96bda3b0b3483afc309d0a37a42fd59286257bff075bc531f3353": true,
		"72795c22df9983db2082322235121a8e5b3edd2a7253aaacc8a3a36fffccf73c": true,
		"79d68b12b5026f79805a0ac3466cb9138439957a52ebb45bf6d4c8a58e56039a": true,
		"7d757d82b145dcfe3b777c85419d5fcd9e59f0b64ae305dda27d8766c358103d": true,
		"7e4f295156ef552b4ef134a1da964dcf0c900f956661d3277597802ca2b27344": true,
		"81c435fe0cc0f456453d5009c96518c8658bdb5b34198ca362ebcea6ee679714": true,
		"85e6bc80bbfd321536578b7f762e0dd3394e322473860fd851b9aed7fbb75c94": true,
		"866ad57a564efe22e0db24d180e923070f916036a8ce11799ea29c3dc9c2b811": true,
		"8719b01a446795e46701f679f3017935f1e51f702a22b0ac98433363fe20e872": true,
		"8be1896605ac0a3c4d674e93ca4472bf0925303f66aa855f2309fe3a363cd3ef": true,
		"8ee1fd506cb8a7433490c5e4b64935c51aa4fe61a55dba48c38977581b343d8f": true,
		"9cc4443cf1a09b1f32d87604f0e7b202eec1de5e8aa2e6bf6f80ea455421bb7d": true,
		"a074eace0494bb9fd44da9511c07235b1a1dfa88f2e6b8f531861a28ddd040dd": true,
		"a18b4310f09abb1c684fcced01634d756a0933f0a13cb1466dfc0ed4d8643d00": true,
		"a41cb314be36089e40075e5d9fa0692309493856aa2f93d946231c77a49e1a1a": true,
		"a421b83c0d0023f98a8522491213a8c870290a4c054289909daff3311c61851c": true,
		"a83c42a726d7b9c8ffb9351e22f0edc8385b0f1b460315d45193818be5dbf3a1": true,
		"aa379f3ba1da054dfc9376d5163652da3f377e52d10babcf2fccfc68ec8cc82c": true,
		"b0afee6e98653977803ffb86733c63c2c32aa89c178141440aa0dd051237462b": true,
		"ba99a5eef088ac14060f528e51f26396e22b718263df0c1d52586d763c9eb469": true,
		"c5a972f70c2cb697794cfd3bdbbaed741981bafadace6508ca5f6ef1cdb707dd": true,
		"cdba55ec3ac232af54d96475022eecddacb5b6c6ecb35589fd782de602a1a254": true,
		"ce9cb48184e0bc1172be0bd475eae21aeb05e027b468297b71703fb12bf0129e": true,
		"d18ab393bc86ff5e61a6e7954ce6ddf5648cb750d2b778272e573056e347bf47": true,
		"d196a443cfa3fe46ad3015809f76a9d7ba55a1d44867025c017ddcf3e3dc4786": true,
		"d339dc9c00d3eeae96a8b554dbd44d511d6577f6b03a5cb75138663776ac3df4": true,
		"d6734a7e4755e742cd3ed8130a52384afc51f3f1f4d2d953d0d9203dd8cd9618": true,
		"dd3def3fa8267ac410690b6ca171a1792965232bd834ec219fad90f05a1ad572": true,
		"deea3e99dd831a4e08b9dc166f664a0d672dccb5c1e50518f07f080213d85629": true,
		"e0015d6fbef746f94d758d634dee7c12365028101739d32e0cf66fa41664b410": true,
		"e1d0b9ade553e84121b990cca8bd03b8328ee696ec93114124786211b3fe633c": true,
		"e6c92f16a38c3009488fcb68f3bd5231f34f5d203d8a074b3ed344a0f395b30f": true,
		"f35d16d4444903c378c32a0fb6552dfba566275e32fbae34862c894b75dc9a3c": true,
		"f5395d7a72a2558a4d7a813c567a333295f5c0e8d9fd119997f28bba8a980ee8": true,
		"f8eba906892b83c9d3cdd8c038433381d5bef1135fd7258b1fcb6eb5e8c06e62": true,
		"f9b2e64f7a0560ea5968af961c5d5fd48066c228e8944569b7e2b03edf96fc85": true,
	},
	"ADR": {
		"037c7222b66ddb529caebacd897fe9c518377efd3b10c63e1ee7900aa0eabff4": true,
		"153ce9fad1a4a262575f870f43a60d0a2541a7d4da77e4935d713c6178c96042": true,
		"20745f050fa4718d679e0d42ab67397296eadccdbde0377f385caabe8ab52f6b": true,
		"2bad7259132ded126452930989d49d7bb54d7768dc342086ea0ee2cd1127600a": true,
		"52c33661b8cc75d729cd1de04f7651f88bc8ca8a737f0e27dd5a9dfe1fc3b8ae": true,
		"70fed5b0497ba6a368bf33ff8531d2f58590afa3424e20e15d0bd637ed63875d": true,
		"7d59ba03ff916cfd38cebc4a646f92124bdeadd4daff1e4b3cd6245a64e87c12": true,
		"a18afde4885e93c731a7a765db8261dce832686666c4d2c4bc89c3b0f4b30b1c": true,
		"d4d39bbc07964cd98ecf2ac5b26cd8c63451250fa0dfda52f472faf492581979": true,
	},
}

func validateS7APNoRescapAuthorityDerivationClaim(name, body string) error {
	actual := map[string]bool{}
	for _, clause := range s7APMarkdownClauses(body) {
		if !strings.Contains(clause, "rescap") {
			continue
		}
		sum := sha256.Sum256([]byte(clause))
		actual[fmt.Sprintf("%x", sum[:])] = true
	}
	want, ok := s7APAcceptedRescapAuthorityClauses[name]
	if !ok {
		return fmt.Errorf("%s has no accepted rescap/prepare clause inventory", name)
	}
	if !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("%s rescap/prepare clause inventory drift:\ngot  %#v\nwant %#v",
			name, actual, want)
	}
	return nil
}

func s7APMarkdownClauses(body string) []string {
	var blocks []string
	var paragraph []string
	flush := func() {
		if len(paragraph) != 0 {
			blocks = append(blocks, strings.Join(paragraph, " "))
			paragraph = nil
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|") {
			flush()
			blocks = append(blocks, trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			flush()
			paragraph = append(paragraph, trimmed)
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	flush()
	var clauses []string
	replacer := strings.NewReplacer("`", "", "*", "", "_", " ")
	splitter := regexp.MustCompile(`[.!?](?:\s+|$)|\s+\|\s+`)
	for _, block := range blocks {
		block = strings.ToLower(replacer.Replace(block))
		for _, raw := range splitter.Split(block, -1) {
			if clause := strings.Trim(strings.Join(strings.Fields(raw), " "), "| "); clause != "" {
				clauses = append(clauses, clause)
			}
		}
	}
	return clauses
}

func validateS7APReferenceTruth(prd, adr string, expected s7APReferenceSet) error {
	actual, err := s7APReferenceGraph(prd, adr)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual.graph, expected.graph) {
		keys := make([]string, 0, len(actual.graph)+len(expected.graph))
		seen := map[string]bool{}
		for key := range actual.graph {
			seen[key] = true
			keys = append(keys, key)
		}
		for key := range expected.graph {
			if !seen[key] {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			if actual.graph[key] != expected.graph[key] {
				return fmt.Errorf("reference graph drift at %s: got %d want %d",
					key, actual.graph[key], expected.graph[key])
			}
		}
		return errors.New("reference graph drifted")
	}
	return nil
}
