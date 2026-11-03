# ADR-051 — Operation-candidate acceptance and explicit recovery

- **Status**: Proposed
- **Date**: 2026-11-03
- **Scope**: GH #13, slice S8
- **Policy authorization**: operator approved `candidate-only-generation-amendment`
  on 2026-11-03; implementation validation and independent review remain pending.

## Context and authority

The accepted [operation-replay PRD](../prds/PRD-reconcile-operation-replay-candidate.md)
§6.14, parity block E, fixes fourteen ordered acceptance steps. §6.15 fixes
explicit, marker-driven recovery; §6.16 fixes the candidate-accept variant of
the existing P3 publication boundary. This ADR records implementation choices
within those contracts. It does not amend their steps, eligibility gates,
producer vocabulary, exit classes, or public documentation.

Related accepted decisions:

- [ADR-037](ADR-037-reconcile-operation-replay-candidate-authority.md):
  candidate authority and the transaction/recovery contract.
- [ADR-043](ADR-043-operation-candidate-capture-evidence.md):
  canonical E identity, with raw C/patch/recipe bindings.
- [ADR-044](ADR-044-candidate-caller-held-lock-capabilities.md):
  caller-owned kernel locks and borrowing.
- [ADR-048](ADR-048-candidate-artifact-lifecycle-authority.md):
  immutable candidate metadata and separate mutable lifecycle state.
- [ADR-050](ADR-050-cli-terminal-candidate-actions.md):
  five terminal flags, exclusivity, resolver separation, and exit envelope.
- [ADR-024](ADR-024-patch-generation-manifest-boundary.md),
  [ADR-036](ADR-036-recipe-coverage-authority.md),
  [ADR-040](ADR-040-p2-publication-reason-semantics.md), and
  [ADR-041](ADR-041-independent-capture-event-evidence.md):
  generation history and the single paired-publication boundary.

## Decisions

### D1 — Continuous caller-owned holds, with borrowed implementation helpers

Acceptance and explicit recovery acquire feature then global holds and release
them in reverse order. A transaction retains borrows through publication,
rollback, pruning, and cleanup. Rematerialization receives those same handles;
it does not call the ordinary executor, reacquire global ownership, or
temporarily release either hold.

Authority is rebuilt from current E/C and immutable commits, not stored proof
hashes. Acceptance validates the existing candidate, rematerializes and proves
the whole operation set, runs the configured test command, then validates the
candidate and bound inputs again. This last check catches a test command that
changes candidate bytes or evidence. No derivation memo's lifetime is widened.

Sources: `internal/workflow/operation_candidate_acceptance.go:22`,
`internal/workflow/operation_candidate_transaction.go:138`,
`internal/candidate/acceptance.go:18`.

**Alternative rejected:** calling the existing ordinary executor/preparation
path. Its lock-acquisition/lifecycle behavior is inappropriate while both
acceptance holds are already owned.

### D2 — A closed write registry, not journal-supplied write authority

The transaction derives live targets from freshly replayed operations. Its
closed feature-file registry is exactly:

1. `artifacts/post-apply.patch`
2. `artifacts/apply-recipe.json`
3. `artifacts/recipe-provenance.json`
4. `artifacts/patch-generations.json`
5. `artifacts/recipe-capture-event.json`
6. `artifacts/recipe-coverage.json`
7. `status.json`

The registry drives preparation, snapshot validation, publication completeness,
and restoration. Recovery recomputes the candidate identity and derives the
same path set from immutable candidate metadata. Journal target fields must
match that set exactly; rollback selects actual targets from the derived set,
not from a journal path. Missing creation ancestors are separately restricted
to proper ancestors of registered absent live targets.

Filesystem operations use a held repository `os.Root`, reject nonregular
targets and symlinked ancestors, and recheck held-directory identity. An atomic
replacement uses the fixed candidate-local `.accept-write` reservation rather
than creating unregistered sidecar files beside live targets. A write is
permitted only for its registered step, or a prepared no-marker restoration.

`SaveFeatureStatus` is deliberately not used inside this transaction: its
best-effort `FEATURES.md` refresh would add a write outside the seven-file
inventory. The typed status is encoded directly, preserving the snapshot's
unrelated status fields. S8 does not update `FEATURES.md` as a hidden eighth
transaction artifact.

Sources: `internal/workflow/operation_candidate_transaction.go:44`,
`internal/workflow/operation_candidate_transaction.go:164`,
`internal/workflow/operation_candidate_transaction.go:255`,
`internal/workflow/operation_candidate_recovery.go:283`,
`internal/workflow/operation_candidate_acceptance.go:360`.

### D3 — Stage through the existing publisher and generation appender

The residual is reconstructed from immutable upstream to the freshly validated
candidate, restricted to candidate-operation paths. Absorbed effects are not
reintroduced. The observation has P3 `reconcile-accept`, capture mode
`reconcile`, the candidate paths, and no fabricated parent claims/exclusions.
Recipe generation and provenance use the existing derivation/provenance
implementation.

Two private inputs extend, rather than duplicate, existing APIs:

- `CoveragePublicationInput.candidateAcceptance` makes `PublishCoverage`
  route its ordinary encoded artifacts through the transaction's staged or
  journaled writer.
- `PatchGenerationInput.candidateStage` supplies the confined prior manifest
  observation and newly derived recipe bytes to
  `AppendPatchGenerationForFeature`, retaining its result for staging rather
  than prematurely replacing the canonical manifest.

There is no eighth producer, new public schema, private E/C encoder, or change
to resolver acceptance/P4–P7. Step 7 stages all six artifacts; step 11 verifies
the staged bytes and publishes inputs, E, then C last. Step 12 checks the
published bytes and agreement among status base, C reference, latest generation
base/recipe/patch/capture, and the reconcile-result upper descriptor before
the commit marker is installed.

Sources: `internal/workflow/operation_candidate_publication.go:15`,
`internal/workflow/operation_candidate_publication.go:85`,
`internal/workflow/operation_candidate_publication.go:139`,
`internal/workflow/recipe_coverage_publish.go:216`,
`internal/workflow/patch_generations.go:44`.

**Operator-approved narrow exception:** on 2026-11-03 the operator approved
`candidate-only-generation-amendment`, now noted in ADR-024 D9. This is a
reachable normal flow, not merely malformed metadata: set `core.abbrev=40`,
record an insertion against B, create descendant U changing another file,
create descendant H deleting the insertion context, reconcile U while HEAD is
H, and restore the live candidate path to U. The phase-4 conflict produces a
candidate whose full residual bytes equal the recorded patch, although its
required base/capture/upper bindings differ. An untyped preparation refusal
for this otherwise eligible input is not the accepted S8 behavior.

Only the private `candidateStage`, with kind `reconcile` and no alternate
intent, may append for this same-patch case. It compares base commit, raw
recipe digest, all three upper fields, capture mode, and both sorted selector
arrays (`pathspecs`, `claim_ids`); nil/empty arrays compare equally. A matching
tuple retains the prior manifest verbatim and appends nothing. Every ordinary
producer keeps its existing skip/classification behavior.

The shared store appender still creates the new row and generation ID.
For this exception alone, its ordinary prior-`canonical_patch` demotion is
undone: prior rows, including their raw JSON formatting, are preserved
byte-for-byte. This is truthful because the canonical bytes did not change.
[PRD-feature-patch-identity-metadata §4.1](../prds/PRD-feature-patch-identity-metadata.md)
explicitly permits historical canonical pointers while those bytes still
match the generation. No historical generation is reidentified or edited, and
there is no new kind, producer, schema, or public writer.

Sources: `internal/workflow/patch_generations.go`: `AppendPatchGenerationForFeature`,
`operationCandidateGenerationBindingsMatch`, `encodeOperationCandidateGenerationManifest`;
`internal/store/patch_generations.go`: `AppendPatchGeneration`;
`internal/cli/operation_candidate_s8_test.go`:
`TestROCS8CLIRealRecordSameByteGenerationRebasesAcceptance`;
`internal/workflow/operation_candidate_acceptance_test.go`:
`TestROCS8GenerationBindingTupleIsCandidateOnly`.

### D4 — A self-contained, strict journal

Version 1 of `accept-journal.json` contains:

| Field | Meaning |
|---|---|
| `version`, `feature`, `candidate_id` | Closed schema and namespace identity |
| `metadata_sha256` | Integrity binding to canonical immutable candidate metadata |
| `prepared_steps` | The ordered integers 1 through 14 |
| `snapshots` | Ordered live images followed by the seven registered feature images |
| `missing_directories` | Sorted, unique absent creation ancestors |
| `pending_state`, `pending_state_mode` | Exact original pending-state JSON bytes and mode |
| `apply_before`, `state_before` | Status values cross-checked against the status image |
| `commit_marker` | Empty before commitment; exactly `committed` afterward |
| `prune_ownership` | Original worktree/registration native identities and exact Git registration binding; null grants no committed-cleanup authority |

Each snapshot has `scope`, `path`, `present`, `mode`, `data`, and `sha256`.
JSON base64 encoding of `data` preserves arbitrary bytes, including malformed
old provenance/evidence. Absence is explicit: `present: false`, mode zero,
empty non-null data, and the empty-byte digest. The decoder rejects duplicate,
unknown, missing, null-for-typed, and trailing members; object-key order and
JSON whitespace are not authority. Array order and the full target inventory
are strict.

The same images are written individually under `snapshots/` and embedded in
the journal. File sync precedes atomic installation; parent-directory sync is
required. The complete inline copy is intentional redundancy: a crash after
snapshot-directory cleanup must not strand the remaining journal without
rollback data.

Step 8 captures cleanup ownership from the **still-held rematerialized root**,
not a newly observed path. It is included in the initial durable step-9 journal
before live writes or the step-13 marker. Marked recovery requires it; no-marker
restoration does not acquire pruning authority from an absent value. Recovery
never captures replacement ownership from whatever directories happen to exist.

Sources: `internal/workflow/operation_candidate_transaction.go:66`,
`internal/workflow/operation_candidate_transaction.go:354`,
`internal/workflow/operation_candidate_transaction.go`:
`prepareJournal`, `validateJournal`, `decodeOperationCandidateJournal`.

**Alternative rejected:** a journal containing only snapshot filenames.
Removing those snapshots before the journal creates an unrecoverable crash
gap. Removing the journal first instead loses recovery evidence while
transaction data and pointer cleanup remain unfinished.

### D5 — The marker, not current E/C or accepted state, selects recovery

A no-marker failure restores every live/feature image or absence, original
pending-state bytes/mode, Apply/State, and pointer. A completed rollback returns
the prescribed exit-3 class with the failing step; a failed rollback keeps the
journal and returns exit 1. Preparation failures before journal installation
are described honestly as preparation failures, not completed rollbacks.

Step 10's leading CAS refusal is special: it has made no live write, so it
returns exit 2 with the no-marker journal intact and does not erase the foreign
edit. Explicit recovery subsequently authorizes restoration of the recorded
preimage/absence.

Explicit recovery validates journal ownership and target confinement without
requiring old E/C validity or presence. No-marker recovery restores; marked
recovery only completes cleanup. A marker installed before a failed sync
acknowledgement never selects eager rollback.

Every ordinary reconcile checks for recovery evidence before planning/evidence
mutation. The CLI calls the same preflight immediately after opening the store,
before dirty-tree refusal, `--preflight`, `--check-applied-only`, rejected-slug
filtering, terminal dispatch, or provider probing. Only explicit
`--resume-candidate-recovery` bypasses that preflight. A pending candidate
without a journal retains S7's read-only flag behavior. The workflow retains its
own preflight for direct callers.

The early shared probe is strictly read-only, including when the candidate
namespace or journal is absent. It uses the confined presence scan without
acquiring a feature lock: `TryAcquire` creates persistent lock directories/files,
which would itself dirty a previously clean tree before the CLI's dirty check.
Presence or an unsafe/unreadable namespace refuses; the probe neither repairs
evidence nor grants mutation authority. Actions and acceptance still recheck
under their existing caller-owned holds. No ignore rule or lock-file deletion
is used to hide this side effect.

The resolver-only coexistence preflight uses this same read-only scan, not the
candidate-action helper that acquires a feature hold. Otherwise a legacy
`reconcile --accept` creates a candidate lock even when no candidate exists,
leaving an out-of-scope path that correctly blocks subsequent `land`. Actual
candidate actions retain their authoritative held-lock checks; neither land's
scope policy nor any producer's publication behavior is changed.

A malformed or nonregular journal is still a blocker. Default-slug
selection cannot hide a journal behind missing status; an occupied feature
lock cannot hide one behind the normal nonterminal lock-refusal envelope.
Ordinary commands do not call the recovery implementation.

Sources: `internal/workflow/operation_candidate_acceptance.go:271`,
`internal/workflow/operation_candidate_recovery.go`: `refuseReconcileCandidateRecovery`,
`RefuseReconcileCandidateRecovery`, `resumeOperationCandidateRecovery`,
`preparationFailed`, `failed`, `rollback`;
`internal/cli/cobra.go`: `reconcileCmd`.

### D6 — Cleanup has durable ownership even after partial worktree deletion

The original worktree and exact `.git/worktrees/<entry>` directory identities
are captured at step 8 and fsynced in the initial journal, **before** the commit
marker. Linux/macOS use native device/inode identity; Windows uses
`GetFileInformationByHandle`'s volume serial and 64-bit file index. Platform and
fixed-width hexadecimal values are journaled; mutable directory timestamps
are not identity. The registration's name, raw detached HEAD, backlink and
common-directory binding are also captured and validated against immutable
candidate identity.

Committed cleanup checks those original identities before renaming the worktree
to fixed `.accept-prune` and before deleting either directory. The reserved name
must have been absent before preparation, but name/type alone is never ownership.
The rematerialized root is closed before deletion, retaining both kernel locks.
The exact captured registration is removed through a confined rooted operation,
not a path/head-based Git selection or global `git worktree prune`. A surviving
registration control file must match its captured binding; missing control files
after partial deletion are safe only under the original directory identity.
The held registry directory is synced using the same platform durability helper.
Any replacement registration for the original worktree remains a refusal.

The two recursive removals are confined to the fixed reserved worktree leaf and
the identity-bound single registry entry. An ordinary directory replacement,
symlink, simultaneous old/new directories, or ambiguous ownership refuses and
retains the journal. Partial deletion can remove the worktree's `.git` or Git
registration control files without invalidating the original native directory
identity. Snapshot/staged cleanup removes only the registered
regular filenames; unknown entries are not swept away. The journal is removed
last, after the pointer and all transaction data. If unlink succeeds but its
directory sync fails, cleanup attempts to reinstall the self-contained
journal before reporting failure.

Known, unjournaled preparation residue can be discarded on a fully revalidated
pending retry, because it predates every live write. A reserved prune subtree
without a journal is not treated as ordinary preparation residue.

Sources: `internal/workflow/operation_candidate_recovery.go`:
`requireFreshPreparation`, `cleanupCommitted`, `relocatePruneWorktree`,
`removePrunedWorktree`, `cleanupTransactionData`, `removeKnownDirectory`;
`internal/workflow/operation_candidate_transaction.go`: `snapshot`, `validateJournal`;
`internal/candidate/acceptance.go`: `CaptureAcceptanceOwnership`,
`AcceptanceOwnership.Validate`, `AcceptanceDirectoryIdentity.CheckRoot`,
`PruneAcceptanceWorktree`;
`internal/candidate/acceptance_identity_{unix,windows,other}.go`.

**Alternative rejected:** asking Git to recursively delete the only worktree
directory and later inferring ownership from whatever survived. A crash can
remove its `.git` file/registration before deleting the directory, making such
an inference ambiguous.

**Alternative rejected after rev-0 review:** checking that `.accept-prune` is
an ordinary directory and comparing two fresh observations with `os.SameFile`.
Moving the original aside and replacing it with a foreign ordinary directory
passes that check. Persisting identities only after the commit marker would
also leave a pre-relocation crash window in which recovery could mistakenly
adopt a replacement; the initial step-9 journal closes that window.

## Boundaries and verification

- This is recoverable multi-file publication, not filesystem-wide atomicity.
  An E/C mismatch during a crash is blocked by the journal until explicit
  recovery. No valid old pair is needed for rollback.
- The last CAS cannot exclude a noncooperating editor after that check; the
  accepted PRD explicitly relies on recovery rather than cross-host prevention.
- The main worktree's refs, reflog, and index are not acceptance targets.
  Detached candidate worktree registration/index management is still necessary.
  No live textual patch apply, shadow acceptance/copy, or refresh is delegated.
- Local filesystem rename and directory-durability failures are errors, never
  silently downgraded success. POSIX uses directory `Sync`; Windows opens an
  identity-checked directory handle with `CreateFile`,
  `GENERIC_READ | GENERIC_WRITE`, read/write/delete sharing,
  `FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT`, then calls
  `FlushFileBuffers`.
  Native Windows durability behavior requires validation; cross-compilation
  alone is not evidence that the filesystem accepts those operations.
- The journal is not a cryptographic defense against an operator rewriting all
  owned recovery evidence. It does not authorize arbitrary paths or repair an
  ambiguous namespace.

Focused tests are in `operation_candidate_acceptance_test.go`,
`operation_candidate_recovery_test.go`, and CLI `operation_candidate_s8_test.go`.
They exercise real acceptance, per-path CAS, exact rollback, child-process
termination without defers, independent E/C absence, cleanup gaps, explicit
terminal exits, residual re-eligibility, and all six shipped base readers.
New recovery cases replace ordinary worktree directories before and after
relocation, replace a Git registration with byte-identical control files, and
resume partial registration/worktree deletion. A native identity round-trip
test closes and reopens handles, serializes the capability, deletes children,
and proves that the same validator rejects a replacement directory.
`TestROCS8NativeDirectoryDurability` tests the real native flush;
`TestROCS8WindowsDirectoryFlushRejectsReplacedPath` exercises its Windows
path-reopen identity check. Windows may prevent renaming the directory while
`os.Root` is open. The native test distinguishes that sharing barrier from
a flush failure, verifies rename succeeds after releasing it, and then passes
the retained original identity and a real replacement directory through the
same production reopen/check/flush helper. A matching replacement identity
must still flush successfully. Thus rejection is not inferred from a failed
fixture setup or a closed-root error. Lock-evidence comparisons likewise read
complete images before acquisition and after release, never through an active
Windows byte-range lock; contention checks prove both holds remain active
through the recheck. These tests are not claims of native validation.

On 2026-11-03 the operator approved a narrowly additive blocking S8 Windows CI
lane. It runs `^TestROCS8` verbosely and uncached in `internal/candidate`,
`internal/workflow`, and `internal/cli`, including persisted native identity,
positive directory flushing and retained-identity rejection. Unfiltered output
is retained under ignored `bin/`; conditional symlink SKIPs remain visible and
are not claimed covered. Exact PASS lines are mandatory for the named native
and acceptance/recovery controls. The existing CI partitions, timeouts, GH #17
allowed-failure baseline, and local wave-close harness remain unchanged.
`TestROCS8WindowsBlockingRuntimeCIGuard` rejects bypasses and missing runtime
proof through mutations of the actual workflow; the companion matcher test
uses the workflow's own PASS expression. Fresh native execution and independent
review are still required.

Source guards examine actual transaction calls/write sinks and include
same-validator broken fixtures for ordering, bindings, legacy delegation,
unregistered writers, recursive-prune confinement, ownership persistence,
fresh-ownership minting during recovery, and lock/ref mutation. The S0 coverage
dispatches reuse the existing constant-expression resolver, with concatenation
and constant-alias negatives through each of the four S8 dispatches.
The existing S4 completion-reporter guard and its ordinary producer set stay
unchanged. S8 instead has its fail-closed transaction guard and one added caller
in the existing P3 graph; removing that caller is a negative fixture through the
same shared mapping validator.

The rev-0 Git-invariant assertion used `reflog --all --single-worktree`.
Git expands `--all` when it is encountered, before that later switch; the result
included disposable candidate HEAD logs. Rematerialization/pruning necessarily
changes those logs. The revised check directly compares live HEAD/index,
shared refs, packed refs and raw main/shared reflog bytes/modes, retaining
per-file diagnostics and real index/ref/reflog mutation controls. This strengthens
the live invariant rather than treating candidate HEAD disposal as a live
history mutation. Source: Git `revision.c`, `handle_revision_pseudo_opt`
and `setup_revisions`; local `rocS8GitIdentity` and
`TestROCS8LiveGitIdentityDetectsIndexRefsAndReflogChanges`.

Validation and acceptance of this Proposed ADR belong to the supervisor's
guarded S8 review; no validation result is asserted here.
