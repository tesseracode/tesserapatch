// Demoted legacy patch scanners (GH #15 / ADR-036 D1, PRD §6.1 PI-2 and
// PI-7).
//
// `FilesInPatch` and `PathsAffectedByPatch` used to be production
// helpers. S1 makes the strict normalized effect grammar the single
// authority for every production consumer that claims a path or an effect
// kind, so both scanners are demoted here rather than deleted outright:
// their measured behaviour is the baseline the strict projections are
// compared against, and the S0 evidence rows that pin the divergence must
// keep running.
//
// Being test-only is load-bearing, not cosmetic. The source-derived
// parser inventory scans production Go only, so a scanner living here is
// unreachable from production by construction — which is exactly the
// PI-2/PI-7 disposition ADR-036 D1 prescribes.
//
// The bodies below are the shipped pre-S1 implementations, byte-for-byte,
// so a comparison against them is a comparison against what actually
// shipped.

package gitutil

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// filesInPatch is the demoted PI-2 fail-soft scanner. It splits each
// `diff --git` header on the first ` b/` and silently skips any header it
// cannot split, which includes every Git C-quoted path. That silence is
// why no production caller may reach it.
func filesInPatch(patch string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		// `diff --git a/<p1> b/<p2>` — we take the b-side since new
		// files have /dev/null on the a-side. Handle quoted paths
		// loosely; git doesn't quote unless the path needs it.
		parts := strings.SplitN(line, " b/", 2)
		if len(parts) != 2 {
			continue
		}
		p := strings.TrimSpace(parts[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// FilesInPatch keeps the demoted scanner reachable under its shipped name
// for the S0 evidence rows that measure the strict/fail-soft divergence.
func FilesInPatch(patch string) []string { return filesInPatch(patch) }

// PathsAffectedByPatch is the demoted PI-7 union. It returns the union of
// both diff-header sides plus rename/copy source and destination paths,
// and it decodes operands with `strconv.Unquote` — a GO-literal decoder
// that accepts escapes Git never emits. `PathsAffectedByPatchStrict` is
// the production replacement: same both-side scope, Git's own C decoder,
// and an error instead of a partial list.
func PathsAffectedByPatch(patch string) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(raw string, stripDiffPrefix bool) {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "/dev/null" {
			return
		}
		if tab := strings.IndexByte(raw, '\t'); tab >= 0 {
			raw = raw[:tab]
		}
		if unquoted, err := strconv.Unquote(raw); err == nil {
			raw = unquoted
		}
		if stripDiffPrefix {
			switch {
			case strings.HasPrefix(raw, "a/"):
				raw = strings.TrimPrefix(raw, "a/")
			case strings.HasPrefix(raw, "b/"):
				raw = strings.TrimPrefix(raw, "b/")
			}
		}
		if raw == "" {
			return
		}
		raw = filepath.ToSlash(raw)
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		paths = append(paths, raw)
	}

	inHeader := false
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inHeader = true
			fields := legacyPathsFromDiffGitHeader(strings.TrimPrefix(line, "diff --git "))
			for _, field := range fields {
				add(field, true)
			}
		case strings.HasPrefix(line, "@@"):
			inHeader = false
		case inHeader && strings.HasPrefix(line, "--- "):
			add(strings.TrimPrefix(line, "--- "), true)
		case inHeader && strings.HasPrefix(line, "+++ "):
			add(strings.TrimPrefix(line, "+++ "), true)
		case inHeader && strings.HasPrefix(line, "rename from "):
			add(strings.TrimPrefix(line, "rename from "), false)
		case inHeader && strings.HasPrefix(line, "rename to "):
			add(strings.TrimPrefix(line, "rename to "), false)
		case inHeader && strings.HasPrefix(line, "copy from "):
			add(strings.TrimPrefix(line, "copy from "), false)
		case inHeader && strings.HasPrefix(line, "copy to "):
			add(strings.TrimPrefix(line, "copy to "), false)
		}
	}
	return paths
}

func legacyPathsFromDiffGitHeader(input string) []string {
	if strings.HasPrefix(input, `"`) {
		fields := splitGitDiffPaths(input)
		if len(fields) == 2 {
			return fields
		}
		return nil
	}
	for offset := 0; offset < len(input); {
		rel := strings.Index(input[offset:], " b/")
		if rel < 0 {
			break
		}
		at := offset + rel
		left, right := input[:at], input[at+1:]
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") &&
			strings.TrimPrefix(left, "a/") == strings.TrimPrefix(right, "b/") {
			return []string{left, right}
		}
		offset = at + len(" b/")
	}
	return nil
}

// TestFilesInPatchLegacy is the demoted PI-2 unit test, moved here from
// internal/workflow/refresh_test.go when its subject left production.
func TestFilesInPatchLegacy(t *testing.T) {
	patch := `diff --git a/foo.txt b/foo.txt
index 111..222 100644
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-hi
+hello
diff --git a/bar/baz.go b/bar/baz.go
new file mode 100644
--- /dev/null
+++ b/bar/baz.go
@@ -0,0 +1 @@
+package baz
diff --git a/foo.txt b/foo.txt
`
	got := filesInPatch(patch)
	want := []string{"foo.txt", "bar/baz.go"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("at %d: got %q want %q", i, got[i], want[i])
		}
	}
	if got := filesInPatch(""); len(got) != 0 {
		t.Errorf("expected no files, got %v", got)
	}
}
