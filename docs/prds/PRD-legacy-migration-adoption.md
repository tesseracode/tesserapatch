# PRD - Truthful Legacy Feature Migration and Adoption

**Status**: Draft — PROPOSED, pending operator decisions
**Date**: 2026-11-15
**Issues**: #18 (cumulative stacks), #19 (manual recipe provenance), #20 (legacy patch generations)
**Related**: `SPEC.md`; `CHANGELOG.md`; `docs/ROADMAP.md`;
`docs/handoff/HISTORY.md`; `docs/supervisor/LOG.md`;
`docs/adrs/ADR-013-verify-freshness-overlay.md`;
`docs/adrs/ADR-024-patch-generation-manifest-boundary.md`;
`docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md`;
`docs/adrs/ADR-036-recipe-coverage-authority.md`;
`docs/adrs/ADR-039-coverage-complete-operation-domain.md`;
`docs/adrs/ADR-040-p2-publication-reason-semantics.md`;
`docs/adrs/ADR-041-independent-capture-event-evidence.md`;
`docs/adrs/ADR-042-ordered-recipe-noop-proof.md`;
`docs/prds/PRD-artifact-validation-and-provenance.md`;
`docs/prds/PRD-recipe-generation-authority.md`;
`docs/prds/PRD-verify-freshness.md`;
`docs/prds/PRD-feature-patch-identity-metadata.md`;
`docs/agent-as-provider.md`; `docs/path-b-operator-guide.md`;
`docs/feature-layout.md`.

## 1. Proposal boundary and vocabulary

This is one planning proposal, not authorization to migrate data, relax a
check, amend an ADR, or implement a command. All new artifact names, command
spellings, classifications and exits below are proposed, not shipped APIs.
The operator questions in section 10 must be answered before implementation.
Accepted contracts cited here remain binding until explicitly amended.

Recommendation: assess all three populations together, but separate three
proofs: historical compatibility, present integrity, and historical origin.
Then ship narrowly typed recipe adoption and patch identity adoption in
separate slices. Defer cumulative conversion until an ordered-stack proof
and a per-feature disposition inventory exist. No retrospective timestamp,
generation event, capture selector, dependency snapshot or landing is inferred.

Terms used in the proposed design:

| Term | Meaning; not an authority escalation |
|---|---|
| Derived fact | Recomputed from exact captured bytes or immutable Git objects now. |
| Recorded candidate | Existing metadata names an object; it is not bound proof of the artifact's original capture. |
| Validated compatibility | The exact artifact passes named checks at an explicit reachable commit now. |
| Operator attestation | An explicit human assertion, retained as such; not authenticated machine history. |
| Generated now | A real current generation/derivation event; never inferred from an old sidecar. |
| Adopted legacy | Existing bytes accepted under a new, bounded validation contract; original creation time remains unknown. |
| Current integrity | The accepted verification contract holds at the captured current commit, not merely an old base. |

## 2. Evidence and problem statements

### 2.1 Evidence limits

The three issue bodies and their comments were read through GitHub; each had
no comments at inspection. Current source was inspected around `96d570c`.
The downstream measurements below are **historical evidence**, not a fresh
reproduction by this design task. No downstream checkout, private artifact,
Go command or runtime migration was used for this proposal.

SW-D13 records a read-only 2026-11-11 rebaseline using `6bb6d13`, whose
production code was recorded as identical to `f7beb47`, against disposable
downstream copies. At `tesseracode/copilot-api@e2d7ce4` it reproduced:

| Population | Historical result | Interpretation |
|---|---|---|
| Whole verify inventory | 56 features: 0 passed, 53 failed, 3 skipped | Failing feature count, not the sum of check counts. |
| V8 | 38 failures; own recorded base: 29 compatible, 9 incompatible | Historical compatibility is partial evidence only. |
| V7 | 16: 10 direct replay, 2 parent replay, 2 parent evidence integrity, 2 malformed historical landing evidence | Four distinct remediation classes. |
| V10 | 6 failures, including four Path B missing-provenance cases | The four are missing-anchor failures, not measured hash mismatches. |
| Intent | 1 failure | Independent artifact repair; not a cumulative bypass candidate. |
| Doctor D2 | 24 missing manifests; candidate bases: 6 compatible, 18 incompatible | Nonempty reachable candidates do not prove generation provenance. |

Sources: `docs/handoff/HISTORY.md:19800`,
`docs/supervisor/LOG.md`, entry “Internal Review — copilot-api feedback triage — 2026-08-18”, Final Evidence,
`docs/supervisor/LOG.md`, entry “Verification — copilot-api cumulative-stack feedback — 2026-08-18”, Verdict, and
`docs/ROADMAP.md`, Open GitHub issues table, GH #18–#20 rows. Check counts overlap; 38+16+6+1 must not be
presented as 61 distinct failing features. SW-D13 does not establish a fresh
v0.22.0 result. The original issue expectations still need a later,
authorized read-only reproduction and per-slug inventory.

The ROADMAP's #19 rebaseline adds that later downstream sidecars used the
commit that first added each recipe as `generated_at`; verify accepts these
sidecars and cannot distinguish them from tool-written ones
(`docs/ROADMAP.md`, Open GitHub issues table, GH #19 row). That commit time is evidence of repository presence,
not the time the recipe was generated.

### 2.2 #18: cumulative recording is not a verify mode

Current forward verification uses the shared closure replay with a shadow
rooted at captured HEAD; landed verification uses historical and current
anchors (`internal/workflow/verify_anchored.go`, functions `forwardPhase` and `runAnchoredClosure`;
`SPEC.md:334-367`). V7 replays hard parents and the target recipe. V8 checks
the canonical patch independently after restoring the closure baseline
(`internal/workflow/verify.go:957-964`). Current verify flags expose
`--all`, `--json`, `--quiet`, `--no-write`, not a cumulative-stack mode
(`internal/cli/verify.go:149-153`).

Reconcile's `--cumulative-legacy` instead preserves caller order, skips
DAG reordering and phase-1.5 detection, and can write `incremental.patch`
(`docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md:55-75`,
`internal/workflow/reconcile.go:1435-1464`). Its current derivation falls
back to the full patch if subtraction fails or yields an empty result.
Neither that fallback nor the overlapping-path migration hint is proof of
an ordered stack. Calling reconcile is not a read-only migration assessment.

Thus 29 old-base successes cannot authorize 29 current passes. The nine V8
incompatibilities and V7 parent/evidence failures must remain visible.
Newer landed-anchor selection and single-child explanation contracts also
remain intact; this proposal does not replace them with a legacy-base rule.

### 2.3 #19: the manual producer checkpoints without a recipe anchor

The manual implement gate requires a file and syntactically valid nonempty
JSON, then advances to `implementing`
(`internal/store/manual.go:27-32`, `internal/store/manual.go:99-119`).
The CLI checkpoint observes exact accepted bytes and publishes coverage
after the state transition; it does not write recipe provenance
(`internal/cli/cobra.go:3634-3648`, `internal/cli/cobra.go:3663-3681`).
P6 coverage is deliberately `no-capture` and incomplete
(`SPEC.md:516`, `internal/workflow/implement.go:283-292`).

Provider/heuristic implement has a distinct sidecar write with
`base_commit`, actual `generated_at` and recipe hash
(`internal/workflow/implement.go:244-260`). The schema describes generation,
not retrospective acceptance (`internal/workflow/implement.go:20-36`).
V10's forward baseline resolver checks presence, base shape, reachability,
and optional recipe hash; it does not validate `generated_at`, identify the
writer, or authenticate origin
(`internal/workflow/verify_landed.go:1455-1482`).

There is a separate ownership gap: `DecodeApplyRecipeStrict` requires a
nonempty `feature`, not equality with the requested slug
(`internal/workflow/implement.go:108-122`). Current coverage distinguishes
envelope-owner binding failure from recipe-owner incomplete coverage, so
coverage publication alone is not the missing strict manual gate
(`SPEC.md:650-664`). New adoption must require exact ownership before any
state/provenance write, including when another feature's supersession
metadata could otherwise affect severity.

The four historical recipes' 11 nonempty preimages matched the recorded
bases in the original investigation
(`docs/supervisor/LOG.md`, entry “Verification — copilot-api cumulative-stack feedback — 2026-08-18”, Verdict). This supports assessing adoption, not
silently copying `status.apply.base_commit` into generation provenance.
That exact shortcut was rejected in
`docs/prds/PRD-verify-freshness.md:1823-1836`.

### 2.4 #20: identity facts exist, capture history does not

D2 reports a missing manifest as `Fixable:false` but recommends refresh
(`internal/workflow/doctor_d2.go:12-33`). Refresh first captures the present
working tree; empty capture returns successfully without creating metadata
(`internal/cli/feature_patch.go:77-98`). Its separate nonempty same-generation
checkpoint republishes coverage but creates no generation
(`internal/cli/feature_patch.go:107-140`). Neither branch is historical adoption.

A v1 generation carries patch/recipe identity, base, upper, capture and
dependency snapshots (`internal/store/patch_generations.go:30-76`).
ADR-024 D4 explicitly rejects historical v1 reconstruction, including
partial hash-only backfill, because the missing fields matter to equivalence
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:129-155`).
All 24 historical candidates being reachable therefore does not fill the
capture-mode/pathspec gap or explain the 18 patch incompatibilities
(`docs/supervisor/LOG.md`, entry “Internal Review — copilot-api feedback triage — 2026-08-18”, Final Evidence).

## 3. Could existing primitives do this?

No existing primitive can truthfully encode all three responsibilities.
Reuse their validated readers and proof helpers where their contracts fit;
do not overload their schemas with untyped adoption labels.

| Existing primitive | What it can carry | What it cannot establish |
|---|---|---|
| Legacy `feature.yaml` | Uninterpreted historical material for manual review. There is no supported current field set to extend. | Current metadata or migration authority: D1 explicitly calls it unsupported (`internal/workflow/doctor_d1.go:31-42`). |
| `status.json` identity/lifecycle | `id`, `slug`, `title`, `state`, compatibility, request/update times, last command, notes, `apply`, `reconcile`, `depends_on`, `verify`. | Original recipe/patch generation history. Notes are last-transition hints, not durable per-artifact provenance. |
| `apply.base_commit`, apply times and `has_patch`/`has_recipe` | Candidate baseline and lifecycle summary. | Artifact-hash-bound historical base or creation time (`internal/store/types.go:362-368`). |
| `depends_on` (`slug`, `kind`, optional `satisfied_by`) | Current graph and explicit satisfaction reference. | A complete historical stack order or parent generation snapshot (`internal/store/types.go:340-342`). |
| `VerifyRecord` and report rows | Last verify time, pass bit, exact recipe/patch hashes, parent-state snapshot; transient modes and V10 baselines. | New lifecycle states, adoption receipts or original authorship (`internal/store/types.go:290-313`). |
| Canonical patch, numbered patches, `incremental.patch` | Exact current diff; historical full-diff snapshots; optional derived delta. | Capture selectors or reliable independent-feature attribution from filenames/order alone (`docs/feature-layout.md:15-38`). |
| v1 `patch-generations.json` | Strict append-only generation identity with capture/base/upper/dependency fields. | Partial legacy adoption without a contract change. Unknown fields/version fail strict loading (`internal/store/patch_generations.go:99-119`). |
| `recipe-provenance.json` | Generation-base claim, generation time string, optional exact recipe hash. | Typed manual/adopted provenance or authenticated writer identity (`internal/workflow/implement.go:20-36`). |
| `preimage_hash`, recipe `feature`, operation `created_by` | Expected initial bytes/absence, declared owner, parent-origin gate for relevant operations. | Authorship, capture history, stack integrity or a replacement for observed preimages (`docs/agent-as-provider.md:120-133`). |
| `recipe-stale.json` | Preservation/drift warning. | Acknowledgement, accepted adoption or new replay authority (`SPEC.md:650-664`). |
| C (`recipe-coverage.json`) and E (`recipe-capture-event.json`) | Bound effects, raw hashes, independent event descriptor and reconstructable consistency. | Authenticated history, unknown historical observations or historical repair by copying stored assertions (`SPEC.md:542-570`). |
| Landing trailers | Four-trailer reachable landing evidence, with base/patch/recipe bindings. | An invented source landing or current content merely because history contains a trailer (`SPEC.md:334-358`). |
| Config and `upstream.lock` | Repository settings and upstream tracking; `features_dependencies`, `patch_id_detector_enabled`, `operation_candidate_enabled` select existing behavior. | Per-artifact adoption proof; changing a flag cannot make a missing baseline trustworthy (`internal/store/types.go:469-488`, `SPEC.md:431-434`). |
| Record capture modes and selectors | Actual current all/staged/unstaged/committed-range observations, operator pathspecs and claim IDs. | Recovering unrecorded historical selections. New capture is a new event (`internal/cli/record_capture_modes.go:27-57`, `SPEC.md:511`). |
| `implement --manual`, `prepare --manual` | Recipe checkpoint/state advance; separately, complete intent-bundle adoption. | Recipe provenance through intent adoption, or complete patch authority from P6 (`SPEC.md:114-116`, `SPEC.md:516`). |
| `feature patch refresh` / `fixup` | Actual current capture and generation-policy update, or the contracted P2 checkpoint. | Same-byte legacy adoption without a real capture; a prior generation is required for fixup (`internal/cli/feature_patch.go:77-98`). |
| `record --regenerate-recipe` | A real complete derivation, if feasible, can justify new provenance now under D16. | Byte-preserving adoption of an arbitrary manual recipe; feasibility is not universal (`SPEC.md:610-617`, `SPEC.md:669-675`). |
| Verify V0-V10 plus coverage | Metadata, intent, recipe parsing/targets, dependency validity/reachability/gates, replay, reconcile outcome, preimages, coverage. | A cumulative representation merely by suppressing V7/V8/V10. The check vocabulary and verdict aggregation are explicit (`internal/workflow/verify.go:47-70`, `internal/workflow/verify.go:669-681`). |
| `verify --all --json --no-write`; doctor | Existing integrity baseline and diagnostic inventory without a persisted verify record; D2 cannot fix the gap. | A new migration classifier or writes authorized by its report (`internal/cli/verify.go:149-160`, `internal/workflow/doctor_d2.go:23-33`). |
| Reconcile legacy mode / operation candidates | Explicit legacy reconciliation; separately, opt-in independently proved cross-base candidates. | Reusing reconcile's write/fallback semantics as an assessment or granting replay from adopted provenance (`SPEC.md:768-783`). |

The notes limitation is documented in `docs/agent-as-provider.md:55-70`.
Current status fields are defined at `internal/store/types.go:206-251`.
There is deliberately no proposed addition to `feature.yaml`, recipe
operation grammar, landing trailers or repository-wide bypass config.

## 4. Invariants and legacy classifications

### 4.1 Constraints preserved by every option

1. Never create an old generation event, `generated_at`, capture mode,
   pathspec list, upper bound, dependency snapshot or trailer from a guess.
2. Never substitute live working-tree bytes for a required V10 baseline.
   Existing landed member/target baselines and supersession rules remain
   those of ADR-013 D15 (`docs/adrs/ADR-013-verify-freshness-overlay.md:788-858`).
3. Keep default verify strict. Missing legacy coverage may already warn, but
   this does not neutralize independent V7/V8/V10 failures (`SPEC.md:650-664`).
4. Preserve ADR-036 D6/D16: only a complete fresh total canonical-byte
   derivation proves record-generated origin now. A semantic near-match,
   hash label, old sidecar or attestation is insufficient
   (`docs/adrs/ADR-036-recipe-coverage-authority.md:1509-1550`,
   `docs/adrs/ADR-036-recipe-coverage-authority.md:2527-2586`).
5. Preserve D8: coverage is not cross-base safety; do not persist new replay
   anchors in recipes (`docs/adrs/ADR-036-recipe-coverage-authority.md:1628-1672`).
6. Preserve ADR-039's complete-operation domain, ADR-040's separate semantic
   explanation/origin predicates, ADR-041's independently produced E then C,
   and ADR-042's initial authorization and ordered no-write witnesses
   (`docs/adrs/ADR-039-coverage-complete-operation-domain.md:27-58`,
   `docs/adrs/ADR-040-p2-publication-reason-semantics.md:25-56`,
   `docs/adrs/ADR-041-independent-capture-event-evidence.md:409-432`,
   `docs/adrs/ADR-042-ordered-recipe-noop-proof.md:26-45`).
7. Malformed existing manifests or bindings are errors to expose, never
   absence to overwrite. Adoption cannot erase a contradictory artifact.

### 4.2 Proposed classification and verify presentation

These are orthogonal evidence dimensions, not mutually exclusive feature states.

| Legacy state | What can be said truthfully | Proposed assessment / verify treatment |
|---|---|---|
| Real tool generation observed now, including D16 reproduction | This run generated or exactly derived these bytes against this base. | Preserve generated provenance and current checks; do not assert independently authenticated historical authorship. |
| Existing untyped sidecar, writer unknown | The sidecar claims a base/time/hash. Shape cannot identify its author. | `origin-unverified` annotation; retain current resolver/check behavior, including weak hash binding. No retrospective global failure solely because origin is unknown. |
| Known hand-written sidecar using first-add commit time | Operator-supplied historical claim, not proven generation time. | Explicit `operator-attested-legacy` source classification on opt-in review; preserve original bytes/hash as evidence and mark historical generation time unknown. Do not silently relabel it tool-generated. |
| No provenance, real preimage gates | No usable forward V10 anchor under today's contract. | Keep `recipe-provenance-unavailable` until accepted validated adoption or genuine generation supplies the appropriate typed anchor. |
| No gates / explicit-empty creation gates | Omitted gates are existing legacy behavior; empty gates assert initial absence. | Keep the distinction; adoption must not remove gates to obtain a pass. Empty gates are validated against absence at the candidate base. |
| Cumulative stack, old-base compatible | Historical compatibility only; individual or ordered current integrity may still fail. | Separate `cumulative-recoverable` assessment only after full proof; existing strict verify failures remain until conversion. |
| Candidate base recorded, not checked | Metadata names a possible base. | `candidate-unvalidated`, never a passed integrity check. |
| Candidate validated / inconsistent / unavailable | Named probe passed / contradicted candidate / could not establish facts. | Separate machine states with actual blockers. Inconsistent is not unknown; unavailable is not incompatible. |
| Validated patch adoption with unknown capture history | Patch identity and compatibility established now; original capture fields unknown. | New bounded adoption annotation; no v1 generation or automatic V8 pass. |

All labels above are proposed assessment vocabulary, not additions to the
existing lifecycle enum. A tool cannot automatically distinguish a forged
consistent local record from an honestly written one. E itself is unkeyed
consistency evidence, not authentication
(`docs/adrs/ADR-041-independent-capture-event-evidence.md:64-80`).

## 5. Options for #18: cumulative stacks

| Option | Benefits | Trade-offs, truthfulness and compatibility |
|---|---|---|
| A. Documentation/operator procedure only | No verifier changes. Capture current reports and review ordered patches in disposable checkouts; plan explicit new records. | Lowest code risk, high manual error cost; cannot close the issue's machine-readable assessment requirement. Must leave all strict failures visible. |
| B. Read-only structured assessment, then explicit conversion | Classifies all evidence before any rewrite; supports reviewed migration to independently recorded features. | Adds a report contract and later conversion work. Does not immediately make legacy `verify --all` green. Recommended. |
| C. New opt-in native cumulative verification contract | Potentially preserves exact canonical legacy bytes while verifying a whole stack. | Larger ADR-013 amendment, order/ownership/freshness model and consumer changes. An own-base flag alone is unsound; do not implement it as a shortcut. |

### 5.1 Recommended proof and report boundary

Start with B, assessment only. A provisional surface is
`tpatch feature migration assess --all --json`; it is not an available
command. Default text and JSON must say "assessment, not verification".
Do not persist `VerifyRecord`, change labels, run providers, fetch objects,
write feature files, or invoke mutating reconcile to obtain the report.
Use temporary indexes and isolated scratch outside tracked metadata.

Capture one immutable current commit, full feature inventory and exact input
digests. Require an explicit operator-proposed order, not RequestedAt
sorting or patch filename inference. Check uniqueness, hard-parent closure,
dependency ordering, effective supersession and completeness of that order.
Report missing order as a blocker, not as permission to guess.

For a candidate cumulative group:

1. Identify a candidate common baseline B from named durable evidence. Validate
   commit syntax/object type, reachability and availability without fetching.
   Differing per-feature bases require an independently proved mapping to B;
   never treat every reachable base as interchangeable.
2. Materialize each exact cumulative patch P_i at B. Record exact resulting
   trees T_i, including paths, modes, additions and deletions. Failure is a
   named incompatible/unavailable/unsupported result, not a full-patch fallback.
3. Derive proposed independent deltas D_i from T_(i-1) to T_i, with T_0=B.
   Replay them in the declared order and require exact prefix and final-tree
   equality. A successful empty delta is explicitly empty, not the original
   cumulative patch. Confirm feature-intent ownership by operator review:
   tree subtraction cannot decide whether removal of an earlier effect was intended.
4. Compare the effective ordered result to the captured current tree over the
   entire affected path/mode union. Separately identify unrelated current
   changes; unexplained differences block current-integrity certification.
   A future broader verifier would need an accepted explanation proof, not
   a general later-touch exemption.
5. Assess recipes independently at their authorized member baselines. Do not
   replace recipe failures with patch successes or inherit the last member's
   provenance. Validate any proposed replacement recipe against its exact
   independent delta and preimage; withhold unsupported derivations.
6. Preserve existing static, landing-evidence, parent arbitration and V10
   outcomes. Explicitly report which migration blockers remain even if the
   ordered patch result equals current content.

Classify `independently-replayable`, `cumulative-recoverable`,
`incoherent`, or `unassessable`; include separate historical compatibility
and current-integrity results. A recoverable classification requires the
complete ordered proof, not just an own-base patch check.

### 5.2 Required historical population dispositions

| Historical population | Required disposition before implementation acceptance |
|---|---|
| 29 V8 own-base successes | All receive ordered/current proof; only proven groups become conversion candidates. None automatically pass verify. |
| 9 V8 own-base failures | Determine wrong/unbound base, artifact inconsistency, unsupported shape or unavailable evidence per slug; repair or explicit non-support. No guessed-base search promoted to authority. |
| 10 direct V7 failures | Evaluate recipe at its lawful anchor; distinguish cumulative duplication from invalid/stale operations. Adopt only valid bindings; otherwise explicit recipe repair/new generation. |
| 2 parent replay failures | Diagnose the named parent and closure order; child remains blocked until its actual cause is repaired. |
| 2 parent evidence integrity failures | Repair/re-land truthful parent evidence or document non-support; never downgrade to cumulative drift. |
| 2 malformed historical landing failures | Preserve failure; require truthful evidence repair/re-landing, not fallback to forward mode. |
| 6 V10 / 1 intent failures | Track separately, including four #19 candidates and two historical malformed-evidence cases; restore missing intent independently. |

No per-slug assignment is asserted here. The historical aggregates are not
the all-38/all-16 explanation that #18 requires for implementation readiness.
An authorized inventory must include every member, cause, evidence and
disposition, preserve overlapping check memberships, and explain any changed
counts on newer CLI versions.

Conversion must be a later reviewed operation, not a side effect of assessment.
Its plan must bind original byte hashes, candidate order/bases, proposed deltas,
new recipe proofs, dependency effects, old/new tree identities and recovery
artifacts. New captures describe migration-time events; old snapshots remain
audit material. Existing landing attestations become historical rather than
being rewritten to match changed bytes. Fresh strict verify must succeed
before declaring a converted feature verified.

## 6. Options for #19: manual and historical recipe adoption

| Option | Benefits | Trade-offs, truthfulness and compatibility |
|---|---|---|
| A. Documentation only; keep failures or genuinely regenerate | No new schema. A feasible D16 derivation can truthfully publish provenance now. | Arbitrary manual bytes cannot be adopted unchanged; regenerate may alter intent. Never prescribe hand-written `generated_at` as the workaround. |
| B. Version existing provenance with generated/manual/adopted variants | One anchor reader and one artifact location. | Must change every reader/version fallback and avoid old decoders misreading adopted data as generation. Larger compatibility boundary. |
| C. Separate typed recipe-adoption artifact | Keeps old generation claims intact and gives adoption its own validation semantics. | Additional reader/conflict/publication handling. Recommended, with an explicit ADR-013 amendment before V10 consumes it. |

### 6.1 Recommended typed acceptance

Propose `artifacts/recipe-adoption.json`, versioned independently. Names are
provisional. Bind requested slug, exact raw recipe SHA-256, full validated
baseline commit, evidence source and validation version. Retain the source
candidate's metadata-field identity/digest and any prior sidecar digest.
Represent `origin` as manual-accepted or adopted-legacy; represent original
generation time as explicitly unknown, not empty/fake `generated_at`.
Do not add a wall clock to the deterministic acceptance payload. A later Git
commit can record publication; it is not a substitute for unknown authorship.

Newly authored manual input is still accepted, not generated by the CLI.
For a new checkpoint, a captured HEAD may be the proposed baseline only if
every gated preimage/absence assertion validates there. For historical
adoption, use an explicitly selected recorded candidate with its source
binding and operator confirmation; full reachability alone cannot distinguish
a wrong-but-content-equivalent base. The receipt certifies validation there,
not that the recipe was originally authored there.

Owner equality is mandatory before provenance lookup, severity calculation or
state writes. Validate all operation schemas, gate grammar, safe paths and
each gated preimage against immutable Git data. Also assess supported recipe
execution in isolation; a V10 anchor does not imply patch coverage, successful
V7, safe cross-base replay, or complete C. Never use live files to fill gaps.

For recipes mixing gated and ungated operations, identify exact proof scope;
do not imply the ungated operations acquired preimage authority. Recipes
without gates keep their existing execution/verify behavior, but a newly
invoked manual checkpoint must still satisfy the proposed strict schema and
owner gate. Adoption itself never changes lifecycle, recipe or patch bytes.

If an old sidecar exists, retain it verbatim. Validate and bind it as source
evidence; do not validate its inferred generation timestamp as historical fact.
An explicitly reviewed attestation can mark that claim unproved. Conflicting
base/hash claims, malformed data or two competing active adoption bindings
must refuse; do not choose whichever produces a pass. Matching old/new anchors
can coexist with the typed receipt explaining the narrower adoption claim.

After an accepted ADR amendment, V10 may consume a valid adoption baseline for
an otherwise provenance-unavailable forward member. It must name
`adopted-legacy` or `manual-accepted` mode and exact member baseline/hash
binding, never `generated`. Landed members keep their existing anchor selection.
All preimage, static, V7/V8 and coverage checks still run with unchanged severity
rules. This adds validated evidence; it does not waive a check.

### 6.2 Refusal and publication matrix

The proposed adoption/checkpoint refusals use named exit 2 diagnostics;
underlying I/O or recovery failure is nonzero and includes the actual cause.
Diagnostic spellings and exact wrapping require the implementing ADR.

| Ordered condition | Required result before successful publication |
|---|---|
| Pending interrupted adoption | Refuse unrelated writes; name explicit inspect/resume/rollback recovery. |
| Unreadable/empty/invalid JSON or strict recipe decoding failure | Refuse; preserve prior state and artifacts. |
| `recipe.feature != requested slug` | Refuse before provenance/state mutation or supersession-based severity handling. |
| Unsafe path, malformed gate or invalid operation | Refuse; never "adopt" a safety failure. |
| Malformed existing provenance/adoption/E/C, owner/hash contradiction | Refuse; preserve evidence and identify repair requirement. |
| Missing, ambiguous, wrong-type or unreachable baseline | Refuse, distinguish unavailable ancestry from a negative ancestry answer. |
| Explicit candidate disagrees with bound source | Refuse as changed/contradictory candidate; no automatic alternate-base search. |
| Any gated preimage disagrees, including creation target already present | Refuse; report op and expected/observed identity, never source bodies. |
| Unsupported proof requirement or failed isolated recipe validation | Refuse validated adoption; leave diagnostic assessment available. |
| Concurrent recipe, status, sidecar, config, graph or HEAD change | Refuse stale plan after recheck under lock; re-assessment required. |
| Receipt/state/E/C write or rename failure | Nonzero; no success message. Restore prior bytes or expose pending recovery that readers cannot mistake for committed adoption. |
| Exact same receipt and unchanged inputs | Idempotent no-write success; do not invent a later historical time. |

The issue requires a stronger guarantee than today's per-file publication.
Use a bounded staged transaction/journal under repository-standard locks for
the manual state transition, typed receipt and owed E/C publication; snapshot
existence as well as bytes. Prevalidate outputs, publish E before C, and make
the committed boundary unambiguous. Synchronous failure restores old state
and bindings; crash recovery is explicit and refuses use of half-publication.
Do not claim multiple renames are inherently atomic.

This is a deliberate new mutation-boundary decision, not an assertion that
the current producer is transactional: `SPEC.md:536-554` explicitly says it
is not. P6 coverage remains a genuine incomplete no-capture event; adoption
must not synthesize a complete historical E/C pair. A historical receipt-only
adoption owes no fabricated P6 event; any real later checkpoint republishes
its own truthful E/C.

The transaction contract must identify every reader/writer that refuses a
pending journal, especially verify, apply and manual re-entry. Older CLIs
cannot enforce that new barrier and must not operate during adoption/recovery;
the operator runbook must state this minimum-version boundary explicitly.

## 7. Options for #20: legacy patch identity adoption

| Option | Benefits | Trade-offs, truthfulness and compatibility |
|---|---|---|
| A. Documentation/procedure only | Diagnose refresh's limitation; preserve patches, make a reviewed new capture only when intended. | Cannot provide same-byte adoption or remove the remediation dead end; D2 continues to report unknown history. |
| B. New generation manifest version with tagged legacy entries | Unified history readers can model unknowns explicitly. | Broad version/identity/dependency migration; cannot reuse v1 D2 equality or fabricate generation 1. Requires explicit supersession of ADR-024's boundary. |
| C. Separate legacy patch-adoption record | Preserves v1 schema/IDs; same-byte opt-in identity validation without inventing capture. | Readers must retain a hard boundary between adopted identity and generation history. Recommended, subject to an ADR-024 boundary amendment. |

Propose `artifacts/patch-adoption.json` with its own version and identity
namespace, never `pg_`. Record slug, canonical raw patch SHA-256, stable
patch-id plus algorithm when defined, strictly parsed affected paths/modes,
and exact evidence bindings. Record candidate source, resolved commit and
validation outcome separately. Do not infer recipe-at-capture hash from the
current recipe; a current recipe hash, if included, is labelled current only.

The field-level report must use `derived`, `candidate`, `validated`,
`inconsistent`, `unknown` and `blocking` explicitly. Historical capture mode,
pathspecs, claims, upper bound and parent snapshots stay typed unknown unless
independently established durable evidence exists. An empty pathspec list
means a known empty selection, not unknown; the schema must distinguish them.
Patch paths do not reconstruct the operator's capture selectors.

Validate candidate syntax, commit object type, reachability and full patch
compatibility through an isolated index. Apply success proves compatibility
there now, not original generation, feature independence or current materialization.
Unsupported patch parsing/identity must be explicit, not partial path success.
One proposed acceptance record is a deterministic assessment-now receipt,
not reconstructed historical generations or a history scan of numbered patches.

Recommend persistent validated adoption only for compatible, fully checked
candidates. Report inconsistent/unavailable candidates read-only with blockers;
do not write an authoritative-looking partial acceptance. Thus the historical
six are candidates, not pre-approved adoptions; the eighteen receive actionable
repair/re-record/non-support dispositions rather than copied bases. Any later
different result must explain changed bytes, baseline or probe semantics.

Same-byte adoption writes only its new receipt, using staged atomic publication,
lock-held input rechecks and exact-input idempotence; it never mutates the patch
to trigger refresh. Do not overwrite any existing v1 manifest. Malformed
manifests block adoption and stay visible; valid manifests retain all IDs and
their established meaning.

### 7.1 Consumer boundaries that must ship together

| Surface | Proposed behavior |
|---|---|
| Verify | Annotate validated legacy identity and unknown capture; continue exact-byte V7/V8/V10 and existing coverage checks. A receipt alone changes no pass/fail. |
| Doctor D2 | Missing manifest/no valid receipt: explain unknown history and assessment route, not unconditional refresh. Valid receipt: distinct non-fixable legacy-adopted advisory, not "native generation present". Inconsistent candidate: name its blocker. |
| `doctor --fix` | No automatic adoption, candidate selection, patch recapture or manifest repair. |
| Refresh/fixup/amend writers | Respect normal state/capture policy; adoption is not a prior generation for fixup. A genuine future capture may start native history under its real event, without historical placeholders. |
| Dependencies / stale-parent readers | Adoption identity is not `parent_generation`; never synthesize a parent version. Continue existing unknown-history behavior unless a separate amendment explicitly defines an adoption binding. |
| Generation and reconcile readers | Do not convert receipts into v1 entries or identity authority. Recompute relevant bytes; preserve malformed-manifest handling. |
| Coverage and operation candidates | No coverage/event/replay eligibility inferred from adoption. Current E/C reconstruction and candidate gates remain required. |

ADR-024's historical prohibition stays in force even if this receipt is
accepted. The needed amendment must explicitly authorize the distinct
assessment/adoption namespace and enumerate its non-generation consumers;
it must not quietly emit reserved import kinds or weaken v1 equality.

## 8. Shared assessment, migration and compatibility contract

Use one read-only inventory/report engine with separate recipe, patch and stack
results, not one blended "legacy passes" flag. Each field includes its evidence
source and proof status. Include schema version, captured current commit, CLI
version, inventory digest, exact artifact digests/presence, candidate bases,
probe outcomes, original verify check results, blockers, proposed writes
(including explicit none), and per-issue disposition.

Sort sets deterministically but preserve explicit stack/operation order.
Exclude machine identity, absolute locations, source bodies and secrets.
No timestamp is needed in deterministic assessment identity. Re-read bound
inputs before finalizing a report; if they changed, report stale/aborted,
not a completed authoritative assessment.

Proposed assessment exits: 0 when a complete assessment was produced with no
blocking migration conditions; 2 when completed with blockers or incomplete
evidence; 1 for operational failure. None is a verify pass. Human and JSON
output must retain that distinction, including unknown and unsupported states.
An authorized implementation must settle limits, platform support, offline
object rules and error ordering before exposing this command.

Acceptance of an old sidecar never authenticates its writer. Old consumers
will ignore separate adoption artifacts and may still fail V10; document the
minimum adoption-aware CLI, with no fake legacy sidecar for compatibility.
Existing generated sidecars and v1 manifests remain byte-identical.
Adoption readers must reject unknown versions/fields, duplicate/conflicting
bindings and stale raw hashes, not silently choose another evidence source.

Changing a recipe/patch after adoption invalidates the relevant receipt by
hash. Changing HEAD requires current verification again; historical receipt
validity does not become an everlasting freshness pass. A receipt cannot make
`VerifyRecord.Passed` true, change a lifecycle state or satisfy dependencies.

## 9. Recommended sequencing, ADR gates and validation implications

### 9.1 Smallest sliceable first step

First approve the assessment vocabulary and assemble the read-only per-slug
evidence inventory, including #18's complete 38/16 matrix, #19's four original
and later attested states, and #20's complete 24-candidate matrix. If runtime
work is authorized later, the first code slice is assessment-only: no new
verify baseline, adoption writes, state changes or conversion. Existing verify
output/exits remain unchanged. A procedure-only inventory can precede even
that slice if the operator chooses no new command.

Then sequence:

1. Recipe contract/ADR, including strict manual input matrix and transaction
   design; implement newly accepted manual recipes and explicit historical
   adoption only after approval. Keep lifecycle-neutral adoption separate.
2. Patch receipt contract/ADR and all listed reader/remediation surfaces.
   Preserve old manifests and patch bytes; no v1 backfill.
3. Re-evaluate the cumulative inventory after those bounded gaps are resolved.
   Propose conversion plans, or a separately accepted native ordered-stack
   verifier, only from actual per-feature results.

Likely ADR work: amend ADR-013 D15 for typed adoption baselines and explicit
report modes without altering landed arbitration or current-anchor proofs;
new manual/adoption publication ADR cross-referencing ADR-036/041 and the
artifact-validation deferral; amend ADR-024 for the separate patch receipt
boundary. Native cumulative verify or sharing verify-only closure helpers
would additionally require an ADR-013/030 decision. Do not allocate ADR
numbers or write these decisions as accepted in this planning document.
Any new verify mode values or report fields also need an explicit report-schema
compatibility decision; assessment's separate schema does not authorize
silently changing the current verify schema 1.3 (`SPEC.md:362-365`).

Manual tightening is a deliberate compatibility change:
`docs/prds/PRD-artifact-validation-and-provenance.md:3020-3041` freezes the
existing gates, and `:3078-3095` requires a new command/input/exit/migration,
symlink and announcement contract. Recommend tightening implement only,
not analyze/define/explore; keep their readiness/lifecycle semantics unchanged.
The implementation ADR must explicitly settle rooted/no-follow handling,
rather than accidentally changing symlink behavior during validator reuse.

### 9.2 Required future evidence and tests

| Area | Acceptance evidence required after authorization |
|---|---|
| Assessment purity | Exact repository/worktree/index/metadata digests unchanged; no provider/network; stable reports, immutable HEAD, concurrent-change abort. |
| Historical population | Reproduce or explain every 29/9, 10/2/2/2, four-recipe and 6/18 member; preserve 56-feature coverage and overlapping failures. |
| False greens | Old-base success/current drift; wrong compatible base; omitted/reordered/duplicate stack members; empty delta; independent sibling subtraction; unsupported modes/deletions; parent evidence failure. |
| Recipe validation | Exact owner mismatch, including supersession severity trap; malformed JSON/strict fields/gates; nil versus empty gates; every preimage; no live-tree fallback. |
| Source classification | Generated-now, untyped legacy, known hand-written first-add-time claim, absent, malformed, weak hash binding and conflicting sidecars. No origin inference from timestamps. |
| Publication | Failure at each receipt/state/E/C write/rename and crash boundary; rollback or visible pending recovery; lock ownership, concurrent edits, idempotent no-op rerun. |
| Patch adoption | Same-byte six-compatible candidates; eighteen incompatible candidates refused; typed unknown capture; stable identity, malformed v1 refusal, no `pg_` aliases or fabricated dependency snapshots. |
| Consumer compatibility | Verify checks/exits and freshness overlay unchanged by assessment/patch identity; adopted V10 only after all new proofs; doctor/amend/dependency/reconcile readers honor explicit boundaries. |
| Coverage/replay | D6/D8/D16, ADR-039 domain, ADR-040 reason distinctions, independent E/C binding, ADR-042 ordering, default-off GH #13 and no cross-base authority from adoption. |

Skill/manual remediation changes will require coordinated updates in all six
shipped formats and the `assets/` parity guards; required commands and recipe
schema anchors are enforced in `assets/assets_test.go:16-66` and
`assets/assets_test.go:68-100`. This draft changes neither assets nor guards.

Golden/freeze impact is real: `internal/cli/prepare_pib_golden_test.go:47-58`
tracks manual-phase and compatibility rows, `:325-329` compares D1-D8,
and `:668-700` captures manual implement. Do not mass-rerecord the baseline.
The assessment-only slice should preserve it. Any later deliberate manual,
verify or D2 diagnostic delta needs operator-approved narrow expected-delta
handling and negative controls proving unaffected rows remain frozen.
Also retain the recipe-authority producer/coverage compatibility guards;
their P6 absence-of-provenance baseline must change only with the approved
new contract (`internal/workflow/recipe_authority_s0_producers_test.go:752-772`).

Future operator documentation must describe actual shipped GH #13 separately
from coverage/adoption. SPEC and CHANGELOG already place the opt-in consumer
in v0.18.0 (`SPEC.md:766-780`, `CHANGELOG.md:241-250`).
Stale "future separate-release work" summaries in guides are not authority.
The current coverage and manual descriptions were checked in
`docs/agent-as-provider.md:171-177` and
`docs/path-b-operator-guide.md:159-173`; guide correction is separate from
authorizing this proposal. No runtime tests are claimed for this design.

## 10. Operator Questions

Each answer is a decision, not consent implied by reading this draft.

1. **First deliverable:** procedure-only evidence inventory, or a new read-only
   assessment command? Recommendation: approve inventory first, then a small
   structured assessment slice; authorize no migration writer yet.
2. **Attested legacy:** accept a machine-distinct operator-attested legacy
   classification, or support only genuine new generation? Recommendation:
   allow attested legacy, but only independently validated compatibility can
   authorize an adoption baseline; attestation alone never passes V10.
3. **Verify policy:** distinct non-failing legacy integrity state, or keep
   strict failures until valid adoption/conversion satisfies the checks?
   Recommendation: keep strict failures; use separate explanatory assessment
   states, not a new green substitute for incomplete integrity.
4. **Recipe representation:** version `recipe-provenance.json`, or use a
   separate typed receipt with an ADR-013 baseline amendment? Recommendation:
   separate receipt; retain old sidecars verbatim, explicitly disclaim inferred
   generation times, and never identify unknown writers as tool-generated.
5. **Manual tightening:** implement-only strict schema/owner/base validation,
   opt-in new strict mode, or preserve the loose gate indefinitely?
   Recommendation: implement-only tightening after the required compatibility
   amendment/announcement, with no unrelated Markdown gate change; authorize
   an explicit rooted/no-follow path policy in that amendment.
6. **Patch boundary:** preserve ADR-024 with procedure only, amend it for a
   separate receipt, or supersede it with a new manifest version?
   Recommendation: explicitly amend the boundary for a separate receipt,
   leaving historical v1 backfill prohibited and generation IDs untouched.
7. **Adoption eligibility:** persist facts-only receipts for inconsistent or
   unavailable candidates, or accept only fully validated compatible ones?
   Recommendation: validated acceptance only; retain other facts/blockers in
   read-only reports, with no authority-shaped partial acceptance.
8. **Publication guarantee:** authorize a bounded recovery journal for manual
   state/receipt/E/C publication, or retain per-file semantics and defer manual
   adoption? Recommendation: authorize the stronger bounded transaction design
   before #19 implementation; do not promise atomicity without recovery.
9. **Cumulative scope/order:** convert to independent features first, or fund
   native cumulative verify now; who supplies and reviews order/intent?
   Recommendation: operator-supplied order and conversion-first assessment;
   defer native verify and all canonical rewrites pending per-slug proof.
10. **Downstream handling and sequence:** authorize disposable, read-only
    reproduction at the issue pin and a separately pinned newer commit, or
    plan from historical evidence only? Recommendation: authorize both read-only
    inventories, sequence #19 then #20 then #18 conversion research, and require
    separate explicit permission before any downstream artifact write.

## 11. Non-goals, risks and unresolved evidence

Non-goals: implementing this design; changing current tracking; fixing other
issues; weakening verify; broad recipe operation-domain expansion; automatic
historical backfill; patch/recipe rewriting to manufacture a generation;
forging trailers or timestamps; rewriting Git history; retroactive authorship
authentication; provider-assisted guessing of missing evidence; a new
repository-wide legacy bypass; downstream changes without explicit approval.

Risks: local consistent forgery remains outside an unkeyed evidence model;
a wrong base can coincidentally have matching preimages; cumulative subtraction
can erase independent intent; migration can invalidate historical hash bindings;
new receipts expand reader/error and transaction recovery surfaces; old CLIs
cannot understand typed adoption; unsupported history/platforms may leave
features unassessable. Preserve uncertainty and explicit non-support rather
than selling a complete migration where only identity was proved.

Not verified in this task: fresh downstream counts on current code; per-slug
causes/dispositions; original command invocations, selectors, generation times
or writer identity; exact downstream sidecar bytes; feasibility/resource cost
of each ordered-stack proof; achievable conversion results. Historical counts
and the first-add-time sidecar report were verified against repository records,
not independently rerun. These are prerequisites for runtime acceptance, not
facts the proposal may fill in.

## Appendix A. Claims audit of current behavior

Every load-bearing current-behavior claim above is grouped below with its
authoritative source. Proposed behavior is intentionally not claimed as current.
Line ranges refer to inspected source; embedded historical citations inside
older PRDs are not substituted for current implementation anchors. For the
anchored verifier, function names replace line numbers to respect its
ledger-governed citation contract. `docs/supervisor/LOG.md` is prepended
newest-first and `docs/ROADMAP.md` tables move as tracking changes, so those
sources are cited by entry heading or table row rather than line number.

| ID | Current-behavior claim | Verified source anchor |
|---|---|---|
| C01 | SW-D13 reproduced the 53-failure, 38/16/6/1, 29/9, four-recipe and 24/6/18 historical populations using the named candidate build. | `docs/handoff/HISTORY.md:19800` |
| C02 | Original inventory was 0/53/3; four V10 cases lack provenance, two fail landing evidence; all 11 nonempty preimages matched. | `docs/supervisor/LOG.md`, entry “Verification — copilot-api cumulative-stack feedback — 2026-08-18”, Verdict |
| C03 | Historical V7 classes are 10/2/2/2; downstream later hand-wrote first-add-time sidecars accepted by verify; refresh gap persists; all 24 D2 candidate bases were reachable. | `docs/ROADMAP.md`, Open GitHub issues table, GH #18–#20 rows; `docs/supervisor/LOG.md`, entry “Internal Review — copilot-api feedback triage — 2026-08-18”, Final Evidence |
| C04 | Verify forward mode uses HEAD-rooted closure; landed mode has historical/current anchors and captured-current reads. | `internal/workflow/verify_anchored.go`, functions `forwardPhase` and `runAnchoredClosure`; `SPEC.md:334-367` |
| C05 | V7 hard-parent/target replay and independent restored-baseline V8 are distinct checks. | `internal/workflow/verify.go:957-964` |
| C06 | Verify flags lack cumulative mode; no-write suppresses persisted record; all aggregation fails if any feature fails/errors. | `internal/cli/verify.go:149-160` |
| C07 | Legacy reconcile preserves caller order and skips phase 1.5. | `docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md:65-75`; `internal/workflow/reconcile.go:191-195`; `internal/workflow/reconcile.go:251-264` |
| C08 | Legacy derivation writes incremental artifacts and falls back to full patch on error/empty delta. | `internal/workflow/reconcile.go:1435-1464` |
| C09 | Manual implement accepts syntactically valid nonempty JSON and marks implementing. | `internal/store/manual.go:27-32`; `internal/store/manual.go:99-119` |
| C10 | Manual CLI observes exact bytes and publishes coverage after state advance, without writing provenance in that path. | `internal/cli/cobra.go:3634-3648`; `internal/cli/cobra.go:3663-3681` |
| C11 | P6 observes no captured tree and publishes incomplete no-capture coverage. | `SPEC.md:516`; `internal/workflow/implement.go:283-292` |
| C12 | Recipe provenance describes generated base/time plus optional hash; provider/heuristic implement writes actual time/base/hash. | `internal/workflow/implement.go:20-36`; `internal/workflow/implement.go:244-260` |
| C13 | Forward provenance resolver checks base shape/reachability and optional recipe hash, not timestamp or writer identity. | `internal/workflow/verify_landed.go:1455-1482` |
| C14 | Strict recipe decode checks nonempty owner, not requested-slug equality. | `internal/workflow/implement.go:108-122` |
| C15 | Recipe-owner mismatch and coverage-envelope mismatch have distinct coverage severities. | `SPEC.md:650-664` |
| C16 | Existing freshness PRD rejects automatic status-base provenance backfill. | `docs/prds/PRD-verify-freshness.md:1823-1836` |
| C17 | D2 missing-manifest finding is non-fixable and recommends refresh. | `internal/workflow/doctor_d2.go:12-33` |
| C18 | Refresh loads manifest, requires a prior generation for fixup, captures current tree and skips empty capture. | `internal/cli/feature_patch.go:77-98` |
| C19 | Nonempty same-generation refresh can republish coverage without appending a generation. | `internal/cli/feature_patch.go:107-140`; `SPEC.md:512` |
| C20 | v1 generation schema carries base/upper/capture/dependency identity; strict decoder/version validation rejects incompatible data. | `internal/store/patch_generations.go:30-76`; `internal/store/patch_generations.go:99-119` |
| C21 | ADR-024 forbids historical v1 backfill, including partial hash-only history reconstruction. | `docs/adrs/ADR-024-patch-generation-manifest-boundary.md:129-155` |
| C22 | Manifest identity includes capture/base/upper inputs; no wall-clock timestamp is allowed. | `docs/adrs/ADR-024-patch-generation-manifest-boundary.md:77-85`; `docs/adrs/ADR-024-patch-generation-manifest-boundary.md:107-127`; `internal/store/patch_generations.go:264-275` |
| C23 | Legacy feature.yaml has no supported current schema; status.json is current metadata. | `internal/workflow/doctor_d1.go:31-42` |
| C24 | Current lifecycle/identity/graph/verify fields live in FeatureStatus; apply base is summary metadata. | `internal/store/types.go:206-251`; `internal/store/types.go:340-342`; `internal/store/types.go:362-368` |
| C25 | Manual notes are overwritten last-transition hints, not durable artifact provenance. | `docs/agent-as-provider.md:55-70` |
| C26 | VerifyRecord persists hashes/pass/time/parent states; extra baseline/check details are report-only. | `internal/store/types.go:290-313` |
| C27 | Canonical patch, incremental delta and historical full-diff snapshots have different roles. | `docs/feature-layout.md:15-38`; `SPEC.md:490-498` |
| C28 | Recipe created_by and preimage fields have parent-origin and initial-byte/absence semantics, not authorship. | `docs/agent-as-provider.md:120-133` |
| C29 | E/C bind immutable evidence and bytes, are unkeyed, do not manufacture history, and publish per-file E before C. | `SPEC.md:536-570`; `docs/adrs/ADR-041-independent-capture-event-evidence.md:64-80` |
| C30 | Genuine missing C is legacy warning; other verify failures still fail. | `SPEC.md:650-664` |
| C31 | Landed verification classifies ten evidence states; only none retains forward behavior; trailers do not alone prove current content. | `SPEC.md:334-358` |
| C32 | Config keys control existing dependency/detection/candidate behavior, not adoption; upstream lock is separate data. | `internal/store/types.go:469-488`; `SPEC.md:431-434` |
| C33 | Six record capture labels and selector/claim flags describe current captures. | `internal/cli/record_capture_modes.go:27-57`; `SPEC.md:511` |
| C34 | Prepare manual bundle adoption is distinct and stricter than per-phase manual gates. | `SPEC.md:114-116`; `docs/agent-as-provider.md:48-53` |
| C35 | D6/D16 allow current exact complete derivation to create/repair provenance; preserved nonmatching manual bytes do not qualify. | `docs/adrs/ADR-036-recipe-coverage-authority.md:1509-1550`; `docs/adrs/ADR-036-recipe-coverage-authority.md:2527-2586`; `SPEC.md:610-617` |
| C36 | D8 forbids persisted contextual anchors and distinguishes complete coverage from cross-base safety. | `docs/adrs/ADR-036-recipe-coverage-authority.md:1628-1672` |
| C37 | Amended GH #15 contract includes ADR-039/040/041/042; planning addenda reflect those boundaries. | `SPEC.md:500-501`; `docs/prds/PRD-recipe-generation-authority.md:18-45` |
| C38 | Complete-operation domain is gated write-file only; origin and semantic explanation remain distinct. | `docs/adrs/ADR-039-coverage-complete-operation-domain.md:27-58`; `docs/adrs/ADR-040-p2-publication-reason-semantics.md:25-56` |
| C39 | E/C cannot be reader-backfilled; actual publication is not a cross-file transaction. | `docs/adrs/ADR-041-independent-capture-event-evidence.md:409-432`; `docs/adrs/ADR-041-independent-capture-event-evidence.md:614-629` |
| C40 | Ordered no-write success preserves initial preimage authorization and operation-position witnesses. | `docs/adrs/ADR-042-ordered-recipe-noop-proof.md:26-45` |
| C41 | V10 selects per-member anchors, never live-tree fallback; later-touch warns while malformed gates/mismatch retain their own failures. | `docs/adrs/ADR-013-verify-freshness-overlay.md:788-858` |
| C42 | V0-V10 and coverage IDs are explicit; failed non-skipped block/block-abort rows give exit 2. | `internal/workflow/verify.go:47-70`; `internal/workflow/verify.go:669-681` |
| C43 | Regeneration guidance requires a real feasible record plan, not a universal repair command. | `SPEC.md:669-675` |
| C44 | Missing native generation history is unknown, not automatically corruption; later real record can start history, not reconstruct old snapshots. | `docs/prds/PRD-feature-patch-identity-metadata.md:322-337` |
| C45 | Existing manual gate preservation and future tightening prerequisites are explicit. | `docs/prds/PRD-artifact-validation-and-provenance.md:3020-3041`; `docs/prds/PRD-artifact-validation-and-provenance.md:3078-3095` |
| C46 | Skill parity guards cover required commands and schema/verify anchor language. | `assets/assets_test.go:16-66`; `assets/assets_test.go:68-100` |
| C47 | PIB goldens freeze manual/compatibility surfaces and D1-D8; producer baseline expects manual provenance absence. | `internal/cli/prepare_pib_golden_test.go:47-58`; `internal/cli/prepare_pib_golden_test.go:325-329`; `internal/cli/prepare_pib_golden_test.go:668-700`; `internal/workflow/recipe_authority_s0_producers_test.go:752-772` |
| C48 | GH #13 shipped opt-in in v0.18.0, validates independent coverage evidence and excludes cumulative multi-slug candidate derivation. | `SPEC.md:766-783`; `CHANGELOG.md:241-250` |
| C49 | Inspected guides describe current P6/domain semantics but contain stale future-consumer prose; source/release contracts override that summary. | `docs/agent-as-provider.md:171-177`; `docs/agent-as-provider.md:213-214`; `docs/path-b-operator-guide.md:159-173`; `SPEC.md:766-780` |
