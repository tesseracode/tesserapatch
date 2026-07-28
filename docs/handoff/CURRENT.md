# Current Handoff

## Active Task

- **Task ID**: `doctor-wave-gamma-d4-d5`
- **Milestone**: `tpatch doctor` implementation — Wave γ (D4 locks + D5 evidence). Third wave of the 4-wave cluster.
- **Description**: Extend the doctor scaffold (Wave α at `a3b9fe3`) + D3/D7 checks (Wave β at `daf2e6f`) with two persisted-artifact detection classes: D4 (old or malformed `upstream.lock` / related lock formats — read-only + safe format normalization if applicable) and D5 (missing `reconcile-evidence.jsonl` for applied/reconciled features + malformed JSONL line reporting). Wave γ closes §6.10-§6.13.
- **Status**: Review.
- **Assigned**: 2026-07-28.

## Doctor implementation cluster wave plan

- **Wave α** ✅ CLOSED 2026-07-27 (three-way APPROVED WITH NOTES). Scaffold + D1 + D2 + D8. §6.1-§6.7 + §6.20-§6.29 MET.
- **Wave β** ✅ CLOSED 2026-07-28 (three-way APPROVED, zero findings). D3 + D7. §6.8, §6.9, §6.18, §6.19 MET. Rule 18 self-applied successfully.
- **Wave γ** ← this handoff (D4 locks + D5 evidence). §6.10-§6.13.
- **Wave δ** — D6 release drift. Needs `--release-metadata <file>` plumbing. §6.14-§6.17.

## Wave γ binding scope

### D4 — Old or malformed lock formats (§6.10, §6.11)

- **Detect** (§6.10): report malformed, old-format, stale-ref, and unreachable-commit lock conditions WITHOUT fetching from remotes.
- **`--fix` (§6.11)**: perform ONLY equivalent lock-format normalization. NEVER advance the locked commit. NEVER guess a branch.
- Lock files to inspect:
  - `upstream.lock` (canonical WP-002 lock format per ADR-011 / ADR-017).
  - Any other lock files under `.tpatch/` that the current schema requires (grep production loaders + PRD refs).
- Malformed classes to detect (implementer verifies list against PRD + production loader):
  - Missing required fields.
  - Unknown fields.
  - Wrong type.
  - Malformed SHA (not a valid hex string of expected length).
  - Old-format markers (e.g., pre-schema-version files).
  - Stale ref: `upstream.lock` names a ref that no longer exists in the local repo.
  - Unreachable commit: locked SHA is not reachable from any local ref.
- **Read-only in v1** for anything beyond format normalization. NEVER touch commit SHAs or refs. NEVER call `git fetch` or `git ls-remote`.
- Rule 12 privacy binding — D4 must NOT read remote git state.

### D5 — Missing reconcile evidence artifacts (§6.12, §6.13)

- **Detect missing evidence** (§6.12): for features whose `status.json` indicates a modern reconcile attempt (i.e., a reconcile that would have written evidence under ADR-025), report missing `reconcile-evidence.jsonl`.
- **Detect malformed JSONL** (§6.13): report malformed lines with filename + 1-indexed line number. Continue inspecting other entries and features (rule 5, ADR-025 D11 pattern — malformed-artifact handling).
- Also inspect `reconcile-revisions.jsonl` for the same malformed-JSONL class (ADR-025 D11 applies to both artifacts).
- Read-only in v1.
- Remediation string for missing evidence: `run tpatch reconcile <slug>` (verify command exists via rule 15 grep of `internal/cli/cobra.go`).

## Wave γ hard constraints (binding, 15)

Same 15 as Wave β with the following emphasis:

1. **PRD as binding contract** — every fix traces to §6.X or STOP.
2. **Safety defaults NON-NEGOTIABLE** — extend Wave α scaffold; D4 `--fix` MUST refuse if the fix would advance the locked commit or guess a branch.
3. **No new lifecycle states** (`FeatureState` untouched).
4. **No new persisted schemas outside doctor's JSON output**.
5. **ADR-025 D11 pattern** — D5 malformed JSONL reports filename + 1-indexed line number; D4 malformed lock reports filename + specific field/error.
6. **Rule 12 privacy** — D4 MUST NOT read remote git state (no `git fetch`, `git ls-remote`, or equivalent). D5 MUST NOT log full content of evidence entries in doctor output; report only line number + class of error + truncated hash if needed.
7. **Rule 15 (trigger-name grep)** — verify `tpatch reconcile <slug>` command exists via `internal/cli/cobra.go` grep. If it doesn't, pick the real command name (likely `tpatch reconcile <slug>` — check).
8. **Rule 11 (flag-surface accuracy)** — no new persistent flags.
9. **Rule 17 (totality claims)** — D4 detection MUST NOT teach "only `upstream.lock` is checked" without enumerating any other lock files the current schema requires. Match Slice 4 F2 framing style.
10. **Rule 16 (anti-drift parity guard)** — if D4 shares a lock loader with production `internal/store/*.go`, use the production loader (do NOT re-implement). If a new lock schema field is needed, draft a small D-clause amendment BEFORE writing schema code (per Wave α hard constraint 4).
11. **Rule 18 (structural trailer verification)** — sanity-check `git log --format='%(trailers)' <sha>` for every commit before pushing.
12. **CHANGELOG.md** — extend the existing `## v0.11.2 (unreleased) — tpatch doctor Wave α` header with a `### Wave γ` subsection alongside the existing `### Wave β`. Do NOT create a separate `(unreleased)` entry.
13. **Assets/skills** — no new asset mentions expected. Parity guard MUST still pass.
14. **Side Research md5** == `b385fe622db9926f48861105239f113e`.
15. **Full gates** + Co-authored-by trailer (structural verify).

## Wave γ suggested layout

- `internal/workflow/doctor_d4.go` — new: D4 detection + optional format normalization.
- `internal/workflow/doctor_d5.go` — new: D5 detection (evidence artifact presence + malformed JSONL).
- Extend `internal/workflow/doctor.go` — register D4 + D5 in check registry from Wave α scaffold.
- Extend `internal/cli/doctor.go` — no new CLI shape; --fix path exercises D4 for the first (limited) lock-format normalization case.
- Tests:
  - `internal/workflow/doctor_d4_test.go` — fixtures: clean lock, malformed field, wrong type, malformed SHA, old-format, stale-ref, unreachable-commit, --fix normalization idempotence, --fix refuses advancing SHA, --fix refuses guessing branch.
  - `internal/workflow/doctor_d5_test.go` — fixtures: clean evidence, missing evidence for reconciled feature, malformed JSONL line (with correct 1-indexed line number reported), continuation-past-malformed-line, revisions.jsonl malformed line.
  - Extend `internal/cli/doctor_test.go` for D4/D5 end-to-end (no new safety-defaults tests expected — Wave α + β covered those).

## Wave γ reviewer-brief additions (folded from Wave β process)

- Rule 18 continues (all Wave β reviewers self-applied successfully).
- Rule 15 (trigger-name grep) for D5 remediation string.
- Rule 12 (privacy) specifically for:
  - D4: no remote git calls.
  - D5: no full content logging of evidence entries beyond line number + error class.
- Rule 5 (ADR-025 D11) for both D4 malformed lock reporting AND D5 malformed JSONL.

Reviewer briefs MUST verify:
1. Grep new doctor code for any `exec.Command("git", "fetch"...)` or `exec.Command("git", "ls-remote"...)` — must be ZERO.
2. Grep new doctor code for `Body` / `Content` / `Message` / `RawJSON` field access on evidence entries — flag any full-content logging.
3. Verify D4 `--fix` refuses commit-advancement + branch-guessing via explicit test coverage.
4. Verify D5 malformed-JSONL 1-indexed line number correctness.

## Process for implementer

1. Read `docs/prds/PRD-tpatch-doctor.md` in FULL. Focus on §3 D4, §3 D5, §5 implementation notes, §6.10-§6.13, §7 open questions.
2. Read this handoff Wave γ binding scope + hard constraints verbatim.
3. Read `docs/supervisor/LOG.md` top 3 entries: user-external Wave β + supervisor Wave β decision + supervisor-external Wave β.
4. Read production ground truth:
   - `internal/store/*.go` — lock loaders (grep for `upstream.lock` + `LoadUpstreamLock` or equivalent).
   - `internal/store/reconcile_evidence.go` — evidence JSONL structure + `LoadReconcileEvidence` (lenient variant if it exists).
   - `internal/store/reconcile_revision.go` — revision JSONL + `LoadReconcileRevisions`.
   - `internal/cli/cobra.go` — grep for `tpatch reconcile` command shape (rule 15).
   - `internal/workflow/doctor.go` — Wave α + β scaffold + check registry pattern.
   - `internal/workflow/doctor_d3.go` / `doctor_d7.go` — Wave β patterns to mirror.
5. Implement D4 first (lock format is the more constrained + safety-critical of the two). Then D5.
6. Fold Wave β lessons:
   - Reuse production loaders (do NOT re-implement schema).
   - Report format errors with filename + specific error (D4) or filename + 1-indexed line (D5).
   - `--fix` MUST refuse anything beyond format normalization for D4.
7. Update `docs/handoff/CURRENT.md`:
   - Flip Status to Review.
   - Add "Wave γ closure summary" subsection: per-§6-criterion + file:line + specific normalization semantics chosen for D4.
   - Preserve Side Research md5.
8. Extend CHANGELOG.md `## v0.11.2 (unreleased)` header with `### Wave γ` subsection (do NOT touch existing `### Wave α` and `### Wave β` subsections beyond adding the new one).
9. Push to `origin/main`. Sanity-check `git log --format='%(trailers)' <sha>` for every new commit before pushing.
10. Return commit hashes + gate output + trailer structural verification of all new commits.

If any §6 criterion is impossible without breaking a hard constraint: STOP and report.

Do not dispatch reviewers — supervisor handles that.

## Session Summary

Doctor Wave γ implementation landed at `cffeabd` and is ready for reviewer dispatch. D4 and D5 are registered in `tpatch doctor`, covered by workflow/CLI tests, and validated with full gates.

### Wave γ closure summary

- **§6.10 D4 detection**: MET at `internal/workflow/doctor_d4.go:30`, `:170`, `:306`. D4 reuses production `store.LoadUpstreamLock` before strict doctor diagnostics, reports missing/empty/malformed/unknown/wrong-type/malformed-SHA/old-format/stale-ref/unreachable-commit findings, and uses only local git commands (`rev-parse`, `cat-file`, `for-each-ref`, `merge-base`) with no `fetch`, `ls-remote`, or remote update.
- **§6.11 D4 safe fix**: MET at `internal/workflow/doctor_d4.go:250`, `:296`. Normalization semantics chosen: canonical key order, double-quoted scalar values, LF endings, deterministic doctor comment, and legacy `branch: "<remote>/<branch>"` rewritten to `branch: "<branch>"` only when `remote`, `branch`, and `commit` are all unambiguous. `--fix` refuses missing commit/branch, malformed SHA, unknown fields, wrong types, and malformed lines rather than advancing commits or guessing branches.
- **§6.12 D5 missing evidence**: MET at `internal/workflow/doctor_d5.go:12`, `:120`. Modern reconcile heuristic: `status.json` in an applied/active/reconciling/reconciling-shadow/blocked/upstream_merged state plus any reconcile signal (`attempted_at`, `outcome`, upstream fields, review verdict, patch-id match, or resolver fields) requires `artifacts/reconcile-evidence.jsonl`; same states with no reconcile signal get a WARN grace as likely pre-ADR-025.
- **§6.13 D5 malformed JSONL**: MET at `internal/workflow/doctor_d5.go:37`, `:80`, backed by `internal/store/reconcile_evidence.go:225` and existing `internal/store/reconcile_revision.go:168` lenient loaders. D5 reports filename + 1-indexed line for evidence and revision corrupt entries and continues across later lines/features without logging full evidence content.
- **Rule 15**: verified `internal/cli/cobra.go` exposes `Use: "reconcile [slug...]"`; D5 remediation uses `run tpatch reconcile <slug>`.
- **Rule 11 / flags**: no new persistent or local doctor flags; `internal/cli/doctor.go` only updates help/check ID text.
- **Rule 18**: `git log -1 --format='%(trailers)' cffeabd` returned `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`.

## Current State

D4/D5 implementation is complete and awaiting review. `tpatch doctor` now runs D1/D2/D3/D4/D5/D7/D8 by default. Fresh `tpatch init` workspaces may report a D4 warning for scaffolded empty `upstream.lock`, while populated locks are checked against local-only git state.

## Files Changed

- `internal/workflow/doctor_d4.go`
- `internal/workflow/doctor_d4_test.go`
- `internal/workflow/doctor_d5.go`
- `internal/workflow/doctor_d5_test.go`
- `internal/workflow/doctor.go`
- `internal/cli/doctor.go`
- `internal/cli/doctor_test.go`
- `internal/store/reconcile_evidence.go`
- `internal/store/reconcile_revision.go`
- `CHANGELOG.md`
- `docs/handoff/CURRENT.md`

## Test Results

- Rule 15 trigger-name grep: `grep -n 'reconcile' internal/cli/cobra.go | head -20` showed `Use: "reconcile [slug...]"`.
- Side Research md5 before docs edit: `b385fe622db9926f48861105239f113e`.
- After code commit `cffeabd`: `gofmt -l .` clean; `go vet ./...` clean; `go build ./cmd/tpatch` OK; `go test ./...` PASS.

## Next Steps

1. Supervisor: dispatch Wave γ reviewers.
2. After Wave γ three-way APPROVED: archive to HISTORY, move to Wave δ (D6 release drift, requires `--release-metadata` plumbing).

## Blockers

None.

## Context for Next Agent

- HEAD at Wave γ kickoff: `7ebd9de` + review LOGs. Verify latest via `git log --oneline -n 5`.
- Doctor waves α + β are unreleased; Wave γ still ships under `v0.11.2 (unreleased)`. Version tag `v0.11.2` deferred until all 4 waves close.
- 18 carry-forward rules binding. Rule 18 (structural trailer verification) proved out in Wave β — implementer + all reviewers self-applied without regression.
- D5 remediation string candidate: `run tpatch reconcile <slug>`. Rule 15: verify actual command name before use.
- Slice 4 F2 framing (six-paths totality avoidance) is the pattern for D4 lock-file scope description.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`
