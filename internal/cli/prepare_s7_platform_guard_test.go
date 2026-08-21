//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS7PIB409WindowsBlockingSelectorGuard(t *testing.T) {
	workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7WindowsBlockingSelector(string(workflow)); err != nil {
		t.Fatal(err)
	}
	wrong := strings.Replace(
		string(workflow),
		"TestS7WindowsPlatformRows/PIB-409",
		"TestS7WindowsPlatformRows/PIB-410",
		1,
	)
	if err := validateS7WindowsBlockingSelector(wrong); err == nil ||
		!strings.Contains(err.Error(), "PIB-409") {
		t.Fatalf("Windows workflow guard accepted a missing PIB-409 native selector: %v", err)
	}
}

func TestS7PIB417WindowsBlockingLeafGuard(t *testing.T) {
	workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	wrong := strings.Replace(
		string(workflow),
		`TestS7WindowsPlatformRows/PIB-417/check-ready-json.txt`,
		`TestS7WindowsPlatformRows/PIB-417/check-ready-json-wrong.txt`,
		1,
	)
	if err := validateS7WindowsBlockingSelector(wrong); err == nil ||
		!strings.Contains(err.Error(), "PIB-417") {
		t.Fatalf("Windows workflow guard accepted a missing exact PIB-417 leaf assertion: %v", err)
	}
}

func validateS7WindowsBlockingSelector(workflow string) error {
	required := []string{
		"-run '^TestPreparePIBUnsupportedPlatformRuntimeGolden$'",
		"-run '^TestS7WindowsPlatformRows$'",
		"--- PASS: TestS7WindowsPlatformRows",
		"TestS7WindowsPlatformRows/PIB-409",
		"TestS7WindowsPlatformRows/PIB-417",
		"TestS7WindowsPlatformRows/PIB-417/check-ready-human.txt",
		"TestS7WindowsPlatformRows/PIB-417/check-ready-json.txt",
		`--- PASS: ${selector}`,
	}
	for _, value := range required {
		if !strings.Contains(workflow, value) {
			return &s7WorkflowGuardError{value: value}
		}
	}
	return nil
}

type s7WorkflowGuardError struct{ value string }

func (err *s7WorkflowGuardError) Error() string {
	return "blocking Windows selector lost exact fragment " + err.value
}
