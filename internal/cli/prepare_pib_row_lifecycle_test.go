//go:build (linux && !android) || (darwin && !ios)

package cli

// Owning acceptance tests for the `--manual`, lifecycle and recovery rows of
// the aggregate ledger. Each leaf performs its own setup and asserts the exact
// observable §18 assigns to the row.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const pibRowManualNotes = "Intent bundle adopted (prepare --manual); artifacts authored by hand"

// pibRowRenameFaultOps refuses exactly one rename whose destination carries the
// configured suffix, which is the in-process stand-in for a process that dies
// between acquiring the authority and completing that rename.
type pibRowRenameFaultOps struct {
	intentpub.RootOps
	suffix string
	fired  *bool
}

func (ops *pibRowRenameFaultOps) Rename(oldName, newName string) error {
	if !*ops.fired && strings.HasSuffix(filepath.ToSlash(newName), ops.suffix) {
		*ops.fired = true
		return errors.New("injected interruption before the canonical rename")
	}
	return ops.RootOps.Rename(oldName, newName)
}

func pibRowLaneJournal(root, slug string) string {
	return filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "journal.json")
}

func pibRowStatusDocument(t *testing.T, root, slug string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(pibRowFeature(root, slug), "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

// TestPIBRowPrepareManualContracts owns the `--manual` adoption rows.
func TestPIBRowPrepareManualContracts(t *testing.T) {
	t.Run("PIB-045", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 045")
		prepareS4WriteReadyBundle(t, root, slug, false)
		loads := pibRowInstallProvider(t, &pibRowRecordingProvider{})
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("manual adoption = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if *loads != 0 {
			t.Fatalf("PIB-045: --manual loaded a provider %d time(s)", *loads)
		}
	})

	t.Run("PIB-047", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 047")
		feature := pibRowFeature(root, slug)
		for name, body := range map[string]string{
			"analysis.md": "hand analysis\n",
			"spec.md":     "hand specification\n",
		} {
			if err := os.WriteFile(filepath.Join(feature, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 2 || report.Refusal == nil || report.Refusal.Code != "not-ready" {
			t.Fatalf("--manual with exploration.md absent = exit %d refusal=%#v", code, report.Refusal)
		}
		if len(report.Artifacts) != 4 {
			t.Fatalf("PIB-047: refusal report carries %d artifact rows, want the full four", len(report.Artifacts))
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-047: the not-ready refusal mutated the workspace")
		}
	})

	t.Run("PIB-048", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 048")
		prepareS4WriteReadyBundle(t, root, slug, false)
		if err := os.WriteFile(
			filepath.Join(pibRowFeature(root, slug), "spec.md"), nil, 0o644,
		); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 2 || report.Refusal == nil || report.Refusal.Code != "not-ready" {
			t.Fatalf("--manual with a zero-byte spec.md = exit %d refusal=%#v", code, report.Refusal)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-048: the zero-byte refusal mutated the workspace")
		}
	})

	t.Run("PIB-049", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 049")
		prepareS4WriteReadyBundle(t, root, slug, false)
		features := filepath.Join(root, ".tpatch", "FEATURES.md")
		if err := os.RemoveAll(features); err != nil {
			t.Fatal(err)
		}
		// A directory at the index path is unwritable for every identity,
		// including one that bypasses mode bits.
		if err := os.Mkdir(features, 0o755); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" {
			t.Fatalf("--manual with an unwritable index = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if report.Outcome != "published" {
			t.Fatalf("PIB-049: outcome = %q, want published", report.Outcome)
		}
		if document := pibRowStatusDocument(t, root, slug); document["state"] != "defined" {
			t.Fatalf("PIB-049: status.json was not published: %v", document["state"])
		}
		advisory := false
		for _, item := range report.Advisories {
			advisory = advisory || item.Code == "features-index-refresh-failed"
		}
		if !advisory {
			t.Fatalf("PIB-049: the index-refresh failure was not disclosed: %#v", report.Advisories)
		}
	})

	t.Run("PIB-050", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 050")
		prepareS4WriteReadyBundle(t, root, slug, false)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("manual adoption = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		document := pibRowStatusDocument(t, root, slug)
		if document["state"] != "defined" {
			t.Fatalf("PIB-050: state = %v, want defined", document["state"])
		}
		if document["last_command"] != "prepare" {
			t.Fatalf("PIB-050: last_command = %v, want prepare", document["last_command"])
		}
		if document["notes"] != pibRowManualNotes {
			t.Fatalf("PIB-050: notes = %v, want the frozen adoption string", document["notes"])
		}
	})

	t.Run("PIB-053", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 053")
		prepareS4WriteReadyBundle(t, root, slug, false)
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatalf("hold the workspace authority: %v", err)
		}
		defer func() { _ = authority.Release() }()
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "transaction-in-progress" {
			t.Fatalf("--manual under contention = exit %d refusal=%#v", code, report.Refusal)
		}
		if !strings.Contains(report.Refusal.Remediation, "Wait") ||
			!strings.Contains(report.Refusal.Remediation, "retry") {
			t.Fatalf("PIB-053: remediation does not say to wait and retry: %q", report.Refusal.Remediation)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-053: the contention refusal mutated the workspace")
		}
	})

	t.Run("PIB-056", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 056")
		prepareS4WriteReadyBundle(t, root, slug, false)
		if code, _, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		); code != 0 {
			t.Fatalf("first manual adoption = exit %d stderr=%q", code, stderr)
		}
		statusBefore := pibRowFileIdentity(t, filepath.Join(pibRowFeature(root, slug), "status.json"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" {
			t.Fatalf("second manual adoption = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if report.Outcome != "no-op" {
			t.Fatalf("PIB-056: second adoption outcome = %q, want no-op", report.Outcome)
		}
		after := pibRowFileIdentity(t, filepath.Join(pibRowFeature(root, slug), "status.json"))
		if after != statusBefore {
			t.Fatalf("PIB-056: the no-op rewrote status.json\n got %s\nwant %s", after, statusBefore)
		}
	})

	t.Run("PIB-057", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 057")
		if err := os.WriteFile(
			filepath.Join(pibRowFeature(root, slug), "analysis.md"), []byte("hand analysis\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 2 || report.Mode != prepareModeManual || report.Outcome != "refused" {
			t.Fatalf("--manual refusal report = exit %d mode=%q outcome=%q", code, report.Mode, report.Outcome)
		}
		if report.Refusal == nil || report.Refusal.Code == "" || report.Refusal.Remediation == "" {
			t.Fatalf("PIB-057: refusal object = %#v", report.Refusal)
		}
		wanted := map[string]bool{
			"analysis": false, "spec": false, "exploration": false, "analysis_sidecar": false,
		}
		for _, artifact := range report.Artifacts {
			if _, expected := wanted[artifact.ID]; expected {
				wanted[artifact.ID] = true
			}
		}
		for id, present := range wanted {
			if !present {
				t.Fatalf("PIB-057: the refusal report omits the %s row: %#v", id, report.Artifacts)
			}
		}
	})

	t.Run("PIB-076", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 076")
		if err := os.WriteFile(
			filepath.Join(pibRowFeature(root, slug), "analysis.md"), []byte("hand analysis\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		dryCode, dryOut, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--dry-run", "--json", "--quiet",
		)
		dry := prepareS4Report(t, dryOut)
		realCode, realOut, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		live := prepareS4Report(t, realOut)
		if dryCode != 2 || dry.Refusal == nil {
			t.Fatalf("--dry-run --manual on an incomplete bundle = exit %d refusal=%#v", dryCode, dry.Refusal)
		}
		if realCode != dryCode || live.Refusal == nil || live.Refusal.Code != dry.Refusal.Code {
			t.Fatalf("PIB-076: dry (%d,%s) and real (%d,%v) refusals differ",
				dryCode, dry.Refusal.Code, realCode, live.Refusal)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-076: the dry-run refusal mutated the workspace")
		}
	})

	t.Run("PIB-131", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 131")
		feature := pibRowFeature(root, slug)
		for name, body := range map[string]string{
			"analysis.md":    "hand analysis\n",
			"exploration.md": "hand exploration\n",
		} {
			if err := os.WriteFile(filepath.Join(feature, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(feature, "spec.md"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 2 || report.Mode != prepareModeManual {
			t.Fatalf("--manual over a zero-byte spec.md = exit %d mode=%q", code, report.Mode)
		}
	})

	t.Run("PIB-132", func(t *testing.T) {
		// The composite: one workspace, the `--check` observation of the
		// zero-byte specification followed by the mutating `--manual` refusal
		// over the same tree, with zero mutation from the second.
		root, slug := prepareS4Workspace(t, "PIB row 132")
		feature := pibRowFeature(root, slug)
		for name, body := range map[string]string{
			"analysis.md":    "hand analysis\n",
			"exploration.md": "hand exploration\n",
		} {
			if err := os.WriteFile(filepath.Join(feature, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(feature, "spec.md"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		checkCode, checkOut, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--json", "--quiet",
		)
		if checkCode != 2 || !strings.Contains(checkOut, "\"not_ready\"") {
			t.Fatalf("PIB-132: --check over a zero-byte spec = exit %d\n%s", checkCode, checkOut)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		manualCode, manualOut, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, manualOut)
		if manualCode != 2 || report.Refusal == nil || report.Refusal.Code != "not-ready" {
			t.Fatalf("PIB-132: --manual over the same tree = exit %d refusal=%#v", manualCode, report.Refusal)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-132: the second observation mutated the workspace")
		}
	})

	t.Run("PIB-257", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 257")
		prepareS4WriteReadyBundle(t, root, slug, false)
		spy := pibRowInstallWriteSpy(t)
		before := pibRowTreeSnapshot(t, root)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("manual adoption = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		after := pibRowTreeSnapshot(t, root)
		// The local lane is transaction scratch, not a publication write
		// target; §6 counts the publication unit, which is what this asserts.
		changed := pibRowExcludeLane(pibRowChangedPaths(before, after))
		want := map[string]bool{
			".tpatch/features/" + slug + "/status.json": true,
			".tpatch/FEATURES.md":                       true,
		}
		statusRel := ".tpatch/features/" + slug + "/status.json"
		wrote := map[string]bool{}
		for _, path := range changed {
			if !want[path] {
				t.Fatalf("PIB-257: --manual wrote outside the two-file unit: %q (all %v)", path, changed)
			}
			wrote[path] = true
		}
		if !wrote[statusRel] {
			t.Fatalf("PIB-257: --manual did not write status.json (changed %v)", changed)
		}
		statusRenames := 0
		for _, name := range spy.renames {
			if strings.HasSuffix(name, "status.json") {
				statusRenames++
			}
		}
		if statusRenames != 1 {
			t.Fatalf("PIB-257: status.json was published by %d renames, want exactly 1: %v",
				statusRenames, spy.renames)
		}
	})

	t.Run("PIB-292", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 292")
		prepareS4WriteReadyBundle(t, root, slug, false)
		fired := false
		old := prepareIntentpubRootOps
		t.Cleanup(func() { prepareIntentpubRootOps = old })
		prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
			return &pibRowRenameFaultOps{
				RootOps: intentpub.NewRootOps(rooted),
				suffix:  "status.json",
				fired:   &fired,
			}
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		prepareIntentpubRootOps = old
		if !fired {
			t.Fatalf("PIB-292: the pre-rename interruption never fired\n%s", stdout)
		}
		if code == 0 {
			t.Fatalf("PIB-292: the interrupted --manual reported success\n%s", stdout)
		}
		if _, err := os.Stat(pibRowLaneJournal(root, slug)); !os.IsNotExist(err) {
			t.Fatalf("PIB-292: --manual left a journal behind: %v", err)
		}
		if document := pibRowStatusDocument(t, root, slug); document["state"] == "defined" {
			t.Fatal("PIB-292: the interrupted --manual published status.json anyway")
		}
		nextCode, nextOut, nextErr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if nextCode != 0 || nextErr != "" {
			t.Fatalf("PIB-292: the next --manual = exit %d stderr=%q\n%s", nextCode, nextErr, nextOut)
		}
		if document := pibRowStatusDocument(t, root, slug); document["state"] != "defined" {
			t.Fatalf("PIB-292: the next --manual did not adopt: %v", document["state"])
		}
	})
}

// TestPIBRowPrepareLifecycleAndRecovery owns the state-gate and crash-recovery
// rows.
func TestPIBRowPrepareLifecycleAndRecovery(t *testing.T) {
	t.Run("PIB-128", func(t *testing.T) {
		// `reopen` restores a feature's PriorState; prepare is admissible
		// exactly when that restored state is in §6's allowed set.
		//
		// The workspace title is derived through the shipped `store.Slugify`
		// before it is handed to `tpatch add`, because one member of the state
		// vocabulary — `upstream_merged` — carries an underscore that the
		// shipped slug rule folds to a dash. Building the title from a raw
		// state name produced a fixture that addressed a directory the CLI had
		// never created, so the bundle write landed on a missing path instead
		// of exercising the state gate. The identity is asserted rather than
		// assumed, and the restored feature directory is proved to exist
		// before anything is written into it.
		allowed := map[string]bool{"requested": true, "analyzed": true, "defined": true}
		states := s6StoreFeatureStates(t)
		if len(states) < 6 {
			t.Fatalf("PIB-128: the shipped state vocabulary has only %d members", len(states))
		}
		observed := 0
		refusals := 0
		admissions := 0
		for _, state := range states {
			title := "PIB row 128 " + store.Slugify(state)
			root, slug := prepareS4Workspace(t, title)
			if slug != store.Slugify(title) {
				t.Fatalf("PIB-128: fixture slug %q is not the shipped slug %q for state %q",
					slug, store.Slugify(title), state)
			}
			feature := pibRowFeature(root, slug)
			if info, err := os.Stat(feature); err != nil || !info.IsDir() {
				t.Fatalf("PIB-128: the feature directory for state %q does not exist: %v", state, err)
			}
			prepareS4WriteReadyBundle(t, root, slug, false)
			statusPath := filepath.Join(feature, "status.json")
			raw, err := os.ReadFile(statusPath)
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			document["state"] = state
			restored, err := json.MarshalIndent(document, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(statusPath, append(restored, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			if report.FeatureState == "" {
				t.Fatalf("PIB-128: the report echoed no restored state for %q\n%s", state, stdout)
			}
			if allowed[state] {
				if code != 0 || report.Refusal != nil ||
					report.Outcome != "published" || report.Action != "adopt" {
					t.Fatalf("PIB-128: prepare did not adopt the allowed restored state %q: exit %d %#v",
						state, code, report)
				}
				admissions++
			} else {
				if code != 3 || report.Refusal == nil || report.Refusal.Code != "state-refused" {
					t.Fatalf("PIB-128: prepare admitted the disallowed restored state %q: exit %d %#v",
						state, code, report)
				}
				if report.Refusal.Remediation != prepareStateRemediation(slug) {
					t.Fatalf("PIB-128: the %q refusal does not name the reopen route: %q",
						state, report.Refusal.Remediation)
				}
				refusals++
			}
			observed++
		}
		if observed != len(states) {
			t.Fatalf("PIB-128: observed %d of %d restored states", observed, len(states))
		}
		if admissions != len(allowed) || refusals != len(states)-len(allowed) {
			t.Fatalf("PIB-128: the gate admitted %d and refused %d of %d restored states",
				admissions, refusals, len(states))
		}
	})

	t.Run("PIB-259", func(t *testing.T) {
		root, slug, preimage := pibRowRolledBackPublication(t)
		status, err := os.ReadFile(filepath.Join(pibRowFeature(root, slug), "status.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(status, preimage) {
			t.Fatalf("PIB-259: status.json is not back to its preimage\n got %q\nwant %q", status, preimage)
		}
		features, err := os.ReadFile(filepath.Join(root, ".tpatch", "FEATURES.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(features), slug) {
			t.Fatalf("PIB-259: the refreshed index does not name the restored feature:\n%s", features)
		}
		want, err := renderPrepareFeaturesIndex(&store.Store{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(features, want) {
			t.Fatalf("PIB-259: the index does not name the restored state\n got %q\nwant %q", features, want)
		}
	})

	t.Run("PIB-260", func(t *testing.T) {
		writable := pibRowInterruptedThenRecover(t, false)
		unwritable := pibRowInterruptedThenRecover(t, true)
		if writable.recoveryCode != 0 {
			t.Fatalf("PIB-260: recovery with a writable index = exit %d\n%s",
				writable.recoveryCode, writable.recoveryStdout)
		}
		if unwritable.recoveryCode != writable.recoveryCode {
			t.Fatalf("PIB-260: a failed index refresh changed the recovery exit code (%d versus %d)",
				unwritable.recoveryCode, writable.recoveryCode)
		}
		for _, observation := range []pibRowRecoveryObservation{writable, unwritable} {
			if observation.recoveryReport.Recovery == nil {
				t.Fatalf("PIB-260: the recovery invocation reported no recovery: %#v", observation.recoveryReport)
			}
			if observation.statusAfter["state"] == "defined" {
				t.Fatal("PIB-260: recovery did not restore status.json to its preimage")
			}
		}
		features, err := os.ReadFile(filepath.Join(writable.root, ".tpatch", "FEATURES.md"))
		if err != nil {
			t.Fatal(err)
		}
		want, err := renderPrepareFeaturesIndex(&store.Store{Root: writable.root})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(features, want) {
			t.Fatalf("PIB-260: recovery did not run the index refresh as its last act\n got %q\nwant %q",
				features, want)
		}
	})

	t.Run("PIB-294", func(t *testing.T) {
		// The row's last clause is "the final run publishes". The bundle is
		// left empty on purpose so the plan the final run executes is the
		// four-artifact `complete` action of §6.1.2 — a genuine publication of
		// the whole intent bundle, including the `artifacts/analysis.json`
		// sidecar. Seeding `analysis.md` first put the slug on the Path-B
		// suffix instead, where the sidecar is `absent-optional` by contract
		// and never published, so the final assertion was demanding a file the
		// planned action does not produce.
		root, slug := prepareS4Workspace(t, "PIB row 294")
		feature := pibRowFeature(root, slug)
		for _, rel := range []string{
			"analysis.md", "spec.md", "exploration.md", "artifacts/analysis.json",
		} {
			if _, err := os.Stat(filepath.Join(feature, filepath.FromSlash(rel))); !os.IsNotExist(err) {
				t.Fatalf("PIB-294: the cycle did not start from an empty bundle (%s: %v)", rel, err)
			}
		}
		baseline := prepareIntentpubHook
		t.Cleanup(func() { prepareIntentpubHook = baseline })
		for cycle := 1; cycle <= 10; cycle++ {
			old := prepareIntentpubHook
			prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
				if point == intentpub.PointAfterJournalDurable {
					return errors.New("injected kill after the journal became durable")
				}
				return nil
			}
			killedCode, killedOut, _, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--json", "--quiet",
			)
			prepareIntentpubHook = old
			killed := prepareS4Report(t, killedOut)
			if killedCode != 6 || killed.Outcome != "recovery-refused" ||
				killed.Refusal == nil || killed.Refusal.Code == "" || killed.Refusal.Remediation == "" {
				t.Fatalf("PIB-294 cycle %d: the killed run has no named outcome: exit %d %#v",
					cycle, killedCode, killed)
			}
			route := killed.Refusal.Remediation
			if !strings.Contains(route, "tpatch prepare "+slug) {
				t.Fatalf("PIB-294 cycle %d: the refusal names no applicable route out: %q", cycle, route)
			}
			if _, err := os.Stat(pibRowLaneJournal(root, slug)); err != nil {
				t.Fatalf("PIB-294 cycle %d: the killed run retained no journal: %v", cycle, err)
			}
			rerunCode, rerunOut, _, _ := runPrepare(
				t, "--path", root, "prepare", slug, "--json", "--quiet",
			)
			rerun := prepareS4Report(t, rerunOut)
			if rerunCode != 0 || rerun.Recovery == nil {
				t.Fatalf("PIB-294 cycle %d: following the route made no progress: exit %d %#v",
					cycle, rerunCode, rerun)
			}
			if _, err := os.Stat(pibRowLaneJournal(root, slug)); !os.IsNotExist(err) {
				t.Fatalf("PIB-294 cycle %d: recovery left the journal behind: %v", cycle, err)
			}
		}
		finalCode, finalOut, finalErr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		final := prepareS4Report(t, finalOut)
		if finalCode != 0 || finalErr != "" || final.Outcome != "published" ||
			final.Action != "complete" {
			t.Fatalf("PIB-294: the final run did not publish: exit %d stderr=%q outcome=%q action=%q",
				finalCode, finalErr, final.Outcome, final.Action)
		}
		// The published set is read out of the shipped report rather than
		// guessed, and then confirmed on disk.
		generated := map[string]bool{}
		for _, artifact := range final.Artifacts {
			if artifact.Disposition != "generated" {
				continue
			}
			generated[strings.TrimPrefix(artifact.Path, prepareFeatureRel(slug)+"/")] = true
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.Path)))
			if err != nil || len(body) == 0 {
				t.Fatalf("PIB-294: the final run reported %s as generated but did not publish it: %v",
					artifact.Path, err)
			}
		}
		for _, rel := range []string{
			"analysis.md", "spec.md", "exploration.md", "artifacts/analysis.json",
		} {
			if !generated[rel] {
				t.Fatalf("PIB-294: the final run did not report %s as published: %#v",
					rel, final.Artifacts)
			}
			if _, err := os.Stat(filepath.Join(feature, filepath.FromSlash(rel))); err != nil {
				t.Fatalf("PIB-294: the final run did not publish %s: %v", rel, err)
			}
		}
	})
}

// pibRowRolledBackPublication drives a real rolled-back publication: the staged
// bytes of the first entry are changed inside the CAS window, so the whole set
// is restored and the run exits 5.
func pibRowRolledBackPublication(t *testing.T) (string, string, []byte) {
	t.Helper()
	root, slug := prepareS4Workspace(t, "PIB rollback publication")
	if code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	); code != 0 {
		t.Fatalf("initial prepare = %d: %s", code, stderr)
	}
	feature := pibRowFeature(root, slug)
	prior := []byte("prior hand bytes\n")
	for _, rel := range []string{"analysis.md", "spec.md", "exploration.md"} {
		if err := os.WriteFile(filepath.Join(feature, rel), prior, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	statusPreimage, err := os.ReadFile(filepath.Join(feature, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := prepareIntentpubHook
	t.Cleanup(func() { prepareIntentpubHook = old })
	changed := false
	prepareIntentpubHook = func(
		point intentpub.CrashPoint, rooted *os.Root, entry *intentpub.Entry,
	) error {
		if point != intentpub.PointBeforeEntryCAS || entry == nil || changed {
			return nil
		}
		changed = true
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
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	prepareIntentpubHook = old
	if !changed || code != 5 {
		t.Fatalf("rolled-back publication = exit %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := prepareS4Report(t, stdout)
	if report.Outcome != "rolled-back" {
		t.Fatalf("rolled-back publication outcome = %q\n%s", report.Outcome, stdout)
	}
	return root, slug, statusPreimage
}

type pibRowRecoveryObservation struct {
	root           string
	recoveryCode   int
	recoveryStdout string
	recoveryReport preparePublishReport
	statusAfter    map[string]any
}

// pibRowInterruptedThenRecover interrupts a publication after the journal is
// durable and then runs the recovering invocation, optionally with an
// unwritable `FEATURES.md`.
func pibRowInterruptedThenRecover(t *testing.T, breakIndex bool) pibRowRecoveryObservation {
	t.Helper()
	label := "writable"
	if breakIndex {
		label = "unwritable"
	}
	root, slug := prepareS4Workspace(t, "PIB recovery "+label)
	old := prepareIntentpubHook
	t.Cleanup(func() { prepareIntentpubHook = old })
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
		if point == intentpub.PointAfterJournalDurable {
			return errors.New("injected interruption after the journal became durable")
		}
		return nil
	}
	code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	prepareIntentpubHook = old
	if code != 6 {
		t.Fatalf("interrupted publication = exit %d\n%s", code, stdout)
	}
	if _, err := os.Stat(pibRowLaneJournal(root, slug)); err != nil {
		t.Fatalf("interrupted publication retained no journal: %v", err)
	}
	if breakIndex {
		features := filepath.Join(root, ".tpatch", "FEATURES.md")
		if err := os.RemoveAll(features); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(features, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	recoveryCode, recoveryOut, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	return pibRowRecoveryObservation{
		root:           root,
		recoveryCode:   recoveryCode,
		recoveryStdout: recoveryOut,
		recoveryReport: prepareS4Report(t, recoveryOut),
		statusAfter:    pibRowStatusDocument(t, root, slug),
	}
}

// pibRowTreeSnapshot maps every repo-relative regular file to its digest.
func pibRowTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = pibRowFileDigest(t, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func pibRowChangedPaths(before, after map[string]string) []string {
	changed := []string{}
	for path, digest := range after {
		if previous, existed := before[path]; !existed || previous != digest {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, kept := after[path]; !kept {
			changed = append(changed, path)
		}
	}
	return changed
}

// pibRowExcludeLane drops the transaction scratch lane from an observed change
// set. `.tpatch/local/` is evidence, not a publication write target.
func pibRowExcludeLane(paths []string) []string {
	kept := []string{}
	for _, path := range paths {
		if strings.HasPrefix(path, ".tpatch/local/") {
			continue
		}
		kept = append(kept, path)
	}
	return kept
}
