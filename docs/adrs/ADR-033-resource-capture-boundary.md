# ADR-033 — Resource Capture Boundary (rev-1)

**Status**: Proposed — rev-1 (supersedes rev-0, writer commit `dd08157`,
adjudicated NEEDS REVISION at `89c8d79`; see `docs/supervisor/LOG.md`)

**Context**: `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
(rev-1, companion document — this ADR binds the decisions that PRD's
design depends on; read the PRD first for full rationale, this ADR
states the decisions themselves plus the Test Matrix).

**Related**: `ADR-027-capture-context-privacy-boundary.md` (D1–D5,
directly extended by D9 below), `ADR-030-multi-slug-reconcile-derivation-mode.md`
(`.git/**` exclusion precedent, extended by D6), `ADR-032-feature-unapply-state-boundary.md`
(fixed-struct JSON + `crypto/rand` ID precedent, reused by D3/D8),
`docs/state-of-the-art/storage-substrate-and-versioned-data.md` §3, §9
(tracked Dolt/substrate research — the normative citation for D7; see
Claims Audit C8 in the companion PRD for why untracked WP-006 is not
cited here)

---

## Rev-1 fold summary

Both rev-0 reviews (internal 7 HIGH + 1 MEDIUM, external 3 HIGH + 8
MEDIUM + 3 LOW — `docs/supervisor/LOG.md`) found rev-0's ADR described
committed-artifact behavior ADR-027 forbids, an under-specified
symlink model, a fabricated Dolt JSON contract, and false source
citations. This rev-1 rewrite resolves every finding; see the
companion PRD §0.1 Claims Audit for the full corrected-citation list
(not repeated here to avoid drift between the two documents — this
ADR cites the PRD's Claims Audit rows by ID, e.g. "C2," where relevant).

**Preserved from rev-0** (both reviews agreed these were sound): a
separate `resources.json` per feature, never inside the canonical
patch or unapply/lifecycle state; Dolt (or any external tool) is never
an authority over tpatch state and is not a build/runtime dependency;
replay/backward-compatibility is Git-only.

## Decision Drivers

- ADR-027 D1–D5's existing committed/local split and hard-failure
  redaction posture must extend cleanly to a new kind of captured
  content, not be reinterpreted.
- Any claim about Dolt's CLI surface must be verified against the
  primary `dolthub/dolt` source where possible (commit
  `59fb843bf6a4b653d7c8b6d997a603b10cf279d9`), cross-checked against
  the DoltHub reference, not invented.
- Any claim about an existing safety/validation primitive
  (`EnsureSafeRepoPath`, `IsPathIgnored`, `ExitCodeError` usage) must
  be verified against current source, not assumed from its name.

## Decision

### D1 — Scope & authority (reaffirmed, task A)

Resources are declared and captured per-feature, in a manifest wholly
separate from the canonical patch and from `apply-recipe.json`/unapply
state. A resource's tracked capture result (§ wire schema below) is
**always** hashes, counts, and structural classifications — **never**
raw file bytes, raw command stdout, or any verbatim scanned content
(PRD §8.1). Raw bytes, when they exist at all, live only under a new
gitignored local tree (D8) and only when explicitly opted into per
resource. This corrects rev-0's claim of "verbatim inheritance" into
tracked sidecars (task A) and rev-0's proposal to commit raw
snapshot/diff bodies, which directly contradicted ADR-027 D1's
committed/local split (PRD Claims Audit C7).

### D2 — Closed resource-kind set: three kinds, `generic-command` removed (task C)

`ignored-file`, `git-metadata`, `adapter-snapshot` (Dolt only). No
plugin mechanism, no user-declarable external-command execution of any
kind in v1. Rev-0's `generic-command` kind (argv-only execution of an
arbitrary user-declared external binary, gated by a user-declared
environment allowlist) is removed entirely: argv-only invocation
(never invoking a shell to interpret the command line) is not a
sandbox, and a user-declarable env-allowlist is not a meaningful
security boundary against an arbitrary binary the tool itself has no
opinion about. A future ADR is required before any sandboxed/consented
generic external-command capability is added. Rev-0's "impossible
`.git` exfiltration" acceptance criterion for `generic-command` is
removed along with the kind it depended on — no sandboxless argv
executor can actually guarantee no `.git`-adjacent read, so the
criterion was unverifiable by construction.

### D3 — Resource ID: canonical-JSON args encoding + golden vectors (task H)

Rev-0's `key=value\n`-joined `args` encoding was ambiguous: no escaping
of `=` or newline inside a key/value meant `{"a=b":"c"}` and
`{"a":"b=c"}` were not distinguishable, nor was a value containing a
literal newline distinguishable from a second declared argument.

**Canonical `args` encoding**: keys sorted byte-ascending; encoded as
`{"k1":"v1","k2":"v2",...}` with no whitespace; only `\` → `\\` and
`"` → `\"` escaped (deliberately not `encoding/json.Marshal`, which
additionally HTML-escapes `<`/`>`/`&` by default — this ADR uses a
small, fully-specified custom encoder instead so that behavior isn't
an implicit surprise); empty map encodes as `{}`; UTF-8 required, **no
NFC/NFD normalization** performed (documented v1 limitation); any NUL
or C0 control byte in any of `feature`/`kind`/`selector`/`adapter`/
`capability`/any `args` key or value is a validation error (exit 2) at
`add` time, rejected before it reaches the encoder.

**Full derivation**:

```
canonical_args := CanonicalArgsJSON(args)
payload        := feature + "\x00" + kind + "\x00" + selector +
                   "\x00" + adapter + "\x00" + capability +
                   "\x00" + canonical_args
digest          := SHA-256(UTF-8 bytes of payload)
resource_id     := "res_" + lowercase-hex(digest)[:12]
```

`adapter`/`capability` are empty strings (present, not omitted) in the
payload for kinds that don't use them — the payload always has exactly
six `\x00`-joined components.

**Golden vectors** (SHA-256, reproduced from the companion PRD §12.3,
byte-identical here):

| Vector | Inputs (`feature`, `kind`, `selector`, `adapter`, `capability`, `args`) | `resource_id` |
|---|---|---|
| 1 | `model-picker`, `git-metadata`, `head`, ``, ``, `{}` | `res_acc91dc23a8b` |
| 2 | `model-picker`, `adapter-snapshot`, `dolt:schema-diff:users`, `dolt`, `schema-diff`, `{"table":"users","from":"main","to":"HEAD"}` (declared `table, from, to` order) | `res_19b4675405e2` |
| 3 | Same as Vector 2, `args` declared `to, table, from` order | `res_19b4675405e2` (**identical** — proves order-independence) |
| 4 | `model-picker`, `ignored-file`, `config/local-secrets.env.template`, ``, ``, `{}` | `res_79f5ac5dca13` |

All four selectors are valid under D2's closed kinds and D5/D7's
closed selector grammars (`head` is one of D5's four views;
`dolt:schema-diff:users` matches D7's `dolt:<capability>:<table>`
shape; the fourth is an ordinary repo-relative path).

### D4 — `ignored-file`: ignored-AND-untracked dual gate, directory limits (task E)

`gitutil.IsPathIgnored` (`internal/gitutil/ignore.go`) invokes `git
check-ignore -q --no-index`. `--no-index` means Git does not consult
the index when answering, so an already-**tracked** file that also
matches a `.gitignore` pattern still reports "ignored" here (PRD Claims
Audit C3). This ADR requires a **second, mandatory** gate before an
`ignored-file` resource is accepted: the path must also be confirmed
**not currently tracked** (`git ls-files --error-unmatch <path>` or
equivalent — reporting "not tracked" required, "tracked" refused). Both
gates are re-checked at **every** `capture`, not only at `add` — a path
that was untracked-and-ignored at `add` time but has since been `git
add`ed is refused at the next `capture`.

Rev-0's additional acceptance of "untracked but not ignored" paths is
**removed** — that combination is ambiguous (a build artifact nobody
`.gitignore`d could become tracked with no signal to tpatch) and is no
longer accepted under any circumstance.

Directory-selector limits, checked at `add` and re-checked (task N:
snapshot-time, not just declaration-time) at every `capture`: **5 MiB**
per file, **20 MiB** total, **200 files** — exceeding any is a state
refusal (exit 3) naming the offending path/count, never a silent
truncation.

Binary detection: a NUL byte in the first 8 KiB classifies the content
`binary`; either way the full redaction scan (D9) still runs — it is
byte-pattern-based, not UTF-8-validity-dependent. No text
normalization is ever applied to `ignored-file` raw bytes (unlike D5/D7's
structural output, which is normalized before hashing) — the tracked
hash is over exact bytes read. A directory selector with local
raw-keeping enabled produces a local, untracked `manifest.json` (path,
size, hash per matched file) distinct from the tracked aggregate
(`file_count`/`total_bytes`/`combined_hash`).

### D5 — `git-metadata`: four closed views, no PII config keys (task F)

Rev-0's `config` view allowed `user.name`, `user.email`, wildcard
`core.*`, `remote.*.url`, `branch.*.merge` — all removed. Rev-1's four
closed views:

| View | Selector | Content |
|---|---|---|
| `head` | `head` | HEAD's symbolic-ref name, or `detached` + resolved OID. |
| `ref` | `ref:<refname>` (`^refs/(heads\|tags)/[A-Za-z0-9._/-]+$`) | Resolved OID of exactly that one ref (`git rev-parse --verify`). No bulk ref dump. |
| `index-entry` | `index-entry:<path>` | Mode/OID/stage for that one path (`git ls-files -s`); refused (exit 2) if zero or more than one matching entry. |
| `config` | `config:<key>`, `<key>` ∈ `{core.filemode, core.ignorecase, core.symlinks, extensions.objectformat}` | Single resolved value (`git config --get`); missing key recorded as `null`, not an error. |

An unlisted `config` key does not even parse as a valid selector (exit
2, shape error, not a runtime policy check). `ref`/`index-entry` values
are re-resolved fresh at every `capture`, never cached in
`resources.json`.

### D6 — Symlink/path safety: mandatory Lstat + EvalSymlinks + containment + `.git`-target refusal (task D)

`safety.EnsureSafeRepoPath`/`store.NormalizeClaimPath` are lexical only
(`filepath.Abs` + string-prefix containment; no `Lstat`, no
`filepath.EvalSymlinks` — PRD Claims Audit C2). This ADR adds, layered
on top (not replacing the existing lexical check), a mandatory gate run
at **both** `add` and **every** `capture`, for **every** path this
feature touches (selector, every directory descendant, `cwd`, the
resolved Dolt executable path):

1. `os.Lstat`; missing → `path-missing` (exit 3).
2. If a symlink, `filepath.EvalSymlinks`; unresolvable (dangling,
   permission error, cycle) → `symlink-unresolvable` (exit 3).
3. Re-run `safety.EnsureSafeRepoPath` containment against the
   **resolved** path; escapes repo root → `symlink-escapes-repo` (exit
   3).
4. Any traversed path component literally named `.git` (matching
   `gitutil.pathIsGitInternal`'s existing `.git`-boundary convention,
   `ADR-030` D3/D4 precedent) → `symlink-targets-git-internal` (exit
   3), refused regardless of containment.

For a directory selector, this four-step gate runs independently per
descendant file (task D: "every directory descendant"); the walk does
not follow symlinked subdirectories (each is refused in isolation by
steps 2–4, not silently descended into). The gate re-runs in full at
every `capture` — a selector that was a plain file at `add` and is a
symlink by the next `capture` is caught then, not grandfathered in.

### D7 — Dolt adapter protocol: verified syntax only, no fabricated JSON schema (task G)

Verified against the primary `dolthub/dolt` source at commit
`59fb843bf6a4b653d7c8b6d997a603b10cf279d9`: `go/cmd/dolt/commands/diff.go`
(synopsis `dolt diff [options] <commit> <commit> [tables...]`,
`--schema`/`--data` selection, `--result-format`/`-r` accepting
`tabular`/`sql`/`json`) and `go/cmd/dolt/commands/version.go`
(`dolt version [--verbose] [--feature]`, prints `dolt version
<version>`) — cross-checked against the DoltHub CLI reference
(`https://www.dolthub.com/docs/cli-reference/cli/`) for the remaining
flags below (`--filter`, `--name-only`) that the source check did not
separately confirm. No literal `dolt diff --json` flag exists in
either the source or the docs reference (JSON output is `-r
json`/`--result-format json`), and its per-row field schema is not
reliably documented across versions — this ADR never encodes it into
a tracked schema. No claim in this ADR depends on `dolt status
--porcelain`, which was not found in the source search.

**Probe**: `dolt version` (expects `dolt version \S+`), 5s timeout, 4
KiB cap. Four explicit outcomes: matched-pattern success (proceed,
record version); exit-0-unexpected-output (`adapter-probe-unexpected-output`,
exit 3); nonzero exit (`adapter-probe-failed`, exit 3, diagnostics
local-only); timeout (`adapter-probe-timeout`, exit 3, `SIGTERM` then
`SIGKILL` after 2s).

**Tracked classification** — `schema-diff`/`table-diff` each run three
invocations varying only `--filter`:

```
dolt diff --schema --name-only --filter=added    <from> <to> <table>
dolt diff --schema --name-only --filter=dropped  <from> <to> <table>
dolt diff --schema --name-only --filter=modified <from> <to> <table>
```

(`table-diff` substitutes `--data` for `--schema`.) Whichever
invocation returns non-empty `--name-only` output determines the
table's `added`/`removed`(dropped)/`changed`(modified) classification
— the only tracked fact recorded; never row content or row counts.

**Exact argv example** (Vector 2 above, `table=users, from=main,
to=HEAD`, `schema-diff`):

```
dolt diff --schema --name-only --filter=added    main HEAD users
dolt diff --schema --name-only --filter=dropped  main HEAD users
dolt diff --schema --name-only --filter=modified main HEAD users
```

Required args: `table`, `from`, `to` (all required, exit 2 if any
missing); a duplicate `--arg table=...` or any key outside `{table,
from, to}` is exit 2. `cwd` is the repo root; no other Dolt database
path is supported in v1. Timeout: 30s per invocation, `SIGTERM` then
`SIGKILL` after 2s, process-group termination. Output cap: 5 MiB
combined stdout+stderr, truncation recorded only in local diagnostics.
Environment passthrough: exactly `DOLT_ROOT_PATH`, `DOLT_CONFIG_PATH`,
`HOME`, `PATH` — no user-declarable env allowlist (D2 removes
`generic-command`, which was the only kind that had one). Any Dolt
invocation beyond the three `--name-only --filter=` calls (i.e. the
richer `--result-format json`/`tabular` output) runs only when local
raw-keeping is enabled, and its output is stored only as an opaque
local blob (hash + byte count in the tracked summary, never parsed).

### D8 — Local storage & transaction model: immutable batches, one atomic pointer rename (task I)

Raw content of any kind lives only under `.tpatch/local/resource-capture/<slug>/`,
which must be confirmed `.gitignore`d before the first write on a
given machine (ADR-027 D1's exact mandate — PRD Claims Audit C7),
refusing (`local-path-not-ignored`, exit 3) otherwise:

```
.tpatch/local/resource-capture/<slug>/
  current                       -- local pointer file (D8.3)
  batches/lb_<12 lowercase hex>/  -- one immutable batch
    meta.json                     -- local diagnostics only
    <resource_id>/{raw, files/<relpath>, manifest.json}
```

Batch IDs: 12 lowercase-hex from `crypto/rand` (mirroring the `ua_`
attempt-ID precedent, `ADR-032` D3/Implementation Notes item 8).

**Commit protocol**: (1) write full batch content under
`batches/.tmp-lb_<id>/`; (2) `fsync` each file, then the temp
directory; (3) atomic `rename(2)` to `batches/lb_<id>/`; (4) only after
(3) succeeds, atomically update `current` (`.tmp-current` → `fsync` →
`rename` over `current`). Advisory lock: `O_CREATE|O_EXCL` on
`.lock`; a concurrent `capture` finding it held refuses immediately
(`capture-in-progress`, exit 3, no waiting/queuing).

**Crash-window analysis**:

| Crash point | Resulting state | Recovery |
|---|---|---|
| Mid-write in `.tmp-lb_<id>/` | Orphaned temp dir; `current` unaffected | Inert; prunable later; no capture ever reads a `.tmp-` prefixed directory. |
| After step 3, before step 4 | Fully valid, unpointed-to batch | Harmless — never read without going through `current`; re-running `capture` retries. |
| After step 4, before tracked `summary.json` write | `current` points at a valid batch not yet reflected tracked-side | Re-running `capture`/`record --resources` recomputes and republishes; no dedicated recovery verb needed (task I). |

No tracked artifact of any kind is ever written on failure — a failed
staging attempt writes only local diagnostics (`meta.json`) inside a
batch that (because step 4 was never reached) is never pointed to by
`current`, so it is never a tracked "failure record" (task I, task N —
this removes rev-0's contradictory numbered-append-history-vs-no-tracked-failure-envelope
tension by removing the numbered-history model entirely, D8.4 below).

**D8.4 — History model, `remove`/`clear`**: there is no numbered
append-only tracked history. Tracked state is exactly one
`summary.json` per resource, overwritten on each successful `capture`
(Git's own commit history of that file is the audit trail, as for
every other tracked tpatch artifact). Local batches are the
append-only, immutable history — never overwritten, prunable only
manually/out-of-band (accepted cost, unbounded local growth until
pruned). `remove`/`clear` delete **only** the tracked declaration and
tracked summary — ordinary sequential file deletion, **not** claimed
atomic across the tracked and local trees (rev-1 removes rev-0's
"atomic cross-tree clean delete" claim, task I) — and do **not** touch
local batch history at all.

### D9 — Privacy & redaction: six closed hard-refusal classes (task B)

`internal/cli/session_redaction.go` is unexported
(`redactSessionForCommit`/`forbiddenContentClasses`), shaped around
`store.SessionObservation`, and applies drop-the-line-and-continue
semantics across 10 heuristic classes lacking dedicated PEM/OpenSSH-key,
DB-URL, or email/PII patterns (PRD Claims Audit C4). This ADR does
**not** change that existing mechanism's behavior for its existing
surface — it requires the implementation cluster to **extract** its
reusable byte-pattern matchers (secret-like-string prefixes,
absolute-home-path) into a new, exported, content-agnostic
`internal/redact.Scan(content []byte) []string`, consumed by both the
existing session-redaction call site (unchanged policy) and this
PRD's new resource-capture call site, which applies a **hard-refusal**
policy — any match on any of the six classes refuses the entire
capture (`redaction-refused`, exit 3), never a partial scrub, matching
ADR-027 D3's existing hard-failure posture.

**Six closed classes**: (1) PEM/OpenSSH private keys; (2) DB/connection
URLs (known schemes with embedded userinfo, plus the generalized
`://user:pass@host` shape for any scheme — this closes rev-0's
Git-remote-specific masking, applying it universally as an enforced
scanner rule rather than a one-off, PRD Claims Audit / task M); (3)
emails/PII; (4) credential assignments (`secret|token|password|api[_-]?key|
access[_-]?key|private[_-]?key|client[_-]?secret` + `[:=]` + value); (5)
bearer/token/key patterns (reusing existing `sk-`/`ghp_`/`gho_`/`ghu_`/
`AKIA`/`xox[baprs]-`/bearer-prefix matchers); (6) home absolute paths
(reusing the existing matcher).

Scanned before any write, on: the selector, every `args` value, every
resolved `git-metadata` value, `ignored-file` raw content (byte-scanned
regardless of binary/text classification), and the local-only raw
Dolt diagnostic blob. Local raw bodies require explicit `keep_local`
opt-in and are written with owner-only permissions (`0700`
directories, `0600` files, set explicitly via `os.Chmod`, not relied
on from umask) — and remain untracked in every case regardless of
`keep_local`.

### D10 — `record --resources`: stage-then-publish, two separate atomic domains (task J)

Rev-0 overloaded `record --resources --dry-run` onto the command that
performs Git-side canonical-patch capture; this ADR removes `--dry-run`
from `record` entirely, moving preview/resource-only/retry flows to
the standalone `feature resource capture --dry-run`/`diff` verbs.

`record <slug> --resources` runs: (1) **zero-resource preflight** —
refuse immediately (`no-resources-declared`, exit 1) if the feature
declares none, before touching Git; (2) **stage** every declared
resource privately (full `capture` pipeline through D8's batch-commit
step, but `current` is not yet updated and no tracked `summary.json`
is yet written) — an all-or-nothing staging failure across any
resource does not stop step 3; (3) **Git-side capture** runs
completely unmodified, unaffected by step 2's outcome (this ADR does
not change `record`'s existing capture-mode mutex group, empty-patch
handling, or `--auto`/range resolution); (4) **publish gated on Git
success**: if Git failed, the staged batch is discarded, `current` is
never updated, and `record`'s existing unmodified failure behavior
propagates; if Git succeeded and staging also succeeded, publish
(update `current`, write each resource's tracked `summary.json`) —
fast, no adapter re-execution; if Git succeeded but staging failed, or
publish itself fails, the result is `resource-domain-incomplete` (exit
1) — Git-side capture is confirmed complete and unaffected, the staged
batch (if any) is preserved and named in the error, and the message
gives an explicit retry command (`feature resource capture <slug>` or
`... --resource <id>`).

This states honestly that **Git capture and resource-pointer
publication are two separate atomic domains** (task J) — `record`
guarantees the canonical patch is correct regardless of resource
outcome, and separately guarantees the resource domain is atomic when
it does complete, but never that both complete together.

**Exit codes** (new per-command contract; `SPEC.md`'s existing,
explicit convention: "Exit codes are per-command contracts, not a
single global enum" — `tpatch verify` already has its own exit-2
meaning distinct from `reject`/`reopen`/`doctor`/`c1`/`feature_deps`/
`reconcile_check_applied`, all six of which already use `ExitCodeError`,
PRD Claims Audit C5 — this is one more contract in that family, not a
reuse of any one existing command's codes):

| Code | `feature resource {add,list,remove,clear,capture,diff}` | `record --resources` |
|---|---|---|
| `0` | Success | Success |
| `1` | Internal error | Internal error, `no-resources-declared`, `resource-domain-incomplete` |
| `2` | Validation (kind/adapter/capability/view shape, unknown/duplicate `--arg`, malformed selector, ambiguous/unresolvable index-entry stage) | n/a (record's existing codes unmodified) |
| `3` | State/policy (`ignored`/tracked-gate failure, disallowed config key, any symlink refusal, size/count limit, `redaction-refused`, `adapter-missing`/`adapter-probe-*`, `local-path-not-ignored`, `capture-in-progress`) | Same set applies to staging; a staging refusal surfaces as `resource-domain-incomplete` (Git succeeded) or a discarded-batch diagnostic (Git failed, record's own exit code) |

## Wire Schema Appendix (task K, byte-identical to companion PRD §11)

Two distinct JSON serializations: **canonical args JSON** (D3, sorted,
minimally-escaped, hash input only) vs. the **file wire format** below
(ordinary `encoding/json` on a fixed-field struct, 2-space indent,
trailing newline, declared-field order — the `ADR-032` Implementation
Notes item 8 precedent). Arrays are always `[]` not `null` and never
omitted; inapplicable fields are present with an explicit zero value
(`""`, `{}`, or `null`), never omitted.

### `resources.json`

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

### `resources/<id>/summary.json` — one example per kind

**`adapter-snapshot`**:

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

**`git-metadata`** (`head` view):

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

**`ignored-file`** (single file):

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

`raw.present` is `true` iff `keep_local` was `true` **and** a local
raw companion was actually written (never for `--dry-run`); when
`false`, `raw.hash`/`raw.bytes` are `null`, never omitted. The
`current` pointer (`.tpatch/local/resource-capture/<slug>/current`) is
**local, not tracked** — a batch-ID string followed by one `\n`, no
JSON.

## Implementation Notes (for Cluster H')

1. Extract `internal/redact.Scan([]byte) []string` per D9, reusing
   existing `session_redaction.go` matchers where their pattern is
   directly applicable (secret-like prefixes, home-path pattern);
   author the three new classes (PEM/OpenSSH, DB-URL, email/PII) fresh.
2. `.gitignore` needs a new `/.tpatch/local/` line — this ADR
   documents the requirement (D8's ADR-027-D1-mandated pre-write check
   depends on it existing) but does not itself edit `.gitignore`.
3. The symlink-safety gate (D6) should be a single shared helper
   consumed by all three resource kinds and by the Dolt executable
   resolution (D7/D2.1) — do not fork three copies of the same
   four-step check.
4. `record`'s existing capture-mode dispatch code should treat
   `--resources` as a fully independent, late-checked flag — it should
   not need to know about resource kinds/adapters at all; it only
   needs to know whether the resource-staging step (run by
   `internal/resourcecapture` or similar, TBD package name) reported
   success before `record` decides to publish.
5. Golden vectors (D3) should become table-driven Go tests directly
   asserting `resource_id` output — do not leave them as documentation
   examples only.

## Negative Consequences Summary

1. Local batch history is unbounded until manually pruned (no
   auto-pruning or `purge-local` verb in v1).
2. `record --resources` always stages before Git, even when Git will
   most likely succeed and the staged work is simply published — a
   deliberate trade for stronger tracked-state consistency over raw
   throughput.
3. No Unicode NFC normalization in the canonical `args` encoding —
   visually-identical selectors in different normalization forms
   produce different `resource_id`s (documented, not silently wrong).
4. Richer Dolt output (`-r json`/`tabular`) is never parsed into
   tracked fields — no structured per-row diff API in v1.

## Test Matrix (task L)

Cites the companion PRD's `AC-<n>` tags (§13 there). Several ACs map
to more than one row (e.g. a human-output row and a `--json`-output
row, or one row per named outcome in a multi-outcome table) — this
matrix does **not** claim a 1:1 row-to-clause mapping anywhere, and
the row count below is computed, not asserted "exactly once."

| # | AC(s) | Category | Scenario | Expected |
|---|---|---|---|---|
| 1 | AC-1 | Surface | `feature resource add` on nonexistent slug, human output | `"no such feature: %s"`, exit 1 |
| 2 | AC-1 | Surface | Same, `--json` | Same message in a JSON error envelope, exit 1 |
| 3 | AC-2 | Surface | `feature resource list` human vs `--json` on the same feature | Both report identical resource set/fields |
| 4 | AC-3 | Surface | `feature resource remove <slug> <full-id>` | Resolves and removes exactly that resource |
| 5 | AC-3 | Surface | `feature resource remove <slug> <unambiguous-prefix>` | Resolves to the one matching resource |
| 6 | AC-3 | Surface | `feature resource remove <slug> <ambiguous-prefix>` | Exit 2, lists matching candidates |
| 7 | AC-3 | Surface | `feature resource remove <slug> <no-match-prefix>` | Exit 2 |
| 8 | AC-4 | Surface | Selector/`--arg` value containing `; \| \`` `$(...)` `&&` | Accepted as a literal string, no shell interpretation, still redaction-scanned |
| 9 | AC-5 | Dolt | `schema-diff`, table only in `--filter=added` output | Classified `added` |
| 10 | AC-5 | Dolt | `schema-diff`, table only in `--filter=dropped` output | Classified `removed` |
| 11 | AC-5 | Dolt | `schema-diff`, table only in `--filter=modified` output | Classified `changed` |
| 12 | AC-5 | Dolt | `table-diff` analogous three-way classification | Same added/removed/changed semantics via `--data --name-only` |
| 13 | AC-6 | Dolt | `dolt` not on `PATH` | `adapter-missing`, exit 3 |
| 14 | AC-7 | Dolt | Probe returns `dolt version 1.42.3` | Proceed; version recorded |
| 15 | AC-7 | Dolt | Probe exits 0 with non-matching output | `adapter-probe-unexpected-output`, exit 3 |
| 16 | AC-7 | Dolt | Probe exits nonzero | `adapter-probe-failed`, exit 3, diagnostics local-only |
| 17 | AC-7 | Dolt | Probe exceeds 5s | `adapter-probe-timeout`, exit 3, SIGTERM→SIGKILL |
| 18 | AC-8 | Dolt | `--arg table=a --arg table=b` | Exit 2, duplicate key |
| 19 | AC-8 | Dolt | `--arg foo=bar` (unknown key) | Exit 2 |
| 20 | AC-8 | Dolt | Missing `--arg to` | Exit 2 |
| 21 | AC-9 | Dolt | Resolved `dolt` executable path lands inside repo tree | `adapter-executable-unsafe`, exit 3, before probe runs |
| 22 | AC-10 | Capture | `capture <slug>` (no `--resource`) | Captures every declared resource |
| 23 | AC-10 | Capture | `capture <slug> --resource <id>` | Captures only that resource |
| 24 | AC-11 | Capture | `capture <slug> --dry-run` | No local batch directory created |
| 25 | AC-11 | Capture | Same | No tracked `summary.json` written/modified |
| 26 | AC-12 | Diff | `diff <slug>` | Never spawns an adapter process, never reads `.tpatch/local/` |
| 27 | AC-12 | Diff | `diff <slug>` before any capture | `"no capture yet"`, exit 0 |
| 28 | AC-13 | Remove | `remove <slug> <id>` | Local batch tree byte-identical before/after |
| 29 | AC-13 | Clear | `clear <slug>` | Same invariant across all removed resources |
| 30 | AC-14 | Privacy | Value containing a PEM private-key block | `redaction-refused`, exit 3, no partial write |
| 31 | AC-14 | Privacy | Value containing `postgres://u:p@host/db` | `redaction-refused`, exit 3 |
| 32 | AC-14 | Privacy | Value containing an email address | `redaction-refused`, exit 3 |
| 33 | AC-14 | Privacy | Value containing `api_key: "abcdef123456"` | `redaction-refused`, exit 3 |
| 34 | AC-14 | Privacy | Value containing `Bearer ghp_xxxxxxxxxxxxxxxxxxxx` | `redaction-refused`, exit 3 |
| 35 | AC-14 | Privacy | Value containing `/Users/alice/...` | `redaction-refused`, exit 3 |
| 36 | AC-15 | Privacy | Value matching none of the six classes | Captures normally (negative control) |
| 37 | AC-16 | Symlink | Plain in-repo file selector | Accepted |
| 38 | AC-17 | Symlink | Symlink → in-repo file | Accepted (symlink target, not existence, is what's checked) |
| 39 | AC-18 | Symlink | Symlink → path outside repo root | `symlink-escapes-repo`, exit 3 |
| 40 | AC-19 | Symlink | Symlink → nonexistent target | `symlink-unresolvable`, exit 3 |
| 41 | AC-20 | Symlink | Symlink/descendant resolves through a `.git` path component | `symlink-targets-git-internal`, exit 3 |
| 42 | AC-21 | Symlink | `cwd` itself is a symlink escaping the repo | `symlink-escapes-repo`, exit 3, before resource-specific work begins |
| 43 | AC-22 | Ignored | Ignored-and-untracked file | Accepted |
| 44 | AC-23 | Ignored | Tracked file that also matches `.gitignore` | Refused (tracked-file gate), exit 3, at both `add` and `capture` |
| 45 | AC-24 | Ignored | Untracked, not `.gitignore`d | Refused, exit 3 |
| 46 | AC-25 | Limits | Directory selector with one file > 5 MiB | Exit 3, names offending path |
| 47 | AC-26 | Limits | Directory selector totaling > 20 MiB | Exit 3 |
| 48 | AC-27 | Limits | Directory selector matching > 200 files | Exit 3 |
| 49 | AC-28 | First capture | First capture vs. second capture of same resource | Byte-identical `summary.json` schema shape (differing only in `captured_at`/`local_batch_id`/adapter-classification-if-changed) |
| 50 | AC-29 | Binary | `ignored-file` content with a NUL byte in first 8 KiB | Classified `binary`; full redaction scan still runs |
| 51 | AC-30 | Multi-file | Directory selector, `keep_local=true` | Local `manifest.json` per-file entries match tracked aggregate fields |
| 52 | AC-31 | Crash | Simulated crash leaves orphaned `.tmp-lb_*` | Next `capture` succeeds normally; orphan untouched |
| 53 | AC-32 | Crash | Simulated crash leaves committed-but-unpointed `lb_<id>/` | Not surfaced by `list`/`diff`; next `capture` succeeds, updates `current` to its own new batch |
| 54 | AC-33 | Concurrency | Two concurrent `capture` invocations, same slug | Second refuses `capture-in-progress`, exit 3, no blocking |
| 55 | AC-34 | Record | `record --resources` on feature with zero declared resources | `no-resources-declared`, exit 1, before any Git invocation |
| 56 | AC-35 | Record | Resource staging fails, Git-side capture succeeds | `resource-domain-incomplete`, exit 1, exact recovery-command text, Git patch confirmed correct |
| 57 | AC-36 | Record | Resource staging fails, Git-side capture also fails | Only `record`'s existing Git-failure behavior surfaces; staged batch discarded, no double-reporting |
| 58 | AC-37 | Record | Staging succeeds, Git-side capture succeeds | `current` and every resource's tracked `summary.json` update together, not partially |
| 59 | AC-38 | Dry-run | `feature resource capture <slug> --dry-run` on a slug with no Git-side activity at all | Runs and reports correctly, independent of any Git-side state |
| 60 | AC-39 | Retry | Re-run `feature resource capture <slug>` after a prior failure | Succeeds with no dedicated recovery command |
| 61 | AC-40 | Golden ID | Recompute Vector 1 | `res_acc91dc23a8b` |
| 62 | AC-40 | Golden ID | Recompute Vector 2 | `res_19b4675405e2` |
| 63 | AC-41 | Golden ID | Recompute Vector 3 (reordered args) | `res_19b4675405e2` (identical to Vector 2) |
| 64 | AC-40 | Golden ID | Recompute Vector 4 | `res_79f5ac5dca13` |
| 65 | AC-10 | Surface | `feature resource capture <slug> --json` | JSON envelope reports per-resource outcome consistent with human output |
| 66 | AC-12 | Surface | `feature resource diff <slug> --json` | JSON envelope's `result` block matches the tracked `summary.json` verbatim |

**Row count: 66.** **Distinct `AC`-tagged clauses covered: 41** (every
`AC-1` through `AC-41` appears in at least one row above; rows 1–2,
9–12, 14–17, 18–20, 24–25, 26–27, 28–29, 30–35, 37–42, 61–64, and 65–66
each expand a single multi-outcome AC or a human/JSON pair into more
than one row — this matrix does not claim any AC is covered "exactly
once"). No row is circular with respect to the AC it verifies (no row
cites its own expected outcome as its own evidence).
