# PRD — Feature Resource Claims & Capture Adapters (rev-3)

**Status**: Draft — rev-3 (supersedes rev-2, writer commits `c603b8f`/
`4255bef`, adjudicated NEEDS REVISION → REV-3 DISPATCHED at `4ea011e`;
see `docs/supervisor/LOG.md` → Cluster H rev-0, rev-1, and rev-2
adjudications)

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

## 0. Rev-3 Fold Summary (read this first)

Rev-2 (`c603b8f`/`4255bef`) redesigned rev-1's Dolt argv, ADR-027
compliance, publication, path-safety, and Git-gate mechanisms, but the
rev-2 adjudication (`4ea011e`, internal 5 HIGH + 5 MEDIUM, external 5
HIGH + 7 MEDIUM + tracking notes; verified clean: 4 golden IDs, 3
shared JSON blocks, 48 AC clauses across 74 contiguous rows) found
rev-2's *execution contracts* still unsafe or under-specified in ten
concrete places: `git --literal-pathspecs check-ignore` is not a valid
invocation at all (§10.1 fix, this fold's C17/C18); ignored-file and
Dolt bytes were still written to an ephemeral **scratch file** before
the privacy scan ran, which is a write-before-scan ordering violation
of ADR-027's "redaction is a write precondition" even though the file
was later deleted (§7.1/§8, task 2); the lock was created via
`O_EXCL` and only *then* had its owner body written, leaving a window
where a contender could observe a lock file that existed but had no
readable owner yet (§7.2 rewrite, task 3); the post-open identity
check re-`Lstat`'d a *pathname* instead of `fstat`ing the already-open
descriptor, missing the point of opening before checking (§9.1 fix,
task 4); `add`/`remove`/`clear` mutated `resources.json`/`current.json`
outside the capture lock, racing a concurrent `capture` (§7.2/§7.6,
task 5); the Dolt design still had no required database path/cwd, no
mandatory `table` argument (so a primary-key-set change could be
silently dropped), and guessed at the `dolt sql -r json` output shape
rather than citing it (§6 full rewrite, task 6/7/8); tracked
publication paths/names, batch-ID generation, and cleanup semantics
still had internal contradictions (§7.3 rewrite, task 9/10); `--dry-run`,
local-ignore coverage, and several wire variants remained incomplete
(task 11/12/14); and `docs/handoff/CURRENT.md`'s counts and wording
had drifted after rev-2's own addendum (task 13). Rev-3 is again a
substantial, targeted rewrite of the affected mechanisms — §6, §7, §9,
§10 are rewritten in full; §5, §8, §11, §12, §13, §14 are revised in
place. **Preserved** because every review across all four revisions has
agreed it is sound:

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

Everything below this point is the standalone rev-3 design; §0.1–§0.4
map each rev-2 finding to its resolution. Rev-2's own `C1`–`C16` remain
correct and are not repeated (see `docs/handoff/HISTORY.md`/git history
for that table); rev-3 adds `C17`–`C24`, all grounded in a direct
source read of the pinned Dolt commit performed for this fold (not
merely the supervisor-relayed facts already folded in rev-2's
addendum) plus an empirical `git check-ignore` behavioral check.

### 0.1 Claims Audit (rev-3 additions)

Rev-1's `C1`–`C10` and rev-2's `C11`–`C16` (citation corrections for
`featureCmd`, the lexical-only safety helpers, `--no-index` ignore
semantics, the existing session-redaction shape, `ExitCodeError` call
sites, the `feature claim` CLI precedent, ADR-027 D1, tracked-vs-untracked
research docs, real Dolt CLI flags, `RemoveClaim`'s line range,
`EnsureLocalIgnoreContract`'s exact scope, `O_NOFOLLOW`'s availability,
the `dolt_diff_summary` column schema/`IsReadOnly`/argument-form
detail, and ADR-027 D3's exact text) all remain correct and are not
repeated here — see the rev-1/rev-2 text preserved in
`docs/handoff/HISTORY.md`/git history for that table. Rev-3 adds:

| # | Claim | Citation | Why this changes rev-1 |
|---|-------|----------|-------------------------|
| C11 | `dolt diff --name-only` combined with `--schema`/`--data` and `--filter=` is **not** how the pinned Dolt source expresses per-table schema/data change classification; the source-verified read-only interface is the `dolt_diff_summary(from, to[, table])` table function, queried over `dolt sql -r json -q "..."`, returning exactly `{from_table_name, to_table_name, diff_type, data_change, schema_change}` per row | Supervisor source check against `dolthub/dolt` at commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9`: `go/cmd/dolt/commands/diff.go` (synopsis, `--schema`/`--data`, `--result-format`), `go/cmd/dolt/commands/sql.go` (`-q`/`--query`, `-r`/`--result-format json`), `go/cmd/dolt/commands/version.go` | rev-1 **error**: rev-1's three-invocation `--name-only --filter={added,dropped,modified}` design combined flags in a way the source does not support as a single coherent invocation, and could not detect renames or represent "both schema and data changed" for one table in one classification. §6 replaces this entirely with the one-query `dolt_diff_summary` design. |
| C12 | `dolt version` is a real subcommand that can, depending on build/config, perform a network update check and read/write files under the resolved `HOME`; it is not a safe no-side-effect probe | Rev-2 threat-modeling of running an arbitrary resolved executable literally named `dolt` with an unconstrained inherited environment, not a specific pinned-commit source citation (no claim here about `dolt version`'s exact internal behavior beyond "runs arbitrary code with inherited env/network access," which is true of any executed binary in v1's threat model) | rev-1 ran `dolt version` as a "probe" step with the invoking process's inherited environment. §6.1 removes this probe entirely; tool identity is now a static file fact (executable basename + `SHA-256` of the resolved binary's bytes), never a code-execution result, and every actual invocation runs with a minimal, non-inherited scratch environment (§6.4). |
| C13 | `internal/workflow/session_ignore.go`'s `EnsureLocalIgnoreContract(repoRoot, resolvedPath)` verifies the path is inside the worktree and that `gitutil.IsPathIgnored` (`--no-index`) reports it ignored; it does **not** independently verify the path is untracked | `internal/workflow/session_ignore.go:138-175` (`EnsureLocalIgnoreContract` body) | New for rev-2: §10.3 reuses this exact function for the ephemeral-scratch root (task 7's "do not invent a second ignore mechanism") but layers the same tracked-file gate used for `ignored-file` selectors (§5.1) on top, since `EnsureLocalIgnoreContract` alone does not close the `--no-index` gap for the scratch root either. |
| C14 | Go's `os.OpenFile` accepts `syscall.O_NOFOLLOW` on Unix build targets (`darwin`/`linux`), which causes the open to fail with `ELOOP` if the **final** path component is a symlink; there is no portable stdlib/syscall equivalent that also binds every **ancestor** directory component against races (no `openat2`/`RESOLVE_NO_SYMLINKS` wrapper in the Go standard library) | Go standard library `os`/`syscall` package documentation (`O_NOFOLLOW` is a documented, platform-gated `syscall` constant; `openat2` has no stdlib wrapper as of the Go versions this project targets) | New for rev-2: §9.1 uses `O_NOFOLLOW` as one real, available hardening measure for the final component and is explicit that ancestor-component TOCTOU is closed by *refusing any symlink component at all* (a stat-time check) rather than by any stronger descriptor-bound guarantee stdlib cannot provide (task 5: "state TOCTOU residual honestly ... do not claim impossible sandbox"). |
| C15 | `dolt_diff_summary`'s five columns are typed and **non-null**: `from_table_name` (`LongText`), `to_table_name` (`LongText`), `diff_type` (`Text`), `data_change` (`Boolean`), `schema_change` (`Boolean`); the function itself reports `IsReadOnly() == true`; accepted invocation forms are the 2-arg `(from, to)` and 3-arg `(from, to, table)` shapes this PRD already uses, plus dot-range forms (e.g. a single `"from..to"`-shaped argument) this PRD deliberately does not use; Dolt's own internal Go usage of the function queries it with `select * from dolt_diff_summary(?, ?)` and sorts results by `ToName` in application code, rather than an explicit `SELECT <columns> ... ORDER BY` at the SQL layer | Supervisor source check against `dolthub/dolt` at commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9` (table-function schema/column typing, `IsReadOnly()`, accepted argument forms, and Dolt's own internal query/sort usage) | Confirms — does not correct — §6.2's design: the non-null column guarantee is why `result.tables[]` entries in every tracked wire example (§12.2/§12.3) never carry a null field; the read-only confirmation reinforces C11's "external, read-only tool" framing; this PRD deliberately does **not** adopt Dolt's own internal `select *` + application-side sort pattern, and instead binds every column by explicit name and applies an explicit SQL `ORDER BY from_table_name, to_table_name` (§6.2), so tracked output does not silently reorder or gain/lose a field if a future Dolt version changes the table function's positional column order; dot-range argument forms are noted as existing but out of scope for v1's exact 2-/3-arg argv template. |
| C16 | `ADR-027-capture-context-privacy-boundary.md` D3 states verbatim: "Local private buffers may keep only the redacted or hashed form; this ADR does not authorize a tpatch-managed raw transcript archive." | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-170` (D3 section), exact quoted sentence at `:168-170` | Directly grounds §7.1/§0.2's "no persistent raw bodies anywhere, ephemeral-scratch-only" design in ADR-027's own binding language, not just this PRD's inference from D1–D6's committed/local split "in spirit" (as rev-2's original §0 fold summary put it) — D3 is explicit and unconditional: a persistent raw local archive of any kind, opt-in or not, is not authorized without an ADR that supersedes it (§2's new non-goal). |
| C17 | `git check-ignore` does not accept a pathspec at all — its positional arguments are plain `<pathname>` values (per `git-check-ignore(1)`'s synopsis and option list, which has no `--literal-pathspecs`/pathspec-magic-related option); `git --literal-pathspecs check-ignore -q --no-index -- <path>` is therefore not a valid invocation and fails immediately with `fatal: <path>: pathspec magic not supported by this command: 'literal'` (exit `128`), never reaching the ignore check at all | Empirically verified against installed Git (`git --literal-pathspecs check-ignore -q --no-index -- 'a/:weird.txt'` → `fatal: a/:weird.txt: pathspec magic not supported by this command: 'literal'`, exit 128) and `git-check-ignore(1)`'s documented option list | rev-2 **error**: §10.1/§5.1 required `--literal-pathspecs` on the `check-ignore` invocation; every `ignored-file` `add`/`capture` would have failed with a fatal Git error before ever checking ignore status. §10.1/§5.1 are rewritten to reuse the **existing**, already-correct `gitutil.IsPathIgnored` invocation shape (`git check-ignore -q --no-index -- <pathname>`, no `--literal-pathspecs`) unchanged. |
| C18 | `check-ignore`'s plain pathname argument **does** parse a leading `:` for pathspec magic (unlike `*`/`?`/`[]`, which are inert literal characters to this command with no glob/fnmatch expansion): a colon-prefixed name using a magic keyword this command does not support (e.g. `:(literal)...`, `:!...`/`:^...` exclude) is a **fatal** error (exit `128`), while `:/...` ("top") magic is silently accepted without error; prefixing any selector beginning with `:` with `./` (e.g. `./:weird.txt`) disarms all colon-magic parsing (the argument no longer begins with a bare `:` byte) while still resolving to the identical file | Empirically verified: `git check-ignore --no-index -- ':(glob)sub/*.txt'` → fatal (exit 128); `git check-ignore --no-index -- ':!exclude.txt'` → fatal (exit 128); `git check-ignore --no-index -- ':/topmagic.txt'` → exit 0, no error; `git check-ignore --no-index -- './:(glob)sub/*.txt'` → exit 0, treated as the literal filename; `*`/`?`/`[]` in a plain (non-colon-prefixed) pathname never trigger wildcard matching (`git check-ignore --no-index -- 'sub2/file*.txt'` does not match a differently-named ignored file) | New for rev-3: §5.1/§10.1's `check-ignore` invocation now prefixes any selector whose first byte is `:` with `./` before passing it as the pathname argument, closing an ambiguity C17's fix would otherwise reintroduce for colon-shaped selectors specifically (the existing `ls-files --error-unmatch` gate already handles this safely via `--literal-pathspecs`, which `check-ignore` cannot accept). |
| C19 | `dolt_diff_summary`'s `from`/`to` arguments accept the literal strings `"WORKING"`/`"STAGED"` (exact case, not case-insensitively) — rev-1/rev-2 left this explicitly unconfirmed; it is now source-confirmed at the pinned commit | `go/libraries/doltcore/doltdb/doltdb.go:51-52` (`Working = "WORKING"`, `Staged = "STAGED"` constants); `go/libraries/doltcore/sqle/dsess/session.go:1022-1031` (`DoltSession.ResolveRootForRef` special-cases an exact match on either literal string before falling through to `doltdb.NewCommitSpec`); `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff.go:378-403` (`loadDetailsForRefs`/`resolveCommitStrings` route both `from` and `to` through `ResolveRootForRef`), commit `59fb843bf6a4b653d7c8b6d997a603b10cf279d9` | Resolves rev-2's §6.2/§15 open question. §6.2 now states `WORKING`/`STAGED` are accepted (exact uppercase) rather than deferring the question to the implementation cluster; this PRD does not change whether it *uses* them in its own examples (it still does not, to keep golden vectors and examples on concrete refs), only whether the capability accepts them if a caller declares them. |
| C20 | A hard hard-error outcome for a primary-key-set change between `from` and `to` on the requested table is source-confirmed, and is conditional on a `table` argument being supplied: `getSummaryForDelta`'s `shouldErrorOnPKChange` parameter is `true` only for the single-table query path (`tableNameExpr != nil`); the whole-database (no `table`) query path passes `false` and silently omits the affected table's row (with a warning, not an error) instead | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:299-321` (single-table call site, `shouldErrorOnPKChange=true`, line 311) vs `:324-341` (multi-table/whole-db loop, `shouldErrorOnPKChange=false`, line 334); `:346-365` (`getSummaryForDelta`'s branch); the wrapped sentinel is `diff.ErrPrimaryKeySetChanged` (`"primary key set changed"`, `go/libraries/doltcore/diff/diff_stat.go:31`), error text `"failed to compute diff summary for table %s: %w"` | Directly grounds task 6/task 8's "require `table` in v1 ... so PK-set changes fail rather than silently omit": this PRD's mandatory-`table` decision (§5.3/§6.2) is not merely a simplicity choice, it is the specific argument shape that routes a PK-set-change into Dolt's own hard-error path instead of its own silent-omission path. §6.2/§14 document the resulting `dolt-query-error` refusal class explicitly. |
| C21 | A `table` argument naming a table that exists in **neither** `from` nor `to` yields zero rows (not an error), a third, distinct outcome from C20's hard error and from a `dolt_ignore`-matched table's zero-row outcome | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:347-350` (`getSummaryForDelta`'s early `return nil, nil` when all of `FromTable`/`ToTable`/`FromRootObject`/`ToRootObject` are nil and neither name carries `diff.DBPrefix`) | Grounds §6.2's "table never existed" first-capture case and §14's `AC` for it with an exact source citation rather than an inferred "behavior not independently re-derived" hedge (rev-2's phrasing for this case). |
| C22 | `dolt sql -r json` wraps a nonempty result as `{"rows": [...]}` (a single top-level key, `"rows"`) and emits the literal, distinct 2-byte string `{}` for a zero-row result — there is **no** `"schema"` key in either case. Rev-2's claim of a `"schema"`-key-bearing envelope was community/docs-corroborated but not source-verified, and is now corrected | `go/libraries/doltcore/table/typed/json/writer.go:37-38` (`jsonHeader = `{"rows": [`` / `jsonFooter = `]}``), `:56-58`,`:62-64` (doc comments: "encodes rows as a single JSON object with a single key: \"rows\""); `go/cmd/dolt/commands/engine/sql_print.go:110-113` (`FormatJson` case constructs this writer), `:147-149` (the zero-row `{}` case is written directly by the caller — `iohelp.WriteLine(cli.CliOut, "{}")` — not by the row writer, precisely when `numRows == 0`) | rev-2 **error**: §6.3's parser assumed a `"schema"` key existed alongside `"rows"` and did not define the zero-row shape at all. §6.3/§6.2 are rewritten: the parser recognizes exactly two valid top-level shapes (`{"rows":[...]}` or `{}`), rejects any other top-level shape (missing/extra/renamed key) as a fatal parse error, and `{}` maps deterministically to an empty `tables: []` result. |
| C23 | `diff_type` has a closed, source-confirmed 4-value string enumeration — `"added"`, `"modified"`, `"renamed"`, `"dropped"` — contrary to rev-2's "not independently confirmed against a guessed closed set" hedge; for a `"dropped"` row `to_table_name` is the empty string `""` (not omitted, not `null`), and for an `"added"` row `from_table_name` is `""`, because `doltdb.TableName{}`'s zero value stringifies to `""` and `GetSummary` only populates the applicable side | `go/libraries/doltcore/diff/table_deltas.go:46-49` (`DiffTypeAdded`/`DiffTypeModified`/`DiffTypeRenamed`/`DiffTypeDropped` constants), `:716-733` (asymmetric `FromTableName`/`ToTableName` population for drop/add), `:735-745` (rename populates both, differing), `:747-760` (modify populates both, same name); `go/libraries/doltcore/doltdb/root_val.go:797-800` (`TableName.String()` zero-value behavior) | §6.2/§12.2 now document the closed 4-value set and the empty-string convention for add/drop rows precisely, while still tracking `diff_type` **verbatim** rather than validating against it (forward-compatible if a future Dolt version adds a 5th value) — a stricter, better-cited version of rev-2's existing "opaque string, not hardcoded" posture, not a reversal of it. |
| C24 | `dolt_diff_summary`'s own argument-count validation inspects the literal SQL-expression string of its **first** argument for a `".."` substring to choose between the dot-range (1–2 args) and explicit-`from`/`to` (2–3 args) parse branches; a `from` value that legitimately contains the literal substring `".."` breaks this design's explicit 3-argument (`from, to, table`) invocation at the SQL layer itself (misrouted argument-count validation, `sql.ErrInvalidArgumentNumber`), independent of and in addition to this design's own choice never to use dot-range syntax | `go/libraries/doltcore/sqle/dtablefunctions/dolt_diff_summary.go:220-238` (`WithExpressions`: `strings.Contains(exprs[0].String(), "..")` branches the accepted-argument-count check) | Upgrades task 6's "`from`/`to` reject `..`" from a defense-in-depth policy choice (rev-2 already refused backslash/control bytes similarly) to a real Dolt-compatibility requirement: refusing any value containing `".."` (§6.2) is not just prudent, it prevents a legitimate-looking value from silently breaking this design's fixed 3-argument invocation shape. |

### 0.2 What rev-3 removes or changes

- The invalid `git --literal-pathspecs check-ignore` invocation (§5.1,
  §10.1). Replaced by the existing, already-correct
  `gitutil.IsPathIgnored` shape (`git check-ignore -q --no-index --
  <pathname>`), with a `./`-prefix rule for colon-leading selectors
  (C18) to avoid `check-ignore`'s own colon-magic parsing without
  needing (or being able to use) `--literal-pathspecs`.
- Writing ignored-file/Dolt raw bytes to a scratch **file** before
  scanning them (§7.1, §8). Rev-3 reads ignored-file content and
  captures Dolt stdout/stderr into **bounded in-process memory
  buffers**, scans/hashes them there, and never writes an unredacted
  byte to any file, ephemeral or otherwise. Scratch-tree writes are
  now limited to control data (the Dolt scratch `HOME`/`DOLT_ROOT_PATH`
  directory contents, the lock, orphan-cleanup bookkeeping) — never a
  captured raw body.
- The `O_EXCL`-create-then-write-body lock sequence (§7.2). Replaced
  by a temp-directory-then-atomic-rename sequence: the owner body is
  fully written and `fsync`'d **before** the lock name is ever visible,
  closing the partial-observation window.
- The pathname-based post-open identity re-check (§9.1). Replaced by
  an `fstat`-on-the-open-descriptor comparison (`os.SameFile`/`f.Stat()`
  against the pre-open `Lstat`), with a pathname re-`Lstat` retained as
  defense in depth, not the primary check.
- Add/remove/clear operating outside the per-slug lock (§7.2, §7.6).
  All five mutating verbs (`add`, `remove`, `clear`, `capture`,
  `record --resources`) now acquire the same per-slug lock before
  touching `resources.json`/`current.json`; only `capture`/
  `record --resources` perform the ephemeral-scratch orphan sweep,
  since only they ever create scratch content.
- The optional `table` argument and the absent `db_path`/cwd concept
  for Dolt (§5.3, §6.2). `table` is now **mandatory** (no whole-database
  form) and a repo-relative `db_path` is now a required declared field,
  path-gated identically to an `ignored-file` selector and used as the
  Dolt child process's working directory.
- The guessed `"schema"`-key JSON envelope and the unconfirmed
  `WORKING`/`STAGED`/PK-set-change/nonexistent-table behaviors (§6.2,
  §6.3). Replaced by the source-confirmed `{"rows":[...]}`/`{}` shape
  (C22), confirmed `WORKING`/`STAGED` acceptance (C19), a defined
  `dolt-query-error` outcome for a PK-set change on the mandatory
  table (C20), and a defined empty-`tables` outcome for a nonexistent
  table (C21).
- Random `batch_id` generation (§7.3, §12.3). `batch_id` is now
  **content-addressed** — `rb_<12-hex>` derived from a canonical
  encoding of the batch's own `feature` + `results` — so an idempotent
  retry that reproduces identical captured content reproduces the
  identical `batch_id`, rather than minting a fresh random one every
  time.
- The `keep_local`/local-raw-history removal itself is **unchanged**
  from rev-2 (already correct) — restated here only because §7 was
  renumbered/rewritten around it.
- `git-metadata`'s `config` view keeps the already-narrow 4-key
  allowlist (`core.filemode`, `core.ignorecase`, `core.symlinks`,
  `extensions.objectformat`) — unchanged.
- No tracked artifact contains a wall-clock timestamp field — unchanged
  from rev-2.

### 0.3 Golden resource-ID vectors

The canonical `args`-encoding algorithm (§13.1) is **unchanged** from
rev-1/rev-2. Vector 1 (`git-metadata`) and Vector 4 (`ignored-file`)
are therefore byte-identical to rev-2. Vectors 2 and 3
(`adapter-snapshot`) are **recomputed** because §0.2's new mandatory
`db_path` field is now part of the declared `args` set, which changes
the hash input (the capability name itself, `diff-summary`, is
unchanged from rev-2, so this is a different recomputation trigger
than rev-1→rev-2's capability rename):

| Vector | Feature | Kind | Selector | Adapter | Capability | Args (declaration order) | `resource_id` |
|---|---|---|---|---|---|---|---|
| 1 | `model-picker` | `git-metadata` | `head` | *(none)* | *(none)* | `{}` | `res_acc91dc23a8b` (unchanged) |
| 2 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `db_path=data/dolt-db, table=users, from=main, to=HEAD` | `res_cf8e47e6564b` (**recomputed**, was `res_f8a28c218dbb`) |
| 3 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `to=HEAD, db_path=data/dolt-db, table=users, from=main` (reordered) | `res_cf8e47e6564b` (**identical to Vector 2** — order-independence still holds under the added field) |
| 4 | `model-picker` | `ignored-file` | `config/local-secrets.env.template` | *(none)* | *(none)* | `{}` | `res_79f5ac5dca13` (unchanged) |

### 0.4 Requirement-item → section map

| Item | Requirement | Section(s) |
|---|---|---|
| 1 | `check-ignore` fix, magic-name handling | §5.1, §10.1, §10.4 |
| 2 | Zero pre-scan persistence (bounded memory) | §7.1, §8 |
| 3 | Atomic lock publication | §7.2 |
| 4 | Descriptor identity (`fstat`) | §9.1 |
| 5 | Serialize all mutators under one lock | §7.2, §7.6 |
| 6 | Dolt args: `db_path`, mandatory `table`, ref/escaping rules | §5.3, §6.2 |
| 7 | Dolt JSON: exact shape, PK-change, nonexistent table | §6.2, §6.3 |
| 8 | PK-set-change limit, documented exit | §6.2, §14 |
| 9 | Content-addressed publication | §7.3, §12.3 |
| 10 | Scratch/cleanup exact paths | §7.1, §7.5 |
| 11 | `--dry-run` exact behavior | §3, §11 |
| 12 | Local ignore contract + tracked-root gate | §10.3 |
| 13 | Permissions at creation | §7.4 |
| 14 | Directory wire shape, all tagged variants | §12 |
| 15 | Tool path exclusion (already rev-2; restated) | §6.1, §12.3 |
| 16 | Batch/wire canonicalization, content address | §12, §7.3 |
| 17 | `record --resources` two-domain, idempotency | §11 |
| 18 | Tracking / cross-references (final-pass only) | throughout, §0.1 |
| 19 | ACs / matrix rebuild | §14 |

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
tpatch feature resource add    <slug> --kind <kind> --selector <sel> [--adapter <a> --capability <c> --arg k=v ...] [--json]
tpatch feature resource list   <slug> [--json]
tpatch feature resource remove <slug> <resource-id-or-prefix> [--json]
tpatch feature resource clear  <slug> [--json]
tpatch feature resource capture <slug> [--resource <resource-id-or-prefix>] [--dry-run] [--json]
tpatch feature resource diff   <slug> [--resource <resource-id-or-prefix>] [--json]
tpatch record <slug> [existing flags...] [--resources] [--json]
```

Rev-2 removed the rev-1 `--keep-local` flag entirely (§0.2, unchanged
in rev-3): raw bytes are always ephemeral now, so there is nothing
left to opt into.

- **`capture`** is the only verb that ever executes the Dolt adapter,
  reads ignored-file content, or reads Git metadata, and the only verb
  that ever writes tracked (§7.3) state. `--dry-run` (task 11) runs the
  **entire** pipeline for real — lock acquisition, orphan sweep,
  ignored-file/Git-metadata reads into bounded memory, the real Dolt
  invocation inside a real scratch `HOME` (§6.4), redaction scanning —
  and reports exactly what would be published, but guarantees **zero**
  tracked writes and **zero** local writes survive past the
  invocation: the lock file and the scratch `HOME`/control directory it
  creates while running (§7.1) are removed at the end of a `--dry-run`
  invocation exactly as they are for a real one (§7.3 step 4), and no
  `batches/<id>.json` or `current.json` write is ever attempted. This
  is a stronger, more exact restatement of rev-2's "no writes at all"
  claim (rev-2 additionally claimed the scratch directory itself was
  never created on disk for `--dry-run`, which rev-3 corrects: since
  bytes are now read into bounded memory rather than written to
  scratch files at all — task 2 — the scratch tree's remaining purpose
  is Dolt's `HOME`/`DOLT_ROOT_PATH` and the lock, both of which a real
  Dolt invocation needs regardless of `--dry-run`, and both of which
  are ephemeral and cleaned up identically to a real capture).
- **`diff`** is read-only: it never executes the adapter, never reads
  `ignored-file`/Git-metadata content, and never touches the scratch
  tree or the lock. It recomputes lightweight **current** metadata
  (size, mtime, file-set) without opening file content or running
  Dolt, and compares that against the last tracked batch's recorded
  `result` for that resource (§5.1, §7.3.4). Called before any capture
  has ever run for a resource, it reports "no capture yet" (exit 0,
  not an error).

`add`/`list`/`remove`/`clear` behave exactly as `feature claim`'s
quartet does (same `"no such feature: %s"` refusal shape, same
`--json` convention), except `add` additionally computes and persists
`resource_id` (§13) and rejects duplicates (same `selector`+`kind`+
`adapter`+`capability`+canonical `args` tuple already declared for
that feature) as a validation error (exit 2). `add`/`remove`/`clear`
now acquire the same per-slug lock `capture` uses before mutating
`resources.json`/`current.json` (§7.2, task 5) — `add`/`remove`/`clear`
never perform the scratch orphan sweep themselves (only `capture`/
`record --resources` do, since only those two ever create scratch
content, §7.1). `remove`/`clear` only ever mutate the declaration
manifest and the tracked pointer's live index (§7.3.5) — they never
delete or rewrite any historical batch file, which remains a
permanent, immutable audit record even after its resource is
undeclared. `list` never acquires the lock (§7.2, task 5): it is a
pure read of whatever `resources.json`/`current.json` content is
currently visible on disk, which — because both files are always
written via temp-then-atomic-rename (§7.3, §7.6) — is always either
the fully-prior or fully-new content, never a partial read, even if a
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
  bare JSON object/map — §12.1), `added_by_tool_version` (the
  `tpatch` version string that created this declaration; informational
  only, not a timestamp). **Never** contains a capture result, a hash,
  or any raw content.
- **`artifacts/resource-captures/`** — the append-only capture
  history: immutable `batches/<batch_id>.json` files (one per
  successful `capture` invocation, containing every resource result
  produced by that invocation) plus one atomically-rewritten
  `current.json` pointer mapping each resource to the batch that holds
  its latest result (§7.3, §12.3–§12.4). `resources.json` is never
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

**Capture** (task 2, task 3): the matched file(s) are read into a
**bounded in-process memory buffer** (task 2's "zero pre-scan
persistence") — never written to a scratch file first — so a
directory selector's multi-file scan sees a single consistent
point-in-time snapshot rather than files that could each change
mid-scan, without ever placing an unredacted byte on disk. Content is
scanned (redaction, §8) in memory, classified `binary` (a `NUL` byte
in the first 8 KiB) or `text`, and hashed (`SHA-256`, verbatim bytes,
**no** text normalization of any kind — CRLF/LF, trailing newline, and
encoding are all left exactly as found, task 5's "raw local bytes are
verbatim" requirement, restated for the in-memory path). The buffer is
discarded (Go's garbage collector reclaims it; there is no file to
delete) once hashing/scanning completes — the tracked `result` for
this kind is `file_kind` (`"text"`/`"binary"`), `size_bytes`, `hash`
(single file) or `file_count`/`total_bytes`/`combined_hash` (directory
— the combined hash is `SHA-256` over each matched file's repo-relative
path and its own hash, sorted by path, joined `\x00`-delimited, so it
changes if any file's content, size, or the file set itself changes).

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
the duration of the invocation and, if it must appear at all for local
debugging, only in an ephemeral, redacted-if-possible local diagnostic
that is itself deleted before the command returns (§7.5) — it is never
written to any file this PRD defines as persistent, tracked or local.

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

There is no separate "probe" step at all (task 2) — `dolt version` is
**never** executed (C12: it is a real subcommand that can perform a
network update check and read/write the resolved `HOME`, which is not
an acceptable side effect for an identity check). The real SQL
invocation in §6.2 **is** the capability check; a failure there is
reported through the same capability-failure taxonomy as any other SQL
error, not a distinct "probe failed" class.

### 6.2 Capability invocation — `diff-summary` (task 1, task 6, task 7, task 8)

**Declared fields** (all required — no optional Dolt argument remains
in rev-3, task 6): `db_path` (repo-relative path to the Dolt database
directory; path-gated identically to an `ignored-file` selector, §9.1,
and used verbatim as the child process's working directory, `cmd.Dir`),
`table` (exactly one table name — **mandatory**, no whole-database
form; C20/C24 ground this choice, see below), `from`, `to` (commit-ish
values). Any other declared key, a missing required key, or a
duplicate `--arg` for an already-declared key is a validation error
(exit 2) at `add` time.

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
a non-null empty string, not a null.

**Refs, `WORKING`/`STAGED` (C19, source-confirmed)**: `from`/`to`
accept any Dolt commit-ish resolvable by `ResolveRootForRef`(
`dsess/session.go:1022-1031`) — branch names, tags, full or abbreviated
commit hashes, **and** the exact, case-sensitive literal strings
`WORKING` and `STAGED` (`doltdb.go:51-52`; lowercase or mixed case does
**not** match and instead attempts ordinary commit-spec resolution,
which will fail for those literal strings). This corrects rev-2's
explicit "not independently confirmed" hedge — the answer is now
confirmed yes, exact-case only. This PRD's own examples and golden
vectors still use only concrete branch/ref names (`main`, `HEAD`),
not `WORKING`/`STAGED`, purely to keep the running example simple; a
caller may declare either value and it is passed through unmodified
(subject to the escaping rules above, which neither literal violates).

**Primary-key-set-change hard error (C20, source-confirmed, task 8)**:
because `table` is now mandatory, every invocation takes the
single-table code path, where `shouldErrorOnPKChange` is `true`
(`dolt_diff_summary.go:299-321`); a primary-key-set change on the
requested table between `from` and `to` therefore surfaces as a hard
Dolt query error (wrapping `diff.ErrPrimaryKeySetChanged`,
`"primary key set changed"`, `diff/diff_stat.go:31`) rather than a
silently omitted row (which is what the unconfirmed, now-removed
whole-database form would have done). This capability reports that
outcome as `dolt-query-error` (exit 3) with the Dolt error text
captured only in local, ephemeral diagnostics (§7.5) — never in the
tracked artifact. §14 has an explicit `AC`/matrix row exercising this
exact case.

**Nonexistent table (C21, source-confirmed)**: a `table` naming a
table that exists in **neither** `from` nor `to` yields **zero rows**,
not an error (`dolt_diff_summary.go:347-350`, `findMatchingDelta`
returns a zero-value delta with no match). This is a distinct, third
outcome from C20's hard error and is the "first capture" / "table
never existed at `from`" case: the tracked `result.tables` array is
simply empty (`[]`, never `null`, task 14) — no special-cased schema.

**Rename detection, closed `diff_type` enum (C23, source-confirmed)**:
`diff_type` is a **closed**, source-confirmed 4-value string
enumeration — `"added"`, `"modified"`, `"renamed"`, `"dropped"`
(`table_deltas.go:46-49`) — correcting rev-2's "not independently
confirmed against a guessed closed set" hedge. For `"dropped"`,
`to_table_name` is the empty string `""` (not `null`, not omitted); for
`"added"`, `from_table_name` is `""`; for `"renamed"`/`"modified"`,
both names are populated (differing for a rename, identical for a
modify) — this asymmetry is source-confirmed (`table_deltas.go:716-760`,
`doltdb/root_val.go:797-800`'s zero-value-stringifies-to-empty
convention). Despite now being source-confirmed and closed, `diff_type`
is still tracked **verbatim** rather than validated against this set
(forward-compatible if a future Dolt version adds a fifth value) —
a stricter, better-cited version of rev-2's existing posture, not a
reversal of it.

### 6.3 Output parsing and normalization (task 1, task 7)

`dolt sql -r json` wraps a **nonempty** result as a single top-level
JSON object with exactly one key, `"rows"`, an array of row objects
(`table/typed/json/writer.go:37-38`, doc comment at `:56-58`/`:62-64`
confirms "a single JSON object with a single key: \"rows\""); for a
**zero-row** result, the caller writes the literal, distinct 2-byte
string `{}` directly, guarded on `numRows == 0`
(`engine/sql_print.go:110-113`, `:147-149`) — **there is no `"schema"`
key in either case** (C22, correcting rev-2's community/docs-based
guess). The parser recognizes exactly these two valid top-level
shapes and treats any other top-level shape — a missing `"rows"` key
where one is expected, a `"schema"` key that does not exist in the
real output, extra unknown top-level keys, or `"rows"` present but not
a JSON array — as a fatal `dolt-json-parse-error` (exit 3), never a
best-effort partial parse. `{}` maps deterministically to
`result.tables: []`.

For a nonempty `"rows"` array, each row object must contain **all
five** fields (`from_table_name`, `to_table_name`, `diff_type` as JSON
strings; `data_change`, `schema_change` as JSON booleans) — a missing
field, an unknown extra field, a duplicate key, or any field present
with the wrong JSON type (e.g. `data_change` as `0`/`1` instead of a
native boolean) is a fatal `dolt-json-parse-error` (exit 3); this PRD
does **not** defensively coerce `0`/`1`/`"true"`/`"false"` to boolean
(rev-2's defensive-coercion design is removed) — Dolt's real,
source-confirmed JSON writer always emits native JSON booleans for
`BOOLEAN`-typed columns in this code path, so a non-boolean value in
that position indicates a real parsing/version mismatch that should
fail loudly rather than be silently normalized.

### 6.4 Timeouts, caps, environment (task 2, task 7)

| Parameter | Value |
|---|---|
| Invocation timeout | 30 seconds. On timeout: `SIGTERM` to the process group, then `SIGKILL` after 2 more seconds if still running. |
| Captured output cap | 5 MiB combined stdout+stderr, captured into **bounded in-process memory buffers** (task 2's "zero pre-scan persistence" — never written to a scratch file first, §7.1/§8); output beyond the cap is truncated, and the truncation fact is recorded only in local, ephemeral diagnostics (§7.5) — never in the tracked artifact, which never contains raw output at all. |
| Environment | **Not** inherited from the invoking process (task 2's "no inherited credentials"). A fresh, minimal environment is constructed: `HOME=<scratch-home>` and `DOLT_ROOT_PATH=<scratch-home>` pointing at a directory created fresh under this invocation's ephemeral scratch tree (§7.1, `0700`, created before the child process starts so Dolt may write its own ephemeral config/state there if it chooses to — this is not a network or version call, just process-local state under an isolated `HOME`); `PATH` is **not** set at all (the adapter is invoked by its already-resolved absolute path, §6.1, so `PATH` lookup is never needed mid-invocation). No other variable is passed through. |
| Termination | Process-group termination (the child and any of its own children) on timeout, to avoid orphaned Dolt subprocesses. |

A concrete, fully-specified argv/SQL example for Vector 2 (§0.3) —
`db_path=data/dolt-db, table=users, from=main, to=HEAD`:

```
cwd:  <repo-root>/data/dolt-db
argv: /usr/local/bin/dolt sql -r json -q "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change FROM dolt_diff_summary('main', 'HEAD', 'users') ORDER BY from_table_name, to_table_name;"
```

(the absolute path shown, `/usr/local/bin/dolt`, is illustrative only
— it is never the tracked value, §6.1.)

## 7. Ephemeral Scratch, Locking, and the Single Publication Point (task 2, task 3, task 5, task 6, task 9, task 10)

### 7.1 Ephemeral scratch layout and lifecycle (task 2, task 6, task 10)

`.tpatch/local/` is the existing gitignored local root (`LocalIgnoreRule`,
`internal/workflow/session_ignore.go:18`). Before this invocation's
first write anywhere under it, `add`/`capture`/`record --resources`
reuse `workflow.EnsureLocalIgnoreContract` (unchanged reuse from rev-2;
this is a deliberate reuse of the **existing** local-ignore mechanism,
not a second, parallel one) to confirm the local root itself is both
ignored and untracked — a local root that is somehow tracked or not
ignored is refused (`local-root-not-ignored`, exit 3) before any
scratch content is created, matching §10.3's row for this exact case.

Rev-3 removes rev-2's per-resource `raw`/`files/<relpath>` scratch
files entirely (task 2's "zero pre-scan persistence" — see §8): the
scratch tree now holds **only** control data, never a captured byte:

```
.tpatch/local/resource-scratch/<slug>/
  .lock/                          -- lock directory, present only while a mutator holds it (§7.2)
  .lock.tmp-<nonce>/               -- transient, present only during lock acquisition itself
  .lock.stale-<12 lowercase hex>/  -- quarantined stale lock, swept under a freshly-acquired live lock
  batches/.tmp-<batch_id>.json     -- transient, present only mid-write of a tracked batch (§7.3)
  .tmp-current.json                -- transient, present only mid-write of the tracked pointer (§7.3)
  es_<12 lowercase hex>/           -- one ephemeral-scratch directory per in-progress capture/record-resources invocation
    dolt-home/                    -- scratch HOME/DOLT_ROOT_PATH for the Dolt adapter (§6.4); may contain Dolt's own config/state files, never repo content
```

`dolt-home/` is the **only** scratch content that can persist for the
duration of a single invocation beyond in-process memory — it holds
whatever ephemeral config Dolt itself chooses to write under an
isolated `HOME`/`DOLT_ROOT_PATH`, never a captured ignored-file byte
or a copy of Dolt's own query output. Every directory under
`es_<id>/` is created `0700` and every file `0600` **at creation**
(`os.Mkdir`/`os.OpenFile` with the final mode passed directly — never
a separate `os.Chmod` after the fact, task 13). `es_<id>/` is removed
(`os.RemoveAll`, best-effort) as the last step of the invocation on
**both** the success and failure paths; a removal failure is a local
diagnostic (§7.5), not a hard failure.

`add`/`remove`/`clear` (task 5) acquire the same per-slug lock (§7.2)
before touching `resources.json`/`current.json`, but never create
`es_<id>/` and never perform the orphan sweep below — only `capture`/
`record --resources` ever create scratch content, so only they are
responsible for cleaning it up.

`--dry-run` (§3) still acquires the lock and may still create a real
`es_<id>/dolt-home/` if the targeted resource set includes a Dolt
capability (a real Dolt invocation needs a real, isolated `HOME`
regardless of `--dry-run`) — but writes no tracked batch/pointer and
removes `es_<id>/` at the end exactly as a real capture does; nothing
it creates survives the invocation (§3).

**Orphan cleanup** (task 5, task 10): a startup sweep for leftover
`es_*` directories, `.lock.tmp-*`/`.lock.stale-*` directories, and
`batches/.tmp-*`/`.tmp-current.json` files runs **only** after the
current invocation has itself acquired the live lock (§7.2) — never
before acquiring it, and never from `add` (which never acquires
scratch-owning responsibility, task 10's "never from add outside
lock"). Sweeping under the lock guarantees the sweep never races a
different, concurrently-running mutator's own in-flight scratch
content, since only one mutator can hold the lock at a time (§7.2).
Removal is best-effort (`os.RemoveAll`/`os.Remove`), silent on
success, logged as a local diagnostic on failure — never a hard
failure of the current invocation.

### 7.2 Lock semantics (task 3, task 5, task 9)

A single lock per slug, `.tpatch/local/resource-scratch/<slug>/.lock`
(a **directory**, not a file — see below), serializes **every**
mutating verb for that slug: `add`, `remove`, `clear`, `capture`,
`record --resources` (task 5) all acquire it before their first write;
`list`/`diff` never acquire it (§3) and instead rely on
`resources.json`/`current.json` always being read in a fully-written,
temp-then-atomic-rename state (§7.6), so a concurrent `list` never
observes a torn read regardless of whether it holds the lock.

**Why a directory, not `O_EXCL` on a file** (task 3): rev-2's
`O_CREATE|O_EXCL` file-based lock had a real observable window between
the file's creation and its body being written/`fsync`'d, during which
a contender's `O_EXCL` correctly failed (the name existed) but a
reader opening that same name could see a zero-byte or partially
written body. A directory closes this: the owner body
(`owner.json` — `{"pid", "process_start", "host"}`) is written in
full and `fsync`'d, together with the directory entry itself
`fsync`'d, **inside a differently-named temporary directory** before
that directory is ever renamed onto the canonical `.lock` name:

**Acquire** (never blocks/waits — succeeds or refuses immediately, no
polling, no configurable timeout in v1):

1. Create `.lock.tmp-<nonce>/` (`os.Mkdir`, `0700`, `<nonce>` is
   `crypto/rand`-derived, 12 lowercase hex, to avoid two concurrent
   acquirers colliding on the temp name itself).
2. Write `owner.json` (`{"pid": <int>, "process_start": "<ps -o
   lstart= output for this pid, captured immediately>", "host":
   "<os.Hostname()>"}`) inside it, `0600`, `fsync` the file, `fsync`
   the temp directory.
3. `os.Rename(".lock.tmp-<nonce>", ".lock")`. On POSIX, a directory
   rename onto an existing, non-empty target name fails atomically
   (`ENOTEMPTY`/`EEXIST`) without partially applying — so this rename
   either **fully** succeeds (no `.lock` existed; ownership is now
   visible, complete, in one atomic step) or **fully** fails (`.lock`
   already exists) — a contender can never observe a `.lock` directory
   with a missing or partially-written `owner.json`, closing rev-2's
   finding.
4. On rename failure (`.lock` already exists): read and parse
   `.lock/owner.json`.
   - **Malformed** (not valid JSON, or missing a required field) or
     unreadable: implies a lock left behind by a process that crashed
     between its own step 2 and step 3 elsewhere — but because step 3
     is atomic, a malformed `owner.json` inside an already-renamed
     `.lock` can only mean on-disk corruption after the fact, not a
     legitimate in-flight acquisition; there is no partial-write case
     to tolerate here (that risk was eliminated by the rename design
     above, not merely reduced). Quarantine immediately (`os.Rename(
     ".lock", ".lock.stale-<12hex>")`, `crypto/rand`-suffixed so two
     concurrent quarantiners cannot collide) and retry step 1 once.
   - **`host` does not match `os.Hostname()`**: the lock may belong to
     another machine on a shared filesystem, whose process liveness
     cannot be verified locally. Refuse immediately
     (`capture-lock-held-remote`, exit 3) — do not attempt to reclaim.
   - **`host` matches**: run `ps -o lstart= -p <pid>` fresh.
     - No output (process no longer exists): stale. Quarantine and
       retry step 1 once.
     - Output **matches** the recorded `process_start` string exactly:
       the same live process still holds the lock. Refuse immediately
       (`capture-in-progress`, exit 3), no wait.
     - Output exists but **differs** from the recorded
       `process_start` (same numeric PID, different process — PID
       reuse guard): stale. Quarantine and retry step 1 once.
   - A brief **initialization grace period is not needed** here
     specifically because step 3's rename is atomic — there is no
     window where a *live, legitimately-acquiring* process's lock
     looks malformed or absent to a concurrent reader; "malformed"
     therefore always means "quarantine-worthy," never "wait and
     recheck."
5. If a single retry of step 1 (after quarantining) also loses the
   rename race to a different concurrent acquirer, refuse
   (`capture-lock-contended`, exit 3) rather than looping — v1 does
   not retry more than once, keeping "no polling, no wait" honest.

**Release**: `os.RemoveAll(".lock")` as the last step of the
invocation (success or failure path alike), best-effort — a failed
removal is a local diagnostic only; the next invocation's PID/
`process_start` staleness check reclaims it correctly once this
process has actually exited.

**Quarantine sweep** (task 3): `.lock.stale-*` directories are only
ever removed by the **next invocation that successfully acquires the
live `.lock`** (§7.1's orphan sweep, now explicitly "under an acquired
live lock" per this section) — never by a process that has not itself
acquired `.lock`, so a quarantine sweep can never race a different,
concurrently-quarantining acquirer.

**Platform scope** (unchanged from rev-2): `ps -o lstart=` and
PID-liveness-via-`ps`, plus directory-rename atomicity, are POSIX-shaped
and validated on macOS/Linux, consistent with this project's existing
macOS/Linux-only validation scope (`ADR-004-m10-copilot-proxy-ux.md`
D6 precedent); Windows lock semantics are best-effort/unsupported in
v1, not claimed otherwise.

### 7.3 The single publication point (task 3, task 9, task 16)

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
2. **Compute the content-addressed `batch_id`** (task 9, task 16):
   build the canonical batch body — `{"feature": <feature>, "results":
   <staged results, sorted by resource_id byte-ascending>}` — through
   `CanonicalBatchJSON` (§12's fixed-field, map-free canonical
   encoder), hash it (`SHA-256`), and take `batch_id = "rb_" +
   hex(...)[:12]`. Unlike rev-2's random `batch_id`, an unchanged
   underlying capture (identical `feature` + identical staged
   `results`) always reproduces the identical `batch_id` — this is
   what makes a retry after a step-3 crash (below) idempotent without
   depending solely on the lock.
3. **Write the batch**: if `artifacts/resource-captures/batches/<batch_id>.json`
   already exists on disk, compare its bytes to the freshly-computed
   canonical body.
   - **Identical bytes**: this exact batch was already fully written
     by a prior (possibly crashed-before-step-4) invocation. Skip
     straight to step 4 (idempotent re-publish) rather than
     rewriting an identical file.
   - **Different bytes**: a `batch_id` collision on genuinely
     different content is a fatal integrity error
     (`batch-id-collision`, exit 3) — refuse rather than silently
     overwrite a distinct historical batch; this is expected to be
     unreachable in practice (`SHA-256` collision) and exists purely
     as a defensive, fail-closed guard, not an anticipated real
     outcome.
   - **Absent**: write to `artifacts/resource-captures/batches/.tmp-<batch_id>.json`
     (ordinary tracked-repo file permissions, `0644`, since it never
     contains raw bytes), `fsync` the file, `os.Rename` it to
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

**Crash-window analysis** (task 3):

| Crash point | State left behind | Recovery |
|---|---|---|
| Before step 3's rename | No new batch file (or an orphaned `batches/.tmp-<batch_id>.json`) | Orphan `.tmp-*` swept at next invocation's start under its own acquired lock (§7.1/§7.2); a re-run recomputes the identical content and therefore the identical `batch_id` (step 2), and step 3's "identical bytes" branch makes the retry idempotent even if the orphan sweep somehow left a partial file behind (a partial `.tmp-*` is removed by the sweep before this check, so in practice the retry always re-lands on the "absent" branch) |
| After step 3's rename, before step 4's rename | A fully-written, permanently orphaned `batches/<batch_id>.json` that no `current.json` entry ever references | Harmless — never surfaced by `list`/`diff`/`capture` (§4.1's "missing batch" case does not apply here; this is an *extra*, unreferenced batch, not a missing one); left in place, not garbage-collected, in v1. A retry recomputes the same `batch_id`, finds it already present with identical bytes (step 3's idempotent branch), and proceeds straight to step 4 |
| During step 4's temp-write, before its rename | Orphaned `.tmp-current.json` | Swept at next invocation's start; the last successfully-renamed `current.json` (from a previous, fully-committed invocation, or absent if this was the first-ever capture) remains authoritative and untouched |
| After step 4's rename | Fully committed | No recovery needed |

**Concurrency**: the lock (§7.2) already prevents two invocations for
the same slug from reaching step 3/4 simultaneously; nothing in §7.3
depends on filesystem-level atomicity across *different* slugs (each
slug's `artifacts/resource-captures/` tree is independent). `list`
(never lock-holding, §7.2) reading `current.json` mid-rename always
observes either the fully-prior or fully-new file, never a torn one,
because the rename is atomic at the filesystem level regardless of
whether the reader holds a lock.

### 7.4 Permissions (task 13)

Restated precisely because it spans both the ephemeral (§7.1) and
tracked (§7.3) trees, which have **different** permission
requirements — every creation call passes its final mode directly, no
call ever creates-then-`chmod`s:

- Ephemeral scratch (`es_<id>/`, `dolt-home/`, the lock directory and
  its `owner.json`): directories `0700`, files `0600`, always at
  creation.
- Tracked artifacts (`resources.json`, `batches/<id>.json`,
  `current.json`): ordinary repository file permissions (`0644`),
  since they never contain raw bytes or secrets by construction
  (§8.1) — there is nothing to protect with tighter permissions, and
  using non-standard permissions on a tracked, checked-in file would
  be surprising to anything else that reads the working tree.

### 7.5 Local diagnostics on failure (task 2, task 10)

When a `capture`/`record --resources` invocation fails at any stage
(§7.3 step 1), no tracked failure envelope is ever written (unchanged
principle from rev-1/rev-2) — failure detail is either printed
directly to the CLI's own stdout/stderr for that invocation (never
persisted to any file) or, if richer detail is useful for later
inspection, written to a file under the **same** `es_<id>/` ephemeral
tree that is deleted at the end of the invocation (§7.1). Because
rev-3 eliminates on-disk raw scratch content entirely (§8), any such
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

### 7.6 Read path (`list`) during a concurrent mutation (task 3, task 5)

`list` never acquires the lock (§7.2) and never blocks on one. It
reads `resources.json` (declaration manifest) and, if present,
`artifacts/resource-captures/current.json` (pointer), both of which
are always in a fully-written state on disk regardless of a
concurrently-running mutator, because every writer uses the same
temp-then-atomic-rename discipline (§7.3 for `current.json`; the
declaration manifest's own existing `add`/`remove`/`clear` write path,
unchanged from rev-1, already used this discipline and continues to).
A `list` that races a mutator therefore always observes either the
state immediately before or immediately after that mutator's next
atomic rename — never a torn file — independent of whether `list`
happens to run while the lock is held.

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
`[]byte`/`io.Reader` (bounded by §5.1's per-file/total/file-count
limits, so the bound is already enforced before the read begins);
Dolt's stdout/stderr are captured into a bounded in-process
`bytes.Buffer` (bounded by §6.4's 5 MiB combined cap) as the child
process runs, never redirected to a scratch file. Scanning (§8.2),
hashing, and classification (§5.1's `binary`/`text` first-8-KiB check)
all operate on these in-memory buffers directly. Once a resource's
`result` has been computed, the buffer is discarded (garbage
collected) — there is no file to delete, because none was ever
created. The only content that can legitimately exist as a file on
disk during an invocation is Dolt's own ephemeral config/state under
its isolated scratch `HOME` (§6.4, §7.1's `dolt-home/`), which this
PRD does not control the shape of and does not scan (it is Dolt's own
operational state, not captured repository/database content).

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
sufficient alone for resource capture. Rev-1's gate resolved the
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
`db_path` (§5.3, §6.2) uses this exact same gate — it is not a
Dolt-specific path policy; only the Dolt **executable** itself (§9.2)
uses the opposite-direction policy.

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
| Dolt executable's device/inode/size/mtime differ immediately after invocation vs. immediately before | Refused: `adapter-executable-replaced`, result discarded |

## 10. Git Ignore/Tracked Gate Semantics (task 1, task 6)

### 10.1 `check-ignore` exit-code handling (task 1)

`git check-ignore -q --no-index -- <pathname>` (no `--literal-pathspecs`
— C17: `check-ignore` has no such option and fails fatally, exit
`128`, if one is passed; this reuses the **existing**
`gitutil.IsPathIgnored` invocation verbatim): exit `0` = ignored (gate
passes); exit `1` = not ignored (gate fails, `not-ignored`, exit 3);
any other exit code (`128` and similar) is a fatal Git error —
refused (`git-ignore-check-error`, exit 3), never silently treated as
either "ignored" or "not ignored."

**Magic-name handling** (C18, empirically verified against installed
Git): because `check-ignore`'s pathname argument still parses a
leading `:` for pathspec magic (unlike `*`/`?`/`[]`, which are inert
to this command), any selector whose first byte is `:` is passed as
`./<selector>` — this disarms colon-magic parsing (the argument no
longer begins with a bare `:` byte) while resolving to the identical
on-disk path. A selector not beginning with `:` is passed unchanged.
This is the one deliberate, documented exception to this project's
general "always pass literal pathspecs" discipline — it exists solely
because `check-ignore` has no `--literal-pathspecs` equivalent to rely
on instead, and the `./`-prefix trick achieves the same literal-path
guarantee for this one call site.

### 10.2 `ls-files --error-unmatch` exit-code handling

`git --literal-pathspecs ls-files --error-unmatch -- <path>`: exit `0`
= tracked (gate fails when combined with "ignored" — §5.1 check 2);
exit `1` **with** the standard "did not match any file(s) known to
git" stderr shape = untracked (gate passes); any other exit code, or
exit `1` with unexpected stderr, is a fatal Git error — refused
(`git-ls-files-error`, exit 3), same fail-closed treatment as §10.1.
Every index-entry-selector call (`git-metadata`'s `index-entry` view,
§5.2) uses this same literal-pathspec form.

### 10.3 Local-ignore-root reuse (task 6)

Before the first write to `.tpatch/local/resource-scratch/` on a given
machine (i.e. once per invocation, cached in-process, not persisted),
`add`/`capture`/`record --resources` call the **existing**
`workflow.EnsureLocalIgnoreContract(repoRoot, resourceScratchRoot)`
(`internal/workflow/session_ignore.go:138`) — reused exactly as-is,
not re-invented — which verifies Git is available, the path is inside
the worktree, and `gitutil.IsPathIgnored` reports it ignored;
`IsPathIgnored`'s own `check-ignore` invocation is precisely the
deliberate pathname exception documented in §10.1 (it does not, and
cannot, use `--literal-pathspecs`). Because `EnsureLocalIgnoreContract`
alone does not close the `--no-index` gap for the scratch root any
more than it does for an `ignored-file` selector (C13), this design
layers the **same** tracked-file gate from §5.1/§10.2 on top: `git
--literal-pathspecs ls-files --error-unmatch -- .tpatch/local/` must
also report untracked. Either check failing to hold is
`local-path-not-ignored`/`local-path-tracked` (exit 3) — refused
before any scratch content is created, exactly mirroring ADR-027 D1's
ignored-before-first-write mandate. This PRD does not invent a second
ignore mechanism — it reuses the one that exists and adds only the
missing tracked-file half, the same addition already made for
`ignored-file` selectors in §5.1.

### 10.4 Pathspec-magic rows (task 1, task 6)

| Selector / call | Invocation | Behavior |
|---|---|---|
| `:(glob)config/*.env` (a literal filename that happens to start with pathspec-magic syntax), `check-ignore` | `git check-ignore -q --no-index -- './:(glob)config/*.env'` (C18's `./`-prefix rule applied) | Treated as the literal filename; no magic parsing, no fatal error |
| Same selector, unprefixed | `git check-ignore -q --no-index -- ':(glob)config/*.env'` | Fatal: unsupported magic keyword for this command (empirically confirmed, exit 128) — this PRD never emits this form (C18's rule always applies the prefix first) |
| `:/topmagic.env`, unprefixed, `check-ignore` | `git check-ignore -q --no-index -- ':/topmagic.env'` | Empirically: silently accepted (exit 0/1 per actual ignore status, no error) — still routed through the `./`-prefix rule regardless, since this PRD's rule is "any leading `:`", not "any *unsupported* magic," to avoid depending on which magic keywords are or are not supported by a given Git version |
| `config/**/local.env`, `check-ignore` (no leading `:`) | `git check-ignore -q --no-index -- 'config/**/local.env'` | `*`/`?`/`[]` are inert to `check-ignore` (empirically confirmed) — treated as literal characters, no glob expansion |
| Same selector, `ls-files --error-unmatch` | `git --literal-pathspecs ls-files --error-unmatch -- 'config/**/local.env'` | Treated as a literal path containing the literal characters `**`; no magic expansion (`--literal-pathspecs` supported here, unlike `check-ignore`) |
| `:(glob)config/*.env`, `ls-files --error-unmatch` | `git --literal-pathspecs ls-files --error-unmatch -- ':(glob)config/*.env'` | Treated as the literal filename `:(glob)config/*.env` (no `./`-prefix needed — `--literal-pathspecs` already disarms this) |

## 11. `record --resources` Semantics (task 11, task 17)

Unchanged high-level ordering from rev-1/rev-2 — Git-side capture and
resource-domain publication remain **two separate atomic domains**;
what changes in rev-3 is that "staging" (§7.3 step 1) is ephemeral
in-memory only (never writes a batch file, task 2), and "publishing"
is the same §7.3 steps 2–4 a standalone `capture` would run, using the
**content-addressed** `batch_id` (§7.3 step 2) rather than a random one.

1. **Zero-resource preflight**: zero declared resources refuses
   immediately, before touching Git and before lock acquisition
   (`no-resources-declared`, exit 1), unchanged from rev-1/rev-2.
2. **Stage** (ephemeral, in-memory metadata only, task 2/task 11):
   acquire the per-slug lock (§7.2), run the lock-gated orphan sweep
   (§7.1), then run §7.3 step 1 for every declared resource —
   ephemeral scratch (Dolt `HOME` only, §7.1), bounded in-memory
   ignored-file/Dolt reads, redaction — but stop before step 2 (no
   batch file written yet); the fully-computed candidate batch content
   (and its content-addressed `batch_id`, computed the same way as
   §7.3 step 2, since it depends only on `feature`+`results`) is held
   in memory pending step 4 below. The lock remains held across steps
   2–4 (it is one invocation of `record --resources`, not two).
3. **Git-side capture**: `record`'s existing, unmodified capture-mode
   dispatch runs, completely unaffected by step 2's outcome.
4. **Publish, gated on Git success**:
   - Git failed: the record command's existing failure behavior
     propagates; the in-memory candidate batch from step 2 is simply
     discarded (never written anywhere — "ephemeral metadata only,"
     task 11) regardless of its own success/failure. The lock is
     released.
   - Git succeeded and step 2 also succeeded: run §7.3 steps 3–4 now
     (write batch, publish pointer, cleanup/release lock) using the
     already-computed candidate content and its precomputed
     `batch_id` — no adapter/Git-metadata re-execution.
   - Git succeeded but step 2 failed, or Git succeeded and step 2
     succeeded but the publish step (§7.3 steps 3–4) itself fails: a
     **partial-domain** result, `resource-domain-incomplete` (exit 1):
     > canonical patch recorded successfully; resource capture did not
     > complete: `<reason>`. Retry with `tpatch feature resource
     > capture <slug>` — this re-stages and republishes and is safe to
     > re-run.

**Idempotency (task 17, corrected from rev-2)**: because `batch_id` is
now content-addressed (§7.3 step 2) rather than random, a retry that
recomputes **identical** underlying content (same declared resources,
same repository/Dolt state at retry time) reproduces the **identical**
`batch_id` and lands on §7.3 step 3's "identical bytes, skip to
pointer publish" branch — this is what makes the retry safe, not a
"fresh ID every time" property (rev-2's phrasing implied the opposite
and is corrected here). A retry that runs after the underlying state
has genuinely changed (e.g. a Dolt table's data changed between the
failed attempt and the retry) correctly produces a **different**
`batch_id`, because the content genuinely differs — this is expected
and correct, not a re-run bug.

**Interactions** (unchanged from rev-1/rev-2): an empty Git-side
capture accepted by existing logic counts as Git-side success for
gating publish; `--auto`/commit-range flags compose with `--resources`
without special-casing; `record --resources` has no `--resource`
subset flag of its own (it always targets every declared resource —
the standalone `feature resource capture <slug> --resource <id>` is
the only subset-targeting entry point, matching its promised
all-declared-resources scope exactly). `record --resources` has no
`--dry-run` of its own either (unchanged — only `feature resource
capture`/`diff` support `--dry-run`/resource-only preview, §3).

**Exit codes** (restated for rev-3's refusal names):

| Code | `feature resource {add,list,remove,clear,capture,diff}` | `record --resources` |
|---|---|---|
| `0` | Success (including `diff` reporting "no capture yet") | Success |
| `1` | Internal error; `tracked-batch-missing` (§4.1) | Same, plus `no-resources-declared` and `resource-domain-incomplete` |
| `2` | Validation: bad kind/adapter/capability/view, unknown/duplicate/missing `--arg` (including missing `db_path`/`table`), `NUL`/control byte/backslash/`..` in a Dolt arg, missing index entry at `add`, `table` mismatch between selector and declared field | n/a (unmodified) |
| `3` | State/policy refusal: `not-ignored`, `tracked-and-ignored`, `git-ignore-check-error`, `git-ls-files-error`, any `symlink-component-refused`/`path-missing`/`path-replaced-during-open`, any size/count limit, `redaction-refused`, `adapter-missing`/`adapter-executable-in-repo`/`adapter-executable-replaced`, `dolt-query-error`, `dolt-json-parse-error`, `local-root-not-ignored`/`local-path-tracked`, `capture-lock-held-remote`/`capture-in-progress`/`capture-lock-contended`, `batch-id-collision`, `index-entry-missing` | Same set applies to staging (§11 step 2); surfaces as `resource-domain-incomplete` (exit 1) if Git succeeded, or as record's own existing exit code (with the discarded-batch diagnostic) if Git failed |

## 12. Wire Schemas (task 14, task 16)

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

(`args` entries are sorted by `key`, byte-ascending — the same sort
order as the canonical-hash encoding, §13.1, though this array and
that hash input are still two independently-defined serializations
that happen to share a sort rule, not the same code path.)

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
no index entry by definition. `combined_hash` remains `SHA-256` over
each entry's `path`+`raw_sha256`, sorted by `path`, `\x00`-joined,
unchanged from §5.1 — `files[]` is additional detail, not a
replacement for that aggregate.)

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

`batch_id` is **content-addressed** (task 9/task 16, §7.3 step 2), not
random: `rb_5cff7f222dce` is the actual, computed `SHA-256`-derived ID
for this exact `{feature, results}` body (`results` sorted by
`resource_id` byte-ascending, per `CanonicalBatchJSON`, §12 intro) —
re-running `CanonicalBatchJSON` over this exact JSON reproduces this
exact `batch_id`, and this revision's validation pass independently
confirmed this by reimplementing `CanonicalBatchJSON` in a standalone
script and recomputing the exact value shown above. `results` is sorted by `resource_id`, byte-ascending
— `res_79f5ac5dca13` < `res_acc91dc23a8b` < `res_cf8e47e6564b`.

`raw` is `null` for `git-metadata` (no raw-byte concept applies) and
always a populated `{hash, byte_count}` object for `adapter-snapshot`/
`ignored-file` (no optional opt-in — the ephemeral, never-persisted
bytes are always hashed in memory before the buffer is discarded,
§8.1). `tool_identity` is `null` for kinds with no adapter/executable
concept (`git-metadata`, `ignored-file`) and populated (`basename`
+`binary_sha256` only, never an absolute path, §6.1) for
`adapter-snapshot`. The example `oid` above is an ordinary,
valid-shaped 64-hex-character SHA-256 Git object ID (illustrative, not
the well-known empty-tree hash `4b825dc642cb6eb9a060e54bf8d69288fbee4904`,
which §12.2's detached-`HEAD` example uses instead, task 10).

### 12.4 `current.json` (the tracked pointer)

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

`resources` is sorted by `resource_id`, byte-ascending, for
determinism (never dependent on map-iteration order — this is a
`[]struct`, not a `map`). `latest_batch_id` is the `batch_id` of the
most recent successful publish (§7.3 step 4), regardless of which
specific resources that invocation touched — a convenience field for
"what's the newest batch that exists at all," distinct from the
per-resource index, which is what `diff`/`list --json` actually
resolve against. `current.json` is the **only** file a reader consults
(§7.3 step 4) — `list`/`diff` never scan `batches/` directly.

### 12.5 First capture, add/remove/change shapes

The **first-ever** capture for a resource produces a `batches/<id>.json`
entry with the exact same schema shape as any subsequent one (§14 has
the `AC` for this) — there is no distinct "initial" schema; a
nonexistent-table Dolt result (C21) and a zero-row Dolt result both use
`{"tables": []}` (§12.2), never a special-cased shape. `remove`/
`clear` (§3, §4) only ever rewrite `resources.json` and prune
`current.json`'s `resources` array (dropping the entry for the
removed `resource_id` — a resource with no declaration and no
`current.json` entry is simply absent from `list`, matching normal
"never declared" behavior) — both do so under the per-slug lock
(§7.2, task 5) and never rewrite any `batches/<id>.json` file; every
batch that ever existed remains on disk, byte-for-byte, forever
(immutable historical audit trail).

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
| 2 | `model-picker`, `adapter-snapshot`, `dolt:diff-summary:users`, `dolt`, `diff-summary`, `{"db_path":"data/dolt-db","table":"users","from":"main","to":"HEAD"}` (declared `db_path, table, from, to` order) | `res_cf8e47e6564b` |
| 3 | Same as Vector 2, `args` declared `to, db_path, table, from` order | `res_cf8e47e6564b` (**identical** — order-independence, reconfirmed with the added `db_path` field) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

Vectors 1 and 4 are byte-identical to rev-1/rev-2 (unaffected by any
rev-3 change). Vectors 2/3 are **recomputed** because `db_path` (§6.2,
task 6) is now a mandatory declared field and therefore part of the
hashed `args` set; recomputed independently via a standalone Python
script implementing §13.1/§13.2 verbatim as part of this revision's
validation pass.

## 14. Acceptance Criteria (task 19)

Clause-level, `AC-<n>` tagged. Each `AC` is one testable clause;
`ADR-033-resource-capture-boundary.md`'s Test Matrix cites these tags
directly. Renumbered from rev-2 (some rev-2 clauses are removed as
obsolete — e.g. the `O_EXCL`-file-lock-specific clauses — and many new
clauses are added for rev-3's redesigns); do not assume `AC-<n>` means
the same thing it did in rev-2.

**Dolt SQL redesign (task 1, task 6, task 7, task 8)**

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
- `AC-7`: `dolt sql -r json` output of the literal 2-byte string `{}`
  is parsed as zero rows (`result.tables: []`), and a `{"rows":[...]}`
  envelope with any other/extra top-level key is refused as a fatal
  parse error.
- `AC-8`: A primary-key-set change on the mandatory `table` between
  `from` and `to` surfaces as `dolt-query-error` (exit 3), not a
  silent omission.
- `AC-9`: A `table` that exists in neither `from` nor `to` yields
  `result.tables: []` (zero rows), not an error — distinct from
  `AC-8`'s outcome.
- `AC-10`: The exact literal strings `WORKING`/`STAGED` (uppercase) are
  accepted as `from`/`to` values without special-casing at the
  argument-validation layer (they pass the same escaping rules as any
  other value).
- `AC-11`: A `from`/`to` value shaped as a dot-range (containing `..`)
  is refused at validation (`AC-2`) before ever reaching Dolt, so the
  dot-range vs. explicit-form ambiguity in `dolt_diff_summary`'s own
  argument parsing is never exercised.
- `AC-12`: The first-ever capture of a resource produces the identical
  `batches/<id>.json` entry schema shape as any later capture, and a
  zero-row Dolt result uses the same `{"tables": []}` shape as a
  nonexistent-table result (§12.5).

**No version probe; executable identity (task 2)**

- `AC-13`: `dolt version` is never invoked anywhere in the capture
  pipeline.
- `AC-14`: The tracked `tool_identity` contains only `basename` and
  `binary_sha256` — never an absolute path, in any tracked file.
- `AC-15`: The Dolt invocation's environment contains only `HOME`/
  `DOLT_ROOT_PATH` (both pointing at a fresh, `0700` ephemeral scratch
  directory created before the child starts) — no inherited variable
  from the invoking process's environment, no `PATH`.
- `AC-16`: A resolved Dolt executable located inside the repository
  working tree (or under any `.git` directory) is refused
  (`adapter-executable-in-repo`).
- `AC-17`: A resolved Dolt executable whose device/inode/size/mtime
  differ immediately after invocation vs. immediately before is
  refused (`adapter-executable-replaced`) and its result is discarded.

**Zero pre-scan persistence; privacy (task 2, task 6)**

- `AC-18`: An `ignored-file` selector's content is read into an
  in-process buffer and never written to any scratch or other file
  before scanning/hashing completes.
- `AC-19`: Dolt's stdout/stderr are captured into an in-process,
  bounded buffer and never redirected to or copied into a scratch
  file before parsing/scanning completes.
- `AC-20`: A value matching any of the six redaction classes refuses
  the entire invocation (`redaction-refused`), with no partial batch
  written for any resource in that invocation, even unaffected ones,
  and with no unredacted byte having been written to any file at any
  point before the refusal.
- `AC-21`: No tracked file anywhere contains a wall-clock timestamp
  field.
- `AC-22`: `feature resource diff` on an `ignored-file` resource
  reports exactly which of `size_bytes`/`hash`/`file_count`/
  `total_bytes`/`combined_hash`/file-set membership changed, never a
  textual line-level diff.

**Descriptor-identity path gate (task 4)**

- `AC-23`: A selector (`ignored-file` or Dolt `db_path`) whose ancestor
  directory (not the final component) is a symlink is refused
  (`symlink-component-refused`), regardless of where that symlink
  points.
- `AC-24`: A selector replaced by a symlink at the final component
  between the walk and the open is refused via `O_NOFOLLOW`/`ELOOP`.
- `AC-25`: A selector whose underlying file is replaced (different
  device/inode) between the walk and the open is refused
  (`path-replaced-during-open`), detected via `os.SameFile` on the
  **open file descriptor**'s `FileInfo`, not a second pathname
  `Lstat`.
- `AC-26`: A dangling ancestor (missing path component) is refused
  (`path-missing`).
- `AC-27`: This gate re-runs independently for every descendant file
  of a directory selector, both at `add` and at every `capture`.
- `AC-28`: The Dolt executable path uses the separate, opposite-
  direction policy (§6.1/§9.2) and is never subject to the ancestor-
  symlink-refusal rule that applies to `AC-23`–`AC-27`.

**`check-ignore` fix, ignored/tracked Git gates (task 1, task 6)**

- `AC-29`: `check-ignore` is invoked without `--literal-pathspecs` (no
  such option exists for it) — verified by asserting the invocation
  never includes that flag.
- `AC-30`: `check-ignore` exit `1` (not ignored) and exit `>1` (fatal)
  produce distinct refusal reasons, neither treated as "ignored."
- `AC-31`: A selector whose first byte is `:` is passed to
  `check-ignore` with a `./` prefix, and resolves to the identical
  on-disk path as the unprefixed form would if `check-ignore` could
  accept it literally.
- `AC-32`: `*`/`?`/`[]` characters in a `check-ignore` pathname
  argument never trigger wildcard/glob matching.
- `AC-33`: `ls-files --error-unmatch` exit `0` (tracked) and any
  non-standard exit/stderr shape produce distinct refusal reasons, and
  every such call uses `--literal-pathspecs`.
- `AC-34`: A selector is refused unless it is **both** ignored (via
  `AC-30`) **and** untracked (via `AC-33`) — recheck at `add` and at
  every `capture`.

**Local ignore contract, tracked-root gate (task 6)**

- `AC-35`: The scratch root's ignored status is verified via the
  existing `EnsureLocalIgnoreContract`, not a second, parallel ignore
  mechanism.
- `AC-36`: The scratch root is also verified untracked via the
  `AC-33`-style `ls-files --error-unmatch` gate; either check failing
  refuses (`local-root-not-ignored`/`local-path-tracked`) before any
  scratch content is created.

**Atomic lock, serialized mutators (task 3, task 5, task 9)**

- `AC-37`: A fresh lock acquisition writes `owner.json` (PID,
  `process_start`, host) inside a temp-named directory and only then
  atomically renames it onto `.lock` — no intermediate state exposes a
  `.lock` directory with a missing/partial `owner.json` to a
  concurrent reader.
- `AC-38`: A second concurrent invocation for the same slug refuses
  immediately (`capture-in-progress`) while the first is live, with no
  blocking/wait.
- `AC-39`: A lock left by a dead PID (verified via `ps -o lstart=`
  returning no output) is quarantined and reclaimed automatically by
  the next invocation.
- `AC-40`: A lock whose PID is alive but whose `process_start` differs
  from the recorded value (PID reuse) is quarantined and reclaimed
  automatically.
- `AC-41`: A malformed `owner.json` is quarantined and reclaimed
  automatically.
- `AC-42`: A lock recorded on a different hostname is refused
  (`capture-lock-held-remote`) without any reclaim attempt.
- `AC-43`: A second retry losing the rename race after quarantining is
  refused (`capture-lock-contended`), not retried a second time.
- `AC-44`: `add`/`remove`/`clear` acquire the same per-slug lock as
  `capture`/`record --resources` before mutating
  `resources.json`/`current.json`.
- `AC-45`: `add`/`remove`/`clear` never create ephemeral scratch
  content and never perform the orphan sweep.
- `AC-46`: `list` never acquires the lock and always observes either
  the fully-prior or fully-new `resources.json`/`current.json` content,
  never a torn read, regardless of a concurrent mutator.

**Permissions, scratch/orphan cleanup (task 10, task 13)**

- `AC-47`: Every ephemeral scratch directory (`es_<id>/`, `dolt-home/`,
  the lock temp/canonical directory) is created `0700` and every file
  `0600` at creation (never via a separate `chmod` after a
  looser-permission create).
- `AC-48`: An orphaned ephemeral scratch directory, `.lock.tmp-*`/
  `.lock.stale-*` directory, or `batches/.tmp-*`/`.tmp-current.json`
  file left by a simulated crash is swept **only after** the sweeping
  invocation has itself acquired the live lock, never before.
- `AC-49`: `add` never performs the orphan sweep (verified by asserting
  no `es_*`/`.lock.stale-*` removal occurs during an `add` invocation
  even when such orphans are present).

**Content-addressed single publication point (task 3, task 9, task 16)**

- `AC-50`: A successful multi-resource `capture` writes exactly one
  new `batches/<id>.json` file (unless an identical one already
  exists, `AC-52`) and rewrites `current.json` exactly once.
- `AC-51`: `batch_id` is deterministically derived from
  `CanonicalBatchJSON({"feature","results"})` — recomputing it from
  the same batch content reproduces the identical `batch_id`.
- `AC-52`: A retry that reproduces identical batch content finds the
  existing `batches/<batch_id>.json` with identical bytes and skips
  directly to pointer publication, without rewriting the batch file.
- `AC-53`: A retry that computes the identical `batch_id` for
  **different** content (a hash collision) is refused
  (`batch-id-collision`), never silently overwritten.
- `AC-54`: A crash simulated between the batch rename and the
  `current.json` rename leaves a permanently orphaned, harmless batch
  file that no subsequent `list`/`diff` ever surfaces, and a re-run
  recomputes the identical `batch_id` and proceeds via `AC-52`.
- `AC-55`: A crash simulated during either temp-file write (before its
  rename) leaves only a `.tmp-*` artifact, swept at the next
  invocation's start (`AC-48`), with no effect on the last successfully
  committed `current.json`.
- `AC-56`: `remove`/`clear` never delete or modify any
  `batches/<id>.json` file, only `resources.json` and `current.json`'s
  live index, and do so under the per-slug lock (`AC-44`).
- `AC-57`: `current.json` is the only file `list`/`diff` read to
  resolve a resource's latest result — neither ever scans `batches/`
  directly.

**Git metadata / tagged variants (task 14)**

- `AC-58`: `head`'s `symbolic_ref` is `null` if and only if `detached`
  is `true`.
- `AC-59`: The `config` view refuses any key outside the exact
  four-key allowlist.
- `AC-60`: An `index-entry` selector queried with a path containing
  pathspec-magic characters resolves to the literal path under
  `--literal-pathspecs`.
- `AC-61`: A directory `ignored-file` result includes a stable,
  `path`-sorted `files[]` array with `{path, raw_sha256, byte_count,
  mode}` per entry, in addition to the aggregate
  `file_count`/`total_bytes`/`combined_hash` fields.
- `AC-62`: Every kind/view's tagged `result` shape (§12.2) is exercised
  by at least one test: `head` attached, `head` detached, `ref`,
  `index-entry`, `config` (set and unset), `ignored-file` single file,
  `ignored-file` directory, `adapter-snapshot`.

**`--dry-run`, transaction / `record --resources` (task 11, task 17)**

- `AC-63`: `feature resource capture <slug> --dry-run` writes zero
  tracked files (`resources.json` is not touched beyond its pre-existing
  content; no `batches/`/`current.json` write occurs) and leaves zero
  local files after it returns (its lock and any scratch `dolt-home/`
  it created are removed before exit, identically to a real capture).
- `AC-64`: `record --resources` on a feature with zero declared
  resources refuses (`no-resources-declared`) before any Git
  invocation and before lock acquisition.
- `AC-65`: A resource-staging failure combined with Git-side success
  produces `resource-domain-incomplete` with the exact recovery-command
  message, while the Git-side canonical patch is confirmed present and
  correct.
- `AC-66`: A resource-staging failure combined with Git-side failure
  discards the staged (never-written) candidate batch and surfaces
  only record's existing Git-failure behavior.
- `AC-67`: A successful stage and successful Git-side capture publish
  the batch and pointer atomically, verified by asserting both
  `batches/<id>.json` and `current.json` reflect the same invocation
  together, never partially.
- `AC-68`: Re-running `feature resource capture <slug>` (or
  `record --resources`) after a publish-step failure, with the
  underlying captured content unchanged, reproduces the identical
  `batch_id` and completes via `AC-52`'s idempotent branch — a retry
  with genuinely changed underlying content correctly produces a
  different `batch_id`.

**Golden IDs, batch golden vector (task 6, task 9, task 16)**

- `AC-69`: Each of the four golden resource-ID vectors in §13.3 is
  independently recomputed by the implementation and matches exactly,
  including the two `db_path`-bearing vectors' order-independence.
- `AC-70`: The worked `batches/<batch_id>.json` example's `batch_id`
  (`rb_5cff7f222dce`, §12.3) is independently recomputed from its
  `CanonicalBatchJSON({"feature","results"})` body and matches exactly.

### 14.1 Exact counts (task 19: no false "exactly once" claims)

This PRD defines **70** `AC`-tagged clauses (`AC-1` through `AC-70`,
each an individually testable statement, no range-notation grouping).
The companion ADR's Test Matrix maps each of these 70 clauses to at
least one row; several clauses map to more than one row (e.g. both a
human-output and `--json`-output verification, or both a success and
a failure path for the same mechanism). The matrix therefore has
**more** rows than there are distinct clauses — this PRD does not
claim any clause is covered "exactly once."

## 15. Open Questions / Negative Consequences

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
- **Mandatory `table` forecloses whole-database diffing** (§6.2): a
  resource declaration that wants "everything that changed in this
  Dolt database" must enumerate every table it cares about; this is a
  deliberate v1 trade-off (favoring the hard PK-change error over
  silent omission, task 8) that a future PRD could revisit with an
  explicit multi-table/whole-db capability if the omission risk is
  judged acceptable with clearer documentation.

*(Resolved this revision, removed from Open Questions: `WORKING`/
`STAGED` support for `dolt_diff_summary` is now source-confirmed
exact-case-string-constant behavior (§6.2, citing `doltdb.go:51-52`);
`diff_type`'s value set is now confirmed as a closed 4-value enum
(§6.4, citing `table_deltas.go:46-49`). Neither is an open question in
rev-3.)

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
