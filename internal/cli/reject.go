package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ─── tpatch reject / tpatch reopen ───────────────────────────────────────────
//
// v0.13.0 GH #6. Contract sources:
//   - docs/prds/PRD-rejected-feature-state.md (§4 CLI shape, §5 state
//     machine, §6 required fields, §7 integration semantics, §8 JSON
//     envelope)
//   - docs/adrs/ADR-031-rejected-feature-state-data-model.md
//     (D1 location, D2 closed reason enum, D3 evidence content-hash,
//     D4 exit codes, D5 append-only reopen, D8 edge guard, D9 actor,
//     D10 verb-collision disposition)
//
// Exit-code envelope (ADR-031 D4 addendum), shared by every command
// this feature touches:
//
//	0  success
//	1  unexpected internal error
//	2  pre-mutation input validation (bad reason, empty note, evidence
//	   that cannot be resolved/hashed, path-safety violation)
//	3  post-validation state-machine refusal (wrong source state, live
//	   dependents, already rejected, reopen of a non-rejected feature,
//	   edge onto a rejected parent, confirm-upstreamed on a rejected
//	   feature)

const (
	exitValidation = 2
	exitStateRefus = 3
)

// rejectReconcileDisambiguation is the one-line cross-reference that
// MUST appear in `tpatch reject --help` (PRD §4.1 mitigation 1,
// ADR-031 D10; asserted by test 27).
const rejectReconcileDisambiguation = "Not to be confused with `tpatch reconcile --reject <slug>`, which prunes a shadow worktree " +
	"(a transient action on a shadow-worktree resource) — see PRD §4.1 / ADR-031 D10."

// reconcileRejectDisambiguation is the symmetric cross-reference that
// MUST appear in `tpatch reconcile --help` (PRD §4.1 mitigation 1,
// ADR-031 D10; asserted by test 27).
const reconcileRejectDisambiguation = "Not to be confused with `tpatch reject <slug>`, which marks a feature as permanently rejected " +
	"(a terminal lifecycle transition on the feature itself) — see PRD §4.1 / ADR-031 D10."

// validationError builds an exit-2 error.
func validationError(format string, a ...any) error {
	return &ExitCodeError{Code: exitValidation, Message: fmt.Sprintf(format, a...)}
}

// stateRefusalError builds an exit-3 error.
func stateRefusalError(format string, a ...any) error {
	return &ExitCodeError{Code: exitStateRefus, Message: fmt.Sprintf(format, a...)}
}

// ─── evidence resolution + hashing (ADR-031 D3 addendum) ─────────────────────

// evidenceHashFn is a package-level seam so tests can observe (or stub)
// every byte-read of an evidence file. Test 21's file-kind-change case
// asserts that a historical evidence path which fails the path-safety
// re-check is NEVER hashed — that assertion needs this hook.
var evidenceHashFn = sha256File

// sha256File returns the lowercase-hex SHA-256 of the file's raw bytes.
func sha256File(abs string) (string, error) {
	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// normalizeEvidencePath applies the path-safety rule's syntactic half
// (ADR-031 D3 addendum): absolute paths are rejected, `..` traversal
// segments are rejected post-Clean, and the accepted value is
// normalized to a forward-slash relative path so status.json
// serialization is deterministic across platforms.
func normalizeEvidencePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("evidence path must not be empty")
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("evidence path %q is absolute; supply a path relative to the feature directory or the repository root", raw)
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("evidence path %q escapes the repository root via `..`", raw)
	}
	if cleaned == "." {
		return "", fmt.Errorf("evidence path %q does not name a file", raw)
	}
	return cleaned, nil
}

// resolveEvidence locates a normalized evidence path on disk and
// classifies any failure using the closed divergence taxonomy.
//
// Resolution order (ADR-031 D3): first relative to the feature
// directory `.tpatch/features/<slug>/`, then relative to the repository
// root. The returned reason is "" when the path resolved to a readable
// regular file inside the repository root.
func resolveEvidence(root, slug, normalized string) (abs string, reason string) {
	// The repository root itself may sit under a symlinked prefix
	// (macOS `/var` → `/private/var` is the common case), so both sides
	// of the containment check must be symlink-resolved or every
	// evidence path would look like an escape.
	safeRoot := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		safeRoot = r
	}
	candidates := []string{
		filepath.Join(root, ".tpatch", "features", slug, filepath.FromSlash(normalized)),
		filepath.Join(root, filepath.FromSlash(normalized)),
	}

	var sawNonRegular bool
	for _, cand := range candidates {
		// Path-safety must be evaluated on the SYMLINK-RESOLVED path so
		// a symlink escaping the repository root is caught before any
		// byte of the target is read.
		resolved, err := filepath.EvalSymlinks(cand)
		if err != nil {
			continue
		}
		if safety.EnsureSafeRepoPath(safeRoot, resolved) != nil {
			return "", store.DivergentReasonPathSafetyFailed
		}
		info, err := os.Stat(resolved)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			sawNonRegular = true
			continue
		}
		return resolved, ""
	}
	if sawNonRegular {
		return "", store.DivergentReasonNonRegular
	}
	return "", store.DivergentReasonMissing
}

// collectEvidence normalizes, resolves, hashes, deduplicates and sorts
// the operator-supplied `--evidence` paths. Any failure is a
// pre-mutation validation error (exit 2).
func collectEvidence(root, slug string, raw []string) ([]store.EvidenceRef, error) {
	byPath := make(map[string]store.EvidenceRef, len(raw))
	for _, r := range raw {
		normalized, err := normalizeEvidencePath(r)
		if err != nil {
			return nil, validationError("evidence path %v", err)
		}
		abs, reason := resolveEvidence(root, slug, normalized)
		if reason != "" {
			return nil, validationError("evidence path %q could not be read: %s", normalized, evidenceFailureDetail(reason))
		}
		sum, herr := evidenceHashFn(abs)
		if herr != nil {
			return nil, validationError("evidence path %q could not be read: %v", normalized, herr)
		}
		byPath[normalized] = store.EvidenceRef{Path: normalized, SHA256: sum}
	}
	out := make([]store.EvidenceRef, 0, len(byPath))
	for _, e := range byPath {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// evidenceFailureDetail renders a divergence reason as operator-facing
// prose for the exit-2 validation message.
func evidenceFailureDetail(reason string) string {
	switch reason {
	case store.DivergentReasonMissing:
		return "no such file or directory"
	case store.DivergentReasonNonRegular:
		return "not a regular file"
	case store.DivergentReasonPathSafetyFailed:
		return "resolves outside the repository root"
	case store.DivergentReasonUnreadable:
		return "permission denied"
	default:
		return reason
	}
}

// ─── reject ──────────────────────────────────────────────────────────────────

func rejectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <slug>",
		Short: "Mark a feature permanently rejected (terminal, pre-implementation)",
		Long: "Mark a feature permanently rejected.\n\n" +
			"`rejected` is a terminal, PRE-implementation lifecycle outcome: it records that this\n" +
			"feature should never be implemented. The complete feature directory is preserved and\n" +
			"the rejection is appended to an append-only audit history — nothing is deleted.\n\n" +
			"Allowed source states: " + joinStates(store.RejectableStateList()) + ".\n" +
			"Refused (exit 3) from implementing, applied, active, reconciling, reconciling-shadow,\n" +
			"blocked and upstream_merged — post-implementation retirement is out of scope.\n\n" +
			"Reason codes (closed enum): " + store.RejectionReasonsJoined() + ".\n\n" +
			"Reverse with `tpatch reopen <slug> --note <string>`.\n\n" +
			rejectReconcileDisambiguation + "\n\n" +
			"Exit codes: 0 success, 1 unexpected error, 2 validation error, 3 state-machine refusal.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReject(cmd, args[0])
		},
	}
	cmd.Flags().String("reason", "", "Rejection reason code (required; one of: "+store.RejectionReasonsJoined()+")")
	cmd.Flags().String("note", "", "Human-readable rationale (required, non-empty)")
	cmd.Flags().StringArray("evidence", nil, "Evidence file path, repeatable (at least one required). Resolved against the feature directory first, then the repository root")
	cmd.Flags().String("actor", "", "Override the resolved actor (default: $TPATCH_ACTOR, then `git config user.email`, then \"unknown\")")
	cmd.Flags().String("related", "", "Optional related reference: a feature slug or a GH#N pointer")
	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func runReject(cmd *cobra.Command, slug string) error {
	reason, _ := cmd.Flags().GetString("reason")
	note, _ := cmd.Flags().GetString("note")
	rawEvidence, _ := cmd.Flags().GetStringArray("evidence")
	actorFlag, _ := cmd.Flags().GetString("actor")
	related, _ := cmd.Flags().GetString("related")
	asJSON, _ := cmd.Flags().GetBool("json")

	emit := func(payload map[string]any, err error) error {
		if asJSON {
			payload["slug"] = slug
			payload["exit_code"] = exitCodeOf(err)
			if err != nil {
				payload["error"] = err.Error()
			}
			data, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
		}
		return err
	}

	// ─── exit 2: pre-mutation input validation ───────────────────────
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return emit(map[string]any{}, validationError("reason required: --reason must be one of %s", store.RejectionReasonsJoined()))
	}
	if !store.IsValidRejectionReason(reason) {
		return emit(map[string]any{}, validationError("invalid reason %q: must be one of %s", reason, store.RejectionReasonsJoined()))
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return emit(map[string]any{}, validationError("note required: --note must be a non-empty rationale string"))
	}
	if len(rawEvidence) == 0 {
		return emit(map[string]any{}, validationError("evidence required: at least one --evidence path must be supplied"))
	}

	s, err := openStoreFromCmd(cmd)
	if err != nil {
		return err
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return emit(map[string]any{}, validationError("feature %q not found: %v", slug, err))
	}

	// ─── exit 3: state-machine refusals ──────────────────────────────
	if status.State == store.StateRejected {
		return emit(map[string]any{"state": string(status.State)},
			stateRefusalError("cannot reject feature %q: it is already rejected; run `tpatch reopen %s` first if you need to re-record the rejection with different fields", slug, slug))
	}
	if !store.IsRejectableState(status.State) {
		return emit(map[string]any{"state": string(status.State)},
			stateRefusalError("cannot reject feature %q from state %q: reject is only valid from %s", slug, status.State, joinStates(store.RejectableStateList())))
	}
	if deps := dependentEdges(s, slug); len(deps) > 0 {
		rendered := make([]string, len(deps))
		jsonDeps := make([]map[string]string, len(deps))
		for i, d := range deps {
			rendered[i] = fmt.Sprintf("%s (kind=%s)", d.slug, d.kind)
			jsonDeps[i] = map[string]string{"slug": d.slug, "kind": d.kind}
		}
		return emit(map[string]any{"state": string(status.State), "dependents": jsonDeps},
			stateRefusalError("cannot reject feature %q: %d dependent feature(s) still reference it: %s. Remove the dependency edge from each dependent (`tpatch feature deps <dependent> remove %s`) before rejecting",
				slug, len(deps), strings.Join(rendered, ", "), slug))
	}

	// ─── exit 2: evidence resolution + content hashing ───────────────
	evidence, eerr := collectEvidence(s.Root, slug, rawEvidence)
	if eerr != nil {
		return emit(map[string]any{}, eerr)
	}

	// ─── mutate ──────────────────────────────────────────────────────
	priorState := status.State
	actor := store.ResolveActorIn(actorFlag, s.Root)
	now := time.Now().UTC().Truncate(time.Second)

	// `reject` appends NOTHING to RejectionHistory: a history entry
	// records one COMPLETED reject→reopen cycle and is appended by the
	// reopen that closes it (PRD §6, ADR-031 D5). The live record below
	// is the rejection half until then.
	status.Rejection = &store.RejectionStatus{
		Reason:     reason,
		Note:       note,
		Actor:      actor,
		Related:    strings.TrimSpace(related),
		Evidence:   evidence,
		RejectedAt: now,
		PriorState: priorState,
	}
	status.State = store.StateRejected
	status.LastCommand = "reject"
	status.UpdatedAt = now.Format(time.RFC3339)
	if err := s.SaveFeatureStatus(status); err != nil {
		return err
	}

	if asJSON {
		payload := map[string]any{
			"state":       string(store.StateRejected),
			"prior_state": string(priorState),
			"reason":      reason,
			"evidence":    evidence,
			"note":        note,
			"rejected_at": now.Format(time.RFC3339),
			"rejected_by": actor,
			"related":     nullableString(related),
		}
		return emit(payload, nil)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Rejected %s (reason=%s); state %s → rejected\n", slug, reason, priorState)
	for _, e := range evidence {
		fmt.Fprintf(out, "  evidence: %s (sha256=%s)\n", e.Path, e.SHA256)
	}
	fmt.Fprintf(out, "Reopen with: tpatch reopen %s --note \"<why this is being reconsidered>\"\n", slug)
	return nil
}

// ─── status --json DTO (PRD §8) ──────────────────────────────────────────────
//
// `tpatch status --json` must not marshal store.RejectionStatus
// directly: its Go-side field names (`actor`, `note`) are internal and
// do not match the wire contract PRD §8 fixes (`rejected_by`, `note`).
// These view types are the sole renderer of the `rejection` object, and
// they are deliberately decoupled from the store struct so a future
// internal rename cannot silently break the envelope (Cluster F' rev-1,
// F-INT-2).

// evidenceRefView is the PRD §8 wire form of one evidence element.
type evidenceRefView struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// rejectionStatusView is the PRD §8 `rejection` object. Field order and
// names are taken verbatim from the §8 `tpatch status --json` example.
type rejectionStatusView struct {
	Reason     string             `json:"reason"`
	Evidence   []evidenceRefView  `json:"evidence,omitempty"`
	Note       string             `json:"note"`
	RejectedAt time.Time          `json:"rejected_at"`
	RejectedBy string             `json:"rejected_by"`
	PriorState store.FeatureState `json:"prior_state"`
	Related    any                `json:"related"`
}

// newRejectionStatusView renders the `rejection` object for a feature,
// or nil when it must be omitted. PRD §8: the object is present ONLY
// when `state == "rejected"`. After a reopen the live record is cleared
// and the feature is back on its own lifecycle, so no `rejection`
// object is emitted — the completed cycle is in `rejection_history`.
func newRejectionStatusView(f store.FeatureStatus) *rejectionStatusView {
	if f.State != store.StateRejected || f.Rejection == nil {
		return nil
	}
	r := f.Rejection
	ev := make([]evidenceRefView, 0, len(r.Evidence))
	for _, e := range r.Evidence {
		ev = append(ev, evidenceRefView{Path: e.Path, SHA256: e.SHA256})
	}
	if len(ev) == 0 {
		ev = nil
	}
	return &rejectionStatusView{
		Reason:     r.Reason,
		Evidence:   ev,
		Note:       r.Note,
		RejectedAt: r.RejectedAt,
		RejectedBy: r.Actor,
		PriorState: r.PriorState,
		Related:    nullableString(r.Related),
	}
}

// ─── shared helpers ──────────────────────────────────────────────────────────

// exitCodeOf maps an error to the JSON envelope's `exit_code` field.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if e := asExitCodeError(err); e != nil {
		return e.ExitCode()
	}
	return 1
}

// nullableString renders "" as a JSON null so the envelope matches
// PRD §8 (`"related": null` when unset).
func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

// nonNilEvidence guarantees `reopen_evidence` serializes as `[]` rather
// than `null` on a note-only reopen (PRD §8's note-only example).
func nonNilEvidence(v []store.EvidenceRef) []store.EvidenceRef {
	if v == nil {
		return []store.EvidenceRef{}
	}
	return v
}

func joinStates(states []store.FeatureState) string {
	parts := make([]string, len(states))
	for i, st := range states {
		parts[i] = string(st)
	}
	return strings.Join(parts, ", ")
}

// refuseIfRejected is the shared precondition used by `apply` and
// `reconcile` to keep a rejected feature out of the apply/reconcile
// pipelines by default (PRD §3.6 / §7). Returns nil for every other
// state, including when status.json cannot be read (the caller's own
// error handling takes over).
func refuseIfRejected(s *store.Store, slug, verb string) error {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil || status.State != store.StateRejected {
		return nil
	}
	reason := ""
	if status.Rejection != nil {
		reason = status.Rejection.Reason
	}
	return stateRefusalError("cannot %s feature %q: feature is rejected (reason=%s); run `tpatch reopen %s` to resume work on it",
		verb, slug, reason, slug)
}
