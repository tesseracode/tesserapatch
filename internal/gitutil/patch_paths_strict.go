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
//
// GH #7 rev-4 tightens the grammar three ways:
//
//  1. Non-blank input with ZERO `diff --git` headers is malformed. It
//     used to return an empty, error-free scope — the same
//     "empty means everything" hole, reached by a different route
//     (a truncated artifact, or a plain `diff -u` patch).
//  2. BOTH operands are parsed and validated before either is used.
//     Validating only the b-side let a corrupt a-side pass unnoticed.
//  3. Quoted operands are decoded by a GIT-SPECIFIC C decoder, not
//     strconv.Unquote. Go accepts `\x41`, `\u0041`, `\U0001F600`, `\'`
//     and one-or-two-digit octal; Git emits none of those. Accepting
//     them means accepting bytes no Git ever wrote, i.e. accepting
//     corruption as if it were a path.

package gitutil

import (
	"fmt"
	"strings"
)

// gitCQuoteEscapes is exactly the single-letter escape set Git's
// quote_c_style() emits (see git's quote.c cq_lookup table). Anything
// else — `\x`, `\u`, `\U`, `\'`, `\0`-prefixed short octal — is a byte
// sequence Git never produces.
var gitCQuoteEscapes = map[byte]byte{
	'a':  0x07,
	'b':  0x08,
	'f':  0x0c,
	'n':  0x0a,
	'r':  0x0d,
	't':  0x09,
	'v':  0x0b,
	'"':  '"',
	'\\': '\\',
}

// unquoteGitCStyle decodes one Git C-quoted field.
//
// Contract, mirroring git's own quoting exactly:
//
//   - the field must start with `"` and end with `"`, with nothing after
//     the closing quote;
//   - escapes are limited to gitCQuoteEscapes plus EXACTLY three octal
//     digits (git always pads: `\303`, never `\3` or `\30`);
//   - an octal value above 0xFF, a short octal run, an unknown escape,
//     a trailing backslash or an unterminated quote are all malformed.
//
// strconv.Unquote is deliberately NOT used: it is a Go-literal decoder
// and accepts escapes Git never emits, so it would silently turn
// corrupted bytes into a plausible-looking path.
func unquoteGitCStyle(field string) (string, error) {
	if len(field) < 2 || field[0] != '"' {
		return "", fmt.Errorf("not a C-quoted field")
	}
	var b strings.Builder
	for i := 1; i < len(field); {
		c := field[i]
		switch {
		case c == '"':
			if i != len(field)-1 {
				return "", fmt.Errorf("trailing bytes after the closing quote")
			}
			return b.String(), nil
		case c != '\\':
			b.WriteByte(c)
			i++
		default:
			i++
			if i >= len(field) {
				return "", fmt.Errorf("escape at end of field")
			}
			e := field[i]
			if decoded, ok := gitCQuoteEscapes[e]; ok {
				b.WriteByte(decoded)
				i++
				continue
			}
			if e < '0' || e > '7' {
				return "", fmt.Errorf("escape %q is not one Git emits", `\`+string(e))
			}
			if i+2 >= len(field) {
				return "", fmt.Errorf("octal escape must be exactly three digits")
			}
			d2, d3 := field[i+1], field[i+2]
			if d2 < '0' || d2 > '7' || d3 < '0' || d3 > '7' {
				return "", fmt.Errorf("octal escape must be exactly three digits")
			}
			val := int(e-'0')*64 + int(d2-'0')*8 + int(d3-'0')
			if val > 0xFF {
				return "", fmt.Errorf("octal escape \\%c%c%c overflows a byte", e, d2, d3)
			}
			b.WriteByte(byte(val))
			i += 3
		}
	}
	return "", fmt.Errorf("unterminated quote")
}

// diffOperand is one validated side of a `diff --git` header.
type diffOperand struct {
	// Prefix is the `a/` or `b/` marker, retained so the caller can
	// verify the pair.
	Prefix string
	// Path is the decoded, prefix-stripped repo-relative path.
	Path string
}

// parseDiffGitOperands parses and validates BOTH operands of a
// `diff --git` header.
//
// ok=false means "structurally fine but the unquoted split is
// ambiguous" (a rename or copy of unquoted names), which the caller
// resolves from `rename to` / `copy to` / `+++` / `---`. err != nil
// means malformed: refuse.
//
// Git always writes `a/<path> b/<path>` here. A repository configured
// with `diff.noprefix` or `diff.mnemonicPrefix` produces a different
// shape; that is refused rather than guessed at, and the error names
// the likely cause.
func parseDiffGitOperands(header string) (a, b diffOperand, ok bool, err error) {
	if strings.TrimSpace(header) == "" {
		return a, b, false, fmt.Errorf("empty diff --git operands")
	}

	if strings.HasPrefix(header, `"`) {
		fields := splitGitDiffPaths(header)
		if len(fields) != 2 {
			return a, b, false, fmt.Errorf("expected exactly two operands, found %d", len(fields))
		}
		for idx, field := range fields {
			wantPrefix := "a/"
			if idx == 1 {
				wantPrefix = "b/"
			}
			decoded, derr := decodeDiffOperandField(field)
			if derr != nil {
				side := "a-side"
				if idx == 1 {
					side = "b-side"
				}
				return a, b, false, fmt.Errorf("%s operand %s: %v", side, field, derr)
			}
			if !strings.HasPrefix(decoded, wantPrefix) {
				return a, b, false, fmt.Errorf("operand %q lacks the %q prefix (a repository using diff.noprefix or diff.mnemonicPrefix is not supported here)", decoded, wantPrefix)
			}
			payload := strings.TrimPrefix(decoded, wantPrefix)
			if payload == "" {
				return a, b, false, fmt.Errorf("operand %q has an empty path", decoded)
			}
			if idx == 0 {
				a = diffOperand{Prefix: wantPrefix, Path: payload}
			} else {
				b = diffOperand{Prefix: wantPrefix, Path: payload}
			}
		}
		return a, b, true, nil
	}

	// Unquoted. Paths may contain spaces, so the ` b/` boundary is only
	// unambiguous when the a/ and b/ payloads are byte-identical.
	// Renames and copies fail here by construction and are resolved by
	// their dedicated headers — but the operands must still be
	// structurally valid before any fallback is consulted.
	if !strings.HasPrefix(header, "a/") {
		return a, b, false, fmt.Errorf("a-side operand does not start with %q", "a/")
	}
	if strings.Contains(header, `"`) {
		return a, b, false, fmt.Errorf("unquoted operands must not contain a double quote")
	}
	for offset := 0; offset < len(header); {
		rel := strings.Index(header[offset:], " b/")
		if rel < 0 {
			break
		}
		at := offset + rel
		left, right := header[:at], header[at+1:]
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") &&
			strings.TrimPrefix(left, "a/") == strings.TrimPrefix(right, "b/") {
			payload := strings.TrimPrefix(right, "b/")
			if payload == "" {
				return a, b, false, fmt.Errorf("operand %q has an empty path", right)
			}
			return diffOperand{Prefix: "a/", Path: payload},
				diffOperand{Prefix: "b/", Path: payload}, true, nil
		}
		offset = at + len(" b/")
	}
	// Ambiguous but not malformed: a rename/copy of unquoted names.
	// Require the header to at least contain a plausible b-side marker
	// so genuine garbage is still refused.
	if !strings.Contains(header, " b/") {
		return a, b, false, fmt.Errorf("no %q b-side operand", " b/")
	}
	return a, b, false, nil
}

// decodeDiffOperandField decodes one operand field: quoted fields go
// through the Git C decoder, unquoted fields are taken verbatim so that
// whitespace bytes Git permits unquoted are preserved.
func decodeDiffOperandField(field string) (string, error) {
	if field == "" {
		return "", fmt.Errorf("empty operand")
	}
	if field[0] != '"' {
		if strings.Contains(field, `"`) {
			return "", fmt.Errorf("unquoted operand contains a double quote")
		}
		return field, nil
	}
	return unquoteGitCStyle(field)
}

// FilesInPatchStrict returns the b-side path of every file entry in a
// unified diff, decoding Git's C-quoting byte-correctly.
//
// Handled: quoted and unquoted paths, paths containing spaces, tabs,
// newlines and octal-escaped bytes, renames, copies, mode-only entries,
// binary entries, new and deleted files.
//
// Refused (error, nil slice — never a partial or empty scope):
//
//   - non-blank input containing zero `diff --git` headers;
//   - a header whose a-side OR b-side operand is malformed;
//   - a quoted operand using an escape Git does not emit;
//   - an operand without a valid `a/` / `b/` prefix, or with an empty
//     path;
//   - a header that no corroborating line can disambiguate.
//
// Whitespace-only input is NOT an error: it legitimately touches
// nothing. Callers that derive a write scope MUST use this function.
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

	headers := 0
	lines := strings.Split(patch, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		headers++
		header := strings.TrimPrefix(line, "diff --git ")

		// Corroborating headers live between this `diff --git` line and
		// the first hunk (or the next entry). `rename to` / `copy to`
		// and `+++` are unambiguous single-path lines, so they resolve
		// the cases the two-operand header cannot.
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

	if headers == 0 && strings.TrimSpace(patch) != "" {
		return nil, fmt.Errorf("patch contains no `diff --git` header but is not empty (%d byte(s)); refusing to treat it as touching nothing", len(patch))
	}
	return out, nil
}

// resolveDiffEntryPath determines the b-side path of one file entry.
//
// Both operands are validated FIRST. Only once the header itself is
// well-formed — but ambiguous, as an unquoted rename or copy is — are
// the corroborating `rename to` / `copy to` / `+++` / `---` lines
// consulted.
func resolveDiffEntryPath(header, renameTo, copyTo, plusPath, minusPath string) (string, error) {
	_, b, ok, err := parseDiffGitOperands(header)
	if err != nil {
		return "", err
	}
	if ok {
		return b.Path, nil
	}
	for _, candidate := range []string{renameTo, copyTo} {
		if candidate == "" {
			continue
		}
		p, derr := decodeDiffOperandField(candidate)
		if derr != nil {
			return "", derr
		}
		if p != "" {
			return p, nil
		}
	}
	for _, candidate := range []string{plusPath, minusPath} {
		if candidate == "" {
			continue
		}
		p, derr := decodeDiffOperandField(stripDiffTimestamp(candidate))
		if derr != nil {
			return "", derr
		}
		p = stripDiffSidePrefix(p)
		if p != "" && p != "/dev/null" {
			return p, nil
		}
	}
	return "", fmt.Errorf("no unambiguous path in the header or its corroborating lines")
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
