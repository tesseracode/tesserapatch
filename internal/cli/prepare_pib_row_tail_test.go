//go:build (linux && !android) || (darwin && !ios)

package cli

// Owning acceptance tests for the last aggregate-ledger rows: the §6.1.2
// coherence table, the redaction-override and publication-unit claim guards,
// the CP2 orphan-blob state, the undo identity refusal and the pending-journal
// purge refusal.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ---------------------------------------------------------------------------
// PIB-250 — the §6.1.2 coherence table is total over the eight present/absent
// combinations, and a ninth synthetic combination refuses rather than
// defaulting to "generate".
// ---------------------------------------------------------------------------

type pibCoherenceRow struct {
	action string
	code   string
	exit   int
}

func pibCoherenceOutcome(present [3]bool, analysisState string) pibCoherenceRow {
	captured := func(is bool) prepareCaptured {
		if is {
			return prepareCaptured{state: intent.StatePresentNonempty}
		}
		return prepareCaptured{state: intent.StateAbsent}
	}
	state := prepareReadState{
		analysis:    captured(present[0]),
		spec:        captured(present[1]),
		exploration: captured(present[2]),
		sidecar:     prepareCaptured{state: intent.StateAbsent},
	}
	if analysisState != "" {
		state.analysis = prepareCaptured{state: analysisState}
	}
	plan, code, exit := buildPreparePlan(state, prepareModeGenerate)
	return pibCoherenceRow{action: plan.action, code: code, exit: exit}
}

func pibDeriveCoherenceTable() map[string]pibCoherenceRow {
	table := map[string]pibCoherenceRow{}
	for index := 0; index < 8; index++ {
		present := [3]bool{index&4 != 0, index&2 != 0, index&1 != 0}
		key := fmt.Sprintf("%v", present)
		table[key] = pibCoherenceOutcome(present, "")
	}
	return table
}

// pibValidateCoherenceTable requires the derived mapping to be total over
// exactly the eight combinations and to name an outcome for every one of them.
// A row that neither names a refusal code nor a mutating action is a silent
// default, which is the failure this guard exists to catch.
func pibValidateCoherenceTable(table map[string]pibCoherenceRow) error {
	if len(table) != 8 {
		return fmt.Errorf("the coherence table covers %d combinations, §6.1.2 fixes eight", len(table))
	}
	for index := 0; index < 8; index++ {
		present := [3]bool{index&4 != 0, index&2 != 0, index&1 != 0}
		key := fmt.Sprintf("%v", present)
		row, covered := table[key]
		if !covered {
			return fmt.Errorf("the coherence table does not cover %s", key)
		}
		named := row.code != "" || (row.action != "" && row.action != "none")
		if !named {
			return fmt.Errorf("%s resolves to no named outcome (%#v)", key, row)
		}
		if row.code != "" && row.exit == 0 {
			return fmt.Errorf("%s names refusal %q with exit 0", key, row.code)
		}
	}
	return nil
}

func TestPIBRowPIB250CoherenceTableTotality(t *testing.T) {
	derived := pibDeriveCoherenceTable()
	if err := pibValidateCoherenceTable(derived); err != nil {
		t.Fatalf("PIB-250: the shipped coherence table failed its own guard: %v", err)
	}
	// The ninth combination is a real out-of-vocabulary artifact state, and the
	// shipped classifier must refuse it rather than fall through to "generate".
	synthetic := pibCoherenceOutcome([3]bool{true, true, true}, "present-thin")
	if synthetic.code == "" || synthetic.exit == 0 {
		t.Fatalf("PIB-250: the ninth synthetic combination defaulted instead of refusing: %#v", synthetic)
	}
	widened := map[string]pibCoherenceRow{}
	for key, row := range derived {
		widened[key] = row
	}
	widened["[true true true present-thin]"] = pibCoherenceRow{action: "complete"}
	if err := pibValidateCoherenceTable(widened); err == nil {
		t.Fatal("PIB-250: the guard accepted a ninth combination that defaults to generate")
	}
	silent := map[string]pibCoherenceRow{}
	for key, row := range derived {
		silent[key] = row
	}
	silent["[false true false]"] = pibCoherenceRow{action: "none"}
	if err := pibValidateCoherenceTable(silent); err == nil {
		t.Fatal("PIB-250: the guard accepted a combination with no named outcome")
	}
}

// ---------------------------------------------------------------------------
// PIB-258 — no shipped string or document claims `--manual` writes exactly one
// file, and none folds FEATURES.md into the publication unit.
//
// Claims are read a sentence at a time, and a sentence ends at a terminator
// that is actually followed by whitespace or end of text. Splitting on every
// `.` used to break `FEATURES.md` and `status.json` into fragments, so a claim
// that named both the publication unit and the index in one breath was cut in
// half before the scan ever saw it.
// ---------------------------------------------------------------------------

// pibSentences segments text at real sentence terminators. A `.` inside a file
// name is followed by a letter, never by whitespace, so it is not a boundary.
func pibSentences(text string) []string {
	sentences := []string{}
	start := 0
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '.', '!', '?':
		default:
			continue
		}
		next := index + 1
		if next < len(text) {
			switch text[next] {
			case ' ', '\t', '\n', '\r', ')', '"', '\'', '`':
			default:
				continue
			}
		}
		sentences = append(sentences, text[start:next])
		start = next
	}
	if start < len(text) {
		sentences = append(sentences, text[start:])
	}
	return sentences
}

// pibSentenceDenies reports whether a sentence carries an explicit exclusion,
// which is what separates "FEATURES.md is not in the publication unit" — a
// sentence the product is entitled to write — from a sentence that folds it in.
func pibSentenceDenies(lower string) bool {
	for _, denial := range []string{
		"not ", "never ", "no part", "outside", "excluded", "excludes", "separate from",
	} {
		if strings.Contains(lower, denial) {
			return true
		}
	}
	return false
}

func pibValidatePublicationUnitClaims(texts map[string]string) error {
	if len(texts) == 0 {
		return fmt.Errorf("the publication-unit scan received no text")
	}
	quantifiers := []string{
		"exactly one file", "only one file", "a single file",
		"one file only", "just one file", "single-file",
	}
	for name, body := range texts {
		for _, sentence := range pibSentences(body) {
			lower := strings.ToLower(sentence)
			if pibSentenceDenies(lower) {
				continue
			}
			for _, quantifier := range quantifiers {
				if !strings.Contains(lower, quantifier) {
					continue
				}
				if strings.Contains(lower, "manual") || strings.Contains(lower, "publish") ||
					strings.Contains(lower, "publication") || strings.Contains(lower, "write") {
					return fmt.Errorf("%s claims a single-file publication unit: %q",
						name, strings.TrimSpace(sentence))
				}
			}
			if strings.Contains(lower, "publication unit") && strings.Contains(sentence, "FEATURES.md") {
				return fmt.Errorf("%s folds FEATURES.md into the publication unit: %q",
					name, strings.TrimSpace(sentence))
			}
		}
	}
	return nil
}

func TestPIBRowPIB258PublicationUnitClaims(t *testing.T) {
	corpus := pibGuardClaimCorpus(t)
	if err := pibValidatePublicationUnitClaims(corpus); err != nil {
		t.Fatalf("PIB-258: the shipped strings and docs failed their own guard: %v", err)
	}
	single := pibGuardClone(corpus)
	single["SPEC.md"] += "\nprepare --manual writes exactly one file.\n"
	if err := pibValidatePublicationUnitClaims(single); err == nil {
		t.Fatal("PIB-258: the guard accepted a single-file claim")
	}
	paraphrased := pibGuardClone(corpus)
	paraphrased["SPEC.md"] += "\nAdopting a hand-authored bundle publishes a single file.\n"
	if err := pibValidatePublicationUnitClaims(paraphrased); err == nil {
		t.Fatal("PIB-258: the guard accepted a paraphrased single-file claim")
	}
	folded := pibGuardClone(corpus)
	folded["SPEC.md"] += "\nThe publication unit is status.json and FEATURES.md together.\n"
	if err := pibValidatePublicationUnitClaims(folded); err == nil {
		t.Fatal("PIB-258: the guard accepted FEATURES.md inside the publication unit")
	}
	// Control: naming FEATURES.md beside the publication unit in order to
	// exclude it is legal neighbouring behaviour and must still pass.
	excluded := pibGuardClone(corpus)
	excluded["SPEC.md"] += "\nFEATURES.md is reconverged by the next transition and is " +
		"not part of the publication unit.\n"
	if err := pibValidatePublicationUnitClaims(excluded); err != nil {
		t.Fatalf("PIB-258: the guard rejected an explicit exclusion: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PIB-267 — nothing offers to skip or override the redaction scan.
//
// The guard is taken over the surface the row names — flags, config keys and
// messages — and the flag/help half is derived from the real cobra commands
// rather than from a source scan, so a registered flag is caught by its
// registered name (`force-archive`, as cobra stores it) instead of by a
// dashed spelling that only appears in prose. §18.53's own fixture is a help
// string that offers to "skip the scan for trusted repositories" *without*
// naming `--force`, so a token list alone cannot be the whole guard.
// ---------------------------------------------------------------------------

// pibCommandSurface derives every flag name, flag usage string and help text
// the named shipped commands register, keyed by where it came from.
func pibCommandSurface(commands ...*cobra.Command) map[string]string {
	surface := map[string]string{}
	var walk func(prefix string, command *cobra.Command)
	walk = func(prefix string, command *cobra.Command) {
		name := strings.TrimSpace(prefix + " " + command.Name())
		surface["help:"+name+":short"] = command.Short
		surface["help:"+name+":long"] = command.Long
		command.Flags().VisitAll(func(flag *pflag.Flag) {
			surface["flag:"+name+":"+flag.Name] = flag.Name
			surface["usage:"+name+":"+flag.Name] = flag.Usage
		})
		for _, child := range command.Commands() {
			walk(name, child)
		}
	}
	for _, command := range commands {
		walk("", command)
	}
	return surface
}

// pibConfigKeys derives the shipped repository/global configuration keys from
// the shipped struct tags, so a config key added to offer an override is seen.
func pibConfigKeys() []string {
	keys := []string{}
	var walk func(structType reflect.Type)
	walk = func(structType reflect.Type) {
		for index := 0; index < structType.NumField(); index++ {
			field := structType.Field(index)
			tag := strings.Split(field.Tag.Get("json"), ",")[0]
			if tag != "" && tag != "-" {
				keys = append(keys, tag)
			}
			if field.Type.Kind() == reflect.Struct {
				walk(field.Type)
			}
		}
	}
	walk(reflect.TypeOf(store.Config{}))
	return keys
}

// pibRedactionOverrideNameTokens are the substrings a flag or config key may
// not carry: each one names a way to stand the redaction scan down.
var pibRedactionOverrideNameTokens = []string{
	"force", "skip", "override", "bypass", "unsafe", "redact", "sensitive", "insecure",
}

// pibBypassVerbs are the verbs an offer to stand the scan down is written with.
// They are matched only as `<verb> <object>`, in that order, so a sentence that
// says the scan *cannot* be disabled does not read as an offer to disable it.
var pibBypassVerbs = []string{
	"skip", "bypass", "override", "disable", "turn off", "opt out of", "suppress", "ignore",
}

// pibRedactionOfferPhrases builds the offer shapes. The prose corpus is bound
// to the redaction noun; the derived command surface additionally admits the
// bare "the scan", because on `prepare` and `feature intent-archive` the only
// scan there is is the redaction scan — which is exactly how §18.53's fixture
// ("skip the scan for trusted repositories") evades a `--force` token list.
func pibRedactionOfferPhrases(commandSurface bool) []string {
	objects := []string{"the redaction scan", "the redaction", "redaction"}
	if commandSurface {
		objects = append(objects, "the scan", "the sensitive-content scan", "scanning")
	}
	phrases := make([]string, 0, len(pibBypassVerbs)*len(objects))
	for _, verb := range pibBypassVerbs {
		for _, object := range objects {
			phrases = append(phrases, verb+" "+object)
		}
	}
	return phrases
}

func pibValidateNoRedactionOverride(texts map[string]string, surface map[string]string, configKeys []string) error {
	if len(texts) == 0 || len(surface) == 0 || len(configKeys) == 0 {
		return fmt.Errorf("the redaction-override scan received an empty surface")
	}
	forbidden := []string{
		"--force" + "-archive",
		"--skip" + "-redaction",
		"--no" + "-redact",
	}
	prosePhrases := pibRedactionOfferPhrases(false)
	surfacePhrases := pibRedactionOfferPhrases(true)
	for name, body := range texts {
		lower := strings.ToLower(body)
		for _, token := range forbidden {
			if strings.Contains(lower, token) {
				return fmt.Errorf("%s offers %q", name, token)
			}
		}
		for _, phrase := range prosePhrases {
			if strings.Contains(lower, phrase) {
				return fmt.Errorf("%s offers to %q", name, phrase)
			}
		}
	}
	for origin, value := range surface {
		lower := strings.ToLower(value)
		if strings.HasPrefix(origin, "flag:") {
			for _, token := range pibRedactionOverrideNameTokens {
				if strings.Contains(lower, token) {
					return fmt.Errorf("%s registers the override flag %q", origin, value)
				}
			}
			continue
		}
		for _, token := range forbidden {
			if strings.Contains(lower, token) {
				return fmt.Errorf("%s offers %q", origin, token)
			}
		}
		for _, phrase := range surfacePhrases {
			if strings.Contains(lower, phrase) {
				return fmt.Errorf("%s offers to %q: %q", origin, phrase, strings.TrimSpace(value))
			}
		}
	}
	for _, key := range configKeys {
		lower := strings.ToLower(key)
		for _, token := range pibRedactionOverrideNameTokens {
			if strings.Contains(lower, token) {
				return fmt.Errorf("the configuration key %q offers an override", key)
			}
		}
	}
	return nil
}

func TestPIBRowPIB267NoRedactionOverride(t *testing.T) {
	corpus := pibGuardClaimCorpus(t)
	for name, body := range pibGuardCLIProductionSources(t) {
		corpus["source:"+name] = body
	}
	surface := pibCommandSurface(prepareCmd(), featureIntentArchiveCmd())
	configKeys := pibConfigKeys()
	if err := pibValidateNoRedactionOverride(corpus, surface, configKeys); err != nil {
		t.Fatalf("PIB-267: the shipped surface failed its own guard: %v", err)
	}
	flagged := pibCommandSurface(prepareCmd(), featureIntentArchiveCmd())
	flagged["flag: prepare:force"+"-archive"] = "force" + "-archive"
	flagged["usage: prepare:force"+"-archive"] = "Archive prior bytes without inspecting them"
	if err := pibValidateNoRedactionOverride(corpus, flagged, configKeys); err == nil {
		t.Fatal("PIB-267: the guard accepted a redaction-override flag on the shipped command surface")
	}
	// §18.53's own fixture: a help string that offers the escape without ever
	// naming a `--force` token.
	helped := pibCommandSurface(prepareCmd(), featureIntentArchiveCmd())
	helped["usage: prepare:regenerate"] = "Replace the bundle and skip the scan for trusted repositories"
	if err := pibValidateNoRedactionOverride(corpus, helped, configKeys); err == nil {
		t.Fatal("PIB-267: the guard accepted a help string offering to skip the scan")
	}
	keyed := append(append([]string(nil), configKeys...), "archive_redaction_disabled")
	if err := pibValidateNoRedactionOverride(corpus, surface, keyed); err == nil {
		t.Fatal("PIB-267: the guard accepted a redaction-override configuration key")
	}
	worded := pibGuardClone(corpus)
	worded["SPEC.md"] += "\nOperators may skip the redaction scan when it is inconvenient.\n"
	if err := pibValidateNoRedactionOverride(worded, surface, configKeys); err == nil {
		t.Fatal("PIB-267: the guard accepted prose offering to skip the scan")
	}
	// Control: a help string that names the redaction scan without offering to
	// stand it down is legal neighbouring behaviour and must still pass.
	neighbour := pibCommandSurface(prepareCmd(), featureIntentArchiveCmd())
	neighbour["usage: prepare:regenerate"] =
		"Replace the complete intent bundle; the redaction scan always runs over the prior bytes"
	neighbour["usage: prepare:quiet"] = "Suppress the human report"
	if err := pibValidateNoRedactionOverride(corpus, neighbour, configKeys); err != nil {
		t.Fatalf("PIB-267: the guard rejected a legal mention of the redaction scan: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PIB-118 — CP2: blobs on disk with no index entry stay as reported residue,
// the external locator stays outside cleanup, and the route names purge.
// ---------------------------------------------------------------------------

func TestPIBRowPIB118CrashPhaseTwoOrphanBlobs(t *testing.T) {
	root, slug := pibRowPublishedWorkspace(t, "PIB row 118")
	if code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
	); code != 0 {
		t.Fatalf("PIB-118: the seeding regenerate = exit %d stderr=%q", code, stderr)
	}
	blobsDir := filepath.Join(pibRowArchiveDir(root, slug), "blobs")
	orphans := map[string][]byte{}
	for _, body := range [][]byte{
		[]byte("crash phase two orphan one\n"),
		[]byte("crash phase two orphan two\n"),
	} {
		hash := pibRowSHA256(body)
		orphans[hash] = body
		if err := os.WriteFile(filepath.Join(blobsDir, hash+".blob"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The external locator lives outside the archive and must stay untouched.
	locator := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "external-locator")
	if err := os.MkdirAll(filepath.Dir(locator), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locator, []byte("external locator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(pibRowArchiveDir(root, slug), "index.json")
	indexBefore, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	// The observable §7 assigns to CP2 is that the blobs survive as *reported*
	// residue with the purge route named — whether the run refuses on the
	// inconsistency or completes carrying the repair advisory.
	routed := ""
	if report.Refusal != nil {
		routed = report.Refusal.Message + " " + report.Refusal.Remediation
	}
	for _, advisory := range report.Advisories {
		routed += " " + advisory.Message
	}
	if report.Outcome == "" {
		t.Fatalf("PIB-118: CP2 residue produced no named outcome (exit %d)\n%s", code, stdout)
	}
	if !strings.Contains(routed, "intent-archive purge") {
		t.Fatalf("PIB-118: no route names purge (exit %d, outcome %q): %q", code, report.Outcome, routed)
	}
	for hash, body := range orphans {
		got, err := os.ReadFile(filepath.Join(blobsDir, hash+".blob"))
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("PIB-118: orphan blob %s was cleaned: %v", hash, err)
		}
	}
	if _, err := os.Stat(locator); err != nil {
		t.Fatalf("PIB-118: the external locator was swept into cleanup: %v", err)
	}
	indexAfter, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(indexBefore, indexAfter) {
		t.Fatal("PIB-118: the index gained an entry for the orphan blobs")
	}
	for hash := range orphans {
		if strings.Contains(string(indexAfter), hash) {
			t.Fatalf("PIB-118: the index references orphan blob %s", hash)
		}
	}
}

// ---------------------------------------------------------------------------
// PIB-276 — a third party removes a published create entry before its undo.
// ---------------------------------------------------------------------------

func TestPIBRowPIB276UndoIdentityIsAbsence(t *testing.T) {
	root, slug := prepareS4Workspace(t, "PIB row 276")
	if err := os.WriteFile(
		filepath.Join(pibRowFeature(root, slug), "analysis.md"), []byte("preserved analysis\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	old := prepareIntentpubHook
	t.Cleanup(func() { prepareIntentpubHook = old })
	published := ""
	casCalls := 0
	removed := false
	prepareIntentpubHook = func(
		point intentpub.CrashPoint, rooted *os.Root, entry *intentpub.Entry,
	) error {
		switch point {
		case intentpub.PointAfterEntryRename:
			if entry != nil && published == "" {
				published = entry.Rel
			}
		case intentpub.PointBeforeEntryCAS:
			casCalls++
			if entry == nil || casCalls < 2 {
				return nil
			}
			file, err := rooted.OpenFile(entry.StagedRel, os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			if _, err := file.Write([]byte("changed staged bytes\n")); err != nil {
				_ = file.Close()
				return err
			}
			return file.Close()
		case intentpub.PointBeforeUndo:
			if entry == nil || removed || entry.Rel != published {
				return nil
			}
			removed = true
			return rooted.Remove(entry.Rel)
		}
		return nil
	}
	code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	prepareIntentpubHook = old
	report := prepareS4Report(t, stdout)
	if published == "" || !removed {
		t.Fatalf("PIB-276: the third-party removal never fired (published=%q removed=%v)\n%s",
			published, removed, stdout)
	}
	if code != 6 || report.Refusal == nil || report.Refusal.Code == "" ||
		report.Refusal.Remediation == "" {
		t.Fatalf("PIB-276: undo over a removed create entry = exit %d refusal=%#v", code, report.Refusal)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(published))); !os.IsNotExist(err) {
		t.Fatalf("PIB-276: the refused undo recreated %s: %v", published, err)
	}
}

// ---------------------------------------------------------------------------
// PIB-350 — purge with a pending journal refuses without parsing or renaming it.
// ---------------------------------------------------------------------------

func TestPIBRowPIB350PurgeRefusesPendingJournal(t *testing.T) {
	root, slug := intentArchiveCLIWorkspace(t)
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	if err := os.MkdirAll(lane, 0o700); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(lane, "journal.json")
	original := []byte("{pending marker")
	if err := os.WriteFile(journal, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := readTree(t, filepath.Join(root, ".tpatch"))
	previous := intentArchiveJournals
	spy := &intentArchiveJournalSpy{delegate: previous}
	intentArchiveJournals = spy
	t.Cleanup(func() { intentArchiveJournals = previous })
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--yes", "--json", "--quiet",
	)
	if code != 3 {
		t.Fatalf("PIB-350: purge with a pending journal = exit %d\n%s", code, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if report.Refusal == nil || report.Refusal.Code != "recovery-pending" {
		t.Fatalf("PIB-350: refusal = %#v", report.Refusal)
	}
	if report.Refusal.Retry != "tpatch prepare "+slug ||
		!strings.Contains(report.Refusal.Remediation, "--abandon-transaction --yes") {
		t.Fatalf("PIB-350: the refusal does not name both recovery routes: %#v", report.Refusal)
	}
	if spy.decodes != 0 || spy.renames != 0 {
		t.Fatalf("PIB-350: the refusal parsed or renamed the journal: decodes=%d renames=%d",
			spy.decodes, spy.renames)
	}
	if got, err := os.ReadFile(journal); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("PIB-350: the journal changed: %v %q", err, got)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("PIB-350: the pending-journal refusal changed the workspace")
	}
}
