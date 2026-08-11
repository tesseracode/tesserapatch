# ADR-033 — Resource Capture Boundary (rev-2)

**Status**: Proposed — rev-2 (supersedes rev-1, writer commits
`e8572b2`/`f0f2c1f`, adjudicated NEEDS REVISION → REV-2 DISPATCHED at
`173bb3c`; see `docs/supervisor/LOG.md`)

**Context**: `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
(rev-2, companion document — this ADR binds the decisions that PRD's
design depends on; read the PRD first for full rationale, this ADR
states the decisions themselves plus the Test Matrix).

**Related**: `ADR-027-capture-context-privacy-boundary.md` (D1–D6,
directly extended by D4 below), `ADR-030-multi-slug-reconcile-derivation-mode.md`
(`.git/**` exclusion precedent), `ADR-032-feature-unapply-state-boundary.md`
(fixed-struct JSON + `crypto/rand` ID precedent), `docs/state-of-the-art/storage-substrate-and-versioned-data.md`
§3, §9 (tracked Dolt/substrate research), `internal/workflow/session_ignore.go`
(`EnsureLocalIgnoreContract`, reused by D8)

---

## Rev-2 fold summary

The rev-1 adjudication (`173bb3c`) found rev-1's Dolt argv templates
combined mutually exclusive flags, its local-storage design still
conflicted with `ADR-027`, its symlink gate missed ancestor components,
its publication design wrote N tracked files instead of one atomic
commit, several wire variants were incomplete, and its lock/failure
design was not crash-recoverable. This rev-2 rewrite resolves every
finding; see the companion PRD §0.1 for the corrected-citation table
(not repeated here to avoid drift — this ADR cites the PRD's rows by
ID, e.g. "C11," where relevant).

**Preserved across all three revisions**: a separate `resources.json`
per feature, never inside the canonical patch or unapply/lifecycle
state; Dolt (or any external tool) is never an authority over tpatch
state and is not a build/runtime dependency; replay/backward-
compatibility is Git-only.

## Decision Drivers

- ADR-027 D1–D6's existing committed/local split and hard-failure
  redaction posture must extend cleanly to a new kind of captured
  content, not be reinterpreted, and must hold even for content that
  only ever exists ephemerally within one command invocation.
- Any claim about Dolt's CLI surface must be verified against the
  primary `dolthub/dolt` source where possible (commit
  `59fb843bf6a4b653d7c8b6d997a603b10cf279d9`), cross-checked against
  the DoltHub reference, not invented — and any gap in that
  verification (e.g. `WORKING`/`STAGED` support) must be stated
  honestly rather than assumed.
- Any claim about an existing safety/validation primitive
  (`EnsureSafeRepoPath`, `IsPathIgnored`, `EnsureLocalIgnoreContract`,
  `ExitCodeError` usage) must be verified against current source, not
  assumed from its name.
- A tracked publication step must be a single atomic commit point, not
  an implicit "N files changed together" convention.
- A crash at any point in the local-scratch or tracked-publication
  pipeline must leave the system in a state a subsequent invocation
  can recover from without manual intervention, and any residual gap
  the Go standard library cannot close must be stated honestly rather
  than glossed over.

## Decision

### D1 — Scope & authority (reaffirmed, unchanged across all three revisions)

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

### D3 — Resource ID: canonical-JSON args encoding + golden vectors (reaffirmed, vectors 2/3 recomputed)

The canonicalization algorithm itself is unchanged from rev-1 (keys
sorted byte-ascending, minimal `{"k":"v",...}` escaping, no
`encoding/json.Marshal` HTML-escaping, UTF-8 required with no NFC/NFD
normalization, `NUL`/C0 control bytes rejected at `add` time — PRD
§13.1–§13.2 for the full grammar). `resource_id := "res_" +
lowercase-hex(SHA-256(feature + "\x00" + kind + "\x00" + selector +
"\x00" + adapter + "\x00" + capability + "\x00" + canonical_args))[:12]`.
Vectors 1 (`git-metadata`/`head`) and 4 (`ignored-file`) are byte-
identical to rev-1: `res_acc91dc23a8b` and `res_79f5ac5dca13`. Vectors
2 and 3 (`adapter-snapshot`) are **recomputed** — not because the
algorithm changed, but because D5 below renames the Dolt capability
from rev-1's `schema-diff`/`table-diff` pair to one unified
`diff-summary` capability, and the capability string is part of the
hashed payload. Both recompute to the same value,
`res_f8a28c218dbb`, which independently reconfirms order-independence
of the `args` canonicalization still holds under the new capability
name (PRD §0.3, §13.3 — golden vector table reproduced there
byte-identical to this decision).

| Vector | Inputs (`feature`, `kind`, `selector`, `adapter`, `capability`, `args`) | `resource_id` |
|---|---|---|
| 1 | `model-picker`, `git-metadata`, `head`, ``, ``, `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker`, `adapter-snapshot`, `dolt:diff-summary:users`, `dolt`, `diff-summary`, `{"table":"users","from":"main","to":"HEAD"}` (declared `table, from, to` order) | `res_f8a28c218dbb` |
| 3 | Same as Vector 2, `args` declared `to, table, from` order | `res_f8a28c218dbb` (**identical** — order-independence) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

### D4 — Full ADR-027 compliance: no persistent raw bodies in either lane (task 3)

Rev-1 still wrote raw bytes to a local, gitignored, opt-in
(`keep_local=true`) batch history (`.tpatch/local/resource-capture/<slug>/batches/lb_.../raw`)
that persisted across invocations. The rev-1 adjudication found this
still conflicted with `ADR-027` D1–D6's committed/local split in
spirit: a *local* raw history that survives indefinitely is not
meaningfully different from a committed one for the purpose of "does
this secret leak get read by something else later" — `ADR-027`'s
intent is that raw bodies exist only as long as strictly necessary to
compute a safe summary.

Rev-2 removes the opt-in and the persistence both: raw bytes for
`ignored-file` content and Dolt's captured stdout exist **only** inside
a single invocation's ephemeral scratch directory
(`.tpatch/local/resource-scratch/<slug>/es_<id>/`, PRD §7.1), are
scanned/hashed/parsed there, and the entire scratch directory is
deleted (best-effort) before the invocation returns — regardless of
`--dry-run`, success, or failure. There is no `keep_local` flag left to
opt into (PRD §0.2); every capture behaves the way rev-1's
`keep_local=false` path did. No tracked artifact ever contains raw
bytes, raw stdout, or a wall-clock timestamp (D10 below). `feature
resource diff` recomputes lightweight current metadata and compares it
against the last tracked batch's `result` — reporting metadata/hash/
file-set-level changes only, never a textual content diff (PRD §5.1,
§2's new non-goal); raw content diffing/versioning is explicitly
deferred to a future ADR that would have to supersede `ADR-027`'s
committed/local split, which this ADR does not attempt. Because there
is no persistent raw store at all, rev-1's "purge" requirement no
longer applies (nothing to purge); what remains is ephemeral-scratch
orphan cleanup, performed at the start of the next invocation for a
given slug (PRD §7.1).

### D5 — Dolt adapter protocol: `dolt_diff_summary` SQL, no version probe (task 1, task 2)

Rev-1's three-invocation `dolt diff --schema/--data --name-only
--filter={added,dropped,modified}` pattern combined flags the
pinned-commit source does not support as written and could not
classify a table as both schema- and data-changed in one pass, nor
detect a rename. Verified against `dolthub/dolt` at commit
`59fb843bf6a4b653d7c8b6d997a603b10cf279d9` (PRD C11): the read-only
`dolt_diff_summary(from, to[, table])` table function, queried via
`dolt sql -r json -q "..."`, returns exactly
`{from_table_name, to_table_name, diff_type, data_change, schema_change}`
per row. Rev-2's sole Dolt capability, `diff-summary`, uses exactly one
argv template:

```
<resolvedDoltPath> sql -r json -q "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change FROM dolt_diff_summary('<esc(from)>', '<esc(to)>'[, '<esc(table)>']) ORDER BY from_table_name, to_table_name;"
```

`from`/`to` required, `table` optional; any other/duplicate `--arg` key
is exit 2. Literal escaping refuses `NUL`/C0 control bytes and any
backslash outright (exit 2) and escapes only `'` → `''` (PRD §6.2) —
deliberately not a general SQL-injection-safe escaper, since
backslash's escape-character status inside a Dolt/MySQL string literal
depends on `sql_mode`, which this ADR does not control. `diff_type` is
tracked verbatim as an opaque string (its full enumeration was not
independently confirmed against source, PRD §6.2) — a rename surfaces
as differing `from_table_name`/`to_table_name` on one row, not a
separate flag. `data_change`/`schema_change` are normalized to genuine
JSON booleans regardless of the underlying `0`/`1` vs. `true`/`false`
typing ambiguity (PRD §6.3). `WORKING`/`STAGED` acceptance by
`dolt_diff_summary` specifically is explicitly **unconfirmed** (PRD
§6.2) — this ADR's own examples never depend on the answer.

**No version probe** (task 2, PRD C12): `dolt version` is never
invoked. Tool identity is `basename(resolvedPath)` + `SHA-256` of the
resolved binary's bytes (read, not executed) — the absolute path is
never tracked, only used in-process and in ephemeral local
diagnostics. The real `diff-summary` SQL invocation is itself the only
capability check; there is no separate "probe" failure class.

### D6 — Executable and path safety: two distinct, opposite-direction policies (task 2, task 5)

`safety.EnsureSafeRepoPath`/`store.NormalizeClaimPath` remain
lexical-only (PRD C2, unchanged citation). Rev-1's symlink gate
resolved and re-validated only the **final** path component, missing
symlinks in ancestor directory components entirely. Rev-2 replaces
this with two separate policies for two separate trust directions:

- **`ignored-file` selector paths** (must stay inside the repo):
  `Lstat` every path component from the repo root down to the leaf;
  refuse (`symlink-component-refused`, exit 3) if **any** component —
  ancestor or final — is a symlink, regardless of where it points.
  No component is ever resolved via `EvalSymlinks` and re-validated;
  a symlink anywhere in the chain is refused outright, a strictly
  simpler fail-closed rule that closes the ancestor gap by
  construction. The final open additionally uses `syscall.O_NOFOLLOW`
  (PRD C14) as a real, available hardening layer against a
  final-component race, and a post-open `Lstat` compares
  device/inode/size/mtime against the pre-open values to detect a
  same-name replacement. This gate re-runs at both `add` and every
  `capture`, independently for every descendant file of a directory
  selector.
- **Dolt executable paths** (must stay outside the repo): symlinks
  ARE followed (`filepath.EvalSymlinks`) since a trusted external
  tool is commonly installed via a version-manager symlink chain; the
  *resolved* target must be a regular, executable file located outside
  the repository working tree and outside any `.git` directory —
  refused (`adapter-executable-in-repo`, exit 3) if not. A cheap
  post-invocation `Lstat` compares device/inode/size/mtime against the
  pre-invocation values (the one-time `SHA-256` computed at resolution
  time is not re-hashed) to detect a mid-invocation replacement
  (`adapter-executable-replaced`, exit 3, result discarded).

**Residual honestly stated** (task 5, PRD C14): Go's standard library
has no portable equivalent of `openat2`/`RESOLVE_NO_SYMLINKS` that
atomically binds every ancestor directory component against a race
between the `Lstat` walk and the final open; a sufficiently well-timed
attacker replacing an ancestor **directory** itself between those two
points is not fully closed using only the standard library. This ADR
does not claim an impossible sandbox guarantee — it documents this as
an accepted, stated v1 residual risk.

### D7 — One tracked publication point per capture (task 4)

Rev-1 published one tracked `summary.json` per resource plus a
separate local `current` pointer — not one atomic commit. Rev-2
replaces this with exactly two tracked artifact types under
`artifacts/resource-captures/`:

- **`batches/<batch_id>.json`** — one immutable file per successful
  capture invocation, containing every resource result that invocation
  produced (PRD §12.2). `batch_id` is `rb_<12 lowercase hex>`
  (`crypto/rand`). Written via temp-file-then-rename with an `fsync`
  of the file before rename and the containing directory after.
- **`current.json`** — one atomically-rewritten pointer, a
  `latest_batch_id` plus a sorted `[]{resource_id, batch_id}` array
  mapping every currently-declared resource to the batch holding its
  latest result (PRD §12.3). Also written via temp-file-then-rename
  with the same `fsync` discipline. **This rename is the single,
  atomic commit point of the entire capture** — nothing is visible to
  a reader until it succeeds.

`resources.json` (the declaration manifest) is explicitly **not** part
of this transaction — `add`/`remove`/`clear` only ever rewrite
`resources.json` and prune `current.json`'s live index; they never
touch any `batches/<id>.json` file, which is permanent, immutable
history regardless of whether its resource is later undeclared.

**Crash windows**: before the batch rename — nothing changed, safe
retry, orphan `.tmp-*` swept at next invocation; after the batch
rename but before the pointer rename — a permanently orphaned, harmless
batch file no reader ever surfaces (not garbage-collected in v1, same
"leave orphans in place" precedent already accepted elsewhere in this
project); during the pointer's temp-write — orphaned `.tmp` swept at
next invocation, prior `current.json` remains authoritative; after the
pointer rename — fully committed.

### D8 — Ignored/tracked Git gates: literal pathspecs, exact exit-code handling, local-ignore reuse (task 6, task 7)

Every Git invocation this ADR relies on (`check-ignore`, `ls-files
--error-unmatch`, `ls-files --stage`) uses `--literal-pathspecs`, so a
selector shaped like pathspec magic (a leading `:`, an embedded `**`)
is always treated as a literal path (PRD §10.4). `check-ignore` exit 0
= ignored, exit 1 = not ignored (refused), any other exit = fatal Git
error (refused, distinct reason, never treated as either "ignored" or
"not ignored"); `ls-files --error-unmatch` exit 0 = tracked (refused
when combined with "ignored"), exit 1 with the standard "did not match
any file(s) known to git" shape = untracked (gate passes), any other
exit/output shape = fatal Git error (refused). The ephemeral-scratch
root itself is verified via the **existing**
`workflow.EnsureLocalIgnoreContract(repoRoot, resourceScratchRoot)`
(PRD C13, `internal/workflow/session_ignore.go:138`, reused rather
than reinvented) plus the same tracked-file gate layered on top, since
`EnsureLocalIgnoreContract` alone does not close the `--no-index` gap
for the scratch root any more than it does for an `ignored-file`
selector.

### D9 — Lock semantics: crash-safe, PID-reuse-guarded, no wait (task 9)

A single advisory per-slug lock,
`.tpatch/local/resource-scratch/<slug>/.lock`, `O_CREATE|O_EXCL` JSON
body `{pid, process_start, host}`, where `process_start` is the output
of `ps -o lstart= -p <pid>` captured at lock-creation time (a portable
macOS/Linux technique requiring no `/proc` or cgo). On contention:
malformed lock → quarantine (random-suffixed rename) and retry;
`host` mismatch → refuse (`capture-lock-held-remote`, exit 3), no
reclaim attempt (two machines racing a shared-FS lock is worse than a
manual-intervention refusal); `host` match, `ps -o lstart= -p <pid>`
returns nothing (dead) → quarantine and retry; returns a **different**
start-time string for the same PID (reuse) → quarantine and retry;
returns the **same** start-time string → refuse immediately
(`capture-in-progress`, exit 3), no blocking/wait, no timeout flag in
v1. Release is `os.Remove`, best-effort, as the last step of every
invocation. Windows liveness/reuse detection is best-effort/
unsupported in v1 (consistent with `ADR-004-m10-copilot-proxy-ux.md`
D6's existing macOS/Linux-only validation-scope precedent).

### D10 — Permissions and no tracked timestamps (task 8, task 3)

Ephemeral scratch directories are created `0700` and files `0600` at
creation (never via a separate `chmod` after a looser-permission
create) — the lock file is also `0600`. Tracked artifacts
(`resources.json`, `batches/<id>.json`, `current.json`) use ordinary
repository file permissions (`0644`), since they never contain raw
bytes or secrets by construction. No tracked artifact contains a
wall-clock timestamp field anywhere (`added_at`/`captured_at` from
rev-1 are both removed); ordering is conveyed by the append-only batch
file layout and `current.json`'s pointer, not by a clock reading.
Failure diagnostics on a failed capture are either printed directly to
the CLI's own output or written inside the same ephemeral-scratch tree
that is deleted regardless of outcome — never a tracked failure
envelope, and never in conflict with "batches are written only on
success," since diagnostics and batches never share a file.

### D11 — Wire canonicalization: no tracked `map` types, `record --resources` two-domain framing retained (task 11, task 12)

Every tracked JSON field that would otherwise be a Go `map` (`args`,
the per-resource index in `current.json`) is instead a sorted
`[]struct` array (`[]{key, value}` / `[]{resource_id, batch_id}`), so
no tracked artifact's determinism depends on `encoding/json`'s
map-key-sort behavior. `record --resources` retains the two-
atomic-domain framing from rev-1 (Git-side capture and resource-domain
publication never commit together): staging is now purely ephemeral
(never writes a batch file until Git succeeds); on Git success,
staging's already-computed candidate content is published through the
exact D7 batch-then-pointer sequence; on Git failure, the candidate is
simply discarded. A `resource-domain-incomplete` (exit 1) partial
result carries an explicit, idempotent recovery command
(`tpatch feature resource capture <slug>`).

## Wire Schema Appendix

Byte-identical to the companion PRD's §12.1–§12.3 (verified
programmatically, not just visually).

```json
{
  "resources": [
    {
      "resource_id": "res_f8a28c218dbb",
      "kind": "adapter-snapshot",
      "selector": "dolt:diff-summary:users",
      "adapter": "dolt",
      "capability": "diff-summary",
      "args": [
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
  "batch_id": "rb_1a2b3c4d5e6f",
  "feature": "model-picker",
  "results": [
    {
      "resource_id": "res_f8a28c218dbb",
      "kind": "adapter-snapshot",
      "selector": "dolt:diff-summary:users",
      "adapter": "dolt",
      "capability": "diff-summary",
      "args": [
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
    }
  ]
}
```

```json
{
  "latest_batch_id": "rb_1a2b3c4d5e6f",
  "resources": [
    { "resource_id": "res_79f5ac5dca13", "batch_id": "rb_1a2b3c4d5e6f" },
    { "resource_id": "res_acc91dc23a8b", "batch_id": "rb_1a2b3c4d5e6f" },
    { "resource_id": "res_f8a28c218dbb", "batch_id": "rb_1a2b3c4d5e6f" }
  ]
}
```

## Implementation Notes

1. `internal/redact.Scan([]byte) []string` is a new exported package;
   the existing `internal/cli/session_redaction.go` matchers for
   bearer/token/key prefixes and absolute-home-paths are extracted
   into it, not duplicated.
2. `ps -o lstart= -p <pid>` is invoked via `exec.Command`, consistent
   with this project's existing pattern of shelling out to system
   utilities (`git`, `dolt`) rather than adding a dependency.
3. The `dolt_diff_summary` query is built via `fmt.Sprintf` over the
   already-validated/escaped `from`/`to`/`table` strings — validation
   (D5) happens strictly before string construction, never after.
4. `es_<id>`/`rb_<id>`/lock-quarantine suffixes all reuse the existing
   `crypto/rand`, 12-lowercase-hex-character ID convention already
   established elsewhere in this project (`ua_` attempt IDs, `lb_`
   batch IDs from rev-1).
5. The post-open `Lstat` re-check (D6) and the post-invocation
   executable re-check (D6) both compare a `(dev, ino, size, mtime)`
   tuple obtainable from `syscall.Stat_t` on Unix build targets; no
   new external dependency is required.

## Negative Consequences Summary

- Ancestor-directory TOCTOU is a documented residual risk, not fully
  closed (D6).
- No raw content diffing/versioning in v1 — only metadata/hash/
  file-set-level change detection (D4).
- Windows lock liveness/reuse detection is best-effort/unsupported
  (D9).
- A feature with many resources cannot parallelize its own staging
  across multiple processes — the lock is per-slug (D9).
- `WORKING`/`STAGED` support for `dolt_diff_summary` remains
  unconfirmed pending implementation-time source re-verification (D5).

## Test Matrix

| # | AC | Area | Scenario | Expected |
|---|----|------|----------|----------|
| 1 | AC-1 | Dolt | Capture a declared `diff-summary` resource | Exact argv `<dolt> sql -r json -q "..."` invoked, no other flag combination |
| 2 | AC-1 | Dolt | Same, `--json` output | Tracked `result.tables` matches the SQL row set |
| 3 | AC-2 | Dolt | `--arg from=$'\x00'` | Exit 2, no SQL constructed |
| 4 | AC-2 | Dolt | `--arg to=a\\b` (backslash) | Exit 2 |
| 5 | AC-3 | Dolt | `--arg from="O'Brien"` | Single quote doubled, query succeeds |
| 6 | AC-4 | Dolt | `--arg table=x --arg table=y` | Exit 2, duplicate key |
| 7 | AC-4 | Dolt | `--arg unexpected=z` | Exit 2, unknown key |
| 8 | AC-5 | Dolt | Table renamed between `from`/`to` | `from_table_name` != `to_table_name` tracked verbatim on one row |
| 9 | AC-6 | Dolt | `-r json` returns `data_change: 1` | Tracked `result.tables[].data_change` is JSON `true` |
| 10 | AC-6 | Dolt | `-r json` returns `data_change: true` | Same normalized `true` result |
| 11 | AC-7 | Dolt | First-ever capture of a fresh `adapter-snapshot` resource | Identical schema shape to a later capture |
| 12 | AC-8 | Dolt | Any `capture` invocation | `dolt version` never appears in the invoked-process log |
| 13 | AC-9 | Dolt | Successful capture | `tool_identity` = `{basename, binary_sha256}` only, no path field present |
| 14 | AC-9 | Dolt | `--json` output | Same assertion via `--json` |
| 15 | AC-10 | Dolt | Inspect the Dolt child process's environment | Exactly `HOME`, `DOLT_ROOT_PATH`; nothing inherited |
| 16 | AC-11 | Dolt | `dolt` resolves to a path under the repo working tree | Refused `adapter-executable-in-repo` |
| 17 | AC-11 | Dolt | `dolt` resolves under a `.git` directory anywhere | Refused `adapter-executable-in-repo` |
| 18 | AC-12 | Dolt | Binary replaced mid-invocation (simulated) | Refused `adapter-executable-replaced`, result discarded |
| 19 | AC-13 | Scratch | Successful `capture` | `resource-scratch/<slug>/` empty immediately after |
| 20 | AC-13 | Scratch | Failed `capture` (redaction refusal) | `resource-scratch/<slug>/` still empty after |
| 21 | AC-14 | Wire | Inspect every tracked JSON field | No timestamp-shaped field anywhere |
| 22 | AC-15 | Diff | `ignored-file` content changed since last batch | `diff` reports which of hash/size/etc. changed, no textual diff |
| 23 | AC-15 | Diff | `ignored-file` unchanged | `diff` reports `unchanged` |
| 24 | AC-16 | Redaction | One resource's content matches a PEM key | Whole invocation refused, no batch written for any resource |
| 25 | AC-16 | Redaction | One resource's content matches a DB URL | Same refusal behavior |
| 26 | AC-16 | Redaction | Content matches none of the six classes | Capture proceeds normally |
| 27 | AC-17 | Publication | Multi-resource successful capture | Exactly one new `batches/<id>.json`, `current.json` rewritten once |
| 28 | AC-18 | Publication | Crash simulated after batch rename, before pointer rename | Orphaned batch never surfaced by `list`/`diff` |
| 29 | AC-19 | Publication | Crash simulated during batch temp-write | `.tmp-*` swept at next invocation; no effect on `current.json` |
| 30 | AC-19 | Publication | Crash simulated during pointer temp-write | Same sweep behavior, prior `current.json` remains authoritative |
| 31 | AC-20 | Manifest | `remove <id>` | `resources.json` and `current.json` updated; all `batches/*.json` untouched |
| 32 | AC-20 | Manifest | `clear` | Same, for every declared resource |
| 33 | AC-21 | Path | Ancestor directory of selector is a symlink | Refused `symlink-component-refused` |
| 34 | AC-21 | Path | Ancestor symlink points inside the repo | Still refused (no target inspection) |
| 35 | AC-22 | Path | Final component replaced by symlink between walk and open | Refused via `O_NOFOLLOW`/`ELOOP` |
| 36 | AC-23 | Path | File replaced (different inode) between walk and open | Refused `path-replaced-during-open` |
| 37 | AC-24 | Path | Missing ancestor component | Refused `path-missing` |
| 38 | AC-25 | Path | Directory selector, one descendant symlinked | That descendant refused, others unaffected |
| 39 | AC-25 | Path | Same selector re-`add`ed after fix | Gate re-verified at `add` |
| 40 | AC-26 | Git gate | `check-ignore` exit 1 | Refused `not-ignored` |
| 41 | AC-26 | Git gate | `check-ignore` exit 128 (fatal) | Refused `git-ignore-check-error`, distinct reason |
| 42 | AC-27 | Git gate | `ls-files --error-unmatch` exit 0 (tracked) | Refused `tracked-and-ignored` |
| 43 | AC-27 | Git gate | `ls-files --error-unmatch` unexpected stderr | Refused `git-ls-files-error` |
| 44 | AC-28 | Git gate | Selector `:(glob)config/*.env` | Treated as literal filename under `--literal-pathspecs` |
| 45 | AC-28 | Git gate | Selector containing `**` | Treated as literal, no magic expansion |
| 46 | AC-29 | Local ignore | `.tpatch/local/` not in `.gitignore` | Refused before scratch dir created |
| 47 | AC-29 | Local ignore | `.tpatch/local/` accidentally tracked | Refused via the layered `ls-files` gate |
| 48 | AC-30 | Permissions | Scratch directory created | Mode `0700` at creation, no later `chmod` call observed |
| 49 | AC-30 | Permissions | Scratch file created | Mode `0600` at creation |
| 50 | AC-31 | Lock | Fresh `capture` invocation | Lock file written with `{pid, process_start, host}` |
| 51 | AC-32 | Lock | Second concurrent `capture` for same slug | Refused `capture-in-progress`, no blocking |
| 52 | AC-33 | Lock | Lock PID no longer exists | Reclaimed automatically, invocation proceeds |
| 53 | AC-34 | Lock | Lock PID alive, different `process_start` | Reclaimed automatically (PID reuse) |
| 54 | AC-35 | Lock | Lock file is malformed JSON | Quarantined and reclaimed |
| 55 | AC-36 | Lock | Lock `host` differs from current host | Refused `capture-lock-held-remote`, no reclaim attempt |
| 56 | AC-37 | Scratch | Orphaned `es_*` dir from simulated crash | Swept at next invocation's start |
| 57 | AC-37 | Scratch | Orphaned `batches/.tmp-*`/`current.tmp.json` | Same sweep behavior |
| 58 | AC-38 | Metadata | HEAD detached | `symbolic_ref` is `null`, `detached` is `true` |
| 59 | AC-38 | Metadata | HEAD on a branch | `symbolic_ref` populated, `detached` is `false` |
| 60 | AC-39 | Metadata | `config` view with key `user.email` | Refused, exit 2 |
| 61 | AC-39 | Metadata | `config` view with `core.filemode` | Accepted |
| 62 | AC-40 | Metadata | `index-entry` selector `:(icase)Foo` | Resolved as the literal path under `--literal-pathspecs` |
| 63 | AC-41 | Record | `record --resources` on feature with zero declared resources | Refused `no-resources-declared`, no Git invocation |
| 64 | AC-42 | Record | Staging fails, Git succeeds | `resource-domain-incomplete`, recovery command shown, Git patch intact |
| 65 | AC-42 | Record | Same, `--json` output | Same fields present in JSON envelope |
| 66 | AC-43 | Record | Staging fails, Git fails | Only record's existing Git-failure behavior surfaces, no double-report |
| 67 | AC-44 | Record | Staging succeeds, Git succeeds | Batch + pointer published together, verified atomically |
| 68 | AC-45 | Record | `feature resource capture <slug> --dry-run` | Zero filesystem writes, no scratch dir created |
| 69 | AC-46 | Record | Re-run `capture` after prior failure | Succeeds without a dedicated recovery command |
| 70 | AC-46 | Record | Re-run twice after two prior failures | Still idempotent, correct `current.json` |
| 71 | AC-47 | Golden ID | Recompute Vector 1 | Matches `res_acc91dc23a8b` |
| 72 | AC-47 | Golden ID | Recompute Vector 2 | Matches `res_f8a28c218dbb` |
| 73 | AC-47 | Golden ID | Recompute Vector 4 | Matches `res_79f5ac5dca13` |
| 74 | AC-48 | Golden ID | Recompute Vector 3 (reordered args) | Matches Vector 2 exactly |
