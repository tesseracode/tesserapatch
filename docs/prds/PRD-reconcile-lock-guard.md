# PRD — Reconcile Upstream-Lock Validation Guard — `feat-reconcile-lock-guard`

**Status**: Draft
**Date**: 2026-05-10
**Owner**: OX47 *(Claude Opus 4.7 — Extra-high reasoning tier; distinct from CO47 base)*
**Milestone**: v0.7 ship target — third deliverable per `docs/market-research/competitive-landscape.md` §6 SMART (lines 475-478).
**Depends on**: None for paper design. Implementation reuses the shared `LoadUpstreamLock` parser primitive coordinated with `PRD-record-auto-base` §5; whichever PRD ships first writes the parser, the second consumes it.

## Related

- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md) — graduated source paper. **This PRD is a sibling of WP-001's three graduated PRDs (T16, 2026-04-28), not a successor.** §5.2 row 11 ("no data-model gap") is *reinforced* — see §0.1.
- [Competitive Landscape](../market-research/competitive-landscape.md) — §6 SMART (the v0.7 target this PRD implements); §9 (gbp-pq `--time-machine N` and "auto-detect patches already applied upstream via git rebase" prior art); §10 (anti-precedents — what we don't take from gbp-pq); §12 (axis-2 versioned-history and axis-3 op-log weaknesses, out of scope here).
- [Personas](../market-research/personas.md) — Persona 1 Platform Pat (audit JTBD, primary motivator); Persona 3 Maintainer Mira (reasoning preservation); Persona 2 Security Sam (forwardability — relevant when this guard interacts with hotfix).
- [PRD-record-auto-base](./PRD-record-auto-base.md) (G55) — especially §3.2 baseline-inference algorithm and §5 implementation note ("Add a small parser for `.tpatch/upstream.lock`"). **Shared parser primitive** — see §5 below.
- [PRD-record-collision-detection](./PRD-record-collision-detection.md) (G55) — refusal-policy and acceptance-criteria style this PRD mirrors.
- [PRD-tpatch-land](./PRD-tpatch-land.md) (CO47) v2 — §3.4 four-trailer block and §6.1 v0.7 SMART target (this PRD is the third deliverable named there).
- [PRD-tpatch-hotfix](./PRD-tpatch-hotfix.md) (OX47) — sibling fast-path PRD; the lock-guard semantics defined here apply uniformly to feature-kind and hotfix-kind reconciles.
- [PRD-patch-already-upstream-detector](./PRD-patch-already-upstream-detector.md) (OX47) — phase 1.5 detector that **depends on** the lock being valid; cites this PRD as a precondition (PRD §3.1 step 1).
- [Reconcile Workflow](../reconcile.md) — current operator-facing reconcile contract and the v0.4.2 anti-pattern refusal that is the precedent for this guard's preflight injection point.
- [Feature Layout](../feature-layout.md) — `.tpatch/upstream.lock` storage location.

## 0. Meta

### 0.1 Relationship to WP-001 — sibling, not contradiction

This PRD originates from `competitive-landscape.md` §6 SMART (refreshed 2026-05-02), drafted **after** WP-001 closed at T16 (2026-04-28). Two facts must hold simultaneously:

1. **WP-001 §5.2 row 11 is intact**: "No data-model gap observed in either case … neither case requires a slice object, a containment edge, or a new schema." This PRD introduces **zero new data-model objects**. The `UpstreamLock` struct (`internal/store/types.go:344-350`) already exists; we only validate it. The `ReconcilePreflight` struct (`internal/gitutil/gitutil.go:94-107`) already exists; we extend it with diagnostic fields, not a new type.
2. **This PRD is the third strict v0.7 deliverable**: `competitive-landscape.md:475-485` names three shipping deliverables — `tpatch land`, `tpatch record collision-detection`, and `tpatch reconcile upstream-lock validation guard` (this PRD). `record --auto` is framed as the *remediation mechanism* for legacy collisions, not a fourth shipping primitive in the strict SMART reading. `PRD-tpatch-land.md:651-672` mirrors this exact framing. WP-001 §9 graduated three earlier PRDs in a separate cluster; this PRD originates from the post-WP-001 SMART target and is independent of that graduation count.

The seam is at runtime, not at design time: WP-001 ratified that the existing primitives (canonical patch + `created_by` + hard children + `upstream.lock`) are sufficient. This PRD validates that **at reconcile time**, the lock primitive's stored value is consistent with current upstream reality. That is operational hygiene, not a new object.

### 0.2 Claims audit

Every claim about current behavior in this PRD is anchored to a file:line cite that was re-verified at draft time (2026-05-10).

| Claim | Evidence |
|---|---|
| `UpstreamLock` is a 4-field struct (`Remote`, `Branch`, `Commit`, `URL`) with JSON tags. | `internal/store/types.go:344-350` |
| `ReconcilePreflight` is a value-type struct with four `Files` slices and a `.Clean()` predicate; reconcile MUST NOT run when any field is non-empty unless `--allow-dirty`. | `internal/gitutil/gitutil.go:85-115` |
| `PreflightReconcile(repoRoot string)` is read-only and returns `(ReconcilePreflight, error)`. | `internal/gitutil/gitutil.go:117-178` |
| `tpatch init` scaffolds `.tpatch/upstream.lock` with all four fields empty (`remote: ""`, `branch: ""`, `commit: ""`, `url: ""`). | `internal/store/store.go:100-110` |
| No `LoadUpstreamLock` / `parseUpstreamLock` parser exists in the repo at draft time (zero hits). | `grep -rn "LoadUpstreamLock\|parseUpstreamLock\|ParseUpstreamLock" internal/` → no matches. |
| The `--allow-dirty` flag bypasses the existing reconcile preflight; the v0.4.2 anti-pattern documents the refusal precedent this PRD extends. | `docs/reconcile.md:45-72` |
| `--upstream-ref <ref>` is the existing reconcile flag for overriding the upstream ref (default `upstream/main`). | `docs/reconcile.md:80`; `internal/cli/cobra.go` reconcile command. |
| `tpatch upstream check [--path]` is declared in SPEC but **not yet implemented** (zero hits in `internal/`). | `SPEC.md:72`; `grep -rn "upstreamCheckCmd\|func.*[Uu]pstream.*[Cc]heck" internal/` → no matches. |
| `IsAncestor`, `MergeBase`, `ResolveRef`, `FetchUpstream` exist as gitutil helpers and are reused by `record --auto` per `PRD-record-auto-base` §5. | Referenced by `PRD-record-auto-base.md:206`. |
| WP-001 §5.2 row 11 reads "No data-model gap observed in either case … true-data-model-gap = none". | `docs/whitepapers/WP-001-feature-slice-gap.md:477,486`. |
| `competitive-landscape.md` §6 SMART names `tpatch reconcile upstream-lock validation guard` as the third v0.7 deliverable, with `record --auto` positioned as remediation. | `docs/market-research/competitive-landscape.md:475-485`. |
| `PRD-record-auto-base.md` §5 says "Add a small parser for `.tpatch/upstream.lock`; the existing file is YAML-like but simple enough for the repo's current zero-dep style." | `docs/prds/PRD-record-auto-base.md:205`. |
| `PRD-tpatch-land.md` §6.1 names the lock-guard as deliverable (3) and notes implementation independence from `land`. | `docs/prds/PRD-tpatch-land.md:651-672`. |

No implementation has been changed by this PRD.

### 0.3 Non-goals (pinned)

These are excluded by design, not omitted by oversight. Each line preserves a locked architectural decision.

- **Not a new reconcile phase.** SPEC.md §7's 4-phase decision tree (reverse-apply → operation-level → provider-semantic → forward-apply) is locked. The lock-guard fires inside `PreflightReconcile`, *before* phase 1, and either proceeds or refuses. It does not introduce a phase-N+1 verdict and does not interact with phase 3.5 (provider-resolver, ADR-010) or the proposed phase 1.5 (PRD-patch-already-upstream-detector).
- **Not a replacement for the 4-phase tree.** A valid lock means "phases 1-4 are about to run on accurate inputs"; an invalid lock means "phases 1-4 would run on wrong inputs, refuse." It changes nothing about how verdicts are computed once inputs are good.
- **Not a data-model change.** No new `status.json` fields. No new `UpstreamLock` fields. The diagnostic state lives in the extended `ReconcilePreflight` struct (in-memory, not persisted) and the on-disk `ReconcileSummary` (existing).
- **Not a reconcile preflight redesign.** We extend the existing `ReconcilePreflight` value type; we do not introduce a separate `LockPreflight` type. The brief explicitly forbids this and we agree: dirty-tree state and lock-state are co-equal preflight conditions for the same verb.
- **Not a substitute for `record --auto`.** Auto-base prevents future collisions by inferring a correct baseline at record time; this PRD prevents bad reconcile verdicts by refusing when the recorded baseline has drifted from upstream. They are complementary; both ship in v0.7.
- **Not a `tpatch upstream check` reimplementation.** Recovery hints point at the existing declared verb (SPEC.md:72). If `upstream check` is stubbed at v0.7-cut, the recovery hint adapts (see §3.4); we do not implement `upstream check` here.
- **Not gbp-pq's drop-and-recreate model.** Per `competitive-landscape.md` §10, we do not adopt gbp-pq's pattern of dropping the patch queue and recomputing it from scratch on upstream movement. The lock-guard surfaces stale state; it does not auto-rewrite anything.
- **Not a content-based comparison.** "Stale" is determined strictly by ref byte-identity and git ancestry (`IsAncestor`). Not heuristic on patch content. Patch-content comparison is `PRD-patch-already-upstream-detector`'s phase 1.5 territory.
- **Not a network operation by default.** The guard does not fetch from the remote during preflight. It compares the lock's recorded `commit` to the *currently-resolved* `<lock.Remote>/<lock.Branch>` ref, which is whatever `git fetch` last produced (or the operator's most recent manual update). Fetching is the operator's responsibility, mirroring the pattern recommended in `docs/reconcile.md:25-28` (Pattern A: `git fetch upstream` before `tpatch reconcile`).
- **Not a guard for `record`.** The lock-guard runs at reconcile time only. `record --auto` (G55) handles record-time lock concerns through merge-base fallback; it deliberately treats an empty/stale lock as "fall back to merge-base" rather than refusing. The two PRDs intentionally have different policies because the cost of a wrong decision differs (record produces a recoverable artifact; reconcile mutates verdicts and may auto-mark features as `upstream_merged`).

## 1. Summary

At the start of every `tpatch reconcile` invocation, validate that `.tpatch/upstream.lock` is consistent with the actual upstream state — specifically, that `upstream.lock.commit` is reachable from the current HEAD of `<upstream.lock.remote>/<upstream.lock.branch>`. Surface the verdict as a typed `LockState` field on the existing `ReconcilePreflight` struct, refuse with a clear diagnostic when the lock is stale, warn-and-proceed when the lock is empty or missing (the v0.6 init-scaffold default), and provide an `--allow-stale-lock` override flag for the rare cases where the operator knows better.

Market grounding (per the pattern set by G55's revised PRDs — see `PRD-record-auto-base.md:41`): WP-001 §5.2 documents 8 of 11 colliding patches in `copilot-api` arose from operators recording against a too-broad upstream reference; reconcile against the same drifted state would silently mis-classify those features as either drift or `upstream_merged`, depending on phase. Persona Pat's audit JTBD treats reconcile output as evidence of upstream alignment; that evidence is only trustworthy if the lock the verdicts are computed against is itself trustworthy. `gbp-pq --time-machine N` (`competitive-landscape.md` §9) validates that deterministic upstream-history search is useful prior art, but we adopt only the **search** primitive there (PRD-patch-already-upstream-detector), not gbp-pq's drop-and-recreate model (`competitive-landscape.md` §10).

The seam is sharp: today, `PreflightReconcile` validates the working tree but trusts the lock implicitly. Every reconcile verdict downstream — including any future `upstream_merged` auto-classification from PRD-patch-already-upstream-detector phase 1.5 — is computed against `upstream.lock.commit` as the lower bound of "what's new upstream." If that commit no longer exists in the configured branch's history (force-push, branch rename, or stale lock from a prior reconcile that didn't update the file), every verdict downstream is wrong. This PRD closes that seam.

## 2. Motivation

### 2.1 Persona grounding

**Platform Pat** (`personas.md` §"Persona 1") owns the audit JTBD: "explain to a regulator why every patch the fork carries is intentional, traceable, and validated against current upstream." Pat reads `tpatch reconcile` output as evidence-of-validation. If the lock is stale, the output is evidence the agent ran *against the wrong baseline*, not evidence of upstream alignment. From Pat's perspective, that's an audit defect, not a UX nit. The lock-guard converts the silent failure into an explicit refusal with a one-step recovery, which is the audit-friendly behavior.

**Maintainer Mira** (`personas.md` §"Persona 3") owns the reasoning-preservation JTBD: "future-me, plus a coding agent, must be able to reconstruct *why* each feature exists and *how* it relates to current upstream." When the lock is stale, the canonical patch's `Tpatch-Base-Commit` trailer (per `PRD-tpatch-land.md` §3.4) and the lock's `commit` field point at different baselines. Mira's tooling silently produces inconsistent provenance. Refusing here forces the operator to align them before reconcile commits new state, preserving the invariant.

**Security Sam** (`personas.md` §"Persona 2") owns the forwardability JTBD for hotfixes. The lock-guard interacts with hotfix-kind features through the same code path: `PRD-tpatch-hotfix.md` does not change reconcile; hotfix-kind features go through normal reconcile and inherit the lock-guard automatically. There is no special case for hotfixes; this is a property, not a coincidence.

### 2.2 Prior-art grounding

`competitive-landscape.md` §9 lists two pieces of relevant prior art:

1. **`gbp-pq --time-machine N`**: deterministic upstream-history search. We adopt the **search** primitive in `PRD-patch-already-upstream-detector` phase 1.5. This PRD adopts the **input-validation** principle: don't search if you don't know which range to search.
2. **"Auto-detect patches already applied upstream via git rebase"**: this is gbp-pq's behavior on `git rebase`. Validating that the rebase target (in our case, the upstream HEAD relative to the lock) is consistent is the precondition for the detection to be meaningful. This PRD provides that precondition.

`competitive-landscape.md` §10 lists what we **don't** take from gbp-pq: the drop-and-recreate model. The lock-guard is the opposite philosophy: detect inconsistency loudly, do not auto-rewrite, surface state for human/agent decision.

### 2.3 The seam — why preflight is the right injection point

Today's `PreflightReconcile` (`internal/gitutil/gitutil.go:117-178`) checks four working-tree conditions: unstaged, untracked, merge markers, leftover `.orig`/`.rej` files. It is the canonical "everything must be clean before reconcile runs" gate. The v0.4.2 anti-pattern refusal documented at `docs/reconcile.md:45-72` is the precedent: silent corruption of verdicts → loud refusal at preflight → `--allow-dirty` override for the rare experts.

Lock-staleness has the same shape:

| Property | Working-tree dirty (v0.4.2) | Lock stale (this PRD) |
|---|---|---|
| Silently corrupts verdicts? | Yes — `git apply --check` reads file bytes. | Yes — phases 1–4 are computed against the wrong base. |
| Loud refusal acceptable? | Yes — operator knows how to recover. | Yes — operator can update the lock. |
| Override flag required for experts? | Yes (`--allow-dirty`). | Yes (`--allow-stale-lock`). |
| Diagnostic must be actionable? | Yes — recovery commands listed. | Yes — `tpatch upstream check` or manual update commands. |
| Default state on fresh init? | Clean tree (no preflight violation). | **Empty** lock (the v0.6 scaffold default — must NOT refuse). |

The last row drives the warn-vs-refuse split in §3.1: the v0.6 init scaffolds an empty lock, and refusing every reconcile on every fresh repo would be a usability disaster. Empty/missing locks warn-and-proceed; populated-but-stale locks refuse with override available.

Reverse seam-check: there is no other obvious injection point. The reconcile command body itself runs phase 1 first, by which time the lock has already been read; injecting between phase-1 and phase-2 would mean some verdicts have already been computed. Preflight is the only place where the guard fires *before* any verdict-mutating logic. No new preflight type is needed — the existing `ReconcilePreflight` is the correct extension target, and the brief explicitly requires this.

## 3. User-Facing Contract

### 3.1 Lock-state taxonomy

The guard classifies the lock into exactly one of five states. This taxonomy is the contract for everything downstream:

| State | Definition | Default policy | Override flag |
|---|---|---|---|
| **Valid** | `upstream.lock` is populated (the three load-bearing fields — `Commit`, `Remote`, `Branch` — are non-empty; `URL` is decorative and may be empty since the v0.4 writer never populates it) AND `lock.commit` resolves AND `IsAncestor(lock.commit, <lock.remote>/<lock.branch>)` returns true after the writer-normalization check (§5.3). | Proceed silently; lock is trustworthy. | n/a |
| **Empty** | `upstream.lock` exists on disk but `lock.commit == ""` (the v0.6 init scaffold default; or any time the operator clears the file). | Warn and proceed; treat as "first reconcile, no recorded baseline yet." | n/a — proceeding is the default |
| **Missing** | `upstream.lock` does not exist on disk. | Warn and proceed; treat as if Empty (legacy repos pre-init or hand-edited away). | n/a |
| **Stale** | `upstream.lock` is populated but `lock.commit` is **not** an ancestor of `<lock.remote>/<lock.branch>` HEAD (force-push, branch rename, or stale lock from a prior reconcile that didn't update). Includes the sub-case where `lock.commit` does not resolve at all (commit absent from local repo). | **Refuse** with diagnostic and recovery hint. | `--allow-stale-lock` |
| **Skipped** | The reconcile invocation passed `--upstream-ref <ref>` where `<ref>` differs from `<lock.remote>/<lock.branch>`. The guard cannot validate the lock against a ref the lock does not key on. | Print a one-line note ("lock-guard skipped: --upstream-ref overrides lock-keyed branch") and proceed. | n/a — domain mismatch, no opinion |

**Stale sub-cases — precise definitions:**

- **STALE-COMMIT**: `lock.commit` is non-empty AND `lock.commit` resolves to an object in the local repo AND `IsAncestor(lock.commit, <lock.remote>/<lock.branch>)` returns false.
- **STALE-RESOLVE**: `lock.commit` is non-empty AND `lock.commit` does NOT resolve in the local repo (commit pruned, force-push GC, or operator typed a wrong SHA).
- **STALE-REF**: `lock.commit` is non-empty BUT either (a) `<lock.remote>` or `<lock.branch>` is itself empty/malformed (partial lock — operator started populating the lock but didn't finish; the lock's own claim about which ref to validate against is broken), OR (b) the recorded `<lock.remote>/<lock.branch>` ref does not resolve in the local repo at all (branch renamed/deleted/never fetched). Both sub-shapes share a verdict: refuse — the lock's claim about its keying ref is not reconcilable with current local state.

Note: `URL` empty is **not** a STALE-REF trigger. The v0.4 writer at `internal/workflow/reconcile.go:601` hard-codes `url: ""`, so requiring `URL` non-empty would classify every existing populated lock as Stale on day one. URL is decorative metadata for human-readable lock inspection; it is not part of the validation contract. See §5.3 for the writer-normalization shape this PRD requires.

These three sub-cases share one verdict (Stale → refuse) but have different diagnostic strings; see §3.2.

**Implementation note on Empty vs Missing**: from a UX perspective these are the same (both warn-and-proceed). They are kept separate in the taxonomy because the diagnostic strings differ ("lock file is empty" vs "lock file does not exist"), which matters for operator debugging. The implementation may collapse to a single warning if the diagnostic distinction is judged not load-bearing; the brief is silent on this and §9 records it as an open question.

### 3.2 Refusal-vs-warn policy

| State | Behavior | Diagnostic shape |
|---|---|---|
| Valid | Silent (no extra output beyond existing reconcile diagnostics). | n/a |
| Empty | One-line warning to stderr, then proceed: `warning: .tpatch/upstream.lock is empty (commit: ""). Reconcile verdicts will be computed against --upstream-ref directly. This is normal for a fresh repo.` | Warning, exit-code unchanged. |
| Missing | One-line warning to stderr, then proceed: `warning: .tpatch/upstream.lock not found. Run 'tpatch init' to scaffold it, or proceed: reconcile will use --upstream-ref directly.` | Warning, exit-code unchanged. |
| Stale (any sub-case) | **Refuse** with diagnostic block to stderr; exit-code non-zero. Mirrors the v0.4.2 dirty-tree refusal shape. | See block below. |
| Skipped | One-line note to stderr (informational, not a warning): `lock-guard skipped: --upstream-ref <override> overrides .tpatch/upstream.lock-keyed branch (<remote>/<branch>).` | Informational, exit-code unchanged. |

**Stale refusal block** (target shape — exact wording subject to copy-edit during implementation, the structure is the contract):

```
error: reconcile refused — .tpatch/upstream.lock is stale.

  recorded commit:    <lock.commit>
  recorded branch:    <lock.remote>/<lock.branch>
  current ref HEAD:   <resolved-head-sha>
  reason:             <one-line per sub-case: STALE-COMMIT|STALE-RESOLVE|STALE-REF>

To recover, pick one:

  # 1. Update the lock to match current upstream (recommended):
  tpatch upstream check                # if implemented
  # or, if upstream check is stubbed (v0.7 cut):
  git fetch <lock.remote>
  # then re-run any reconcile; the next reconcile that completes with a
  # populated lock will overwrite the file with current state.

  # 2. Override (emits a warning; verdicts may be wrong):
  tpatch reconcile --allow-stale-lock [other flags]

  # 3. Reconcile against an explicit ref different from the lock-keyed
  #    branch. This triggers the lock-state SKIPPED path (the guard
  #    cannot validate a lock keyed on a branch you are not reconciling
  #    against). The override ref-name must DIFFER from
  #    <lock.remote>/<lock.branch>; passing the same ref-name as the
  #    lock keys on validates normally per §3.3.
  tpatch reconcile --upstream-ref <ref-different-from-lock-keyed-branch>

This refusal exists because reconcile verdicts are computed against the
recorded baseline. A stale baseline produces wrong verdicts silently.
Refusing now is louder than the wrong verdict later.
```

`--allow-stale-lock` is the canonical override flag. It mirrors `--allow-dirty` (`docs/reconcile.md:79`) in shape: bypass the preflight for this one invocation, print a one-line warning, proceed. The flag is **not** persistent — it must be passed on every invocation where staleness is acceptable, by design (an override that lingers becomes invisible).

The default position is **strict refusal on Stale**, mirroring G55's `--allow-collision <reason>` pattern in `PRD-record-collision-detection.md` §3.1. Rationale: the cost of a wrong reconcile verdict is asymmetric — Pat's audit trust is the headline product property (`personas.md` §"Persona 1"); a few false-positive refusals while operators learn to keep the lock fresh are a much cheaper failure mode than silently wrong `upstream_merged` classifications.

### 3.3 Interaction with `--upstream-ref <ref>`

The existing `--upstream-ref <ref>` flag (`docs/reconcile.md:80`) overrides the upstream ref that reconcile compares against. The lock-guard's interaction is:

1. If `--upstream-ref` is **not** passed: validate the lock against `<lock.remote>/<lock.branch>`. State ∈ {Valid, Empty, Missing, Stale}.
2. If `--upstream-ref <ref>` is passed AND `<ref>` resolves to the same ref-name as `<lock.remote>/<lock.branch>`: validate as in (1). The flag is functionally a no-op for the guard — the operator just spelled out the default explicitly.
3. If `--upstream-ref <ref>` is passed AND `<ref>` resolves to a different ref-name than `<lock.remote>/<lock.branch>`: state = **Skipped**. The lock is keyed on a different branch than the operator wants to reconcile against, so the guard cannot meaningfully validate. Print the informational note and proceed.

The intent of (3) is to support the legitimate use case "I want to reconcile this fork against an experimental upstream branch without rewriting the lock." Pat may use `--upstream-ref upstream/v2.0-rc` for a one-off audit while keeping the lock keyed on `upstream/main`. Refusing in that case would be hostile.

Edge case: if the operator passes `--upstream-ref` AND `--allow-stale-lock`, the lock-guard never fires in either path (Skipped state from `--upstream-ref` short-circuits before the override flag is evaluated). The override flag in this combination is a no-op, but the CLI accepts it without error to simplify scripting.

Override-stacking with `--allow-dirty`: orthogonal. The two preflight conditions check different things (working tree vs lock state). Both warnings/refusals can fire on the same invocation; both override flags can be passed together. See §9 open question: do we allow stacking by default? **Default position v1**: yes, stack freely; both warnings emit; no error if both flags are passed. Re-evaluate at v0.7+30 days if the audit data shows operators routinely passing both as a habit.

### 3.4 Recovery hint surface — `tpatch upstream check`

`tpatch upstream check [--path]` is declared at `SPEC.md:72` as the canonical "check for new upstream versions" verb. At PRD draft time (2026-05-10), no implementation exists in `internal/cli/cobra.go`. The lock-guard's recovery hint must handle both cases:

- **If `tpatch upstream check` is implemented at v0.7-cut**: the diagnostic block points at it as recovery step 1 (`tpatch upstream check` → updates the lock to current upstream HEAD).
- **If `tpatch upstream check` is still stubbed at v0.7-cut**: the diagnostic falls back to manual recovery (`git fetch <lock.remote>` then re-run; the next completing reconcile rewrites the lock with current state).

The PRD does not require `upstream check` to be implemented as a precondition for shipping the guard. Both diagnostic shapes are acceptable; the implementer chooses based on the feature's status at v0.7-cut. The acceptance criterion (§8) tests both modes: in the stubbed-mode test, the diagnostic must mention `git fetch` as recovery; in the implemented-mode test, it must mention `tpatch upstream check` first.

If `upstream check` exists but is broken or hangs, that is `upstream check`'s bug, not the lock-guard's. The guard prints the recovery hint; the operator runs it; if it fails, the operator falls back to manual `git fetch`. The guard does not invoke `upstream check` itself.

## 4. Detection Algorithm

The decision tree fires inside `PreflightReconcile`, after the existing four working-tree checks. It is read-only and bounded — every check is one git command that runs in O(1) on repo size.

**Inputs**: `repoRoot string`, `upstreamRefOverride string` (empty if `--upstream-ref` not passed).

**Steps**:

1. **Load lock**:
   - Read `<repoRoot>/.tpatch/upstream.lock`.
   - If file does not exist: `LockState = Missing`. Return.
   - If file exists but is empty bytes or contains only whitespace/comments: treat as Empty.
   - Otherwise call `LoadUpstreamLock(<path>)` (the shared parser primitive — see §5). Returns `(UpstreamLock, error)`.
   - On parse error: log the parse error verbatim, then `LockState = Missing` with a sub-diagnostic noting the parse failure. Reasoning: a malformed lock is operationally identical to a missing lock for guard purposes. We don't refuse on parse errors because that would block recovery; we warn-and-proceed and let the operator fix the file at their leisure.

2. **Empty check**:
   - If `lock.Commit == ""`: `LockState = Empty`. Return.

3. **Partial check** (STALE-REF sub-case (a)):
   - If `lock.Commit != ""` AND (`lock.Remote == ""` OR `lock.Branch == ""`): `LockState = Stale`, sub = STALE-REF. Return.
   - Note: `lock.URL` empty is **not** a partial-lock trigger. The v0.4 writer at `internal/workflow/reconcile.go:601` hard-codes `url: ""`; requiring URL non-empty would classify every existing populated lock as partial. URL is decorative.

4. **Legacy-writer normalization** (writer-bug compatibility, see §5.3):
   - The v0.4 writer at `internal/workflow/reconcile.go:595-605` writes `branch: <upstreamRef>` where `<upstreamRef>` may include the remote prefix (e.g., `branch: upstream/main` after `--upstream-ref upstream/main`). This produces locks whose `<lock.remote>/<lock.branch>` would be `upstream/upstream/main` — a ref that does not resolve.
   - Apply this normalization on read: if `lock.Branch` contains a `/` AND its leading slash-segment exactly equals `lock.Remote`, strip the leading `<lock.Remote>/` segment from `lock.Branch` before any subsequent ref resolution. Document this as legacy-tolerance behavior; the §5.3 writer-fix removes the need going forward.
   - This is read-side normalization only. The parser does not rewrite the file; the next successful reconcile (which already updates the lock) will write the corrected form once §5.3's writer-fix lands.

5. **`--upstream-ref` skip check**:
   - If `upstreamRefOverride != ""`:
     - Resolve `<lock.Remote>/<lock.Branch>` (post-normalization from step 4) to a **symbolic full ref-name** via `git rev-parse --symbolic-full-name <lock.Remote>/<lock.Branch>` (e.g. `refs/remotes/upstream/main`). **Not** `git rev-parse <ref>` — that resolves to a SHA, and SHA equality would treat two different branches pointing at the same commit as the same lock key, contradicting the "keyed branch" model in §3.3.
     - Resolve `upstreamRefOverride` to a symbolic full ref-name the same way.
     - If the two symbolic full ref-names differ: `LockState = Skipped`. Return.
     - If they match: continue to step 6 (validate normally; the flag was a no-op for the guard).
   - Implementation note: `git rev-parse --symbolic-full-name` returns the canonical `refs/...` form; comparing those strings is the contract. If either side fails to symbolic-resolve (e.g. ambiguous short-name with no remote-tracking ref), fall back to STALE-REF for the lock side and proceed for the override side. Tests must include the same-SHA-different-branch case (two branches at the same commit must classify as Skipped, not Valid-via-skip).

6. **Resolve check** (STALE-RESOLVE sub-case):
   - Run `git rev-parse --verify <lock.Commit>^{commit}`.
   - If exit non-zero: `LockState = Stale`, sub = STALE-RESOLVE. Return.

7. **Ancestor check** (STALE-COMMIT sub-case — primary path):
   - Resolve `<lock.Remote>/<lock.Branch>` (post-normalization from step 4) to a SHA via `git rev-parse <lock.Remote>/<lock.Branch>`.
   - If that ref does not resolve: `LockState = Stale`, sub = STALE-REF (the recorded ref-name itself doesn't exist locally, e.g. branch was renamed/deleted). Return.
   - Run `IsAncestor(lock.Commit, <resolved-head-sha>)`.
   - If false: `LockState = Stale`, sub = STALE-COMMIT. Return.

8. **Valid**:
   - All checks passed. `LockState = Valid`. Return.

**Field on `ReconcilePreflight`**: add two fields (struct tags omitted; see §7):

```go
type ReconcilePreflight struct {
    // ... existing four file-list fields unchanged ...
    LockState      LockState           // Valid | Empty | Missing | Stale | Skipped
    LockDiagnostic LockDiagnostic      // sub-cause + cite + recovery hint key
}

type LockState int
const (
    LockStateUnknown LockState = iota
    LockStateValid
    LockStateEmpty
    LockStateMissing
    LockStateStale
    LockStateSkipped
)

type LockDiagnostic struct {
    SubCause     string  // "" for Valid/Empty/Missing/Skipped; "STALE-COMMIT"|"STALE-RESOLVE"|"STALE-REF" for Stale
    LockCommit   string  // verbatim from upstream.lock
    LockRefName  string  // "<remote>/<branch>"
    HeadSHA      string  // resolved <remote>/<branch> HEAD, "" if unresolved
    ParseError   string  // verbatim parser error if any, else ""
}
```

`Clean()` is **not** extended to include `LockState`. The existing `Clean()` predicate stays strictly working-tree-only (the four file-list fields). Reasoning: extending `Clean()` would conflate two independent override flags. With the conflated predicate, an operator passing `--allow-stale-lock` on a clean tree with a stale lock would still fall through the `if !p.Clean()` gate and hit the dirty-tree refusal (because `!Clean()` would be true via `LockState=Stale`, but the only override the cli sees there is `--allow-dirty`). Working-tree state and lock state are co-equal preflight conditions for the same verb, but they have separate override flags and must be evaluated separately at the call site. See §7.3 for the corrected pseudocode.

Implementation note: `LockState` and `LockDiagnostic` are deliberately *not* persisted to disk. Per §0.3 non-goals, lock-state is in-memory preflight diagnostic, not part of the data model. This preserves the WP-001 §5.2 row 11 invariant (no new data-model objects) and the byte-identical `status.json` round-trip guarantee.

**Cost analysis**: at most 3 git invocations on a Stale-check path (`rev-parse` for `lock.Commit`, `rev-parse` for `<remote>/<branch>`, `merge-base --is-ancestor`); at most 2 on Empty/Missing paths; 0 if `--upstream-ref` overrides to a different ref. All bounded by repo size only via git's standard ref resolution; no walks. No network calls.

## 5. Coordination with PRD-record-auto-base — the shared parser

`PRD-record-auto-base.md` §5 (line 205) commits to: "Add a small parser for `.tpatch/upstream.lock`; the existing file is YAML-like but simple enough for the repo's current zero-dep style."

This PRD requires the **same parser**. Two consumers, one primitive — exactly the pattern CO47's brief flagged.

### 5.1 Parser primitive

**Location**: `internal/store/upstream_lock.go` (new file). The store package already owns the upstream.lock scaffolding (`internal/store/store.go:100-110`), so the parser belongs there.

**Signature**:

```go
// LoadUpstreamLock reads .tpatch/upstream.lock from the given store
// root and returns the parsed lock plus a parse-state classification.
// Returns LockStateUnknown only on I/O errors; parse errors yield
// LockStateMissing with a diagnostic, since a malformed lock is
// operationally a missing lock for both consumers.
func LoadUpstreamLock(storeRoot string) (UpstreamLock, ParseState, error)

type ParseState int
const (
    ParseStateOK ParseState = iota
    ParseStateFileMissing
    ParseStateFileEmpty
    ParseStateMalformed
)
```

The parser returns the raw `UpstreamLock` value plus a parse-state. The lock-guard's `LockState` taxonomy is *derived* from `ParseState` plus the ancestry check; auto-base's baseline-inference algorithm is *derived* from `ParseState` plus its own field-by-field dispatch (PRD-record-auto-base.md §3.2 steps 2–4 already dispatch on `lock.Commit != ""` and `lock.Remote, lock.Branch != ""`). Both PRDs consume the same parser, derive their state from its result.

**Format conventions** (matching the v0.6 init scaffold at `internal/store/store.go:100-110`):

- Flat YAML-like key:value pairs (`remote: ""`, `branch: ""`, `commit: ""`, `url: ""`).
- `#` introduces line comments (the scaffold already uses two leading comment lines).
- Quoted string values; quotes optional for empty strings.
- No nested keys, no arrays, no anchors.
- Unknown keys are tolerated and ignored (forward-compat).

This matches the existing flat-scalar config-parsing convention documented in repo memory (citation: `internal/store/store.go:456-497` for the config parser; the upstream-lock parser follows the same pattern).

### 5.2 Coordination contract

| Question | Answer |
|---|---|
| Who writes the parser file? | Whichever PRD ships first writes `internal/store/upstream_lock.go`. The other PRD imports it. No duplicate parsers. |
| Who tests the parser? | Both. Each consumer adds focused tests for *its* dispatch logic; one set of parser-level tests (in `internal/store/upstream_lock_test.go`) lives with the parser file. |
| Can the parser surface change? | Yes, before either PRD lands. After the first lands, breaking changes require updating the consumer; both authors must agree. |
| Does the parser ever write the lock? | No. Writing is a separate verb (`tpatch upstream check` per SPEC.md:72, or future `tpatch reconcile`'s end-of-run lock-update). The parser is read-only. |
| Where does the parser tolerate quote/whitespace variation? | Inside the value: trimmed leading/trailing whitespace; **double-quoted values only**, matching the v0.6 init scaffold (`internal/store/store.go:100-110`) and the v0.4 reconcile writer (`internal/workflow/reconcile.go:595-605`). Single-quoted values, bare values, multi-line values, and YAML anchors are rejected and produce `ParseStateMalformed`. The scaffold uses `""` for empties; honoring that exact byte sequence is required for backwards-compat. Operators who hand-edit the lock with non-double-quoted values get the malformed-warn-and-proceed path (treat as Missing for guard purposes; treat as missing for auto-base purposes). Permitting multiple quote styles in v1 was rejected as unnecessary complexity that would create round-trip ambiguity for any future writer. |
| What error messages does the parser emit? | Parser returns `(UpstreamLock, ParseState, error)`. The `error` is non-nil only on I/O. `ParseState == Malformed` carries no error (consumer decides what to do). Each consumer formats its own diagnostic. |

**Coordination order** (for whichever ships first):

- If `PRD-record-auto-base` ships first: G55 writes `internal/store/upstream_lock.go` per §5 of that PRD. OX47's lock-guard implementation imports it, no rework.
- If `PRD-reconcile-lock-guard` ships first: OX47 writes `internal/store/upstream_lock.go` per this PRD. G55's `record --auto` implementation imports it; G55's PRD §5 line is satisfied by import, no parser duplication.
- If they ship in the same wave: one author writes the parser as part of their feature branch; the other author's branch rebases on top after the parser merges. Coordination happens in code-review or supervisor handoff, not in this PRD.

**Cross-cite**: this section is cited by `PRD-record-auto-base.md` §5 (G55, after this PRD lands and the supervisor confirms parser location). G55's PRD does not need to be edited here; the cite is one-directional in v1 (this PRD knows about auto-base; auto-base learns about this PRD when supervisor reviews).

### 5.3 Writer-normalization — precondition for the guard

**This PRD requires a sibling fix to the existing lock writer.** Without it, the guard would refuse on the very first reconcile after a populated lock is written.

**The bug.** `internal/workflow/reconcile.go:595-605` writes:

```go
func updateUpstreamLock(s *store.Store, ref, commit string) {
    content := fmt.Sprintf(`# Upstream Lock
# Updated by tpatch reconcile at %s
remote: upstream
branch: %s
commit: %s
url: ""
`, time.Now().UTC().Format(time.RFC3339), ref, commit)
    // ...
}
```

Two problems:

1. **`remote: upstream` is hard-coded.** If the operator's actual remote is `origin` (single-remote setup, common in personal forks), the lock claims a remote that doesn't exist in their repo.
2. **`branch: %s` is the full `--upstream-ref` value.** With the default `--upstream-ref upstream/main`, `branch` becomes `upstream/main`, and `<lock.remote>/<lock.branch>` resolves to `upstream/upstream/main` — a ref that does not exist. Every populated lock written by v0.4 reconcile is malformed under the lock-guard's contract.

**Required writer shape (v0.7 fix).** Update `updateUpstreamLock` to split the upstream ref into `(remote, branch)` and validate both before writing:

```go
func updateUpstreamLock(s *store.Store, upstreamRef, commit string) error {
    // Split "upstream/main" into ("upstream", "main").
    // Refuse to write if upstreamRef has zero or more than one slash;
    // refuse if either segment is empty.
    remote, branch, ok := splitUpstreamRef(upstreamRef)
    if !ok {
        return fmt.Errorf("malformed upstream ref %q (expected <remote>/<branch>)", upstreamRef)
    }
    // Optionally populate URL from `git remote get-url <remote>` if available;
    // empty URL is acceptable (decorative, not validated by the guard).
    url, _ := gitRemoteURL(s.RepoRoot(), remote)  // ignore error; empty is fine
    // Write all four fields, double-quoted, matching the scaffold format.
    // ...
}
```

**Legacy tolerance.** §4 step 4 already specifies read-side normalization for pre-fix locks: if `lock.Branch` contains `/` AND its leading slash-segment equals `lock.Remote`, strip the leading segment. This means:

- Pre-fix locks written by v0.4 reconcile (`remote: upstream`, `branch: upstream/main`) classify as Valid through legacy tolerance — they are not refused on day one.
- The next successful reconcile re-writes the lock with the normalized form, eliminating the legacy case.
- After ~one reconcile cycle per repo, all locks are in the v0.7 normalized shape; the legacy-tolerance code path can be removed in a future cleanup PRD.

**Coordination ownership**: the writer fix lives in `internal/workflow/reconcile.go` (existing file). It is **not** in the lock-guard's path; it is a sibling change that ships in the same PR/wave as the guard. The implementer of the lock-guard PRD must also touch `updateUpstreamLock` (or coordinate with whoever does). PRD-record-auto-base does **not** write the lock, so this fix does not ripple into that PRD's implementation.

**Out of scope.** Any operator-facing UX for choosing the upstream remote (e.g. `tpatch init --remote origin`) is out of scope here. The writer fix takes whatever `--upstream-ref` the operator passed at reconcile time and splits it. If operators routinely pass `origin/main`, the lock will record `remote: origin`, `branch: main` — that is correct, no special handling needed.

**Acceptance criterion**: §8 adds tests for both the writer-normalized output and the legacy-lock read-side tolerance. The guard must pass these tests in addition to the existing reconcile suite.

## 6. Backwards Compatibility

### 6.1 The empty-lock landmine — why warn-not-refuse

`tpatch init` at `internal/store/store.go:100-110` writes `upstream.lock` with all four fields empty by default. Every fresh tpatch repo at v0.6 has an empty lock until the operator runs the first reconcile (which populates it) or manually edits the file. If the lock-guard treated Empty as Stale and refused, **every fresh repo's first reconcile would fail**.

Resolution: Empty and Missing both warn-and-proceed. The taxonomy keeps them as separate states because the diagnostic strings differ, but the policy is identical.

This is a deliberate weakening of the strict-refusal default. Justification:

- Empty is the documented zero-state. An empty lock makes no false claim about upstream — it makes no claim at all. Refusing on no-claim is hostile.
- The first reconcile's job is to populate the lock from current upstream HEAD. Refusing first reconcile prevents the only mechanism for moving Empty → Valid.
- Pat's audit JTBD does not require pre-population. Pat cares that *populated* claims are accurate, not that every repo carries a populated claim before any reconcile has ever run.
- The warning is non-empty and points at the safe default ("verdicts will be computed against `--upstream-ref` directly"). Silent success would be objectionable; warning success is acceptable.

### 6.2 Pre-v0.6 repos (no `.tpatch/upstream.lock` file)

The lock file was added in v0.4 alongside reconcile primitives. Repos initialized before v0.4 (none in case-study scope, but possibly some hand-rolled repos) have no `.tpatch/upstream.lock` file. The Missing state covers this exact case: warn-and-proceed, suggest `tpatch init` to scaffold.

### 6.3 The `--allow-stale-lock` override semantics

The flag is **session-scoped**: it applies to one invocation only, must be re-passed on subsequent invocations where staleness is acceptable. There is no persistent suppression mechanism (no `.tpatch/lock-guard.disabled` file, no `Config.IgnoreStaleLock`). This is intentional — a persistent suppression that lingers across reboots is the kind of override that becomes invisible and erodes the audit trail Pat depends on.

If field evidence at v0.7+30 days shows operators routinely passing `--allow-stale-lock`, the response is **not** to add persistence; it is to investigate why operators have stale locks routinely and fix the root cause (probably: `tpatch upstream check` isn't implemented yet, so the lock auto-refresh path is missing — that's the work, not a flag with longer reach).

### 6.4 Existing reconcile tests

All existing reconcile tests must remain green. The lock-guard adds new diagnostic state but does not change the verdict computation in phases 1-4. Existing test fixtures with populated upstream.lock files continue to pass through the guard as Valid (assuming their `lock.commit` is reachable from `<lock.remote>/<lock.branch>` — which it should be, since the fixtures were authored to be self-consistent).

If any existing test fixture has a *stale* lock (intentionally or otherwise), the test will start failing under the lock-guard. Resolution: add `--allow-stale-lock` to the test invocation, or update the fixture's lock. The PRD recommends the second; staleness in test fixtures is a bug the guard correctly surfaces.

### 6.5 Forward-compatibility with future preflight conditions

The `ReconcilePreflight` struct is the canonical "things that must be OK before reconcile runs" container. Future PRDs may add more conditions (e.g., a `feat-reconcile-tracked-tpatch` guard that warns when `.tpatch/` is not in `git ls-files`). The pattern this PRD establishes — typed state field + diagnostic struct, policy applied at the call site — is the template. New conditions should follow the same shape.

## 7. Implementation Notes

### 7.1 Where the guard fires

Inside `PreflightReconcile` at `internal/gitutil/gitutil.go:117-178`, after the existing `*.orig`/`*.rej` walk and the `sort.Strings` calls, before `return p, nil`:

```go
// Existing four file-list conditions populated above this line.

// Lock-guard: classify upstream.lock state for the caller's policy
// decision. This does not mutate the lock; it only reads it.
state, diag := classifyLockState(repoRoot, upstreamRefOverride)
p.LockState = state
p.LockDiagnostic = diag

return p, nil
```

`classifyLockState` is a new private helper in the same file (or in `internal/gitutil/lock_guard.go` if file size warrants splitting; see §9 open question). It implements the §4 algorithm using existing `IsAncestor`, `MergeBase`, `ResolveRef` helpers.

### 7.2 Threading `--upstream-ref` to the preflight

The existing signature `PreflightReconcile(repoRoot string)` does not know about `--upstream-ref`, but the guard's Skipped state needs it. Two implementation shapes are acceptable; the **preferred** shape is the sibling function:

**Preferred**: add a sibling `PreflightReconcileWithOverride`:

```go
// PreflightReconcile inspects the working tree and the upstream lock for
// conditions that make reconcile verdicts unreliable. It is read-only.
// Equivalent to PreflightReconcileWithOverride(repoRoot, "").
func PreflightReconcile(repoRoot string) (ReconcilePreflight, error)

// PreflightReconcileWithOverride is like PreflightReconcile but takes the
// value of --upstream-ref (or "" if not set). When non-empty and resolving
// to a different ref-name than the lock keys on, the lock-guard's verdict
// is Skipped.
func PreflightReconcileWithOverride(repoRoot, upstreamRefOverride string) (ReconcilePreflight, error)
```

`PreflightReconcile` delegates to `PreflightReconcileWithOverride(repoRoot, "")`. Existing callers that don't know about `--upstream-ref` (preflight-only CI gates that just check working-tree state) keep their current call site verbatim. New callers (cli.go reconcile command) call the override variant.

**Alternative considered and rejected**: a breaking signature change `PreflightReconcile(repoRoot, upstreamRefOverride string)`. Rejected because the override is rare-path (most invocations don't pass `--upstream-ref`), the existing single-arg signature is well-documented and used by `--preflight` CI gates, and a breaking change ripples into every test that constructs a preflight. The sibling-function shape preserves backward compatibility for existing callers and tests at the cost of one extra exported name; that trade is correct here.

### 7.3 Policy application at the call site

The reconcile command body inspects `p.LockState` after preflight, before phase 1, and applies the policy (refuse on Stale unless `--allow-stale-lock`; warn on Empty/Missing; note on Skipped; silent on Valid). This keeps `PreflightReconcile` policy-free; the gitutil package classifies, the cli package decides. Mirrors the existing pattern where `PreflightReconcile` reports dirty-tree conditions and the cli decides whether `--allow-dirty` was passed.

**Working-tree state and lock state are evaluated independently.** This is the corrected pseudocode (see §4 for why `Clean()` is **not** extended):

```go
// in cobra.go reconcile command:
p, err := gitutil.PreflightReconcileWithOverride(repoRoot, upstreamRef)
if err != nil { return err }

// 1. Working-tree gate (existing semantics, untouched).
if !p.Clean() {  // Clean() remains working-tree-only
    if !allowDirty { return formatDirtyTreeRefusal(p) }
    fmt.Fprintln(stderr, dirtyTreeOverrideWarning(p))
}

// 2. Lock-state gate (independent of working-tree gate; separate
//    override flag). Evaluated even when the tree is clean.
switch p.LockState {
case gitutil.LockStateValid:
    // silent
case gitutil.LockStateEmpty:
    fmt.Fprintln(stderr, formatEmptyLockWarning())
case gitutil.LockStateMissing:
    fmt.Fprintln(stderr, formatMissingLockWarning())
case gitutil.LockStateStale:
    if !allowStaleLock { return formatStaleLockRefusal(p.LockDiagnostic) }
    fmt.Fprintln(stderr, formatStaleLockOverrideWarning(p.LockDiagnostic))
case gitutil.LockStateSkipped:
    fmt.Fprintln(stderr, formatSkippedLockNote(p.LockDiagnostic))
}

// 3. Proceed to phase 1.
```

The two gates are co-equal — either can refuse independently of the other; either can be overridden independently of the other. A clean tree with a stale lock + `--allow-stale-lock` proceeds. A dirty tree with a valid lock + `--allow-dirty` proceeds. A dirty tree with a stale lock requires both override flags to proceed; this is intentional (see §3.2 override-stacking note and §9.3).

**Why not extend `Clean()`?** An earlier draft of this PRD proposed extending `Clean()` to include `LockState != Stale`. G55's review (2026-05-10) correctly flagged that this conflates two independent override flags: `if !p.Clean()` followed by `if !allowDirty` would refuse on stale-lock-but-clean-tree even when the operator passed `--allow-stale-lock` (because the only override the conflated gate sees is `--allow-dirty`). Keeping the predicates separate avoids this footgun; the two gates compose cleanly via plain `if` statements.

### 7.4 Flag wiring

Add `--allow-stale-lock` as a boolean flag on the reconcile command, mirroring `--allow-dirty`. Default false. Help text:

```
--allow-stale-lock    Bypass the upstream.lock validation guard.
                      Verdicts may be computed against a baseline that no
                      longer exists in the configured upstream branch.
                      Use only when you understand why the lock is stale
                      and accept that downstream verdicts may be wrong.
```

The flag is **not** persisted in any config file (see §6.3). It applies to one invocation only.

### 7.5 Diagnostic strings

Diagnostic strings live in a new file `internal/cli/lock_guard_diag.go` (or alongside the dirty-tree diag if that's a single file at implementation time). Strings are template-formatted with the `LockDiagnostic` fields. They are user-facing copy and should be reviewed by whoever owns operator-facing error messages at implementation time; this PRD specifies the *structure* (fields, recovery commands, sub-cause routing) but not the final wording.

### 7.6 Documentation updates

`docs/reconcile.md` must be updated to:

- Add a new "Upstream-lock validation" subsection between the v0.4.2 anti-pattern section (lines 45-72) and the Flags table (line 74).
- Add `--allow-stale-lock` to the Flags table (line 78).
- Add a new troubleshooting entry under "Troubleshooting" (line 83): "How do I update the lock?" → `tpatch upstream check` if implemented, else `git fetch` + re-run reconcile.

Skill assets in `assets/skills/**` may need a one-line update to mention the new failure mode; the parity-guard test (`assets/assets_test.go`) determines this at implementation time. If the skill files don't currently reference reconcile preflight (likely), no change is needed.

## 8. Acceptance Criteria

Concrete, supervisor-checklist style. Each must be testable independently.

1. **Valid lock → silent proceed**. Test fixture: lock with `commit = <SHA reachable from upstream/main>`. `tpatch reconcile` produces no lock-guard diagnostic; existing reconcile output is byte-identical to pre-PRD baseline.
2. **Empty lock → warning, then proceed**. Test fixture: lock with `commit: ""` (the v0.6 init scaffold default). `tpatch reconcile` emits the empty-lock warning to stderr and proceeds; exit code matches what the rest of reconcile would produce.
3. **Missing lock file → warning, then proceed**. Test fixture: `.tpatch/upstream.lock` deleted. `tpatch reconcile` emits the missing-lock warning to stderr and proceeds.
4. **Malformed lock file → warning, then proceed**. Test fixture: `.tpatch/upstream.lock` with non-YAML garbage. `tpatch reconcile` emits a parse-error warning + missing-lock-equivalent warning to stderr and proceeds. The malformed file is not auto-rewritten.
5. **Stale-COMMIT lock → refuse**. Test fixture: lock with `commit = <SHA that exists locally but is not an ancestor of upstream/main HEAD>`. `tpatch reconcile` exits non-zero with the stale-lock refusal block; the diagnostic includes the recorded commit, the recorded branch, the resolved HEAD, and "STALE-COMMIT" sub-cause.
6. **Stale-RESOLVE lock → refuse**. Test fixture: lock with `commit = <SHA that does not exist locally>`. `tpatch reconcile` exits non-zero; diagnostic includes "STALE-RESOLVE" sub-cause and recovery hints.
7. **Stale-REF lock (partial) → refuse**. Test fixture: lock with `commit != "" AND remote == ""`. `tpatch reconcile` exits non-zero; diagnostic includes "STALE-REF".
8. **`--allow-stale-lock` overrides Stale**. Same fixture as (5). With the flag, `tpatch reconcile` proceeds, prints a one-line override warning, and continues to phase 1.
9. **`--upstream-ref` to different ref → Skipped, no validation**. Test fixture: lock keyed on `upstream/main`; invocation passes `--upstream-ref upstream/v2.0`. `tpatch reconcile` prints the Skipped informational note and proceeds without consulting the lock's freshness.
10. **`--upstream-ref` to same ref → validates normally**. Test fixture: lock keyed on `upstream/main`; invocation passes `--upstream-ref upstream/main`. The lock-guard runs as if the flag were absent.
11. **`--allow-dirty` and `--allow-stale-lock` stack**. Test fixture: dirty tree AND stale lock. Both flags passed. Reconcile proceeds, both override warnings emit, no error from flag interaction.
12. **Recovery hint adapts to `tpatch upstream check` status**. Two test variants: (a) build with `upstream check` implemented → hint mentions `tpatch upstream check` first; (b) build without `upstream check` → hint mentions `git fetch` first. Both variants must compile from the same lock-guard code; the diagnostic wraps the conditional. (Implementer's choice on the conditional mechanism — feature flag, build tag, or runtime detection of the verb's existence.)
13. **Existing reconcile tests remain green**. `go test ./...` produces zero new failures. Existing fixtures whose locks are not deliberately stale must classify as Valid; if any existing fixture is stale, the PR updates the fixture or adds `--allow-stale-lock` to its invocation, with a comment explaining which.
14. **`docs/reconcile.md` updated** with the new subsection, the new flag entry, and the new troubleshooting entry. The doc's existing v0.4.2 anti-pattern section (lines 45-72) is preserved verbatim.
15. **Parity guard passes**. `assets/assets_test.go` continues to pass. If any skill file references reconcile preflight, update both the skill and the parity-guard fixture; otherwise no skill change.
16. **Writer-normalization (G55 review ripple, §5.3)**: `updateUpstreamLock` writes `remote: <split-remote>`, `branch: <split-branch>` — splitting `--upstream-ref` into the correct two segments. Test fixture: `--upstream-ref upstream/main` produces `remote: "upstream"`, `branch: "main"` (not `branch: "upstream/main"`). Refuses to write when `--upstream-ref` has zero or more than one slash, with a clear error.
17. **Legacy-lock tolerance (G55 review ripple, §4 step 4)**: a pre-fix lock with `remote: upstream`, `branch: upstream/main` (the v0.4 writer's output) classifies as Valid via the read-side normalization in §4 step 4. Test fixture covers both forms: pre-fix (`branch: upstream/main`) and v0.7-fixed (`branch: main`).
18. **Ref-name comparison uses symbolic full ref-name (G55 review ripple, §4 step 5)**: `--upstream-ref` to a *different branch pointing at the same commit* as the lock-keyed branch classifies as Skipped, not Valid. Test fixture: lock keyed on `upstream/main`; create local branch `upstream/main-mirror` pointing at the same SHA; invoke with `--upstream-ref upstream/main-mirror`. Expected: Skipped (different ref-name despite identical SHA).
19. **`PreflightReconcile` and `PreflightReconcileWithOverride` coexist (§7.2)**: the original single-arg `PreflightReconcile(repoRoot)` continues to work; existing tests using it pass without modification. New `PreflightReconcileWithOverride(repoRoot, upstreamRef)` accepts the override and threads it to the lock-guard. The single-arg variant delegates to the override variant with `""`.
20. **Independent gate evaluation (§7.3)**: clean tree + stale lock + `--allow-stale-lock` (without `--allow-dirty`) proceeds. Dirty tree + valid lock + `--allow-dirty` (without `--allow-stale-lock`) proceeds. The two override flags compose without either implying the other.

## 9. Open Questions

These are deliberately unresolved at PRD time. Each will be answered during implementation, in a follow-up PRD revision, or when v0.7+30-day audit data is available.

### 9.1 Strict-refuse-on-Stale: should the default flip to warn at v0.7+30 days?

The PRD ships strict refusal (the "louder is safer" default). If field evidence shows operators routinely have stale locks (because `tpatch upstream check` isn't implemented and the lock-auto-update path is missing), the override might become a habit. Two responses:

- **Response A (preferred)**: implement `tpatch upstream check` so the recovery hint actually works one-step. This is the v0.7-stretch line item. Once the auto-refresh path exists, strict refusal is genuinely cheap (operator runs `tpatch upstream check`, lock updates, re-runs reconcile).
- **Response B (fallback)**: flip the default to warn. Document the decision in an ADR. The flag becomes `--strict-stale-lock` (opt-in for audit-strict environments).

Default position: Response A. Re-evaluate at v0.7+30 days based on `--allow-stale-lock` invocation rate in case-study repos.

### 9.2 Empty vs Missing — should they collapse to one state?

The taxonomy keeps them separate for diagnostic distinctness; the policy is identical. If implementation finds the diagnostic distinction not load-bearing (operators don't act differently on Empty vs Missing), collapse to a single Absent state. PRD does not block this; it's an implementation-time judgment call.

### 9.3 Override-stacking with `--allow-dirty`

**Default position v1**: stack freely. Both warnings emit; both override flags can be passed together.

If audit data at v0.7+30 days shows operators routinely passing both as a habit (rather than as a one-off recovery), revisit. The harder failure mode would be both flags becoming a "force reconcile" macro that operators paste from muscle memory; that erodes the audit trail. Mitigation if observed: introduce a single `--force` umbrella flag with an explicit list of which preflight conditions it bypasses, requiring the operator to acknowledge what is being overridden.

### 9.4 Should the guard run when `--preflight` is passed (preflight-only mode)?

`docs/reconcile.md:78` documents `--preflight` as "Run only the preflight checks and exit. 0 = clean, non-zero = violations." With the lock-guard, a stale lock now contributes to the "non-zero on violations" signal. CI gates that already use `--preflight` will start failing on stale locks.

**Default position**: yes, the guard runs in `--preflight` mode and contributes to the exit code. CI gates failing on stale locks is correct behavior — that's the value of `--preflight` in CI. PRs introducing stale locks should fail CI before they merge.

If a CI gate wants to ignore lock-staleness (e.g., it intentionally tests against an experimental upstream), it passes `--allow-stale-lock` alongside `--preflight`. That combination is supported.

### 9.5 What if `lock.Remote/lock.Branch` requires a fetch to resolve?

Step 6 of §4 resolves `<lock.Remote>/<lock.Branch>` via `git rev-parse`. This requires the local repo to have an up-to-date remote-tracking ref. If the operator has not run `git fetch` recently, the local ref might point to an older HEAD than upstream actually has, and the guard might classify a *truly* stale lock as Valid (because the local ref still includes lock.commit in its history).

**Default position**: this is acceptable. The guard's job is to validate against the local view of upstream, which is what reconcile itself uses. If the local view is stale relative to true upstream, every reconcile verdict downstream is also computed against the stale view; the guard cannot fix that. Pre-fetching is the operator's responsibility (per `docs/reconcile.md:25-28` Pattern A guidance).

If audit data shows operators routinely forgetting to fetch, a future PRD can add a `--fetch` reconcile flag that runs `git fetch <lock.Remote>` before preflight. That is **not** in scope here. The lock-guard does not perform network operations.

### 9.6 Should `LockState` be persisted in `ReconcileSummary`?

`internal/store/types.go::ReconcileSummary` (existing) records the per-feature reconcile verdict. Per §0.3 non-goals, this PRD does **not** add lock-state to the summary; the diagnostic is in-memory only.

**Open question**: should v2 of this feature persist `LockState` in `ReconcileSummary` as an audit-trail aid for Pat? Pro: Pat can grep `status.json` for "this reconcile ran with --allow-stale-lock=true" — useful for audits. Con: violates WP-001 §5.2 row 11 ("no data-model gap") if read strictly; expands the surface area of `ReconcileSummary`.

**Default position**: defer. v1 keeps lock-state in-memory. If audit data at v0.7+30 days shows Pat asking for the trail in `status.json`, promote to v2 with an ADR.

## 10. Sources

Every market-research and codebase claim in this PRD is anchored to a file:line cite. This table is the audit shape.

| Claim | Source |
|---|---|
| `tpatch reconcile upstream-lock validation guard` is the third v0.7 deliverable. | `docs/market-research/competitive-landscape.md:475-478` |
| The SMART target's success criteria are zero new collisions + ≥50% legacy collision-group remediation, audited at v0.7+30 days. | `docs/market-research/competitive-landscape.md:477-485` |
| `gbp-pq --time-machine N` is the upstream-history-search prior art; we adopt the search primitive in `PRD-patch-already-upstream-detector`, not the drop-and-recreate model. | `docs/market-research/competitive-landscape.md` §9 + §10. |
| WP-001 §5.2 row 11 ratifies "no data-model gap." This PRD reinforces, does not contradict. | `docs/whitepapers/WP-001-feature-slice-gap.md:477,486`. |
| WP-001 graduated three PRDs at T16 (2026-04-28). This PRD originates from `competitive-landscape.md` §6 SMART (post-WP-001) and is independent of WP-001's graduation count. | `docs/whitepapers/WP-001-feature-slice-gap.md` §9 graduation plan (lines ~688-727); ratification turn at `docs/whitepapers/WP-001-feature-slice-gap.turns.md` Turn 13 (line 869, G55-authored, 2026-04-28). |
| `UpstreamLock` struct exists with 4 fields. | `internal/store/types.go:344-350` |
| `ReconcilePreflight` struct exists with four file-list fields and `.Clean()` predicate. | `internal/gitutil/gitutil.go:85-115` |
| `PreflightReconcile` is read-only. | `internal/gitutil/gitutil.go:117-178` |
| `tpatch init` scaffolds an empty upstream.lock. | `internal/store/store.go:100-110` |
| The v0.4.2 dirty-tree refusal precedent. | `docs/reconcile.md:45-72` |
| `--upstream-ref` and `--allow-dirty` flags. | `docs/reconcile.md:79-80` |
| `tpatch upstream check` declared at SPEC. | `SPEC.md:72`. Implementation status: stubbed at draft time. |
| `PRD-record-auto-base.md` §5 commits to a parser primitive. | `docs/prds/PRD-record-auto-base.md:205` |
| `PRD-record-auto-base.md` §3.2 baseline-inference dispatches on lock fields. | `docs/prds/PRD-record-auto-base.md:109-147` |
| `PRD-tpatch-land.md` §6.1 names this PRD as v0.7 deliverable (3). | `docs/prds/PRD-tpatch-land.md:651-672` |
| `PRD-patch-already-upstream-detector.md` §3.1 step 1 cites this PRD as a precondition for phase 1.5. | `docs/prds/PRD-patch-already-upstream-detector.md:177-181` |
| Persona Pat's audit JTBD. | `docs/market-research/personas.md` §"Persona 1" (lines 47-101) |
| Persona Mira's reasoning-preservation JTBD. | `docs/market-research/personas.md` §"Persona 3" (lines 165-223) |
| Persona Sam's forwardability JTBD. | `docs/market-research/personas.md` §"Persona 2" (lines 103-163) |
| Repo's flat-scalar config-parsing convention. | Memory citation: `internal/store/store.go:456-497`. |
| `IsAncestor`, `MergeBase`, `ResolveRef`, `FetchUpstream` exist. | Referenced by `PRD-record-auto-base.md:206`. |

## 11. Cross-review notes

### 11.1 For CO47 (PRD-tpatch-land owner)

Verdict on `PRD-tpatch-land.md` §6.1 v0.7 SMART target wording (lines 651-672): **correct, no drift**. The four-deliverable framing (land + collision-detection + lock-guard + record --auto-as-remediation) matches the market-research SMART exactly (`competitive-landscape.md:475-485`); deliverable (3) names this PRD as `PRD-reconcile-lock-guard` (line 659); the implementation-independence note ("they cover different verbs and can ship in either order", line 671) is accurate — the lock-guard fires inside `PreflightReconcile`, which `land` does not invoke.

No edit requested to `PRD-tpatch-land.md`. If CO47 wants to add a one-line cite back to this PRD's §5 (parser coordination), that's optional and CO47-owned.

### 11.2 For G55 (PRD-record-auto-base owner)

Parser coordination per §5 of this PRD: the shared primitive lives at `internal/store/upstream_lock.go` (new file). Whichever PRD ships first writes it; the other consumes it. G55's PRD §5 line ("Add a small parser for `.tpatch/upstream.lock`") is satisfied by import once the parser exists — no rewrite needed in `PRD-record-auto-base.md`. If G55 wants to add a one-line cross-cite, the location is `PRD-record-auto-base.md:205` after the existing parser sentence: e.g., "Coordinated with `PRD-reconcile-lock-guard` §5 — same primitive, two consumers."

This PRD does not edit `PRD-record-auto-base.md`. The cite is optional and G55-owned.

### 11.3 For the supervisor

Three ratification asks before this PRD enters formal acceptance:

1. **Confirm the WP-001 sibling framing** in §0.1 is correct: this PRD does not introduce data-model objects and therefore does not contradict §5.2 row 11. Spot-check the §0.3 non-goals list and the absence of any new struct fields persisted to disk.
2. **Confirm `tpatch upstream check` stubbed-vs-implemented branch** in §3.4 + acceptance §8.12 is acceptable. Alternative: gate the lock-guard on `upstream check` being implemented first (delays ship). This PRD takes the position that the guard ships independently; the diagnostic adapts.
3. **Confirm parser coordination** with G55. If G55 disagrees with the `internal/store/upstream_lock.go` location or the parse-state shape, this PRD's §5 is the owner of the renegotiation; both PRDs should re-state the agreed location before either lands.

Post-acceptance, an ADR to lock the `LockState` taxonomy and `--allow-stale-lock` semantics is appropriate. The ADR can be written by whichever agent picks up implementation; it should cite this PRD's §3.1 + §3.2 verbatim and document why strict-refuse-on-Stale was the v1 default (per §9.1).

---

*End of PRD-reconcile-lock-guard.*
