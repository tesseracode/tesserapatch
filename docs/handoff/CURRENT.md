# Current Handoff

## Status

**Cluster state**: IN PROGRESS

Research intake/backlog reconciliation is complete, archived and pushed at
`c8344df`. GH #13's Proposed rev-7/ADR-043 review requires revision: missing
explicit capture-selector comparison, an impossible schema-validator claim,
and inconsistent E4/E5 numbering. Coordinator also requires truthful interrupted
E/C recovery wording. No runtime implementation is authorized.

v0.17.0 is already published from `d1d6c3f`; do not republish or move its tag.
Its accepted work, evidence and release are recorded in [HISTORY](HISTORY.md).
The prior accumulated CURRENT snapshot remains in Git at `0ca98db`; historical
pre-release paragraphs are not active instructions.

## Active Task

- **Task ID**: `plan-gh13-independent-capture-evidence`
- **Milestone**: GH #13 / ADR-041 section 7 planning amendment
- **Issue**: [GH #13](https://github.com/tesseracode/tesserapatch/issues/13)
- **Description**: Bind independent capture evidence into replay-candidate gates, identity and recovery planning
- **Status**: In progress — correcting independent planning findings; runtime undispatched
- **Assigned**: 2026-09-22
- **WAVE_BASE**: `6e8096e03849617a240fa586b109e60f44fad69f`

WAVE_BASE = 6e8096e03849617a240fa586b109e60f44fad69f

## Session Summary

Independent `gpt-5.4` review is NEEDS REVISION on one high/two medium contract
issues. Add explicit E/C capture-mode/pathspec/claim comparison, separate shape
validation from canonical-digest recomputation, and align current rev-7 gate
numbering. Also qualify ROC-299 so interrupted E/C pairs are detectable rather
than falsely impossible. Return only the three planning files to the implementer.

Coordinator added Proposed-only backlinks in ADR-041, the ADR index and CLUSTERS.
The index's stale v0.16/S5 runtime-pending statements now point to shipped
v0.16/v0.17 history. ADR-043 reserves number 043; next unused number is 044.
No draft is marked accepted. Extended structural checks also verify unchanged
C1/C2/D2/D3 regions, fourteen transaction steps and 132 planning-link targets.

The verified `gpt-5.4` implementer delivered only ADR-043, ADR-037 and the
companion PRD. The draft binds canonical E digest in candidate identity/schema,
adds three capture-event refusal codes within a fifteen-gate inventory, and
carries E through staged publication/snapshots/rollback/recovery. These are
proposed contract changes, not accepted runtime behavior.

Coordinator structural checks pass: all nine actual parity IDs
`A,B,C1,C2,D1,D2,D3,E,F` are unique and byte-identical across ADR/PRD;
ROC-001..301 are contiguous/unique with I61/C87/G76/U68/S9; `gatesPassed[15]`
matches E1-E15. Prior rev-0..rev-6 revision-table rows remain byte-identical.
Semantic first-match/recovery correctness still requires independent review;
structural parity alone is not acceptance.

Intake closed and pushed before planning dispatch. The operator selected
`identity-digest`: add validated E's canonical SHA-256 to immutable candidate
identity. Amend the accepted rev-6 ADR-037/PRD coherently and record the
architecture rationale in ADR-043. The revision is proposed until independent
review; no accepted shipped producer contract is silently widened.

### Completed intake context

All thirteen previously untracked research files are now tracked in `0ca98db`.
The intake preserves paper-only approval/exploration statuses, append-only turn
logs and historical evidence uncertainty. WP-004 remains the preferred later
planning track, not an automatically graduated PRD. Recurring patches remain
blocked on their missing metadata-boundary ADR; WP-006/007 remain Exploring.

TWS is identified as an aggregate historical development study, not a normal
reconciliation-validator corpus. The reported seven-day duration and the
summary's derived five-date window remain explicitly unresolved. No missing
feature/hunk/patch rows were fabricated. Risky old workflow recommendations
are marked as historical suggestions, not accepted runtime policy.

Research indexes and CLUSTERS now distinguish shipped work from future proposals.
The allowlist has no active exemptions: thirteen paths became tracked and the
WP-005 pair was already tracked. Historical hotfix/singleton/superseded-paper
dispositions remain visible.

GH #15 was closed with release evidence and explicit ADR-036 D8 anchor deferral.
GH #13 remains open with the shipped prerequisite satisfied and ADR-041 section
7's planning amendment outstanding. Other issues retain their own scopes.
The operator chose `identity-digest`: bind the validated capture event's canonical
SHA-256 into candidate identity in the subsequent planning amendment.

## Current State

- Planning ownership: implementer owns only new
  `docs/adrs/ADR-043-operation-candidate-capture-evidence.md`, ADR-037 and its
  companion PRD. Coordinator owns ADR-041's completion backlink, indexes/
  dashboard/tracking and issue updates after the amendment is reviewed.
- Preserve ADR-036 D3/schema, ADR-039 domain, ADR-040 semantics, D16 origin,
  no-autorepair, existing candidate isolation/locking and legacy fallthrough.
- Required planning surfaces: closed/reachable first-match eligibility gates;
  E-derived reference/capture/exclusion inputs and offline reconstruction;
  canonical E digest in identity/schema/reuse/staleness/acceptance;
  staged E-before-C and complete snapshot/rollback/recovery set for both P3
  variants; synchronized parity blocks, refusal counts and matrix updates.

- No runtime, installed skill, test source, dependency, wire schema or release change.
- Research artifacts are intake evidence, not implementation authority.
- Backlog has eleven open issues after GH #15 closure.
- Current queue: GH #13 capture-evidence planning; WP-004 as a separate later
  planning proposal; GH #9/#17 reliability; GH #18-22 v0.17 re-baselining/planning;
  GH #12/#14/#24 remain evidence-gated research or contract planning.
- No replay implementation, scheduler, substrate interface or decision-ticket
  feature is authorized by this work.

## Files Changed

- Current planning scope: ADR-043, ADR-037 and the GH #13 companion PRD,
  with Proposed backlinks in ADR-041, ADR index and CLUSTERS plus tracking.
  No source, asset, test or new
  runtime schema implementation.

- Thirteen imported files: recurring-patches PRD; WP-004/006/007 and their
  companion turn logs; t3code dependency-analysis addendum; five TWS aggregate
  study files.
- Research indexes: `docs/whitepapers/README.md`, `docs/state-of-the-art/README.md`,
  `docs/state-of-the-art/case-studies/README.md`.
- Dashboard/sentinel: `docs/CLUSTERS.md`, `.wave-close-allowlist`.
- Tracking: CURRENT, ROADMAP and supervisor LOG; HISTORY at phase closure.
- GitHub: GH #15 closed as shipped under accepted scope; GH #13 prerequisite
  and planning comment added. No other issue was closed.

## Test Results

- Proposed amendment structural PASS: nine paired regions byte-identical;
  301 contiguous ROC rows (I61/C87/G76/U68/S9); fifteen gates/proof array agree;
  fourteen transaction steps, unchanged C1/C2/D2/D3 blocks, 132 planning-link
  targets exist; historical rev-0..rev-6 revision rows unchanged. No Go execution or runtime
  acceptance claim. Semantic review and remaining wave-close checks are pending.

- `git diff --check`: PASS.
- Coordinator checks **141** in-repository Markdown file targets: all exist.
- Both TWS JSON files parse; study IDs and counts agree: 42 = 35 + 7 features,
  18 tags. Reported duration 7 and derived five-date window remain explicit.
- No active allowlist entries remain; both removed WP-005 paths were already tracked.
- No Go/runtime test was run or claimed. The aggregate study is not represented
  as passing the normal reconciliation-dataset validator.
- Independent intake review: no reported content finding; handoff consistency
  HIGH and closed-issue routing MEDIUM are corrected at `2a4d2f4` and directly
  verified by the coordinator. Final planning-wave review remains separate.

## Next Steps

1. Author the ADR-041 section-7 amendment and ADR-043 rationale for the selected
   identity-digest direction; synchronize every affected ADR-037/PRD surface.
2. Check document parity/counts and obtain independent OpenAI review; keep
   changes Proposed until accepted. Final wave closure follows review/checks.
3. Do not implement GH #13 or auto-graduate WP-004. Their implementation/PRD
   assignments remain separate from this documentation/planning request.

## Blockers

No intake blocker remains. The GH #13 amendment has three independent review
findings plus one coordinator recovery-claim correction to resolve before
acceptance. GH #24 widening is not a prerequisite for this narrow amendment.

## Context for Next Agent

- **Model constraint**: OpenAI/MAI only for this work. Both task calls explicitly
  selected `gpt-5.4`; runtime metadata confirms that model for implementer
  `e50ddbe5-44d6-4d7e-9cd2-8cab288dc8be` and reviewer
  `fe74222d-a22c-4e34-96d7-f1dc558fc078`. No optional unverified AI feature is used.
- Coordinator owns tracking/GitHub actions; agents do not commit or run Go.
- User authorized integration of all thirteen research files; they are no longer
  untracked WIP. Preserve their historical claims/turns when making corrections.
- Candidate identity direction was explicitly selected as `identity-digest`.
  Implementing replay itself is not authorized.
- v0.17.0 tag object `e7c748ada40988fe72f47b638cb37d07e24f0ac3` remains at
  `d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`. No tag operation is part of intake.
- Existing fresh-resource/serial validation rules remain applicable if Go
  validation is needed later; no previous gate result certifies new changes.
