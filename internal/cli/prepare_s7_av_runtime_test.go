//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	_ "unsafe"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

//go:linkname s7AVAfterPurgeBlobRevalidate github.com/tesseracode/tesserapatch/internal/store.afterPurgeBlobRevalidate
var s7AVAfterPurgeBlobRevalidate func(string)

// s7AVRestoreSeam registers a package-global seam restore with t.Cleanup the
// moment the seam is installed, and returns an idempotent restore the caller
// runs explicitly before any later phase that must observe production
// behaviour. A t.Fatal between install and that explicit call therefore cannot
// leak an installed seam into another test.
func s7AVRestoreSeam(t *testing.T, restore func()) func() {
	t.Helper()
	restored := false
	once := func() {
		if restored {
			return
		}
		restored = true
		restore()
	}
	t.Cleanup(once)
	return once
}

// ─── PIB-547 ──────────────────────────────────────────────────────────────────

// s7AVForbiddenCommandTokens is §10.7's closed forbidden set. `find` is not in
// that set and is caught by the allowlist instead, which is the point of rule 2.
var s7AVForbiddenCommandTokens = []string{
	"cp", "git", "readlink", "mv", "rsync", "tar", "ln", "install", "dd", "chmod",
}

// s7AVCommandArgv0Allowlist is §10.7 rule 2 over the corrupt-object and
// divergent-evidence surfaces.
var s7AVCommandArgv0Allowlist = map[string]bool{"tpatch": true, "rm": true}

// s7AVGitHistoryCaveat is §9.6.2's mandatory caveat, carried as a **must-pass**
// fixture: rev-12's prose-substring rule failed exactly this sentence.
const s7AVGitHistoryCaveat = "it is still in this repository's Git history"

// s7AVPrintedProcedure is one emitted removal procedure: the destructive
// warning, the single removal command line, every other structural command line
// the surface emits, and the surrounding prose.
type s7AVPrintedProcedure struct {
	label          string
	block          string
	warning        string
	removeCommand  string
	commandLines   []string
	blobRel        string
	historyCaveats []string
}

// s7AVShellSplit splits a rendered command line into argv, honouring the single
// POSIX quoting form the emitters use.
func s7AVShellSplit(line string) ([]string, error) {
	argv := []string{}
	current := strings.Builder{}
	quoted, started := false, false
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case char == '\'':
			quoted = !quoted
			started = true
		case (char == ' ' || char == '\t') && !quoted:
			if started {
				argv = append(argv, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteByte(char)
			started = true
		}
	}
	if quoted {
		return nil, fmt.Errorf("unterminated quote in %q", line)
	}
	if started {
		argv = append(argv, current.String())
	}
	return argv, nil
}

// s7AVValidatePrintedRemoval is the single validator PIB-547's five real
// observations and its three semantic sensitivity fixtures are all judged by.
func s7AVValidatePrintedRemoval(procedure s7AVPrintedProcedure) error {
	if procedure.removeCommand == "" {
		return fmt.Errorf("%s: no removal command was printed", procedure.label)
	}
	if strings.Contains(procedure.removeCommand, "\n") {
		return fmt.Errorf("%s: the removal is not a single line", procedure.label)
	}
	argv, err := s7AVShellSplit(procedure.removeCommand)
	if err != nil {
		return fmt.Errorf("%s: %w", procedure.label, err)
	}
	if len(argv) == 0 || argv[0] != "rm" {
		return fmt.Errorf("%s: the removal argv is %v, want the type-total `rm -rf --` form", procedure.label, argv)
	}
	if len(argv) < 2 || argv[1] != "-rf" {
		return fmt.Errorf(
			"%s: the removal argv is %v, which is not the type-total recursive-and-forced removal",
			procedure.label, argv,
		)
	}
	if len(argv) < 3 || argv[2] != "--" {
		return fmt.Errorf("%s: the removal argv is %v: the `--` terminator is missing", procedure.label, argv)
	}
	if len(argv) != 4 {
		return fmt.Errorf("%s: the removal argv is %v, which names a second path or an extra word", procedure.label, argv)
	}
	if argv[3] != procedure.blobRel {
		return fmt.Errorf("%s: the removal names %q, want the managed blob path %q", procedure.label, argv[3], procedure.blobRel)
	}
	if strings.ContainsAny(procedure.removeCommand, "*?[") {
		return fmt.Errorf("%s: the removal contains a wildcard: %q", procedure.label, procedure.removeCommand)
	}
	if !strings.HasSuffix(argv[3], ".blob") || path.Dir(argv[3]) == "." ||
		path.Base(path.Dir(argv[3])) != "blobs" {
		return fmt.Errorf("%s: the removal names %q, which is not a managed blob path", procedure.label, argv[3])
	}

	// The warning is explicit, destructive and printed above the command.
	if procedure.warning == "" ||
		!strings.Contains(procedure.warning, "WARNING") ||
		!strings.Contains(procedure.warning, "permanently deletes") {
		return fmt.Errorf("%s: the destructive warning is missing", procedure.label)
	}
	warningIndex := strings.Index(procedure.block, procedure.warning)
	commandIndex := strings.Index(procedure.block, procedure.removeCommand)
	if warningIndex < 0 || commandIndex < 0 || warningIndex >= commandIndex {
		return fmt.Errorf("%s: the destructive warning is not printed above the removal", procedure.label)
	}
	for _, line := range strings.Split(procedure.block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "rm ") && trimmed != procedure.removeCommand {
			return fmt.Errorf("%s: a second removal line was printed: %q", procedure.label, trimmed)
		}
	}

	// §9.6.2's caveat is mandatory and must not fail the guard.
	for _, caveat := range procedure.historyCaveats {
		if !strings.Contains(procedure.block, caveat) {
			return fmt.Errorf("%s: the mandatory Git-history caveat %q is missing", procedure.label, caveat)
		}
	}

	// §10.7 rule 2: every structural command line's argv[0] is allowlisted.
	prose := procedure.block
	for _, line := range append([]string{procedure.removeCommand}, procedure.commandLines...) {
		if line == "" {
			continue
		}
		lineArgv, err := s7AVShellSplit(line)
		if err != nil {
			return fmt.Errorf("%s: %w", procedure.label, err)
		}
		if len(lineArgv) == 0 || !s7AVCommandArgv0Allowlist[lineArgv[0]] {
			return fmt.Errorf("%s: emitted command line %q has a non-allowlisted argv[0]", procedure.label, line)
		}
		prose = strings.ReplaceAll(prose, line, "")
	}
	// A fenced or indented line that looks like an invocation but is not one of
	// the structural command lines is still a command line by §10.7 rule 1.
	for _, line := range strings.Split(prose, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasSuffix(trimmed, ":") {
			continue
		}
		lineArgv, err := s7AVShellSplit(trimmed)
		if err != nil || len(lineArgv) < 2 {
			continue
		}
		if !s7AVLooksLikeInvocation(lineArgv) {
			continue
		}
		if !s7AVCommandArgv0Allowlist[lineArgv[0]] {
			return fmt.Errorf("%s: emitted command line %q has a non-allowlisted argv[0]", procedure.label, trimmed)
		}
	}

	// §10.7 rule 3: prose fails only in command-invocation shape.
	prose = strings.ReplaceAll(prose, procedure.blobRel, "")
	for _, token := range s7AVForbiddenCommandTokens {
		inlineCode := regexp.MustCompile("`\\s*" + regexp.QuoteMeta(token) + "\\b")
		if inlineCode.MatchString(prose) {
			return fmt.Errorf("%s: the prose renders %q as an inline-code command", procedure.label, token)
		}
		adjacent := regexp.MustCompile(
			`(^|[^0-9A-Za-z_./-])` + regexp.QuoteMeta(token) + `[ \t]+(-{1,2}[A-Za-z]|[^\s]*/[^\s]*)`,
		)
		if adjacent.MatchString(prose) {
			return fmt.Errorf("%s: the prose names %q in command-invocation shape", procedure.label, token)
		}
	}
	return nil
}

// s7AVLooksLikeInvocation recognises the shape §10.7 rule 1 treats as a command
// line: a lowercase bare word followed by an option- or path-shaped word.
func s7AVLooksLikeInvocation(argv []string) bool {
	head := argv[0]
	if head != strings.ToLower(head) || strings.ContainsAny(head, ".,;:'\"()") {
		return false
	}
	for _, word := range argv[1:] {
		if strings.HasPrefix(word, "-") || strings.Contains(word, "/") {
			return true
		}
	}
	return false
}

// s7AVDivergenceProcedure builds the divergent-blob procedure exactly as the
// confirmed purge printed it.
func s7AVDivergenceProcedure(
	label string,
	report intentArchivePurgeReport,
	human, blobRel string,
) s7AVPrintedProcedure {
	procedure := s7AVPrintedProcedure{
		label:          label,
		block:          human,
		blobRel:        blobRel,
		historyCaveats: []string{s7AVGitHistoryCaveat},
	}
	if report.Divergence != nil {
		procedure.warning = report.Divergence.Warning
		procedure.removeCommand = report.Divergence.RemoveCommand
		if report.Divergence.Retry != "" {
			procedure.commandLines = append(procedure.commandLines, report.Divergence.Retry)
		}
	}
	return procedure
}

func s7AVExtractRemovalLine(text string) (string, string) {
	warning := ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "rm ") {
			return warning, trimmed
		}
		if strings.Contains(trimmed, "WARNING") {
			warning = trimmed
		}
	}
	return warning, ""
}

// s7AVDirectoryWithTwoFiles satisfies PIB-547's directory fixture, which is a
// directory *containing two files* so `rm -rf --` is proved to be type-total
// rather than accidentally succeeding on an empty directory.
func s7AVDirectoryWithTwoFiles(t *testing.T, blobPath string) {
	t.Helper()
	for index, name := range []string{"first.keep", "second.keep"} {
		body := fmt.Sprintf("PIB-547 directory child %d\n", index)
		if err := os.WriteFile(filepath.Join(blobPath, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestS7AVPrintedRemovalProcedureContracts(t *testing.T) {
	kinds := []string{"regular", "symlink", "directory", "fifo", "device-seam"}

	for _, kind := range kinds {
		// ── the divergent-blob procedure, printed by the confirmed purge ──
		fixture := s7AROwnedDivergenceFixture(t)
		s7ARReplaceArchiveBlobKind(t, fixture, kind)
		blobPath := filepath.Join(fixture.root, filepath.FromSlash(fixture.blobRel))
		if kind == "directory" {
			s7AVDirectoryWithTwoFiles(t, blobPath)
		}
		siblingRel, err := store.IntentArchiveBlobRel(fixture.slug, fixture.safeHash)
		if err != nil {
			t.Fatal(err)
		}
		siblingPath := filepath.Join(fixture.root, filepath.FromSlash(siblingRel))
		siblingBefore, err := os.ReadFile(siblingPath)
		if err != nil {
			t.Fatal(err)
		}
		var targetBefore []byte
		if kind == "symlink" {
			targetBefore, err = os.ReadFile(fixture.targetPath)
			if err != nil {
				t.Fatal(err)
			}
		}

		restore := func() {}
		if kind == "device-seam" {
			t.Log("device-node classification uses PIB-560's injected file-kind seam because unprivileged CI cannot mknod")
			restore = s7AVRestoreSeam(t, s7ARInstallDeviceProbe(t, fixture.blobRel))
		}
		code, stdout, stderr := s7APRunFromWorkspace(t, fixture.root, []string{
			"feature", "intent-archive", "purge", fixture.slug,
			"--blob", fixture.hash, "--yes", "--json",
		})
		restore()
		if code != 6 {
			t.Fatalf("PIB-547 %s divergence exit=%d, want 6\nstderr=%q\n%s", kind, code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Divergence == nil || report.Divergence.Kind != "blob" {
			t.Fatalf("PIB-547 %s divergence report = %#v\n%s", kind, report.Divergence, stdout)
		}
		procedure := s7AVDivergenceProcedure(
			"PIB-547 "+kind+"/divergent-blob", report, stderr, fixture.blobRel,
		)
		if err := s7AVValidatePrintedRemoval(procedure); err != nil {
			t.Fatalf("PIB-547 %s divergent-blob procedure rejected: %v\n%s", kind, err, stderr)
		}
		// The message states that preserving the object needs kind-appropriate
		// tooling chosen by the operator, and names none.
		if !strings.Contains(report.Divergence.Warning, "preserve it with tooling appropriate to its type") ||
			!strings.Contains(report.Divergence.Warning, "tpatch does not name a preservation command") {
			t.Fatalf("PIB-547 %s preservation sentence = %q", kind, report.Divergence.Warning)
		}
		for _, interactive := range []string{" -i", " -I", "--interactive"} {
			if strings.Contains(report.Divergence.RemoveCommand, interactive) {
				t.Fatalf("PIB-547 %s removal offers the interactive form %q", kind, interactive)
			}
		}

		// Executed verbatim from the workspace root.
		s7AVExecutePrintedRemoval(t, "PIB-547 "+kind, fixture.root, report.Divergence.RemoveCommand)
		if _, err := os.Lstat(blobPath); !os.IsNotExist(err) {
			t.Fatalf("PIB-547 %s left the managed object in place: %v", kind, err)
		}
		if kind == "symlink" {
			targetAfter, err := os.ReadFile(fixture.targetPath)
			if err != nil || !bytes.Equal(targetBefore, targetAfter) {
				t.Fatalf("PIB-547 symlink removal touched the link target: err=%v", err)
			}
		}
		siblingAfter, err := os.ReadFile(siblingPath)
		if err != nil || !bytes.Equal(siblingBefore, siblingAfter) {
			t.Fatalf("PIB-547 %s removal touched a sibling blob: err=%v", kind, err)
		}

		// ── the corrupt-object procedure, printed by `list` for a hash no
		//    purge transaction owns ──
		corrupt := s7AVCorruptListProcedure(t, kind)
		if err := s7AVValidatePrintedRemoval(corrupt.procedure); err != nil {
			t.Fatalf("PIB-547 %s corrupt-object procedure rejected: %v\n%s", kind, err, corrupt.procedure.block)
		}
		s7AVExecutePrintedRemoval(t, "PIB-547 "+kind+"/corrupt", corrupt.root, corrupt.procedure.removeCommand)
		if _, err := os.Lstat(corrupt.blobPath); !os.IsNotExist(err) {
			t.Fatalf("PIB-547 %s corrupt removal left the managed object: %v", kind, err)
		}
		if kind == "symlink" {
			if _, err := os.Stat(corrupt.targetPath); err != nil {
				t.Fatalf("PIB-547 %s corrupt removal touched the link target: %v", kind, err)
			}
		}
	}

	// The index-divergence form of the same code names no removal command at
	// all and no managed blob path.
	indexFixture := s7AROwnedDivergenceFixture(t)
	indexAbs := filepath.Join(indexFixture.root, filepath.FromSlash(indexFixture.indexRel))
	previousAfterRename := s7APAfterPurgeIndexRename
	s7APAfterPurgeIndexRename = func(string) {
		if err := os.WriteFile(indexAbs, []byte("{broken"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	restoreIndexRename := s7AVRestoreSeam(t, func() { s7APAfterPurgeIndexRename = previousAfterRename })
	indexCode, indexStdout, indexStderr := s7APRunFromWorkspace(t, indexFixture.root, []string{
		"feature", "intent-archive", "purge", indexFixture.slug,
		"--blob", indexFixture.hash, "--yes", "--json",
	})
	// Restored before the sensitivity phase below, which runs no CLI but must
	// not inherit an installed seam either.
	restoreIndexRename()
	indexReport := decodeIntentArchivePurgeReport(t, indexStdout)
	if indexCode != 6 || indexReport.Divergence == nil || indexReport.Divergence.Kind != "index" {
		t.Fatalf("PIB-547 index divergence exit=%d report=%#v\n%s", indexCode, indexReport.Divergence, indexStdout)
	}
	if indexReport.Divergence.RemoveCommand != "" || indexReport.Divergence.Blob != "" {
		t.Fatalf("PIB-547 index divergence named a removal %q / blob %q",
			indexReport.Divergence.RemoveCommand, indexReport.Divergence.Blob)
	}
	for _, surface := range []string{indexStdout, indexStderr} {
		if strings.Contains(surface, indexFixture.blobRel) || strings.Contains(surface, "rm -rf") {
			t.Fatalf("PIB-547 index divergence named a managed blob path or a removal:\n%s", surface)
		}
	}

	// ── semantic sensitivity fixtures, judged by the same validator ──
	baseBlobRel := ".tpatch/features/av-sensitivity/artifacts/intent-archive/blobs/" +
		strings.Repeat("a", 64) + ".blob"
	warning := "WARNING: destructive archive repair. The next command permanently deletes whatever " +
		"object is at the managed blob path, including directory contents, with no undo."
	caveat := "If that blob was ever committed, " + s7AVGitHistoryCaveat +
		"; removing it from history is not something tpatch does."

	fixtures := []struct {
		name      string
		procedure s7AVPrintedProcedure
		wantClass string
	}{
		{
			name: "rev-10-cp-plus-plain-rm-pair",
			procedure: s7AVPrintedProcedure{
				label: "rev-10 cp + rm pair",
				block: warning + "\ncp " + baseBlobRel + " " + baseBlobRel + ".bak\nrm " +
					baseBlobRel + "\n" + caveat,
				warning:        warning,
				removeCommand:  "rm " + baseBlobRel,
				blobRel:        baseBlobRel,
				historyCaveats: []string{s7AVGitHistoryCaveat},
			},
			wantClass: "not the type-total recursive-and-forced removal",
		},
		{
			name: "missing-double-dash-terminator",
			procedure: s7AVPrintedProcedure{
				label:          "missing terminator",
				block:          warning + "\nrm -rf " + baseBlobRel + "\n" + caveat,
				warning:        warning,
				removeCommand:  "rm -rf " + baseBlobRel,
				blobRel:        baseBlobRel,
				historyCaveats: []string{s7AVGitHistoryCaveat},
			},
			wantClass: "the `--` terminator is missing",
		},
		{
			name: "rev-11-preservation-alternatives-in-prose",
			procedure: s7AVPrintedProcedure{
				label: "rev-11 preservation prose",
				block: warning + "\nrm -rf -- " + baseBlobRel + "\n" +
					"Preserve the object first if you want it: cp -R for a directory, " +
					"cp -P for a symlink, git show for a version-controlled original.\n" + caveat,
				warning:        warning,
				removeCommand:  "rm -rf -- " + baseBlobRel,
				blobRel:        baseBlobRel,
				historyCaveats: []string{s7AVGitHistoryCaveat},
			},
			wantClass: "command-invocation shape",
		},
	}
	for _, fixture := range fixtures {
		err := s7AVValidatePrintedRemoval(fixture.procedure)
		if err == nil {
			t.Fatalf("PIB-547 sensitivity fixture %q was accepted by the printed-removal validator", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-547 sensitivity fixture %q: want error class %q, got: %v",
				fixture.name, fixture.wantClass, err)
		}
	}

	// The mandatory §9.6.2 caveat is a must-pass fixture: rev-12's
	// prose-substring rule failed exactly this sentence.
	mustPass := s7AVPrintedProcedure{
		label:          "must-pass caveat",
		block:          warning + "\nrm -rf -- " + baseBlobRel + "\n" + caveat,
		warning:        warning,
		removeCommand:  "rm -rf -- " + baseBlobRel,
		blobRel:        baseBlobRel,
		historyCaveats: []string{s7AVGitHistoryCaveat},
	}
	if err := s7AVValidatePrintedRemoval(mustPass); err != nil {
		t.Fatalf("PIB-547 must-pass caveat fixture was rejected: %v", err)
	}
}

// s7AVExecutePrintedRemoval runs the printed line verbatim from the workspace
// root, as an operator would.
func s7AVExecutePrintedRemoval(t *testing.T, label, root, command string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s printed removal %q failed: %v\n%s", label, command, err, output)
	}
	if len(bytes.TrimSpace(output)) != 0 {
		t.Fatalf("%s printed removal %q wrote output: %s", label, command, output)
	}
}

type s7AVCorruptListObservation struct {
	root       string
	slug       string
	blobPath   string
	targetPath string
	procedure  s7AVPrintedProcedure
}

// s7AVCorruptListProcedure builds a non-owned corrupt-object fixture and returns
// the procedure `list` prints for it.
func s7AVCorruptListProcedure(t *testing.T, kind string) s7AVCorruptListObservation {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	data := []byte("PIB-547 corrupt-object " + kind + "\n")
	retained := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained,
	)
	generation := intentArchiveCLIGeneration(t, slug, retained)
	writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, generation),
		map[string][]byte{retained.ContentSHA256: data})
	blobRel, err := store.IntentArchiveBlobRel(slug, retained.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(root, filepath.FromSlash(blobRel))
	targetPath := filepath.Join(root, "outside-target.keep")
	if err := os.WriteFile(targetPath, []byte("target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s7ARReplaceArchiveBlobKind(t, s7ARDivergenceFixture{
		root: root, slug: slug, hash: retained.ContentSHA256,
		blobRel: blobRel, targetPath: targetPath,
	}, kind)
	if kind == "directory" {
		s7AVDirectoryWithTwoFiles(t, blobPath)
	}

	restore := func() {}
	if kind == "device-seam" {
		restore = s7AVRestoreSeam(t, s7ARInstallDeviceProbe(t, blobRel))
	}
	code, stdout, stderr := s7APRunFromWorkspace(t, root, []string{
		"feature", "intent-archive", "list", slug, "--json",
	})
	restore()
	if code != 3 {
		t.Fatalf("PIB-547 %s corrupt list exit=%d, want 3\nstderr=%q\n%s", kind, code, stderr, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	repair := ""
	for _, generationReport := range report.Generations {
		for _, entry := range generationReport.Entries {
			if entry.Storage == "corrupt" {
				repair = entry.Repair
			}
		}
	}
	if repair == "" {
		t.Fatalf("PIB-547 %s corrupt list printed no repair\n%s", kind, stdout)
	}
	warning, removal := s7AVExtractRemovalLine(repair)
	procedure := s7AVPrintedProcedure{
		label:         "PIB-547 " + kind + "/corrupt-object",
		block:         repair + "\n" + report.HistoryDisclosure,
		warning:       warning,
		removeCommand: removal,
		blobRel:       blobRel,
		// §9.6.2's caveat on this surface is `list`'s shipped history
		// disclosure; the exact rev-13 sentence ships on the divergent-blob
		// surface. Both must pass the tokenizer, which is the property the
		// rev-13 correction is about.
		historyCaveats: []string{"Git history"},
	}
	return s7AVCorruptListObservation{
		root: root, slug: slug, blobPath: blobPath, targetPath: targetPath,
		procedure: procedure,
	}
}

// ─── PIB-548 ──────────────────────────────────────────────────────────────────

type s7AVRepairSpec struct {
	residues int
	dangling int
	mixed    int
	corrupt  bool
	ready    bool
}

type s7AVRepairArchive struct {
	root        string
	slug        string
	indexRel    string
	residues    []string
	dangling    []string
	mixed       []string
	corrupt     string
	blobRel     map[string]string
	generations map[string]string
}

func (archive s7AVRepairArchive) abs(rel string) string {
	return filepath.Join(archive.root, filepath.FromSlash(rel))
}

func (archive s7AVRepairArchive) tree(t *testing.T) []byte {
	t.Helper()
	return readTree(t, filepath.Join(archive.root, ".tpatch"))
}

func (archive s7AVRepairArchive) indexBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(archive.abs(archive.indexRel))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func s7AVBlobPurgeCommand(slug string, hashes []string) string {
	sorted := append([]string(nil), hashes...)
	sort.Strings(sorted)
	command := "tpatch feature intent-archive purge " + slug
	for _, hash := range sorted {
		command += " --blob " + hash
	}
	return command + " --yes"
}

func s7AVOrphanPurgeCommand(slug string) string {
	return "tpatch feature intent-archive purge " + slug + " --orphans --yes"
}

// s7AVWriteRepairArchive builds one archive holding the requested number of
// instances of each repair class, every instance a real filesystem observation.
func s7AVWriteRepairArchive(t *testing.T, label string, spec s7AVRepairSpec) s7AVRepairArchive {
	t.Helper()
	var root, slug string
	if spec.ready {
		root, slug = prepareS4Workspace(t, "S7 AV PIB 548 "+label)
		prepareS4WriteReadyBundle(t, root, slug, true)
	} else {
		root, slug = intentArchiveCLIWorkspace(t)
	}
	archive := s7AVRepairArchive{
		root:        root,
		slug:        slug,
		blobRel:     map[string]string{},
		generations: map[string]string{},
	}
	generations := []store.IntentArchiveGeneration{}
	blobs := map[string][]byte{}
	record := func(hash string) {
		rel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		archive.blobRel[hash] = rel
	}

	for index := 0; index < spec.residues; index++ {
		data := []byte(fmt.Sprintf("PIB-548 %s residue %d\n", label, index))
		replacement := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireTombstoned,
		)
		generation := intentArchiveCLIGeneration(t, slug, replacement)
		generations = append(generations, generation)
		blobs[replacement.ContentSHA256] = data
		archive.residues = append(archive.residues, replacement.ContentSHA256)
		archive.generations[replacement.ContentSHA256] = generation.GenerationID
		record(replacement.ContentSHA256)
	}
	for index := 0; index < spec.dangling; index++ {
		data := []byte(fmt.Sprintf("PIB-548 %s dangling %d\n", label, index))
		replacement := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained,
		)
		generation := intentArchiveCLIGeneration(t, slug, replacement)
		generations = append(generations, generation)
		archive.dangling = append(archive.dangling, replacement.ContentSHA256)
		archive.generations[replacement.ContentSHA256] = generation.GenerationID
		record(replacement.ContentSHA256)
	}
	for index := 0; index < spec.mixed; index++ {
		data := []byte(fmt.Sprintf("PIB-548 %s mixed %d\n", label, index))
		retained := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactExploration, data, store.IntentArchiveWireRetained,
		)
		tombstoned := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireTombstoned,
		)
		retainedGen := intentArchiveCLIGeneration(t, slug, retained)
		tombstonedGen := intentArchiveCLIGeneration(t, slug, tombstoned)
		generations = append(generations, retainedGen, tombstonedGen)
		blobs[retained.ContentSHA256] = data
		archive.mixed = append(archive.mixed, retained.ContentSHA256)
		archive.generations[retained.ContentSHA256] = retainedGen.GenerationID
		record(retained.ContentSHA256)
	}
	if spec.corrupt {
		data := []byte("PIB-548 " + label + " corrupt object\n")
		replacement := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysisSidecar, data, store.IntentArchiveWireRetained,
		)
		generation := intentArchiveCLIGeneration(t, slug, replacement)
		generations = append(generations, generation)
		archive.corrupt = replacement.ContentSHA256
		archive.generations[replacement.ContentSHA256] = generation.GenerationID
		record(replacement.ContentSHA256)
	}

	writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, generations...), blobs)
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	archive.indexRel = indexRel
	if spec.corrupt {
		corruptPath := archive.abs(archive.blobRel[archive.corrupt])
		if err := os.Mkdir(corruptPath, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(corruptPath, "child.keep"), []byte("child\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return archive
}

// s7AVListRepairs collects every repair string `list` renders, keyed by the
// storage token of the observation carrying it.
// wantExit is the shipped `list` exit for the archive's class set: unreferenced
// residue alone is an exit-0 observation, every other class is exit 3.
func s7AVListRepairs(
	t *testing.T,
	archive s7AVRepairArchive,
	wantExit int,
) (map[string][]string, intentArchiveListReport) {
	t.Helper()
	code, stdout, _, _ := runPrepare(t,
		"--path", archive.root, "feature", "intent-archive", "list", archive.slug, "--json", "--quiet",
	)
	if code != wantExit {
		t.Fatalf("PIB-548 list exit=%d, want %d\n%s", code, wantExit, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	repairs := map[string][]string{}
	for _, generation := range report.Generations {
		for _, entry := range generation.Entries {
			if entry.Repair == "" {
				continue
			}
			repairs[entry.Storage] = append(repairs[entry.Storage], entry.Repair)
		}
	}
	for _, orphan := range report.Orphans {
		repairs["orphan"] = append(repairs["orphan"], orphan.Repair)
	}
	return repairs, report
}

func s7AVDoctorClassFinding(
	t *testing.T,
	archive s7AVRepairArchive,
	class store.IntentArchiveRepairClass,
) workflow.DoctorFinding {
	t.Helper()
	code, stdout, stderr, _ := runPrepare(t,
		"--path", archive.root, "doctor", "--json", "--check", "D9",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-548 doctor exit=%d stderr=%q\n%s", code, stderr, stdout)
	}
	var report workflow.DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("PIB-548 doctor decode: %v\n%s", err, stdout)
	}
	matches := []workflow.DoctorFinding{}
	for _, finding := range report.Findings {
		if finding.CheckID == "D9" && finding.Tag == string(class) {
			matches = append(matches, finding)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("PIB-548 doctor emitted %d findings for class %s, want exactly 1 (one class, not one per instance)\n%s",
			len(matches), class, stdout)
	}
	return matches[0]
}

func s7AVAssertSameRepair(t *testing.T, label string, repairs []string, want string, instances int) {
	t.Helper()
	if len(repairs) != instances {
		t.Fatalf("%s rendered %d repairs, want one per observed instance (%d)", label, len(repairs), instances)
	}
	for _, repair := range repairs {
		if repair != want {
			t.Fatalf("%s repair = %q, want the single class repair %q", label, repair, want)
		}
		if strings.Count(repair, "tpatch feature intent-archive purge") != 1 {
			t.Fatalf("%s implies more than one invocation: %q", label, repair)
		}
	}
}

func TestS7AVMultiInstanceRepairClassContracts(t *testing.T) {
	// (a) three globally unreferenced tombstone-beside-blob residues.
	residue := s7AVWriteRepairArchive(t, "residues", s7AVRepairSpec{residues: 3, ready: true})
	residueRepairs, residueReport := s7AVListRepairs(t, residue, 0)
	if len(residueReport.Orphans) != 3 {
		t.Fatalf("PIB-548 (a) list rendered %d orphans, want 3", len(residueReport.Orphans))
	}
	s7AVAssertSameRepair(t, "PIB-548 (a) orphan", residueRepairs["orphan"],
		s7AVOrphanPurgeCommand(residue.slug), 6)
	residueFinding := s7AVDoctorClassFinding(t, residue, store.IntentArchiveRepairUnreferencedResidue)
	if residueFinding.Remediation != s7AVOrphanPurgeCommand(residue.slug) {
		t.Fatalf("PIB-548 (a) doctor remediation = %q", residueFinding.Remediation)
	}
	for _, hash := range residue.residues {
		if !strings.Contains(residueFinding.Message, residue.blobRel[hash]) {
			t.Fatalf("PIB-548 (a) doctor omitted instance %s\n%s", hash, residueFinding.Message)
		}
	}
	residueIndexBefore := residue.indexBytes(t)
	code, stdout, stderr, _ := runPrepare(t,
		s7ASPurgeArgs(residue.root, residue.slug, []string{"--orphans"}, true, true, true)...,
	)
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-548 (a) orphans exit=%d stderr=%q\n%s", code, stderr, stdout)
	}
	for _, hash := range residue.residues {
		if _, err := os.Stat(residue.abs(residue.blobRel[hash])); !os.IsNotExist(err) {
			t.Fatalf("PIB-548 (a) left residue %s in place: %v", hash, err)
		}
	}
	if !bytes.Equal(residueIndexBefore, residue.indexBytes(t)) {
		t.Fatal("PIB-548 (a) rewrote index.json while clearing unreferenced residue")
	}
	if report := decodeIntentArchivePurgeReport(t, stdout); report.RemainingRepairs != nil {
		t.Fatalf("PIB-548 (a) left repairs outstanding: %#v", report.RemainingRepairs)
	}

	// (b) two dangling retained hashes: one invocation, zero removals.
	dangling := s7AVWriteRepairArchive(t, "dangling", s7AVRepairSpec{dangling: 2, ready: true})
	danglingRepair := s7AVBlobPurgeCommand(dangling.slug, dangling.dangling)
	danglingRepairs, _ := s7AVListRepairs(t, dangling, 3)
	s7AVAssertSameRepair(t, "PIB-548 (b) dangling", danglingRepairs["dangling"], danglingRepair, 2)
	danglingFinding := s7AVDoctorClassFinding(t, dangling, store.IntentArchiveRepairDanglingReference)
	if danglingFinding.Remediation != danglingRepair {
		t.Fatalf("PIB-548 (b) doctor remediation = %q, want %q", danglingFinding.Remediation, danglingRepair)
	}
	danglingRemovals := s7AVRunPurgeWithRemoveSpy(t, dangling.root, dangling.slug,
		s7AVBlobSelector(dangling.dangling), 0, "PIB-548 (b)")
	if len(danglingRemovals) != 0 {
		t.Fatalf("PIB-548 (b) removed %v, want zero removals", danglingRemovals)
	}
	s7AVAssertArchiveRepaired(t, dangling, "PIB-548 (b)")

	// (c) two mixed tombstone/live-reference hashes: one invocation, two removals.
	mixed := s7AVWriteRepairArchive(t, "mixed", s7AVRepairSpec{mixed: 2, ready: true})
	mixedRepair := s7AVBlobPurgeCommand(mixed.slug, mixed.mixed)
	mixedRepairs, _ := s7AVListRepairs(t, mixed, 3)
	s7AVAssertSameRepair(t, "PIB-548 (c) mixed", mixedRepairs["mixed-reference"], mixedRepair, 2)
	mixedFinding := s7AVDoctorClassFinding(t, mixed, store.IntentArchiveRepairMixedReference)
	if mixedFinding.Remediation != mixedRepair {
		t.Fatalf("PIB-548 (c) doctor remediation = %q, want %q", mixedFinding.Remediation, mixedRepair)
	}
	mixedRemovals := s7AVRunPurgeWithRemoveSpy(t, mixed.root, mixed.slug,
		s7AVBlobSelector(mixed.mixed), 0, "PIB-548 (c)")
	if len(mixedRemovals) != 2 {
		t.Fatalf("PIB-548 (c) removed %v, want both mixed blobs", mixedRemovals)
	}
	s7AVAssertArchiveRepaired(t, mixed, "PIB-548 (c)")

	// (d) two classes at once, asserted at the rev-12 sequential outcome.
	sequential := s7AVWriteRepairArchive(t, "sequential", s7AVRepairSpec{residues: 3, mixed: 2, ready: true})
	sequentialResidueRepair := s7AVOrphanPurgeCommand(sequential.slug)
	sequentialMixedRepair := s7AVBlobPurgeCommand(sequential.slug, sequential.mixed)
	for _, refusal := range []struct {
		name string
		args []string
	}{
		{name: "generation", args: []string{"--generation", sequential.generations[sequential.residues[0]]}},
		{name: "all", args: []string{"--all"}},
		{name: "partial-blob", args: []string{"--blob", sequential.mixed[0]}},
	} {
		before := sequential.tree(t)
		refusalCode, refusalStdout, _, _ := runPrepare(t,
			s7ASPurgeArgs(sequential.root, sequential.slug, refusal.args, true, true, true)...,
		)
		if refusalCode != 3 {
			t.Fatalf("PIB-548 (d) %s exit=%d, want 3\n%s", refusal.name, refusalCode, refusalStdout)
		}
		if !bytes.Equal(before, sequential.tree(t)) {
			t.Fatalf("PIB-548 (d) %s was not zero-write", refusal.name)
		}
		refusalReport := decodeIntentArchivePurgeReport(t, refusalStdout)
		s7AVAssertStages(t, "PIB-548 (d) "+refusal.name, refusalReport.RemainingRepairs,
			[]string{"mixed-reference", "unreferenced-residue"},
			[]string{sequentialMixedRepair, sequentialResidueRepair})
	}
	sequentialIndexBefore := sequential.indexBytes(t)
	mixedBlobsBefore := map[string][]byte{}
	for _, hash := range sequential.mixed {
		raw, err := os.ReadFile(sequential.abs(sequential.blobRel[hash]))
		if err != nil {
			t.Fatal(err)
		}
		mixedBlobsBefore[hash] = raw
	}
	admittedCode, admittedStdout, admittedStderr, _ := runPrepare(t,
		s7ASPurgeArgs(sequential.root, sequential.slug, []string{"--orphans"}, true, true, true)...,
	)
	if admittedCode != 0 || admittedStderr != "" {
		t.Fatalf("PIB-548 (d) orphans exit=%d stderr=%q\n%s", admittedCode, admittedStderr, admittedStdout)
	}
	admitted := decodeIntentArchivePurgeReport(t, admittedStdout)
	if admitted.Outcome != string(store.IntentArchivePurgePurged) || admitted.Action != "none" {
		t.Fatalf("PIB-548 (d) admitted outcome/action = %q/%q", admitted.Outcome, admitted.Action)
	}
	for _, hash := range sequential.residues {
		if _, err := os.Stat(sequential.abs(sequential.blobRel[hash])); !os.IsNotExist(err) {
			t.Fatalf("PIB-548 (d) left residue %s: %v", hash, err)
		}
	}
	if !bytes.Equal(sequentialIndexBefore, sequential.indexBytes(t)) {
		t.Fatal("PIB-548 (d) rewrote index.json for the residue class")
	}
	for hash, before := range mixedBlobsBefore {
		after, err := os.ReadFile(sequential.abs(sequential.blobRel[hash]))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("PIB-548 (d) touched mixed hash %s: err=%v", hash, err)
		}
	}
	if admitted.RemainingRepairs == nil ||
		admitted.RemainingRepairs.RepairedClass != string(store.IntentArchiveRepairUnreferencedResidue) ||
		admitted.RemainingRepairs.StagesRemaining != 1 {
		t.Fatalf("PIB-548 (d) remaining repairs = %#v\n%s", admitted.RemainingRepairs, admittedStdout)
	}
	s7AVAssertStages(t, "PIB-548 (d) admitted", admitted.RemainingRepairs,
		[]string{"mixed-reference"}, []string{sequentialMixedRepair})
	rerunCode, rerunStdout, rerunStderr, _ := runPrepare(t,
		s7ASPurgeArgs(sequential.root, sequential.slug, s7AVBlobSelector(sequential.mixed), true, true, true)...,
	)
	if rerunCode != 0 || rerunStderr != "" {
		t.Fatalf("PIB-548 (d) rerun exit=%d stderr=%q\n%s", rerunCode, rerunStderr, rerunStdout)
	}
	rerun := decodeIntentArchivePurgeReport(t, rerunStdout)
	if rerun.RemainingRepairs != nil {
		t.Fatalf("PIB-548 (d) rerun left repairs outstanding: %#v", rerun.RemainingRepairs)
	}
	s7AVAssertArchiveRepaired(t, sequential, "PIB-548 (d)")

	// (e) the same archive plus one corrupt-object instance: rank 1 blocks
	// every confirmed selector, `--orphans --yes` included.
	blocked := s7AVWriteRepairArchive(t, "blocked",
		s7AVRepairSpec{residues: 3, mixed: 2, corrupt: true, ready: true})
	for _, selector := range []struct {
		name string
		args []string
	}{
		{name: "orphans", args: []string{"--orphans"}},
		{name: "blob-mixed-class", args: s7AVBlobSelector(blocked.mixed)},
		{name: "blob-partial", args: []string{"--blob", blocked.mixed[0]}},
		{name: "generation", args: []string{"--generation", blocked.generations[blocked.residues[0]]}},
		{name: "all", args: []string{"--all"}},
	} {
		before := blocked.tree(t)
		blockedCode, blockedStdout, _, _ := runPrepare(t,
			s7ASPurgeArgs(blocked.root, blocked.slug, selector.args, true, true, true)...,
		)
		if blockedCode != 3 {
			t.Fatalf("PIB-548 (e) %s exit=%d, want 3\n%s", selector.name, blockedCode, blockedStdout)
		}
		if !bytes.Equal(before, blocked.tree(t)) {
			t.Fatalf("PIB-548 (e) %s was not zero-write", selector.name)
		}
		blockedReport := decodeIntentArchivePurgeReport(t, blockedStdout)
		if blockedReport.Refusal == nil ||
			blockedReport.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) {
			t.Fatalf("PIB-548 (e) %s refusal = %#v", selector.name, blockedReport.Refusal)
		}
		if blockedReport.RemainingRepairs == nil ||
			blockedReport.RemainingRepairs.NextStage == nil ||
			blockedReport.RemainingRepairs.NextStage.Kind != string(store.IntentArchiveRepairStageManual) ||
			blockedReport.RemainingRepairs.NextStage.Class != string(store.IntentArchiveRepairCorruptObject) {
			t.Fatalf("PIB-548 (e) %s next stage = %#v", selector.name, blockedReport.RemainingRepairs)
		}
	}
}

func s7AVBlobSelector(hashes []string) []string {
	sorted := append([]string(nil), hashes...)
	sort.Strings(sorted)
	args := []string{}
	for _, hash := range sorted {
		args = append(args, "--blob", hash)
	}
	return args
}

// s7AVRunPurgeWithRemoveSpy runs one confirmed purge and reports exactly which
// blob paths the store removed.
func s7AVRunPurgeWithRemoveSpy(
	t *testing.T,
	root, slug string,
	selector []string,
	wantExit int,
	label string,
) []string {
	t.Helper()
	removed := []string{}
	previous := intentArchiveNewStorage
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &s7ASRemoveSpyStorage{
			IntentArchiveStorage: previous(authority, rootFS),
			removed:              &removed,
		}
	}
	restoreStorage := s7AVRestoreSeam(t, func() { intentArchiveNewStorage = previous })
	code, stdout, stderr, _ := runPrepare(t, s7ASPurgeArgs(root, slug, selector, true, true, true)...)
	// The spy is uninstalled before this helper returns, so every later phase
	// of the caller runs against the shipped storage constructor.
	restoreStorage()
	if code != wantExit {
		t.Fatalf("%s purge exit=%d, want %d stderr=%q\n%s", label, code, wantExit, stderr, stdout)
	}
	report := decodeIntentArchivePurgeReport(t, stdout)
	if wantExit == 0 && report.Refusal != nil {
		t.Fatalf("%s purge refused: %#v", label, report.Refusal)
	}
	return removed
}

// s7AVAssertArchiveRepaired proves X11 is satisfied afterwards and an ordinary
// mutating prepare proceeds.
func s7AVAssertArchiveRepaired(t *testing.T, archive s7AVRepairArchive, label string) {
	t.Helper()
	code, stdout, _, _ := runPrepare(t,
		"--path", archive.root, "feature", "intent-archive", "list", archive.slug, "--json", "--quiet",
	)
	if code != 0 {
		t.Fatalf("%s archive still refuses list: exit=%d\n%s", label, code, stdout)
	}
	prepareCode, prepareStdout, prepareStderr, _ := runPrepare(t,
		"--path", archive.root, "prepare", archive.slug, "--json", "--quiet",
	)
	if prepareCode != 0 || prepareStderr != "" {
		t.Fatalf("%s ordinary mutation did not proceed: exit=%d stderr=%q\n%s",
			label, prepareCode, prepareStderr, prepareStdout)
	}
}

func s7AVAssertStages(
	t *testing.T,
	label string,
	remaining *intentArchiveRemainingRepairsReport,
	wantClasses, wantRepairs []string,
) {
	t.Helper()
	if remaining == nil {
		t.Fatalf("%s carried no remaining_repairs object", label)
	}
	if !remaining.RerunRequired || remaining.StagesRemaining != len(wantClasses) ||
		len(remaining.Stages) != len(wantClasses) {
		t.Fatalf("%s remaining repairs = %#v, want %d stage(s)", label, remaining, len(wantClasses))
	}
	if remaining.NextStage == nil || remaining.NextStage.Ordinal != 1 ||
		remaining.NextStage.Class != wantClasses[0] {
		t.Fatalf("%s next stage = %#v, want ordinal 1 class %q", label, remaining.NextStage, wantClasses[0])
	}
	for index, stage := range remaining.Stages {
		if stage.Ordinal != index+1 {
			t.Fatalf("%s stage %d carries ordinal %d", label, index, stage.Ordinal)
		}
		if stage.Class != wantClasses[index] {
			t.Fatalf("%s stage %d class = %q, want %q", label, index, stage.Class, wantClasses[index])
		}
		if stage.Repair != wantRepairs[index] {
			t.Fatalf("%s stage %d repair = %q, want %q", label, index, stage.Repair, wantRepairs[index])
		}
		if stage.RepairCWD != store.IntentArchiveRepairCWD {
			t.Fatalf("%s stage %d repair_cwd = %q", label, index, stage.RepairCWD)
		}
	}
}

// ─── PIB-550 ──────────────────────────────────────────────────────────────────

// s7AVClosureClaims are the affirmative sentences that would claim the
// revalidate→unlink window closed, conditioned or wholly detected. The
// disclosure's own denial of the residual probe→unlink gap is deliberately not
// in this set: the guard fails a *claim*, not a denial.
var s7AVClosureClaims = []string{
	"the removal is conditioned on the revalidated content",
	"the unlink is conditioned on the revalidated content",
	"the revalidate→unlink window is closed",
	"this window is closed",
	"removes only the validated bytes",
	"the replacement is always detected",
}

// s7AVDisclosureAnchor is one residual the disclosure must render. Each anchor
// must occur **exactly once** in its document, and the anchors of a document
// must appear in the listed order, so "disclosed beside, never in place of"
// is asserted positionally rather than by mere presence.
type s7AVDisclosureAnchor struct {
	label string
	text  string
}

// s7AVDisclosureDocument is one document under PIB-550's disclosure contract:
// the ordered residual anchors, the sentence that must keep denying closure,
// and the retired rev-11 readings that the rev-18 erratum replaced and that
// must never reappear.
type s7AVDisclosureDocument struct {
	label    string
	body     string
	anchors  []s7AVDisclosureAnchor
	mustDeny string
	retired  []string
}

// s7AVNormativeProse drops the two line kinds that quote a closure claim in
// order to reject or to specify it rather than to assert it: an
// alternatives-table row whose subject begins "Claiming …", and an
// acceptance-matrix row, which is a test specification rather than a shipped or
// normative sentence.
func s7AVNormativeProse(document string) string {
	kept := []string{}
	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| Claiming ") || strings.HasPrefix(trimmed, "| PIB-") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// s7AVAssertsClaim reports whether a closure phrase is *asserted* rather than
// denied. A denial negates inside its own sentence — "No shipped message claims
// this window is closed" — so the sentence containing the occurrence is the
// scope the negation is looked for in.
func s7AVAssertsClaim(text, claim string) bool {
	lowered := strings.ToLower(text)
	needle := strings.ToLower(claim)
	offset := 0
	for {
		index := strings.Index(lowered[offset:], needle)
		if index < 0 {
			return false
		}
		at := offset + index
		start := strings.LastIndexAny(lowered[:at], ".!?|\n")
		sentence := lowered[start+1 : at]
		if !s7AVNegated(sentence) {
			return true
		}
		offset = at + len(needle)
	}
}

func s7AVNegated(sentence string) bool {
	for _, marker := range []string{
		"no ", "not ", "never", "nothing", "nor ", "unimplementable", "rather than",
	} {
		if strings.Contains(sentence, marker) {
			return true
		}
	}
	return false
}

// s7AVValidateWindowDisclosure is the single validator for PIB-550's guard half
// and for its sensitivity fixtures.
func s7AVValidateWindowDisclosure(
	documents []s7AVDisclosureDocument,
	shipped map[string]string,
) error {
	for _, document := range documents {
		previous := -1
		for _, anchor := range document.anchors {
			occurrences := strings.Count(document.body, anchor.text)
			if occurrences != 1 {
				return fmt.Errorf(
					"%s: the %s residual is disclosed %d times, want exactly once",
					document.label, anchor.label, occurrences,
				)
			}
			at := strings.Index(document.body, anchor.text)
			if at <= previous {
				return fmt.Errorf(
					"%s: the %s residual is rendered out of order at %d (previous anchor at %d)",
					document.label, anchor.label, at, previous,
				)
			}
			previous = at
		}
		for _, phrase := range document.retired {
			if strings.Contains(document.body, phrase) {
				return fmt.Errorf(
					"%s: the retired undetected-removal reading is back: %q", document.label, phrase,
				)
			}
		}
		if strings.Count(document.body, document.mustDeny) != 1 {
			return fmt.Errorf(
				"%s: the disclosure no longer denies that the residual gap is closed", document.label,
			)
		}
		normative := s7AVNormativeProse(document.body)
		for _, claim := range s7AVClosureClaims {
			if s7AVAssertsClaim(normative, claim) {
				return fmt.Errorf("%s: the document claims the window is closed: %q", document.label, claim)
			}
		}
	}
	labels := make([]string, 0, len(shipped))
	for label := range shipped {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		for _, claim := range s7AVClosureClaims {
			if s7AVAssertsClaim(shipped[label], claim) {
				return fmt.Errorf("the shipped %s surface claims the window is closed: %q", label, claim)
			}
		}
	}
	return nil
}

// s7AVWindowDisclosureDocuments states the rev-18 disclosure both documents
// carry: the window itself, the residual probe→unlink gap the identity
// re-probe narrows the window to, and the post-CAS final-syscall race the gap
// is rendered beside — in that order, each exactly once.
func s7AVWindowDisclosureDocuments(prd, adr string) []s7AVDisclosureDocument {
	return []s7AVDisclosureDocument{
		{
			label: "PRD §9.7.2",
			body:  prd,
			anchors: []s7AVDisclosureAnchor{
				{
					label: "revalidate→unlink window",
					text:  "**The file at `blobs/h.blob` is replaced between step 2's revalidation and step 3's unlink**",
				},
				{
					label: "probe→unlink gap",
					text:  "**What stays open is the gap between that re-probe and the unlink syscall**",
				},
				{
					label: "post-CAS final-syscall race",
					text:  "A write that lands inside the CAS→rename final syscall window is **not** detected",
				},
			},
			mustDeny: "**The probe→unlink gap is not detected, and is not claimed to be.**",
			retired: []string{
				"**Not detected, and not claimed to be.**",
				"Step 3 removes whatever object is at that path",
			},
		},
		{
			label: "ADR-035",
			body:  adr,
			anchors: []s7AVDisclosureAnchor{
				{
					label: "revalidate→unlink window",
					text:  "A replacement of the object at `blobs/<h>.blob` landing between the",
				},
				{
					label: "probe→unlink gap",
					text:  "**the gap between that identity re-probe and the unlink syscall**",
				},
				{
					label: "post-CAS final-syscall race",
					text:  "**a write inside the final CAS→rename syscall window**",
				},
			},
			mustDeny: "**both gaps are disclosed here as residuals this ADR does not claim to close**",
			retired: []string{
				"which the unlink cannot be conditioned on",
				"so the replacement is what gets removed",
			},
		},
	}
}

// s7AVRunReplacementWindow drives the revalidate→unlink window: the external
// writer replaces the managed object at `beforePurgeBlobRemove`, which fires
// after `afterPurgeBlobRevalidate` has already passed.
func s7AVRunReplacementWindow(
	t *testing.T,
	kind string,
) (s7ARDivergenceFixture, int, string, string, int) {
	t.Helper()
	fixture := s7AROwnedDivergenceFixture(t)
	blobPath := filepath.Join(fixture.root, filepath.FromSlash(fixture.blobRel))
	revalidated := 0
	fired := 0
	previousRevalidate := s7AVAfterPurgeBlobRevalidate
	previousRemove := s7APBeforePurgeBlobRemove
	s7AVAfterPurgeBlobRevalidate = func(rel string) {
		if rel == fixture.blobRel {
			revalidated++
		}
	}
	restoreRevalidate := s7AVRestoreSeam(t, func() { s7AVAfterPurgeBlobRevalidate = previousRevalidate })
	s7APBeforePurgeBlobRemove = func(rel string) {
		if rel != fixture.blobRel || revalidated == 0 {
			return
		}
		fired++
		if err := os.RemoveAll(blobPath); err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "regular":
			if err := os.WriteFile(blobPath, []byte("PIB-550 external replacement bytes\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		case "directory":
			if err := os.Mkdir(blobPath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(blobPath, "child.keep"), []byte("PIB-550 replacement child\n"), 0o600,
			); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unknown replacement kind %q", kind)
		}
	}
	restoreRemove := s7AVRestoreSeam(t, func() { s7APBeforePurgeBlobRemove = previousRemove })
	code, stdout, stderr := s7APRunFromWorkspace(t, fixture.root, []string{
		"feature", "intent-archive", "purge", fixture.slug, "--orphans", "--yes", "--json",
	})
	// Both seams are restored before the caller's rerun and disclosure phases,
	// which must observe the shipped machine with no injection installed.
	restoreRevalidate()
	restoreRemove()
	if revalidated == 0 || fired == 0 {
		t.Fatalf("PIB-550 %s never reached the revalidate→unlink window (revalidated=%d fired=%d)\n%s",
			kind, revalidated, fired, stdout)
	}
	return fixture, code, stdout, stderr, fired
}

func TestS7AVRevalidateUnlinkWindowContracts(t *testing.T) {
	shipped := map[string]string{}

	for _, kind := range []string{"regular", "directory"} {
		fixture, code, stdout, stderr, fired := s7AVRunReplacementWindow(t, kind)
		shipped[kind+"/purge-stdout"] = stdout
		shipped[kind+"/purge-stderr"] = stderr
		if fired != 1 {
			t.Fatalf("PIB-550 %s fired the window %d times, want exactly 1", kind, fired)
		}
		blobPath := filepath.Join(fixture.root, filepath.FromSlash(fixture.blobRel))

		// The rev-18 erratum's observable, observed rather than assumed:
		// `prepareArchiveStorage.RemoveBlob` re-probes the managed object's
		// identity as the removal's capability check, so a replacement that
		// lands before that probe is refused and the transaction stops
		// consistent and resumable. The residual this row still discloses is
		// the unseamed gap between that probe and `root.Remove`.
		if code != 5 {
			t.Fatalf("PIB-550 %s exit=%d, want 5 archive-purge-partial\nstderr=%q\n%s",
				kind, code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgePartial) ||
			report.PurgeProgress == nil ||
			report.PurgeProgress.PendingHash != fixture.hash ||
			report.PurgeProgress.Resume != string(store.IntentArchiveResumePendingRecoveryThenCompletion) ||
			report.PurgeProgress.State != store.IntentArchivePurgeStateConsistent {
			t.Fatalf("PIB-550 %s progress = %#v\n%s", kind, report.PurgeProgress, stdout)
		}
		if len(report.PurgeProgress.CompletedHashes) != 0 {
			t.Fatalf("PIB-550 %s completed %v while the window was open", kind, report.PurgeProgress.CompletedHashes)
		}

		// The replacement itself survives byte-for-byte: no byte-level loss
		// occurs at this seam.
		switch kind {
		case "regular":
			body, err := os.ReadFile(blobPath)
			if err != nil || string(body) != "PIB-550 external replacement bytes\n" {
				t.Fatalf("PIB-550 regular replacement was destroyed: err=%v body=%q", err, body)
			}
		case "directory":
			entries, err := os.ReadDir(blobPath)
			if err != nil || len(entries) != 1 || entries[0].Name() != "child.keep" {
				t.Fatalf("PIB-550 directory replacement was destroyed: err=%v entries=%v", err, entries)
			}
		}

		// The hash stays owned, so the archive is not left in a silent
		// inconsistency: the rerun routes to the owner's exit-6 evidence
		// divergence, naming the managed blob path and its removal procedure.
		_, index := readIntentArchiveCLIIndex(t, fixture.root, fixture.slug)
		pending := false
		for _, state := range s7ATWireStates(index, fixture.hash) {
			if state == store.IntentArchiveWireRemovalPending {
				pending = true
			}
		}
		if !pending {
			t.Fatalf("PIB-550 %s left no removal-pending reference to the owned hash", kind)
		}
		rerunCode, rerunStdout, rerunStderr := s7APRunFromWorkspace(t, fixture.root, []string{
			"feature", "intent-archive", "purge", fixture.slug, "--orphans", "--yes", "--json",
		})
		shipped[kind+"/rerun-stdout"] = rerunStdout
		shipped[kind+"/rerun-stderr"] = rerunStderr
		rerun := decodeIntentArchivePurgeReport(t, rerunStdout)
		if rerunCode != 6 || rerun.Divergence == nil ||
			rerun.Divergence.Kind != "blob" ||
			rerun.Divergence.PendingHash != fixture.hash ||
			rerun.Divergence.Blob != fixture.blobRel {
			t.Fatalf("PIB-550 %s rerun exit=%d divergence=%#v\nstderr=%q\n%s",
				kind, rerunCode, rerun.Divergence, rerunStderr, rerunStdout)
		}
	}

	// ── the guard half: the disclosure, not the behaviour ──
	prd := s7AVRepoDocument(t, s7AVPRDRelPath)
	adr := s7AVRepoDocument(t, s7AVADRRelPath)
	documents := s7AVWindowDisclosureDocuments(prd, adr)
	if err := s7AVValidateWindowDisclosure(documents, shipped); err != nil {
		t.Fatalf("PIB-550 baseline disclosure validation failed: %v", err)
	}

	// The three residual anchors are unique and ordered inside the validator
	// itself, so "disclosed beside, never in place of" is a property of the
	// validator rather than an assertion this test repeats. The fixtures below
	// prove each of those properties bites.
	prdWindow := documents[0].anchors[0].text
	prdGap := documents[0].anchors[1].text
	prdFinalSyscall := documents[0].anchors[2].text
	adrGap := documents[1].anchors[1].text

	claim := "The removal is conditioned on the revalidated content."
	retiredReading := "**Not detected, and not claimed to be.** " +
		"Step 3 removes whatever object is at that path — the replacement, not the validated bytes."
	fixtures := []struct {
		name      string
		documents []s7AVDisclosureDocument
		shipped   map[string]string
		wantClass string
	}{
		{
			name: "prd-claims-the-window-conditioned",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(prd, documents[0].mustDeny,
					documents[0].mustDeny+" "+claim, 1), adr),
			shipped:   shipped,
			wantClass: "PRD §9.7.2: the document claims the window is closed",
		},
		{
			name: "adr-claims-the-window-conditioned",
			documents: s7AVWindowDisclosureDocuments(prd,
				strings.Replace(adr, adrGap, adrGap+"\n\n"+claim, 1)),
			shipped:   shipped,
			wantClass: "ADR-035: the document claims the window is closed",
		},
		{
			name:      "report-claims-the-window-conditioned",
			documents: documents,
			shipped:   map[string]string{"purge report": "purge-partial. " + claim},
			wantClass: "the shipped purge report surface claims the window is closed",
		},
		{
			name: "disclosure-drops-the-final-syscall-race",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(prd, prdFinalSyscall, "is handled", 1), adr,
			),
			shipped:   shipped,
			wantClass: "the post-CAS final-syscall race residual is disclosed 0 times",
		},
		{
			name: "prd-drops-the-probe-unlink-gap",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(prd, prdGap, "the window is narrowed", 1), adr,
			),
			shipped:   shipped,
			wantClass: "PRD §9.7.2: the probe→unlink gap residual is disclosed 0 times",
		},
		{
			name: "adr-drops-the-probe-unlink-gap",
			documents: s7AVWindowDisclosureDocuments(prd,
				strings.Replace(adr, adrGap, "the narrowed window", 1),
			),
			shipped:   shipped,
			wantClass: "ADR-035: the probe→unlink gap residual is disclosed 0 times",
		},
		{
			name: "residuals-rendered-out-of-order",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(
					strings.Replace(prd, prdFinalSyscall, "the post-CAS residual is stated above", 1),
					"The five windows are enumerated,",
					"The five windows are enumerated ("+prdFinalSyscall+"),", 1,
				), adr,
			),
			shipped:   shipped,
			wantClass: "the post-CAS final-syscall race residual is rendered out of order",
		},
		{
			name: "prd-restores-the-undetected-removal-reading",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(prd, documents[0].mustDeny,
					documents[0].mustDeny+" "+retiredReading, 1), adr),
			shipped:   shipped,
			wantClass: "PRD §9.7.2: the retired undetected-removal reading is back",
		},
		{
			name: "adr-restores-the-unconditionable-unlink-reading",
			documents: s7AVWindowDisclosureDocuments(prd,
				strings.Replace(adr, documents[1].mustDeny,
					"which the unlink cannot be conditioned on, "+documents[1].mustDeny, 1)),
			shipped:   shipped,
			wantClass: "ADR-035: the retired undetected-removal reading is back",
		},
		{
			name: "prd-drops-the-residual-denial",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(prd, documents[0].mustDeny, "", 1), adr),
			shipped:   shipped,
			wantClass: "PRD §9.7.2: the disclosure no longer denies that the residual gap is closed",
		},
		{
			name: "prd-duplicates-the-window-row",
			documents: s7AVWindowDisclosureDocuments(
				strings.Replace(prd, prdWindow, prdWindow+" "+prdWindow, 1), adr),
			shipped:   shipped,
			wantClass: "the revalidate→unlink window residual is disclosed 2 times",
		},
	}
	for _, fixture := range fixtures {
		err := s7AVValidateWindowDisclosure(fixture.documents, fixture.shipped)
		if err == nil {
			t.Fatalf("PIB-550 sensitivity fixture %q was accepted by the disclosure validator", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-550 sensitivity fixture %q: want error class %q, got: %v",
				fixture.name, fixture.wantClass, err)
		}
	}

	if err := s7AVValidateWindowDisclosure(documents, shipped); err != nil {
		t.Fatalf("PIB-550 unmutated disclosure was rejected after sensitivity: %v", err)
	}
}
