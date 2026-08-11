# PRD — Feature Resource Claims & Capture Adapters (rev-1)

**Status**: Draft — rev-1 (supersedes rev-0, writer commit `dd08157`,
adjudicated NEEDS REVISION at `89c8d79`; see `docs/supervisor/LOG.md` →
Cluster H rev-0 internal + external verdicts)

**Owner**: Cluster H implementation lane (planning phase — this document
does not ship code; a future "Cluster H'" implementation cluster consumes
it)

**Related**: `ADR-033-resource-capture-boundary.md` (binding decisions
this PRD assumes), `ADR-027-capture-context-privacy-boundary.md` (D1–D5,
directly extended), `ADR-030-multi-slug-reconcile-derivation-mode.md`
(`.git/**` two-layer exclusion precedent), `ADR-032-feature-unapply-state-boundary.md`
(ID-generation and fixed-struct-JSON precedent), `docs/state-of-the-art/storage-substrate-and-versioned-data.md`
§3, §9 (tracked Dolt/substrate research — see §0.4 below on why this
replaces the untracked WP-006 as the normative citation)

---

## 0. Rev-1 Fold Summary (read this first)

Both rev-0 reviews (internal: 7 HIGH + 1 MEDIUM; external: 3 HIGH + 8
MEDIUM + 3 LOW — `docs/supervisor/LOG.md`) converged on: rev-0 described
committed-artifact behavior that ADR-027 forbids, an under-specified
symlink/path model, a fabricated Dolt JSON contract, false source
citations, and a resource-ID scheme with unverified collision/ordering
guarantees. Rev-1 is a substantial rewrite, not a patch. What is
**preserved** from rev-0 because both reviews agreed it was sound:

- Resources live in a **separate** `resources.json` manifest per
  feature — never inside the canonical patch, never touching
  `apply-recipe.json` or unapply/lifecycle state (ADR-032 territory
  untouched).
- Dolt (or any external tool) is **never** an authority over tpatch's
  own state and is **not** a build/runtime dependency of `tpatch`
  itself — it is invoked, if present, strictly as an external
  read-only probe.
- Replay/backward-compatibility remains **Git-only** — a repository
  with no resources declared is byte-identical in behavior to today's
  `tpatch`, and resource data is never required to reconstruct or
  replay a canonical patch.

Everything else below is rewritten. §0.1–§0.4 map each rev-0 finding to
its rev-1 resolution; §1 onward is the standalone rev-1 design.

### 0.1 Claims Audit (rev-1)

Every normative claim in this PRD that names a source file is verified
against the file at the commit this document was written against
(`HEAD` at the time of writing: post-`89c8d79`, pre-rev-1-commit). Rows
marked "rev-0 error" identify a claim rev-0 made that this audit found
false; rev-1 does not repeat it.

| # | Claim | Citation | rev-0 status |
|---|-------|----------|---------------|
| C1 | `feature` is the noun-scoped home for per-feature verbs (`claim`, `deps`, and now `resource`) | `internal/cli/feature_deps.go:38-51` (`featureCmd` doc comment) | rev-0 **error**: cited a nonexistent "ADR-031 D10" for this claim in two places (Claims Audit row + §3.1 candidate-command table). ADR-031's real D10 (`docs/adrs/ADR-031-rejected-feature-state-data-model.md:889`) is about the `reject` verb naming disposition and is unrelated. Corrected here to cite the actual source. |
| C2 | `safety.EnsureSafeRepoPath` and `store.NormalizeClaimPath` are **lexical-only** — `filepath.Abs` + string-prefix containment, no `Lstat`, no `filepath.EvalSymlinks` | `internal/safety/safety.go:1-28` (whole file); `internal/store/claims.go` (`NormalizeClaimPath`, calls `safety.EnsureSafeRepoPath` with no symlink resolution) | rev-0 **error**: rev-0's ADR D-series described this pair as "symlink-aware validation" and relied on it as the sole gate for resource selectors. It is not symlink-aware. §9 below defines the new, additional symlink-resolution gate this PRD requires. |
| C3 | `gitutil.IsPathIgnored` invokes `git check-ignore -q --no-index` | `internal/gitutil/ignore.go:1-78` (whole file, the invocation itself) | rev-0 **error** (by omission): rev-0 treated "ignored" as sufficient for the `ignored-file` resource kind. `--no-index` means Git does not consult the index when answering, so an already-**tracked** file that also matches a `.gitignore` pattern still reports "ignored" here. §5.1/§8 below add a mandatory second, explicit tracked-file gate (`git ls-files`) that rev-0 lacked. |
| C4 | `internal/cli/session_redaction.go` is unexported (`redactSessionForCommit`, `forbiddenContentClasses` are package-private), operates on `store.SessionObservation` (observation-shaped, not a generic byte/string scanner), and uses heuristic **drop-line-and-continue** logic across 10 classes that do **not** include dedicated PEM/OpenSSH-key, DB-connection-URL, or email/PII patterns | `internal/cli/session_redaction.go` (whole file) | rev-0 **error**: rev-0's Privacy section implied this existing mechanism already covered resource-capture content and could be reused as-is. It cannot: it is unexported, shaped for a different data structure, and missing three of the six closed classes this PRD now requires (§8). |
| C5 | `ExitCodeError` (`internal/cli/exit_error.go`) is constructed and returned by **at least six** commands: `c1.go`, `doctor.go`, `feature_deps.go`, `reconcile_check_applied.go`, `reject.go`, `verify.go` | `internal/cli/exit_error.go` (type definition); `grep -rn "&ExitCodeError{" internal/cli` (six call sites as of this writing) | rev-0 **error**: rev-0's Claims Audit asserted `verify` was the "sole" pre-existing user of a bespoke exit code 2/3 pair, echoing `SPEC.md`'s own explicit disclaimer that exit codes are **per-command contracts, not a global enum** (`SPEC.md` §"Exit-code envelope": "`tpatch verify` has its own, unrelated exit-2 meaning"). Rev-1's exit-code table (§10.5) is one more per-command contract in this family, not a redefinition of any existing command's codes. |
| C6 | `internal/cli/feature_claim.go` establishes the `add`/`list`/`remove`/`clear` verb quartet under a noun subcommand, including the exact `"no such feature: %s"` refusal shape | `internal/cli/feature_claim.go:1-207` (whole file) | rev-0: accurate, kept as the CLI-shape precedent for `feature resource {add,list,remove,clear}`. |
| C7 | `docs/adrs/ADR-027-capture-context-privacy-boundary.md` D1 requires that any future PRD choosing a worktree-local, gitignored storage path "MUST verify that path is ignored before the first write and MUST refuse rather than risk accidental commit" | `docs/adrs/ADR-027-capture-context-privacy-boundary.md` D1 (verified against current file text) | rev-0 **partial error**: rev-0 cited ADR-027 generally but did not implement the ignored-before-first-write refusal for its own proposed local buffer, and separately proposed committing raw snapshot/diff bodies in tracked sidecars — directly contradicting D1's committed/local split. §7 below implements the refusal; §4/§8 remove all committed-raw-body claims. |
| C8 | `docs/state-of-the-art/storage-substrate-and-versioned-data.md` §3 and §9 are **tracked** (committed) files; `docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md` is **untracked** as of this writing (`git status --short` shows `??`) | `git status --short docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md` (untracked marker); `docs/state-of-the-art/storage-substrate-and-versioned-data.md` §3, §9 (tracked, present in `git ls-files`) | rev-0 **error**: rev-0 cited WP-006 normatively for Dolt/substrate guidance. A docs-only PRD/ADR pair cannot depend normatively on a file with no guarantee of ever being committed. Rev-1 cites the tracked storage-substrate research document instead; WP-006 is not cited at all in rev-1 (not even non-normatively, to avoid any residual dependency). |
| C9 | Real Dolt CLI: `dolt diff` has no literal `--json` flag — JSON output is `-r json` / `--result-format json`; the per-row field schema of that output is not reliably documented and is treated as opaque | Primary source, `dolthub/dolt` at commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9`: `go/cmd/dolt/commands/diff.go` (synopsis `dolt diff [options] <commit> <commit> [tables...]`, `--result-format`/`-r` accepting `tabular`/`sql`/`json`, `--schema`/`--data` selection), `go/cmd/dolt/commands/version.go` (`dolt version [--verbose] [--feature]`, prints `dolt version <version>`); cross-checked against the DoltHub CLI reference (`https://www.dolthub.com/docs/cli-reference/cli/`, fetched during rev-1 research); see §6 for the exact verified flag list this PRD relies on | rev-0 **error**: rev-0's adapter protocol examples showed a fabricated `dolt diff --json` invocation and a fabricated per-row JSON schema (invented field names). §6 replaces this with a design that uses only verified, stable flags for anything tracked, and treats richer output as an opaque local-only blob. No claim in this PRD relies on `dolt status --porcelain`, which was not found in either the source or the docs reference. |
| C10 | `internal/store/claims.go` `RemoveClaim` spans lines 436-444 | `internal/store/claims.go:436-444` | Corrects a stale line-range drift noted during this audit; re-verified against current file content. |

### 0.2 What rev-1 removes entirely

- The `generic-command` resource kind/adapter and its shell-tokenizer +
  user-declared-env-allowlist machinery (task letter C). Closed
  external-adapter set for v1 is **Dolt only**. A future ADR is
  required before any sandboxed/consented generic external-command
  capability is added; this PRD takes no position on what that ADR
  will decide.
- The impossible `.git` "exfiltration" acceptance criterion rev-0 wrote
  for `generic-command` (it depended on the removed kind and asserted
  something no sandboxless argv executor can actually guarantee).
- The claim that committed sidecars ever contain raw snapshot/diff
  bytes, raw adapter stdout, or raw file content of any kind.
- The claim of a single cross-tree atomic delete across tracked and
  local state (§7.4).
- The `git config` view's access to `user.name`, `user.email`, and
  wildcarded `core.*`/`remote.*.url`/`branch.*.merge` keys (§5.3).
- Any literal `record --dry-run` framing (rev-0 never wrote a bare
  `record --dry-run` without `--resources`, but did overload
  `record --resources --dry-run` onto the same command that performs
  the Git-side canonical-patch capture; rev-1 moves all dry-run and
  resource-only preview/retry flows onto the standalone
  `feature resource capture` verb — see §10).

### 0.3 Golden resource-ID vectors

Computed and reproduced deterministically (SHA-256, Python `hashlib`,
see §12 for the exact byte-string construction). These four vectors
are embedded byte-identically in both this PRD (§12.3) and
`ADR-033-resource-capture-boundary.md` D3:

| Vector | Feature | Kind | Selector | Adapter | Capability | Args (declaration order) | `resource_id` |
|---|---|---|---|---|---|---|---|
| 1 | `model-picker` | `git-metadata` | `head` | *(none)* | *(none)* | `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker` | `adapter-snapshot` | `dolt:schema-diff:users` | `dolt` | `schema-diff` | `table=users, from=main, to=HEAD` | `res_19b4675405e2` |
| 3 | `model-picker` | `adapter-snapshot` | `dolt:schema-diff:users` | `dolt` | `schema-diff` | `to=HEAD, table=users, from=main` (reordered) | `res_19b4675405e2` (**identical to Vector 2** — proves key-order independence) |
| 4 | `model-picker` | `ignored-file` | `config/local-secrets.env.template` | *(none)* | *(none)* | `{}` | `res_79f5ac5dca13` |

### 0.4 Requirement-letter → section map

| Letter | Requirement | Section(s) |
|---|---|---|
| A | Privacy/authority — no committed raw bodies | §4, §7, §8.1 |
| B | Redaction — closed hard-refusal classes | §8 |
| C | Generic-command removed | §0.2, §6 |
| D | Symlink/path safety | §9 |
| E | Ignored + untracked, limits, binary/multi-file | §5.1, §7.2 |
| F | Git metadata scope | §5.3 |
| G | Dolt exact syntax | §6 |
| H | Resource ID canonicalization | §12 |
| I | Transaction/batch design | §7 |
| J | `record --resources` semantics | §10 |
| K | Wire schemas | §11 |
| L | ACS/matrix rebuild | §13 |
| M | Citations/tracking | §0.1, throughout |
| N | Additional review details | §5.1, §6.4, §7.3, §9, §10.4 |

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
point-in-time, privacy-safe **summary** of each into a tracked
per-feature manifest, with any raw content kept strictly local and
opt-in.

**Non-goals** (unchanged from rev-0, reaffirmed): resources are not
inputs to `apply`/`unapply`/reconcile; resources do not gate `land`;
resources are not a general-purpose secrets vault; resources do not
make Dolt (or any external tool) a runtime dependency of `tpatch`;
resources do not support arbitrary sandboxless external commands
(§0.2); this PRD does not change any existing command's exit-code
contract (C5) or `record`'s existing Git-side capture-mode semantics
(mutex group, empty-patch handling, `--auto`/range resolution) — it
only adds an orthogonal `--resources` flag (§10).

## 3. Command Surface

All new verbs live under the existing `feature` noun (C1), mirroring
`feature claim`'s shape (C6):

```
tpatch feature resource add    <slug> --kind <kind> --selector <sel> [--adapter <a> --capability <c> --arg k=v ...] [--keep-local] [--json]
tpatch feature resource list   <slug> [--json]
tpatch feature resource remove <slug> <resource-id-or-prefix> [--json]
tpatch feature resource clear  <slug> [--json]
tpatch feature resource capture <slug> [--resource <resource-id-or-prefix>] [--dry-run] [--json]
tpatch feature resource diff   <slug> [--resource <resource-id-or-prefix>] [--json]
tpatch record <slug> [existing flags...] [--resources] [--json]
```

`capture` and `diff` are two distinct verbs (rev-0 conflated them into
one overloaded `diff` that both executed adapters and persisted
results, which reads as read-only but wasn't):

- **`capture`** is the only verb that ever executes adapters, reads
  local/ignored file content, or reads Git metadata, and the only verb
  that ever writes tracked (`resources/<id>/summary.json`) or local
  (`.tpatch/local/resource-capture/...`) state. `--dry-run` runs the
  full staging pipeline (including redaction) and prints what would be
  written, but performs no writes at all — not even to the local
  scratch tree.
- **`diff`** is read-only and never executes an adapter or touches the
  local tree: it loads the last-published tracked summary (if any) and
  reports it. Called before any capture has ever run for a resource,
  it reports "no capture yet" (not an error — exit 0, per §10.5's
  distinction between "nothing to show" and "refused").

`add`/`list`/`remove`/`clear` behave exactly as `feature claim`'s
quartet does (same `"no such feature: %s"` refusal shape, same
`--json` flag convention, C6), except `add` additionally computes and
persists the `resource_id` (§12) and rejects duplicates (same
`selector`+`kind`+`adapter`+`capability`+canonical `args` tuple already
present for that feature) as a validation error (exit 2).

`--keep-local` (an `add`-time, per-resource, persisted boolean field,
default `false`) is the explicit opt-in required before `capture` ever
writes raw bytes to the local tree (§7.3, §8.1). It has no effect on
tracked output, which never contains raw bytes regardless of this
flag.

## 4. Data Model

Two tracked files per feature, both under the existing per-feature
artifacts directory (`store.featureArtifactsDir`, `internal/store/store.go`),
never inside `apply-recipe.json` or any unapply/lifecycle-state file:

- **`resources.json`** — the declaration manifest. One entry per
  declared resource: `resource_id`, `kind`, `selector`, `adapter`
  (empty string if not applicable to the kind), `capability` (empty
  string if not applicable), `args` (a string-keyed map, may be
  empty), `keep_local` (bool), `added_at` (RFC3339 UTC, matching the
  existing timestamp convention elsewhere in the store package),
  `added_by_tool_version` (the `tpatch` version string, informational
  only). **Never** contains raw content, hashes, or capture results —
  those live only in the per-resource summary.
- **`resources/<resource_id>/summary.json`** — the tracked capture
  result, written only by `capture` (§10), read by `diff` and `list
  --json`. Contains: the resolved `kind`/`selector`/`adapter`/
  `capability`/`args` (redundant copy, so a summary is self-describing
  without cross-referencing `resources.json`), a `captured_at`
  timestamp, `tool_identity` (executable absolute path + version
  string, adapter kinds only), a structural `changes` block (§11) that
  is **always** hashes/counts/classifications, **never** raw bytes,
  a `raw` block that is **only ever a hash + byte count + presence
  flag** (§11.4), and `local_batch_id` (the local batch this summary's
  raw companion lives under, or `null` if `keep_local` was `false` for
  this capture — see §4.1 on why this is per-capture, not per-resource).

### 4.1 Why `local_batch_id` is per-capture, not per-resource

`keep_local` is a declared, persisted intent (§3) checked at every
`capture`, but whether a *given* capture actually produced a local
raw companion also depends on capture-time success (§7.1) and on
`--dry-run` (which never writes locally regardless of `keep_local`).
`summary.json` therefore records the batch ID that was actually
produced for *this* capture, or `null` — never inferred from the
`resources.json` declaration.

### 4.2 Missing-local behavior (closes rev-0's undefined gap, task A)

If `local_batch_id` is non-null in `summary.json` but the referenced
local batch directory is absent (pruned out-of-band, moved to another
machine, or a fresh clone/worktree that never had `.tpatch/local/`
populated), `diff` and `list` are **unaffected** — they only ever read
tracked fields and never resolve `local_batch_id` into a filesystem
read. Only a dedicated inspection affordance (`feature resource
capture <slug> --dry-run` re-run, or a future explicit "show local
raw" verb, out of scope for this PRD) would attempt to open the local
path, and it reports `local raw capture not present for this summary`
(exit 0, informational, not a failure — the tracked summary is
authoritative and complete on its own).

## 5. Resource Kinds

Three kinds in v1, closed set (no plugin mechanism):

### 5.1 `ignored-file` (task E, task N)

Selector: a repo-relative path. **Both** of the following must hold at
`add` time, and are **rechecked at every `capture`** (not just once):

1. `gitutil.IsPathIgnored(path) == true` (`git check-ignore -q
   --no-index`, C3).
2. The path is **not currently tracked**: a new gate, `git ls-files
   --error-unmatch <path>` (exit non-zero means "not tracked," per
   `git-ls-files(1)`), must confirm the path is absent from the
   index. This closes the exact `--no-index` gap in C3 — an
   already-tracked file that also matches a `.gitignore` pattern is
   **refused**, not silently accepted, because `--no-index` alone
   would say "ignored" for it. (Task N's "add a tracked-file gate.")

Refusal (exit 3, state/policy) if either check fails, both at `add`
and at `capture` — a file that was untracked-and-ignored at `add` time
but has since been `git add`ed is refused at the next `capture`, not
silently captured as if nothing changed.

Rev-0 additionally promised acceptance for paths that are merely
"untracked but not ignored" (e.g. a build artifact nobody bothered to
`.gitignore`). **Removed** in rev-1: an untracked-but-not-ignored path
is ambiguous — it may become tracked at any moment with no signal to
tpatch, and its presence in the repo may be entirely accidental. Only
the ignored-and-untracked intersection is accepted.

**Selector scope and limits**: a selector may name a single file or a
directory. For a directory selector, every regular file found by a
bounded recursive walk (symlinks excluded per §9; the walk does not
follow directory symlinks) must independently pass both checks above.
Exact limits, checked at `add` and re-checked at every `capture`
(snapshot-time bounds, not just declaration-time — task N):

- Per-file size: **5 MiB**. A single file over this limit fails the
  whole capture for that resource (exit 3), naming the offending path.
- Total directory size: **20 MiB** across all files matched by the
  selector at capture time.
- File count: **200 files** matched by the selector at capture time.

Exceeding any limit at `capture` time is a state refusal (exit 3),
not silently truncated — partial/truncated capture of directory
content would be a misleading tracked summary.

**Binary detection**: content is classified `binary` if it contains a
NUL byte in the first 8 KiB read (matching the common Git/`file(1)`
heuristic), else `text`. This classification is recorded in the
tracked summary (§11.2) but never changes whether redaction (§8) runs
— the six closed classes are byte-pattern-based and run identically
over binary or text content.

**No text normalization on raw local bytes**: unlike the `git-metadata`
and `adapter-snapshot` kinds (§5.2, §5.3), whose structural output is
normalized before hashing (line-ending and whitespace normalization,
§11.5, so hash stability doesn't depend on the invoking OS), an
`ignored-file` resource's local raw copy (when `keep_local=true`) is a
byte-for-byte verbatim copy — no normalization of any kind. The
**hash** recorded in the tracked summary is computed over these same
verbatim bytes, so the tracked hash is reproducible from the local
raw copy without any transform, if the local copy exists (§4.2).

**Multi-file local manifest**: when the selector is a directory and
`keep_local=true`, the local batch (§7) contains a `files/` subtree
mirroring the matched relative paths, plus a local-only
`manifest.json` listing each file's relative path, size, and hash —
this manifest is **local-only**, never copied into the tracked
summary, which instead records only the directory-level aggregate
(file count, total bytes, combined hash — §11.2).

### 5.2 `git-metadata` (task F)

Four closed views, selector is the view name plus (for `ref`) the ref
name appended after a colon:

| View | Selector | Content (tracked, structural only) |
|---|---|---|
| `head` | `head` | The symbolic ref name HEAD currently points to (e.g. `refs/heads/main`), or the literal string `detached` plus the resolved OID if HEAD is detached. |
| `ref` | `ref:<refname>` where `<refname>` matches `^refs/(heads|tags)/[A-Za-z0-9._/-]+$` | The resolved OID of exactly that one ref, resolved via `git rev-parse --verify <refname>`. No bulk ref dump — one explicitly named ref per resource. |
| `index-entry` | `index-entry:<path>` | The index entry's mode, OID, and stage number for that one repo-relative path, via `git ls-files -s -- <path>` (refused, exit 2, if the path has zero or more than one matching index entry — a merge-conflict path with multiple stages is out of scope for v1 and must be refused rather than silently picking one). |
| `config` | `config:<key>` where `<key>` is one of exactly `core.filemode`, `core.ignorecase`, `core.symlinks`, `extensions.objectformat` | The single resolved value of that key via `git config --get <key>` (missing key is recorded as `null`, not an error — these keys are frequently unset and rely on Git's built-in default). |

Rev-0's `config` view allowed `user.name`, `user.email`, wildcard
`core.*`, `remote.*.url`, and `branch.*.merge` — all **removed**. The
four keys retained are non-PII, boolean-or-enum filesystem/format
settings with no realistic sensitive content; every other key,
including any wildcard expansion, is refused at `add` (exit 2,
validation — the view's selector grammar does not even parse an
unlisted key as valid input, so this is a shape error, not a runtime
policy refusal).

`ref` and `index-entry` selectors are re-resolved fresh at every
`capture` (the OID a ref points to, or an index entry's OID/mode, may
have changed since `add`) — there is no caching of the resolved value
in `resources.json`, only in the per-capture `summary.json`.

### 5.3 `adapter-snapshot` (tasks C, G)

Closed adapter set: **`dolt` only**. Selector shape:
`dolt:<capability>:<table>`, where `<capability>` is one of
`schema-diff` or `table-diff`, and `<table>` is a Dolt table name.
`--arg table=<table> --arg from=<ref> --arg to=<ref>` are all
**required** (exit 2 if any is missing); `from`/`to` are Dolt
revision specs (branch name, commit hash, or `HEAD`/`WORKING`/
`STAGED`). Any `--arg` key outside `{table, from, to}` is a validation
error (exit 2); a duplicate `--arg table=...` (same key given twice)
is also a validation error (exit 2) — never silently "last one wins."

See §6 for the exact adapter execution contract.

## 6. Adapter Protocol — Dolt (task G, task N)

Verified against the primary `dolthub/dolt` source at commit
`59fb843bf6a4b653d7c8b6d997a603b10cf279d9` — specifically
`go/cmd/dolt/commands/diff.go` (synopsis `dolt diff [options] <commit>
<commit> [tables...]`, `--schema`/`--data` selection,
`--result-format`/`-r` accepting `tabular`/`sql`/`json`) and
`go/cmd/dolt/commands/version.go` (`version [--verbose] [--feature]`
subcommand, output format) — cross-checked against the DoltHub CLI
reference (`https://www.dolthub.com/docs/cli-reference/cli/`) during
rev-1 research for the remaining flags below (`--filter`,
`--name-only`) that the source check did not separately confirm.
Nothing in this section is a guessed or invented flag or output
schema. This PRD does not rely on `dolt status --porcelain`
anywhere — no such flag was found in the source search, and no claim
here depends on it.

### 6.1 Executable resolution and probe

The adapter locates `dolt` via `exec.LookPath("dolt")` at `capture`
time (not at `add` time — `add` only validates selector/arg shape,
never touches the filesystem or spawns a process). If not found:
`adapter-missing` (exit 3, state refusal, distinct from a validation
error because the *declaration* was valid — only the environment
lacks the tool).

Once resolved, before running the requested capability, the adapter
runs a **probe**: `dolt version` with a 5-second timeout and a 4 KiB
captured-output cap. Per `go/cmd/dolt/commands/version.go`
(`dolt version [--verbose] [--feature]`), the bare `dolt version`
invocation prints a single line, `dolt version <version>` (confirmed
against source — no subcommand output requires JSON/structured
parsing for the probe). Probe outcomes and their exact treatment
(task N: "probe failure semantics must be explicit"):

| Probe outcome | Treatment |
|---|---|
| Exits 0, output matches `^dolt version \S+` | Proceed; the matched version string is recorded as `tool_identity.version` in the summary. |
| Exits 0, output does not match the expected pattern | `adapter-probe-unexpected-output` (exit 3) — the binary at this path is not recognized as Dolt (or is a version whose output shape changed in a way this PRD did not anticipate); refuse rather than guess. |
| Nonzero exit | `adapter-probe-failed` (exit 3), with the exit code and first 4 KiB of combined output recorded **locally only** (§7, diagnostics — never in the tracked summary). |
| Timeout (>5s) | `adapter-probe-timeout` (exit 3). The probe process is sent `SIGTERM`; if it has not exited 2 seconds later, `SIGKILL`. |
| `exec.LookPath` resolves to a path, but `Lstat`/`EvalSymlinks` on that resolved path lands inside the repository working tree | Refused before the probe even runs: `adapter-executable-unsafe` (exit 3) — the symlink/path safety gate (§9) applies to the resolved adapter executable path exactly as it does to any other path this feature touches, closing task D's "in-repo executable" case. |

### 6.2 Capability invocation — exact argv templates


Both capabilities run inside the repository's Dolt database directory
(`cwd` = the repo root, matching where `tpatch` itself always runs
from; this PRD does not support a Dolt database at a different path in
v1 — a future ADR would be required to add that).

**`schema-diff`**: `dolt diff --schema --name-only <from> <to>
<table>` — three separate invocations are actually run, varying only
`--filter`:

```
dolt diff --schema --name-only --filter=added   <from> <to> <table>
dolt diff --schema --name-only --filter=dropped <from> <to> <table>
dolt diff --schema --name-only --filter=modified <from> <to> <table>
```

`--filter`'s accepted values (`added`, `modified`, `renamed`, `dropped`,
with `removed` as a documented alias for `dropped`) and `--name-only`
are both confirmed present in the DoltHub reference. Whichever of the
three invocations returns non-empty `--name-only` output for `<table>`
determines that table's classification for this capture: `added` /
`removed` (dropped) / `changed` (modified) — recorded structurally in
the tracked summary (§11.2), never the invocation's raw output.

**`table-diff`**: same three-invocation pattern, using `--data
--name-only` instead of `--schema --name-only`, to classify rows as
added/removed/changed **by table name**, not by row content — this
PRD's tracked output is a per-table row-change *classification*, never
row-level content or row counts (row counts would require parsing
Dolt's richer output, which §6.3 explicitly refuses to fabricate a
schema for).

### 6.3 Why `-r json` / `--result-format json` is never used for tracked output

`go/cmd/dolt/commands/diff.go` at commit
`59fb843bf6a4b653d7c8b6d997a603b10cf279d9` confirms `-r`/
`--result-format` accepts `tabular`, `sql`, or `json`, and confirms
there is **no separate bare `--json` flag** (rev-0 fabricated one).
Rev-1 research found the exact per-row field schema of `-r json`
output is not consistently documented across Dolt versions or
independently confirmed against source (informal sources disagree on
field names). Rather than encode an unverified schema into a tracked,
supposedly-stable artifact, rev-1 draws a hard line: **any Dolt
invocation beyond the three `--name-only --filter=...` calls above is
only ever run when `keep_local=true`**, using `--result-format json`
purely as a richer local diagnostic, and its output is stored
**only** in the local batch (§7) as an opaque blob (hash + byte count
recorded in the tracked summary, exactly like `ignored-file`'s raw
bytes — §11.4), never parsed into any tracked structured field.

### 6.4 Timeouts, caps, environment (task N: exact examples)

| Parameter | Value |
|---|---|
| Per-invocation timeout | 30 seconds. On timeout: `SIGTERM`, then `SIGKILL` after 2 more seconds if still running. |
| Captured output cap | 5 MiB combined stdout+stderr per invocation; output beyond the cap is truncated and the truncation is recorded in local diagnostics (never in the tracked summary, which never contains raw output at all). |
| Environment passthrough | Exactly `DOLT_ROOT_PATH`, `DOLT_CONFIG_PATH`, `HOME`, `PATH`, forwarded from the invoking process's environment if set; no other environment variables are passed, and (since `generic-command` is removed, task C) there is no user-declarable env-allowlist mechanism at all in v1. |
| Termination | Process group termination (the child and any of its own children) to avoid orphaned Dolt subprocesses on timeout. |

A concrete, fully-specified example for Vector 2 (§0.3) — the exact
argv array run for the `schema-diff` capability with `table=users,
from=main, to=HEAD` (task N: "exact adapter examples must include
selector/discriminator and all declared capability args"):

```
selector:     dolt:schema-diff:users
capability:   schema-diff
args:         {"table":"users","from":"main","to":"HEAD"}

argv[0]: dolt diff --schema --name-only --filter=added   main HEAD users
argv[1]: dolt diff --schema --name-only --filter=dropped main HEAD users
argv[2]: dolt diff --schema --name-only --filter=modified main HEAD users
```

## 7. Local Storage & Transaction Design (task I)

### 7.1 Layout

```
.tpatch/local/resource-capture/<slug>/
  current                       -- plain-text file, contents = one batch ID, atomically updated
  batches/
    lb_<12 lowercase hex>/       -- one immutable batch, written once, never mutated after commit
      meta.json                 -- local-only diagnostics (see 7.3)
      <resource_id>/
        raw                     -- present only if keep_local=true and this resource is
                                    ignored-file/adapter-snapshot with raw bytes to keep
        files/<relpath>          -- present only for a directory ignored-file selector
        manifest.json            -- present only for a directory ignored-file selector (5.1)
```

Batch IDs are 12 lowercase-hex characters from `crypto/rand`, mirroring
the `ua_` attempt-ID precedent (`ADR-032` D3, Implementation Notes item
8) — collision probability is the same 48-bit space already accepted
there.

Per ADR-027 D1's exact mandate (C7), `capture` verifies
`.tpatch/local/` is `.gitignore`d **before its first write on a given
machine** and refuses (`local-path-not-ignored`, exit 3) if it is not
— this is a one-time-per-worktree check (cached in-process per
invocation, not persisted), not a per-file recheck, since the ignore
rule covers the whole subtree by construction (a single `/.tpatch/local/`
line is the expected `.gitignore` entry; this PRD documents that entry
as a required addition but does not itself edit `.gitignore` — that is
an implementation-cluster task).

### 7.2 Commit protocol

1. `capture` allocates a fresh batch ID and writes the batch's full
   contents under a temporary sibling name, `batches/.tmp-lb_<id>/`.
2. Every file within is written, then `fsync`'d individually; the
   temporary directory itself is then `fsync`'d (POSIX directory-fsync
   pattern, matching the durability posture used elsewhere in the
   store package for tracked writes).
3. The temporary directory is `rename`'d to its final name,
   `batches/lb_<id>/` — a single POSIX `rename(2)`, atomic within the
   same filesystem, which this design requires (`.tpatch/local/` must
   be on the same filesystem as `.tpatch/` itself; this PRD does not
   support a symlinked or bind-mounted `.tpatch/local/` on a different
   filesystem in v1).
4. Only **after** step 3 succeeds does `capture` update `current`: a
   new `.tmp-current` file is written with the batch ID, `fsync`'d,
   then `rename`'d over `current` — again one atomic POSIX rename.

Advisory concurrency lock: `capture` opens `resource-capture/<slug>/.lock`
with `O_CREATE|O_EXCL` before step 1 and removes it after step 4 (or on
any failure exit). A second concurrent `capture` for the same slug that
finds the lock held refuses immediately: `capture-in-progress` (exit 3)
— it does not wait or queue.

### 7.3 Crash-window analysis (task I, task N)

| Crash point | Resulting state | Recovery |
|---|---|---|
| Mid-write inside `.tmp-lb_<id>/` | An orphaned temp directory; `current` still points at the prior batch (or is absent, for a first capture) | Inert. A future housekeeping pass may delete any `batches/.tmp-lb_*` directory (no capture ever reads a `.tmp-` prefixed directory); until then it is harmless disk usage. No tracked state is affected. |
| After step 3 (batch committed) but before step 4 (`current` not yet updated) | A fully-written, valid `lb_<id>/` batch exists but is not pointed to by `current` | Harmless — this batch is simply never read (nothing resolves it without going through `current`). Re-running `capture` produces a new batch and retries the `current` update; the orphaned batch is prunable, not corrupt. |
| After step 4, before the tracked `summary.json` write (§10 publish step) | `current` points at a fully valid local batch that has not yet been reflected in any tracked file | Re-running `capture` (or `record --resources`, §10) recomputes fresh and republishes; no special recovery command is needed because there is no partially-written tracked state to clean up (tracked writes are a separate atomic step, §10.2). |

"Recovery" in every case above is simply **re-running the capture
command** — there is no dedicated recovery verb, because no crash
window described above leaves tracked state in an inconsistent form;
the local tree is append-only-by-batch and self-healing by
construction (task I: "no special recovery command needed").

**Failures write no tracked artifact at all** (task I, task N — "no
batch failure envelope contradiction"): if staging fails for any
reason (adapter error, redaction refusal, size-limit exceeded), no
`summary.json` is written or modified, and no partially-written batch
is ever promoted via `current`. Diagnostics about *why* a capture
failed are written only to `batches/lb_<id>/meta.json` inside a batch
that — because the failure means step 4 was never reached — is never
pointed to by `current` and is therefore purely a local, informational
artifact, never a tracked "failure record." Rev-0 had contradictory
language suggesting both "no tracked failure envelope" and a
`history/NNN-diff.json`-style append-on-every-attempt model that would
have required *some* tracked artifact per attempt, including failed
ones; rev-1 resolves this by removing the numbered-history model
entirely (§7.4) — there is nothing to contradict once tracked state is
a single current summary per resource, written only on success.

### 7.4 History model, `remove`/`clear` (task I)

There is no numbered append-only tracked history (`history/NNN-*.json`)
in rev-1. The **tracked** artifact for each resource is exactly one
`summary.json`, overwritten on each successful `capture` (the prior
tracked content is not retained anywhere tracked — Git's own history
of the file, once committed, is the audit trail for tracked summaries,
exactly as it already is for every other tracked tpatch artifact).

The **local** batches are the append-only, immutable history — each
`capture` produces a new `lb_<id>/` batch that is never overwritten or
deleted by any command in this PRD. This local history is prunable
only manually/out-of-band (no `tpatch` command deletes old batches in
v1) — an explicitly accepted, documented cost (§14 Negative
Consequences: unbounded local disk growth until pruned).

`feature resource remove <slug> <id>` and `feature resource clear
<slug>` delete **only** the tracked `resources.json` entry/entries and
the corresponding tracked `resources/<id>/summary.json` file(s) —
ordinary sequential file operations, **not** claimed to be atomic
across the tracked and local trees (rev-1 explicitly does **not**
repeat rev-0's "atomic cross-tree clean delete" claim, task I). They
do **not** touch `.tpatch/local/resource-capture/<slug>/` at all — the
local batch history for a removed resource is preserved exactly as it
was, on the reasoning that local history is an audit trail that
`remove`/`clear` (tracked-declaration operations) should not silently
destroy. A future explicit `feature resource purge-local` verb (out of
scope here) would be the place to add deliberate local-history
deletion.

## 8. Privacy & Redaction (task A, task B)

### 8.1 Committed content is always structural, never raw (task A)

No tracked file (`resources.json`, any `resources/<id>/summary.json`)
ever contains: raw file bytes, raw adapter stdout/stderr, a full Git
object's content, or any string copied verbatim from a scanned
source. Tracked content is limited to: hashes (`sha256`, hex-encoded),
byte counts, file counts, path/table/ref names that are themselves the
*declared selector* (not scanned content), structural
added/removed/changed classifications, and the six-class redaction
**pass/refuse** verdict (never the matched substring itself, which
would defeat the purpose — task B: "closed classes," not "closed
classes, logged verbatim").

Raw bytes exist **only** under `.tpatch/local/resource-capture/` (§7),
and only when `keep_local=true` was declared at `add` time (§3). This
directly corrects rev-0's claim of "verbatim inheritance" of
snapshot/diff content into tracked sidecars (task A: "correct any
claim of verbatim inheritance") — there is no verbatim inheritance
into any tracked file in rev-1, full stop.

### 8.2 The hard-refusal scanner (task B)

`internal/cli/session_redaction.go` today is: unexported
(`redactSessionForCommit`, `forbiddenContentClasses` are
package-private), shaped around `store.SessionObservation` (not a
generic byte scanner), and applies **drop-the-offending-line-and-continue**
semantics across 10 heuristic classes (secret-like-string,
absolute-home-path, prompt-text-marker, tool-call-argument,
command-output-marker, stack-trace-marker, ide-buffer-marker,
clipboard-marker, vector-embedding-payload, source-snippet-marker) —
none of which are a dedicated PEM/OpenSSH-key, DB-connection-URL, or
email/PII pattern (C4). This PRD does not change that existing
mechanism's behavior for its existing surface (session summaries) —
it is out of scope here.

This PRD **requires** the implementation cluster to extract the
existing byte-pattern matchers that are reusable (`matchSecretLikeString`'s
bearer/token/API-key-prefix patterns, `matchAbsoluteHomePath`) into a
new, **exported**, content-agnostic location (e.g. a new `internal/redact`
package) with a `Scan(content []byte) []string` entry point that
operates on raw bytes (no `store.SessionObservation` dependency, no
UTF-8-validity requirement — every pattern below is ASCII-marker-based
and byte-safe against binary input), returning the set of matched
class names. Both the existing session-redaction call site and the new
resource-capture call site (below) consume this shared scanner, but
apply **different policies** to its findings — the existing surface
may keep its drop-and-continue policy (unchanged, out of scope); the
new resource-capture surface below always hard-refuses.

**Six closed classes** (task B, exact list), applied to every tracked
selector, summary field, and metadata value before it is written
anywhere (tracked or local) — a match on **any** class is a hard
refusal of the entire capture (`redaction-refused`, exit 3), never a
partial scrub-and-continue (matching ADR-027 D3's existing "redaction
failure is a hard failure" posture, C7):

1. **PEM / OpenSSH private keys** — `-----BEGIN (RSA |EC |DSA |OPENSSH |ENCRYPTED )?PRIVATE KEY-----` (OpenSSH's own private-key PEM header, `-----BEGIN OPENSSH PRIVATE KEY-----`, is matched by the optional `OPENSSH ` group, so this single class covers both PEM-armored classic keys and OpenSSH's private-key format — task B lists these as one combined class).
2. **DB / connection URLs** — either a known DB URL scheme with embedded userinfo (`(postgres|postgresql|mysql|mongodb(\+srv)?|redis|amqp|sqlserver)://\S*:\S*@\S+`), or, generalized, **any** URL of the shape `://[^/\s]*:[^/\s]*@` regardless of scheme — this generalization is what makes the class also catch the masked-userinfo shape rev-0 only handled in prose for Git remote URLs (`<user>:<pass>@<host>`, task M) as a real, enforced scanner rule rather than a one-off comment.
3. **Emails / PII** — `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`.
4. **Credential assignments** — `(?i)\b(secret|token|password|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret)\b\s*[:=]\s*['"]?\S{8,}`.
5. **Bearer/token/key patterns** — reuses the existing `session_redaction.go` secret-like-string prefixes (`sk-`, `ghp_`, `gho_`, `ghu_`, `AKIA`, `xox[baprs]-`, and a generic `(?i)bearer\s+[A-Za-z0-9._-]{16,}`).
6. **Home absolute paths** — reuses the existing `session_redaction.go` absolute-home-path pattern.

### 8.3 What is scanned, and when

Every candidate string before it is committed to any file (tracked or
local): the resource's `selector`, every `args` value, every
`git-metadata` resolved value (a ref OID is not scannable content
itself, but a `config` view's resolved value is — e.g. a hypothetical
future config key holding a URL would be caught by class 2), every
`ignored-file`'s raw content (byte-scanned regardless of binary/text
classification, §5.1), and the local-only raw Dolt diagnostic blob
(§6.3) if `keep_local=true`. Scanning happens **before** any write —
staging computes the scan result first; a refusal means the batch
(§7.2) is never written past its temp-directory stage, and no tracked
file is touched.

### 8.4 Local raw bodies require explicit opt-in (task B)

Restated from §3/§7.1: raw bytes are written locally only when
`keep_local=true`. When they are written, the batch directory and its
contents are created with owner-only permissions (`0700` directories,
`0600` files) — "safe permissions" per task B — regardless of the
process's umask (explicitly set via `os.Chmod` after creation, not
relied upon from umask alone). Raw bodies remain untracked in every
case — `keep_local` never affects tracked output (§8.1).

## 9. Symlink & Path Safety (task D)

`safety.EnsureSafeRepoPath` and `store.NormalizeClaimPath` are
**lexical only** (C2) — `filepath.Abs` plus a string-prefix
containment check, no `Lstat`, no `filepath.EvalSymlinks`. This is
sufficient for their existing callers (claim paths that are validated
once against a manifest, not repeatedly re-read from a potentially
attacker-influenced filesystem), but is **not** sufficient on its own
for resource capture, where a selector may point through a symlink an
attacker (or an innocently misconfigured worktree) controls, and where
the same selector is re-resolved at every `capture`, not just once.

Rev-1 requires, at **both `add` and every `capture`**, and for
**every** path this feature touches — the selector path itself, every
regular file discovered while walking a directory selector (§5.1),
the process `cwd` tpatch is invoked from, and the resolved adapter
executable path (§6.1) — the following gate, run in this order:

1. `os.Lstat` the path. If it does not exist: `path-missing` (exit 3).
2. If the `Lstat` result is itself a symlink, resolve with
   `filepath.EvalSymlinks`. If resolution fails (dangling target,
   permission error, or a symlink cycle): `symlink-unresolvable`
   (exit 3) — never silently skipped or treated as "just don't
   follow it."
3. Compute the resolved absolute path and re-run the existing
   `safety.EnsureSafeRepoPath` **containment** check against it (not
   just the original, pre-resolution path) — this is the new
   requirement layered *on top of* the existing lexical check, not a
   replacement for it. If the resolved path escapes the repository
   root: `symlink-escapes-repo` (exit 3).
4. If the resolved path's final path component, or any path component
   the resolution traversed through, is literally named `.git` (case-
   sensitive; matching `gitutil.pathIsGitInternal`'s existing `.git`
   boundary convention used for the diff/store-write exclusions in
   ADR-030 D3/D4): `symlink-targets-git-internal` (exit 3) — refused
   even if the resolved target is technically still inside the
   repository root, because `.git` internals are never a valid
   resource target regardless of containment.

This four-step gate is re-run **fully** at every `capture` (task D:
"at add and capture for every directory descendant") — a selector that
was a plain file at `add` time but has since been replaced by a
symlink (a classic TOCTOU-style concern for any tool that re-reads a
path by name) is caught at the next `capture`, not just accepted
because it passed once. For a directory selector, step 2's resolution
requirement means the walk itself does not follow symlinked
subdirectories (a symlinked subdirectory is treated as a leaf that
fails step 2/3/4 in isolation, refusing that one entry rather than
silently descending into a target outside the intended tree) — this
closes task D's "every directory descendant" requirement precisely:
each descendant file is independently Lstat'd and resolved, not just
the top-level selector.

| Case | Outcome |
|---|---|
| Selector resolves to a plain file, fully inside repo, not through `.git` | Accepted |
| Selector is a symlink to a plain file, resolved target fully inside repo, not through `.git` | Accepted — the symlink itself is fine; what's refused is where it points, not that it exists |
| Selector is a symlink to a path outside the repo root | Refused: `symlink-escapes-repo` |
| Selector is a symlink whose target does not exist | Refused: `symlink-unresolvable` |
| Selector (or a directory descendant) resolves through a `.git` path component | Refused: `symlink-targets-git-internal` |
| `cwd` itself (the directory `tpatch` was invoked from) is a symlink whose resolved target escapes the repo | Refused before any resource-specific work begins: `symlink-escapes-repo` |
| Resolved Dolt executable path (§6.1) lands inside the repository working tree | Refused: `adapter-executable-unsafe` |

## 10. `record --resources` Semantics (task J)

Rev-0 never wrote a literal bare `record --dry-run`; it wrote `record
--resources --dry-run`, overloading dry-run preview onto the same
command that performs the Git-side canonical-patch capture. Rev-1
removes `--dry-run` from `record` entirely and moves all dry-run and
resource-only retry flows to the standalone `feature resource capture`
verb (§3), which already supports `--dry-run` and an optional single
`--resource <id>` target. `record --resources` itself never accepts
`--dry-run` — it always either fully stages and (on Git success)
publishes, or does neither.

### 10.1 Ordering: stage first, publish only after Git success

`record <slug> --resources [other existing flags]` runs, in order:

1. **Zero-resource preflight** (task J): if the feature has zero
   declared resources, refuse immediately, before touching Git at all
   — `no-resources-declared` (exit 1, matching rev-0's original exit
   choice for this case, kept for consistency): "no resources declared
   for `<slug>`; declare one with `feature resource add` first, or
   omit `--resources`."
2. **Stage** (task J: "preflights/stages privately before existing Git
   capture"): run the full `capture` pipeline (§7.2 steps 1-3 only —
   write and commit a local batch under `.tmp-lb_<id>` → `lb_<id>`,
   including redaction §8) for every declared resource, but do **not**
   yet update `current` and do **not** yet write any tracked
   `summary.json`. If **any** resource's staging fails (adapter error,
   size limit, redaction refusal, symlink refusal), the whole staging
   step is marked failed (existing all-or-nothing policy, §7 D8
   precedent) — but this does **not** stop step 3 below.
3. **Git-side capture**: `record`'s existing capture-mode dispatch runs
   completely unaffected by step 2's outcome — this PRD does not
   change `record`'s existing capture-mode mutex group, empty-patch
   handling, or `--auto`/commit-range resolution (§2 non-goals).
4. **Publish, gated on Git success** (task J: "publishes pointer only
   after Git success"):
   - If Git-side capture **failed**: the record command's own,
     existing, unmodified failure/exit-code behavior propagates. The
     locally-staged batch from step 2 (whether it itself succeeded or
     failed) is **discarded** — `current` is never updated and no
     tracked summary is ever written. A diagnostic notes "resource
     capture was staged but not published because the canonical
     Git-side capture did not succeed."
   - If Git-side capture **succeeded** and step 2 staging **also
     succeeded**: publish — update `current` (§7.2 step 4) and write
     each resource's tracked `summary.json` from the already-staged
     batch content. This is expected to be fast (no adapter
     re-execution; the batch was already fully computed in step 2).
   - If Git-side capture **succeeded** but step 2 staging **failed**:
     a **partial-domain** result (task J) — see §10.2.
   - If Git-side capture **succeeded**, step 2 staging succeeded, but
     the publish step itself fails (disk/permission error writing the
     tracked files): also a **partial-domain** result — see §10.2.

### 10.2 Partial-domain error (task J)

Both failure shapes in step 4 above ("Git succeeded, resource domain
did not") produce the same class of result, `resource-domain-incomplete`
(exit 1 — record's overall command surfaces this as a failure, exit 1,
matching rev-0's precedent of a non-zero overall exit when the
resource-capture portion fails even though the primary Git-side action
succeeded):

> canonical patch recorded successfully (Git-side capture complete);
> resource capture did not complete: `<reason>`. Your computed
> resource batch is safely staged and was not lost — retry with
> `tpatch feature resource capture <slug>` (recomputes and publishes a
> fresh batch) or, if only the publish step failed and you want to
> avoid redoing adapter work, `tpatch feature resource capture <slug>
> --resource <id>` for just the resources that need attention.

This is deliberately honest that **Git capture and resource-pointer
publication are two separate atomic domains** (task J's exact framing)
— `record` guarantees the canonical patch is captured correctly
regardless of resource-domain outcome, and separately guarantees that
if the resource domain does complete, it does so atomically (§7.2);
it does **not** guarantee both domains complete together, and never
claims to.

### 10.3 Interactions

- **Empty Git-side capture**: whether `record`'s existing capture-mode
  dispatch accepts or refuses a capture-mode selection that produces
  zero changes is existing, unmodified behavior (§2 non-goals) — this
  PRD does not alter it. If an empty Git-side capture is accepted by
  existing logic, it is treated as Git-side *success* for the purpose
  of gating publish in §10.1 step 4 — an empty canonical patch is
  still a completed capture.
- **`--auto` / commit-range flags**: `--resources` composes with every
  existing capture-mode flag exactly the same way regardless of which
  mode is selected — it has no special-cased interaction with any of
  them; it only observes whether step 3 (whichever mode was chosen)
  succeeded or failed.
- **Retry / idempotency**: re-running `feature resource capture <slug>`
  (standalone, not through `record`) after any failure always recomputes
  a fresh local batch and publishes directly (no Git dependency at
  all in the standalone verb — it stages, then immediately publishes,
  skipping §10.1's Git-gating entirely, since it is not `record`). The
  **tracked** `summary.json` content is deterministic given unchanged
  underlying state (no timestamps or IDs vary in the compared fields
  used for `diff`'s output — `captured_at` and `local_batch_id` do
  change every run, which is expected and documented, not a
  correctness bug). The **local** batch history grows by one batch per
  invocation regardless of whether anything changed (§7.4, accepted
  cost).

### 10.4 Resource-only scope matches validation (task N)

`record --resources`'s promised scope — "capture every resource
declared for this feature" — matches exactly what step 2 validates:
every entry in `resources.json` for the slug, no subset selection (the
standalone `feature resource capture <slug> --resource <id>` verb is
the only way to target a single resource; `record --resources` has no
`--resource` flag of its own, precisely because it must stage the
complete declared set before it can decide whether to publish at all).

### 10.5 Exit codes (new per-command contract, task M correction)

Per `SPEC.md`'s existing, explicit convention ("Exit codes are
per-command contracts, not a single global enum" — `SPEC.md`
§"Exit-code envelope"), this is one more independent contract in that
family, not a redefinition of any existing command's codes, and not
"reusing verify's" codes (C5 corrects rev-0's false claim that `verify`
was the sole existing bespoke user of 2/3 — six commands already use
this pattern).

| Code | `feature resource {add,list,remove,clear,capture,diff}` | `record --resources` |
|---|---|---|
| `0` | Success | Success (including a `diff` with nothing captured yet) |
| `1` | Unexpected internal error (I/O, store load failure) | Same, **plus** `no-resources-declared` preflight and `resource-domain-incomplete` partial-domain result (§10.2) |
| `2` | Validation: bad kind/adapter/capability/view, unknown `--arg` key, duplicate `--arg` key, malformed selector, unresolvable index-entry stage | n/a (record's existing validation codes are unmodified) |
| `3` | State/policy refusal: not-ignored, tracked-file gate failure, disallowed config key, any symlink refusal (§9), any size/count limit exceeded, `redaction-refused`, `adapter-missing`/`adapter-probe-*`, `local-path-not-ignored`, `capture-in-progress` | Same refusal set applies to the staging step (§10.1 step 2); a staging-step refusal on any single resource fails the whole staged batch, surfacing as `resource-domain-incomplete` (exit 1) if Git succeeded, or as a discarded-batch diagnostic (record's own existing exit code) if Git failed |

## 11. Wire Schemas (task K)

Two distinct JSON serializations exist in this design and must not be
conflated:

- **Canonical args JSON** (§12) — used **only** as hash input when
  deriving `resource_id`. Sorted keys, minimal escaping, no
  whitespace, custom encoder (not `encoding/json`).
- **File wire format** (this section) — the actual bytes written to
  `resources.json` and `resources/<id>/summary.json`. Ordinary Go
  `encoding/json` on a fixed-field struct (matching the ADR-032
  Implementation Notes item 8 precedent: `json.Marshal` on a
  declared-order struct, not a map, so key order is the Go struct's
  field declaration order, not alphabetical), 2-space indent, trailing
  newline. Arrays that are conceptually "empty" are always `[]`, never
  `null`, and the key is never omitted. Fields that do not apply to a
  given `kind` are present with an explicit zero value (`""` for
  inapplicable strings, `{}` for inapplicable `args`, `null` for
  inapplicable `tool_identity`) — never omitted.

### 11.1 `resources.json` (declaration manifest)

```json
{
  "resources": [
    {
      "resource_id": "res_19b4675405e2",
      "kind": "adapter-snapshot",
      "selector": "dolt:schema-diff:users",
      "adapter": "dolt",
      "capability": "schema-diff",
      "args": {
        "table": "users",
        "from": "main",
        "to": "HEAD"
      },
      "keep_local": false,
      "added_at": "2026-08-10T00:00:00Z",
      "added_by_tool_version": "tpatch/0.13.0"
    },
    {
      "resource_id": "res_79f5ac5dca13",
      "kind": "ignored-file",
      "selector": "config/local-secrets.env.template",
      "adapter": "",
      "capability": "",
      "args": {},
      "keep_local": true,
      "added_at": "2026-08-10T00:01:00Z",
      "added_by_tool_version": "tpatch/0.13.0"
    }
  ]
}
```

### 11.2 `resources/<id>/summary.json` — one example per kind

**`adapter-snapshot`** (added/removed/changed table names, always
present arrays, lexicographically sorted):

```json
{
  "resource_id": "res_19b4675405e2",
  "kind": "adapter-snapshot",
  "selector": "dolt:schema-diff:users",
  "adapter": "dolt",
  "capability": "schema-diff",
  "args": {
    "table": "users",
    "from": "main",
    "to": "HEAD"
  },
  "captured_at": "2026-08-10T00:05:00Z",
  "tool_identity": {
    "executable_path": "/usr/local/bin/dolt",
    "version": "1.42.3"
  },
  "result": {
    "added": [],
    "removed": [],
    "changed": ["users"]
  },
  "raw": {
    "present": false,
    "hash": null,
    "bytes": null
  },
  "local_batch_id": null
}
```

**`git-metadata`** (`head` view; no diffing, `result` is a
point-in-time resolved value, not an added/removed/changed set):

```json
{
  "resource_id": "res_acc91dc23a8b",
  "kind": "git-metadata",
  "selector": "head",
  "adapter": "",
  "capability": "",
  "args": {},
  "captured_at": "2026-08-10T00:05:00Z",
  "tool_identity": null,
  "result": {
    "symbolic_ref": "refs/heads/main",
    "oid": "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
    "detached": false
  },
  "raw": {
    "present": false,
    "hash": null,
    "bytes": null
  },
  "local_batch_id": null
}
```

**`ignored-file`** (single-file selector; directory selectors use the
aggregate shape shown in §5.1 with `file_count`/`total_bytes`/
`combined_hash` in place of `size_bytes`/`hash`):

```json
{
  "resource_id": "res_79f5ac5dca13",
  "kind": "ignored-file",
  "selector": "config/local-secrets.env.template",
  "adapter": "",
  "capability": "",
  "args": {},
  "captured_at": "2026-08-10T00:06:00Z",
  "tool_identity": null,
  "result": {
    "file_kind": "text",
    "size_bytes": 214,
    "hash": "sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60"
  },
  "raw": {
    "present": true,
    "hash": "sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60",
    "bytes": 214
  },
  "local_batch_id": "lb_1a2b3c4d5e6f"
}
```

Note `result.hash` and `raw.hash` are the **same** value here — both
are computed over the same verbatim bytes (§5.1); `result.hash` is
the tracked structural fact, `raw` additionally confirms a local copy
exists and reports its own (here identical) hash and size, matching
§4.1's requirement that `raw` fields are independent of `result` and
must be re-derivable from the local batch, not inferred.

### 11.3 Local batch `meta.json` (diagnostics, local-only, informal shape)

Not a stable, versioned wire contract (it is never read by any tracked
process — only a human debugging a failed capture reads it), so this
PRD only illustrates its shape rather than mandating an exact schema:

```json
{
  "batch_id": "lb_1a2b3c4d5e6f",
  "slug": "model-picker",
  "started_at": "2026-08-10T00:05:58Z",
  "outcome": "staged",
  "resources": [
    { "resource_id": "res_79f5ac5dca13", "outcome": "ok" }
  ]
}
```

### 11.4 `raw` field rules (task K)

`raw.present` is `true` iff `keep_local` was `true` for this capture
**and** a local raw companion was actually written (never true for a
`--dry-run` capture, §3). When `false`, `raw.hash` and `raw.bytes` are
`null` (not omitted). When `true`, `raw.hash` is always populated
(computed over the same bytes written locally) and `raw.bytes` is the
exact byte count of that local content (for a directory `ignored-file`
selector, the aggregate across all files, matching `result.total_bytes`).

### 11.5 Normalization before hashing (structural kinds only)

`git-metadata` and `adapter-snapshot` values are normalized before
hashing/recording (trailing-newline and CRLF/LF normalization) so the
same underlying state produces the same recorded value across
operating systems. `ignored-file` raw bytes are **never** normalized
(§5.1) — the tracked hash for that kind is computed over the exact
bytes read, with no transform.

### 11.6 The `current` pointer (local, not tracked)

`.tpatch/local/resource-capture/<slug>/current` is a local file (§7.1),
never a tracked artifact. Its content is exactly the batch ID string
(`lb_` followed by 12 lowercase hex characters) followed by one `\n`,
nothing else — no JSON, no additional metadata, to keep the atomic
rename-based update (§7.2 step 4) as small and simple as possible.

## 12. Resource ID Canonicalization (task H)

Rev-0's `key=value\n`-joined `args` encoding was ambiguous (no escaping
of `=`/newline inside a key or value, so `{"a=b":"c"}` and
`{"a":"b=c"}` were not distinguishable from each other, nor was a value
containing a literal newline distinguishable from a second argument).
Rev-1 replaces it with a dedicated canonical-JSON encoding used **only**
as ID-derivation hash input (§11 draws the explicit line between this
and the ordinary file wire format).

### 12.1 Canonical `args` encoding

1. Keys are sorted by byte value (ascending, Go's default `<` on
   `string`, equivalent to `sort.Strings`).
2. Encoding is `{"k1":"v1","k2":"v2",...}` — no whitespace anywhere.
3. Only two characters are escaped, in keys and values alike:
   backslash (`\` → `\\`) and double-quote (`"` → `\"`). No other
   escaping (deliberately **not** using `encoding/json.Marshal`, which
   by default also HTML-escapes `<`, `>`, and `&` — an implicit,
   easy-to-forget behavior this PRD avoids by using a small,
   fully-specified custom encoder instead).
4. An empty `args` map encodes as the literal two-character string
   `{}`.
5. Input is required to be valid UTF-8; **no NFC/NFD Unicode
   normalization is performed** (a documented v1 limitation — two
   selectors that are visually identical but differ in Unicode
   normalization form will produce different IDs; adding a
   normalization dependency is deferred, not silently assumed).
6. Any NUL byte (`0x00`) or other C0 control character (`0x01`–`0x1F`,
   `0x7F`) anywhere in `feature`, `kind`, `selector`, `adapter`,
   `capability`, or any `args` key/value is a **validation error**
   (exit 2) at `add` time — rejected before it can ever reach the
   encoder, not escaped or transformed.

### 12.2 Full ID derivation

```
canonical_args := CanonicalArgsJSON(args)          // §12.1
payload        := feature + "\x00" + kind + "\x00" + selector +
                   "\x00" + adapter + "\x00" + capability +
                   "\x00" + canonical_args
digest          := SHA-256(UTF-8 bytes of payload)
resource_id     := "res_" + lowercase-hex(digest)[:12]
```

`adapter` and `capability` are empty strings (not omitted from the
payload) for kinds that don't use them (`git-metadata`,
`ignored-file`) — the `\x00`-joined payload always has exactly six
components in this fixed order, regardless of kind.

### 12.3 Golden vectors (reproduced from §0.3, byte-identical to `ADR-033-resource-capture-boundary.md` D3)

Computed with Python `hashlib.sha256` over the exact payload
construction in §12.2; independently reproducible by any SHA-256
implementation given the same inputs.

| Vector | Inputs (`feature`, `kind`, `selector`, `adapter`, `capability`, `args`) | `resource_id` |
|---|---|---|
| 1 | `model-picker`, `git-metadata`, `head`, ``, ``, `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker`, `adapter-snapshot`, `dolt:schema-diff:users`, `dolt`, `schema-diff`, `{"table":"users","from":"main","to":"HEAD"}` (declared in `table, from, to` order) | `res_19b4675405e2` |
| 3 | Same as Vector 2, `args` declared in `to, table, from` order | `res_19b4675405e2` (**identical** — proves canonical encoding is order-independent because keys are sorted before joining, §12.1 step 1) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

All four selectors above are valid under the closed selector grammars
defined in §5: `head` is one of §5.2's four `git-metadata` views;
`dolt:schema-diff:users` matches §5.3's `dolt:<capability>:<table>`
shape; `config/local-secrets.env.template` is an ordinary
repo-relative path, valid for §5.1's `ignored-file` kind.

## 13. Acceptance Criteria (task L)

Clause-level, `AC-<n>` tagged. Each AC is a single testable clause;
rows in the ADR's Test Matrix (§13 of `ADR-033-resource-capture-boundary.md`)
cite these tags directly rather than re-deriving criteria — this PRD
is the source of truth for *what* must be true, the ADR's matrix is
the source of truth for *how each is verified*.

**Command surface & output (human + JSON, task L)**

- `AC-1`: `feature resource add/list/remove/clear` exist under the
  `feature` noun and refuse a nonexistent slug with `"no such feature:
  %s"` (C6 shape), for both human and `--json` output.
- `AC-2`: `feature resource list --json` and human output are both
  exercised and agree on content (JSON is not a strict superset with
  extra debug fields, nor a subset missing fields the human view
  shows).
- `AC-3`: `feature resource remove`/`capture`/`diff` resolve a full
  `resource_id`, an unambiguous prefix, and refuse an ambiguous prefix
  (exit 2, listing the matching candidates) or a prefix matching zero
  resources (exit 2).
- `AC-4`: A selector or `--arg` value containing shell metacharacters
  (`;`, `|`, `` ` ``, `$(...)`, `&&`) is accepted as an ordinary string
  (argv-only invocation, §6, means no shell is ever invoked to
  interpret it) and is subject to the same redaction scan (§8) as any
  other value — it is not specially rejected for containing
  metacharacters, only for matching a closed redaction class or
  failing shape validation.

**Dolt adapter (task L)**

- `AC-5`: `schema-diff` and `table-diff` capabilities each run the
  exact three-invocation `--name-only --filter=` sequence (§6.2) and
  classify a table as `added`/`removed`/`changed` correctly for each
  case.
- `AC-6`: A missing `dolt` binary produces `adapter-missing` (exit 3),
  not a generic internal error.
- `AC-7`: The `dolt version` probe's four outcomes (§6.1 table) each
  produce their named, distinct error class.
- `AC-8`: A duplicate `--arg table=...` (same key twice) at `add` is a
  validation error (exit 2); an unknown `--arg` key is a validation
  error (exit 2); a missing required arg (`table`, `from`, or `to`) is
  a validation error (exit 2).
- `AC-9`: The resolved `dolt` executable path, when it lands inside
  the repository working tree, is refused (`adapter-executable-unsafe`,
  exit 3) before the probe runs.

**Capture / list / remove / clear / diff (task L)**

- `AC-10`: `capture` (no `--resource`) captures every declared
  resource for the slug; `capture --resource <id>` captures only that
  one.
- `AC-11`: `capture --dry-run` performs the full pipeline including
  redaction (§8) but writes nothing — not the local tree, not any
  tracked file.
- `AC-12`: `diff` never executes an adapter, never touches the local
  tree, and reports "no capture yet" (exit 0) for a resource with no
  prior tracked summary.
- `AC-13`: `remove`/`clear` delete only the tracked declaration and
  tracked summary; the local batch history for that resource is left
  untouched (§7.4) — verified by asserting the local tree is byte-for-byte
  unchanged before/after a `remove`.

**Privacy classes (task L)**

- `AC-14`: Each of the six closed redaction classes (§8.2), given a
  crafted matching input in a selector/`args`/git-metadata-`config`-value/
  `ignored-file` body, produces `redaction-refused` (exit 3) and the
  captured artifact set (tracked and local) is unchanged by the
  attempt (no partial write).
- `AC-15`: A value that does **not** match any of the six classes
  captures normally (negative-control case, to prove the scanner is
  not overly broad in a way that would make the whole feature
  unusable).

**Symlinks (task L)**

- `AC-16` – `AC-21`: each row of §9's outcome table (six rows) is an
  individually testable AC — a plain in-repo file, a symlink to an
  in-repo file, a symlink escaping the repo root, a dangling symlink,
  a symlink/descendant resolving through `.git`, and an escaping `cwd`
  — each produces exactly the outcome named in that table's second
  column.

**Ignored + tracked dual-gate, limits (task L)**

- `AC-22`: An ignored-and-untracked file is accepted.
- `AC-23`: A tracked file that also matches a `.gitignore` pattern is
  refused (the `--no-index` gap, C3/§5.1) at both `add` and `capture`.
- `AC-24`: An untracked-but-not-ignored file is refused (rev-0's
  removed acceptance case, §5.1).
- `AC-25` – `AC-27`: each of the three directory-selector limits
  (per-file 5 MiB, total 20 MiB, 200 files) independently triggers a
  refusal (exit 3) when exceeded, re-checked at `capture` time even if
  the selector passed at `add` time (task N: snapshot-time bounds).

**First capture, binary, multi-file (task L)**

- `AC-28`: The first-ever `capture` for a resource and a subsequent
  `capture` produce byte-identical `summary.json` schema shape (§11.2
  note on "first snapshot" having no special schema).
- `AC-29`: A binary `ignored-file` (NUL byte within the first 8 KiB) is
  classified `binary` in the tracked summary and is still subject to
  the full redaction scan (§8.3) despite not being valid UTF-8 text.
- `AC-30`: A directory `ignored-file` selector with `keep_local=true`
  produces a local `manifest.json` listing every matched file's
  relative path, size, and hash, and the tracked summary's aggregate
  fields (`file_count`, `total_bytes`, `combined_hash`) are consistent
  with that local manifest.

**Crash/recovery/concurrency (task L)**

- `AC-31`: An orphaned `.tmp-lb_*` directory left behind by a
  simulated crash between §7.2 steps 2 and 3 does not affect the
  outcome of a subsequent `capture` (it produces a fresh batch and
  succeeds normally; the orphan is left in place, not auto-deleted, in
  v1).
- `AC-32`: A fully-committed `lb_<id>/` batch that a simulated crash
  left un-pointed-to by `current` (crash between §7.2 steps 3 and 4)
  is never surfaced by `list`/`diff`, and a subsequent `capture`
  succeeds and correctly updates `current` to its own new batch.
- `AC-33`: Two concurrent `capture` invocations for the same slug: the
  second one to acquire the lock file refuses immediately with
  `capture-in-progress` (exit 3), it does not block/wait.

**Partial-domain `record --resources` (task L)**

- `AC-34`: `record --resources` on a feature with zero declared
  resources refuses (`no-resources-declared`, exit 1) before any Git
  invocation.
- `AC-35`: A resource-staging failure (§10.1 step 2) combined with
  Git-side capture success produces `resource-domain-incomplete` (exit
  1) with the exact recovery-command message (§10.2), while the Git-side
  canonical patch is confirmed present and correct.
- `AC-36`: A resource-staging failure combined with Git-side capture
  failure discards the staged batch and surfaces only `record`'s
  existing, unmodified Git-failure behavior (no `resource-domain-incomplete`
  double-reporting).
- `AC-37`: A successful stage and successful Git-side capture publish
  the resource batch atomically (§7.2 step 4), verified by asserting
  `current` and every declared resource's tracked `summary.json` are
  updated together, not partially.

**`--dry-run` / resource-only / retry (task L)**

- `AC-38`: `feature resource capture <slug> --dry-run` on a slug whose
  Git-side canonical patch has not been touched at all still runs and
  reports correctly (resource capture is fully independent of any
  Git-side action, confirming the two-domain separation, §10.2).
- `AC-39`: Re-running `feature resource capture <slug>` after a prior
  failure succeeds without requiring any dedicated recovery command
  (§7.3).

**Golden IDs (task L)**

- `AC-40`: Each of the four golden vectors in §12.3 is independently
  recomputed by the implementation and matches exactly.
- `AC-41`: Vectors 2 and 3 (identical content, differently-ordered
  `args`) produce the identical `resource_id` (order-independence
  property, not just a fixed example).

### 13.1 Exact counts (task L: no false "exactly once" claims)

This PRD defines **41** `AC`-tagged clauses (`AC-1` through `AC-41`,
with `AC-16`–`AC-21` explicitly expanding to six individually-testable
sub-clauses inside one tag range, and `AC-25`–`AC-27` expanding to
three). Counting each lettered/numbered sub-clause individually (i.e.
treating `AC-16` through `AC-21` as six distinct clauses and `AC-25`
through `AC-27` as three distinct clauses, rather than one clause
each) yields **41 − 6 − 3 + 6 + 3 = 41** — the range notation already
counts each sub-clause once, so the total distinct testable clauses is
exactly **41**. The companion ADR's Test Matrix (§13 there) maps each
of these 41 clauses to at least one row; several clauses (the
symlink-outcome and limit sub-clauses in particular) map to more than
one matrix row where both a human-output and `--json`-output
verification are both required. The matrix therefore has **more** rows
than 41 — the exact matrix row count is computed and stated in the
ADR itself (§13 there), not asserted here, and this PRD does not claim
the mapping is one-row-per-clause ("exactly once") anywhere.

## 14. Open Questions / Negative Consequences

**Open questions** (unresolved, out of scope for rev-1, left for a
future PRD/ADR if pursued): (1) whether a future generic
sandboxed/consented external-command adapter should share this PRD's
`adapter-snapshot` kind or need its own kind; (2) whether local batch
history should eventually get a `purge-local` verb or size-based
auto-pruning; (3) whether Unicode NFC normalization should be added to
the canonical `args` encoding (§12.1 step 5) once/if a normalization
dependency becomes acceptable elsewhere in the codebase.

**Negative consequences** (explicitly accepted costs):

1. Local batch history grows unboundedly until manually pruned — no
   `tpatch` command deletes old batches in v1 (§7.4).
2. `record --resources` does strictly more work than rev-0's
   Git-first ordering in the common case (staging always runs before
   Git, even though most of the time Git will succeed and the staged
   result will simply be published) — a deliberate trade for the
   stronger no-tracked-inconsistency guarantee (§10.1).
3. No NFC Unicode normalization (§12.1 step 5) means visually-identical
   selectors in different normalization forms get different resource
   IDs — an accepted v1 limitation, not a silent bug.
4. The `-r json`/richer Dolt output is never parsed into tracked
   fields (§6.3) — a user wanting per-row Dolt diff detail must inspect
   the local raw blob by hand; this PRD does not offer a structured
   API for it.

## 15. Rev-1 Changelog (vs. rev-0, `dd08157`)

- Removed `generic-command` kind/adapter entirely; closed adapter set
  is Dolt only (task C).
- Moved all raw snapshot/diff/file bytes to a new gitignored local
  tree, `.tpatch/local/resource-capture/<slug>/`; tracked sidecars are
  hash/count/classification-only (task A).
- Replaced rev-0's ambiguous `key=value\n` `args` encoding with a
  sorted, minimally-escaped canonical-JSON encoding for `resource_id`
  derivation, with four independently-verified golden vectors
  including a reordered-key equivalence proof (task H).
- Narrowed the `git-metadata` kind from an unrestricted `config` view
  (`user.name`/`user.email`/wildcards) to four closed views with
  exactly four safe config keys (task F).
- Added a mandatory Lstat + `EvalSymlinks` + containment + `.git`-target
  refusal gate, re-run at both `add` and every `capture`, for every
  path this feature touches (task D).
- Added a mandatory ignored-**and**-untracked dual gate (closing the
  `--no-index` gap), removed the "untracked but not ignored" acceptance
  case, and added exact per-file/total/file-count directory limits
  (task E).
- Replaced the fabricated `dolt diff --json` invocation and invented
  per-row schema with a verified three-invocation `--name-only
  --filter=` design for tracked structural output, treating richer
  output as an opaque local-only blob (task G).
- Replaced numbered append-only tracked history with immutable local
  batches plus a single atomic `current` pointer, removed the
  cross-tree atomic-delete claim from `remove`/`clear`, and defined
  the full crash-window/recovery analysis (task I).
- Reordered `record --resources` to stage privately before Git-side
  capture and publish the resource pointer only after Git succeeds;
  removed `record --dry-run` entirely in favor of `feature resource
  capture --dry-run`/`diff` (task J).
- Wrote out complete local and tracked wire schemas with one example
  per kind, kept byte-identical between this PRD and the companion ADR
  (task K).
- Rebuilt the acceptance-criteria list at clause granularity (41
  `AC`-tagged clauses) and require the companion ADR's Test Matrix to
  compute its own row count rather than assume a 1:1 mapping (task L).
- Corrected six false/fabricated citations (fabricated `ADR-031 D10`,
  false "`EnsureSafeRepoPath` is symlink-aware," false "`verify` is
  `ExitCodeError`'s sole user," normative dependence on untracked
  WP-006, and others — full list in §0.1) (task M).
- Added the tracked-file gate alongside `IsPathIgnored`, aligned
  resource-only promised scope with actual validation, required
  snapshot-time directory bounds (not just declaration-time), made
  probe-failure semantics an explicit four-outcome table, and removed
  the batch-failure-envelope contradiction (task N).
