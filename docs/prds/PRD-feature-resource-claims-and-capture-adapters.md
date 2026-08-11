# PRD — Feature Resource Claims & Capture Adapters (rev-5)

**Status**: Draft — rev-5 (supersedes rev-4, writer commits `ceda294`
(rev-4) + `b7ddccb` (rev-4 platform-citation addendum), adjudicated
NEEDS REVISION → REV-5 DISPATCHED at `07eab8e`; see
`docs/supervisor/LOG.md` → Cluster H rev-0 through rev-4 adjudications)

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

## 0. Rev-5 Fold Summary (read this first)

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

### 0.1 Claims Audit (rev-4 + rev-5 additions)

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
| C13 | `internal/workflow/session_ignore.go`'s `EnsureLocalIgnoreContract(repoRoot, resolvedPath)` verifies the path is inside the worktree and that `gitutil.IsPathIgnored` (`--no-index`) reports it ignored; it does **not** independently verify the path is untracked | `internal/workflow/session_ignore.go:138-175` (`EnsureLocalIgnoreContract` body) | New for rev-2: §10.3 reuses this exact function for the ephemeral-scratch root (task 7's "do not invent a second ignore mechanism") but layers the same tracked-file gate used for `ignored-file` selectors (§5.1) on top, since `EnsureLocalIgnoreContract` alone does not close the `--no-index` gap for the scratch root either. |
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
| C28 | `//go:build unix` is a real, documented Go build constraint that matches every POSIX-family `GOOS` value Go supports (including `aix`, `solaris`, `illumos`, `dragonfly`), not only `linux`/`darwin` | Go standard library/toolchain documentation for build constraints (the `unix` build tag is defined as the union of all Unix-like `GOOS` values the toolchain recognizes, a strictly larger set than this project's tested platforms) | rev-4 **imprecision**: this project's actual CI matrix (`.github/workflows/ci.yml:18-25`) tests only `ubuntu-latest`/`macos-latest`; a `unix`-tagged lock file would silently compile (and claim to work) on untested POSIX targets. §7.2 now uses the exact `//go:build linux \|\| darwin` constraint, with `//go:build !linux && !darwin` as the explicit, narrower fallback. |
| C29 | `flock(2)`'s advisory-lock guarantee is documented as applying to the local kernel's view of an open file description; POSIX and Linux/BSD manual pages do not guarantee `flock` semantics are honored identically by every network/shared filesystem client (NFS in particular has historically had inconsistent or client-local-only `flock` emulation depending on protocol version and mount options) | `flock(2)` manual page ("Advisory locks... ", the "NOTES" section on NFS behavior varying by kernel/mount configuration is a widely-documented caveat of this syscall, not specific to this project) | Grounds §7.2's new filesystem-contract preflight (rev-5): this design does not claim `flock` provides real mutual exclusion on every mount a `.tpatch/` directory could live on, and refuses known-risky/unrecognized filesystem types via `statfs` before creating the lock file at all. |
| C30 | `os/exec.Cmd`'s `StdoutPipe`/`StderrPipe` return `io.ReadCloser`s that must be fully drained (or the child can block writing to a full pipe); `cmd.Wait()` requires all reads from these pipes to complete before it is called, per the documented contract | Go standard library `os/exec` package documentation for `Cmd.StdoutPipe`/`Cmd.StderrPipe` ("It is thus incorrect to call Wait before all reads from the pipe have completed") | Grounds §6.4/§8.1's redesigned output-cap mechanism (rev-5): continuing to drain both pipes to completion after a cap-triggered kill (rather than abandoning the read) is required by this documented contract, not merely a defensive nicety. |

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
both **unchanged** by rev-4 — nothing in this fold touches the
hash-derivation functions themselves, only the lock/idempotency-
comparison/read-semantics mechanisms around them. All four resource-ID
golden vectors and the worked batch example's content-addressed
`batch_id` (§12.3, §13.3) therefore remain byte-identical to rev-3:

| Vector | Feature | Kind | Selector | Adapter | Capability | Args (declaration order) | `resource_id` |
|---|---|---|---|---|---|---|---|
| 1 | `model-picker` | `git-metadata` | `head` | *(none)* | *(none)* | `{}` | `res_acc91dc23a8b` (unchanged) |
| 2 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `db_path=data/dolt-db, table=users, from=main, to=HEAD` | `res_cf8e47e6564b` (unchanged from rev-3) |
| 3 | `model-picker` | `adapter-snapshot` | `dolt:diff-summary:users` | `dolt` | `diff-summary` | `to=HEAD, db_path=data/dolt-db, table=users, from=main` (reordered) | `res_cf8e47e6564b` (unchanged — order-independence) |
| 4 | `model-picker` | `ignored-file` | `config/local-secrets.env.template` | *(none)* | *(none)* | `{}` | `res_79f5ac5dca13` (unchanged) |

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
  that ever writes tracked (§7.3) state. `--dry-run` (task 7) runs the
  **entire** pipeline for real — lock acquisition, orphan sweep,
  ignored-file/Git-metadata reads into bounded memory, the real Dolt
  invocation inside a real scratch `HOME` (§6.4), redaction scanning —
  and reports exactly what would be published, but guarantees **no
  tracked writes and no persistent local writes**: no
  `batches/<id>.json` or `current.json` write is ever attempted, and
  every ephemeral `es_<id>/` directory it creates (including a real
  Dolt `dolt-home/`) is removed before it returns, exactly as a real
  capture's cleanup step does. This is a narrower, more honest claim
  than rev-3's "zero local writes survive past the invocation": rev-4's
  `.lock` file (§7.2) is a **persistent**, ignored/untracked control
  file that is never deleted after its first creation for a given
  slug — `--dry-run` acquires and may create this file exactly like
  any other mutator, and it is explicitly not part of the "nothing
  persists" guarantee, only the tracked artifacts and the per-invocation
  ephemeral scratch tree are.
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

`add`/`list`/`remove`/`clear` behave exactly as `feature claim`'s
quartet does (same `"no such feature: %s"` refusal shape, same
`--json` convention), except `add` additionally computes and persists
`resource_id` (§13) and rejects duplicates (same `selector`+`kind`+
`adapter`+`capability`+canonical `args` tuple already declared for
that feature) as a validation error (exit 2). `add`/`remove`/`clear`
now acquire the same per-slug lock `capture` uses (§7.2, task 1)
before mutating `resources.json` — before the lock file itself is ever
created, all three (like `capture`/`record --resources`) first run the
local-ignore + untracked gate on the scratch root (§10.3, task 8).
`add`/`remove`/`clear` never perform the scratch orphan sweep
themselves (only `capture`/`record --resources` do, since only those
two ever create ephemeral scratch content, §7.1) and **never** write
or rewrite `current.json` or any `batches/<id>.json` file — those
remain untouched by every verb except `capture`/`record --resources`.
`remove`/`clear` only ever mutate `resources.json` — a resource's
`current.json` entry, if one exists from a prior capture, simply
becomes orphaned (harmless, permanent history, exactly like a batch
file that outlives its resource's declaration); `list` never surfaces
it because `list` iterates `resources.json`'s declared entries, not
`current.json`'s index. `list` never acquires the lock (§7.2, task 1):
it is a pure read of whatever `resources.json`/`current.json` content
is currently visible on disk, which — because both files are always
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
  or any raw content. `add`/`remove`/`clear` are the **only** verbs
  that ever write this file, under the per-slug lock (§7.2).
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
  transaction, and `add`/`remove`/`clear` never write to
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
combined hash is `SHA-256` over each matched file's canonical
**`path\x00mode\x00hash`** tuple (`path` repo-relative, `mode` the
same 6-digit octal-string convention as `index-entry`'s `mode`
sourced from a plain `os.Stat`, `hash` the file's own `SHA-256` hex
digest), one tuple per matched file, sorted by `path`, tuples
themselves `\x00`-joined — so a **chmod-only** change (identical
bytes, different permission bits) now changes `combined_hash`, not
just a content or file-set change; rev-4's formula omitted `mode`
from the hash input entirely even though `files[]` already carried a
per-file `mode` (§12.2), so a permission-only change was silently
invisible to `combined_hash` and to `diff` — this is corrected).

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
a non-null empty string, not a null. The schema declaration establishes
the **column type**; see §6.3 (C25) for the separate, stronger claim
that the code path emitting each row also produces a **native Go
`bool`** for these two columns.

**Refs, `WORKING`/`STAGED` explicitly refused in v1 (C19, rev-5
correction)**: `dsess/session.go:1022-1031`'s `ResolveRootForRef`
does accept the exact, case-sensitive literal strings `WORKING` and
`STAGED` (`doltdb.go:51-52`) in addition to any ordinary commit-ish —
this remains a **true, source-confirmed fact** about upstream Dolt's
capability, unchanged from rev-4's citation. What rev-5 corrects is
this PRD's own design choice: rev-4 stated `from`/`to` **accept**
either literal and pass it through; this design now **explicitly
refuses** both (case-insensitively, so `Working`/`staged`/etc. are
also caught rather than silently falling through to a failed
commit-spec resolution), returning `dolt-argument-refused` (exit 2)
before the child process is ever started. The reason is the working
tree/staged index, unlike any committed ref, is itself gated by
Dolt's own `dolt_ignore` table (a Dolt-level analog of `.gitignore`)
— a table matched by `dolt_ignore` can be silently absent from
`WORKING`/`STAGED`'s row set the same way an ignored file is absent
from a Git working tree listing, and since this PRD already makes
`table` mandatory specifically so a primary-key-set change is a hard
error rather than a silent omission (C20 below), allowing a ref value
that reintroduces a different, independent silent-omission path would
undercut that guarantee for no compensating benefit; v1's committed-
ref-only scope (branch names, tags, full/abbreviated commit hashes)
side-steps `dolt_ignore` entirely, since ignore-table membership is a
working-set-only concept that a committed diff never consults.
Accepting `WORKING`/`STAGED` remains a plausible **future**
capability (e.g. a v2 opt-in that explicitly documents the
`dolt_ignore` interaction rather than silently inheriting it) but is
out of scope for this PRD, not merely deferred by omission — a caller
that declares either literal today gets a validation-time refusal, not
a passthrough.

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
table that exists in **neither** `from` nor `to` yields **zero rows**,
not an error: `findMatchingDelta` (called from the 3-arg branch,
`dolt_diff_summary.go:301`) returns a delta with no `FromTable`/
`ToTable`/`FromRootObject`/`ToRootObject` populated for a name unknown
to either root, and `getSummaryForDelta`'s own early `return nil, nil`
for exactly that all-nil case (`:346-350`) means the single-table
call's `summs` slice — which only appends when the returned summary is
non-nil (`:313-318`) — stays empty. This is a distinct, third outcome
from C20's hard error and is the "first capture" / "table never
existed at `from`" case: the tracked `result.tables` array is simply
empty (`[]`, never `null`, task 14) — no special-cased schema.

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
exists solely as a caller-side filter value for a different function
and this design's fixed `dolt_diff_summary` query neither emits it nor
needs to recognize it as a possible row value. For `"dropped"`,
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
| Captured output cap | 5 MiB combined stdout+stderr, enforced as a **refusal, never a truncation** (rev-5 correction — rev-4's "output beyond the cap is truncated" contradicted its own "zero pre-scan persistence"/fail-closed framing, since silently truncating and proceeding is itself a form of trusting unbounded input up to the point of truncation). The adapter uses `cmd.StdoutPipe()`/`cmd.StderrPipe()` (never `cmd.Stdout`/`cmd.Stderr` set to a `*bytes.Buffer`, which has no built-in bound) with two concurrent goroutines draining each pipe into a **shared** cap-plus-one budget (a single `int64` counter both readers atomically decrement, so stdout and stderr together, not each independently, are bounded by the 5 MiB total) — task 2's "zero pre-scan persistence," never written to a scratch file first, §7.1/§8. The **instant** the shared budget's `limit+1`th byte is actually read from either pipe, the adapter sends `SIGTERM` then (after the existing 2-second grace) `SIGKILL` to the child's **process group** (the same termination mechanism as a timeout), continues draining both pipes to completion (so the child is never left blocked writing to a full, unread pipe while being killed) and `cmd.Wait()`s for it, then refuses the whole invocation with `resource-limit-exceeded` (exit 3) — no partial/truncated output is ever handed to the JSON parser (§6.3) or scanned for redaction (§8), and no tracked artifact reflects a truncated result. **stdout and stderr are captured and bounded identically, but are never merged for parsing purposes**: only the stdout buffer is ever handed to §6.3's JSON parser; the stderr buffer exists solely to be scanned for redaction (§8.3) and, on a non-zero exit or a `dolt-query-error`, to populate the local, ephemeral diagnostic (§7.5) — it is never itself parsed as JSON and never influences whether stdout parses successfully. |
| Environment | **Not** inherited from the invoking process (task 2's "no inherited credentials"). A fresh, minimal environment is constructed: `HOME=<scratch-home>` and `DOLT_ROOT_PATH=<scratch-home>` pointing at a directory created fresh under this invocation's ephemeral scratch tree (§7.1, `0700`, created before the child process starts so Dolt may write its own ephemeral config/state there if it chooses to — this is not a network or version call, just process-local state under an isolated `HOME`); `PATH` is **not** set at all (the adapter is invoked by its already-resolved absolute path, §6.1, so `PATH` lookup is never needed mid-invocation). No other variable is passed through. |
| Termination | Process-group termination (the child and any of its own children) on timeout **or** on a cap-plus-one overflow (identical mechanism, §6.4 above), to avoid orphaned Dolt subprocesses. |

A concrete, fully-specified argv/SQL example for Vector 2 (§0.3) —
`db_path=data/dolt-db, table=users, from=main, to=HEAD`:

```
cwd:  <repo-root>/data/dolt-db
argv: /usr/local/bin/dolt sql -r json -q "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change FROM dolt_diff_summary('main', 'HEAD', 'users') ORDER BY from_table_name, to_table_name;"
```

(the absolute path shown, `/usr/local/bin/dolt`, is illustrative only
— it is never the tracked value, §6.1.)

## 7. Ephemeral Scratch, Locking, and the Single Publication Point (task 1, task 2, task 5, task 6, task 7, task 9)

### 7.1 Ephemeral scratch layout and lifecycle (task 1, task 2, task 6, task 9)

`.tpatch/local/` is the existing gitignored local root (`LocalIgnoreRule`,
`internal/workflow/session_ignore.go:18`). Before this invocation's
first write anywhere under it — including the `.lock` file itself,
before its very first creation for a given slug — `add`/`remove`/
`clear`/`capture`/`record --resources` (task 8: **every** mutator, not
only `capture`/`record --resources`) reuse
`workflow.EnsureLocalIgnoreContract` (unchanged reuse from rev-2/rev-3;
this is a deliberate reuse of the **existing** local-ignore mechanism,
not a second, parallel one) to confirm the local root itself is both
ignored and untracked — a local root that is somehow tracked or not
ignored is refused (`local-root-not-ignored`/`local-path-tracked`,
exit 3) before any scratch content, including the lock file, is
created, matching §10.3's row for this exact case.

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
invocation beyond in-process memory — it holds whatever ephemeral
config Dolt itself chooses to write under an isolated `HOME`/
`DOLT_ROOT_PATH`, never a captured ignored-file byte or a copy of
Dolt's own query output. Every directory under `es_<id>/` is created
`0700` and every file `0600` **at creation** (`os.Mkdir`/`os.OpenFile`
with the final mode passed directly — never a separate `os.Chmod`
after the fact, task 8). `es_<id>/` is removed (`os.RemoveAll`,
best-effort) as the last step of the invocation on **both** the
success and failure paths; a removal failure is a local diagnostic
(§7.5), not a hard failure.

`add`/`remove`/`clear` (task 1) acquire the same per-slug `flock`
(§7.2) before touching `resources.json`, but never create `es_<id>/`
and never perform either orphan sweep below — only `capture`/
`record --resources` ever create scratch content (local or tracked),
so only they are responsible for cleaning it up.

`--dry-run` (§3, task 7) still acquires the lock and may still create
a real `es_<id>/dolt-home/` if the targeted resource set includes a
Dolt capability (a real Dolt invocation needs a real, isolated `HOME`
regardless of `--dry-run`) — but writes no tracked batch/pointer,
**runs neither the local nor the tracked orphan sweep** (both sweeps
below are exclusively `capture`/`record --resources` behavior, and
`--dry-run` never reaches §7.3's publication sequence where the
tracked sweep is invoked), and removes `es_<id>/` at the end exactly
as a real capture does; the persistent `.lock` file, once created, is
not removed for `--dry-run` either, exactly as it is not for a real
capture — "no tracked writes and no persistent local writes" (§3) is
about the tracked tree and the ephemeral scratch tree, not the lock,
which by design is never ephemeral.

**Orphan cleanup** (task 1, task 6): a startup sweep runs **only**
after the current invocation has itself acquired the live `flock`
(§7.2) — never before acquiring it, never from `add`/`remove`/`clear`
(which acquire the lock but never sweep, task 6's "never from add
outside lock," extended to `remove`/`clear` too, since v1 has no
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
corrected diagram above. Because rev-4 has no lock-acquisition-time
temporary state at all (§7.2 — `flock` is acquired directly on the
persistent `.lock` file with no intermediate name), there is no third
"lock temp/quarantine" sweep to run; sweeping under the lock still
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
that slug: `add`, `remove`, `clear`, `capture`, `record --resources`
all acquire it before their first write; `list`/`diff` never acquire
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
(task 2, new in rev-5): `flock(2)`'s advisory-lock semantics are only
guaranteed by the kernel for genuinely local filesystems; several
POSIX-compliant systems document `flock` as unsupported, only
partially supported, or supported solely through NFS lock-manager
(`NLM`)/`nfs4` client behavior that this design does not verify or
depend on for network/shared/clustered filesystems (NFS, network SMB
mounts, some FUSE-backed or overlay filesystems, and other non-local
mounts) — using `flock` there could silently fail to serialize
concurrent invocations from **different hosts**, which this design
never claims to support in the first place (v1's lock is a
single-host, same-machine-process serialization primitive only, never
a distributed/cross-client lock). To fail closed rather than silently
degrade, `Acquire` performs a `statfs`-family preflight on
`.tpatch/local/resource-scratch/<slug>/` (the directory that will
contain `.lock`) **before** the `O_CREATE`/`flock` sequence above:

- **Linux**: `unix.Statfs(path, &buf)`, inspecting `buf.Type` against a
  small, explicit **allowlist** of local-filesystem magic numbers
  (`EXT4_SUPER_MAGIC`, `XFS_SUPER_MAGIC`, `BTRFS_SUPER_MAGIC`,
  `TMPFS_MAGIC`, `APFS_SUPER_MAGIC` where applicable via a Linux VM/
  container mounting one) — a **known-denied** type (`NFS_SUPER_MAGIC`,
  `CIFS_MAGIC_NUMBER`, `SMB2_MAGIC_NUMBER`, `FUSE_SUPER_MAGIC`) or any
  **unrecognized** type both refuse identically.
- **Darwin**: `unix.Statfs(path, &buf)`, inspecting `buf.Fstypename`
  (a short string, e.g. `"apfs"`/`"hfs"`) against a small allowlist
  (`"apfs"`, `"hfs"`) — a known-denied value (`"nfs"`, `"smbfs"`,
  `"webdav"`) or any unrecognized value both refuse identically.
- Refusal outcome, either platform: `resource-lock-filesystem-unsupported`
  (**exit 3**), returned before `.lock` is ever opened or created —
  this is a distinct error class from `resource-lock-unsupported`
  (which is about the *build tag*/`GOOS`, not the *mount*) so a caller
  can distinguish "your OS isn't supported at all" from "your OS is
  supported, but this particular directory is on a filesystem this
  design refuses to trust `flock` on."

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
   - **Different bytes**: a `batch_id` collision on genuinely
     different content is a fatal integrity error
     (`batch-id-collision`, exit 3) — refuse rather than silently
     overwrite a distinct historical batch; this is expected to be
     unreachable in practice (`SHA-256` collision on the hash-input
     encoding) and exists purely as a defensive, fail-closed guard, not
     an anticipated real outcome.
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
stdout/stderr are captured via `cmd.StdoutPipe()`/`cmd.StderrPipe()`
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
   against the same held descriptor from step 1 — a mismatch is
   reported as `db-path-identity-changed` in local diagnostics (§7.5)
   alongside the capture's own result, as a **detection**, not a
   prevention, signal: by the time this check runs, the child process
   has already used whatever directory it actually resolved at its own
   open time, so this can only flag that something suspicious happened
   during the process's lifetime, not undo it or prove the child
   itself was affected.

This provides **detection of, not protection against**, a mid-invocation
`db_path` directory swap by a local concurrent attacker who can win a
narrow race against this process — it is honestly documented as a
residual risk in Negative Consequences (ADR-033 D6), not claimed as a
closed sandbox. The general ancestor-symlink/final-component TOCTOU
residual described above for ordinarily-opened paths applies to
`db_path`'s own validation walk identically; this subsection documents
the **additional** residual specific to `cmd.Dir` being pathname-bound
rather than descriptor-bound.

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

### 10.3 Local-ignore-root reuse (task 1, task 8)

Before the first write to `.tpatch/local/resource-scratch/<slug>/` for
a given slug — which now means before the persistent `.lock` file's
own first creation (§7.1, §7.2), not merely before scratch-content
creation — **every** mutating verb (`add`, `remove`, `clear`,
`capture`, `record --resources`, task 8's "every mutator checks
EnsureLocalIgnoreContract... before creating lock/scratch") calls the
**existing** `workflow.EnsureLocalIgnoreContract(repoRoot,
resourceScratchRoot)` (`internal/workflow/session_ignore.go:138`) —
reused exactly as-is, not re-invented — which verifies Git is
available, the path is inside the worktree, and
`gitutil.IsPathIgnored` reports it ignored; `IsPathIgnored`'s own
`check-ignore` invocation is precisely the deliberate pathname
exception documented in §10.1 (it does not, and cannot, use
`--literal-pathspecs`). Because `EnsureLocalIgnoreContract` alone does
not close the `--no-index` gap for the scratch root any more than it
does for an `ignored-file` selector (C13), this design layers the
**same** tracked-file gate from §5.1/§10.2 on top: `git
--literal-pathspecs ls-files --error-unmatch -- .tpatch/local/` must
also report untracked. Either check failing to hold is
`local-path-not-ignored`/`local-path-tracked` (exit 3) — refused
before any scratch content, **including the lock file itself**, is
created, exactly mirroring ADR-027 D1's ignored-before-first-write
mandate. This gate now runs identically for `remove`/`clear` (which
previously, in rev-3, only acquired the lock without first re-running
this check — corrected here since rev-4 makes the lock file itself
the very first piece of scratch state any mutator creates) — §14 has
explicit `AC`/matrix rows for `remove`/`clear` exercising this exact
case, not just `add`/`capture`. This PRD does not invent a second
ignore mechanism — it reuses the one that exists and adds only the
missing tracked-file half, the same addition already made for
`ignored-file` selectors in §5.1.

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

**Exit codes** (restated for rev-4's refusal names):

| Code | `feature resource {add,list,remove,clear,capture,diff}` | `record --resources` |
|---|---|---|
| `0` | Success (including `diff` reporting "no capture yet") | Success |
| `1` | Internal error; `tracked-batch-missing` (§4.1) | Same, plus `no-resources-declared` and `resource-domain-incomplete` |
| `2` | Validation: bad kind/adapter/capability/view, unknown/duplicate/missing `--arg` (including missing `db_path`/`table`), `NUL`/control byte/backslash/`..` in a Dolt arg, missing index entry at `add`, `table` mismatch between selector and declared field | n/a (unmodified) |
| `3` | State/policy refusal: `not-ignored`, `tracked-and-ignored`, `git-ignore-check-error`, `git-ls-files-error`, any `symlink-component-refused`/`path-missing`/`path-replaced-during-open`, any size/count limit, `resource-limit-exceeded`, `redaction-refused`, `adapter-missing`/`adapter-executable-in-repo`/`adapter-executable-replaced`, `dolt-query-error`, `dolt-json-parse-error`, `local-root-not-ignored`/`local-path-tracked`, `capture-in-progress`, `resource-lock-unsupported`, `resource-lock-filesystem-unsupported`, `batch-id-collision`, `index-entry-missing` | Same set applies to staging (§11 step 2); surfaces as `resource-domain-incomplete` (exit 1) if Git succeeded, or as record's own existing exit code (with the discarded-batch diagnostic) if Git failed |

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
no index entry by definition. `combined_hash` is `SHA-256` over each
entry's `path\x00mode\x00raw_sha256` tuple, sorted by `path`, tuples
themselves `\x00`-joined (§5.1, rev-5: `mode` is now part of the hash
input, not just a displayed `files[]` field — a chmod-only change now
changes `combined_hash`) — `files[]` is additional per-file detail,
not a replacement for that aggregate.)

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
  "batch_id": "rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7",
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

`batch_id` is **content-addressed** (§7.3 step 2), not
random: `rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7` is the actual, computed `SHA-256`-derived ID
for this exact `{feature, results}` body (`results` sorted by
`resource_id` byte-ascending, per `CanonicalBatchJSON`, §12 intro) —
re-running `CanonicalBatchJSON` over this exact JSON (**excluding**
the `batch_id` field itself and using the compact, unindented
canonical encoding — the **hash-input bytes**, §7.3 step 2/step 3)
reproduces this exact `batch_id`, and this revision's validation pass
independently confirmed this by reimplementing `CanonicalBatchJSON` in
a standalone script and recomputing the exact value shown above.
`results` is sorted by `resource_id`, byte-ascending — `res_79f5ac5dca13`
< `res_acc91dc23a8b` < `res_cf8e47e6564b`. The JSON rendering shown
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
`adapter-snapshot`. The example `oid` above is an ordinary,
valid-shaped 64-hex-character SHA-256 Git object ID (illustrative, not
the well-known empty-tree hash `4b825dc642cb6eb9a060e54bf8d69288fbee4904`,
which §12.2's detached-`HEAD` example uses instead, task 10).

### 12.4 `current.json` (the tracked pointer)

```json
{
  "latest_batch_id": "rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7",
  "resources": [
    { "resource_id": "res_79f5ac5dca13", "batch_id": "rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7" },
    { "resource_id": "res_acc91dc23a8b", "batch_id": "rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7" },
    { "resource_id": "res_cf8e47e6564b", "batch_id": "rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7" }
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
rev-3 change). Vectors 2/3 are **recomputed** (as of rev-3) because
`db_path` (§6.2) is now a mandatory declared field and therefore part
of the hashed `args` set; recomputed independently via a standalone
Python script implementing §13.1/§13.2 verbatim as part of that
revision's validation pass. **Rev-4 changes nothing about §13.1/§13.2**
(§0.3) — all four vectors above remain byte-identical to rev-3; this
revision's validation pass re-ran the same standalone script
unmodified and reconfirmed all four values, since rev-4's changes are
entirely about lock/publish/read mechanics, never about the
`resource_id` derivation itself.

## 14. Acceptance Criteria (task 14)

Clause-level, `AC-<n>` tagged. Each `AC` is one testable clause;
`ADR-033-resource-capture-boundary.md`'s Test Matrix cites these tags
directly. Renumbered from rev-3: rev-3's PID/temp-directory/quarantine
lock clauses (old `AC-37`–`AC-43`) are **removed** entirely (that
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
- `AC-18`: A resolved Dolt executable whose device/inode/size/mtime
  differ immediately after invocation vs. immediately before is
  refused (`adapter-executable-replaced`) and its result is discarded.

**Zero pre-scan persistence; privacy; bounded reads (task 2)**

- `AC-19`: An `ignored-file` selector's content is read into an
  in-process buffer and never written to any scratch or other file
  before scanning/hashing completes.
- `AC-20`: Dolt's stdout/stderr are captured into an in-process,
  bounded buffer and never redirected to or copied into a scratch
  file before parsing/scanning completes.
- `AC-21`: A value matching any of the six redaction classes refuses
  the entire invocation (`redaction-refused`), with no partial batch
  written for any resource in that invocation, even unaffected ones,
  and with no unredacted byte having been written to any file at any
  point before the refusal.
- `AC-22`: No tracked file anywhere contains a wall-clock timestamp
  field.
- `AC-23`: `feature resource diff` on an `ignored-file` resource
  **reads current file content** through the same bounded in-memory
  scanner `capture` uses (not a stat-only/metadata-only check) to
  recompute `size_bytes`/`hash`/`file_count`/`total_bytes`/
  `combined_hash`, and reports exactly which of those (or file-set
  membership) changed — never a textual line-level diff.
- `AC-24`: A file/stream that grows beyond the declared limit between
  an initial `Stat`/length check and the actual read is still refused
  (`resource-limit-exceeded`), because the enforcement is an actual
  cap-plus-one read (reads `limit+1` bytes and refuses if that many
  were read), never a stat-only/pre-read-length check that trusts the
  claimed size.
- `AC-25`: A directory `capture`/`diff` is documented and tested as a
  **sequential**, per-file read — not a single atomic point-in-time
  directory snapshot — and this residual is stated in §15/Negative
  Consequences, not claimed closed.

**Descriptor-identity path gate; `db_path`/`cmd.Dir` residual (task 3)**

- `AC-26`: A selector (`ignored-file` or Dolt `db_path`) whose ancestor
  directory (not the final component) is a symlink is refused
  (`symlink-component-refused`), regardless of where that symlink
  points.
- `AC-27`: A selector replaced by a symlink at the final component
  between the walk and the open is refused via `O_NOFOLLOW`/`ELOOP`.
- `AC-28`: A selector whose underlying file is replaced (different
  device/inode) between the walk and the open is refused
  (`path-replaced-during-open`), detected via `os.SameFile` on the
  **open file descriptor**'s `FileInfo`, not a second pathname
  `Lstat`.
- `AC-29`: A dangling ancestor (missing path component) is refused
  (`path-missing`).
- `AC-30`: This gate re-runs independently for every descendant file
  of a directory selector, both at `add` and at every `capture`.
- `AC-31`: The Dolt executable path uses the separate, opposite-
  direction policy (§6.1/§9.2) and is never subject to the ancestor-
  symlink-refusal rule that applies to `AC-26`–`AC-30`.
- `AC-32`: `db_path`'s gate is re-run a second time immediately before
  `cmd.Start()` — a **fresh** pathname `Lstat`/resolve, not a reuse of
  the initial gate's cached `FileInfo` — compared via `os.SameFile`
  against the directory descriptor already held open from the initial
  gate; a **third**, independently fresh pathname resolution runs
  after the Dolt child process exits and is again compared
  (`os.SameFile`) against the same held descriptor, surfacing a
  detected mismatch as `db-path-identity-changed` in local diagnostics
  — this is verified as a **detection**, not a prevention, mechanism
  (a test simulating a post-gate, pre-exit directory swap confirms the
  mismatch is *flagged*, not silently ignored, without claiming the
  swap itself was prevented), and a second test confirms the check is
  genuinely pathname-vs-descriptor, not descriptor-vs-descriptor (a
  test that only re-`fstat`s the same, unchanged descriptor twice and
  asserts equality would pass trivially and detect nothing — `AC-32`
  requires the comparison input to be a freshly-obtained `FileInfo`
  from a new `Lstat` of the `db_path` string each time).

**`check-ignore` fix, ignored/tracked Git gates (task 1)**

- `AC-33`: `check-ignore` is invoked without `--literal-pathspecs` (no
  such option exists for it) — verified by asserting the invocation
  never includes that flag.
- `AC-34`: `check-ignore` exit `1` (not ignored) and exit `>1` (fatal)
  produce distinct refusal reasons, neither treated as "ignored."
- `AC-35`: A selector whose first byte is `:` is passed to
  `check-ignore` with a `./` prefix, and resolves to the identical
  on-disk path as the unprefixed form would if `check-ignore` could
  accept it literally.
- `AC-36`: `*`/`?`/`[]` characters in a `check-ignore` pathname
  argument never trigger wildcard/glob matching.
- `AC-37`: `ls-files --error-unmatch` exit `0` (tracked) and any
  non-standard exit/stderr shape produce distinct refusal reasons, and
  every such call uses `--literal-pathspecs`.
- `AC-38`: A selector is refused unless it is **both** ignored (via
  `AC-34`) **and** untracked (via `AC-37`) — recheck at `add` and at
  every `capture`.

**Local ignore contract, tracked-root gate, all mutators (task 8)**

- `AC-39`: The scratch root's ignored status is verified via the
  existing `EnsureLocalIgnoreContract`, not a second, parallel ignore
  mechanism.
- `AC-40`: The scratch root is also verified untracked via the
  `AC-37`-style `ls-files --error-unmatch` gate; either check failing
  refuses (`local-root-not-ignored`/`local-path-tracked`) before any
  scratch content — including the persistent `.lock` file's own
  first-ever creation — is created.
- `AC-41`: `remove`/`clear` run the identical local-ignore/untracked
  gate as `add`/`capture`/`record --resources`, before their own
  `.lock` acquisition — not just the mutators that create scratch
  content (correcting rev-3, which did not explicitly extend this gate
  to `remove`/`clear`).

**Kernel `flock` lock semantics (task 1)**

- `AC-42`: `.lock` is opened `O_CREATE|O_RDWR, 0600` and
  `flock(LOCK_EX|LOCK_NB)`'d; success proceeds immediately, and
  `EWOULDBLOCK`/`EAGAIN` refuses immediately (`capture-in-progress`),
  with no polling, wait, or configurable timeout.
- `AC-43`: The `.lock` file is never removed, renamed, or replaced by
  any code path across repeated invocations — verified by asserting
  the file's device+inode identity is unchanged across multiple
  successive invocations for the same slug.
- `AC-44`: A process holding the `flock` that is killed (simulating a
  crash) releases the lock at the kernel level with no code of this
  design's own running — the next invocation acquires it successfully
  immediately, with no manual reclaim/quarantine step of any kind.
- `AC-45`: All five mutating verbs (`add`, `remove`, `clear`,
  `capture`, `record --resources`) acquire the identical per-slug
  `flock` before their first write; `list`/`diff` never acquire it.
- `AC-46`: On a build tagged `!linux && !darwin` (the exact fallback
  constraint, not a generic `!unix`), every mutating verb returns
  `resource-lock-unsupported` (exit 3) deterministically, never
  silently proceeding unlocked.
- `AC-47`: Two invocations racing to acquire `.lock` for the same slug
  resolve with exactly one succeeding and the other refusing
  immediately (`capture-in-progress`) — contention is instantaneous,
  never a queued/blocking wait.

**Permissions, scratch/orphan cleanup (task 6, task 8)**

- `AC-48`: Every ephemeral scratch directory (`es_<id>/`, `dolt-home/`)
  is created `0700` and every file `0600` at creation (never via a
  separate `chmod` after a looser-permission create); the persistent
  `.lock` file is created `0600` at its one-time creation and never
  `chmod`'d afterward.
- `AC-49`: An orphaned ephemeral scratch directory (`es_*`) or tracked
  temp file (`batches/*.tmp-*.json`/`.tmp-current.json`) left by a
  simulated crash is swept **only after** the sweeping invocation has
  itself acquired the live `flock`, never before, and the local/tracked
  sweeps are exercised as two independently verified enumerations.
- `AC-50`: `add`/`remove`/`clear` never perform the orphan sweep
  (verified by asserting no `es_*`/tracked-temp removal occurs during
  an `add`/`remove`/`clear` invocation even when such orphans are
  present).

**Content-addressed single publication point (task 5, task 6)**

- `AC-51`: A successful multi-resource `capture` writes exactly one
  new `batches/<id>.json` file (unless an identical one already
  exists, `AC-53`) and rewrites `current.json` exactly once.
- `AC-52`: `batch_id` is deterministically derived from
  `CanonicalBatchJSON({"feature","results"})` (the hash-input encoding,
  which excludes `batch_id` itself) — recomputing it from the same
  batch content reproduces the identical `batch_id`.
- `AC-53`: A retry that reproduces identical batch content re-encodes
  the **complete file-wire bytes** (including `batch_id`, with real
  on-disk indentation) and finds them identical to the existing
  `batches/<batch_id>.json`, skipping directly to pointer publication
  without rewriting the batch file — verified by asserting the
  comparison is against file-wire bytes, not the hash-input bytes
  (which would never match and would incorrectly trigger
  `AC-54`, rev-3's bug).
- `AC-54`: A retry that computes the identical `batch_id` for
  **different** file-wire content (a hash collision) is refused
  (`batch-id-collision`), never silently overwritten.
- `AC-55`: A crash simulated between the batch rename and the
  `current.json` rename leaves a permanently orphaned, harmless batch
  file that no subsequent `list`/`diff` ever surfaces, and a re-run
  recomputes the identical `batch_id` and proceeds via `AC-53`.
- `AC-56`: A crash simulated during either temp-file write (before its
  rename) leaves only a temp artifact matching the exact naming in
  §7.1/§7.3, swept at the next invocation's start (`AC-49`), with no
  effect on the last successfully committed `current.json`.
- `AC-57`: `remove`/`clear` never write, rename, or delete
  `current.json` **or** any `batches/<id>.json` file at all — only
  `resources.json`, under the per-slug `flock` (`AC-45`) — correcting
  rev-3, in which `remove`/`clear` pruned `current.json`'s live index.
- `AC-58`: A resource removed from `resources.json` while `current.json`
  still references it leaves a harmless orphaned pointer entry that
  `list` never surfaces (since `list` iterates `resources.json`'s
  declared entries, never `current.json`'s index directly) — verified
  by asserting the orphaned entry is not garbage-collected and does
  not appear in `list`'s output.
- `AC-59`: `current.json` is the only file `list`/`diff` read to
  resolve a resource's latest result — neither ever scans `batches/`
  directly.

**Git metadata / tagged variants (task 10)**

- `AC-60`: `head`'s `symbolic_ref` is `null` if and only if `detached`
  is `true`.
- `AC-61`: The `config` view refuses any key outside the exact
  four-key allowlist.
- `AC-62`: An `index-entry` selector queried with a path containing
  pathspec-magic characters resolves to the literal path under
  `--literal-pathspecs`.
- `AC-63`: A directory `ignored-file` result includes a stable,
  `path`-sorted `files[]` array with `{path, raw_sha256, byte_count,
  mode}` per entry, in addition to the aggregate
  `file_count`/`total_bytes`/`combined_hash` fields.
- `AC-64`: Every kind/view's tagged `result` shape (§12.2) is exercised
  by at least one test: `head` attached, `head` detached, `ref`,
  `index-entry`, `config` (set and unset), `ignored-file` single file,
  `ignored-file` directory, `adapter-snapshot`.

**`--dry-run`, transaction / `record --resources` (task 7, task 11)**

- `AC-65`: `feature resource capture <slug> --dry-run` writes **zero
  tracked files** (`resources.json` is not touched beyond its
  pre-existing content; no `batches/`/`current.json` write occurs) and
  leaves **zero persistent resource data locally** — but its
  persistent `.lock` file, if newly created by this invocation, is not
  removed (that is expected, not a leak; only ephemeral scratch, e.g.
  `es_<id>/dolt-home/`, is removed) — the AC therefore asserts "no
  tracked writes and no persistent resource-data local writes," not
  "zero filesystem writes."
- `AC-66`: `record --resources` on a feature with zero declared
  resources refuses (`no-resources-declared`) before any Git
  invocation and before lock acquisition.
- `AC-67`: A resource-staging failure combined with Git-side success
  produces `resource-domain-incomplete` with the exact recovery-command
  message, while the Git-side canonical patch is confirmed present and
  correct.
- `AC-68`: A resource-staging failure combined with Git-side failure
  discards the staged (never-written) candidate batch and surfaces
  only record's existing Git-failure behavior.
- `AC-69`: A successful stage and successful Git-side capture publish
  the batch and pointer atomically, verified by asserting both
  `batches/<id>.json` and `current.json` reflect the same invocation
  together, never partially.
- `AC-70`: Re-running `feature resource capture <slug>` (or
  `record --resources`) after a publish-step failure, with the
  underlying captured content unchanged, reproduces the identical
  `batch_id` and completes via `AC-53`'s idempotent branch — a retry
  with genuinely changed underlying content correctly produces a
  different `batch_id`.

**Golden IDs, batch golden vector (task 12)**

- `AC-71`: Each of the four golden resource-ID vectors in §13.3 is
  independently recomputed by the implementation and matches exactly,
  including the two `db_path`-bearing vectors' order-independence.
- `AC-72`: The worked `batches/<batch_id>.json` example's `batch_id`
  (`rb_5cff7f222dce2ed9c342375cdba813dd6d57d5e58695ad3fd02df49a78e7efa7`, §12.3) is independently recomputed from its
  `CanonicalBatchJSON({"feature","results"})` **hash-input** body
  (excluding `batch_id` itself) and matches exactly.

**Rev-5 additions: build tags, filesystem contract, output-cap
refusal, `WORKING`/`STAGED` refusal, batch ordering, directory mode**

- `AC-73`: The lock implementation file carries exactly `//go:build
  linux || darwin`, and its fallback file carries exactly `//go:build
  !linux && !darwin` — verified by asserting the literal build-tag
  comment text in each file, not merely its behavior.
- `AC-74`: A `.tpatch/local/resource-scratch/<slug>/` root created on
  a filesystem type outside this design's allowlist (simulated via a
  fake/stubbed `statfs` result, since CI cannot mount a real network
  filesystem) refuses `resource-lock-filesystem-unsupported` (exit 3)
  before `.lock` is ever created — distinct from `AC-46`'s
  build-tag-based refusal, verified by asserting the two error
  strings never collide and the filesystem check runs strictly before
  the lock-file-creation code path.
- `AC-75`: A simulated Dolt child process that writes more than 5 MiB
  combined stdout+stderr is killed (process group `SIGTERM` then
  `SIGKILL`) the instant the shared cap-plus-one budget is exceeded,
  the invocation refuses with `resource-limit-exceeded` (exit 3), and
  no partial/truncated stdout is ever handed to the JSON parser —
  verified by asserting the parser is never invoked at all for an
  over-cap invocation (not invoked-then-discarded).
- `AC-76`: `dolt_diff_summary`'s stdout and stderr share **one**
  cap-plus-one budget (not two independent 5 MiB budgets) —
  verified by a test where stdout alone is under 5 MiB but
  stdout+stderr combined exceeds it, and the invocation still refuses.
- `AC-77`: A directory `ignored-file` capture followed by a
  chmod-only change (identical file set and byte content, changed
  permission bits on one file) produces a different `combined_hash`
  on the next `capture`/`diff`, and `diff` reports the specific
  `files[]` entry's `mode` as the differing field, with `hash`/
  `byte_count` unchanged for that entry.
- `AC-78`: A resource captured with content `A`, then content `B`,
  then content `A` again (three `capture` invocations) results in
  exactly **two** distinct `batches/<id>.json` files on disk (one for
  `A`'s content, one for `B`'s), not three — the third invocation's
  `current.json` rewrite repoints at the already-existing batch for
  `A` without creating a new batch file, verified by asserting the
  directory listing of `batches/` is unchanged in file count after the
  third invocation.

### 14.1 Exact counts (task 14: no false "exactly once" claims)

This PRD defines **78** `AC`-tagged clauses (`AC-1` through `AC-78`,
each an individually testable statement, no range-notation grouping).
This is a **net +6** change from rev-4's 72: rev-5 adds six new
clauses and removes none — `AC-73` (exact build-tag text), `AC-74`
(filesystem-preflight refusal, distinct error from build-tag refusal),
`AC-75` (output-cap-as-refusal via process-group kill, never
truncation), `AC-76` (stdout/stderr share one budget, not two), `AC-77`
(chmod-only directory change is diff-distinguishable via `mode` in the
hash input), and `AC-78` (an A→B→A content sequence produces exactly
two batch files, not three, verifying the no-chronology/unordered-set
design) — `72 + 6 = 78`. Two existing clauses were rewritten in place
without changing the total count: `AC-10`/`AC-11` flipped from
"`WORKING`/`STAGED` accepted" to "`WORKING`/`STAGED` refused," and
`AC-32`/`AC-46` were reworded for the pathname-vs-descriptor honesty
fix and the exact `!linux && !darwin` fallback tag, respectively — none
of these in-place rewrites add or remove a clause. The companion ADR's
Test Matrix maps each of these 78 clauses to at least one row; several
clauses map to more than one row (e.g. both a human-output and
`--json`-output verification, or both a success and a failure path for
the same mechanism). The matrix therefore has **more** rows than there
are distinct clauses — this PRD does not claim any clause is covered
"exactly once."

## 15. Open Questions / Negative Consequences

- **Ancestor-directory TOCTOU** (§9.1) is a documented residual risk,
  not fully closed by the Go standard library alone; a future PRD
  could revisit this if a portable `openat2`-equivalent becomes
  available in stdlib.
- **`db_path`/`cmd.Dir` pathname-bound residual** (§9.1, task 3): Go's
  `os/exec.Cmd.Dir` takes a pathname, not a descriptor, so a
  sufficiently well-timed local concurrent attacker can in principle
  swap the validated `db_path` directory between this design's final
  re-check and the moment Dolt's own process actually resolves its
  cwd; this design narrows the window (re-check immediately before
  `cmd.Start()`, hold an open directory descriptor across the child's
  lifetime, re-check identity after exit) but provides **detection**,
  not **prevention** — stated honestly here and in ADR-033 D6, not
  claimed as a closed sandbox.
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
  during the child's own execution (§9.1, AC-32).
- Narrowed the platform build-tag contract from the broad `unix` tag
  to an exact `linux || darwin` (real implementation) /
  `!linux && !darwin` (fallback) split, matching this project's actual
  tested CI matrix (`ubuntu-latest`/`macos-latest`) rather than every
  POSIX-family `GOOS` value `unix` also covers (§7.2, AC-46, AC-73).
- Added a `statfs`-based filesystem preflight for the lock, refusing
  `resource-lock-filesystem-unsupported` (exit 3, distinct from the
  build-tag-based `resource-lock-unsupported`) on network/shared or
  unrecognized filesystem types, with an explicit per-OS allow/deny
  list and a documented "operator-configured local-only precondition"
  fallback if the allowlist proves too brittle (§7.2, AC-74).
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
  diagnostics only (§6.4, §8.1, AC-75, AC-76).
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
  §12.3, AC-72).
- Added an explicit "batches are an unordered, content-addressed set —
  not a chronology" clarification: `batch_id` names distinct content,
  not a position in a sequence; an A→B→A capture sequence produces
  exactly two batch files, not three, and `current.json` is the sole
  authority for "what is current now" — event-level chronology is
  explicitly out of scope for v1 (§4, §7.3, AC-78).
- Folded directory `mode` into the per-file `combined_hash` input
  (`path\x00mode\x00hash`, sorted by path) and into `diff`'s
  comparison, so a chmod-only change (identical content, changed
  permission bits) is now distinguishable from a content change
  (§5.1, §12.2, AC-77).
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
  clauses (`AC-73` exact build-tag text, `AC-74` filesystem-preflight
  refusal, `AC-75` output-cap-as-refusal via process-group kill,
  `AC-76` shared stdout+stderr budget, `AC-77` chmod-only
  diff-distinguishability, `AC-78` A→B→A produces exactly two batch
  files) and two clauses rewritten in place without changing the total
  (`AC-10`/`AC-11` WORKING/STAGED refusal, `AC-32`/`AC-46` pathname-
  vs-descriptor and exact build-tag wording) (§14, §14.1).
- Line counts, `§`-cross-references, and every `docs/handoff/CURRENT.md`
  attestation are updated to their final, post-edit values as the last
  step of this revision, per this fold's explicit tracking requirement
  (§0, CURRENT.md).
