# ADR-027: Capture Context Privacy Boundary

**Status**: Accepted (three-way review 2026-07-17: internal `7dbf6f4`, supervisor-external `a363ed2`, user-external in LOG.md)
**Date**: 2026-07-16
**Owner**: Core
**Cluster**: Capture and metadata foundation (roadmap §3.1)
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-024](./ADR-024-patch-generation-manifest-boundary.md), [ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md) (privacy pattern precedent)
**Blocks**: `PRD-active-feature-session`, `PRD-record-context-summary`, `PRD-agent-event-log`, `PRD-ide-capture-hooks`, `PRD-git-hook-capture-guards`, `ADR-capture-metadata-branch`

## Context

The patch-capture research front asks how tpatch should capture not just patch
bytes, but intent, precise scope, agent context, recipe provenance, and Git
history boundaries while work is happening
(`docs/state-of-the-art/patch-capture-context-research-brief.md:14-27`).
The current baseline is intentionally narrower: `record` captures working-tree
or committed-range diffs; feature docs preserve some intent; `apply-recipe.json`
captures deterministic operations; but tpatch does not know why every file
belongs to a feature, what context was used, or whether unrelated edits leaked
into the diff (`docs/state-of-the-art/patch-capture-context-research-brief.md:38-58`).

The research packet recommends layered capture: intent, scope claims, edit
telemetry, recipe provenance, Git boundary, patch artifact, and a privacy-safe
context summary (`docs/state-of-the-art/patch-capture-context-research-brief.md:177-197`;
`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:356-369`). The
roadmap places this ADR first in the capture/privacy sequence, before active
feature sessions, record context summaries, agent event logs, IDE hooks, Git
hook guards, or a metadata branch are designed
(`docs/state-of-the-art/research-roadmap.md:99-113`,
`docs/state-of-the-art/research-roadmap.md:149-171`).

Prior art provides both useful primitives and warning signs. Quilt's explicit
file ownership shows why claims are safer than reconstructing scope from a large
diff after the fact (`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:83-109`).
Git's index, trailers, and hooks provide composable capture boundaries, but the
research explicitly says hooks must be opt-in, visible, and reversible
(`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:171-194`). Entire's
separate `entire/checkpoints/v1` metadata branch demonstrates a clean write-path
for rich agent context, linked from commits by trailers, while also showing the
privacy cost of storing prompts, responses, file contents, tool calls, secrets,
PII, and concurrent sessions
(`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:212-242`). The same
research says tpatch should not copy full raw transcript storage by default and
should prefer summaries, references, and hashes unless a user explicitly opts in
(`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:263-271`,
`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:416-437`).

The drafted capture PRDs already lean on a restrictive boundary. File claims are
scope metadata, not transcript storage, and v1 deliberately omits free-text
reasons until this privacy ADR lands
(`docs/prds/PRD-feature-file-claims.md:53-72`,
`docs/prds/PRD-feature-file-claims.md:191-234`,
`docs/prds/PRD-feature-file-claims.md:289-299`). Record capture modes make
staged, unstaged, all, and claimed-only boundaries explicit while declaring that
raw agent-context capture is a non-goal
(`docs/prds/PRD-record-capture-modes.md:52-79`,
`docs/prds/PRD-record-capture-modes.md:102-127`,
`docs/prds/PRD-record-capture-modes.md:268-292`).

This ADR inherits two hard precedents:

1. **Determinism and content-addressing**. ADR-024 uses a monotonic generation
   number plus content-addressed `pg_<12hex>` IDs, forbids wall-clock timestamps
   in patch-generation manifests, refuses historical backfill, and snapshots
   dependency identities by hashes and generation references
   (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:77-187`).
2. **Privacy as a schema boundary**. ADR-025 D10 forbids raw source bodies,
   provider transcripts, prompt text, vectors, and embeddings in reconcile
   JSONL artifacts; allowed material is limited to paths, hashes, operation IDs,
   enum reason codes, verdicts, refs, counts, validation references, and
   cross-artifact IDs
   (`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:273-291`).

ADR-026 is the sibling policy-ADR precedent for locking downstream writer policy
without changing code or PRDs; it also keeps metadata-only amendment outside
patch-generation history unless a future ADR creates a dedicated metadata log
(`docs/adrs/ADR-026-patch-amendment-policy.md:33-35`,
`docs/adrs/ADR-026-patch-amendment-policy.md:187-204`).

This ADR is documentation-only. It changes no code, schema, CLI behavior, PRD
body, asset text, handoff file, or supervisor log. It locks the persistence and
privacy boundary that downstream capture PRDs must stay within.

## Decision

### D1 — Storage boundary is two-lane: committed summaries, local private buffers

tpatch capture persistence has exactly two allowed storage lanes until a future
ADR supersedes this boundary:

1. **Committed summary lane**: privacy-safe, deterministic artifacts under the
   existing per-feature tree, e.g. `.tpatch/features/<slug>/artifacts/`, may be
   committed with the repository. This lane stores only redacted summaries,
   hashes, normalized repo-relative paths, IDs, claim references, capture-mode
   provenance, and external symbolic references.
2. **Local private buffer lane**: high-granularity session, agent, IDE, and hook
   observations live outside the tracked worktree by default, under
   `.git/tpatch/capture/` or an equivalent OS user-cache location. If a future
   PRD chooses any worktree path such as `.tpatch/local/capture/`, the writer
   MUST verify that path is ignored before the first write and MUST refuse rather
   than risk accidental commit.

Surface-specific storage is locked as follows:

| Surface | Storage location | Commit policy |
|---|---|---|
| Active feature sessions | Local private buffer lane. A committed summary may reference the session only after `record` or an explicit summary command promotes a redacted snapshot. | Raw session state is never committed. |
| Agent event logs | Local private buffer lane. Committed lane stores only summary records, event hashes, prompt refs, tool refs, file lists, and test/command summaries. | Raw prompts, responses, tool inputs/outputs, and transcripts are never committed. |
| IDE capture hooks | Local private buffer lane. Committed lane may store paths, operation IDs, diagnostic codes, and redacted summaries. | Editor selections, buffers, unsaved contents, and raw diagnostics containing source bodies are never committed. |
| Git hook capture guards | Hook installation/configuration is local. Guard observations may be local; trailers or committed summary IDs may be written only after passing this ADR's redaction contract. | Hooks must not silently commit context bodies. |
| Record context summary | Committed summary lane, per feature and per patch generation, with content-addressed IDs and redaction metadata. | This is the only capture surface intended to be repo-visible by default after explicit consent. |

Separate metadata branches, Git notes, external object stores, and remote
checkpoint services are not allowed write targets under this ADR. They are
deferred to `ADR-capture-metadata-branch`.

### D2 — Content shape is summaries, hashes, IDs, and symbolic references, not raw context

Committed capture artifacts MUST use the smallest privacy-safe representation
that can support audit and future reconcile:

- human/agent-authored summaries;
- normalized repo-relative paths and pathspecs;
- claim IDs, capture modes, generation IDs, reconcile IDs, operation IDs, and
  hook/event IDs;
- cryptographic hashes of omitted content;
- opaque external references such as `transcript_ref`, `prompt_ref`,
  `tool_use_id`, or `keychain_ref`;
- enum reason codes and short result summaries;
- command/test names and exit classifications, without raw stdout/stderr unless
  that output has been redacted and summarized.

tpatch-managed artifacts MUST NOT persist raw provider transcripts, prompt text,
assistant responses, chain-of-thought, IDE buffer contents, editor selections,
tool inputs, raw tool outputs, raw environment dumps, unredacted diagnostics, or
source snippets outside the canonical patch artifacts that already own source
bytes. Source bodies belong to Git diffs, `artifacts/post-apply.patch`, and the
normal repository tree, not to capture-context metadata.

The secret-by-reference pattern is mandatory. A context artifact may say
`env:GITHUB_TOKEN`, `keychain:github/copilot`, or `external:agent-session/<id>`;
it may not store the value behind that reference.

### D3 — Redaction is a write precondition, not a best-effort cleanup

Every tpatch-owned writer MUST run the redaction contract before persisting any
capture artifact in either lane. Promotion from local private buffers to
committed summaries MUST refuse if the redactor cannot prove the promoted
payload is safe.

At minimum, writers MUST scrub or replace with symbolic references:

- API keys, personal access tokens, OAuth refresh/access tokens, session cookies,
  JWTs, SSH/private keys, cloud credentials, database URLs, passwords, and
  bearer/basic authorization headers;
- environment variable values and process environment dumps;
- OS usernames, home-directory absolute paths, emails, customer names, and other
  PII that is not required to identify a repo-relative artifact;
- prompt text, provider responses, chain-of-thought, tool-call arguments,
  command stdout/stderr, stack traces, and diagnostics when they include source
  bodies or secrets;
- unsaved IDE buffer text, selected text, clipboard content, and raw LSP payloads;
- embeddings, vector payloads, and other derived representations that can leak
  source or prompt content.

Redaction failure is a hard failure for committed summaries. Local private
buffers may keep only the redacted or hashed form; this ADR does not authorize a
tpatch-managed raw transcript archive.

### D4 — Reader/writer authority is command-scoped and least-privilege

Writers are restricted by artifact class:

- Active-session commands, agent adapters, IDE adapters, and Git hook adapters
  may write only the local private buffer lane unless a downstream PRD gives them
  an explicit summary-promotion command.
- `tpatch record` and a future `tpatch context summarize`-style command may read
  same-feature local buffers and write committed context summaries, but only
  after explicit user consent and successful redaction.
- Git hooks may write local guard observations and validated trailers. They may
  not promote context bodies to committed artifacts on their own.
- `tpatch reconcile` writes only reconcile-owned artifacts governed by ADR-025
  and patch-generation artifacts governed by ADR-024/ADR-026. It does not become
  a capture-context writer under this ADR.

Readers are restricted by sensitivity:

- `status`, `list`, `verify`, and read-only inspection commands may read
  committed summaries and IDs. They read local private buffers only with an
  explicit local-context flag defined by a future PRD.
- `record` may read local buffers for the same feature to compile a summary.
  It must ignore unassigned or cross-feature local buffers unless the user
  explicitly selects them.
- `reconcile` may read committed summaries and hashes for the target feature and
  explicitly declared dependencies. It MUST NOT read local private buffers, raw
  event logs, IDE buffers, transcript refs that require dereferencing, or
  cross-feature context by default. Provider-assisted reconcile may receive only
  redacted committed summaries, never local raw context.

This answers the reconcile-time question directly: reconcile-time reads are
allowed for committed redacted summaries and cross-artifact IDs; local capture
buffers are write-local/read-by-record-only until a future ADR says otherwise.

### D5 — Retention semantics are append-only for committed summaries and purgeable for local buffers

Committed context summaries are append-only audit artifacts while present. A
later summary may supersede an earlier one by ID, but writers MUST NOT silently
rewrite or compact prior summaries. Explicit compaction may be introduced only if
it operates on already-redacted summaries and records the source summary IDs it
replaces.

Privacy purge is a first-class exception to append-only audit. A future purge
command may remove or tombstone committed summaries and local buffers when the
user decides privacy outweighs replay/audit convenience. After purge, readers
must render the context as `purged` or `unavailable`, not reconstruct it from
hashes, transcripts, Git history, shell history, provider logs, or other
side-channels.

Local private buffers are not long-term project records. They MUST be explicitly
purgeable, MUST NOT be required for deterministic replay, and SHOULD default to
bounded retention chosen by the downstream PRD. No capture PRD may require
indefinite local raw-context retention as a condition for normal tpatch use.

### D6 — Determinism uses content-addressed IDs and excludes wall-clock time

All committed capture artifacts that need stable identity use content-addressed
IDs in the existing tpatch style:

- context summaries reserve `ctx_<12hex>`;
- capture sessions reserve `cs_<12hex>` when their identity is promoted into a
  committed summary;
- capture events or observations reserve `ce_<12hex>` when individual events are
  promoted by hash.

The 12 lowercase hex characters are the prefix of SHA-256 over canonical JSON of
the persisted identity fields, excluding the ID itself, redacted content hashes
that are not part of identity, and any local-only ordering fields. This mirrors
ADR-024's `pg_<12hex>` generation precedent and ADR-025's `re_<12hex>` /
`rr_<12hex>` JSONL precedent
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:77-127`,
`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:112-132`,
`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:181-191`).

Wall-clock timestamps are forbidden in committed capture artifacts and in their
ID hash inputs. Ordering must come from patch generation IDs, content hashes,
Git object IDs, explicit sequence numbers scoped to a local session, or
append order in a strict JSONL artifact. Local buffers may contain adapter-native
timestamps only as untrusted, non-promotable diagnostic fields; promotion to the
committed lane must drop them.

### D7 — High-risk capture surfaces are default-off

Existing `record` behavior, file claims, capture-mode provenance, patch
generation metadata, and reconcile evidence keep their existing defaults. New
context capture is different: active sessions, agent event logs, IDE hooks, Git
hook guards, record context summaries, external refs, and any local buffer read
are default-off unless a downstream PRD proves the surface is safe to enable.

Default-off means:

- `tpatch init` MUST NOT install capture hooks silently;
- `tpatch record <slug>` MUST NOT ingest local agent/IDE logs unless the user
  opted into context capture for that feature or invocation;
- provider-assisted commands MUST NOT receive capture summaries unless the user
  already selected a provider path and the summaries are committed/redacted;
- raw transcript capture, raw IDE buffer capture, and external upload are not
  merely default-off: they are unauthorized until a future ADR explicitly accepts
  them.

### D8 — CLI consent surface must make storage and sensitivity visible

Downstream PRDs must expose capture behavior with explicit user-facing language.
The following vocabulary is reserved for future CLI design; equivalent names may
be accepted only if the PRD preserves the same semantics:

```text
--capture-context=off|summary|local-events
--context-summary <text-or-file>
--include-local-context
--no-context
tpatch hooks install --capture-guards
tpatch context purge [<slug>|--all]
```

Rules:

- The default mode is `--capture-context=off`.
- `summary` may write only committed redacted summaries.
- `local-events` may write local private buffers but does not imply committed
  promotion.
- `--include-local-context` is never implied by `--json`, `--verbose`, provider
  mode, or reconcile mode.
- A future `--capture-raw` flag is explicitly reserved but MUST fail closed until
  a separate ADR defines raw storage, consent, redaction, retention, and purge.
- Hook installation UX must name every hook it edits, the files it writes, and
  how to uninstall or disable it.

### D9 — Cross-feature isolation is mandatory

Capture artifacts are feature-scoped. A context summary for feature A may cite
feature B only through existing dependency, claim, patch-generation, or reconcile
IDs, or through an explicit user-selected cross-reference. Shared file paths do
not grant permission to copy feature B's context into feature A's summaries.

Unassigned local events are not automatically promoted. They may become part of a
feature summary only when the user or an authorized adapter assigns them to that
feature and the redaction contract passes.

Repo-wide or cross-feature search over summaries, hashes, event logs, prompts,
or embeddings is out of scope. A future vector/RAG or metadata-branch PRD must
reopen this clause before building any cross-feature context index.

### D10 — Machine boundary is local-first; external services store references only

tpatch capture is local-first. Under this ADR, tpatch MUST NOT upload context
artifacts, event logs, summaries, hashes, prompts, transcripts, embeddings, or
metadata branches to an external service. It may store symbolic references to
external systems that already exist outside tpatch, provided the reference does
not dereference automatically during normal reads.

**Provider-assisted carve-out (D4 + D7 sole authorized channel)**: the ban above
does NOT apply to redacted committed summaries transmitted to a user-selected
provider endpoint (`OpenAICompatible`, `Anthropic`, or future providers) as part
of a provider-assisted `tpatch reconcile` or equivalent command. That channel is
authorized only when ALL of the following hold: (a) the user has explicitly
selected a provider path (per D7), (b) the payload is a committed summary that
has already passed the D3 redaction contract, (c) no local private buffer, raw
transcript, IDE buffer, or transcript-ref dereference is included (per D4), and
(d) no automatic dereference of symbolic external references occurs. Provider
endpoints are the only external channel this ADR authorizes; all other external
uploads remain banned.

Secrets live outside `.tpatch/` and outside committed artifacts. If a future
integration needs credentials, the committed artifact stores only an environment
variable name, keychain reference, or provider profile name. OS keychains may be
used for local credentials, but keychain contents are never copied into capture
artifacts.

Separate metadata branches, private metadata remotes, Git notes, object stores,
and checkpoint services require `ADR-capture-metadata-branch` or a successor ADR.
That future ADR must preserve D2-D9 unless it explicitly supersedes them.

### D11 — Malformed-artifact handling mirrors ADR-024 and ADR-025

Capture artifacts are malformed when they fail JSON/JSONL parsing, omit required
schema or redaction fields, use unsupported versions, contain unknown fields in a
strict v1 schema, contain forbidden raw content, fail content-addressed ID
validation, point at local buffers from a committed summary, or violate
cross-feature isolation.

Writers refuse to append, promote, or rewrite on malformed input. Read-only
commands warn and continue with context disabled. `reconcile` distrusts malformed
context artifacts, excludes them from provider prompts and evidence, and proceeds
from canonical patch bytes and ADR-025 evidence where possible. No v1 command may
silently truncate, repair, redact-in-place, or reserialize a malformed artifact.

This is the same asymmetric blast-radius policy as ADR-024 D7 and ADR-025 D11:
writers are the corruption chokepoint; readers preserve operator visibility
without compounding bad state
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:218-251`,
`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:293-316`).

### D12 — Backward compatibility treats pre-ADR context as absent or untrusted

Existing feature directories have no ADR-027 context metadata. That means "no
captured context declared"; existing commands continue to behave as they do now.
No historical backfill scans `patches/`, provider logs, IDE histories, shell
histories, Git reflogs, or previous conversations to reconstruct context. This
follows ADR-024's no-backfill precedent for generation manifests
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:129-155`).

If a repository already contains ad hoc context files, transcript dumps,
`Entire-Checkpoint` trailers, Git notes, or agent-specific metadata, tpatch v1
does not parse or promote them as ADR-027 artifacts. Committed files without an
ADR-027 schema version and redaction marker are untrusted. Read-only commands may
warn; writers skip them unless a future import PRD defines a safe migration path.

### D13 — Explicit non-scope

This ADR does not decide:

- exact JSON/JSONL schemas for `PRD-active-feature-session`,
  `PRD-record-context-summary`, `PRD-agent-event-log`, `PRD-ide-capture-hooks`,
  or `PRD-git-hook-capture-guards`;
- whether tpatch should use a separate metadata branch, Git notes, private
  remote, or external checkpoint store (`ADR-capture-metadata-branch`);
- hook installation implementation details beyond opt-in, visibility,
  reversibility, and this privacy boundary;
- raw transcript archives, raw IDE buffer archives, local debug bundles, or
  `--capture-raw`;
- vector indexes, embeddings, RAG over context, or cross-feature semantic search;
- provider prompt templates that consume context summaries;
- concurrency, locking, or conflict rules for multiple active sessions;
- retention durations, compaction formats, and purge UX details beyond the D5
  requirement that purge exists before any local buffer becomes long-lived;
- legal/compliance policy for teams that need stricter rules than this ADR.

Future PRDs may fill these gaps only inside the D1-D12 privacy boundary or by
explicitly superseding this ADR.

## Alternatives considered

### A1 — Commit raw transcripts and IDE logs with the feature

Rejected. It gives future reconcile the most context, but it violates the
research packet's warning that transcripts can contain prompts, tool
interactions, file contents, secrets, and PII
(`docs/state-of-the-art/patch-capture-context-research-brief.md:153-175`;
`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:46-57`). It also
contradicts ADR-025 D10's hard privacy boundary.

### A2 — Copy Entire's metadata branch model immediately

Rejected for this ADR. Entire's separate branch keeps user commits clean and is
worth studying, but it introduces branch propagation, private remotes, redaction,
rewrite remapping, and retention mechanics that are explicitly queued for
`ADR-capture-metadata-branch`
(`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:212-242`,
`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:481-495`). This ADR
must unblock near-term PRDs without pre-binding that storage design.

### A3 — Store everything local-only and never commit summaries

Rejected. It is privacy-preserving, but it prevents patch generations, future
reconcile, and review from seeing the context that the research specifically
wants to preserve. The accepted compromise is a committed summary lane with
strict redaction and content-addressed IDs.

### A4 — Default-on agent/IDE/hook capture with opt-out

Rejected. Hooks and adapters observe high-risk context before the user has
reviewed it. The prior-art survey says Git hooks should be opt-in, visible, and
reversible (`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:193-194`,
`docs/state-of-the-art/patch-capture-prior-art-and-hooks.md:439-451`). The
research roadmap says privacy must be settled before richer capture begins.

### A5 — Let reconcile read all local context for better provider prompts

Rejected. Reconcile is high-value, but local event logs can contain prompt,
selection, command, or workspace details never meant to leave the machine or
cross feature boundaries. D4 allows reconcile to use committed redacted summaries
and hashes while refusing local raw/local-private buffers.

### A6 — Use timestamps, UUIDs, or adapter IDs as primary identity

Rejected. They are convenient for live sessions, but they make committed
artifacts non-deterministic and conflict with ADR-024 D3's no-wall-clock
precedent. Content-addressed IDs preserve byte-repeatability and make duplicate
summary detection possible.

## Consequences

Positive:

- Downstream capture PRDs can proceed with a fixed privacy perimeter.
- Existing record, reconcile, patch-generation, and claim behavior remains
  unchanged until explicit capture surfaces ship.
- tpatch gets enough future audit value through summaries, hashes, IDs, claims,
  capture modes, and generation refs without storing raw sensitive context.
- Provider-assisted reconcile can eventually consume redacted summaries without
  learning local raw event logs.
- Explicit purge and default-off capture make privacy failures less likely and
  easier to remediate.

Negative:

- Future agent/IDE integrations cannot offer "full session replay" without a new
  ADR.
- Summary quality becomes important; poor summaries may omit context that raw
  transcripts would have preserved.
- Redaction becomes a required shared primitive before any capture writer ships.
- Local-only buffers may be lost before record if users purge or move machines.
- Cross-feature and vector/RAG use cases remain blocked until separate privacy
  and storage decisions land.

Implementation follow-through required by future PRDs:

- define schemas with `schema_version`, redaction metadata, strict readers, and
  content-addressed IDs;
- land readers before writers when a new committed artifact is introduced;
- test forbidden-content rejection and no-wall-clock determinism;
- verify local buffer paths cannot be committed accidentally;
- provide purge UX before local buffers become long-lived.

## Follow-up

Recommended sequence:

1. Review/accept `PRD-feature-file-claims` with this ADR as the reason that
   claim reasons remain omitted or summary-only.
2. Review/accept `PRD-record-capture-modes` with this ADR as the reason that
   capture-mode provenance remains paths/claims/modes, not raw context.
3. Draft `PRD-active-feature-session` inside the D1 local-private storage lane.
4. Draft `PRD-record-context-summary` as the first committed summary-lane schema.
5. Draft `PRD-agent-event-log`, `PRD-ide-capture-hooks`, and
   `PRD-git-hook-capture-guards` only after the summary schema and purge story
   exist.
6. Draft `ADR-capture-metadata-branch` only if committed summaries plus local
   private buffers prove insufficient.

## References

- [State-of-the-art research roadmap](../state-of-the-art/research-roadmap.md)
- [Patch capture and agent context research brief](../state-of-the-art/patch-capture-context-research-brief.md)
- [Patch capture prior art and hook models](../state-of-the-art/patch-capture-prior-art-and-hooks.md)
- [PRD-feature-file-claims](../prds/PRD-feature-file-claims.md)
- [PRD-record-capture-modes](../prds/PRD-record-capture-modes.md)
- [ADR-024: Patch generation manifest boundary](./ADR-024-patch-generation-manifest-boundary.md)
- [ADR-025: Reconcile evidence and revision schema](./ADR-025-reconcile-evidence-and-revision-schema.md)
- [ADR-026: Patch amendment policy](./ADR-026-patch-amendment-policy.md)
