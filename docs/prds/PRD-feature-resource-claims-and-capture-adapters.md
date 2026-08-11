# PRD — Feature Resource Claims & Capture Adapters (rev-2)

**Status**: Draft — rev-2 (supersedes rev-1, writer commits `e8572b2`/
`f0f2c1f`, adjudicated NEEDS REVISION → REV-2 DISPATCHED at `173bb3c`;
see `docs/supervisor/LOG.md` → Cluster H rev-0 and rev-1 adjudications)

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

## 0. Rev-2 Fold Summary (read this first)

Rev-1 (`e8572b2`/`f0f2c1f`) was a substantial rewrite of rev-0 that
closed most of the rev-0 findings, but the rev-1 adjudication
(`173bb3c`, internal 5 HIGH + 2 MEDIUM, external 1 CRITICAL + 3 HIGH +
7 MEDIUM + 2 LOW) found rev-1 itself still unsafe or under-specified
in several places: every Dolt argv template combined mutually
exclusive flags; local persistent raw bodies and tracked wall-clock
timestamps still conflicted with ADR-027; the symlink gate only
handled the final path component, missing ancestor escapes, while the
executable policy left no valid location for a real Dolt binary to
live; tracked publication still spanned per-resource files instead of
one atomic commit; several wire variants and exit paths were
undefined; the lock/failure-directory design was not crash-recoverable;
and the Git ignore/tracked gates lacked literal-pathspec handling and
local-ignore-root coverage. Rev-2 is, again, a substantial rewrite of
the affected sections, not a patch. **Preserved** because every review
across all three revisions has agreed it is sound:

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

Everything below this point is the standalone rev-2 design; §0.1–§0.4
map each rev-1 finding to its resolution.

### 0.1 Claims Audit (rev-2 additions)

Rev-1's `C1`–`C10` (citation corrections for `featureCmd`, the
lexical-only safety helpers, `--no-index` ignore semantics, the
existing session-redaction shape, `ExitCodeError` call sites, the
`feature claim` CLI precedent, ADR-027 D1, tracked-vs-untracked
research docs, real Dolt CLI flags, and `RemoveClaim`'s line range)
all remain correct and are not repeated here — see the rev-1 text
preserved in `docs/handoff/HISTORY.md`/git history for that table.
Rev-2 adds:

| # | Claim | Citation | Why this changes rev-1 |
|---|-------|----------|-------------------------|
| C11 | `dolt diff --name-only` combined with `--schema`/`--data` and `--filter=` is **not** how the pinned Dolt source expresses per-table schema/data change classification; the source-verified read-only interface is the `dolt_diff_summary(from, to[, table])` table function, queried over `dolt sql -r json -q "..."`, returning exactly `{from_table_name, to_table_name, diff_type, data_change, schema_change}` per row | Supervisor source check against `dolthub/dolt` at commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9`: `go/cmd/dolt/commands/diff.go` (synopsis, `--schema`/`--data`, `--result-format`), `go/cmd/dolt/commands/sql.go` (`-q`/`--query`, `-r`/`--result-format json`), `go/cmd/dolt/commands/version.go` | rev-1 **error**: rev-1's three-invocation `--name-only --filter={added,dropped,modified}` design combined flags in a way the source does not support as a single coherent invocation, and could not detect renames or represent "both schema and data changed" for one table in one classification. §6 replaces this entirely with the one-query `dolt_diff_summary` design. |
| C12 | `dolt version` is a real subcommand that can, depending on build/config, perform a network update check and read/write files under the resolved `HOME`; it is not a safe no-side-effect probe | Rev-2 threat-modeling of running an arbitrary resolved executable literally named `dolt` with an unconstrained inherited environment, not a specific pinned-commit source citation (no claim here about `dolt version`'s exact internal behavior beyond "runs arbitrary code with inherited env/network access," which is true of any executed binary in v1's threat model) | rev-1 ran `dolt version` as a "probe" step with the invoking process's inherited environment. §6.1 removes this probe entirely; tool identity is now a static file fact (executable basename + `SHA-256` of the resolved binary's bytes), never a code-execution result, and every actual invocation runs with a minimal, non-inherited scratch environment (§6.4). |
| C13 | `internal/workflow/session_ignore.go`'s `EnsureLocalIgnoreContract(repoRoot, resolvedPath)` verifies the path is inside the worktree and that `gitutil.IsPathIgnored` (`--no-index`) reports it ignored; it does **not** independently verify the path is untracked | `internal/workflow/session_ignore.go:138-175` (`EnsureLocalIgnoreContract` body) | New for rev-2: §10.3 reuses this exact function for the ephemeral-scratch root (task 7's "do not invent a second ignore mechanism") but layers the same tracked-file gate used for `ignored-file` selectors (§5.1) on top, since `EnsureLocalIgnoreContract` alone does not close the `--no-index` gap for the scratch root either. |
| C14 | Go's `os.OpenFile` accepts `syscall.O_NOFOLLOW` on Unix build targets (`darwin`/`linux`), which causes the open to fail with `ELOOP` if the **final** path component is a symlink; there is no portable stdlib/syscall equivalent that also binds every **ancestor** directory component against races (no `openat2`/`RESOLVE_NO_SYMLINKS` wrapper in the Go standard library) | Go standard library `os`/`syscall` package documentation (`O_NOFOLLOW` is a documented, platform-gated `syscall` constant; `openat2` has no stdlib wrapper as of the Go versions this project targets) | New for rev-2: §9.1 uses `O_NOFOLLOW` as one real, available hardening measure for the final component and is explicit that ancestor-component TOCTOU is closed by *refusing any symlink component at all* (a stat-time check) rather than by any stronger descriptor-bound guarantee stdlib cannot provide (task 5: "state TOCTOU residual honestly ... do not claim impossible sandbox"). |

### 0.2 What rev-2 removes or changes

- The three-invocation `--name-only --schema/--data --filter=` Dolt
  argv pattern and the `schema-diff`/`table-diff` two-capability split
  (§6). Replaced by one capability, `diff-summary`, one argv template,
  one SQL query.
- The `dolt version` probe step entirely (§6.1). Tool identity no
  longer depends on executing the adapter at all before the real
  capability invocation.
- **Every** persistent local raw-body concept: the `keep_local`
  per-resource opt-in flag, the `.tpatch/local/resource-capture/<slug>/batches/lb_.../raw`
  files, and the local `current` batch-ID pointer file (§7). Raw bytes
  now exist only inside a single command's ephemeral scratch directory
  and are deleted (best-effort) before that command returns — never
  persisted across commands, opt-in or not.
- Per-resource tracked `resources/<id>/summary.json` files and their
  implied "N tracked writes per capture." Replaced by one immutable
  tracked batch file per capture invocation plus one atomically
  rewritten tracked pointer file (§7.3, §12).
- The `changes` field name in the per-resource tracked result.
  Renamed to `result` throughout (it was already called `result` in
  one place and `changes` in another in rev-1 — rev-2 uses `result`
  everywhere, consistently).
- `git-metadata`'s `config` view keeps rev-1's already-narrow 4-key
  allowlist (`core.filemode`, `core.ignorecase`, `core.symlinks`,
  `extensions.objectformat`) — unchanged, restated here only because
  §5.2 was renumbered.
- All wall-clock timestamp fields in tracked artifacts (`added_at` in
  the declaration manifest, `captured_at` in the per-resource result).
  Tracked artifacts are now timestamp-free; batch identity (`rb_<id>`)
  and the append-only file layout itself convey ordering, not a clock
  reading.

### 0.3 Golden resource-ID vectors

The canonical `args`-encoding algorithm (§13.1) is **unchanged** from
rev-1 — none of rev-2's fixes touch the ID-derivation hash function
itself, only the storage/wire/tool/path layers around it. Vectors 1
and 4 are therefore byte-identical to rev-1. Vectors 2 and 3 are
**recomputed** because the Dolt capability name they embed changed
from `schema-diff` to `diff-summary` (§0.2), which changes the hash
input:

| Vector | Feature | Kind | Selector | Adapter | Capability | Args (declaration order) | `resource_id` |
|---|---|---|---|---|---|---|---|
| 1 | `model-picker` | `git-metadata` | `head` | *(none)* | *(none)* | `{}` | `res_acc91dc23a8b` (unchanged) |
| 2 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `table=users, from=main, to=HEAD` | `res_f8a28c218dbb` (**recomputed**, was `res_19b4675405e2`) |
| 3 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `to=HEAD, table=users, from=main` (reordered) | `res_f8a28c218dbb` (**identical to Vector 2** — order-independence still holds) |
| 4 | `model-picker` | `ignored-file` | `config/local-secrets.env.template` | *(none)* | *(none)* | `{}` | `res_79f5ac5dca13` (unchanged) |

### 0.4 Requirement-item → section map

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | Dolt SQL redesign | §6 |
| 2 | Remove version probe; executable identity | §6.1, §9.2 |
| 3 | Full ADR-027 compliance, no persistent raw | §7, §8 |
| 4 | One real publication point | §7.3 |
| 5 | Path gate (ancestor symlinks) | §9.1 |
| 6 | Ignored/tracked Git calls, literal pathspecs | §10 |
| 7 | Local ignore contract reuse | §10.3 |
| 8 | Permissions | §7.4 |
| 9 | Lock semantics | §7.2 |
| 10 | Git metadata / tool fields | §5.2, §5.3, §12 |
| 11 | Transaction / `record --resources` | §11 |
| 12 | Wire / canonicalization | §12, §13 |
| 13 | Tracking / cross-references | throughout, §0.1 |
| 14 | ACs / matrix rebuild | §14 |

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
append-only, per-feature capture history, with all raw content
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
detection (§5.1, §7.3.4). Persisting raw content across captures would
require a future ADR that explicitly supersedes `ADR-027-capture-context-privacy-boundary.md`'s
committed/local split (which today forbids exactly that); this PRD
takes no position on whether such a future ADR should exist.

## 3. Command Surface

All new verbs live under the existing `feature` noun, mirroring
`feature claim`'s shape:

```
tpatch feature resource add    <slug> --kind <kind> --selector <sel> [--adapter <a> --capability <c> --arg k=v ...] [--json]
tpatch feature resource list   <slug> [--json]
tpatch feature resource remove <slug> <resource-id-or-prefix> [--json]
tpatch feature resource clear  <slug> [--json]
tpatch feature resource capture <slug> [--resource <resource-id-or-prefix>] [--dry-run] [--json]
tpatch feature resource diff   <slug> [--resource <resource-id-or-prefix>] [--json]
tpatch record <slug> [existing flags...] [--resources] [--json]
```

Rev-2 removes the rev-1 `--keep-local` flag entirely (§0.2): raw bytes
are always ephemeral now, so there is nothing left to opt into.

- **`capture`** is the only verb that ever executes the Dolt adapter,
  reads ignored-file content, or reads Git metadata, and the only verb
  that ever writes tracked (§7.3) or local scratch (§7.1) state.
  `--dry-run` runs the full pipeline (redaction included) and reports
  what would be written, but performs **no** writes at all — not even
  to the ephemeral scratch tree (§7.1 explicitly special-cases
  `--dry-run` to use an in-process buffer instead of creating a
  scratch directory on disk).
- **`diff`** is read-only: it never executes the adapter, never reads
  `ignored-file`/Git-metadata content, and never touches the scratch
  tree. It recomputes lightweight **current** metadata (size, mtime,
  file-set) without opening file content or running Dolt, and compares
  that against the last tracked batch's recorded `result` for that
  resource (§5.1, §7.3.4). Called before any capture has ever run for
  a resource, it reports "no capture yet" (exit 0, not an error).

`add`/`list`/`remove`/`clear` behave exactly as `feature claim`'s
quartet does (same `"no such feature: %s"` refusal shape, same
`--json` convention), except `add` additionally computes and persists
`resource_id` (§13) and rejects duplicates (same `selector`+`kind`+
`adapter`+`capability`+canonical `args` tuple already declared for
that feature) as a validation error (exit 2). `remove`/`clear` only
ever mutate the declaration manifest and the tracked pointer's live
index (§7.3.5) — they never delete or rewrite any historical batch
file, which remains a permanent, immutable audit record even after its
resource is undeclared.

## 4. Data Model

Two tracked artifacts per feature, both under the existing per-feature
artifacts directory (`store.featureArtifactsDir`, `internal/store/store.go`),
never inside `apply-recipe.json` or any unapply/lifecycle-state file:

- **`resources.json`** — the declaration manifest (declaration-only;
  not itself part of any capture transaction, task 4). One entry per
  declared resource: `resource_id`, `kind`, `selector`, `adapter`
  (empty string if not applicable), `capability` (empty string if not
  applicable), `args` (a sorted array of `{key, value}` pairs, never a
  bare JSON object/map — §12.1), `added_by_tool_version` (the
  `tpatch` version string that created this declaration; informational
  only, not a timestamp). **Never** contains a capture result, a hash,
  or any raw content.
- **`artifacts/resource-captures/`** — the append-only capture
  history: immutable `batches/<batch_id>.json` files (one per
  successful `capture` invocation, containing every resource result
  produced by that invocation) plus one atomically-rewritten
  `current.json` pointer mapping each resource to the batch that holds
  its latest result (§7.3, §12.2–§12.3). `resources.json` is never
  itself part of this transaction — `add`/`remove`/`clear` write only
  to `resources.json` (and, for `remove`/`clear`, prune the
  corresponding live entries from `current.json`'s index — never the
  batch files themselves, §3).

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

1. `git --literal-pathspecs check-ignore -q --no-index -- <selector>`
   exits `0` (ignored). Exit `1` means "not ignored" (gate fails,
   refused). Any other exit code is a fatal Git error (`git-ignore-check-error`,
   exit 3, fail-closed — never treated as "not ignored" or "ignored,"
   just refused outright, §10.1).
2. `git --literal-pathspecs ls-files --error-unmatch -- <selector>`
   exits non-zero with the standard "did not match any file(s) known
   to git" message (untracked; gate passes). Exit `0` means the path
   **is** tracked — refused (`tracked-and-ignored`, exit 3) even
   though check 1 said "ignored," closing the exact `--no-index` gap
   where an already-tracked file can still report "ignored" (§10.2).
   Any exit/output combination that is neither of these two well-known
   shapes is a fatal Git error, refused the same as check 1
   (`git-ls-files-error`, exit 3).

Both invocations use `--literal-pathspecs` (task 6) so a selector that
happens to look like pathspec magic (a leading `:`, an embedded `**`,
etc.) is always treated as a literal path, never reinterpreted —
closing an ambiguity rev-1 did not address (§10.4 has the exact rows).

**Path/symlink gate** (task 5, full rewrite — see §9.1 for the
complete algorithm): every path component from the repository root
down to the selector (and, for a directory selector, down to each
matched descendant file independently) is `Lstat`'d; **any** symlink
component anywhere in that chain is refused outright
(`symlink-component-refused`, exit 3) — rev-2 does not attempt to
resolve and re-validate a symlink's target (rev-1's approach, which
missed ancestor components); it simply refuses the presence of a
symlink anywhere in the path, a strictly simpler and safer fail-closed
v1 rule. Only after every component in the chain is confirmed a
regular (non-symlink) file/directory is the path opened, using
`O_NOFOLLOW` on the final open as an additional, real hardening layer
(§9.1) — and re-`Lstat`'d immediately after opening to compare
device/inode/size/mtime against the pre-open check, refusing
(`path-replaced-during-open`, exit 3) if they differ (a TOCTOU
replacement between the check and the open).

**Directory limits** (unchanged from rev-1): 5 MiB per file, 20 MiB
total, 200 files — refused (exit 3) if exceeded, re-checked at every
`capture` even if the selector passed these limits at `add` time
(snapshot-time bounds, not a one-time check).

**Capture** (task 3): the matched file(s) are read into the
invocation's ephemeral scratch directory (§7.1) — never left in place
for scanning-in-place, so a directory selector's multi-file scan sees
a single consistent point-in-time snapshot rather than files that
could each change mid-scan. Content is scanned (redaction, §8),
classified `binary` (a `NUL` byte in the first 8 KiB) or `text`, and
hashed (`SHA-256`, verbatim bytes, **no** text normalization of any
kind — CRLF/LF, trailing newline, and encoding are all left exactly as
found, task 5's "raw local bytes are verbatim" requirement). The
scratch copy is then deleted (best-effort) before the command returns
(§7.1) — the tracked `result` for this kind is `file_kind`
(`"text"`/`"binary"`), `size_bytes`, `hash` (single file) or
`file_count`/`total_bytes`/`combined_hash` (directory — the combined
hash is `SHA-256` over each matched file's repo-relative path and its
own hash, sorted by path, joined `\x00`-delimited, so it changes if
any file's content, size, or the file set itself changes).

**`diff`** (§3, task 3): recomputes the same metadata (size,
`file_kind` via a fresh first-8-KiB peek, file set) **without**
persisting a new scratch copy of full content beyond what the
first-8-KiB binary check and hash computation require in-process, and
compares against the last tracked batch's `result` for this resource.
Reports `unchanged`, or exactly which of `size_bytes`/`hash`/
`file_count`/`total_bytes`/`combined_hash`/file-set membership
differs — never a textual line-level diff of file content (§2's new
non-goal).

### 5.2 `git-metadata` (task 10)

Four closed views, tagged result variants (exact fields, §12.3):

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

### 5.3 `adapter-snapshot` (task 1, task 2, task 10)

Selector: `dolt:<capability>:<table>`. `dolt` is the only adapter in
v1 (`generic-command` remains removed, per rev-1 §0.2, restated here).
`diff-summary` is the only capability (§6) — the rev-1 `schema-diff`/
`table-diff` split is gone; one query now reports both dimensions per
table (§0.2).

## 6. Adapter Protocol — Dolt (task 1, task 2)

### 6.1 Executable resolution and identity (no version probe, task 2)

The adapter locates `dolt` via `exec.LookPath("dolt")` at `capture`
time (never at `add` time). Distinct from the `ignored-file` path
policy (§9.1, which requires the path stay **inside** the repo), the
executable policy requires the opposite:

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
   3).

Once resolved and validated, **tool identity** is a static file fact,
never a code-execution result (C12): `basename(resolvedPath)` (e.g.
`"dolt"`) and the `SHA-256` hex digest of the resolved binary's bytes
(read directly, not executed). The **resolved absolute path itself is
never tracked** — only `basename` and `binary_sha256` appear in any
tracked artifact (§12.3); the absolute path exists only in-process for
the duration of the invocation and in local diagnostics on failure
(§7.5), never written to any file this PRD defines as persistent.

Immediately before invoking the actual capability (§6.2), the adapter
`Lstat`s the resolved path and records device/inode/size/mtime; the
`SHA-256` hash computed above **is** the one-time identity check (no
second hash after invocation, to avoid doubling the cost of hashing a
large binary on every capture) — the cheap post-invocation check is a
re-`Lstat` compared against the pre-invocation device/inode/size/mtime
tuple: any difference is `adapter-executable-replaced` (exit 3), and
that invocation's result — even if the SQL query itself appeared to
succeed — is discarded, never written to a batch (a replaced binary
mid-invocation means the output cannot be trusted regardless of its
apparent shape).

There is no separate "probe" step at all (task 2) — the real SQL
invocation in §6.2 **is** the capability check; a failure there is
reported through the same capability-failure taxonomy as any other SQL
error, not a distinct "probe failed" class.

### 6.2 Capability invocation — `diff-summary` (task 1)

One capability, one exact argv template, using the resolved absolute
Dolt path (never the bare string `"dolt"`, to avoid a second, redundant
`PATH` lookup at invocation time):

```
<resolvedDoltPath> sql -r json -q "<SQL>"
```

Where `<SQL>` is exactly:

```sql
SELECT from_table_name, to_table_name, diff_type, data_change, schema_change
FROM dolt_diff_summary('<esc(from)>', '<esc(to)>'[, '<esc(table)>'])
ORDER BY from_table_name, to_table_name;
```

(the `[, '<esc(table)>']` third argument is present **iff** `table`
was declared; omitted entirely — not passed as an empty string —
when it was not, per `dolt_diff_summary`'s documented optional third
argument.)

**Args**: exactly `from` and `to` are required; `table` is optional.
Any other key, or a duplicate `--arg` for an already-declared key, is
a validation error (exit 2) at `add` time — unchanged in spirit from
rev-1, now scoped to this one capability's exact argument set.

**Literal escaping** (`esc(...)`, task 1's "strict SQL-literal
escaping"): fail-closed, not a full SQL-injection-safe general-purpose
escaper. Each of `from`/`to`/`table` is validated **before** encoding:
any `NUL` byte or other C0 control character (`0x00`–`0x1F`, `0x7F`)
is a validation error (exit 2, same discipline as §13.1's canonical
`args` encoding); a literal backslash (`\`) anywhere in the value is
also a validation error (exit 2) — rev-2 deliberately refuses
backslash rather than attempting to encode it, because whether a
backslash is itself an escape character inside a Dolt/MySQL string
literal depends on the session's `sql_mode` (`NO_BACKSLASH_ESCAPES`),
which this PRD does not control or verify; refusing is simpler and
strictly safer than guessing. The only transform applied to an
otherwise-valid value is doubling a single quote (`'` → `''`), the
one escaping rule that is unambiguous under both interpretations of
`sql_mode`.

**Refs and `WORKING`/`STAGED`**: `from`/`to` accept any Dolt commit-ish
resolvable by `dolt_diff_summary` — branch names, tags, full or
abbreviated commit hashes. Whether the literal identifiers `WORKING`
and `STAGED` are accepted by `dolt_diff_summary` specifically (as
opposed to Dolt's diff subsystem generally) was **not** independently
confirmed against the pinned-commit source in this fold (the
supervisor's source citation covered the function's name, arguments,
and five-column return shape, not its acceptance of these two special
identifiers). This PRD does **not** fabricate that confirmation: it
documents `WORKING`/`STAGED` as commonly accepted by Dolt's other
diff-family interfaces (`https://www.dolthub.com/docs/concepts/dolt/git/diff/`)
and defers a definitive yes/no to the implementation cluster, which
must re-verify directly against the pinned-commit source (or a newer
one, re-citing the commit used) before shipping either behavior;
until then, this PRD's own examples and golden vectors use only
concrete branch/ref names (`main`, `HEAD`), never `WORKING`/`STAGED`,
so nothing here depends on the unconfirmed answer.

**Rename detection**: a rename surfaces as a `diff_type` value on the
row pairing the old `from_table_name` with the new `to_table_name`
(distinct names on the same row is itself the rename signal,
independent of whatever exact string `diff_type` holds for that case).
This PRD does **not** hardcode an assumed closed enumeration of
`diff_type`'s possible string values (the pinned-commit source
citation confirmed the column's existence and role, not its full
value set) — `diff_type` is tracked **verbatim** as an opaque string
classification (it is a single short structural word, not raw
body/stdout, so tracking it verbatim does not violate §8.1's "never
raw content" rule) rather than validated against a guessed closed set
that could silently reject a legitimate value.

**First capture**: when `from` refers to a point before a table
existed, that table's row (if `dolt_diff_summary` reports one at all
for a nonexistent `from`-side table — behavior not independently
re-derived from source beyond the five-column shape) is tracked
exactly as returned, with no special-cased "first capture" schema
distinct from any other capture (§14, `AC` for this case).

### 6.3 Output parsing and normalization (task 1)

`dolt sql -r json` wraps rows under a `"rows"` key alongside a
`"schema"` key describing column types (community/docs-corroborated
shape, `https://www.dolthub.com/docs/products/dolthub/api/v1alpha1/sql/`
— not independently re-derived from the pinned-commit source's exact
JSON-marshaling code, and flagged as such rather than presented with
the same confidence as the five confirmed column names). This PRD
does not assume `data_change`/`schema_change` are marshaled as native
JSON booleans (`true`/`false`) as opposed to `0`/`1` integers (MySQL/Dolt
`BOOLEAN` columns are `TINYINT(1)` under the hood, and integer JSON
typing for such columns is at least as plausible as native-boolean
typing). The parser therefore normalizes defensively: any of `1`,
`"1"`, `true`, or `"true"` in that field is treated as boolean `true`;
everything else (`0`, `"0"`, `false`, `"false"`, absent) is `false` —
and the **tracked** `result` always contains a genuine JSON boolean
after this normalization, never the raw, ambiguously-typed value.

### 6.4 Timeouts, caps, environment (task 2, task 8)

| Parameter | Value |
|---|---|
| Invocation timeout | 30 seconds. On timeout: `SIGTERM` to the process group, then `SIGKILL` after 2 more seconds if still running. |
| Captured output cap | 5 MiB combined stdout+stderr; output beyond the cap is truncated, and the truncation fact is recorded only in local, ephemeral diagnostics (§7.5) — never in the tracked artifact, which never contains raw output at all. |
| Environment | **Not** inherited from the invoking process (task 2's "no inherited credentials"). A fresh, minimal environment is constructed: `HOME=<scratch-home>` and `DOLT_ROOT_PATH=<scratch-home>` pointing at a directory created fresh under this invocation's ephemeral scratch tree (§7.1, `0700`), so Dolt never reads or writes the real invoking user's `~/.dolt` config/credentials; `PATH` is **not** set at all (the adapter is invoked by its already-resolved absolute path, §6.1, so `PATH` lookup is never needed mid-invocation). No other variable is passed through. |
| Termination | Process-group termination (the child and any of its own children) on timeout, to avoid orphaned Dolt subprocesses. |

A concrete, fully-specified argv/SQL example for Vector 2 (§0.3) —
`table=users, from=main, to=HEAD`:

```
argv: /usr/local/bin/dolt sql -r json -q "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change FROM dolt_diff_summary('main', 'HEAD', 'users') ORDER BY from_table_name, to_table_name;"
```

(the absolute path shown, `/usr/local/bin/dolt`, is illustrative only
— it is never the tracked value, §6.1.)

## 7. Ephemeral Scratch, Locking, and the Single Publication Point (task 3, task 4, task 8, task 9)

### 7.1 Ephemeral scratch layout and lifecycle (task 3)

`.tpatch/local/` is the existing gitignored local root (`LocalIgnoreRule`,
`internal/workflow/session_ignore.go:18`). Rev-2 adds a sibling of the
existing `.tpatch/local/capture/` (session capture) tree:

```
.tpatch/local/resource-scratch/<slug>/
  .lock                          -- advisory lock, present only while a capture runs (§7.2)
  es_<12 lowercase hex>/         -- one ephemeral-scratch directory per in-progress invocation
    dolt-home/                   -- scratch HOME/DOLT_ROOT_PATH for the Dolt adapter (§6.4)
    <resource_id>/
      raw                        -- present only transiently while this resource is being processed
      files/<relpath>            -- present only transiently, for a directory ignored-file selector
```

Every file under `es_<id>/` is created `0600` and every directory
`0700` **at creation** (`os.OpenFile`/`os.Mkdir` with the final mode
passed directly — never `os.Create` followed by a separate
`os.Chmod`, which leaves a race window at the default, looser
umask-derived permissions, task 8). The entire `es_<id>/` directory is
removed (`os.RemoveAll`, best-effort) as the last step of the
invocation, on **both** the success and failure paths — a removal
failure (e.g. a permission error) is logged as a local diagnostic
(§7.5) but does not itself fail an otherwise-successful capture,
since the tracked result has already been safely computed and (on the
success path) published by the time cleanup runs.

`--dry-run` (§3) never creates `es_<id>/` on disk at all — its
scratch equivalent is an in-process, bounded `bytes.Buffer` per
resource, discarded when the process exits, since a dry run never
needs to survive a crash mid-invocation (nothing it produces is ever
published).

**Orphan cleanup** (task 3, task 8): before creating a new `es_<id>/`,
every `capture`/`add` invocation first sweeps
`.tpatch/local/resource-scratch/<slug>/` for `es_*` directories and
`batches/.tmp-*` / `.tmp-current.json` files left behind by a
previous crashed run (§7.3.3's crash windows) and removes them
(`os.RemoveAll`/`os.Remove`, best-effort, silent on success, logged as
a local diagnostic on failure — never a hard failure of the current
invocation).

### 7.2 Lock semantics (task 9)

A single advisory lock per slug, `.tpatch/local/resource-scratch/<slug>/.lock`,
guards against two concurrent `capture`/`record --resources`
invocations for the same feature racing on scratch creation or
publication.

**Acquire** (never blocks/waits — either succeeds immediately or
refuses immediately, no polling, no configurable timeout in v1):

1. `os.OpenFile(lockPath, O_CREATE|O_EXCL|O_WRONLY, 0o600)`. On
   success, write a JSON body — `{"pid": <int>, "process_start":
   "<ps -o lstart= output for this pid, captured immediately>", "host":
   "<os.Hostname()>"}` — and proceed; the lock is held for the
   remainder of the invocation.
2. On `O_EXCL` failure (file already exists), read and parse the
   existing lock file:
   - **Malformed** (not valid JSON, or missing a required field): a
     valid lock is always well-formed (written atomically by this
     same code path) — malformed implies leftover corruption from a
     crash mid-write. Quarantine it (`os.Rename` to a
     `crypto/rand`-suffixed `.lock.stale-<8hex>` name, so a
     concurrent racer doing the same thing cannot collide) and retry
     step 1 once.
   - **`host` does not match `os.Hostname()`**: the lock may belong to
     another machine on a shared filesystem, whose process liveness
     cannot be verified locally. Refuse immediately
     (`capture-lock-held-remote`, exit 3) — **do not** attempt to
     reclaim; two machines racing to reclaim the same shared-FS lock
     is worse than a manual-intervention refusal.
   - **`host` matches**: run `ps -o lstart= -p <pid>` fresh.
     - No output (process no longer exists): stale. Quarantine (as
       above) and retry step 1 once.
     - Output **matches** the recorded `process_start` string exactly:
       the same live process still holds the lock. Refuse immediately
       (`capture-in-progress`, exit 3), no wait.
     - Output exists but **differs** from the recorded
       `process_start` (same numeric PID, different process — PID
       reuse): stale. Quarantine and retry step 1 once.

**Release**: `os.Remove(lockPath)` as the last step of the invocation
(success or failure path alike), best-effort — a failed removal is a
local diagnostic only; the next invocation's PID/`process_start`
staleness check reclaims it correctly once this process has actually
exited.

**Platform scope**: `ps -o lstart=` and PID-liveness-via-`ps` are
POSIX-shaped and validated on macOS/Linux, consistent with this
project's existing macOS/Linux-only validation scope
(`ADR-004-m10-copilot-proxy-ux.md` D6 precedent); Windows lock
liveness/reuse detection is best-effort/unsupported in v1, not claimed
otherwise.

### 7.3 The single publication point (task 3, task 4)

Per-invocation sequence, after lock acquisition (§7.2) and orphan
sweep (§7.1):

1. **Stage**: for every targeted resource (all declared, or the
   `--resource <id>` subset), perform the kind-specific capture work
   (§5) into `es_<id>/<resource_id>/` and compute its `result` +
   (where applicable) `raw.hash`/`raw.byte_count` entirely in memory
   or scratch. If **any** targeted resource's staging fails (adapter
   error, size limit, redaction refusal, path/symlink refusal), the
   **whole** invocation aborts here: no batch file is written, `es_<id>/`
   is removed, the lock is released, and the command exits with the
   specific refusal's code (all-or-nothing, matching the precedent
   already established for `record --resources` staging in rev-1).
2. **Write the batch** (task 4): build the batch object (§12.2) from
   every staged result, write it to `artifacts/resource-captures/batches/.tmp-<batch_id>.json`
   (ordinary tracked-repo file permissions — `0644` — since it never
   contains raw bytes), `fsync` the file, then `os.Rename` it to
   `artifacts/resource-captures/batches/<batch_id>.json` (same-directory
   atomic rename) and `fsync` the containing directory. `batch_id` is
   `rb_<12 lowercase hex>` (`crypto/rand`, same 48-bit space already
   accepted for other `tpatch` ID conventions).
3. **Publish the pointer** (task 4 — "the sole commit point"): compute
   the new `current.json` content (§12.3) — every resource staged in
   step 1 now points at the new `batch_id`; every other, previously
   tracked resource not touched this invocation keeps its existing
   entry, carried forward unchanged from the prior `current.json`.
   Write to `artifacts/resource-captures/current.tmp.json`, `fsync`,
   `os.Rename` to `artifacts/resource-captures/current.json`, `fsync`
   the directory. **This rename is the actual, single, atomic commit
   point of the whole capture** — before it succeeds, nothing has
   changed from any reader's perspective (the new batch file, if
   already renamed into place in step 2, is a harmless orphan no
   `current.json` entry references yet); after it succeeds, the
   capture is fully and atomically visible.
4. **Cleanup**: remove `es_<id>/`, release the lock (§7.2).

**Crash-window analysis** (task 4):

| Crash point | State left behind | Recovery |
|---|---|---|
| Before step 2's rename | No new batch file (or an orphaned `.tmp-<batch_id>.json`) | Orphan `.tmp-*` swept at next invocation's start (§7.1); safe to retry immediately, produces a fresh `batch_id` |
| After step 2's rename, before step 3's rename | A fully-written, permanently orphaned `batches/<batch_id>.json` that no `current.json` entry ever references | Harmless — never surfaced by `list`/`diff`/`capture` (§4.1's "missing batch" case does not apply here; this is an *extra*, unreferenced batch, not a missing one); left in place, not garbage-collected, in v1 (same "orphans are left, not auto-deleted" precedent already accepted for the rev-1 lock/scratch design) |
| During step 3's temp-write, before its rename | Orphaned `current.tmp.json` | Swept at next invocation's start; the last successfully-renamed `current.json` (from a previous, fully-committed invocation, or absent if this was the first-ever capture) remains authoritative and untouched |
| After step 3's rename | Fully committed | No recovery needed |

**Concurrency**: the lock (§7.2) already prevents two invocations for
the same slug from reaching step 2/3 simultaneously; nothing in §7.3
depends on filesystem-level atomicity across *different* slugs (each
slug's `artifacts/resource-captures/` tree is independent).

### 7.4 Permissions (task 8)

Restated precisely because it spans both the ephemeral (§7.1) and
tracked (§7.3) trees, which have **different** permission
requirements:

- Ephemeral scratch (`es_<id>/` and everything under it, plus the
  scratch Dolt `HOME`): directories `0700`, files `0600`, always at
  creation.
- Tracked artifacts (`resources.json`, `batches/<id>.json`,
  `current.json`): ordinary repository file permissions (`0644`),
  since they never contain raw bytes or secrets by construction
  (§8.1) — there is nothing to protect with tighter permissions, and
  using non-standard permissions on a tracked, checked-in file would
  be surprising to anything else that reads the working tree.
- The lock file (§7.2): `0600` (it is local/gitignored, and its
  content — PID, process-start string, hostname — is host-identifying
  operational metadata, not secret, but there is no reason to make it
  world-readable either).

### 7.5 Local diagnostics on failure (task 8, task 3)

When a `capture` invocation fails at any stage (§7.3 step 1), no
tracked failure envelope is ever written (unchanged principle from
rev-1) — failure detail is either printed directly to the CLI's own
stdout/stderr for that invocation (never persisted to any file) or, if
richer detail is useful for later inspection (e.g. the last 4 KiB of a
timed-out Dolt invocation's combined output, or the truncation fact
from §6.4), written to a file under the **same** `es_<id>/` ephemeral
tree that is deleted at the end of the invocation (§7.1) — diagnostics
never outlive the failed invocation any more than a successful one's
scratch content does. There is no contradiction between "diagnostics
are recorded" and "batches are immutable and only ever created on
success": diagnostics live in the ephemeral tree (deleted regardless
of outcome), batches live in the tracked tree (written only on
success, §7.3 step 2) — the two never share a file.

## 8. Privacy & Redaction (task 3)

### 8.1 Tracked content is always structural, never raw

No tracked file (`resources.json`, any `batches/<id>.json`,
`current.json`) ever contains: raw file bytes, raw adapter stdout, a
full Git object's content, or any string copied verbatim from a
scanned source (`diff_type`, §6.2, is the one narrow exception — a
short structural classification word, not "content" in the sense this
rule means). Tracked content is limited to hashes, byte/file counts,
the declared selector/args (themselves inputs, not scanned content),
structural true/false change flags, and `basename`+`binary_sha256`
tool identity. Raw bytes exist **only** transiently, inside a single
invocation's ephemeral scratch tree (§7.1) — never opt-in, never
persisted, regardless of `--dry-run` or any other flag.

### 8.2 The hard-refusal scanner

Unchanged from rev-1: `internal/cli/session_redaction.go` today is
unexported, shaped around `store.SessionObservation`, and applies
drop-the-line-and-continue semantics across 10 heuristic classes that
do not include dedicated PEM/OpenSSH-key, DB-connection-URL, or
email/PII patterns. This PRD requires the implementation cluster to
extract the reusable byte-pattern matchers into a new, exported,
content-agnostic `internal/redact.Scan(content []byte) []string`,
shared by both the existing session-redaction call site (unchanged
policy there) and the new resource-capture call site (always
hard-refuses on any match).

**Six closed classes** (unchanged from rev-1), applied to every
candidate string before it is written anywhere — tracked or
ephemeral-scratch — a match on any class is a hard refusal of the
**entire** invocation (`redaction-refused`, exit 3), never a partial
scrub-and-continue, and never a partial batch (§7.3 step 1's
all-or-nothing rule):

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
`ignored-file`'s raw content (byte-scanned regardless of binary/text
classification), and Dolt's captured stdout (§6.4) before it is
normalized into a tracked `result` (§6.3). Scanning happens **before**
any write of any kind — staging computes the scan result first; a
refusal means no batch is ever written (§7.3 step 1), not even for the
other, unaffected resources in the same invocation.

## 9. Path & Executable Safety (task 5, task 2)

### 9.1 `ignored-file` path gate: refuse any ancestor symlink (task 5)

`safety.EnsureSafeRepoPath` and `store.NormalizeClaimPath` remain
lexical-only (`filepath.Abs` + string-prefix containment, no `Lstat`,
no `EvalSymlinks`) — sufficient for their existing callers, not
sufficient alone for resource capture. Rev-1's gate resolved the
**final** component if it was a symlink and re-validated containment
on the resolved target; the rev-1 adjudication found this missed
symlinks in **ancestor** directory components (e.g. a selector
`a/b/secret.txt` where `a` or `a/b` itself is a symlink escaping the
repo, never checked by a final-component-only rule).

Rev-2 replaces this with a strictly simpler fail-closed rule, run at
both `add` and every `capture`, for every path this feature touches
(the selector itself, every directory descendant for a directory
selector, and the process `cwd`):

1. Split the repo-relative path into components. Starting from the
   repository root, `Lstat` each successive prefix (root, root/a,
   root/a/b, ..., the full path).
2. If **any** component's `Lstat` result is a symlink — regardless of
   where it points, whether the target exists, or whether the target
   would itself be safely inside the repo — refuse immediately:
   `symlink-component-refused` (exit 3). Rev-2 does **not** attempt
   `EvalSymlinks`-and-re-validate for any component; a symlink
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
5. Immediately after a successful open, `Lstat` the path again and
   compare device/inode/size/mtime against the values captured in
   step 2's walk for the final component: any difference is
   `path-replaced-during-open` (exit 3), and the just-opened content is
   discarded, never scanned or hashed.

**Refuse dangling/external/`.git`**: a missing prefix is `path-missing`
(step 3); a symlink anywhere is refused unconditionally (step 2) —
this subsumes rev-1's separate `symlink-escapes-repo` and
`symlink-targets-git-internal` outcomes, since **no** symlink is ever
followed or inspected for where it points; refusing all of them is
strictly more conservative than checking whether a specific one
happens to escape or target `.git`.

**TOCTOU residual, stated honestly** (task 5, C14): steps 1–5 close
the ancestor-symlink gap and the final-component race as far as Go's
standard library allows (`O_NOFOLLOW` binds only the *final* open
call; there is no stdlib/syscall wrapper — no `openat2`/
`RESOLVE_NO_SYMLINKS` — that also atomically binds every *ancestor*
directory component against a race between step 2's walk and step 4's
open). A sufficiently well-timed attacker who can replace an ancestor
**directory** itself (not just a leaf symlink) between steps 2 and 4
is not fully closed by this design using only the Go standard library.
This PRD does not claim otherwise; a stronger guarantee would require
platform-specific syscalls (`openat2` on Linux, no direct macOS
equivalent) outside this PRD's zero-external-dependency,
Unix-portable scope, and is explicitly left as a documented residual
risk rather than an impossible sandbox claim.

For a directory selector, this five-step gate runs **independently
per matched descendant file** (task 5's "every directory descendant"),
not just once for the top-level selector — a selector that was a
plain directory of plain files at `add` time but has since had one
entry replaced by a symlink is caught at the next `capture`, not
grandfathered in because the top-level directory itself still passes.

### 9.2 Executable path safety (task 2, distinct policy)

The Dolt executable's resolution uses the **separate** policy defined
in §6.1 (external-required, symlinks followed and their resolved
target validated, not refused) — the opposite direction from §9.1's
"stay inside the repo, refuse any symlink" rule, because an adapter
executable is a trusted external tool, not repo-owned content. The two
policies are never conflated: `ignored-file`/directory-descendant
paths always use §9.1; the Dolt executable path always uses §6.1/§9.2.

| Case | Outcome |
|---|---|
| `ignored-file` selector, every path component a plain file/dir, fully inside repo | Accepted |
| `ignored-file` selector, any ancestor component is a symlink (regardless of target) | Refused: `symlink-component-refused` |
| `ignored-file` selector, final component replaced by a symlink between the walk and the open | Refused: `symlink-component-refused` (via `O_NOFOLLOW`/`ELOOP`) |
| `ignored-file` selector, file replaced (same name, different inode) between the walk and the open | Refused: `path-replaced-during-open` |
| `ignored-file` selector, a prefix component does not exist | Refused: `path-missing` |
| Dolt executable resolves (possibly through symlinks) to a path outside the repo and `.git` | Accepted |
| Dolt executable resolves to a path inside the repo working tree or under any `.git` directory | Refused: `adapter-executable-in-repo` |
| Dolt executable's device/inode/size/mtime differ immediately after invocation vs. immediately before | Refused: `adapter-executable-replaced`, result discarded |

## 10. Git Ignore/Tracked Gate Semantics (task 6, task 7)

### 10.1 `check-ignore` exit-code handling

`git --literal-pathspecs check-ignore -q --no-index -- <path>`: exit
`0` = ignored (gate passes); exit `1` = not ignored (gate fails,
`not-ignored`, exit 3); any other exit code (`128` and similar) is a
fatal Git error — refused (`git-ignore-check-error`, exit 3), never
silently treated as either "ignored" or "not ignored."

### 10.2 `ls-files --error-unmatch` exit-code handling

`git --literal-pathspecs ls-files --error-unmatch -- <path>`: exit `0`
= tracked (gate fails when combined with "ignored" — §5.1 check 2);
exit `1` **with** the standard "did not match any file(s) known to
git" stderr shape = untracked (gate passes); any other exit code, or
exit `1` with unexpected stderr, is a fatal Git error — refused
(`git-ls-files-error`, exit 3), same fail-closed treatment as §10.1.

### 10.3 Local-ignore-root reuse (task 7)

Before the first write to `.tpatch/local/resource-scratch/` on a given
machine (i.e. once per invocation, cached in-process, not persisted),
`capture` calls the **existing**
`workflow.EnsureLocalIgnoreContract(repoRoot, resourceScratchRoot)`
(`internal/workflow/session_ignore.go:138`) — reused exactly as-is,
not re-invented (task 7) — which verifies Git is available, the path
is inside the worktree, and `gitutil.IsPathIgnored` reports it
ignored. Because `EnsureLocalIgnoreContract` alone does not close the
`--no-index` gap for the scratch root any more than it does for an
`ignored-file` selector (C13), rev-2 layers the **same** tracked-file
gate from §5.1/§10.2 on top: `git --literal-pathspecs ls-files
--error-unmatch -- .tpatch/local/` must also report untracked. Both
checks failing to hold is `local-path-not-ignored`/`local-path-tracked`
(exit 3) — refused before any scratch directory is created, exactly
mirroring ADR-027 D1's ignored-before-first-write mandate (this PRD
does not invent a second ignore mechanism, task 7 — it reuses the one
that exists and adds only the missing tracked-file half, the same
addition already made for `ignored-file` selectors in §5.1).

### 10.4 Pathspec-magic rows (task 6)

| Selector | Without `--literal-pathspecs` | With `--literal-pathspecs` (rev-2) |
|---|---|---|
| `:(glob)config/*.env` (a literal filename that happens to start with pathspec-magic syntax) | Reinterpreted as glob magic — matches an unintended set of paths | Treated as the literal filename `:(glob)config/*.env`; ignored/tracked checks run against that exact literal path |
| `config/**/local.env` | `**` reinterpreted as recursive-glob magic | Treated as a literal path containing the literal characters `**`; no magic expansion |

## 11. `record --resources` Semantics (task 11)

Unchanged high-level ordering from rev-1 — Git-side capture and
resource-domain publication remain **two separate atomic domains**;
what changes is that "staging" (§7.3 step 1) is now ephemeral-only
(never writes a batch file) and "publishing" is the same §7.3 steps
2–4 a standalone `capture` would run.

1. **Zero-resource preflight**: zero declared resources refuses
   immediately, before touching Git (`no-resources-declared`, exit 1),
   unchanged from rev-1.
2. **Stage** (ephemeral metadata only, task 11): run §7.3 step 1 for
   every declared resource — lock, orphan sweep, ephemeral scratch,
   redaction — but stop before step 2 (no batch file written yet); the
   fully-computed candidate batch content is held in memory pending
   step 4 below.
3. **Git-side capture**: `record`'s existing, unmodified capture-mode
   dispatch runs, completely unaffected by step 2's outcome.
4. **Publish, gated on Git success**:
   - Git failed: the record command's existing failure behavior
     propagates; the in-memory candidate batch from step 2 is simply
     discarded (never written anywhere — "ephemeral metadata only,"
     task 11) regardless of its own success/failure.
   - Git succeeded and step 2 also succeeded: run §7.3 steps 2–4 now
     (write batch, publish pointer, cleanup/release lock) using the
     already-computed candidate content — no adapter/Git-metadata
     re-execution.
   - Git succeeded but step 2 failed, or Git succeeded and step 2
     succeeded but the publish step (§7.3 steps 2–4) itself fails: a
     **partial-domain** result, `resource-domain-incomplete` (exit 1):
     > canonical patch recorded successfully; resource capture did not
     > complete: `<reason>`. Retry with `tpatch feature resource
     > capture <slug>` — this recomputes and publishes a fresh batch
     > and is safe to re-run (idempotent: each retry produces a new
     > `batch_id` and a correct `current.json`, regardless of how many
     > prior attempts failed).

**Interactions** (unchanged from rev-1): an empty Git-side capture
accepted by existing logic counts as Git-side success for gating
publish; `--auto`/commit-range flags compose with `--resources`
without special-casing; `record --resources` has no `--resource`
subset flag of its own (it always targets every declared resource —
the standalone `feature resource capture <slug> --resource <id>` is
the only subset-targeting entry point, matching its promised
all-declared-resources scope exactly, task 11/task 5's "resource-only
promised scope must match validation").

**Exit codes** (unchanged shape from rev-1, restated for the new
refusal names):

| Code | `feature resource {add,list,remove,clear,capture,diff}` | `record --resources` |
|---|---|---|
| `0` | Success (including `diff` reporting "no capture yet") | Success |
| `1` | Internal error; `tracked-batch-missing` (§4.1) | Same, plus `no-resources-declared` and `resource-domain-incomplete` |
| `2` | Validation: bad kind/adapter/capability/view, unknown/duplicate `--arg`, `NUL`/control byte/backslash in a Dolt arg, missing index entry at `add` | n/a (unmodified) |
| `3` | State/policy refusal: `not-ignored`, `tracked-and-ignored`, `git-ignore-check-error`, `git-ls-files-error`, any `symlink-component-refused`/`path-missing`/`path-replaced-during-open`, any size/count limit, `redaction-refused`, `adapter-missing`/`adapter-executable-in-repo`/`adapter-executable-replaced`, `local-path-not-ignored`/`local-path-tracked`, `capture-lock-held-remote`/`capture-in-progress`, `index-entry-missing` | Same set applies to staging (§11 step 2); surfaces as `resource-domain-incomplete` (exit 1) if Git succeeded, or as record's own existing exit code (with the discarded-batch diagnostic) if Git failed |

## 12. Wire Schemas (task 12)

Two distinct JSON serializations, unchanged distinction from rev-1:

- **Canonical `args` JSON** (§13.1) — hash input for `resource_id`
  only. Sorted keys, minimal escaping, custom encoder.
- **File wire format** (this section) — every tracked file. Ordinary
  Go `encoding/json` on a fixed-field struct (declared field order,
  2-space indent, trailing newline). Arrays are always `[]`, never
  `null`; inapplicable fields are present with an explicit
  `null`/zero value, never omitted. **No Go `map` type appears in any
  tracked wire schema** (task 12) — every place rev-1 used a bare
  `map[string]string`/`map[string]interface{}` (`args`, the rev-1
  `current` pointer's implied per-resource index) is instead a sorted
  `[]{key, value}` (for `args`) or `[]{resource_id, batch_id}` (for
  `current.json`'s index) array of a fixed struct, so tracked output
  never depends on `encoding/json`'s map-key-sort behavior at all.

### 12.1 `resources.json` (declaration manifest)

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

(`args` entries are sorted by `key`, byte-ascending — the same sort
order as the canonical-hash encoding, §13.1, though this array and
that hash input are still two independently-defined serializations
that happen to share a sort rule, not the same code path.)

### 12.2 `batches/<batch_id>.json`

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

`raw` is `null` for `git-metadata` (no raw-byte concept applies) and
always a populated `{hash, byte_count}` object for `adapter-snapshot`/
`ignored-file` (no more optional opt-in, §0.2 — the ephemeral bytes
are always hashed before being discarded). `tool_identity` is `null`
for kinds with no adapter/executable concept (`git-metadata`,
`ignored-file`) and populated for `adapter-snapshot`. The example
`oid` above is an ordinary, valid-shaped 64-hex-character SHA-256 Git
object ID (illustrative, not the well-known empty-tree hash, task 10).

### 12.3 `current.json` (the tracked pointer)

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

`resources` is sorted by `resource_id`, byte-ascending, for
determinism (never dependent on map-iteration order, task 12 — this
is a `[]struct`, not a `map`). `latest_batch_id` is the `batch_id` of
the most recent successful publish (§7.3 step 3), regardless of which
specific resources that invocation touched — a convenience field for
"what's the newest batch that exists at all," distinct from the
per-resource index, which is what `diff`/`list --json` actually
resolve against.

### 12.4 First capture, add/remove/change shapes

The **first-ever** capture for a resource produces a `batches/<id>.json`
entry with the exact same schema shape as any subsequent one (§14 has
the `AC` for this) — there is no distinct "initial" schema. `remove`/
`clear` (§3, §4) only ever rewrite `resources.json` and prune
`current.json`'s `resources` array (dropping the entry for the
removed `resource_id` — a resource with no declaration and no
`current.json` entry is simply absent from `list`, matching normal
"never declared" behavior); every `batches/<id>.json` file that ever
existed remains on disk, byte-for-byte, forever (immutable historical
audit trail, task 4).

## 13. Resource ID Canonicalization (task 12, unchanged algorithm)

### 13.1 Canonical `args` encoding

Unchanged from rev-1 (§0.3 — no design in this fold touches the
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
| 2 | `model-picker`, `adapter-snapshot`, `dolt:diff-summary:users`, `dolt`, `diff-summary`, `{"table":"users","from":"main","to":"HEAD"}` (declared `table, from, to` order) | `res_f8a28c218dbb` |
| 3 | Same as Vector 2, `args` declared `to, table, from` order | `res_f8a28c218dbb` (**identical** — order-independence) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

## 14. Acceptance Criteria (task 14)

Clause-level, `AC-<n>` tagged. Each `AC` is one testable clause;
`ADR-033-resource-capture-boundary.md`'s Test Matrix cites these tags
directly.

**Dolt SQL redesign (task 1)**

- `AC-1`: The exact argv `<resolvedDolt> sql -r json -q "<SQL>"` is
  invoked with the exact `dolt_diff_summary('<from>','<to>'[,'<table>'])`
  query shape (§6.2) — no other Dolt subcommand or flag combination is
  ever invoked for `diff-summary`.
- `AC-2`: A value for `from`/`to`/`table` containing a `NUL`/C0
  control byte or a literal backslash is refused (exit 2) before any
  SQL is constructed.
- `AC-3`: A single quote in `from`/`to`/`table` is escaped by doubling
  and round-trips correctly through the invoked query.
- `AC-4`: An unknown or duplicate `--arg` key is refused (exit 2).
- `AC-5`: A rename (differing `from_table_name`/`to_table_name` on one
  row) is tracked verbatim, not collapsed into a same-name
  added+removed pair.
- `AC-6`: `data_change`/`schema_change` are always genuine JSON
  booleans in the tracked `result`, regardless of whether the
  underlying `-r json` output typed them as `0`/`1` or `true`/`false`.
- `AC-7`: The first-ever capture of a resource produces the identical
  `batches/<id>.json` entry schema shape as any later capture (§12.4).

**No version probe; executable identity (task 2)**

- `AC-8`: `dolt version` is never invoked anywhere in the capture
  pipeline.
- `AC-9`: The tracked `tool_identity` contains only `basename` and
  `binary_sha256` — never an absolute path.
- `AC-10`: The Dolt invocation's environment contains only `HOME`/
  `DOLT_ROOT_PATH` (both pointing at ephemeral scratch) — no inherited
  variable from the invoking process's environment.
- `AC-11`: A resolved Dolt executable located inside the repository
  working tree (or under any `.git` directory) is refused
  (`adapter-executable-in-repo`).
- `AC-12`: A resolved Dolt executable whose device/inode/size/mtime
  differ immediately after invocation vs. immediately before is
  refused (`adapter-executable-replaced`) and its result is discarded.

**ADR-027 full compliance, no persistent raw (task 3)**

- `AC-13`: No file under `.tpatch/local/` persists past the
  invocation that created it (verified by asserting
  `resource-scratch/<slug>/` is empty immediately after any
  `capture`/`record --resources` invocation, success or failure).
- `AC-14`: No tracked file anywhere contains a wall-clock timestamp
  field.
- `AC-15`: `feature resource diff` on an `ignored-file` resource
  reports exactly which of `size_bytes`/`hash`/`file_count`/
  `total_bytes`/`combined_hash`/file-set membership changed, never a
  textual line-level diff.
- `AC-16`: A value matching any of the six redaction classes refuses
  the entire invocation (`redaction-refused`), with no partial batch
  written for any resource in that invocation, even unaffected ones.

**Single publication point (task 4)**

- `AC-17`: A successful multi-resource `capture` writes exactly one
  new `batches/<id>.json` file and rewrites `current.json` exactly
  once.
- `AC-18`: A crash simulated between the batch rename and the
  `current.json` rename leaves a permanently orphaned, harmless batch
  file that no subsequent `list`/`diff` ever surfaces.
- `AC-19`: A crash simulated during either temp-file write (before its
  rename) leaves only a `.tmp-*` artifact, swept at the next
  invocation's start, with no effect on the last successfully
  committed `current.json`.
- `AC-20`: `remove`/`clear` never delete or modify any
  `batches/<id>.json` file, only `resources.json` and `current.json`'s
  live index.

**Path gate (task 5)**

- `AC-21`: A selector whose ancestor directory (not the final
  component) is a symlink is refused (`symlink-component-refused`),
  regardless of where that symlink points.
- `AC-22`: A selector replaced by a symlink at the final component
  between the walk and the open is refused via `O_NOFOLLOW`/`ELOOP`.
- `AC-23`: A selector whose underlying file is replaced (different
  inode) between the walk and the open is refused
  (`path-replaced-during-open`).
- `AC-24`: A dangling ancestor (missing path component) is refused
  (`path-missing`).
- `AC-25`: This five-step gate re-runs independently for every
  descendant file of a directory selector, both at `add` and at every
  `capture`.

**Ignored/tracked Git gates, literal pathspecs (task 6, task 7)**

- `AC-26`: `check-ignore` exit `1` (not ignored) and exit `>1` (fatal)
  produce distinct refusal reasons, neither treated as "ignored."
- `AC-27`: `ls-files --error-unmatch` exit `0` (tracked) and any
  non-standard exit/stderr shape produce distinct refusal reasons.
- `AC-28`: A selector shaped like pathspec magic (e.g. leading `:` or
  embedded `**`) is treated as a literal path under
  `--literal-pathspecs`, verified by asserting the ignore/tracked
  checks operate on the exact literal string.
- `AC-29`: The `.tpatch/local/` scratch root itself is verified both
  ignored (via `EnsureLocalIgnoreContract`) and untracked (via the
  `ls-files` gate) before its first write per invocation; either check
  failing refuses before any scratch directory is created.

**Permissions, lock, scratch orphan cleanup (task 8, task 9)**

- `AC-30`: Every ephemeral scratch directory is created `0700` and
  every file `0600` at creation (never via a separate `chmod` after a
  looser-permission create).
- `AC-31`: A fresh lock acquisition succeeds and writes the expected
  PID/`process_start`/host body.
- `AC-32`: A second concurrent invocation for the same slug refuses
  immediately (`capture-in-progress`) while the first is live, with no
  blocking/wait.
- `AC-33`: A lock left by a dead PID (verified via `ps -o lstart=`
  returning no output) is reclaimed automatically by the next
  invocation.
- `AC-34`: A lock whose PID is alive but whose `process_start` differs
  from the recorded value (PID reuse) is reclaimed automatically.
- `AC-35`: A malformed lock file is quarantined and reclaimed
  automatically.
- `AC-36`: A lock recorded on a different hostname is refused
  (`capture-lock-held-remote`) without any reclaim attempt.
- `AC-37`: An orphaned ephemeral scratch directory or `.tmp-*` file
  left by a simulated crash is swept at the start of the next
  invocation for that slug.

**Git metadata / tool fields (task 10)**

- `AC-38`: `head`'s `symbolic_ref` is `null` if and only if `detached`
  is `true`.
- `AC-39`: The `config` view refuses any key outside the exact
  four-key allowlist.
- `AC-40`: An `index-entry` selector queried with a path containing
  pathspec-magic characters resolves to the literal path under
  `--literal-pathspecs`.

**Transaction / `record --resources` (task 11)**

- `AC-41`: `record --resources` on a feature with zero declared
  resources refuses (`no-resources-declared`) before any Git
  invocation.
- `AC-42`: A resource-staging failure combined with Git-side success
  produces `resource-domain-incomplete` with the exact recovery-command
  message, while the Git-side canonical patch is confirmed present and
  correct.
- `AC-43`: A resource-staging failure combined with Git-side failure
  discards the staged (never-written) candidate batch and surfaces
  only record's existing Git-failure behavior.
- `AC-44`: A successful stage and successful Git-side capture publish
  the batch and pointer atomically, verified by asserting both
  `batches/<id>.json` and `current.json` reflect the same invocation
  together, never partially.
- `AC-45`: `feature resource capture <slug> --dry-run` performs zero
  filesystem writes (no scratch directory created, no tracked file
  touched) regardless of whether the underlying capture would have
  succeeded or failed.
- `AC-46`: Re-running `feature resource capture <slug>` after any
  prior failure succeeds without a dedicated recovery command, and is
  idempotent in the sense that each retry produces a correct
  `current.json` regardless of how many prior attempts failed.

**Golden IDs (task 12, unchanged algorithm)**

- `AC-47`: Each of the four golden vectors in §13.3 is independently
  recomputed by the implementation and matches exactly.
- `AC-48`: Vectors 2 and 3 (identical content, differently-ordered
  `args`) produce the identical `resource_id`.

### 14.1 Exact counts (task 14: no false "exactly once" claims)

This PRD defines **48** `AC`-tagged clauses (`AC-1` through `AC-48`,
each an individually testable statement — no range-notation grouping
this time, unlike rev-1's `AC-16`–`AC-21`/`AC-25`–`AC-27`, since
rev-2's redesign happens to decompose cleanly into one tag per clause
without needing an expandable range). The companion ADR's Test Matrix
maps each of these 48 clauses to at least one row; several clauses map
to more than one row (e.g. both a human-output and `--json`-output
verification). The matrix therefore has **more** rows than there are
distinct clauses — this PRD does not claim any clause is covered
"exactly once."

## 15. Open Questions / Negative Consequences

- **`WORKING`/`STAGED` support for `dolt_diff_summary`** (§6.2) is
  explicitly unresolved pending direct source re-verification by the
  implementation cluster; neither this PRD's examples nor its golden
  vectors depend on the answer.
- **Ancestor-directory TOCTOU** (§9.1) is a documented residual risk,
  not fully closed by the Go standard library alone; a future PRD
  could revisit this if a portable `openat2`-equivalent becomes
  available in stdlib.
- **No raw content diffing/versioning** (§2): a real value proposition
  for Dolt/ignored-file resources — seeing an actual textual diff, not
  just "the hash changed" — is deliberately out of scope, and would
  require a future ADR that explicitly supersedes
  `ADR-027-capture-context-privacy-boundary.md`'s committed/local
  split, which this PRD does not attempt.
- **Windows lock/liveness detection** (§7.2) is best-effort/unsupported
  in v1, consistent with this project's existing macOS/Linux-only
  validation scope.
- **Per-invocation lock granularity is per-slug, not global**: two
  different features' `capture` invocations never contend, which is
  intentional (§7.3's concurrency note) but means a single feature
  with many resources cannot parallelize its own staging across
  multiple processes in v1.

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
  rewritten tracked pointer (§7.3, §12.2–§12.3).
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
