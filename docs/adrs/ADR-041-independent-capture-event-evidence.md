# ADR-041 - Independent Capture-Event Consistency Evidence

**Status**: Accepted rev-1 — independent contract review approved 2026-09-09
**Date**: 2026-09-09
**Owner**: Core
**Issue**: [GH #15](https://github.com/tesseracode/tesserapatch/issues/15), S5
**Direction selected**: `independent-evidence` (operator, 2026-09-09)
**Planning baseline**: pushed `c2f0581`; S5 WAVE_BASE
`537ffd9bff153efe37afa3bc6d66f4e00fc55d35`
**Amends**: [ADR-036](./ADR-036-recipe-coverage-authority.md)
D2/D9/D10/D11/D13/D15/D17 and
[PRD-recipe-generation-authority](../prds/PRD-recipe-generation-authority.md)
as enumerated in §9
**Preserves**: ADR-036 D3 and its canonical ten predicates, D16,
[ADR-039](./ADR-039-coverage-complete-operation-domain.md),
[ADR-040](./ADR-040-p2-publication-reason-semantics.md)'s semantic-reason policy
**Related, not amended here**:
[ADR-037](./ADR-037-reconcile-operation-replay-candidate-authority.md),
[PRD-reconcile-operation-replay-candidate](../prds/PRD-reconcile-operation-replay-candidate.md)

The operator selected the independent-evidence direction. Independent rev-0
review required binary-payload and parent-exclusion corrections; rev-1 closes
both findings and is APPROVED. The supervisor accepts this concrete contract
for bounded S5 implementation. Acceptance is not a claim that runtime code
is implemented, validated or reviewed. GH #13 candidate work, GH #24 widening,
S6 public documentation, release and tag remain outside this authorization.

## 1. Problem and bounded decision

D9 promises independent comparison of a coverage record's capture descriptor.
The accepted S4 publication contract cannot supply that comparison:

- `internal/workflow/patch_generations.go:57-62` returns on the same patch
  hash before writing new generation metadata.
- `internal/workflow/recipe_coverage_publish.go:141-170` builds coverage
  from the current immutable observation even when no generation is appended.
- `internal/cli/feature_patch.go:129-141` publishes coverage alone on P2's
  non-empty same-patch checkpoint. P6/P7 can likewise publish `no-capture`
  beside an older capturing generation.

A latest-generation comparison therefore rejects real events. Feeding
`coverage.capture` back to its own validator cannot detect capture-only drift.
Neither a `producer` label nor a generation ID closes that gap.

**Decision:** add one replace-in-place per-feature artifact,
`artifacts/recipe-capture-event.json` (below, **E**), independently constructed
from the same immutable S1 observation and exact final bound-artifact inputs
that produce `artifacts/recipe-coverage.json` (**C**). Publish E atomically,
then C atomically **last**, on every already-governed P1-P7 event. Readers
independently compare E with C and with their captured patch/recipe bytes;
they still reconstruct content and trees. No coverage-schema version bump,
new completeness predicate, journal, generation entry, timestamp or nonce.

### What this evidence does and does not prove

E is an **unkeyed consistency authority for the event descriptor**, independent
of C as a persisted comparison input. It is not an independent witness to who
ran a command. A producer derives its fields from observation inputs, not by
decoding C and copying its claims; C contributes only its final raw-byte hash
for pairing. At read time E is loaded independently, not synthesized from C.

Edits to one artifact's bound capture/reference/observation or pairing fields,
missing dependencies and differing mixed publications are detectable.
Coordinated consistent editing of E, C, patch and recipe is
**not cryptographic authentication**, authorship or historical proof. A
consistent rollback of the whole local set is likewise not detected as a
rollback. There is no append-only history, monotonic event sequence or claim
that two identical invocations are distinguishable. D16 remains a derivation
performed now with total raw canonical-byte equality, not historical origin
inferred from either artifact.

## 2. Existing primitives and alternatives

| Alternative | Decision and reason |
|---|---|
| Compare C to the latest generation | Reject: generations track patch-byte change, not all capture events; false-rejects P1/P2 same-byte events and P6/P7 |
| Always append or mutate a generation | Reject: changes ADR-024's history boundary, P2 checkpoint semantics and more writers than necessary; generation identity still is not a content proof |
| Treat capture as descriptive, or copy it into its own validator | Reject: the operator selected independent evidence to preserve D9's capture-drift refusal |
| Add another field/hash inside C alone | Reject: no independent persisted descriptor; hashing self-description alone is circular as a verification argument |
| Companion containing only C's hash | Insufficient: detects C-only changes but supplies neither an independent capture/reference comparison nor the observation-availability ceiling needed by read reconstruction |
| Persist source bodies, a signed log or a transaction journal | Reject for this task: unnecessary privacy/storage/key/lifecycle expansion; signing without a separately managed trust root is not authentication |
| One small typed companion with a one-way C hash | Select: covers all existing events, retains coverage-last and single-file atomicity, and exposes rather than conceals interrupted publication |

The repeated observation fields below are necessary comparison inputs, not a
second effect grammar or a second coverage schema. Operation assignments,
coverage status, reason arrays, contextual hints, provenance and generation
metadata remain out of E. Existing strict S1/S3 types and digest definitions
are reused; no new complete-operation domain is introduced.

## 3. E v1: exact typed schema and identity

Path: `.tpatch/features/<requested-slug>/artifacts/recipe-capture-event.json`.
The reader constructs this path from its validated requested slug, never from
a field in either artifact. It uses the same artifact path-safety policy as C.

This is the sole E schema; placeholders below denote full digests,
not literal accepted values. The example is a P1 existing-file write.

```json
{
  "schema_version": 1,
  "feature": "fix-model-id-translation",
  "patch_present": true,
  "patch_sha256": "<64 lowercase hex>",
  "recipe_present": true,
  "recipe_sha256": "<64 lowercase hex>",
  "reference": {
    "kind": "commit",
    "commit": "<40 lowercase hex>",
    "preimage_set_sha256": "<64 lowercase hex>"
  },
  "capture": {
    "mode": "working-tree-all",
    "pathspecs": [],
    "claim_ids": []
  },
  "event": {
    "patch_rewritten": true,
    "recipe_regenerated": true,
    "bound_artifact_edited": false,
    "stale_marker_present": false
  },
  "parent_created_paths": [],
  "observations": [
    {
      "ordinal": 1,
      "change_kind": "modify",
      "content_kind": "text",
      "object_kind": "regular",
      "path": "command.go",
      "old_path": "",
      "old_mode": "100644",
      "new_mode": "100644",
      "preimage_observed": true,
      "preimage_present": true,
      "preimage_sha256": "<64 lowercase hex>",
      "postimage_observed": true,
      "postimage_present": true,
      "postimage_sha256": "<64 lowercase hex>",
      "patch_fragment_sha256": "<64 lowercase hex>"
    }
  ],
  "coverage_sha256": "<64 lowercase hex>"
}
```

### 3.1 Field rules

All fields shown are required, in exactly these types: `schema_version` and
`ordinal` are JSON integers; `*_present`, `*_observed` and the four `event`
members are booleans; all other leaves are strings; `pathspecs`, `claim_ids`,
`parent_created_paths` and `observations` are arrays, never null. No optional fields, duplicate
members, unknown fields, trailing JSON value, invalid UTF-8 or lone surrogate
escapes. Unknown schema versions refuse. No permissive fallback decoder.

- **Owner:** `feature` is the exact requested feature slug. E and C must each
  match that slug, independently of matching each other. A pair copied
  together from a different feature still refuses.
- **Bound inputs:** `patch_present` / `recipe_present` retain D3/D9's
  **readable-existence** meaning. True requires the digest of exact bytes,
  including zero-byte or undecodable files; false requires `""` and no bytes.
  Absence and unreadability remain collapsed in these two persisted flags,
  but the runtime read result retains the actual path and error. No persisted
  third state or fabricated empty-file hash. E deliberately does not duplicate
  `recipe_decodable`: readers recompute it from the captured raw recipe.
- **Reference:** the three fields and digest algorithm are exactly D2/D3's.
  `kind` is `commit`, `unavailable` or reserved `index-snapshot`; `commit`
  is 40 lowercase hex for `commit`, otherwise `""`.
  `preimage_set_sha256` is always 64 lowercase hex, including the defined
  empty-set digest. Reuse S1 `PreimageSetDigest`, whose projection includes
  both sides, axes, modes and path-byte encodings
  (`internal/patchobs/patchobs.go:884-921`); do not hash only preimage bodies
  because of the digest's name.
- **Capture:** `mode` is exactly `working-tree-all`, `staged-index`,
  `unstaged-worktree`, `committed-range`, `auto-committed-range`,
  `explicit-committed-range`, `reconcile` or `no-capture`.
  Selector arrays use D3's string validation, ascending bytewise order and
  deduplication. `no-capture` requires both arrays empty. Reject contradictory
  inputs rather than silently clearing or sorting them in a decoder.
- **Event facts:** the four booleans are the existing S3 `CoverageEvents`
  inputs, not new authority labels. `patch_rewritten` describes an actual
  successful canonical patch write, `recipe_regenerated` means the authorized
  record-derivation write occurred (not merely a provider/manual recipe write),
  `bound_artifact_edited` describes P7's observed canonical mutation, and
  `stale_marker_present` is the marker fact used at publication. They are
  supplied from the real event/finalizer, not inferred from C's reason list
  or from a producer name. They allow exact semantic reason recomputation
  under D3/ADR-040, never an origin claim. No successful recipe regeneration
  can be recorded beside `recipe_present: false`. A boolean change that has
  no effect on any applicable condition is not independently authenticated
  history; this contract does not promise to detect it merely from a changed
  event identity.
- **Parent exclusion input:** `parent_created_paths` is the effective
  `Observation.ParentCreatedPaths` set supplied to S3, independently sourced
  from frozen producer discovery plus S4's deterministic augmentation from
  `created_by` operations in the exact final recipe. It is not inferred from
  C's reasons, effect dispositions or status, and not reloaded from current
  parent state at read time. A captured exclusion without recipe `created_by`
  is valid and must survive; see the accepted cohort in
  `recipe_authority_s3_test.go:598-615`.
  Producers normalize lexical targets through the same root-relative
  execution semantics as `coverageParents`/`coveragePath`, then persist
  slash-separated repository-relative paths in ascending bytewise,
  deduplicated order. Equivalent spellings such as `./a.txt` and
  `folder/../a.txt` produce `a.txt`; normalization requires no live filesystem
  lookup. Empty, escaping, absolute or otherwise unsafe targets refuse.
  Decoders require that canonical representation and never repair it.
  Preserve the whole effective set, including paths outside this patch.
  Readers feed this independently captured set into the unchanged S3
  exclusion rule; they do not reconstruct it from C's claimed exclusion.
  E's content identity includes the field; S1's existing preimage-set digest
  definition is unchanged. As with inert event facts, a coordinated or
  semantically inert exclusion edit is not authenticated historical truth.
- **Observation projection:** one entry per normalized S1 effect in gapless
  one-based order, exactly matching C's corresponding observation fields.
  Field grammar is D3's: `change_kind` = `add|modify|delete|rename|copy`;
  `content_kind` = `text|binary|none|unknown`; `object_kind` =
  `regular|executable|symlink|gitlink|unknown`. Modes are exactly `100644`,
  `100755`, `120000`, `160000` or `""`. Paths, hash conditions,
  extant-side contradictions and fragment boundaries retain S1/S3's rules.
  There is no independent E parser for patches. A legitimate normalized
  typechange spans its strict record pair just as in S1.
- An unobserved side has false presence, empty mode/hash and its existing
  availability reason in C. Observed absence uses D3's absence encoding;
  observed-and-absent on an extant side is refused. Unknown axes and partial
  observations remain incomplete. The effect set follows D3 exactly:
  absent patch, semantic emptiness or parse
  refusal produces `observations: []`, never invented effects. A missing
  patch cannot carry observations. Source bodies, I/O error text, prompts,
  credentials and absolute checkout paths are never persisted in E.
- **Pairing:** `coverage_sha256` is SHA-256 of the exact final bytes returned
  by the canonical C encoder, including its final LF. It is never a hash of
  reserialized C at read time. It binds *all* C fields, including producer
  context, reason arrays and operation assignments, without making those
  fields proofs of origin.

### 3.2 Canonical bytes; stable identity; no circular dependency

Use a fixed-field typed encoder in the displayed field order, with the same
JSON escaping as S3's `json.MarshalIndent`, two-space indentation and exactly
one trailing LF (`recipe_coverage_codec.go:77-87`). Builders establish valid
order; encoders validate, never repair. Arrays preserve their prescribed
ordering. E's deterministic content identity is the full lowercase SHA-256 of
its canonical encoded bytes; no `event_id` member is persisted.

Construction is a directed dependency:

```text
immutable observation + exact final patch/recipe inputs + actual event facts
             ├── S3 build/validate ── canonical C bytes ── SHA256(C)
             └── E projection + SHA256(C) ── canonical E bytes ── identity
```

C contains no E digest. E contains no self-digest. Identical complete inputs
produce byte-identical E and C on rerun, even if an atomic replacement occurs.
A different capture for identical patch/recipe bytes changes E and C but does
not need a generation append. Equality of identities denotes equality of
recorded content, not equality of invocations or proof of sequence.

## 4. Observation and reconstruction boundary

### 4.1 Producer inputs

P1-P6 use the **same immutable pre-write S1 observation**, not a fresh live
capture in the finalizer. An authorized planned recipe replacement supplies
its exact final bytes; a preserved recipe supplies its frozen bytes and read
result. A successful write must be of those bytes. E's descriptor and
observations are projected directly from that input; only the one-way pairing
hash comes from C. Changes to observed bytes after planning cause a refusal,
not silent re-observation or rewriting of the descriptor to fit C.

The projection also preserves the effective parent-exclusion input used by
the S3 build. Both E and C consume the same frozen discovery set and the same
deterministic final-recipe augmentation. C's reason arrays are never a source
for that set, even when they happen to reveal an exclusion.

P7 retains D2's necessary before/after exception: freeze the before snapshot
and any permitted reference reconstruction before starting the editor; take
the after snapshot on its return, including error return. The final immutable
observation is built from that fixed context and those exact resulting bytes.
No finalizer rereads the worktree to improve that observation. A missing or
unreadable after-snapshot is not guessed to be unchanged; preserve the
underlying failure and the accepted S4 invalidation/refusal behavior.

The existing mode/reference map remains: capture modes use the resolved lower
commit (HEAD for working-tree/staged/accepted unstaged, accepted upstream for
`reconcile`) or `unavailable`; v1 producers never emit `index-snapshot`.
Committed ranges bind their resolved lower reference and captured effects,
not a live HEAD substituted later. E adds no upper-commit or origin assertion
that S1 did not retain.

P6/P7 keep `no-capture` even if a durable reference is reconstructed. They
may carry `commit` only after the prior bound inputs validate and the
independently reconstructed preimage-set digest matches. Once this amendment
is implemented that prior validation includes E/C pairing; an old S4 C alone
does not establish it. If it fails, publish truthful `unavailable`/incomplete,
not a copied prior commit. A real capturing producer can establish fresh
authority without trusting either prior artifact.

### 4.2 Read-side evidence is a ceiling, not bytes or observation

Readers separate three results: strict record consistency, binding consistency,
and independently reconstructed content proof. E never supplies source bodies
and cannot replace any of them. The same inputs are used throughout a read:

1. Recompute readable existence and exact patch/recipe hashes from the
   inventory and compare each flag/hash to **both** E and C, strict-decode
   the recipe, and run the existing strict patch grammar. Compare counts,
   order, paths and fragment hashes independently.
2. Compare E's reference, capture, event-derived semantic conditions and
   observation projection with C, rather than copying C into an expected
   observation. Recompute the S1 digest over E's validated projection and
   compare both reference digests. This is a consistency check, **not a tree
   proof**. Supply E's independently captured `parent_created_paths` to S3's
   existing parent-exclusion logic; compare the resulting applicable reasons
   with C. An empty final-recipe `created_by` field does not erase that input.
3. For a `commit` reference, reconstruct its objects offline from the named
   local commit, not from a generation ID or the current worktree. Obtain
   preimage existence, modes and bytes independently; derive postimages from
   that base and exact canonical patch using the existing strict machinery.
   Recompute every reconstructable side hash, effect digest and preimage-set
   digest, and run the unchanged S3 validator/simulation on actual bytes
   wherever that full observation can be supplied.
4. E's `*_observed: false` is an **availability ceiling** for this event.
   Even if a later read can reconstruct that side, it may not promote the
   event to observed/complete, erase its reason, or treat header text as an
   observed mode/kind. Validate reconstructed facts without rewriting the
   persisted event. True never licenses synthetic bytes: an expected-present
   hash without independently available matching bytes is not a content proof.

**Incomplete records need an explicit limit, not an invented proof.** If
the event itself had no durable reference, an unobserved side, or an observed
side whose body the persisted unsupported patch representation cannot
reconstruct,
the reader can establish raw-artifact/pair/descriptor consistency and the
strict D3 incomplete shape, but cannot reconstruct unavailable historical
source bodies. It reports that limitation and retains the exact incomplete
reasons. It checks every independently reconstructable field; it does not
call the full S3 validator with hash-only or coverage-derived fake bodies,
claim full reconstruction succeeded, or waive a failed strict validation.
This is the limited meaning of *valid incomplete* in D13 rung 3 / D17
orders 2-4, not replay authority. It is essential for truthful no-capture P6/P7
and ADR-038 retention-limited observations, as well as fully observed binary
stubs, to stay incomplete rather than become fabricated observations or
universal binding failures.

**Observed at publication does not imply reconstructable from the stored
patch.** Ordinary capture uses `git diff` without `--binary`
(`internal/gitutil/gitutil.go:354`); a binary modification can therefore
publish two observed sides but persist only `Binary files ... differ`.
The local reference reconstructs the preimage; neither E nor that marker
contains the postimage bytes. This is an inherent payload limitation, not
evidence of a subsequently lost object. The existing S3 observed binary
cohort is legitimate incomplete coverage (`recipe_authority_s3_test.go:268`).

For this cohort, and a strict-grammar unsupported effect that likewise has
no executable postimage payload, the reader must:

- retain the true publication-time observation flags, modes and hashes;
  never change them to unobserved, invent bytes, or add availability reasons
  that were not true at publication;
- require valid paired incomplete C with `cross_base_status: unsupported`
  and the applicable existing capability exclusions; independently check
  the grammar/fragment/axes facts that establish the unsupported shape;
- validate every independently available side, preimage object, mode, hash
  and representable text effect. A mixed text/binary record does not exempt
  the text portion from exact reconstruction or excuse a text-side mismatch;
- report exactly which sides lack reconstructable payload. Their body
  hashes have only E/C consistency verification, not independent body proof.
  Verify's coverage row remains rung-3 warning, explicit apply retains D17
  orders 2-4, and doctor remains warning-only; no complete authority or
  automatic complete-regeneration claim follows from this limited result.

The limitation must be established from the bound strict patch shape, never
from an arbitrary reconstruction error or C's label alone. A malformed hunk,
failed application of an otherwise reconstructable text effect, hash/mode
mismatch, or missing local object required by the declared durable reference
remains a binding failure. Do not use today's worktree, recipe output or an
unbound cached blob to fill an omitted historical binary payload. Optional
cache contents cannot change this deterministic validation classification.

Conversely, losing a reference object needed by a record that claims a
durable reconstructed binding, or finding different bytes for an observed
side that can be reconstructed, is binding failure, not a reason to demote
that record and execute it. `complete` must survive **all** D9 content/tree
checks and all ten predicates; E alone can never make it valid. Reserved
`index-snapshot` is non-durable and ineligible, even with a consistent E.
This qualification of D9's universal recomputation wording is explicit in
§9; it does not change the strict S3 core or its accepted complete domain.

`event.stale_marker_present` records publication-time input, not a promise
that a marker can never appear later. A newly present live marker follows
D13's warning precedence; it is not by itself E drift. Existing incomplete
marker reasons remain in the record even if a marker later disappears.
Readers neither remove those reasons nor republish.

## 5. Reopened S4 publication contract

### 5.1 Sequence and ownership

The single shared publication API owns **both E and C**. Each producer first
establishes a real D15 category-(a), -(b) or -(c) event. It prepares and
validates both outputs from immutable inputs in memory, including the C hash,
before publishing either. Conditional earlier writes retain their existing
authorization and ordering:

1. Bound patch/recipe writes and checkpoints at their existing event boundary;
   authorized recipe generation, provenance and generation metadata in their
   existing order. Never invent a write merely to obtain an event.
2. Existing producer-specific state/error attempts; notably P6's provenance
   attempt then state-mark attempt after its recipe write.
3. Single-file atomic publication of **E**.
4. Single-file atomic publication of **C last**.
5. Propagate the combined result. Emit the common status/reasons line only
   after successful completion, never after a publication or primary error.

Neither E nor C is written by a reader. There is no cross-file transaction,
rollback promise, new lock or history append. Failure of the E write/rename
prevents the C write. Failure of the C write/rename is returned. **Every**
current publication error is nonzero, with the actual failing path/cause;
the existing producer error wrapper covers E too, rather than disguising it
as an incomplete-coverage warning.

### 5.2 All seven producers; unchanged no-events

| Producer/event | Required publication; preserved boundary |
|---|---|
| P1 `record`, including `land` orchestration | E then C for every successful bound event and every autogen outcome, including preserved/noop recipes and identical patch bytes; generation no-op never suppresses the pair |
| P2 patch-writing `refresh\|fixup` | E then C; preserve D16 origin restriction and ADR-040's exact semantic rewrite reasons |
| P2 category-(c), non-empty capture matching latest generation | **Exactly E and C may change**. No canonical/numbered patch, recipe, provenance, generation, marker or state write. Preserve skipped message after successful publication; no rewrite fact/reason. The checkpoint's existing semantic completeness test remains, not a new D16 origin demand |
| P3 `RefreshAfterAccept` | E then C on each successful patch write, including same-byte writes with no generation append; unchanged non-regenerating policy |
| P4 `cycle` patch step | E then C only after its successful patch event; the earlier implement step separately owes P6's pair |
| P5 `apply --mode done` | E then C only when it writes the canonical patch; reapply and empty capture remain no-events |
| P6 valid-JSON and raw-unmarshal-failure writes; manual checkpoint | Both writes and the successful manual checkpoint owe E then C, using `no-capture`; raw bytes remain bound even if undecodable. Recipe write → provenance attempt → state attempt → E → C → return. State failure cannot cancel publication |
| P7 resolved canonical edit with mutation | Before/after snapshot rule unchanged; E then C before returning an editor error. Root-decoy/unrelated edits are not events; no new trigger on edits to E or C themselves |

No-event invocations write **neither artifact**: P2 empty capture, P4/P5
empty-capture/no-write paths, P4 early exits before its patch event, canonical
reapply reads, failed writes with no successful event, P7 no change,
`$EDITOR` unset, unresolved/no-process paths, unrelated/root-decoy edits.
P4's early exit must still retain a pair already published by its earlier
P6 event. A GUI editor's later asynchronous save remains external tamper.

P6 error order remains primary state failure first, publication failure next,
both preserved/chained; neither is swallowed. P7 similarly retains editor
failure first and publication failure next. A successful pair can describe a
real write even though the enclosing state/editor operation failed; that run
still returns nonzero and emits no success-shaped completion status.

### 5.3 Fault boundaries and recovery

| Boundary/fault | Durable result and reader behavior |
|---|---|
| Before any successful bound event | No owed E/C; earlier valid pair or absence is unchanged |
| After bound/recipe/provenance/generation work, before E replaces old E | Prior E/C or absence remains. Changed bound bytes fail independent binding checks when C exists. Identical bound bytes can leave the prior pair valid: it describes the last completed publication, **not the interrupted new event** |
| E encode/write/rename fails | C is not attempted; old E or absence remains, never partial E. Return nonzero. The same prior-pair limitation applies |
| E replaced, before C replacement, or C rename fails | New E with old C detects a mixed pair via raw C hash and descriptor comparison when outputs differ; new E with absent C stays the uniformly missing-coverage state. Never infer that E commits C |
| C replacement succeeds | Both files are whole records. Readers still verify E/C, raw artifacts and tree/effect bindings; a matching pair is not a shortcut |
| Subsequent concurrent/external change | Snapshot instability or independent mismatch refuses; no claim of protection against undetectable ABA/coordinated consistent edits |

If all newly prepared bytes equal the old bytes, an interruption need not
produce a mismatch. **No scheme with this write set can detect a metadata-only
event interrupted before its first evidence write.** D10's old universal
"any interruption leaves missing or hash-stale authority" and D15's "never
silently-current" wording are qualified accordingly. Nonzero error propagation
and no success status are mandatory; the retained old coherent pair cannot
attest the failed event. This proposal intentionally does not add an
in-progress marker to make that stronger history claim.

A rerun of a real producer obtains a fresh immutable observation, recomputes
E and C and republishes E then C without duplicate generation entries.
No-event commands do not repair a mixed pair. P2's category-(c) is a repair
path only when its non-empty/same-latest-patch preconditions actually hold.
After a crash, a reader never completes C from orphan E, restores E from C,
deletes evidence, or repairs provenance. D16 alone governs any producer-side
provenance repair.

## 6. S5 read integration and diagnostics

### 6.1 Immutable read pipeline

Extend `verify_landed.go`'s one immutable inventory with C, E and marker
snapshots alongside existing patch/recipe/provenance snapshots
(`inventoryEntry` / `buildInventory`, currently lines 251-420). Capture each
artifact's exact bytes, genuine absence or real read error; a failed read is
not silently dropped. Shared validators consume only this inventory, never a
second live `ReadFeatureFile` to get a more convenient C/E pair.

Extend the existing final instability restatement
(`inventoryInstability`, currently lines 466-538) to C, E and marker:
bytes, presence and readable/unreadable transitions in either direction.
That restatement is detection only, **not a second source for validation**;
it may invalidate the report but never replace the original observation.
Retain the existing `snapshot-unstable` behavior and target/closure/unrelated
feature distinctions. C/E read failures must reach the coverage classifier,
not bypass the coverage-absent rule through a blanket new inventory error.
If C is absent throughout, an orphan E's unreadability does not turn the
missing-coverage row into a block; actual in-run inventory changes can still
trigger the existing independent instability check.

Use an injectable artifact-read seam shared by inventory, apply and doctor
tests. Inject absence, permission/EIO failures and changing observations;
`chmod` is neither the required mechanism nor sufficient acceptance evidence.
No extra live reread inside a validator, including a hidden reader in dry
remediation. Other existing verification rows, including V10, remain
independently enforced.

### 6.2 One narrow new surface code; six rungs retained

Add **`recipe-coverage-capture-evidence-invalid`**, a binding-layer surface
code with **no schema-reason counterpart**. It covers coverage-present with E
missing, unreadable, malformed, unsupported-version, wrong-owner, mismatched
pair or disagreeing capture/reference/observation facts. The diagnostic names
the companion path and precise condition/read cause; do not report an E
problem as a missing patch, malformed C or a generic generation failure.
There is no new E schema-reason array or per-fault surface enum.

Using only `recipe-coverage-reference-stale` would misdescribe an absent or
malformed companion; using `recipe-coverage-malformed` would claim the wrong
file failed decoding. One aggregate new code is the smallest truthful
vocabulary extension: **seven mapped + twelve unmapped = nineteen surface
codes**, superseding the old eighteen/seven/eleven totals only. The D3 reason
vocabulary and its disjointness are unchanged. Producer publication errors
remain ordinary nonzero command errors, not a twentieth warning code.

The full verify precedence remains six rungs:

| Rung | First applicable condition | Row / severity / exit contribution |
|---|---|---|
| 1 | C exists but unreadable, malformed, unknown-field or otherwise strict-invalid | `recipe-coverage-malformed`; failed row, `block`, verdict failed / exit 2 |
| 2 | C decodes but any binding fails, **including required E** | matching binding code; failed row, `block`, verdict failed / exit 2 |
| 3 | C and required E bind; C is valid incomplete, including §4.2's explicitly limited incomplete case | `recipe-coverage-incomplete`; failed row, `warn`, does not fail verdict / exit 0 from this row; exact sorted reasons and reconstruction limitations |
| 4 | Valid complete pair and bindings, live stale marker present | `recipe-coverage-stale-marker`; failed row, `warn`, exit 0 from this row; ineligible |
| 5 | C genuinely absent, **regardless of orphan E or marker** | `recipe-coverage-missing`; failed row, `warn`, exit 0 from this row; ineligible |
| 6 | Valid complete pair and all independently reconstructed bindings, no marker | passing row of severity `block`; no verdict/exit contribution |

Rung 2 diagnostic first-match order: C envelope owner; actual patch
presence/hash; actual recipe presence/hash/strict-decodability binding;
E availability/strict decoding/owner; E-to-C descriptor and observation
comparison (including E's bound patch/recipe flag/hash vector against both
C and inventory); E's raw-C pairing hash; independently reconstructed reference/
effect bindings. Retain existing owner/patch/recipe/reference codes for the
conditions they already name; E-related failures use the new code.
Within E's comparisons use patch/recipe flags and hashes, capture
mode/pathspecs/claim IDs, reference, observations, event-derived conditions
in that order. Multiple
details may be reported but never change the deterministic primary code.

A stale marker never lowers rungs 1/2; incomplete plus marker remains rung 3
with its reason visible; absent C plus marker/orphan E remains rung 5.
Deleting C can still reduce verify severity but can never grant replay
authority. **No missing-produced versus missing-legacy split is recreated.**
Doctor renders even the new blocking-class finding as warning-only,
`Fixable: false`, no writes/lock/backup/normalization under `--fix`, and no
failure exit contributed by D10.

### 6.3 Apply placement and behavior

Use the shared read-only D9/D17 preflight at order 1 **before `LoadRecipe` or
any progress/state/recipe/worktree mutation** on the non-reapplying execute
fallthrough. C-present/E-invalid refuses by the new code, exit **2**, no
operations or writes. Rungs 1/2 outrank withheld or undecodable recipe
orders 2/3, which retain `recipe-generation-incomplete`.

D17 remains seven cases: order 1 is qualified by E; orders 2/3 retain their
named refusals; order 4 still executes a readable, decodable recipe under
valid incomplete coverage with its non-authority warning and unchanged
ADR-029 safety gates; absent-C orders 5/6 remain the exact legacy behavior
and exits, even with orphan/malformed E; order 7 executes only after complete
bindings validate. A generation ID cannot override any mismatch.

The coordinator has already adjudicated auto placement: the same read-only
preflight also runs **before auto-mode prepare**, preventing preparation
writes before a refusal that execute would have issued. This is normal
placement, not a new selectable policy. Preserve state-selected canonical
reapply (including unapplied-baseline selection) and legacy paths; do not
gate a canonical reapply with a recipe-only refusal. Re-evaluate at explicit
execute's pre-mutation boundary with that invocation's immutable snapshot.
No new CLI mode, speculative `unapply` recommendation or unreachable reapply
instruction is introduced.

### 6.4 Remediation is a producer feasibility proof, never reader repair

D11's dry proof must now demonstrate the **actual named producer command**
can regenerate a complete record **and publish E then C**, not merely that a
pure recipe derivation returns complete. Use a read-only plan from captured
inputs: verify the command's state/capture/ownership preconditions, eligible
event path (not a no-event shortcut), complete derivation, D16 constraints,
strictly encodable E/C pair and applicable publication path preconditions.
Do not recommend P2 merely because coverage needs repair or copy a missing
descriptor from C to get a passing plan. No writes, temporary artifact
publication, locks, provider invocation or provenance synthesis by a reader.

The proof is of feasibility under the observed inputs, not a guarantee
against later I/O failure or concurrent change. If publication feasibility
cannot be established read-only, fail the recommendation conservatively.
The real producer must recheck and propagate every publication error.
Otherwise print `recipe-generation-no-truthful-regeneration`, concrete
blocking reasons and manual-review guidance, not an executable command
known to be unreachable. Apply the same rule to doctor, verify and producer
remediation output. A missing E can be fixed only by an honest new producer
event; readers never manufacture a sidecar from old metadata.

## 7. Migration and GH #13 planning dependency

S4 coverage is **unshipped** at this baseline. Its C-only records remain
strictly decodable D3 records, but once the amended reader ships they lack
required binding evidence: C-present/E-missing is rung 2 / D17 order 1,
not a legacy exception or an automatic backfill. Republish with an actual
governed producer, capturing fresh event evidence. Do not assert an old
capture happened by copying C or a generation into E.

P6/P7 that cannot reconstruct a validated prior pair may honestly republish
an unavailable/incomplete no-capture pair; this does not recover complete
authority. Complete repair needs a real reconstructable capture/derivation.
Repositories with C absent retain uniformly warning verify and unchanged
legacy explicit apply; an orphan E does not prove prior production. There is
no cleanup migration in verify/doctor, and no compatibility flag or
generation-based exemption.

**GH #13 is not amended or implemented here.** Its accepted planning needs a
separate precise follow-up before implementation:

1. ADR-037 D4 / companion §6.5's parity block B fixes E1-E15 with no companion
   check. Add required E validation without silently changing that closed
   ordering or making failure codes unreachable; specify its first-match
   mapping and negative controls. GH #13 must hard-refuse every E failure
   even though doctor/legacy surfaces can warn.
2. D5/D8/D10/D12 and the candidate proofs must consume the independently
   validated event descriptor, never a generation capture or a hash-only
   substitute for offline tree reconstruction. A no-capture complete pair,
   if independently reconstructable, is not rejected merely for its mode.
3. D19/D20/D21, companion §6.10-§6.11 and identity/integrity guards must decide
   explicitly whether to bind E's canonical digest into candidate identity
   or add an equivalent immutable E recheck. The current `coverage_sha256`
   binds C, **not the bytes of E**. Existing candidate metadata cannot be
   described as already binding this new dependency.
4. D29 parity block E steps 4/7/8/11, D29b rollback/recovery, D30/D31 and
   companion §6.14-§6.16 must stage, snapshot (including absence), publish
   E before C, and restore E with the other artifacts. Both P3 variants must
   use the amended shared API; journal/error and candidate-acceptance tests
   need the additional failure boundary. Do not insert E into the current
   fourteen-step transaction without that planning review.
5. Update GH #13 parity blocks, refusal counts, acceptance/identity/recovery
   matrices and its prerequisites together. Its no-autorepair rule remains.

That follow-up is a downstream planning dependency, not authority to implement
a replay candidate in S5. ADR-039's domain, ADR-040's reasons and D16 remain
unchanged; GH #24 and S6/release stay outside this amendment.

## 8. Supplementary acceptance plan

These **42 accepted cases** are separate from the accepted rev-7
`RGA-001`–`RGA-360` matrix. Do not renumber, rewrite or recount its 360 rows.
Current producer/consumer fixtures gain honest E inputs where required; the
historical matrix remains the accepted baseline with §9's explicit
qualifications. `ICE-*` IDs below are unique to this amendment.

Every negative control must pass the deliberately wrong input through the
**actual validator/finalizer/classifier** used by the positive case. A token
search, helper-only assertion or mutated expected string is not sensitivity.
Publication tests inject each writer boundary; read tests inject failures
through the artifact-read seam, not OS permissions.

| ID | Case and required observable |
|---|---|
| ICE-001 | All P1-P7 event variants, complete and incomplete where reachable, publish exactly one E then final C through the shared API; skip-E and C-first mutations fail the same ordering guard |
| ICE-002 | P1 same patch/recipe bytes with two legitimate captures (e.g. staged and working-tree) republishes distinct bound E/C without generation change; latest-generation-equality mutation fails the real reader |
| ICE-003 | P2 refresh and fixup category-(c) repair missing/stale pairs; exactly E/C can change; recipe/provenance/generation/marker/state/numbered-patch bytes and absence remain fixed |
| ICE-004 | P2 formatting-only and actual semantic drift preserve ADR-040's respective reason sets and D16 restriction; fake rewrite-event fact fails real recomputation when it changes applicable reasons |
| ICE-005 | P3 same-byte accept with no generation append still publishes pair; P4's two events publish two successive pairs; P5 write path publishes once |
| ICE-006 | P6 both parse arms and manual checkpoint publish no-capture pair beside older generation; undecodable raw recipe retains raw hash and exact incomplete reasons |
| ICE-007 | P7 changed canonical file, explicit artifacts path, decoy/unrelated/no-change/unset-editor paths obey the same resolved-path boundary for both outputs |
| ICE-008 | P6/P7 no-capture unavailable and independently reconstructed commit cases remain distinct; copying an old S4 reference without a valid prior pair cannot establish commit authority |
| ICE-009 | Identical event inputs yield byte-identical E/C and identity; changed capture changes identity; clocks, nonce, map iteration and provenance time cannot affect E/C |
| ICE-010 | C-only mode, pathspec or claim-ID edit (separately) refuses at rung 2/order 1; E-only equivalent edits also refuse; matching generation does not rescue either |
| ICE-011 | C-only/E-only reference kind, commit and preimage digest edits (separately), including same-tree alternate commit, refuse rather than silently switching bases |
| ICE-012 | E coverage-pair hash tamper and C raw whitespace-only edit refuse; comparing a reserialized rather than raw C hash is rejected by the actual pair validator |
| ICE-013 | Wrong requested slug, C owner alone, E owner alone and whole E/C pair swapped between two otherwise byte-identical features all refuse with deterministic primary code |
| ICE-014 | C present/E absent, unreadable, zero-byte, malformed, unsupported version, duplicate/unknown/missing/null field each fail closed with new code and actual path/cause |
| ICE-015 | C unreadable/malformed with E good or bad remains rung 1/order 1; E cannot turn unusable C into absent/legacy |
| ICE-016 | C absent with E absent, valid orphan, unreadable orphan or malformed orphan remains rung 5 and both legacy D17 rows; deleting C never authorizes replay |
| ICE-017 | For patch and recipe independently: readable→absent, readable→unreadable, absent→readable and unreadable→readable fail binding from both flags/hashes; truthful unreadable stays false and diagnostic retains cause |
| ICE-018 | E unavailable→readable and readable→unavailable during inventory, plus C/marker changes, trigger snapshot instability; validators never switch to the later bytes |
| ICE-019 | Side-flag/mode/hash/ordinal/fragment/axis tamper fails projection or independent content checks; dropping the observation-ceiling check cannot promote an S3 unobserved side |
| ICE-020 | Truthful unavailable/retention-limited incomplete pair retains exact reasons and limited-proof diagnostic; no source body is manufactured to call S3; supported complete fixture still requires actual tree/byte validation |
| ICE-021 | Missing local commit, changed reconstructed preimage or wrong exact postimage refuses even if E/C hashes match; hash-only/fake-body inputs fail the same complete validator |
| ICE-022 | C/E `index-snapshot`, forbidden no-capture selectors, impossible observed-extant absence and unsupported complete-operation shapes refuse or remain non-durable/incomplete exactly as contracted; no ADR-039 widening |
| ICE-023 | Failure before/after each earlier bound/recipe/provenance/generation boundary leaves only allowed outputs; same-byte failure before E may retain previous coherent pair but cannot report new completion |
| ICE-024 | E atomic write and rename failures prevent C publication and exit nonzero for all producers; old file or absence, never partial JSON |
| ICE-025 | Crash after new E before C, and injected C rename failure, detect different mixed pair; C absent remains missing warning/legacy; rerun restores pair without duplicate generation |
| ICE-026 | P6 state failure still attempts E/C; state plus E/C failure chains both in fixed order, returns nonzero, emits no success status |
| ICE-027 | P7 editor failure with changed bytes still snapshots and attempts E/C first; editor plus publication failures retain both; no bytes changed owes no pair |
| ICE-028 | All no-event branches leave E/C byte/absence state untouched, including deliberately broken existing pairs; cycle's earlier P6 pair is not erased |
| ICE-029 | Full six-rung verify table, marker with malformed/mixed/incomplete/complete/absent C, and independent V10/supersession controls preserve exact precedence and exits |
| ICE-030 | All seven D17 cases with E requirement; order-1 refusal precedes LoadRecipe and every mutation; incomplete executable recipe still warns/executes and absent-C legacy behavior remains exact |
| ICE-031 | Auto read-only preflight refuses before prepare writes; explicit execute preflight still runs; state-selected reapply and legacy positive controls remain unchanged |
| ICE-032 | Doctor including `--fix` reports all E faults as warning-only Fixable:false; operation trace proves no writes, backups, normalization, lock or hidden live reread |
| ICE-033 | Dry remediation passes only for a real reachable producer capable of complete derivation and E→C publication; no-event path, unvalidated prior descriptor, blocked path or incomplete result suppresses command |
| ICE-034 | Missing E never causes reader backfill; dry-proof execution leaves all artifacts unchanged; an actual later producer invocation creates the pair from a new observation |
| ICE-035 | Vocabulary validator accepts exactly seven mapped/twelve unmapped disjoint surface codes; missing-new-code, invented schema reason, old eleven-count and warning-demotion mutations fail |
| ICE-036 | Coordinated consistent E/C/artifact edits and full-set rollback are explicitly not authentication tests; privacy/no-source-body and unchanged D16 raw-byte near-match controls prevent an origin claim from a valid pair |
| ICE-037 | Actual reader accepts a producer-created durable-reference binary-stub pair with both sides observed as limited-proof incomplete; preserves flags/hashes/reasons, validates the preimage, reports unavailable postimage payload and stays rung-3 warning rather than binding-stale |
| ICE-038 | Actual reader on mixed reconstructable text plus observed binary stub validates the text exactly and limits only the omitted binary payload; text-byte/fragment/preimage tamper still refuses instead of gaining a whole-record incomplete exemption |
| ICE-039 | Same-reader controls reject missing required reference objects, malformed/apply-failing reconstructable text and complete-authority promotion; mutations treating a binary stub as a lost object, inventing a body, or clearing observed flags fail the positive binary/mixed controls |
| ICE-040 | Actual reader reproduces the accepted gated-addition parent-exclusion cohort with nonempty captured parent set and no final recipe created_by; remains incomplete with parent-created-target-unsupported, with equivalent lexical input spellings normalized identically |
| ICE-041 | Omitting the effective exclusion from E while retaining C, or removing C's parent reason/claiming complete while retaining E's set (even if its C pairing hash is refreshed), fails the actual independent-input validator; inferring exclusions from C's reasons fails the same control |
| ICE-042 | E parent_created_paths is required/non-null, sorted, unique and root-relative; null/missing/duplicate/escaping/absolute/noncanonical wire mutations refuse, and current-parent mutation cannot silently replace the captured exclusion set |

Implementation must extend actual publication and
source-derived site-mapping guards to E, preserve all existing eleven
phase-boundary mutations plus S4 alias/import/shadow controls, and add
mutation-sensitive E schema/pair/inventory/order/vocabulary controls. Frozen
source/golden expectations receive separately documented current deltas,
not edited historical evidence. Runtime validation and independent review
are subsequent work; no Go command or test result is claimed by this ADR.

## 9. Exact supersession and preservation list

These are **accepted qualifications**. The
primary ADR/PRD addenda point here rather than making their historical text
silently universal.

| Existing statement/surface | Accepted qualification |
|---|---|
| ADR-036 D2; PRD §6.2: immutable capture used for coverage | Same input also constructs E; P7 retains its before/after exception; new commit carry-forward requires a valid prior pair plus independent reconstruction (§4) |
| D9; PRD §6.14: reference/capture independently recomputed without a persisted event carrier | E is required independent capture/reference/event/parent-exclusion consistency input; generation never substitutes. Actual content/tree proofs remain mandatory for complete authority; unavailable historical sides and inherently omitted unsupported payloads in truthful incomplete records have only explicitly limited consistency validation (§4.2), without changing publication-time observation flags |
| D10 and D15; PRD §6.10/§6.15 and S4: coverage-only finalization and publication failures | E atomically precedes C last on every existing event; both publication failures are nonzero; no success status after combined failure (§5) |
| D10/D15 universal crash/hash-stale and rerun-repairs wording; RGA-048, RGA-307–309 | Mixed differing pair after E is detectable, but an interrupted identical-byte event before first E write can retain the previous coherent pair; absent C still warns. Only a real event can repair (§5.3) |
| D15 P2 category-(c); PRD §6.15; RGA-013/014 and “every other artifact untouched” statements | “Coverage only” becomes exactly **E + C only**, with all other no-write guarantees and semantic reasons preserved. ADR-040's checkpoint-only phrase is qualified solely in this write-set respect, not its accepted reason policy (§5.2) |
| D13; PRD §6.12/§7: seven mapped/eleven unmapped/eighteen total, old binding ladder | Seven mapped/**twelve** unmapped/**nineteen** total; required E failure joins binding rung 2 with one new code. Six rungs, marker precedence, absence warning and schema vocabulary unchanged (§6.2) |
| D17; PRD §6.11 and S5: coverage-present “valid” cases | Valid now includes E; order 1 refuses E failures before LoadRecipe/mutation, also before auto prepare on its ordinary non-reapply route. Seven cases and state-aware/legacy behavior retained (§6.3) |
| D11; PRD §6.11/§6.13 and S5: complete dry derivation sufficient for command recommendation | Prove actual named producer event and E/C publication feasibility as well; no reader repair. Doctor stays warning-only/read-only (§6.4) |
| PRD §9 accepted matrix and counts | Keep all 360 rows verbatim as historical baseline. Add ICE-001–042 separately; add pair prerequisites to runtime fixtures, not to historical row text. In particular publication rows RGA-003–062/303–313, binding RGA-220/319, vocabulary RGA-331/334–336 and remediation RGA-341–345 now need this qualification |
| ADR-036 D12 and PRD §6.14/§13 downstream assumptions | GH #13 needs its own accepted planning follow-up for ordered gates, identity and journal publication/recovery dependencies (§7); no change to its code or planning documents here |

**Unchanged:** D3's exact schema/reason sets and byte-identical canonical
ten-predicate blocks; strict S3 construction/validation; ADR-039's complete
domain; ADR-040's semantic rewrite conditions; D16 raw canonical-byte origin
proof; D7/D14 and ADR-029 safety/accounting/severity; seven producer identities
and existing event triggers; generation append policy; no persisted source
bodies; C-absent verify/legacy apply behavior; no replay authority from a
warning, no GH #13 execution and no GH #24 widening.

Independent rev-1 review approves the new wire artifact and vocabulary,
bounded incomplete-observation meaning (§4.2), honest pre-E crash limit
(§5.3) and migration (§7). These are accepted decisions, not claims that the
old text or implementation already supplied them. Runtime validation and
independent implementation review remain required.
