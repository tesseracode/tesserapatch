# ADR-035 — Intent Bundle Publication and History

**Status**: Proposed — 2026-08-13 (rev-0)
**Date**: 2026-08-13 (Proposed)
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, rev-0 based on dispatch HEAD `20e8bbe`
**Cluster**: WP-005 spec-driven workflows / GH #11
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-027](./ADR-027-capture-context-privacy-boundary.md) (D2 no
raw context, D6 no wall-clock), [ADR-033](./ADR-033-resource-capture-boundary.md)
(D10 no tracked timestamps, D11 no Go map in a wire schema),
[ADR-034](./ADR-034-rooted-filesystem-inspection-boundary.md) (the rooted
**read** boundary, reused unchanged and **not** extended to writes)
**Companion**: [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md)
(rev-0, Draft — Awaiting Review). **The two documents must be reviewed
together.** Read the PRD for the full product contract and its 234-row
acceptance matrix; this ADR states the decisions the PRD's §7, §8 and §9 depend
on, and where the two overlap **this ADR is normative**.
**Blocks**: implementation of the mutating `tpatch prepare` modes (PRD §17
slices S1–S6). No implementation is authorized until both documents are
accepted.

**Revision history**

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | Proposed | First draft: D1–D14. |

---

## Context

The accepted [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
(rev-5) shipped a read-only inspector and deliberately reserved the mutating
half of the `prepare` verb. Its §20 enumerates what a mutating PRD must answer
first: atomic publication, non-destructive overwrite, provenance, partial-
failure exposure, and whether readiness becomes a gate
(`docs/prds/PRD-artifact-validation-and-provenance.md:4016-4047`).

Two of those are architecture decisions rather than product choices, and both
are non-obvious enough that "the agents agreed" is not an acceptable record:

1. **How is a five-file publication made all-or-nothing on a filesystem that
   offers no multi-file atomic rename?** The shipped single-file writer
   (`internal/store/store.go:878-917`) is correct for one file and silent about
   sets. The shipped `land` journal (`internal/cli/land_journal.go:11-23`)
   solves an adjacent problem in the opposite direction.
2. **Where do the bytes that `--regenerate` replaces go?** Today they go
   nowhere: `WriteFeatureFile` truncates in place
   (`internal/store/store.go:443-449,918-923`), and `.tpatch/` is frequently
   untracked (`internal/cli/cobra.go:3405-3407`), so Git is not a reliable
   backstop.

Answering (2) requires selecting a **persistent representation**, which is
precisely the class of choice AGENTS.md requires an ADR for. This ADR records
that selection, its boundaries, and — critically — what it must never be read
as.

### What this ADR is *not*

- It is **not** a provenance decision (D9). The WP-005 provenance trigger stays
  unfired.
- It does **not** extend ADR-034. ADR-034 governs rooted **reads** and its D14
  explicitly forbids citing it as a persistence precedent
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961`). Every write
  in this cluster is governed here.
- It does **not** authorize any new lifecycle state, any semantic quality
  signal, or any change to the accepted `--check` contract.

---

## Decisions

### D1 — Three separate guarantees; only two are claimed

The publication contract is stated as three distinct properties, and the design
claims exactly two:

| | Property | Claimed |
|---|---|---|
| **T0** | Instantaneous multi-file visibility to a concurrent reader | **No** |
| **T1** | Command-boundary all-or-old / all-new | **Yes** |
| **T2** | Crash recoverability to all-old, or an explicit refusal | **Yes** |

**Rationale.** POSIX provides `rename(2)` for one path. Publishing a set is *N*
independent renames; a concurrent reader can observe any prefix applied. No
journal, lock, fsync ordering or temp-directory trick changes that. A design
document that says "atomic publication" without this split trains reviewers to
accept a guarantee that does not exist.

**Consequence.** The word "atomic" (and "atomically", "simultaneously") is
forbidden in any shipped string, help text, doc sentence or report field that
describes the multi-file publication. It remains correct for the single-file
`status.json` write of `--manual` (D3). A mechanical over-claim guard enforces
this (PRD PIB-155), with a sensitivity fixture.

**Alternatives rejected.** (a) Claim T0 and hope: dishonest. (b) Achieve T0 by
publishing a whole replacement *directory* and swapping it: directory swap is
not atomic either, invalidates open descriptors, and would move audit-trail
files (`patches/**`, `record.md`) the command has no business touching. (c)
Achieve T0 with an advisory lock every reader must take: would require adding
lock acquisition to every read command in the CLI, converting a read-only
inspector into a lock client — rejected as a much larger blast radius than the
window it closes.

### D2 — Writes get their own boundary; ADR-034 covers reads only

Every canonical **read** in the mutating flow — preflight inspection,
generation context, publish-time revalidation — uses the accepted ADR-034
mechanism unchanged: one held `*os.Root`, `fs.ValidPath` root-relative names,
observed-symlink refusal, non-regular refusal, bounded `Max+1` reads, identity
checks, fail-closed `unix || windows` allowlist.

Every **write** obeys, additionally:

1. Targets come from a **closed compile-time list** of root-relative names
   derived from the validated slug. No write path is ever composed from
   provider output, report data or file content.
2. Immediately before each rename, the target's `Lstat` through the held root
   must show non-existence or a **regular file**. A symlink, junction,
   directory, FIFO, socket or device at a publication target aborts the
   transaction. Publication never follows a link and never writes through one.
3. Temp files are created in the destination directory so the rename is
   same-filesystem, matching `internal/store/store.go:882-886`.
4. Modes: `0644` for new tracked files, preserved permission bits on
   replacement (`internal/store/store.go:871-876`), `0700`/`0600` under
   `.tpatch/local/`.

**Rationale.** ADR-034's guarantee is about *name resolution* for reads. A write
that resolves safely can still clobber a symlink the operator created
deliberately, or write through it to a target outside the feature directory.
Rule 2 closes that without inventing a second path model.

### D3 — A single-file publication takes no journal

`--manual` writes exactly one file (`status.json`). One rename **is** atomic in
the ordinary POSIX sense, so that path uses the shipped `writeFileAtomic`
(`internal/store/store.go:871-917`) directly, with no journal, no archive and
no preimage capture. It still takes the transaction lock (D4) so it cannot
interleave with a sibling mutating `prepare`.

**Rationale.** Ceremony that protects nothing is a cost with no benefit, and it
would blur the line D1 exists to draw: this is the one case where "atomic" is
the correct word.

### D4 — One per-feature exclusive lock, held by mutating `prepare` only

- Path: `.tpatch/local/intent-prepare/<slug>/prepare.lock`.
- `O_CREATE|O_EXCL|O_WRONLY`, `0600`, nonce-bearing, fsynced — the shipped
  pattern at `internal/cli/land_journal.go:629-648`.
- A stale lock is removed **only** when its nonce matches the pending journal's;
  any other lock is left untouched and refuses the command — the shipped
  owned-lock discipline at `internal/cli/land_journal.go:675-698`.
- Ordering: lock → recovery → inspect → stage → revalidate → archive → journal
  → publish → clear → unlock.

**The authority is deliberately narrow, and the limit is part of the decision.**
The lock excludes only another mutating `prepare` on the same slug. It does not
exclude `define`, `cycle`, an editor or a script, because this ADR does **not**
add lock acquisition to any shipped command. What covers those writers is
detection, not exclusion: a publish-time revalidation of every entry's identity
against the preflight capture, under the lock, immediately before the first
rename. A mismatch aborts before the window opens.

**Rationale for not locking everywhere.** Adding a lock to eleven shipped verbs
creates a deadlock surface, a new failure mode on every one of them, and a
compatibility delta far larger than the window it closes — while the window is
already detected. See D6 for the residual limit that remains.

### D5 — The journal is **undo-only**; recovery never rolls forward

The journal is a durable record of *how to get back to the old state*, not of
*how to finish*. Recovery restores preimages and lets the operator re-run.

**Rationale, and the contrast with `land`.** `land` must roll forward because
`git commit` has already advanced HEAD and cannot be undone
(`internal/cli/land_journal.go:11-23`); its journal is therefore a redo log with
a three-way evidence matrix. Nothing in an intent publication is irreversible:
the new artifacts did not exist before, generation is re-runnable, and the
staged bytes live in a gitignored lane that may legitimately have been pruned.
Roll-forward would therefore add failure modes (missing staging, stale staged
bytes, a partially valid set) to buy nothing.

**Recovery decides from evidence, never from a recorded phase.** The journal
carries **no** `phase` field at all — `land` keeps one and marks it advisory
(`internal/cli/land_journal.go:110-111`); this design removes the temptation
entirely. The decision function over each entry is:

| Evidence | Outcome |
|---|---|
| every entry == new-image | **complete** — clear the journal, undo nothing |
| every entry == preimage | **nothing published** — clear the journal |
| each entry is preimage **or** new-image, mixed | **partial** — undo the published ones, clear |
| any entry matches neither | **divergent** — refuse, preserve everything, name the journal and the archive |

The "complete" row is why evidence beats phase: a crash between the final
rename and the journal clear is indistinguishable from a completed transaction
by any marker that could itself be interrupted.

**Journal determinism.** No wall-clock field. `land`'s journal records
`created_at` (`internal/cli/land_journal.go:109`); this one does not, because
nothing in recovery reads time, and omitting it makes two identical operations
produce identical journals — a testable property. Strict decoding (unknown
fields and trailing content refused, version mismatch refused) follows
`internal/cli/land_journal.go:348-380` and
`internal/store/resource_publish.go:305-320`.

### D6 — The residual race is disclosed, not closed

A write that lands **between** the revalidation and that entry's rename is
overwritten, and its bytes are **not** archived. The design narrows the window
(revalidation is the last act before publication, under the lock) but does not
eliminate it, and no acceptance row claims the write is preserved.

**Rationale.** The only ways to close it are (a) an exclusive lock every writer
takes (rejected in D4) or (b) an open-descriptor-based publication that
verifies identity at rename time, which POSIX `rename(2)` does not offer
(`renameat2(RENAME_EXCHANGE)` is Linux-only and still not a content check).
Overstating this would repeat exactly the failure ADR-034 D8 corrected for the
read side, where an identity promise was weakened to "an object observed as
different is never read".

### D7 — Publication order is fixed, with `status.json` last

`analysis.md` → `spec.md` → `exploration.md` → `artifacts/analysis.json` →
`artifacts/intent-archive/index.json` → `status.json`. Each entry's parent
directory is fsynced after its rename.

**Rationale.** `status.json` carries the claim "this feature is `defined`". A
crash before it leaves new artifacts and an old state — recoverable, and never
a false completeness claim. The reverse order would publish the claim first.
Fixing the order also makes the crash-phase table (PRD §7.9) enumerable rather
than combinatorial.

### D8 — Replaced bytes go to a **tracked**, content-addressed, immutable archive

`--regenerate` copies every artifact it is about to replace into
`.tpatch/features/<slug>/artifacts/intent-archive/blobs/<sha256>.blob`, with a
tracked `index.json` naming which artifacts each generation replaced. The
canonical files remain the **sole** authority; no reader consults the archive
to determine current state.

**Rationale for durable-and-tracked over the alternatives:**

- *Ephemeral rollback-only preimages* satisfy crash-safety and lose the bytes on
  the **success** path, which is the path the requirement is about.
- *Git* is not available as a guarantee: `.tpatch/` is frequently untracked, and
  a Path B agent's freshly authored files are uncommitted by construction.
- *`.tpatch/local/`* would make recovery vanish on clone or machine change.
- *Pointer/symlink generation directories* would make the canonical artifacts
  symlinks, which the accepted inspector refuses by design
  (`docs/prds/PRD-artifact-validation-and-provenance.md:1768`) — every prepared
  feature would classify as `symlink-refused`.
- *A hash-only record on `FeatureStatus`* proves change but cannot recover
  content, and inlining content would bloat the hottest file in the feature
  directory.

**No new exposure class.** The archived bytes are the bytes of files that live
at tracked paths in the same directory; the identical content was already
committable at `analysis.md`.

**Shape is inherited, not invented.** Immutable content-addressed set plus one
atomically-rewritten pointer/manifest is the shipped resource-capture design
(`internal/store/resource_publish.go:1-9,219-278`), including its idempotent
re-publish (`:236-244`) and its named collision/corruption refusals (`:195-202`).

### D9 — The archive is **not** provenance, and the provenance trigger stays unfired

The archive records *that these exact bytes existed at this path*. It records
no author, agent, model, provider, endpoint, Path-A/Path-B tag or any other
source claim, and its `mode` field names the **replacing invocation's mode**,
never the origin of the replaced bytes.

Therefore:

- The WP-005 provenance ADR trigger — which fires on "selection of a persistent
  **provenance** representation" and on nothing else
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972`) — remains
  **unfired**.
- `prepare --check` keeps emitting the constant `provenance: unknown` for every
  artifact and **must not** consult the archive.
- The accepted forbidden-inference list
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920`) gains
  exactly one entry: the intent archive.
- Neither this ADR nor ADR-034 may be cited as precedent for persisting
  provenance. A PRD that wants to answer "who wrote this" must write the
  provenance ADR first.

**Rationale for stating it this explicitly.** A durable per-artifact byte store
is one field away from being read as a provenance store. The distinction has to
be written down in the normative document, or it will be lost in the first PRD
that finds the archive convenient.

### D10 — Content-addressed, deterministic identifiers; no wall-clock in tracked bytes

- Blob name: lowercase-hex SHA-256 of the file's exact raw bytes, `.blob`
  suffix. `^[0-9a-f]{64}\.blob$`.
- `generation_id`: lowercase-hex SHA-256 over the canonical encoding of
  `{feature, mode, replaced[]}` with `replaced` sorted by `artifact_id`,
  derived at **exactly one** point so every consumer agrees by construction —
  the shipped pattern at `internal/store/resource_publish.go:131-141,152-160`.
- `index.json`: `schema_version: 1`, fixed key order by struct declaration
  order, **no Go map anywhere** (ADR-033 D11,
  `internal/store/canonjson.go:11-17`), **no wall-clock field** (ADR-027 D6,
  ADR-033 D10).
- Appending a generation whose id already exists is a **no-op**: the archive is
  a *set* of distinct generations, not a chronology. `generations` order is
  publication order and carries no wall-clock claim, because no clock is
  recorded.
- A blob whose on-disk bytes disagree with its own name is corruption and
  refuses the invocation; it is never overwritten. A `generation_id` collision
  between distinct canonical bodies likewise refuses. Both follow
  `internal/store/resource_publish.go:199-203`.

### D11 — Two small metadata preimages live in the journal directory, not the archive

`index.json` and `status.json` preimages are stored as raw bytes under
`.tpatch/local/intent-prepare/<slug>/`, `0600`, removed with the journal.

**Rationale.** Archiving the archive index in the archive is circular.
`status.json` is lifecycle metadata, not intent content, and putting it in an
*intent* archive would blur what the archive means. Both are small,
machine-written and reconstructible in principle; they are kept as raw bytes
because rollback must not depend on re-deriving them.

### D12 — Generators are extracted as pure functions; the incremental writers keep their behavior

`RunAnalysis`, `RunDefine` and `RunExplore` are split into a pure generation
half and their existing publication half. The pure half takes no `*store.Store`,
holds no writer, and performs no filesystem write. The existing functions are
refactored to call it and keep byte-identical observable behavior, pinned
against goldens captured **before** the refactor.

Corollary: `RetryOptions.Store` is left nil on the prepare path, because a
non-nil store makes the shipped retry loop write raw responses into the
canonical artifacts directory mid-generation
(`internal/workflow/retry.go:105-109`). Raw responses are captured through a
sink into the gitignored staging tree instead, and are never published.

**Rationale.** Calling the shipped functions would publish four to twelve
canonical files before the transaction decided anything — exactly what WP-005
Turn 3 rules out
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`). Extraction
without goldens would risk regressing four shipped commands, so the goldens are
a slice prerequisite, not a nicety.

### D13 — Recovery has exactly two entry points, and neither is a read command

- **Automatic**: every mutating `prepare` for that slug, under the lock, before
  any other work — the shipped shape at `internal/cli/land_journal.go:445-482`.
- **Diagnostic**: one new `doctor` check that *reports* a pending journal,
  never acts on it, never takes the lock and never changes `doctor`'s exit code.

No other command recovers, refuses, or blocks on a pending journal.

**Rationale.** The evaluated alternative — "every relevant command must recover
or refuse" — would put a write path and a lock into read-only commands and let
a stale journal wedge `tpatch status`. It is safe to decline because recovery
is undo-only (D5): an un-recovered journal leaves the tree in a state that is
already one of the enumerated crash phases, and the accepted read-only check
describes that state truthfully without needing to know the journal exists.

**Consequently there is no pointer or generation-indirection model.** The
canonical files are self-describing; no reader needs to consult a pointer, a
generation directory or the journal to know what the current bundle is. That is
a deliberate rejection of the pointer-based alternative, which would have made
every reader depend on control state.

### D14 — The accepted read-only contract is frozen

`prepare --check` keeps its grammar, precedence, schema, exact bytes, exit
envelope and zero-mutation contract. The mutating modes may only be added
*above* it in the mode dispatch. In particular the check's output is
byte-identical whether or not a prepare journal exists, given identical
canonical bytes: the journal is transient control state in a gitignored lane,
not an artifact, and it has no state in the accepted nine-value enum.

**Rationale.** The check was accepted on its exact bytes. Adding a field would
be a `schema_version` change to an accepted contract, and this cluster's whole
value proposition is that the read half stays trustworthy while the write half
is added.

---

## Consequences

**Positive**

- `--regenerate` becomes a safe operation with a durable, Git-independent
  recovery route, and the safety is content-addressed, so repeated regeneration
  of identical content costs zero bytes.
- The transaction contract is stated in terms a reviewer can falsify: T0 is
  explicitly not claimed, and T1/T2 have enumerable crash phases.
- Four shipped commands keep byte-identical behavior, pinned by goldens.
- The read boundary (ADR-034) and the write boundary (this ADR) stay separable,
  so a future change to one does not silently license the other.

**Negative / accepted costs**

- The tracked tree grows: one blob per distinct replaced content, capped
  individually by the accepted artifact byte cap. No pruning verb ships in v1.
- A new tracked directory appears under `artifacts/`, visible in `git status`
  and swept by `land`'s feature path set (`internal/cli/land.go:723-725`).
- The lock's authority is narrow, so a concurrent `define` can still lose a
  write inside the publication window (D6).
- Extraction touches `internal/workflow/workflow.go`, a shared surface, which
  forces sequential implementer dispatch for the slices that touch it.
- Recovery is undo-only, so an interrupted prepare always costs a re-run of
  generation, including its provider calls.

---

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Claim multi-file atomicity | False; see D1. |
| Whole-directory copy-on-write and swap | Directory swap is not atomic, breaks open descriptors, and would move audit-trail files this command does not own. |
| Redo-style journal, like `land` | Nothing here is irreversible; roll-forward adds failure modes for no benefit (D5). |
| Lock acquisition in every writing command | Deadlock surface and a compatibility delta across eleven verbs, to close a window revalidation already detects (D4, D6). |
| Ephemeral preimages only | Loses bytes on the success path — the path the requirement is about (D8). |
| Git as the preservation guarantee | `.tpatch/` is frequently untracked and fresh Path B work is uncommitted (D8). |
| Archive in `.tpatch/local/` | A guarantee that vanishes on clone is not a guarantee (D8). |
| Pointer/symlink generation directories | The accepted inspector refuses symlinks, so every prepared feature would classify unsafe (D8, D13). |
| Provenance sub-record on `FeatureStatus` | Would fire the provenance trigger and answer a question this cluster is not authorized to answer (D9). |
| Workspace-level blob store with cross-feature dedupe | Couples feature directories, complicates `land`'s per-feature path set and makes feature removal ambiguous (PRD Q7). |
| Adding a field to `prepare --check` for the pending journal | Changes an accepted, frozen schema (D14). |

---

## Acceptance dependencies

Each decision is verified by named rows in the companion PRD's §18 matrix.

| Decision | Verified by (PRD §18) |
|---|---|
| D1 three guarantees; T0 not claimed | PIB-103, PIB-155, PIB-206 |
| D2 write boundary, no-follow, closed target list | PIB-148…PIB-151, PIB-154, PIB-182, PIB-183 |
| D3 single-file `--manual` publication | PIB-051, PIB-052 |
| D4 lock scope and owned-lock discipline | PIB-053, PIB-080, PIB-117, PIB-124, PIB-125 |
| D5 undo-only, evidence-based recovery | PIB-113, PIB-116…PIB-123, PIB-126, PIB-164 |
| D6 disclosed residual race | PIB-100…PIB-105 |
| D7 fixed publication order, status last | PIB-108, PIB-109, PIB-121, PIB-122 |
| D8 tracked content-addressed archive | PIB-060…PIB-068, PIB-071, PIB-152, PIB-153 |
| D9 archive is not provenance; trigger unfired | PIB-140…PIB-147 |
| D10 deterministic ids, no wall-clock, set semantics | PIB-064, PIB-065, PIB-069, PIB-160…PIB-162, PIB-165, PIB-168, PIB-169 |
| D11 metadata preimages in the journal directory | PIB-119, PIB-122, PIB-164 |
| D12 pure generators, goldens, nil retry store | PIB-184…PIB-190, PIB-208…PIB-211 |
| D13 two recovery entry points only | PIB-079, PIB-133…PIB-136, PIB-177, PIB-213 |
| D14 accepted check frozen | PIB-198…PIB-207 |

---

## References

- [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md) — companion
- [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md) — accepted prerequisite
- [ADR-034 rooted filesystem inspection boundary](./ADR-034-rooted-filesystem-inspection-boundary.md)
- [ADR-033 resource capture boundary](./ADR-033-resource-capture-boundary.md)
- [ADR-027 capture context privacy boundary](./ADR-027-capture-context-privacy-boundary.md)
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md)
- `internal/cli/land_journal.go` — the shipped journal and recovery precedent
- `internal/store/resource_publish.go` — the shipped content-addressed publication precedent
- `internal/store/store.go` — the shipped single-file atomic writer
