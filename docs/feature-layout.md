# Feature Layout — What lives under `.tpatch/features/<slug>/`

Every feature managed by tpatch has a directory at `.tpatch/features/<slug>/`. This doc is the authoritative map of what each file is, who writes it, and — most importantly — **which file is canonical vs. audit-trail** when you want to replay or reason about the feature.

The #1 confusion, observed in live stress testing, is that users see many numbered patches under `patches/` and wonder which one is current. Short answer: **none of them — use `artifacts/post-apply.patch`.** Long answer below.

## At a glance

```
.tpatch/features/<slug>/
├── request.md              ← user's original request, verbatim
├── analysis.md             ← LLM / heuristic analysis output (phase: analyze)
├── spec.md                 ← acceptance criteria, plan (phase: define)
├── exploration.md          ← file-level investigation (phase: explore)
├── record.md               ← human-readable summary of the last record run
├── status.json             ← machine state (state, last_command, timestamps, apply.*)
├── artifacts/
│   ├── apply-recipe.json   ← operation list (phase: implement)
│   ├── post-apply.patch    ★ CANONICAL feature diff, always-current
│   ├── incremental.patch   ← (optional) delta between two post-apply snapshots
│   ├── post-apply-diff.txt ← `git diff --stat` of the recorded patch
│   ├── raw-*-response-*.txt← LLM raw responses for debugging
│   ├── intent-archive/     ← (optional) prior intent bytes from regenerate
│   │   ├── index.json      ← content-addressed generations and tombstones
│   │   └── blobs/<sha256>.blob ← immutable while retained; removable by purge
│   ├── resources.json      ← (optional) typed resource declarations
│   └── resource-captures/  ← (optional) immutable resource capture set
│       ├── batches/<batch_id>.json  ← one file per DISTINCT content
│       └── current.json             ← the only file readers consult
└── patches/
    ├── 001-started.patch   ← HISTORICAL full-diff snapshots, append-only
    ├── 002-record.patch    ← each file is a *full* diff at write-time,
    ├── 003-record.patch    ← NOT incremental. Highest number = latest.
    └── …
```

Legend: **★** = canonical; **←** = lifecycle / debug; anything under `patches/` = audit trail only.

## Intent-bundle inspection

`tpatch prepare <slug> --check` classifies four intent artifacts:
`analysis.md`, `spec.md`, `exploration.md`, and
`artifacts/analysis.json`. The three Markdown documents are the structural
readiness set. The JSON analysis sidecar is optional and is written by the
CLI-driven analyze path, so its absence does not block readiness for a
hand-authored Path B feature.

The full `prepare` surface is optional and remains outside lifecycle phase
ordering. Default generate fills only a dependency-coherent missing suffix,
`--manual` adopts a complete hand-authored bundle, and `--regenerate` replaces
the complete bundle while preserving eligible prior bytes in the intent
archive. Regenerate requires a configured successful provider unless
`--allow-heuristic` is explicit.

## `artifacts/intent-archive/` — retained bytes, not feature truth

Only `tpatch prepare <slug> --regenerate` can create this directory, and only
when at least one existing intent artifact is replaced:

```text
artifacts/intent-archive/
├── index.json
└── blobs/
    └── <64-lowercase-hex>.blob
```

Each blob is named by the SHA-256 of its exact prior bytes. Blob content is
immutable while retained: identical content reuses the existing blob rather
than rewriting it. Blobs are nevertheless removable through the purge command.
`index.json` records content-addressed generations, retained references,
removal-pending references, and tombstones. It is rewritten as one canonical
JSON file; the directory is not a chronology.

This archive is **never canonical patch, lifecycle, or provenance truth**. It
does not identify an author, Path A/Path B, provider, or model, and it does not
certify semantic quality. It is not a general history or undo facility. Its
one purpose is exact byte recovery while a blob remains retained:

```bash
cp .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob \
   .tpatch/features/<slug>/spec.md
```

Inspect and bound retention with:

```bash
tpatch feature intent-archive list <slug>
tpatch feature intent-archive purge <slug> --blob <hash> --yes
tpatch feature intent-archive purge <slug> --generation <generation-id> --yes
tpatch feature intent-archive purge <slug> --orphans --yes
tpatch feature intent-archive purge <slug> --all --yes
```

`purge` is preview-by-default; omit `--yes` to inspect the plan. Confirmed
purges mark references removal-pending before removal and then tombstone them.
An orphan is a blob with no live reference. A dangling retained reference has
one tpatch repair: confirmed `--blob <hash> --yes`, which tombstones every
reference to that absent hash without pretending the bytes were restored.
Unsafe or hash-wrong managed objects require the exact report-listed manual
prerequisite or restoration before a confirmed purge can proceed.

Before archiving, prior bytes pass the shared redaction policy. A match refuses
regeneration rather than retaining or scrubbing secret-shaped content; edit or
move the material and retry. When `.tpatch/` is tracked, `tpatch land` stages
the archive like other `artifacts/**` files. Once committed, deleting a blob
from the current tree does not remove it from Git history. When `.tpatch/` is
untracked, the archive does not survive a fresh clone and may be removed by
`git clean -fd` or `git clean -xfd`.

Transaction staging, journal, metadata preimages, and abandoned evidence live
under gitignored `.tpatch/local/intent-prepare/<slug>/`, not in the archive.
The journal is bounded undo evidence for one interrupted publication. If it is
lost, ordinary partial canonical bytes cannot reliably prove that a transaction
existed; the archive is not a substitute transaction log.

## Canonical vs. audit trail

### `artifacts/post-apply.patch` — use this one

`tpatch record` writes this file on every invocation, overwriting the previous contents. It is *always* the current full diff of the feature against the baseline commit recorded in `status.json:apply.base_commit`.

**Replay path:**

```
git apply .tpatch/features/<slug>/artifacts/post-apply.patch
```

`tpatch reconcile` reads this file to decide what to re-apply onto a new upstream. Downstream tooling (CI/CD, scripts, agents) should treat this as the source of truth for "what does this feature do?".

### `artifacts/incremental.patch` — sometimes present

Set by the apply flow when a started/done pair produces a delta that differs from the full diff (see `DeriveIncrementalPatch` in `internal/gitutil/`). Reconcile uses it in preference to `post-apply.patch` when both exist and the delta is smaller. You can ignore it for day-to-day work — it's an optimisation detail.

### `patches/NNN-<label>.patch` — audit trail, not replay input

Every time `tpatch record` (or certain apply modes) runs, it appends a numbered snapshot here via `Store.NextPatchNumber` (scan the directory, take max+1). The labels you'll see in the wild:

| Label | Written by | Meaning |
|---|---|---|
| `record` | `tpatch record` | Full feature diff at record time |
| `started` | `tpatch apply --mode started` | Diff captured right before execute |
| `cycle` | `tpatch cycle` | Patch from a cycle run |
| `done` | `tpatch apply --mode done` | Diff captured after execute |

Each file is a **complete** diff of the feature vs baseline — not an incremental delta between `NNN` and `NNN-1`. They exist so you can audit history ("what did my feature look like three days ago?"), not so you can replay them in order. **Applying `patches/001-record.patch` replays a stale state** that is missing every amendment recorded after it.

Rule of thumb:

- **Replay or reconcile** → `artifacts/post-apply.patch`.
- **"What did the feature look like before amendment X?"** → `patches/<older-number>-*.patch`.
- **Pruning is safe** as long as you keep the latest numbered file and `artifacts/post-apply.patch`. A dedicated `tpatch patches <slug> --prune` subcommand is planned (see `feat-patches-subcommand`) — for now, `rm` the older files manually if the directory bothers you.

When `patches/` exceeds six files, `tpatch record` will print a one-line reminder so you don't have to memorise this doc.

## Lifecycle files

These are written once or twice per feature, by named phases:

| File | Written by | Purpose |
|---|---|---|
| `request.md` | `tpatch add` | The user's original prompt, stored verbatim for context. |
| `analysis.md` | `tpatch analyze` | LLM's (or heuristic's) classification + risk rating. |
| `spec.md` | `tpatch define` (alias: `tpatch spec`) | Acceptance criteria + phased plan. Drives implement. |
| `exploration.md` | `tpatch explore` | Target files + existing-code facts. Grounds implement. |
| `apply-recipe.json` | `tpatch implement` | Operation list (create/modify) the apply flow executes. |
| `record.md` | `tpatch record` | Human-readable summary of the last record run. |

## Typed resources (optional, v0.15.0)

`artifacts/resources.json` and `artifacts/resource-captures/` appear only
once a feature declares a typed resource with
[`tpatch feature resource add`](../SPEC.md). They are **audit sidecars**:
nothing in this pair is canonical patch or lifecycle truth, and neither
is ever read by `apply`, `reconcile`, `land` or the state machine.

- `resources.json` is the declaration manifest. It is written only by
  `add`/`remove`/`clear`/`trust-dolt`, never by a capture.
- `resource-captures/` is an unordered, content-addressed **set**, not a
  chronology. A `batches/<batch_id>.json` file names exactly one distinct
  piece of content, so re-capturing unchanged state writes zero new bytes
  and only rewrites the pointer; reverting content back to a previously
  captured state repoints `current.json` at the batch that already exists.
- `current.json` is the only file a reader consults. Do not scan
  `batches/` to infer state — an orphaned batch that no pointer entry
  references is a normal, permanent artifact of a crash window.
- No file in either artifact ever contains raw file bytes, raw adapter
  output, or a wall-clock timestamp.

Ephemeral control state lives outside the tracked tree, under the
gitignored `.tpatch/local/resource-scratch/<slug>/`: a persistent,
zero-length `.lock` file that is never deleted, plus one `es_<12hex>/`
directory per in-flight invocation that is removed on both the success
and failure paths.

## State & debug files

- `status.json` — authoritative machine state. Fields include `state`, `last_command`, `apply.has_patch`, `apply.base_commit`, and timestamps. Only `tpatch` writes this; editing it by hand is unsupported.
- `artifacts/raw-*-response-*.txt` — one file per LLM call. Inspect when an agent did something surprising; these are what `tpatch implement` hands back to `JSONObjectValidator`.
- `artifacts/post-apply-diff.txt` — `git diff --stat` output for quick eyeballing.

## Feature ↔ commit binding (`Tpatch-Feature` trailer)

When a feature is landed via [`tpatch land <slug>`](./land.md), the resulting Git commit carries a four-trailer block whose first line is `Tpatch-Feature: <slug>`. That trailer is the **sole** feature↔commit binding in tracked Git state — `git log --grep '^Tpatch-Feature: <slug>$'` enumerates every commit that lands `<slug>`. Notably, `status.json:apply.base_commit` is **not** rewritten with the new HEAD; that field stays owned by `record` / auto-base resolution (a commit cannot embed its own SHA in tracked content).

The full schema (four trailers, ordering, and the additive `Tpatch-CVE` reservation for hotfix) is locked in [`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`](./adrs/ADR-019-tpatch-land-trailer-block-schema.md). See [`docs/land.md`](./land.md) for the operator-facing contract.

## Related

- [Recording Patches](./record.md) — when and how to run `tpatch record` (plus the anti-pattern refusal).
- [Landing Features as Git Commits](./land.md) — `tpatch land`, the trailer-block producer.
- [`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`](./adrs/ADR-019-tpatch-land-trailer-block-schema.md) — locks the four-trailer schema.
- [`docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`](./prds/PRD-feature-resource-claims-and-capture-adapters.md) — the typed-resource contract.
- [`docs/adrs/ADR-033-resource-capture-boundary.md`](./adrs/ADR-033-resource-capture-boundary.md) — the resource capture boundary decisions.
- `SPEC.md` — authoritative CLI surface and state machine.
- `AGENTS.md` — file ownership matrix for the implementation team.
