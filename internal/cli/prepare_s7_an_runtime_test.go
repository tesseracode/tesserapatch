//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
)

func TestS7PIB412NestedMutatorThreadsOneAuthorityAndSerializesSlugs(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 PIB 412 first")
	if code, _, stderr, _ := runPrepare(t, "--path", root, "add", "S7 PIB 412 second"); code != 0 {
		t.Fatalf("add second slug: %s", stderr)
	}
	second := storeSlug("S7 PIB 412 second")

	oldAcquire := prepareAcquireAuthority
	t.Cleanup(func() { prepareAcquireAuthority = oldAcquire })
	acquires := 0
	prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return oldAcquire(path)
	}
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--allow-heuristic", "--json", "--quiet",
	)
	prepareAcquireAuthority = oldAcquire
	if code != 0 || stderr != "" || prepareS4Report(t, stdout).Outcome != "published" || acquires != 1 {
		t.Fatalf("nested prepare = exit:%d stderr:%q acquires:%d\n%s", code, stderr, acquires, stdout)
	}

	holder, closeHolder := s7StartCLIProcessHolder(t, root)
	defer closeHolder()
	output, contenderCode := s7RunCLIProcessContender(t, root, second)
	if contenderCode != 3 || !strings.Contains(output, `"code": "transaction-in-progress"`) {
		t.Fatalf("cross-slug contender = exit:%d\n%s", contenderCode, output)
	}
	if holder.ProcessState != nil {
		t.Fatalf("holder exited while serializing a different slug: %v", holder.ProcessState)
	}
}

func TestS7PIB414RootReplacementBeforePublicationRefusesAndPreservesReplacement(t *testing.T) {
	observation := s7ObservePrepareRootReplacement(t, false)
	if observation.code != 5 || observation.refusalCode != "workspace-root-changed" ||
		!observation.replacementPreserved {
		t.Fatalf("pre-publication root replacement = %+v", observation)
	}
}

func TestS7PIB415RootReplacementAfterPublicationIsExitSixWithEvidence(t *testing.T) {
	observation := s7ObservePrepareRootReplacement(t, true)
	if observation.code != 6 ||
		observation.refusalCode != "workspace-root-replaced-after-publication" ||
		!observation.replacementPreserved || !observation.evidencePreserved {
		t.Fatalf("post-publication root replacement = %+v", observation)
	}
}

func TestS7PIB426GitCleanCanRemoveUntrackedIntentArchive(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "fd", args: []string{"clean", "-fd", "--", ".tpatch/features"}},
		{name: "xfd", args: []string{"clean", "-xfd", "--", ".tpatch/features"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, slug := prepareS4Workspace(t, "S7 PIB 426 "+test.name)
			s7PrepareInitialBundle(t, root, slug)
			feature := filepath.Join(root, ".tpatch", "features", slug)
			spec := filepath.Join(feature, "spec.md")
			if err := os.WriteFile(spec, []byte("bytes to archive\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if code, stdout, stderr, _ := runPrepare(
				t, "--path", root, "prepare", slug,
				"--regenerate", "--allow-heuristic", "--json", "--quiet",
			); code != 0 {
				t.Fatalf("regenerate = %d stderr=%q\n%s", code, stderr, stdout)
			}
			archive := filepath.Join(feature, "artifacts", "intent-archive")
			if _, err := os.Stat(filepath.Join(archive, "index.json")); err != nil {
				t.Fatalf("archive fixture missing: %v", err)
			}
			command := exec.Command("git", test.args...)
			command.Dir = root
			command.Env = append(os.Environ(),
				"GIT_CONFIG_GLOBAL="+filepath.Join(root, "missing-global"),
				"GIT_CONFIG_SYSTEM="+filepath.Join(root, "missing-system"),
				"LC_ALL=C", "LANG=C",
			)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", test.args, err, output)
			}
			if _, err := os.Stat(feature); !os.IsNotExist(err) {
				t.Fatalf("git clean %s did not remove untracked feature/archive: %v", test.name, err)
			}
			code, output, stderr, _ := runPrepare(
				t, "--path", root, "doctor", "--check", "D9", "--json",
			)
			lower := strings.ToLower(output + stderr)
			if code != 0 || strings.Contains(lower, "recovery-pending") ||
				strings.Contains(lower, "journal") && strings.Contains(lower, "lost") {
				t.Fatalf("doctor invented recovery after git clean: exit=%d stderr=%q\n%s", code, stderr, output)
			}
		})
	}
}

type s7RootReplacementObservation struct {
	code                 int
	refusalCode          string
	replacementPreserved bool
	evidencePreserved    bool
}

func s7ObservePrepareRootReplacement(t *testing.T, afterPublication bool) s7RootReplacementObservation {
	t.Helper()
	root, slug := prepareS4Workspace(t, "S7 root replacement")
	prepareS4WriteReadyBundle(t, root, slug, false)
	held := root + ".held"
	marker := []byte("replacement root must survive\n")
	replaced := false
	oldHook := prepareIntentpubHook
	t.Cleanup(func() {
		prepareIntentpubHook = oldHook
		if replaced {
			_ = os.RemoveAll(root)
			_ = os.Rename(held, root)
		}
	})
	prepareIntentpubHook = func(
		point intentpub.CrashPoint,
		_ *os.Root,
		_ *intentpub.Entry,
	) error {
		want := intentpub.PointBeforeSetValidation
		if afterPublication {
			want = intentpub.PointAfterAllRenames
		}
		if point != want || replaced {
			return nil
		}
		if err := os.Rename(root, held); err != nil {
			return err
		}
		if err := os.Mkdir(root, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "replacement.marker"), marker, 0o600); err != nil {
			return err
		}
		replaced = true
		return nil
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	observation := s7RootReplacementObservation{code: code}
	if report.Refusal != nil {
		observation.refusalCode = report.Refusal.Code
	}
	if got, err := os.ReadFile(filepath.Join(root, "replacement.marker")); err != nil || !bytes.Equal(got, marker) {
		t.Fatalf("replacement root bytes changed: %q err=%v", got, err)
	}
	observation.replacementPreserved = true
	if afterPublication {
		lane := filepath.Join(held, ".tpatch", "local", "intent-prepare", slug)
		if _, err := os.Stat(filepath.Join(lane, "journal.json")); err != nil {
			t.Fatalf("post-publication evidence not preserved: %v", err)
		}
		observation.evidencePreserved = true
	}
	return observation
}
