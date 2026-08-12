package workflow

// Recipe-operation presence PREDICATES — diagnostics only (ADR-013 D13).
//
// These predicates never certify presence: the patch ladder (D12) is the
// sole authority. They exist to localise which operation and path a
// failure concerns, and to feed the D15 write-file signals.
//
// The `replace-in-file` predicate is the existential-inverse form
// measured in ADR-013's amendment table: 0 false reds and 0 false greens
// over 52 416 decided cases, with 4 368 undecidable.

import (
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// OpPresenceVerdict is the closed outcome set of a diagnostic predicate.
type OpPresenceVerdict int

const (
	// OpPresenceUnsupported — the predicate cannot be expressed for this
	// operation shape (e.g. an empty `search`).
	OpPresenceUnsupported OpPresenceVerdict = iota
	// OpPresenceUndecidable — every content admits a preimage, so the
	// judgement defers to patch authority.
	OpPresenceUndecidable
	// OpPresentLikely — the operation's postimage is consistent with the
	// observed content. NEVER a certification.
	OpPresentLikely
	// OpAbsentLikely — the observed content is inconsistent with the
	// operation's postimage.
	OpAbsentLikely
)

func (v OpPresenceVerdict) String() string {
	switch v {
	case OpPresenceUnsupported:
		return "unsupported"
	case OpPresenceUndecidable:
		return "undecidable"
	case OpPresentLikely:
		return "present-likely"
	case OpAbsentLikely:
		return "absent-likely"
	}
	return "unknown"
}

// DiagnoseOpPresence evaluates one recipe operation against observed
// content. `exists` and `isDir` describe the target path; `content` is
// its bytes when it exists.
//
// The result is DIAGNOSTIC. Callers must never use it to skip a member,
// certify materialization, or flip a check's verdict.
func DiagnoseOpPresence(op RecipeOperation, content []byte, exists, isDir bool) OpPresenceVerdict {
	switch op.Type {
	case "write-file":
		if !exists {
			return OpAbsentLikely
		}
		if string(content) == op.Content {
			return OpPresentLikely
		}
		return OpAbsentLikely
	case "append-file":
		if op.Content == "" {
			// Every content ends with the empty string.
			return OpPresenceUndecidable
		}
		if !exists {
			return OpAbsentLikely
		}
		if strings.HasSuffix(string(content), op.Content) {
			return OpPresentLikely
		}
		return OpAbsentLikely
	case "replace-in-file":
		if !exists {
			return OpAbsentLikely
		}
		return replaceInFilePresence(string(content), op.Search, op.Replace)
	case "ensure-directory":
		if exists && isDir {
			return OpPresentLikely
		}
		return OpAbsentLikely
	default:
		return OpPresenceUnsupported
	}
}

// replaceInFilePresence is the existential-inverse predicate.
//
//   - `search == ""` is UNSUPPORTED: Go's strings.Replace inserts at the
//     start for an empty search, so no meaningful inverse exists.
//   - `replace == ""` is UNDECIDABLE: every content admits the preimage
//     `search + content`.
//   - otherwise the predicate is true iff there is an index i at which
//     `replace` occurs in the content such that reconstructing the
//     preimage there and re-applying the replacement reproduces the
//     content exactly. Every occurrence of `replace` is tried.
func replaceInFilePresence(content, search, replace string) OpPresenceVerdict {
	if search == "" {
		return OpPresenceUnsupported
	}
	if replace == "" {
		return OpPresenceUndecidable
	}
	for i := 0; i+len(replace) <= len(content); i++ {
		if content[i:i+len(replace)] != replace {
			continue
		}
		pre := content[:i] + search + content[i+len(replace):]
		if strings.Replace(pre, search, replace, 1) == content {
			return OpPresentLikely
		}
	}
	return OpAbsentLikely
}

// diagnosePathForBlockedLadder returns the first path whose operation is
// NOT consistent with the tree the ladder blocked at, or "" when no
// operation localises the failure. Used only to fill the `<path>` slot
// of R1/R2 when the `--verbose` output did not name one.
func diagnosePathForBlockedLadder(ctx *verifyRunContext, treeish string, recipe ApplyRecipe) string {
	for _, op := range recipe.Operations {
		if op.Path == "" {
			continue
		}
		data, found, _, err := gitutilBlobAtTree(ctx, treeish, op.Path)
		if err != nil {
			continue
		}
		switch DiagnoseOpPresence(op, data, found, false) {
		case OpAbsentLikely:
			return op.Path
		}
	}
	return ""
}

func gitutilBlobAtTree(ctx *verifyRunContext, treeish, path string) ([]byte, bool, string, error) {
	return gitutil.BlobAtTree(ctx.root, treeish, path)
}
