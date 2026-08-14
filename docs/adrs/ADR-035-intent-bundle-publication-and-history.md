# ADR-035 — Intent Bundle Publication and History

**Status**: Proposed — Awaiting Review (rev-1), 2026-08-14
**Date**: 2026-08-13 (Proposed rev-0), 2026-08-14 (rev-1)
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, rev-1 based on dispatch HEAD `2e2a67d`
**Cluster**: WP-005 spec-driven workflows / GH #11
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-027](./ADR-027-capture-context-privacy-boundary.md)
(**D3 redaction as a write precondition**, D2 no raw context, D5 retention
semantics, D6 no wall-clock),
[ADR-033](./ADR-033-resource-capture-boundary.md)
(D4 the closed six-class scan, D10 no tracked timestamps, D11 no Go map in a
wire schema),
[ADR-034](./ADR-034-rooted-filesystem-inspection-boundary.md) (the rooted
**read** boundary, reused unchanged and **not** extended to writes)
**Companion**: [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md)
(rev-1, Draft — Awaiting Review). **The two documents must be reviewed
together.** Read the PRD for the full product contract and its 394-row
acceptance matrix; this ADR states the decisions the PRD's §7, §8 and §9 depend
on, and where the two overlap **this ADR is normative**.
**Blocks**: implementation of the mutating `tpatch prepare` modes (PRD §17.2
slices S1–S6). No implementation is authorized until both documents are
accepted **and** the accepted `prepare --check` contract is implemented and
landed (PRD §17.1).

**Revision history**

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | Proposed | First draft: D1–D14. |
| rev-1 | Proposed — Awaiting Review | Closes the rev-0 review findings. **D4 replaced**: the `O_EXCL`-plus-nonce lock (unprovable after CP1/CP2/CP8 and every `--manual` crash) is replaced by a kernel, process-lifetime advisory lock whose ownership dies with the process. **D5 rebuilt**: plan-digest binding, old-and-new identity for every canonical **and** metadata entry, and a semantic compare-and-swap before every publish, undo and remove. **D6 rescoped**: T1 is a command-owned final verification, not a claim about the world; the residual race is narrowed to content identity. **D2 rebuilt**: `(*os.Root).Rename` and the rooted write primitives close the ancestor-swap race. **D8 conditioned**: the archive selection now includes the ADR-027 D3 redaction precondition and a bounded removal surface; the "no new exposure class" claim is withdrawn. **D13 broadened**: `doctor` reports every residue (pending journal, live transaction, inert lock, staging, abandoned evidence, orphan blobs, corrupt index, journal loss) and an operator abandon route exists. **New**: D15 redaction gate, D16 retention/removal, D17 conditional Git dependency, D18 provider authority for `--regenerate`, D19 default-mode coherence, D20 `FEATURES.md` outside the transaction. |

---

## Context

The accepted [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md)
(rev-5) shipped a read-only inspector and deliberately reserved the mutating
half of the `prepare` verb. Its §20 enumerates what a mutating PRD must answer
first: atomic publication, non-destructive overwrite, provenance, partial-
failure exposure, and whether readiness becomes a gate
(`docs/prds/PRD-artifact-validation-and-provenance.md:4016-4047`).

Four of those are architecture decisions rather than product choices, and each
is non-obvious enough that "the agents agreed" is not an acceptable record:

1. **How is a six-file publication made all-or-nothing on a filesystem that
   offers no multi-file atomic rename?** The shipped single-file writer
   (`internal/store/store.go:878-917`) is correct for one file, silent about
   sets, and path-based. The shipped `land` journal
   (`internal/cli/land_journal.go:11-23`) solves an adjacent problem in the
   opposite direction.
2. **What makes mutual exclusion provable after a crash?** A lock file's
   existence is data, not liveness. The shipped resource-capture design already
   records why: a PID-plus-rename protocol has ABA and staleness problems a
   kernel lock does not (`internal/rescap/lock_unix.go:6-11`).
3. **Where do the bytes that `--regenerate` replaces go?** Today they go
   nowhere: `WriteFeatureFile` truncates in place
   (`internal/store/store.go:443-449,918-923`), and `.tpatch/` is frequently
   untracked (`internal/cli/cobra.go:3405-3407`), so Git is not a reliable
   backstop.
4. **What bounds the thing that keeps them?** A durable store of replaced bytes
   is a retention decision, and ADR-027 D3 already governs what a tpatch-owned
   writer may persist
   (`docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-177`).

Answering (3) requires selecting a **persistent representation**, which is
precisely the class of choice AGENTS.md requires an ADR for. Answering (4)
means that selection cannot be made without its privacy and retention
conditions. This ADR records the selection, its conditions, its boundaries,
and — critically — what it must never be read as.

### What this ADR is *not*

- It is **not** a provenance decision (D9). The WP-005 provenance trigger stays
  unfired.
- It does **not** extend ADR-034. ADR-034 governs rooted **reads** and its D14
  explicitly forbids citing it as a persistence precedent
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961`). Every write
  in this cluster is governed here.
- It does **not** authorize any new lifecycle state, any semantic quality
  signal, or any change to the accepted `--check` contract.
- It does **not** claim the canonical files stop being the sole authority. The
  archive is recovery material; no reader consults it to decide current state.
- It does **not** authorize implementation. Both documents must be accepted,
  and the accepted `--check` contract must be implemented and landed first
  (D14).

---

## Decisions

### D1 — Three separate guarantees; only two are claimed, and one is scoped

The publication contract is stated as three distinct properties, and the design
claims exactly two:

| | Property | Claimed |
|---|---|---|
| **T0** | Instantaneous multi-file visibility to a concurrent reader | **No** |
| **T1** | **Command-owned final verification**: at a point under the lock, after the last write and before the command returns, every publication entry matches its intended new image — or divergence is reported | **Yes, scoped** |
| **T2** | Crash recoverability to all-old, or an explicit refusal | **Yes, bounded** (D21) |

**Rationale.** POSIX provides `rename(2)` for one path. Publishing a set is *N*
independent renames; a concurrent reader can observe any prefix applied. No
journal, lock, fsync ordering or temp-directory trick changes that. A design
document that says "atomic publication" without this split trains reviewers to
accept a guarantee that does not exist.

**T1's scope is part of the decision.** rev-0 said "at the command boundary the
tree is entirely the new bytes". That is a claim about the *world*, and no
command that does not exclude every writer can own it. rev-1 claims only what
this command can verify under its own lock, and states that writes landing
after that verification point are outside the postcondition (PRD §7.1).

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

### D2 — Writes get their own boundary, and it is **rooted**

Every canonical **read** in the mutating flow — preflight inspection,
generation context, publish-time revalidation — uses the accepted ADR-034
mechanism unchanged: one held `*os.Root`, `fs.ValidPath` root-relative names,
observed-symlink refusal, non-regular refusal, bounded `Max+1` reads, identity
checks, fail-closed `unix || windows` allowlist.

Every **write** obeys, additionally:

1. Targets come from a **closed compile-time list** of root-relative names
   derived from the validated slug. No write path is ever composed from
   provider output, report data or file content.
2. **Every write is handle-relative.** Temp creation is
   `(*os.Root).OpenFile(name, O_CREATE|O_EXCL|O_WRONLY, 0600)` in the
   destination directory; publication is `(*os.Root).Rename`; removal is
   `(*os.Root).Remove`; directory creation is `(*os.Root).Mkdir`. The pinned
   toolchain provides all of them (`go.mod:3`, Go 1.26.1). **No path-based
   `os.Rename`, `os.CreateTemp`, `os.WriteFile` or `filepath.Join(repoRoot, …)`
   appears in the publication path** (PRD PIB-308).
3. Immediately before each rename, the target's `Lstat` through the held root
   must show non-existence or a **regular file**. A symlink, junction,
   directory, FIFO, socket or device at a publication target aborts the
   transaction. Publication never follows a link and never writes through one.
4. Modes: `0644` for new tracked files, preserved permission bits on
   replacement (`internal/store/store.go:871-876`), `0700`/`0600` under
   `.tpatch/local/`.

**Rationale for the rooted rewrite.** rev-0 performed rooted *reads* and then a
path-based `os.Rename`, so a concurrent rename of an ancestor directory could
redirect a publication write outside the intended tree — the exact TOCTOU the
read boundary exists to prevent, reintroduced on the write side. `*os.Root`
resolves every component against a held directory handle, so the **pathname**
half of the race is closed. What remains is the **content** half: a writer can
still replace the destination's bytes between an entry's compare-and-swap and
its rename (D6). rev-0 over-conceded by describing the pathname half as
unclosable.

**Not reused: the shipped single-file writer.** `writeFileAtomicWithRename`
(`internal/store/store.go:878-917`) is path-based by construction. It is left
untouched for its shipped callers, including `--manual`'s single `status.json`
write (D3), and is never used by the transactional publication path.

### D3 — A single-file publication takes no journal

`--manual` writes exactly one *publication-set* file (`status.json`). One
rename **is** atomic in the ordinary POSIX sense, so that path uses the shipped
`writeFileAtomic` (`internal/store/store.go:871-917`) directly, with no
journal, no archive and no preimage capture. It still takes the transaction
lock (D4) so it cannot interleave with a sibling mutating `prepare`.

**The publication set is not the process's whole write footprint.**
`SaveFeatureStatus` also refreshes the derived `FEATURES.md` best-effort
(`internal/store/store.go:363-377`). D20 states that carve-out; the atomicity
claim here is scoped to the publication set and to nothing else.

**Rationale.** Ceremony that protects nothing is a cost with no benefit, and it
would blur the line D1 exists to draw: this is the one case where "atomic" is
the correct word.

### D4 — The transaction lock is a **kernel, process-lifetime** advisory lock

**Replaces rev-0's `O_EXCL`-plus-nonce lock, which was unprovable.**

Under rev-0, a crash at CP1 (lock created, journal not yet written), CP2 (blobs
written, journal not yet written), CP8 (journal cleared, lock not released) or
anywhere in `--manual` (which writes no journal at all) left a lock file with
no journal to match its nonce against. rev-0's own rule then left that lock
untouched and refused the command — **permanently**, with no route out except
an undocumented manual `rm`. A recovery model that depends on a journal a crash
may have prevented from existing is not a recovery model.

**The decision:**

| Platform | Mechanism | Contention | Release |
|---|---|---|---|
| `linux`, `darwin`, `freebsd`, `netbsd`, `openbsd`, `dragonfly` | `syscall.Flock(fd, LOCK_EX\|LOCK_NB)` on an `O_CREATE\|O_RDWR` `0600` descriptor — the shipped `rescap` sequence (`internal/rescap/lock_unix.go:63-88`) | `EWOULDBLOCK`/`EAGAIN` | descriptor close, including on `SIGKILL`, panic or power loss (`internal/rescap/lock_unix.go:43-52`) |
| `windows` | `syscall.CreateFile` with `dwShareMode = 0`, `OPEN_ALWAYS`, `FILE_ATTRIBUTE_NORMAL\|FILE_FLAG_OPEN_REPARSE_POINT` | `ERROR_SHARING_VIOLATION` (32) / `ERROR_LOCK_VIOLATION` (33), declared locally as `syscall.Errno` constants because the stdlib does not export them | the OS closing the handle at process exit |
| `solaris`, `aix`, `js`, `plan9`, `wasip1` | none — **refuse**, fail-closed, before any write (`internal/rescap/lock_unsupported.go:29-32` is the shipped shape) | — | — |

Consequences that are part of the decision:

1. **The lock file carries no authority and no data.** Zero bytes: no nonce, no
   PID, no hostname. A leftover file is inert and never causes a refusal. There
   is no "stale lock" concept, therefore no stale-lock policy, therefore no
   crash phase that can brick a slug.
2. **Liveness is decidable from the kernel alone**, with or without a journal:
   acquisition succeeds (no live holder) or returns the contention errno (live
   holder). Contention is a **transient** refusal — exit 3,
   `transaction-in-progress` — not exit 6.
3. **Extraction, not invention.** The unix implementation is the shipped
   `rescap` lock moved into a shared package, with `rescap` keeping its
   behavior, refusal codes, statfs preflight and `linux || darwin` envelope
   byte-identical against pre-change goldens (PRD PIB-286, PIB-287).
4. **The filesystem preflight travels with the lock.** `flock(2)` is only
   trusted on genuinely local filesystems, so the shipped `statfs` allowlist
   (`internal/rescap/statfs_linux.go:78-96`,
   `internal/rescap/statfs_darwin.go:59-75`) runs before anything is created,
   and a non-allowlisted filesystem **refuses** (`lock-filesystem-unsupported`).
   A lock that may not exclude is worse than no lock, because the entire
   recovery model rests on liveness being decidable. On Windows the deny-share
   open is enforced by the filesystem driver locally and by the SMB redirector
   for network shares; that limit is disclosed rather than assumed away (PRD
   PIB-392, PIB-393).
5. **The mutating platform envelope is narrower than the read envelope**
   (`unix || windows`). That delta is enumerated (PRD §12.6 D8) rather than
   hidden, and `--check` is unaffected on the excluded targets.
6. **No `--force`, ever.** Nothing in this design removes a lock another
   process may hold, because nothing needs to.

**The authority is deliberately narrow, and the limit is part of the decision.**
The lock excludes only another mutating `prepare` on the same slug (and the
archive-purge subcommand, which takes the same lock). It does not exclude
`define`, `cycle`, an editor or a script, because this ADR does **not** add
lock acquisition to any shipped command. What covers those writers is detection,
not exclusion: a set-level revalidation before the window opens, plus a
per-entry compare-and-swap immediately before each rename (D5). See D6 for the
residual limit that remains.

### D5 — The journal is **undo-only**, plan-bound, and every step is CAS-gated

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

**Every path is modelled, including the metadata paths.** Each of the six
publication entries — three Markdown artifacts, the sidecar, the archive index,
`status.json` — carries `action`, a full **preimage** and a full **intended new
image**. rev-0 modelled the two metadata files as raw side-preimages outside the
entry model, which is why it could not state a rollback rule for them. Their
prior *bytes* still live as raw files in the journal directory (D11); their
*identities* live in the entry model like everything else.

**The journal is bound to its plan and to its feature.** Ten strict binds run
before any restore: single JSON value then EOF; no unknown fields; matching
`version`; matching `slug`; closed `mode`; well-formed `run_nonce`; every path
root-relative and contained; `plan_digest` recomputed over the canonical
encoding of `entries`; closed, unique `artifact_id`s; no null arrays (PRD §7.5,
J1–J10). A forged journal fails `plan_digest`; a foreign one fails `slug`; a
truncated one fails the decoder. Any failure refuses with exit 6, preserves
everything, and names the abandon route (D13).

**Every mutating step is a semantic compare-and-swap.** Before publishing an
entry its current identity must equal the preimage (or absence for a `create`);
before undoing an entry its current identity must equal the **new image**;
before writing a blob the existing blob must be absent or byte-consistent with
its own name; before rewriting the index its identity must equal the recorded
preimage. A mismatch on a publish aborts (exit 5, roll back); a mismatch on an
undo **refuses that entry** (exit 6, preserve). Two properties follow:

- A concurrent edit is never clobbered by an undo — the undo refuses instead.
- A journal that survives the binds still cannot cause a write over content it
  did not expect.

The shipped precedent is `PublishBatch`, which compares an existing immutable
file's semantic body before reusing or refusing it and never rewrites it in
place (`internal/store/resource_publish.go:230-285`, `internal/store/resource_publish.go:358-399`).

**Recovery decides from evidence, never from a recorded phase.** The journal
carries **no** `phase` field at all — `land` keeps one and marks it advisory
(`internal/cli/land_journal.go:110-111`); this design removes the temptation
entirely. The decision function over each entry is:

| Evidence | Outcome |
|---|---|
| every entry == new-image | **complete** — clear the journal, undo nothing |
| every entry == preimage | **nothing published** — clear the journal |
| each entry is preimage **or** new-image, mixed | **partial** — undo the published ones (each CAS-gated), clear |
| any entry matches neither | **divergent** — refuse, preserve everything, name the journal, the archive and the abandon route |

The "complete" row is why evidence beats phase: a crash between the final
rename and the journal clear is indistinguishable from a completed transaction
by any marker that could itself be interrupted.

**Journal determinism.** No wall-clock field. `land`'s journal records
`created_at` (`internal/cli/land_journal.go:109`); this one does not, because
nothing in recovery reads time, and omitting it makes two identical operations
produce identical journals — a testable property. Strict decoding follows
`internal/cli/land_journal.go:348-380` and
`internal/store/resource_publish.go:305-328`.

**Identity is `(exists, sha256, size, mode)`.** The shipped
`landJournalFileState` is `(exists, sha256, mode)` and carries **no size**
(`internal/cli/land_journal.go:65-79`); rev-0 mis-cited it. Size is added
deliberately as a cheap mismatch signal and is *compared*, never *trusted* —
equality still requires the hash.

### D6 — The residual race is content identity, and it is disclosed

A write that lands **between** an entry's compare-and-swap and that entry's
rename is overwritten, and its bytes are **not** archived. The window is a few
instructions under the lock, and no acceptance row claims the write is
preserved.

**What rev-1 closed, and what it did not.** D2's rooted writes close the
**pathname** half (no ancestor swap can redirect the write). The **content**
half remains, because closing it would require either (a) an exclusive lock
every writer takes (rejected in D4) or (b) a rename that verifies destination
content at rename time, which POSIX does not offer
(`renameat2(RENAME_EXCHANGE)` is Linux-only and still not a content check).

**Writes after the final verification point are outside the postcondition, not
a violated guarantee.** T1 is scoped to what the command verifies under its own
lock (D1). A write landing after that is the ordinary consequence of not
excluding other writers, and the PRD's concurrency matrix says so rather than
implying the tree stays new forever.

Overstating any of this would repeat exactly the failure ADR-034 D8 corrected
for the read side, where an identity promise was weakened to "an object
observed as different is never read".

### D7 — Publication order is fixed, with `status.json` last

`analysis.md` → `spec.md` → `exploration.md` → `artifacts/analysis.json` →
`artifacts/intent-archive/index.json` → `status.json`. Each entry's parent
directory is fsynced after its rename.

**Rationale.** `status.json` carries the claim "this feature is `defined`". A
crash before it leaves new artifacts and an old state — recoverable, and never
a false completeness claim. The reverse order would publish the claim first.
Fixing the order also makes the crash-phase table (PRD §7.10) enumerable rather
than combinatorial.

### D8 — Replaced bytes go to a durable, content-addressed archive — **conditioned on D15 and D16**

`--regenerate` copies every artifact it is about to replace into
`.tpatch/features/<slug>/artifacts/intent-archive/blobs/<sha256>.blob`, with an
`index.json` naming which artifacts each generation replaced. The canonical
files remain the **sole** authority; no reader consults the archive to
determine current state.

**The selection is conditional.** An archive without D15's redaction
precondition and D16's removal surface is a different decision — one that
silently retains credentials forever — and it is **not** what is selected here.
D8, D15 and D16 are accepted or rejected together.

**Rationale for durable over the alternatives:**

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

**Tracked is conditional, and no clone-durability is claimed before a commit.**
The archive is tracked **iff the operator tracks `.tpatch/`** — which the CLI
already detects (`internal/cli/cobra.go:3405-3407`). Where it is untracked, the
archive survives crashes and re-runs on that machine and does **not** survive a
fresh clone. No shipped string or document may claim otherwise.

**"No new exposure class" is withdrawn.** rev-0 argued that archiving changed
nothing because the same content was already committable at `analysis.md`. That
is wrong: before this design, overwriting a file **destroyed** the prior bytes,
which was the operator's removal mechanism. The archive converts a destructive
act into durable retention, and once committed that retention is in every
clone's history. The exposure class is *retention*, and D15/D16 exist because
of it.

**Shape is inherited, not invented.** Immutable content-addressed set plus one
atomically-rewritten pointer/manifest is the shipped resource-capture design
(`internal/store/resource_publish.go:1-9,219-285`), including its idempotent
re-publish (`internal/store/resource_publish.go:240-246`), its semantic-body comparison (`internal/store/resource_publish.go:358-399`) and its named
collision/corruption refusals (`internal/store/resource_publish.go:198-202`).

**Immutability is about modification, not deletion.** A blob's content is never
rewritten in place; a blob may be **removed** by an explicit purge (D16), which
leaves a tombstone rather than a lie in the index.

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
- `status.json.notes` remains a **last-transition hint**, not provenance: every
  transition overwrites it (`internal/store/store.go:380-393`), and the archive
  does not upgrade it into a durable authorship record.
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
  ADR-033 D10), and a per-entry `purged` tombstone flag (D16).
- `generation_id` is computed over the **pre-purge** body, so purging never
  changes an existing identifier and never invalidates an earlier record.
- **Every read of `index.json` is strictly decoded**: single value then EOF, no
  unknown fields, matching `schema_version` (a *higher* version refuses with an
  "upgrade tpatch" message rather than a corruption message), matching
  `feature`, no null arrays, well-formed unique ids, `blob == ""` iff `purged`,
  contained paths, closed `artifact_id`s, and a recomputed `generation_id`
  (PRD §9.3.1, X1–X10). A corrupt index causes a named refusal — never a
  rewrite, never a blob removal, never a canonical write.
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

**They are nonetheless full publication-set entries** with `preimage`,
`new_image` and the same CAS treatment as any artifact (D5). The raw file is
*where the bytes live*, not *whether the entry is modelled* — rev-0 conflated
the two and could not state a metadata rollback rule as a result.

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

### D13 — Recovery has three entry points; two are automatic-or-diagnostic, one is the operator's

- **Automatic**: every mutating `prepare` for that slug, under the lock, before
  any other work — the shipped shape at `internal/cli/land_journal.go:445-482`.
- **Operator**: `tpatch prepare <slug> --abandon-transaction`, which **moves**
  the journal, both metadata preimages and every staging tree into a gitignored
  `abandoned-<12hex>/` evidence directory and **touches no canonical file**. It
  is a preview without `--yes` (the shipped `session purge` gate,
  `internal/cli/session.go:353-419`), it never removes a lock, and it never
  restores or overwrites anything.
- **Diagnostic**: one new `doctor` check that *reports* residue and never acts
  on it, never creates the lock file and never changes `doctor`'s exit code.

No other command recovers, refuses, or blocks on a pending journal.
`--dry-run` refuses with `recovery-pending` rather than printing a plan it
cannot honour.

**Why an operator route is required.** Exit 6 exists for divergence — an undo
whose CAS refused, a journal that failed a bind, a post-publication mismatch.
Without an explicit route, the only escape from exit 6 is hand-deleting a
gitignored control file, which is undocumented, unguarded and performed under
stress. `--abandon-transaction` makes it explicit, previewable, evidence-
preserving and incapable of touching canonical bytes. **No population of exit 6
is permanent** (PRD PIB-362, PIB-363).

**What `doctor` must report — the total residue set.** Reporting only a pending
journal (rev-0) leaves an operator blind to everything else this design can
leave behind. D9 reports: a pending journal; a **live** transaction (via a
non-creating, non-blocking lock probe that is released immediately); an inert
lock file, explicitly as *informational, not a defect*; retained staging trees;
abandoned evidence directories; orphan archive blobs, with the exact purge
command; a corrupt archive index, with its failing bind code; and the
journal-loss symptom of D21. It repairs none of them.

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

**Its implementation is a hard prerequisite, not a companion.** The accepted
contract is planning, not code. No mutating slice may be dispatched until that
implementation has landed on `origin/main` and passes its own matrix, and the
goldens the compatibility rows compare against must come from **its** commit
range, not from this cluster (PRD §17.1, PIB-391).

### D15 — Redaction is a **precondition** for archiving, and a match refuses

ADR-027 D3 requires every tpatch-owned writer to run the redaction contract
before persisting any capture artifact, and makes redaction failure a hard
failure for committed artifacts
(`docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-177`). rev-0 did not
engage it. The archive is a tpatch-owned writer persisting bytes into the
tracked lane, so it engages fully:

1. Before any blob is written, each replaced artifact's prior bytes are scanned
   with the shipped closed six-class resource policy
   (`internal/redact/redact.go:53-72,81-90`).
2. **A match refuses the whole invocation** (exit 3,
   `archive-content-refused-sensitive`), before the first blob exists and
   before any canonical mutation.
3. The refusal names artifact ids and matched class codes — **never** matched
   bytes, excerpts or offsets.
4. There is **no override flag and no config key**. An override on a hard gate
   converts it into a suggestion.

**Why refuse rather than scrub.** The archive's only purpose is exact byte
recovery. Scrubbed bytes are not the bytes that were replaced, so a "recovery"
copy would silently differ from the original — a worse failure than refusing.
The two requirements genuinely conflict, and the conflict is resolved by
refusing the **destructive** operation, never by silently persisting the
secret. `redact.Scan` takes in-memory bytes and never a path, so "scan before
write" is structural rather than a call-site discipline
(`internal/redact/redact.go:19-22`).

**Accepted cost.** A false positive (most plausibly `home-absolute-path` inside
a legitimate example) blocks a regeneration until the operator edits the file.
That cost is chosen over the alternative: a credential retained in every future
clone.

### D16 — Retention is bounded: listing, purging, tombstones and orphans

A durable store with no removal route is a leak. The surface is deliberately
small and modelled on the shipped `session purge`:

- `tpatch feature intent-archive list <slug>` — read-only; every generation,
  entry, blob, presence flag and the orphan set.
- `tpatch feature intent-archive purge <slug> (--blob | --generation |
  --orphans | --all) [--yes]` — preview by default; **one scope selector is
  required** (the shipped lesson that "neither" must not mean "all",
  `internal/cli/session.go:369-375`); takes the same per-slug lock; refuses
  while a journal is pending, because a rollback may need the blob.

**Purge semantics:**

1. CAS the blob (present, bytes consistent with its name) before removing it.
2. Rewrite `index.json` **first**, tombstoning every referencing entry as
   `{"blob": "", "purged": true}` while keeping `artifact_id`, `path`,
   `size_bytes` and `generation_id`.
3. Then remove the blob file and fsync the directory.

The order is load-bearing: an index referencing a removed blob would be a lie;
a tombstoned index plus a still-present blob is a harmless orphan that
`purge --orphans` collects. A blob still referenced by a non-selected entry
refuses with `archive-blob-referenced` rather than breaking that reference.

**Orphans are reported and removable**, not "normal and permanent". They arise
from crashes between the blob write and the index rename, and from refusals
after a blob write. Every outcome that leaves one enumerates it, including the
rolled-back message — which therefore does **not** claim the whole tree is
byte-identical (rev-0's wording did).

**Recovery constraint, stated in every surface.** A purged blob is gone;
`list`, `doctor` and the purge report all say so in the same words. And a blob
that was already **committed** remains in Git history: purging removes it from
the working tree only, and removing it from history is a Git operation tpatch
does not perform. Claiming otherwise would be the most dangerous false
statement this cluster could make.

### D17 — The Git dependency is real, read-only, and conditional

rev-0 claimed `prepare` performs no Git operation. It was false: the local-lane
contract gate shells out and **refuses when Git is unavailable**
(`internal/workflow/session_ignore.go:138-145`).

**The exact set** — all read-only, none touching the index, a commit or
`.git/**`: `git rev-parse --is-inside-work-tree`
(`internal/gitutil/ignore.go:25-34`); `git check-ignore -q --no-index`
(`internal/gitutil/ignore.go:59-78`); `git --literal-pathspecs ls-files --
.tpatch/local/` (`internal/rescap/gitgate.go:104-115`); and, only to decide the
untracked advisory in `--regenerate`, `git ls-files -- .tpatch`
(`internal/cli/cobra.go:3405-3407`). A git-exec spy asserts the argv set is
exactly that subset (PRD PIB-107).

**Conditional application**, decided by a single existence `Lstat` of `.git`
through the held root:

| `.git` | `git` usable | Behavior |
|---|---|---|
| absent | — | gate inapplicable (nothing can be tracked or ignored); no Git command runs; advisory `workspace-not-git`; the command proceeds |
| present | yes | the shipped two-target gate runs unchanged |
| present | no | **refuse**, exit 3, `local-lane-unverifiable` — a repository exists, so the privacy question is real and unanswerable |

The third row is a disclosed regression against `define --manual` (PRD §12.6
D9). Making the gate optional there was rejected: it would let a mutating
command write into a lane that may be tracked, which is precisely what the
privacy boundary forbids. Path B keeps a complete route through the per-phase
`--manual` commands, and the refusal says so.

### D18 — `--regenerate` requires provider success; heuristic replacement is opt-in

Default mode keeps the shipped fallback: no provider, or a failed call, yields
heuristic output with an advisory. That is safe **because default mode only
ever creates absent files** (D19).

`--regenerate` does not. It replaces content a human may have authored, so:

- with no provider configured it **refuses before any mutation** (exit 3,
  `provider-required-for-regenerate`);
- on provider failure or deadline expiry it **refuses before publication**
  (exit 5), with zero blobs and zero canonical writes;
- `--allow-heuristic` is the **only** way to permit heuristic replacement, it is
  never implied, there is no config key for it, and its use is reported by a
  dedicated advisory.

**Deadlines are per-phase and total** (`--timeout-phase`, default 90s, clamped
to the remaining `--timeout`, default 180s). A total expiry that causes two or
more fallbacks emits a distinct cascade advisory, so one expiry can never be
silently reported as three independent provider failures. No deadline is
consulted after staging validation: a validated publication is never aborted
because a clock expired.

**Rationale.** rev-0 let one expired context replace three hand-authored
documents with boilerplate and exit 0. The archive makes that recoverable; it
does not make it acceptable, and "recoverable" is not the standard a
destructive default should be held to.

### D19 — The default mode generates only a dependency-coherent suffix, and never touches a sidecar

`spec.md` derives from `analysis.md` and `exploration.md` from both
(`internal/workflow/workflow.go:120-129,165-172`). The default mode may
therefore generate only a **suffix** of analysis → spec → exploration.
Generating an analysis underneath an existing spec would publish an analysis
the spec provably did not derive from, with nothing on disk recording the
mismatch — which rev-0 permitted.

- Admissible generated sets: `{}`, `{exploration}`, `{spec, exploration}`,
  `{analysis, spec, exploration}`.
- Every other gap refuses with `incoherent-bundle-gap` (exit 2) and a
  remediation naming `--regenerate` and `--manual`.
- The structured sidecar is **never written by the default mode when it is
  present in any state**, and is generated only when `analysis.md` is generated
  *and* the sidecar is absent. A present sidecar with an absent analysis is an
  incoherent gap, not an overwrite opportunity.

**Consequence: the default mode has no `replace` entry, ever**, so it writes no
blob, appends no generation and never creates the archive directory. The
archive semantics of every mode are therefore unambiguous, which rev-0's
sidecar rule was not.

### D20 — `FEATURES.md` is derived, best-effort, and outside the transaction

`SaveFeatureStatus` refreshes `FEATURES.md` and **swallows the error**, because
`status.json` is the source of truth and must land regardless
(`internal/store/store.go:363-377`, rebuild at `internal/store/store.go:703-712`). Every shipped state
transition already behaves this way.

It is therefore: not in the publication set; not journaled; not CAS-gated; not
rolled back; and explicitly outside T1. `prepare` additionally **reports** a
failed refresh as an advisory, and re-runs the same best-effort refresh as the
last act of any rollback or recovery that restored `status.json`, so the
derived index reconverges within the invocation where the filesystem permits.

**Rationale for not making it transactional.** CAS on an index that eleven
unlocked commands rewrite would convert ordinary concurrent activity into
exit-5 aborts on a *derived* file. The shipped design treats it as a cache;
this ADR matches that and says so instead of claiming `--manual` writes exactly
one file.

### D21 — T2's boundary: a lost journal is unrecoverable, and that is disclosed

The journal lives in the gitignored `.tpatch/local/` lane, so `git clean -xfd`,
a fresh clone, or an out-of-band delete destroys it. If that happens while a
transaction is pending, the preimages go with it and the old bytes are
unrecoverable by `tpatch` — except for `--regenerate`, whose blobs are in the
tracked tree.

This is a **stated boundary of T2**, not an oversight: `doctor` detects the
symptom and repairs nothing, because repair would require evidence that no
longer exists, and reconstructing it from artifact content is exactly the guess
D5's CAS rule forbids.

**Rejected: relocating the journal under the Git directory.** It would survive
`git clean` but not a fresh clone (the boundary moves rather than closes);
`.git` is Git's namespace and writing control state there is a compatibility
hazard across worktrees, submodules and the `.git`-as-a-file indirection; a
non-Git workspace would still need a second location, so the design would carry
two lane implementations and two crash-phase tables; and it would give `prepare`
a hard Git dependency in exactly the mode D17 is making conditional.

---

## Consequences

**Positive**

- `--regenerate` becomes a safe operation with a durable, Git-independent
  recovery route, and the safety is content-addressed, so repeated regeneration
  of identical content costs zero bytes.
- The transaction contract is stated in terms a reviewer can falsify: T0 is
  explicitly not claimed, T1 is scoped to a verification point the command
  owns, and T2 has enumerable crash phases plus a stated loss boundary.
- **No crash can permanently block a slug.** Lock ownership dies with the
  process, and every refusal has a named route out.
- **No third-party bytes are ever clobbered by recovery**: every undo is
  CAS-gated and refuses rather than overwriting.
- The pathname half of the publication race is closed by rooted writes; the
  residual is content identity and is disclosed as such.
- Retention is bounded and auditable: an operator can see what is archived,
  purge it, and be told plainly what purging cannot undo.
- Six shipped commands (`analyze`, `define`, `explore`, `cycle`, `next`,
  `feature resource *`) keep byte-identical behavior, pinned by goldens.
- The read boundary (ADR-034) and the write boundary (this ADR) stay separable,
  so a future change to one does not silently license the other.

**Negative / accepted costs**

- The tracked tree grows: one blob per distinct replaced content, capped
  individually by the accepted artifact byte cap, bounded only by explicit
  purging (D16).
- Committed blobs are in Git history; purging removes them from the working
  tree only, and this ADR says so rather than implying more.
- A redaction false positive blocks a regeneration until the operator edits the
  artifact (D15). That cost is chosen deliberately.
- **Mutating `prepare` supports fewer platforms than `prepare --check`**
  (`solaris`, `aix`, `js`, `plan9`, `wasip1` refuse), because a
  process-lifetime lock is a hard requirement (D4).
- **Mutating `prepare` refuses in a Git repository whose `git` is unusable**
  (D17), where the per-phase `--manual` commands succeed today.
- `--regenerate` fails where rev-0 would have silently produced heuristic
  output (D18); that is the point, but it is a real behavior cost for offline
  users, who must pass `--allow-heuristic`.
- The lock's authority is narrow, so a concurrent `define` can still lose a
  write inside the residual window (D6).
- Extraction touches `internal/workflow/workflow.go` and
  `internal/rescap/lock_unix.go`, both shared surfaces, which forces sequential
  implementer dispatch for the slices that touch them.
- Recovery is undo-only, so an interrupted prepare always costs a re-run of
  generation, including its provider calls.
- A journal lost with the gitignored lane is unrecoverable (D21).

---

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Claim multi-file atomicity | False; see D1. |
| Claim T1 as a property of the world after the command returns | Unownable by a command that excludes no other writer; scoped to a command-owned verification point instead (D1, D6). |
| Whole-directory copy-on-write and swap | Directory swap is not atomic, breaks open descriptors, and would move audit-trail files this command does not own. |
| `O_EXCL` lock file with nonce/inode ownership (rev-0, and `land`'s shape) | A file's existence is data, not liveness; CP1/CP2/CP8 and every `--manual` crash leave a lock with no journal to match, permanently refusing the slug (D4). |
| POSIX record locks (`fcntl`) to cover `solaris`/`aix` | Their close-any-descriptor release semantics are a well-known footgun in multi-threaded processes; refusing those targets is safer than a subtly broken lock (D4). |
| Redo-style journal, like `land` | Nothing here is irreversible; roll-forward adds failure modes for no benefit (D5). |
| A journal without plan binding | A forged or hand-edited journal could direct a restore over canonical content; `plan_digest` plus per-step CAS makes it inert (D5). |
| Path-based `os.Rename` after rooted checks (rev-0) | Leaves an ancestor-swap race that `(*os.Root).Rename` closes outright in the pinned toolchain (D2). |
| Lock acquisition in every writing command | Deadlock surface and a compatibility delta across eleven verbs, to close a window per-entry CAS already detects (D4, D6). |
| Ephemeral preimages only | Loses bytes on the success path — the path the requirement is about (D8). |
| Git as the preservation guarantee | `.tpatch/` is frequently untracked and fresh Path B work is uncommitted (D8). |
| Archive in `.tpatch/local/` | A guarantee that vanishes on clone is not a guarantee (D8). |
| Journal under the Git directory | Survives `git clean` but not a fresh clone, colonises Git's namespace, needs a second lane for non-Git workspaces, and imports a hard Git dependency (D21). |
| Archiving without a redaction scan | ADR-027 D3 makes redaction a precondition for every tpatch-owned writer, and a tracked archive is the worst place to retain a credential silently (D15). |
| Scrubbing secrets instead of refusing | A scrubbed blob is not the replaced bytes, so the recovery guarantee would silently become false (D15). |
| An override flag on the redaction gate | Converts a hard gate into a suggestion; deferred to PRD Q9 with a stated default of "no" (D15). |
| An immutable archive with no removal route (rev-0) | Immutable must mean "never modified in place", not "undeletable forever"; without D16 the design is a retention leak (D8, D16). |
| Orphan blobs as "normal and permanent" (rev-0) | Unbounded, unreportable tracked growth from failed attempts; they are now reported and removable (D16). |
| Heuristic fallback for `--regenerate` (rev-0) | One expired context could replace a hand-authored bundle with boilerplate and exit 0 (D18). |
| Claiming `prepare` runs no Git command (rev-0) | False: the lane gate shells out and refuses when Git is unavailable (D17). |
| Making the lane gate optional when Git is unusable | Would let a mutating command write into a possibly-tracked lane, violating the privacy boundary the gate enforces (D17). |
| Generating an analysis beneath an existing spec (rev-0's default mode) | Publishes an artifact the downstream artifact provably did not derive from, with nothing recording the mismatch (D19). |
| Synthesizing or overwriting a sidecar in the default mode | Fabricates structured data that does not derive from the preserved analysis, and is a false Path A signal (D19). |
| Bringing `FEATURES.md` into the publication set | CAS on a derived index that eleven unlocked commands rewrite would turn ordinary concurrency into exit-5 aborts (D20). |
| Letting `--dry-run` simulate the post-recovery plan | Requires deciding recovery's outcome without holding the lock or performing it — the guess D5 forbids (D13). |
| Pointer/symlink generation directories | The accepted inspector refuses symlinks, so every prepared feature would classify unsafe (D8, D13). |
| Provenance sub-record on `FeatureStatus` | Would fire the provenance trigger and answer a question this cluster is not authorized to answer (D9). |
| Workspace-level blob store with cross-feature dedupe | Couples feature directories, complicates `land`'s per-feature path set and makes feature removal ambiguous (PRD Q7). |
| Adding a field to `prepare --check` for the pending journal | Changes an accepted, frozen schema (D14). |

---

## Acceptance dependencies

Each decision is verified by named rows in the companion PRD's §18 matrix.
Every row cited here exists in that matrix (PRD PIB-229 asserts it
mechanically).

| Decision | Verified by (PRD §18) |
|---|---|
| D1 three guarantees; T0 not claimed; T1 scoped | PIB-103, PIB-155, PIB-206, PIB-314, PIB-315, PIB-324 |
| D2 rooted write boundary, no-follow, closed target list | PIB-148…PIB-151, PIB-154, PIB-182, PIB-183, PIB-308…PIB-313 |
| D3 single-file `--manual` publication set | PIB-051, PIB-052, PIB-257, PIB-258 |
| D4 process-lifetime lock, inert lock file, platform and filesystem envelope | PIB-053, PIB-080, PIB-117, PIB-124, PIB-125, PIB-284…PIB-295, PIB-392…PIB-394 |
| D5 undo-only recovery, plan binding, semantic CAS | PIB-113, PIB-116…PIB-123, PIB-126, PIB-164, PIB-275…PIB-278, PIB-296…PIB-307 |
| D6 disclosed residual content race | PIB-100…PIB-105, PIB-296, PIB-297, PIB-324 |
| D7 fixed publication order, status last | PIB-108, PIB-109, PIB-121, PIB-122 |
| D8 durable content-addressed archive, conditional tracking | PIB-060…PIB-068, PIB-071, PIB-152, PIB-153, PIB-330, PIB-342…PIB-344 |
| D9 archive is not provenance; trigger unfired | PIB-140…PIB-147, PIB-378, PIB-379 |
| D10 deterministic ids, tombstone field, strict index decoding | PIB-064, PIB-065, PIB-069, PIB-160…PIB-162, PIB-165, PIB-168, PIB-169, PIB-331…PIB-341 |
| D11 metadata preimages are raw files **and** modelled entries | PIB-119, PIB-122, PIB-164, PIB-277, PIB-278 |
| D12 pure generators, goldens, nil retry store | PIB-184…PIB-190, PIB-208…PIB-211, PIB-376 |
| D13 three recovery entry points; total doctor residue reporting | PIB-079, PIB-133…PIB-136, PIB-177, PIB-213, PIB-268…PIB-274, PIB-380…PIB-387 |
| D14 accepted check frozen; implementation is a prerequisite | PIB-198…PIB-207, PIB-391 |
| D15 redaction precondition; refusal, not scrubbing | PIB-262…PIB-267, PIB-388 |
| D16 bounded retention: list, purge, tombstones, orphans | PIB-316, PIB-317, PIB-345…PIB-361 |
| D17 exact, read-only, conditional Git usage | PIB-106, PIB-107, PIB-279…PIB-283, PIB-326…PIB-329 |
| D18 provider authority for `--regenerate`; deadlines | PIB-365, PIB-368…PIB-375 |
| D19 coherent-suffix default; sidecar never overwritten | PIB-245…PIB-256 |
| D20 `FEATURES.md` derived and outside T1 | PIB-049, PIB-257…PIB-261 |
| D21 journal-loss boundary; detected, not repaired | PIB-321…PIB-323 |

---

## References

- [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md) — companion (rev-1)
- [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md) — accepted prerequisite; its **implementation** is a dispatch precondition
- [ADR-034 rooted filesystem inspection boundary](./ADR-034-rooted-filesystem-inspection-boundary.md)
- [ADR-033 resource capture boundary](./ADR-033-resource-capture-boundary.md)
- [ADR-027 capture context privacy boundary](./ADR-027-capture-context-privacy-boundary.md) — D3 is normative for D15
- [PRD-feature-resource-claims-and-capture-adapters](../prds/PRD-feature-resource-claims-and-capture-adapters.md) — the shipped lock and content-addressed publication precedents
- [PRD-active-feature-session](../prds/PRD-active-feature-session.md) — the shipped purge grammar reused by D16
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md)
- `internal/rescap/lock_unix.go` — the shipped kernel-lock precedent extracted by D4
- `internal/redact/redact.go` — the shipped scanner D15 requires
- `internal/cli/land_journal.go` — the shipped journal and recovery precedent
- `internal/store/resource_publish.go` — the shipped content-addressed publication and semantic-CAS precedent
- `internal/store/store.go` — the shipped single-file atomic writer, deliberately **not** reused for publication (D2)
