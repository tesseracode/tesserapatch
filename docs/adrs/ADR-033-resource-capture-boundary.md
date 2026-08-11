# ADR-033 — Resource Capture Boundary (rev-13)

**Status**: Accepted — Cluster H rev-13 (`650b44f`; internal APPROVED
WITH NOTES, external APPROVED; see `docs/supervisor/LOG.md`)
**Planning range**: `f04dec7..650b44f`; implementation is a separately
based Cluster H′ wave.

**Context**: `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
(rev-13, companion document — this ADR binds the decisions that PRD's
design depends on; read the PRD first for full rationale, this ADR
states the decisions themselves plus the Test Matrix).

**Related**: `ADR-027-capture-context-privacy-boundary.md` (D1–D6,
directly extended by D4 below), `ADR-030-multi-slug-reconcile-derivation-mode.md`
(`.git/**` exclusion precedent), `ADR-032-feature-unapply-state-boundary.md`
(fixed-struct JSON precedent), `docs/state-of-the-art/storage-substrate-and-versioned-data.md`
§3, §9 (tracked Dolt/substrate research), `internal/workflow/session_ignore.go`
(`EnsureLocalIgnoreContract`, reused by D8)

---

## Rev-6 fold summary

The rev-5 adjudication (`b312e4a`) found rev-5's own new mechanisms
still unsafe or under-specified in eight concrete places, framed as a
bounded compatibility fold that does not reopen D1/D2's authority/
scope decisions: any resolved `dolt` binary was accepted without a
compatibility trust pin, so a same-named but semantically different
binary could silently change what a tracked result means (D5/D6 fix);
a detected post-exit `db_path` pathname replacement was
diagnostic-only, logging the mismatch but still publishing the batch,
rather than refusing the capture outright (D6 fix); process-group
termination lacked `SysProcAttr{Setpgid:true}`, so `SIGTERM`/
`SIGKILL` could reach `tpatch`'s own process group instead of only the
spawned Dolt child and its descendants (D5 fix); the 12-hex-truncated
`resource_id` keyspace had no distinct-payload collision refusal (D3/
D7 fix); `latest_batch_id` reintroduced chronological ("newest")
language into a design whose own D7 insists batches are an unordered
content-addressed set (D7 fix); an existing batch file's byte-level
drift from a freshly re-encoded candidate was labeled a `SHA-256`
collision without first canonicalizing and comparing the **semantic**
body, conflating presentation drift with a genuine cryptographic
collision (D7 fix); the filesystem preflight used
`golang.org/x/sys/unix` with Linux/macOS allow/deny lists that
differed between the PRD and ADR, included at least one invalid
constant, omitted `overlayfs`, and did not `fsync` the first-created
parent directory of a scratch tree (D9 fix); and the directory
`combined_hash` tuple never stated whether its hash component was raw
hex or `sha256:`-prefixed, and had no worked golden vector (D4/Wire
Schema Appendix fix). This rev-6 rewrite resolves every finding; see
the companion PRD's §0.1 Claims Audit (C31–C34) for the
corrected-citation rows (not repeated here to avoid drift — this ADR
cites the PRD's rows by ID, e.g. "C31," where relevant).

**Preserved across every review pass to date (rev-1 through rev-6,
plus the rev-3 citation addendum — six review passes total, matching
the companion PRD's count)**: a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Rev-7 fold summary

The rev-6 adjudication (`d503d55`) found rev-6's own new trust/process
mechanisms still unsafe or under-specified in seven concrete places,
framed as a bounded maintenance/trust fold that does not reopen D1/D2's
authority/scope decisions: the binary trust pin hashed the **resolved
pathname** before and after invocation but never bound those hashes to
the bytes the child process actually executed, leaving a TOCTOU window
between the pre-invocation hash and `cmd.Start()` (D5/D6 fix — private-
copy execution); `binary_sha256` lived inside `args` and therefore
inside `resource_id`'s hash, so a legitimate Dolt binary upgrade
destroyed a resource's identity, `current.json` pointer, and batch
history (D3/D5 fix — trust/identity split, new `contract` enum key,
new `trust-dolt` command); process-group escalation cancelled the
instant `cmd.Wait()` observed *the direct child's* exit, which does not
prove the whole process group has exited and permits PGID reuse while
a rogue descendant remains alive (D5 fix — unreaped-leader-through-
grace); a single load-time `resource-id-collision` outcome conflated
two distinct-declaration collisions with one entry's own self-
inconsistent recorded ID (D3 fix — `resources-file-corrupt` split
out); the first-create ignore/untracked gate and the `statfs`
preflight both ran against whatever ancestor happened to exist,
establishing nothing about the specific not-yet-existing leaf about to
receive untracked, ignored content (D9 fix — leaf-targeted gate,
ancestor-targeted `statfs`); a retried invocation only re-`fsync`ed
directories `MkdirAll` reported as newly created, which can miss an
earlier, not-yet-durable creation that became `Stat`-visible before an
earlier crash (D9 fix — unconditional retry-fsync); and the Linux
`statfs` allow/deny comparison used `buf.Type` directly without
normalizing its architecture-dependent width/signedness (`int64` on
`amd64`/`arm64`, `int32` on `386`/`arm`, `uint32` on `s390x`) against
`uint32`-typed constants (D9 fix). This rev-7 rewrite resolves every
finding; see the companion PRD's §0.1 Claims Audit (C35–C37) for the
new source-grounding rows.

**Preserved across every review pass to date (rev-1 through rev-7,
plus the rev-3 citation addendum — seven review passes total, matching
the companion PRD's count)**: a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Rev-8 fold summary

The rev-7 adjudication (`bc2c068`) found rev-7's own new lifecycle/
trust mechanisms still unsafe or under-specified in eight concrete
places, framed as a bounded acceptance/lifecycle micro-fold that does
not reopen D1/D2's authority/scope decisions: the Test Matrix's row
146 still described the superseded rev-6 ancestor-only ignore/
untracked+`statfs` design rather than rev-7's own leaf-targeted gate
(D9/matrix fix); process-group termination had two separate paths (a
kill path deferring `Wait()` past `SIGKILL`, and a normal-success path
calling `Wait()` immediately on EOF), which is two designs to keep
consistent rather than one, and left undefined what happens to a
leader that closes its pipes while still alive (D5 fix — one unified
sequence for every invocation); `add --trust-current-dolt` literally
would fire during its own first-time pin computation, since by
definition no pin exists yet the first time that flag runs; the
sequence needed to be split into a non-executing add-time bootstrap
and the existing pin-requiring, executing capture-time sequence (D5
fix — TOFU bootstrap split); `trust-dolt` was missing from at least
one "every mutator"/local-ignore-gate enumeration, and the duplicate-
`add` idempotency language ambiguously implied a re-passed
`--trust-current-dolt` might update an existing entry's `trust` field
as a side effect (D8/D9 fix — `trust-dolt` added everywhere, strict
no-repin made explicit); the exit-code taxonomy had `dolt-trust-
required` doing double duty as both an add-time exit-2 name and a
capture-time exit-3 name in the same named-refusal slot, violating
this design's own "each named refusal in exactly one row" convention
(D5/exit-table fix — `dolt-trust-flag-required` rename); the capture-
time private-copy sequence had no defined behavior for a `noexec`-
mounted scratch filesystem or an `ENOSPC`/`EIO` failure during the
copy (D5 fix — `statfs` no-exec preflight, host-I/O-failure handling);
the scratch-root untracked-gate target was ambiguously reused from the
per-selector leaf-targeted gate, contradicting the scratch root's own
worked fresh-clone example, which already targeted the whole
`.tpatch/local/` subtree (D8 fix — ignore/untracked target split); and
C36's rationale framed the fix as "an unreaped leader prevents PGID
reuse" rather than the actually-load-bearing fact that the escalation
is never skipped or cancelled merely because the direct child appears
to have exited (Claims Audit fix). This rev-8 rewrite resolves every
finding; see the companion PRD's §0.1 Claims Audit (C38–C39, C36
reworded) for the new/corrected source-grounding rows.

**Preserved across every review pass to date (rev-1 through rev-8,
plus the rev-3 citation addendum — eight review passes total, matching
the companion PRD's count)**: a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Rev-9 fold summary

The rev-8 adjudication (`8152a8b`) found rev-8's own new termination/
lock-list mechanisms still unsafe or under-specified in three concrete
places (plus three findings verified already resolved by rev-8's own
work and recorded rather than silently dropped), framed as a bounded
terminal micro-fold that does not reopen D1/D2's authority/scope
decisions: the Test Matrix's row 146 still said the retry-fsync step
targeted "each newly-created directory's parent" rather than D9's own
already-correct unconditional whole-chain fsync (text-only fix, no
behavioral change); D5's unified process-group termination sequence
gated the entire `SIGTERM`→grace→`SIGKILL` escalation behind "once its
pipes are drained," but pipe drain-to-`EOF` cannot complete while any
process — including a descendant the leader itself spawned — still
holds the write end open, so a leader that exited successfully while
such a descendant lingered left the old design with no way to detect
that exit and begin the escalation at all (D5 fix — leader-exit
detection decoupled from pipe drain via caller-owned `os.Pipe()`s
assigned to `cmd.Stdout`/`cmd.Stderr`, so `cmd.Wait()` reflects only
the leader's own OS-level exit; the escalation now triggers on
whichever of three events fires first — the decoupled `cmd.Wait()`,
the 30-second timeout, or the output-cap exceeded — with the same
unconditional signal sequence run from all three); and one of D9's own
supporting sections still enumerated only five of the six per-slug
`flock`-acquiring verbs, omitting `trust-dolt`, even though D9's own
primary enumeration already included it (text-only fix — the omission
was in a second, narrower list, not in D9's own authoritative
statement, and required no new AC). The three verified-clean findings
— the add-time TOFU bootstrap already requiring no existing pin and
never executing Dolt, duplicate `add` already being a strict no-op
that never re-pins trust, and the exit-code table already cleanly
distinguishing `dolt-trust-flag-required` from `dolt-trust-required` —
are confirmed against this rev-9 adjudication's own described symptoms
and found already resolved; no further change was made for them. This
rev-9 rewrite resolves the three real findings and confirms the three
verified-clean ones; see the companion PRD's §0.1 Claims Audit (`C40`)
for the new source-grounding row.

**Preserved across every review pass to date (rev-1 through rev-9,
plus the rev-3 citation addendum — nine review passes total, matching
the companion PRD's count)**: a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Rev-10 fold summary

The rev-9 adjudication (`19e3024`) found rev-9's own termination
redesign reopened the exact PGID-reuse race it set out to close, plus
five further consolidated findings, framed as a bounded terminal
correction that does not reopen D1/D2's authority/scope decisions:
rev-9's decoupled-`cmd.Wait()` design called `cmd.Wait()` as soon as
leader-exit was detected via the caller-owned `os.Pipe()`s — but that
call reaps the leader, releasing its PID/PGID back to the kernel's
reuse pool, *before* the `SIGTERM`→grace→`SIGKILL(-pgid)` sequence has
necessarily run at all in the "leader exited on its own" branch (D5
fix, task 1): rev-10 keeps the caller-owned pipes but replaces the
early `cmd.Wait()` with a build-tagged (`linux`/`darwin`) **non-
reaping** observer built on the raw `waitid(P_PID, leaderPID, ...,
WEXITED|WNOWAIT)` syscall (stdlib-only, no `golang.org/x/sys`), which
detects the leader's exit without reaping it; that observation races
against the invocation timeout and the output-cap trigger, and
whichever fires first enters one cleanup function — unconditional
`SIGTERM(-pgid)` → grace → unconditional `SIGKILL(-pgid)` → await the
observation if not already fired → join pipe drains → call
`cmd.Wait()` exactly once, only now — so the leader's PID/PGID remains
reserved by the kernel throughout the entire signal-then-grace-then-
kill window, closing the reopened race; the escaped-session-descendant
residual is now stated honestly, and a cross-compile/source-shape test
confirms the build-tagged observer compiles for `linux`/`darwin` with
no external dependency (new `AC-106`). D9's supporting per-slug-lock
enumeration at §7.1's top level (a distinct list from the one D9's
rev-9 fix already reached) still omitted `trust-dolt` — fixed, and
row 86/`AC-52` now cover `add`/`remove`/`clear`/`trust-dolt` (task 2).
`adapter-missing` was reused across the add-time (exit 2) and
capture-time (exit 3) refusal slots — renamed the add-time occurrence
to `adapter-missing-at-add`, keeping `adapter-missing` exclusively for
capture; `path-outside-repo` was referenced but never defined as a
named exit-3 refusal — now defined as the pre-existing lexical
containment pre-filter's own refusal name and added to the exit-3
table (task 3). D5 and D10 each described a different private-copy
mode-creation sequence than the PRD's own §6.1/§7.1 — all four
locations are now unified on one sequence: add-time TOFU hashes the
opened descriptor directly (no copy file, zero processes); capture-
time creates the copy mode `0600`, streams/hashes/`Sync`s it, verifies
the digest, then hardens to `0500` via a descriptor-based `Fchmod`
(task 4). D3's Vector 3 declaration order was already correct
(`to, db_path, table, from, contract`, matching the PRD's now-fixed
§13.3) — verified unchanged, no ADR edit needed for D3 itself (task 5).
Stale surfaces — `C36`'s re-attribution, `C40`'s narrowed grounding,
and the Test Matrix rows mirroring `AC-85`/`AC-96`/`AC-97`/`AC-102` —
are corrected to match the companion PRD (task 6). See the companion
PRD's §0.1 Claims Audit (`C41`) for the new source-grounding row.

**Preserved across every review pass to date (rev-1 through rev-10,
plus the rev-3 citation addendum — ten review passes total, matching
the companion PRD's count)**: a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Rev-11 fold summary

The rev-10 adjudication (`bc313df`) found rev-10's own Darwin-facing
observer semantics over-claimed, plus six further consolidated
findings, framed as a bounded platform/state-machine fold that does
not reopen D1/D2's authority/scope decisions. Rev-10's non-reaping
`waitid`/`WNOWAIT` observer was described as detecting "the leader has
exited," but Darwin's `waitid` can return successfully for a merely-
**stopped** (not exited) child even when only `WEXITED` is requested —
a documented quirk Go's own standard library avoids by never using
`waitid` on Darwin at all (`$(go env GOROOT)/src/os/wait_waitid.go`,
`//go:build linux` only, citing issue #19314 verbatim; the companion
`wait_unimp.go` confirms Darwin's `blockUntilWaitable` is a no-op for
exactly this reason, new `C42`). D5 is corrected: the observer's
successful return is renamed a **leader event/cleanup trigger**, never
"leader has exited"/"proof of exit"; any successful Darwin return
(including a stopped child) is a fail-closed cleanup-sequence entry
only; the raw `waitid` call retries on `EINTR` (mirroring Go's own
linux-only `ignoringEINTR` pattern); terminal, non-`EINTR` observer
errors split into `ECHILD` (no `-pgid` signals, force-close owned pipe
read ends, refuse `adapter-process-observer-failed` exit 1, no cleanup
claim — process identity is no longer safely pinned) and any other
terminal errno (best-effort group signaling still runs, but
`cmd.Wait()` is never called, leaving the child unreaped, then the
same refusal) (task A). D5 also now tolerates `syscall.EPERM`
alongside `nil`/`syscall.ESRCH` at both `-pgid` group-signal steps
(Darwin can return `EPERM` when the group's sole remaining member is
an unreaped zombie); any other errno is named
`adapter-group-signal-failed` (exit 1), and cleanup continues through
the remaining steps regardless, with the first substantive error
reported (task B). D5's pipe-drain join, previously unconditional and
resting on the over-claimed premise that `-pgid` signaling confirms
every potential pipe-writer dead, is now **bounded**: a short explicit
read deadline follows the reap, force-closing both ends and refusing
`adapter-drain-timeout` (exit 3, publishes nothing, releases the
`flock`) if either drain has not completed; a non-`EOF` reader error at
any other point is itself a fourth cleanup-entry trigger and refuses
`adapter-output-read-failed` (exit 1); a `SetReadDeadline` failure
surfaces the same name (task C). The Test Matrix rows and ACs mirroring
`AC-85`/`AC-96`/`AC-97` are rewritten to match, and five new rows/ACs
are added (`AC-107`-`AC-111`) covering the Darwin `SIGSTOP` trigger,
`EPERM` tolerance, `EINTR` retry/terminal observer errors, non-`EOF`
reader errors, and the bounded drain-timeout case (task D). D5's
add-time TOFU description (and the companion PRD's `AC-20`) still
described a stale "resolve/open/stream-copy-while-hash into an
ephemeral private copy" mandate directly contradicting rev-10's own
fix that add-time TOFU hashes the opened descriptor directly with zero
copies and zero processes started — corrected to match (task E). D5's
executed-binary-binding claim that "the bytes hashed and the bytes
executed are provably the same file" over-claims, since `cmd.Start()`
opens the private copy by pathname, not via the already-verified
descriptor — corrected to the narrower, actual guarantee: the pinned
digest verifies the bytes **written** to the copy; execution is
pathname-based after the descriptor-scoped `Fchmod`; same-UID
replacement in that narrow window remains outside the threat model
(task F). Finally, the companion PRD's `--dry-run` self-contradiction
(claiming an orphan sweep runs, then denying it in the same paragraph)
and its stale "Ubuntu/macOS `amd64`" CI-architecture claim (corrected
to "Ubuntu `amd64`/macOS `arm64`," since `macos-latest` runners are
Apple Silicon) are fixed for consistency (task G). See the companion
PRD's §0.1 Claims Audit (`C42`) for the new source-grounding row.

**Preserved across every review pass to date (rev-1 through rev-11,
plus the rev-3 citation addendum — eleven review passes total,
matching the companion PRD's count)**: a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Rev-12 fold summary

The rev-11 adjudication (`c4c367c`) found seven consolidated findings
in rev-11's process-group termination design, framed as a bounded
process-finalizer fold that does not reopen D1/D2's authority/scope
decisions. (1) The terminal-observer-error branches (`ECHILD` and any
other errno) acted outside any single ownership guard, so an `ECHILD`
branch's own forced pipe-close could produce a reader error that
independently raced to start a second, `ECHILD`-ignorant cleanup
attempt and send a `-pgid` signal `ECHILD` explicitly forbids. D5 now
requires a single atomic ownership primitive attempted by all **five**
candidate trigger sources — leader event, terminal observer error
(both errno classes), reader error, cap, timeout — before any cleanup
action, plus a cleanup-initiated flag that filters every subsequent
owner-induced reader error as a join-completion signal only (task 1).
(2) Because ownership acquisition is not itself deterministic, a fixed
priority re-check (terminal observer error > reader error > cap >
timeout > benign leader event) now determines the reported
classification once ownership resolves, with a phase-order
(signal → reap → drain) tiebreak for benign entries and two worked
multi-error examples (task 2). (3) `ECHILD` now has its own separated
finalizer: no signal, no `cmd.Wait()`, force-close, bounded join,
refuse, release lock, with **zero** negative-PGID calls proven even
under an induced reader error (task 3). (4) Every other classification
now shares one bounded finalizer with a concrete **2-second Reap
deadline** (`adapter-reap-timeout` on expiry, no second `Wait()`,
disclosed abandoned-goroutine residual) followed by a concrete
**2-second Pipe-drain deadline**, strictly sequenced after the reap
step resolves (task 4). (5) Signal-delivery honesty is restated: no
claim that a `-pgid` signal reaches every group member or proves the
group empty; `EPERM` can mean a tolerated zombie or a genuinely
unsignalable descendant, and the bounded Reap deadline is what
prevents an unsignalable **leader** from hanging the invocation (task
5). (6) A parity sweep removed every remaining claim that `cmd.Wait()`
is the last event in the sequence (corrected order: signals → bounded
Wait/reap → bounded drain finalization) and fixed a PRD §6.1 residual
paragraph that misnamed the `Fchmod` step and overclaimed the
executed bytes were provably traced to one continuous chain (task 6).
D5's Test Matrix rows (151/152/165/166/178 and new rows for
`AC-112`-`AC-115`) and the companion PRD's §0.1 `C36` "Grounds" column
are updated to match. See the companion PRD's §0/§26 for the full
per-directive accounting.

**Preserved across every review pass to date (rev-1 through rev-12,
plus the rev-3 citation addendum — twelve review passes total,
matching the companion PRD's count)**: a separate `resources.json` per
feature, never inside the canonical patch or unapply/lifecycle state;
Dolt (or any external tool) is never an authority over tpatch state
and is not a build/runtime dependency; replay/backward-compatibility
is Git-only.

## Rev-13 fold summary

The rev-12 adjudication (`521091d`) found four narrow edge-contract
omissions in rev-12's process-finalizer mechanism, none reopening any
product, schema, privacy, or taxonomy decision. (1) `cmd.Start()`
-failure endpoint handling was unspecified: D5 now states both
`os.Pipe()` pairs are created, and both write ends assigned, before
`cmd.Start()` is ever invoked; the two drain goroutines launch only
once `Start()` returns successfully; on `Start()` failure, the adapter
closes all four parent-held endpoints directly and synchronously, in
a wholly separate, goroutine-free path from either finalizer — no
observer, no ownership CAS, no signal, no `cmd.Wait()` (task 1). (2)
Not every forced-close branch was proven to join: the
`SetReadDeadline`-failure branch now explicitly joins both drain
goroutines before returning or unlocking, matching the other two
forced-close branches; the `ECHILD` finalizer's join step is renamed a
join-only helper that never calls `SetReadDeadline` on already-closed
ends (task 2). (3) The reap-timeout residual named only the abandoned
`cmd.Wait()` goroutine: D5 now discloses up to **two** background
goroutines can remain outstanding — that one, plus, for a non-leader-
event classification, the still-blocked leader-event observer
goroutine — both reporting over capacity-one, non-blocking-send
channels so neither can ever block trying to report (task 3). (4) A
late `ECHILD` between the priority re-check and the first `-pgid`
signal was not excluded: D5 now adds a single, fixed **cutoff drain**
that re-checks the terminal-observer-error flag immediately before the
first signal, overriding to the `ECHILD` finalizer if it has since
become set, proven sufficient by a narrowly-scoped exclusive-waiter
invariant (only this parent can reap its own child, and `tpatch`
issues no reaping syscall before the cutoff) rather than a claim that
out-of-contract concurrent waiters are handled (task 4). D5's Test
Matrix grows from 184 to **189** rows; five new PRD `AC`s
(`AC-116`-`AC-120`) are added, none rewritten in place, none removed.
See the companion PRD's §0/§27 for the full per-directive accounting.

**Preserved across every review pass to date (rev-1 through rev-13,
plus the rev-3 citation addendum — thirteen review passes total,
matching the companion PRD's count)**: a separate `resources.json` per
feature, never inside the canonical patch or unapply/lifecycle state;
Dolt (or any external tool) is never an authority over tpatch state
and is not a build/runtime dependency; replay/backward-compatibility
is Git-only.

## Decision Drivers

- ADR-027 D1–D6's existing committed/local split and hard-failure
  redaction posture must extend cleanly to a new kind of captured
  content, not be reinterpreted, and must hold even for content that
  only ever exists ephemerally in memory within one command
  invocation — D3's exact language ("local private buffers may keep
  only the redacted or hashed form") forecloses any persistent raw
  local store, opt-in or not, and this ADR takes that literally: no
  raw bytes are ever written to a scratch file at all, only held
  in-process.
- Any claim about Dolt's CLI/SQL surface must be verified against the
  primary `dolthub/dolt` source at the pinned commit
  (`59fb843bf6a4b653d7c8b6d997a603b10cf279d9`), not invented, and every
  such claim must cite the exact source file/line this ADR relied on.
- Any claim about an existing safety/validation primitive
  (`EnsureSafeRepoPath`, `IsPathIgnored`, `EnsureLocalIgnoreContract`,
  `ExitCodeError` usage, `check-ignore`'s actual flag surface) must be
  verified against current source, not assumed from its name or from a
  prior revision's unverified claim.
- A tracked publication step must be a single atomic commit point,
  reproducible byte-for-byte from its own content (content-addressed),
  so a retry of unchanged content is provably idempotent rather than
  merely "probably fine."
- Every mutating verb (`add`/`remove`/`clear`/`capture`/`trust-dolt`/
  `record --resources`) must serialize against every other mutating
  verb for the same feature slug, not just `capture` against itself.
- A crash at any point in the local-scratch, lock, or tracked-
  publication pipeline must leave the system in a state a subsequent
  invocation can recover from without manual intervention, and any
  residual gap the Go standard library cannot close must be stated
  honestly rather than glossed over.

## Decision

### D1 — Scope & authority (reaffirmed, unchanged across every review pass to date)

Resources are declared and captured per-feature, in a manifest wholly
separate from the canonical patch and from `apply-recipe.json`/unapply
state. Dolt (or any external tool) is never an authority over tpatch's
own state and is not a build/runtime dependency of `tpatch` itself.
Replay/backward-compatibility remains Git-only.

### D2 — Closed resource-kind set: three kinds, `generic-command` removed (reaffirmed)

`ignored-file`, `git-metadata`, `adapter-snapshot` (Dolt only, one
capability, `diff-summary` — D5 below). No plugin mechanism, no
user-declarable external-command execution of any kind in v1. A future
ADR is required before any sandboxed/consented generic external-command
capability is added.

### D3 — Resource ID: canonical-JSON args encoding + golden vectors (reaffirmed algorithm; vectors 2/3 recomputed for `contract` replacing `binary_sha256`)

The canonicalization algorithm itself is unchanged from rev-1/rev-2
(keys sorted byte-ascending, minimal `{"k":"v",...}` escaping, no
`encoding/json.Marshal` HTML-escaping, UTF-8 required with no NFC/NFD
normalization, `NUL`/C0 control bytes rejected at `add` time — PRD
§13.1–§13.2 for the full grammar). `resource_id := "res_" +
lowercase-hex(SHA-256(feature + "\x00" + kind + "\x00" + selector +
"\x00" + adapter + "\x00" + capability + "\x00" + canonical_args))[:12]`.
Vectors 1 (`git-metadata`/`head`) and 4 (`ignored-file`) are byte-
identical to every prior revision: `res_acc91dc23a8b` and
`res_79f5ac5dca13`. Vectors 2 and 3 (`adapter-snapshot`) are
**recomputed a third time** (rev-7, D5's trust/identity split) — a
Dolt resource declaration's `args` no longer carries `binary_sha256`
at all (moved to the separate, identity-excluded `trust` field, D5
below); instead `args` gains a new `contract` key
(`"dolt-diff-summary-v1"` in v1), which **does** participate in
`resource_id`'s hash. Both recompute to the same value,
`res_4b62313b6cce` (superseding rev-6's `res_00189e66780a`), which
independently reconfirms order-independence of the `args`
canonicalization holds with `contract` present instead of
`binary_sha256` (PRD §0.3, §13.3 — golden vector table reproduced
there byte-identical to this decision).

**Resource-ID collision refusal, narrowed to distinct declarations**
(rev-6 introduced, rev-7 narrowed — D3/D4 split): `add` and every verb
that loads `resources.json` (`list`, `remove`, `clear`, `capture`,
`diff`, `record --resources`) recompute each entry's `resource_id`
from its own fields and compare it against the entry's own recorded
value. At `add`, an identical canonical payload at an existing ID is
idempotent (exit 0, no second entry written); a **different**
canonical payload at the same 12-hex `resource_id` is refused
`resource-id-collision` (exit 3), never overwritten — this outcome
now covers **only** the case of two independently-correct, distinct
declarations whose canonical payloads happen to collide. At load
time, an entry whose **own** recorded `resource_id` does not match its
own freshly-recomputed value from its own fields (only reachable via a
hand-edited/corrupted `resources.json` — a single entry's internal
self-inconsistency, not a collision between two entries) is a
separate, distinct outcome, `resources-file-corrupt` (exit 3) — rev-6
conflated both cases under one `resource-id-collision` label; rev-7
splits them because they have different causes and different operator
remedies (delete/rename one of two colliding declarations vs. repair
or regenerate a single corrupted entry). `resources.json` is loaded as
a map keyed by `resource_id`, not a bare array, making the
self-consistency check and the by-ID lookup the same pass. A test-only
stub `resource_id`-derivation function remains the collision-testing
seam, since a real `SHA-256` collision cannot be produced for a test
fixture.

| Vector | Inputs (`feature`, `kind`, `selector`, `adapter`, `capability`, `args`) | `resource_id` |
|---|---|---|
| 1 | `model-picker`, `git-metadata`, `head`, ``, ``, `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker`, `adapter-snapshot`, `dolt:diff-summary:users`, `dolt`, `diff-summary`, `{"contract":"dolt-diff-summary-v1","db_path":"data/dolt-db","table":"users","from":"main","to":"HEAD"}` (declared `contract, db_path, table, from, to` order) | `res_4b62313b6cce` |
| 3 | Same as Vector 2, `args` declared `to, db_path, table, from, contract` order | `res_4b62313b6cce` (**identical** — order-independence, reconfirmed with `contract` present instead of `binary_sha256`) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

### D4 — Full ADR-027 compliance: zero pre-scan persistence, no raw bytes ever written to disk, honest read/diff semantics (task 2, task 9)

Rev-2 removed the opt-in local raw-history persistence rev-1 had, but
still wrote ignored-file content and Dolt's captured stdout to a
scratch **file** before scanning/hashing it. The rev-2 adjudication
(citing ADR-027 D3's exact language directly, PRD C16) found this
still creates a window where an unredacted raw byte exists on disk,
even if briefly and even if later deleted — D3 says local buffers may
keep only the *redacted or hashed form*, which a pre-scan raw scratch
file, however transient, is not.

Rev-3 closes this gap completely: `ignored-file` content is read into
a bounded in-process `[]byte` buffer (bounded by the existing
per-file/per-directory size caps, PRD §5.2) and scanned/hashed
entirely in memory — it is never written to any file, scratch or
otherwise, at any point. Dolt's stdout/stderr are captured via
**caller-owned `os.Pipe()` pairs** assigned directly to
`cmd.Stdout`/`cmd.Stderr` (rev-9 design, kept unchanged through
rev-11, superseding an earlier `exec.Cmd.StdoutPipe()`/`StderrPipe()`
approach this paragraph historically described; see D5/§6.4 for the
full termination-protocol rationale — rev-5's original correction, an
unbounded `*bytes.Buffer` set directly on `Cmd.Stdout`/`Stderr` has no
way to refuse output past a cap short of reading it all first, still
holds), drained concurrently by two goroutines into a single
**shared** cap-plus-one memory budget — combined stdout+stderr bytes,
not two independent per-stream budgets — that is checked on every
read; on overflow, this design kills the child's entire process group
via the unified cleanup sequence (D5/§6.4: unconditional
`SIGTERM(-pgid)`, bounded grace, unconditional `SIGKILL(-pgid)`,
tolerating `ESRCH`, rather than signaling just the immediate child,
since Dolt may itself fork helper processes), continues
draining/discards the remainder to let the child exit cleanly, awaits
its exit exactly once via the same unified sequence, and refuses with
`resource-limit-exceeded` (exit 3) — never truncating stdout and
feeding a partial, possibly-invalid-JSON prefix to the parser. Only
stdout is ever handed to the JSON parser; stderr is captured
**within the same shared budget** purely for local diagnostics (never
tracked, D10) and is never consumed by the structural parser.
**only** on-disk scratch content that remains is Dolt's own ephemeral
`HOME`/`DOLT_ROOT_PATH` working directory, which Dolt itself may write
to as part of running the query (its own config/state, not the
captured diff content) — this is unavoidable since Dolt is an
external process, not something this ADR's own code writes, and it
never contains the tracked/scanned result data itself. That directory
is deleted (best-effort) at the end of every invocation, regardless of
`--dry-run`, success, or failure, exactly as in rev-2.

No tracked artifact ever contains raw bytes, raw stdout, or a
wall-clock timestamp (D10 below). `feature resource diff` **reads
current file content** through the same bounded in-memory scanner
`capture` uses to recompute `size_bytes`/`hash`/`file_count`/
`total_bytes`/`combined_hash` and compares that fresh result against
the last tracked batch's `result` — reporting metadata/hash/file-set-
level changes only, never a textual content diff (PRD §5.1, §2's
non-goal is about line-level diffing, not about whether content is
read at all — an earlier framing describing this as recomputing
metadata "without opening file content" contradicted its own
hash-recomputation requirement and is corrected here); raw content
diffing/versioning remains explicitly deferred to a future ADR that
would have to supersede `ADR-027`'s committed/local split, which this
ADR does not attempt. A directory `capture`/`diff` reads each matched
file **sequentially, one at a time** — not under a single atomic
filesystem-level snapshot — so a `combined_hash` can in principle
reflect a state that never existed as one consistent point-in-time
directory content if an external process mutates a later-scanned file
while an earlier one has already been read; this residual is stated
honestly (Negative Consequences below) rather than the "point-in-time
snapshot" language an earlier revision used, which overclaimed
atomicity this design does not actually provide. There is no
persistent local raw store of any kind (rev-2's ephemeral-scratch-
with-a-raw-file design is gone); what remains is: (a) in-memory
scanning that never touches disk for captured content, with an actual
cap-plus-one read enforcing size limits rather than a pre-read
`Stat().Size()` check that a concurrently-growing file could bypass,
and (b) orphan cleanup of Dolt's own scratch `HOME` directory and the
tracked-temp artifacts at the next invocation (PRD §7.1, §7.3). This
design is grounded directly in `ADR-027` D3's own binding language:
"Local private buffers may keep only the redacted or hashed form; this
ADR does not authorize a tpatch-managed raw transcript archive" (PRD
C16, `docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-170`)
— an in-memory-only design is the most literal possible compliance
with that sentence, stronger than rev-2's "ephemeral file, deleted
after."

### D5 — Dolt adapter protocol: mandatory `db_path`/`table`, exact `dolt_diff_summary` SQL, trust pin/identity split, private-copy execution, single-owner bounded process finalizer (task 4, task 8, task 12; rev-7 private-copy execution/`contract` enum/`trust-dolt`; rev-11 platform/state-machine fold; rev-12 finalizer/ownership fold; rev-13 edge-contract fold)

Rev-2's `dolt_diff_summary(from, to[, table])` design left `table`
optional, permitting a whole-database query whose PK-set-change
behavior silently omits affected tables rather than erroring (source-
confirmed asymmetry, PRD C20). Rev-3 makes `db_path` and `table`
**both mandatory** Dolt-selector fields:

- `db_path`: a repo-relative path (subject to the same D6 path gate as
  an `ignored-file` selector) used as the **child process's working
  directory** for the Dolt invocation. Rejects any value containing
  `..`, `NUL`/C0 control bytes, or a backslash (exit 2) before any
  path resolution is attempted.
- `table`: mandatory, so a PK-set-change between `from`/`to` on that
  table is a hard SQL error (`dolt-query-error`, exit 3) — never a
  silently omitted row — closing the ambiguity rev-2 left open (PRD
  C20, source-confirmed via the single-table call site's
  `shouldErrorOnPKChange=true` vs. the multi-table call site's silent
  handling).

The exact, sole argv template, invoked with `db_path` as `cmd.Dir`:

```
<resolvedDoltPath> sql -r json -q "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change FROM dolt_diff_summary('<esc(from)>', '<esc(to)>', '<esc(table)>') ORDER BY from_table_name, to_table_name;"
```

No other Dolt subcommand, flag combination, or argument count is ever
invoked for `diff-summary` — the flag-based `--name-only`/`--schema`/
`--data` templates from rev-1/rev-2 are entirely removed. `from`/`to`
reject the literal substring `".."` in addition to `NUL`/control/
backslash (exit 2), because `dolt_diff_summary`'s own argument
validation branches on whether the first argument contains `".."` to
decide between a dot-range and an explicit `from,to[,table]` form
(source-confirmed via `WithExpressions`, PRD C24) — rejecting `..`
outright means this design never depends on which branch Dolt's own
parser takes, and the dot-range form is never used or exercised by
this ADR. **`WORKING`/`STAGED` are explicitly refused in v1**
(rev-5 correction — rev-4 accepted them as ordinary `from`/`to`
values): `ResolveRootForRef` does resolve the exact-case string
constants `Working = "WORKING"`/`Staged = "STAGED"`
(`doltdb.go:51-52`, PRD C19, a true, unchanged source fact) in
addition to any ordinary commit-ish, but the working tree/staged index
they name is itself gated by Dolt's own `dolt_ignore` table — a
Dolt-level analog of `.gitignore` that can cause a table to be
silently absent from `WORKING`/`STAGED`'s row set the same way an
ignored file never appears in a Git working-tree listing. Because this
ADR already makes `table` mandatory specifically so a primary-key-set
change is a hard error rather than a silent omission (below), allowing
a ref value that reintroduces a *different*, independent
silent-omission path would undercut that guarantee for no
compensating benefit. This design therefore validates `from`/`to`
case-insensitively against `WORKING`/`STAGED` and refuses either with
`dolt-argument-refused` (exit 2) before Dolt is ever invoked — v1
accepts committed refs only (branch names, tags, full/abbreviated
commit hashes). A future ADR revision could accept these values if it
explicitly documents the `dolt_ignore` interaction rather than
silently inheriting it; this is out of scope, not merely deferred by
omission, for v1.

The five-column result schema is source-confirmed non-null and typed
(`from_table_name`/`to_table_name` `LongText`, `diff_type` `Text`,
`data_change`/`schema_change` `Boolean`), and the function itself
reports `IsReadOnly() == true` — reinforcing this decision's read-only
framing. The declared column *type* is one thing; the stronger claim
that the code path emitting each row actually produces a **native Go
`bool`** with no runtime coercion is separately evidenced by the row
constructor itself, `getRowFromSummary` (`dolt_diff_summary.go:457-464`),
which passes `TableDeltaSummary.DataChange`/`.SchemaChange` — both
declared `bool` (`table_deltas.go:83-90`) — directly into the output
row (PRD C25, a more precise citation than a schema-declaration
citation alone). `dolt sql -r json` output is parsed **structurally**,
not by exact byte-matching, with strict field presence/type checking:
the captured buffer's surrounding whitespace is trimmed first, and the
trimmed result is accepted as either the top-level object `{}` (zero
rows) or `{"rows":[...]}` (PRD C27 — the real, cited captured shapes
are `"...]}\n"` for nonempty results and `"{}\n\n"` for zero rows, per
`table/typed/json/writer.go`'s footer/`Close` behavior and
`engine/sql_print.go`'s unconditional trailing blank-line write, cited
here as **illustrative examples of the actual trailing-whitespace
shape** this design must tolerate, not as an exact-byte contract —
this design intentionally parses the *structure* after trimming,
rather than pattern-matching those specific literal byte sequences):
the trimmed `{}` maps to zero rows; a `{"rows":[...]}`
envelope with any other/extra top-level key, a row missing/duplicating
a field, or a non-boolean `data_change`/`schema_change` value is a
fatal `dolt-json-parse-error` — never silently coerced (PRD C22,
source-confirmed no `"schema"` key exists in this output format).
`diff_type` is tracked as one of the closed 4-value enum
`"added"`/`"modified"`/`"renamed"`/`"dropped"`, evidenced by the four
exact assignment lines inside `GetSummary` — the only places any row's
`DiffType` field is ever set: `table_deltas.go:722` (`Dropped`), `:733`
(`Added`), `:745` (`Renamed`), `:760` (`Modified`) — a stronger
citation than a `const`-block-only reference, since that same block
also declares a fifth constant, `DiffTypeAll = "all"`, that is never
assigned to any row's `DiffType` field anywhere in the file (confirmed
by an exhaustive grep) — it is a caller-side filter value only, never
a value this design's fixed query can emit (PRD C26). A rename
surfaces via `diff_type: "renamed"` with differing
`from_table_name`/`to_table_name` on that row. A nonexistent `table`
(in neither `from` nor `to`) yields `result.tables: []` (zero rows,
PRD C21) — distinct from the PK-change hard error above. Literal
escaping refuses `NUL`/C0 control bytes, backslash, and `..` outright
(exit 2) and escapes only `'` → `''` (PRD §6.2) — deliberately not a
general SQL-injection-safe escaper, since backslash's escape-character
status inside a Dolt/MySQL string literal depends on `sql_mode`, which
this ADR does not control (unchanged reasoning from rev-2).

**No version probe** (PRD C12, unchanged from rev-2): `dolt
version` is never invoked. Tool identity is `basename(resolvedPath)` +
`SHA-256` of the resolved binary's bytes (read, not executed) — the
absolute path is never tracked, only used in-process and never
persisted anywhere, tracked or local. The real `diff-summary` SQL
invocation is itself the only capability check; there is no separate
"probe" failure class. The invocation's environment contains only a
fresh, `0700` ephemeral `HOME` (and `DOLT_ROOT_PATH` pointing at the
same tree) — no inherited variable from the invoking process, no
`PATH`, no credentials of any kind (PRD §6.1).

**Binary trust pin, separated from resource identity, add-time TOFU
bootstrap vs. capture-time verified execution** (rev-6 introduced
pinning; rev-7 splits identity from trust and binds execution to
verified bytes; rev-8 splits the *bootstrap* from the *execution*
sequence and adds host-failure handling to the copy step): an
`adapter-snapshot`/Dolt resource declaration's `args` no longer carries
`binary_sha256` at all — it instead carries a mandatory `contract` key
(the single v1 value `"dolt-diff-summary-v1"`), which **does**
participate in `resource_id`'s hash (D3). The binary trust pin itself
moves to a separate, top-level `trust` field on the `resources.json`
entry (`{"binary_sha256": "<64hex>"}` or `null`), **excluded** from
`resource_id`'s hash — a legitimate Dolt binary upgrade re-pins trust
without destroying the resource's identity, `current.json` pointer, or
batch history, which rev-6's identity-entangled pin could not do.

Rev-8 makes explicit that there are **two distinct sequences**, never
conflated:

- **Add-time trust bootstrap (TOFU)**: `add --kind adapter-snapshot
  --adapter dolt ... --trust-current-dolt` runs only the shared
  resolution prefix (`LookPath`/`EvalSymlinks`/validate/in-repo-check)
  followed by opening the resolved binary and hashing its **opened
  descriptor directly** (`io.Copy` into a `SHA-256` hasher, equivalent
  to discarding the read bytes) — write `trust.binary_sha256` from
  that digest. This sequence creates **zero** private copies or
  scratch files, starts **zero** processes, and has **no
  existing-pin precondition** (there is, by definition, no pin yet the
  first time this runs) — it exists solely to compute and record the
  initial pin (rev-11: corrects a stale description of this step that
  contradicted rev-8's own already-correct capture-time/add-time
  split by describing a stream-copy-into-an-ephemeral-private-copy
  sequence at add time; PRD `AC-20` and `AC-102` are aligned on this
  same descriptor-hash-only wording). `add --kind adapter-snapshot
  --adapter dolt`
  **without** `--trust-current-dolt` is refused
  `dolt-trust-flag-required` (exit 2) — renamed rev-8 from the
  previously-overloaded `dolt-trust-required`, which shared one name
  across this add-time exit-2 case and the distinct capture-time
  exit-3 case below, violating the "every named refusal in exactly one
  row/table context" convention this ADR otherwise holds itself to.
- **Capture-time trust verification and execution**: a Dolt resource
  with `trust: null` is refused at capture time, `dolt-trust-required`
  (exit 3, now unambiguously capture-time-only) — before opening
  anything. Given a pin, this is the **only** sequence that ever opens
  and executes the Dolt binary, described in full below.

A duplicate `add` targeting an already-declared, identical resource —
whether or not `--trust-current-dolt` is re-passed, and regardless of
whether the currently-resolved binary's hash now differs from the
stored pin — is a **strict** no-op: `trust.binary_sha256` is left
byte-for-byte unchanged, and neither the copy nor any process-start
step runs. The `trust-dolt <slug> <resource-id> --binary-sha256
<64hex>` command remains the **only** way to re-pin trust after the
initial `add`, under the same per-slug `flock`, atomically rewriting
only `trust.binary_sha256` while leaving `resource_id`/
`current_batch_id`/history untouched — `trust-dolt` is added to every
"every mutating verb" enumeration alongside `add`/`remove`/`clear`/
`capture`/`record --resources` (D9), correcting a rev-7 omission.

Every `capture` now closes the swap-TOCTOU gap rev-6 left open (the
pre/post-invocation hashes were of the **resolved pathname**, re-read
independently each time, never bound to the exact bytes the child
process executed): the resolved Dolt binary is opened once. Before any
byte is copied, rev-8 adds a `statfs`-based no-exec preflight on the
private-copy scratch filesystem — Linux `ST_NOEXEC` / Darwin
`MNT_NOEXEC` set on the target filesystem's flags refuses
`adapter-copy-noexec` (exit 3) before the copy begins (mirroring D6's
`statfs` allow/deny mechanism, but checking the no-exec **flag**, not
the filesystem **type**, since a filesystem otherwise on the D6 allow
list can still be mounted `noexec`). The binary's bytes are then
streamed through a `SHA-256` digest (`io.TeeReader`) while being copied
into a private, per-invocation ephemeral scratch file created
`O_CREATE|O_EXCL|O_WRONLY, 0600` (rev-10, task 4 — corrects the prior
"created directly at mode `0500`" design: the copy must not be
executable at any point before its digest has actually been verified
against the pin, and the file is `Sync`ed once the streamed copy
completes). A write failure during this streamed copy with `ENOSPC` or
`EIO` is `adapter-copy-failed` (exit 1, distinct from the exit-3
policy-refusal family — this is a host I/O failure, not a policy
decision), with the partial copy file removed best-effort and no
process started. Once the copy (and `Sync`) completes, the digest is
compared against `trust.binary_sha256`; a mismatch is
`adapter-binary-untrusted` (exit 3), no process started, copy deleted.
Only **after** that comparison matches is the copy hardened to `0500`
via a **descriptor-based** `Fchmod` on the still-open file (rev-10,
task 4 — never a path-based `os.Chmod`, which would reopen a TOCTOU
window between the mode change and the subsequent open-for-exec), with
the resulting mode verified before the descriptor is closed. The child
process is then executed using the **private copy's own path**, never
the originally-resolved pathname — so, unlike rev-6's re-hash-the-
pathname-twice design, the digest that gated execution verifies the
bytes **written** to the private copy, and `Fchmod`+`close` happen on
that same still-open descriptor before any pathname-based `exec`
occurs (rev-11: corrects an over-claim that the hashed and executed
bytes are "provably" the same descriptor's content — `cmd.Start()`
necessarily opens the private copy **by pathname**, not via the
already-verified descriptor, since `os/exec` exposes no
descriptor-bound exec primitive; the actual, narrower guarantee is
that the pinned digest verifies what was written to that path, and a
same-UID replacement of the private-copy path in the brief window
between `close` and `cmd.Start()`'s own open remains outside this
design's threat model, disclosed honestly rather than claimed closed).
Executing a copy that physically resides inside
`.tpatch/local/` (rather than "outside the repo," D6's stated
rationale for the *resolved* binary) remains safe because the copy's
bytes are descriptor-bound during the hash-and-write phase,
hash-verified against the trust pin before it is ever made executable,
and owner-only (`0500`) once hardened —
the safety property comes from those facts, not from the copy's
filesystem location. The private copy is deleted after the child
exits (success or failure). The pin establishes *which* binary is
operator-approved to define Dolt's semantic contract for this
resource — it is **not** proof that binary matches any specific pinned
upstream source commit, only that it is byte-identical to what the
operator explicitly trusted; the new `contract` enum value
(`"dolt-diff-summary-v1"`) is the disclosed semantic-contract label,
and D5's strict five-field JSON parser remains the independent,
separate runtime capability gate on *what the binary actually
printed* — none of the three substitutes for either of the others.
`tool_identity.binary_sha256` in every tracked result remains always
identical to the declaration's pinned value (D10), never a
freshly-recomputed value presented as if it might differ. The add-time
TOFU bootstrap (`add --trust-current-dolt`) never reaches any of the
above: it opens the resolved binary and hashes the opened descriptor
directly (`io.Copy` into the digest, discarding the bytes) — no
private copy file is ever created under `es_<id>/`, and no process is
ever started (rev-10, task 4 — corrects a prior design that
unnecessarily created and then deleted an unexecuted private copy at
add time).

**Process-group termination — one unified sequence for every invocation,
using a leader event/cleanup-trigger observer instead of an early
`cmd.Wait()`** (rev-6 introduced `Setpgid`; rev-7 fixed the reap-timing
gap; rev-8 unified the two paths rev-7 left separate but gated the
whole sequence behind pipe drain; rev-9 decoupled leader-exit detection
from pipe drain via caller-owned `os.Pipe()`s but called `cmd.Wait()`
as soon as that detection fired, reaping the leader — releasing its
PID/PGID back to the kernel — *before* the group-signal sequence had
necessarily run, reopening a PGID-reuse race; rev-10 fixed this while
keeping the caller-owned pipes, introducing a non-reaping
`waitid`/`WNOWAIT` observer, but described its successful return as
"the leader has exited" — the rev-11 adjudication found this over-
claims on Darwin, where the same call can return for a **stopped**
(not exited) child even when only `WEXITED` is requested (Go's own
stdlib avoids `waitid` on Darwin for exactly this reason —
`$(go env GOROOT)/src/os/wait_waitid.go`, `//go:build linux` only,
issue #19314, PRD `C42`); rev-11 renamed the concept accordingly and
hardened several edges, but the rev-11 adjudication (`c4c367c`) found
a further compositional gap — the terminal-observer-error branches
acted outside any single ownership guard, an unbounded reap could hang
indefinitely on a live-but-unsignalable leader, and the trigger/error
precedence across all five sources was left undefined. **Rev-12
introduces a single-owner, five-source, deterministically-prioritized
finalizer while keeping rev-9/rev-10/rev-11's core ordering
guarantee** — the leader is never reaped except through the one
finalizer that runs after the process group has already been signaled
(or, for `ECHILD`, is deliberately never signaled at all)): before
`cmd.Start()`, the adapter sets `cmd.SysProcAttr =
&syscall.SysProcAttr{Setpgid: true}` (C31; `linux`/`darwin` only,
matching D9's build-tag contract), making the spawned child the leader
of a **new** process group distinct from `tpatch`'s own. **Pipe setup
(kept from rev-9/rev-10/rev-11)**: instead of `cmd.StdoutPipe()`/
`cmd.StderrPipe()`, the adapter creates two ordinary `os.Pipe()` pairs
itself, assigns each write end directly to `cmd.Stdout`/`cmd.Stderr`
(as `*os.File`) before `Start()` — **both pipe pairs, and both
endpoint assignments, exist in full before `cmd.Start()` is ever
called** (rev-13, directive 1) — and, immediately after `Start()`
**returns successfully**, closes its own reference to each write end.
**The two drain goroutines are launched only once `Start()` has
returned successfully** (rev-13, directive 1 — see the Start-failure
carve-out below for the distinct, goroutine-free path taken when it
has not); once launched, they read exclusively from the parent's own
read ends, and from that point on the finalizer (below) is the only
code that ever sets a deadline on, or closes, those read ends (PRD
`C40`). This claim is scoped to the post-`Start()`-success lifetime of
the invocation; the separate pre-`Start()`-success path below has no
finalizer, no goroutine, and no ownership CAS at all.

**Leader event/cleanup-trigger observer (rev-11, unchanged in
substance by rev-12)**: instead of running `cmd.Wait()` in a goroutine,
the adapter runs a build-tagged (`//go:build linux || darwin`)
observer goroutine wrapping the raw syscall `waitid(P_PID, leaderPID,
&buf, WEXITED|WNOWAIT)` in a loop that retries on `EINTR` (mirroring
Go's own linux-only `ignoringEINTR` helper, C42) — no
`golang.org/x/sys` dependency. `WNOWAIT` leaves the leader in a
waitable (zombie) state; this call, once it returns successfully,
**does not reap the leader**, so its PID/PGID is not released back to
the kernel's PID-reuse pool merely because this observer has fired. On
Linux a successful return is exit-only; on Darwin the same call may
return for a merely-**stopped** child despite `WEXITED` being the only
flag requested — a documented Darwin quirk Go's own stdlib avoids by
not using `waitid` there at all (C42). This design relabels the
semantic claim accordingly: the observer's successful return is termed
a **leader event**, a **cleanup trigger**, never "the leader has
exited" or "proof of exit"; any successful Darwin return — exit or a
stop — is treated identically as a fail-closed signal to enter
cleanup, safe because the unconditional `SIGTERM`→grace→`SIGKILL`
sequence below is correct whether the leader has exited or is merely
stopped. The observer reports "leader event observed" over a
**capacity-one, buffered channel with a non-blocking send** (rev-13,
directive 3 — see the reap-timeout residual below for why).

**Five candidate cleanup-trigger sources (rev-12, directives 1/2 —
supersedes rev-11's four-way race)**: the leader-event observer, a
terminal observer error (covering both `ECHILD` and any other terminal
errno — rev-12 folds these into the same race rather than treating
them as side paths, see below), a non-EOF pipe-reader-error signal
from the two independent pipe-draining goroutines, the
output-cap-exceeded signal, and the 30-second invocation timer.

**Single cleanup owner, acquired by every one of the five sources
(rev-12, directive 1 — closes a compositional gap the rev-11
adjudication found)**: rev-11 let the terminal-observer-error branches
act as soon as their own condition became true, without first
acquiring the same ownership primitive the other three (rev-11-era)
triggers shared — so an `ECHILD` branch's own forced pipe-close could
produce a reader-goroutine error that independently raced to start a
*second*, `ECHILD`-ignorant cleanup attempt and send a `-pgid` signal
that `ECHILD` explicitly forbids sending at all. Rev-12 closes this: a
single atomic ownership primitive (a `sync.Once`, or equivalently a
CAS on a per-invocation atomic flag) is attempted by **every** one of
the five sources, including both terminal-observer-error branches,
before any of them takes any cleanup action whatsoever — not merely
before signaling, but before even a single pipe `Close` or
`SetReadDeadline` call. Only the first source to successfully acquire
this primitive proceeds to run any cleanup; every later attempt, from
any source, is a no-op for ownership purposes. Terminal observer
errors are not a side path outside this guard: `ECHILD` and every
other terminal errno attempt the identical CAS as the other three
sources.

**Deterministic trigger priority (rev-12, directive 2)**: the race to
*acquire* ownership is not itself deterministic. To make the
*reported* trigger classification reproducible regardless, each of the
five sources also records its own occurrence independently and
non-blockingly (a small per-source atomic flag or single-slot buffered
channel) as part of, or immediately before, attempting the CAS. The
instant the CAS resolves, the owning goroutine synchronously
re-examines all five recorded-occurrence flags in one fixed priority
order — **terminal observer error (`ECHILD` or any other errno) >
genuine non-EOF pipe-reader error > output-cap exceeded > invocation
timeout > benign leader event** — and whichever of these is the
highest-priority flag observed set at that instant becomes the entry's
classification, regardless of which source's own CAS attempt actually
won the ownership race.

**Cutoff drain, closing the late-`ECHILD` gap (rev-13, directive 4,
new)**: the priority re-check above happens once, but the shared
bounded finalizer's own first `-pgid` signal does not fire in that
same instant — some scheduling delay separates classification
selection from the first `SIGTERM` `syscall.Kill(-pgid, ...)` call. If
a genuine `ECHILD` becomes true in that narrow window — strictly after
the initial re-check selected some other classification but strictly
before the first `-pgid` signal — sending that signal would violate
the same "never signal a possibly-recycled PID/PGID" rule the `ECHILD`
finalizer exists to uphold. To close this, the owning goroutine
performs exactly **one** additional deterministic drain of the
terminal-observer-error flag at a single, fixed **cutoff** instant:
immediately after the initial classification is selected, but strictly
before the shared bounded finalizer's first `-pgid` syscall. If
`ECHILD` is now found set, it **overrides** the initial classification:
the owning goroutine switches to, and runs, the `ECHILD` finalizer
instead — guaranteeing **zero** negative-PGID signals for the whole
invocation. If the cutoff drain finds `ECHILD` still unset, the
initial classification is final. **The exclusive-waiter invariant that
makes this sufficient, stated narrowly**: before this cutoff instant,
the raw `waitid(P_PID, leaderPID, ..., WEXITED|WNOWAIT)` call inside
the observer goroutine is the **sole** wait-family syscall this process
ever issues against this child — `cmd.Wait()` is never launched until
strictly after the shared bounded finalizer's signal phase (steps 1-3)
has fully completed, and no other goroutine in this design ever calls
any wait-family syscall against this child. Because only a process's
actual parent can ever reap a child via any wait-family syscall (a
POSIX guarantee this design relies on but does not itself enforce
against arbitrary code), and `tpatch` has not issued any reaping
syscall against this child by the cutoff instant, the child cannot
already have been reaped by anyone if `ECHILD` was not set at that
instant — which is exactly why a **late** `ECHILD` (one whose
condition becomes true only after cutoff, once signaling has already
begun) is impossible in a conforming implementation of this exact
contract. This guarantee is deliberately scoped to that
**exclusive-waiter invariant** alone: it is not a claim that this
design can detect or recover from an out-of-contract concurrent
waiter (some other goroutine, injected via a bug, independently
calling `syscall.Wait4` or an equivalent against the same PID) — such
a violation is outside this design's control and is not claimed to be
safely handled; the guarantee holds precisely because, and only
because, this design itself never introduces a second waiter. PRD
`AC-119` (new) verifies the race: publishing `ECHILD` between the
initial trigger snapshot and the cutoff drain wins, switching to the
`ECHILD` finalizer and sending zero `-pgid` signals. PRD `AC-120` (new)
separately verifies `cmd.Wait()` is never launched until strictly
after the signal phase has completed, for every classification that
runs the shared bounded finalizer.

**Cleanup-initiated flag, suppressing owner-induced reader errors
(rev-12, directive 1)**: the instant ownership is acquired — before
any `SetReadDeadline` call, before any `Close` call on either owned
pipe read end, before the `ECHILD` finalizer's force-close below — the
owner sets a single shared **cleanup-initiated** flag (a simple atomic
bool, visible to both drain goroutines). From that instant on, any
non-EOF read error either drain goroutine subsequently observes is
first checked against this flag: if already `true`, the error is
attributable to the owner's own action and is treated purely as a
**join-completion signal**, never resubmitted as a fresh reader-error
trigger. Only a reader error observed **before** the flag is set can
ever be classified as a genuine, trigger-eligible reader error.

**Primary-error selection (rev-12, directive 2)**: once the entry's
classification is fixed by the priority re-check, at most one reported
reason is **primary** for the whole invocation. If the classification
is non-benign (terminal observer error, genuine reader error, or
output-cap-exceeded), that classification's own named refusal is
primary, and any error surfacing later while the shared finalizer runs
— an untolerated group-signal errno, a reap-deadline expiry, a
drain-deadline expiry, an unrelated drain read error — is recorded
only as a local diagnostic. If the classification is instead
**benign** — the leader-event observer fired, or the invocation-timeout
timer fired with no higher-priority source also set at the re-check
instant — the **first** cleanup-phase failure encountered while
walking the shared finalizer's fixed phase order, **signal → reap →
drain**, is primary; if no phase fails, the child's own collected exit
status determines success or `dolt-query-error`. **Worked example
(PRD `AC-114`)**: an output-cap-exceeded invocation whose finalizer
also hits an untolerated group-signal errno and a drain timeout still
reports `resource-limit-exceeded` (exit 3) as the sole primary reason
— the non-benign classification outranks the phase-order rule
entirely. A benign leader-event entry whose finalizer's signal step
fails and whose drain also times out reports
`adapter-group-signal-failed` (exit 1) as primary — the signal phase
is earlier than the drain phase in the fixed walk order.

**Terminal observer errors, folded into the same ownership race
(rev-12, directive 1 — rewritten from rev-11; cutoff interaction
rev-13, directive 4)**: if the retried `waitid` call returns a
non-`EINTR` error, this is one of the five sources above — subject
also to the cutoff-drain override above, which lets a genuine `ECHILD`
that only becomes true after another classification's initial
re-check still preempt it, strictly before any `-pgid` signal is
sent. If it is the highest-priority one set at the re-check
instant, the entry's classification is "terminal observer error," and
which finalizer runs depends on the specific errno: (a) **`ECHILD`
specifically** runs the `ECHILD` finalizer below; (b) **any other
terminal errno** runs the shared bounded finalizer below, with
`adapter-process-observer-failed` (exit 1) as its primary reason — but
the shared finalizer's own bounded reap step is still attempted (a
live leader may still be genuinely running, and the bound is what
prevents indefinite blocking either way).

**`ECHILD` finalizer (rev-12, directive 3 — runs only under the
already-held owner)**: no `-pgid` signal of any kind is ever sent in
this branch — the kernel no longer considers this PID a child of this
process at all, so its numeric PID/PGID could, in principle, already
have been recycled to an unrelated process group — and `cmd.Wait()` is
never called. Sequence: (1) the cleanup-initiated flag is set; (2)
both owned pipe read ends are force-closed immediately; (3) both drain
goroutines are joined using a **join-only** helper (rev-13, directive
2, clarified — shares its 2-second ceiling with the shared finalizer's
own drain-join step but never itself calls `SetReadDeadline`, since
both read ends are already closed by step 2 and there is no open
descriptor to set a deadline on), as a defensive ceiling only; (4) the
invocation refuses `adapter-process-observer-failed` (exit 1) and
releases the per-slug `flock`. Any reader error observed during step 3
is, per the cleanup-initiated-flag rule, ignored as a trigger.
**Unsafe-identity residual, stated honestly**: because no signal is
ever sent and the leader is never reaped in this branch, both the
leader and any of its descendants remain fully alive, unsignaled, and
unreaped after this finalizer completes — a deliberate, disclosed
trade-off, never a claim that any cleanup occurred against the Dolt
process tree itself. PRD `AC-109` (extended) verifies this branch
sends **zero** negative-PGID signal calls even when the `ECHILD`
finalizer's own force-close induces an `os.ErrClosed`-shaped read error
on both drain goroutines.

**Shared bounded finalizer (rev-12, directive 4 — every other
classification: benign leader event, invocation timeout, output-cap
exceeded, genuine non-EOF reader error, and any non-`ECHILD` terminal
observer error)**: all five of these classifications, once ownership
is acquired, run through one identical finalizer sequence, differing
only in which reason (if any) is reported as primary: (1)
`syscall.Kill(-pgid, syscall.SIGTERM)`, tolerating `nil`/
`syscall.ESRCH`/`syscall.EPERM` (see signal-delivery honesty below),
recording any other errno as `adapter-group-signal-failed` without
halting the sequence; (2) a fixed 2-second grace period, during which
the leader remains unreaped; (3) `syscall.Kill(-pgid,
syscall.SIGKILL)`, the same tolerance/recording rule; (4) **bounded
reap**: launch `cmd.Wait()` in its own goroutine — reporting its own
completion over its own **capacity-one, buffered, non-blocking-send
channel** (rev-13, directive 3, symmetric with the observer's channel
above) — and wait at most a concrete, fixed **2-second Reap
deadline**. If `cmd.Wait()` returns
within the deadline, the leader is reaped, its real exit status is
collected, and the finalizer proceeds to step (5). If the deadline
elapses first, the finalizer records `adapter-reap-timeout` (exit 1,
primary only per the selection rule above) and proceeds **without** a
second `Wait()` call — the original goroutine is left running, still
blocked inside its own `cmd.Wait()` call, never joined or forcibly
stopped; the per-slug `flock` is not held any longer than this bound
requires. **Blocked-goroutines (plural)/live-child residual, stated
honestly (rev-12; extended rev-13, directive 3 — the residual is not
one goroutine but potentially two)**: when the Reap deadline elapses,
the leader is not proven dead, reaped, or even successfully signaled —
only that this invocation itself stops waiting on it. Two distinct
background goroutines can remain outstanding: (i) the abandoned
`cmd.Wait()` goroutine from this step, still blocked inside its own
call; and (ii), if this entry's own classification was not itself the
leader-event, the original leader-event observer goroutine may also
still be blocked inside its own retried raw `waitid` call, since its
own condition never independently became true before some other
source won the ownership race. Both persist for as long as the OS
keeps the leader unreaped, unbounded in the kernel-uninterruptible-
sleep case below; neither can ever block this or any future invocation
trying to report its own eventual completion, since each reports over
its own capacity-one, non-blocking-send channel (above) that this
invocation has already stopped listening on by the time it returns;
(5) **bounded pipe-drain
finalization, sequenced strictly after step (4)**: set a concrete,
fixed **2-second Pipe-drain deadline** via `SetReadDeadline` on both
owned `os.Pipe` read ends. If `SetReadDeadline` itself fails on either
end, both ends are closed immediately, **both drain goroutines are
joined via the same bounded-join helper before this branch returns or
releases the `flock`** (rev-13, directive 2 — every forced-close
branch in this design always joins both drain goroutines before
returning or unlocking; no forced-close branch ever returns early),
and the invocation refuses `adapter-output-read-failed` (exit 1). If both drains reach `io.EOF`
before the deadline, this is the common case. If the deadline elapses
first, both ends are force-closed (setting the cleanup-initiated flag
first), both goroutines are joined, and the invocation refuses
`adapter-drain-timeout` (exit 3), publishing nothing and releasing the
`flock`. **Explicit ordering, corrected (rev-12, directive 6 —
rev-10/rev-11 summary language claimed `cmd.Wait()` is the last event
in the sequence; this was never accurate once a bounded drain
finalization step exists)**: the normative sequence is **signals →
bounded Wait/reap (step 4) → bounded drain finalization (step 5)** —
`cmd.Wait()` is emphatically **not** the last event; bounded drain
finalization always follows it.

**Signal-delivery honesty (rev-11, extended rev-12)**: `nil`,
`syscall.ESRCH` (no signalable process group), and `syscall.EPERM`
(expected on Darwin when the group's sole remaining member is an
unreaped zombie; `EPERM` can also more generally represent an
unsignalable descendant, best-effort outside the same-UID
trusted-adapter threat model) are all **tolerated** at both `-pgid`
signal steps, never inferred as proof the group is empty or non-empty;
any other errno is recorded as `adapter-group-signal-failed` (exit 1)
but cleanup continues through the remaining steps regardless. No claim
anywhere states a `-pgid` signal reaches every group member or proves
the group empty — the bounded Reap deadline exists precisely so an
unsignalable **leader** cannot hang the invocation, while an
unsignalable **descendant** remains an accepted residual.

**Narrowed premise, stated honestly (rev-11, unchanged in substance by
rev-12)**: only the **leader** is ever proven reaped by this design
(the shared finalizer's single `cmd.Wait()` call, when it returns
within its own bound); same-PGID descendants are reached only by
**best-effort** `-pgid` signaling, and a descendant that has escaped
the process group or is otherwise unsignalable is not proven
terminated — this is exactly why the pipe-drain step is **bounded**
rather than assumed-safe. There is no "`Wait()`" — nor the
leader-event observer, nor the reap step's own successful return —
"proves group emptiness" claim anywhere in this design: the design's
actual, defensible safety property remains narrow and purely
sequential — **the unconditional `SIGTERM`→grace→`SIGKILL(-pgid)`
sequence always completes, targeting a PGID the kernel has not yet
been able to recycle, before the leader is ever reaped** (C36) — not a
claim that this sequence itself proves the group empty at any instant,
nor that a successful reap proves every group member has exited.
Signaling `-pgid` reaches only the Dolt child and any of its own
descendants that remain in the same group — it never reaches
`tpatch`'s own process, `tpatch`'s own process group, or a parent
shell. **Start failure, fully specified (rev-13, directive 1,
rewritten)**: both `os.Pipe()` pairs are created, and both write ends
assigned to `cmd.Stdout`/`cmd.Stderr`, before `cmd.Start()` is ever
invoked — so if `Start()` itself returns an error, the parent still
holds all **four** endpoints of its own two pipe pairs by itself (no
child was ever forked to inherit a duplicate of either write end). The
adapter closes all four of these parent-held endpoints directly and
synchronously, in the same goroutine that called `Start()` — no drain
goroutine is ever launched, no ownership CAS is ever attempted by
anything, no observer goroutine is ever started, no `-pgid` signal of
any kind is ever sent, and `cmd.Wait()` is never called for that
invocation, because no leader process exists at all. This is a wholly
separate, synchronous, goroutine-free code path — not a degenerate
case of either the shared bounded finalizer or the `ECHILD` finalizer
— that completes (four closes, then an error return) before any of
the five-source race machinery above is even reachable. PRD `AC-116`
(new) verifies zero goroutines, zero ownership-CAS attempts, zero
signals, and no `cmd.Wait()` call, with all four endpoints confirmed
closed.

**A disclosed trade-off, carried forward**: a fully successful
invocation whose leader exits with no lingering descendants still
incurs the (now-redundant, but harmless) `SIGTERM`/grace/`SIGKILL`
calls before the bounded reap and bounded drain — accepted as the cost
of one unified code path rather than a branch-specific fast path.
**Bounded lock-holding residual (rev-11, extended rev-12)**: the
per-slug `flock` is held for the duration of whichever bounded step is
currently running — the 2-second Reap deadline, then, if reached, the
2-second Pipe-drain deadline — in the `adapter-reap-timeout` and/or
`adapter-drain-timeout` cases; an accepted, strictly **bounded**
trade-off (at most roughly 4 additional seconds beyond the grace
period in the worst case), replacing an unbounded hang that could
otherwise have held the lock indefinitely. **Residuals, stated
honestly**: (a) a **new** process spawned by a group member *after*
`SIGKILL` was delivered but *before* the kernel has fully torn down the
group is a kernel-level race this design does not claim to close; (b)
a descendant that has **escaped the process group** no longer shares
the leader's PGID and is **not** reached by `-pgid` signaling at all,
and may itself be the process whose held pipe write end causes
`adapter-drain-timeout`; (c) a descendant (or, in the
`adapter-reap-timeout` case, the leader itself) in an OS-level
**kernel-uninterruptible** sleep cannot be terminated by `SIGKILL`
until it returns from that syscall — an honest, disclosed OS-level
limitation, not a gap this design claims to close, and not conflated
with the bounded, `tpatch`-controlled deadline residuals above.

Verification: this design's own tests cover all **five**
cleanup-trigger sources (leader event, terminal observer error — both
`ECHILD` and non-`ECHILD` — timeout, output-cap, reader-error) through
the appropriate one of the two finalizers (`ECHILD`-specific or shared
bounded), plus: a Darwin-only test double asserting a stopped-not-
exited child is still treated as a fail-closed cleanup trigger; a test
asserting `EPERM` from a group-signal call against a sole unreaped
zombie is tolerated identically to `ESRCH`; a test asserting the raw
`waitid` helper retries correctly on injected `EINTR`; a test asserting
a genuinely terminal, non-`ECHILD` observer error still runs the
shared bounded finalizer (including its own bounded reap attempt)
while reporting `adapter-process-observer-failed` as primary; a test
asserting `ECHILD` specifically sends **no** signals, force-closes the
owned read ends, joins via the bounded-ceiling helper, refuses without
any cleanup claim, and induces zero negative-PGID calls even under an
injected `os.ErrClosed`-shaped reader error; a test with a
successfully-exiting leader whose descendant ignores `SIGTERM` and
only closes its pipes once killed, asserting the leader-event fires
first, the unconditional signal sequence still runs and kills the
descendant, `cmd.Wait()` reaps the leader within the 2-second Reap
deadline, and the bounded drain finalization then completes normally,
strictly after the reap; a test asserting an escaped-session
descendant that keeps a pipe write end open past the bounded drain
deadline causes `adapter-drain-timeout` while the `tpatch` test-runner
process itself is never signaled or observably affected; a test
injecting a leader that never becomes waitable within the 2-second
Reap deadline, asserting `adapter-reap-timeout` is reported, the
per-slug `flock` is released promptly, and the abandoned `cmd.Wait()`
goroutine is disclosed as a residual rather than joined or cancelled; a
test asserting the deterministic priority re-check selects a terminal
observer error's classification over a simultaneously-recorded
reader-error/cap/timeout/leader-event occurrence regardless of which
source's own CAS attempt happens to win the ownership race; a test
reproducing the two worked multi-error examples above; a test
asserting a `cmd.Start()` failure closes exactly the four parent-held
pipe endpoints synchronously and spawns zero goroutines, attempts zero
ownership-CAS operations, sends zero signals, and never calls
`cmd.Wait()` (rev-13, PRD `AC-116`); a test asserting every
forced-close branch — `ECHILD`, drain-deadline expiry, and
`SetReadDeadline` failure — always joins both drain goroutines before
returning or unlocking, and confirming the `ECHILD` join-only helper
never calls `SetReadDeadline` (rev-13, PRD `AC-117`); a test asserting
that, on `adapter-reap-timeout` for a non-leader-event classification,
both the abandoned `cmd.Wait()` goroutine and the still-blocked
observer goroutine are disclosed, and that both goroutines' own
eventual completion sends succeed instantly with no receiver present
(rev-13, PRD `AC-118`); a race test asserting a genuine `ECHILD`
published strictly between the initial trigger snapshot and the
cutoff drain wins, switching to the `ECHILD` finalizer and sending
zero `-pgid` signals (rev-13, PRD `AC-119`); and a test asserting
`cmd.Wait()` is never launched until strictly after the signal phase
completes, for every classification running the shared bounded
finalizer (rev-13, PRD `AC-120`). A separate cross-compile/source-shape
test (PRD `AC-106`) confirms the build-tagged observer's source layout
compiles for both `linux`
(`amd64`/`arm64`) and `darwin` (`arm64`) with no external dependency.

### D6 — Executable and path safety: descriptor-identity gate for selectors, `db_path`/`cmd.Dir` hard refusal, opposite-direction policy for the Dolt binary (task 3; rev-6 hard refusal)

`safety.EnsureSafeRepoPath`/`store.NormalizeClaimPath` remain
lexical-only (PRD C2, unchanged citation, unchanged across all
revisions). Two separate policies for two separate trust directions,
both unchanged in structure from rev-3 except for one addition
(below):

- **Selector paths that must stay inside the repo** (`ignored-file`
  selectors, and now Dolt's `db_path`): `Lstat` every path component
  from the repo root down to the leaf; refuse
  (`symlink-component-refused`, exit 3) if **any** component —
  ancestor or final — is a symlink, regardless of where it points. The
  final open uses `syscall.O_NOFOLLOW` as a real, available hardening
  layer against a final-component race. A genuine descriptor-level
  check backs this: `os.SameFile(preOpenInfo, openFile.Stat())`,
  comparing the `FileInfo` obtained from the pre-open `Lstat` against
  the `FileInfo` obtained from `Stat()` on the **already-open file
  descriptor** — this is a real check against the actually-opened
  inode, not a second pathname lookup that could itself race a
  replacement between the two `Lstat` calls. A pathname re-`Lstat` is
  retained only as a secondary, defense-in-depth signal, not the
  primary check. This gate re-runs at both `add` and every `capture`,
  independently for every descendant file of a directory selector, and
  independently for `db_path`.
- **`db_path`/`cmd.Dir` honesty, hard refusal** (rev-6, supersedes
  rev-4/rev-5's diagnostic-only detection): unlike every other gated
  path, `db_path` is never opened and read by this process — it is
  handed to Dolt as a child process's working directory via Go's
  `os/exec.Cmd.Dir`, which is a plain pathname **string**, not a file
  descriptor. There is no portable stdlib mechanism (no
  `fdopendir`-plus-`fexecve`-shaped API exposed by `os/exec`) to bind a
  spawned child's cwd to an already-opened, already-validated
  directory descriptor, so the gap between this gate's last validation
  and the moment the child actually resolves its own cwd cannot be
  fully closed using only the Go standard library. **The check is
  pathname-vs-descriptor, not descriptor-vs-descriptor**: `fstat`ing
  an already-held open descriptor twice and comparing the two results
  is a tautology that always matches, regardless of what has happened
  to the *name* in the filesystem — the real question is always
  whether the pathname `db_path` still resolves to the exact directory
  this invocation has open, which requires a **fresh** pathname
  resolution each time. This design narrows, but does not eliminate,
  that window: the full gate re-runs a second time **immediately
  before `cmd.Start()`** — a fresh `Lstat` of the pathname component
  chain, not a reuse of the initial gate's cached `FileInfo` — and the
  result is compared (`os.SameFile`) against the directory descriptor
  already held open from the initial gate; that same descriptor is
  kept open for the entire lifetime of the Dolt child process; and
  after the child exits, `db_path`'s pathname is resolved **fresh a
  third time** and compared (`os.SameFile`) against the same held
  descriptor. **Rev-6 change**: any mismatch at either the
  pre-`cmd.Start()` or the post-exit recheck is now a **hard refusal**,
  `db-path-identity-changed` (exit 3) — the invocation's output is
  discarded and never written to a batch, rather than rev-4/rev-5's
  behavior of merely logging the mismatch as a local diagnostic while
  still publishing the batch. This is still **detection**, not
  **prevention**, of a swap that completes entirely *during* the child
  process's own execution window (between the pre-`cmd.Start()` check
  and the post-exit check) — a sufficiently well-timed local
  concurrent attacker who swaps `db_path`'s target and reverts it
  before the post-exit check runs remains a documented residual this
  design does not claim to close, since nothing here holds a
  descriptor across the child's own internal directory resolution.
  This narrower, honestly-scoped residual is documented in Negative
  Consequences below, not claimed as a closed sandbox.
- **Dolt executable paths** (must stay outside the repo, unchanged
  from rev-2/rev-3): symlinks ARE followed (`filepath.EvalSymlinks`);
  the *resolved* target must be a regular, executable file located
  outside the repository working tree and outside any `.git`
  directory — refused (`adapter-executable-in-repo`, exit 3) if not.
  Rev-7 supersedes rev-6's "re-hash the resolved pathname twice"
  approach: the resolved binary is opened once and its bytes are
  streamed into a private, per-invocation copy while hashing (D5) —
  the executed path is that private copy, never the originally-
  resolved pathname, closing the swap-TOCTOU window a pathname-only
  re-hash could not close. See D5 for the exact mechanism and the
  `adapter-binary-untrusted`/`dolt-trust-required` refusals.

**Residual honestly stated** (PRD C14): Go's standard library has no
portable equivalent of `openat2`/`RESOLVE_NO_SYMLINKS` that atomically
binds every ancestor directory component against a race between the
`Lstat` walk and the final open; `os.SameFile` on the open descriptor
closes the *file-identity* half of this race completely (it is
checking the real, already-open inode), but a sufficiently well-timed
attacker replacing an ancestor **directory** itself between the walk
and the open is still not fully closed using only the standard
library — and, as described above, `db_path` carries an **additional**
residual specific to `cmd.Dir` being pathname-bound rather than
descriptor-bound: a swap that completes entirely *during* the child's
own execution window (between the pre-`cmd.Start()` and post-exit
rechecks) remains a documented residual, not claimed closed. Rev-7's
private-copy execution (D5) closes the analogous **Dolt-binary**
swap-TOCTOU window completely — the hash that gated execution and the
bytes actually run are the same already-open descriptor's content, not
two independent pathname reads — but does **not** claim to close a
same-user local attacker with write access to the private copy's own
ephemeral scratch directory during the narrow window between its
creation and its execution; that narrower residual is stated
explicitly, not folded into the closed-TOCTOU claim. This ADR does not
claim an impossible sandbox guarantee — it documents all of these as
accepted, stated v1 residual risks.

### D7 — One tracked publication point per capture, content-addressed batch ID, presentation-drift-aware idempotency, unordered batch set (task 5, task 6, task 8; rev-6 file-wire-drift split, `current_batch_id` rename)

Rev-2 introduced the batch-then-pointer design but used a random
`crypto/rand` batch ID, meaning an idempotent retry of unchanged
content produced a *different* `batch_id` every time — rev-2's own
changelog claimed this was intentional ("fresh ID on every retry"),
which the rev-2 adjudication correctly identified as neither
idempotent nor a real transaction guarantee. Rev-3 replaced this with
a content-addressed ID, but its idempotency check compared the wrong
two byte sequences — the rev-3 adjudication found this bug and rev-4
fixed it; rev-6 further splits the idempotency comparison to
distinguish presentation drift from a genuine collision:

- **`batches/<batch_id>.json`** — one immutable file per **distinct
  batch content** (not one per invocation — see "batches are an
  unordered set," below), containing every resource result that
  invocation produced (PRD §12.3). `batch_id := "rb_" +
  lowercase-hex(SHA-256(CanonicalBatchJSON({"feature": feature,
  "results": sorted_by_resource_id})))` — the **full, untruncated**
  64-hex-character SHA-256 digest (rev-5 correction: rev-4 truncated
  this to `[:12]`, 48 bits, which is collision-prone for a scheme
  whose own collision handling is a fatal integrity error, not a
  display convenience; resource IDs, `res_` + 12 hex, are a completely
  separate, unaffected convention — they identify a *declared
  resource*, not a batch's content, and are not derived from a content
  hash at all). Written via temp-file-then-rename with an `fsync` of
  the file before rename and the containing directory after, using
  `O_EXCL` on the temp name. **Idempotency check, corrected then
  refined (rev-6)**: if `batches/<batch_id>.json` already exists on
  disk, this design re-encodes the **complete file-wire bytes** for
  the candidate batch — the full JSON object as it will actually be
  written, **including** the `batch_id` field and using the real
  on-disk indentation/newline convention — and compares *that* against
  the on-disk bytes, not the hash-input bytes used to derive
  `batch_id` in the first place (rev-3's bug: those two byte sequences
  can never be equal even for byte-for-byte identical semantic
  content, since the hash-input encoding omits `batch_id` and uses no
  indentation). **Identical file-wire bytes** skip the write step
  entirely (idempotent re-publish). **Different file-wire bytes**
  (rev-6, task 6) no longer means an immediate collision refusal — the
  design first decodes the on-disk file (an unparseable file, or one
  whose own internal `batch_id` field does not match the target
  filename, is `batch-file-corrupt`, exit 3, a distinct failure never
  routed through the comparison below), canonicalizes its `{feature,
  results}` body via the same `CanonicalBatchJSON` encoder (dropping
  `batch_id`, exactly as the fresh candidate's own hash-input
  computation does), and compares that **semantic body** against the
  candidate's own canonical hash-input bytes. Matching semantic bodies
  is **presentation drift** — the immutable file is not rewritten, and
  the invocation proceeds to the pointer step exactly as the
  byte-identical case does. Genuinely differing semantic bodies is the
  real, fatal case: a `SHA-256` collision on the hash-input encoding,
  refused `batch-id-collision` (exit 3), never silently overwritten.
- **`current.json`** — one atomically-rewritten pointer, a
  `current_batch_id` plus a sorted `[]{resource_id, batch_id}` array
  mapping every currently-declared resource to the batch holding its
  current result (PRD §12.4; rev-6 renames `latest_batch_id` to
  `current_batch_id` — see "batches are an unordered set" below for
  why "latest" is not a concept this design tracks). Also written via
  temp-file-then-rename with the same `fsync` discipline. **This
  rename is the single, atomic commit point of the entire capture** —
  nothing is visible to a reader until it succeeds.

`resources.json` (the declaration manifest) is explicitly **not** part
of this transaction — `add`/`remove`/`clear` only ever rewrite
`resources.json` and **never** touch `current.json` or any
`batches/<id>.json` file, under the same per-slug `flock` as `capture`
(D9). This is a **correction from rev-3**, in which `remove`/`clear`
pruned `current.json`'s live index — that design made `current.json`
writable by a third verb class, directly contradicting this decision's
own "single, atomic commit point" framing, since a resource's
`current.json` entry could then change outside of any `capture`/
`record --resources` invocation. Under rev-4 onward, a `current.json`
entry for a resource later removed from `resources.json` simply
becomes a harmless, permanent orphan — exactly like an orphaned
`batches/<id>.json` file below — never surfaced by `list` (which
iterates `resources.json`'s declared entries, never `current.json`'s
index directly) and never garbage-collected in v1.

**Crash windows** (rev-6 adds the first-publication row): before the
tracked tree's first-ever `MkdirAll`/parent-`fsync` completes (a
slug's first-ever `capture`/`record --resources`, D9's first-create
sequencing) — the retried invocation re-runs `MkdirAll` (idempotent)
and re-`fsync`s the parent chain before proceeding, since no partial
tracked file can exist yet; before the batch rename — nothing changed,
safe retry recomputes the identical content-addressed `batch_id` and
resumes via the idempotent-skip branch above; after the batch rename
but before the pointer rename — a permanently orphaned, harmless batch
file no reader ever surfaces (not garbage-collected in v1); during the
pointer's temp-write — orphaned `.tmp-current.json` swept at next
invocation, prior `current.json` remains authoritative; after the
pointer rename — fully committed. Every crash window is now
recoverable via a plain re-run with unchanged content, without any
dedicated recovery command, since the batch ID is reproducible rather
than random and the idempotency comparison is now correct.

**Batches are an unordered, content-addressed set — not a chronology**
(rev-5 clarification, rev-6 rename to `current_batch_id`): `batch_id`
names a **distinct content value**, not a position in a sequence. A
capture of content A, followed by a capture of content B, followed by
a re-capture of the original content A, produces exactly **two** batch
files (`rb_<hashA>`, `rb_<hashB>`) — the third invocation's identical
content re-derives `rb_<hashA>` and takes the idempotent-skip branch
above rather than creating a third file — while `current.json`'s
pointer moves A → B → A across the three captures, simply reusing the
already-existing batch `A`; this is not a "rewind" restoring a prior
state from a log, because there is no log. The set of files under
`batches/` is unordered with respect to time; `current.json` is the
**sole** authority for "what is current now" — its `current_batch_id`
field (renamed from `latest_batch_id`, which wrongly implied
chronological recency) names only *this file's own* provenance fact:
the `batch_id` the invocation that most recently rewrote `current.json`
staged, not a claim that it is newer than any batch referenced by the
per-resource array below. Event-level chronology (which capture
happened when, in what order) is explicitly out of scope for v1 and
deferred to a future revision, should it prove necessary.

### D8 — Ignored/tracked Git gates: fixed `check-ignore` invocation, literal pathspecs elsewhere, local-ignore reuse for every mutator (task 1, task 8)


Rev-2 invoked `check-ignore` with `--literal-pathspecs`; that flag does
not exist for `check-ignore` and causes a fatal exit 128
(`"pathspec magic not supported by this command: 'literal'"`, verified
empirically this revision, and supervisor-independently reconfirmed
with a second, non-colon example — `git --literal-pathspecs
check-ignore -q --no-index -- 'docs/*.md'` fails identically, showing
the fatal outcome does not depend on the argument looking like
pathspec magic). Rev-3 removes it: `check-ignore` is invoked as `git
check-ignore -q --no-index -- <pathname>`, matching the existing
`internal/gitutil/ignore.go:59-79` `IsPathIgnored` implementation
exactly. Because `--literal-pathspecs` cannot be used here, a selector
whose first byte is a literal `:` (which `check-ignore` would
otherwise parse as pathspec-magic, e.g. `:(glob)`/`:(literal)`/`:!`/`:^`
— all four verified empirically as fatal exit 128 for those forms, the
`:(literal)` case supervisor-independently reconfirmed) is instead
passed with a `./` prefix, which disarms the magic-colon parsing
without needing the missing flag and is confirmed safe (exit 0 or 1,
never fatal) for both `:(glob)` and `:(literal)` forms; `*`/`?`/`[]`
characters are inert for `check-ignore` specifically (verified
empirically — no glob expansion occurs) and require no such
workaround. `check-ignore` exit
0 = ignored, exit 1 = not ignored (refused), any other exit = fatal
Git error (refused, distinct reason, never treated as either "ignored"
or "not ignored"). `ls-files --error-unmatch` (for the tracked/
untracked gate) and every index-entry selector call **do** correctly
use `--literal-pathspecs` (that flag is valid there) — exit 0 = tracked
(a real, unambiguous match, refused: a tracked path can never be a
valid `ignored-file` selector), exit 1 = the expected, valid,
**untracked** outcome (`--error-unmatch` reports "did not match any
file(s) known to git" and this design treats that specific exit-1
shape as confirmation of untracked status, not an error), and any
exit greater than 1 is a fatal Git error (`git-ls-files-error`,
refused, distinct reason, never coerced into either the tracked or
untracked outcome). A selector is refused unless it is **both**
ignored (via `check-ignore` exit 0) **and** untracked (via
`ls-files --error-unmatch` exit 1), rechecked at `add` and every
`capture` — this per-selector gate (an `ignored-file` selector's own
ignored/tracked check) is unchanged from rev-7 and remains distinct
from the scratch-root gate described next. The ephemeral-scratch root
itself is verified via **two separate checks with two separate
targets** (rev-8 corrects rev-7's design, which reused this same
per-selector `ls-files --error-unmatch` gate — targeted at the
per-slug leaf — for the scratch root's untracked half too, silently
disagreeing with the scratch root's own fresh-clone worked example,
which already targeted the whole `.tpatch/local/` subtree for that
half):

1. **Ignored half — the per-slug leaf, existence-independent**: the
   **existing** `workflow.EnsureLocalIgnoreContract(repoRoot,
   resourceScratchRoot)` (PRD C13,
   `internal/workflow/session_ignore.go:138`, reused rather than
   reinvented — its own internal `check-ignore` call is the same
   deliberate pathname exception to the literal-pathspec rule
   described above) checks **only** the ignored half, against the
   exact per-slug leaf `.tpatch/local/resource-scratch/<slug>/`.
2. **Untracked half — the whole `.tpatch/local/` subtree** (rev-8):
   `git --literal-pathspecs ls-files -- .tpatch/local/` (no
   `--error-unmatch`) must report **empty stdout** — a tracked file
   anywhere under `.tpatch/local/`, for *any* slug, refuses every
   mutator, not only the one whose leaf happens to be tracked. A
   non-zero exit from this `git` invocation is always a fatal Git
   error, never interpreted as "untracked." This deliberately does
   **not** reuse the per-selector `ls-files --error-unmatch` gate
   above: that gate answers "is this one exact, already-fully-formed
   pathname tracked," a question naturally suited to
   `--error-unmatch`'s single-path exit-code/stderr contract; the
   scratch-root question is "is anything anywhere under this whole
   gitignored root tracked," which the plain, no-flag `ls-files` form
   with an empty-stdout convention answers directly and correctly
   regardless of whether the root itself currently exists on disk.

Either half failing is `local-root-not-ignored` (step 1) or
`local-path-tracked` (step 2) (exit 3), refused before any scratch
content, including the lock file itself, is created. **Every**
mutating verb — `add`, `remove`, `clear`, `trust-dolt`, `capture`, and
`record --resources` — runs this same two-part local-ignore-plus-
untracked gate before creating the per-slug `.lock` file for the first
time in an invocation (D9), not only `capture`/`record --resources` as
in earlier revisions (rev-8 adds `trust-dolt` to this list, correcting
a rev-7 omission): `remove`/`clear`/`trust-dolt` still only ever
rewrite `resources.json`, but they now acquire the same lock as every
other mutator (D9) and must clear the same root gate first, so a
misconfigured or tracked scratch root is refused symmetrically for
every verb rather than only the content-producing ones.

### D9 — Lock semantics: kernel-released nonblocking advisory `flock`, no owner metadata, serializes every mutator (task 1)

Rev-3's temp-directory-then-atomic-rename lock (owner JSON with
`{pid, process_start, host}`, `ps -o lstart=` liveness probing,
quarantine-and-retry reclaim) closed rev-2's partial-observation
window, but the rev-3 adjudication found the *reclaim* protocol itself
still ABA-prone: a quarantine-then-retry sequence has its own
observable intermediate states, and `ps -o lstart=` is a fragile,
shell-out-based liveness signal whose output format is not portable
and whose parsing this design had to hand-roll. Rev-4 removes the
entire ownership-verification problem class rather than hardening it
further, replacing it with a kernel-released nonblocking advisory
lock:

1. Open a single, fixed, **persistent** lock file,
   `.tpatch/local/resource-scratch/<slug>/.lock`
   (`O_CREATE|O_RDWR`, `0600` at creation — never `chmod`'d
   afterward). This file has **no body** — it is never written to,
   read from, or parsed; it exists purely as a `flock`-able handle.
2. Call `syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)` on that
   file descriptor. Success means this invocation now holds the lock
   for as long as the descriptor stays open; the kernel itself
   guarantees the lock is released the instant the holding process
   exits or crashes for **any** reason (process termination, `SIGKILL`,
   panic) — there is no owner metadata to go stale, no PID to reuse,
   and no liveness probe to run, because the kernel *is* the liveness
   check.
3. On `EWOULDBLOCK`/`EAGAIN` (another live process already holds the
   lock): refuse immediately with `capture-in-progress` (exit 3) — no
   blocking wait, no timeout flag, no retry, in v1. There is nothing to
   quarantine or reclaim: if the holder crashed, the kernel has
   *already* released the lock, so a nonblocking attempt would succeed
   rather than observe a stale artifact.
4. **The lock file itself is never removed or renamed.** Rev-3's
   design removed its lock directory on release; rev-4 does not,
   because unlinking a lock file while another process might hold an
   open descriptor to the old inode is the classic unlink/recreate
   (ABA) race this rev-4 change is specifically intended to avoid —
   the fixed lock file persists indefinitely as an ordinary, empty,
   locally-ignored control file, and its presence on disk carries no
   meaning (only an active `flock` on it does).

**Platform contract**: `flock(2)` is a POSIX/BSD primitive with no
Windows equivalent exposed identically by `syscall`. This design uses
an exact Go build-tag split — **not** the broader `unix` tag rev-4
used, which also matches untested POSIX-family targets (AIX, Solaris,
illumos, `js/wasm`'s POSIX-like build) this project neither builds nor
tests, and for which `syscall.Flock` availability/semantics are not
independently verified here: a real implementation tagged
`//go:build linux || darwin` providing the `flock`-based lock
described above, and a fallback build tagged
`//go:build !linux && !darwin` whose lock acquisition unconditionally
returns `resource-lock-unsupported` (exit 3) without touching the
filesystem — consistent with this project's actual, tested CI matrix
(`.github/workflows/ci.yml:18-25` runs `test (${{ matrix.os }})` over
exactly `os: [ubuntu-latest, macos-latest]`, with no third OS runner),
matching the existing `ADR-004-m10-copilot-proxy-ux.md` D6 precedent
("Windows: not supported in M10"), now made an explicit, source-visible
build contract tied one-to-one to the CI matrix rather than to the
broader `unix` convenience tag. Windows and every other non-Linux/
non-macOS host are therefore explicitly unsupported and deferred for
resource capture in v1 — this is a hard refusal grounded
in the hosts this project actually builds and tests, not a portable-
lock design in disguise.

**Filesystem contract** (rev-5 addition; stdlib-only API and exact
constants corrected rev-6, task 7): a successful `flock(2)` only
guarantees exclusion **on filesystems that implement genuine kernel-
level advisory locking** — on network/shared filesystems (NFS, CIFS/
SMB, FUSE-backed mounts, and similar), `flock` semantics are
frequently emulated, partial, or silently no-op depending on kernel
version, mount options, and server support, which would silently
degrade this design's entire serialization guarantee (D9's core
promise) without any visible failure. Rather than claim an
unconditional cross-filesystem `flock` guarantee, this design adds a
`statfs`-based preflight, performed against the **nearest existing
ancestor** of `.tpatch/local/resource-scratch/<slug>/` (D9's
first-create sequencing below — the directory itself may not exist
yet on a fresh clone) immediately before the lock file is first
opened, that classifies the backing filesystem and fails closed
(`resource-lock-filesystem-unsupported`, exit 3) unless the filesystem
is on an explicit per-OS allowlist, using **stdlib-only**
`syscall.Statfs` (rev-6 — no `golang.org/x/sys/unix` import, matching
this project's minimal-external-dependency rule):

- **Linux** (`//go:build linux`, `syscall.Statfs(path, &buf)`,
  `buf.Type` architecture-dependent width/signedness — `int64` on
  `amd64`/`arm64`, `int32` on `386`/`arm`, `uint32` on `s390x`, PRD
  C35): rev-7 normalizes `fsType := uint32(buf.Type)` before every
  comparison, with the allow/deny constants themselves typed `uint32`
  — an unnormalized raw comparison against a signed/narrower `buf.Type`
  is architecture-fragile (a magic number that fits `uint32` can
  compare unequal to itself across sign-extension/truncation depending
  on the build architecture); this normalization is exercised by a
  unit-test seam covering `linux/amd64`, `linux/386`, `linux/arm`, and
  `linux/s390x`. Allowed —

  | Filesystem | Magic (hex) |
  |---|---|
  | ext2/ext3/ext4 | `0xEF53` |
  | XFS | `0x58465342` |
  | Btrfs | `0x9123683E` |
  | tmpfs | `0x01021994` |
  | overlayfs (rev-6: newly allowed — the default Docker/Podman/most-container-runtime storage driver, common under this project's own CI) | `0x794C7630` |

  denied (explicit, fail-closed) —

  | Filesystem | Magic (hex) |
  |---|---|
  | NFS | `0x6969` |
  | CIFS | `0xFF534D42` |
  | SMB2 | `0xFE534D42` |
  | FUSE | `0x65735546` |

  any other, unrecognized magic number is **also** denied (fail-closed
  by default, not fail-open). Rev-6 removes rev-5's invalid
  `APFS_SUPER_MAGIC`-on-Linux entry entirely — no such Linux kernel
  constant exists; APFS reached from Linux at all (e.g. via a FUSE
  driver) surfaces under the FUSE magic number above, already denied.
- **macOS** (`//go:build darwin`, `syscall.Statfs(path, &buf)`,
  `buf.Fstypename` a fixed-size `[16]int8` NUL-terminated byte array):
  allowed — `"apfs"`, `"hfs"`, `"tmpfs"` (rev-6: `tmpfs` newly
  allowed); denied — `"nfs"`, `"smbfs"`, `"webdav"`, `"osxfuse"`,
  `"macfuse"` (rev-6: named FUSE variants added explicitly); any
  other, unrecognized name is likewise denied by default.

**Why allow container-overlay but deny network filesystems** (rev-6):
`overlayfs` behaves as a genuinely local advisory lock (the lower/
upper layers are themselves local in every configuration this design
targets) and refusing it would make this design unusable in the
container-backed CI environments this project's own tested matrix
actually runs in. Network filesystems are denied because `flock`'s
cross-client exclusion guarantee on them is exactly the historically-
inconsistent case this preflight exists to guard against — this is
"genuinely single-host local" vs. "may silently span multiple hosts,"
not a performance distinction.

This allowlist is intentionally narrow and may prove too brittle for
some legitimate local setups (e.g. an uncommon but genuinely local
filesystem type); if so, a future revision should replace it with an
explicit, documented **operator-configured local-only precondition**
("resource capture requires a genuinely local filesystem for
`.tpatch/local/`, and refuses to run otherwise, with this flag/config
key to override after the operator has verified that locally") rather
than silently expanding the allowlist without evidence. This preflight
makes **no** claim about cross-client or cross-host serialization even
on an allowed local filesystem — `flock` on a genuinely local
filesystem serializes concurrent *processes on that one host*, nothing
more.

**First-create sequencing, leaf-targeted ignore gate + subtree-targeted
untracked gate vs. ancestor-targeted `statfs`** (rev-6 introduced;
rev-7 split ignore/untracked from `statfs`; rev-8 further splits the
ignore half's target from the untracked half's target — task 1, task
7): on a fresh clone or a slug's first-ever mutating invocation,
`.tpatch/local/resource-scratch/<slug>/` (and, independently, the
tracked `artifacts/resource-captures/` tree) may not exist yet. Rev-6
ran **both** the local-ignore/untracked gate (D8) and the `statfs`
preflight against the nearest existing ancestor, which is correct for
`statfs` (a genuinely existence-bound kernel call — there is nothing
to `statfs` for a path that does not exist) but wrong for the ignore/
untracked gate: `IsPathIgnored`/`check-ignore` and the `ls-files`
untracked check are **pathname** checks, existence-independent, and
can and must be evaluated directly against the actual, intended
(possibly not-yet-existing) target — running them against whatever
ancestor happens to exist establishes nothing about whether the
specific target about to receive untracked, ignored content is itself
correctly ignored and untracked. Rev-7 ran both the ignore half and
the untracked half against the intended per-slug **leaf** directly;
rev-8 corrects this further — the ignore half continues to target the
per-slug leaf (D8 step 1, unchanged from rev-7), but the untracked
half now targets the **whole** `.tpatch/local/` subtree (D8 step 2),
since that check answers a subtree-scoped question ("is anything
tracked anywhere under this gitignored root") that the per-slug leaf
does not fully capture — a tracked file under a *different* slug's
scratch tree is just as much a privacy-boundary violation. Only the
`statfs` preflight remains targeted at the nearest existing ancestor,
before `MkdirAll` creates the tree (mode `0700` throughout, D10).
After `MkdirAll`, rev-7 adds **unconditional retry-fsync**: every
directory in the relevant chain is `fsync`'d, not only directories
`MkdirAll` itself reports as newly created — a directory can become
visible to `Stat`/`Lstat` before the kernel has made its creation
durable across a crash, so a retried invocation that only re-`fsync`s
directories it perceives as "new" could still lose an earlier,
not-yet-durable creation across a second crash between the first
attempt and the retry. This same unconditional retry-fsync discipline
applies identically to the tracked
`artifacts/resource-captures/`/`batches/` tree's own first-ever
creation (D7's crash-window table includes the corresponding
first-publication and retry rows).

**Every mutating verb serializes on this lock** (unchanged intent from
rev-3, mechanism replaced): `add`, `remove`, `clear`, `capture`,
`trust-dolt` (rev-7, new), and `record --resources` all acquire the
same per-slug `flock` before touching
`resources.json`/`current.json`/`batches/`. `remove`/`clear`/
`trust-dolt` acquire and release the lock around a simple, fast
manifest rewrite and never sweep orphan scratch (only
`capture`/`record --resources` do that, D4/D7). `list`/`diff` never
acquire the lock; they always observe either the fully-prior or
fully-new `resources.json`/`current.json` content (guaranteed by D7's
atomic-rename publication), never a torn read — reads proceed even
while another invocation holds the lock.

### D10 — Permissions and no tracked timestamps (task 8)

Ephemeral scratch directories (including Dolt's own `HOME`) are
created `0700` and files `0600` at creation (never via a separate
`chmod` after a looser-permission create). The per-slug `.lock` file
(D9) is likewise created `0600` at open time and never `chmod`'d —
and, unlike other scratch content, is never removed, since removing it
would reintroduce the ABA race D9 exists to avoid. The private,
per-invocation Dolt-binary copy (D5) is the one deliberate exception to
the `0600` file default: it is created `O_CREATE|O_EXCL|O_WRONLY`
`0600` (rev-10, task 4 — corrects the prior "created `0700`... then
hardened to `0500`" design, which was itself never consistent with the
PRD's own then-current "created directly at mode `0500`" text; both
are replaced by this single sequence), streamed/hashed/`Sync`ed, and —
only **after** its digest has been confirmed to match the trust pin —
hardened to `0500` via a **descriptor-based** `Fchmod` on the still-open
file, never a path-based `os.Chmod` (which would reopen a TOCTOU window
between the mode change and the subsequent open-for-exec) — a
narrower, execute-permission-bearing mode reflecting that this file,
uniquely among scratch content, is meant to be executed, not merely
read or written, and is never executable before its bytes have already
been verified. Tracked artifacts
(`resources.json`, `batches/<id>.json`, `current.json`) use ordinary
repository file permissions (`0644`), since they never contain raw
bytes or secrets by construction. No tracked artifact contains a
wall-clock timestamp field anywhere; `current.json`'s pointer conveys
only "what is current now," not "what happened when" — batch files
themselves are an **unordered, content-addressed set** (D7), not an
append-only log, so no ordering claim is made or implied by their
presence on disk. Failure diagnostics on a failed capture are printed
directly to the CLI's own output only — since there is no persistent
local raw-content scratch tree (D4), there is nothing left for a
"written inside scratch" diagnostic to be.

### D11 — Wire canonicalization: content-addressed batch ID via `CanonicalBatchJSON`, corrected idempotency comparison, `record --resources` two-domain framing retained (task 5, task 11)

Every tracked JSON field that would otherwise be a Go `map` (`args`,
the per-resource index in `current.json`) is a sorted `[]struct` array
(`[]{key, value}` / `[]{resource_id, batch_id}`), so no tracked
artifact's determinism depends on `encoding/json`'s map-key-sort
behavior (unchanged since rev-2). A new, distinct canonical encoder,
`CanonicalBatchJSON` (separate from resource-ID's `CanonicalArgsJSON`),
recursively encodes a batch's `{feature, results}` body (excluding
`batch_id`, compact encoding) — strings, booleans, `null`,
non-negative integers, arrays (each with an explicit, documented sort
rule), and fixed-field objects in struct-declared field order (never a
Go `map`) — and is the sole input to D7's content-addressed
`batch_id`. The **file-wire** encoding of the same batch (used for the
actual on-disk write and for D7's corrected idempotency comparison)
additionally includes `batch_id` itself and uses the real on-disk
indentation/newline convention — these are two distinct byte
sequences derived from the same logical content, and D7's idempotency
fix consists precisely of comparing like-for-like (file-wire vs.
file-wire) rather than rev-3's hash-input-vs-file-wire mismatch.
`record --resources` retains the two-atomic-domain framing from
rev-1 through rev-3 (Git-side capture and resource-domain publication
never commit together): staging remains purely in-memory/ephemeral
(never writes a batch file until Git succeeds); on Git success,
staging's already-computed candidate content is published through the
exact D7 batch-then-pointer sequence (which is itself now correctly
idempotent under retry, D7); on Git failure, the candidate is simply
discarded. A `resource-domain-incomplete` (exit 1) partial result
carries an explicit, idempotent recovery command
(`tpatch feature resource capture <slug>`), which — because the batch
ID is content-addressed and the idempotency comparison is now correct
— is safely re-runnable any number of times without producing
divergent batch history.

## Wire Schema Appendix

Byte-identical to the companion PRD's §12.1, §12.3, §12.4, §12.6
(verified programmatically, not just visually).

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

(rev-7: `trust` is new — `null` for every kind except
`adapter-snapshot`/`dolt`, where it holds the mutable trust pin
`{"binary_sha256": "<64hex>"}`, written by `add --trust-current-dolt`
or updated later by `trust-dolt`, and **excluded** from `resource_id`'s
canonical hash input, D3/D5 — unlike every `args` entry, which does
participate; `args` no longer carries `binary_sha256`, replaced by the
identity-participating `contract` key.)

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

`results` is sorted by `resource_id`, byte-ascending —
`res_4b62313b6cce` < `res_79f5ac5dca13` < `res_acc91dc23a8b`
(unchanged sort order across rev-6→rev-7's `resource_id`
recomputation — `contract` replacing `binary_sha256` in `args` changed
the digest's exact byte value but not its position relative to the
other two; D3's golden vectors and this ordering were recomputed
together for rev-7). `tool_identity.binary_sha256` above is always
identical to the declaration's `trust.binary_sha256` value — **not**
an `args` entry (rev-7: moved out of `args` into the separate `trust`
field) — never a freshly-recomputed value that could silently differ
from the pin.

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

(rev-6 renames `latest_batch_id` to `current_batch_id`, D7; rev-7
recomputes both IDs above for the `contract`/`trust` split, D3/D5 —
the field name and shape are unchanged from rev-6.)

**`trust-dolt` update wire** (rev-7, task 4/10, byte-identical to PRD
§12.6): `trust-dolt <slug> <resource-id> --binary-sha256 <64hex>`
rewrites only the target entry's `trust.binary_sha256` field, in
place, under the same per-slug `flock` — `resource_id`, `args`,
`current_batch_id`, and every batch/history file are untouched.
Before:

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
by this command.

**Directory `ignored-file` result, `combined_hash` tuple encoding**
(rev-6, task 9, mirrors PRD §5.1/§12.2): each matched file contributes
one tuple — `path` (repo-relative) + `0x00` + `mode` (the same octal
string as `index-entry`'s `mode`) + `0x00` + the file's own **raw,
unprefixed 64-lowercase-hex** `SHA-256` digest (distinct from the
wire-level `"sha256:"`-prefixed `raw_sha256` field in `files[]` below)
+ `0x00` — each field individually `0x00`-terminated, files'
contributions concatenated directly with no further separator (paths/
modes/hashes cannot themselves contain a `NUL` byte, so each field's
own trailing terminator already delimits it unambiguously).
`combined_hash` is `SHA-256` over the concatenation of every matched
file's tuple, sorted by `path`. A worked golden vector, computed and
independently reconfirmed via a standalone script during this
revision's validation pass — two files, `config/a.txt` (mode
`100644`, empty content, hash
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`)
and `config/sub/b.sh` (mode `100755`, content `#!/bin/sh\necho hi\n`,
hash
`299001868fb8c02fd431c336c6d058f5558c5dff5b5af5e6fe04b870a6a9cbba`) —
yields `combined_hash =
5af4d6754656795b49c6e22acc2034ed6a2b3426470b0c42156f5ad0b4bcb9ad`.
This encoding is unaffected by rev-7's Dolt trust/identity split.

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

## Implementation Notes

1. `internal/redact.Scan([]byte) []string` is a new exported package;
   the existing `internal/cli/session_redaction.go` matchers for
   bearer/token/key prefixes and absolute-home-paths are extracted
   into it, not duplicated, and is invoked directly against in-memory
   buffers (D4) — it never receives a file path.
2. The per-slug lock (D9) is implemented via a `//go:build linux ||
   darwin` file wrapping `syscall.Flock` plus a `statfs`-based
   filesystem preflight, and a `//go:build !linux && !darwin` fallback
   build tag returning `resource-lock-unsupported`; no `ps`/PID/
   process-start probing of any kind is invoked anywhere in this
   design — the rev-3 `ps -o lstart=` shell-out is removed entirely,
   not merely replaced by a different shell-out.
3. The `dolt_diff_summary` query is built via `fmt.Sprintf` over the
   already-validated/escaped `from`/`to`/`table` strings — validation
   (D5) happens strictly before string construction, never after, and
   `table`/`db_path` are mandatory, never optional, fields on the Go
   struct representing a Dolt resource declaration.
4. `es_<id>` ephemeral-scratch-subdirectory suffixes reuse the
   existing `crypto/rand`, 12-lowercase-hex-character ID convention
   already established elsewhere in this project (`ua_` attempt IDs);
   `rb_<id>` is the one exception — it is content-addressed
   (`SHA-256`-derived, D7), not `crypto/rand`-derived, and this
   distinction is deliberate, not an oversight. There is no lock-
   quarantine-nonce convention in rev-4 — D9's lock file is never
   renamed.
5. `CanonicalBatchJSON` (D11) is implemented as its own function,
   distinct from `CanonicalArgsJSON` (D3) — the two encoders share no
   code, since they encode structurally different shapes (a flat
   sorted key/value list vs. a nested `{feature, results[]}` object
   with per-result fixed-field structs), even though both follow the
   same no-`encoding/json.Marshal`, no-HTML-escaping, deterministic-
   ordering philosophy. A second, thin wrapper produces the
   **file-wire** encoding (adds `batch_id`, applies real indentation)
   used for the actual on-disk write and D7's idempotency comparison —
   this wrapper is not a separate canonicalization algorithm, only a
   presentation-layer difference over the same logical content.
6. `os.SameFile` (D6) is the primary descriptor-identity check,
   applied to the `os.FileInfo` returned by `Stat()` on an
   already-open `*os.File` compared against the pre-open `Lstat`
   result — this is a real `fstat`-equivalent on the actual open
   descriptor, not a second pathname lookup. For `db_path`, this is
   **pathname-vs-descriptor**, not descriptor-vs-descriptor: a fresh
   `Lstat` of the `db_path` pathname is taken immediately before
   `cmd.Start()` and again immediately after the child exits, each
   compared (`os.SameFile`) against the same held directory
   descriptor — never `fstat`ing the held descriptor against itself,
   which would always trivially match and prove nothing (D6).
7. `syscall.Flock` (D9) is a single, well-understood kernel primitive
   with no owner-metadata protocol to implement or reason about —
   this replaces rev-3's directory-rename-based lock acquisition
   entirely; there is no longer any `os.Rename`-based lock-acquisition
   step in this design.
8. `syscall.Statfs` (D9, rev-6 stdlib-only import, rev-7 width
   normalization) is stdlib-only — no `golang.org/x/sys/unix` import —
   with the platform split expressed via the exact same `//go:build
   linux`/`//go:build darwin` tags used for the lock and Dolt
   process-group code, never a broader `unix` tag; the Linux
   comparison normalizes `fsType := uint32(buf.Type)` before comparing
   against `uint32`-typed allow/deny constants (rev-7), since
   `Statfs_t.Type`'s width/signedness varies by architecture (PRD
   C35).
9. The Dolt binary trust pin (D5, rev-6 introduced; rev-7 rebinds it
   to executed bytes) is a full-`SHA-256` hash of the resolved
   executable's bytes, computed **once**, while streaming those bytes
   (`io.TeeReader`) into a private, per-invocation ephemeral copy —
   the pin comparison happens against that single hash before the
   copy is executed, and the copy's own path (never the originally-
   resolved pathname) is what `cmd.Path` is set to. This supersedes
   rev-6's design of re-hashing the resolved *pathname* independently
   both before and after invocation, which never bound either hash to
   the bytes actually `exec`'d.
10. `resource_id` collision detection (D3, rev-6 introduced; rev-7
    splits the outcome) recomputes the 12-hex ID from an entry's own
    fields on every load, not only at `add` time, and loads
    `resources.json` as a map keyed by `resource_id` rather than a
    bare array — a self-mismatch (an entry's own recorded ID does not
    match its own recomputed ID) is `resources-file-corrupt`; two
    distinct, independently-correct declarations whose recomputed IDs
    collide is `resource-id-collision`; these are two different Go
    error values, not two branches of one check.
11. Process-group termination timing (D5, rev-7 introduced the
    unreaped-through-grace fix; rev-8 unified it into one sequence but
    gated it behind pipe drain; rev-9 decoupled leader-exit detection
    from pipe drain but reaped the leader too early, reopening a
    PGID-reuse race; rev-10 fixed this with a non-reaping
    `waitid`/`WNOWAIT` observer; rev-11 renames the observer's
    successful return to a leader event/cleanup trigger and hardens
    the edges; rev-12 introduces a single-ownership CAS, deterministic
    priority re-check, and concrete bounded Reap/Pipe-drain deadlines):
    the identical unconditional group-signal sequence —
    `SIGTERM(-pgid)` (tolerating `nil`/`syscall.ESRCH`/`syscall.EPERM`)
    → fixed grace period → `SIGKILL(-pgid)` (same tolerated set) — now
    runs on whichever of **five** sources' recorded-occurrence flag is
    highest-priority once a single atomic ownership primitive is
    acquired: the leader-event/cleanup-trigger observer (a
    build-tagged raw `waitid(P_PID, leaderPID, ..., WEXITED|WNOWAIT)`
    call, retried on `EINTR`, that reports without reaping the
    leader), a terminal observer error (`ECHILD` or any other errno),
    a non-`EOF` pipe-reader error, the output-cap exceeded, or the
    30-second timeout — priority order terminal-observer-error >
    reader-error > cap > timeout > benign-leader-event. `cmd.Wait()`
    is never called before the group-signal sequence completes in any
    non-`ECHILD` classification, and is launched exactly once, in its
    own goroutine, bounded by a fixed 2-second Reap deadline; a bounded
    2-second Pipe-drain deadline is sequenced strictly after the reap
    step (or its timeout fallback) resolves — `cmd.Wait()` is
    emphatically **not** the last event in the sequence (rev-12
    corrects rev-10/rev-11's stronger claim here). Terminal, non-
    `EINTR` observer errors split into two separated finalizers:
    `ECHILD` sends no `-pgid` signals at all and never calls
    `cmd.Wait()` (process identity no longer safely pinned) and makes
    no cleanup claim; any other terminal errno runs the same shared
    bounded finalizer as every other non-`ECHILD` classification,
    including its own bounded reap attempt. There is no fast-path
    exemption, and no code path infers group emptiness from the direct
    child's own apparent exit, from the leader-event observer's firing
    (which, on Darwin, may reflect a stop rather than an exit), from
    `cmd.Wait()`'s own return value/timing, or from a signal call's
    own tolerated-errno outcome — only the signal sequence itself
    reaching every signalable member of the PGID is the design's
    claim, and even that claim is bounded to "not yet recycled," never
    "empty." A cleanup-initiated flag, set the instant ownership is
    acquired, ensures every owner-induced pipe-close's resulting
    reader error is treated as a join-completion signal only, never a
    fresh trigger attempt. **Rev-13 edge-contract additions**: both
    `os.Pipe()` pairs exist, and the two drain goroutines launch, only
    once `cmd.Start()` returns successfully — a `cmd.Start()` failure
    instead closes all four parent-held endpoints directly and
    synchronously with zero goroutines, zero ownership-CAS attempts,
    zero signals, and no `cmd.Wait()` call, a wholly separate path
    from either finalizer. Every forced-close branch — `ECHILD`, the
    shared finalizer's drain-deadline expiry, and its
    `SetReadDeadline`-failure branch alike — now always joins both
    drain goroutines before returning or unlocking; the `ECHILD`
    finalizer's own join step is a join-only helper that never calls
    `SetReadDeadline` on already-closed ends. On `adapter-reap-timeout`,
    up to two goroutines can remain outstanding — the abandoned
    `cmd.Wait()` goroutine and, for a non-leader-event classification,
    the still-blocked observer goroutine — both reporting their own
    eventual completion over a capacity-one, non-blocking-send channel
    so neither ever blocks trying to deliver a result nobody is
    listening for. Finally, a single, fixed cutoff drain — run
    immediately after the initial classification is selected but
    strictly before the shared finalizer's first `-pgid` signal —
    re-checks the terminal-observer-error flag one last time and
    overrides to the `ECHILD` finalizer if `ECHILD` has since become
    set, guaranteeing zero signals in that case; this is sufficient
    because, before this cutoff, the observer's own `WNOWAIT` call is
    the sole wait-family syscall issued against the child and
    `cmd.Wait()` is never launched before the signal phase completes
    — an exclusive-waiter invariant that makes a late `ECHILD` after
    cutoff impossible in a conforming implementation, though not a
    claim that an out-of-contract concurrent waiter would be handled
    safely.
12. Trust-pin storage (D5, rev-7) is a top-level `trust` field on each
    `resources.json` entry, deliberately excluded from
    `resource_id`'s hash-input computation (D3) — the field is
    populated/rewritten by `add --trust-current-dolt` (add-time TOFU
    bootstrap only, rev-8) and `trust-dolt` (the only command that may
    re-pin after `add`), both of which acquire the per-slug `flock`
    (D9) before rewriting it in place.
13. Add-time trust bootstrap vs. capture-time trust verification (D5,
    rev-8): these are two distinct functions sharing only the resolve/
    validate prefix — the add-time function never opens a process-
    start code path at all (there is no `exec.Command` construction
    reachable from it), while the capture-time function is the sole
    caller of the process-start code path and is the only one gated by
    an existing-pin precondition.
14. The private-copy `statfs` no-exec preflight (D5, rev-8) reuses the
    same stdlib-only `syscall.Statfs` call and `//go:build linux`/
    `//go:build darwin` tags as D9's filesystem-type allowlist (item
    8, above), but inspects the no-exec **flag** bit
    (`ST_NOEXEC`/`MNT_NOEXEC`) on the `Flags` field, not the
    filesystem-**type** magic number/`Fstypename` comparison — a
    filesystem already on D9's type allowlist can still independently
    be mounted `noexec`, so this is a second, orthogonal check, not a
    duplicate of D9's.
15. `ENOSPC`/`EIO` during the streamed copy-while-hash step (D5,
    rev-8) are detected via Go's standard `errors.Is` against the
    wrapped `syscall.Errno` values returned by the failing `io.Copy`/
    `Write` call — no new syscall or platform-specific code is
    introduced for this; the failure path removes the partial copy
    file (best-effort, ignoring a second failure) before returning
    `adapter-copy-failed`.
16. The scratch-root untracked gate's whole-subtree check (D8, rev-8)
    is a single `exec.Command("git", "--literal-pathspecs",
    "ls-files", "--", ".tpatch/local/")` invocation per mutating call,
    with the pass/fail decision made on `len(stdout) == 0` after a
    zero exit — a non-zero exit is unconditionally treated as
    `git-ls-files-error`, never coerced into either the tracked or
    untracked outcome; this is a distinct code path from D8's
    per-selector `ls-files --error-unmatch` gate, which examines exit
    code/stderr shape for a single pathname argument instead.

## Negative Consequences Summary

- Ancestor-directory TOCTOU is a documented residual risk for
  `ignored-file`/directory selectors — not fully closed (D6).
- `db_path`/`cmd.Dir` carries an **additional**, distinct residual,
  narrower after rev-6's hard-refusal upgrade: a mismatch detected at
  either the pre-`cmd.Start()` or post-exit recheck is now a hard
  refusal (`db-path-identity-changed`), not merely a logged
  diagnostic — but Go's `os/exec.Cmd.Dir` is still a pathname, not a
  descriptor, so a swap that both occurs and is reverted **entirely
  within** the child process's own execution window (between the
  pre-`cmd.Start()` check and the post-exit check) remains
  undetectable by any check in this design, since nothing here holds
  a descriptor across the child's own internal directory resolution
  (D6).
- No raw content diffing/versioning in v1 — only metadata/hash/
  file-set-level change detection; a directory `diff`/`capture` reads
  its matched files sequentially, not under one atomic point-in-time
  snapshot (D4).
- `flock`-based locking is restricted to hosts this project actually
  builds and tests (`linux || darwin`, matching
  `.github/workflows/ci.yml:18-25`'s `[ubuntu-latest, macos-latest]`
  matrix exactly, not the broader `unix` tag); unsupported platforms
  refuse every mutating verb with `resource-lock-unsupported` in v1,
  and a `statfs`-based filesystem preflight additionally refuses
  network/shared/unrecognized filesystems even on a supported OS
  (`resource-lock-filesystem-unsupported`) — there is no partial/
  degraded lock behavior, no cross-client/cross-host serialization
  claim, on either axis. Windows/other non-Linux/non-macOS hosts are
  explicitly unsupported and deferred, not silently assumed to work
  (D9).
- A feature with many resources cannot parallelize its own staging
  across multiple processes — the lock is per-slug and serializes
  `add`/`remove`/`clear` against `capture`/`record --resources` (D9).
- Mandatory `table` (D5) forecloses a convenient whole-database Dolt
  diff in v1 — a resource declaration must enumerate every table it
  cares about, trading that inconvenience for a hard PK-change error
  instead of a silent omission.
- An orphaned `current.json` entry for a resource later removed from
  `resources.json`, or an orphaned `batches/<id>.json` file from a
  crash between the batch and pointer renames, is permanent and never
  garbage-collected in v1 — harmless (never read by anything) but not
  cleaned up (D7).
- The Dolt binary trust pin (D5, rev-6 introduced; rev-7 rebinds it to
  executed bytes) proves only that the operator explicitly approved
  the exact bytes now pinned — it is **not** proof those bytes match
  any specific upstream source commit, and does not attempt to verify
  provenance, signatures, or a binary's build history. Rev-7's
  private-copy execution narrows, but does not fully close, the
  swap-TOCTOU window between hashing and executing: the pinned digest
  verifies the bytes **written** to the private copy while that copy's
  descriptor is still open, but `cmd.Start()` necessarily opens the
  private copy **by pathname** afterward, not via the already-verified
  descriptor (rev-11 correction — `os/exec` exposes no descriptor-bound
  exec primitive) — so a same-user local attacker with write access to
  the private copy's own ephemeral scratch directory during the narrow
  window between `close` and the subsequent pathname-based `exec`
  remains outside this design's threat model; that narrower residual
  is stated explicitly (D6).
- Rev-12's single-owner, bounded process-group termination sequence
  (D5) still trades a fixed grace-period delay, plus up to two
  further concrete bounds, for one consistent group-signal code path:
  **every** non-`ECHILD` classification — the leader-event/cleanup-
  trigger observer firing, a timeout, a cap-overflow, a non-`EOF`
  pipe-reader error, or a non-`ECHILD` terminal observer error — runs
  the same `SIGTERM`→grace→`SIGKILL(-pgid)`→bounded-Reap→bounded-drain
  sequence, so even a fully successful invocation with no lingering
  descendants pays that fixed latency (now harmless, since the group
  is already empty) before the invocation returns; there is no
  fast-path exemption. A leader that closes both of its pipes (an
  event that does not itself gate anything in this design) while still
  alive doing further work is still unconditionally terminated once
  the leader-event observer fires, a timeout elapses, or the cap is
  exceeded; this surfaces through the existing `dolt-query-error`
  taxonomy, not a distinct named refusal, so an operator debugging such
  a case must know to look at the process-group design (D5) rather
  than expecting a Dolt-specific error message. This design discloses
  several narrower residuals: (a) on Darwin, the observer's successful
  return can reflect a merely-**stopped**, not exited, leader — this
  design treats any successful return as a fail-closed cleanup trigger
  rather than claiming to distinguish the two cases; (b) the `ECHILD`
  classification leaves the leader **fully alive, unsignaled, and
  unreaped** — no signal is ever sent and no cleanup claim is made at
  all in this specific, disclosed case, distinct from every other
  classification; (c) if the shared bounded finalizer's own 2-second
  Reap deadline elapses (a genuinely live, unsignalable, or
  kernel-uninterruptibly-sleeping leader), the invocation reports
  `adapter-reap-timeout`, releases its lock promptly, and leaves up to
  **two** abandoned goroutines running in the background for as long
  as the OS keeps the leader unreaped — the `cmd.Wait()` goroutine
  itself, and, if the triggering classification was not the
  leader-event path, the still-blocked leader-event observer
  goroutine as well — an operator may observe a lingering leader
  process alongside one or two long-lived background goroutines in
  this specific, disclosed failure class, distinct from the normal
  path's guaranteed single bounded reap. Both goroutines report their
  own eventual completion over a dedicated capacity-one,
  non-blocking-send channel, so neither can ever itself block trying
  to deliver a result to a controller that has already returned and
  stopped listening — the residual is bounded to "leaked goroutine
  until the OS event finally occurs," never "blocked goroutine
  forever."
- Rev-13 adds three further narrow, disclosed edges to the same
  process-group design (D5): a `cmd.Start()` failure closes all four
  parent pipe endpoints directly and synchronously with no goroutine,
  signal, or `cmd.Wait()` involvement at all — a wholly separate,
  simpler path from either finalizer, so it carries none of the
  residuals above. Every forced-close branch across both finalizers
  now always joins both drain goroutines before returning or
  unlocking (previously under-specified for the `SetReadDeadline`-
  failure branch), so no forced-close path can return while a drain
  goroutine is still outstanding under this design's own accounting.
  Finally, a single fixed cutoff drain, run after the initial
  triggering classification is selected but strictly before the
  shared finalizer's first `-pgid` signal, re-checks for a
  since-arrived `ECHILD` and, if found, overrides to the no-signal
  `ECHILD` finalizer — this closes the narrow race where `ECHILD`
  becomes true in the small window between classification and the
  first signal, at the cost of that one extra deterministic check on
  every non-`ECHILD` invocation; the guarantee is scoped narrowly to
  the fact that, before this cutoff, this process issues no
  wait-family syscall other than the non-reaping `WNOWAIT` observer
  call, not to arbitrary out-of-contract concurrent waiters.
- The add-time trust bootstrap (D5, rev-8) means a duplicate `add`
  targeting an already-declared resource never re-pins trust, even if
  `--trust-current-dolt` is re-passed and the currently-resolved
  binary's hash has since changed — an operator who upgrades their
  Dolt installation and expects a re-run `add` to pick up the new
  binary will be surprised that nothing changes; `trust-dolt` must be
  invoked explicitly. This is a deliberate trade-off (predictable,
  side-effect-free `add`) rather than an oversight, but it is a real
  behavioral surprise worth stating plainly.
- The private-copy `noexec`/`ENOSPC`/`EIO` handling (D5, rev-8) adds
  three new hard-failure classes an operator may encounter purely as a
  function of their local scratch filesystem's configuration or
  available space/health, independent of anything about the Dolt
  resource declaration itself — a `capture` that worked yesterday can
  fail today if the scratch filesystem's mount options change or fills
  up, with no retry/backoff built into this design (the operator must
  resolve the host condition and re-run `capture`).
- `resources-file-corrupt` vs. `resource-id-collision` (D3, rev-7
  split) are both v1 hard refusals with no auto-repair path — an
  operator must manually edit `resources.json` to resolve either, and
  a genuinely corrupted or colliding file blocks every mutating verb
  (and every read verb that loads the manifest) until fixed.
- No event-chronology/history log exists for resource captures (D7,
  rev-5/rev-6) — `current_batch_id` is a provenance fact about
  `current.json`'s own last rewrite, not a recency claim, and an
  A→B→A capture sequence is indistinguishable from "A was always
  current" once `current.json` no longer names `B`; a future revision
  would need a genuinely new, explicitly-scoped mechanism to add
  event-level history, not an extension of the current pointer's
  semantics.

## Test Matrix

| # | AC | Area | Scenario | Expected |
|---|----|------|----------|----------|
| 1 | AC-1 | Dolt | Capture a declared `diff-summary` resource | Exact argv `<dolt> sql -r json -q "..."` invoked, one 3-arg `dolt_diff_summary(from,to,table)` shape with `ORDER BY from_table_name, to_table_name`, no other Dolt subcommand |
| 2 | AC-1 | Dolt | Inspect invoked argv for any run | No `--name-only`/`--schema`/`--data` flag ever appears |
| 3 | AC-2 | Dolt | `--arg from=$'\x00'` | Exit 2, no SQL constructed |
| 4 | AC-2 | Dolt | `--arg to=a\\b` (backslash) | Exit 2 |
| 5 | AC-2 | Dolt | `--arg from=main..HEAD` (contains `..`) | Exit 2, refused before reaching Dolt |
| 6 | AC-3 | Dolt | `--arg from="O'Brien"` | Single quote doubled, query succeeds, round-trips |
| 7 | AC-4 | Dolt | Missing `db_path` | Exit 2 |
| 8 | AC-4 | Dolt | `--arg table=x --arg table=y` | Exit 2, duplicate key |
| 9 | AC-4 | Dolt | `--arg unexpected=z` | Exit 2, unknown key |
| 10 | AC-5 | Dolt | Table renamed between `from`/`to` | `diff_type: "renamed"`, `from_table_name` != `to_table_name` tracked verbatim on one row, not collapsed |
| 11 | AC-6 | Dolt | `-r json` row missing `schema_change` | Fatal `dolt-json-parse-error` |
| 12 | AC-6 | Dolt | `-r json` row has extra unknown field | Fatal `dolt-json-parse-error` |
| 13 | AC-6 | Dolt | `data_change` is a JSON string, not boolean | Fatal `dolt-json-parse-error`, no coercion attempted |
| 14 | AC-6 | Dolt | Duplicate field key in one row | Fatal `dolt-json-parse-error` |
| 15 | AC-7 | Dolt | `dolt sql -r json` returns literal `{}` (post-trim) | Parsed as `result.tables: []` |
| 16 | AC-7 | Dolt | `-r json` returns `{"rows":[...],"extra":1}` | Fatal parse error, extra top-level key refused |
| 17 | AC-8 | Dolt | PK set changed on the mandatory `table` between `from`/`to` | `dolt-query-error`, exit 3 |
| 18 | AC-9 | Dolt | `table` absent from both `from` and `to` | `result.tables: []`, no error — distinct outcome from `AC-8` |
| 19 | AC-10 | Dolt | `--arg from=WORKING --arg to=STAGED` | Refused, `dolt-argument-refused` (exit 2), case-insensitive, before any Dolt invocation |
| 20 | AC-10 | Dolt | `--arg from=working --arg to=staged` (lowercase) | Refused `dolt-argument-refused`, case-insensitive match confirmed |
| 21 | AC-11 | Dolt | `--arg from=main..HEAD` | Refused at `AC-2` before ever reaching Dolt; dot-range form never exercised |
| 22 | AC-12 | Dolt | First-ever capture of a fresh `adapter-snapshot` resource | Identical schema shape to a later capture; zero-row shape matches nonexistent-table shape |
| 23 | AC-13 | Dolt | Captured stdout buffer is `"...]}\n"` (real nonempty shape) | Parses identically to an idealized exact-bytes fixture |
| 24 | AC-13 | Dolt | Captured stdout buffer is `"{}\n\n"` (real zero-row shape, two newlines) | Trimmed and parsed as zero rows |
| 25 | AC-14 | Dolt | Any `capture` invocation | `dolt version` never appears in the invoked-process log |
| 26 | AC-15 | Dolt | Successful capture | `tool_identity` = `{basename, binary_sha256}` only, no path field present, in every tracked file |
| 27 | AC-16 | Dolt | Inspect the Dolt child process's environment | Exactly `HOME`, `DOLT_ROOT_PATH` (both fresh `0700` scratch); nothing inherited, no `PATH` |
| 28 | AC-17 | Dolt | `dolt` resolves to a path under the repo working tree | Refused `adapter-executable-in-repo` |
| 29 | AC-17 | Dolt | `dolt` resolves under a `.git` directory anywhere | Refused `adapter-executable-in-repo` |
| 30 | AC-18 | Dolt | Resolved Dolt binary's bytes streamed+hashed while copying into a private ephemeral copy; digest does not match the pinned `trust.binary_sha256` | Refused `adapter-binary-untrusted`, no copy executed, copy deleted — supersedes rev-6's independent pre/post-invocation pathname re-hash pair |
| 31 | AC-19 | Dolt | Private-copy digest matches the pin | Copy executed at its own private path (never the originally-resolved pathname); capture-time missing pin (`trust: null`) refused `dolt-trust-required` before any hashing is attempted |
| 32 | AC-20 | Dolt | `add --kind adapter-snapshot --adapter dolt` without `--trust-current-dolt` | Refused `dolt-trust-flag-required` (exit 2, renamed rev-8 from the previously-overloaded `dolt-trust-required`) |
| 33 | AC-20 | Dolt | `add --kind adapter-snapshot --adapter dolt --trust-current-dolt` | Runs only the add-time TOFU bootstrap: declared `trust.binary_sha256` set equal to the `SHA-256` of the binary resolved at that moment; `args` unaffected (no `binary_sha256` key); no existing-pin precondition; zero process-start calls (rev-8) |
| 34 | AC-21 | Privacy | Ignored-file content read for scanning | No file under `resource-scratch/<slug>/` ever contains the raw content bytes |
| 35 | AC-21 | Privacy | Directory selector read for scanning | Every descendant file's bytes stay in-process, never written to scratch |
| 36 | AC-22 | Privacy | Dolt stdout captured for scanning | Stdout never redirected to or copied into a scratch file |
| 37 | AC-22 | Privacy | Dolt stderr captured for scanning | Stderr never redirected to or copied into a scratch file |
| 38 | AC-23 | Privacy | One resource's content matches a PEM key | Whole invocation refused, no batch written for any resource, no unredacted byte written anywhere |
| 39 | AC-23 | Privacy | One resource's content matches a DB connection URL | Same refusal behavior |
| 40 | AC-23 | Privacy | Content matches none of the six classes | Capture proceeds normally |
| 41 | AC-24 | Wire | Inspect every tracked JSON field | No timestamp-shaped field anywhere |
| 42 | AC-25 | Diff | `ignored-file` content changed since last batch | `diff` reads current content through the same bounded scanner `capture` uses, reports which of hash/size/etc. changed, no textual diff |
| 43 | AC-25 | Diff | `ignored-file` unchanged | `diff` reports `unchanged` after recomputing the same hash |
| 44 | AC-26 | Limits | File grows beyond declared limit between `Stat` and actual read | Refused `resource-limit-exceeded` via an actual `limit+1`-byte read, not a stat-only check |
| 45 | AC-26 | Limits | Dolt stdout exceeds declared cap | Refused via the same cap-plus-one read discipline |
| 46 | AC-27 | Diff | Directory `capture`/`diff` documented sequential-read residual | Test asserts files are read one at a time, not under one atomic snapshot; residual stated in §15 |
| 47 | AC-28 | Path | Ancestor directory of selector is a symlink | Refused `symlink-component-refused`, regardless of target |
| 48 | AC-28 | Path | `db_path` ancestor is a symlink | Refused `symlink-component-refused`, same gate as an `ignored-file` selector |
| 49 | AC-29 | Path | Final component replaced by symlink between walk and open | Refused via `O_NOFOLLOW`/`ELOOP` |
| 50 | AC-30 | Path | File replaced (different device/inode) between walk and open | Refused `path-replaced-during-open`, detected via `os.SameFile` on the open descriptor |
| 51 | AC-31 | Path | Missing ancestor component | Refused `path-missing` |
| 52 | AC-32 | Path | Directory selector, one descendant symlinked | That descendant refused, others unaffected |
| 53 | AC-32 | Path | Same selector re-`add`ed after fix | Gate re-verified at `add` |
| 54 | AC-33 | Path | Dolt executable itself is a symlink to an external binary | Followed via `EvalSymlinks`, not refused by the ancestor-symlink rule of `AC-28`-`AC-32` |
| 55 | AC-34 | Path | `db_path` swapped between the pre-`cmd.Start()` fresh check and child exit (simulated) | Hard-refused `db-path-identity-changed` (exit 3), result discarded, no batch written — a swap that both occurs and reverts entirely within the child's own execution window remains a documented residual, not claimed closed |
| 56 | AC-34 | Path | `db_path` unchanged for the full child lifetime | No mismatch flagged, capture proceeds |
| 57 | AC-34 | Path | Comparison-input freshness check: a test that only re-`fstat`s the same unchanged descriptor twice | Correctly asserted as an invalid test for `AC-34` — the real check compares a **fresh** pathname `Lstat` of `db_path` against the held descriptor each time, never descriptor-vs-descriptor |
| 58 | AC-35 | Git gate | `check-ignore` invocation inspected | Never includes `--literal-pathspecs` |
| 59 | AC-36 | Git gate | `check-ignore` exit 1 | Refused `not-ignored` |
| 60 | AC-36 | Git gate | `check-ignore` exit 128 (fatal) | Refused `git-ignore-check-error`, distinct reason |
| 61 | AC-37 | Git gate | Selector `:(glob)config/*.env` | Passed to `check-ignore` with `./` prefix, resolves to the same on-disk path |
| 62 | AC-37 | Git gate | Selector `:(literal)config/name.env` | Passed to `check-ignore` with `./` prefix; supervisor-independently reconfirmed non-fatal outcome |
| 63 | AC-38 | Git gate | Selector containing `*`/`?`/`[]` | No wildcard/glob matching occurs for `check-ignore` |
| 64 | AC-39 | Git gate | `ls-files --error-unmatch` exit 0 (tracked) | Refused `tracked-and-ignored` |
| 65 | AC-39 | Git gate | `ls-files --error-unmatch` unexpected stderr shape | Refused `git-ls-files-error`; call verified to use `--literal-pathspecs` |
| 66 | AC-39 | Git gate | `ls-files --error-unmatch` on a genuinely untracked path | Exit 1 with the expected no-match shape, treated as a valid untracked outcome, not an error |
| 67 | AC-40 | Git gate | Selector ignored but tracked | Refused; both checks (`AC-36` ignored, `AC-39` untracked) must pass |
| 68 | AC-40 | Git gate | Recheck at `add` and at every `capture` | Both checks re-run each time, not cached |
| 69 | AC-41 | Local ignore | Scratch root verified via `EnsureLocalIgnoreContract` | Refused before scratch content created if the contract fails |
| 70 | AC-42 | Local ignore | Scratch root's `ls-files` gate fails (root tracked) | Refused `local-path-tracked` before the persistent `.lock` file is first created |
| 71 | AC-43 | Local ignore | `remove` invoked | Same local-ignore/untracked gate runs before `.lock` acquisition |
| 72 | AC-43 | Local ignore | `clear` invoked | Same local-ignore/untracked gate runs before `.lock` acquisition |
| 73 | AC-44 | Lock | Fresh `capture` invocation | `.lock` opened `O_CREATE|O_RDWR,0600`, `flock(LOCK_EX|LOCK_NB)` succeeds immediately |
| 74 | AC-44 | Lock | Second concurrent `capture` for same slug | `EWOULDBLOCK`/`EAGAIN` refuses immediately `capture-in-progress`, no polling |
| 75 | AC-45 | Lock | `.lock` inode identity across repeated invocations for the same slug | Unchanged device+inode across every invocation — never removed/renamed/replaced |
| 76 | AC-46 | Lock | Process holding `flock` is killed (simulated crash) | Kernel releases lock immediately; next invocation acquires with no manual reclaim |
| 77 | AC-47 | Lock | Each of `add`/`remove`/`clear`/`trust-dolt`/`capture`/`record --resources` invoked | Same per-slug `flock` acquired before first write (rev-9 fix: `trust-dolt` was omitted from this row's own verb enumeration, though D9's own primary list already included it since rev-7) |
| 78 | AC-47 | Lock | `list`/`diff` invoked | Neither ever acquires the `flock` |
| 79 | AC-48 | Lock | Build tagged exactly `!linux && !darwin` (not a generic `!unix`), any mutating verb invoked | `resource-lock-unsupported` (exit 3) deterministically, never silently proceeding unlocked |
| 80 | AC-49 | Lock | Two invocations race to acquire `.lock` for the same slug | Exactly one succeeds, the other refuses immediately `capture-in-progress`, no queued wait |
| 81 | AC-50 | Permissions | Every ephemeral scratch directory created | Mode `0700` at creation, no later `chmod` call observed |
| 82 | AC-50 | Permissions | Every ephemeral scratch file created | Mode `0600` at creation |
| 83 | AC-50 | Permissions | The persistent `.lock` file's one-time creation | Mode `0600` at creation, never `chmod`'d afterward |
| 84 | AC-51 | Scratch | Orphaned `es_*` ephemeral scratch left by a simulated crash | Swept only after the sweeping invocation has itself acquired the live `flock` |
| 85 | AC-51 | Scratch | Orphaned `batches/*.tmp-*.json`/`.tmp-current.json` left by a simulated crash | Swept as an independently verified enumeration, under the exact tracked-root path from §7.1, only after lock acquisition |
| 86 | AC-52 | Scratch | `add`/`remove`/`clear`/`trust-dolt` invoked with orphans present | No `es_*`/tracked-temp removal occurs during any of the four (rev-10, task 2 — corrects a stale three-verb enumeration that omitted `trust-dolt`) |
| 87 | AC-53 | Publication | Multi-resource successful capture | Exactly one new `batches/<id>.json` (unless already-identical, `AC-55`), `current.json` rewritten exactly once |
| 88 | AC-54 | Publication | Recompute `batch_id` from the hash-input `CanonicalBatchJSON` body | Identical `batch_id` reproduced |
| 89 | AC-55 | Publication | Retry with identical batch content | File-wire-bytes comparison (including `batch_id`, real indentation) matches existing file, skips to pointer publish |
| 90 | AC-55 | Publication | Confirm rev-3's bug does not recur | Comparison is never against hash-input bytes, which would never match |
| 91 | AC-56 | Publication | Existing `batches/<batch_id>.json` file-wire bytes differ from the freshly-staged candidate only in whitespace/indentation (semantically identical canonical body) | Classified as presentation drift: on-disk file decoded, `batch_id` verified, semantic body canonicalized and found to match — file left untouched, proceeds to pointer publication exactly as `AC-55`'s byte-identical case does |
| 92 | AC-57 | Publication | `AC-56`'s canonicalized on-disk semantic body genuinely differs from the freshly-staged candidate's (simulated true collision) | Refused `batch-id-collision` (exit 3), never overwritten — reached only after presentation drift has been ruled out by `AC-56`'s comparison |
| 93 | AC-58 | Publication | On-disk `batches/<batch_id>.json` fails to parse as valid JSON | Refused `batch-file-corrupt` (exit 3), never routed through `AC-56`/`AC-57`'s comparisons |
| 94 | AC-58 | Publication | On-disk `batches/<batch_id>.json` parses but its own decoded `batch_id` field does not equal the filename's `batch_id` | Refused `batch-file-corrupt` (exit 3), distinct from both `AC-56` and `AC-57` |
| 95 | AC-59 | Crash/Recovery | Crash simulated after batch rename, before pointer rename | Orphaned batch never surfaced by `list`/`diff`; re-run recomputes identical `batch_id` and proceeds via `AC-55` |
| 96 | AC-60 | Crash/Recovery | Crash simulated during batch temp-write (before its rename) | Only `batches/<id>.tmp-*.json` remains, swept at next invocation's start (`AC-51`), last-committed `current.json` unaffected |
| 97 | AC-60 | Crash/Recovery | Crash simulated during pointer temp-write (before its rename) | Only the exact single `.tmp-current.json` name/pattern remains, swept at next invocation, last-committed `current.json` unaffected |
| 98 | AC-61 | Manifest | `remove <id>` | `resources.json` updated under lock; `current.json` and every `batches/*.json` file untouched |
| 99 | AC-61 | Manifest | `clear` | Same, for every declared resource |
| 100 | AC-62 | Manifest | Resource removed from `resources.json` while `current.json` still references it | Harmless orphaned pointer entry, never garbage-collected, never surfaced by `list` |
| 101 | AC-63 | Read path | `list`/`diff` invoked | Only `current.json` consulted, `batches/` never scanned directly |
| 102 | AC-64 | Metadata | HEAD detached | `symbolic_ref` is `null`, `detached` is `true` |
| 103 | AC-64 | Metadata | HEAD on a branch | `symbolic_ref` populated, `detached` is `false` |
| 104 | AC-65 | Metadata | `config` view with key `user.email` | Refused, exit 2, outside the exact allowlist |
| 105 | AC-65 | Metadata | `config` view with `core.filemode` | Accepted |
| 106 | AC-66 | Metadata | `index-entry` selector `:(icase)Foo` | Resolved as the literal path under `--literal-pathspecs` |
| 107 | AC-67 | Wire | Directory `ignored-file` resource captured | `files[]` present, `path`-sorted, `{path,raw_sha256,byte_count,mode}` per entry, plus aggregate `file_count`/`total_bytes`/`combined_hash` fields |
| 108 | AC-68 | Wire | Each of `head`(attached+detached)/`ref`/`index-entry`/`config`(set+unset)/`ignored-file`(single+dir)/`adapter-snapshot` captured | Every variant's exact tagged shape matches §12.2 |
| 109 | AC-69 | Dry-run | `feature resource capture <slug> --dry-run` | Zero tracked writes; ephemeral scratch removed; newly-created `.lock` is not removed (expected, since it is a persistent ignored control file) |
| 110 | AC-69 | Dry-run | `--dry-run` invoked when no prior tracked tree exists | No `artifacts/resource-captures/` tracked-tree sweep/deletion occurs — only local ephemeral cleanup runs |
| 111 | AC-70 | Record | `record --resources` on feature with zero declared resources | Refused `no-resources-declared` before any Git invocation and before lock acquisition |
| 112 | AC-71 | Record | Staging fails, Git succeeds | `resource-domain-incomplete`, recovery command shown, Git-side canonical patch confirmed present and correct |
| 113 | AC-72 | Record | Staging fails, Git fails | Staged candidate discarded (never written), only record's existing Git-failure behavior surfaces |
| 114 | AC-73 | Record | Staging succeeds, Git succeeds | `batches/<id>.json` and `current.json` reflect the same invocation together, never partially |
| 115 | AC-74 | Record | Re-run after publish-step failure, content unchanged | Identical `batch_id` reproduced, idempotent skip-branch (`AC-55`) taken |
| 116 | AC-74 | Record | Re-run after genuinely changed content | Different `batch_id` produced |
| 117 | AC-75 | Golden ID | Recompute Vector 1 (`git-metadata`/head) | Matches `res_acc91dc23a8b` |
| 118 | AC-75 | Golden ID | Recompute Vector 2 (`adapter-snapshot`, includes `contract`, not `binary_sha256`) | Matches `res_4b62313b6cce` |
| 119 | AC-75 | Golden ID | Recompute Vector 3 (reordered `args` keys) | Matches Vector 2 exactly (`res_4b62313b6cce`), demonstrating key-order independence |
| 120 | AC-75 | Golden ID | Recompute Vector 4 (`ignored-file`) | Matches `res_79f5ac5dca13` |
| 121 | AC-76 | Golden ID | Recompute the worked batch example's `batch_id` from its `CanonicalBatchJSON({"feature","results"})` hash-input body (excluding `batch_id` itself) | Matches `rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021` exactly |
| 122 | AC-77 | Platform | Real lock implementation file's build-tag comment inspected | Exactly `//go:build linux || darwin` |
| 123 | AC-77 | Platform | Fallback lock implementation file's build-tag comment inspected | Exactly `//go:build !linux && !darwin` |
| 124 | AC-78 | Filesystem | `.tpatch/local/resource-scratch/<slug>/` root created on a stubbed denylisted filesystem type (e.g. NFS) | Refused `resource-lock-filesystem-unsupported` (exit 3) before `.lock` is ever created — distinct from `AC-48`'s build-tag-based refusal |
| 125 | AC-78 | Filesystem | Same root on a stubbed allowlisted local filesystem type | Preflight passes, `.lock` creation proceeds |
| 126 | AC-78 | Filesystem | Refusal-string comparison between `AC-48` and `AC-78` | Two distinct error strings, never conflated |
| 127 | AC-79 | Filesystem | Source/import-list of the lock/filesystem package inspected | Uses stdlib `syscall.Statfs` exclusively — no `golang.org/x/sys/unix` import anywhere |
| 128 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for ext (`0xEF53`) | Preflight passes (`Linux` allowlisted type) |
| 129 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for xfs (`0x58465342`) | Preflight passes (`Linux` allowlisted type) |
| 130 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for btrfs (`0x9123683E`) | Preflight passes (`Linux` allowlisted type) |
| 131 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for tmpfs (`0x01021994`) | Preflight passes (`Linux` allowlisted type) |
| 132 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for overlayfs (`0x794C7630`) | Preflight passes (`Linux` allowlisted type) — container-overlay filesystems are permitted; the design allows local overlay mounts (e.g. container runtimes) while denying network-backed filesystems |
| 133 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for nfs (`0x6969`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Linux` denylisted type |
| 134 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for cifs (`0xFF534D42`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Linux` denylisted type |
| 135 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for smb2 (`0xFE534D42`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Linux` denylisted type |
| 136 | AC-80 | Filesystem | Stubbed `Linux` `statfs`/`Fstypename` result for fuse (`0x65735546`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Linux` denylisted type |
| 137 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for apfs (`Fstypename`) | Preflight passes (`Darwin` allowlisted type) |
| 138 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for hfs (`Fstypename`) | Preflight passes (`Darwin` allowlisted type) |
| 139 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for tmpfs (`Fstypename`) | Preflight passes (`Darwin` allowlisted type) |
| 140 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for nfs (`Fstypename`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Darwin` denylisted type |
| 141 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for smbfs (`Fstypename`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Darwin` denylisted type |
| 142 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for webdav (`Fstypename`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Darwin` denylisted type |
| 143 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for osxfuse (`Fstypename`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Darwin` denylisted type |
| 144 | AC-80 | Filesystem | Stubbed `Darwin` `statfs`/`Fstypename` result for macfuse (`Fstypename`) | Refused `resource-lock-filesystem-unsupported` (exit 3) — `Darwin` denylisted type |
| 145 | AC-80 | Filesystem | A filesystem type value on neither the Linux allow/deny list nor the Darwin allow/deny list (unrecognized) | Refused identically to a denylisted value — fail-closed on any unrecognized type, on either platform |
| 146 | AC-81 | Scratch | First-ever creation of `.tpatch/local/resource-scratch/<slug>/`'s intermediate directories | Ignore gate targets the intended not-yet-created leaf directly (existence-independent); untracked gate targets the whole `.tpatch/local/` subtree (rev-8, task 7); `statfs` preflight targets the nearest existing ancestor only; then `MkdirAll`, then unconditional `fsync` of every directory in the relevant chain, both newly-created and already-visible on retry (rev-9 text fix: row previously said "fsync of each newly-created directory's parent," stale relative to D9's own already-correct unconditional-whole-chain text) |
| 147 | AC-81 | Scratch | Crash simulated immediately after `MkdirAll` but before the `fsync` sequence completes | Retried invocation re-creates (idempotent `MkdirAll`) and re-`fsync`s the chain rather than assuming prior durability |
| 148 | AC-82 | Publication | First-ever `capture`/`record --resources` for a slug (no prior `artifacts/resource-captures/` tree), crash simulated between the tracked tree's `MkdirAll` and its parent-directory `fsync` completing | Retried invocation recovers cleanly — re-running the idempotent `MkdirAll`, re-`fsync`ing, and proceeding to §7.3 steps 2-4 exactly as if the tree had always existed |
| 149 | AC-83 | Output cap | Simulated Dolt child writes 6 MiB combined stdout+stderr | The output-cap-exceeded trigger enters the unified cleanup sequence (§6.4/D5): unconditional `SIGTERM(-pgid)`→grace→`SIGKILL(-pgid)`; refused `resource-limit-exceeded` (exit 3); JSON parser never invoked at all for the over-cap invocation |
| 150 | AC-84 | Output cap | Stdout alone 4 MiB, stderr alone 4 MiB (each under cap, combined over) | Refused `resource-limit-exceeded` — proves one shared budget, not two independent 5 MiB budgets |
| 151 | AC-85 | Process group | `cmd.SysProcAttr{Setpgid: true}` set before `cmd.Start()` on `linux`/`darwin`, inspected | Confirmed set; regardless of which of the **five** cleanup-trigger sources is selected as the entry's classification (leader event, terminal observer error, output cap, invocation timeout, or a non-`EOF` pipe-reader error) that is not `ECHILD`, the leader is never reaped (no `cmd.Wait()` call) until after the unconditional `SIGTERM(-pgid)`→grace→`SIGKILL(-pgid)` sequence has fully run; `cmd.Wait()` is launched exactly once, strictly after that sequence completes, in its own goroutine bounded by the fixed 2-second Reap deadline (rev-12, task 4 — corrects rev-10/rev-11's stronger "always the last event" claim, which did not account for a leader that never becomes waitable within a bounded window); for `ECHILD` specifically, no `-pgid` signal and no `cmd.Wait()` call occurs at all (rev-12, task 3) |
| 152 | AC-85 | Process group | Test Dolt-adapter stub spawns a descendant that ignores `SIGTERM` and closes its own pipes; the leader-event/cleanup-trigger observer fires first (leader's own exit) | The descendant is eventually terminated by the group `SIGKILL` (not orphaned, not reaped early); `cmd.Wait()` is asserted **not yet called** when the observer fires, and is called for the first time strictly after the group-signal sequence completes, returning well within the 2-second Reap deadline in this fixture; the parent `tpatch` process itself is never signaled or observably affected |
| 153 | AC-86 | Diff | Directory `ignored-file`, one file's mode changed, content/byte_count unchanged (chmod-only) | `combined_hash` differs on next `capture`/`diff`; `diff` reports that entry's `mode` as the differing field, `hash`/`byte_count` unchanged for that entry |
| 154 | AC-87 | Publication | Resource captured with content `A`, then `B`, then `A` again (three `capture` invocations) | Exactly two distinct `batches/<id>.json` files exist after the third invocation, not three; `current.json` repoints to the pre-existing `A` batch without creating a new batch file |
| 155 | AC-88 | Manifest | Adding a resource whose recomputed `resource_id` matches an existing `resources.json` entry with byte-identical canonical declaration bytes | Idempotent (exit 0, no duplicate entry) |
| 156 | AC-88 | Manifest | Adding (or loading, via any verb reading `resources.json`) an entry whose recomputed `resource_id` matches a **different, independently-correct** existing entry's recorded ID with different canonical bytes (via a test-only stub hash-collision seam) | Refused `resource-id-collision` (exit 3) — narrowed in rev-7 to two-distinct-declarations only |
| 157 | AC-89 | Golden ID | Recompute the two-file golden directory vector (`config/a.txt` empty, `config/sub/b.sh` with known content) via a reference implementation of the `path`+`0x00`+`mode`+`0x00`+`hash`+`0x00` tuple-encoding rule | Matches the documented `combined_hash` `5af4d6754656795b49c6e22acc2034ed6a2b3426470b0c42156f5ad0b4bcb9ad` exactly |
| 158 | AC-90 | Trust | `capture` for an `adapter-snapshot`/`dolt` resource with `trust: null` | Refused `dolt-trust-required` (exit 3) before any Dolt-binary descriptor is opened |
| 159 | AC-91 | Trust | `add --kind adapter-snapshot --adapter dolt --arg contract=dolt-diff-summary-v2` (unsupported value) | Refused `dolt-contract-unsupported` (exit 2), before `db_path`/`table`/`from`/`to` validation |
| 160 | AC-91 | Trust | `contract=dolt-diff-summary-v1` present in two differently-ordered `args` declarations | Both recompute to the same `resource_id` (identity participation confirmed) |
| 161 | AC-92 | Trust | `trust-dolt <slug> <resource-id> --binary-sha256 <64hex>` on an existing Dolt resource | Only `trust.binary_sha256` changes; `resource_id`, `current.json`, and every `batches/<id>.json` file are byte-for-byte unchanged, confirmed via before/after `list --json`/`diff` comparison |
| 162 | AC-93 | Trust | Private Dolt-binary copy's permissions/lifetime inspected across success and failure invocations | Created `0600` (`O_CREATE\|O_EXCL\|O_WRONLY`), streamed/hashed/`Sync`ed, and hardened to `0500` via a descriptor-based `Fchmod` on the still-open file only after a matching digest (rev-10, task 4 — corrects rev-8's direct-`0500`-at-creation design; never a path-based `os.Chmod`); deleted (best-effort) after the child exits on both outcomes |
| 163 | AC-94 | Trust | Test double records `cmd.Path`/observed `argv[0]` for a Dolt invocation | Equals the private copy's own ephemeral path, never the originally `LookPath`/`EvalSymlinks`-resolved pathname |
| 164 | AC-95 | Manifest | A hand-constructed `resources.json` entry whose own recorded `resource_id` does not match its own freshly-recomputed value | Refused `resources-file-corrupt` (exit 3) at load time — distinct from `AC-88`'s two-declaration collision |
| 165 | AC-96 | Process group | A leader exits successfully while a descendant sharing its PGID keeps both stdout/stderr pipe write ends open | The leader-event/cleanup-trigger observer (§6.4/D5) reports the leader event **without reaping it**; ownership is acquired and the deterministic priority re-check selects "benign leader event" (no higher-priority source co-occurring); the unconditional `SIGTERM`→grace→`SIGKILL(-pgid)` sequence still runs and kills the descendant while the leader remains unreaped throughout; `cmd.Wait()` is called for the first and only time strictly after the sequence completes, within the 2-second Reap deadline; the bounded pipe-drain deadline then completes normally, sequenced after the reap, since the descendant that held the pipes open is already dead by that point |
| 166 | AC-97 | Process group | All **five** cleanup-trigger sources — leader-event observer fires first, 30-second timeout elapses first, output cap is exceeded first, a non-`EOF` pipe-reader error occurs first, or a terminal observer error occurs first (rev-12, task 1/2, new fifth source and deterministic priority) — exercised through the ownership-CAS/priority-re-check design, each resolving to the appropriate one of the two finalizers | The unconditional group-signal sequence and its `-pgid` argument are byte-for-byte identical across every non-`ECHILD` source, tolerating `nil`/`syscall.ESRCH`/`syscall.EPERM` at both signal steps; exactly one cleanup body runs per invocation regardless of how many sources' own conditions become true; the reported classification matches the fixed priority order (terminal observer error > reader error > cap > timeout > benign leader event) regardless of which source's own CAS attempt won the ownership race; `cmd.Wait()` is observed at most once per invocation and is **never** the last event — bounded drain finalization always follows it (rev-12, task 6 — corrects rev-10/rev-11's "always the last event" claim) |
| 167 | AC-98 | Scratch | Leaf-targeted **ignore** half of the gate evaluated for `.tpatch/local/resource-scratch/<slug>/` both when the leaf directory exists and when it does not (fresh clone) | Identical refusal/pass outcome in both cases; distinguished from the `statfs` preflight, which necessarily targets the nearest existing ancestor |
| 168 | AC-99 | Scratch | Retried invocation after a simulated first-attempt crash, with some directories in the chain already `Stat`-visible from the failed attempt | Every directory in the chain (local scratch and tracked `artifacts/resource-captures/`/`batches/`) is unconditionally re-`fsync`'d, not only directories `MkdirAll` reports as newly created on the retry |
| 169 | AC-100 | Filesystem | Stubbed `linux/s390x` `Statfs_t.Type` (`uint32`) compared against the `uint32`-typed allow/deny constants | Comparison succeeds identically to `linux/amd64`/`arm64` (`int64`) and `linux/386`/`arm` (`int32`) stubs for the same filesystem magic number, confirming architecture-independent normalization |
| 170 | AC-101 | Scratch | A tracked file planted under a *different* slug's scratch tree than the one being mutated (`.tpatch/local/resource-scratch/other-slug/leftover`) | Refused `local-path-tracked` for the slug currently being mutated too — untracked half is whole-`.tpatch/local/`-subtree-scoped, not per-slug-leaf-scoped, via plain `git --literal-pathspecs ls-files -- .tpatch/local/` with empty-stdout convention |
| 171 | AC-102 | Trust | `add --kind adapter-snapshot --adapter dolt ... --trust-current-dolt` for a resource with no prior `trust` entry at all | Succeeds, records `trust.binary_sha256`; opens and hashes the resolved Dolt binary directly with no private copy file ever created under `es_<id>/`; test double asserts zero process-start calls AND zero scratch-file-creation calls occur during `add`, with or without the flag (rev-10, task 4 — corrects a prior design that unnecessarily created and deleted an unexecuted private copy at add time) |
| 172 | AC-103 | Trust | Duplicate `add` of an identical declaration, re-passing `--trust-current-dolt` while the currently-resolved Dolt binary's hash differs from the stored pin | Strict no-op: `trust.binary_sha256` byte-for-byte unchanged; no copy/exec test-double calls observed |
| 173 | AC-104 | Trust | Stubbed private-copy scratch filesystem `statfs` result with the no-exec flag set (Linux `ST_NOEXEC`/Darwin `MNT_NOEXEC`) | Refused `adapter-copy-noexec` (exit 3) before any byte of the Dolt binary is copied |
| 174 | AC-105 | Trust | Test double injects `ENOSPC` then, in a second run, `EIO` during the streamed copy-while-hash step | Refused `adapter-copy-failed` (exit 1) in both cases; partial copy file removed (best-effort); no Dolt process started |
| 175 | AC-106 | Process group | Cross-compile/source-shape test builds the build-tagged (`//go:build linux`, `//go:build darwin`, shared `//go:build linux \|\| darwin`) leader-event/cleanup-trigger observer source layout for `linux/amd64`, `linux/arm64`, and `darwin/arm64` | All three targets compile with no build errors and no `golang.org/x/sys` dependency; the `darwin/arm64` build additionally runs successfully on the development host, observing a spawned child's exit via the non-reaping call (a second, immediately-repeated call also succeeds, proving the child was not reaped) followed by a single `cmd.Wait()` that still correctly reaps the child and reports its real exit code |
| 176 | AC-107 | Process group | Darwin-only test double: a spawned leader receives a `SIGSTOP`-family signal (stopped, not exited); the `waitid(..., WEXITED\|WNOWAIT)` observer call returns successfully | Treated identically to a genuine leader-event cleanup trigger — the unconditional `SIGTERM`→grace→`SIGKILL(-pgid)` sequence still runs; no code path anywhere asserts or logs this as "the leader has exited," only as an unqualified leader event/cleanup trigger (PRD `C42`) |
| 177 | AC-108 | Process group | A `-pgid` `SIGTERM` or `SIGKILL` call returns `syscall.EPERM` (simulated via a test double on a fixture whose only remaining process-group member is an unreaped zombie) | Tolerated identically to `syscall.ESRCH` — cleanup continues through the remaining steps, no refusal is raised, and no claim of group emptiness or non-emptiness is inferred from either outcome; the group-signal step's own return value never gates any later step's execution |
| 178 | AC-109 | Process group | Raw `waitid` syscall helper injected with `EINTR` a fixed number of times before succeeding; separately, a genuinely terminal non-`EINTR` observer error is injected, once as `ECHILD` and once as another errno | The observer retries transparently on `EINTR` and eventually reports the leader event; a non-`ECHILD` terminal error acquires ownership, runs the shared bounded finalizer (including its own bounded 2-second-Reap-deadline reap attempt) with `adapter-process-observer-failed` (exit 1) as primary; a terminal `ECHILD` error instead runs the separated `ECHILD` finalizer: **no** `-pgid` signal is ever sent, both owned pipe read ends are force-closed, both drain goroutines are joined via the bounded-ceiling helper, the invocation refuses the same name, and makes no cleanup claim whatsoever — even under an injected `os.ErrClosed`-shaped reader error induced by the force-close, **zero** negative-PGID signal calls are observed, proving the cleanup-initiated-flag suppression works (rev-12, task 1/3, extended) |
| 179 | AC-110 | Process group | A non-`EOF` read error is injected on one pipe mid-invocation; separately, the post-cleanup `SetReadDeadline` call itself is injected to fail | The reader error is itself a cleanup-entry trigger, runs the unconditional cleanup sequence, and refuses `adapter-output-read-failed` (exit 1) once cleanup completes; the `SetReadDeadline` failure closes both read ends immediately and refuses the same name rather than a distinct one |
| 180 | AC-111 | Process group | Leader exits successfully but its descendant has escaped the process group (own `setsid()`) and keeps a pipe write end open past the bounded post-cleanup drain deadline | Both read ends are force-closed, invocation refuses `adapter-drain-timeout` (exit 3), publishes nothing (no tracked write attempted), and the per-slug `flock` is released; the `tpatch` test-runner process itself is never signaled or observably affected; the escaped descendant's continued existence afterward is a disclosed residual, not a claim of successful cleanup |
| 181 | AC-112 | Process group | Two or more of the five cleanup-trigger sources are simulated as becoming true near-simultaneously (e.g. output-cap exceeded and terminal observer error injected together, with the ownership CAS artificially forced to be won by the lower-priority source's goroutine) | Exactly one cleanup body executes; the reported classification is always the higher-priority source (terminal observer error) per the fixed re-check order, regardless of which source's own CAS attempt actually won the ownership race |
| 182 | AC-113 | Process group | The `ECHILD` finalizer's own force-close of both owned pipe read ends is asserted to occur only after the cleanup-initiated flag is set; a drain goroutine's resulting `os.ErrClosed`-shaped read is inspected | The induced read error is classified as a join-completion signal only, never resubmitted as a fresh reader-error trigger attempting the ownership CAS; an equivalent test for the shared finalizer's drain-timeout force-close confirms the same suppression |
| 183 | AC-114 | Process group | Worked multi-error scenarios: (a) an output-cap-exceeded invocation whose finalizer also hits an untolerated group-signal errno and a drain timeout; (b) a benign leader-event invocation whose finalizer's signal step fails and whose drain also times out | (a) reports `resource-limit-exceeded` (exit 3) as the sole primary reason, the later signal/drain failures recorded only as local diagnostics; (b) reports `adapter-group-signal-failed` (exit 1) as primary — the signal phase precedes the drain phase in the fixed walk order |
| 184 | AC-115 | Process group | A test leader process injected to never become waitable within the 2-second Reap deadline (simulating a kernel-uninterruptible sleep) | The finalizer records `adapter-reap-timeout` (exit 1), proceeds without a second `Wait()` call, releases the per-slug `flock` promptly rather than holding it indefinitely, and still attempts the subsequent bounded pipe-drain finalization; the abandoned `cmd.Wait()` goroutine is disclosed as a residual, never joined or cancelled |
| 185 | AC-116 | Process group | `cmd.Start()` itself is forced to return an error | All four parent-held pipe endpoints (both read ends, both write ends) are closed directly and synchronously by the same goroutine that called `Start()`; zero drain goroutines are spawned; zero ownership-CAS attempts occur by any source; zero `-pgid` signals are sent; `cmd.Wait()` is never invoked — this path is proven wholly separate from, and never reachable through, either the `ECHILD` finalizer or the shared bounded finalizer |
| 186 | AC-117 | Process group | Table-driven: the `ECHILD` finalizer's force-close, the shared finalizer's drain-deadline-expiry force-close, and the shared finalizer's `SetReadDeadline`-failure force-close, exercised in turn | Every forced-close branch joins both drain goroutines (via the bounded-join helper, or the `ECHILD` finalizer's join-only variant) strictly before returning control or releasing the per-slug `flock`; no branch returns with either drain goroutine unjoined; the `ECHILD` join-only helper is confirmed to never call `SetReadDeadline` on either already-closed read end |
| 187 | AC-118 | Process group | An invocation is driven to `adapter-reap-timeout` via a non-leader-event classification (e.g. invocation timeout) with the leader never becoming waitable | Both the abandoned `cmd.Wait()` goroutine and the original leader-event observer goroutine (still blocked in its own retried `waitid` call) are disclosed as residuals; both goroutines' own eventual completion sends succeed instantly over their capacity-one, non-blocking-send channels even with no receiver present after the invocation has returned, so neither ever blocks trying to report |
| 188 | AC-119 | Process group | A race test publishes the `ECHILD` recorded-occurrence flag strictly between the initial trigger snapshot (priority re-check, selecting a lower-priority classification) and the fixed cutoff-drain instant, strictly before any `-pgid` signal has been sent | The invocation switches to, and runs, the `ECHILD` finalizer instead of the originally-selected shared bounded finalizer; **zero** `-pgid` signal calls are observed for the whole invocation |
| 189 | AC-120 | Process group | A test-only call-count hook instruments `cmd.Wait()` invocation timing across every classification that runs the shared bounded finalizer | `cmd.Wait()` is never launched until strictly after the signal phase (steps 1-3: `SIGTERM`, grace, `SIGKILL`) has fully completed, for every one of those classifications — confirming the exclusive-waiter invariant the cutoff-drain guarantee depends on actually holds |

**189 rows** cover **120** distinct `AC` clauses; several clauses (e.g.
`AC-1`, `AC-2`, `AC-4`, `AC-6`, `AC-7`, `AC-10`, `AC-13`, `AC-17`,
`AC-20`, `AC-21`, `AC-22`, `AC-23`, `AC-25`, `AC-26`, `AC-28`, `AC-32`,
`AC-34`, `AC-36`, `AC-37`, `AC-39`, `AC-40`, `AC-43`, `AC-44`, `AC-47`,
`AC-50`, `AC-51`, `AC-55`, `AC-58`, `AC-60`, `AC-61`, `AC-64`, `AC-65`,
`AC-69`, `AC-74`, `AC-75`, `AC-77`, `AC-78`, `AC-80`, `AC-81`, `AC-85`,
`AC-88`, `AC-91`) are exercised by more than one row — this matrix
does not claim any clause is covered "exactly once." `AC-80` alone
contributes 18 of those 189 rows: 17 named allow/deny filesystem-type
fixtures (matching `AC-80`'s own "17 supporting Test Matrix rows"
text) plus 1 additional row for the unrecognized-type case its
definition text separately calls out as "also exercised here" — the
largest single-clause row count in this table. Rows 176-180 (rev-11)
cover `AC-107`-`AC-111` one-to-one, the Darwin
`SIGSTOP`/`EPERM`-tolerance/`EINTR`-retry-and-terminal-error/
non-`EOF`-reader-error/bounded-drain-timeout hardening added by the
rev-11 platform/state-machine fold (§6.4/D5). Rows 181-184 (rev-12,
not further changed this revision) cover `AC-112`-`AC-115` one-to-one,
the single-ownership-CAS/deterministic-priority, cleanup-initiated-flag,
multi-error primary-selection, and bounded-Reap-deadline hardening
added by the rev-12 process-finalizer fold (§6.4/D5); rows
151/152/165/166/178 were corrected in place for rev-12 and are
unchanged this revision. Rows 185-189 (rev-13, new) cover
`AC-116`-`AC-120` one-to-one, the `cmd.Start()`-failure four-endpoint
close, the every-forced-close-joins audit, the dual-goroutine
reap-timeout residual, the late-`ECHILD` cutoff-drain race, and the
exclusive-waiter `cmd.Wait()`-gating invariant added by the rev-13
edge-contract fold (§6.4/D5); no existing row's own text required
correction this revision beyond the pipe-setup/`Start()`-ordering,
`ECHILD` join-only-helper, `SetReadDeadline`-failure-join, and
observer/reap-goroutine-channel prose already updated in D5 above.
