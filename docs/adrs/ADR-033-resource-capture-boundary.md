# ADR-033 — Resource Capture Boundary (rev-8)

**Status**: Proposed — rev-8 (supersedes rev-7, writer commit
`2aba39b`, rev-7 adjudicated NEEDS REVISION → REV-8 DISPATCHED at
`bc2c068`; see `docs/supervisor/LOG.md`)

**Context**: `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
(rev-8, companion document — this ADR binds the decisions that PRD's
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
`exec.Cmd.StdoutPipe()`/`StderrPipe()` **piped reads** (rev-5
correction: an unbounded `*bytes.Buffer` set directly on
`Cmd.Stdout`/`Stderr` has no way to refuse output past a cap short of
reading it all first), drained concurrently by two goroutines into a
single **shared** cap-plus-one memory budget — combined stdout+stderr
bytes, not two independent per-stream budgets — that is checked on
every read; on overflow, this design kills the child's entire process
group (`SIGTERM` then, after a short grace, `SIGKILL`, rather than
just the immediate child, since Dolt may itself fork helper
processes), continues draining/discards the remainder to let the
child exit cleanly, waits for it, and refuses with
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

### D5 — Dolt adapter protocol: mandatory `db_path`/`table`, exact `dolt_diff_summary` SQL, trust pin/identity split, private-copy execution, unreaped-leader termination (task 4, task 8, task 12; rev-7 private-copy execution/`contract` enum/`trust-dolt`)

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
  followed by open→stream-copy-while-hash→write `trust.binary_sha256`
  →delete the copy. This sequence has **no existing-pin precondition**
  (there is, by definition, no pin yet the first time this runs) and
  **never executes** the Dolt binary — it exists solely to compute and
  record the initial pin. `add --kind adapter-snapshot --adapter dolt`
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
into a private, per-invocation ephemeral scratch file created directly
at mode `0500` (`O_CREATE|O_EXCL|O_WRONLY, 0500` — rev-8 creates the
file at its final hardened mode directly, rather than rev-7's
`0700`-then-`chmod 0500`, since the no-exec preflight already ran and
no code needs to write further once the mode is set). A write failure
during this streamed copy with `ENOSPC` or `EIO` is `adapter-copy-failed`
(exit 1, distinct from the exit-3 policy-refusal family — this is a
host I/O failure, not a policy decision), with the partial copy file
removed best-effort and no process started. Once the copy completes,
the digest is compared against `trust.binary_sha256` **before** the
copy is ever executed — a mismatch is `adapter-binary-untrusted` (exit
3), no process started, copy deleted. The child process is then
executed using the **private copy's own path**, never the
originally-resolved pathname — so, unlike rev-6's re-hash-the-pathname-
twice design, the hash that gated execution and the bytes actually
`exec`'d are provably the same descriptor's content, not two
independent reads of a mutable name that a concurrent attacker could
swap between them. Executing a copy that physically resides inside
`.tpatch/local/` (rather than "outside the repo," D6's stated
rationale for the *resolved* binary) remains safe because the copy's
bytes are descriptor-bound, hash-verified against the trust pin, and
owner-only (`0500`) — the safety property comes from those three
facts, not from the copy's filesystem location. The private copy is
deleted after the child exits (success or failure). The pin
establishes *which* binary is operator-approved to define Dolt's
semantic contract for this resource — it is **not** proof that binary
matches any specific pinned upstream source commit, only that it is
byte-identical to what the operator explicitly trusted; the new
`contract` enum value (`"dolt-diff-summary-v1"`) is the disclosed
semantic-contract label, and D5's strict five-field JSON parser
remains the independent, separate runtime capability gate on *what the
binary actually printed* — none of the three substitutes for either of
the others. `tool_identity.binary_sha256` in every tracked result
remains always identical to the declaration's pinned value (D10),
never a freshly-recomputed value presented as if it might differ.

**Process-group termination — one unified sequence for every invocation**
(rev-6 introduced `Setpgid`; rev-7 fixed the reap-timing gap; rev-8
unifies the two paths rev-7 left separate): before `cmd.Start()`, the
adapter sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`
(`linux`/`darwin` only, matching D9's build-tag contract), making the
spawned child the leader of a **new** process group distinct from
`tpatch`'s own. Rev-8 replaces rev-7's two-path design (a kill path
that deferred `cmd.Wait()` past the group `SIGKILL`, and a separate
"normal-success" path that called `cmd.Wait()` immediately on EOF)
with a **single sequence applied identically to every invocation**,
regardless of whether it times out, overflows the output cap, or
completes normally: (1) drain both pipes to EOF; (2) unconditionally
signal `syscall.Kill(-pgid, syscall.SIGTERM)` (the negative-PGID
form), tolerating `ESRCH` (meaning the group is already fully gone);
(3) keep the leader **unreaped** (`cmd.Wait()` not yet called) through
a fixed grace period, regardless of what triggered this sequence or
whether the direct child appears to have already exited; (4)
unconditionally signal `syscall.Kill(-pgid, syscall.SIGKILL)`,
tolerating `ESRCH`; (5) only then call `cmd.Wait()`, exactly once.
There is no "`cmd.Wait()` observes the group has exited" claim
anywhere in this design, and no code path infers group emptiness from
the direct child's own exit status. Rev-7's own rationale — that
`cmd.Wait()` only ever waits on the direct child, not the whole
process group, so reaping the leader early frees its PID for reuse
while a rogue descendant remains alive — is retained and generalized:
the load-bearing fact is that the `SIGTERM`→grace→`SIGKILL`→group→
`Wait()` escalation is **never skipped or cancelled merely because the
direct child appears to have exited**, not that an unreaped leader by
itself "prevents PGID reuse" (a rev-7 framing rev-8 corrects; the
prevention comes from the escalation always running to completion,
not from any inherent property of leaving a PID unreaped). Rev-8
discloses two trade-offs this unification introduces, both previously
absent from rev-7's two-path design: (a) **every** invocation, even
one that completes successfully almost instantly, now pays the fixed
grace-period latency before `cmd.Wait()` is called — there is no
fast-path exemption; (b) a leader that closes both of its pipes (EOF)
while remaining alive to do further work is now unconditionally
carried through `SIGTERM`→grace→`SIGKILL` and forcibly terminated —
this failure mode surfaces through the existing `dolt-query-error`
taxonomy (D5, exit 3), not a new named refusal, since from the
adapter's perspective it is indistinguishable from any other
truncated-but-parseable-or-unparseable Dolt output. Verification:
tests spawn a Dolt-invocation stand-in that itself forks a descendant
which ignores `SIGTERM` and closes its pipes, exercised through
**both** the timeout/cap-overflow trigger and the ordinary
successful-completion trigger via the identical code path, and assert
(i) the descendant's eventual termination, (ii) the `tpatch`
test-runner process's own survival, (iii) exactly one `cmd.Wait()`
call per invocation, occurring only after the group `SIGKILL`, in
every case.

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
per-invocation Dolt-binary copy (D5, rev-7) is the one deliberate
exception to the `0600` file default: it is created `0700` (matching
its containing scratch directory), then hardened to `0500` only after
its streamed hash has been confirmed to match the trust pin — a
narrower, execute-permission-bearing mode reflecting that this file,
uniquely among scratch content, is meant to be executed, not merely
read or written. Tracked artifacts
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
    unreaped-through-grace fix; rev-8 unifies it into one sequence):
    every invocation — success or kill-triggered alike — runs the
    identical sequence: drain both pipes to EOF → unconditional
    `SIGTERM(-pgid)` (tolerating `ESRCH`) → unreaped through the fixed
    grace period regardless of trigger → unconditional `SIGKILL(-pgid)`
    (tolerating `ESRCH`) → exactly one `cmd.Wait()` call. Rev-7's two
    separate code paths (a kill path and a "normal-success" path
    calling `Wait()` immediately on EOF) are replaced by this single
    function; there is no fast-path exemption, and no code path infers
    group emptiness from the direct child's own apparent exit.
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
  private-copy execution closes the swap-TOCTOU window between
  hashing and executing (the executed bytes and the hashed bytes are
  now the same open descriptor's content, not two independent
  pathname reads) but does **not** close a same-user local attacker
  with write access to the private copy's own ephemeral scratch
  directory during the narrow window between its creation and its
  execution — that narrower residual is stated explicitly (D6).
- Rev-8's unified process-group termination sequence (D5) trades
  latency for one consistent code path: **every** Dolt invocation, even
  one that completes almost instantly, now pays the fixed grace-period
  delay before `cmd.Wait()` is called — there is no fast-path exemption
  for an ordinary quick success. A leader that closes both of its
  pipes (EOF) while still alive doing further work is now
  unconditionally terminated by this same sequence; this surfaces
  through the existing `dolt-query-error` taxonomy, not a distinct
  named refusal, so an operator debugging such a case must know to
  look at the process-group design (D5) rather than expecting a
  Dolt-specific error message.
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
| 77 | AC-47 | Lock | Each of `add`/`remove`/`clear`/`capture`/`record --resources` invoked | Same per-slug `flock` acquired before first write |
| 78 | AC-47 | Lock | `list`/`diff` invoked | Neither ever acquires the `flock` |
| 79 | AC-48 | Lock | Build tagged exactly `!linux && !darwin` (not a generic `!unix`), any mutating verb invoked | `resource-lock-unsupported` (exit 3) deterministically, never silently proceeding unlocked |
| 80 | AC-49 | Lock | Two invocations race to acquire `.lock` for the same slug | Exactly one succeeds, the other refuses immediately `capture-in-progress`, no queued wait |
| 81 | AC-50 | Permissions | Every ephemeral scratch directory created | Mode `0700` at creation, no later `chmod` call observed |
| 82 | AC-50 | Permissions | Every ephemeral scratch file created | Mode `0600` at creation |
| 83 | AC-50 | Permissions | The persistent `.lock` file's one-time creation | Mode `0600` at creation, never `chmod`'d afterward |
| 84 | AC-51 | Scratch | Orphaned `es_*` ephemeral scratch left by a simulated crash | Swept only after the sweeping invocation has itself acquired the live `flock` |
| 85 | AC-51 | Scratch | Orphaned `batches/*.tmp-*.json`/`.tmp-current.json` left by a simulated crash | Swept as an independently verified enumeration, under the exact tracked-root path from §7.1, only after lock acquisition |
| 86 | AC-52 | Scratch | `add`/`remove`/`clear` invoked with orphans present | No `es_*`/tracked-temp removal occurs during any of the three |
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
| 146 | AC-81 | Scratch | First-ever creation of `.tpatch/local/resource-scratch/<slug>/`'s intermediate directories | Ignore gate targets the intended not-yet-created leaf directly (existence-independent); untracked gate targets the whole `.tpatch/local/` subtree (rev-8, task 7); `statfs` preflight targets the nearest existing ancestor only; then `MkdirAll`, then `fsync` of each newly-created directory's parent |
| 147 | AC-81 | Scratch | Crash simulated immediately after `MkdirAll` but before the `fsync` sequence completes | Retried invocation re-creates (idempotent `MkdirAll`) and re-`fsync`s the chain rather than assuming prior durability |
| 148 | AC-82 | Publication | First-ever `capture`/`record --resources` for a slug (no prior `artifacts/resource-captures/` tree), crash simulated between the tracked tree's `MkdirAll` and its parent-directory `fsync` completing | Retried invocation recovers cleanly — re-running the idempotent `MkdirAll`, re-`fsync`ing, and proceeding to §7.3 steps 2-4 exactly as if the tree had always existed |
| 149 | AC-83 | Output cap | Simulated Dolt child writes 6 MiB combined stdout+stderr | Process group killed (`SIGTERM` then `SIGKILL`), refused `resource-limit-exceeded` (exit 3), JSON parser never invoked at all for the over-cap invocation |
| 150 | AC-84 | Output cap | Stdout alone 4 MiB, stderr alone 4 MiB (each under cap, combined over) | Refused `resource-limit-exceeded` — proves one shared budget, not two independent 5 MiB budgets |
| 151 | AC-85 | Process group | `cmd.SysProcAttr{Setpgid: true}` set before `cmd.Start()` on `linux`/`darwin`, inspected | Confirmed set; a timeout/cap-triggered kill signals the negative PGID with `SIGTERM`, then holds the leader **unreaped** through the full grace period (no `cmd.Wait()` call), then `SIGKILL`s the group (tolerating `ESRCH`), then reaps via `cmd.Wait()` |
| 152 | AC-85 | Process group | Test Dolt-adapter stub spawns a descendant that ignores `SIGTERM` and closes its own pipes; a timeout/cap kill is triggered | The descendant is eventually terminated by the group `SIGKILL` (not orphaned, not reaped early); the parent `tpatch` process itself is provably unaffected |
| 153 | AC-86 | Diff | Directory `ignored-file`, one file's mode changed, content/byte_count unchanged (chmod-only) | `combined_hash` differs on next `capture`/`diff`; `diff` reports that entry's `mode` as the differing field, `hash`/`byte_count` unchanged for that entry |
| 154 | AC-87 | Publication | Resource captured with content `A`, then `B`, then `A` again (three `capture` invocations) | Exactly two distinct `batches/<id>.json` files exist after the third invocation, not three; `current.json` repoints to the pre-existing `A` batch without creating a new batch file |
| 155 | AC-88 | Manifest | Adding a resource whose recomputed `resource_id` matches an existing `resources.json` entry with byte-identical canonical declaration bytes | Idempotent (exit 0, no duplicate entry) |
| 156 | AC-88 | Manifest | Adding (or loading, via any verb reading `resources.json`) an entry whose recomputed `resource_id` matches a **different, independently-correct** existing entry's recorded ID with different canonical bytes (via a test-only stub hash-collision seam) | Refused `resource-id-collision` (exit 3) — narrowed in rev-7 to two-distinct-declarations only |
| 157 | AC-89 | Golden ID | Recompute the two-file golden directory vector (`config/a.txt` empty, `config/sub/b.sh` with known content) via a reference implementation of the `path`+`0x00`+`mode`+`0x00`+`hash`+`0x00` tuple-encoding rule | Matches the documented `combined_hash` `5af4d6754656795b49c6e22acc2034ed6a2b3426470b0c42156f5ad0b4bcb9ad` exactly |
| 158 | AC-90 | Trust | `capture` for an `adapter-snapshot`/`dolt` resource with `trust: null` | Refused `dolt-trust-required` (exit 3) before any Dolt-binary descriptor is opened |
| 159 | AC-91 | Trust | `add --kind adapter-snapshot --adapter dolt --arg contract=dolt-diff-summary-v2` (unsupported value) | Refused `dolt-contract-unsupported` (exit 2), before `db_path`/`table`/`from`/`to` validation |
| 160 | AC-91 | Trust | `contract=dolt-diff-summary-v1` present in two differently-ordered `args` declarations | Both recompute to the same `resource_id` (identity participation confirmed) |
| 161 | AC-92 | Trust | `trust-dolt <slug> <resource-id> --binary-sha256 <64hex>` on an existing Dolt resource | Only `trust.binary_sha256` changes; `resource_id`, `current.json`, and every `batches/<id>.json` file are byte-for-byte unchanged, confirmed via before/after `list --json`/`diff` comparison |
| 162 | AC-93 | Trust | Private Dolt-binary copy's permissions/lifetime inspected across success and failure invocations | Created, hashed, hardened to `0500` only after a matching digest, and deleted (best-effort) after the child exits on both outcomes |
| 163 | AC-94 | Trust | Test double records `cmd.Path`/observed `argv[0]` for a Dolt invocation | Equals the private copy's own ephemeral path, never the originally `LookPath`/`EvalSymlinks`-resolved pathname |
| 164 | AC-95 | Manifest | A hand-constructed `resources.json` entry whose own recorded `resource_id` does not match its own freshly-recomputed value | Refused `resources-file-corrupt` (exit 3) at load time — distinct from `AC-88`'s two-declaration collision |
| 165 | AC-96 | Process group | Instrumented test double asserts no `cmd.Wait()` call occurs between the `SIGTERM` and the post-grace `SIGKILL`, exercised via both the timeout/cap-overflow trigger and the ordinary successful-completion trigger through the identical code path | No `Wait()` observed until after `SIGKILL` is issued, in either case; exactly one `Wait()` call per invocation |
| 166 | AC-97 | Process group | Direct child closes both pipes (EOF) while a lingering descendant or the leader itself remains alive doing further work | Forcibly terminated via the unified `SIGTERM`→grace→`SIGKILL` sequence regardless; surfaces as `dolt-query-error` (no new refusal name); elapsed time between EOF and `Wait()` is never less than the grace duration, even for an otherwise-instant success |
| 167 | AC-98 | Scratch | Leaf-targeted **ignore** half of the gate evaluated for `.tpatch/local/resource-scratch/<slug>/` both when the leaf directory exists and when it does not (fresh clone) | Identical refusal/pass outcome in both cases; distinguished from the `statfs` preflight, which necessarily targets the nearest existing ancestor |
| 168 | AC-99 | Scratch | Retried invocation after a simulated first-attempt crash, with some directories in the chain already `Stat`-visible from the failed attempt | Every directory in the chain (local scratch and tracked `artifacts/resource-captures/`/`batches/`) is unconditionally re-`fsync`'d, not only directories `MkdirAll` reports as newly created on the retry |
| 169 | AC-100 | Filesystem | Stubbed `linux/s390x` `Statfs_t.Type` (`uint32`) compared against the `uint32`-typed allow/deny constants | Comparison succeeds identically to `linux/amd64`/`arm64` (`int64`) and `linux/386`/`arm` (`int32`) stubs for the same filesystem magic number, confirming architecture-independent normalization |
| 170 | AC-101 | Scratch | A tracked file planted under a *different* slug's scratch tree than the one being mutated (`.tpatch/local/resource-scratch/other-slug/leftover`) | Refused `local-path-tracked` for the slug currently being mutated too — untracked half is whole-`.tpatch/local/`-subtree-scoped, not per-slug-leaf-scoped, via plain `git --literal-pathspecs ls-files -- .tpatch/local/` with empty-stdout convention |
| 171 | AC-102 | Trust | `add --kind adapter-snapshot --adapter dolt ... --trust-current-dolt` for a resource with no prior `trust` entry at all | Succeeds, records `trust.binary_sha256`; test double asserts zero process-start calls occur during `add`, with or without the flag |
| 172 | AC-103 | Trust | Duplicate `add` of an identical declaration, re-passing `--trust-current-dolt` while the currently-resolved Dolt binary's hash differs from the stored pin | Strict no-op: `trust.binary_sha256` byte-for-byte unchanged; no copy/exec test-double calls observed |
| 173 | AC-104 | Trust | Stubbed private-copy scratch filesystem `statfs` result with the no-exec flag set (Linux `ST_NOEXEC`/Darwin `MNT_NOEXEC`) | Refused `adapter-copy-noexec` (exit 3) before any byte of the Dolt binary is copied |
| 174 | AC-105 | Trust | Test double injects `ENOSPC` then, in a second run, `EIO` during the streamed copy-while-hash step | Refused `adapter-copy-failed` (exit 1) in both cases; partial copy file removed (best-effort); no Dolt process started |

**174 rows** cover **105** distinct `AC` clauses; several clauses (e.g.
`AC-1`, `AC-2`, `AC-4`, `AC-6`, `AC-7`, `AC-10`, `AC-13`, `AC-17`,
`AC-20`, `AC-21`, `AC-22`, `AC-23`, `AC-25`, `AC-26`, `AC-28`, `AC-32`,
`AC-34`, `AC-36`, `AC-37`, `AC-39`, `AC-40`, `AC-43`, `AC-44`, `AC-47`,
`AC-50`, `AC-51`, `AC-55`, `AC-58`, `AC-60`, `AC-61`, `AC-64`, `AC-65`,
`AC-69`, `AC-74`, `AC-75`, `AC-77`, `AC-78`, `AC-80`, `AC-81`, `AC-85`,
`AC-88`, `AC-91`) are exercised by more than one row — this matrix
does not claim any clause is covered "exactly once." `AC-80` alone
contributes 18 of those 174 rows: 17 named allow/deny filesystem-type
fixtures (matching `AC-80`'s own "17 supporting Test Matrix rows"
text) plus 1 additional row for the unrecognized-type case its
definition text separately calls out as "also exercised here" — the
largest single-clause row count in this table.
