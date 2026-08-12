// Strict `diff --git` header parsing (GH #7 rev-3 F2).
//
// FilesInPatch is a fail-soft scanner: it splits each header on the
// first ` b/` and silently skips any line it cannot split. That is
// tolerable for advisory callers, but it is a safety hole for the two
// surfaces that derive a WRITE SCOPE from a patch:
//
//   - workflow.RefreshAfterAccept — the touched-path set drives the
//     regenerated post-apply.patch;
//   - cli.computePathSet — the path set drives `git add` before the
//     landing commit.
//
// A path that Git C-quotes (because it contains a space plus a control
// byte, a quote, a backslash, or a newline) renders as
// `diff --git "a/wt/new\nline" "b/wt/new\nline"`. FilesInPatch's
// ` b/` split misses it, the scope silently becomes EMPTY, and an empty
// scope means "everything" to `git diff` — so a stale
// worktree-only patch broadened the refresh to unrelated working-tree
// dirt. Silently producing an empty scope is precisely the failure mode
// this file removes: FilesInPatchStrict returns an error instead.

package gitutil

import (
	"fmt"
	"strconv"
	"strings"
)

// FilesInPatchStrict returns the b-side path of every file entry in a
// unified diff, decoding Git's C-quoting byte-correctly.
//
// Handled: quoted and unquoted paths, paths containing spaces, tabs,
// newlines and octal-escaped bytes, renames, copies, mode-only entries,
// binary entries, new and deleted files.
//
// Any `diff --git` header whose paths cannot be resolved unambiguously
// returns an error. Callers that derive a write scope MUST use this
// function; an unparseable header must never degrade to an empty scope.
//
// The returned slice is de-duplicated and in first-seen order, matching
// FilesInPatch's contract for the inputs both can parse.
func FilesInPatchStrict(patch string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	lines := strings.Split(patch, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		header := strings.TrimPrefix(line, "diff --git ")

		// Corroborating headers live between this `diff --git` line and
		// the first hunk (or the next entry). `rename to` / `copy to`
		// and `+++` are unambiguous single-path lines, so they resolve
		// the cases the two-path header cannot.
		var renameTo, copyTo, plusPath, minusPath string
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if strings.HasPrefix(next, "diff --git ") || strings.HasPrefix(next, "@@") {
				break
			}
			switch {
			case strings.HasPrefix(next, "rename to "):
				renameTo = strings.TrimPrefix(next, "rename to ")
			case strings.HasPrefix(next, "copy to "):
				copyTo = strings.TrimPrefix(next, "copy to ")
			case strings.HasPrefix(next, "+++ "):
				plusPath = strings.TrimPrefix(next, "+++ ")
			case strings.HasPrefix(next, "--- "):
				minusPath = strings.TrimPrefix(next, "--- ")
			}
		}

		path, err := resolveDiffEntryPath(header, renameTo, copyTo, plusPath, minusPath)
		if err != nil {
			return nil, fmt.Errorf("unparseable diff header %q: %w", line, err)
		}
		add(path)
	}
	return out, nil
}

// resolveDiffEntryPath determines the b-side path of one file entry.
//
// Precedence: the `diff --git` header itself when it is unambiguous
// (both paths quoted, or both unquoted with byte-identical payloads),
// then `rename to` / `copy to`, then `+++`, then `---` for deletions.
func resolveDiffEntryPath(header, renameTo, copyTo, plusPath, minusPath string) (string, error) {
	if p, ok, err := pathFromDiffGitHeaderStrict(header); err != nil {
		return "", err
	} else if ok {
		return p, nil
	}
	for _, candidate := range []string{renameTo, copyTo} {
		if candidate == "" {
			continue
		}
		p, err := decodeDiffPathField(candidate)
		if err != nil {
			return "", err
		}
		if p != "" {
			return p, nil
		}
	}
	for _, candidate := range []string{plusPath, minusPath} {
		if candidate == "" {
			continue
		}
		p, err := decodeDiffPathField(stripDiffTimestamp(candidate))
		if err != nil {
			return "", err
		}
		p = stripDiffSidePrefix(p)
		if p != "" && p != "/dev/null" {
			return p, nil
		}
	}
	return "", fmt.Errorf("no unambiguous path in the header or its corroborating lines")
}

// pathFromDiffGitHeaderStrict resolves the b-side path directly from
// the `diff --git` operand. ok=false means "ambiguous, use the
// corroborating headers"; err != nil means "malformed, refuse".
//
// Git's quote_two() quotes BOTH operands or neither, so a leading `"`
// requires exactly two well-formed quoted fields.
func pathFromDiffGitHeaderStrict(header string) (string, bool, error) {
	if header == "" {
		return "", false, fmt.Errorf("empty diff --git operand")
	}
	if strings.HasPrefix(header, `"`) {
		fields := splitGitDiffPaths(header)
		if len(fields) != 2 {
			return "", false, fmt.Errorf("expected two quoted paths, got %d", len(fields))
		}
		bSide, err := decodeDiffPathField(fields[1])
		if err != nil {
			return "", false, err
		}
		bSide = stripDiffSidePrefix(bSide)
		if bSide == "" {
			return "", false, fmt.Errorf("empty b-side path")
		}
		return bSide, true, nil
	}

	// Unquoted. Paths may contain spaces, so the ` b/` boundary is only
	// unambiguous when the a/ and b/ payloads are byte-identical — the
	// same rule pathsFromDiffGitHeader uses. Renames and copies fail
	// here by construction and are resolved by their dedicated headers.
	for offset := 0; offset < len(header); {
		rel := strings.Index(header[offset:], " b/")
		if rel < 0 {
			break
		}
		at := offset + rel
		left, right := header[:at], header[at+1:]
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") &&
			strings.TrimPrefix(left, "a/") == strings.TrimPrefix(right, "b/") {
			return strings.TrimPrefix(right, "b/"), true, nil
		}
		offset = at + len(" b/")
	}
	return "", false, nil
}

// decodeDiffPathField decodes one path field. A field wrapped in double
// quotes is Git C-quoted; strconv.Unquote understands the same escape
// set Git emits (\a \b \f \n \r \t \v \" \\ and three-digit octal), so
// it is reused rather than reimplemented. A field that looks quoted but
// does not decode is malformed and refused — never taken literally.
func decodeDiffPathField(field string) (string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", nil
	}
	if !strings.HasPrefix(field, `"`) {
		return field, nil
	}
	unquoted, err := strconv.Unquote(field)
	if err != nil {
		return "", fmt.Errorf("malformed C-quoted path %q: %v", field, err)
	}
	return unquoted, nil
}

// stripDiffSidePrefix removes the `a/` or `b/` diff-side prefix.
func stripDiffSidePrefix(p string) string {
	switch {
	case strings.HasPrefix(p, "a/"):
		return strings.TrimPrefix(p, "a/")
	case strings.HasPrefix(p, "b/"):
		return strings.TrimPrefix(p, "b/")
	}
	return p
}

// stripDiffTimestamp drops the optional tab-separated timestamp field
// that `---`/`+++` lines can carry.
func stripDiffTimestamp(p string) string {
	if tab := strings.IndexByte(p, '\t'); tab >= 0 {
		return p[:tab]
	}
	return p
}
