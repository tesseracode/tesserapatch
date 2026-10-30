# Overnight Development — 2026-10-29

## Authorization and limits

The operator authorized autonomous sequential development for approximately
12 hours, starting with GH #13 S3 and continuing to ready stages/clusters only
after each predecessor is accepted. Window: 2026-10-30 06:00-18:00 UTC
(2026-10-29 23:00 through 2026-10-30 11:00 PDT).

Keep the existing implementation/independent-review loop, native CI and the
complete admitted 39-command local gate. Do not relax memory/swap/load guards,
skip assertions, adopt unreviewed code or count partial runs as complete.
At the window boundary, preserve any unfinished stage with an explicit status.
No automatic scope expansion into unaccepted product contracts.

## Stage scoreboard

| Stage | Status | Baseline / checkpoint | Review and validation |
|-------|--------|-----------------------|-----------------------|
| GH #13 S3 | ACCEPTED and pushed | code `083d20f`; close `5d22acf` | Review/local/native pass; closing CI `41028948776` also green |
| GH #13 S4 | Acceptance RESTORED and pushed | validated `c4f2f1e`, close `f9d5501` | Fresh review, 39/39 local tests and native five jobs + Windows CLW pass |
| GH #13 S5 | Independent review / remaining basic checks | `1e1dae6`, WAVE_BASE `f9d5501` | Reviewer `7712b628`; affected pass; full gates pending |
| GH #13 S6+ | Not dispatched | Depends on accepted S5 | Pending |

## Decisions

### ON-001 — Preserve serial acceptance boundaries

Advance only after independent review, native CI, all 39 local shards,
tracking/archive updates and push are complete. Full-gate time is material
(recently about 100 minutes), so do not equate parallel preparation with
permission to dispatch a dependent stage. Do not cancel an active exact-tip
CI run with tracking-only pushes.

### ON-002 — S3 follows the accepted exact algorithm

Use the literal A1-A8 / L1-L3 contract: byte-exact records, all-optimal F/G
unanimity, no traceback/tie selection, two int32 matrices and fixed limits.
Apply the specified ordering per sorted file and boundary; refuse the whole
plan on a structural failure, without publishing partial operations.
No configurable limits, approximate matching or provider fallback.

### ON-003 — Ephemeral snapshot and planner boundary

Proposed implementation choice is recorded in ADR-045: freeze upstream object
identities before classification, read bounded immutable blobs per file, and
keep plans/payloads ephemeral. Do not retain an unbounded whole-repository
body snapshot or execute persisted recipe contents. Independent S3 review
must validate this choice before acceptance.
The draft captures both trees' path/object metadata before classification,
then reads per-file immutable objects. Modify inputs obey L1 before body
reads and matrix allocation. Extracted insertion runs are independently
replayed through a streaming digest before alignment; a correct walker digest
alone is not sufficient proof that the extracted runs are correct.

### ON-004 — Fix complexity without adding a new limit

Independent review found O(boundaries * base * upstream) pair rescans despite
the bounded matrices. Do not invent a fourth boundary-count limit: L1-L3 are
closed contract constants. Compute cut bounds first and reuse matrix boundary
columns for prefix/suffix optimal-pair extrema in one grid pass. This keeps
both the accepted predicate and the exact 8 MiB matrix budget. Validate with
the exhaustive oracle and a many-boundary case at the L3 ceiling.

## Operator review notes

### ON-005 — Candidate execution snapshot proposal needs independent review

The implementer proposes a shared derivation core with candidate-rooted
worktree snapshots for the second pass, preserving commit-only public
reference APIs (ADR-046). This is not yet accepted: review must establish
the snapshot/unchanged-tree proof and lock/cleanup contract. The draft's
premature Accepted ADR label was restored to Proposed before review.
Independent bounded review accepts the rooted candidate snapshot binding,
but requires one-path expected-content validation to preserve ADR-045's
body-lifetime discipline. No additional snapshot architecture is introduced.

### ON-006 — Keep postconditions distinct from idempotency

The routing correction exposed a separate contract gap: an all-present
second-pass gate makes the later applicable-effect postcondition check
ineffective for real boundary corruption. Validate exact inserted/created
payloads at derived positions before running the production second pass;
then require every effect present with stable tree/diff. This preserves both
ROC-140's postcondition refusal and mandatory candidate-not-idempotent
second-pass outcomes, rather than relabeling one as the other.

### ON-007 — Reopen green work when contract evidence contradicts it

Prerequisite inspection exposed bare-root arguments in S4 Execute/executeWithPlan
and a required global handle spanning execution. Both contradict the accepted
typed-root/short-lock contract despite green tests and review. Do not proceed
to S5 or rationalize the API; repair under a new recorded wave base, preserve
the original acceptance audit, and repeat full review/validation. ADR-047
will document the typed execution and lock-ownership correction.

### ON-008 — Preserve the deadline without shortening validation

After repaired S4 close, roughly 3.3 hours remain. Begin accepted S5 because
it is ready, but do not equate a draft with acceptance or skip full gates
to fit the window. An attached deadline timer will trigger checkpointing;
any unfinished stage keeps an explicit unaccepted status and remaining work.

S3 closed durably at 08:51 UTC, leaving roughly nine hours. S4 is scoped to
the accepted execution/structural-validation contract only. Parent supervises
tracking and validation infrastructure; one bounded implementation agent may
own production/tests/ADR, followed by one fresh independent reviewer. No two
implementers edit a shared file concurrently.

S0-S2 are accepted. S2 closing-tip CI `41010587650` now passes all five
required jobs; S2 code was already fully validated at `8d3b9ef`.
Upstream Git-strategy research remains queued, not automatically selected
ahead of already accepted implementation slices.

Detailed progress, failures and review verdicts remain in CURRENT and LOG;
completed handoffs are archived immediately in HISTORY.
