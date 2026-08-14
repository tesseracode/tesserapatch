# PRD — Prepare Intent Bundle — `tpatch prepare <slug>` (mutating modes)

**Status**: Draft — Awaiting Review (rev-2)
**Date**: 2026-08-14
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, rev-2 from reviewed writer tip `91dea32`;
dispatch/base `176ee06`; WAVE_BASE `d060ff4`
**Milestone**: TBD — this document ships no code
**Issue**: [GH #11 — define the mutating prepare intent-bundle contract](https://github.com/tesseracode/tesserapatch/issues/11)
**Graduates from**: [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md), Turns 2–4
**Prerequisite (accepted)**: [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md)
rev-5 (Accepted 2026-08-13) and [ADR-034](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md)
rev-2 (Accepted 2026-08-13). This PRD builds **on top of** that read-only
contract and does not reopen it. Its **implementation** is a hard sequencing
prerequisite for every mutating slice here (§17.1).
**Architecture**: [ADR-035 — Intent bundle publication and history](../adrs/ADR-035-intent-bundle-publication-and-history.md)
(**Proposed**, rev-2). **This PRD and ADR-035 must be reviewed together.**
ADR-035 locks the publication/history decisions; this PRD states the product
contract that depends on them. Where they overlap, **ADR-035 is normative**.

> **Implementation is not authorized by this document.** No Go file, test,
> asset or CLI surface may change until **both** this PRD and ADR-035 are
> accepted. §19 states the gate; §17.2 states the slices that become dispatchable
> afterwards.

## Related

- [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md) — the accepted read-only `prepare <slug> --check` contract this PRD extends without modifying (its §20 lists exactly what this document must answer)
- [ADR-034 rooted filesystem inspection boundary](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md) — D1–D18; reused verbatim for every canonical **read** this command performs
- [ADR-035 intent bundle publication and history](../adrs/ADR-035-intent-bundle-publication-and-history.md) — **companion, Proposed rev-2**; the transaction, the archive and the honesty limits
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md) — `## Agreed — Turns 2–3` items 4–9
- [WP-005 turn log](../whitepapers/WP-005-spec-driven-workflows.turns.md) — Turns 2, 3 and 4
- [Agent as Provider — Path B workflow](../agent-as-provider.md) — the phase → artifact → state contract
- [Path B Operator Guide](../path-b-operator-guide.md) — the hand-authored artifact flow this PRD's `--manual` mode adopts
- [Feature Layout](../feature-layout.md) — canonical vs audit-trail files under `.tpatch/features/<slug>/`
- [PRD-tpatch-land](./PRD-tpatch-land.md) — the shipped journal + crash-recovery precedent this PRD reuses and deliberately diverges from (redo vs undo)
- [PRD-feature-resource-claims-and-capture-adapters](./PRD-feature-resource-claims-and-capture-adapters.md) — the shipped content-addressed immutable-set + pointer precedent the archive reuses, **and the shipped kernel `flock(2)` process-lifetime lock this PRD's lock is extracted from** (`internal/rescap/lock_unix.go:6-11`)
- [ADR-033 resource capture boundary](../adrs/ADR-033-resource-capture-boundary.md) — D4 (the closed six-class redaction scan), D10 (no tracked timestamps), D11 (no Go map in a wire schema)
- [ADR-027 capture context privacy boundary](../adrs/ADR-027-capture-context-privacy-boundary.md) — **D3 (redaction is a write precondition, not best-effort cleanup)**, D2 (no raw context), D5 (retention semantics), D6 (no wall-clock in determinism)
- [ADR-031 rejected feature state data model](../adrs/ADR-031-rejected-feature-state-data-model.md) — the per-command exit envelope and closed-enum precedent
- [PRD-active-feature-session](./PRD-active-feature-session.md) — the shipped `session purge` deletion grammar (dry-run default + `--yes`) reused by the archive retention surface (§9.7)
- [PRD-tpatch-doctor](./PRD-tpatch-doctor.md) — the diagnostic surface that gains the pending-transaction / orphan-state check (§12.5)

## Revision history

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | Draft — Awaiting Review | First draft. Defines the four modes, the publication transaction, the undo journal, the content-addressed intent archive (**architecture gate fired → ADR-035**), the staged generator extraction, the lifecycle/compatibility deltas and a 234-row acceptance matrix. |
| rev-1 | Draft — Awaiting Review | Closes every rev-0 finding. **Lock authority replaced**: the unprovable `O_EXCL` stale-lock model is gone; the lock is the shipped kernel `flock(2)` process-lifetime lock (extracted, plus a Windows deny-share sibling), so ownership is released on process death and liveness is decidable without a journal (§7.4, ADR-035 D4). **Journal rebuilt**: plan-digest binding, old *and* intended-new identity for every canonical **and** metadata path, and a semantic compare-and-swap before every publish, undo and remove (§7.5, §7.6, §7.9, ADR-035 D5). **T1 scoped honestly** to a command-owned verification point under the lock (§7.1, ADR-035 D6). **Rooted writes**: `(*os.Root).Rename` / `(*os.Root).OpenFile` adopted for every publication write, so the residual race narrows to content identity (§7.7, ADR-035 D2). **Default coherence**: the sidecar is never overwritten without `--regenerate`, and the generated set must be a dependency-coherent suffix (§6.1). **Archive privacy/retention**: ADR-027 D3's redaction scan is a write precondition and a match refuses regeneration; the "no new exposure class" claim is withdrawn; a bounded `tpatch feature intent-archive list\|purge` retention surface, tombstones and orphan-blob reporting are added (§9, ADR-035 D8/D15/D16). **`--regenerate` requires a configured provider** unless `--allow-heuristic` is passed (§11.3, ADR-035 D18). **Git usage disclosed and made conditional** (§7.13, ADR-035 D17). **`FEATURES.md` named as derived, best-effort and outside T1** (§12.3.1). **Dry-run refuses when recovery is pending** (§6.4). **Exit 6 gains a safe `--abandon-transaction` route** (§6.6). Matrix rebuilt to 394 rows (160 new, 15 amended in place). |
| rev-2 | Draft — Awaiting Review | Adjudicates every rev-1 finding without reopening prior closures. **Stable lock authority** moves outside the worktree to a persistent, opaque per-user cache namespace keyed by canonical native workspace-root identity; Git cleanup cannot unlink/recreate a live authority (§7.4, ADR-035 D4). `--manual` is now a rooted single-file temp+rename with an immediate status preimage CAS (§6.2, ADR-035 D2/D3). Tombstones retain immutable content digests, X10 validates them after purge, and a duplicate tombstoned generation rehydrates rather than leaving an orphan (§9.3, §9.7, ADR-035 D10/D16). Purge obtains a strict index preimage and CASes it immediately before rename. Mutating support is honestly `linux`, `darwin`, `windows`; read-only `--check` remains `unix \|\| windows`. G1, run from the workspace root, replaces `.git` probing; only Git-established non-worktrees may use the non-Git route (§7.13). Rooted writes stop outside-root escape but can follow an in-root redirect; CAS detects only an identity mismatch, and the exact residual is disclosed (§7.7, ADR-035 D2/D6). Doctor no longer infers journal loss from ordinary lifecycle bytes. Matrix: 409 rows, with 15 new rev-2 rows (`PIB-395`…`PIB-409`) and the listed amended rows. |

## Summary

The accepted prerequisite PRD gave `tpatch` a truthful **answer**: which intent
artifacts a feature has, and that their provenance is `unknown`. It
deliberately shipped no way to **act** on that answer, and refused plain
`tpatch prepare <slug>` with a reserved-surface exit 4
(`docs/prds/PRD-artifact-validation-and-provenance.md:356-382`).

This PRD defines that action. It adds three mutating modes and one recovery
escape hatch to the same verb:

```text
tpatch prepare <slug>                        # Path A: generate ONLY the missing required artifacts
tpatch prepare <slug> --manual               # Path B: adopt an already-complete hand-authored bundle
tpatch prepare <slug> --regenerate           # explicit, archived, coherent overwrite of the whole bundle
tpatch prepare <slug> --abandon-transaction  # unblock a slug an interrupted run left divergent (§6.6)
```

Four properties are load-bearing, and each is stated as a limit rather than a
slogan:

1. **Default preserves.** Every canonical artifact that is already
   `present-nonempty` — **including the structured sidecar
   `artifacts/analysis.json`** — is never opened for writing, never staged and
   never renamed over. Only genuinely missing required artifacts are generated,
   and only when the missing set is a **dependency-coherent suffix** of
   analysis → spec → exploration (§6.1.2). An existing required artifact that
   is `present-empty`, `invalid-structured`, `symlink-refused`, `not-regular`,
   `unreadable`, `oversize` or `unstable` is **refused**, not silently
   overwritten (§6.1, §7.2).
2. **`--regenerate` cannot lose bytes, and cannot silently downgrade them.**
   Before the first canonical mutation, every artifact it is about to replace
   is copied into a durable, content-addressed, immutable **intent archive**
   inside the feature directory — after passing the ADR-027 D3 redaction scan,
   which **refuses** rather than archiving secret-shaped content (§9.6). The
   archive is a *byte-recovery* mechanism and is explicitly **not** a
   provenance representation (§9, ADR-035 D9). `--regenerate` additionally
   requires a **successful configured provider** unless the operator opts in to
   heuristic replacement with `--allow-heuristic` (§11.3): a hand-authored
   bundle is never replaced by boilerplate because a provider call timed out.
   This selection is what fires the architecture gate and requires ADR-035
   (§8.4).
3. **The transaction guarantee is command-boundary, verified at a point this
   command owns.** An ordinary rename sequence over six files cannot make them
   appear simultaneously to a concurrent reader, and this PRD never claims it
   does. What is guaranteed is (a) at a **final verification point under the
   lock, immediately before the command returns**, every publication entry
   matches its intended new image or the command reports divergence, (b) a
   crash inside the publication window is recoverable to all-old, or refused
   with everything preserved, and (c) the exposure window is bounded and
   enumerated (§7.1). External writes that land **after** that verification
   point are outside the postcondition and are stated as such.
4. **No new lifecycle state.** A successful mutation ends at the existing
   `defined` (`internal/store/types.go:11`). `--check` mode is byte-identical
   to the accepted contract. Nothing calls `prepare`; `next`, `cycle` and the
   individual phase commands keep their current routing except for the
   enumerated deltas in §12.6.

The publication unit — the thing that is all-old or all-new at the command
boundary — is exactly WP-005 Agreed item 7's set
(`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`): `analysis.md`,
`spec.md`, `exploration.md`, the structured sidecar `artifacts/analysis.json`,
the archive index, and the final `status.json` transition. `FEATURES.md` is
**not** in that set: it is a derived index the shipped writer refreshes
best-effort (`internal/store/store.go:363-377`), and §12.3.1 states its
semantics truthfully instead of pretending it does not exist.

### What this PRD does not claim

- It does not claim the generated artifacts are *good*. The accepted
  disclaimer ("Structural presence only. This report does not certify semantic
  quality.", `docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798`)
  applies verbatim to everything this command writes.
- It does not claim durable per-artifact provenance. `prepare --check` keeps
  reporting `provenance: unknown` for every artifact after this PRD ships, for
  every artifact this command writes, and §9.8 states exactly why the archive
  does not change that.
- It does not claim multi-file atomicity (§7.1).
- It does not claim that the archive introduces **no new exposure class**.
  rev-0 said that and it was wrong: §9.6 discloses the durable tracked
  retention the archive creates and the removal route that bounds it.
- It does not claim `prepare` runs no Git command. rev-0 said that and it was
  wrong: §7.13 enumerates the exact read-only Git invocations, when each runs,
  and what happens when Git is absent or unusable.
- It does not claim bounded runtime. ADR-034 D16 withdrew every such claim for
  the read half; the write half adds provider calls, so the honest statement is
  a **per-phase and total deadline** (§11.5), not a guarantee of termination
  without one.
- It does not mandate any spec-driven methodology on downstream users. WP-005
  Agreed item 3 (`docs/whitepapers/WP-005-spec-driven-workflows.md:56-58`) is
  asserted mechanically, not assumed (§14.2, PIB-217).


## 1. Problem statement

### 1.1 There is no way to complete an intent bundle in one step

Today an operator completes a bundle by running up to three separate commands
(`analyze`, `define`, `explore`), each of which writes its artifact and
advances state independently (`internal/workflow/workflow.go:90-105`,
`:151-155`, `:196-200`). If the second fails, the tree keeps the first
artifact and a `analyzed` state — a half-bundle that no command reports as
such. WP-005 Agreed item 7 names this the thing a mutating `prepare` must fix
(`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`).

### 1.2 `cycle` is not that command

`cycle` runs analyze → define → explore → implement → apply → record in one
process (`internal/cli/phase2.go:26-32`). It calls the same incremental phase
functions in sequence (`internal/cli/phase2.go:62-96`), asserts state after
each (`internal/cli/phase2.go:69-71`), and continues past `defined` into
`implementing` and beyond unless `--skip-execute` is passed
(`internal/cli/phase2.go:122-126`). It is a **pipeline**, not a transaction:
every intermediate write is published to the canonical tree as it happens, and
a failure at step 3 leaves steps 1 and 2 on disk. §4 develops this as the
required existing-primitives pre-flight.

### 1.3 The phase functions cannot be reused as-is

`RunAnalysis` writes `artifacts/analysis.json`
(`internal/workflow/workflow.go:90`), then `analysis.md`
(`internal/workflow/workflow.go:96`), then mutates state
(`internal/workflow/workflow.go:103`) — three canonical mutations inside one
call, in that order, with a live `*store.Store`. `RunDefine` and `RunExplore`
have the same shape (`internal/workflow/workflow.go:151-155`, `:196-200`).
Worse, the retry helper writes raw provider responses into the canonical
artifacts directory *during* generation
(`internal/workflow/retry.go:105-109`), as does `RunAnalysis`'s provider-error
fallback (`internal/workflow/workflow.go:72`, `:80`). A `prepare` that called
these functions would have published four to twelve canonical files before it
knew whether the bundle would complete. WP-005 Turn 3 states exactly this
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`).

### 1.4 Overwriting is currently unconditional and unrecoverable

`WriteFeatureFile` → `writeFile` → `os.WriteFile`
(`internal/store/store.go:443-449`, `:918-923`) truncates in place. There is no
preimage capture anywhere in the intent path. `tpatch define` run twice
destroys a hand-authored `spec.md` with no recovery route inside `tpatch`, and
`.tpatch/` is frequently untracked in real repositories — the CLI has a
dedicated helper to detect that (`internal/cli/cobra.go:3405-3407`). Any
`--regenerate` flag built on today's primitives would be a byte-shredder.

### 1.5 Path B has no completion verb

The documented Path B flow authors three files by hand and then runs three
`--manual` commands (`docs/path-b-operator-guide.md:61-73`). Each of those
commands validates only that the file *exists*
(`internal/store/manual.go:56-66`) — a zero-byte `spec.md` advances the feature
to `defined`. There is no single "I have authored the whole bundle, adopt it"
step, and no step that checks the bundle as a set before committing the
transition.

## 2. Goals / Non-goals

### 2.1 Goals

1. One command that takes a feature from any allowed pre-`defined` state to a
   **complete** intent bundle at `defined`, or leaves the tree exactly as it
   found it.
2. Preservation by default: never overwrite an existing non-empty canonical
   intent artifact — including the structured sidecar — unless the operator
   explicitly asked for it.
3. A `--regenerate` route whose destructiveness is bounded by a durable,
   in-repository byte archive, so prior hand-authored content is recoverable
   without Git, **and** whose replacement content is never silently downgraded
   to heuristic boilerplate.
4. A truthful transaction contract that distinguishes command-boundary
   all-or-nothing, crash recoverability, and instantaneous multi-file
   visibility — claims only the first two, and scopes the first to a
   verification point this command owns.
5. Crash recovery that decides from **evidence**, never from a recorded phase
   label, refuses rather than guesses when the evidence is divergent, and can
   never permanently brick a slug: lock ownership dies with the process, and
   every refusal has a safe operator route out (§6.6).
6. Behavior compatibility: `--check`, `next`, `cycle`, the individual phase
   commands, `status`, `verify`, `doctor`, `land`, `record` and `reconcile` are
   unchanged except for the enumerated deltas in §12.6.
7. Reuse of the accepted ADR-034 rooted-inspection boundary for every canonical
   read, and of `*os.Root`'s handle-relative write primitives for every
   canonical write, with no path-based write after a rooted check.
8. Determinism in tracked artifacts: no wall-clock field, content-addressed
   identifiers, canonical JSON, stable key order.
9. A **bounded** archive: durable retention is disclosed, an operator can list
   what is retained, purge it safely, and remediate an accidentally archived
   secret without hand-editing tracked JSON.

### 2.2 Non-goals

1. **No new `FeatureState`.** WP-005 Agreed item 6
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:71-74`).
2. **No semantic quality judgement.** No thinness score, no lint, no heading
   requirement, no "stub detector".
3. **No durable per-artifact provenance.** The WP-005 provenance ADR trigger
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2951-2985`) stays
   **unfired**; §9.8 and ADR-035 D9 state why the archive does not fire it.
4. **No change to `--check`'s report schema, exit envelope or bytes.**
5. **No mandatory SDD methodology** for downstream users.
6. **No restore verb.** Recovering an archived blob in v1 is an ordinary file
   copy (§9.5, Q4). Removal, unlike restoration, *is* in scope (§9.7), because
   an unbounded tracked retention surface with no removal route is a privacy
   defect rather than a missing convenience.
7. **No `--all` sweep**, no multi-slug form, no `--fix`.
8. **No interactive confirmation prompt** in the mutating publication modes
   (§5.4). The archive-purge and abandon routes are the two exceptions, and
   they use the shipped non-interactive `--yes` gate, not a prompt.
9. **No implement/apply/record orchestration.** `prepare` stops at `defined`.
10. **No `--force` in any mode.** There is no flag anywhere in this design that
    overwrites a canonical file whose current bytes failed a compare-and-swap,
    and no flag that removes a lock a live process holds.


## 3. Terminology

| Term | Meaning in this document |
|---|---|
| **Intent bundle** | `analysis.md`, `spec.md`, `exploration.md` — the three canonical Markdown artifacts, per the accepted PRD §6.2 (`docs/prds/PRD-artifact-validation-and-provenance.md:432-436`). |
| **Publication set** | The ordered list of canonical files this invocation will create or replace, plus the archive index and `status.json`. Computed at preflight, frozen before the journal is written, and bound into the journal by a digest (§7.2, §7.5). |
| **Publication unit** | The set that is all-old or all-new at the command-owned final verification point (§7.1 T1). Identical to the publication set. Excludes `FEATURES.md` (§12.3.1). |
| **Preimage** | The identity (existence, SHA-256, size, mode) of a canonical or metadata file as captured under the transaction lock immediately before publication. |
| **New-image** | The identity of the staged bytes that will replace it. |
| **Semantic CAS** | A compare-and-swap on **identity, not on a lock**: before every publish, rollback or remove of one entry, the file's current identity is recomputed and compared to the exact identity that step expects (preimage for a publish, new-image for a rollback, absence for a create-undo). A mismatch refuses that step and preserves the bytes (§7.8). |
| **Staging tree** | `.tpatch/local/intent-prepare/<slug>/stage-<12hex>/` — gitignored scratch where generated bytes live before publication. |
| **Journal** | `.tpatch/local/intent-prepare/<slug>/journal.json` — the durable **undo** record, bound to its plan by `plan_digest` (§7.5). |
| **Transaction lock** | A kernel, process-lifetime advisory lock at an opaque, persistent per-user cache locator keyed by native workspace-root identity plus canonical slug; it is outside the worktree and released by the OS on holder death (§7.4). The zero-byte cache file carries no authority. |
| **Intent archive** | `.tpatch/features/<slug>/artifacts/intent-archive/` — the tracked, content-addressed, immutable store of replaced bytes (§9). |
| **Generation** | One archive record naming the set of artifacts replaced by a single `--regenerate` publication (§9.3). Content-addressed; not a timestamped event. |
| **Tombstone** | An index entry whose blob was deliberately purged (§9.7): the generation record and immutable `content_sha256` survive, `blob` is empty and `purged` is `true`, so X10 remains computable and the index never claims bytes it no longer has. |
| **Orphan blob** | A blob file written by an attempt that never published an index entry (crash, or a refusal after the blob write). Additive, harmless, reported and removable (§9.7.3). |
| **Coherent suffix** | The rule that the default mode may only generate a suffix of the dependency chain analysis → spec → exploration, so a generated artifact is never published beneath a downstream artifact that did not derive from it (§6.1.2). |
| **Mode** | Exactly one of `check`, `generate` (default), `manual`, `regenerate`, `abandon` (§5.2). |
| **Inspector** | The accepted read-only classifier from the prerequisite PRD §7, reused unchanged. |


## 4. Existing-primitives preflight

WP-005 Turn 3 requires this section by name
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:118-123`). The
question is not "is a new command nicer" but "can a shipped primitive already
carry this responsibility".

| Primitive | What it does today | Why it cannot carry this |
|---|---|---|
| `analyze\|define\|explore` (Path A) | One provider call, one artifact write, one state transition each (`internal/workflow/workflow.go:90-105,151-155,196-200`) | Each publishes to the canonical tree before the next runs. Three invocations cannot be all-or-nothing, and a failure in the middle is not observable as a half-bundle. |
| `analyze\|define\|explore --manual` (Path B) | Validates the artifact **exists**, then advances state (`internal/store/manual.go:56-81`) | Presence-only; a zero-byte file advances state. One artifact per invocation. No set-level view. Accepted PRD §12 deliberately leaves these loose (`docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015`), and this PRD does not tighten them (§12.4). |
| `cycle` | Sequential pipeline through `record` (`internal/cli/phase2.go:26-32,62-121`) | Not a transaction (§1.2); overshoots `defined`; regenerates unconditionally, so it is exactly the byte-shredder §1.4 describes. |
| `next` | Emits one action, inferring the `defined` sub-state from `exploration.md` presence (`internal/cli/phase2.go:437-446`) | Advisory only; writes nothing; cannot generate. |
| `prepare --check` (accepted) | Read-only classification and readiness verdict | Writes nothing by contract (`docs/prds/PRD-artifact-validation-and-provenance.md:2840-2866`). It is the **input** to this PRD, reused wholesale. |
| `store.writeFileAtomic` | Path-based temp + `Chmod` + `Write` + `Sync` + `Close` + `Rename` + parent-dir `Sync` (`internal/store/store.go:878-917`) | Correct historical single-file shape, but not safe as a `prepare` writer because its names are path-based. `prepare` uses the rooted equivalent even for `--manual` (§6.2, §7.7.3). |
| `land`'s journal | Durable journal + evidence-based recovery under the index lock (`internal/cli/land_journal.go:445-482`) | The right **shape**, wrong direction: land must roll *forward* because `git commit` already advanced HEAD and cannot be undone (`internal/cli/land_journal.go:11-23`). Nothing in a prepare publication is irreversible, so this PRD's journal is undo-only (§7.5, ADR-035 D5). Its `O_EXCL`-plus-nonce lock is reused **only as a rejected alternative** (§7.4.1). |
| `rescap.AcquireLock` | Kernel `flock(2)` on an open descriptor, `LOCK_EX\|LOCK_NB`, released by descriptor close — including on SIGKILL, crash or power loss (`internal/rescap/lock_unix.go:63-88`), with a fail-closed stub on unsupported targets (`internal/rescap/lock_unsupported.go:29-32`) | The right **primitive**, wrong package boundary: it is welded to resource capture's refusal vocabulary and its `linux \|\| darwin` build partition. §7.4 extracts it into a shared package, adds a Windows sibling, and leaves `rescap`'s observable behavior byte-identical. |
| `store.PublishBatch` | Content-addressed immutable batch + atomically rewritten pointer, with a semantic-body compare before any rewrite (`internal/store/resource_publish.go:230-285`, `internal/store/resource_publish.go:358-399`) | The right **shape** for the archive (§9) and the right **precedent** for semantic CAS (§7.8), reused as design precedents. It publishes one pointer, not a six-file set, so it is not the transaction either. |
| `redact.Scan` | The closed six-class secret scan applied to in-memory bytes before anything is persisted (`internal/redact/redact.go:53-72,81-90`) | Reused verbatim as the archive's write precondition (§9.6), because ADR-027 D3 makes redaction a precondition for **every** tpatch-owned writer, not an archive-specific idea. |
| `MkdirAllAndSyncChain` / `SyncDir` / `RandomHex12` | Crash-safe directory and scratch primitives (`internal/store/fsdurable.go:22-33,41-52,96-103`) | Reused verbatim. They are ingredients, not the recipe. |
| `rescap` scratch + local-ignore gate | `.tpatch/local/<...>` scratch with a mandatory gitignore contract (`internal/rescap/scratch.go:47-62`) | Reused for the staging tree, lock and journal location (§7.3, §13.5) — but the gate **runs Git** and refuses when Git is unavailable (`internal/workflow/session_ignore.go:138-145`), so §7.13 makes its application conditional and discloses it instead of claiming this command runs no Git. |
| `session purge` | Dry-run-by-default deletion of local buffers with an explicit `--yes` gate and a scope mutex (`internal/cli/session.go:353-419`) | Reused as the **grammar precedent** for the archive retention surface (§9.7). It purges a gitignored lane; the archive is tracked, so §9.7 adds the tombstone and reference rules that a tracked store needs. |

**Conclusion.** Every ingredient exists; the composition does not. The new
surface is the composition (a transaction over a publication set) plus one new
durable object (the archive) plus one bounded retention surface for it.
Nothing else is invented.


## 5. CLI grammar and mode selection

### 5.1 Authorized grammar (v1, complete)

```text
tpatch prepare <slug> --check       [--json] [--quiet] [--path <dir>]
tpatch prepare <slug>               [--json] [--quiet] [--path <dir>] [--timeout <d>] [--timeout-phase <d>] [--no-retry] [--dry-run]
tpatch prepare <slug> --manual      [--json] [--quiet] [--path <dir>] [--dry-run]
tpatch prepare <slug> --regenerate  [--json] [--quiet] [--path <dir>] [--timeout <d>] [--timeout-phase <d>] [--no-retry] [--dry-run] [--allow-heuristic]
tpatch prepare <slug> --abandon-transaction [--json] [--quiet] [--path <dir>] [--yes]

tpatch feature intent-archive list  <slug> [--json] [--quiet] [--path <dir>]
tpatch feature intent-archive purge <slug> (--blob <hash>... | --generation <id>... | --orphans | --all) [--yes] [--json] [--quiet] [--path <dir>]
```

- `<slug>` — exactly one, required, validated by the accepted canonical-slug
  grammar before any path is composed
  (`docs/prds/PRD-artifact-validation-and-provenance.md:696-772`). No new slug
  rule is introduced.
- `--check` — the accepted read-only mode. **Unchanged in every respect**
  (§12.1).
- `--manual` — Path B adoption. No provider call, no artifact write (§6.2).
- `--regenerate` — explicit archived overwrite of the whole bundle (§6.3).
- `--abandon-transaction` — the operator escape hatch from a divergent
  interrupted run. Touches **no** canonical file (§6.6).
- `--allow-heuristic` — permitted **only** with `--regenerate`. Opts in to
  replacing the bundle with heuristic output when the provider is missing or
  fails. Without it, `--regenerate` requires a successful configured provider
  (§11.3). Rejected with any other mode as a parse-time exit 1.
- `--dry-run` — report the plan; write nothing at all, including no lock, no
  journal and no staging tree; make no provider call (§6.4).
- `--timeout` — the **total** deadline for all generation in the invocation.
  Default `180s` (§11.5, Q2). Rejected in `--check`, `--manual` and
  `--abandon-transaction`, which make no provider call.
- `--timeout-phase` — the per-generator deadline. Default `90s`, and always
  clamped to the remaining total (§11.5). Same mode restrictions as
  `--timeout`.
- `--no-retry` — disables validator-driven retry, exactly as the phase commands
  do today (`internal/cli/cobra.go:630`, `internal/workflow/retry.go:83-85`).
  Rejected in `--check`, `--manual` and `--abandon-transaction`.
- `--yes` — required to make `--abandon-transaction` and
  `feature intent-archive purge` act; without it both are previews. The shipped
  `session purge` gate (`internal/cli/session.go:353-419`) is the precedent.
  Rejected in every other prepare mode.
- `--json`, `--quiet` — the report routing shipped by the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419`), extended
  to the mutating report schema of §10.
- `--path` — the inherited root persistent flag (`internal/cli/cobra.go:66`).

No other flag is registered on `prepare`. There is no `--all`, no `--fix`, no
`--force`, no `--restore`, no `--format`, no `--interactive`.

**Why the retention surface is a `feature` subcommand and not a sixth prepare
mode.** `tpatch feature` already groups per-feature management verbs
(`internal/cli/feature_deps.go:43-53`), including `feature resource remove` and
`feature resource clear` (`internal/cli/feature_resource.go:655,699`). A purge
is management of a stored artifact, not preparation of intent; putting it on
`prepare` would make the mode mutex non-total and would give a *generation*
verb a *deletion* mode. §9.7 states the full contract.

### 5.2 Mode selection is total and mutually exclusive

`--check`, `--manual`, `--regenerate` and `--abandon-transaction` are declared
mutually exclusive with cobra's `MarkFlagsMutuallyExclusive`, so any two of them
is a **parse-time** error and exits `1` before `RunE` runs. Exactly one mode is
selected:

| `--check` | `--manual` | `--regenerate` | `--abandon-transaction` | Mode |
|---|---|---|---|---|
| set | — | — | — | `check` |
| — | set | — | — | `manual` |
| — | — | set | — | `regenerate` |
| — | — | — | set | `abandon` |
| — | — | — | — | `generate` (default) |

`--timeout`, `--timeout-phase` and `--no-retry` are additionally declared
mutually exclusive with `--check`, `--manual` and `--abandon-transaction`.
`--allow-heuristic` is declared mutually exclusive with `--check`, `--manual`
and `--abandon-transaction`, so it is legal only in `generate` and
`regenerate` mode — and in `generate` mode it is a no-op that emits advisory
`allow-heuristic-redundant`, because fallback is already the default there
(§11.3). `--dry-run` is mutually exclusive with `--check` only. `--yes` is
mutually exclusive with `--check`, `--manual`, `--regenerate` and the default
mode, so it can only appear with `--abandon-transaction`.

### 5.3 Behavior deltas this grammar creates — enumerated, not silent

The accepted contract reserved plain `prepare` and both mutating flags. Shipping
this PRD changes observable behavior on the `prepare` surface, and the accepted
PRD anticipated exactly this
(`docs/prds/PRD-artifact-validation-and-provenance.md:396-403`). The table is
**complete over every flag this PRD registers**: before this PRD, the accepted
implementation registers `--check`, `--json`, `--quiet` only, so every other
flag below is today a cobra `unknown flag` → exit 1.

| Input | Before this PRD | After this PRD |
|---|---|---|
| `tpatch prepare <slug>` | exit `4`, frozen refusal line, no report (`docs/prds/PRD-artifact-validation-and-provenance.md:356-382`) | a real Path A run (§6.1) |
| `tpatch prepare <slug> --manual` | exit `1`, `unknown flag: --manual` | Path B adoption (§6.2) |
| `tpatch prepare <slug> --regenerate` | exit `1`, `unknown flag: --regenerate` | archived coherent overwrite (§6.3) |
| `tpatch prepare <slug> --abandon-transaction` | exit `1`, `unknown flag` | preview of the abandon plan; with `--yes`, performs it (§6.6) |
| `tpatch prepare <slug> --allow-heuristic` | exit `1`, `unknown flag` | legal only with `--regenerate` (or a no-op advisory in default mode) |
| `tpatch prepare <slug> --dry-run` | exit `1`, `unknown flag` | plan-only report (§6.4) |
| `tpatch prepare <slug> --timeout 5s` | exit `1`, `unknown flag` | total generation deadline (§11.5) |
| `tpatch prepare <slug> --timeout-phase 5s` | exit `1`, `unknown flag` | per-generator deadline (§11.5) |
| `tpatch prepare <slug> --no-retry` | exit `1`, `unknown flag` | single-attempt generation (§11.4) |
| `tpatch prepare <slug> --yes` | exit `1`, `unknown flag` | exit `1`, cobra mutual-exclusion text (legal only with `--abandon-transaction`) |
| `tpatch prepare <slug> --check --manual` | exit `1`, `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch prepare <slug> --check --regenerate` | exit `1`, `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch prepare <slug> --check --dry-run` | exit `1`, `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch feature intent-archive …` | exit `1`, `unknown command` | the retention surface (§9.7) |

Consequences that must be carried through, each with an acceptance row:

1. Exit `4`'s reserved-surface population **disappears** and the frozen refusal
   string is deleted. Exit `4` is left **retired with no population** rather
   than rebound to a new meaning, so a harness that hard-coded "4 means
   `prepare` needs `--check`" can never silently misread a different condition
   as that one (§10.4, PIB-013, PIB-014).
2. The accepted PRD's `--help` mitigation for the `apply --mode prepare` name
   collision (`docs/prds/PRD-artifact-validation-and-provenance.md:344-355`)
   stays mandatory and must be updated to describe five modes, not one
   (PIB-016, PIB-017).
3. Every `--check`-plus-mutating-flag row still exits `1` and still writes
   nothing, so the *observable class* is unchanged even though the message text
   changes (PIB-011, PIB-012, PIB-235).
4. Each newly registered flag has its own delta row (PIB-236 … PIB-243), so
   "the flag delta table is complete" is a mechanical assertion, not a claim
   (PIB-244).

### 5.4 No confirmation prompt — decision and justification

The three publication modes perform no interactive confirmation, and there is
no `--yes` on them.

- The repository's only interactive precedent is `cycle --interactive`
  (`internal/cli/phase2.go:50`), which is opt-in and off by default.
- A prompt is not a safety mechanism here: the archive (§9) is. A prompt would
  be theater on top of a mechanism that already makes the operation
  recoverable.
- Prompts break harnesses and non-TTY invocation, and this command is
  explicitly harness-facing.
- The preview route is `--dry-run`, which is deterministic and scriptable.

`--regenerate` is itself the explicit act of consent: it is a flag the operator
must type, it is never implied by any other flag, and it is never the default.

**The two deletion routes are different and are gated differently.**
`--abandon-transaction` and `feature intent-archive purge` destroy state that
no archive backs up, so both are **previews by default** and both require the
non-interactive `--yes` flag to act — the shipped `session purge` shape
(`internal/cli/session.go:353-419`). That is a flag, not a prompt, so it stays
harness-compatible.


## 6. Mode contracts

### 6.1 `tpatch prepare <slug>` — Path A, missing-only

**Intent**: complete the bundle without touching anything that already exists.

**Step 1 — inspect.** Run the accepted inspector over the four artifacts and
`status.json` (`docs/prds/PRD-artifact-validation-and-provenance.md:1653-1782`),
through one held `*os.Root` per ADR-034 D1. No second path model.

**Step 1a — the generation input.** All three shipped generators read
`request.md` and fail hard when it cannot be read
(`internal/workflow/workflow.go:37-40,112,160`). `prepare` therefore captures
`request.md` through the same bounded rooted read and **refuses before any
generation** when it is absent, empty, unsafe or unreadable: exit 3,
`request-unreadable`, zero mutation, no provider call (PIB-233). `--manual`
generates nothing and therefore does **not** require it (PIB-234).

#### 6.1.1 Per-artifact admissibility

For each of the three **required** Markdown artifacts, the inspector state
decides:

| Inspector state | Default-mode disposition |
|---|---|
| `present-nonempty` | **Preserve.** Not staged, not opened for writing, not renamed over. Its bytes are read only as generation context (§11.2). |
| `absent` | **Candidate to generate.** Enters the publication set as a `create` entry **iff** §6.1.2's coherence rule admits it. |
| `present-empty` | **Refuse** (exit 2, `artifact-empty-not-overwritten`). An empty file is *present*; overwriting it is the `--regenerate` route. |
| `invalid-structured` | Not reachable for Markdown (sidecar only). |
| `symlink-refused` | **Refuse** (exit 3, `artifact-unsafe`). Never followed, target never named. |
| `not-regular` | **Refuse** (exit 3, `artifact-unsafe`). |
| `unreadable` | **Refuse** (exit 3, `artifact-unsafe`). |
| `oversize` | **Refuse** (exit 3, `artifact-unsafe`). |
| `unstable` | **Refuse** (exit 3, `artifact-unstable`). |

The table is **total** over the accepted nine-value enum
(`docs/prds/PRD-artifact-validation-and-provenance.md:1765-1773`), and PIB-224
asserts totality mechanically.

The sidecar `artifacts/analysis.json` never causes a *readiness* refusal in the
accepted `--check` sense, exactly as in the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:437-441`). It does,
however, participate in coherence, because a sidecar is derived data whose
parent this command may be about to create (§6.1.3).

#### 6.1.2 The coherent-suffix rule — the only admissible missing sets

`spec.md` is generated from `analysis.md` (`internal/workflow/workflow.go:120-129`)
and `exploration.md` from both (`internal/workflow/workflow.go:165-172`). A
default run that generated `analysis.md` **underneath** an existing `spec.md`
would publish an analysis that the spec provably did not derive from, and
nothing on disk would record the mismatch. rev-0 permitted exactly that.

The default mode therefore generates only a **suffix** of the dependency chain
(ADR-035 D19).
The table is total over the eight present/absent combinations of the three
required artifacts (unsafe and empty states have already refused above):

| analysis | spec | exploration | Default-mode outcome |
|---|---|---|---|
| absent | absent | absent | generate all three (plus the sidecar, §6.1.3) |
| present | absent | absent | generate `spec.md`, `exploration.md` |
| present | present | absent | generate `exploration.md` |
| present | present | present | **no-op success** (§6.1.4) |
| absent | present | absent | **refuse**, exit 2, `incoherent-bundle-gap` |
| absent | absent | present | **refuse**, exit 2, `incoherent-bundle-gap` |
| absent | present | present | **refuse**, exit 2, `incoherent-bundle-gap` |
| present | absent | present | **refuse**, exit 2, `incoherent-bundle-gap` |

`incoherent-bundle-gap`'s remediation is self-contained and names both real
routes: `tpatch prepare <slug> --regenerate` (coherent whole-bundle
replacement, prior bytes archived), or author the missing file by hand and run
`tpatch prepare <slug> --manual`. It never suggests deleting a file. PIB-245 …
PIB-250 cover the four refusing rows, the message, and the totality guard.

#### 6.1.3 The sidecar rule — preservation is absolute

1. **A present sidecar is never written by the default mode.** Not when it is
   `present-nonempty`, not when it is `present-empty`, not when it is
   `invalid-structured`. `--regenerate` is the only route that replaces it
   (PIB-251, PIB-252).
2. **The sidecar is generated iff `analysis.md` is generated this run and the
   sidecar is `absent`.** Generating an analysis leaves the tree with a sidecar
   that derives from exactly the analysis being published.
3. **If `analysis.md` is absent but the sidecar is present in any state, the
   run refuses** with `incoherent-bundle-gap` (exit 2 for
   `present-nonempty`/`present-empty`/`invalid-structured`; exit 3 for the
   unsafe states, which refuse earlier as `artifact-unsafe`). A sidecar whose
   parent analysis does not exist describes an analysis this run is about to
   invent, and silently overwriting it is exactly the rev-0 defect (PIB-253,
   PIB-254).
4. **If `analysis.md` is preserved, the sidecar is left exactly as found —
   including left absent.** Synthesizing a sidecar for a preserved (possibly
   hand-authored) `analysis.md` would fabricate structured data that does not
   derive from the preserved bytes, and its presence would then be a false
   Path A signal, which
   `docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920` forbids
   (advisory `analysis-preserved-sidecar-untouched`).

**Consequence for the archive: the default mode has no `replace` entry, ever.**
Every entry it publishes is a `create` over an `absent` path. It therefore
writes no blob, appends no generation, and never creates
`artifacts/intent-archive/` (PIB-255, PIB-256). The archive semantics of every
mode are stated once, unambiguously, in §9.2.1.

#### 6.1.4 Nothing to do

If all three required artifacts are `present-nonempty` and the feature is
already `defined`, the command is a **no-op success**: exit 0,
`action: "none"`, zero bytes written anywhere, including `status.json`
(PIB-035). If they are all present but the state is below `defined`, the
command publishes **only** the `status.json` transition (§6.2's single-file
path) and reports `action: "adopt"` (PIB-036).

**Step 5 — generate, stage, validate, publish.** §11 then §7.

**Step 6 — final state.** `defined`, `last_command: "prepare"`, notes per
§12.3.

### 6.2 `tpatch prepare <slug> --manual` — Path B adoption

**Intent**: "I authored the bundle by hand; adopt it."

- **No provider is constructed, no network call is made, and no artifact byte is
  written.** PIB-045 asserts a provider spy records zero calls; PIB-046 asserts
  all four artifact files are byte-identical afterwards.
- The accepted inspector runs unchanged. Adoption proceeds **only** when the
  verdict is the accepted `ready` — all three required Markdown artifacts
  `present-nonempty` (`docs/prds/PRD-artifact-validation-and-provenance.md:432-436`).
- Not ready → refuse with the accepted `not_ready` semantics, **exit 2**, the
  full per-artifact report, and zero mutation (PIB-047, PIB-048).
- Ready → commit the transition: `state = defined`, `last_command = "prepare"`,
  notes per §12.3.

**One-file publication, rooted and CAS-gated.** `status.json` is the only
publication-set file. The command holds the per-slug lock, captures its full
rooted preimage while forming the new status bytes, writes a same-directory
`0600` temporary file through the held `*os.Root`, fsyncs and closes it, then
**immediately** re-captures `status.json` through that root. Only an exact
identity match with the preimage permits `(*os.Root).Rename(temp, status)` and
parent-directory fsync. A changed, appeared, disappeared, unsafe, or unreadable
status target refuses `status-changed` (exit 5) before the rename and before a
`FEATURES.md` refresh; the concurrent bytes survive. This helper preserves the
prior status permission bits, never calls `writeFileAtomic`/`SaveFeatureStatus`,
and is the D2 write primitive for this mode (PIB-399, PIB-400).

One rooted rename is atomic in the ordinary single-file sense. Consequently the
mode still has **no journal and no archive**: it has one publication-set entry,
not a multi-file rollback problem (ADR-035 D3). A crash leaves that entry old or
new, never torn. The CAS is nevertheless required because single-file atomicity
does not authorize clobbering a concurrent status edit.

**The publication set is not the process's whole write footprint.** After a
successful status rename, `prepare` invokes the existing derived-index behavior
through a rooted best-effort `FEATURES.md` helper. It is not journaled, CASed or
covered by T1; §12.3.1 keeps that carve-out. The `--manual` write set is thus
`{status.json} ∪ {FEATURES.md, best-effort}`, but no path-based writer is an
exception to D2.

### 6.3 `tpatch prepare <slug> --regenerate` — archived coherent overwrite

**Intent**: "Throw away the current bundle and produce a coherent new one."

**Scope decision: all three Markdown files plus the sidecar, always.**

Partial regeneration is not offered, and the reason is coherence, not
convenience: `spec.md` is generated from `analysis.md`
(`internal/workflow/workflow.go:120-129`) and `exploration.md` from both
(`internal/workflow/workflow.go:165-172`). Regenerating `spec.md` against an
old `analysis.md` and calling the result a bundle would produce a set whose
parts describe different intents, and nothing on disk would record the
mismatch. Q1 records a future `--only <ids>` as an additive, enumerated
extension.

**Generation authority.** `--regenerate` **requires a configured provider and a
successful provider result for every artifact**, unless `--allow-heuristic` is
passed. Without a provider it refuses **before any mutation** (exit 3,
`provider-required-for-regenerate`); on a provider failure or deadline expiry it
refuses **before publication** (exit 5, `regenerate-generation-failed`), with
zero canonical mutations and zero blobs written. §11.3 states the full policy
and why silent heuristic replacement of a hand-authored bundle is the one
fallback this design refuses to make.

**Preservation.** Before the first canonical mutation, every artifact whose
state is `present-nonempty`, `present-empty` or `invalid-structured` is scanned
under the ADR-027 D3 redaction contract (§9.6) and then copied into the intent
archive (§9). Artifacts that are `absent` archive nothing. Artifacts that are
`symlink-refused`, `not-regular`, `unreadable`, `oversize` or `unstable`
**refuse the whole invocation** (exit 3) — `--regenerate` will not overwrite
something it could not first safely read and archive. A redaction match also
refuses the whole invocation (exit 3,
`archive-content-refused-sensitive`), because exact-byte recovery and
scrubbing are mutually exclusive and archiving a secret silently is the
outcome ADR-027 D3 exists to prevent. This is the single most important
refusal in the document: an overwrite route whose preservation step can
silently fail — or can silently retain a credential forever — is not a
preservation guarantee (PIB-060 … PIB-066, PIB-262 … PIB-267).

**Lifecycle.** Identical to §6.1's step 6.

### 6.4 `--dry-run` — plan, not outcome

`--dry-run` is available in `generate`, `manual` and `regenerate` modes.

- It performs the preflight inspection and computes the publication set.
- It writes **nothing**: no external-lock creation, no journal, no staging tree, no
  directory, no `status.json`. PIB-072 asserts a filesystem spy records zero
  create/write/rename/mkdir calls of any kind.
- It makes **no provider call** (PIB-073).
- It exits `0` when the plan is admissible, and with the same refusal code the
  real run would produce when it is not (PIB-074 … PIB-076).
- Its report carries `dry_run: true` and the planned `actions` array, and it
  states verbatim: `Plan only. Generation was not attempted and may still
  fail.` Because it cannot run the generators, it is a statement about
  admissibility, never about outcome (PIB-077).

**Pending recovery makes the plan unknowable, so `--dry-run` refuses.** A
mutating run performs recovery first (§7.11), and recovery can undo published
entries, which changes the artifact states the plan is computed from. rev-0 let
`--dry-run` skip recovery and still print a plan — a plan for a tree that would
no longer exist by the time the real run computed one. rev-1 refuses instead:

- If a journal exists for the slug, `--dry-run` exits `3` with refusal code
  `recovery-pending`, writes nothing, does not take the lock, and does not
  touch the journal (PIB-079, PIB-268, PIB-269).
- The remediation names the two real routes: run the mutating command (which
  recovers first), or `tpatch prepare <slug> --abandon-transaction --yes` when
  the operator wants the slug unblocked without any canonical change.
- Simulating the post-recovery plan was evaluated and rejected: it would
  require *deciding* recovery's outcome without holding the lock and without
  performing it, which is exactly the class of guess §7.11 refuses to make
  (§21).

### 6.5 `--check` — unchanged

See §12.1. The accepted report bytes, schema, exit codes and zero-mutation
contract are preserved exactly, and PIB-198 … PIB-205 are the regressions that
prove it.

### 6.6 `--abandon-transaction` — the safe way out of exit 6

**Intent**: "An interrupted or divergent run has pinned this slug; unblock it
without touching my files."

Exit 6 must never be a permanent state (ADR-035 D13). rev-0's only escape was hand-deleting a
gitignored journal, which is an undocumented, unguarded operation an operator
would perform under stress. This mode makes it explicit, previewable and
canonical-safe.

**Contract.**

1. Takes the transaction lock (§7.4). A live sibling `prepare` therefore
   refuses it — exit 3, `transaction-in-progress` — instead of pulling control
   state out from under a running process.
2. Refuses with exit 3, `no-pending-transaction`, when no journal, no staging
   tree and no abandoned-evidence directory exist for the slug.
3. **Touches no canonical file. Ever.** No artifact, no `status.json`, no
   archive index, no blob is created, modified or removed by this mode. It is a
   `.tpatch/local/` operation only, and PIB-270 asserts the whole
   `.tpatch/features/` subtree is byte-identical afterwards.
4. **Preserves the evidence rather than deleting it.** The journal, both
   metadata preimages and every `stage-*` tree for the slug are **moved** into
   `.tpatch/local/intent-prepare/<slug>/abandoned-<12hex>/`, whose name uses the
   shipped `RandomHex12` (`internal/store/fsdurable.go:96-103`) and no clock.
   The report names that directory and states, in one line, that removing it is
   `rm -rf <dir>` and that nothing under it is tracked (PIB-271, PIB-272).
5. **Preview by default.** Without `--yes` it prints exactly what it would move
   and exits 0 having written nothing (PIB-273). With `--yes` it performs the
   move and exits 0.
6. After it completes, the slug is ordinary: the next mutating `prepare` finds
   no journal, runs its own preflight, and refuses or proceeds on the evidence
   of the actual files (PIB-274).
7. It never removes a lock. With a process-lifetime lock there is no stale lock
   to remove (§7.4), so the one operation that could have destroyed another
   process's mutual exclusion does not exist in this design.

**What it deliberately does not do.** It does not restore preimages, does not
re-run recovery, and does not decide anything about the artifacts. Restoration
from a divergent state is a human judgement — the operator has the archive
blobs (for `--regenerate`) and the moved preimage files (for the two metadata
files), both named in the exit-6 report and again in this mode's report.


## 7. The publication transaction

### 7.1 What is guaranteed, and what a filesystem cannot give

Three distinct properties are routinely conflated under the word "atomic".
This PRD separates them, claims exactly two, and scopes the one that rev-0
overstated.

| | Property | Claimed? | Mechanism / why not |
|---|---|---|---|
| **T0** | **Instantaneous multi-file visibility.** A concurrent reader observes either the complete old set or the complete new set, at every instant. | **NO** | POSIX offers no multi-file atomic rename. Publishing six files is six independent `rename(2)` calls; between call *k* and *k+1* a concurrent reader observes a mixed set. No journal, lock or fsync changes this. Any claim otherwise would be false, so this PRD makes none. |
| **T1** | **Command-owned final verification.** At a verification point under the transaction lock, after the last rename and before the command returns, **every publication entry's current identity equals its intended new image** (exit 0) — or the command reports divergence and does not claim success. On any refusal or in-command failure, every entry equals its preimage at that same verification point, or the command exits 6 naming what it could not restore. | **YES, scoped** | Semantic CAS before every publish and every undo (§7.6), plus a final set-level verification pass under the lock (§7.8 step 10). |
| **T2** | **Crash recoverability.** After a kill, panic or power loss anywhere in the window, the *next* mutating `prepare` restores the complete old set, or refuses and preserves everything. | **YES, bounded** | The durable undo journal plus evidence-based recovery (§7.5, §7.10, §7.11), bounded by §7.11.1's explicit journal-loss boundary. |

**What T1 does not say.** It does **not** say the world is all-new after the
command returns. It says that at a point this command owns — under its lock,
after its last write — the set matched the intended images. A process that
holds no lock (an editor, `tpatch define`, `git checkout`) may write any of
those paths one microsecond later, and no postcondition of this command can
survive that. rev-0's unconditional "at the command boundary the tree is
entirely the new bytes" was therefore withdrawn: it claimed a property about
the *world*, which this command cannot own, rather than about its own final
verification, which it can.

**External writes inside the window are detected where they are observable, and
they produce divergence, not destruction.** Every entry is CAS-checked
immediately before it is published and immediately before it is undone
(§7.6). A third-party write that lands before an entry's CAS is caught and the
transaction refuses or rolls back; a write that lands after an entry's CAS but
before its rename is overwritten (the disclosed content race, §7.7.2, ADR-035
D6); a write that lands after the final verification point is simply outside
the postcondition and is stated as such. In no case does the command *undo*
third-party bytes: an undo whose CAS fails refuses that entry and exits 6 with
everything preserved (PIB-112, PIB-275, PIB-276).

**The T0 exposure window, stated exactly.** It opens at the first canonical
rename and closes at the last. Inside it a concurrent reader — `tpatch status`,
`tpatch next`, `prepare --check` in another process, an editor, `git status` —
may observe any prefix of the publication order (§7.8) applied and the rest not.
The window contains no provider call, no network I/O, no subprocess and no
user-visible prompt: it is *N* renames, *N* CAS reads and *M* directory fsyncs,
where *N ≤ 6*. That is the honest bound — a small number of syscalls, not zero
time.

**What the accepted read-only check does inside the window.** It reports what is
actually on disk, which is the truthful answer. Its own instability probes
(`docs/prds/PRD-artifact-validation-and-provenance.md:1827-1917`) may
legitimately classify a file being renamed over as `unstable`. That is correct
behavior and is not a defect of either contract (PIB-206).

### 7.2 The publication set

Computed at preflight, frozen before the journal is written, bound into the
journal by `plan_digest` (§7.5), and never recomputed. Each entry is
`(artifact_id, canonical root-relative path, action, preimage, new_image)` with
`action ∈ {create, replace}`.

| Order | Entry | Present when | Possible actions |
|---|---|---|---|
| 1 | `analysis.md` | it is generated this run | `create` (generate mode), `create`/`replace` (regenerate) |
| 2 | `spec.md` | it is generated this run | same |
| 3 | `exploration.md` | it is generated this run | same |
| 4 | `artifacts/analysis.json` | `analysis.md` is generated this run (§6.1.3) | `create` in generate mode; `create`/`replace` in regenerate |
| 5 | `artifacts/intent-archive/index.json` | at least one entry has `action = replace` — i.e. **only in `regenerate` mode** (§6.1.3) | `create`/`replace` |
| 6 | `status.json` | always, in every mutating mode that reaches publication | `replace` (or `create` for a legacy tree with no status file, which refuses earlier as `feature-not-found`) |

Every entry — including entries 5 and 6, the two *metadata* paths — carries a
full preimage **and** a full intended new image, and every entry is CAS-checked
on publish and on undo. rev-0 modelled the two metadata files as raw
side-preimages outside the entry model, which is precisely why its rollback
could not state a CAS rule for them (PIB-277, PIB-278).

Archive **blob** files are not publication-set entries: they are additive,
immutable, content-addressed and written before the window opens (§9.2). A blob
left by a crashed or refused attempt is an **orphan** — normal, harmless, and
now explicitly **reported and removable** (§9.7.3), unlike rev-0, where it was
permanent by design.

`status.json` is **last**, deliberately (ADR-035 D7). If a crash lands between entry 5 and
entry 6, the tree holds new artifacts and the old state — a strictly recoverable
combination, because recovery's undo has every preimage it needs and the state
has not yet claimed the bundle is complete. The reverse order would publish a
`defined` claim over artifacts that might never land.

### 7.3 Staging

- Root: `.tpatch/local/intent-prepare/<slug>/stage-<12hex>/`, mode `0700`,
  suffix from the shipped `RandomHex12` (`internal/store/fsdurable.go:96-103`).
- Created with `MkdirAllAndSyncChain` so the whole chain is durable even when
  intermediate directories already existed
  (`internal/store/fsdurable.go:41-52`, and the rationale at
  `internal/store/fsdurable.go:1-10`).
- Before any byte is written under `.tpatch/local/`, the local-lane contract
  gate of §7.13 runs. Where Git is available it is the shipped two-target gate
  (`internal/rescap/scratch.go:47-62`); where the workspace is not a Git
  worktree it is inapplicable and skipped with an advisory; where Git is
  expected but unusable the command refuses (exit 3,
  `local-lane-unverifiable`) — PIB-186, PIB-187, PIB-279 … PIB-283.
- Each staged file is written, `Sync`ed and closed; then the stage directory is
  `Sync`ed. Staged bytes are therefore durable before the journal names them.
- **Raw provider responses stay here.** They are written into the staging tree,
  never into `artifacts/` (§11.4). This is an enumerated delta from
  `analyze`/`define`/`explore` (§12.6 D3).
- **Cleanup.** On success the staging tree is removed and its parent `Sync`ed.
  On failure it is **retained** and its repo-relative path is named in the
  failure report, so a failed generation is inspectable. Retained trees are
  removed by the next successful mutating `prepare` for the same slug, by
  `--abandon-transaction` (which moves them under `abandoned-<12hex>/`), and by
  nothing else (§7.12, PIB-098, PIB-099).

### 7.4 Lock: a kernel, process-lifetime advisory lock

#### 7.4.1 Why the in-worktree lock was withdrawn

rev-0's `O_EXCL` lock was unprovable because a surviving pathname said nothing
about whether its owner lived. rev-1 improved ownership by using a kernel lock,
but retained its **locator** at
`.tpatch/local/intent-prepare/<slug>/prepare.lock`. That makes the kernel lock
split: while process A holds inode A, `git clean -xfd` can unlink that ignored
path; process B can then create inode B at the same pathname and lock it. Both
processes believe they hold the per-slug authority. `git checkout` and `git
stash` do not normally remove ignored paths, but the design may not depend on
that distinction when `git clean` is in its cleanup adversary set.

A lock file is still not liveness. The error was treating a **worktree pathname**
as the stable locator of an otherwise sound kernel authority. Rev-2 keeps the
kernel primitive and replaces the locator.

#### 7.4.2 The authority and locator

**Authority.** The authority is one OS-held advisory lock on one persistent,
zero-byte file in a **per-effective-OS-user cache namespace outside every
workspace**. It is the only transaction authority for `generate`, `manual`,
`regenerate`, `abandon`, and archive `purge`; `.tpatch/local/` contains staging
and journals only and contains no lock file.

**Locator construction (fixed v1 algorithm).** Before any local-lane or
canonical write, the mutator must:

1. resolve the discovered workspace root with `filepath.Abs` then
   `filepath.EvalSymlinks`; failure, a non-directory result, or inability to
   obtain its native directory identity is `lock-authority-unavailable` (exit
   3), before mutation;
2. encode the resulting native root identity in a fixed binary form: on Linux
   and Darwin `(st_dev, st_ino)`; on Windows `(volume serial number, file
   index)`. The canonical absolute path is retained only for diagnostics in
   memory and is **not** put in the key or lock file;
3. compute `root_key = SHA-256("tpatch-intent-lock-v1\x00" || GOOS || native-root-identity)`
   and `lock_key = SHA-256(root_key || "\x00" || canonical-slug)`, both lower
   hex; and
4. open exactly `intent-locks/v1/<lock_key>.lock` below the platform user-cache
   base returned by `os.UserCacheDir`. The raw root path, slug, hostname, PID
   and journal nonce never reach its pathname or contents.

The cache base is canonicalized before use. `tpatch/intent-locks/v1` is created
`0700`; every component **owned by tpatch** is `Lstat`ed as a real directory,
not a symlink or reparse point, and is traversed through a held cache `*os.Root`.
The lock file is created/opened through that root at `0600`, is a regular file,
and remains zero bytes. Any unavailable, relative, non-private, substituted or
unsafe app-owned cache component fails closed with
`lock-authority-unavailable`. The OS-selected cache base itself is a user
platform precondition, not a claim that tpatch can secure arbitrary hostile
home-directory ancestors.

Native identity makes all ordinary path aliases to the same root — including a
symlinked invocation path and a bind/worktree alias that reports the same native
root object — select one file. SHA-256 collision would serialize unrelated
roots, which is fail-safe; it cannot create a split. Namespace scope is the
**same effective OS user on the same machine**. Different OS accounts or a
cache manager/manual deletion outside this process's control are not covered;
tpatch makes no cross-user or distributed-filesystem mutual-exclusion claim.

| Platform | Mechanism | Contention | Release / support boundary |
|---|---|---|---|
| `linux`, `darwin` | `syscall.Flock(fd, LOCK_EX\|LOCK_NB)` on the persistent cache lock descriptor — the shipped primitive (`internal/rescap/lock_unix.go:63-88`) | `EWOULDBLOCK` / `EAGAIN` | descriptor close, including `SIGKILL`, panic or power loss (`internal/rescap/lock_unix.go:43-52`); the existing exact `statfs` allowlist is applied to the **cache authority namespace**, before its lock file is created |
| `windows` | new deny-share `CreateFile(OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL\|FILE_FLAG_OPEN_REPARSE_POINT)` on the cache lock handle | `ERROR_SHARING_VIOLATION` (32) / `ERROR_LOCK_VIOLATION` (33) | OS closes the handle on process exit; implementation must accept only a local fixed cache volume and has a native `windows-latest` contention/unlink fixture |
| every other target, including `freebsd`, `netbsd`, `openbsd`, `dragonfly`, `solaris`, `aix`, `js`, `plan9`, `wasip1` | none | — | refuse `prepare-unsupported-platform` before opening a root or creating a cache/worktree byte |

The current shipped lock and statfs code is Linux/Darwin-only
(`internal/rescap/lock_unix.go:1-32`, `internal/rescap/statfs_linux.go:1-96`,
`internal/rescap/statfs_darwin.go:1-75`). BSD is deliberately not admitted by
an unimplemented statfs/CI promise. `prepare --check` remains the separately
accepted `unix || windows` read-only envelope.

**No automatic cleanup.** A released cache lock file is inert but deliberately
persists. Deleting it after close races a process that opened the old inode and
would recreate the split. Git cannot name the cache namespace: `git clean`,
`checkout`, and `stash` therefore cannot unlink/recreate the live authority.
An operator/cache cleaner that deletes the external file while a holder lives is
outside this contract and can split it; documentation must say so rather than
call it a stale-lock recovery route. There is no `--force`, stale-lock removal,
or automatic cache-prune command in v1.

**Workspace topology.** The key is the `.tpatch` workspace root, not `.git` or
a Git common directory. Separate Git worktrees/submodules with distinct
workspace roots intentionally do not serialize; the same physical workspace
reached through aliases does. A nested repository with its own `.tpatch` has a
new root/key; one without it belongs to the outer discovered workspace. Git
identity is not used in the key, so the algorithm is equally defined for a
non-Git workspace.

#### 7.4.3 Liveness and contention

The kernel decides liveness: acquisition succeeds when no holder has the same
cache file open, or returns a contention error while a holder does. A dead
process releases its descriptor; its persistent zero-byte cache file remains
inert. Contention is a transient exit 3, `transaction-in-progress`, and creates
no worktree artifact. This covers process death without requiring a journal.

The authority excludes only another mutating `prepare` or archive purge for the
same `(workspace root, slug, effective user)`. It excludes neither `define`,
`cycle`, editors, scripts, another OS user nor another machine. Those writers
are handled only by revalidation/CAS (§7.6); see §7.7.2 for the residual.

#### 7.4.4 Executable authority cases

The implementation must exercise all of these against separate real processes,
not just an in-process mock: plant the old ignored lane-path decoy, hold the
external authority, run `git clean -xfd` (which unlinks the decoy), recreate
that decoy, and start a contender — it must contend on the **same cache inode**,
not lock the recreated lane inode. The analogous `checkout` and `stash` fixture
must also leave the authority intact. Holder death followed by acquisition and
two live holders contending are separate cases. It additionally tests symlink-alias key equality,
worktree/submodule distinct-root behavior, opaque key naming, cache-directory
symlink refusal, and the Windows counterpart. PIB-395…PIB-398 are the binding
rows. No fixture claims safety against an external cache manager deleting the
cache file while a holder lives.

### 7.5 Journal: location, schema, binding, and why it is undo-only

**Location**: `.tpatch/local/intent-prepare/<slug>/journal.json`, mode `0600`,
written with the shipped durable writer `gitutil.DurableWriteFile`
(`internal/gitutil/index_snapshot.go:455-500`) and read with a **strict**
decoder — the discipline at `internal/cli/land_journal.go:348-380` and
`internal/store/resource_publish.go:305-320`.

**Location rationale**: gitignored `.tpatch/local/`, because the journal is
transient control state, is machine-local, and must never enter a commit. It
carries **no artifact content** except two small machine-written preimages
(§7.6.2). §7.11.1 states the boundary this location creates.

```json
{
  "version": 1,
  "slug": "fix-model-id-translation",
  "mode": "regenerate",
  "run_nonce": "9f13a0c4b7e25d81",
  "plan_digest": "6b1f…",
  "stage_rel": ".tpatch/local/intent-prepare/fix-model-id-translation/stage-a1b2c3d4e5f6",
  "entries": [
    {
      "artifact_id": "analysis",
      "rel": ".tpatch/features/fix-model-id-translation/analysis.md",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 4211, "mode": 420 },
      "preimage_blob": "a91e…",
      "new_image": { "exists": true, "sha256": "…", "size": 5017, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/analysis.md"
    },
    {
      "artifact_id": "archive_index",
      "rel": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/index.json",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 512, "mode": 420 },
      "preimage_raw_rel": ".tpatch/local/intent-prepare/…/index.preimage.json",
      "new_image": { "exists": true, "sha256": "…", "size": 883, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/index.json"
    },
    {
      "artifact_id": "status",
      "rel": ".tpatch/features/fix-model-id-translation/status.json",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 604, "mode": 420 },
      "preimage_raw_rel": ".tpatch/local/intent-prepare/…/status.preimage.json",
      "new_image": { "exists": true, "sha256": "…", "size": 641, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/status.json"
    }
  ]
}
```

**Strict decode and binding — a journal is used only when it proves it is
ours.** Every one of these is checked before a single byte is restored, and any
failure refuses with exit 6, preserves everything, names the journal path and
points at §6.6's abandon route (PIB-298 … PIB-305):

| # | Bind | Failure code |
|---|---|---|
| J1 | exactly one JSON value, then EOF; no trailing content | `journal-corrupt` |
| J2 | no unknown field at any nesting level | `journal-corrupt` |
| J3 | `version` equals this build's constant — refused, never guessed (`internal/cli/land_journal.go:56-58` is the precedent) | `journal-version-mismatch` |
| J4 | `slug` equals the invocation's validated slug | `journal-foreign` |
| J5 | `mode` is in the closed set `{generate, manual, regenerate}`; `manual` is impossible and refuses | `journal-corrupt` |
| J6 | `run_nonce` is exactly 16 lowercase hex bytes | `journal-corrupt` |
| J7 | every `rel` is a `fs.ValidPath` root-relative name inside `.tpatch/features/<slug>/`, and every `preimage_raw_rel` / `staged_rel` is inside that slug's `.tpatch/local/` lane — the shipped containment check (`internal/cli/land_journal.go:383-415`) | `journal-path-escape` |
| J8 | `plan_digest` equals the SHA-256 recomputed over the canonical encoding of `entries` (ids, paths, actions, preimages, new-images) | `journal-forged` |
| J9 | every entry's `artifact_id` is in the closed six-value set and appears at most once | `journal-corrupt` |
| J10 | `entries` is non-empty and is not `null` — the shipped no-null-arrays rule (`internal/store/resource_publish.go:305-320`) | `journal-corrupt` |

J8 is what makes forgery pointless: an attacker who edits an entry's preimage
to point at content they control invalidates the digest, and recovery refuses
rather than restoring attacker-chosen bytes over a canonical file. A journal
that survives J1–J10 still cannot cause a write, because every restore is
additionally CAS-gated on the file's *current* identity (§7.6).

- **No wall-clock field.** ADR-027 D6 and ADR-033 D10 forbid it in tracked
  artifacts; this PRD applies the same rule to the journal so that two
  identical operations produce identical journals, which is what makes
  PIB-160's determinism test possible. `land`'s journal does carry
  `created_at` (`internal/cli/land_journal.go:109`); this design deliberately
  does not, and ADR-035 D5 records the divergence and its reason (recovery
  decides from evidence, never from time).
- **No content**, with two enumerated exceptions (§7.6.2).
- **`Phase` is deliberately absent.** `land` keeps one and marks it advisory
  (`internal/cli/land_journal.go:110-111`); this design omits it entirely so
  no reader can be tempted to decide from it.
- **`run_nonce` is not lock ownership.** It exists only so a report can name
  which interrupted run produced the state an operator is looking at. Lock
  ownership is the kernel's, not a field's (§7.4).

**Undo-only, and why.** `land` must roll *forward* because `git commit` already
advanced HEAD, an irreversible act (`internal/cli/land_journal.go:11-23`).
Nothing in a prepare publication is irreversible: the new artifacts did not
exist before, and regenerating them is a re-runnable operation. So recovery
never completes a partial publication. It restores the old set and lets the
operator re-run. This is simpler, has strictly fewer failure modes, and cannot
publish a bundle whose staged half was pruned (ADR-035 D5).

### 7.6 Identity, and the semantic compare-and-swap

#### 7.6.1 Identity

Identity is `(exists, sha256, size, mode)`; equality requires all four (or both
non-existent).

**Citation correction.** The shipped `landJournalFileState` is
`(exists, sha256, mode)` — it carries **no size**
(`internal/cli/land_journal.go:65-79`). rev-0 claimed size was part of the
shipped shape; it is not. This design **extends** the shipped shape with
`size`, deliberately: size is a cheap mismatch signal that catches truncation
and growth before a hash is computed, and it is *compared*, never *trusted* —
equality still requires the SHA-256 (PIB-306).

- **Canonical artifact preimages** are recovered from the intent archive
  (§9.2): the blob is written and fsynced *before* the journal is finalized, so
  by the time the window can open, every replaceable byte already has a durable
  copy at a content-addressed path. The journal stores the blob's hash, not its
  bytes.
- New-image identities are computed from the staged files after they are
  fsynced, so the journal cannot describe bytes that are not durable.

#### 7.6.2 The two raw metadata preimages

`index.json` and `status.json` prior bytes are stored raw under the journal
directory (`index.preimage.json`, `status.preimage.json`, `0600`, gitignored,
removed with the journal), because neither belongs in a tracked,
content-addressed *intent* archive: archiving the archive index in the archive
is circular, and `status.json` is lifecycle metadata, not intent content
(ADR-035 D11).

**They are still full publication-set entries** with `preimage`, `new_image`
and CAS treatment identical to every artifact entry (§7.2). The raw file is
*where the bytes live*, not *whether the entry is modelled*. This is the rev-0
gap that made metadata rollback unspecifiable.

#### 7.6.3 The CAS rule — total over every mutating step

Before **every** step that changes a byte, the target's current identity is
recomputed through the held root and compared:

| Step | Expected current identity | On mismatch |
|---|---|---|
| publish entry *i* (`create`) | **absent** | abort before the rename; exit 5, `entry-appeared`; roll back what is published |
| publish entry *i* (`replace`) | equals `preimage` | abort before the rename; exit 5, `entry-changed`; roll back what is published |
| undo entry *i* (`create`) → remove | equals `new_image` | **refuse that entry**; exit 6, `undo-cas-mismatch`; the file is left exactly as found |
| undo entry *i* (`replace`) → restore | equals `new_image` | **refuse that entry**; exit 6, `undo-cas-mismatch`; the file is left exactly as found |
| restore `index.json` / `status.json` | equals that entry's `new_image` | same as above |
| `--manual` `status.json` rename | equals the rooted status preimage immediately before rename | exit 5, `status-changed`; no rename or derived-index refresh |
| write archive blob `<h>.blob` | absent, **or** present with bytes hashing to `<h>` | exit 3, `archive-blob-corrupt`; never overwritten |
| publish/rehydrate `index.json` | equals the index entry's `preimage` | exit 5, `archive-index-changed`; nothing written |
| purge `index.json` | strict-decoded raw identity equals the captured purge index preimage immediately before rooted rename | exit 3, `archive-index-changed`; remove no blob |
| purge a blob (§9.7) | present with bytes hashing to its name | exit 3, `archive-blob-corrupt`; nothing removed |

Two invariants follow, and both are asserted rather than asserted-about:

1. **A concurrent edit is never clobbered by an undo.** If a third party
   rewrote a published file, its identity no longer equals the new image, so
   the undo refuses and the bytes survive (PIB-112, PIB-275).
2. **A forged or corrupt journal cannot cause a write.** J1–J10 refuse it
   before recovery starts, and even a journal that passes them cannot restore
   over a file whose current identity does not match the exact image that step
   expects (PIB-303, PIB-307).

The precedent is shipped: `PublishBatch` compares the *semantic body* of an
existing immutable file before deciding to reuse or refuse it, and never
rewrites it in place (`internal/store/resource_publish.go:230-285`,
`internal/store/resource_publish.go:358-399`).

### 7.7 Rooted publication writes

#### 7.7.1 Every write is handle-relative

rev-0 performed rooted *reads* and then a path-based `os.Rename`, leaving an
ancestor-swap race: a directory component could be replaced between the check
and the rename, and the rename would then land outside the intended tree. The
pinned toolchain (`go 1.26.1`, `go.mod:3`) provides `(*os.Root).Rename`, so
that gap has no reason to exist.

| Operation | Primitive | Note |
|---|---|---|
| create a temp file in the destination directory | `(*os.Root).OpenFile(tmpRel, O_CREATE\|O_EXCL\|O_WRONLY, 0600)` | `O_EXCL` so a pre-existing temp name never gets written through; name is `.<base>.tmp-<12hex>` inside the destination directory, so the rename is same-filesystem (the shipped rationale at `internal/store/store.go:884-886`) |
| publish | `(*os.Root).Rename(tmpRel, destRel)` | replaces the destination in one syscall; never follows a final-component symlink |
| pre-publish CAS read | `(*os.Root).Lstat` + the accepted bounded rooted capture | ADR-034 D1/D8/D9, unchanged |
| create a directory | rooted component-by-component `(*os.Root).Mkdir` helper | `0755` tracked, `0700` local; no path-based `MkdirAll` |
| remove (undo of a `create`, temp cleanup) | `(*os.Root).Remove` | |
| fsync a directory | `(*os.Root).Open(dirRel)` then `Sync()` | best-effort on platforms where directory fsync is not meaningful, matching the shipped writer (`internal/store/store.go:910-914`) |

**No `os.Rename`, `os.CreateTemp`, `os.WriteFile`, `os.Remove`, `os.MkdirAll` or
`filepath.Join(repoRoot, …)` appears in a mutating prepare write path.** PIB-308
source-scans for exactly that, and PIB-309 is its sensitivity fixture.

#### 7.7.2 What this closes, and what it does not

- **Closed: escape from the held root.** `*os.Root` resolves the supplied
  relative name beneath its held root and rejects a resolution that would leave
  it. A raced ancestor replacement cannot redirect this publication outside the
  workspace root (PIB-310).
- **Not closed: in-root pathname redirection.** `os.Root` may follow a relative
  symlink that remains inside the root. An attacker can redirect `features/` or
  `<slug>/` to a different in-root directory after preflight; a rooted rename
  can then target that in-root alias. This is not an outside-root escape and no
  sentence may call it impossible. Pre-rename identity CAS detects the ordinary
  case where the resolved target differs from the preimage; it **cannot** prove
  that the canonical pathname designated the same object throughout, and cannot
  detect an in-root alias whose resolved regular file has exactly the expected
  preimage. PIB-406 exercises the detected redirect and guards the disclosure.
- **Closed only for the final leaf: writing through a final symlink.** The
  pre-rename rooted `Lstat` gate refuses a final symlink, junction, directory,
  FIFO, socket or device. This says nothing about a redirected ancestor.
- **Not closed: content identity.** Between an entry CAS and its rename, a
  concurrent writer can replace the resolved destination bytes; the rename can
  overwrite them. That is a residual content race. The in-root redirection
  limit above is a separate name-to-object residual; neither is described as a
  "closed pathname half" (ADR-035 D6).
- **Not used: the documented `Root.Chmod`, `Root.Chown`, and `Root.Chtimes`
  caveat.** This design calls none of them (PIB-311).

#### 7.7.3 The shipped single-file writer is not reused by `prepare`

`writeFileAtomicWithRename` (`internal/store/store.go:878-917`) creates its temp
with `os.CreateTemp` and renames path-based, so it remains unchanged for shipped
callers and is never a `prepare` writer. Both transactional publication and the
single-file `--manual` status publication use the rooted temp/identity/CAS
helpers in §7.7.1 and §6.2. The only non-transactional `FEATURES.md` refresh is
also routed through a rooted best-effort helper; its derivation/T1 carve-out,
not a path-based exemption, is §12.3.1. PIB-312 pins the shipped helper and its
old callers; PIB-313 proves no mutating `prepare` path calls it.

### 7.8 Publication order, CAS points, fsync and durability

Ordered algorithm. Every step is durable before the next begins.

**Pre-step (no mutation):** discover/validate the workspace and platform,
derive the prospective external-lock key without creating it, and run G1 then
the applicable G2/G3 local-lane gate. This is §10.5's precedence: an
unverifiable Git/privacy state refuses before **any** cache, local-lane, or
canonical byte. Only a successful Git gate (or a Git-established non-worktree)
may continue to step 1.

1. Acquire the lock (§7.4). Contention → exit 3, `transaction-in-progress`.
2. Run recovery (§7.11) **before** anything else.
3. Inspect (accepted inspector, ADR-034 boundaries). Compute and freeze the
   publication set. Refuse here for every §6 admissibility failure — nothing
   has been written except a possible zero-byte external cache locator.
4. Stage and validate generated bytes (§7.3, §11.6). Any failure here aborts
   with zero canonical mutations.
5. **Set-level revalidation**: every entry's canonical identity vs preflight.
   Mismatch → abort, exit 5, before anything is archived.
6. Scan every `replace` entry's prior bytes under the redaction contract
   (§9.6). A match refuses the invocation (exit 3) before any blob exists.
7. Write archive blobs for every `replace` entry; fsync each; fsync `blobs/`.
   Capture `index.preimage.json` and `status.preimage.json`; fsync them.
8. Write the journal; fsync it; fsync the journal directory. **The window is
   now armed.**
9. For each entry in the fixed order of §7.2: **CAS the entry** (§7.6.3), then
   `(*os.Root).Rename` staged → canonical, then fsync the entry's parent
   directory.
10. **Final verification** (T1's owning point): re-read every entry's identity
    under the lock and require equality with its new image. A mismatch here is
    reported as divergence — exit 6, `post-publication-divergence`, nothing is
    undone, everything is named — because at this point an undo would be
    destroying a third party's bytes (PIB-314, PIB-315).
11. Clear the journal (remove `journal.json`, both preimage files and the
    staging tree; fsync the journal directory).
12. Release the lock by closing the descriptor. The persistent external cache file remains inert (§7.4.2).

Step 11 is the point after which the transaction is invisible. Steps 8→9 are
the armed window; step 9 is the T0 exposure window (final T1 verification is step 10).

**File modes.** Created files use `0644` (the shipped default at
`internal/store/store.go:918-923`); replaced files preserve the existing
file's permission bits, matching the shipped writer's existing behavior
(`internal/store/store.go:871-876`). Directories use `0755` in the tracked tree
and `0700` in `.tpatch/local/`.

### 7.9 In-command rollback

If any CAS or rename in step 9 fails, the command rolls back **immediately**, in
reverse publication order, and every undo is itself CAS-gated (§7.6.3):

| Entry action | Undo | Guard |
|---|---|---|
| `create` | `(*os.Root).Remove` the canonical file | only if its identity equals the new image; otherwise refuse |
| `replace` | restore from the archive blob named in the journal, staged into the destination directory and renamed | only if its identity equals the new image; otherwise refuse |
| `archive_index` | restore from `index.preimage.json` | same guard |
| `status` | restore from `status.preimage.json` | same guard |

Then clear the journal and release the lock. Exit `5`, report
`outcome: "rolled-back"`, and state in one line what is true — which is
**not** "the tree is exactly as it was":

> Every canonical artifact, `status.json` and the archive index are back to
> their pre-run bytes. Archive blob files written by this attempt remain as
> orphans and are listed below; remove them with
> `tpatch feature intent-archive purge <slug> --orphans --yes`.

rev-0's rolled-back message claimed the whole tree was byte-identical while
additive blobs remained on disk. That is now a named, listed, removable
residue, and PIB-316 asserts the message enumerates every orphan blob it left
(PIB-317 asserts the count is exact).

If the rollback itself fails, or any undo CAS refuses, the journal is **kept**,
the lock is released, and the command exits `6` with a report naming the
journal path, the archive directory, the specific entry that could not be
restored, the blob or preimage file that holds its old bytes, and the §6.6
abandon route. No further automatic action is attempted (PIB-110, PIB-111,
PIB-112).

### 7.10 Crash phases — enumerated, each with its recovery outcome

The lock is process-lifetime, so in **every** row below the next invocation
acquires the lock without ceremony; there is no stale-lock analysis and no row
in which the slug stays blocked.

| Phase | Crash point | On-disk evidence | Recovery outcome |
|---|---|---|---|
| CP0 | before the lock exists | nothing | nothing to do |
| CP1 | external lock held, before journal | maybe a staging tree; persistent cache locator outside the worktree | acquire the same cache authority; remove staging trees; proceed |
| CP2 | blobs written, before journal | blobs, no journal; persistent cache locator outside the worktree | acquire; blobs are additive **orphans**, reported by `doctor` and removable (§9.7.3); proceed |
| CP3 | journal durable, before first rename | journal; all entries == preimage | clear journal; nothing to restore |
| CP4 | after rename *k* of *n* (`0 < k < n`) | journal; first *k* == new-image, rest == preimage | undo the *k* published entries (each CAS-gated); clear |
| CP5 | after the last artifact rename, before `index.json` | journal; artifacts new, index old | undo all published entries; clear |
| CP6 | after `index.json`, before `status.json` | journal; artifacts + index new, status old | undo all published entries including the index; clear |
| CP7 | after `status.json` rename, before journal clear | journal; **every** entry == new-image | **complete**: clear journal only; publish nothing, undo nothing |
| CP8 | after journal clear, before lock release | inert persistent cache locator only | acquire the same cache authority; proceed |
| CP9 | any of the above, plus a third party wrote one of the entries | at least one entry matches neither preimage nor new-image | **refuse** (exit 6, `recovery-divergent`), preserve every file, the journal and the archive; name them and name §6.6 |
| CP10 | `--manual` crash after the external lock, before or during the single `status.json` rename | persistent cache locator; `status.json` old or new; no journal | acquire; nothing to recover (single-file publication, ADR-035 D3); `FEATURES.md` may be stale and is reconverged by the next transition (§12.3.1) |
| CP11 | crash between an archive blob write and the journal write, on a **retry** of the same regeneration | journal absent; blob already present with matching bytes | acquire; the blob is reused, not rewritten (content-addressed idempotency, `internal/store/resource_publish.go:240-246`); proceed |
| CP12 | purge after rooted index rename, before blob removal | tombstoned index; still-present blob orphan | list/doctor report the orphan; later `purge --orphans` removes it; no index lies |
| CP13 | duplicate-generation rehydration after blob durability, before index rename | old tombstone plus an unreferenced/reused blob | retry strict-decodes and rehydrates by CAS; no duplicate generation is appended |

CP7 is why recovery decides from **evidence, not phase**: the process died after
the semantically final act, and no marker write could have made that
distinguishable without introducing its own crash point. This is the same
reasoning the shipped land journal records
(`internal/cli/land_journal.go:11-23`) and applies to the opposite direction.

Rows: PIB-116 … PIB-123, PIB-126, PIB-291 … PIB-295, PIB-318, PIB-319.

### 7.11 Recovery: entry points, idempotency, cleanup

**Entry points.**

- **Automatic**: every mutating `prepare` for that slug runs recovery under the
  lock before any other work, exactly as `land` does
  (`internal/cli/land_journal.go:445-482`).
- **Operator**: `tpatch prepare <slug> --abandon-transaction --yes` (§6.6) for
  the cases automatic recovery refuses.
- **Diagnostic**: `tpatch doctor` gains a check that *reports* pending or
  orphaned transaction state and never acts on it (§12.5).
- **Nothing else.** `--check`, `next`, `cycle`, `status`, `verify`, `record`,
  `land`, `reconcile` and the phase commands neither recover nor refuse on a
  pending journal (§7.14, PIB-118 … PIB-123). `--dry-run` refuses rather than
  guessing (§6.4).

**Why not "every relevant command must recover or refuse".** That alternative
was evaluated and rejected (§8.2 alternative J2): it would put a write path
into read-only commands, add a lock acquisition to eleven shipped verbs, and
make a stale journal able to wedge `tpatch status`. The chosen model is
**pointer-free and generation-free**: a pending journal is a fact about one
slug's `.tpatch/local/` lane, it never changes the meaning of any canonical
file, and the canonical tree remains self-describing without it. Because
recovery is undo-only, a never-recovered journal leaves the tree in a state
that is *already* one of the enumerated CP-phases — mixed, but classified
truthfully by the accepted read-only check.

**Idempotency.** Recovery is a function of evidence, so running it twice is a
no-op the second time: the journal is gone. A recovery interrupted mid-undo
leaves a state that is itself a valid CP-phase and is re-recoverable (PIB-113).

**Cleanup.** Recovery removes: the journal, both preimage files, the staging
tree named in the journal, and any other `stage-*` directories under that
slug's lane. It never removes archive blobs, never removes the archive index,
never removes an `abandoned-*` evidence directory, and never touches another
slug's lane (PIB-114, PIB-115, PIB-320).

#### 7.11.1 The T2 boundary: a lost journal is unrecoverable and usually undetectable

The journal is intentionally in the gitignored local lane. `git clean -xfd`, a
fresh clone, or an out-of-band deletion can remove it while a transaction is
pending. If that happens, tpatch has lost the undo evidence. It cannot recover
old bytes by guessing from canonical artifacts; for a regeneration, any archive
blobs that remain are merely the remaining recovery material.

**There is no general journal-loss detector.** A normal sequence of
`analyze`/`define`/`explore`, or a hand-authored partial bundle, can produce the
same mixed canonical artifacts as an interrupted prepare. No journal plus mixed
canonical bytes proves neither `prepare` nor journal deletion. `doctor` must
not label that ordinary lifecycle state an interrupted transaction and must not
offer recovery for it (PIB-407).

D9 reports only evidence-bearing **current residues** — a present journal,
staging/abandoned control evidence, an active external lock, an orphan blob, or
a corrupt archive index — and describes each without inferring why it exists.
A completely cleaned lane/archive after a lost journal is indistinguishable
from a workspace that never ran `prepare`; the loss is therefore an explicit,
undetectable T2 boundary. `git clean` and a fresh clone both take this ordinary
no-evidence branch; neither causes a journal-loss diagnostic (PIB-321,
PIB-322). The mandatory disclosure says this limitation, not that `doctor`
detects it (PIB-323).

**Rejected: journal in Git control state.** It would not survive a fresh clone,
would colonize Git's namespace across worktrees/submodules and `.git` files,
would require a second non-Git lane, and would not fix the external cache lock
locator. It is not a solution to the evidence-loss boundary.

### 7.12 Concurrency matrix

| Concurrent actor | Behavior |
|---|---|
| A second mutating `prepare`, same slug | The kernel lock contends: exit 3, `transaction-in-progress`; the first is unaffected (PIB-124, PIB-290). |
| A second mutating `prepare`, same slug, first process **killed** | The kernel released the lock on death; the second acquires it, recovers from evidence and proceeds (PIB-291). |
| A second mutating `prepare`, different slug | Independent lanes, no interaction (PIB-125). |
| `prepare --check`, any slug | Never blocked; reports what is on disk; may report `unstable` inside the window (PIB-206). |
| `prepare --dry-run`, same slug, transaction pending | exit 3, `recovery-pending`; nothing written; the journal untouched (PIB-268). |
| An editor writing `spec.md` between preflight and set-level revalidation | Detected at step 5 → abort before the window, exit 5, nothing written (PIB-100). |
| An editor writing `spec.md` between set-level revalidation and that entry's own CAS | Detected at the per-entry CAS → abort, roll back published entries, exit 5 (PIB-296). |
| An editor writing `spec.md` between its CAS and its rename | Overwritten; bytes not archived. Stated limit (§7.4.4), ADR-035 D6, PIB-103. |
| An editor writing an already-published entry before rollback reaches it | The undo CAS refuses; exit 6; the editor's bytes survive (PIB-112, PIB-275). |
| An editor writing an entry after the final verification point | Outside the postcondition (§7.1); the command has already returned 0 (PIB-324). |
| `tpatch define <slug>` concurrently | Not excluded (it takes no lock). Its write is caught by revalidation or by the per-entry CAS if it lands before that entry's rename, and lost if it lands inside the last few instructions. Same limit, same disclosure (PIB-104). |
| `tpatch cycle <slug>` concurrently | Same class as `define`; `cycle` publishes three artifacts sequentially, so it can be caught at more than one CAS point (PIB-325). |
| `git checkout` / `git stash` moving `.tpatch/` under the command | Same class as an editor: detected before or at a CAS, lost only inside the residual window. `prepare` holds no Git lock (PIB-105). |
| `git clean -xfd` during the window | Removes only the gitignored lane (journal and staging), never the external cache lock. The holder retains the same cache authority; later recovery has §7.11.1's evidence-loss boundary, which doctor cannot generally diagnose. |
| A Git worktree or a nested repository | `prepare` runs only the read-only Git commands of §7.13; it never writes the index, never commits, never touches `.git/**` (PIB-106, source-scanned by PIB-107). |

### 7.13 Git usage — exact, conditional, disclosed

Git is a real, read-only privacy dependency for the local-lane gate. Its
presence is **not** inferred from `.git` at the `.tpatch` root: that probe
fails for nested worktrees and makes a `.git` file/indirection look unlike a
repository.

#### 7.13.1 The exact invocations

| # | Command | Called from | When | Mode(s) |
|---|---|---|---|---|
| G1 | `git rev-parse --is-inside-work-tree` | a new tri-state workspace probe | first, with `cmd.Dir` exactly the discovered workspace root; no `.tpatch/local/` or cache write precedes it | every mutating mode and purge |
| G2 | `git check-ignore -q --no-index -- <lane-leaf>` | `workflow.EnsureLocalIgnoreContract` | only after G1 establishes Git | same |
| G3 | `git --literal-pathspecs ls-files -- .tpatch/local/` | `rescap.AnythingTrackedUnder` | only after G1 establishes Git | same |
| G4 | `git ls-files -- .tpatch` | untracked-archive advisory | only after G1 establishes Git | regenerate only |

All are read-only. None stages, commits, checks out, stashes, writes the index,
or modifies `.git/**`. PIB-107's exec spy checks this closed argv set and
PIB-326 is its sensitivity fixture.

#### 7.13.2 G1 is authoritative; the tri-state is fail-closed

G1 is run from the **workspace root**, not from a guessed `.git` path. It
returns exactly one classified result:

| G1 result | Meaning | Behavior |
|---|---|---|
| successful, trimmed stdout exactly `true` | Git worktree (ordinary repo, linked worktree, or submodule all qualify) | run G2/G3, and G4 only for regenerate; a gate failure refuses as before |
| Git cleanly establishes `false` / not-a-worktree | non-Git workspace | do not run G2–G4; proceed with `workspace-not-git` |
| executable lookup/spawn/cwd failure, unexpected output/exit, or any Git error not classified as the standard C-locale not-a-worktree result | Git presence or privacy status is unverifiable | refuse `local-lane-unverifiable`, exit 3, before cache/local/canonical mutation |

The implementation invokes Git directly with a C locale for G1 classification;
it must accept only the exact `true`/`false` protocol and the known Git
not-a-worktree result, never broad-match arbitrary stderr. Missing `git` is
therefore **not** a non-Git proof. This preserves non-Git operation only where
Git itself can establish it, while failing closed when a broken Git executable,
repository error, or hostile environment leaves the privacy gate unknowable.

A linked worktree's `.git` is commonly a file, and a submodule/nested workspace
may have its Git metadata above or indirect from `.tpatch`; G1 handles both.
The lock key deliberately does not use Git identity (§7.4.2), so Git topology
never changes lock ownership.

`bundle-untracked-in-git` is emitted only in the first row when G4 proves that
`.tpatch/` is untracked. PIB-327…PIB-329 are amended for the tri-state; PIB-408
covers a nested worktree/submodule that a local `.git` `Lstat` would misclassify.

#### 7.13.3 What Git is never used for

`prepare` never uses Git as preservation, recovery, lock, or content authority.
G4 is only an advisory; archive recovery does not depend on Git (§8.2 H2,
ADR-035 D8).

### 7.14 What the read-only check reports while a journal is pending

**Nothing about the journal.** This is a decision, not an omission:

1. The accepted `--check` report schema is frozen and its field set is closed
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2420-2576`). Adding a
   field would be a `schema_version` change to an accepted contract that this
   PRD is explicitly forbidden to modify.
2. `--check`'s subject is canonical artifacts. A journal is transient control
   state in a gitignored lane; it is not an artifact and has no state in the
   accepted nine-value enum.
3. What the operator actually needs — "is a prepare half-done here?" — is
   answered truthfully by `--check` anyway, because a mixed set reports as
   mixed, and by the new `doctor` check by name (§12.5).

So: `prepare --check` is byte-identical whether or not a journal exists, given
identical canonical bytes (PIB-207). The pending transaction is surfaced by
`doctor`, resolved by the next mutating `prepare`, and escapable through §6.6.

## 8. Non-destructive overwrite — the architecture gate

### 8.1 The question

`--regenerate` overwrites files a human may have hand-authored. The dispatch
constraint is absolute: **it must not lose prior hand-authored bytes.** The
question this section answers is *by what mechanism*, and whether that
mechanism is a persistent representation — because selecting one fires the
architecture gate and requires an ADR before acceptance.

### 8.2 Alternatives evaluated

| # | Option | For | Against | Verdict |
|---|---|---|---|---|
| **H1** | **Refuse overwrite entirely.** No `--regenerate`; the operator deletes files by hand first. | Zero new machinery; no new object; trivially safe. | Does not satisfy the dispatch, which requires an explicit coherent overwrite route. Pushes the destructive act onto `rm`, which has no archive at all — strictly *less* safe in practice. | **Rejected.** |
| **H2** | **Rely on Git.** Refuse `--regenerate` unless `.tpatch/features/<slug>/` is tracked and clean; recovery is `git checkout`. | No new object; uses a tool the user already has; zero storage growth. | `.tpatch/` is frequently untracked — the CLI ships a dedicated detector for exactly that condition (`internal/cli/cobra.go:3405-3407`) and warns about it. A Path B agent that authored three files five minutes ago has committed none of them. The guarantee would evaporate in the single most common case it is needed. It would also turn Git from a *conditional* read-only dependency (§7.13) into a hard precondition for the one mode that destroys bytes. | **Rejected as the sole mechanism.** Retained as a *reported* advisory (§10.3). |
| **H3** | **Ephemeral rollback-only journal.** Keep preimages only for the duration of the transaction; delete them at commit. | Satisfies T1/T2 with no durable growth; no new tracked object; no ADR. | Satisfies crash-safety and *not* the actual requirement: after a **successful** `--regenerate`, the prior bytes are gone forever. The dispatch's requirement is about the success path, not the crash path. | **Rejected for `--regenerate`.** Adopted for the transaction half (§7.5) — the journal *is* this, and it is sufficient there precisely because the archive covers the success path. |
| **H4** | **Durable immutable, content-addressed intent-generation snapshots; canonical files remain the sole authority.** Replaced bytes are copied to `artifacts/intent-archive/blobs/<sha256>.blob`; an `index.json` names which artifacts each generation replaced. No reader consults the archive to determine current state. | Prior bytes survive success, crash, and — once committed — clone and machine change. Content addressing dedupes: regenerating to identical content writes zero new bytes. Directly reuses the shipped, reviewed resource-capture shape — immutable content-addressed set plus one atomically-rewritten pointer (`internal/store/resource_publish.go:1-9,219-285`). Canonical readers are entirely unaffected. | Creates **durable retention of bytes the operator was previously destroying** (§9.6.2), so it requires the ADR-027 D3 redaction gate (§9.6.1) and a bounded removal surface (§9.7) as parts of the selection, not as follow-ups. Grows the tracked tree (mitigated: content-addressed, so only *distinct* content costs bytes; purge bounds it). Adds one tracked object and therefore **fires the architecture gate**. | **SELECTED, with §9.6 and §9.7 as conditions of the selection.** |
| **H5** | **Pointer-based generation directories.** Canonical paths become pointers/symlinks into `generations/<id>/`. | Cheap "switch the whole bundle" semantics; history is a first-class directory. | Breaks every existing reader: `os.ReadFile` on `spec.md` now depends on link resolution, and the accepted inspector **refuses symlinks by design** (`docs/prds/PRD-artifact-validation-and-provenance.md:1768`), so the bundle would classify as `symlink-refused` — the check would call every prepared feature unsafe. Also breaks Git checkout on Windows without developer mode. | **Rejected outright.** |
| **H6** | **Sub-record on `FeatureStatus` / a dedicated manifest carrying prior content or hashes.** | Written atomically with state by the one writer (`internal/store/store.go:368-377`); the ADR-031 D1 argument. | A hash is not the bytes: it proves change, it does not recover content. Inlining content into `status.json` bloats the hottest file in the feature directory and makes every state transition rewrite it. And a *hash of an artifact* is exactly the shape §9.8 must avoid being read as a provenance claim. | **Rejected.** |

### 8.3 Selection

**H4 is selected**, with the transaction half of H3 (§7.5) retained for
crash-safety and H2 demoted to an advisory (§10.3). The selection is
**conditional**: the redaction precondition (§9.6.1), the retention disclosure
(§9.6.2) and the listing/purge surface (§9.7) are part of what is selected. An
H4 without them is a different, unacceptable design — it retains secrets
silently and forever.

### 8.4 The gate fires — ADR-035 is required

H4 is a **persistent history representation**: a new durable, tracked object
with its own wire format, its own identifiers and its own lifecycle. Per the
dispatch and per AGENTS.md's "ADR on every architecture decision" rule, this
PRD therefore creates
[`ADR-035-intent-bundle-publication-and-history.md`](../adrs/ADR-035-intent-bundle-publication-and-history.md)
as **Proposed rev-2**, keeps it in the ADR index, and requires it to be
reviewed together with this PRD. **Neither may be accepted alone.**

### 8.5 What this selection is NOT

Two things must not be inferred from it, and both are asserted mechanically:

1. **It is not a provenance representation, and it does not fire the WP-005
   provenance trigger.** That trigger fires on "selection of a persistent
   provenance representation" and on nothing else
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972`). The
   archive records *bytes that existed at some point at a path*. It records no
   author, no agent, no model, no Path A/Path B tag, and no source claim. It
   cannot answer "who wrote this", and nothing in this PRD's output claims it
   can. `prepare --check` keeps emitting the constant `provenance: unknown` for
   every artifact, unchanged (§9.8, PIB-140 … PIB-144).
2. **It is not a licence to cite ADR-034 as precedent for persistence.**
   ADR-034 D14 forbids exactly that
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961`). ADR-034 is
   reused in this PRD for one thing only: the rooted, race-safe **read**
   boundary. Every write path in this document is governed by ADR-035
   (PIB-145).

## 9. The intent archive

Normative source: ADR-035 D8–D16. This section states the product-visible
contract.

### 9.1 Layout

```text
.tpatch/features/<slug>/artifacts/intent-archive/
├── index.json                     ← tracked manifest, canonical JSON, atomically rewritten
└── blobs/
    └── <64 lowercase hex>.blob    ← immutable, content-addressed prior bytes
```

The archive lives at tracked paths — but "tracked" is a property of the
operator's repository, not of this design, and §9.6.2 states exactly what
follows from that. Putting the archive in `.tpatch/local/` instead was rejected
because a recovery guarantee that vanishes on a fresh clone is not a recovery
guarantee (ADR-035 D8).

### 9.2 Blobs

- Name: the lowercase-hex SHA-256 of the file's exact bytes, plus `.blob`. The
  hash is over the **raw bytes**, not a canonicalization.
- Immutable **in content**: an existing blob whose bytes already equal the
  content is **never rewritten**; the publication simply reuses it.
  Content-addressed dedupe means regenerating to previously-seen content costs
  zero new bytes — the shipped idempotency argument at
  `internal/store/resource_publish.go:240-246`. Immutability means *no
  in-place modification*; it does **not** mean undeletable (§9.7).
- An existing blob file whose bytes **differ** from the content that hashes to
  its name is a corruption and refuses the invocation
  (`archive-blob-corrupt`, exit 3) rather than being overwritten — the shipped
  `batch-file-corrupt` disposition (`internal/store/resource_publish.go:198-202`).
- Written and fsynced before the journal is finalized, so a preimage is durable
  before it can be needed (§7.8 step 7).
- A blob written by an attempt that then crashed or refused is an **orphan**.
  Orphans are reported and removable (§9.7.3) — rev-0 called them "normal and
  permanent", which turned every failed attempt into unbounded, undeletable
  tracked growth.

#### 9.2.1 Archive semantics per mode — unambiguous, total

| Mode | Has `replace` entries? | Blobs written | Index entry appended | Archive directory created |
|---|---|---|---|---|
| `check` | — (read-only) | none | none | never |
| `generate` (default) | **never** (§6.1.3) | none | none | never (PIB-255, PIB-256) |
| `manual` | never (single-file publication) | none | none | never |
| `regenerate` without any present artifact | no (all `create`) | none | none | never (PIB-330) |
| `regenerate` with ≥1 present artifact | yes | one per distinct replaced content, except an already-retained duplicate | append one new generation, no-op for an already-retained duplicate, or **rehydrate** matching tombstones (§9.3) | on demand, lazily |
| `abandon` | — (no canonical write) | none | none | never |

There is exactly one route that ever archives, and it is the one the operator
typed `--regenerate` to reach.

### 9.3 `index.json`

```json
{
  "schema_version": 1,
  "feature": "fix-model-id-translation",
  "generations": [
    {
      "generation_id": "3f0c…",
      "mode": "regenerate",
      "replaced": [
        { "artifact_id": "analysis", "path": "analysis.md", "content_sha256": "a91e…", "blob": "a91e…", "size_bytes": 4211, "purged": false },
        { "artifact_id": "spec", "path": "spec.md", "content_sha256": "b12d…", "blob": "", "size_bytes": 1902, "purged": true }
      ]
    }
  ]
}
```

`content_sha256` is immutable identity, always the raw-byte SHA-256 that named
the original blob. `blob` is a present-storage locator: it equals
`content_sha256` while retained and is empty after purge. Thus purging removes
bytes without destroying the digest/body needed to validate the generation.
The canonical generation body is `{feature, mode, replaced[]}` sorted by
`artifact_id`, where every replaced item contains exactly
`artifact_id,path,content_sha256,size_bytes`; it excludes `blob` and `purged`.
`generation_id` is the SHA-256 of that immutable body. No map or wall-clock is
permitted; fixed struct order applies.

A newly generated body with an existing `generation_id` is not blindly a no-op.
The implementation first compares its immutable canonical body to the stored
one. A mismatch is `archive-generation-id-collision`. An equal body has two
deterministic cases:

1. no matching entry is tombstoned: duplicate no-op, no blob write and no index
   append; or
2. one or more matching entries are tombstoned: scan their source bytes under
   the redaction gate, write/reuse every required content-addressed blob, then
   CAS-publish one rooted index rewrite that sets only those matching entries to
   `blob: content_sha256, purged: false`. This is **rehydration**, not an
   appended generation; it leaves no duplicate-id no-op and no orphan.

A shared physical blob may serve multiple generations. Rehydrating one
reference does not un-tombstone another generation's reference; a later purge
uses the current non-tombstoned reference set before removal. Repeated
purge→rehydrate cycles therefore retain stable ids and deterministic index
order. Redaction is still checked before rehydration writes a blob; a refusal
leaves the tombstone and creates no blob. Blob durability precedes the rooted
index rename, so a crash before it leaves at most an orphan; after it, the
index never claims a missing referenced blob.

#### 9.3.1 Strict decoding — the index is never guessed at

Every index reader — publication, rehydration, purge, list, doctor — strictly
decodes one JSON value plus EOF. Failure writes nothing.

| # | Bind | Failure code | Exit |
|---|---|---|---|
| X1 | exactly one JSON value followed by EOF | `archive-index-corrupt` | 3 |
| X2 | no unknown field at any nesting level | `archive-index-corrupt` | 3 |
| X3 | schema version equals this build; a higher version says upgrade | `archive-index-version-unsupported` | 3 |
| X4 | feature equals the directory slug | `archive-index-foreign` | 3 |
| X5 | `generations` and `replaced` are non-null arrays | `archive-index-corrupt` | 3 |
| X6 | generation ids are unique lowercase SHA-256 hex | `archive-index-corrupt` | 3 |
| X7 | every `content_sha256` is lowercase SHA-256 hex; `purged=false` iff `blob == content_sha256`; `purged=true` iff `blob == ""` | `archive-index-corrupt` | 3 |
| X8 | every path is relative and contained in its feature directory | `archive-index-path-escape` | 3 |
| X9 | artifact ids are the closed four-value set and unique per generation | `archive-index-corrupt` | 3 |
| X10 | recomputing `generation_id` from the immutable pre-purge body (including `content_sha256`) reproduces its recorded value, whether or not entries are tombstoned | `archive-index-generation-mismatch` | 3 |

A corrupt index is never rewritten, used to remove a blob, or used for a
canonical write. PIB-331…PIB-341 plus PIB-401 cover the binds and tombstone
identity.

### 9.4 Determinism

Two invocations that replace the same artifacts with the same prior bytes
produce **byte-identical** blobs and a byte-identical `index.json` entry. The
whole archive is a pure function of the sequence of replaced content — no
clock, no PID, no hostname, no random suffix reaches it (the random suffix is
confined to the gitignored staging tree). PIB-160, PIB-161, PIB-162.

### 9.5 Recovery UX in v1

The success report names, for each replaced artifact, the repo-relative blob
path. Recovery is an ordinary file copy:

```text
cp .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob \
   .tpatch/features/<slug>/spec.md
```

No `--restore` verb ships in v1 (non-goal 6, Q4). The report must state the
copy form explicitly and must not imply a verb that does not exist (PIB-163).

### 9.6 Privacy: the redaction precondition and the retention disclosure

#### 9.6.1 ADR-027 D3 applies, and it is a hard gate

ADR-027 D3 is unambiguous: *"Every tpatch-owned writer MUST run the redaction
contract before persisting any capture artifact in either lane"*, and
*"Redaction failure is a hard failure for committed summaries"*
(`docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-177`). The archive
is a tpatch-owned writer persisting bytes into the **tracked** lane. rev-0 did
not engage D3 at all.

**The gate** (ADR-035 D15): before any blob is written, each `replace` entry's prior bytes are
passed to the shipped scanner `redact.Scan`
(`internal/redact/redact.go:81-90`), which applies the closed six-class
resource policy (`internal/redact/redact.go:53-72`): `private-key`,
`connection-url`, `email-pii`, `credential-assignment`, `bearer-or-key-token`,
`home-absolute-path`.

**On a match, the whole invocation refuses**: exit 3,
`archive-content-refused-sensitive`, zero blobs written, zero canonical
mutations, and the report names **the artifact and the matched class codes —
never the matched bytes, never an excerpt, never an offset** (PIB-262 …
PIB-267).

Refusal rather than scrubbing is forced by the archive's purpose. Scrubbing
produces bytes that are not the file's bytes, so a "recovery" copy would
silently differ from what was replaced — a worse failure than refusing. The two
requirements (exact recovery, and never persisting secret-shaped content) are
genuinely in conflict, and the design resolves the conflict by refusing the
*destructive* operation, never by silently persisting the secret. `redact.Scan`
takes in-memory bytes and never a path precisely so this ordering is structural
rather than a call-site discipline (`internal/redact/redact.go:18-21`).

**The operator's route forward** is stated in the refusal itself: remove the
sensitive material from the artifact (or move the file aside yourself), then
re-run. The refusal never suggests `--force`, because none exists.

**A false positive is possible and is not silently overridable.**
`home-absolute-path` **and the broad empirically relevant `email-pii` class** can match legitimate Markdown examples or contact text inside a hand-authored spec. The chosen trade is: a false positive costs the operator an
edit and a re-run; a false negative costs a credential retained in every future
clone. Q9 records a future scoped-override design; it is deliberately not in
v1, because an override flag on a redaction gate is exactly the shape that
turns a hard gate into a suggestion.

#### 9.6.2 The retention disclosure — rev-0's "no new exposure class" is withdrawn

rev-0 argued that archiving introduced no new exposure class because the same
content was already committable at `analysis.md`. That is **wrong**, and the
correction is normative:

1. **Overwriting used to be a removal.** Before this PRD, replacing `spec.md`
   destroyed the prior bytes. An operator who pasted a credential into a spec
   could remove it by overwriting the file. With the archive, the same
   `--regenerate` **preserves** those bytes at a second tracked path. The
   exposure class is not "different content"; it is **durable retention of
   content the operator was destroying**.
2. **Committed retention is durable across clones.** Once committed, a blob is
   in the history of every clone. That is the point of the recovery guarantee
   and the cost of it, and §9.7 is the bound.
3. **Tracking is conditional, and no clone-durability is claimed before a
   commit.** The archive is tracked **iff the operator tracks `.tpatch/`**.
   Where `.tpatch/` is untracked (which the CLI already detects,
   `internal/cli/cobra.go:3405-3407`), the archive is an ordinary untracked
   directory: it survives crashes and re-runs on that machine, and it does
   **not** survive a fresh clone. No sentence in this PRD, in any report, or in
   any doc may claim clone-durable recovery for uncommitted blobs; advisory
   `bundle-untracked-in-git` states the conditional truthfully (PIB-139,
   PIB-342, PIB-343).
4. **`land` sweeps the archive into the operator's commit** like any other
   file under the feature directory (`internal/cli/land.go:723-725`), so an
   operator who lands a feature after a `--regenerate` commits the blobs
   without a separate action. That is disclosed in the human report and in
   `docs/feature-layout.md`, and PIB-152 asserts the sweep behavior (PIB-344
   asserts the disclosure exists).

### 9.7 Retention: listing, purging, tombstones and orphans

A durable tracked store with no removal route is not a design, it is a leak.
ADR-035 D16 is normative here. The retention surface is deliberately small,
previewable, and modelled on the shipped `session purge` (`internal/cli/session.go:353-419`).

#### 9.7.1 Grammar and outputs

```text
tpatch feature intent-archive list  <slug> [--json] [--quiet] [--path <dir>]
tpatch feature intent-archive purge <slug> (--blob <hash>... | --generation <id>... | --orphans | --all) [--yes] [--json] [--quiet] [--path <dir>]
```

- `list` is read-only. It prints each generation, each replaced entry, its blob
  hash (or `purged`), its size, whether the blob file is present, and a final
  `orphans:` section naming blob files no index entry references. Exit 0 with
  an empty archive; exit 3 on a strict-decode failure (§9.3.1). PIB-345,
  PIB-346.
- `purge`'s scope selectors are **mutually exclusive and one is required** —
  the shipped `session purge` lesson, where "neither" silently meant "all"
  (`internal/cli/session.go:369-375`). Missing scope → exit 1 with a message
  naming the four selectors (PIB-347).
- `purge` **previews by default**: without `--yes` it prints exactly what it
  would remove and writes nothing (PIB-348). With `--yes` it acts.
- Both take the per-slug transaction lock (§7.4), so a purge can never race a
  live `prepare`. Contention → exit 3, `transaction-in-progress` (PIB-349).
- `purge` refuses entirely while a journal is pending for that slug — exit 3,
  `recovery-pending` — because a pending rollback may need exactly the blob
  being removed (PIB-350).

#### 9.7.2 What a purge does, exactly

For `--blob`, `--generation`, and `--all`, purge takes one strict-decoded
**index preimage** under the lock and records its raw identity before planning
selection. It then:

1. validates every selected blob is present and hashes to its immutable
   `content_sha256`; rejects a selected blob still referenced by a non-selected,
   non-tombstoned entry;
2. creates and fsyncs a rooted same-directory temporary containing the one
   intended tombstoned index (preserving every immutable body field);
3. **immediately before the rooted index rename**, strictly re-decodes the live
   index and requires its raw identity to equal the captured index preimage. A
   mismatch refuses `archive-index-changed` (exit 3), deletes the temp, and
   removes **no** blob; and
4. renames and fsyncs `index.json`, then removes selected blobs and fsyncs
   `blobs/`.

The ordering is load-bearing. A crash after the index rename but before blob
removal leaves an honest tombstone plus an orphan; a crash before the rename
leaves the old referenced index and all blobs. `--orphans` removes only files
that a strict current index does not reference and rewrites no index. Purging
twice is idempotent. PIB-405 injects a concurrent index edit at the immediate
CAS point and proves no blob is removed.

Tombstones retain `artifact_id`, `path`, `content_sha256`, `size_bytes`, and
`generation_id`; they remove bytes, not historical identity. A purged blob is
not recoverable by tpatch. A committed blob remains in Git history even after
working-tree purge.

#### 9.7.3 Orphans

An orphan is a `blobs/*.blob` file that no index entry references. Orphans come
from crashes between §7.8 step 7 and step 9's index rename (including rehydration), and from refusals
after a blob write.

- They are **reported**: by `list` in an `orphans:` section, by `doctor`
  (§12.5), and by the rolled-back / refused reports that created them (§7.9,
  PIB-316).
- They are **removable**: `purge --orphans` removes exactly the unreferenced
  blob files and rewrites nothing, since no entry references them (PIB-359).
- They are **harmless**: no reader consults blobs to determine current state,
  so an orphan changes no behavior. It is tracked bytes, which is why it must
  be removable rather than "normal and permanent".

#### 9.7.4 The remediation route for an accidentally archived secret

If a secret reaches a blob — through a false negative of the six-class scan, or
through content that predates this feature — the operator's route inside
`tpatch` is exactly two commands, and the refusal, `doctor` and
`docs/feature-layout.md` all state it:

```text
tpatch feature intent-archive list  <slug>
tpatch feature intent-archive purge <slug> --blob <hash> --yes
```

and, when the blob was already committed, the honest addendum: *"a committed
blob remains in Git history; purging removes it from the working tree only.
Removing it from history is a Git operation (`git filter-repo` or equivalent)
that tpatch does not perform."* Claiming otherwise would be the single most
dangerous false statement this document could make (PIB-360, PIB-361).

### 9.8 Why the archive is not provenance — stated so it cannot drift

| Question a provenance record answers | Does the archive answer it? |
|---|---|
| Who or what authored these bytes? | **No.** Nothing is recorded about the author. |
| Was this artifact produced by Path A or Path B? | **No.** `mode` names the *tpatch invocation mode that replaced it*, not the origin of the replaced bytes. A hand-authored file replaced by `--regenerate` records `mode: "regenerate"` — a fact about the replacing act. |
| Which model/provider produced them? | **No.** No provider, model, endpoint or configuration is recorded. |
| Is the current `spec.md` provider-generated? | **No.** The archive says nothing about the *current* file. |
| Have these exact bytes existed at this path before? | **Yes** — and that is the only question it answers, and only until they are purged. |

Consequently:

- `prepare --check` continues to report the constant `provenance: unknown` for
  every artifact, and **must not** consult the archive (PIB-140, source-scanned
  by PIB-141).
- No mutating report field may assert Path A vs Path B for a feature (PIB-142).
- The forbidden-inference list of the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920`) is extended
  by exactly one entry — **the intent archive** — and PIB-143 asserts that
  entry exists in the shipped source.
- `status.json.notes` remains a **last-transition hint, not provenance**
  (§12.3.2), and the archive does not upgrade it into one.
- Any future PRD that wants to answer the first four rows must write the
  provenance ADR first. This PRD does not, and ADR-035 D9 says so normatively.

## 10. CLI output, exit codes and precedence

### 10.1 Stream routing

Composed with the **real** root error printer, which emits `error: %v` for
every non-nil `RunE` error (`internal/cli/cobra.go:33-39`) before
`exitCodeFor` maps the code (`internal/cli/cobra.go:43-52`). This PRD inherits
that composition from the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419`) and does not
re-litigate it.

| Flags | stdout | stderr |
|---|---|---|
| (none) | human report + progress lines | one `error:` line on every nonzero exit |
| `--json` | the JSON report, one document, trailing newline | one `error:` line on every nonzero exit |
| `--quiet` | one summary line only | one `error:` line on every nonzero exit |
| `--json --quiet` | the JSON report only | one `error:` line on every nonzero exit; **empty only on exit 0** |

**Progress vs final report.** Progress lines ("[1/3] Generating analysis…")
are written to **stderr**, never stdout, and are suppressed entirely under
`--json` or `--quiet`. This keeps stdout a single parseable document under
`--json`, which the shipped `next --format harness-json` convention already
assumes (`internal/cli/phase2.go:381-384`). The existing phase commands print
progress to stdout (`internal/cli/cobra.go:621-626`); this command deliberately
does not, and §12.6 D5 enumerates that as a *new-surface* choice, not a change
to those commands. PIB-170, PIB-171.

### 10.2 JSON report (v1)

`schema_version: 1`. Fixed key order by struct declaration order. No Go map.
This is a **different schema** from `--check`'s: `command` distinguishes them
and consumers must switch on it (PIB-172).

```json
{
  "schema_version": 1,
  "command": "prepare",
  "mode": "regenerate",
  "dry_run": false,
  "slug": "fix-model-id-translation",
  "outcome": "published",
  "action": "regenerate",
  "feature_state": "defined",
  "disclaimer": "Structural presence only. This report does not certify semantic quality.",
  "artifacts": [
    {
      "id": "analysis",
      "path": ".tpatch/features/fix-model-id-translation/analysis.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "provider",
      "archived_blob": "a91e…"
    },
    {
      "id": "spec",
      "path": ".tpatch/features/fix-model-id-translation/spec.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "heuristic",
      "archived_blob": "77bd…"
    },
    {
      "id": "exploration",
      "path": ".tpatch/features/fix-model-id-translation/exploration.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "heuristic",
      "archived_blob": ""
    },
    {
      "id": "analysis_sidecar",
      "path": ".tpatch/features/fix-model-id-translation/artifacts/analysis.json",
      "role": "optional",
      "disposition": "regenerated",
      "generator": "provider",
      "archived_blob": "c40a…"
    }
  ],
  "archive": {
    "generation_id": "3f0c…",
    "blobs_dir": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs"
  },
  "orphan_blobs": [],
  "advisories": [
    {
      "code": "regenerate-heuristic-allowed",
      "artifact_id": "",
      "message": "--allow-heuristic was passed, so at least one artifact was replaced by heuristic output."
    },
    {
      "code": "provider-fallback-heuristic",
      "artifact_id": "spec",
      "message": "The provider call failed and the heuristic generator was used instead. Re-run after fixing the provider to replace it."
    }
  ]
}
```

The example is a `--regenerate --allow-heuristic` run: without that flag a
provider failure would have refused before publication (§11.3.2), so a
`regenerate` report can only carry `generator: "heuristic"` alongside
`regenerate-heuristic-allowed` (PIB-372).

Closed vocabularies:

| Field | Closed set |
|---|---|
| `mode` | `generate`, `manual`, `regenerate`, `abandon` (`check` never emits this schema) |
| `outcome` | `published`, `no-op`, `planned`, `refused`, `rolled-back`, `recovery-refused`, `abandoned`, `abandon-planned` |
| `action` | `none`, `adopt`, `complete`, `regenerate`, `abandon` |
| `disposition` (per artifact) | `preserved`, `generated`, `regenerated`, `untouched`, `absent-optional` |
| `generator` | `provider`, `heuristic`, `` (empty for anything not generated this run) |
| `advisories[].code` | the seventeen codes of §10.3 |
| `refusal.code` | the closed refusal catalog of §10.4, asserted complete by PIB-228 |
| `orphan_blobs[]` | zero or more `^[0-9a-f]{64}$` hashes; present on every outcome that leaves one (§7.9, §9.7.3) |

**`generator` is a statement about this process, not about the file's
history.** It is emitted transiently, is never persisted to any tracked
artifact by this field name, and §9.8 governs why it is not provenance
(PIB-144).

**Refusal shape.** On any refusal, `outcome` is `refused`, `artifacts` carries
the per-artifact dispositions the plan *would* have had (or `[]` when the
refusal happens before inspection, e.g. workspace or slug), `archive` is
omitted, `orphan_blobs` lists every blob this attempt left behind (usually
empty), and a `refusal` object carries a closed `code`, a message and a
self-contained remediation (PIB-173, PIB-174, PIB-316).

**Abandon shape.** In `abandon` mode the report carries `mode: "abandon"`,
`outcome: "abandon-planned"` (preview) or `"abandoned"` (`--yes`), an
`abandoned` object naming the evidence directory and the control files moved
into it, `artifacts: []` — because the mode inspects no artifact and asserts
nothing about them — and `archive` omitted (PIB-273, PIB-366).

### 10.3 Advisory catalog (closed, seventeen codes)

| Code | Fires when | Says |
|---|---|---|
| `provider-not-configured` | default mode, no provider configured or reachable | the heuristic generator was used; how to configure one |
| `provider-fallback-heuristic` | default mode (or `--regenerate --allow-heuristic`), a provider call failed and the heuristic generator produced the artifact | which artifact; that re-running replaces it |
| `provider-deadline-heuristic` | a **deadline** (per-phase or total) rather than a provider error caused the fallback | which artifact, and which of the two deadlines expired |
| `provider-deadline-cascade` | the **total** deadline expired and therefore two or more artifacts fell back for the same reason | how many artifacts, and that one expiry — not three independent failures — caused it (§11.5) |
| `regenerate-heuristic-allowed` | `--regenerate --allow-heuristic` was used **and** at least one artifact fell back | that a hand-authored bundle was replaced by heuristic output at the operator's explicit request |
| `allow-heuristic-redundant` | `--allow-heuristic` was passed in default mode | that fallback is already the default there and the flag changed nothing |
| `analysis-preserved-sidecar-untouched` | `analysis.md` preserved, sidecar absent | the sidecar is not synthesized for a preserved analysis, and its absence is not a defect |
| `bundle-untracked-in-git` | `--regenerate`, `.git` present, `.tpatch/` not tracked (`internal/cli/cobra.go:3405-3407`) | the archive exists but is **not** clone-durable until committed (§9.6.2) |
| `workspace-not-git` | `.git` is absent, so the local-lane gate was inapplicable and was skipped (§7.13.2) | that no Git check ran and no Git recovery route exists here |
| `archive-blob-reused` | a replaced artifact's content already existed as a blob | zero new bytes were written for it |
| `archive-generation-duplicate` | the computed `generation_id` already exists in the index | no index entry was appended; the archive is a set |
| `archive-orphan-blobs` | orphan blobs exist for this slug after this invocation, whatever its outcome | how many, and the exact `purge --orphans` command |
| `staging-retained` | a failure left a staging tree | its repo-relative path; that the next successful run removes it |
| `recovered-prior-transaction` | recovery undid a pending transaction before this run | which entries were restored |
| `features-index-refresh-failed` | the best-effort `FEATURES.md` refresh returned an error (§12.3.1) | that `status.json` is authoritative and the next transition retries the index |
| `feature-state-below-defined` | the source state was `requested` or `analyzed` | the transition that was performed |
| `heuristic-mode-recorded-in-sidecar` | the sidecar was generated in heuristic mode | that `heuristic_mode: true` is set in the sidecar, exactly as `RunAnalysis` does today (`internal/workflow/workflow.go:25,208`) |

Advisory selection is a **total function of observed state**, never of artifact
id: every advisory's precondition is a predicate over the inspection result and
the transaction outcome, and PIB-175 asserts no advisory can contradict its own
artifact row. PIB-227 asserts the shipped catalog is exactly these seventeen
codes and that each is reachable by a named fixture.

### 10.4 Exit codes

Per-command contract, per `SPEC.md:137`.

| Code | Meaning | Report emitted? | Wrote anything? |
|---|---|---|---|
| `0` | success: published, no-op, plan OK, preview of an abandon/purge, or `--check` ready | yes | only on `published` and on `--yes` abandon/purge |
| `1` | generic CLI/parse error (arity, unknown flag, mutually exclusive flags, missing purge scope), or an unexpected internal error | no (parse) / yes (internal) | no |
| `2` | **not-ready / not-coherent refusal**: `--manual` on an incomplete bundle; default mode on a `present-empty` required artifact; an incoherent bundle gap (§6.1.2); staged-output validation failure | yes | no |
| `3` | **cannot-act refusal**, four documented populations distinguished by `refusal.code`: (a) *indeterminate* — workspace not initialized, feature not found, unsafe slug, `status.json` malformed/unreadable, an artifact in an unsafe/unstable state, archive or index corruption, local-lane gate failure or unverifiability, unsupported platform, unsupported lock filesystem, `request.md` unreadable; (b) *lifecycle-state* — the source state does not permit preparation (§12.2); (c) *transient* — `transaction-in-progress` (a live sibling holds the lock), `recovery-pending` (`--dry-run`/`purge` with a pending journal); (d) *policy* — `provider-required-for-regenerate`, `archive-content-refused-sensitive` | yes | no |
| `4` | **retired.** The reserved-surface population (`prepare` without `--check`) no longer exists, and no new population is bound to it. `prepare` never exits 4. | — | — |
| `5` | **transaction aborted**: revalidation or per-entry CAS mismatch, generation failure after staging began, or a rename failure that was **successfully rolled back**. No canonical artifact, no `status.json` and no index entry changed; orphan blobs may remain and are listed (§7.9). | yes | possibly additive orphan blobs, fully enumerated |
| `6` | **manual intervention required**: an undo CAS refused, rollback failed, recovery found divergent evidence, a journal failed a J1–J10 bind, or the final verification found post-publication divergence. Everything is preserved, and §6.6's abandon route is named in the message. | yes | possibly a partial publication, fully described |

`0`/`2`/`3` keep the meanings the accepted `--check` contract already binds
(`docs/prds/PRD-artifact-validation-and-provenance.md:1981-1987`), so the same
condition yields the same code in both halves of the verb. Population (b) of
exit `3` additionally matches the repository's shipped state-refusal
convention: `reject`, `reopen` and `refuseIfUnappliedState` all bind
state-transition refusal to `3` (`internal/cli/reject.go:45-48,68-70`;
`internal/cli/feature_unapply.go:464-473`; `SPEC.md:148`). Collapsing lifecycle
refusal into `3` rather than inventing a fifth code is what lets `prepare`
reuse the shipped `refuseIfUnappliedState` helper unchanged. `5` and `6` are
new and have no read-mode population.

**Exit 6 is never terminal.** Every exit-6 message names, in this order: the
divergent entry, the file that holds its prior bytes (an archive blob or a
journal-directory preimage), and the exact command
`tpatch prepare <slug> --abandon-transaction` to unblock the slug without
touching any canonical file (§6.6). PIB-362 asserts all three elements are
present in every exit-6 population; PIB-363 asserts that after the abandon
route runs, the same slug reaches exit 0 on a subsequent admissible run.

### 10.5 Precedence (first match wins)

1. Cobra/pflag parse, arity or mutual-exclusion error → `1`. Nothing else
   runs; the text is pflag's and is outside this command's schema, exactly as
   ADR-034 D17 scopes it.
2. `--check` selected → the **entire accepted contract** takes over, including
   its own precedence, codes and bytes. Nothing below applies (PIB-198).
3. Canonical slug validation → `3` (`slug-unsafe`), reusing the accepted
   grammar and its no-echo rule
   (`docs/prds/PRD-artifact-validation-and-provenance.md:696-772`).
4. Workspace discovery → `3` (`workspace-not-initialized`).
5. Read-boundary platform support → `3` (`workspace-unsupported-platform`),
   reusing ADR-034 D5's fail-closed allowlist unchanged.
6. **Lock-primitive platform and filesystem support** (mutating modes only) →
   `3` (`prepare-unsupported-platform`, `lock-filesystem-unsupported`), §7.4.2.
   Distinct from step 5 because the write half's platform set is narrower than
   the read half's, and conflating them would misreport which half is
   unsupported.
7. Local-lane contract → `3` (`local-lane-not-ignored` or
   `local-lane-unverifiable`), or skipped with advisory `workspace-not-git`
   (§7.13.2).
8. Feature directory / `status.json` population → `3`
   (`feature-not-found`, `status-malformed`, `status-unreadable`).
9. Lock acquisition → `3` (`transaction-in-progress`) on contention.
10. `--dry-run` with a pending journal → `3` (`recovery-pending`), §6.4.
11. Pending-transaction recovery → `6` on divergent evidence or any J1–J10
    bind failure; otherwise recovery completes and evaluation continues with
    advisory `recovered-prior-transaction`.
12. `--abandon-transaction` → its own contract (§6.6); nothing below applies.
13. `request.md` capture (generating modes only) → `3`
    (`request-unreadable`).
14. Lifecycle-state gate (§12.2) → `3` (`state-refused`).
15. Artifact admissibility (§6.1.1 / §6.3 tables) → `3` for unsafe/unstable,
    `2` for `present-empty` in default mode, `2` for `--manual` not-ready.
16. Bundle coherence (§6.1.2, §6.1.3) → `2` (`incoherent-bundle-gap`).
17. `--regenerate` provider authority (§11.3) → `3`
    (`provider-required-for-regenerate`).
18. Generation, staging, staged-output validation → `2` on validation failure,
    `5` on an unrecoverable generation failure after staging began.
19. Set-level revalidation mismatch → `5`.
20. Redaction scan of the bytes about to be archived → `3`
    (`archive-content-refused-sensitive`), §9.6.1.
21. Archive/index integrity (§9.2, §9.3.1) → `3`.
22. Per-entry CAS, publication and rollback → `5` if rolled back, `6` if an
    undo CAS refused or the rollback failed.
23. Final verification (§7.8 step 10) → `6` on divergence.
24. Otherwise → `0`.

The order is load-bearing in five places, and each has a row: the slug is
validated before any path is composed (PIB-176); the lock is acquired before
recovery, so recovery never races a live sibling (PIB-364); recovery runs
before the lifecycle gate so a pending transaction is never left behind by a
refusal that happens to come first (PIB-177); the redaction scan runs before
the first blob write, so a refusal leaves no sensitive bytes on disk (PIB-262);
and the provider-authority gate runs before any generation, so a
`--regenerate` without a provider costs nothing (PIB-365).

### 10.6 Human output

Deterministic layout, fixed section order, no color, no wall-clock, no
duration, no absolute path, no symlink target, no artifact content:

```text
Feature: fix-model-id-translation   (state: defined)
Mode:    regenerate --allow-heuristic

  analysis.md              regenerated   (provider)   archived a91e…
  spec.md                  regenerated   (heuristic)  archived 77bd…
  exploration.md           regenerated   (heuristic)
  artifacts/analysis.json  regenerated   (provider)   archived c40a…

Archive: .tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs
  Restore a prior file with: cp <blobs-dir>/<hash>.blob <path>
  Remove archived bytes with: tpatch feature intent-archive purge fix-model-id-translation --blob <hash> --yes

Advisory: --allow-heuristic was passed, so hand-authored content was replaced
          by heuristic output where the provider failed.
Advisory: The provider call failed for spec.md and the heuristic generator was
          used instead. Re-run after fixing the provider to replace it.

Structural presence only. This report does not certify semantic quality.
```

The disclaimer is the **frozen** accepted string
(`docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798`) and is
asserted byte-for-byte in both surfaces (PIB-178).

`--quiet` prints exactly one line per invocation, ending in the outcome token:

```text
prepare fix-model-id-translation: regenerate published (4 artifacts, 3 archived)
prepare fix-model-id-translation: abandon abandoned (3 control files moved)
prepare fix-model-id-translation: generate refused incoherent-bundle-gap
```

The line is a fixed grammar — `prepare <slug>: <mode> <outcome>[ <detail>]` —
so a harness can parse it without knowing the mode (PIB-019, PIB-367).

### 10.7 Refusal remediation is self-contained

Every refusal names only shipped commands, shipped flags and repo-relative
paths that exist. It must not cite a PRD path, an ADR path, an issue URL or any
`docs/` file — a shipped binary's diagnostic cannot depend on a document the
user does not have. This is the accepted contract's rule
(`docs/prds/PRD-artifact-validation-and-provenance.md:374-381`), applied to
every refusal code in §10.4 (PIB-179, mechanically guarded by PIB-180).

## 11. Path A generation

### 11.1 Pure, staged generators — the extraction

The three phase functions cannot be called (§1.3). ADR-035 D12 governs the
extraction; the implementation splits their **generation** half from their
**publication** half:

| New (pure) | Extracted from | Returns |
|---|---|---|
| `GenerateAnalysis(ctx, in AnalysisInput) (AnalysisResult, GenNote, error)` | `internal/workflow/workflow.go:35-88` (everything above the first `WriteArtifact`) | the struct, plus which generator ran and any raw response |
| `RenderAnalysisMD(result, slug) string` | already pure (`internal/workflow/workflow.go:351-401`) | markdown bytes |
| `GenerateSpec(ctx, in DefineInput) (string, GenNote, error)` | `internal/workflow/workflow.go:111-150` | markdown bytes |
| `GenerateExploration(ctx, in ExploreInput) (string, GenNote, error)` | `internal/workflow/workflow.go:159-195` | markdown bytes |

Hard constraints on the extracted functions, each source-scanned:

1. They take **no `*store.Store`** and hold no writer (PIB-184).
2. They perform **no filesystem write** of any kind (PIB-185).
3. Their inputs are plain values: request text, prior-artifact text, file tree,
   guidance text, provider handle and config.
4. `RunAnalysis`, `RunDefine` and `RunExplore` are **refactored to call them**
   and keep their current observable behavior byte-for-byte, so `analyze`,
   `define`, `explore` and `cycle` are unchanged (PIB-186, proved against
   pre-change goldens per §17.2).

`captureFileTree` and `readGuidanceFiles`
(`internal/workflow/workflow.go:249-253,290-301`) are already pure reads and
are reused unchanged.

### 11.2 Context flow and dependency order

Generation order is fixed: **analysis → spec → exploration**, matching the
shipped data dependencies (`internal/workflow/workflow.go:113-129,161-172`).

Each generator's context is assembled from the **effective** bundle — that is,
staged output from this run where it exists, and the **existing canonical
bytes** where an artifact was preserved:

| Generator | Consumes |
|---|---|
| analysis | `request.md` |
| spec | `request.md`, effective `analysis.md`, effective `artifacts/analysis.json` (when present) |
| exploration | `request.md`, effective `analysis.md`, effective `spec.md`, file tree |

Two rules make "effective" safe:

1. **Existing artifacts are read through the accepted inspector's bounded,
   rooted capture** — same cap, same refusals, same instability handling
   (ADR-034 D1, D8, D9). A preserved artifact that cannot be safely read has
   already refused the invocation at §6.1.1, so no generator ever receives
   partially-read or unsafely-resolved bytes (PIB-187).
2. **Consuming a preserved artifact claims nothing about it.** The generated
   `spec.md` does not record that its analysis was hand-authored, and the
   report does not either (PIB-142). Today's `RunDefine` already reads whatever
   `analysis.md` is on disk without asking where it came from
   (`internal/workflow/workflow.go:118-121`); this preserves that behavior
   rather than inventing a claim.

### 11.3 Provider vs heuristic — the authority policy

Default mode preserves the shipped behavior. `--regenerate` does not, and the
difference is the point.

#### 11.3.1 Default mode (`generate`): fallback, disclosed

| Situation | Behavior | Observable |
|---|---|---|
| No provider configured | heuristic generators run (`internal/workflow/workflow.go:82-84,144-146,189-191`) | `generator: "heuristic"`, advisory `provider-not-configured`, exit 0 |
| Provider configured, call fails or validation fails after retries | heuristic generator for that artifact | `generator: "heuristic"`, advisory `provider-fallback-heuristic`, exit 0 |
| A deadline expired | heuristic generator for that artifact | `generator: "heuristic"`, advisory `provider-deadline-heuristic` (plus `provider-deadline-cascade` when one total expiry caused two or more), exit 0 |
| Provider succeeds | provider output | `generator: "provider"`, no advisory |

This is safe **because the default mode only ever creates absent files**
(§6.1.3). A heuristic `exploration.md` written where none existed replaces
nothing; the operator can re-run after fixing the provider, and the advisory
says so. The provider is loaded with the **non-probing** loader `cycle` uses
(`internal/cli/phase2.go:55`), not the probing loader the single-phase commands
use (`internal/cli/cobra.go:609-612`), so a missing provider is a fallback
condition rather than a command failure — which is what makes `prepare` usable
offline, per locked-in decision 8 of `CLAUDE.md`.

#### 11.3.2 `--regenerate`: a configured, successful provider is required

rev-0 applied the same fallback to `--regenerate`. That meant a single expired
context could replace a human's three hand-authored documents with heuristic
boilerplate, report exit 0, and archive the originals into a directory the
operator did not know existed. The archive makes that *recoverable*; it does
not make it *acceptable*.

| Situation | Behavior |
|---|---|
| No provider configured | **refuse before any mutation**: exit 3, `provider-required-for-regenerate`; zero blobs, zero canonical writes; remediation names `tpatch provider set`, `tpatch provider check` and `--allow-heuristic` |
| Provider configured but unreachable, or every retry fails | **refuse before publication**: exit 5, `regenerate-generation-failed`; staging retained; zero blobs written (the redaction scan and blob writes come *after* staging, §7.8) |
| Any deadline expires during generation | same refusal, with the expired deadline named |
| `--allow-heuristic` passed and any of the above occurs | proceed with heuristic output; advisory `regenerate-heuristic-allowed` **plus** the specific fallback advisory; exit 0 |
| Provider succeeds for every artifact | publish; exit 0 |

**One expiry cannot silently cascade.** In `--regenerate` without
`--allow-heuristic`, the first failure refuses the whole invocation, so a
cascade is impossible by construction. In default mode and under
`--allow-heuristic`, a total-deadline expiry that causes two or more fallbacks
emits `provider-deadline-cascade`, which states that **one** expiry — not three
independent provider failures — produced them, so the report cannot be
misread as "the provider is broken" (PIB-368 … PIB-373).

**`--allow-heuristic` is the complete opt-in, and it is spelled out.** Its help
text reads: *"Permit `--regenerate` to replace the bundle with heuristic
output when the provider is missing or fails. Without this flag, regeneration
refuses rather than downgrading hand-authored content."* It is legal only with
`--regenerate` (a no-op advisory in default mode, a parse error elsewhere,
§5.2), it is never implied, and there is no config key that turns it on
globally — an opt-in that can be set once in a file and forgotten is not an
opt-in (PIB-374, PIB-375).

- Fallback, where it is permitted, still yields a **complete** new set, so
  WP-005 Agreed item 7's "complete prior set or complete new set"
  (`docs/whitepapers/WP-005-spec-driven-workflows.md:80-81`) holds: a provider
  failure never produces a half-bundle.
- The heuristic sidecar keeps `heuristic_mode: true`
  (`internal/workflow/workflow.go:25,208`), byte-compatible with what
  `analyze` writes today. That field is pre-existing persisted data, not a new
  provenance representation (PIB-146).
- **There is no `--require-provider` in v1.** It would be redundant:
  `--regenerate` requires one already, and default mode's fallback writes only
  files that did not exist. Q3 records the residual question.

### 11.4 Retry and raw responses

- Retry uses the shipped `GenerateWithRetry` loop and its corrective-prompt
  behavior (`internal/workflow/retry.go:77-131`), with `MaxRetries` from config
  and `--no-retry` mapping to the shipped `WithDisableRetry`
  (`internal/workflow/retry.go:47-58,83-85`).
- **`RetryOptions.Store` is left nil.** As shipped, a non-nil `Store` makes the
  retry loop write `artifacts/raw-<prefix>-response-<n>.txt` into the canonical
  tree mid-generation (`internal/workflow/retry.go:105-109`) — a canonical
  write before the transaction has decided anything. `prepare` instead
  captures raw responses through a sink and writes them into the **staging
  tree** (§7.3), where they are gitignored and never published (PIB-188,
  PIB-189).
- Consequence, enumerated as delta §12.6 D3: after `prepare`, no
  `raw-*-response-*.txt` appears under `artifacts/`, unlike after `analyze`.
  Failed runs keep them in the retained staging tree and the report names that
  path (PIB-190).
- **Raw responses never leave the local lane.** They are never archived, never
  included in a report field, never named in an advisory message beyond their
  containing directory, and are removed with the staging tree on success. A
  guard asserts no `raw-*` file ever appears under `.tpatch/features/` after
  any mutating `prepare` (PIB-376), and that no report field carries response
  text (PIB-158).

### 11.5 Timeout budget — per-phase and total

Two deadlines, both flags, both enforced:

| Deadline | Flag | Default | Applies to |
|---|---|---|---|
| **Total** | `--timeout` | `180s` | all generation in the invocation, measured from the first generator call |
| **Per-phase** | `--timeout-phase` | `90s` | each individual generator call, clamped to the remaining total |

Each generator runs under `context.WithTimeout(parent, min(perPhase,
remainingTotal))`. The single-phase commands each use `60s`
(`internal/cli/cobra.go:629`); `prepare` may make three calls, so a single 60s
budget would fail routinely, and three unclamped budgets would make the
command's worst case unbounded-by-flag. Two clamped deadlines make the worst
case exactly `--timeout` plus the non-generation work.

- **The deadlines cover generation only.** They **cannot** interrupt the
  filesystem reads of the inspection half — ADR-034 D16 withdrew every
  bounded-runtime claim, and this PRD does not resurrect one (PIB-191).
- **A per-phase expiry** is a generation failure for that artifact: fallback in
  default mode (advisory `provider-deadline-heuristic`), refusal in
  `--regenerate` without `--allow-heuristic` (PIB-192, PIB-369).
- **A total expiry** ends provider use for the whole invocation: every
  remaining artifact is either heuristic (default mode / `--allow-heuristic`,
  with `provider-deadline-cascade` when two or more are affected) or the
  invocation refuses (`--regenerate`) — PIB-370, PIB-371.
- **Failure is decided before publication, never during it.** No deadline is
  consulted after staging validation succeeds. If the total deadline expires
  after staging and before publication, the command **publishes anyway**: the
  deadline governs generation, not the transaction, and aborting a validated
  publication because a clock expired would be strictly worse (PIB-193).
- The deadline values never reach a tracked artifact or a report field
  (no durations in output, §13.4) — PIB-377.

### 11.6 Staged-output validation — before any canonical write

Every staged byte passes all of these before the window can open. Failure of
V1–V5 is exit `2`; failure of V6 is exit `5`, because a staged file whose
identity moved is a transaction-integrity failure rather than an output-quality
one. In both cases: zero canonical mutations, staging retained.

| # | Check | Rationale |
|---|---|---|
| V1 | non-empty after whitespace trim | the accepted `present-empty` rule (`docs/prds/PRD-artifact-validation-and-provenance.md:1765`) — publishing an artifact the check would call empty is incoherent |
| V2 | size ≤ the accepted `MaxArtifactBytes` | publishing something the check would call `oversize` is incoherent |
| V3 | no NUL byte | these are Markdown/JSON text artifacts; a NUL indicates a truncated or binary provider response |
| V4 | valid UTF-8 | same |
| V5 | sidecar parses as a JSON **object** | the accepted `invalid-structured` rule (`docs/prds/PRD-artifact-validation-and-provenance.md:1772`); the shipped validator already enforces object-ness at generation time (`internal/workflow/workflow.go:62`) |
| V6 | the staged file's post-fsync identity equals the identity recorded for it | nothing may be published that was not durably staged |

**V1–V6 are structural only.** No check inspects headings, length,
plausibility, or whether the text is on-topic. §14.1's disclaimer is what the
report says instead (PIB-194).

**The self-consistency invariant.** After a successful `published` outcome,
running the accepted `prepare <slug> --check` in the same tree must report
`ready` and exit 0. This is the single strongest end-to-end assertion in the
matrix (PIB-195) and is what V1/V2/V5 exist to guarantee.

## 12. Lifecycle and compatibility

### 12.1 `--check` is byte-identical

ADR-035 D14 freezes it. The accepted mode keeps its grammar, its precedence, its report schema, its
exact bytes, its exit envelope and its zero-mutation contract. The only
permitted change to the shipped `prepare` command file is the addition of the
new flags and the mode dispatch **above** the `--check` path. Regressions:
PIB-198 (bytes, human), PIB-199 (bytes, JSON), PIB-200 (exit codes across all
populations), PIB-201 (zero mutation), PIB-202 (`provenance: unknown`
constant), PIB-203 (disclaimer string), PIB-204 (abort shapes), PIB-205
(`artifacts` ⇔ `abort` invariant), PIB-207 (byte-identical with a pending
journal).

### 12.2 Allowed source states

| State | Mutating `prepare` | Why |
|---|---|---|
| `requested` | **allowed** | The ordinary entry point. Generates the whole bundle; ends `defined`. |
| `analyzed` | **allowed** | Completes the bundle from an existing analysis. |
| `defined` | **allowed** | Idempotent: completes a missing `exploration.md`, adopts under `--manual`, or regenerates. May be a no-op success (§6.1.4). |
| `implementing` | refused, exit 3 | A recipe already derives from the current intent (`internal/workflow/workflow.go` implement phase; `docs/feature-layout.md:85`). Rewriting intent underneath it desynchronizes the pair with nothing recording the mismatch. |
| `applied`, `active` | refused, exit 3 | A canonical patch exists (`docs/feature-layout.md:19,34`). Same argument, plus `verify`'s later `intent_files_present` check would compare against changed intent. |
| `reconciling`, `reconciling-shadow` | refused, exit 3 | A reconcile session is live; its inputs must not move under it. |
| `blocked` | refused, exit 3 | Post-implementation; resolve the blocker first. |
| `upstream_merged` | refused, exit 3 | Terminal, and asserts the opposite verdict from "prepare this feature" (`internal/store/types.go:26-31`). |
| `rejected` | refused, exit 3 | Terminal pre-implementation verdict; the shipped exit route is `tpatch reopen` (`internal/store/status.go:139-152`). |
| `unapplied` | refused, exit 3 | Delegated verbatim to the shipped `refuseIfUnappliedState` helper (`internal/cli/feature_unapply.go:464-473`), which already returns exit 3 and already names `tpatch apply` as the remedy. |

The table is **total** over the twelve shipped states
(`internal/store/types.go:9-38`); PIB-225 asserts totality mechanically against
`ValidFeatureState` (`internal/store/types.go:40-47`), so a thirteenth state
added later cannot silently default to "allowed".

**Interaction with reject/reopen.** `prepare` never rejects and never reopens.
A rejected feature refuses with a remediation naming `tpatch reopen <slug>`; a
reopened feature returns to its `PriorState`
(`internal/store/status.go:130-133`) and is then subject to this same table
(PIB-127, PIB-128).

### 12.3 Exact status metadata written

On a successful mutating publication, and only then:

| Field | Value |
|---|---|
| `state` | `defined` (`internal/store/types.go:11`) |
| `last_command` | `"prepare"` |
| `updated_at` | refreshed by the shipped writer (`internal/store/store.go:388`) |
| `notes` | one of exactly three frozen strings (below) |
| everything else | **untouched**, including `verify`, `rejection`, `rejection_history`, `depends_on`, `apply`, `reconcile`, `compatibility`, `id`, `title`, `requested_at` |

Frozen notes strings, one per publishing mode:

```text
Intent bundle prepared (prepare); generated: analysis.md, spec.md
Intent bundle adopted (prepare --manual); artifacts authored by hand
Intent bundle regenerated (prepare --regenerate); prior bytes archived
```

The generated-file list in the first form is the publication set's Markdown
entries in canonical order, so the string is deterministic for a given plan.
`--abandon-transaction` publishes nothing and therefore writes **no** note and
does not touch `status.json` at all (PIB-270).

**`Verify` is not touched.** ADR-013 makes `verify` and `amend` the only
writers of that record (`internal/store/types.go:236-251`), and `prepare` is
neither. This is asserted, not assumed (PIB-129). Note the honest consequence:
a feature that was `verified-fresh` before a `--regenerate` keeps that label
even though its intent changed. That is existing `verify` freshness semantics —
`define` has exactly the same effect today — and this PRD does not widen
`verify`'s writer set to fix it. Q6 records it.

#### 12.3.1 `FEATURES.md`: derived, best-effort, outside T1

`SaveFeatureStatus` writes `status.json` atomically and then calls
`RefreshFeaturesIndex`, **swallowing its error**, because "status.json is the
source of truth and must land even if the derived index can't be rewritten"
(`internal/store/store.go:363-377`, and the rebuild itself at
`internal/store/store.go:703-712`). Every state transition in the product
already behaves this way. rev-0 described `--manual` as writing "exactly one
file", which was false.

The contract (ADR-035 D20), stated once and applied to every mode:

| Property | Value |
|---|---|
| In the publication set? | **No.** `FEATURES.md` is derived from every feature's `status.json`, not from this feature's intent. |
| Journaled? | **No.** No preimage, no new image, no CAS. |
| Rolled back? | **No.** A rollback restores `status.json`; it does not restore the index. |
| Covered by T1? | **No.** T1 is scoped to the publication set (§7.1), and this file is explicitly named as outside it. |
| Failure behavior | Swallowed by the shipped writer; `prepare` additionally **reports** it as advisory `features-index-refresh-failed`, which today's phase commands do not (a new-surface improvement, §12.6 D10). |
| Reconvergence | Any later `SaveFeatureStatus` — from `prepare`, any phase command, `reject`, `reopen`, `record`, `land` — rebuilds it from the authoritative `status.json` files. |

**Two consequences are stated rather than hidden:**

1. **The `--manual` write set is `{status.json}` in the publication set plus a
   best-effort `FEATURES.md` rewrite.** The single-rename atomicity claim of
   ADR-035 D3 applies to the publication set, not to the process's whole write
   footprint (PIB-257, PIB-258).
2. **After a rollback or an undo, `FEATURES.md` may briefly name a state that
   `status.json` no longer claims.** To bound that, every rollback and every
   recovery that restores `status.json` re-runs the same best-effort refresh as
   its last act, so the derived index reconverges within the same invocation
   whenever the filesystem permits it (PIB-259, PIB-260). If it does not, the
   advisory fires and the next transition fixes it. No row anywhere claims
   `FEATURES.md` is transactional (PIB-261).

**Why not make it transactional.** Bringing `FEATURES.md` into the publication
set would make every `prepare` CAS a file that eleven other commands rewrite
without a lock, turning ordinary concurrent activity into exit-5 aborts on a
*derived* file. The shipped design deliberately treats it as a cache; this PRD
matches that and says so.

#### 12.3.2 `notes` is a last-transition hint, not provenance

`MarkFeatureState` overwrites `Notes` on every transition
(`internal/store/store.go:380-393`), so the field describes **the most recent
transition only**. It is not a log, not an append-only record, and not evidence
about who authored any artifact. Three rules follow, each asserted:

1. No report field, no doc sentence and no skill text may present `notes` as
   evidence of Path A vs Path B authorship — the correction the accepted
   contract already requires for `docs/agent-as-provider.md`
   (`docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435`) and which
   §14.1 carries out (PIB-142, PIB-220).
2. The archive does not upgrade `notes` into provenance: a
   `prepare --regenerate` note records that *this invocation* regenerated the
   bundle, not where the replaced bytes came from (§9.8, PIB-378).
3. The frozen strings above are **state**, not history: they are overwritten by
   the next transition, and the wording is chosen so that reading a stale one
   cannot mislead — each names the command that wrote it, and none claims a
   property of the current files (PIB-379).

### 12.4 The mutating `--manual` gates are unchanged

`analyze|define|explore|implement --manual` keep their current loose,
presence-only behavior (`internal/store/manual.go:56-81`). The accepted PRD
made that decision deliberately
(`docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015`) and this PRD
does not reopen it. In particular:

- `tpatch define <slug> --manual` on a zero-byte `spec.md` still advances to
  `defined` and still exits 0 (PIB-130).
- `tpatch prepare <slug> --manual` on the same tree refuses with exit 2
  (PIB-131).
- The differential is intentional and is a composite acceptance row, not a
  footnote: the bundle verb is strict, the per-artifact verbs stay loose
  (PIB-132).

### 12.5 `doctor` gains one check, and it covers evidence-bearing residue

D9 is a warning-only diagnostic. It never mutates, takes no workspace lock,
reads no canonical artifact content, and does not change `doctor`'s exit code.
The external lock probe opens the cache authority `read-only`, without create,
at the opaque key, attempts non-blocking acquisition, and releases immediately.
It never creates a cache lock file.

| Residue | Reported behavior |
|---|---|
| Present journal | names an interrupted prepare and the mutating recovery route |
| Live external lock contention | says a prepare is running now; reports no other racy observation for that slug |
| Persistent cache lock file with no contention | informational and harmless; never asks the user to remove it |
| Retained staging / abandoned evidence | names paths and their documented cleanup semantics |
| Orphan blobs | names count and exact `purge --orphans` command |
| Corrupt index | names the strict bind and leaves repair to the operator |

These are **residues, not proof of cause**. There is deliberately no
"journal-loss symptom" row: canonical partial/mixed artifacts alone are normal
lifecycle state and cannot diagnose a lost journal (§7.11.1). PIB-134 is silent
only for a completely clean cache-lane and archive — no journal, contention,
staging, abandoned evidence, orphan, corrupt index, or persistent cache lock
file — while PIB-407 guards the ordinary partial-bundle negative case.

Acceptance: PIB-133…PIB-136, PIB-380…PIB-387, PIB-407.

#### 12.5.1 The lock probe is read-only

The probe has no `O_CREATE`/`OPEN_ALWAYS` path. It uses the same opaque external
locator as a mutator, opens existing only, and closes before D9 returns. A
momentary successful probe can make a concurrent prepare observe transient
contention; that remains a wait-and-retry exit 3, not a terminal state.

### 12.6 Enumerated behavior deltas — the complete list

Nothing outside this table changes.

| # | Delta | Surface |
|---|---|---|
| D1 | `tpatch prepare <slug>` stops refusing with exit 4 and performs a Path A run | `prepare` |
| D2 | `--manual`, `--regenerate`, `--abandon-transaction`, `--allow-heuristic`, `--dry-run`, `--timeout`, `--timeout-phase`, `--no-retry` and `--yes` become registered flags; supplying an illegal combination changes the exit-1 message text from `unknown flag` to cobra's mutual-exclusion text | `prepare` |
| D3 | `prepare` writes no `raw-*-response-*.txt` under `artifacts/`, unlike `analyze`/`define`/`explore` | `prepare` (new surface only) |
| D4 | A new tracked directory `artifacts/intent-archive/` can appear under a feature, and `land` sweeps it into the operator's commit like any other artifact file | `.tpatch/features/<slug>/` |
| D5 | `prepare` prints progress to stderr, not stdout | `prepare` (new surface only) |
| D6 | `doctor` gains check `D9` (§12.5) | `doctor` |
| D7 | `RunAnalysis`/`RunDefine`/`RunExplore` are refactored to call the extracted pure generators; their observable behavior is unchanged and golden-pinned | `analyze`, `define`, `explore`, `cycle` |
| D8 | **Mutating** `prepare` supports only `linux`, `darwin`, and `windows`; it refuses BSD, `solaris`, `aix`, `js`, `plan9`, and `wasip1`. This is narrower than `--check`'s accepted `unix \|\| windows` read allowlist. | `prepare` (mutating modes only) |
| D9 | **Mutating** `prepare` refuses with `local-lane-unverifiable` unless G1 can establish either a Git worktree or a non-worktree; a Git-established non-worktree proceeds with an advisory. `define --manual` remains unchanged. | `prepare` (mutating modes only) |
| D10 | `prepare` **reports** a failed `FEATURES.md` refresh as an advisory; the shipped phase commands swallow it silently. `status.json` remains authoritative in both (§12.3.1) | `prepare` (new surface only) |
| D11 | A new command group `tpatch feature intent-archive {list,purge}` appears (§9.7) | `feature` |
| D12 | A shared external user-cache lock authority gains Linux/Darwin and Windows implementations; `rescap` remains Linux/Darwin-only with byte-identical behavior, refusal codes and platform envelope (§7.4.2) | `feature resource *` (internal refactor only) |

**Non-invalidation obligations**, each with a row:

- `next`'s routing is byte-identical for every state, including the
  `exploration.md`-presence branch (`internal/cli/phase2.go:437-446`) — PIB-208
  against pre-change goldens.
- `cycle` is byte-identical end to end — PIB-209.
- `analyze`, `define`, `explore`, `implement`, with and without `--manual`, are
  byte-identical — PIB-210, PIB-211.
- `status`, `verify`, `record`, `land`, `reconcile`, `doctor` D1…D8 are
  byte-identical for a feature that never runs `prepare` — PIB-212, PIB-136.
- Every `tpatch feature resource` subcommand is byte-identical after the lock
  extraction, including its contention refusal text and its unsupported-platform
  refusal — PIB-286, PIB-287.
- No command gains a `prepare` precondition, and nothing calls `prepare`;
  asserted in both directions by a reverse call-graph guard — PIB-213.
- `prepare` runs exactly the four read-only Git commands of §7.13.1 and no
  other; the Git index, `.git/**` and every worktree are untouched — PIB-106,
  source-scanned by PIB-107.

## 13. Security, privacy and determinism

### 13.1 Reads reuse the accepted boundary, unchanged

Every canonical read — inspection, generation context, revalidation — goes
through the accepted rooted capture and its refusals: one held `*os.Root`,
`fs.ValidPath` root-relative names, observed-symlink refusal, non-regular
refusal, the bounded `Max+1` read, the identity checks and the fail-closed
`unix || windows` platform allowlist (ADR-034 D1–D9, D15). No second path model
is introduced, and `filepath.Join(repoRoot, …)`, `os.Stat`, `os.Lstat`,
`os.Open`, `os.ReadFile` remain forbidden in the read path (PIB-181,
source-scanned).

### 13.2 Writes: rooted primitives, final-leaf refusal, and disclosed in-root redirects

Writes are a **new** surface and ADR-035 D2 governs them:

1. Every write target is one of a **closed, compile-time list** of
   root-relative names derived from the validated slug: the four artifacts,
   `status.json`, `artifacts/intent-archive/index.json`,
   `artifacts/intent-archive/blobs/<hash>.blob`, and the `.tpatch/local/` lane.
   No write path is ever composed from provider output, from a report field, or
   from any file's content (PIB-182).
2. **Every write is handle-relative** — `(*os.Root).OpenFile`,
   `(*os.Root).Rename`, `(*os.Root).Remove`, `(*os.Root).Mkdir` — so no
   ancestor redirect can escape the held root, and no path-based
   `os.Rename`/`os.CreateTemp`/`os.WriteFile` follows a rooted check (§7.7,
   PIB-308, PIB-310). A relative ancestor symlink that remains in-root may
   still redirect the name; CAS detects a changed resolved identity but not an
   equal-identity alias (§7.7.2, PIB-406).
3. **Final publication targets are never followed.** Immediately before each rename,
   the target's `Lstat` through the held root must show either non-existence or
   a **regular file**. A symlink, junction, directory, FIFO, socket or device
   at that final target aborts the transaction (exit 5) — it is never replaced
   or written through (PIB-183, PIB-196, PIB-197). This does not make an
   ancestor redirect impossible.
4. Temp files are created **inside the destination directory**, root-relative,
   with `O_CREATE|O_EXCL`, so the rename is same-filesystem — the shipped
   rationale (`internal/store/store.go:884-886`) reimplemented on the rooted
   primitive rather than reused path-based (§7.7.3).
5. Modes: `0644` for new tracked files, preserved permissions on replacement
   (`internal/store/store.go:871-876`), `0700`/`0600` in `.tpatch/local/`.

### 13.3 Bounded provider outputs

Provider responses are bounded twice: by the request's `MaxTokens`
(`internal/workflow/workflow.go:58`, `:134`, `:179`) and, decisively, by V2's
byte cap on the staged file (§11.6). A response that exceeds the cap fails
validation and is never published (PIB-157).

### 13.4 No raw secret or provider-response leakage

- Config stores an env-var **name**, never a secret
  (`internal/store/store.go:72`), and `prepare` never reads, echoes or
  persists the value.
- Raw provider responses never reach the tracked tree (§11.4). They live in the
  gitignored staging lane and are deleted on success. This is strictly less
  exposure than `analyze` today, which writes them to `artifacts/`
  (`internal/workflow/retry.go:105-109`).
- No report field carries artifact content, a symlink target, an absolute path,
  a hostname, a PID, a duration or a wall-clock timestamp. The forbidden-field
  guard scopes to **key names and human labels**, not raw substrings, so a
  legitimate value like `archived_blob` cannot be made unspellable — the
  scoping lesson from the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2554-2556,3626`) — PIB-158,
  PIB-159.
- **Blob content is the artifact's own bytes, and that is a retention decision,
  not a non-event.** §9.6.1 makes the ADR-027 D3 redaction scan a precondition
  for writing one, and §9.6.2 discloses the durable retention it creates. The
  redaction refusal's own message names only artifact ids and class codes,
  never matched bytes, offsets or excerpts (PIB-263, PIB-388).

### 13.5 Tracked vs local artifacts — the complete split

| Path | Tracked? | Lifetime |
|---|---|---|
| `.tpatch/features/<slug>/{analysis,spec,exploration}.md` | tracked | canonical |
| `.tpatch/features/<slug>/artifacts/analysis.json` | tracked | canonical |
| `.tpatch/features/<slug>/artifacts/intent-archive/index.json` | tracked when `.tpatch/` is (§9.6.2) | durable until purged; tombstones survive purge |
| `.tpatch/features/<slug>/artifacts/intent-archive/blobs/*.blob` | tracked when `.tpatch/` is (§9.6.2) | durable until purged; content-immutable while present |
| `.tpatch/features/<slug>/status.json` | tracked | canonical |
| `FEATURES.md` (repo root) | tracked | derived, best-effort, outside T1 (§12.3.1) |
| user cache `tpatch/intent-locks/v1/<opaque>.lock` | outside the worktree; private user cache | persistent zero-byte locator; its kernel lock lasts one invocation (§7.4.2) |
| `.tpatch/local/intent-prepare/<slug>/journal.json` | **gitignored** | one transaction |
| `.tpatch/local/intent-prepare/<slug>/{index,status}.preimage.json` | **gitignored** | one transaction |
| `.tpatch/local/intent-prepare/<slug>/stage-*/**` | **gitignored** | one invocation (retained on failure) |
| `.tpatch/local/intent-prepare/<slug>/abandoned-*/**` | **gitignored** | until the operator removes it (§6.6) |

The `.tpatch/local/` ignore contract is enforced before any byte is written
there, conditionally on Git presence per §7.13.2, and a tracked file anywhere
under `.tpatch/local/` refuses the command (PIB-186, PIB-187).

### 13.6 Determinism

- No wall-clock field in any tracked artifact this command writes, and none in
  the journal either (§7.5). The only clock that moves is `status.json`'s
  pre-existing `updated_at`, written by the shipped `SaveFeatureStatus`
  (`internal/store/store.go:369-371`) — pre-existing behavior of every state
  transition, not a new field.
- Canonical JSON with fixed key order and **no Go map** in any wire format
  (ADR-033 D11, `internal/store/canonjson.go:11-17`) — PIB-165.
- The archive is a pure function of replaced content (§9.4).
- The JSON report's key order is fixed and its arrays are ordered by the
  canonical artifact order, never by map iteration — PIB-166.
- Two identical runs against identical trees with the heuristic generator
  produce byte-identical artifacts and byte-identical reports except for
  `status.json`'s `updated_at` — PIB-167.

## 14. Docs, assets and SPEC parity

### 14.1 Documents the implementation wave must update

| Document | Required change |
|---|---|
| `SPEC.md` | The `prepare` surface: five modes, the complete flag mutex table, the seven-code exit envelope with exit 4 recorded as retired, the publication-unit statement (including that `FEATURES.md` is outside it), the narrower mutating-platform envelope (§12.6 D8) and the `tpatch feature intent-archive` grammar. |
| `docs/feature-layout.md` | A new subsection for `artifacts/intent-archive/` — what it is, that it is tracked when `.tpatch/` is, that blob **content** is immutable but blobs are **removable**, that it is never canonical truth, the `cp` restore form, the orphan/tombstone semantics (§9.7), the durable-retention disclosure (§9.6.2), the accidental-secret remediation route (§9.7.4) and the journal-loss boundary (§7.11.1). |
| `docs/agent-as-provider.md` | A `prepare --manual` row alongside the per-phase `--manual` table (`docs/agent-as-provider.md:40-45`), stating that it adopts the **whole** bundle and is strict where the per-phase gates are loose. The existing sentence presenting `status.json.notes` as what "distinguishes Path B transitions from provider output" (`docs/agent-as-provider.md:47-54`) must additionally be corrected to a last-transition hint, **not** durable per-artifact provenance — a correction the accepted PRD already requires (`docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435`) and which this PRD must not contradict. |
| `docs/path-b-operator-guide.md` | The three-`--manual`-commands flow (`docs/path-b-operator-guide.md:61-73`) gains `tpatch prepare <slug> --manual` as the one-step adoption alternative. |
| `docs/path-b-operator-guide.md` (second change) | The non-Git and unusable-Git behavior of §7.13.2, so a Path B operator learns the `local-lane-unverifiable` refusal from the guide rather than from the refusal. |
| `CHANGELOG.md` | The twelve deltas of §12.6. |
| `docs/adrs/README.md` | The ADR-035 index row (created with this PRD at rev-0; unchanged at rev-1). |

### 14.2 Skill asset parity

All six shipped skill surfaces must name the command, and the parity guard must
be extended:

1. `requiredCommands` (`assets/assets_test.go:14-53`) gains `tpatch prepare`.
2. `requiredAnchors` (`assets/assets_test.go:62-73` is the shipped shape) gains
   three anchors: one for the preservation default, one for the archive, and one
   for the `--regenerate` provider requirement, so removing or paraphrasing any
   of them fires the guard.
3. **Hard constraint — the non-mandate assertion.** `prepare` must **not** be
   added to the skills' phase-ordering table or preflight sequence
   (`assets/assets_test.go:66-73`). WP-005 Agreed items 3 and 6
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:56-59,73-76`) forbid
   making a bundle methodology mandatory; a skill that lists `prepare` as a
   required step would do exactly that in the surface agents actually read.
   PIB-216 asserts the command appears in all six files; PIB-217 asserts it
   appears in **no** phase-ordering or preflight block in any of them.
4. Skill text must not claim the command certifies quality, and must not claim
   provenance (PIB-218).
5. Skill text must not present the archive as a general history or undo
   facility, and must state that `--regenerate` requires a provider unless
   `--allow-heuristic` is passed (PIB-389, PIB-390).

## 15. Migration

No migration step, no backfill, no rewrite of any existing file.

| Existing situation | Behavior on first `prepare` |
|---|---|
| Feature with all three Markdown files, `defined` (Path A or Path B) | default mode: no-op success, zero bytes written (PIB-035) |
| Feature with `analysis.md` + `spec.md`, no `exploration.md`, `defined` | default mode: generates `exploration.md` only; `analysis.md`, `spec.md` and the sidecar untouched (PIB-030) |
| Path A feature with a sidecar | sidecar untouched unless `analysis.md` is generated this run (PIB-032) |
| Path B feature with no sidecar | no sidecar is synthesized; advisory `analysis-preserved-sidecar-untouched` (PIB-033) |
| Feature with a zero-byte `spec.md` that reached `defined` through the loose `--manual` gate | default mode refuses (exit 2); `--regenerate` archives the zero-byte file and replaces it (PIB-034, PIB-067) |
| Legacy feature whose `status.json` predates any field added since | untouched; `omitempty` round-trip preserved (PIB-137) |
| Feature with no `artifacts/` directory | created on demand with `MkdirAllAndSyncChain`; no other feature touched (PIB-138) |
| G1-established Git worktree whose `.tpatch/` is untracked | works; advisory `bundle-untracked-in-git`, and no clone-durability is claimed for the archive (PIB-139, PIB-342) |
| Workspace Git establishes as not-a-worktree | works in every mutating mode; the local-lane gate is inapplicable; advisory `workspace-not-git` (PIB-327) |
| Workspace where G1 cannot execute or classify Git | mutating modes refuse with `local-lane-unverifiable`; the per-phase `--manual` commands still work (PIB-329) |
| BSD or another unsupported mutating platform | mutating modes refuse with `prepare-unsupported-platform`; accepted `--check` behavior is unaffected (PIB-288, PIB-289, PIB-409) |
| Archive written by a **newer** `tpatch` (`schema_version` ahead) | every mode that reads the index refuses with `archive-index-version-unsupported` and says to upgrade, never that the file is broken (PIB-333) |
| A feature that never runs `prepare` | byte-identical in every command (PIB-212) |

The archive directory is created **lazily**, only by the first `--regenerate`
that actually replaces something. A repository that never regenerates never
grows one (PIB-068).

## 16. Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | An operator reads "transaction" as T0 and assumes concurrent readers are safe. | §7.1's three-way table is normative; the term "atomic" is never applied to the multi-file publication in any shipped string; PIB-155 is a mechanical over-claim guard over every shipped message and doc sentence this PRD owns. |
| R2 | `--regenerate` destroys work despite the archive, because the archive step silently failed. | Archive failure refuses the whole invocation before the window (§6.3); PIB-060…PIB-066 cover each failure shape. The redaction gate refuses *before* the first blob write, so a refusal never leaves sensitive bytes behind (§9.6.1, PIB-262). |
| R3 | The undo-only journal is mistaken for land's redo journal by a future implementer. | ADR-035 D5 states the direction and its reason; §4's table contrasts them; the journal carries no `phase` field to decide from. |
| R4 | Extraction of the generators regresses `analyze`/`define`/`explore`/`cycle`. | S2 lands pre-change goldens **before** the refactor; PIB-186, PIB-208…PIB-211 compare against them. A refactor with no golden is exactly the no-op-vs-no-op comparison the accepted PRD warns about (`docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520`). |
| R5 | Exit-code confusion between `--check` and mutating modes. | 0/2/3 mean the same thing in both; 4 is retired rather than rebound; 5/6 are new; §10.4 and PIB-013…PIB-015 pin it. |
| R6 | Unbounded archive growth, or a secret retained forever. | Content-addressed dedupe means only distinct content costs bytes; blobs are the artifacts' own sizes, capped by `MaxArtifactBytes`; `tpatch feature intent-archive purge` bounds it explicitly (§9.7); orphans are reported and removable (§9.7.3); the redaction gate refuses secret-shaped content up front (§9.6.1); the committed-history caveat is stated rather than papered over (§9.7.4). |
| R7 | The lock's limited authority is read as full mutual exclusion. | §7.4.4 states it in the negative and names the unexcluded writers; PIB-104 exercises a concurrent `define`; every entry is CAS-gated on publish and on undo (§7.6.3). |
| R8 | The archive is later cited as provenance. | §9.8's table, ADR-035 D9, the extension of the forbidden-inference list (PIB-143), the `notes`-is-a-hint rule (§12.3.2) and the over-claim guard (PIB-155). |
| R9 | A future reviewer assumes a test proves semantics because it exists. | §18.1's disqualifying assertion shapes; §18.42's sensitivity and semantic-fixture requirements over every guard row. |
| R10 | Blob files confuse `git status` / `land` staging for users. | They are ordinary files under `artifacts/`, swept by the shipped feature-path-set rule (`internal/cli/land.go:723-725`); PIB-152 asserts `land` stages them like any other artifact and PIB-153 asserts `record`'s canonical patch is unaffected. |
| R11 | `doctor`'s lock probe makes a concurrent `prepare` see contention. | The probe is non-blocking, non-creating and released immediately (§12.5.1); `prepare`'s contention refusal is a transient exit 3 whose remediation is "wait and retry", never a terminal state (§7.4.3). |
| R12 | A false-positive `home-absolute-path` or broad `email-pii` match blocks a legitimate regeneration. | The trade is stated in §9.6.1: an edit and a re-run versus a credential in every future clone. The refusal names the artifact and class codes so the operator can see what matched; Q9 records a scoped-override design, deliberately not in v1. |
| R13 | The lock extraction regresses `feature resource *`. | S1b lands pre-change goldens for every `feature resource` subcommand before the extraction; PIB-286 and PIB-287 compare against them, including refusal text and the unsupported-platform envelope. |
| R14 | An operator assumes `--abandon-transaction` fixed their files. | Its report states in one line that it moved control state only and changed no canonical file; §6.6 and PIB-270 pin it; the human output names the evidence directory rather than claiming a repair. |
| R15 | A journal lost to `git clean`/fresh clone is read as recoverable or diagnosable. | §7.11.1 states the evidence-loss boundary honestly: `doctor` reports residue but cannot infer loss from ordinary partial bytes; docs disclose the undetectable clean-loss case (PIB-321…PIB-323, PIB-407). |

## 17. Implementation slices and file ownership

All slices are gated on §19. Each is independently reviewable.

### 17.1 The prerequisite that must land first

**S0 is not a slice of this cluster; it is a precondition on another one.**

The accepted [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md)
is **accepted planning, not shipped code**: `prepare --check` does not exist in
any binary yet, and its slices S1–S5 have never been dispatched. Every slice
below builds on it — the inspector, the slug grammar, the state enum, the
report routing, the `internal/cli/prepare.go` file itself. Three rules follow
and they are hard:

1. **No mutating slice may be dispatched until the accepted `--check`
   implementation has landed on `origin/main` and its own acceptance matrix
   passes.** Not "is in flight", not "is in the same cluster": landed and
   green.
2. **Its goldens must be produced by its own implementation, not by this
   cluster.** A golden captured from a binary this cluster built, and then
   compared against a binary this cluster built, proves nothing — it is
   precisely the no-op-versus-no-op comparison the accepted contract warns
   about (`docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520`).
   PIB-198 … PIB-207 compare against goldens committed by the `--check`
   implementation cluster, and PIB-391 asserts the golden fixtures' provenance
   is that commit range rather than this one.
3. **S1b — the lock extraction goldens.** Before `internal/rescap`'s lock is
   moved (§7.4.2), pre-change goldens for every `tpatch feature resource`
   subcommand — including contention and unsupported-platform refusal text —
   must be captured and committed. PIB-286 and PIB-287 compare against them.

If (1) is not satisfied at dispatch time, the cluster lead does not dispatch:
there is no partial ordering that makes a mutating slice safe on top of an
unimplemented read half.

### 17.2 Slices

| Slice | Scope | New/modified files |
|---|---|---|
| **S1** | Transaction core: journal schema + strict decoder and J1–J10 binds, identity, semantic CAS, rooted write primitives, staging, revalidation, publication order, rollback, recovery, cleanup. Pure package, no CLI. | new `internal/intentpub/**` |
| **S1b** | External lock authority: `internal/proclock` owns versioned opaque cache keying, safe cache-root setup, Linux/Darwin flock, Windows deny-share, Linux/Darwin cache-filesystem preflight and the unsupported stub. `rescap` remains a byte-identical wrapper around its own existing Linux/Darwin contract. | new `internal/proclock/**`; modified `internal/rescap/lock_unix.go`, `internal/rescap/lock_unsupported.go` |
| **S2** | Generator extraction: `GenerateAnalysis`/`GenerateSpec`/`GenerateExploration`, the raw-response sink, and the refactor of `RunAnalysis`/`RunDefine`/`RunExplore` to call them. **Lands the pre-change goldens for `analyze`/`define`/`explore`/`cycle`/`next` first.** | modified `internal/workflow/workflow.go`, `internal/workflow/retry.go`; new `internal/workflow/generate_*.go` |
| **S3** | The archive: immutable digest-bearing tombstones, X1–X10 strict decode, idempotent append versus rehydration, redaction-before-rehydrate, strict purge-index preimage CAS, shared references and orphan handling. | new `internal/store/intent_archive.go` |
| **S4** | CLI wiring: modes, rooted manual status helper, flag mutexes, G1 tri-state Git gate, precedence, report model, renderers, exits, advisories, `--dry-run`, `--abandon-transaction`. | modified `internal/cli/prepare.go` (the file the accepted S3 creates), new `internal/cli/prepare_publish.go` |
| **S4b** | The retention surface: `tpatch feature intent-archive list\|purge`. | new `internal/cli/feature_intent_archive.go`; modified `internal/cli/feature_deps.go` (the group registration line only) |
| **S5** | `doctor` D9; compatibility, non-invalidation, concurrency and crash-injection proofs. | new `internal/workflow/doctor_d9.go`; modified `internal/workflow/doctor.go` (registry line only) |
| **S6** | Docs, six skill surfaces, parity-guard extension, over-claim and citation guards, sensitivity meta-check. | `SPEC.md`, `docs/**`, `assets/skills/**`, `assets/assets_test.go` |
| **S7** | Rev-2 hardening after the owned implementation slices: real-process cache lock cleanup/death/contention tests; manual/purge CAS races; rehydration and in-root redirect tests; nested-worktree G1, platform, and journal-loss-negative guards. | tests beside `internal/proclock`, `internal/intentpub`, `internal/store`, `internal/cli`, `internal/workflow`; no new public surface |

**Ordering.** S1b → S1 → S3 → S4 → S4b is strict. S2 may run in parallel with
S1/S1b/S3 **only** under an explicit file partition; S5 and S6 follow S4b;
**S7 is last and sequential** because it exercises every revised surface.

**Parallel-implementer discipline.** `internal/cli/prepare.go`,
`internal/workflow/workflow.go`, `internal/workflow/doctor.go` and
`internal/rescap/lock_unix.go` are the shared surfaces. Per AGENTS.md, same-file
overlap is a hard trigger for sequential execution: **no two implementers may
touch any one of those files.** The cluster lead must declare the partition at
dispatch, every implementer stages by explicit path, and `git commit -a`,
`git add .`, `git add -A` and directory-scope adds are forbidden for this
cluster.

**Golden prerequisite.** S2's pre-change goldens for `analyze`, `define`,
`explore`, `cycle`, `next` and `doctor` D1…D8, and S1b's for
`feature resource *`, must be captured and committed **before** the
corresponding refactor lands, or PIB-186, PIB-208…PIB-212 and PIB-286…PIB-287
degrade into comparing a changed binary against itself.

## 18. Acceptance matrix

### 18.1 How to read this matrix

IDs are stable (`PIB-NNN`) and are never renumbered; a retired row is struck
through and keeps its number. Each row names the **observable** the test must
assert.

**A test does not satisfy a row merely by existing.** A row is satisfied only
when its test asserts the named observable. Disqualifying assertion shapes,
inherited from the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:3531-3548`):

- asserting "the command did not return an error" satisfies no row;
- asserting "some JSON was produced" satisfies no row that names a field,
  value, order or exit code;
- a row naming an exit code must assert the numeric **process** exit code, not
  the presence of a Go error;
- a row naming "byte-identical" must compare bytes, not lengths, sizes or
  mtimes;
- a row naming a state, code or enum must assert that exact literal, not a
  truthy proxy;
- a row naming a **guard** must assert the guard's semantics — that it fails on
  a wrong input — not that the guard function or its test file exists;
- a row naming "zero writes" must be asserted with a filesystem spy or a
  whole-tree byte snapshot, never by re-reading one file.

If a row cannot be placed as written, the PRD is amended — the row is not
silently re-tiered.

Legend for **Kind**: `U` unit, `I` integration (real CLI invocation over a real
temp workspace), `S` source scan (AST), `G` mechanical guard, `C` crash/fault
injection through the declared seams.

**Injection seams.** Crash and failure rows are driven through named,
production-inert seams: `beforeLockAcquire`, `failLockAcquire`,
`beforeRedactionScan`, `beforeBlobWrite`, `afterBlobWrite`,
`beforeJournalWrite`, `afterJournalWrite`, `beforeEntryCAS(i)`,
`beforeRename(i)`, `afterRename(i)`, `beforeIndexRewrite`,
`beforeStatusRename`, `afterStatusRename`, `beforeFinalVerify`,
`beforeJournalClear`, `beforeLockRelease`, `beforeBlobRemove`,
`beforeManualStatusCAS`, `beforePurgeIndexCAS`, `afterPurgeIndexRename`,
`beforeRehydrateIndexRename`, `failFsync(path)`, `failRename(path)`. Each is a function-valued package
variable that is `nil` in production; PIB-232 asserts every one is `nil` at
init and that no production call path assigns one.

**Amended rows.** IDs are never renumbered. Rows whose *meaning* changed in
rev-1 — `PIB-015`, `PIB-016`, `PIB-017`, `PIB-020`, `PIB-041`, `PIB-053`,
`PIB-079`, `PIB-110`, `PIB-117`, `PIB-118`, `PIB-124`, `PIB-196`, `PIB-197`,
`PIB-215`, `PIB-227` — were amended in place, and each carries the same ID it
had in rev-0. **Rev-2 additionally amends in place** `PIB-051`, `PIB-052`,
`PIB-053`, `PIB-079`, `PIB-080`, `PIB-103`, `PIB-117`, `PIB-118`,
`PIB-124`, `PIB-134`, `PIB-135`, `PIB-139`, `PIB-221`…`PIB-223`,
`PIB-279`, `PIB-282`, `PIB-284`, `PIB-285`, `PIB-288`…`PIB-290`,
`PIB-310`, `PIB-312`, `PIB-313`, `PIB-321`…`PIB-323`,
`PIB-327`…`PIB-329`, `PIB-340`, `PIB-349`, `PIB-352`, `PIB-354`,
`PIB-380`, `PIB-381`, `PIB-386`, and `PIB-392`…`PIB-394`.
The only new rev-2 IDs are the contiguous `PIB-395`…`PIB-409`.

### 18.2 A — CLI grammar, modes and flag mutexes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-001 | I | `prepare <slug>` on a feature missing `spec.md` | exit 0; `spec.md` created; report `outcome: published` |
| PIB-002 | I | `prepare` with no slug | exit 1; cobra arity text; zero writes |
| PIB-003 | I | `prepare a b` | exit 1; zero writes |
| PIB-004 | I | `prepare <slug> --manual --regenerate` | exit 1; cobra mutual-exclusion text; zero writes |
| PIB-005 | I | `prepare <slug> --check --manual` | exit 1; mutual-exclusion text; zero writes |
| PIB-006 | I | `prepare <slug> --check --regenerate` | exit 1; mutual-exclusion text; zero writes |
| PIB-007 | I | `prepare <slug> --check --timeout 5s` | exit 1; mutual-exclusion text |
| PIB-008 | I | `prepare <slug> --manual --timeout 5s` | exit 1; mutual-exclusion text |
| PIB-009 | I | `prepare <slug> --manual --no-retry` | exit 1; mutual-exclusion text |
| PIB-010 | I | `prepare <slug> --check --dry-run` | exit 1; mutual-exclusion text |
| PIB-011 | I | `prepare <slug> --check --manual` | zero mutation: whole `.tpatch/` tree byte-identical |
| PIB-012 | I | `prepare <slug> --check --regenerate` | zero mutation: whole `.tpatch/` tree byte-identical |
| PIB-013 | I | `prepare <slug>` (no `--check`) in a workspace | exit is **not** 4; a real run occurs |
| PIB-014 | G | source scan over `internal/cli/**` | the retired reserved-surface refusal string is absent, and no code path constructs `ExitCodeError{Code: 4}` in the `prepare` command file |
| PIB-015 | I | table over all seven codes | each of 0,1,2,3,5,6 is reachable by a named input, including exit 3 for lock contention and exit 6 for an undo-CAS refusal; 4 is unreachable by every input in the table |
| PIB-016 | I | `prepare --help` | lists all five modes; states it is unrelated to `apply --mode prepare`; names `--regenerate` as the only overwrite route and `--allow-heuristic` as its only downgrade opt-in |
| PIB-017 | I | `apply --help` | `--mode` description still points at `prepare`; text updated for five modes |
| PIB-018 | I | `prepare <slug> --json --quiet` on success | stdout is exactly one JSON document; stderr empty |
| PIB-019 | I | `prepare <slug> --quiet` on success | stdout is exactly one line ending in the outcome token |
| PIB-020 | S | the `prepare` command's flag registration | exactly twelve flags registered: `check`, `manual`, `regenerate`, `abandon-transaction`, `allow-heuristic`, `dry-run`, `yes`, `json`, `quiet`, `timeout`, `timeout-phase`, `no-retry`, plus the inherited persistent `path`; no `--all`, `--fix`, `--force`, `--restore`, `--format`, `--interactive` |

### 18.3 B — Default mode: preservation and missing-only

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-021 | I | all three Markdown files missing | all three created; exit 0; state `defined` |
| PIB-022 | I | `analysis.md` present-nonempty, other two missing | `analysis.md` **byte-identical** afterwards; other two created |
| PIB-023 | C | same as PIB-022, with a write spy | zero `OpenFile`-for-write, zero rename targeting `analysis.md` |
| PIB-024 | I | all three present-nonempty, state `defined` | zero bytes written anywhere in `.tpatch/`, including `status.json` |
| PIB-025 | U | `spec.md` present-empty (zero bytes) | refusal `artifact-empty-not-overwritten`, exit 2 |
| PIB-026 | U | `spec.md` whitespace-only | same as PIB-025 (whitespace-only is empty per the accepted rule) |
| PIB-027 | U | `spec.md` is a symlink to an in-repo file | refusal `artifact-unsafe`, exit 3; symlink not followed; target string absent from all output |
| PIB-028 | U | `exploration.md` is a directory | refusal `artifact-unsafe`, exit 3 |
| PIB-029 | U | `analysis.md` is a FIFO | refusal `artifact-unsafe`, exit 3; no open of it |
| PIB-030 | I | `analysis.md` + `spec.md` present, `exploration.md` absent | only `exploration.md` is created; the other two and the sidecar byte-identical |
| PIB-031 | U | `spec.md` mode 0000 | refusal `artifact-unsafe`, exit 3 |
| PIB-032 | I | Path A feature: `analysis.md` preserved, sidecar present | sidecar byte-identical afterwards |
| PIB-033 | I | Path B feature: `analysis.md` preserved, sidecar absent | sidecar still absent; advisory `analysis-preserved-sidecar-untouched` present |
| PIB-034 | I | zero-byte `spec.md` reached `defined` via `define --manual` | `prepare` refuses exit 2; `spec.md` still zero bytes |
| PIB-035 | I | complete bundle, state `defined` | `outcome: no-op`, `action: none`, exit 0, `status.json` byte-identical (including `updated_at`) |
| PIB-036 | I | complete bundle, state `analyzed` | `action: adopt`; only `status.json` changes; the four artifacts byte-identical |
| PIB-037 | U | `analysis.md` oversize (cap + 1) | refusal `artifact-unsafe`, exit 3; the file is never opened |
| PIB-038 | C | `spec.md` grows during its capture | classified `unstable`; refusal `artifact-unstable`, exit 3 |
| PIB-039 | U | sidecar is `invalid-structured`, all Markdown present-nonempty | no refusal; sidecar untouched; exit 0 with `action: none` |
| PIB-040 | U | sidecar is a symlink, all Markdown present-nonempty | no refusal (optional artifact never gates admissibility); sidecar untouched |
| PIB-041 | I | `analysis.md` generated this run and the sidecar is `absent` | the sidecar is created in the same publication; with a sidecar present in any state the run refuses instead (PIB-253) |
| PIB-042 | I | `analysis.md` preserved, `spec.md` generated | sidecar **not** in the publication set; a rename spy records none targeting it |
| PIB-043 | I | generated `spec.md` | its provider prompt contains the preserved `analysis.md`'s bytes (context flow) |
| PIB-044 | G | the §6.1 disposition table | its Go representation covers all nine accepted enum values with no `default` fallthrough |

### 18.4 C — `--manual` adoption

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-045 | I | `--manual` on a ready bundle, provider spy installed | zero provider calls |
| PIB-046 | I | `--manual` on a ready bundle | all four artifact files byte-identical afterwards |
| PIB-047 | I | `--manual` with `exploration.md` absent | exit 2; full per-artifact report; zero mutation |
| PIB-048 | I | `--manual` with zero-byte `spec.md` | exit 2; zero mutation |
| PIB-049 | I | `--manual` success with `FEATURES.md` unwritable | exit 0; `status.json` written; the index-refresh failure does not fail the command |
| PIB-050 | I | `--manual` success | `state=defined`, `last_command=prepare`, notes equal the frozen adoption string |
| PIB-051 | C | `--manual` success, rooted rename spy | exactly one rooted temp→`status.json` rename after immediate preimage CAS |
| PIB-052 | C | `--manual` success, journal/archive spy | no journal or archive directory is created; the rooted single-file CAS remains required |
| PIB-053 | I | `--manual` while a live process holds the external cache authority | exit 3, `transaction-in-progress`; zero mutation; the remediation says to wait and retry |
| PIB-054 | I | `--manual` with `analysis.md` symlinked | exit 3, `artifact-unsafe`; zero mutation |
| PIB-055 | I | `--manual` from state `requested` with a complete bundle | exit 0; advisory `feature-state-below-defined`; state `defined` |
| PIB-056 | I | `--manual` on a feature already `defined` with a complete bundle | exit 0; `status.json` rewritten only if a field actually changes, else `no-op` |
| PIB-057 | I | `--manual --json` refusal | report carries `mode: manual`, `outcome: refused`, and the per-artifact rows |
| PIB-058 | I | `--manual` with `status.json` malformed | exit 3, `status-malformed`; zero mutation |
| PIB-059 | S | the `--manual` code path | contains no provider construction and no artifact write call |

### 18.5 D — `--regenerate` and the archive

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-060 | I | `--regenerate` on a full hand-authored bundle | each prior file's exact bytes exist at `intent-archive/blobs/<sha256>.blob` |
| PIB-061 | I | `--regenerate` with `spec.md` symlinked | exit 3; **nothing** archived, nothing regenerated, nothing renamed |
| PIB-062 | C | `--regenerate` with the blob write failing (`failFsync`) | exit 5 or 3; zero canonical mutations |
| PIB-063 | I | `--regenerate` where a blob file already exists with equal bytes | it is not rewritten (mtime and inode unchanged); advisory `archive-blob-reused` |
| PIB-064 | I | `--regenerate` where a blob file exists with **different** bytes for its name | exit 3, `archive-blob-corrupt`; nothing overwritten |
| PIB-065 | U | `generation_id` collision, injected through the derivation seam | exit 3, `archive-generation-id-collision`; index unchanged |
| PIB-066 | I | `--regenerate` with an `unreadable` required artifact | exit 3; nothing archived, nothing published |
| PIB-067 | I | `--regenerate` over a zero-byte `spec.md` | the zero-byte file is archived; new `spec.md` non-empty; exit 0 |
| PIB-068 | I | a workspace that never regenerates | no `intent-archive/` directory exists anywhere |
| PIB-069 | I | `--regenerate` twice with identical prior content | the second appends **no** index entry; advisory `archive-generation-duplicate` |
| PIB-070 | I | `--regenerate` | all three Markdown files **and** the sidecar are replaced; no partial-scope option exists |
| PIB-071 | I | `--regenerate` success report | names each archived blob and prints the `cp` restore form verbatim |

### 18.6 E — `--dry-run`

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-072 | C | `--dry-run` in every mode, filesystem spy | zero `mkdir`, `create`, `write`, `rename`, `remove` calls of any kind |
| PIB-073 | I | `--dry-run`, provider spy | zero provider calls |
| PIB-074 | I | `--dry-run` on an admissible plan | exit 0; `outcome: planned`; `dry_run: true` |
| PIB-075 | I | `--dry-run` on a `present-empty` required artifact (default mode) | exit 2, same refusal code as the real run |
| PIB-076 | I | `--dry-run --manual` on an incomplete bundle | exit 2, same refusal code as the real run |
| PIB-077 | I | `--dry-run` report | contains the verbatim plan-only sentence; contains no `generator` value and no archive hash |
| PIB-078 | I | `--dry-run --regenerate` | lists every artifact it would archive; creates no archive directory |
| PIB-079 | C | `--dry-run` with a pending journal | exit 3, `recovery-pending`; recovery does **not** run; the journal is byte-identical; no plan or external cache lock is created |
| PIB-080 | C | `--dry-run`, lock spy | the external cache lock locator is never created |

### 18.7 F — Staging, validation and no partial canonical write

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-081 | C | generation of the 2nd of 3 artifacts fails hard | zero canonical files created or modified; whole `.tpatch/features/<slug>/` byte-identical |
| PIB-082 | C | staged `spec.md` is empty (V1) | exit 2; zero canonical mutations; staging retained |
| PIB-083 | C | staged `spec.md` is whitespace-only (V1) | exit 2; zero canonical mutations |
| PIB-084 | C | staged `analysis.md` exceeds the cap (V2) | exit 2; zero canonical mutations |
| PIB-085 | C | staged bytes contain a NUL (V3) | exit 2; zero canonical mutations |
| PIB-086 | C | staged bytes are invalid UTF-8 (V4) | exit 2; zero canonical mutations |
| PIB-087 | C | staged sidecar is `[1,2,3]` (V5) | exit 2; zero canonical mutations |
| PIB-088 | C | staged sidecar is `{` (V5) | exit 2; zero canonical mutations |
| PIB-089 | C | a staged file changes identity after fsync (V6) | exit 5; zero canonical mutations |
| PIB-090 | I | successful run | the staging tree no longer exists and its parent is empty of `stage-*` |
| PIB-091 | I | failed run | the staging tree still exists; the report names its repo-relative path; advisory `staging-retained` |
| PIB-092 | I | a second successful run after a failed one | the earlier retained staging tree is removed |
| PIB-093 | I | staging tree permissions | directory `0700`; files `0600` |
| PIB-094 | I | `.tpatch/local/` not covered by an ignore rule | exit 3, `local-lane-not-ignored`, before any staging byte |
| PIB-095 | I | a tracked file exists under `.tpatch/local/` | exit 3, `local-lane-not-ignored`; zero mutation |
| PIB-096 | C | staged files after generation | each is fsynced and its identity recorded before the journal is written |
| PIB-097 | S | the staging path builder | derives from the validated slug and `RandomHex12` only; never from provider output |
| PIB-098 | I | run that fails after staging | staging retained; canonical tree byte-identical |
| PIB-099 | I | run that succeeds | staging removed; parent directory fsynced |

### 18.8 G — Publication, revalidation and concurrency

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-100 | C | an external writer modifies `spec.md` between preflight and revalidation | exit 5 before the first rename; zero canonical mutations; staging retained |
| PIB-101 | C | an artifact appears where preflight saw `absent` | exit 5; the appeared file is byte-identical afterwards |
| PIB-102 | C | an artifact disappears between preflight and revalidation | exit 5; zero canonical mutations |
| PIB-103 | I | documented limit: a write landing inside the window | test asserts the shipped docs and messages state the limit; **no row claims the write is preserved** |
| PIB-104 | I | a concurrent `tpatch define` completing before revalidation | exit 5; `define`'s bytes preserved |
| PIB-105 | I | `.tpatch/` replaced wholesale between preflight and revalidation | exit 5 or 3; nothing published |
| PIB-106 | I | any successful `prepare` in a Git repo with a dirty index | `git status --porcelain` shows only the expected new/modified `.tpatch` paths; the index checksum is unchanged |
| PIB-107 | S | the whole `prepare` call graph | no `exec.Command("git", …)`, no `gitutil` write entry point |
| PIB-108 | C | rename order | the observed rename sequence equals §7.2's order, with `status.json` last |
| PIB-109 | C | fsync sequence | each entry's parent directory is fsynced after its rename; the journal directory is fsynced after clear |
| PIB-110 | C | rename 2 of 4 fails; rollback succeeds | exit 5; every canonical file, `status.json` and the archive index byte-identical to the pre-run state; journal cleared; the message does **not** claim the whole tree is unchanged, and lists any orphan blob |
| PIB-111 | C | rename 2 of 4 fails and rollback also fails | exit 6; journal **retained**; report names the journal, the archive and the failing entry |
| PIB-112 | C | rollback attempted when a published entry no longer matches its new-image | rollback refuses that entry; exit 6; nothing overwritten |

### 18.9 H — Crash injection and recovery

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-113 | C | recovery run twice | the second is a no-op; no journal; identical tree |
| PIB-114 | C | recovery with two `stage-*` trees for the slug | both removed; no blob removed; the index untouched |
| PIB-115 | C | recovery with a pending journal for another slug | that slug's lane is untouched |
| PIB-116 | C | crash phase CP0 (before lock) | next run proceeds normally |
| PIB-117 | C | crash phase CP1 (external cache authority held, no journal) | the next run acquires the same persistent cache lock after death; staging is removed; run proceeds |
| PIB-118 | C | crash phase CP2 (blobs, no journal) | blobs remain as reported orphans; the external locator stays outside cleanup; no index entry was added; advisory names purge |
| PIB-119 | C | crash phase CP3 (journal, no rename) | journal cleared; every canonical file byte-identical to pre-run |
| PIB-120 | C | crash phase CP4 (2 of 4 renamed) | the 2 published entries are restored to preimage; journal cleared |
| PIB-121 | C | crash phase CP5 (artifacts new, index old) | all restored; journal cleared |
| PIB-122 | C | crash phase CP6 (index new, status old) | all restored including `index.json`; journal cleared |
| PIB-123 | C | crash phase CP7 (everything new, journal not cleared) | recovery **undoes nothing**; journal cleared; tree stays all-new |
| PIB-124 | I | a second mutating `prepare` while a live process holds the external cache lock | exit 3, `transaction-in-progress`; the first run's outcome unaffected |
| PIB-125 | I | two mutating `prepare` runs on different slugs | both succeed; neither lane touched by the other |
| PIB-126 | C | crash phase CP9 (a third party rewrote a published entry) | exit 6; every file, the journal and the archive preserved and named |

### 18.10 I — Lifecycle, states and status metadata

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-127 | I | `prepare` on a `rejected` feature | exit 3, `state-refused`; remediation names `tpatch reopen <slug>`; zero mutation |
| PIB-128 | I | reopen, then `prepare` | allowed iff the restored `PriorState` is in the allowed set |
| PIB-129 | I | successful `prepare` on a feature with a `verify` record | `status.verify` is byte-identical afterwards |
| PIB-130 | I | `define --manual` on a zero-byte `spec.md` | exit 0; state `defined` — unchanged loose behavior |
| PIB-131 | I | `prepare --manual` on the same tree | exit 2 |
| PIB-132 | I | composite of PIB-130 then PIB-131 in one workspace | both observables in one run; zero mutation from the second |
| PIB-133 | I | `doctor` with a pending prepare journal | a `D9` warning naming the slug; exit code unchanged from the no-journal case |
| PIB-134 | I | `doctor` with a completely clean lane/archive/cache locator state | no `D9` output at all |
| PIB-135 | C | `doctor` with a pending journal, filesystem/cache spy | zero writes; the external cache lock is never created |
| PIB-136 | G | `doctor` D1…D8 output vs pre-change goldens | byte-identical |
| PIB-137 | I | legacy `status.json` with no optional fields | after a successful `prepare`, only `state`/`last_command`/`updated_at`/`notes` differ; no field is added |
| PIB-138 | I | feature with no `artifacts/` directory | created 0755; no other feature's directory touched |
| PIB-139 | I | G1-established Git worktree with untracked `.tpatch/` | exit 0; advisory `bundle-untracked-in-git` present |

### 18.11 J — Provenance boundary

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-140 | I | `prepare --check` after any mutating `prepare` | every artifact still reports `provenance: "unknown"` |
| PIB-141 | S | the inspector's call graph | it never reads `intent-archive/**`; no import, no path constant |
| PIB-142 | G | every shipped string of both report schemas | no field, value or message asserts Path A vs Path B for a feature |
| PIB-143 | S | the shipped forbidden-inference list | it contains an `intent-archive` entry |
| PIB-144 | G | `generator` field | present only in the mutating report; never written to any tracked file; a guard asserts the token appears in no `.tpatch/` byte after a run |
| PIB-145 | G | source + docs scan | ADR-034 is never cited as precedent for persistence; ADR-035 governs every write |
| PIB-146 | G | heuristic-mode run | the sidecar carries `heuristic_mode: true`, byte-compatible with `analyze`'s output for the same input |
| PIB-147 | G | the archive index and blob layout | carry no author, agent, model, provider, endpoint, or Path-A/Path-B field; sensitivity fixture proves the guard fails when one is added |

### 18.12 K — Security, privacy and forbidden fields

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-148 | U | publication target is a symlink at rename time | transaction aborts (exit 5); the symlink and its target are byte-identical afterwards |
| PIB-149 | U | publication target is a directory at rename time | exit 5; nothing written |
| PIB-150 | U | publication target is a FIFO at rename time | exit 5; nothing written; no open of it |
| PIB-151 | U | `artifacts/` is a symlink at publication time | exit 5; not followed |
| PIB-152 | I | `land` after a `--regenerate` | archive blobs and `index.json` are staged like any other `artifacts/**` file; no new refusal |
| PIB-153 | I | `record` after a `--regenerate` | `artifacts/post-apply.patch` is byte-identical to the same run without an archive present |
| PIB-154 | S | every write target constructor | derives from the compile-time name list plus the validated slug; no write path derives from provider output or file content |
| PIB-155 | G | every shipped message, help string and PRD/ADR sentence this cluster owns | no occurrence of "atomic", "atomically" or "simultaneously" applied to the multi-file publication; the guard fails when a fixture sentence adds one |
| PIB-156 | I | any refusal | the raw `<slug>` argument is never echoed when it fails canonical validation |
| PIB-157 | C | a provider response larger than the cap | V2 rejects it; exit 2; zero canonical mutations; the oversized bytes never reach a tracked path |
| PIB-158 | G | JSON report key walk at every nesting level | no key from the forbidden set (absolute path, timestamp, duration, hostname, pid, content, symlink target, secret, token, env value) |
| PIB-159 | G | sensitivity fixture for PIB-158 | injecting a `created_at` key makes the guard fail |

### 18.13 L — Determinism and canonical encoding

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-160 | U | two archive publications of identical prior bytes | byte-identical blob files and byte-identical index entries |
| PIB-161 | G | `index.json` bytes | no wall-clock field; key order fixed; `replaced` sorted by `artifact_id` |
| PIB-162 | U | `generation_id` derivation | one derivation point; equal canonical bodies produce equal ids; a one-byte change produces a different id |
| PIB-163 | I | success report | the `cp` restore form is printed verbatim and names no verb that does not exist |
| PIB-164 | G | journal bytes | no wall-clock field; no `phase` field; strict decode rejects an unknown field and trailing content |
| PIB-165 | S | every wire struct this PRD adds | no Go `map` type in any of them |
| PIB-166 | U | JSON report key order | equals the declared struct order in both the ordinary and refusal shapes |
| PIB-167 | I | two identical heuristic runs on identical trees | artifacts and reports byte-identical; only `status.json`'s `updated_at` differs |
| PIB-168 | U | archive index round-trip | decode→encode is byte-identical for a fixture with three generations |
| PIB-169 | G | blob filenames | match `^[0-9a-f]{64}\.blob$`; never uppercase hex, never truncated |

### 18.14 M — Output, exits and precedence

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-170 | I | default output | progress lines appear on **stderr**, never stdout |
| PIB-171 | I | `--json` and `--quiet` | no progress line appears on either stream |
| PIB-172 | I | both report schemas | `command` is `"prepare --check"` vs `"prepare"`; a consumer can switch on it |
| PIB-173 | I | refusal before inspection (bad slug) | `artifacts` is `[]`; `archive` omitted; `refusal.code` from the closed set |
| PIB-174 | I | refusal after inspection | `artifacts` carries the planned dispositions; `refusal.code` set |
| PIB-175 | G | advisory selection | a total function over observed state; no advisory can contradict its own artifact row; sensitivity fixture proves it fails when one does |
| PIB-176 | I | `prepare ../../etc --check`-less form outside a workspace | slug refusal (exit 3) precedes workspace discovery; the raw argument is not echoed |
| PIB-177 | C | pending journal plus a `rejected` feature | recovery runs and clears the journal **before** the exit-3 lifecycle refusal is returned |
| PIB-178 | I | every emission, both surfaces | the frozen disclaimer string byte-for-byte |
| PIB-179 | I | every refusal code | remediation names only shipped commands/flags/paths |
| PIB-180 | G | every refusal string | contains no `docs/`, no `.md`, no `http` |
| PIB-181 | S | the read path | no `os.Stat`, `os.Lstat`, `os.Open`, `os.ReadFile`, `filepath.Join(repoRoot, …)` |
| PIB-182 | S | the write path | targets come from the closed compile-time name list only |
| PIB-183 | U | pre-rename `Lstat` gate | a non-regular, non-absent target aborts before the rename |

### 18.15 N — Generator extraction and generation semantics

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-184 | S | the three extracted generators | none takes a `*store.Store` and none holds a writer |
| PIB-185 | C | the three extracted generators, filesystem spy | zero writes of any kind |
| PIB-186 | G | `analyze`/`define`/`explore` after the refactor | stdout, stderr, exit code and every written byte identical to pre-change goldens |
| PIB-187 | U | preserved artifact read as generation context | goes through the accepted bounded rooted capture; an unsafe one already refused at admissibility |
| PIB-188 | C | provider retry with `--no-retry` unset | raw responses land in the staging tree, not `artifacts/` |
| PIB-189 | S | the `prepare` generation path | `RetryOptions.Store` is nil at every construction site |
| PIB-190 | I | failed generation with retries | the retained staging tree contains the raw responses; `artifacts/` contains none |
| PIB-191 | G | shipped strings and docs | no claim that the command cannot hang or has bounded runtime |
| PIB-192 | C | provider deadline expiry during generation | heuristic fallback for that artifact; exit 0; advisory `provider-fallback-heuristic` |
| PIB-193 | C | deadline expiry after staging, before publication | the publication proceeds; exit 0 |
| PIB-194 | G | the validation set | exactly V1…V6; no check inspects headings, length or topicality |
| PIB-195 | I | successful `prepare`, then `prepare --check` in the same tree | `structural_readiness: ready`, exit 0 |
| PIB-196 | I | no provider configured, **default mode** | all missing artifacts generated heuristically; exit 0; advisory `provider-not-configured` |
| PIB-197 | I | provider configured but unreachable, **default mode** | heuristic fallback, exit 0 — **not** a command failure; `--regenerate` on the same tree refuses instead (PIB-369) |

### 18.16 O — `--check` compatibility

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-198 | G | `--check` human output vs pre-change goldens | byte-identical across the ready / not-ready / abort populations |
| PIB-199 | G | `--check --json` vs pre-change goldens | byte-identical |
| PIB-200 | I | `--check` exit codes | 0/2/3 unchanged for every accepted population |
| PIB-201 | C | `--check`, filesystem spy | zero writes |
| PIB-202 | I | `--check` | `provenance: "unknown"` for all four artifacts |
| PIB-203 | I | `--check` | the frozen disclaimer byte-for-byte |
| PIB-204 | I | `--check` abort populations | `artifacts` is `[]` iff `abort` is present |
| PIB-205 | I | `--check` | the accepted `overall` totals unchanged |
| PIB-206 | C | `--check` executed inside a publication window | reports what is on disk; a mid-rename artifact may be `unstable`; no crash, no lock, no write |
| PIB-207 | G | `--check` with and without a pending journal, identical canonical bytes | byte-identical output |

### 18.17 P — Non-invalidation of shipped commands

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-208 | G | `next` for all twelve states, incl. the `exploration.md` branch | byte-identical to pre-change goldens |
| PIB-209 | G | `cycle` end-to-end | byte-identical to pre-change goldens |
| PIB-210 | G | `analyze\|define\|explore\|implement` | byte-identical to pre-change goldens |
| PIB-211 | G | the same four with `--manual` | byte-identical to pre-change goldens |
| PIB-212 | G | `status`, `verify`, `record`, `land`, `reconcile` on a feature that never ran `prepare` | byte-identical to pre-change goldens |
| PIB-213 | S | reverse call graph | nothing imports or invokes the prepare publication package except the `prepare` command file; `prepare` is not an `OnComplete` in `next` and not a step in `cycle` |
| PIB-214 | S | `store.Store` writers | no new writer is added to `internal/store` beyond the archive functions named in S3 |

### 18.18 Q — Docs, skills and asset parity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-215 | G | `SPEC.md` | documents five modes, the complete flag mutex table, all seven exit-code rows including retired `4`, the `FEATURES.md` carve-out and the `feature intent-archive` grammar |
| PIB-216 | G | all six skill files | each names `tpatch prepare`; `requiredCommands` extended |
| PIB-217 | G | all six skill files | `prepare` appears in **no** phase-ordering table and **no** preflight block; sensitivity fixture proves the guard fails when it is added |
| PIB-218 | G | all six skill files | no sentence claims semantic certification or provenance |
| PIB-219 | G | `docs/feature-layout.md` | documents `intent-archive/`, its immutability, that it is never canonical truth, and the `cp` restore form |
| PIB-220 | G | `docs/agent-as-provider.md` | carries the `prepare --manual` row and the corrected `notes` sentence |

### 18.19 R — Platform and build

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-221 | G | `go vet` and supported-target cross-build | clean for linux/amd64, linux/arm64, darwin/arm64, windows/amd64; no BSD mutation-support assertion |
| PIB-222 | I | the full mutating flow on native `windows-latest` | rooted publication, external deny-share authority, rollback and recovery pass; junction fixtures **fail** rather than skip |
| PIB-223 | U | `freebsd` and another unsupported mutating `GOOS` | mutation refuses before workspace/cache root open; accepted read-only check keeps its own allowlist |

### 18.20 S — Totality, ledger and sensitivity guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-224 | G | §6.1 and §6.3 disposition tables | total over the accepted nine-value enum; adding a tenth value fails compilation or the guard |
| PIB-225 | G | §12.2 state table | total over `ValidFeatureState`; a thirteenth state fails the guard rather than defaulting to allowed |
| PIB-226 | G | the closed vocabularies of §10.2 | the shipped constant sets equal the tables exactly |
| PIB-227 | G | the advisory catalog | exactly seventeen codes; every one reachable by a named fixture; a sensitivity fixture proves an eighteenth code fails the guard |
| PIB-228 | G | the refusal-code catalog | closed; every code reachable; every code has a remediation |
| PIB-229 | G | every `PIB-NNN` cited in prose | resolves to a real matrix row |
| PIB-230 | G | the acceptance ledger | every row maps to a resolvable package, `func TestX(*testing.T)` declaration and optional literal subtest, via AST — not a byte scan |
| PIB-231 | G | sensitivity meta-check | every row whose Kind contains `G` has a fixture proving the guard fails on a wrong input |
| PIB-232 | G | the injection seams of §18.1 | all are `nil` at init; no production path assigns one; a sensitivity fixture proves the guard fails when one is assigned |

### 18.21 T — Feature-request input

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-233 | I | `request.md` absent, empty, symlinked or unreadable, in `generate` and `regenerate` modes | exit 3, `request-unreadable`; zero mutation; a provider spy records zero calls |
| PIB-234 | I | `request.md` absent, `--manual` mode on a complete bundle | exit 0; adoption succeeds; generation is not required |

### 18.22 U — Flag deltas and grammar completeness

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-235 | I | `prepare <slug> --check --dry-run` | exit 1; mutual-exclusion text; whole `.tpatch/` tree byte-identical |
| PIB-236 | I | `prepare <slug> --manual` before/after fixture pair | before: exit 1 `unknown flag`; after: adoption path reached — the §5.3 row asserted as a pair, not as prose |
| PIB-237 | I | `prepare <slug> --regenerate` before/after pair | before: exit 1 `unknown flag`; after: the regenerate path is reached (provider gate may then refuse) |
| PIB-238 | I | `prepare <slug> --abandon-transaction` before/after pair | before: exit 1; after: preview printed, zero writes |
| PIB-239 | I | `prepare <slug> --allow-heuristic` alone (no `--regenerate`) | exit 0 with advisory `allow-heuristic-redundant` in default mode; exit 1 with `--check`/`--manual`/`--abandon-transaction` |
| PIB-240 | I | `prepare <slug> --dry-run` before/after pair | before: exit 1; after: `outcome: planned` |
| PIB-241 | I | `prepare <slug> --timeout 5s` and `--timeout-phase 5s` | both accepted in `generate`/`regenerate`; both exit 1 with `--check`, `--manual`, `--abandon-transaction` |
| PIB-242 | I | `prepare <slug> --no-retry` | accepted in `generate`/`regenerate`; exit 1 in the other three modes |
| PIB-243 | I | `prepare <slug> --yes` without `--abandon-transaction` | exit 1; mutual-exclusion text; zero writes |
| PIB-244 | G | the §5.3 flag delta table vs the shipped flag set | every registered flag has exactly one delta row and every delta row names a registered flag; a sensitivity fixture adds a flag and proves the guard fails |

### 18.23 V — Default-mode coherence and sidecar preservation

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-245 | I | `analysis.md` absent, `spec.md` present-nonempty | exit 2, `incoherent-bundle-gap`; zero mutation; nothing generated |
| PIB-246 | I | `analysis.md` absent, `exploration.md` present | exit 2, `incoherent-bundle-gap`; zero mutation |
| PIB-247 | I | `analysis.md` absent, `spec.md` and `exploration.md` present | exit 2, `incoherent-bundle-gap`; zero mutation |
| PIB-248 | I | `analysis.md` present, `spec.md` absent, `exploration.md` present | exit 2, `incoherent-bundle-gap`; zero mutation |
| PIB-249 | I | the `incoherent-bundle-gap` message | names `--regenerate` and `--manual` as the two routes; contains no `rm`, no `docs/`, no `.md` path |
| PIB-250 | G | the §6.1.2 coherence table | total over all eight present/absent combinations; a ninth synthetic combination fails the guard rather than defaulting to "generate" |
| PIB-251 | C | default mode, `analysis.md` preserved, sidecar `present-nonempty`, write spy | zero `OpenFile`-for-write and zero rename targeting `artifacts/analysis.json`; sidecar byte-identical |
| PIB-252 | I | default mode, `analysis.md` preserved, sidecar `present-empty` | sidecar still zero bytes afterwards; no refusal caused by it; exit 0 or `no-op` per the Markdown states |
| PIB-253 | I | `analysis.md` absent, sidecar `present-nonempty` | exit 2, `incoherent-bundle-gap`; sidecar byte-identical; no analysis generated |
| PIB-254 | I | `analysis.md` absent, sidecar `invalid-structured` | exit 2, `incoherent-bundle-gap`; sidecar byte-identical |
| PIB-255 | I | any successful default-mode run | no `artifacts/intent-archive/` directory exists afterwards |
| PIB-256 | C | any default-mode run, filesystem spy | zero writes under `artifacts/intent-archive/`; zero blob creations; the journal contains no `replace` entry |

### 18.24 W — `FEATURES.md`, status truth and the T1 carve-out

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-257 | C | `--manual` success, filesystem spy over the whole repo | exactly two write targets: `status.json` (via one rename) and `FEATURES.md`; the report's write-set statement matches the spy |
| PIB-258 | G | every shipped string and doc sentence this PRD owns | none claims `--manual` writes exactly one file, and none includes `FEATURES.md` in the publication unit |
| PIB-259 | C | publication rolled back after `status.json` was published | `status.json` is back to its preimage **and** a second `FEATURES.md` refresh ran; the index names the restored state |
| PIB-260 | C | recovery (next invocation) restores `status.json` | the same best-effort refresh runs as recovery's last act; failure of the refresh does not change the exit code |
| PIB-261 | I | `FEATURES.md` unwritable during a successful publication | exit 0; `status.json` correct; advisory `features-index-refresh-failed` present; no row and no message claims `FEATURES.md` is transactional |

### 18.25 X — Archive redaction gate and retention disclosure

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-262 | C | `--regenerate` where `spec.md` contains a `bearer-or-key-token` match, blob-write spy | exit 3, `archive-content-refused-sensitive`; **zero** blob files created; zero canonical mutations; the scan ran before the first blob write |
| PIB-263 | I | the same refusal's human and JSON output | names the artifact id and the matched class codes; contains no matched bytes, no excerpt, no offset, no line number |
| PIB-264 | U | one fixture per class (`private-key`, `connection-url`, `email-pii`, `credential-assignment`, `bearer-or-key-token`, `home-absolute-path`) | each refuses `--regenerate` with the same code and names its own class |
| PIB-265 | I | `--regenerate` after the operator removes the sensitive line | exit 0; the cleaned bytes are archived; the earlier refusal left nothing behind |
| PIB-266 | S | the archive write path | `redact.Scan` is called on in-memory bytes before any blob file is created; no code path writes a blob first and scans after |
| PIB-267 | G | shipped strings and docs | no flag, config key or message offers to skip or override the redaction scan; a sensitivity fixture adding a `--force-archive` string fails the guard |

### 18.26 Y — Dry-run with pending recovery, and the abandon route

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-268 | I | `--dry-run` with a pending journal, all three modes | exit 3, `recovery-pending`; no plan printed; journal/staging remain byte-identical and no external cache locator is created |
| PIB-269 | I | the `recovery-pending` message | names the mutating re-run and `--abandon-transaction`; never claims the plan would be unchanged |
| PIB-270 | I | `--abandon-transaction --yes` on a divergent slug | whole `.tpatch/features/<slug>/` subtree byte-identical afterwards, including `status.json` and every blob |
| PIB-271 | I | the same run | journal, both preimage files and every `stage-*` tree now live under one `abandoned-<12hex>/`; nothing was deleted |
| PIB-272 | I | the abandon report | names the evidence directory, the `rm -rf` form, and states that no canonical file changed |
| PIB-273 | C | `--abandon-transaction` without `--yes`, filesystem spy | zero writes, zero renames, zero removals; exit 0; `outcome: abandon-planned` |
| PIB-274 | I | mutating `prepare` after a successful abandon | no `recovery-pending`, no exit 6; the run proceeds on artifact evidence and reaches exit 0 on an admissible tree |

### 18.27 Z — Undo CAS and the metadata entries

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-275 | C | a third party rewrites a published entry, then rollback reaches it | the undo refuses that entry; exit 6; the third party's bytes are byte-identical afterwards |
| PIB-276 | C | a third party removes a published `create` entry before its undo | the undo refuses (identity is absence, not the new image); exit 6; nothing is recreated |
| PIB-277 | U | the journal of a `--regenerate` run | `archive_index` and `status` appear as full entries with `preimage`, `new_image` and `action`, exactly like artifact entries |
| PIB-278 | C | `status.json` modified by another process between its publication and a rollback | the undo refuses; exit 6; the other process's `status.json` survives |

### 18.28 AA — Lock authority, lane gate and platform envelope

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-279 | I | G1-established Git workspace, `.tpatch/local/` not covered by an ignore rule | exit 3, `local-lane-not-ignored`, before any staging byte and before external cache locator creation |
| PIB-280 | I | Git-backed workspace with a tracked file under `.tpatch/local/` | exit 3, `local-lane-not-ignored`; zero mutation |
| PIB-281 | C | the lane gate, git-exec spy | the observed argv set is exactly {G1, G2, G3} for a non-`--regenerate` run |
| PIB-282 | I | G1 plus both lane-gate halves pass | the external cache locator is created only afterwards |
| PIB-283 | S | the lane-gate call site | it runs before the first `.tpatch/local/` write in every mutating mode, including `--manual` and `--abandon-transaction` |
| PIB-284 | I | a persistent opaque external cache lock file from a killed process | next run reopens/locks the same cache file; its presence causes no stale-lock refusal |
| PIB-285 | S | the external lock implementation | opaque cache file is zero-byte; no workspace lock path, nonce, PID, hostname, raw root or slug is persisted |
| PIB-286 | G | every `tpatch feature resource` subcommand after the lock extraction | stdout, stderr, exit code and written bytes byte-identical to pre-change goldens, including the contention refusal text |
| PIB-287 | G | `rescap`'s unsupported-platform refusal after the extraction | byte-identical string and code; the `linux \|\| darwin` envelope is unchanged for resource capture |
| PIB-288 | U | mutating `prepare` on BSD or another unsupported target | exit 3, `prepare-unsupported-platform`, before workspace/cache root open and before any write |
| PIB-289 | U | `prepare --check` on the same BSD target | unchanged accepted behavior; ADR-034's `unix \|\| windows` allowlist still governs the read half |
| PIB-290 | I | two live mutating processes, same root/slug/effective user | the second contends on the external cache authority; the first completes normally |
| PIB-392 | U | Linux/Darwin user-cache authority filesystem outside the shipped allowlist | exit 3, `lock-filesystem-unsupported`, before cache locator creation; `--check` unaffected |
| PIB-393 | G | docs/help and Windows authority guard | document local-fixed-cache-volume boundary; sensitivity fixture removing it fails |
| PIB-394 | G | refusal/platform catalog | Linux/Darwin/Windows mutation vs BSD refusal is explicit; `prepare-unsupported-platform` and `lock-filesystem-unsupported` stay distinct and absent from `--check` |

### 18.29 AB — Crash liveness and per-entry CAS

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-291 | I | first process `SIGKILL`ed mid-transaction; second process runs | the second acquires the lock (the kernel released it), recovers, and reaches a terminal outcome — never a permanent refusal |
| PIB-292 | I | `--manual` process `SIGKILL`ed after the lock, before the rename | the next `--manual` succeeds; no journal existed and none was needed |
| PIB-293 | I | crash at CP8 (journal cleared, lock not released) | the next run proceeds to exit 0 |
| PIB-294 | I | ten sequential kill-and-rerun cycles on one slug | every cycle terminates in a named outcome; the slug is never permanently blocked; the final run publishes |
| PIB-295 | G | the CP table of §7.10 | every row's recovery outcome is reachable by a named injection fixture, and no row's outcome is "refuses forever" |
| PIB-296 | C | an editor writes `exploration.md` after set-level revalidation but before that entry's CAS | exit 5; entries already published are rolled back; the editor's bytes survive |
| PIB-297 | C | an editor writes entry 3 after entry 1 was renamed | rollback restores entry 1 to its preimage and refuses nothing; the editor's entry-3 bytes are untouched |

### 18.30 AC — Journal strict binding and identity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-298 | U | journal with trailing content after the JSON value (J1) | exit 6, `journal-corrupt`; nothing restored |
| PIB-299 | U | journal with an unknown field at any nesting level (J2) | exit 6, `journal-corrupt` |
| PIB-300 | U | journal with a different `version` (J3) | exit 6, `journal-version-mismatch`; the message says the journal was written by another build |
| PIB-301 | U | journal whose `slug` names a different feature (J4) | exit 6, `journal-foreign`; the other slug's lane untouched |
| PIB-302 | U | journal with a `rel` outside the feature directory (J7) | exit 6, `journal-path-escape`; no file outside the feature directory is read or written |
| PIB-303 | C | journal whose entry preimage was edited to point at attacker-chosen content (J8) | exit 6, `journal-forged`; the canonical file is byte-identical afterwards |
| PIB-304 | U | journal with a duplicate `artifact_id` or a `null` `entries` array (J9, J10) | exit 6, `journal-corrupt` |
| PIB-305 | G | the J1–J10 bind list | every bind is reachable by a named fixture; a sensitivity fixture removing one bind makes the guard fail |
| PIB-306 | U | identity comparison | equality requires all four of exists/sha256/size/mode; a same-size, same-mode, different-content file compares unequal |
| PIB-307 | C | a journal that passes J1–J10 but whose entry no longer matches any expected image | no write occurs; exit 6, `recovery-divergent` |

### 18.31 AD — Rooted publication writes and final verification

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-308 | S | the publication path | no `os.Rename`, `os.CreateTemp`, `os.WriteFile`, `os.Remove`, `os.MkdirAll` or `filepath.Join(repoRoot, …)`; every write goes through a `*os.Root` method |
| PIB-309 | G | sensitivity fixture for PIB-308 | inserting one `os.Rename` call into a fixture package makes the scan fail |
| PIB-310 | C | ancestor redirect attempts an escape outside the held root | rooted write refuses; nothing is written outside the root; this row makes no in-root-redirection claim |
| PIB-311 | S | the publication path | `Root.Chmod`, `Root.Chown` and `Root.Chtimes` are never called |
| PIB-312 | G | `internal/store/store.go` | `writeFileAtomicWithRename` is unchanged and every shipped caller still reaches it; no `prepare` caller is added |
| PIB-313 | S | every mutating `prepare` write path, including manual | it never calls `writeFileAtomic`/`writeFileAtomicWithRename`/`SaveFeatureStatus` |
| PIB-314 | C | an external write lands between the last rename and the final verification | exit 6, `post-publication-divergence`; **nothing is undone**; the external bytes survive; the report names the entry |
| PIB-315 | I | an ordinary successful run | the final verification ran, and its result is what the exit-0 report asserts |

### 18.32 AE — Orphans, late crash phases, journal-loss boundary and concurrency

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-316 | C | a rolled-back `--regenerate` that had written two blobs | the exit-5 report lists both blob hashes and the exact purge command; the message does not claim the tree is byte-identical |
| PIB-317 | C | the same run | the number of listed orphans equals the number of blob files created by the attempt |
| PIB-318 | C | crash phase CP10 (`--manual`, during the single rename) | the next run finds no journal; `status.json` is old or new, never partial; exit 0 |
| PIB-319 | C | crash phase CP11 (retry after a blob write) | the existing blob is reused, not rewritten (inode and mtime unchanged); advisory `archive-blob-reused` |
| PIB-320 | C | recovery with an `abandoned-*` directory present | the directory is untouched; recovery removes only journal, preimages and `stage-*` |
| PIB-321 | I | `git clean -xfd` between a crash and the next run | next run has no journal and follows ordinary preflight; doctor does not claim it detected journal loss |
| PIB-322 | I | fresh clone after an interrupted committed archive state | no journal, no refusal; ordinary preflight; present blobs are not proof of journal loss |
| PIB-323 | G | doctor/docs disclosure | both state that a removed journal is unrecoverable **and ordinarily undetectable**; sensitivity fixture fails on a detection claim |
| PIB-324 | I | an external write to a published artifact after exit 0 | no tpatch postcondition is violated; the next `--check` reports the external content truthfully |
| PIB-325 | I | a concurrent `tpatch cycle` publishing `spec.md` mid-window | detected at the corresponding CAS; exit 5; `cycle`'s bytes survive |

### 18.33 AF — Git usage and per-mode archive semantics

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-326 | G | sensitivity fixture for the git-exec spy (PIB-107) | adding a `git status` call to a fixture path makes the spy assertion fail |
| PIB-327 | I | G1 cleanly establishes a non-worktree | exit 0; advisory `workspace-not-git`; no G2–G4 invocation |
| PIB-328 | I | the same G1-established non-worktree under `--regenerate` | `bundle-untracked-in-git` is **not** emitted; `workspace-not-git` is |
| PIB-329 | I | G1 executable/semantic failure, including missing `git` | exit 3, `local-lane-unverifiable`; zero mutation; no `.git` existence heuristic is used |
| PIB-330 | I | `--regenerate` on a feature with all three artifacts absent | zero blobs, no index entry, no `intent-archive/` directory; exit 0 |

### 18.34 AG — Archive index strict decoding and tracking claims

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-331 | U | `index.json` with trailing content (X1) | exit 3, `archive-index-corrupt`; nothing written |
| PIB-332 | U | `index.json` with an unknown field (X2) | exit 3, `archive-index-corrupt` |
| PIB-333 | U | `index.json` with a higher `schema_version` (X3) | exit 3, `archive-index-version-unsupported`; the message says to upgrade tpatch and never says the file is broken |
| PIB-334 | U | `index.json` whose `feature` names another slug (X4) | exit 3, `archive-index-foreign` |
| PIB-335 | U | `index.json` with `generations: null` or `replaced: null` (X5) | exit 3, `archive-index-corrupt` |
| PIB-336 | U | duplicate or malformed `generation_id` (X6) | exit 3, `archive-index-corrupt` |
| PIB-337 | U | `blob: ""` with `purged: false`, and `blob` set with `purged: true` (X7) | both refuse with `archive-index-corrupt` |
| PIB-338 | U | a `path` escaping the feature directory (X8) | exit 3, `archive-index-path-escape`; no path outside the feature directory is touched |
| PIB-339 | U | an unknown `artifact_id` (X9) | exit 3, `archive-index-corrupt` |
| PIB-340 | U | a retained or tombstoned `generation_id` that does not reproduce from its immutable body (X10) | exit 3, `archive-index-generation-mismatch` |
| PIB-341 | G | sensitivity fixture for the strict decoder | a valid index plus one injected unknown key must fail; the guard is not satisfied by "decoding succeeded" |
| PIB-342 | G | shipped strings and docs | no sentence claims clone-durable recovery for an uncommitted archive; `bundle-untracked-in-git`'s text states the condition |
| PIB-343 | I | untracked `.tpatch/`, `--regenerate` | exit 0; blobs exist on disk; the report states the archive is not clone-durable until committed |
| PIB-344 | G | the human report and `docs/feature-layout.md` | both disclose that `land` sweeps `intent-archive/**` into the operator's commit |

### 18.35 AH — The retention surface

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-345 | I | `feature intent-archive list <slug>` on a populated archive | every generation, entry, blob hash, size, presence flag and the `orphans:` section; exit 0 |
| PIB-346 | C | the same command, filesystem spy | zero writes of any kind |
| PIB-347 | I | `purge <slug>` with no scope selector | exit 1; the message names `--blob`, `--generation`, `--orphans`, `--all`; zero writes |
| PIB-348 | C | `purge <slug> --all` without `--yes`, filesystem spy | zero writes; the preview lists exactly what `--yes` would remove |
| PIB-349 | I | `purge` while a live prepare holds the external cache authority | exit 3, `transaction-in-progress`; zero writes |
| PIB-350 | I | `purge` with a pending journal for the slug | exit 3, `recovery-pending`; zero writes; the message names the two recovery routes |
| PIB-351 | I | `purge --blob <h> --yes` run twice | the second is a no-op success; the index is byte-identical after the second run |
| PIB-352 | C | purge crash after rooted index rename and before blob removal | index tombstoned; blob is orphan; later `purge --orphans` collects it; no state claims bytes it lacks |
| PIB-353 | C | the same purge, crash injected before the index rewrite | nothing changed; the blob is still referenced and present |
| PIB-354 | I | `purge --generation <id> --yes` | every entry is tombstoned with `content_sha256` retained; generation id unchanged; strict decode passes |
| PIB-355 | I | `purge --all --yes` | every entry tombstoned; `blobs/` empty; `index.json` still present and still decodes |
| PIB-356 | G | `list`, `doctor` D9 and the purge report | all three carry the same "purged entries … not recoverable by tpatch" sentence |
| PIB-357 | I | `purge --blob <h> --yes` where `<h>` is referenced by a non-selected generation | that blob refuses with `archive-blob-referenced` naming the referencing generations; other selected blobs still purge |
| PIB-358 | I | `purge --all --yes` on the same archive | no `archive-blob-referenced` occurs; everything is purged |
| PIB-359 | I | `purge --orphans --yes` | exactly the unreferenced blob files are removed; `index.json` is byte-identical afterwards |
| PIB-360 | I | the accidental-secret remediation path (`list` then `purge --blob`) | the blob file is gone; the index is tombstoned; exit 0 |
| PIB-361 | G | the purge report and `docs/feature-layout.md` | both state that a **committed** blob remains in Git history and that removing it from history is not something tpatch performs |

### 18.36 AI — Exit-6 escape, precedence and report shapes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-362 | G | every exit-6 population | the message names the divergent entry, the file holding its prior bytes, and the `--abandon-transaction` command |
| PIB-363 | I | exit 6, then `--abandon-transaction --yes`, then a mutating re-run | the re-run reaches exit 0 on an admissible tree; no population of exit 6 is permanent |
| PIB-364 | C | recovery with a live sibling process | the lock is acquired before recovery is attempted; recovery never runs while another process holds the lock |
| PIB-365 | C | `--regenerate` with no provider, provider and filesystem spies | zero provider calls, zero staged files, zero blobs; exit 3 `provider-required-for-regenerate` |
| PIB-366 | I | `--abandon-transaction --json` | `mode: "abandon"`, `artifacts: []`, `archive` absent, `abandoned` object present with the evidence directory |
| PIB-367 | I | `--quiet` across all five modes and all outcomes | every line matches `prepare <slug>: <mode> <outcome>[ <detail>]`; exactly one line |

### 18.37 AJ — Provider authority, deadlines and leakage

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-368 | I | `--regenerate` with no provider configured | exit 3, `provider-required-for-regenerate`; remediation names `tpatch provider set`, `tpatch provider check` and `--allow-heuristic`; the bundle is byte-identical |
| PIB-369 | I | `--regenerate` with a provider that fails every attempt | exit 5, `regenerate-generation-failed`; the bundle byte-identical; staging retained; zero blobs |
| PIB-370 | C | `--regenerate`, per-phase deadline expires on the second artifact | exit 5; the message names which deadline expired; zero canonical mutations |
| PIB-371 | C | default mode, total deadline expires after the first artifact | the remaining artifacts are heuristic; advisories `provider-deadline-heuristic` **and** `provider-deadline-cascade`; exit 0 |
| PIB-372 | I | `--regenerate --allow-heuristic` with a failing provider | exit 0; advisory `regenerate-heuristic-allowed` plus the specific fallback advisory; prior bytes archived first |
| PIB-373 | G | the report of any run with ≥2 deadline-caused fallbacks | it states that one expiry caused them; no message presents them as independent provider failures |
| PIB-374 | I | `--allow-heuristic --help` text | contains the verbatim opt-in sentence of §11.3.2 |
| PIB-375 | S | config loading | no config key, env var or file setting can enable `--allow-heuristic` behavior; the flag is the only route |
| PIB-376 | G | the tracked tree after any mutating `prepare` | no `raw-*` file exists anywhere under `.tpatch/features/` |
| PIB-377 | G | both report surfaces | no duration, deadline value or elapsed-time field appears in any output |

### 18.38 AK — Notes semantics, doctor probe and residue reporting

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-378 | G | every shipped string, report field and doc sentence | none presents `status.json.notes` or the archive as evidence of who authored an artifact |
| PIB-379 | I | a `--regenerate` note read after a later `define` transition | the note is the later transition's, proving `notes` is last-transition state; no consumer treats it as history |
| PIB-380 | C | `doctor` with a live `prepare` running, filesystem spy | the external cache lock locator is never created by `doctor`; the probe holds nothing after it returns; D9 reports "running now" |
| PIB-381 | C | `doctor` with no external cache lock file present | no external cache lock file is created; D9 reports no live transaction |
| PIB-382 | I | `doctor` with retained `stage-*` trees | D9 names each path and the fact that the next successful run removes them |
| PIB-383 | I | `doctor` with an `abandoned-*` directory | D9 names it and the `rm -rf` form |
| PIB-384 | I | `doctor` with orphan blobs | D9 names the count and the exact `purge --orphans` command |
| PIB-385 | I | `doctor` with a corrupt `index.json` | D9 names the failing bind code and suggests only `list`; no repair is offered |
| PIB-386 | I | doctor with an inert persistent external cache lock locator and nothing else | D9 reports it informationally; no removal suggestion |
| PIB-387 | G | the §12.5 residue table | total over the observable lane and archive state; a synthetic tenth residue fails the guard rather than being ignored |

### 18.39 AL — Guards, skills and golden provenance

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-388 | G | sensitivity fixture for the redaction-message rule | a fixture message containing a matched substring makes the guard fail |
| PIB-389 | G | all six skill files | none presents the archive as a general history or undo facility |
| PIB-390 | G | all six skill files | each states that `--regenerate` requires a provider unless `--allow-heuristic` is passed |
| PIB-391 | G | the `--check` golden fixtures used by PIB-198 … PIB-207 | their provenance is the accepted `--check` implementation's commit range, not this cluster's; a fixture regenerated by this cluster's binary fails the guard |

### 18.40 AM — Rev-2 adjudication: authority, identity and evidence limits

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-395 | C | plant the old ignored lane lock decoy; holder runs `git clean -xfd`, test recreates the decoy, then runs `checkout`/`stash` and a same-slug contender | cleanup can unlink/recreate the decoy but not the external cache inode; the contender receives `transaction-in-progress`, never acquires the recreated lane inode |
| PIB-396 | I | holder process is killed while holding the external cache authority | the next same-root/same-slug process acquires the persistent cache file and reaches recovery; the old file remains inert |
| PIB-397 | I | two live real processes, same effective user/root/slug | exactly one owns the opaque cache authority; the other exits 3 and no worktree lock file exists |
| PIB-398 | G | external locator construction | symlink aliases of one native root produce one opaque key; distinct worktree/submodule roots differ; raw root paths/slugs never appear in the cache pathname/content; unsafe app-owned cache components fail closed |
| PIB-399 | C | ready `--manual` publication with rooted-operation spy | a same-directory rooted temp is fsynced then renamed to `status.json`; no `writeFileAtomic`, `SaveFeatureStatus`, or path-based writer is called; no journal/archive is created |
| PIB-400 | C | external `status.json` edit after manual preimage capture and before rename | exit 5 `status-changed`; the external status bytes and `FEATURES.md` are byte-identical; no rename occurs |
| PIB-401 | U | tombstoned index entry with immutable `content_sha256` | X10 recomputes the recorded generation id after purge; missing/mismatched digest, blob/purged inconsistency, or altered immutable body refuses |
| PIB-402 | I | later regeneration computes an existing fully tombstoned generation | redaction passes; blobs are written/reused before a CAS index rewrite; matching entries rehydrate, no duplicate generation is appended, and no orphan remains |
| PIB-403 | I | repeated purge→rehydrate with a blob shared by two generations | ids/order remain stable; rehydrating one reference does not revive the other; a later purge respects all current non-tombstoned references |
| PIB-404 | C | rehydration redaction and crash-order subtests | a redaction match leaves tombstones/no blob; a crash before index rename leaves at most an orphan, while no index ever references a missing blob |
| PIB-405 | C | concurrent index edit after purge preimage capture and immediately before rooted rename | strict re-read/CAS refuses `archive-index-changed` before **any** selected blob removal; live edited index and blobs survive |
| PIB-406 | C | ancestor becomes a relative symlink to another directory inside the held root | outside-root escape remains refused; an identity-changing in-root redirect is caught by CAS; fixture/docs do not claim detection of an equal-identity alias |
| PIB-407 | I | ordinary partial bundle made by `analyze`/`define`, with completely clean lane/archive | D9 emits neither residue nor journal-loss diagnosis; no recovery/repair is suggested |
| PIB-408 | I | linked worktree and submodule/nested workspace with no `.git` at the `.tpatch` root | G1 from the workspace root establishes `true`, runs the Git gate; Git exec failure remains `local-lane-unverifiable`, while Git-established non-worktree proceeds |
| PIB-409 | G | mutating platform/build matrix | mutation is admitted only on Linux, Darwin, Windows with required native Windows authority tests; BSD and all other targets refuse; accepted read-only `unix \|\| windows` behavior is unchanged |

### 18.41 Counts, kinds and slice partition

- **409 rows**, `PIB-001`…`PIB-409`, contiguous, zero duplicates, zero retired.
  160 rows are new in rev-1 (`PIB-235`…`PIB-394`); **15** are new in rev-2
  (`PIB-395`…`PIB-409`); rev-0 rows retain their IDs when amended (§18.1).
- **39 categories**: A 20, B 24, C 15, D 12, E 9, F 19, G 13, H 14, I 13, J 8,
  K 12, L 10, M 14, N 14, O 10, P 7, Q 6, R 3, S 9, T 2, U 10, V 12, W 5, X 6,
  Y 7, Z 4, AA 15, AB 7, AC 10, AD 8, AE 10, AF 5, AG 14, AH 17, AI 6, AJ 10,
  AK 10, AL 4, AM 15. Sum = 409.
- **By kind**: `I` 185, `C` 85, `G` 74, `U` 44, `S` 21. Sum = 409. Every row
  whose kind is `G` is covered by §18.42's sensitivity requirement.
- **Slice partition** (each row in exactly one slice):

| Slice | Categories | Rows |
|---|---|---|
| S1 transaction core | F, G, H, Z, AB, AC, AD | 75 |
| S1b lock extraction | AA | 15 |
| S2 generator extraction | N, AJ | 24 |
| S3 archive | D, L, X, AG | 42 |
| S4 CLI wiring | A, B, C, E, I, M, T, U, V, W, Y, AF, AI | 142 |
| S4b retention surface | AH | 17 |
| S5 doctor, compatibility, non-invalidation | J, O, P, R, AE, AK | 48 |
| S6 docs, skills, guards | K, Q, S, AL | 31 |
| S7 rev-2 cross-cutting hardening | AM | 15 |

Sum = 409; zero unassigned, zero double-assigned (PIB-230's ledger check
derives this partition mechanically rather than trusting the table).

### 18.42 Sensitivity requirement

Every row whose Kind contains `G` carries a **sensitivity fixture**: a
deliberately wrong input that the guard must reject. A byte-scanning or
name-matching guard can false-pass silently, and the repository has already
been burned by exactly that
(`docs/prds/PRD-artifact-validation-and-provenance.md:3960-3991`). PIB-231 is
the meta-check that derives the guard set mechanically from the Kind column
rather than from a hand-maintained list.

**Semantic guards additionally require a semantic fixture.** Six guards in this
matrix assert a *meaning*, not a token, and a byte scan cannot establish them.
Each therefore carries a fixture that is textually valid and semantically
wrong, and the guard must still fail:

| Guard | Semantic fixture that must fail |
|---|---|
| PIB-155 over-claim | a sentence that says "the six files appear together to any reader" without using the word "atomic" |
| PIB-244 flag-delta completeness | a delta table that lists every flag but describes one flag's *before* behavior incorrectly |
| PIB-250 coherence totality | a table covering all eight combinations where one outcome contradicts §6.1.2 |
| PIB-267 redaction non-override | a help string offering "skip the scan for trusted repositories" without the token `--force` |
| PIB-342 clone-durability | a sentence claiming durable recovery that never uses the word "clone" |
| PIB-391 golden provenance | a golden file with the right bytes committed by the wrong commit range |

PIB-231 derives the guard set mechanically; this table is what stops six of
those guards from being satisfied by a spelling check (PIB-231's own
sensitivity fixture covers the derivation).

## 19. Implementation authorization gate

**No implementation is authorized by this document.**

1. This PRD must be accepted.
2. **ADR-035 must be accepted.** It is `Proposed` at rev-1, and a writer cannot
   accept its own ADR; the two are reviewed together and acceptance of both is
   the precondition for dispatching any slice.
3. **The accepted `prepare --check` PRD must be implemented, landed on
   `origin/main`, and passing its own acceptance matrix** — §17.1. Its goldens
   must come from that implementation's commit range, not from this cluster
   (PIB-391).
4. Only then may the cluster lead declare the §17.2 file partition and
   dispatch, beginning with S1b's pre-change `feature resource` goldens.

Until (1), (2) and (3) all hold, no file under `cmd/`, `internal/`, `assets/`,
`tests/`, and no change to `SPEC.md` or `CHANGELOG.md`, is authorized by this
PRD.

## 20. Open questions

Only genuinely unresolved items. Everything else is a decision. None blocks
review.

| # | Question | Why open | Default if unanswered |
|---|---|---|---|
| Q1 | Should `--regenerate --only <ids>` exist later? | Coherence argues for all-or-nothing (§6.3), but an operator who only wants a new `exploration.md` currently has no route short of deleting the file. | Not in v1; additive later with an enumerated coherence statement. |
| Q2 | Is `180s` the right whole-command deadline? | No distribution of three-call prepare durations has been measured; the per-phase `60s` (`internal/cli/cobra.go:629`) was not chosen empirically either. | `180s`, overridable; changing it moves only the fallback boundary. |
| Q3 | Should a `--require-provider` flag exist for the **default** mode? | `--regenerate` already requires one (§11.3.2), and default-mode fallback only ever creates absent files — but an operator who wants provider-or-nothing everywhere has no switch. | No flag in v1; the advisories name every fallback. |
| Q4 | Should a `tpatch prepare <slug> --restore <hash>` verb exist? | `cp` is unambiguous and needs no new surface, but it is unguarded — nothing stops restoring a blob over the wrong path. | No verb in v1; the report prints the exact `cp` form. |
| Q5 | Should purge gain an **age- or count-based** retention policy? | §9.7 ships explicit, operator-driven purging; an automatic policy would need a clock, which every determinism rule in this cluster forbids in tracked artifacts. | Explicit purge only; any automatic policy needs its own determinism argument. |
| Q6 | Should `--regenerate` invalidate a `verified-fresh` label? | Intent changed, so the freshness claim is arguably stale — but ADR-013 makes `verify`/`amend` the only writers of that record, and `define` has the same effect today without invalidating. | No change; widening `verify`'s writer set is a separate PRD with its own delta. |
| Q7 | Should the archive be per-feature (chosen) or per-workspace with cross-feature dedupe? | A workspace-level blob store would dedupe across features, but it couples feature directories, complicates `land`'s per-feature path set (`internal/cli/land.go:723-725`) and makes `tpatch remove` ambiguous. | Per-feature, as specified. |
| Q8 | Should `doctor` D9 be an error rather than a warning? | A pending journal is recoverable automatically, so failing `doctor` would over-alarm — but a journal that persists for weeks indicates an abandoned transaction. | Warning; a future PRD may add an age-based escalation, which would require a clock and therefore its own determinism argument. |
| Q9 | Should the redaction gate ever be overridable? | A false positive on `home-absolute-path` or the broad `email-pii` class inside legitimate Markdown blocks a regeneration, and the operator's only route is to edit the file (§9.6.1, R12). A scoped override (one class, one artifact, one invocation) is conceivable. | No override in v1. An override flag on a hard gate is how hard gates become suggestions. |
| Q10 | Is `90s` the right per-phase deadline? | Chosen as 1.5× the shipped single-phase `60s` (`internal/cli/cobra.go:629`) because a prepare-time call carries more context; no distribution has been measured. | `90s`, clamped to the remaining total. |
| Q11 | Should the retention surface be `feature intent-archive` (chosen) or a top-level `tpatch archive`? | The `feature` group already holds per-feature management verbs (`internal/cli/feature_deps.go:43-53`), but `intent-archive` is a long path to type. | `feature intent-archive`, consistent with `feature resource`. |
| Q12 | Should mutating `prepare` support BSD, `solaris` or `aix` through additional lock/statfs implementations? | The shipped primitive and filesystem preflight exist only on Linux/Darwin; admitting BSD without its own implementation and native CI would be an unsupported promise. | Refuse (§7.4.2, §12.6 D8). A future target needs its own primitive, cache-filesystem proof and native CI. |
| Q13 | Should a non-allowlisted **cache authority** filesystem be able to opt in? | Fail-closed is right for the external lock namespace; a workspace filesystem is no longer the lock authority. | Refuse. A relaxation needs evidence that the cache filesystem honours the selected primitive, not a flag. |

## 21. Alternatives considered (summary)

Beyond §8's six overwrite alternatives:

| Option | Verdict |
|---|---|
| A new `prepared` FeatureState | **Rejected.** WP-005 Agreed item 6 forbids it, and completeness is an artifact-level fact that `prepare --check` already reports. |
| A separate verb (`tpatch bundle`, `tpatch intent`) instead of extending `prepare` | **Rejected.** It would fork the vocabulary the accepted PRD, WP-005 and GH #10/#11 already share, and leave `prepare --check` orphaned from its own mutating half. |
| Redo-style recovery (complete the publication like `land`) | **Rejected.** Nothing in the publication is irreversible, so undo is strictly simpler and cannot publish from a pruned staging tree (§7.5, ADR-035 D5). |
| Taking the transaction lock in every command that writes a feature artifact | **Rejected.** Eleven shipped verbs would gain a lock, a deadlock surface and a new failure mode, to close a window that revalidation and per-entry CAS already detect (§7.4.4, §7.6.3). |
| Copy-on-write of the whole feature directory, then swap | **Rejected.** Directory swap is not atomic either, breaks any open descriptor, and would move audit-trail files (`patches/**`, `record.md`) that this command has no business touching. |
| Writing the archive to `.tpatch/local/` | **Rejected.** A recovery guarantee that vanishes on clone is not a guarantee (§9.1, ADR-035 D8). |
| Storing preimage **content** in the journal instead of the archive | **Rejected** for artifacts (unbounded gitignored growth, lost on clone); **adopted** for the two small machine-written metadata preimages, where the archive would be circular or category-wrong (§7.6). |
| `O_EXCL` lock file plus nonce/inode ownership (rev-0's model, and `land`'s) | **Rejected.** A file's existence is data, not liveness; CP1/CP2/CP8 and every `--manual` crash leave a lock with no journal to match against, which permanently refuses the slug (§7.4.1). Replaced by the kernel process-lifetime lock (§7.4.2). |
| Keeping the journal under the Git directory so `git clean` cannot remove it | **Rejected.** It does not survive a fresh clone either, `.git` is Git's namespace, worktree/submodule indirection complicates it, a non-Git workspace still needs a second location, and it would make the journal a hard Git dependency (§7.11.1). |
| Letting `--dry-run` simulate the post-recovery plan | **Rejected.** Deciding recovery's outcome without holding the lock and without performing it is exactly the guess §7.6.3 forbids; `--dry-run` refuses with `recovery-pending` instead (§6.4). |
| Keeping heuristic fallback for `--regenerate` (rev-0's behavior) | **Rejected.** One expired context could replace three hand-authored documents with boilerplate and exit 0. Recoverable is not the same as acceptable; the opt-in is `--allow-heuristic` (§11.3.2). |
| Archiving prior bytes without a redaction scan | **Rejected.** ADR-027 D3 makes redaction a precondition for every tpatch-owned writer, and a tracked archive is the worst possible place to silently retain a credential (§9.6.1). |
| Scrubbing secret-shaped content instead of refusing | **Rejected.** A scrubbed blob is not the bytes that were replaced, so the recovery guarantee would silently become false (§9.6.1). |
| An immutable archive with no removal route (rev-0's "normal and permanent") | **Rejected.** Immutable must mean "never modified in place", not "undeletable forever"; without §9.7 the design is a retention leak with a nice hash function. |
| Path-based `os.Rename` after rooted checks (rev-0's publication) | **Rejected.** `(*os.Root).Rename` prevents resolution outside the held root; it does not prove an ancestor did not redirect inside that root, which §7.7.2 discloses. |
| Bringing `FEATURES.md` into the publication set | **Rejected.** CAS on a derived index that eleven unlocked commands rewrite would turn ordinary concurrency into exit-5 aborts (§12.3.1). |

## 22. Claims-audit appendix

Every load-bearing claim this PRD makes about **current** behavior, with a
`file:line` anchor. A reviewer should spot-check that each anchor lands within
±5 lines of the cited construct at HEAD `2e2a67d`. Claims corrected in rev-1
are marked †; claims added in rev-1 start at C143.

| # | Claim | Anchor |
|---|---|---|
| C1 | `RunAnalysis` writes the sidecar `artifacts/analysis.json` before anything else | `internal/workflow/workflow.go:90` |
| C2 | `RunAnalysis` then writes `analysis.md` | `internal/workflow/workflow.go:96` |
| C3 | `RunAnalysis` then mutates feature state to `analyzed` | `internal/workflow/workflow.go:103` |
| C4 | `RunDefine` writes `spec.md` | `internal/workflow/workflow.go:151` |
| C5 | `RunDefine` marks `defined` with a fixed notes string | `internal/workflow/workflow.go:155` |
| C6 | `RunExplore` writes `exploration.md` | `internal/workflow/workflow.go:196` |
| C7 | `RunExplore` also marks `defined` | `internal/workflow/workflow.go:200` |
| C8 | `RunAnalysis` writes a raw provider response into `artifacts/` on the fallback path | `internal/workflow/workflow.go:72` |
| C9 | …and again on the parse-failure path | `internal/workflow/workflow.go:80` |
| C10 | `RunAnalysis` falls back to the heuristic generator on provider error | `internal/workflow/workflow.go:67-69` |
| C11 | `RunDefine` falls back to the heuristic generator | `internal/workflow/workflow.go:141-143` |
| C12 | `RunExplore` falls back to the heuristic generator | `internal/workflow/workflow.go:186-188` |
| C13 | Heuristic mode is recorded in the sidecar as `heuristic_mode` | `internal/workflow/workflow.go:25` |
| C14 | …and set by the heuristic analysis constructor | `internal/workflow/workflow.go:208` |
| C15 | `RunDefine` reads `analysis.json` and `analysis.md` as context | `internal/workflow/workflow.go:117-123` |
| C16 | `RunExplore` reads `analysis.md` and `spec.md` as context | `internal/workflow/workflow.go:165-166` |
| C17 | Analysis requests are capped at 4096 output tokens | `internal/workflow/workflow.go:58` |
| C18 | Define requests are capped at 4096 output tokens | `internal/workflow/workflow.go:132` |
| C19 | Explore requests are capped at 4096 output tokens | `internal/workflow/workflow.go:177` |
| C20 | The analysis validator requires a JSON **object** | `internal/workflow/workflow.go:62` |
| C21 | `captureFileTree` is a pure read used for generation context | `internal/workflow/workflow.go:262-266` |
| C22 | `readGuidanceFiles` is a pure read used for generation context | `internal/workflow/workflow.go:293-304` |
| C23 | `renderAnalysisMD` is already pure | `internal/workflow/workflow.go:351-401` |
| C24 | `GenerateWithRetry` writes `raw-<prefix>-response-<n>.txt` when a `Store` is supplied | `internal/workflow/retry.go:105-109` |
| C25 | `GenerateWithRetry` retries with a corrective prompt and returns the last response | `internal/workflow/retry.go:94-131` |
| C26 | `WithDisableRetry` forces a single attempt | `internal/workflow/retry.go:47-58` |
| C27 | …consumed as `attempts = 1` | `internal/workflow/retry.go:83-85` |
| C28 | `--manual` validates only that the artifact exists (JSON validation is implement-only) | `internal/store/manual.go:56-66` |
| C29 | The `--manual` phase map is the single source of truth for four phases | `internal/store/manual.go:26-32` |
| C30 | `AdvanceStateManually` writes a free-text note and marks state | `internal/store/manual.go:78-80` |
| C31 | `WriteFeatureFile` path-checks then calls `writeFile` | `internal/store/store.go:443-449` |
| C32 | `writeFile` is `os.WriteFile` — a truncating in-place write | `internal/store/store.go:918-923` |
| C33 | `writeFileAtomic` preserves the existing file's permission bits | `internal/store/store.go:871-876` |
| C34 | `writeFileAtomicWithRename` is temp→chmod→write→sync→close→rename→dir-sync | `internal/store/store.go:878-917` |
| C35 | The temp file is created in the destination directory | `internal/store/store.go:884-886` |
| C36 | `SaveFeatureStatus` writes `status.json` atomically and best-effort refreshes `FEATURES.md`, swallowing the refresh error because `status.json` is the source of truth | `internal/store/store.go:363-377` |
| C37 | `SaveFeatureStatus` stamps `updated_at` when empty | `internal/store/store.go:369-371` |
| C38 | `MarkFeatureState` overwrites `Notes` on every transition | `internal/store/store.go:380-393` |
| C39 | `MarkFeatureState` bumps `updated_at` | `internal/store/store.go:388` |
| C40 | Config stores an auth **env-var name**, not a secret | `internal/store/store.go:72` |
| C41 | `MkdirAllAndSyncChain` fsyncs the whole chain unconditionally | `internal/store/fsdurable.go:41-52` |
| C42 | …and the rationale is crash-durability of already-existing directories | `internal/store/fsdurable.go:1-10` |
| C43 | `SyncDir` opens and fsyncs a directory | `internal/store/fsdurable.go:22-33` |
| C44 | `RandomHex12` is the per-invocation scratch suffix convention | `internal/store/fsdurable.go:96-103` |
| C45 | The twelve `FeatureState` values | `internal/store/types.go:9-38` |
| C46 | `defined` is an existing state | `internal/store/types.go:11` |
| C47 | `ValidFeatureState` enumerates them in one switch | `internal/store/types.go:40-47` |
| C48 | `upstream_merged` asserts an upstream implementation exists | `internal/store/types.go:26-31` |
| C49 | `FeatureStatus` carries `state`, `last_command`, `updated_at`, `notes` | `internal/store/types.go:206-216` |
| C50 | `Verify` is written only by `verify` and `amend` | `internal/store/types.go:236-251` |
| C51 | `RejectableStates` is pre-implementation only | `internal/store/status.go:139-152` |
| C52 | A reopen restores `PriorState` | `internal/store/status.go:130-133` |
| C53 | The land journal exists because `git commit` is irreversible | `internal/cli/land_journal.go:11-23` |
| C54 | The land journal is versioned and refuses other versions | `internal/cli/land_journal.go:56-58` |
| C55 | It lives under the gitignored `.tpatch/local/` root | `internal/cli/land_journal.go:60-62` |
| C56 † | `landJournalFileState` is `(exists, sha256, mode)` with a `matches` comparison — it carries **no size**; rev-0 claimed it did, and §7.6.1 now states the size field as this design's deliberate extension | `internal/cli/land_journal.go:65-79` |
| C57 | Its `Phase` field is explicitly advisory | `internal/cli/land_journal.go:110-111` |
| C58 | The journal records a wall-clock `created_at` | `internal/cli/land_journal.go:109` |
| C59 | The journal is written through `gitutil.DurableWriteFile` | `internal/cli/land_journal.go:310-326` |
| C60 | The journal decoder is strict (unknown fields refused) | `internal/cli/land_journal.go:348-380` |
| C61 | Recovery classifies the live state as preimage / postimage / divergent | `internal/cli/land_journal.go:418-444` |
| C62 | Recovery runs before the caller mutates anything | `internal/cli/land_journal.go:445-482` |
| C63 † | `land`'s recovery lock is `O_CREATE\|O_EXCL` with a nonce, fsynced — cited in rev-1 only as the **rejected** alternative (§7.4.1), not as this design's lock | `internal/cli/land_journal.go:629-648` |
| C64 | Lock release removes the file and fsyncs the directory | `internal/cli/land_journal.go:650-662` |
| C65 † | `removeOwnedLandLock` removes a stale lock only when the nonce **and**, where the platform exposes one, the recorded inode match — the discipline rev-1 replaces with a kernel lock | `internal/cli/land_journal.go:675-698` |
| C66 | `DurableWriteFile` is temp→write→fsync→close→rename | `internal/gitutil/index_snapshot.go:455-500` |
| C67 | The resource capture tree is an unordered content-addressed set plus one pointer | `internal/store/resource_publish.go:1-9` |
| C68 † | Batch IDs are derived at exactly one point (a swappable deriver seam) | `internal/store/resource_publish.go:131-143` |
| C69 † | `ComputeBatchID` hashes the canonical body | `internal/store/resource_publish.go:155-167` |
| C70 | An already-identical batch file is an idempotent re-publish, never rewritten | `internal/store/resource_publish.go:240-246` |
| C71 † | Named publication refusals include collision and corruption | `internal/store/resource_publish.go:198-202` |
| C72 † | Publication is "write immutable content, then rewrite the pointer", and an existing file is compared **semantically** before being reused | `internal/store/resource_publish.go:230-285` |
| C73 | Batch decoding is strict: unknown fields, trailing content and null arrays refused | `internal/store/resource_publish.go:305-328` |
| C74 | No Go map may reach a wire format | `internal/store/canonjson.go:11-17` |
| C75 | The root printer emits `error: %v` for every non-nil `RunE` error | `internal/cli/cobra.go:33-39` |
| C76 | `exitCodeFor` maps typed errors to their code and everything else to 1 | `internal/cli/cobra.go:43-52` |
| C77 | The root sets `SilenceUsage` and `SilenceErrors` | `internal/cli/cobra.go:56-62` |
| C78 | `--path` is a root persistent string flag | `internal/cli/cobra.go:66` |
| C79 | `openStoreFromCmd` resolves `--path` then `FindProjectRoot` | `internal/cli/cobra.go:3782-3793` |
| C80 | The single-phase commands use the **probing** provider loader | `internal/cli/cobra.go:609-612` |
| C81 | They print progress to stdout | `internal/cli/cobra.go:621-626` |
| C82 | Their timeout default is 60s | `internal/cli/cobra.go:629` |
| C83 | `--no-retry` is a shipped phase flag | `internal/cli/cobra.go:630` |
| C84 | `--manual` and `--skip-llm` are installed by one shared helper | `internal/cli/cobra.go:3410-3414` |
| C85 | `runManualPhase` is the single manual entry point for four phases | `internal/cli/cobra.go:3429-3437` |
| C86 | `isTpatchUntracked` detects an untracked `.tpatch/` | `internal/cli/cobra.go:3405-3407` |
| C87 | `cycle` is a sequential pipeline through record | `internal/cli/phase2.go:26-32` |
| C88 | `cycle` calls the incremental phase functions in order | `internal/cli/phase2.go:62-96` |
| C89 | `cycle` asserts state after each phase | `internal/cli/phase2.go:69-71` |
| C90 | `cycle` uses the **non-probing** provider loader | `internal/cli/phase2.go:55` |
| C91 | `cycle --interactive` is the repo's only interactive prompt precedent | `internal/cli/phase2.go:50` |
| C92 | `cycle --skip-execute` stops before recipe execution | `internal/cli/phase2.go:122-126` |
| C93 | `next` emits a single harness task and supports `--format harness-json` | `internal/cli/phase2.go:381-384` |
| C94 | `next` infers the `defined` sub-state from `exploration.md` presence | `internal/cli/phase2.go:437-446` |
| C95 | `refuseIfUnappliedState` refuses with the shipped state-refusal code | `internal/cli/feature_unapply.go:464-473` |
| C96 | The repo binds exit 2 to validation and exit 3 to state refusal | `internal/cli/reject.go:45-48` |
| C97 | …surfaced through `ExitCodeError` | `internal/cli/reject.go:68-70` |
| C98 | `doctor` runs a registry of checks D1…D8 | `internal/workflow/doctor.go:226-237` |
| C99 | `land` sweeps everything dirty under the feature directory into its path set | `internal/cli/land.go:723-725` |
| C100 | The `.tpatch/local/` prefix is the gitignored scratch root | `internal/rescap/scratch.go:34-37` |
| C101 | A local-lane ignore contract gate runs before any scratch write | `internal/rescap/scratch.go:47-62` |
| C102 | Skill parity is enforced by a required-command list | `assets/assets_test.go:14-53` |
| C103 | …and by verbatim required anchors including phase ordering and preflight | `assets/assets_test.go:62-73` |
| C104 | `SPEC.md` makes exit codes per-command contracts | `SPEC.md:137` |
| C105 | …and binds exit 3 to state-transition refusal for the reject family | `SPEC.md:148` |
| C106 | The Path B guide teaches authoring all three Markdown files then three `--manual` runs | `docs/path-b-operator-guide.md:61-73` |
| C107 | The phase → artifact → state contract table | `docs/agent-as-provider.md:40-45` |
| C108 | `status.json.notes` is currently presented as distinguishing Path B from provider output | `docs/agent-as-provider.md:47-54` |
| C109 | The feature layout marks `post-apply.patch` canonical and `patches/**` audit-only | `docs/feature-layout.md:19,34` |
| C110 | The recipe is written by the implement phase | `docs/feature-layout.md:85` |
| C111 | Capture batches are an unordered content-addressed set, not a chronology | `docs/feature-layout.md:98-100` |
| C112 | An orphaned batch left by a crash is normal and permanent | `docs/feature-layout.md:103-106` |
| C113 | WP-005 Agreed item 3: downstream SDD is encouraged, never enforced | `docs/whitepapers/WP-005-spec-driven-workflows.md:56-58` |
| C114 | WP-005 Agreed item 6: no new lifecycle state; exploration must not become mandatory | `docs/whitepapers/WP-005-spec-driven-workflows.md:71-74` |
| C115 | WP-005 Agreed item 7: the all-or-nothing publication unit and the complete-prior-or-complete-new rule | `docs/whitepapers/WP-005-spec-driven-workflows.md:75-81` |
| C116 | WP-005 Turn 3: prepare cannot call the incremental writers and claim atomicity | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117` |
| C117 | WP-005 Turn 3: the required existing-primitives pre-flight | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:118-123` |
| C118 | The accepted PRD reserves plain `prepare` with exit 4 and a frozen refusal | `docs/prds/PRD-artifact-validation-and-provenance.md:356-382` |
| C119 | …and anticipates a future PRD registering the flags as an enumerated delta | `docs/prds/PRD-artifact-validation-and-provenance.md:396-403` |
| C120 | The accepted required set is the three Markdown artifacts | `docs/prds/PRD-artifact-validation-and-provenance.md:432-436` |
| C121 | The sidecar is optional and never affects readiness | `docs/prds/PRD-artifact-validation-and-provenance.md:437-441` |
| C122 | The closed nine-value state enum | `docs/prds/PRD-artifact-validation-and-provenance.md:1765-1773` |
| C123 | The frozen non-certification disclaimer | `docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798` |
| C124 | The accepted exit-code table (0/1/2/3/4) | `docs/prds/PRD-artifact-validation-and-provenance.md:1981-1987` |
| C125 | The accepted stream-routing contract | `docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419` |
| C126 | The accepted JSON schema and its no-map rule | `docs/prds/PRD-artifact-validation-and-provenance.md:2420-2576` |
| C127 | The accepted zero-mutation contract | `docs/prds/PRD-artifact-validation-and-provenance.md:2840-2866` |
| C128 | The forbidden provenance-inference sources | `docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920` |
| C129 | The provenance ADR trigger fires only on selection of a persistent **provenance** representation | `docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972` |
| C130 | ADR-034 D14 forbids citing ADR-034 as provenance precedent | `docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961` |
| C131 | The accepted decision that `--manual` gates stay loose | `docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015` |
| C132 | The accepted disqualifying assertion shapes | `docs/prds/PRD-artifact-validation-and-provenance.md:3531-3548` |
| C133 | The pre-change-golden prerequisite lesson | `docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520` |
| C134 | The key-and-label scoping rule for forbidden-field guards | `docs/prds/PRD-artifact-validation-and-provenance.md:2554-2556,3626` |
| C135 | The guard-sensitivity requirement | `docs/prds/PRD-artifact-validation-and-provenance.md:3960-3991` |
| C136 | The accepted §20 list of what this PRD must answer | `docs/prds/PRD-artifact-validation-and-provenance.md:4016-4047` |
| C137 | The accepted slug grammar and its no-echo rule | `docs/prds/PRD-artifact-validation-and-provenance.md:696-772` |
| C138 | The accepted per-artifact classification ladder | `docs/prds/PRD-artifact-validation-and-provenance.md:1653-1782` |
| C139 | The accepted snapshot/instability semantics and their stated limits | `docs/prds/PRD-artifact-validation-and-provenance.md:1827-1917` |
| C140 | The accepted `--help` mitigation for the `apply --mode prepare` collision | `docs/prds/PRD-artifact-validation-and-provenance.md:344-355` |
| C141 | The accepted self-contained-remediation rule | `docs/prds/PRD-artifact-validation-and-provenance.md:374-381` |
| C142 | The accepted docs-update obligations, including the `agent-as-provider` correction | `docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435` |
| C143 | `compareSemanticBody` is the shipped semantic-CAS precedent | `internal/store/resource_publish.go:358-399` |
| C144 | The shipped per-slug lock is a kernel `flock(2)` on an open file description, taken `LOCK_EX\|LOCK_NB` after a statfs preflight and a whole-chain fsync | `internal/rescap/lock_unix.go:63-88` |
| C145 | It is released by **closing the descriptor**, with no separate unlock step that could fail or leave a stale artifact | `internal/rescap/lock_unix.go:43-52` |
| C146 | Its rationale is explicitly that a kernel lock has none of the ABA/staleness problems of a PID-plus-rename protocol, and is released on SIGKILL, crash or power loss | `internal/rescap/lock_unix.go:6-11` |
| C147 | Unsupported build targets refuse unconditionally without creating or opening anything | `internal/rescap/lock_unsupported.go:29-32` |
| C148 | `LockSupported` is a build-tagged constant, true only on linux/darwin today | `internal/rescap/lock_unix.go:26`, `internal/rescap/lock_unsupported.go:15` |
| C149 | The CLI already runs every resource mutation under that lock | `internal/cli/feature_resource.go:101-116` |
| C150 | The local ignore contract **refuses when Git is unavailable** | `internal/workflow/session_ignore.go:138-145` |
| C151 | `IsGitAvailable` shells out to `git rev-parse --is-inside-work-tree` after a `LookPath` | `internal/gitutil/ignore.go:25-34` |
| C152 | `IsPathIgnored` shells out to `git check-ignore -q --no-index` | `internal/gitutil/ignore.go:59-78` |
| C153 | The untracked half shells out to `git --literal-pathspecs ls-files` | `internal/rescap/gitgate.go:104-115` |
| C154 | `redact.Scan` applies the closed six-class resource policy to in-memory bytes and returns sorted, deduplicated codes | `internal/redact/redact.go:81-90` |
| C155 | The six resource classes are private-key, connection-url, email-pii, credential-assignment, bearer-or-key-token, home-absolute-path | `internal/redact/redact.go:53-72` |
| C156 | `Scan` deliberately takes bytes and never a path, so "scan before write" is structural | `internal/redact/redact.go:18-21` |
| C157 | ADR-027 D3 makes redaction a write precondition for every tpatch-owned writer, and a hard failure for committed artifacts | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-177` |
| C158 | `session purge` is dry-run-by-default with an explicit `--yes` gate and a required scope | `internal/cli/session.go:353-419` |
| C159 | …and its rev-1 lesson was that "neither selector" must not silently mean "all" | `internal/cli/session.go:369-375` |
| C160 | `tpatch feature` is the shipped per-feature management group | `internal/cli/feature_deps.go:43-53` |
| C161 | `feature resource remove` / `clear` are the shipped per-feature removal verbs | `internal/cli/feature_resource.go:655,699` |
| C162 | `RefreshFeaturesIndex` rebuilds the index from every feature's status | `internal/store/store.go:703-712` |
| C163 | The toolchain is pinned at Go 1.26.1, which provides `(*os.Root).Rename` | `go.mod:3` |
| C164 | The shipped atomic writer creates its temp with `os.CreateTemp` and renames path-based, so it is **not** a rooted writer | `internal/store/store.go:878-917` |
| C165 | `land`'s journal path containment is validated before use | `internal/cli/land_journal.go:383-415` |
