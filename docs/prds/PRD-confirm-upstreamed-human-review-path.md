# PRD - Confirm-Upstreamed Human Review Path

**Status**: Accepted
**Date**: 2026-07-31
**Owner**: Core
**Byline**: GH #4 drafter (session `ded62f1e`)
**Milestone**: Reconcile safety follow-up. Post-v0.12.0. Not yet roadmap-committed.
**Depends on**: [PRD-upstreamed-confirmation-gate](./PRD-upstreamed-confirmation-gate.md),
[PRD-reconcile-revision-pass-log](./PRD-reconcile-revision-pass-log.md),
[PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md),
[PRD-reconcile-retirement-state-audit](./PRD-reconcile-retirement-state-audit.md).
Implementation reuses `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [PRD-upstreamed-confirmation-gate](./PRD-upstreamed-confirmation-gate.md) — v0.11 gate this PRD completes
- [PRD-reconcile-revision-pass-log](./PRD-reconcile-revision-pass-log.md) — revision schema and CLI
- [PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md) — evidence schema and attempt-id anchoring
- [PRD-reconcile-retirement-state-audit](./PRD-reconcile-retirement-state-audit.md) — audit that must run on the transition
- [PRD-feature-supersession](./PRD-feature-supersession.md) — v0.12.0 supersession semantics (interaction)
- [ADR-013 verify-freshness-overlay](../adrs/ADR-013-verify-freshness-overlay.md) — reserves `amend --state`
- GH issue #4 (external report from t3code WSL v0.11.3 reconciliation)

## 0. Claims Audit

Every load-bearing claim about current behavior below is anchored to a
file:line. Any reviewer accepting this PRD should spot-check that the cites
land within ±5 lines of current `main` (`HEAD=5ac458d`).

| Claim | Evidence |
|---|---|
| The current `confirm-upstreamed` gate rejects rejected-upstreamed features with the exact error the reporter hit. | `internal/cli/cobra.go:2136-2138`: `if status.Reconcile.Outcome != store.ReconcileUpstreamed && status.Reconcile.ReviewVerdict != "confirmed-upstreamed" { return fmt.Errorf("confirm-upstreamed requires reconcile outcome %q or review_verdict %q for %s", store.ReconcileUpstreamed, "confirmed-upstreamed", slug) }`. |
| The gate reads only `status.json.Reconcile.{Outcome,ReviewVerdict}`; it does not consult `reconcile-revisions.jsonl`. | `internal/cli/cobra.go:2129-2138` — only `s.LoadFeatureStatus(slug)` is called; `store.LoadReconcileRevisions` is not. |
| `confirm-upstreamed` never mutates `status.State`; it only runs retirement audit and prints. | `internal/cli/cobra.go:2121-2172` — no `MarkFeatureState` or `status.State =` after the gate. All `MarkFeatureState` call sites are enumerated at `internal/cli/cobra.go:713,721,755,835,1476,2415`; none live inside `reconcileConfirmUpstreamedCmd`. |
| `updateFeatureState` is the only writer that sets `StateUpstreamMerged` from the reconcile path. | `internal/workflow/reconcile.go:1091-1094`: `case store.ReconcileUpstreamed: status.State = store.StateUpstreamMerged`. Also called out at `reconcile.go:984` for the revision-log finalState derivation. |
| `reconcile review add` persists verdict, action, final-state, reason-code, and evidence-attempt, but wires `ValidationRefs` to an empty slice — the CLI has no flag to attach an upstream commit reference to the revision. | `internal/cli/cobra.go:2227-2242`: `ValidationRefs: []store.ValidationRef{}`; the flag block at `cobra.go:2247-2253` has no `--upstream-commit`, `--validation-ref`, or equivalent. |
| The revision schema already supports arbitrary `ValidationRef{Kind,Value,Result}` entries and the codebase already uses `Kind: "upstream-commit"` for reconcile-auto revisions. | `internal/store/reconcile_revision.go:44-49` (`ValidationRef` struct); `internal/workflow/reconcile.go:1001-1003`: `validationRefs = append(validationRefs, store.ValidationRef{Kind: "upstream-commit", Value: result.UpstreamCommit, Result: "referenced"})`. |
| `amend --state` is deliberately reserved with exit-2, so operators have no supported state-transition command outside phase verbs. | `internal/cli/c1.go:261`: `cmd.Flags().String("state", "", "Reserved — amend does not accept arbitrary state transitions")`. Rejection message at `c1.go:275-283`. Rationale documented at `c1.go:257-273` (ADR-013 D3). |
| The retirement audit's first check verifies that `status.State`, `Reconcile.Outcome`, and `Reconcile.ReviewVerdict` agree on retirement. | `internal/workflow/retirement_audit.go:38`: `if status.State != store.StateUpstreamMerged || status.Reconcile.Outcome != store.ReconcileUpstreamed || (status.Reconcile.ReviewVerdict != "" && status.Reconcile.ReviewVerdict != "confirmed-upstreamed") { add("retirement-confirmed", ...) }`. |
| The retirement audit already checks reachability of `Apply.BaseCommit` via `gitutil.IsAncestor(HEAD)` and produces `retirement-audit-*` cleanup revisions. | `internal/workflow/retirement_audit.go:41`, `115-118`, `147-162`. |
| The revision log honours append-only semantics via `SupersedesEntryID`; supersession is explicit, and old entries remain in the file. | `internal/store/reconcile_revision.go:62` (`SupersedesEntryID`); `internal/cli/cobra.go:2303-2321` (`latestRevisionEntries` filter honours supersedure). See also PRD-reconcile-revision-pass-log §5. |
| `reconcile review add` computes `EntryID` deterministically via `store.ComputeRevisionID(entry)`. | `internal/cli/cobra.go:2240`; `internal/store/reconcile_revision.go:75-81`. |
| `ReviewVerdict` on `ReconcileSummary` is a free-string field; downstream code compares against string literals like `"confirmed-upstreamed"` and `"rejected-upstreamed"`. | `internal/store/types.go:318`; comparisons at `internal/cli/cobra.go:2107-2108, 2137`, `internal/workflow/retirement_audit.go:38`, `internal/workflow/reconcile.go:983-988`. |
| v0.12.0 supersession stores `kind: "supersedes"` on `depends_on` edges; superseded historical features are excluded from the effective set only when the superseder is healthy (`applied`/`active`/`upstream_merged`). | `internal/store/types.go:299` (`DependencyKindSupersedes`); `PRD-feature-supersession.md` §3.5. |

No code, schema, command behavior, or asset text is changed by this PRD.

## 1. Summary

The v0.11 upstreamed confirmation gate correctly downgrades an unproven
provider-semantic `upstreamed` verdict to `blocked` /
`review_verdict=rejected-upstreamed`. After a human independently validates
that the feature really was absorbed upstream and records that finding
with `tpatch reconcile review add`, no supported CLI path exists to
transition the feature to `upstream_merged` and run the retirement audit.

This PRD proposes a safe, evidence-linked human completion path that
(a) validates the operator-supplied upstream commit, (b) mutates
`status.Reconcile.{Outcome,ReviewVerdict,UpstreamCommit}` and `status.State`
under the same guardrails as automated confirmation, (c) preserves the
append-only evidence/revision chain, and (d) runs the retirement audit
established by PRD-reconcile-retirement-state-audit.

## 2. Problem Statement

The confirmation gate exists because raw provider verdicts were unreliable
enough to silently retire local feature code (see t3code case study). The
gate is correctly conservative:

> `confirm-upstreamed requires reconcile outcome "upstreamed" or
> review_verdict "confirmed-upstreamed"`
> — `internal/cli/cobra.go:2137`

However, the reporter shows a legitimate reconciliation that produces
`outcome=blocked`, `review_verdict=rejected-upstreamed`,
`reason=missing-upstream-commit-ref`. The human then:

1. independently validates that the upstream tree does contain the feature
   intent, and
2. identifies the adopting upstream commit, and
3. records their finding with `tpatch reconcile review add ... --verdict
   confirmed --action confirmed-retired --final-state upstream_merged
   --reason-code manual-review`.

Step 3 succeeds and appends a valid revision entry. But nothing consumes it:
`confirm-upstreamed` still reads `status.Reconcile`, sees the automated
`rejected-upstreamed` verdict, and refuses.

Because `amend --state` is reserved (`internal/cli/c1.go:261,275-283`), no
alternate command can flip the feature to `upstream_merged`. Operators are
left with two bad choices:

- Manually edit `status.json` — bypasses the audit chain the gate was
  meant to protect.
- Leave the feature permanently blocked — undermines reconciliation
  progress for a case the human has actually validated.

Neither is acceptable long-term. The gate's audit guarantees are being
undermined by the very failure mode it was designed to catch.

## 3. Existing Primitives Audit — "Could this be done without new schema?"

Before proposing new fields, this section enumerates the primitives already
in the codebase that the fix could reuse.

### 3.1 Fields `reconcile review add` already persists

Per `internal/cli/cobra.go:2227-2242`, a revision written today already
carries (attempt-id anchoring semantics per
[PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md)):

| Field | Source flag | Reporter's step-4 value |
|---|---|---|
| `EvidenceAttemptID` | `--evidence` | `re_1125ecf82225` |
| `RawReconcileVerdict` | `--raw-verdict` (defaults to status outcome) | `upstreamed` |
| `ReviewVerdict` | `--verdict` | `confirmed` |
| `ActionTaken` | `--action` | `confirmed-retired` |
| `ReasonCode` | `--reason-code` | `manual-review` |
| `FinalFeatureState` | `--final-state` | `upstream_merged` |
| `EntryID` | derived (`ComputeRevisionID`) | `rr_<12hex>` |
| `SchemaVersion` | constant | `1` |

That is enough to authorize a transition. The verdict-triple `(verdict=confirmed,
action=confirmed-retired, final_state=upstream_merged)` is unambiguous and
already the canonical "human approved retirement" tuple in
`internal/workflow/reconcile.go:983-988`.

### 3.2 The one gap: upstream commit reference

`ReconcileRevision.ValidationRefs []ValidationRef` **is schema-supported**
(`internal/store/reconcile_revision.go:44-49`) and the codebase **already
writes** `ValidationRef{Kind: "upstream-commit", Value: <sha>, Result:
"referenced"}` for reconcile-auto revisions
(`internal/workflow/reconcile.go:1001-1003`). But `reconcile review add`
initialises `ValidationRefs: []store.ValidationRef{}`
(`internal/cli/cobra.go:2238`) and exposes no `--upstream-commit` /
`--validation-ref` flag.

**Conclusion**: the fix does not require a new revision field. It requires
(a) a way for the operator to attach an `upstream-commit` validation ref
to their review revision, and (b) a consumer that reads it and completes
the transition. No new persisted metadata; both parts sit inside existing
primitives.

### 3.3 State-mutation primitives already present

- `store.MarkFeatureState(slug, state, command, notes)` — the standard
  writer (`internal/store/store.go:379-380`).
- `updateFeatureState` in `internal/workflow/reconcile.go:1058-1094` shows
  the canonical sequence for setting `StateUpstreamMerged`: populate
  `status.Reconcile.{Outcome,ReviewVerdict,UpstreamCommit}`, set
  `status.State`, set `status.Notes`, set `status.UpdatedAt`.
- `AuditRetirement` + `AppendRetirementCleanupRevisions`
  (`internal/workflow/retirement_audit.go:28,147`) already produce the
  post-transition cleanup findings and revisions.

Everything the fix needs is already in the codebase. The design is a wiring
change, not a schema change.

## 4. Design

Three options were evaluated (all three from GH #4). Recommendation:
**D1**.

### D1 — Recommended: extend `confirm-upstreamed` to consume the latest non-superseded review revision

Signature:

```bash
tpatch reconcile confirm-upstreamed <slug> \
    --upstream-commit <sha> \
    [--evidence <attempt-id>] \
    [--from-revision <entry-id>]
```

Behaviour:

1. Load `status.json`.
2. If `status.Reconcile.Outcome == upstreamed` **or** `status.Reconcile.ReviewVerdict == "confirmed-upstreamed"`, follow the existing (v0.11) fast path unchanged. This preserves byte-identical behavior for the automated case.
3. Otherwise, load `reconcile-revisions.jsonl` via `store.LoadReconcileRevisions` and select the latest non-superseded entry whose triple is `(ReviewVerdict=confirmed, ActionTaken=confirmed-retired, FinalFeatureState=upstream_merged)`. `--from-revision <entry-id>` overrides selection to a specific entry (useful for CI and tests).
4. If no such revision exists, refuse with a specific error naming the required tuple and pointing at `tpatch reconcile review add`. Do **not** re-emit the v0.11 error verbatim — the new message is a distinct failure mode.
5. Require `--upstream-commit <sha>` when taking the review-driven path. The sha must satisfy the safety checks in §7.
6. Append a **new** revision entry recording the transition itself: `RawReconcileVerdict = <prior outcome>`, `ReviewVerdict = confirmed`, `ActionTaken = confirmed-retired`, `FinalFeatureState = upstream_merged`, `ReasonCode = confirmed-via-review`, `ValidationRefs = [{Kind: "upstream-commit", Value: <sha>, Result: "verified"}, {Kind: "source-revision", Value: <consumed entry_id>, Result: "consumed"}]`, `SupersedesEntryID = <consumed entry_id>`. This is append-only — the prior revision remains in the file.
7. Mutate `status.Reconcile.Outcome = ReconcileUpstreamed`, `status.Reconcile.ReviewVerdict = "confirmed-upstreamed"`, `status.Reconcile.UpstreamCommit = <sha>`, `status.State = StateUpstreamMerged`, `status.Notes = "Feature adopted by upstream — confirmed via human review revision <entry_id>"`, `status.UpdatedAt = now`.
8. Run `AuditRetirement` + `AppendRetirementCleanupRevisions` exactly as the fast path does today. Preserve v0.11's guarantee that the audit runs on every confirmed transition.
9. Emit human or JSON output as today (`--format` / `--json`), with the JSON payload additionally naming the consumed `source_revision_entry_id`, the operator-supplied `upstream_commit`, and the newly appended `revision_entry_id`.

Rationale:

- **Reuses `reconcile review add` unchanged.** The human's step-3 revision is the authorization record. No schema change; no new flag on `review add`.
- **Single verb for the transition.** Operators already know `confirm-upstreamed`; the review path is a new invocation shape, not a new verb.
- **Preserves audit chain.** Append-only supersedure links the consumed revision to the transition revision to the retirement-audit findings. Every step is queryable via `reconcile review list --all`.
- **Backward compatible.** The v0.11 automated path is byte-identical for pre-existing repos.
- **Zero new persisted fields.** Uses `ValidationRefs[Kind="upstream-commit"]` (existing precedent at `internal/workflow/reconcile.go:1001-1003`) and `SupersedesEntryID` (existing schema).

### D2 — Alternative: `--evidence + --upstream-commit` on `confirm-upstreamed`, no revision consumption

The operator supplies the sha and evidence attempt-id directly on the
confirm-upstreamed command line. The command:

1. Bypasses the gate,
2. Constructs the transition revision itself (same shape as D1 step 6),
3. Mutates `status.json` (same as D1 step 7),
4. Runs the audit.

Rejected because:

- It provides no scaffolding for two-person review (one person writes the
  revision, another confirms). The `reconcile review add` primitive
  becomes decorative.
- The `--evidence <attempt-id>` referenced by the operator would need to
  point at *something*. If it points at the original evidence that lacked
  an upstream commit ref (the reporter's `re_1125ecf82225`), the sha would
  live only on the transition revision — not linked back to a
  human-authored review record. The revision log's audit purpose is
  weakened.
- The reporter listed it explicitly as an option, but the invariant "every
  transition consumes an authoring revision" is worth preserving.

### D3 — Alternative: new `tpatch reconcile review apply <entry-id>` verb

Adds a third `reconcile review` subcommand alongside `add` and `list`. The
verb explicitly names the revision to apply. `confirm-upstreamed` stays
unchanged.

Rejected because:

- Two verbs are needed for a single lifecycle transition (`review add` +
  `review apply`). Operators must remember both, and CI scripts get
  longer.
- The transition still needs `--upstream-commit`, so the verb doesn't
  actually shrink the flag surface — it just moves it.
- Splits the retirement audit invocation from the confirmation verb it
  currently lives with (`internal/cli/cobra.go:2148-2159`). The
  operator's mental model is "confirm = run audit"; splitting it risks
  the audit becoming optional in practice.

D3 is not strictly worse than D1 — the trade-off is UX. If a future
review workflow becomes formally two-person (§7.4), D3 may return as a
follow-up. Not blocking for GH #4.

### D4 — Selection semantics for "latest non-superseded revision"

Reuse `internal/cli/cobra.go:2303-2321` `latestRevisionEntries` filter
(the same one `reconcile review list` uses without `--all`). Its
semantics — dedupe by `(feature_slug, evidence_attempt, review_verdict,
action_taken)` keeping last file-order entry, then drop entries pointed at
by `SupersedesEntryID` — are already the canonical "current review state"
projection. `confirm-upstreamed` MUST use the exact same filter so
`review list --all=false` and `confirm-upstreamed` agree on which entry
would authorize the transition.

**Tie-break (rev-1)**. When multiple non-superseded entries survive the
dedupe and match the authorising tuple (this happens when two humans
record reviews against *different* `evidence_attempt` ids, so the dedupe
key differs and both survive), select the last-in-file-order entry
(`RevisionLog.Entries[-1]`) — the same "newer entry wins" bias used by
the dedupe itself. Operators who want a specific older entry must pass
`--from-revision <entry-id>` to override.

### D5 — Error message shape

The v0.11 gate error must **not** change verbatim (byte-identity for the
existing automated failure). When the review-path selection returns no
matching revision, emit a distinct message, for example:

```text
confirm-upstreamed: feature %q has review_verdict %q and no non-superseded
revision authorises retirement. Record a review with:
  tpatch reconcile review add %s --verdict confirmed --action confirmed-retired \
      --final-state upstream_merged --reason-code manual-review [--evidence <id>]
then re-run confirm-upstreamed with --upstream-commit <sha>.
```

This is one of two distinct failure modes; both must have JSON-parseable
exit-code semantics (see §6 AC-9).

## 5. State Semantics

State transitions authorised by this PRD:

| Prior state | Prior `Reconcile.Outcome` | Prior `Reconcile.ReviewVerdict` | Command | New state | New `Outcome` | New `ReviewVerdict` |
|---|---|---|---|---|---|---|
| `blocked` | `blocked` | `rejected-upstreamed` | `confirm-upstreamed --upstream-commit <sha>` (with authorising revision) | `upstream_merged` | `upstreamed` | `confirmed-upstreamed` |
| `upstream_merged` | `upstreamed` | `` or `confirmed-upstreamed` | `confirm-upstreamed` (no flags) | `upstream_merged` (unchanged) | `upstreamed` (unchanged) | `confirmed-upstreamed` |
| any | any | any | `confirm-upstreamed` (no authorising revision, not fast path) | (unchanged) | (unchanged) | (unchanged), refused with §4 D5 error |

The transition is one-way in v1: this PRD does NOT define a
`revert-upstreamed` inverse. Operators wanting to undo must record a
superseding revision manually and are handled by a future PRD.

**Fast-path entry invariant (rev-1)**. The v0.11 fast path presumes
`status.State == StateUpstreamMerged` on entry — the shipped code path
that flips `Reconcile.Outcome` to `upstreamed` is `updateFeatureState`
(`internal/workflow/reconcile.go:1091-1094`), which sets both fields
atomically. If a future contract regression allows `Reconcile.Outcome ==
upstreamed` (or `ReviewVerdict == confirmed-upstreamed`) with
`status.State != StateUpstreamMerged` on entry, the fast path MUST refuse
with a diagnostic naming the state mismatch rather than silently succeed
and re-run the audit against inconsistent metadata. The review path is
symmetric on exit: it *writes* `StateUpstreamMerged` (D1 step 7); it does
not assume it on entry.

## 6. Acceptance Criteria

### AC-1 — Reporter's reproduction succeeds end-to-end

The exact 5-step reproduction from GH #4 must complete without editing
`status.json` by hand. Extended step 5:

```bash
tpatch reconcile confirm-upstreamed windows-wsl-support \
    --upstream-commit <sha> --json
```

Exits 0, prints a JSON payload naming the consumed source-revision
entry-id, the operator-supplied upstream commit, and the newly appended
transition-revision entry-id.

### AC-2 — v0.11 fast path is byte-identical

For a feature with `Reconcile.Outcome=upstreamed` and no review revision
in `reconcile-revisions.jsonl`, `confirm-upstreamed <slug>` (no flags)
produces the same status mutation, retirement audit findings, and stdout
bytes as it does on `main` today. This must be asserted with a golden
fixture.

### AC-3 — Review-path requires all three tuple fields

The command refuses with §4 D5 error when the latest non-superseded
revision has any of `ReviewVerdict != confirmed`, `ActionTaken !=
confirmed-retired`, or `FinalFeatureState != upstream_merged`. All three
tuple fields are load-bearing.

### AC-4 — `--upstream-commit` required on review path

When the fast path does not apply (`Outcome != upstreamed` AND
`ReviewVerdict != confirmed-upstreamed`), `--upstream-commit <sha>` is
mandatory. The command refuses with an explicit error naming the flag.

### AC-5 — Upstream commit reachability enforced

The supplied `<sha>` must satisfy the §7.1 preferred contract when an
upstream ref is resolvable: `gitutil.IsAncestor(s.Root, sha,
resolvedUpstreamRef)` where `resolvedUpstreamRef` is
`status.Reconcile.UpstreamRef` if non-empty else `git rev-parse
--symbolic-full-name @{upstream}`. If neither resolves, fall back to
`gitutil.IsAncestor(s.Root, sha, "HEAD")` (existing primitive at
`retirement_audit.go:115-118`) AND emit the §7.1 residual-risk warning.
If not reachable under whichever check applies, refuse with a diagnostic
naming both the sha and the ref against which reachability was checked.
Test both the upstream-ref path (TS 9.11) and the HEAD-only fall-back
with warning (TS 9.12) plus refusal (TS 9.6), including a synthetic
non-reachable sha under both branches.

### AC-6 — Append-only chain: new revision supersedes consumed one

The transition writes a new revision entry with `SupersedesEntryID =
<consumed entry_id>`. `reconcile review list <slug> --all` still returns
both entries; `reconcile review list <slug>` returns only the new one.
The consumed entry remains byte-identical in the file.

### AC-7 — Retirement audit runs on the transition

After the review-path transition, `runConfirmedUpstreamedRetirementAudit`
(`internal/cli/cobra.go:2174-2192`) runs unchanged: cleanup revisions get
appended for any stale `satisfied_by`, unreachable base commits, or
`dependent-broken` residues.

### AC-8 — Reporter's audit trail is queryable

After the reproduction, `tpatch reconcile review list windows-wsl-support
--all --json` returns at minimum three revision entries: (1) the original
automated `rejected-upstreamed` revision written by the v0.11 gate, (2)
the human-authored review revision with tuple `(confirmed,
confirmed-retired, upstream_merged)`, (3) the transition revision written
by `confirm-upstreamed` with `SupersedesEntryID` pointing at #2, plus any
retirement-audit cleanup revisions. All are queryable via existing
`reconcile review list` and never leave partial state on error.

### AC-9 — Distinct exit-code semantics for the three failure shapes

`confirm-upstreamed` returns non-zero with distinct, testable error
substrings for: (a) v0.11 gate rejection (unchanged wording, unchanged
error), (b) review path with no authorising revision, (c) review path
with unreachable `--upstream-commit`. Callers can grep for the shape.

### AC-10 — `reconcile review add` schema unchanged

This PRD does not touch `reconcile review add`'s CLI or storage schema.
Operators may still record a revision without an upstream commit; the
new authorising path just requires `confirm-upstreamed --upstream-commit`
to supply it separately. (D1 does NOT add `--upstream-commit` to
`review add`.)

### AC-11 — Supersession safety invariant respected

Refuse the transition when the feature has a healthy active
`supersedes`-edge target that is not yet `upstream_merged` (see §7.4).
Add a diagnostic naming the target slug.

### AC-12 — Deterministic revision content

The transition revision's `EntryID` is derived via
`store.ComputeRevisionID` (existing precedent at
`internal/cli/cobra.go:2240`). Given the same inputs, replaying the
transition twice produces byte-identical revision content up to
`ValidationRefs` ordering (which is already sorted by
`normalizeReconcileRevision`).

**Scope (rev-1)**. The byte-identity requirement applies only to the
fields that determine `EntryID` and downstream audit correctness on the
*transition* revision: `EntryID`, `FeatureSlug`, `EvidenceAttemptID`,
`RawReconcileVerdict`, `ReviewVerdict`, `ActionTaken`, `FinalFeatureState`,
`ReasonCode`, `ValidationRefs`, `SupersedesEntryID`, `SchemaVersion`. It
explicitly **excludes** `RecordedAt` (wall-clock, expected to differ across
replays) and the audit-cleanup revisions appended by
`AppendRetirementCleanupRevisions` (which timestamp per invocation and
carry findings that may vary as filesystem state evolves). Callers who
need bytewise-identical audit revisions across replays must inject a
frozen clock — that is out of scope for this PRD.

## 7. Safety Invariants

The confirmation gate exists because the automated verdict was
unreliable. The human-completion path must preserve every invariant the
gate protects.

### 7.1 Reachability of the operator-supplied upstream commit

`gitutil.IsAncestor(sha, HEAD)` proves the sha exists somewhere in the
current branch's history — it does **not** prove that upstream absorbed
the feature. An operator can cherry-pick, revert, or dev-branch any
commit into HEAD's ancestry and pass a HEAD-only check while upstream
never saw it. The reporter's original failure mode was literally
`missing-upstream-commit-ref`, so this v1 contract must be stronger than
"HEAD-only" whenever a real upstream anchor can be resolved automatically.

**Preferred contract**. Resolve an upstream ref in this order:

1. `status.Reconcile.UpstreamRef` on the feature, if non-empty
   (`internal/store/types.go:315` — the field is populated by the same
   reconcile paths that already set `UpstreamCommit`).
2. Otherwise `git rev-parse --symbolic-full-name @{upstream}` on the
   current branch — the operator's declared upstream tracking ref.

If either resolves to a ref R, require
`gitutil.IsAncestor(s.Root, sha, R)`. If not reachable from R, refuse
with a diagnostic naming both the sha and R.

**Fall-back contract**. If neither source yields a ref (detached HEAD,
unconfigured tracking, no `UpstreamRef` on the feature), fall back to
`gitutil.IsAncestor(s.Root, sha, "HEAD")` (the pre-rev-1 shape). Because
this is a strictly weaker check, the run MUST log a warning in the
confirm-upstreamed output naming the residual risk, of the shape:

> `warning: no upstream ref resolvable for %s; verified <sha> is reachable from HEAD only. Local operators can insert commits into HEAD's ancestry without upstream ever seeing them — audit before relying on this transition.`

The warning is text-only; the transition still proceeds. Operators
running under CI or in automation should set `UpstreamRef` on the
feature during reconcile (existing capability) so the fall-back never
fires in production.

**Rationale**. The reporter's failure mode was `missing-upstream-commit-ref`.
The fix must actually attach a verifiable ref, not just accept any string
that happens to be in the local commit graph. Preferring the upstream ref
when resolvable closes the primary gap; the fall-back keeps the command
usable when no anchor is available and makes the residual risk explicit.

**No new flags**. `--upstream-ref` on the CLI is deliberately NOT added
— the upstream ref is resolved automatically from persisted feature
metadata or the branch's tracking ref, matching the reporter's constraint
that the fix should not multiply the flag surface.

### 7.2 Append-only evidence chain

The transition:

- Appends a NEW revision entry with `SupersedesEntryID = <consumed>`.
- Never mutates or deletes the consumed revision.
- Never mutates or deletes prior evidence entries in
  `reconcile-evidence.jsonl` (which this PRD does not touch).

The retirement audit's `revision-log` check
(`internal/workflow/retirement_audit.go:71-82`) will now be satisfied by
either the consumed revision or the transition revision, both of which
satisfy the `ActionTaken == confirmed-retired` predicate at line 74.

### 7.3 Required revision fields — reject when incomplete

Match the conservatism of the current gate. If any of the three
authorising fields is missing on the selected revision, refuse (AC-3).
The gate rejects on missing evidence; the review path rejects on
incomplete review.

### 7.4 Supersession interaction (v0.12.0)

`upstream_merged` retires the feature; the feature is no longer part of
the effective replay set. Two supersession cases must be considered:

1. **The feature being confirmed is itself a superseder** (carries a
   `depends_on[]{kind: "supersedes"}` edge, per `internal/store/types.go:299`).
   Confirming it as `upstream_merged` is safe **only when the superseded
   target is in a state that will not be left inconsistent by the
   superseder's retirement**. The decision matrix, indexed by the
   target's current `status.State` and the shape of its supersession
   relation:

   | Target state | Target relation shape | Confirm A? |
   |---|---|---|
   | `applied` (healthy) | Superseder A replaces unshipped local behavior in target B | **Refuse** — retiring A drops the replacement while B is still active; audit B first |
   | `applied` (stale `satisfied_by`) | B claims A satisfies it but the linkage is stale (`satisfied_by` no longer resolves) | **Refuse** — same class as healthy `applied`; the supersession contract is not upheld until B's `satisfied_by` is refreshed or resolved |
   | `promoted` (not yet closed) | B was promoted but has not yet reached a terminal state | **Refuse** — the promotion contract expects the superseder to remain available until B closes |
   | `blocked` | B is blocked for reasons independent of A | **Refuse** — B needs its own resolution before A can retire; do not compound blockers |
   | `upstream_merged` | B has already been confirmed as absorbed upstream | **Proceed** — B is already retired; A's retirement is symmetric |

   The refusal diagnostic must name the target slug and its state.
   Any target state not enumerated above (e.g. `unknown`, future states
   introduced by later PRDs) is **out of scope for v1** and defaults to
   refuse until the interaction is explicitly modelled.

2. **The feature being confirmed is a superseded target** (i.e. some other
   active-superseder has a `supersedes` edge pointing at it). This case is
   benign: the feature was already excluded from effective replay by
   supersession, and upstream_merged just formalises retirement.
   Proceed without refusal.

Case 1 is the load-bearing safety invariant. Its detection reuses
existing walker `store.CollectBrokenRefs` or the label composer
`workflow.ComposeLabels`. Implementation may pick whichever is
lower-friction; correctness is what matters, and both surface the
same edges.

### 7.5 No secret handling required

The command accepts no secrets. `--upstream-commit` is a sha — safe to
persist and log. `--from-revision` is an entry-id — same.

## 8. Non-goals

1. **No new `reconcile review add` flags.** Operators still record
   revisions with the same field surface. The `--upstream-commit` lives
   on `confirm-upstreamed` because that is where the sha is *validated*
   against `HEAD`.
2. **No `revert-upstreamed` inverse verb.** Reversing a confirmed
   transition is out of scope for v1. A future PRD may add it via
   superseding revisions.
3. **No auto-selection heuristics beyond the D4 filter.** If the operator
   wants a specific revision, they pass `--from-revision <entry-id>`.
4. **No changes to `amend --state`.** Its exit-2 reservation stays.
5. **No changes to `reconcile-evidence.jsonl`.** The evidence artifact is
   read-only from this PRD's perspective; only revision log and status
   are mutated.
6. **No changes to v0.11 gate error wording** on the fast path. Byte
   identity for the pre-existing failure mode.
7. **No two-person review enforcement.** A single operator may still both
   add the revision and run `confirm-upstreamed`. Future workflows may
   split the roles via D3-style verbs.

## 9. Test Scenarios

The following scenarios are the minimum test surface. Each maps to at
least one AC.

### 9.1 Golden: v0.11 fast path unchanged

Fixture: feature with `Reconcile.Outcome=upstreamed`, no review revisions.
Run `confirm-upstreamed <slug>` and `confirm-upstreamed <slug> --json`.
Assert byte-identical stdout, retirement audit findings, and
`status.json` mutation against a golden checked in with this PRD's
implementation. — AC-2.

### 9.2 Reporter reproduction — happy path

Fixture: feature with `Reconcile.Outcome=blocked`,
`Reconcile.ReviewVerdict=rejected-upstreamed`, one review revision with
tuple `(confirmed, confirmed-retired, upstream_merged)`. Synthetic git
repo where the operator-supplied `<sha>` is reachable from `HEAD`.

Run `confirm-upstreamed <slug> --upstream-commit <sha> --json`. Assert:

- Exit 0.
- `status.State == upstream_merged`.
- `status.Reconcile.Outcome == upstreamed`.
- `status.Reconcile.ReviewVerdict == confirmed-upstreamed`.
- `status.Reconcile.UpstreamCommit == <sha>`.
- `reconcile-revisions.jsonl` has three entries: original automated
  rejected-upstreamed, human review, transition (supersedes → human review).
- Retirement audit ran (JSON output contains `retirement_audit` block).
— AC-1, AC-6, AC-7, AC-8, AC-12.

### 9.3 Review path with no authorising revision

Fixture: feature with `Reconcile.Outcome=blocked`, no review revision.
Run `confirm-upstreamed <slug> --upstream-commit <sha>`.
Assert: non-zero exit; error names §4 D5 shape; `status.json` unchanged.
— AC-3, AC-9(b), AC-10.

### 9.4 Review path with incomplete revision tuple

Fixture: feature with revision tuple `(confirmed, deferred,
upstream_merged)` (wrong action) or `(confirmed, confirmed-retired,
applied)` (wrong final state).
Assert: refuse with error naming which tuple field is missing. — AC-3.

### 9.5 Review path missing `--upstream-commit`

Fixture: valid authorising revision present.
Run `confirm-upstreamed <slug>` (no flag).
Assert: refuse with explicit mention of `--upstream-commit`. — AC-4.

### 9.6 Unreachable upstream commit

Fixture: valid authorising revision; `<sha>` is a synthetic commit not in
HEAD's ancestry (`git commit-tree` in a detached scratch worktree).
Assert: refuse with reachability diagnostic; `status.json` unchanged.
Cover both branches: (a) `status.Reconcile.UpstreamRef` non-empty and
sha not reachable from that ref; (b) no upstream ref resolvable, sha not
reachable from HEAD — same refusal shape.
— AC-5, AC-9(c).

### 9.7 `--from-revision <entry-id>` overrides latest selection

Fixture: two review revisions, both matching the tuple; older one is not
superseded.
Run `confirm-upstreamed <slug> --upstream-commit <sha> --from-revision
<older-entry-id>`.
Assert: transition supersedes the older entry, not the newer one.
— AC-6, D4.

### 9.8 Superseder confirming while target is still applied

Fixture: feature A supersedes feature B; A has valid authorising revision
tuple; B is currently `applied`.
Run `confirm-upstreamed A --upstream-commit <sha>`.
Assert: refuse with diagnostic naming B (see §7.4 case 1). — AC-11.

### 9.9 Superseded target being confirmed as upstream_merged

Fixture: feature B is superseded by A; B has valid authorising revision.
Run `confirm-upstreamed B --upstream-commit <sha>`.
Assert: proceeds. (§7.4 case 2 is benign.) — AC-11 (negative case).

### 9.10 Distinct error wording for v0.11 gate

Fixture: feature with `Reconcile.Outcome=blocked` and no revisions.
Assert: error string exactly matches the v0.11 wording at
`internal/cli/cobra.go:2137`. This test locks the fast-path failure to
byte identity. — AC-9(a), AC-2 (partial).

### 9.11 Upstream-ref preferred reachability

Fixture: valid authorising revision; `status.Reconcile.UpstreamRef` is
non-empty and points at a real ref R; `<sha>` is reachable from R (and
also from HEAD, so the test isolates the preferred path). Synthetic
sibling case: `<sha>` is reachable from HEAD but **not** from R (e.g.
operator cherry-picked a commit into a dev branch that never landed on
upstream).
Assert: (a) reachable-from-R case succeeds and does NOT emit the §7.1
residual-risk warning; (b) reachable-from-HEAD-only case refuses with a
diagnostic naming both `<sha>` and R. — AC-5, §7.1 preferred contract.

### 9.12 HEAD-only fall-back emits residual-risk warning

Fixture: valid authorising revision; feature has no
`status.Reconcile.UpstreamRef`; branch has no `@{upstream}` tracking ref
(detached HEAD or unconfigured); `<sha>` is reachable from HEAD.
Assert: the transition succeeds AND stdout/stderr contain the §7.1
residual-risk warning naming the sha. — AC-5, §7.1 fall-back contract.

### 9.13 D4 tie-break — two matching revisions, different evidence attempts

Fixture: two review revisions on the same feature, both non-superseded,
both with the authorising tuple `(confirmed, confirmed-retired,
upstream_merged)`, but with *different* `EvidenceAttemptID` values (so
the `latestRevisionEntries` dedupe key differs and both survive the
filter). No `--from-revision` is passed.
Run `confirm-upstreamed <slug> --upstream-commit <sha>`.
Assert: the transition supersedes the last-in-file-order entry
(`RevisionLog.Entries[-1]`) per §4 D4 tie-break; the older-in-file entry
is untouched and still queryable via `review list --all`. — AC-6, §4 D4.

## 10. Open Questions

- Should the transition revision's `EvidenceAttemptID` copy from the
  consumed revision, or be left blank? Leaving it blank keeps the
  attempt-id anchored to the automated evidence chain, which is more
  conservative; copying it may improve retrieval. **Tentative**: copy
  from consumed revision. Reviewer to confirm.
- Should `--upstream-commit` accept short shas? Existing helpers accept
  full shas; short shas would need `gitutil` resolution first. **Tentative**:
  require full sha in v1; short sha handling is a follow-up.
- Should the human-facing text for the review-path failure include the
  reporter's exact reproduction snippet? **Tentative**: yes — the CLI is
  already the primary discovery surface for this workflow.

## 11. Disputes

None yet.

## 12. References

- GH issue #4 (external report, t3code WSL v0.11.3 reconciliation).
- `internal/cli/cobra.go:2091-2296` — reconcile subcommand implementations.
- `internal/cli/c1.go:255-283` — `amend --state` reservation.
- `internal/workflow/reconcile.go:975-1094` — revision-log producer and
  `updateFeatureState` state-writer path.
- `internal/workflow/retirement_audit.go:28-162` — audit implementation
  and cleanup-revision writer.
- `internal/store/reconcile_revision.go:1-90` — revision schema.
- `internal/store/types.go:299` — supersession edge kind.
- `docs/prds/PRD-upstreamed-confirmation-gate.md` — v0.11 gate contract
  this PRD completes.
- `docs/prds/PRD-reconcile-revision-pass-log.md` — revision log
  primitives.
- `docs/prds/PRD-reconcile-retirement-state-audit.md` — audit primitives.
- `docs/prds/PRD-feature-supersession.md` — v0.12.0 supersession
  semantics.
