# PRD — Artifact Validation and Provenance — `tpatch prepare <slug> --check`

**Status**: Draft — Awaiting Review (rev-0)
**Date**: 2026-08-13
**Owner**: Core (planning lane)
**Byline**: writer sub-agent, dispatched from `WAVE_BASE` `0aa0d95`
**Milestone**: TBD — this document ships no code
**Issue**: [GH #10 — define truthful intent-artifact validation and provenance](https://github.com/tesseracode/tesserapatch/issues/10)
**Graduates from**: [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md), Turns 2–4
**Blocks**: `PRD-prepare-intent-bundle.md` — **that PRD remains blocked until this
one is accepted.** See §20.

## Related

- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md) — `## Agreed — Turns 2–3` and §6.2
- [WP-005 turn log](../whitepapers/WP-005-spec-driven-workflows.turns.md) — Turns 2, 3 and 4
- [Agent as Provider — Path B workflow](../agent-as-provider.md) — the phase → artifact → state contract
- [Path B Operator Guide](../path-b-operator-guide.md) — the hand-authored artifact set
- [Feature Layout](../feature-layout.md) — canonical vs audit-trail files under `.tpatch/features/<slug>/`
- [PRD-verify-freshness](./PRD-verify-freshness.md) — the freshness overlay whose `intent_files_present` check this PRD deliberately does **not** replace
- [PRD-tpatch-doctor](./PRD-tpatch-doctor.md) — workspace metadata drift, the adjacent read-only diagnostic
- [PRD-rejected-feature-state](./PRD-rejected-feature-state.md) — precedent for closed enums, evidence handling and per-command exit envelopes
- [ADR-013 verify freshness overlay](../adrs/ADR-013-verify-freshness-overlay.md) — precedent: a read surface that is not a lifecycle transition
- [ADR-027 capture context privacy boundary](../adrs/ADR-027-capture-context-privacy-boundary.md) — D2 (no raw context), D6 (no wall-clock in determinism)
- [ADR-031 rejected feature state data model](../adrs/ADR-031-rejected-feature-state-data-model.md) — D1 sub-record-on-`FeatureStatus` precedent, weighed in §11
- [ADR-033 resource capture boundary](../adrs/ADR-033-resource-capture-boundary.md) — D10 (no tracked timestamps), D11 (no Go map in a wire schema)

## Summary

`tpatch` cannot currently answer, truthfully and in one place, the question
**"which intent artifacts does this feature actually have, and where did they
come from?"** Today the closest answers are wrong in different directions:
`--manual` accepts a zero-byte `spec.md` (`internal/store/manual.go:51-78`),
`next` infers a lifecycle sub-state from bare `os.Stat` presence
(`internal/cli/phase2.go:437-446,555-558`), and `verify`'s
`intent_files_present` refuses to run at all before apply
(`internal/workflow/verify.go:245-252`).

This PRD specifies exactly one new product surface:

```text
tpatch prepare <slug> --check [--json] [--quiet] [--path <dir>]
```

It is **read-only**. It calls no provider, writes no byte anywhere in the
repository, advances no state, and adds no lifecycle state. It classifies four
canonical artifacts into a closed structural vocabulary, derives a
structural-readiness verdict from the two required ones, reports
`provenance: unknown` for every artifact because no durable per-artifact source
metadata exists yet, and exits with a per-command code contract.

It also makes one decision the WP-005 Turn 4 review demanded explicitly
(§12): **the existing mutating `analyze|define|explore --manual` gates do not
change in this slice.** The inspector is a pure, shared function, but it is
wired to `prepare --check` and nothing else.

## 1. Problem statement

### 1.1 Structural presence is checked inconsistently, and never as a set

`AdvanceStateManually` is the single source of truth for `--manual`
(`internal/store/manual.go:27-32,51-81`). It:

- resolves a fixed per-phase artifact path,
- refuses when the path does not exist,
- refuses when the path is a directory,
- validates JSON **only** for `implement` (`ValidateJSON` is set on that row
  alone, `internal/store/manual.go:31`),
- and therefore **accepts a zero-byte or whitespace-only `analysis.md`,
  `spec.md` or `exploration.md`** and advances lifecycle state on it.

It also uses `os.Stat` (`internal/store/manual.go:57`), which follows symlinks. A `spec.md`
symlinked to a file outside the repository satisfies the current gate.

### 1.2 Path A and Path B produce different artifact sets

Path A's `RunAnalysis` writes **two** files: the structured sidecar
`artifacts/analysis.json` and the human `analysis.md`
(`internal/workflow/workflow.go:89-97`). `RunDefine` reads the sidecar
opportunistically and ignores the error when it is missing
(`internal/workflow/workflow.go:117-121`). Path B — `analyze <slug> --manual` —
requires only `analysis.md` (`internal/store/manual.go:28`) and writes no sidecar.

So a Path B feature legitimately has no `artifacts/analysis.json`. Any check
that treats the sidecar as mandatory would declare every agent-authored feature
defective; any check that ignores it entirely would hide a real Path A/Path B
asymmetry from operators.

### 1.3 Provenance is not durable

`FeatureStatus` has exactly one free-text `Notes` field
(`internal/store/types.go:215`), and `MarkFeatureState` **overwrites** it on
every transition (`internal/store/store.go:388-392`). `--manual` writes
`"Phase advanced manually (--manual); artifact authored at <path>"`
(`internal/store/manual.go:79`); the very next `define` overwrites it with
`"Acceptance criteria and plan generated"`
(`internal/workflow/workflow.go:155`). A single string that is destroyed by the
next phase cannot prove **per-artifact** authorship.

The one existing per-artifact sidecar, `artifacts/recipe-provenance.json`
(`internal/workflow/implement.go:18-34,222-238`), does not close this gap
either: it records `base_commit`, `generated_at` and `recipe_sha256` — commit
and content anchors, not *source of authorship* — it covers only
`apply-recipe.json`, it is written only by the Path A `RunImplement` path, and
it is skipped entirely outside a Git repository (`internal/workflow/implement.go:222`).

### 1.4 A mutating `prepare` would amplify all three

WP-005 Turn 2 concluded that `prepare` is a valid product seam but not an
implementation authorization, and that a mutating bundle must not be built on
top of validation this weak
(`docs/whitepapers/WP-005-spec-driven-workflows.md:59-64,75-81`). This PRD is
the prerequisite contract.

## 2. Goals / Non-goals

### 2.1 Goals

1. Ship one read-only command, `tpatch prepare <slug> --check`, that reports
   the structural state of the canonical intent artifacts for one feature.
2. Define a **closed, deterministic** structural vocabulary and a total
   classification function over it: every reachable filesystem condition maps
   to exactly one state.
3. Define required vs optional artifacts, and derive structural readiness from
   the required set only.
4. Report the Path A analysis sidecar **separately**, never as a readiness
   input, and never as evidence of which path produced the feature.
5. Emit `provenance: unknown` for every artifact in v1, and forbid inference
   from notes, filenames, sidecar presence, timestamps or content.
6. Define the human and JSON output shapes, key order, exit codes and error
   precedence exactly enough to test byte-stably.
7. Guarantee zero mutation, bounded reads, no symlink following, no content
   echo and no wall-clock in output.
8. Keep every existing behavior — `defined` semantics, `analyze|define|explore
   [--manual]`, `implement`, `cycle`, `next`, `verify`, `status`, `doctor`,
   `apply`, `record`, worktree handling, the Git index — bit-for-bit
   compatible.
9. Decide the WP-005 Turn 4 question explicitly, in acceptance criteria, not in
   open questions (§12).

### 2.2 Non-goals

1. **No mutating `prepare`.** Plain `tpatch prepare <slug>` is refused (§5.3).
2. **No `--manual` bundle adoption and no `--regenerate`.** Those flags are not
   registered in v1 (§5.4).
3. **No provider call.** `prepare --check` never constructs a provider, never
   reads provider config for network use, and never emits a token.
4. **No new `FeatureState`, no lifecycle transition, no status/notes/labels/
   index/artifact write** (§13).
5. **No semantic quality judgement.** The report never asserts that an artifact
   is good, complete, sufficient, well-formed prose, or correct (§7.6).
6. **No persistent provenance representation is selected** in rev-0 (§11.4).
7. **No routing change.** `next` and `cycle` do not consult the inspector
   (§13.2). WP-005 Agreed item 9 (`docs/whitepapers/WP-005-spec-driven-workflows.md:89-92`).
8. **No downstream methodology mandate.** Running `prepare --check` is never
   required to use tpatch.
9. **No repair, no `--fix`, no backfill, no migration of existing features.**

## 3. Terminology

| Term | Definition in this PRD |
|---|---|
| **Intent artifact** | One of the four canonical files in §6.1. Nothing else. |
| **Canonical path** | The fixed, non-configurable repo-relative path of an intent artifact. Not user-supplied, not derived from flags. |
| **Structural state** | One value of the closed enum in §7.2. A fact about the file's existence, kind and byte shape. Never a statement about meaning. |
| **Required artifact** | An artifact whose structural state participates in the readiness verdict (§6.2). |
| **Optional artifact** | An artifact that is reported but can never change the readiness verdict (§6.2). |
| **Structural readiness** | The three-valued verdict `ready` / `not_ready` / `indeterminate` over the required set only (§9.1). |
| **Provenance** | The *source of authorship* of one artifact — provider-generated, agent-authored, human-authored, or unknown. v1 emits only `unknown` (§11). |
| **Capture** | The single per-artifact read performed by the inspector. Classification reads the capture, never the file a second time (§8.2). |
| **Snapshot instability** | An observation that the artifact changed identity or size across its own capture window (§8.3). |
| **Semantic quality** | Anything about the artifact's content beyond "at least one non-whitespace byte exists" and, for the sidecar only, "these bytes parse as a JSON object". Out of scope in v1 and every acceptance row. |

## 4. Existing-primitives preflight

WP-005 §6.2 requires this section
(`docs/whitepapers/WP-005-spec-driven-workflows.md:504-518`). Each row below is
a bounded, cited reason the existing primitive cannot answer the read-only
bundle question.

| Primitive | What it does today | Why it does not solve this |
|---|---|---|
| `analyze\|define\|explore --manual` | Mutating single-artifact adoption: existence + not-a-directory, JSON only for `implement` (`internal/store/manual.go:27-32,51-81`). | Mutates state as a side effect of asking. One artifact per invocation, no bundle view. Cannot report `analysis.json`. Uses `os.Stat`, so it follows symlinks. Accepts whitespace-only Markdown. Emits no machine-readable report and no per-artifact exit contract. |
| `cycle` | Runs analyze → define → explore → implement → apply → record, calling the provider at each phase (`internal/cli/phase2.go:25-145`); `--skip-execute` stops after recipe generation (`internal/cli/phase2.go:122-127`). | Maximally mutating and provider-driven. Its `assertCycleState` calls read state *after* a write. There is no inspect-only entry point. |
| `next` | Emits one `HarnessTask` and distinguishes the two `defined` sub-states from raw `exploration.md` presence (`internal/cli/phase2.go:437-446`) using `fileExistsAt`, an `os.Stat` wrapper (`internal/cli/phase2.go:555-558`). | Answers "what do I run next", not "what do I have". `os.Stat` cannot distinguish empty from non-empty, symlink from regular, or unreadable from absent. Output is a task envelope with `--format text\|harness-json`, not an artifact report. No exit-code contract. |
| `verify` | `intent_files_present` checks `spec.md` **and** `exploration.md` exist and have non-zero size (`internal/workflow/verify.go:413-439`). | Refuses every pre-apply lifecycle state before V1 ever runs (`internal/workflow/verify.go:245-252`), which is precisely the window `prepare --check` serves. Omits `analysis.md` and `artifacts/analysis.json`. `info.Size() == 0` passes a whitespace-only file. `os.Stat` follows symlinks. Persists a `Verify` record unless `--no-write` (`internal/workflow/verify.go:347-352`), so it is a writer by default. Its 11-check report is a different contract with a different audience. |
| `status` | Lifecycle dashboard over `ListFeatures` (`internal/store/store.go:210-224`). | Renders `FeatureStatus` fields. It has never inspected artifact files, and adding artifact classification to it would change a shipped read surface's meaning for every feature at once. |
| `doctor` | D1–D8 workspace metadata drift; D1 validates `status.json` and flags legacy `feature.yaml` (`internal/workflow/doctor_d1.go:14-52`). | Scoped to metadata/manifest drift across the workspace, is `--fix`-capable (a writer), and reports findings, not a per-feature readiness verdict. No doctor check reads `analysis.md`, `spec.md`, `exploration.md` or `artifacts/analysis.json`. |
| `safety.EnsureSafeRepoPath` | Lexical containment only (`internal/safety/safety.go:12-28`), described as "the coarse pre-filter that runs before any Lstat of any component" (`internal/rescap/pathgate.go:50-54`). | Says nothing about symlinks, file kind, readability or size. It is one input to §7.3, not a substitute for it. |
| `rescap.GatePath` | Full five-step symlink/descriptor-identity gate (`internal/rescap/pathgate.go:68-83`). | Correct in spirit but scoped to the typed-resource capture domain, refuses on a *missing* path (`ReasonPathMissing`), and returns an open descriptor with a lock/scratch lifecycle. `prepare --check` must treat "missing" as an ordinary reportable state, not a refusal. §7.3 reuses the **policy**, not the entry point. |
| `store.WriteFeatureFile` / `WriteArtifact` / `writeFile` | The write side (`internal/store/store.go:443-449,461-472,918-922`). | Listed here only because `prepare --check` must call none of them (§10.5), and because `writeFile` → `os.WriteFile` truncates in place, which is the concurrency hazard §8.3 exists to classify honestly. |

**Conclusion.** No existing surface answers the pre-implementation,
read-only, four-artifact question. `prepare --check` is not an alias for an
existing stop point.

## 5. CLI grammar and surface boundary

### 5.1 Authorized grammar (v1, complete)

```text
tpatch prepare <slug> --check [--json] [--quiet] [--path <dir>]
```

- `<slug>` — exactly one, required.
- `--check` — **required** in v1. It is the only mode.
- `--json` — emit the structured report on stdout.
- `--quiet` — suppress the per-artifact human output.
- `--path` — the existing root persistent flag, inherited unchanged
  (`internal/cli/cobra.go:3782-3793`).

No other flag is registered. There is no `--all`, no `--fix`, no `--format`, no
`--timeout` (nothing can time out — no provider, no network, no subprocess).

### 5.2 Name collision with `apply --mode prepare` — accepted, mitigated

`tpatch apply --mode prepare` already exists and means "write the agent
packet, mark apply progress" (`internal/cli/cobra.go:822-824,840,857`;
`SPEC.md:81`). A new top-level `tpatch prepare` verb therefore collides
lexically with an existing, unrelated apply mode.

Alternatives weighed:

| Option | Verdict |
|---|---|
| `tpatch prepare <slug> --check` | **Chosen.** The name is the one WP-005 Turns 2–4 and GH #10 reason about; renaming it here would fork the vocabulary between the accepted paper trail and the shipped surface. |
| `tpatch intent check <slug>` | Rejected: introduces a third noun (`intent`) for a four-file set that `docs/agent-as-provider.md:40-45` already calls the phase→artifact contract, and orphans the WP-005 → PRD → issue naming chain. |
| Rename `apply --mode prepare` | Rejected outright: it is a shipped, skill-referenced surface; renaming it is a breaking change wholly unrelated to this PRD. |

Mitigation is mandatory, not optional:

1. `tpatch prepare --help` must state, verbatim, that it is unrelated to
   `tpatch apply --mode prepare`.
2. `tpatch apply --help`'s `--mode` description must gain the reciprocal
   pointer.
3. Both are acceptance rows (AVP-009, AVP-010).

### 5.3 `tpatch prepare <slug>` without `--check` — reserved-surface refusal

Plain `prepare` is the mutating bundle. It is **not authorized by this PRD**.
Invoking it refuses **before the store is opened**, so the refusal is
deterministic even outside a tpatch workspace:

```text
tpatch prepare requires --check in this release.

The mutating intent-bundle form of `prepare` is specified separately and is
not implemented. See docs/prds/PRD-prepare-intent-bundle.md (blocked) and
https://github.com/tesseracode/tesserapatch/issues/10.

Run: tpatch prepare <slug> --check
```

Exit code `4` (§9.2). Precedent for refusing a deliberately reserved surface
with a typed non-1 code rather than a cobra usage error: `amend --state`
(`internal/cli/c1.go:276-289`).

### 5.4 `--manual` and `--regenerate` — deliberately unregistered

Neither flag is registered on `prepare` in v1. Cobra therefore rejects them as
unknown flags and the process exits `1` (§9.2). This is deliberate:

- registering a flag that immediately refuses advertises a capability in
  `--help` that does not exist;
- an unknown-flag error is the honest signal that the surface is absent, not
  disabled.

A future PRD that registers either flag changes the exit code for those inputs
from `1` to whatever it specifies. That is an enumerated behavior delta which
that PRD must state; it is not a silent change this PRD authorizes.

### 5.5 Interaction with other commands

`prepare --check` does not appear in `cycle`, is not emitted by `next` as an
`OnComplete`, and is not a precondition for any command. Nothing calls it.

## 6. Canonical artifact set

### 6.1 The four artifacts

Fixed, non-configurable, repo-relative paths under
`.tpatch/features/<slug>/`:

| `id` | Canonical path | Written by | Role |
|---|---|---|---|
| `analysis` | `analysis.md` | `RunAnalysis` (`internal/workflow/workflow.go:96`) or Path B by hand (`internal/store/manual.go:28`) | required |
| `spec` | `spec.md` | `RunDefine` (`internal/workflow/workflow.go:151`) or Path B by hand (`internal/store/manual.go:29`) | required |
| `exploration` | `exploration.md` | `RunExplore` (`internal/workflow/workflow.go:196`) or Path B by hand (`internal/store/manual.go:30`) | optional |
| `analysis_sidecar` | `artifacts/analysis.json` | `RunAnalysis` only (`internal/workflow/workflow.go:89-90`) | optional |

`request.md`, `status.json`, `artifacts/apply-recipe.json`,
`artifacts/recipe-provenance.json`, `artifacts/post-apply.patch`, `record.md`,
`patches/**`, `artifacts/resources.json` and `artifacts/resource-captures/**`
are **out of scope**. `request.md` is an input, not an intent artifact;
everything else belongs to phases after `defined`
(`docs/feature-layout.md:10-33,88-97`). The set is closed for v1; adding to it
is a schema change (§10.4).

### 6.2 Required vs optional — and why

**Required: `analysis.md`, `spec.md`.** These are exactly the artifacts whose
presence advances a feature to `defined` under either path:
`analyze → analyzed` and `define → defined`
(`internal/store/manual.go:28-29`; `internal/workflow/workflow.go:103,155`).

**Optional: `exploration.md`.** `explore` also lands on `defined`
(`internal/store/manual.go:30`; `internal/workflow/workflow.go:200`), so a feature can be
`defined` without it, and `next` treats its absence as "run explore next"
rather than as a defect (`internal/cli/phase2.go:437-446`). Making it required
would contradict WP-005 Agreed item 6 — invoking the optional bundle command
"must not make exploration mandatory for every trivial feature"
(`docs/whitepapers/WP-005-spec-driven-workflows.md:71-74`).

However, `verify` *does* require it later, with block severity
(`internal/workflow/verify.go:413-432`). Suppressing that fact would make the
report misleading in the other direction. Resolution: when `exploration` is not
`present-nonempty`, the report emits the advisory
`exploration-absent-verify-requires-later` (§10.3). An advisory never changes
readiness or the exit code.

**Optional: `artifacts/analysis.json`.** Path A writes it; Path B does not
(§1.2). Requiring it would fail every agent-authored feature.

### 6.3 Path A / Path B parity — reported, never inferred

v1 readiness is path-agnostic: a Path B feature with `analysis.md` and
`spec.md` and no sidecar is `ready`.

The sidecar gets its own reported row and, when it is not
`present-nonempty`, the advisory `analysis-sidecar-absent-path-b-normal`,
whose message is a fixed statement about **tpatch's** behavior:

> `artifacts/analysis.json` is written by the CLI-driven analyze phase and is
> not produced by `analyze --manual`. Its absence is not a defect.

**Hard constraint.** The report must contain no field, value or message that
asserts *this feature* was produced by Path A or Path B. That would be a
provenance inference from sidecar presence, which §11.2 forbids. Acceptance
rows AVP-073 and AVP-074 assert the absence of such a claim.

## 7. Structural classification

### 7.1 Contract

```go
// package internal/intent — pure, read-only, no store, no provider.
func Inspect(repoRoot, slug string) (Report, error)
```

`Inspect` performs filesystem reads only. It must not import
`internal/store`, `internal/provider`, or `internal/gitutil`. The
`feature_state` echo (§10.2) is loaded by the CLI layer via
`store.LoadFeatureStatus` (`internal/store/store.go:351-361`) and passed in;
the inspector never sees a `*store.Store`, which is the type that exposes every
writer. Enforced by AVP-087.

Because `internal/intent` cannot use the store's unexported path helpers
(`internal/store/store.go:785-797`), it declares its own canonical path
constants. AVP-088 is a parity guard asserting those constants agree with
`store.ManualPhase("analyze"|"define"|"explore")` (`internal/store/manual.go:36-39`) so the
two path tables cannot drift.

### 7.2 The closed state enum

The vocabulary extends the presence vocabulary already shipped in
`internal/workflow/verify_landed.go:124-126` (`absent`, `present-empty`,
`present-nonempty`) rather than inventing a parallel one.

| State | Meaning | Satisfies a required artifact? |
|---|---|---|
| `present-nonempty` | Regular file; the captured bytes contain at least one non-whitespace byte, and — for `analysis_sidecar` only — additionally parse as a JSON **object**. | **yes** |
| `present-empty` | Regular file; captured bytes are zero-length or whitespace-only. | no |
| `absent` | The final path component does not exist (`os.ErrNotExist` from `Lstat`). | no |
| `symlink-refused` | Any component at or below the feature directory, including the leaf, is a symbolic link. Never followed, never read, target never named. | no |
| `not-regular` | The leaf exists and is not a symlink but is not a regular file (directory, socket, FIFO, device). | no |
| `unreadable` | `Lstat`, `open` or `read` failed for any non-absence reason (EACCES, EIO, ELOOP, ENAMETOOLONG, …). | no |
| `oversize` | The leaf's reported size exceeds `MaxArtifactBytes`; the file is deliberately **not** read. | no |
| `invalid-structured` | `analysis_sidecar` only: bytes captured, but they are not valid JSON, or are valid JSON that is not an object. | no (and it is optional, so it never affects readiness) |
| `unstable` | The artifact changed identity or size across its own capture window (§8.3). The captured bytes are not trusted and are not classified further. | no — and it forces `indeterminate` (§9.1) |

The enum is **closed and total**: §7.4 maps every reachable condition into
exactly one row. AVP-093 mechanically asserts the implementation's exported
state constants equal this table.

`invalid-structured` applies only to `analysis_sidecar`. The three Markdown
artifacts are never parsed, never linted, and never inspected for headings —
that would be semantic quality (§7.6).

### 7.3 Path and I/O policy

1. **Fixed paths only.** Every path is `repoRoot` + a constant. No flag, no
   config key and no artifact content ever contributes a path component.
2. **Lexical containment first.** `safety.EnsureSafeRepoPath`
   (`internal/safety/safety.go:12-28`) runs on each resolved path. With fixed
   constants this is defence in depth; it is required anyway so a future
   refactor that parameterises a path cannot silently escape.
3. **Feature-directory precondition walk.** `Lstat` each component of
   `<root>/.tpatch/features/<slug>`. A symlink component or a non-directory
   aborts the whole run with `feature-dir-unsafe` (§9.3). A missing component
   aborts with `feature-not-found`. This mirrors the ancestor-walk **policy**
   of `internal/rescap/pathgate.go:68-83` without adopting its
   missing-path-is-a-refusal semantics.
4. **Per-artifact walk.** For the remaining components (`artifacts/` for the
   sidecar) and the leaf, `Lstat` only. Any symlink → `symlink-refused`. The
   symlink **target is never resolved, never opened and never named in
   output**.
5. **Never `Stat`.** `os.Stat` follows symlinks; every current call site that
   inspects an intent artifact uses it (`internal/store/manual.go:57`,
   `internal/cli/phase2.go:556`, `internal/workflow/verify.go:416`). The
   inspector uses `os.Lstat` exclusively. AVP-089 is a source scan.
6. **Bounded read.** `MaxArtifactBytes = 4 MiB` (4,194,304). Chosen as ~3
   orders of magnitude above any real intent artifact while keeping a
   read-only inspector from becoming a memory amplifier on a corrupt or
   hostile tree. Exceeding it yields `oversize` with no read at all.
7. **Descriptor-scoped capture.** After opening the leaf, `fstat` the
   descriptor and compare with `os.SameFile` against the `Lstat` result. All
   bytes come from that one descriptor. Classification never re-opens the path.
8. **No writes of any kind.** No directory creation, no lock file, no temp
   file, no `.orig` backup, no `FEATURES.md` refresh (§10.5).

### 7.4 Per-artifact precedence (first match wins, total)

| # | Condition | Resulting state |
|---|---|---|
| 1 | Any non-leaf component below the feature directory is a symlink | `symlink-refused` |
| 2 | Any non-leaf component `Lstat` fails with `ErrNotExist` | `absent` |
| 3 | Any non-leaf component `Lstat` fails otherwise, or is not a directory | `unreadable` |
| 4 | Leaf `Lstat` fails with `ErrNotExist` | `absent` |
| 5 | Leaf `Lstat` fails otherwise | `unreadable` |
| 6 | Leaf mode has `os.ModeSymlink` | `symlink-refused` |
| 7 | Leaf mode is not regular | `not-regular` |
| 8 | Leaf size > `MaxArtifactBytes` | `oversize` |
| 9 | `open` fails with `ErrNotExist` (it existed at step 4) | `unstable` |
| 10 | `open` fails otherwise | `unreadable` |
| 11 | `fstat(fd)` and the step-4 `Lstat` are not `os.SameFile` | `unstable` |
| 12 | bytes read ≠ `fstat(fd)` size | `unstable` |
| 13 | read error | `unreadable` |
| 14 | `strings.TrimSpace(bytes)` is empty | `present-empty` |
| 15 | `analysis_sidecar` and bytes are not valid JSON | `invalid-structured` (`sidecar-not-json`) |
| 16 | `analysis_sidecar` and bytes are valid JSON but not an object | `invalid-structured` (`sidecar-not-json-object`) |
| 17 | otherwise | `present-nonempty` |

Three orderings are load-bearing and must not be reordered:

- **`unstable` (9, 11, 12) precedes every content-derived row (14–17).** A file
  observed mid-truncation looks empty or looks like invalid JSON. Classifying
  it `present-empty` would be exactly the false statement this PRD exists to
  prevent. Guarded by AVP-094.
- **`symlink-refused` (1, 6) precedes `not-regular` (7) and every read row.**
  A symlink is refused on kind alone; nothing downstream ever touches it.
- **Emptiness (14) precedes the structured rows (15, 16).** A whitespace-only
  `analysis.json` is not valid JSON, so classifying JSON first would report it
  as `invalid-structured` and hide the far more useful fact that the file is
  simply empty. `present-empty` is the honest answer for all four artifacts
  alike. Pinned by AVP-030.

`json.Valid` (row 15) is the same primitive the existing implement gate uses
(`internal/store/manual.go:75`); "must be an object" (row 16) is the same shape
constraint as the analyze-phase `JSONObjectValidator`
(`internal/workflow/retry.go:145-157`, wired at
`internal/workflow/workflow.go:54,62`). v1 deliberately validates **shape
only** — never field names, never the `AnalysisResult` field set — so a future
sidecar field addition cannot retroactively turn an existing feature red.

### 7.5 What the enum does *not* encode

No state means "thin", "stub", "placeholder", "low quality", "TODO-only" or
"heuristic". A `spec.md` whose entire content is `TODO: write me` is
`present-nonempty`, and the report says so without editorial comment. If a
future PRD wants a thinness signal it must define it, justify it, and enumerate
its behavior delta; it may not smuggle it in as a new enum value.

### 7.6 The non-certification statement

Every emission carries a fixed, verbatim disclaimer:

```text
Structural presence only. This report does not certify semantic quality.
```

Human output prints it as its last line. JSON carries it as the
`disclaimer` field. The string is frozen; AVP-046 asserts it byte-for-byte in
both surfaces.

## 8. Snapshot and race semantics

### 8.1 Why this section exists at all

`store.writeFile` is `os.WriteFile` (`internal/store/store.go:918-922`), which
truncates in place. `WriteFeatureFile` — the writer for all three Markdown
artifacts — goes through it (`internal/store/store.go:443-449`). So a concurrent
`tpatch define` in another process genuinely exposes a window where `spec.md`
is zero-length on disk. Reporting that window as `present-empty` would be a
false statement produced by the very command whose purpose is truthfulness.

### 8.2 One capture per artifact

The repository already has an accepted single-capture contract for report
surfaces: verify's V2 "parses the bytes the run CAPTURED, never the file",
because re-reading "produced a report built from two different versions of the
same artifact — the exact split the immutable-inventory contract exists to
prevent" (`internal/workflow/verify.go:449-455`).

`prepare --check` adopts the same discipline: each artifact is captured exactly
once, and every classification, count and message derives from that capture.
There is no second read anywhere, including in the JSON renderer.

### 8.3 Instability detection and its honest limits

Instability is detected per artifact by the three probes in §7.4 rows 9, 11 and
12: disappearance between `Lstat` and `open`, inode change between `Lstat` and
`fstat`, and a byte count that disagrees with the descriptor's size.

**Scope claim, stated precisely.** These probes detect the enumerated
conditions and nothing more. They do **not** guarantee that every torn in-place
rewrite is caught: a writer that truncates and fully rewrites the same inode to
the same length between the `fstat` and the read is not detectable by size or
identity. v1 states this limitation in the PRD and does not claim otherwise in
any output string, help text or doc. Overclaiming here would repeat the
category error the PRD is correcting.

**No retry, no spin, no lock.** The inspector never re-reads to "resolve"
instability and never takes a lock. `unstable` is a reported outcome, and the
remediation is "re-run when no other tpatch process is writing this feature".
This keeps a read-only command free of any lock-acquisition side effect and
keeps runtime bounded.

### 8.4 No cross-artifact atomicity claim

The four captures are independent and sequential. The report **must not** claim
they represent one instant. Concretely:

- there is no snapshot id, generation counter or `captured_at` field;
- `status.json` is read once, before the captures, and its echoed
  `feature_state` is documented as "read before the artifact captures, not
  simultaneously with them";
- the human footer says `readiness` — never "the feature is ready".

A cross-artifact atomic view would require a repository-wide read lock, which
is out of scope and would make a read-only command a lock acquirer. AVP-086
asserts no such field exists.

### 8.5 Determinism on a quiescent tree

On a tree no other process is writing, every probe is a no-op and two
consecutive invocations produce **byte-identical** stdout. This is the
determinism contract (AVP-050) and it holds because no output field derives
from wall-clock, mtime, size, ordering of directory reads, or map iteration.

## 9. Readiness, exit codes and error precedence

### 9.1 Readiness derivation

Over the **required** set only (`analysis`, `spec`):

| Condition (first match wins) | `structural_readiness` |
|---|---|
| any required artifact is `unstable` | `indeterminate` |
| any required artifact is not `present-nonempty` | `not_ready` |
| otherwise | `ready` |

Optional artifacts never contribute — including when an optional artifact is
`unstable` or `invalid-structured`. Those produce advisories only (§10.3).

`ready` means exactly: *both required artifacts exist as regular, readable,
non-symlink files under the bounded size, and each contains at least one
non-whitespace byte.* It means nothing else.

### 9.2 Exit codes

Per-command contract, per `SPEC.md:135-141` ("Exit codes are **per-command
contracts**, not a single global enum") and `internal/cli/exit_error.go:9-13`.
Non-1 codes are surfaced through `*ExitCodeError`
(`internal/cli/exit_error.go:12-33`).

| Code | Meaning | Report emitted? |
|---|---|---|
| `0` | `structural_readiness = ready` | yes |
| `1` | Generic CLI/usage error: unknown flag (including `--manual`, `--regenerate`), wrong argument count, malformed `--path`. Produced by cobra before `RunE`. | no |
| `2` | `structural_readiness = not_ready` | yes |
| `3` | `structural_readiness = indeterminate`: an abort precondition failed (§9.3) **or** a required artifact is `unstable` | yes (with `abort` populated for the precondition case) |
| `4` | Reserved-surface refusal: `prepare <slug>` without `--check` (§5.3) | no |

### 9.3 Error precedence (first match wins)

1. **Cobra parse/arity** → `1`. Nothing else runs.
2. **Reserved-surface guard** (`--check` absent) → `4`. Evaluated before the
   store is opened, so `tpatch prepare foo` outside a workspace is `4`, not
   `3`. Deterministic and asserted by AVP-006.
3. **Abort preconditions** → `3`, with one `abort.code`, evaluated in this
   order: `workspace-not-initialized` → `feature-dir-unsafe` →
   `feature-not-found` → `status-unreadable` → `status-malformed`.
4. **Required-artifact instability** → `3`.
5. **Required-artifact shortfall** → `2`.
6. **Otherwise** → `0`.

Exactly one `abort` object is ever emitted. A run that reaches step 4 or later
has `abort` absent.

### 9.4 Abort codes

| `abort.code` | Trigger | Source anchor for the underlying condition |
|---|---|---|
| `workspace-not-initialized` | `.tpatch/` missing → `store.Open` fails | `internal/store/store.go:134-144` |
| `feature-dir-unsafe` | a symlink component, or a non-directory, at or above `.tpatch/features/<slug>` | §7.3 step 3 |
| `feature-not-found` | `.tpatch/features/<slug>` does not exist | `internal/store/store.go:786` |
| `status-unreadable` | `status.json` exists but cannot be read | `internal/store/store.go:351-355` |
| `status-malformed` | `status.json` is not valid JSON for `FeatureStatus` | `internal/store/store.go:356-359` |

`status.json` is required only for the `feature_state` echo. It is **not**
consulted for readiness, and no artifact classification depends on it.

## 10. Output contracts

### 10.1 Stream routing

Mirrors the shipped `verify` convention exactly
(`internal/cli/verify.go:112-124`), so harness authors do not learn a second
rule:

| Flags | stdout | stderr |
|---|---|---|
| *(none)* | full human report | — |
| `--json` | JSON report only | full human report |
| `--quiet` | one readiness line | — |
| `--json --quiet` | JSON report only | — |

The refusal in §5.3 and the cobra usage errors in §5.4 go to stderr and emit no
report on either stream.

### 10.2 JSON schema (v1)

`schema_version: 1`. Field order below is the emission order; it is fixed by Go
struct field declaration order, the same mechanism `doctor` uses
(`internal/workflow/doctor.go:26-33,162-167`). **No Go map appears anywhere in
this schema** — ADR-033 D11's rule, restated in
`internal/store/canonjson.go:11-17`, is that map iteration order must never
reach a wire format.

```json
{
  "schema_version": 1,
  "command": "prepare --check",
  "slug": "fix-model-id-translation",
  "feature_state": "defined",
  "disclaimer": "Structural presence only. This report does not certify semantic quality.",
  "artifacts": [
    {
      "id": "analysis",
      "path": ".tpatch/features/fix-model-id-translation/analysis.md",
      "role": "required",
      "state": "present-nonempty",
      "reason_code": "",
      "provenance": "unknown",
      "remediation": ""
    },
    {
      "id": "spec",
      "path": ".tpatch/features/fix-model-id-translation/spec.md",
      "role": "required",
      "state": "present-empty",
      "reason_code": "artifact-empty",
      "provenance": "unknown",
      "remediation": "Author .tpatch/features/fix-model-id-translation/spec.md with non-whitespace content, then re-run tpatch prepare fix-model-id-translation --check."
    },
    {
      "id": "exploration",
      "path": ".tpatch/features/fix-model-id-translation/exploration.md",
      "role": "optional",
      "state": "absent",
      "reason_code": "artifact-absent",
      "provenance": "unknown",
      "remediation": ""
    },
    {
      "id": "analysis_sidecar",
      "path": ".tpatch/features/fix-model-id-translation/artifacts/analysis.json",
      "role": "optional",
      "state": "absent",
      "reason_code": "artifact-absent",
      "provenance": "unknown",
      "remediation": ""
    }
  ],
  "overall": {
    "structural_readiness": "not_ready",
    "required_total": 2,
    "required_satisfied": 1,
    "optional_total": 2,
    "optional_satisfied": 0
  },
  "advisories": [
    {
      "code": "exploration-absent-verify-requires-later",
      "artifact_id": "exploration",
      "message": "exploration.md is not required to reach the defined state, but tpatch verify requires spec.md and exploration.md to be present and non-empty."
    },
    {
      "code": "analysis-sidecar-absent-path-b-normal",
      "artifact_id": "analysis_sidecar",
      "message": "artifacts/analysis.json is written by the CLI-driven analyze phase and is not produced by analyze --manual. Its absence is not a defect."
    },
    {
      "code": "provenance-unknown-by-design",
      "artifact_id": "",
      "message": "Per-artifact provenance is reported as unknown for every artifact. tpatch does not yet persist durable per-artifact source metadata."
    }
  ]
}
```

Rules:

1. **`artifacts` is always length 4**, always in the order
   `analysis`, `spec`, `exploration`, `analysis_sidecar` — including on the
   abort path, where every entry carries `state: "absent"`,
   `reason_code: "not-inspected"` and `remediation: ""`. A fixed-shape array
   means a consumer never branches on array length.
2. **`advisories` is sorted** by the fixed advisory-code order of §10.3, and is
   `[]` (never `null`) when empty.
3. **`abort`** is an optional trailing object, present only on the §9.3 step-3
   path:
   `{"code": "feature-not-found", "message": "<fixed template>"}`.
4. **Every string is from a closed catalog.** The only interpolated values are
   the slug and canonical repo-relative paths. No `%v`-wrapped `os` error ever
   reaches output (§14.3).
5. **`reason_code` is `""` exactly when `state == "present-nonempty"`.**
6. **`remediation` is non-empty only for a `required` artifact that is not
   `present-nonempty`.** Optional artifacts carry advisories instead, so a
   consumer cannot mistake an optional gap for a required action.

**Absent by construction** — asserted by AVP-051: `captured_at`,
`generated_at`, `timestamp`, `mtime`, `size`, `size_bytes`, `bytes`, `sha256`,
`hash`, `content`, `excerpt`, `first_line`, `title`, `path_absolute`,
`symlink_target`, `path_kind` (`a`/`b`), `snapshot_id`.

**Versioning.** The report is stdout-only; tpatch never writes it to disk and
never reads it back, so there is no reader-side version rejection (contrast
`internal/store/reconcile_evidence.go:344-345`, which must reject unknown
versions because it re-reads persisted JSONL). Consumers must ignore unknown
fields. Removing a field, renaming a field, changing a field's type, or
changing the meaning of an existing enum value requires
`schema_version: 2`. Adding a field, adding an enum value, or adding an
advisory code does not.

### 10.3 Closed catalogs

**Reason codes** (per artifact, one of):

| `reason_code` | Paired state |
|---|---|
| `""` | `present-nonempty` |
| `artifact-empty` | `present-empty` |
| `artifact-absent` | `absent` |
| `artifact-symlink-refused` | `symlink-refused` |
| `artifact-not-regular` | `not-regular` |
| `artifact-unreadable` | `unreadable` |
| `artifact-oversize` | `oversize` |
| `sidecar-not-json` | `invalid-structured` |
| `sidecar-not-json-object` | `invalid-structured` |
| `artifact-snapshot-unstable` | `unstable` |
| `not-inspected` | abort path only |

**Advisory codes**, in emission order:

1. `exploration-absent-verify-requires-later`
2. `analysis-sidecar-absent-path-b-normal`
3. `analysis-sidecar-invalid-structured`
4. `analysis-sidecar-unstable`
5. `optional-artifact-unstable`
6. `provenance-unknown-by-design` — emitted on **every** non-abort run.

**Abort codes**: the five in §9.4.

### 10.4 Human output

Fixed shape. Column positions are computed from the fixed path set, so they do
not vary with the data:

```text
prepare --check  fix-model-id-translation
lifecycle state: defined  (echoed from status.json; not evaluated by this check)

required
  analysis.md                        present-nonempty
  spec.md                            present-empty
    → Author .tpatch/features/fix-model-id-translation/spec.md with non-whitespace
      content, then re-run tpatch prepare fix-model-id-translation --check.
optional
  exploration.md                     absent
  artifacts/analysis.json            absent

provenance: unknown (all artifacts)

advisories
  exploration-absent-verify-requires-later
  analysis-sidecar-absent-path-b-normal
  provenance-unknown-by-design

readiness: not_ready (1 of 2 required artifacts are present-nonempty)
Structural presence only. This report does not certify semantic quality.
```

`--quiet` (without `--json`) prints exactly one line:

```text
prepare --check fix-model-id-translation — not_ready
```

The word `not_ready` in the quiet line uses the same token as the JSON field,
so a script grepping one is not surprised by the other.

### 10.5 Zero-mutation contract

`prepare --check` must not, on any code path including every error path:

- call `MarkFeatureState`, `SaveFeatureStatus`, `WriteVerifyRecord`,
  `WriteFeatureFile`, `WriteArtifact`, `writeFile`, `writeFileAtomic`,
  `writeJSONAtomic` or `RefreshFeaturesIndex`
  (`internal/store/store.go:368-393,443-472,863-922`) — note that
  `SaveFeatureStatus` also rewrites `FEATURES.md` (`internal/store/store.go:363-377`), so a
  single stray status write mutates two tracked files;
- create `.tpatch/features/<slug>/` or `artifacts/`;
- write a lock, temp, scratch or `.orig` file anywhere;
- invoke `git` (no index refresh, no `git status`, no worktree operation);
- open any file for writing, including `O_APPEND` or `O_TRUNC`.

Enforced three ways: a byte-level fixture assertion (AVP-053), a
`git status --porcelain` equality assertion (AVP-054), and an AST source scan
of the implementing packages (AVP-087).

## 11. Provenance

### 11.1 v1 behavior

Every artifact row emits `"provenance": "unknown"`. The value is a constant. It
is not computed, not configurable, and not affected by any flag, any file, or
any lifecycle state. The advisory `provenance-unknown-by-design` accompanies
every non-abort run so no operator reads `unknown` as an anomaly.

### 11.2 Forbidden inference sources — explicit

The implementation must not read, and must not derive provenance from, any of:

| Forbidden source | Why |
|---|---|
| `FeatureStatus.Notes` | Overwritten on every transition (`internal/store/store.go:388-392`); the `--manual` marker at `internal/store/manual.go:79` survives only until the next phase (`internal/workflow/workflow.go:155`). WP-005 Turn 3 pins this (`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:106-111`). |
| `FeatureStatus.LastCommand` | Records the last *command*, not the source of each *artifact*, and is likewise overwritten (`internal/store/store.go:389`). |
| Filename / path | Both paths write the identical canonical names (`internal/store/manual.go:28-30` vs `internal/workflow/workflow.go:96,151,196`). |
| Sidecar presence | `artifacts/analysis.json` correlates with Path A but does not prove it: it can be hand-authored, and it survives a later `analyze --manual` that overwrites only `analysis.md`. |
| Timestamps (`mtime`, `UpdatedAt`, `RequestedAt`) | Trivially forgeable, non-deterministic, and forbidden in output by ADR-027 D6 / ADR-033 D10. |
| Content (headings, "Generated in heuristic mode" markers, prose style) | The heuristic templates (`internal/workflow/workflow.go:205-262`) are copyable text, not attestations. Reading content for provenance would also breach §14.2. |
| `artifacts/recipe-provenance.json` | Covers `apply-recipe.json` only, records commit/hash not authorship, and is Path-A-and-Git-only (`internal/workflow/implement.go:18-34,222-238`). |

AVP-059 and AVP-060 assert both the constant output and the absence of these
reads.

### 11.3 Alternatives for a future durable representation

Evaluated, **not selected** (§11.4):

- **P1 — sub-record on `FeatureStatus`.** Add
  `Provenance *ProvenanceRecord \`json:"provenance,omitempty"\`` alongside
  `Verify` and `Rejection` (`internal/store/types.go:236-251,253-265`).
  *For:* written atomically with state by the one `SaveFeatureStatus` writer
  (`internal/store/store.go:368-377`), exactly the ADR-031 D1 argument; `omitempty` gives
  byte-identical round-trip for every legacy fixture, the documented
  `DependsOn` precedent (`internal/store/types.go:207-215`).
  *Against:* every phase writer must be taught to populate it; it grows the
  single hottest file in the feature directory; concurrent phase writers
  contend on one document.
- **P2 — dedicated append-only manifest.** A JSONL sidecar in the shape of
  `reconcile-evidence` (`internal/store/reconcile_evidence.go:16,90,344-345`).
  *For:* append-only audit trail, per-artifact rows, own `schema_version`,
  no contention with `status.json`.
  *Against:* a second source of truth that can drift from the artifacts it
  describes, plus a malformed-artifact policy in the shape of ADR-024/ADR-025.
- **P3 — derive, persist nothing.** *Rejected as non-viable*, not merely
  deferred: §11.2 shows every available signal is either overwritten or
  forgeable. This is the option v1 is forced into, and it is precisely why v1
  answers `unknown`.
- **P4 — write-time attestation.** Extend the write path to record a
  content hash plus a source tag at generation time, following
  `recipe-provenance.json`. *For:* proves the artifact has not changed since
  the recorded source claimed it. *Against:* requires a hash-exposure policy
  (§14.2), an out-of-band-edit story, and touches every phase writer.

### 11.4 The ADR trigger — stated, not exercised

rev-0 **selects none** of P1/P2/P4 and needs none: `unknown` is a constant, so
no persistent representation is required for this PRD to be coherent,
implementable or testable.

The trigger, per WP-005 Agreed item 8
(`docs/whitepapers/WP-005-spec-driven-workflows.md:82-88`) and Turn 3
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:106-111`):

> The moment any PRD selects a persistent provenance representation, an ADR
> (`ADR-0NN-intent-artifact-provenance-representation`) must be written and
> accepted **before** that PRD is accepted for implementation.

That ADR would have to lock: which of P1/P2/P4; the field set; whether a
content hash is persisted and whether it is ever emitted; the closed source
vocabulary; the migration answer for artifacts that predate it (which must
remain `unknown`, never backfilled by guess); and the concurrency/atomicity
story against `status.json`.

Until then, any surface that claims to know an artifact's source is
unsupported.

## 12. Decision: existing mutating `--manual` gates do not change

This is the explicit decision WP-005 Turn 4 requires
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:150-157`).

### 12.1 The decision

**Accepted for slice 1: `analyze --manual`, `define --manual`,
`explore --manual` and `implement --manual` keep their exact current
behavior.** The shared inspector is built as a pure function (§7.1) and wired
to `prepare --check` **only**. `AdvanceStateManually`
(`internal/store/manual.go:51-81`) is not modified, not re-routed through the
inspector, and not made stricter.

Concretely and deliberately, after this PRD ships:

| Input | Current behavior | Behavior after slice 1 |
|---|---|---|
| `define --manual` with a zero-byte `spec.md` | succeeds → `defined` | **succeeds → `defined`** (unchanged) |
| `analyze --manual` with a whitespace-only `analysis.md` | succeeds → `analyzed` | **succeeds → `analyzed`** (unchanged) |
| `explore --manual` with `exploration.md` a symlink to a readable file | succeeds → `defined` (`os.Stat` follows) | **succeeds → `defined`** (unchanged) |
| `<phase> --manual` with the artifact absent | refuses, no transition | refuses, no transition (unchanged) |
| `<phase> --manual` with the artifact a directory | refuses, no transition | refuses, no transition (unchanged) |
| `implement --manual` with invalid-JSON recipe | refuses, no transition | refuses, no transition (unchanged) |

AVP-064 through AVP-069 pin these rows — including the counter-intuitive ones —
as *deliberately unchanged*, so a later refactor cannot tighten them by
accident and call it a bug fix.

### 12.2 Why report-only

1. **Existing Path B workflows would break.** The operator guide teaches
   authoring the three Markdown files by hand before advancing
   (`docs/path-b-operator-guide.md:63-73`). A stub-then-fill sequence — write
   the file, advance, fill it in — is legal today. Tightening the gate turns a
   working agent loop into a refusal with no migration path.
2. **It would retroactively change what `defined` means.** Features that
   reached `defined` through a gate that accepted empty content would become
   unreachable-by-replay. WP-005 Agreed item 5 requires existing `defined`
   features be "reported, not retroactively invalidated"
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:65-70`).
3. **WP-005 Agreed item 9 makes slice 1 advisory** by construction
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:89-92`).
4. **Separation of concerns.** The value of this PRD is a truthful *report*.
   Coupling the report to a *gate* means every classification bug becomes a
   workflow outage.

### 12.3 What a future tightening PRD must contain

Not authorized here. To tighten any `--manual` gate, a future PRD must
enumerate, at minimum:

- the exact command × input matrix that newly refuses, in the shape of §12.1;
- the exit code for each new refusal and whether it is `ExitCodeError`-typed;
- the migration answer for features already in `analyzed` / `defined` that were
  advanced through the looser gate;
- whether `os.Stat` → `os.Lstat` (which changes symlink behavior for shipped
  workflows) is in scope;
- the deprecation/announcement path across `CHANGELOG.md`, the six skill
  surfaces and `docs/agent-as-provider.md`.

"Reuse the inspector in `AdvanceStateManually`" is not a sufficient
specification of a behavior change.

## 13. Compatibility and migration

### 13.1 No lifecycle change

No new `FeatureState`. The enum stays exactly as it is
(`internal/store/types.go:9-18,30,36`), and `ValidFeatureState`'s closed switch
(`internal/store/types.go:41`) is untouched. `prepare --check` performs no transition, so
`MarkFeatureState`'s `unapplied` guard (`internal/store/store.go:385-387`) is never reached
from this path.

### 13.2 Every existing behavior is preserved

| Surface | Guarantee |
|---|---|
| `defined` semantics | Unchanged. A `defined` feature that reports `not_ready` is still `defined`; nothing re-derives, downgrades or annotates its state (AVP-070). |
| `analyze` / `define` / `explore` / `implement` (Path A) | Unchanged; not routed through the inspector. |
| the same commands with `--manual` | Unchanged — the §12.1 decision, pinned by AVP-064…AVP-069. |
| `next` | Unchanged. It keeps using `fileExistsAt` (`internal/cli/phase2.go:439,555-558`); it does not import the inspector and its `HarnessTask` output is byte-identical for every input (AVP-071). |
| `cycle` | Unchanged. No new phase, no new gate, no new refusal (AVP-072). |
| `verify` | Unchanged, including `intent_files_present`'s `os.Stat` + `Size() == 0` semantics (`internal/workflow/verify.go:413-432`). The two checks disagree by design — `verify` requires `exploration.md`, `prepare --check` does not — and each documents its own scope. |
| `status`, `doctor`, `apply`, `record`, `land`, `reconcile`, `reject`/`reopen`, `feature unapply`, `session` | Unchanged; no new check, no new finding, no new field. |
| status labels (`ComposeLabels`) | Unchanged. Readiness is never a label and is never persisted. |
| `FEATURES.md` | Never rewritten (§10.5). |
| Worktrees and the real Git index | Untouched — the command never invokes Git. |
| `status.json` on disk | Byte-identical before and after (AVP-053). |

### 13.3 Legacy and non-standard features

`prepare --check` inspects a feature in **any** lifecycle state and never
refuses on state:

| Existing population | Behavior |
|---|---|
| `requested` (no intent artifacts yet) | Reports `absent`/`absent`, `not_ready`, exit 2. This is a truthful description, not an error. |
| `analyzed` | Typically `analysis` present, `spec` absent → `not_ready`. |
| `defined` (pre- and post-`exploration.md`) | Typically `ready`; `exploration` reported with its advisory. |
| `implementing`, `applied`, `active`, `reconciling`, `reconciling-shadow`, `blocked`, `upstream_merged` | Inspected and reported normally. Post-implementation features are not the intended audience, but the command must not pretend they do not exist. |
| `rejected` | Inspected and reported normally. The slug is explicit, so the default-hiding rule that applies to the `status` listing is irrelevant here; no `--include-rejected` flag is added (AVP-066). |
| `unapplied` | Inspected and reported normally. `prepare --check` deliberately does **not** adopt `refuseIfUnappliedState` (`internal/cli/feature_unapply.go:464-473`), which guards *mutating* verbs. Refusing a read-only inspection on an `unapplied` feature would be a gratuitous new restriction (AVP-067). |
| Features created before this PRD | No migration, no backfill, no new file. `provenance: unknown` is exactly correct for them, and remains correct forever unless §11.4's ADR lands. |
| A feature directory containing extra or legacy files (e.g. `feature.yaml`) | Ignored. The inspector reads four fixed paths and never enumerates the directory. |

### 13.4 Forward compatibility

Adding a fifth artifact, a new state, or a new advisory code later is additive
under §10.2's versioning rule. Changing the required set, changing readiness
derivation, or making an advisory affect the exit code is a breaking change
that requires a `schema_version` bump **and** an enumerated behavior delta.

## 14. Security and privacy

### 14.1 Path safety

Per §7.3: fixed canonical paths, lexical containment via
`safety.EnsureSafeRepoPath`, `Lstat`-only component walk, symlinks refused and
never followed, non-regular files refused, bounded reads,
descriptor-scoped capture. The threat model is a hostile or corrupted
`.tpatch/` tree — for example `spec.md` symlinked to `/etc/shadow`, or
`exploration.md` replaced by a FIFO that would block a naive reader forever.
Both are classified and reported without a read.

### 14.2 No content, no content hashes

- **No content bytes** in any output: no excerpt, no first line, no extracted
  title, no line/word count, no size. ADR-027 D2's rule ("summaries, hashes,
  IDs and symbolic references, not raw context") is applied here in its
  strictest form: not even summaries.
- **No content hashes in v1.** A hash of a short or guessable artifact is a
  confirmation oracle: an attacker holding the report can test candidate
  contents offline. Since the readiness verdict needs no hash, v1 emits none.
  If a future provenance representation (§11.3 P4) persists one, its ADR must
  decide separately whether it may ever be *emitted*.
- **No symlink targets.** Naming the target of a refused symlink would leak a
  path outside the repository — the exact information the refusal exists to
  protect.

### 14.3 Diagnostics hygiene

Raw `error` strings from `os` frequently embed absolute paths (and therefore
usernames and home-directory layout). No output string is built by wrapping an
`os` error. Every message is a fixed template whose only interpolations are the
slug and a repo-relative canonical path. Absolute paths never appear on stdout
or stderr. AVP-078 asserts this against a fixture rooted in a directory whose
name is a distinctive sentinel.

### 14.4 No provider, no network, no subprocess

The command constructs no provider, reads no API key or token from config or
environment for use, performs no network I/O, and spawns no subprocess
(including `git`). AVP-081 is a source scan; AVP-082 asserts a successful run
with an intentionally broken provider configuration.

## 15. Failure and recovery

| Failure | Behavior | Recovery |
|---|---|---|
| Not a tpatch workspace | exit 3, `abort.code: workspace-not-initialized` | `tpatch init` |
| Unknown slug | exit 3, `abort.code: feature-not-found` | `tpatch status` to list slugs |
| `status.json` unreadable / malformed | exit 3, `abort.code: status-unreadable` / `status-malformed` | `tpatch doctor` (D1 owns metadata repair, `internal/workflow/doctor_d1.go:14-27`) |
| Feature dir is a symlink or not a directory | exit 3, `abort.code: feature-dir-unsafe` | inspect manually; the command never resolves it |
| A required artifact is unreadable | exit 2, `state: unreadable` | fix permissions, re-run |
| A required artifact is a symlink | exit 2, `state: symlink-refused` | replace with a regular file |
| A required artifact is oversize | exit 2, `state: oversize` | inspect manually; the command refuses to read >4 MiB |
| A required artifact is unstable | exit 3, `state: unstable` | re-run when no other tpatch process is writing the feature |
| Sidecar is invalid JSON | readiness unaffected; advisory `analysis-sidecar-invalid-structured` | re-run `tpatch analyze <slug>` to regenerate, or delete the sidecar |

There is nothing to roll back after any failure: the command has written
nothing. That is the whole recovery story, and it is the strongest argument for
shipping the read-only slice first.

## 16. Rollout, docs and asset parity

### 16.1 Docs the implementation wave must update

| File | Change |
|---|---|
| `SPEC.md` | Add `tpatch prepare <slug> --check [--json] [--quiet] [--path]` to the command table (near `SPEC.md:81`) and add this command's exit-code envelope under the existing per-command exit-code section (`SPEC.md:135-141`). |
| `docs/agent-as-provider.md` | After the phase → artifact → state table (`:40-45`), add a short "inspect before you advance" note: `prepare --check` reports the same artifacts read-only and never advances state. Must also state that `--manual`'s gate is unchanged (§12). |
| `docs/path-b-operator-guide.md` | In the preferred Path B flow (`:63-73`), show `tpatch prepare <slug> --check` as an optional verification step before `analyze --manual`. |
| `docs/feature-layout.md` | Note that the four intent artifacts are the set `prepare --check` classifies, and that `artifacts/analysis.json` is Path A only. |
| `CHANGELOG.md` | New command, new exit-code contract, explicit "no existing behavior changed" line. |

This PRD edits none of them; it is planning-only. `SPEC.md` in particular is
owned by the implementation lane (`AGENTS.md` → File Ownership).

### 16.2 Skill asset parity

The six shipped skill surfaces (`assets/assets_test.go:203-213`) are the
agent-facing contract, and the parity guard exists so a shipped CLI surface
cannot drift from them (`assets/assets_test.go:215-243`). Path B agents are the
primary consumer of a read-only intent check, so v1 **does** ship it in the
skills:

1. Add `"tpatch prepare"` to `requiredCommands` (`assets/assets_test.go:14-53`).
2. Add a verbatim anchor to `requiredAnchors` in the shape of the existing
   verify bullet (`assets/assets_test.go:83-88`), e.g.
   `{"prepare-check/read-only", "tpatch prepare <slug> --check is read-only"}`.
3. Update all six surfaces in the same commit — Claude, Copilot, Copilot
   Prompt, Cursor, Windsurf, Generic.
4. The added text must not contain a bare repo-relative `docs/...md` reference,
   or `TestSkillDocReferencesAreSelfContained`
   (`assets/assets_test.go:281-320`) fails. Guidance must be inlined.

Acceptance rows AVP-090, AVP-091, AVP-092.

### 16.3 Rollout

Additive, no flag gate, no config key, no opt-in. A new subcommand cannot
regress an existing invocation, and §13.2 is asserted rather than assumed. No
telemetry, no deprecation, no migration script.

## 17. Implementation slices

Each slice is independently reviewable; all are gated on this PRD being
accepted.

| Slice | Scope | Exit criteria |
|---|---|---|
| **S1** | `internal/intent`: canonical paths, closed state enum, `Inspect`, precedence (§7.4), bounded read, instability probes. Pure; no CLI. | AVP-011…AVP-030, AVP-083…AVP-086, AVP-093, AVP-094 |
| **S2** | Report model + renderers: JSON schema, human renderer, closed catalogs, determinism, privacy. Still no CLI wiring. | AVP-039…AVP-052, AVP-059, AVP-077…AVP-080 |
| **S3** | `internal/cli`: `prepareCmd`, flag set, reserved-surface refusal, abort precedence, exit codes, stream routing. | AVP-001…AVP-010, AVP-031…AVP-038 |
| **S4** | Zero-mutation, provenance, parity and compatibility proofs; source scans. | AVP-053…AVP-058, AVP-060…AVP-063, AVP-064…AVP-076, AVP-081, AVP-082, AVP-087…AVP-089 |
| **S5** | Docs + six skill surfaces + parity guard extension. | AVP-090…AVP-092, AVP-095 |

Slices S1→S3 are strictly ordered. S4 and S5 may run in parallel with each
other **only if** they touch disjoint files; both touch neither `cobra.go` nor
`internal/intent`, so the AGENTS.md parallel-implementer rule (same-file
overlap ⇒ sequential) is satisfiable — but the cluster lead must declare the
file partition at dispatch and stage by explicit path.

## 18. Acceptance matrix

### 18.1 How to read this matrix

IDs are stable (`AVP-NNN`) and are never renumbered; a retired row is struck
through and keeps its number. Each row names the **observable** the test must
assert.

**A test does not satisfy a row merely by existing.** A row is satisfied only
when its test asserts the named observable. Specifically:

- asserting "the command did not return an error" satisfies no row;
- asserting "some JSON was produced" satisfies no row that names a field,
  value, order or exit code;
- a row naming an exit code must assert the numeric process exit code, not the
  presence of a Go error;
- a row naming "byte-identical" must compare bytes, not lengths or mtimes;
- a row naming a state must assert that exact enum literal, not a truthy
  proxy.

If a row cannot be placed as written, the PRD is amended — the row is not
silently re-tiered. (This is `PRD-verify-freshness §7.1`'s rule, and the ledger
mechanics of `internal/workflow/acceptance_ledger_test.go:1-30` apply if these
rows are later added to a ledger.)

Legend for **Kind**: `U` unit, `I` integration (real CLI invocation over a real
temp workspace), `S` source scan (AST), `G` mechanical guard.

### 18.2 A — CLI grammar and surface boundary

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-001 | I | `prepare <slug> --check` on a ready feature | exit 0; human report on stdout |
| AVP-002 | I | `prepare` with no slug | exit 1; cobra arity message; no report on stdout |
| AVP-003 | I | `prepare a b --check` | exit 1; no report on stdout |
| AVP-004 | I | `prepare <slug> --check --manual` | exit 1; stderr contains `unknown flag`; no report; `status.json` byte-identical |
| AVP-005 | I | `prepare <slug> --check --regenerate` | exit 1; stderr contains `unknown flag`; no report |
| AVP-006 | I | `prepare <slug>` (no `--check`) **outside** any tpatch workspace | exit 4 (not 3); refusal text names `--check`; store never opened |
| AVP-007 | I | `prepare <slug>` (no `--check`) inside a workspace with a ready feature | exit 4; no report on stdout; `.tpatch/` byte-identical |
| AVP-008 | I | `prepare <slug> --check --path <dir>` from an unrelated cwd | exit 0; inspects the feature under `<dir>` |
| AVP-009 | I | `prepare --help` | output states it is unrelated to `apply --mode prepare` |
| AVP-010 | I | `apply --help` | `--mode` description points at `prepare --check` |

### 18.3 B — Structural classification

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-011 | U | `analysis.md` with ordinary content | `present-nonempty`, `reason_code: ""` |
| AVP-012 | U | `spec.md` zero bytes | `present-empty`, `artifact-empty` |
| AVP-013 | U | `spec.md` containing only `" \t\n\r\n"` | `present-empty` (whitespace-only is empty) |
| AVP-014 | U | `spec.md` containing a single `x` | `present-nonempty` |
| AVP-015 | U | `analysis.md` absent | `absent`, `artifact-absent` |
| AVP-016 | U | `spec.md` is a symlink to a readable in-repo file | `symlink-refused`; target never opened (spy on the open helper records zero opens for that path) |
| AVP-017 | U | `spec.md` is a symlink to a path outside the repo | `symlink-refused`; the target path string appears nowhere in output |
| AVP-018 | U | `spec.md` is a dangling symlink | `symlink-refused` (not `absent`) |
| AVP-019 | U | `artifacts/` is a symlink, sidecar underneath | sidecar is `symlink-refused` (non-leaf component rule) |
| AVP-020 | U | `spec.md` is a directory | `not-regular` |
| AVP-021 | U | `exploration.md` is a FIFO | `not-regular`; the test completes (no blocking read) |
| AVP-022 | U | `spec.md` mode `0000` (unreadable), non-root | `unreadable`, `artifact-unreadable` |
| AVP-023 | U | `spec.md` size = `MaxArtifactBytes` exactly | classified normally (boundary is inclusive-OK) |
| AVP-024 | U | `spec.md` size = `MaxArtifactBytes + 1` | `oversize`; the read helper records zero bytes read for that path |
| AVP-025 | U | sidecar containing `{"summary":"x"}` | `present-nonempty` |
| AVP-026 | U | sidecar containing `{` | `invalid-structured`, `sidecar-not-json` |
| AVP-027 | U | sidecar containing `[1,2,3]` | `invalid-structured`, `sidecar-not-json-object` |
| AVP-028 | U | sidecar containing `"a string"` | `invalid-structured`, `sidecar-not-json-object` |
| AVP-029 | U | sidecar containing `{"unknown_future_field":1}` | `present-nonempty` — unknown fields never fail |
| AVP-030 | U | sidecar whitespace-only | `present-empty` (emptiness precedes JSON parsing) |

### 18.4 C — Readiness and exit codes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-031 | I | both required `present-nonempty`, both optional absent | `ready`; exit 0 |
| AVP-032 | I | `analysis` present, `spec` absent | `not_ready`; exit 2; `required_satisfied: 1` |
| AVP-033 | I | both required absent (fresh `requested` feature) | `not_ready`; exit 2; `required_satisfied: 0` |
| AVP-034 | I | required both present; `exploration` absent; sidecar absent | `ready`; exit 0 — optional gaps never block |
| AVP-035 | I | required both present; sidecar `invalid-structured` | `ready`; exit 0; advisory `analysis-sidecar-invalid-structured` present |
| AVP-036 | I | required both present; `exploration` `unstable` | `ready`; exit 0; advisory `optional-artifact-unstable` |
| AVP-037 | I | `spec` `unstable` | `indeterminate`; exit 3; no `abort` object |
| AVP-038 | I | unknown slug | `indeterminate`; exit 3; `abort.code: feature-not-found` |

### 18.5 D — Output shape, order and determinism

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-039 | I | `--json` on a ready feature | stdout parses as JSON; `schema_version: 1`; `command: "prepare --check"` |
| AVP-040 | I | `--json` | top-level key order is exactly `schema_version, command, slug, feature_state, disclaimer, artifacts, overall, advisories[, abort]` |
| AVP-041 | I | `--json`, any input | `artifacts` has length 4 in order `analysis, spec, exploration, analysis_sidecar` |
| AVP-042 | I | `--json` on the abort path | `artifacts` still length 4, every `state: "absent"`, `reason_code: "not-inspected"` |
| AVP-043 | I | `--json` with no advisories triggerable except the constant one | `advisories` is a JSON array (never `null`) containing `provenance-unknown-by-design` |
| AVP-044 | I | `--json` with several advisories | order matches §10.3's fixed list |
| AVP-045 | I | artifact-object key order | exactly `id, path, role, state, reason_code, provenance, remediation` |
| AVP-046 | I | both surfaces | the disclaimer string appears byte-for-byte in JSON `disclaimer` and as the human report's last line |
| AVP-047 | I | `--json` alone | JSON on stdout; human report on stderr |
| AVP-048 | I | `--json --quiet` | JSON on stdout; stderr empty |
| AVP-049 | I | `--quiet` alone | stdout is exactly one line ending in the readiness token |
| AVP-050 | I | two consecutive runs on a quiescent tree | stdout byte-identical for both human and `--json` |
| AVP-051 | G | `--json` over a matrix covering every state | none of the forbidden field names of §10.2 appears anywhere in the output |
| AVP-052 | I | `remediation` population | non-empty only for `required` artifacts not `present-nonempty`; empty for every optional artifact |

### 18.6 E — Zero mutation

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-053 | I | ready feature, run `--check` | every file under `.tpatch/` byte-identical before/after; the file set is identical (no additions, no deletions) |
| AVP-054 | I | inside a Git repo with `.tpatch/` tracked | `git status --porcelain` output identical before/after |
| AVP-055 | I | every abort path (all five codes) | `.tpatch/` byte-identical; no directory created |
| AVP-056 | I | run against a slug whose feature directory does not exist | `.tpatch/features/<slug>/` is **not** created |
| AVP-057 | I | `.tpatch/` mounted read-only (or all files mode `0444`) | the command still succeeds and reports |
| AVP-058 | I | `FEATURES.md` present | byte-identical after the run |

### 18.7 F — Provenance

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-059 | I | every artifact in every state | `provenance` is the literal `"unknown"` in all four rows, every time |
| AVP-060 | S | inspector + renderer packages | no reference to `Notes`, `LastCommand`, `UpdatedAt`, `RequestedAt`, `ModTime`, or `recipe-provenance.json` |
| AVP-061 | I | feature advanced via `<phase> --manual` (notes carry the manual marker) | `provenance` is still `"unknown"` for every artifact |
| AVP-062 | I | feature advanced via Path A (`analyze` then `define`) | `provenance` is still `"unknown"` for every artifact |
| AVP-063 | I | feature with `artifacts/analysis.json` present and valid | `provenance` is still `"unknown"`; no field or message claims Path A |

### 18.8 G — Compatibility (including the §12 decision)

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-064 | I | `define --manual` with a **zero-byte** `spec.md` | still exits 0 and state becomes `defined` — deliberately unchanged (§12.1) |
| AVP-065 | I | `analyze --manual` with a **whitespace-only** `analysis.md` | still exits 0 and state becomes `analyzed` — deliberately unchanged |
| AVP-066 | I | `prepare --check` on a `rejected` feature | reports normally; exit reflects readiness; no `--include-rejected` flag exists |
| AVP-067 | I | `prepare --check` on an `unapplied` feature | reports normally; **not** refused by the unapplied guard |
| AVP-068 | I | `explore --manual` with `exploration.md` a symlink to a readable file | still exits 0 and state becomes `defined` — deliberately unchanged (`--manual` still uses `os.Stat`) |
| AVP-069 | I | `implement --manual` with invalid-JSON recipe | still refuses, state unchanged — unchanged |
| AVP-070 | I | `defined` feature reporting `not_ready`, then `tpatch status --json` | the feature's state field is still `defined`; no readiness field appears in `status` output |
| AVP-071 | I | `next <slug>` output before and after a `prepare --check` run, across `requested`/`analyzed`/`defined`-pre-explore/`defined`-post-explore | byte-identical in both `text` and `harness-json` formats |
| AVP-072 | I | `cycle <slug> --skip-execute` with a heuristic (no-provider) workspace | same phases run, same final state, same stdout as without this feature — no new gate |

### 18.9 H — Path A / Path B parity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-073 | I | Path B feature (`--manual` throughout), no sidecar | `ready`; exit 0; advisory `analysis-sidecar-absent-path-b-normal` present |
| AVP-074 | G | full output matrix | no output field or message asserts that *this feature* is Path A or Path B; the tokens `"path_kind"`, `"path": "A"`, `"path": "B"` never appear |
| AVP-075 | I | Path A feature (heuristic provider) after `analyze` + `define` | `ready`; sidecar `present-nonempty`; `optional_satisfied` counts it |
| AVP-076 | I | sidecar present but `analysis.md` deleted | `not_ready`; exit 2 — the sidecar never substitutes for a required artifact |

### 18.10 I — Security and privacy

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-077 | I | artifacts containing a distinctive sentinel string | the sentinel appears in neither stdout nor stderr, in any flag combination |
| AVP-078 | I | workspace rooted at a directory containing a distinctive sentinel path segment | no absolute path (and no sentinel segment) appears in stdout or stderr on any path, including all aborts |
| AVP-079 | G | full output matrix | no 64-hex-character token appears anywhere (no content hashes) |
| AVP-080 | I | symlink refusal case | the symlink's target path appears nowhere in output |
| AVP-081 | S | inspector + CLI command packages | no import of `internal/provider`, `net/http`, `os/exec`; no `exec.Command` call |
| AVP-082 | I | workspace whose provider config points at an unreachable endpoint with a bogus key | `prepare --check` exits on readiness alone; no network attempt; runtime bounded |

### 18.11 J — Concurrency and snapshot

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-083 | U | deterministic hook deletes `spec.md` between `Lstat` and `open` | `unstable`, `artifact-snapshot-unstable` (not `absent`) |
| AVP-084 | U | deterministic hook replaces `spec.md` with a different inode between `Lstat` and `fstat` | `unstable` via the `os.SameFile` probe |
| AVP-085 | U | hook truncates `spec.md` to zero after `fstat`, before the read | `unstable` via the byte-count/size disagreement — **never** `present-empty` |
| AVP-086 | I | quiescent tree | no artifact is `unstable`; no snapshot/atomicity field exists in the JSON (`snapshot_id`, `captured_at` absent) |

### 18.12 K — Source scans and parity guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-087 | S | inspector package | imports neither `internal/store` nor `internal/gitutil`; the CLI command's call graph reaches none of the writer symbols listed in §10.5 |
| AVP-088 | U | canonical path constants | `analysis.md`, `spec.md`, `exploration.md` equal `store.ManualPhase("analyze"\|"define"\|"explore").Path`; the sidecar path equals `filepath.Join("artifacts", "analysis.json")` |
| AVP-089 | S | inspector package | zero calls to `os.Stat`; all stat calls are `os.Lstat` |
| AVP-090 | U | `assets/assets_test.go` | `requiredCommands` contains `tpatch prepare` |
| AVP-091 | U | all six skill surfaces | each contains the verbatim `prepare --check` read-only anchor |
| AVP-092 | U | all six skill surfaces | `TestSkillDocReferencesAreSelfContained` still passes with the new text |

### 18.13 L — Totality and completeness guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-093 | G | state enum totality | the implementation's exported state constants, sorted, equal the §7.2 table parsed from this PRD; a state added in code without a PRD row fails, and vice versa |
| AVP-094 | G | precedence ordering | the ordering test asserts `unstable` outranks `present-empty` and `invalid-structured`, and `symlink-refused` outranks `not-regular`; reordering the implementation's branches fails it |
| AVP-095 | G | catalog totality | every reason code (§10.3), every advisory code (§10.3) and every abort code (§9.4) is produced by at least one AVP row's fixture, and no code exists in the implementation that no row produces |

**Count: 95 acceptance rows** (A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6,
J 4, K 6, L 3).

### 18.14 Sensitivity requirement

AVP-093, AVP-094 and AVP-095 are mechanical guards, and mechanical guards can
false-pass. Each must ship with a **sensitivity regression** proving it fails on
a deliberately broken input — a synthetic extra enum value, a swapped
precedence pair, an orphan catalog code. This is the lesson recorded against
`TestAcceptanceLedger_TestsExist`, which could false-pass on a comment because
it searched raw bytes (`docs/handoff/CURRENT.md`, Post-Release Review
Adjudication, F2). A guard without a proven failure mode is not evidence.

## 19. Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Operators read `ready` as "this feature is well specified". | High | The frozen disclaimer in both surfaces (§7.6, AVP-046); state names are purely structural (§7.5); `ready` is defined operationally in §9.1. |
| R2 | `prepare` collides with `apply --mode prepare` and confuses agents. | Medium | Reciprocal help text (§5.2), asserted by AVP-009/AVP-010; skill surfaces name the full invocation including `--check` (§16.2). |
| R3 | A future wave quietly reuses the inspector inside `AdvanceStateManually`, silently tightening a shipped gate. | Medium | §12's decision is written as acceptance rows AVP-064…AVP-069 that pin the *loose* behavior; a tightening refactor turns them red. |
| R4 | Instability detection over-promises and operators trust a torn read. | Medium | §8.3 states the limit in the PRD and forbids any stronger claim in output or docs; no retry/lock is added. |
| R5 | `oversize` makes a legitimate large artifact unusable. | Low | 4 MiB is ~1000× any real intent artifact; the state is reported honestly with a manual-inspection remediation rather than a silent truncation. |
| R6 | Schema churn breaks harness consumers. | Low | §10.2's versioning rule; fixed-length `artifacts` array; consumers must ignore unknown fields. |
| R7 | The four-artifact set is wrong for some workflow. | Low | The set is closed for v1 and additive later; §6.1 states the exclusion rationale rather than leaving it implicit. |
| R8 | Skill-surface edits drift across the six formats. | Low | The existing parity guard is the mechanism; AVP-090…AVP-092 extend it in the same commit. |

## 20. Relationship to `PRD-prepare-intent-bundle` (blocked)

**`PRD-prepare-intent-bundle.md` remains blocked until this PRD is accepted.**
It is not drafted, not scheduled, and not implied by anything here.

When it is unblocked it must, at minimum, address what this PRD deliberately
does not:

1. **Atomic publication.** WP-005 Turn 3 fixes the unit as the three canonical
   Markdown files **plus** structured sidecars **plus** the final `status.json`
   transition (`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`). The current
   Path A phase functions write artifacts and state incrementally
   (`internal/workflow/workflow.go:89-105,151-155,196-200`), so `prepare`
   cannot call them in sequence and claim atomicity.
2. **Non-destructive overwrite** and a `--regenerate` policy that cannot lose
   hand-authored content.
3. **Provenance**, which requires §11.4's ADR first if the bundle intends to
   claim any artifact's source.
4. **Partial-failure exposure**: a provider failure must leave either the
   complete prior set or the complete new set
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`).
5. **Whether readiness classification becomes a gate** — a routing behavior
   delta that §12.3's rules govern.

This PRD gives that work a truthful input. It does not authorize it.

## 21. Open questions

Only genuinely unresolved items are listed. Everything else in this document is
a decision.

| # | Question | Why it is open | Default if unanswered |
|---|---|---|---|
| Q1 | Should `--all` (inspect every feature) exist in a later slice? | It is useful for harnesses but multiplies output-shape and ordering questions, and `verify --all` shows that sweep semantics deserve their own design pass. | Not in v1; a later PRD may add it additively. |
| Q2 | Should exit code `4` be reused by other future reserved-surface refusals, or stay local to `prepare`? | `SPEC.md:135-141` establishes exit codes as per-command contracts, so a cross-command meaning would be a new convention. | Local to `prepare`; a cross-command convention needs its own decision. |
| Q3 | Is `MaxArtifactBytes = 4 MiB` right? | No empirical distribution of intent-artifact sizes has been gathered. | 4 MiB; changing it later is additive and affects only the `oversize` boundary. |
| Q4 | Should `request.md` be reported as a fifth (optional) row? | It is an input rather than a phase output (`docs/feature-layout.md:88-97`), but its absence does make every downstream artifact suspect. | Excluded from v1; additive later. |

## 22. Alternatives considered

| Alternative | Why not |
|---|---|
| **Extend `verify` with a pre-apply mode** instead of a new command | `verify` refuses pre-apply states by design (`internal/workflow/verify.go:245-252`), persists a record by default (`internal/workflow/verify.go:347-352`), and has a distinct 11-check contract and audience. Bending it to serve the pre-`defined` window would change a shipped surface's meaning for existing users. |
| **Add a `doctor` check (D9)** | `doctor` is workspace-wide, `--fix`-capable, and finding-shaped. A per-feature readiness verdict with its own exit-code contract does not fit that report model, and making `doctor` artifact-aware would expand a writer's surface. |
| **Extend `next` to report artifact structure** | `next` answers "what runs next" and is consumed by harnesses as a task envelope. Adding classification would change a shipped output shape and couple routing to classification — exactly what WP-005 Agreed item 9 forbids for slice 1. |
| **Make the check mutating from day one** (adopt artifacts, advance state) | The core WP-005 finding: validation precedes orchestration (`docs/whitepapers/WP-005-spec-driven-workflows.md:59-64`). A mutating command built on today's validation would amplify §1.1–§1.3. |
| **Introduce a `prepared` lifecycle state** | Explicitly rejected by WP-005 Agreed item 6 (`docs/whitepapers/WP-005-spec-driven-workflows.md:71-74`). Completeness is an artifact-level fact; adding a state would force every consumer of `FeatureState` to learn it. |
| **Infer provenance heuristically and label it "best effort"** | A best-effort provenance field is worse than none: consumers use it, and §11.2 shows every signal is overwritten or forgeable. `unknown` is the only truthful v1 answer. |
| **Emit content hashes for stability comparison across runs** | Rejected on privacy grounds (§14.2) and because readiness needs no hash. |
| **Take a workspace lock for a coherent multi-artifact snapshot** | Makes a read-only command a lock acquirer, can block on a stuck writer, and still cannot prove atomicity for files written by a truncating writer. §8.4 chooses honest non-atomicity instead. |

## 23. Claims-audit appendix

Every load-bearing claim about current behavior, anchored. All citations
verified against HEAD `12980f2`, which is `WAVE_BASE` `0aa0d95` plus
tracking-doc-only commits; no source file cited below differs between the two.

| # | Claim | Anchor |
|---|---|---|
| C1 | `manualPhaseMap` is the single source of truth for `--manual` and maps analyze/define/explore/implement to `analysis.md`/`spec.md`/`exploration.md`/`artifacts/apply-recipe.json`. | `internal/store/manual.go:25-32` |
| C2 | `ValidateJSON` is set on the `implement` row only, so only that phase's artifact is content-validated. | `internal/store/manual.go:31,67-78` |
| C3 | `AdvanceStateManually` checks existence and not-a-directory, and otherwise advances state; whitespace-only Markdown passes. | `internal/store/manual.go:51-81` |
| C4 | That check uses `os.Stat`, which follows symlinks. | `internal/store/manual.go:57` |
| C5 | `--manual` writes a fixed notes string recording the manual transition. | `internal/store/manual.go:79` |
| C6 | `ManualPhase` exposes the per-phase contract for reuse. | `internal/store/manual.go:36-39` |
| C7 | `MarkFeatureState` overwrites the single `Notes` field on every transition. | `internal/store/store.go:380-393` (assignment at `388-392`) |
| C8 | `FeatureStatus.Notes` is one optional free-text string. | `internal/store/types.go:215` |
| C9 | `SaveFeatureStatus` also refreshes `FEATURES.md`, so one status write mutates two tracked files. | `internal/store/store.go:363-377` |
| C10 | `LoadFeatureStatus` is a pure reader over `status.json`. | `internal/store/store.go:351-361` |
| C11 | `store.Open` refuses when `.tpatch/` is absent. | `internal/store/store.go:134-144` |
| C12 | Feature paths are `<root>/.tpatch/features/<slug>/...`, via unexported helpers. | `internal/store/store.go:779,785-797` |
| C13 | `WriteFeatureFile` and `WriteArtifact` are the artifact writers; both funnel to `writeFile`. | `internal/store/store.go:443-449,461-472` |
| C14 | `writeFile` is `os.WriteFile`, which truncates in place — the concurrency hazard §8 addresses. | `internal/store/store.go:918-922` |
| C15 | Path A `RunAnalysis` writes `artifacts/analysis.json` **and** `analysis.md`, then marks `analyzed`. | `internal/workflow/workflow.go:89-105` |
| C16 | `RunDefine` reads the sidecar opportunistically and ignores its absence. | `internal/workflow/workflow.go:117-121` |
| C17 | `RunDefine` writes `spec.md` and marks `defined` with a fixed notes string that overwrites any prior note. | `internal/workflow/workflow.go:151-155` |
| C18 | `RunExplore` writes `exploration.md` and also marks `defined`, so exploration is not required to reach `defined`. | `internal/workflow/workflow.go:196-200` |
| C19 | Heuristic fallbacks exist and produce templated content, so content markers cannot attest authorship. | `internal/workflow/workflow.go:205-262` |
| C20 | `JSONObjectValidator` is the existing "must be a JSON object" primitive, used by the analyze phase. | `internal/workflow/retry.go:145-157`; wired at `internal/workflow/workflow.go:54,62` |
| C21 | The `FeatureState` enum and its closed validity switch. | `internal/store/types.go:6-18,30,36,41` |
| C22 | `Verify` and `Rejection` are `omitempty` sub-records on `FeatureStatus` — the P1 precedent. | `internal/store/types.go:236-251,253-265` |
| C23 | `DependsOn`'s doc comment states the `omitempty` byte-identity migration contract. | `internal/store/types.go:207-215` |
| C24 | `cycle` runs analyze → define → explore → implement → apply → record with provider calls. | `internal/cli/phase2.go:25-145` |
| C25 | `cycle --skip-execute` stops after recipe generation. | `internal/cli/phase2.go:122-127` |
| C26 | `next` distinguishes the two `defined` sub-states from `exploration.md` presence. | `internal/cli/phase2.go:437-446` |
| C27 | `fileExistsAt` is an `os.Stat` wrapper — no emptiness, kind or readability discrimination. | `internal/cli/phase2.go:555-558` |
| C28 | `next` emits a `HarnessTask` with `--format text\|harness-json`. | `internal/cli/phase2.go:360-407` |
| C29 | `analyze` dispatches to `runManualPhase` when `--manual`/`--skip-llm` is set; `define` and `explore` follow the identical shape. | `internal/cli/cobra.go:603-605,651-653,693-695` |
| C30 | `addManualFlag` / `isManualFlag` / `runManualPhase` are the shared `--manual` helpers. | `internal/cli/cobra.go:3407-3436` |
| C31 | `apply --mode prepare` already exists as a shipped mode — the §5.2 collision. | `internal/cli/cobra.go:822-824,836,840`; `SPEC.md:81` |
| C32 | `openStoreFromCmd` resolves `--path` then the project root. | `internal/cli/cobra.go:3782-3793` |
| C33 | `ExitCodeError` is the mechanism for non-1 exit codes, and exit codes are per-command contracts. | `internal/cli/exit_error.go:9-33`; `SPEC.md:135-141` |
| C34 | `amend --state` is the precedent for refusing a deliberately reserved surface with a typed exit code. | `internal/cli/c1.go:276-289` |
| C35 | `refuseIfUnappliedState` guards mutating verbs only. | `internal/cli/feature_unapply.go:464-473` |
| C36 | `verify`'s `intent_files_present` checks only `spec.md` and `exploration.md`, via `os.Stat` and `Size() == 0`. | `internal/workflow/verify.go:413-439` |
| C37 | `verify` refuses every pre-apply / mid-flight lifecycle state before V1 runs. | `internal/workflow/verify.go:245-252` |
| C38 | `verify` persists a record unless `--no-write`, so it is a writer by default. | `internal/workflow/verify.go:347-352`; `internal/cli/verify.go:151` |
| C39 | `verify` V2 parses the bytes the run captured, never the file — the single-capture precedent. | `internal/workflow/verify.go:449-455` |
| C40 | The presence vocabulary `absent` / `present-empty` / `present-nonempty` already exists. | `internal/workflow/verify_landed.go:124-126` |
| C41 | `snapshotArtifact` already distinguishes a non-absence read failure from absence. | `internal/workflow/verify_landed.go:221-251` |
| C42 | Snapshot instability is an existing, shipped concept with a `snapshot-unstable` outcome. | `internal/workflow/verify_landed.go:83,466-500,1644-1662` |
| C43 | `verify`'s `--json` / `--quiet` stream routing is the precedent §10.1 copies. | `internal/cli/verify.go:112-124,147-151` |
| C44 | `recipe-provenance.json` records `base_commit` / `generated_at` / `recipe_sha256` — not authorship — and is written only on the Path A implement path inside a Git repo. | `internal/workflow/implement.go:18-34,222-238` |
| C45 | `doctor` has a `schema_version`-tagged JSON report with struct-order field emission and a per-command exit-code function. | `internal/workflow/doctor.go:17,26-33,162-167,199-207`; `internal/cli/doctor.go:44-53` |
| C46 | `doctor` D1 validates `status.json` and legacy `feature.yaml`; no doctor check reads the intent Markdown artifacts. | `internal/workflow/doctor_d1.go:14-52` |
| C47 | Persisted JSONL schemas reject unknown `schema_version` on read — the contrast §10.2 draws for a stdout-only report. | `internal/store/reconcile_evidence.go:16,344-345` |
| C48 | "No Go map type appears in any tracked wire schema" is an existing, documented rule. | `internal/store/canonjson.go:11-17` |
| C49 | `safety.EnsureSafeRepoPath` is lexical-only containment. | `internal/safety/safety.go:12-28`; described as the coarse pre-filter at `internal/rescap/pathgate.go:50-54` |
| C50 | `rescap.GatePath`'s five-step gate (component `Lstat`, symlink refusal, `O_NOFOLLOW`, descriptor identity) is the policy §7.3 reuses; it refuses missing paths. | `internal/rescap/pathgate.go:68-83,97-120,133-148` |
| C51 | `ListFeatures` enumerates feature directories and skips features without a valid `status.json`. | `internal/store/store.go:209-227` |
| C52 | The six shipped skill surfaces and the `requiredCommands` / `requiredAnchors` parity guard. | `assets/assets_test.go:13-53,56-88,203-213,215-243` |
| C53 | Skill surfaces must not carry bare repo-relative `docs/...md` references. | `assets/assets_test.go:281-320` |
| C54 | The phase → artifact → state contract and the `--manual` notes string are documented for agents. | `docs/agent-as-provider.md:33-54` |
| C55 | The Path B operator flow instructs authoring the three Markdown artifacts by hand. | `docs/path-b-operator-guide.md:63-73` |
| C56 | `docs/feature-layout.md` is the authoritative map of the feature directory and its lifecycle files. | `docs/feature-layout.md:10-33,88-97` |
| C57 | WP-005 Agreed: validation precedes orchestration; `prepare --check` is the first slice; presence is not semantic quality; provenance is `unknown`; no new lifecycle state; mutating preparation is gated; two ordered PRDs; slice 1 is advisory. | `docs/whitepapers/WP-005-spec-driven-workflows.md:46-98` |
| C58 | WP-005 §6.2 requires the existing-primitives pre-flight over `--manual`, `cycle` and `next`. | `docs/whitepapers/WP-005-spec-driven-workflows.md:481-518` |
| C59 | WP-005 Turn 2 records the council split and the two ordered PRDs. | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:26-82` |
| C60 | WP-005 Turn 3 pins `unknown` provenance, the ADR trigger, the atomic-publication unit, and slice-1 routing compatibility. | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:84-141` |
| C61 | WP-005 Turn 4 requires the first PRD to decide report-only vs stronger `--manual` gates in acceptance criteria. | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:143-165` |
| C62 | GH #10 scopes this PRD and states that `PRD-prepare-intent-bundle.md` remains blocked. | [GH #10](https://github.com/tesseracode/tesserapatch/issues/10) |
| C63 | The acceptance-ledger machinery (AC ids → real test functions, AST-resolved) is the precedent §18.1 follows. | `internal/workflow/acceptance_ledger_test.go:1-30` |
| C64 | A byte-scanning guard can false-pass, which is why §18.14 requires sensitivity regressions. | `docs/handoff/CURRENT.md` → "Post-Release Review Adjudication" → F2 |

**64 claims audited.**
