# PRD - Active Feature Session
**Status**: Accepted (v0.12.0 Wave γ landed; PRD flipped after implementer rev-0)
**Date**: 2026-07-29
**Owner**: Core
**Cluster**: Capture and metadata foundation (research-roadmap §3.1)
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-027](../adrs/ADR-027-capture-context-privacy-boundary.md) (locks D1 F3 downstream path)
**Blocks**: `PRD-record-context-summary`, `PRD-agent-event-log`, `PRD-ide-capture-hooks`, `PRD-git-hook-capture-guards`, `ADR-capture-metadata-branch`

## Related
- [ADR-027 — Capture Context Privacy Boundary](../adrs/ADR-027-capture-context-privacy-boundary.md)
- [State-of-the-art research roadmap](../state-of-the-art/research-roadmap.md)
- [Patch capture and agent context research brief](../state-of-the-art/patch-capture-context-research-brief.md)
- [Patch capture prior art and hook models](../state-of-the-art/patch-capture-prior-art-and-hooks.md)
- [PRD-feature-file-claims](./PRD-feature-file-claims.md)
- [PRD-record-capture-modes](./PRD-record-capture-modes.md)
- [PRD-tpatch-doctor](./PRD-tpatch-doctor.md) as §6 acceptance precedent

## 0. Meta

### 0.1 Paper-only status
This PRD is **Proposed**. It changes no code, schema, CLI behavior, shipped asset
text, CHANGELOG entry, release artifact, or migration state. Acceptance requires
the supervisor review plus the external review pair named in the stream dispatch.

### 0.2 ADR-027 F3 lock
ADR-027 D1 deliberately left the active-session local-buffer path soft. The
binding text this PRD closes is:

> **Local private buffer lane**: high-granularity session, agent, IDE, and hook
> observations live outside the tracked worktree by default, under
> `.git/tpatch/capture/` or an equivalent OS user-cache location. If a future
> PRD chooses any worktree path such as `.tpatch/local/capture/`, the writer
> MUST verify that path is ignored before the first write and MUST refuse rather
> than risk accidental commit.

This PRD locks Option A for active feature sessions:

- local buffer root: `.tpatch/local/capture/`;
- local session manifest: `.tpatch/local/capture/<slug>/<cs_id>/session.json`;
- local observation log: `.tpatch/local/capture/<slug>/<cs_id>/observations.jsonl`;
- committed summary directory: `.tpatch/features/<slug>/artifacts/context/`;
- committed summary file: `.tpatch/features/<slug>/artifacts/context/<ctx_id>.json`.

These strings are Rule 8 display-string contracts after acceptance.

### 0.3 Claims audit
| Claim | Evidence |
|---|---|
| ADR-027 D1 permits `.tpatch/local/capture/` only with ignored-before-write refusal. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:88-103`. |
| ADR-027 D2-D3 forbid raw context and make redaction a write precondition. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:119-170`. |
| ADR-027 D6 reserves `ctx_<12hex>`, `cs_<12hex>`, `ce_<12hex>` and forbids wall-clock ID input. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:226-251`. |
| ADR-027 D7 makes active sessions and local buffer reads default-off. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:253-270`. |
| ADR-027 D9-D10 require cross-feature isolation and define the provider-assisted carve-out. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:300-343`. |
| Research roadmap queues `PRD-active-feature-session` after privacy/claims/capture-mode work and before richer agent/IDE/hook capture. | `docs/state-of-the-art/research-roadmap.md:99-113`, `docs/state-of-the-art/research-roadmap.md:149-171`. |
| Production exposes `tpatch analyze <slug>`, `tpatch record <slug>`, and `tpatch reconcile [slug...]`; `tpatch session` is new. | `internal/cli/cobra.go:438-470`, `internal/cli/cobra.go:942-960`, `internal/cli/cobra.go:1766-1977`, `internal/cli/cobra.go:57-80`. |
| Root `--path` is the user-defined persistent flag inherited by subcommands. | `internal/cli/cobra.go:54-55`; `docs/supervisor/LOG.md:2028-2048`. |
| `tpatch init` creates `.tpatch/features`, `.tpatch/steering`, `.tpatch/workflows`; it does not create `.tpatch/local/` or edit `.gitignore`. | `internal/store/store.go:42-125`. |
| Per-feature artifacts currently live under `.tpatch/features/<slug>/artifacts`. | `internal/store/store.go:578-605`. |

## 1. Problem statement

`tpatch record` captures final patch bytes, and adjacent capture PRDs define
explicit claims and capture modes. During work, however, tpatch has no first-party
session object answering which feature an edit belongs to, where local session
buffers live, or how privacy-safe session context becomes a committed feature
summary.

The research brief says tpatch can know what changed at record time without
knowing why each file belongs to the feature, which action introduced it, or
whether unrelated edits leaked into the diff. The prior-art survey identifies
active sessions as the coordination point for agents, IDEs, hooks, and `record`,
while warning that full context can contain prompts, file contents, secrets, PII,
tool calls, and concurrent sessions.

ADR-027 resolved the privacy boundary but left the local path open. This PRD
locks that path and the active-session product contract inside ADR-027 D1-D12.

## 2. Goals / Non-goals

### 2.1 Goals

1. Define active feature session start, stop, list, summarize, and purge.
2. Lock ADR-027 F3 to `.tpatch/local/capture/` for active sessions.
3. Require `tpatch init` and writers to ensure `.tpatch/local/` is ignored before
   any local buffer write.
4. Define local-buffer to committed-summary promotion under
   `.tpatch/features/<slug>/artifacts/context/`.
5. Keep high-risk context capture default-off and opt-in.
6. Use content-addressed `cs_<12hex>`, `ctx_<12hex>`, and promoted `ce_<12hex>`
   IDs, with no wall-clock committed identity input.
7. Preserve cross-feature isolation and ADR-027 D10 provider boundaries.
8. Make CLI strings reviewable under Rules 8, 11, 15, and 17.

### 2.2 Non-goals

1. No agent event log implementation; deferred to `PRD-agent-event-log`.
2. No IDE capture hook implementation; deferred to `PRD-ide-capture-hooks`.
3. No Git hook guard implementation; deferred to `PRD-git-hook-capture-guards`.
4. No broad committed context-summary schema; deferred to `PRD-record-context-summary`.
5. No metadata branch, Git notes, remotes, object stores, or checkpoint services;
   deferred to `ADR-capture-metadata-branch`.
6. No cross-repo session sharing.
7. No multi-user concurrent session merge.
8. No raw transcript archive, raw IDE buffer archive, local debug bundle,
   embeddings, vectors, or `--capture-raw`.
9. No change to default `tpatch record <slug>` when no session flag is used.

## 3. Session lifecycle

### D1 — Start is explicit and per-feature

The v1 start trigger is:

```text
tpatch session start <slug> [--capture-context=summary|local-events] [--label <text>]
```

Rules:

1. `<slug>` must identify an existing feature.
2. Sessions are feature-scoped, not per-tpatch-command.
3. Existing commands such as `tpatch analyze <slug>` do not start sessions
   implicitly.
4. Agent lifecycle events do not start sessions implicitly in v1.
5. Starting when the same feature already has one active session in the workspace
   is idempotent: print the existing `cs_<12hex>` and write no new buffer.

Rationale: feature scope matches tpatch's feature directory model and gives
agents, IDEs, hooks, and `record` one stable feature association. Per-command
sessions are too short; agent-level lifecycle is too tool-specific for core v1.

### D2 — Stop is explicit; record-time close is opt-in

Stop triggers are:

```text
tpatch session stop <slug> [--session <cs_id>]
tpatch record <slug> --with-session [--from-session <cs_id>]
```

Rules:

1. `session stop` marks the selected same-feature local session `closed`.
2. `record --with-session` promotes the selected session and closes it only after
   the committed summary succeeds.
3. `record <slug>` without `--with-session` ignores local buffers.
4. End of process and timeout do not close sessions in v1.
5. Stale sessions may be reported by `session list`; they are not auto-promoted
   or auto-closed.

### D3 — Identity is content-addressed

Active sessions use `cs_<12hex>` IDs. Committed summaries use `ctx_<12hex>` IDs.
Promoted event references, if used, use `ce_<12hex>` IDs.

Rules:

1. The 12 hex characters are SHA-256 prefixes over canonical JSON identity fields,
   excluding the ID itself.
2. Required session identity inputs are repository identity, feature slug, base
   commit at start, capture mode, schema version, and a normalized local workspace
   discriminator.
3. Wall-clock timestamps, process IDs, adapter IDs, and local sequence numbers are
   not committed identity inputs.
4. Local buffers may contain adapter-native timestamps only as non-promotable
   diagnostic fields; promotion drops them.

### D4 — Local state machine

| State | Meaning |
|---|---|
| `active` | Same-feature local observations may be appended. |
| `closed` | No more observations; may be summarized or purged. |
| `promoted` | A redacted committed summary exists. |
| `purged` | Local buffer removed or tombstoned. |

Valid v1 transitions are `active -> closed`, `active -> promoted`,
`closed -> promoted`, and any state `-> purged`. Reopen and concurrent merge are
out of scope.

## 4. Storage — locks ADR-027 F3

### D5 — Canonical local-buffer path is `.tpatch/local/capture/`

The accepted storage path for v1 active sessions is:

```text
.tpatch/local/capture/<slug>/<cs_id>/session.json
.tpatch/local/capture/<slug>/<cs_id>/observations.jsonl
```

Rationale:

1. It stays inside the existing `.tpatch/` mental model.
2. It works more predictably than `.git/tpatch/capture/` across linked worktrees
   and `.git` file indirection.
3. It avoids OS-cache platform branching and accidental sharing across clones.
4. It keeps local state inspectable and purgeable while outside committed
   artifacts.
5. It is explicitly allowed by ADR-027 D1 when ignored-before-write is enforced.

### D6 — Ignore-before-write is mandatory

Rules:

1. `tpatch init` must add or preserve an effective `.gitignore` rule for
   `.tpatch/local/` before active-session implementation is enabled.
2. If `.gitignore` cannot be safely edited, `tpatch init` must print the exact
   rule and active-session writers must refuse until Git reports the path ignored.
3. `session start` must verify the effective ignore status of the concrete path
   before creating `.tpatch/local/capture/<slug>/<cs_id>/`.
4. Writers must refuse when Git is unavailable or the path is not ignored.
5. Verification must check effective Git ignore behavior, not just a textual line.
6. Pre-PRD workspaces are valid; the first active-session writer prompts/refuses
   until the invariant is satisfied.

Current production `tpatch init` does not create `.tpatch/local/` or edit
`.gitignore`, so implementation must amend `tpatch init` before active-session
writers ship.

### D7 — Rejected paths for v1

- `.git/tpatch/capture/`: rejected because linked worktrees and `.git` internals
  make the user-visible contract harder to reason about.
- OS user-cache locations: rejected because platform-specific cache roots are
  harder to discover and can blur multiple-clone boundaries.

A future PRD may supersede this storage choice.

### D8 — Local buffers are purgeable and non-authoritative

Local buffers are not required for deterministic replay, must not be committed,
must be purgeable before writers create long-lived state, and must not use
wall-clock timestamps in filenames. Local sequence numbers are allowed for append
order but are not promoted unless normalized and privacy-safe.

## 5. Session-to-summary promotion

### D9 — Promotion is explicit and opt-in

Promotion triggers are:

```text
tpatch record <slug> --with-session [--from-session <cs_id>]
tpatch session summarize <slug> [--session <cs_id>] [--dry-run] [--write] [--json]
```

Rules:

1. `record <slug>` without `--with-session` does not read local buffers.
2. `--with-session` selects the active or closed-and-unpromoted same-feature
   session when exactly one is eligible; otherwise `--from-session <cs_id>` is
   required.
3. `session summarize` defaults to dry-run; `--write` is the mutating mode.
4. Provider-assisted summarization is not required in v1. If introduced later, it
   must obey ADR-027 D10 and cannot receive raw local buffers.

### D10 — Promotion writes committed summaries under per-feature artifacts

Committed summaries are written to:

```text
.tpatch/features/<slug>/artifacts/context/<ctx_id>.json
```

Rules:

1. The file is strict schema-versioned JSON.
2. Allowed fields include `feature`, `session_id`, `summary`, `claim_ids`,
   `capture_mode`, `patch_generation_id`, `operation_ids`, `event_hashes`,
   `redaction`, and normalized repo-relative paths.
3. Forbidden fields include local buffer paths, absolute home paths, raw
   auto-dereferencing transcript refs, raw event bodies, and source snippets.
4. `record.md` may mention the context summary ID after promotion, but JSON is the
   machine-readable artifact.
5. The broader schema remains deferred to `PRD-record-context-summary`.

### D11 — Redaction is the boundary

Before writing a committed summary, the writer must prove ADR-027 D3 redaction
passed. At minimum, promotion must scrub or replace with symbolic references:
secrets, environment values, credentials, PII, absolute home paths, prompt text,
provider responses, chain-of-thought, tool-call arguments, command output,
stack traces, IDE buffers/selections/clipboard/LSP payloads, embeddings, vectors,
and source snippets outside canonical patch artifacts.

Redaction failure is a hard failure. Existing committed summaries remain
unchanged and the refusal explains the unsafe class.

### D12 — Promotion is optional

Users may use sessions locally and purge them without ever writing a committed
summary. `record --with-session` is opt-in per invocation. A feature with no
session artifacts behaves like a pre-PRD feature.

## 6. User-facing contract (CLI surface)

### D13 — New `tpatch session` command group

This PRD proposes a new command group; it does not exist in production today.

```text
tpatch session start <slug> [--capture-context=summary|local-events] [--label <text>]
tpatch session stop <slug> [--session <cs_id>]
tpatch session list [<slug>] [--json]
tpatch session summarize <slug> [--session <cs_id>] [--dry-run] [--write] [--json]
tpatch session purge [<slug>|--all] [--session <cs_id>] [--dry-run] [--yes]
```

Rules:

1. Rule 15: existing command mentions in this PRD match production shapes;
   `tpatch session` is explicitly proposed as new.
2. Rule 11: `tpatch session` inherits root `--path <dir>`; help must say tables
   list subcommand-specific flags only.
3. Rule 8: command names, flags, enums, JSON fields, and output code strings here
   are binding after acceptance.
4. Rule 17: these are v1 proposed active-session flags, not a claim about all
   future flags.

### D14 — Command-specific contracts

- `session start`: default `--capture-context=summary`; `local-events` still must
  not store raw prompts, raw tool bodies, IDE buffers, or secrets. `--label` is
  local-only unless redacted before promotion.
- `session list`: human output lists feature, `cs_<12hex>`, state, capture mode,
  and committed `ctx_<12hex>` if present. `--json` is deterministic and sorted by
  feature slug then session ID. It does not print local buffer bodies.
- `session summarize`: defaults to `--dry-run`; `--write` mutates;
  `--dry-run --write` is invalid. JSON includes `schema_version`, `feature`,
  `session_id`, `would_write`, `summary_id`, `redaction_status`, and
  `forbidden_content_findings`.
- `session purge`: defaults to `--dry-run`; deletion requires `--yes`. `--all`
  and `<slug>` are mutually exclusive. Purge shows exactly which local paths under
  `.tpatch/local/capture/` would be removed and refuses unsafe paths.

### D15 — New `record` flags

```text
tpatch record <slug> --with-session [--from-session <cs_id>]
```

Rules:

1. `--with-session` opts into same-feature local buffer read and summary
   promotion.
2. `--from-session <cs_id>` selects a session when multiple eligible same-feature
   sessions exist.
3. `--from-session` requires `--with-session`.
4. Existing `record` capture modes and flags keep current semantics when session
   flags are absent.
5. If promotion fails redaction, no partial committed context artifact remains.

## 7. Privacy invariants

### D16 — Active session to local buffer

Allowed v1 local-buffer content: `cs_<12hex>`, feature slug, capture mode, base
commit, claim IDs, operation IDs, normalized repo-relative paths, redacted short
summaries, hashes of omitted observations, symbolic references that do not
auto-dereference, and local append sequence numbers.

Forbidden v1 local-buffer content: raw transcripts, raw prompts, assistant
responses, chain-of-thought, tool call bodies, raw command stdout/stderr, IDE
buffers, selections, clipboard contents, raw LSP payloads, environment dumps,
secret values, source snippets outside canonical patch artifacts, embeddings, and
vectors.

This is intentionally stricter than a theoretical local raw transcript store;
v1 active sessions do not authorize raw local transcript archival.

### D17 — Local buffer to committed summary

Committed summaries may contain ADR-027 D2 material only: summaries, normalized
repo-relative paths/pathspecs, claim IDs, capture modes, generation/reconcile/
operation/event IDs, hashes of omitted content, opaque non-dereferencing symbolic
refs, enum reason codes, and short result summaries. They must not require
readers to dereference `.tpatch/local/capture/`.

### D18 — Cross-feature isolation

A session under `.tpatch/local/capture/<slug>/` belongs to `<slug>`. `record
--with-session` and `session summarize` read same-feature sessions only. Shared
paths do not authorize copying another feature's context. Unassigned local
observations are not auto-promoted. Manifest feature mismatch is a hard refusal.

### D19 — Provider-assisted carve-out

Committed summaries may flow to a provider endpoint only when all ADR-027 D10
conditions hold: the user selected a provider path; the payload is a committed
summary that already passed redaction; no local private buffer, raw transcript,
IDE buffer, or transcript-ref dereference is included; and symbolic external
references are not auto-dereferenced. This PRD does not authorize provider upload
of local buffers.

## 8. Acceptance criteria

1. **§8.1** `tpatch init` installs or preserves effective ignore coverage for
   `.tpatch/local/`, or active-session writers refuse with remediation.
2. **§8.2** `session start` refuses before writing if `.tpatch/local/capture/` is
   not effectively ignored by Git.
3. **§8.3** Starting the same feature session twice without close is idempotent.
4. **§8.4** `cs_<12hex>` and `ctx_<12hex>` IDs are content-addressed and exclude
   wall-clock identity input.
5. **§8.5** `session stop` transitions same-feature session state to `closed`
   without committed writes.
6. **§8.6** `record <slug>` without `--with-session` ignores local buffers and
   preserves existing behavior.
7. **§8.7** `record --with-session` promotes only same-feature sessions and
   refuses ambiguous selection without `--from-session <cs_id>`.
8. **§8.8** `--from-session` requires `--with-session`.
9. **§8.9** `session summarize` defaults to dry-run; writes require `--write`.
10. **§8.10** `session purge` defaults to dry-run; deletes require `--yes`.
11. **§8.11** Stop, summarize, and purge are idempotent on repeated runs.
12. **§8.12** Redaction failure prevents committed summary writes and leaves
    existing summaries unchanged.
13. **§8.13** Tests cover forbidden content classes: token-looking strings, env
    values, absolute home paths, prompt text, tool arguments, command output, IDE
    selection text, source snippets, and vector-like payloads.
14. **§8.14** Cross-feature promotion is rejected when session manifest feature and
    command slug differ.
15. **§8.15** `session list --json` and `session summarize --json` emit
    deterministic schema-versioned JSON, stable sort/order, and no wall-clock
    timestamps.
16. **§8.16** Per-session parse/redaction errors do not abort listing unrelated
    sessions; malformed sessions get isolated findings where possible.
17. **§8.17** Malformed local manifests make writers refuse append, promotion, or
    rewrite; read-only commands warn and continue.
18. **§8.18** Pre-PRD workspaces without `.tpatch/local/capture/` or context
    summaries continue to support existing `record`, `reconcile`, `status`,
    `verify`, and `doctor` behavior.
19. **§8.19** Purge refuses paths that escape `.tpatch/local/capture/` after
    cleaning and symlink evaluation.
20. **§8.20** Human output names local private buffer vs committed redacted summary
    so users see storage and sensitivity boundaries.
21. **§8.21** `tpatch session` help documents inherited root `--path <dir>` and
    says listed flags are subcommand-specific.
22. **§8.22** Existing `record` capture-mode mutex tests remain green when session
    flags are absent.
23. **§8.23** No active-session writer stores raw transcripts, prompts, assistant
    responses, chain-of-thought, raw tool bodies, raw env dumps, IDE buffers,
    selections, source snippets, embeddings, or vectors.
24. **§8.24** Provider-assisted reconcile receives committed summaries only when
    ADR-027 D10's four conditions are true; tests cover refusal when local buffer
    data or symbolic-ref dereference would be included.
25. **§8.25** Docs state `.tpatch/local/` is local private state, not a committed
    artifact, and may be purged without breaking deterministic replay.

## 9. Open questions

1. Should v2 allow multiple concurrent same-feature sessions with distinct adapter
   scopes?
2. Should `--capture-context=off|summary|local-events` become shared across
   `session`, `record`, and adapter commands?
3. Should active-session summaries be folded into `PRD-record-context-summary`
   once that PRD is accepted?
4. Should labels remain local-only forever, or can redacted labels become
   committed summary fields?
5. Should committed-summary purge be a `session purge --committed` mode or a
   separate privacy command?

## 10. Out of scope

This PRD does not authorize or decide:

1. `PRD-agent-event-log` event schema, adapter protocol, token usage, subagent
   events, or tool summaries.
2. `PRD-ide-capture-hooks` editor save/rename/delete hooks, diagnostics,
   selections, buffers, and IDE configuration.
3. `PRD-git-hook-capture-guards` hook installation, chaining, warnings, trailers,
   post-rewrite remapping, or pre-push behavior.
4. `PRD-record-context-summary` broader per-generation context summary schema.
5. `ADR-capture-metadata-branch` metadata branches, Git notes, object stores,
   private metadata remotes, checkpoint services, and propagation policy.
6. Cross-repo session sharing.
7. Multi-user concurrent session merge.
8. Raw transcript storage, raw IDE buffer storage, raw debug bundles,
   `--capture-raw`, embeddings, vectors, RAG over context, and cross-feature
   semantic search.
9. Automatic session creation from `tpatch analyze`, `tpatch implement`, agent
   lifecycle hooks, IDE hooks, or Git hooks.

## 11. Sources
- `docs/handoff/CURRENT.md:11-53` for Stream A binding scope.
- `docs/adrs/ADR-027-capture-context-privacy-boundary.md:88-117` for D1 two-lane storage and local-buffer path softness.
- `docs/adrs/ADR-027-capture-context-privacy-boundary.md:119-170` for D2-D3 content and redaction boundaries.
- `docs/adrs/ADR-027-capture-context-privacy-boundary.md:226-270` for D6-D7 IDs and default-off capture.
- `docs/adrs/ADR-027-capture-context-privacy-boundary.md:300-343` for D9-D10 isolation and provider carve-out.
- `docs/adrs/ADR-027-capture-context-privacy-boundary.md:365-401` for backward compatibility and non-scope precedent.
- `docs/state-of-the-art/research-roadmap.md:99-113`, `docs/state-of-the-art/research-roadmap.md:149-171` for queue position.
- `docs/state-of-the-art/patch-capture-context-research-brief.md:38-58`, `docs/state-of-the-art/patch-capture-context-research-brief.md:177-197` for capture gap and layered model.
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:35-63`, `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:397-415`, `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:481-495` for prior art and sequencing.
- `docs/prds/PRD-feature-file-claims.md:53-72`, `docs/prds/PRD-feature-file-claims.md:191-234`, `docs/prds/PRD-feature-file-claims.md:280-319` for claim context.
- `docs/prds/PRD-record-capture-modes.md:52-79`, `docs/prds/PRD-record-capture-modes.md:218-292`, `docs/prds/PRD-record-capture-modes.md:320-343` for record modes.
- `docs/prds/PRD-tpatch-doctor.md:400-459` for acceptance shape.
- `internal/cli/cobra.go:54-80`, `internal/cli/cobra.go:438-470`, `internal/cli/cobra.go:942-960`, `internal/cli/cobra.go:1488-1500`, `internal/cli/cobra.go:1766-1977` for CLI ground truth.
- `internal/store/store.go:42-125`, `internal/store/store.go:578-605` for `.tpatch/` initialization and artifact paths.
- `docs/supervisor/LOG.md:2028-2048` for Rule 11 persistent-flag inheritance.
