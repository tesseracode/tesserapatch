//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestS7APFilesystemCLIContract(t *testing.T) {
	t.Run("PIB-480", func(t *testing.T) {
		for _, class := range []string{"overlayfs", "sshfs"} {
			root := t.TempDir()
			classifier := func(*os.File) (string, bool, error) {
				return class, false, nil
			}
			authority, err := intentlock.AcquireWithFilesystemClassifier(root, classifier)
			if err != nil {
				t.Fatalf("PIB-480 %s did not reach real flock: %v", class, err)
			}
			contender, contenderErr := intentlock.AcquireWithFilesystemClassifier(root, classifier)
			if contender != nil {
				_ = contender.Release()
				t.Fatalf("PIB-480 %s allowed a concurrent authority", class)
			}
			var typed *intentlock.Error
			if !errors.As(contenderErr, &typed) ||
				typed.Code != intentlock.CodeTransactionInProgress {
				t.Fatalf("PIB-480 %s contender = %v", class, contenderErr)
			}
			if err := authority.Release(); err != nil {
				t.Fatal(err)
			}
			reacquired, err := intentlock.AcquireWithFilesystemClassifier(root, classifier)
			if err != nil {
				t.Fatalf("PIB-480 %s release did not free real flock: %v", class, err)
			}
			if err := reacquired.Release(); err != nil {
				t.Fatal(err)
			}
		}

		code, help, stderr, _ := runPrepare(t, "prepare", "--help")
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-480 prepare help = exit:%d stderr:%q", code, stderr)
		}
		if err := validateS7APFilesystemDisclosure(help); err != nil {
			t.Fatal(err)
		}
		wrong := strings.Replace(
			help,
			"so no\ncross-machine guarantee follows",
			"and therefore provides\ncross-machine exclusion",
			1,
		)
		if err := validateS7APFilesystemDisclosure(wrong); err == nil {
			t.Fatal("PIB-480 disclosure validator accepted a cross-machine guarantee")
		}

		root, slug := prepareS4Workspace(t, "AP detected class")
		lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
		if err := os.MkdirAll(lane, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		previous := prepareAcquireAuthority
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			return intentlock.AcquireWithFilesystemClassifier(
				path,
				func(*os.File) (string, bool, error) { return "nfs", true, nil },
			)
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		prepareAcquireAuthority = previous
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil ||
			report.Refusal.Code != "lock-filesystem-unsupported" ||
			!strings.Contains(strings.ToLower(report.Refusal.Message), "(nfs)") ||
			!strings.Contains(report.Refusal.Remediation, ".tpatch/local/intent-prepare/"+slug+"/") ||
			strings.Contains(stdout, root) {
			t.Fatalf("PIB-480 denied public report = exit:%d report:%+v", code, report)
		}
	})
}

func TestS7APTransactionInProgressTruth(t *testing.T) {
	t.Run("PIB-471", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 AP holder truth")
		authority, err := intentlock.Acquire(root)
		if err != nil {
			t.Fatal(err)
		}
		defer authority.Release()

		assertTruth := func(name string, code int, stdout, stderr string, refusal *prepareRefusalReport) {
			t.Helper()
			if code != 3 || stderr == "" || refusal == nil ||
				refusal.Code != "transaction-in-progress" ||
				!strings.Contains(refusal.Message, "workspace mutation authority is held") ||
				!strings.Contains(refusal.Message, "holder's identity is unknowable") ||
				!strings.Contains(strings.ToLower(refusal.Remediation), "wait") ||
				!strings.Contains(strings.ToLower(refusal.Remediation), "retry") ||
				strings.Contains(stdout, root) {
				t.Fatalf("%s contention truth = exit:%d stderr:%q refusal:%+v\n%s",
					name, code, stderr, refusal, stdout)
			}
		}

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		assertTruth("prepare", code, stdout, stderr, report.Refusal)

		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--yes", "--json", "--quiet",
		)
		archive := decodeIntentArchivePurgeReport(t, stdout)
		if archive.Refusal == nil {
			t.Fatalf("archive contention report lacks refusal: %+v", archive)
		}
		assertTruth("archive", code, stdout, stderr, &prepareRefusalReport{
			Code:        archive.Refusal.Code,
			Message:     archive.Refusal.Message,
			Remediation: archive.Refusal.Remediation,
			Retry:       archive.Refusal.Retry,
			RetryCWD:    archive.Refusal.RetryCWD,
		})
	})
}

func validateS7APFilesystemDisclosure(help string) error {
	lower := strings.ToLower(help)
	for _, required := range []string{
		"host-local",
		"unrecognized local filesystem may accept",
		"directory flock",
		"no\ncross-machine guarantee",
	} {
		if !strings.Contains(lower, required) {
			return errors.New("prepare help lacks filesystem authority disclosure: " + required)
		}
	}
	for _, forbidden := range []string{
		"cross-machine exclusion",
		"cross-machine guarantee is provided",
		"all writers are excluded",
	} {
		if strings.Contains(lower, forbidden) {
			return errors.New("prepare help overclaims authority: " + forbidden)
		}
	}
	return nil
}
