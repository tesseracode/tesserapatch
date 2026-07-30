package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// sessionCmd is the new `tpatch session` command group proposed by
// PRD-active-feature-session §6 D13. Subcommands:
//
//	tpatch session start <slug> [--capture-context=summary|local-events] [--label <text>]
//	tpatch session stop <slug> [--session <cs_id>]
//	tpatch session list [<slug>] [--json]
//	tpatch session summarize <slug> [--session <cs_id>] [--dry-run] [--write] [--json]
//	tpatch session purge [<slug>|--all] [--session <cs_id>] [--dry-run] [--yes]
//
// Rule 11 note: the root `--path <dir>` persistent flag is inherited
// by every subcommand; help text on each subcommand lists ONLY
// subcommand-specific flags (PRD §8.21).
func sessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage active feature sessions (PRD-active-feature-session)",
		Long: "Manage active feature sessions.\n\n" +
			"Sessions live in the LOCAL private buffer lane at .tpatch/local/capture/ per\n" +
			"PRD-active-feature-session §4 D5 and ADR-027 D1. .tpatch/local/ MUST be gitignored\n" +
			"before the first write; `tpatch init` installs the rule and `session start`\n" +
			"re-verifies via `git check-ignore` at start time (PRD §4 D6 six mandates).\n\n" +
			"Sessions are FEATURE-SCOPED. A session for feature A cannot observe feature B's\n" +
			"buffer (PRD §7 D18). Promotion of a redacted summary to the committed lane at\n" +
			".tpatch/features/<slug>/artifacts/context/<ctx_id>.json is EXPLICIT and OPT-IN\n" +
			"per PRD §5 D9.\n\n" +
			"Subcommands: start, stop, list, summarize, purge. Root persistent flag --path\n" +
			"is inherited; the flag tables below list subcommand-specific flags only.",
	}
	cmd.AddCommand(
		sessionStartCmd(),
		sessionStopCmd(),
		sessionListCmd(),
		sessionSummarizeCmd(),
		sessionPurgeCmd(),
	)
	return cmd
}

// ─── session start ───────────────────────────────────────────────────────────

func sessionStartCmd() *cobra.Command {
	var captureMode string
	var label string
	cmd := &cobra.Command{
		Use:   "start <slug>",
		Short: "Start an active session for a feature",
		Long: "Start an active session for a feature.\n\n" +
			"Idempotent (PRD §3 D1.5): starting when the same feature already has one\n" +
			"active session prints the existing cs_<12hex> and writes no new buffer.\n" +
			"Session IDs are content-addressed per PRD §3 D3.\n\n" +
			"Before writing, `session start` verifies the concrete resolved path is\n" +
			"effectively ignored via `git check-ignore` (PRD §4 D6 mandates 3+4+5).\n" +
			"Refuses when Git is unavailable OR the path is not ignored.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			// Feature must exist (PRD §3 D1.1).
			if _, err := s.LoadFeatureStatus(slug); err != nil {
				return fmt.Errorf("session start: %w", err)
			}
			mode := store.SessionCaptureMode(captureMode)
			if !store.ValidSessionCaptureMode(mode) {
				return fmt.Errorf("session start: --capture-context must be one of summary|local-events (got %q)", captureMode)
			}

			// D18 idempotence: reuse an existing active session for the
			// same feature if any is already open. Enforce single-active
			// invariant at write time.
			existing, err := s.ListSessions(slug)
			if err != nil {
				return fmt.Errorf("session start: %w", err)
			}
			var activeCount int
			var activeID string
			for _, e := range existing {
				if e.Session != nil && e.Session.State == store.SessionActive {
					activeCount++
					activeID = e.Session.ID
				}
			}
			if activeCount > 1 {
				return fmt.Errorf("session start: feature %q already has %d active sessions; run `tpatch session list %s` and close extras", slug, activeCount, slug)
			}

			// PRD §4 D6 mandates 3+4+5: verify the resolved concrete
			// path is effectively ignored BEFORE writing anything.
			sessionRoot := s.LocalCaptureDir()
			if err := workflow.EnsureLocalIgnoreContract(s.Root, sessionRoot); err != nil {
				return err
			}

			if activeCount == 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "session %s already active for feature %s (idempotent, no new buffer written)\n", activeID, slug)
				return nil
			}

			// Compute cs_<12hex> from the identity inputs. Wall-clock
			// deliberately excluded per PRD §3 D3.3.
			baseCommit := ""
			if head, err := gitutil.HeadCommit(s.Root); err == nil {
				baseCommit = strings.TrimSpace(head)
			}
			// A stable per-workspace discriminator makes clones/worktrees
			// yield distinct cs_ IDs without leaking absolute paths into
			// the manifest fields themselves. filepath.Base of Root keeps
			// the ID stable across `git clone` re-clones of the same
			// repository under the same directory name.
			discriminator := filepath.Base(s.Root)
			id := store.ComputeSessionID(store.SessionIdentityInputs{
				SchemaVersion:          store.SessionSchemaVersion,
				RepositoryIdentity:     baseCommit,
				Feature:                slug,
				BaseCommit:             baseCommit,
				CaptureMode:            string(mode),
				WorkspaceDiscriminator: discriminator,
			})

			sess := store.Session{
				SchemaVersion: store.SessionSchemaVersion,
				ID:            id,
				Feature:       slug,
				State:         store.SessionActive,
				CaptureMode:   mode,
				BaseCommit:    baseCommit,
				Label:         label,
			}
			if err := s.SaveSession(sess); err != nil {
				return fmt.Errorf("session start: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "started session %s for feature %s (capture=%s, state=active)\n", id, slug, mode)
			fmt.Fprintf(cmd.OutOrStdout(), "  local buffer: %s\n", s.SessionManifestPath(slug, id))
			return nil
		},
	}
	cmd.Flags().StringVar(&captureMode, "capture-context", string(store.SessionCaptureSummary), "Local capture mode: summary|local-events (PRD §6 D14)")
	cmd.Flags().StringVar(&label, "label", "", "Local-only label (never promoted without redaction)")
	return cmd
}

// ─── session stop ────────────────────────────────────────────────────────────

func sessionStopCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "stop <slug>",
		Short: "Close an active session (no committed writes)",
		Long: "Close an active session for the given feature.\n\n" +
			"Transitions state from active -> closed. No committed writes occur\n" +
			"(PRD §3 D2). Idempotent per PRD §8.11 — stopping an already-closed\n" +
			"session succeeds without change.\n\n" +
			"When multiple eligible sessions exist, --session <cs_id> is required.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			target, err := pickSessionForOp(s, slug, sessionID, sessionEligibleForStop)
			if err != nil {
				return fmt.Errorf("session stop: %w", err)
			}
			if target.State == store.SessionClosed || target.State == store.SessionPromoted || target.State == store.SessionPurged {
				fmt.Fprintf(cmd.OutOrStdout(), "session %s already %s (idempotent)\n", target.ID, target.State)
				return nil
			}
			target.State = store.SessionClosed
			if err := s.SaveSession(target); err != nil {
				return fmt.Errorf("session stop: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "closed session %s for feature %s\n", target.ID, slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Select a specific cs_<12hex> when multiple are eligible")
	return cmd
}

// ─── session list ────────────────────────────────────────────────────────────

// SessionListJSON is the deterministic --json shape for `session list`
// per PRD §6 D14 (sorted by feature slug then session ID, no wall-clock
// timestamps).
type SessionListJSON struct {
	SchemaVersion string            `json:"schema_version"`
	Sessions      []SessionListItem `json:"sessions"`
}

// SessionListItem is one row in the list.
type SessionListItem struct {
	Feature       string `json:"feature"`
	SessionID     string `json:"session_id"`
	State         string `json:"state"`
	CaptureMode   string `json:"capture_mode"`
	PromotedCtxID string `json:"promoted_ctx_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

func sessionListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list [slug]",
		Short: "List local sessions (deterministic, no wall-clock)",
		Long: "List local sessions.\n\n" +
			"Human output lists feature, cs_<12hex>, state, capture mode, and any\n" +
			"promoted ctx_<12hex>. --json emits deterministic schema-versioned JSON\n" +
			"sorted by (feature slug, session ID) per PRD §6 D14 + §8.15. Local\n" +
			"buffer bodies are NEVER printed (PRD §7 D16).\n\n" +
			"Malformed manifests are reported as isolated entries; they do not\n" +
			"abort listing unrelated sessions (PRD §8.16).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}
			entries, err := s.ListSessions(filter)
			if err != nil {
				return fmt.Errorf("session list: %w", err)
			}
			out := cmd.OutOrStdout()
			if asJSON {
				items := make([]SessionListItem, 0, len(entries))
				for _, e := range entries {
					item := SessionListItem{Feature: e.Slug, SessionID: e.SessionID}
					if e.Err != nil {
						item.Error = e.Err.Error()
					} else if e.Session != nil {
						item.Feature = e.Session.Feature
						item.State = string(e.Session.State)
						item.CaptureMode = string(e.Session.CaptureMode)
						item.PromotedCtxID = e.Session.PromotedCtxID
					}
					items = append(items, item)
				}
				payload := SessionListJSON{SchemaVersion: store.SessionSchemaVersion, Sessions: items}
				data, err := json.MarshalIndent(payload, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "no sessions")
				return nil
			}
			fmt.Fprintf(out, "%-40s %-18s %-10s %-13s %s\n", "FEATURE", "SESSION", "STATE", "CAPTURE", "PROMOTED")
			for _, e := range entries {
				if e.Err != nil {
					fmt.Fprintf(out, "%-40s %-18s %-10s %-13s <error: %s>\n", e.Slug, e.SessionID, "?", "?", e.Err.Error())
					continue
				}
				sess := e.Session
				promoted := sess.PromotedCtxID
				if promoted == "" {
					promoted = "-"
				}
				fmt.Fprintf(out, "%-40s %-18s %-10s %-13s %s\n", sess.Feature, sess.ID, string(sess.State), string(sess.CaptureMode), promoted)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit deterministic schema-versioned JSON")
	return cmd
}

// ─── session purge ───────────────────────────────────────────────────────────

func sessionPurgeCmd() *cobra.Command {
	var sessionID string
	var dryRun bool
	var yes bool
	var all bool
	cmd := &cobra.Command{
		Use:   "purge [slug]",
		Short: "Delete local session buffers (dry-run by default)",
		Long: "Delete local session buffers under .tpatch/local/capture/.\n\n" +
			"Defaults to --dry-run per PRD §6 D14. Deletion requires --yes.\n" +
			"--all and <slug> are mutually exclusive. Refuses unsafe paths\n" +
			"(symlink escape after evaluation) per PRD §8.19.\n\n" +
			"Idempotent per PRD §8.11 — purging already-absent buffers\n" +
			"succeeds without change.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("session purge: --all and <slug> are mutually exclusive")
			}
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}
			entries, err := s.ListSessions(filter)
			if err != nil {
				return fmt.Errorf("session purge: %w", err)
			}
			if sessionID != "" {
				filtered := entries[:0]
				for _, e := range entries {
					if e.SessionID == sessionID {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			out := cmd.OutOrStdout()
			if len(entries) == 0 {
				fmt.Fprintln(out, "no sessions to purge")
				return nil
			}
			for _, e := range entries {
				dir := s.SessionDir(e.Slug, e.SessionID)
				if dryRun || !yes {
					fmt.Fprintf(out, "would remove %s\n", dir)
					continue
				}
				if err := s.PurgeSession(e.Slug, e.SessionID); err != nil {
					return fmt.Errorf("purge %s/%s: %w", e.Slug, e.SessionID, err)
				}
				fmt.Fprintf(out, "removed %s\n", dir)
			}
			if !yes && !dryRun {
				fmt.Fprintln(out, "(no changes; pass --yes to confirm deletion)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Purge only the given cs_<12hex>")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletions without removing anything (default)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion (required to actually remove buffers)")
	cmd.Flags().BoolVar(&all, "all", false, "Purge sessions for every feature")
	return cmd
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// sessionEligibleForStop returns true when the session is either
// active (the only state that can be meaningfully closed) OR already
// closed / promoted — those are idempotent-stop targets per PRD §8.11.
// Returning them here lets `session stop` emit the "already <state>"
// idempotent message instead of a "no eligible sessions" refusal.
// Purged sessions are excluded because their local buffer no longer
// exists.
func sessionEligibleForStop(sess store.Session) bool {
	return sess.State == store.SessionActive ||
		sess.State == store.SessionClosed ||
		sess.State == store.SessionPromoted
}

// pickSessionForOp chooses the target session for a same-feature
// operation. When a specific `--session <cs_id>` is provided, it must
// match; otherwise if exactly one same-feature session is eligible
// per predicate, use it. Ambiguous selection is a refusal per
// PRD-active-feature-session §5 D9.2.
func pickSessionForOp(s *store.Store, slug, sessionID string, eligible func(store.Session) bool) (store.Session, error) {
	if sessionID != "" {
		if !store.IsValidSessionID(sessionID) {
			return store.Session{}, fmt.Errorf("--session %q is not a cs_<12hex> id", sessionID)
		}
		sess, err := s.LoadSession(slug, sessionID)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return store.Session{}, fmt.Errorf("no session %s under feature %s", sessionID, slug)
			}
			return store.Session{}, err
		}
		return sess, nil
	}
	entries, err := s.ListSessions(slug)
	if err != nil {
		return store.Session{}, err
	}
	var matches []store.Session
	for _, e := range entries {
		if e.Session == nil {
			continue
		}
		if eligible == nil || eligible(*e.Session) {
			matches = append(matches, *e.Session)
		}
	}
	switch len(matches) {
	case 0:
		return store.Session{}, fmt.Errorf("no eligible sessions for feature %s", slug)
	case 1:
		return matches[0], nil
	default:
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return store.Session{}, fmt.Errorf("multiple eligible sessions for feature %s (%s); pass --session <cs_id>", slug, strings.Join(ids, ", "))
	}
}

// sessionExists is a cheap probe used by tests / doctor forward-integration.
func sessionExists(s *store.Store, slug, sessionID string) bool {
	_, err := os.Stat(s.SessionManifestPath(slug, sessionID))
	return err == nil
}

// ─── session summarize ──────────────────────────────────────────────────────
// The full redaction-boundary promotion logic lands in Slice 3. Slice 2
// wires the subcommand skeleton so the parity guard sees the command
// name AND the cobra tree exposes it. --dry-run is the default per
// PRD §6 D14; --write and --promote perform the mutating actions.

// SessionSummarizeJSON is the deterministic --json shape returned by
// `session summarize` per PRD §6 D14.
type SessionSummarizeJSON struct {
	SchemaVersion             string   `json:"schema_version"`
	Feature                   string   `json:"feature"`
	SessionID                 string   `json:"session_id"`
	WouldWrite                bool     `json:"would_write"`
	SummaryID                 string   `json:"summary_id,omitempty"`
	RedactionStatus           string   `json:"redaction_status"`
	ForbiddenContentFindings  []string `json:"forbidden_content_findings"`
	CommittedSummaryPath      string   `json:"committed_summary_path,omitempty"`
	PromotionRefusalReasonMsg string   `json:"promotion_refusal_reason,omitempty"`
}

// sessionSummarizeCmd wires the subcommand skeleton in Slice 2 and is
// fleshed out in Slice 3 with the redaction contract (PRD §5 D11) and
// the committed-summary write path (PRD §5 D10). Slice 2 lands the
// dry-run/eligibility surface and the deterministic JSON shape so
// parity guard + CLI-tree tests can be authored before the write path.
func sessionSummarizeCmd() *cobra.Command {
	var sessionID string
	var dryRun bool
	var write bool
	var asJSON bool
	var promote bool
	cmd := &cobra.Command{
		Use:   "summarize <slug>",
		Short: "Preview / write a redacted committed summary for a session",
		Long: "Summarize a session for eventual promotion to the committed lane.\n\n" +
			"Defaults to --dry-run per PRD §6 D14. `--write` mutates by writing the\n" +
			"redacted summary to .tpatch/features/<slug>/artifacts/context/<ctx_id>.json.\n" +
			"--dry-run + --write is invalid. `--promote` transitions the source session\n" +
			"state to `promoted` in the same call (opt-in per PRD §5 D9).\n\n" +
			"Redaction (PRD §5 D11) is enforced at this boundary. Raw session bodies\n" +
			"NEVER cross into the committed lane. Redaction failure prevents the write\n" +
			"and leaves existing committed summaries unchanged (PRD §8.12).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if dryRun && write {
				return fmt.Errorf("session summarize: --dry-run and --write are mutually exclusive")
			}
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			target, err := pickSessionForOp(s, slug, sessionID, sessionEligibleForSummarize)
			if err != nil {
				return fmt.Errorf("session summarize: %w", err)
			}
			out := cmd.OutOrStdout()
			return runSessionSummarize(out, s, target, sessionSummarizeOpts{
				Write:   write,
				Promote: promote,
				AsJSON:  asJSON,
			})
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Select a specific cs_<12hex> when multiple are eligible")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview only (default when neither --dry-run nor --write is set)")
	cmd.Flags().BoolVar(&write, "write", false, "Write the redacted committed summary")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit deterministic schema-versioned JSON")
	cmd.Flags().BoolVar(&promote, "promote", false, "Also transition the source session to promoted state (PRD §5 D9 opt-in)")
	return cmd
}

// sessionEligibleForSummarize returns true when the session is in an
// active or closed state — both are permitted sources per PRD §3 D4
// (active -> promoted and closed -> promoted are valid transitions).
func sessionEligibleForSummarize(sess store.Session) bool {
	return sess.State == store.SessionActive || sess.State == store.SessionClosed
}
