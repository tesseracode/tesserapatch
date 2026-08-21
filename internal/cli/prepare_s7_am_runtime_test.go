//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7PIB395RealCLIContenderExitsThreeUnderHeldRoot(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 same-root contender")
	prepareS4WriteReadyBundle(t, root, slug, false)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	holder, closeHolder := s7StartCLIProcessHolder(t, root)
	defer closeHolder()

	output, code := s7RunCLIProcessContender(t, root, slug)
	if code != 3 || !strings.Contains(output, `"code": "transaction-in-progress"`) {
		t.Fatalf("real CLI contender = exit:%d output:%q", code, output)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("contended CLI process changed the workspace")
	}
	if holder.ProcessState != nil {
		t.Fatal("holder exited before explicit release")
	}
}

func TestS7PIB396KilledHolderReleasesAuthorityForTerminalRecovery(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 killed holder recovery")
	oldHook := prepareIntentpubHook
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
		if point == intentpub.PointAfterJournalDurable {
			return fmt.Errorf("S7 stop after journal")
		}
		return nil
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	prepareIntentpubHook = oldHook
	t.Cleanup(func() { prepareIntentpubHook = oldHook })
	report := prepareS4Report(t, stdout)
	if code != 6 || report.Outcome != "recovery-refused" {
		t.Fatalf("journal fixture = %d %#v", code, report)
	}
	journal := filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("pending journal: %v", err)
	}

	holder, _ := s7StartCLIProcessHolder(t, root)
	if err := holder.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = holder.Wait()

	output, recoveredCode := s7RunCLIProcess(t, root, slug)
	if recoveredCode != 0 {
		t.Fatalf("post-death recovery = exit:%d output:%q", recoveredCode, output)
	}
	recovered := prepareS4Report(t, output)
	if recovered.Outcome != "recovered" ||
		recovered.Recovery == nil ||
		recovered.Recovery.Kind != "journal-undo" {
		t.Fatalf("post-death recovery report = %#v", recovered)
	}
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatalf("terminal recovery left journal artifact: %v", err)
	}
}

func TestS7PIB397RealDifferentSlugCLIProcessesSerialize(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 holder slug")
	prepareS4WriteReadyBundle(t, root, slug, false)
	const otherTitle = "S7 other slug"
	if code, _, stderr, _ := runPrepare(t, "--path", root, "add", otherTitle); code != 0 {
		t.Fatalf("add second feature: %s", stderr)
	}
	otherSlug := storeSlug(otherTitle)
	prepareS4WriteReadyBundle(t, root, otherSlug, false)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	_, closeHolder := s7StartCLIProcessHolder(t, root)
	defer closeHolder()

	output, code := s7RunCLIProcessContender(t, root, otherSlug)
	if code != 3 || !strings.Contains(output, `"code": "transaction-in-progress"`) {
		t.Fatalf("cross-slug real CLI contender = exit:%d output:%q", code, output)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("cross-slug contention changed the workspace")
	}
}

func TestS7PIB407AnalyzeAndDefinePartialBundlesAreCleanForD9(t *testing.T) {
	for _, phase := range []string{"analyze", "define"} {
		t.Run(phase, func(t *testing.T) {
			root, slug := prepareS4Workspace(t, "S7 D9 "+phase)
			if code, _, stderr, _ := runPrepare(t, "--path", root, "analyze", slug); code != 0 {
				t.Fatalf("analyze = %d: %s", code, stderr)
			}
			if phase == "define" {
				if code, _, stderr, _ := runPrepare(t, "--path", root, "define", slug); code != 0 {
					t.Fatalf("define = %d: %s", code, stderr)
				}
			}
			lane := filepath.Join(root, filepath.FromSlash(".tpatch/local/intent-prepare/"+slug))
			if _, err := os.Stat(lane); !os.IsNotExist(err) {
				t.Fatalf("ordinary %s created prepare lane: %v", phase, err)
			}
			index := filepath.Join(root, filepath.FromSlash(".tpatch/features/"+slug+"/artifacts/intent-archive/index.json"))
			if _, err := os.Stat(index); !os.IsNotExist(err) {
				t.Fatalf("ordinary %s created archive index: %v", phase, err)
			}
			before := readTree(t, filepath.Join(root, ".tpatch"))
			code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "doctor", "--check", "D9", "--json",
			)
			var report struct {
				Findings []json.RawMessage `json:"findings"`
				Summary  struct {
					Warnings int `json:"warnings"`
					Errors   int `json:"errors"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatalf("decode D9 report: %v\n%s", err, stdout)
			}
			if code != 0 || stderr != "" || len(report.Findings) != 0 ||
				report.Summary.Warnings != 0 || report.Summary.Errors != 0 {
				t.Fatalf("clean %s D9 = code:%d stderr:%q report:%#v", phase, code, stderr, report)
			}
			if strings.Contains(stdout, "journal-loss") ||
				strings.Contains(stdout, "recovery") ||
				strings.Contains(stdout, "repair") {
				t.Fatalf("clean %s D9 suggested recovery/repair:\n%s", phase, stdout)
			}
			if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatalf("clean %s D9 mutated workspace", phase)
			}
		})
	}
}

func TestS7AuthorityProcessHelper(t *testing.T) {
	if os.Getenv("TPATCH_S7_AUTHORITY_HELPER") != "1" {
		return
	}
	root := os.Getenv("TPATCH_S7_AUTHORITY_ROOT")
	switch os.Getenv("TPATCH_S7_AUTHORITY_ROLE") {
	case "holder":
		authority, err := intentlock.Acquire(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(97)
		}
		fmt.Fprintln(os.Stdout, "READY")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		if err := authority.Release(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		os.Exit(0)
	case "contender":
		command := buildRootCmd()
		command.SetOut(os.Stdout)
		command.SetErr(os.Stderr)
		command.SetArgs([]string{
			"--path", root, "prepare", os.Getenv("TPATCH_S7_AUTHORITY_SLUG"),
			"--manual", "--json", "--quiet",
		})
		os.Exit(execute(command, os.Stderr))
	default:
		os.Exit(99)
	}
}

func TestS7PIB399ManualStatusUsesRootedDurableWriterOnly(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 rooted manual status")
	prepareS4WriteReadyBundle(t, root, slug, false)
	statusRel := filepath.ToSlash(filepath.Join(".tpatch", "features", slug, "status.json"))
	state := &s7RootedWriteState{target: statusRel}
	oldFactory := prepareIntentpubRootOps
	prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
		return &s7RootedWriteOps{RootOps: intentpub.NewRootOps(rooted), state: state}
	}
	t.Cleanup(func() { prepareIntentpubRootOps = oldFactory })

	s7RunManualPrepare(t, root, slug)
	if state.targetTemp == "" {
		t.Fatal("manual status write did not rename an authority-rooted temporary")
	}
	open := "open:" + state.targetTemp
	sync := "sync:" + state.targetTemp
	rename := "rename:" + state.targetTemp + ">" + statusRel
	if s7EventIndex(state.events, open) < 0 ||
		s7EventIndex(state.events, sync) <= s7EventIndex(state.events, open) ||
		s7EventIndex(state.events, rename) <= s7EventIndex(state.events, sync) {
		t.Fatalf("manual rooted write order = %v, want %q -> %q -> %q", state.events, open, sync, rename)
	}
	for _, event := range state.events {
		if strings.Contains(event, "/journal") || strings.Contains(event, "/archive/") {
			t.Fatalf("manual status write touched transaction/archive state: %v", state.events)
		}
	}
	for _, rel := range []string{
		intentpub.JournalRel(slug),
		".tpatch/features/" + slug + "/artifacts/intent-archive",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("manual status publication created %s: %v", rel, err)
		}
	}
}

func TestS7PIB400ManualStatusConcurrentEditHasNoRenameOrFeaturesRefresh(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 manual status concurrent edit")
	prepareS4WriteReadyBundle(t, root, slug, false)
	statusRel := filepath.ToSlash(filepath.Join(".tpatch", "features", slug, "status.json"))
	statusPath := filepath.Join(root, filepath.FromSlash(statusRel))
	featuresPath := filepath.Join(root, ".tpatch", "FEATURES.md")
	featuresBefore, err := os.ReadFile(featuresPath)
	if err != nil {
		t.Fatal(err)
	}
	concurrent := []byte("{\"concurrent\":\"operator-visible\"}\n")
	state := &s7RootedWriteState{target: statusRel}
	oldFactory := prepareIntentpubRootOps
	prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
		return &s7RootedWriteOps{RootOps: intentpub.NewRootOps(rooted), state: state}
	}
	oldHook := beforeManualStatusCAS
	beforeManualStatusCAS = func() {
		if err := os.WriteFile(statusPath, concurrent, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		prepareIntentpubRootOps = oldFactory
		beforeManualStatusCAS = oldHook
	})

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 5 || report.Refusal == nil || report.Refusal.Code != "status-changed" {
		t.Fatalf("manual status CAS = %d %#v", code, report)
	}
	statusAfter, err := os.ReadFile(statusPath)
	if err != nil || !bytes.Equal(statusAfter, concurrent) {
		t.Fatalf("latest concurrent status = %q err=%v", statusAfter, err)
	}
	featuresAfter, err := os.ReadFile(featuresPath)
	if err != nil || !bytes.Equal(featuresAfter, featuresBefore) {
		t.Fatalf("FEATURES.md changed after refused status CAS: err=%v\nbefore=%q\nafter=%q", err, featuresBefore, featuresAfter)
	}
	if state.targetTemp != "" {
		t.Fatalf("refused status CAS reached rename: %v", state.events)
	}
}

func TestS7PIB404CP13PrepareRefusalRendersExactRepairRoutes(t *testing.T) {
	t.Run("globally-unreferenced", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 CP13 unreferenced")
		prepareS4WriteReadyBundle(t, root, slug, false)
		body := []byte("S7 CP13 residue\n")
		tombstone := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireTombstoned)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, tombstone)),
			map[string][]byte{tombstone.ContentSHA256: body},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		want := "tpatch feature intent-archive purge " + slug + " --orphans --yes"
		if code != 3 || report.Refusal == nil ||
			report.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) ||
			!strings.Contains(report.Refusal.Remediation, want) ||
			strings.Contains(stdout, "archive-purge-evidence-divergent") {
			t.Fatalf("unreferenced CP13 route = code:%d report:%#v", code, report)
		}
	})

	t.Run("live-retained-reference", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 CP13 live")
		prepareS4WriteReadyBundle(t, root, slug, false)
		body := []byte("S7 CP13 shared\n")
		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireRetained)
		tombstone := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, body, store.IntentArchiveWireTombstoned)
		writeIntentArchiveCLIFixture(t, root, slug,
			intentArchiveCLIIndex(t, slug,
				intentArchiveCLIGeneration(t, slug, retained),
				intentArchiveCLIGeneration(t, slug, tombstone),
			),
			map[string][]byte{retained.ContentSHA256: body},
		)
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		want := "tpatch feature intent-archive purge " + slug +
			" --blob " + retained.ContentSHA256 + " --yes"
		if code != 3 || report.Refusal == nil ||
			report.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) ||
			!strings.Contains(report.Refusal.Remediation, want) ||
			strings.Contains(report.Refusal.Remediation, "--orphans") ||
			strings.Contains(stdout, "archive-purge-evidence-divergent") {
			t.Fatalf("live CP13 route = code:%d report:%#v", code, report)
		}
	})
}

func TestS7PIB408LinkedWorktreeSubmoduleAndNonWorktreeGitGate(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "base")
	linked := filepath.Join(parent, "linked")
	if err := os.Mkdir(base, 0o755); err != nil {
		t.Fatal(err)
	}
	s7Git(t, base, "init", "-q", "-b", "main")
	s7Git(t, base, "config", "user.email", "s7@example.invalid")
	s7Git(t, base, "config", "user.name", "S7")
	if err := os.WriteFile(filepath.Join(base, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s7Git(t, base, "add", "README.md")
	s7Git(t, base, "commit", "-qm", "base")
	s7Git(t, base, "worktree", "add", "-q", "-b", "linked", linked)
	linkedSlug := s7InitializeReadyManualWorkspace(t, linked, "S7 linked worktree")
	if info, err := os.Stat(filepath.Join(linked, ".git")); err != nil || info.IsDir() {
		t.Fatalf("linked worktree .git = %#v err=%v, want file", info, err)
	}
	s7RunManualPrepare(t, linked, linkedSlug)

	subSource := filepath.Join(parent, "sub-source")
	super := filepath.Join(parent, "super")
	if err := os.Mkdir(subSource, 0o755); err != nil {
		t.Fatal(err)
	}
	s7Git(t, subSource, "init", "-q", "-b", "main")
	s7Git(t, subSource, "config", "user.email", "s7@example.invalid")
	s7Git(t, subSource, "config", "user.name", "S7")
	if err := os.WriteFile(filepath.Join(subSource, "README.md"), []byte("sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s7Git(t, subSource, "add", "README.md")
	s7Git(t, subSource, "commit", "-qm", "sub")
	if err := os.Mkdir(super, 0o755); err != nil {
		t.Fatal(err)
	}
	s7Git(t, super, "init", "-q", "-b", "main")
	s7Git(t, super, "config", "user.email", "s7@example.invalid")
	s7Git(t, super, "config", "user.name", "S7")
	command := exec.Command("git", "-c", "protocol.file.allow=always", "submodule", "add", "-q", subSource, "nested")
	command.Dir = super
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("add local submodule: %v\n%s", err, output)
	}
	nested := filepath.Join(super, "nested")
	nestedSlug := s7InitializeReadyManualWorkspace(t, nested, "S7 nested workspace")
	if info, err := os.Stat(filepath.Join(nested, ".git")); err != nil || info.IsDir() {
		t.Fatalf("submodule .git = %#v err=%v, want file", info, err)
	}
	s7RunManualPrepare(t, nested, nestedSlug)

	bare := filepath.Join(parent, "bare.git")
	if err := os.Mkdir(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	s7Git(t, bare, "init", "--bare", "-q")
	bareSlug := s7InitializeReadyManualWorkspace(t, bare, "S7 established nonworktree")
	s7RunManualPrepare(t, bare, bareSlug)

	emptyPath := filepath.Join(parent, "no-git")
	if err := os.Mkdir(emptyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", emptyPath)
	code, stdout, _, _ := runPrepare(
		t, "--path", linked, "prepare", linkedSlug, "--manual", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 3 || report.Refusal == nil || report.Refusal.Code != "local-lane-unverifiable" {
		t.Fatalf("Git exec failure = %d report=%#v", code, report)
	}
}

func s7InitializeReadyManualWorkspace(t *testing.T, root, title string) string {
	t.Helper()
	if code, _, stderr, _ := runPrepare(t, "--path", root, "init"); code != 0 {
		t.Fatalf("tpatch init: %s", stderr)
	}
	if code, _, stderr, _ := runPrepare(t, "--path", root, "add", title); code != 0 {
		t.Fatalf("tpatch add: %s", stderr)
	}
	slug := storeSlug(title)
	prepareS4WriteReadyBundle(t, root, slug, false)
	return slug
}

func s7RunManualPrepare(t *testing.T, root, slug string) {
	t.Helper()
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("manual prepare = %d stderr=%q\n%s", code, stderr, stdout)
	}
}

func s7Git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(dir, "missing-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(dir, "missing-system"),
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func s7StartCLIProcessHolder(t *testing.T, root string) (*exec.Cmd, func()) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestS7AuthorityProcessHelper$")
	command.Env = append(os.Environ(),
		"TPATCH_S7_AUTHORITY_HELPER=1",
		"TPATCH_S7_AUTHORITY_ROLE=holder",
		"TPATCH_S7_AUTHORITY_ROOT="+root,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "READY" {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("authority holder readiness = %q err=%v", line, err)
	}
	return command, func() {
		_, _ = stdin.Write([]byte{'\n'})
		_ = stdin.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("authority holder release: %v", err)
		}
	}
}

func s7RunCLIProcessContender(t *testing.T, root, slug string) (string, int) {
	t.Helper()
	output, code := s7RunCLIProcess(t, root, slug)
	if code == 0 {
		t.Fatal("real CLI contender unexpectedly acquired the authority")
	}
	return output, code
}

func s7RunCLIProcess(t *testing.T, root, slug string) (string, int) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestS7AuthorityProcessHelper$")
	command.Env = append(os.Environ(),
		"TPATCH_S7_AUTHORITY_HELPER=1",
		"TPATCH_S7_AUTHORITY_ROLE=contender",
		"TPATCH_S7_AUTHORITY_ROOT="+root,
		"TPATCH_S7_AUTHORITY_SLUG="+slug,
	)
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("real CLI process: %v", err)
	}
	return string(output), exit.ExitCode()
}

type s7RootedWriteState struct {
	target     string
	targetTemp string
	events     []string
}

type s7RootedWriteOps struct {
	intentpub.RootOps
	state *s7RootedWriteState
}

func (ops *s7RootedWriteOps) OpenFile(name string, flag int, mode fs.FileMode) (intentpub.RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	ops.state.events = append(ops.state.events, "open:"+name)
	return &s7RootedWriteFile{RootFile: file, name: name, state: ops.state}, nil
}

func (ops *s7RootedWriteOps) Rename(oldName, newName string) error {
	ops.state.events = append(ops.state.events, "rename:"+oldName+">"+newName)
	if newName == ops.state.target {
		ops.state.targetTemp = oldName
	}
	return ops.RootOps.Rename(oldName, newName)
}

type s7RootedWriteFile struct {
	intentpub.RootFile
	name  string
	state *s7RootedWriteState
}

func (file *s7RootedWriteFile) Sync() error {
	file.state.events = append(file.state.events, "sync:"+file.name)
	return file.RootFile.Sync()
}

func s7EventIndex(events []string, want string) int {
	for index, event := range events {
		if event == want {
			return index
		}
	}
	return -1
}
