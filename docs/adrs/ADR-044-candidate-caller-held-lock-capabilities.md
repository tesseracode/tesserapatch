# ADR-044 — Caller-held lock capabilities for candidate worktrees

**Status**: Proposed — narrow S1 amendment; implementation waits for review
**Date**: 2026-10-29
**Owner**: Core
**Issue**: GH #13
**Amends**: ADR-037 D12/D28 and PRD-reconcile-operation-replay-candidate
sections 6.9/6.13 and S1's primitive API, not transaction semantics
**Related**:
[ADR-037](ADR-037-reconcile-operation-replay-candidate-authority.md),
[companion PRD](../prds/PRD-reconcile-operation-replay-candidate.md),
[ADR-034](ADR-034-rooted-filesystem-inspection-boundary.md)

## Problem

ADR-037 D28 / PRD section 6.13 require a per-feature lock throughout candidate
finalization and both feature/global locks throughout acceptance. Creation and
pruning must therefore compose with locks the caller already holds.

The first S1 implementation instead acquires both locks inside creation/pruning
and releases them before returning. A correctly locked caller contends with
itself, because two independently opened handles do not provide reentrancy.
Releasing the caller's lock merely to enter a primitive would violate the
transaction scope and invalidate the caller's revalidation.

Independent review identified this mismatch at unaccepted checkpoint
`f5df7852a39a476808a9146f65970a809b7ea44c`, particularly
`internal/candidate/root.go:122-189,309-364`. The locked lifecycle is already
explicit in `docs/prds/PRD-reconcile-operation-replay-candidate.md:2171-2184`.
The operator authorized a narrow lock-capability/API amendment before further
implementation. This ADR proposes that amendment; it does not authorize S2+
behavior or a change in repository branching strategy.

## Existing primitives and alternatives

The S1 `candidatelock.Handle` already represents an acquired kernel lock through
unexported descriptor state. Reuse that capability rather than add a serialized
ownership object or a second lock registry. A path string, PID, boolean, token
label or stored candidate metadata cannot prove a live lock hold.

| Alternative | Disposition |
|---|---|
| Reacquire inside create/prune | Reject: self-contention and inability to preserve the broader hold. |
| Temporarily release the caller's locks | Reject: breaks the accepted transaction's scope/revalidation. |
| Make locks reentrant through ambient process state | Reject: hides ownership, complicates independent callers, and changes the accepted nonblocking lock contract. |
| Create an unlocked public mutation helper | Reject: makes correct locking a caller convention rather than a checked precondition. |
| Pass the existing live lock handles explicitly | Proposed: preserves kernel exclusion and makes ownership/borrowing inspectable at each mutation boundary. |

## D1 — The caller owns acquisition, order and release

The orchestration layer acquires the per-feature lock first and the global
worktree-mutation lock second, using the existing nonblocking `TryAcquire`.
If the second acquisition fails, it releases the first before returning.

The handles remain caller-owned. Worktree primitives borrow them only for the
duration of the operation: they do not reacquire, release, transfer ownership
of, or retain those handles after returning. The caller may use the same holds
for subsequent operations in the transaction, then releases global before
feature. No stale-breaking, wait, retry, PID metadata or reentrancy is added.

Existing transaction scopes remain unchanged:

- Finalization: feature held across revalidation, worktree creation, artifact
  publication and pointer update; global held only around worktree add/remove.
- Acceptance: feature and global held across the entire transaction.
- Rejection/recovery/preflight pruning: feature held across candidate-state
  changes; global additionally held for the worktree-removal operation.
- Read-only reference access and pure derivation: no candidate lock required.

S1 implements only the primitives and composition tests. Actual finalization,
acceptance, pointer writes and recovery stay in their already assigned slices.

## D2 — Amend one constructor and the prune method

The two named bare-root constructors remain the only candidate-package
boundaries that accept a repository-root string. The reference signature is
unchanged; the candidate constructor receives two explicit typed capabilities:

```go
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error)

func CreateCandidateRoot(
    repoRoot, slug, candidateID, commit string,
    featureLock, worktreeMutationLock *candidatelock.Handle,
) (*CandidateRoot, error)

func (r *CandidateRoot) Prune(
    featureLock, worktreeMutationLock *candidatelock.Handle,
) error
```

No overload, variadic optional hold, third root constructor or implicit fallback
may bypass these required arguments. The package-private checks supporting
them are not alternative public mutation entrypoints.

`ReferenceRoot.Close` and `CandidateRoot.Close` own only their rooted filesystem
resources. They never close caller-owned lock handles. After a prune operation
begins closing owned roots, every subsequent exit must finish releasing those
owned roots and report failures; no half-closed object may appear reusable.
Lock release remains the caller's responsibility even if creation/pruning fails.

## D3 — Validate capabilities, not labels

Before any namespace creation, Git worktree add/remove or residue cleanup,
validate that both capabilities:

1. were minted by a successful lock acquisition and still own live descriptors;
2. are non-nil, initialized, unreleased, and distinct;
3. refer respectively to the exact requested repository/feature lock
   `.tpatch/locks/reconcile/<slug>.lock` and the same repository's
   `.tpatch/locks/worktree-mutation.lock`;
4. still match the opened regular lock objects and their safe parent/path
   identities, not merely the strings returned by `Handle.Path`.

Wrong-feature, wrong-repository, swapped-role, zero, closed and replaced-path
handles refuse before mutation. A supplied global handle alone is insufficient.
Checks must bind to the actual held repository/root identity and preserve
path bytes; aliases must not silently select another repository or lock.

A borrow must keep the underlying lock descriptor alive throughout the
operation. Concurrent release must not silently close the lock mid-operation:
refuse release while borrowed rather than wait or claim success. Serialize
handle state access internally; do not introduce an ambient ownership registry.
The primitive releases its borrow on every exit but leaves the caller's
underlying lock held. This is in-process capability lifetime, not another
kernel acquisition or a cross-host guarantee.

Malformed capabilities are primitive misuse/refusal errors, not a new CLI
flag or evidence reason-code taxonomy. Existing acquisition contention remains
`candidate-lock-held`, unsupported targets `lock-unsupported`, and unknown/
remote filesystems `lock-filesystem-unsupported`.

## D4 — No relaxation of the accepted safety boundary

Preserve ADR-034 held-root confinement and explicit path/object ownership at
cleanup, exact local commit validation, no lazy fetch, nonblocking filesystem
lock support/refusals and the no-Git-history-rewrite rule. Borrowed locks are
necessary for worktree mutation but do not establish snapshot/content identity,
CAS correctness or safe replay by themselves.

This amendment changes no E1-E15 eligibility gate, CG1-CG7 confirmation rule,
candidate identity/schema, alignment limit, dependency policy, transaction
step, publication order, phase outcome, public CLI, resource default or release.
The nine existing PRD/ADR parity blocks remain byte-identical and unchanged.

## Required validation

Use the same production capability validator in positive and negative cases.
Before acceptance of the implementation, prove at least:

1. A caller holding feature then global can create/prune without self-contention,
   and can perform another operation using those still-held handles.
2. A separate handle/process cannot acquire either lock while the caller holds
   it, including after a successful or failed borrowed operation; acquisition
   succeeds after explicit caller release.
3. Nil/zero/released/wrong-feature/wrong-repository/swapped/duplicate/replaced
   capabilities refuse before namespace or worktree mutation.
4. Release during a live borrow refuses without dropping exclusion; after
   borrow completion the caller can release normally. Failure exits end borrows.
5. Forced Git-remove failure and residue-cleanup failure close all owned roots,
   retain honest failure state and do not release caller locks or delete foreign
   replacement data.
6. The constructor/prune signature and source guards reject omitted capabilities,
   implicit reacquisition/release and unlocked alternative mutation entrypoints.
   Existing S0 goldens remain untouched.
7. Native Linux/macOS/Windows contention, forced-process-exit release and
   same-process reacquisition still pass in required validation paths.
   Cross-compilation alone does not establish native support.

No extra production transaction or post-S1 behavior is needed to demonstrate
composition: synthetic disposable repos and real caller-held handles suffice.

## Amendment map after review approval

| Surface | Narrow update |
|---|---|
| ADR-037 D12 / PRD 6.9 | Replace the candidate constructor signature above; retain reference signature and the exact two bare-root constructor names. Add explicit borrowed-lock precondition. |
| ADR-037 D28 / PRD 6.13 | Clarify caller-owned acquisition/release, borrowed operation lifetime, and feature/global scope table without changing ordering or scope. |
| PRD S1 / ROC-120 and lock/prune test obligations | Require lock-aware creation/pruning and focused composition/invalid-capability controls; do not renumber existing rows or claim later-slice tests are implemented. |
| S1 code/guard registry | Include only this authorized contract amendment and necessary capability validation; retain original S0 capture restrictions and dependency boundaries. |

The frozen ADR index is not silently rewritten by this proposal. Register this
ADR through these explicit links and current tracking; any index maintenance
must preserve its existing frozen contract tests. No full-suite pass is claimed
for documentation-only drafting.

## Open questions

Independent review must confirm that passing the existing handles is sufficient
without a second public ownership type. If it is not, revise this proposal
explicitly before implementation rather than add an undocumented constructor
or implicit ownership mechanism.
