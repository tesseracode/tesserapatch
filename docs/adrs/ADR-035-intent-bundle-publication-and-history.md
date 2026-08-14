# ADR-035 — Intent Bundle Publication and History

**Status**: Proposed — Awaiting Review (rev-10), 2026-08-14
**Date**: 2026-08-13 (Proposed rev-0), 2026-08-14 (rev-1 through rev-10)
**Owner**: Core (planning lane)
**Byline**: sole sequential planning writer, rev-10 from reviewed writer tip `ebd1be8`;
dispatch/base `2f8f697`; WAVE_BASE `d060ff4`
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
(rev-10, Draft — Awaiting Review). **The two documents must be reviewed
together.** Read the PRD for the full product contract and its 545-row
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
| rev-4 | Proposed — Awaiting Review | Adjudicates rev-3’s implementation contradictions without reopening the product choices. D4 retains a concrete root+directory-file authority through explicit `SyscallConn.Control` unlock/close and no claimed renamed-path rediscovery; its filesystem policy becomes deny-known-remote/user-space plus real flock, scoped to the locked root inode. D10/D16 define one per-hash pending → remove/recover → tombstone machine and repairable dangling references. D13 permits a non-creating, racy doctor flock probe. D17 becomes a closed Git argv contract with one threaded G1 state. D12’s raw-response protection is structural. The companion matrix gains `PIB-433`…`PIB-448`. |
| rev-5 | Proposed — Awaiting Review | Bounded reachability and totality adjudication of rev-4; no product choice reopened. D13 moves the `--abandon-transaction` branch after the lock and **before** both recovery passes, and removes the doctor flock probe entirely. D2/D5 route the journal, both raw preimages, staging and every `.tpatch/local/` write through a prepare-owned rooted durable single-file helper; `gitutil.DurableWriteFile` is a shape precedent only. D10/D16 make the confirmed global-hash purge the sole shipped dangling-reference repair, restrict rehydration to tombstoned/pending references, require a complete purge preflight and add the retryable `archive-purge-partial` outcome at exit 5. D4 classifies with `fstatfs` on the already-held root descriptor, pins the exact denied magic/name lists, discloses the lying-filesystem and cross-machine limits, and states the `SyscallConn.Control`-versus-release ownership rule. D17 adds `internal/rescap/gitgate.go` to the authorized central-gate refactor, pins the environment scrub including the indexed `GIT_CONFIG_*` forms, and deliberately preserves global/system ignore configuration. D12's provenance guard is schema-scoped rather than byte-scoped. The companion matrix gains `PIB-449`…`PIB-482`. |
| rev-6 | Proposed — Awaiting Review | Final bounded ordering/grammar adjudication of rev-5; no product choice reopened. **D13 makes recovery terminal**: a successful journal or pending-purge recovery ends the invocation at exit 0 `recovered` with a sanitized retry and never continues into the requested operation's exit-2/exit-3 gates, which is what lets exit 3's zero-write meaning stand unqualified. **D13/D17 make abandon reachable through a broken Git**: the mode still requires the D4 authority but bypasses G1–G4 and the local-lane gate, because it only renames existing bytes inside the same lane; where platform, filesystem or `flock` denies the mode outright, the refusal names the repo-relative lane and a last-resort manual removal, and every “never terminal” claim is qualified by that route. **D16 stops purge from touching a prepare journal**: `intent-archive purge` refuses `recovery-pending` without decoding, moving or consuming it, recovers only pending purge state, and that recovery is terminal too; its retry, and every other retry, is the equivalent command without root-selection argv plus `retry_cwd: "workspace-root"`. **D4 classifies inside `SyscallConn().Control`**: `fstatfs`, `flock` and the unlock all run in a `Control` callback on the retained directory file, and `(*os.File).Fd()` appears nowhere in the authority path. The companion matrix gains `PIB-483`…`PIB-505`. |
| rev-7 | Proposed — Awaiting Review | Bounded totality fold of rev-6; no product choice reopened. **D13 partitions the exit-6 remediation by population**: the nine journal/publication codes keep the abandon route, and `archive-purge-evidence-divergent` — which abandon provably cannot consume, since that mode touches no index and no blob — is routed to D16's own repo-relative archive procedure and is forbidden from naming abandon. **D13 replaces rev-6's “three environmental conditions” with a total pre-abandon gate**: parse/flag, slug, workspace, read platform, mutating platform, root filesystem, contention and lock failure, each with the one route it may offer; contention offers wait-and-retry only, because a live holder's undo journal must not be deleted under it; and the mode is confirmed to branch without reading the feature directory or `status.json`, so an absent, malformed or unreadable feature cannot block it. **D16 makes the partial-purge resumption conditional**: a pending hash means one `recovered` run then one completing run, while a failure between hashes and an `--orphans` stop each take a single completing run and promise no `recovered` outcome; a closed `resume` discriminator carries the branch. **D16 defines the pending-purge preview**: with no journal and a pending hash it takes no lock, writes nothing, recovers nothing, and names the `--yes` rerun. **D17 scopes G1 to normal mutating `prepare` and gives `intent-archive purge` a zero-Git boundary**, because purge writes only the tracked archive and never the gitignored lane; the lane-gate parity row is amended to match. The companion matrix gains `PIB-506`…`PIB-520`. |
| rev-8 | Proposed — Awaiting Review | Final bounded archive-state correction of rev-7; no product choice reopened. **D10 classifies a tombstone whose named blob is present as unreferenced physical residue, not purge divergence**: no pending reference exists, so nothing can be finalized and D16's pending+absent escape is unexecutable on it. X11 refuses exit 3 `archive-index-storage-inconsistent`, writing nothing, and the one shipped repair is `purge <slug> --orphans --yes`, explicitly admitted past that observation after the strict X1–X10 wire decode; it removes the unreferenced blob and rewrites no index, and the preview reports it. **D13 gives the pending-purge transaction exactly one owner**: `RecoverPendingPurge` has a single call site, on `intent-archive purge --yes`. A mutating `prepare` that observes `purge_pending` refuses exit 3 `recovery-pending` with zero writes and names the sanitized purge command with `retry_cwd`, so `archive-purge-partial` and `archive-purge-evidence-divergent` have one command that can both produce and resolve them; “after the first per-hash mutation” is scoped to the transaction in flight, which may have begun in a prior invocation. **D16 makes recovery and preview total over selectors**: every `--yes` selector including `--orphans` finalizes pending hashes first and returns terminal `recovered`, every preview selector takes no lock, writes nothing and returns the closed outcome `recovery-required` with a bound `pending_purge` object, and every emitted retry preserves the operator's own selector. **D13's pre-abandon gate is total over reachable stops only**, dropping rev-7's intentionally unreachable `--yes`-validation row. **D4's never-acquire population names the purge preview explicitly**; only `purge --yes` acquires, and purge stays zero-Git in both forms. The companion matrix gains `PIB-521`…`PIB-530`. |
| rev-9 | Proposed — Awaiting Review | Final bounded global-hash and parity fold of rev-8; no product choice reopened. **D10 classifies a tombstone beside a present blob globally, not per reference**: X11 first computes whether the hash is still live — any reference to it retained or removal-pending, in any generation. Unreferenced ⇒ unreferenced physical residue, `list` exit 0 rendering `orphan`, repaired by `purge <slug> --orphans --yes`. Still live ⇒ a **mixed tombstone/live-reference global-availability inconsistency**: the blob is live storage, `--orphans` excludes it and may never remove it, `list` exits 3 rendering `mixed-reference`, and the repair is the confirmed `purge <slug> --blob <h> --yes`, which selects every reference to the hash and runs the ordinary pending/removal/tombstone machine. rev-8's per-reference rule would have deleted a retained reference's blob through the one selector that promises to remove nothing referenced. X11's refusal map becomes a total map over (wire state × blob observation × global liveness). **D16 defines `--orphans` globally by hash** and fixes the compound pending-plus-residue ordering: preview without a lock, terminal recovery on `--yes`, then the rerun performs the correct subcase repair, with no dead end. **D16 collapses the retry heading to one verbatim sentence** — `Run this again from the same workspace root:` — over every emitter, withdrawing rev-8's second spelling and the numbered divergence step; PIB-498 becomes the derived totality guard. **D13's pre-abandon gate table is total over every argv that attempts to select the mode**, parsing or not, which makes its parse/arity/mutex row reachable and removes rev-8's "syntactically valid" contradiction. **D4 classifies `os.OpenRoot` and `Root.Open(".")` failures** as `directory-flock-unavailable`, exit 3, before any `fstatfs` or `flock`, carried by the gate's authority row. The companion matrix gains `PIB-531`…`PIB-536` and re-kinds `PIB-498`. |
| rev-10 | Proposed — Awaiting Review | Final bounded ownership/scope fold of rev-9; no product choice reopened. **D10 gives a removal-pending reference global ownership of its content hash.** rev-9 classified globally but still acted per reference, so D16's recovery could remove `h.blob` while another generation's reference to `h` stayed retained — the recovery manufacturing the dangling reference the design exists to prevent. Rev-10 states the invariant instead: *if any reference to `h` is removal-pending, the purge transaction owns `h` globally*; ownership outranks every other classification of that hash, every reader but `purge --yes` routes to `recovery-pending`, and **`h.blob` is removed only when the index, revalidated immediately before the removal, holds no reference to `h` that is not removal-pending**. D16's machine therefore claims `h` globally first — one CAS rewrite making every reference to `h` removal-pending, taken only while the blob is present and hash-correct — then revalidates, then removes, then tombstones every reference; where the blob is already absent it publishes straight to tombstones, which is the shipped dangling repair. X7 is explicitly per-reference so the claim's transient is not a decode error, and the four external same-hash insertion windows are modelled with their CAS residuals rather than assumed away. **D16 makes X11's observation global and selector-independent**: the whole index is validated before every archive mutation, a mixed or corrupt hash refuses an unrelated `--orphans`/`--blob`/`--generation`/`--all` at exit 3 zero-write naming the owning repair, and each admitted repair is admitted only for the exact inconsistency it repairs after global classification — no partial cleanup around a lying index. **D10 classifies a present but non-regular or hash-wrong blob under a *retained* reference** as `archive-blob-corrupt`, exit 3, zero-write, with one repo-relative route that ends in an already-admitted repair: preserve, remove the managed blob path, then the confirmed `purge --blob <h> --yes` that tombstones every now-dangling reference — or restore the exact correct blob. **D13's pre-abandon table domain is every argv requesting a *true* abandon, or failing parse while naming the flag**; `--abandon-transaction=false` selects `generate` and is outside it. **D16's one retry heading is a column-0 literal**, alone on its line with no leading or trailing whitespace. `list` and `doctor` render every safe observation and take the highest exit when observations of different exits coexist. The companion matrix gains `PIB-537`…`PIB-545` and re-kinds nothing. |

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
   perm)`, fsynced and closed; publication/removal uses rooted `Rename`/`Remove`;
   directory creation and directory sync use rooted operations. There is no
   `os.Rename`, `os.CreateTemp`, `os.WriteFile`, `filepath.Join(repoRoot, …)`,
   `writeFileAtomic`, `SaveFeatureStatus`, or `gitutil.DurableWriteFile` in a
   mutating prepare write path.
3. **One rooted durable single-file helper carries every control write.** The
   journal, both raw metadata preimages (D11), each staged canonical output,
   the `--manual` status publication, the `abandoned-*` evidence moves and every
   other `.tpatch/local/` write use the same helper: rooted directory chain,
   rooted `O_CREATE|O_EXCL` temp in the destination directory, write, `Sync`,
   `Close`, the step's identity CAS where one is defined, rooted `Rename`,
   rooted parent-directory `Sync`. It accepts the workspace authority and
   root-relative names only, so a path-based control write is not expressible.
   `gitutil.DurableWriteFile` is the **shape precedent** this copies and remains
   the writer `land`'s journal uses; it is never called by `prepare`, because it
   takes a directory pathname and `os.CreateTemp`s inside it. **The gitignored
   local lane gets no carve-out from this rule**: it lives inside the same held
   root, is reached through the same ancestor chain, and holds the undo evidence
   for a rooted publication (PRD §7.5, §7.7.1; PIB-454…PIB-456).
4. Immediately before every canonical/index rename, the command re-captures the
   destination through the held root and compares the exact identity expected
   by that step. A final symlink, junction, directory, FIFO, socket or device
   aborts; a concurrent identity mismatch aborts before rename.
5. New tracked files are `0644`, replacements preserve permission bits, and
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

A mutating invocation creates one explicit authority holding both the workspace
`*os.Root` and the directory `*os.File` returned by `Root.Open(".")`. It keeps
both pointers strongly reachable for every rooted operation; a naked integer fd
is never authority. On Linux/Darwin, `lockFile.SyscallConn().Control` performs
exactly one nonblocking `syscall.Flock(LOCK_EX|LOCK_NB)` while the descriptor is
held. The authority and rooted-I/O root are the same inode. Success is
authority; only `EWOULDBLOCK`/`EAGAIN` is `transaction-in-progress` (exit 3);
every other flock result is `directory-flock-unavailable` (exit 3), fail-closed.

**Failing to obtain the handles takes the same refusal.** `os.OpenRoot` on the
workspace root and `Root.Open(".")` both precede the classification and the
lock, and either can fail — the root was renamed or removed after discovery, a
permission or descriptor-limit denies it, the path resolved to a non-directory.
rev-8 left those unclassified, which is the one way this fail-closed ladder
could still surface an unnamed error. They are `directory-flock-unavailable`
(exit 3) before any `fstatfs` or `flock` is attempted: the authority could not
be established, no lock was taken and no holder is implied — the exact meaning
that code already carries. It is deliberately not `transaction-in-progress`
(nothing was observed held), not `lock-filesystem-unsupported` (nothing was
classified) and not `workspace-not-initialized` (discovery already succeeded).
Because no holder is implied, D13's pre-abandon gate carries it in the same row
as the non-contention `flock` failures, with the repo-relative lane and the
last-resort manual removal where evidence exists (PRD §7.4.1, §6.6, PIB-536).

There is no file authority: no lock/cache file, key, per-user namespace,
HOME/XDG/LocalAppData access, cleanup or durable residue. The scope is the
workspace, not a slug: all mutating prepare modes and `archive purge --yes`
serialize
across every slug. Acquire once per invocation and pass the authority to nested
mutators; recursive acquisition is forbidden. The never-acquire population is
closed: check, dry-run, archive list, **every archive `purge` preview — that is,
`purge` on any selector without `--yes`** — and
`doctor` do not acquire — **no diagnostic command opens or locks the workspace
root at all** (D13, D16). Only `purge --yes` takes the authority, so the
confirmation flag and not the verb is what serializes. Explicit release at
invocation end unlocks through
`SyscallConn().Control`, closes the retained directory `*os.File`, closes the
root and calls `runtime.KeepAlive(authority)` at that boundary. No finalizer may
release a live authority. Provider contexts have deadlines; filesystem, Git,
recovery, publication and release have no hard wall-clock bound.

**`Control` versus release is resolved by ownership, not by locking.**
`Control` holds a descriptor reference for its callback, so a `Close` cannot
pull the fd out mid-`flock`; it does not order `Control` against a concurrent
`Close`, which returns `ErrClosed`/`ErrFileClosing` instead of running the
callback. The accepted rule is that the authority is owned by the single
invocation goroutine — acquisition, every threaded use and the one release
happen on it — so the racing case is only reachable through a bug. A `Control`
error after release is a programming error and is never evidence about the
lock: it may not be reported as contention, as lock loss, or as acquisition,
and fails closed as `directory-flock-unavailable` where reachable (PIB-481).

Mutation is Linux/Darwin only. **Classification runs `fstatfs` on the
already-held root-directory descriptor, inside a `SyscallConn().Control`
callback on that same retained `*os.File`**, after the root and `"."` are open
and before the flock attempt, so the classified object and the locked object are
the same inode by construction; a path-based `statfs` before opening would leave
a rename window between the check and the lock. The `Control` requirement is not
decoration: `(*os.File).Fd()` returns a bare integer holding no reference to the
file, which is the naked-descriptor hazard this decision already rejects for the
`flock` on that file, so **every syscall in the authority path — `fstatfs`,
`flock` and the unlock — runs inside a `Control` callback, and `Fd()` appears
nowhere in it** (PIB-504). Linux denies exactly the magic
numbers `0x6969` (`NFS_SUPER_MAGIC`, shared by NFS and NFSv4), `0x517B`
(`SMB_SUPER_MAGIC`, legacy smbfs), `0xFF534D42` (`CIFS_MAGIC_NUMBER`),
`0xFE534D42` (`SMB2_MAGIC_NUMBER`) and `0x65735546` (`FUSE_SUPER_MAGIC`, shared
by `fuse` and `fuseblk`), by exact numeric equality. Darwin denies exactly the
kernel-assigned `f_fstypename` values `nfs`, `smbfs`, `webdav`, `macfuse` and
`osxfuse`, by exact string equality on the NUL-trimmed 15-byte name — no
prefix, suffix or substring rule is applied, so a third-party FUSE filesystem
reporting its own name is not denied by pattern. It does not deny overlayfs,
exFAT, ZFS, f2fs or an unknown local class solely because that class is absent
from an old allowlist. After a non-denied class, successful real flock remains
required; classification or non-contention flock errors fail closed. The
classification concerns only the filesystem containing the locked root inode,
not nested mounts or every later write target.

**Two limits are disclosed rather than softened.** An unrecognized local
filesystem can accept `LOCK_EX|LOCK_NB` and provide no real exclusion, so a
successful flock is evidence the kernel accepted the call, not proof a second
process is excluded; and `flock` is per-host, so **no cross-machine guarantee
follows** for a shared filesystem. Windows/BSD/other targets refuse
`prepare-unsupported-platform`; ADR-034's read-only `unix || windows` contract
is unchanged (PIB-478…PIB-480).

Native root identity comes from the held directory file. Alias paths acquired
before a rename retain that inode and contend. The originally discovered
pathname cannot be portably rediscovered after a rename: the held root protects
the old inode but does not disclose its moved pathname. Re-resolving only that
original pathname before the publication window and final verification therefore
refuses a missing/different identity as `workspace-root-changed` (exit 5), or
after publication as `workspace-root-replaced-after-publication` (exit 6) with
evidence preserved. No exit 0 is permitted for an unlinked or moved old root.

Cache/user-cache, lock-file, `.git`, named-semaphore and POSIX `fcntl`
alternatives are rejected; fcntl's close-any-descriptor behavior is unsafe here.
`rescap` is a file-lock precedent, not an extraction or reuse — no statement in
this ADR or the companion PRD, outside a clearly historical revision-history
entry, says otherwise. S1b owns a
prepare-specific root-directory authority and preserves rescap behavior and
goldens. PIB-395…PIB-398, PIB-410…PIB-418, PIB-433…PIB-442 and
PIB-478…PIB-482 require
real-process GC, alias, rename-refusal, delete/recreate, death, contention,
self-reentry, `fstatfs`-ordering, denied-class, release-versus-`Control` and
Linux/macOS release coverage.

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

**Every non-divergent row ends the invocation.** Recovery is not a preamble to
the requested operation: when it acts, the command returns exit 0 with
`outcome: "recovered"` and the sanitized retry, and the operator re-runs. D13
states why.

The "complete" row is why evidence beats phase: a crash between the final
rename and the journal clear is indistinguishable from a completed transaction
by any marker that could itself be interrupted.

**The journal is written with the rooted control helper of D2, not with
`gitutil.DurableWriteFile`.** The shipped helper is the durability shape this
copies and remains correct for `land`; it is path-based, so using it would put
the undo evidence for a rooted publication outside the rooted boundary.

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
purge_pending == false`. Per hash, pending means exactly one of: a present,
regular hash-correct blob still needs removal; an absent blob has been removed
and only tombstone publication remains; or a wrong/unsafe blob refuses with
evidence preserved. A tombstone is valid only when its named blob is absent.
Thus a purge has one truthful, recoverable intermediate state rather than a
tombstone that claims absent bytes while the blob exists, and X10 remains
computable from the retained immutable digest.

**A removal-pending reference makes the purge transaction the global owner of
its content hash, and that is the invariant every other archive rule rests on.**
Three predicates are computed over **every** reference to `h` in **every**
generation: `h` is **live** when at least one reference to it is retained or
removal-pending, **unreferenced** otherwise, and **owned** when at least one
reference to it is removal-pending. Ownership has three consequences:

1. One transaction decides an owned hash's next state. `RecoverPendingPurge`,
   whose single call site is `feature intent-archive purge --yes` (D13), is the
   only code that advances it; nothing else may remove its blob, tombstone,
   un-tombstone or re-select it.
2. Ownership **outranks** every other classification of that hash. A retained,
   tombstoned or corrupt observation on an owned hash is reported as pending and
   routed to the owner — a mutating `prepare` refuses exit 3
   `recovery-pending`, a `purge` preview reports `recovery-required` and takes
   no lock, `list` renders the pending state for every reference to the hash at
   exit 0, and `doctor` reports it without acting.
3. **`blobs/<h>.blob` is removed only when the index, re-read and revalidated
   immediately before the removal, holds no reference to `h` that is not
   removal-pending.** rev-9 acted on the pending references alone, so an index
   with `G1` retaining `h` and `G2`'s reference to `h` pending would have had
   the blob removed and `G1` left retained — a dangling reference manufactured
   by the recovery itself, which rev-9 then routed to the dangling repair. A
   mechanism that repairs the archive must not produce the states it repairs, so
   rev-10 removes the composition rather than documenting it: D16's machine
   **claims** the hash globally before it removes anything.

X7 remains deliberately **per reference**: references to one hash that disagree
are the transient of a transaction in flight, not a wire-shape error, and a
stricter X7 would refuse the exact index the recovery exists to normalize.

**A tombstone whose named blob is present is never purge divergence, and which
repair applies is decided globally over the content hash.** rev-7 filed that
observation with D16's pending-hash divergence population and offered it that
population's escape — name the pending hash, remove the divergent blob, rerun so
the pending+absent case finalizes the tombstone. There is no pending reference
in this state, so there is no hash to name and nothing for the rerun to
finalize; the escape is unexecutable by construction. rev-8 corrected that but
evaluated the observation **per reference**, which rev-9 corrects in turn:
`content_sha256` is global, several generations may reference one hash, and one
of them being tombstoned says nothing about whether the bytes are still live for
another. Two derived predicates decide it, both computed over every reference to
`h` in every generation: `h` is **live** when at least one reference to it is
retained or removal-pending, and **unreferenced** otherwise; and the split is
taken only after **ownership** has been ruled out, since an owned hash is routed
to its transaction rather than classified here. The decision is
therefore a split, and in both halves X11 refuses exit 3
`archive-index-storage-inconsistent` **writing nothing** for every ordinary
archive or canonical mutation:

- **`h` unreferenced** — the file is the same object D16 calls an **orphan**,
  observed through the index rather than through a directory scan, and it is
  exactly what a crash in the rehydration window (blob durable, index rename not
  yet landed) leaves behind. `list` exits 0 rendering it as `orphan`, `doctor`
  reports it, and the one shipped repair is the confirmed
  `purge <slug> --orphans --yes`, explicitly admitted past that X11 observation
  **after** the strict X1–X10 wire decode has passed, which validates the file
  as a regular hash-correct blob, removes it, and rewrites no index.
- **`h` live** — the blob is the live reference's storage, so it is **not** an
  orphan, `--orphans` must exclude it, and no `--orphans --yes` invocation may
  remove it. Calling it an orphan would delete a retained reference's bytes and
  convert that reference into a dangling one — destroying recoverable data
  through the one selector whose whole promise is that it removes nothing
  anybody references. The state is a **mixed tombstone/live-reference
  global-availability inconsistency**: `list` exits 3 rendering
  `mixed-reference`, `doctor` reports it, and the one shipped repair is the
  ordinary confirmed global-hash purge `purge <slug> --blob <h> --yes`, admitted
  past X11 only for a confirmed selection total over `h`. It selects every
  reference to `h` (D16), claims them all, removes the blob
  and tombstones them all, which makes the index's global claim true. The
  opposite repair — un-tombstoning to agree with the live reference — is
  rejected: it would silently reverse a confirmed destructive decision on the
  strength of an index edit tpatch did not make.

**A present but unidentifiable blob under a *retained* reference is
`archive-blob-corrupt`, exit 3, and it has one executable route.** A
non-regular file, or one whose bytes do not hash to its name, beneath a
retained reference is neither dangling (the file exists, and tombstoning a live
reference against bytes tpatch never validated would destroy unseen evidence)
nor an orphan (the hash is live, so `--orphans` may never select it). It refuses
zero-write for `list`, `doctor` and every ordinary mutation, and its route is
repo-relative and **total** because it terminates in an already-admitted repair:
preserve the unexpected bytes if wanted, remove the managed blob path — after
which the reference is dangling — then run the confirmed
`purge <slug> --blob <h> --yes` that D16 already admits for a dangling hash, or
restore the exact correct blob and retry. The destructive cost and the
Git-history caveat are stated with it, and the `cp`/`rm` pair is the same
permitted external shape D16's divergence procedure uses, not a fourth one
(PRD §9.3.1, §9.7.3, §10.7, PIB-428, PIB-543).

A non-regular or hash-wrong file at a path whose hash is **unreferenced** is not an orphan either
and keeps its `archive-blob-corrupt` exit-3 refusal. Neither tombstone-beside-blob half is ever
`archive-purge-evidence-divergent` and neither is ever routed to abandon, and
the same is true of the retained corrupt-blob observation
(PRD §9.3, §9.3.1, §9.7.3, PIB-521…PIB-524, PIB-531…PIB-533, PIB-535, PIB-543).

An existing equal generation id is body-compared. A distinct body is a
collision refusal. The same body with all retained entries is an idempotent
no-op. Otherwise redaction scan and blob write/reuse precede **one**
CAS-published index rewrite that makes every **tombstoned or removal-pending**
reference in every generation with
that `content_sha256` retained; it neither appends a duplicate nor leaves a
false tombstone. Repeated purge/rehydrate and shared-blob references retain
this global-by-hash rule. Blob write precedes the rehydration index rename, so
a crash produces at most a blob beside its tombstone before rename — the residue
classified above, cleared by `--orphans --yes` when its hash is unreferenced and
by the confirmed `--blob <h> --yes` when another reference keeps it live — never
a live reference to a missing blob.

All reads strict-decode and bind X1–X10 including the immutable body digest.
X11 accepts the two valid pending storage observations above, and its
observation is over the **whole index**, never over a selection (D16). Its
refusal
classification is a **total map** over (wire state × blob observation × the
hash's global **ownership** × its global **liveness**), with one route each: any
tuple whose hash is **owned** → `recovery-pending`, routed to
`feature intent-archive purge … --yes`, for every reader but that command;
retained/absent →
`archive-blob-dangling`, repaired by the confirmed
`--blob <h> --yes`; retained/present-non-regular-or-hash-wrong →
`archive-blob-corrupt`, routed to the preserve/remove-then-confirmed-purge or
restore procedure above;
pending/unsafe-or-wrong → `archive-index-storage-inconsistent`,
routed to D16's archive procedure; tombstoned/present with `h` **unreferenced** →
`archive-index-storage-inconsistent`, repaired by `--orphans --yes`;
tombstoned/present with `h` **live but not owned** →
`archive-index-storage-inconsistent`,
repaired by the confirmed `--blob <h> --yes` and never by `--orphans`. No tuple
maps to zero routes or to two (PIB-524, PIB-545). A missing
retained blob is a repairable dangling live reference, and its repair is
**singular**: the confirmed global-hash purge of D16, which tombstones every
reference to that hash after confirming the blob is still absent and performs
no removal. Rehydration is **not** a dangling repair — X11 refuses every
ordinary mutation while the dangling reference exists, so no run can reach the
point of producing the replacement bytes, which is why rev-4's second remedy
was unreachable. After the purge repair, an ordinary later regeneration that
reproduces the tombstoned generation rehydrates it through the path above.
PIB-401…PIB-404, PIB-443…PIB-444, PIB-457…PIB-460 and PIB-539…PIB-545 bind this
schema,
recovery, rehydration, sharing/repetition, redaction and crash ordering.

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
retry/raw-attempt or transcript sink. ADR-027 D2/D3 forbids tpatch-managed raw
provider transcripts even in a local lane. A pure generator may hold an attempt
response in memory only long enough to validate/render, then discard it;
persistent attempt data is at most redacted/hashed ordinal/reason/sha256/size
metadata. Intended canonical staged output is a publication temporary, not
transcript history, and may happen to equal the provider response; guards are
structural/path/sink based, never impossible provider-byte substring scans.

The same feasibility rule governs the provenance guard of D9. It walks the
**declared keys** of every wire struct this cluster writes — the mutating
report, `status.json`, the sidecar, `index.json`, the journal — and the
enumerated persistence sinks, asserting that no `generator`-class key is
declared or persisted. It does **not** scan `.tpatch/` bytes for the token
`generator`: canonical artifacts and provider prose are free text and may
contain any word, so a byte scan would be both unsatisfiable and useless. A
fixture with the word in canonical prose must pass; a fixture adding the key to
a persisted schema must fail (PIB-144, PIB-477).

**Rationale.** Calling the shipped functions would publish four to twelve
canonical files before the transaction decided anything — exactly what WP-005
Turn 3 rules out
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`). Extraction
without goldens would risk regressing four shipped commands, so the goldens are
a slice prerequisite, not a nicety.

### D13 — Recovery has three entry points, it is terminal, the operator's runs *instead of* the automatic ones, and the diagnostic touches nothing

Automatic journal recovery runs under D4's held authority in every mutating
`prepare` except abandon. Pending **archive-hash** recovery is not a `prepare`
pass at all: it belongs to `intent-archive purge --yes` alone (D16). D9 never
mutates. No other command recovers or blocks.

**The pending-purge transaction has exactly one owner.** rev-6 and rev-7 let
both a mutating `prepare` and `intent-archive purge --yes` run the per-hash
state machine. Both terminated afterwards, so exit 3's zero-write meaning was
safe; the defect was elsewhere. `archive-purge-partial` and
`archive-purge-evidence-divergent` are *discovered by* that machine, so a second
owner meant `prepare` could emit codes whose entire remediation, exit-window
scoping and route belong to `purge` — and the archive procedure would have had
to be duplicated into `prepare`'s output and kept in agreement with itself, the
exact drift this cluster already paid for in the exit-6 partition.
`RecoverPendingPurge` therefore has **one** call site, on `purge --yes`. A
mutating `prepare` that observes `purge_pending` in the index refuses exit 3
`recovery-pending` with the whole tree byte-identical and names the sanitized
`tpatch feature intent-archive purge <slug> --all --yes` (or `--blob <h> --yes`
where one hash provably covers the pending set) with
`retry_cwd: "workspace-root"`. The cost is one extra invocation in a rare state;
what it buys is that every statement about the pending-purge transaction has one
subject (PRD §7.8 step 5, §7.11, §10.4.1, PIB-525, PIB-526).

**A successful recovery ends the invocation.** It returns exit 0 with
`outcome: "recovered"`, an object naming what was restored or finalized, the
`recovered-prior-transaction` advisory and a sanitized retry; it does **not**
continue into the operation the operator asked for. rev-5 recovered and then
fell through into the lifecycle, admissibility, coherence, provider and archive
gates, any of which can return exit 2 or exit 3 — from an invocation that had
already written, because recovery restores canonical bytes and removes the
journal. The two available repairs were to qualify exit 3 as "wrote nothing
except after recovery" or to make recovery terminal. This ADR takes the second:
exit 3's zero-write meaning is the single strongest thing a harness can rely on
across this whole surface, and a qualification that depends on state the harness
cannot observe destroys it. The cost is one extra invocation after an
interrupted run, paid explicitly and reported. There is no opt-in that restores
the fall-through, because a flag would move the same defect one argument away
(PRD §7.8, §7.11, PIB-483…PIB-488).

Recovery **failure** is unchanged: divergent evidence, a J1–J10 bind failure or
a refused undo CAS is exit 6 with everything preserved, and a rolled-back undo
is exit 5. Removing stale staging directories when no journal exists is lane
hygiene, not recovery: nothing was pending, so nothing is terminal and the run
proceeds.

**`--abandon-transaction` branches after the lock and before the Git gate, the
feature-directory step, journal recovery and the pending-archive-hash gate.** It
is not a fallback that
runs after recovery has had its turn; it replaces recovery for that invocation.
rev-4 placed recovery first,
which made the escape unreachable exactly where it is needed: automatic recovery
would either consume, undo or delete the evidence the operator asked to preserve,
or refuse exit 6 first — the state abandon exists to leave. The mode therefore
validates only kind and containment of its own evidence, never the J1–J10
binds, so a corrupt, forged, version-mismatched or foreign journal is
abandonable; it moves journal, preimage and staging evidence and touches no
canonical or archive file. It also never reads `.tpatch/features/<slug>/` or
decodes `status.json`: its subject is the `.tpatch/local/` lane, so
`feature-not-found`, `status-malformed` and `status-unreadable` are unreachable
from it and an absent, malformed or unreadable feature cannot block the
inspection or the move of local evidence. A removal-pending archive index with
no journal is
**not** abandonable and is not consumed: abandon refuses `no-pending-transaction`
and names the archive purge route — and **only** that route, because a mutating
`prepare` now refuses `recovery-pending` in the same state rather than
finalizing it. A second abandon over nothing but
prior `abandoned-*` residue likewise refuses `no-pending-transaction`, preserves
that residue untouched and reports it repo-relative rather than nesting it
(PRD §6.6, PIB-449…PIB-453, PIB-499, PIB-509, PIB-510, PIB-513).

**Abandon does not run the local-lane Git gate.** It still requires D4's
authority — supported platform, non-denied root filesystem, exclusive flock —
but G1–G4 are skipped and neither lane refusal is reachable from it. The gate
proves the gitignored lane is really ignored *before tpatch writes new content
into it*; abandon writes none, renaming existing lane entries one directory
deeper in the same lane. Requiring a working `git` to move bytes already on disk
would make an absent, broken or misconfigured Git a **permanent** block on the
only escape from exit 6, which is the failure this decision exists to prevent.
The privacy boundary is unchanged; the reachability is restored (PRD §6.6,
§7.13, PIB-492…PIB-494).

**"Exit 6 is never terminal" is qualified by an executable route, and the
population is partitioned.** rev-6 named one route — the abandon mode — for all
ten exit-6 codes. That is true of nine and false of the tenth:
`archive-purge-evidence-divergent` lives entirely in the tracked archive, which
abandon is forbidden to touch, so sending an operator there would hand them a
command guaranteed to refuse `no-pending-transaction`. The routes are therefore
partitioned, and each code has exactly one:

- the nine journal/publication codes (`undo-cas-mismatch`,
  `recovery-divergent`, the five `journal-*` binds, `post-publication-divergence`
  and `workspace-root-replaced-after-publication`) →
  `tpatch prepare <slug> --abandon-transaction`, with the manual fallback below;
- `archive-purge-evidence-divergent` → D16's archive procedure: report the
  pending hash and the repo-relative managed blob and `index.json` paths, offer
  to preserve the unexpected bytes, remove the divergent managed blob path (or
  restore an index that stopped strict-decoding), then rerun the sanitized
  purge, whose pending+absent case finalizes the tombstone terminally. That
  message is forbidden from naming abandon (PIB-506, PIB-507, PIB-508). That
  code's population is exactly the **pending**-hash and pending-index evidence:
  a tombstone whose named blob is present is D10's globally-classified residue
  or mixed state, an exit-**3** refusal with its own `--orphans --yes` or
  confirmed `--blob <h> --yes` repair, and a present but unidentifiable blob
  under a retained reference is D10's `archive-blob-corrupt`; none of them is
  an exit-6 population at all (PIB-521, PIB-524, PIB-543).

**The pre-abandon gate is total, not a count of three.** rev-6 said three
environmental conditions could refuse abandon before it inspects anything. The
honest set is a table, because parse/flag validation, the slug grammar,
workspace discovery and the read-boundary platform allowlist all precede the
mutating gate, and rev-6 additionally left the feature-directory step able to
refuse the mode. The total set of stops, and the one route each may offer, is:
parse/flag → fix the command line; slug → supply a valid slug, with no lane path
composed or echoed; workspace discovery and read platform → **truly
unavoidable**, since without them there is no repo-relative lane to name at all;
mutating platform, root filesystem, and any failure to establish the authority
at all — a root or directory open that failed, or a non-contention `flock`
failure → the
repo-relative `.tpatch/local/intent-prepare/<slug>/` lane and the last-resort
manual removal, with its cost, when evidence exists; **lock contention → wait
and retry, and nothing else**, because the evidence may be a live holder's undo
journal and deleting it under that holder would destroy a running transaction's
undo material. The feature-directory step is removed from this mode entirely.
Nothing outside that table may stop abandon before it inspects the lane.

The table's **domain** is **every argv that requests a *true* abandon** — the
bare `--abandon-transaction` or an explicit `=true` — **plus every argv that
names the flag and stops in the parse layer** on a pflag, arity or mutex error.
rev-8 said
"syntactically valid invocation", which contradicted the table's own first row:
a parse, arity or mutex error is exactly the stop a syntactically **invalid**
argv takes, so under that domain row 1 was unreachable and the table failed its
own reachability rule. rev-9 replaced it with "every argv in which the flag
appears", which over-reached the other way: `--abandon-transaction=false` is a
false boolean that selects `generate` and runs that mode's whole contract, so a
table of abandon's stops would be claiming rows over an invocation that never
enters the mode. The second clause keeps the domain closed under the parse
layer, which makes row 1
reachable by construction — and captures the mixed spellings, since cobra's
mutex is keyed on flag presence rather than value, so
`--check --abandon-transaction=false` stops there too — while keeping the three
grammar-excluded branches out as
post-parse rows: the `--yes` preflight of PRD §10.5 step 1a never fires because
`--yes` is legal here, and the `--check` handoff and `--dry-run` branch are
mutex-incompatible, so an argv pairing them with this flag stops at row 1 rather
than at a row of its own. The guard derives both the domain and the row set and
fails in both directions — a reachable stop with no row, and a listed row no
fixture in the domain can produce
(PRD §5.2, §6.6, PIB-495, PIB-496, PIB-511, PIB-512, PIB-537).

The claim this ADR and the PRD are permitted to
make is therefore: **every exit-6 population has exactly one named, executable
route out — the abandon mode where the environment supports mutation, the named
manual procedure where it does not, and the archive procedure for archive
divergence** (PRD §6.6, §9.7.2, §10.4, PIB-495, PIB-496, PIB-508, PIB-519).

D9 reports **persistent evidence only**: a present journal, allowed retained
staging, abandoned evidence, orphan blobs, mixed tombstone/live-reference
hashes, corrupt blobs under retained references, owned (removal-pending)
hashes, a corrupt index, and X11 dangling
live references — every observation it made, each with its own repair, not just
the first or the worst (PIB-541). **It does not probe the lock.** rev-4 permitted a non-creating
nonblocking flock attempt; that is withdrawn, because the probe takes the real
authority and can make a genuine mutator refuse `transaction-in-progress`, and
because two concurrent doctors would diagnose each other rather than the
workspace. A health check that can fail the thing it checks is a defect at any
window size. The consequence is stated rather than replaced with a weaker
probe: **no diagnostic command can identify a lock holder or prove that none
exists**, and only the mutator that actually contended ever reports the
authority — as held, with the holder unknowable, and with wait-and-retry as the
safe action (PIB-470, PIB-471).

D9 must not infer that canonical mixed/partial bytes prove a lost journal:
ordinary analyze/define/manual lifecycle has the same shape. A clean
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

`list` is read-only: no Git process, lock or state creation. Its exits are
pinned per observation: exit 0 for a healthy archive, for an **owned** hash
(which it renders `pending-remove`/`pending-finalize` for every reference to it,
naming the owning `purge … --yes`) and for a globally
unreferenced tombstone-beside-blob, which it renders `orphan` with the
`--orphans --yes` repair; exit 3 for a dangling live reference, for a mixed
tombstone/live-reference hash, which it renders `mixed-reference` with the
confirmed `--blob <h> --yes` repair, and for a present but unidentifiable blob
under a retained reference, which it renders `corrupt` with D10's
preserve/remove-then-confirmed-purge or restore route (D10, PIB-531, PIB-535,
PIB-543). **Where observations of different exits coexist, `list` renders every
safe observation with its own repair and takes the highest exit**, so one pass
yields the whole inventory instead of one discover-fix cycle per inconsistency;
`doctor`'s D9 reports the identical set, warning-only (PIB-541). `purge` requires
one selector and previews without `--yes`; **the preview takes no lock on any
selector, and only `--yes` takes D4's authority** (D4, PIB-530). **Neither runs a
Git process**, in either form and on every selector:
`purge` writes only the tracked archive and never the gitignored staging lane,
so the D17 lane gate has no subject in it (PIB-514).

**`purge` never recovers a prepare journal.** A pending journal is an
interrupted *publication* whose undo evidence binds canonical artifacts,
`status.json` and the index together (D5); recovering it from a retention verb
would undo a publication the operator did not ask about, under binds that
belong to `prepare`. So `purge` — in both its preview and its `--yes` form —
checks for the journal **marker** before any archive work and refuses exit 3
`recovery-pending` with the whole tree byte-identical: it does not strict-decode
the journal, does not move it, does not consume it and undoes no entry. Its
remediation names the mutating `prepare` and the abandon route without
performing either. The `--yes` path takes D4's authority first, so a live
sibling still yields `transaction-in-progress`; the preview takes no lock
(PIB-350, PIB-489, PIB-490).

Only when **no** journal exists does the `--yes` path recover every pending
archive hash before a new archive mutation — and, per D13, **that recovery is
terminal**: it finalizes the pending hashes, reports exit 0 `recovered`, and does
not go on to process the selector in the same invocation, because the selection
was computed against an index the recovery has just rewritten. The operator
re-runs the reported retry (PIB-486, PIB-491).

**That recovery acts on the hash, not on the reference that carries the flag.**
Because a removal-pending reference makes the transaction the global owner of
`h` (D10), the per-hash machine begins by **claiming** `h`: one CAS-published
index rewrite setting every reference to `h`, in every generation, to
removal-pending, taken only while `blobs/<h>.blob` is present, regular and
hash-correct. It then re-reads and revalidates — requiring that no reference to
`h` is anything but removal-pending and that the file still hashes to `h` —
removes the blob, and CAS-publishes tombstones for every reference to `h`. Where
the blob is already absent, the claim is published straight to tombstones over
every reference and no removal is attempted, which is byte-for-byte the shipped
dangling repair. The invariant is stated rather than implied: **the blob is
removed only when the index, as revalidated immediately before the removal,
holds no reference to `h` that is not removal-pending.** An external writer may
insert a same-hash reference before the claim (it is claimed with the rest),
between the claim's read and its CAS (the CAS fails, nothing is removed),
between the CAS and the revalidation (no removal; exit 5 `archive-purge-partial`
whose retry re-derives the claim), or after the removal (the tombstone CAS
detects it; the retry tombstones it, which is truthful because the bytes are
gone). Only a write inside the disclosed final-syscall window escapes, and it
lands in `archive-blob-dangling` with the repair D10 already names. There is no
in-process retry loop: the operator's rerun is the loop, so the command always
terminates and always reports what it saw (PIB-539, PIB-540, PIB-544, PIB-545).

**That pass is total over selectors, `--orphans` included.** The orphan set is
derived from the index's live-reference set, and a removal-pending reference is
one whose liveness is mid-decision — finalize it and its blob may become an
orphan, leave it and the same blob is still referenced — so an orphan scan
against a pending index computes its answer from state that is about to change,
which is the defect terminal recovery exists to prevent. Every `--yes` selector
therefore takes the same recovery first and returns the same terminal
`recovered`, and the reported retry carries **the operator's own selector**,
never a widened or narrowed substitute (PIB-527, PIB-528).

**The preview recovers nothing, and says so — on every selector.** With no
journal and one or more
removal-pending hashes, a `purge` without `--yes` takes no lock, writes nothing
and finalizes nothing. It exits 0 with the closed outcome token
`recovery-required` — `planned` belongs to `--dry-run` and implies a plan the
command would execute, `refused` implies a non-zero exit and a `refusal`
object, and this is neither — carrying one closed `pending_purge` object with
`recovery_required`, `pending_hashes[]` (each `hash`, repo-relative `blob`,
repo-relative `index` and the fixed per-hash `plan`), the closed `selector`, the
sanitized `retry` and `retry_cwd: "workspace-root"`. The human rendering carries
the same hashes, paths, plan and retry line, so the two surfaces are checkable
against each other field by field. It never emits `recovered`,
because it recovered nothing. Letting a preview perform the recovery it reports
would make the confirmation flag decorative on the one command whose entire gate
is that flag (PIB-515, PIB-529). X11 requires each retained reference to
name a present regular matching blob — a present but non-regular or hash-wrong
one is `archive-blob-corrupt` with D10's route; a pending reference may have that blob
(remove next) or no blob (tombstone next); each tombstone has none. A dangling
retained reference is `archive-blob-dangling` (exit 3) for ordinary mutation,
and its **only** repair is the confirmed `purge --blob <hash> --yes` (or an
applicable `--all --yes`), which tombstones every reference to that hash after
confirming the blob is still absent and removes nothing. Rehydration is not
offered as a repair for it (D10).

`--blob h` selects every reference to h. A generation selection with a hash
shared by an unselected generation refuses `archive-blob-shared`, naming
`--blob h` or `--all`. Rehydration of h un-tombstones every reference to h in
one CAS rewrite. Thus a tombstone never lies while h.blob exists. "Not
recoverable" means only "until identical content is archived again."

**`--orphans` is defined globally by content hash, and that is load-bearing.** A
blob is an orphan only when **no** reference to its hash, in any generation, is
retained or removal-pending — which covers a blob the index never mentions and a
blob all of whose references are tombstoned, and excludes every blob whose hash
is still live. The predicate is never evaluated per reference: doing so would
let one generation's tombstone admit a blob that another generation still
retains, and `--orphans --yes` would then delete live data and leave a dangling
reference behind. The mixed state is D10's inconsistency with its own confirmed
`--blob <h> --yes` repair, not an orphan (PIB-531, PIB-533).

**A pending hash and a residue observation can coexist, and the ordering is
fixed.** They are facts about different hashes. The preview takes no lock,
writes nothing and reports only the pending recovery; the `--yes` run performs
that recovery terminally and touches the residue hash not at all; the rerun then
reaches the preflight with nothing pending and performs the residue subcase's
own repair, or refuses exit 3 naming it when the selector is not that subcase's
admitted repair. Every step of that ladder ends either in work done or in a
refusal whose remediation is an executable command, so the composition has no
dead end. rev-9 recorded one composition here that *did* look like one —
recovery removing a blob while another reference to the same hash stayed
retained — and rev-10 removes it rather than routing it: the global claim above
makes that reference part of the transaction before the bytes go, and the
pre-removal revalidation refuses to remove if anything about that has changed
(PIB-534, PIB-539).

**Everything predictable is preflighted before the first mutation, and the
storage validation is global**: selector well-formedness, strict X1–X10 decode
of the captured index preimage, X11 storage observation over the **whole
index** — never over the references the selector touches — shared-reference
analysis, per-orphan identity and hash, and the global live-reference count per
selected hash. Each of those refuses at exit 3 with nothing written, which is
what lets exit 3 keep its zero-write meaning across the whole command surface
(PIB-465, PIB-469).

**Selector-independence is load-bearing, not a scoping detail.** A mixed,
dangling or corrupt hash anywhere in the index refuses every selector —
`--orphans`, `--blob`, `--generation` and `--all` alike — at exit 3 with the
whole tree byte-identical, naming the owning repair for the observation that
refused rather than for the selector the operator typed. No partial cleanup
proceeds around a global inconsistency: the command does not remove the orphans
it could have removed and report the rest as leftovers, because acting on part
of an index that is lying about another part leaves an archive whose next
observation differs from the one just acted on. The three admissions above are
consequently scoped to the exact observation each repairs, after global
classification — `--orphans --yes` is admitted past a globally unreferenced
tombstone-beside-blob and past nothing else (PIB-533, PIB-542).

Purge then processes selected unique hashes in deterministic lexical order.
Selection is always total over each selected hash — `--blob` and `--all` select
every reference to it, and `--generation` refuses `archive-blob-shared` rather
than selecting part of one — so for one
hash it CAS-publishes every reference to that hash as removal-pending and
fsyncs, which is the same global claim the recovery makes;
if the blob is present it immediately revalidates identity/hash and removes it;
if absent it recognizes completed removal; if wrong/unsafe it refuses. It then
CAS-publishes tombstones for every reference to that hash. Crash/retry
does the same next action from pending evidence. A multi-hash `--all` or
generation selection therefore remains valid after its first removal, and no
step of it removes a blob while a retained reference to that hash survives. A
confirmed purge of a dangling retained hash tombstones it only after confirming
the blob remains absent.
`--orphans` also
captures/revalidates index and per-blob identity/hash.

**After the first pending/tombstone/index/blob mutation, a later I/O or
external-change failure is a distinct retryable outcome, not exit 3.** It is
`archive-purge-partial` at **exit 5**, reporting `completed_hashes`, an optional
`pending_hash`, `remaining_hashes`, a closed `resume` discriminator, a sanitized
retry and the fact that the current index decodes and is consistent.

**How many runs finish it depends on where it stopped, and the report must say
which case applies.** rev-6 told one story for three different stopping points,
two of which have no pending marker at all, so promising a `recovered` outcome
for them was false. The three branches are closed and total:
`pending-recovery-then-completion` — the failure landed inside one hash's
machine after its removal-pending CAS, so the first retry runs the terminal
per-hash recovery and reports `recovered`, and a second run completes the
remaining hashes; `completion-only` — the failure landed exactly between two
hashes, no reference is pending, and a single retry completes the work with **no**
`recovered` outcome; `orphan-scan` — an `--orphans` purge stopped after a
removal, and since `--orphans` rewrites no index there is no pending state, so a
single retry rescans and removes what remains, again with no `recovered`
outcome. `pending_hash` is present if and only if the branch is the first one,
which makes the shape itself the discriminator (PIB-466, PIB-467, PIB-516,
PIB-517, PIB-518).

**Retries are sanitized, and there is exactly one heading above them.** No retry
or remediation this cluster emits — the partial-purge retry, both
terminal-recovery retries of D13, the pending-purge preview retry, the
archive-divergence rerun, the dangling repair and both D10 tombstone-beside-blob
repairs — reproduces the
operator's inherited `--path`, which is an absolute pathname in ordinary use and
would violate the no-absolute-path rule on the very output an operator copies
and pastes. Each `retry` is the **equivalent** command with no root-selection
argv, accompanied by the closed field `retry_cwd: "workspace-root"` and the
single verbatim human sentence `Run this again from the same workspace root:`,
emitted at **column 0** with no leading or trailing whitespace, alone on its
line and immediately above the command. rev-8 shipped a second
spelling (`… to perform it:`) on the preview path and a numbered "3. run this
again …" step inside the divergence procedure; both are withdrawn, and rev-9's
worked examples indented the sentence with the surrounding message, which made
the byte-for-byte promise untestable — two emitters could agree on the words and
disagree on the bytes at the start of the line. A heading an operator's eye and
a harness's matcher both key on can vary in neither. The three external shell
forms the PRD's §10.7 permits are not retries
and keep their own labels. PIB-498 is the derived totality guard over every
emitter, with indentation and trailing-whitespace sensitivity fixtures
(PIB-497, PIB-498, PIB-529). Exit 5 is chosen because the
state is self-describing and resumable without human judgement: exit 3 must keep
its zero-write promise, and exit 6 means manual intervention. Divergent or
unsafe evidence discovered after the first mutation is instead exit 6,
`archive-purge-evidence-divergent` (PIB-466…PIB-468).

**"After the first mutation" is a property of the transaction, not of the
process.** The mutation that opens this window is the first one of the per-hash
machine **in flight**, which is very often a write an *earlier* invocation
performed and whose pending record this run adopted. A `purge --yes` that
acquires the authority, finds a pending reference and fails inside the recovery
before writing a byte of its own is still in the post-first-mutation window: the
index has already advanced, and exit 3 would claim a zero-write the transaction
cannot honour. Both exit-5 and exit-6 purge outcomes are therefore reachable
from a recovery invocation that wrote nothing itself, and neither is reachable
before the pending record exists — which is also why both are purge-owned
(D13, PIB-526).

**That exit 6 gets an archive-shaped escape, and it is not the abandon mode.**
Abandon touches no index and no blob (D13), so it cannot consume this evidence;
routing it there would be a dead end. The command still refuses to act, because
it will not remove or overwrite bytes it cannot identify, but the refusal is a
procedure. Its population is exactly the **pending**-hash and pending-index
evidence: a tombstone whose named blob is present holds no pending reference, so
this procedure's rerun would have nothing to finalize, and D10 classifies it by
the hash's global liveness as either unreferenced physical residue with the
`--orphans --yes` repair or a mixed tombstone/live-reference inconsistency with
the confirmed `--blob <h> --yes` repair; a present but unidentifiable blob under
a **retained** reference likewise holds no pending reference and is D10's
`archive-blob-corrupt` (PIB-521, PIB-524, PIB-531, PIB-543). For a divergent **blob** under a pending reference —
hash-wrong or non-regular — it names the pending hash, the repo-relative
`blobs/<hash>.blob` and `index.json` paths, an optional `cp` to preserve the
unexpected bytes, the `rm` of that managed blob path, and the sanitized rerun
from the workspace root; the rerun then takes the already-defined pending+absent
case, finalizes the tombstone and exits 0 `recovered`. For a divergent
**index** — one that stopped strict-decoding under an external write mid-run —
no blob is at fault, so the route is to restore `index.json` from the operator's
own version control or backup and rerun; removing the index is never offered,
because it would discard every generation record to resolve one hash. Both forms
name only repo-relative paths, state the destructive cost, and carry the
Git-history caveat this ADR already requires of every statement about removing
archived bytes (PRD §9.7.2, PIB-506, PIB-507, PIB-508).

CAS then rename/remove
is not atomic: external edit/replacement in the final syscall window can still
be overwritten/removed. The tpatch lock excludes tpatch mutators only. This is
stated for canonical and purge paths, with detected pre-CAS and disclosed
post-CAS injection rows PIB-405, PIB-421/422, PIB-429/430 and PIB-443/447.

### D17 — The Git dependency is real, read-only, and conditional

G1, `git rev-parse --is-inside-work-tree`, is run from the discovered workspace
root and is authoritative. There is no `.git` `Lstat` presence test. The
prepare executor has a closed, read-only G1–G4 argv allowlist; “no Git” is not
the contract. Exact
`true` establishes a Git worktree (including linked worktrees/submodules) and
permits G2/G3/G4; a clean Git-established false/not-worktree result permits the
non-Git lane with `workspace-not-git`; missing executable, unexpected output or
exit, and any non-standard Git failure are `local-lane-unverifiable` before
mutation. G1 uses a C locale and a narrow known-not-worktree classifier so an
arbitrary Git error cannot fail open.

G1 runs **once per normal mutating `prepare` invocation** — `generate`,
`manual`, `regenerate` — and in nothing else. A worktree non-regenerate mutator
runs exactly G1+G2+G3; a worktree regenerate
runs exactly G1–G4; an established non-worktree runs G1 only; `--check`,
dry-run, `--abandon-transaction`, `intent-archive list` and
`intent-archive purge` run **none**. The abandon exemption is D13's: that
mode writes no new content into the lane, and making the only escape from exit 6
depend on a working `git` would make a broken Git a permanent block. **The purge
exemption is structural rather than a concession**: rev-6's "per mutating
invocation or purge" implied a lane gate for a command that has no lane. Purge's
entire write set is the tracked archive — a same-directory rewrite of
`index.json` and rooted removals of `blobs/<hash>.blob` — with no staging tree,
no journal, no preimage and not one byte under `.tpatch/local/`, so G2's
question has no subject there. The retention exposure purge raises is answered
by D8's tracked-retention disclosure, not by a gate re-run at deletion time. The
lane-gate parity row is scoped to the normal mutating modes accordingly, and a
process spy asserts the zero for both purge forms and every selector (PIB-283,
PIB-514). G2/G3 receive the established state and repo-relative
lane, rather than probing again.

All prepare-specific G1–G4 invocations use a C locale and scrub a **pinned,
closed** list: `GIT_DIR`, `GIT_WORK_TREE`,
`GIT_INDEX_FILE`, `GIT_COMMON_DIR`, `GIT_CEILING_DIRECTORIES`,
`GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`,
`GIT_DISCOVERY_ACROSS_FILESYSTEM`, `GIT_PREFIX`, `GIT_IMPLICIT_WORK_TREE`,
`GIT_SUPER_PREFIX`, `GIT_CONFIG_COUNT`, and **every** variable matching
`GIT_CONFIG_KEY_<n>` / `GIT_CONFIG_VALUE_<n>` regardless of the declared count.
The list is exact: shortening it is a contract change, and extending it means
amending this decision and PRD §7.13.

**Global and system ignore configuration is deliberately preserved.**
`GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, `HOME` and `XDG_CONFIG_HOME` are not
scrubbed, because G2 asks whether the lane is ignored *for this operator* and
that answer legitimately depends on their `core.excludesFile`; suppressing it
would manufacture false `local-lane-not-ignored` refusals for correctly
configured workspaces. The scrub therefore covers repository/index/object
selection and per-invocation inline config injection only, and this ADR states
plainly that it is not a sandbox: a caller who can set these variables can also
set `HOME` and `PATH`. No argv element and no report field carries an absolute
path.

G1 runs exactly once; G2/G3 accept the
established Git tri-state and a repo-relative lane path through the central
seam and cannot call G1/IsGitAvailable again. The authorized consumers of that
one gate are `gitutil/ignore.go`, `workflow/session_ignore.go`,
`rescap/scratch.go` and **`rescap/gitgate.go`** — the last named explicitly
because it is where the shipped `check-ignore`/`ls-files` executor actually
lives; centralizing the others while leaving it spawning unscrubbed Git would
preserve the duplicate privacy gate this decision forbids. Existing callers
retain **explicit compatibility wrappers** that keep their current environment,
argv asymmetries, exit-code interpretation, output and goldens; this cluster
authorizes no behavior change for them, so the refactor moves the spawn seam
and not the semantics. Applying prepare's scrub to those callers would be a
separate, deliberately tested change. Foreign-`GIT_DIR`, indexed-config,
relative-path, closed-argv and exact conditional-count spies are mandatory. Git
has no role in keying, authority, preservation, or recovery.
`bundle-untracked-in-git` exists
only when G1 established a worktree and G4 proved `.tpatch` untracked, and says
both clone and Git-clean loss risk. PIB-327…PIB-329, PIB-408, PIB-427 and
PIB-472…PIB-476 bind it.

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
  dedicated advisory. In the default mode the same flag is a **legal no-op**
  with its own redundancy advisory, because fallback is already the behavior
  there; it is a parse error only with `--check`, `--manual` and
  `--abandon-transaction` (PRD §5.1, §5.2, PIB-500, PIB-501).

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
- **No crash leaves a slug blocked without a named route out**, and the route
  is always the one that can actually consume that evidence. Lock ownership dies
  with the process; the abandon
  branch precedes the Git gate, the feature-directory step and automatic
  recovery, so a corrupt journal, a broken `git` and a missing or malformed
  feature are all escapable; where the platform, filesystem or
  `flock` denies the mode itself, the refusal names the repo-relative lane and a
  last-resort manual removal; where the lock is merely *contended*, the route is
  wait-and-retry rather than deletion under a live holder; and archive
  divergence, which abandon cannot touch, is routed to its own archive procedure
  instead (D13, D16). The claim is stated with those qualifications rather than
  as an unconditional "always recoverable".
- **Every write this command performs is rooted**, tracked lane and gitignored
  lane alike, so the undo evidence enjoys the same boundary as the publication
  it protects (D2).
- **Exit 3 keeps a single, checkable meaning across the whole surface**: it was
  reached before the first byte of mutation, with **no** post-recovery
  qualification, because a successful recovery is terminal (D13). Partial
  archive progress has its own retryable code instead of eroding that (D16).
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
- **No diagnostic can answer "is a prepare running here?"** Removing the doctor
  probe removes the only mechanism that could have tried, and this ADR accepts
  the loss rather than shipping a check that perturbs the thing it checks
  (D13).
- **A non-denied but unrecognized filesystem may lie about `flock`**, and no
  cross-machine exclusion is provided at all; both limits are disclosed rather
  than mitigated (D4).
- **An operator's global/system Git configuration still influences the lane
  gate**, by design, so the scrub is not a sandbox (D17).
- **A purge can end partially applied.** The state is consistent and resumable
  by the reported sanitized command, but the operator must re-run it (D16).
- **An interrupted run costs two invocations, not one.** A mutating `prepare`
  that finds a pending journal, and a `purge --yes` that finds a pending archive
  hash, each recover and stop, so the
  operator (or harness) must run the reported retry to get the work done. This
  is the deliberate price of an unqualified exit-3 zero-write contract (D13).
- **A mutating `prepare` blocked by a pending archive purge costs an extra
  command too.** It refuses `recovery-pending` and hands over the purge
  invocation rather than finalizing the hash itself. That is the price of giving
  the pending-purge transaction, and the two purge-only exit codes it can
  produce, exactly one owning command (D13, D16).
- **Clearing a blob left beside a tombstone is an operator step, and which
  command clears it depends on the hash's global liveness.** A crash in the
  rehydration window leaves physical residue that tpatch refuses to delete on
  its own; `purge --orphans --yes` clears it in one command when the hash is
  unreferenced, and the confirmed `purge --blob <h> --yes` clears the mixed
  tombstone/live-reference case in one command when it is not. Either way an
  ordinary retry refuses until the right one is run, and an operator who reaches
  for `--orphans` in the live case gets an exit-3 refusal naming the correct
  command rather than a silent deletion of live bytes (D10, D16).
- **`intent-archive purge` is blocked by an unrelated pending prepare journal.**
  It refuses `recovery-pending` rather than recovering it, so retention work
  waits on a `prepare` or an abandon; that is chosen over letting a retention
  verb undo a publication (D16).
- **A global inconsistency blocks unrelated archive work.** Because X11's
  observation is over the whole index and selector-independent, one mixed,
  dangling or corrupt hash refuses every selector and every mutating `prepare`
  mode — including `--manual`, which writes no archive — until it is repaired.
  That is the deliberate price of never leaving an archive partly repaired, and
  the refusal always names the owning repair (D10, D16).
- **An operator holding a present but unidentifiable blob under a retained
  reference must run two shell commands before a tpatch command.** tpatch will
  not delete or overwrite bytes it cannot identify, so the route is preserve,
  remove the managed blob path, then the confirmed purge — or restore the
  correct blob. The cost and the Git-history caveat are stated in the refusal
  (D10).
- Extraction touches `internal/workflow/workflow.go`; the new prepare authority
  is separate from `rescap/lock_unix.go`. The Git gate refactor touches
  `internal/gitutil/ignore.go`, `internal/workflow/session_ignore.go` and
  `internal/rescap/scratch.go`, so those files are explicitly partitioned and
  their existing caller goldens are preserved.
- Recovery is undo-only, so an interrupted prepare always costs a re-run of
  generation, including its provider calls.
- A journal lost with the gitignored lane is unrecoverable and, after clean
  lane/archive deletion, ordinarily undetectable (D21).
- **Resuming a partial purge takes one run or two, depending on where it
  stopped.** The report's `resume` field says which, and the operator must read
  it rather than assume a uniform story (D16).
- **Archive divergence costs the operator a manual `rm` and a rerun.** tpatch
  will not delete or overwrite bytes it cannot identify, so the escape from
  `archive-purge-evidence-divergent` is a documented procedure rather than a
  command, and executing it discards that hash's archived recovery material
  (D16).

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
| Running automatic recovery before the abandon branch (rev-4) | Makes the operator escape unreachable: recovery consumes the evidence or refuses exit 6 first (D13). |
| Writing the journal with path-based `gitutil.DurableWriteFile` (rev-4) | Right durability shape, wrong resolution model; it would place a rooted publication's undo evidence outside the rooted boundary (D2). |
| Exact-content rehydration as a dangling-reference repair (rev-4) | Unreachable: X11 refuses every mutation that could produce the replacement bytes (D10, D16). |
| Reporting a post-first-write purge failure as exit 3 (rev-4) | Destroys exit 3's zero-write meaning, which is the one thing a harness can rely on (D16). |
| A non-creating doctor flock probe (rev-4) | Perturbs real mutators and makes concurrent doctors diagnose each other (D13). |
| Claiming dry-run reproduces every real-run refusal | Contradicted by dry-run's own design, which runs no mutating gate (PRD §6.4). |
| Scrubbing `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` in the ignore gate | Changes the answer to the question actually being asked and manufactures false refusals (D17). |
| A `.tpatch/`-wide byte scan for the token `generator` | Unsatisfiable against free-text canonical artifacts; the guard is schema-scoped instead (D12). |
| Continuing into the requested operation after a successful recovery (rev-5) | Recovery writes, and the gates below it return exit 2/3, whose contract is "wrote nothing"; making recovery terminal preserves the one code a harness can trust (D13). |
| A flag that restores the recover-then-continue fall-through | Moves the same defect one argument away: an exit code's meaning would depend on a flag (D13). |
| Recovering a pending prepare journal from `intent-archive purge` (rev-5) | A retention verb would be undoing a publication the operator did not ask about, under binds that belong to `prepare` (D16). |
| Running the local-lane Git gate before the abandon branch (rev-5) | Abandon writes no new lane content, so the gate answers nothing; requiring `git` would make a broken Git a permanent block on the only escape from exit 6 (D13, D17). |
| Leaving "exit 6 is never terminal" unqualified where platform/filesystem/`flock` denies abandon | The claim would be false exactly where it is needed; the three refusals name the lane and a manual removal instead (D13). |
| Echoing the operator's inherited absolute `--path` in a retry command (rev-5) | Puts an absolute path in output that forbids them, on the line most likely to be copied; the retry is the equivalent command plus `retry_cwd` (D16). |
| A cobra mutex between `--yes` and the default prepare mode (rev-5) | Unimplementable: the default mode is the absence of a flag, and `MarkFlagsMutuallyExclusive` takes flag names. Command-owned validation with one fixed literal replaces it (PRD §5.2). |
| Classifying the root filesystem through `int(f.Fd())` (rev-5) | Reintroduces the naked-descriptor hazard D4 rejects for the `flock` on the same file; `fstatfs` runs inside `SyscallConn().Control` (D4). |
| Routing `archive-purge-evidence-divergent` to `--abandon-transaction` (rev-6) | A dead end: abandon touches no index and no blob, so it would refuse `no-pending-transaction` and leave the operator with two refusals and no route. The population gets its own archive procedure and the message may not name abandon (D13, D16). |
| Offering `rm index.json` as the repair for an index that stopped strict-decoding | Discards every generation record in the archive to resolve one hash; the route is to restore the file and rerun (D16). |
| Keeping "exactly three environmental conditions can stop abandon" (rev-6) | A count, not a proof: parse, slug, workspace and read-platform stops precede the mutating gate, and rev-6 also left the feature-directory step able to refuse the mode. The gate is now a total table (D13). |
| Offering the lane `rm -rf` on a `transaction-in-progress` refusal | The evidence may be a live holder's undo journal, and the holder is unknowable; deleting it would destroy a running transaction's undo material. Contention's route is wait and retry (D13). |
| Running G1 for `intent-archive purge` (rev-6's "per mutating invocation or purge") | Purge writes only the tracked archive and never the gitignored lane, so the gate has no subject there; it runs zero Git processes (D17). |
| Promising "one recovered run, then one completing run" for every partial purge (rev-6) | False for the two branches that carry no pending marker: a failure between hashes and an `--orphans` stop each finish in one run and never emit `recovered` (D16). |
| Letting a `purge` preview perform the pending-hash recovery it reports | Makes the confirmation flag decorative on the one command whose entire gate is that flag; the preview names the `--yes` rerun instead (D16). |
| Filing a tombstone whose named blob is present under `archive-purge-evidence-divergent` (rev-7) | Unexecutable: that population's escape names a pending hash and reruns so the pending+absent case finalizes the tombstone, and a tombstoned reference has no pending record, no hash to name and nothing left to finalize. It is exit-3 zero-write on discovery, repaired by `purge --orphans --yes` or by the confirmed `purge --blob <h> --yes` depending on the hash's global liveness (D10, D16). |
| Classifying a tombstone beside a present blob **per reference** (rev-8) | A tombstoned reference proves nothing about a hash another generation still retains. Under that rule `--orphans --yes` deletes a live blob and leaves a dangling retained reference — live data destroyed through the one selector that promises to remove nothing referenced. The predicate is global by hash, and the mixed state gets the confirmed `--blob <h> --yes` repair (D10, D16). |
| Repairing the mixed state by un-tombstoning the tombstoned reference | It would make tpatch adopt bytes into a reference the operator purged, silently reversing a confirmed destructive decision on the strength of an index edit tpatch did not make. Purging in the direction the tombstone already recorded destroys nothing the operator had not already asked to destroy for that hash (D10). |
| Two spellings of the retry heading (rev-8's `… to perform it:` and the numbered divergence step) | A heading an operator's eye and a harness's matcher both key on cannot have variants. One verbatim sentence covers every emitter, and PIB-498 derives the emitter set rather than checking a list (D16). |
| Defining the pre-abandon gate table over "syntactically valid" invocations (rev-8) | It contradicted the table's own parse/arity/mutex row, which only a syntactically invalid argv can reach, so the table failed its own reachability rule. The domain is every argv that attempts to select the mode (D13). |
| Leaving `os.OpenRoot` / `Root.Open(".")` failures unclassified (rev-8) | The one remaining way this fail-closed ladder could surface an unnamed error. They are `directory-flock-unavailable`: no lock taken, no holder implied, and the manual route stays safe to name (D4, D13). |
| Letting a mutating `prepare` run `RecoverPendingPurge` (rev-6, rev-7) | Gives one transaction two owners, so `prepare` can emit purge-only codes whose remediation, exit-window scoping and route belong to `purge`, and the archive procedure must be duplicated and kept in agreement. One call site, and `prepare` refuses `recovery-pending` naming the purge command (D13, D16). |
| Exempting `--orphans` from the pending-hash recovery and the pending-purge preview (rev-7) | The orphan set is derived from a reference set a pending record is mid-decision about, so the scan would answer from state about to change. Every selector takes the same terminal recovery and the same `recovery-required` preview (D16). |
| Reporting a widened `--all --yes` retry from a pending-purge preview | The retry line is the one an operator copies unread; it must be their own selector with `--yes` appended, never widened or narrowed (D16). |
| Carrying the pending-purge preview on `planned` or `refused` (rev-7) | `planned` implies an executable plan and `refused` implies a non-zero exit and a `refusal` object; the preview is neither, so it gets the closed token `recovery-required` (D16). |
| Listing grammar-excluded branches as pre-abandon gate rows (rev-7) | A row no fixture can reach is not completeness; rev-7's own cell called its `--yes` row unreachable. The table's domain is the stops an abandon-requesting or parse-failing argv can reach (D13). |
| Defining the pre-abandon gate table over "every argv in which the flag appears" (rev-9) | `--abandon-transaction=false` selects `generate` and runs that mode's whole contract, so the table would claim rows over an invocation that never enters abandon. The domain is every argv requesting a *true* abandon, plus every argv that names the flag and fails in the parse layer — which keeps row 1 reachable and keeps presence-keyed mutex errors such as `--check --abandon-transaction=false` inside it (D13, PIB-511, PIB-537). |
| Letting the per-hash purge recovery act on the pending references alone (rev-9) | It removes `h.blob` while another generation's reference to `h` is still retained, so the mechanism that repairs dangling references becomes a producer of them. A pending reference now owns the hash globally: the machine claims every reference to `h` before removing anything and revalidates immediately before the removal (D10, D16). |
| Refusing the mixed pending/retained index at X11 instead of claiming it | X11 refuses ordinary mutation, so a refusal would stop the only command that can advance the transaction — an index no command may act on, whose documented repair is the command that just refused. Normalizing under the owning transaction resolves it, and every reference's final state is the one the pending record already committed to (D10). |
| Retrying the global claim in-process until no new same-hash reference appears | The authority excludes tpatch mutators only, so an external writer can defeat any fixed retry count and starve an unbounded loop while the workspace lock is held. A revalidation failure is `archive-purge-partial`; the operator's rerun is the loop, and it terminates and is observable (D16). |
| Validating X11 only over the references the requested selector touches | It leaves an archive partly repaired: `--orphans --yes` would delete residue while a mixed hash elsewhere went unmentioned, and the operator's next observation would differ from the one just acted on. The scan is global before every archive mutation and each admission is scoped to the exact inconsistency it repairs (D16, PIB-542). |
| Treating a present but hash-wrong blob under a retained reference as dangling, or as an orphan | It is not dangling — the file exists, and tombstoning a live reference against unvalidated bytes destroys unseen evidence — and it is not an orphan, because the hash is live. It is `archive-blob-corrupt` with one repo-relative route ending in the already-admitted dangling repair (D10). |
| Letting `list` or `doctor` stop at the first exit-3 observation, or report only the worst | It turns an *n*-inconsistency archive into *n* discover-fix cycles whose refusals look alike. Both render every safe observation with its own repair and take the highest exit (D16, PIB-541). |
| Leaving the retry heading's indentation unspecified (rev-9) | The sentence was promised byte-for-byte and then indented differently in different worked examples, so two emitters could satisfy the promise and disagree on the bytes a harness matches. It is a column-0 literal with no leading or trailing whitespace, alone on its line (D16, PIB-498). |

---

## Acceptance dependencies

Each decision is verified by named rows in the companion PRD's §18 matrix.
Every row cited here exists in that matrix (PRD PIB-229 asserts it
mechanically).

| Decision | Verified by (PRD §18) |
|---|---|
| D1 three guarantees; T0 not claimed; T1 scoped | PIB-103, PIB-155, PIB-206, PIB-314, PIB-315, PIB-324 |
| D2 rooted writes, the rooted control writer, outside-root boundary, manual CAS and in-root residual | PIB-148…PIB-151, PIB-154, PIB-182, PIB-183, PIB-308…PIB-313, PIB-399, PIB-400, PIB-406, PIB-454…PIB-456 |
| D3 single-file rooted/CAS `--manual` publication set | PIB-051, PIB-052, PIB-257, PIB-258, PIB-399, PIB-400 |
| D4 held-root directory authority, `Control`-scoped `fstatfs`/`flock`, exact denied classes, the classified root/directory open failure, release-versus-`Control`, root replacement boundary, the closed never-acquire population and platform envelope | PIB-053, PIB-080, PIB-117, PIB-124, PIB-125, PIB-279…PIB-295, PIB-395…PIB-398, PIB-409…PIB-418, PIB-433…PIB-437, PIB-442, PIB-478…PIB-481, PIB-504, PIB-530, PIB-536 |
| D5 undo-only recovery, plan binding, semantic CAS and purge residual disclosure | PIB-113, PIB-116…PIB-123, PIB-126, PIB-164, PIB-275…PIB-278, PIB-296…PIB-307, PIB-421, PIB-422, PIB-429, PIB-430, PIB-488 |
| D6 disclosed content and in-root name-to-object residuals | PIB-100…PIB-105, PIB-296, PIB-297, PIB-324, PIB-406 |
| D7 fixed publication order, status last | PIB-108, PIB-109, PIB-121, PIB-122 |
| D8 durable content-addressed archive, conditional tracking | PIB-060…PIB-068, PIB-071, PIB-152, PIB-153, PIB-330, PIB-342…PIB-344 |
| D9 archive is not provenance; trigger unfired | PIB-140…PIB-147, PIB-378, PIB-379 |
| D10 deterministic ids, immutable tombstone identity, **global pending ownership and the removal invariant**, the total X11 storage-observation map over ownership and liveness, unreferenced physical residue and its `--orphans --yes` repair, the mixed tombstone/live-reference state and its confirmed `--blob <h> --yes` repair, the retained corrupt-blob route, global rehydration and the singular dangling repair | PIB-064, PIB-065, PIB-069, PIB-160…PIB-162, PIB-165, PIB-168, PIB-169, PIB-331…PIB-341, PIB-401…PIB-404, PIB-423…PIB-425, PIB-428, PIB-443, PIB-444, PIB-447, PIB-457…PIB-460, PIB-521…PIB-524, PIB-531…PIB-533, PIB-535, PIB-539…PIB-545 |
| D11 metadata preimages are raw files **and** modelled entries | PIB-119, PIB-122, PIB-164, PIB-277, PIB-278 |
| D12 pure generators, goldens, nil retry store, structural no-raw sink and the schema-scoped provenance guard | PIB-184…PIB-190, PIB-208…PIB-211, PIB-376, PIB-419, PIB-420, PIB-446, PIB-144, PIB-477 |
| D13 terminal journal recovery, the single-owner pending-purge transaction and `prepare`'s zero-write `recovery-pending` refusal, abandon before the Git gate, the feature-directory step, journal recovery and the pending-hash gate, the pre-abandon gate total over every argv attempting the mode, the partitioned exit-6 routes, three entry points, and evidence-only doctor with no probe | PIB-079, PIB-133…PIB-136, PIB-177, PIB-213, PIB-268…PIB-274, PIB-380…PIB-387, PIB-445, PIB-449…PIB-453, PIB-470, PIB-471, PIB-483…PIB-488, PIB-492…PIB-496, PIB-499, PIB-506…PIB-513, PIB-519, PIB-525, PIB-526, PIB-536, PIB-537 |
| D14 accepted check frozen; implementation is a prerequisite | PIB-198…PIB-207, PIB-391 |
| D1–D21 reference truth, step-reference truth, dry-run scope honesty and grammar-line completeness | PIB-461…PIB-464, PIB-482, PIB-502, PIB-503, PIB-505, PIB-520 |
| D15 redaction precondition; refusal, not scrubbing | PIB-262…PIB-267, PIB-388 |
| D16 bounded retention: list and its pinned exits and coexistence precedence, the journal-marker refusal, the complete **selector-independent** preflight, the globally-claimed terminal per-hash pending recovery on every selector, the non-recovering `recovery-required` preview and its bound object, rehydration, the globally-defined orphan predicate and both tombstone-beside-blob subcases, the compound pending-plus-residue ordering, the single dangling repair, sanitized selector-preserving retries under one verbatim heading, the three conditional `archive-purge-partial` branches, the archive-divergence procedure and honest CAS | PIB-316, PIB-317, PIB-345…PIB-361, PIB-405, PIB-421…PIB-430, PIB-443, PIB-444, PIB-447, PIB-457…PIB-460, PIB-465…PIB-469, PIB-486, PIB-489…PIB-491, PIB-497, PIB-498, PIB-506…PIB-508, PIB-514…PIB-518, PIB-521…PIB-523, PIB-527…PIB-530, PIB-531…PIB-535, PIB-538…PIB-545 |
| D17 exact, scrubbed, single-probe Git usage scoped to normal mutating `prepare`, the abandon and purge exemptions, `gitgate.go` inclusion and compatibility wrappers | PIB-106, PIB-107, PIB-279…PIB-283, PIB-326…PIB-329, PIB-408, PIB-427, PIB-438, PIB-439, PIB-472…PIB-476, PIB-492…PIB-494, PIB-514 |
| D18 provider authority for `--regenerate`; the default-mode `--allow-heuristic` no-op; deadlines | PIB-365, PIB-368…PIB-375, PIB-500, PIB-501 |
| D19 coherent-suffix default; sidecar never overwritten | PIB-245…PIB-256 |
| D20 `FEATURES.md` derived and outside T1 | PIB-049, PIB-257…PIB-261 |
| D21 journal-loss boundary; evidence-only residue and clean-loss non-detectability | PIB-321…PIB-323, PIB-407 |

---

## References

- [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md) — companion (rev-10)
- [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md) — accepted prerequisite; its **implementation** is a dispatch precondition
- [ADR-034 rooted filesystem inspection boundary](./ADR-034-rooted-filesystem-inspection-boundary.md)
- [ADR-033 resource capture boundary](./ADR-033-resource-capture-boundary.md)
- [ADR-027 capture context privacy boundary](./ADR-027-capture-context-privacy-boundary.md) — D3 is normative for D15
- [PRD-feature-resource-claims-and-capture-adapters](../prds/PRD-feature-resource-claims-and-capture-adapters.md) — the shipped lock and content-addressed publication precedents
- [PRD-active-feature-session](../prds/PRD-active-feature-session.md) — the shipped purge grammar reused by D16
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md)
- `internal/rescap/lock_unix.go` — the shipped kernel-lock precedent D4 adapts without extraction
- `internal/redact/redact.go` — the shipped scanner D15 requires
- `internal/cli/land_journal.go` — the shipped journal and recovery precedent
- `internal/store/resource_publish.go` — the shipped content-addressed publication and semantic-CAS precedent
- `internal/store/store.go` — the shipped single-file atomic writer, deliberately **not** reused for publication (D2)
- `internal/gitutil/index_snapshot.go` — the shipped `DurableWriteFile` durability shape, a precedent only (D2)
- `internal/rescap/gitgate.go` — the shipped Git ignore/tracked executor the central-gate refactor must include (D17)
- `internal/rescap/statfs_linux.go`, `internal/rescap/statfs_darwin.go` — the shipped filesystem-classification precedents D4 adapts to a held descriptor
