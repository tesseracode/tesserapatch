# ADR-035 — Intent Bundle Publication and History

**Status**: Proposed — Awaiting Review (rev-3), 2026-08-14
**Date**: 2026-08-13 (Proposed rev-0), 2026-08-14 (rev-1, rev-2, rev-3)
**Owner**: Core (planning lane)
**Byline**: sole sequential planning writer, rev-3 from reviewed writer tip `faf055e`;
dispatch/base `155e721`; WAVE_BASE `d060ff4`
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
(rev-3, Draft — Awaiting Review). **The two documents must be reviewed
together.** Read the PRD for the full product contract and its 432-row
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
| rev-2 | Proposed — Awaiting Review | Corrects the live-lock unlink/recreate flaw by selecting a persistent external cache locator; defines rooted/CAS `--manual`; preserves tombstone identity and rehydrates duplicate generations; CAS-gates purge-index publication; limits mutation to Linux/Darwin/Windows; makes G1 authoritative; narrows rooted-write claims to outside-root escape; and withdraws unsupported journal-loss diagnosis. The companion matrix gains `PIB-395`…`PIB-409`. |
| rev-3 | Proposed — Awaiting Review | Removes the prohibited raw-provider local transcript lane and replaces the cache authority with the held workspace-root directory inode: `Root.Open(".")` + one Linux/Darwin nonblocking flock, workspace-wide scope, no lock/cache artifact and live-root identity revalidation. It makes purge CAS honest about the final syscall race; makes content-hash availability global for purge/rehydration; recognizes untracked archive deletion by Git clean; scrubs/counts G1–G4; and adds X11 dangling-reference storage truth. The companion matrix gains `PIB-410`…`PIB-432`. |

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

Every canonical read keeps ADR-034's accepted boundary. Every mutating write —
transactional publication, archive/index publication, the one-file `--manual`
status transition, and the best-effort derived-index refresh — uses a held
workspace `*os.Root` and a closed root-relative target list.

1. A target is derived only from the validated slug and fixed artifact IDs (or a
   validated content digest). Provider output, report text and artifact content
   never provide a pathname component.
2. A same-directory temp is created with rooted `OpenFile(O_CREATE|O_EXCL|O_WRONLY,
   0600)`, fsynced and closed; publication/removal uses rooted `Rename`/`Remove`;
   directory creation and directory sync use rooted operations. There is no
   `os.Rename`, `os.CreateTemp`, `os.WriteFile`, `filepath.Join(repoRoot, …)`,
   `writeFileAtomic`, or `SaveFeatureStatus` in a mutating prepare write path.
3. Immediately before every canonical/index rename, the command re-captures the
   destination through the held root and compares the exact identity expected
   by that step. A final symlink, junction, directory, FIFO, socket or device
   aborts; a concurrent identity mismatch aborts before rename.
4. New tracked files are `0644`, replacements preserve permission bits, and
   local journal/staging material remains `0700`/`0600`.

`os.Root` is a **root-escape** boundary, not a universal pathname-identity
proof. It prevents a relative resolution from leaving the held workspace root,
but follows relative symlinks that remain inside it. A raced ancestor can
therefore redirect a write to another in-root object. CAS catches an ordinary
redirect whose resolved identity differs from the expected preimage; it cannot
prove the canonical name never denoted an in-root alias with the identical
preimage. This residual is separate from the content write between CAS and
rename (D6). No document may call the pathname half "closed" or claim that
rooted writes eliminate all pathname redirection.

### D3 — A single-file publication takes no journal

`--manual` has one publication-set entry, `status.json`, and so requires no
undo journal or archive. It is **not** exempt from D2: under the held D4 lock it
captures a rooted status preimage, stages a same-directory rooted temp, and
immediately before `Root.Rename` CASes the live status identity against that
preimage. Mismatch refuses `status-changed` before a rename or derived-index
refresh. A crash leaves the single entry old or new, never torn.

The existing path-based `writeFileAtomic`/`SaveFeatureStatus` are not used. A
rooted best-effort `FEATURES.md` refresh follows only a successful status
rename, remains outside T1/D1's publication set, and does not reintroduce a
path-writer exception (D20). PIB-399 and PIB-400 prove rooted temp/rename and
concurrent-status preservation.

### D4 — The transaction lock is the held workspace-root **directory inode**

A mutating invocation opens one workspace `*os.Root`, retains it for every
rooted operation, opens `"."` through it, and takes one nonblocking
`syscall.Flock(LOCK_EX|LOCK_NB)` on that directory descriptor on Linux/Darwin.
The authority and the rooted-I/O root are the same inode. Success is authority;
only `EWOULDBLOCK`/`EAGAIN` is `transaction-in-progress` (exit 3); every other
flock result is `directory-flock-unavailable` (exit 3), fail-closed.

There is no file authority: no lock/cache file, key, per-user namespace,
HOME/XDG/LocalAppData access, cleanup or durable residue. The scope is the
workspace, not a slug: all mutating prepare modes and archive purge serialize
across every slug. Acquire once per invocation and pass it to nested mutators;
recursive acquisition is forbidden. Check, dry-run and archive list do not
acquire. The bounded hold is total generation deadline plus publication/recovery
I/O.

Mutation is Linux/Darwin only, with a root-descriptor local filesystem allowlist
(Linux ext4/XFS/Btrfs/tmpfs; Darwin APFS/HFS+). Other or unknown filesystems
refuse `lock-filesystem-unsupported` (exit 3) and remediation says to move the
workspace to a supported local filesystem. Windows/BSD/other targets refuse
`prepare-unsupported-platform`; ADR-034's read-only `unix || windows` contract
is unchanged. The authority does not promise cross-machine exclusion, resist a
malicious local fd-holder DoS, protect against external editors/Git, or survive
unmount/eject.

Native root identity comes from the held descriptor. Aliases and renames retain
that inode and lock. Re-resolving the live workspace-root pathname before the
publication window and final verification detects deletion/recreation: before
window `workspace-root-changed` exits 5; after publication begins
`workspace-root-replaced-after-publication` exits 6 with evidence preserved.
No exit 0 is permitted for an unlinked old root. Deletion/recreation is the
one split boundary.

Cache/user-cache, lock-file, `.git`, named-semaphore and POSIX `fcntl`
alternatives are rejected; fcntl's close-any-descriptor behavior is unsafe here.
A `rescap` file-lock extraction is not this implementation. S1b owns a
prepare-specific root-directory authority and leaves rescap byte-identical.
PIB-395…PIB-398 and PIB-410…PIB-418 require real-process alias, rename,
delete/recreate, death, contention and self-reentry coverage.

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
its own name; before a publication/rehydration index rename its identity must
equal the recorded entry preimage. Purge captures a strict index preimage and immediately before its rooted rename
re-reads/identity-compares it; mismatch is distinct `archive-purge-index-changed`
(exit 3) before any blob removal. It revalidates the live reference set and blob
identity/hash immediately before each removal, but this does not close the final
CAS→rename/remove syscall race. A mismatch on a publish aborts (exit 5, roll back);
a mismatch on an undo **refuses that entry** (exit 6, preserve). Two properties
follow:

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

### D6 — Root escape is closed; root replacement, content races and in-root aliases remain

D2 closes a rename that would resolve **outside** the held workspace root. It
does not prove every ancestor name continues to designate the same in-root
object: `os.Root` may follow an in-root relative symlink. Pre-rename CAS detects
a redirected target whose identity differs from the expected preimage, but an
in-root alias with the exact expected identity remains unobservable. That is a
name-to-object residual, not an escape and not a license to claim all pathname
redirection is closed.

Separately, a write landing between an entry CAS and its rename can be
overwritten without archival. Neither residual can be eliminated without every
writer taking a common lock or an unsupported content-checked rename. Writes
after final verification remain outside T1. PIB-310 covers outside-root escape;
PIB-406 covers the in-root redirect/CAS distinction.

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
archive survives crashes and re-runs only absent external deletion and does **not** survive a
fresh clone or `git clean -fd`/`git clean -xfd`, which can delete untracked `.tpatch/features/**`. No shipped string or document may claim otherwise.

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

Each archive replaced entry has immutable `content_sha256`, `path`,
`artifact_id`, and `size_bytes`. The blob name is that raw-byte SHA-256 plus
`.blob`. `generation_id` hashes the canonical sorted body of those immutable
fields plus feature/mode; it excludes the mutable storage fields `blob`,
`purged` and `purge_pending`.

`index.json` is schema version 1, fixed struct-key order, no map and no
wall-clock. A retained entry has `blob == content_sha256, purged == false,
purge_pending == false`; a removal-pending entry retains that locator with
`purge_pending == true`; a tombstone has `blob == "", purged == true,
purge_pending == false`. A tombstone is valid only when its named blob is
absent. Thus a purge has a truthful intermediate state rather than a tombstone
that claims absent bytes while the blob exists, and X10 remains computable from
the retained immutable digest.

An existing equal generation id is body-compared. A distinct body is a
collision refusal. The same body with all retained entries is an idempotent
no-op. Otherwise redaction scan and blob write/reuse precede **one**
CAS-published index rewrite that makes every reference in every generation with
that `content_sha256` retained; it neither appends a duplicate nor leaves a
false tombstone. Repeated purge/rehydrate and shared-blob references retain
this global-by-hash rule. Blob write precedes the rehydration index rename, so
a crash produces at most an orphan before rename, never a live reference to a
missing blob.

All reads strict-decode, bind X1–X10 including the immutable body digest, and
refuse before any rewrite/removal/canonical write. PIB-401…PIB-404 bind this
schema, rehydration, sharing/repetition, redaction and crash ordering.

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

Corollary: `RetryOptions.Store` is nil on the prepare path and there is no
response sink. ADR-027 D2/D3 forbids tpatch-managed raw provider transcripts
even in a local lane. A pure generator may hold an attempt response in memory
only long enough to validate/render, then discard it; persistent attempt data is
at most redacted/hashed ordinal/reason/sha256/size metadata. Intended canonical
staged output is a publication temporary, not transcript history.

**Rationale.** Calling the shipped functions would publish four to twelve
canonical files before the transaction decided anything — exactly what WP-005
Turn 3 rules out
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`). Extraction
without goldens would risk regressing four shipped commands, so the goldens are
a slice prerequisite, not a nicety.

### D13 — Recovery has three entry points; two are automatic-or-diagnostic, one is the operator's

Automatic recovery runs under D4's held directory authority before a mutating prepare;
`--abandon-transaction` moves journal/preimage/staging evidence only; D9 doctor
reports residue only and never mutates. No other command recovers or blocks.

D9 may report a present journal, allowed retained staging, abandoned evidence,
orphan blobs, corrupt index, and X11 dangling live references. It cannot observe
a directory flock without acquiring it and therefore reports no invented lock state. It must not infer that canonical mixed/partial bytes prove a lost
journal: ordinary analyze/define/manual lifecycle has the same shape. A clean
lane/archive after `git clean`, fresh clone, or external deletion is ordinarily
undetectable (D21). This preserves the abandon route without inventing recovery
evidence.

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

**Accepted cost.** A false positive — most plausibly `home-absolute-path` or the broad empirically relevant `email-pii` class in legitimate Markdown — blocks regeneration until the operator edits the file.
That cost is chosen over the alternative: a credential retained in every future
clone.

### D16 — Retention is bounded: listing, purging, tombstones and orphans

`list` is read-only. `purge` requires one selector, previews without `--yes`,
takes D4's workspace authority, and refuses while a journal is pending. X11
requires each retained/removal-pending reference to name a present regular
matching blob and each tombstone to have none; a dangling or
storage-inconsistent reference refuses and doctor/list name purge/tombstone or
safe identical-content rehydration as remediation.

`--blob h` selects every reference to h. A generation selection with a hash
shared by an unselected generation refuses `archive-blob-shared`, naming
`--blob h` or `--all`. Rehydration of h un-tombstones every reference to h in
one CAS rewrite. Thus a tombstone never lies while h.blob exists. "Not
recoverable" means only "until identical content is archived again."

Purge captures/strict-decodes index preimage, validates selection and blobs,
then immediately pre-rename revalidates the live index; mismatch is distinct
`archive-purge-index-changed` before removal. Its first rewrite marks selected
references removal-pending; after each validated removal its second rewrite
tombstones them. It revalidates the live reference set and blob identity/hash
immediately before every removal; `--orphans` also captures/revalidates index
and per-blob identity/hash. CAS then rename/remove is not atomic: external
edit/replacement in the final syscall window can still be overwritten/removed.
The tpatch lock excludes tpatch mutators only. This is stated for canonical and
purge paths, with detected pre-CAS and disclosed post-CAS injection rows
PIB-405, PIB-421/422 and PIB-429/430.

### D17 — The Git dependency is real, read-only, and conditional

G1, `git rev-parse --is-inside-work-tree`, is run from the discovered workspace
root and is authoritative. There is no `.git` `Lstat` presence test. Exact
`true` establishes a Git worktree (including linked worktrees/submodules) and
permits G2/G3/G4; a clean Git-established false/not-worktree result permits the
non-Git lane with `workspace-not-git`; missing executable, unexpected output or
exit, and any non-standard Git failure are `local-lane-unverifiable` before
mutation. G1 uses a C locale and a narrow known-not-worktree classifier so an
arbitrary Git error cannot fail open.

All G1–G4 invocations use C locale and scrub `GIT_DIR`, `GIT_WORK_TREE`,
`GIT_INDEX_FILE`, `GIT_COMMON_DIR`, `GIT_CEILING_DIRECTORIES`,
`GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`,
`GIT_DISCOVERY_ACROSS_FILESYSTEM`, `GIT_IMPLICIT_WORK_TREE`, `GIT_PREFIX` and
`GIT_SUPER_PREFIX`. Non-selection environment needed by the intentional ignore
behavior is preserved. G1 runs exactly once; G2/G3 accept the established Git
state through a seam and cannot call G1/IsGitAvailable again.
Foreign-`GIT_DIR` and call-count spies are mandatory. Git has no role in
keying, authority, preservation, or recovery. `bundle-untracked-in-git` exists
only when G1 established a worktree and G4 proved `.tpatch` untracked, and says
both clone and Git-clean loss risk. PIB-327…PIB-329, PIB-408 and PIB-427 bind it.

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

### D21 — T2's boundary: a lost journal is unrecoverable and usually undetectable

A journal removed while pending loses undo evidence. Tpatch does not reconstruct
it from canonical bytes. `git clean`, fresh clone and manual local-lane deletion
can all produce a clean no-journal state indistinguishable from a workspace that
never used prepare; doctor must not call normal partial lifecycle state journal
loss. It reports only present/evidence-bearing residue and never repairs it.
Archive blobs that remain are recovery material, not proof that loss occurred.
This undetectable boundary is disclosed in docs and guarded by PIB-321…PIB-323
and PIB-407.

## Consequences

**Positive**

- `--regenerate` becomes a guarded operation with recovery bytes while the archive remains present (or in committed Git history), and the safety is content-addressed, so repeated regeneration
  of identical content costs zero bytes.
- The transaction contract is stated in terms a reviewer can falsify: T0 is
  explicitly not claimed, T1 is scoped to a verification point the command
  owns, and T2 has enumerable crash phases plus a stated loss boundary.
- **No crash can permanently block a slug.** Lock ownership dies with the
  process, and every refusal has a named route out.
- **No third-party bytes are ever clobbered by recovery**: every undo is
  CAS-gated and refuses rather than overwriting.
- Rooted writes prevent outside-root escape. In-root relative-symlink redirection and the CAS→rename content race remain explicitly disclosed; neither is called a closed pathname half.
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
- **Mutating `prepare` supports fewer platforms than `prepare --check`**:
  only Linux and Darwin mutate; BSD, `solaris`, `aix`, `js`, `plan9`,
  and `wasip1` refuse because the lock authority is a hard requirement (D4).
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
- A journal lost with the gitignored lane is unrecoverable and, after clean
  lane/archive deletion, ordinarily undetectable (D21).

---

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Claim multi-file atomicity | False; see D1. |
| Claim T1 as a property of the world after the command returns | Unownable by a command that excludes no other writer; scoped to a command-owned verification point instead (D1, D6). |
| Whole-directory copy-on-write and swap | Directory swap is not atomic, breaks open descriptors, and would move audit-trail files this command does not own. |
| `O_EXCL` lock file with nonce/inode ownership (rev-0, and `land`'s shape) | A file's existence is data, not liveness; CP1/CP2/CP8 and every `--manual` crash leave a lock with no journal to match, permanently refusing the slug (D4). |
| POSIX record locks (`fcntl`) to cover other targets | Their close-any-descriptor release semantics are a multithreaded-process footgun; refusing unsupported targets is safer than a subtly broken lock (D4). |
| Cache/user-cache file, named semaphore, `.git` or a workspace lock file | Each introduces a pathname/namespace and lifecycle policy separate from the rooted workspace inode; D4 deliberately has none. |
| Redo-style journal, like `land` | Nothing here is irreversible; roll-forward adds failure modes for no benefit (D5). |
| A journal without plan binding | A forged or hand-edited journal could direct a restore over canonical content; `plan_digest` plus per-step CAS makes it inert (D5). |
| Path-based `os.Rename` after rooted checks (rev-0) | Leaves outside-root escape and is rejected. `(*os.Root).Rename` prevents that escape, but D2/D6 disclose that an in-root ancestor redirect remains possible. |
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
| D2 rooted writes, outside-root boundary, manual CAS and in-root residual | PIB-148…PIB-151, PIB-154, PIB-182, PIB-183, PIB-308…PIB-313, PIB-399, PIB-400, PIB-406 |
| D3 single-file rooted/CAS `--manual` publication set | PIB-051, PIB-052, PIB-257, PIB-258, PIB-399, PIB-400 |
| D4 held-root directory authority, root replacement boundary and platform envelope | PIB-053, PIB-080, PIB-117, PIB-124, PIB-125, PIB-279…PIB-295, PIB-395…PIB-398, PIB-409…PIB-418 |
| D5 undo-only recovery, plan binding, semantic CAS and purge residual disclosure | PIB-113, PIB-116…PIB-123, PIB-126, PIB-164, PIB-275…PIB-278, PIB-296…PIB-307, PIB-421, PIB-422, PIB-429, PIB-430 |
| D6 disclosed content and in-root name-to-object residuals | PIB-100…PIB-105, PIB-296, PIB-297, PIB-324, PIB-406 |
| D7 fixed publication order, status last | PIB-108, PIB-109, PIB-121, PIB-122 |
| D8 durable content-addressed archive, conditional tracking | PIB-060…PIB-068, PIB-071, PIB-152, PIB-153, PIB-330, PIB-342…PIB-344 |
| D9 archive is not provenance; trigger unfired | PIB-140…PIB-147, PIB-378, PIB-379 |
| D10 deterministic ids, immutable tombstone identity, X11 storage truth and global rehydration | PIB-064, PIB-065, PIB-069, PIB-160…PIB-162, PIB-165, PIB-168, PIB-169, PIB-331…PIB-341, PIB-401…PIB-404, PIB-423…PIB-425, PIB-428 |
| D11 metadata preimages are raw files **and** modelled entries | PIB-119, PIB-122, PIB-164, PIB-277, PIB-278 |
| D12 pure generators, goldens, nil retry store and no raw persistence | PIB-184…PIB-190, PIB-208…PIB-211, PIB-376, PIB-419, PIB-420 |
| D13 three recovery entry points; total doctor residue reporting | PIB-079, PIB-133…PIB-136, PIB-177, PIB-213, PIB-268…PIB-274, PIB-380…PIB-387 |
| D14 accepted check frozen; implementation is a prerequisite | PIB-198…PIB-207, PIB-391 |
| D15 redaction precondition; refusal, not scrubbing | PIB-262…PIB-267, PIB-388 |
| D16 bounded retention: list, purge, global tombstones/rehydration, orphans, dangling storage and honest CAS | PIB-316, PIB-317, PIB-345…PIB-361, PIB-405, PIB-421…PIB-430 |
| D17 exact, scrubbed, single-probe Git usage | PIB-106, PIB-107, PIB-279…PIB-283, PIB-326…PIB-329, PIB-408, PIB-427 |
| D18 provider authority for `--regenerate`; deadlines | PIB-365, PIB-368…PIB-375 |
| D19 coherent-suffix default; sidecar never overwritten | PIB-245…PIB-256 |
| D20 `FEATURES.md` derived and outside T1 | PIB-049, PIB-257…PIB-261 |
| D21 journal-loss boundary; evidence-only residue and clean-loss non-detectability | PIB-321…PIB-323, PIB-407 |

---

## References

- [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md) — companion (rev-3)
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
