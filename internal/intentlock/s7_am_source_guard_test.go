package intentlock

import (
	"fmt"
	"go/build/constraint"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestS7PIB398DirectoryAuthorityConstructionGuardAndWrongInput(t *testing.T) {
	sources := s7AuthorityProductionSources(t)
	if err := validateS7PIB398AuthoritySources(sources); err != nil {
		t.Fatal(err)
	}

	wrong := make(map[string]string, len(sources)+1)
	for name, source := range sources {
		wrong[name] = source
	}
	wrong["pib398-user-cache-wrong.go"] = `package intentlock
import "os"
func pib398WrongInput() { _, _ = os.UserCacheDir() }
`
	if err := validateS7PIB398AuthoritySources(wrong); err == nil ||
		!strings.Contains(err.Error(), "user cache") {
		t.Fatalf("PIB-398 one-delta wrong input did not fail for its semantic reason: %v", err)
	}
}

func TestS7PIB409MutationPlatformMatrixGuardAndWrongInput(t *testing.T) {
	sources := s7AuthorityProductionSources(t)
	confinement := s7ConfinementProductionSources(t)
	targets := s7DistTargets(t)
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PIB409PlatformSources(sources, confinement, targets); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name        string
		authority   map[string]string
		confinement map[string]string
	}{
		{
			name: "windows-mutation-enabled",
			authority: s7ReplaceSource(sources, "acquire_supported.go",
				"//go:build (linux && !android) || (darwin && !ios)",
				"//go:build (linux && !android) || (darwin && !ios) || windows"),
			confinement: confinement,
		},
		{
			name:      "plan9-check-enabled",
			authority: sources,
			confinement: s7ReplaceSource(confinement, "confine_supported.go",
				"//go:build unix || windows",
				"//go:build unix || windows || plan9"),
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7PIB409PlatformSources(
				fixture.authority, fixture.confinement, targets,
			); err == nil {
				t.Fatal("same PIB-409 validator accepted a one-delta GOOS support mutation")
			}
		})
	}
}

func validateS7PIB409PlatformSources(
	sources, confinement map[string]string,
	targets map[string][]string,
) error {
	supported := sources["acquire_supported.go"]
	unsupported := sources["acquire_unsupported.go"]
	supportedExpr, err := s7BuildConstraint(supported)
	if err != nil {
		return fmt.Errorf("PIB-409 supported build constraint: %w", err)
	}
	unsupportedExpr, err := s7BuildConstraint(unsupported)
	if err != nil {
		return fmt.Errorf("PIB-409 unsupported build constraint: %w", err)
	}
	checkSupportedExpr, err := s7BuildConstraint(confinement["confine_supported.go"])
	if err != nil {
		return fmt.Errorf("PIB-417 supported build constraint: %w", err)
	}
	checkUnsupportedExpr, err := s7BuildConstraint(confinement["confine_unsupported.go"])
	if err != nil {
		return fmt.Errorf("PIB-417 unsupported build constraint: %w", err)
	}
	if len(targets) < 10 {
		return fmt.Errorf("PIB-409 go tool dist list exposed only %d GOOS values", len(targets))
	}
	for goos := range targets {
		tags := s7GOOSTags(goos)
		mutationWant := goos == "linux" || goos == "darwin"
		mutationGot := supportedExpr.Eval(func(tag string) bool { return tags[tag] })
		mutationFallback := unsupportedExpr.Eval(func(tag string) bool { return tags[tag] })
		if mutationGot != mutationWant {
			return fmt.Errorf("PIB-409 %s mutation support = %t, want %t", goos, mutationGot, mutationWant)
		}
		if mutationFallback == mutationGot {
			return fmt.Errorf("PIB-409 %s mutation constraints overlap or gap", goos)
		}
		checkWant := tags["unix"] || goos == "windows"
		checkGot := checkSupportedExpr.Eval(func(tag string) bool { return tags[tag] })
		checkFallback := checkUnsupportedExpr.Eval(func(tag string) bool { return tags[tag] })
		if checkGot != checkWant {
			return fmt.Errorf("PIB-417 %s check support = %t, want unix||windows=%t", goos, checkGot, checkWant)
		}
		if checkFallback == checkGot {
			return fmt.Errorf("PIB-417 %s check constraints overlap or gap", goos)
		}
	}
	for fragment, source := range map[string]string{
		"supported true":       supported,
		"root classifier":      supported,
		"real directory flock": supported,
		"unsupported false":    unsupported,
		"closed refusal":       unsupported,
	} {
		want := map[string]string{
			"supported true":       "const AuthoritySupported = true",
			"root classifier":      "classify:     classifyHeldFilesystem",
			"real directory flock": "lock:         lockHeldDirectory",
			"unsupported false":    "const AuthoritySupported = false",
			"closed refusal":       "return unsupportedAuthorityError()",
		}[fragment]
		if !strings.Contains(source, want) {
			return fmt.Errorf("PIB-409 missing %s", fragment)
		}
	}
	return nil
}

func s7DistTargets(t *testing.T) map[string][]string {
	t.Helper()
	output, err := exec.Command("go", "tool", "dist", "list").Output()
	if err != nil {
		t.Fatal(err)
	}
	targets := map[string][]string{}
	for _, line := range strings.Fields(string(output)) {
		goos, goarch, ok := strings.Cut(line, "/")
		if !ok {
			t.Fatalf("invalid go tool dist list row %q", line)
		}
		targets[goos] = append(targets[goos], goarch)
	}
	for goos := range targets {
		sort.Strings(targets[goos])
	}
	return targets
}

func s7GOOSTags(goos string) map[string]bool {
	tags := map[string]bool{goos: true}
	switch goos {
	case "android":
		tags["linux"] = true
	case "ios":
		tags["darwin"] = true
	case "illumos":
		tags["solaris"] = true
	}
	switch goos {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios",
		"linux", "netbsd", "openbsd", "solaris":
		tags["unix"] = true
	}
	return tags
}

func s7ReplaceSource(
	sources map[string]string,
	name, old, replacement string,
) map[string]string {
	clone := make(map[string]string, len(sources))
	for key, source := range sources {
		clone[key] = source
	}
	clone[name] = strings.Replace(clone[name], old, replacement, 1)
	return clone
}

func s7ConfinementProductionSources(t *testing.T) map[string]string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(current), "..", "intent")
	sources := map[string]string{}
	for _, name := range []string{"confine_supported.go", "confine_unsupported.go"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(data)
	}
	return sources
}

func s7BuildConstraint(source string) (constraint.Expr, error) {
	line, _, ok := strings.Cut(source, "\n")
	if !ok {
		return nil, fmt.Errorf("source lacks build line")
	}
	return constraint.Parse(line)
}

func validateS7PIB398AuthoritySources(sources map[string]string) error {
	issues := []string{}
	openRootCalls := 0
	rootDotCalls := 0
	for name, source := range sources {
		result := inspectAuthoritySource(name, source)
		for _, issue := range result.issues {
			issues = append(issues, name+": "+issue)
		}
		openRootCalls += result.openRootCalls
		rootDotCalls += result.rootDotCalls
	}
	if len(issues) != 0 {
		return fmt.Errorf("PIB-398 authority source guard: %s", strings.Join(issues, "; "))
	}
	if openRootCalls != 1 {
		return fmt.Errorf("PIB-398 os.OpenRoot calls = %d, want 1", openRootCalls)
	}
	if rootDotCalls != 1 {
		return fmt.Errorf(`PIB-398 Root.Open(".") calls = %d, want 1`, rootDotCalls)
	}
	return nil
}

func s7AuthorityProductionSources(t *testing.T) map[string]string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(current)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sources[entry.Name()] = string(data)
	}
	return sources
}
