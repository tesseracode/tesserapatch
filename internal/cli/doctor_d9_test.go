package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func TestDoctorCLID9SelectionWarningExitAndFixNoWrite(t *testing.T) {
	rootDir := t.TempDir()
	s, err := store.Init(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "D9 CLI", Slug: "d9-cli"}); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(rootDir, ".tpatch", "local", "intent-prepare", "d9-cli", "journal.json")
	if err := os.MkdirAll(filepath.Dir(journal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journal, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}

	human, err := runDoctorCLI(t, rootDir, "doctor", "--check", "D9")
	if err != nil {
		t.Fatalf("warning-only D9 should preserve exit 0: %v\n%s", err, human)
	}
	for _, want := range []string{"WARNING  D9 prepare-transaction-pending", "feature=d9-cli", "summary: 0 drift findings, 1 warnings, 0 fixed, 0 errors"} {
		if !strings.Contains(human, want) {
			t.Fatalf("D9 human output missing %q:\n%s", want, human)
		}
	}

	structured, err := runDoctorCLI(t, rootDir, "doctor", "--json", "--check", "D9")
	if err != nil {
		t.Fatalf("D9 JSON warning exit: %v\n%s", err, structured)
	}
	var report workflow.DoctorReport
	if err := json.Unmarshal([]byte(structured), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.ChecksRun != 1 || len(report.Checks) != 1 || report.Checks[0].CheckID != "D9" {
		t.Fatalf("D9 selection = %#v", report)
	}

	fixed, err := runDoctorCLI(t, rootDir, "doctor", "--fix", "--json", "--check", "D9")
	if err != nil {
		t.Fatalf("D9 --fix warning exit: %v\n%s", err, fixed)
	}
	after, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("D9 --fix mutated journal: before=%q after=%q", before, after)
	}
}

func TestDoctorCLID9HelpAndDefaultRegistryCount(t *testing.T) {
	rootDir := t.TempDir()
	if _, err := store.Init(rootDir); err != nil {
		t.Fatal(err)
	}
	root := buildRootCmd()
	doctor, _, err := root.Find([]string{"doctor"})
	if err != nil {
		t.Fatal(err)
	}
	help := doctor.Long + "\n" + doctor.Flags().Lookup("check").Usage
	for _, want := range []string{
		"durable prepare/archive evidence (D9)",
		"never repairs findings",
		"opens or probes workspace mutation authority",
		"ordinarily undetectable",
		"D8, D9",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("doctor D9 help missing %q:\n%s", want, help)
		}
	}

	report, err := workflow.RunDoctor(&store.Store{Root: rootDir}, workflow.DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.ChecksRun != 9 || len(report.Checks) != 9 || report.Checks[8].CheckID != "D9" {
		t.Fatalf("default doctor registry = %#v", report.Checks)
	}
}

func TestDoctorD9DisclosureRows(t *testing.T) {
	t.Run("PIB-323", func(t *testing.T) {
		root := buildRootCmd()
		doctor, _, err := root.Find([]string{"doctor"})
		if err != nil {
			t.Fatal(err)
		}
		prd, err := os.ReadFile(filepath.Join(avpRepoRoot(t), "docs", "prds", "PRD-prepare-intent-bundle.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := validateDoctorD9LossDisclosure(doctor.Long, string(prd)); err != nil {
			t.Fatal(err)
		}
		overclaim := strings.Replace(
			doctor.Long,
			"A removed prepare journal is unrecoverable and ordinarily undetectable.",
			"D9 detects a removed prepare journal and reports the loss.",
			1,
		)
		if err := validateDoctorD9LossDisclosure(overclaim, string(prd)); err == nil {
			t.Fatal("journal-loss detection overclaim escaped the disclosure guard")
		}
		docsOverclaim := string(prd) + "\nD9 detects a removed prepare journal while the loss remains unrecoverable and ordinarily undetectable.\n"
		if err := validateDoctorD9LossDisclosure(doctor.Long, docsOverclaim); err == nil {
			t.Fatal("docs-only journal-loss detection overclaim escaped the disclosure guard")
		}
	})
}

func validateDoctorD9LossDisclosure(doctorText, docsText string) error {
	doctorLower := strings.ToLower(doctorText)
	docsLower := strings.ToLower(docsText)
	texts := map[string]string{"doctor": doctorLower, "docs": docsLower}
	for name, text := range texts {
		if !strings.Contains(text, "unrecoverable") || !strings.Contains(text, "ordinarily undetectable") {
			return fmt.Errorf("%s disclosure lost the unrecoverable/ordinarily-undetectable boundary", name)
		}
		for _, forbidden := range []string{
			"d9 detects a removed prepare journal",
			"doctor detects a removed journal",
			"journal loss detected",
		} {
			if strings.Contains(text, forbidden) {
				return fmt.Errorf("%s disclosure overclaims detection with %q", name, forbidden)
			}
		}
	}
	return nil
}
