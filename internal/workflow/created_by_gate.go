// Apply-time gate for RecipeOperation.CreatedBy (M14 fix-pass / ADR-011 D4).
//
// Until this file landed, RecipeOperation.CreatedBy was persisted but inert
// — recipe.go ignored it and a `replace-in-file` / `append-file` op against
// a target that a hard parent owns would surface only as the bare
// "file not found" error, with no hint that the operator should apply the
// parent first.
//
// PRD §4.3 contract (authoritative):
//
//   - op.CreatedBy == ""                  → no-op (preserve v0.5.3 behaviour).
//   - flag off (DAGEnabled false)         → no-op (CreatedBy is inert metadata).
//   - hard parent + target missing        → ErrPathCreatedByParent.
//   - hard parent + target exists         → proceed normally.
//   - soft parent + target missing        → emit warning, fall through to
//                                            the bare not-found error.
//   - parent not in child's depends_on    → recipe-shape validation error.
//   - hard parent in upstream_merged
//     state with target present           → proceed (ADR-011 D5).
//
// Soft parents never gate apply (ADR-011 D4) — they are ordering hints only.
//
// Authoritative source guard: this gate reads child status via
// store.LoadFeatureStatus and only inspects DependsOn. It does NOT consult
// any reconcile-session or apply-session artifact.

package workflow

import (
	"errors"
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// ErrPathCreatedByParent is returned by checkCreatedByGate when an op
// declares a hard `created_by` parent and the target file does not exist.
// The operator must apply (or upstream-merge) the parent feature first.
//
// Wrappable via errors.Is. The wrapping fmt.Errorf message carries the
// per-op detail (path + parent slug); callers should not pattern-match on
// the message text — match on this sentinel instead.
var ErrPathCreatedByParent = errors.New("path will be created by parent feature")

// checkCreatedByGate enforces the PRD §4.3 contract. It returns nil when
// the op should proceed to the existing target-must-exist preconditions
// (or, for soft parents with missing targets, when the warning has been
// emitted and the caller should let the bare not-found error surface).
//
// `targetExists` is supplied by the caller because it has already
// computed the absolute path and statted the file — duplicating that here
// would mean two stat calls per op.
//
// `childSlug` is the recipe's owning feature (recipe.Feature). It is
// validated lazily: only when op.CreatedBy is non-empty AND the flag is
// on do we touch the store. This keeps flag-off byte-identity intact.
func checkCreatedByGate(s *store.Store, childSlug string, op RecipeOperation, targetExists bool) error {
	if op.CreatedBy == "" {
		return nil
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		return err
	}
	if !cfg.DAGEnabled() {
		// ADR-011 D9: when the flag is off, CreatedBy is inert metadata.
		// Behaviour is byte-identical to v0.5.3 (bare not-found error).
		return nil
	}

	child, err := s.LoadFeatureStatus(childSlug)
	if err != nil {
		return fmt.Errorf("created_by gate: cannot load feature status for %q: %w", childSlug, err)
	}

	var (
		kind  string
		found bool
	)
	for _, dep := range child.DependsOn {
		if dep.Slug == op.CreatedBy {
			kind = dep.Kind
			found = true
			break
		}
	}
	if !found {
		// Recipe-shape validation: an op cannot reference a parent the
		// child does not declare a dependency on. Surfaced at recipe-load
		// time inside dryRunOperation/executeOperation per the M14
		// fix-pass scope; the implement-phase writer will eventually
		// enforce this earlier (separate backlog item).
		return fmt.Errorf("recipe op declares created_by=%s but %s is not in depends_on", op.CreatedBy, op.CreatedBy)
	}

	if targetExists {
		// Hard or soft, when the target is already present the gate is
		// satisfied. For hard parents in upstream_merged state this is
		// the path that allows the op through (ADR-011 D5).
		return nil
	}

	switch kind {
	case store.DependencyKindHard:
		return fmt.Errorf("%w: path %s will be created by parent feature %s; apply %s first",
			ErrPathCreatedByParent, op.Path, op.CreatedBy, op.CreatedBy)
	case store.DependencyKindSoft:
		// Soft deps never gate apply (ADR-011 D4). Emit a warning so the
		// operator knows the recipe was written with parent-ordering in
		// mind, then fall through to the existing not-found error.
		fmt.Fprintf(WarnWriter, "note: op declares created_by=%s; soft deps do not gate apply\n", op.CreatedBy)
		return nil
	default:
		// Unknown dependency kind — be conservative and let the existing
		// validation flag this rather than silently passing.
		return fmt.Errorf("recipe op declares created_by=%s with unknown dependency kind %q", op.CreatedBy, kind)
	}
}
