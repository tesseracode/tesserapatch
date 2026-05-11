# PRD — Patch-already-upstream detector (deterministic `upstream_merged` fast path)

**Status**: Draft
**Date**: 2026-05-09
**Owner**: Core (research-driven; no implementation owner yet)
**Milestone**: Reconcile fast-path improvements (post-M14, post-WP-001). Not yet on the roadmap.
**Depends on**:
  - [ADR-010](../adrs/ADR-010-provider-assisted-resolver.md) — phase-3.5 provider-assisted resolver. This PRD does not change ADR-010; it adds a phase-1.5 fast path *before* phase 3, reducing how often the provider is called.
  - [WP-001 §5.2](../whitepapers/WP-001-feature-slice-gap.md) — collision-shape claims. The detector is ineffective in the WP-001 collision regimes; this PRD claims wins only in the *non-collision-with-upstream-absorption* regime.

**Related**:
  - [`docs/market-research/personas.md`](../market-research/personas.md) — Persona 1 "Platform Pat" audit JTBD ("did upstream merge their fix? — drop ours") and Persona 2 "Security Sam" SLA JTBD (drop the CVE patch when upstream's fix lands).
  - [`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md) §9 "What we take" — the gbp-pq prior art row "Auto-detect 'patches already applied upstream' via `git rebase`" is the intellectual precedent for this PRD.
  - [`docs/reconcile.md`](../reconcile.md) — 4-phase reconcile (reverse-apply → operation-level → provider-semantic → forward-apply). This PRD inserts a phase-1.5.
  - [`internal/workflow/reconcile.go`](../../internal/workflow/reconcile.go) — phase orchestration. The new phase slots between `phase-1-reverse-apply` and `phase-2-operation-level`.
  - [`internal/store/types.go`](../../internal/store/types.go) `StateUpstreamMerged` is already a first-class lifecycle state and `ReconcileSummary` already carries an `Outcome`.
  - [PRD-tpatch-hotfix](PRD-tpatch-hotfix.md) — this detector is the deterministic mechanism behind the hotfix PRD's "upstream-merged hint" (§5.3). The two PRDs are independent but reinforcing.

---

## 0. Claims Audit

This PRD is a **proposal**. It changes nothing. The audit below verifies the
factual claims it relies on, against the codebase as of 2026-05-09.

| Claim | Verified? | Evidence |
|---|---|---|
| `StateUpstreamMerged` is a first-class lifecycle state. | ✅ | `internal/store/types.go:18` `StateUpstreamMerged FeatureState = "upstream_merged"`. |
| `ReconcileUpstreamed` is a first-class reconcile outcome (today emitted by phase 3). | ✅ | `internal/store/types.go:35` `ReconcileUpstreamed ReconcileOutcome = "upstreamed"`. |
| Reconcile today has 4 phases (1, 2, 3, 4) plus an opt-in phase 3.5 (ADR-010 provider-assisted resolver). | ✅ | `internal/workflow/reconcile.go:175` (phase 1), `:186` (phase 2), `:207` (phase 3), `:231` (phase 4), `:281` (phase 3.5). |
| Today, `upstream_merged` is reached only via phase 3 provider-semantic analysis (no deterministic fast path). | ✅ | `internal/workflow/reconcile.go:207-219` — the only branch that emits `Outcome=upstreamed`. ROADMAP §M4 closure states "Feature A → upstream_merged (Phase 3)". |
| `git patch-id` is available in any modern git installation; produces a hash stable across cherry-picks, minor whitespace, and trailing-newline normalisation. | ✅ | git documentation (`git help patch-id`); gbp-pq's reliance on `git rebase` (which uses patch-id internally) for "drop already-applied" is the existence proof. |
| `gitutil` package already wraps git commands; no new git binary dependency is introduced. | ✅ | `internal/gitutil/gitutil.go` is the existing wrapper; this PRD adds one method. |
| `upstream.lock` records the upstream commit baseline used by reconcile. | ✅ | `ReconcileSummary.UpstreamCommit` (`internal/store/types.go:236-241`); `docs/reconcile.md:1-80` describes the lock file. |
| The persona counter-evidence in `personas.md` lists "patch-already-upstream detector" as one of the five concrete backlog items. | ✅ | `docs/market-research/personas.md` Pat + Sam sections. |

**No code has been changed by this PRD.** Implementation lands behind a
flag (see §6) gated on supervisor approval and an explicit milestone
assignment.

---

## Summary

Add a **deterministic, provider-free** detector to `tpatch reconcile`
that fires the existing `upstream_merged` verdict when `git patch-id`
of the feature's `post-apply.patch` matches a `git patch-id` of any
commit in the upstream's reachable history range
`(upstream.lock.commit, upstream/HEAD]`.

This slots into the existing phase ladder as **phase 1.5**:

```
phase 1   reverse-apply check  (existing — does the patch revert cleanly?)
phase 1.5 patch-id sweep       ← NEW (this PRD)
phase 2   operation-level eval (existing)
phase 3   provider-semantic    (existing)
phase 3.5 provider-resolver    (existing — ADR-010 opt-in)
phase 4   forward-apply preview (existing)
```

The pitch is shaped by two persona JTBDs:

- **Platform Pat (audit):** "did upstream finally merge their fix? if
  yes, mark our patch as obsolete in the audit trail without me having
  to read 50 commits."
- **Security Sam (CVE drop):** "the CVE was patched upstream this
  morning; reconcile should figure that out without my LLM bill paying
  for a redundant semantic analysis."

The detector is **conservative by default** — it surfaces the verdict,
the user confirms removal. An opt-in `--auto-drop-merged` flag closes
the loop for users who want the full automatic path.

The detector is also **prior art**: gbp-pq has done this for ~18 years
via `gbp pq import`, and it works. We are porting the proven mechanism
into our seam.

---

## 1. Problem statement

### 1.1 Today's behaviour

Reconcile reaches the `upstream_merged` verdict **only via phase 3**
(provider-semantic LLM analysis). Phase 3 is:

- **Slow** — a network round-trip plus model inference.
- **Costly** — N tokens per reconcile attempt.
- **Non-deterministic** — phase-3 verdicts can vary across model
  versions or temperature settings.
- **Wasteful in the common case** — when upstream did a *clean
  cherry-pick* of the same fix (the most common upstream-absorbs case
  for CVE-driven hotfixes), the patches are byte-equivalent up to
  whitespace. No semantic analysis is needed. `git patch-id` answers
  "yes" in <100ms.

The two persona JTBDs that this PRD addresses are explicitly named in
`docs/market-research/personas.md` Counter-evidence subsections:

- **Pat (audit):** Pat reconciles 12 features against an upstream
  bump every Friday. Today every "upstream merged ours" verdict pays
  Pat's LLM bill. Most of those should be free.
- **Sam (CVE):** Sam's hotfix is upstream-absorbed within hours when
  the CVE is well-publicised. Pat's Friday reconcile finds the
  absorption — but only via phase 3. Sam wants this *deterministic*
  for his audit log.

### 1.2 Prior art validation

gbp-pq's `gbp pq import` runs the patch queue through `git rebase`
*before* re-applying. `git rebase` uses `git patch-id` internally to
detect that a commit has already been applied to the new base, and
**silently drops it**. gbp-pq has shipped this as the default for ~18
years across the Debian package ecosystem. The mechanism is correct,
fast, and battle-tested.

The thing tpatch lacks today is the *vocabulary* in our reconcile flow
to say "deterministic match" — every match has to go through the
provider, even the trivial ones. This PRD fixes that.

## 2. Goals / Non-goals

### Goals

1. A new phase 1.5 fast path that emits `Outcome=upstreamed` when
   `git patch-id` of the feature patch matches any commit in the
   upstream-since-lock range, without invoking the provider.
2. Phase 1.5 is **conservative**: it never *flips* a non-merged
   verdict to merged. It only short-circuits the provider call when
   the deterministic match is unambiguous.
3. New `tpatch reconcile --check-applied-only <slug>` flag — read-only
   sweep that prints the patch-id verdict and exits, useful for Pat's
   "did upstream merge ours?" status checks.
4. Optional `tpatch reconcile --auto-drop-merged <slug>` flag —
   when phase 1.5 fires and the user has opted into the loop closure,
   the feature is removed from the DAG and `status.json` is updated.
5. Detector reads `upstream.lock` for the baseline; it does not
   require the user to specify the range manually.
6. Heuristic-fallback friendly: phase 1.5 has no provider dependency.
   Works in `--no-provider` mode.
7. Hotfix-kind features (PRD-tpatch-hotfix) get `--auto-drop-merged`
   ON by default (subject to global config opt-in). Feature-kind
   features keep it OFF by default.
8. Backwards compatible: existing `status.json` round-trips
   byte-identical; no new persisted state by default. Phase 1.5
   verdict is emitted via the existing `ReconcileSummary` fields.

### Non-goals

1. **Replacing phase 3 entirely.** Phase 1.5 is a fast path. Phase 3
   is still authoritative when patch-ids don't match (e.g. upstream
   did a *non-cherry-pick* re-implementation that semantically
   matches but lexically differs).
2. **Cross-repo / distributed obsolescence-marker exchange.** That's
   ADR-015 D7 (deferred). This PRD is single-repo only.
3. **Detecting partial absorption.** "Upstream merged 60% of our
   patch" is a phase-3 problem; phase 1.5 only sees full
   patch-id matches.
4. **Auto-flipping the lifecycle state without user confirmation
   (default).** Auto-drop is opt-in.
5. **Walking arbitrary commit ranges.** Detector walks
   `(upstream.lock.commit, upstream/HEAD]` only. A "scan all of
   history" mode is out of scope (and would scale poorly).
6. **Phase ordering changes for non-`upstream_merged` outcomes.**
   Phase 2 / 3 / 4 fire identically when phase 1.5 does not match.

## 3. User-facing contract

### 3.1 Phase 1.5 (default behaviour)

Inserted between phase 1 (reverse-apply) and phase 2 (operation-level).

Algorithm:

1. Read `upstream.lock.commit` from the repo's `upstream.lock`.
   **Precondition**: per `PRD-reconcile-lock-guard`, the lock must
   be in a `valid` state (commit reachable from `<remote>/<branch>`
   HEAD) for this phase to run usefully. The lock-guard preflight
   refuses or warns on stale / empty / missing locks before this
   phase fires; phase 1.5 itself does no lock validation.
2. Read `post-apply.patch` for the feature being reconciled.
3. Compute `our_pid = git patch-id < post-apply.patch`.
4. Walk `git rev-list <upstream.lock.commit>..<upstream-ref>` (default
   `upstream/HEAD`).
5. For each `<sha>`, compute `their_pid = git patch-id <(git show
   --stat=0 --no-color --format= <sha>)`.
6. If `our_pid == their_pid` for some `<sha>` (the "match commit"):
   - Set `ReconcileSummary.Outcome = ReconcileUpstreamed`.
   - Set `ReconcileSummary.UpstreamCommit = <sha>` (the commit that
     absorbed our patch — already a documented field).
   - Set the feature's `State = StateUpstreamMerged`.
   - Set `ReconcileSummary.Phase = "phase-1.5-patch-id-match"` (new
     phase tag).
   - Skip phases 2 / 3 / 4.
7. If no match: continue to phase 2 unchanged.

Console output on match:

```
[reconcile] cve-2026-12345-validate-input
  phase 1   reverse-apply         OK
  phase 1.5 patch-id sweep        MATCHED upstream/main commit deadbee
  outcome   upstream_merged
  next      consider `tpatch feature remove cve-2026-12345-validate-input`
```

### 3.2 New flag — `--check-applied-only`

```
tpatch reconcile --check-applied-only <slug>
```

Read-only. Runs phase 1 + phase 1.5 only. Prints the verdict.
Exits 0 on match, 2 on no-match (so it composes with shell scripts).
Does **not** write `status.json` or any artifacts. Useful for Pat's
"any patches absorbed this week?" cron job.

### 3.3 New flag — `--auto-drop-merged`

```
tpatch reconcile --auto-drop-merged <slug>
```

When phase 1.5 fires:

1. The verdict is recorded in `status.json` as today.
2. The feature is removed from the DAG (subject to the same rules as
   `tpatch feature remove`: dependents are detached or migrated per
   ADR-011 cascade rules).
3. `Tpatch-CVE` and `Tpatch-Slug` trailers are preserved in the
   removal-commit message so the audit trail survives.

When phase 1.5 does not fire: `--auto-drop-merged` is a no-op; phases
2/3/4 fire as today.

`--auto-drop-merged` is **off** by default for feature-kind features
and **on** by default for hotfix-kind features (PRD-tpatch-hotfix
§5.3). The default can be flipped via `Config.AutoDropOnUpstreamMerge`.

### 3.4 New JSON output field

`tpatch reconcile --json <slug>` adds one nested object:

```json
{
  "outcome": "upstreamed",
  "phase": "phase-1.5-patch-id-match",
  "upstream_match": {
    "our_patch_id": "abc123…",
    "matched_upstream_sha": "deadbee…",
    "scanned_range": "upstream.lock.commit..upstream/main",
    "scanned_count": 17
  }
}
```

The `upstream_match` block is **omitted** when phase 1.5 did not match
(or did not run). Existing JSON consumers round-trip unchanged.

## 4. Schema additions

In `internal/store/types.go::ReconcileSummary` — **no new persisted
fields are required**. The existing `UpstreamCommit`, `Outcome`, and
`Phase` carry the verdict.

Optional (suggested but not required) field for richer audit:

```go
// PatchIDMatch, when set, records the patch-id sweep result that
// drove the `upstreamed` verdict. Empty when the verdict came from
// phase 3 provider-semantic analysis or did not fire. Persisted
// only when Config.PatchIDDetectorEnabled is true (default false
// until v0.7.x).
//
// `omitempty` is load-bearing for back-compat.
PatchIDMatch *PatchIDMatch `json:"patch_id_match,omitempty"`
```

```go
type PatchIDMatch struct {
    OurPatchID         string `json:"our_patch_id"`
    MatchedUpstreamSHA string `json:"matched_upstream_sha"`
    ScannedRange       string `json:"scanned_range"`
    ScannedCount       int    `json:"scanned_count"`
}
```

If the optional field is dropped from v1 (to ship the simplest
possible PRD), the verdict is still recoverable from
`(Phase == "phase-1.5-patch-id-match", UpstreamCommit, Outcome)`.

## 5. Behaviour

### 5.1 Patch-id calculation

- Use `git patch-id --stable` (the stable hash that ignores
  whitespace and line-number drift).
- Where the feature has both `post-apply.patch` and
  `incremental.patch`, prefer `post-apply.patch` (canonical per
  `docs/feature-layout.md`). The incremental form may match a
  partial absorption that *isn't* a real merge.
- Fail soft: if `git patch-id` returns non-zero, log a debug line
  and fall through to phase 2. **Never** treat a tooling failure as
  a no-match verdict (the no-match verdict has lifecycle
  consequences; tooling failure has none).

### 5.2 Walk strategy

- `git rev-list --no-merges <upstream.lock.commit>..<upstream-ref>`.
- Optional cap: `Config.PatchIDScanLimit` (default 5000 commits).
  When the range exceeds the cap, skip phase 1.5 entirely and
  log a hint suggesting `--upstream-ref <closer-ref>`. Pat's
  Friday reconcile against a 51-commit-behind upstream stays well
  under any reasonable cap; a "1-year-behind kernel fork" is
  out of scope for this PRD's claims.

### 5.3 Multi-match policy

If `git patch-id` matches more than one upstream commit (rare, but
possible when upstream cherry-picks a fix to multiple branches):

- Record **all** matches in `PatchIDMatch.MatchedUpstreamSHA` (or
  promote it to a list).
- Set `UpstreamCommit` to the **earliest** matching commit (closest
  to lock baseline). Rationale: that's when upstream actually
  absorbed the patch.
- Console output lists all matches.

### 5.4 Hotfix-kind interaction

Per PRD-tpatch-hotfix §5.3 + §5.4:

- Hotfix-kind features default to `--auto-drop-merged ON` *if* the
  global config flag `Config.AutoDropOnUpstreamMerge` is also ON.
- This double-gate is deliberate: the global flag is the operator's
  trust threshold; the feature-kind discriminator is the case-by-
  case opt-in.

### 5.5 Hard-parent satisfaction

When phase 1.5 fires and the feature transitions to
`StateUpstreamMerged`:

- Per ADR-011 D5, `SatisfiedBy` should be set to the matched
  upstream SHA on any child feature that depended on this parent.
- This satisfies the hard-dependency gate exactly as today's
  phase-3 path does. No change to dependency-gate code.

### 5.6 Heuristic-fallback friendliness

Phase 1.5 needs no provider. It runs in `--no-provider` mode.
Important for:

- Air-gapped environments.
- CI runners without secrets.
- Pat's Friday cron (no LLM budget required for the deterministic
  case).

## 6. Migration / rollout

- Behind config flag `Config.PatchIDDetectorEnabled` (default
  **false** until v0.7.x).
- When OFF: phase 1.5 is silently skipped; the existing 4-phase
  ladder runs unchanged. `PatchIDMatch` field never persists.
- When ON: phase 1.5 fires by default; `--auto-drop-merged` remains
  opt-in for feature-kind features.
- Pre-flag fixtures: zero churn — `omitempty` on the optional
  field guarantees byte-identical round-trip.
- **No SPEC change required at v0.** This PRD is research-driven;
  the supervisor assigns a milestone when scope locks.

## 7. Out of scope

- **Distributed obsolescence-marker exchange** (ADR-015 D7) —
  cross-repo "patch X was made obsolete by upstream commit Y"
  metadata travels via a custom protocol. Out of scope.
- **`git format-patch`-style multi-file patch threads.** This
  detector reads `post-apply.patch` (single multi-file patch).
- **Auto-creating an `upstream-merged-as` ledger.** Anyone wanting
  history can `git log --grep='Tpatch-Slug:' --grep='Tpatch-CVE:'`.
- **Cross-feature de-duplication** ("two of our features are the
  same patch"). Different problem.

## 8. Open questions

1. **Cap on the scanned range.** 5000 commits is a guess; needs
   validation against real fork-vs-upstream lag (e.g. t3code is
   currently ~5 days behind; copilot-api is ahead). Suggested:
   measure a few real cases before locking the default.
2. **Should `--check-applied-only` be a separate verb (`tpatch
   reconcile-status`) instead of a flag?** Verb is cleaner;
   flag is faster to ship. Default-position: flag for v1.
3. **Should phase 1.5 also handle `incremental.patch`?** Today the
   safer answer is no (per §5.1). Re-evaluate if data shows
   incremental-patch features absorb upstream more often than
   full-patch features.
4. **Should the verdict be persisted as `ReconcileUpstreamed`
   (existing) or as a new `ReconcileUpstreamedDeterministic`?**
   Keeping the existing outcome avoids a new enum value but loses
   the "this verdict came from a deterministic source" signal.
   Default-position: keep existing outcome; record the
   determinism in `Phase` ("phase-1.5-patch-id-match" vs
   "phase-3-provider-semantic"). Anyone needing the distinction
   reads `Phase`.
5. **Behaviour when `upstream.lock` is missing.** Skip phase 1.5
   silently and emit a one-line hint to run `tpatch upstream
   pin`? Default-position: yes — phase 1.5 is best-effort and
   never blocks reconcile.

## 9. Validation plan

1. Unit tests on `git patch-id --stable` invocation (with a fake
   git binary or fixture patches) — match / no-match / multi-match.
2. Unit tests on the upstream-range walk with a fixture history.
3. Integration test: feature applied, upstream cherry-picks the
   same patch, reconcile fires phase 1.5 and emits
   `upstream_merged` without any provider call.
4. Integration test: feature applied, upstream re-implements the
   same fix differently, reconcile fires phase 1.5 → no match,
   falls through to phase 3 unchanged.
5. Integration test: `--check-applied-only` returns exit 0 / 2
   correctly and writes nothing.
6. Integration test: `--auto-drop-merged` removes the feature on
   match and the audit-trail commit carries the right trailers.
7. Integration test: `--no-provider` mode + phase 1.5 match →
   `upstream_merged` verdict succeeds.
8. Backwards-compat test: pre-flag fixtures round-trip
   byte-identical with `Config.PatchIDDetectorEnabled = false`.
9. Heuristic-fallback test: provider misconfigured, phase 1.5
   still fires.

## 10. Cross-references

- **`docs/market-research/personas.md`** — Pat audit JTBD + Sam
  CVE-drop JTBD are the persona drivers.
- **`docs/market-research/competitive-landscape.md` §9 "What we
  take"** — gbp-pq prior art row "Auto-detect 'patches already
  applied upstream' via `git rebase`".
- **PRD-tpatch-hotfix §5.3** — this PRD is the deterministic
  mechanism behind hotfix's "upstream-merged hint".
- **ADR-011 D5** — `upstream_merged` already satisfies hard
  parents; phase 1.5 inherits that for free.
- **ADR-010** — phase 3.5 provider-resolver is unaffected; phase
  1.5 fires *before* phase 3, which is *before* phase 3.5.
