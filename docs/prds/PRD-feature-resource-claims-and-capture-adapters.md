# PRD — Feature Resource Claims & Capture Adapters (rev-10)

**Status**: Draft — rev-10 (supersedes rev-9, writer commit `0b15495`,
adjudicated NEEDS REVISION → REV-10 DISPATCHED at `19e3024`; see
`docs/supervisor/LOG.md` → Cluster H rev-0 through rev-9 adjudications)

**Owner**: Cluster H implementation lane (planning phase — this document
does not ship code; a future "Cluster H'" implementation cluster consumes
it)

**Related**: `ADR-033-resource-capture-boundary.md` (binding decisions
this PRD assumes), `ADR-027-capture-context-privacy-boundary.md` (D1–D6,
directly extended), `ADR-030-multi-slug-reconcile-derivation-mode.md`
(`.git/**` two-layer exclusion precedent), `ADR-032-feature-unapply-state-boundary.md`
(ID-generation and fixed-struct-JSON precedent), `docs/state-of-the-art/storage-substrate-and-versioned-data.md`
§3, §9 (tracked Dolt/substrate research), `internal/workflow/session_ignore.go`
(`EnsureLocalIgnoreContract`, reused rather than re-invented — §10.3)

---

## 0. Rev-10 Fold Summary (read this first)

Rev-9 (`0b15495`) decoupled leader-exit detection from pipe drain and
fixed the CURRENT/matrix cross-reference bugs the rev-8 adjudication
found — but the rev-9 adjudication (`19e3024`) found six consolidated
findings, framed as a bounded terminal correction that does **not**
reopen §1's authority/scope decisions or §2's goals:

1. **Process contract — the actual PGID-reuse defect.** Rev-9's own
   termination redesign called `cmd.Wait()` as soon as its
   pipe-decoupled leader-exit detection fired — but that call reaps
   the leader immediately, releasing its PID/PGID back to the kernel's
   reuse pool, **before** the `SIGTERM`→grace→`SIGKILL(-pgid)` sequence
   has necessarily run at all (in the "leader exited on its own"
   branch, that sequence runs *after* the reap, against a PGID number
   the kernel may already have recycled for an unrelated process).
   Rev-10 keeps the caller-owned `os.Pipe()` pairs (still assigned
   directly to `cmd.Stdout`/`cmd.Stderr`, never `cmd.StdoutPipe()`/
   `cmd.StderrPipe()` — C30 remains historical rationale for *why*
   pipe ownership matters, not a normative requirement to use those
   accessor methods) but replaces the early `cmd.Wait()` with a
   build-tagged (`linux`/`darwin`) **non-reaping** observer built on
   the raw `waitid(P_PID, leaderPID, &buf, WEXITED\|WNOWAIT)` syscall
   (stdlib-only, no `golang.org/x/sys` dependency), which detects the
   leader's exit **without reaping it**. That observation is raced
   (via `select`) against the invocation timeout and the output-cap
   trigger; whichever fires first enters **one** cleanup function:
   unconditional `SIGTERM(-pgid)` (tolerate `ESRCH`) → fixed grace →
   unconditional `SIGKILL(-pgid)` (tolerate `ESRCH`) → await the
   non-reaping observation if it has not already fired → join the
   pipe-drain goroutines → call `cmd.Wait()` **exactly once**, only
   now, to finally reap the leader. Because no step before the final
   `cmd.Wait()` reaps the leader, its PID/PGID remains reserved by the
   kernel throughout the entire signal-then-grace-then-kill window —
   this is what actually closes the race, not any claim that `Wait()`
   or the observer "proves the group is empty." Start failure
   performs no wait/signal at all. The escaped-session-descendant
   residual (a descendant that has detached from the leader's PGID,
   e.g. via its own `setsid()`, is not reached by `-pgid` signaling)
   is now stated honestly alongside the pre-existing post-`SIGKILL`
   kernel-teardown-race residual, rather than left implicit. A
   cross-compile/source-shape test confirms the build-tagged observer
   compiles for `linux` (`amd64`/`arm64`) and `darwin` (`arm64`) with
   no external dependency (§6.4/D5 fix, task 1, new `AC-106`, `C41`).
2. §7.1's **top-level** "every mutator" local-gate enumeration still
   listed only `add`/`remove`/`clear`/`capture`/`record --resources`,
   omitting `trust-dolt` — a distinct gap from the §7.2 lock-verb list
   rev-8 already fixed and the §7.1 mode-conflict paragraph rev-9
   otherwise left alone; `AC-52`/row 86 are corrected to state that
   `add`/`remove`/`clear`/`trust-dolt` (any of the four) never perform
   the orphan sweep, which only `capture`/`record --resources` run
   (§7.1 fix, task 2).
3. **Taxonomy collision.** `adapter-missing` was reused for both the
   add-time (exit 2) missing-executable refusal and the capture-time
   (exit 3) missing-executable refusal in the same named-refusal slot;
   renamed the add-time occurrence to `adapter-missing-at-add` (exit
   2), keeping `adapter-missing` exclusively for capture (exit 3).
   Separately, `path-outside-repo` was referenced once (in the Dolt
   `db_path` gate) but never actually defined as a named exit-3
   refusal in the consolidated table; §9.1 now defines it explicitly
   as the pre-existing lexical `EnsureSafeRepoPath`/`NormalizeClaimPath`
   containment pre-filter's own refusal name, distinct from
   `symlink-component-refused` (the later, `Lstat`-based
   ancestor-symlink check), and the exit-3 table now lists it (§3/§9.1
   fix, task 3).
4. **Private-copy mode sequence, four locations disagreed.** PRD §6.1
   capture-time, PRD §7.1, ADR D5, and ADR D10 each described a
   different creation-mode sequence for the executed private copy (two
   said direct-`0500`-at-creation, two said `0700`-then-`chmod`-to-
   `0500`) — and PRD §6.1's add-time TOFU bootstrap unnecessarily
   created and then deleted a private copy it never executed. Rev-10
   unifies all four locations on one sequence: add-time TOFU opens the
   resolved binary and hashes the opened descriptor directly (no
   private copy file is ever created, zero processes started);
   capture-time creates the private copy `O_CREATE\|O_EXCL\|O_WRONLY`
   mode `0600`, streams/hashes it, `Sync`s it, verifies the digest
   against the pinned `trust.binary_sha256`, and only *after* that
   match hardens it to `0500` via a descriptor-based `Fchmod` (never a
   path-based `os.Chmod`, closing a TOCTOU window a path-based chmod
   would reopen), verifies the resulting mode, closes, and executes
   that private path (§6.1/§7.1/D5/D10 fix, task 4, `AC-93`/`AC-102`
   rewritten).
5. **Vector 3/CURRENT parity.** PRD §13.3's Vector 3 worked example
   declared its five arguments in a different order
   (`to, db_path, contract, table, from`) than PRD §0.3 and ADR D3
   (`to, db_path, table, from, contract`) — a pure documentation
   inconsistency (the canonical, sorted-key JSON encoding used to
   compute `resource_id` is order-independent, so no hash value is
   affected) — now normalized to the canonical order everywhere.
   Separately, `docs/handoff/CURRENT.md` referred twice to an
   "invented Vector 5" that does not exist: `res_acc91dc23a8b` is
   actually **Vector 1** (`git-metadata`/head, ADR Test Matrix rows
   117–120), not a fifth vector — there are, and have only ever been,
   four vectors — both occurrences are corrected (task 5).
6. **Stale surfaces.** `AC-85` is rewritten to describe the single
   non-reaping-observer-based sequence applied identically to all
   three trigger branches (superseding its rev-7-vintage
   "timeout/cap-triggered kill" framing); `AC-96`/`AC-97` are
   rewritten to remove every reference to an early, leader-exit-
   triggered `cmd.Wait()` call, replacing it with the non-reaping
   observer and restating that `cmd.Wait()` is always the *last*
   event, never the first, in every branch; a new `C41` grounds the
   `waitid`/`WNOWAIT`/`SYS_WAITID` mechanism (empirical non-reaping
   validation plus the cross-compile feasibility test); `C40`'s own
   grounding text is narrowed to state only what it actually proved
   (pipe-drain/`Wait()` decoupling), not that this decoupling alone
   justified calling `Wait()` early; `C36`'s attribution is corrected
   to credit the rev-10 mechanism, not rev-8/rev-9, as the first
   design that actually implements "escalation never skipped, leader
   unreaped through signaling" correctly for *all three* branches, not
   only the timeout/cap branch; and a stray `cmd.StdoutPipe()`/
   `cmd.StderrPipe()` reference left over in §5.1's directory-capture
   prose is corrected to caller-owned `os.Pipe()`s (task 6).

Rev-10 is a **bounded terminal correction** — of the six findings, one
(finding 1) is a genuine mechanism fix closing a real correctness bug;
the remaining five are documentation/taxonomy/cross-reference
corrections with no behavioral change beyond what finding 1 and
finding 4's mode-sequence unification already require. It does not
touch §1/§2's scope, §5.2/§5.3's closed resource kinds, or the
separate-manifest/no-lifecycle-authority/Dolt-optional/Git-only-replay
decisions every prior revision has preserved. §6.1, §6.4, §7.1, §9.1,
§13.3 are revised in place; the ADR's D5, D10, D3 (Vector 3 order,
already-correct, verified unchanged), and Test Matrix rows 86, 149–152,
162, 165–166, 171, 173–174 mirror these fixes, and a new row is added
for `AC-106`. **Preserved** because every review across all eleven
revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.
- No tracked artifact ever contains a raw file byte, raw adapter
  stdout, or any content copied verbatim from a scanned source
  (ADR-027 D3); scanning happens strictly in bounded in-process memory,
  never pre-persisted to a scratch file (task 2).

Everything below this point folds rev-10's corrections into the
standalone design; §0.1 lists the new citation (`C41`) grounding the
non-reaping `waitid`/`WNOWAIT` observer redesign (appended to the
existing Claims Audit table).
Rev-9's own `C1`–`C40` remain correct and are not repeated (see
`docs/handoff/HISTORY.md`/git history for that table).

**Historical context — rev-9's own fold summary** (superseded by the
above as the *current* state, preserved verbatim below for
continuity; its internal §0.1–§0.4 subsection numbers are the ones
this document's cross-references still use, now carrying rev-10's
additions appended to them):

Rev-8 (`816bc14`) closed the acceptance/lifecycle gaps rev-7 left
open — Test Matrix/prose consistency for the leaf/subtree/ancestor
gate split, a single unified process-group termination sequence,
add-time-TOFU-vs-capture-time-execution trust split, `trust-dolt`
added to lock/gate enumerations, exit-taxonomy consolidation, and
private-copy `noexec`/`ENOSPC`/`EIO` handling — but the rev-8
adjudication (`8152a8b`) found six residual internal findings, framed
as a bounded terminal micro-fold that does **not** reopen §1's
authority/scope decisions or §2's goals (a separately-scoped external
pass on rev-8 had accidentally re-run against the stale rev-7 range
and is excluded from this adjudication; its already-folded
observations are discarded, not re-litigated):

1. The ADR's Test Matrix row 146 still said the retry-fsync step
   `fsync`s "each newly-created directory's parent" — rev-7's own
   normative prose (§7.1) already committed to an **unconditional**
   fsync of the entire relevant chain, both newly-created **and**
   already-visible-on-retry directories, not merely each newly-created
   entry's parent; the matrix row's summary text had not been updated
   to match its own normative source (ADR Test Matrix fix, task 1 —
   text-only; no behavioral change, §7.1/D9 were already correct).
2. §6.4's unified termination sequence gated the **entire**
   `SIGTERM`→grace→`SIGKILL` group-cleanup sequence behind "once its
   pipes are drained" — but pipe drain-to-`EOF` cannot complete while
   *any* process still holds the write end open, including a
   descendant the direct child (leader) itself spawned; if the leader
   exits successfully while a descendant keeps the pipe open, the old
   design has no independent way to detect the leader's own exit and
   is left waiting on a drain that only the (not-yet-signaled)
   descendant can unblock — a genuine ordering deadlock, not merely
   an unproven edge case. Rev-9 decouples "leader has exited" detection
   from pipe drain entirely by switching from `cmd.StdoutPipe()`/
   `cmd.StderrPipe()` to a pair of caller-created `os.Pipe()`s whose
   write ends are assigned directly to `cmd.Stdout`/`cmd.Stderr` before
   `Start()`, with the caller closing its own reference to each write
   end immediately after `Start()` returns; because `os/exec` neither
   creates nor manages these particular `*os.File` objects (that
   coupling is specific to `StdoutPipe()`/`StderrPipe()`, per C30),
   `cmd.Wait()` in this configuration blocks only on the OS-level reap
   of the leader's own PID and returns as soon as the leader exits,
   independent of whether a descendant still holds either pipe open
   (C40). The unconditional group-signal sequence now triggers on
   whichever of three events fires first — the (now pipe-independent)
   `cmd.Wait()` completing, the 30-second invocation timeout, or the
   output-cap exceeded (still detected by two separate goroutines
   reading the pipes' read ends into the shared budget) — closing the
   actual gap: a descendant sharing the PGID can no longer outlive a
   successfully-exited leader, because the leader's own exit is
   detected the instant it happens rather than only once the pipes
   themselves reach `EOF` (§6.4/D5 fix, task 2).
3. §7.2's Lock semantics enumeration of "every mutating verb" that
   acquires the per-slug `flock` still listed only `add`/`remove`/
   `clear`/`capture`/`record --resources` — omitting `trust-dolt`,
   even though §7.1/§10.3/D9's own enumerations already included it;
   this is exactly the "one universal lock/gate list omits
   `trust-dolt`" gap the rev-8 adjudication flagged, just in a
   different section than rev-8's own fix reached (§7.2 fix, task 3;
   the companion ADR's Test Matrix row 77/`AC-47` had an identical
   omission and is fixed alongside it).
4. (Verified, no fix needed) `add --trust-current-dolt`'s add-time
   TOFU bootstrap (§6.1) already ran only the shared resolution
   prefix followed by open→copy→hash→write-pin→delete, with **no**
   "refuse if unpinned" precondition anywhere in that sequence — the
   rev-7-vintage single-numbered-list bug the rev-8 adjudication
   described was already fully corrected by rev-8's own §6.1 rewrite;
   this finding is recorded here as confirmed-clean, not re-fixed.
5. (Verified, no fix needed) Duplicate `add` targeting an existing,
   identical declaration is already a **strict** no-op with respect to
   `trust.binary_sha256` regardless of whether `--trust-current-dolt`
   is re-passed (§4, `AC-103`) — only `trust-dolt` may re-pin after the
   initial `add`; this finding is recorded here as confirmed-clean.
6. (Verified, no fix needed) The exit-code table already names the
   add-time missing-flag refusal `dolt-trust-flag-required` (exit 2)
   distinctly from the capture-time missing-pin refusal
   `dolt-trust-required` (exit 3), and both `adapter-copy-noexec`/
   `adapter-copy-failed` already appear exactly once each in the
   consolidated table (§3); this finding is recorded here as
   confirmed-clean.

Rev-9 is a **bounded terminal micro-fold** — of the six findings, only
three (row 146's stale summary text, §6.4's termination-protocol
deadlock, and §7.2's `trust-dolt` omission) required an actual change;
the remaining three were verified against the adjudication's own
described symptom and found already resolved by rev-8's own rewrite,
and are recorded above rather than silently dropped. It does not touch
§1/§2's scope, §5.2/§5.3's closed resource kinds, or the separate-
manifest/no-lifecycle-authority/Dolt-optional/Git-only-replay
decisions every prior revision has preserved. §6.4, §7.2 are revised
in place; the ADR's Test Matrix rows 77, 146, 165, 166 and D5 mirror
these fixes. **Preserved** because every review across all ten
revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.
- No tracked artifact ever contains a raw file byte, raw adapter
  stdout, or any content copied verbatim from a scanned source
  (ADR-027 D3); scanning happens strictly in bounded in-process memory,
  never pre-persisted to a scratch file (task 2).

Everything below this point folds rev-9's corrections into the
standalone design; §0.1 lists the new citation grounding the
`os.Pipe()`-based termination redesign (appended to the existing
Claims Audit table).
Rev-8's own `C1`–`C39` remain correct and are not repeated (see
`docs/handoff/HISTORY.md`/git history for that table).

**Historical context — rev-8's own fold summary** (superseded by the
above as the *current* state, preserved verbatim below for
continuity; its internal §0.1–§0.4 subsection numbers are the ones
this document's cross-references still use, now carrying rev-9's
additions appended to them):

Rev-7 (`2aba39b`) closed the compatibility gaps rev-6 left open —
leaf-targeted ignore/untracked gate for fresh clones, unreaped-leader-
through-grace group termination, executed-binary-binding via a
private hash-verified copy, trust/identity split with a `trust-dolt`
re-pin command, `contract`-enum runtime-compatibility honesty, a
corruption-vs-collision taxonomy split, `uint32` `statfs` width
normalization, and unconditional fsync discipline — but the rev-7
adjudication (`bc2c068`) found eight residual gaps in rev-7's own new
mechanisms, framed as a bounded "acceptance/lifecycle micro-fold" that
does **not** reopen §1's authority/scope decisions or §2's goals: the
ADR's Test Matrix row 146 still encoded the *superseded* "ignore/
untracked gate against the nearest existing ancestor" text even though
the PRD's own `AC-81` had already been corrected to leaf-target the
ignore/untracked checks in rev-7 — the two documents disagreed with
each other (ADR Test Matrix fix, task 1); process-group termination's
**normal-success path** called `cmd.Wait()` immediately once pipes
reached EOF, with no grace period and no group-wide `SIGKILL`, so a
*successful* Dolt invocation could still leave a rogue descendant
sharing the same PGID alive past `tpatch`'s own return — only the
*kill-triggered* path's unreaped-leader-through-grace sequence actually
proved the group was empty (§6.4 fix, task 2); §6.1's single, undivided
numbered sequence opened with "refuse if unpinned," which — read
literally — would fire during `add --trust-current-dolt`'s own
first-time pin computation, since by definition no pin exists yet the
first time that flag runs; the sequence needed to be split into a
non-executing add-time bootstrap and the existing pin-requiring,
executing capture-time sequence (§6.1/§3 fix, task 3); `trust-dolt`
was missing from at least one "every mutator"/local-ignore-gate
enumeration, and the duplicate-`add` idempotency language ambiguously
implied a re-passed `--trust-current-dolt` might update an existing
entry's `trust` field as a side effect, when only `trust-dolt` may
ever re-pin after the initial `add` (§3/§4/§7.2/§10.3 fix, task 4);
the exit-code taxonomy had `dolt-trust-required` doing double duty as
both an add-time exit-2 name and a capture-time exit-3 name in the
same named-refusal slot, violating this design's own "each named
refusal in exactly one row" convention (§3/§14 fix, task 5); the
capture-time private-copy sequence had no defined behavior for a
`noexec`-mounted scratch filesystem or an `ENOSPC`/`EIO` failure
during the copy itself (§6.1 addition, task 6); and §7.1 and §10.3
disagreed on what the untracked-check's target path even was — §7.1's
own worked example already used the exact per-slug leaf, while §10.3
described re-using the whole `.tpatch/local/` subtree for the same
check (§7.1/§10.3/ADR-D8 fix, task 7). Rev-8 is a **bounded
acceptance/lifecycle micro-fold** — it does not touch §1/§2's scope,
§5.2/§5.3's closed resource kinds, or the separate-manifest/
no-lifecycle-authority/Dolt-optional/Git-only-replay decisions every
prior revision has preserved. §3, §4, §6.1, §6.4, §7.1, §7.2, §10.3,
§14, §14.1 are revised in place; the ADR's Test Matrix row 146, D5, D8
mirror these fixes. **Preserved** because every review across all nine
revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.
- No tracked artifact ever contains a raw file byte, raw adapter
  stdout, or any content copied verbatim from a scanned source
  (ADR-027 D3); scanning happens strictly in bounded in-process memory,
  never pre-persisted to a scratch file (task 2).

Everything below this point folds rev-8's corrections into the
standalone design; §0.1 lists the new citations grounding those
corrections (appended to the existing Claims Audit table). Rev-7's
own `C1`–`C37` remain correct and are not repeated (see
`docs/handoff/HISTORY.md`/git history for that table).

**Historical context — rev-7's own fold summary** (superseded by the
above as the *current* state, preserved verbatim below for
continuity; its internal §0.1–§0.4 subsection numbers are the ones
this document's cross-references still use, now carrying rev-8's
additions appended to them):

Rev-6 (`f195998`) closed the compatibility gaps rev-5 left open —
binary trust pin, `db_path` hard refusal, `Setpgid` termination,
resource-ID collision refusal, `current_batch_id` rename, file-wire-
drift-vs-collision split, stdlib `statfs`, first-create sequencing,
directory-hash golden vector, `dolt-argument-refused` — but the rev-6
adjudication (`d503d55`) found seven of rev-6's own new mechanisms
still unsafe, mislabeled, or under-specified, framed as a bounded
"bounded maintenance/trust fold" that does **not** reopen §1's
authority/scope decisions or §2's goals: the first-create local-ignore/
untracked gate was written to run against the **nearest existing
ancestor** for *every* check, including the ignore/untracked gate,
when only `statfs` genuinely requires an already-existing path — the
ignore/untracked checks are pathname checks and can and must target
the actual, intended (possibly not-yet-existing) leaf directory
(§7.1 fix, task 1); process-group termination cancelled escalation
"the moment `cmd.Wait()` observes the group has already exited," which
reaps the process-group leader based on the **direct child's** exit
alone and does not independently prove every descendant sharing the
group is also gone, permitting the kernel to recycle the PGID for an
unrelated process while a rogue descendant in the old group is still
alive (§6.4 fix, task 2); the binary trust pin hashed the **resolved
pathname** both before and after invocation, but never bound the hash
check to the *exact bytes actually executed* — a swap between the
pre-invocation hash and `cmd.Start()`'s own internal path lookup was
never fully closed by re-hashing the same pathname a second time
(§6.1 fix, task 3); a load-time mismatch between a `resources.json`
entry's own recorded `resource_id` and its freshly recomputed value
was folded into the same `resource-id-collision` outcome as a genuine
two-distinct-declarations collision, conflating "this file was
hand-edited/corrupted" with "two different declarations happen to
hash to the same 12-hex prefix" (§4 fix, task 6); there was no
operation to update a Dolt resource's trust pin without discarding its
`resource_id`, `current.json` entry, and capture history — because
`binary_sha256` was an ordinary `args` key participating in the ID
hash, re-pinning trust after a Dolt upgrade orphaned the resource's
entire identity and history (§6.1/§13 fix, task 4); `Statfs_t.Type`'s
width and signedness vary across Linux architectures, and comparing an
unnormalized value against untyped numeric constants is not portable
(§7.2 fix, task 7); and a handful of historical rev-5 AC-numbering and
review-pass-count statements had drifted from the actual current
counts (§0.1/§14.1 fix, task 9). Rev-7 is a **bounded maintenance/
trust fold** — it does not touch §1/§2's scope, §5.2/§5.3's closed
resource kinds, or the separate-manifest/no-lifecycle-authority/
Dolt-optional/Git-only-replay decisions every prior revision has
preserved. §4, §6.1, §6.2, §6.4, §7.1, §7.2, §12.1, §12.3, §13 are
revised in place; §3, §9.1, §9.2, §14, §14.1, §15 are updated for the
private-copy-execution/trust-repin/corruption-taxonomy/statfs-width
corrections. **Preserved** because every review across all eight
revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.
- No tracked artifact ever contains a raw file byte, raw adapter
  stdout, or any content copied verbatim from a scanned source
  (ADR-027 D3); scanning happens strictly in bounded in-process memory,
  never pre-persisted to a scratch file (task 2).

Everything below this point folds rev-7's corrections into the
standalone design; §0.1 lists the new citations grounding those
corrections (appended to the existing Claims Audit table). Rev-6's
own `C1`–`C34` remain correct and are not repeated (see
`docs/handoff/HISTORY.md`/git history for that table).

**Historical context — rev-6's own fold summary** (superseded by the
above as the *current* state, preserved verbatim below for
continuity; its internal §0.1–§0.4 subsection numbers are the ones
this document's cross-references still use, now carrying rev-7's
additions appended to them):

Rev-5 (`7f653da`) redesigned the build-tag precision, filesystem
preflight, `db_path` honesty, output-cap-as-refusal, `WORKING`/`STAGED`
refusal, full-SHA-256 `batch_id`, unordered-batch framing, and
directory-mode hashing, but the rev-5 adjudication (`b312e4a`) found
rev-5's own new mechanisms still unsafe or under-specified in eight
concrete places, framed as a bounded "compatibility micro-fold" that
does **not** reopen §1's authority/scope decisions or §2's goals: any
resolved `dolt` binary was accepted without a compatibility trust pin,
so a same-named but semantically different binary could silently
change what a tracked result means (§6.1 fix, task 1); a detected
post-exit `db_path` pathname replacement was diagnostic-only, logging
the mismatch but still publishing the batch, rather than refusing the
capture outright (§9.1 fix, task 2); process-group termination lacked
`SysProcAttr{Setpgid:true}`, so `SIGTERM`/`SIGKILL` could reach the
`tpatch` process's own process group (including its parent shell)
instead of only the spawned Dolt child and its descendants (§6.4 fix,
task 3); the 12-hex-truncated `resource_id` keyspace had no distinct-
payload collision refusal, so two different declarations that happened
to truncate to the same ID had no defined outcome (§13/§4 fix, task
4); `latest_batch_id` reintroduced chronological ("newest") language
into a design whose own rev-5 fold insists batches are an unordered
content-addressed set (§4/§7.3/§12.4 fix, task 5); an existing
batch file's byte-level drift from a freshly re-encoded candidate was
labeled a `SHA-256` collision without first canonicalizing and
comparing the **semantic** body, conflating presentation drift
(whitespace/key-order) with a genuine cryptographic collision (§7.3
fix, task 6); the filesystem preflight used `golang.org/x/sys/unix`
(an external dependency this project's own rule minimizes) with
Linux/macOS allow/deny lists that differed between the PRD and ADR,
included at least one invalid constant, omitted `overlayfs` (a common
container/CI filesystem), and did not fsync the first-created parent
directory of a scratch tree (§7.1/§7.2 fix, task 7/task 8); and the
directory `combined_hash` tuple never stated whether its hash
component was raw hex or `sha256:`-prefixed, and had no worked golden
vector (§5.1/§12.2 fix, task 9). Rev-6 is a **bounded compatibility
fold** — it does not touch §1/§2's scope, §5.2/§5.3's closed resource
kinds, or the separate-manifest/no-lifecycle-authority/Dolt-optional/
Git-only-replay decisions every prior revision has preserved. §6.1,
§6.4, §7.1, §7.2, §7.3, §9.1, §13 are revised in place; §3, §4, §5.1,
§6.2, §12.2, §12.3, §12.4, §14, §14.1, §15 are updated for the trust-
pin/collision/rename/hash-tuple corrections. **Preserved** because
every review across all seven revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.
- No tracked artifact ever contains a raw file byte, raw adapter
  stdout, or any content copied verbatim from a scanned source
  (ADR-027 D3); scanning happens strictly in bounded in-process memory,
  never pre-persisted to a scratch file (task 2).

Everything below this point folds rev-6's corrections into the
standalone design; §0.1 lists the new citations grounding those
corrections (appended to the existing Claims Audit table). Rev-5's
own `C1`–`C27` remain correct and are not repeated (see
`docs/handoff/HISTORY.md`/git history for that table).

**Historical context — rev-5's own fold summary** (superseded by the
above as the *current* state, preserved verbatim below for
continuity; its internal §0.1–§0.4 subsection numbers are the ones
this document's cross-references still use, now carrying rev-6's
additions appended to them):

Rev-4 (`ceda294`/`b7ddccb`) redesigned the lock protocol, publication
idempotency, `diff`'s content-reading honesty, and three Dolt
citations, but the rev-4 adjudication (`07eab8e`) found rev-4's own
new mechanisms still unsafe or under-specified in eight concrete
places: the `db_path` post-exit check compared the held descriptor
against **itself** rather than a freshly re-resolved pathname, making
the "detection" claim tautological (§9.1 fix, task 3); `//go:build
unix` is broader than this project's actual, tested
`ubuntu-latest`/`macos-latest` CI matrix and would silently compile on
POSIX-family targets (AIX, Solaris) with no `syscall.Flock`
portability guarantee (§7.2 fix, tasks 1); an unconditional `flock`
claim says nothing about network/shared filesystems, where advisory
locking may not provide real cross-client exclusion (§7.2 addition,
task 2); rev-4's local scratch-tree diagram incorrectly placed the
tracked batch/pointer temp files under the *local*, gitignored
scratch tree rather than beside their *tracked* destinations, breaking
the same-directory-rename invariant §7.3 itself depends on (§7.1 fix,
task 4); the Dolt output cap was described as both "truncated" and
"refused," which are contradictory, and `bytes.Buffer` has no
built-in bound (§6.4/§8.1 fix, task 5); `WORKING`/`STAGED` were
accepted as `from`/`to` values even though they load Dolt's own
`dolt_ignore` table, which can silently omit the mandatory `table`
before the PK-change hard-error logic ever fires — a second,
independent silent-omission path this design otherwise works hard to
close (§6.2 fix, task 6); `batch_id`'s 12-hex (48-bit) truncation is
collision-prone for a scheme whose own collision outcome is a fatal
integrity error, not a display convenience (§7.3/§12.3/§13 fix, task
7); and "one batch per invocation" language could be misread as
implying content-addressed batches carry a chronological ordering,
which they do not (§4/§7.3 fix, task 8). Directory `mode` was also
tracked per-file but never folded into `combined_hash`, so a
chmod-only change was invisible to both the hash and to `diff` (§5.1/
§12.2 fix, task 9). Rev-5 is again a substantial, targeted rewrite of
these eight-plus-one mechanisms — §6.2, §6.4, §7.1, §7.2, §8.1, §9.1
are revised in place; §4, §5.1, §12.3, §14, §14.1, §15 are updated for
the batch-ID/mode/ordering corrections. **Preserved** because every
review across all six revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.
- No tracked artifact ever contains a raw file byte, raw adapter
  stdout, or any content copied verbatim from a scanned source
  (ADR-027 D3); scanning happens strictly in bounded in-process memory,
  never pre-persisted to a scratch file (task 2).

Everything below this point folds rev-5's corrections into the
standalone design; §0.1 lists the new citations grounding those
corrections (appended to the existing Claims Audit table). Rev-4's
own `C1`–`C27` remain correct and are not repeated (see
`docs/handoff/HISTORY.md`/git history for that table).

**Historical context — rev-4's own fold summary** (superseded by the
above as the *current* state, preserved verbatim below for
continuity; its internal §0.1–§0.4 subsection numbers are the ones
this document's cross-references still use, now carrying rev-5's
additions appended to them):

Rev-3 (`151a50e`, including the check-ignore citation addendum)
redesigned rev-2's Dolt argv, ADR-027 compliance, publication,
path-safety, and Git-gate mechanisms, but the rev-3 adjudication
(`4d9dd21`) found rev-3's *execution contracts* still unsafe or
under-specified in ten concrete places, most centrally: rev-3's
temp-directory/`owner.json`/PID+process-start lock protocol was still
ABA-prone (a quarantine-then-retry sequence a concurrent process could
observe mid-transition, and `ps -o lstart=` parsing/liveness is itself
a fragile, shell-out-based mechanism) where a kernel-released advisory
lock removes the entire ownership-verification problem class (§7.2
full redesign, task 1); the batch-publication idempotency check
compared the **canonical hash-input bytes** (which exclude `batch_id`
and use no pretty-printing) against the **on-disk file bytes** (which
include `batch_id` and use indentation), so an identical retry could
never match and idempotency was broken by construction (§7.3 fix, task
5); `db_path` was gated by the same ancestor-symlink walk as an
`ignored-file` selector but then handed to `exec.Cmd.Dir` as a bare
pathname, which is not descriptor-bound — the PRD did not say so
honestly (§9.1 addition, task 3); `remove`/`clear` were described as
pruning `current.json`'s live index, which is not "declaration-only"
and contradicts §4's claim that `resources.json` is the sole thing
those verbs touch (§3/§4/§7.3/§12.5 fix, task 5/task 9); `diff`
claimed to recompute "lightweight metadata... without opening file
content," which contradicts §5.1's own hash-recomputation requirement
one paragraph away, and separately over-claimed a "single consistent
point-in-time snapshot" for a sequential, not atomic, directory scan
(§5.1 fix, task 9); and three Dolt-citation precision gaps the
adjudication asked to close specifically: the "native JSON boolean"
claim cited the SQL **schema/writer-type** declaration rather than the
**row constructor** that actually proves a Go `bool` is what gets
handed to the row (task 12), the closed 4-value `diff_type` enum was
cited via the **const block** (which also contains a fifth,
never-assigned `DiffTypeAll` filter-only value) rather than via the
four **exact assignment lines** that are the only places any row's
`diff_type` field is ever set (task 12), and the "single-table path"
citation for the hard PK-change error should be phrased specifically
as "the 3-argument form" since `table` is mandatory and no other form
is ever emitted (task 12). Rev-4 is again a substantial, targeted
rewrite of the affected mechanisms — §7.2 is rewritten in full; §0,
§3, §4, §5.1, §6.2, §6.3, §7.1, §7.3, §7.4, §9.1, §10, §11, §12, §14
are revised in place. **Preserved** because every review across all
five revisions has agreed it is sound:

- Resources live in a **separate** manifest per feature — never
  inside the canonical patch, never touching `apply-recipe.json` or
  unapply/lifecycle state.
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external,
  read-only, externally-located tool.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`.

Everything below this point is the standalone rev-4 design; §0.1–§0.4
map each rev-3 finding to its resolution. Rev-3's own `C1`–`C24` remain
correct and are not repeated (see `docs/handoff/HISTORY.md`/git history
for that table); rev-4 adds `C25`–`C27`, grounded in a second direct
source read of the pinned Dolt commit performed specifically for this
fold's citation-precision requirements (row-constructor evidence,
exact assignment-line citations, and the stdout-whitespace shape),
plus the supervisor-relayed `dolt_diff_summary` schema/read-only/
literal-argv facts already folded in the accompanying supervisor
message.

### 0.1 Claims Audit (rev-4 + rev-5 + rev-6 + rev-7 + rev-8 + rev-9 + rev-10 additions)

Rev-1's `C1`–`C10`, rev-2's `C11`–`C16`, and rev-3's `C17`–`C24`
(citation corrections for `featureCmd`, the lexical-only safety
helpers, `--no-index` ignore semantics, the existing session-redaction
shape, `ExitCodeError` call sites, the `feature claim` CLI precedent,
ADR-027 D1/D3, tracked-vs-untracked research docs, real Dolt CLI
flags, `RemoveClaim`'s line range, `EnsureLocalIgnoreContract`'s exact
scope, `O_NOFOLLOW`'s availability, the `dolt_diff_summary` column
schema/`IsReadOnly`/argument-form detail, the invalid
`--literal-pathspecs check-ignore` invocation, the `./`-prefix
colon-magic workaround, `WORKING`/`STAGED` source support (now
explicitly refused as a design choice, not accepted), the PK-set-change
hard-error/nonexistent-table/closed-`diff_type`-enum facts, the
`{"rows":[...]}`/`{}` JSON envelope shape, and the `".."`
argument-count-parsing hazard) all remain correct and are not repeated
here — see the rev-1/rev-2/rev-3 text preserved in
`docs/handoff/HISTORY.md`/git history for that table. Rev-4 adds:

| # | Claim | Citation | Why this changes rev-1 |
|---|-------|----------|-------------------------|
| C11 | `dolt diff --name-only` combined with `--schema`/`--data` and `--filter=` is **not** how the pinned Dolt source expresses per-table schema/data change classification; the source-verified read-only interface is the `dolt_diff_summary(from, to[, table])` table function, queried over `dolt sql -r json -q "..."`, returning exactly `{from_table_name, to_table_name, diff_type, data_change, schema_change}` per row | Supervisor source check against `dolthub/dolt` at commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9`: `go/cmd/dolt/commands/diff.go` (synopsis, `--schema`/`--data`, `--result-format`), `go/cmd/dolt/commands/sql.go` (`-q`/`--query`, `-r`/`--result-format json`), `go/cmd/dolt/commands/version.go` | rev-1 **error**: rev-1's three-invocation `--name-only --filter={added,dropped,modified}` design combined flags in a way the source does not support as a single coherent invocation, and could not detect renames or represent "both schema and data changed" for one table in one classification. §6 replaces this entirely with the one-query `dolt_diff_summary` design. |
| C12 | `dolt version` is a real subcommand that can, depending on build/config, perform a network update check and read/write files under the resolved `HOME`; it is not a safe no-side-effect probe | Rev-2 threat-modeling of running an arbitrary resolved executable literally named `dolt` with an unconstrained inherited environment, not a specific pinned-commit source citation (no claim here about `dolt version`'s exact internal behavior beyond "runs arbitrary code with inherited env/network access," which is true of any executed binary in v1's threat model) | rev-1 ran `dolt version` as a "probe" step with the invoking process's inherited environment. §6.1 removes this probe entirely; tool identity is now a static file fact (executable basename + `SHA-256` of the resolved binary's bytes), never a code-execution result, and every actual invocation runs with a minimal, non-inherited scratch environment (§6.4). |
| C13 | `internal/workflow/session_ignore.go`'s `EnsureLocalIgnoreContract(repoRoot, resolvedPath)` verifies the path is inside the worktree and that `gitutil.IsPathIgnored` (`--no-index`) reports it ignored; it does **not** independently verify the path is untracked | `internal/workflow/session_ignore.go:138-194` (`EnsureLocalIgnoreContract` body, full function span, corrected in rev-6 from a prior `138-175` citation that stopped mid-function before the `ignored` check itself) | New for rev-2: §10.3 reuses this exact function for the ephemeral-scratch root (task 7's "do not invent a second ignore mechanism") but layers the same tracked-file gate used for `ignored-file` selectors (§5.1) on top, since `EnsureLocalIgnoreContract` alone does not close the `--no-index` gap for the scratch root either. |
| C14 | Go's `os.OpenFile` accepts `syscall.O_NOFOLLOW` on Unix build targets (`darwin`/`linux`), which causes the open to fail with `ELOOP` if the **final** path component is a symlink; there is no portable stdlib/syscall equivalent that also binds every **ancestor** directory component against races (no `openat2`/`RESOLVE_NO_SYMLINKS` wrapper in the Go standard library) | Go standard library `os`/`syscall` package documentation (`O_NOFOLLOW` is a documented, platform-gated `syscall` constant; `openat2` has no stdlib wrapper as of the Go versions this project targets) | New for rev-2: §9.1 uses `O_NOFOLLOW` as one real, available hardening measure for the final component and is explicit that ancestor-component TOCTOU is closed by *refusing any symlink component at all* (a stat-time check) rather than by any stronger descriptor-bound guarantee stdlib cannot provide (task 5: "state TOCTOU residual honestly ... do not claim impossible sandbox"). |
| C15 | `dolt_diff_summary`'s five columns are typed and **non-null**: `from_table_name` (`LongText`), `to_table_name` (`LongText`), `diff_type` (`Text`), `data_change` (`Boolean`), `schema_change` (`Boolean`); the function itself reports `IsReadOnly() == true`; accepted invocation forms are the 2-arg `(from, to)` and 3-arg `(from, to, table)` shapes this PRD already uses, plus dot-range forms (e.g. a single `"from..to"`-shaped argument) this PRD deliberately does not use; Dolt's own internal Go usage of the function queries it with `select * from dolt_diff_summary(?, ?)` and sorts results by `ToName` in application code, rather than an explicit `SELECT <columns> ... ORDER BY` at the SQL layer | Supervisor source check against `dolthub/dolt` at commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9` (table-function schema/column typing, `IsReadOnly()`, accepted argument forms, and Dolt's own internal query/sort usage) | Confirms — does not correct — §6.2's design: the non-null column guarantee is why `result.tables[]` entries in every tracked wire example (§12.2/§12.3) never carry a null field; the read-only confirmation reinforces C11's "external, read-only tool" framing; this PRD deliberately does **not** adopt Dolt's own internal `select *` + application-side sort pattern, and instead binds every column by explicit name and applies an explicit SQL `ORDER BY from_table_name, to_table_name` (§6.2), so tracked output does not silently reorder or gain/lose a field if a future Dolt version changes the table function's positional column order; dot-range argument forms are noted as existing but out of scope for v1's exact 2-/3-arg argv template. |
| C16 | `ADR-027-capture-context-privacy-boundary.md` D3 states verbatim: "Local private buffers may keep only the redacted or hashed form; this ADR does not authorize a tpatch-managed raw transcript archive." | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-170` (D3 section), exact quoted sentence at `:168-170` | Directly grounds §7.1/§0.2's "no persistent raw bodies anywhere, ephemeral-scratch-only" design in ADR-027's own binding language, not just this PRD's inference from D1–D6's committed/local split "in spirit" (as rev-2's original §0 fold summary put it) — D3 is explicit and unconditional: a persistent raw local archive of any kind, opt-in or not, is not authorized without an ADR that supersedes it (§2's new non-goal). |
| C17 | `git check-ignore` does not accept a pathspec at all — its positional arguments are plain `<pathname>` values (per `git-check-ignore(1)`'s synopsis and option list, which has no `--literal-pathspecs`/pathspec-magic-related option); `git --literal-pathspecs check-ignore -q --no-index -- <path>` is therefore not a valid invocation and fails immediately with `fatal: <path>: pathspec magic not supported by this command: 'literal'` (exit `128`), never reaching the ignore check at all, and this holds regardless of whether the argument itself looks like pathspec-magic (a plain glob-shaped argument such as `docs/*.md` fails identically, not only colon-prefixed ones) | Empirically verified against installed Git (`git --literal-pathspecs check-ignore -q --no-index -- 'a/:weird.txt'` → `fatal: a/:weird.txt: pathspec magic not supported by this command: 'literal'`, exit 128; supervisor-independently reconfirmed with `git --literal-pathspecs check-ignore -q --no-index -- 'docs/*.md'` → identical fatal exit 128 with the identical error text) and `git-check-ignore(1)`'s documented option list | rev-2 **error**: §10.1/§5.1 required `--literal-pathspecs` on the `check-ignore` invocation; every `ignored-file` `add`/`capture` would have failed with a fatal Git error before ever checking ignore status. §10.1/§5.1 are rewritten to reuse the **existing**, already-correct `gitutil.IsPathIgnored` invocation shape (`git check-ignore -q --no-index -- <pathname>`, no `--literal-pathspecs`) unchanged. The fatal outcome is independent of the argument's shape — this PRD does not rely on `--literal-pathspecs` ever succeeding for any `check-ignore` argument, glob-shaped or not. |
| C18 | `check-ignore`'s plain pathname argument **does** parse a leading `:` for pathspec magic (unlike `*`/`?`/`[]`, which are inert literal characters to this command with no glob/fnmatch expansion): a colon-prefixed name using a magic keyword this command does not support (e.g. `:(literal)...`, `:(glob)...`, `:!...`/`:^...` exclude) is a **fatal** error (exit `128`), while `:/...` ("top") magic is silently accepted without error; prefixing any selector beginning with `:` with `./` (e.g. `./:weird.txt`, `./:(literal)weird.txt`) disarms all colon-magic parsing (the argument no longer begins with a bare `:` byte) and is instead treated as a literal pathname — resolving to the identical on-disk file if it exists, or exit `1` (no match, not fatal) if it does not, never a fatal error | Empirically verified: `git check-ignore --no-index -- ':(glob)sub/*.txt'` → fatal (exit 128); `git check-ignore --no-index -- ':!exclude.txt'` → fatal (exit 128); `git check-ignore --no-index -- ':/topmagic.txt'` → exit 0, no error; `git check-ignore --no-index -- './:(glob)sub/*.txt'` → exit 0, treated as the literal filename; `*`/`?`/`[]` in a plain (non-colon-prefixed) pathname never trigger wildcard matching (`git check-ignore --no-index -- 'sub2/file*.txt'` does not match a differently-named ignored file); supervisor-independently reconfirmed `:(literal)...` as a second concretely-fatal magic keyword (`git check-ignore -q --no-index -- ':(literal)name'` → fatal, exit 128) and confirmed the `./`-prefixed form of that same keyword — `git check-ignore -q --no-index -- './:(literal)name'` — is treated purely as a pathname (exit `1`, no match, no fatal), matching this PRD's `./`-prefix rule exactly | New for rev-3: §5.1/§10.1's `check-ignore` invocation now prefixes any selector whose first byte is `:` with `./` before passing it as the pathname argument, closing an ambiguity C17's fix would otherwise reintroduce for colon-shaped selectors specifically (the existing `ls-files --error-unmatch` gate already handles this safely via `--literal-pathspecs`, which `check-ignore` cannot accept). This rule is now confirmed against two independently-fatal magic keywords (`:(glob)`, `:(literal)`) plus the two exclude forms (`:!`, `:^`), and the `./`-prefix's safe non-fatal (exit-0-or-1) outcome is confirmed for both `:(glob)` and `:(literal)` inputs, not merely one — this PRD's rule ("any leading `:` byte gets the `./` prefix, unconditionally") does not depend on enumerating every magic keyword Git supports, so this remains a closed, keyword-agnostic fix rather than a per-keyword allowlist. |
| C19 | `dolt_diff_summary`'s `from`/`to` arguments accept the literal strings `"WORKING"`/`"STAGED"` (exact case, not case-insensitively) at the source level — rev-1/rev-2 left this explicitly unconfirmed; it is now source-confirmed at the pinned commit | `go/libraries/doltcore/doltdb/doltdb.go:51-52` (`Working = "WORKING"`, `Staged = "STAGED"` constants); `go/libraries/doltcore/sqle/dsess/session.go:1022-1031` (`DoltSession.ResolveRootForRef` special-cases an exact match on either literal string before falling through to `doltdb.NewCommitSpec`); `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff.go:378-403` (`loadDetailsForRefs`/`resolveCommitStrings` route both `from` and `to` through `ResolveRootForRef`), commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9` | Resolves rev-2's §6.2/§15 open question as a **source fact**: `WORKING`/`STAGED` are indeed accepted by upstream Dolt at this literal-string level. Rev-5 corrects rev-4's *design* conclusion drawn from this fact — §6.2 no longer states this PRD's own `from`/`to` **accept** these values; it now **explicitly refuses** them (case-insensitively) because either ref bypasses the mandatory-`table` PK-change hard-error guarantee's sibling concern, `dolt_ignore`-gated silent table omission (see §6.2's refusal rationale) — the capability existing upstream and this design choosing not to expose it in v1 are two separate, non-contradictory statements. |
| C20 | A hard hard-error outcome for a primary-key-set change between `from` and `to` on the requested table is source-confirmed, and is conditional on a `table` argument being supplied: `getSummaryForDelta`'s `shouldErrorOnPKChange` parameter is `true` only for the single-table query path (`tableNameExpr != nil`); the whole-database (no `table`) query path passes `false` and silently omits the affected table's row (with a warning, not an error) instead | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:299-321` (single-table call site, `shouldErrorOnPKChange=true`, line 311) vs `:324-341` (multi-table/whole-db loop, `shouldErrorOnPKChange=false`, line 334); `:346-365` (`getSummaryForDelta`'s branch); the wrapped sentinel is `diff.ErrPrimaryKeySetChanged` (`"primary key set changed"`, `go/libraries/doltcore/diff/diff_stat.go:31`), error text `"failed to compute diff summary for table %s: %w"` | Directly grounds task 6/task 8's "require `table` in v1 ... so PK-set changes fail rather than silently omit": this PRD's mandatory-`table` decision (§5.3/§6.2) is not merely a simplicity choice, it is the specific argument shape that routes a PK-set-change into Dolt's own hard-error path instead of its own silent-omission path. §6.2/§14 document the resulting `dolt-query-error` refusal class explicitly. |
| C21 | A `table` argument naming a table that exists in **neither** `from` nor `to` yields zero rows (not an error), a third, distinct outcome from C20's hard error and from a `dolt_ignore`-matched table's zero-row outcome | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:347-350` (`getSummaryForDelta`'s early `return nil, nil` when all of `FromTable`/`ToTable`/`FromRootObject`/`ToRootObject` are nil and neither name carries `diff.DBPrefix`) | Grounds §6.2's "table never existed" first-capture case and §14's `AC` for it with an exact source citation rather than an inferred "behavior not independently re-derived" hedge (rev-2's phrasing for this case). |
| C22 | `dolt sql -r json` wraps a nonempty result as `{"rows": [...]}` (a single top-level key, `"rows"`) and emits the literal, distinct 2-byte string `{}` for a zero-row result — there is **no** `"schema"` key in either case. Rev-2's claim of a `"schema"`-key-bearing envelope was community/docs-corroborated but not source-verified, and is now corrected | `go/libraries/doltcore/table/typed/json/writer.go:37-38` (`jsonHeader = `{"rows": [`` / `jsonFooter = `]}``), `:56-58`,`:62-64` (doc comments: "encodes rows as a single JSON object with a single key: \"rows\""); `go/cmd/dolt/commands/engine/sql_print.go:110-113` (`FormatJson` case constructs this writer), `:147-149` (the zero-row `{}` case is written directly by the caller — `iohelp.WriteLine(cli.CliOut, "{}")` — not by the row writer, precisely when `numRows == 0`) | rev-2 **error**: §6.3's parser assumed a `"schema"` key existed alongside `"rows"` and did not define the zero-row shape at all. §6.3/§6.2 are rewritten: the parser recognizes exactly two valid top-level shapes (`{"rows":[...]}` or `{}`), rejects any other top-level shape (missing/extra/renamed key) as a fatal parse error, and `{}` maps deterministically to an empty `tables: []` result. |
| C23 | `diff_type` has a closed, source-confirmed 4-value string enumeration — `"added"`, `"modified"`, `"renamed"`, `"dropped"` — contrary to rev-2's "not independently confirmed against a guessed closed set" hedge; for a `"dropped"` row `to_table_name` is the empty string `""` (not omitted, not `null`), and for an `"added"` row `from_table_name` is `""`, because `doltdb.TableName{}`'s zero value stringifies to `""` and `GetSummary` only populates the applicable side | `go/libraries/doltcore/diff/table_deltas.go:46-49` (`DiffTypeAdded`/`DiffTypeModified`/`DiffTypeRenamed`/`DiffTypeDropped` constants), `:716-733` (asymmetric `FromTableName`/`ToTableName` population for drop/add), `:735-745` (rename populates both, differing), `:747-760` (modify populates both, same name); `go/libraries/doltcore/doltdb/root_val.go:797-800` (`TableName.String()` zero-value behavior) | §6.2/§12.2 now document the closed 4-value set and the empty-string convention for add/drop rows precisely, while still tracking `diff_type` **verbatim** rather than validating against it (forward-compatible if a future Dolt version adds a 5th value) — a stricter, better-cited version of rev-2's existing "opaque string, not hardcoded" posture, not a reversal of it. |
| C24 | `dolt_diff_summary`'s own argument-count validation inspects the literal SQL-expression string of its **first** argument for a `".."` substring to choose between the dot-range (1–2 args) and explicit-`from`/`to` (2–3 args) parse branches; a `from` value that legitimately contains the literal substring `".."` breaks this design's explicit 3-argument (`from, to, table`) invocation at the SQL layer itself (misrouted argument-count validation, `sql.ErrInvalidArgumentNumber`), independent of and in addition to this design's own choice never to use dot-range syntax | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:220-238` (`WithExpressions`: `strings.Contains(exprs[0].String(), "..")` branches the accepted-argument-count check) | Upgrades task 6's "`from`/`to` reject `..`" from a defense-in-depth policy choice (rev-2 already refused backslash/control bytes similarly) to a real Dolt-compatibility requirement: refusing any value containing `".."` (§6.2) is not just prudent, it prevents a legitimate-looking value from silently breaking this design's fixed 3-argument invocation shape. |
| C25 | The claim "Dolt's real, source-confirmed JSON writer always emits native JSON booleans for `BOOLEAN`-typed columns" is best grounded in the **row constructor** that builds each output row from a `TableDeltaSummary`, not the SQL schema/writer-type declaration alone: `getRowFromSummary` builds `sql.Row{ds.FromTableName.String(), ds.ToTableName.String(), ds.DiffType, ds.DataChange, ds.SchemaChange}`, passing the struct's native Go `bool` fields (`DataChange bool`, `SchemaChange bool`, confirmed at `table_deltas.go:83-90`) directly into the row with no intermediate string/int conversion, which is what the JSON writer then serializes as a native JSON boolean | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:457-464` (`getRowFromSummary`, the row constructor) plus `go/libraries/doltcore/diff/table_deltas.go:83-90` (`TableDeltaSummary` struct field types) | rev-3's C15/§6.2 cited only the SQL schema declaration (`dolt_diff_summary.go:48-54`, `Boolean` column type) for this claim — a schema type alone does not prove what a *particular code path* actually writes into the row. §6.3 now cites the row constructor as the primary evidence for "native JSON boolean, no coercion," retaining the schema citation only for the non-null-column claim it does support. |
| C26 | `diff_type`'s closed 4-value enumeration is more precisely evidenced by the four **exact assignment lines** inside `GetSummary` — the only places any row's `DiffType` field is ever set — than by the `const` block alone, which also declares a fifth value, `DiffTypeAll = "all"`, that exists only as a caller-side **filter** argument to a different function and is never itself assigned to any row's `DiffType` field (confirmed by an exhaustive grep of every `DiffType` assignment in this file: zero occurrences of `DiffTypeAll` outside the `const` block itself) | `go/libraries/doltcore/diff/table_deltas.go:713-761` (`GetSummary`): `:722` `DiffType: DiffTypeDropped`, `:733` `DiffType: DiffTypeAdded`, `:745` `DiffType: DiffTypeRenamed`, `:760` `DiffType: DiffTypeModified` — the complete, exhaustive set of assignment sites; `:45-51` (`const` block, including the unused-for-this-purpose `DiffTypeAll = "all"`) | rev-3's C23/§6.2 cited only the `const` block (`table_deltas.go:46-49`), which a reader could misread as implying 5 possible row values. §6.2 now cites the four assignment lines directly and notes `DiffTypeAll` is a filter-only value this design's fixed `dolt_diff_summary` query never emits and never needs to recognize as a row value. |
| C27 | The pinned commit's own `dolt sql -q ... -r json` one-shot (non-interactive) invocation path emits trailing whitespace **beyond** the JSON body itself in both the nonempty- and zero-row cases: the JSON row writer's `Close` writes only the literal footer `]}` with no trailing newline of its own (`writer.go:243-249`); `execSingleQuery` (the one-shot `-q` code path, `sql.go:461-470`) calls `PrettyPrintResults`, which is `prettyPrintResultsWithSummary(..., PrintNoSummary, ...)` (`sql_print.go:59-61`) — so no "N rows in set" summary line is appended for this invocation shape — but regardless of row count, `sql_print.go`'s final, unconditional `case FormatJson, ...: return iohelp.WriteLine(cli.CliOut, "")` (`:168-170`) appends exactly one more `"\n"` after everything else, and `WriteLine` (`iohelp/write.go:66-68`) always appends a trailing `"\n"` to whatever it's given; the zero-row case additionally writes `iohelp.WriteLine(cli.CliOut, "{}")` (`sql_print.go:148-149`) — itself already `"\n"`-terminated — before that same final blank-line write, so the real zero-row stdout is `"{}\n\n"`, not a bare 2-byte `"{}"` | `go/libraries/doltcore/table/typed/json/writer.go:243-249` (`Close`, footer write, no added newline); `go/cmd/dolt/commands/sql.go:452-470` (`execSingleQuery`, the `-q` one-shot path, calls `PrettyPrintResults` not `PrettyPrintResultsExtended`); `go/cmd/dolt/commands/engine/sql_print.go:55-61` (`PrintNoSummary` is what `PrettyPrintResults` passes), `:148-149` (zero-row `{}` write), `:168-170` (unconditional trailing blank-line write for `FormatJson`); `go/libraries/utils/iohelp/write.go:66-68` (`WriteLine` always appends `"\n"`) | Directly grounds task 4's "trim JSON surrounding whitespace before structural parse" requirement with real, cited evidence of trailing whitespace in captured stdout, rather than a purely defensive assumption: §6.3 now states the parser trims leading/trailing ASCII whitespace from the captured buffer before attempting to match either of the two valid top-level shapes, and that both the nonempty (`{"rows":[...]}\n`) and zero-row (`{}\n\n`) real outputs parse correctly under that rule. |
| C28 | `//go:build unix` is a real, documented Go build constraint that matches every POSIX-family `GOOS` value Go's toolchain recognizes under that tag (`aix`, `android`, `darwin`, `dragonfly`, `freebsd`, `hurd`, `illumos`, `ios`, `linux`, `netbsd`, `openbsd`, `solaris` — `illumos` and `solaris` are two distinct, separately-listed `GOOS` values, not synonyms, despite their shared lineage), not only `linux`/`darwin`; it explicitly does **not** match `js`/`wasip1` (`GOOS=js`/`GOOS=wasip1` are not part of the `unix` build-tag set at all, so no part of this correction is about excluding WASM targets specifically — they were never included) | Go standard library/toolchain documentation for build constraints (the `unix` build tag is defined as the union of the Unix-like `GOOS` values above, a strictly larger set than this project's tested platforms, and does not extend to the non-Unix `js`/`wasip1` targets) | rev-4 **imprecision**: this project's actual CI matrix (`.github/workflows/ci.yml:18-25`) tests only `ubuntu-latest`/`macos-latest`; a `unix`-tagged lock file would silently compile (and claim to work) on ten untested POSIX targets. §7.2 now uses the exact `//go:build linux \|\| darwin` constraint, with `//go:build !linux && !darwin` as the explicit, narrower fallback. |
| C29 | `flock(2)`'s advisory-lock guarantee is documented as applying to the local kernel's view of an open file description; POSIX and Linux/BSD manual pages do not guarantee `flock` semantics are honored identically by every network/shared filesystem client (NFS in particular has historically had inconsistent or client-local-only `flock` emulation depending on protocol version and mount options) | `flock(2)` manual page ("Advisory locks... ", the "NOTES" section on NFS behavior varying by kernel/mount configuration is a widely-documented caveat of this syscall, not specific to this project) | Grounds §7.2's new filesystem-contract preflight (rev-5): this design does not claim `flock` provides real mutual exclusion on every mount a `.tpatch/` directory could live on, and refuses known-risky/unrecognized filesystem types via `statfs` before creating the lock file at all. |
| C30 | `os/exec.Cmd`'s `StdoutPipe`/`StderrPipe` return `io.ReadCloser`s that must be fully drained (or the child can block writing to a full pipe); `cmd.Wait()` requires all reads from these pipes to complete before it is called, per the documented contract | Go standard library `os/exec` package documentation for `Cmd.StdoutPipe`/`Cmd.StderrPipe` ("It is thus incorrect to call Wait before all reads from the pipe have completed") | Grounds §6.4/§8.1's redesigned output-cap mechanism (rev-5): continuing to drain both pipes to completion after a cap-triggered kill (rather than abandoning the read) is required by this documented contract, not merely a defensive nicety. |
| C31 | `syscall.SysProcAttr` on `linux`/`darwin` has a `Setpgid bool` field; when `true`, the started child becomes the leader of a **new** process group whose PGID equals the child's own PID (distinct from `tpatch`'s own process group), and `syscall.Kill(-pgid, sig)` (a **negative** PID argument) delivers `sig` to every process in that group — this is standard POSIX `kill(2)` group-signal semantics, not a Dolt- or Go-specific behavior | Go standard library `syscall` package documentation for `SysProcAttr.Setpgid`/`Setpgid`+`Pgid` on Unix-like `GOOS` values; POSIX `kill(2)` manual page (negative `pid` argument targets a process group) | Grounds §6.4's `Setpgid`-based process-group isolation (task 3): without `Setpgid:true`, a spawned Dolt child inherits `tpatch`'s own process group, and a signal sent to "the group" would also reach `tpatch` itself (and any parent shell sharing that group) — `Setpgid:true` plus signaling the child's own (negative) PGID is what confines `SIGTERM`/`SIGKILL` to the Dolt invocation and its own descendants only. |
| C32 | Go's stdlib `syscall` package exposes `Statfs`/`Statfs_t` on both `linux` and `darwin` build targets with no external dependency: on `linux`, `Statfs_t.Type` is an integer filesystem-type magic number (`int64` on most architectures); on `darwin`, `Statfs_t.Fstypename` is a fixed-size byte array holding the filesystem type name as a NUL-terminated C string (e.g. `"apfs"`, `"hfs"`, `"nfs"`) | Go standard library `syscall` package (`Statfs`/`Statfs_t` are defined for `GOOS=linux` and `GOOS=darwin` with the field shapes above; no `golang.org/x/sys/unix` import is required to reach either) | Grounds task 7's "stdlib-only, no `x/sys` dependency" filesystem preflight (§7.2): rev-5's design imported `golang.org/x/sys/unix` for this same `Statfs` call, an unnecessary external dependency this project's own "minimal external Go dependencies" rule (`CLAUDE.md`) should avoid when the stdlib already exposes the identical syscall on both tested platforms. |
| C33 | The Linux kernel's public filesystem-magic-number header defines (among others) `EXT2_SUPER_MAGIC`/`EXT3_SUPER_MAGIC`/`EXT4_SUPER_MAGIC` as the single shared value `0xEF53`, `XFS_SUPER_MAGIC` as `0x58465342`, `BTRFS_SUPER_MAGIC` as `0x9123683E`, `TMPFS_MAGIC` as `0x01021994`, and `OVERLAYFS_SUPER_MAGIC` as `0x794C7630`; on the deny side, `NFS_SUPER_MAGIC` is `0x6969`, `CIFS_MAGIC_NUMBER` is `0xFF534D42`, `SMB2_MAGIC_NUMBER` is `0xFE534D42`, and `FUSE_SUPER_MAGIC` is `0x65735546`; there is no Linux `APFS_SUPER_MAGIC` constant at all (APFS is a Darwin-only filesystem with no Linux kernel driver/magic number in the mainline kernel) | Linux kernel `include/uapi/linux/magic.h` (the canonical, stable public header defining every `*_SUPER_MAGIC`/`*_MAGIC_NUMBER` constant used by `statfs(2)`'s `f_type` field) | Corrects rev-5's §7.2 allowlist, which cited `APFS_SUPER_MAGIC` as a Linux-reachable value ("via a Linux VM/container mounting one") — no such Linux kernel constant exists; APFS access from Linux (where it exists at all, e.g. via FUSE) would surface as a FUSE magic number, already on the deny list. §7.2's Linux allowlist/denylist now uses exactly the eight numeric constants above, byte-identical between this PRD and the companion ADR, and explicitly allows `overlayfs` (common under Docker/container CI). |
| C34 | Darwin's `statfs(2)`/`getfsstat(2)` family reports the filesystem type name as a short lowercase string in `Fstypename`; observed/documented values include `"apfs"`, `"hfs"`, `"tmpfs"`, `"nfs"`, `"smbfs"`, `"webdav"`, and FUSE-backed mounts (including `osxfuse`/macFUSE) surface their own registered type name (historically `"osxfuse"`, `"macfuse"` for newer installs) rather than a shared generic "fuse" string | Apple/BSD `statfs(2)` manual page and `sys/mount.h` (`Fstypename` is populated from the mounted filesystem's registered VFS name; FUSE-based filesystems on macOS register their own distinct type name rather than reusing a generic value) | Grounds §7.2's Darwin allow/deny lists (task 7): allow `"apfs"`, `"hfs"`, `"tmpfs"`; deny `"nfs"`, `"smbfs"`, `"webdav"`, and known FUSE-registered names (`"osxfuse"`, `"macfuse"`) as representative known-network/known-non-local entries, with every other, unrecognized `Fstypename` value failing closed exactly as the Linux side does for an unrecognized numeric magic. |

Rev-7 adds:

| # | Claim | Citation | Why this changes rev-6 |
|---|-------|----------|-------------------------|
| C35 | `syscall.Statfs_t.Type`'s width and signedness on Linux is **architecture-dependent**, not a single fixed type: `int64` on `linux/amd64` and `linux/arm64`, `int32` on `linux/386` and `linux/arm`, and `uint32` on `linux/s390x` — a raw comparison of `buf.Type` against an untyped numeric constant compiles differently (and can behave inconsistently at the boundary of the value range) depending on target architecture | Go standard library generated syscall type files: `syscall/ztypes_linux_amd64.go:118` (`Type int64`), `syscall/ztypes_linux_arm64.go:119` (`Type int64`), `syscall/ztypes_linux_386.go:118` (`Type int32`), `syscall/ztypes_linux_arm.go:119` (`Type int32`), `syscall/ztypes_linux_s390x.go:118` (`Type uint32`) — verified directly against the installed Go toolchain's `GOROOT/src/syscall` sources | rev-6 **imprecision**: §7.2 compared `buf.Type` against the allow/deny numeric constants without stating a normalized comparison type, which is architecture-fragile (a `linux/386`/`arm` build's `int32` `Type` could differ in overflow/sign behavior from the `int64` value on `amd64`/`arm64` for the same on-disk magic number, and `s390x`'s `uint32` is a third, unsigned shape). §7.2 now explicitly normalizes `fsType := uint32(buf.Type)` before comparison, and the allow/deny constants themselves are typed `uint32`, making the comparison identical across every architecture regardless of `Statfs_t.Type`'s own declared width on that target. |
| C36 | POSIX process-group semantics: a process group is not eligible for PGID reuse by the kernel until **every** process that is a member of that group has exited and been reaped — an implementation that calls `Wait()` on only the group's leader (the direct child) the instant that one process exits, while other descendants sharing the same PGID (e.g. a sub-process the leader itself forked) are still running, does not itself prove the group is empty; the leader's own exit status remaining unreaped for a bounded interval does not, by itself, prevent PGID reuse — reuse is gated on group membership going to zero, not on any single member staying unwaited | POSIX.1-2017 `wait`/`waitpid` and process group lifecycle description (`_exit`, "Process Groups" — a process group persists as long as it has at least one member; PGID values are not recycled by the kernel while any member process still exists, independent of whether a controlling process has called `wait` on any specific member) | Grounds task 1's rev-10 correction to §6.4 (re-attributed: rev-8/rev-9 partially applied this principle but only for the timeout/cap branch — rev-9's "leader exited on its own" branch called `Wait()` *before* the group-signal sequence, reopening exactly the PGID-reuse gap C36 describes). Rev-10's non-reaping `waitid`/`WNOWAIT` observer (C41) is the first design that keeps the leader unreaped through the **entire** `SIGTERM`→grace→`SIGKILL` sequence in **all three** trigger branches identically, so the escalation is never skipped or cancelled merely because the leader appears to have exited, in any branch — not a claim that this sequence itself *proves* PGID non-reuse is impossible in every kernel scheduling interleaving, only that this design never signals a PGID number the kernel could have already recycled. |
| C37 | Go's stdlib `io.TeeReader`/`io.MultiWriter` composed with `crypto/sha256` allow computing a streaming digest of bytes as they are copied from one `io.Reader` to an `io.Writer`, with no intermediate full-buffer materialization required beyond the copy's own internal buffer — a standard, dependency-free pattern for "hash while copying" | Go standard library `io`/`crypto/sha256` package documentation (`io.TeeReader(r, w)` returns a `Reader` that writes to `w` everything read from `r`; `io.Copy` combined with a `sha256.New()`-backed `io.Writer` target is the idiomatic streaming-hash-while-copying construction) | Grounds §6.1's rev-7 executed-binary-binding redesign (task 3): the resolved Dolt binary's bytes are streamed from the already-opened source descriptor into the private ephemeral copy file via exactly this `io.TeeReader`/`sha256.New()` composition, so the digest computed is provably the digest of the exact bytes written to the copy that is later executed — no separate "hash the file, then separately copy the file" two-pass design that could itself race a swap between the two passes. |

Rev-8 adds:

| # | Claim | Citation | Why this changes rev-7 |
|---|-------|----------|-------------------------|
| C38 | Linux `statfs(2)`'s `f_flags` field (surfaced by Go stdlib as `syscall.Statfs_t.Flags`, populated "since Linux 2.6.36") carries mount-option bits from a documented `ST_*` namespace shared with `statvfs(3)`, including `ST_NOEXEC` ("Execution of programs is disallowed on this filesystem"); the stable, long-standing numeric value of `ST_NOEXEC` in this namespace is `0x8` | `man7.org` `statfs(2)` manual page, "The f_flags field" section (documents `f_flags`'s availability since Linux 2.6.36 and lists `ST_NOEXEC` as a named bit; https://man7.org/linux/man-pages/man2/statfs.2.html); numeric value `0x8` independently cross-checked against a locally available vendored copy of the standard glibc/Linux `ST_*` constant block (`ST_RDONLY=1`, `ST_NOSUID=2`, `ST_NODEV=4`, `ST_NOEXEC=8`) | Grounds §6.1 task 6's Linux `noexec`-mount preflight for the capture-time private-copy scratch directory: `adapter-copy-noexec` refuses (exit 3) before the copy file is created when `buf.Flags&0x8 != 0`, using the same stdlib-only `syscall.Statfs_t` primitive §7.2 already established (C32), not a new dependency. |
| C39 | Darwin's `sys/mount.h` defines `MNT_NOEXEC` as `0x00000004` in the same mount-flags namespace `syscall.Statfs_t.Flags` (Go stdlib, `darwin` build target) surfaces | Installed macOS SDK header (`usr/include/sys/mount.h`, `#define MNT_NOEXEC 0x00000004 /* can't exec from filesystem */`) | Grounds §6.1 task 6's Darwin `noexec`-mount preflight: `adapter-copy-noexec` refuses (exit 3) when `buf.Flags&0x00000004 != 0`, the platform-specific bit-value counterpart to C38's Linux `ST_NOEXEC`, using the identical stdlib-only `Statfs_t.Flags` field already read for this project's other `statfs`-based preflights. |

Rev-9 adds:

| # | Claim | Citation | Why this changes rev-8 |
|---|-------|----------|-------------------------|
| C40 | Assigning a caller-created `os.Pipe()` write end directly to `exec.Cmd.Stdout`/`Cmd.Stderr` (an `*os.File`, not `Cmd.StdoutPipe()`/`Cmd.StderrPipe()`) means Go's `os/exec` package neither owns nor auto-closes that file: `Cmd.Wait()` in this configuration blocks only on the underlying `wait4`/`waitid` reap of the **direct child's own PID** and returns as soon as that child exits, **independently of whether any other process (e.g. a descendant that inherited the same pipe write-end fd) still holds the pipe open** — this is a materially different, and here more useful, contract than `Cmd.StdoutPipe()`/`Cmd.StderrPipe()`'s documented "must fully drain before `Wait()`" coupling (C30), which applies only to those two specific accessor methods, not to a raw `*os.File` assigned directly | Empirically verified on this project's `darwin` development host (Go stdlib `os/exec`/`os` packages, no third-party dependency): a child process that itself backgrounds a grandchild sharing its stdout fd and then exits immediately causes `cmd.Wait()` to return in ~4ms, while a concurrent read of the pipe's read end only reaches `io.EOF` ~3s later, once the grandchild (still holding the write end) itself exits — proving `Wait()`'s completion is decoupled from pipe-EOF when the pipe is caller-owned rather than `Cmd`-owned. A second empirical check confirms `syscall.Kill(-pgid, sig)` (`pgid` = the already-`Wait()`-reaped leader's own former PID) still successfully reaches a surviving descendant sharing that PGID, consistent with C36's "group persists while any member exists" citation | Grounds §6.4/D5's retained caller-owned-pipe design (task 2, unchanged from rev-9): this claim establishes only that `cmd.Wait()`'s completion is decoupled from pipe drain, and remains true and useful for that narrower purpose in rev-10 — it is **not**, on its own, grounds for calling `cmd.Wait()` as soon as leader-exit is detected (that was rev-9's actual defect, corrected by C41/task 1); the decoupling is still valuable because it means the final `cmd.Wait()` call, whenever it is made, never itself blocks on a lingering descendant's pipe hold. |
| C41 | POSIX `waitid()` accepts a `WNOWAIT` option flag that, combined with `WEXITED`, reports a terminated child's status **without** reaping it — the child remains in a waitable (zombie) state after the call returns, so its PID (and, for a process-group leader created via `Setpgid:true`, its PGID, which the kernel assigns equal to the leader's own PID) is not released for reuse; a subsequent `waitid()`/`wait()`/`waitpid()` call is required to actually reap it | POSIX.1-2017 `waitid()` specification ("If `WNOWAIT` is set... the state of the child shall not be affected" — the child remains available for a later wait call); Go's `syscall` package exposes `syscall.SYS_WAITID`, `syscall.WEXITED`, and `syscall.WNOWAIT` as per-`GOOS`-resolved constants on both `linux` and `darwin` (no `golang.org/x/sys` import required — this project's own stdlib-only rule, §7.2/C28), though no `syscall.Waitid()` Go-level wrapper function exists, requiring a raw `syscall.Syscall6(syscall.SYS_WAITID, uintptr(1 /* P_PID */), uintptr(pid), uintptr(unsafe.Pointer(&buf[0])), uintptr(syscall.WEXITED\|syscall.WNOWAIT), 0, 0)` call (`Syscall6`, not `RawSyscall6`, so the call properly yields the goroutine's OS thread to the Go scheduler while blocked). Empirically verified on this project's `darwin` development host: a raw `waitid(P_PID, childPID, ..., WEXITED\|WNOWAIT)` call blocks until the child exits, returns success, and a repeated call to the same syscall against the same (still-unreaped) PID also succeeds instantly — proving the first call did not consume/reap the child; a subsequent `cmd.Wait()` on the same child still correctly reaps it and reports its real exit code. A separate source-shape/cross-compile test (build-tagged `//go:build linux` and `//go:build darwin` constant files, plus a shared `//go:build linux \|\| darwin` invocation file) was built successfully for `darwin/arm64` (native), `linux/amd64` (cross), and `linux/arm64` (cross) with `go build`, confirming the numerically-different per-platform `WEXITED`/`WNOWAIT` constant values (`darwin`: `WEXITED=0x4`, `WNOWAIT=0x20`; `linux`: `WEXITED=0x4`, `WNOWAIT=0x1000000`) do not require any additional platform branching beyond the two constant files already planned | Grounds §6.4/D5's rev-10 termination-protocol redesign (task 1): the non-reaping `waitid`/`WNOWAIT` observer replaces rev-9's early `cmd.Wait()` as the leader-exit-detection mechanism, closing the PGID-reuse race described under C36 by ensuring the leader is never reaped until strictly after the unconditional `SIGTERM`→grace→`SIGKILL(-pgid)` sequence has fully run, in every trigger branch (`AC-85`/`AC-96`/`AC-97`/new `AC-106`). |

### 0.2 What rev-4 removes or changes

- The kernel-independent temp-directory/`owner.json`/PID+process-start
  lock protocol (§7.2, full rewrite). Replaced with a kernel-released
  nonblocking advisory `flock` on a single, persistent, ignored+
  untracked `.lock` file — the kernel releases the lock automatically
  when the holding process's file descriptor is closed for any reason,
  including a crash, eliminating the entire owner-verification/
  PID-reuse/quarantine problem class rev-2/rev-3's design needed to
  solve for. Unsupported (non-POSIX) hosts refuse explicitly
  (`resource-lock-unsupported`, exit 3) rather than silently degrading.
- The batch-publication idempotency check comparing the wrong byte
  representation (§7.3 fix). Rev-3 compared the canonical
  `CanonicalBatchJSON` hash-input bytes (which exclude `batch_id` and
  use no pretty-printing) against the on-disk file bytes (which
  include `batch_id` and use 2-space indentation) — these can never be
  equal even for identical content, breaking idempotency by
  construction. Rev-4 compares the freshly re-encoded **complete
  intended file-wire bytes** (including `batch_id`, with the actual
  on-disk indentation/newline convention) against the bytes on disk;
  `batch_id` itself is still derived by hashing the canonical body
  **without** `batch_id` (unchanged).
- `remove`/`clear` pruning `current.json`'s live index (§3, §4, §7.3,
  §12.5). `current.json` is now a purely historical/capture pointer
  that only `capture`/`record --resources` ever write —
  `remove`/`clear` mutate **only** `resources.json`, under the same
  per-slug lock, and never touch `current.json` or any
  `batches/<id>.json` file. A resource with no `resources.json`
  declaration but a stale `current.json` entry is simply not consulted
  by `list` (which iterates declared resources only) — the stale
  pointer entry is harmless, permanent history, exactly like a batch
  file.
- `diff`'s "lightweight metadata... without opening file content"
  overclaim (§5.1 fix). `diff` **does** read file content through the
  same bounded in-memory scanner `capture` uses, to recompute a real
  hash — it does not perform a textual line-level diff of that content
  (§2's non-goal is unchanged), but it is not metadata-only either; the
  two paragraphs contradicted each other in rev-3 and are now
  reconciled around the correct (content-reading) behavior.
- The "single consistent point-in-time snapshot" overclaim for a
  directory `ignored-file` scan (§5.1 fix). Files are read
  **sequentially**, one at a time, not atomically-simultaneously — a
  mid-scan external modification to a later file in the same directory
  scan could in principle produce a hash-set that never existed
  together at any single instant. This sequential-read-consistency
  residual is now stated honestly rather than claimed away.
- `db_path`'s path-gate-then-`cmd.Dir` handoff being described as
  fully closed (§9.1 addition). Go's `exec.Cmd.Dir` takes a pathname
  string, not a file descriptor — there is no portable stdlib way to
  bind a child process's working directory to an already-validated
  open directory descriptor. §9.1 now states this honestly: the full
  gate re-runs immediately before `cmd.Start()` to minimize the race
  window, an open directory descriptor is held across the child's
  lifetime and re-checked after it exits, but a sufficiently
  well-timed local attacker replacing the final directory component
  mid-invocation is a documented residual, not a closed guarantee.
- The Dolt-citation precision gaps (§6.2, §6.3, task 12, C25–C27): the
  native-JSON-boolean claim now cites the row constructor, not just the
  schema type; the closed `diff_type` enum now cites the four exact
  assignment lines and notes `DiffTypeAll`'s filter-only status; the
  JSON parser now explicitly trims surrounding whitespace before
  structural parsing, grounded in the real trailing-newline shape of
  captured `dolt sql -r json` stdout.
- The `O_EXCL`-create-then-write-body lock sequence and the
  pathname-based post-open identity re-check are **unchanged** from
  rev-3 (already correct; the first no longer applies since §7.2 is a
  different mechanism entirely, the second — `os.SameFile` on the open
  descriptor — remains this design's primary identity check).
- The optional `table` argument and the absent `db_path`/cwd concept
  for Dolt, the guessed `"schema"`-key JSON envelope, random `batch_id`
  generation, the `keep_local`/local-raw-history removal, the
  `git-metadata` `config` 4-key allowlist, and "no tracked
  wall-clock-timestamp field" are all **unchanged** from rev-3
  (already correct).

### 0.3 Golden resource-ID vectors

The canonical `args`-encoding algorithm (§13.1) and the
`CanonicalBatchJSON` batch-body encoder (§7.3 step 2, §12 intro) are
**unchanged** by rev-7 — nothing in this fold touches the
hash-derivation functions themselves. What **does** change (task 4):
`binary_sha256` is **removed** from `adapter-snapshot`/Dolt `args`
entirely (it is now mutable trust metadata stored in a separate
`trust` field, §12.1, excluded from the identity hash) and replaced,
as an `args` key, by a new `contract` enum value (`"dolt-diff-summary-v1"`,
task 5) — Vectors 2/3 below are therefore genuinely **recomputed**
again, not merely re-verified, and the recomputation was independently
confirmed via a standalone Python script implementing §13.1/§13.2
verbatim, run as part of this revision's validation pass. Vectors 1/4
(no `args`, unaffected by either the rev-6 `binary_sha256` addition or
this rev-7 removal/replacement) remain byte-identical:

| Vector | Feature | Kind | Selector | Adapter | Capability | Args (declaration order) | `resource_id` |
|---|---|---|---|---|---|---|---|
| 1 | `model-picker` | `git-metadata` | `head` | *(none)* | *(none)* | `{}` | `res_acc91dc23a8b` (unchanged) |
| 2 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `contract=dolt-diff-summary-v1, db_path=data/dolt-db, table=users, from=main, to=HEAD` | `res_4b62313b6cce` (**recomputed** — rev-7 removes `binary_sha256` from the hashed `args` set entirely and adds `contract` in its place) |
| 3 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `to=HEAD, db_path=data/dolt-db, table=users, from=main, contract=dolt-diff-summary-v1` (reordered) | `res_4b62313b6cce` (**identical to Vector 2** — order-independence, reconfirmed with `binary_sha256` gone and `contract` present) |
| 4 | `model-picker` | `ignored-file` | `config/local-secrets.env.template` | *(none)* | *(none)* | `{}` | `res_79f5ac5dca13` (unchanged) |

**Why this changes again** (task 4): rev-6 made `binary_sha256` a
normal `args` key precisely so a differently-trusted Dolt binary would
produce a different `resource_id` "by construction, with no
special-casing needed" — but the rev-6 adjudication found this meant
re-pinning trust after a legitimate Dolt upgrade (a new, still-correct
binary with a new hash) silently orphaned the resource's entire
identity, `current.json` entry, and capture history, since the ID
itself changed. Rev-7 corrects this: a Dolt resource's **identity**
is now exactly `kind`/`adapter`/`db_path`/`table`/`from`/`to`/
`contract` — the semantic contract being captured — while
`binary_sha256` becomes purely operational trust metadata, mutable via
`trust-dolt` (§3, §6.1) without perturbing `resource_id`, `current.json`,
or history at all.

### 0.4 Requirement-item → section map

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Lock redesign: kernel `flock`, unsupported-host exit 3, all mutators | §7.1, §7.2 |
| 2 | No pre-scan writes; bounded-memory cap-plus-one reads | §7.1, §8 |
| 3 | `db_path`/`cmd.Dir` selector identity, honest residual | §9.1 |
| 4 | Dolt JSON/args exactness, whitespace trim, row types | §6.2, §6.3 |
| 5 | Publication idempotency fix (full file-wire compare) | §7.3 |
| 6 | Temps/cleanup exact naming | §7.1, §7.3, §7.5 |
| 7 | `--dry-run` wording (no "zero filesystem writes") | §3, §7.1 |
| 8 | Local ignore + perms for every mutator | §7.4, §10.3 |
| 9 | Ignored-file `diff` reads content; directory wire | §5.1, §12.2 |
| 10 | Complete wire variants | §12 |
| 11 | `record` two-domain retention, remove/clear declarations-only | §11, §3, §4 |
| 12 | Dolt DB/PK exactness, precise citations | §6.2, §0.1 (C25–C27) |
| 13 | CURRENT/refs final-count updates | `docs/handoff/CURRENT.md` |
| 14 | ACs/matrix rebuild | §14 |

**Rev-5 items** (18-item list, `07eab8e` dispatch):

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Exact `linux \|\| darwin` / `!linux && !darwin` build tags | §7.2 |
| 2 | `statfs` filesystem preflight, allow/deny lists | §7.2 |
| 3 | `db_path` honest pathname-vs-descriptor pre/post checks | §9.1 |
| 4 | Dolt JSON/args exactness (unchanged from rev-4, reconfirmed) | §6.2 |
| 5 | Output-cap-as-refusal via `StdoutPipe`/`StderrPipe` + kill | §6.4, §8.1 |
| 6 | `WORKING`/`STAGED` refusal (`dolt_ignore` risk) | §6.2, AC-10/11 |
| 7 | Full SHA-256 (untruncated) `batch_id` | §7.3, §12.3 |
| 8 | Batches are an unordered content-addressed set, not a chronology | §4, §7.3 |
| 9 | Directory `mode` folded into `combined_hash`/diff | §5.1, §12.2 |
| 10-18 | Citation/wording/count corrections (build-tag precision, lock-body, JSON whitespace, etc.) | §0, §14.1, §15 |

**Rev-6 items** (13-item compatibility fold, `b312e4a` dispatch):

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Dolt binary trust pin (`binary_sha256`), pre/post hash, `adapter-binary-untrusted` | §6.1, §12.1, §13 |
| 2 | `db_path` mismatch is a hard refusal, not diagnostic-only | §9.1 |
| 3 | `SysProcAttr{Setpgid:true}`, group-only signal, parent survival | §6.4 |
| 4 | `resource-id-collision` refusal (distinct-payload same-ID) | §4, §13 |
| 5 | `current_batch_id` rename, remove chronology language | §4, §7.3, §12.4 |
| 6 | File-wire drift vs. semantic-body collision split | §7.3 |
| 7 | Stdlib `syscall.Statfs`, exact allow/deny lists incl. overlayfs | §7.2 |
| 8 | First-create sequencing (nearest-ancestor gate/statfs, parent fsync) | §7.1 |
| 9 | Directory `combined_hash` tuple clarity + golden vector | §5.1, §12.2 |
| 10 | `dolt-argument-refused` named exit-2 reason | §6.2, §14 |
| 11 | Citation/wording cleanup (`unix` scope, refs, counts) | §0.1, §7.2, §9.1 |
| 12 | Re-confirm output/publish invariants after rename | §6.4, §7.3, §12 |
| 13 | ACs/matrix rebuild for all rev-6 behaviors | §14 |

**Rev-7 items** (11-item bounded maintenance/trust fold, `d503d55` dispatch):

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Fresh-clone leaf-ignore/untracked gate targets intended (not-yet-existing) leaf; nearest-existing-ancestor for `statfs` only | §7.1 |
| 2 | Unreaped leader through grace, group `SIGKILL` before `Wait`, normal-success `Wait` defined separately | §6.4 |
| 3 | Executed-binary binding: private hash-verified copy execution, not the resolved pathname | §6.1 |
| 4 | Trust/identity split: `binary_sha256` excluded from `resource_id`; `trust-dolt` re-pin command | §3, §6.1, §12.1, §13 |
| 5 | Runtime-compatibility honesty: `contract` enum, `dolt-trust-required` for missing pin | §6.1, §6.2 |
| 6 | Corruption taxonomy: `resources-file-corrupt` (load-time self-mismatch) vs. `resource-id-collision` (two declarations) | §4 |
| 7 | Statfs width: `uint32` normalization, arch coverage | §7.2 |
| 8 | Fsync discipline: every directory in chain, including retry dirs, first/retry crash sequence | §7.1, §7.3 |
| 9 | Counts/history corrections (rev-5 AC-numbering, review-pass counts) | §0.1, §14.1 |
| 10 | Wire/vector updates: `resources.json` trust split, recomputed golden vectors, trust-update wire | §12, §13 |
| 11 | ACs/matrix rebuild for all rev-7 behaviors | §14 |

**Rev-8 items** (11-item acceptance/lifecycle micro-fold, `bc2c068` dispatch):

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Test Matrix row 146 (ADR): leaf-ignore/subtree-untracked + ancestor-`statfs` + unconditional retry-fsync, matching `AC-81` | ADR Test Matrix |
| 2 | Unified process-group termination for every invocation (success or kill-triggered alike) | §6.4 |
| 3 | Add-time TOFU bootstrap split from capture-time trust verification/execution | §6.1, §3 |
| 4 | `trust-dolt` in every universal-mutator/local-gate list; duplicate `add` never re-pins | §3, §4, §7.2, §10.3 |
| 5 | Exit taxonomy consolidation, each named refusal in exactly one row | §3, §14 |
| 6 | Private-copy host failures: `ST_NOEXEC`/`MNT_NOEXEC` preflight, `ENOSPC`/`EIO` handling | §6.1 |
| 7 | Local gate targets split: ignore=leaf, untracked=whole `.tpatch/local/` subtree | §7.1, §10.3, ADR D8 |
| 8 | Corruption/collision taxonomy confirmation (no rewrite needed) | §4 |
| 9 | Counts/history corrections; ground C36 in "escalation never cancels on leader exit" | §0.1, §14.1 |
| 10 | Wire/vector re-verification (trust/gate changes do not affect canonical IDs) | §12, §13 |
| 11 | ACs/matrix rebuild for all rev-8 behaviors | §14 |

**Rev-9 items** (6-item terminal micro-fold, `8152a8b` dispatch):

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | ADR Test Matrix row 146: fix stale "fsync each newly-created directory's parent" to match §7.1/D9's unconditional whole-chain retry-fsync | ADR Test Matrix |
| 2 | Process-group completion protocol: decouple leader-exit detection from pipe-drain-to-`EOF` via caller-owned `os.Pipe()`s assigned to `cmd.Stdout`/`cmd.Stderr` (not `cmd.StdoutPipe()`/`cmd.StderrPipe()`), so `cmd.Wait()` reflects only the leader's own exit — closing the successful-exit-can-leave-descendants deadlock | §6.4, §0.1 (C40) |
| 3 | `trust-dolt` added to §7.2's per-slug-lock "every mutating verb" enumeration (ADR Test Matrix row 77/`AC-47` mirrors the fix) | §7.2 |
| 4 | Add-time TOFU vs. capture-time missing-pin separation — verified already correct, no fix needed | §6.1 |
| 5 | Duplicate `add` strict no-repin — verified already correct, no fix needed | §4 |
| 6 | Exit-taxonomy consistency (`dolt-trust-flag-required`/`dolt-trust-required`/`adapter-copy-noexec`/`adapter-copy-failed`) — verified already correct, no fix needed | §3 |

**Rev-10 items** (6-item consolidated correction, `19e3024` dispatch):

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Non-reaping `waitid`/`WNOWAIT` observer replaces rev-9's early `cmd.Wait()`; unified cleanup (`SIGTERM`(-pgid)→grace→`SIGKILL`(-pgid)→await observation→join pipes→single final `cmd.Wait()`); no "Wait proves group emptiness" claim; escaped-session-descendant residual disclosed; cross-compile/source-shape test | §6.4, §0.1 (C36, C40, C41) |
| 2 | `trust-dolt` added to §7.1's top-level "every mutator" local-gate enumeration (distinct from §7.2's already-fixed list); `AC-52`/row 86 cover `add`/`remove`/`clear`/`trust-dolt` | §7.1 |
| 3 | Taxonomy split: `adapter-missing-at-add` (exit 2, add-time) vs. `adapter-missing` (exit 3, capture-time); `path-outside-repo` defined and added to the exit-3 table | §3, §9.1 |
| 4 | Private-copy mode unification: add-time TOFU hashes the opened descriptor directly (no copy file, zero processes); capture-time creates `0600`, streams/hashes/`Sync`s, verifies digest, then descriptor-based `Fchmod` to `0500` | §6.1, §7.1 |
| 5 | Vector 3 declaration-order parity (PRD §0.3/§13.3, ADR D3); CURRENT's invented "Vector 5" corrected (there are only 4 vectors) | §0.3, §13.3, `docs/handoff/CURRENT.md` |
| 6 | Stale-surface sweep: `AC-85`/`AC-96`/`AC-97` rewritten for the non-reaping observer; `C36`/`C40` re-grounded; stray `cmd.StdoutPipe()` reference in §5.1 corrected | §5.1, §14.1, §0.1 |

---

## 1. Problem Statement

`tpatch` features today capture and replay Git-tracked content only.
Some features (a schema migration alongside a Dolt-backed data
service; a feature whose behavior depends on a deliberately
`.gitignore`d local config template; a feature whose reviewer needs to
know "what was `HEAD` and what were the relevant index entries at
capture time") have relevant state that is **not** a Git blob and
today has no place to live in tpatch's model. Reviewers and future
readers of a feature's history currently have no structured record of
this adjacent state at all.

## 2. Goals / Non-Goals

**Goals**: let a feature declare zero or more *resources* — typed,
named references to non-canonical-patch state — and capture a
point-in-time, privacy-safe **summary** of each into one tracked,
content-addressed, per-feature capture set (an unordered set of
distinct-content batches plus a sole current-state pointer — not a
chronological append-only log, rev-5 §7.3), with all raw content
strictly ephemeral (never persisted past the single command that read
it, in either the tracked or local tree).

**Non-goals**: resources are not inputs to `apply`/`unapply`/
reconcile; resources do not gate `land`; resources are not a
general-purpose secrets vault; resources do not make Dolt (or any
external tool) a runtime dependency of `tpatch`; resources do not
support arbitrary sandboxless external commands; this PRD does not
change any existing command's exit-code contract or `record`'s
existing Git-side capture-mode semantics (mutex group, empty-patch
handling, `--auto`/range resolution) — it only adds an orthogonal
`--resources` flag (§11). **New non-goal (rev-2)**: this PRD does not
provide textual/byte-level content diffing or any versioned history of
raw resource content — only metadata/hash/file-set-level change
detection (§5.1, §7.3.4). `ADR-027-capture-context-privacy-boundary.md`
D3 states verbatim (C16): "Local private buffers may keep only the
redacted or hashed form; this ADR does not authorize a tpatch-managed
raw transcript archive" — persisting raw content across captures, in
any local lane, opt-in or not, would require a future ADR that
explicitly supersedes that sentence; this PRD takes no position on
whether such a future ADR should exist.

## 3. Command Surface

All new verbs live under the existing `feature` noun, mirroring
`feature claim`'s shape:

```
tpatch feature resource add        <slug> --kind <kind> --selector <sel> [--adapter <a> --capability <c> --arg k=v ...] [--trust-current-dolt] [--json]
tpatch feature resource list       <slug> [--json]
tpatch feature resource remove     <slug> <resource-id-or-prefix> [--json]
tpatch feature resource clear      <slug> [--json]
tpatch feature resource trust-dolt <slug> <resource-id-or-prefix> --binary-sha256 <64hex> [--json]
tpatch feature resource capture    <slug> [--resource <resource-id-or-prefix>] [--dry-run] [--json]
tpatch feature resource diff       <slug> [--resource <resource-id-or-prefix>] [--json]
tpatch record <slug> [existing flags...] [--resources] [--json]
```

Rev-2 removed the rev-1 `--keep-local` flag entirely (§0.2, unchanged
since): raw bytes are always ephemeral now, so there is nothing left
to opt into. **`--trust-current-dolt`** (§6.1) remains the only way to
declare an `adapter="dolt"` resource. `add` refuses `--kind
adapter-snapshot --adapter dolt` without it (`dolt-trust-flag-required`,
exit 2, renamed rev-8, task 3/5 — previously shared the name
`dolt-trust-required` with the unrelated capture-time exit-3 refusal
below; every named refusal now appears in exactly one row/table
context, task 5) — there is no default-trust fallback. **Rev-7 changes
where the resulting pin is stored, rev-8 clarifies the sequence run**
(task 3, task 4): when passed, `add` runs only the **add-time trust
bootstrap (TOFU)** sub-sequence of §6.1 — `exec.LookPath`+
`EvalSymlinks`+validate, open, stream-copy-while-hashing into an
ephemeral private copy, write the digest, delete the copy — and
**never** the capture-time steps that require an *existing* pin or
that execute anything (rev-8 corrects rev-7's ambiguous "executes the
private-copy-verification sequence" phrasing, which read as if `add`
also ran the unpinned-refusal check and the `cmd.Start()` execution
step; it does not, and could not, since by definition no pin exists
yet the first time `add --trust-current-dolt` runs, §6.1). The
resulting digest is written into the resource's **separate `trust`
field** (§12.1) — `trust: {"binary_sha256": "<hex>"}` — **never** into
`args`. `args` for a Dolt resource instead always carries a `contract`
key (§6.1/§6.2, `"dolt-diff-summary-v1"` in v1) alongside `db_path`/
`table`/`from`/`to`; unlike rev-6's design, the trust pin therefore
does **not** participate in `resource_id` (§13) — two declarations
differing only in which Dolt binary they currently trust produce the
**identical** `resource_id`, by construction.

**`trust-dolt`** is new in rev-7 (task 4, §6.1): the **only** way to
change an already-declared Dolt resource's trust pin after `add`,
without discarding its `resource_id`, `current.json` entry, or capture
history (rev-6's design had no such operation — a Dolt binary upgrade
requiring a new pin meant `remove`+`add`, which produced a **new**
`resource_id` since `binary_sha256` was part of the identity hash,
silently orphaning the old resource's entire history; rev-7 closes
this entirely by removing the pin from the identity hash in the first
place). `trust-dolt <slug> <resource-id-or-prefix> --binary-sha256
<64hex>` resolves the target resource by exact ID or unambiguous
prefix (identical ambiguity handling to `remove`), refuses if the
resolved resource's `kind`/`adapter` is not `adapter-snapshot`/`dolt`
(`resource-not-dolt-adapter`, exit 2 — `trust-dolt` is meaningless for
any other kind), acquires the same per-slug `flock` (§7.2) `add`/
`remove`/`clear` use, and atomically rewrites **only** that resource's
`trust.binary_sha256` field in `resources.json` — `resource_id`,
`selector`, `kind`, `adapter`, `capability`, `args`,
`added_by_tool_version`, and every other resource's entry are left
byte-for-byte unchanged, and `current.json`/every `batches/<id>.json`
file are never touched (an update to `trust` is not a capture and
never participates in publication, §7.3). A `--binary-sha256` value
that is not exactly 64 lowercase hex characters is a validation error
(exit 2) before the lock is ever acquired. `trust-dolt` does **not**
itself resolve or hash a live `dolt` binary (unlike `add
--trust-current-dolt`, which computes the pin from whatever `dolt` is
currently on `PATH`) — it takes the caller's asserted hash directly,
so an operator can pre-approve a hash before installing the
corresponding binary, or pin a hash obtained from a separate,
out-of-band verification step. `add --trust-current-dolt` remains the
convenience path for "trust whatever `dolt` I have installed right
now"; `trust-dolt` is the explicit, hash-first re-pin path for every
other case, including routine upgrades.

- **`capture`** is the only verb that ever executes the Dolt adapter,
  reads ignored-file content, or reads Git metadata, and the only verb
  that ever writes tracked (§7.3) state. `--dry-run` (task 7) runs the
  **entire** pipeline for real — lock acquisition, orphan sweep,
  ignored-file/Git-metadata reads into bounded memory, the real Dolt
  invocation inside a real scratch `HOME` (§6.4), redaction scanning —
  and reports exactly what would be published, but guarantees **no
  tracked writes and no persistent local writes**: no
  `batches/<id>.json` or `current.json` write is ever attempted, and
  every ephemeral `es_<id>/` directory it creates (including a real
  Dolt `dolt-home/` and, when a Dolt capability is targeted, the
  private-copy scratch file, §6.1) is removed before it returns,
  exactly as a real capture's cleanup step does; `--dry-run` never
  runs either startup orphan sweep against the **tracked** tree
  (§7.1). This is a narrower, more honest claim than "zero local
  writes survive past the invocation": the `.lock` file (§7.2) is a
  **persistent**, ignored/untracked control file that is never deleted
  after its first creation for a given slug — `--dry-run` acquires and
  may create this file exactly like any other mutator, and it is
  explicitly not part of the "nothing persists" guarantee, only the
  tracked artifacts and the per-invocation ephemeral scratch tree are.
- **`diff`** is read-only: it never executes the adapter and never
  writes tracked state or acquires the lock (§7.6). For `ignored-file`
  it **does** read current file content through the same bounded
  in-memory scanner `capture` uses, to recompute a real `hash`/
  `combined_hash` (§5.1) — it is not a metadata-only stat check —
  though it never produces a textual line-level diff of that content
  (§2's non-goal). It compares the recomputed result against the last
  tracked batch's recorded `result` for that resource (§5.1, §7.3.4).
  Called before any capture has ever run for a resource, it reports
  "no capture yet" (exit 0, not an error).

`add`/`list`/`remove`/`clear`/`trust-dolt` behave exactly as `feature
claim`'s quartet does (same `"no such feature: %s"` refusal shape,
same `--json` convention), except `add` additionally computes and
persists `resource_id` (§13); an `add` whose full declaration tuple
(`selector`+`kind`+`adapter`+`capability`+canonical `args`) is
byte-identical to an already-declared entry for that `resource_id` is
**idempotent** (exit 0, no second entry written), while a distinct
tuple that happens to collide with an existing entry's 12-hex-
truncated `resource_id` is refused (`resource-id-collision`, exit 3 —
see §4's collision-refusal subsection, which rev-7 splits from the
distinct load-time `resources-file-corrupt` outcome, task 6).
`add`/`remove`/`clear`/`trust-dolt` all acquire the same per-slug lock
`capture` uses (§7.2, task 1) before mutating `resources.json` —
before the lock file itself is ever created, all four (like
`capture`/`record --resources`) first run the local-ignore + untracked
gate (§10.3, task 8; rev-7, task 1: against the intended scratch-root
leaf directly, §7.1, not merely its nearest existing ancestor).
`add`/`remove`/`clear`/`trust-dolt` never perform the scratch orphan
sweep themselves (only `capture`/`record --resources` do, since only
those two ever create ephemeral scratch content, §7.1) and **never**
write or rewrite `current.json` or any `batches/<id>.json` file —
those remain untouched by every verb except `capture`/
`record --resources`. `remove`/`clear` only ever mutate
`resources.json` — a resource's `current.json` entry, if one exists
from a prior capture, simply becomes orphaned (harmless, permanent
history, exactly like a batch file that outlives its resource's
declaration); `list` never surfaces it because `list` iterates
`resources.json`'s declared entries, not `current.json`'s index.
`list` never acquires the lock (§7.2, task 1): it is a pure read of
whatever `resources.json`/`current.json` content is currently visible
on disk, which — because both files are always written via
temp-then-atomic-rename (§7.3, §7.6) — is always either the
fully-prior or fully-new content, never a partial read, even if a
`list` races a concurrent mutator.

## 4. Data Model

Two tracked artifacts per feature, both under the existing per-feature
artifacts directory (`store.featureArtifactsDir`, `internal/store/store.go`),
never inside `apply-recipe.json` or any unapply/lifecycle-state file:

- **`resources.json`** — the declaration manifest (declaration-only;
  not itself part of any capture transaction, task 4). One entry per
  declared resource: `resource_id`, `kind`, `selector`, `adapter`
  (empty string if not applicable), `capability` (empty string if not
  applicable), `args` (a sorted array of `{key, value}` pairs, never a
  bare JSON object/map — §12.1), `trust` (rev-7, task 4: `null` for
  every kind except `adapter-snapshot`/`dolt`, where it is
  `{"binary_sha256": "<64hex>"}` — **mutable** trust metadata,
  **excluded** from `resource_id`'s canonical identity hash, §13; see
  below), `added_by_tool_version` (the `tpatch` version string that
  created this declaration; informational only, not a timestamp).
  **Never** contains a capture result, a hash of captured content, or
  any raw content. `add`/`remove`/`clear`/`trust-dolt` are the
  **only** verbs that ever write this file, under the per-slug lock
  (§7.2); `trust-dolt` (§3, task 4) is the only one of those four that
  ever mutates an **existing** entry's field in place — `add` only
  ever appends a new entry (or is idempotent against a byte-identical
  existing one), and `remove`/`clear` only ever delete entries.

  **`trust` is mutable, and deliberately excluded from identity**
  (rev-7, task 4 — supersedes rev-6's "`binary_sha256` is a normal
  `args` key, part of `resource_id`'s hash" design): a Dolt resource's
  **identity** — what `resource_id` is computed from (§13.2) — is
  exactly `kind`/`selector`/`adapter`/`capability`/`args`, where `args`
  for a Dolt resource is `contract`/`db_path`/`table`/`from`/`to`
  (§6.1/§6.2) and **never** `binary_sha256`. The **trust pin** — which
  specific binary is currently approved to satisfy that identity's
  semantic contract — lives in the separate `trust` field precisely so
  it can change (via `trust-dolt`, §3) without changing
  `resource_id`, without orphaning `current.json`'s existing pointer
  for that resource, and without discarding any prior batch history.
  This directly closes the gap the rev-6 adjudication found: because
  `binary_sha256` was part of the identity hash in rev-6, re-pinning
  trust after a legitimate Dolt binary upgrade produced a **new**
  `resource_id`, silently orphaning the old resource's entire capture
  history even though nothing about *what* was being captured (the
  semantic contract) had changed — only *which binary* was trusted to
  capture it.

  **Resource-ID collision refusal** (task 4, rev-6; corruption-vs-
  collision split, task 6, rev-7): `resource_id` remains `res_` + the
  first 12 lowercase-hex characters of the §13.2 digest — a 48-bit
  truncation this design keeps for a short, reader-facing ID (unlike
  `batch_id`, which grew to the full digest in rev-5 specifically
  because collisions there are silently overwrite-prone; a 12-hex
  `resource_id` collision, by contrast, is always caught at `add`/load
  time below before it can silently overwrite anything). Both `add`
  and every verb that **loads** `resources.json` (`list`, `remove`,
  `clear`, `trust-dolt`, `capture`, `diff`, `record --resources`)
  recompute, for every entry already present, a fresh `resource_id`
  from that entry's own `feature`/`kind`/`selector`/`adapter`/
  `capability`/`args` fields (§13.2 — `trust` never participates) and
  compare it against the entry's own recorded `resource_id` key, then
  map every loaded entry by its recorded `resource_id` (a Go
  `map[string]...`, in-process only — never a tracked wire
  representation, which stays the sorted `[]struct` array, §12 intro):

  - At `add`, if the candidate declaration's freshly-computed
    `resource_id` matches an **existing** entry's `resource_id` and
    the two declarations' full canonical byte payloads (the exact
    §13.2 hash-input string: `feature\x00kind\x00selector\x00adapter\x00capability\x00canonical_args`)
    are **byte-identical**, the `add` is idempotent — no second entry
    is written, and `add` reports success (exit 0) rather than an
    error, since nothing about declared identity actually changes. A
    duplicate `add --kind adapter-snapshot --adapter dolt
    --trust-current-dolt` targeting an already-declared Dolt resource
    is a **strict** idempotent no-op with respect to `trust` too
    (rev-8, task 4, correcting rev-7's ambiguous "the trust pin is
    compared/updated separately" phrasing, which could be misread as
    implying a duplicate `add` might still refresh `trust.binary_sha256`
    as a side effect): the existing entry's `trust.binary_sha256` is
    left **byte-for-byte unchanged**, even if the currently-resolved
    `dolt` binary's hash now differs from the stored pin — `add` never
    re-pins an already-declared resource, regardless of whether
    `--trust-current-dolt` is re-passed; `trust-dolt` (§3) is the
    **only** operation that may change an existing resource's `trust`
    field after its initial `add`. This is consistent with `trust`
    never participating in the canonical payload/identity check above
    at all: the *presence* of an existing entry for the same identity
    is what makes `add` idempotent, and idempotent means "nothing
    changes," not "everything except identity is refreshed."
  - If the candidate's `resource_id` matches an existing entry but the
    two canonical byte payloads **differ** — a genuine collision in
    the 12-hex-truncated ID space between two **distinct**
    declarations, each independently correct with respect to its own
    fields — `add` refuses with `resource-id-collision` (exit 3) and
    does **not** write or overwrite the existing entry. This is
    astronomically unlikely for real `SHA-256` inputs (48 bits of
    truncation) but is not economically impossible, and `resource_id`
    is a reader-facing, trusted identifier this design must never
    silently double-assign.
  - **At load time** (rev-7, task 6 — corrects rev-6's conflation): if
    any entry's own recorded `resource_id` does not match the
    `resource_id` freshly recomputed from that **same single entry's**
    own fields, this is a **different** outcome from the bullet
    above — there are not two distinct declarations in tension with
    each other here, there is exactly **one** entry whose own recorded
    identity no longer matches its own content, which can only arise
    from a hand-edited or otherwise corrupted `resources.json` (since
    `add` itself never persists a mismatched pair). This refuses
    `resources-file-corrupt` (exit 3) for that entry — a distinct named
    outcome from `resource-id-collision`, never silently trusted or
    auto-repaired. `resource-id-collision` is reserved exclusively for
    the two-distinct-declarations-same-ID case (at `add`, or — in the
    rarer case two already-loaded, individually self-consistent
    entries happen to share a recorded `resource_id`, itself only
    reachable via hand-editing two entries into existence with the
    same ID and differing bytes — detected via the same by-ID map
    above finding two keys for one map slot) — never for a single
    entry's own internal self-inconsistency, which is
    `resources-file-corrupt` instead.
  - A collision seam is exercised in tests via a stub `resource_id`
    derivation function that can be forced to return the same value
    for two different canonical payloads (a real `SHA-256` collision
    is not economically producible for a test fixture); the production
    code path has no test-only branch, only the derivation function
    itself is substitutable in tests. `resources-file-corrupt` is
    exercised directly (no stub needed) by hand-constructing a fixture
    entry whose recorded `resource_id` does not match its own fields.
- **`artifacts/resource-captures/`** — the tracked capture store: an
  **unordered, content-addressed set** of immutable `batches/
  <batch_id>.json` files (one per **distinct content**, not one per
  invocation — rev-5 correction, §7.3; a `capture` invocation that
  reproduces already-published content writes zero new batch bytes)
  plus one atomically-rewritten `current.json` pointer mapping each
  resource to the batch that holds its current result (§7.3,
  §12.3–§12.4). This is a set of content, not a chronological log —
  no batch file or the pointer ever records an ordering, sequence
  number, or timestamp (§0.2), and reverting a resource's content back
  to a previously-seen state simply repoints `current.json` at the
  already-existing batch for that content, rather than creating a new
  entry. This tree is written **exclusively** by `capture`/
  `record --resources` — `resources.json` is never part of this
  transaction, and `add`/`remove`/`clear`/`trust-dolt` never write to
  `current.json` or any `batches/<id>.json` file, only to
  `resources.json` (§3).

### 4.1 Missing-referenced-batch (closes rev-1's "missing-local" case, now about tracked state)

Rev-1 defined a "missing-local" case for an opt-in local raw
companion; that concept no longer exists (§0.2). The analogous rev-2
case is: `current.json` names a `batch_id` for some resource, but the
corresponding **tracked** `batches/<batch_id>.json` file is absent
(e.g. a shallow clone, a manually pruned history, or filesystem
corruption — this should not happen in the normal atomic-publish flow,
§7.3.3, but is a real possibility for anything that reads a
`.tpatch/features/<slug>/` tree from outside `tpatch` itself). `list
--json`/`diff` report this as `tracked-batch-missing` for that
resource specifically (exit 1 — a data-integrity condition, distinct
from "no capture yet," which is exit 0) and do not fail the whole
command if other resources' batches are present and readable.

## 5. Resource Kinds

Three kinds in v1, closed set (no plugin mechanism):

### 5.1 `ignored-file` (task 5, task 6, task 7)

Selector: a repo-relative path (file or directory). **Both** of the
following must hold at `add` time and are **rechecked at every
`capture`**:

1. `git check-ignore -q --no-index -- <pathname>` exits `0` (ignored).
   Exit `1` means "not ignored" (gate fails, refused). Any other exit
   code is a fatal Git error (`git-ignore-check-error`, exit 3,
   fail-closed — never treated as "not ignored" or "ignored," just
   refused outright, §10.1). This reuses the **existing**,
   already-correct `gitutil.IsPathIgnored` invocation shape verbatim
   (C17) — `check-ignore` has no `--literal-pathspecs` option and
   fails fatally if one is passed (`fatal: <path>: pathspec magic not
   supported by this command: 'literal'`, exit 128), so rev-2's
   requirement to add it there was itself invalid and is removed.
   Because `check-ignore`'s plain pathname argument still parses a
   leading `:` for pathspec magic (C18), any selector whose first byte
   is `:` is passed as `./<selector>` instead of `<selector>` verbatim
   — this disarms the colon-magic parse (the argument no longer begins
   with a bare `:`) while resolving to the identical file; no other
   selector shape needs this prefix (`*`/`?`/`[]` are inert to this
   command, empirically confirmed, C18).
2. `git --literal-pathspecs ls-files --error-unmatch -- <selector>`
   exits non-zero with the standard "did not match any file(s) known
   to git" message (untracked; gate passes). Exit `0` means the path
   **is** tracked — refused (`tracked-and-ignored`, exit 3) even
   though check 1 said "ignored," closing the exact `--no-index` gap
   where an already-tracked file can still report "ignored" (§10.2).
   Any exit/output combination that is neither of these two well-known
   shapes is a fatal Git error, refused the same as check 1
   (`git-ls-files-error`, exit 3). Unlike `check-ignore`, `ls-files`
   **does** support `--literal-pathspecs` (task 6) and this call keeps
   it, so a colon-shaped or otherwise magic-looking selector is always
   treated as a literal path here regardless of the workaround check 1
   needs — closing an ambiguity rev-1 did not address (§10.4 has the
   exact rows for both calls).

**Path/symlink gate** (task 5, full rewrite — see §9.1 for the
complete algorithm): every path component from the repository root
down to the selector (and, for a directory selector, down to each
matched descendant file independently) is `Lstat`'d; **any** symlink
component anywhere in that chain is refused outright
(`symlink-component-refused`, exit 3) — this design does not attempt
to resolve and re-validate a symlink's target (rev-1's approach, which
missed ancestor components); it simply refuses the presence of a
symlink anywhere in the path, a strictly simpler and safer fail-closed
v1 rule. Only after every component in the chain is confirmed a
regular (non-symlink) file/directory is the path opened, using
`O_NOFOLLOW` on the final open as an additional, real hardening layer
(§9.1) — and the **open file descriptor** is compared against the
pre-open `Lstat` via `os.SameFile` (task 4, C-descriptor-identity,
§9.1): this is a real `fstat` on the thing that was actually opened,
not a second pathname lookup, closing the TOCTOU window rev-2's
pathname-re-`Lstat` design left open (a symlink swapped in between the
check and the open, with the same pathname re-resolving to a
similarly-shaped regular file, would have passed rev-2's check; it
cannot pass an `os.SameFile` comparison against the actual open
descriptor). A pathname re-`Lstat` still runs afterward as defense in
depth but is no longer the primary identity check. Any mismatch is
`path-replaced-during-open` (exit 3).

**Directory limits** (unchanged from rev-1/rev-2): 5 MiB per file,
20 MiB total, 200 files — refused (exit 3) if exceeded, re-checked at
every `capture` even if the selector passed these limits at `add` time
(snapshot-time bounds, not a one-time check).

**Capture** (task 2): the matched file(s) are read into a **bounded
in-process memory buffer** (task 2's "zero pre-scan persistence") —
never written to a scratch file first, and bounded by an actual
cap-plus-one read (the reader stops after `limit+1` bytes and refuses
if that many were actually read, rather than trusting a pre-read
`Stat().Size()`, so a file that grows between `Stat` and read cannot
silently bypass the size cap) — so a directory selector's multi-file
scan reads each file's real content once, sequentially, without ever
placing an unredacted byte on disk. **This is a sequential read, not
an atomic multi-file snapshot** (task 9): each file is opened, read,
and hashed one at a time; there is no whole-directory lock or
copy-on-write snapshot, so an external process modifying a *later*
file in the same directory scan while an *earlier* file has already
been read and hashed can in principle produce a `combined_hash` that
never corresponded to any single point-in-time state of the directory
as a whole. This residual is stated honestly rather than claimed away
(§15). Content is scanned (redaction, §8) in memory, classified
`binary` (a `NUL` byte in the first 8 KiB) or `text`, and hashed
(`SHA-256`, verbatim bytes, **no** text normalization of any kind —
CRLF/LF, trailing newline, and encoding are all left exactly as found,
task 5's "raw local bytes are verbatim" requirement, restated for the
in-memory path). The buffer is discarded (Go's garbage collector
reclaims it; there is no file to delete) once hashing/scanning
completes — the tracked `result` for this kind is `file_kind`
(`"text"`/`"binary"`), `size_bytes`, `hash` (single file) or
`file_count`/`total_bytes`/`combined_hash` (directory — rev-5: the
combined hash is `SHA-256` over each matched file's canonical tuple,
sorted by `path`; **rev-6 (task 9) removes the ambiguity in how
tuples are joined**: each of the tuple's three fields — `path`
(repo-relative), `mode` (the same 6-digit octal-string convention as
`index-entry`'s `mode`, sourced from a plain `os.Stat`), and `hash`
(the file's own **raw, unprefixed 64-lowercase-hex** `SHA-256` digest
— explicitly **not** the `"sha256:"`-prefixed form used by the
wire-level `hash`/`raw.hash` fields elsewhere in this design, §12.2;
the prefix is a wire/display convention for those fields, not part of
any hash's byte input anywhere) — is individually terminated by a
single `0x00` byte, i.e. the per-file contribution to the hash input
is exactly `path + 0x00 + mode + 0x00 + hash + 0x00` (three fields,
three trailing NUL bytes, not two separators), and files' contributions
are concatenated directly with **no** additional separator between
them (each field's own trailing `0x00` already delimits it
unambiguously, since neither a repo-relative `path` nor a fixed-width
`mode`/`hash` can itself contain a NUL byte — rev-5's "`\x00`-joined"
phrasing left open whether the join was inter-field, inter-tuple, or
both, which task 9 flagged as ambiguous). So a **chmod-only** change
(identical bytes, different permission bits) now changes
`combined_hash`, not just a content or file-set change; rev-4's
formula omitted `mode` from the hash input entirely even though
`files[]` already carried a per-file `mode` (§12.2), so a
permission-only change was silently invisible to `combined_hash` and
to `diff` — this is corrected).

**Golden directory vector** (rev-6, task 9 — byte-identical to
ADR-033 D3): a two-file directory selector matching exactly
`config/a.txt` (mode `100644`, empty file) and `config/sub/b.sh` (mode
`100755`, content `#!/bin/sh\necho hi\n`) produces:

| Field | Value |
|---|---|
| `config/a.txt` `hash` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` — i.e. `SHA-256("")` |
| `config/sub/b.sh` `hash` | `299001868fb8c02fd431c336c6d058f5558c5dff5b5af5e6fe04b870a6a9cbba` |
| Tuple 1 bytes | `"config/a.txt"` + `0x00` + `"100644"` + `0x00` + `config/a.txt`'s `hash` (above) + `0x00` |
| Tuple 2 bytes | `"config/sub/b.sh"` + `0x00` + `"100755"` + `0x00` + `config/sub/b.sh`'s `hash` (above) + `0x00` |
| `combined_hash` input | Tuple 1 bytes directly concatenated with Tuple 2 bytes (sorted by `path`, `a.txt` before `sub/b.sh`) |
| `combined_hash` | `5af4d6754656795b49c6e22acc2034ed6a2b3426470b0c42156f5ad0b4bcb9ad` |

This vector is reproduced verbatim (identical input files, identical
`combined_hash`) in ADR-033 D3 so both documents can be diffed for
parity; a reference Python/Go re-implementation of the tuple-encoding
rule above, run against these two files, must reproduce this exact
digest.
**`diff`** (§3, task 9): **reads current file content** through the
same bounded in-memory scanner `capture` uses (not a metadata-only
stat check — rev-3's "without opening file content" claim
contradicted this same paragraph's own hash-recomputation requirement
and is corrected) to recompute `file_kind`/`size_bytes`/`hash`
(single file) or `file_count`/`total_bytes`/`combined_hash`
(directory) exactly as `capture` would, and compares the fresh result
against the last tracked batch's `result` for this resource. Reports
`unchanged`, or exactly which of `size_bytes`/`hash`/`file_count`/
`total_bytes`/`combined_hash`/file-set membership/per-file `mode`
differs — never a textual line-level diff of file content (§2's
non-goal is about line-level diffing, not about whether content is
read at all). Because `mode` is now part of `combined_hash`'s input
(above, rev-5), a chmod-only change (identical file set, identical
byte content, changed permission bits on one or more files) is
distinguishable from a content change: `diff` reports it as a `mode`
difference on the specific `files[]` entry/entries whose `mode`
changed, with `hash`/`byte_count` unchanged for that entry, rather
than folding it into an undifferentiated "`combined_hash` differs"
report. `diff` never writes any tracked or scratch file and never
acquires the lock (§7.6) — it is read-only in effect, not in the
sense of "never opens a file." A directory `diff` inherits the same
sequential-read consistency residual as a directory `capture` (above).

### 5.2 `git-metadata` (task 10)

Four closed views, tagged result variants (exact fields, §12.2):

- **`head`**: always defines `symbolic_ref` (the resolved full ref
  name HEAD points to, e.g. `refs/heads/main`, or JSON `null` when
  detached — "detached consistently" means `symbolic_ref` is `null`
  **iff** `detached` is `true`, never independently) and `detached`
  (bool) and `oid` (the current commit OID, always populated).
- **`ref`**: an explicitly selected ref (selector is the ref name
  itself, e.g. `refs/heads/feature-x` or `refs/tags/v1.0.0`); result
  is `ref` (the resolved full ref name) and `oid`.
- **`index-entry`**: selector is a repo-relative path; queried via
  `git --literal-pathspecs ls-files --stage -- <selector>` (literal
  pathspec, task 6); result is `path`, `mode` (octal string, e.g.
  `"100644"`), `oid`, `stage` (`0`–`3`). A path with no index entry at
  all is a validation error (exit 2) at `add` time, and a state
  refusal (`index-entry-missing`, exit 3) if it disappears from the
  index by the time of a later `capture`.
- **`config`**: selector is one of exactly four keys —
  `core.filemode`, `core.ignorecase`, `core.symlinks`,
  `extensions.objectformat` (unchanged from rev-1; no `user.*`, no
  wildcarded `core.*`/`remote.*`/`branch.*`). Any other key is a
  validation error (exit 2). Result is `key`, `value` (the resolved
  config value, or `null` if unset — unset is a valid, reportable
  state, not an error, since these are all keys with sensible
  defaults when absent).

Every resolved string value from any view passes through the
redaction scanner (§8) before being written anywhere — in practice
none of the four closed views can ever produce a value shaped like
any of the six closed classes, but the scan runs unconditionally
rather than being skipped "because it's Git metadata."

### 5.3 `adapter-snapshot` (task 1, task 6, task 10)

Selector: `dolt:<capability>:<table>`. `dolt` is the only adapter in
v1 (`generic-command` remains removed, per rev-1 §0.2, restated here).
`diff-summary` is the only capability (§6) — the rev-1 `schema-diff`/
`table-diff` split is gone; one query now reports both dimensions per
table. `table` in the selector and the mandatory `table` declared field
(§6.2) must match — a mismatch is a validation error (exit 2) at `add`
time; the selector exists to make the resource human-readable in
`list` output, the declared field is the authoritative value bound
into the SQL query and the resource ID (§13).

## 6. Adapter Protocol — Dolt (task 1, task 6, task 7)

### 6.1 Executable resolution, trust bootstrap, private-copy execution, and identity (task 3, task 4, task 5, rev-7; add-time/capture-time split + private-copy host failures, rev-8; add-time descriptor-hash-only + capture-time `0600`-create/`Fchmod`-`0500` mode unification, rev-10)

The adapter locates `dolt` via `exec.LookPath("dolt")` at **two,
clearly distinct** times — `capture` time, and `add` time when
`--trust-current-dolt` is passed (§3) — and runs a **shared resolution
prefix** (steps 1-4 below) followed by **two different, non-shared
continuations**: an **add-time trust bootstrap (TOFU)** (never
executes anything, never requires an existing pin) and a
**capture-time trust verification and execution** sequence (always
requires an existing pin, always executes the private copy). Rev-7
described these as one continuous numbered list; the rev-7 adjudication
found this literally reads as if `add --trust-current-dolt`'s
bootstrap run would itself hit the "refuse if unpinned" check the
capture-time sequence opens with — which cannot be right, since by
definition no pin exists yet the *first* time `add --trust-current-dolt`
computes one. Rev-8 (task 3) splits the two sequences explicitly below
so no single numbered list is read as applying to both call sites.
`trust-dolt` (§3) is a third, narrower operation: it never resolves or
executes a live binary at all, and is covered separately at the end of
this section. Distinct from the `ignored-file` path policy (§9.1,
which requires the path stay **inside** the repo), the executable
policy requires the opposite:

**Shared resolution prefix** (steps 1-4, identical for `add
--trust-current-dolt` and `capture`):

1. `exec.LookPath` result, then `filepath.EvalSymlinks` on it (unlike
   the ignored-file gate, symlinks in the *executable's* resolution
   path are followed and the *resolved* target is what's validated —
   an external tool is expected to be installed via a symlink chain,
   e.g. a version manager's shim).
2. The resolved path must be a regular file with at least one
   executable bit set (`os.Stat(...).Mode()` — regular file,
   `mode&0o111 != 0`).
3. The resolved path must **not** be inside the repository working
   tree or under any `.git` directory anywhere on the filesystem
   (`adapter-executable-in-repo`, exit 3) — the opposite refusal
   direction from `ignored-file`'s containment requirement, and
   deliberately so: a trusted external tool must live outside the
   tree an attacker (or a merge/checkout) could have just modified.
   Any path outside the repository is accepted regardless of how deep
   the symlink chain that led there was.
4. Not found at all (`exec.LookPath` fails): `adapter-missing` (exit
   3, at `capture`); at `add --trust-current-dolt`, the same
   underlying `LookPath` failure is a **distinctly named** validation
   error, `adapter-missing-at-add` (exit 2, rev-10, task 3 — corrects
   rev-8's overload of the single name `adapter-missing` across two
   different exit codes, which the "one reason name/one exit code/one
   normative table row" convention this PRD otherwise holds itself to
   does not permit; resolving *some* `dolt` binary to hash is a
   precondition of the flag succeeding at all).

**Executed-binary binding: hash the exact bytes executed, not a
separately-resolved pathname twice** (rev-7, task 3 — supersedes
rev-6's "hash the resolved pathname before invocation, then re-hash
the same resolved pathname again after invocation" design, which the
rev-6 adjudication found never actually bound the hash check to *the
bytes the child process itself ended up executing*: two hashes of a
pathname, taken at two different times, do not prove nothing was
substituted in between, or that `cmd.Start()`'s own internal `PATH`/
path resolution opened the identical inode either check observed).
Once resolved and validated per steps 1-4, the adapter never executes
the resolved pathname directly at either call site. The **stream-copy-
while-hashing** primitive (task 3, C37) below is shared by both
sequences; only the capture-time sequence goes on to harden and
execute the copy.

**Add-time trust bootstrap (TOFU)** (rev-8, task 3 — the entire body of
what `add --trust-current-dolt` runs, and nothing more):

1. Run the shared resolution prefix (steps 1-4 above). There is **no**
   "refuse if unpinned" check here — an add-time bootstrap is, by
   definition, the operation that establishes the *first* pin; there
   is nothing to check it against yet.
2. **Open** the resolved path (`os.Open`, read-only).
3. **Hash the opened descriptor directly, no private copy** (rev-10,
   task 4 — simplifies rev-8's design, which unnecessarily created and
   then deleted a private ephemeral copy at add-time even though
   nothing ever executed it): `io.Copy(sha256Hasher, sourceFile)` — a
   single streamed pass through the already-opened source descriptor,
   discarding the bytes into the digest and nowhere else (`io.Discard`
   is the effective destination; no file under `es_<id>/` is ever
   created for this bootstrap). This is deliberately the **only**
   difference from the capture-time sequence below: both hash the
   exact bytes of the same opened descriptor via the identical
   `io.TeeReader`/`sha256.New()` primitive (C37), but the bootstrap has
   no reason to materialize a second, executable copy of bytes it will
   never execute.
4. **Write the pin**: the completed `SHA-256` hex digest becomes
   `trust.binary_sha256` in the resource's declaration (§4, §12.1),
   written under the same per-slug `flock` (§7.2) every `add` acquires.
5. **Close the source descriptor** (`os.Open`'d in step 2) — there is
   no private copy to delete, because none was ever created. **No
   Dolt binary is ever executed during `add`, and no `es_<id>/`
   scratch directory or file is ever created for this bootstrap**
   (rev-10 corrects rev-8's "the bootstrap never `chmod`s it to
   `0500`" framing, which implied a copy existed and was merely never
   hardened — no copy exists at all); `add --trust-current-dolt` only
   ever resolves, opens, and hashes.

**Capture-time trust verification and execution** (rev-7, task 3/4;
private-copy host-failure handling added rev-8, task 6) — this is the
sequence a `capture` (or `--dry-run`, §3) runs for every Dolt
invocation, and the **only** sequence that ever executes a Dolt
binary:

1. **Refuse if unpinned**: if the resource's `trust.binary_sha256`
   (§4, §12.1) is `null`/absent, refuse `dolt-trust-required` (exit 3
   — distinct from `add`-time's exit-2 `dolt-trust-flag-required`
   refusal for a missing `--trust-current-dolt` flag, rev-8, task 3/5:
   the two are different named errors at different validation layers,
   not one name shared across two exit codes) before opening anything.
2. **Open** the resolved path (`os.Open`, read-only) — this is the
   file descriptor every subsequent step operates on; no step
   re-resolves the pathname from scratch after this point.
3. **Preflight the scratch filesystem for `noexec`** (rev-8, task 6):
   before creating the private copy, `syscall.Statfs`/`Statfs_t` (§9.2,
   same stdlib-only primitive already used for the local-root `statfs`
   preflight, §7.1) the `es_<id>/` scratch directory and inspect the
   platform's mount-flags field — Linux's `Statfs_t.Flags` bit
   `ST_NOEXEC` (value `0x8`; documented in the `statfs(2)` man page's
   `f_flags` description, "Execution of programs is disallowed on this
   filesystem", present since Linux 2.6.36 — https://man7.org/linux/man-pages/man2/statfs.2.html)
   or Darwin's `Statfs_t.Flags` bit `MNT_NOEXEC` (value `0x00000004`,
   `sys/mount.h`). Either bit set refuses `adapter-copy-noexec` (exit
   3) **before** the private copy file is created at all — creating an
   executable-intent copy on a filesystem the OS has already marked
   non-executable can only fail later, and more confusingly, at
   `cmd.Start()`.
4. **Stream-copy while hashing, then `Sync`** (task 3, C37; mode
   sequence corrected rev-10, task 4): create the private,
   per-invocation ephemeral scratch file under `es_<id>/` (§7.1, mode
   `0700` for the containing directory, the file itself created via
   `os.OpenFile(..., O_CREATE|O_EXCL|O_WRONLY, 0600)` — the same file
   default as every other scratch file in this tree, §7.1; rev-10
   replaces rev-8's direct-`0500`-at-creation design, which left the
   descriptor's writability during the copy resting on the correct
   but easy-to-misread POSIX rule that permission bits are checked
   only at `open()` time, not on every subsequent write), and copy the
   opened source descriptor's bytes into it via `io.Copy(privateFile,
   io.TeeReader(sourceFile, sha256Hasher))` — a single pass that both
   writes the private copy and feeds every byte through a running
   `SHA-256` digest (C37), so the digest computed is provably the
   digest of the exact bytes that land in the private copy; there is
   no separate "hash the source, then separately copy the source"
   two-pass design that could itself race a substitution of the source
   between the two passes. Once the copy completes, `Sync` the file
   (durability of the copy's bytes before anything downstream trusts
   them). A copy I/O failure (`ENOSPC`/`EIO` from `io.Copy`, `Sync`, or
   `Close`) is `adapter-copy-failed` (exit 1 — a host/environment
   fault, not a trust or policy refusal) with best-effort cleanup of
   the partial copy; no invocation is attempted.
5. **Verify before finalizing**: compare the completed digest against
   `trust.binary_sha256`. A mismatch is `adapter-binary-untrusted`
   (exit 3) — the partial/complete private copy is deleted
   (best-effort) and **no invocation is attempted at all**; no process
   is started, no output exists to discard.
6. **Harden the copy via descriptor-based `Fchmod`, then verify/close**
   (rev-10, task 4): on a match, call `f.Chmod(0o500)`
   (`Fchmod` on the still-open file descriptor `f` from step 4 — never
   a path-based `os.Chmod`, which re-resolves the pathname and could
   itself race a swap of the scratch entry between the copy finishing
   and the `chmod` call) to reduce the copy to owner read+execute
   only, confirm the mode change via `f.Stat()`, then close the
   descriptor. From this point the copy cannot be rewritten by this
   process (or, since it lives under the `0700` `es_<id>/` scratch
   directory, by any other process running as a different OS user)
   without first restoring write permission. No unverified bytes are
   ever made executable: the file is `0600` (not executable at all)
   for the entire duration of the copy and digest comparison, and only
   becomes `0500` after the digest has already matched.
7. **Optionally re-verify immediately before exec** (task 3 — adopted
   in this design as a cheap, additional closure of the narrow window
   between step 6 and `cmd.Start()`): re-open the private copy
   read-only, re-stream it through a fresh `SHA-256` digest, and
   compare against `trust.binary_sha256` once more; a mismatch at this
   point is also `adapter-binary-untrusted` (exit 3), copy deleted, no
   invocation attempted.
8. **Execute the private copy's path**, never the originally
   `LookPath`/`EvalSymlinks`-resolved pathname — `cmd.Path` (and
   `argv[0]`) is set to the private copy's own path under `es_<id>/`.
   This is what closes rev-6's residual: the exact bytes that were
   hashed and verified (steps 4-5, optionally reconfirmed in step 7)
   are the exact bytes `cmd.Start()` opens and executes, because they
   are now the *only* copy of those bytes this invocation ever asks
   the kernel to run — there is no second, independent path-resolution
   step for the kernel to race against a swap of the *original*
   `PATH`-resolved binary, since the original is never referenced
   again after step 2's open.
9. **Delete the private copy** (`os.Remove`, best-effort, like any
   other `es_<id>/` scratch content, §7.1) after the child process
   exits, on both the success and failure paths.

**Private-copy safety despite its scratch location, qualified** (rev-8,
task 6): the general executable-safety rationale above (step 3 of the
shared resolution prefix) requires the *originally-resolved* Dolt
binary to live outside the repository tree, because that tree is
attacker-reachable. The **private copy** the capture-time sequence
executes instead (step 8) necessarily lives *inside* `.tpatch/local/`
(§7.1) — itself repository-adjacent, though gitignored — and this is
safe for a different, narrower reason than "outside the tree": the
copy's bytes are descriptor-bound (copied from an already-`os.Open`ed
source descriptor, never re-resolved by pathname, step 2/4), hash-
verified against the operator's own pin immediately before creation is
finalized (step 5, optionally step 7), and owner-only (`0500`, no
group/other bits, under a `0700` parent). Its location does not weaken
these guarantees; what matters is that every byte executed traces back
to one continuous, never-re-resolved, hash-checked chain, not that the
path it temporarily occupies happens to sit under the repo.

**Residual, stated honestly** (task 3): this design closes the
"resolve, hash, then separately re-resolve-and-execute a possibly-
swapped pathname" TOCTOU rev-6 left open — the bytes hashed and the
bytes executed are now provably the same file. It does **not** claim
to close every conceivable local race: a concurrent process running
as the *same* OS user, with write access to the per-invocation
`es_<id>/` scratch directory, could in principle still interfere with
the private copy between step 5's `chmod 0500` and `cmd.Start()`'s
open of it (removing and recreating the same path, for instance).
This residual mirrors the general local-concurrent-attacker model
already accepted throughout this design (§9.1's `db_path` residual,
§6.4's process-group residual) and is not claimed to be fully closed
either — what **is** closed is the specific swap-the-originally-
resolved-binary-after-hashing-it TOCTOU the rev-6 adjudication flagged,
since resolution (step 2's open) and the bytes ultimately executed
(step 7) now trace back to one continuous, never-re-resolved chain.

Once resolved, verified, and (for the duration of the invocation)
copied, **tool identity** is a static file fact, never a
code-execution result (C12): `basename(resolvedPath)` (e.g. `"dolt"`,
the *original* resolved path's basename, not the private copy's
randomized scratch name) and the verified `SHA-256` hex digest from
step 3/4 above. The **resolved absolute path itself is never
tracked**, and neither is the private copy's own ephemeral path —
only `basename` and `binary_sha256` appear in any tracked artifact
(§12.3); both paths exist only in-process for the duration of the
invocation and, if either must appear at all for local debugging, only
in an ephemeral, redacted-if-possible local diagnostic that is itself
deleted before the command returns (§7.5).

**Trust pin is mutable metadata, excluded from identity** (rev-7, task
4 — supersedes rev-6's "`binary_sha256` is a normal `args` key,
merely recorded" **and** rev-6's later "trust pin, not bare identity
recording" design, both of which folded the pin into `resource_id`'s
hash input): the resource's declared `args` for `adapter-snapshot`/
`dolt` are `contract`/`db_path`/`table`/`from`/`to` (§6.2) — **never**
`binary_sha256`. The pin itself lives in the resource's separate
`trust` field (§4, §12.1), written by `add --trust-current-dolt` or
updated later by `trust-dolt` (§3), and is **excluded** from
`resource_id`'s canonical hash input (§13.2) entirely. Two
declarations differing only in `trust.binary_sha256` — including the
*same* declaration before and after a `trust-dolt` re-pin — share the
identical `resource_id`, by construction. `tool_identity.binary_sha256`
in every tracked result (§12.2, §12.3) is always identical to the
declaration's **pinned** value at the time of that capture (never a
freshly-recomputed value presented as if it might differ — a mismatch
during a capture is a refusal, §6.1 steps 4/6, never a divergent
tracked fact); a later `trust-dolt` re-pin does not retroactively
change any already-published batch's `tool_identity`, since batches
are immutable (§7.3).

**What the pin does and does not prove — a contract enum makes this
explicit** (task 5): the pinned `binary_sha256` is an
**operator-approved exact binary identity**, not a cryptographic proof
that the binary equals any specific pinned upstream source commit —
this design has no way to verify provenance beyond "this exact byte
sequence was explicitly approved." What the pin *does* establish is
which binary this design trusts to define Dolt's **semantic**
contract, named explicitly as a declared `contract` value (§6.2):
`"dolt-diff-summary-v1"` in v1, the specific query/schema shape this
PRD's C11/C15/C19–C27 source-verified claims were established against.
The strict five-field JSON parser (§6.3) is a **separate, independent
runtime capability gate**: even a correctly-pinned binary's output is
still fully validated against the exact `{from_table_name,
to_table_name, diff_type, data_change, schema_change}` shape before
any of it is trusted. A pinned binary that has silently drifted from
the commit its hash was originally approved against (e.g. an operator
mistakenly re-pinning a differently-behaving fork or a corrupted
rebuild that happens to still pass the parser) is a **disclosed
residual** (§15) — the parser catches a *structural* drift (wrong
shape, wrong types, extra/missing fields) but cannot catch a
same-shaped **semantic** drift (a build that answers the same query
shape with subtly different results). The pin establishes *which*
binary is allowed to run at all; the parser establishes that *what it
actually printed* still matches this design's contract; neither
substitutes for the other, and neither claims to prove the pinned
binary matches any specific upstream source revision.

There is no separate "probe" step at all (task 2, unchanged) — `dolt
version` is **never** executed (C12: it is a real subcommand that can
perform a network update check and read/write the resolved `HOME`,
which is not an acceptable side effect for an identity check). The
real SQL invocation in §6.2 **is** the capability check; a failure
there is reported through the same capability-failure taxonomy as any
other SQL error, not a distinct "probe failed" class.

### 6.2 Capability invocation — `diff-summary` (task 1, task 6, task 7, task 8)

**Declared fields** (all required — no optional Dolt argument remains
since rev-3, task 6; rev-7 adds one more, task 5): `contract` (the
closed, single-valued v1 capability enum — always the literal string
`"dolt-diff-summary-v1"`, §6.1; any other value is a validation error,
`dolt-contract-unsupported`, exit 2, at `add` time, **before**
`db_path`/`table`/`from`/`to` are even inspected — this is a v1
forward-compatibility seam: a future contract value would name a
**different** query/schema shape entirely, and this design refuses
silently guessing at one), `db_path` (repo-relative path to the Dolt
database directory; path-gated identically to an `ignored-file`
selector, §9.1, and used verbatim as the child process's working
directory, `cmd.Dir`), `table` (exactly one table name — **mandatory**,
no whole-database form; C20/C24 ground this choice, see below),
`from`, `to` (commit-ish values). Any other declared key, a missing
required key, or a duplicate `--arg` for an already-declared key is a
validation error (exit 2) at `add` time. `contract` participates in
`resource_id`'s hash input (§13.2) exactly like `db_path`/`table`/
`from`/`to` — it is a semantic-identity fact, unlike `trust`, which is
excluded (§4, §6.1).

One capability, one exact argv template, using the resolved absolute
Dolt path (never the bare string `"dolt"`, to avoid a second, redundant
`PATH` lookup at invocation time), run with `cmd.Dir = <repo-root>/<db_path>`:

```
<resolvedDoltPath> sql -r json -q "<SQL>"
```

Where `<SQL>` is exactly:

```sql
SELECT from_table_name, to_table_name, diff_type, data_change, schema_change
FROM dolt_diff_summary('<esc(from)>', '<esc(to)>', '<esc(table)>')
ORDER BY from_table_name, to_table_name;
```

This is the **only** shape emitted: there is no separate whole-database
invocation, no `--schema`/`--data`/`--name-only` flag combination (all
removed, C11), and no dot-range form (`dolt_diff_summary('from..to')`)
— dot-range is explicitly refused (see escaping rules below), never
generated by this PRD regardless of what a caller declares.

**Literal escaping and value validation** (`esc(...)`, task 1's "strict
SQL-literal escaping", task 6's `..`/control/backslash rejection): each
of `from`/`to`/`table` is validated **before** encoding, in this exact
order, any failure is a validation error (exit 2):

1. Reject any `NUL` byte or other C0 control character
   (`0x00`–`0x1F`, `0x7F`) — same discipline as §13.1's canonical
   `args` encoding.
2. Reject a literal backslash (`\`) anywhere in the value — rev-2's
   reasoning is unchanged: whether a backslash is itself an escape
   character inside a Dolt/MySQL string literal depends on the
   session's `sql_mode` (`NO_BACKSLASH_ESCAPES`), which this PRD does
   not control or verify; refusing is simpler and strictly safer than
   guessing.
3. Reject any value containing the literal two-character substring
   `".."` — **not merely defense in depth**: `dolt_diff_summary`'s own
   `WithExpressions` argument-count validation inspects the first
   argument's literal SQL-expression string for a `".."` substring to
   choose between its dot-range (1–2 arg) and explicit (2–3 arg) parse
   branches (C24, `dolt_diff_summary.go:220-238`); a `from` value that
   legitimately contained `".."` would misroute this design's fixed
   3-argument invocation into Dolt's dot-range parser and fail with an
   argument-count error unrelated to this PRD's own escaping. Refusing
   `".."` up front avoids that failure mode entirely, in addition to
   being a reasonable general hardening rule.
4. The only transform applied to an otherwise-valid value is doubling
   a single quote (`'` → `''`), the one escaping rule that is
   unambiguous under both interpretations of `sql_mode`.
5. **`WORKING`/`STAGED` refusal** (case-insensitive) applies to `from`
   and `to` specifically — see the dedicated subsection below; this is
   a distinct refusal (`dolt-argument-refused`, exit 2) from the
   escaping rules above, and is checked independently of them.

`db_path` is validated by the same ancestor-symlink/containment gate as
an `ignored-file` selector (§9.1) — it must resolve to an existing
directory inside the repository working tree; a `db_path` that fails
this gate is refused with the same taxonomy as `ignored-file`'s gate
(`symlink-component-refused`/`path-outside-repo`, exit 3), not a
separate Dolt-specific error class.

**Column schema (C15, source-confirmed)**: all five selected columns
are non-null and typed (`from_table_name`/`to_table_name` `LongText`,
`diff_type` `Text`, `data_change`/`schema_change` `Boolean`,
`dolt_diff_summary.go:48-54`), and the function reports
`IsReadOnly() == true` (`:115-117`). Because every column is
guaranteed non-null at the SQL layer, `result.tables[]` entries in the
tracked wire schema (§12.2/§12.3) never carry a JSON `null` for a returned
row — but see §6.3 for the add/drop empty-string convention, which is
a non-null empty string, not a null. The schema declaration establishes
the **column type**; see §6.3 (C25) for the separate, stronger claim
that the code path emitting each row also produces a **native Go
`bool`** for these two columns — grounded by the exact row-construction
site (`table_deltas.go`'s `sql.Row{...}` literal, not `RowSig`/schema
type declaration).

**Refs, `WORKING`/`STAGED` explicitly refused in v1 — committed refs
only (C19, rev-5 correction, rev-6/rev-7 scope note, task 6)**:
`dsess/session.go:1022-1031`'s `ResolveRootForRef` does accept the
exact, case-sensitive literal strings `WORKING` and `STAGED`
(`doltdb.go:51-52`) in addition to any ordinary commit-ish — this
remains a **true, source-confirmed fact** about upstream Dolt's
capability, unchanged from rev-4's citation, and is cited here again
(C19) purely as a **source fact establishing what upstream permits**,
not as a capability this design exposes: v1 explicitly refuses both
literals and is scoped to **committed refs only**. What rev-5
corrected was this PRD's own design choice: rev-4 stated `from`/`to`
**accept** either literal and pass it through; this design **refuses**
both (case-insensitively, so `Working`/`staged`/etc. are also caught
rather than silently falling through to a failed commit-spec
resolution), returning `dolt-argument-refused` (exit 2) before the
child process is ever started. The reason is the working tree/staged
index, unlike any committed ref, is itself gated by Dolt's own
`dolt_ignore` table (a Dolt-level analog of `.gitignore`) — a table
matched by `dolt_ignore` can be silently absent from `WORKING`/
`STAGED`'s row set the same way an ignored file is absent from a Git
working tree listing, and since this PRD already makes `table`
mandatory specifically so a primary-key-set change is a hard error
rather than a silent omission (C20 below), allowing a ref value that
reintroduces a different, independent silent-omission path would
undercut that guarantee for no compensating benefit; v1's committed-
ref-only scope (branch names, tags, full/abbreviated commit hashes)
side-steps `dolt_ignore` entirely, since ignore-table membership is a
working-set-only concept that a committed diff never consults. This
also bounds C21's "zero rows can mean nonexistent-or-ignored table"
ambiguity to committed refs specifically: for a committed `from`/`to`,
zero rows for a named table means only "the table did not exist in
that committed root," since `dolt_ignore` cannot suppress a row from a
committed-ref query the way it could from `WORKING`/`STAGED` — v1's
refusal of the latter is precisely what keeps this narrower, single-
cause reading of "zero rows" valid; it would **not** hold if
`WORKING`/`STAGED` were ever accepted, which is exactly why they
remain refused rather than merely discouraged. Accepting `WORKING`/
`STAGED` remains a plausible **future** capability (e.g. a v2 opt-in
that explicitly documents the `dolt_ignore` interaction rather than
silently inheriting it) but is out of scope for this PRD, not merely
deferred by omission — a caller that declares either literal today
gets a validation-time refusal, not a passthrough. §14 has a
dedicated AC/matrix row (C19 as a cited source fact, immediately
followed by the refusal AC) making this distinction explicit.

**Primary-key-set-change hard error (C20, source-confirmed, task 12)**:
because `table` is now mandatory, every invocation takes the single
code path that emits the 3-argument form (`tableNameExpr != nil`,
`dolt_diff_summary.go:300-320`), where `shouldErrorOnPKChange` is
`true` (line 311); a primary-key-set change on the requested table
between `from` and `to` therefore surfaces as a hard Dolt query error
(wrapping `diff.ErrPrimaryKeySetChanged`, `"primary key set changed"`,
`diff/diff_stat.go:31`) rather than a silently omitted row (which is
what the unconfirmed, now-removed whole-database form would have
done). This is deliberately cited as "the 3-argument form" rather than
"the single-table path" (rev-3's phrasing) because those two
descriptions are now exactly the same thing under this design's
mandatory-`table` constraint — there is no other invocation shape this
PRD ever emits. This capability reports that outcome as
`dolt-query-error` (exit 3) with the Dolt error text captured only in
local, ephemeral diagnostics (§7.5) — never in the tracked artifact.
§14 has an explicit `AC`/matrix row exercising this exact case.

**Nonexistent table (C21, source-confirmed)**: a `table` naming a
table that exists in **neither** `from` nor `to` (both **committed**
refs, per the scope note above) yields **zero rows**, not an error:
`findMatchingDelta` (called from the 3-arg branch,
`dolt_diff_summary.go:301`) returns a delta with no `FromTable`/
`ToTable`/`FromRootObject`/`ToRootObject` populated for a name unknown
to either root, and `getSummaryForDelta`'s own early `return nil, nil`
for exactly that all-nil case (`:346-350`) means the single-table
call's `summs` slice — which only appends when the returned summary is
non-nil (`:313-318`) — stays empty. This is a distinct, third outcome
from C20's hard error and is the "first capture" / "table never
existed at `from`" case: the tracked `result.tables` array is simply
empty (`[]`, never `null`, task 14) — no special-cased schema. Because
v1 refuses `WORKING`/`STAGED` (above), this zero-rows outcome for a
committed `from`/`to` has exactly one cause — the table did not exist
in either committed root — and cannot also mean "the table exists but
is `dolt_ignore`d," which is a limitation only reachable through a ref
kind this design does not accept.

**Rename detection, closed `diff_type` enum (C23/C26, source-confirmed)**:
`diff_type` is a **closed** 4-value string enumeration —
`"added"`, `"modified"`, `"renamed"`, `"dropped"` — evidenced by the
four **exact assignment lines** inside `GetSummary`, the only places
any row's `DiffType` field is ever set: `table_deltas.go:722`
(`DiffType: DiffTypeDropped`), `:733` (`DiffTypeAdded`), `:745`
(`DiffTypeRenamed`), `:760` (`DiffTypeModified`) — a stronger citation
than rev-3's `const`-block-only citation (`:45-51`), since that block
also declares a fifth constant, `DiffTypeAll = "all"`, that is **never
assigned** to any row's `DiffType` field anywhere in this file (an
exhaustive grep of every `DiffType` assignment confirms this) — it
exists solely as a caller-side **filter value** for a different
function (i.e. a value a caller could pass to *select* "all" diff
types when querying, never a value Dolt itself *emits* as a row's
`diff_type`) and this design's fixed `dolt_diff_summary` query neither
emits it nor needs to recognize it as a possible row value. For
`"dropped"`, `to_table_name` is the empty string `""` (not `null`, not
omitted); for `"added"`, `from_table_name` is `""`; for `"renamed"`/
`"modified"`, both names are populated (differing for a rename,
identical for a modify) — this asymmetry is source-confirmed
(`table_deltas.go:716-760`, `doltdb/root_val.go:797-800`'s
zero-value-stringifies-to-empty convention). Despite now being
source-confirmed and closed, `diff_type` is still tracked **verbatim**
rather than validated against this set (forward-compatible if a
future Dolt version adds a fifth value) — a stricter, better-cited
version of rev-2's existing posture, not a reversal of it.

**Dolt-specific argument refusals share one named exit-2 outcome**
(task 10): `dolt-argument-refused` is the single named refusal used
for every Dolt-argument-shape rejection this design defines —
`WORKING`/`STAGED` (above), the `".."` substring rule, and any other
argument-validation rejection specific to the Dolt capability (as
distinct from generic escaping/control-character rejections, which use
the ordinary validation-error exit 2 without a Dolt-specific named
reason). §9.2's exit-code table lists this name explicitly (task 10).

### 6.3 Output parsing and normalization (task 4)

`dolt sql -r json` wraps a **nonempty** result as a single top-level
JSON object with exactly one key, `"rows"`, an array of row objects
(`table/typed/json/writer.go:37-38`, doc comment at `:56-58`/`:62-64`
confirms "a single JSON object with a single key: \"rows\""); for a
**zero-row** result, the caller writes the literal, distinct 2-byte
string `{}` directly, guarded on `numRows == 0`
(`engine/sql_print.go:110-113`, `:148-149`) — **there is no `"schema"`
key in either case** (C22, correcting rev-2's community/docs-based
guess). **The captured buffer carries trailing whitespace beyond the
JSON body itself** (C27, task 4): the row writer's `Close` emits only
the literal footer `]}` with no added newline
(`table/typed/json/writer.go:243-249`); the one-shot `-q` invocation
path (`execSingleQuery`, `commands/sql.go:452-470`) calls
`PrettyPrintResults`, which passes `PrintNoSummary`
(`engine/sql_print.go:59-61`), so no "N rows in set" line is appended —
but `sql_print.go`'s final, unconditional
`case FormatJson, ...: return iohelp.WriteLine(cli.CliOut, "")`
(`:168-170`) always appends one more `"\n"` regardless of row count,
and `WriteLine` (`iohelp/write.go:66-68`) itself always appends `"\n"`
to whatever it writes — so the real zero-row capture is `"{}\n\n"`
(two trailing newlines), not a bare 2-byte string, and the real
nonempty capture is `"...]}\n"` (one trailing newline). The parser
therefore **trims leading and trailing ASCII whitespace from the
captured buffer before** attempting to match either valid top-level
shape — this is grounded in the cited real output shape, not a purely
defensive guess. After trimming, the parser recognizes exactly these
two valid top-level shapes and treats any other top-level shape — a
missing `"rows"` key where one is expected, a `"schema"` key that does
not exist in the real output, extra unknown top-level keys, or
`"rows"` present but not a JSON array — as a fatal
`dolt-json-parse-error` (exit 3), never a best-effort partial parse.
Trimmed `{}` maps deterministically to `result.tables: []`.

For a nonempty `"rows"` array, each row object must contain **all
five** fields (`from_table_name`, `to_table_name`, `diff_type` as JSON
strings; `data_change`, `schema_change` as JSON booleans) — a missing
field, an unknown extra field, a duplicate key, or any field present
with the wrong JSON type (e.g. `data_change` as `0`/`1` instead of a
native boolean) is a fatal `dolt-json-parse-error` (exit 3); this PRD
does **not** defensively coerce `0`/`1`/`"true"`/`"false"` to boolean
(rev-2's defensive-coercion design is removed) — the **row
constructor** that builds each output row, `getRowFromSummary`
(`dolt_diff_summary.go:457-464`), passes `ds.DataChange`/
`ds.SchemaChange` — native Go `bool` fields on `TableDeltaSummary`
(`table_deltas.go:83-90`) — directly into the row with no intermediate
string/int conversion (C25, a stronger citation than rev-3's
schema-type-only citation, `dolt_diff_summary.go:48-54`, which shows
only the declared column *type*, not what a specific code path
actually writes), so a non-boolean value in that position indicates a
real parsing/version mismatch that should fail loudly rather than be
silently normalized.

### 6.4 Timeouts, caps, environment (task 2, task 7)

| Parameter | Value |
|---|---|
| Invocation timeout | 30 seconds. On timeout: `SIGTERM` to the process group, then `SIGKILL` after 2 more seconds if still running. |
| Captured output cap | 5 MiB combined stdout+stderr, enforced as a **refusal, never a truncation** (rev-5 correction — rev-4's "output beyond the cap is truncated" contradicted its own "zero pre-scan persistence"/fail-closed framing, since silently truncating and proceeding is itself a form of trusting unbounded input up to the point of truncation). The adapter uses caller-owned `os.Pipe()` pairs assigned directly to `cmd.Stdout`/`cmd.Stderr` (rev-10: **not** `cmd.StdoutPipe()`/`cmd.StderrPipe()`, and never `cmd.Stdout`/`cmd.Stderr` set to a `*bytes.Buffer`, which has no built-in bound — see the Termination row below for why the caller-owned pipe mechanism is retained) with two concurrent goroutines draining each pipe into a **shared** cap-plus-one budget (a single `int64` counter both readers atomically decrement, so stdout and stderr together, not each independently, are bounded by the 5 MiB total) — task 2's "zero pre-scan persistence," never written to a scratch file first, §7.1/§8. The **instant** the shared budget's `limit+1`th byte is actually read from either pipe, the adapter signals the output-cap-exceeded trigger into the unified termination sequence below (Termination row) — the same group-signal-then-single-`cmd.Wait()` sequence a timeout or a naturally-exiting leader triggers — refusing the whole invocation with `resource-limit-exceeded` (exit 3) once that sequence completes: no partial/truncated output is ever handed to the JSON parser (§6.3) or scanned for redaction (§8), and no tracked artifact reflects a truncated result. **stdout and stderr are captured and bounded identically, but are never merged for parsing purposes**: only the stdout buffer is ever handed to §6.3's JSON parser; the stderr buffer exists solely to be scanned for redaction (§8.3) and, on a non-zero exit or a `dolt-query-error`, to populate the local, ephemeral diagnostic (§7.5) — it is never itself parsed as JSON and never influences whether stdout parses successfully. |
| Environment | **Not** inherited from the invoking process (task 2's "no inherited credentials"). A fresh, minimal environment is constructed: `HOME=<scratch-home>` and `DOLT_ROOT_PATH=<scratch-home>` pointing at a directory created fresh under this invocation's ephemeral scratch tree (§7.1, `0700`, created before the child process starts so Dolt may write its own ephemeral config/state there if it chooses to — this is not a network or version call, just process-local state under an isolated `HOME`); `PATH` is **not** set at all (the adapter is invoked by its already-resolved absolute path, §6.1, so `PATH` lookup is never needed mid-invocation). No other variable is passed through. |
| Termination | **One unified process-group cleanup sequence for every invocation, success or kill-triggered alike, using a non-reaping `waitid`/`WNOWAIT` observer instead of an early `cmd.Wait()`** (rev-8 task 2 unified the two paths rev-7 left separate; rev-9 decoupled leader-exit detection from pipe drain via caller-owned `os.Pipe()`s but called `cmd.Wait()` as soon as that detection fired — the rev-9 adjudication found this reaps the leader, releasing its PID/PGID back to the kernel, **before** the group-signal sequence has necessarily run, reopening a PGID-reuse race; rev-10 fixes this while keeping the caller-owned pipes). Before `cmd.Start()`, the adapter sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` (C31; `linux`/`darwin` only, matching §7.2's build-tag contract — unsupported platforms are already refused before any Dolt invocation is reachable). This makes the spawned child the leader of a **new** process group whose PGID equals the child's own PID, distinct from `tpatch`'s own process group. **Pipe setup (kept from rev-9, task 2)**: instead of `cmd.StdoutPipe()`/`cmd.StderrPipe()`, the adapter creates two ordinary `os.Pipe()` pairs itself, assigns each write end directly to `cmd.Stdout`/`cmd.Stderr` (as `*os.File`, not via the `Cmd`-managed accessor methods) before `Start()`, and — immediately after `Start()` returns — closes its own reference to each write end (the child's dup'd fd keeps the pipe alive for as long as the child, or any descendant it forks that inherits the same fd, holds it open). This decoupling (C40) remains true and useful — `cmd.Wait()`, whenever it is eventually called, does not itself block on pipe drain — but rev-10 no longer treats "decoupled from pipes" as license to call `cmd.Wait()` the instant leader-exit is detected: doing so is exactly the PGID-reuse defect above, independent of pipes. **Non-reaping leader-exit observer (new, rev-10, task 1, C41)**: instead of running `cmd.Wait()` in a goroutine, the adapter runs a build-tagged (`//go:build linux \|\| darwin`) observer goroutine that calls a small stdlib-only helper wrapping the raw syscall `waitid(P_PID, leaderPID, &buf, WEXITED\|WNOWAIT)` — `syscall.Syscall6(syscall.SYS_WAITID, uintptr(1 /* P_PID */), uintptr(leaderPID), uintptr(unsafe.Pointer(&buf[0])), uintptr(syscall.WEXITED\|syscall.WNOWAIT), 0, 0)`, where `buf` is a discarded, unparsed `siginfo_t`-sized scratch buffer (only the syscall's own success/failure matters; the leader's actual exit status is still collected later, via the single `cmd.Wait()` below) — no `golang.org/x/sys` dependency, matching this project's stdlib-only rule (C41). This call **blocks until the leader exits but does not reap it**: `WNOWAIT` leaves the leader in a waitable (zombie) state, so its PID — and, since `Setpgid:true` makes the PGID equal the leader's PID, the process group's own numeric identity — is **not** released back to the kernel's PID-reuse pool merely because this observer has fired; a second, immediately-repeated call to the same observer would also succeed instantly, proving the leader was not consumed by the first call (verified empirically on this project's `darwin` development host, C41; Linux behavior is grounded in the identical, publicly documented `waitid(2)`/POSIX `WNOWAIT` contract — "leave the child in a waitable state" — via the same `syscall.WEXITED`/`syscall.WNOWAIT`/`syscall.SYS_WAITID` stdlib constants, not independently executed on a Linux host in this planning pass). The observer's goroutine reports "leader has exited (not yet reaped)" over a channel, and the adapter races that channel (via `select`) against the pre-existing 30-second invocation timer and the output-cap-exceeded signal from the two independent pipe-draining goroutines (§8.1) — **whichever of these three fires first** enters the **one** unified cleanup function below; there is no branch-specific cleanup logic. **Unified cleanup function** (rev-10, task 1): (1) `syscall.Kill(-pgid, syscall.SIGTERM)` — sent every time regardless of which of the three events triggered entry, tolerating `syscall.ESRCH` (the whole group has already exited on its own); (2) a fixed grace period (2 seconds, unchanged), during which the leader remains unreaped (no `cmd.Wait()` call has been made yet, by construction — the observer that fired does not reap, and no other reap has occurred); (3) `syscall.Kill(-pgid, syscall.SIGKILL)`, again tolerating `syscall.ESRCH`; (4) **await the non-reaping observation if it has not already fired** — if the timeout or output-cap trigger is what entered cleanup (the leader may still have been running at that instant), the same non-reaping `waitid`/`WNOWAIT` call (idempotent, per above) is awaited now, and is guaranteed to return promptly once the `SIGKILL` in step 3 takes effect, since `SIGKILL` cannot be caught, blocked, or ignored; if the leader-exit branch is what fired, this observation is already available and this step is a no-op; (5) join the two pipe-draining goroutines to their own real completion (now unblocked, since every process that could hold either pipe's write end open — the leader and any descendant sharing its PGID — is confirmed dead by step 4); (6) call `cmd.Wait()` **exactly once**, for the first time in the entire invocation, to finally reap the leader and collect its real exit status. Because no step before (6) ever reaps the leader, its PID (and PGID) remains reserved by the kernel for the leader throughout the entire `SIGTERM`→grace→`SIGKILL` window — **this is what closes the PGID-reuse race rev-9 reopened**: there is no interval in which the leader has been reaped (freeing its PID/PGID for kernel reuse) while a subsequent group-signal call targeting that same numeric PGID is still pending. **There is no "`Wait()`" — nor the non-reaping observer — "proves group emptiness" claim anywhere in this design**: neither is used to infer that the process group has no remaining members; the design's actual, defensible safety property is narrower and purely sequential — **the unconditional `SIGTERM`→grace→`SIGKILL`(-pgid) sequence always completes, targeting a PGID the kernel has not yet been able to recycle, before the leader is ever reaped** (C36) — not a claim that this sequence itself proves the group empty at any instant. Signaling `-pgid` reaches only the Dolt child and any of its own descendants that remain in the same group (e.g. a sub-process Dolt itself spawns) — it never reaches `tpatch`'s own process, `tpatch`'s own process group, or a parent shell. **Start failure**: if `cmd.Start()` itself returns an error, no leader process exists at all — no observer is launched, no signal is ever sent, and `cmd.Wait()` is never called for that invocation. **A disclosed trade-off, carried forward**: a fully successful invocation whose leader exits with no lingering descendants still incurs the (now-redundant, but harmless, since the group is already empty) `SIGTERM`/grace/`SIGKILL` calls before the single `cmd.Wait()` — accepted as the cost of one unified code path rather than a branch-specific fast path. **Residuals, stated honestly**: (a) a **new** process spawned by a group member *after* `SIGKILL` was delivered but *before* the kernel has fully torn down the group is a kernel-level race this design does not claim to close, bounded by the OS's own signal-delivery/process-teardown semantics, not by anything `tpatch` controls; (b) a descendant that has **escaped the process group** before cleanup runs — for example by calling its own `setsid()`, or by being re-parented into a different session/group — no longer shares the leader's PGID and is **not** reached by `-pgid` signaling at all; this design signals the group the leader was created into at `Setpgid:true` time, not every process transitively descended from it, and does not claim to reach a descendant that has deliberately detached itself from that group. **Verification**: this design's own tests cover all three trigger branches through the **same** unconditional cleanup function — (a) a successfully-exiting leader whose descendant ignores `SIGTERM`, closes its own pipes only once killed: assert the non-reaping observer's channel fires first, `cmd.Wait()` has **not** been called at that point, the unconditional cleanup sequence still runs and kills the descendant, and `cmd.Wait()` is called exactly once, strictly after that sequence completes; (b) the pre-existing timeout-triggered case, with the leader itself still running when the timer fires; and (c) the pre-existing output-cap-exceeded case; in all three, assert the `tpatch` test-runner process itself is never signaled or observably affected, that the group-signal sequence and its `-pgid` argument are byte-for-byte identical across all three branches, and that `cmd.Wait()` is observed exactly once per invocation, always as the last event in the sequence. A separate cross-compile/source-shape test (AC-106) confirms the build-tagged observer's source layout compiles for both `linux` (`amd64`/`arm64`) and `darwin` (`arm64`) with no external dependency. |

A concrete, fully-specified argv/SQL example for Vector 2 (§0.3) —
`contract=dolt-diff-summary-v1, db_path=data/dolt-db, table=users, from=main, to=HEAD` (trust pin `binary_sha256` lives in the separate `trust` field, §4/§6.1, and never appears in `args` or this argv):

```
cwd:  <repo-root>/data/dolt-db
argv: /usr/local/bin/dolt sql -r json -q "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change FROM dolt_diff_summary('main', 'HEAD', 'users') ORDER BY from_table_name, to_table_name;"
```

(the absolute path shown is illustrative only, and is in any case
never the path actually executed — §6.1 executes a private, hash-
verified copy under ephemeral scratch, never this resolved pathname
directly, and neither path is ever the tracked value.)

## 7. Ephemeral Scratch, Locking, and the Single Publication Point (task 1, task 2, task 5, task 6, task 7, task 9)

### 7.1 Ephemeral scratch layout and lifecycle (task 1, task 2, task 6, task 9)

`.tpatch/local/` is the existing gitignored local root (`LocalIgnoreRule`,
`internal/workflow/session_ignore.go:18`). Before this invocation's
first write anywhere under it — including the `.lock` file itself,
before its very first creation for a given slug — `add`/`remove`/
`clear`/`trust-dolt`/`capture`/`record --resources` (task 8, rev-10:
**every** mutator, not only `capture`/`record --resources` — this
enumeration itself omitted `trust-dolt` through rev-9, a distinct gap
from §7.2's own "every mutating verb" lock list, which already
included it since rev-7) reuse
`workflow.EnsureLocalIgnoreContract` (unchanged reuse from rev-2/rev-3;
this is a deliberate reuse of the **existing** local-ignore mechanism,
not a second, parallel one) to confirm the local root itself is both
ignored and untracked — a local root that is somehow tracked or not
ignored is refused (`local-root-not-ignored`/`local-path-tracked`,
exit 3) before any scratch content, including the lock file, is
created, matching §10.3's row for this exact case.

**First-create sequencing — leaf-targeted ignore gate + subtree-targeted untracked gate, statfs on nearest ancestor** (rev-7, task 1; ignore/untracked target split, rev-8, task 7 — corrects rev-6's design, which ran **both** the ignore/untracked gate and the `statfs` preflight against the nearest existing ancestor, and rev-7's design, which correctly leaf-targeted the ignore check but left the untracked check's target ambiguous/inconsistent with §10.3. `EnsureLocalIgnoreContract` (the ignore half) is a pathname check whose semantics are **existence-independent** — `git check-ignore` answers "would this path be ignored if it existed" without requiring the path to exist first, so there is no technical reason to weaken it to an ancestor; the untracked half (§10.3 step 2) answers a **different** question — "is anything tracked under this whole gitignored root" — which is naturally subtree-scoped, not leaf-scoped, and is likewise existence-independent (`git ls-files` over an empty/nonexistent subtree simply reports empty stdout); `statfs`, by contrast, is a **kernel syscall on an existing inode** and genuinely cannot run against a not-yet-created leaf):

1. **Ignore gate — targets the intended leaf directly, existence-independent** (task 1): run `EnsureLocalIgnoreContract` (§10.3 step 1) against the **exact** intended path,
   `.tpatch/local/resource-scratch/<slug>/` (the directory about to
   contain `.lock`), regardless of whether it currently exists on
   disk. On a fresh clone this path does not yet exist, but the check
   is pathname-based and answers correctly regardless (Git's own
   `check-ignore` semantics do not require the target path to exist).
   **Untracked gate — targets the whole `.tpatch/local/` subtree,
   also existence-independent** (rev-8, task 7): run the separate
   `ls-files`-based untracked gate (§10.3 step 2) against
   `.tpatch/local/` as a whole, not the per-slug leaf — a tracked file
   under any slug's scratch tree is refused the same way regardless of
   which slug is currently being mutated. A failure in either half
   (`local-root-not-ignored`/`local-path-tracked`, exit 3) is refused
   **before** any directory in the scratch tree, including
   `.lock`'s own containing directory, is created — this closes
   rev-6's AC-81/82 gap, where the gate ran against whatever ancestor
   happened to already exist (potentially the repository root itself
   on a truly fresh clone), which does not establish anything about
   the specific, not-yet-existing leaf that is actually about to
   receive untracked, ignored content.
2. Walk upward from that same intended leaf toward the repository root
   until an **already-existing** directory is found — on a fresh clone
   this is typically the repository root itself, or `.tpatch/local/`
   if a prior invocation (for this or a different slug) already
   created it. This nearest-existing-ancestor **is used for `statfs`
   only** (§7.2's filesystem-type preflight) — a genuinely
   existence-bound kernel call that cannot target a path that does not
   yet exist; unlike the ignore/untracked gate above, there is no way
   to avoid this narrowing for `statfs` specifically.
3. Only after **both** checks pass (leaf-targeted ignore/untracked in
   step 1, ancestor-targeted `statfs` in step 2) does `os.MkdirAll`
   create every missing intermediate directory down to and including
   `es_<id>/` (and `dolt-home/` inside it, when needed), each created
   `0700` **at creation** (task 8, unchanged from rev-3/rev-4).
4. **`fsync` every newly-created directory in the chain, including
   already-visible directories on a retried invocation** (rev-7, task
   8 — refines rev-6's "fsync every newly-created directory's parent"
   to be explicit about the retry case): after `MkdirAll` returns, the
   adapter opens and `fsync`s each directory from the deepest
   newly-created directory back up to (and including) the nearest-
   existing-ancestor found in step 2 — **and**, for every directory in
   that same chain that already existed before this invocation's
   `MkdirAll` call (e.g. a retried invocation after an earlier crash
   left some but not all of the chain durable), re-`fsync`s it too,
   rather than assuming an already-visible directory implies an
   already-durable one: a directory can be visible to a subsequent
   `Stat`/`Lstat` immediately after creation on many filesystems well
   before the kernel has made that visibility crash-durable, so a
   retry that only fsyncs newly-created entries could still leave an
   earlier, not-yet-durable creation permanently lost across a second
   crash. **First/retry crash sequence, stated exactly**: (a) first
   invocation creates the whole chain via `MkdirAll`, then fsyncs every
   directory in the chain from leaf to ancestor; if it crashes before
   this fsync sequence completes, some prefix of the chain may not be
   durable; (b) a retried invocation re-runs `MkdirAll` (a no-op for
   any directory that already exists) and then re-runs the full
   fsync-every-directory-in-the-chain sequence unconditionally,
   regardless of which directories `MkdirAll` actually had to create
   this time — this guarantees that after any retry's fsync sequence
   completes, the entire chain is durable, even if the first attempt's
   fsync sequence was itself interrupted partway through. The same
   discipline applies identically to the tracked
   `artifacts/resource-captures/`/`batches/` tree's own first-ever
   creation (below).

**First-publication crash row** (task 8): the analogous first-ever
creation of the **tracked** `artifacts/resource-captures/` tree (its
`batches/` subdirectory and the very first `current.json`) has the
identical retry-fsync discipline — a first-ever `capture`/
`record --resources` for a slug that has never captured before must
`MkdirAll` these tracked directories before §7.3 step 3/4 can rename
anything into them, subject to the same crash-before-durable window
and the same unconditional re-fsync-on-retry rule as the local scratch
tree above. §7.3's crash-window table (below) includes this case
explicitly rather than only covering steady-state file renames into
an already-existing tree.

Rev-3 removed rev-2's per-resource `raw`/`files/<relpath>` scratch
files entirely (task 2's "zero pre-scan persistence" — see §8); rev-4
additionally replaced rev-3's temp-directory/`owner.json`/PID-based
lock protocol with a single, persistent, kernel-`flock`'d file (§7.2,
task 1). Rev-5 corrects a diagram error rev-4 introduced: the
tracked-batch and tracked-pointer temp files were shown living under
this **local**, gitignored scratch tree, but §7.3 has always created
them beside their **tracked** destination (`artifacts/resource-
captures/batches/<batch_id>.tmp-*.json` and `artifacts/resource-
captures/.tmp-current.json`, so that the final rename is a
same-directory, filesystem-atomic operation) — the local scratch tree
never contains either of them. The local scratch tree now holds
**only** control data, never a captured byte and never a tracked-temp
file:

```
.tpatch/local/resource-scratch/<slug>/
  .lock                            -- persistent, ignored+untracked control file, created once and never deleted (§7.2)
  es_<12 lowercase hex>/           -- one ephemeral-scratch directory per in-progress capture/record-resources invocation
    dolt-home/                    -- scratch HOME/DOLT_ROOT_PATH for the Dolt adapter (§6.4); may contain Dolt's own config/state files, never repo content
    dolt-copy-<12 lowercase hex>   -- private, hash-verified copy of the resolved Dolt binary, executed in place of the resolved pathname (§6.1); deleted after the child process exits
```

The **tracked** tree (`artifacts/resource-captures/`, §7.3, §12.3–§12.4)
separately holds its own transient temp files, each beside its own
final destination, never under `.tpatch/local/`:

```
artifacts/resource-captures/
  batches/<batch_id>.json                          -- committed batch (§7.3 step 3)
  batches/<batch_id>.tmp-<12 lowercase hex>.json    -- transient, present only mid-write of that batch (§7.3 step 3)
  current.json                                      -- committed pointer (§7.3 step 4)
  .tmp-current.json                                 -- transient, present only mid-write of the pointer (§7.3 step 4, one exact name, no suffix — the lock already serializes this file)
```

`.lock` (task 1) is created once, `0600`, the first time any mutator
runs for a given slug, and is **never removed** by any mutator — see
§7.2 for why deleting an advisory lock file is itself an unsafe
pattern this design deliberately avoids. `dolt-home/` is the only
local scratch content that can persist for the duration of a single
invocation beyond in-process memory (alongside the private Dolt-binary
copy, above, which persists only for the duration of one Dolt
invocation within that same window) — it holds whatever ephemeral
config Dolt itself chooses to write under an isolated `HOME`/
`DOLT_ROOT_PATH`, never a captured ignored-file byte or a copy of
Dolt's own query output. Every directory under `es_<id>/` is created
`0700` and every file `0600` **at creation** (`os.Mkdir`/`os.OpenFile`
with the final mode passed directly — never a separate `os.Chmod`
after the fact, task 8) — except the private Dolt-binary copy, which
is created `0600` (`O_CREATE|O_EXCL|O_WRONLY, 0600`, the same file
default as every other scratch file in this tree, rev-10, task 4:
corrects rev-8's direct-`0500`-at-creation design, which left the
descriptor's continued writability during the streamed copy resting
on POSIX's "permission bits are checked only at `open()` time, not on
every subsequent read/write" rule — technically sound but needlessly
surprising for a mode bit that reads as "not writable," and easy for a
future maintainer to misread as a race) and is hardened to `0500`
**via `Fchmod` on the still-open file descriptor** — never a
path-based `os.Chmod`, which would re-open (or re-resolve) the path
and could itself race a swap of the scratch entry between the copy
finishing and the `chmod` call — only **after** the streamed digest
has matched `trust.binary_sha256` (§6.1). `es_<id>/` is removed (`os.RemoveAll`,
best-effort) as the last step of the invocation on **both** the
success and failure paths; a removal failure is a local diagnostic
(§7.5), not a hard failure.

`add`/`remove`/`clear`/`trust-dolt` (task 1) acquire the same per-slug
`flock` (§7.2) before touching `resources.json`, and (task 1, this
subsection) run the leaf-targeted ignore/untracked gate before that
lock's own file is ever created, but never create `es_<id>/` and
never perform either orphan sweep below — only `capture`/
`record --resources` ever create scratch content (local or tracked),
so only they are responsible for cleaning it up.

`--dry-run` (§3, task 7) still acquires the lock and may still create
a real `es_<id>/dolt-home/` and a real private Dolt-binary copy if the
targeted resource set includes a Dolt capability (a real Dolt
invocation needs a real, isolated `HOME` and a real hash-verified
executable copy regardless of `--dry-run`) — but writes no tracked
batch/pointer, **runs neither the local nor the tracked orphan sweep**
(both sweeps below are exclusively `capture`/`record --resources`
behavior, and `--dry-run` never reaches §7.3's publication sequence
where the tracked sweep is invoked), and removes `es_<id>/` at the end
exactly as a real capture does; the persistent `.lock` file, once
created, is not removed for `--dry-run` either, exactly as it is not
for a real capture — "no tracked writes and no persistent local
writes" (§3) is about the tracked tree and the ephemeral scratch tree,
not the lock, which by design is never ephemeral.

**Orphan cleanup** (task 1, task 6): a startup sweep runs **only**
after the current invocation has itself acquired the live `flock`
(§7.2) — never before acquiring it, never from `add`/`remove`/`clear`/
`trust-dolt` (which acquire the lock but never sweep, since v1 has no
reason to sweep from a verb that never creates scratch content), and
**never** from `--dry-run` (which acquires the lock and may create
local scratch, but never reaches the publication step where the
tracked sweep runs, and only ever removes its own `es_<id>/`
directly, not via the general orphan-sweep code path). There are two,
separately-enumerated sweeps, both under the same acquired lock and
run only by `capture`/`record --resources`: (1) the **local** sweep
removes any leftover `es_*/` directory under
`.tpatch/local/resource-scratch/<slug>/` for this slug (a prior
`capture`/`record --resources` that crashed mid-invocation); (2) the
**tracked** sweep removes any leftover
`artifacts/resource-captures/batches/*.tmp-*.json`/
`artifacts/resource-captures/.tmp-current.json` file (§7.3) — this
sweep operates on the tracked tree, never on `.tpatch/local/`, per the
corrected diagram above. There is no lock-acquisition-time temporary
state at all (§7.2 — `flock` is acquired directly on the persistent
`.lock` file with no intermediate name), so there is no third "lock
temp/quarantine" sweep to run; sweeping under the lock still
guarantees neither sweep ever races a different, concurrently-running
mutator's own in-flight scratch content, since only one mutator can
hold the `flock` at a time (§7.2). Removal is best-effort
(`os.RemoveAll`/`os.Remove`), silent on success, logged as a local
diagnostic on failure — never a hard failure of the current
invocation.

### 7.2 Lock semantics (task 1)

A single lock per slug, `.tpatch/local/resource-scratch/<slug>/.lock`
(a **regular file**, not a directory — rev-3's directory-rename-based
lock is removed entirely), serializes **every** mutating verb for
that slug: `add`, `remove`, `clear`, `capture`, `trust-dolt`,
`record --resources` all acquire it before their first write (rev-9
task 3 fix: `trust-dolt` was previously omitted from this specific
enumeration, though §7.1/§10.3/D9's own lists already included it —
see §0's rev-9 fold summary, item 3); `list`/`diff` never acquire
it (§3, §7.6) and instead rely on `resources.json`/`current.json`/
`batches/<id>.json` always being read in a fully-written,
temp-then-atomic-rename state (§7.6), so a concurrent `list` never
observes a torn read regardless of whether it holds the lock.

**Why kernel `flock`, not a PID/temp-directory protocol** (task 1):
rev-1 through rev-3's owner-metadata lock (PID + process-start string,
quarantine, stale-reclaim, `.lock.tmp-*`/`.lock.stale-*` directories)
required this design to independently reinvent process-liveness
detection (`ps -o lstart=`, hostname comparison, PID-reuse guards) and
still left an unavoidable class of ABA-prone edge cases: any lock
protocol built from `os.Rename`/`os.RemoveAll` on a *named* filesystem
entry is vulnerable to a classic unlink/recreate race, where a
contender that read the lock's identity, then acted on a decision to
remove or replace it, can be fooled by a different process recreating
the same name in between. A **kernel-held advisory lock on an open
file descriptor** (POSIX `flock(2)`) has none of this: the lock is not
a piece of *data* a process reads and reasons about, it is a
kernel-tracked association between one open file description and one
inode, automatically and atomically released by the kernel itself the
moment every file descriptor referencing that open file description is
closed — including when a process holding it is `SIGKILL`ed, crashes,
or otherwise exits without running any cleanup code. There is no
owner metadata to read, no staleness to detect, no quarantine to
sweep, and no rename/unlink race, because the lock file's *name* is
never touched by the locking protocol itself (task 1's "no owner
JSON, PID/start, quarantine, stale reclaim, lock-temp dirs, or
ABA-prone `RemoveAll`").

**Acquire** (POSIX-supported hosts; never blocks — nonblocking only,
no polling, no configurable timeout in v1):

1. `os.OpenFile(".lock", O_CREATE|O_RDWR, 0600)` — creates the lock
   file the first time (after the local-ignore/untracked gate, §7.1,
   task 8), or opens the existing one on every subsequent invocation.
   The file has **no body at all** (rev-5 correction: rev-4's text
   allowed an optional, non-authoritative debugging comment to be
   written at creation, which the companion ADR never mirrored — the
   two documents must describe the identical byte content, and empty
   is the simpler, unambiguous choice): it is never written to, read
   from, or parsed by any code path — it exists purely to be an inode
   `flock` can attach to, and stays a zero-length file for its entire
   lifetime.
2. `syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)`. Success means
   this process now exclusively holds the advisory lock on this
   inode — proceed. `EWOULDBLOCK`/`EAGAIN` means a different live
   process already holds it — refuse immediately (`capture-in-progress`,
   exit 3), no wait, no retry.
3. The open file descriptor is held for the **entire remaining
   duration** of the invocation (through scratch creation, capture/
   record work, and tracked publication); closing it (explicitly at
   the end of the invocation, or implicitly by the kernel on process
   exit/crash) is what releases the `flock` — there is no separate
   "release" step that can itself fail or leave a stale artifact,
   because the kernel guarantees release happens exactly once, exactly
   when the last referencing descriptor closes.
4. The `.lock` file itself is **never removed, renamed, or
   `RemoveAll`'d** by any code path, ever — after its one-time
   creation it persists permanently as an ordinary, ignored, untracked,
   zero-behavioral-significance control file (§7.1). Deleting it would
   reintroduce exactly the unlink/recreate race this design exists to
   avoid: a second process could recreate the name and `flock` the
   *new* inode while a still-running first process holds the `flock`
   on the *old*, now-unlinked inode, and the two would never
   contend — silently defeating serialization. Never deleting the name
   is what keeps "the same inode" and "the name `.lock`" permanently
   synonymous.

**Release**: implicit only — the kernel releases the `flock` when this
process's file descriptor for `.lock` is closed, which happens (a)
explicitly, as the very last step of a successful invocation, (b)
explicitly, as the very last step of a failed invocation (same
code path, deferred), or (c) automatically, with no code running at
all, if the process is killed, crashes, or the machine loses power
mid-invocation — case (c) is exactly the crash-safety property a
PID/temp-directory protocol had to simulate with fallible userspace
bookkeeping, and here it is a kernel guarantee with zero lines of this
design's own code involved.

**Contention is immediate, not queued**: `LOCK_NB` means a contender
never waits — a second invocation for the same slug started while the
first is still running is refused instantly (`capture-in-progress`,
exit 3) rather than blocking. There is no v1 requirement to serialize
by waiting; callers that want serialized execution retry the whole
command themselves.

**Platform contract, exact build tags** (task 1, rev-5 correction):
`flock(2)` (via `syscall.Flock`) is implemented behind a Go build tag
restricted to **exactly** `//go:build linux || darwin` — not the
broader `unix` set rev-4 named, which Go's build-constraint vocabulary
also expands to include AIX, Solaris, and other Unix-family targets
where `syscall.Flock` either does not compile identically or is not
independently verified by this project. A second, complementary
build-tagged file, `//go:build !linux && !darwin`, provides a stub
`Acquire` that unconditionally returns a distinct, documented error
class, `resource-lock-unsupported` (**exit 3**), on every other
`GOOS`/`GOARCH` combination — including AIX, Solaris, Windows, and any
future target — without touching the filesystem. This is not a
speculative scope choice: the project's actual CI matrix
(`.github/workflows/ci.yml:18-25`) runs `test (${{ matrix.os }})` over
exactly `os: [ubuntu-latest, macos-latest]` — Linux and Darwin only, no
AIX/Solaris/Windows runner exists in the tested matrix today — so a
`linux || darwin`-only `flock` v1 is consistent with, and does not
regress, the hosts this project actually builds and tests on, matching
the existing `ADR-004-m10-copilot-proxy-ux.md` D6 precedent ("Windows:
not supported in M10"). Windows and every other non-`linux`/non-`darwin`
host are therefore **explicitly unsupported and deferred** for resource
capture in v1 — not a portable-lock design in disguise, and not
implicitly assumed safe merely because the code happens to compile on
a broader `unix` build tag; a future PRD would need to add both CI
coverage for any additional target and a real locking primitive for it
(e.g. `LockFileEx` for Windows) before lifting this restriction. This
is a hard, explicit "unsupported" contract, not a best-effort fallback.

**Filesystem contract — `flock` is a local-filesystem-only guarantee**
(task 2, rev-5; API and exact constants corrected in rev-6, task 7):
`flock(2)`'s advisory-lock semantics are only guaranteed by the kernel
for genuinely local filesystems; several POSIX-compliant systems
document `flock` as unsupported, only partially supported, or
supported solely through NFS lock-manager (`NLM`)/`nfs4` client
behavior that this design does not verify or depend on for
network/shared/clustered filesystems (NFS, network SMB mounts, some
FUSE-backed or overlay filesystems, and other non-local mounts) —
using `flock` there could silently fail to serialize concurrent
invocations from **different hosts**, which this design never claims
to support in the first place (v1's lock is a single-host,
same-machine-process serialization primitive only, never a
distributed/cross-client lock). To fail closed rather than silently
degrade, `Acquire` performs a `statfs`-family preflight on the
**nearest existing ancestor** of `.tpatch/local/resource-scratch/
<slug>/` (§7.1's first-create sequencing — the directory that will
contain `.lock` may not exist yet on a fresh clone, and `statfs`,
unlike the leaf-targeted ignore/untracked gate §7.1 now runs
separately, is a kernel call that genuinely requires an existing
inode, §7.1 task 1) **before** the `O_CREATE`/`flock` sequence above,
using **stdlib-only** `syscall.Statfs` (no `golang.org/x/sys/unix`
import, C32, consistent with this project's minimal-external-
dependency rule):

- **Linux** (`//go:build linux`): `syscall.Statfs(path, &buf)`, then
  **normalizing** `fsType := uint32(buf.Type)` (rev-7, task 7, C35 —
  `Statfs_t.Type`'s width and signedness is architecture-dependent:
  `int64` on `amd64`/`arm64`, `int32` on `386`/`arm`, `uint32` on
  `s390x`; a raw comparison against an untyped constant is
  architecture-fragile, so the comparison always happens against a
  single normalized `uint32` value regardless of build target) against
  the exact, byte-identical (PRD/ADR) numeric **allowlist**, itself
  typed `uint32`:

  | Filesystem | Magic (hex) |
  |---|---|
  | ext2/ext3/ext4 | `0xEF53` |
  | XFS | `0x58465342` |
  | Btrfs | `0x9123683E` |
  | tmpfs | `0x01021994` |
  | overlayfs (common under Docker/container CI, C33) | `0x794C7630` |

  and the exact numeric **denylist**, also typed `uint32`:

  | Filesystem | Magic (hex) |
  |---|---|
  | NFS | `0x6969` |
  | CIFS | `0xFF534D42` |
  | SMB2 | `0xFE534D42` |
  | FUSE | `0x65735546` |

  Any type not on the allowlist — including a denylisted type and any
  wholly unrecognized magic number — refuses identically (below).
  There is no Linux kernel constant for APFS at all; APFS reachable
  from Linux at all (e.g. via a FUSE driver) surfaces under the FUSE
  magic number, already denied. **Architecture coverage** (task 7):
  the normalization is exercised by a build-tag/unit-test seam that
  constructs a fixture `Statfs_t`-shaped value for each of
  `linux/amd64`, `linux/386`, `linux/arm`, `linux/s390x` and confirms
  the same allow/deny outcome regardless of the underlying field's
  native width/signedness — this design does not require actually
  cross-compiling and running on all four architectures in CI (§7.2's
  existing CI matrix is Ubuntu/macOS `amd64`, unchanged), only that
  the normalization logic itself is architecture-agnostic by
  construction and unit-tested against fixture values representative
  of each width/signedness class.
- **Darwin** (`//go:build darwin`): `syscall.Statfs(path, &buf)`,
  inspecting `buf.Fstypename` — accurately, an array of **signed
  8-bit integers**, `[16]int8` (not a `[]byte`; converting to a
  comparable Go string requires an explicit signed-byte-to-`byte`
  cast per element or `bytes.IndexByte`-style traversal accounting for
  the signedness, then trimming at the first NUL) — against the
  exact, byte-identical (PRD/ADR) string **allowlist** — `"apfs"`,
  `"hfs"`, `"tmpfs"` — and **denylist** — `"nfs"`, `"smbfs"`,
  `"webdav"`, `"osxfuse"`, `"macfuse"` (C34) as representative
  known-network/known-FUSE entries. Any other, unrecognized
  `Fstypename` value refuses identically to a denylisted one.
- Refusal outcome, either platform: `resource-lock-filesystem-unsupported`
  (**exit 3**), returned before `.lock` is ever opened or created —
  this is a distinct error class from `resource-lock-unsupported`
  (which is about the *build tag*/`GOOS`, not the *mount*) so a caller
  can distinguish "your OS isn't supported at all" from "your OS is
  supported, but this particular directory is on a filesystem this
  design refuses to trust `flock` on."
- **Why allow container-overlay but deny network filesystems** (task
  7): `overlayfs` (Linux) is the default storage driver for Docker/
  Podman/most container runtimes — refusing it would make this design
  unusable in the CI/container environments this project's own tested
  matrix actually runs in (`.github/workflows/ci.yml`'s `ubuntu-latest`
  runners frequently execute inside such a container layer), and
  `flock` on a mounted `overlayfs` behaves as a genuinely local
  advisory lock (the lower/upper filesystem layers are themselves
  local in every configuration this design targets). Network
  filesystems (NFS/CIFS/SMB2) are denied because `flock`'s
  cross-client exclusion guarantee on them is exactly the
  historically-inconsistent case C29 documents — this is not a
  performance or convenience distinction, it is "genuinely single-host
  local" vs. "may silently span multiple hosts."
- **If the exact allow/deny lists prove too brittle in practice** (an
  unrecognized-but-actually-safe local filesystem type appears on some
  future host): the documented fallback remains an explicit,
  **operator-configured local-only precondition** — "resource capture
  requires `.tpatch/local/` to live on a recognized local
  filesystem" — with this exact, detectable, actionable refusal, never
  a silent claim that `flock` works universally on every filesystem a
  future host might mount.

This is explicitly **not** a claim of cross-client/cross-host
serialization on any filesystem, networked or otherwise — v1's lock
exists solely to serialize same-host, same-machine concurrent
processes, and the filesystem-type check exists only to fail closed
rather than silently provide a false sense of safety on a mount where
even that narrower guarantee might not hold. If a project's actual
deployment target set turns out to make this allowlist too brittle in
practice (an unrecognized-but-actually-safe local filesystem type, for
instance), the fallback is an explicit, **documented operator
precondition** — "resource capture requires `.tpatch/local/` to live
on a recognized local filesystem" — with a detectable, actionable
refusal (this exact error), not a silent claim that `flock` works
universally on every filesystem a future host might mount.

**`list` and `diff`** (task 1, §7.6) never acquire `.lock` at all —
they are pure reads of whatever `resources.json`/`current.json`/
`batches/<id>.json` content is currently visible on disk, which,
because every tracked file is always written via temp-then-atomic-
rename (§7.3, §7.6), is always either the fully-prior or fully-new
content, never a torn read, regardless of a concurrent mutator holding
`.lock`.

### 7.3 The single publication point (task 5, task 6, task 9)

Per-invocation sequence, after lock acquisition (§7.2) and the
lock-gated orphan sweep (§7.1):

1. **Stage**: for every targeted resource (all declared, or the
   `--resource <id>` subset), perform the kind-specific capture work
   (§5, §6) entirely into bounded in-process memory (§8) and compute
   its `result`. If **any** targeted resource's staging fails (adapter
   error, size limit, redaction refusal, path/symlink refusal), the
   **whole** invocation aborts here: no batch file is written, `es_<id>/`
   (if created) is removed, the lock is released, and the command
   exits with the specific refusal's code (all-or-nothing).
2. **Compute the content-addressed `batch_id`** (task 9, task 16,
   rev-5: full digest): build the canonical batch body — `{"feature":
   <feature>, "results": <staged results, sorted by resource_id
   byte-ascending>}` — through `CanonicalBatchJSON` (§12's
   fixed-field, map-free canonical encoder), hash it (`SHA-256`), and
   take `batch_id = "rb_" + hex(...)` — the **full**, untruncated
   64-character lowercase-hex digest, not a truncated prefix (rev-4
   truncated to `[:12]`, a 48-bit space that is collision-prone for a
   content-addressing scheme meant to be a fatal-on-mismatch integrity
   guarantee, not merely a display-friendly short ID; resource IDs
   (§13, `res_` + 12 hex) are a **separate, unaffected** convention —
   only `batch_id` grows to the full digest). Unlike rev-2's random
   `batch_id`, an unchanged underlying capture (identical `feature` +
   identical staged `results`) always reproduces the identical
   `batch_id` — this is what makes a retry after a step-3 crash
   (below) idempotent without depending solely on the lock.
3. **Write the batch**: encode the **complete, intended on-disk file**
   for this batch — the full JSON object as it will actually be
   written to `batches/<batch_id>.json`, including the `batch_id`
   field itself and using the real on-disk formatting (2-space
   indented, trailing newline, §12) — call this the **file-wire
   bytes**, distinct from the canonical **hash-input bytes** from step
   2 (which omit `batch_id` and use the compact, map-free canonical
   encoding with no indentation). If `artifacts/resource-captures/batches/<batch_id>.json`
   already exists on disk, compare its bytes to these freshly-encoded
   **file-wire** bytes — **not** to the step-2 hash-input bytes, which
   can never equal the on-disk file (they differ in both the presence
   of `batch_id` and the formatting convention, so comparing against
   them was rev-3's bug: it would always report "different," turning
   every retry into a spurious `batch-id-collision` refusal).
   - **Identical file-wire bytes**: this exact batch was already fully
     written by a prior (possibly crashed-before-step-4) invocation.
     Skip straight to step 4 (idempotent re-publish) rather than
     rewriting an identical file.
   - **Different bytes — decode and compare the semantic body before
     deciding anything** (rev-6, task 6 — a byte-level difference no
     longer, by itself, means a collision): a byte-level file-wire
     difference can arise from two entirely different causes that this
     design must not conflate — genuinely different staged content
     (a real hash collision, integrity-fatal), or **presentation
     drift** on otherwise-identical content (e.g. this codebase's own
     JSON-encoding conventions changing between the version that wrote
     the on-disk file and the version running this invocation — field
     order, whitespace, or a cosmetic struct change that does not
     affect any field's actual value). Byte-inequality alone cannot
     distinguish these, so before refusing anything the adapter:
     1. **Decodes** the on-disk file as JSON (strict parse, same
        parser used everywhere else in this design — a file that does
        not even parse as valid JSON is a distinct, harder failure,
        `batch-file-corrupt`, exit 3, not routed through the
        comparison below at all).
     2. **Verifies** the decoded file's own `batch_id` field equals the
        target `batch_id` this step is about to write (a sanity check
        on the on-disk file's own internal consistency, independent of
        the filename it happens to be stored under) — a mismatch here
        is also `batch-file-corrupt` (exit 3), never silently trusted.
     3. **Canonicalizes** the decoded file's own `{feature, results}`
        body via the identical `CanonicalBatchJSON` encoder used in
        step 2 (dropping its `batch_id` field, exactly as step 2 does
        for the freshly-staged candidate) — call this the on-disk
        file's **semantic body**.
     4. **Compares** that semantic body, byte-for-byte, against this
        invocation's own step-2 canonical hash-input bytes for the
        freshly-staged candidate (the exact bytes that were hashed to
        produce this `batch_id` in the first place).
     - **Semantic bodies match** (presentation-only drift): this is an
       **idempotent** retry in substance, even though the raw on-disk
       bytes differ — the immutable file is **not** rewritten (an
       already-published `batches/<batch_id>.json` is never modified
       in place, drift or not, preserving the "immutable once written"
       invariant), and the invocation proceeds straight to step 4
       (pointer publish) exactly as the byte-identical case does.
     - **Semantic bodies genuinely differ**: this is the real,
       fatal case — two distinct staged contents produced the
       identical `SHA-256` `batch_id`, a cryptographic collision on
       the hash-input encoding — refused as `batch-id-collision`
       (exit 3), never silently overwritten. This remains expected to
       be unreachable in practice (a real full-`SHA-256` collision)
       and exists purely as a defensive, fail-closed guard, exactly as
       before — rev-6 only corrects *when* this label is applied
       (after ruling out presentation drift), not the underlying
       "collisions are fatal, never overwritten" policy itself.
   - **Absent**: write the file-wire bytes to
     `artifacts/resource-captures/batches/<batch_id>.tmp-<12 lowercase hex>.json`
     (`O_CREATE|O_EXCL`, `0644`, since it never contains raw bytes;
     the random 12-hex suffix — distinct per attempt — means two
     concurrent writers for the same never-yet-seen `batch_id` cannot
     collide on the temp name itself, even though the lock already
     serializes same-slug writers; this defends the temp name against
     a hypothetical future relaxation of that invariant), `fsync` the
     file, `os.Rename` it to
     `artifacts/resource-captures/batches/<batch_id>.json`
     (same-directory atomic rename), `fsync` the containing directory.
4. **Publish the pointer** (task 9 — "the sole commit point"): compute
   the new `current.json` content (§12.4) — every resource staged in
   step 1 now points at `batch_id`; every other, previously tracked
   resource not touched this invocation keeps its existing entry,
   carried forward unchanged from the prior `current.json`. Write to
   `artifacts/resource-captures/.tmp-current.json`, `fsync`,
   `os.Rename` to `artifacts/resource-captures/current.json`, `fsync`
   the directory. **This rename is the actual, single, atomic commit
   point of the whole capture** — before it succeeds, nothing has
   changed from any reader's perspective (a batch file already renamed
   into place in step 3 is a harmless orphan no `current.json` entry
   references yet); after it succeeds, the capture is fully and
   atomically visible. Readers (`list`/`diff`) use `current.json`
   only — they never scan `batches/` directly to infer state.
5. **Cleanup**: remove `es_<id>/` (if created), release the lock
   (§7.2).

**Batches are an unordered, content-addressed set — not a chronology**
(task 8, rev-5 correction): rev-4's "one batch per successful capture
invocation" phrasing could be misread as implying batches accumulate
as an ordered event log, or that "the newest batch" is a meaningful
concept. Neither is true. A `batch_id` names **exactly one, distinct
piece of content** — two invocations (whether the same run retried, or
two genuinely separate captures that happen to stage byte-identical
`results` for the same `feature`) that produce the identical canonical
body land on the identical `batch_id` and therefore the identical file
on disk (step 3's idempotent branch); there is no invocation counter,
sequence number, or timestamp anywhere in a batch file (§0.2) that
would let two occurrences of the same content be distinguished as
"first" and "second." `current.json` (§12.4) is the **sole**
authoritative statement of current state — it names, per resource, the
one `batch_id` that resource currently points at, and nothing else in
this design is chronologically ordered or historically authoritative.
A concrete consequence: if a resource's content is captured (batch
`A`), then changes and is captured again (batch `B`), then reverts to
exactly its original content and is captured a third time, the third
capture recomputes the identical canonical body as the first and
therefore the identical `batch_id` `A` — `current.json` is simply
rewritten to point at `A` again (step 4), and **no third batch file is
ever created**, because `A` already exists on disk with matching
file-wire bytes (step 3's idempotent branch). This is not a "rewind"
in the sense of restoring a prior historical state from a log — there
is no log — it is simply "the pointer now names the same content it
named before," and the fact that `B` was ever current is not
recoverable from this design at all once `current.json` no longer
names it (batch `B`'s file remains on disk as an orphan per this
section's crash-window table, but no tracked mechanism ever
lists/enumerates it as "history" — see §4.1/§16 which explicitly defer
event-chronology tracking to a future PRD). "One file per invocation"
(rev-4's phrasing) is corrected everywhere in this PRD/ADR to "one file
per distinct content" — an invocation that reproduces existing content
writes zero new bytes to `batches/`, it only (re)writes the pointer.

**Crash-window analysis** (task 6):

| Crash point | State left behind | Recovery |
|---|---|---|
| Before the tracked tree's first-ever `MkdirAll`/parent-`fsync` completes (rev-6, task 8 — a slug's first-ever `capture`/`record --resources`) | `artifacts/resource-captures/batches/` and/or `current.json`'s parent directory entries may not yet be crash-durable | Retried invocation re-runs `MkdirAll` (idempotent — existing directories are left untouched) and re-`fsync`s the parent chain before proceeding; no partial tracked file exists yet at this point, since step 3/4 cannot begin until the tree exists |
| Before step 3's rename | No new batch file (or an orphaned `batches/<batch_id>.tmp-<12hex>.json`) | Orphan `<batch_id>.tmp-*.json` swept at next invocation's start under its own acquired lock (§7.1/§7.2); a re-run recomputes the identical content and therefore the identical `batch_id` (step 2), and step 3's "identical file-wire bytes" branch makes the retry idempotent even if the orphan sweep somehow left a partial file behind (a partial temp is removed by the sweep before this check, so in practice the retry always re-lands on the "absent" branch) |
| After step 3's rename, before step 4's rename | A fully-written, permanently orphaned `batches/<batch_id>.json` that no `current.json` entry ever references | Harmless — never surfaced by `list`/`diff`/`capture` (§4.1's "missing batch" case does not apply here; this is an *extra*, unreferenced batch, not a missing one); left in place, not garbage-collected, in v1. A retry recomputes the same `batch_id`, finds it already present with identical file-wire bytes (step 3's idempotent branch), and proceeds straight to step 4 |
| During step 4's temp-write, before its rename | Orphaned `.tmp-current.json` (the one exact, single temp name used for the pointer, distinct from the batch temp naming above — there is only ever one in-flight pointer write per slug, since the lock serializes it) | Swept at next invocation's start; the last successfully-renamed `current.json` (from a previous, fully-committed invocation, or absent if this was the first-ever capture) remains authoritative and untouched |
| After step 4's rename | Fully committed | No recovery needed |

**Concurrency**: the lock (§7.2) already prevents two invocations for
the same slug from reaching step 3/4 simultaneously; nothing in §7.3
depends on filesystem-level atomicity across *different* slugs (each
slug's `artifacts/resource-captures/` tree is independent). `list`
(never lock-holding, §7.2) reading `current.json` mid-rename always
observes either the fully-prior or fully-new file, never a torn one,
because the rename is atomic at the filesystem level regardless of
whether the reader holds a lock.

### 7.4 Permissions (task 8)

Restated precisely because it spans both the ephemeral (§7.1) and
tracked (§7.3) trees, which have **different** permission
requirements — every creation call passes its final mode directly, no
call ever creates-then-`chmod`s:

- Ephemeral scratch (`es_<id>/`, `dolt-home/`): directories `0700`,
  files `0600`, always at creation.
- The persistent `.lock` file (§7.2): `0600` at its one-time creation,
  never `chmod`'d afterward, and — unlike the rest of the ephemeral
  tree — never removed; its permanence does not change its
  permission requirement.
- Tracked artifacts (`resources.json`, `batches/<id>.json`,
  `current.json`): ordinary repository file permissions (`0644`),
  since they never contain raw bytes or secrets by construction
  (§8.1) — there is nothing to protect with tighter permissions, and
  using non-standard permissions on a tracked, checked-in file would
  be surprising to anything else that reads the working tree.

### 7.5 Local diagnostics on failure (task 2)

When a `capture`/`record --resources` invocation fails at any stage
(§7.3 step 1), no tracked failure envelope is ever written (unchanged
principle from rev-1/rev-2) — failure detail is either printed
directly to the CLI's own stdout/stderr for that invocation (never
persisted to any file) or, if richer detail is useful for later
inspection, written to a file under the **same** `es_<id>/` ephemeral
tree that is deleted at the end of the invocation (§7.1). Because
rev-3 eliminated on-disk raw scratch content entirely (§8), any such
diagnostic can only ever contain redacted/summarized/bounded excerpts
already produced by the in-memory scan (§8.2) — never a fresh,
unredacted copy of the failing content — and it is deleted regardless
of outcome exactly as a successful capture's scratch content is.
There is no contradiction between "diagnostics are recorded" and
"batches are immutable and only ever created on success": diagnostics
live in the ephemeral tree (deleted regardless of outcome), batches
live in the tracked tree (written only on success, §7.3 step 3) — the
two never share a file, and no failed capture is ever promoted to a
tracked batch.

### 7.6 Read path (`list`/`diff`) during a concurrent mutation (task 1)

`list`/`diff` never acquire the lock (§7.2) and never block on one.
They read `resources.json` (declaration manifest) and, if present,
`artifacts/resource-captures/current.json` (pointer) and its
referenced `batches/<id>.json`, all of which are always in a
fully-written state on disk regardless of a concurrently-running
mutator, because every writer uses the same temp-then-atomic-rename
discipline (§7.3 for `current.json`/batch files; the declaration
manifest's own existing `add`/`remove`/`clear` write path, unchanged
from rev-1, already used this discipline and continues to). A
`list`/`diff` that races a mutator therefore always observes either
the state immediately before or immediately after that mutator's next
atomic rename — never a torn file — independent of whether `list`/
`diff` happens to run while the lock is held.

## 8. Privacy & Redaction (task 2, task 6)

### 8.1 Tracked content is always structural, never raw; scanning is always in-memory, never pre-persisted

No tracked file (`resources.json`, any `batches/<id>.json`,
`current.json`) ever contains: raw file bytes, raw adapter stdout, a
full Git object's content, or any string copied verbatim from a
scanned source (`diff_type`, §6.2, is the one narrow exception — a
short structural classification word, not "content" in the sense this
rule means). Tracked content is limited to hashes, byte/file counts,
the declared selector/args (themselves inputs, not scanned content),
structural true/false change flags, and `basename`+`binary_sha256`
tool identity.

Rev-3 goes further than rev-2's "raw bytes are ephemeral-scratch-only"
design (task 2's "zero pre-scan persistence"): raw bytes are **never
written to any file at all**, ephemeral or tracked. An `ignored-file`
selector's content is read directly into a bounded in-process
`[]byte`/`io.Reader` **via an actual cap-plus-one read** (task 2): the
reader (`io.LimitReader(f, limit+1)` followed by a length check, not a
pre-read `Stat().Size()` comparison) stops after at most `limit+1`
bytes have actually been read and refuses (`resource-limit-exceeded`,
exit 3) if that many were read, so a file that is grown by a
concurrent process between an initial `Stat` and the actual read
cannot silently bypass §5.1's per-file/total/file-count limits by
appearing small at stat-time and large at read-time; Dolt's
stdout/stderr are captured via caller-owned `os.Pipe()` pairs assigned
directly to `cmd.Stdout`/`cmd.Stderr` (rev-10: not
`cmd.StdoutPipe()`/`cmd.StderrPipe()` — §6.4)
into two in-memory buffers under the identical actual-read discipline,
sharing a single cap-plus-one budget (§6.4's 5 MiB combined cap,
enforced as a refusal via process-group kill on overflow, never a
`*bytes.Buffer` with no bound of its own) as the child process runs,
never redirected to a scratch file. Scanning (§8.2), hashing, and
classification (§5.1's `binary`/`text` first-8-KiB check) all operate
on these in-memory
buffers directly. Once a resource's `result` has been computed, the
buffer is discarded (garbage collected) — there is no file to delete,
because none was ever created. The only content that can legitimately
exist as a file on disk during an invocation is Dolt's own ephemeral
config/state under its isolated scratch `HOME` (§6.4, §7.1's
`dolt-home/`), which this PRD does not control the shape of and does
not scan (it is Dolt's own operational state, not captured
repository/database content).

### 8.2 The hard-refusal scanner

Unchanged from rev-2: `internal/cli/session_redaction.go` today is
unexported, shaped around `store.SessionObservation`, and applies
drop-the-line-and-continue semantics across 10 heuristic classes that
do not include dedicated PEM/OpenSSH-key, DB-connection-URL, or
email/PII patterns. This PRD requires the implementation cluster to
extract the reusable byte-pattern matchers into a new, exported,
content-agnostic `internal/redact.Scan(content []byte) []string`,
shared by both the existing session-redaction call site (unchanged
policy there) and the new resource-capture call site (always
hard-refuses on any match). `Scan` operates on an in-memory `[]byte`
(§8.1) — it is never handed a file path to open itself, keeping the
"never write raw bytes to open a scanner target" property structural
rather than a call-site discipline that could be violated later.

**Six closed classes** (unchanged from rev-1/rev-2), applied to every
candidate byte slice/string before it is written anywhere — tracked or
(now, per §8.1, never-created) ephemeral-scratch — a match on any
class is a hard refusal of the **entire** invocation
(`redaction-refused`, exit 3), never a partial scrub-and-continue, and
never a partial batch (§7.3 step 1's all-or-nothing rule):

1. PEM / OpenSSH private keys.
2. DB / connection URLs (known schemes with embedded userinfo, or the
   generalized `://user:pass@host` shape).
3. Emails / PII.
4. Credential assignments (`secret`/`token`/`password`/`api_key`/etc.
   `=`/`:` a quoted or bare value).
5. Bearer/token/key patterns (reusing existing `session_redaction.go`
   prefixes and a generic bearer-token pattern).
6. Home absolute paths (reusing the existing matcher).

### 8.3 What is scanned, and when

Every candidate string before it is written anywhere: the resource's
`selector`, every `args` value, every resolved `git-metadata` value, an
`ignored-file`'s in-memory content (byte-scanned regardless of
binary/text classification), and Dolt's in-memory captured
stdout/stderr (§6.4) before it is parsed into a tracked `result`
(§6.3). Scanning happens **before** any write of any kind — staging
computes the scan result first, entirely in memory; a refusal means no
batch is ever written (§7.3 step 1), not even for the other, unaffected
resources in the same invocation, and no partial or unredacted content
was ever placed on disk to begin with.

## 9. Path & Executable Safety (task 4, task 6)

### 9.1 `ignored-file`/`db_path` path gate: refuse any ancestor symlink, descriptor-identity check (task 4)

`safety.EnsureSafeRepoPath` and `store.NormalizeClaimPath` remain
lexical-only (`filepath.Abs` + string-prefix containment, no `Lstat`,
no `EvalSymlinks`) — sufficient for their existing callers, not
sufficient alone for resource capture. This lexical containment check
still runs first, as a coarse pre-filter before the ancestor-symlink
walk below: a declared `db_path`/`ignored-file` path that does not
lexically resolve inside the repository working tree at all (an
absolute path elsewhere, or a `filepath.Clean`d relative path that
walks above the repository root via `..`) is refused immediately,
**before** any `Lstat` of any component, named `path-outside-repo`
(exit 3, rev-10, task 3 — this design's own name for
`EnsureSafeRepoPath`'s pre-existing containment refusal, not a new
Dolt-specific error class, and distinct from the symlink-specific
names the ancestor walk below produces once lexical containment has
already passed). Rev-1's gate resolved the
**final** component if it was a symlink and re-validated containment
on the resolved target; the rev-1 adjudication found this missed
symlinks in **ancestor** directory components (e.g. a selector
`a/b/secret.txt` where `a` or `a/b` itself is a symlink escaping the
repo, never checked by a final-component-only rule).

Rev-2 replaced this with a strictly simpler fail-closed rule; rev-3
keeps that rule unchanged for the ancestor-walk itself and only
replaces the **post-open identity check** (step 5 below, task 4). The
gate runs at both `add` and every `capture`, for every path this
feature touches (an `ignored-file` selector, a `db_path` declared for
`adapter-snapshot`, every directory descendant for a directory
selector, and the process `cwd`):

1. Split the repo-relative path into components. Starting from the
   repository root, `Lstat` each successive prefix (root, root/a,
   root/a/b, ..., the full path).
2. If **any** component's `Lstat` result is a symlink — regardless of
   where it points, whether the target exists, or whether the target
   would itself be safely inside the repo — refuse immediately:
   `symlink-component-refused` (exit 3). This design does **not**
   attempt `EvalSymlinks`-and-re-validate for any component; a symlink
   anywhere in the chain is refused outright, full stop. This is
   simpler than rev-1's resolve-then-validate approach and closes the
   ancestor gap by construction — there is no "resolve the ancestor
   and check its target" step to get wrong, because ancestors are
   never resolved at all.
3. If any prefix component does not exist: `path-missing` (exit 3).
4. If every component is confirmed a plain (non-symlink) entry, open
   the final path with `O_NOFOLLOW` set (Unix build targets;
   `syscall.O_NOFOLLOW` — a real, available hardening layer for the
   **final** component specifically, C14) — a symlink that appeared at
   the final component between step 2's check and this open fails the
   open with `ELOOP`, refused the same as `symlink-component-refused`.
5. **Descriptor identity, not pathname re-`Lstat`** (task 4): rather
   than re-`Lstat`ing the pathname after opening (rev-2's design,
   which still races a symlink-swap-then-swap-back between the second
   `Lstat` and any subsequent use of the descriptor), the opened file
   handle itself is `fstat`'d (`f.Stat()`) and compared, via
   `os.SameFile(preOpenInfo, postOpenInfo)`, against the `FileInfo`
   captured for the final component during step 1's walk.
   `os.SameFile` compares the OS-level file identity (device+inode on
   Unix) of the two `FileInfo` values — this is a real property of
   **the thing that was actually opened**, not a second, independent
   pathname lookup that could itself race a further swap. Any mismatch
   is `path-replaced-during-open` (exit 3), and the just-opened content
   is discarded, never scanned or hashed. A pathname re-`Lstat` still
   runs immediately afterward as defense in depth (matching rev-2's
   original check), but `os.SameFile` against the open descriptor is
   now the primary, load-bearing identity guarantee.

**Refuse dangling/external/`.git`**: a missing prefix is `path-missing`
(step 3); a symlink anywhere is refused unconditionally (step 2) —
this subsumes rev-1's separate `symlink-escapes-repo` and
`symlink-targets-git-internal` outcomes, since **no** symlink is ever
followed or inspected for where it points; refusing all of them is
strictly more conservative than checking whether a specific one
happens to escape or target `.git`.

**TOCTOU residual, stated honestly** (task 4, C14): steps 1–5 close
the ancestor-symlink gap and the final-component race as far as Go's
standard library allows (`O_NOFOLLOW` binds only the *final* open
call; `os.SameFile` on the open descriptor closes the "was the
identity check itself racing a further pathname swap" gap rev-2 left
open, but there is still no stdlib/syscall wrapper — no `openat2`/
`RESOLVE_NO_SYMLINKS` — that also atomically binds every *ancestor*
directory component against a race between step 1's walk and step 4's
open). A sufficiently well-timed attacker who can replace an ancestor
**directory** itself (not just a leaf symlink) between steps 1 and 4
is not fully closed by this design using only the Go standard library.
This PRD does not claim otherwise; a stronger guarantee would require
platform-specific syscalls (`openat2` on Linux, no direct macOS
equivalent) outside this PRD's zero-external-dependency,
Unix-portable scope, and is explicitly left as a documented residual
risk rather than an impossible sandbox claim.

For a directory selector, this five-step gate runs **independently
per matched descendant file** (task 4's "every directory descendant"),
not just once for the top-level selector — a selector that was a
plain directory of plain files at `add` time but has since had one
entry replaced by a symlink is caught at the next `capture`, not
grandfathered in because the top-level directory itself still passes.
`db_path` (§5.3, §6.2) uses this exact same gate for its own
ancestor-walk and open-time checks — it is not a Dolt-specific path
policy; only the Dolt **executable** itself (§9.2) uses the
opposite-direction policy.

**`db_path`/`cmd.Dir` honesty** (task 3, rev-5 correction): unlike
every other path this gate protects, `db_path` is not opened and read
by this process — it is handed to Dolt as a **child process's working
directory** via Go's `os/exec.Cmd.Dir`, which is a plain pathname
**string**, not a file descriptor. There is no portable stdlib
mechanism to bind a spawned child's working directory to an
already-opened, already-validated directory descriptor (no
`fdopendir`-plus-`fexecve`-shaped API is exposed by `os/exec`) — so
between this gate's validation and the moment the child process
actually opens its cwd internally, the validated pathname could in
principle be swapped for something else by a sufficiently well-timed
local concurrent process. This design narrows, but does not eliminate,
that window. The check is **pathname-vs-descriptor**, not
descriptor-vs-descriptor — a stat on an already-held open file
descriptor always matches itself regardless of what has happened to
the *name* in the filesystem since it was opened, so re-`fstat`ing the
same descriptor twice and comparing the two results would be a
tautology that can never detect a swap; the real question is always
"does the pathname `db_path` still resolve to the exact directory we
have open," which requires a **fresh** pathname resolution each time:

1. Steps 1–5 above run once at `add` time, and again at the start of
   every `capture`/`diff` (as with any other gated path). The final
   step's directory open (`O_NOFOLLOW`) is kept open for the remainder
   of the invocation.
2. **Immediately before `cmd.Start()`**: the gate re-runs its full
   ancestor-walk-plus-open sequence a second time from scratch — a
   **fresh** `Lstat` of the `db_path` pathname component chain, not a
   reuse of step 1's cached `FileInfo` — and the resulting `FileInfo`
   is compared, via `os.SameFile`, against the `FileInfo` of the
   directory descriptor **already held open** from step 1. A match
   confirms the pathname still names the same directory this
   invocation has open; a mismatch is refused before the child is ever
   started.
3. The held directory descriptor from step 1 remains open for the
   entire lifetime of the Dolt child process — `cmd.Dir` itself still
   receives only the pathname string, not this descriptor, but holding
   it open at least guarantees the underlying inode cannot be fully
   deleted-and-reused while the descriptor is live, and gives step 4 a
   stable reference to compare a fresh resolution against.
4. **After the child process exits**: the gate resolves `db_path`'s
   pathname **fresh** a third time (a new `Lstat` chain, not the step-2
   result) and compares that `FileInfo`, again via `os.SameFile`,
   against the same held descriptor from step 1 — a mismatch is now a
   **hard refusal** (rev-6, task 2 — supersedes rev-4/rev-5's
   diagnostic-only detection): `db-path-identity-changed` (exit 3),
   and the invocation's output is discarded and **not** written to a
   batch, exactly as any other post-invocation identity failure in
   this design is handled (§6.1's Dolt-binary trust-pin recheck is the
   direct analog). Publishing a result gathered against a `db_path`
   that no longer resolves to the directory this invocation actually
   validated would mean the tracked result describes an unverified
   database, which this design's trust model does not allow merely
   because the failure is "only" a detection.

This provides **hard refusal upon detection of** a mid-invocation
`db_path` directory swap — rev-6 closes the gap where rev-4/rev-5
logged the mismatch as a local diagnostic (§7.5) but still published
the batch regardless; a caller now gets `db-path-identity-changed`
(exit 3) and no tracked artifact at all for that invocation. This is
still **detection**, not **prevention**, of a swap that completes
entirely *during* the child process's own execution window (between
step 2's pre-`cmd.Start()` check and the process's own internal
`cwd` resolution) — a sufficiently well-timed local concurrent
attacker who swaps `db_path`'s target and swaps it back before this
step 4 check runs would not be caught by any check in this design,
since nothing here holds a descriptor across the child's own internal
directory resolution (that resolution happens inside the Dolt
process, not this one). This narrower, honestly-scoped residual — a
swap that both occurs and is reverted entirely inside the child's own
execution window — is documented explicitly in Negative Consequences
(ADR-033 D6) as a local-concurrent-attacker residual this design does
not claim to close; it is not the same as rev-4/rev-5's "detected but
not refused" gap, which rev-6 closes completely. The general
ancestor-symlink/final-component TOCTOU residual described above for
ordinarily-opened paths applies to `db_path`'s own validation walk
identically; this subsection documents the **additional** residual
specific to `cmd.Dir` being pathname-bound rather than
descriptor-bound.

### 9.2 Executable path safety (task 2, distinct policy)

The Dolt executable's resolution uses the **separate** policy defined
in §6.1 (external-required, symlinks followed and their resolved
target validated, not refused) — the opposite direction from §9.1's
"stay inside the repo, refuse any symlink" rule, because an adapter
executable is a trusted external tool, not repo-owned content. The two
policies are never conflated: `ignored-file`/`db_path`/directory-descendant
paths always use §9.1; the Dolt executable path always uses §6.1/§9.2.

| Case | Outcome |
|---|---|
| `ignored-file`/`db_path`, every path component a plain file/dir, fully inside repo | Accepted |
| `ignored-file`/`db_path`, any ancestor component is a symlink (regardless of target) | Refused: `symlink-component-refused` |
| `ignored-file`/`db_path`, final component replaced by a symlink between the walk and the open | Refused: `symlink-component-refused` (via `O_NOFOLLOW`/`ELOOP`) |
| `ignored-file`/`db_path`, entry replaced (same name, different device/inode) between the walk and the open, detected via `os.SameFile` on the open descriptor | Refused: `path-replaced-during-open` |
| `ignored-file`/`db_path`, a prefix component does not exist | Refused: `path-missing` |
| Dolt executable resolves (possibly through symlinks) to a path outside the repo and `.git` | Accepted |
| Dolt executable resolves to a path inside the repo working tree or under any `.git` directory | Refused: `adapter-executable-in-repo` |
| Resource's `trust.binary_sha256` is `null`/absent at capture time | Refused: `dolt-trust-required` (exit 3, rev-7 task 5 — renamed distinct from the add-time `dolt-trust-flag-required` refusal, rev-8, task 3/5; this is a runtime/load-time gate on an already-declared resource, not a CLI-flag-validation gate) |
| Streamed private-copy `SHA-256` (computed while copying the opened source descriptor's bytes) does not match the resource's pinned `trust.binary_sha256` | Refused: `adapter-binary-untrusted`, private copy deleted, no invocation attempted (rev-7, task 3/4 — supersedes rev-6's "hash the resolved pathname, then invoke" design; the digest compared is now provably the digest of the bytes actually executed, §6.1) |
| Optional pre-`cmd.Start()` re-verification of the private copy fails | Refused: `adapter-binary-untrusted`, private copy deleted, no invocation attempted (rev-7, task 3) |
| Private-copy scratch directory's filesystem has an `ST_NOEXEC`/`MNT_NOEXEC` mount flag set (§6.1 step 3 of the capture-time sequence) | Refused: `adapter-copy-noexec` (exit 3, rev-8, task 6), before the private copy file is created |
| Private-copy stream-copy/`fsync`/`Close` fails with `ENOSPC`/`EIO` | Refused: `adapter-copy-failed` (exit 1, rev-8, task 6 — a host/environment fault, not a trust/policy refusal), partial copy cleaned up best-effort |
| `add --kind adapter-snapshot --adapter dolt` without `--trust-current-dolt` | Refused: `dolt-trust-flag-required` (add-time validation error, exit 2, renamed rev-8, task 3/5 — previously shared the name `dolt-trust-required` with the distinct capture-time exit-3 refusal above) |
| `add --kind adapter-snapshot --adapter dolt`, `--arg contract=<anything other than "dolt-diff-summary-v1">` | Refused: `dolt-contract-unsupported` (rev-7, task 5, exit 2) |
| `trust-dolt` targets a resource whose `kind`/`adapter` is not `adapter-snapshot`/`dolt` | Refused: `resource-not-dolt-adapter` (rev-7, task 4, exit 2) |
| `trust-dolt --binary-sha256` value is not exactly 64 lowercase hex characters | Refused: validation error (exit 2), before the lock is acquired (rev-7, task 4) |
| Duplicate `add` with an identical declaration (`--trust-current-dolt` re-passed or not) targeting an existing resource | Accepted as a strict idempotent no-op — `trust.binary_sha256` (if already set) is left byte-for-byte unchanged; only `trust-dolt` may re-pin after the initial `add` (rev-8, task 4) |
| An entry's own recorded `resource_id` does not match its own recomputed identity | Refused: `resources-file-corrupt` (rev-7, task 6 — distinct from `resource-id-collision`, which is reserved for two distinct declarations) |

## 10. Git Ignore/Tracked Gate Semantics (task 1, task 6)

### 10.1 `check-ignore` exit-code handling (task 1)

`git check-ignore -q --no-index -- <pathname>` (no `--literal-pathspecs`
— C17: `check-ignore` has no such option and fails fatally, exit
`128`, if one is passed, regardless of whether the argument itself
looks like pathspec magic — a plain glob-shaped argument such as
`docs/*.md` fails identically to a colon-magic-shaped one; this reuses
the **existing** `gitutil.IsPathIgnored` invocation verbatim): exit `0`
= ignored (gate passes); exit `1` = not ignored (gate fails,
`not-ignored`, exit 3); any other exit code (`128` and similar) is a
fatal Git error — refused (`git-ignore-check-error`, exit 3), never
silently treated as either "ignored" or "not ignored."

**Magic-name handling** (C18, empirically verified against installed
Git, including two independently-fatal magic keywords `:(glob)` and
`:(literal)`): because `check-ignore`'s pathname argument still parses
a leading `:` for pathspec magic (unlike `*`/`?`/`[]`, which are inert
to this command), any selector whose first byte is `:` is passed as
`./<selector>` — this disarms colon-magic parsing (the argument no
longer begins with a bare `:` byte) while resolving to the identical
on-disk path if it exists, or exit `1` (no match, not fatal) if it does
not. A selector not beginning with `:` is passed unchanged. This is the
one deliberate, documented exception to this project's general "always
pass literal pathspecs" discipline — it exists solely because
`check-ignore` has no `--literal-pathspecs` equivalent to rely on
instead, and the `./`-prefix trick achieves the same literal-path
guarantee for this one call site regardless of which specific magic
keyword (if any) the selector's leading colon would otherwise trigger.

### 10.2 `ls-files --error-unmatch` exit-code handling

`git --literal-pathspecs ls-files --error-unmatch -- <path>`: exit `0`
= tracked (gate fails when combined with "ignored" — §5.1 check 2);
exit `1` **with** the standard "did not match any file(s) known to
git" stderr shape = untracked (gate passes); any other exit code, or
exit `1` with unexpected stderr, is a fatal Git error — refused
(`git-ls-files-error`, exit 3), same fail-closed treatment as §10.1.
Every index-entry-selector call (`git-metadata`'s `index-entry` view,
§5.2) uses this same literal-pathspec form.

### 10.3 Local-ignore-root reuse (task 1, task 8; ignore/untracked target split, rev-8 task 7)

Before the first write to `.tpatch/local/resource-scratch/<slug>/` for
a given slug — which now means before the persistent `.lock` file's
own first creation (§7.1, §7.2), not merely before scratch-content
creation — **every** mutating verb (`add`, `remove`, `clear`,
`trust-dolt`, `capture`, `record --resources`, task 8's "every mutator
checks `EnsureLocalIgnoreContract`... before creating lock/scratch")
runs **two separate checks with two separate targets** (rev-8, task 7
— corrects rev-7's design, which reused the single per-slug leaf path
for both the ignore check *and* the untracked check, silently
disagreeing with §7.1's own worked fresh-clone example, which already
targeted the whole `.tpatch/local/` subtree for the untracked half):

1. **Ignored half — targets the exact per-slug leaf**, existence-
   independent, nested-negation-aware: the **existing**
   `workflow.EnsureLocalIgnoreContract(repoRoot, resourceScratchRoot)`
   (`internal/workflow/session_ignore.go:138`) — reused exactly as-is,
   not re-invented — verifies Git is available, the path
   (`.tpatch/local/resource-scratch/<slug>/`) is inside the worktree,
   and `gitutil.IsPathIgnored` reports it ignored; `IsPathIgnored`'s
   own `check-ignore` invocation is precisely the deliberate pathname
   exception documented in §10.1 (it does not, and cannot, use
   `--literal-pathspecs`). **`EnsureLocalIgnoreContract` itself only
   ever covers this ignored half** — it has no untracked-checking
   logic of its own (C13), which is exactly why step 2 below is a
   separate call, not a parameter or mode of this same function.
2. **Untracked half — targets the whole `.tpatch/local/` subtree, not
   the per-slug leaf** (rev-8, task 7): `git --literal-pathspecs
   ls-files -- .tpatch/local/` (no `--error-unmatch`) is run once per
   mutating invocation; the check passes only if **stdout is exactly
   empty** — any tracked path anywhere under `.tpatch/local/` (for
   *any* slug, not only the one being mutated) makes this list
   non-empty and refuses; a non-zero exit from the `git` invocation
   itself (a genuine Git error, distinct from "nothing tracked") is
   always fatal, never interpreted as "untracked." This deliberately
   does **not** reuse `AC-39`'s `ls-files --error-unmatch`-based
   per-selector gate (§10.2): `--error-unmatch`'s "tracked vs.
   untracked" distinction is inferred from an exit-code/stderr-text
   shape that is designed for a **single, existing-or-not** pathname
   argument, not for "is anything anywhere under this whole directory
   tracked" — the plain, no-flag `ls-files` form with an
   empty-stdout convention answers that broader question directly,
   without depending on stderr text matching at all, and is
   deliberately targeted at the whole `.tpatch/local/` subtree (not
   the per-slug leaf) precisely because the ignore half (step 1) is
   already the leaf-targeted, existence-independent check — splitting
   the two removes the need for the untracked half to also be
   existence-independent or leaf-scoped, since `ls-files` naturally
   answers "is anything tracked under this prefix" regardless of
   whether the prefix itself currently exists on disk.

Either check failing is `local-root-not-ignored` (step 1) or
`local-path-tracked` (step 2) (exit 3) — refused before any scratch
content, **including the lock file itself**, is created, exactly
mirroring ADR-027 D1's ignored-before-first-write mandate. This gate
now runs identically for `remove`/`clear`/`trust-dolt` (which
previously, in rev-3, only acquired the lock without first re-running
this check — corrected here since rev-4 makes the lock file itself
the very first piece of scratch state any mutator creates) — §14 has
explicit `AC`/matrix rows for `remove`/`clear`/`trust-dolt` exercising
this exact case, not just `add`/`capture`. This PRD does not invent a
second ignore mechanism — it reuses the one that exists (step 1) and
adds only the missing tracked-file half (step 2), scoped to the whole
gitignored root rather than any one slug's leaf, since a tracked file
anywhere under `.tpatch/local/` is a privacy-boundary violation
regardless of which slug's scratch tree it happens to sit under.

### 10.4 Pathspec-magic rows (task 1, task 6)


| Selector / call | Invocation | Behavior |
|---|---|---|
| `:(glob)config/*.env` (a literal filename that happens to start with pathspec-magic syntax), `check-ignore` | `git check-ignore -q --no-index -- './:(glob)config/*.env'` (C18's `./`-prefix rule applied) | Treated as the literal filename; no magic parsing, no fatal error |
| Same selector, unprefixed | `git check-ignore -q --no-index -- ':(glob)config/*.env'` | Fatal: unsupported magic keyword for this command (empirically confirmed, exit 128) — this PRD never emits this form (C18's rule always applies the prefix first) |
| `:(literal)config/name.env` (a second, independently-confirmed-fatal magic keyword), `check-ignore` | `git check-ignore -q --no-index -- './:(literal)config/name.env'` (C18's `./`-prefix rule applied) | Supervisor-independently reconfirmed: treated as the literal filename; no magic parsing, no fatal error, matching the `:(glob)` case above |
| Same selector, unprefixed | `git check-ignore -q --no-index -- ':(literal)config/name.env'` | Supervisor-independently reconfirmed: fatal, exit 128 — this PRD never emits this form |
| `:/topmagic.env`, unprefixed, `check-ignore` | `git check-ignore -q --no-index -- ':/topmagic.env'` | Empirically: silently accepted (exit 0/1 per actual ignore status, no error) — still routed through the `./`-prefix rule regardless, since this PRD's rule is "any leading `:`", not "any *unsupported* magic," to avoid depending on which magic keywords are or are not supported by a given Git version |
| `config/**/local.env`, `check-ignore` (no leading `:`) | `git check-ignore -q --no-index -- 'config/**/local.env'` | `*`/`?`/`[]` are inert to `check-ignore` (empirically confirmed) — treated as literal characters, no glob expansion |
| `docs/*.md` passed with `--literal-pathspecs check-ignore` (invalid invocation, never emitted) | `git --literal-pathspecs check-ignore -q --no-index -- 'docs/*.md'` | Supervisor-independently reconfirmed: fatal, exit 128, `pathspec magic not supported by this command: 'literal'` — identical failure mode to a colon-prefixed argument, confirming C17's fix applies unconditionally regardless of argument shape |
| Same selector, `ls-files --error-unmatch` | `git --literal-pathspecs ls-files --error-unmatch -- 'config/**/local.env'` | Treated as a literal path containing the literal characters `**`; no magic expansion (`--literal-pathspecs` supported here, unlike `check-ignore`) |
| `:(glob)config/*.env`, `ls-files --error-unmatch` | `git --literal-pathspecs ls-files --error-unmatch -- ':(glob)config/*.env'` | Treated as the literal filename `:(glob)config/*.env` (no `./`-prefix needed — `--literal-pathspecs` already disarms this) |

## 11. `record --resources` Semantics (task 11)

Unchanged high-level ordering from rev-1/rev-2/rev-3 — Git-side
capture and resource-domain publication remain **two separate atomic
domains**; "staging" (§7.3 step 1) is ephemeral in-memory only (never
writes a batch file, task 2), and "publishing" is the same §7.3 steps
2–4 a standalone `capture` would run, using the **content-addressed**
`batch_id` (§7.3 step 2) rather than a random one.

1. **Zero-resource preflight**: zero declared resources refuses
   immediately, before touching Git and before lock acquisition
   (`no-resources-declared`, exit 1), unchanged from rev-1/rev-2/rev-3.
2. **Stage** (ephemeral, in-memory metadata only, task 11): acquire
   the per-slug `flock` (§7.2), run the lock-gated orphan sweep
   (§7.1), then run §7.3 step 1 for every declared resource —
   ephemeral scratch (Dolt `HOME` only, §7.1), bounded in-memory
   ignored-file/Dolt reads, redaction — but stop before step 2 (no
   batch file written yet); the fully-computed candidate batch content
   (and its content-addressed `batch_id`, computed the same way as
   §7.3 step 2, since it depends only on `feature`+`results`) is held
   in memory pending step 4 below. The `flock` remains held across
   steps 2–4 (it is one invocation of `record --resources`, not two).
3. **Git-side capture**: `record`'s existing, unmodified capture-mode
   dispatch runs, completely unaffected by step 2's outcome.
4. **Publish, gated on Git success**:
   - Git failed: the record command's existing failure behavior
     propagates; the in-memory candidate batch from step 2 is simply
     discarded (never written anywhere — "ephemeral metadata only,"
     task 11) regardless of its own success/failure. The `flock` is
     released (by closing the descriptor).
   - Git succeeded and step 2 also succeeded: run §7.3 steps 3–4 now
     (write batch using the file-wire-byte idempotency comparison,
     publish pointer, cleanup/release lock) using the already-computed
     candidate content and its precomputed `batch_id` — no
     adapter/Git-metadata re-execution.
   - Git succeeded but step 2 failed, or Git succeeded and step 2
     succeeded but the publish step (§7.3 steps 3–4) itself fails: a
     **partial-domain** result, `resource-domain-incomplete` (exit 1):
     > canonical patch recorded successfully; resource capture did not
     > complete: `<reason>`. Retry with `tpatch feature resource
     > capture <slug>` — this re-stages and republishes and is safe to
     > re-run.

**Idempotency (corrected from rev-2)**: because `batch_id` is
now content-addressed (§7.3 step 2) rather than random, a retry that
recomputes **identical** underlying content (same declared resources,
same repository/Dolt state at retry time) reproduces the **identical**
`batch_id` and lands on §7.3 step 3's "identical file-wire bytes, skip
to pointer publish" branch — this is what makes the retry safe, not a
"fresh ID every time" property (rev-2's phrasing implied the opposite
and is corrected here). A retry that runs after the underlying state
has genuinely changed (e.g. a Dolt table's data changed between the
failed attempt and the retry) correctly produces a **different**
`batch_id`, because the content genuinely differs — this is expected
and correct, not a re-run bug. `remove`/`clear` never participate in
this retry path at all (§3, §4) — they mutate only `resources.json`
and are never a source of publish-step failure or recovery.

**Interactions** (unchanged from rev-1/rev-2/rev-3): an empty
Git-side capture accepted by existing logic counts as Git-side success
for gating publish; `--auto`/commit-range flags compose with
`--resources` without special-casing; `record --resources` has no
`--resource` subset flag of its own (it always targets every declared
resource — the standalone `feature resource capture <slug> --resource
<id>` is the only subset-targeting entry point, matching its promised
all-declared-resources scope exactly). `record --resources` has no
`--dry-run` of its own either (unchanged — only `feature resource
capture`/`diff` support `--dry-run`/resource-only preview, §3).

**Exit codes** (restated for rev-4/rev-6's refusal names; rev-8, task 5
consolidates the taxonomy so every named refusal appears in exactly
one row/table context):

| Code | `feature resource {add,list,remove,clear,capture,diff,trust-dolt}` | `record --resources` |
|---|---|---|
| `0` | Success (including `diff` reporting "no capture yet") | Success |
| `1` | Internal error; `tracked-batch-missing` (§4.1); `adapter-copy-failed` (rev-8, task 6 — `ENOSPC`/`EIO` during the capture-time private-copy stream/`fsync`/`Close`, a host/environment fault rather than a trust or policy refusal) | Same, plus `no-resources-declared` and `resource-domain-incomplete` |
| `2` | Validation: bad kind/adapter/capability/view, unknown/duplicate/missing `--arg` (including missing `db_path`/`table` — `binary_sha256` is **not** a declared `--arg`, rev-7: it lives in the separate `trust` field, set only via `--trust-current-dolt`/`trust-dolt`, §3/§6.1), `NUL`/control byte/backslash/`..` in a Dolt arg (named `dolt-argument-refused`, task 10 — covers `..`, control/backslash bytes, and case-insensitive `WORKING`/`STAGED`, all refused at the same argument-validation layer before Dolt is invoked), missing index entry at `add`, `table` mismatch between selector and declared field, `dolt-trust-flag-required` (`add --adapter dolt` without `--trust-current-dolt`, renamed rev-8, task 3/5 from the previously-overloaded `dolt-trust-required` name), `adapter-missing-at-add` (rev-10, task 3, renamed from rev-8's overloaded `adapter-missing` — `exec.LookPath` fails while computing the bootstrap pin at `add --trust-current-dolt`; distinct name from the same underlying failure's exit-3 meaning at `capture`, below), `dolt-contract-unsupported` (rev-7, task 5), `resource-not-dolt-adapter` (rev-7, task 4, `trust-dolt` on a non-Dolt resource) | n/a (unmodified) |
| `3` | State/policy refusal: `not-ignored`, `tracked-and-ignored`, `git-ignore-check-error`, `git-ls-files-error`, any `symlink-component-refused`/`path-missing`/`path-replaced-during-open`/`path-outside-repo` (rev-10, task 3 — the pre-existing lexical `EnsureSafeRepoPath`/`NormalizeClaimPath` containment check, §9.1, run as the first-pass filter before the ancestor-symlink walk; a `db_path` or `ignored-file` selector that lexically resolves outside the repository root, with no symlink involved at all, is refused this name rather than the symlink-specific one), any size/count limit, `resource-limit-exceeded`, `redaction-refused`, `adapter-missing` (rev-10: capture-time-only now that the add-time meaning has its own name, `adapter-missing-at-add`, above)/`adapter-executable-in-repo`/`adapter-binary-untrusted` (rev-6, §6.1), `dolt-trust-required` (capture-time missing pin, renamed-distinct rev-8, task 3/5), `adapter-copy-noexec` (rev-8, task 6 — scratch filesystem `ST_NOEXEC`/`MNT_NOEXEC`), `db-path-identity-changed` (rev-6, §9.1 — upgraded from a diagnostic-only detection), `dolt-query-error`, `dolt-json-parse-error`, `local-root-not-ignored`/`local-path-tracked`, `capture-in-progress`, `resource-lock-unsupported`, `resource-lock-filesystem-unsupported`, `batch-id-collision`, `batch-file-corrupt` (rev-6, §7.3), `resources-file-corrupt` (rev-7, task 6), `resource-id-collision` (rev-6, §4/§13), `index-entry-missing` | Same set applies to staging (§11 step 2); surfaces as `resource-domain-incomplete` (exit 1) if Git succeeded, or as record's own existing exit code (with the discarded-batch diagnostic) if Git failed |

Named `dolt-argument-refused` cases, exhaustively (task 10): a `from`/
`to`/`table` value containing a `NUL`/C0 control byte, a literal
backslash, the two-character substring `".."` (C24), or matching
`WORKING`/`STAGED` case-insensitively (C19, §6.2's refusal rationale)
— every one of these is refused at validation, before Dolt is ever
invoked, and all five cases share this single named reason so a
caller does not need to distinguish "which specific value shape was
rejected" from the exit code/reason alone.

## 12. Wire Schemas (task 10, task 12)

Three distinct JSON serializations:

- **Canonical `args` JSON** (§13.1) — hash input for `resource_id`
  only. Sorted keys, minimal escaping, custom encoder. Unchanged from
  rev-1/rev-2.
- **Canonical batch JSON** (`CanonicalBatchJSON`, new in rev-3, task
  16) — hash input for the content-addressed `batch_id` (§7.3 step 2)
  only. A distinct, general-purpose recursive canonical encoder
  supporting: strings (same minimal escaping as §13.1 — only `\`→`\\`
  and `"`→`\"`, no HTML-escaping), JSON booleans, JSON `null`,
  non-negative integers, arrays (each field's own predefined order —
  `results` sorted by `resource_id` byte-ascending, `args`/`tables`/
  `files` each sorted by their own documented key), and fixed-field
  objects (field order = the struct's declared field order below,
  identical to the file wire format's field order — **not** a `map`,
  so there is no map-iteration-order dependency to canonicalize away).
  Only `{"feature": <feature>, "results": <sorted results>}` is hashed
  — `batch_id` itself is never part of its own hash input (no
  self-reference/placeholder hack).
- **File wire format** (this section) — every tracked file. Ordinary
  Go `encoding/json` on a fixed-field struct (declared field order,
  2-space indent, trailing newline). Arrays are always `[]`, never
  `null`; inapplicable fields are present with an explicit
  `null`/zero value, never omitted. **No Go `map` type appears in any
  tracked wire schema** (task 16) — every place rev-1 used a bare
  `map[string]string`/`map[string]interface{}` (`args`, the `current`
  pointer's implied per-resource index) is instead a sorted
  `[]{key, value}` (for `args`) or `[]{resource_id, batch_id}` (for
  `current.json`'s index) array of a fixed struct, so tracked output
  never depends on `encoding/json`'s map-key-sort behavior at all. No
  tracked file contains a wall-clock timestamp field anywhere.

### 12.1 `resources.json` (declaration manifest)

```json
{
  "resources": [
    {
      "resource_id": "res_4b62313b6cce",
      "kind": "adapter-snapshot",
      "selector": "dolt:diff-summary:users",
      "adapter": "dolt",
      "capability": "diff-summary",
      "args": [
        { "key": "contract", "value": "dolt-diff-summary-v1" },
        { "key": "db_path", "value": "data/dolt-db" },
        { "key": "from", "value": "main" },
        { "key": "table", "value": "users" },
        { "key": "to", "value": "HEAD" }
      ],
      "trust": { "binary_sha256": "3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b" },
      "added_by_tool_version": "tpatch/0.13.0"
    },
    {
      "resource_id": "res_79f5ac5dca13",
      "kind": "ignored-file",
      "selector": "config/local-secrets.env.template",
      "adapter": "",
      "capability": "",
      "args": [],
      "trust": null,
      "added_by_tool_version": "tpatch/0.13.0"
    }
  ]
}
```

(`args` entries are sorted by `key`, byte-ascending — the same sort
order as the canonical-hash encoding, §13.1, though this array and
that hash input are still two independently-defined serializations
that happen to share a sort rule, not the same code path. `trust` is
new in rev-7 (task 4): `null` for every kind except
`adapter-snapshot`/`dolt`, where it holds the mutable trust pin
`{"binary_sha256": "<64hex>"}` — written by `add --trust-current-dolt`
or updated later by `trust-dolt` (§3), and **excluded** from
`resource_id`'s canonical hash input (§6.1, §13) — unlike every `args`
entry, which does participate.)

### 12.2 Tagged result variants (task 14)

Every kind/view produces a distinct, fixed-field `result` shape,
tracked inside a `batches/<batch_id>.json` entry (§12.3). This
subsection documents every one exhaustively (no kind/view is left
implicit); §12.3's worked example instantiates three of them together
in one real batch.

**`git-metadata` / `head`, attached** (§5.2):

```json
{ "symbolic_ref": "refs/heads/main", "oid": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", "detached": false }
```

**`git-metadata` / `head`, detached** (`symbolic_ref` is `null` **iff**
`detached` is `true`, never independently, §5.2):

```json
{ "symbolic_ref": null, "oid": "4b825dc642cb6eb9a060e54bf8d69288fbee4904", "detached": true }
```

**`git-metadata` / `ref`** (an explicitly selected ref, §5.2):

```json
{ "ref": "refs/heads/feature-x", "oid": "1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d" }
```

**`git-metadata` / `index-entry`** (§5.2):

```json
{ "path": "src/main.go", "mode": "100644", "oid": "6b2257eaa9d7fff64994c37758376f6383e63f7d", "stage": 0 }
```

**`git-metadata` / `config`** (a resolved value, and the "unset is
valid, not an error" case, §5.2):

```json
{ "key": "core.filemode", "value": "true" }
```
```json
{ "key": "extensions.objectformat", "value": null }
```

**`ignored-file`, single file** (§5.1):

```json
{ "file_kind": "text", "size_bytes": 214, "hash": "sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60" }
```

**`ignored-file`, directory** (task 14's "complete directory-file-array
variant"): a stable-sorted (`path`, byte-ascending) fixed-field array
of every matched file, plus the aggregate fields already defined in
§5.1 — `file_count`/`total_bytes`/`combined_hash` are unchanged;
`files[]` is new in rev-3 (rev-2 defined only the aggregate fields,
not a full per-file array):

```json
{
  "file_count": 2,
  "total_bytes": 731,
  "combined_hash": "sha256:1c2b3a4e5f607b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d",
  "files": [
    { "path": "config/local/a.txt", "raw_sha256": "sha256:2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b", "byte_count": 412, "mode": "100644" },
    { "path": "config/local/b.txt", "raw_sha256": "sha256:3a4e5f607b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b", "byte_count": 319, "mode": "100644" }
  ]
}
```

(`files[]` entries never carry a `null` field; `mode` is the same
octal-string convention as `index-entry`'s `mode`, sourced from a
plain `os.Stat` of the file, not the Git index — an ignored file has
no index entry by definition. `combined_hash` is `SHA-256` over each
matched file's canonical tuple, sorted by `path` (rev-6, task 9:
tuple-encoding rule stated precisely, identical to §5.1) — each of
`path` (repo-relative), `mode` (this same octal string), and the raw
hash (the file's own **unprefixed 64-lowercase-hex** `SHA-256` digest
— distinct from this array's own `raw_sha256` field above, which
carries the `"sha256:"`-prefixed wire form; the tuple's hash
component is never prefixed) is individually terminated by a single
`0x00` byte, and files' contributions concatenate directly with no
further separator — see §5.1 for the full rule and a worked golden
vector. `files[]` is additional per-file detail, not a replacement
for that aggregate.)

**`adapter-snapshot` / Dolt `diff-summary`** (§6.2, §6.3):

```json
{
  "tables": [
    { "from_table_name": "users", "to_table_name": "users", "diff_type": "modified", "data_change": true, "schema_change": false }
  ]
}
```

Zero rows (nonexistent table, C21, or `dolt sql -r json`'s `{}` zero-row
case, §6.3):

```json
{ "tables": [] }
```

### 12.3 `batches/<batch_id>.json`

```json
{
  "batch_id": "rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021",
  "feature": "model-picker",
  "results": [
    {
      "resource_id": "res_4b62313b6cce",
      "kind": "adapter-snapshot",
      "selector": "dolt:diff-summary:users",
      "adapter": "dolt",
      "capability": "diff-summary",
      "args": [
        { "key": "contract", "value": "dolt-diff-summary-v1" },
        { "key": "db_path", "value": "data/dolt-db" },
        { "key": "from", "value": "main" },
        { "key": "table", "value": "users" },
        { "key": "to", "value": "HEAD" }
      ],
      "tool_identity": {
        "basename": "dolt",
        "binary_sha256": "3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b"
      },
      "result": {
        "tables": [
          {
            "from_table_name": "users",
            "to_table_name": "users",
            "diff_type": "modified",
            "data_change": true,
            "schema_change": false
          }
        ]
      },
      "raw": {
        "hash": "sha256:9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b3f9c8e1a2d4c5b6a7e8f",
        "byte_count": 187
      }
    },
    {
      "resource_id": "res_79f5ac5dca13",
      "kind": "ignored-file",
      "selector": "config/local-secrets.env.template",
      "adapter": "",
      "capability": "",
      "args": [],
      "tool_identity": null,
      "result": {
        "file_kind": "text",
        "size_bytes": 214,
        "hash": "sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60"
      },
      "raw": {
        "hash": "sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60",
        "byte_count": 214
      }
    },
    {
      "resource_id": "res_acc91dc23a8b",
      "kind": "git-metadata",
      "selector": "head",
      "adapter": "",
      "capability": "",
      "args": [],
      "tool_identity": null,
      "result": {
        "symbolic_ref": "refs/heads/main",
        "oid": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
        "detached": false
      },
      "raw": null
    }
  ]
}
```

`batch_id` is **content-addressed** (§7.3 step 2), not
random: `rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021` is the actual, computed `SHA-256`-derived ID
for this exact `{feature, results}` body (`results` sorted by
`resource_id` byte-ascending, per `CanonicalBatchJSON`, §12 intro) —
re-running `CanonicalBatchJSON` over this exact JSON (**excluding**
the `batch_id` field itself and using the compact, unindented
canonical encoding — the **hash-input bytes**, §7.3 step 2/step 3)
reproduces this exact `batch_id`, and this revision's validation pass
independently confirmed this by reimplementing `CanonicalBatchJSON` in
a standalone script and recomputing the exact value shown above.
`results` is sorted by `resource_id`, byte-ascending — `res_4b62313b6cce`
< `res_79f5ac5dca13` < `res_acc91dc23a8b` (unchanged sort order across
rev-6→rev-7's `resource_id` recomputation — the `adapter-snapshot`
resource ID still sorts first; rev-7's `contract`-for-`binary_sha256`
substitution in `args` changed the digest's exact byte value but not
its position relative to the other two, `'4' < '7' < 'a'` either way;
§13.3's golden vectors and this ordering were recomputed together for
rev-7). The JSON rendering shown
above — 2-space indentation, `batch_id` present as an ordinary field,
trailing newline — is the **file-wire bytes**: the exact, complete
on-disk representation of `batches/<batch_id>.json`, and is what §7.3
step 3's idempotency check compares a re-encoded candidate against
(never the hash-input bytes, which omit `batch_id` and use a different,
compact encoding — conflating the two was rev-3's idempotency bug,
§7.3).

`raw` is `null` for `git-metadata` (no raw-byte concept applies) and
always a populated `{hash, byte_count}` object for `adapter-snapshot`/
`ignored-file` (no optional opt-in — the ephemeral, never-persisted
bytes are always hashed in memory before the buffer is discarded,
§8.1). `tool_identity` is `null` for kinds with no adapter/executable
concept (`git-metadata`, `ignored-file`) and populated (`basename`
+`binary_sha256` only, never an absolute path, §6.1) for
`adapter-snapshot` — `binary_sha256` here is the **pinned, trusted**
digest at the time of this capture (§6.1), identical to the
resource's `trust.binary_sha256` value (§12.1) — **not** an `args`
entry (rev-7, task 4: `binary_sha256` moved out of `args` into the
separate `trust` field, and `args` instead carries `contract`, above)
— never a freshly-recomputed value that could silently differ from
the pin. The example `oid` above is an ordinary,
valid-shaped 64-hex-character SHA-256 Git object ID (illustrative, not
the well-known empty-tree hash `4b825dc642cb6eb9a060e54bf8d69288fbee4904`,
which §12.2's detached-`HEAD` example uses instead, task 10).

### 12.4 `current.json` (the tracked pointer)

```json
{
  "current_batch_id": "rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021",
  "resources": [
    { "resource_id": "res_4b62313b6cce", "batch_id": "rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021" },
    { "resource_id": "res_79f5ac5dca13", "batch_id": "rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021" },
    { "resource_id": "res_acc91dc23a8b", "batch_id": "rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021" }
  ]
}
```

`resources` is sorted by `resource_id`, byte-ascending, for
determinism (never dependent on map-iteration order — this is a
`[]struct`, not a `map`). **Rev-6 rename** (task 5): the field
previously named `latest_batch_id` is renamed `current_batch_id`,
and its meaning is restated to remove every "newest"/"latest"
implication — batches are an unordered, content-addressed set (§7.3),
so "newest" is not a concept this design tracks at all.
`current_batch_id` is simply **this `current.json` file's own
provenance fact**: it names the `batch_id` that the invocation which
most recently rewrote *this file* (§7.3 step 4) staged, regardless of
how many or which resources that invocation actually touched — it is
not a claim that this `batch_id` is chronologically newer than any
other `batch_id` referenced by the per-resource `resources` array
below (an invocation that captures only one of several declared
resources rewrites `current.json` with `current_batch_id` set to
*that* invocation's `batch_id`, while every other resource's entry in
`resources` is carried forward unchanged from whatever earlier
invocation last touched it — §7.3 step 4 already describes this
carry-forward; this field is simply named to describe the same fact
honestly). The per-resource `resources` array — never
`current_batch_id` alone — is what `diff`/`list --json` actually
resolve against for any single resource's current state.
`current.json` is the **only** file a reader consults
(§7.3 step 4) — `list`/`diff` never scan `batches/` directly.


### 12.5 First capture, add/remove/change shapes

The **first-ever** capture for a resource produces a `batches/<id>.json`
entry with the exact same schema shape as any subsequent one (§14 has
the `AC` for this) — there is no distinct "initial" schema; a
nonexistent-table Dolt result (C21) and a zero-row Dolt result both use
`{"tables": []}` (§12.2), never a special-cased shape. `remove`/
`clear` (§3, §4) only ever rewrite `resources.json` and **never** touch
`current.json` or any `batches/<id>.json` file (corrected from rev-3,
which had `remove`/`clear` prune `current.json`'s `resources` array —
that design would have made `current.json` writes a **third** verb
class beyond `capture`/`record --resources`, contradicting §7.3's "the
sole commit point" framing; rev-4 restores that invariant by making
`current.json` writable only by the actual capture/publish path). A
resource removed from `resources.json` while `current.json` still
holds a stale entry for its `resource_id` is simply not surfaced by
`list` (which iterates `resources.json`'s declared entries, never
`current.json`'s index directly, §3) — the orphaned `current.json`
entry is harmless and is not cleaned up in v1, exactly like an
orphaned `batches/<id>.json` file (§7.3's crash-window analysis) — both
are permanent, immutable historical artifacts once written. `remove`/
`clear` acquire the per-slug `flock` (§7.2) before writing
`resources.json`, and never rewrite any `batches/<id>.json` file;
every batch that ever existed remains on disk, byte-for-byte, forever
(immutable historical audit trail).

### 12.6 `trust-dolt` update wire (rev-7, task 4/10)

`trust-dolt <slug> <resource-id> --binary-sha256 <64hex>` (§3, §6.1)
rewrites **only** the target resource's `trust` field in
`resources.json`, in place — no other field of that entry, no other
entry, and neither `current.json` nor any `batches/<id>.json` file are
touched. Before:

```json
{ "resource_id": "res_4b62313b6cce", "kind": "adapter-snapshot", "selector": "dolt:diff-summary:users", "adapter": "dolt", "capability": "diff-summary", "args": [ { "key": "contract", "value": "dolt-diff-summary-v1" }, { "key": "db_path", "value": "data/dolt-db" }, { "key": "from", "value": "main" }, { "key": "table", "value": "users" }, { "key": "to", "value": "HEAD" } ], "trust": { "binary_sha256": "3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b" }, "added_by_tool_version": "tpatch/0.13.0" }
```

After `trust-dolt model-picker res_4b62313b6cce --binary-sha256
634ba799ac8169f5a914d5c3f2b2d0c7a514867d8dcd470e0c5fc60b1b499b02`:

```json
{ "resource_id": "res_4b62313b6cce", "kind": "adapter-snapshot", "selector": "dolt:diff-summary:users", "adapter": "dolt", "capability": "diff-summary", "args": [ { "key": "contract", "value": "dolt-diff-summary-v1" }, { "key": "db_path", "value": "data/dolt-db" }, { "key": "from", "value": "main" }, { "key": "table", "value": "users" }, { "key": "to", "value": "HEAD" } ], "trust": { "binary_sha256": "634ba799ac8169f5a914d5c3f2b2d0c7a514867d8dcd470e0c5fc60b1b499b02" }, "added_by_tool_version": "tpatch/0.13.0" }
```

Every field except `trust.binary_sha256` is byte-identical —
`resource_id`, `args`, `added_by_tool_version` are all unchanged, and
neither `current.json` nor any `batches/<id>.json` file is rewritten
by this command (verified by `list`/`diff` reporting the identical
`resource_id` and capture history before and after). A subsequent
`capture` for this resource records the **new** pin's value in that
capture's `tool_identity.binary_sha256` (§12.3), while every
already-published batch's `tool_identity` remains whatever pin was
active at the time that batch was published (batches are immutable,
§7.3) — a `trust-dolt` re-pin never retroactively edits history.

## 13. Resource ID Canonicalization (task 6, unchanged algorithm)

### 13.1 Canonical `args` encoding

Unchanged from rev-1/rev-2 (§0.3 — no design in this fold touches the
hash-derivation function itself):

1. Keys sorted byte-ascending.
2. Encoded as `{"k1":"v1","k2":"v2",...}`, no whitespace.
3. Only `\` → `\\` and `"` → `\"` escaped (not `encoding/json.Marshal`,
   which also HTML-escapes `<`/`>`/`&` by default).
4. Empty map encodes as `{}`.
5. UTF-8 required; no NFC/NFD normalization (documented v1
   limitation).
6. Any `NUL`/C0 control byte anywhere in `feature`/`kind`/`selector`/
   `adapter`/`capability`/any `args` key or value is a validation
   error (exit 2) at `add` time.

### 13.2 Full ID derivation

```
canonical_args := CanonicalArgsJSON(args)          // §13.1
payload        := feature + "\x00" + kind + "\x00" + selector +
                   "\x00" + adapter + "\x00" + capability +
                   "\x00" + canonical_args
digest          := SHA-256(UTF-8 bytes of payload)
resource_id     := "res_" + lowercase-hex(digest)[:12]
```

### 13.3 Golden vectors (reproduced from §0.3, byte-identical to `ADR-033-resource-capture-boundary.md` D3)

| Vector | Inputs (`feature`, `kind`, `selector`, `adapter`, `capability`, `args`) | `resource_id` |
|---|---|---|
| 1 | `model-picker`, `git-metadata`, `head`, ``, ``, `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker`, `adapter-snapshot`, `dolt:diff-summary:users`, `dolt`, `diff-summary`, `{"contract":"dolt-diff-summary-v1","db_path":"data/dolt-db","table":"users","from":"main","to":"HEAD"}` (declared `contract, db_path, table, from, to` order — rev-7, task 4: `binary_sha256` is **removed** from `args` and moved to the separate, identity-excluded `trust` field, §4/§6.1/§12.1; `contract` takes its place and does participate in identity) | `res_4b62313b6cce` |
| 3 | Same as Vector 2, `args` declared `to, db_path, table, from, contract` order | `res_4b62313b6cce` (**identical** — order-independence, reconfirmed with `contract` replacing `binary_sha256`) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

Vectors 1 and 4 are byte-identical to rev-1/rev-2 (unaffected by any
rev-3 change). Vectors 2/3 were **recomputed** as of rev-3 (`db_path`
became a mandatory declared field), **recomputed again in rev-6**
(`binary_sha256` became a mandatory declared field, task 1's trust-pin
requirement), and **recomputed a third time in rev-7** (§0.3 —
`binary_sha256` is removed from `args` entirely and replaced by
`contract`, since rev-7 excludes the mutable trust pin from identity,
§4/§6.1/task 4); all three recomputations were performed independently
via a standalone Python script implementing §13.1/§13.2 verbatim as
part of each revision's validation pass. Rev-4/rev-5 changed nothing
about §13.1/§13.2 — all four vectors remained byte-identical to rev-3
through rev-5; rev-6's mandatory `binary_sha256` field changed Vectors
2/3 once, and rev-7's removal-and-replacement with `contract` changes
them again to a **new** value (`res_4b62313b6cce`, distinct from
rev-6's `res_00189e66780a`) — the `resource_id` derivation function
itself (§13.1/§13.2) has never changed across any revision; only the
declared `args` set for a Dolt resource has.

## 14. Acceptance Criteria (task 14)

Clause-level, `AC-<n>` tagged. Each `AC` is one testable clause;
`ADR-033-resource-capture-boundary.md`'s Test Matrix cites these tags
directly. Renumbered from rev-3: rev-3's PID/temp-directory/quarantine
lock clauses (old `AC-39`–`AC-45`) are **removed** entirely (that
lock protocol no longer exists, §7.2) and replaced with kernel-`flock`
clauses; several other clauses gain corrected wording (remove/clear
never touching `current.json`, `diff` reading content, the
file-wire-vs-hash-input idempotency comparison); several new clauses
are added (cap-plus-one reads, `db_path`/`cmd.Dir` residual, JSON
whitespace trimming, local-ignore gate extended to `remove`/`clear`,
sequential-read residual). Do not assume `AC-<n>` means the same thing
it did in rev-3.

**Dolt SQL redesign (task 4, task 6, task 8, task 12)**

- `AC-1`: The exact argv `<resolvedDolt> sql -r json -q "<SQL>"` is
  invoked with the exact 3-argument
  `dolt_diff_summary('<from>','<to>','<table>')` query shape and
  explicit `ORDER BY from_table_name, to_table_name` (§6.2) — no other
  Dolt subcommand, flag combination, or argument count is ever invoked
  for `diff-summary`.
- `AC-2`: A value for `from`/`to`/`table` containing a `NUL`/C0
  control byte, a literal backslash, or the literal substring `".."`
  is refused (exit 2) before any SQL is constructed.
- `AC-3`: A single quote in `from`/`to`/`table` is escaped by doubling
  and round-trips correctly through the invoked query.
- `AC-4`: A missing required field (`db_path`/`table`/`from`/`to`), an
  unknown key, or a duplicate `--arg` key is refused (exit 2).
- `AC-5`: A rename (differing `from_table_name`/`to_table_name` on one
  row) is tracked verbatim, not collapsed into a same-name
  added+removed pair.
- `AC-6`: The five-column result is parsed with strict field
  presence/type checking — a missing field, extra field, duplicate
  key, or a non-boolean `data_change`/`schema_change` value is a fatal
  `dolt-json-parse-error`, never silently coerced.
- `AC-7`: `dolt sql -r json` output — **after trimming leading/trailing
  whitespace from the captured buffer** (§6.3/C27) — of the literal
  2-byte string `{}` is parsed as zero rows (`result.tables: []`), and
  a `{"rows":[...]}` envelope with any other/extra top-level key is
  refused as a fatal parse error.
- `AC-8`: A primary-key-set change on the mandatory `table` between
  `from` and `to` surfaces as `dolt-query-error` (exit 3), not a
  silent omission.
- `AC-9`: A `table` that exists in neither `from` nor `to` yields
  `result.tables: []` (zero rows), not an error — distinct from
  `AC-8`'s outcome.
- `AC-10`: The exact literal strings `WORKING`/`STAGED`, in any
  case, are refused with `dolt-argument-refused` (exit 2) at the
  argument-validation layer, before Dolt is ever invoked — v1 accepts
  committed refs only.
- `AC-11`: A `from`/`to` value shaped as a dot-range (containing `..`)
  is refused at validation (`AC-2`) before ever reaching Dolt, so the
  dot-range vs. explicit-form ambiguity in `dolt_diff_summary`'s own
  argument parsing is never exercised.
- `AC-12`: The first-ever capture of a resource produces the identical
  `batches/<id>.json` entry schema shape as any later capture, and a
  zero-row Dolt result uses the same `{"tables": []}` shape as a
  nonexistent-table result (§12.5).
- `AC-13`: A captured Dolt stdout buffer carrying the real, cited
  trailing-whitespace shapes (`"...]}\n"` for nonempty rows, `"{}\n\n"`
  for zero rows, §6.3/C27) parses identically to a buffer without
  that whitespace — the parser is verified against both shapes, not
  just an idealized exact-bytes fixture.

**No version probe; executable identity (task 4)**

- `AC-14`: `dolt version` is never invoked anywhere in the capture
  pipeline.
- `AC-15`: The tracked `tool_identity` contains only `basename` and
  `binary_sha256` — never an absolute path, in any tracked file.
- `AC-16`: The Dolt invocation's environment contains only `HOME`/
  `DOLT_ROOT_PATH` (both pointing at a fresh, `0700` ephemeral scratch
  directory created before the child starts, with no inherited
  credential/variable from the invoking process's environment, no
  `PATH`).
- `AC-17`: A resolved Dolt executable located inside the repository
  working tree (or under any `.git` directory) is refused
  (`adapter-executable-in-repo`).
- `AC-18`: The `SHA-256` digest fed into the private-copy verification
  (§6.1 capture-time steps 4-5) is computed via a single streaming
  `io.TeeReader`-based pass over the opened source descriptor's bytes
  while those same bytes are simultaneously written to the private
  ephemeral copy — never a separate hash-then-copy two-pass sequence
  — and a mismatch against `trust.binary_sha256` refuses
  (`adapter-binary-untrusted`, exit 3), deletes the partial/complete
  private copy, and starts no invocation (rev-7, task 3 — supersedes
  the rev-6 "re-hash the resolved pathname after invocation" AC, which
  never proved the bytes hashed were the bytes ultimately executed).
- `AC-19`: The child process's `cmd.Path`/`argv[0]` is the private,
  hash-verified copy's own ephemeral path — never the originally
  `LookPath`/`EvalSymlinks`-resolved pathname (rev-7, task 3). A
  missing `trust.binary_sha256` (null/absent) at capture time refuses
  `dolt-trust-required` (exit 3, renamed-distinct from the add-time
  `dolt-trust-flag-required` refusal, rev-8, task 3/5) before the
  private copy is even created.
- `AC-20`: `add --kind adapter-snapshot --adapter dolt` without
  `--trust-current-dolt` is refused (`dolt-trust-flag-required`, exit
  2, renamed rev-8 task 3/5 from `dolt-trust-required`, which now
  names only the distinct capture-time exit-3 refusal above); with it,
  `add` runs **only** the add-time trust bootstrap (TOFU, §6.1, rev-8
  task 3) — resolve/open/stream-copy-while-hash, no existing-pin
  check, no execution — and the resource's separate `trust` field
  (§4/§12.1) — **not** its `args` — gains a `binary_sha256` entry
  equal to the `SHA-256` of the binary `add` itself resolved at that
  moment (rev-7, task 4 — supersedes rev-6's "`binary_sha256` is a
  mandatory `args` entry" AC, since the pin is now excluded from
  `resource_id`'s identity hash entirely). A duplicate `add` (task 4,
  rev-8) — same declaration, `--trust-current-dolt` re-passed or not —
  is a strict idempotent no-op: an already-set `trust.binary_sha256`
  is left byte-for-byte unchanged even if the freshly-resolved
  binary's hash now differs; only `trust-dolt` (`AC-92`) may re-pin
  after the initial `add`.

**Zero pre-scan persistence; privacy; bounded reads (task 2)**

- `AC-21`: An `ignored-file` selector's content is read into an
  in-process buffer and never written to any scratch or other file
  before scanning/hashing completes.
- `AC-22`: Dolt's stdout/stderr are captured into an in-process,
  bounded buffer and never redirected to or copied into a scratch
  file before parsing/scanning completes.
- `AC-23`: A value matching any of the six redaction classes refuses
  the entire invocation (`redaction-refused`), with no partial batch
  written for any resource in that invocation, even unaffected ones,
  and with no unredacted byte having been written to any file at any
  point before the refusal.
- `AC-24`: No tracked file anywhere contains a wall-clock timestamp
  field.
- `AC-25`: `feature resource diff` on an `ignored-file` resource
  **reads current file content** through the same bounded in-memory
  scanner `capture` uses (not a stat-only/metadata-only check) to
  recompute `size_bytes`/`hash`/`file_count`/`total_bytes`/
  `combined_hash`, and reports exactly which of those (or file-set
  membership) changed — never a textual line-level diff.
- `AC-26`: A file/stream that grows beyond the declared limit between
  an initial `Stat`/length check and the actual read is still refused
  (`resource-limit-exceeded`), because the enforcement is an actual
  cap-plus-one read (reads `limit+1` bytes and refuses if that many
  were read), never a stat-only/pre-read-length check that trusts the
  claimed size.
- `AC-27`: A directory `capture`/`diff` is documented and tested as a
  **sequential**, per-file read — not a single atomic point-in-time
  directory snapshot — and this residual is stated in §15/Negative
  Consequences, not claimed closed.

**Descriptor-identity path gate; `db_path`/`cmd.Dir` residual (task 3)**

- `AC-28`: A selector (`ignored-file` or Dolt `db_path`) whose ancestor
  directory (not the final component) is a symlink is refused
  (`symlink-component-refused`), regardless of where that symlink
  points.
- `AC-29`: A selector replaced by a symlink at the final component
  between the walk and the open is refused via `O_NOFOLLOW`/`ELOOP`.
- `AC-30`: A selector whose underlying file is replaced (different
  device/inode) between the walk and the open is refused
  (`path-replaced-during-open`), detected via `os.SameFile` on the
  **open file descriptor**'s `FileInfo`, not a second pathname
  `Lstat`.
- `AC-31`: A dangling ancestor (missing path component) is refused
  (`path-missing`).
- `AC-32`: This gate re-runs independently for every descendant file
  of a directory selector, both at `add` and at every `capture`.
- `AC-33`: The Dolt executable path uses the separate, opposite-
  direction policy (§6.1/§9.2) and is never subject to the ancestor-
  symlink-refusal rule that applies to `AC-28`–`AC-32`.
- `AC-34`: `db_path`'s gate is re-run a second time immediately before
  `cmd.Start()` — a **fresh** pathname `Lstat`/resolve, not a reuse of
  the initial gate's cached `FileInfo` — compared via `os.SameFile`
  against the directory descriptor already held open from the initial
  gate; a **third**, independently fresh pathname resolution runs
  after the Dolt child process exits and is again compared
  (`os.SameFile`) against the same held descriptor. Rev-6 (task 2)
  upgrades **both** the pre-`cmd.Start()` and the post-exit mismatch
  from a diagnostic-only detection to a **hard refusal**
  (`db-path-identity-changed`, exit 3): the result is discarded and no
  batch is written — verified by a test simulating a post-gate,
  pre-start or post-exit directory swap and asserting the invocation
  fails closed rather than merely logging. A second test confirms the
  check is genuinely pathname-vs-descriptor, not descriptor-vs-
  descriptor (a test that only re-`fstat`s the same, unchanged
  descriptor twice and asserts equality would pass trivially and
  detect nothing — `AC-34` requires the comparison input to be a
  freshly-obtained `FileInfo` from a new `Lstat` of the `db_path`
  string each time). A **documented residual** remains and is
  explicitly out of scope for this `AC`: a swap that occurs and fully
  reverts entirely within the child process's own execution window
  (between the pre-start check and the post-exit check, invisible to
  either fresh resolution) is not detectable by any pathname-based
  check and is recorded as a residual in ADR-033 D6's Negative
  Consequences, not claimed away.

**`check-ignore` fix, ignored/tracked Git gates (task 1)**

- `AC-35`: `check-ignore` is invoked without `--literal-pathspecs` (no
  such option exists for it) — verified by asserting the invocation
  never includes that flag.
- `AC-36`: `check-ignore` exit `1` (not ignored) and exit `>1` (fatal)
  produce distinct refusal reasons, neither treated as "ignored."
- `AC-37`: A selector whose first byte is `:` is passed to
  `check-ignore` with a `./` prefix, and resolves to the identical
  on-disk path as the unprefixed form would if `check-ignore` could
  accept it literally.
- `AC-38`: `*`/`?`/`[]` characters in a `check-ignore` pathname
  argument never trigger wildcard/glob matching.
- `AC-39`: `ls-files --error-unmatch` exit `0` (tracked) and any
  non-standard exit/stderr shape produce distinct refusal reasons, and
  every such call uses `--literal-pathspecs`.
- `AC-40`: A selector is refused unless it is **both** ignored (via
  `AC-36`) **and** untracked (via `AC-39`) — recheck at `add` and at
  every `capture`.

**Local ignore contract, tracked-root gate, all mutators (task 8)**

- `AC-41`: The scratch root's ignored status is verified via the
  existing `EnsureLocalIgnoreContract`, not a second, parallel ignore
  mechanism — targeting the exact per-slug leaf (§10.3 step 1, rev-8
  task 7).
- `AC-42`: The scratch root is also verified untracked via a plain
  `git --literal-pathspecs ls-files -- .tpatch/local/` (no
  `--error-unmatch`) requiring **empty stdout**, scoped to the
  **whole** `.tpatch/local/` subtree, not the per-slug leaf (§10.3
  step 2, rev-8 task 7 — corrects the prior `AC-39`-style
  `--error-unmatch`-based per-leaf design, which conflated the
  single-selector-scoped gate's mechanics with this broader
  whole-subtree question); either check failing refuses
  (`local-root-not-ignored`/`local-path-tracked`) before any scratch
  content — including the persistent `.lock` file's own first-ever
  creation — is created.
- `AC-43`: `remove`/`clear`/`trust-dolt` run the identical
  local-ignore/untracked gate as `add`/`capture`/`record --resources`,
  before their own `.lock` acquisition — not just the mutators that
  create scratch content (correcting rev-3, which did not explicitly
  extend this gate to `remove`/`clear`; rev-8, task 4, adds
  `trust-dolt` to this same list).

**Kernel `flock` lock semantics (task 1)**

- `AC-44`: `.lock` is opened `O_CREATE|O_RDWR, 0600` and
  `flock(LOCK_EX|LOCK_NB)`'d; success proceeds immediately, and
  `EWOULDBLOCK`/`EAGAIN` refuses immediately (`capture-in-progress`),
  with no polling, wait, or configurable timeout.
- `AC-45`: The `.lock` file is never removed, renamed, or replaced by
  any code path across repeated invocations — verified by asserting
  the file's device+inode identity is unchanged across multiple
  successive invocations for the same slug.
- `AC-46`: A process holding the `flock` that is killed (simulating a
  crash) releases the lock at the kernel level with no code of this
  design's own running — the next invocation acquires it successfully
  immediately, with no manual reclaim/quarantine step of any kind.
- `AC-47`: All **six** mutating verbs (`add`, `remove`, `clear`,
  `trust-dolt`, `capture`, `record --resources`) acquire the identical
  per-slug `flock` before their first write; `list`/`diff` never
  acquire it (rev-8, task 4 — corrects the prior five-verb count,
  which omitted `trust-dolt`).
- `AC-48`: On a build tagged `!linux && !darwin` (the exact fallback
  constraint, not a generic `!unix`), every mutating verb returns
  `resource-lock-unsupported` (exit 3) deterministically, never
  silently proceeding unlocked.
- `AC-49`: Two invocations racing to acquire `.lock` for the same slug
  resolve with exactly one succeeding and the other refusing
  immediately (`capture-in-progress`) — contention is instantaneous,
  never a queued/blocking wait.

**Permissions, scratch/orphan cleanup (task 6, task 8)**

- `AC-50`: Every ephemeral scratch directory (`es_<id>/`, `dolt-home/`)
  is created `0700` and every file `0600` at creation (never via a
  separate `chmod` after a looser-permission create); the persistent
  `.lock` file is created `0600` at its one-time creation and never
  `chmod`'d afterward.
- `AC-51`: An orphaned ephemeral scratch directory (`es_*`) or tracked
  temp file (`batches/*.tmp-*.json`/`.tmp-current.json`) left by a
  simulated crash is swept **only after** the sweeping invocation has
  itself acquired the live `flock`, never before, and the local/tracked
  sweeps are exercised as two independently verified enumerations.
- `AC-52`: `add`/`remove`/`clear`/`trust-dolt` never perform the
  orphan sweep (verified by asserting no `es_*`/tracked-temp removal
  occurs during any of the four invocations even when such orphans are
  present; rev-10, task 2 — `trust-dolt` was omitted from this
  clause's own enumeration through rev-9, though every gate/lock list
  elsewhere already covered it).

**Content-addressed single publication point (task 5, task 6)**

- `AC-53`: A successful multi-resource `capture` writes exactly one
  new `batches/<id>.json` file (unless an identical one already
  exists, `AC-55`) and rewrites `current.json` exactly once.
- `AC-54`: `batch_id` is deterministically derived from
  `CanonicalBatchJSON({"feature","results"})` (the hash-input encoding,
  which excludes `batch_id` itself) — recomputing it from the same
  batch content reproduces the identical `batch_id`.
- `AC-55`: A retry that reproduces identical batch content re-encodes
  the **complete file-wire bytes** (including `batch_id`, with real
  on-disk indentation) and finds them identical to the existing
  `batches/<batch_id>.json`, skipping directly to pointer publication
  without rewriting the batch file — verified by asserting the
  comparison is against file-wire bytes, not the hash-input bytes
  (which would never match and would incorrectly trigger
  `AC-56`, rev-3's bug).
- `AC-56`: When an existing `batches/<batch_id>.json`'s file-wire
  bytes differ from the freshly-staged candidate, the implementation
  decodes the on-disk file, verifies its own `batch_id` field, and
  canonicalizes its `{feature,results}` body via
  `CanonicalBatchJSON` (rev-6, task 6 — a byte-level difference is
  **not**, by itself, classified as anything yet); if that recomputed
  semantic body matches this invocation's own step-2 canonical bytes,
  it is treated as **presentation drift**: the immutable on-disk file
  is left untouched (never rewritten) and the invocation proceeds to
  pointer publication exactly as `AC-55`'s byte-identical case does.
- `AC-57`: Only when `AC-56`'s canonicalized on-disk semantic body
  genuinely **differs** from the freshly-staged candidate's is the
  outcome a true `batch-id-collision` (exit 3, refused, never
  overwritten) — rev-6 corrects the prior design, which reached this
  refusal directly off a raw byte-level difference without first
  ruling out presentation drift.
- `AC-58`: An on-disk `batches/<batch_id>.json` that fails to parse
  as valid JSON, or whose own decoded `batch_id` field does not equal
  the filename's `batch_id`, is refused as `batch-file-corrupt`
  (exit 3) — a distinct error from both `AC-56`'s presentation-drift
  path and `AC-57`'s collision path, never routed through either
  comparison.
- `AC-59`: A crash simulated between the batch rename and the
  `current.json` rename leaves a permanently orphaned, harmless batch
  file that no subsequent `list`/`diff` ever surfaces, and a re-run
  recomputes the identical `batch_id` and proceeds via `AC-55`.
- `AC-60`: A crash simulated during either temp-file write (before its
  rename) leaves only a temp artifact matching the exact naming in
  §7.1/§7.3, swept at the next invocation's start (`AC-51`), with no
  effect on the last successfully committed `current.json`.
- `AC-61`: `remove`/`clear` never write, rename, or delete
  `current.json` **or** any `batches/<id>.json` file at all — only
  `resources.json`, under the per-slug `flock` (`AC-47`) — correcting
  rev-3, in which `remove`/`clear` pruned `current.json`'s live index.
- `AC-62`: A resource removed from `resources.json` while `current.json`
  still references it leaves a harmless orphaned pointer entry that
  `list` never surfaces (since `list` iterates `resources.json`'s
  declared entries, never `current.json`'s index directly) — verified
  by asserting the orphaned entry is not garbage-collected and does
  not appear in `list`'s output.
- `AC-63`: `current.json` is the only file `list`/`diff` read to
  resolve a resource's latest result — neither ever scans `batches/`
  directly.

**Git metadata / tagged variants (task 10)**

- `AC-64`: `head`'s `symbolic_ref` is `null` if and only if `detached`
  is `true`.
- `AC-65`: The `config` view refuses any key outside the exact
  four-key allowlist.
- `AC-66`: An `index-entry` selector queried with a path containing
  pathspec-magic characters resolves to the literal path under
  `--literal-pathspecs`.
- `AC-67`: A directory `ignored-file` result includes a stable,
  `path`-sorted `files[]` array with `{path, raw_sha256, byte_count,
  mode}` per entry, in addition to the aggregate
  `file_count`/`total_bytes`/`combined_hash` fields.
- `AC-68`: Every kind/view's tagged `result` shape (§12.2) is exercised
  by at least one test: `head` attached, `head` detached, `ref`,
  `index-entry`, `config` (set and unset), `ignored-file` single file,
  `ignored-file` directory, `adapter-snapshot`.

**`--dry-run`, transaction / `record --resources` (task 7, task 11)**

- `AC-69`: `feature resource capture <slug> --dry-run` writes **zero
  tracked files** (`resources.json` is not touched beyond its
  pre-existing content; no `batches/`/`current.json` write occurs) and
  leaves **zero persistent resource data locally** — but its
  persistent `.lock` file, if newly created by this invocation, is not
  removed (that is expected, not a leak; only ephemeral scratch, e.g.
  `es_<id>/dolt-home/`, is removed) — the AC therefore asserts "no
  tracked writes and no persistent resource-data local writes," not
  "zero filesystem writes."
- `AC-70`: `record --resources` on a feature with zero declared
  resources refuses (`no-resources-declared`) before any Git
  invocation and before lock acquisition.
- `AC-71`: A resource-staging failure combined with Git-side success
  produces `resource-domain-incomplete` with the exact recovery-command
  message, while the Git-side canonical patch is confirmed present and
  correct.
- `AC-72`: A resource-staging failure combined with Git-side failure
  discards the staged (never-written) candidate batch and surfaces
  only record's existing Git-failure behavior.
- `AC-73`: A successful stage and successful Git-side capture publish
  the batch and pointer atomically, verified by asserting both
  `batches/<id>.json` and `current.json` reflect the same invocation
  together, never partially.
- `AC-74`: Re-running `feature resource capture <slug>` (or
  `record --resources`) after a publish-step failure, with the
  underlying captured content unchanged, reproduces the identical
  `batch_id` and completes via `AC-55`'s idempotent branch — a retry
  with genuinely changed underlying content correctly produces a
  different `batch_id`.

**Golden IDs, batch golden vector (task 12)**

- `AC-75`: Each of the four golden resource-ID vectors in §13.3 is
  independently recomputed by the implementation and matches exactly,
  including the two `db_path`-bearing vectors' order-independence.
- `AC-76`: The worked `batches/<batch_id>.json` example's `batch_id`
  (`rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021`, §12.3, recomputed in rev-6 for the corrected `resource_id` sort order and new `binary_sha256` field) is independently recomputed from its
  `CanonicalBatchJSON({"feature","results"})` **hash-input** body
  (excluding `batch_id` itself) and matches exactly.

**Rev-5 additions: build tags, filesystem contract, output-cap
refusal, `WORKING`/`STAGED` refusal, batch ordering, directory mode**

- `AC-77`: The lock implementation file carries exactly `//go:build
  linux || darwin`, and its fallback file carries exactly `//go:build
  !linux && !darwin` — verified by asserting the literal build-tag
  comment text in each file, not merely its behavior.
- `AC-78`: A `.tpatch/local/resource-scratch/<slug>/` root created on
  a filesystem type outside this design's allowlist (simulated via a
  fake/stubbed `statfs` result, since CI cannot mount a real network
  filesystem) refuses `resource-lock-filesystem-unsupported` (exit 3)
  before `.lock` is ever created — distinct from `AC-48`'s
  build-tag-based refusal, verified by asserting the two error
  strings never collide and the filesystem check runs strictly before
  the lock-file-creation code path.
- `AC-79`: The preflight uses stdlib `syscall.Statfs` exclusively —
  no `golang.org/x/sys/unix` import anywhere in the lock/filesystem
  package (rev-6, task 7, verified via a source/import-list assertion,
  not merely behavior).
- `AC-80`: Every filesystem type in §7.2's Linux allow/deny table
  (`ext`/`xfs`/`btrfs`/`tmpfs`/`overlayfs` allowed; `nfs`/`cifs`/
  `smb2`/`fuse` denied) and Darwin allow/deny table (`apfs`/`hfs`/
  `tmpfs` allowed; `nfs`/`smbfs`/`webdav`/`osxfuse`/`macfuse` denied)
  is individually exercised via a table-driven test stubbing the
  corresponding `buf.Type`/`buf.Fstypename` value and asserting the
  expected allow/deny outcome — table-driven **supporting rows** for
  this single `AC`, one per listed type (rev-6, task 7's "AC for every
  listed type" is satisfied via exhaustive fixture coverage under this
  one clause, not one distinct numbered `AC` per filesystem type; §14.1
  counts this as one `AC`-tagged clause with 17 supporting Test Matrix
  rows).
  An unrecognized type on either platform (a value on neither the
  allow nor the deny list) is refused identically to a denylisted
  value, also exercised here.
- `AC-81`: The first-ever creation of `.tpatch/local/resource-scratch/
  <slug>/`'s intermediate directories runs the ignore/untracked gate
  against the **intended leaf path itself** (`.tpatch/local/
  resource-scratch/<slug>/`, existence-independent — not merely its
  nearest existing ancestor), and the `statfs` preflight against the
  **nearest existing ancestor** (a genuinely existence-bound kernel
  call), then `MkdirAll`s the missing directories, then unconditionally
  `fsync`s every directory in the chain (both newly-created and
  already-visible on a retry) before the invocation proceeds — verified
  by simulating a crash immediately after `MkdirAll` but before the
  `fsync` sequence completes and asserting a retried invocation
  re-creates (idempotent `MkdirAll`) and re-`fsync`s the **entire**
  chain rather than assuming prior durability, and separately
  verified by a fixture where the leaf path is untracked-but-not-
  ignored (only the ancestor is ignored), confirming the leaf-targeted
  gate refuses where an ancestor-only gate would have wrongly passed
  (rev-7, task 1 — corrects rev-6's AC, which ran the ignore/untracked
  gate against the ancestor too).
- `AC-82`: The first-ever `capture`/`record --resources` for a slug
  (no prior `artifacts/resource-captures/` tree) simulates a crash
  between the tracked tree's `MkdirAll` and its directory-chain
  `fsync` sequence completing, and a retried invocation recovers
  cleanly — re-running the idempotent `MkdirAll`, unconditionally
  re-`fsync`ing the full chain, and proceeding to §7.3 steps 2-4
  exactly as if the tree had always existed (task 8's first-
  publication crash row).
- `AC-83`: A simulated Dolt child process that writes more than 5 MiB
  combined stdout+stderr is killed (process group `SIGTERM` then
  `SIGKILL`) the instant the shared cap-plus-one budget is exceeded,
  the invocation refuses with `resource-limit-exceeded` (exit 3), and
  no partial/truncated stdout is ever handed to the JSON parser —
  verified by asserting the parser is never invoked at all for an
  over-cap invocation (not invoked-then-discarded).
- `AC-84`: `dolt_diff_summary`'s stdout and stderr share **one**
  cap-plus-one budget (not two independent 5 MiB budgets) —
  verified by a test where stdout alone is under 5 MiB but
  stdout+stderr combined exceeds it, and the invocation still refuses.
- `AC-85`: `cmd.SysProcAttr{Setpgid: true}` is set before
  `cmd.Start()` on `linux`/`darwin`, and — regardless of which of the
  three trigger branches fires (leader-exit observed, timeout, or
  output cap) — the leader is never reaped (no `cmd.Wait()` call)
  until **after** the unconditional `SIGTERM`(-pgid)→grace→`SIGKILL`
  (-pgid) sequence has fully run: `cmd.Wait()` is called **exactly
  once**, strictly after that sequence completes, never before it
  (rev-10, task 1 — rewritten to describe the single non-reaping
  `waitid`/`WNOWAIT`-observer-based sequence applied identically to
  all three branches, replacing rev-7's "timeout/cap-triggered kill"-
  only framing and rev-9's now-superseded early-`cmd.Wait()` design) —
  verified with a test Dolt-adapter stub that spawns its own
  descendant child process configured to ignore `SIGTERM` and close
  its pipes without exiting, confirming (a) the leader is not reaped
  until after the `SIGKILL` call regardless of trigger, (b) the
  descendant is also terminated (not orphaned) by the time the full
  sequence completes, and (c) the parent `tpatch` process itself is
  provably unaffected (rev-7, task 2, rev-10 task 1 — supersedes
  rev-6's "escalation cancelled the moment `cmd.Wait()` observes the
  group has exited" AC, which only proved the direct child's exit, not
  the whole group's).
- `AC-86`: A directory `ignored-file` capture followed by a
  chmod-only change (identical file set and byte content, changed
  permission bits on one file) produces a different `combined_hash`
  on the next `capture`/`diff`, and `diff` reports the specific
  `files[]` entry's `mode` as the differing field, with `hash`/
  `byte_count` unchanged for that entry.
- `AC-87`: A resource captured with content `A`, then content `B`,
  then content `A` again (three `capture` invocations) results in
  exactly **two** distinct `batches/<id>.json` files on disk (one for
  `A`'s content, one for `B`'s), not three — the third invocation's
  `current.json` rewrite repoints at the already-existing batch for
  `A` without creating a new batch file, verified by asserting the
  directory listing of `batches/` is unchanged in file count after the
  third invocation.
- `AC-88`: Adding a resource whose recomputed `resource_id` matches an
  **existing, distinct** declaration's `resource_id` in
  `resources.json` with **byte-identical** canonical declaration bytes
  is idempotent (exit 0, no duplicate entry); adding a **distinct**
  declaration whose recomputed `resource_id` matches an existing
  entry with **different** canonical bytes is refused
  (`resource-id-collision`, exit 3) — this AC is scoped strictly to
  the **two-distinct-declarations** case (rev-7, task 6 — narrowed
  from rev-6, which also covered the single-entry self-mismatch case
  now split out as `AC-95`); verified via a test-only stub hash
  function seam since a genuine `SHA-256` collision cannot be produced
  for a real test.
- `AC-89`: The two-file golden directory vector in §5.1/ADR-033 D3
  (`config/a.txt` empty, `config/sub/b.sh` with known content) is
  independently recomputed by a reference implementation of the
  `path`+`0x00`+`mode`+`0x00`+`hash`+`0x00` tuple-encoding rule and
  matches the documented `combined_hash` exactly.

**Rev-7 bounded maintenance/trust fold (task 1–8, 11)**

- `AC-90`: A `capture` for an `adapter-snapshot`/`dolt` resource whose
  `trust.binary_sha256` is `null`/absent is refused
  (`dolt-trust-required`, exit 3) before any file descriptor for the
  Dolt binary is opened — distinct from `add`-time's exit-2 refusal of
  the same name for a missing `--trust-current-dolt` flag.
- `AC-91`: `add --kind adapter-snapshot --adapter dolt --arg
  contract=<value other than "dolt-diff-summary-v1">` is refused
  (`dolt-contract-unsupported`, exit 2) before `db_path`/`table`/
  `from`/`to` are validated; `contract=dolt-diff-summary-v1`
  participates in `resource_id`'s hash input identically to
  `db_path`/`table`/`from`/`to`.
- `AC-92`: `trust-dolt <slug> <resource-id> --binary-sha256 <64hex>`
  atomically rewrites only the target resource's `trust.binary_sha256`
  field under the same per-slug `flock` `add`/`remove`/`clear` use,
  leaving `resource_id`, every other declared field, `current.json`,
  and every `batches/<id>.json` file byte-for-byte unchanged —
  verified via before/after `list --json`/`diff` output comparison.
- `AC-93`: The private, per-invocation Dolt-binary copy is created
  `0600` under the ephemeral scratch tree, hardened to `0500` via a
  descriptor-based `Fchmod` on the still-open file (never a
  path-based `os.Chmod`) only after its streamed hash matches
  `trust.binary_sha256`, and is deleted (best-effort) after the child
  process exits on both the success and failure paths (rev-10, task
  4 — corrects rev-8's direct-`0500`-at-creation design; the copy is
  never executable at any point before its digest has already been
  verified) — verified by asserting the copy's mode is `0600`
  immediately after creation and before the digest comparison, `0500`
  immediately after a matching digest, and that the copy's path does
  not exist after the invocation completes, on both outcomes.
- `AC-94`: The Dolt child process's `cmd.Path` (and observed `argv[0]`
  via a test double) is the private copy's own ephemeral path, never
  the originally `LookPath`/`EvalSymlinks`-resolved pathname.
- `AC-95`: A `resources.json` entry whose own recorded `resource_id`
  does not match the `resource_id` freshly recomputed from that same
  entry's own fields is refused `resources-file-corrupt` (exit 3) at
  load time — distinct from `AC-88`'s `resource-id-collision`, which
  is reserved for two **independently self-consistent** declarations
  that happen to collide; verified via a hand-constructed fixture
  entry with a deliberately mismatched `resource_id`.
- `AC-96`: A leader process that exits successfully while a
  descendant it spawned keeps sharing the same PGID's stdout/stderr
  pipe write ends still has that descendant killed before the
  invocation returns, and the leader itself remains unreaped (no
  `cmd.Wait()` call) throughout the entire group-signal sequence
  (rev-10, task 1 rewrite — closes the PGID-reuse risk the rev-9
  adjudication found in the prior "call `cmd.Wait()` as soon as the
  leader's exit is detected" design, which reaped the leader, and
  thereby released its PID/PGID back to the kernel, **before** the
  `SIGTERM`/`SIGKILL` group signals had necessarily been sent): the
  build-tagged, non-reaping `waitid(P_PID, leaderPID, ...,
  WEXITED|WNOWAIT)` observer (§6.4) reports the leader's exit without
  reaping it, so the leader stays waitable (its PID/PGID is not yet
  released) through the unconditional `SIGTERM`→grace→`SIGKILL`
  sequence that follows — verified by instrumenting the test double
  so the non-reaping observer's channel (racing, via `select`,
  against the timeout timer and the cap-exceeded signal) is observed
  to fire first, `cmd.Wait()` is asserted **not yet called** at that
  point, the unconditional `SIGTERM`→grace→`SIGKILL` sequence against
  the negative PGID runs regardless, the descendant no longer exists
  after the sequence completes, `cmd.Wait()` is called for the first
  and only time strictly after the sequence completes, and the
  pipe-draining goroutines are only joined (and reach `EOF`) *after*
  the descendant has been killed.
- `AC-97`: Across all three trigger branches — the non-reaping
  observer reports the leader's exit, the 30-second timeout elapses,
  or the output cap is exceeded — the identical unconditional
  `SIGTERM`→grace→`SIGKILL`(-pgid) sequence runs, tolerating `ESRCH`
  at both signal steps, the leader is confirmed exited (via the
  observer, awaited if not already fired) before `cmd.Wait()` is ever
  called, and `cmd.Wait()` is observed exactly once per invocation,
  strictly after the group-signal sequence completes in every branch,
  never before it (rev-10, task 1 rewrite — removes rev-9's
  branch-dependent "already-available channel value vs. a `cmd.Wait()`
  that completes moments after `SIGKILL`" framing, since no branch now
  calls `cmd.Wait()` before signaling) — verified by instrumenting the
  test double across all three branches and asserting the group-signal
  call sequence and argument (`-pgid`) are byte-for-byte identical
  regardless of trigger, and that `cmd.Wait()`'s single call is always
  the last event observed in the sequence, in all three branches.
- `AC-98`: The leaf-targeted **ignore** half of the gate for
  `.tpatch/local/resource-scratch/<slug>/` (§10.3 step 1) runs and
  refuses correctly even when that exact leaf directory does not yet
  exist on disk (fresh clone), distinguished from the `statfs`
  preflight, which necessarily targets the nearest existing ancestor
  instead — verified by a fixture asserting the ignore gate's
  refusal/pass outcome is identical whether or not the leaf directory
  has been pre-created.
- `AC-99`: A retried invocation after a simulated first-attempt crash
  re-`fsync`s every directory in the relevant chain — both local
  scratch and tracked `artifacts/resource-captures/`/`batches/` —
  unconditionally, not only directories `MkdirAll` reports as newly
  created on the retry.
- `AC-100`: The Linux `statfs` allow/deny comparison normalizes
  `fsType := uint32(buf.Type)` before comparison, verified via a
  build-tag/unit-test seam constructing fixture `Statfs_t`-shaped
  values representative of each of `linux/amd64` (`int64`),
  `linux/386`/`linux/arm` (`int32`), and `linux/s390x` (`uint32`) and
  confirming identical allow/deny outcomes across all four.

**Untracked-subtree gate, add-time TOFU bootstrap, private-copy host failures (rev-8, tasks 3, 4, 6, 7)**

- `AC-101`: The **untracked** half of the local-root gate (§10.3 step
  2, rev-8 task 7) is verified via a plain `git --literal-pathspecs
  ls-files -- .tpatch/local/` (no `--error-unmatch`) scoped to the
  **whole** `.tpatch/local/` subtree — a fixture with a tracked file
  planted under a *different* slug's scratch tree than the one being
  mutated still refuses (`local-path-tracked`) — distinguishing this
  from `AC-98`'s leaf-targeted ignore half, and from `AC-39`'s
  `--error-unmatch`-based single-selector gate, neither of which this
  check reuses.
- `AC-102`: `add --kind adapter-snapshot --adapter dolt ... --arg
  contract=<value> --trust-current-dolt` succeeds and records
  `trust.binary_sha256` even when no prior pin exists for this
  resource (there is no "existing pin" precondition on the add-time
  path), opens and hashes the resolved Dolt binary directly with no
  private copy file ever created under `es_<id>/`, and never executes
  the resolved Dolt binary (rev-10, task 4 — the bootstrap's own
  descriptor-hash-only design, distinct from the capture-time private
  copy) — verified via a test double asserting zero process-start
  calls and zero `es_<id>/` scratch-file-creation calls occur during
  `add`, regardless of the `--trust-current-dolt` flag's presence.
- `AC-103`: A duplicate `add` of an identical declaration — whether or
  not `--trust-current-dolt` is re-passed, and regardless of whether
  the currently-resolved Dolt binary's hash now differs from the
  stored pin — is a **strict** no-op: `trust.binary_sha256` is
  byte-for-byte unchanged afterward, and no process-start or
  binary-copy call occurs — verified by asserting the pre/post
  `trust.binary_sha256` bytes are identical and no copy/exec test
  double calls are observed across the duplicate `add`.
- `AC-104`: A capture-time private-copy scratch filesystem reporting
  the platform's no-exec mount flag (`ST_NOEXEC` on Linux,
  `MNT_NOEXEC` on Darwin) is refused `adapter-copy-noexec` (exit 3)
  before any byte of the Dolt binary is copied — verified via a
  build-tag/unit-test seam constructing a fixture `Statfs_t`/
  `Statfs_t.Flags` value with the no-exec bit set.
- `AC-105`: A capture-time private-copy write failing with `ENOSPC` or
  `EIO` is refused `adapter-copy-failed` (exit 1), the partial copy
  file is removed (best-effort), and no Dolt process is started —
  verified via a test double injecting each of `ENOSPC`/`EIO` during
  the streamed copy-while-hash step.

**Non-reaping process-group termination, `waitid`/`WNOWAIT` observer (rev-10, task 1)**

- `AC-106`: The build-tagged (`linux`/`darwin`) non-reaping leader-exit
  observer — a raw `syscall.Syscall6(syscall.SYS_WAITID, P_PID,
  leaderPID, &buf, syscall.WEXITED|syscall.WNOWAIT, 0, 0)` call,
  stdlib-only, no `golang.org/x/sys` dependency — compiles for both
  `GOOS=linux` (`amd64`/`arm64`) and `GOOS=darwin` (`arm64`, the
  development host's native target), confirmed via a cross-compile/
  source-shape test that builds the same platform-tagged source
  layout (`//go:build linux` and `//go:build darwin` constant files,
  a shared `//go:build linux || darwin` syscall-invocation file) for
  all three targets with no build errors, and that the darwin build
  additionally runs successfully on the development host, empirically
  observing a spawned child's exit via the non-reaping call (a second,
  immediately-repeated call to the same observer also succeeds,
  proving the child was not reaped by the first call) followed by a
  single `cmd.Wait()` that still correctly reaps the child and reports
  its real exit code — this is a planning-time source-shape/build-tag
  feasibility test recorded here for the future implementation cluster
  to reproduce verbatim; it does not itself ship code.

### 14.1 Exact counts (task 14: no false "exactly once" claims)

This PRD defines **106** `AC`-tagged clauses (`AC-1` through
`AC-106`, each an individually testable statement, no range-notation
grouping; sequentially numbered with no gaps, verified by an automated
scan of every definition line matching `- \`AC-<n>\`:` in §14, in
document order).

This is a **net +1** change from rev-9's 105 (rev-9's own count was
`AC-1` through `AC-105`, sequentially numbered with no gaps): rev-10
adds one new clause and removes none —

- `AC-106` (cross-compile/source-shape feasibility of the build-tagged
  `linux`/`darwin` non-reaping `waitid`/`WNOWAIT` observer, task 1)

— `105 + 1 = 106`. Six existing clauses were rewritten in place
without changing the total count: `AC-52` (`trust-dolt` added to its
own orphan-sweep-never-runs enumeration, task 2), `AC-85` (previously
"timeout/cap-triggered kill sequence," now "the single non-reaping-
observer-based sequence applied identically to all three trigger
branches"), `AC-93` (previously direct-`0500`-at-creation, now
`0600`-create → stream/hash → verify digest → descriptor-based
`Fchmod`-to-`0500`, task 4), `AC-96` and `AC-97` (previously described
an early, leader-exit-triggered `cmd.Wait()` call; rewritten to
describe the non-reaping `waitid`/`WNOWAIT` observer keeping the
leader unreaped through the full unconditional group-signal sequence
in every branch, task 1 — the actual mechanism fix, not merely a
wording pass), and `AC-102` (rewritten to state explicitly that the
add-time TOFU bootstrap opens and hashes the resolved binary directly,
with no private copy file ever created, task 4). `AC-84` needed no
change: its "one shared cap-plus-one budget" claim is orthogonal to
the termination-mechanism fix and remains accurate as written.

This is a **net +5** change from rev-7's 100 (rev-7's own count was
`AC-1` through `AC-100`, sequentially numbered with no gaps — see the
historical rev-7 narrative below for that revision's own derivation):
rev-8 adds five new clauses and removes none —

- `AC-101` (untracked half of the local-root gate, whole-`.tpatch/local/`
  subtree via plain `ls-files`, distinguished from `AC-98`'s
  leaf-targeted ignore half and from `AC-39`'s per-selector gate)
- `AC-102` (add-time TOFU bootstrap requires no existing pin and never
  executes Dolt)
- `AC-103` (duplicate `add` is a strict no-op that never re-pins trust)
- `AC-104` (`adapter-copy-noexec` refusal on a no-exec scratch
  filesystem)
- `AC-105` (`adapter-copy-failed` on `ENOSPC`/`EIO` during the private
  copy)

— `100 + 5 = 105`. Three existing clauses were rewritten in place
without changing the total count: `AC-96` (previously "kill-path-only
unreaped-leader-through-grace sequence," now "the single unified
sequence applied to every invocation, success or kill-triggered
alike"), `AC-97` (previously "normal-success path's immediate
`Wait()`, no artificial grace delay" — no longer true under the
unified design, since every invocation now pays the fixed grace
latency; rewritten to the pipes-EOF-but-leader-still-alive
forced-termination trade-off test instead), and `AC-98` (previously
covering both the ignore and untracked halves of the local-root gate
together, now narrowed to the ignore half only, with the untracked
half split out as the new `AC-101`, since rev-8 task 7 gives the two
halves different targets — leaf vs. whole subtree — that can no
longer share one clause) — none of these in-place rewrites add or
remove a clause; the count above already reflects them as the same
clause numbers before and after. `AC-47`, `AC-41`–`AC-43`, and the
`add`/`trust-dolt` narrative around `AC-92`/`AC-102`/`AC-103` were also
reworded in place (six-verb lock-list count, `trust-dolt` added to the
gate/lock verb enumerations, duplicate-`add` behavior made explicit)
without adding or removing any clause.

**Rev-9's own count derivation**: rev-9 is a **net +0** change from
rev-8's 105 (`AC-1` through `AC-105`, sequentially numbered with no
gaps) — no clause was added or removed. Exactly two existing clauses
were rewritten in place a second time: `AC-96` (previously the
rev-8-vintage "single unified sequence... drain both pipes to EOF,
then unconditionally send `SIGTERM`" wording, now rewritten to test
that a leader exiting successfully while a descendant lingers on the
pipes is still detected and its descendant killed, via the decoupled
`cmd.Wait()`/`os.Pipe()` mechanism — §6.4/D5, task 2) and `AC-97`
(previously the "pipes-EOF-but-leader-still-alive forced-termination"
trade-off test, now rewritten to assert the group-signal sequence and
its `-pgid` argument are identical across all three trigger branches,
with only the leader's exit-status source differing). §7.2's separate
prose fix (adding `trust-dolt` to that section's own "every mutating
verb" enumeration, task 3) required no AC change at all: `AC-47`
already covered all six verbs including `trust-dolt` since rev-8 — the
rev-9 finding was a text-only inconsistency in a second, narrower
enumeration in §7.2 itself, not a gap in AC-47's own test coverage.
Row 146's fsync-wording fix (task 1) is likewise text-only in the
companion ADR's Test Matrix and does not touch any AC in this PRD.

**Historical — rev-7's own count derivation** (preserved for
provenance, not a current fact about this revision's total): rev-7 was
a **net +11** change from rev-6's 89 (rev-6's own count was `AC-1`
through `AC-89`, sequentially numbered with no gaps), adding
`AC-90`–`AC-100` (Dolt trust/private-copy/termination/gate/statfs
clauses, several since rewritten above) — `89 + 11 = 100`, the total
this revision's fold starts from.

The companion ADR's Test Matrix maps each of these 106 clauses
to at least one row; several clauses map to more than one row (e.g.
both a human-output and `--json`-output verification, or both a
success and a failure path for the same mechanism, or `AC-80`'s 17
filesystem-type supporting rows). The matrix therefore has **more**
rows than there are distinct clauses — this PRD does not claim any
clause is covered
"exactly once." (Exact row total is recorded in the companion ADR's
own Test Matrix section and cross-referenced from `docs/handoff/
CURRENT.md`'s rev-8 entry, since the ADR is this PRD's normative
source for the matrix itself.)

## 15. Open Questions / Negative Consequences

- **Ancestor-directory TOCTOU** (§9.1) is a documented residual risk,
  not fully closed by the Go standard library alone; a future PRD
  could revisit this if a portable `openat2`-equivalent becomes
  available in stdlib.
- **`db_path`/`cmd.Dir` pathname-bound residual** (§9.1, task 3, rev-6
  update): Go's `os/exec.Cmd.Dir` takes a pathname, not a descriptor.
  Rev-6 upgrades both the pre-`cmd.Start()` and the post-exit identity
  checks from a diagnostic-only detection to a **hard refusal**
  (`db-path-identity-changed`, exit 3, result discarded, no batch
  written) — this closes the "detected but not refused" gap left open
  by rev-4/rev-5. What remains an honestly-documented **residual**,
  not claimed as closed: a sufficiently well-timed local concurrent
  attacker who swaps the `db_path` directory **and reverts the swap**
  entirely within the child process's own execution window — after
  this design's pre-start check passes and before its post-exit check
  runs, but back to the original before that post-exit check actually
  observes it — remains undetectable by any pathname-based check, since
  Dolt's own process resolves its `cwd` via the pathname at whatever
  instant the kernel actually opens it, a moment this design cannot
  observe directly without also holding an fd-bound `cmd.Dir`
  equivalent (which Go's `os/exec` does not offer). This design
  narrows the window to the smallest interval the platform's API
  surface allows (fresh pathname re-checks immediately before
  `cmd.Start()` and immediately after the child exits, both compared
  via `os.SameFile` against a directory descriptor held open across
  the entire child lifetime) and refuses hard on any detected
  mismatch, but does not claim to close the in-window residual —
  stated honestly here and in ADR-033 D6's Negative Consequences.
- **No raw content diffing/versioning** (§2): a real value proposition
  for Dolt/ignored-file resources — seeing an actual textual diff, not
  just "the hash changed" — is deliberately out of scope, and would
  require a future ADR that explicitly supersedes
  `ADR-027-capture-context-privacy-boundary.md`'s committed/local
  split, which this PRD does not attempt.
- **`flock` platform scope** (§7.2, rev-5 correction): kernel-`flock`-
  based locking is implemented for exactly `//go:build linux ||
  darwin`-tagged builds, not a generic `unix` tag (rev-4's language
  was imprecise — a bare `unix` build constraint would also match
  other POSIX-family targets, such as AIX or Solaris, where this
  project has no test coverage and no `syscall.Flock` portability
  guarantee) — consistent with this project's actual, tested
  `ubuntu-latest`/`macos-latest` CI matrix
  (`.github/workflows/ci.yml:18-25`); the fallback build
  (`//go:build !linux && !darwin`) returns `resource-lock-unsupported`
  (exit 3) for every mutating verb rather than a best-effort
  approximation — a future PRD could add a Windows-specific locking
  primitive (`LockFileEx`) if that platform becomes a validation
  target.
- **`flock` is a local-filesystem-only guarantee, not a claim about
  every mount** (§7.2, task 2): even on a `linux`/`darwin` build,
  `flock`'s advisory-lock semantics are not honored identically by
  every filesystem a `.tpatch/` directory could be created on — most
  notably network/shared filesystems (NFS, SMB/CIFS) may silently
  fail to provide cross-client mutual exclusion. This design adds a
  `statfs`-based preflight (§7.2) that refuses
  `resource-lock-filesystem-unsupported` (exit 3) for a known-bad or
  unrecognized filesystem type before the lock file is ever created;
  this is explicitly **not** a claim that every allowed filesystem
  type provides cross-host/cross-client serialization in every
  configuration — only that this design fails closed rather than
  silently trusting an unverified mount, and the exact allow/deny list
  is this PRD's own choice (§7.2), not a claim sourced from a formal
  POSIX guarantee, with an explicit escape hatch to an
  operator-configured-precondition framing if the allowlist proves too
  brittle in practice.
- **Per-invocation lock granularity is per-slug, not global**: two
  different features' `capture` invocations never contend, which is
  intentional (§7.3's concurrency note) but means a single feature
  with many resources cannot parallelize its own staging across
  multiple processes in v1.
- **Mandatory `table` forecloses whole-database diffing** (§6.2): a
  resource declaration that wants "everything that changed in this
  Dolt database" must enumerate every table it cares about; this is a
  deliberate v1 trade-off (favoring the hard PK-change error over
  silent omission) that a future PRD could revisit with an
  explicit multi-table/whole-db capability if the omission risk is
  judged acceptable with clearer documentation.
- **Sequential-read directory-scan residual** (§5.1, §8.1, task 9): a
  directory `capture`/`diff` reads each matched file one at a time, not
  under a single atomic filesystem-level snapshot; a `combined_hash`
  can in principle reflect a state that never existed as a single
  point-in-time directory content if an external process mutates a
  later-scanned file while an earlier one has already been read. This
  is stated honestly rather than claimed away.
- **No event-chronology/history log** (§7.3, task 5, rev-5/rev-6): the
  content-addressed `batches/` tree is an **unordered set of distinct
  content**, not an append-only log — `current.json`'s
  `current_batch_id` (rev-6 rename) is solely this file's own
  provenance fact (which batch the last rewriting invocation staged),
  never a claim about recency across the batch set as a whole. An
  `A`→`B`→`A` content sequence reuses `A`'s existing batch file and
  simply repoints the pointer back to it (`AC-87`) — this design
  deliberately does not provide "what changed since three captures
  ago" or any other chronological query in v1; a future PRD could add
  an explicit, separate event log if that need arises, but this
  design's batch set must not be mistaken for one.
- **Trust-pin scope is operator-approved binary identity, not
  provenance or proof of matching pinned source** (§6.1, task 4/5,
  rev-7 update): `--trust-current-dolt` pins the `SHA-256` of
  whatever binary `add` resolves via `PATH` at that moment, storing
  it in the resource's separate `trust` field — it verifies the
  *same bytes run every time*, not that those bytes are a
  legitimate/unmodified upstream Dolt release, nor that the pinned
  binary is free of vulnerabilities, nor that it matches any specific
  pinned upstream source commit. A locally-built or already-
  compromised Dolt binary pinned at `add` time remains trusted by
  this design for its entire lifetime unless the operator re-pins via
  `trust-dolt` (§3). This is a deliberate, narrower guarantee ("this
  exact binary, unchanged, every time, executed as a private
  verified copy") than a supply-chain attestation, and is documented
  as such rather than oversold. Rev-7's private-copy-execution binding
  (§6.1) closes the specific TOCTOU where the hashed bytes and the
  executed bytes could diverge; it does not and cannot address
  provenance beyond "this exact byte sequence was explicitly
  approved" — the strict five-field JSON parser (§6.3) is a separate,
  independent runtime capability gate that catches structural drift
  but not same-shaped semantic drift in a pinned binary that has
  silently diverged from its originally-approved behavior.
- **Executed-binary private-copy residual, stated honestly** (§6.1,
  task 3, rev-7): rev-7's private-copy-execution binding closes the
  specific "resolve, hash the pathname, then separately re-resolve
  and execute a possibly-swapped pathname" TOCTOU that rev-6 left
  open — the exact bytes hashed and the exact bytes executed are now
  provably the same file, since they trace back to one continuous,
  never-re-resolved open. It does not close every conceivable local
  race: a concurrent process running as the *same* OS user, with
  write access to the per-invocation ephemeral scratch directory,
  could in principle still interfere with the private copy between
  its hardening (`chmod 0500`) and `cmd.Start()`'s open of it. This
  mirrors the general local-concurrent-attacker model already
  accepted elsewhere in this design (the `db_path` residual directly
  below, §6.4's process-group residual) and is not claimed to be
  fully closed either.

*(Resolved in earlier revisions, remaining resolved: `diff_type`'s
value set is confirmed as a closed 4-value enum via its four exact
assignment lines (§6.2, citing `table_deltas.go:722/733/745/760`,
C26). `WORKING`/`STAGED` support is a confirmed **source** fact
(§6.2, citing `doltdb.go:51-52`, C19) but rev-5 explicitly **refuses**
both values in this design's own `from`/`to` validation — see §6.2's
refusal rationale; this is a design choice layered on top of the
source fact, not a re-opening of C19's citation. Rev-3's PID/temp-directory lock design's "Windows
best-effort" framing is superseded by rev-4's explicit
build-tag-gated `resource-lock-unsupported` contract above — grounded
in the project's actual, tested `ubuntu-latest`/`macos-latest` CI
matrix (`.github/workflows/ci.yml:18-25`), not a hypothetical
cross-platform target — which is a harder, more honest guarantee than
"best-effort.")

## 16. Rev-2 Changelog (vs. rev-1, `e8572b2`/`f0f2c1f`)

- Replaced the fabricated/invalid `--name-only --schema/--data
  --filter=` Dolt argv pattern and two-capability split with one
  source-verified `dolt_diff_summary` SQL query and one capability,
  `diff-summary` (§6).
- Removed the `dolt version` probe entirely; tool identity is now a
  static file fact (`basename` + `SHA-256`), never a code-execution
  result (§6.1).
- Removed every persistent local raw-body concept (`keep_local`, the
  local batch/`current` pointer tree) — raw bytes are now strictly
  ephemeral, scoped to a single invocation's scratch directory (§7.1,
  §8.1).
- Replaced per-resource tracked `summary.json` files with one
  immutable tracked batch file per invocation plus one atomically
  rewritten tracked pointer (§7.3, §12.3–§12.4 as renumbered in
  rev-3 — these were §12.2–§12.3 at the time this changelog entry was
  written in rev-2; kept as historical narrative, not a live
  cross-reference).
- Rewrote the symlink/path gate to refuse any ancestor symlink
  component outright (fail-closed, simpler) instead of resolving and
  re-validating only the final component (§9.1), and introduced a
  separate, opposite-direction executable-path policy (§6.1/§9.2).
- Added literal-pathspec handling and exit-code-shape distinctions to
  every Git ignore/tracked/index-entry invocation (§10).
- Reused the existing `workflow.EnsureLocalIgnoreContract` for the
  scratch root, layering the same tracked-file gate already used for
  `ignored-file` selectors on top (§10.3).
- Designed crash-safe, PID-reuse-guarded lock semantics using
  `ps -o lstart=` (§7.2).
- Removed all wall-clock timestamp fields from every tracked artifact
  (§0.2).
- Replaced every `map`-typed tracked JSON field with a sorted
  `[]struct` array (§12).
- Renamed the per-resource `changes` field to `result` consistently
  (§0.2).
- Recomputed golden vectors 2/3 for the renamed `diff-summary`
  capability; vectors 1/4 unchanged (§0.3).
- Rebuilt the acceptance-criteria set from 41 to 48 clauses, covering
  every new mechanism above (§14).

## 17. Rev-3 Changelog (vs. rev-2, `4ea011e`)

- Replaced all `dolt diff --name-only`/`--schema`/`--data`-flag
  templates and multi-filter classification with one exact
  `dolt sql -r json -q "SELECT ... FROM
  dolt_diff_summary(from,to,table) ORDER BY ..."` invocation, source-
  cited at pinned Dolt commit `59fb843` (§6.2).
- Made `db_path` and `table` mandatory Dolt-selector fields (both
  previously absent/optional); documented the resulting hard
  PK-set-change error vs. the previous, now-removed whole-database
  silent-omission path (§5.3, §6.2, §6.4).
- Hard-refused `..`/`NUL`/control/backslash in `from`/`to`/`table`,
  closing the dot-range-vs-explicit-form argument-parsing ambiguity
  discovered via direct source reading of
  `dolt_diff_summary`'s `WithExpressions` (§6.2).
- Rewrote the Dolt JSON parser for the exact `{"rows":[...]}`/`{}`
  shape with strict field presence/type checking and no defensive
  boolean coercion, replacing rev-2's `0`/`1`-tolerant parse (§6.3).
- Confirmed `WORKING`/`STAGED` support and the closed 4-value
  `diff_type` enum via direct source reads, resolving both of rev-2's
  open questions (§6.2, §6.4, §15).
- Replaced pathname re-`Lstat` descriptor verification with
  `os.SameFile` on the actually-opened file descriptor's `FileInfo`,
  closing a residual TOCTOU race a second pathname lookup could still
  hit (§9.1).
- Replaced the `O_CREATE|O_EXCL` single-file lock with a
  temp-directory-then-atomic-rename design (`owner.json` written and
  fsynced inside a `.lock.tmp-<nonce>/` directory before the directory
  itself is renamed onto `.lock`), eliminating the partial-observation
  window rev-2's file-based lock could not fully close (§7.2).
- Extended the per-slug lock to `add`/`remove`/`clear`, not only
  `capture`/`record --resources`, so no mutator can race a concurrent
  capture (§7.2, §7.6).
- Replaced scratch-file capture of ignored-file content and Dolt
  stdout/stderr with bounded in-process memory buffers — no
  unredacted byte is ever written to any file before scanning
  completes, closing the last remaining persistent-raw gap ADR-027 D3
  identifies (§7.1, §8.1).
- Replaced the random/sequential batch-ID scheme with a
  content-addressed `rb_<12hex>` derived from
  `CanonicalBatchJSON({"feature","results"})`, making retries of
  unchanged content idempotent by construction and defining explicit
  collision handling for the (expected-unreachable) case of differing
  content under the same ID (§7.3, §12.3).
- Corrected rev-2's "fresh `batch_id` on every retry" framing — an
  idempotent retry of *unchanged* content now correctly reproduces the
  same `batch_id` (§11).
- Fixed the invalid `check-ignore --literal-pathspecs` invocation
  (that flag does not exist for `check-ignore`); added the `./`-prefix
  rule for colon-leading selectors and documented `*`/`?`/`[]` as
  inert for this specific command, while `ls-files
  --error-unmatch` keeps `--literal-pathspecs` (§10.1).
- Fixed the stale `§7.3, §12.2–§12.3` cross-reference in the rev-2
  changelog (§16) to reflect rev-3's renumbered `§12.3`–`§12.4`,
  annotated as historical narrative rather than a live pointer.
- Added the directory `ignored-file` result's per-file `files[]` array
  (`{path, raw_sha256, byte_count, mode}`) alongside the existing
  aggregate fields (§12.2).
- Rebuilt the acceptance-criteria set from 48 to 70 clauses, covering
  every mechanism above (§14).

## 18. Rev-4 Changelog (vs. rev-3 + addendum, `151a50e`, adjudication `4d9dd21`)

- Replaced the entire PID/`process_start`/hostname/temp-directory/
  quarantine lock protocol with a single, persistent, kernel-`flock`'d
  file (`O_CREATE|O_RDWR, 0600` + `flock(LOCK_EX|LOCK_NB)`), eliminating
  the unlink/recreate (ABA) race class inherent to any lock protocol
  built from `os.Rename`/`os.RemoveAll` on a named filesystem entry,
  and removing this design's need to reimplement process-liveness
  detection (`ps -o lstart=`) at all — the kernel now guarantees
  crash-safe release with zero lines of this design's own code
  involved (§7.2).
- Defined an explicit build-tag platform contract for the lock:
  `unix`-tagged real `flock`, `!unix`-tagged stub returning
  `resource-lock-unsupported` (exit 3) deterministically for every
  mutating verb, replacing rev-3's "best-effort/unsupported" framing
  with a hard, testable contract (§7.2, §15).
- Fixed the batch-write idempotency-comparison bug: rev-3 compared a
  candidate batch's canonical **hash-input** bytes (which exclude
  `batch_id` and use a compact encoding) against the on-disk file,
  which always includes `batch_id` and uses indented formatting — this
  comparison could never succeed, turning every retry into a spurious
  `batch-id-collision` refusal. Rev-4 compares the freshly-encoded,
  complete **file-wire** bytes (including `batch_id`, real indentation)
  against the on-disk file instead (§7.3, §12.3).
- Corrected `remove`/`clear` to **never** write, rename, or delete
  `current.json` or any `batches/<id>.json` file — only
  `resources.json`, under the per-slug `flock` — reversing rev-3's
  design where `remove`/`clear` pruned `current.json`'s live index,
  which contradicted §7.3's "the sole commit point" framing by making
  `current.json` writable by a third verb class (§3, §4, §12.5).
- Corrected `diff` to explicitly state that it **reads current file
  content** through the same bounded in-memory scanner `capture` uses
  to recompute a real hash — removing rev-3's residual "without opening
  file content"/metadata-only framing, which contradicted this same
  section's own hash-recomputation requirement (§3, §5.1).
- Added the sequential-read-consistency residual for directory
  `capture`/`diff`: each matched file is read one at a time, not under
  a single atomic filesystem-level snapshot; removed the "point-in-time
  snapshot" overclaim and documented the residual honestly (§5.1, §15).
- Enforced actual cap-plus-one reads (not a pre-read
  `Stat().Size()`/output-length check) for both ignored-file content
  and Dolt stdout/stderr capture, so a file/stream that grows after an
  initial size check cannot silently bypass the declared limits (§6.4,
  §8.1).
- Added the `db_path`/`cmd.Dir` honesty paragraph: because
  `os/exec.Cmd.Dir` is a pathname, not a descriptor, this design
  re-runs the path gate immediately before `cmd.Start()`, holds an
  open directory descriptor across the Dolt child's lifetime, and
  re-checks identity after the process exits — a **detection**, not
  **prevention**, mechanism, stated as an honest residual rather than
  a closed sandbox claim (§9.1, §15).
- Upgraded three Dolt-protocol citations to the precise code path this
  design actually exercises: the row-constructor `getRowFromSummary`
  (`dolt_diff_summary.go:457-464`) for native-boolean evidence (C25,
  superseding a schema-type-only citation), the four exhaustive
  `DiffType`-field assignment lines in `GetSummary`
  (`table_deltas.go:722/733/745/760`) for the closed enum, explicitly
  noting `DiffTypeAll` is filter-only and never assigned (C26,
  superseding a const-block-only citation), and the real captured
  stdout whitespace shapes (`"...]}\n"` nonempty, `"{}\n\n"` zero-row)
  grounding the "trim whitespace before parse" requirement with cited
  evidence (C27) (§0.1, §6.2, §6.3).
- Extended the local-ignore/untracked-root gate to run identically for
  `remove`/`clear`, before their own `.lock` acquisition — not just the
  mutators that create scratch content (§10.3).
- Rebuilt the acceptance-criteria set from 70 to 72 clauses (removed
  rev-3's 7 PID/quarantine-specific clauses, added 9 new clauses for
  the flock redesign, cap-plus-one reads, `db_path` residual detection,
  JSON whitespace-trim parsing, and the remove/clear/`current.json`
  correction) (§14).
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).

## 19. Rev-5 Changelog (vs. rev-4, `07eab8e`)

- Replaced the tautological `db_path`/`cmd.Dir` post-exit check
  (`fstat` of a held descriptor compared against itself) with an
  honest **pathname-vs-descriptor** design: a fresh `Lstat` of the
  `db_path` pathname immediately before `cmd.Start()`, compared
  (`os.SameFile`) against the held descriptor, and a second,
  independently fresh `Lstat` after the child exits, again compared
  against the same held descriptor — explicitly labeled as detection,
  not prevention, with a documented well-timed-attacker residual
  during the child's own execution (§9.1, AC-34).
- Narrowed the platform build-tag contract from the broad `unix` tag
  to an exact `linux || darwin` (real implementation) /
  `!linux && !darwin` (fallback) split, matching this project's actual
  tested CI matrix (`ubuntu-latest`/`macos-latest`) rather than every
  POSIX-family `GOOS` value `unix` also covers (§7.2, AC-48, AC-77).
- Added a `statfs`-based filesystem preflight for the lock, refusing
  `resource-lock-filesystem-unsupported` (exit 3, distinct from the
  build-tag-based `resource-lock-unsupported`) on network/shared or
  unrecognized filesystem types, with an explicit per-OS allow/deny
  list and a documented "operator-configured local-only precondition"
  fallback if the allowlist proves too brittle (§7.2, AC-78).
- Fixed a scratch-tree diagram bug: rev-4's local-scratch-tree diagram
  incorrectly placed tracked `batches/<id>.tmp-*.json` and
  `.tmp-current.json` temp files under the local, gitignored tree.
  Rev-5 splits this into a corrected local-only tree and a separate
  tracked-tree diagram showing these temps beside their real,
  tracked destinations, and clarifies that `--dry-run` never runs
  either sweep (§7.1).
- Replaced the unbounded `bytes.Buffer` Dolt stdout/stderr capture
  with `StdoutPipe`/`StderrPipe`-based concurrent draining into one
  **shared** cap-plus-one budget; on overflow, the child's entire
  process group is killed (`SIGTERM` then `SIGKILL`), never truncated,
  and the JSON parser is never invoked on partial output — refused
  `resource-limit-exceeded` (exit 3). Only stdout is ever parsed;
  stderr is captured within the same shared budget for local
  diagnostics only (§6.4, §8.1, AC-83, AC-84).
- Flipped `WORKING`/`STAGED` from accepted to explicitly refused
  (case-insensitive, `dolt-argument-refused`, exit 2): although
  `ResolveRootForRef` genuinely does resolve these exact-case
  constants (a true, unchanged source fact, C19), the working tree/
  staged index they name is gated by Dolt's own `dolt_ignore` table,
  which can silently omit a table from the row set the same way
  `.gitignore` omits a file from a Git listing — an independent
  silent-omission path this design otherwise works to close via
  mandatory `table`. v1 accepts committed refs only (§6.2, AC-10,
  AC-11).
- Removed the `[:12]`-truncated `batch_id` in favor of the full,
  untruncated 64-hex-character SHA-256 digest — a 48-bit truncated ID
  is collision-prone for a scheme whose own collision handling is a
  fatal integrity error, not a display convenience. Resource IDs
  (`res_` + 12 hex) are a separate, unaffected convention (§7.3,
  §12.3, AC-76).
- Added an explicit "batches are an unordered, content-addressed set —
  not a chronology" clarification: `batch_id` names distinct content,
  not a position in a sequence; an A→B→A capture sequence produces
  exactly two batch files, not three, and `current.json` is the sole
  authority for "what is current now" — event-level chronology is
  explicitly out of scope for v1 (§4, §7.3, AC-87).
- Folded directory `mode` into the per-file `combined_hash` input
  (`path\x00mode\x00hash`, sorted by path) and into `diff`'s
  comparison, so a chmod-only change (identical content, changed
  permission bits) is now distinguishable from a content change
  (§5.1, §12.2, AC-86).
- Corrected the JSON zero-row parsing description from an exact-byte
  match on the trimmed literal `{}` to a **structural** acceptance of
  either `{}` or `{"rows":[...]}` after trimming surrounding
  whitespace, citing the real captured shapes (`"...]}\n"`,
  `"{}\n\n"`) as illustrative examples, not an exact-byte contract
  (§6.2).
- Removed the optional "debugging comment" allowance for the `.lock`
  file's body — the lock file has no body at all, matching the
  existing ADR-033 D9 language exactly (§7.2).
- Rebuilt the acceptance-criteria set from 72 to 78 clauses: six new
  clauses (`AC-77` exact build-tag text, `AC-78` filesystem-preflight
  refusal, `AC-83` output-cap-as-refusal via process-group kill,
  `AC-84` shared stdout+stderr budget, `AC-86` chmod-only
  diff-distinguishability, `AC-87` A→B→A produces exactly two batch
  files) and two clauses rewritten in place without changing the total
  (`AC-10`/`AC-11` WORKING/STAGED refusal, `AC-34`/`AC-48` pathname-
  vs-descriptor and exact build-tag wording) (§14, §14.1).
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).

## 20. Rev-6 Changelog (vs. rev-5, `b312e4a`)

- Added a Dolt binary **trust pin**: `add --kind adapter-snapshot
  --adapter dolt --trust-current-dolt` resolves and hashes the Dolt
  executable at `add` time, writing a mandatory `binary_sha256`
  `args` entry (participates in `resource_id`, §13); `add` without
  the flag is refused `dolt-trust-required` (exit 2). Every `capture`
  now re-hashes the resolved binary both immediately **before**
  invocation (mismatch: `adapter-binary-untrusted`, exit 3, no
  invocation attempted, `AC-19`) and immediately **after** (same
  error, result discarded, `AC-18`) — superseding rev-4/rev-5's
  cheaper device/inode/size/mtime `Lstat` recheck (§6.1, AC-18,
  AC-19, AC-20).
- Upgraded the `db_path`/`cmd.Dir` identity check from a
  diagnostic-only detection (rev-4/rev-5) to a **hard refusal**
  (`db-path-identity-changed`, exit 3, result discarded, no batch
  written) at both the pre-`cmd.Start()` and post-exit re-checks,
  while continuing to document the narrower, honestly-scoped
  in-window residual (§9.1, ADR-033 D6, AC-34).
- Replaced the vague "process-group termination" language with an
  exact `SysProcAttr{Setpgid: true}` mechanism (`linux`/`darwin`):
  the child's negative PGID is signaled `SIGTERM` then `SIGKILL`
  after a grace period, verified by tests that spawn a descendant
  process and confirm both the descendant's termination and the
  `tpatch` parent process's own survival (§6.4, AC-85).
- Added `resource-id-collision` (exit 3): `add`/load-time recomputes
  each `resources.json` entry's `resource_id` from its own fields —
  identical canonical bytes at an existing ID is an idempotent no-op
  (exit 0), different bytes at the same ID is refused, never
  overwritten; a test-only stub hash function is the collision-
  testing seam since a real `SHA-256` collision cannot be produced
  for a test (§4, §13, AC-88).
- Renamed `latest_batch_id` to `current_batch_id` and removed all
  "newest"/"latest" chronology language: the field is solely
  `current.json`'s own provenance fact (which batch the last
  rewriting invocation staged), not a claim about recency across the
  unordered batch set (§4, §7.3, §12.4).
- Split the batch-publish idempotency check into three distinct
  outcomes instead of a single byte-comparison: identical file-wire
  bytes (unchanged, `AC-55`); **different** file-wire bytes whose
  canonicalized semantic body (decoded, `batch_id`-stripped,
  re-encoded) matches the freshly-staged candidate — **presentation
  drift**, treated as idempotent, the immutable file is never
  rewritten (`AC-56`); and genuinely different semantic bodies under
  the same `batch_id` — a true `batch-id-collision` (`AC-57`). An
  unparseable or internally-inconsistent on-disk batch file is a
  separate `batch-file-corrupt` (`AC-58`), never routed through
  either comparison (§7.3).
- Replaced `golang.org/x/sys/unix.Statfs` with **stdlib-only**
  `syscall.Statfs` (`AC-79`), corrected the exact Linux magic-number
  allow/deny lists (added `overlayfs` `0x794C7630`; removed the
  invalid rev-5 Linux-APFS entry, since no such constant exists) and
  Darwin `Fstypename` allow/deny lists (added `tmpfs` to the allow
  list; named `osxfuse`/`macfuse` to the deny list), with every
  listed type exercised via table-driven Test Matrix supporting rows
  under one `AC` (§7.2, AC-80).
- Added **first-create sequencing**: the local-ignore/untracked gate
  and filesystem preflight now run against the **nearest existing
  ancestor** of a not-yet-created directory (never the not-yet-
  created leaf itself), followed by `MkdirAll` and an explicit
  `fsync` of every newly-created directory's parent before the
  invocation proceeds — closing a durability gap rev-5 left open for
  a slug's or tree's first-ever creation (§7.1, AC-81, AC-82).
- Clarified the directory `combined_hash` tuple encoding
  unambiguously: each of `path`/`mode`/`hash` (the **raw, unprefixed
  64-lowercase-hex** digest, distinct from the wire-level
  `"sha256:"`-prefixed `hash`/`raw_sha256` fields) is individually
  `0x00`-terminated, and files' tuples concatenate directly with no
  further separator — rev-5's "`\x00`-joined" phrasing left this
  ambiguous. Added a worked, byte-identical (PRD/ADR) golden
  directory vector (§5.1, §12.2, AC-89).
- Added `dolt-argument-refused` as the exact named exit-2 reason
  covering `..`, control/backslash bytes, and case-insensitive
  `WORKING`/`STAGED`, consolidating what were previously several
  inline mentions into one named, cross-referenced error class
  (§6.2, §11).
- Corrected two stale citations: `internal/workflow/session_ignore.go`'s
  `EnsureLocalIgnoreContract` line range from `138-175` (which stopped
  mid-function, before the `ignored` check itself) to the full
  `138-194` function span (§0.1, C13); and the resource-ID/full-batch-ID
  golden vectors in §13.3 (which still carried stale rev-3-era values)
  to match §0.3's rev-6-recomputed values.
- Rebuilt the acceptance-criteria set from 78 to **89** clauses (11
  new, none removed, 3 rewritten in place) — see §14.1 for the exact
  accounting.
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).

## 21. Rev-7 Changelog (vs. rev-6, `d503d55`)

- **Executed-binary binding via private-copy execution**: closed the
  rev-6 residual where the resolved Dolt pathname was hashed twice
  (pre/post-invocation) but never bound to the bytes actually
  executed. Rev-7 opens the resolved binary once, streams its bytes
  through a `SHA-256` digest (`io.TeeReader`) while copying them into
  a private, per-invocation ephemeral scratch file, verifies the
  digest against `trust.binary_sha256` before hardening the copy
  (`chmod 0500`), and executes the **copy's own path** — never the
  originally-resolved pathname — deleting the copy after the child
  exits (§6.1, AC-18, AC-19, AC-93, AC-94).
- **Trust/identity split**: `binary_sha256` is removed from a Dolt
  resource's `args` and `resource_id` hash input entirely, replacing
  it with a new `contract` enum key (`"dolt-diff-summary-v1"` in v1,
  which **does** participate in identity). The trust pin now lives in
  a separate, mutable `trust` field, excluded from identity — so a
  legitimate Dolt binary upgrade no longer silently orphans a
  resource's `resource_id`/`current.json`/history (§4, §6.1, §12.1,
  §13, AC-20, AC-91). Added `trust-dolt <slug> <resource-id>
  --binary-sha256 <64hex>`, the only way to re-pin trust after `add`
  without discarding identity or history — under the same per-slug
  `flock`, rewriting only `trust.binary_sha256` (§3, §12.6, AC-92).
- **Runtime-compatibility honesty**: reframed the trust pin as
  operator-approved exact binary identity, not proof of matching any
  specific pinned upstream source commit; added `dolt-trust-required`
  as a distinct capture-time refusal for a missing pin (§6.1, AC-90).
- **Resource corruption taxonomy split**: separated the previously
  conflated rev-6 `resource-id-collision` outcome into two named
  cases — `resource-id-collision` (two **distinct** declarations
  whose independently-correct IDs collide, `AC-88` narrowed) and the
  new `resources-file-corrupt` (a single entry's own recorded ID no
  longer matches its own recomputed identity, `AC-95`) — with loaded
  entries now explicitly mapped by `resource_id` (§4, AC-88, AC-95).
- **Group-termination fix**: on the kill path, the process-group
  leader is now kept **unreaped** (no `cmd.Wait()`) through the entire
  fixed grace period regardless of the direct child's own apparent
  exit, with `SIGKILL` to the negative PGID (tolerating `ESRCH`) sent
  unconditionally before the first `Wait()` call — closing the rev-6
  gap where "escalation cancelled the moment `cmd.Wait()` observes the
  group has exited" only proved the direct child's exit, not the
  whole group's, permitting PGID reuse while a rogue descendant
  remained alive. A separate normal-success path calls `Wait()`
  immediately on EOF with no artificial delay (§6.4, AC-85, AC-96,
  AC-97).
- **Fresh-clone gate correction**: split the first-create sequencing
  so the local-ignore/untracked gate targets the **intended,
  not-yet-existing leaf** directly (existence-independent pathname
  checks), while the `statfs` preflight remains scoped to the nearest
  existing ancestor (a genuinely existence-bound kernel call) —
  closing the rev-6 gap where both checks ran against whatever
  ancestor happened to exist, which does not establish anything about
  the specific leaf about to receive untracked, ignored content (§7.1,
  AC-81, AC-98).
- **Unconditional retry-fsync**: both the local scratch tree and the
  tracked `artifacts/resource-captures/`/`batches/` tree now
  re-`fsync` every directory in the relevant chain on a retried
  invocation, not only directories `MkdirAll` reports as newly created
  — a directory can become visible to `Stat`/`Lstat` before the kernel
  has made its creation crash-durable, so a retry that only fsyncs
  "new" entries could still lose an earlier, not-yet-durable creation
  across a second crash (§7.1, AC-81, AC-82, AC-99).
- **Statfs width normalization**: the Linux `statfs` allow/deny
  comparison now explicitly normalizes `fsType := uint32(buf.Type)`
  before comparison, with allow/deny constants themselves typed
  `uint32` — `Statfs_t.Type`'s width/signedness is architecture-
  dependent (`int64` on `amd64`/`arm64`, `int32` on `386`/`arm`,
  `uint32` on `s390x`, C35), so an unnormalized raw comparison is
  architecture-fragile (§7.2, AC-100).
- Recomputed all Dolt-adapter golden vectors for the `args`
  `binary_sha256`→`contract` substitution: `resource_id` Vectors 2/3
  changed from rev-6's `res_00189e66780a` to `res_4b62313b6cce`
  (reordered-key equivalence reconfirmed); the full batch ID changed
  from `rb_fcc1d4c46051f192b9005f8941fa54dbf9e907e2609e9fceb393acef2c70ed0a`
  to `rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021`
  (§0.3, §12.3, §12.4, §13.3) — the three-result sort order within the
  batch is unchanged (`res_4b62313b6cce` < `res_79f5ac5dca13` <
  `res_acc91dc23a8b`).
- Rebuilt the acceptance-criteria set from 89 to **100** clauses (11
  new, none removed, 6 rewritten in place: `AC-18`/`AC-19`/`AC-20`/
  `AC-81`/`AC-85`/`AC-88`) — see §14.1 for the exact accounting.
- Added three new Claims-Audit rows (`C35`/`C36`/`C37`) grounding the
  `statfs` width/architecture fact, the POSIX process-group/PGID-reuse
  fact, and the `io.TeeReader`+`crypto/sha256` streaming-hash-while-
  copying stdlib pattern (§0.1).
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).

## 22. Rev-8 Changelog (vs. rev-7, `bc2c068`)

- **Unified process-group termination**: replaced rev-7's two-path
  design (a kill path deferring `Wait()` past `SIGKILL`, and a
  separate normal-success path calling `Wait()` immediately on EOF)
  with a **single sequence** applied to every invocation regardless of
  outcome: drain both pipes to EOF, unconditional `SIGTERM(-pgid)`
  (tolerating `ESRCH`), unreaped through the fixed grace period
  regardless of trigger, unconditional `SIGKILL(-pgid)` (tolerating
  `ESRCH`), then exactly one `Wait()` call. There is no "`Wait()`
  observes the group" claim anywhere in this design. Two trade-offs
  are now explicitly disclosed: every invocation, even an instant
  success, pays the fixed grace latency before `Wait()`; and a leader
  that closes its pipes while still alive doing further work is now
  forcibly terminated, surfaced through the existing `dolt-query-error`
  taxonomy (§6.4, AC-96, AC-97).
- **Trust bootstrap split**: `add --trust-current-dolt` is now a
  distinct **add-time TOFU bootstrap** sub-sequence — resolve, open,
  stream-copy-while-hash, write the pin, delete the copy — that
  requires no existing pin and never executes the binary. Capture-time
  trust verification is a separate, later sequence that requires the
  pin to already exist (`dolt-trust-required`, exit 3) and is the only
  path that executes the private copy. The add-time missing-flag
  refusal is renamed to the distinct `dolt-trust-flag-required` (exit
  2), no longer sharing a name with the capture-time refusal (§3,
  §6.1, AC-18, AC-19, AC-20, AC-102).
- **Duplicate-`add` no-repin guarantee made explicit**: a duplicate
  `add` targeting an identical declaration — with or without
  `--trust-current-dolt` re-passed, and regardless of whether the
  freshly-resolved binary's hash now differs from the stored pin — is
  a strict no-op that leaves `trust.binary_sha256` byte-for-byte
  unchanged; only `trust-dolt` may ever re-pin after the initial `add`
  (§4, AC-103).
- **Private-copy host-failure handling**: added a `statfs`-based
  no-exec preflight on the private-copy scratch filesystem
  (`ST_NOEXEC` on Linux, `MNT_NOEXEC` on Darwin), refusing
  `adapter-copy-noexec` (exit 3) before any byte is copied; and added
  explicit `ENOSPC`/`EIO` handling during the streamed copy, refusing
  `adapter-copy-failed` (exit 1) with best-effort cleanup of the
  partial copy. Added a paragraph qualifying why executing the private
  copy remains safe despite living inside `.tpatch/local/`: the bytes
  are descriptor-bound, hash-verified against the trust pin, and
  owner-only (`0500`), independent of which directory holds them
  (§6.1, AC-104, AC-105).
- **Local-gate target split**: the ignore half of the local-root gate
  continues to target the exact per-slug leaf, existence-independent
  (`EnsureLocalIgnoreContract`, unchanged from rev-7); the untracked
  half now targets the **whole** `.tpatch/local/` subtree via a plain
  `git --literal-pathspecs ls-files -- .tpatch/local/` with an
  empty-stdout convention (dropping `--error-unmatch` entirely for
  this check), rather than reusing the per-selector leaf-targeted gate
  — a tracked file anywhere under `.tpatch/local/`, for any slug, now
  refuses every mutator uniformly (§7.1, §10.3, AC-98, AC-101).
- **`trust-dolt` added to every mutator/lock-list enumeration**:
  `trust-dolt` now appears alongside `add`/`remove`/`clear`/`capture`/
  `record --resources` everywhere those five were previously
  enumerated as "all mutating verbs" — the per-slug `flock` list, the
  local-ignore/untracked gate list, and the lock-contention Test
  Matrix rows — correcting a rev-7 omission (§7.2, §10.3, AC-43,
  AC-47).
- **Exit-taxonomy consolidation**: every named refusal now appears in
  exactly one row/table context across §3's exit-code table and §9's
  exit-3 gate table; the `dolt-trust-required`/`dolt-trust-flag-required`
  rename (above) was the specific fix needed to satisfy this, since
  rev-7 had one name shared across two different exit codes and
  contexts (§3, §9).
- **C36 rationale correction**: reworded the Claims-Audit rationale for
  the process-group design away from the debatable "an unreaped leader
  prevents PGID reuse" framing toward the actually-load-bearing fact:
  the `SIGTERM`→grace→`SIGKILL`→group→`Wait()` escalation is never
  skipped or cancelled merely because the direct child appears to have
  exited (§0.1, C36).
- Added two new Claims-Audit rows (`C38`/`C39`) grounding the Linux
  `ST_NOEXEC = 0x8` and Darwin `MNT_NOEXEC = 0x00000004` constants used
  by the new no-exec preflight (§0.1).
- Rebuilt the acceptance-criteria set from 100 to **105** clauses (5
  new, none removed, 3 rewritten in place: `AC-96`/`AC-97`/`AC-98`) —
  see §14.1 for the exact accounting.
- Confirmed all resource-ID, full-batch-ID, and directory-hash golden
  vectors are **unaffected** by this revision's changes (none of the
  trust-split, gate-target-split, or termination-sequence changes
  touch `resource_id`'s or `batch_id`'s canonical hash inputs) — the
  vectors already recomputed in rev-7 remain current (§0.3, §12.3,
  §12.4, §13.3).
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).

## 23. Rev-9 Changelog (vs. rev-8, `8152a8b`)

- **ADR Test Matrix row 146 text fix**: corrected a stale summary
  ("`fsync` of each newly-created directory's parent") to match
  §7.1/D9's already-correct normative prose (unconditional `fsync` of
  every directory in the relevant chain, both newly-created and
  already-visible on retry) — text-only, no behavioral change (task 1;
  §14.1, ADR Test Matrix).
- **Process-group termination redesign, closing a real deadlock**:
  rev-8's unified sequence gated the entire `SIGTERM`→grace→`SIGKILL`
  escalation behind "once its pipes are drained," but pipe drain-to-
  `EOF` cannot complete while any process — including a descendant the
  leader itself spawned — still holds the write end open; a
  successfully-exiting leader with such a lingering descendant left
  the old design with no way to detect that exit and begin the
  group-signal sequence at all. Rev-9 switches from
  `cmd.StdoutPipe()`/`cmd.StderrPipe()` to caller-owned `os.Pipe()`
  pairs assigned directly to `cmd.Stdout`/`cmd.Stderr` (write ends
  closed by the caller immediately after `Start()`), so `cmd.Wait()`
  reflects only the leader's own OS-level exit, independent of pipe
  state (C40, empirically verified). The unconditional group-signal
  sequence now triggers on whichever of three events fires first —
  the decoupled `cmd.Wait()` completing, the 30-second timeout, or the
  output-cap exceeded — and is identical across all three branches;
  only the source of the leader's own exit-status differs (task 2;
  §6.4, D5, AC-96, AC-97, C40).
- **`trust-dolt` added to §7.2's own lock-verb enumeration**: §7.1/
  §10.3/D9 already listed `trust-dolt` among the per-slug-`flock`
  verbs since rev-8, but §7.2's own "every mutating verb" prose still
  named only the prior five — a second, narrower enumeration the
  rev-8 fix had not reached. Fixed to list all six verbs; no AC change
  (`AC-47` already covered all six) (task 3; §7.2).
- **Three findings verified already resolved, recorded rather than
  silently dropped**: (a) `add --trust-current-dolt`'s TOFU bootstrap
  already has no "refuse if unpinned" precondition and never executes
  Dolt (§6.1); (b) duplicate `add` is already a strict no-op that
  never re-pins trust (§4, `AC-103`); (c) the exit-code table already
  distinguishes `dolt-trust-flag-required` (exit 2) from
  `dolt-trust-required` (exit 3), with `adapter-copy-noexec`/
  `adapter-copy-failed` each appearing exactly once (§3). No changes
  made for these three; see §0's rev-9 fold summary, items 4-6.
- Confirmed all resource-ID, full-batch-ID, and directory-hash golden
  vectors, and the six shared JSON blocks, are **unaffected** by this
  revision's changes (none of the fixes above touch canonical hash
  inputs or wire schemas) — the vectors already current as of rev-7/
  rev-8 remain current (§0.3, §12.3, §12.4, §13.3).
- Acceptance-criteria count is **unchanged** at 105 (`AC-1` through
  `AC-105`): two clauses rewritten in place a second time (`AC-96`,
  `AC-97`), none added or removed — see §14.1 for the exact accounting.
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).

## 24. Rev-10 Changelog (vs. rev-9, `0b15495`)

- **Process-group termination redesign, closing the actual
  PGID-reuse race**: rev-9's decoupled-`cmd.Wait()` design called
  `cmd.Wait()` as soon as leader-exit was detected — but that call
  reaps the leader, releasing its PID/PGID back to the kernel, before
  the `SIGTERM`→grace→`SIGKILL(-pgid)` sequence has necessarily run in
  the "leader exited on its own" branch, reopening the exact PGID-reuse
  gap this design set out to close. Rev-10 keeps the caller-owned
  `os.Pipe()`s but replaces the early `cmd.Wait()` with a build-tagged
  (`linux`/`darwin`) non-reaping observer built on the raw
  `waitid(P_PID, leaderPID, ..., WEXITED|WNOWAIT)` syscall (stdlib-only,
  no `golang.org/x/sys`), which detects leader-exit without reaping it.
  That observation races against the invocation timeout and the
  output-cap trigger; whichever fires first enters one cleanup
  function: unconditional `SIGTERM(-pgid)` → grace → unconditional
  `SIGKILL(-pgid)` → await the observation if not already fired → join
  pipe drains → call `cmd.Wait()` exactly once, only now. Because no
  step before the final `cmd.Wait()` reaps the leader, its PID/PGID
  remains reserved throughout the entire signal-then-grace-then-kill
  window. No "Wait/observer proves group emptiness" claim is made; the
  escaped-session-descendant residual is now stated honestly alongside
  the pre-existing post-`SIGKILL` kernel-teardown-race residual. A
  cross-compile/source-shape test confirms the build-tagged observer
  compiles for `linux` (`amd64`/`arm64`) and `darwin` (`arm64`) with no
  external dependency (task 1; §6.4, D5, AC-85, AC-96, AC-97, new
  AC-106, C36, C40, new C41).
- **`trust-dolt` added to §7.1's top-level "every mutator" local-gate
  enumeration**: distinct from the §7.2 lock-verb list rev-8 already
  fixed and the §7.1 mode-conflict paragraph rev-9 otherwise left
  alone; `AC-52`/ADR row 86 now cover `add`/`remove`/`clear`/
  `trust-dolt` (task 2; §7.1).
- **Taxonomy collision fixed**: `adapter-missing` was reused for both
  the add-time (exit 2) and capture-time (exit 3) missing-executable
  refusal in the same named-refusal slot; renamed the add-time
  occurrence to `adapter-missing-at-add` (exit 2), keeping
  `adapter-missing` exclusively for capture (exit 3). `path-outside-repo`
  was referenced once but never defined as a named exit-3 refusal;
  §9.1 now defines it as the pre-existing lexical
  `EnsureSafeRepoPath`/`NormalizeClaimPath` containment pre-filter's
  own refusal name, and the exit-3 table lists it (task 3; §3, §9.1).
- **Private-copy mode sequence unified across four locations**: PRD
  §6.1 capture-time, PRD §7.1, ADR D5, and ADR D10 each described a
  different creation-mode sequence; unified on one: add-time TOFU
  opens and hashes the resolved binary's descriptor directly (no
  private copy file, zero processes started); capture-time creates the
  private copy `O_CREATE|O_EXCL|O_WRONLY` mode `0600`, streams/hashes/
  `Sync`s it, verifies the digest against the pinned
  `trust.binary_sha256`, and only after that match hardens it to
  `0500` via a descriptor-based `Fchmod` (never a path-based
  `os.Chmod`), verifies the mode, closes, and executes that private
  path (task 4; §6.1, §7.1, D5, D10, AC-93, AC-102).
- **Vector 3/CURRENT parity fixes**: normalized PRD §13.3's Vector 3
  declared-argument order to match PRD §0.3 and ADR D3
  (`to, db_path, table, from, contract`) — a pure documentation fix,
  since the canonical sorted-key JSON encoding used for `resource_id`
  is order-independent and no ID value changed. Corrected
  `docs/handoff/CURRENT.md`'s two references to an "invented Vector 5"
  that does not exist (`res_acc91dc23a8b` is Vector 1, not Vector 5 —
  there are, and have only ever been, four vectors) (task 5; §0.3,
  §13.3, CURRENT.md).
- **Stale-surface sweep**: `AC-85` rewritten to describe the unified
  non-reaping-observer sequence applied to all three trigger branches;
  `AC-96`/`AC-97` rewritten to remove every reference to an early,
  leader-exit-triggered `cmd.Wait()` call; `C36`'s attribution
  corrected to credit the rev-10 mechanism, not rev-8/rev-9, as the
  first design implementing "escalation never skipped, leader unreaped
  through signaling" correctly for all three branches; `C40`'s
  grounding narrowed to state only what it actually proved
  (pipe-drain/`Wait()` decoupling), not that decoupling alone justified
  an early `Wait()`; a stray `cmd.StdoutPipe()`/`cmd.StderrPipe()`
  reference in §5.1's directory-capture prose corrected to caller-owned
  `os.Pipe()`s (task 6; §5.1, §14.1, §0.1).
- Confirmed all resource-ID, full-batch-ID, and directory-hash golden
  vectors, and the six shared JSON blocks, are **unaffected in value**
  by this revision's changes (Vector 3's fix is declaration-order-only;
  no canonical hash input changed) — the vectors already current as of
  rev-7/rev-8/rev-9 remain current (§0.3, §12.3, §12.4, §13.3).
- Acceptance-criteria count is a **net +1**, now **106** (`AC-1`
  through `AC-106`): one clause added (`AC-106`), six rewritten in
  place (`AC-52`, `AC-85`, `AC-93`, `AC-96`, `AC-97`, `AC-102`), none
  removed — see §14.1 for the exact accounting.
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).
