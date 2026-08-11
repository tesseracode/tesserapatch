# ADR-033 — Resource Capture Boundary (rev-3)

**Status**: Proposed — rev-3 (supersedes rev-2, writer commit
`c603b8f`(rev-1)/`4255bef`(rev-2), rev-2 adjudicated NEEDS REVISION →
REV-3 DISPATCHED at `4ea011e`; see `docs/supervisor/LOG.md`)

**Context**: `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
(rev-3, companion document — this ADR binds the decisions that PRD's
design depends on; read the PRD first for full rationale, this ADR
states the decisions themselves plus the Test Matrix).

**Related**: `ADR-027-capture-context-privacy-boundary.md` (D1–D6,
directly extended by D4 below), `ADR-030-multi-slug-reconcile-derivation-mode.md`
(`.git/**` exclusion precedent), `ADR-032-feature-unapply-state-boundary.md`
(fixed-struct JSON precedent), `docs/state-of-the-art/storage-substrate-and-versioned-data.md`
§3, §9 (tracked Dolt/substrate research), `internal/workflow/session_ignore.go`
(`EnsureLocalIgnoreContract`, reused by D8)

---

## Rev-3 fold summary

The rev-2 adjudication (`4ea011e`) found: the Dolt argv design still
used flag-based `--name-only`/`--schema`/`--data` templates instead of
the source-verified `dolt_diff_summary` SQL table function and left
`table` optional (allowing silent PK-change omission); the lock design
(`O_CREATE|O_EXCL` single file) had a partial-observation window; the
publication batch ID was random rather than content-addressed,
producing a "fresh ID on every retry" claim that was actually
incorrect for an idempotent retry; ephemeral scratch still wrote raw
ignored-file bytes and Dolt stdout to a file before scanning, rather
than scanning them in memory; `add`/`remove`/`clear` did not
participate in the per-slug lock; and `check-ignore` was invoked with
an invalid `--literal-pathspecs` flag that command does not accept.
This rev-3 rewrite resolves every finding; see the companion PRD's
§0.1 Claims Audit (C17–C24) for the corrected-citation table (not
repeated here to avoid drift — this ADR cites the PRD's rows by ID,
e.g. "C19," where relevant).

**Preserved across all four revisions (rev-1 through rev-3)**: a
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
- Every mutating verb (`add`/`remove`/`clear`/`capture`/
  `record --resources`) must serialize against every other mutating
  verb for the same feature slug, not just `capture` against itself.
- A crash at any point in the local-scratch, lock, or tracked-
  publication pipeline must leave the system in a state a subsequent
  invocation can recover from without manual intervention, and any
  residual gap the Go standard library cannot close must be stated
  honestly rather than glossed over.

## Decision

### D1 — Scope & authority (reaffirmed, unchanged across all four revisions)

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

### D3 — Resource ID: canonical-JSON args encoding + golden vectors (reaffirmed algorithm; vectors 2/3 recomputed for mandatory `db_path`)

The canonicalization algorithm itself is unchanged from rev-1/rev-2
(keys sorted byte-ascending, minimal `{"k":"v",...}` escaping, no
`encoding/json.Marshal` HTML-escaping, UTF-8 required with no NFC/NFD
normalization, `NUL`/C0 control bytes rejected at `add` time — PRD
§13.1–§13.2 for the full grammar). `resource_id := "res_" +
lowercase-hex(SHA-256(feature + "\x00" + kind + "\x00" + selector +
"\x00" + adapter + "\x00" + capability + "\x00" + canonical_args))[:12]`.
Vectors 1 (`git-metadata`/`head`) and 4 (`ignored-file`) are byte-
identical to rev-1/rev-2: `res_acc91dc23a8b` and `res_79f5ac5dca13`.
Vectors 2 and 3 (`adapter-snapshot`) are **recomputed again** — D6
below now makes `db_path` a mandatory Dolt-selector field, which adds
a new key to the hashed `args` payload. Both recompute to the same
value, `res_cf8e47e6564b`, which independently reconfirms order-
independence of the `args` canonicalization holds with the additional
field present (PRD §0.3, §13.3 — golden vector table reproduced there
byte-identical to this decision).

| Vector | Inputs (`feature`, `kind`, `selector`, `adapter`, `capability`, `args`) | `resource_id` |
|---|---|---|
| 1 | `model-picker`, `git-metadata`, `head`, ``, ``, `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker`, `adapter-snapshot`, `dolt:diff-summary:users`, `dolt`, `diff-summary`, `{"db_path":"data/users-db","table":"users","from":"main","to":"HEAD"}` (declared `db_path, table, from, to` order) | `res_cf8e47e6564b` |
| 3 | Same as Vector 2, `args` declared `to, db_path, table, from` order | `res_cf8e47e6564b` (**identical** — order-independence) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

### D4 — Full ADR-027 compliance: zero pre-scan persistence, no raw bytes ever written to disk (task 2, task 3)

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
`exec.Cmd.Stdout`/`Stderr` set to bounded in-memory
`*bytes.Buffer`s (capped, PRD §6.3) and parsed/scanned directly from
those buffers — never redirected to or copied into a file first. The
**only** on-disk scratch content that remains is Dolt's own ephemeral
`HOME`/`DOLT_ROOT_PATH` working directory, which Dolt itself may write
to as part of running the query (its own config/state, not the
captured diff content) — this is unavoidable since Dolt is an
external process, not something this ADR's own code writes, and it
never contains the tracked/scanned result data itself. That directory
is deleted (best-effort) at the end of every invocation, regardless of
`--dry-run`, success, or failure, exactly as in rev-2.

No tracked artifact ever contains raw bytes, raw stdout, or a
wall-clock timestamp (D10 below). `feature resource diff` recomputes
lightweight current metadata and compares it against the last tracked
batch's `result` — reporting metadata/hash/file-set-level changes
only, never a textual content diff (PRD §5.1, §2's non-goal); raw
content diffing/versioning remains explicitly deferred to a future ADR
that would have to supersede `ADR-027`'s committed/local split, which
this ADR does not attempt. There is no persistent local raw store of
any kind (rev-2's ephemeral-scratch-with-a-raw-file design is gone);
what remains is: (a) in-memory scanning that never touches disk for
captured content, and (b) orphan cleanup of Dolt's own scratch `HOME`
directory and the lock/batch temp artifacts at the next invocation
(PRD §7.1, §7.4). This design is grounded directly in `ADR-027` D3's
own binding language: "Local private buffers may keep only the
redacted or hashed form; this ADR does not authorize a tpatch-managed
raw transcript archive" (PRD C16,
`docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-170`) — an
in-memory-only design is the most literal possible compliance with
that sentence, stronger than rev-2's "ephemeral file, deleted after."

### D5 — Dolt adapter protocol: mandatory `db_path`/`table`, exact `dolt_diff_summary` SQL, no version probe (task 1, task 2, task 6, task 7, task 8)

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
this ADR. `WORKING`/`STAGED` (exact-case) are accepted as ordinary
`from`/`to` values without special-casing, confirmed as exact-case
string constants in `doltdb.go:51-52`, resolved via
`ResolveRootForRef` (PRD C19) — this is no longer an open question as
it was in rev-2.

The five-column result schema is source-confirmed non-null and typed
(`from_table_name`/`to_table_name` `LongText`, `diff_type` `Text`,
`data_change`/`schema_change` `Boolean`), and the function itself
reports `IsReadOnly() == true` — reinforcing this decision's read-only
framing. `dolt sql -r json` output is parsed with strict field
presence/type checking: the literal 2-byte string `{}` is zero rows;
a `{"rows":[...]}` envelope with any other/extra top-level key, a row
missing/duplicating a field, or a non-boolean `data_change`/
`schema_change` value is a fatal `dolt-json-parse-error` — never
silently coerced (PRD C22, source-confirmed no `"schema"` key exists
in this output format). `diff_type` is tracked as one of the closed
4-value enum `"added"`/`"modified"`/`"renamed"`/`"dropped"` (source-
confirmed, PRD C23); a rename surfaces via `diff_type: "renamed"` with
differing `from_table_name`/`to_table_name` on that row. A nonexistent
`table` (in neither `from` nor `to`) yields `result.tables: []` (zero
rows, PRD C21) — distinct from the PK-change hard error above.
Literal escaping refuses `NUL`/C0 control bytes, backslash, and `..`
outright (exit 2) and escapes only `'` → `''` (PRD §6.2) — deliberately
not a general SQL-injection-safe escaper, since backslash's escape-
character status inside a Dolt/MySQL string literal depends on
`sql_mode`, which this ADR does not control (unchanged reasoning from
rev-2).

**No version probe** (task 2, PRD C12, unchanged from rev-2): `dolt
version` is never invoked. Tool identity is `basename(resolvedPath)` +
`SHA-256` of the resolved binary's bytes (read, not executed) — the
absolute path is never tracked, only used in-process and never
persisted anywhere, tracked or local. The real `diff-summary` SQL
invocation is itself the only capability check; there is no separate
"probe" failure class. The invocation's environment contains only a
fresh, `0700` ephemeral `HOME` (and `DOLT_ROOT_PATH` pointing at the
same tree) — no inherited variable from the invoking process, no
`PATH`, no credentials of any kind (PRD §6.1).

### D6 — Executable and path safety: descriptor-identity gate for selectors, opposite-direction policy for the Dolt binary (task 2, task 4)

`safety.EnsureSafeRepoPath`/`store.NormalizeClaimPath` remain
lexical-only (PRD C2, unchanged citation, unchanged across all
revisions). Two separate policies for two separate trust directions,
both unchanged in structure from rev-2 except for one fix (below):

- **Selector paths that must stay inside the repo** (`ignored-file`
  selectors, and now Dolt's `db_path`): `Lstat` every path component
  from the repo root down to the leaf; refuse
  (`symlink-component-refused`, exit 3) if **any** component —
  ancestor or final — is a symlink, regardless of where it points. The
  final open uses `syscall.O_NOFOLLOW` as a real, available hardening
  layer against a final-component race. Rev-3 replaces rev-2's
  post-open pathname re-`Lstat` with a genuine descriptor-level check:
  `os.SameFile(preOpenInfo, openFile.Stat())`, comparing the
  `FileInfo` obtained from the pre-open `Lstat` against the `FileInfo`
  obtained from `Stat()` on the **already-open file descriptor** —
  this is a real check against the actually-opened inode, not a second
  pathname lookup that could itself race a replacement between the
  two `Lstat` calls. A pathname re-`Lstat` is retained only as a
  secondary, defense-in-depth signal, not the primary check. This gate
  re-runs at both `add` and every `capture`, independently for every
  descendant file of a directory selector, and independently for
  `db_path`.
- **Dolt executable paths** (must stay outside the repo, unchanged
  from rev-2): symlinks ARE followed (`filepath.EvalSymlinks`); the
  *resolved* target must be a regular, executable file located outside
  the repository working tree and outside any `.git` directory —
  refused (`adapter-executable-in-repo`, exit 3) if not. A cheap
  post-invocation `Lstat` compares device/inode/size/mtime against the
  pre-invocation values to detect a mid-invocation replacement
  (`adapter-executable-replaced`, exit 3, result discarded).

**Residual honestly stated** (unchanged from rev-2, PRD C14): Go's
standard library has no portable equivalent of `openat2`/
`RESOLVE_NO_SYMLINKS` that atomically binds every ancestor directory
component against a race between the `Lstat` walk and the final open;
`os.SameFile` on the open descriptor closes the *file-identity* half
of this race completely (it is checking the real, already-open inode),
but a sufficiently well-timed attacker replacing an ancestor
**directory** itself between the walk and the open is still not fully
closed using only the standard library. This ADR does not claim an
impossible sandbox guarantee — it documents this as an accepted,
stated v1 residual risk, now narrower than rev-2's since the
final-component identity check is no longer itself vulnerable to a
second race.

### D7 — One tracked publication point per capture, content-addressed batch ID (task 3, task 4, task 9)

Rev-2 introduced the batch-then-pointer design but used a random
`crypto/rand` batch ID, meaning an idempotent retry of unchanged
content produced a *different* `batch_id` every time — rev-2's own
changelog claimed this was intentional ("fresh ID on every retry"),
which the rev-2 adjudication correctly identified as neither
idempotent nor a real transaction guarantee. Rev-3 replaces this with
a content-addressed ID:

- **`batches/<batch_id>.json`** — one immutable file per successful
  capture invocation, containing every resource result that
  invocation produced (PRD §12.3). `batch_id := "rb_" +
  lowercase-hex(SHA-256(CanonicalBatchJSON({"feature": feature,
  "results": sorted_by_resource_id})))[:12]` — a **content hash**, not
  a random ID. Written via temp-file-then-rename with an `fsync` of
  the file before rename and the containing directory after, using
  `O_EXCL` on the temp name. If `batches/<batch_id>.json` already
  exists with **identical bytes**, the write step is skipped entirely
  (idempotent re-publish, proceeding directly to the pointer step); if
  it exists with **different** bytes under the same ID (a hash
  collision, expected unreachable in practice), the invocation refuses
  with `batch-id-collision` (exit 3) rather than silently overwriting.
- **`current.json`** — one atomically-rewritten pointer, a
  `latest_batch_id` plus a sorted `[]{resource_id, batch_id}` array
  mapping every currently-declared resource to the batch holding its
  latest result (PRD §12.4). Also written via temp-file-then-rename
  with the same `fsync` discipline. **This rename is the single,
  atomic commit point of the entire capture** — nothing is visible to
  a reader until it succeeds.

`resources.json` (the declaration manifest) is explicitly **not** part
of this transaction — `add`/`remove`/`clear` only ever rewrite
`resources.json` and prune `current.json`'s live index (under the same
per-slug lock as `capture`, D9); they never touch any `batches/<id>.json`
file, which is permanent, immutable history regardless of whether its
resource is later undeclared.

**Crash windows**: before the batch rename — nothing changed, safe
retry recomputes the identical content-addressed `batch_id` and
resumes via the idempotent-skip branch above; after the batch rename
but before the pointer rename — a permanently orphaned, harmless batch
file no reader ever surfaces (not garbage-collected in v1); during the
pointer's temp-write — orphaned `.tmp` swept at next invocation, prior
`current.json` remains authoritative; after the pointer rename — fully
committed. Every crash window is now recoverable via a plain re-run
with unchanged content, without any dedicated recovery command, since
the batch ID is reproducible rather than random.

### D8 — Ignored/tracked Git gates: fixed `check-ignore` invocation, literal pathspecs elsewhere, local-ignore reuse (task 1, task 6)

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
(refused when combined with "ignored"), exit 1 with the standard
"did not match any file(s) known to git" shape = untracked (gate
passes), any other exit/output shape = fatal Git error (refused). A
selector is refused unless it is **both** ignored **and** untracked,
rechecked at `add` and every `capture`. The ephemeral-scratch root
itself is verified via the **existing**
`workflow.EnsureLocalIgnoreContract(repoRoot, resourceScratchRoot)`
(PRD C13, `internal/workflow/session_ignore.go:138`, reused rather
than reinvented — its own internal `check-ignore` call is the same
deliberate pathname exception to the literal-pathspec rule described
above) plus the same tracked-file gate layered on top, verified before
any scratch content (including the lock) is created for the first time
in an invocation.

### D9 — Lock semantics: atomic directory-rename acquisition, crash-safe, PID-reuse-guarded, serializes every mutator (task 3, task 5)

Rev-2's `O_CREATE|O_EXCL` single-file lock had a window where a
concurrent reader could observe a `.lock` file that existed but whose
JSON body was not yet fully written — not truly atomic. Rev-3 replaces
this with a temp-directory-then-atomic-rename design that has no such
window:

1. Compute owner metadata (`{pid, process_start, host}`, where
   `process_start` is `ps -o lstart= -p <pid>` captured before any lock
   artifact is created) up front.
2. Create `.tpatch/local/resource-scratch/<slug>/.lock.tmp-<nonce>/`
   (`0700`), write `owner.json` (`0600`) inside it, `fsync` the file,
   then `fsync` the temp directory.
3. `os.Rename(".lock.tmp-<nonce>", ".lock")` — a single atomic
   directory rename; POSIX guarantees this either succeeds completely
   (the full `.lock/owner.json` becomes visible atomically) or fails
   completely (`EEXIST`/`ENOTEMPTY` if `.lock` already exists) — no
   partial-observation window is possible for a concurrent reader.

On contention (an existing `.lock`): `host` mismatch → refuse
(`capture-lock-held-remote`, exit 3), no reclaim attempt; `host` match,
`ps -o lstart= -p <pid>` returns nothing (dead) → quarantine
(`os.Rename(".lock", ".lock.stale-<12hex>")`) and retry the acquisition
once; returns a **different** start-time string for the same PID
(reuse) → quarantine and retry once; malformed `owner.json` beyond a
short initialization grace period → quarantine and retry once; returns
the **same** start-time string → refuse immediately
(`capture-in-progress`, exit 3), no blocking/wait, no timeout flag in
v1; a second retry that itself loses the rename race (another process
already reclaimed) → refuse (`capture-lock-contended`, exit 3), not
retried a second time. Quarantined `.lock.stale-*` directories are
swept only by an invocation that has itself already acquired the live
`.lock` (never by `add`/`remove`/`clear`, which acquire the lock but
never sweep). Release is `os.RemoveAll(".lock")`, best-effort, as the
last step of every invocation. Windows liveness/reuse detection is
best-effort/unsupported in v1 (consistent with this project's existing
macOS/Linux-only validation-scope precedent).

**Every mutating verb serializes on this lock** (task 5, new this
revision): `add`, `remove`, `clear`, `capture`, and
`record --resources` all acquire the same per-slug lock before
touching `resources.json`/`current.json`/`batches/`. Only `capture`
and `record --resources` create ephemeral scratch content or perform
the orphan sweep — `add`/`remove`/`clear` acquire and release the lock
around a simple, fast manifest rewrite and never sweep or create
scratch. `list` never acquires the lock; it always observes either the
fully-prior or fully-new `resources.json`/`current.json` content
(guaranteed by D7's atomic-rename publication), never a torn read.

### D10 — Permissions and no tracked timestamps (unchanged from rev-2, task 13)

Ephemeral scratch directories (including the lock temp/canonical
directory and Dolt's own `HOME`) are created `0700` and files `0600`
at creation (never via a separate `chmod` after a looser-permission
create). Tracked artifacts (`resources.json`, `batches/<id>.json`,
`current.json`) use ordinary repository file permissions (`0644`),
since they never contain raw bytes or secrets by construction. No
tracked artifact contains a wall-clock timestamp field anywhere;
ordering is conveyed by the append-only batch file layout and
`current.json`'s pointer, not by a clock reading. Failure diagnostics
on a failed capture are printed directly to the CLI's own output only
— since there is no persistent local raw-content scratch tree in
rev-3 (D4), there is nothing left for a "written inside scratch"
diagnostic to be, closing rev-2's residual ambiguity there.

### D11 — Wire canonicalization: content-addressed batch ID via `CanonicalBatchJSON`, `record --resources` two-domain framing retained (task 4, task 12, task 17)

Every tracked JSON field that would otherwise be a Go `map` (`args`,
the per-resource index in `current.json`) is a sorted `[]struct` array
(`[]{key, value}` / `[]{resource_id, batch_id}`), so no tracked
artifact's determinism depends on `encoding/json`'s map-key-sort
behavior (unchanged from rev-2). A new, distinct canonical encoder,
`CanonicalBatchJSON` (separate from resource-ID's `CanonicalArgsJSON`),
recursively encodes a batch's `{feature, results}` body — strings,
booleans, `null`, non-negative integers, arrays (each with an
explicit, documented sort rule), and fixed-field objects in
struct-declared field order (never a Go `map`) — and is the sole input
to D7's content-addressed `batch_id`. `record --resources` retains the
two-atomic-domain framing from rev-1/rev-2 (Git-side capture and
resource-domain publication never commit together): staging remains
purely in-memory/ephemeral (never writes a batch file until Git
succeeds); on Git success, staging's already-computed candidate content
is published through the exact D7 batch-then-pointer sequence (which
is itself now idempotent under retry, D7); on Git failure, the
candidate is simply discarded. A `resource-domain-incomplete` (exit 1)
partial result carries an explicit, idempotent recovery command
(`tpatch feature resource capture <slug>`), which — because the batch
ID is content-addressed — is now safely re-runnable any number of
times without producing divergent batch history.

## Wire Schema Appendix

Byte-identical to the companion PRD's §12.2–§12.4 (verified
programmatically, not just visually).

```json
{
  "resources": [
    {
      "resource_id": "res_cf8e47e6564b",
      "kind": "adapter-snapshot",
      "selector": "dolt:diff-summary:users",
      "adapter": "dolt",
      "capability": "diff-summary",
      "args": [
        { "key": "db_path", "value": "data/dolt-db" },
        { "key": "from", "value": "main" },
        { "key": "table", "value": "users" },
        { "key": "to", "value": "HEAD" }
      ],
      "added_by_tool_version": "tpatch/0.13.0"
    },
    {
      "resource_id": "res_79f5ac5dca13",
      "kind": "ignored-file",
      "selector": "config/local-secrets.env.template",
      "adapter": "",
      "capability": "",
      "args": [],
      "added_by_tool_version": "tpatch/0.13.0"
    }
  ]
}
```

```json
{
  "batch_id": "rb_5cff7f222dce",
  "feature": "model-picker",
  "results": [
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
    },
    {
      "resource_id": "res_cf8e47e6564b",
      "kind": "adapter-snapshot",
      "selector": "dolt:diff-summary:users",
      "adapter": "dolt",
      "capability": "diff-summary",
      "args": [
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
    }
  ]
}
```

```json
{
  "latest_batch_id": "rb_5cff7f222dce",
  "resources": [
    { "resource_id": "res_79f5ac5dca13", "batch_id": "rb_5cff7f222dce" },
    { "resource_id": "res_acc91dc23a8b", "batch_id": "rb_5cff7f222dce" },
    { "resource_id": "res_cf8e47e6564b", "batch_id": "rb_5cff7f222dce" }
  ]
}
```

## Implementation Notes

1. `internal/redact.Scan([]byte) []string` is a new exported package;
   the existing `internal/cli/session_redaction.go` matchers for
   bearer/token/key prefixes and absolute-home-paths are extracted
   into it, not duplicated, and is invoked directly against in-memory
   buffers (D4) — it never receives a file path.
2. `ps -o lstart= -p <pid>` is invoked via `exec.Command`, consistent
   with this project's existing pattern of shelling out to system
   utilities (`git`, `dolt`) rather than adding a dependency.
3. The `dolt_diff_summary` query is built via `fmt.Sprintf` over the
   already-validated/escaped `from`/`to`/`table` strings — validation
   (D5) happens strictly before string construction, never after, and
   `table`/`db_path` are mandatory, never optional, fields on the Go
   struct representing a Dolt resource declaration.
4. `es_<id>`/lock-quarantine-nonce suffixes reuse the existing
   `crypto/rand`, 12-lowercase-hex-character ID convention already
   established elsewhere in this project (`ua_` attempt IDs); `rb_<id>`
   is the one exception — it is content-addressed (`SHA-256`-derived,
   D7), not `crypto/rand`-derived, and this distinction is deliberate,
   not an oversight.
5. `CanonicalBatchJSON` (D11) is implemented as its own function,
   distinct from `CanonicalArgsJSON` (D3) — the two encoders share no
   code, since they encode structurally different shapes (a flat
   sorted key/value list vs. a nested `{feature, results[]}` object
   with per-result fixed-field structs), even though both follow the
   same no-`encoding/json.Marshal`, no-HTML-escaping, deterministic-
   ordering philosophy.
6. `os.SameFile` (D6) is the primary descriptor-identity check,
   applied to the `os.FileInfo` returned by `Stat()` on an
   already-open `*os.File` compared against the pre-open `Lstat`
   result — this is a real `fstat`-equivalent on the actual open
   descriptor, not a second pathname lookup.
7. The lock directory rename (D9) relies on `os.Rename`'s POSIX
   atomicity guarantee for renaming a directory onto an existing
   non-empty path failing with `ENOTEMPTY`/`EEXIST` rather than
   partially merging — this is standard POSIX `rename(2)` behavior on
   both macOS and Linux, the project's two supported platforms.

## Negative Consequences Summary

- Ancestor-directory TOCTOU is a documented residual risk, narrower
  than rev-2's (the file-identity check no longer itself races), but
  not fully closed (D6).
- No raw content diffing/versioning in v1 — only metadata/hash/
  file-set-level change detection (D4).
- Windows lock liveness/reuse detection is best-effort/unsupported
  (D9).
- A feature with many resources cannot parallelize its own staging
  across multiple processes — the lock is per-slug, and now also
  serializes `add`/`remove`/`clear` against `capture` (D9).
- Mandatory `table` (D5) forecloses a convenient whole-database Dolt
  diff in v1 — a resource declaration must enumerate every table it
  cares about, trading that inconvenience for a hard PK-change error
  instead of a silent omission.

## Test Matrix

| # | AC | Area | Scenario | Expected |
|---|----|------|----------|----------|
| 1 | AC-1 | Dolt | Capture a declared `diff-summary` resource | Exact argv `<dolt> sql -r json -q "..."` invoked, one `dolt_diff_summary(from,to,table)` shape, no other Dolt subcommand |
| 2 | AC-1 | Dolt | Same, `--json` output | Tracked `result.tables` matches the SQL row set |
| 3 | AC-2 | Dolt | `--arg from=$'\x00'` | Exit 2, no SQL constructed |
| 4 | AC-2 | Dolt | `--arg to=a\\b` (backslash) | Exit 2 |
| 5 | AC-2 | Dolt | `--arg from=main..HEAD` (contains `..`) | Exit 2, refused before reaching Dolt |
| 6 | AC-3 | Dolt | `--arg from="O'Brien"` | Single quote doubled, query succeeds |
| 7 | AC-4 | Dolt | Missing `db_path` | Exit 2 |
| 8 | AC-4 | Dolt | `--arg table=x --arg table=y` | Exit 2, duplicate key |
| 9 | AC-4 | Dolt | `--arg unexpected=z` | Exit 2, unknown key |
| 10 | AC-5 | Dolt | Table renamed between `from`/`to` | `diff_type: "renamed"`, `from_table_name` != `to_table_name` tracked verbatim on one row |
| 11 | AC-6 | Dolt | `-r json` row missing `schema_change` | Fatal `dolt-json-parse-error` |
| 12 | AC-6 | Dolt | `-r json` row has extra unknown field | Fatal `dolt-json-parse-error` |
| 13 | AC-6 | Dolt | `data_change` is a JSON string, not boolean | Fatal `dolt-json-parse-error`, no coercion attempted |
| 14 | AC-7 | Dolt | `dolt sql -r json` returns literal `{}` | Parsed as `result.tables: []` |
| 15 | AC-7 | Dolt | `-r json` returns `{"rows":[...],"extra":1}` | Fatal parse error, extra top-level key refused |
| 16 | AC-8 | Dolt | PK set changed on the mandatory `table` between `from`/`to` | `dolt-query-error`, exit 3 |
| 17 | AC-9 | Dolt | `table` absent from both `from` and `to` | `result.tables: []`, no error |
| 18 | AC-10 | Dolt | `--arg from=WORKING --arg to=STAGED` | Accepted, no special-casing at validation, query succeeds |
| 19 | AC-11 | Dolt | `--arg from=main..HEAD` | Refused at `AC-2`/`AC-5` before any Dolt invocation; dot-range form never exercised |
| 20 | AC-12 | Dolt | First-ever capture of a fresh `adapter-snapshot` resource | Identical schema shape to a later capture; zero-row shape matches nonexistent-table shape |
| 21 | AC-13 | Dolt | Any `capture` invocation | `dolt version` never appears in the invoked-process log |
| 22 | AC-14 | Dolt | Successful capture | `tool_identity` = `{basename, binary_sha256}` only, no path field present, in every tracked file |
| 23 | AC-15 | Dolt | Inspect the Dolt child process's environment | Exactly `HOME`, `DOLT_ROOT_PATH` (both scratch); nothing inherited, no `PATH` |
| 24 | AC-16 | Dolt | `dolt` resolves to a path under the repo working tree | Refused `adapter-executable-in-repo` |
| 25 | AC-16 | Dolt | `dolt` resolves under a `.git` directory anywhere | Refused `adapter-executable-in-repo` |
| 26 | AC-17 | Dolt | Binary replaced mid-invocation (simulated) | Refused `adapter-executable-replaced`, result discarded |
| 27 | AC-18 | Privacy | Ignored-file content read for scanning | No file under `resource-scratch/<slug>/` ever contains the raw content bytes |
| 28 | AC-19 | Privacy | Dolt stdout captured for scanning | Stdout never redirected to or copied into a scratch file |
| 29 | AC-20 | Privacy | One resource's content matches a PEM key | Whole invocation refused, no batch written for any resource, no unredacted byte written anywhere |
| 30 | AC-20 | Privacy | One resource's content matches a DB URL | Same refusal behavior |
| 31 | AC-20 | Privacy | Content matches none of the six classes | Capture proceeds normally |
| 32 | AC-21 | Wire | Inspect every tracked JSON field | No timestamp-shaped field anywhere |
| 33 | AC-22 | Diff | `ignored-file` content changed since last batch | `diff` reports which of hash/size/etc. changed, no textual diff |
| 34 | AC-22 | Diff | `ignored-file` unchanged | `diff` reports `unchanged` |
| 35 | AC-23 | Path | Ancestor directory of selector is a symlink | Refused `symlink-component-refused` |
| 36 | AC-23 | Path | Ancestor symlink points inside the repo | Still refused (no target inspection) |
| 37 | AC-23 | Path | `db_path` ancestor is a symlink | Refused `symlink-component-refused`, same gate as an `ignored-file` selector |
| 38 | AC-24 | Path | Final component replaced by symlink between walk and open | Refused via `O_NOFOLLOW`/`ELOOP` |
| 39 | AC-25 | Path | File replaced (different inode) between walk and open | Refused `path-replaced-during-open`, detected via `os.SameFile` on the open descriptor |
| 40 | AC-26 | Path | Missing ancestor component | Refused `path-missing` |
| 41 | AC-27 | Path | Directory selector, one descendant symlinked | That descendant refused, others unaffected |
| 42 | AC-27 | Path | Same selector re-`add`ed after fix | Gate re-verified at `add` |
| 43 | AC-28 | Path | Dolt executable itself is a symlink to an external binary | Followed via `EvalSymlinks`, not refused by the ancestor-symlink rule |
| 44 | AC-29 | Git gate | `check-ignore` invocation inspected | Never includes `--literal-pathspecs` |
| 45 | AC-30 | Git gate | `check-ignore` exit 1 | Refused `not-ignored` |
| 46 | AC-30 | Git gate | `check-ignore` exit 128 (fatal) | Refused `git-ignore-check-error`, distinct reason |
| 47 | AC-31 | Git gate | Selector `:(glob)config/*.env` | Passed to `check-ignore` with `./` prefix, resolves to the same on-disk path |
| 48 | AC-31 | Git gate | Selector `:(literal)config/name.env` (second independently-confirmed-fatal magic keyword) | Passed to `check-ignore` with `./` prefix, resolves to the same on-disk path; supervisor-independently reconfirmed non-fatal outcome |
| 49 | AC-32 | Git gate | Selector containing `*`/`?`/`[]` | No wildcard/glob matching occurs for `check-ignore` |
| 50 | AC-33 | Git gate | `ls-files --error-unmatch` exit 0 (tracked) | Refused `tracked-and-ignored` |
| 51 | AC-33 | Git gate | `ls-files --error-unmatch` unexpected stderr | Refused `git-ls-files-error`; call verified to use `--literal-pathspecs` |
| 52 | AC-34 | Git gate | Selector ignored but tracked | Refused; both checks must pass |
| 53 | AC-35 | Local ignore | Scratch root verified via `EnsureLocalIgnoreContract` | Refused before scratch content created if the contract fails |
| 54 | AC-36 | Local ignore | `.tpatch/local/` accidentally tracked | Refused `local-path-tracked` via the `ls-files` gate on the root |
| 55 | AC-37 | Lock | Fresh `capture` invocation | `.lock.tmp-<nonce>/owner.json` written+fsynced, then renamed to `.lock` atomically |
| 56 | AC-38 | Lock | Second concurrent `capture` for same slug | Refused `capture-in-progress`, no blocking |
| 57 | AC-39 | Lock | Lock PID no longer exists | Quarantined and reclaimed automatically |
| 58 | AC-40 | Lock | Lock PID alive, different `process_start` | Quarantined and reclaimed automatically (PID reuse) |
| 59 | AC-41 | Lock | `owner.json` is malformed JSON | Quarantined and reclaimed |
| 60 | AC-42 | Lock | Lock `host` differs from current host | Refused `capture-lock-held-remote`, no reclaim attempt |
| 61 | AC-43 | Lock | Reclaim retry itself loses the rename race | Refused `capture-lock-contended`, not retried again |
| 62 | AC-44 | Lock | `add`/`remove`/`clear` invoked | Same per-slug lock acquired before manifest rewrite |
| 63 | AC-45 | Lock | `add` invoked with orphaned scratch present | No scratch content created, no orphan sweep performed |
| 64 | AC-46 | Lock | `list` invoked during a concurrent `capture` | Observes fully-prior or fully-new content, never a torn read |
| 65 | AC-47 | Permissions | Every scratch directory/lock directory created | Mode `0700` at creation, no later `chmod` call observed |
| 66 | AC-47 | Permissions | Every scratch file created | Mode `0600` at creation |
| 67 | AC-48 | Scratch | Orphaned `es_*`/`.lock.stale-*`/`.tmp-*` from simulated crash | Swept only after the sweeping invocation has itself acquired the live lock |
| 68 | AC-49 | Scratch | `add` invoked with orphans present | Orphan sweep never runs during `add` |
| 69 | AC-50 | Publication | Multi-resource successful capture | Exactly one new `batches/<id>.json`, `current.json` rewritten once |
| 70 | AC-51 | Publication | Recompute `batch_id` from batch content | Identical `batch_id` reproduced |
| 71 | AC-52 | Publication | Retry with identical batch content | Existing `batches/<id>.json` reused, skip to pointer publish |
| 72 | AC-53 | Publication | Same `batch_id`, different content (simulated collision) | Refused `batch-id-collision` |
| 73 | AC-54 | Publication | Crash simulated after batch rename, before pointer rename | Orphaned batch never surfaced; re-run recomputes identical `batch_id` and proceeds |
| 74 | AC-55 | Publication | Crash simulated during batch/pointer temp-write | `.tmp-*` swept at next invocation; no effect on last-committed `current.json` |
| 75 | AC-56 | Manifest | `remove <id>` | `resources.json`/`current.json` updated under lock; all `batches/*.json` untouched |
| 76 | AC-56 | Manifest | `clear` | Same, for every declared resource |
| 77 | AC-57 | Read path | `list`/`diff` invoked | Only `current.json` consulted, `batches/` never scanned directly |
| 78 | AC-58 | Metadata | HEAD detached | `symbolic_ref` is `null`, `detached` is `true` |
| 79 | AC-58 | Metadata | HEAD on a branch | `symbolic_ref` populated, `detached` is `false` |
| 80 | AC-59 | Metadata | `config` view with key `user.email` | Refused, exit 2 |
| 81 | AC-59 | Metadata | `config` view with `core.filemode` | Accepted |
| 82 | AC-60 | Metadata | `index-entry` selector `:(icase)Foo` | Resolved as the literal path under `--literal-pathspecs` |
| 83 | AC-61 | Wire | Directory `ignored-file` resource captured | `files[]` present, `path`-sorted, `{path,raw_sha256,byte_count,mode}` per entry |
| 84 | AC-62 | Wire | Each of `head`/`ref`/`index-entry`/`config`/`ignored-file`(single+dir)/`adapter-snapshot` captured | Every variant's exact tagged shape matches §12.2 |
| 85 | AC-63 | Dry-run | `feature resource capture <slug> --dry-run` | Zero tracked writes; lock/scratch removed identically to a real capture |
| 86 | AC-64 | Record | `record --resources` on feature with zero declared resources | Refused `no-resources-declared` before lock/Git |
| 87 | AC-65 | Record | Staging fails, Git succeeds | `resource-domain-incomplete`, recovery command shown, Git patch intact |
| 88 | AC-65 | Record | Same, `--json` output | Same fields present in JSON envelope |
| 89 | AC-66 | Record | Staging fails, Git fails | Only record's existing Git-failure behavior surfaces, no double-report |
| 90 | AC-67 | Record | Staging succeeds, Git succeeds | Batch + pointer published together, verified atomically |
| 91 | AC-68 | Record | Re-run `capture`/`record --resources` after publish-step failure, content unchanged | Identical `batch_id` reproduced, idempotent skip-branch taken |
| 92 | AC-68 | Record | Re-run after genuinely changed content | Different `batch_id` produced |
| 93 | AC-69 | Golden ID | Recompute Vector 1 | Matches `res_acc91dc23a8b` |
| 94 | AC-69 | Golden ID | Recompute Vector 2 | Matches `res_cf8e47e6564b` |
| 95 | AC-69 | Golden ID | Recompute Vector 3 (reordered args) | Matches Vector 2 exactly |
| 96 | AC-69 | Golden ID | Recompute Vector 4 | Matches `res_79f5ac5dca13` |
| 97 | AC-70 | Golden ID | Recompute the worked batch example's `batch_id` | Matches `rb_5cff7f222dce` |

**97 rows** cover **70** distinct `AC` clauses; several clauses (e.g.
`AC-6`, `AC-9`, `AC-16`, `AC-20`, `AC-23`, `AC-30`, `AC-31`, `AC-33`,
`AC-47`, `AC-58`, `AC-59`, `AC-65`, `AC-68`, `AC-69`) are exercised by
more than one row — this matrix does not claim any clause is covered
"exactly once."
