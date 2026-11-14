package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseNotesExtraction(t *testing.T) {
	script := filepath.Join(avpRepoRoot(t), "scripts", "release-notes.sh")
	section := "## v0.23.0 - example\n\n### Fixed\n\n- First fix.\n- Second fix.\n\n"
	changelog := "# Changelog\n\n## v0.23.00 - different tag\n\n- Other.\n\n" +
		section + "## v0.22.0 - previous\n\n- Previous fix.\n"
	for _, test := range []struct {
		name, input, want string
		args              []string
		refuse            bool
	}{
		{name: "exact-section", input: changelog, args: []string{"v0.23.0"}, want: section},
		{name: "last-section", input: section, args: []string{"v0.23.0"}, want: section},
		{name: "prerelease", input: "## v0.23.0-rc.1\n\n- Candidate.\n", args: []string{"v0.23.0-rc.1"}, want: "## v0.23.0-rc.1\n\n- Candidate.\n"},
		{name: "missing", input: changelog, args: []string{"v0.24.0"}, refuse: true},
		{name: "duplicate", input: changelog + section, args: []string{"v0.23.0"}, refuse: true},
		{name: "empty-section", input: "## v0.23.0\n\n### Fixed\n\n## v0.22.0\n- Previous.\n", args: []string{"v0.23.0"}, refuse: true},
		{name: "missing-argument", input: changelog, refuse: true},
		{name: "extra-argument", input: changelog, args: []string{"v0.23.0", "extra"}, refuse: true},
		{name: "malformed-tag", input: changelog, args: []string{"v0.23"}, refuse: true},
		{name: "multiline-tag", input: changelog, args: []string{"v0.23.0\nv0.22.0"}, refuse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("sh", append([]string{script}, test.args...)...)
			cmd.Stdin = strings.NewReader(test.input)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if test.refuse {
				if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "release-notes") {
					t.Fatalf("refusal: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
				}
			} else if err != nil || stdout.String() != test.want {
				t.Fatalf("extraction: err=%v stdout=%q want=%q stderr=%q", err, stdout.String(), test.want, stderr.String())
			}
		})
	}
}

func TestReleaseNotesWorkflow(t *testing.T) {
	root := avpRepoRoot(t)
	workflowBytes, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	if err := checkReleaseNotesWorkflow(t, root, workflow); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ name, from, to string }{
		{"generated-notes", "body_path: ${{ runner.temp }}/release-notes.md", "generate_release_notes: true"},
		{"wrong-body-file", "body_path: ${{ runner.temp }}/release-notes.md", "body_path: ${{ runner.temp }}/other.md"},
		{"wrong-tag-source", "RELEASE_TAG: ${{ github.ref_name }}", "RELEASE_TAG: v0.22.0"},
		{"wrong-extraction-tag", `release-notes.sh "$RELEASE_TAG"`, "release-notes.sh v0.22.0"},
		{"failure-masked", `sh scripts/release-notes.sh "$RELEASE_TAG" < CHANGELOG.md > "$RUNNER_TEMP/release-notes.md"`,
			`sh scripts/release-notes.sh "$RELEASE_TAG" < CHANGELOG.md > "$RUNNER_TEMP/release-notes.md" || true`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(workflow, mutation.from, mutation.to, 1)
			if changed == workflow {
				t.Fatal("mutation changed nothing")
			}
			if err := checkReleaseNotesWorkflow(t, root, changed); err == nil {
				t.Fatal("broken release-note publishing accepted")
			}
		})
	}
}

func checkReleaseNotesWorkflow(t *testing.T, root, workflow string) error {
	t.Helper()
	steps, err := parseS7CITimeoutSteps(workflow)
	if err != nil {
		return err
	}
	var extract, publish []s7CITimeoutStep
	extractIndex, publishIndex := -1, -1
	for i, step := range steps {
		if step.job != "release" {
			continue
		}
		if step.name == "Extract canonical release notes" {
			extract, extractIndex = append(extract, step), i
		}
		if step.name == "Create GitHub Release" {
			publish, publishIndex = append(publish, step), i
		}
	}
	if len(extract) != 1 || len(publish) != 1 || extractIndex >= publishIndex {
		return fmt.Errorf("release-note extraction must run once before publishing")
	}
	if extract[0].shell != "bash" || extract[0].condition != "" ||
		extract[0].continueOnError != "" || extract[0].environment["RELEASE_TAG"] != "${{ github.ref_name }}" {
		return fmt.Errorf("release-note extraction must block publishing and use the tag")
	}
	if publish[0].uses != "softprops/action-gh-release@v2" ||
		publish[0].inputs["body_path"] != "${{ runner.temp }}/release-notes.md" ||
		publish[0].inputs["generate_release_notes"] != "" {
		return fmt.Errorf("release must publish the extracted body, not generated notes")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "scripts"), 0755); err != nil {
		return err
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "release-notes.sh"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "release-notes.sh"), script, 0644); err != nil {
		return err
	}
	want := "## v0.23.0 - fixture\n\n- Canonical notes.\n\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(want+"## v0.22.0\n- Previous.\n"), 0644); err != nil {
		return err
	}
	for _, tag := range []string{"v0.23.0", "v0.24.0"} {
		cmd := exec.Command("bash", "-c", extract[0].run)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BASH_ENV=/dev/null", "RELEASE_TAG="+tag, "RUNNER_TEMP="+dir)
		out, err := cmd.CombinedOutput()
		if tag == "v0.24.0" {
			if err == nil {
				return fmt.Errorf("missing release section did not block publishing")
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("extract workflow: %w: %s", err, out)
		}
		got, err := os.ReadFile(filepath.Join(dir, "release-notes.md"))
		if err != nil || string(got) != want {
			return fmt.Errorf("release body: err=%v got=%q want=%q", err, got, want)
		}
	}
	return nil
}
