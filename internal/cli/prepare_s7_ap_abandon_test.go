//go:build (linux && !android) || (darwin && !ios)

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
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7APCrashFixtureHelper(t *testing.T) {
	if os.Getenv("TPATCH_S7_AP_CRASH_HELPER") != "1" {
		return
	}
	root := os.Getenv("TPATCH_S7_AP_CRASH_ROOT")
	slug := os.Getenv("TPATCH_S7_AP_CRASH_SLUG")
	renames := 0
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
		if point == intentpub.PointAfterEntryRename {
			renames++
			if renames == 2 {
				os.Exit(97)
			}
		}
		return nil
	}
	command := buildRootCmd()
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	args := []string{"--path", root, "prepare", slug}
	if os.Getenv("TPATCH_S7_AP_CRASH_REGENERATE") == "1" {
		args = append(args, "--regenerate")
	}
	args = append(args, "--allow-heuristic", "--json", "--quiet")
	command.SetArgs(args)
	os.Exit(execute(command, os.Stderr))
}

func TestS7APAbandonContracts(t *testing.T) {
	t.Run("PIB-449", func(t *testing.T) {
		for _, class := range []string{
			"journal-corrupt",
			"journal-forged",
			"journal-version-mismatch",
			"journal-foreign",
		} {
			root, slug := prepareS4Workspace(t, "AP abandon "+class)
			s6WriteJournalFixture(t, root, slug, class)
			code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "prepare", slug,
				"--abandon-transaction", "--yes", "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
				report.Abandoned == nil || len(report.Abandoned.Moved) == 0 ||
				report.Refusal != nil {
				t.Fatalf("PIB-449 %s abandon = exit:%d stderr:%q report:%+v", class, code, stderr, report)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))); !os.IsNotExist(err) {
				t.Fatalf("PIB-449 %s journal remained in live lane: %v", class, err)
			}
			code, stdout, stderr, _ = runPrepare(
				t, "--path", root, "prepare", slug,
				"--allow-heuristic", "--json", "--quiet",
			)
			if code != 0 || stderr != "" {
				t.Fatalf("PIB-449 %s subsequent mutation = exit:%d stderr:%q stdout:%s", class, code, stderr, stdout)
			}
		}
	})

	t.Run("PIB-450", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AP CP9 abandon")
		s7APCreateCP4Journal(t, root, slug)
		_, journalRaw, published := s7APCP4JournalState(t, root, slug)
		if len(published) != 2 {
			t.Fatalf("PIB-450 CP4 published entries = %v, want exactly two", published)
		}
		divergent := filepath.Join(root, filepath.FromSlash(published[0].Rel))
		if err := os.WriteFile(divergent, []byte("third-party CP9 bytes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if identity := s7APCaptureIdentity(t, root, published[0].Rel); identity.Equal(published[0].Preimage) || identity.Equal(published[0].NewImage) {
			t.Fatalf("PIB-450 fixture is not CP9 divergent: %+v", identity)
		}
		otherFeature := filepath.Join(root, ".tpatch", "features", "ap-other-feature")
		if err := os.MkdirAll(filepath.Join(otherFeature, "nested"), 0o751); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(otherFeature, "nested", "sentinel.bin"),
			[]byte{0, 1, 2, '\n'}, 0o640,
		); err != nil {
			t.Fatal(err)
		}
		featuresRoot := filepath.Join(root, ".tpatch", "features")
		featuresBefore := snapshotTreeMetadata(t, "PIB-450 features", featuresRoot)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		featuresAfter := snapshotTreeMetadata(t, "PIB-450 features", featuresRoot)
		destination := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(report.Abandoned.Directory, "/")))
		movedJournal := filepath.Join(destination, "journal.json")
		lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
		evidenceDirectories, globErr := filepath.Glob(filepath.Join(lane, "abandoned-*"))
		expectedEvidenceRel, relErr := filepath.Rel(root, destination)
		if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
			report.Abandoned == nil || report.Recovery != nil ||
			featuresBefore != featuresAfter ||
			globErr != nil || len(evidenceDirectories) != 1 ||
			evidenceDirectories[0] != destination || relErr != nil ||
			report.Abandoned.Directory != filepath.ToSlash(expectedEvidenceRel)+"/" {
			t.Fatalf("PIB-450 CP9 abandon = exit:%d stderr:%q report:%+v featuresChanged:%t evidence:%v globErr:%v relErr:%v",
				code, stderr, report, featuresBefore != featuresAfter,
				evidenceDirectories, globErr, relErr)
		}
		if moved, err := os.ReadFile(movedJournal); err != nil ||
			!bytes.Equal(moved, journalRaw) {
			t.Fatalf("PIB-450 report directory did not contain exact moved journal: %v", err)
		}
	})

	t.Run("PIB-451", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AP clean CP4 abandon")
		s7APCreateCP4Journal(t, root, slug)
		_, journalRaw, published := s7APCP4JournalState(t, root, slug)
		if len(published) != 2 {
			t.Fatalf("PIB-451 CP4 published entries = %v, want exactly two", published)
		}
		for _, entry := range published {
			if entry.Action != intentpub.ActionReplace ||
				entry.PreimageBlobRel == "" || !entry.Preimage.Exists {
				t.Fatalf("PIB-451 published entry lacks durable preimage: %+v", entry)
			}
		}
		canonicalBefore := readTree(t, filepath.Join(root, ".tpatch", "features", slug))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		canonicalAfter := readTree(t, filepath.Join(root, ".tpatch", "features", slug))
		if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
			report.Abandoned == nil || report.Recovery != nil ||
			!bytes.Equal(canonicalBefore, canonicalAfter) {
			t.Fatalf("PIB-451 CP4 abandon = exit:%d stderr:%q report:%+v canonicalChanged:%t",
				code, stderr, report, !bytes.Equal(canonicalBefore, canonicalAfter))
		}
		for _, entry := range published {
			if identity := s7APCaptureIdentity(t, root, entry.Rel); !identity.Equal(entry.NewImage) {
				t.Fatalf("PIB-451 abandon undid %s: got %+v want new image %+v",
					entry.Rel, identity, entry.NewImage)
			}
		}
		liveJournal := filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))
		if _, err := os.Stat(liveJournal); !os.IsNotExist(err) {
			t.Fatalf("PIB-451 live journal was consumed instead of moved: %v", err)
		}
		movedJournal := filepath.Join(
			root,
			filepath.FromSlash(strings.TrimSuffix(report.Abandoned.Directory, "/")),
			"journal.json",
		)
		if moved, err := os.ReadFile(movedJournal); err != nil ||
			!bytes.Equal(moved, journalRaw) {
			t.Fatalf("PIB-451 exact journal was not moved: %v", err)
		}

		humanRoot, humanSlug := prepareS4Workspace(t, "AP clean CP4 human abandon")
		s7APCreateCP4Journal(t, humanRoot, humanSlug)
		code, stdout, stderr, _ = runPrepare(
			t, "--path", humanRoot, "prepare", humanSlug,
			"--abandon-transaction", "--yes",
		)
		if code != 0 || stderr != "" ||
			!strings.Contains(stdout, "No canonical file changed.") ||
			strings.Contains(stdout, "Recovered an interrupted prepare transaction") ||
			strings.Contains(stdout, "restored  ") ||
			strings.Contains(strings.ToLower(stdout), "repair") {
			t.Fatalf("PIB-451 human abandon implied canonical repair: exit:%d stderr:%q\n%s",
				code, stderr, stdout)
		}
	})

	t.Run("PIB-452", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AP abandon ordering")
		lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
		if err := os.MkdirAll(lane, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		events := []string{}
		oldLock, oldBranch := beforeLockAcquire, beforeAbandonBranch
		beforeLockAcquire = func() { events = append(events, "authority") }
		beforeAbandonBranch = func() { events = append(events, "abandon") }
		t.Cleanup(func() {
			beforeLockAcquire, beforeAbandonBranch = oldLock, oldBranch
		})
		bin := filepath.Join(root, "git-spy")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		logPath := filepath.Join(root, "git.log")
		script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TPATCH_S7_AP_GIT_LOG\"\nexit 88\n"
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		t.Setenv("TPATCH_S7_AP_GIT_LOG", logPath)
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		_, logErr := os.Stat(logPath)
		source := s6RepoFile(t, "internal/cli/prepare_publish.go")
		if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
			!reflect.DeepEqual(events, []string{"authority", "abandon"}) ||
			!os.IsNotExist(logErr) {
			t.Fatalf("PIB-452 ordering = exit:%d stderr:%q report:%+v events:%v git:%v",
				code, stderr, report, events, logErr)
		}
		if err := validateS7APAbandonControlFlow(source); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PIB-453", func(t *testing.T) {
		for count := 1; count <= 3; count++ {
			root, slug := prepareS4Workspace(t, fmt.Sprintf("AP pending hashes %d", count))
			ids := []store.IntentArchiveArtifactID{
				store.IntentArchiveArtifactAnalysis,
				store.IntentArchiveArtifactSpec,
				store.IntentArchiveArtifactExploration,
			}
			replacements := make([]store.IntentArchiveReplacement, 0, count)
			blobs := map[string][]byte{}
			hashes := make([]string, 0, count)
			for index := 0; index < count; index++ {
				data := []byte(fmt.Sprintf("pending-%d\n", index))
				replacement := intentArchiveCLIReplacement(
					t, ids[index], data, store.IntentArchiveWireRemovalPending,
				)
				replacements = append(replacements, replacement)
				blobs[replacement.ContentSHA256] = data
				hashes = append(hashes, replacement.ContentSHA256)
			}
			sort.Slice(replacements, func(i, j int) bool {
				return replacements[i].ArtifactID < replacements[j].ArtifactID
			})
			sort.Strings(hashes)
			index := intentArchiveCLIIndex(t, slug,
				intentArchiveCLIGeneration(t, slug, replacements...),
			)
			writeIntentArchiveCLIFixture(t, root, slug, index, blobs)
			before := readTree(t, filepath.Join(root, ".tpatch"))
			code, stdout, _, _ := runPrepare(
				t, "--path", root, "prepare", slug,
				"--abandon-transaction", "--yes", "--json", "--quiet",
			)
			report := prepareS4Report(t, stdout)
			if code != 3 || report.Refusal == nil ||
				report.Refusal.Code != "no-pending-transaction" ||
				!bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatalf("PIB-453 count %d = exit:%d report:%+v", count, code, report)
			}
			if err := validateS7APPendingHashRoute(report.Refusal, slug, hashes); err != nil {
				t.Fatal(err)
			}
			wrong := *report.Refusal
			wrong.Retry = "tpatch feature intent-archive purge " + slug + " --all --yes"
			if err := validateS7APPendingHashRoute(&wrong, slug, hashes); err == nil {
				t.Fatalf("PIB-453 count %d validator accepted widened --all route", count)
			}
		}
	})
}

func TestS7APRootedWriterGuards(t *testing.T) {
	sources, err := s6PrepareWriteSources(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("PIB-454", func(t *testing.T) {
		if err := validateS7APRootedWriterSources(t, sources); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("PIB-456", func(t *testing.T) {
		if err := validateS7APRootedWriterSources(t, sources); err != nil {
			t.Fatal(err)
		}
		wrong := s6CloneSourceSet(sources)
		const needle = "func runPrepareAbandon("
		source := wrong["internal/cli/prepare_publish.go"]
		if !strings.Contains(source, needle) {
			t.Fatal("PIB-456 mutation anchor missing")
		}
		// Inject into the first statement of the real function, not a dead fixture.
		open := strings.Index(source, needle)
		body := strings.Index(source[open:], "{")
		if body < 0 {
			t.Fatal("PIB-456 function body missing")
		}
		at := open + body + 1
		wrong["internal/cli/prepare_publish.go"] = source[:at] +
			"\n\t_ = os.WriteFile(filepath.Join(repoRoot, \".tpatch\", \"bypass\"), []byte(\"x\"), 0o600)\n" +
			source[at:]
		if err := validateS7APRootedWriterSources(t, wrong); err == nil {
			t.Fatal("PIB-456 same validator accepted a real reachable path writer")
		}
	})
}

func s7APCreateCP4Journal(t *testing.T, root, slug string) {
	t.Helper()
	prepareS4WriteReadyBundle(t, root, slug, false)
	command := exec.Command(os.Args[0], "-test.run=^TestS7APCrashFixtureHelper$")
	command.Env = append(os.Environ(),
		"TPATCH_S7_AP_CRASH_HELPER=1",
		"TPATCH_S7_AP_CRASH_REGENERATE=1",
		"TPATCH_S7_AP_CRASH_ROOT="+root,
		"TPATCH_S7_AP_CRASH_SLUG="+slug,
	)
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 97 {
		t.Fatalf("CP4 helper = err:%v output:%s", err, output)
	}
	journal := filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug)))
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("CP4 helper did not leave journal: %v", err)
	}
}

func s7APCP4JournalState(
	t *testing.T,
	root, slug string,
) (intentpub.Journal, []byte, []intentpub.Entry) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(intentpub.JournalRel(slug))))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := intentpub.DecodeJournal(raw, slug)
	if err != nil {
		t.Fatalf("strict decode CP4 journal: %v", err)
	}
	var published []intentpub.Entry
	for _, entry := range journal.Entries {
		identity := s7APCaptureIdentity(t, root, entry.Rel)
		switch {
		case identity.Equal(entry.NewImage):
			published = append(published, entry)
		case identity.Equal(entry.Preimage):
		default:
			t.Fatalf("CP4 entry %s is neither preimage nor new-image: %+v", entry.Rel, identity)
		}
	}
	return journal, raw, published
}

func s7APCaptureIdentity(t *testing.T, root, rel string) intentpub.Identity {
	t.Helper()
	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	identity, captureErr := intentpub.CaptureIdentity(authority, rel, intentpub.Options{})
	releaseErr := authority.Release()
	if captureErr != nil {
		t.Fatal(captureErr)
	}
	if releaseErr != nil {
		t.Fatal(releaseErr)
	}
	return identity
}

func validateS7APAbandonControlFlow(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "prepare_publish.go", source, 0)
	if err != nil {
		return err
	}
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && function.Body != nil {
			functions[function.Name.Name] = function
		}
	}
	root := functions["runPrepareAbandon"]
	if root == nil {
		return errors.New("abandon control-flow root missing")
	}
	reachable := map[string]bool{}
	queue := []string{"runPrepareAbandon"}
	for len(queue) != 0 {
		name := queue[0]
		queue = queue[1:]
		if reachable[name] || functions[name] == nil {
			continue
		}
		reachable[name] = true
		ast.Inspect(functions[name].Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, _ := call.Fun.(*ast.Ident); ident != nil && functions[ident.Name] != nil {
				queue = append(queue, ident.Name)
			}
			return true
		})
	}
	required := map[string]int{
		"prepareAcquireAuthority":    0,
		"beforeAbandonBranch":        0,
		"movePrepareAbandonEvidence": 0,
	}
	for name := range reachable {
		ast.Inspect(functions[name].Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if _, ok := required[fun.Name]; ok {
					required[fun.Name]++
				}
				switch fun.Name {
				case "intentArchiveRecoverPurge", "prepareRecoverPendingArchivePurge":
					err = fmt.Errorf("abandon reaches archive recovery through %s", name)
				}
			case *ast.SelectorExpr:
				pkg, _ := fun.X.(*ast.Ident)
				if pkg != nil {
					switch pkg.Name {
					case "gitutil":
						err = fmt.Errorf("abandon reaches Git through %s", name)
					case "intentpub":
						if fun.Sel.Name == "Recover" || fun.Sel.Name == "Execute" {
							err = fmt.Errorf("abandon reaches journal transaction through %s", name)
						}
					case "store":
						if fun.Sel.Name == "RecoverPendingPurge" {
							err = fmt.Errorf("abandon reaches purge recovery through %s", name)
						}
					}
				}
			}
			return err == nil
		})
		if err != nil {
			return err
		}
	}
	for name, count := range required {
		if count == 0 {
			return fmt.Errorf("abandon control flow lacks %s", name)
		}
	}
	return nil
}

func validateS7APPendingHashRoute(
	refusal *prepareRefusalReport,
	slug string,
	hashes []string,
) error {
	if refusal == nil || refusal.RetryCWD != "workspace-root" ||
		strings.Contains(refusal.Retry, "--all") {
		return fmt.Errorf("pending route metadata = %+v", refusal)
	}
	argv, err := s7APParseRenderedCommand(refusal.Retry)
	if err != nil {
		return err
	}
	want := []string{"feature", "intent-archive", "purge", slug}
	for _, hash := range hashes {
		want = append(want, "--blob", hash)
	}
	want = append(want, "--yes")
	if fmt.Sprint(argv) != fmt.Sprint(want) {
		return fmt.Errorf("pending route argv = %v, want %v", argv, want)
	}
	return nil
}

func s7APParseRenderedCommand(command string) ([]string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 || fields[0] != "tpatch" {
		return nil, fmt.Errorf("rendered command lacks tpatch prefix: %q", command)
	}
	for _, field := range fields {
		for _, char := range field {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._/", char) {
				return nil, fmt.Errorf("rendered command contains unsafe token %q", field)
			}
		}
	}
	return fields[1:], nil
}

func validateS7APRootedWriterSources(t *testing.T, sources map[string]string) error {
	t.Helper()
	if err := validateS6WriteTargetSourceSet(t, sources); err != nil {
		return err
	}
	model, err := s6BuildSourceTypeModel(sources)
	if err != nil {
		return err
	}
	reachable, err := s6PrepareReachableFunctions(sources, model)
	if err != nil {
		return err
	}
	roleCounts := map[string]int{}
	for rel, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), rel, source, 0)
		if err != nil {
			return err
		}
		highLevel := strings.HasPrefix(rel, "internal/cli/") ||
			strings.HasPrefix(rel, "internal/store/") ||
			strings.HasPrefix(rel, "internal/workflow/")
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil ||
				!reachable[s6PrepareReachabilityKey(rel, filepath.Dir(rel), function)] {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if err != nil {
					return false
				}
				switch value := node.(type) {
				case *ast.KeyValueExpr:
					key, _ := value.Key.(*ast.Ident)
					roleName := ""
					switch role := value.Value.(type) {
					case *ast.Ident:
						roleName = role.Name
					case *ast.SelectorExpr:
						roleName = role.Sel.Name
					}
					if key != nil && key.Name == "Role" &&
						strings.HasPrefix(roleName, "WriteRole") {
						roleCounts[roleName]++
					}
				case *ast.CallExpr:
					if !highLevel {
						return true
					}
					selector, _ := value.Fun.(*ast.SelectorExpr)
					if selector == nil {
						return true
					}
					pkg, _ := selector.X.(*ast.Ident)
					if pkg == nil {
						return true
					}
					switch pkg.Name + "." + selector.Sel.Name {
					case "os.WriteFile", "os.Create", "os.CreateTemp", "os.OpenFile",
						"gitutil.DurableWriteFile":
						err = fmt.Errorf("%s:%s reachable high-level path writer %s.%s",
							rel, function.Name.Name, pkg.Name, selector.Sel.Name)
					}
				}
				return err == nil
			})
		}
		if err != nil {
			return err
		}
	}
	for _, role := range []string{
		"WriteRoleOrdinaryCanonical",
		"WriteRoleCanonicalStatus",
		"WriteRoleControl",
	} {
		if roleCounts[role] == 0 {
			return fmt.Errorf("rooted write-role inventory lacks %s", role)
		}
	}
	return nil
}

func s7APDecodeJSONReport(t *testing.T, stdout string, destination any) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(stdout))
	if err := decoder.Decode(destination); err != nil {
		t.Fatalf("decode AP JSON report: %v\n%s", err, stdout)
	}
}
