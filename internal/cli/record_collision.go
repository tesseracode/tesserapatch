package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// collisionMatch describes a single byte-identical existing
// post-apply.patch found by the record-time collision scan.
type collisionMatch struct {
	Slug   string
	Path   string // relative to repo root, for diagnostics
	SHA256 string
	Bytes  int
	Files  int
}

// collisionScanResult bundles the matches found and the
// classification per PRD-record-collision-detection §4 steps 5–6.
type collisionScanResult struct {
	NewSHA256    string
	NewBytes     int
	SameFeature  bool // current slug already has a byte-identical post-apply.patch
	CrossFeature []collisionMatch
}

// scanCanonicalPatchCollisions enumerates every feature's
// artifacts/post-apply.patch (skipping missing files) and reports
// byte-identical matches against the proposed patch. The current slug
// is split out into SameFeature / CrossFeature buckets so the caller
// can apply distinct policies per PRD §3.
//
// Comparison ladder per PRD §4: byte length → SHA-256 → byte-for-byte.
// The byte-for-byte step is defence-in-depth against a SHA-256 false
// positive: cryptographically negligible, but cheap when length+digest
// already match.
//
// Numbered audit snapshots under patches/ are intentionally NOT scanned:
// they may legitimately repeat (PRD §7).
func scanCanonicalPatchCollisions(s *store.Store, currentSlug, newPatch string) (collisionScanResult, error) {
	res := collisionScanResult{}
	res.NewSHA256, res.NewBytes = gitutil.PatchSignature(newPatch)

	features, err := s.ListFeatures()
	if err != nil {
		return res, err
	}
	slugs := make([]string, 0, len(features))
	for _, f := range features {
		slugs = append(slugs, f.Slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		patchPath := filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
		st, statErr := os.Stat(patchPath)
		if statErr != nil {
			// PRD §4 step 4a: skip missing files silently. Other
			// stat errors (permission denied, ...) are also skipped
			// here — record cannot reliably reason about a feature
			// directory whose canonical patch is unreadable, and
			// false-positive blocking is worse than missing a
			// pathological collision.
			continue
		}
		if int64(res.NewBytes) != st.Size() {
			continue
		}
		raw, readErr := os.ReadFile(patchPath)
		if readErr != nil {
			continue
		}
		existingSHA, _ := gitutil.PatchSignature(string(raw))
		if existingSHA != res.NewSHA256 {
			continue
		}
		// Defence in depth: digest equality is statistically
		// definitive, but we still byte-compare so a hypothetical
		// hash collision (or a future signature change) cannot
		// false-positive a refusal.
		if string(raw) != newPatch {
			continue
		}
		m := collisionMatch{
			Slug:   slug,
			Path:   filepath.Join(".tpatch", "features", slug, "artifacts", "post-apply.patch"),
			SHA256: res.NewSHA256,
			Bytes:  res.NewBytes,
			Files:  countPatchFiles(string(raw)),
		}
		if slug == currentSlug {
			res.SameFeature = true
			continue
		}
		res.CrossFeature = append(res.CrossFeature, m)
	}
	return res, nil
}

// printCollisionRefusal writes the cross-feature refusal diagnostic to
// w in PRD §3.1 format, tailored to the capture mode that produced the
// collision (PRD §5).
func printCollisionRefusal(w io.Writer, slug string, matches []collisionMatch, captureMode, filesFlag string) {
	fmt.Fprintf(w, "Error: recorded patch for %q is byte-identical to existing feature patch(es):\n", slug)
	for _, m := range matches {
		short := m.SHA256
		if len(short) > 12 {
			short = short[:12]
		}
		fmt.Fprintf(w, "  - %s  sha256=%s... bytes=%d files=%d\n", m.Slug, short, m.Bytes, m.Files)
	}
	fmt.Fprintln(w)

	hasFiles := strings.TrimSpace(filesFlag) != ""

	// PRD §5: when the collision spans many existing features, the
	// operator is almost certainly looking at an already-corrupted
	// store; recommend stopping rather than stamping another
	// feature directory.
	if len(matches) >= 3 {
		fmt.Fprintln(w, "Multiple existing features already match these bytes. Consider stopping")
		fmt.Fprintln(w, "and using future feature import/resplit tooling rather than stamping another")
		fmt.Fprintln(w, "feature directory.")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "This usually means the record range is too broad.")
	fmt.Fprintln(w, "Try one of:")

	switch captureMode {
	case "auto committed range":
		// PRD §5: --auto without --files — auto-base found a base
		// but cannot infer feature ownership.
		if hasFiles {
			fmt.Fprintf(w, "  tpatch record %s --auto --files <narrower-paths>\n", slug)
		} else {
			fmt.Fprintf(w, "  tpatch record %s --auto --files <feature-paths>\n", slug)
		}
		fmt.Fprintf(w, "  tpatch record %s --from <feature-base> --files <feature-paths>\n", slug)
		fmt.Fprintf(w, "  tpatch record %s --from <feature-base> --to <feature-tip>\n", slug)
	case "explicit committed range":
		// PRD §5: --commit-range without --files — suggest narrowing
		// by pathspec.
		fmt.Fprintf(w, "  tpatch record %s --commit-range <a>..<b> --files <feature-paths>\n", slug)
		fmt.Fprintf(w, "  tpatch record %s --from <feature-base> --to <feature-tip>\n", slug)
	case "committed range":
		// --from (with or without --to). The range may include
		// unrelated commits.
		fmt.Fprintf(w, "  tpatch record %s --auto --files <feature-paths>\n", slug)
		fmt.Fprintf(w, "  tpatch record %s --from <feature-base> --files <feature-paths>\n", slug)
		fmt.Fprintf(w, "  tpatch record %s --from <base> --to <feature-tip>\n", slug)
	default:
		// Working-tree capture — the user may have multiple feature
		// edits in the working tree.
		fmt.Fprintf(w, "  tpatch record %s --files <feature-paths>\n", slug)
		fmt.Fprintln(w, "  # or split unrelated working-tree edits into separate features before recording")
		fmt.Fprintf(w, "  tpatch record %s --auto   # if the feature has already been committed\n", slug)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "To accept an intentional duplicate, rerun with:")
	fmt.Fprintf(w, "  tpatch record %s --allow-collision \"<reason>\"\n", slug)
}
