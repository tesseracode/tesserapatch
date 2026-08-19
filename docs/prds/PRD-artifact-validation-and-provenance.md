# PRD — Artifact Validation and Provenance — `tpatch prepare <slug> --check`

**Status**: Accepted — 2026-08-13 (rev-5); **errata rev-7 — 2026-08-19** (no decision change)
**Date**: 2026-08-13
**Owner**: Core (planning lane)
**Byline**: supervisor note-fold, rev-5 based on dispatch HEAD `70876c1`
**Milestone**: TBD — this document ships no code
**Issue**: [GH #10 — define truthful intent-artifact validation and provenance](https://github.com/tesseracode/tesserapatch/issues/10)
**Graduates from**: [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md), Turns 2–4
**Architecture**: [ADR-034 — Rooted filesystem inspection boundary](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md)
(Accepted — 2026-08-13, rev-2). **This PRD and ADR-034 were reviewed
together.** The ADR locks the decisions; this PRD states the design that
depends on them. Where they overlap, ADR-034 is normative.
**Unblocks**: `PRD-prepare-intent-bundle.md` for planning only. Mutating
implementation remains unauthorized until that separate PRD is accepted. See
§20.

## Related

- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md) — `## Agreed — Turns 2–3` and §6.2
- [WP-005 turn log](../whitepapers/WP-005-spec-driven-workflows.turns.md) — Turns 2, 3 and 4
- [Agent as Provider — Path B workflow](../agent-as-provider.md) — the phase → artifact → state contract
- [Path B Operator Guide](../path-b-operator-guide.md) — the hand-authored artifact set
- [Feature Layout](../feature-layout.md) — canonical vs audit-trail files under `.tpatch/features/<slug>/`
- [PRD-verify-freshness](./PRD-verify-freshness.md) — the freshness overlay whose `intent_files_present` check this PRD deliberately does **not** replace
- [PRD-tpatch-doctor](./PRD-tpatch-doctor.md) — workspace metadata drift, the adjacent read-only diagnostic
- [PRD-rejected-feature-state](./PRD-rejected-feature-state.md) — precedent for closed enums, evidence handling and per-command exit envelopes
- [PRD-feature-resource-claims-and-capture-adapters](./PRD-feature-resource-claims-and-capture-adapters.md) — precedent for the path gate and the cap-plus-one bounded read reused in §7.3
- [ADR-013 verify freshness overlay](../adrs/ADR-013-verify-freshness-overlay.md) — precedent: a read surface that is not a lifecycle transition
- [ADR-027 capture context privacy boundary](../adrs/ADR-027-capture-context-privacy-boundary.md) — D2 (no raw context), D6 (no wall-clock in determinism)
- [ADR-031 rejected feature state data model](../adrs/ADR-031-rejected-feature-state-data-model.md) — D1 sub-record-on-`FeatureStatus` precedent, weighed in §11
- [ADR-033 resource capture boundary](../adrs/ADR-033-resource-capture-boundary.md) — D10 (no tracked timestamps), D11 (no Go map in a wire schema)
- [ADR-034 rooted filesystem inspection boundary](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md) — **companion, Accepted rev-2.** D1–D18 lock the `os.Root` decision, the logical-confinement scope, workspace discovery's exclusion, the `fs.ValidPath` name rule, the fail-closed `unix || windows` platform allowlist, the open flags, the Windows mapping and `winsymlink` policy, the observed-symlink/identity limits, the single reused scratch buffer, the contract/tripwire split and its Go-upgrade gate, the non-migration of `rescap`, the two-adapter test seam (including the injectable `SameFile`), native Windows CI, the absence of any provenance persistence, the descriptor-close contract, the **withdrawal of every bounded-runtime claim**, Cobra parse-error ownership, and the deliberate exit-3 workspace divergence

## Revision history

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | NEEDS REVISION (internal), APPROVED WITH NOTES (external) | First draft. |
| rev-1 | NEEDS REVISION (internal and external) | Readiness now evaluates the **full intent bundle** (§6.2). CLI output composed with the real root error printer (§10.1, §9.5). Slug validated before any path is composed (§7.2). Race-safe platform open + `Max+1` bounded read replace the `Lstat`→ordinary-open design (§7.4). Total per-artifact ladder rebuilt (§7.5). Advisory selection made a total state→advisory function (§10.4). Absent `status.json` continues instead of aborting (§9.4). `unknown` provenance given a stable, forward-compatible definition (§11.1). Acceptance matrix rebuilt to 140 rows; claims audit to 75. |
| rev-2 | NEEDS REVISION (internal and external) | Path architecture pivoted to **one held `*os.Root`** opened for the repository root; every status and artifact `Lstat`/open is handle-relative through it (§7.3, §7.4). Ancestor-symlink races addressed with an honest same-identity limit (§7.4.4). `io.ReadAll(io.LimitReader(...))` replaced by **one preallocated `MaxArtifactBytes+1` buffer** with defined EOF semantics (§7.4.5). `status.json` inspected through the same discipline with its own cap and a **total** population table mapping every failure to a closed abort code, message and remediation (§9.4). `FeatureState` validated before echo (§9.4.3). Windows contract rebuilt on `os.Root`'s handle-relative primitives instead of a raw `CreateFile`; native `windows-latest` CI is now an acceptance obligation (§7.4.3, §16.1, §17). `--path` / workspace-discovery exit ownership corrected to exit 3 (§9.2, §9.3). Abort messages closed as exact templates (§9.4.5). "Printable ASCII" replaced by a control-byte/argument-byte rule (§14.3). `slug-unsafe` remediation made loop-free (§7.2). Guard arithmetic corrected (§18.26, renumbered §18.27 in rev-3). Acceptance matrix rebuilt to 188 rows; claims audit to 88 repository claims plus 12 Go-standard-library claims. |
| rev-3 | NEEDS REVISION (internal and external) | Companion **[ADR-034](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md)** created (Proposed rev-0) and reviewed with this PRD. `os.Root`'s guarantee restated as **logical rooted pathname confinement**, explicitly *not* physical filesystem or repository-content confinement — mount points, bind mounts, `/proc` files and device files remain reachable inside the namespace and are handled by the kind and bounded-read gates (§7.3, §7.4.2). Workspace discovery declared **outside** the rooted capture (§7.3, §9.3). `safety.EnsureSafeRepoPath` **removed** from the design; root-relative names are canonical `fs.ValidPath` names (§7.3). Platform selection converted from a fail-open denylist to a **fail-closed allowlist**, so an unmatched future `GOOS` aborts (§7.4.1). Slug validation moved **before** the platform check (§9.3). Windows reparse mapping corrected (AF_UNIX → `ModeSocket`, DEDUP deliberately regular, junction/name-surrogate behavior, `ModeIrregular` only in the default branch) and `//go:debug winsymlink=1` pinned in `package main` (§7.4.3). `O_NOFOLLOW` removed from the caller's flags as inert; `O_NONBLOCK` pass-through recorded as an implementation tripwire (§7.4.3). Identity promise weakened to "an object **observed as different** is never read", with hard-link alias, file-ID reuse, swap-and-restore and walk→`Lstat` window limits stated (§7.4.4, §8.3). Pre- **and** post-component walks per capture (§7.4.4). **One** reusable `MaxArtifactBytes+1` scratch buffer per inspection command, reused sequentially by the status capture and all four artifacts (§7.4.5). `RootOps`/`FileOps` test seam defined (§7.1.1). Attacker-byte guarantees scoped to command-owned output (§14.3). Native Windows junction fixtures must **fail, not skip** (§16.1). Every drifted prose→`AVP` citation corrected and a mechanical citation-resolution guard added (AVP-202). Acceptance matrix 188 → 202 rows; claims audit 88 → 92 repository claims and 12 → 21 Go-standard-library claims, each classified as a public **contract** or a version-pinned implementation **tripwire**. |
| rev-4 | NEEDS REVISION (internal), APPROVED WITH NOTES (external) | Narrow revision closing the rev-3 adjudication's four implementability gaps and four contract-parity corrections. **(1) Platform allowlist narrowed to `unix \|\| windows`** and its exact negation; `wasip1` is now an **unsupported** target that aborts `workspace-unsupported-platform`, because this design's `openFlags()` split and its FIFO tripwire are specified for exactly two target classes and no `wasip1` implementation, runner or fixture is proposed. The rev-3 "byte-identical to the stdlib tag" claim is **withdrawn** and replaced by an asserted **subset** relation (§7.4.1, AVP-191, AVP-208). **(2) Every universal bounded-runtime / no-unbounded-wait / no-hang claim is withdrawn** (§2.1, §5.1, §7.4.2, §7.4.3, §8.3, §14.1, §15, §16.2, §19 R18). What is guaranteed is **bounded allocation** and **bounded bytes requested**, plus a tested non-blocking *open* for the static and raced FIFO cases. Ordinary reads of a regular file on a mounted, network, `/proc` or FUSE filesystem **may block indefinitely**, and v1 offers **no timeout and no cancellation** (Q11, AVP-207). **(3) The test seam gains an injectable `SameFile(a, b fs.FileInfo) bool`** so identity-equal and identity-different rows are writable without constructing a private `os.fileStat`; the production adapter delegates to `os.SameFile`. `RootOps` is now three methods, `FileOps` three, and there are **exactly two** production adapters — one per interface, both declared in `internal/intent` (§7.1.1, ADR D12, AVP-194, AVP-206). **(4) `Close` is now explicit in both ladders**: artifact ladder row 20c and status ladder row 16a, evaluated after the last descriptor use and **before** any content classification, with a stated zero-leak obligation and **no new abort code** (§7.4.4 step 11a, §7.5, §9.4.2, AVP-203…AVP-205). **(5) Cobra parse-error ownership rewritten accurately**: the root command sets `SilenceUsage`/`SilenceErrors` (`internal/cli/cobra.go:56-62`), so cobra prints **no usage block and no error**; the repository's own root printer emits pflag/Cobra's error text **unsanitized** and `exitCodeFor` maps it to generic exit 1. Those pre-`RunE` bytes are outside this command's report schema and outside its security guarantee, and AVP-193 becomes a real scoped guard (§9.5, §10.1, §14.3.1). **(6)** `eleven codes` → **`twelve codes`** in §9.5 and §10.5.2; twelve canonical-slug-known codes plus `slug-unsafe` = the unchanged **thirteen**. **(7)** G18's anchor corrected to the `io/fs` package doc's `# Path Names` section. **(8)** ADR D7's name-surrogate column corrected: the predicate is the `0x20000000` tag bit, not a two-tag list. **(9)** §9.2 records that this command's exit-3 `workspace-not-initialized` **deliberately diverges** from every existing command's generic exit 1. **(10)** The fixed-buffer rationale no longer claims `io.ReadAll(io.LimitReader(...))` is unbounded — it is bounded, but allocates a growth sequence per capture; the fixed reusable buffer caps and flattens that across five sequential captures, at a stated one-time ~4 MiB zeroing cost (§7.4.5, G20–G22). Acceptance matrix 202 → **208 rows**; claims audit 92 → **95 repository claims** and 21 → **24 Go-standard-library claims**. |
| rev-5 | APPROVED (internal and external) | Final no-decision-change correction: AVP-082 no longer contradicts D16 by promising bounded runtime; the advisory catalog count is corrected from eleven to ten; the identity-limit paragraph says five cases because it lists five; ADR/PRD revision labels are synchronized. |
| rev-6 | **Errata — Accepted status retained, no decision change** | Implementation erratum raised by the GH #16 rev-0 joint review. rev-4's two-half `openFlags()` partition (`!windows` / `windows`) **does not compile** on the targets rev-4 itself moved into the unsupported branch: `syscall.O_NONBLOCK` is undefined in `syscall` for `js/wasm` and `plan9`, so a `!windows` file referencing it fails to build for exactly the `GOOS` set D5 refuses. Buildability therefore requires **three** halves — `//go:build unix`, `//go:build windows` and `//go:build !(unix \|\| windows)` — with the third returning a constant `0` that is **never reached in production** because §9.3 step 2's platform gate aborts `workspace-unsupported-platform` before any root is opened or any name is composed. The product contract is unchanged: the supported partition is still exactly `unix \|\| windows`, the Unix half still returns exactly `syscall.O_NONBLOCK`, the Windows half exactly `0`, and no unsupported target ever opens a root. §7.4.1 and §7.4.3, AVP-118 and AVP-208 are amended to describe the three-half partition and the unreachability obligation; ADR-034 D5/D6 carry the matching rev-3 errata. No acceptance row is added, retired or re-kinded; the matrix remains **208** rows and the guard set remains **43**. |
| rev-7 | **Errata — Accepted status retained, no decision change** | Cross-contract synchronization required by accepted ADR-035 D9 and `PRD-prepare-intent-bundle.md` PIB-143: §11.2 now names `intent-archive/**` explicitly as replaced-byte/recovery data, not authorship/provenance, and forbids provenance inference from it. No acceptance row or guard is added, retired or re-kinded; the matrix remains **208** rows and the guard set remains **43**. |

## Summary

`tpatch` cannot currently answer, truthfully and in one place, the question
**"which intent artifacts does this feature actually have, and where did they
come from?"** Today the closest answers are wrong in different directions:
`--manual` accepts a zero-byte `spec.md` (`internal/store/manual.go:51-81`),
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
structural-readiness verdict from the three canonical Markdown artifacts,
reports `provenance: unknown` for every artifact because no durable
per-artifact source metadata exists yet, and exits with a per-command code
contract.

Every filesystem operation it performs **after workspace discovery** — including
the `status.json` read whose value it echoes — goes through **one `*os.Root`
handle opened for the repository root** (Go 1.26), so no *name* it resolves can
escape the repository root as a logical pathname, and a pre-read identity
comparison guarantees an object **observed as different** is never read (§7.3,
§7.4).

Two boundaries of that claim are stated up front, because rev-2 overstated
both. **(a)** `os.Root` provides *logical rooted pathname confinement*, not
physical filesystem or repository-content confinement: a mount point, a Linux
bind mount, a `/proc` special file or a Unix device file that lives inside the
root's namespace is reachable through it, by the `Root` doc comment's own
words. Those leaves are handled by the kind gate and the bounded read, not by
the root (§7.4.2). **(b)** Workspace discovery — `store.FindProjectRoot`, which
resolves `--path` and walks upward for `.tpatch/` — necessarily runs **before**
`os.OpenRoot` and is therefore **outside** the rooted capture. It is an
ordinary pathname walk in the shipped store package, it is not part of this
command's confinement claim, and §9.3 orders it explicitly.

A third boundary is stated up front because **rev-3** overstated it. This
command's cost guarantees are about **space, not time**: it allocates one fixed
buffer per invocation and requests a bounded number of bytes per capture, and
it opens non-blockingly on Unix so a static or raced FIFO cannot wedge the
open. It does **not** guarantee that the process terminates in bounded time. An
ordinary `read(2)` on a regular file that lives on a stalled NFS mount, a FUSE
filesystem, a `/proc` entry or a slow device can block indefinitely, and v1
registers no `--timeout` and threads no cancellable context. rev-3's "the
command has no unbounded wait anywhere" is **withdrawn** (§7.4.2, §8.3, §14.1,
Q11, AVP-207).

It also makes one decision the WP-005 Turn 4 review demanded explicitly
(§12): **the existing mutating `analyze|define|explore --manual` gates do not
change in this slice.** The inspector is a pure, shared function, but it is
wired to `prepare --check` and nothing else.

### The readiness decision, stated up front

`prepare --check` reports `structural_readiness: ready` only when **all three**
canonical Markdown artifacts — `analysis.md`, `spec.md`, `exploration.md` — are
present and non-empty. `artifacts/analysis.json` remains optional and can never
affect readiness.

This does **not** make exploration mandatory. Nothing in `tpatch` requires an
operator to run `prepare --check`: it is not called by `cycle`, not emitted by
`next`, and not a precondition for any command (§5.5, §13.2). `define --manual`
still reaches `defined` without an `exploration.md`, and `next` still routes
exactly as it does today.

What changes is only what the *voluntary* report means when you ask for it.
`prepare` is the intent-**bundle** verb (WP-005 Agreed item 7 fixes the
publication unit as the three Markdown files plus sidecars plus the final
`status.json` transition,
`docs/whitepapers/WP-005-spec-driven-workflows.md:79-83`). A bundle-completeness
check that declared a bundle complete while a third of it was missing would
reproduce, at the report layer, exactly the category error §1 documents at the
gate layer. §6.2 works this through in full, including the WP-005 Agreed item 6
"must not make exploration mandatory" constraint it has to satisfy.

## 1. Problem statement

### 1.1 Structural presence is checked inconsistently, and never as a set

`AdvanceStateManually` is the single source of truth for `--manual`
(`internal/store/manual.go:27-32,51-81`). It:

- resolves a fixed per-phase artifact path,
- refuses when the path does not exist,
- refuses when the path is a directory,
- validates content **only** for `implement`, whose row is the only one with
  `ValidateJSON` set (`internal/store/manual.go:31`); that branch rejects
  whitespace-only bytes and invalid JSON (`internal/store/manual.go:67-78`),
- and therefore **accepts a zero-byte or whitespace-only `analysis.md`,
  `spec.md` or `exploration.md`** and advances lifecycle state on it.

It also uses `os.Stat` (`internal/store/manual.go:57`), which follows symlinks.
A `spec.md` symlinked to a file outside the repository satisfies the current
gate.

### 1.2 Path A and Path B produce different artifact sets

Path A's `RunAnalysis` writes **two** files: the structured sidecar
`artifacts/analysis.json` and the human `analysis.md`
(`internal/workflow/workflow.go:89-98`). `RunDefine` reads the sidecar
opportunistically and ignores the error when it is missing
(`internal/workflow/workflow.go:117-121`). Path B — `analyze <slug> --manual` —
requires only `analysis.md` (`internal/store/manual.go:28`) and writes no
sidecar.

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
next phase cannot prove **per-artifact** authorship — and
`docs/agent-as-provider.md:47-54` currently presents that string as if it could
(§16.1 requires that wording be corrected).

The one existing per-artifact sidecar, `artifacts/recipe-provenance.json`
(`internal/workflow/implement.go:18-34,222-238`), does not close this gap
either: it records `base_commit`, `generated_at` and `recipe_sha256` — commit
and content anchors, not *source of authorship* — it covers only
`apply-recipe.json`, it is written only by the Path A `RunImplement` path, and
it is skipped entirely outside a Git repository
(`internal/workflow/implement.go:222`).

### 1.4 A mutating `prepare` would amplify all three

WP-005 Turn 2 concluded that `prepare` is a valid product seam but not an
implementation authorization, and that a mutating bundle must not be built on
top of validation this weak
(`docs/whitepapers/WP-005-spec-driven-workflows.md:59-64,75-83`). This PRD is
the prerequisite contract.

## 2. Goals / Non-goals

### 2.1 Goals

1. Ship one read-only command, `tpatch prepare <slug> --check`, that reports
   the structural state of the canonical intent artifacts for one feature.
2. Define a **closed, deterministic** structural vocabulary and a total
   classification function over it: every reachable filesystem condition maps
   to exactly one state (§7.5).
3. Define required vs optional artifacts, and derive structural readiness from
   the required set only (§6.2, §9.1).
4. Report the Path A analysis sidecar **separately**, never as a readiness
   input, and never as evidence of which path produced the feature.
5. Emit `provenance: unknown` for every artifact in v1, with a definition that
   stays true after a future PRD adds known values (§11.1).
6. Define the human and JSON output shapes, key order, stream routing,
   process-level exit codes and error precedence exactly enough to test
   byte-stably — **composed with the real root error printer**
   (`internal/cli/cobra.go:33-39`), not against an idealised one.
7. Guarantee zero mutation; a **rooted** namespace whose logical pathname
   resolution cannot escape the repository root (§7.3, and §7.4.2 for what
   that does and does not mean); refusal of every **observed** symlink or
   reparse component, checked before and after each capture; a pre-read
   identity check so an object observed as different is never read; a
   descriptor that is closed exactly once per successful open, with the close
   outcome evaluated before any content classification (§7.4.4, §7.5);
   **fixed-allocation** bounded reads from one reused scratch buffer; no
   content echo; no hostile-byte echo in command-owned output; and no
   wall-clock in output.
   **This goal is about space and correctness, not time.** It does **not**
   include a bounded-runtime, no-hang or no-unbounded-wait guarantee; §7.4.2
   and §8.3 state why, and AVP-207 guards the documents and the shipped
   strings against restoring the claim.
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
   is good, complete, sufficient, well-formed prose, or correct (§7.7).
6. **No persistent provenance representation is selected** in this rev (§11.4).
7. **No routing change.** `next` and `cycle` do not consult the inspector
   (§13.2). WP-005 Agreed item 9
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:91-94`).
8. **No downstream methodology mandate.** Running `prepare --check` is never
   required to use tpatch, and no skill, doc or command may present it as a
   required step (§16.2).
9. **No repair, no `--fix`, no backfill, no migration of existing features.**

## 3. Terminology

| Term | Definition in this PRD |
|---|---|
| **Intent artifact** | One of the four canonical files in §6.1. Nothing else. |
| **Intent bundle** | The three canonical Markdown artifacts as a set. This is the unit `prepare` is named after and the unit readiness is computed over (§6.2). |
| **Canonical path** | The fixed, non-configurable **root-relative**, slash-separated name of an intent artifact, resolved only through the held `*os.Root` (§7.3). It is a canonical `fs.ValidPath` name: no leading slash, no `.` or `..` element, no empty element. Not user-supplied, not derived from flags, never composed into an absolute path inside the inspector. |
| **Canonical slug** | A feature name matching the grammar in §7.2. The only string this command will ever join into a path. |
| **Structural state** | One value of the closed enum in §7.6. A fact about the file's existence, kind and byte shape. Never a statement about meaning. |
| **Required artifact** | An artifact whose structural state participates in the readiness verdict (§6.2). |
| **Optional artifact** | An artifact that is reported but can never change the readiness verdict (§6.2). |
| **Structural readiness** | The three-valued verdict `ready` / `not_ready` / `indeterminate` over the required set only (§9.1). |
| **Provenance** | The *source of authorship* of one artifact. v1 emits only `unknown`, whose stable meaning is fixed in §11.1. |
| **Capture** | The single per-artifact descriptor-scoped read performed by the inspector. Classification reads the capture, never the file a second time (§8.2). |
| **Capture window** | The interval from the pre-capture component walk through the leaf `Root.Lstat` to the post-read `(*os.File).Stat`, the post-capture component walk and the descriptor `Close` for one artifact (§7.4.4, §8.3). `status.json` has its own, separate window (§9.4.2). |
| **Snapshot instability** | An observation that the artifact changed identity, kind or size across its own capture window (§8.3). |
| **Abort** | A precondition failure that ends the run before any artifact is inspected, with one of the thirteen closed codes of §9.4.4. |
| **Semantic quality** | Anything about the artifact's content beyond "at least one non-whitespace byte exists" and, for the sidecar only, "these bytes parse as a JSON object". Out of scope in v1 and every acceptance row. |

## 4. Existing-primitives preflight

WP-005 §6.2 requires this section
(`docs/whitepapers/WP-005-spec-driven-workflows.md:504-518`). Each row below is
a bounded, cited reason the existing primitive cannot answer the read-only
bundle question.

| Primitive | What it does today | Why it does not solve this |
|---|---|---|
| `analyze\|define\|explore --manual` | Mutating single-artifact adoption: existence + not-a-directory, content validation only for `implement` (`internal/store/manual.go:27-32,51-81`). | Mutates state as a side effect of asking. One artifact per invocation, no bundle view. Cannot report `analysis.json`. Uses `os.Stat`, so it follows symlinks. Accepts whitespace-only Markdown. Emits no machine-readable report and no per-artifact exit contract. |
| `cycle` | Runs analyze → define → explore → implement → apply → record, calling the provider at each phase (`internal/cli/phase2.go:25-145`); `--skip-execute` stops after recipe generation (`internal/cli/phase2.go:122-127`). | Maximally mutating and provider-driven. Its `assertCycleState` reads state *after* a write. There is no inspect-only entry point. |
| `next` | Emits one `HarnessTask` and distinguishes the two `defined` sub-states from raw `exploration.md` presence (`internal/cli/phase2.go:437-446`) using `fileExistsAt`, an `os.Stat` wrapper (`internal/cli/phase2.go:555-558`). | Answers "what do I run next", not "what do I have". `os.Stat` cannot distinguish empty from non-empty, symlink from regular, or unreadable from absent. Output is a task envelope with `--format text\|harness-json`, not an artifact report. No exit-code contract. |
| `verify` | `intent_files_present` checks `spec.md` **and** `exploration.md` exist and have non-zero size (`internal/workflow/verify.go:413-439`). | Refuses every pre-apply lifecycle state before V1 ever runs (`internal/workflow/verify.go:245-252`), which is precisely the window `prepare --check` serves. Omits `analysis.md` and `artifacts/analysis.json`. `info.Size() == 0` passes a whitespace-only file. `os.Stat` follows symlinks. Persists a `Verify` record unless `--no-write` (`internal/workflow/verify.go:347-352`), so it is a writer by default. Its 11-check report is a different contract with a different audience. |
| `status` | Lifecycle dashboard over `ListFeatures` (`internal/store/store.go:209-227`). | Renders `FeatureStatus` fields. It has never inspected artifact files, and adding artifact classification to it would change a shipped read surface's meaning for every feature at once. |
| `doctor` | D1–D8 workspace metadata drift; D1 validates `status.json` and flags legacy `feature.yaml` (`internal/workflow/doctor_d1.go:14-52`). | Scoped to metadata/manifest drift across the workspace, is `--fix`-capable (a writer), and reports findings, not a per-feature readiness verdict. No doctor check reads `analysis.md`, `spec.md`, `exploration.md` or `artifacts/analysis.json`. |
| `store.Slugify` | Normalises free text into a kebab-case directory name (`internal/store/slug.go:14-51`). | It is a *producer*, not a validator: it never rejects, it rewrites. Feeding a hostile argument through it would silently inspect a different feature than the one named. §7.2 needs an accept/refuse predicate, which does not exist today. |
| `safety.EnsureSafeRepoPath` | Lexical containment only (`internal/safety/safety.go:11-28`), described as "the coarse pre-filter that runs before any Lstat of any component" (`internal/rescap/pathgate.go:50-54`). | **Not used by this design at all** (rev-3 removal). It is a *lexical* test over a pathname, and it says nothing about symlinks, file kind, readability, resolution or size. rev-2 kept it as "defence in depth" over a root-relative name, which was incoherent twice over: the names it would have checked are compile-time constants joined to an already-validated canonical slug, and its containment notion is defined against a repository *prefix* rather than a `Root` handle, so under `--path` from an unrelated cwd it is checking a relationship the inspector does not have. §7.3 replaces it with an `fs.ValidPath` canonicality assertion on the composed name, which is the property that actually matters for a `Root` method argument. |
| `rescap.GatePath` | Full five-step pathname-walk / descriptor-identity gate (`internal/rescap/pathgate.go:68-83`). | Correct in spirit but scoped to the typed-resource capture domain, refuses on a *missing* path (`ReasonPathMissing`), refuses non-regular kinds through a different vocabulary, and returns an open descriptor with a lock/scratch lifecycle. `prepare --check` must treat "missing" as an ordinary reportable state, not a refusal. It is also **pathname-resolution based**, and its Windows half is an unsupported compile-only stub (`internal/rescap/pathopen_windows.go:1-20`). §7.3 reuses its ancestor-walk **policy** and nothing else; the mechanism is `os.Root`. |
| `store.LoadFeatureStatus` | Reads and unmarshals `status.json` (`internal/store/store.go:351-361`). | The obvious way to answer "what state is this feature in", and the one rev-1 used. It is `os.ReadFile` on an absolute pathname: follows symlinks, unbounded, no kind check, no identity check, outside any rooted namespace, and it cannot distinguish an unrecognised `FeatureState` from a valid one. §9.4 replaces it for this command, and §7.1 forbids it. |
| `os.Root` (Go 1.26) | Directory-handle-relative filesystem access whose methods refuse to resolve outside the root (`$GOROOT/src/os/root.go`; `go.mod:3`). | This *is* the primitive §7.3 adopts. Listed here for completeness: it solves confinement and nothing else — it does not classify, does not bound reads, does not bound wait time, follows in-root symlinks, and is not implemented against a held descriptor/handle on every `GOOS` Go can target. §7.4.1 refuses every target outside the `unix \|\| windows` allowlist rather than enumerating today's unconfined ones. The rest of §7 is what turns it into an answer. |
| `rescap.readBounded` | Cap-plus-one bounded read whose header states the reason: a pre-read `Stat().Size()` check alone can be bypassed by a file that grows (`internal/rescap/content.go:9-11,50-70`). | The right *discipline*, wrong package boundary, wrong refusal type, and a growable `append` buffer rather than a fixed allocation. §7.4.5 adopts the cap-plus-one reasoning with one preallocated `MaxArtifactBytes+1` buffer and maps the overflow onto a reported state rather than a refusal. |
| `store.WriteFeatureFile` / `WriteArtifact` / `writeFile` | The write side (`internal/store/store.go:443-449,461-472,918-922`). | Listed here only because `prepare --check` must call none of them (§10.6), and because `writeFile` → `os.WriteFile` truncates in place, which is the concurrency hazard §8.3 exists to classify honestly. |

**Conclusion.** No existing surface answers the pre-implementation,
read-only, four-artifact question. `prepare --check` is not an alias for an
existing stop point.

## 5. CLI grammar and surface boundary

### 5.1 Authorized grammar (v1, complete)

```text
tpatch prepare <slug> --check [--json] [--quiet] [--path <dir>]
```

- `<slug>` — exactly one, required, validated by §7.2 before use.
- `--check` — **required** in v1. It is the only mode.
- `--json` — emit the structured report on stdout.
- `--quiet` — suppress the per-artifact human output.
- `--path` — the existing root **persistent string** flag, inherited unchanged
  (registered at `internal/cli/cobra.go:66`, consumed at
  `internal/cli/cobra.go:3782-3793`). pflag validates nothing about its value,
  so its failure mode is exit 3, not exit 1 (§9.2).

No other flag is registered. There is no `--all`, no `--fix`, no `--format` and
no `--timeout`.

**Why no `--timeout`, stated correctly (rev-4 correction).** rev-3 justified
the omission with "nothing can time out", which was false. The honest
justification is narrower: the command constructs no provider, performs no
network I/O and spawns no subprocess (§14.4), so there is **no remote or
child-process wait to bound** — and v1 deliberately defines **no cancellation
contract** for the local filesystem reads it does perform. Those reads
**can** block: §7.4.3's `O_NONBLOCK` bounds the *open* of a FIFO or blocking
character device on Unix, and nothing in this design bounds an ordinary
`read(2)` on a regular file served by a stalled network mount, a FUSE
filesystem or a `/proc`-style provider. Q11 records adding a deadline or a
cancellable `context.Context` as a revisable, additive later decision;
AVP-207 is the build guard against the forbidden assertion that the command
cannot hang.

### 5.2 Name collision with `apply --mode prepare` — accepted, mitigated

`tpatch apply --mode prepare` already exists and means "write the agent
packet, mark apply progress" (`internal/cli/cobra.go:822-824,840,857`;
`SPEC.md:81`). A new top-level `tpatch prepare` verb therefore collides
lexically with an existing, unrelated apply mode.

Alternatives weighed:

| Option | Verdict |
|---|---|
| `tpatch prepare <slug> --check` | **Chosen.** The name is the one WP-005 Turns 2–4 and GH #10 reason about; renaming it here would fork the vocabulary between the accepted paper trail and the shipped surface. |
| `tpatch intent check <slug>` | Rejected: introduces a third noun (`intent`) for a four-file set that `docs/agent-as-provider.md:33-45` already calls the phase→artifact contract, and orphans the WP-005 → PRD → issue naming chain. |
| Rename `apply --mode prepare` | Rejected outright: it is a shipped, skill-referenced surface; renaming it is a breaking change wholly unrelated to this PRD. |

Mitigation is mandatory, not optional:

1. `tpatch prepare --help` must state, verbatim, that it is unrelated to
   `tpatch apply --mode prepare`.
2. `tpatch apply --help`'s `--mode` description must gain the reciprocal
   pointer.
3. Both are acceptance rows (AVP-009, AVP-010).

### 5.3 `tpatch prepare <slug>` without `--check` — reserved-surface refusal

Plain `prepare` is the mutating bundle. It is **not authorized by this PRD**.
Invoking it refuses at the top of `RunE`, **before the slug is validated,
before workspace discovery, and before `os.OpenRoot` is called**, so the
refusal is deterministic even outside a tpatch workspace and even for a hostile
slug. It is implementable exactly there: cobra has finished parsing (so
`--check` is readable from the flag set) and no name, root or filesystem call
has yet run.

The refusal is delivered as the message of an `*ExitCodeError{Code: 4}`
(`internal/cli/exit_error.go:12-33`), so the process emits exactly one stderr
line through the root printer (`internal/cli/cobra.go:33-39`):

```text
error: tpatch prepare requires --check in this release; the mutating intent-bundle form is not implemented. Run `tpatch prepare <slug> --check`, or `tpatch prepare --help` for the full grammar.
```

The remediation is **self-contained**: it names only shipped commands and the
command's own `--help`. It must not cite `docs/prds/…`, an issue URL, or any
other repo-internal path — a shipped binary's diagnostic cannot depend on a
document the user does not have, and rev-0's pointer at an undrafted PRD was
exactly that failure. AVP-006, AVP-007 and AVP-100 assert this.

Precedent for refusing a deliberately reserved surface with a typed non-1 code
rather than a cobra usage error: `amend --state`
(`internal/cli/c1.go:276-289`).

### 5.4 `--manual` and `--regenerate` — deliberately unregistered

Neither flag is registered on `prepare` in v1. Cobra therefore rejects them as
unknown flags **before `RunE` runs**, and the process exits `1` (§9.2, §14.3.1).
Because the root command sets `SilenceUsage: true` and `SilenceErrors: true`
(`internal/cli/cobra.go:56-62`), cobra prints neither a usage block nor the
error itself: the parse error is returned from `rootCmd.Execute()` and the
repository's own printer emits it as one `error:` line
(`internal/cli/cobra.go:33-39`), after which `exitCodeFor` maps the untyped
error to `1` (`internal/cli/cobra.go:43-52`). This is deliberate:

- registering a flag that immediately refuses advertises a capability in
  `--help` that does not exist;
- an unknown-flag error is the honest signal that the surface is absent, not
  disabled.

A future PRD that registers either flag changes the exit code for those inputs
from `1` to whatever it specifies. That is an enumerated behavior delta which
that PRD must state; it is not a silent change this PRD authorizes.

### 5.5 Interaction with other commands

`prepare --check` does not appear in `cycle`, is not emitted by `next` as an
`OnComplete`, and is not a precondition for any command. Nothing calls it, and
§13.2 asserts that mechanically in both directions (AVP-134, AVP-135).

## 6. Canonical artifact set

### 6.1 The four artifacts

Fixed, non-configurable, repo-relative paths under
`.tpatch/features/<slug>/`:

| `id` | Canonical path | Written by | Role |
|---|---|---|---|
| `analysis` | `analysis.md` | `RunAnalysis` (`internal/workflow/workflow.go:96`) or Path B by hand (`internal/store/manual.go:28`) | required |
| `spec` | `spec.md` | `RunDefine` (`internal/workflow/workflow.go:151`) or Path B by hand (`internal/store/manual.go:29`) | required |
| `exploration` | `exploration.md` | `RunExplore` (`internal/workflow/workflow.go:196`) or Path B by hand (`internal/store/manual.go:30`) | required |
| `analysis_sidecar` | `artifacts/analysis.json` | `RunAnalysis` only (`internal/workflow/workflow.go:89-90`) | optional |

`request.md`, `status.json`, `artifacts/apply-recipe.json`,
`artifacts/recipe-provenance.json`, `artifacts/post-apply.patch`, `record.md`,
`patches/**`, `artifacts/resources.json` and `artifacts/resource-captures/**`
are **out of scope**. `request.md` is an input, not an intent artifact;
everything else belongs to phases after `defined`
(`docs/feature-layout.md:10-32,86-94`). The set is closed for v1; adding to it
is a schema change (§10.3).

### 6.2 Required vs optional — the decision and its full justification

**Required: `analysis.md`, `spec.md`, `exploration.md` — the intent bundle.**

**Optional: `artifacts/analysis.json`.** Path A writes it; Path B does not
(§1.2). Requiring it would fail every agent-authored feature, and its presence
is not evidence of anything about authorship (§11.2).

#### 6.2.1 Why all three Markdown files

1. **`prepare` is the bundle verb.** WP-005 Agreed item 7 fixes the
   publication unit as "the three canonical Markdown files, structured
   sidecars and the final `status.json` transition"
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:79-83`), and Turn 3
   restates it (`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`).
   `prepare --check` is the read-only half of exactly that verb. A completeness
   check whose `ready` verdict is silent about one third of the unit it is
   named after is not a completeness check.
2. **The documented Path B flow already authors all three.**
   `docs/path-b-operator-guide.md:63-73` instructs the operator to hand-author
   `analysis.md`, `spec.md` and `exploration.md` and then run all three
   `--manual` commands. `ready` therefore describes the flow tpatch already
   teaches, not a new expectation.
3. **`verify` will require two of the three later, with block severity**
   (`internal/workflow/verify.go:413-439`). A `ready` verdict that could be
   followed by a blocking `intent_files_present` failure is a trap. Rev-0
   papered over this with an advisory; rev-1 removes the divergence for
   `exploration.md` outright and states the remaining, deliberate difference in
   §13.2: `prepare --check` additionally requires `analysis.md`, which `verify`
   does not check at all.
4. **Every failure mode is still fully reported, not hidden.** A feature with
   `analysis.md` and `spec.md` but no `exploration.md` reports
   `exploration: absent` with a remediation and `not_ready`, exit 2. That is a
   truthful description of an incomplete bundle, and §13.3 states plainly that
   it neither changes nor questions that feature's `defined` state.

#### 6.2.2 Why this does not make exploration mandatory

WP-005 Agreed item 6 says invoking the optional bundle command "must not make
exploration mandatory for every trivial feature"
(`docs/whitepapers/WP-005-spec-driven-workflows.md:73-76`). rev-1 satisfies it
because the constraint is about the **lifecycle**, and this command is not part
of the lifecycle:

| Question | Answer after this PRD ships |
|---|---|
| Must an operator run `prepare --check` at all? | No. Nothing calls it, nothing requires it (§5.5, §13.2). |
| Can a feature reach `defined` without `exploration.md`? | Yes, exactly as today: `define` and `define --manual` both mark `defined` (`internal/workflow/workflow.go:155`; `internal/store/manual.go:29`). |
| Does `next` change its routing for such a feature? | No — byte-identical (`internal/cli/phase2.go:437-446`; AVP-071, AVP-136). |
| Does `cycle` gain a gate? | No (AVP-072, AVP-137). |
| Does such a feature become invalid, downgraded or annotated? | No (§13.3, AVP-070). |
| Does anything fail because `prepare --check` says `not_ready`? | Only `prepare --check`'s own exit code. It is the only consumer of its own verdict. |

The distinction rev-1 draws, and which rev-0 conflated: **optional command
adoption** (nobody has to run it) is not the same as **relaxed bundle
semantics** (when run, it may under-report). Making the command optional is the
WP-005 constraint; weakening what its verdict means is not.

#### 6.2.3 Path A / Path B parity — reported, never inferred

v1 readiness is path-agnostic: a Path B feature with all three hand-authored
Markdown files and no sidecar is `ready`.

The sidecar gets its own reported row and a state-accurate advisory (§10.4).
When it is `absent`, the advisory `analysis-sidecar-absent-path-b-normal`
carries a fixed statement about **tpatch's** behavior:

> `artifacts/analysis.json` is written by the CLI-driven analyze phase and is
> not produced by `analyze --manual`. Its absence is not a defect.

**Hard constraint.** The report must contain no field, value or message that
asserts *this feature* was produced by Path A or Path B. That would be a
provenance inference from sidecar presence, which §11.2 forbids. Acceptance
rows AVP-073 and AVP-074 assert the absence of such a claim.

## 7. Slug validation, path policy and structural classification

### 7.1 Contract

```go
// package internal/intent — pure, read-only, no store, no provider.

// CanonicalSlug validates raw against §7.2 and returns the validated
// slug. It never rewrites; it accepts or refuses.
func CanonicalSlug(raw string) (string, error)

// Inspect returns a total Report. Every failure is a closed abort or a
// closed per-artifact state inside the Report; there is no error return.
//
// ops is the rooted operation set the CLI layer constructed from a
// *os.Root already opened for the repository root and owned by it.
// Every filesystem operation Inspect performs goes through ops (§7.3,
// §7.4). Inspect never composes an absolute path, never calls a
// package-level os filesystem function, and never closes the root.
// Inspect DOES close every descriptor it opens through ops, exactly
// once, and evaluates the close outcome before classifying content
// (§7.4.4 step 11a, §7.5 row 20c, §9.4.2 row 16a).
//
// scratch is the single reusable data buffer the CLI layer allocated
// for this invocation. It is exactly MaxArtifactBytes+1 bytes long and
// is reused, sequentially, by the status capture and by all four
// artifact captures (§7.4.5). Inspect never allocates a second data
// buffer and never grows this one.
//
// Inspect's cost guarantees are about space, not time: it performs a
// bounded number of allocations and requests a bounded number of bytes.
// It does NOT guarantee bounded runtime — an ordinary read through ops
// can block indefinitely on a stalled filesystem, and v1 provides no
// deadline and no cancellation (§7.4.2, §8.3, Q11).
func Inspect(ops RootOps, canonicalSlug string, scratch []byte) Report
```

`Inspect` performs filesystem reads only, all of them through `ops`. It must
not import `internal/store`, `internal/provider`, or `internal/gitutil`.

#### 7.1.1 The rooted operation seam

The decision recorded here is locked by ADR-034 D12.

rev-2 specified `Inspect(root *os.Root, …)` against the concrete type. That
made a whole class of acceptance rows unimplementable as written: rows that
require an **injected** `fstat` failure, an injected short or erroring read, an
injected `Close` failure, an injected identity verdict, or a deterministic
before/after race hook (AVP-083…AVP-085, AVP-107…AVP-115, AVP-148, AVP-151,
AVP-160…AVP-163, AVP-195, AVP-196, AVP-203…AVP-206) cannot be driven through
a concrete `*os.Root` without either a real hostile filesystem or a package
variable that production code also reads. rev-3 declared the seam; **rev-4
completes it with an injectable identity comparison and an explicit second
production adapter**:

```go
// RootOps is the whole filesystem surface the inspector may use.
// It is deliberately three methods wide: there is no ReadDir, no Walk,
// no Readlink, no Create and no pathname-taking method of any kind.
type RootOps interface {
    Lstat(name string) (fs.FileInfo, error)
    OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error)

    // SameFile answers the §7.4.4 step-6 identity question. It is part
    // of the seam because os.SameFile is only meaningful over the
    // unexported *os.fileStat values os produces: a test cannot
    // construct one, so with a package-level os.SameFile call every
    // identity-equal and identity-different row (AVP-084, AVP-151,
    // AVP-160, AVP-196) would be unwritable except through a real
    // hostile filesystem. The production adapter delegates verbatim.
    SameFile(a, b fs.FileInfo) bool
}

// FileOps is the descriptor surface. Read fills a caller-owned slice;
// the inspector never asks a FileOps for a new buffer. Close is part of
// the interface because its failure is a classified outcome
// (§7.5 row 20c, §9.4.2 row 16a), not a discarded error.
type FileOps interface {
    Stat() (fs.FileInfo, error)
    Read(p []byte) (int, error)
    Close() error
}
```

Rules that keep the seam from becoming an escape hatch:

1. **Exactly two non-test implementations exist — one per interface**, both
   unexported adapters declared in `internal/intent`:

   ```go
   type osRootOps struct{ root *os.Root }

   func (o osRootOps) Lstat(name string) (fs.FileInfo, error) { return o.root.Lstat(name) }

   func (o osRootOps) OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error) {
       f, err := o.root.OpenFile(name, flag, perm)
       if err != nil { return nil, err }
       return osFileOps{file: f}, nil
   }

   func (o osRootOps) SameFile(a, b fs.FileInfo) bool { return os.SameFile(a, b) }

   type osFileOps struct{ file *os.File }

   func (o osFileOps) Stat() (fs.FileInfo, error) { return o.file.Stat() }
   func (o osFileOps) Read(p []byte) (int, error) { return o.file.Read(p) }
   func (o osFileOps) Close() error               { return o.file.Close() }
   ```

   `osFileOps` is declared explicitly rather than returning the bare `*os.File`
   so that the "exactly one production implementation **per interface**"
   property is a fact about types declared in this package, which is what
   AVP-194 can actually assert by AST. rev-3's sample returned `f` directly,
   which made `*os.File` — a type declared in `os` — the production `FileOps`,
   and left AVP-194's wording ("declared outside a `_test.go` file") with
   nothing to point at. Every other implementation of either interface lives
   in a `_test.go` file. A sensitivity fixture adding a third production
   implementation fails AVP-194.
2. **The seam does not weaken the forbidden-reader guards.** AVP-089 and
   AVP-150 scan the *whole* package, test files included, for
   `os.Stat`/`os.Lstat`/`os.Open`/`os.OpenFile`/`os.ReadFile`/`os.ReadDir`/
   `filepath.Walk`/`store.LoadFeatureStatus` **outside** the two adapters and
   outside test *fixture construction*. Concretely, the guard's allowlist is:
   the adapter file may name `*os.Root` and `*os.File` methods and the single
   `os.SameFile` call; a `_test.go` file may call `os.WriteFile`, `os.Symlink`,
   `os.Mkdir` and friends to **build** a fixture tree. What no file may do —
   production or test — is read an intent artifact or `status.json` through a
   pathname reader. AVP-194's sensitivity fixture adds a pathname-based
   `RootOps` implementation in production and fails; AVP-206's asserts that
   `os.SameFile` appears at exactly one production call site, inside
   `osRootOps.SameFile`.
3. **Deterministic hooks are part of the seam, not of production.** A test
   implementation may carry `before(name string)` and `after(name string)`
   callbacks that fire immediately before and after each underlying call, which
   is how "replace the leaf between `Lstat` and `OpenFile`" becomes a
   deterministic unit test rather than a sleep-and-hope race. Production
   `osRootOps` has no hook field, and AVP-194 asserts each production adapter
   struct has exactly one field (`root`, `file`).
4. **The seam is `internal/intent`-unexported to the rest of the binary.**
   `RootOps` and `FileOps` are exported *within* the package for readability
   but the package itself is `internal/`, and AVP-134's reverse call-graph
   guard already limits the importer set to the `prepare` command file.
5. **The seam is six methods wide in total** — three on `RootOps`, three on
   `FileOps` — and none takes or returns an absolute path. Because `RootOps`
   exposes no mutator and no pathname-taking method, "the inspector cannot
   mutate and cannot enumerate" stays a *type-level* property. AVP-194 asserts
   the method counts, so widening either interface requires a PRD amendment.

The seam is the *only* thing rev-3/rev-4 add to make the injected-failure rows
implementable. It adds no configurability, no flag, no env var and no
production branch: on every production path there is exactly one `RootOps`
value, it wraps exactly one `*os.Root`, and every `FileOps` it hands back wraps
exactly one `*os.File`.

#### 7.1.2 Forbidden readers

rev-1 routed the `feature_state` echo through a caller-supplied `StateReader`
closure that wrapped `store.LoadFeatureStatus`. **rev-2 removed that seam, and
rev-3 does not bring it back** — `RootOps` is a rooted *primitive* seam, not a
"read this file for me" seam. `store.LoadFeatureStatus` is `os.ReadFile` on an
absolute pathname followed by `json.Unmarshal`
(`internal/store/store.go:351-361`): it follows symlinks, has no size bound,
has no kind check, has no identity check, and resolves outside the rooted
namespace. Using it would have left the single most security-relevant read in
the command — the one that names the feature's lifecycle state — as the only
read not covered by §7.4. So the status read is performed by the inspector
itself, through the same `RootOps` and the same discipline, and is specified in
§9.4. **No code path of this command may call `store.LoadFeatureStatus`,
`os.ReadFile`, `os.Open`, `os.OpenFile`, `os.Stat` or `os.Lstat`** (AVP-089,
AVP-150, AVP-194).

Because `internal/intent` cannot use the store's unexported path helpers
(`internal/store/store.go:779-797`), it declares its own canonical
root-relative path constants. AVP-088 is a parity guard asserting those
constants agree with `store.ManualPhase("analyze"|"define"|"explore")`
(`internal/store/manual.go:34-39`) so the two path tables cannot drift.

For the same reason it declares its own closed `FeatureState` value list
rather than importing `store.ValidFeatureState`
(`internal/store/types.go:39-46`). AVP-165 is the two-way parity guard: every
value in the inspector's list satisfies `store.ValidFeatureState`, and an AST
scan of the `FeatureState` const block (`internal/store/types.go:8-37`) yields
exactly the inspector's list. A thirteenth state added to `store` without a
matching entry fails the guard.

`Inspect` returning no `error` is deliberate: an inspector whose job is
totality must not have an escape hatch that renders a shape the output contract
does not describe.

### 7.2 Canonical slug validation — before any path is composed

**Rule: no byte of the user's slug argument is joined into a path, passed to a
filesystem call, or emitted on any stream until it has been accepted by
`CanonicalSlug`.**

**Ordering (rev-3 correction).** Slug validation is §9.3 precedence **step 3**
and the platform-confinement guard is **step 4**. rev-2 had them the other way
round, which broke a contract rev-2 itself asserted: §10.2 rule 8 says `slug`
is `""` **exactly** on `slug-unsafe`, and that biconditional is false if a
hostile slug on an unsupported target aborts `workspace-unsupported-platform`
with `slug: ""` instead. Validating the slug first restores the biconditional
on every target, and costs nothing: `CanonicalSlug` is a pure string predicate
that touches no filesystem, no root and no platform-conditional code. AVP-192
pins the order.

Grammar (a closed predicate, not a rewriter):

```text
slug := segment ( "-" segment )*
segment := [a-z0-9]+
1 <= len(slug) <= 60
```

Equivalently `^[a-z0-9]+(-[a-z0-9]+)*$` with a 60-byte cap. This is exactly the
value space `store.Slugify` produces — lowercase, `[a-z0-9-]` only
(`internal/store/slug.go:9-10,33-42`), no leading/trailing dash, no doubled
dash, capped at 60 (`internal/store/slug.go:12,44-51`) — and therefore accepts
every slug `tpatch add` can create (`internal/store/store.go:157-163`).
AVP-105 pins that round-trip.

Additionally refused, because a canonical slug can still be hostile on one
platform:

- any slug whose upper-cased form is a Windows reserved device name (`CON`,
  `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`). These parse as canonical
  under the grammar but resolve to a device rather than a directory on Windows.
  Refusing them on every platform keeps the contract identical everywhere and
  keeps §7.4.3's open away from a character device. `os.Root` independently
  refuses Windows reserved device names (`Root` doc comment), so the two
  mechanisms agree.

Refusal is the abort code `slug-unsafe` (§9.4.4), exit 3.

**No echo of hostile bytes.** On `slug-unsafe`:

- the JSON `slug` field is the empty string `""`;
- the human header prints `prepare --check  (slug withheld: not a canonical tpatch slug)`;
- the `--quiet` line prints `prepare --check — indeterminate (slug-unsafe)`;
- the abort message is the fixed template
  `the requested feature name is not a canonical tpatch slug. Canonical slugs are lowercase letters, digits and single dashes, 1-60 bytes. Create features with tpatch add, or rename a hand-made feature directory under .tpatch/features/ to a canonical name.`

**The remediation must not send the operator into a contradictory loop.**
rev-1's message said "Run `tpatch status` to list valid slugs". That is wrong
for the population it addresses: `ListFeatures` enumerates every feature
directory that carries a parseable `status.json`
(`internal/store/store.go:208-227`) and applies **no** canonicality filter, so
`tpatch status` will happily print the very non-canonical name that
`prepare --check` just refused, and the operator is told to re-run a command
that reproduces the refusal. rev-2's message is self-contained and actionable
in one step: create through `tpatch add` (which slugifies,
`internal/store/store.go:157-163`) or rename the directory. It names one
repo-relative path and two shipped commands, no `docs/` path, no URL
(AVP-186).

No stream ever contains the raw argument. This closes the traversal
(`../../etc`), absolute (`/etc/passwd`), control-byte, newline and non-ASCII
cases at once — none of them can be echoed, and none of them reaches the
rooted namespace. AVP-102…AVP-104, AVP-106.

**`slug` field domain.** The top-level `slug` field carries a canonical slug
whenever one exists, and `""` exactly when the run aborted with `slug-unsafe`.
It is never a rewritten or truncated form of the argument. Because §7.2's
ordering puts slug validation ahead of every other abort that could produce an
empty slug, the "`""` iff `slug-unsafe`" biconditional is total (AVP-106,
AVP-192).

### 7.3 Rooted path policy — logical pathname confinement

The decision recorded here is locked by
[ADR-034](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md) D1–D4.

rev-1 composed absolute pathnames with `filepath.Join` and then re-validated
them lexically and component-by-component with `os.Lstat`. That is the
pathname-resolution model, and it is the model every published
symlink-race defect against this shape exploits: between the component walk
and the open, the kernel resolves the *name* again, from scratch, with
whatever the tree looks like at that instant.

rev-2 replaces it with a **rooted namespace**. Go 1.26's `os.Root`
(`go.mod:3` pins `go 1.26.1`) is a directory handle plus a resolver that walks
one path component at a time relative to that handle, and refuses any
resolution that would leave it:

> Methods on `Root` can only access files and directories beneath a root
> directory. If any component of a file name passed to a method of `Root`
> references a location outside the root, the method returns an error. […]
> Methods on `Root` will follow symbolic links, but symbolic links may not
> reference a location outside the root. Symbolic links must not be absolute.
>
> Methods on `Root` do not prohibit traversal of filesystem boundaries, Linux
> bind mounts, /proc special files, or access to Unix device files.
>
> — Go 1.26 `os` package, `Root` doc comment (`$GOROOT/src/os/root.go`)

**Read the second paragraph as carefully as the first.** It is quoted here
rather than paraphrased because it is the sentence rev-2 elided, and eliding it
is what produced rev-2's over-broad "no operation can resolve outside the
repository" claim. What `Root` bounds is the **logical pathname**: the sequence
of names it resolves stays under the root directory. It does **not** bound the
physical filesystem, the physical device, or the semantics of the object a
name finally reaches. §7.4.2 states the consequence in full.

**The policy, in order:**

1. **One root, opened once, held for the whole run.** The CLI layer calls
   `os.OpenRoot(repoRoot)` exactly once, after workspace discovery, wraps it in
   the `osRootOps` adapter (§7.1.1) and passes that to `Inspect`. It is closed
   exactly once, by the CLI layer, after the report is rendered. `repoRoot` is
   the directory `store.FindProjectRoot` returned
   (`internal/store/store.go:23-40`). AVP-141, AVP-142.

   **Workspace discovery is outside the rooted capture, and this PRD says so
   rather than implying otherwise.** `store.FindProjectRoot` resolves the
   `--path` value with `filepath.Abs` and walks upward looking for `.tpatch/`
   (`internal/store/store.go:23-40`). That walk is an ordinary, symlink-
   following pathname traversal in the shipped store package. It runs
   **before** any root exists — it is what *produces* the directory the root is
   opened on — so it cannot be inside the root's guarantee, and no claim in
   this PRD covers it. rev-2's summary sentence "every filesystem operation it
   performs goes through one `*os.Root`" was therefore false as written; the
   true statement is "every filesystem operation **after workspace discovery**".
   The threat model consequence is stated in §14.1: an operator who points
   `--path` at a hostile directory gets a root opened on that directory, and
   the command then confines itself to it. AVP-189 is a mechanical guard that
   no shipped string, PRD sentence or ADR sentence makes the stronger claim.
2. **Root-relative names only, and canonical.** Every name handed to a `Root`
   method is a slash-separated, relative name composed from fixed constants and
   the canonical slug: `.tpatch/features/<slug>/analysis.md` and its three
   siblings, plus `.tpatch/features/<slug>/status.json`. No absolute path is
   ever composed inside `internal/intent`, and no flag, config key or artifact
   content ever contributes a component. `os.Root` splits on both separators
   on Windows (`$GOROOT/src/os/root.go`, `splitPathInRoot` → `IsPathSeparator`)
   and lexically cleans the name through `GetFullPathName` there
   (`$GOROOT/src/os/root_windows.go`, `rootCleanPath`), so one slash-joined
   constant set is correct on every target.

   **The canonicality rule is `fs.ValidPath`** (`$GOROOT/src/io/fs/fs.go`,
   `ValidPath`). The property it enforces is the one the **`io/fs` package
   documentation's `# Path Names` section** defines — UTF-8-encoded, unrooted,
   slash-separated, no `.`/`..`/empty element, no leading or trailing slash —
   with the documented special case that the single name `"."` is valid and
   denotes the root directory. `ValidPath`'s own doc comment states only that
   the name is "valid for use in a call to Open" and links to that section for
   the details; G18 anchors both, because rev-3 attributed the whole property
   list to the function comment alone. None of the names this design composes
   is `"."` — every one is a multi-element constant path joined to a canonical
   slug — so the special case changes no composed name here, but a future
   refactor that reached the resolver with a bare `"."` would satisfy
   `fs.ValidPath` and must not be assumed refused by it. Every composed name —
   the four artifact names, the status name,
   and every prefix used by the component walk — is asserted against
   `fs.ValidPath` before it is passed to a `Root` method, and a name that fails
   is a programming error, not a user-reachable condition: with fixed constants
   and a `[a-z0-9-]`-only canonical slug it cannot fail. The assertion exists
   so that a future refactor which parameterises a component cannot reach the
   resolver with a `..`, an absolute name, or an empty element. AVP-144.

   **`safety.EnsureSafeRepoPath` is not used** (rev-3 removal). rev-2 kept it
   as "defence in depth", which was wrong in kind: it is a lexical test defined
   against a repository *prefix string*, and this inspector holds a *root
   handle* and composes *relative* names. Under `--path <dir>` from an
   unrelated cwd there is no absolute prefix for it to test against that the
   inspector even knows. `fs.ValidPath` is the property that actually
   constrains a `Root` method argument, so that is what rev-3 asserts. AVP-144
   asserts the removal as well as the replacement, and AVP-089's source scan
   fails if `EnsureSafeRepoPath` reappears in `internal/intent`.
3. **Component policy: any *observed* symlink or reparse component is
   refused.** Each successive prefix of the relative name is inspected with
   `Root.Lstat`, which does not follow a symbolic link in the final component
   of the name it is given (`$GOROOT/src/os/root.go`, `Root.Lstat`). The
   refusal predicate is

   ```go
   info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
   ```

   and it is deliberately **not** the whole kind policy — it is the *refuse the
   component outright* half. The rest is closed by the regular-file gate
   (`Mode().IsRegular()`, ladder rows 7 and 14) and by the directory gate on
   ancestors. §7.4.3's Windows table gives the exact tag → mode mapping,
   including the two cases rev-2 got wrong (`IO_REPARSE_TAG_AF_UNIX` →
   `ModeSocket`, and `IO_REPARSE_TAG_DEDUP` → deliberately regular), which is
   precisely why the predicate cannot be the only gate. AVP-146, AVP-190,
   AVP-198, AVP-199.
4. **The walk decides by component order, first match wins.** For the feature
   directory prefix `.tpatch/features/<slug>`:
   - a component that is a symlink or reparse point → abort `feature-dir-unsafe`;
   - a component that does not exist → abort `feature-not-found`;
   - a component that exists, is not a symlink/reparse point and is not a
     directory → abort `feature-dir-unsafe`;
   - a `Root.Lstat` failure for any other reason → abort `feature-dir-unsafe`.

   This mirrors the ancestor-walk **policy** of
   `internal/rescap/pathgate.go:97-120` without adopting its
   missing-path-is-a-refusal semantics for the leaves. AVP-182.
5. **Per-capture walk, before *and* after.** For the remaining components
   (`artifacts/` for the sidecar) and the leaf, `Root.Lstat` only. Any symlink
   or reparse point → `symlink-refused`. The link **target is never resolved,
   never read and never named in output**. §7.4.4 specifies that the walk runs
   twice per capture — once before the open and once after the read — and what
   a post-walk observation means.
6. **No package-level `os` filesystem call, ever.** `os.Stat`, `os.Lstat`,
   `os.Open`, `os.OpenFile` and `os.ReadFile` all resolve a pathname outside
   the rooted namespace. Every current call site that inspects an intent
   artifact uses `os.Stat` (`internal/store/manual.go:57`,
   `internal/cli/phase2.go:556`, `internal/workflow/verify.go:416`), which
   additionally follows symlinks. The inspector uses `RootOps.Lstat`,
   `RootOps.OpenFile` and `FileOps.Stat` exclusively, and the one production
   implementation of those is the `*os.Root`/`*os.File` adapter of §7.1.1.
   AVP-089 and AVP-194 are the source scans; AVP-149's process-wide filesystem
   spy is the runtime check.
7. **No writes of any kind.** No directory creation, no lock file, no temp
   file, no `.orig` backup, no `FEATURES.md` refresh (§10.6). The root is
   opened read-only in intent: only `Lstat` and `OpenFile` with `O_RDONLY` are
   ever called on it — `RootOps` (§7.1.1) exposes no other method, which makes
   this a type-level property rather than a convention — and AVP-087 asserts no
   `Root` mutator (`Create`, `Mkdir`, `MkdirAll`, `Remove`, `RemoveAll`,
   `Rename`, `Chmod`, `Chown`, `Chtimes`, `Link`, `Symlink`, `WriteFile`)
   appears in the call graph.

### 7.4 Rooted, race-safe capture — the platform contract

#### 7.4.1 Platform scope — a fail-closed allowlist

The decision recorded here is locked by ADR-034 D5.

`os.Root`'s confinement guarantee is not uniform across every `GOOS` Go can
target, and the PRD does not pretend it is. The Go implementation splits three
ways (`$GOROOT/src/os/root_openat.go`, `root_noopenat.go`, `root_unix.go`,
`root_windows.go` build tags):

| Target class | Selected by | `os.Root` implementation | Logical confinement | Supported by this design? |
|---|---|---|---|---|
| `unix` | `//go:build unix \|\| wasip1` (the `unix` half) | `openat`/`fstatat` relative to a held directory descriptor (`$GOROOT/src/os/root_unix.go`) | Held-descriptor; escaping name refused | **yes** |
| `windows` | `//go:build windows` | `NtCreateFile`-based `openat` relative to a held handle, with `O_NOFOLLOW_ANY` and lexical pre-cleaning (`$GOROOT/src/os/root_windows.go`) | Held-handle; escaping name refused | **yes** |
| `wasip1` | `//go:build unix \|\| wasip1` (the `wasip1` half) | `openat`/`fstatat` relative to a held descriptor, same file as the `unix` half | Held-descriptor; escaping name refused | **no — refused** (see below) |
| everything else | the negation of the above | name-based, no directory handle (`$GOROOT/src/os/root_noopenat.go`, tagged `(js && wasm) \|\| plan9`) — **or nothing at all**, for a `GOOS` no `os/root_*.go` file matches | **Not guaranteed.** The `Root` doc comment states that on `js` it "is vulnerable to TOCTOU […] and cannot ensure that operations will not escape the root", and that on `js`/`plan9` "a `Root` references a directory name, not a file descriptor" | **no — refused** |

The three targets tpatch's CI exercises or plausibly ships to are `linux`,
`darwin` and `windows` (`.github/workflows/ci.yml:25`; §16.1 adds
`windows-latest`), and all three are supported.

**The selection is an allowlist and it fails closed** (rev-3 correction, kept).
rev-2 wrote the pair as a denylist:

```go
//go:build (js && wasm) || plan9        → false
//go:build !((js && wasm) || plan9)     → true
```

That is **fail-open**, and the failure is not hypothetical: it hard-codes the
claim that the *only* unconfined targets are the two that exist today. A future
Go release that adds a `GOOS` — or a build of this module for an existing
`GOOS` whose `os.Root` support Go later reclassifies — silently lands in the
`true` branch and the command asserts a confinement guarantee nobody checked.
The allowlist inverts the default:

```go
// confine_supported.go
//go:build unix || windows

package intent

// rootConfinementSupported records that this target is one of the two
// GOOS classes this design specifies, builds and tests: the ones for
// which os.Root is implemented against a held directory descriptor or
// handle AND for which §7.4.3 defines an openFlags() half. It is a
// deliberate strict SUBSET of the stdlib's confined set, which also
// contains wasip1 (§7.4.1, AVP-208).
const rootConfinementSupported = true
```

```go
// confine_unsupported.go
//go:build !(unix || windows)

package intent

const rootConfinementSupported = false
```

**`wasip1` is excluded, and rev-4 says why rather than leaving the gap
implicit.** rev-3's allowlist read `unix || windows || wasip1` and justified
itself as being *byte-identical* to `$GOROOT/src/os/root_openat.go`'s own build
tag. That justification does not survive contact with the rest of this design:

1. **The design has exactly two implementation halves, not three.** §7.4.3's
   `openFlags()` is build-tagged (rev-6 errata: `unix` / `windows` /
   `!(unix || windows)`; see §7.4.3), and its Unix
   half returns `syscall.O_NONBLOCK`. Under WASI preview 1, `syscall.O_NONBLOCK`
   is not the `openat` non-blocking bit this design reasons about, and the FIFO
   semantics AVP-107/AVP-200 are written against are not the semantics a
   `wasip1` host provides. Admitting `wasip1` into the `true` branch would
   therefore claim a property the `unix` half does not deliver there.
2. **No `wasip1` runner, fixture or cross-build is proposed.** §16.1 makes
   `windows-latest` an acceptance obligation precisely because an unexecuted
   platform path degrades silently (R7). Admitting a third target while
   proposing no runner for it reproduces that defect one platform over.
3. **Splitting the implementation is out of scope, and rev-4 does not do it.**
   A `wasip1` half would need its own `openFlags()` file, its own FIFO/device
   kind story and its own tripwire test. That is a separate slice with its own
   review; it is not a build-tag edit. **No split implementation is specified
   here** — the target is simply refused.
4. **Refusing costs nothing and fails closed.** `wasip1` lands in the `false`
   branch, aborts `workspace-unsupported-platform` (exit 3) before any name is
   composed, and the abort is total and tested (AVP-177, AVP-179).

**The asserted property is therefore subset-ness, not textual identity.**
rev-3's "the `true` expression is byte-identical to the stdlib's tag" claim is
**withdrawn**. What AVP-191 and AVP-208 assert instead is:

- the pair is exactly `unix || windows` and its exact syntactic negation
  `!(unix || windows)`;
- the two sets are exhaustive and disjoint over every `GOOS` in
  `go tool dist list`;
- **every `GOOS` in the `true` set is also matched by
  `$GOROOT/src/os/root_openat.go`'s build tag** — the design never claims
  confinement on a target where the standard library does not implement the
  confined resolver;
- `wasip1` is matched by the stdlib tag and **not** by ours, and that gap is
  recorded as a deliberate refusal rather than an oversight.

`js/wasm`, `plan9`, `wasip1` and any future `GOOS` that matches neither `unix`
nor `windows` land in the `false` branch and abort
`workspace-unsupported-platform` (exit 3) **before** `os.OpenRoot` is called
and before any name is composed.

This is a refusal, not a degraded mode: a read-only inspector that cannot
promise confinement — or that has no tested implementation half for a target —
must say so rather than silently inspect. AVP-191 asserts the tag texts and the
exhaustive/disjoint property, with a sensitivity half that fails when the pair
is rewritten back into rev-2's denylist form. AVP-208 asserts the subset
relation and the deliberate `wasip1` gap, with a sensitivity half that fails
when `wasip1` is re-added to the `true` tag without a `wasip1` `openFlags()`
half and a runner. AVP-177 retains the runtime half: the `false` path aborts
before any filesystem call. AVP-192 pins that this guard runs **after** slug
validation (§7.2, §9.3).

#### 7.4.2 What `os.Root` does and does not prevent

The scope statement recorded here is locked by ADR-034 D2.

**Prevents — logical rooted pathname confinement.** No *name* resolved through
the root can leave the root directory. Every component is resolved relative to
the held descriptor/handle; an absolute symlink is refused outright; a relative
symlink is re-resolved from the root with a `..`-escape check and a
symlink-count limit (`$GOROOT/src/os/root_openat.go`, `doInRoot`;
`rootMaxSymlinks = 8`). This holds no matter what symlinks exist inside the
tree and no matter when they are created.

**Does not prevent — and rev-2 claimed otherwise, which is the finding this
subsection closes:**

1. **It is not a physical filesystem boundary.** The `Root` doc comment says,
   verbatim: *"Methods on `Root` do not prohibit traversal of filesystem
   boundaries, Linux bind mounts, /proc special files, or access to Unix
   device files."* A mount point, a bind mount, a `/proc` entry or a device
   node that is reachable **by name** from inside the root is reachable
   **through** the root. The bytes such a leaf yields may physically originate
   outside the repository's own filesystem, outside its own device, and
   outside anything a Git checkout contains.

   rev-2 said the design guarantees "no path outside the repository is ever
   opened, read, or named" and that a raced link's blast radius is bounded to
   "some other object inside this repository". **Both sentences are removed.**
   The correct statements are: no *name* outside the repository root is ever
   resolved, and the blast radius of a raced in-root link is bounded to *some
   object nameable from inside this repository root* — which, on a tree with a
   bind mount or a device node, is not the same set as "this repository's
   content".
2. **It is not a repository-content boundary.** `os.Root` knows nothing about
   Git. A file inside `repoRoot` that is untracked, ignored, or a live device
   is as reachable as a tracked one.
3. **It is not a no-follow primitive.** `Root` **follows** symbolic links it
   encounters during resolution, as long as they stay inside the root. Two
   consequences the PRD states rather than papers over:
   - A symlink created between the `Root.Lstat` walk and the `Root.OpenFile` —
     at an ancestor component or at the leaf — **is** followed, in-root, by the
     open.
   - Therefore the open alone cannot be relied on to refuse a raced link. The
     refusal comes from §7.4.4's pre-read identity comparison and its
     post-capture walk.

**What actually closes the leaf cases, then.** Not the root — the *kind gate*
and the *bounded read*:

| Hazard the root does not close | What closes it |
|---|---|
| A leaf that is a mount point (Unix: an ordinary directory; Windows: a junction, a name-surrogate reparse point) | Ladder row 1/6 (`ModeSymlink\|ModeIrregular`, which a junction sets, §7.4.3) for the reparse form, and row 7 (`!IsRegular()`) for the directory form. A mounted **regular file** — a bind mount of a file — is not distinguishable from a regular file and is deliberately not claimed to be (AVP-190). |
| A leaf that is a `/proc`-style special file that reports size 0 and streams indefinitely | Row 7 refuses it when it is not a regular file. When such a file *does* report as regular (Linux `/proc` files report `ModeRegular` with size 0), the fixed `MaxArtifactBytes+1` read (§7.4.5) bounds it: at most `Max+1` bytes are ever requested, and the byte-count/size disagreement at row 18 classifies it `unstable` rather than reporting fabricated content. AVP-190 exercises exactly this shape with an injected `FileInfo`. |
| A leaf that is a Unix device file | Row 7 (`ModeDevice`/`ModeCharDevice` fail `IsRegular`), plus `O_NONBLOCK` (§7.4.3) so the *open* of a blocking character device returns instead of waiting for a carrier. |
| A leaf that is a FIFO | Row 7 pre-open; `O_NONBLOCK` for the raced case, which bounds the **open**, not the process. |

**What is not closed, stated as plainly as what is. The withdrawal recorded
here is locked by ADR-034 D16.** None of the gates above
bounds *time*. `O_NONBLOCK` makes the FIFO/device **open** return; it does not
make a subsequent `read(2)` return. A leaf that passes every kind gate — an
ordinary regular file — can still block the process indefinitely when it lives
on a stalled NFS or SMB mount, a FUSE filesystem whose server has wedged, a
`/proc`-style provider, or a device-backed file whose driver is not responding.
The same is true of a `Root.Lstat` on such a path. **rev-3's "no leaf kind can
hang it" and "the command has no unbounded wait anywhere" are withdrawn.** v1
adds no deadline, no `context.Context` and no watchdog; Q11 records that as a
revisable, additive decision, and AVP-207 fails the build if any shipped
string, this PRD or ADR-034 restores the claim.

The PRD makes **no** claim that the bytes it reads physically originate inside
the repository's filesystem, and **no** claim about how long it takes. It
claims that the *name* it resolved did not leave the repository root, that the
object it read was **observed as the same object** it classified, that at most
`MaxArtifactBytes+1` bytes were requested from any one descriptor, that exactly
one data buffer was allocated for the whole invocation, and that a FIFO or
blocking character device cannot wedge the **open** on Unix. AVP-189 is a
mechanical guard over every shipped string, this PRD's own text and ADR-034's
own text: none may contain a physical-confinement claim. AVP-207 is its
timing-dimension sibling.

#### 7.4.3 The open, and the Windows kind mapping

The decisions recorded here are locked by ADR-034 D6 and D7.

```go
// package internal/intent, one file per build tag.

// openFlags returns the extra open flags for the final leaf on this
// target. It never includes a write, create, truncate or append bit,
// and it never includes a flag whose only effect would be to duplicate
// something os.Root already does.
func openFlags() int
```

**Unix (`//go:build unix`)** — `openFlags()` returns
`syscall.O_NONBLOCK`, and the call is
`ops.OpenFile(rel, os.O_RDONLY|openFlags(), 0)`.

> **rev-6 erratum — three build halves, not two.** rev-4 wrote this half's tag
> as `!windows`. That does not compile. `syscall.O_NONBLOCK` is **not declared
> in `syscall` for `js/wasm` or `plan9`** — precisely the targets D5 moved into
> the unsupported branch — so a `!windows` file naming the constant fails to
> build for exactly the `GOOS` set the design refuses, and `go build` for those
> targets fails before any refusal can be reported. The partition is therefore
> **three** halves:
>
> - `//go:build unix` → `syscall.O_NONBLOCK` (the supported Unix contract,
>   unchanged);
> - `//go:build windows` → `0` (the supported Windows contract, unchanged);
> - `//go:build !(unix || windows)` → `0`, **buildability only**.
>
> The third half is dead code by construction, and that is the obligation the
> guard enforces rather than a comment: §9.3 step 2's platform gate returns
> `workspace-unsupported-platform` **before** `os.OpenRoot` is called and
> before any root-relative name is composed (§7.4.1, AVP-177, AVP-179), so on
> an unsupported target no capture, no open and therefore no `openFlags()`
> call is ever reached. The supported allowlist is still exactly
> `unix || windows`, the unsupported set still refuses, and no product
> behavior changes. AVP-118 and AVP-208 are amended to the three-half form;
> ADR-034 D5/D6 carry the matching rev-3 errata.

- `O_NONBLOCK` is the only flag, and it is load-bearing **for the open**. It is
  what keeps a read-only inspector from wedging on the `open(2)` of a FIFO or a
  blocking character device that appears at the leaf **after** the pre-open kind
  check refused the stable case. A stable FIFO never reaches the open at all —
  ladder row 7 refuses it from `Root.Lstat` — so `O_NONBLOCK` covers exactly the
  raced window: the open returns immediately, and the post-open kind recheck at
  row 14 classifies the result `unstable`. It has no effect on a regular file,
  so the happy path is unchanged. AVP-107, AVP-021.

  **Its scope is the open, and rev-4 states the limit that rev-3 elided.**
  `O_NONBLOCK` does not bound the subsequent `read(2)`, and it does not bound
  `Root.Lstat`. A regular file on a stalled network or FUSE mount blocks with or
  without it. The property this flag buys is "a FIFO with no writer cannot wedge
  the open"; it is **not** "this command terminates" (§7.4.2, §8.3, AVP-207).

- **`O_NONBLOCK` pass-through is an implementation dependency, not a public
  contract — this is a tripwire, and rev-3 labels it as one.** `os.Root`'s
  documentation does not promise that caller flags reach `openat`. The behavior
  is read from the pinned toolchain: `rootOpenFileNolog` computes
  `openFlag := syscall.O_NOFOLLOW | syscall.O_CLOEXEC | flag` and passes it to
  `unix.Openat`, then constructs the `*File` with
  `unix.HasNonblockFlag(flag)` (`$GOROOT/src/os/root_unix.go`). Both halves are
  unexported. G17 records the claim as a **tripwire** rather than a contract,
  and **AVP-200 is a Go-upgrade tripwire test**: it opens a real FIFO with no
  writer through a real `os.Root` under a hard deadline and fails if the open
  blocks. A Go release that stopped forwarding the flag would turn AVP-200 red
  at upgrade time instead of turning the *open* into a wedge in the field.
  AVP-200 bounds the open only; it asserts nothing about read time.

- **`O_NOFOLLOW` is removed from the caller's flags** (rev-3 correction). rev-2
  passed it "belt-and-braces" while simultaneously documenting that it cannot
  produce a refusal: `Root` already ORs it internally on the final component,
  and then converts the resulting signal into an in-root symlink *resolution*
  via `checkSymlink`/`doInRoot`'s `case errSymlink`
  (`$GOROOT/src/os/root_unix.go`, `$GOROOT/src/os/root_openat.go`). A flag the
  design documents as producing no observable effect is not defence in depth,
  it is a false affordance: the next reader of the code reasonably infers a
  no-follow guarantee that does not exist. rev-3 drops it, keeps the
  explanation (so the omission is deliberate and reviewable), and **no
  acceptance row anywhere claims a caller-supplied `O_NOFOLLOW` produces a
  refusal through `Root`.** AVP-118 asserts the flag set is exactly
  `O_NONBLOCK` on non-Windows and exactly `0` on Windows, that no write,
  create, truncate or append bit is present, and — the sensitivity half — that
  reintroducing `syscall.O_NOFOLLOW` fails the guard.

**Windows (`//go:build windows`)** — `openFlags()` returns `0`, and the call is
`ops.OpenFile(rel, os.O_RDONLY, 0)`.

rev-1 specified a raw `syscall.CreateFile` with
`FILE_FLAG_OPEN_REPARSE_POINT` and asserted that the open would *succeed* on a
reparse point and hand back a reparse-point handle, while the ladder
simultaneously classified the same case from an *open error*. That was
self-contradictory, and rev-2 removed it entirely. There is no raw
`CreateFile` in this design: reparse points are refused **before** the open, by
§7.3 step 3's `Root.Lstat` mode test, whose Windows implementation is itself an
`O_FILE_FLAG_OPEN_REPARSE_POINT` handle open plus a handle-derived stat
(`$GOROOT/src/os/root_windows.go` `rootStat`). The open is never reached for a
refused reparse point, so no claim about its error is needed.

**The reparse-tag → mode mapping, corrected.** rev-2 asserted that "every other
reparse tag […] sets `os.ModeIrregular`". That is **false**, and the two
exceptions matter. Under the current `(*fileStat).mode`
(`$GOROOT/src/os/types_windows.go`) the mapping is:

| Reparse tag | Name surrogate? (`FILE_ATTRIBUTE_REPARSE_POINT` set **and** `ReparseTag & 0x20000000 != 0`) | Resulting `FileMode` bits | Refused by §7.3 step 3's `ModeSymlink\|ModeIrregular` predicate? |
|---|---|---|---|
| `IO_REPARSE_TAG_SYMLINK` | **yes** (`0xA000000C`) | `ModeSymlink` | **yes** |
| `IO_REPARSE_TAG_MOUNT_POINT` (junction / volume mount point) | **yes** (`0xA0000003`) | `ModeIrregular`, via the `default` branch. Because it is a name surrogate, `mode()` **suppresses** the `ModeDir` bit and the `GetFileType`-derived bits first, so a junction to a directory does not look like a directory | **yes** |
| `IO_REPARSE_TAG_AF_UNIX` | **no** (`0x80000023`: surrogate bit clear) | `ModeSocket` — **not** `ModeIrregular`. Because it is *not* a surrogate, the `ModeDir` and `GetFileType`-derived bits are **not** suppressed | **no** — closed instead by the `!IsRegular()` kind gate at ladder rows 7 and 14 |
| `IO_REPARSE_TAG_DEDUP` | **no** (`0x80000013`: surrogate bit clear) | **no type bit at all**: Go deliberately treats a Data-Deduplication reparse point as a *regular* file, with an explicit comment saying so, because DEDUP files support ordinary random-access reads and should not flip from regular to irregular when the dedup job runs. Not being a surrogate, it keeps its `GetFileType`-derived bits | **no, and deliberately so** — it is classified and read as the regular file Go says it is |
| any other tag | **depends on the tag's own `0x20000000` bit** — `isReparseTagNameSurrogate` is a *bit test*, not an enumerated two-tag list, so a future or vendor tag may be a surrogate (bits suppressed) or not (bits kept) | `ModeIrregular`, via the `default` branch, either way | **yes** |

Three consequences the PRD now states plainly:

1. **The `ModeSymlink|ModeIrregular` predicate is a *refusal* predicate, not
   the kind policy.** It is not, and was never, sufficient on its own. AF_UNIX
   sockets pass it and are caught one row later by `!IsRegular()`. rev-2's
   "both bits are load-bearing and neither is redundant" was true but
   incomplete; rev-3 adds "and neither is sufficient".
2. **A DEDUP reparse point is inspected as a regular file.** This is a
   deliberate acceptance of the standard library's own decision, recorded here
   so it is a decision rather than an omission. AVP-198 asserts an injected
   DEDUP-shaped `FileInfo` (reparse attribute set, no type bit, `IsRegular()`
   true) is classified normally and **not** refused.
3. **The mapping is version-pinned behavior, not a documented contract.**
   `(*fileStat).mode` is unexported. G14 records it as a **tripwire**.

**`//go:debug winsymlink=1` is pinned in `package main`, and it is tested.**
`(*fileStat).Mode` consults the `winsymlink` GODEBUG setting: when it is `0`
the mode is computed by `modePreGo1_23` instead, under which a **junction maps
to `ModeSymlink`** rather than `ModeIrregular`
(`$GOROOT/src/os/types_windows.go`, `Mode` and `modePreGo1_23`). Both values
happen to be refused by this design's predicate, so the *refusal* is stable —
but the *reason code path*, the mode bits a test asserts, and any future
narrowing of the predicate are not. Two obligations follow, and both are
implementation obligations of this PRD rather than assumptions:

- **The directive.** `cmd/tpatch/main.go` (`cmd/tpatch/main.go:1`, currently a
  bare `package main` with no directives) gains
  `//go:debug winsymlink=1` immediately above `package main`. A `//go:debug`
  directive is only honored in the **main** package, which is why it lives
  there and not in `internal/intent`. The module already declares
  `go 1.26.1` (`go.mod:3`), so `1` is already the default for this module —
  the directive makes it **explicit and reviewable**, so that a future
  `go.mod` language downgrade, a vendored build, or a copy of this main package
  into another module cannot silently move the mapping to the pre-1.23 shape.
- **The test.** AVP-198 asserts the directive is present in `cmd/tpatch`'s
  `package main` file with exactly that text, and — the behavioral half, native
  Windows only — that a junction created by the fixture reports
  `ModeIrregular` and **not** `ModeSymlink`, which is the observable that
  distinguishes `winsymlink=1` from `winsymlink=0` at runtime.
- **Honest limit.** A `//go:debug` directive sets the *default*; the `GODEBUG`
  **environment variable** at process start still overrides it. The PRD does
  not claim otherwise. What the directive buys is that the default is pinned
  and reviewed; what AVP-198's behavioral half buys is that a change to the
  effective value is detected by CI rather than in the field.

**Windows has no `O_NONBLOCK`, and needs none for the *stable* kinds.** The
FIFO/character-device class is closed by kind, not by read semantics:
`statHandle` calls `GetFileType` on the handle and reports `FILE_TYPE_PIPE` as
`os.ModeNamedPipe` and `FILE_TYPE_CHAR` as `os.ModeDevice|os.ModeCharDevice`
(`$GOROOT/src/os/stat_windows.go` `statHandle`;
`$GOROOT/src/os/types_windows.go` `(*fileStat).mode`). Both fail
`Mode().IsRegular()` at ladder row 7 (pre-open, from `Root.Lstat`) and again
at row 14 (post-open, from `File.Stat`). AVP-176.

**This is a kind guarantee, not a timing guarantee.** A named pipe substituted
between the pre-open `Root.Lstat` and the open is caught by row 14 *after* the
open returns; Windows offers this design no pre-open non-blocking bit, and a
regular file on a wedged SMB share blocks on Windows exactly as it does on
Unix. §7.4.2's withdrawal applies to both targets equally.

§7.2's reserved-device-name refusal removes the remaining way a device could
be named through a slug at all, and `os.Root` independently refuses Windows
reserved device names (`Root` doc comment).

**The rescap Windows stub is precedent for the problem, not a reusable
implementation.** `internal/rescap/pathopen_windows.go:12-20` is a compile-only
stub: it falls back to a bare `os.OpenFile` with no hardening and its own
comment says resource capture "is explicitly unsupported there […] the lock
layer refuses before any gated path is ever opened", and `isSymlinkLoopError`
"always reports false on this target". rev-1 cited it as the seam to reuse;
that was wrong in kind. rev-2 cites it only as evidence that the *previous*
design could not be made cross-platform without `os.Root`, and AVP-180 asserts
that no `internal/intent` file calls `rescap.openNoFollow` or reproduces its
fallback. C60 is corrected accordingly. **ADR-034 D11 locks that this PRD does
not migrate `rescap.GatePath` and does not replace the shipped pathname gate.**

#### 7.4.4 The capture sequence, the pre/post walks, and the exact race behavior

The scope statements recorded here are locked by ADR-034 D8.

For every artifact, in order:

1. **Pre-capture component walk.** `Lstat` each non-leaf component below the
   feature directory (§7.3 step 5); refuse an **observed** symlink/reparse
   component.
2. `pre := ops.Lstat(rel)` — the leaf. This is the **identity of record**: it
   is captured from a handle-relative lookup, not from a pathname `os.Lstat`.
3. Kind and size decisions from `pre` (§7.5 rows 6–8). An oversize leaf is
   never opened.
4. `f, err := ops.OpenFile(rel, os.O_RDONLY|openFlags(), 0)`.
5. `post, err := f.Stat()` — an `fstat`/`GetFileInformationByHandle` on the
   descriptor we hold, not a second pathname lookup.
6. **Identity comparison before any byte is read**: `ops.SameFile(pre, post)`,
   whose one production implementation is `os.SameFile` (§7.1.1).
7. **Kind recheck on the descriptor**: `post.Mode().IsRegular()`.
8. **Size cross-check**: `post.Size() == pre.Size()`.
9. Bounded read into the shared scratch buffer (§7.4.5).
10. Post-read `f.Stat()` and size re-check.
11. **Post-capture component walk.** Re-`Lstat` the same non-leaf components as
    step 1, in the same order. **Any component now observed as a symlink or
    reparse point makes the result unstable** — see below.
11a. **Close the descriptor**, exactly once, and evaluate the outcome
    (`FileOps.Close`, ladder row 20c). The close is deliberately the **last**
    filesystem operation of the capture, after the post-capture walk: holding
    the descriptor across step 11 keeps the object pinned so it cannot be
    unlinked and its identity reclaimed while the ancestors are being
    re-observed, which is exactly the window step 11 exists to inspect. A
    close error means the capture did not complete cleanly, so it is a
    capture-level failure (`unreadable`) and **not** a content fact — it is
    evaluated **before** step 12.
12. Content classification from the captured bytes only.

**Step 11a is new in rev-4, and it exists because rev-3 declared `Close` on
`FileOps` and then never used it. The decision recorded here is locked by
ADR-034 D15.** A descriptor that is opened must be closed
on every path — including every `unstable`, `unreadable` and `oversize`-after-
open path — or a long-lived harness invoking the command in a loop leaks
descriptors until it hits `EMFILE`. rev-4 makes three things explicit:

- **Exactly one `Close` per successful `OpenFile`, on every path.** The close is
  unconditional (a `defer`-shaped or explicit single call, never a conditional
  one), including on the ladder rows that abandon the capture at step 5, 6, 7,
  8, 9 or 10. AVP-205 counts opens and closes across every ladder row and every
  abort and asserts equality with zero residue.
- **The close *outcome* is only consulted when the capture got that far.** A
  capture that ended at row 12 (`f.Stat()` failed) is already `unreadable`;
  first-match-wins means row 20c cannot demote or promote it. Row 20c decides
  only for a capture that reached step 11a with no earlier match.
- **No new state, code, message or advisory.** A close failure is
  `unreadable` / `artifact-unreadable` for an artifact and `status-unreadable`
  for `status.json` (§9.4.2 row 16a). §10.3 stays at ten reason codes, §9.4.4
  stays at thirteen abort codes, §10.4 stays at ten advisory codes. rev-4
  deliberately does **not** mint a fourteenth abort code: the operator-facing
  truth ("this file could not be read and closed cleanly; run `tpatch doctor`")
  is identical, and a code whose only distinguishing feature is which syscall
  failed would fragment a catalog §9.4.4 keeps closed for a reason.

**Steps 1 and 11 are the pre/post walk pair, and step 11 is new in rev-3.**
rev-2 walked the ancestors once, before the open, and then relied entirely on
the leaf identity comparison. That leaves a real gap the reviewer named: the
identity comparison proves the *leaf* descriptor is the object the leaf
`Lstat` observed, and proves nothing about whether the ancestor chain was the
same chain at both moments. The post-walk closes the reportable half of that
gap:

- If the post-walk observes an ancestor that is now a symlink or reparse
  point, the tree moved under the capture. The result is **not** trusted: the
  artifact is reported `unstable` (ladder row 20a), the captured bytes are
  discarded, and — because instability of a *required* artifact forces
  `indeterminate` (§9.1) — the run exits 3 rather than reporting a content
  state derived from an ancestor chain that changed. AVP-195.
- If the post-walk observes an ancestor that vanished, or that is now a
  non-directory, the same rule applies: `unstable`, not `absent` and not
  `unreadable`. A component that existed at step 1 and does not exist at step
  11 is a change, not an absence.
- The post-walk is a *detector*, not a *guarantee*. It narrows the window; it
  does not eliminate it. §8.3 limit 4 states the residual precisely.

**Why the identity comparison is sound on both platform classes.** On Unix,
`Root.Lstat` yields a `fileStat` populated from `fstatat` and `File.Stat`
yields one populated from `fstat`; `os.SameFile` compares `Dev` and `Ino`
(`$GOROOT/src/os/types_unix.go` `sameFile`). On Windows, `Root.Lstat` yields a
`fileStat` populated by `statHandle` →
`newFileStatFromGetFileInformationByHandle` from an
`O_FILE_FLAG_OPEN_REPARSE_POINT` handle, and `File.Stat` is literally
`statHandle(file.name, file.pfd.Sysfd)` on the held handle
(`$GOROOT/src/os/stat_windows.go`). Both set `vol`/`idxhi`/`idxlo` from
`GetFileInformationByHandle` **and deliberately leave the struct's `path`
field empty so that `os.SameFile` will not re-fetch them by pathname**
(`$GOROOT/src/os/types_windows.go`, the comment on
`newFileStatFromGetFileInformationByHandle`; `sameFile` → `loadFileId`, which
returns immediately when `path == ""`). So on Windows the comparison is
volume-serial + file-index, both handle-derived, with **no pathname reopen**.
rev-1's claim that `os.SameFile` on Windows would re-open by pathname is
removed, and no row depends on it.

**The comparison is reached through the seam, and rev-4 says why that matters.**
Step 6 calls `ops.SameFile(pre, post)`, not `os.SameFile(pre, post)`. The
production adapter's body is exactly `return os.SameFile(a, b)`, so production
behavior is identical — but `os.SameFile` is only meaningful over the
**unexported** `*os.fileStat` values `os` produces (it type-asserts and
compares `Dev`/`Ino` or `vol`/`idxhi`/`idxlo`), and no test outside `os` can
construct one. With a package-level call, every row that needs a *chosen*
identity verdict — AVP-084 (different inode between `Lstat` and `fstat`),
AVP-151 (same-identity alias), AVP-160 (status identity), AVP-196 (b)
(file-ID reuse) — would be unwritable except against a real hostile
filesystem, which is the same defect rev-3 fixed for `Lstat`/`OpenFile` and
left in place here. AVP-206 pins both halves: the injectable verdict, and the
AST assertion that `os.SameFile` appears at exactly one production call site.

**This Windows soundness argument is a version-pinned implementation
dependency, not a public guarantee — rev-3 labels it as one.** Nothing in
`os`'s documentation promises that a handle-derived `FileInfo` carries a
comparable identity, that `Stat` on Windows is handle-based, or that
`SameFile` will not fall back to a pathname. Every one of those is read from
unexported code in the pinned toolchain (G7, G8, G16, all classified
**tripwire** in §23.2). Two consequences:

- **AVP-176 is a Go-version tripwire**, not an eternal contract test. It runs
  natively on Windows and asserts the observable — `os.SameFile` over
  `Root.Lstat` and `File.Stat` is true for an unchanged file and false after an
  injected replacement — so a toolchain change that breaks the assumption is
  caught at upgrade time by a red test rather than by a silent weakening of the
  identity check in the field.
- **The PRD does not promise this property to consumers.** No shipped string,
  help text, doc or skill surface states that the command detects object
  substitution on Windows as a guarantee. What it states is the outcome:
  `unstable`, with a re-run remediation. AVP-189, AVP-152.

**Ancestor-symlink races, stated precisely.** Suppose an attacker replaces
`.tpatch/features/<slug>` (or `artifacts/`, or the leaf itself) with a symlink
after step 1/2 and before step 4.

| Raced link points at | What `os.Root` does | What the inspector reports | Bytes read |
|---|---|---|---|
| a name **outside** `repoRoot`, or an absolute target | resolution refused; `OpenFile` returns an error | `unreadable` (ladder row 11) | none |
| a name **inside** `repoRoot` resolving to an object with a **different** identity | followed, in-root; the open succeeds on the other object | `unstable` (ladder row 13 — `ops.SameFile` reports false) | **none** — step 6 precedes step 9 |
| a name **inside** `repoRoot` resolving to an object with the **same** identity (a link to the very file that was `Lstat`ed, or a hard link to it) | followed, in-root | the artifact's real state, computed from that object | that object's bytes |
| any of the above at an **ancestor**, still in place at step 11 | as above for the open; step 11 additionally observes the changed component | `unstable` (row 20a) even when the leaf identity matched | discarded |

**The guarantee, weakened to what is actually true.** rev-2 said "a
*different* object is never read". rev-3 says:

> An object **observed as different** is never read.

The distinction is the whole finding. "Different" is a property of the world;
"observed as different" is a property of two `FileInfo` values this process
compared. Five cases where those diverge, all stated here, none of them
claimed away, and all five repeated in §8.3:

1. **Same-identity alias.** A raced link that resolves, in-root, to the very
   object `Lstat` observed. `ops.SameFile` matches and the capture proceeds. The
   report is still true of the object that was read — but note what the report
   *says*: it labels those bytes with the **canonical artifact path**. So a
   consistent in-root alias is **attributed to the canonical name**. The
   command's claim is "the object I classified is the object I read", not "the
   canonical name still designated this object at every instant". rev-4 states
   that attribution explicitly rather than leaving a reader to infer the
   stronger name→object binding. AVP-151.
2. **Hard-link alias.** A hard link is not an alias *of* an inode, it **is**
   the inode: `pre` and `post` carry the same `Dev`/`Ino` (Unix) or the same
   volume serial and file index (Windows) because they *are* the same file.
   Swapping the name to a hard link of the same file is undetectable **by
   construction**, not by omission, and no additional check could detect it.
   AVP-196 case (a).
3. **Inode / file-ID reuse.** If the object is unlinked and a new object is
   created that the filesystem assigns the same `Ino` (Unix) or the same
   `FileIndex` (Windows — NTFS file IDs are reusable after deletion), the
   comparison matches two genuinely different objects. This is a known,
   unavoidable property of identity-by-number, and the PRD asserts nothing
   stronger. AVP-196 case (b).
4. **Swap-and-restore between probes.** An attacker who replaces the object
   between two probes and restores it before the next probe defeats every
   probe individually. The design has finitely many observation points
   (steps 1, 2, 5, 9, 10, 11); an adversary with arbitrary timing control can
   arrange for each one to see the intended object. AVP-196 case (c).
5. **The walk→`Lstat` window is real and is not closed.** Between step 1's
   observation of an ancestor and step 2's `Lstat` of the leaf, and again
   between step 2 and step 4's open, the tree can change. The design's answer
   is *detect and report*, not *prevent*: `os.Root` bounds where a changed name
   can resolve **to**, the identity comparison catches the substitutions it can
   observe, and steps 11/20a catch a persisting ancestor change. What remains
   is a genuine TOCTOU residue, and rev-3 names it rather than implying it is
   closed. **The residue's exact shape, restated in rev-4 because it is the one
   most easily read as stronger than it is:** an object *observed as different*
   is never read, but an object the process never observed to be different —
   most importantly a consistent in-root alias or hard link that satisfies every
   probe — **is** read, and its bytes are then attributed to the canonical
   artifact name in the report. AVP-195, AVP-196.

What the design **does** guarantee, exactly and no more:

- **Logical confinement**: the raced resolution can never leave the repository
  root *as a name*. It can still reach a mount point, a bind mount, a `/proc`
  file or a device node that is nameable from inside the root (§7.4.2).
- **No observed substitution**: an object the inspector observed to be
  different from the one it classified is never read, because identity is
  compared before the first byte and a mismatch ends the capture. What it reads
  is attributed to the canonical name, with the alias limits above.
- **Bounded allocation and bounded bytes requested**: exactly one data buffer
  is allocated per invocation, and at most `MaxArtifactBytes+1` bytes are
  requested from any one descriptor (§7.4.5).
- **A non-wedging open on Unix**: a FIFO or blocking character device at the
  leaf cannot wedge the `open(2)` (§7.4.3).

**Explicitly not guaranteed: bounded runtime.** **Withdrawn rev-3 wording:**
"bounded cost […] with no unbounded wait". An ordinary read of an ordinary
regular file can block indefinitely on
a stalled filesystem; v1 has no deadline and no cancellation (§7.4.2, §8.3,
Q11, AVP-207).

**What is deliberately not claimed.** rev-1 asserted a "final no-follow"
guarantee on both platforms. rev-2 dropped that and asserted a
"no substitution" guarantee that was still too strong. rev-3 asserted three
properties, one of which ("no unbounded wait") rev-4 withdraws. The design
asserts only the four properties above. Any output string, help text, doc or
skill surface that claims the command detects a same-identity alias, a
hard-link alias, an ID-reuse substitution or a swap-and-restore, or that it
refuses every raced link, or that bytes it read are physically inside the
repository, or that it cannot hang / always terminates / has bounded runtime,
is a defect (AVP-152, AVP-189, AVP-207).

#### 7.4.5 The bounded read — one reused scratch buffer per invocation

The decision recorded here is locked by ADR-034 D9.

`MaxArtifactBytes = 4 MiB` (4,194,304). `MaxStatusBytes = 1 MiB` (1,048,576),
§9.4.2.

**Why a fixed reusable buffer, stated correctly (rev-4 correction; ADR-034 D9).** rev-1
specified `io.ReadAll(io.LimitReader(f, MaxArtifactBytes+1))` and claimed it
"allocates at most `MaxArtifactBytes+1` bytes". That precise claim was false,
and rev-2/rev-3 over-corrected it into "the *allocation* is not bounded", which
is also false. The accurate statement is:

> `io.ReadAll(io.LimitReader(f, Max+1))` **is bounded** — the limit reader caps
> the result at `Max+1` bytes, so the total allocation is `O(Max)` no matter
> what the file does. What it is not is **fixed**: `io.ReadAll` starts from a
> small slice and grows it by `append`, so a single capture performs a
> *sequence* of allocations of increasing size and copies the accumulated bytes
> between them. The peak live footprint and the number of allocations depend on
> the file's actual length, and the whole sequence is paid again on each of the
> five sequential captures.

So the choice is not bounded-vs-unbounded; it is **variable-and-repeated** vs
**fixed-and-shared**. This design takes the second:

- one allocation for the whole invocation instead of a growth sequence per
  capture;
- a flat, predictable footprint that does not vary with artifact size, so the
  worst case is the same as the common case;
- no copy-on-grow inside a capture, which removes the one place where a file
  that grows during the read could still influence allocation behavior.

No acceptance row and no sentence anywhere may claim `io.ReadAll` over a
`LimitReader` is unbounded (G20, G21; AVP-172's sensitivity fixture tests the
*mechanism*, not a boundedness defect).

**rev-3's per-invocation reuse is retained.** Five captures per run
(`status.json` plus four artifacts) at one 4 MiB+1 allocation each is a worst
case of ~20 MiB of allocation for a command whose entire job is to answer a
five-line question. rev-2 stated the ceiling per capture without ever totalling
it. The mechanism:

```go
const (
    MaxArtifactBytes = 4 << 20 // 4 MiB
    MaxStatusBytes   = 1 << 20 // 1 MiB
)

// internal/cli, once per invocation of the command, before Inspect:
scratch := make([]byte, MaxArtifactBytes+1)   // the only data buffer
report := intent.Inspect(ops, slug, scratch)
```

- **Exactly one data buffer is allocated per invocation of the command**, of
  exactly `MaxArtifactBytes+1` bytes, before any capture begins.
- **It is reused sequentially.** Captures are strictly sequential (§8.4: four
  independent, ordered captures, no concurrency anywhere in this command), so
  reuse is safe without a lock, a pool or a copy. Each capture fills the buffer
  from index 0; each classification consumes `scratch[:n]` **before** the next
  capture starts; nothing downstream retains a reference to the slice.
- **The status capture gets a smaller *slice of the same array*, not its own
  array**: `statusBuf := scratch[:MaxStatusBytes+1]`. `MaxStatusBytes <
  MaxArtifactBytes` is therefore a **compile-time-checkable invariant** of the
  design, not an accident, and the implementation asserts it
  (`const _ = MaxArtifactBytes - MaxStatusBytes` fails to compile if the
  relation inverts). The two caps stay distinct constants and neither is
  defined in terms of the other (Q3/Q7), but the reuse imposes the ordering.
- **Honest cost statement.** The command's fixed data-buffer cost is
  `MaxArtifactBytes + 1` bytes = **4,194,305 bytes, allocated once**, for every
  invocation — including a run that aborts before any capture, and including a
  run against a feature whose artifacts are all absent or all 12 bytes long.
  **`make([]byte, n)` yields a zeroed slice** (G22), so the invocation also
  pays one ~4 MiB zeroing, once, before the first capture — not once per
  capture, and never again. This PRD does **not** claim the allocation scales
  with the data. It is a deliberate trade: a fixed, predictable ~4 MiB and one
  zeroing pass against five variable allocation sequences with copy-on-grow.
  Q9 records the alternative (allocate lazily on the first capture that
  actually opens something) as revisable; note that lazy allocation removes the
  cost only for runs that open nothing, since the zeroing is per-invocation
  either way. AVP-197.

The read itself is unchanged in shape:

```go
n, err := io.ReadFull(f, buf) // buf is scratch, or scratch[:MaxStatusBytes+1]
```

`io.ReadFull` is used as a bounded fill, not as a "must be exactly this long"
assertion, and its outcomes are exactly the cases the ladder needs:

| `io.ReadFull` result | Meaning | Ladder |
|---|---|---|
| `err == io.EOF` (so `n == 0`) | the file was empty | captured bytes are `buf[:0]`; classification continues at row 21 |
| `err == io.ErrUnexpectedEOF` (`0 < n < len(buf)`) | the whole file fit inside the cap | captured bytes are `buf[:n]`; classification continues at row 18 |
| `err == nil` (`n == len(buf)`, i.e. the cap `+1`) | the file is **larger** than the cap. Row 8 already excluded a stably-oversize leaf, so this can only mean it grew inside the capture window | row 17 → `unstable` |
| any other `err` | a real read failure | row 16 → `unreadable` |

Properties this buys, each separately asserted:

- **Exactly one data-buffer allocation per invocation**, of exactly
  `MaxArtifactBytes+1` bytes, before the first read begins. There is no
  capacity growth, no reallocation and no copy, whatever any file does during
  any read (AVP-197).
- **A hard read ceiling per capture.** At most `MaxArtifactBytes+1` bytes are
  ever requested from any artifact descriptor, and at most `MaxStatusBytes+1`
  from the status descriptor (AVP-171, AVP-174).
- **`os.ReadFile`, `io.ReadAll`, `io.LimitReader` and `bufio.Scanner` are
  forbidden** in the inspector, and the source guard fails if any of them
  reappears, if either `+1` is dropped, or if a growable slice replaces the
  fixed buffer (AVP-116, AVP-172).
- **Cap changes are coupled to frozen message text.** `MaxArtifactBytes`'s
  value appears verbatim as `4 MiB` in the `analysis-sidecar-oversize` advisory
  (§10.4) and in §15's remediation, and `MaxStatusBytes`'s appears verbatim as
  `1 MiB` in the `status-oversize` abort message (§9.4.5). Changing either
  constant without changing its message would ship a diagnostic that states a
  limit the binary does not enforce. AVP-201 is a mechanical guard: it derives
  the human-readable unit string from each constant and asserts it appears in
  exactly the messages listed, with a sensitivity fixture that changes the
  constant alone and fails. **The coupling mechanism is unchanged in rev-4 and
  is stated here exactly**: the guard *derives* the string from the constant
  (it does not compare two hard-coded literals), it asserts appearance in the
  enumerated messages **and** non-appearance of any other limit figure, and it
  fails in **both** directions — constant changed without message, and message
  changed without constant.
- The captured slice is `buf[:n]`; nothing downstream ever reads past `n`, and
  no reference to the buffer survives the invocation.

An equivalent hand-rolled bounded loop over the same fixed buffer is
acceptable — the requirement is the single fixed allocation and the byte count,
not the specific stdlib helper. `internal/rescap/content.go:50-70` is the
shipped precedent for the cap-plus-one discipline, and
`internal/rescap/content.go:9-11` states the reason: a pre-read
`Stat().Size()` check alone is bypassed by a file that grows. rev-2 kept that
reasoning and corrected the *exact-ceiling* claim rev-1 attached to it; rev-3
fixed the per-capture multiplication; rev-4 corrects the over-correction, so
the record now reads: `ReadAll(LimitReader(...))` is bounded but variable and
repeated, and the fixed reusable buffer is chosen to cap and flatten that cost.

### 7.5 Per-artifact classification ladder (first match wins, total)

Every row is reachable, every reachable condition is a row, and the order is
normative. Every `Lstat` below is `RootOps.Lstat` and every open is
`RootOps.OpenFile` (§7.3 step 6); `fstat` means `FileOps.Stat` on the held
descriptor.

| # | Condition | Resulting state |
|---|---|---|
| 1 | A non-leaf component below the feature directory is a symlink or reparse point (`Mode()&(ModeSymlink\|ModeIrregular) != 0`) | `symlink-refused` |
| 2 | A non-leaf component `Root.Lstat` fails with `ErrNotExist` | `absent` |
| 3 | A non-leaf component `Root.Lstat` fails otherwise, or exists and is not a directory | `unreadable` |
| 4 | Leaf `Root.Lstat` fails with `ErrNotExist` | `absent` |
| 5 | Leaf `Root.Lstat` fails otherwise | `unreadable` |
| 6 | Leaf `Root.Lstat` mode is a symlink or reparse point | `symlink-refused` |
| 7 | Leaf `Root.Lstat` mode is not regular (dir, FIFO, socket, device, Windows `FILE_TYPE_PIPE`/`FILE_TYPE_CHAR`) | `not-regular` |
| 8 | Leaf `Root.Lstat` size > `MaxArtifactBytes` | `oversize` (no open, no read) |
| 9 | `Root.OpenFile` fails with `ErrNotExist` (it existed at row 4) | `unstable` |
| 10 | `Root.OpenFile` fails because resolution would leave the root — a raced link pointing outside `repoRoot`, refused by `os.Root` (§7.4.4) | `unreadable` |
| 11 | `Root.OpenFile` fails for any other reason | `unreadable` |
| 12 | `f.Stat()` fails | `unreadable` |
| 13 | `!ops.SameFile(pre, post)` — the descriptor is not the object `Root.Lstat` observed (§7.1.1; production body is `os.SameFile`) | `unstable` |
| 14 | `!post.Mode().IsRegular()` — the descriptor is not a regular file although row 7 passed | `unstable` |
| 15 | `post.Size() != pre.Size()` | `unstable` |
| 16 | the bounded read returned an error other than `io.EOF` / `io.ErrUnexpectedEOF` | `unreadable` |
| 17 | the bounded read filled the whole `MaxArtifactBytes+1` buffer (`err == nil`) — the file grew past the cap during the read | `unstable` |
| 18 | `int64(n) != post.Size()` | `unstable` |
| 19 | post-read `f.Stat()` fails | `unreadable` |
| 20 | post-read size differs from `post.Size()` | `unstable` |
| 20a | the **post-capture component walk** (§7.4.4 step 11) observes a non-leaf component that is now a symlink or reparse point, that has vanished, or that is now a non-directory | `unstable` |
| 20b | the post-capture component walk fails for any other reason | `unreadable` |
| 20c | `f.Close()` (§7.4.4 step 11a) returns an error | `unreadable` |
| 21 | `strings.TrimSpace(buf[:n])` is empty | `present-empty` |
| 22 | `analysis_sidecar` and the captured bytes are not valid JSON | `invalid-structured` (`sidecar-not-json`) |
| 23 | `analysis_sidecar` and the captured bytes are valid JSON but not an object | `invalid-structured` (`sidecar-not-json-object`) |
| 24 | otherwise | `present-nonempty` |

Rows 20a, 20b and 20c are lettered rather than renumbered so that every row
number cited in rev-2's prose, acceptance rows and reviews still resolves to
the same condition. The ladder is **27 rows** (1–20, 20a, 20b, 20c, 21–24).

Row 10 is stated separately from row 11 for honesty, not for behavior: both
yield `unreadable`, because `os.Root` reports an escape as an ordinary
`*os.PathError` wrapping an unexported sentinel
(`$GOROOT/src/os/file.go`, `errPathEscapes`) with no exported way to
discriminate it. The row exists so the ladder documents where a raced,
outward-pointing link actually lands rather than leaving it implicit, and
AVP-149 asserts that landing. No output string distinguishes rows 10 and 11,
and none names the link target.

Seven orderings are load-bearing and must not be reordered:

- **`oversize` (8) precedes every open (9+).** A file whose *stable* observed
  size already exceeds the cap is never opened and never read. This is the only
  way `oversize` can be produced.
- **`unstable` growth (17) follows the open and therefore outranks any second
  `oversize` claim.** A file that was within the cap at row 8 and grew past it
  during the read is `unstable`, never `oversize`: `oversize` is a claim about
  a file whose size was *observed to be* over the cap, and a file changing
  underneath the reader was not stably observed at all. Rows 8 and 17 are
  mutually exclusive by construction — 8 is decided before the descriptor
  exists, 17 only after — so the pair is first-match explicit with no overlap.
  Guarded by AVP-112 and AVP-140.
- **`unstable` (9, 13, 14, 15, 17, 18, 20, 20a) precedes every content-derived
  row (21–24).** A file observed mid-truncation looks empty or looks like
  invalid JSON. Classifying it `present-empty` would be exactly the false
  statement this PRD exists to prevent. Guarded by AVP-094.
- **The post-capture walk (20a, 20b) precedes every content-derived row
  (21–24).** This is what makes step 11 of §7.4.4 mean anything: a capture
  whose ancestor chain changed is never reported with a content state, even
  when the bytes were read successfully and the leaf identity matched.
  Guarded by AVP-195.
- **The close (20c) precedes every content-derived row (21–24) and follows
  every descriptor-scoped row (12–20).** The descriptor is closed after the
  last operation that needs it — including the post-capture walk, which is
  deliberately performed while the object is still pinned (§7.4.4 step 11a) —
  and a close failure is a capture-level failure, not a content fact. A capture
  that already matched an earlier row keeps that row: first-match-wins means
  20c never overwrites an `unstable` or `unreadable` verdict decided upstream,
  and never suppresses one. Guarded by AVP-203 and AVP-205.
- **`symlink-refused` (1, 6) precedes `not-regular` (7 for the leaf's own kind)
  and every read row.** An *observed* symlink or reparse point is refused on
  kind alone, before the open; nothing downstream ever touches it. A link that
  appears only after row 6 is not observed and is handled by row 13's identity
  comparison instead (§7.4.4) — the ladder never pretends to have seen it.
- **Emptiness (21) precedes the structured rows (22, 23).** A whitespace-only
  `analysis.json` is not valid JSON, so classifying JSON first would report it
  as `invalid-structured` and hide the far more useful fact that the file is
  simply empty. `present-empty` is the honest answer for all four artifacts
  alike. Pinned by AVP-030.

`json.Valid` (row 22) is the same primitive the existing implement gate uses
(`internal/store/manual.go:75`); "must be an object" (row 23) is the same shape
constraint as the analyze-phase `JSONObjectValidator`
(`internal/workflow/retry.go:145-157`, wired at
`internal/workflow/workflow.go:54,62`). v1 deliberately validates **shape
only** — never field names, never the `AnalysisResult` field set — so a future
sidecar field addition cannot retroactively turn an existing feature red.

Rows 1–24 (including 20a, 20b and 20c) apply identically to
`analysis_sidecar`; rows 22–23 apply *only* to it. There is no
artifact-specific short-cut anywhere in the ladder, so every instability probe
has a sidecar case (AVP-117), and so does the close failure (AVP-203).

### 7.6 The closed state enum

The vocabulary extends the presence vocabulary already shipped in
`internal/workflow/verify_landed.go:122-125` (`absent`, `present-empty`,
`present-nonempty`) rather than inventing a parallel one.

| State | Meaning | Satisfies a required artifact? |
|---|---|---|
| `present-nonempty` | Regular file; the captured bytes contain at least one non-whitespace byte, and — for `analysis_sidecar` only — additionally parse as a JSON **object**. | **yes** |
| `present-empty` | Regular file; captured bytes are zero-length or whitespace-only. | no |
| `absent` | A path component or the leaf does not exist (`os.ErrNotExist` from `Lstat`). | no |
| `symlink-refused` | Any component at or below the feature directory, including the leaf, was **observed** to be a symbolic link or a Windows reparse point that maps to `ModeSymlink` or `ModeIrregular` (junction / mount point / any other non-`AF_UNIX`, non-`DEDUP` tag, §7.4.3) by `Root.Lstat`. Never followed, never read, target never named. | no |
| `not-regular` | The leaf exists at `Root.Lstat` time, is not a refused symlink/reparse point, and is not a regular file (directory, socket — including a Windows `AF_UNIX` reparse point — FIFO, device). | no |
| `unreadable` | An inspector operation — `Lstat`, `OpenFile`, `FileOps.Stat`, the bounded read, the post-capture walk, or `FileOps.Close` (ladder row 20c) — failed for a reason other than absence (EACCES, EIO, ENOTDIR, ENAMETOOLONG, EBADF, or `os.Root` refusing a resolution whose name would leave the repository root). | no |
| `oversize` | The leaf's `Root.Lstat` size exceeds `MaxArtifactBytes`; the file is deliberately never opened and never read. | no |
| `invalid-structured` | `analysis_sidecar` only: bytes captured, but they are not valid JSON, or are valid JSON that is not an object. | no (and it is optional, so it never affects readiness) |
| `unstable` | The artifact changed identity, kind or size across its own capture window, or its ancestor chain was observed to have changed by the post-capture walk (§7.4.4 step 11, §8.3). The captured bytes, if any, are not trusted and are not classified further. | no — and for a required artifact it forces `indeterminate` (§9.1) |

The enum is **closed and total**: §7.5 maps every reachable condition into
exactly one row. AVP-093 mechanically asserts the implementation's exported
state constants equal this table.

`invalid-structured` applies only to `analysis_sidecar`. The three Markdown
artifacts are never parsed, never linted, and never inspected for headings —
that would be semantic quality (§7.7).

### 7.7 What the enum does *not* encode

No state means "thin", "stub", "placeholder", "low quality", "TODO-only" or
"heuristic". A `spec.md` whose entire content is `TODO: write me` is
`present-nonempty`, and the report says so without editorial comment. If a
future PRD wants a thinness signal it must define it, justify it, and enumerate
its behavior delta; it may not smuggle it in as a new enum value.

### 7.8 The non-certification statement

Every emission carries a fixed, verbatim disclaimer:

```text
Structural presence only. This report does not certify semantic quality.
```

Human output prints it as its last line. JSON carries it as the
`disclaimer` field. The string is frozen; AVP-046 asserts it byte-for-byte in
both surfaces.

## 8. Snapshot and race semantics

### 8.1 Why this section exists at all

`store.writeFile` is `os.WriteFile` (`internal/store/store.go:918-923`), which
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
once through one descriptor, and every classification, count and message
derives from that capture. There is no second read anywhere, including in the
JSON renderer.

### 8.3 Instability detection and its honest limits

Instability is detected per artifact by the eight probes in §7.5 rows 9, 13,
14, 15, 17, 18, 20 and 20a:

| Probe | Row | Detects |
|---|---|---|
| open-time disappearance | 9 | the leaf vanished between the leaf `Lstat` and the `OpenFile` |
| descriptor identity | 13 | the name resolved, in-root, to an object **observed as different** from the one the leaf `Lstat` reported |
| descriptor kind | 14 | the thing opened is not the regular file the leaf `Lstat` saw |
| pre-read size cross-check | 15 | size changed between the leaf `Lstat` and the descriptor `fstat` |
| growth past the cap | 17 | the file grew beyond `MaxArtifactBytes` during the read |
| byte-count disagreement | 18 | the read returned a different length than the descriptor `fstat` reported |
| post-read size change | 20 | the file changed after the read but inside the window |
| post-capture ancestor walk | 20a | a non-leaf component is now a symlink/reparse point, is now a non-directory, or has vanished (§7.4.4 step 11) |

`snapshot-unstable` is an existing, shipped tpatch concept, not a new one:
verify already carries a `snapshot-unstable` failure vocabulary
(`internal/workflow/verify_landed.go:83,93`), and the reason code
`artifact-snapshot-unstable` keeps that word deliberately.

**Scope claim, stated precisely.** These probes detect the enumerated
conditions and nothing more. The command's identity promise is *"an object
**observed as different** is never read"* (§7.4.4), and the gap between
"different" and "observed as different" is enumerated here in full. Six limits,
all of them stated here and in §7.4.4, and none of them contradicted by any
output string, help text, doc or skill surface (AVP-152, AVP-189):

1. **Same-length in-place rewrite is undetectable.** A writer that truncates
   and fully rewrites the same inode to the same length between the pre-read
   `fstat` and the post-read `fstat` is not detectable by size, kind or
   identity.
2. **A same-identity alias is undetectable, and is deliberately not claimed.**
   If a raced link resolves — inside the root — to the very object the leaf
   `Lstat` observed, `ops.SameFile` matches and the capture proceeds. The
   report is still true of the object that was read (§7.4.4), and those bytes
   are **attributed to the canonical artifact path** in the report. The command
   does not claim to have refused the alias, and does not claim the canonical
   name designated that object at every instant. AVP-151.
3. **A hard-link alias is undetectable by construction.** A hard link *is* the
   same inode: `pre` and `post` carry identical `Dev`/`Ino` (Unix) or identical
   volume serial and file index (Windows) because they describe one object with
   two names. No additional check inside this design could distinguish the two
   names, and none is claimed. AVP-196 (a).
4. **Inode / file-ID reuse defeats identity-by-number.** If the object is
   unlinked and the filesystem assigns the same inode number (Unix) or the same
   NTFS file ID (Windows — file IDs are reusable after deletion) to a new
   object, `ops.SameFile` reports two genuinely different objects as the same.
   This is an inherent property of identity-by-number, not a defect in this
   design, and the PRD asserts nothing stronger than what `os.SameFile`
   observes. AVP-196 (b).
5. **Swap-and-restore between probes defeats every probe individually.** The
   capture has finitely many observation points (§7.4.4 steps 1, 2, 5, 9, 10,
   11). An adversary with arbitrary timing control can restore the intended
   object before each one. The post-capture ancestor walk (probe 20a) narrows
   the window; it does not close it, and the residual walk→`Lstat`→open TOCTOU
   window is a genuine, named limitation rather than an implied absence.
   AVP-195, AVP-196 (c).
6. **A second `fstat` on a held descriptor is a tautology for pathname
   questions.** It cannot detect a name swap; it can only detect a change to
   the object it already holds. `internal/rescap/pathgate.go:181-190` states
   this explicitly for the analogous case. Probes 19–20 are therefore scoped to
   "did this object change", not "does this name still point here" — the
   name question is answered, to the extent it can be, by row 13's identity
   comparison and row 20a's post-walk.

Overclaiming here would repeat the category error the PRD is correcting.

**No retry, no spin, no lock.** The inspector never re-reads to "resolve"
instability and never takes a lock. `unstable` is a reported outcome, and the
remediation is "re-run when no other tpatch process is writing this feature".
This keeps a read-only command free of any lock-acquisition side effect and
keeps the *number* of operations bounded: five captures, each with a fixed
number of `Lstat`s, one open, at most `Max+1` requested bytes and one close.

**It does not keep the command's runtime bounded, and rev-4 withdraws the claim
that it does (ADR-034 D16).** rev-3 wrote: "Combined with §7.4.3's non-blocking open on Unix
and the pre-open kind refusal on Windows, the command has no unbounded wait
anywhere, which is why it registers no `--timeout`." Both halves were wrong.
`O_NONBLOCK` bounds the FIFO/device **open**, not any read; the Windows kind
refusal handles the *stable* non-regular kinds, not a slow regular file; and
neither says anything about a regular file served by a stalled NFS/SMB mount,
a wedged FUSE server, a `/proc`-style provider or an unresponsive device
driver, on which an ordinary `read(2)` — or even an `Lstat` — can block
indefinitely. v1 registers no `--timeout` because it has no provider, network
or subprocess wait to bound and because it deliberately defines no cancellation
contract (§5.1) — **not** because a wait is impossible. Q11 records a deadline
or a cancellable `context.Context` as a revisable, additive later decision, and
AVP-207 fails the build if the withdrawn claim reappears in any shipped string,
this PRD or ADR-034.

### 8.4 No cross-artifact atomicity claim

The four captures are independent and **strictly sequential** — there is no
goroutine, no `errgroup` and no concurrency anywhere in this command, which is
also what makes §7.4.5's single reused scratch buffer safe without a lock. The
report **must not** claim the captures represent one instant. Concretely:

- there is no snapshot id, generation counter or `captured_at` field;
- `status.json` is captured once, before the artifact captures, through its own
  independent capture window (§9.4.2), and its echoed `feature_state` is
  documented as "read before the artifact captures, not simultaneously with
  them";
- the human footer says `readiness` — never "the feature is ready".

A cross-artifact atomic view would require a repository-wide read lock, which
is out of scope and would make a read-only command a lock acquirer. AVP-086
asserts no such field exists.

The single held `*os.Root` is **not** an atomicity mechanism and is never
described as one. It fixes the *directory name resolution base* the run uses —
including across a rename of `repoRoot` itself, since on the allowlisted
targets the root is a descriptor/handle, not a name (`Root` doc comment;
AVP-143) — but it says nothing about the contents of the files beneath it, and
nothing about whether those files live on the same filesystem (§7.4.2).

### 8.5 Determinism on a quiescent tree

On a tree no other process is writing, every probe is a no-op and two
consecutive invocations produce **byte-identical** stdout and stderr. This is
the determinism contract (AVP-050) and it holds because no output field derives
from wall-clock, mtime, size, ordering of directory reads, or map iteration.

## 9. Readiness, exit codes and error precedence

### 9.1 Readiness derivation

Over the **required** set only (`analysis`, `spec`, `exploration`):

| Condition (first match wins) | `structural_readiness` |
|---|---|
| the run aborted (§9.4.4) | `indeterminate` |
| any required artifact is `unstable` | `indeterminate` |
| any required artifact is not `present-nonempty` | `not_ready` |
| otherwise | `ready` |

The optional artifact never contributes — including when it is `unstable`,
`invalid-structured`, `oversize`, `symlink-refused` or `unreadable`. Those
produce advisories only (§10.4).

`ready` means exactly: *all three canonical Markdown artifacts exist as
regular, readable, non-symlink files under the bounded size, each was captured
without instability, and each contains at least one non-whitespace byte.* It
means nothing else — in particular it makes no claim about `analysis.json`,
about content quality, or about the feature's lifecycle state.

### 9.2 Exit codes

Per-command contract, per `SPEC.md:135-141` ("Exit codes are **per-command
contracts**, not a single global enum") and `internal/cli/exit_error.go:9-13`.
Non-1 codes are surfaced through `*ExitCodeError`
(`internal/cli/exit_error.go:12-33`) and mapped at
`internal/cli/cobra.go:43-52`.

| Code | Meaning | Report emitted? | stderr `error:` line? |
|---|---|---|---|
| `0` | `structural_readiness = ready` | yes | **no** |
| `1` | Generic CLI/usage error surfaced **before** `RunE` runs: an unknown flag (including `--manual`, `--regenerate`), a wrong argument count, or a flag supplied without its required value (`--path` with no argument). pflag/Cobra produces the error text; because the root sets `SilenceUsage`/`SilenceErrors` (`internal/cli/cobra.go:56-62`) cobra prints nothing itself, and the repository's own root printer emits that text unsanitized as one `error:` line (§14.3.1). | no | yes (one `error:` line carrying pflag/Cobra's text) |
| `2` | `structural_readiness = not_ready` | yes | yes (§9.5) |
| `3` | `structural_readiness = indeterminate`: an abort (§9.4.4) **or** a required artifact is `unstable` | yes | yes (§9.5) |
| `4` | Reserved-surface refusal: `prepare <slug>` without `--check` (§5.3) | no | yes (§9.5) |

**`--path` is exit 3's, not exit 1's.** rev-1 listed "malformed `--path`" under
exit 1. That was wrong. `--path` is a persistent **string** flag registered on
the root command (`internal/cli/cobra.go:66`); pflag accepts any string value
and performs no existence, kind or syntax validation, so a nonexistent,
unreadable or nonsensical directory parses cleanly and `RunE` runs. The failure
surfaces inside `RunE`, in workspace discovery: `openStoreFromCmd` reads the
flag and calls `store.FindProjectRoot(start)`
(`internal/cli/cobra.go:3782-3793`), which resolves the start directory with
`filepath.Abs` and then walks upward looking for a `.tpatch` directory,
returning `could not find .tpatch in this directory or any parent` when it
reaches the filesystem root (`internal/store/store.go:23-40`). **That is the
actual trigger**, and this command binds it to abort
`workspace-not-initialized`, exit 3 — with the abort report emitted, like every
other abort.

The only way to reach exit `1` through `--path` is a pflag parse error, i.e.
`tpatch prepare <slug> --check --path` with no following value. AVP-183 and
AVP-184 pin the two populations apart.

**This exit-3 binding deliberately diverges from every other tpatch command,
and rev-4 discloses the divergence rather than leaving a reader to discover
it. The decision recorded here is locked by ADR-034 D18.** Today every command that opens a workspace does so through
`openStoreFromCmd` (`internal/cli/cobra.go:3782-3793`), which returns
`FindProjectRoot`'s plain `errors.New("could not find .tpatch in this directory
or any parent")` (`internal/store/store.go:23-40`). That error is not an
`*ExitCodeError`, so `exitCodeFor` maps it to the generic `1`
(`internal/cli/cobra.go:43-52`) and the root printer emits it as one `error:`
line. Running `tpatch status` outside a workspace therefore exits **1**, while
running `tpatch prepare <slug> --check` outside a workspace exits **3** with a
full abort report. That difference is intentional, and it is legitimate for
three stated reasons:

1. **`SPEC.md:135-141` makes exit codes per-command contracts, not a global
   enum.** `verify` already binds a non-1 meaning to `2` that no other command
   shares. A per-command code for a per-command condition is the established
   shape, not an exception carved for this PRD.
2. **This command's exit code is a *verdict*, not an error class.** `0`/`2`/`3`
   are the three values of `structural_readiness`, and every one of them emits
   the same report schema. A workspace that cannot be found yields the same
   answer as a `status.json` that cannot be trusted — `indeterminate` — and
   collapsing it to the generic `1` would be the only path on which this
   command returned a nonzero code *without* a report, breaking the
   `artifacts` ⇔ `abort` invariant consumers are told to rely on (§10.2 rule 1,
   AVP-127).
3. **The divergence is additive and reversible.** No existing command's exit
   code changes; `openStoreFromCmd` is not modified, and this command does not
   call it (it calls `FindProjectRoot` directly and never opens a
   `*store.Store`, see below). A future PRD that wanted a cross-command
   convention could adopt exit 3 elsewhere as an enumerated behavior delta —
   §21 Q2 already tracks the analogous question for exit `4`.

What this PRD does **not** do is claim the divergence away: a harness author
who greps for exit 1 as "no workspace" across all tpatch commands will not get
that answer from `prepare --check`, and §16.1's `SPEC.md` row requires the
command's exit envelope — including this row — to be documented alongside the
existing per-command envelopes.

`store.Open` (`internal/store/store.go:134-144`) is **not** the trigger and is
not called by this command at all: `FindProjectRoot` has already established
that `.tpatch` exists at the root it returns, and opening a `*store.Store`
would hand the command the type that exposes every writer (§10.6). The
inspector takes an `*os.Root` (§7.1), not a store.

### 9.3 Error precedence (first match wins)

1. **Cobra/pflag parse or arity** → `1`. Nothing else runs. **This population
   is outside this command's output schema and outside its security guarantee
   entirely** (§14.3.1): the error *text* is pflag's and Cobra's, and the
   `error:` line that carries it is the shared root printer's.
2. **Reserved-surface guard** (`--check` absent) → `4`. Evaluated at the top of
   `RunE`, before slug validation and before workspace discovery, so
   `tpatch prepare ../../etc` outside a workspace is `4`, not `3`.
   Deterministic and asserted by AVP-006.
3. **Slug validation** (§7.2) → `3`, `abort.code: slug-unsafe`. Evaluated
   before any name is composed, before the platform guard, before workspace
   discovery and before `os.OpenRoot`. **rev-3 moved this ahead of the
   platform guard.** `CanonicalSlug` is a pure string predicate with no
   filesystem, root or platform-conditional dependency, so nothing about the
   platform can inform it — and putting it second would break §10.2 rule 8's
   biconditional ("`slug` is `""` **exactly** on `slug-unsafe`") on any
   unsupported target, because a hostile slug there would abort
   `workspace-unsupported-platform` while still withholding the slug. AVP-192.
4. **Platform-confinement guard** (§7.4.1) → `3`, `abort.code:
   workspace-unsupported-platform`. Evaluated after slug validation and before
   `os.OpenRoot`. AVP-177, AVP-179, AVP-191.
5. **Workspace discovery** (§9.2) → `3`, `abort.code:
   workspace-not-initialized`. **This step runs outside the rooted namespace**
   (§7.3 step 1) — it is what produces the directory the root is opened on.
6. **Root open** — `os.OpenRoot(repoRoot)` fails → `3`, `abort.code:
   workspace-root-unopenable`.
7. **Feature-directory walk** (§7.3 step 4), whose own first-match rule is by
   component order → `3`, `abort.code: feature-dir-unsafe` or
   `feature-not-found`.
8. **`status.json` inspection** (§9.4.2, §9.4.3), whose own first-match rule is
   the §9.4.2 ladder → `3` with one of the seven `status-*` abort codes, or
   **continue** for the two non-abort populations (`ok`, `absent`).
9. **Required-artifact instability** → `3`, no `abort` object.
10. **Required-artifact shortfall** → `2`.
11. **Otherwise** → `0`.

Exactly one `abort` object is ever emitted. A run that reaches step 9 or later
has `abort` absent. **Every abort is decided before the first per-artifact
`Lstat`**, which is what makes the `artifacts: []` rule of §10.2 total
rather than a special case; AVP-128 asserts the ordering in source and at
runtime.

### 9.4 The `status.json` inspection and the closed abort catalog

#### 9.4.1 Why `status.json` gets the full treatment

`status.json` is the only file this command reads that it does not classify
into the artifact vocabulary, and it is the file whose bytes reach output —
`feature_state` is echoed from it. rev-1 delegated the read to
`store.LoadFeatureStatus`, i.e. `os.ReadFile` on an absolute pathname
(`internal/store/store.go:351-361`). That read follows symlinks, has no size
bound, no kind check, no identity check and no rooted confinement: every
property §7.4 exists to provide was absent from the one read whose result is
printed. rev-2 closes that gap.

**`prepare --check` never calls `store.LoadFeatureStatus`, `os.ReadFile`, or
any package-level `os` open/stat for this or any other file** (§7.1.2,
AVP-089, AVP-150, AVP-194).

#### 9.4.2 The status capture, and its own cap

The status file is captured through the **same** rooted discipline as an
artifact — component policy with a pre- and post-capture walk, `OpenFile` with
the same build-tagged `openFlags()`, `FileOps.Stat` identity and kind rechecks
(the identity comparison through the same injectable `ops.SameFile`), size
cross-check, the **same shared scratch buffer** (§7.4.5), post-read recheck,
and the same exactly-once `FileOps.Close` with its outcome evaluated before any
parse (§7.3, §7.4) — with one deliberate difference:

**`MaxStatusBytes = 1 MiB` (1,048,576), separate from `MaxArtifactBytes`.**

The two caps answer different questions and are deliberately not shared as one
constant. `MaxArtifactBytes` bounds a human-authored Markdown document whose
size is unpredictable, so it is generous. `MaxStatusBytes` bounds a
machine-written metadata record with a fixed field set — `FeatureStatus` plus
its free-text `Notes`, its `DependsOn` list and its `Verify`/`Rejection`
sub-records (`internal/store/types.go:215,234,236-265`) — written by
`writeJSONAtomic` through the one `SaveFeatureStatus` writer
(`internal/store/store.go:363-377`). 1 MiB is roughly three orders of magnitude
above anything that writer can plausibly produce, and a `status.json` larger
than that is a corruption signal in its own right, not a document to parse. A
smaller, separately-named cap also means widening the artifact cap later
(§21 Q3) cannot silently widen the metadata cap. Q7 records the value as
revisable.

**The status read borrows the artifact buffer; it does not allocate its own.**
Per §7.4.5, the status capture fills `scratch[:MaxStatusBytes+1]`, a slice of
the one array the command allocated. This makes `MaxStatusBytes <
MaxArtifactBytes` a structural invariant of the design (asserted at compile
time), and it is the reason the status cap can stay a separate, independently
revisable constant without costing a second megabyte-scale allocation.
AVP-157, AVP-158, AVP-174, AVP-197.

**The cap value is coupled to a frozen message.** `status-oversize`'s §9.4.5
template names `the 1 MiB inspection limit` verbatim. AVP-201 fails if the
constant and the message disagree.

The status capture produces exactly one value of a closed outcome enum. The
ladder is first-match-wins and total:

| # | Condition | Outcome |
|---|---|---|
| 1 | `status.json` `Root.Lstat` mode is a symlink or reparse point | `symlink` |
| 2 | `Root.Lstat` fails with `ErrNotExist` | `absent` |
| 3 | `Root.Lstat` fails otherwise | `unreadable` |
| 4 | `Root.Lstat` mode is not regular (dir, FIFO, socket, device) | `not-regular` |
| 5 | `Root.Lstat` size > `MaxStatusBytes` | `oversize` (no open, no read) |
| 6 | `Root.OpenFile` fails with `ErrNotExist` | `unstable` |
| 7 | `Root.OpenFile` fails for any other reason (including an `os.Root` escape refusal) | `unreadable` |
| 8 | `f.Stat()` fails | `unreadable` |
| 9 | `!ops.SameFile(pre, post)` (§7.1.1) | `unstable` |
| 10 | `!post.Mode().IsRegular()` | `unstable` |
| 11 | `post.Size() != pre.Size()` | `unstable` |
| 12 | the bounded read returned an error other than `io.EOF` / `io.ErrUnexpectedEOF` | `unreadable` |
| 13 | the bounded read filled the whole `MaxStatusBytes+1` buffer | `unstable` |
| 14 | `int64(n) != post.Size()` | `unstable` |
| 15 | post-read `f.Stat()` fails | `unreadable` |
| 16 | post-read size differs | `unstable` |
| 16a | `f.Close()` returns an error (§7.4.4 step 11a, applied identically here) | `unreadable` |
| 17 | the captured bytes are not valid JSON, or are valid JSON that is not an object, or fail to unmarshal into `FeatureStatus` | `malformed` |
| 18 | the unmarshalled `State` is not accepted by the closed `FeatureState` list (§7.1), including the empty string | `invalid-state` |
| 19 | otherwise | `ok` |

Row 16a is lettered rather than renumbered so every row number cited in rev-2's
and rev-3's prose still resolves. The status ladder is **20 rows** (1–16, 16a,
17–19). Its placement mirrors the artifact ladder exactly: **after** the last
descriptor-scoped probe and the post-capture walk, and **before** any
interpretation of the captured bytes — so a `status.json` whose descriptor
would not close cleanly is never parsed into a lifecycle state. It maps to the
existing `unreadable` outcome and therefore to the existing
`status-unreadable` abort code; **no fourteenth abort code is minted**
(§9.4.4). AVP-204.

Row 17 folds whitespace-only and zero-byte bytes into `malformed`: unlike an
intent artifact, an empty metadata record has no honest reading — there is no
"present but empty lifecycle state". Row 18 is separate from row 17 because the
document parsed: the file is well-formed and the *value* is the problem, and
the two need different remediations (`doctor` repairs a broken document;
an unknown state usually means a newer tpatch wrote it).

#### 9.4.3 The nine populations, and what each one does

The outcome enum has nine values, and the mapping to behavior is total:

| Outcome | Behavior | `feature_state` | Abort code |
|---|---|---|---|
| `ok` | **continue** the full four-artifact inspection | the echoed `FeatureState` value | — |
| `absent` | **continue** the full four-artifact inspection | `"unknown"` + advisory `feature-state-absent` | — |
| `symlink` | abort | `"unknown"` | `status-symlink-refused` |
| `not-regular` | abort | `"unknown"` | `status-not-regular` |
| `oversize` | abort | `"unknown"` | `status-oversize` |
| `unreadable` | abort | `"unknown"` | `status-unreadable` |
| `unstable` | abort | `"unknown"` | `status-unstable` |
| `malformed` | abort | `"unknown"` | `status-malformed` |
| `invalid-state` | abort | `"unknown"` | `status-invalid-state` |

**`ok` — the only population that echoes.** `feature_state` is emitted **only**
when the document parsed *and* its `State` is a member of the closed list. The
echo is the enum constant, never the raw bytes from the file. AVP-166.

**`absent` — continue, do not abort.** A feature directory that exists without
a `status.json` is a legitimate, reachable shape: `ListFeatures` already skips
such directories rather than treating them as corruption
(`internal/store/store.go:208-227`), and a half-created or hand-assembled
feature directory is exactly the population an inspection command should be
able to describe. The run continues through the full four-artifact inspection,
the advisory `feature-state-absent` is emitted (§10.4), and the exit code is
derived from readiness exactly as normal — `0`, `2` or `3`. Aborting here would
make the command useless for the population that needs it most, and would
assert a precondition the artifacts do not depend on: **no artifact
classification reads `status.json` at all.** AVP-123, AVP-124.

**The seven abort populations.** A `status.json` that is present but cannot be
turned into a trustworthy lifecycle state is workspace corruption, and
`doctor` D1 owns both its detection and its repair
(`internal/workflow/doctor_d1.go:14-52`). `prepare --check` refuses to report
around it, because a read-only inspector that silently tolerated a corrupt
metadata file would become a second, weaker doctor with no repair path. The
seven codes stay distinct from each other so the remediation can differ, and
because grouping them would reintroduce exactly the "one bucket, several
different truths" shape §1 documents.

**No status bytes are ever echoed.** Not the invalid state value (row 18), not
the malformed document or any fragment of it (row 17), not an `os` error string
(rows 3/7/8/12/15/16a), not a symlink target (row 1), not a size (row 5), not
an absolute path. Every abort message is the fixed template in §9.4.5 and every
interpolation is a canonical slug, a repo-relative canonical path, or a closed
code. AVP-164, AVP-185.

#### 9.4.4 The complete abort catalog

Thirteen codes. The catalog is closed; a fourteenth requires a schema decision
(§10.2's versioning rule) and a new row here.

| `abort.code` | Trigger | Source anchor for the underlying condition |
|---|---|---|
| `slug-unsafe` | the slug argument is not canonical per §7.2 | `internal/store/slug.go:9-12,44-51` |
| `workspace-unsupported-platform` | `GOOS` is one where `os.Root` cannot guarantee confinement (§7.4.1) | `$GOROOT/src/os/root.go` `Root` doc comment; `$GOROOT/src/os/root_noopenat.go` build tag |
| `workspace-not-initialized` | workspace discovery failed: the start directory could not be resolved, or no `.tpatch/` exists at it or any ancestor | `internal/store/store.go:23-40`; `internal/cli/cobra.go:3782-3793` |
| `workspace-root-unopenable` | `os.OpenRoot(repoRoot)` failed | `$GOROOT/src/os/root.go` `OpenRoot` |
| `feature-dir-unsafe` | a symlink/reparse component, a non-directory, or a non-absence `Root.Lstat` failure at or above `.tpatch/features/<slug>` | §7.3 step 4 |
| `feature-not-found` | a component of `.tpatch/features/<slug>` does not exist | `internal/store/store.go:786` |
| `status-symlink-refused` | `status.json` is a symlink or reparse point (ladder row 1) | §9.4.2 |
| `status-not-regular` | `status.json` is not a regular file (ladder row 4) | §9.4.2 |
| `status-oversize` | `status.json` exceeds `MaxStatusBytes` (ladder row 5) | §9.4.2 |
| `status-unreadable` | a `status.json` operation failed for a non-absence reason (rows 3, 7, 8, 12, 15, 16a — including a `Close` failure) | `internal/store/store.go:352-354` |
| `status-unstable` | `status.json` changed identity, kind or size across its capture window (rows 6, 9, 10, 11, 13, 14, 16) | §9.4.2 |
| `status-malformed` | `status.json` was read but is not a valid `FeatureStatus` document (row 17) | `internal/store/store.go:356-359` |
| `status-invalid-state` | `status.json` parsed but its `State` is not a value this tpatch understands (row 18) | `internal/store/types.go:39-46` |

#### 9.4.5 Closed abort messages

`abort.message` is a **fixed template per code**. No template wraps an `os`
error, interpolates an absolute path, or echoes any byte of the slug argument
or of `status.json`. The only interpolation permitted anywhere below is the
canonical slug inside a canonical repo-relative path. AVP-181, AVP-185.

| `abort.code` | `abort.message` (exact) |
|---|---|
| `slug-unsafe` | `the requested feature name is not a canonical tpatch slug. Canonical slugs are lowercase letters, digits and single dashes, 1-60 bytes. Create features with tpatch add, or rename a hand-made feature directory under .tpatch/features/ to a canonical name.` |
| `workspace-unsupported-platform` | `this build of tpatch cannot guarantee that artifact inspection stays inside the repository on this platform, so prepare --check refuses to run here. Inspect the files under .tpatch/features/ directly.` |
| `workspace-not-initialized` | `no tpatch workspace was found here or in any parent directory. Run tpatch init in the repository root, or pass --path with the repository directory.` |
| `workspace-root-unopenable` | `the repository root could not be opened for inspection. Check that the directory still exists and is readable, then re-run.` |
| `feature-dir-unsafe` | `.tpatch/features/<slug> could not be inspected safely: a directory on the way to it is a symbolic link, a reparse point, or not a directory. Replace it with a real directory, or inspect the feature by hand.` |
| `feature-not-found` | `no feature directory exists at .tpatch/features/<slug>. Run tpatch status to list the features in this workspace.` |
| `status-symlink-refused` | `.tpatch/features/<slug>/status.json is a symbolic link or reparse point and was not followed. Replace it with a regular file, then run tpatch doctor.` |
| `status-not-regular` | `.tpatch/features/<slug>/status.json is not a regular file and was not read. Replace it with a regular file, then run tpatch doctor.` |
| `status-oversize` | `.tpatch/features/<slug>/status.json exceeds the 1 MiB inspection limit and was not read. Inspect it by hand, then run tpatch doctor.` |
| `status-unreadable` | `.tpatch/features/<slug>/status.json could not be read and closed cleanly, so the lifecycle state was not determined. Check the file's permissions and the filesystem it lives on, then run tpatch doctor.` |
| `status-unstable` | `.tpatch/features/<slug>/status.json changed while it was being read, so the lifecycle state could not be determined. Re-run when no other tpatch process is writing this feature.` |
| `status-malformed` | `.tpatch/features/<slug>/status.json was read but is not a valid tpatch status document. Run tpatch doctor to inspect and repair the workspace metadata.` |
| `status-invalid-state` | `.tpatch/features/<slug>/status.json was read but records a lifecycle state this version of tpatch does not recognise. Upgrade tpatch, or run tpatch doctor to inspect the workspace metadata.` |

Two properties of the table are asserted mechanically rather than trusted:

- **Every code has exactly one message and every message belongs to exactly one
  code** (AVP-181).
- **No message contains an absolute path, a `docs/` path, a `.md` filename, a
  URL, an `os` error string, or any byte of the raw argument** (AVP-185,
  AVP-186, AVP-078).

**Abort report shape.** On every abort code without exception:

| Field | Value |
|---|---|
| `slug` | the canonical slug, or `""` for `slug-unsafe` (§7.2) |
| `feature_state` | `"unknown"` |
| `artifacts` | `[]` (empty array, never `null`) — nothing was inspected |
| `overall.structural_readiness` | `"indeterminate"` |
| `overall.required_total` | `3` (a schema constant) |
| `overall.required_satisfied` | `0` |
| `overall.optional_total` | `1` (a schema constant) |
| `overall.optional_satisfied` | `0` |
| `advisories` | `[]` — advisories describe inspected artifacts, and none were |
| `abort` | present, with the one code and its §9.4.5 message |
| exit code | `3` |
| stderr | exactly one `error:` line from §9.5 |

The `*_satisfied` counters are `0` because nothing was inspected. They are
**not** a claim that the artifacts are missing; `artifacts: []` together with a
present `abort` object is the documented discriminator, and it is exact:
`artifacts` is empty **if and only if** `abort` is present (AVP-127).

`feature_state` is `"unknown"` on every abort, including
`workspace-not-initialized` — it is never the empty string, so its domain is
the `FeatureState` enum (`internal/store/types.go:8-37`) plus the single
sentinel `"unknown"`, and a consumer never has to handle a third shape.

### 9.5 Closed catalog of process-level error messages

Because the root printer emits `error: %v` for **every** non-nil `RunE` error
(`internal/cli/cobra.go:33-39`), the `Message` of each `*ExitCodeError` this
command returns is part of the observable contract and is therefore closed:

| Exit | Condition | `ExitCodeError.Message` (fixed template) |
|---|---|---|
| `2` | not ready | `prepare --check <slug>: not_ready (<n> of 3 required artifacts are present-nonempty)` |
| `3` | required artifact unstable | `prepare --check <slug>: indeterminate (a required artifact changed while it was being inspected; re-run when no other tpatch process is writing this feature)` |
| `3` | abort, canonical slug known (**twelve** codes: `workspace-unsupported-platform`, `workspace-not-initialized`, `workspace-root-unopenable`, `feature-dir-unsafe`, `feature-not-found`, and the seven `status-*` codes) | `prepare --check <slug>: indeterminate (<abort-code>)` |
| `3` | abort `slug-unsafe` | `prepare --check: indeterminate (slug-unsafe)` |
| `4` | reserved-surface refusal | the single line in §5.3 |

Twelve canonical-slug-known codes plus `slug-unsafe` is the unchanged
**thirteen** of §9.4.4. rev-3 wrote "eleven" here and in §10.5.2 while listing
5 + 7 = 12 codes in the same cell; rev-4 corrects both occurrences and the
arithmetic now reconciles with §9.4.4, AVP-098, AVP-101 and AVP-127.

**Exit `1` is not owned here, and rev-4 states the ownership precisely.** The
error *text* is produced by `spf13/pflag` or `spf13/cobra` during parsing,
before `RunE`. Because the root command sets `SilenceUsage: true` and
`SilenceErrors: true` (`internal/cli/cobra.go:56-62`), cobra prints **neither a
usage block nor the error**; it returns the error from `rootCmd.Execute()`, and
the repository's own printer emits it verbatim as `error: %v`
(`internal/cli/cobra.go:33-39`) before `exitCodeFor` maps the untyped error to
`1` (`internal/cli/cobra.go:43-52`). So the *line shape* is the repository's
and the *interpolated text* is the flag library's, unsanitized. Those bytes are
outside this catalog, outside §10.2's schema and outside §14.3.3's byte rules
(§14.3.1, AVP-193).

The `error:` line carries the **abort code**, not the abort message: the
message is the report's, the line is the process's, and duplicating a
multi-sentence remediation onto stderr would make the last line of every failed
run unstable to grep. The code is the stable machine token, and §10.5's quiet
line carries the same one.

Every template interpolates only: the literal command name, a **canonical**
slug, a small integer, and a closed abort code from §9.4.4. No absolute path,
no `os` error string, no artifact path, no `status.json` bytes, no raw slug
bytes (§14.3). AVP-101 asserts the catalog is closed over all thirteen abort
codes and AVP-078 asserts the absolute-path property against a sentinel-rooted
fixture.

## 10. Output contracts

### 10.1 Stream routing, composed with the real root error printer

The report-shaped part of the routing mirrors the shipped `verify` convention
exactly (`internal/cli/verify.go:109-124`), so harness authors do not learn a
second rule:

| Flags | stdout | stderr (report part) |
|---|---|---|
| *(none)* | full human report | — |
| `--json` | JSON report only | full human report |
| `--quiet` | one readiness line | — |
| `--json --quiet` | JSON report only | — |

**On top of that**, `internal/cli/cobra.go:33-39` prints `error: %v\n` to
stderr for every non-nil `RunE` error, and this command returns one for every
nonzero exit. rev-0's claim that `--json --quiet` leaves stderr empty was
therefore false for exits 2, 3 and 4. The complete, composed contract is:

| Exit | flags | stdout | stderr |
|---|---|---|---|
| `0` | *(none)* | human report | empty |
| `0` | `--json` | JSON report | human report |
| `0` | `--quiet` | one line: `prepare --check <slug> — ready` | empty |
| `0` | `--json --quiet` | JSON report | empty |
| `2` | *(none)* | human report | one `error:` line (§9.5) |
| `2` | `--json` | JSON report | human report, then one `error:` line |
| `2` | `--quiet` | one line: `prepare --check <slug> — not_ready` | one `error:` line |
| `2` | `--json --quiet` | JSON report | one `error:` line |
| `3` | *(none)* | human report (abort form when `abort` is present) | one `error:` line |
| `3` | `--json` | JSON report | human report, then one `error:` line |
| `3` | `--quiet` | one line: `prepare --check <slug> — indeterminate` or `… — indeterminate (<abort-code>)` | one `error:` line |
| `3` | `--json --quiet` | JSON report | one `error:` line |
| `3` | any, `slug-unsafe` | as above but the slug is withheld everywhere (§7.2) | one `error:` line, slug withheld |
| `4` | any | **empty** | exactly one `error:` line (§5.3) |
| `1` | any | **empty** | exactly one `error:` line from the shared root printer (`internal/cli/cobra.go:33-39`) carrying pflag/Cobra's unknown-flag, arity or missing-value text verbatim. **No usage block** — the root sets `SilenceUsage: true` (`internal/cli/cobra.go:56-62`) — and no sanitisation (§14.3.1) |

Rules that make this testable:

1. The report is written **before** `RunE` returns, so the `error:` line is
   always the **last** line of stderr.
2. Exit `0` emits **no** `error:` line on any stream in any flag combination
   (AVP-099).
3. Exits `1` and `4` emit **no report** on either stream (AVP-002…AVP-007).
4. The `error:` line count is exactly one for every nonzero exit — including
   exit 1, where the single line carries pflag/Cobra's text rather than a
   §9.5 template. The command never returns a wrapped multi-error and never
   prints a second diagnostic of its own (AVP-101, AVP-193).
5. The `--quiet` abort line names the abort **code**, and it does so for all
   thirteen codes — a `--quiet` consumer can therefore tell
   `status-malformed` from `feature-not-found` from
   `workspace-unsupported-platform` without parsing JSON (§10.5, AVP-098,
   AVP-184).

### 10.2 JSON schema (v1)

`schema_version: 1`. Field order below is the emission order; it is fixed by Go
struct field declaration order, the same mechanism `doctor` uses
(`internal/workflow/doctor.go:26-33,162-167`). **No Go map appears anywhere in
this schema** — ADR-033 D11's rule, restated in
`internal/store/canonjson.go:11-17`, is that map iteration order must never
reach a wire format.

Ordinary (non-abort) run:

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
      "role": "required",
      "state": "absent",
      "reason_code": "artifact-absent",
      "provenance": "unknown",
      "remediation": "Author .tpatch/features/fix-model-id-translation/exploration.md, then re-run tpatch prepare fix-model-id-translation --check."
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
    "required_total": 3,
    "required_satisfied": 1,
    "optional_total": 1,
    "optional_satisfied": 0
  },
  "advisories": [
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

Abort run (`feature-not-found`):

```json
{
  "schema_version": 1,
  "command": "prepare --check",
  "slug": "no-such-feature",
  "feature_state": "unknown",
  "disclaimer": "Structural presence only. This report does not certify semantic quality.",
  "artifacts": [],
  "overall": {
    "structural_readiness": "indeterminate",
    "required_total": 3,
    "required_satisfied": 0,
    "optional_total": 1,
    "optional_satisfied": 0
  },
  "advisories": [],
  "abort": {
    "code": "feature-not-found",
    "message": "no feature directory exists at .tpatch/features/no-such-feature. Run tpatch status to list the features in this workspace."
  }
}
```

Rules:

1. **`artifacts` is empty if and only if `abort` is present.** When `abort` is
   absent, `artifacts` is exactly length 4, always in the order `analysis`,
   `spec`, `exploration`, `analysis_sidecar`. §9.3 guarantees every abort is
   decided before the first per-artifact `Lstat`, so no abort can ever truncate
   a partially inspected collection — the two shapes are the only two that
   exist. rev-0 emitted four all-`absent` rows on the abort path, which claimed
   an inspection that never happened; rev-1 does not.
2. **`advisories` is sorted** by the fixed advisory-code order of §10.4, is
   `[]` (never `null`) when empty, and is always `[]` on an abort.
3. **`abort`** is an optional trailing object, present only on the §9.3
   step-3…step-8 abort paths: `{"code": "...", "message": "<fixed template>"}`.
   `code` is one of the thirteen values of §9.4.4 and `message` is that code's
   exact §9.4.5 template.
4. **Every string is from a closed catalog.** The only interpolated values are
   a canonical slug and canonical repo-relative paths. No `%v`-wrapped `os`
   error ever reaches output (§14.3).
5. **`reason_code` is `""` exactly when `state == "present-nonempty"`**, and
   non-empty for every other state.
6. **`remediation` is non-empty exactly for a `required` artifact that is not
   `present-nonempty`.** The optional artifact carries advisories instead, so a
   consumer cannot mistake an optional gap for a required action.
7. **`feature_state`** is a `FeatureState` value
   (`internal/store/types.go:8-37`) that `store.ValidFeatureState` accepts
   (`internal/store/types.go:39-46`), or the sentinel `"unknown"`. Never empty,
   and never a value read from `status.json` that failed validation — that
   population aborts with `status-invalid-state` instead (§9.4.3).
8. **`slug`** is a canonical slug, or `""` exactly on `slug-unsafe`.

**Absent by construction** — asserted by AVP-051, which parses the JSON and
compares **exact key names at every nesting level** (never a raw substring
scan): `captured_at`, `generated_at`, `timestamp`, `mtime`, `size`,
`size_bytes`, `bytes`, `sha256`, `hash`, `content`, `excerpt`, `first_line`,
`title`, `path_absolute`, `symlink_target`, `path_kind`, `snapshot_id`.

Key-name scoping is load-bearing, not pedantry: a substring scan for `size`
would match the legitimate state `oversize` and the legitimate reason code
`artifact-oversize`, so a substring-based guard would either be permanently red
or would have to be weakened until it proved nothing. AVP-140 exercises exactly
that case — an `oversize` artifact in the output with the guard green.

For the human surface the same guard compares against the closed set of
**emitted labels** (the fixed left-column tokens of §10.5) rather than
searching the whole rendered text, for the identical reason.

**Versioning.** The report is stdout-only; tpatch never writes it to disk and
never reads it back, so there is no reader-side version rejection (contrast
`internal/store/reconcile_evidence.go:344-345`, which must reject unknown
versions because it re-reads persisted JSONL). Consumers must ignore unknown
fields. Removing a field, renaming a field, changing a field's type, or
changing the meaning of an existing enum value requires `schema_version: 2`.
Adding a field, adding an enum value, or adding an advisory code does not.

### 10.3 Reason-code catalog (closed, total over §7.6)

| `reason_code` | Paired state |
|---|---|
| `""` | `present-nonempty` |
| `artifact-empty` | `present-empty` |
| `artifact-absent` | `absent` |
| `artifact-symlink-refused` | `symlink-refused` |
| `artifact-not-regular` | `not-regular` |
| `artifact-unreadable` | `unreadable` |
| `artifact-oversize` | `oversize` |
| `sidecar-not-json` | `invalid-structured` (row 22) |
| `sidecar-not-json-object` | `invalid-structured` (row 23) |
| `artifact-snapshot-unstable` | `unstable` |

There is no `not-inspected` reason code in rev-1: nothing is ever partially
inspected, because `artifacts` is empty on every abort (§10.2 rule 1).

There is likewise **no `artifact-close-failed` code in rev-4**. A `Close`
failure (ladder row 20c) produces the existing `unreadable` state and therefore
the existing `artifact-unreadable` code, and the sidecar variant produces the
existing `analysis-sidecar-unreadable` advisory (§10.4). The catalog stays at
**ten** reason codes over nine states, §10.4 stays at **ten** advisory codes
and §9.4.4 stays at **thirteen** abort codes, so every totality assertion
(AVP-095, AVP-119, AVP-181) keeps its arithmetic. The reasoning is the same one
§9.4.4 uses for grouping: a code is worth minting when it changes the
remediation, and "the file could not be read and closed cleanly — check
permissions and the filesystem, then run `tpatch doctor`" is the same
remediation whichever syscall failed.

The mapping is **total in both directions**: every one of the nine states in
§7.6 appears in at least one row above, and every row's state is in §7.6. It is
injective except for the single deliberate one-to-two case,
`invalid-structured`, whose two codes are distinguished by ladder rows 22 and
23 (`sidecar-not-json` vs `sidecar-not-json-object`). AVP-095 asserts exactly
this shape — ten codes over nine states with that one documented pair — so
neither an orphan code nor an uncoded state can pass.

### 10.4 Advisory selection — a total state → advisory function

rev-0 selected advisories by artifact id, which let an `*-absent-*` message
describe a file that existed. rev-1 makes advisory selection a **total function
of the optional artifact's state**, evaluated once, producing **at most one**
`analysis-sidecar-*` advisory per run.

| `analysis_sidecar` state | Advisory code | Message (fixed) |
|---|---|---|
| `present-nonempty` | *(none)* | — |
| `absent` | `analysis-sidecar-absent-path-b-normal` | `artifacts/analysis.json is written by the CLI-driven analyze phase and is not produced by analyze --manual. Its absence is not a defect.` |
| `present-empty` | `analysis-sidecar-empty` | `artifacts/analysis.json exists but contains no non-whitespace bytes. This is not a readiness input; the file can be regenerated by re-running the analyze phase or removed.` |
| `invalid-structured` | `analysis-sidecar-invalid-structured` | `artifacts/analysis.json exists but is not a JSON object. This is not a readiness input; the file can be regenerated by re-running the analyze phase or removed.` |
| `unstable` | `analysis-sidecar-unstable` | `artifacts/analysis.json changed while it was being inspected, so its state was not determined. This is not a readiness input; re-run when no other tpatch process is writing this feature.` |
| `symlink-refused` | `analysis-sidecar-symlink-refused` | `artifacts/analysis.json is a symbolic link and was not followed or read. This is not a readiness input; replace it with a regular file or remove it.` |
| `not-regular` | `analysis-sidecar-not-regular` | `artifacts/analysis.json exists but is not a regular file, so it was not read. This is not a readiness input.` |
| `unreadable` | `analysis-sidecar-unreadable` | `artifacts/analysis.json could not be inspected. This is not a readiness input; check the file's permissions or remove it.` |
| `oversize` | `analysis-sidecar-oversize` | `artifacts/analysis.json exceeds the 4 MiB inspection limit and was not read. This is not a readiness input; inspect it manually.` |

**Exclusivity and truthfulness invariants** (AVP-119, AVP-120, AVP-121,
AVP-122):

- exactly one row of the table applies to any run, because `state` is exactly
  one value of the closed enum;
- `analysis-sidecar-absent-path-b-normal` is emitted **only** when
  `state == absent`, so no advisory can claim a file is absent while its own
  artifact row reports it as present, corrupt, refused or unstable;
- every non-`absent`, non-`present-nonempty` state has its own truthful,
  **neutral** message: none of them calls the condition a defect, because none
  of them is one for readiness;
- adding a state to §7.6 without adding a row here fails AVP-119.

Two further advisories are not keyed on the sidecar:

| Advisory code | Emitted when | Message (fixed) |
|---|---|---|
| `feature-state-absent` | the feature directory exists but `status.json` does not (§9.4.3) | `This feature directory has no status.json, so the lifecycle state is reported as unknown. Artifact inspection is unaffected: no artifact classification reads status.json.` |
| `provenance-unknown-by-design` | every non-abort run | `Per-artifact provenance is reported as unknown for every artifact. tpatch does not yet persist durable per-artifact source metadata.` |

**Emission order** (fixed; `advisories` is sorted by this rank, not by
insertion):

1. `feature-state-absent`
2. `analysis-sidecar-absent-path-b-normal`
3. `analysis-sidecar-empty`
4. `analysis-sidecar-invalid-structured`
5. `analysis-sidecar-unstable`
6. `analysis-sidecar-symlink-refused`
7. `analysis-sidecar-not-regular`
8. `analysis-sidecar-unreadable`
9. `analysis-sidecar-oversize`
10. `provenance-unknown-by-design`

The maximum length of `advisories` is therefore 3 on a non-abort run
(`feature-state-absent`, at most one `analysis-sidecar-*`, and the constant),
and 0 on an abort. rev-0's `exploration-absent-verify-requires-later` and
`optional-artifact-unstable` codes are **removed**: `exploration.md` is now a
required artifact with a remediation, and the only optional artifact has its
own named codes.

### 10.5 Human output

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
  exploration.md                     absent
    → Author .tpatch/features/fix-model-id-translation/exploration.md, then
      re-run tpatch prepare fix-model-id-translation --check.
optional
  artifacts/analysis.json            absent

provenance: unknown (all artifacts)

advisories
  analysis-sidecar-absent-path-b-normal
  provenance-unknown-by-design

readiness: not_ready (1 of 3 required artifacts are present-nonempty)
Structural presence only. This report does not certify semantic quality.
```

#### 10.5.1 The lifecycle line — one exact annotation per population

The second line of every human report is the lifecycle line, and its
parenthetical must be **true of what actually happened**. rev-1 printed
`(status.json was not read)` on every abort, which is a false statement for
`status-malformed`, `status-invalid-state`, `status-unreadable` and
`status-unstable` — in three of those the file *was* read, and in the fourth
the read was attempted and failed partway. rev-2 fixes the annotation per
population. The table is total over §9.4.3's nine outcomes plus the six
non-status aborts:

| Population | Lifecycle line (exact) |
|---|---|
| status outcome `ok` (non-abort) | `lifecycle state: <state>  (echoed from status.json; not evaluated by this check)` |
| status outcome `absent` (non-abort) | `lifecycle state: unknown  (this feature directory has no status.json)` |
| abort `slug-unsafe` | `lifecycle state: unknown  (no feature was identified, so status.json was not read)` |
| abort `workspace-unsupported-platform` | `lifecycle state: unknown  (inspection was refused on this platform, so status.json was not read)` |
| abort `workspace-not-initialized` | `lifecycle state: unknown  (no workspace was found, so status.json was not read)` |
| abort `workspace-root-unopenable` | `lifecycle state: unknown  (the repository root could not be opened, so status.json was not read)` |
| abort `feature-dir-unsafe` | `lifecycle state: unknown  (the feature directory could not be inspected safely, so status.json was not read)` |
| abort `feature-not-found` | `lifecycle state: unknown  (no feature directory exists, so status.json was not read)` |
| abort `status-symlink-refused` | `lifecycle state: unknown  (status.json is a symbolic link or reparse point and was not followed)` |
| abort `status-not-regular` | `lifecycle state: unknown  (status.json is not a regular file and was not read)` |
| abort `status-oversize` | `lifecycle state: unknown  (status.json exceeds the inspection limit and was not read)` |
| abort `status-unreadable` | `lifecycle state: unknown  (status.json could not be read and closed cleanly)` |
| abort `status-unstable` | `lifecycle state: unknown  (status.json changed while it was being read)` |
| abort `status-malformed` | `lifecycle state: unknown  (status.json was read but is not a valid status document)` |
| abort `status-invalid-state` | `lifecycle state: unknown  (status.json was read but records a state this tpatch does not recognise)` |

Three properties, each asserted (AVP-153, AVP-154, AVP-164):

- **Truthfulness.** No annotation says "was not read" for a population where a
  read was performed or attempted; the four read-touching status aborts each
  say what actually happened instead. **rev-4 widens `status-unreadable`'s
  annotation to "could not be read and closed cleanly"** because status ladder
  row 16a routes a `Close` failure into that same code: with the rev-3 wording
  a run that read every byte and then failed to close would have printed
  "could not be read", which is the same class of small false statement this
  table exists to remove. The widened form is true of all six of its rows
  (3, 7, 8, 12, 15, 16a).
- **No echo.** No annotation contains the unrecognised state value, any
  fragment of the document, an `os` error string, a size, or an absolute path.
  `status-invalid-state` in particular describes the failure without printing
  the offending value.
- **Totality.** The table has one row per reachable population and the renderer
  is a `switch` over the closed outcome/abort enums, so adding a fourteenth
  abort code without adding a row here fails the guard.

Abort form (nothing was inspected, so no artifact block is printed):

```text
prepare --check  no-such-feature
lifecycle state: unknown  (no feature directory exists, so status.json was not read)

no artifacts were inspected

abort: feature-not-found
  no feature directory exists at .tpatch/features/no-such-feature.
  Run tpatch status to list the features in this workspace.

readiness: indeterminate
Structural presence only. This report does not certify semantic quality.
```

Status-abort form (the file was read; the annotation says so):

```text
prepare --check  fix-model-id-translation
lifecycle state: unknown  (status.json was read but is not a valid status document)

no artifacts were inspected

abort: status-malformed
  .tpatch/features/fix-model-id-translation/status.json was read but is not a
  valid tpatch status document.
  Run tpatch doctor to inspect and repair the workspace metadata.

readiness: indeterminate
Structural presence only. This report does not certify semantic quality.
```

`slug-unsafe` form (the argument is never echoed):

```text
prepare --check  (slug withheld: not a canonical tpatch slug)
lifecycle state: unknown  (no feature was identified, so status.json was not read)

no artifacts were inspected

abort: slug-unsafe
  the requested feature name is not a canonical tpatch slug. Canonical slugs
  are lowercase letters, digits and single dashes, 1-60 bytes.
  Create features with tpatch add, or rename a hand-made feature directory
  under .tpatch/features/ to a canonical name.

readiness: indeterminate
Structural presence only. This report does not certify semantic quality.
```

The abort body is the §9.4.5 message for that code, wrapped for the terminal at
a fixed column. Wrapping is the renderer's; the message string is the same
bytes as the JSON `abort.message`, and AVP-181 compares them after unwrapping.

#### 10.5.2 `--quiet`

`--quiet` (without `--json`) prints exactly one line:

| Outcome | Line |
|---|---|
| ready | `prepare --check fix-model-id-translation — ready` |
| not ready | `prepare --check fix-model-id-translation — not_ready` |
| indeterminate, required artifact unstable | `prepare --check fix-model-id-translation — indeterminate` |
| indeterminate, abort with a canonical slug (**twelve** codes) | `prepare --check fix-model-id-translation — indeterminate (<abort-code>)` |
| indeterminate, `slug-unsafe` | `prepare --check — indeterminate (slug-unsafe)` |

The readiness token in the quiet line is the same token as the JSON
`structural_readiness` field, so a script grepping one is not surprised by the
other. **The abort code is always present on an abort line**, and it is the
same closed token as `abort.code` and as the §9.5 `error:` line — so the three
surfaces never disagree, and a quiet consumer can distinguish all thirteen
abort populations without JSON (AVP-098, AVP-184). The bare
`— indeterminate` form (no parenthetical) is reserved for the one
non-abort indeterminate case, required-artifact instability, which is exactly
how a consumer tells "the tree moved under me, re-run" from "this workspace or
feature is broken".

The quiet line goes to **stdout**; the `error:` line for a nonzero exit goes to
stderr (§10.1), so a `--quiet` consumer reading only stdout still gets exactly
one line.

The closed set of **human labels** the forbidden-field guard compares against
is: `prepare --check`, `lifecycle state`, `required`, `optional`, `provenance`,
`advisories`, `abort`, `readiness`, `no artifacts were inspected`, plus the
four canonical path strings and the frozen disclaimer.

### 10.6 Zero-mutation contract

`prepare --check` must not, on any code path including every error path:

- call `MarkFeatureState`, `SaveFeatureStatus`, `WriteVerifyRecord`,
  `WriteFeatureFile`, `WriteArtifact`, `writeFile`, `writeFileAtomic`,
  `writeJSONAtomic` or `RefreshFeaturesIndex`
  (`internal/store/store.go:368-393,443-472,863-923`) — note that
  `SaveFeatureStatus` also rewrites `FEATURES.md`
  (`internal/store/store.go:363-377`), so a single stray status write mutates
  two tracked files;
- create `.tpatch/features/<slug>/` or `artifacts/`;
- write a lock, temp, scratch or `.orig` file anywhere;
- invoke `git` (no index refresh, no `git status`, no worktree operation);
- open any file for writing, including `O_APPEND` or `O_TRUNC` — §7.4.3's open
  is `O_RDONLY` on every platform, and no `os.Root` mutator (`Create`,
  `Mkdir`, `MkdirAll`, `Remove`, `RemoveAll`, `Rename`, `Chmod`, `Chown`,
  `Chtimes`, `Link`, `Symlink`, `WriteFile`) appears anywhere in the call
  graph;
- call `store.LoadFeatureStatus`, `os.ReadFile`, `os.Open`, `os.OpenFile`,
  `os.Stat` or `os.Lstat` — these are read-only, but they resolve outside the
  rooted namespace and are forbidden for that reason (§7.3 step 6, §9.4.1).

Enforced three ways: a byte-level fixture assertion (AVP-053), a
`git status --porcelain` equality assertion (AVP-054), and an AST source scan
of the implementing packages (AVP-087, AVP-089, AVP-150).

## 11. Provenance

### 11.1 v1 behavior and the stable meaning of `unknown`

Every artifact row emits `"provenance": "unknown"`. The value is a constant. It
is not computed, not configurable, and not affected by any flag, any file, or
any lifecycle state. The advisory `provenance-unknown-by-design` accompanies
every non-abort run so no operator reads `unknown` as an anomaly.

**Definition, fixed now and not revisable by a later PRD:**

> `unknown` means **no trustworthy provenance for this artifact is available
> from an accepted source**.

Three consequences follow, and they are what make the enum forward-compatible:

1. **`unknown` is not "not yet implemented".** It is a permanent, well-defined
   member of the enum with a stable referent. In v1 it happens to be the value
   of every row, because §11.2 shows no accepted source exists yet — but that
   is a fact about the world, not about the schema.
2. **Adding known values later is purely additive.** When a future PRD lands an
   accepted representation (§11.3, §11.4) and starts emitting, say,
   `provider-generated`, `agent-authored` or `human-authored`, an artifact that
   still reports `unknown` means **exactly what it means today**: nothing
   trustworthy was found. No consumer has to re-interpret historical output,
   and no `schema_version: 2` is required, because §10.2's versioning rule
   makes adding an enum value additive and changing the meaning of an existing
   one breaking. This definition is what keeps the addition on the additive
   side of that line.
3. **Legacy artifacts stay `unknown` forever unless proof appears.** No future
   migration may backfill a guess. An artifact written before any provenance
   representation existed has no accepted source, so `unknown` is not a
   placeholder for it — it is the correct final answer.

AVP-129 pins the definition to the constant's doc comment and asserts that the
v1 emitted domain is the single literal `unknown`.

### 11.2 Forbidden inference sources — explicit

The implementation must not read, and must not derive provenance from, any of:

| Forbidden source | Why |
|---|---|
| `FeatureStatus.Notes` | Overwritten on every transition (`internal/store/store.go:388-392`); the `--manual` marker at `internal/store/manual.go:79` survives only until the next phase (`internal/workflow/workflow.go:155`). WP-005 Turn 3 pins this (`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:106-111`). |
| `FeatureStatus.LastCommand` | Records the last *command*, not the source of each *artifact*, and is likewise overwritten (`internal/store/store.go:389`). |
| Filename / path | Both paths write the identical canonical names (`internal/store/manual.go:28-30` vs `internal/workflow/workflow.go:96,151,196`). |
| Sidecar presence | `artifacts/analysis.json` correlates with Path A but does not prove it: it can be hand-authored, and it survives a later `analyze --manual` that overwrites only `analysis.md`. |
| Timestamps (`mtime`, `UpdatedAt`, `RequestedAt`) | Trivially forgeable, non-deterministic, and forbidden in output by ADR-027 D6 / ADR-033 D10. |
| Content (headings, "Generated in heuristic mode" markers, prose style) | The heuristic templates (`internal/workflow/workflow.go:203-259`) are copyable text, not attestations. Reading content for provenance would also breach §14.2. |
| `artifacts/recipe-provenance.json` | Covers `apply-recipe.json` only, records commit/hash not authorship, and is Path-A-and-Git-only (`internal/workflow/implement.go:18-34,222-238`). |
| `intent-archive/**` | Replaced-byte/recovery data, not authorship/provenance; provenance must never be inferred from it (ADR-035 D9). |

AVP-059 and AVP-060 assert both the constant output and the absence of these
reads.

### 11.3 Alternatives for a future durable representation

Evaluated, **not selected** (§11.4):

- **P1 — sub-record on `FeatureStatus`.** Add
  `Provenance *ProvenanceRecord \`json:"provenance,omitempty"\`` alongside
  `Verify` and `Rejection` (`internal/store/types.go:236-251,253-265`).
  *For:* written atomically with state by the one `SaveFeatureStatus` writer
  (`internal/store/store.go:368-377`), exactly the ADR-031 D1 argument;
  `omitempty` gives byte-identical round-trip for every legacy fixture, the
  documented `DependsOn` precedent (`internal/store/types.go:219-234`).
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

### 11.4 The ADR trigger — stated, bounded, not exercised

rev-1 **selects none** of P1/P2/P4 and needs none: `unknown` is a constant with
a stable meaning (§11.1), so no persistent representation is required for this
PRD to be coherent, implementable or testable. **ADR-034 D14 locks this**: it
authorizes no persistent provenance representation, states that the trigger
below stays unfired, and forbids citing ADR-034 as provenance precedent — it is
a filesystem *access* boundary and says nothing about what may be persisted
about an artifact's authorship.

The trigger, per WP-005 Agreed item 8
(`docs/whitepapers/WP-005-spec-driven-workflows.md:84-90`) and Turn 3
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:106-111`), is scoped
narrowly and does not expand:

> The moment any PRD **selects a persistent provenance representation**, an ADR
> (`ADR-0NN-intent-artifact-provenance-representation`) must be written and
> accepted **before** that PRD is accepted for implementation.

The trigger fires on *selection of a persistent representation*, and on nothing
else. In particular it does **not** fire on: adding a known enum value that is
derived from an already-accepted representation, changing an advisory message,
or this PRD's own decision to emit a constant. Keeping the trigger this narrow
is deliberate — a trigger that fires on every provenance-adjacent edit would be
ignored.

That ADR would have to lock: which of P1/P2/P4; the field set; whether a
content hash is persisted and whether it is ever emitted; the closed source
vocabulary; the migration answer for artifacts that predate it (which must
remain `unknown` per §11.1, never backfilled by guess); and the
concurrency/atomicity story against `status.json`.

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
| `implement --manual` with a whitespace-only recipe | refuses (`internal/store/manual.go:72-74`) | refuses (unchanged) |
| `implement --manual` with invalid-JSON recipe | refuses, no transition | refuses, no transition (unchanged) |

AVP-064 through AVP-069 pin these rows — including the counter-intuitive ones —
as *deliberately unchanged*, so a later refactor cannot tighten them by
accident and call it a bug fix.

### 12.2 The differential this creates, stated and tested end to end

Because the gate stays loose and the report is strict, a feature can be
`defined` and simultaneously `not_ready`. That divergence is the *point* of the
slice, not a bug, and rev-1 requires it to be proven by composite end-to-end
rows rather than asserted in prose. AVP-130…AVP-133 each: (a) advance a real
feature to `analyzed`/`defined` through the real `--manual` command using a
deliberately degenerate artifact — zero-byte, whitespace-only, or a symlink;
(b) assert the transition succeeded; (c) run the real `prepare --check` against
the same feature; (d) assert the honest non-`ready` verdict and the exact
per-artifact state; and (e) assert `status.json` and the whole `.tpatch/` tree
are byte-identical afterwards.

A test that only ran `prepare --check` on a hand-built fixture would not prove
this: it would not show that the shipped mutating path really does accept the
input the report calls deficient.

### 12.3 Why report-only

1. **Existing Path B workflows would break.** The operator guide teaches
   authoring the three Markdown files by hand before advancing
   (`docs/path-b-operator-guide.md:63-73`). A stub-then-fill sequence — write
   the file, advance, fill it in — is legal today. Tightening the gate turns a
   working agent loop into a refusal with no migration path.
2. **It would retroactively change what `defined` means.** Features that
   reached `defined` through a gate that accepted empty content would become
   unreachable-by-replay. WP-005 Agreed item 5 requires existing `defined`
   features be "reported, not retroactively invalidated"
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:67-72`).
3. **WP-005 Agreed item 9 makes slice 1 advisory** by construction
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:91-94`).
4. **Separation of concerns.** The value of this PRD is a truthful *report*.
   Coupling the report to a *gate* means every classification bug becomes a
   workflow outage.

### 12.4 What a future tightening PRD must contain

Not authorized here. To tighten any `--manual` gate, a future PRD must
enumerate, at minimum:

- the exact command × input matrix that newly refuses, in the shape of §12.1;
- the exit code for each new refusal and whether it is `ExitCodeError`-typed;
- the migration answer for features already in `analyzed` / `defined` that were
  advanced through the looser gate;
- whether `os.Stat` → a rooted, no-follow lookup (which changes symlink
  behavior for shipped workflows) is in scope;
- whether the bundle-completeness semantics of §6.2 become a gate, which is a
  routing behavior delta and not merely a stricter file check;
- the deprecation/announcement path across `CHANGELOG.md`, the six skill
  surfaces and `docs/agent-as-provider.md`.

"Reuse the inspector in `AdvanceStateManually`" is not a sufficient
specification of a behavior change.

## 13. Compatibility and migration

### 13.1 No lifecycle change

No new `FeatureState`. The enum stays exactly as it is
(`internal/store/types.go:6-18,30,36`), and `ValidFeatureState`'s closed switch
(`internal/store/types.go:41`) is untouched. `prepare --check` performs no
transition, so `MarkFeatureState`'s `unapplied` guard
(`internal/store/store.go:385-387`) is never reached from this path.

### 13.2 Every existing behavior is preserved

| Surface | Guarantee |
|---|---|
| `defined` semantics | Unchanged. A `defined` feature that reports `not_ready` is still `defined`; nothing re-derives, downgrades or annotates its state (AVP-070). |
| `analyze` / `define` / `explore` / `implement` (Path A) | Unchanged; not routed through the inspector. |
| the same commands with `--manual` | Unchanged — the §12.1 decision, pinned by AVP-064…AVP-069 and the composite rows AVP-130…AVP-133. |
| `next` | Unchanged. It keeps using `fileExistsAt` (`internal/cli/phase2.go:439,555-558`); its `HarnessTask` output is byte-identical to the pre-change binary's recorded goldens for all four routing populations (AVP-071, AVP-136), and a reverse call-graph guard proves `internal/cli/phase2.go` neither imports nor calls the inspector (AVP-134, AVP-135). |
| `cycle` | Unchanged. No new phase, no new gate, no new refusal; compared against the pre-change binary's golden transcript and final state (AVP-072, AVP-137). |
| `verify` | Unchanged, including `intent_files_present`'s `os.Stat` + `Size() == 0` semantics (`internal/workflow/verify.go:413-439`). The remaining, deliberate difference is now in one direction only: `prepare --check` additionally requires `analysis.md`, which `verify` does not check; both require `spec.md` and `exploration.md`. rev-0's opposite-direction divergence (a `ready` verdict that `verify` would later block) is gone. |
| `status`, `doctor`, `apply`, `record`, `land`, `reconcile`, `reject`/`reopen`, `feature unapply`, `session` | Unchanged; no new check, no new finding, no new field. |
| status labels (`ComposeLabels`) | Unchanged. Readiness is never a label and is never persisted. |
| `FEATURES.md` | Never rewritten (§10.6). |
| Worktrees and the real Git index | Untouched — the command never invokes Git. |
| `status.json` on disk | Byte-identical before and after (AVP-053). |

### 13.3 Legacy and non-standard features

`prepare --check` inspects a feature in **any** lifecycle state and never
refuses on state:

| Existing population | Behavior |
|---|---|
| `requested` (no intent artifacts yet) | Reports three `absent` required rows, `not_ready`, exit 2. This is a truthful description, not an error. |
| `analyzed` | Typically `analysis` present, `spec` and `exploration` absent → `not_ready`. |
| `defined` **without** `exploration.md` | `not_ready`, exit 2, with `exploration: absent` and a remediation. **This is the population most affected by the §6.2 decision.** It is a report about the intent bundle, not about the feature: the feature remains `defined`, `next` still routes it to `explore` exactly as before, `cycle` still runs it, `verify` behaves identically, and nothing is annotated or persisted (AVP-070, AVP-071, AVP-136). WP-005 Agreed item 5's "reported, not retroactively invalidated" is satisfied literally — this is the report. |
| `defined` **with** `exploration.md` | Typically `ready`, exit 0. |
| `implementing`, `applied`, `active`, `reconciling`, `reconciling-shadow`, `blocked`, `upstream_merged` | Inspected and reported normally. Post-implementation features are not the intended audience, but the command must not pretend they do not exist. |
| `rejected` | Inspected and reported normally. The slug is explicit, so the default-hiding rule that applies to the `status` listing is irrelevant here; no `--include-rejected` flag is added (AVP-066). |
| `unapplied` | Inspected and reported normally. `prepare --check` deliberately does **not** adopt `refuseIfUnappliedState` (`internal/cli/feature_unapply.go:464-473`), which guards *mutating* verbs. Refusing a read-only inspection on an `unapplied` feature would be a gratuitous new restriction (AVP-067). |
| Feature directory with no `status.json` | Fully inspected; `feature_state: "unknown"`; advisory `feature-state-absent`; exit from readiness (§9.4.3, AVP-123, AVP-124). |
| A **canonically named**, hand-assembled feature directory (created by hand, not by `tpatch add`) | Fully inspected exactly like any other feature. If it carries the three Markdown files it reports `ready`, exit 0, whether or not it has a `status.json` (AVP-124, AVP-186). This is the only hand-assembly claim the PRD makes. |
| A hand-assembled feature directory whose **name is not a canonical slug** | Refused with `slug-unsafe`, exit 3, and the argument is never echoed (§7.2). The remediation is self-contained and loop-free: create through `tpatch add`, or rename the directory. It deliberately does **not** say "run `tpatch status`", because `ListFeatures` applies no canonicality filter (`internal/store/store.go:208-227`) and would print the same non-canonical name back, sending the operator around the same refusal again (AVP-186). |
| Features created before this PRD | No migration, no backfill, no new file. `provenance: unknown` is exactly correct for them under §11.1, and remains correct forever unless §11.4's ADR lands. |
| A feature directory containing extra or legacy files (e.g. `feature.yaml`) | Ignored. The inspector reads four fixed paths and never enumerates the directory. |

### 13.4 Forward compatibility

Adding a fifth artifact, a new state, a new advisory code, or a known
`provenance` value later is additive under §10.2's versioning rule and §11.1's
`unknown` definition. Changing the required set, changing readiness derivation,
or making an advisory affect the exit code is a breaking change that requires a
`schema_version` bump **and** an enumerated behavior delta.

## 14. Security and privacy

### 14.1 Path safety

Per §7.2–§7.4 and §9.4.2: canonical slug validation before any name is
composed and before the platform guard; one held `*os.Root` for the repository
root, with every `Lstat` and open handle-relative to it; root-relative names
built only from fixed constants and the canonical slug and asserted canonical
with `fs.ValidPath`; observed symlink and reparse components refused without
being followed, resolved or named, checked **before and after** every capture;
a non-blocking open on Unix and a pre-open kind refusal on Windows so a FIFO or
blocking character device cannot wedge the **open**; post-open descriptor
identity, kind and size rechecks **before the first byte is read**; one reused
`MaxArtifactBytes+1` scratch buffer; a post-read recheck; and exactly one
`Close` per successful open, whose failure is classified rather than discarded.

The threat model is a hostile or corrupted `.tpatch/` tree plus a hostile
argument — for example `spec.md` symlinked to `/etc/shadow`, `status.json`
replaced by a junction on Windows, `exploration.md` replaced by a FIFO that
would block a naive reader's *open* forever, an ancestor directory swapped for
a symlink at exactly the instant between the walk and the open, a slug of
`../../../etc`, or a sparse file that grows during the read. Every one of these
is classified and reported without resolving a name outside the repository
root, without reading an object observed to be different from the one
classified, without an unbounded allocation, and without echoing the hostile
bytes into command-owned output.

**Four boundaries of the threat model, stated rather than implied:**

1. **`--path` is trusted input, and workspace discovery is outside the rooted
   capture.** `store.FindProjectRoot` (`internal/store/store.go:23-40`) runs
   before `os.OpenRoot` and is an ordinary symlink-following pathname walk in
   the shipped store package (§7.3 step 1, §9.3 step 5). An operator who points
   `--path` at a hostile directory gets a root opened on that directory; the
   command then confines itself to it, and reports about it. Nothing in this
   PRD hardens the discovery walk, and nothing claims to.
2. **The root is a logical name boundary, not a physical one.** §7.4.2 is
   normative here: mount points, bind mounts, `/proc` special files and device
   files nameable from inside the root remain reachable. They are handled by
   the kind gate and the bounded read, and the PRD makes no claim that the
   bytes it reads physically originate inside the repository's filesystem.
3. **The identity guarantee is "observed as different".** §7.4.4 and §8.3
   enumerate six limits — same-length rewrite, same-identity alias, hard-link
   alias, inode/file-ID reuse, swap-and-restore, and the residual
   walk→`Lstat`→open window. Bytes read through an unobserved consistent alias
   are attributed to the canonical artifact name.
4. **Availability is out of scope. This command is not hardened against a
   denial of service, and rev-4 states that instead of implying the
   opposite.** A hostile or broken filesystem can make an ordinary read of an
   ordinary regular file block indefinitely (stalled NFS/SMB, a wedged FUSE
   server, a `/proc`-style provider, an unresponsive device driver). v1 has no
   deadline, no cancellation and no watchdog, so the honest security property
   is **confidentiality and integrity, not availability**. rev-3's "without a
   hang" in this paragraph is withdrawn. Q11, AVP-207.

The guarantees stated exactly, and no more than exactly, in §7.4.4:
**logical confinement** (no resolved name leaves `repoRoot`), **no observed
substitution** (an object observed as different is never read), **bounded
allocation and bounded bytes requested** (exactly one data buffer per
invocation; at most `MaxArtifactBytes+1` bytes from any one descriptor), and a
**non-wedging open** on Unix. Every limit is stated in §7.4.2, §7.4.4 and
§8.3, and none is claimed away anywhere in the shipped output — AVP-152,
AVP-189 and AVP-207 are the mechanical guards.

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
- **No raw slug bytes.** A non-canonical slug is never echoed on any stream
  (§7.2), so the command cannot be used to reflect attacker-controlled bytes
  into a log or a terminal.
- **No `status.json` bytes.** An unrecognised lifecycle state, a malformed
  document, or any fragment of either is never echoed (§9.4.3, §10.5.1). Only
  a value that passed the closed `FeatureState` validation reaches output.

### 14.3 Diagnostics hygiene

#### 14.3.1 Scope: command-owned output only

**This subsection's guarantees apply to bytes this command emits, and to
nothing else. rev-3 stated the boundary but mis-attributed the excluded bytes;
rev-4 states the mechanism exactly. The ownership decision recorded here is
locked by ADR-034 D17.**

The command owns:

- the human report and the `--quiet` line on stdout;
- the JSON report on stdout;
- the human report mirrored to stderr under `--json`;
- the single `error:` line the root printer produces from an
  `*ExitCodeError` **this command returned** (`internal/cli/cobra.go:33-39`,
  §9.5).

The command does **not** own, and this section makes no claim about, the
**exit-1 population** of §9.2 and §9.3 step 1: `unknown flag: --manual`,
`accepts 1 arg(s), received 2`, `flag needs an argument: --path`. The mechanism
matters, because rev-3 described it wrongly in two ways:

1. **There is no usage block.** rev-3 wrote "and the usage block cobra prints
   with them". The root command sets `SilenceUsage: true`
   (`internal/cli/cobra.go:56-62`), so cobra prints **no** usage text on error,
   for this command or any other. Any acceptance row or doc sentence that
   expects one is asserting behavior this binary does not have.
2. **Cobra does not print the error either.** `SilenceErrors: true` is set on
   the same struct. Cobra returns the parse error from `rootCmd.Execute()`, and
   **this repository's own printer** writes it: `fmt.Fprintf(os.Stderr,
   "error: %v\n", err)` at `internal/cli/cobra.go:33-39`, after which
   `exitCodeFor` finds no `*ExitCodeError` and returns `1`
   (`internal/cli/cobra.go:43-52`).

So the excluded bytes are a **repository-emitted line wrapping third-party
error text**. The text is produced by `spf13/pflag` or `spf13/cobra` from the
raw `os.Args` the shell handed the process, before `RunE` runs, and pflag
interpolates the offending argument verbatim. **This PRD does not claim that
text is sanitized**, and the fact that the repository's own `Fprintf` carries
it does not make it so: the printer is a shared, pre-existing surface
(`SPEC.md`-level behavior for every command), it applies no filtering, and
changing it would be a cross-command behavior change no single command's PRD
may make. An argument containing an ESC byte therefore *can* reach stderr
through an unknown-flag or arity error. rev-2's unqualified "no byte of stdout
or stderr is an ASCII control character" claimed otherwise and is withdrawn.

Also not owned: anything a shell, terminal, pager or CI log processor does
downstream.

**What still holds for the hostile slug, unconditionally.** The exit-1
population is reached only for a *flag* or *arity* error. A hostile slug
supplied as the positional argument with a well-formed flag set parses fine,
reaches `RunE`, and is refused by `CanonicalSlug` at §9.3 step 3 — at which
point §7.2's withholding rule applies and **the argument bytes are never
echoed** (AVP-102…AVP-104, AVP-106). So the hostile-slug case the threat model
cares about is fully covered; what is excluded is the case where the operator
also mistyped a flag, and there the text belongs to the flag library.

**The scoped guard (AVP-193, rewritten in rev-4).** rev-3's row asserted the
exclusion and then attached a sensitivity fixture — "a fixture that intercepts
and re-renders cobra's error inside `RunE`" — that cannot exist: a parse error
is raised *before* `RunE`, so no `RunE` body can intercept it, and a guard
whose negative fixture is unconstructible proves nothing. The real, scoped
guard is a source-plus-behavior pair over the mechanism that *can* actually
change:

- **Source half.** The `prepare` command file declares no
  `FlagErrorFunc`, no `SetFlagErrorFunc`, no `SetErr`, no `SetOut`, no custom
  `Args` validator that formats its own message, and does not set
  `SilenceUsage`/`SilenceErrors` locally (it inherits the root's). An AST scan
  asserts each absence. **Sensitivity**: adding a `FlagErrorFunc` that rewrites
  pflag's message fails the guard, because installing one moves those bytes
  into this command's ownership without a schema decision.
- **Behavior half.** For each of the five exit-1 inputs: exit code is exactly
  1; stdout is empty; stderr is exactly one line beginning `error: `; that line
  matches **none** of the §9.5 templates and contains no `abort.code`; no
  report is emitted on either stream; and `.tpatch/` is byte-identical. The
  guard's own name and doc comment record that the line's text is unsanitized
  third-party output, so a reader cannot mistake the exclusion for an
  oversight.

#### 14.3.2 No wrapped `os` errors, no absolute paths

Raw `error` strings from `os` frequently embed absolute paths (and therefore
usernames and home-directory layout). No command-owned output string —
including every `ExitCodeError.Message` in §9.5 and every `abort.message` in
§9.4.5 — is built by wrapping an `os` error. Every message is a fixed template
whose only interpolations are a canonical slug, a repo-relative canonical path,
a small integer and a closed code. Absolute paths never appear in
command-owned stdout or stderr. AVP-078 asserts this against a fixture rooted
in a directory whose name is a distinctive sentinel, across every
command-owned exit code including all thirteen aborts.

#### 14.3.3 The byte-level rule, stated correctly and scoped

rev-1 asserted that output "remains printable ASCII". That was both wrong and
unenforceable: this project's house style uses non-ASCII characters
deliberately — the em dash in the `--quiet` line
(`prepare --check <slug> — ready`, §10.5.2) and the `→` remediation marker in
the human report (§10.5) are both required output, and both are multi-byte
UTF-8. A printable-ASCII assertion would fail on the command's own happy path.

The real property is about **control bytes and attacker-supplied bytes**, over
**command-owned output** as scoped by §14.3.1:

1. **No control bytes.** No byte of command-owned stdout or stderr is an ASCII
   control character — `0x00`–`0x08`, `0x0B`, `0x0C`, `0x0E`–`0x1F`, or `0x7F`
   — with the single exception of the `0x0A` line terminators the renderer
   itself emits. In particular no `0x1B`, so no terminal escape sequence can be
   reflected through this command's own report or `error:` line. `0x09` (tab)
   and `0x0D` (carriage return) are also excluded: the renderer emits neither,
   and either could be used to overwrite a line.
2. **No attacker argument bytes.** No byte sequence from the raw `<slug>`
   argument appears in command-owned output unless the argument was accepted by
   `CanonicalSlug`, in which case it is by construction `[a-z0-9-]` only.
3. **Valid UTF-8, house style preserved.** Command-owned output is valid UTF-8
   and may contain the project's non-ASCII punctuation. The guard checks for
   control bytes and argument bytes, not for a restricted alphabet.

AVP-104 and AVP-187 assert all three over the full flag × exit × abort matrix
**of command-owned populations** (exits 0, 2, 3 and 4), including a slug
argument built from `0x1B[2J`, a raw newline and a non-ASCII rune. AVP-193
asserts the complementary exit-1 population is excluded by design and says so
in the guard's own name, so a reader of the test cannot mistake the exclusion
for an oversight.

### 14.4 No provider, no network, no subprocess

The command constructs no provider, reads no API key or token from config or
environment for use, performs no network I/O, and spawns no subprocess
(including `git`). AVP-081 is a source scan; AVP-082 asserts a successful run
with an intentionally broken provider configuration. **This is why the command
has no remote wait to bound — it is not a claim that it has no wait at all**
(§5.1, §8.3, AVP-207).

## 15. Failure and recovery

| Failure | Behavior | Recovery |
|---|---|---|
| Slug is not canonical | exit 3, `abort.code: slug-unsafe`, argument never echoed | create through `tpatch add`, or rename the feature directory to a canonical name (§7.2 — deliberately **not** "run `tpatch status`") |
| Platform cannot guarantee rooted confinement | exit 3, `abort.code: workspace-unsupported-platform` | inspect `.tpatch/features/<slug>/` by hand on this platform |
| Not a tpatch workspace (including a `--path` that resolves nowhere useful) | exit 3, `abort.code: workspace-not-initialized` | `tpatch init`, or pass the right `--path` |
| Repository root could not be opened | exit 3, `abort.code: workspace-root-unopenable` | check the directory still exists and is readable |
| Unknown slug | exit 3, `abort.code: feature-not-found` | `tpatch status` to list slugs |
| Feature dir is a symlink/reparse point or not a directory | exit 3, `abort.code: feature-dir-unsafe` | inspect manually; the command never resolves it |
| `status.json` absent | **no abort**; full report with `feature_state: unknown` + advisory | none needed; `tpatch status` if the feature should exist |
| `status.json` is a symlink/reparse point, not a regular file, or oversize | exit 3, `abort.code: status-symlink-refused` / `status-not-regular` / `status-oversize` | replace it with a regular file (or inspect it by hand), then `tpatch doctor` |
| `status.json` unreadable, unstable, malformed, or records an unknown state | exit 3, `abort.code: status-unreadable` / `status-unstable` / `status-malformed` / `status-invalid-state` | `tpatch doctor` (D1 owns metadata repair, `internal/workflow/doctor_d1.go:14-27`); for `status-unstable`, re-run when nothing else is writing; for `status-invalid-state`, upgrade tpatch. A `status.json` whose descriptor fails to close also lands on `status-unreadable` (§9.4.2 row 16a) |
| A required artifact is unreadable | exit 2, `state: unreadable` | fix permissions, re-run |
| A required artifact is a symlink or reparse point | exit 2, `state: symlink-refused` | replace with a regular file |
| A required artifact is a FIFO/device/directory | exit 2, `state: not-regular`; on Unix the open is non-blocking and on Windows the kind is refused before the open, so neither can wedge the open | replace with a regular file |
| A required artifact's descriptor fails to close | exit 2, `state: unreadable`, `reason_code: artifact-unreadable` (ladder row 20c); the captured bytes are discarded and no content state is reported | check the filesystem the workspace lives on, then re-run |
| A required artifact is oversize | exit 2, `state: oversize` | inspect manually; the command refuses to read a file larger than the 4 MiB inspection limit (`MaxArtifactBytes`, §7.4.5 — the figure and the constant are coupled by AVP-201) |
| A required artifact is unstable | exit 3, `state: unstable`, no `abort` object | re-run when no other tpatch process is writing the feature |
| Sidecar in any non-`present-nonempty` state | readiness unaffected; exactly one `analysis-sidecar-*` advisory (§10.4) | re-run `tpatch analyze <slug>` to regenerate, or delete the sidecar |

There is nothing to roll back after any failure: the command has written
nothing. That is the whole recovery story, and it is the strongest argument for
shipping the read-only slice first.

## 16. Rollout, docs and asset parity

### 16.1 Docs the implementation wave must update

| File | Change |
|---|---|
| `SPEC.md` | Add `tpatch prepare <slug> --check [--json] [--quiet] [--path]` to the command table (near `SPEC.md:81`) and add this command's exit-code envelope under the existing per-command exit-code section (`SPEC.md:135-141`), including the exit-4 reserved-surface row. |
| `docs/agent-as-provider.md` | Two required edits. **(a)** After the phase → artifact → state table (`:33-45`), add a short "inspect before you advance" note: `prepare --check` reports the same artifacts read-only, evaluates the whole intent bundle, and never advances state — and `--manual`'s gate is unchanged (§12). **(b) A correction, not an addition**: the current text at `:47-54` presents the `status.json.notes` string as the thing that "distinguishes Path B transitions from provider output". That is true only of the **last** transition, because `MarkFeatureState` overwrites `Notes` every time (`internal/store/store.go:388-392`). The wording must be corrected to say the notes string is a **hint about the most recent transition, not durable per-artifact provenance**, and to point at `provenance: unknown` (§11.1) as the current honest answer. Leaving that sentence as-is would leave a shipped doc contradicting this PRD's central finding. |
| `docs/path-b-operator-guide.md` | In the preferred Path B flow (`:63-73`), show `tpatch prepare <slug> --check` as an optional verification step after authoring the three Markdown files and before the `--manual` commands. It must be presented as optional (§2.2 item 8). |
| `docs/feature-layout.md` | Note that the four intent artifacts are the set `prepare --check` classifies, that the three Markdown files are the readiness set, and that `artifacts/analysis.json` is Path A only. |
| `.github/workflows/ci.yml` | **Required, not optional.** Add `windows-latest` to the test matrix (`.github/workflows/ci.yml:25`, currently `[ubuntu-latest, macos-latest]`). §7.3 step 3 and §7.4.3 rest on Windows-specific `os.Root` behavior — reparse tags mapping to `ModeSymlink` / `ModeSocket` / `ModeIrregular` / regular, handle-derived `GetFileInformationByHandle` identity, `GetFileType`-derived kinds, and the `winsymlink` GODEBUG default — none of which a cross-compile can execute. Cross-building for `GOOS=windows` proves the code compiles; it proves nothing about the behavior this PRD depends on. Without this row, AVP-175, AVP-176, AVP-198 and AVP-199 are unrunnable and the Windows half of the design is unverified. The existing steps (`gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./... -count=1`, install smoke test — `.github/workflows/ci.yml:37-58`) are matrix-wide and need no change. |
| `cmd/tpatch/main.go` | **Required.** Add `//go:debug winsymlink=1` immediately above `package main` (`cmd/tpatch/main.go:1`). §7.4.3 explains why: `(*fileStat).Mode` falls back to the pre-Go-1.23 mapping when the setting is `0`, under which a Windows junction reports `ModeSymlink` instead of `ModeIrregular`. A `//go:debug` directive is honored only in the main package, so it cannot live in `internal/intent`. The module already declares `go 1.26.1` (`go.mod:3`) and therefore already defaults to `1`; the directive makes the dependency explicit and reviewable rather than inherited. AVP-198. |
| `CHANGELOG.md` | New command, new exit-code contract, explicit "no existing behavior changed" line. |

This PRD edits none of them; it is planning-only. `SPEC.md` in particular is
owned by the implementation lane (`AGENTS.md` → File Ownership).

**Native Windows CI is an acceptance obligation of this PRD, stated plainly.**
tpatch's CI matrix today is Linux and macOS only
(`.github/workflows/ci.yml:25`), and this PRD does **not** claim that any
Windows behavior is currently exercised by any runner. It requires that the
implementation wave add `windows-latest` **in the same slice that lands the
Windows open/stat path** (S1, §17), and it makes the native rows (AVP-175,
AVP-176, AVP-198, AVP-199) exit criteria of that slice. A wave that ships the
Windows code without the runner has not satisfied this PRD.

**Junction fixtures must FAIL, not SKIP — this is the rev-3 tightening, and
ADR-034 D13 locks it.** The
Windows half of §7.3 step 3 and §7.4.3 stands or falls on one observable: a
**junction** (`IO_REPARSE_TAG_MOUNT_POINT`) is a name-surrogate reparse point
that maps to `ModeIrregular`, and the refusal predicate must catch it. Creating
a junction in a Go test is not a one-liner: `os.Symlink` creates a *symlink*
(tag `IO_REPARSE_TAG_SYMLINK`), which is the case the predicate would catch
even if the `ModeIrregular` half were wrong, so a symlink fixture proves
nothing about junctions.

The mandated mechanism, in test code, native Windows only:

```go
// windows-only test helper
cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetPath)
out, err := cmd.CombinedOutput()
if err != nil {
    t.Fatalf("mklink /J failed; this test must not be skipped: %v\n%s", err, out)
}
```

`mklink /J` creates a directory junction and, unlike `mklink /D`, does **not**
require the `SeCreateSymbolicLinkPrivilege` or Developer Mode — an ordinary
unprivileged `windows-latest` runner can create one. Two rules follow, and
AVP-199 asserts both:

1. **If `mklink /J` is unavailable or fails for any reason, the test calls
   `t.Fatal`, never `t.Skip`.** A skipped Windows junction test is
   indistinguishable, in a green CI log, from a passing one — and the entire
   reason R7's severity is High is that an unexercised Windows path degrades
   silently. A failure is loud; a skip is a lie of omission.
2. **`t.Skip` is forbidden in the Windows fixture path**, and AVP-199's source
   half asserts no `t.Skip`/`t.Skipf`/`t.SkipNow` appears in the
   junction-fixture helper or in any test that consumes it. The only permitted
   guard is `runtime.GOOS != "windows"`, which is a *platform* skip on
   non-Windows runners, not a *capability* skip on Windows. A sensitivity
   fixture converting the `t.Fatal` back into a `t.Skip` fails the guard.

A test that cannot create the object it exists to test has not passed.

### 16.2 Skill asset parity

The six shipped skill surfaces (`assets/assets_test.go:202-212`) are the
agent-facing contract, and the parity guard exists so a shipped CLI surface
cannot drift from them (`assets/assets_test.go:215-243`). Path B agents are the
primary consumer of a read-only intent check, so v1 **does** ship it in the
skills:

1. Add `"tpatch prepare"` to `requiredCommands` (`assets/assets_test.go:13-53`).
2. Add a verbatim anchor to `requiredAnchors` in the shape of the existing
   verify bullet (`assets/assets_test.go:82-89`), e.g.
   `{"prepare-check/read-only", "tpatch prepare <slug> --check is read-only"}`.
3. Update all six surfaces in the same commit — Claude, Copilot, Copilot
   Prompt, Cursor, Windsurf, Generic.
4. The added text must not contain a bare repo-relative `docs/...md` reference,
   or `TestSkillDocReferencesAreSelfContained`
   (`assets/assets_test.go:281-320`) fails. Guidance must be inlined.
5. **The skill text must present the command as optional** and must not add it
   to the phase-ordering table or the preflight sequence. Non-goal 8 forbids a
   downstream methodology mandate, and the skills are exactly where such a
   mandate would leak in. AVP-092 asserts the phase-ordering and preflight
   anchors are byte-unchanged.
6. **The skill text must say that exit `2` from this command is an expected
   report outcome, not a failure of the workflow or of tpatch.** An agent that
   reads a nonzero exit as "the tool broke" will retry, escalate, or abandon
   the run — and exit 2 here means only "the bundle is incomplete", which is
   the single most common truthful answer this command has. The required
   wording is, verbatim in all six surfaces:

   > `tpatch prepare <slug> --check` exits 2 when the intent bundle is
   > incomplete. That is a report result, not a workflow or system failure:
   > the command wrote nothing, changed nothing, and the per-artifact rows say
   > exactly what is missing. Author the missing files and re-run, or continue
   > without it — this check is optional.

   It must not describe exit 2 as an error, a failure, or a blocker, and it
   must not instruct the agent to abort the workflow on it. AVP-188.

7. **The skill text must not restate any `os.Root` guarantee, and must not
   make a timing promise.** Skill surfaces describe *what the command reports*,
   never *how it reads* and never *how long it takes*. In particular no surface
   may claim the command "cannot read files outside the repository", "detects
   tampering", "is safe against symlink attacks", "always terminates", "cannot
   hang", or "is safe to run in a blocking preflight step" — §7.4.2, §7.4.4 and
   §8.3 bound those claims precisely, and a skill paraphrase is exactly where
   the bound would be lost. An agent that reads "cannot hang" and wires the
   command into a non-cancellable harness step inherits a defect this PRD does
   not own. AVP-189's shipped-string scan covers all six surfaces; AVP-207
   covers the timing half.

Acceptance rows AVP-090, AVP-091, AVP-092, AVP-188, AVP-189, AVP-207.

### 16.3 Rollout

Additive, no flag gate, no config key, no opt-in. A new subcommand cannot
regress an existing invocation, and §13.2 is asserted rather than assumed. No
telemetry, no deprecation, no migration script.

## 17. Implementation slices

Each slice is independently reviewable; all are gated on this PRD being
accepted.

| Slice | Scope | Exit criteria |
|---|---|---|
| **S1** (83 rows) | `internal/intent` core: `CanonicalSlug`, canonical root-relative `fs.ValidPath` constants, the §7.1.1 `RootOps`/`FileOps` seam and its two production adapters, the closed `FeatureState` list, the closed state enum, the §7.5 ladder (including 20a/20b/20c), the §7.3 rooted policy with pre/post component walks, the descriptor-close contract of §7.4.4 step 11a, the §7.4.3 build-tagged `openFlags()` (all three rev-6 target sets), the §7.4.1 fail-closed `unix \|\| windows` platform allowlist, the §7.4.5 shared scratch buffer, the eight instability probes, and the §9.4.2 status ladder (including 16a). Pure; no CLI. **Also lands the `windows-latest` CI matrix row and the `//go:debug winsymlink=1` directive (§16.1), and the pre-change routing goldens.** | AVP-011…AVP-030, AVP-083…AVP-086, AVP-093, AVP-094, AVP-105, AVP-107…AVP-118, AVP-144…AVP-152, AVP-155…AVP-163, AVP-165, AVP-168, AVP-170…AVP-178, AVP-180, AVP-190, AVP-191, AVP-194…AVP-200, AVP-203…AVP-206, AVP-208 |
| **S2** (28 rows) | Report model + renderers: JSON schema, human renderer (ordinary / abort / status-abort / slug-withheld forms), the §10.5.1 lifecycle-line table, the §9.4.5 abort-message catalog, total advisory function, cap↔message coupling, determinism, privacy. Still no CLI wiring. | AVP-039…AVP-052, AVP-059, AVP-077…AVP-080, AVP-119…AVP-122, AVP-153, AVP-154, AVP-181, AVP-187, AVP-201 |
| **S3** (49 rows) | `internal/cli`: `prepareCmd`, flag set, reserved-surface refusal, root open/close lifetime and the scratch-buffer allocation, abort precedence (slug before platform), exit codes, the §9.5 process-message catalog, stream routing composed with the root printer, and the §14.3.1 command-owned output boundary. | AVP-001…AVP-010, AVP-031…AVP-038, AVP-096…AVP-104, AVP-106, AVP-123…AVP-128, AVP-141…AVP-143, AVP-164, AVP-166, AVP-167, AVP-169, AVP-179, AVP-182…AVP-186, AVP-192, AVP-193 |
| **S4** (38 rows) | Zero-mutation, provenance, parity and compatibility proofs; source scans; the composite differential and routing goldens. | AVP-053…AVP-058, AVP-060…AVP-076, AVP-081, AVP-082, AVP-087…AVP-089, AVP-129…AVP-138 |
| **S5** (10 rows) | Docs + six skill surfaces + parity guard extension + guard-sensitivity meta-check + the over-claim and citation guards. | AVP-090…AVP-092, AVP-095, AVP-139, AVP-140, AVP-188, AVP-189, AVP-202, AVP-207 |

Slices S1→S3 are strictly ordered. S4 and S5 may run in parallel with each
other **only if** they touch disjoint files; both touch neither `cobra.go` nor
`internal/intent` production code, so the AGENTS.md parallel-implementer rule
(same-file overlap ⇒ sequential) is satisfiable — but the cluster lead must
declare the file partition at dispatch and stage by explicit path.

**Routing-golden prerequisite.** AVP-136 and AVP-137 compare `next` and `cycle`
output against goldens recorded from the **pre-change binary**. Those goldens
must be captured and committed in S1, before any CLI wiring exists, or the
comparison degenerates into the before/after-a-no-op comparison rev-0 relied on
(which cannot detect a change that is present in both halves).

**Windows-runner prerequisite.** The `windows-latest` matrix row and the
`//go:debug winsymlink=1` directive (§16.1) must land in S1, in the same slice
as the Windows-relevant code, not deferred to S5's docs pass. A slice that adds
the Windows open/stat path without a runner that executes it ships unverified
behavior; AVP-175 is written so it can only pass on a native run, and AVP-199
is written so a runner that cannot create a junction **fails** rather than
quietly skipping.

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
  proxy;
- a row naming a **guard** must assert the guard's semantics — that it fails on
  a wrong input — not that the guard function or its test file exists.

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
| AVP-006 | I | `prepare <slug>` (no `--check`) **outside** any tpatch workspace | exit 4 (not 3); the single `error:` line names `--check`; a filesystem spy records zero calls of any kind — no `FindProjectRoot`, no `os.OpenRoot`, no `Root.Lstat` |
| AVP-007 | I | `prepare <slug>` (no `--check`) inside a workspace with a ready feature | exit 4; stdout empty; `.tpatch/` byte-identical |
| AVP-008 | I | `prepare <slug> --check --path <dir>` from an unrelated cwd | exit 0; inspects the feature under `<dir>`; the held root is the workspace `FindProjectRoot` resolved from `<dir>`, not the cwd |
| AVP-009 | I | `prepare --help` | output states it is unrelated to `apply --mode prepare`; presents `--check` as the only mode |
| AVP-010 | I | `apply --help` | `--mode` description points at `prepare --check` |

### 18.3 B — Structural classification

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-011 | U | `analysis.md` with ordinary content | `present-nonempty`, `reason_code: ""` |
| AVP-012 | U | `spec.md` zero bytes | `present-empty`, `artifact-empty` |
| AVP-013 | U | `spec.md` containing only `" \t\n\r\n"` | `present-empty` (whitespace-only is empty) |
| AVP-014 | U | `spec.md` containing a single `x` | `present-nonempty` |
| AVP-015 | U | `analysis.md` absent | `absent`, `artifact-absent` |
| AVP-016 | U | `spec.md` is a symlink to a readable in-repo file | `symlink-refused` from the pre-open `Root.Lstat` mode test; a spy on `Root.OpenFile` records zero opens for that name |
| AVP-017 | U | `spec.md` is a symlink to a path outside the repo | `symlink-refused`; the target path string appears nowhere in output |
| AVP-018 | U | `spec.md` is a dangling symlink | `symlink-refused` (not `absent`) |
| AVP-019 | U | `artifacts/` is a symlink, sidecar underneath | sidecar is `symlink-refused` (non-leaf component rule, ladder row 1); zero `Root.OpenFile` calls for that name |
| AVP-020 | U | `spec.md` is a directory | `not-regular` |
| AVP-021 | U | `exploration.md` is a FIFO with no writer | `not-regular` (ladder row 7, decided at `Root.Lstat` before any open); the test asserts completion under a hard deadline |
| AVP-022 | U | `spec.md` mode `0000` (unreadable), non-root | `unreadable`, `artifact-unreadable` |
| AVP-023 | U | `spec.md` size = `MaxArtifactBytes` exactly | classified normally (boundary is inclusive-OK); the read is still `Max+1`-bounded |
| AVP-024 | U | `spec.md` size = `MaxArtifactBytes + 1` | `oversize`; the `Root.OpenFile` spy records zero opens and the read spy zero bytes for that name |
| AVP-025 | U | sidecar containing `{"summary":"x"}` | `present-nonempty` |
| AVP-026 | U | sidecar containing `{` | `invalid-structured`, `sidecar-not-json` |
| AVP-027 | U | sidecar containing `[1,2,3]` | `invalid-structured`, `sidecar-not-json-object` |
| AVP-028 | U | sidecar containing `"a string"` | `invalid-structured`, `sidecar-not-json-object` |
| AVP-029 | U | sidecar containing `{"unknown_future_field":1}` | `present-nonempty` — unknown fields never fail |
| AVP-030 | U | sidecar whitespace-only | `present-empty` (emptiness precedes JSON parsing, ladder rows 21 vs 22–23) |

### 18.4 C — Readiness and exit codes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-031 | I | all three required `present-nonempty`, sidecar absent | `ready`; exit 0; `required_total: 3`, `required_satisfied: 3`, `optional_total: 1`, `optional_satisfied: 0` |
| AVP-032 | I | `analysis` + `spec` present, `exploration` absent | `not_ready`; exit 2; `required_satisfied: 2`; `exploration` row `absent` with a non-empty `remediation` |
| AVP-033 | I | all three required absent (fresh `requested` feature) | `not_ready`; exit 2; `required_satisfied: 0`; three non-empty remediations |
| AVP-034 | I | all three required present; sidecar absent | `ready`; exit 0 — the optional gap never blocks, and `optional_satisfied: 0` does not affect readiness |
| AVP-035 | I | all three required present; sidecar `invalid-structured` | `ready`; exit 0; advisory `analysis-sidecar-invalid-structured` present |
| AVP-036 | I | `exploration` `unstable`, other two `present-nonempty` | `indeterminate`; exit 3; **no** `abort` object; `artifacts` length 4 |
| AVP-037 | I | `spec` `unstable` | `indeterminate`; exit 3; no `abort` object |
| AVP-038 | I | unknown slug | `indeterminate`; exit 3; `abort.code: feature-not-found`; `artifacts: []` |

### 18.5 D — Output shape, order and determinism

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-039 | I | `--json` on a ready feature | stdout parses as JSON; `schema_version: 1`; `command: "prepare --check"` |
| AVP-040 | I | `--json` | top-level key order is exactly `schema_version, command, slug, feature_state, disclaimer, artifacts, overall, advisories[, abort]` |
| AVP-041 | I | `--json`, non-abort input | `artifacts` has length 4 in order `analysis, spec, exploration, analysis_sidecar` |
| AVP-042 | I | `--json` on every one of the **thirteen** abort codes (§9.4.4) | `artifacts` is `[]` (empty array, not `null`, not length 4); `advisories` is `[]`; `abort` present with that exact code |
| AVP-043 | I | `--json` with no advisories triggerable except the constant one | `advisories` is a JSON array (never `null`) containing exactly `provenance-unknown-by-design` |
| AVP-044 | I | `--json` with `feature-state-absent` + a sidecar advisory + the constant | order matches §10.4's fixed rank list; length is exactly 3 |
| AVP-045 | I | artifact-object key order | exactly `id, path, role, state, reason_code, provenance, remediation` |
| AVP-046 | I | both surfaces | the disclaimer string appears byte-for-byte in JSON `disclaimer` and as the human report's last line, including on abort and slug-withheld forms |
| AVP-047 | I | `--json` alone, exit 0 | JSON on stdout; human report on stderr; no `error:` line |
| AVP-048 | I | `--json --quiet`, exit 0 | JSON on stdout; stderr **empty** |
| AVP-049 | I | `--quiet` alone, exit 0 | stdout is exactly one line ending in ` — ready` |
| AVP-050 | I | two consecutive runs on a quiescent tree | stdout **and** stderr byte-identical for both human and `--json`, at exit 0 and at exit 2 |
| AVP-051 | G | `--json` over a matrix covering every state and every abort | a recursive **key-name** walk finds none of the forbidden key names of §10.2 at any nesting level; the human surface is checked against the closed label set of §10.5, not by substring search |
| AVP-052 | I | `remediation` population | non-empty exactly for `required` artifacts not `present-nonempty`; empty for the optional artifact in all nine of its states |

### 18.6 E — Zero mutation

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-053 | I | ready feature, run `--check` | every file under `.tpatch/` byte-identical before/after; the file set is identical (no additions, no deletions) |
| AVP-054 | I | inside a Git repo with `.tpatch/` tracked | `git status --porcelain` output identical before/after |
| AVP-055 | I | every abort path (all thirteen codes) | `.tpatch/` byte-identical; no directory created |
| AVP-056 | I | run against a slug whose feature directory does not exist | `.tpatch/features/<slug>/` is **not** created |
| AVP-057 | I | `.tpatch/` mounted read-only (or all files mode `0444`) | the command still succeeds and reports |
| AVP-058 | I | `FEATURES.md` present | byte-identical after the run |

### 18.7 F — Provenance

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-059 | I | every artifact in every state | `provenance` is the literal `"unknown"` in all four rows, every time |
| AVP-060 | S | inspector + renderer packages | no reference to `Notes`, `LastCommand`, `UpdatedAt`, `RequestedAt`, `ModTime`, or `recipe-provenance.json` |
| AVP-061 | I | feature advanced via `<phase> --manual` (notes carry the manual marker) | `provenance` is still `"unknown"` for every artifact |
| AVP-062 | I | feature advanced via Path A (`analyze` then `define` then `explore`) | `provenance` is still `"unknown"` for every artifact |
| AVP-063 | I | feature with `artifacts/analysis.json` present and valid | `provenance` is still `"unknown"`; no field or message claims Path A |

### 18.8 G — Compatibility (including the §12 decision)

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-064 | I | `define --manual` with a **zero-byte** `spec.md` | still exits 0 and state becomes `defined` — deliberately unchanged (§12.1) |
| AVP-065 | I | `analyze --manual` with a **whitespace-only** `analysis.md` | still exits 0 and state becomes `analyzed` — deliberately unchanged |
| AVP-066 | I | `prepare --check` on a `rejected` feature | reports normally; exit reflects readiness; no `--include-rejected` flag exists on `prepare` |
| AVP-067 | I | `prepare --check` on an `unapplied` feature | reports normally; **not** refused by the unapplied guard |
| AVP-068 | I | `explore --manual` with `exploration.md` a symlink to a readable file | still exits 0 and state becomes `defined` — deliberately unchanged (`--manual` still uses `os.Stat`) |
| AVP-069 | I | `implement --manual` with an invalid-JSON recipe, and separately with a whitespace-only recipe | still refuses in both cases, state unchanged — unchanged (`internal/store/manual.go:67-78`) |
| AVP-070 | I | `defined` feature (no `exploration.md`) reporting `not_ready`, then `tpatch status --json` | the feature's state field is still `defined`; no readiness field appears in `status` output; `status.json` byte-identical |
| AVP-071 | I | `next <slug>` across `requested`/`analyzed`/`defined`-pre-explore/`defined`-post-explore | output byte-identical in both `text` and `harness-json` formats to the S1-recorded pre-change goldens (see AVP-136) |
| AVP-072 | I | `cycle <slug> --skip-execute` with a heuristic (no-provider) workspace | same phases run, same final state, same stdout as the pre-change golden — no new gate |

### 18.9 H — Path A / Path B parity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-073 | I | Path B feature (`--manual` throughout, all three Markdown files hand-authored), no sidecar | `ready`; exit 0; advisory `analysis-sidecar-absent-path-b-normal` present |
| AVP-074 | G | full output matrix | no output field or message asserts that *this feature* is Path A or Path B; the key `path_kind` and the values `"A"`/`"B"` for any path-role field never appear |
| AVP-075 | I | Path A feature (heuristic provider) after `analyze` + `define` + `explore` | `ready`; sidecar `present-nonempty`; `optional_satisfied: 1` |
| AVP-076 | I | sidecar present and valid but `analysis.md` deleted | `not_ready`; exit 2 — the sidecar never substitutes for a required artifact |

### 18.10 I — Security and privacy

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-077 | I | artifacts containing a distinctive sentinel string | the sentinel appears in neither stdout nor stderr, in any flag combination |
| AVP-078 | I | workspace rooted at a directory containing a distinctive sentinel path segment | no absolute path (and no sentinel segment) appears in stdout or stderr on any path — exit 0, 2, 3 (all thirteen aborts) and 4, including the `error:` line and every `abort.message` |
| AVP-079 | G | full output matrix | no 64-hex-character token appears anywhere (no content hashes) |
| AVP-080 | I | symlink refusal case | the symlink's target path appears nowhere in output |
| AVP-081 | S | inspector + CLI command packages | no import of `internal/provider`, `net/http`, `os/exec`; no `exec.Command` call |
| AVP-082 | I | workspace whose provider config points at an unreachable endpoint with a bogus key | `prepare --check` never reads provider config and makes no network attempt; exit depends only on the inspected workspace. This row makes no timing claim (D16). |

### 18.11 J — Concurrency and snapshot

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-083 | U | deterministic hook deletes `spec.md` between the leaf `Lstat` and the `OpenFile`, driven through the §7.1.1 seam's `before` hook | `unstable`, `artifact-snapshot-unstable` (not `absent`) — ladder row 9 |
| AVP-084 | U | deterministic hook replaces `spec.md` with a different inode between `Lstat` and `fstat` | `unstable` via the `ops.SameFile` probe — ladder row 13 (the verdict is injectable, AVP-206) |
| AVP-085 | U | hook truncates `spec.md` to zero after `fstat`, before the read | `unstable` via the byte-count/size disagreement — **never** `present-empty` — ladder row 18 |
| AVP-086 | I | quiescent tree | no artifact is `unstable`; no snapshot/atomicity field exists in the JSON (`snapshot_id`, `captured_at` absent by key-name walk) |

### 18.12 K — Source scans and parity guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-087 | S | inspector package | imports neither `internal/store` nor `internal/gitutil`; the CLI command's call graph reaches none of the writer symbols listed in §10.6, and no `os.Root` mutator (`Create`, `Mkdir`, `MkdirAll`, `Remove`, `RemoveAll`, `Rename`, `Chmod`, `Chown`, `Chtimes`, `Link`, `Symlink`, `WriteFile`) |
| AVP-088 | U | canonical path constants | `analysis.md`, `spec.md`, `exploration.md` equal `store.ManualPhase("analyze"\|"define"\|"explore").Path`; the sidecar path equals `filepath.Join("artifacts", "analysis.json")` |
| AVP-089 | S+G | inspector package **and** the `prepare` command file | zero calls to `os.Stat`, `os.Lstat`, `os.Open`, `os.OpenFile`, `os.ReadFile`, `os.ReadDir` or `filepath.Walk`; every name lookup is `(*os.Root).Lstat`, every open is `(*os.Root).OpenFile`, every descriptor stat is `(*os.File).Stat`; a sensitivity fixture reintroducing one `os.Lstat` fails the guard |
| AVP-090 | U | `assets/assets_test.go` | `requiredCommands` contains `tpatch prepare` |
| AVP-091 | U | all six skill surfaces | each contains the verbatim `prepare --check` read-only anchor |
| AVP-092 | U | all six skill surfaces | `TestSkillDocReferencesAreSelfContained` still passes with the new text; the `phase-ordering/table`, `phase-ordering/never-skip` and the five `preflight/*` anchors are byte-unchanged, so the command was not added to the mandated sequence (§16.2 item 5) |

### 18.13 L — Totality and completeness guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-093 | G | state enum totality | the implementation's exported state constants, sorted, equal the §7.6 table parsed from this PRD; a state added in code without a PRD row fails, and vice versa |
| AVP-094 | G | precedence ordering | the ordering test asserts `unstable` outranks `present-empty` and `invalid-structured`, `symlink-refused` outranks `not-regular`, and pre-open `oversize` outranks the open; reordering any of the three pairs in the implementation fails it |
| AVP-095 | G | catalog totality | every reason code (§10.3), every advisory code (§10.4) and every one of the thirteen abort codes (§9.4.4) is produced by at least one AVP row's fixture, and no code exists in the implementation that no row produces; the reason-code ↔ state mapping is total in both directions over §7.6's nine states, with `invalid-structured` the single documented one-to-two case; every abort code has exactly one §9.4.5 message and one §10.5.1 lifecycle line |

### 18.14 M — CLI output envelope, composed with the root error printer

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-096 | I | `--json --quiet` on a `not_ready` feature | exit 2; stdout is the JSON report only; stderr is **exactly one** line, equal to `error: ` + the §9.5 exit-2 template |
| AVP-097 | I | `--quiet` (no `--json`) on a `not_ready` feature | exit 2; stdout is exactly one line ending ` — not_ready`; stderr is exactly one `error:` line |
| AVP-098 | I | `--quiet` on each of the thirteen abort codes | exit 3; stdout is exactly one line ending ` — indeterminate (<abort-code>)` (or the slug-withheld form); stderr is exactly one `error:` line naming the same code; all thirteen stdout lines are pairwise distinct |
| AVP-099 | I | exit-0 run in all four flag combinations | stderr contains **no** line beginning `error:`; with `--json --quiet` stderr is completely empty |
| AVP-100 | I | `prepare <slug>` without `--check` | exit 4; stderr is exactly one `error:` line; that line names `--check` and `tpatch prepare --help`, and contains **no** `docs/` path, no `.md` filename, and no URL |
| AVP-101 | G | every reachable nonzero exit (2, 3 × thirteen aborts, 3 × instability, 4) | the emitted `error:` line matches one of the closed §9.5 templates exactly; the line count is exactly one; no template contains an absolute path, a wrapped `os` error, or any `status.json` byte; a sensitivity fixture adding a fourteenth abort code without a template fails the guard |

### 18.15 N — Slug safety

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-102 | I | `prepare '../../etc' --check` | exit 3; `abort.code: slug-unsafe`; the argument bytes appear in neither stdout nor stderr; a filesystem spy records **zero** `Lstat`/`open` calls anywhere |
| AVP-103 | I | `prepare '/etc/passwd' --check` | same as AVP-102 |
| AVP-104 | I | slugs containing a newline, an ESC (`0x1B`) byte, a tab, a non-ASCII rune, `..`, a trailing dash, a doubled dash, an uppercase letter, and a 61-byte value | each: exit 3, `slug-unsafe`, argument bytes never echoed on either stream, and the §14.3 control-byte rule holds (see AVP-187) |
| AVP-105 | U | slug grammar round-trip | every non-empty output of `store.Slugify` over a corpus (including 60-byte truncation cases) is **accepted** by `CanonicalSlug`; every Windows reserved device name is **refused**; `CanonicalSlug` never rewrites its input |
| AVP-106 | I | `slug-unsafe` in `--json`, human and `--quiet` forms | JSON `slug` is `""`; the human header prints the withheld form; the quiet line omits the slug; `feature_state` is `"unknown"` |

### 18.16 O — Race-safe capture and bounded reads

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-107 | U | hook replaces `spec.md` with a FIFO (no writer) between `Root.Lstat` and `Root.OpenFile` | the open returns without blocking (hard test deadline); result is `unstable` — the descriptor-kind recheck, ladder row 14. On Unix this is `O_NONBLOCK`; on Windows the handle-derived `GetFileType` kind |
| AVP-108 | U | hook replaces `spec.md` with an in-root symlink to a **different** file between `Root.Lstat` and `Root.OpenFile` | `unstable` via the row-13 identity comparison — **not** `symlink-refused`, because the link was never observed; zero bytes are read from the substituted object; its path is never named. This row pins §7.4.4's second table row and is the reason no acceptance row claims a final no-follow refusal, and no caller-supplied `O_NOFOLLOW` exists to claim one with |
| AVP-109 | U | injected `fstat(fd)` failure | `unreadable`, `artifact-unreadable` (ladder row 12) |
| AVP-110 | U | descriptor is a non-regular file although `Lstat` reported regular | `unstable` (ladder row 14) |
| AVP-111 | U | `fstat` size differs from the `Lstat` size | `unstable` (ladder row 15) |
| AVP-112 | U | file grows past `MaxArtifactBytes` during the read | `unstable` (ladder row 17), **not** `oversize`; `io.ReadFull` returned `err == nil` with `n == MaxArtifactBytes+1`; a counting reader asserts at most `MaxArtifactBytes+1` bytes were requested |
| AVP-113 | U | file grows within the cap during the read | `unstable` via the byte-count disagreement (ladder row 18) |
| AVP-114 | U | post-read `fstat` reports a different size | `unstable` (ladder row 20) |
| AVP-115 | U | injected post-read `fstat` failure | `unreadable` (ladder row 19) |
| AVP-116 | S+G | inspector package | every artifact read fills the caller-supplied `MaxArtifactBytes+1` scratch slice and every status read the `scratch[:MaxStatusBytes+1]` sub-slice of the **same array** (§7.4.5); `internal/intent` contains no `make([]byte, …)` for a data buffer at all; zero calls to `os.ReadFile`, `io.ReadAll`, `io.LimitReader` or `bufio.Scanner`; the guard fails if either `+1` is dropped, if either limit is widened, if a growable slice replaces the fixed buffer, or if the status capture allocates its own array |
| AVP-117 | U | each of the **eight** instability probes (§8.3, including the post-capture ancestor walk) applied to `analysis_sidecar` | sidecar is `unstable` with `artifact-snapshot-unstable`; advisory `analysis-sidecar-unstable` is emitted; readiness is `ready` when the three Markdown artifacts are `present-nonempty`; exit 0 |
| AVP-118 | G | platform contract | the build-tagged `openFlags()` set is exhaustive and disjoint over the **three** rev-6 halves `unix` / `windows` / `!(unix \|\| windows)`; the `unix` file returns **exactly** `syscall.O_NONBLOCK`, the `windows` file **exactly** `0`, and the unsupported file **exactly** `0` with a doc comment recording that it exists for buildability and is unreachable in production because the platform gate aborts before any root is opened (§7.4.3 rev-6 erratum); `syscall.O_NOFOLLOW` appears in **no** file (§7.4.3's rev-3 removal); no file calls `syscall.CreateFile`, `rescap.openNoFollow`, or a bare `os.Open`; no `openFlags()` return value contains a write, create, truncate or append bit; the guard's doc comment records that `O_NONBLOCK` bounds the **open** only and asserts no read-time property (§7.4.2, AVP-207); three sensitivity fixtures fail the guard — one adding `os.O_WRONLY`, one reintroducing `syscall.O_NOFOLLOW`, and one collapsing the partition back to rev-4's two `!windows` / `windows` halves |

### 18.17 P — Diagnostic totality

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-119 | G | advisory selection totality | for each of the nine sidecar states in §7.6, the implementation produces exactly the §10.4 row (or none for `present-nonempty`); adding a state without an advisory row fails the guard |
| AVP-120 | I | sidecar `present-empty` | advisory is `analysis-sidecar-empty`; `analysis-sidecar-absent-path-b-normal` is **not** emitted; the message contains no claim of absence |
| AVP-121 | I | sidecar `invalid-structured`, `symlink-refused`, `not-regular`, `unreadable`, `oversize` (five runs) | each emits its own §10.4 code with its neutral message; none claims absence; none affects readiness or exit code |
| AVP-122 | G | every sidecar state | at most one `analysis-sidecar-*` advisory appears in `advisories` on any run; `advisories` length never exceeds 3 |
| AVP-123 | I | feature directory exists, `status.json` absent, all three Markdown files degenerate | exit 2 (readiness-derived, **not** 3); no `abort`; `artifacts` length 4; `feature_state: "unknown"`; advisory `feature-state-absent` present; lifecycle line is the §10.5.1 `absent` row |
| AVP-124 | I | feature directory exists, `status.json` absent, all three Markdown files `present-nonempty` | exit 0; `ready`; `feature_state: "unknown"`; advisory `feature-state-absent`; stderr has no `error:` line |
| AVP-125 | I | `status.json` present but not valid JSON, and separately valid JSON that is not an object, and separately zero bytes, and separately whitespace-only | each: exit 3; `abort.code: status-malformed`; `artifacts: []`; `feature_state: "unknown"`; no byte of the document appears on either stream |
| AVP-126 | I | `status.json` present but unreadable (mode `0000`, non-root) | exit 3; `abort.code: status-unreadable`; `artifacts: []`; `feature_state: "unknown"`; the `os` error string appears on neither stream |
| AVP-127 | G | all thirteen abort codes | each emits `feature_state: "unknown"`, `structural_readiness: "indeterminate"`, `artifacts: []`, `advisories: []`, `required_total: 3`, `optional_total: 1`, both `*_satisfied: 0`, exit 3; and `artifacts` is empty **iff** `abort` is present across the whole matrix; a sensitivity fixture emitting one artifact row alongside an abort fails the guard |
| AVP-128 | S+I | abort ordering | no abort code can be produced after the first per-artifact `Root.Lstat`: an AST check that every abort return precedes the capture loop, plus a runtime spy asserting zero artifact-name `Root.Lstat` calls on every one of the thirteen abort runs |

### 18.18 Q — Provenance forward compatibility

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-129 | G | provenance constant | the emitted domain in v1 is the single literal `"unknown"`; the constant's doc comment states §11.1's definition verbatim; a second emitted value added without a schema decision fails the guard |

### 18.19 R — Composite differential and routing non-invalidation

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-130 | I | real `define --manual` on a **zero-byte** `spec.md`, then real `prepare --check` | step 1 exits 0 and state is `defined`; step 2 exits 2, `spec` is `present-empty` with a non-empty remediation, verdict `not_ready`; `status.json` and the whole `.tpatch/` tree byte-identical after step 2 |
| AVP-131 | I | real `explore --manual` with `exploration.md` a **symlink**, then real `prepare --check` | step 1 exits 0 and state is `defined`; step 2 exits 2 with `exploration: symlink-refused`; no mutation; the symlink target never named |
| AVP-132 | I | real `analyze --manual` on a **whitespace-only** `analysis.md`, then real `prepare --check` | step 1 exits 0 and state is `analyzed`; step 2 reports `analysis: present-empty` |
| AVP-133 | I | all three degenerate artifacts adopted through the real `--manual` commands to reach `defined`, then real `prepare --check` | state is `defined`; report is `not_ready` with three failing required rows and three remediations; `git status --porcelain` identical before/after step 2 |
| AVP-134 | S | reverse call graph | no package other than the `prepare` command file imports `internal/intent`; the inspector's caller set contains no symbol from `internal/cli/phase2.go` or `internal/workflow` |
| AVP-135 | S | `internal/cli/phase2.go` | contains no reference to the inspector package identifier, `Inspect`, `CanonicalSlug` or any state constant |
| AVP-136 | I | `next <slug>` for all four routing populations, both formats | byte-identical to goldens recorded from the **pre-change** binary in S1 — not a before/after comparison across a no-op run |
| AVP-137 | I | `cycle <slug> --skip-execute` transcript + final state | byte-identical to the pre-change binary's recorded golden |
| AVP-138 | I | one feature that simultaneously exhibits §1.1 (zero-byte `spec.md` accepted by `--manual`), §1.2 (Path B, no sidecar) and §1.3 (`Notes` overwritten by a later phase), inspected in one real CLI run | exit 2; `not_ready`; `spec: present-empty`; `provenance: "unknown"` on all four rows; advisory `analysis-sidecar-absent-path-b-normal` present; no message references the notes string |

### 18.20 S — Guard sensitivity and scoping

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-139 | G | meta-check over the guard set, **derived** rather than hand-listed: every matrix row whose Kind string contains `G` (43 rows per §18.27) | each ships a paired sensitivity fixture on which it **fails**; a guard with no such fixture fails this meta-check; the derived set is compared against §18.27's stated arithmetic, and a row whose Kind contains no `G` (e.g. AVP-128, `S+I`) is **not** in the set. The slice-partition and count arithmetic are AVP-202's, so the two meta-guards do not restate each other |
| AVP-140 | I+G | a feature whose `spec.md` is `oversize` and whose sidecar is `oversize` | the report contains `state: "oversize"` and `reason_code: "artifact-oversize"` and the advisory `analysis-sidecar-oversize`, **and** the AVP-051 forbidden-field guard is green — proving the guard is key-name scoped rather than substring scoped |

### 18.21 T — Rooted namespace, ancestor races and root lifetime

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-141 | S+I | root lifetime | `os.OpenRoot` is called **exactly once** per invocation, in the CLI layer, after workspace discovery; an AST check finds exactly one call site in the command's call graph, and a runtime counter asserts one open and one `Close` per run |
| AVP-142 | I | root ownership | `Inspect` never closes the root: after `Inspect` returns, the root is still usable (a further `Root.Lstat` succeeds); `Close` happens once, after the report is rendered |
| AVP-143 | I | `repoRoot` renamed mid-run | with a deterministic hook that renames `repoRoot` after the root is opened and before the first artifact capture, the run completes and reports against the **original** directory in its new location; no path outside it is opened; exit code is readiness-derived |
| AVP-144 | U+G | root-relative names are canonical `fs.ValidPath` names | every name passed to a `RootOps` method — the four artifact names, the status name and every component-walk prefix — satisfies `fs.ValidPath`: relative, slash-separated, non-empty, no `.`/`..`/empty element, valid UTF-8; the implementation asserts it at the call site; an AST check finds no `filepath.Join(repoRoot, …)`, no `filepath.Abs`, no absolute-path literal and **no reference to `safety.EnsureSafeRepoPath`** inside `internal/intent` (§7.3 step 2's rev-3 removal); a sensitivity fixture composing a name with a `..` element fails the assertion |
| AVP-145 | U | ancestor component policy, symlink — `.tpatch`, `features`, `<slug>` and `artifacts` each replaced by a symlink in four separate fixtures | each is refused: the first three abort `feature-dir-unsafe`, the fourth yields sidecar `symlink-refused`; zero `Root.OpenFile` calls |
| AVP-146 | U+G | ancestor component policy, reparse — the same four fixtures using a Windows junction created by `cmd /c mklink /J` (native run, see AVP-199) or an injected `ModeIrregular` `FileInfo` through the §7.1.1 seam (all targets) | each is refused identically to AVP-145, because the predicate tests `ModeSymlink\|ModeIrregular`; a sensitivity fixture testing only `ModeSymlink` lets the junction through and fails the guard |
| AVP-147 | U | ancestor not a directory — `.tpatch/features/<slug>` exists as a regular file | abort `feature-dir-unsafe`; zero artifact `Root.Lstat` calls |
| AVP-148 | U | raced ancestor symlink, in-root, different identity — a hook replaces `artifacts/` with an in-root symlink to a different directory after the walk and before the sidecar open | sidecar is `unstable` (row 13); **zero bytes read**; the substituted directory's path is never named |
| AVP-149 | U | raced ancestor symlink, out-of-root — a hook replaces `artifacts/` with a symlink to a directory outside `repoRoot`, in both a relative `../../..` form and an absolute form | both: `Root.OpenFile` fails; sidecar is `unreadable` (ladder row 10/11); zero bytes read; no path outside `repoRoot` is opened, stat'd or named, asserted with a filesystem spy over the whole process |
| AVP-150 | S+G | forbidden readers | neither `internal/intent` nor the `prepare` command file references `store.LoadFeatureStatus`, `os.ReadFile`, `os.Open`, `os.OpenFile`, `os.Stat`, `os.Lstat` or `*store.Store`; a sensitivity fixture reintroducing `store.LoadFeatureStatus` fails the guard |
| AVP-151 | U | raced leaf symlink, same identity — a hook replaces `spec.md` with an in-root symlink pointing at the very inode `Root.Lstat` observed | the capture proceeds and reports that object's real state; the test asserts the report is **true of the object that was read**, and that **no output string claims the alias was detected or refused**. This row pins §7.4.4's stated limitation; it does not claim a capability |
| AVP-152 | G | no over-claim in shipped strings | no shipped string — help text, human report, advisory, abort message, `error:` line or skill text — contains a claim that every symlink race is detected, that the final open never follows a link, that a same-identity alias is refused, that a hard-link alias is detected, that an inode/file-ID reuse is detected, or that a swap-and-restore is defeated; a sensitivity fixture adding any one of those sentences fails the guard. AVP-189 is the complementary guard over the **documents** (this PRD and ADR-034) |

### 18.22 V — `status.json` inspection totality

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-153 | G | lifecycle-line totality | the human renderer produces exactly the §10.5.1 line for each of the fifteen populations (status `ok`, status `absent`, thirteen abort codes); the renderer is a closed `switch`, and a sensitivity fixture adding a fourteenth abort code without a line fails the guard |
| AVP-154 | I | lifecycle-line truthfulness | for `status-malformed`, `status-invalid-state`, `status-unreadable` and `status-unstable` the line does **not** contain the substring `was not read`; for the six pre-status aborts it does |
| AVP-155 | U | status symlink — `status.json` is a symlink with an in-repo target, an out-of-repo target, and dangling | all three abort `status-symlink-refused`; zero `Root.OpenFile` calls for that name; the target is never named |
| AVP-156 | U | status not regular — `status.json` is a directory, and separately a FIFO with no writer | both abort `status-not-regular`; the FIFO case completes under a hard test deadline |
| AVP-157 | U | status oversize — `status.json` size is `MaxStatusBytes + 1` | abort `status-oversize`; the open spy records zero opens and the read spy zero bytes |
| AVP-158 | U | status boundary — `status.json` size is exactly `MaxStatusBytes` and it is still a valid document | parsed normally; outcome `ok`; the read is still bounded by the `MaxStatusBytes+1` buffer |
| AVP-159 | U | status growth — `status.json` grows past `MaxStatusBytes` during the read | abort `status-unstable` (§9.4.2 row 13), **not** `status-oversize` |
| AVP-160 | U | status identity — a hook replaces `status.json` with a different in-root inode between `Root.Lstat` and `f.Stat()` | abort `status-unstable` (§9.4.2 row 9); zero bytes classified |
| AVP-161 | U | status fstat failure — an injected `f.Stat()` failure on the status descriptor | abort `status-unreadable` (§9.4.2 row 8) |
| AVP-162 | U | status read failure — an injected read error on the status descriptor | abort `status-unreadable` (§9.4.2 row 12) |
| AVP-163 | U | status byte-count disagreement — a hook truncates `status.json` after `f.Stat()` and before the read | abort `status-unstable` (§9.4.2 row 14) — **never** `status-malformed`, because a torn read must not be reported as corruption |
| AVP-164 | I | status invalid state — `status.json` is a valid JSON object whose `state` is `"prepared"`, and separately `""`, and separately a 4 KiB junk string | each: exit 3; `abort.code: status-invalid-state`; the offending value appears in **neither** stdout nor stderr; `feature_state` is `"unknown"` |
| AVP-165 | G | `FeatureState` parity | every value in the inspector's closed list satisfies `store.ValidFeatureState` (`internal/store/types.go:39-46`), and an AST scan of the `FeatureState` const block (`internal/store/types.go:8-37`) yields exactly that list; adding a thirteenth state to `store` without updating the inspector fails the guard, and vice versa |
| AVP-166 | I | status echo domain | across a corpus covering all twelve `FeatureState` values, `feature_state` equals the enum constant exactly; across the seven status abort populations it is `"unknown"`; it is never the empty string and never a value that failed validation |
| AVP-167 | I | status absent, **human surface** — the same fixture as AVP-124, rendered without `--json` | the lifecycle line is the §10.5.1 `absent` row byte-for-byte; `feature-state-absent` is the **first** entry of `advisories` per §10.4's rank list; the disclaimer is the last line; stderr has no `error:` line. AVP-124 asserts the JSON and the exit code; this row asserts the human bytes, so the two are not the same test twice |
| AVP-168 | G | status abort totality | the seven `status-*` abort codes are produced by exactly the §9.4.2 ladder rows listed in §9.4.4, are pairwise disjoint, and cover every non-`ok`, non-`absent` outcome; an outcome added without an abort code fails the guard |
| AVP-169 | I | status abort precedence | with `status.json` malformed **and** all three Markdown artifacts absent, the run aborts `status-malformed` with `artifacts: []` — the status decision precedes artifact inspection (§9.3 step 8) |

### 18.23 W — Fixed-buffer bounded reads

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-170 | U+G | allocation ceiling, per capture | an allocation-counting fixture over a capture of a 1-byte, a 4 MiB−1, a 4 MiB and a growing artifact records **zero** data-buffer allocations inside the capture itself — the buffer is the caller's (§7.4.5) — and no reallocation, no copy and no growth in any case. AVP-197 asserts the complementary per-invocation property |
| AVP-171 | U | read ceiling | a counting reader asserts at most `MaxArtifactBytes+1` bytes are requested from the descriptor in every case, including the unbounded-growth case |
| AVP-172 | G | forbidden read primitives | the inspector contains no `io.ReadAll`, `io.LimitReader`, `os.ReadFile`, `ioutil.ReadFile` or `bufio.Scanner`; a sensitivity fixture substituting `io.ReadAll(io.LimitReader(f, Max+1))` fails the guard, proving the guard tests the mechanism and not just the byte count |
| AVP-173 | U | EOF semantics, total | zero-byte file → `io.EOF`, `n == 0`, captured `buf[:0]`, classified `present-empty`; short file → `io.ErrUnexpectedEOF`, captured `buf[:n]`; exactly-`Max` file → `io.ErrUnexpectedEOF` with `n == Max`, classified normally; file larger than `Max` at read time → `err == nil`, `n == Max+1`, classified `unstable` |
| AVP-174 | U | status buffer | the same four assertions for the status capture against `MaxStatusBytes+1`; the two caps are distinct constants and neither is defined in terms of the other |

### 18.24 X — Platform matrix and native Windows CI

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-175 | G | CI matrix | `.github/workflows/ci.yml`'s test matrix contains `windows-latest`; the guard parses the workflow and fails if it is absent, so the Windows rows below cannot silently become unrun |
| AVP-176 | I | native Windows behavior, guarded by `runtime.GOOS == "windows"` (a **platform** guard; capability skips are forbidden, AVP-199) | on a native runner: a symlink `spec.md` is `symlink-refused`; a **junction** `artifacts/` created per AVP-199 is `symlink-refused` via `ModeIrregular`; a `status.json` reparse point aborts `status-symlink-refused`; `os.SameFile` over `Root.Lstat` and `File.Stat` is true for an unchanged file and false after an injected replacement; a `FILE_TYPE_CHAR` handle is `not-regular`. The test's doc comment records that the identity half rests on version-pinned unexported behavior (G7, G8, G16) and is therefore a **Go-upgrade tripwire**, not a public contract (§7.4.4) |
| AVP-177 | G | platform-confinement constant, runtime half | `rootConfinementSupported` is declared exactly twice; the `false` path returns abort `workspace-unsupported-platform` **before** `os.OpenRoot` is called and before any name is composed; a sensitivity fixture that makes the tags overlap fails the build and the guard. AVP-191 owns the build-tag *text* half (allowlist vs denylist) |
| AVP-178 | G | cross-build | `GOOS=linux`, `GOOS=darwin` and `GOOS=windows` each vet and build the whole module; the guard fails if any target stops compiling |
| AVP-179 | I | unsupported-platform abort shape | with the confinement constant forced `false`: exit 3; `abort.code: workspace-unsupported-platform`; `artifacts: []`; the §9.4.5 message exactly; **zero** filesystem calls of any kind (spy over the process) |
| AVP-180 | S | no raw platform syscalls | neither `internal/intent` nor the `prepare` command file references `syscall.CreateFile`, `windows.Openat`, `GetFileType`, `FILE_FLAG_OPEN_REPARSE_POINT`, or `rescap.openNoFollow`; the Windows contract is expressed entirely through `os.Root` and `os.FileInfo` |

### 18.25 Y — CLI ownership, output bytes and remediation coherence

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-181 | G | abort-message catalog | the thirteen §9.4.5 templates are a bijection with the thirteen §9.4.4 codes; each emitted `abort.message` equals its template byte-for-byte (after the human renderer's wrapping is undone); no template contains `%v`, an `os` error string, an absolute path, a `docs/` path, a `.md` filename or a URL; a sensitivity fixture wrapping an `os` error fails the guard |
| AVP-182 | I | feature-directory walk order — four fixtures in which `.tpatch`, `features`, `<slug>` and the leaf respectively are the first offending component | each yields the abort the §7.3 step-4 order predicts (`feature-dir-unsafe` or `feature-not-found`), and the component **after** the offending one is never passed to `Lstat` |
| AVP-183 | I | `--path` exit ownership — `--path <nonexistent-dir>`, and `--path <existing-dir-with-no-.tpatch-anywhere-above>` | both: exit **3**, `abort.code: workspace-not-initialized`, full abort report emitted — **not** cobra exit 1; the message names `tpatch init` and `--path` and contains no absolute path |
| AVP-184 | I | the genuine exit-1 population — `--path` supplied with no value, `--manual`, `--regenerate`, zero args, two args | each: exit 1 from cobra/pflag before `RunE`; no report on either stream; `.tpatch/` byte-identical. The §14.3.3 byte rules are **not** asserted here — that population is outside command-owned output (§14.3.1, AVP-193) |
| AVP-185 | I | no status bytes leak — fixtures whose `status.json` carries a distinctive sentinel inside the state value, inside a note, and inside malformed bytes | the sentinel appears in neither stdout nor stderr in any flag combination, on `status-malformed` and `status-invalid-state` alike |
| AVP-186 | I | remediation coherence | a hand-assembled feature directory named canonically (three Markdown files, no `status.json`) reports `ready`/exit 0; a hand-assembled directory named `My_Feature` yields `slug-unsafe` whose message names `tpatch add` and the rename path and does **not** contain the string `tpatch status` — following the message once produces a canonical name that the command then accepts |
| AVP-187 | G | output byte rule, over **command-owned** output | over the full flag × exit × abort matrix for exits 0, 2, 3 and 4 — the command-owned populations of §14.3.1 — including a slug argument containing `0x1B[2J`, `0x09`, `0x0D` and a raw newline: stdout and stderr contain no ASCII control byte other than the renderer's own `0x0A`; no byte sequence from a rejected argument appears; both streams are valid UTF-8; the project's `—` and `→` characters are present on the happy path and do **not** fail the guard. Exit 1 is excluded by name, and AVP-193 asserts the exclusion is deliberate |
| AVP-188 | U | skill exit-2 wording | all six skill surfaces contain the §16.2 item 6 paragraph verbatim; none describes exit 2 from this command as an error, a failure, a blocker, or a reason to abort the workflow; a sensitivity fixture rewording it to "fails with exit 2" fails the guard |

### 18.26 Z — Rooted-boundary honesty, seams, descriptor lifecycle, platform policy and citation integrity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| AVP-189 | G | no physical-confinement over-claim, in **documents and strings** | a scan over every shipped string (help text, human report, advisory, abort message, `error:` line, all six skill surfaces) **and** over `docs/prds/PRD-artifact-validation-and-provenance.md` and `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` finds no sentence **asserting** that bytes read are physically inside the repository, that no file outside the repository can be read, that the root is a filesystem or device boundary, or that mount points / bind mounts / `/proc` / device files are prevented; every occurrence of a confinement claim is qualified as **logical**, **pathname** or **name** confinement. The guard carries a fixed list of forbidden phrasings derived from rev-2's own removed sentences, and — as with AVP-207 — matches them **only outside a withdrawal marker**, because both documents quote the removed sentences in their withdrawal, alternatives and risk sections; the shipped-string half admits no such exemption. A sensitivity fixture reinserting rev-2's "no path outside the repository is ever opened, read, or named" as a bare claim fails it, and an inverse fixture asserts the guard is not red on the documents as written |
| AVP-190 | U | leaves the root does **not** close, handled by the kind gate and the bounded read | four injected-`FileInfo` fixtures through the §7.1.1 seam: (a) a directory-shaped mount point → `not-regular` (row 7); (b) a `/proc`-shaped leaf reporting `IsRegular()` with `Size() == 0` that then streams more than `MaxArtifactBytes+1` bytes → `unstable` via row 17, with a counting reader asserting at most `MaxArtifactBytes+1` bytes requested and **no** content in output; (c) a character-device leaf → `not-regular`; (d) a bind-mounted **regular file** → classified exactly like an ordinary regular file, and the test asserts in its own name that this case is deliberately **not** distinguished (§7.4.2) |
| AVP-191 | G | platform build tags are a fail-closed allowlist | the `rootConfinementSupported = true` file's `//go:build` line is exactly `unix \|\| windows`; the `false` file's line is its exact syntactic negation `!(unix \|\| windows)`; the two sets are exhaustive and disjoint over every `GOOS` reported by `go tool dist list`; a sensitivity fixture rewriting the pair into rev-2's denylist form (`(js && wasm) \|\| plan9` / its negation) fails the guard, and so does a fixture adding a hypothetical new `GOOS` that the denylist form would have silently admitted. **rev-4 removes the "byte-identical to `$GOROOT/src/os/root_openat.go`'s tag" assertion**: the allowlist is now a strict subset of the stdlib's confined set, and AVP-208 owns the subset relation |
| AVP-192 | I | slug validation precedes the platform guard | with the confinement constant forced `false` **and** a non-canonical slug argument, the run aborts `slug-unsafe` (not `workspace-unsupported-platform`), exit 3, `slug: ""`; with the constant forced `false` and a canonical slug it aborts `workspace-unsupported-platform` with `slug` set to that canonical slug; across the whole abort matrix `slug == ""` **iff** `abort.code == "slug-unsafe"` (§10.2 rule 8) |
| AVP-193 | S+G | command-owned output scope, and the parse-error ownership mechanism | **Source half** (AST over the `prepare` command file): no `FlagErrorFunc`, no `SetFlagErrorFunc`, no `SetErr`, no `SetOut`, no custom `Args` validator that formats its own message, and no local `SilenceUsage`/`SilenceErrors` assignment — the command inherits the root's (`internal/cli/cobra.go:56-62`) and installs no interception point. **Behavior half** over the five exit-1 inputs (`--manual`, `--regenerate`, zero args, two args, `--path` with no value): exit code exactly 1; stdout empty; stderr exactly one line beginning `error: `; that line matches **none** of the §9.5 templates and contains no `abort.code`; no report on either stream; `.tpatch/` byte-identical. The guard's doc comment records that the line's text is unsanitized `spf13/pflag`/`spf13/cobra` output carried by the shared root printer, and that **no usage block is printed** because `SilenceUsage` is set — so a reader cannot mistake either fact for an oversight. **Sensitivity**: adding a `FlagErrorFunc` that rewrites pflag's message fails the source half, because installing one moves those bytes into this command's ownership without a schema decision. Separately asserted: a hostile slug with a **well-formed** flag set never reaches this population — it reaches `slug-unsafe` and is withheld (AVP-102…AVP-104) |
| AVP-194 | S+G | the `RootOps`/`FileOps` seam | exactly **two** production implementations exist — one per interface — and both are declared in `internal/intent` outside any `_test.go` file: `osRootOps` (over `*os.Root`) and `osFileOps` (over `*os.File`). Each production adapter struct has exactly one field (`root`, `file`); no production type carries a before/after hook field; `RootOps` declares exactly three methods (`Lstat`, `OpenFile`, `SameFile`) and `FileOps` exactly three (`Stat`, `Read`, `Close`), none of them taking or returning an absolute path, and none of them a mutator or an enumerator. Sensitivity: a fixture adding a third production implementation fails; so does one adding a pathname-based method to either interface; so does one returning a bare `*os.File` from `OpenFile` instead of `osFileOps`, since that would put the production `FileOps` back in a package this guard cannot scan |
| AVP-195 | U | pre- and post-capture component walks | with the §7.1.1 seam's `after` hook replacing `artifacts/` with a symlink **after** the read completes and before the post-walk: the sidecar is `unstable` (ladder row 20a), the captured bytes are discarded, and the reported state is **not** a content state even though the read succeeded and the leaf identity matched. Three companion fixtures: the component vanishes → `unstable` (not `absent`); the component becomes a regular file → `unstable` (not `unreadable`); the post-walk `Lstat` fails for another reason → `unreadable` (row 20b). A required artifact in this state forces `indeterminate`/exit 3 (§9.1) |
| AVP-196 | U | identity limits, asserted as limits | three fixtures, each asserting the **documented** outcome and each named for the limit it pins: (a) **hard-link alias** — the leaf name is swapped to a hard link of the same inode; `os.SameFile` matches, the capture proceeds, and the test asserts the report is true of the object read and that **no output string claims detection**; (b) **file-ID reuse** — an injected `FileInfo` pair with identical `Dev`/`Ino` (or `vol`/`idxhi`/`idxlo`) representing two different objects; the comparison matches and the test records that this is inherent to identity-by-number; (c) **swap-and-restore** — the seam's hooks restore the intended object before each of the six observation points; every probe passes and the test asserts the PRD documents this residue (§8.3 limit 5) rather than claiming to close it. All three are *limit* rows: none asserts a capability |
| AVP-197 | U+G | one reused scratch buffer per invocation, with a stated cost | an allocation-counting fixture over a full real run (status capture plus four artifact captures, artifacts sized 0 B, 1 B, 4 MiB−1 and 4 MiB) records **exactly one** data-buffer allocation for the whole invocation, of exactly `MaxArtifactBytes+1 == 4,194,305` bytes; the status capture is asserted to use a sub-slice of that same array (compared by backing-array pointer), not its own; the same single allocation occurs on an abort run that captures nothing; `MaxStatusBytes < MaxArtifactBytes` is asserted at compile time; a sensitivity fixture restoring rev-2's per-capture `make` fails the guard by counting five allocations |
| AVP-198 | G | `winsymlink` directive and the Windows tag mapping | source half (all targets): `cmd/tpatch`'s `package main` file contains the exact line `//go:debug winsymlink=1` immediately above `package main`, and a sensitivity fixture removing it or setting `0` fails. Mapping half (injected `FileInfo` through the seam, all targets): a symlink-tag stat → refused; a mount-point/name-surrogate stat → refused via `ModeIrregular` **with no `ModeDir` bit**; an `AF_UNIX`-tag stat → `ModeSocket`, **not** refused by the predicate but caught by `!IsRegular()` at row 7; a `DEDUP`-tag stat → **no type bit, classified as a regular file and read normally**, which the test asserts as the deliberate behavior of §7.4.3. Native half (`runtime.GOOS == "windows"`): a junction created per AVP-199 reports `ModeIrregular` and **not** `ModeSymlink`, which is the runtime observable distinguishing `winsymlink=1` from `winsymlink=0` |
| AVP-199 | I | native Windows junction fixtures fail, never skip | on `windows-latest`: the junction helper runs `cmd /c mklink /J <link> <target>` and calls `t.Fatal` — never `t.Skip` — if the command is unavailable or returns non-zero; a source-scan half asserts no `t.Skip`/`t.Skipf`/`t.SkipNow` appears in the junction-fixture helper or in any test consuming it, the only permitted guard being `runtime.GOOS != "windows"`; the created object is asserted to be a real junction (`ModeIrregular`, name-surrogate, `IsDir()` false). Sensitivity: a fixture converting the `t.Fatal` into a `t.Skip` fails the guard |
| AVP-200 | G | `O_NONBLOCK` pass-through Go-upgrade tripwire | a real `os.Root` over a real temp directory containing a real FIFO with **no writer** is opened with `os.O_RDONLY\|syscall.O_NONBLOCK` under a hard deadline (non-Windows only); the open must return — success or error — within the deadline. The test's name and comment state that it verifies an **unexported implementation detail of `os.Root`** (`rootOpenFileNolog` forwarding caller flags to `openat`, G17), not a documented contract, so a Go release that stops forwarding the flag turns this red at upgrade time rather than turning `prepare --check` into a field hang. A sensitivity fixture opening without `O_NONBLOCK` blocks and fails the deadline |
| AVP-201 | G | cap ↔ frozen-message coupling | the guard derives the human unit string from each constant (`MaxArtifactBytes` → `4 MiB`, `MaxStatusBytes` → `1 MiB`) and asserts: the derived artifact string appears in the `analysis-sidecar-oversize` advisory (§10.4) and in §15's oversize remediation, and nowhere claims a different figure; the derived status string appears in the `status-oversize` abort template (§9.4.5) and nowhere else claims a different figure; no message names a limit that is not one of the two constants. Two sensitivity fixtures fail the guard: one changing `MaxArtifactBytes` without changing its message, one changing the message without changing the constant |
| AVP-202 | G | citation-resolution and matrix-arithmetic guard | a mechanical pass over this PRD asserts: (a) every `AVP-NNN` token appearing **anywhere** in the prose resolves to a declared row in §18.2–§18.26; (b) declared IDs are contiguous `AVP-001`…`AVP-208` with zero duplicates and zero gaps; (c) §18.27's category table sums to the declared total, and each category's stated count equals the number of rows under that section heading; (d) §18.27's kind table sums to the declared total, and each kind's count equals the number of rows whose Kind column is exactly that string; (e) the guard-arithmetic predicate ("Kind contains `G`") reproduces §18.27's stated guard-row count; (f) §17's slice assignment is a partition of `1…208` — zero unassigned, zero double-assigned — and each slice's row count matches. Sensitivity fixtures, all of which must fail the guard: a prose reference to a row number beyond the declared maximum; a category count off by one; a row moved between categories without updating the table; a row assigned to two slices. **What this guard does not prove, stated in the guard's own doc comment:** it verifies that citations *resolve* and that counts are *arithmetically consistent*. It cannot verify that a cited row is the *semantically right* row for the sentence citing it — a prose sentence about the status cap citing a row about the artifact cap resolves fine and is still wrong. Semantic mapping remains a reviewer obligation, and §18.1's "a test does not satisfy a row merely by existing" rule is its analogue on the implementation side |
| AVP-203 | U | artifact descriptor close failure | with the §7.1.1 seam's `FileOps.Close` returning an injected error on a capture that reached §7.4.4 step 11a with no earlier ladder match: the artifact is `unreadable` with `reason_code: artifact-unreadable` (ladder row 20c); the captured bytes are **discarded** and no content state (`present-empty`, `present-nonempty`, `invalid-structured`) is reported even though the read succeeded; a required artifact in this state yields exit 2; the sidecar variant yields advisory `analysis-sidecar-unreadable` and leaves readiness untouched. Two ordering companions: a capture that already matched row 13 (identity) stays `unstable` when `Close` also fails — first-match-wins, 20c does not overwrite it — and a capture that already matched row 12 stays `unreadable` with the same code. The `os` error string appears on neither stream |
| AVP-204 | U | `status.json` descriptor close failure | with an injected `FileOps.Close` error on the status capture: the run aborts with `abort.code: status-unreadable` (§9.4.2 row 16a), exit 3, `artifacts: []`, `feature_state: "unknown"`; the §9.4.5 message is the "could not be read and closed cleanly" template byte-for-byte and the §10.5.1 lifecycle line is its matching annotation; **no fourteenth abort code exists** — the guard asserts the abort catalog is still exactly the thirteen of §9.4.4; no `os` error string reaches either stream |
| AVP-205 | U+G | descriptor lifecycle: exactly one close, zero leaks | a counting `RootOps`/`FileOps` pair records every `OpenFile` that returned a non-nil `FileOps` and every `Close`, over a corpus that drives **every** ladder row that can follow a successful open (12, 13, 14, 15, 16, 17, 18, 19, 20, 20a, 20b, 20c, 21, 22, 23, 24) plus the status ladder's post-open rows plus all thirteen abort paths: opens == closes, each descriptor closed exactly once, and zero descriptors outstanding when `Inspect` returns. A second half asserts the close is **unconditional** in source (an AST check that the close is not inside a success-only branch). Sensitivity: a fixture that skips the close on one early-return path fails by leaving one descriptor outstanding; a fixture that closes twice fails the exactly-once assertion |
| AVP-206 | U+G | injectable `SameFile` | identity rows are driven through `RootOps.SameFile` with plain `fs.FileInfo` doubles and a chosen verdict, with **no construction of any `os` package-private `fileStat`**: verdict `true` on an unchanged capture proceeds to the read; verdict `false` yields `unstable` at ladder row 13 with **zero bytes read** (artifact) and `status-unstable` at §9.4.2 row 9 (status). Source half: `os.SameFile` appears at **exactly one** production call site in `internal/intent`, inside `osRootOps.SameFile`, whose body is exactly `return os.SameFile(a, b)`. Sensitivity: a fixture calling `os.SameFile` directly from the capture path fails the source half; a fixture whose `SameFile` double is never consulted (the capture calling `os.SameFile` behind the seam's back) fails the behavioral half because the injected verdict has no effect |
| AVP-207 | G | no bounded-runtime over-claim, in **documents and strings** | a scan over every shipped string (help text, human report, advisory, abort message, `error:` line, all six skill surfaces) **and** over `docs/prds/PRD-artifact-validation-and-provenance.md` and `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` finds no sentence **asserting** that the command cannot hang, always terminates, has bounded or predictable runtime, has "no unbounded wait", is safe to run in a non-cancellable preflight step, or that a read cannot block. The permitted forms are bounded **allocation**, bounded **bytes requested**, a bounded **number of operations**, and a non-wedging **open** — each of which names what it bounds. **Quotation context is part of the guard, not an afterthought** (the same requirement AVP-189 carries): both documents deliberately *quote* the withdrawn phrasings in their withdrawal, alternatives and risk sections, so a naive substring scan would be permanently red and would be weakened until it proved nothing. The guard therefore matches a forbidden phrasing **only** outside a withdrawal marker — a sentence containing `withdrawn`, `rejected`, `removed`, `must not`, `may not`, `may claim`, `no document may` or `forbidden` within the same sentence is a quotation, not an assertion — and the shipped-string half admits no such exemption, because a help string has no withdrawal context. Three sensitivity fixtures fail the guard, each asserting the phrase **without** a withdrawal marker: reinserting rev-3's "the command has no unbounded wait anywhere" as a bare claim (§8.3), reinserting "so nothing hangs" (§15), and reinserting "no leaf kind can hang it" (§7.4.2). A fourth, inverse fixture asserts the guard is **not** red on the shipped documents as written, proving the quotation exemption works. This is AVP-189's timing-dimension sibling and they do not overlap: AVP-189 owns confinement claims, AVP-207 owns termination claims |
| AVP-208 | G | the platform allowlist is a justified strict subset of the stdlib's confined set | the `GOOS` set for which `rootConfinementSupported == true` is computed from the build tags and asserted to be a **subset** of the set matched by `$GOROOT/src/os/root_openat.go`'s own build tag — the design never claims confinement on a target where the standard library does not implement the confined resolver. The subset is asserted **proper**: `wasip1` is matched by the stdlib tag and **not** by ours, and the guard requires `wasip1` to resolve to `rootConfinementSupported == false` and therefore to abort `workspace-unsupported-platform` (§7.4.1, AVP-177). A companion assertion pins the reason the gap exists: `openFlags()` is declared in exactly three build-tagged files (rev-6 errata: `unix`, `windows`, `!(unix \|\| windows)`) and **none of them is a `wasip1`-specific half** — `wasip1` is served by the unsupported constant-`0` buildability half, which the platform gate makes unreachable, not by an implementation. Sensitivity: a fixture widening the `true` tag to the stdlib's exact text (re-adding `wasip1`) fails, because the subset becomes non-proper while no `wasip1` implementation half and no `wasip1` runner exist; a fixture narrowing the stdlib comparison to a hard-coded literal instead of reading the live tag also fails |

### 18.27 Count, kinds and slice partition

**Count: 208 acceptance rows**, `AVP-001`…`AVP-208`, contiguous, no duplicates,
no retired rows. Twenty-five categories:

| Cat | § | Rows | Cat | § | Rows |
|---|---|---|---|---|---|
| A | 18.2 | 10 | N | 18.15 | 5 |
| B | 18.3 | 20 | O | 18.16 | 12 |
| C | 18.4 | 8 | P | 18.17 | 10 |
| D | 18.5 | 14 | Q | 18.18 | 1 |
| E | 18.6 | 6 | R | 18.19 | 9 |
| F | 18.7 | 5 | S | 18.20 | 2 |
| G | 18.8 | 9 | T | 18.21 | 12 |
| H | 18.9 | 4 | V | 18.22 | 17 |
| I | 18.10 | 6 | W | 18.23 | 5 |
| J | 18.11 | 4 | X | 18.24 | 6 |
| K | 18.12 | 6 | Y | 18.25 | 8 |
| L | 18.13 | 3 | Z | 18.26 | 20 |
| M | 18.14 | 6 | | | |

The category letters sum to **208** (202 carried forward + 6 new `Z` rows).
`U` is deliberately skipped as a category letter so it cannot be confused with
the `U` (unit) **kind**; `G` and `S` remain as both a category letter and a
kind letter for historical reasons — they are disambiguated by column position
and were not renumbered, per §18.1's stability rule.

**By kind** (the Kind column, exactly as written in each row):

| Kind | Rows | Carries a guard component? |
|---|---|---|
| `U` | 61 | no |
| `I` | 96 | no |
| `S` | 6 | no |
| `G` | 31 | **yes** |
| `S+G` | 5 (AVP-089, AVP-116, AVP-150, AVP-193, AVP-194) | **yes** |
| `U+G` | 6 (AVP-144, AVP-146, AVP-170, AVP-197, AVP-205, AVP-206) | **yes** |
| `I+G` | 1 (AVP-140) | **yes** |
| `S+I` | 2 (AVP-128, AVP-141) | no |

61 + 96 + 6 + 31 + 5 + 6 + 1 + 2 = **208**.

**Guard arithmetic, stated once and used everywhere.** A row "carries a guard
component" **iff** its Kind string contains `G`. That is **31 pure `G` rows +
5 `S+G` + 6 `U+G` + 1 `I+G` = 43 rows**, and 43 is the number §18.28 and
AVP-139 operate over. Restated as the arithmetic a reviewer can reproduce
directly: 31 + 5 + 6 + 1 = 43, and the complement (61 `U` + 96 `I` + 6 `S` +
2 `S+I` = 165) carries no guard component; 43 + 165 = 208.

rev-1 got this wrong in two directions at once: it claimed "13 `G` rows plus 3
combined = 15" in the count paragraph while its sensitivity section listed "the
13 `G` rows plus AVP-116, AVP-128 and AVP-140" — sixteen items — and it
included **AVP-128**, whose kind is `S+I` and which contains no guard at all.
rev-2 removed AVP-128 from the guard set (it is an AST + runtime-spy row, not a
mechanical guard over a declared table), stated the rule as a predicate rather
than a hand-list, and made AVP-139 derive the set mechanically. **rev-3 added
AVP-202**, which re-derives *all* of the arithmetic above from the matrix
itself — category counts, kind counts, the guard predicate and the slice
partition — so a future rev cannot restate a stale total in prose while the
rows say otherwise. **rev-4 adds six rows and re-kinds AVP-193 from `G` to
`S+G`** (it gained an AST half), which is why `S+G` moves from 4 to 5 while
pure `G` moves from 30 to 31: it loses AVP-193 and gains AVP-207 and AVP-208,
both pure `G`.
AVP-202's own doc comment states what it cannot do: resolution and arithmetic
are mechanical, semantic correctness of a citation is not.

**Slice partition.** All 208 rows are assigned to exactly one slice in §17's
exit-criteria column: zero unassigned, zero double-assigned (S1 83, S2 28,
S3 49, S4 38, S5 10 — sum 208). AVP-139 and AVP-202 cross-check the §17 table.

### 18.28 Sensitivity requirement

Every row carrying a guard component — the **43** rows whose Kind contains `G`,
per §18.27 — is a mechanical guard, and mechanical guards can false-pass. Each
must ship with a **sensitivity regression** proving it fails on a deliberately
broken input: a synthetic extra enum value, a swapped precedence pair, an
orphan catalog code, a dropped `+1` on a read limit, a growable slice
substituted for the shared scratch buffer, a substring- rather than key-scoped
field check, an advisory row deleted from the total function, an abort code
without a message template, a lifecycle line without a population, the platform
build tags rewritten into a denylist, a reparse predicate narrowed to
`ModeSymlink` alone, a reinstated `syscall.O_NOFOLLOW`, a removed
`//go:debug winsymlink=1`, a `t.Skip` restored in the Windows junction helper,
a second production `RootOps` implementation, a cap changed without its frozen
message, a physical-confinement sentence reinserted into the PRD or ADR, a
bounded-runtime sentence reinserted into the PRD or ADR, a skipped descriptor
close on one early-return path, an identity comparison routed around the
`SameFile` seam, `wasip1` re-added to the confinement allowlist without an
implementation half, or a prose citation pointed at a row number beyond the
declared maximum.

AVP-139 is the meta-check, and it derives the guard set from the matrix rather
than from a hand-maintained list: it parses this section's rows, selects those
whose Kind contains `G`, and asserts each has a paired sensitivity fixture on
which it fails. A guard added later without a fixture fails the meta-check
automatically; a hand-list would have to be remembered.

This is the lesson recorded against `TestAcceptanceLedger_TestsExist`, which
could false-pass on a comment because it searched raw bytes
(`docs/handoff/CURRENT.md`, Post-Release Review Adjudication, F2). A guard
without a proven failure mode is not evidence.

## 19. Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Operators read `ready` as "this feature is well specified". | High | The frozen disclaimer in both surfaces (§7.8, AVP-046); state names are purely structural (§7.7); `ready` is defined operationally in §9.1. |
| R2 | The §6.2 full-bundle readiness decision is misread as "tpatch now requires exploration". | **High** | §6.2.2 answers the six concrete questions in a table; §13.3 gives the affected population its own row; six acceptance rows (AVP-070, AVP-071, AVP-072, AVP-136, AVP-137, plus the AVP-134/135 reverse call-graph guards) prove no lifecycle, routing or gate change. §16.2 item 5 keeps the skills from presenting it as a mandated step, asserted by AVP-092. |
| R3 | `prepare` collides with `apply --mode prepare` and confuses agents. | Medium | Reciprocal help text (§5.2), asserted by AVP-009/AVP-010; skill surfaces name the full invocation including `--check` (§16.2). |
| R4 | A future wave quietly reuses the inspector inside `AdvanceStateManually`, silently tightening a shipped gate. | Medium | §12's decision is written as acceptance rows AVP-064…AVP-069 that pin the *loose* behavior, plus the composite rows AVP-130…AVP-133 that exercise the real mutating path; a tightening refactor turns them red. |
| R5 | Instability detection over-promises and operators trust a torn read. | Medium | §8.3 states **six** limits — same-length in-place rewrite, same-identity alias, hard-link alias, inode/file-ID reuse, swap-and-restore, and the held-descriptor tautology — and §7.4.4 weakens the promise to "an object **observed as different** is never read", adding that bytes read through an unobserved consistent alias are **attributed to the canonical artifact name**; AVP-152 is a mechanical over-claim guard over every shipped string and AVP-196 pins the three alias/reuse limits as *limits*, not capabilities; the post-capture ancestor walk (row 20a, AVP-195) narrows the window without claiming to close it; no retry/lock is added. |
| R6 | Harness authors expect stderr to be empty on `--json --quiet` and break on the `error:` line. | Medium | §10.1 tables the composed behavior for every exit × flag combination; AVP-096…AVP-101 assert it, including the exit-0 case where stderr *is* empty. The `error:` line is always last and always exactly one line. |
| R7 | The Windows capture contract diverges from the Unix one and quietly degrades, and no runner would notice. | **High** | rev-1's severity was too low: CI is Linux + macOS only today (`.github/workflows/ci.yml:25`), so the entire Windows half was unexecuted. Five mitigations, all mandatory: (a) the contract is expressed through `os.Root` rather than a hand-rolled `CreateFile`, so both targets share one code path and one set of ladder rows (§7.4.3); (b) `windows-latest` is added to the CI matrix **in S1**, as an acceptance obligation, not a nicety (§16.1, §17, AVP-175); (c) AVP-176 exercises symlink, junction, reparse-`status.json`, handle identity and `FILE_TYPE_CHAR` natively, and AVP-178 keeps all three targets building; (d) the reparse-tag mapping is corrected and pinned, including the AF_UNIX and DEDUP exceptions rev-2 got wrong, with `//go:debug winsymlink=1` in `package main` (AVP-198); (e) **junction fixtures must fail, not skip** — `cmd /c mklink /J` with `t.Fatal` on unavailability, and a source guard forbidding `t.Skip` in the fixture path (AVP-199), because a skipped Windows test is indistinguishable from a passing one in a green log. |
| R8 | `oversize` makes a legitimate large artifact unusable. | Low | 4 MiB is ~1000× any real intent artifact; the state is reported honestly with a manual-inspection remediation rather than a silent truncation. `MaxStatusBytes` is separate (§9.4.2) so widening one cannot silently widen the other — though §7.4.5's shared buffer does make `MaxStatusBytes < MaxArtifactBytes` a structural invariant, asserted at compile time. Changing either constant without changing its frozen message fails AVP-201. |
| R9 | Schema churn breaks harness consumers. | Low | §10.2's versioning rule; the two-shape `artifacts` contract with an exact `iff` discriminator; §11.1's stable `unknown` keeps a future provenance value additive. |
| R10 | The four-artifact set is wrong for some workflow. | Low | The set is closed for v1 and additive later; §6.1 states the exclusion rationale rather than leaving it implicit. |
| R11 | Skill-surface edits drift across the six formats. | Low | The existing parity guard is the mechanism; AVP-090…AVP-092 and AVP-188 extend it in the same commit. |
| R12 | An agent treats exit `2` as a tool failure and aborts or retries the workflow. | Medium | §16.2 item 6 fixes verbatim skill wording that names exit 2 an expected report outcome and forbids calling it an error or a blocker; AVP-188 asserts it in all six surfaces with a sensitivity fixture on the word "fails". |
| R13 | A future refactor reintroduces an unrooted `os` read — the exact defect rev-1 shipped by delegating `status.json` to `store.LoadFeatureStatus`. | **High** | AVP-089, AVP-150 and AVP-194 are AST guards over both the inspector and the command file, each with a sensitivity fixture that reintroduces one forbidden call or a second production `RootOps` implementation; §7.1.1, §7.1.2 and §9.4.1 state the rule as a package invariant rather than a convention. |
| R14 | A reader takes `os.Root` for a **physical** boundary and assumes the command cannot read a device file, a bind mount or a `/proc` entry — then builds a security argument on it. | **High** | §7.4.2 quotes the `Root` doc comment's own "do not prohibit traversal of filesystem boundaries, Linux bind mounts, /proc special files, or access to Unix device files" sentence and tables what actually closes each leaf; §14.1 names the four boundaries; ADR-034 D2 locks the scope; AVP-189 is a mechanical over-claim guard over the shipped strings **and** over this PRD and ADR-034; AVP-190 exercises the four leaf shapes. |
| R15 | A Go toolchain upgrade silently removes a behavior this design depends on but the standard library never promised — `O_NONBLOCK` forwarding, handle-derived Windows identity, or the reparse-tag mapping. | Medium | §23.2 classifies every stdlib claim as **contract** or **tripwire**, and every tripwire names a runtime acceptance row: AVP-200 (FIFO open deadline) for `O_NONBLOCK`, AVP-176 (native identity) for Windows `SameFile`, AVP-198 (tag mapping + `winsymlink` directive) for the mode mapping. A red test at upgrade time is the designed failure mode; a wedged open or a silently weakened identity check is not. |
| R16 | The `RootOps` seam becomes a general-purpose injection point and a future change routes a real read — or a real identity comparison — through a non-adapter implementation. | Medium | §7.1.1 rule 1 permits exactly **two** production implementations, one per interface, and AVP-194 asserts it by AST with a sensitivity fixture; the interfaces are three + three methods wide with no pathname-taking method and no mutator; AVP-206 pins `os.SameFile` to a single production call site; AVP-089 and AVP-150 continue to scan the whole package including test files for pathname readers. |
| R17 | The fixed ~4 MiB scratch allocation is read as "the command allocates proportionally to the data" and someone later multiplies it back out per capture. | Low | §7.4.5 states the cost as a flat 4,194,305 bytes per invocation, including on abort runs, plus one zeroing pass (G22); AVP-197 counts exactly one allocation for a whole run and its sensitivity fixture fails at five when the per-capture form is restored; Q9 records the lazy-allocation alternative rather than leaving it unconsidered. |
| R18 | An operator or an agent reads the command as "cannot hang" — rev-3 said exactly that — and wires it into a blocking, non-cancellable harness or CI preflight step; a stalled NFS/SMB mount, a wedged FUSE server or an unresponsive device then hangs the pipeline. | **High** | rev-4 **withdraws every bounded-runtime claim**: §7.4.2, §7.4.3, §8.3, §14.1, §15 and §5.1 now say what is bounded (allocation, bytes requested, operation count, the Unix open) and state plainly that an ordinary read can block indefinitely with no timeout and no cancellation in v1. §14.1 boundary 4 records that availability is out of scope. AVP-207 is a mechanical over-claim guard over every shipped string, this PRD and ADR-034, with three sensitivity fixtures drawn from rev-3's own removed sentences; §16.2 item 7 forbids any skill surface from making a timing promise; Q11 records adding a deadline or cancellable context as an additive later decision. |
| R19 | A descriptor is leaked on one of the fifteen post-open ladder paths, and a harness that invokes the command in a loop exhausts its descriptor table. | Medium | §7.4.4 step 11a makes the close unconditional and exactly-once; §7.5 row 20c makes its failure a classified outcome rather than a discarded error; AVP-205 counts opens against closes over every post-open ladder row, every status post-open row and all thirteen aborts, asserts zero outstanding descriptors when `Inspect` returns, and ships two sensitivity fixtures (a skipped close and a double close). |

## 20. Relationship to `PRD-prepare-intent-bundle` (unblocked for planning)

This PRD is now accepted, so `PRD-prepare-intent-bundle.md` is unblocked for
planning. That document remains undrafted at this acceptance point, and no
mutating implementation is authorized. Nothing in this PRD's future shipped
output — including the §5.3 refusal — may reference the planning document, for
the reason given in §5.3.

The prepare-intent-bundle PRD must, at minimum, address what this PRD
deliberately does not:

1. **Atomic publication.** WP-005 Turn 3 fixes the unit as the three canonical
   Markdown files **plus** structured sidecars **plus** the final `status.json`
   transition
   (`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`). The
   current Path A phase functions write artifacts and state incrementally
   (`internal/workflow/workflow.go:89-105,151-155,196-200`), so `prepare`
   cannot call them in sequence and claim atomicity. §6.2's readiness set is
   the read-only half of exactly that unit, which is why the two contracts
   agree on scope.
2. **Non-destructive overwrite** and a `--regenerate` policy that cannot lose
   hand-authored content.
3. **Provenance**, which requires §11.4's ADR first if the bundle intends to
   claim any artifact's source.
4. **Partial-failure exposure**: a provider failure must leave either the
   complete prior set or the complete new set
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:77-83`).
5. **Whether readiness classification becomes a gate** — a routing behavior
   delta that §12.4's rules govern.

This PRD gives that work a truthful input. It does not authorize it.

## 21. Open questions

Only genuinely unresolved items are listed. Everything else in this document is
a decision.

| # | Question | Why it is open | Default if unanswered |
|---|---|---|---|
| Q1 | Should `--all` (inspect every feature) exist in a later slice? | It is useful for harnesses but multiplies output-shape and ordering questions, and `verify --all` shows that sweep semantics deserve their own design pass. | Not in v1; a later PRD may add it additively. |
| Q2 | Should exit code `4` be reused by other future reserved-surface refusals, or stay local to `prepare`? | `SPEC.md:135-141` establishes exit codes as per-command contracts, so a cross-command meaning would be a new convention. | Local to `prepare`; a cross-command convention needs its own decision. |
| Q3 | Is `MaxArtifactBytes = 4 MiB` right? | No empirical distribution of intent-artifact sizes has been gathered. `rescap` chose 5 MiB for a different domain (`internal/rescap/content.go:29-32`), so there is no single house value to inherit. | 4 MiB; changing it later moves only the `oversize`/`unstable` boundary and is additive. |
| Q4 | Should `request.md` be reported as a fifth (optional) row? | It is an input rather than a phase output (`docs/feature-layout.md:10-32`), but its absence does make every downstream artifact suspect. | Excluded from v1; additive later. |
| Q5 | Should the Windows reserved-device-name refusal (§7.2) be platform-conditional rather than universal? | A universal refusal makes one slug name unusable on Linux for a Windows-only reason; a conditional one makes the same slug behave differently per platform, which is worse for a deterministic contract. | Universal, for determinism; a later PRD may narrow it with an enumerated delta. |
| Q6 | Should `--format text\|json` be offered as an alias for `--json`, matching `next`'s flag shape (`internal/cli/phase2.go:360-407`)? | The repo has two conventions: `verify` uses `--json`/`--quiet`, `next` uses `--format`. This PRD follows `verify` because the output is a report, not a task envelope. | `--json`/`--quiet` only; an alias is additive. |
| Q7 | Is `MaxStatusBytes = 1 MiB` right, and should it stay separate from `MaxArtifactBytes`? | `status.json` is machine-written with a fixed field set (`internal/store/types.go:215,234,236-265`), so 1 MiB is ~three orders of magnitude above anything `SaveFeatureStatus` can produce — but no distribution has been measured, and a single shared cap would be one fewer constant. | Separate, at 1 MiB. Sharing the cap would mean a future widening of the artifact cap (Q3) silently widened the metadata cap, which is the coupling §9.4.2 exists to avoid. Changing the value later moves only the `status-oversize`/`status-unstable` boundary and is additive. |
| Q8 | Should `workspace-unsupported-platform` be a **compile-time** refusal instead of a runtime abort? | A build constraint would make the refusal impossible to forget, but it would break `go build ./...` for anyone cross-compiling the whole module to a non-allowlisted target, which the repo does not currently forbid. | Runtime abort (§7.4.1). It is total, testable (AVP-179) and does not change what the module can be built for. With the fail-closed allowlist the runtime abort covers *unknown future* targets and `wasip1` alike, which was Q8's real worry. A later PRD may tighten it with an enumerated delta. |
| Q9 | Should the `MaxArtifactBytes+1` scratch buffer be allocated **lazily**, on the first capture that actually opens a file, instead of unconditionally per invocation? | §7.4.5's flat ~4 MiB cost is paid by every invocation, including aborts that capture nothing and the common case of four small Markdown files. Lazy allocation would remove that, at the price of a nilable buffer parameter and a branch on every capture — i.e. exactly the kind of conditional the fixed-buffer design exists to eliminate. | Unconditional. The cost is stated honestly (§7.4.5) rather than hidden, it is bounded and predictable, and AVP-197 asserts it as one allocation. A later PRD may make it lazy with an enumerated delta and a revised AVP-197. |
| Q10 | Should the `RootOps`/`FileOps` seam (§7.1.1) be unexported (`rootOps`/`fileOps`) rather than exported within `internal/intent`? | Exported names read better in the `Inspect` signature and in test files, but they widen the apparent API of a package whose whole point is a narrow surface. The package is `internal/`, so neither choice is reachable from outside the module. | Exported within the package, as written. AVP-134's reverse call-graph guard already limits the importer set to the `prepare` command file, which is the property that actually matters. Renaming to unexported is a mechanical change a reviewer may request without reopening any decision. |
| Q11 | **(new in rev-4)** Should v1 bound the command's wall-clock — a `--timeout` flag, a hard-coded deadline, or a cancellable `context.Context` threaded through `Inspect`? | rev-4 withdrew every bounded-runtime claim (§7.4.2, §8.3, §14.1), which makes the *absence* of a deadline a stated limit rather than an unnoticed one. Adding one is not free: Go's `os` file reads are not context-cancellable, so a real deadline means either a watchdog goroutine that abandons a blocked read (leaking the goroutine and the descriptor, in direct tension with AVP-205's zero-leak assertion) or `SetReadDeadline`, which is not supported for ordinary files. A `--timeout` that cannot actually interrupt the read would be the same class of false affordance as rev-2's inert `O_NOFOLLOW` (§7.4.3). | **No deadline and no cancellation in v1**, stated as a limit rather than implied. The command's guarantees are bounded allocation, bounded bytes requested, a bounded operation count and a non-wedging Unix open; termination is not among them (AVP-207). A later PRD may add cancellation with an enumerated behavior delta, a defined exit code for the timed-out population, and a resolution of the abandoned-descriptor question. |

## 22. Alternatives considered

| Alternative | Why not |
|---|---|
| **Keep rev-0's two-artifact required set** (`analysis.md` + `spec.md` only, `exploration.md` optional-with-advisory) | It made `prepare`'s bundle-completeness verdict silent about a third of the bundle WP-005 Agreed item 7 defines (`docs/whitepapers/WP-005-spec-driven-workflows.md:79-83`), and it could return `ready` for a feature that `verify` would later block on `intent_files_present` (`internal/workflow/verify.go:413-439`). An advisory that does not affect the verdict is exactly the "best effort" shape §11 rejects elsewhere. §6.2.2 shows the WP-005 "must not make exploration mandatory" constraint is satisfied without weakening the verdict. |
| **Extend `verify` with a pre-apply mode** instead of a new command | `verify` refuses pre-apply states by design (`internal/workflow/verify.go:245-252`), persists a record by default (`internal/workflow/verify.go:347-352`), and has a distinct 11-check contract and audience. Bending it to serve the pre-`defined` window would change a shipped surface's meaning for existing users. |
| **Add a `doctor` check (D9)** | `doctor` is workspace-wide, `--fix`-capable, and finding-shaped. A per-feature readiness verdict with its own exit-code contract does not fit that report model, and making `doctor` artifact-aware would expand a writer's surface. |
| **Extend `next` to report artifact structure** | `next` answers "what runs next" and is consumed by harnesses as a task envelope. Adding classification would change a shipped output shape and couple routing to classification — exactly what WP-005 Agreed item 9 forbids for slice 1. |
| **Make the check mutating from day one** (adopt artifacts, advance state) | The core WP-005 finding: validation precedes orchestration (`docs/whitepapers/WP-005-spec-driven-workflows.md:59-64`). A mutating command built on today's validation would amplify §1.1–§1.3. |
| **Introduce a `prepared` lifecycle state** | Explicitly rejected by WP-005 Agreed item 6 (`docs/whitepapers/WP-005-spec-driven-workflows.md:73-76`). Completeness is an artifact-level fact; adding a state would force every consumer of `FeatureState` to learn it. |
| **Abort when `status.json` is absent** (rev-0 behavior) | It refused to describe the population an inspection command most needs to describe, and asserted a precondition no artifact classification depends on. `ListFeatures` already treats such a directory as ordinary-and-skippable rather than corrupt (`internal/store/store.go:209-227`). §9.4 continues with `feature_state: "unknown"` instead. |
| **Emit four all-`absent` artifact rows on the abort path** (rev-0 behavior) | It claimed an inspection that never happened — a false statement of exactly the class this PRD exists to remove. §10.2 rule 1 uses an empty collection with an exact `iff` discriminator instead. |
| **`Lstat` then an ordinary `os.Open`** (rev-0 behavior) | Follows a symlink that appears after the `Lstat`, and its **open** of a writer-less FIFO blocks. §7.4 replaces it with a rooted, non-blocking open plus post-open identity and kind rechecks. rev-0 and rev-1 proposed reusing `internal/rescap/pathopen_unix.go:20-28`; rev-2 rejected that (its Windows sibling is a stub, C60) and rev-3 additionally drops the caller-supplied `O_NOFOLLOW` that seam is built around, because `os.Root` consumes it (§7.4.3). Note the scope: the replacement bounds the *open*, not the read (§7.4.2). |
| **Bound the read with a pre-read size check only** | A file that grows between the `Lstat` and the read bypasses it, so the inspector could allocate without bound. `internal/rescap/content.go:9-11` states the same reasoning for the same hazard. §7.4.5 uses one preallocated `MaxArtifactBytes+1` buffer. |
| **`io.ReadAll(io.LimitReader(f, Max+1))`** (rev-1 behavior) | Rejected on **cost shape**, not on boundedness. rev-1 claimed an exact allocation ceiling the mechanism does not provide; rev-2/rev-3 over-corrected into "the allocation is not bounded", which is equally wrong. The limit reader caps the result at `Max+1` bytes, so total allocation is `O(Max)` — bounded. What it is not is *fixed*: `io.ReadAll` grows by `append`, so one capture performs a sequence of increasing allocations with copies, and the sequence is paid again on each of the five sequential captures. §7.4.5 replaces it with one preallocated `Max+1` buffer and `io.ReadFull`, which caps and flattens the cost; AVP-172's sensitivity fixture fails if the `ReadAll` form is reintroduced, and it tests the *mechanism*, not a boundedness defect (G20, G21). |
| **Pathname resolution with `os.Lstat` component walks and `openNoFollow`** (rev-1 behavior) | Every check and the open resolve the *name* independently, so the ancestor chain is re-resolved by the kernel at open time with whatever the tree looks like then. Its Windows half was worse than unimplemented: rev-1 cited `internal/rescap/pathopen_windows.go:12-20` as a reusable seam when that file is an explicitly unsupported compile-only stub whose `isSymlinkLoopError` "always reports false on this target". §7.3 replaces the whole model with one held `*os.Root`, which confines resolution by construction on every shipped target. |
| **A raw `syscall.CreateFile` Windows open with `FILE_FLAG_OPEN_REPARSE_POINT`** (rev-1 behavior) | Self-contradictory as specified: the flag makes the open **succeed** on a reparse point and return a reparse-point handle, while rev-1's ladder classified the same case from an open **error**. It also duplicated, less carefully, what `os.Root`'s Windows implementation already does. §7.4.3 refuses reparse points before the open, from `Root.Lstat`'s handle-derived mode, and needs no raw syscall (AVP-180). |
| **`os.SameFile` over two pathname `os.Lstat` results on Windows** (rev-1 behavior) | On a pathname-derived `fileStat` Windows must re-open by name to fetch the volume serial and file index, which reintroduces exactly the TOCTOU the identity check exists to close. `Root.Lstat` and `File.Stat` both return handle-derived `fileStat`s with the `path` field cleared precisely so `os.SameFile` will not re-fetch (`$GOROOT/src/os/types_windows.go`), so §7.4.4 compares handle-derived identity only. |
| **Reading `status.json` through `store.LoadFeatureStatus`** (rev-1 behavior) | It is `os.ReadFile` on an absolute pathname (`internal/store/store.go:351-361`): it follows symlinks, is unbounded, checks no kind and no identity, and resolves outside the rooted namespace — leaving the one read whose bytes reach output as the only read not covered by §7.4. §9.4 gives it the full discipline and its own cap, and AVP-150 makes the old call a guard failure. |
| **Grouping the seven `status-*` aborts into one `status-broken` code** | It would reproduce, at the abort layer, the "one bucket, several different truths" shape §1 documents at the gate layer: a symlink, an oversize file, a torn read and an unknown lifecycle value need four different remediations. §9.4.4 keeps them distinct; §9.4.5 gives each its own message. |
| **Asserting output is "printable ASCII"** (rev-1 behavior) | Wrong on the command's own happy path: the `—` in the `--quiet` line and the `→` remediation marker are required, deliberate, non-ASCII house style. §14.3 states the property that actually matters — no control bytes and no attacker-argument bytes — and AVP-187 asserts it while letting the house style through. |
| **Keeping `safety.EnsureSafeRepoPath` as a "defence in depth" pre-filter** (rev-2 behavior) | It is a lexical test defined against an absolute repository *prefix*, applied to a *root-relative* name in a design whose containment primitive is a *handle*. With fixed constants and a canonical slug it can never fire, so it proved nothing; and under `--path <dir>` from an unrelated cwd the prefix it wants to test against is not a value the inspector holds. Worse, keeping it implied a containment contribution it does not make. §7.3 step 2 replaces it with an `fs.ValidPath` assertion — the property that actually constrains a `Root` method argument — and AVP-089 fails if it reappears in `internal/intent`. |
| **A denylist of unconfined platforms** (`(js && wasm) \|\| plan9` → `false`, its negation → `true`) (rev-2 behavior) | **Fail-open.** It encodes the claim that today's two unconfined targets are the only ones there will ever be. A new `GOOS`, or an existing one Go later reclassifies, lands silently in the `true` branch and the command asserts a confinement guarantee nobody checked. §7.4.1 inverts it into an allowlist, and AVP-191's sensitivity fixture fails if the denylist form returns. |
| **Including `wasip1` in the confinement allowlist** (rev-3 behavior) | Rejected in rev-4, and locked out by ADR-034 D5. rev-3 admitted `wasip1` for one reason — textual identity with `$GOROOT/src/os/root_openat.go`'s build tag — which is a property of the *stdlib's* implementation set, not of *this design's*. This design declares exactly three `openFlags()` halves (rev-6 errata: `unix`, `windows`, `!(unix \|\| windows)`, the third being an unreachable buildability constant), and its Unix half returns `syscall.O_NONBLOCK` with FIFO semantics (AVP-107, AVP-200) that a WASI preview-1 host does not provide; no `wasip1` runner, fixture or cross-build is proposed, which would reproduce R7's unexecuted-platform defect one target over. Adding a third implementation half is a separate slice with its own review, and **rev-4 does not split the implementation** — `wasip1` simply lands in the `false` branch and aborts. The asserted property becomes a **subset** relation (AVP-208) rather than textual identity. |
| **Claiming the command has "no unbounded wait anywhere"** (rev-3 behavior) | Rejected as false. `O_NONBLOCK` bounds the FIFO/device **open**, not any read; the Windows kind refusal handles stable non-regular kinds, not a slow regular file; and neither addresses a regular file served by a stalled NFS/SMB mount, a wedged FUSE server, a `/proc`-style provider or an unresponsive device driver, on which `read(2)` and even `Lstat` can block indefinitely. §7.4.2, §7.4.3, §8.3, §14.1 and §15 withdraw the claim in every place rev-3 made it; the design guarantees bounded allocation, bounded bytes requested, a bounded operation count and a non-wedging Unix open. AVP-207 is the mechanical guard with three sensitivity fixtures drawn from rev-3's own sentences, and Q11 records the additive alternative. |
| **A fourteenth abort code for a `status.json` close failure** (`status-close-failed`) | Rejected. The catalog is closed for a reason (§9.4.4), and a code earns its place by changing the remediation. A close failure and a read failure share one remediation — check permissions and the filesystem, then run `tpatch doctor` — so a separate code would fragment the catalog, add a fourteenth `--quiet` token, a fourteenth `error:` template and a fourteenth lifecycle line, and break the arithmetic AVP-095/AVP-101/AVP-153/AVP-181 assert, all to name a syscall the operator cannot act on differently. §9.4.2 row 16a routes it to `status-unreadable`, whose message and lifecycle line rev-4 widens to "could not be read and closed cleanly" so the annotation stays true of all six of its rows. |
| **Discarding the descriptor `Close` error** (rev-3 behavior, by omission) | Rejected. rev-3 declared `Close` on `FileOps` and then never called it in the capture sequence or the ladders, which left two defects: an unspecified descriptor lifetime (R19's leak) and a silently discarded error on a capture that otherwise reported a content state. §7.4.4 step 11a makes the close unconditional and exactly-once, §7.5 row 20c and §9.4.2 row 16a classify its failure before any content interpretation, and AVP-203…AVP-205 assert the outcome, the ordering and the zero-leak property. |
| **Intercepting cobra's parse error inside `RunE` to re-render it** | Rejected, and unimplementable as rev-3 specified it. A parse error is raised **before** `RunE`, so no `RunE` body can intercept it — rev-3's AVP-193 sensitivity fixture could not have been written. The real interception point is a `FlagErrorFunc` on the command, and installing one would move pflag's bytes into this command's ownership without a schema decision, so §14.3.1 forbids it and AVP-193's source half asserts its absence. The exit-1 text stays third-party and unsanitized, carried by the shared root printer (`internal/cli/cobra.go:33-39`), and the PRD says so rather than implying a sanitisation it does not perform. |
| **Passing `syscall.O_NOFOLLOW` from the caller "belt-and-braces"** (rev-2 behavior) | `Root.OpenFile` already ORs it internally and then converts the resulting signal into an in-root symlink *resolution* rather than a refusal (G10). rev-2 documented that and passed the flag anyway. A flag whose own specification says it produces no observable effect is not defence in depth — it is a false affordance that invites the next reader to infer a no-follow guarantee. rev-3 removes it and keeps the explanation; AVP-118's sensitivity fixture fails if it returns. |
| **Asserting the command's byte rules over *all* process output** (rev-2 behavior) | rev-2's "no byte of stdout or stderr is an ASCII control character" swept in cobra's and pflag's own parse diagnostics, which are produced before `RunE` from raw `os.Args` and interpolate the offending argument verbatim. The claim was therefore false for the exit-1 population and, worse, was a security assertion about third-party code this PRD does not modify. §14.3.1 scopes every byte guarantee to command-owned output after successful parsing, and AVP-193 asserts the exclusion is deliberate rather than forgotten. |
| **One `MaxArtifactBytes+1` buffer per capture** (rev-2 behavior) | Correct on allocation *shape* and wrong on allocation *total*: five captures per run means a ~20 MiB worst case for a command that answers a five-line question, and rev-2 stated the per-capture ceiling without ever totalling it. §7.4.5 allocates one buffer per invocation and reuses it sequentially (captures are strictly sequential, §8.4), which keeps every property rev-2 bought and states the flat cost honestly. AVP-197's sensitivity fixture counts five allocations when the per-capture form returns. |
| **Claiming "a different object is never read"** (rev-2 behavior) | Unprovable as stated. `os.SameFile` compares identity *numbers*, so a hard-link alias, an inode/file-ID reuse and a swap-and-restore between probes all defeat it — and the first of those is undetectable by construction, not by omission. §7.4.4 weakens the promise to "an object **observed as different** is never read" and §8.3 enumerates six limits; AVP-196 pins three of them as *limits*. |
| **Walking the ancestor components once, before the open only** (rev-2 behavior) | The leaf identity comparison proves the *leaf* descriptor is the object the leaf `Lstat` saw; it proves nothing about the ancestor chain at two different instants. §7.4.4 step 11 adds a post-capture walk and ladder rows 20a/20b, so an ancestor observed to have changed makes the result `unstable` rather than yielding a content state derived from a chain that moved. It narrows the window; §8.3 limit 5 says plainly that it does not close it. |
| **Testing `Inspect` against a concrete `*os.Root`** (rev-2 behavior) | It left roughly two dozen acceptance rows — every injected `fstat` failure, every injected read error, every injected close error, every chosen identity verdict, every deterministic before/after race hook — unimplementable except through a real hostile filesystem or a production-visible package variable. §7.1.1 declares a two-interface, six-method seam with exactly one production implementation per interface, and AVP-194 makes "exactly two, one each" a mechanical, sensitivity-tested property rather than a convention. |
| **Calling `os.SameFile` directly from the capture path** (rev-3 behavior) | Rejected in rev-4. `os.SameFile` is only meaningful over the unexported `*os.fileStat` values `os` produces, and no test outside `os` can construct one — so every row needing a *chosen* identity verdict (AVP-084, AVP-151, AVP-160, AVP-196 (b)) was unwritable except against a real hostile filesystem. That is the same defect rev-3 fixed for `Lstat`/`OpenFile` and left in place one line later. §7.1.1 moves the comparison behind `RootOps.SameFile`, whose one production body is `return os.SameFile(a, b)`; AVP-206 asserts the single production call site and the injectable verdict. |
| **Migrating `rescap.GatePath` onto `os.Root` in this slice** | Rejected, and locked out by ADR-034 D11. `rescap` is a *capture* domain with a lock/scratch lifecycle, a missing-path-is-a-refusal policy and a different refusal vocabulary; converting it is a behavior change to a shipped surface with its own PRD lineage (ADR-033). This PRD reuses `rescap`'s ancestor-walk **policy** and cites its cap-plus-one **reasoning**; it does not touch its code, and a wave that "helpfully" migrates it has exceeded this PRD's scope. |
| **Infer provenance heuristically and label it "best effort"** | A best-effort provenance field is worse than none: consumers use it, and §11.2 shows every signal is overwritten or forgeable. `unknown`, with §11.1's stable meaning, is the only truthful v1 answer. |
| **Emit content hashes for stability comparison across runs** | Rejected on privacy grounds (§14.2) and because readiness needs no hash. |
| **Take a workspace lock for a coherent multi-artifact snapshot** | Makes a read-only command a lock acquirer, can block on a stuck writer, and still cannot prove atomicity for files written by a truncating writer. §8.4 chooses honest non-atomicity instead. |
| **Echo the offending slug in the `slug-unsafe` diagnostic** | Reflects attacker-controlled bytes — including control characters and terminal escapes — into a log or terminal, and would leak an absolute path when the argument is one. §7.2 withholds it and points at `tpatch status` instead. |

## 23. Claims-audit appendix

Every load-bearing claim about current behavior, anchored. Claims split into
two tables:

- **§23.1 — repository claims (C1…C95)**, anchored as `file:line`. All
  citations verified against HEAD `be33d2a`, whose only difference from
  `WAVE_BASE` `0aa0d95` is tracking-doc, PRD and ADR commits; no source file
  cited below differs between the two.
- **§23.2 — Go standard library claims (G1…G22, plus `G5a` and `G9a`)**,
  anchored by **symbol** in
  `$GOROOT/src/...` rather than by line, and **classified** as either a public
  **contract** or a version-pinned implementation **tripwire**. Line numbers in
  the toolchain are not stable across Go patch releases, and a stale line
  anchor is worse than a symbol name that a reviewer can `grep`. Every G-claim
  is additionally backed by a runtime acceptance row, so the design does not
  rest on the citation alone. Verified against the toolchain this repository
  pins — `go.mod:3` declares `go 1.26.1`; the claims were read from a
  `go1.26.5` `GOROOT`.

### 23.1 Repository claims

| # | Claim | Anchor |
|---|---|---|
| C1 | `manualPhaseMap` is the single source of truth for `--manual` and maps analyze/define/explore/implement to `analysis.md`/`spec.md`/`exploration.md`/`artifacts/apply-recipe.json`. | `internal/store/manual.go:25-32` |
| C2 | `ValidateJSON` is set on the `implement` row only, so only that phase's artifact is content-validated. | `internal/store/manual.go:31,67-78` |
| C3 | The `implement` branch rejects whitespace-only bytes *and* invalid JSON; the three Markdown phases have no content check at all. | `internal/store/manual.go:67-78` |
| C4 | `AdvanceStateManually` checks existence and not-a-directory, and otherwise advances state; whitespace-only Markdown passes. | `internal/store/manual.go:51-81` |
| C5 | That check uses `os.Stat`, which follows symlinks. | `internal/store/manual.go:57` |
| C6 | `--manual` writes a fixed notes string recording the manual transition. | `internal/store/manual.go:79` |
| C7 | `ManualPhase` exposes the per-phase contract for reuse. | `internal/store/manual.go:34-39` |
| C8 | `MarkFeatureState` overwrites the single `Notes` field on every transition. | `internal/store/store.go:379-393` (assignment at `388-392`) |
| C9 | `FeatureStatus.Notes` is one optional free-text string. | `internal/store/types.go:215` |
| C10 | `SaveFeatureStatus` also refreshes `FEATURES.md`, so one status write mutates two tracked files. | `internal/store/store.go:363-377` |
| C11 | `LoadFeatureStatus` is `os.ReadFile` on an absolute pathname followed by `json.Unmarshal`: it follows symlinks, is unbounded, checks no file kind and no descriptor identity, and resolves outside any rooted namespace. Its read error and its unmarshal error are distinguishable, which is what makes `status-unreadable` and `status-malformed` separable — but §9.4.1 forbids the function itself. | `internal/store/store.go:351-361` |
| C12 | `store.Open` refuses when `.tpatch/` is absent — superseded for this command by C76/C78; retained because §22 weighs it. | `internal/store/store.go:133-144` |
| C13 | Feature paths are `<root>/.tpatch/features/<slug>/...`, via unexported helpers — which is why `internal/intent` must declare its own root-relative constants and guard the parity (AVP-088). | `internal/store/store.go:779-797` |
| C14 | `WriteFeatureFile` and `WriteArtifact` are the artifact writers; both funnel to `writeFile`. | `internal/store/store.go:443-449,461-472` |
| C15 | `writeFile` is `os.WriteFile`, which truncates in place — the concurrency hazard §8 addresses. | `internal/store/store.go:918-923` |
| C16 | `ListFeatures` enumerates feature directories, skips those without a valid `status.json`, and applies **no canonicality filter to the directory name** — so a directory without a status file is an ordinary shape rather than corruption (§9.4.3), and `tpatch status` will print a non-canonical name back at an operator, which is why the `slug-unsafe` remediation must not point there (§7.2). | `internal/store/store.go:208-227` |
| C17 | `AddFeature` derives the slug through `Slugify` and refuses when the result is empty, so every created feature directory carries a canonical slug. | `internal/store/store.go:157-163` |
| C18 | `Slugify` lowercases, replaces every non-`[a-z0-9-]` byte with a dash, collapses runs of dashes and trims leading/trailing dashes. | `internal/store/slug.go:9-10,20-42` |
| C19 | `Slugify` caps the result at 60 bytes, preferring a dash boundary. | `internal/store/slug.go:12,44-51` |
| C20 | Path A `RunAnalysis` writes `artifacts/analysis.json` **and** `analysis.md`, then marks `analyzed`. | `internal/workflow/workflow.go:89-105` |
| C21 | `RunDefine` reads the sidecar opportunistically and ignores its absence. | `internal/workflow/workflow.go:117-121` |
| C22 | `RunDefine` writes `spec.md` and marks `defined` with a fixed notes string that overwrites any prior note. | `internal/workflow/workflow.go:151-155` |
| C23 | `RunExplore` writes `exploration.md` and also marks `defined`, so exploration is not required to reach `defined`. | `internal/workflow/workflow.go:196-200` |
| C24 | Heuristic fallbacks exist and produce templated content, so content markers cannot attest authorship. | `internal/workflow/workflow.go:203-259` |
| C25 | `JSONObjectValidator` is the existing "must be a JSON object" primitive, used by the analyze phase. | `internal/workflow/retry.go:145-157`; wired at `internal/workflow/workflow.go:54,62` |
| C26 | The `FeatureState` enum has twelve values, and `ValidFeatureState` is its closed validity switch — the predicate §9.4.2 row 18 mirrors and AVP-165 guards for parity. | `internal/store/types.go:8-37,39-46` |
| C27 | `Verify` and `Rejection` are `omitempty` sub-records on `FeatureStatus` — the P1 precedent. | `internal/store/types.go:236-251,253-265` |
| C28 | `DependsOn`'s doc comment states the `omitempty` byte-identity migration contract. **rev-2 correction**: rev-0/rev-1 anchored this at `:207-215`, which is the plain field block, not the comment. | `internal/store/types.go:219-234` |
| C29 | `cycle` runs analyze → define → explore → implement → apply → record with provider calls. | `internal/cli/phase2.go:25-145` |
| C30 | `cycle --skip-execute` stops after recipe generation. | `internal/cli/phase2.go:122-127` |
| C31 | `next` distinguishes the two `defined` sub-states from `exploration.md` presence. | `internal/cli/phase2.go:437-446` |
| C32 | `fileExistsAt` is an `os.Stat` wrapper — no emptiness, kind or readability discrimination. | `internal/cli/phase2.go:555-558` |
| C33 | `next` emits a `HarnessTask` with `--format text\|harness-json`. | `internal/cli/phase2.go:360-407` |
| C34 | `analyze` dispatches to `runManualPhase` when `--manual`/`--skip-llm` is set; `define` and `explore` follow the identical shape. | `internal/cli/cobra.go:603-605,651-653,693-695` |
| C35 | `addManualFlag` / `isManualFlag` / `runManualPhase` are the shared `--manual` helpers. | `internal/cli/cobra.go:3407-3436` |
| C36 | `apply --mode prepare` already exists as a shipped mode — the §5.2 collision. | `internal/cli/cobra.go:822-824,836,840`; `SPEC.md:81` |
| C37 | `openStoreFromCmd` reads the `--path` string flag, defaults it to `"."`, and calls `store.FindProjectRoot` — so a `--path` that resolves nowhere useful fails **inside** `RunE`, not at parse time. | `internal/cli/cobra.go:3782-3793` |
| C38 | **The root printer emits `error: %v` on stderr for every non-nil error returned by `rootCmd.Execute()`** — which includes pflag/Cobra **parse and arity errors raised before `RunE`**, not only `RunE` errors, because the root sets `SilenceUsage`/`SilenceErrors` (C93). This is why §10.1 cannot promise an empty stderr on any nonzero exit, and why §14.3.1's exit-1 population is a repository-emitted line carrying third-party, unsanitized text. | `internal/cli/cobra.go:33-39` |
| C39 | `exitCodeFor` is the single place a typed exit code is mapped to the process exit status. | `internal/cli/cobra.go:43-52` |
| C40 | `ExitCodeError` is the mechanism for non-1 exit codes, and exit codes are per-command contracts. | `internal/cli/exit_error.go:9-33`; `SPEC.md:135-141` |
| C41 | `amend --state` is the precedent for refusing a deliberately reserved surface with a typed exit code. | `internal/cli/c1.go:276-289` |
| C42 | `refuseIfUnappliedState` guards mutating verbs only. | `internal/cli/feature_unapply.go:464-473` |
| C43 | `verify`'s `intent_files_present` checks only `spec.md` and `exploration.md`, via `os.Stat` and `Size() == 0`. | `internal/workflow/verify.go:413-439` |
| C44 | `verify` refuses every pre-apply / mid-flight lifecycle state before V1 runs. | `internal/workflow/verify.go:245-252` |
| C45 | `verify` persists a record unless `--no-write`, so it is a writer by default. | `internal/workflow/verify.go:347-352`; `internal/cli/verify.go:151` |
| C46 | `verify` V2 parses the bytes the run captured, never the file — the single-capture precedent. | `internal/workflow/verify.go:449-455` |
| C47 | `verify`'s `--json` / `--quiet` stream routing is the report-shaped precedent §10.1 copies, and it returns a typed `ExitCodeError` whose message the root printer then prints. | `internal/cli/verify.go:108-146` |
| C48 | The presence vocabulary `absent` / `present-empty` / `present-nonempty` already exists. | `internal/workflow/verify_landed.go:122-126` |
| C49 | `snapshotArtifact` already distinguishes a non-absence read failure from absence. | `internal/workflow/verify_landed.go:234-251` |
| C50 | `snapshot-unstable` is an existing, shipped failure vocabulary — §8.3 reuses the word deliberately. | `internal/workflow/verify_landed.go:83,93` |
| C51 | `recipe-provenance.json` records `base_commit` / `generated_at` / `recipe_sha256` — not authorship — and is written only on the Path A implement path inside a Git repo. | `internal/workflow/implement.go:18-34,222-238` |
| C52 | `doctor` has a `schema_version`-tagged JSON report with struct-order field emission and a per-command exit-code function. | `internal/workflow/doctor.go:17,26-33,162-167,199-207`; `internal/cli/doctor.go:44-53` |
| C53 | `doctor` D1 validates `status.json` and legacy `feature.yaml`; no doctor check reads the intent Markdown artifacts. | `internal/workflow/doctor_d1.go:14-52` |
| C54 | Persisted JSONL schemas reject unknown `schema_version` on read — the contrast §10.2 draws for a stdout-only report. | `internal/store/reconcile_evidence.go:16,344-345` |
| C55 | "No Go map type appears in any tracked wire schema" is an existing, documented rule. | `internal/store/canonjson.go:11-17` |
| C56 | `safety.EnsureSafeRepoPath` is lexical-only containment. | `internal/safety/safety.go:11-28`; described as the coarse pre-filter at `internal/rescap/pathgate.go:50-54` |
| C57 | `rescap.GatePath`'s gate — component `Lstat`, symlink refusal, no-follow open, descriptor identity via `os.SameFile` — is the policy §7.3/§7.4 reuse; it refuses missing paths. | `internal/rescap/pathgate.go:68-83,97-120,133-155` |
| C58 | A second `fstat` on a **held** descriptor is a tautology and cannot detect a pathname swap — the honest limit §8.3 restates. | `internal/rescap/pathgate.go:181-190` |
| C59 | `openNoFollow` on non-Windows is `O_RDONLY\|O_NOFOLLOW`, and `ELOOP` is the "it became a symlink" signal. | `internal/rescap/pathopen_unix.go:20-28` |
| C60 | **Corrected in rev-2.** `rescap`'s Windows sibling is a compile-only stub, not an implementation: `openNoFollow` falls back to a bare `os.OpenFile` with no hardening, `isSymlinkLoopError` "always reports false on this target", and the file's own header says resource capture "is explicitly unsupported there". It is precedent that the pathname-resolution design could not be made cross-platform — **not** a seam to reuse. rev-1 cited it as reusable; §7.4.3 does not, and AVP-180 asserts `internal/intent` never calls it. | `internal/rescap/pathopen_windows.go:1-20` |
| C61 | A cap-plus-one bounded read is the shipped discipline, and the stated reason is that a pre-read `Stat().Size()` check can be bypassed by a file that grows. `readBounded` accumulates into a **growable** `buf` via `append`, which is exactly the allocation behavior §7.4.5 replaces with a fixed preallocation — the discipline is inherited, the allocation shape is not. | `internal/rescap/content.go:9-11,50-70` |
| C62 | `rescap` chose 5 MiB for its own single-file cap, so there is no single inherited house value for §7.4.5's 4 MiB or §9.4.2's 1 MiB. | `internal/rescap/content.go:29-32` |
| C63 | The six shipped skill surfaces and the `requiredCommands` / `requiredAnchors` parity guard. | `assets/assets_test.go:13-53,56-89,202-212,215-243` |
| C64 | The `phase-ordering` and `preflight` anchors are the mandated-sequence anchors §16.2 item 5 must leave byte-unchanged. | `assets/assets_test.go:69-75` |
| C65 | Skill surfaces must not carry bare repo-relative `docs/...md` references. | `assets/assets_test.go:281-320` |
| C66 | The phase → artifact → state contract is documented for agents. | `docs/agent-as-provider.md:33-45` |
| C67 | `docs/agent-as-provider.md` currently presents the `status.json.notes` string as what "distinguishes Path B transitions from provider output" — the wording §16.1 requires be corrected, because C8 shows it survives only until the next transition. | `docs/agent-as-provider.md:47-54` |
| C68 | The Path B operator flow instructs authoring all three Markdown artifacts by hand and then running all three `--manual` commands — which is why §6.2's readiness set matches the flow tpatch already teaches. | `docs/path-b-operator-guide.md:63-73` |
| C69 | `docs/feature-layout.md` is the authoritative map of the feature directory and its lifecycle files. | `docs/feature-layout.md:10-32,86-94` |
| C70 | WP-005 Agreed: validation precedes orchestration; `prepare --check` is the first slice; presence is not semantic quality; provenance is `unknown`; no new lifecycle state; mutating preparation is gated and its publication unit is the three Markdown files plus sidecars plus `status.json`; two ordered PRDs; slice 1 is advisory. | `docs/whitepapers/WP-005-spec-driven-workflows.md:46-100` |
| C71 | WP-005 §6.2 requires the existing-primitives pre-flight over `--manual`, `cycle` and `next`. | `docs/whitepapers/WP-005-spec-driven-workflows.md:481-518` |
| C72 | WP-005 Turn 2 records the council split and the two ordered PRDs. | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:26-82` |
| C73 | WP-005 Turn 3 pins `unknown` provenance, the ADR trigger, the atomic-publication unit, and slice-1 routing compatibility. | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:84-141` |
| C74 | WP-005 Turn 4 requires the first PRD to decide report-only vs stronger `--manual` gates in acceptance criteria. | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:143-165` |
| C75 | The acceptance-ledger machinery (AC ids → real test functions, AST-resolved) is the precedent §18.1 follows, and a byte-scanning guard can false-pass — the reason §18.28 requires sensitivity regressions. | `internal/workflow/acceptance_ledger_test.go:1-30`; `docs/handoff/CURRENT.md` → "Post-Release Review Adjudication" → F2 |

| C76 | `FindProjectRoot` resolves the start directory with `filepath.Abs` and walks upward for a `.tpatch` directory, returning `could not find .tpatch in this directory or any parent` at the filesystem root. This is the **actual** workspace-discovery trigger §9.2 binds to exit 3. | `internal/store/store.go:23-40` |
| C77 | `--path` is a persistent **string** flag on the root command; pflag validates nothing about its value, so a nonexistent or unreadable directory parses cleanly and `RunE` runs. | `internal/cli/cobra.go:66` |
| C78 | `store.Open` refuses when `.tpatch/` is absent — but it is reached only after `FindProjectRoot` has already found `.tpatch`, which is why §9.2 does not treat it as the workspace-discovery trigger and why this command never calls it. | `internal/store/store.go:134-144` |
| C79 | `ValidFeatureState` is a closed switch over the twelve `FeatureState` constants; anything else, including the empty string, is invalid. This is the predicate §9.4.2 row 18 enforces before any echo. | `internal/store/types.go:39-46` |
| C80 | `FeatureStatus`'s free-text `Notes`, its `DependsOn` list, and its `Verify`/`Rejection` sub-records are the whole of what can make a `status.json` large — the basis for §9.4.2's separate 1 MiB cap. | `internal/store/types.go:215,234,236-265` |
| C81 | `SaveFeatureStatus` is the single `status.json` writer and goes through `writeJSONAtomic`; nothing else produces the document §9.4.2 bounds. | `internal/store/store.go:363-377` |
| C82 | `rescap.readBounded` accumulates into a growable slice via `append` — the shipped cap-plus-one discipline, but not a fixed-allocation read. | `internal/rescap/content.go:50-70` |
| C83 | `rescap.GatePath`'s pathname model — component `os.Lstat` walk, then `openNoFollow`, then descriptor identity, then a redundant pathname re-`Lstat` — is the design §7.3 replaces with a rooted namespace; its ancestor-walk **policy** is retained. | `internal/rescap/pathgate.go:68-83,97-120,133-155` |
| C84 | The CI test matrix is `[ubuntu-latest, macos-latest]`: there is **no** native Windows runner today, which is why §16.1 makes adding `windows-latest` an acceptance obligation rather than an assumption. | `.github/workflows/ci.yml:22-25` |
| C85 | CI runs `gofmt -l`, `go vet ./...`, `go build ./...` and `go test ./... -count=1` on every matrix entry, so adding a runner adds full coverage rather than a partial job. | `.github/workflows/ci.yml:37-53` |
| C86 | The module targets Go 1.26, which is the version that provides `os.Root`. | `go.mod:3` |
| C87 | `AddFeature` derives the slug through `Slugify` and refuses an empty result — the "create through `tpatch add`" half of the `slug-unsafe` remediation. | `internal/store/store.go:157-163` |
| C88 | `verify`'s CLI wrapper is the shipped precedent for report routing plus a typed `ExitCodeError` whose message the root printer prints, which §10.1 composes against rather than replacing. | `internal/cli/verify.go:108-146` |
| C89 | `cmd/tpatch/main.go` is the module's only `package main`: eleven lines, no build tags and **no `//go:debug` directive today**. It is therefore the one file where §16.1's `//go:debug winsymlink=1` can be placed, since the directive is honored only in the main package. | `cmd/tpatch/main.go:1-11` |
| C90 | The CI matrix line is a single-line list — `os: [ubuntu-latest, macos-latest]` — so adding `windows-latest` is a one-token edit to one line, not a workflow restructure. The surrounding job (`runs-on: ${{ matrix.os }}`, `fail-fast: false`) already parameterises every step by matrix entry. | `.github/workflows/ci.yml:20-25` |
| C91 | `safety.EnsureSafeRepoPath` takes a **repository root** and a candidate path and tests lexical containment of the latter within the former; it has no notion of a directory handle and no notion of a root-relative name. That signature is why §7.3 step 2 drops it rather than adapting it. | `internal/safety/safety.go:11-28` |
| C92 | `internal/rescap`'s ancestor walk, no-follow open, descriptor-identity check and bounded read are a shipped, reviewed **capture** surface with its own lock/scratch lifecycle and its own refusal vocabulary — the reason ADR-034 D11 keeps it out of scope rather than migrating it onto `os.Root` in this slice. | `internal/rescap/pathgate.go:68-83,97-120,133-155`; `internal/rescap/content.go:29-32,50-70` |
| C93 | The root command is constructed with `SilenceUsage: true` and `SilenceErrors: true`, so **cobra prints neither a usage block nor the error text itself** on any failure, for any subcommand. The parse error is returned from `rootCmd.Execute()` instead. This is the claim §14.3.1 rests on, and it is why rev-3's "the usage block cobra prints with them" was false. | `internal/cli/cobra.go:56-62` |
| C94 | `exitCodeFor` returns the typed code for an `*ExitCodeError` and **`1` for anything else**, so a pflag/Cobra parse error — which is not an `*ExitCodeError` — becomes generic exit 1. This is the mechanism behind §9.2's exit-1 row, not a cobra default. | `internal/cli/cobra.go:43-52` |
| C95 | Every existing command that opens a workspace does so through `openStoreFromCmd`, which returns `FindProjectRoot`'s plain `errors.New("could not find .tpatch in this directory or any parent")` — an untyped error, hence generic **exit 1** by C94. `prepare --check` deliberately binds the same condition to **exit 3** with a full abort report; §9.2 discloses and justifies the divergence rather than leaving it for a harness author to discover. | `internal/cli/cobra.go:3782-3793`; `internal/store/store.go:23-40` |

**95 repository claims audited.**

### 23.2 Go standard library claims

Anchored by symbol, not by line (see the note above). Each row names the
acceptance row that verifies the behavior at runtime, so no design decision
rests on a citation alone.

**The `Class` column is the rev-3 addition, and it is the point of this
table.** rev-2 cited exported doc comments and unexported implementation
details side by side, with no way for a reader to tell which claims Go has
promised and which are read from source that may change in any release:

- **`contract`** — stated in the package's own documentation (a doc comment on
  an exported identifier, or the package's `GODEBUG` history). Go's
  compatibility promise covers it. A change here would be a documented
  breaking change.
- **`tripwire`** — read from **unexported** code in the pinned toolchain
  (`go1.26.5`, for a module declaring `go 1.26.1`). Go promises nothing. Every
  tripwire row names a runtime acceptance test whose job is to turn **red at
  upgrade time** rather than let the behavior change silently in the field.
  ADR-034 D10 records the maintenance obligation: a Go minor-version bump
  requires re-reading every `tripwire` row before the bump lands.

A design that rests on a `tripwire` is not automatically wrong — it is wrong
only if it *claims* the tripwire as a guarantee. §7.4.3 and §7.4.4 state each
one as a dependency, and AVP-189 guards the documents against restating a
tripwire as a promise.

| # | Class | Claim | Anchor (Go 1.26) | Verified at runtime by |
|---|---|---|---|---|
| G1 | contract | `Root` confines every operation beneath its directory: a name whose component references a location outside the root returns an error, symlinks are followed but may not reference a location outside the root, and absolute symlinks are refused. This is **logical pathname** confinement — see G13 for what it explicitly does not cover. | `os/root.go` — `Root` doc comment | AVP-149 |
| G2 | contract | On most platforms creating a `Root` opens a descriptor/handle for the directory; if the directory is moved, methods on the `Root` still reference the original directory in its new location. | `os/root.go` — `Root` doc comment | AVP-143 |
| G3 | contract | On `GOOS=js` `Root` is vulnerable to TOCTOU in symlink validation and cannot ensure operations will not escape the root; on `js` and `plan9` a `Root` references a directory **name**, not a descriptor. These are the two *currently known* unconfined targets — §7.4.1 refuses them **and every target outside the allowlist**, rather than enumerating them. | `os/root.go` — `Root` doc comment | AVP-177, AVP-179 |
| G4 | tripwire | The confined implementation is selected by the build tag `unix \|\| windows \|\| wasip1`, with the per-platform halves under `unix \|\| wasip1` and `windows`, and the unconfined fallback under `(js && wasm) \|\| plan9`. Build tags are not API. **rev-4 correction**: §7.4.1's allowlist is `unix \|\| windows`, a deliberate **strict subset** of this expression — rev-3's "copies it verbatim" is withdrawn. The asserted relation is subset-ness (never claim confinement where the stdlib does not implement it), plus the recorded reason `wasip1` is excluded despite being in the stdlib set. | `os/root_openat.go`, `os/root_unix.go`, `os/root_windows.go`, `os/root_noopenat.go` build tags | AVP-191, AVP-208, AVP-177, AVP-178 |
| G5 | contract | `Root.Lstat` describes the symbolic link itself rather than its target. | `os/root.go` — `Root.Lstat` doc comment | AVP-145, AVP-155 |
| G5a | tripwire | On Unix that is implemented as an `fstatat` relative to the resolved parent descriptor with no-follow semantics. | `os/root_unix.go` — `rootStat` | AVP-145 |
| G6 | tripwire | On Windows `Root.Lstat` opens the entry with `O_FILE_FLAG_OPEN_REPARSE_POINT` relative to the parent handle and derives the `FileInfo` from that handle via `statHandle` — it is handle-relative, not a pathname `os.Lstat`. | `os/root_windows.go` — `rootStat` | AVP-176 |
| G7 | tripwire | On Windows a `FileInfo` derived from a handle sets `vol`, `idxhi` and `idxlo` from `GetFileInformationByHandle` and deliberately leaves the struct's `path` empty so that `os.SameFile` will not re-fetch them by pathname. §7.4.4's Windows identity argument rests on this and says so. | `os/types_windows.go` — `newFileStatFromGetFileInformationByHandle`; `sameFile` → `loadFileId` | AVP-176 |
| G8 | tripwire | `(*os.File).Stat` on Windows is `statHandle(file.name, file.pfd.Sysfd)` — a stat of the held handle. On Unix it is an `fstat` of the held descriptor. So both sides of §7.4.4's identity comparison are handle-derived on both platform classes. | `os/stat_windows.go` — `(*File).Stat`; `os/stat_unix.go` — `(*File).Stat` | AVP-176 |
| G9 | tripwire | On Windows `statHandle` calls `GetFileType` and reports `FILE_TYPE_PIPE` as `ModeNamedPipe` and `FILE_TYPE_CHAR` as `ModeDevice\|ModeCharDevice`. For a **name-surrogate** reparse tag (`IO_REPARSE_TAG_SYMLINK`, `IO_REPARSE_TAG_MOUNT_POINT`) `mode()` suppresses the `ModeDir` and `GetFileType`-derived bits entirely, so a junction to a directory does not report as a directory. | `os/stat_windows.go` — `statHandle`; `os/types_windows.go` — `(*fileStat).mode`, `isReparseTagNameSurrogate` | AVP-176, AVP-198 |
| G9a | tripwire | **rev-3 correction.** The reparse-tag switch in `(*fileStat).mode` is: `IO_REPARSE_TAG_SYMLINK` → `ModeSymlink`; `IO_REPARSE_TAG_AF_UNIX` → **`ModeSocket`**; `IO_REPARSE_TAG_DEDUP` → **no type bit at all**, deliberately treated as a regular file with an explanatory comment; **`default`** (which is where a junction's `IO_REPARSE_TAG_MOUNT_POINT` lands) → `ModeIrregular`. rev-2's "every other reparse tag maps to `ModeIrregular`" was false for AF_UNIX and DEDUP, which is why §7.3 step 3's predicate is a *refusal* predicate backed by the `IsRegular` kind gate rather than the whole kind policy. | `os/types_windows.go` — `(*fileStat).mode` | AVP-198 |
| G10 | tripwire | `Root.OpenFile` passes the caller's flags through to the platform `openat`; on Unix it additionally ORs `O_NOFOLLOW\|O_CLOEXEC` itself, and converts the resulting symlink signal into an **in-root resolution** rather than a refusal. A caller therefore cannot obtain a final-leaf no-follow refusal from `Root` — which is why §7.4.3 **removed** the caller-supplied `O_NOFOLLOW` in rev-3 rather than keeping an inert flag. | `os/root_unix.go` — `rootOpenFileNolog`; `os/root_openat.go` — `doInRoot`, `case errSymlink` | AVP-107, AVP-108, AVP-118 |
| G11 | tripwire | `Root` reports an escape attempt as an ordinary `*os.PathError` wrapping an unexported sentinel, so a caller cannot discriminate it from other open failures. This is why ladder rows 10 and 11 share the `unreadable` state. | `os/file.go` — `errPathEscapes` | AVP-149 |
| G12 | contract | `io.ReadFull` into a fixed buffer returns `io.EOF` when nothing was read, `io.ErrUnexpectedEOF` on a short fill, and `nil` on a complete fill — the exact three-way split §7.4.5's ladder needs, with no allocation of its own. | `io` — `ReadFull` doc comment | AVP-173, AVP-174 |
| G13 | contract | **The sentence rev-2 elided.** The `Root` doc comment states verbatim: *"Methods on `Root` do not prohibit traversal of filesystem boundaries, Linux bind mounts, /proc special files, or access to Unix device files."* This is the documented basis for §7.4.2's logical-versus-physical distinction and for AVP-189's over-claim guard. | `os/root.go` — `Root` doc comment | AVP-189, AVP-190 |
| G14 | tripwire | `(*fileStat).Mode` consults the `winsymlink` GODEBUG setting and, when it is `0`, recomputes the mode with `modePreGo1_23`, under which a junction (`IO_REPARSE_TAG_MOUNT_POINT`) maps to **`ModeSymlink`** instead of `ModeIrregular`. The refusal outcome is stable across both values; the mode bits an assertion sees are not. | `os/types_windows.go` — `(*fileStat).Mode`, `modePreGo1_23` | AVP-198 |
| G15 | contract | `winsymlink` is a registered `GODEBUG` setting whose default changed to `1` in Go 1.23; a module declaring `go 1.23` or later gets `1` unless a `//go:debug` directive or the `GODEBUG` environment variable says otherwise. A `//go:debug` directive is honored **only in the main package**, and the environment variable still overrides it at process start — §7.4.3 states that limit rather than claiming the directive is absolute. | `internal/godebugs/table.go` — `{Name: "winsymlink", Package: "os", Changed: 23, Old: "0"}`; the Go toolchain's `//go:debug` documentation | AVP-198 |
| G16 | tripwire | For a `FILE_TYPE_PIPE` or `FILE_TYPE_CHAR` handle, Windows `statHandle` returns early with a `fileStat` carrying only `name` and `filetype` — `vol`, `idxhi` and `idxlo` are all zero. `os.SameFile` over two such values is therefore degenerate (it compares zeros and reports "same"). The design does not depend on it: both such handles fail `IsRegular()` at ladder rows 7 and 14 before any identity conclusion is used. Recorded so the limit is known rather than discovered. | `os/stat_windows.go` — `statHandle` | AVP-176, AVP-198 |
| G17 | tripwire | `rootOpenFileNolog` computes `openFlag := syscall.O_NOFOLLOW \| syscall.O_CLOEXEC \| flag` and passes it to `unix.Openat`, then constructs the `*File` with `unix.HasNonblockFlag(flag)`. **Caller-flag forwarding is not documented anywhere in `os`.** §7.4.3's `O_NONBLOCK` **non-wedging-open** property rests entirely on this, and AVP-200 is the Go-upgrade tripwire test that fails loudly if a release stops forwarding it. The property is scoped to the open; no read-time claim follows from it (§7.4.2, AVP-207). | `os/root_unix.go` — `rootOpenFileNolog`; `internal/syscall/unix` — `HasNonblockFlag` | AVP-200, AVP-107 |
| G18 | contract | The `io/fs` **package documentation's `# Path Names` section** defines the path-name syntax every `fs` interface operates on: "Path names are UTF-8-encoded, unrooted, slash-separated sequences of path elements, like 'x/y/z'. Path names must not contain an element that is '.' or '..' or the empty string, **except for the special case that the name `"."` may be used for the root directory**. Paths must not start or end with a slash." `fs.ValidPath`'s own doc comment states only that it "reports whether the given path name is valid for use in a call to Open", notes that paths are slash-separated on all systems, and **delegates to that section by link** for the details. **rev-4 correction**: rev-3 anchored the full property list on `ValidPath`'s doc comment alone, which does not contain it. The anchor is the package doc section; the function is the predicate that implements it. Note the `"."` special case: §7.3 step 2's names are all multi-element constants joined to a canonical slug, so none of them is `"."` and the distinction does not change any composed name — but a future refactor that passed a bare `"."` would satisfy `fs.ValidPath` and must not be assumed refused by it. | `io/fs` — package doc `# Path Names` section (`$GOROOT/src/io/fs/fs.go`, package comment); `io/fs` — `ValidPath` doc comment and implementation | AVP-144 |
| G19 | tripwire | `os.Root`'s resolver caps symlink traversal at `rootMaxSymlinks = 8`, described in source as `__POSIX_SYMLOOP_MAX`. The design does not depend on the specific number — it depends only on there being *a* finite limit, which bounds the number of resolution steps (not their duration) — and no acceptance row asserts the value. | `os/root.go` — `rootMaxSymlinks` | AVP-149 |
| G20 | contract | `io.LimitReader(r, n)` returns a Reader that reads at most `n` bytes, after which it reports `io.EOF`. Therefore `io.ReadAll(io.LimitReader(f, Max+1))` returns at most `Max+1` bytes and its **total allocation is bounded** by `O(Max)`. **rev-4 records this to prevent the opposite over-correction**: rev-2/rev-3 wrote that the allocation "is not bounded", which is false. The reason §7.4.5 rejects the form is cost *shape*, not boundedness. | `io` — `LimitReader` doc comment, `LimitedReader` | AVP-171, AVP-172 |
| G21 | tripwire | `io.ReadAll` starts from a small slice and grows it with `append`, so a single bounded read still performs a *sequence* of allocations of increasing size with copies between them. The starting size and growth factor are unexported implementation detail and may change in any release. §7.4.5's fixed-buffer choice rests on flattening this variable, per-capture, five-times-repeated cost — not on a boundedness defect (G20). | `io` — `ReadAll` | AVP-197, AVP-172 |
| G22 | contract | `make([]byte, n)` yields a slice whose elements are set to the zero value, so the single `MaxArtifactBytes+1` allocation of §7.4.5 also costs one ~4 MiB zeroing pass — once per invocation, never per capture. Stated so the honest cost sentence covers the zeroing as well as the allocation. | Go language specification, "Making slices, maps and channels" (the `make` builtin); `builtin` — `make` doc comment | AVP-197 |

**24 Go standard library claims audited**: `G1`…`G22` plus the two sub-rows
`G5a` and `G9a`, which split rev-2 claims that mixed a contract with a
tripwire. **10 are `contract`** (G1, G2, G3, G5, G12, G13, G15, G18, G20, G22)
and **14 are `tripwire`** (G4, G5a, G6, G7, G8, G9, G9a, G10, G11, G14, G16,
G17, G19, G21); 10 + 14 = 24. Every tripwire names a runtime verifier, and
ADR-034 D10 makes re-reading all fourteen a precondition of any Go
minor-version bump.
