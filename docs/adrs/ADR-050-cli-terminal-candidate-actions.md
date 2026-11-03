# ADR-050: CLI terminal operation-candidate actions

- **Status**: Proposed
- **Date**: 2026-11-03
- **Scope**: GH #13 slice S7 only

## Context

S5 provides candidate diff, rejection and rejection-log lifecycle primitives.
S8 will provide the acceptance transaction and journal recovery. S7 must expose
the complete CLI surface without inventing either later behavior.

## Decisions

### Source-derived exclusivity

The second validation stage runs only when one of the five new terminal flags
is explicitly set. It enumerates `cmd.Flags()`, removes the presentation flags
`path` and `format`, removes Cobra's auto-registered `help` by name, and checks
`Changed` for every remaining registered reconcile flag. The five new names
are known because they define the terminal surface; existing behavioral names
are never copied into a list.

For the current command this derives 15 existing behavioral flags, producing
75 new-versus-existing and 10 new-versus-new refusing pairs. Default values do
not imply operator intent. Existing-only calls bypass this stage and retain the
three shipped `validateReconcileFlags` messages byte-for-byte.

Resolver refusal uses the same effective-slug resolver as `RunReconcile`.
That resolver performs the no-argument applied/active sweep, supersession
filtering and DAG ordering before the CLI checks every effective slug for a
pending candidate. The check therefore runs before provider loading or
mutation even when the operator supplies no positional slug. It runs after the
existing `--check-applied-only` and `--preflight` early returns, because those
read-only modes never enter resolver execution and retain their shipped
behavior even when `--resolve` is also present.

### S7 and S8 boundary

`DiffOperationCandidate`, `RejectOperationCandidate` and
`ClearOperationCandidateRejections` export the accepted S5 lifecycle behavior.
`AcceptOperationCandidate` and `ResumeOperationCandidateRecovery` are explicit
workflow seams that currently return the typed
`candidate-acceptance-unavailable` state refusal. They map to exit 3 and state
that acceptance or recovery is not yet available. S7 never writes acceptance
state, fabricates a journal, or reports success. S8 replaces these seams with
the approved transaction and recovery behavior.

### Exit envelope

Terminal action errors carry a stable workflow refusal code. The CLI maps the
§7.4 classes through `ExitCodeError`:

| Exit | Codes |
|---|---|
| 2 | `candidate-absent`, `head-diverged`, `candidate-cas-failed`, `candidate-path-occupied`, `candidate-binding-changed`, `candidate-test-failed` |
| 3 | `candidate-pending`, `shadow-pending`, `candidate-not-pending`, `candidate-already-accepted`, `candidate-feature-state-changed`, `candidate-lock-held`, `lock-unsupported`, `lock-filesystem-unsupported`, `recovery-required`, `coverage-publication-failed`, `acceptance-rolled-back`, and the temporary S7 `candidate-acceptance-unavailable` |
| 1 | `acceptance-rollback-failed` and unexpected internal errors |

Flag and slug-arity refusals occur before store mutation and use exit 2.
Resolver `--accept`, `--reject` and `--shadow-diff` remain resolver-only; a
pending operation candidate produces `candidate-pending` with named
candidate-clearing remediation.

Every terminal renderer propagates its writer error through the same envelope.
Human reject/clear output and JSON output therefore return exit 1 instead of
reporting success after a failed write.

## Consequences

- `--help` remains documentation, not a behavioral conflict.
- Future reconcile flags automatically become exclusive with new terminal
  actions unless deliberately added to the presentation allowlist.
- S7 exposes honest, non-successful placeholders for S8-only actions.
- The existing gate-on
  `TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch` already pins
  the required expectation beside the historical gate-off golden, so S7 does
  not duplicate or edit it.
