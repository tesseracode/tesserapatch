# Patch Capture Prior Art and Hook Models

**Status**: Snapshot research (paper-only; no implementation authorized)
**Date**: 2026-05-11
**Owner**: Core
**Related**: [Patch capture brief](patch-capture-context-research-brief.md),
[Patch identity metadata research](tpatch-metadata-for-patch-identity.md),
[Middle-pass synthesis](tpatch-middle-pass-synthesis.md),
[Recording Patches](../record.md),
[Feature Layout](../feature-layout.md),
[Research roadmap](research-roadmap.md)

## Why this doc exists

tpatch already captures feature bytes through `tpatch record`, and it already
captures intent through lifecycle docs such as `request.md`, `spec.md`, and
`exploration.md`. The open question is whether the capture boundary is rich
enough before reconcile:

> Can tpatch capture patch scope, agent context, provenance, and Git history
> boundaries while the work is happening, instead of reconstructing everything
> after the fact?

This note starts that research by looking at Quilt, StGit, Git hooks/trailers,
Entire, and Aider-style agent workflows.

## Refresh triggers

- A PRD opens for file claims, active-feature sessions, agent event logs, IDE
  hooks, Git hook guards, or patch-generation manifests.
- Entire changes its checkpoint storage or external agent protocol.
- tpatch changes `record`, `apply-recipe.json`, feature layout, or skill
  capture guidance.

## 1. Executive findings

1. **Quilt's strongest lesson is explicit scope.** A patch owns only files that
   were added to it before editing; refresh compares those files to saved
   originals. This is a strong antidote to accidental diff capture.
2. **StGit's strongest lesson is Git-native patch stacks.** Patches are Git
   commits that can be refreshed, reordered, applied/unapplied, and eventually
   migrated into ordinary history.
3. **Git's strongest lesson is composable capture points.** The index, commit
   message trailers, and hooks each capture a different boundary: staged bytes,
   durable metadata, and lifecycle events.
4. **Entire's strongest lesson is a separate metadata write-path.** It keeps user
   commits clean while persisting full agent context on `entire/checkpoints/v1`,
   linked by an `Entire-Checkpoint` trailer.
5. **Entire also shows the cost of full context.** Transcripts can contain file
   contents, prompts, tool calls, secrets, PII, and multiple concurrent sessions;
   redaction and branch isolation become product-critical.
6. **Aider's strongest lesson is edit separation.** It auto-commits agent edits
   and first protects preexisting dirty files, making undo/review easier.
7. **tpatch should probably combine these ideas, not copy one.** Keep current
   `record` as the fallback, add optional explicit claims, capture agent/IDE
   events in a local-first log, attach durable IDs through trailers/manifests,
   and summarize sensitive context instead of storing raw sessions by default.
8. **Amendment policy is a first-class design choice.** Quilt and StGit mostly
   rewrite the patch under management, Git supports both amend and fixup-forward,
   and Entire preserves context around rewrites instead of defining patch
   content. tpatch likely needs a canonical-current patch plus append-only patch
   generations/fixups.

## 2. Current tpatch capture baseline

Current tpatch has these capture surfaces:

| Surface | Current role | Capture gap |
|---|---|---|
| `request.md` | Original feature request. | Intent is preserved, but not linked to every later edit. |
| `analysis.md`, `spec.md`, `exploration.md` | Planning and file investigation. | Mostly prose, not operation/session provenance. |
| `apply-recipe.json` | Deterministic operations from implement. | No stable op IDs, read/write sets, active session IDs, or tool provenance. |
| `tpatch record <slug>` | Captures working-tree diff including untracked files. | Can capture too much if unrelated edits exist. |
| `tpatch record --from <base>` | Captures commit-range diff after the fact. | Depends on users picking the right boundary. |
| `artifacts/post-apply.patch` | Canonical replay diff. | Stores bytes, not why/scope/provenance. |
| `patches/NNN-*.patch` | Historical full-diff audit snapshots. | Not canonical patch generations. |

So tpatch has a good **final artifact** story, but a weaker **during-work
capture** story.

## 3. Prior art

### 3.1 Quilt: explicit file ownership

Quilt manages a stack of patch files plus a `series` order. The key capture
mechanism is explicit file ownership:

- `quilt new <patch>` starts a patch.
- `quilt add <file>` adds files to the top patch before modification.
- Quilt saves backup copies of added files under `.pc/<patch>/`.
- `quilt refresh` regenerates the patch by diffing those backups against the
  current tree.
- `quilt files` lists files owned by a patch.
- `quilt graph` can generate dependency graphs from shared files or overlapping
  modified lines.

tpatch translation:

| Quilt idea | tpatch candidate |
|---|---|
| `quilt add` before editing | `tpatch claim <slug> path...` |
| `.pc/<patch>/` backups | generation-scoped base snapshots or base commit + blob IDs |
| `quilt refresh` | `tpatch record --claimed-only` or claim-aware `record` warnings |
| `series` file | tpatch dependency/order graph |
| `quilt graph --lines` | commutation graph evidence |

Product implication: tpatch should consider **claims as scope constraints**.
Claims could start advisory, then support strict capture modes.

Amendment behavior:

- `quilt refresh` rewrites the specified patch file, or the top patch by
  default. This is the normal "my patch was wrong/incomplete; update the patch"
  path.
- Quilt preserves documentation/header text before the actual patch when it
  refreshes.
- `quilt refresh --backup` can keep a backup copy of the previous patch as
  `patch~`.
- `quilt refresh -z[new_name]` creates a new patch containing the changes
  instead of refreshing the top patch; if no name is given, Quilt uses a forked
  style name such as `-2`.
- `quilt fork` copies the top patch under a new name so the user can amend a
  copy while preserving the original.
- `quilt remove` removes files from a patch, and `quilt revert` reverts
  uncommitted changes for files in a patch.
- Refreshing a non-top patch is possible, but Quilt aborts by default if newer
  patches modify the same files; `-f` can force it but ignores shadowed changes.

So Quilt's default is **mutate the main patch**, with optional backup/fork/new
patch paths when the user wants history or split concerns.

### 3.2 StGit: Git-native patch stack

StGit manages Git commits as a patch stack. Its homepage describes operations
to push/pop/goto patches, refresh a patch from working-tree changes, edit patch
metadata, create/delete patches, and migrate between stack patches and ordinary
commits.

Relevant ideas for tpatch:

- Treat a feature patch as a mutable stack element during development.
- Use Git objects as durable patch storage rather than inventing every storage
  primitive.
- Allow migration between "patch under management" and "ordinary commit".
- Keep each patch focused on one concern.

tpatch differs because it has natural-language intent, recipes, and reconcile
metadata; still, StGit reinforces that patch generation history should be
Git-native or at least Git-addressable.

Amendment behavior:

- `stg refresh` includes latest worktree/index changes in the current patch.
- Refresh creates a new Git commit object for the patch; the old commit is no
  longer visible in normal stack view.
- StGit records stack-log entries, so undo can walk back the refresh/merge
  process.
- If refreshing a non-top patch, StGit first creates a temporary patch and then
  merges it into the requested patch. Conflicts leave the temporary patch for
  the user to handle, for example with `stg squash`.
- `stg new` creates a new empty patch when the change is a distinct concern.
- `stg squash` combines multiple patches into one patch when a later fixup
  should become part of the original logical change.
- `stg commit` finalizes managed patches into the base and removes them from
  the stack.

So StGit's default is also **mutate the managed patch**, but because patches are
Git commits it has a stronger undo/log story and supports explicit
new-patch-then-squash workflows.

### 3.3 Git index, trailers, and hooks

Git has three capture primitives that matter for tpatch:

| Primitive | What it captures | tpatch use |
|---|---|---|
| Index/staging area | User-curated set of hunks/files for the next commit. | `record --staged`, hunk-scoped capture, "feature boundary = index". |
| Trailers | Structured commit metadata at message end. | `Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`, future context IDs. |
| Hooks | Lifecycle events around commit/push/checkout/rebase. | Warn, add trailers, condense context, push metadata, detect rewrite breakage. |

Most relevant hooks:

| Hook | Git behavior | Possible tpatch behavior |
|---|---|---|
| `pre-commit` | Can abort before commit. | Warn/abort when staged files are unclaimed or feature state is stale. |
| `prepare-commit-msg` | Can edit message before user sees it; not skipped by `--no-verify`. | Add feature/context trailers or preserve them on amend. |
| `commit-msg` | Can edit/validate message; skipped by `--no-verify`. | Validate tpatch trailers and prevent malformed metadata. |
| `post-commit` | Notification after commit; cannot affect outcome. | Record/condense context and update patch generation metadata. |
| `post-rewrite` | Runs after amend/rebase. | Remap feature commit/context links after rewrites. |
| `pre-push` | Can abort push, sees pushed refs on stdin. | Warn if context/patch metadata branch is not pushed or if unverified patches are included. |
| `post-checkout` / `post-merge` | Cannot affect outcome. | Restore active feature/session hints or warn on stale context. |

Guardrail: tpatch should not silently install hooks by default. Use opt-in,
visible installation and reversible hook chaining.

Amendment behavior:

- `git commit --amend` replaces the tip commit with a new commit object that has
  the amended tree/message. This is direct mutation of current history.
- `git commit --fixup=<commit>` creates a new `fixup!` commit meant to be
  autosquashed into the target later.
- `git commit --fixup=amend:<commit>` creates an `amend!` commit that can also
  replace the target commit message during autosquash.
- `git commit --squash=<commit>` creates a `squash!` commit intended to combine
  content and commit messages during interactive rebase.
- Git notes can add metadata to commits without changing the commit objects, but
  notes require their own propagation/merge policy.

So Git exposes **both models**: rewrite the commit now, or add a follow-up
fixup/squash commit and fold it later.

### 3.4 Entire: Git-native agent context checkpoints

Entire is now verified as a specific tool: <https://entire.io/> and
<https://github.com/entireio/cli>.

From its website and README:

- Entire hooks into Git workflow to capture AI agent sessions on every push.
- It supports Claude Code, Gemini CLI, Cursor, OpenCode, GitHub Copilot CLI,
  FactoryAI, and preview Codex support.
- It creates checkpoints for every commit: the code change paired with the agent
  session that produced it.
- User commits stay clean; session metadata is stored separately on
  `entire/checkpoints/v1`.
- Commits link to metadata through `Entire-Checkpoint: <id>` trailers.
- Sessions include prompts, responses, files touched, token usage, tool calls,
  and timestamps.

Architecture details from Entire docs:

| Concept | Entire model | tpatch lesson |
|---|---|---|
| Session | Unit of work with ID, description, strategy, start time, checkpoints. | tpatch could add an active feature/session object independent of final patch. |
| Temporary checkpoint | Full state snapshot on a shadow branch. | Useful for rewind, but risky because it may store raw source blobs. |
| Committed checkpoint | Metadata on `entire/checkpoints/v1`, sharded by checkpoint ID. | Separate metadata branch avoids polluting active branch. |
| Checkpoint ID | 12-hex ID linked from commit trailer and metadata tree. | tpatch patch generations/context summaries need stable IDs. |
| Git hooks | `prepare-commit-msg`, `commit-msg`, `post-commit`, `post-rewrite`, `pre-push`. | Same hook set is a strong candidate for tpatch capture guards. |
| Agent hooks | Agent-specific hook configs in `.claude`, `.codex`, `.github/hooks`, `.cursor`, `.gemini`, `.opencode`, etc. | tpatch can define an agent-neutral event protocol instead of IDE-specific core code. |
| External plugin protocol | `entire-agent-<name>` binaries over JSON stdin/stdout with capabilities. | A tpatch event protocol could be executable-based and optional. |
| Redaction | Always-on secret scanning, optional PII redaction. | Context capture must be privacy-first and explicit. |
| Separate remote | Optional checkpoint remote for private metadata. | tpatch may need a private metadata remote if context is sensitive. |

Entire's external agent protocol is especially relevant. It normalizes:

- session start/end and turn start/end events;
- prompt text;
- model;
- tool use IDs and tool input;
- modified/new/deleted files;
- transcript refs;
- token usage;
- subagent start/end and aggregate token usage;
- resume commands.

Strong tpatch adaptation:

> Do not require every agent to write `apply-recipe.json` directly. Let agents
> emit an append-only, agent-neutral event stream; `tpatch record` can compile
> claims, events, recipe operations, and Git diffs into patch-generation
> metadata.

What not to copy blindly:

- Full raw transcript storage by default. tpatch's secret-by-reference posture
  should prefer summarized context and hashed references unless the user opts in.
- Automatic push of metadata branches for every repo. tpatch should treat this
  as an explicit team decision.
- Attribution percentages as truth. Entire itself documents heuristics for
  agent-vs-human line attribution; tpatch should treat attribution as evidence,
  not authority.

Amendment behavior:

- Entire does not define patch content, so it does not answer "change the main
  patch or add a patch?"
- It does handle Git history rewrites: its managed hooks include `post-rewrite`
  and its docs discuss preserving/restoring checkpoint links during
  `git commit --amend`.
- The important idea is to keep context linkage durable across rewrites. If a
  commit changes SHA, the context link must be remapped or reattached.

So Entire's lesson is **preserve provenance around amendments**, not choose the
patch amendment semantics.

### 3.5 Aider: auto-commit and dirty-file separation

Aider integrates tightly with Git:

- It auto-commits AI edits by default.
- It commits preexisting dirty user files before applying agent edits, keeping
  human and agent work separated.
- `/diff` shows changes since the last user message.
- `/undo` can undo the last Aider commit.
- `/add` selects files into chat context, separate from Git staging.
- Commit messages are generated from diffs plus chat history.
- Commit attribution can mark author/committer names or add trailers.

tpatch translation:

- A "claim" is not the same as "file in context"; tpatch should distinguish
  **read context** from **write scope**.
- Dirty-file protection is important before an agent writes.
- Per-turn diffs are useful for recipe operation provenance.
- Auto-commits are not necessarily right for tpatch, but lightweight per-turn
  checkpoints could provide similar undo/audit value.

Amendment behavior:

- Aider's default mode creates additional Git commits as it edits.
- `/undo` discards the last Aider-made commit/change when an AI edit went
  sideways.
- Users can continue with a new correcting commit or clean history with normal
  Git tools.

So Aider favors **small forward commits plus undo**, rather than a patch-stack
refresh model.

## 4. Amendment model comparison

When the first patch is wrong or incomplete, tools choose among four broad
models:

| Model | Used by | Behavior | Strength | Risk |
|---|---|---|---|---|
| Rewrite current patch | Quilt `refresh`, StGit `refresh`, Git `commit --amend` | Replace the managed patch/commit with a new version. | Clean current logical patch. | Loses visible generation history unless logs/backups preserve it. |
| Append fixup patch/commit | Quilt `refresh -z`, Git `fixup!` / `squash!`, Aider follow-up commits | Add a correcting patch/commit after the original. | Preserves history and review trail. | Replay/reconcile must know whether fixup is separate or folded. |
| Fork/copy then amend | Quilt `fork`, Git branches, tpatch could do patch generations | Preserve original and edit a copy. | Safe experimentation. | More objects to manage and name. |
| Metadata-only amendment | Git notes, Entire checkpoint remap | Change context/provenance without changing code bytes. | Does not rewrite code history. | Metadata propagation can drift from code if not linked tightly. |

tpatch should not force one model globally. It likely needs:

1. **Canonical current patch**: `artifacts/post-apply.patch` remains the current
   replay authority.
2. **Append-only generations**: every record/amend writes a patch-generation
   manifest entry with parent generation, hashes, claims, capture mode, and
   context summary ID.
3. **Optional fixup generations**: a user can record a correction as either
   folded into the canonical patch or as a separate fixup feature/generation.
4. **Explicit fold/squash action**: if a fixup is no longer conceptually
   separate, tpatch can fold it into the feature's current canonical patch while
   preserving the old generations.
5. **Rewrite-aware context links**: if Git commits are amended/rebased, tpatch
   needs to remap trailers/context IDs instead of losing provenance.

Potential vocabulary:

| Term | Meaning |
|---|---|
| `record` | Capture current feature bytes into the canonical patch and append a generation. |
| `amend` | Replace the current canonical patch with a corrected generation. |
| `fixup` | Add a dependent corrective generation/feature that can later be folded. |
| `fold` / `squash` | Combine a fixup into the canonical current feature while preserving generation history. |
| `fork` | Copy a feature/patch generation to experiment without mutating the original. |

## 5. Proposed tpatch capture ladder

tpatch should separate capture into layers:

| Layer | Question answered | Candidate artifact |
|---|---|---|
| Intent | Why are we doing this? | `request.md`, `spec.md`, context summary. |
| Read context | What did the agent/user inspect? | Agent/IDE event log, references, hashes. |
| Write claim | What is allowed to change? | `claims.json` or patch-generation manifest section. |
| Edit provenance | Who/what changed it and when? | Agent event log, prompt/turn IDs, tool IDs. |
| Recipe intent | What deterministic operations were intended? | `apply-recipe.json` with op IDs/read-write sets. |
| Git boundary | What bytes are in the feature? | working tree, staged index, commit range. |
| Patch generation | What is the durable moving identity? | patch-generation manifest. |
| Context summary | What should future humans/agents remember? | redacted/summarized context artifact. |

## 6. Candidate tpatch design direction

### 6.1 Keep current `record` as fallback

Default `tpatch record <slug>` remains valuable because it is zero setup and
works without hooks or agents.

### 6.2 Add explicit claims

Candidate behavior:

```text
tpatch claim <slug> src/foo.go docs/foo.md
tpatch record <slug> --claimed-only
```

Modes:

| Mode | Behavior |
|---|---|
| advisory | `record` captures all changes but warns on unclaimed paths. |
| strict | `record` refuses unclaimed paths. |
| staged | Git index defines the boundary. |
| claimed-only | Claims define the boundary. |
| hybrid | Captured paths must be both claimed and staged. |

### 6.3 Add active feature sessions

Candidate artifact:

```json
{
  "session_id": "2026-05-11-<uuid>",
  "feature": "fix-model-id-translation",
  "base_commit": "<sha>",
  "started_at": "2026-05-11T20:00:00Z",
  "claims": ["src/foo.go"],
  "agents": ["copilot-cli"],
  "state": "active"
}
```

This gives IDEs, agents, and hooks a single answer to "which feature is being
edited right now?"

### 6.4 Add optional agent event log

Borrow from Entire's protocol shape, but keep it tpatch-specific and minimal:

```json
{
  "event_id": "evt-...",
  "session_id": "session-...",
  "feature": "slug",
  "type": "turn_end",
  "agent": "copilot-cli",
  "model": "gpt-5.5",
  "prompt_ref": "sha256:...",
  "tool_use_id": "toolu_...",
  "files_modified": ["src/foo.go"],
  "tests": [{"command": "go test ./...", "result": "pass"}],
  "summary": "Adjusted model lookup fallback."
}
```

Raw prompts/transcripts should be opt-in. Default should be summaries,
references, hashes, and file lists.

### 6.5 Add Git hook guards, not silent automation

Potential hook behavior:

- `pre-commit`: warn/refuse if staged paths are not claimed by any active
  feature.
- `prepare-commit-msg`: add `Tpatch-Feature` and context IDs.
- `commit-msg`: validate trailers.
- `post-commit`: compile patch-generation/context summary.
- `post-rewrite`: remap context IDs after amend/rebase.
- `pre-push`: warn if context branch/manifests are stale or unpushed.

Default should be opt-in and reversible.

## 7. Implications for queued PRDs/ADRs

The earlier metadata PRDs become stronger with this capture research:

| Existing candidate | Capture-front addition |
|---|---|
| `PRD-feature-patch-identity-metadata` | Include capture mode, claim set, context summary ID, agent session IDs. |
| `ADR-patch-generation-manifest-boundary` | Decide whether event logs and context summaries live per feature, repo-wide, or on a metadata branch. |
| `ADR-patch-amendment-policy` | Decide when tpatch rewrites canonical patch, appends fixups, forks, or only amends metadata. |
| `PRD-recipe-operation-identity` | Link operation IDs to agent events/tool calls and claimed read/write scope. |
| `PRD-structural-anchor-manifest` | Generate anchors at record time from claimed paths and operation scopes. |
| `PRD-patch-vector-index` | Index context summaries and prompts only if privacy boundary allows it. |

New capture-front docs to add:

| Candidate | Scope |
|---|---|
| `PRD-feature-file-claims` | Explicit path/symbol claims and strict/advisory capture behavior. |
| `PRD-record-capture-modes` | `--staged`, `--unstaged`, `--all`, `--claimed-only`, `--from`, and combinations. |
| `PRD-active-feature-session` | Session state that agents, IDEs, hooks, and `record` can share. |
| `PRD-agent-event-log` | Agent-neutral JSONL event schema. |
| `PRD-ide-capture-hooks` | Optional editor event ingestion. |
| `PRD-git-hook-capture-guards` | Opt-in hook installation, warnings, trailers, and rewrite handling. |
| `ADR-capture-context-privacy-boundary` | Raw transcript vs summary vs hash/reference rules. |
| `PRD-record-context-summary` | Privacy-safe per-generation context summary artifact. |
| `ADR-capture-metadata-branch` | Whether tpatch should use a separate metadata branch like Entire. |
| `PRD-feature-patch-amend` | CLI/user workflow for amend, fixup, fold/squash, fork, and generation history. |

## 8. Recommended near-term order

1. **ADR-capture-context-privacy-boundary** — decide what tpatch may store before
   designing any agent/IDE capture.
2. **ADR-patch-amendment-policy** — decide how tpatch represents wrong or
   incomplete first patches: rewrite canonical, append fixup, fork, fold, or
   metadata-only correction.
3. **PRD-feature-file-claims** — smallest high-value capture improvement.
4. **PRD-record-capture-modes** — align claims with staged/unstaged/commit-range
   record paths.
5. **PRD-active-feature-session** — shared coordination point for hooks/agents.
6. **PRD-agent-event-log** — optional provenance stream.
7. **PRD-git-hook-capture-guards** — opt-in warnings/trailers/condensation.
8. **ADR-capture-metadata-branch** — decide whether to keep richer context in
   `.tpatch/`, Git notes, or a separate metadata branch.

## 9. References

- Entire website: <https://entire.io/>
- Entire launch post: <https://entire.io/blog/hello-entire-world>
- Entire CLI repo: <https://github.com/entireio/cli>
- Entire README sections on sessions, hooks, configuration, and security.
- Entire docs:
  - `docs/architecture/sessions-and-checkpoints.md`
  - `docs/architecture/external-agent-protocol.md`
  - `docs/architecture/agent-integration-checklist.md`
  - `docs/architecture/attribution.md`
  - `docs/security-and-privacy.md`
  - `docs/KNOWN_LIMITATIONS.md`
- Quilt manual: <https://manpages.debian.org/testing/quilt/quilt.1.en.html>
- StGit homepage: <https://stacked-git.github.io/>
- Git hooks: <https://git-scm.com/docs/githooks>
- Git commit amend/fixup/squash: <https://git-scm.com/docs/git-commit>
- Git notes: <https://git-scm.com/docs/git-notes>
- Git trailers: <https://git-scm.com/docs/git-interpret-trailers>
- Aider Git docs: <https://aider.chat/docs/git.html>
- Aider commands: <https://aider.chat/docs/usage/commands.html>

## Open questions

- Should tpatch ever store raw agent transcripts, or should the default be
  summaries plus hashes/references only?
- Should the feature claim unit be path, glob, symbol, hunk, operation, or all
  of the above?
- Should `record --staged` be the first implementation of precise capture?
- Should active feature sessions live in `.git/`, `.tpatch/`, or a separate
  metadata branch?
- Should tpatch install Git hooks, generate hook snippets, or only validate when
  invoked directly?
- Can tpatch support concurrent feature sessions safely without adding a lock
  model?
- Is a separate metadata branch worth the complexity for tpatch, or is a
  `.tpatch/context/` tree enough?
- Should tpatch expose separate `amend`, `fixup`, `fold`, and `fork` concepts,
  or keep them as record modes?
- Should fixups be first-class features, patch generations, or both?
- When a feature's canonical patch is amended, what must happen to dependent
  features that were recorded against the old generation?

## Disputes

None logged.
