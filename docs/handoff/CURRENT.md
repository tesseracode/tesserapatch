# Current Handoff

## Active Task

- **Task ID**: `feat-amend-dependent-warning` — v0.7.0
- **Milestone**: v0.7.0 — M15 W3 freshness continuation
- **Status**: ✅ COMPLETE — APPROVED by external supervisor (rev-1, commit `6e78eac`); ready to push and tag `v0.7.0`
- **Assigned**: 2026-05-10 (rev-0); 2026-05-11 (rev-1)
- **Ship commits**: `8306367` (impl) → `6e78eac` (rev-1 fixes) → `a5e7de0` (sub-agent verdict) → tracking commit → tag `v0.7.0`

## Brief — `feat-amend-dependent-warning` (v0.7.0)

### Background

The M14 dependency model lets feature B declare a dependency on feature A's `apply.base_commit` SHA via `B.dependencies[].satisfied_by`. M15 W2 hardened `satisfied_by` to require a 40-hex SHA reachable from HEAD (sentinel `ErrSatisfiedBySHANotReachable` at `internal/store/validation.go`). M15 W3 added the freshness overlay so `tpatch verify` can flag features whose patches no longer apply cleanly to their stated base.

What's still missing: there is no proactive warning at the point a user is about to break a downstream dependency. The classic failure mode is `git commit --amend` (or `git rebase -i`) on a commit referenced by another feature's `satisfied_by` — the SHA disappears, every downstream feature silently becomes unsatisfiable, and the breakage is only noticed at the next reconcile / verify / record run.

### Goals

Surface this class of breakage **before** the rewrite (where feasible) and **immediately after** the rewrite (where pre-rewrite intercept isn't feasible).

### Acceptance Criteria

**Detection algorithm (shared)**
1. Build set `S = {sha | sha is referenced as base_commit OR satisfied_by in any active feature.yaml}`. Source: walk `.tpatch/features/*/feature.yaml`, collect both fields. Cache for the duration of a single CLI invocation.
2. A SHA is "broken" if it is no longer reachable from HEAD. Use `git merge-base --is-ancestor <sha> HEAD` (helper in `internal/gitutil/`).
3. Build the back-pointer set: which features reference each broken SHA.

**Surface 1 — `tpatch status` (post-hoc detection)**
- Add a derived label `dependent-broken`, composable with existing freshness labels per ADR-013.
- Emitted on any feature whose own `apply.base_commit` is unreachable, OR any feature whose `satisfied_by` parent points at an unreachable SHA.
- Plain-text line: `dependent-broken: feature 'foo' references SHA <abbrev> which is no longer reachable (likely amend / rebase upstream)`.
- JSON: new field `"dependent_broken": true` per affected feature, with `"broken_refs": [{"kind": "satisfied_by"|"base_commit", "sha": "...", "feature": "..."}]`.

**Surface 2 — `tpatch record` (warn before persisting an amend)**
- When `tpatch record` runs and detects an amend shape (previous HEAD's parent matches new HEAD's parent — classic `git commit --amend`), check if the previous HEAD's SHA is in set `S`.
- If yes, **abort by default** with: `record refuses: this amend would orphan downstream feature(s) [list]. Their satisfied_by/base_commit references would dangle. Use --force-amend to proceed (you take responsibility for re-recording downstream features).`
- New flag: `--force-amend` (boolean) — bypass the gate. Documented in help text + skill surfaces.
- If no amend detected (clean append), no behavior change.

**Surface 3 — recovery hint**
- When `tpatch status` shows `dependent-broken`, also emit: `hint: re-record affected feature(s) on the new base, or run 'tpatch reconcile' to attempt automatic re-anchor.` (Auto-recovery is out of scope for this slice.)

### Non-Goals

- Git pre-amend hook installation (tpatch does not install git hooks; users opt in via `tpatch record` being their amend gateway).
- Auto-re-anchoring downstream features (`tpatch reconcile` already exists — this slice only WARNS).
- Cross-repo dependent detection (only `.tpatch/features/*` in current repo).
- Pre-receive / push-time blocking.

### Test Fixtures

1. `TestRecord_RefusesAmendThatOrphansDependent` — chain A→B (`B.dependencies[0].satisfied_by = A.commit_sha`), `git commit --amend` on A's commit, run `tpatch record` for amended A, assert exit != 0 and stderr contains "orphan downstream feature 'B'".
2. `TestRecord_ForceAmendBypassesGate` — same setup + `--force-amend`, assert exit 0 with warning still on stderr.
3. `TestStatus_DependentBrokenLabel` — chain A→B, `git reset --hard <pre-A>` then `git commit -am "different A"`, run `tpatch status`, assert B shows `dependent-broken`.
4. `TestStatus_DependentBrokenJSON` — same as #3 with `--format json`, assert `"dependent_broken": true` and `"broken_refs": [{"kind":"satisfied_by",...}]`.
5. `TestDependentDetection_BaseCommitBroken` — single feature whose own `apply.base_commit` becomes unreachable; asserts `dependent-broken` fires on that feature itself.

### Skill-Surface Impact

Minimal. One-line addition to each of the 6 skill surfaces in the troubleshooting / freshness section: "If `tpatch status` reports `dependent-broken`, a downstream feature's base SHA is no longer reachable — re-record affected features on the new base or run `tpatch reconcile`." Parity-guard anchor TBD by implementer (suggest regex `dependent-broken` substring is sufficient given uniqueness).

### Files Likely Touched

- `internal/store/dependents.go` (new) — `CollectDependentSHAs(store) (map[sha]→[]featureRef, error)` + `IsAmendBreaking(prevHead, newHead, dependents) ([]featureRef, error)`.
- `internal/cli/cobra.go` — `record` amend-detection + `--force-amend` flag wiring; `status` `dependent-broken` emission.
- `internal/store/status.go` (or wherever derived labels live per M15 W3) — new label kind.
- `internal/gitutil/` — `IsAncestor(sha)` helper if not already present.
- `assets/skills/{6 surfaces}` + `assets/assets_test.go` — one-line addition + parity anchor.
- `CHANGELOG.md` — v0.7.0 section.
- Tests as enumerated above.

### Open Questions for User Before Dispatch

1. **Default abort vs warn-and-proceed on `tpatch record`?** Brief proposes `abort by default + --force-amend`. Alternative: warn-and-proceed by default + `--strict-amend-check` to abort. **Recommendation: abort.** Matches the M14 `satisfied_by` reachability validator (also abort-by-default).
2. **JSON shape OK?** The `dependent_broken` + `broken_refs[]` shape mirrors M15 W3 freshness overlay JSON. Implementer can refine if needed.
3. **PRD or no PRD?** Brief is ~120 lines and covers the contract. **Recommendation: ship without a PRD** — the M14 dependency model already specifies the data, this slice only adds detection + UX. PRD-first is overkill for a slice this size.

### Cosmetic ROADMAP Lint (bundle into v0.7.0 tracking commit)

`docs/ROADMAP.md` line 331 still says `## M15+ — Future` even though M16 and M17 exist. Rename to `## M18+ — Future` when v0.7.0 ships.

## Resolved Earlier This Session

- **v0.6.4 shipped** — M16 Slice 3 (apply-default-auto + 6-surface alignment + parity-anchor strengthening). 4-commit stack `eab2c3c` + `4556387` + `38d13fc` + `477ccc9` + tracking `9bd5fb1`. Tag pushed.
- **v0.7 cluster routing pass** (`7196ae8` + decisions `603506d`) — opened ADR-016..019 placeholders, slugged M17 with Wave A/B/C/D structure, resolved 3 supervisor decisions (LOG.md "Supervisor Decisions — v0.7 Cluster Routing — 2026-05-10").

## Just Shipped

- **v0.6.4** — M16 Slice 3 (apply-default-auto + 6-surface skill alignment + parity-anchor strengthening). 4-commit stack `eab2c3c` + `4556387` + `38d13fc` + `477ccc9` + tracking `9bd5fb1`. Tag pushed, ROADMAP M16 ✅, archived to HISTORY.md.
- **v0.7 cluster routing pass** (`7196ae8` + tracking) — opened ADR-016..019 placeholders, slugged M17 with Wave A/B/C/D structure, surfaced 3 supervisor decisions (now resolved — see LOG.md "Supervisor Decisions — v0.7 Cluster Routing — 2026-05-10").

## Resolved Supervisor Decisions (2026-05-10)

1. **PRD-patch-already-upstream-detector**: accepted as accepted-exploratory; slotted into M17 as **Wave D** (default-OFF reconcile fast-path, ships with cluster, user-visible flag flipped on a later v0.8.x point release).
2. **Implementation owner assignment**: deferred to backlog (`backlog-assign-m17-owners`); fresh implementer sub-agent dispatched per Wave at start time.
3. **Claims-audit-table convention**: codified in AGENTS.md as "PRD Authoring — Strongly Encouraged Conventions" (3 conventions, no automated guard, reviewer cross-pass remains the safety net).
4. **Ordering** (bonus): v0.7.0 = `feat-amend-dependent-warning`. M17 boundary-capture cluster ships as **v0.8.0**.

## Revision-1 Implementation — pending-commit — 2026-05-11

External supervisor review of `8306367` returned **NEEDS REVISION** with two
findings, both verified by code inspection. Both fixed in this revision.

### Finding 1 (Medium) — `dependent-broken` missing from `status --dag` and `status --dag --json`

**Root cause.** `renderNodeLineWithFreshness` and `writeDAGJSON` in
`internal/cli/status_dag.go` only called `mergedLabels(st, freshness)` and
never received the `brokenByFeature` map collected at the top of the status
command. The `appendLabel(LabelDependentBroken)` overlay existed in
`status_dag.go` but was only invoked from the non-DAG path in `cobra.go`.

**Fix.** Threaded the broken-refs map (`map[string][]store.FeatureRef`) through
the DAG renderers as a new trailing parameter:

- `runStatusDAG(out, s, scopeSlug, asJSON, brokenByFeature)`
- `writeDAGTree(..., brokenByFeature)` and `writeDAGJSON(..., brokenByFeature)`
- `walkTree(..., brokenByFeature)`
- `renderNodeLine(s, st, brokenRefs)` and `renderNodeLineWithFreshness(st, freshness, brokenRefs)`

Inside the renderers, `appendLabel(labels, store.LabelDependentBroken)` now
fires when `len(brokenRefs) > 0` — same overlay rule the non-DAG path used.
The `dagJSONNode` struct gains two fields shaped identically to the non-DAG
`--json` payload:

```go
DependentBroken bool                `json:"dependent_broken,omitempty"`
BrokenRefs      []dagJSONBrokenRef  `json:"broken_refs,omitempty"`
```

with `dagJSONBrokenRef{Kind, SHA, Feature}` mirroring the anonymous
`brokenRefJSON` struct in `cobra.go`. **No-recompute rule respected**:
`CollectBrokenRefs` is still called exactly once per status invocation
(at `cobra.go:239`) and the result is threaded into both render paths.

**Tests added.**
- `TestStatusDAG_DependentBrokenLabel` — runs `status --dag` (text), asserts
  the rendered tree shows `dependent-broken` in the labels parens for the
  affected feature.
- `TestStatusDAG_DependentBrokenJSON` — runs `status --dag --json`, parses the
  payload, asserts `dependent_broken: true`, asserts a `satisfied_by` broken
  ref with the original A SHA + feature=b, and asserts `dependent-broken` is
  in the `labels` array.

### Finding 2 (Low) — Plain-text status emitted one line per broken ref

**Root cause.** `cobra.go:346-351` was nested (`for slug` outer, `for r := range
brokenByFeature[slug]` inner), printing one `dependent-broken: feature %q ...`
line per broken ref. The comment on lines 337-339 advertised "one diagnostic
line per affected feature" — the implementation contradicted the advertised
contract whenever a feature carried both `apply.base_commit` AND `satisfied_by`
pointing at the same rewritten SHA.

**Fix.** Coalesced per feature: collect the abbrev SHAs for the feature, dedupe
(via `map[string]struct{}`), sort ascending, and emit ONE line listing all the
abbrevs joined by `, `. The line text changed slightly to match the multi-SHA
case uniformly:

```
dependent-broken: feature "b" references SHA(s) <a7>, <x7> which are no longer reachable (likely amend / rebase upstream)
```

The `SHA(s)` / `are` phrasing is intentionally singular-friendly — reads
correctly for both the 1-SHA and N-SHA cases without per-count branching. The
recovery hint line below remains emitted exactly once per status invocation
(unchanged, was already correct).

**Tests added.**
- `TestStatus_DependentBrokenSingleLinePerFeature` — feature `b` has BOTH
  `apply.base_commit` and `satisfied_by` pointing at the same broken SHA;
  asserts exactly ONE `dependent-broken: feature "b"` line is emitted, and
  that the line includes the abbrev SHA.
- `TestStatus_DependentBrokenMultipleSHAsPerFeature` — feature `c` has
  `apply.base_commit` = SHA-X (broken) and `satisfied_by` = SHA-A (also
  broken, distinct SHA); asserts exactly ONE line for feature `c` listing
  BOTH abbrevs joined `", "` in ascending order.

### Files Changed (rev-1)

Implementation:
- `internal/cli/status_dag.go` — `dagJSONNode` extended with
  `DependentBroken` + `BrokenRefs`; new `dagJSONBrokenRef` type; `runStatusDAG`,
  `writeDAGTree`, `writeDAGJSON`, `walkTree`, `renderNodeLine`,
  `renderNodeLineWithFreshness` all gain a trailing `brokenByFeature` /
  `brokenRefs` parameter; `appendLabel(store.LabelDependentBroken)` overlay
  composed in both renderers.
- `internal/cli/cobra.go` — `runStatusDAG` callers pass the precomputed
  `brokenByFeature` map; plain-text emission loop coalesced per feature with
  deduped + sorted abbrev SHA list.

Tests:
- `internal/cli/dependent_broken_test.go` — added `sort` import; 4 new tests
  (`TestStatusDAG_DependentBrokenLabel`, `TestStatusDAG_DependentBrokenJSON`,
  `TestStatus_DependentBrokenSingleLinePerFeature`,
  `TestStatus_DependentBrokenMultipleSHAsPerFeature`).

Tracking:
- `docs/handoff/CURRENT.md` — Active Task status flipped to "Revision-1"; this
  section appended.
- `CHANGELOG.md` — v0.7.0 section gains a `### Fixed (revision-1)` subsection
  naming both findings transparently.

### Verification

- `gofmt -l .` — clean.
- `go build ./cmd/tpatch` — green.
- `go test ./...` — all green (full suite).
- `go test ./assets -run TestSkillParityGuard -count=1 -v` — all 6 surfaces
  pass.
- Live repro (the exact reviewer-supplied script with the broken A→B chain
  and pre-A reset): plain `status` emits ONE `dependent-broken` line per
  feature; `status --dag` shows `(dependent-broken, never-verified)` in the
  labels parens for both `a` and `b`; `status --dag --json` payload carries
  `"dependent_broken": true` and a `"broken_refs": [...]` array shaped
  identically to the non-DAG `--json` payload.

### Hands-Off Confirmations

- `internal/workflow/reconcile.go:599` HIGH bug NOT touched (Wave A2).
- `docs/prds/`, `docs/whitepapers/`, `docs/market-research/`,
  `docs/adrs/ADR-016..019.md` NOT touched.
- M17 ROADMAP rows NOT touched.
- `docs/state-of-the-art/` NOT touched (untracked, user-owned).
- "Side Research — State-of-the-art middle pass" section in this file NOT
  modified.

## Background — `feat-amend-dependent-warning`

Continuation of the M15 W3 freshness overlay work (verify-freshness shipped in v0.6.2). The amend-dependent-warning feature warns when a user is about to `git commit --amend` (or otherwise rewrite history) on a commit that has dependent features downstream — preventing silent corruption of the dependency graph.

**Next step before implementer dispatch**: draft a concrete brief covering:
- Scope: which `tpatch` commands/hooks emit the warning (record? apply? a new git pre-amend hook?)
- Detection algorithm: how to identify "this commit is depended on by feature X"
- Output shape: warning text, exit code, override flag (`--force`?)
- Skill-surface impact (if any)
- Test fixtures: amend-on-dependent reproducer

The PRD does not yet exist for this feature — it's a smaller polish item that may not need a full PRD. **Decision required from user**: PRD-first or brief-and-implement?

## M17 — Queued for v0.8.0

| Slice | PRD | ADR placeholder | Wave deps |
|-------|-----|-----------------|-----------|
| A1 — `impl-record-auto-base` | PRD-record-auto-base | ADR-016 | Independent |
| A2 — `impl-reconcile-lock-guard` + writer-norm fix | PRD-reconcile-lock-guard | ADR-017 | Independent (bundles HIGH bug fix at `internal/workflow/reconcile.go:599`) |
| B — `impl-record-collision-detection` | PRD-record-collision-detection | ADR-018 | Wave A1 |
| C — `impl-tpatch-land` | PRD-tpatch-land | ADR-019 | Wave A1 + Wave A2 + Wave B |
| D — `impl-patch-already-upstream-detector` | PRD-patch-already-upstream-detector | TBD | Independent (default-OFF) |

Owner assignment per slice: backlog `backlog-assign-m17-owners`.

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

### Candidate follow-up names

These are research outputs only, not queued roadmap work:

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

## Blockers

None. Awaiting user sign-off on:
- Brief scope (especially: abort-by-default vs warn-and-proceed-by-default on `tpatch record`)
- No-PRD recommendation
- Then dispatch implementer.
